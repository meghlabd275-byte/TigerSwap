// SPDX-License-Identifier: MIT
pragma solidity ^0.8.19;

/**
 * @title TimelockController
 * @notice Time-delayed execution controller
 */
contract TimelockController {
    struct Transaction {
        address target;
        uint256 value;
        bytes data;
        uint256 nonce;
        uint256 executeAfter;
        bool executed;
        bool cancelled;
    }
    
    mapping(bytes32 => Transaction) public transactions;
    mapping(address => uint256) public nonces;
    
    uint256 public delay;
    address public admin;
    
    event TransactionQueued(bytes32 indexed txHash, address target, uint256 executeAfter);
    event TransactionExecuted(bytes32 indexed txHash);
    event TransactionCancelled(bytes32 indexed txHash);
    
    constructor(uint256 _delay) {
        delay = _delay;
        admin = msg.sender;
    }
    
    modifier onlyAdmin() {
        require(msg.sender == admin, "Not admin");
        _;
    }
    
    function queueTransaction(
        address target,
        uint256 value,
        bytes memory data
    ) external onlyAdmin returns (bytes32) {
        bytes32 txHash = keccak256(abi.encode(target, value, data, nonces[target]++));
        
        transactions[txHash] = Transaction({
            target: target,
            value: value,
            data: data,
            nonce: nonces[target],
            executeAfter: block.timestamp + delay,
            executed: false,
            cancelled: false
        });
        
        emit TransactionQueued(txHash, target, block.timestamp + delay);
        return txHash;
    }
    
    function executeTransaction(bytes32 txHash) external payable returns (bytes memory) {
        Transaction storage tx = transactions[txHash];
        require(tx.target != address(0), "Not queued");
        require(!tx.executed, "Executed");
        require(!tx.cancelled, "Cancelled");
        require(block.timestamp >= tx.executeAfter, "Too early");
        
        tx.executed = true;
        
        (bool success, bytes memory result) = tx.target.call{value: tx.value}(tx.data);
        require(success, "Failed");
        
        emit TransactionExecuted(txHash);
        return result;
    }
    
    function cancelTransaction(bytes32 txHash) external onlyAdmin {
        Transaction storage tx = transactions[txHash];
        require(tx.target != address(0), "Not queued");
        require(!tx.executed, "Executed");
        
        tx.cancelled = true;
        
        emit TransactionCancelled(txHash);
    }
    
    function setDelay(uint256 _delay) external onlyAdmin {
        delay = _delay;
    }
    
    function getTransactionHash(
        address target,
        uint256 value,
        bytes memory data
    ) external view returns (bytes32) {
        return keccak256(abi.encode(target, value, data, nonces[target]));
    }
}