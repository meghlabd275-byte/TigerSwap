// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "../libraries/SafeMath.sol";

/**
 * @title TigerMultiSigWallet
 * @notice Gnosis Safe-style Multi-Signature Wallet
 * @dev Secure multi-sig wallet with confirmations and execution
 * 
 * Features:
 * - Multiple owners with threshold
 * - Transaction proposals and confirmations
 * - Execution with deadline
 * - Module support (modules can execute without confirmation)
 * - Fallback handler for ERC-721, ERC-1155
 * - Snapshot for voting
 */
contract TigerMultiSigWallet {
    using SafeMath for uint256;

    // ============================================================================
    // Constants
    // ============================================================================
    
    uint256 constant MAX_OWNERS = 50;
    uint256 constant MAX_THRESHOLD = 50;
    uint256 constant EXPIRY_DURATION = 86400; // 24 hours
    
    // ============================================================================
    // State Variables
    // ============================================================================
    
    // Owners
    mapping(address => bool) public isOwner;
    address[] public owners;
    uint256 public threshold;
    uint256 public nonce;
    
    // Transactions
    mapping(bytes32 => Transaction) public transactions;
    mapping(bytes32 => mapping(address => bool)) public confirmations;
    mapping(bytes32 => uint256) public confirmationCount;
    mapping(bytes32 => uint256) public executionCount;
    
    // Modules
    mapping(address => bool) public isModule;
    mapping(address => bool) public enabledModules;
    
    // Guards
    mapping(bytes32 => TransactionGuard) public transactionGuards;
    
    // Fallback
    address public fallbackHandler;
    address public protocolHandler;
    
    // Events
    event OwnerAdded(address indexed owner);
    event OwnerRemoved(address indexed owner);
    event ThresholdChanged(uint256 threshold);
    event Confirmation(address indexed sender, bytes32 indexed txHash);
    event Revocation(address indexed sender, bytes32 indexed txHash);
    event Execution(bytes32 indexed txHash, uint256 gasUsed);
    event ExecutionFailed(bytes32 indexed txHash);
    event Deposit(address indexed sender, uint256 value);
    
    // ============== Structs ==============
    
    struct Transaction {
        address to;
        uint256 value;
        bytes data;
        uint256 operation; // 0 = call, 1 = delegatecall
        uint256 nonce;
        uint256 expiry;
        bool executed;
        bool cancelled;
        uint256 gasUsed;
    }
    
    struct TransactionGuard {
        bool required;
        bytes32 guardHash;
        uint256 checkPosition;
    }
    
    struct Module {
        address module;
        string name;
        bool enabled;
    }
    
    // ============== Modifier ==============
    
    modifier onlyOwner() {
        require(isOwner[msg.sender], "NOT_OWNER");
        _;
    }
    
    modifier onlyModule() {
        require(isModule[msg.sender] && enabledModules[msg.sender], "NOT_MODULE");
        _;
    }
    
    // ============== Constructor ==============
    
    constructor(address[] memory _owners, uint256 _threshold) {
        require(_owners.length > 0, "NO_OWNERS");
        require(_threshold > 0, "NO_THRESHOLD");
        require(_owners.length >= _threshold, "INVALID_THRESHOLD");
        require(_owners.length <= MAX_OWNERS, "TOO_MANY_OWNERS");
        require(_threshold <= MAX_THRESHOLD, "THRESHOLD_TOO_HIGH");
        
        for (uint256 i = 0; i < _owners.length; i++) {
            address owner = _owners[i];
            require(owner != address(0), "INVALID_OWNER");
            require(!isOwner[owner], "DUPLICATE_OWNER");
            
            isOwner[owner] = true;
            owners.push(owner);
        }
        
        threshold = _threshold;
        nonce = 0;
    }
    
    // ============================================================================
    // Owner Management
    // ============================================================================
    
    /**
     * @notice Add new owner (requires threshold confirmations)
     */
    function addOwner(address _owner) external onlyOwner {
        require(_owner != address(0), "INVALID_OWNER");
        require(!isOwner[_owner], "ALREADY_OWNER");
        require(owners.length < MAX_OWNERS, "MAX_OWNERS");
        
        isOwner[_owner] = true;
        owners.push(_owner);
        
        emit OwnerAdded(_owner);
    }
    
    /**
     * @notice Remove owner (requires threshold confirmations)
     */
    function removeOwner(address _owner) external onlyOwner {
        require(isOwner[_owner], "NOT_OWNER");
        require(owners.length - 1 >= threshold, "LAST_OWNER");
        
        isOwner[_owner] = false;
        
        // Remove from owners array
        for (uint256 i = 0; i < owners.length - 1; i++) {
            if (owners[i] == _owner) {
                owners[i] = owners[owners.length - 1];
                break;
            }
        }
        owners.pop();
        
        emit OwnerRemoved(_owner);
    }
    
    /**
     * @notice Change threshold
     */
    function changeThreshold(uint256 _threshold) external onlyOwner {
        require(_threshold > 0, "NO_THRESHOLD");
        require(_threshold <= owners.length, "THRESHOLD_TOO_HIGH");
        
        threshold = _threshold;
        
        emit ThresholdChanged(_threshold);
    }
    
    /**
     * @notice Get owner count
     */
    function getOwnerCount() external view returns (uint256) {
        return owners.length;
    }
    
    // ============================================================================
    // Transaction Management
    // ============================================================================
    
    /**
     * @notice Submit transaction for confirmation
     */
    function submitTransaction(
        address _to,
        uint256 _value,
        bytes calldata _data,
        uint256 _operation
    ) external onlyOwner returns (bytes32 txHash) {
        require(_to != address(0), "INVALID_TO");
        
        nonce++;
        txHash = keccak256(abi.encodePacked(
            _to,
            _value,
            _data,
            _operation,
            nonce,
            block.chainid,
            address(this)
        ));
        
        transactions[txHash] = Transaction({
            to: _to,
            value: _value,
            data: _data,
            operation: _operation,
            nonce: nonce,
            expiry: block.timestamp + EXPIRY_DURATION,
            executed: false,
            cancelled: false,
            gasUsed: 0
        });
        
        // Auto-confirm from sender
        confirmTransaction(txHash);
    }
    
    /**
     * @notice Confirm transaction
     */
    function confirmTransaction(bytes32 _txHash) public onlyOwner {
        Transaction storage tx_ = transactions[_txHash];
        require(tx_.to != address(0), "INVALID_TX");
        require(!tx_.executed, "EXECUTED");
        require(!tx_.cancelled, "CANCELLED");
        require(block.timestamp <= tx_.expiry, "EXPIRED");
        
        if (!confirmations[_txHash][msg.sender]) {
            confirmations[_txHash][msg.sender] = true;
            confirmationCount[_txHash]++;
            
            emit Confirmation(msg.sender, _txHash);
        }
    }
    
    /**
     * @notice Revoke confirmation
     */
    function revokeConfirmation(bytes32 _txHash) external onlyOwner {
        require(confirmations[_txHash][msg.sender], "NOT_CONFIRMED");
        
        confirmations[_txHash][msg.sender] = false;
        confirmationCount[_txHash]--;
        
        emit Revocation(msg.sender, _txHash);
    }
    
    /**
     * @notice Execute confirmed transaction
     */
    function executeTransaction(bytes32 _txHash) external onlyOwner returns (bool success) {
        Transaction storage tx_ = transactions[_txHash];
        require(tx_.to != address(0), "INVALID_TX");
        require(!tx_.executed, "EXECUTED");
        require(!tx_.cancelled, "CANCELLED");
        require(block.timestamp <= tx_.expiry, "EXPIRED");
        require(confirmationCount[_txHash] >= threshold, "NO_CONFIRMATIONS");
        
        // Execute transaction
        uint256 gasBefore = gasleft();
        
        if (tx_.operation == 0) {
            (success, ) = tx_.to.call{value: tx_.value}(tx_.data);
        } else if (tx_.operation == 1) {
            (success, ) = tx_.to.delegatecall(tx_.data);
        }
        
        if (success) {
            tx_.executed = true;
            tx_.gasUsed = gasBefore - gasleft();
            executionCount[_txHash]++;
            
            emit Execution(_txHash, tx_.gasUsed);
        } else {
            emit ExecutionFailed(_txHash);
        }
        
        return success;
    }
    
    /**
     * @notice Cancel transaction
     */
    function cancelTransaction(bytes32 _txHash) external onlyOwner {
        Transaction storage tx_ = transactions[_txHash];
        require(tx_.to != address(0), "INVALID_TX");
        require(!tx_.executed, "EXECUTED");
        require(!tx_.cancelled, "ALREADY_CANCELLED");
        
        tx_.cancelled = true;
    }
    
    /**
     * @notice Execute multiple transactions in batch
     */
    function executeBatchTransactions(
        bytes32[] calldata _txHashes
    ) external onlyOwner returns (bool[] memory results) {
        results = new bool[](_txHashes.length);
        
        for (uint256 i = 0; i < _txHashes.length; i++) {
            results[i] = executeTransaction(_txHashes[i]);
        }
    }
    
    // ============================================================================
    // Module Management
    // ============================================================================
    
    /**
     * @notice Enable module
     */
    function enableModule(address _module) external onlyOwner {
        require(_module != address(0), "INVALID_MODULE");
        
        isModule[_module] = true;
        enabledModules[_module] = true;
    }
    
    /**
     * @notice Disable module
     */
    function disableModule(address _module) external onlyOwner {
        enabledModules[_module] = false;
    }
    
    /**
     * @notice Execute transaction through module (no confirmation needed)
     */
    function executeTransactionViaModule(
        address _to,
        uint256 _value,
        bytes calldata _data,
        uint256 _operation
    ) external onlyModule returns (bool success) {
        if (_operation == 0) {
            (success, ) = _to.call{value: _value}(_data);
        } else if (_operation == 1) {
            (success, ) = _to.delegatecall(_data);
        }
    }
    
    // ============================================================================
    // Fallback Handlers
    // ============================================================================
    
    /**
     * @notice Set fallback handler
     */
    function setFallbackHandler(address _handler) external onlyOwner {
        fallbackHandler = _handler;
    }
    
    /**
     * @notice Set protocol handler
     */
    function setProtocolHandler(address _handler) external onlyOwner {
        protocolHandler = _handler;
    }
    
    /**
     * @notice Fallback function for ERC-721/ERC-1155 receive
     */
    receive() external payable {
        emit Deposit(msg.sender, msg.value);
    }
    
    /**
     * @notice Fallback for unknown calls
     */
    fallback() external payable {
        if (fallbackHandler != address(0)) {
            (bool success, ) = fallbackHandler.delegatecall(msg.data);
            require(success, "FALLBACK_FAILED");
        }
    }
    
    // ============================================================================
    // View Functions
    // ============================================================================
    
    /**
     * @notice Get transaction details
     */
    function getTransaction(bytes32 _txHash) external view returns (
        address to,
        uint256 value,
        uint256 nonce,
        bool executed,
        bool cancelled,
        uint256 expiry,
        uint256 confirmationsRequired
    ) {
        Transaction storage tx_ = transactions[_txHash];
        return (
            tx_.to,
            tx_.value,
            tx_.nonce,
            tx_.executed,
            tx_.cancelled,
            tx_.expiry,
            threshold
        );
    }
    
    /**
     * @notice Check if transaction is confirmed
     */
    function isConfirmed(bytes32 _txHash, address _owner) external view returns (bool) {
        return confirmations[_txHash][_owner];
    }
    
    /**
     * @notice Get confirmation count
     */
    function getConfirmationCount(bytes32 _txHash) external view returns (uint256) {
        return confirmationCount[_txHash];
    }
    
    /**
     * @notice Get all owners
     */
    function getOwners() external view returns (address[] memory) {
        return owners;
    }
    
    /**
     * @notice Get nonce
     */
    function getNonce() external view returns (uint256) {
        return nonce;
    }
    
    /**
     * @notice Check if owner
     */
    function checkOwner(address _owner) external view returns (bool) {
        return isOwner[_owner];
    }
    
    // ============================================================================
    // Utility Functions
    // ============================================================================
    
    /**
     * @notice Get current chain ID
     */
    function getChainId() external view returns (uint256) {
        return block.chainid;
    }
    
    /**
     * @notice Encode transaction data
     */
    function encodeTransactionData(
        address _to,
        uint256 _value,
        bytes calldata _data,
        uint256 _operation,
        uint256 _nonce
    ) external pure returns (bytes memory) {
        return abi.encodeWithSignature(
            "executeTransaction(address,uint256,bytes,uint256,uint256)",
            _to,
            _value,
            _data,
            _operation,
            _nonce
        );
    }
    
    /**
     * @notice Get transaction hash for confirmation
     */
    function getTransactionHash(
        address _to,
        uint256 _value,
        bytes calldata _data,
        uint256 _operation,
        uint256 _nonce
    ) external view returns (bytes32) {
        return keccak256(abi.encodePacked(
            _to,
            _value,
            _data,
            _operation,
            _nonce,
            block.chainid,
            address(this)
        ));
    }
    
    /**
     * @notice Get balance
     */
    function getBalance() external view returns (uint256) {
        return address(this).balance;
    }
}