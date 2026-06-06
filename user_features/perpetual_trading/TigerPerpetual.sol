// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/**
 * @title TigerPerpetual
 * @notice Perpetual Contract Trading (Like GMX, dYdX)
 * @dev Leveraged trading with up to 50x leverage
 * 
 * Features:
 * - Long/Short positions
 * - Up to 50x leverage
 * - PnL calculation
 * - Funding payments
 * - Liquidation
 * - Oracle price feed
 * - Stable asset pool
 */
import "../libraries/SafeMath.sol";

contract TigerPerpetual {
    using SafeMath for uint256;

    // ============================================================================
    // Constants
    // ============================================================================

    uint256 public constant MAX_LEVERAGE = 50e18;
    uint256 public constant MIN_COLLATERAL = 100e18;
    uint256 public constant LIQUIDATION_THRESHOLD = 80e18; // 80% of position value
    uint256 public constant FUNDING_INTERVAL = 8 hours;
    uint256 public constant BASIS_POINTS = 1e18;
    uint256 public constant PRICE_PRECISION = 1e18;

    // ============================================================================
    // Enums
    // ============================================================================

    enum PositionSide {
        Long,
        Short
    }

    enum OrderType {
        Market,
        Limit,
        StopLoss,
        TakeProfit
    }

    // ============================================================================
    // State Variables
    // ============================================================================

    // Global
    uint256 public positionCount = 0;
    bool public paused = false;
    address public admin;
    address public feeRecipient;
    address public priceOracle;

    // Pools
    mapping(address => uint256) public poolBalances;
    mapping(address => uint256) public poolTotalLong;
    mapping(address => uint256) public poolTotalShort;
    address[] public poolAssets;

    // Positions
    mapping(bytes32 => Position) public positions;
    bytes32[] public positionIds;

    // Orders
    mapping(bytes32 => Order) public orders;
    bytes32[] public orderIds;

    // Funding
    mapping(address => int256) public fundingRates;
    mapping(address => uint256) public lastFundingTime;

    // Liquidations
    mapping(address => uint256) public liquidationRewards;

    // ============================================================================
    // Structs
    // ============================================================================

    struct Position {
        bytes32 id;
        address owner;
        address asset;
        PositionSide side;
        uint256 size;
        uint256 collateral;
        uint256 averagePrice;
        uint256 lastFundingPayment;
        uint256 profit;
        uint256 loss;
        uint256 openTime;
        bool isLiquidated;
    }

    struct Order {
        bytes32 id;
        address owner;
        address asset;
        OrderType orderType;
        PositionSide side;
        uint256 size;
        uint256 triggerPrice;
        uint256 collateral;
        uint256 leverage;
        uint256 createdAt;
        bool isFilled;
    }

    // ============================================================================
    // Events
    // ============================================================================

    event PositionOpened(
        bytes32 indexed positionId,
        address indexed owner,
        address asset,
        PositionSide side,
        uint256 size,
        uint256 collateral,
        uint256 leverage,
        uint256 entryPrice
    );

    event PositionClosed(
        bytes32 indexed positionId,
        address indexed owner,
        uint256 pnl,
        uint256 fees
    );

    event PositionLiquidated(
        bytes32 indexed positionId,
        address indexed liquidator,
        uint256 reward
    );

    event OrderCreated(
        bytes32 indexed orderId,
        address indexed owner,
        OrderType orderType,
        address asset,
        uint256 size,
        uint256 triggerPrice
    );

    event OrderFilled(bytes32 indexed orderId, uint256 fillPrice);
    event OrderCancelled(bytes32 indexed orderId);

    event FundingUpdated(address indexed asset, int256 rate);
    event PoolFunded(address indexed asset, uint256 amount);

    // ============================================================================
    // Constructor
    // ============================================================================

    constructor(address _admin, address _feeRecipient, address _priceOracle) {
        admin = _admin;
        feeRecipient = _feeRecipient;
        priceOracle = _priceOracle;
    }

    // ============================================================================
    // Position Management
    // ============================================================================

    /**
     * @notice Open a position
     */
    function openPosition(
        address asset,
        PositionSide side,
        uint256 collateral,
        uint256 leverage,
        uint256 sizeDelta
    ) external returns (bytes32 positionId) {
        require(!paused, "Paused");
        require(collateral >= MIN_COLLATERAL, "Insufficient collateral");
        require(leverage >= BASIS_POINTS && leverage <= MAX_LEVERAGE, "Invalid leverage");
        require(sizeDelta > 0, "Invalid size");
        require(poolBalances[asset] > 0, "Asset not supported");

        // Verify pool has liquidity
        uint256 positionValue = collateral.mul(leverage) / BASIS_POINTS;
        require(poolBalances[asset] >= positionValue.mul(2), "Insufficient liquidity");

        // Get current price
        uint256 entryPrice = _getPrice(asset);
        require(entryPrice > 0, "Invalid price");

        // Create position
        positionId = keccak256(abi.encode(
            asset,
            msg.sender,
            side,
            block.timestamp,
            positionCount++
        ));

        Position storage position = positions[positionId];
        position.id = positionId;
        position.owner = msg.sender;
        position.asset = asset;
        position.side = side;
        position.size = sizeDelta;
        position.collateral = collateral;
        position.averagePrice = entryPrice;
        position.lastFundingPayment = block.timestamp;
        position.openTime = block.timestamp;

        positionIds.push(positionId);

        // Update pool
        if (side == PositionSide.Long) {
            poolTotalLong[asset] += sizeDelta;
        } else {
            poolTotalShort[asset] += sizeDelta;
        }

        emit PositionOpened(positionId, msg.sender, asset, side, sizeDelta, collateral, leverage, entryPrice);
    }

    /**
     * @notice Increase position size
     */
    function increasePosition(
        bytes32 positionId,
        uint256 sizeDelta,
        uint256 collateralDelta
    ) external returns (uint256 newSize) {
        Position storage position = positions[positionId];
        require(position.owner == msg.sender, "Not owner");
        require(!position.isLiquidated, "Liquidated");

        // Add to position
        position.size += sizeDelta;
        position.collateral += collateralDelta;

        emit PositionOpened(position.id, position.owner, position.asset, position.side, position.size, position.collateral, 0, position.averagePrice);

        return position.size;
    }

    /**
     * @notice Close position
     */
    function closePosition(bytes32 positionId) external returns (uint256 pnl, uint256 fees) {
        Position storage position = positions[positionId];
        require(position.owner == msg.sender, "Not owner");
        require(!position.isLiquidated, "Liquidated");

        // Get exit price
        uint256 exitPrice = _getPrice(position.asset);
        require(exitPrice > 0, "Invalid price");

        // Calculate PnL
        if (position.side == PositionSide.Long) {
            // Long: profit if price went up
            if (exitPrice > position.averagePrice) {
                pnl = position.size.mul(exitPrice - position.averagePrice) / exitPrice;
            } else {
                position.loss = position.size.mul(position.averagePrice - exitPrice) / exitPrice;
            }
        } else {
            // Short: profit if price went down
            if (exitPrice < position.averagePrice) {
                pnl = position.size.mul(position.averagePrice - exitPrice) / exitPrice;
            } else {
                position.loss = position.size.mul(exitPrice - position.averagePrice) / exitPrice;
            }
        }

        // Calculate fees (0.1% closing fee)
        fees = position.size / 1000;

        // Calculate total to return
        uint256 totalReturn = position.collateral;
        if (pnl > 0) {
            totalReturn = totalReturn.add(pnl);
        }
        if (position.loss > 0) {
            totalReturn = totalReturn > position.loss ? totalReturn - position.loss : 0;
        }

        // Transfer back to user (minus fees)
        poolBalances[position.asset] -= totalReturn;
        // Note: Would transfer via IERC20 in production

        // Update pool
        if (position.side == PositionSide.Long) {
            poolTotalLong[position.asset] -= position.size;
        } else {
            poolTotalShort[position.asset] -= position.size;
        }

        position.isLiquidated = true;

        emit PositionClosed(positionId, msg.sender, pnl, fees);

        return (pnl, fees);
    }

    // ============================================================================
    // Liquidation
    // ============================================================================

    /**
     * @notice Liquidate position
     */
    function liquidate(bytes32 positionId) external returns (uint256 reward) {
        Position storage position = positions[positionId];
        require(!position.isLiquidated, "Already liquidated");

        // Check if liquidatable
        uint256 currentPrice = _getPrice(position.asset);
        uint256 positionValue = position.size.mul(currentPrice) / PRICE_PRECISION;
        uint256 collateralValue = position.collateral;

        bool isLiquidatable = (position.side == PositionSide.Long && 
                     currentPrice < position.averagePrice.mul(LIQUIDATION_THRESHOLD) / BASIS_POINTS) ||
                     (position.side == PositionSide.Short && 
                     currentPrice > position.averagePrice.mul(BASIS_POINTS + BASIS_POINTS - LIQUIDATION_THRESHOLD) / BASIS_POINTS);

        require(isLiquidatable || collateralValue.mul(100) / positionValue < LIQUIDATION_THRESHOLD, "Not liquidatable");

        // Calculate reward (5% of position value)
        reward = positionValue / 20;
        liquidationRewards[msg.sender] += reward;

        // Update pool
        if (position.side == PositionSide.Long) {
            poolTotalLong[position.asset] -= position.size;
        } else {
            poolTotalShort[position.asset] -= position.size;
        }

        position.isLiquidated = true;

        emit PositionLiquidated(positionId, msg.sender, reward);
    }

    // ============================================================================
    // Orders
    // ============================================================================

    /**
     * @notice Create order
     */
    function createOrder(
        address asset,
        OrderType orderType,
        PositionSide side,
        uint256 size,
        uint256 triggerPrice,
        uint256 collateral,
        uint256 leverage
    ) external returns (bytes32 orderId) {
        require(!paused, "Paused");

        orderId = keccak256(abi.encode(
            asset,
            msg.sender,
            orderType,
            side,
            size,
            triggerPrice,
            block.timestamp
        ));

        Order storage order = orders[orderId];
        order.id = orderId;
        order.owner = msg.sender;
        order.asset = asset;
        order.orderType = orderType;
        order.side = side;
        order.size = size;
        order.triggerPrice = triggerPrice;
        order.collateral = collateral;
        order.leverage = leverage;
        order.createdAt = block.timestamp;

        orderIds.push(orderId);

        emit OrderCreated(orderId, msg.sender, orderType, asset, size, triggerPrice);
    }

    /**
     * @notice Cancel order
     */
    function cancelOrder(bytes32 orderId) external {
        Order storage order = orders[orderId];
        require(order.owner == msg.sender, "Not owner");
        require(!order.isFilled, "Already filled");

        order.isFilled = true; // Mark as cancelled
        emit OrderCancelled(orderId);
    }

    /**
     * @notice Execute orders
     */
    function executeOrders(address asset) external {
        uint256 currentPrice = _getPrice(asset);

        for (uint256 i = 0; i < orderIds.length; i++) {
            Order storage order = orders[orderIds[i]];
            if (order.asset != asset || order.isFilled) continue;

            bool shouldExecute = false;
            uint256 triggerPrice = order.triggerPrice;

            if (order.orderType == OrderType.StopLoss) {
                if (order.side == PositionSide.Long && currentPrice <= triggerPrice) shouldExecute = true;
                if (order.side == PositionSide.Short && currentPrice >= triggerPrice) shouldExecute = true;
            } else if (order.orderType == OrderType.TakeProfit) {
                if (order.side == PositionSide.Long && currentPrice >= triggerPrice) shouldExecute = true;
                if (order.side == PositionSide.Short && currentPrice <= triggerPrice) shouldExecute = true;
            } else if (order.orderType == OrderType.Limit) {
                if (order.side == PositionSide.Long && currentPrice <= triggerPrice) shouldExecute = true;
                if (order.side == PositionSide.Short && currentPrice >= triggerPrice) shouldExecute = true;
            }

            if (shouldExecute) {
                order.isFilled = true;
                openPosition(asset, order.side, order.collateral, order.leverage, order.size);
                emit OrderFilled(order.id, currentPrice);
            }
        }
    }

    // ============================================================================
    // Funding
    // ============================================================================

    /**
     * @notice Update funding rate
     */
    function updateFunding(address asset) external {
        require(block.timestamp >= lastFundingTime[asset] + FUNDING_INTERVAL, "Too early");

        // Calculate funding rate based on pool imbalance
        uint256 longDiff = poolTotalLong[asset] > poolTotalShort[asset] ? 
            poolTotalLong[asset] - poolTotalShort[asset] : 0;
        uint256 shortDiff = poolTotalShort[asset] > poolTotalLong[asset] ?
            poolTotalShort[asset] - poolTotalLong[asset] : 0;

        // Funding rate = 0.01% per hour * imbalance ratio
        if (longDiff > shortDiff) {
            fundingRates[asset] = int256(longDiff.mul(1e14) / poolTotalLong[asset]);
        } else if (shortDiff > longDiff) {
            fundingRates[asset] = -int256(shortDiff.mul(1e14) / poolTotalShort[asset]);
        }

        lastFundingTime[asset] = block.timestamp;

        emit FundingUpdated(asset, fundingRates[asset]);
    }

    /**
     * @notice Pay funding
     */
    function payFunding(bytes32 positionId) external {
        Position storage position = positions[positionId];
        require(position.owner == msg.sender, "Not owner");

        uint256 fundingPayment = fundingRates[position.asset].mul(block.timestamp - position.lastFundingPayment) / 1e18;

        if (fundingPayment > 0) {
            if (fundingRates[position.asset] > 0) {
                // Long pays short
                poolTotalLong[position.asset] -= uint256(fundingRates[position.asset]);
            } else {
                // Short pays long
                poolTotalShort[position.asset] -= uint256(-fundingRates[position.asset]);
            }
        }

        position.lastFundingPayment = block.timestamp;
    }

    // ============================================================================
    // Pool Management
    // ============================================================================

    /**
     * @notice Add liquidity to pool
     */
    function addLiquidity(address asset, uint256 amount) external {
        poolBalances[asset] += amount;
        
        // Add to assets list if new
        bool found = false;
        for (uint256 i = 0; i < poolAssets.length; i++) {
            if (poolAssets[i] == asset) {
                found = true;
                break;
            }
        }
        if (!found) {
            poolAssets.push(asset);
        }

        emit PoolFunded(asset, amount);
    }

    /**
     * @notice Remove liquidity
     */
    function removeLiquidity(address asset, uint256 amount) external {
        require(msg.sender == admin, "Not admin");
        require(poolBalances[asset] >= amount, "Insufficient liquidity");
        
        poolBalances[asset] -= amount;
    }

    // ============================================================================
    // Price
    // ============================================================================

    function _getPrice(address asset) internal view returns (uint256) {
        // Would integrate with oracle
        return 1e18; // Placeholder
    }

    function getPrice(address asset) external view returns (uint256) {
        return _getPrice(asset);
    }

    // ============================================================================
    // Admin
    // ============================================================================

    function setPaused(bool _paused) external {
        require(msg.sender == admin, "Not admin");
        paused = _paused;
    }

    function setAdmin(address _admin) external {
        require(msg.sender == admin, "Not admin");
        admin = _admin;
    }

    function setFeeRecipient(address _feeRecipient) external {
        require(msg.sender == admin, "Not admin");
        feeRecipient = _feeRecipient;
    }

    function setPriceOracle(address _priceOracle) external {
        require(msg.sender == admin, "Not admin");
        priceOracle = _priceOracle;
    }

    // ============================================================================
    // View
    // ============================================================================

    function getPosition(bytes32 positionId) external view returns (
        address owner,
        address asset,
        uint8 side,
        uint256 size,
        uint256 collateral,
        uint256 averagePrice,
        bool isLiquidated
    ) {
        Position storage p = positions[positionId];
        return (
            p.owner,
            p.asset,
            uint8(p.side),
            p.size,
            p.collateral,
            p.averagePrice,
            p.isLiquidated
        );
    }

    function getPositionPnL(bytes32 positionId) external view returns (uint256 pnl, uint256 loss) {
        Position storage p = positions[positionId];
        if (p.isLiquidated) return (0, 0);

        uint256 currentPrice = _getPrice(p.asset);
        
        if (p.side == PositionSide.Long) {
            if (currentPrice > p.averagePrice) {
                pnl = p.size.mul(currentPrice - p.averagePrice) / currentPrice;
            } else {
                loss = p.size.mul(p.averagePrice - currentPrice) / currentPrice;
            }
        } else {
            if (currentPrice < p.averagePrice) {
                pnl = p.size.mul(p.averagePrice - currentPrice) / currentPrice;
            } else {
                loss = p.size.mul(currentPrice - p.averagePrice) / currentPrice;
            }
        }
    }

    function getPoolInfo(address asset) external view returns (
        uint256 balance,
        uint256 totalLong,
        uint256 totalShort
    ) {
        return (
            poolBalances[asset],
            poolTotalLong[asset],
            poolTotalShort[asset]
        );
    }
}