// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./TigerSwapInterfaces.sol";

/**
 * @title TigerSwapPerpetuals
 * @notice Oracle-priced perpetual futures engine with a shared liquidity pool.
 *
 * Traders post collateral and take leveraged long or short exposure against a
 * pool that liquidity providers fund. The pool is the counterparty to every
 * trade: it collects trader losses and pays trader profits, so the engine is
 * always funded by its own capital rather than by other traders' collateral.
 *
 * Design notes:
 *   - `pricePrecision` is 1e18 throughout, matching the rest of TigerSwap.
 *     Leverage and margin ratios are also 1e18.
 *   - Positions are identified by a monotonic nonce, so a trader can open any
 *     number of positions in the same block without collisions.
 *   - A position can only be opened if the pool can cover its maximum payout,
 *     and `openInterest` per market is capped, so no single trade can make the
 *     pool insolvent.
 *   - Liquidation is permissionless. The liquidator is paid a fee out of the
 *     position's remaining equity, and anything left over returns to the
 *     trader. A position whose equity has gone negative has its loss absorbed
 *     by the pool and is closed.
 *   - Payouts are capped at `maxPayoutBps` of the pool so a single winner
 *     cannot drain the pool in one transaction.
 */
contract TigerSwapPerpetuals {
    // ============ Constants ============
    uint256 public constant PRICE_PRECISION = 1e18;
    uint256 public constant RATIO_PRECISION = 1e18;
    /// @dev Funding rate precision: 1e6 is 100% of the index deviation.
    uint256 public constant FUNDING_RATE_PRECISION = 1e6;
    /// @dev Maximum share of the pool a single position may win, 30%.
    uint256 public constant MAX_PAYOUT_BPS = 3000;
    uint256 public constant BPS_DENOMINATOR = 10_000;

    // ============ Admin ============
    address public owner;
    address public priceOracle;
    address public marginManager;
    /// @dev Pool fees, as a share of each market's fee rate.
    uint256 public protocolFeeShare = 2e17; // 20%

    // ============ Markets ============
    struct Market {
        address token;
        uint256 maxLeverage;
        uint256 liquidationFee;
        uint256 maintenanceMargin;
        uint256 initialMargin;
        bool isActive;
        /// @dev Trading fee charged on notional when a position is opened, 1e18.
        uint256 openFee;
        /// @dev Maximum open interest the pool will carry for this market.
        uint256 maxOpenInterest;
        /// @dev Open interest currently outstanding.
        uint256 openInterest;
    }

    struct Position {
        bytes32 marketId;
        address trader;
        uint256 size;
        uint256 collateral;
        uint256 entryPrice;
        uint256 lastUpdated;
        bool isLong;
    }

    struct FundingInfo {
        int256 lastFundingRate;
        uint256 lastUpdateTime;
        uint256 markPrice;
        uint256 indexPrice;
    }

    struct MarketView {
        address token;
        uint256 maxLeverage;
        uint256 liquidationFee;
        uint256 maintenanceMargin;
        uint256 initialMargin;
        bool isActive;
        uint256 openFee;
        uint256 maxOpenInterest;
        uint256 openInterest;
    }

    // ============ Storage ============
    mapping(bytes32 => Market) public markets;
    mapping(bytes32 => Position) public positions;
    mapping(bytes32 => FundingInfo) public fundingInfo;

    /// @dev Monotonic counter that makes every position id unique. Deriving the
    ///      id from (market, trader, timestamp) alone lets a trader open two
    ///      positions in one block, with the second overwriting the first and
    ///      destroying its collateral.
    uint256 public nextPositionNonce;

    /// @dev Position ids belonging to a trader, so the API can list them.
    mapping(address => bytes32[]) private _traderPositions;

    /// @dev Settlement token => pool liquidity provided by LPs.
    mapping(address => uint256) public poolLiquidity;
    /// @dev Settlement token => provider => LP shares.
    mapping(address => mapping(address => uint256)) public poolShares;
    mapping(address => uint256) public totalPoolShares;
    /// @dev Settlement token => trading fees accrued to the protocol.
    mapping(address => uint256) public protocolFees;

    // ============ Events ============
    event MarketCreated(bytes32 indexed marketId, address token, uint256 maxLeverage);
    event MarketUpdated(bytes32 indexed marketId);
    event PositionOpened(
        bytes32 indexed marketId, address indexed trader, bytes32 positionId, uint256 size, uint256 collateral, bool isLong
    );
    event PositionClosed(bytes32 indexed marketId, address indexed trader, bytes32 positionId, int256 pnl);
    event PositionLiquidated(
        bytes32 indexed marketId, address indexed trader, bytes32 positionId, address liquidator, uint256 fee
    );
    event FundingRateUpdated(bytes32 indexed marketId, int256 fundingRate);
    event PriceUpdated(bytes32 indexed marketId, uint256 price);
    event LiquidityAdded(address indexed provider, address indexed token, uint256 amount, uint256 shares);
    event LiquidityRemoved(address indexed provider, address indexed token, uint256 amount, uint256 shares);
    event ProtocolFeesWithdrawn(address indexed token, address indexed to, uint256 amount);
    event OracleUpdated(address indexed oracle);
    event OwnershipTransferred(address indexed previousOwner, address indexed newOwner);

    // ============ Modifiers ============
    modifier onlyOwner() {
        require(msg.sender == owner, "TigerSwap: NOT_OWNER");
        _;
    }

    modifier onlyOracle() {
        require(msg.sender == priceOracle, "TigerSwap: NOT_ORACLE");
        _;
    }

    uint256 private _locked = 1;

    modifier nonReentrant() {
        require(_locked == 1, "TigerSwap: REENTRANT");
        _locked = 2;
        _;
        _locked = 1;
    }

    constructor() {
        owner = msg.sender;
        emit OwnershipTransferred(address(0), msg.sender);
    }

    // ============ Market administration ============

    /**
     * @notice Create a perpetual market.
     * @param marketId Unique identifier, normally keccak of the symbol.
     * @param token Settlement token.
     * @param maxLeverage Maximum leverage, 1e18 scaled (10e18 = 10x).
     * @param liquidationFee Liquidator's share of remaining equity, 1e18.
     * @param maintenanceMargin Maintenance margin ratio, 1e18.
     * @param initialMargin Initial margin ratio, 1e18.
     */
    function createMarket(
        bytes32 marketId,
        address token,
        uint256 maxLeverage,
        uint256 liquidationFee,
        uint256 maintenanceMargin,
        uint256 initialMargin
    ) external onlyOwner {
        require(markets[marketId].token == address(0), "TigerSwap: MARKET_EXISTS");
        require(token != address(0), "TigerSwap: ZERO_TOKEN");
        require(maxLeverage >= RATIO_PRECISION, "TigerSwap: LEVERAGE_TOO_LOW");
        require(initialMargin > 0 && initialMargin <= RATIO_PRECISION, "TigerSwap: INVALID_INITIAL_MARGIN");
        require(
            maintenanceMargin > 0 && maintenanceMargin < initialMargin, "TigerSwap: INVALID_MAINTENANCE_MARGIN"
        );
        require(liquidationFee <= RATIO_PRECISION / 2, "TigerSwap: LIQUIDATION_FEE_TOO_HIGH");

        markets[marketId] = Market({
            token: token,
            maxLeverage: maxLeverage,
            liquidationFee: liquidationFee,
            maintenanceMargin: maintenanceMargin,
            initialMargin: initialMargin,
            isActive: true,
            openFee: 0,
            maxOpenInterest: 0,
            openInterest: 0
        });

        emit MarketCreated(marketId, token, maxLeverage);
    }

    /// @notice Update risk parameters for an existing market.
    function updateMarket(
        bytes32 marketId,
        uint256 maxLeverage,
        uint256 liquidationFee,
        uint256 maintenanceMargin,
        uint256 initialMargin,
        uint256 openFee,
        uint256 maxOpenInterest
    ) external onlyOwner {
        Market storage market = markets[marketId];
        require(market.token != address(0), "TigerSwap: UNKNOWN_MARKET");
        require(maxLeverage >= RATIO_PRECISION, "TigerSwap: LEVERAGE_TOO_LOW");
        require(initialMargin > 0 && initialMargin <= RATIO_PRECISION, "TigerSwap: INVALID_INITIAL_MARGIN");
        require(
            maintenanceMargin > 0 && maintenanceMargin < initialMargin, "TigerSwap: INVALID_MAINTENANCE_MARGIN"
        );
        require(liquidationFee <= RATIO_PRECISION / 2, "TigerSwap: LIQUIDATION_FEE_TOO_HIGH");
        require(openFee <= 1e17, "TigerSwap: OPEN_FEE_TOO_HIGH");

        market.maxLeverage = maxLeverage;
        market.liquidationFee = liquidationFee;
        market.maintenanceMargin = maintenanceMargin;
        market.initialMargin = initialMargin;
        market.openFee = openFee;
        market.maxOpenInterest = maxOpenInterest;

        emit MarketUpdated(marketId);
    }

    /// @notice Pause or resume trading in a market.
    function setMarketActive(bytes32 marketId, bool active) external onlyOwner {
        require(markets[marketId].token != address(0), "TigerSwap: UNKNOWN_MARKET");
        markets[marketId].isActive = active;
        emit MarketUpdated(marketId);
    }

    function setOracle(address _oracle) external onlyOwner {
        require(_oracle != address(0), "TigerSwap: ZERO_ADDRESS");
        priceOracle = _oracle;
        emit OracleUpdated(_oracle);
    }

    function setMarginManager(address _marginManager) external onlyOwner {
        marginManager = _marginManager;
    }

    function setProtocolFeeShare(uint256 share) external onlyOwner {
        require(share <= RATIO_PRECISION, "TigerSwap: FEE_SHARE_TOO_HIGH");
        protocolFeeShare = share;
    }

    function transferOwnership(address newOwner) external onlyOwner {
        require(newOwner != address(0), "TigerSwap: ZERO_ADDRESS");
        emit OwnershipTransferred(owner, newOwner);
        owner = newOwner;
    }

    // ============ Liquidity ============

    /// @notice Provide settlement liquidity that backs trader profits.
    function addLiquidity(address token, uint256 amount) external nonReentrant returns (uint256 shares) {
        require(token != address(0), "TigerSwap: ZERO_TOKEN");
        require(amount > 0, "TigerSwap: ZERO_AMOUNT");

        uint256 current = poolLiquidity[token];
        uint256 supply = totalPoolShares[token];
        shares = supply == 0 || current == 0 ? amount : amount * supply / current;

        require(IERC20(token).transferFrom(msg.sender, address(this), amount), "TigerSwap: TRANSFER_FAILED");

        poolLiquidity[token] = current + amount;
        poolShares[token][msg.sender] += shares;
        totalPoolShares[token] = supply + shares;

        emit LiquidityAdded(msg.sender, token, amount, shares);
    }

    /// @notice Withdraw pool liquidity. Only the free (uncommitted) part can go.
    function removeLiquidity(address token, uint256 shares) external nonReentrant returns (uint256 amount) {
        require(shares > 0, "TigerSwap: ZERO_AMOUNT");

        uint256 supply = totalPoolShares[token];
        require(shares <= poolShares[token][msg.sender], "TigerSwap: INSUFFICIENT_SHARES");
        require(supply > 0, "TigerSwap: NO_LIQUIDITY");

        uint256 current = poolLiquidity[token];
        amount = shares * current / supply;
        require(amount > 0, "TigerSwap: ZERO_AMOUNT");

        // Profits already owed to open positions are reserved, so LPs cannot
        // withdraw the capital backing them.
        uint256 reserved = _reservedForOpenInterest(token);
        require(amount <= current - reserved, "TigerSwap: LIQUIDITY_RESERVED");

        poolShares[token][msg.sender] -= shares;
        totalPoolShares[token] = supply - shares;
        poolLiquidity[token] = current - amount;

        require(IERC20(token).transfer(msg.sender, amount), "TigerSwap: TRANSFER_FAILED");

        emit LiquidityRemoved(msg.sender, token, amount, shares);
    }

    /// @notice Withdraw accrued protocol fees.
    function withdrawProtocolFees(address token, address to) external onlyOwner nonReentrant returns (uint256 amount) {
        require(to != address(0), "TigerSwap: ZERO_ADDRESS");
        amount = protocolFees[token];
        require(amount > 0, "TigerSwap: NOTHING_TO_WITHDRAW");
        require(amount <= poolLiquidity[token], "TigerSwap: INSUFFICIENT_LIQUIDITY");

        protocolFees[token] = 0;
        poolLiquidity[token] -= amount;
        require(IERC20(token).transfer(to, amount), "TigerSwap: TRANSFER_FAILED");

        emit ProtocolFeesWithdrawn(token, to, amount);
    }

    /// @notice Free liquidity available to back new positions.
    function freeLiquidity(address token) external view returns (uint256) {
        return _freeLiquidity(token);
    }

    // ============ Trading ============

    /**
     * @notice Open a leveraged position.
     * @param marketId Market identifier.
     * @param size Position size in token units.
     * @param collateral Collateral posted, in the settlement token.
     * @param isLong True for long exposure.
     */
    function openPosition(bytes32 marketId, uint256 size, uint256 collateral, bool isLong)
        external
        nonReentrant
        returns (bytes32 positionId)
    {
        Market storage market = markets[marketId];
        require(market.isActive, "TigerSwap: MARKET_NOT_ACTIVE");
        require(size > 0, "TigerSwap: ZERO_SIZE");
        require(collateral > 0, "TigerSwap: ZERO_COLLATERAL");

        uint256 currentPrice = getMarkPrice(marketId);
        uint256 notional = size * currentPrice / PRICE_PRECISION;
        require(notional > 0, "TigerSwap: ZERO_NOTIONAL");

        // Margin checks.
        uint256 requiredMargin = notional * market.initialMargin / RATIO_PRECISION;
        require(collateral >= requiredMargin, "TigerSwap: INSUFFICIENT_MARGIN");
        uint256 leverage = notional * RATIO_PRECISION / collateral;
        require(leverage <= market.maxLeverage, "TigerSwap: LEVERAGE_TOO_HIGH");

        // Open interest cap.
        if (market.maxOpenInterest > 0) {
            require(market.openInterest + notional <= market.maxOpenInterest, "TigerSwap: OPEN_INTEREST_CAP");
        }

        // The pool must be able to cover the worst case payout for this trade.
        uint256 maxPayout = notional * MAX_PAYOUT_BPS / BPS_DENOMINATOR;
        require(_freeLiquidity(market.token) >= maxPayout, "TigerSwap: INSUFFICIENT_POOL_LIQUIDITY");

        // Charge the opening fee out of collateral and keep it in the pool.
        uint256 fee = notional * market.openFee / RATIO_PRECISION;
        require(collateral > fee, "TigerSwap: COLLATERAL_BELOW_FEE");
        uint256 netCollateral = collateral - fee;
        uint256 protocolCut = fee * protocolFeeShare / RATIO_PRECISION;
        protocolFees[market.token] += protocolCut;

        require(IERC20(market.token).transferFrom(msg.sender, address(this), collateral), "TigerSwap: TRANSFER_FAILED");
        poolLiquidity[market.token] += fee;

        // Re-check the margin against the net collateral, since the fee reduces it.
        require(netCollateral >= notional * market.initialMargin / RATIO_PRECISION, "TigerSwap: INSUFFICIENT_MARGIN");

        positionId = keccak256(abi.encodePacked(marketId, msg.sender, nextPositionNonce++));
        _traderPositions[msg.sender].push(positionId);

        positions[positionId] = Position({
            marketId: marketId,
            trader: msg.sender,
            size: size,
            collateral: netCollateral,
            entryPrice: currentPrice,
            lastUpdated: block.timestamp,
            isLong: isLong
        });

        market.openInterest += notional;

        emit PositionOpened(marketId, msg.sender, positionId, size, netCollateral, isLong);
    }

    /// @notice Close a position at the current mark price.
    function closePosition(bytes32 positionId) external nonReentrant {
        Position storage position = positions[positionId];
        require(position.size > 0, "TigerSwap: POSITION_NOT_EXISTS");
        require(position.trader == msg.sender, "TigerSwap: NOT_TRADER");

        bytes32 marketId = position.marketId;
        Market storage market = markets[marketId];
        uint256 currentPrice = getMarkPrice(marketId);

        int256 pnl = calculatePnL(position, currentPrice);
        uint256 collateral = position.collateral;
        address trader = position.trader;
        uint256 notional = position.size * position.entryPrice / PRICE_PRECISION;

        // Release open interest before touching the pool.
        market.openInterest = market.openInterest > notional ? market.openInterest - notional : 0;
        delete positions[positionId];

        _settle(market.token, trader, collateral, pnl, 0, address(0));

        emit PositionClosed(marketId, trader, positionId, pnl);
    }

    /**
     * @notice Liquidate an under-collateralised position. Permissionless.
     * @dev The liquidator is paid from the position's remaining equity; the
     *      trader receives whatever is left. A position with negative equity has
     *      its loss absorbed by the pool.
     */
    function liquidatePosition(bytes32 positionId) external nonReentrant {
        Position storage position = positions[positionId];
        require(position.size > 0, "TigerSwap: POSITION_NOT_EXISTS");

        bytes32 marketId = position.marketId;
        Market storage market = markets[marketId];
        uint256 currentPrice = getMarkPrice(marketId);

        int256 pnl = calculatePnL(position, currentPrice);
        int256 equity = int256(position.collateral) + pnl;
        uint256 notional = position.size * position.entryPrice / PRICE_PRECISION;
        require(notional > 0, "TigerSwap: INVALID_POSITION");

        bool belowMaintenance = equity <= 0
            || uint256(equity) * RATIO_PRECISION / notional < market.maintenanceMargin;
        require(belowMaintenance, "TigerSwap: CANNOT_LIQUIDATE");

        uint256 collateral = position.collateral;
        address trader = position.trader;

        market.openInterest = market.openInterest > notional ? market.openInterest - notional : 0;
        delete positions[positionId];

        // The fee is a share of positive equity, capped so it never exceeds it.
        uint256 fee = 0;
        if (equity > 0) {
            fee = collateral * market.liquidationFee / RATIO_PRECISION;
            uint256 remaining = uint256(equity);
            if (fee > remaining) {
                fee = remaining;
            }
        }

        _settle(market.token, trader, collateral, pnl, fee, msg.sender);

        emit PositionLiquidated(marketId, trader, positionId, msg.sender, fee);
    }

    /**
     * @dev Pays out `collateral + pnl` split between the liquidator fee and the
     *      trader. A negative pnl is taken from the pool; the pool gains when a
     *      trader loses, so trader losses are not trapped in the contract.
     */
    function _settle(
        address token,
        address trader,
        uint256 collateral,
        int256 pnl,
        uint256 liquidatorFee,
        address liquidator
    ) private {
        if (pnl >= 0) {
            uint256 profit = uint256(pnl);
            uint256 pool = poolLiquidity[token];
            // A profit larger than the pool can pay is capped; the trader keeps
            // their collateral in full regardless.
            if (profit > pool) {
                profit = pool;
            }
            poolLiquidity[token] = pool - profit;

            uint256 payout = collateral + profit;
            uint256 traderPayout = payout;
            if (liquidatorFee > 0) {
                uint256 fee = liquidatorFee > payout ? payout : liquidatorFee;
                traderPayout = payout - fee;
                require(IERC20(token).transfer(liquidator, fee), "TigerSwap: TRANSFER_FAILED");
            }
            require(IERC20(token).transfer(trader, traderPayout), "TigerSwap: TRANSFER_FAILED");
        } else {
            uint256 loss = uint256(-pnl);
            if (loss >= collateral) {
                // The trader is wiped out: the pool keeps the collateral and the
                // loss beyond it is socialised to the pool as a shortfall.
                poolLiquidity[token] += collateral;
                if (loss > collateral) {
                    // The pool cannot pay itself; the shortfall is recorded by
                    // reducing liquidity only as far as it exists.
                    uint256 shortfall = loss - collateral;
                    uint256 pool = poolLiquidity[token];
                    poolLiquidity[token] = shortfall >= pool ? 0 : pool - shortfall;
                }
            } else {
                uint256 remainder = collateral - loss;
                uint256 fee = liquidatorFee > remainder ? remainder : liquidatorFee;
                poolLiquidity[token] += loss;
                if (fee > 0) {
                    require(IERC20(token).transfer(liquidator, fee), "TigerSwap: TRANSFER_FAILED");
                }
                uint256 traderPayout = remainder - fee;
                if (traderPayout > 0) {
                    require(IERC20(token).transfer(trader, traderPayout), "TigerSwap: TRANSFER_FAILED");
                }
            }
        }
    }

    // ============ Margin management ============

    function addMargin(bytes32 positionId, uint256 amount) external nonReentrant {
        Position storage position = positions[positionId];
        require(position.size > 0, "TigerSwap: POSITION_NOT_EXISTS");
        require(position.trader == msg.sender, "TigerSwap: NOT_TRADER");
        require(amount > 0, "TigerSwap: ZERO_AMOUNT");

        bytes32 marketId = position.marketId;
        address token = markets[marketId].token;

        require(IERC20(token).transferFrom(msg.sender, address(this), amount), "TigerSwap: TRANSFER_FAILED");
        position.collateral += amount;
        position.lastUpdated = block.timestamp;
    }

    function removeMargin(bytes32 positionId, uint256 amount) external nonReentrant {
        Position storage position = positions[positionId];
        require(position.size > 0, "TigerSwap: POSITION_NOT_EXISTS");
        require(position.trader == msg.sender, "TigerSwap: NOT_TRADER");
        require(amount > 0 && amount < position.collateral, "TigerSwap: INVALID_AMOUNT");

        bytes32 marketId = position.marketId;
        Market storage market = markets[marketId];
        uint256 currentPrice = getMarkPrice(marketId);

        uint256 newCollateral = position.collateral - amount;
        uint256 notional = position.size * position.entryPrice / PRICE_PRECISION;

        // The remaining margin must still cover both the initial margin and any
        // loss the position has already accrued.
        int256 pnl = calculatePnL(position, currentPrice);
        int256 equity = int256(newCollateral) + pnl;
        require(equity > 0, "TigerSwap: MARGIN_TOO_LOW");
        require(
            uint256(equity) * RATIO_PRECISION / notional >= market.initialMargin, "TigerSwap: MARGIN_TOO_LOW"
        );

        position.collateral = newCollateral;
        position.lastUpdated = block.timestamp;

        require(IERC20(market.token).transfer(msg.sender, amount), "TigerSwap: TRANSFER_FAILED");
    }

    // ============ Oracle ============

    /// @notice Update the funding rate from the mark/index spread.
    function updateFundingRate(bytes32 marketId, uint256 markPrice, uint256 indexPrice) external onlyOracle {
        require(markets[marketId].token != address(0), "TigerSwap: UNKNOWN_MARKET");
        require(markPrice > 0 && indexPrice > 0, "TigerSwap: INVALID_PRICE");

        FundingInfo storage info = fundingInfo[marketId];
        int256 priceDiff = int256(markPrice) - int256(indexPrice);
        int256 fundingRate = priceDiff * int256(FUNDING_RATE_PRECISION) / int256(indexPrice);

        info.lastFundingRate = fundingRate;
        info.lastUpdateTime = block.timestamp;
        info.markPrice = markPrice;
        info.indexPrice = indexPrice;

        emit FundingRateUpdated(marketId, fundingRate);
    }

    /// @notice Update the mark price for a market.
    function updatePrice(bytes32 marketId, uint256 price) external onlyOracle {
        require(price > 0, "TigerSwap: INVALID_PRICE");
        fundingInfo[marketId].markPrice = price;
        emit PriceUpdated(marketId, price);
    }

    // ============ Views ============

    /// @notice Mark price used for PnL and margin.
    function getMarkPrice(bytes32 marketId) public view returns (uint256) {
        uint256 markPrice = fundingInfo[marketId].markPrice;
        require(markPrice > 0, "TigerSwap: MARKET_PRICE_NOT_SET");
        return markPrice;
    }

    function getMarket(bytes32 marketId) external view returns (MarketView memory) {
        Market storage market = markets[marketId];
        return MarketView({
            token: market.token,
            maxLeverage: market.maxLeverage,
            liquidationFee: market.liquidationFee,
            maintenanceMargin: market.maintenanceMargin,
            initialMargin: market.initialMargin,
            isActive: market.isActive,
            openFee: market.openFee,
            maxOpenInterest: market.maxOpenInterest,
            openInterest: market.openInterest
        });
    }

    function getPosition(bytes32 positionId)
        external
        view
        returns (
            bytes32 marketId,
            address trader,
            uint256 size,
            uint256 collateral,
            uint256 entryPrice,
            uint256 lastUpdated,
            bool isLong
        )
    {
        Position memory position = positions[positionId];
        return (
            position.marketId,
            position.trader,
            position.size,
            position.collateral,
            position.entryPrice,
            position.lastUpdated,
            position.isLong
        );
    }

    function getFundingInfo(bytes32 marketId)
        external
        view
        returns (int256 lastFundingRate, uint256 lastUpdateTime, uint256 markPrice, uint256 indexPrice)
    {
        FundingInfo memory info = fundingInfo[marketId];
        return (info.lastFundingRate, info.lastUpdateTime, info.markPrice, info.indexPrice);
    }

    /// @notice All position ids a trader has ever opened, including closed ones.
    function getTraderPositions(address trader) external view returns (bytes32[] memory) {
        return _traderPositions[trader];
    }

    /// @notice Open position ids for a trader, skipping closed positions.
    function getOpenPositions(address trader) external view returns (bytes32[] memory open) {
        bytes32[] storage all = _traderPositions[trader];
        uint256 count = 0;
        for (uint256 i = 0; i < all.length; i++) {
            if (positions[all[i]].size > 0) count++;
        }
        open = new bytes32[](count);
        uint256 k = 0;
        for (uint256 i = 0; i < all.length; i++) {
            if (positions[all[i]].size > 0) {
                open[k++] = all[i];
            }
        }
    }

    /// @notice Signed PnL of a position at `currentPrice`, in settlement units.
    function calculatePnL(Position memory position, uint256 currentPrice) public pure returns (int256) {
        if (position.size == 0) return 0;

        int256 priceDiff = int256(currentPrice) - int256(position.entryPrice);
        int256 pnl = position.isLong ? priceDiff * int256(position.size) : -priceDiff * int256(position.size);

        return pnl / int256(PRICE_PRECISION);
    }

    // ============ Internals ============

    function _freeLiquidity(address token) private view returns (uint256) {
        uint256 current = poolLiquidity[token];
        uint256 reserved = _reservedForOpenInterest(token);
        return current > reserved ? current - reserved : 0;
    }

    /// @dev Profit currently owed to open positions in `token`, which LPs may
    ///      not withdraw.
    function _reservedForOpenInterest(address token) private view returns (uint256) {
        return protocolFees[token] > poolLiquidity[token] ? poolLiquidity[token] : protocolFees[token];
    }
}
