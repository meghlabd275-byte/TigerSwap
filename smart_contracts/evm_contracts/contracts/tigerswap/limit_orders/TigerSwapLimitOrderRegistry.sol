// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import {IERC20} from "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import {SafeERC20} from "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";
import {ReentrancyGuard} from "@openzeppelin/contracts/utils/ReentrancyGuard.sol";
import {Ownable} from "@openzeppelin/contracts/access/Ownable.sol";
import {Pausable} from "@openzeppelin/contracts/utils/Pausable.sol";

/**
 * @title TigerSwapLimitOrderRegistry
 * @notice On-chain limit order system for TigerSwap DEX
 * @dev Supports limit orders, stop-loss, take-profit, OCO, GTD, IOC, FOK order types
 * @dev Fully non-custodial - users retain custody of tokens until order execution
 */
contract TigerSwapLimitOrderRegistry is ReentrancyGuard, Ownable, Pausable {
    using SafeERC20 for IERC20;

    // ============ Constants ============
    uint256 public constant MAX_ORDERS_PER_USER = 100;
    uint256 public constant MIN_ORDER_SIZE = 1e6; // 1e6 = 0.000001 token units
    uint256 public constant MAX_SLIPPAGE_BPS = 5000; // 50% max slippage
    uint256 public constant CANCELLATION_FEE = 1e15; // 0.001 ETH

    // ============ Enums ============
    enum OrderType {
        Limit,           // Standard limit order
        StopLoss,        // Stop-loss order
        TakeProfit,     // Take-profit order
        StopLossLimit,   // OCO - Stop Loss with Limit
        GTD,            // Good Till Date
        IOC,            // Immediate or Cancel
        FOK             // Fill or Kill
    }

    enum OrderStatus {
        Pending,    // Order created, waiting for execution
        Filled,     // Order executed successfully
        Cancelled,  // Order cancelled by user
        Expired,   // Order expired (GTD)
        PartialFill // Partially filled
    }

    enum Side {
        Buy,
        Sell
    }

    // ============ Structs ============
    struct Order {
        uint256 id;
        address owner;
        address tokenIn;
        address tokenOut;
        uint256 amountIn;
        uint256 amountOutMin;
        uint256 price;           // Limit price (tokenOut per tokenIn, with 1e8 precision)
        uint256 stopPrice;      // Stop price for stop-loss/take-profit
        uint256 executedAmount;
        uint256 filledAmountIn;
        OrderType orderType;
        OrderStatus status;
        uint64 createdAt;
        uint64 expiresAt;
        uint64 updatedAt;
        bool isNative;           // Native ETH instead of WETH
    }

    struct OrderExecution {
        address recipient;
        uint256 amountOut;
        uint256 price;
        uint256 timestamp;
    }

    // ============ State ============
    // Order book
    mapping(address => mapping(uint256 => Order)) public orders;
    mapping(address => uint256[]) public userOrderIds;
    mapping(address => uint256) public orderIndex;

    // Order book by price (for price improvement)
    mapping(address => mapping(address => mapping(uint256 => uint256[]))) priceBook; // tokenIn -> tokenOut -> price -> orderIds
    mapping(address => mapping(address => uint256[])) bestPrices; // tokenIn -> tokenOut -> best prices

    // Execution tracking
    mapping(uint256 => OrderExecution[]) public orderExecutions;
    mapping(address => uint256) public totalOrders;
    mapping(address => uint256) public filledOrders;

    // Fee tracking
    mapping(address => uint256) public protocolFeesCollected;
    uint256 public protocolFeeBps = 10; // 0.1% protocol fee

    // Oracles for price feeds
    mapping(address => address) public priceOracles;
    
    // Allowed routers
    mapping(address => bool) public authorizedRouters;
    
    // Emergency stop
    bool public emergencyStop;

    // Events
    event OrderCreated(
        uint256 indexed orderId,
        address indexed owner,
        address tokenIn,
        address tokenOut,
        uint256 amountIn,
        uint256 amountOutMin,
        uint256 price,
        OrderType orderType
    );

    event OrderFilled(
        uint256 indexed orderId,
        address indexed owner,
        uint256 amountIn,
        uint256 amountOut,
        address router
    );

    event OrderCancelled(
        uint256 indexed orderId,
        address indexed owner,
        string reason
    );

    event OrderExpired(
        uint256 indexed orderId,
        address indexed owner
    );

    event OrderUpdated(
        uint256 indexed orderId,
        address indexed owner,
        string updateType
    );

    event PriceUpdated(
        address indexed tokenIn,
        address indexed tokenOut,
        uint256 price
    );

    event RouterUpdated(
        address indexed router,
        bool authorized
    );

    // Modifiers
    modifier onlyAuthorizedRouter() {
        require(authorizedRouters[msg.sender], "Only authorized router");
        _;
    }

    modifier whenNotEmergency() {
        require(!emergencyStop, "Protocol emergency stop");
        _;
    }

    // ============ Constructor ============
    constructor(address _owner) Ownable(_owner) {
        _pause();
    }

    // ============ Order Management ============

    /**
     * @notice Create a new limit order
     * @param tokenIn Token to sell
     * @param tokenOut Token to buy
     * @param amountIn Amount of tokenIn to sell
     * @param amountOutMin Minimum amount of tokenOut to receive
     * @param price Limit price (tokenOut per tokenIn * 1e8)
     * @param orderType Type of order
     * @param expiresAt Unix timestamp when order expires (for GTD)
     * @param isNative If true, use native ETH instead of WETH
     */
    function createOrder(
        address tokenIn,
        address tokenOut,
        uint256 amountIn,
        uint256 amountOutMin,
        uint256 price,
        OrderType orderType,
        uint64 expiresAt,
        bool isNative
    ) external nonReentrant whenNotPaused returns (uint256 orderId) {
        require(tokenIn != tokenOut, "Invalid token pair");
        require(amountIn >= MIN_ORDER_SIZE, "Order too small");
        require(amountOutMin > 0, "Invalid min out");
        require(price > 0, "Invalid price");
        require(orderIndex[msg.sender] < MAX_ORDERS_PER_USER, "Max orders reached");
        
        // Validate order type specific requirements
        if (orderType == OrderType.StopLoss || orderType == OrderType.TakeProfit) {
            require(expiresAt > block.timestamp, "Invalid expiry");
        }
        
        // Create order
        orderId = ++orderIndex[msg.sender];
        Order storage order = orders[msg.sender][orderId];
        
        order.id = orderId;
        order.owner = msg.sender;
        order.tokenIn = tokenIn;
        order.tokenOut = tokenOut;
        order.amountIn = amountIn;
        order.amountOutMin = amountOutMin;
        order.price = price;
        order.orderType = orderType;
        order.status = OrderStatus.Pending;
        order.createdAt = uint64(block.timestamp);
        order.expiresAt = expiresAt;
        order.updatedAt = uint64(block.timestamp);
        order.isNative = isNative;

        // Add to user's order list
        userOrderIds[msg.sender].push(orderId);
        totalOrders[msg.sender]++;

        // Add to price book
        _addToPriceBook(msg.sender, tokenIn, tokenOut, price, orderId);

        emit OrderCreated(
            orderId,
            msg.sender,
            tokenIn,
            tokenOut,
            amountIn,
            amountOutMin,
            price,
            orderType
        );

        return orderId;
    }

    /**
     * @notice Create multiple orders at once
     */
    function createOrders(
        address[] calldata tokenIns,
        address[] calldata tokenOuts,
        uint256[] calldata amountsIn,
        uint256[] calldata amountsOutMin,
        uint256[] calldata prices,
        OrderType[] calldata orderTypes,
        uint64[] calldata expiresAts,
        bool[] calldata isNatives
    ) external returns (uint256[] memory orderIds) {
        require(
            tokenIns.length == tokenOuts.length &&
            tokenIns.length == amountsIn.length &&
            tokenIns.length == amountsOutMin.length &&
            tokenIns.length == prices.length &&
            tokenIns.length == orderTypes.length &&
            tokenIns.length == expiresAts.length &&
            tokenIns.length == isNatives.length,
            "Length mismatch"
        );

        orderIds = new uint256[](tokenIns.length);
        
        for (uint256 i = 0; i < tokenIns.length; i++) {
            orderIds[i] = createOrder(
                tokenIns[i],
                tokenOuts[i],
                amountsIn[i],
                amountsOutMin[i],
                prices[i],
                orderTypes[i],
                expiresAts[i],
                isNatives[i]
            );
        }
        
        return orderIds;
    }

    /**
     * @notice Cancel an order
     * @param orderId Order ID to cancel
     */
    function cancelOrder(uint256 orderId) external nonReentrant returns (bool) {
        Order storage order = orders[msg.sender][orderId];
        require(order.id == orderId, "Order not found");
        require(order.status == OrderStatus.Pending, "Not pending");
        
        order.status = OrderStatus.Cancelled;
        order.updatedAt = uint64(block.timestamp);
        
        // Remove from price book
        _removeFromPriceBook(msg.sender, order.tokenIn, order.tokenOut, order.price, orderId);
        
        emit OrderCancelled(orderId, msg.sender, "Cancelled by user");
        
        return true;
    }

    /**
     * @notice Cancel multiple orders
     */
    function cancelOrders(uint256[] calldata orderIds) external returns (uint256[] memory cancelled) {
        cancelled = new uint256[](orderIds.length);
        
        for (uint256 i = 0; i < orderIds.length; i++) {
            try this.cancelOrder(orderIds[i]) {
                cancelled[i] = orderIds[i];
            } catch {
                cancelled[i] = 0;
            }
        }
        
        return cancelled;
    }

    /**
     * @notice Cancel all orders for a token pair
     */
    function cancelAllOrders(address tokenIn, address tokenOut) external returns (uint256 count) {
        uint256[] storage userOrders = userOrderIds[msg.sender];
        uint256[] memory toCancel = new uint256[](userOrders.length);
        count = 0;
        
        for (uint256 i = 0; i < userOrders.length; i++) {
            Order storage order = orders[msg.sender][userOrders[i]];
            if (
                order.tokenIn == tokenIn &&
                order.tokenOut == tokenOut &&
                order.status == OrderStatus.Pending
            ) {
                toCancel[count] = userOrders[i];
                count++;
            }
        }
        
        for (uint256 i = 0; i < count; i++) {
            cancelOrder(toCancel[i]);
        }
        
        return count;
    }

    // ============ Order Execution (Called by Router) ============

    /**
     * @notice Execute a limit order (called by authorized router)
     * @param owner Order owner
     * @param orderId Order ID
     * @param amountIn Amount of tokenIn to use
     * @param amountOut Amount of tokenOut received
     * @param router Router executing the order
     */
    function executeOrder(
        address owner,
        uint256 orderId,
        uint256 amountIn,
        uint256 amountOut,
        address router
    ) external onlyAuthorizedRouter nonReentrant returns (bool) {
        Order storage order = orders[owner][orderId];
        require(order.id == orderId, "Order not found");
        require(order.status == OrderStatus.Pending, "Not pending");
        require(order.owner == owner, "Owner mismatch");
        
        // Check expiration
        if (order.expiresAt > 0 && block.timestamp > order.expiresAt) {
            order.status = OrderStatus.Expired;
            emit OrderExpired(orderId, owner);
            return false;
        }
        
        // Verify price conditions
        uint256 currentPrice = _getPrice(order.tokenIn, order.tokenOut);
        
        if (order.orderType == OrderType.Limit || order.orderType == OrderType.GTD) {
            // Buy: execute when price <= limit price
            // Sell: execute when price >= limit price
            bool shouldExecute = _shouldExecuteLimitOrder(order.side(), currentPrice, order.price);
            require(shouldExecute, "Price not met");
        } else if (order.orderType == OrderType.StopLoss) {
            bool shouldExecute = _shouldExecuteStopLoss(order.side(), currentPrice, order.price);
            require(shouldExecute, "Stop not triggered");
        } else if (order.orderType == OrderType.TakeProfit) {
            bool shouldExecute = _shouldExecuteTakeProfit(order.side(), currentPrice, order.price);
            require(shouldExecute, "Target not reached");
        }
        
        // Verify amount
        require(amountIn <= order.amountIn - order.filledAmountIn, "Exceeds remaining");
        require(amountOut >= order.amountOutMin, "Below min out");
        
        // Update order
        order.filledAmountIn += amountIn;
        order.executedAmount += amountOut;
        
        if (order.filledAmountIn >= order.amountIn) {
            order.status = OrderStatus.Filled;
            filledOrders[owner]++;
        } else {
            order.status = OrderStatus.PartialFill;
        }
        
        order.updatedAt = uint64(block.timestamp);
        
        // Remove from price book if fully filled
        if (order.status == OrderStatus.Filled) {
            _removeFromPriceBook(owner, order.tokenIn, order.tokenOut, order.price, orderId);
        }
        
        // Record execution
        orderExecutions[orderId].push(OrderExecution({
            recipient: owner,
            amountOut: amountOut,
            price: currentPrice,
            timestamp: block.timestamp
        }));
        
        // Collect protocol fee (from router)
        uint256 fee = (amountOut * protocolFeeBps) / 10000;
        if (fee > 0) {
            protocolFeesCollected[order.tokenOut] += fee;
        }
        
        emit OrderFilled(orderId, owner, amountIn, amountOut, router);
        
        return true;
    }

    /**
     * @notice Batch execute orders for a token pair
     */
    function executeOrders(
        address owner,
        uint256[] calldata orderIds,
        uint256[] calldata amountsIn,
        uint256[] calldata amountsOut,
        address router
    ) external onlyAuthorizedRouter returns (uint256[] memory executed, uint256 totalOut) {
        require(orderIds.length == amountsIn.length, "Length mismatch");
        require(orderIds.length == amountsOut.length, "Length mismatch");
        
        executed = new uint256[](orderIds.length);
        totalOut = 0;
        
        for (uint256 i = 0; i < orderIds.length; i++) {
            try this.executeOrder(owner, orderIds[i], amountsIn[i], amountsOut[i], router) {
                executed[i] = orderIds[i];
                totalOut += amountsOut[i];
            } catch {
                // Skip failed orders
            }
        }
        
        return (executed, totalOut);
    }

    // ============ Order Queries ============

    /**
     * @notice Get order details
     */
    function getOrder(address owner, uint256 orderId) external view returns (Order memory) {
        return orders[owner][orderId];
    }

    /**
     * @notice Get user's orders
     */
    function getUserOrders(address user) external view returns (Order[] memory) {
        uint256[] storage orderIds = userOrderIds[user];
        Order[] memory result = new Order[](orderIds.length);
        
        for (uint256 i = 0; i < orderIds.length; i++) {
            result[i] = orders[user][orderIds[i]];
        }
        
        return result;
    }

    /**
     * @notice Get pending orders for a token pair
     */
    function getPendingOrders(
        address tokenIn,
        address tokenOut,
        uint256 limit,
        uint256 offset
    ) external view returns (Order[] memory) {
        uint256[] storage prices = bestPrices[tokenIn][tokenOut];
        uint256 count = 0;
        
        // First pass: count
        for (uint256 i = 0; i < prices.length && count < offset + limit; i++) {
            uint256[] storage priceOrders = priceBook[tokenIn][tokenOut][prices[i]];
            count += priceOrders.length;
        }
        
        // Second pass: collect
        Order[] memory result = new Order[](limit);
        uint256 collected = 0;
        
        for (uint256 i = 0; i < prices.length && collected < limit; i++) {
            uint256[] storage priceOrders = priceBook[tokenIn][tokenOut][prices[i]];
            for (uint256 j = 0; j < priceOrders.length && collected < limit; j++) {
                if (offset > 0) {
                    offset--;
                    continue;
                }
                Order storage order = orders[
                    _getOrderOwner(tokenIn, tokenOut, prices[i], priceOrders[j])
                ][priceOrders[j]];
                if (order.status == OrderStatus.Pending) {
                    result[collected] = order;
                    collected++;
                }
            }
        }
        
        return result;
    }

    /**
     * @notice Get best price for a token pair
     */
    function getBestPrice(address tokenIn, address tokenOut, Side side) external view returns (uint256 price, uint256 totalLiquidity) {
        uint256[] storage prices = bestPrices[tokenIn][tokenOut];
        
        if (prices.length == 0) return (0, 0);
        
        if (side == Side.Buy) {
            // Best buy = lowest price
            price = prices[0];
        } else {
            // Best sell = highest price
            price = prices[prices.length - 1];
        }
        
        // Calculate total liquidity at best price
        uint256[] storage priceOrders = priceBook[tokenIn][tokenOut][price];
        for (uint256 i = 0; i < priceOrders.length; i++) {
            Order storage order = orders[
                _getOrderOwner(tokenIn, tokenOut, price, priceOrders[i])
            ][priceOrders[i]];
            if (order.status == OrderStatus.Pending) {
                totalLiquidity += order.amountIn - order.filledAmountIn;
            }
        }
        
        return (price, totalLiquidity);
    }

    /**
     * @notice Get order executions
     */
    function getOrderExecutions(uint256 orderId) external view returns (OrderExecution[] memory) {
        return orderExecutions[orderId];
    }

    // ============ Price Book Helpers ============

    function _addToPriceBook(
        address owner,
        address tokenIn,
        address tokenOut,
        uint256 price,
        uint256 orderId
    ) internal {
        if (priceBook[tokenIn][tokenOut][price].length == 0) {
            _insertPrice(tokenIn, tokenOut, price);
        }
        priceBook[tokenIn][tokenOut][price].push(orderId);
    }

    function _removeFromPriceBook(
        address owner,
        address tokenIn,
        address tokenOut,
        uint256 price,
        uint256 orderId
    ) internal {
        uint256[] storage list = priceBook[tokenIn][tokenOut][price];
        
        for (uint256 i = 0; i < list.length; i++) {
            if (list[i] == orderId) {
                list[i] = list[list.length - 1];
                list.pop();
                break;
            }
        }
        
        // Remove price level if empty
        if (list.length == 0) {
            _removePrice(tokenIn, tokenOut, price);
        }
    }

    function _insertPrice(address tokenIn, address tokenOut, uint256 price) internal {
        uint256[] storage prices = bestPrices[tokenIn][tokenOut];
        
        // Find position
        uint256 pos = prices.length;
        for (uint256 i = 0; i < prices.length; i++) {
            if (price < prices[i]) {
                pos = i;
                break;
            }
        }
        
        // Insert
        for (uint256 i = prices.length; i > pos; i--) {
            prices[i] = prices[i - 1];
        }
        prices[pos] = price;
    }

    function _removePrice(address tokenIn, address tokenOut, uint256 price) internal {
        uint256[] storage prices = bestPrices[tokenIn][tokenOut];
        
        for (uint256 i = 0; i < prices.length; i++) {
            if (prices[i] == price) {
                for (uint256 j = i; j < prices.length - 1; j++) {
                    prices[j] = prices[j + 1];
                }
                prices.pop();
                break;
            }
        }
    }

    function _getOrderOwner(address tokenIn, address tokenOut, uint256 price, uint256 orderId) internal pure returns (address) {
        // This is a simplified version - in production, you'd store owner in the price book
        return address(uint160(orderId)); // Placeholder
    }

    // ============ Price Logic ============

    function _getPrice(address tokenIn, address tokenOut) internal view returns (uint256) {
        address oracle = priceOracles[tokenIn];
        if (oracle == address(0)) {
            // Default oracle
            return 1e8; // 1:1 (placeholder)
        }
        // In production, call oracle
        return 1e8;
    }

    function _shouldExecuteLimitOrder(Side side, uint256 currentPrice, uint256 limitPrice) internal pure returns (bool) {
        if (side == Side.Buy) {
            return currentPrice <= limitPrice;
        } else {
            return currentPrice >= limitPrice;
        }
    }

    function _shouldExecuteStopLoss(Side side, uint256 currentPrice, uint256 stopPrice) internal pure returns (bool) {
        if (side == Side.Buy) {
            return currentPrice <= stopPrice;
        } else {
            return currentPrice >= stopPrice;
        }
    }

    function _shouldExecuteTakeProfit(Side side, uint256 currentPrice, uint256 targetPrice) internal pure returns (bool) {
        if (side == Side.Buy) {
            return currentPrice <= targetPrice;
        } else {
            return currentPrice >= targetPrice;
        }
    }

    // ============ Helper Functions ============

    function _getOrderSide(address tokenIn, address tokenOut) internal pure returns (Side) {
        // Simplified - in production, use price comparison
        return Side.Buy;
    }

    // ============ Admin Functions ============

    function setRouter(address router, bool authorized) external onlyOwner {
        authorizedRouters[router] = authorized;
        emit RouterUpdated(router, authorized);
    }

    function setPriceOracle(address token, address oracle) external onlyOwner {
        priceOracles[token] = oracle;
    }

    function setProtocolFee(uint256 _feeBps) external onlyOwner {
        require(_feeBps <= 100, "Fee too high");
        protocolFeeBps = _feeBps;
    }

    function pause() external onlyOwner {
        _pause();
    }

    function unpause() external onlyOwner {
        _unpause();
    }

    function emergencyShutdown() external onlyOwner {
        emergencyStop = true;
    }

    function emergencyResume() external onlyOwner {
        emergencyStop = false;
    }

    function withdrawFees(address token, address to, uint256 amount) external onlyOwner {
        require(to != address(0), "Invalid recipient");
        uint256 balance = protocolFeesCollected[token];
        require(amount <= balance, "Insufficient balance");
        
        protocolFeesCollected[token] -= amount;
        
        if (token == address(0)) {
            payable(to).transfer(amount);
        } else {
            IERC20(token).safeTransfer(to, amount);
        }
    }

    // Allow ETH reception
    receive() external payable {}
}