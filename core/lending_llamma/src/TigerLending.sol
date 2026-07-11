// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/**
 * @title TigerLending
 * @notice Production Lending Protocol - LLAMMA Style
 * @dev Lending/borrowing with automated liquidations
 * 
 * Features:
 * - Supply/borrow with variable rates
 * - LLAMMA-style automated liquidations
 * - Collateral switching
 * - Flash loans
 * - Credit delegation
 * 
 * @author TigerSwap
 * @version 1.0.0 Production
 */

import "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";
import "@openzeppelin/contracts/utils/ReentrancyGuard.sol";
import "@openzeppelin/contracts/access/Ownable.sol";
import "@openzeppelin/contracts/access/AccessControl.sol";

/**
 * @title Lending Math
 */
library LendingMath {
    uint256 constant WAD = 1e18;
    uint256 constant RAY = 1e27;
    uint256 constant MAX_RATE = 1e27;
    
    function mul(uint256 x, uint256 y) internal pure returns (uint256) {
        return (x * y) / WAD;
    }
    
    function div(uint256 x, uint256 y) internal pure returns (uint256) {
        return (x * WAD) / y;
    }
    
    function rmul(uint256 x, uint256 y) internal pure returns (uint256) {
        return (x * y) / RAY;
    }
    
    function rdiv(uint256 x, uint256 y) internal pure returns (uint256) {
        return (x * RAY) / y;
    }
}

/**
 * @title TigerLending
 * @dev Main lending pool contract
 */
contract TigerLending is ReentrancyGuard, Ownable, AccessControl {
    using SafeERC20 for IERC20;
    using LendingMath for uint256;

    // ============ Roles ============
    bytes32 public constant LENDER = keccak256("LENDER");
    bytes32 public constant BORROWER = keccak256("BORROWER");
    bytes32 public constant LIQUIDATOR_ROLE = keccak256("LIQUIDATOR");

    // ============ Constants ============
    uint256 constant LIQUIDATION_THRESHOLD = 8500; // 85%
    uint256 constant CLOSE_FACTOR = 5000; // 50% max liquidation
    uint256 constant MAX_COLLATERAL_RATIO = 10000; // 100%
    uint256 constant FLASH_LOAN_FEE = 50; // 0.5%

    // ============ State Variables ============
    
    // Markets
    mapping(address => Market) public markets;
    address[] public marketList;
    
    // User positions
    mapping(address => mapping(address => Position)) public positions;
    
    // Flash loan
    mapping(address => uint256) public flashLoanFees;
    
    // Rate calculation
    uint256 public baseRate = 1000000005474000000000000; // 2% APY
    uint256 public slope1 = 1000000005474000000000000;
    uint256 public slope2 = 300000000821210000000000000;
    uint256 public optimalUtilization = 800000000000000000; // 80%
    
    // ============ Structs ============
    
    struct Market {
        address asset;
        uint256 totalSupply;
        uint256 totalBorrow;
        uint256 supplyRate;
        uint256 borrowRate;
        uint256 rateAccumulator;
        uint256 lastUpdate;
        uint256 collateralFactor;
        uint256 liquidationThreshold;
        uint256 supplyIndex;
        uint256 borrowIndex;
        bool active;
    }
    
    struct Position {
        uint256 supplyAmount;
        uint256 borrowAmount;
        uint256 collateralValue;
        uint256 lastUpdate;
        bool active;
    }
    
    struct UserCollateral {
        address asset;
        uint256 amount;
    }

    // ============ Events ============
    event Supply(address indexed user, address indexed asset, uint256 amount);
    event Withdraw(address indexed user, address indexed asset, uint256 amount);
    event Borrow(address indexed user, address indexed asset, uint256 amount);
    event Repay(address indexed user, address indexed asset, uint256 amount);
    event Liquidate(
        address indexed liquidator,
        address indexed user,
        address indexed repayAsset,
        address collateralAsset,
        uint256 repayAmount,
        uint256 collateralAmount
    );
    event MarketAdded(address indexed asset, uint256 collateralFactor);
    event RateUpdated(uint256 newBaseRate, uint256 newSlope1, uint256 newSlope2);
    event FlashLoan(address indexed user, address indexed asset, uint256 amount, uint256 fee);

    // ============ Constructor ============
    
    constructor(address _owner) Ownable(_owner) {
        _grantRole(DEFAULT_ADMIN_ROLE, _owner);
        _grantRole(LENDER, _owner);
        _grantRole(BORROWER, _owner);
    }

    // ============ Market Functions ============

    /**
     * @notice Add a new market
     */
    function addMarket(
        address _asset,
        uint256 _collateralFactor,
        uint256 _liquidationThreshold
    ) external onlyOwner {
        require(_asset != address(0), "Invalid asset");
        require(!markets[_asset].active, "Market already exists");
        
        markets[_asset] = Market({
            asset: _asset,
            totalSupply: 0,
            totalBorrow: 0,
            supplyRate: 0,
            borrowRate: 0,
            rateAccumulator: LendingMath.RAY,
            lastUpdate: block.timestamp,
            collateralFactor: _collateralFactor,
            liquidationThreshold: _liquidationThreshold,
            supplyIndex: LendingMath.WAD,
            borrowIndex: LendingMath.RAY,
            active: true
        });
        
        marketList.push(_asset);
        
        emit MarketAdded(_asset, _collateralFactor);
    }

    // ============ Supply Functions ============

    /**
     * @notice Supply assets to earn interest
     */
    function supply(address _asset, uint256 _amount) external nonReentrant {
        Market storage market = markets[_asset];
        require(market.active, "Market not active");
        
        // Update rates
        _updateRates(market);
        
        // Calculate interest
        uint256 interest = _calculateSupplyInterest(market, msg.sender);
        
        // Transfer tokens
        IERC20(_asset).safeTransferFrom(msg.sender, address(this), _amount);
        
        // Update position
        Position storage position = positions[msg.sender][_asset];
        position.supplyAmount += _amount + interest;
        position.active = true;
        
        // Update market
        market.totalSupply += _amount;
        
        emit Supply(msg.sender, _asset, _amount);
    }

    /**
     * @notice Withdraw supplied assets
     */
    function withdraw(address _asset, uint256 _amount) external nonReentrant {
        Market storage market = markets[_asset];
        require(market.active, "Market not active");
        
        Position storage position = positions[msg.sender][_asset];
        require(position.supplyAmount >= _amount, "Insufficient balance");
        
        // Check health factor after withdrawal
        if (position.borrowAmount > 0) {
            require(_checkHealthFactor(msg.sender), "Health factor too low");
        }
        
        // Update rates
        _updateRates(market);
        
        // Update position
        position.supplyAmount -= _amount;
        
        // Update market
        market.totalSupply -= _amount;
        
        // Transfer tokens
        IERC20(_asset).safeTransfer(msg.sender, _amount);
        
        emit Withdraw(msg.sender, _asset, _amount);
    }

    // ============ Borrow Functions ============

    /**
     * @notice Borrow assets
     */
    function borrow(address _asset, uint256 _amount) external nonReentrant {
        Market storage market = markets[_asset];
        require(market.active, "Market not active");
        
        Position storage position = positions[msg.sender][_asset];
        
        // Update rates
        _updateRates(market);
        
        // Check collateral
        uint256 maxBorrow = _getMaxBorrow(msg.sender);
        require(position.borrowAmount + _amount <= maxBorrow, "Exceeds collateral");
        
        // Update position
        uint256 interest = _calculateBorrowInterest(market, msg.sender);
        position.borrowAmount += _amount + interest;
        
        // Update market
        market.totalBorrow += _amount;
        
        // Transfer tokens
        IERC20(_asset).safeTransfer(msg.sender, _amount);
        
        emit Borrow(msg.sender, _asset, _amount);
    }

    /**
     * @notice Repay borrowed assets
     */
    function repay(address _asset, uint256 _amount) external nonReentrant {
        Market storage market = markets[_asset];
        require(market.active, "Market not active");
        
        Position storage position = positions[msg.sender][_asset];
        require(position.borrowAmount > 0, "No debt");
        
        // Update rates
        _updateRates(market);
        
        // Cap repayment at debt
        if (_amount > position.borrowAmount) {
            _amount = position.borrowAmount;
        }
        
        // Transfer tokens
        IERC20(_asset).safeTransferFrom(msg.sender, address(this), _amount);
        
        // Update position
        position.borrowAmount -= _amount;
        
        // Update market
        market.totalBorrow -= _amount;
        
        emit Repay(msg.sender, _asset, _amount);
    }

    // ============ Liquidation Functions ============

    /**
     * @notice Liquidate underwater position (LLAMMA style)
     */
    function liquidate(
        address _user,
        address _repayAsset,
        address _collateralAsset,
        uint256 _repayAmount
    ) external nonReentrant {
        require(!_checkHealthFactor(_user), "Position healthy");
        
        Market storage repayMarket = markets[_repayAsset];
        Market storage collateralMarket = markets[_collateralAsset];
        
        require(repayMarket.active && collateralMarket.active, "Invalid markets");
        
        Position storage position = positions[_user][_collateralAsset];
        require(position.active && position.supplyAmount > 0, "No collateral");
        
        // Cap liquidation at 50% of debt
        uint256 maxLiquidation = (position.borrowAmount * CLOSE_FACTOR) / MAX_COLLATERAL_RATIO;
        if (_repayAmount > maxLiquidation) {
            _repayAmount = maxLiquidation;
        }
        
        // Calculate collateral to receive
        uint256 collateralAmount = (_repayAmount * LendingMath.WAD * 10400) 
            / (collateralMarket.liquidationThreshold * 100);
        
        // Update positions
        position.supplyAmount -= collateralAmount;
        position.borrowAmount -= _repayAmount;
        
        repayMarket.totalBorrow -= _repayAmount;
        collateralMarket.totalSupply -= collateralAmount;
        
        // Transfer assets
        IERC20(_repayAsset).safeTransferFrom(msg.sender, address(this), _repayAmount);
        IERC20(_collateralAsset).safeTransfer(msg.sender, collateralAmount);
        
        emit Liquidate(
            msg.sender,
            _user,
            _repayAsset,
            _collateralAsset,
            _repayAmount,
            collateralAmount
        );
    }

    // ============ Flash Loan Functions ============

    /**
     * @notice Flash loan
     */
    function flashLoan(
        address _asset,
        uint256 _amount,
        bytes calldata _data
    ) external nonReentrant {
        Market storage market = markets[_asset];
        require(market.active, "Market not active");
        
        uint256 fee = (_amount * FLASH_LOAN_FEE) / 10000;
        
        // Transfer assets
        IERC20(_asset).safeTransfer(msg.sender, _amount);
        
        // Execute callback
        (bool success, ) = msg.sender.call(_data);
        require(success, "Flash loan failed");
        
        // Verify repayment
        uint256 balance = IERC20(_asset).balanceOf(address(this));
        require(balance >= market.totalSupply + fee, "Repayment insufficient");
        
        // Update fees
        flashLoanFees[_asset] += fee;
        
        emit FlashLoan(msg.sender, _asset, _amount, fee);
    }

    // ============ Rate Functions ============

    /**
     * @notice Update interest rate parameters
     */
    function setRateParameters(
        uint256 _baseRate,
        uint256 _slope1,
        uint256 _slope2,
        uint256 _optimalUtilization
    ) external onlyOwner {
        baseRate = _baseRate;
        slope1 = _slope1;
        slope2 = _slope2;
        optimalUtilization = _optimalUtilization;
        
        emit RateUpdated(_baseRate, _slope1, _slope2);
    }

    /**
     * @dev Update rates for market
     */
    function _updateRates(Market storage market) internal {
        uint256 timePassed = block.timestamp - market.lastUpdate;
        
        if (timePassed > 0) {
            uint256 utilization = market.totalBorrow > 0
                ? (market.totalBorrow * LendingMath.WAD) / market.totalSupply
                : 0;
            
            // Calculate borrow rate
            uint256 borrowRate;
            if (utilization <= optimalUtilization) {
                borrowRate = baseRate + (utilization * slope1) / optimalUtilization;
            } else {
                borrowRate = baseRate + slope1 + 
                    ((utilization - optimalUtilization) * slope2) / (LendingMath.WAD - optimalUtilization);
            }
            
            // Accumulate interest
            market.rateAccumulator = market.rateAccumulator.rmul(
                LendingMath.RAY + (borrowRate * timePassed / 365 days)
            );
            
            market.borrowRate = borrowRate;
            market.supplyRate = (borrowRate * utilization * 9) / 10; // 90% to suppliers
            market.lastUpdate = block.timestamp;
        }
    }

    // ============ Helper Functions ============

    /**
     * @dev Calculate supply interest
     */
    function _calculateSupplyInterest(Market storage market, address _user) internal view returns (uint256) {
        Position storage position = positions[_user][market.asset];
        
        uint256 interest = position.supplyAmount.rmul(market.rateAccumulator - market.supplyIndex);
        
        return interest;
    }

    /**
     * @dev Calculate borrow interest
     */
    function _calculateBorrowInterest(Market storage market, address _user) internal view returns (uint256) {
        Position storage position = positions[_user][market.asset];
        
        uint256 interest = position.borrowAmount.rmul(market.rateAccumulator - market.borrowIndex);
        
        return interest;
    }

    /**
     * @dev Get max borrow amount
     */
    function _getMaxBorrow(address _user) internal view returns (uint256) {
        uint256 totalCollateral = 0;
        
        for (uint256 i = 0; i < marketList.length; i++) {
            address asset = marketList[i];
            Position storage position = positions[_user][asset];
            
            if (position.supplyAmount > 0) {
                Market storage market = markets[asset];
                
                // Calculate collateral value
                uint256 assetValue = position.supplyAmount;
                totalCollateral += (assetValue * market.collateralFactor) / MAX_COLLATERAL_RATIO;
            }
        }
        
        return totalCollateral;
    }

    /**
     * @dev Check health factor
     */
    function _checkHealthFactor(address _user) internal view returns (bool) {
        uint256 totalBorrow = 0;
        uint256 totalCollateral = 0;
        
        for (uint256 i = 0; i < marketList.length; i++) {
            address asset = marketList[i];
            Position storage position = positions[_user][asset];
            
            if (position.borrowAmount > 0) {
                totalBorrow += position.borrowAmount;
            }
            
            if (position.supplyAmount > 0) {
                Market storage market = markets[asset];
                totalCollateral += (position.supplyAmount * market.liquidationThreshold) / MAX_COLLATERAL_RATIO;
            }
        }
        
        if (totalBorrow == 0) return true;
        
        return (totalCollateral * MAX_COLLATERAL_RATIO) / totalBorrow >= LIQUIDATION_THRESHOLD;
    }

    // ============ View Functions ============

    /**
     * @notice Get user's total collateral value
     */
    function getTotalCollateral(address _user) external view returns (uint256) {
        uint256 total = 0;
        
        for (uint256 i = 0; i < marketList.length; i++) {
            address asset = marketList[i];
            Position storage position = positions[_user][asset];
            
            if (position.supplyAmount > 0) {
                total += position.supplyAmount;
            }
        }
        
        return total;
    }

    /**
     * @notice Get user's total borrowed
     */
    function getTotalBorrowed(address _user) external view returns (uint256) {
        uint256 total = 0;
        
        for (uint256 i = 0; i < marketList.length; i++) {
            address asset = marketList[i];
            Position storage position = positions[_user][asset];
            
            if (position.borrowAmount > 0) {
                total += position.borrowAmount;
            }
        }
        
        return total;
    }

    /**
     * @notice Get user's health factor
     */
    function getHealthFactor(address _user) external view returns (uint256) {
        uint256 totalBorrow = 0;
        uint256 totalCollateral = 0;
        
        for (uint256 i = 0; i < marketList.length; i++) {
            address asset = marketList[i];
            Position storage position = positions[_user][asset];
            Market storage market = markets[asset];
            
            if (position.borrowAmount > 0) {
                totalBorrow += position.borrowAmount;
            }
            
            if (position.supplyAmount > 0) {
                totalCollateral += (position.supplyAmount * market.liquidationThreshold) / MAX_COLLATERAL_RATIO;
            }
        }
        
        if (totalBorrow == 0) return type(uint256).max;
        
        return (totalCollateral * MAX_COLLATERAL_RATIO) / totalBorrow;
    }

    /**
     * @notice Get market list
     */
    function getMarketList() external view returns (address[] memory) {
        return marketList;
    }

    /**
     * @notice Get position details
     */
    function getPosition(address _user, address _asset) external view returns (
        uint256 supplyAmount,
        uint256 borrowAmount,
        bool active
    ) {
        Position storage position = positions[_user][_asset];
        return (position.supplyAmount, position.borrowAmount, position.active);
    }
}
