// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/**
 * @title TigerSwapAdvancedOrders
 * @notice Advanced order types: Limit, Stop-Loss, Take-Profit, TWAP, GTC, IOC, Post-Only
 */
contract TigerSwapAdvancedOrders {
    // ============ Enums ============
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
    
    // ============ Structs ============
    struct Order {
        uint256 id;
        address owner;
        OrderType orderType;
        OrderSide side;
        address tokenIn;
        address tokenOut;
        uint256 amountIn;
        uint256 amountOutMin;
        uint256 price;           // For limit orders
        uint256 stopPrice;        // For stop orders
        uint256 filledAmount;
        uint256 createdAt;
        uint256 expiresAt;
        OrderStatus status;
        uint256 twapInterval;    // For TWAP orders
        uint256 twapParts;       // For TWAP orders
        uint256 twapCompletedParts;
    }
    
    // ============ State Variables ============
    mapping(uint256 => Order) public orders;
    uint256 public orderCount;
    
    // Order ID by user
    mapping(address => uint256[]) public userOrders;
    
    // Allowed executors (keepers)
    mapping(address => bool) public executors;
    
    // Oracle for price verification
    address public priceOracle;
    
    // Fees
    uint256 public limitOrderFee = 30; // 0.3%
    
    // Events
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
    
    event OrderFilled(
        uint256 indexed orderId,
        uint256 fillAmount,
        uint256 fillPrice
    );
    
    event OrderCancelled(
        uint256 indexed orderId,
        address indexed owner,
        string reason
    );
    
    event OrderTriggered(
        uint256 indexed orderId,
        uint256 triggerPrice
    );
    
    // ============ Modifiers ============
    modifier onlyExecutor() {
        require(executors[msg.sender] || msg.sender == address(this), "Not authorized executor");
        _;
    }
    
    // ============ Constructor ============
    constructor(address _priceOracle) {
        priceOracle = _priceOracle;
    }
    
    // ============ Create Orders ============
    
    /**
     * @notice Create a limit order
     */
    function createLimitOrder(
        OrderSide side,
        address tokenIn,
        address tokenOut,
        uint256 amountIn,
        uint256 amountOutMin,
        uint256 price,
        uint256 expiresIn
    ) external returns (uint256) {
        return _createOrder(
            OrderType.LIMIT,
            side,
            tokenIn,
            tokenOut,
            amountIn,
            amountOutMin,
            price,
            0,
            expiresIn,
            0,
            0
        );
    }
    
    /**
     * @notice Create a stop-loss order
     */
    function createStopLossOrder(
        OrderSide side,
        address tokenIn,
        address tokenOut,
        uint256 amountIn,
        uint256 amountOutMin,
        uint256 stopPrice,
        uint256 expiresIn
    ) external returns (uint256) {
        require(stopPrice > 0, "Invalid stop price");
        
        return _createOrder(
            OrderType.STOP_LOSS,
            side,
            tokenIn,
            tokenOut,
            amountIn,
            amountOutMin,
            0,
            stopPrice,
            expiresIn,
            0,
            0
        );
    }
    
    /**
     * @notice Create a take-profit order
     */
    function createTakeProfitOrder(
        OrderSide side,
        address tokenIn,
        address tokenOut,
        uint256 amountIn,
        uint256 amountOutMin,
        uint256 takeProfitPrice,
        uint256 expiresIn
    ) external returns (uint256) {
        require(takeProfitPrice > 0, "Invalid take profit price");
        
        return _createOrder(
            OrderType.TAKE_PROFIT,
            side,
            tokenIn,
            tokenOut,
            amountIn,
            amountOutMin,
            takeProfitPrice,
            0,
            expiresIn,
            0,
            0
        );
    }
    
    /**
     * @notice Create a TWAP (Time-Weighted Average Price) order
     */
    function createTWAPOrder(
        OrderSide side,
        address tokenIn,
        address tokenOut,
        uint256 totalAmount,
        uint256 amountOutMin,
        uint256 twapInterval,
        uint256 twapParts
    ) external returns (uint256) {
        require(twapInterval > 0, "Invalid interval");
        require(twapParts > 0, "Invalid parts");
        require(totalAmount >= twapParts, "Amount too small for parts");
        
        return _createOrder(
            OrderType.TWAP,
            side,
            tokenIn,
            tokenOut,
            totalAmount,
            amountOutMin,
            0,
            0,
            block.timestamp + 7 days,
            twapInterval,
            twapParts
        );
    }
    
    /**
     * @notice Create GTC (Good Till Cancel) order
     */
    function createGTCOrder(
        OrderSide side,
        address tokenIn,
        address tokenOut,
        uint256 amountIn,
        uint256 amountOutMin,
        uint256 price,
        uint256 expiresIn
    ) external returns (uint256) {
        return _createOrder(
            OrderType.GTC,
            side,
            tokenIn,
            tokenOut,
            amountIn,
            amountOutMin,
            price,
            0,
            expiresIn,
            0,
            0
        );
    }
    
    /**
     * @notice Create IOC (Immediate or Cancel) order
     */
    function createIOCOrder(
        OrderSide side,
        address tokenIn,
        address tokenOut,
        uint256 amountIn,
        uint256 amountOutMin,
        uint256 price
    ) external returns (uint256) {
        uint256 orderId = _createOrder(
            OrderType.IOC,
            side,
            tokenIn,
            tokenOut,
            amountIn,
            amountOutMin,
            price,
            0,
            block.timestamp, // Expires immediately
            0,
            0
        );
        
        // IOC orders execute immediately then cancel if not filled
        _executeIOC(orderId);
        
        return orderId;
    }
    
    /**
     * @notice Create Post-Only order (only maker, no taking liquidity)
     */
    function createPostOnlyOrder(
        OrderSide side,
        address tokenIn,
        address tokenOut,
        uint256 amountIn,
        uint256 amountOutMin,
        uint256 price,
        uint256 expiresIn
    ) external returns (uint256) {
        return _createOrder(
            OrderType.POST_ONLY,
            side,
            tokenIn,
            tokenOut,
            amountIn,
            amountOutMin,
            price,
            0,
            expiresIn,
            0,
            0
        );
    }
    
    // ============ Internal Order Creation ============
    
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
        require(amountIn > 0, "Invalid amount");
        require(tokenIn != tokenOut, "Same token");
        
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
        order.filledAmount = 0;
        order.createdAt = block.timestamp;
        order.expiresAt = expiresAt > 0 ? expiresAt : block.timestamp + 30 days;
        order.status = OrderStatus.OPEN;
        order.twapInterval = twapInterval;
        order.twapParts = twapParts;
        
        // Transfer tokens from user
        IERC20(tokenIn).transferFrom(msg.sender, address(this), amountIn);
        
        // Add to user's orders
        userOrders[msg.sender].push(orderId);
        
        emit OrderCreated(
            orderId,
            msg.sender,
            orderType,
            side,
            tokenIn,
            tokenOut,
            amountIn,
            price
        );
        
        return orderId;
    }
    
    // ============ Execute Orders ============
    
    /**
     * @notice Execute an order (called by executor/keeper)
     */
    function executeOrder(
        uint256 orderId,
        uint256 currentPrice,
        uint256 fillAmount
    ) external onlyExecutor returns (uint256) {
        Order storage order = orders[orderId];
        require(order.status == OrderStatus.OPEN, "Order not open");
        require(block.timestamp <= order.expiresAt, "Order expired");
        
        // Verify price conditions
        bool shouldExecute = _checkPriceCondition(order, currentPrice);
        require(shouldExecute, "Price condition not met");
        
        // Calculate fill amount
        uint256 actualFill = fillAmount;
        if (actualFill > order.amountIn - order.filledAmount) {
            actualFill = order.amountIn - order.filledAmount;
        }
        
        // Update order
        order.filledAmount += actualFill;
        
        if (order.filledAmount >= order.amountIn) {
            order.status = OrderStatus.FILLED;
        } else if (order.orderType == OrderType.TWAP) {
            order.twapCompletedParts++;
            if (order.twapCompletedParts >= order.twapParts) {
                order.status = OrderStatus.FILLED;
            }
        } else {
            order.status = OrderStatus.PARTIALLY_FILLED;
        }
        
        // Calculate output
        uint256 outputAmount = _calculateOutput(
            actualFill,
            currentPrice,
            order.amountOutMin
        );
        
        // Transfer output to order owner
        IERC20(order.tokenOut).transfer(order.owner, outputAmount);
        
        // Apply fees
        uint256 fee = actualFill * limitOrderFee / 10000;
        
        emit OrderFilled(orderId, actualFill, currentPrice);
        
        return outputAmount;
    }
    
    /**
     * @notice Trigger stop-loss or take-profit orders
     */
    function triggerOrder(uint256 orderId, uint256 currentPrice) external onlyExecutor {
        Order storage order = orders[orderId];
        
        require(order.orderType == OrderType.STOP_LOSS || order.orderType == OrderType.TAKE_PROFIT,
            "Not a triggerable order");
        require(order.status == OrderStatus.OPEN, "Order not open");
        
        bool shouldTrigger = _checkTriggerCondition(order, currentPrice);
        require(shouldTrigger, "Trigger condition not met");
        
        order.status = OrderStatus.TRIGGERED;
        
        // Execute the order
        _executeTriggeredOrder(orderId, currentPrice);
        
        emit OrderTriggered(orderId, currentPrice);
    }
    
    // ============ Cancel Orders ============
    
    /**
     * @notice Cancel an order
     */
    function cancelOrder(uint256 orderId) external {
        Order storage order = orders[orderId];
        
        require(order.owner == msg.sender, "Not order owner");
        require(order.status == OrderStatus.OPEN || order.status == OrderStatus.PARTIALLY_FILLED,
            "Cannot cancel");
        
        // Refund remaining tokens
        uint256 remaining = order.amountIn - order.filledAmount;
        if (remaining > 0) {
            IERC20(order.tokenIn).transfer(order.owner, remaining);
        }
        
        order.status = OrderStatus.CANCELLED;
        
        emit OrderCancelled(orderId, msg.sender, "Cancelled by user");
    }
    
    // ============ Internal Functions ============
    
    function _checkPriceCondition(Order storage order, uint256 currentPrice) internal view returns (bool) {
        if (order.orderType == OrderType.MARKET) {
            return true;
        }
        
        if (order.orderType == OrderType.LIMIT) {
            if (order.side == OrderSide.BUY) {
                return currentPrice <= order.price;
            } else {
                return currentPrice >= order.price;
            }
        }
        
        if (order.orderType == OrderType.POST_ONLY) {
            // Post-only should only execute as maker (limit order that doesn't take liquidity)
            return currentPrice == order.price;
        }
        
        return false;
    }
    
    function _checkTriggerCondition(Order storage order, uint256 currentPrice) internal view returns (bool) {
        if (order.orderType == OrderType.STOP_LOSS) {
            if (order.side == OrderSide.BUY) {
                // Buy stop: trigger when price rises above stop
                return currentPrice >= order.stopPrice;
            } else {
                // Sell stop: trigger when price falls below stop
                return currentPrice <= order.stopPrice;
            }
        }
        
        if (order.orderType == OrderType.TAKE_PROFIT) {
            if (order.side == OrderSide.BUY) {
                // Take profit on buy: trigger when price rises above target
                return currentPrice >= order.price;
            } else {
                // Take profit on sell: trigger when price falls below target
                return currentPrice <= order.price;
            }
        }
        
        return false;
    }
    
    function _calculateOutput(uint256 inputAmount, uint256 price, uint256 minOutput) internal pure returns (uint256) {
        uint256 output = inputAmount * price / 1e8; // Assuming price has 8 decimals
        if (output < minOutput) {
            return minOutput;
        }
        return output;
    }
    
    function _executeIOC(uint256 orderId) internal {
        Order storage order = orders[orderId];
        
        if (order.status == OrderStatus.OPEN) {
            // Return tokens if not filled
            uint256 remaining = order.amountIn - order.filledAmount;
            if (remaining > 0) {
                IERC20(order.tokenIn).transfer(order.owner, remaining);
            }
            
            order.status = OrderStatus.CANCELLED;
            
            emit OrderCancelled(orderId, address(this), "IOC expired");
        }
    }
    
    function _executeTriggeredOrder(uint256 orderId, uint256 currentPrice) internal {
        Order storage order = orders[orderId];
        
        // Execute the swap
        uint256 fillAmount = order.amountIn - order.filledAmount;
        uint256 outputAmount = _calculateOutput(fillAmount, currentPrice, order.amountOutMin);
        
        order.filledAmount = order.amountIn;
        order.status = OrderStatus.FILLED;
        
        IERC20(order.tokenOut).transfer(order.owner, outputAmount);
        
        emit OrderFilled(orderId, fillAmount, currentPrice);
    }
    
    // ============ View Functions ============
    
    function getOrder(uint256 orderId) external view returns (Order memory) {
        return orders[orderId];
    }
    
    function getUserOrders(address user) external view returns (uint256[] memory) {
        return userOrders[user];
    }
    
    function getOrdersByStatus(OrderStatus status) external view returns (uint256[] memory) {
        uint256[] memory result = new uint256[](orderCount);
        uint256 count = 0;
        
        for (uint256 i = 1; i <= orderCount; i++) {
            if (orders[i].status == status) {
                result[count] = i;
                count++;
            }
        }
        
        // Resize array
        uint256[] memory finalResult = new uint256[](count);
        for (uint256 i = 0; i < count; i++) {
            finalResult[i] = result[i];
        }
        
        return finalResult;
    }
    
    // ============ Admin Functions ============
    
    function setExecutor(address executor, bool allowed) external {
        executors[executor] = allowed;
    }
    
    function setPriceOracle(address oracle) external {
        priceOracle = oracle;
    }
    
    function setFee(uint256 newFee) external {
        require(newFee < 1000, "Fee too high"); // Max 10%
        limitOrderFee = newFee;
    }
}

// ============ IERC20 ============
interface IERC20 {
    function transfer(address to, uint256 amount) external returns (bool);
    function transferFrom(address from, address to, uint256 amount) external returns (bool);
    function balanceOf(address account) external view returns (uint256);
}
