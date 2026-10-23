// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./TigerSwapInterfaces.sol";

/// @notice Router surface the order book needs to settle fills.
interface ITigerSwapRouter {
    function swapExactTokensForTokens(
        uint256 amountIn,
        uint256 amountOutMin,
        address[] calldata path,
        address to,
        uint256 deadline
    ) external returns (uint256[] memory amounts);

    function getAmountsOut(uint256 amountIn, address[] memory path)
        external view returns (uint256[] memory amounts);
}

/**
 * @title TigerSwapAdvancedOrders
 * @notice On-chain order book for Limit, Stop-Loss, Take-Profit, TWAP, GTC,
 *         IOC and Post-Only orders.
 *
 * Input tokens are escrowed when the order is created. Executors (keepers)
 * settle fills through the TigerSwap router, so every fill is a real AMM swap
 * at the pool's actual price rather than a price the keeper asserts. Slippage
 * protection comes from `amountOutMin`, which the router enforces.
 */
contract TigerSwapAdvancedOrders {
    enum OrderType {
        MARKET,
        LIMIT,
        STOP_LOSS,
        TAKE_PROFIT,
        TWAP,
        GTC,
        IOC,
        POST_ONLY
    }

    enum OrderSide {
        BUY,
        SELL
    }

    enum OrderStatus {
        PENDING,
        OPEN,
        PARTIALLY_FILLED,
        FILLED,
        CANCELLED,
        EXPIRED,
        TRIGGERED
    }

    struct Order {
        uint256 id;
        address owner;
        OrderType orderType;
        OrderSide side;
        address tokenIn;
        address tokenOut;
        uint256 amountIn;
        uint256 amountOutMin;
        uint256 price;
        uint256 stopPrice;
        uint256 filledAmount;
        uint256 amountOutReceived;
        uint256 createdAt;
        uint256 expiresAt;
        OrderStatus status;
        uint256 twapInterval;
        uint256 twapParts;
        uint256 twapCompletedParts;
        uint256 lastFillAt;
    }

    uint256 public constant BPS = 10_000;

    mapping(uint256 => Order) public orders;
    uint256 public orderCount;

    mapping(address => uint256[]) public userOrders;
    mapping(address => bool) public executors;

    address public priceOracle;
    address public router;
    uint256 public limitOrderFee = 30; // 0.3% of the input, taken at fill time
    mapping(address => uint256) public feesCollected;

    event OrderCreated(
        uint256 indexed orderId,
        address indexed owner,
        OrderType orderType,
        OrderSide side,
        address tokenIn,
        address tokenOut,
        uint256 amountIn,
        uint256 price
    );
    event OrderFilled(uint256 indexed orderId, uint256 fillAmount, uint256 amountOut, address executor);
    event OrderCancelled(uint256 indexed orderId, address indexed owner, string reason);
    event OrderTriggered(uint256 indexed orderId, uint256 triggerPrice);
    event ExecutorUpdated(address indexed executor, bool allowed);
    event RouterUpdated(address indexed router);
    event FeeUpdated(uint256 feeBps);
    event FeesWithdrawn(address indexed token, address indexed to, uint256 amount);

    address public owner;

    modifier onlyOwner() {
        require(msg.sender == owner, "TigerSwap: NOT_OWNER");
        _;
    }

    modifier onlyExecutor() {
        require(executors[msg.sender], "TigerSwap: NOT_EXECUTOR");
        _;
    }

    constructor(address _priceOracle, address _router) {
        require(_priceOracle != address(0), "TigerSwap: ZERO_ORACLE");
        require(_router != address(0), "TigerSwap: ZERO_ROUTER");
        owner = msg.sender;
        priceOracle = _priceOracle;
        router = _router;
    }

    // ============ Admin ============

    function setExecutor(address executor, bool allowed) external onlyOwner {
        require(executor != address(0), "TigerSwap: ZERO_ADDRESS");
        executors[executor] = allowed;
        emit ExecutorUpdated(executor, allowed);
    }

    function setPriceOracle(address oracle) external onlyOwner {
        require(oracle != address(0), "TigerSwap: ZERO_ORACLE");
        priceOracle = oracle;
    }

    function setRouter(address _router) external onlyOwner {
        require(_router != address(0), "TigerSwap: ZERO_ROUTER");
        router = _router;
        emit RouterUpdated(_router);
    }

    function setFee(uint256 feeBps) external onlyOwner {
        require(feeBps <= 500, "TigerSwap: FEE_TOO_HIGH");
        limitOrderFee = feeBps;
        emit FeeUpdated(feeBps);
    }

    function withdrawFees(address token, address to) external onlyOwner {
        uint256 amount = feesCollected[token];
        require(amount > 0, "TigerSwap: NO_FEES");
        feesCollected[token] = 0;
        require(IERC20(token).transfer(to, amount), "TigerSwap: TRANSFER_FAILED");
        emit FeesWithdrawn(token, to, amount);
    }

    // ============ Create Orders ============

    function createLimitOrder(
        OrderSide side,
        address tokenIn,
        address tokenOut,
        uint256 amountIn,
        uint256 amountOutMin,
        uint256 price,
        uint256 expiresAt
    ) external returns (uint256) {
        return _createOrder(
            OrderType.LIMIT, side, tokenIn, tokenOut, amountIn, amountOutMin, price, 0, expiresAt, 0, 0
        );
    }

    function createStopLossOrder(
        OrderSide side,
        address tokenIn,
        address tokenOut,
        uint256 amountIn,
        uint256 amountOutMin,
        uint256 stopPrice,
        uint256 expiresAt
    ) external returns (uint256) {
        return _createOrder(
            OrderType.STOP_LOSS, side, tokenIn, tokenOut, amountIn, amountOutMin, 0, stopPrice, expiresAt, 0, 0
        );
    }

    function createTakeProfitOrder(
        OrderSide side,
        address tokenIn,
        address tokenOut,
        uint256 amountIn,
        uint256 amountOutMin,
        uint256 targetPrice,
        uint256 expiresAt
    ) external returns (uint256) {
        return _createOrder(
            OrderType.TAKE_PROFIT, side, tokenIn, tokenOut, amountIn, amountOutMin, targetPrice, 0, expiresAt, 0, 0
        );
    }

    function createTWAPOrder(
        OrderSide side,
        address tokenIn,
        address tokenOut,
        uint256 amountIn,
        uint256 amountOutMin,
        uint256 parts,
        uint256 interval,
        uint256 expiresAt
    ) external returns (uint256) {
        require(parts > 0, "TigerSwap: ZERO_PARTS");
        require(interval > 0, "TigerSwap: ZERO_INTERVAL");
        return _createOrder(
            OrderType.TWAP, side, tokenIn, tokenOut, amountIn, amountOutMin, 0, 0, expiresAt, interval, parts
        );
    }

    function createGTCOrder(
        OrderSide side,
        address tokenIn,
        address tokenOut,
        uint256 amountIn,
        uint256 amountOutMin,
        uint256 price
    ) external returns (uint256) {
        return _createOrder(
            OrderType.GTC, side, tokenIn, tokenOut, amountIn, amountOutMin, price, 0, 0, 0, 0
        );
    }

    function createIOCOrder(
        OrderSide side,
        address tokenIn,
        address tokenOut,
        uint256 amountIn,
        uint256 amountOutMin,
        uint256 price
    ) external returns (uint256) {
        return _createOrder(
            OrderType.IOC, side, tokenIn, tokenOut, amountIn, amountOutMin, price, 0, block.timestamp, 0, 0
        );
    }

    function createPostOnlyOrder(
        OrderSide side,
        address tokenIn,
        address tokenOut,
        uint256 amountIn,
        uint256 amountOutMin,
        uint256 price,
        uint256 expiresAt
    ) external returns (uint256) {
        return _createOrder(
            OrderType.POST_ONLY, side, tokenIn, tokenOut, amountIn, amountOutMin, price, 0, expiresAt, 0, 0
        );
    }

    function _createOrder(
        OrderType orderType,
        OrderSide side,
        address tokenIn,
        address tokenOut,
        uint256 amountIn,
        uint256 amountOutMin,
        uint256 price,
        uint256 stopPrice,
        uint256 expiresAt,
        uint256 twapInterval,
        uint256 twapParts
    ) internal returns (uint256) {
        require(amountIn > 0, "TigerSwap: ZERO_AMOUNT");
        require(tokenIn != tokenOut, "TigerSwap: SAME_TOKEN");
        require(tokenIn != address(0) && tokenOut != address(0), "TigerSwap: ZERO_TOKEN");

        orderCount++;
        uint256 orderId = orderCount;

        Order storage order = orders[orderId];
        order.id = orderId;
        order.owner = msg.sender;
        order.orderType = orderType;
        order.side = side;
        order.tokenIn = tokenIn;
        order.tokenOut = tokenOut;
        order.amountIn = amountIn;
        order.amountOutMin = amountOutMin;
        order.price = price;
        order.stopPrice = stopPrice;
        order.createdAt = block.timestamp;
        order.expiresAt = expiresAt > 0 ? expiresAt : block.timestamp + 30 days;
        order.status = OrderStatus.OPEN;
        order.twapInterval = twapInterval;
        order.twapParts = twapParts;

        // Escrow the input so a fill is always fundable.
        require(IERC20(tokenIn).transferFrom(msg.sender, address(this), amountIn), "TigerSwap: TRANSFER_FAILED");

        userOrders[msg.sender].push(orderId);

        emit OrderCreated(orderId, msg.sender, orderType, side, tokenIn, tokenOut, amountIn, price);
        return orderId;
    }

    // ============ Execution ============

    /// @notice Fill an open order through the AMM. `currentPrice` is the keeper's
    ///         view of the oracle price and gates limit/stop conditions; the
    ///         actual output is whatever the pool returns.
    function executeOrder(uint256 orderId, uint256 currentPrice, uint256 fillAmount)
        external
        onlyExecutor
        returns (uint256 amountOut)
    {
        Order storage order = orders[orderId];
        require(order.status == OrderStatus.OPEN || order.status == OrderStatus.PARTIALLY_FILLED,
            "TigerSwap: ORDER_NOT_OPEN");
        require(block.timestamp <= order.expiresAt, "TigerSwap: ORDER_EXPIRED");
        require(_checkPriceCondition(order, currentPrice), "TigerSwap: PRICE_NOT_MET");

        uint256 remaining = order.amountIn - order.filledAmount;
        require(remaining > 0, "TigerSwap: ORDER_FILLED");

        uint256 actualFill = fillAmount == 0 || fillAmount > remaining ? remaining : fillAmount;
        if (order.orderType == OrderType.TWAP) {
            uint256 part = order.amountIn / order.twapParts;
            if (actualFill > part) actualFill = part;
            require(block.timestamp >= order.lastFillAt + order.twapInterval, "TigerSwap: TWAP_TOO_SOON");
        }

        uint256 fee = actualFill * limitOrderFee / BPS;
        uint256 swapAmount = actualFill - fee;

        amountOut = _swapThroughRouter(order.tokenIn, order.tokenOut, swapAmount, order.amountOutMin);
        require(amountOut >= order.amountOutMin, "TigerSwap: INSUFFICIENT_OUTPUT");

        order.filledAmount += actualFill;
        order.amountOutReceived += amountOut;
        order.lastFillAt = block.timestamp;
        feesCollected[order.tokenIn] += fee;

        if (order.filledAmount >= order.amountIn) {
            order.status = OrderStatus.FILLED;
        } else if (order.orderType == OrderType.TWAP) {
            order.twapCompletedParts++;
            order.status = order.twapCompletedParts >= order.twapParts
                ? OrderStatus.FILLED
                : OrderStatus.PARTIALLY_FILLED;
        } else {
            order.status = OrderStatus.PARTIALLY_FILLED;
        }

        require(IERC20(order.tokenOut).transfer(order.owner, amountOut), "TigerSwap: TRANSFER_FAILED");

        emit OrderFilled(orderId, actualFill, amountOut, msg.sender);
    }

    /// @notice Trigger a stop-loss or take-profit order.
    function triggerOrder(uint256 orderId, uint256 currentPrice) external onlyExecutor {
        Order storage order = orders[orderId];
        require(
            order.orderType == OrderType.STOP_LOSS || order.orderType == OrderType.TAKE_PROFIT,
            "TigerSwap: NOT_TRIGGERABLE"
        );
        require(order.status == OrderStatus.OPEN, "TigerSwap: ORDER_NOT_OPEN");
        require(_checkTriggerCondition(order, currentPrice), "TigerSwap: TRIGGER_NOT_MET");

        order.status = OrderStatus.TRIGGERED;

        uint256 remaining = order.amountIn - order.filledAmount;
        uint256 fee = remaining * limitOrderFee / BPS;
        uint256 amountOut = _swapThroughRouter(order.tokenIn, order.tokenOut, remaining - fee, order.amountOutMin);

        order.filledAmount = order.amountIn;
        order.amountOutReceived += amountOut;
        order.status = OrderStatus.FILLED;
        feesCollected[order.tokenIn] += fee;

        require(IERC20(order.tokenOut).transfer(order.owner, amountOut), "TigerSwap: TRANSFER_FAILED");

        emit OrderTriggered(orderId, currentPrice);
        emit OrderFilled(orderId, remaining, amountOut, msg.sender);
    }

    /// @notice Cancel an open order and return escrowed input.
    function cancelOrder(uint256 orderId) external {
        Order storage order = orders[orderId];
        require(order.owner == msg.sender, "TigerSwap: NOT_OWNER");
        require(
            order.status == OrderStatus.OPEN || order.status == OrderStatus.PARTIALLY_FILLED,
            "TigerSwap: CANNOT_CANCEL"
        );

        uint256 remaining = order.amountIn - order.filledAmount;
        order.status = OrderStatus.CANCELLED;
        if (remaining > 0) {
            require(IERC20(order.tokenIn).transfer(order.owner, remaining), "TigerSwap: TRANSFER_FAILED");
        }

        emit OrderCancelled(orderId, msg.sender, "cancelled by owner");
    }

    /// @notice Expire an order past its deadline and return escrowed input.
    function expireOrder(uint256 orderId) external {
        Order storage order = orders[orderId];
        require(
            order.status == OrderStatus.OPEN || order.status == OrderStatus.PARTIALLY_FILLED,
            "TigerSwap: ORDER_NOT_OPEN"
        );
        require(block.timestamp > order.expiresAt, "TigerSwap: NOT_EXPIRED");

        uint256 remaining = order.amountIn - order.filledAmount;
        order.status = OrderStatus.EXPIRED;
        if (remaining > 0) {
            require(IERC20(order.tokenIn).transfer(order.owner, remaining), "TigerSwap: TRANSFER_FAILED");
        }

        emit OrderCancelled(orderId, order.owner, "expired");
    }

    /// @notice Cancel an IOC order that could not be filled in its block.
    function cancelIOC(uint256 orderId) external onlyExecutor {
        Order storage order = orders[orderId];
        require(order.orderType == OrderType.IOC, "TigerSwap: NOT_IOC");
        require(order.status == OrderStatus.OPEN, "TigerSwap: ORDER_NOT_OPEN");

        uint256 remaining = order.amountIn - order.filledAmount;
        order.status = OrderStatus.CANCELLED;
        if (remaining > 0) {
            require(IERC20(order.tokenIn).transfer(order.owner, remaining), "TigerSwap: TRANSFER_FAILED");
        }

        emit OrderCancelled(orderId, order.owner, "IOC expired");
    }

    // ============ Internals ============

    function _swapThroughRouter(
        address tokenIn,
        address tokenOut,
        uint256 amountIn,
        uint256 amountOutMin
    ) internal returns (uint256 amountOut) {
        address[] memory path = new address[](2);
        path[0] = tokenIn;
        path[1] = tokenOut;

        require(IERC20(tokenIn).approve(router, amountIn), "TigerSwap: APPROVE_FAILED");
        uint256[] memory amounts = ITigerSwapRouter(router).swapExactTokensForTokens(
            amountIn, amountOutMin, path, address(this), block.timestamp
        );
        amountOut = amounts[amounts.length - 1];
    }

    function _checkPriceCondition(Order storage order, uint256 currentPrice) internal view returns (bool) {
        if (order.orderType == OrderType.MARKET || order.orderType == OrderType.STOP_LOSS
            || order.orderType == OrderType.TAKE_PROFIT) {
            return true;
        }

        if (order.orderType == OrderType.LIMIT || order.orderType == OrderType.GTC
            || order.orderType == OrderType.IOC) {
            return order.side == OrderSide.BUY ? currentPrice <= order.price : currentPrice >= order.price;
        }

        if (order.orderType == OrderType.POST_ONLY) {
            // Post-only only fills at or better than the posted price.
            return order.side == OrderSide.BUY ? currentPrice < order.price : currentPrice > order.price;
        }

        if (order.orderType == OrderType.TWAP) {
            return true;
        }

        return false;
    }

    function _checkTriggerCondition(Order storage order, uint256 currentPrice) internal view returns (bool) {
        if (order.orderType == OrderType.STOP_LOSS) {
            return order.side == OrderSide.BUY
                ? currentPrice >= order.stopPrice
                : currentPrice <= order.stopPrice;
        }

        if (order.orderType == OrderType.TAKE_PROFIT) {
            return order.side == OrderSide.BUY
                ? currentPrice >= order.price
                : currentPrice <= order.price;
        }

        return false;
    }

    // ============ Views ============

    function getOrder(uint256 orderId) external view returns (Order memory) {
        return orders[orderId];
    }

    function getUserOrders(address user) external view returns (uint256[] memory) {
        return userOrders[user];
    }

    function quoteOutput(address tokenIn, address tokenOut, uint256 amountIn)
        external
        view
        returns (uint256)
    {
        address[] memory path = new address[](2);
        path[0] = tokenIn;
        path[1] = tokenOut;
        uint256[] memory amounts = ITigerSwapRouter(router).getAmountsOut(amountIn, path);
        return amounts[amounts.length - 1];
    }

    function getOrdersByStatus(OrderStatus status) external view returns (uint256[] memory) {
        uint256 count = 0;
        for (uint256 i = 1; i <= orderCount; i++) {
            if (orders[i].status == status) count++;
        }

        uint256[] memory result = new uint256[](count);
        uint256 index = 0;
        for (uint256 i = 1; i <= orderCount; i++) {
            if (orders[i].status == status) {
                result[index] = i;
                index++;
            }
        }
        return result;
    }

    function transferOwnership(address newOwner) external onlyOwner {
        require(newOwner != address(0), "TigerSwap: ZERO_ADDRESS");
        owner = newOwner;
    }
}
