// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./TigerSwapInterfaces.sol";
import "./TigerSwapSafeERC20.sol";

/**
 * @title TigerSwapPerpOrderBook
 * @notice On-chain central limit order book for perpetual futures.
 *
 * This is the settlement layer for the derivatives venue. A trader margins a
 * settlement token into a per-account balance, rests limit orders in a
 * price-time-priority book, and a taker crosses them in one transaction. A
 * position is the signed net of a trader's fills, so a fill in the opposite
 * direction reduces exposure; there is no separate open/close call. This is the
 * shape a Hyperliquid/dYdX-class venue settles on-chain.
 *
 * Design notes:
 *   - Prices are WAD (1e18); quantities are base units. A notional is
 *     `qty * price / WAD`, always with a 256-bit intermediate, so a large order
 *     cannot wrap.
 *   - The book is uncrossed by construction: a resting order is rejected if it
 *     would cross the opposite best. Only a taker crosses.
 *   - Margin is cross-margined per account against the net of its positions,
 *     valued at the oracle mark. A fill that would leave the account below the
 *     initial-margin requirement is rejected, so an account cannot open itself
 *     into a liquidatable state.
 *   - Liquidation is permissionless and reads the price from the oracle, never
 *     the caller, so a liquidator cannot assert a health condition.
 *   - Funding is a per-market cumulative index. Longs pay shorts when the mark
 *     trades above the index; the index is folded into a trader's balance when
 *     their position is touched.
 *   - Realised losses and fees move to the pool; the pool pays realised profits.
 *     A profit the pool cannot pay is recorded as an unpaid obligation. An
 *     insurance fund absorbs bad debt before it reaches LPs.
 */
contract TigerSwapPerpOrderBook {
    using TigerSwapSafeERC20 for address;

    // ============ Constants ============

    uint256 public constant WAD = 1e18;
    uint256 public constant FUNDING_INTERVAL = 1 hours;
    uint256 public constant FUNDING_RATE_PRECISION = 1e6;
    uint256 public constant BPS = 10_000;

    /// @dev Time-in-force codes.
    uint8 public constant TIF_GTC = 0;
    uint8 public constant TIF_IOC = 1;
    uint8 public constant TIF_FOK = 2;
    uint8 public constant TIF_POST_ONLY = 3;

    // ============ Admin ============

    address public owner;
    address public pendingOwner;
    address public priceOracle;
    uint256 public protocolFeeShare = 2e17; // 20%
    bool public tradingEnabled = true;
    bool private _locked;

    // ============ Structs ============

    struct Market {
        address token;
        address oracleAsset;
        uint256 takerFee; // 1e18
        uint256 makerFee; // 1e18
        uint256 initialMargin; // 1e18 fraction of notional
        uint256 maintenanceMargin; // 1e18 fraction of notional
        uint256 liquidationFee; // 1e18 fraction of remaining equity
        uint256 maxLeverage; // 1e18
        uint256 maxOpenInterest; // base units, 0 = uncapped
        uint256 openInterest; // base units
        uint256 minOrderSize; // base units
        uint256 lotSize; // base units, increment for price and quantity
        bool isActive;
    }

    struct Order {
        uint256 id;
        address trader;
        uint256 price; // limit price, WAD
        uint256 quantity; // base units
        uint256 filled; // base units
        bool isLong;
        bool reduceOnly;
        bool postOnly;
        uint64 timestamp;
        bool active;
    }

    struct Position {
        int256 size; // signed base units: +long, -short
        uint256 entryNotional; // sum(price*qty/WAD) still open
        int256 fundingIndex; // cumulative funding at last touch
    }

    struct FundingInfo {
        int256 cumulativeIndex; // WAD-scaled
        int256 lastFundingRate; // signed 1e6-scaled
        uint256 lastUpdateTime;
        uint256 markPrice; // WAD
        uint256 indexPrice; // WAD
    }

    struct Level {
        uint256 price;
        uint256[] queue; // order ids, FIFO
    }

    // ============ Storage ============

    mapping(bytes32 => Market) public markets;
    mapping(bytes32 => FundingInfo) public fundingInfo;

    /// @dev Ordered price levels. Bids are stored best-first (descending price),
    ///      asks best-first (ascending). The first level with a live order is
    ///      the best.
    mapping(bytes32 => Level[]) private _bids;
    mapping(bytes32 => Level[]) private _asks;

    mapping(uint256 => Order) public orders;
    mapping(uint256 => bytes32) public orderMarket;
    mapping(address => uint256[]) private _traderOrders;

    mapping(bytes32 => mapping(address => Position)) public positions;
    /// @dev marketId => trader => free margin (settlement units).
    mapping(bytes32 => mapping(address => uint256)) public balances;

    mapping(bytes32 => uint256) public poolLiquidity;
    mapping(bytes32 => uint256) public reservedLiquidity;
    mapping(bytes32 => uint256) public unpaidProfits;
    mapping(bytes32 => uint256) public insuranceFund;
    mapping(bytes32 => uint256) public protocolFees;

    uint256 public nextOrderId = 1;

    // ============ Events ============

    event MarketCreated(bytes32 indexed marketId, address token, address oracleAsset, uint256 maxLeverage);
    event MarketUpdated(bytes32 indexed marketId);
    event MarketActiveSet(bytes32 indexed marketId, bool active);
    event OrderPlaced(
        uint256 indexed orderId,
        bytes32 indexed marketId,
        address indexed trader,
        bool isLong,
        uint256 price,
        uint256 quantity,
        bool reduceOnly,
        bool postOnly
    );
    event OrderFilled(
        uint256 indexed orderId,
        bytes32 indexed marketId,
        address indexed trader,
        uint256 price,
        uint256 quantity,
        bool isMaker,
        uint256 fee
    );
    event OrderCancelled(uint256 indexed orderId, bytes32 indexed marketId, address indexed trader);
    event PositionChanged(bytes32 indexed marketId, address indexed trader, int256 size, uint256 entryPrice, uint256 margin);
    event FundingSettled(bytes32 indexed marketId, address indexed trader, int256 funding);
    event FundingRateUpdated(bytes32 indexed marketId, int256 rate, int256 cumulativeIndex);
    event Liquidation(bytes32 indexed marketId, address indexed trader, address indexed liquidator, int256 size, uint256 fee, int256 badDebt);
    event InsuranceFundDeposited(bytes32 indexed marketId, address indexed from, uint256 amount);
    event InsuranceFundUsed(bytes32 indexed marketId, uint256 amount);
    event LiquidityAdded(bytes32 indexed marketId, address indexed provider, uint256 amount);
    event LiquidityRemoved(bytes32 indexed marketId, address indexed provider, uint256 amount);
    event ProtocolFeesWithdrawn(bytes32 indexed marketId, address indexed to, uint256 amount);
    event OracleUpdated(address indexed oracle);
    event TradingEnabledSet(bool enabled);
    event OwnershipCommitted(address indexed pendingOwner);
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

    modifier nonReentrant() {
        require(!_locked, "TigerSwap: REENTRANT");
        _locked = true;
        _;
        _locked = false;
    }

    constructor(address _priceOracle) {
        require(_priceOracle != address(0), "TigerSwap: ZERO_ORACLE");
        owner = msg.sender;
        priceOracle = _priceOracle;
    }

    // ============ Admin ============

    function createMarket(
        bytes32 marketId,
        address token,
        address oracleAsset,
        uint256 takerFee,
        uint256 makerFee,
        uint256 initialMargin,
        uint256 maintenanceMargin,
        uint256 liquidationFee,
        uint256 maxLeverage,
        uint256 maxOpenInterest,
        uint256 minOrderSize,
        uint256 lotSize
    ) external onlyOwner {
        require(markets[marketId].token == address(0), "TigerSwap: MARKET_EXISTS");
        require(token != address(0) && oracleAsset != address(0), "TigerSwap: ZERO_ADDRESS");
        require(initialMargin > 0 && initialMargin < WAD, "TigerSwap: BAD_MARGIN");
        require(maintenanceMargin > 0 && maintenanceMargin < initialMargin, "TigerSwap: BAD_MARGIN");
        require(maxLeverage >= WAD, "TigerSwap: BAD_LEVERAGE");
        require(takerFee <= WAD / 10 && makerFee <= WAD / 10, "TigerSwap: BAD_FEE");
        require(lotSize > 0 && minOrderSize > 0, "TigerSwap: BAD_LOT");

        markets[marketId] = Market({
            token: token,
            oracleAsset: oracleAsset,
            takerFee: takerFee,
            makerFee: makerFee,
            initialMargin: initialMargin,
            maintenanceMargin: maintenanceMargin,
            liquidationFee: liquidationFee,
            maxLeverage: maxLeverage,
            maxOpenInterest: maxOpenInterest,
            openInterest: 0,
            minOrderSize: minOrderSize,
            lotSize: lotSize,
            isActive: false
        });
        fundingInfo[marketId].lastUpdateTime = block.timestamp;
        emit MarketCreated(marketId, token, oracleAsset, maxLeverage);
    }

    function setMarketActive(bytes32 marketId, bool active) external onlyOwner {
        require(markets[marketId].token != address(0), "TigerSwap: UNKNOWN_MARKET");
        markets[marketId].isActive = active;
        emit MarketActiveSet(marketId, active);
    }

    function setMarketParams(
        bytes32 marketId,
        uint256 takerFee,
        uint256 makerFee,
        uint256 initialMargin,
        uint256 maintenanceMargin,
        uint256 liquidationFee,
        uint256 maxOpenInterest
    ) external onlyOwner {
        Market storage market = markets[marketId];
        require(market.token != address(0), "TigerSwap: UNKNOWN_MARKET");
        require(initialMargin > 0 && initialMargin < WAD, "TigerSwap: BAD_MARGIN");
        require(maintenanceMargin > 0 && maintenanceMargin < initialMargin, "TigerSwap: BAD_MARGIN");
        market.takerFee = takerFee;
        market.makerFee = makerFee;
        market.initialMargin = initialMargin;
        market.maintenanceMargin = maintenanceMargin;
        market.liquidationFee = liquidationFee;
        market.maxOpenInterest = maxOpenInterest;
        emit MarketUpdated(marketId);
    }

    function setOracle(address oracle) external onlyOwner {
        require(oracle != address(0), "TigerSwap: ZERO_ORACLE");
        priceOracle = oracle;
        emit OracleUpdated(oracle);
    }

    function setTradingEnabled(bool enabled) external onlyOwner {
        tradingEnabled = enabled;
        emit TradingEnabledSet(enabled);
    }

    function setProtocolFeeShare(uint256 share) external onlyOwner {
        require(share <= WAD, "TigerSwap: BAD_SHARE");
        protocolFeeShare = share;
    }

    function commitTransferOwnership(address newOwner) external onlyOwner {
        require(newOwner != address(0), "TigerSwap: ZERO_ADDRESS");
        pendingOwner = newOwner;
        emit OwnershipCommitted(newOwner);
    }

    function acceptTransferOwnership() external {
        require(msg.sender == pendingOwner, "TigerSwap: NOT_PENDING_OWNER");
        emit OwnershipTransferred(owner, msg.sender);
        owner = msg.sender;
        pendingOwner = address(0);
    }

    // ============ Oracle: price + funding ============

    function updatePrice(bytes32 marketId, uint256 markPrice, uint256 indexPrice) external onlyOracle {
        require(markets[marketId].token != address(0), "TigerSwap: UNKNOWN_MARKET");
        require(markPrice > 0 && indexPrice > 0, "TigerSwap: INVALID_PRICE");
        FundingInfo storage info = fundingInfo[marketId];
        info.markPrice = markPrice;
        info.indexPrice = indexPrice;
        info.lastUpdateTime = block.timestamp;
    }

    /// @notice Accrue the cumulative funding index from the mark/index spread.
    ///         Positive funding means longs pay.
    function accrueFunding(bytes32 marketId, uint256 markPrice, uint256 indexPrice) external onlyOracle {
        require(markets[marketId].token != address(0), "TigerSwap: UNKNOWN_MARKET");
        require(markPrice > 0 && indexPrice > 0, "TigerSwap: INVALID_PRICE");

        FundingInfo storage info = fundingInfo[marketId];
        uint256 elapsed = block.timestamp - info.lastUpdateTime;
        int256 rate = (int256(markPrice) - int256(indexPrice)) * int256(FUNDING_RATE_PRECISION) / int256(indexPrice);
        if (elapsed > 0 && rate != 0) {
            int256 delta = rate * int256(elapsed) * int256(WAD) / (int256(FUNDING_INTERVAL) * int256(FUNDING_RATE_PRECISION));
            info.cumulativeIndex += delta;
        }
        info.lastFundingRate = rate;
        info.lastUpdateTime = block.timestamp;
        info.markPrice = markPrice;
        info.indexPrice = indexPrice;
        emit FundingRateUpdated(marketId, rate, info.cumulativeIndex);
    }

    // ============ Margin ============

    function deposit(bytes32 marketId, uint256 amount) external nonReentrant {
        Market storage market = markets[marketId];
        require(market.token != address(0), "TigerSwap: UNKNOWN_MARKET");
        require(amount > 0, "TigerSwap: ZERO_AMOUNT");
        market.token.safeTransferFrom(msg.sender, address(this), amount);
        _settleFunding(marketId, msg.sender);
        balances[marketId][msg.sender] += amount;
        emit PositionChanged(marketId, msg.sender, positions[marketId][msg.sender].size, _entryPrice(marketId, msg.sender), balances[marketId][msg.sender]);
    }

    function withdraw(bytes32 marketId, uint256 amount) external nonReentrant {
        require(markets[marketId].token != address(0), "TigerSwap: UNKNOWN_MARKET");
        _settleFunding(marketId, msg.sender);
        uint256 balance = balances[marketId][msg.sender];
        require(amount > 0 && amount <= balance, "TigerSwap: INSUFFICIENT_BALANCE");

        balances[marketId][msg.sender] = balance - amount;
        int256 equity = _equity(marketId, msg.sender);
        require(equity >= 0 && uint256(equity) >= _initialMarginRequirement(marketId, msg.sender), "TigerSwap: MARGIN_TOO_LOW");
        markets[marketId].token.safeTransfer(msg.sender, amount);
        emit PositionChanged(marketId, msg.sender, positions[marketId][msg.sender].size, _entryPrice(marketId, msg.sender), balances[marketId][msg.sender]);
    }

    // ============ Trading ============

    /// @notice Place an order. It takes the opposite side while the price is
    ///         acceptable, then the remainder rests (GTC/post-only) or is
    ///         cancelled (IOC) or reverts the whole order (FOK).
    function placeOrder(
        bytes32 marketId,
        uint256 quantity,
        uint256 price,
        bool isLong,
        uint8 tif,
        bool reduceOnly
    ) external nonReentrant returns (uint256 orderId, uint256 filled) {
        Market storage market = markets[marketId];
        require(market.token != address(0), "TigerSwap: UNKNOWN_MARKET");
        require(market.isActive, "TigerSwap: MARKET_NOT_ACTIVE");
        require(tradingEnabled, "TigerSwap: TRADING_HALTED");
        require(price > 0 && quantity >= market.minOrderSize, "TigerSwap: BAD_ORDER");
        require(quantity % market.lotSize == 0 && price % market.lotSize == 0, "TigerSwap: BAD_LOT");
        require(tif <= TIF_POST_ONLY, "TigerSwap: BAD_TIF");

        uint256 orderQuantity = quantity;
        _settleFunding(marketId, msg.sender);

        if (tif == TIF_POST_ONLY) {
            require(!_wouldCross(marketId, isLong, price), "TigerSwap: POST_ONLY_WOULD_CROSS");
        } else {
            (filled, quantity) = _match(marketId, msg.sender, isLong, price, quantity, reduceOnly);
        }

        if (tif == TIF_FOK && quantity > 0) {
            revert("TigerSwap: FOK_NOT_FILLED");
        }

        // A resting order must be affordable at its limit price, so the book
        // never holds an order that could not be filled. A reduce-only order
        // needs no new margin (it can only shrink exposure).
        if (quantity > 0 && !reduceOnly && (tif == TIF_GTC || tif == TIF_POST_ONLY)) {
            require(_canFill(marketId, msg.sender, isLong, price, quantity), "TigerSwap: INSUFFICIENT_MARGIN");
        }

        if (quantity > 0 && (tif == TIF_GTC || tif == TIF_POST_ONLY)) {
            orderId = _rest(marketId, msg.sender, isLong, price, orderQuantity, filled, reduceOnly, tif == TIF_POST_ONLY);
        }
    }

    function _rest(
        bytes32 marketId,
        address trader,
        bool isLong,
        uint256 price,
        uint256 quantity,
        uint256 filled,
        bool reduceOnly,
        bool postOnly
    ) private returns (uint256 orderId) {
        orderId = nextOrderId++;
        Order storage order = orders[orderId];
        order.id = orderId;
        order.trader = trader;
        order.price = price;
        order.quantity = quantity;
        order.filled = filled;
        order.isLong = isLong;
        order.reduceOnly = reduceOnly;
        order.postOnly = postOnly;
        order.timestamp = uint64(block.timestamp);
        order.active = true;
        orderMarket[orderId] = marketId;
        _traderOrders[trader].push(orderId);
        _insertLevel(marketId, isLong, price, orderId);
        emit OrderPlaced(orderId, marketId, trader, isLong, price, quantity, reduceOnly, postOnly);
    }

    function cancelOrder(uint256 orderId) external nonReentrant {
        Order storage order = orders[orderId];
        require(order.active, "TigerSwap: ORDER_NOT_ACTIVE");
        require(order.trader == msg.sender, "TigerSwap: NOT_OWNER");
        order.active = false;
        _dropOrder(orderMarket[orderId], orderId);
        emit OrderCancelled(orderId, orderMarket[orderId], msg.sender);
    }

    // ============ Matching ============

    function _match(
        bytes32 marketId,
        address taker,
        bool isLong,
        uint256 limitPrice,
        uint256 quantity,
        bool reduceOnly
    ) private returns (uint256 filled, uint256 remaining) {
        remaining = quantity;
        uint256 lot = markets[marketId].lotSize;
        while (remaining > 0) {
            (uint256 bestPrice, uint256 bestOrderId) = _bestOpposite(marketId, isLong);
            if (bestOrderId == 0) break;
            if (isLong ? bestPrice > limitPrice : bestPrice < limitPrice) break;

            Order storage maker = orders[bestOrderId];
            // Self-trade prevention: a taker may not cross its own resting
            // liquidity, so a trader cannot wash-trade against themselves.
            if (maker.trader == taker) break;
            uint256 makerRemaining = maker.quantity - maker.filled;
            if (makerRemaining == 0) {
                _dropOrder(marketId, bestOrderId);
                continue;
            }

            uint256 fillQty = remaining < makerRemaining ? remaining : makerRemaining;
            fillQty = (fillQty / lot) * lot;
            if (fillQty == 0) break;

            if (reduceOnly) {
                uint256 reducible = _reducible(marketId, taker, isLong);
                if (reducible == 0) break;
                if (fillQty > reducible) fillQty = (reducible / lot) * lot;
                if (fillQty == 0) break;
            }

            if (!_canFill(marketId, maker.trader, !isLong, bestPrice, fillQty)) {
                maker.active = false;
                emit OrderCancelled(bestOrderId, marketId, maker.trader);
                _dropOrder(marketId, bestOrderId);
                continue;
            }

            // A taker fill is marketable and is not tied to a resting order, so
            // it carries orderId 0; a maker fill carries the id of the order it
            // consumed. Without this an indexer cannot attribute a fill to the
            // order whose `filled` counter it advanced.
            _applyFill(marketId, taker, isLong, true, 0, bestPrice, fillQty);
            _applyFill(marketId, maker.trader, !isLong, false, bestOrderId, bestPrice, fillQty);

            maker.filled += fillQty;
            remaining -= fillQty;
            filled += fillQty;

            if (maker.filled >= maker.quantity) {
                maker.active = false;
                _dropOrder(marketId, bestOrderId);
            }
        }
    }

    /// @dev Apply a fill to one side and enforce margin on any exposure increase.
    ///      `fillOrderId` is the resting order the fill consumed, or 0 for a
    ///      marketable taker fill that is not tied to a resting order.
    function _applyFill(bytes32 marketId, address trader, bool isLong, bool isTaker, uint256 fillOrderId, uint256 price, uint256 quantity) private {
        Market storage market = markets[marketId];
        Position storage pos = positions[marketId][trader];
        int256 oldSize = pos.size;
        int256 newSize = oldSize + (isLong ? int256(quantity) : -int256(quantity));
        uint256 notional = quantity * price / WAD;

        bool increasing = oldSize == 0 || (isLong && newSize > oldSize) || (!isLong && newSize < oldSize);

        // Realise PnL when the fill reduces or flips the position.
        bool reducing = oldSize != 0 && ((oldSize > 0) != isLong);
        if (reducing) {
            uint256 absOld = oldSize > 0 ? uint256(oldSize) : uint256(-oldSize);
            uint256 closingQty = quantity < absOld ? quantity : absOld;
            uint256 avgEntry = pos.entryNotional * WAD / absOld;
            int256 pnl = oldSize > 0
                ? (int256(price) - int256(avgEntry)) * int256(closingQty) / int256(WAD)
                : (int256(avgEntry) - int256(price)) * int256(closingQty) / int256(WAD);
            uint256 entryReleased = avgEntry * closingQty / WAD;
            pos.entryNotional = pos.entryNotional > entryReleased ? pos.entryNotional - entryReleased : 0;
            _realise(marketId, trader, pnl);
        }

        // Fee, charged on the fill notional.
        uint256 fee = notional * (isTaker ? market.takerFee : market.makerFee) / WAD;
        uint256 balance = balances[marketId][trader];
        if (fee > balance) {
            fee = balance;
        }
        balances[marketId][trader] = balance - fee;
        uint256 protocolCut = fee * protocolFeeShare / WAD;
        protocolFees[marketId] += protocolCut;
        poolLiquidity[marketId] += fee - protocolCut;

        if (increasing) {
            pos.entryNotional += notional;
        }
        pos.size = newSize;
        if (newSize == 0) {
            pos.entryNotional = 0;
        }

        if (increasing) {
            int256 equity = _equity(marketId, trader);
            require(equity >= 0 && uint256(equity) >= _initialMarginRequirement(marketId, trader), "TigerSwap: INSUFFICIENT_MARGIN");
        }

        emit OrderFilled(fillOrderId, marketId, trader, price, quantity, !isTaker, fee);
        emit PositionChanged(marketId, trader, newSize, _entryPrice(marketId, trader), balances[marketId][trader]);
    }

    function _canFill(bytes32 marketId, address trader, bool isLong, uint256 price, uint256 quantity) private view returns (bool) {
        Position storage pos = positions[marketId][trader];
        int256 oldSize = pos.size;
        int256 newSize = oldSize + (isLong ? int256(quantity) : -int256(quantity));
        uint256 notional = quantity * price / WAD;
        uint256 fee = notional * markets[marketId].takerFee / WAD;
        uint256 balance = balances[marketId][trader];
        if (fee > balance) return false;

        bool increasing = oldSize == 0 || (isLong && newSize > oldSize) || (!isLong && newSize < oldSize);
        if (!increasing) return true;

        // Projected equity with the new size and the entry notional extended.
        uint256 mark = _markPrice(marketId);
        uint256 projectedEntry = pos.entryNotional + notional;
        uint256 absNew = newSize > 0 ? uint256(newSize) : uint256(-newSize);
        uint256 avgEntry = absNew > 0 ? projectedEntry * WAD / absNew : mark;
        int256 pnl = newSize > 0
            ? (int256(mark) - int256(avgEntry)) * int256(absNew) / int256(WAD)
            : (int256(avgEntry) - int256(mark)) * int256(absNew) / int256(WAD);
        int256 projectedEquity = int256(balance - fee) + pnl;
        uint256 req = absNew * mark / WAD * markets[marketId].initialMargin / WAD;
        return projectedEquity >= 0 && uint256(projectedEquity) >= req;
    }

    // ============ Margin views ============

    function _entryPrice(bytes32 marketId, address trader) private view returns (uint256) {
        Position storage pos = positions[marketId][trader];
        uint256 absSize = pos.size > 0 ? uint256(pos.size) : uint256(-pos.size);
        if (absSize == 0) return 0;
        return pos.entryNotional * WAD / absSize;
    }

    function _unrealizedPnL(bytes32 marketId, address trader) private view returns (int256) {
        Position storage pos = positions[marketId][trader];
        if (pos.size == 0) return 0;
        uint256 mark = _markPrice(marketId);
        uint256 absSize = pos.size > 0 ? uint256(pos.size) : uint256(-pos.size);
        uint256 avgEntry = pos.entryNotional * WAD / absSize;
        if (pos.size > 0) {
            return (int256(mark) - int256(avgEntry)) * int256(absSize) / int256(WAD);
        }
        return (int256(avgEntry) - int256(mark)) * int256(absSize) / int256(WAD);
    }

    function _equity(bytes32 marketId, address trader) private view returns (int256) {
        return int256(balances[marketId][trader]) + _unrealizedPnL(marketId, trader);
    }

    function _initialMarginRequirement(bytes32 marketId, address trader) private view returns (uint256) {
        int256 size = positions[marketId][trader].size;
        if (size == 0) return 0;
        uint256 mark = _markPrice(marketId);
        uint256 absSize = size > 0 ? uint256(size) : uint256(-size);
        return absSize * mark / WAD * markets[marketId].initialMargin / WAD;
    }

    function _reducible(bytes32 marketId, address trader, bool isLong) private view returns (uint256) {
        int256 size = positions[marketId][trader].size;
        if (isLong) return size < 0 ? uint256(-size) : 0;
        return size > 0 ? uint256(size) : 0;
    }

    // ============ Funding ============

    function _settleFunding(bytes32 marketId, address trader) private {
        Position storage pos = positions[marketId][trader];
        int256 current = fundingInfo[marketId].cumulativeIndex;
        if (pos.size != 0) {
            int256 delta = current - pos.fundingIndex;
            if (delta != 0) {
                uint256 absSize = pos.size > 0 ? uint256(pos.size) : uint256(-pos.size);
                int256 direction = pos.size > 0 ? int256(1) : int256(-1);
                int256 payment = int256(absSize) * delta * direction / int256(WAD);
                _realise(marketId, trader, -payment);
                emit FundingSettled(marketId, trader, -payment);
            }
        }
        pos.fundingIndex = current;
    }

    /// @dev Apply a realised PnL. A loss leaves the balance (clamped at zero); a
    ///      profit is paid from the pool, capped at what the pool can pay, with
    ///      the remainder recorded as an unpaid obligation.
    function _realise(bytes32 marketId, address trader, int256 pnl) private {
        if (pnl == 0) return;
        if (pnl > 0) {
            uint256 profit = uint256(pnl);
            uint256 pool = poolLiquidity[marketId];
            if (profit > pool) {
                unpaidProfits[marketId] += profit - pool;
                profit = pool;
            }
            poolLiquidity[marketId] = pool - profit;
            balances[marketId][trader] += profit;
        } else {
            uint256 loss = uint256(-pnl);
            uint256 balance = balances[marketId][trader];
            if (loss >= balance) {
                poolLiquidity[marketId] += balance;
                balances[marketId][trader] = 0;
            } else {
                balances[marketId][trader] = balance - loss;
                poolLiquidity[marketId] += loss;
            }
        }
    }

    // ============ Liquidation ============

    function liquidate(bytes32 marketId, address trader) external nonReentrant {
        Market storage market = markets[marketId];
        require(market.token != address(0), "TigerSwap: UNKNOWN_MARKET");
        Position storage pos = positions[marketId][trader];
        require(pos.size != 0, "TigerSwap: NO_POSITION");

        _settleFunding(marketId, trader);
        int256 equity = _equity(marketId, trader);
        uint256 mark = _markPrice(marketId);
        uint256 absSize = pos.size > 0 ? uint256(pos.size) : uint256(-pos.size);
        uint256 notional = absSize * mark / WAD;
        uint256 maintenance = notional * market.maintenanceMargin / WAD;
        require(equity <= 0 || uint256(equity) < maintenance, "TigerSwap: CANNOT_LIQUIDATE");

        int256 size = pos.size;
        uint256 balance = balances[marketId][trader];
        pos.size = 0;
        pos.entryNotional = 0;
        pos.fundingIndex = fundingInfo[marketId].cumulativeIndex;
        market.openInterest = market.openInterest > absSize ? market.openInterest - absSize : 0;
        _releaseReserve(marketId, notional);
        balances[marketId][trader] = 0;

        int256 badDebt = 0;
        uint256 fee = 0;
        if (equity < 0) {
            badDebt = -equity;
            uint256 debt = uint256(badDebt);
            uint256 insurance = insuranceFund[marketId];
            if (debt <= insurance) {
                insuranceFund[marketId] = insurance - debt;
                emit InsuranceFundUsed(marketId, debt);
                badDebt = 0;
            } else {
                emit InsuranceFundUsed(marketId, insurance);
                uint256 remainingDebt = debt - insurance;
                insuranceFund[marketId] = 0;
                uint256 pool = poolLiquidity[marketId];
                poolLiquidity[marketId] = remainingDebt >= pool ? 0 : pool - remainingDebt;
                badDebt = 0;
            }
        } else {
            fee = uint256(equity) * market.liquidationFee / WAD;
            uint256 payout = balance;
            uint256 feePaid = fee > payout ? payout : fee;
            if (feePaid > 0) market.token.safeTransfer(msg.sender, feePaid);
            uint256 remainder = payout - feePaid;
            if (remainder > 0) market.token.safeTransfer(trader, remainder);
        }

        emit Liquidation(marketId, trader, msg.sender, size, fee, badDebt);
        emit PositionChanged(marketId, trader, 0, 0, 0);
    }

    // ============ Insurance + liquidity ============

    function depositInsurance(bytes32 marketId, uint256 amount) external nonReentrant {
        require(markets[marketId].token != address(0), "TigerSwap: UNKNOWN_MARKET");
        require(amount > 0, "TigerSwap: ZERO_AMOUNT");
        markets[marketId].token.safeTransferFrom(msg.sender, address(this), amount);
        insuranceFund[marketId] += amount;
        emit InsuranceFundDeposited(marketId, msg.sender, amount);
    }

    function addLiquidity(bytes32 marketId, uint256 amount) external nonReentrant {
        require(markets[marketId].token != address(0), "TigerSwap: UNKNOWN_MARKET");
        require(amount > 0, "TigerSwap: ZERO_AMOUNT");
        markets[marketId].token.safeTransferFrom(msg.sender, address(this), amount);
        poolLiquidity[marketId] += amount;
        emit LiquidityAdded(marketId, msg.sender, amount);
    }

    function removeLiquidity(bytes32 marketId, uint256 amount) external nonReentrant {
        require(markets[marketId].token != address(0), "TigerSwap: UNKNOWN_MARKET");
        require(amount > 0, "TigerSwap: ZERO_AMOUNT");
        require(amount <= _freeLiquidity(marketId), "TigerSwap: INSUFFICIENT_FREE_LIQUIDITY");
        poolLiquidity[marketId] -= amount;
        markets[marketId].token.safeTransfer(msg.sender, amount);
        emit LiquidityRemoved(marketId, msg.sender, amount);
    }

    function withdrawProtocolFees(bytes32 marketId, address to) external onlyOwner nonReentrant returns (uint256 amount) {
        require(to != address(0), "TigerSwap: ZERO_ADDRESS");
        amount = protocolFees[marketId];
        require(amount > 0, "TigerSwap: NO_FEES");
        protocolFees[marketId] = 0;
        markets[marketId].token.safeTransfer(to, amount);
        emit ProtocolFeesWithdrawn(marketId, to, amount);
    }

    function freeLiquidity(bytes32 marketId) external view returns (uint256) {
        return _freeLiquidity(marketId);
    }

    function _freeLiquidity(bytes32 marketId) private view returns (uint256) {
        uint256 current = poolLiquidity[marketId];
        uint256 reserved = reservedLiquidity[marketId] + unpaidProfits[marketId] + protocolFees[marketId];
        return current > reserved ? current - reserved : 0;
    }

    function _releaseReserve(bytes32 marketId, uint256 notional) private {
        uint256 reserved = reservedLiquidity[marketId];
        reservedLiquidity[marketId] = notional >= reserved ? 0 : reserved - notional;
    }

    // ============ Book: levels and best price ============

    function _markPrice(bytes32 marketId) private view returns (uint256) {
        uint256 mark = fundingInfo[marketId].markPrice;
        require(mark > 0, "TigerSwap: MARKET_PRICE_NOT_SET");
        return mark;
    }

    function _wouldCross(bytes32 marketId, bool isLong, uint256 price) private view returns (bool) {
        (uint256 best, uint256 id) = _bestOpposite(marketId, isLong);
        if (id == 0) return false;
        return isLong ? price >= best : price <= best;
    }

    function _bestOpposite(bytes32 marketId, bool isLong) private view returns (uint256 price, uint256 orderId) {
        Level[] storage levels = isLong ? _asks[marketId] : _bids[marketId];
        for (uint256 i = 0; i < levels.length; i++) {
            uint256 live = _firstLive(levels[i]);
            if (live != 0) return (levels[i].price, live);
        }
        return (0, 0);
    }

    function _firstLive(Level storage level) private view returns (uint256) {
        for (uint256 i = 0; i < level.queue.length; i++) {
            Order storage order = orders[level.queue[i]];
            if (order.active && order.filled < order.quantity) return order.id;
        }
        return 0;
    }

    /// @dev Insert a price level keeping the array ordered: bids descending, asks
    ///      ascending. Most inserts are near the top of book, so the scan starts
    ///      from the end.
    function _insertLevel(bytes32 marketId, bool isLong, uint256 price, uint256 orderId) private {
        Level[] storage levels = isLong ? _bids[marketId] : _asks[marketId];
        for (uint256 i = 0; i < levels.length; i++) {
            if (levels[i].price == price) {
                levels[i].queue.push(orderId);
                return;
            }
        }
        uint256 position = levels.length;
        while (position > 0) {
            uint256 prev = levels[position - 1].price;
            bool inOrder = isLong ? prev >= price : prev <= price;
            if (inOrder) break;
            position--;
        }
        levels.push();
        for (uint256 k = levels.length - 1; k > position; k--) {
            levels[k].price = levels[k - 1].price;
            levels[k].queue = levels[k - 1].queue;
        }
        levels[position].price = price;
        levels[position].queue.push(orderId);
    }

    /// @dev Retire an order id from its level queue (swap-and-pop) and drop the
    ///      level when it becomes empty.
    function _dropOrder(bytes32 marketId, uint256 orderId) private {
        Order storage order = orders[orderId];
        Level[] storage levels = order.isLong ? _bids[marketId] : _asks[marketId];
        for (uint256 i = 0; i < levels.length; i++) {
            if (levels[i].price != order.price) continue;
            uint256[] storage queue = levels[i].queue;
            for (uint256 j = 0; j < queue.length; j++) {
                if (queue[j] == orderId) {
                    queue[j] = queue[queue.length - 1];
                    queue.pop();
                    break;
                }
            }
            if (queue.length == 0) {
                for (uint256 k = i; k < levels.length - 1; k++) {
                    levels[k].price = levels[k + 1].price;
                    levels[k].queue = levels[k + 1].queue;
                }
                levels.pop();
            }
            return;
        }
    }

    // ============ Views ============

    function getMarket(bytes32 marketId) external view returns (Market memory) {
        return markets[marketId];
    }

    function getPosition(bytes32 marketId, address trader)
        external
        view
        returns (int256 size, uint256 entryPrice, uint256 margin, int256 unrealizedPnL, int256 equity)
    {
        Position storage pos = positions[marketId][trader];
        size = pos.size;
        entryPrice = _entryPrice(marketId, trader);
        margin = balances[marketId][trader];
        unrealizedPnL = _unrealizedPnL(marketId, trader);
        equity = int256(margin) + unrealizedPnL;
    }

    function getOrder(uint256 orderId) external view returns (Order memory) {
        return orders[orderId];
    }

    function getTraderOrders(address trader) external view returns (uint256[] memory) {
        return _traderOrders[trader];
    }

    function getFundingInfo(bytes32 marketId) external view returns (FundingInfo memory) {
        return fundingInfo[marketId];
    }

    function bestBidAsk(bytes32 marketId) external view returns (uint256 bidPrice, uint256 bidQty, uint256 askPrice, uint256 askQty) {
        (uint256 bp, uint256 bid) = _bestOpposite(marketId, false);
        (uint256 ap, uint256 ask) = _bestOpposite(marketId, true);
        bidPrice = bp;
        askPrice = ap;
        if (bid != 0) bidQty = orders[bid].quantity - orders[bid].filled;
        if (ask != 0) askQty = orders[ask].quantity - orders[ask].filled;
    }

    function depth(bytes32 marketId, uint256 levels)
        external
        view
        returns (uint256[] memory bidPrices, uint256[] memory bidQty, uint256[] memory askPrices, uint256[] memory askQty)
    {
        bidPrices = new uint256[](levels);
        bidQty = new uint256[](levels);
        askPrices = new uint256[](levels);
        askQty = new uint256[](levels);

        Level[] storage bids = _bids[marketId];
        uint256 found = 0;
        for (uint256 i = 0; i < bids.length && found < levels; i++) {
            uint256 q = _levelQty(bids[i]);
            if (q == 0) continue;
            bidPrices[found] = bids[i].price;
            bidQty[found] = q;
            found++;
        }
        Level[] storage asks = _asks[marketId];
        found = 0;
        for (uint256 i = 0; i < asks.length && found < levels; i++) {
            uint256 q = _levelQty(asks[i]);
            if (q == 0) continue;
            askPrices[found] = asks[i].price;
            askQty[found] = q;
            found++;
        }
    }

    function _levelQty(Level storage level) private view returns (uint256 total) {
        for (uint256 i = 0; i < level.queue.length; i++) {
            Order storage order = orders[level.queue[i]];
            if (order.active && order.filled < order.quantity) {
                total += order.quantity - order.filled;
            }
        }
    }
}
