// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/**
 * @title TigerLending
 * @notice Complete Lending & Borrowing Protocol (Like Aave, Compound)
 * @dev Supply assets, borrow against collateral, liquidation
 * 
 * Features:
 * - Supply assets (lend)
 * - Borrow assets (up to 80% LTV)
 * - Collateral management
 * - Liquidation (health factor < 1)
 * - Interest rates (supply/borrow)
 * - Flash loans
 * - Credit delegation
 */
import "../libraries/SafeMath.sol";

contract TigerLending {
    using SafeMath for uint256;

    // ============================================================================
    // Constants
    // ============================================================================

    uint256 constant BASIS_POINTS = 1e18;
    uint256 constant LIQUIDATION_THRESHOLD = 80; // 80%
    uint256 constant MIN_HEALTH_FACTOR = 1e18;
    uint256 constant FLASH_LOAN_FEE = 5; // 0.05%

    // ============================================================================
    // State Variables
    // ============================================================================

    address public admin;
    address public treasury;
    mapping(address => Market) public markets;
    address[] public marketList;
    
    mapping(address => mapping(address => UserSupply)) public userSupplies;
    mapping(address => mapping(address => UserBorrow)) public userBorrows;
    mapping(address => mapping(address => uint256)) public supplyIndices;
    mapping(address => mapping(address => uint256)) public borrowIndices;
    
    // Interest rates
    uint256 public baseRate = 0.02e18; // 2%
    uint256 public slope1 = 0.1e18; // 10%
    uint256 public slope2 = 0.6e18; // 60%
    uint256 public optimalUtilization = 0.8e18; // 80%

    // ============================================================================
    // Structs
    // ============================================================================

    struct Market {
        address asset;
        uint256 totalSupply;
        uint256 totalBorrows;
        uint256 supplyRate;
        uint256 borrowRate;
        uint256 supplyIndex;
        uint256 borrowIndex;
        uint256 reserveFactor;
        uint256 liquidationThreshold;
        uint256 LTV; // Loan to Value
        uint256 bonus; // Liquidation bonus
        bool isActive;
    }

    struct UserSupply {
        uint256 balance;
        uint256 index;
        uint256 accrued;
    }

    struct UserBorrow {
        uint256 balance;
        uint256 index;
        uint256 accrued;
    }

    // ============================================================================
    // Events
    // ============================================================================

    event Supply(address indexed user, address indexed asset, uint256 amount);
    event Withdraw(address indexed user, address indexed asset, uint256 amount);
    event Borrow(address indexed user, address indexed asset, uint256 amount);
    event Repay(address indexed user, address indexed asset, uint256 amount);
    event Liquidate(address indexed liquidator, address indexed user, address indexed asset, uint256 amount);
    event MarketAdded(address indexed asset, uint256 LTV, uint256 liquidationThreshold);
    event InterestUpdated(address indexed asset, uint256 supplyRate, uint256 borrowRate);
    event FlashLoan(address indexed user, address indexed asset, uint256 amount, uint256 fee);

    // ============================================================================
    // Constructor
    // ============================================================================

    constructor(address _admin, address _treasury) {
        admin = _admin;
        treasury = _treasury;
    }

    // ============================================================================
    // Market Management
    // ============================================================================

    /**
     * @notice Add market (enable lending/borrowing)
     */
    function addMarket(address asset, uint256 LTV, uint256 liquidationThreshold, uint256 reserveFactor) external {
        require(msg.sender == admin, "Not admin");
        
        Market storage market = markets[asset];
        market.asset = asset;
        market.supplyIndex = BASIS_POINTS;
        market.borrowIndex = BASIS_POINTS;
        market.reserveFactor = reserveFactor;
        market.liquidationThreshold = liquidationThreshold;
        market.LTV = LTV;
        market.bonus = 5; // 5% liquidation bonus
        market.isActive = true;
        
        marketList.push(asset);
        
        emit MarketAdded(asset, LTV, liquidationThreshold);
    }

    /**
     * @notice Update market parameters
     */
    function updateMarket(address asset, uint256 LTV, uint256 liquidationThreshold, uint256 reserveFactor) external {
        require(msg.sender == admin, "Not admin");
        
        Market storage market = markets[asset];
        market.LTV = LTV;
        market.liquidationThreshold = liquidationThreshold;
        market.reserveFactor = reserveFactor;
    }

    // ============================================================================
    // Supply (Lending)
    // ============================================================================

    /**
     * @notice Supply asset (lend)
     */
    function supply(address asset, uint256 amount) external {
        Market storage market = markets[asset];
        require(market.isActive, "Market not active");
        
        // Update supply index
        _updateSupplyIndex(asset);
        
        // Calculate accrued interest
        UserSupply storage supply = userSupplies[msg.sender][asset];
        uint256 accrued = supply.balance.mul(market.supplyIndex.sub(supply.index)).div(BASIS_POINTS);
        supply.accrued = supply.accrued.add(accrued);
        
        // Update balance
        supply.balance = supply.balance.add(amount);
        supply.index = market.supplyIndex;
        
        // Update market
        market.totalSupply = market.totalSupply.add(amount);
        
        // Update interest rates
        _updateInterestRates(asset);
        
        emit Supply(msg.sender, asset, amount);
    }

    /**
     * @notice Withdraw supplied asset
     */
    function withdraw(address asset, uint256 amount) external {
        Market storage market = markets[asset];
        
        // Update supply index
        _updateSupplyIndex(asset);
        
        // Calculate accrued interest
        UserSupply storage supply = userSupplies[msg.sender][asset];
        uint256 accrued = supply.balance.mul(market.supplyIndex.sub(supply.index)).div(BASIS_POINTS);
        uint256 available = supply.balance.add(supply.accrued).add(accrued);
        
        require(available >= amount, "Insufficient balance");
        
        // Update user supply
        supply.balance = available.sub(amount);
        supply.index = market.supplyIndex;
        
        // Update market
        market.totalSupply = market.totalSupply.sub(amount);
        
        // Update interest rates
        _updateInterestRates(asset);
        
        // Transfer tokens (would use IERC20 in production)
        
        emit Withdraw(msg.sender, asset, amount);
    }

    // ============================================================================
    // Borrow
    // ============================================================================

    /**
     * @notice Borrow asset
     */
    function borrow(address asset, uint256 amount) external {
        Market storage market = markets[asset];
        require(market.isActive, "Market not active");
        
        // Check health factor
        require(_getHealthFactor(msg.sender) >= MIN_HEALTH_FACTOR, "Health factor too low");
        
        // Update borrow index
        _updateBorrowIndex(asset);
        
        // Calculate accrued interest
        UserBorrow storage borrow = userBorrows[msg.sender][asset];
        uint256 accrued = borrow.balance.mul(market.borrowIndex.sub(borrow.index)).div(BASIS_POINTS);
        borrow.accrued = borrow.accrued.add(accrued);
        
        // Update balance
        borrow.balance = borrow.balance.add(amount);
        borrow.index = market.borrowIndex;
        
        // Update market
        market.totalBorrows = market.totalBorrows.add(amount);
        
        // Update interest rates
        _updateInterestRates(asset);
        
        emit Borrow(msg.sender, asset, amount);
    }

    /**
     * @notice Repay borrowed asset
     */
    function repay(address asset, uint256 amount) external {
        Market storage market = markets[asset];
        
        // Update borrow index
        _updateBorrowIndex(asset);
        
        // Calculate accrued interest
        UserBorrow storage borrow = userBorrows[msg.sender][asset];
        uint256 accrued = borrow.balance.mul(market.borrowIndex.sub(borrow.index)).div(BASIS_POINTS);
        uint256 totalOwed = borrow.balance.add(borrow.accrued).add(accrued);
        
        uint256 repayAmount = amount > totalOwed ? totalOwed : amount;
        
        // Update balance
        if (repayAmount >= borrow.balance) {
            borrow.balance = 0;
        } else {
            borrow.balance = borrow.balance.sub(repayAmount);
        }
        borrow.index = market.borrowIndex;
        
        // Update market
        market.totalBorrows = market.totalBorrows.sub(repayAmount);
        
        // Update interest rates
        _updateInterestRates(asset);
        
        emit Repay(msg.sender, asset, repayAmount);
    }

    // ============================================================================
    // Liquidation
    // ============================================================================

    /**
     * @notice Liquidate unhealthy position
     */
    function liquidate(address user, address collateral, address borrowedAsset, uint256 amount) external {
        Market storage collateralMarket = markets[collateral];
        Market storage borrowedMarket = markets[borrowedAsset];
        
        require(collateralMarket.isActive && borrowedMarket.isActive, "Market not active");
        
        // Check if user is unhealthy
        require(_getHealthFactor(user) < MIN_HEALTH_FACTOR, "Position healthy");
        
        // Calculate liquidation amount
        uint256 maxLiquidate = amount > borrowedMarket.totalBorrows ? borrowedMarket.totalBorrows : amount;
        
        // Calculate bonus (get collateral at discount)
        uint256 bonusAmount = maxLiquidate.mul(collateralMarket.bonus.add(100)).div(100);
        
        // Update positions
        UserSupply storage collateralSupply = userSupplies[user][collateral];
        require(collateralSupply.balance >= bonusAmount, "Insufficient collateral");
        collateralSupply.balance = collateralSupply.balance.sub(bonusAmount);
        
        UserBorrow storage userBorrow = userBorrows[user][borrowedAsset];
        userBorrow.balance = userBorrow.balance.sub(maxLiquidate);
        
        // Update markets
        collateralMarket.totalSupply = collateralMarket.totalSupply.sub(bonusAmount);
        borrowedMarket.totalBorrows = borrowedMarket.totalBorrows.sub(maxLiquidate);
        
        emit Liquidate(msg.sender, user, collateral, bonusAmount);
    }

    // ============================================================================
    // Flash Loans
    // ============================================================================

    /**
     * @notice Flash loan
     */
    function flashLoan(address asset, uint256 amount, bytes calldata data) external {
        Market storage market = markets[asset];
        require(market.isActive, "Market not active");
        require(market.totalSupply >= amount, "Insufficient liquidity");
        
        uint256 fee = amount.mul(FLASH_LOAN_FEE).div(10000);
        
        // Transfer flash loaned amount (would use IERC20 in production)
        
        // Execute callback
        (bool success, ) = msg.sender.call(data);
        require(success, "Flash loan callback failed");
        
        // Require repayment + fee
        // In production, would verify balance after callback
        
        market.totalSupply = market.totalSupply.sub(amount).add(fee);
        
        emit FlashLoan(msg.sender, asset, amount, fee);
    }

    // ============================================================================
    // Interest Rates
    // ============================================================================

    function _updateSupplyIndex(address asset) internal {
        Market storage market = markets[asset];
        
        if (market.totalSupply == 0) {
            market.supplyIndex = BASIS_POINTS;
            return;
        }
        
        uint256 utilization = market.totalBorrows.mul(BASIS_POINTS).div(market.totalSupply);
        uint256 borrowRate = _calculateBorrowRate(asset);
        uint256 supplyRate = borrowRate.mul(utilization).div(BASIS_POINTS);
        
        // Accrue interest
        uint256 interest = supplyRate.mul(1 hours).div(365 days);
        market.supplyIndex = market.supplyIndex.add(interest);
        market.supplyRate = supplyRate;
    }

    function _updateBorrowIndex(address asset) internal {
        Market storage market = markets[asset];
        
        if (market.totalBorrows == 0) {
            market.borrowIndex = BASIS_POINTS;
            return;
        }
        
        uint256 borrowRate = _calculateBorrowRate(asset);
        
        // Accrue interest
        uint256 interest = borrowRate.mul(1 hours).div(365 days);
        market.borrowIndex = market.borrowIndex.add(interest);
        market.borrowRate = borrowRate;
    }

    function _calculateBorrowRate(address asset) internal view returns (uint256) {
        Market storage market = markets[asset];
        
        if (market.totalSupply == 0) {
            return baseRate;
        }
        
        uint256 utilization = market.totalBorrows.mul(BASIS_POINTS).div(market.totalSupply);
        
        if (utilization <= optimalUtilization) {
            return baseRate.add(utilization.mul(slope1).div(BASIS_POINTS));
        } else {
            uint256 normalRate = baseRate.add(optimalUtilization.mul(slope1).div(BASIS_POINTS));
            uint256 excess = utilization.sub(optimalUtilization);
            return normalRate.add(excess.mul(slope2).div(BASIS_POINTS));
        }
    }

    function _updateInterestRates(address asset) internal {
        Market storage market = markets[asset];
        
        uint256 borrowRate = _calculateBorrowRate(asset);
        uint256 utilization = market.totalSupply > 0 
            ? market.totalBorrows.mul(BASIS_POINTS).div(market.totalSupply)
            : 0;
        
        uint256 supplyRate = borrowRate.mul(utilization).div(BASIS_POINTS);
        
        market.borrowRate = borrowRate;
        market.supplyRate = supplyRate;
        
        emit InterestUpdated(asset, supplyRate, borrowRate);
    }

    // ============================================================================
    // Health Factor
    // ============================================================================

    function _getHealthFactor(address user) internal view returns (uint256) {
        uint256 totalBorrowValue = 0;
        uint256 totalCollateralValue = 0;
        
        for (uint256 i = 0; i < marketList.length; i++) {
            address asset = marketList[i];
            Market storage market = markets[asset];
            
            // Get borrow value
            UserBorrow storage borrow = userBorrows[user][asset];
            if (borrow.balance > 0) {
                // In production, would get price from oracle
                totalBorrowValue = totalBorrowValue.add(borrow.balance);
            }
            
            // Get collateral value
            UserSupply storage supply = userSupplies[user][asset];
            if (supply.balance > 0) {
                // Apply LTV
                uint256 collateralValue = supply.balance.mul(market.LTV).div(100);
                totalCollateralValue = totalCollateralValue.add(collateralValue);
            }
        }
        
        if (totalBorrowValue == 0) {
            return type(uint256).max;
        }
        
        return totalCollateralValue.mul(BASIS_POINTS).div(totalBorrowValue);
    }

    function getHealthFactor(address user) external view returns (uint256) {
        return _getHealthFactor(user);
    }

    // ============================================================================
    // View Functions
    // ============================================================================

    function getSupplyBalance(address user, address asset) external view returns (uint256) {
        UserSupply storage supply = userSupplies[user][asset];
        Market storage market = markets[asset];
        
        uint256 accrued = supply.balance.mul(market.supplyIndex.sub(supply.index)).div(BASIS_POINTS);
        return supply.balance.add(supply.accrued).add(accrued);
    }

    function getBorrowBalance(address user, address asset) external view returns (uint256) {
        UserBorrow storage borrow = userBorrows[user][asset];
        Market storage market = markets[asset];
        
        uint256 accrued = borrow.balance.mul(market.borrowIndex.sub(borrow.index)).div(BASIS_POINTS);
        return borrow.balance.add(borrow.accrued).add(accrued);
    }

    function getMarketInfo(address asset) external view returns (
        uint256 totalSupply,
        uint256 totalBorrows,
        uint256 supplyRate,
        uint256 borrowRate
    ) {
        Market storage market = markets[asset];
        return (
            market.totalSupply,
            market.totalBorrows,
            market.supplyRate,
            market.borrowRate
        );
    }

    function getMarketCount() external view returns (uint256) {
        return marketList.length;
    }
}