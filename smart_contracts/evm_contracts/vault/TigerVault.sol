// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "@openzeppelin/contracts/token/ERC20/IERC20.sol";

/**
 * @title TigerVault
 * @notice Multi-signature vault for fund management
 */
contract TigerVault {
    struct Transaction {
        address to;
        uint256 value;
        bytes data;
        bool executed;
        uint256 confirmations;
        mapping(address => bool) confirmedBy;
    }

    uint256 public requiredSignatures;
    uint256 public transactionCount;
    uint256 public dailyLimit;
    uint256 public spentToday;
    uint256 public lastReset;
    
    address[] public owners;
    mapping(address => bool) public isOwner;
    mapping(uint256 => Transaction) public transactions;

    event Deposit(address indexed sender, uint256 amount, uint256 balance);
    event SubmitTransaction(address indexed owner, uint256 indexed txIndex);
    event ConfirmTransaction(address indexed owner, uint256 indexed txIndex);
    event ExecuteTransaction(address indexed owner, uint256 indexed txIndex);
    event RevokeConfirmation(address indexed owner, uint256 indexed txIndex);
    event DailyLimitChanged(uint256 newLimit);

    modifier onlyOwner() {
        require(isOwner[msg.sender], "TigerVault: NOT_OWNER");
        _;
    }

    constructor(address[] memory _owners, uint256 _required, uint256 _dailyLimit) {
        require(_owners.length > 0, "TigerVault: NO_OWNERS");
        require(_required > 0 && _required <= _owners.length, "TigerVault: INVALID_REQUIRED");
        
        for (uint256 i = 0; i < _owners.length; i++) {
            require(_owners[i] != address(0), "TigerVault: ZERO_ADDRESS");
            require(!isOwner[_owners[i]], "TigerVault: DUPLICATE_OWNER");
            isOwner[_owners[i]] = true;
            owners.push(_owners[i]);
        }
        
        requiredSignatures = _required;
        dailyLimit = _dailyLimit;
        lastReset = block.timestamp;
    }

    receive() external payable {
        emit Deposit(msg.sender, msg.value, address(this).balance);
    }

    function submitTransaction(address to, uint256 value, bytes memory data) public onlyOwner returns (uint256) {
        uint256 txIndex = transactionCount++;
        Transaction storage t = transactions[txIndex];
        t.to = to;
        t.value = value;
        t.data = data;
        t.executed = false;
        t.confirmations = 0;

        emit SubmitTransaction(msg.sender, txIndex);
        return txIndex;
    }

    function confirmTransaction(uint256 txIndex) public onlyOwner {
        require(!transactions[txIndex].executed, "TigerVault: ALREADY_EXECUTED");
        require(!transactions[txIndex].confirmedBy[msg.sender], "TigerVault: ALREADY_CONFIRMED");
        
        transactions[txIndex].confirmedBy[msg.sender] = true;
        transactions[txIndex].confirmations++;

        emit ConfirmTransaction(msg.sender, txIndex);

        if (transactions[txIndex].confirmations >= requiredSignatures) {
            executeTransaction(txIndex);
        }
    }

    function executeTransaction(uint256 txIndex) public onlyOwner {
        require(!transactions[txIndex].executed, "TigerVault: ALREADY_EXECUTED");
        require(transactions[txIndex].confirmations >= requiredSignatures, "TigerVault: NOT_CONFIRMED");
        
        Transaction storage t = transactions[txIndex];
        t.executed = true;

        (bool success, ) = t.to.call{value: t.value}(t.data);
        require(success, "TigerVault: CALL_FAILED");

        emit ExecuteTransaction(msg.sender, txIndex);
    }

    function revokeConfirmation(uint256 txIndex) public onlyOwner {
        require(transactions[txIndex].confirmedBy[msg.sender], "TigerVault: NOT_CONFIRMED");
        require(!transactions[txIndex].executed, "TigerVault: ALREADY_EXECUTED");

        transactions[txIndex].confirmedBy[msg.sender] = false;
        transactions[txIndex].confirmations--;

        emit RevokeConfirmation(msg.sender, txIndex);
    }

    function executeDailyTransfer(address to, uint256 value) external onlyOwner {
        _resetDailySpent();
        require(spentToday + value <= dailyLimit, "TigerVault: EXCEEDS_DAILY_LIMIT");
        
        spentToday += value;
        payable(to).transfer(value);
    }

    function _resetDailySpent() internal {
        if (block.timestamp - lastReset >= 24 hours) {
            spentToday = 0;
            lastReset = block.timestamp;
        }
    }

    function getTransactionCount() external view returns (uint256) {
        return transactionCount;
    }

    function getOwners() external view returns (address[] memory) {
        return owners;
    }

    function isConfirmed(uint256 txIndex) public view returns (bool) {
        return transactions[txIndex].confirmations >= requiredSignatures;
    }

    function setDailyLimit(uint256 _dailyLimit) external onlyOwner {
        dailyLimit = _dailyLimit;
        emit DailyLimitChanged(_dailyLimit);
    }
}