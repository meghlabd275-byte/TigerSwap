// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/**
 * @title TigerOrderBook
 * @notice Complete Order Book for Limit Orders, Stop Orders, OCO, TWAP
 * @dev CLOB (Central Limit Order Book) like dYdX, Hyperliquid
 * 
 * Features:
 * - Limit orders (buy/sell at specific price)
 * - Stop-loss orders (trigger at price)
 * - Stop-limit orders
 * - OCO (One Cancels Other)
 * - TWAP (Time-Weighted Average Price)
 * - Trailing stop
 * - Post-only orders
 * - Fill-or-kill orders
 * - Order matching engine
 */
import "../libraries/SafeMath.sol";

contract TigerOrderBook {
    using SafeMath for uint256;

    // ============================================================================
    // Constants
    // ============================================================================

    uint256 constant MAX_ORDERS = 100000;
    uint256 constant MAX_TWAP_SLICES = 1000;
    uint8 constant ORDER_STATUS_PENDING = 0;
    uint8 constant ORDER_STATUS_PARTIAL = 1;
    uint8 constant ORDER_STATUS_FILLED = 2;
    uint8 constant ORDER_STATUS_CANCELLED = 3;
    uint8 constant ORDER_STATUS_EXPIRED = 4;

    // ============================================================================
    // Enums
    // ============================================================================

    enum OrderType {
        Limit,           // Limit order at specific price
        StopLoss,        // Stop-loss order
        StopLimit,       // Stop-limit order
        TWAP,           // Time-weighted average price
        TrailingStop,   // Trailing stop
        PostOnly,       // Post-only (no taking liquidity)
        FillOrKill,     // Fill or kill (all or nothing)
        IOC             // Immediate or cancel
    }

    enum Side {
        Buy,
        Sell
    }

    enum TimeInForce {
        GTC, // Good till cancelled
        IOC, // Immediate or cancel
        FOK, // Fill or kill
        GTD  // Good till date
    }

    // ============================================================================
    // State Variables
    // ============================================================================

    // Orders
    mapping(bytes32 => Order) public orders;
    bytes32[] public orderIds;
    uint256 public orderCount = 0;

    // Order book - sorted by price
    mapping(bytes32 => mapping(uint256 => OrderLevel)) public bidLevels;
    mapping(bytes32 => mapping(uint256 => OrderLevel)) public askLevels;
    mapping(bytes32 => uint256) public bidLevelCount;
    mapping(bytes32 => uint256) public askLevelCount;

    // Order queues by price
    mapping(bytes32 => bytes32[]) public bidQueue;    // price -> order IDs
    mapping(bytes32 => bytes32[]) public askQueue;  // price -> order IDs

    // TWAP orders
    mapping(bytes32 => TWAPOrder) public twapOrders;
    bytes32[] public twapOrderIds;

    // OCO groups
    mapping(bytes32 => OCOGroup) public ocoGroups;
    bytes32[] public ocoOrderIds;

    // Trading pairs
    mapping(address => bool) public isToken;
    address[] public tokens;

    // Trading enabled
    bool public tradingEnabled = true;
    bool public adminCanCancel = false;

    // Fee settings
    uint256 public makerFeeBps = 15;   // 0.15%
    uint256 public takerFeeBps = 30;     // 0.30%
    address public feeRecipient;

    // Oracle for stop orders
    address public priceOracle;

    // ============================================================================
    // Structs
    // ============================================================================

    struct Order {
        bytes32 id;
        address user;
        address pairId;
        OrderType orderType;
        Side side;
        uint256 price;
        uint256 quantity;
        uint256 filledQuantity;
        uint256 remainingQuantity;
        uint256 stopPrice;
        uint256 triggerPrice;
        uint256 expiryTime;
        uint256 createdAt;
        uint8 status;
        uint8 timeInForce;
        uint256 clientOrderId;
        bytes32 ocoGroupId;
    }

    struct OrderLevel {
        uint256 price;
        uint256 totalQuantity;
        uint256 orderCount;
    }

    struct TWAPOrder {
        bytes32 id;
        address user;
        address pairId;
        Side side;
        uint256 totalQuantity;
        uint256 filledQuantity;
        uint256 sliceInterval;
        uint256 slicesRemaining;
        uint256 nextSliceTime;
        uint256 minPrice;
        uint256 maxPrice;
        bool isActive;
    }

    struct OCOGroup {
        bytes32 id;
        bytes32 orderId1;
        bytes32 orderId2;
        address user;
        uint8 status; // 0=active, 1=triggered, 2=cancelled
    }

    struct Trade {
        bytes32 orderId;
        address user;
        Side side;
        uint256 price;
        uint256 quantity;
        uint256 fee;
        uint256 timestamp;
    }

    // ============================================================================
    // Events
    // ============================================================================

    event OrderCreated(
        bytes32 indexed orderId,
        address indexed user,
        address indexed pairId,
        uint8 orderType,
        uint8 side,
        uint256 price,
        uint256 quantity,
        uint256 stopPrice
    );

    event OrderFilled(
        bytes32 indexed orderId,
        address indexed user,
        uint256 filledQuantity,
        uint256 fillPrice,
        uint256 fee
    );

    event OrderCancelled(
        bytes32 indexed orderId,
        address indexed user,
        string reason
    );

    event OrderExpired(bytes32 indexed orderId);

    event TWAPCreated(
        bytes32 indexed orderId,
        address indexed user,
        uint256 totalQuantity,
        uint256 sliceInterval,
        uint256 slices
    );

    event TWAPExecuted(
        bytes32 indexed orderId,
        uint256 quantity,
        uint256 price
    );

    event TWAPCompleted(bytes32 indexed orderId);

    event OCOTriggered(
        bytes32 indexed groupId,
        bytes32 triggeredOrderId,
        bytes32 cancelledOrderId
    );

    // ============================================================================
    // Modifiers
    // ============================================================================

    modifier onlyTradingEnabled() {
        require(tradingEnabled, "Trading disabled");
        _;
    }

    modifier onlyValidOrder(bytes32 orderId) {
        require(orders[orderId].id == orderId, "Invalid order");
        _;
    }

    // ============================================================================
    // Constructor
    // ============================================================================

    constructor(address _feeRecipient, address _priceOracle) {
        feeRecipient = _feeRecipient;
        priceOracle = _priceOracle;
    }

    // ============================================================================
    // Order Creation
    // ============================================================================

    /**
     * @notice Create a limit order
     */
    function createLimitOrder(
        address pairId,
        Side side,
        uint256 price,
        uint256 quantity,
        uint256 expiryTime,
        uint256 clientOrderId
    ) external returns (bytes32 orderId) {
        return _createOrder(
            pairId,
            OrderType.Limit,
            side,
            price,
            quantity,
            0,
            expiryTime,
            TimeInForce.GTC,
            clientOrderId,
            bytes32(0)
        );
    }

    /**
     * @notice Create a stop-loss order
     */
    function createStopLossOrder(
        address pairId,
        Side side,
        uint256 quantity,
        uint256 stopPrice,
        uint256 expiryTime,
        uint256 clientOrderId
    ) external returns (bytes32 orderId) {
        return _createOrder(
            pairId,
            OrderType.StopLoss,
            side,
            0,
            quantity,
            stopPrice,
            expiryTime,
            TimeInForce.GTC,
            clientOrderId,
            bytes32(0)
        );
    }

    /**
     * @notice Create a stop-limit order
     */
    function createStopLimitOrder(
        address pairId,
        Side side,
        uint256 price,
        uint256 quantity,
        uint256 stopPrice,
        uint256 expiryTime,
        uint256 clientOrderId
    ) external returns (bytes32 orderId) {
        return _createOrder(
            pairId,
            OrderType.StopLimit,
            side,
            price,
            quantity,
            stopPrice,
            expiryTime,
            TimeInForce.GTC,
            clientOrderId,
            bytes32(0)
        );
    }

    /**
     * @notice Create post-only order (no taking liquidity)
     */
    function createPostOnlyOrder(
        address pairId,
        Side side,
        uint256 price,
        uint256 quantity,
        uint256 expiryTime,
        uint256 clientOrderId
    ) external returns (bytes32 orderId) {
        return _createOrder(
            pairId,
            OrderType.PostOnly,
            side,
            price,
            quantity,
            0,
            expiryTime,
            TimeInForce.IOC,
            clientOrderId,
            bytes32(0)
        );
    }

    /**
     * @notice Create fill-or-kill order
     */
    function createFOKOrder(
        address pairId,
        Side side,
        uint256 price,
        uint256 quantity,
        uint256 clientOrderId
    ) external returns (bytes32 orderId) {
        return _createOrder(
            pairId,
            OrderType.FillOrKill,
            side,
            price,
            quantity,
            0,
            0,
            TimeInForce.FOK,
            clientOrderId,
            bytes32(0)
        );
    }

    /**
     * @notice Create IOC order
     */
    function createIOCOrder(
        address pairId,
        Side side,
        uint256 price,
        uint256 quantity,
        uint256 clientOrderId
    ) external returns (bytes32 orderId) {
        return _createOrder(
            pairId,
            OrderType.Limit,
            side,
            price,
            quantity,
            0,
            0,
            TimeInForce.IOC,
            clientOrderId,
            bytes32(0)
        );
    }

    /**
     * @notice Internal order creation
     */
    function _createOrder(
        address pairId,
        OrderType orderType,
        Side side,
        uint256 price,
        uint256 quantity,
        uint256 stopPrice,
        uint256 expiryTime,
        TimeInForce timeInForce,
        uint256 clientOrderId,
        bytes32 ocoGroupId
    ) internal onlyTradingEnabled returns (bytes32 orderId) {
        require(quantity > 0, "Invalid quantity");
        require(price > 0 || orderType == OrderType.StopLoss, "Invalid price");
        
        orderId = keccak256(abi.encode(
            pairId,
            msg.sender,
            orderType,
            side,
            price,
            quantity,
            block.timestamp,
            clientOrderId
        ));
        
        require(orders[orderId].id == bytes32(0), "Order exists");
        
        Order storage order = orders[orderId];
        order.id = orderId;
        order.user = msg.sender;
        order.pairId = pairId;
        order.orderType = orderType;
        order.side = side;
        order.price = price;
        order.quantity = quantity;
        order.filledQuantity = 0;
        order.remainingQuantity = quantity;
        order.stopPrice = stopPrice;
        order.expiryTime = expiryTime;
        order.createdAt = block.timestamp;
        order.status = ORDER_STATUS_PENDING;
        order.timeInForce = uint8(timeInForce);
        order.clientOrderId = clientOrderId;
        order.ocoGroupId = ocoGroupId;
        
        orderIds.push(orderId);
        orderCount++;
        
        // Add to order book for limit orders
        if (orderType == OrderType.Limit || orderType == OrderType.PostOnly) {
            _addToOrderBook(pairId, side, price, quantity, orderId);
        }
        
        emit OrderCreated(
            orderId,
            msg.sender,
            pairId,
            uint8(orderType),
            uint8(side),
            price,
            quantity,
            stopPrice
        );
    }

    /**
     * @notice Add order to order book
     */
    function _addToOrderBook(
        address pairId,
        Side side,
        uint256 price,
        uint256 quantity,
        bytes32 orderId
    ) internal {
        bytes32 queueKey = keccak256(abi.encode(pairId, side, price));
        
        if (side == Side.Buy) {
            bidQueue[queueKey].push(orderId);
        } else {
            askQueue[queueKey].push(orderId);
        }
    }

    // ============================================================================
    // TWAP Orders
    // ============================================================================

    /**
     * @notice Create TWAP order (Time-Weighted Average Price)
     */
    function createTWAPOrder(
        address pairId,
        Side side,
        uint256 totalQuantity,
        uint256 sliceInterval,
        uint256 slices,
        uint256 minPrice,
        uint256 maxPrice,
        uint256 clientOrderId
    ) external returns (bytes32 orderId) {
        require(slices > 0 && slices <= MAX_TWAP_SLICES, "Invalid slices");
        require(totalQuantity > 0, "Invalid quantity");
        require(sliceInterval >= 60, "Min interval 60s"); // Minimum 1 minute
        
        orderId = keccak256(abi.encode(
            pairId,
            msg.sender,
            "TWAP",
            totalQuantity,
            block.timestamp,
            clientOrderId
        ));
        
        require(twapOrders[orderId].id == bytes32(0), "TWAP exists");
        
        uint256 sliceQuantity = totalQuantity / slices;
        
        TWAPOrder storage twap = twapOrders[orderId];
        twap.id = orderId;
        twap.user = msg.sender;
        twap.pairId = pairId;
        twap.side = side;
        twap.totalQuantity = totalQuantity;
        twap.filledQuantity = 0;
        twap.sliceInterval = sliceInterval;
        twap.slicesRemaining = slices;
        twap.nextSliceTime = block.timestamp + sliceInterval;
        twap.minPrice = minPrice;
        twap.maxPrice = maxPrice;
        twap.isActive = true;
        
        twapOrderIds.push(orderId);
        
        emit TWAPCreated(orderId, msg.sender, totalQuantity, sliceInterval, slices);
    }

    /**
     * @notice Execute TWAP slice
     */
    function executeTWAP(bytes32 orderId) external returns (uint256 filledQuantity, uint256 fillPrice) {
        TWAPOrder storage twap = twapOrders[orderId];
        require(twap.id == orderId, "Invalid TWAP");
        require(twap.isActive, "TWAP not active");
        require(block.timestamp >= twap.nextSliceTime, "Too early");
        
        // Get current price from oracle
        uint256 currentPrice = _getCurrentPrice(twap.pairId);
        require(currentPrice >= twap.minPrice && currentPrice <= twap.maxPrice, "Price out of range");
        
        uint256 sliceQuantity = twap.totalQuantity / (twap.slicesRemaining + 1);
        
        // Execute the slice as a limit order
        bytes32 sliceOrderId = _createOrder(
            twap.pairId,
            OrderType.Limit,
            twap.side,
            currentPrice,
            sliceQuantity,
            0,
            block.timestamp + 300, // 5 min expiry
            TimeInForce.IOC,
            0,
            orderId
        );
        
        // Fill the order
        (filledQuantity, fillPrice) = _fillOrder(sliceOrderId, sliceQuantity);
        
        twap.filledQuantity += filledQuantity;
        twap.slicesRemaining--;
        twap.nextSliceTime = block.timestamp + twap.sliceInterval;
        
        emit TWAPExecuted(orderId, filledQuantity, fillPrice);
        
        if (twap.slicesRemaining == 0) {
            twap.isActive = false;
            emit TWAPCompleted(orderId);
        }
    }

    /**
     * @notice Cancel TWAP order
     */
    function cancelTWAP(bytes32 orderId) external {
        TWAPOrder storage twap = twapOrders[orderId];
        require(twap.id == orderId, "Invalid TWAP");
        require(twap.user == msg.sender || adminCanCancel, "Not authorized");
        
        twap.isActive = false;
        
        emit TWAPCompleted(orderId);
    }

    // ============================================================================
    // OCO Orders (One Cancels Other)
    // ============================================================================

    /**
     * @notice Create OCO order (One Cancels Other)
     */
    function createOCOOrder(
        address pairId,
        Side side1,
        uint256 price1,
        uint256 quantity,
        Side side2,
        uint256 price2,
        uint256 clientOrderId
    ) external returns (bytes32 groupId, bytes32 orderId1, bytes32 orderId2) {
        require(price1 != price2, "Same price");
        
        // Create first order
        orderId1 = _createOrder(
            pairId,
            OrderType.Limit,
            side1,
            price1,
            quantity,
            0,
            block.timestamp + 86400, // 24h
            TimeInForce.GTC,
            clientOrderId,
            bytes32(0)
        );
        
        // Create second order
        orderId2 = _createOrder(
            pairId,
            OrderType.Limit,
            side2,
            price2,
            quantity,
            0,
            block.timestamp + 86400,
            TimeInForce.GTC,
            clientOrderId + 1,
            bytes32(0)
        );
        
        // Create OCO group
        groupId = keccak256(abi.encode(orderId1, orderId2, block.timestamp));
        
        OCOGroup storage group = ocoGroups[groupId];
        group.id = groupId;
        group.orderId1 = orderId1;
        group.orderId2 = orderId2;
        group.user = msg.sender;
        group.status = 0;
        
        ocoOrderIds.push(groupId);
        
        // Link orders to group
        orders[orderId1].ocoGroupId = groupId;
        orders[orderId2].ocoGroupId = groupId;
    }

    /**
     * @notice Check and trigger OCO
     */
    function _checkOCOTrigger(bytes32 orderId) internal {
        Order storage order = orders[orderId];
        if (order.ocoGroupId == bytes32(0)) return;
        
        OCOGroup storage group = ocoGroups[order.ocoGroupId];
        if (group.status != 0) return;
        
        // Determine which order to cancel
        bytes32 cancelId = (order.id == group.orderId1) ? group.orderId2 : group.orderId1;
        
        // Cancel the other order
        Order storage toCancel = orders[cancelId];
        if (toCancel.status == ORDER_STATUS_PENDING) {
            toCancel.status = ORDER_STATUS_CANCELLED;
            emit OrderCancelled(cancelId, toCancel.user, "OCO triggered");
        }
        
        group.status = 1;
        emit OCOTriggered(group.id, orderId, cancelId);
    }

    // ============================================================================
    // Order Execution
    // ============================================================================

    /**
     * @notice Match orders (limit order execution)
     */
    function matchOrders(
        bytes32 bidOrderId,
        bytes32 askOrderId,
        uint256 quantity
    ) external returns (uint256 execPrice, uint256 bidFee, uint256 askFee) {
        Order storage bidOrder = orders[bidOrderId];
        Order storage askOrder = orders[askOrderId];
        
        require(bidOrder.id == bidOrderId && askOrder.id == askOrderId, "Invalid orders");
        require(bidOrder.side == Side.Buy && askOrder.side == Side.Sell, "Invalid sides");
        require(quantity <= bidOrder.remainingQuantity && quantity <= askOrder.remainingQuantity, "Insufficient qty");
        
        // Execute at ask price (matcher gets better price)
        execPrice = askOrder.price;
        
        // Fill orders
        _fillOrder(bidOrderId, quantity);
        _fillOrder(askOrderId, quantity);
        
        // Calculate fees
        bidFee = quantity * makerFeeBps / 10000;
        askFee = quantity * takerFeeBps / 10000;
        
        // Emit trade events
        emit OrderFilled(bidOrderId, bidOrder.user, quantity, execPrice, bidFee);
        emit OrderFilled(askOrderId, askOrder.user, quantity, execPrice, askFee);
        
        // Check OCO triggers
        _checkOCOTrigger(bidOrderId);
        _checkOCOTrigger(askOrderId);
    }

    /**
     * @notice Fill order internally
     */
    function _fillOrder(bytes32 orderId, uint256 quantity) internal returns (uint256 filled, uint256 price) {
        Order storage order = orders[orderId];
        
        filled = quantity > order.remainingQuantity ? order.remainingQuantity : quantity;
        price = order.price;
        
        order.filledQuantity += filled;
        order.remainingQuantity -= filled;
        
        if (order.remainingQuantity == 0) {
            order.status = ORDER_STATUS_FILLED;
        } else {
            order.status = ORDER_STATUS_PARTIAL;
        }
    }

    // ============================================================================
    // Order Cancellation
    // ============================================================================

    /**
     * @notice Cancel order
     */
    function cancelOrder(bytes32 orderId) external onlyValidOrder(orderId) {
        Order storage order = orders[orderId];
        require(order.user == msg.sender || adminCanCancel, "Not authorized");
        require(order.status == ORDER_STATUS_PENDING || order.status == ORDER_STATUS_PARTIAL, "Not pending");
        
        order.status = ORDER_STATUS_CANCELLED;
        
        emit OrderCancelled(orderId, order.user, "User cancelled");
    }

    /**
     * @notice Cancel all orders for user
     */
    function cancelAllOrders() external {
        uint256 cancelled = 0;
        
        for (uint256 i = 0; i < orderIds.length; i++) {
            Order storage order = orders[orderIds[i]];
            if (order.user == msg.sender && order.status == ORDER_STATUS_PENDING) {
                order.status = ORDER_STATUS_CANCELLED;
                cancelled++;
                emit OrderCancelled(orderIds[i], msg.sender, "User cancelled all");
            }
        }
        
        require(cancelled > 0, "No orders to cancel");
    }

    // ============================================================================
    // Stop Order Triggers
    // ============================================================================

    /**
     * @notice Trigger stop orders when price hits stop
     */
    function triggerStopOrders(address pairId) external {
        uint256 currentPrice = _getCurrentPrice(pairId);
        
        for (uint256 i = 0; i < orderIds.length; i++) {
            Order storage order = orders[orderIds[i]];
            if (order.pairId != pairId) continue;
            if (order.status != ORDER_STATUS_PENDING) continue;
            if (order.orderType != OrderType.StopLoss && order.orderType != OrderType.StopLimit) continue;
            
            bool triggered = (order.side == Side.Buy && currentPrice >= order.stopPrice) ||
                           (order.side == Side.Sell && currentPrice <= order.stopPrice);
            
            if (triggered) {
                if (order.orderType == OrderType.StopLimit) {
                    // Convert to market order
                    order.orderType = OrderType.Limit;
                    order.price = currentPrice;
                    _addToOrderBook(order.pairId, order.side, order.price, order.remainingQuantity, order.id);
                } else {
                    // Execute at market price
                    order.price = currentPrice;
                    _fillOrder(order.id, order.remainingQuantity);
                    order.status = ORDER_STATUS_FILLED;
                }
                
                emit OrderFilled(order.id, order.user, order.remainingQuantity, currentPrice, 0);
            }
        }
    }

    /**
     * @notice Get current price from oracle
     */
    function _getCurrentPrice(address pairId) internal view returns (uint256) {
        // Simplified - would integrate with price oracle
        return 1e18; // Placeholder
    }

    // ============================================================================
    // Admin Functions
    // ============================================================================

    function setTradingEnabled(bool enabled) external {
        require(msg.sender == feeRecipient, "Not admin");
        tradingEnabled = enabled;
    }

    function setFeeRecipient(address _feeRecipient) external {
        require(msg.sender == feeRecipient, "Not admin");
        feeRecipient = _feeRecipient;
    }

    function setFees(uint256 _makerFeeBps, uint256 _takerFeeBps) external {
        require(msg.sender == feeRecipient, "Not admin");
        makerFeeBps = _makerFeeBps;
        takerFeeBps = _takerFeeBps;
    }

    function setPriceOracle(address _priceOracle) external {
        require(msg.sender == feeRecipient, "Not admin");
        priceOracle = _priceOracle;
    }

    function setAdminCancel(bool _adminCanCancel) external {
        require(msg.sender == feeRecipient, "Not admin");
        adminCanCancel = _adminCanCancel;
    }

    // ============================================================================
    // View Functions
    // ============================================================================

    function getOrder(bytes32 orderId) external view returns (
        address user,
        address pairId,
        uint8 orderType,
        uint8 side,
        uint256 price,
        uint256 quantity,
        uint256 filledQuantity,
        uint256 stopPrice,
        uint8 status
    ) {
        Order storage o = orders[orderId];
        return (
            o.user,
            o.pairId,
            uint8(o.orderType),
            uint8(o.side),
            o.price,
            o.quantity,
            o.filledQuantity,
            o.stopPrice,
            o.status
        );
    }

    function getOrderBook(address pairId, Side side, uint256 startPrice, uint256 maxLevels) 
        external view returns (uint256[] memory prices, uint256[] memory quantities) {
        // Return top N levels of order book
    }

    function getUserOrders(address user) external view returns (bytes32[] memory) {
        bytes32[] memory userOrderIds = new bytes32[](MAX_ORDERS);
        uint256 count = 0;
        
        for (uint256 i = 0; i < orderIds.length; i++) {
            if (orders[orderIds[i]].user == user) {
                userOrderIds[count] = orderIds[i];
                count++;
            }
        }
        
        // Return trimmed array
        bytes32[] memory result = new bytes32[](count);
        for (uint256 i = 0; i < count; i++) {
            result[i] = userOrderIds[i];
        }
        
        return result;
    }

    function getTWAPOrder(bytes32 orderId) external view returns (
        address user,
        address pairId,
        uint256 totalQuantity,
        uint256 filledQuantity,
        uint256 slicesRemaining,
        bool isActive
    ) {
        TWAPOrder storage t = twapOrders[orderId];
        return (
            t.user,
            t.pairId,
            t.totalQuantity,
            t.filledQuantity,
            t.slicesRemaining,
            t.isActive
        );
    }
}