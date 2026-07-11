// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/**
 * @title TigerSwapLending
 * @notice Lending protocol for collateralized borrowing
 */
contract TigerSwapLending {
    struct Market {
        address token;
        uint256 totalDeposits;
        uint256 totalBorrows;
        uint256 depositRate;
        uint256 borrowRate;
        uint256 collateralFactor;
        uint256 liquidationThreshold;
        bool isActive;
    }
    
    struct Account {
        address owner;
        uint256 deposits;
        uint256 borrows;
        mapping(address => uint256) collaterals;
    }
    
    mapping(address => Market) public markets;
    mapping(address => Account) public accounts;
    address public owner;
    
    event MarketCreated(address indexed token, uint256 collateralFactor);
    event Deposited(address indexed user, address indexed token, uint256 amount);
    event Borrowed(address indexed user, address indexed token, uint256 amount);
    event Repaid(address indexed user, address indexed token, uint256 amount);
    event Liquidated(address indexed liquidator, address indexed user, address indexed token, uint256 amount);
    
    modifier onlyOwner() { require(msg.sender == owner, "NOT_OWNER"); _; }
    
    constructor() { owner = msg.sender; }
    
    function createMarket(address token, uint256 collateralFactor, uint256 liquidationThreshold) external onlyOwner {
        markets[token] = Market({
            token: token,
            totalDeposits: 0,
            totalBorrows: 0,
            depositRate: 0,
            borrowRate: 0,
            collateralFactor: collateralFactor,
            liquidationThreshold: liquidationThreshold,
            isActive: true
        });
        emit MarketCreated(token, collateralFactor);
    }
    
    function deposit(address token, uint256 amount) external {
        Market storage market = markets[token];
        require(market.isActive, "MARKET_INACTIVE");
        
        IERC20(token).transferFrom(msg.sender, address(this), amount);
        accounts[msg.sender].deposits += amount;
        market.totalDeposits += amount;
        
        emit Deposited(msg.sender, token, amount);
    }
    
    function borrow(address token, uint256 amount) external {
        Market storage market = markets[token];
        require(market.isActive, "MARKET_INACTIVE");
        
        Account storage account = accounts[msg.sender];
        uint256 maxBorrow = (account.deposits * market.collateralFactor) / 1e18;
        require(account.borrows + amount <= maxBorrow, "INSUFFICIENT_COLLATERAL");
        
        account.borrows += amount;
        market.totalBorrows += amount;
        IERC20(token).transfer(msg.sender, amount);
        
        emit Borrowed(msg.sender, token, amount);
    }
    
    function repay(address token, uint256 amount) external {
        Account storage account = accounts[msg.sender];
        require(account.borrows >= amount, "REPAY_TOO_MUCH");
        
        IERC20(token).transferFrom(msg.sender, address(this), amount);
        account.borrows -= amount;
        markets[token].totalBorrows -= amount;
        
        emit Repaid(msg.sender, token, amount);
    }
    
    function liquidate(address user, address token) external {
        Account storage account = accounts[user];
        Market storage market = markets[token];
        
        require(account.borrows > 0, "NO_DEBT");
        require(account.deposits * market.collateralFactor / 1e18 < account.borrows, "HEALTHY");
        
        uint256 reward = account.borrows;
        account.borrows = 0;
        account.deposits = 0;
        
        emit Liquidated(msg.sender, user, token, reward);
    }
    
    function getAccountHealth(address user) external view returns (uint256) {
        Account storage account = accounts[user];
        if (account.borrows == 0) return type(uint256).max;
        
        uint256 collateralValue = account.deposits;
        return (collateralValue * 1e18) / account.borrows;
    }
}

interface IERC20 {
    function transferFrom(address from, address to, uint256 value) external returns (bool);
    function transfer(address to, uint256 value) external returns (bool);
}
