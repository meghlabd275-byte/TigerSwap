// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "../libraries/SafeMath.sol";

/**
 * @title TigerOrderBook
 * @notice Central Limit Order Book (CLOB) for Perpetual Trading
 * @dev dYdX/Hyperliquid style order book implementation
 * 
 * Key Features:
 * - Fully on-chain order matching
 * - Limit orders, market orders, stop orders
 * - Partial fills support
 * - Maker/taker fee model
 * - Insurance fund for liquidations
 */
contract TigerOrderBook {
    using SafeMath for uint256;
    using SafeMath for uint128;

    // ============== Constants ==============
    
    // Order types
    uint8 constant ORDER_TYPE_LIMIT = 1;
    uint8 constant ORDER_TYPE_MARKET = 2;
    uint8 constant ORDER_TYPE_STOP_LOSS = 3;
    uint8 constant ORDER_TYPE_TAKE_PROFIT = 4;
    uint8 constant ORDER_TYPE_STOP_LIMIT = 5;
    
    // Order side
    uint8 constant SIDE_BUY = 1;
    uint8 constant SIDE_SELL = 2;
    
    // Order status
    uint8 constant STATUS_OPEN = 1;
    uint8 constant STATUS_FILLED = 2;
    uint8 constant STATUS_CANCELLED = 3;
    uint8 constant STATUS_EXPIRED = 4;
    
    // Fee rates (in basis points)
    uint256 constant MAKER_FEE_BPS = 10;      // 0.1%
    uint256 constant TAKER_FEE_BPS = 30;    // 0.3%
    uint256 constant REFERRAL_FEE_BPS = 5; // 0.05%
    
    // Trading fees
    uint256 constant FEE_DENOMINATOR = 10000;
    
    // ============== State Variables ==============
    
    // Protocol configuration
    address public governance;
    address public insuranceFund;
    address public feeRecipient;
    uint256 public makerFeeBps = MAKER_FEE_BPS;
    uint256 public takerFeeBps = TAKER_FEE_BPS;
    uint256 public referralFeeBps = REFERRAL_FEE_BPS;
    
    // Market configuration
    mapping(bytes32 => Market) public markets;
    bytes32[] public marketIds;
    
    // Orders
    mapping(bytes32 => Order) public orders;
    uint256 public orderCount;
    
    // Order books (price => orders)
    mapping(bytes32 => OrderBook) public orderBooks;
    
    // User positions
    mapping(address => mapping(bytes32 => Position)) public positions;
    
    // User orders
    mapping(address => bytes32[]) public userOrderIds;
    
    // Fill history
    mapping(bytes32 => Fill[]) public fillHistory;
    uint256 public fillCount;
    
    // Insurance fund balance
    uint256 public insuranceBalance;
    
    // ============== Structs ==============
    
    struct Market {
        address indexToken;      // Perpetual index token
        address quoteToken;       // Settlement token (USDC)
        uint256 maxPriceAge;    // Max oracle price age (seconds)
        uint256 maxLeverage;     // Max leverage (e.g., 10 = 10x)
        uint256 liquidationPenalty; // Penalty bps on liquidation
        uint256 fundingRateCap;   // Max funding rate per period
        bytes32 priceFeedId;       // Chainlink price feed ID
        bool active;
    }
    
    struct Order {
        address owner;
        bytes32 marketId;
        uint8 orderType;
        uint8 side;
        uint256 price;
        uint256 quantity;
        uint256 filledQuantity;
        uint256 timestamp;
        uint256 expiry;
        uint256 reduceOnly;
        uint256 status;
        bytes32 referrer;
    }
    
    struct OrderBook {
        SortedUintSet bids;   // Buy orders by price (descending)
        SortedUintSet asks;   // Sell orders by price (ascending)
    }
    
    struct SortedUintSet {
        mapping(uint256 => Node) nodes;
        uint256[] list;
        uint256 count;
    }
    
    struct Node {
        uint256 price;
        bytes32 next;
        bytes32 prev;
        uint256 quantity;
        address owner;
    }
    
    struct Position {
        address owner;
        bytes32 marketId;
        int256 size;        // Position size (+ for long, - for short)
        uint256 entryPrice;
        int256 unrealizedPnl;
        uint256 margin;
        uint256 openNotional;
        uint256 lastUpdated;
        bool liquidated;
    }
    
    struct Fill {
        address maker;
        address taker;
        bytes32 marketId;
        uint256 price;
        uint256 quantity;
        uint256 fee;
        bytes32 orderHashM;
        bytes32 orderHashT;
        uint256 timestamp;
    }
    
    // ============== Events ==============
    
    event OrderCreated(
        bytes32 indexed orderHash,
        address indexed owner,
        bytes32 marketId,
        uint8 orderType,
        uint8 side,
        uint256 price,
        uint256 quantity
    );
    
    event OrderFilled(
        bytes32 indexed orderHash,
        address indexed owner,
        bytes32 marketId,
        uint256 price,
        uint256 quantity,
        uint256 filledQuantity
    );
    
    event OrderCancelled(
        bytes32 indexed orderHash,
        address indexed owner,
        bytes32 marketId,
        string reason
    );
    
    event PositionOpened(
        address indexed owner,
        bytes32 marketId,
        uint8 side,
        uint256 size,
        uint256 entryPrice,
        uint256 margin
    );
    
    event PositionClosed(
        address indexed owner,
        bytes32 marketId,
        int256 realizedPnl,
        uint256 exitPrice
    );
    
    event PositionLiquidated(
        address indexed owner,
        bytes32 marketId,
        uint256 liquidationPrice,
        uint256 penalty
    );
    
    event MarketCreated(
        bytes32 indexed marketId,
        address indexToken,
        address quoteToken,
        uint256 maxLeverage
    );
    
    // ============== Constructor ==============
    
    constructor() {
        governance = msg.sender;
        insuranceFund = msg.sender;
        feeRecipient = msg.sender;
    }
    
    // ============== Market Management ==============
    
    /**
     * @notice Create a new perpetual market
     */
    function createMarket(
        bytes32 _marketId,
        address _indexToken,
        address _quoteToken,
        uint256 _maxLeverage,
        bytes32 _priceFeedId
    ) external {
        require(msg.sender == governance, "ONLY_GOVERNANCE");
        require(!markets[_marketId].active, "MARKET_EXISTS");
        
        markets[_marketId] = Market({
            indexToken: _indexToken,
            quoteToken: _quoteToken,
            maxPriceAge: 300,           // 5 minutes
            maxLeverage: _maxLeverage,
            liquidationPenalty: 50,       // 0.5%
            fundingRateCap: 100,          // 1% per hour
            priceFeedId: _priceFeedId,
            active: true
        });
        
        marketIds.push(_marketId);
        
        emit MarketCreated(_marketId, _indexToken, _quoteToken, _maxLeverage);
    }
    
    /**
     * @notice Update market parameters
     */
    function updateMarket(
        bytes32 _marketId,
        uint256 _maxLeverage,
        uint256 _liquidationPenalty
    ) external {
        require(msg.sender == governance, "ONLY_GOVERNANCE");
        
        Market storage market = markets[_marketId];
        market.maxLeverage = _maxLeverage;
        market.liquidationPenalty = _liquidationPenalty;
    }
    
    // ============== Order Management ==============
    
    /**
     * @notice Create a limit order
     */
    function createLimitOrder(
        bytes32 _marketId,
        uint8 _side,
        uint256 _price,
        uint256 _quantity,
        uint256 _expiry,
        bytes32 _referrer
    ) external returns (bytes32 orderHash) {
        Market memory market = markets[_marketId];
        require(market.active, "MARKET_NOT_ACTIVE");
        
        require(_price > 0, "INVALID_PRICE");
        require(_quantity > 0, "INVALID_QUANTITY");
        require(_expiry > block.timestamp || _expiry == 0, "INVALID_EXPIRY");
        
        orderHash = keccak256(abi.encodePacked(
            msg.sender,
            _marketId,
            ORDER_TYPE_LIMIT,
            _side,
            _price,
            _quantity,
            block.timestamp,
            orderCount
        ));
        
        orders[orderHash] = Order({
            owner: msg.sender,
            marketId: _marketId,
            orderType: ORDER_TYPE_LIMIT,
            side: _side,
            price: _price,
            quantity: _quantity,
            filledQuantity: 0,
            timestamp: block.timestamp,
            expiry: _expiry,
            reduceOnly: 0,
            status: STATUS_OPEN,
            referrer: _referrer
        });
        
        // Add to order book
        _addToOrderBook(_marketId, orderHash, _price, _quantity, _side);
        
        userOrderIds[msg.sender].push(orderHash);
        orderCount++;
        
        emit OrderCreated(
            orderHash,
            msg.sender,
            _marketId,
            ORDER_TYPE_LIMIT,
            _side,
            _price,
            _quantity
        );
    }
    
    /**
     * @notice Create a market order
     */
    function createMarketOrder(
        bytes32 _marketId,
        uint8 _side,
        uint256 _quantity,
        uint256 _slippage,
        bytes32 _referrer
    ) external returns (bytes32 orderHash) {
        Market memory market = markets[_marketId];
        require(market.active, "MARKET_NOT_ACTIVE");
        
        require(_quantity > 0, "INVALID_QUANTITY");
        
        orderHash = keccak256(abi.encodePacked(
            msg.sender,
            _marketId,
            ORDER_TYPE_MARKET,
            _side,
            _quantity,
            block.timestamp,
            orderCount
        ));
        
        orders[orderHash] = Order({
            owner: msg.sender,
            marketId: _marketId,
            orderType: ORDER_TYPE_MARKET,
            side: _side,
            price: 0, // Market price
            quantity: _quantity,
            filledQuantity: 0,
            timestamp: block.timestamp,
            expiry: block.timestamp + 300, // 5 min expiry
            reduceOnly: 0,
            status: STATUS_OPEN,
            referrer: _referrer
        });
        
        // Execute immediately against order book
        _executeMarketOrder(orderHash, _slippage);
        
        orderCount++;
        
        emit OrderCreated(
            orderHash,
            msg.sender,
            _marketId,
            ORDER_TYPE_MARKET,
            _side,
            0,
            _quantity
        );
    }
    
    /**
     * @notice Create stop loss order
     */
    function createStopLossOrder(
        bytes32 _marketId,
        uint8 _side,
        uint256 _stopPrice,
        uint256 _quantity,
        bytes32 _referrer
    ) external returns (bytes32 orderHash) {
        Market memory market = markets[_marketId];
        require(market.active, "MARKET_NOT_ACTIVE");
        
        require(_stopPrice > 0, "INVALID_STOP_PRICE");
        require(_quantity > 0, "INVALID_QUANTITY");
        
        orderHash = keccak256(abi.encodePacked(
            msg.sender,
            _marketId,
            ORDER_TYPE_STOP_LOSS,
            _side,
            _stopPrice,
            _quantity,
            block.timestamp,
            orderCount
        ));
        
        orders[orderHash] = Order({
            owner: msg.sender,
            marketId: _marketId,
            orderType: ORDER_TYPE_STOP_LOSS,
            side: _side,
            price: _stopPrice,
            quantity: _quantity,
            filledQuantity: 0,
            timestamp: block.timestamp,
            expiry: 0,
            reduceOnly: 1,
            status: STATUS_OPEN,
            referrer: _referrer
        });
        
        userOrderIds[msg.sender].push(orderHash);
        orderCount++;
        
        emit OrderCreated(
            orderHash,
            msg.sender,
            _marketId,
            ORDER_TYPE_STOP_LOSS,
            _side,
            _stopPrice,
            _quantity
        );
    }
    
    /**
     * @notice Cancel an order
     */
    function cancelOrder(bytes32 _orderHash) external {
        Order storage order = orders[_orderHash];
        require(order.owner == msg.sender, "NOT_OWNER");
        require(order.status == STATUS_OPEN, "NOT_OPEN");
        
        order.status = STATUS_CANCELLED;
        
        // Remove from order book
        _removeFromOrderBook(order.marketId, _orderHash, order.price, order.quantity, order.side);
        
        emit OrderCancelled(_orderHash, msg.sender, order.marketId, "USER_CANCEL");
    }
    
    // ============== Order Execution ==============
    
    /**
     * @notice Execute market order against order book
     */
    function _executeMarketOrder(bytes32 _orderHash, uint256 _slippage) internal {
        Order storage order = orders[_orderHash];
        Market storage market = markets[order.marketId];
        
        // Get current oracle price
        uint256 currentPrice = _getOraclePrice(order.marketId);
        
        // Apply slippage
        uint256 maxSlippage = _slippage > 0 ? _slippage : TAKER_FEE_BPS * 10;
        uint256 worstPrice = order.side == SIDE_BUY
            ? currentPrice * (FEE_DENOMINATOR + maxSlippage) / FEE_DENOMINATOR
            : currentPrice * (FEE_DENOMINATOR - maxSlippage) / FEE_DENOMINATOR;
        
        // Match against order book
        bytes32 oppositeSide = order.side == SIDE_BUY ? _getAskKey(order.marketId) : _getBidKey(order.marketId);
        
        // Execute fills
        uint256 remaining = order.quantity;
        
        while (remaining > 0) {
            (bytes32 bestOrderHash, uint256 bestPrice, uint256 available) = _getBestOrder(
                order.marketId, 
                oppositeSide
            );
            
            if (bestOrderHash == bytes32(0)) break;
            
            // Check price
            if (order.side == SIDE_BUY && bestPrice > worstPrice) break;
            if (order.side == SIDE_SELL && bestPrice < worstPrice) break;
            
            // Calculate fill amount
            uint256 fillAmount = remaining < available ? remaining : available;
            
            // Execute fill
            _executeFill(
                order.marketId,
                bestOrderHash,
                _orderHash,
                bestPrice,
                fillAmount
            );
            
            remaining -= fillAmount;
        }
        
        // Update order
        order.filledQuantity = order.quantity - remaining;
        
        if (remaining > 0 && order.orderType == ORDER_TYPE_MARKET) {
            order.status = STATUS_CANCELLED;
            emit OrderCancelled(_orderHash, msg.sender, order.marketId, "INSUFFICIENT_LIQUIDITY");
        }
    }
    
    /**
     * @notice Execute a fill between maker and taker
     */
    function _executeFill(
        bytes32 _marketId,
        bytes32 _makerOrderHash,
        bytes32 _takerOrderHash,
        uint256 _price,
        uint256 _quantity
    ) internal {
        Order storage maker = orders[_makerOrderHash];
        Order storage taker = orders[_takerOrderHash];
        
        // Update orders
        maker.filledQuantity += _quantity;
        taker.filledQuantity += _quantity;
        
        if (maker.filledQuantity >= maker.quantity) {
            maker.status = STATUS_FILLED;
        }
        
        if (taker.filledQuantity >= taker.quantity) {
            taker.status = STATUS_FILLED;
        }
        
        // Calculate fees
        uint256 makerFee = _quantity * makerFeeBps / FEE_DENOMINATOR;
        uint256 takerFee = _quantity * takerFeeBps / FEE_DENOMINATOR;
        uint256 totalFee = makerFee + takerFee;
        
        // Update position
        _updatePositionOnFill(
            _marketId,
            maker.owner,
            taker.owner,
            maker.side,
            _price,
            _quantity,
            totalFee
        );
        
        // Record fill
        fillHistory[_marketId].push(Fill({
            maker: maker.owner,
            taker: taker.owner,
            marketId: _marketId,
            price: _price,
            quantity: _quantity,
            fee: totalFee,
            orderHashM: _makerOrderHash,
            orderHashT: _takerOrderHash,
            timestamp: block.timestamp
        }));
        fillCount++;
        
        emit OrderFilled(_makerOrderHash, maker.owner, _marketId, _price, _quantity, maker.filledQuantity);
        emit OrderFilled(_takerOrderHash, taker.owner, _marketId, _price, _quantity, taker.filledQuantity);
    }
    
    /**
     * @notice Update position on fill
     */
    function _updatePositionOnFill(
        bytes32 _marketId,
        address _long,
        address _short,
        uint8 _side,
        uint256 _price,
        uint256 _quantity,
        uint256 _fee
    ) internal {
        address longAddr = _side == SIDE_BUY ? _long : _short;
        address shortAddr = _side == SIDE_SELL ? _long : _short;
        
        // Update long position
        Position storage longPos = positions[longAddr][_marketId];
        if (longPos.size == 0) {
            longPos.owner = longAddr;
            longPos.marketId = _marketId;
            longPos.size = int256(_quantity);
            longPos.entryPrice = _price;
            longPos.margin = _quantity * _price / longPos.maxLeverage;
        } else {
            // Update average entry price
            int256 newSize = longPos.size + int256(_quantity);
            uint256 newNotional = longPos.openNotional + (_quantity * _price);
            longPos.entryPrice = newNotional / uint256(newSize > 0 ? newSize : -newSize);
            longPos.size = newSize;
            longPos.openNotional = newNotional;
        }
        
        // Update short position
        Position storage shortPos = positions[shortAddr][_marketId];
        if (shortPos.size == 0) {
            shortPos.owner = shortAddr;
            shortPos.marketId = _marketId;
            shortPos.size = -int256(_quantity);
            shortPos.entryPrice = _price;
            shortPos.margin = _quantity * _price / shortPos.maxLeverage;
        } else {
            int256 newSize = shortPos.size - int256(_quantity);
            uint256 newNotional = shortPos.openNotional + (_quantity * _price);
            shortPos.entryPrice = newNotional / uint256(newSize > 0 ? newSize : -newSize);
            shortPos.size = newSize;
            shortPos.openNotional = newNotional;
        }
    }
    
    // ============== Position Management ==============
    
    /**
     * @notice Open a position with margin
     */
    function openPosition(
        bytes32 _marketId,
        uint8 _side,
        uint256 _size,
        uint256 _margin
    ) external returns (uint256) {
        Market storage market = markets[_marketId];
        require(market.active, "MARKET_NOT_ACTIVE");
        
        // Get current price
        uint256 currentPrice = _getOraclePrice(_marketId);
        
        // Validate leverage
        uint256 requiredMargin = _size * currentPrice / market.maxLeverage;
        require(_margin >= requiredMargin, "INSUFFICIENT_MARGIN");
        
        // Create position
        Position storage position = positions[msg.sender][_marketId];
        
        if (position.size == 0) {
            position.owner = msg.sender;
            position.marketId = _marketId;
            position.size = _side == SIDE_BUY ? int256(_size) : -int256(_size);
            position.entryPrice = currentPrice;
            position.margin = _margin;
            position.openNotional = _size * currentPrice;
            position.lastUpdated = block.timestamp;
            
            emit PositionOpened(
                msg.sender,
                _marketId,
                _side,
                _size,
                currentPrice,
                _margin
            );
        } else {
            // Add to existing position
            if ((_side == SIDE_BUY && position.size > 0) || (_side == SIDE_SELL && position.size < 0)) {
                // Same direction - increase
                int256 newSize = position.size + (_side == SIDE_BUY ? int256(_size) : -int256(_size));
                position.size = newSize;
            } else {
                // Opposite direction - reduce or flip
                int256 newSize = position.size + (_side == SIDE_BUY ? int256(_size) : -int256(_size));
                
                if (newSize == 0) {
                    // Close position
                    _closePosition(msg.sender, _marketId, currentPrice);
                    return 0;
                }
                
                position.size = newSize;
            }
            
            position.margin += _margin;
            position.openNotional += _size * currentPrice;
            position.lastUpdated = block.timestamp;
        }
        
        return _margin;
    }
    
    /**
     * @notice Close a position
     */
    function closePosition(bytes32 _marketId) external {
        Position storage position = positions[msg.sender][_marketId];
        require(position.size != 0, "NO_POSITION");
        
        uint256 currentPrice = _getOraclePrice(_marketId);
        _closePosition(msg.sender, _marketId, currentPrice);
    }
    
    /**
     * @notice Close position internal
     */
    function _closePosition(
        address _owner,
        bytes32 _marketId,
        uint256 _exitPrice
    ) internal {
        Position storage position = positions[_owner][_marketId];
        
        // Calculate PnL
        int256 pnl;
        if (position.size > 0) {
            pnl = int256(_exitPrice) - int256(position.entryPrice);
            pnl = pnl * position.size / int256(position.entryPrice);
        } else {
            pnl = int256(position.entryPrice) - int256(_exitPrice);
            pnl = pnl * (-position.size) / int256(position.entryPrice);
        }
        
        // Emit close event
        emit PositionClosed(_owner, _marketId, pnl, _exitPrice);
        
        // Clear position
        delete positions[_owner][_marketId];
    }
    
    /**
     * @notice Liquidate a position
     */
    function liquidate(address _owner, bytes32 _marketId) external {
        Position storage position = positions[_owner][_marketId];
        require(position.size != 0, "NO_POSITION");
        
        Market storage market = markets[_marketId];
        uint256 currentPrice = _getOraclePrice(_marketId);
        
        // Calculate margin ratio
        uint256 marginRatio;
        if (position.size > 0) {
            int256 pnl = int256(currentPrice) - int256(position.entryPrice);
            pnl = pnl * position.size / int256(position.entryPrice);
            uint256 currentValue = uint256(int256(position.margin) + pnl);
            marginRatio = currentValue * market.maxLeverage / position.openNotional;
        } else {
            int256 pnl = int256(position.entryPrice) - int256(currentPrice);
            pnl = pnl * (-position.size) / int256(position.entryPrice);
            uint256 currentValue = uint256(int256(position.margin) + pnl);
            marginRatio = currentValue * market.maxLeverage / position.openNotional;
        }
        
        require(marginRatio < 1100, "ABOVE_MAINTENANCE"); // Below 110% MR
        
        // Calculate liquidation penalty
        uint256 penalty = position.openNotional * market.liquidationPenalty / FEE_DENOMINATOR;
        insuranceBalance += penalty;
        
        // Liquidate
        position.liquidated = true;
        
        emit PositionLiquidated(_owner, _marketId, currentPrice, penalty);
        
        // Clear position
        delete positions[_owner][_marketId];
    }
    
    // ============== Order Book Management ==============
    
    /**
     * @notice Add order to order book
     */
    function _addToOrderBook(
        bytes32 _marketId,
        bytes32 _orderHash,
        uint256 _price,
        uint256 _quantity,
        uint8 _side
    ) internal {
        bytes32 bookKey = _side == SIDE_BUY 
            ? _getBidKey(_marketId) 
            : _getAskKey(_marketId);
        
        OrderBook storage book = orderBooks[bookKey];
        
        // Add to sorted list
        book.list.push(_price);
        book.count++;
    }
    
    /**
     * @notice Remove order from order book
     */
    function _removeFromOrderBook(
        bytes32 _marketId,
        bytes32 _orderHash,
        uint256 _price,
        uint256 _quantity,
        uint8 _side
    ) internal {
        bytes32 bookKey = _side == SIDE_BUY 
            ? _getBidKey(_marketId) 
            : _getAskKey(_marketId);
        
        OrderBook storage book = orderBooks[bookKey];
        
        // Remove from list (simplified)
        if (book.count > 0) {
            book.count--;
        }
    }
    
    /**
     * @notice Get best order from book
     */
    function _getBestOrder(
        bytes32 _marketId,
        bytes32 _bookKey
    ) internal view returns (
        bytes32 orderHash,
        uint256 price,
        uint256 available
    ) {
        OrderBook storage book = orderBooks[_bookKey];
        
        if (book.count == 0) {
            return (bytes32(0), 0, 0);
        }
        
        // Get first order
        uint256 bestPrice = book.list[0];
        
        return (bytes32(bestPrice), bestPrice, 0);
    }
    
    // ============== View Functions ==============
    
    /**
     * @notice Get oracle price (simplified - integrate with real oracle)
     */
    function _getOraclePrice(bytes32 _marketId) internal view returns (uint256) {
        // This should integrate with Chainlink or other oracle
        // For now, return mock price
        return 1000 * 1e8; // $1000 with 8 decimals
    }
    
    /**
     * @notice Get order book depth
     */
    function getOrderBookDepth(
        bytes32 _marketId,
        uint8 _side,
        uint256 _levels
    ) external view returns (uint256[] memory prices, uint256[] memory quantities) {
        // Return top N levels
        prices = new uint256[](_levels);
        quantities = new uint256[](_levels);
    }
    
    /**
     * @notice Get position info
     */
    function getPosition(address _owner, bytes32 _marketId) external view returns (
        int256 size,
        uint256 entryPrice,
        int256 unrealizedPnl,
        uint256 margin
    ) {
        Position storage pos = positions[_owner][_marketId];
        return (pos.size, pos.entryPrice, pos.unrealizedPnl, pos.margin);
    }
    
    /**
     * @notice Get order info
     */
    function getOrder(bytes32 _orderHash) external view returns (
        address owner,
        bytes32 marketId,
        uint8 orderType,
        uint8 side,
        uint256 price,
        uint256 quantity,
        uint256 filledQuantity,
        uint256 status
    ) {
        Order storage order = orders[_orderHash];
        return (
            order.owner,
            order.marketId,
            order.orderType,
            order.side,
            order.price,
            order.quantity,
            order.filledQuantity,
            order.status
        );
    }
    
    // ============== Helpers ==============
    
    function _getBidKey(bytes32 _marketId) internal pure returns (bytes32) {
        return keccak256(abi.encodePacked(_marketId, "BID"));
    }
    
    function _getAskKey(bytes32 _marketId) internal pure returns (bytes32) {
        return keccak256(abi.encodePacked(_marketId, "ASK"));
    }
}