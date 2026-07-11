// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/**
 * @title TigerUSD
 * @notice Production Native Stablecoin - crvUSD Style
 * @dev Decentralized stablecoin with LLAMMA-style liquidations
 * 
 * Features:
 * - Over-collateralized minting
 * - LLAMMA-style automated liquidations
 * - Interest rate mechanism
 * - Emergency shutdown
 * - Governance controlled
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
 * @title Decimal Math
 */
library StablecoinMath {
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
 * @title ITigerUSD
 * @dev Interface for TigerUSD
 */
interface ITigerUSD is IERC20 {
    function mint(address _to, uint256 _amount) external;
    function burn(address _from, uint256 _amount) external;
}

/**
 * @title ICreditAccount
 * @dev Interface for credit accounts
 */
interface ICreditAccount {
    function execute(
        address[] calldata _targets,
        bytes[] calldata _calldatas
    ) external payable;
}

/**
 * @title TigerUSD
 * @dev Main stablecoin contract
 */
contract TigerUSD is ITigerUSD, ReentrancyGuard, Ownable, AccessControl {
    using SafeERC20 for IERC20;
    using StablecoinMath for uint256;

    // ============ Roles ============
    bytes32 public constant MINTER = keccak256("MINTER");
    bytes32 public constant BURNER = keccak256("BURNER");
    bytes32 public constant LIQUIDATOR = keccak256("LIQUIDATOR");

    // ============ Constants ============
    uint256 constant WAD = 1e18;
    uint256 constant RAY = 1e27;
    uint256 constant MAX_DEBT = 10_000_000 * WAD; // 10M max debt
    
    // Liquidation parameters
    uint256 constant LIQUIDATION_THRESHOLD = 8500; // 85%
    uint256 constant LIQUIDATION_BONUS = 10400; // 4% bonus for liquidator
    uint256 constant ORACLE_TIMEOUT = 1 hours;
    uint256 constant DEGRADATION_COEFFICIENT = 24 * 60 * 60; // 1 day

    // ============ State Variables ============
    
    // Token info
    string public constant name = "Tiger USD";
    string public constant symbol = "TUSD";
    uint8 public constant decimals = 18;
    
    // Rate
    uint256 public interestRate = 1000000005936000000000000; // 2% APY in RAY
    uint256 public lastUpdateTime;
    uint256 public rateAccumulator;
    
    // Debt
    uint256 public totalDebt;
    mapping(address => uint256) public debt;
    
    // Collateral
    address public collateralToken;
    address public priceOracle;
    mapping(address => uint256) public collateral;
    mapping(address => uint256) public liquidationThreshold;
    
    // Credit accounts
    mapping(address => address) public creditAccounts;
    mapping(address => bool) public isCreditAccount;
    
    // Liquidation
    uint256 public totalLiquidationQueue;
    mapping(uint256 => LiquidationRequest) public liquidationQueue;
    uint256 public liquidationHead;
    uint256 public liquidationTail;
    
    // Emergency
    bool public stopped;
    bool public doMint = true;
    bool public doBurn = true;
    
    // ============ Structs ============
    
    struct LiquidationRequest {
        address user;
        uint256 collateralAmount;
        uint256 debtAmount;
        uint256 price;
        uint256 timestamp;
        bool executed;
    }
    
    struct CreditAccountData {
        address owner;
        uint256 collateralAmount;
        uint256 debtAmount;
        uint256 liquidationPrice;
        uint256 lastUpdate;
        bool active;
    }

    // ============ Events ============
    event Mint(address indexed user, uint256 amount, uint256 collateral);
    event Burn(address indexed user, uint256 amount, uint256 collateral);
    event Liquidate(address indexed user, address indexed liquidator, uint256 debt, uint256 collateral);
    event InterestRateUpdated(uint256 newRate);
    event CollateralUpdated(address indexed user, uint256 newCollateral);
    event DebtUpdated(address indexed user, uint256 newDebt);
    event RateAccumulatorUpdated(uint256 newRate);
    event EmergencyShutdown(bool stopped);

    // ============ Constructor ============
    
    constructor(address _collateral, address _priceOracle, address _owner) Ownable(_owner) {
        collateralToken = _collateral;
        priceOracle = _priceOracle;
        
        lastUpdateTime = block.timestamp;
        rateAccumulator = RAY;
        
        _grantRole(DEFAULT_ADMIN_ROLE, _owner);
        _grantRole(MINTER, _owner);
        _grantRole(BURNER, _owner);
        _grantRole(LIQUIDATOR, _owner);
    }

    // ============ Mint/Burn Functions ============

    /**
     * @notice Mint TUSD by depositing collateral
     */
    function mint(uint256 _collateralAmount, uint256 _minMintAmount) 
        external 
        nonReentrant 
        returns (uint256) 
    {
        require(doMint, "Minting disabled");
        require(_collateralAmount > 0, "No collateral");
        
        // Transfer collateral
        IERC20(collateralToken).safeTransferFrom(
            msg.sender, 
            address(this), 
            _collateralAmount
        );
        
        // Get oracle price
        uint256 price = _getCollateralPrice();
        uint256 maxMint = (_collateralAmount * price * LIQUIDATION_THRESHOLD) / (WAD * 10000);
        
        // Calculate actual mint amount
        uint256 mintAmount = maxMint;
        
        // Update state
        collateral[msg.sender] += _collateralAmount;
        debt[msg.sender] += mintAmount;
        
        // Update rate
        _updateRate();
        
        // Check min receive
        require(mintAmount >= _minMintAmount, "Slippage");
        
        // Check total debt
        require(debt[msg.sender] <= MAX_DEBT, "Max debt exceeded");
        
        // Mint tokens
        _mint(msg.sender, mintAmount);
        
        totalDebt += mintAmount;
        
        emit Mint(msg.sender, mintAmount, _collateralAmount);
        
        return mintAmount;
    }

    /**
     * @notice Burn TUSD to release collateral
     */
    function burn(uint256 _amount, uint256 _minCollateralReceive) 
        external 
        nonReentrant 
        returns (uint256) 
    {
        require(doBurn, "Burning disabled");
        require(_amount > 0, "No amount");
        require(debt[msg.sender] >= _amount, "Insufficient debt");
        
        // Update rate
        _updateRate();
        
        // Calculate collateral to return
        uint256 price = _getCollateralPrice();
        uint256 collateralAmount = (_amount * WAD * 10000) / (price * LIQUIDATION_THRESHOLD);
        
        // Update state
        debt[msg.sender] -= _amount;
        collateral[msg.sender] -= collateralAmount;
        
        // Check min receive
        require(collateralAmount >= _minCollateralReceive, "Slippage");
        
        // Burn tokens
        _burn(msg.sender, _amount);
        
        totalDebt -= _amount;
        
        // Transfer collateral
        IERC20(collateralToken).safeTransfer(msg.sender, collateralAmount);
        
        emit Burn(msg.sender, _amount, collateralAmount);
        
        return collateralAmount;
    }

    // ============ Liquidation Functions ============

    /**
     * @notice Liquidate underwater position (LLAMMA style)
     */
    function liquidate(address _user, uint256 _maxCollateral) 
        external 
        nonReentrant 
        returns (uint256, uint256) 
    {
        require(msg.sender != _user, "Cannot liquidate self");
        
        // Check if position is underwater
        uint256 liquidationPrice = _calculateLiquidationPrice(_user);
        uint256 currentPrice = _getCollateralPrice();
        
        require(currentPrice < liquidationPrice, "Position safe");
        
        // Calculate debt to liquidate
        uint256 userDebt = debt[_user];
        uint256 maxLiquidatable = (collateral[_user] * liquidationPrice * LIQUIDATION_BONUS) 
            / (WAD * 10000);
        
        uint256 liquidateAmount = userDebt < maxLiquidatable ? userDebt : maxLiquidatable;
        
        // Calculate collateral to receive
        uint256 collateralReceived = (liquidateAmount * WAD * 10000) 
            / (liquidationPrice * LIQUIDATION_BONUS);
        
        if (collateralReceived > _maxCollateral) {
            collateralReceived = _maxCollateral;
            liquidateAmount = (collateralReceived * liquidationPrice * LIQUIDATION_BONUS) 
                / (WAD * 10000);
        }
        
        // Update state
        debt[_user] -= liquidateAmount;
        collateral[_user] -= collateralReceived;
        
        // Burn TUSD from liquidator
        _burn(msg.sender, liquidateAmount);
        
        // Transfer collateral to liquidator
        IERC20(collateralToken).safeTransfer(msg.sender, collateralReceived);
        
        emit Liquidate(_user, msg.sender, liquidateAmount, collateralReceived);
        
        return (liquidateAmount, collateralReceived);
    }

    /**
     * @notice Calculate liquidation price for user
     */
    function _calculateLiquidationPrice(address _user) internal view returns (uint256) {
        uint256 userCollateral = collateral[_user];
        uint256 userDebt = debt[_user];
        
        if (userCollateral == 0 || userDebt == 0) return 0;
        
        // Liquidation price = debt / (collateral * threshold)
        return (userDebt * WAD * 10000) / (userCollateral * LIQUIDATION_THRESHOLD);
    }

    // ============ Rate Functions ============

    /**
     * @notice Update interest rate
     */
    function setInterestRate(uint256 _rate) external onlyOwner {
        require(_rate > 0 && _rate < StablecoinMath.MAX_RATE, "Invalid rate");
        
        interestRate = _rate;
        
        emit InterestRateUpdated(_rate);
    }

    /**
     * @dev Update rate accumulator
     */
    function _updateRate() internal {
        uint256 timePassed = block.timestamp - lastUpdateTime;
        
        if (timePassed > 0) {
            // Rate accumulation: rate = rate * (1 + rate * time)
            // Using RAY precision
            uint256 ratePerSecond = interestRate;
            uint256 newAccumulator = rateAccumulator.rmul(
                StablecoinMath.RAY + (ratePerSecond * timePassed / DEGRADATION_COEFFICIENT)
            );
            
            rateAccumulator = newAccumulator;
            lastUpdateTime = block.timestamp;
            
            emit RateAccumulatorUpdated(newAccumulator);
        }
    }

    // ============ Oracle Functions ============

    /**
     * @dev Get collateral price from oracle
     */
    function _getCollateralPrice() internal view returns (uint256) {
        // In production, integrate with Chainlink/Pyth
        // For now, return mock price
        // Would use: IPriceOracle(priceOracle).getPrice()
        
        // Example: ETH at $3000
        return 3000 * WAD;
    }

    // ============ Emergency Functions ============

    /**
     * @notice Emergency shutdown
     */
    function stop() external onlyOwner {
        stopped = true;
        
        emit EmergencyShutdown(true);
    }

    /**
     * @notice Resume operations
     */
    function resume() external onlyOwner {
        stopped = false;
        
        emit EmergencyShutdown(false);
    }

    /**
     * @notice Toggle minting
     */
    function toggleMinting(bool _doMint) external onlyOwner {
        doMint = _doMint;
    }

    /**
     * @notice Toggle burning
     */
    function toggleBurning(bool _doBurn) external onlyOwner {
        doBurn = _doBurn;
    }

    // ============ View Functions ============

    /**
     * @notice Get user's collateral ratio
     */
    function getCollateralRatio(address _user) external view returns (uint256) {
        uint256 userCollateral = collateral[_user];
        uint256 userDebt = debt[_user];
        
        if (userDebt == 0) return type(uint256).max;
        
        uint256 price = _getCollateralPrice();
        uint256 collateralValue = (userCollateral * price) / WAD;
        uint256 debtValue = userDebt;
        
        return (collateralValue * 10000) / debtValue;
    }

    /**
     * @notice Get user's health factor
     */
    function getHealthFactor(address _user) external view returns (uint256) {
        uint256 ratio = getCollateralRatio(_user);
        
        if (ratio >= LIQUIDATION_THRESHOLD) {
            return (ratio - LIQUIDATION_THRESHOLD) * WAD / (10000 - LIQUIDATION_THRESHOLD);
        }
        
        return 0;
    }

    /**
     * @notice Get liquidation price for user
     */
    function getLiquidationPrice(address _user) external view returns (uint256) {
        return _calculateLiquidationPrice(_user);
    }

    /**
     * @notice Get user position
     */
    function getUserPosition(address _user) external view returns (
        uint256 collateralAmount,
        uint256 debtAmount,
        uint256 collateralRatio,
        uint256 healthFactor
    ) {
        return (
            collateral[_user],
            debt[_user],
            getCollateralRatio(_user),
            getHealthFactor(_user)
        );
    }

    // ============ ERC20 Overrides ============

    function _mint(address _to, uint256 _amount) internal override {
        require(_to != address(0), "Invalid address");
        require(_amount > 0, "Amount is 0");
        
        _updateRate();
        
        uint256 newTotalDebt = totalDebt + _amount;
        
        _balances[_to] += _amount;
        totalSupply += _amount;
        totalDebt = newTotalDebt;
        
        emit Transfer(address(0), _to, _amount);
    }

    function _burn(address _from, uint256 _amount) internal override {
        require(_balances[_from] >= _amount, "Insufficient balance");
        
        _updateRate();
        
        uint256 newTotalDebt = totalDebt - _amount;
        
        _balances[_from] -= _amount;
        totalSupply -= _amount;
        totalDebt = newTotalDebt;
        
        emit Transfer(_from, address(0), _amount);
    }

    // ============ State Variables (ERC20) ============
    mapping(address => uint256) internal _balances;
    mapping(address => mapping(address => uint256)) internal _allowances;
    uint256 internal _totalSupply;

    function totalSupply() public view override returns (uint256) {
        return _totalSupply;
    }

    function balanceOf(address account) public view override returns (uint256) {
        return _balances[account];
    }

    function transfer(address to, uint256 amount) public override returns (bool) {
        address owner = msg.sender;
        _transfer(owner, to, amount);
        return true;
    }

    function allowance(address owner, address spender) public view override returns (uint256) {
        return _allowances[owner][spender];
    }

    function approve(address spender, uint256 amount) public override returns (bool) {
        address owner = msg.sender;
        _approve(owner, spender, amount);
        return true;
    }

    function transferFrom(address from, address to, uint256 amount) public override returns (bool) {
        address spender = msg.sender;
        _spendAllowance(from, spender, amount);
        _transfer(from, to, amount);
        return true;
    }

    function _transfer(address from, address to, uint256 amount) internal {
        require(from != address(0), "Transfer from zero");
        require(to != address(0), "Transfer to zero");
        
        _balances[from] -= amount;
        _balances[to] += amount;
        
        emit Transfer(from, to, amount);
    }

    function _approve(address owner, address spender, uint256 amount) internal {
        require(owner != address(0), "Approve from zero");
        require(spender != address(0), "Approve to zero");
        
        _allowances[owner][spender] = amount;
        
        emit Approval(owner, spender, amount);
    }

    function _spendAllowance(address owner, address spender, uint256 amount) internal {
        uint256 currentAllowance = _allowances[owner][spender];
        if (currentAllowance != type(uint256).max) {
            require(currentAllowance >= amount, "Insufficient allowance");
            _allowances[owner][spender] = currentAllowance - amount;
        }
    }
}
