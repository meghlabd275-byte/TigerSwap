// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "@openzeppelin/contracts/token/ERC20/IERC20.sol";

/**
 * @title TigerTreasury
 * @notice Treasury contract for protocol funds management
 */
contract TigerTreasury {
    struct Allocation {
        uint256 percentage;
        address recipient;
        uint256 currentAmount;
        bool active;
    }

    uint256 public totalBalance;
    uint256 public constant DAY = 24 hours;
    uint256 public constant MIN_ALLOCATION = 100;
    uint256 public constant MAX_ALLOCATION = 3000; // 30%

    address public governance;
    address public emergencyWallet;
    address[] public allocationAddresses;
    
    mapping(address => Allocation) public allocations;
    mapping(address => bool) public isSpender;
    mapping(address => uint256) public lastSpentTime;
    mapping(address => uint256) public spendingLimit;

    event Deposit(address indexed sender, uint256 amount, uint256 newBalance);
    event Withdrawal(address indexed recipient, uint256 amount, address indexed spender);
    event AllocationSet(address indexed recipient, uint256 percentage);
    event SpenderAdded(address indexed spender, uint256 limit);
    event SpenderRemoved(address indexed spender);

    modifier onlyGovernance() {
        require(msg.sender == governance, "TigerTreasury: FORBIDDEN");
        _;
    }

    modifier onlySpender() {
        require(isSpender[msg.sender], "TigerTreasury: NOT_SPENDER");
        _;
    }

    constructor(address _governance, address _emergencyWallet) {
        require(_governance != address(0) && _emergencyWallet != address(0), "TigerTreasury: ZERO_ADDRESS");
        governance = _governance;
        emergencyWallet = _emergencyWallet;
    }

    receive() external payable {
        totalBalance += msg.value;
        emit Deposit(msg.sender, msg.value, totalBalance);
    }

    function depositToken(address token, uint256 amount) external {
        require(IERC20(token).transferFrom(msg.sender, address(this), amount), "TigerTreasury: TRANSFER_FAILED");
        emit Deposit(msg.sender, amount, totalBalance);
    }

    function withdraw(address payable recipient, uint256 amount, string memory reason) external onlySpender {
        require(amount <= totalBalance, "TigerTreasury: INSUFFICIENT_BALANCE");
        require(amount <= spendingLimit[msg.sender], "TigerTreasury: EXCEEDS_LIMIT");
        
        totalBalance -= amount;
        recipient.transfer(amount);
        
        lastSpentTime[msg.sender] = block.timestamp;
        emit Withdrawal(recipient, amount, msg.sender);
    }

    function setAllocation(address recipient, uint256 percentage) external onlyGovernance {
        require(percentage >= MIN_ALLOCATION && percentage <= MAX_ALLOCATION, "TigerTreasury: INVALID_PERCENTAGE");
        
        if (!allocations[recipient].active) {
            allocationAddresses.push(recipient);
        }
        
        allocations[recipient] = Allocation({
            percentage: percentage,
            recipient: recipient,
            currentAmount: (totalBalance * percentage) / 10000,
            active: true
        });
        
        emit AllocationSet(recipient, percentage);
    }

    function distribute() external onlyGovernance {
        uint256 totalPercentage = 0;
        for (uint256 i = 0; i < allocationAddresses.length; i++) {
            address recipient = allocationAddresses[i];
            if (allocations[recipient].active) {
                totalPercentage += allocations[recipient].percentage;
                uint256 amount = (totalBalance * allocations[recipient].percentage) / 10000;
                allocations[recipient].currentAmount = amount;
            }
        }
        require(totalPercentage <= 10000, "TigerTreasury: EXCEEDS_100_PERCENT");
    }

    function addSpender(address spender, uint256 limit) external onlyGovernance {
        require(!isSpender[spender], "TigerTreasury: ALREADY_SPENDER");
        isSpender[spender] = true;
        spendingLimit[spender] = limit;
        emit SpenderAdded(spender, limit);
    }

    function removeSpender(address spender) external onlyGovernance {
        require(isSpender[spender], "TigerTreasury: NOT_SPENDER");
        isSpender[spender] = false;
        emit SpenderRemoved(spender);
    }

    function emergencyWithdraw() external {
        require(msg.sender == emergencyWallet, "TigerTreasury: FORBIDDEN");
        uint256 amount = address(this).balance;
        emergencyWallet.transfer(amount);
        totalBalance = 0;
    }

    function getBalance() external view returns (uint256) {
        return totalBalance;
    }

    function getAllocationRecipients() external view returns (address[] memory) {
        return allocationAddresses;
    }
}