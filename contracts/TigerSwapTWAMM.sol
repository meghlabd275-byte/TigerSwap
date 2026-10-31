// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./TigerSwapInterfaces.sol";

/**
 * @title TigerSwapTWAMM
 * @notice Time-weighted average market maker: long-term orders execute
 *         continuously rather than in one transaction.
 *
 * A single large swap moves the price against itself and can be sandwiched.
 * Splitting the same order across a period removes both problems: there is no
 * single transaction to front-run, and the average execution price approaches
 * the time-average price rather than the price after the order's own impact.
 *
 * ## The execution maths
 *
 * Between two updates the sell rates are constant, so the pool trades along a
 * constant-product curve with two linear flows. Each flow is applied as an
 * exact swap that preserves `x * y`:
 *
 *     token0 sellers:  x' = x + a,       y' = x*y / x'
 *     token1 sellers:  y' = y + b,       x' = x*y / y'
 *
 * When both sides sell in the same period the flows are applied in sequence,
 * token0 first, so the policy is deterministic and every step is exact.
 *
 * Gradual execution is deliberately *path independent*: splitting a swap into
 * increments at the same ratio returns exactly what one swap of the same total
 * returns (up to integer rounding). Nothing is lost or invented by the
 * accounting, which is the property the tests pin down.
 *
 * The benefit is elsewhere. A single large swap is one transaction that a
 * searcher can sandwich, and it moves the price in one step that everyone else
 * then trades against. Slicing the same order over time removes the single
 * transaction to front-run and spreads the price movement, so the seller's
 * exposure to other flow is averaged rather than concentrated at one instant.
 *
 * ## Order accounting
 *
 * Orders are funded in escrow at creation, so an order can never fail to
 * deliver the tokens it sells. Each update distributes the period's output
 * across the orders on a side in proportion to the sell amount each
 * contributed. Because every order on a side executes at the same average
 * price, that distribution is exact and leaves no rounding dust in the pool.
 */
contract TigerSwapTWAMM {
    // ============ Constants ============
    uint256 public constant FEE_DENOMINATOR = 1e6;
    /// @dev Hard cap on orders touched in one update, so `executeVirtualOrders`
    ///      cannot be made to run out of gas by opening many orders.
    uint256 public constant MAX_ORDERS_PER_UPDATE = 64;
    uint256 public constant MIN_ORDER_DURATION = 1 hours;
    uint256 public constant MAX_ORDER_DURATION = 365 days;

    // ============ Types ============
    struct LongTermOrder {
        address owner;
        bool isToken0;          // true when selling token0 for token1
        uint256 sellRate;       // sellToken per second, in token units
        uint256 expiration;     // timestamp the order stops selling
        uint256 escrow;         // sellToken still held for this order
        uint256 proceeds;       // buyToken owed to the owner
        bool active;
    }

    /// @dev Working set for one virtual-order update. Grouping it into a single
    ///      memory struct keeps every helper's stack shallow, which matters
    ///      because the IR optimizer's scheduling is what limits how many live
    ///      locals these loops may hold.
    struct UpdateContext {
        uint256 nowTs;
        uint256 elapsed;
        uint256 limit;
        uint256 grossIn0;
        uint256 grossIn1;
        uint256 netIn0;
        uint256 netIn1;
        /// @dev Per-order sell amount for this update, indexed alongside `orderIds`.
        uint256[] gross;
        /// @dev Payout pass: which side is being credited, its total, its output.
        bool sideToken0;
        uint256 grossTotal;
        uint256 output;
    }

    // ============ Immutables ============
    address public immutable token0;
    address public immutable token1;

    // ============ State ============
    address public owner;
    address public futureOwner;
    address public feeRecipient;

    /// @dev Fee on the sell side, in `FEE_DENOMINATOR` units.
    uint256 public fee;
    /// @dev Fees retained per token, claimable by the fee recipient.
    uint256 public feeReserve0;
    uint256 public feeReserve1;

    /// @dev Real pool reserves, excluding nothing: fees are tracked separately
    ///      so LPs and the fee recipient never claim the same tokens.
    uint256 public reserve0;
    uint256 public reserve1;

    uint256 public totalSupply;
    mapping(address => uint256) public balanceOf;
    mapping(address => mapping(address => uint256)) public allowance;

    /// @dev Timestamp the virtual orders were last advanced to.
    uint256 public lastVirtualOrderTimestamp;

    uint256 public nextOrderId = 1;
    mapping(uint256 => LongTermOrder) public orders;
    uint256[] public orderIds;

    // ============ Events ============
    event OrderCreated(
        uint256 indexed orderId,
        address indexed owner,
        bool isToken0,
        uint256 sellRate,
        uint256 expiration,
        uint256 escrow
    );
    event OrderWithdrawn(uint256 indexed orderId, address indexed owner, uint256 proceeds, uint256 refund);
    event OrderCancelled(uint256 indexed orderId, address indexed owner, uint256 refund);
    event VirtualOrdersExecuted(
        uint256 timestamp,
        uint256 amountIn0,
        uint256 amountIn1,
        uint256 amountOut0,
        uint256 amountOut1
    );
    event LiquidityAdded(address indexed provider, uint256 amount0, uint256 amount1, uint256 shares);
    event LiquidityRemoved(address indexed provider, uint256 amount0, uint256 amount1, uint256 shares);
    event FeesCollected(address indexed to, uint256 amount0, uint256 amount1);
    event FeeUpdated(uint256 previousFee, uint256 newFee);
    event OwnershipTransferred(address indexed previousOwner, address indexed newOwner);
    event Transfer(address indexed from, address indexed to, uint256 value);
    event Approval(address indexed tokenOwner, address indexed spender, uint256 value);

    // ============ Modifiers ============
    modifier onlyOwner() {
        require(msg.sender == owner, "TigerSwap: NOT_OWNER");
        _;
    }

    modifier nonReentrant() {
        require(_reentrancyStatus != 2, "TigerSwap: REENTRANT");
        _reentrancyStatus = 2;
        _;
        _reentrancyStatus = 1;
    }

    uint256 private _reentrancyStatus = 1;

    constructor(address _token0, address _token1, address _feeRecipient, uint256 _fee) {
        require(_token0 != address(0) && _token1 != address(0), "TigerSwap: ZERO_TOKEN");
        require(_token0 != _token1, "TigerSwap: SAME_TOKEN");
        require(_feeRecipient != address(0), "TigerSwap: ZERO_RECIPIENT");
        require(_fee < FEE_DENOMINATOR / 10, "TigerSwap: FEE_TOO_HIGH");

        token0 = _token0;
        token1 = _token1;
        owner = msg.sender;
        feeRecipient = _feeRecipient;
        fee = _fee;
        lastVirtualOrderTimestamp = block.timestamp;
    }

    // ============ Integer square root ============

    /**
     * @notice Integer square root, rounded down.
     * @dev Newton's method seeded from a power of two at least as large as the
     *      true root, which makes the iteration converge monotonically from
     *      above. Seven steps are enough for a 256-bit input; the closing
     *      `min` returns the floor without a multiplication that could
     *      overflow.
     */
    function sqrt(uint256 value) public pure returns (uint256) {
        if (value == 0) {
            return 0;
        }
        uint256 guess = 1 << (log2(value) >> 1);
        unchecked {
            guess = (guess + value / guess) >> 1;
            guess = (guess + value / guess) >> 1;
            guess = (guess + value / guess) >> 1;
            guess = (guess + value / guess) >> 1;
            guess = (guess + value / guess) >> 1;
            guess = (guess + value / guess) >> 1;
            guess = (guess + value / guess) >> 1;
            uint256 other = value / guess;
            return guess < other ? guess : other;
        }
    }

    /// @dev Floor of log2(value). `value` must be non-zero.
    function log2(uint256 value) internal pure returns (uint256 result) {
        if (value >> 128 > 0) {
            value >>= 128;
            result += 128;
        }
        if (value >> 64 > 0) {
            value >>= 64;
            result += 64;
        }
        if (value >> 32 > 0) {
            value >>= 32;
            result += 32;
        }
        if (value >> 16 > 0) {
            value >>= 16;
            result += 16;
        }
        if (value >> 8 > 0) {
            value >>= 8;
            result += 8;
        }
        if (value >> 4 > 0) {
            value >>= 4;
            result += 4;
        }
        if (value >> 2 > 0) {
            value >>= 2;
            result += 2;
        }
        if (value >> 1 > 0) {
            result += 1;
        }
    }

    // ============ Liquidity ============

    /**
     * @notice Seeds or adds to the pool.
     * @dev The first deposit sets the price and mints `sqrt(amount0*amount1)`
     *      shares; later deposits must match the current ratio within the
     *      caller's tolerance, and the smaller side determines the shares.
     */
    function addLiquidity(uint256 amount0, uint256 amount1, uint256 minShares)
        external
        nonReentrant
        returns (uint256 shares)
    {
        require(amount0 > 0 && amount1 > 0, "TigerSwap: ZERO_AMOUNT");
        _executeVirtualOrders();

        if (totalSupply == 0) {
            shares = sqrt(amount0 * amount1);
        } else {
            uint256 shares0 = (amount0 * totalSupply) / reserve0;
            uint256 shares1 = (amount1 * totalSupply) / reserve1;
            shares = shares0 < shares1 ? shares0 : shares1;
        }
        require(shares > 0, "TigerSwap: ZERO_SHARES");
        require(shares >= minShares, "TigerSwap: SLIPPAGE");

        require(IERC20(token0).transferFrom(msg.sender, address(this), amount0), "TigerSwap: TRANSFER_IN");
        require(IERC20(token1).transferFrom(msg.sender, address(this), amount1), "TigerSwap: TRANSFER_IN");

        reserve0 += amount0;
        reserve1 += amount1;
        totalSupply += shares;
        balanceOf[msg.sender] += shares;

        emit LiquidityAdded(msg.sender, amount0, amount1, shares);
        emit Transfer(address(0), msg.sender, shares);
    }

    /**
     * @notice Withdraws liquidity at the current ratio.
     * @param shares Shares to burn.
     * @param minAmount0 Minimum token0 out, for slippage protection.
     * @param minAmount1 Minimum token1 out.
     */
    function removeLiquidity(uint256 shares, uint256 minAmount0, uint256 minAmount1)
        external
        nonReentrant
        returns (uint256 amount0, uint256 amount1)
    {
        require(shares > 0, "TigerSwap: ZERO_SHARES");
        require(balanceOf[msg.sender] >= shares, "TigerSwap: BALANCE");
        _executeVirtualOrders();

        amount0 = (reserve0 * shares) / totalSupply;
        amount1 = (reserve1 * shares) / totalSupply;
        require(amount0 >= minAmount0 && amount1 >= minAmount1, "TigerSwap: SLIPPAGE");
        require(amount0 > 0 && amount1 > 0, "TigerSwap: ZERO_OUTPUT");

        balanceOf[msg.sender] -= shares;
        totalSupply -= shares;
        reserve0 -= amount0;
        reserve1 -= amount1;

        require(IERC20(token0).transfer(msg.sender, amount0), "TigerSwap: TRANSFER_OUT");
        require(IERC20(token1).transfer(msg.sender, amount1), "TigerSwap: TRANSFER_OUT");

        emit LiquidityRemoved(msg.sender, amount0, amount1, shares);
        emit Transfer(msg.sender, address(0), shares);
    }

    // ============ Long-term orders ============

    /**
     * @notice Opens a long-term order that sells `sellRate` per second until
     *         `expiration`.
     * @dev The whole sell amount is taken into escrow up front, so the order
     *      cannot fail to deliver. `sellRate * (expiration - now)` is the total
     *      sold; anything unspent is refunded on withdrawal.
     */
    function createOrder(bool isToken0, uint256 sellRate, uint256 expiration)
        external
        nonReentrant
        returns (uint256 orderId)
    {
        require(sellRate > 0, "TigerSwap: ZERO_RATE");
        require(expiration >= block.timestamp + MIN_ORDER_DURATION, "TigerSwap: TOO_SHORT");
        require(expiration <= block.timestamp + MAX_ORDER_DURATION, "TigerSwap: TOO_LONG");
        require(orderIds.length < 100_000, "TigerSwap: TOO_MANY_ORDERS");

        // Advance the pool before adding a new rate, so the new order does not
        // earn proceeds from the period before it existed.
        _executeVirtualOrders();

        uint256 duration = expiration - block.timestamp;
        uint256 escrow = sellRate * duration;

        orderId = nextOrderId++;
        // Fields are written one at a time rather than through a struct literal:
        // the literal needs all seven values live at once, which is what pushes
        // this function over the stack limit under the IR optimizer.
        LongTermOrder storage order = orders[orderId];
        order.owner = msg.sender;
        order.isToken0 = isToken0;
        order.sellRate = sellRate;
        order.expiration = expiration;
        order.escrow = escrow;
        order.active = true;
        orderIds.push(orderId);

        address sellToken = isToken0 ? token0 : token1;
        require(IERC20(sellToken).transferFrom(msg.sender, address(this), escrow), "TigerSwap: TRANSFER_IN");

        emit OrderCreated(orderId, msg.sender, isToken0, sellRate, expiration, escrow);
    }

    /**
     * @notice Withdraws a finished order's proceeds and any unspent escrow.
     */
    function withdrawOrder(uint256 orderId) external nonReentrant returns (uint256 proceeds, uint256 refund) {
        LongTermOrder storage order = orders[orderId];
        require(order.owner == msg.sender, "TigerSwap: NOT_OWNER");
        require(order.active, "TigerSwap: NOT_ACTIVE");

        _executeVirtualOrders();

        (proceeds, refund) = _settleOrder(orderId, order, false);
    }

    /**
     * @notice Cancels an active order, refunding unspent escrow and paying out
     *         proceeds earned so far.
     */
    function cancelOrder(uint256 orderId) external nonReentrant returns (uint256 proceeds, uint256 refund) {
        LongTermOrder storage order = orders[orderId];
        require(order.owner == msg.sender, "TigerSwap: NOT_OWNER");
        require(order.active, "TigerSwap: NOT_ACTIVE");

        _executeVirtualOrders();

        (proceeds, refund) = _settleOrder(orderId, order, true);
    }

    /**
     * @dev Clears an order and pays out what it earned plus what it did not
     *      spend. Shared by withdrawal and cancellation, which differ only in
     *      the event they emit.
     */
    function _settleOrder(uint256 orderId, LongTermOrder storage order, bool cancelled)
        internal
        returns (uint256 proceeds, uint256 refund)
    {
        proceeds = order.proceeds;
        refund = order.escrow;

        order.proceeds = 0;
        order.escrow = 0;
        order.active = false;

        address buyToken = order.isToken0 ? token1 : token0;
        address sellToken = order.isToken0 ? token0 : token1;

        if (proceeds > 0) {
            require(IERC20(buyToken).transfer(msg.sender, proceeds), "TigerSwap: TRANSFER_OUT");
        }
        if (refund > 0) {
            require(IERC20(sellToken).transfer(msg.sender, refund), "TigerSwap: TRANSFER_OUT");
        }

        if (cancelled) {
            emit OrderCancelled(orderId, msg.sender, refund);
        } else {
            emit OrderWithdrawn(orderId, msg.sender, proceeds, refund);
        }
    }

    // ============ Virtual order execution ============

    /**
     * @notice Advances every active order to the current block timestamp.
     * @dev Public and permissionless: anyone may call it, and a caller cannot
     *      gain by choosing the moment, because the result is a function of
     *      elapsed time only.
     */
    function executeVirtualOrders() external nonReentrant {
        _executeVirtualOrders();
    }

    function _executeVirtualOrders() internal {
        UpdateContext memory ctx;
        ctx.nowTs = block.timestamp;
        ctx.elapsed = ctx.nowTs - lastVirtualOrderTimestamp;
        if (ctx.elapsed == 0) {
            return;
        }

        uint256 count = orderIds.length;
        ctx.limit = count < MAX_ORDERS_PER_UPDATE ? count : MAX_ORDERS_PER_UPDATE;

        // Every order's contribution is computed once, from a single consistent
        // reading of the state, and kept in memory. The pool update and the
        // payout distribution then use the same numbers, so the shares sum to
        // the output exactly rather than drifting apart between two passes.
        ctx.gross = new uint256[](ctx.limit);
        _accumulate(ctx);

        if (ctx.netIn0 > 0 || ctx.netIn1 > 0) {
            (uint256 amountOut0, uint256 amountOut1) = _twammSwap(ctx.netIn0, ctx.netIn1);

            // Sellers of token0 receive token1, and the other way round.
            ctx.sideToken0 = true;
            ctx.grossTotal = ctx.grossIn0;
            ctx.output = amountOut1;
            _distribute(ctx);

            ctx.sideToken0 = false;
            ctx.grossTotal = ctx.grossIn1;
            ctx.output = amountOut0;
            _distribute(ctx);

            emit VirtualOrdersExecuted(ctx.nowTs, ctx.netIn0, ctx.netIn1, amountOut0, amountOut1);
        }

        _consumeEscrow(ctx);
        lastVirtualOrderTimestamp = ctx.nowTs;
    }

    /**
     * @dev Sums the sell amounts for the elapsed period, charging the fee on each
     *      input and recording the gross amount per order so the payout pass can
     *      weight by it.
     */
    function _accumulate(UpdateContext memory ctx) internal {
        for (uint256 i = 0; i < ctx.limit; ++i) {
            LongTermOrder storage order = orders[orderIds[i]];
            if (!order.active || order.escrow == 0) {
                continue;
            }

            uint256 amount = _sellAmount(order, ctx);
            if (amount == 0) {
                continue;
            }
            if (amount > order.escrow) {
                amount = order.escrow;
            }

            ctx.gross[i] = amount;
            uint256 feeAmount = (amount * fee) / FEE_DENOMINATOR;

            if (order.isToken0) {
                ctx.grossIn0 += amount;
                ctx.netIn0 += amount - feeAmount;
                feeReserve0 += feeAmount;
            } else {
                ctx.grossIn1 += amount;
                ctx.netIn1 += amount - feeAmount;
                feeReserve1 += feeAmount;
            }
        }
    }

    /**
     * @dev Decrements each order's escrow by what it sold. Done only after the
     *      payouts have been credited, so an order is never left paid but
     *      unfunded.
     */
    function _consumeEscrow(UpdateContext memory ctx) internal {
        for (uint256 i = 0; i < ctx.limit; ++i) {
            uint256 amount = ctx.gross[i];
            if (amount == 0) {
                continue;
            }
            orders[orderIds[i]].escrow -= amount;
        }
    }

    /**
     * @dev The amount an order sells over the elapsed interval, before the
     *      escrow cap. An order stops selling at its expiration, so an interval
     *      that spans the expiration only counts the part before it.
     */
    function _sellAmount(LongTermOrder storage order, UpdateContext memory ctx)
        internal
        view
        returns (uint256)
    {
        if (ctx.nowTs <= order.expiration) {
            return order.sellRate * ctx.elapsed;
        }
        // Only the portion of the interval before expiration counts.
        uint256 beforeExpiry = order.expiration > lastVirtualOrderTimestamp
            ? order.expiration - lastVirtualOrderTimestamp
            : 0;
        return order.sellRate * beforeExpiry;
    }

    /**
     * @dev Applies the period's flows to the pool, preserving `x * y`.
     *
     * A single-side flow has one exact answer: the sold tokens all accumulate,
     * so `x` grows by the whole input and `y` follows from the invariant. The
     * geometric-mean form that is often quoted for TWAMMs does *not* have this
     * property — with one side idle it leaves part of the input unaccounted for
     * — so it is not used here.
     *
     * Because each step is an exact constant-product swap, applying a flow in
     * many small steps and in one large step return the same total output. The
     * TWAMM does not manufacture a better price than the pool's own curve; it
     * removes the single transaction to sandwich.
     *
     * When both sides sell in the same period the two flows are applied in
     * sequence: token0 sellers trade first, then token1 sellers trade against
     * the updated pool. Each step is an exact constant-product swap, so the
     * invariant holds throughout, and the policy is deterministic rather than
     * depending on the order the orders were opened in.
     */
    function _twammSwap(uint256 amountIn0, uint256 amountIn1)
        internal
        returns (uint256 amountOut0, uint256 amountOut1)
    {
        uint256 x = reserve0;
        uint256 y = reserve1;
        if (x == 0 || y == 0) {
            return (0, 0);
        }
        uint256 invariant = x * y;

        if (amountIn0 > 0) {
            uint256 newX = x + amountIn0;
            uint256 newY = invariant / newX;
            amountOut1 = y > newY ? y - newY : 0;
            x = newX;
            y = newY;
        }

        if (amountIn1 > 0) {
            uint256 newY = y + amountIn1;
            uint256 newX = invariant / newY;
            amountOut0 = x > newX ? x - newX : 0;
            x = newX;
            y = newY;
        }

        reserve0 = x;
        reserve1 = y;
    }

    /**
     * @dev Credits `output` to the orders on one side, in proportion to the sell
     *      amount each contributed. Integer division leaves a remainder; it is
     *      retained as LP income rather than lost.
     */
    function _distribute(UpdateContext memory ctx) internal {
        if (ctx.grossTotal == 0 || ctx.output == 0) {
            return;
        }

        uint256 distributed;
        for (uint256 i = 0; i < ctx.limit; ++i) {
            if (ctx.gross[i] == 0) {
                continue;
            }
            LongTermOrder storage order = orders[orderIds[i]];
            if (order.isToken0 != ctx.sideToken0) {
                continue;
            }

            uint256 share = (ctx.output * ctx.gross[i]) / ctx.grossTotal;
            if (share == 0) {
                continue;
            }
            order.proceeds += share;
            distributed += share;
        }

        if (distributed < ctx.output) {
            if (ctx.sideToken0) {
                feeReserve1 += ctx.output - distributed;
            } else {
                feeReserve0 += ctx.output - distributed;
            }
        }
    }

    // ============ Views ============

    /// @notice The pool price of token0 in token1 terms, scaled by 1e18.
    function price0In1() external view returns (uint256) {
        if (reserve0 == 0) {
            return 0;
        }
        return (reserve1 * 1e18) / reserve0;
    }

    /// @notice Total sell rate currently active for a side, per second.
    function activeSellRate(bool isToken0Side) external view returns (uint256 total) {
        uint256 now_ = block.timestamp;
        uint256 count = orderIds.length;
        uint256 limit = count < MAX_ORDERS_PER_UPDATE ? count : MAX_ORDERS_PER_UPDATE;
        for (uint256 i = 0; i < limit; ++i) {
            LongTermOrder storage order = orders[orderIds[i]];
            if (order.active && order.isToken0 == isToken0Side && order.expiration > now_ && order.escrow > 0) {
                total += order.sellRate;
            }
        }
    }

    function orderCount() external view returns (uint256) {
        return orderIds.length;
    }

    // ============ Admin ============

    function setFee(uint256 newFee) external onlyOwner {
        require(newFee < FEE_DENOMINATOR / 10, "TigerSwap: FEE_TOO_HIGH");
        emit FeeUpdated(fee, newFee);
        fee = newFee;
    }

    function setFeeRecipient(address newRecipient) external onlyOwner {
        require(newRecipient != address(0), "TigerSwap: ZERO_RECIPIENT");
        feeRecipient = newRecipient;
    }

    function collectFees() external nonReentrant returns (uint256 amount0, uint256 amount1) {
        _executeVirtualOrders();
        amount0 = feeReserve0;
        amount1 = feeReserve1;
        feeReserve0 = 0;
        feeReserve1 = 0;

        // Fees are paid out of the reserves, which is why they are tracked
        // separately from LP claims.
        if (amount0 > 0) {
            require(reserve0 >= amount0, "TigerSwap: FEE_EXCEEDS_RESERVE");
            reserve0 -= amount0;
            require(IERC20(token0).transfer(feeRecipient, amount0), "TigerSwap: TRANSFER_OUT");
        }
        if (amount1 > 0) {
            require(reserve1 >= amount1, "TigerSwap: FEE_EXCEEDS_RESERVE");
            reserve1 -= amount1;
            require(IERC20(token1).transfer(feeRecipient, amount1), "TigerSwap: TRANSFER_OUT");
        }

        emit FeesCollected(feeRecipient, amount0, amount1);
    }

    function commitTransferOwnership(address newOwner) external onlyOwner {
        require(newOwner != address(0), "TigerSwap: ZERO_OWNER");
        futureOwner = newOwner;
    }

    function acceptOwnership() external {
        require(msg.sender == futureOwner, "TigerSwap: NOT_FUTURE_OWNER");
        emit OwnershipTransferred(owner, futureOwner);
        owner = futureOwner;
        futureOwner = address(0);
    }

    // ============ LP share token ============

    function approve(address spender, uint256 value) external returns (bool) {
        allowance[msg.sender][spender] = value;
        emit Approval(msg.sender, spender, value);
        return true;
    }

    function transfer(address to, uint256 value) external returns (bool) {
        _transferShares(msg.sender, to, value);
        return true;
    }

    function transferFrom(address from, address to, uint256 value) external returns (bool) {
        uint256 allowed = allowance[from][msg.sender];
        if (allowed != type(uint256).max) {
            require(allowed >= value, "TigerSwap: ALLOWANCE");
            allowance[from][msg.sender] = allowed - value;
        }
        _transferShares(from, to, value);
        return true;
    }

    function _transferShares(address from, address to, uint256 value) private {
        require(to != address(0), "TigerSwap: ZERO_TO");
        require(balanceOf[from] >= value, "TigerSwap: BALANCE");
        balanceOf[from] -= value;
        balanceOf[to] += value;
        emit Transfer(from, to, value);
    }
}
