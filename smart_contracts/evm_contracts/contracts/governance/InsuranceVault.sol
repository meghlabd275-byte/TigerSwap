// SPDX-License-Identifier: MIT
pragma solidity ^0.8.19;

/**
 * @title InsuranceVault
 * @notice Insurance fund for covering protocol losses
 */
contract InsuranceVault {
    mapping(address => uint256) public deposits;
    mapping(address => uint256) public withdrawals;
    
    uint256 public totalDeposits;
    uint256 public totalWithdrawals;
    uint256 public maxCoverage;
    
    address public governance;
    bool public paused;
    
    event Deposit(address indexed user, uint256 amount);
    event Withdrawal(address indexed user, uint256 amount);
    event CoveragePaid(address indexed recipient, uint256 amount);
    
    modifier onlyGovernance() {
        require(msg.sender == governance, "Not governance");
        _;
    }
    
    modifier whenNotPaused() {
        require(!paused, "Paused");
        _;
    }
    
    constructor(address _governance) {
        governance = _governance;
        maxCoverage = 1000000e18;
    }
    
    function deposit() external payable whenNotPaused {
        require(msg.value > 0, "Zero amount");
        
        deposits[msg.sender] += msg.value;
        totalDeposits += msg.value;
        
        emit Deposit(msg.sender, msg.value);
    }
    
    function withdraw(uint256 amount) external {
        require(deposits[msg.sender] >= amount, "Insufficient");
        
        deposits[msg.sender] -= amount;
        totalDeposits -= amount;
        withdrawals[msg.sender] += amount;
        totalWithdrawals += amount;
        
        payable(msg.sender).transfer(amount);
        
        emit Withdrawal(msg.sender, amount);
    }
    
    function payCoverage(address recipient, uint256 amount) external onlyGovernance {
        require(amount <= maxCoverage, "Exceeds max");
        require(address(this).balance >= amount, "Insufficient balance");
        
        totalWithdrawals += amount;
        
        payable(recipient).transfer(amount);
        
        emit CoveragePaid(recipient, amount);
    }
    
    function setMaxCoverage(uint256 _maxCoverage) external onlyGovernance {
        maxCoverage = _maxCoverage;
    }
    
    function pause() external onlyGovernance {
        paused = true;
    }
    
    function unpause() external onlyGovernance {
        paused = false;
    }
    
    receive() external payable {}
}