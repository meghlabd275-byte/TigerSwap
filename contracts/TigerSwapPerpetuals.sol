// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/**
 * @title TigerSwapPerpetuals
 * @notice Perpetual futures trading contract
 */
contract TigerSwapPerpetuals {
    // Constants
    uint256 constant PRICE_PRECISION = 1e8;
    uint256 constant LEVERAGE_PRECISION = 1e18;
    uint256 constant FUNDING_RATE_PRECISION = 1e6;
    
    // State variables
    address public owner;
    address public priceOracle;
    address public marginManager;
    
    // Market configuration
    struct Market {
        address token;
        uint256 maxLeverage;
        uint256 liquidationFee;
        uint256 maintenanceMargin;
        uint256 initialMargin;
        bool isActive;
    }
    
    // Position tracking
    struct Position {
        bytes32 marketId;
        address trader;
        uint256 size;
        uint256 collateral;
        uint256 entryPrice;
        uint256 lastUpdated;
        bool isLong;
    }
    
    // Funding rate tracking
    struct FundingInfo {
        int256 lastFundingRate;
        uint256 lastUpdateTime;
        uint256 markPrice;
        uint256 indexPrice;
    }
    
    // State mappings
    mapping(bytes32 => Market) public markets;
    mapping(bytes32 => Position) public positions;
    mapping(bytes32 => FundingInfo) public fundingInfo;
    
    // Events
    event MarketCreated(bytes32 indexed marketId, address token, uint256 maxLeverage);
    event PositionOpened(bytes32 indexed marketId, address indexed trader, uint256 size, uint256 collateral, bool isLong);
    event PositionClosed(bytes32 indexed marketId, address indexed trader, uint256 pnl);
    event PositionLiquidated(bytes32 indexed marketId, address indexed trader, uint256 liquidationFee);
    event FundingRateUpdated(bytes32 indexed marketId, int256 fundingRate);
    event PriceUpdated(bytes32 indexed marketId, uint256 price);
    
    // Modifiers
    modifier onlyOwner() {
        require(msg.sender == owner, "TigerSwap: NOT_OWNER");
        _;
    }
    
    modifier onlyOracle() {
        require(msg.sender == priceOracle, "TigerSwap: NOT_ORACLE");
        _;
    }
    
    constructor() {
        owner = msg.sender;
    }
    
    /**
     * @notice Create a new perpetual market
     * @param marketId Unique market identifier
     * @param token Trading token address
     * @param maxLeverage Maximum allowed leverage
     * @param liquidationFee Fee percentage for liquidation (in wei)
     * @param maintenanceMargin Maintenance margin ratio
     * @param initialMargin Initial margin ratio
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
        
        markets[marketId] = Market({
            token: token,
            maxLeverage: maxLeverage,
            liquidationFee: liquidationFee,
            maintenanceMargin: maintenanceMargin,
            initialMargin: initialMargin,
            isActive: true
        });
        
        emit MarketCreated(marketId, token, maxLeverage);
    }
    
    /**
     * @notice Open a position
     * @param marketId Market identifier
     * @param size Position size
     * @param collateral Collateral amount
     * @param isLong Whether position is long
     */
    function openPosition(
        bytes32 marketId,
        uint256 size,
        uint256 collateral,
        bool isLong
    ) external returns (bytes32 positionId) {
        Market memory market = markets[marketId];
        require(market.isActive, "TigerSwap: MARKET_NOT_ACTIVE");
        
        // Get current price
        uint256 currentPrice = getMarkPrice(marketId);
        require(currentPrice > 0, "TigerSwap: INVALID_PRICE");
        
        // Calculate position value
        uint256 positionValue = size * currentPrice / PRICE_PRECISION;
        
        // Calculate required margin
        uint256 requiredMargin = positionValue * market.initialMargin / LEVERAGE_PRECISION;
        require(collateral >= requiredMargin, "TigerSwap: INSUFFICIENT_MARGIN");
        
        // Calculate leverage
        uint256 leverage = positionValue * LEVERAGE_PRECISION / collateral;
        require(leverage <= market.maxLeverage, "TigerSwap: LEVERAGE_TOO_HIGH");
        
        // Generate position ID
        positionId = keccak256(abi.encodePacked(marketId, msg.sender, block.timestamp));
        
        // Create position
        positions[positionId] = Position({
            marketId: marketId,
            trader: msg.sender,
            size: size,
            collateral: collateral,
            entryPrice: currentPrice,
            lastUpdated: block.timestamp,
            isLong: isLong
        });
        
        // Transfer collateral
        IERC20(market.token).transferFrom(msg.sender, address(this), collateral);
        
        emit PositionOpened(marketId, msg.sender, size, collateral, isLong);
    }
    
    /**
     * @notice Close a position
     * @param positionId Position identifier
     */
    function closePosition(bytes32 positionId) external {
        Position storage position = positions[positionId];
        require(position.trader == msg.sender, "TigerSwap: NOT_TRADER");
        require(position.size > 0, "TigerSwap: POSITION_NOT_EXISTS");
        
        bytes32 marketId = position.marketId;
        Market memory market = markets[marketId];

        // Get current price
        uint256 currentPrice = getMarkPrice(marketId);
        
        // Calculate PnL
        int256 pnl = calculatePnL(position, currentPrice);
        
        // Calculate final collateral
        uint256 finalCollateral;
        if (pnl > 0) {
            finalCollateral = position.collateral + uint256(pnl);
        } else {
            finalCollateral = position.collateral - uint256(-pnl);
            require(finalCollateral > 0, "TigerSwap: LIQUIDATED");
        }
        
        // Transfer back to trader
        IERC20(market.token).transfer(msg.sender, finalCollateral);
        
        // Delete position
        delete positions[positionId];
        
        emit PositionClosed(marketId, msg.sender, uint256(pnl));
    }
    
    /**
     * @notice Liquidate a position
     * @param positionId Position to liquidate
     */
    function liquidatePosition(bytes32 positionId) external {
        Position storage position = positions[positionId];
        require(position.size > 0, "TigerSwap: POSITION_NOT_EXISTS");
        
        bytes32 marketId = getMarketId(positionId);
        Market memory market = markets[marketId];
        
        // Get current price
        uint256 currentPrice = getMarkPrice(marketId);
        
        // Calculate margin ratio against the signed position value. A losing
        // position has less equity, so losses must reduce the numerator.
        int256 pnl = calculatePnL(position, currentPrice);
        int256 equity = int256(position.collateral) + pnl;
        uint256 positionValue = position.size * currentPrice / PRICE_PRECISION;
        require(positionValue > 0, "TigerSwap: INVALID_POSITION");

        // A position is liquidatable when equity is negative or the margin
        // ratio falls below the maintenance margin.
        bool belowMaintenance = equity < 0
            || uint256(equity) * LEVERAGE_PRECISION / positionValue < market.maintenanceMargin;
        require(belowMaintenance, "TigerSwap: CANNOT_LIQUIDATE");

        // The liquidator receives a fee capped at the remaining equity.
        uint256 remainingEquity = equity > 0 ? uint256(equity) : 0;
        uint256 liquidationFee = position.collateral * market.liquidationFee / LEVERAGE_PRECISION;
        if (liquidationFee > remainingEquity) {
            liquidationFee = remainingEquity;
        }
        address trader = position.trader;

        // Transfer the fee to the liquidator and the remainder to the trader.
        IERC20(market.token).transfer(msg.sender, liquidationFee);
        if (remainingEquity > liquidationFee) {
            IERC20(market.token).transfer(trader, remainingEquity - liquidationFee);
        }

        // Delete position
        delete positions[positionId];

        emit PositionLiquidated(marketId, trader, liquidationFee);
    }
    
    /**
     * @notice Add margin to position
     * @param positionId Position identifier
     * @param amount Amount to add
     */
    function addMargin(bytes32 positionId, uint256 amount) external {
        Position storage position = positions[positionId];
        require(position.size > 0, "TigerSwap: POSITION_NOT_EXISTS");
        require(position.trader == msg.sender, "TigerSwap: NOT_TRADER");
        require(amount > 0, "TigerSwap: ZERO_AMOUNT");
        
        bytes32 marketId = getMarketId(positionId);
        Market memory market = markets[marketId];
        
        // Transfer tokens
        IERC20(market.token).transferFrom(msg.sender, address(this), amount);
        
        // Update collateral
        position.collateral += amount;
        position.lastUpdated = block.timestamp;
    }
    
    /**
     * @notice Remove margin from position
     * @param positionId Position identifier
     * @param amount Amount to remove
     */
    function removeMargin(bytes32 positionId, uint256 amount) external {
        Position storage position = positions[positionId];
        require(position.size > 0, "TigerSwap: POSITION_NOT_EXISTS");
        require(position.trader == msg.sender, "TigerSwap: NOT_TRADER");
        require(amount > 0 && amount < position.collateral, "TigerSwap: INVALID_AMOUNT");
        
        bytes32 marketId = getMarketId(positionId);
        Market memory market = markets[marketId];
        
        // Get current price
        uint256 currentPrice = getMarkPrice(marketId);
        
        // Calculate new collateral
        uint256 newCollateral = position.collateral - amount;
        
        // Check if above minimum margin
        uint256 positionValue = position.size * currentPrice / PRICE_PRECISION;
        uint256 newMarginRatio = newCollateral * LEVERAGE_PRECISION / positionValue;
        require(newMarginRatio >= market.initialMargin, "TigerSwap: MARGIN_TOO_LOW");
        
        // Update position
        position.collateral = newCollateral;
        position.lastUpdated = block.timestamp;
        
        // Transfer tokens back
        IERC20(market.token).transfer(msg.sender, amount);
    }
    
    /**
     * @notice Update funding rate
     * @param marketId Market identifier
     * @param markPrice Current mark price
     * @param indexPrice Current index price
     */
    function updateFundingRate(bytes32 marketId, uint256 markPrice, uint256 indexPrice) external onlyOracle {
        FundingInfo storage info = fundingInfo[marketId];
        
        // Calculate funding rate based on price difference
        int256 priceDiff = int256(markPrice) - int256(indexPrice);
        int256 fundingRate = (priceDiff * int256(FUNDING_RATE_PRECISION)) / int256(indexPrice);
        
        info.lastFundingRate = fundingRate;
        info.lastUpdateTime = block.timestamp;
        info.markPrice = markPrice;
        info.indexPrice = indexPrice;
        
        emit FundingRateUpdated(marketId, fundingRate);
    }
    
    /**
     * @notice Update price for a market
     * @param marketId Market identifier
     * @param price New price
     */
    function updatePrice(bytes32 marketId, uint256 price) external onlyOracle {
        FundingInfo storage info = fundingInfo[marketId];
        info.markPrice = price;
        
        emit PriceUpdated(marketId, price);
    }
    
    /**
     * @notice Calculate PnL for a position
     */
    function calculatePnL(Position memory position, uint256 currentPrice) internal pure returns (int256) {
        if (position.size == 0) return 0;
        
        int256 priceDiff = int256(currentPrice) - int256(position.entryPrice);
        int256 pnl = position.isLong ? priceDiff * int256(position.size) : -priceDiff * int256(position.size);
        
        return pnl / int256(PRICE_PRECISION);
    }
    
    /**
     * @notice Get position details
     */
    function getPosition(bytes32 positionId) external view returns (
        bytes32 marketId,
        address trader,
        uint256 size,
        uint256 collateral,
        uint256 entryPrice,
        uint256 lastUpdated,
        bool isLong
    ) {
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
    
    /**
     * @notice Get mark price for a market
     */
    function getMarkPrice(bytes32 marketId) public view returns (uint256) {
        uint256 markPrice = fundingInfo[marketId].markPrice;
        require(markPrice > 0, "TigerSwap: MARKET_PRICE_NOT_SET");
        return markPrice;
    }
    
    /**
     * @notice Get market info
     */
    function getMarket(bytes32 marketId) external view returns (
        address token,
        uint256 maxLeverage,
        uint256 liquidationFee,
        uint256 maintenanceMargin,
        uint256 initialMargin,
        bool isActive
    ) {
        Market memory market = markets[marketId];
        return (
            market.token,
            market.maxLeverage,
            market.liquidationFee,
            market.maintenanceMargin,
            market.initialMargin,
            market.isActive
        );
    }
    
    /**
     * @notice Get funding info
     */
    function getFundingInfo(bytes32 marketId) external view returns (
        int256 lastFundingRate,
        uint256 lastUpdateTime,
        uint256 markPrice,
        uint256 indexPrice
    ) {
        FundingInfo memory info = fundingInfo[marketId];
        return (
            info.lastFundingRate,
            info.lastUpdateTime,
            info.markPrice,
            info.indexPrice
        );
    }
    
    /**
     * @notice Get the market id a position belongs to
     */
    function getMarketId(bytes32 positionId) internal view returns (bytes32) {
        return positions[positionId].marketId;
    }
    
    // --- Admin Functions ---
    
    function setOracle(address _oracle) external onlyOwner {
        priceOracle = _oracle;
    }
    
    function setMarginManager(address _marginManager) external onlyOwner {
        marginManager = _marginManager;
    }
    
    function setMarketActive(bytes32 marketId, bool isActive) external onlyOwner {
        markets[marketId].isActive = isActive;
    }
    
    function transferOwnership(address newOwner) external onlyOwner {
        owner = newOwner;
    }
}

interface IERC20 {
    function transfer(address to, uint256 value) external returns (bool);
    function transferFrom(address from, address to, uint256 value) external returns (bool);
    function balanceOf(address account) external view returns (uint256);
}
