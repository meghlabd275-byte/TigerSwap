// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "../libraries/SafeMath.sol";

/**
 * @title TigerAdvancedOrders
 * @notice Advanced Order Types - Stop Loss, Take Profit, TWAP
 * @dev Provides sophisticated order types for risk management
 * 
 * Features:
 * - Stop Loss: Auto-exit when price drops below threshold
 * - Take Profit: Auto-exit when price reaches target
 * - Stop Limit: Stop order with limit price
 * - TWAP: Time-Weighted Average orders for large trades
 * - Trailing Stop: Dynamic stop based on price movement
 */
contract TigerAdvancedOrders {
    using SafeMath for uint256;

    // ============== Constants ==============
    
    // Order types
    uint8 constant ORDER_TYPE_STOP_LOSS = 1;
    uint8 constant ORDER_TYPE_TAKE_PROFIT = 2;
    uint8 constant ORDER_TYPE_STOP_LIMIT = 3;
    uint8 constant ORDER_TYPE_TWAP = 4;
    uint8 constant ORDER_TYPE_TRAILING_STOP = 5;
    
    // Order status
    uint8 constant STATUS_PENDING = 1;
    uint8 constant STATUS_TRIGGERED = 2;
    uint8 constant STATUS_EXECUTED = 3;
    uint8 constant STATUS_CANCELLED = 4;
    uint8 constant STATUS_EXPIRED = 5;
    
    // ============== State Variables ==============
    
    // Order storage
    mapping(bytes32 => AdvancedOrder) public orders;
    mapping(bytes32 => uint256) public orderIndices;
    bytes32[] public orderIds;
    
    // TWAP execution state
    mapping(bytes32 => TWAPState) public twapStates;
    
    // Price oracle reference
    address public priceOracle;
    
    // Executor reference
    address public executor;
    
    // Governance
    address public governance;
    
    // ============== Structs ==============
    
    struct AdvancedOrder {
        address owner;
        bytes32 marketId;
        uint8 orderType;
        address tokenIn;
        address tokenOut;
        uint256 amountIn;
        uint256 triggerPrice;
        uint256 limitPrice;
        uint256 stopPrice;
        uint256 filledAmount;
        uint256 timestamp;
        uint256 expiry;
        uint256 interval;          // For TWAP: interval between executions
        uint256 numIntervals;    // For TWAP: total intervals
        uint256 nextExecutionTime;
        uint256 trailingStop;    // For trailing stop: trailing distance
        uint256 highestPrice;    // For trailing stop: highest price since order
        uint256 status;
    }
    
    struct TWAPState {
        bytes32 orderId;
        uint256 lastExecutionTime;
        uint256 totalFilled;
        uint256 executionsCompleted;
        uint256 lastExecutionPrice;
    }
    
    // ============== Events ==============
    
    event AdvancedOrderCreated(
        bytes32 indexed orderId,
        address indexed owner,
        uint8 orderType,
        bytes32 marketId,
        uint256 triggerPrice,
        uint256 amountIn
    );
    
    event AdvancedOrderTriggered(
        bytes32 indexed orderId,
        address indexed owner,
        uint256 triggerPrice,
        uint256 executionPrice
    );
    
    event AdvancedOrderExecuted(
        bytes32 indexed orderId,
        address indexed owner,
        uint256 amountIn,
        uint256 amountOut,
        uint256 executionPrice
    );
    
    event AdvancedOrderCancelled(
        bytes32 indexed orderId,
        address indexed owner,
        string reason
    );
    
    event TWAPExecution(
        bytes32 indexed orderId,
        uint256 executionIndex,
        uint256 amount,
        uint256 price
    );
    
    // ============== Constructor ==============
    
    constructor() {
        governance = msg.sender;
    }
    
    // ============== Configuration ==============
    
    /**
     * @notice Set price oracle
     */
    function setPriceOracle(address _oracle) external {
        require(msg.sender == governance, "ONLY_GOVERNANCE");
        priceOracle = _oracle;
    }
    
    /**
     * @notice Set executor
     */
    function setExecutor(address _executor) external {
        require(msg.sender == governance, "ONLY_GOVERNANCE");
        executor = _executor;
    }
    
    // ============== Order Creation ==============
    
    /**
     * @notice Create stop loss order
     */
    function createStopLossOrder(
        bytes32 _marketId,
        address _tokenIn,
        address _tokenOut,
        uint256 _amountIn,
        uint256 _stopPrice,
        uint256 _expiry
    ) external returns (bytes32 orderId) {
        require(_stopPrice > 0, "INVALID_STOP_PRICE");
        require(_amountIn > 0, "INVALID_AMOUNT");
        
        orderId = keccak256(abi.encodePacked(
            msg.sender,
            _marketId,
            ORDER_TYPE_STOP_LOSS,
            _stopPrice,
            _amountIn,
            block.timestamp,
            orderIds.length
        ));
        
        orders[orderId] = AdvancedOrder({
            owner: msg.sender,
            marketId: _marketId,
            orderType: ORDER_TYPE_STOP_LOSS,
            tokenIn: _tokenIn,
            tokenOut: _tokenOut,
            amountIn: _amountIn,
            triggerPrice: _stopPrice,
            limitPrice: 0,
            stopPrice: _stopPrice,
            filledAmount: 0,
            timestamp: block.timestamp,
            expiry: _expiry > 0 ? _expiry : block.timestamp + 365 days,
            interval: 0,
            numIntervals: 0,
            nextExecutionTime: 0,
            trailingStop: 0,
            highestPrice: 0,
            status: STATUS_PENDING
        });
        
        orderIds.push(orderId);
        
        emit AdvancedOrderCreated(
            orderId,
            msg.sender,
            ORDER_TYPE_STOP_LOSS,
            _marketId,
            _stopPrice,
            _amountIn
        );
    }
    
    /**
     * @notice Create take profit order
     */
    function createTakeProfitOrder(
        bytes32 _marketId,
        address _tokenIn,
        address _tokenOut,
        uint256 _amountIn,
        uint256 _targetPrice,
        uint256 _expiry
    ) external returns (bytes32 orderId) {
        require(_targetPrice > 0, "INVALID_TARGET_PRICE");
        require(_amountIn > 0, "INVALID_AMOUNT");
        
        orderId = keccak256(abi.encodePacked(
            msg.sender,
            _marketId,
            ORDER_TYPE_TAKE_PROFIT,
            _targetPrice,
            _amountIn,
            block.timestamp,
            orderIds.length
        ));
        
        orders[orderId] = AdvancedOrder({
            owner: msg.sender,
            marketId: _marketId,
            orderType: ORDER_TYPE_TAKE_PROFIT,
            tokenIn: _tokenIn,
            tokenOut: _tokenOut,
            amountIn: _amountIn,
            triggerPrice: _targetPrice,
            limitPrice: _targetPrice,
            stopPrice: 0,
            filledAmount: 0,
            timestamp: block.timestamp,
            expiry: _expiry > 0 ? _expiry : block.timestamp + 365 days,
            interval: 0,
            numIntervals: 0,
            nextExecutionTime: 0,
            trailingStop: 0,
            highestPrice: 0,
            status: STATUS_PENDING
        });
        
        orderIds.push(orderId);
        
        emit AdvancedOrderCreated(
            orderId,
            msg.sender,
            ORDER_TYPE_TAKE_PROFIT,
            _marketId,
            _targetPrice,
            _amountIn
        );
    }
    
    /**
     * @notice Create stop limit order
     */
    function createStopLimitOrder(
        bytes32 _marketId,
        address _tokenIn,
        address _tokenOut,
        uint256 _amountIn,
        uint256 _stopPrice,
        uint256 _limitPrice,
        uint256 _expiry
    ) external returns (bytes32 orderId) {
        require(_stopPrice > 0, "INVALID_STOP_PRICE");
        require(_limitPrice > 0, "INVALID_LIMIT_PRICE");
        require(_amountIn > 0, "INVALID_AMOUNT");
        
        orderId = keccak256(abi.encodePacked(
            msg.sender,
            _marketId,
            ORDER_TYPE_STOP_LIMIT,
            _stopPrice,
            _limitPrice,
            _amountIn,
            block.timestamp,
            orderIds.length
        ));
        
        orders[orderId] = AdvancedOrder({
            owner: msg.sender,
            marketId: _marketId,
            orderType: ORDER_TYPE_STOP_LIMIT,
            tokenIn: _tokenIn,
            tokenOut: _tokenOut,
            amountIn: _amountIn,
            triggerPrice: _stopPrice,
            limitPrice: _limitPrice,
            stopPrice: _stopPrice,
            filledAmount: 0,
            timestamp: block.timestamp,
            expiry: _expiry > 0 ? _expiry : block.timestamp + 365 days,
            interval: 0,
            numIntervals: 0,
            nextExecutionTime: 0,
            trailingStop: 0,
            highestPrice: 0,
            status: STATUS_PENDING
        });
        
        orderIds.push(orderId);
        
        emit AdvancedOrderCreated(
            orderId,
            msg.sender,
            ORDER_TYPE_STOP_LIMIT,
            _marketId,
            _stopPrice,
            _amountIn
        );
    }
    
    /**
     * @notice Create TWAP order
     */
    function createTWAPOrder(
        bytes32 _marketId,
        address _tokenIn,
        address _tokenOut,
        uint256 _totalAmount,
        uint256 _intervalSeconds,
        uint256 _numIntervals,
        uint256 _startDelay
    ) external returns (bytes32 orderId) {
        require(_totalAmount > 0, "INVALID_AMOUNT");
        require(_intervalSeconds > 0, "INVALID_INTERVAL");
        require(_numIntervals > 0, "INVALID_NUM_INTERVALS");
        require(_numIntervals <= 1000, "TOO_MANY_INTERVALS");
        
        orderId = keccak256(abi.encodePacked(
            msg.sender,
            _marketId,
            ORDER_TYPE_TWAP,
            _totalAmount,
            _intervalSeconds,
            _numIntervals,
            block.timestamp,
            orderIds.length
        ));
        
        uint256 firstExecution = block.timestamp + _startDelay;
        
        orders[orderId] = AdvancedOrder({
            owner: msg.sender,
            marketId: _marketId,
            orderType: ORDER_TYPE_TWAP,
            tokenIn: _tokenIn,
            tokenOut: _tokenOut,
            amountIn: _totalAmount,
            triggerPrice: 0,
            limitPrice: 0,
            stopPrice: 0,
            filledAmount: 0,
            timestamp: block.timestamp,
            expiry: block.timestamp + (_intervalSeconds * _numIntervals) + 86400,
            interval: _intervalSeconds,
            numIntervals: _numIntervals,
            nextExecutionTime: firstExecution,
            trailingStop: 0,
            highestPrice: 0,
            status: STATUS_PENDING
        });
        
        // Initialize TWAP state
        twapStates[orderId] = TWAPState({
            orderId: orderId,
            lastExecutionTime: 0,
            totalFilled: 0,
            executionsCompleted: 0,
            lastExecutionPrice: 0
        });
        
        orderIds.push(orderId);
        
        emit AdvancedOrderCreated(
            orderId,
            msg.sender,
            ORDER_TYPE_TWAP,
            _marketId,
            0,
            _totalAmount
        );
    }
    
    /**
     * @notice Create trailing stop order
     */
    function createTrailingStopOrder(
        bytes32 _marketId,
        address _tokenIn,
        address _tokenOut,
        uint256 _amountIn,
        uint256 _trailingDistance,
        uint256 _expiry
    ) external returns (bytes32 orderId) {
        require(_trailingDistance > 0, "INVALID_TRAILING_DISTANCE");
        require(_amountIn > 0, "INVALID_AMOUNT");
        
        // Get current price
        uint256 currentPrice = _getCurrentPrice(_marketId);
        
        orderId = keccak256(abi.encodePacked(
            msg.sender,
            _marketId,
            ORDER_TYPE_TRAILING_STOP,
            _trailingDistance,
            _amountIn,
            block.timestamp,
            orderIds.length
        ));
        
        orders[orderId] = AdvancedOrder({
            owner: msg.sender,
            marketId: _marketId,
            orderType: ORDER_TYPE_TRAILING_STOP,
            tokenIn: _tokenIn,
            tokenOut: _tokenOut,
            amountIn: _amountIn,
            triggerPrice: 0,
            limitPrice: 0,
            stopPrice: 0,
            filledAmount: 0,
            timestamp: block.timestamp,
            expiry: _expiry > 0 ? _expiry : block.timestamp + 365 days,
            interval: 0,
            numIntervals: 0,
            nextExecutionTime: 0,
            trailingStop: _trailingDistance,
            highestPrice: currentPrice,
            status: STATUS_PENDING
        });
        
        orderIds.push(orderId);
        
        emit AdvancedOrderCreated(
            orderId,
            msg.sender,
            ORDER_TYPE_TRAILING_STOP,
            _marketId,
            _trailingDistance,
            _amountIn
        );
    }
    
    // ============== Order Cancellation ==============
    
    /**
     * @notice Cancel an order
     */
    function cancelOrder(bytes32 _orderId) external {
        AdvancedOrder storage order = orders[_orderId];
        require(order.owner == msg.sender, "NOT_OWNER");
        require(order.status == STATUS_PENDING, "NOT_PENDING");
        
        order.status = STATUS_CANCELLED;
        
        emit AdvancedOrderCancelled(_orderId, msg.sender, "USER_CANCEL");
    }
    
    // ============== Order Execution ==============
    
    /**
     * @notice Execute triggered orders (called by keeper/executor)
     */
    function executeOrders(bytes32[] calldata _orderIds) external {
        require(msg.sender == executor || msg.sender == governance, "ONLY_EXECUTOR");
        
        for (uint256 i = 0; i < _orderIds.length; i++) {
            _checkAndExecute(_orderIds[i]);
        }
    }
    
    /**
     * @notice Check and execute single order
     */
    function _checkAndExecute(bytes32 _orderId) internal {
        AdvancedOrder storage order = orders[_orderId];
        
        if (order.status != STATUS_PENDING) return;
        if (block.timestamp > order.expiry) {
            order.status = STATUS_EXPIRED;
            return;
        }
        
        uint256 currentPrice = _getCurrentPrice(order.marketId);
        
        if (order.orderType == ORDER_TYPE_STOP_LOSS) {
            // Execute if price drops to or below stop
            if (currentPrice <= order.triggerPrice) {
                _executeOrder(_orderId, currentPrice);
            }
        } else if (order.orderType == ORDER_TYPE_TAKE_PROFIT) {
            // Execute if price reaches or exceeds target
            if (currentPrice >= order.triggerPrice) {
                _executeOrder(_orderId, currentPrice);
            }
        } else if (order.orderType == ORDER_TYPE_STOP_LIMIT) {
            // Check stop trigger first
            if (currentPrice <= order.triggerPrice) {
                order.status = STATUS_TRIGGERED;
            }
            // Then check limit execution
            if (order.status == STATUS_TRIGGERED && currentPrice <= order.limitPrice) {
                _executeOrder(_orderId, currentPrice);
            }
        } else if (order.orderType == ORDER_TYPE_TWAP) {
            _executeTWAP(_orderId, currentPrice);
        } else if (order.orderType == ORDER_TYPE_TRAILING_STOP) {
            _executeTrailingStop(_orderId, currentPrice);
        }
    }
    
    /**
     * @notice Execute TWAP order
     */
    function _executeTWAP(bytes32 _orderId, uint256 _currentPrice) internal {
        AdvancedOrder storage order = orders[_orderId];
        TWAPState storage twap = twapStates[_orderId];
        
        // Check if it's time to execute
        if (block.timestamp < order.nextExecutionTime) return;
        
        // Calculate execution amount
        uint256 execAmount = order.amountIn / order.numIntervals;
        
        // Check remaining
        if (twap.totalFilled >= order.amountIn) {
            order.status = STATUS_EXECUTED;
            return;
        }
        
        // Execute partial fill
        order.filledAmount += execAmount;
        twap.totalFilled += execAmount;
        twap.executionsCompleted++;
        twap.lastExecutionPrice = _currentPrice;
        twap.lastExecutionTime = block.timestamp;
        
        // Schedule next execution
        order.nextExecutionTime = block.timestamp + order.interval;
        
        emit TWAPExecution(
            _orderId,
            twap.executionsCompleted,
            execAmount,
            _currentPrice
        );
        
        // Mark as executed if complete
        if (twap.totalFilled >= order.amountIn) {
            order.status = STATUS_EXECUTED;
        }
    }
    
    /**
     * @notice Execute trailing stop
     */
    function _executeTrailingStop(bytes32 _orderId, uint256 _currentPrice) internal {
        AdvancedOrder storage order = orders[_orderId];
        
        // Update highest price (for long positions)
        if (_currentPrice > order.highestPrice) {
            order.highestPrice = _currentPrice;
            return;
        }
        
        // Calculate trailing stop price
        uint256 stopPrice = order.highestPrice - order.trailingStop;
        
        // Execute if price drops below trailing stop
        if (_currentPrice <= stopPrice) {
            _executeOrder(_orderId, _currentPrice);
        }
    }
    
    /**
     * @notice Execute an order
     */
    function _executeOrder(bytes32 _orderId, uint256 _executionPrice) internal {
        AdvancedOrder storage order = orders[_orderId];
        
        // Mark as executed
        order.status = STATUS_EXECUTED;
        
        // Emit events
        emit AdvancedOrderTriggered(_orderId, order.owner, order.triggerPrice, _executionPrice);
        emit AdvancedOrderExecuted(
            _orderId,
            order.owner,
            order.amountIn,
            0, // amountOut would be calculated by router
            _executionPrice
        );
    }
    
    // ============== View Functions ==============
    
    /**
     * @notice Get current price (should integrate with oracle)
     */
    function _getCurrentPrice(bytes32 _marketId) internal view returns (uint256) {
        // Should integrate with price oracle
        return 1000 * 1e8;
    }
    
    /**
     * @notice Get order details
     */
    function getOrder(bytes32 _orderId) external view returns (
        address owner,
        uint8 orderType,
        bytes32 marketId,
        uint256 amountIn,
        uint256 triggerPrice,
        uint256 filledAmount,
        uint256 status,
        uint256 nextExecutionTime
    ) {
        AdvancedOrder storage order = orders[_orderId];
        return (
            order.owner,
            order.orderType,
            order.marketId,
            order.amountIn,
            order.triggerPrice,
            order.filledAmount,
            order.status,
            order.nextExecutionTime
        );
    }
    
    /**
     * @notice Get TWAP state
     */
    function getTWAPState(bytes32 _orderId) external view returns (
        uint256 totalFilled,
        uint256 executionsCompleted,
        uint256 lastExecutionPrice
    ) {
        TWAPState storage twap = twapStates[_orderId];
        return (
            twap.totalFilled,
            twap.executionsCompleted,
            twap.lastExecutionPrice
        );
    }
    
    /**
     * @notice Get pending orders for a user
     */
    function getPendingOrders(address _owner) external view returns (bytes32[] memory) {
        uint256 count = 0;
        for (uint256 i = 0; i < orderIds.length; i++) {
            if (orders[orderIds[i]].owner == _owner && orders[orderIds[i]].status == STATUS_PENDING) {
                count++;
            }
        }
        
        bytes32[] memory result = new bytes32[](count);
        count = 0;
        for (uint256 i = 0; i < orderIds.length; i++) {
            if (orders[orderIds[i]].owner == _owner && orders[orderIds[i]].status == STATUS_PENDING) {
                result[count++] = orderIds[i];
            }
        }
        
        return result;
    }
}