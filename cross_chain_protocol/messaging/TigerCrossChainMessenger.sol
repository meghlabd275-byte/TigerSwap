// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "../libraries/SafeMath.sol";

/**
 * @title TigerCrossChainMessenger
 * @notice Cross-Chain Messaging Protocol
 * @dev Secure message passing across chains
 * 
 * Features:
 * - Message verification with validators
 * - Optimistic confirmation
 * - Message batching
 * - Failed message retry
 * - Automatic retry mechanism
 * - Emergency message processing
 */
contract TigerCrossChainMessenger {
    using SafeMath for uint256;

    // ============================================================================
    // Constants
    // ============================================================================
    
    uint256 constant MAX_VALIDATORS = 25;
    uint256 constant CONFIRMATION_THRESHOLD = 3;
    uint256 constant MESSAGE_EXPIRY = 172800; // 48 hours
    uint256 constant BATCH_SIZE = 50;
    uint256 constant MAX_RETRY = 3;
    
    // Message status
    uint8 constant STATUS_PENDING = 1;
    uint8 constant STATUS_CONFIRMED = 2;
    uint8 constant STATUS_EXECUTED = 3;
    uint8 constant STATUS_FAILED = 4;
    uint8 constant STATUS_EXPIRED = 5;
    
    // ============================================================================
    // State Variables
    // ============================================================================
    
    // Governance
    address public governance;
    address public pendingGovernance;
    
    // Supported chains
    mapping(uint256 => bool) public supportedChains;
    uint256[] public chainIds;
    
    // Validators
    mapping(address => bool) public isValidator;
    address[] public validators;
    uint256 public validatorCount;
    uint256 public confirmationThreshold = CONFIRMATION_THRESHOLD;
    
    // Messages
    mapping(bytes32 => CrossChainMessage) public messages;
    mapping(bytes32 => mapping(address => bool)) public messageConfirmations;
    mapping(bytes32 => uint256) public confirmationCount;
    mapping(bytes32 => uint256) public executionCount;
    
    // Message IDs
    bytes32[] public messageIds;
    mapping(uint256 => bytes32) public messageNonce;
    uint256 public nextNonce;
    
    // Failed messages
    mapping(bytes32 => uint256) public failedAttempts;
    mapping(bytes32 => uint256) public lastAttemptTime;
    
    // Emergency
    bool public emergencyMode;
    bool public pauseProcessing;
    
    // Events
    event MessageSent(
        bytes32 indexed messageId,
        uint256 sourceChain,
        uint256 destChain,
        address sender,
        address recipient,
        bytes data
    );
    event MessageConfirmed(
        bytes32 indexed messageId,
        address indexed validator
    );
    event MessageExecuted(
        bytes32 indexed messageId,
        bool success,
        bytes returnData
    );
    event MessageFailed(
        bytes32 indexed messageId,
        uint256 attempt
    );
    event ValidatorAdded(address indexed validator);
    event ValidatorRemoved(address indexed validator);
    event ChainAdded(uint256 indexed chainId);
    event ChainRemoved(uint256 indexed chainId);
    
    // ============== Structs ==============
    
    struct CrossChainMessage {
        bytes32 id;
        uint256 sourceChain;
        uint256 destChain;
        address sender;
        address recipient;
        bytes data;
        uint256 amount;
        uint256 nonce;
        uint256 timestamp;
        uint256 expiry;
        uint8 status;
        bytes32 refundAddress;
        uint256 gasLimit;
    }
    
    struct MessageProof {
        bytes32 messageId;
        address[] validators;
        bytes[] signatures;
        uint256 timestamp;
    }
    
    struct FailedMessage {
        bytes32 messageId;
        uint256 attemptCount;
        uint256 lastAttempt;
        bytes revertReason;
    }
    
    // ============== Modifier ==============
    
    modifier onlyGovernance() {
        require(msg.sender == governance, "ONLY_GOVERNANCE");
        _;
    }
    
    modifier onlyValidator() {
        require(isValidator[msg.sender], "ONLY_VALIDATOR");
        _;
    }
    
    // ============== Constructor ==============
    
    constructor() {
        governance = msg.sender;
        isValidator[msg.sender] = true;
        validators.push(msg.sender);
        validatorCount = 1;
    }
    
    // ============================================================================
    // Chain Management
    // ============================================================================
    
    /**
     * @notice Add supported chain
     */
    function addChain(uint256 _chainId) external onlyGovernance {
        require(!supportedChains[_chainId], "CHAIN_EXISTS");
        
        supportedChains[_chainId] = true;
        chainIds.push(_chainId);
        
        emit ChainAdded(_chainId);
    }
    
    /**
     * @notice Remove supported chain
     */
    function removeChain(uint256 _chainId) external onlyGovernance {
        require(supportedChains[_chainId], "CHAIN_NOT_EXISTS");
        
        supportedChains[_chainId] = false;
        
        emit ChainRemoved(_chainId);
    }
    
    /**
     * @notice Check if chain is supported
     */
    function isChainSupported(uint256 _chainId) external view returns (bool) {
        return supportedChains[_chainId];
    }
    
    // ============================================================================
    // Validator Management
    // ============================================================================
    
    /**
     * @notice Add validator
     */
    function addValidator(address _validator) external onlyGovernance {
        require(_validator != address(0), "INVALID_VALIDATOR");
        require(!isValidator[_validator], "ALREADY_VALIDATOR");
        require(validatorCount < MAX_VALIDATORS, "MAX_VALIDATORS");
        
        isValidator[_validator] = true;
        validators.push(_validator);
        validatorCount++;
        
        emit ValidatorAdded(_validator);
    }
    
    /**
     * @notice Remove validator
     */
    function removeValidator(address _validator) external onlyGovernance {
        require(isValidator[_validator], "NOT_VALIDATOR");
        require(_validator != governance, "CANNOT_REMOVE_GOVERNANCE");
        
        isValidator[_validator] = false;
        validatorCount--;
        
        emit ValidatorRemoved(_validator);
    }
    
    /**
     * @notice Update confirmation threshold
     */
    function updateConfirmationThreshold(uint256 _threshold) external onlyGovernance {
        require(_threshold > 0, "INVALID_THRESHOLD");
        require(_threshold <= validatorCount, "THRESHOLD_TOO_HIGH");
        
        confirmationThreshold = _threshold;
    }
    
    /**
     * @notice Get validator count
     */
    function getValidatorCount() external view returns (uint256) {
        return validatorCount;
    }
    
    // ============================================================================
    // Message Sending
    // ============================================================================
    
    /**
     * @notice Send cross-chain message
     */
    function sendMessage(
        uint256 _destChain,
        address _recipient,
        bytes calldata _data,
        uint256 _amount,
        uint256 _gasLimit,
        bytes32 _refundAddress
    ) external payable returns (bytes32 messageId) {
        require(!emergencyMode, "EMERGENCY_MODE");
        require(!pauseProcessing, "PROCESSING_PAUSED");
        require(supportedChains[_destChain], "UNSUPPORTED_CHAIN");
        require(_recipient != address(0), "INVALID_RECIPIENT");
        
        messageId = keccak256(abi.encodePacked(
            block.chainid,
            _destChain,
            msg.sender,
            _recipient,
            _data,
            _amount,
            nextNonce,
            block.timestamp
        ));
        
        messages[messageId] = CrossChainMessage({
            id: messageId,
            sourceChain: block.chainid,
            destChain: _destChain,
            sender: msg.sender,
            recipient: _recipient,
            data: _data,
            amount: _amount,
            nonce: nextNonce,
            timestamp: block.timestamp,
            expiry: block.timestamp + MESSAGE_EXPIRY,
            status: STATUS_PENDING,
            refundAddress: _refundAddress != bytes32(0) ? _refundAddress : bytes32(uint256(uint160(msg.sender))),
            gasLimit: _gasLimit > 0 ? _gasLimit : 200000
        });
        
        messageIds.push(messageId);
        messageNonce[nextNonce] = messageId;
        nextNonce++;
        
        emit MessageSent(
            messageId,
            block.chainid,
            _destChain,
            msg.sender,
            _recipient,
            _data
        );
    }
    
    /**
     * @notice Send batch messages
     */
    function sendBatchMessages(
        uint256[] calldata _destChains,
        address[] calldata _recipients,
        bytes[] calldata _datas,
        uint256[] calldata _amounts
    ) external payable returns (bytes32[] memory messageIds_) {
        require(_destChains.length == _recipients.length, "LENGTH_MISMATCH");
        require(_destChains.length <= BATCH_SIZE, "TOO_MANY");
        
        messageIds_ = new bytes32[](_destChains.length);
        
        for (uint256 i = 0; i < _destChains.length; i++) {
            messageIds_[i] = sendMessage(
                _destChains[i],
                _recipients[i],
                _datas[i],
                _amounts[i],
                0,
                bytes32(0)
            );
        }
    }
    
    // ============================================================================
    // Message Confirmation
    // ============================================================================
    
    /**
     * @notice Confirm message (validators only)
     */
    function confirmMessage(bytes32 _messageId) external onlyValidator {
        CrossChainMessage storage message = messages[_messageId];
        require(message.id != bytes32(0), "INVALID_MESSAGE");
        require(message.status == STATUS_PENDING, "NOT_PENDING");
        
        if (!messageConfirmations[_messageId][msg.sender]) {
            messageConfirmations[_messageId][msg.sender] = true;
            confirmationCount[_messageId]++;
            
            emit MessageConfirmed(_messageId, msg.sender);
            
            // Auto-execute if threshold reached
            if (confirmationCount[_messageId] >= confirmationThreshold) {
                message.status = STATUS_CONFIRMED;
            }
        }
    }
    
    /**
     * @notice Batch confirm messages
     */
    function batchConfirmMessages(bytes32[] calldata _messageIds) external onlyValidator {
        for (uint256 i = 0; i < _messageIds.length; i++) {
            confirmMessage(_messageIds[i]);
        }
    }
    
    // ============================================================================
    // Message Execution
    // ============================================================================
    
    /**
     * @notice Execute confirmed message
     */
    function executeMessage(bytes32 _messageId) external returns (bool success, bytes memory returnData) {
        require(!emergencyMode, "EMERGENCY_MODE");
        require(!pauseProcessing, "PROCESSING_PAUSED");
        
        CrossChainMessage storage message = messages[_messageId];
        require(message.id != bytes32(0), "INVALID_MESSAGE");
        require(message.status == STATUS_CONFIRMED, "NOT_CONFIRMED");
        require(block.timestamp <= message.expiry, "EXPIRED");
        
        // Execute message
        if (message.amount > 0) {
            (success, ) = message.recipient.call{value: message.amount, gas: message.gasLimit}(message.data);
        } else {
            (success, ) = message.recipient.call{gas: message.gasLimit}(message.data);
        }
        
        if (success) {
            message.status = STATUS_EXECUTED;
            executionCount[_messageId]++;
            
            emit MessageExecuted(_messageId, true, "");
        } else {
            // Handle failure
            failedAttempts[_messageId]++;
            lastAttemptTime[_messageId] = block.timestamp;
            
            if (failedAttempts[_messageId] >= MAX_RETRY) {
                message.status = STATUS_FAILED;
            }
            
            emit MessageFailed(_messageId, failedAttempts[_messageId]);
        }
        
        returnData = "";
    }
    
    /**
     * @notice Retry failed message
     */
    function retryMessage(bytes32 _messageId) external {
        CrossChainMessage storage message = messages[_messageId];
        require(message.status == STATUS_FAILED, "NOT_FAILED");
        
        // Reset and try again
        message.status = STATUS_CONFIRMED;
        
        executeMessage(_messageId);
    }
    
    /**
     * @notice Cancel and refund message
     */
    function cancelAndRefund(bytes32 _messageId) external {
        CrossChainMessage storage message = messages[_messageId];
        require(message.id != bytes32(0), "INVALID_MESSAGE");
        require(
            message.sender == msg.sender || isValidator[msg.sender],
            "NOT_AUTHORIZED"
        );
        require(message.status == STATUS_FAILED || message.status == STATUS_EXPIRED, "CANNOT_REFUND");
        
        // Refund to refund address
        if (message.amount > 0) {
            payable(address(uint160(uint256(message.refundAddress)))).transfer(message.amount);
        }
        
        message.status = STATUS_EXPIRED;
    }
    
    // ============================================================================
    // Emergency Controls
    // ============================================================================
    
    /**
     * @notice Enable emergency mode
     */
    function enableEmergencyMode() external onlyGovernance {
        emergencyMode = true;
        pauseProcessing = true;
    }
    
    /**
     * @notice Disable emergency mode
     */
    function disableEmergencyMode() external onlyGovernance {
        emergencyMode = false;
    }
    
    /**
     * @notice Pause message processing
     */
    function pauseProcessing() external onlyGovernance {
        pauseProcessing = true;
    }
    
    /**
     * @notice Resume message processing
     */
    function resumeProcessing() external onlyGovernance {
        pauseProcessing = false;
    }
    
    // ============================================================================
    // Governance
    // ============================================================================
    
    /**
     * @notice Transfer governance
     */
    function transferGovernance(address _newGovernance) external onlyGovernance {
        pendingGovernance = _newGovernance;
    }
    
    /**
     * @notice Accept governance
     */
    function acceptGovernance() external {
        require(msg.sender == pendingGovernance, "NOT_PENDING");
        
        governance = msg.sender;
        
        // Add new governance as validator
        isValidator[msg.sender] = true;
        validators.push(msg.sender);
        validatorCount++;
        
        delete pendingGovernance;
    }
    
    // ============================================================================
    // View Functions
    // ============================================================================
    
    /**
     * @notice Get message details
     */
    function getMessage(bytes32 _messageId) external view returns (
        uint256 sourceChain,
        uint256 destChain,
        address sender,
        address recipient,
        uint256 amount,
        uint8 status,
        uint256 expiry,
        uint256 confirmations
    ) {
        CrossChainMessage storage message = messages[_messageId];
        return (
            message.sourceChain,
            message.destChain,
            message.sender,
            message.recipient,
            message.amount,
            message.status,
            message.expiry,
            confirmationCount[_messageId]
        );
    }
    
    /**
     * @notice Check if message is confirmed
     */
    function isMessageConfirmed(bytes32 _messageId) external view returns (bool) {
        return confirmationCount[_messageId] >= confirmationThreshold;
    }
    
    /**
     * @notice Get message count
     */
    function getMessageCount() external view returns (uint256) {
        return messageIds.length;
    }
    
    /**
     * @notice Get all supported chains
     */
    function getSupportedChains() external view returns (uint256[] memory) {
        return chainIds;
    }
    
    /**
     * @notice Get all validators
     */
    function getValidators() external view returns (address[] memory) {
        return validators;
    }
    
    /**
     * @notice Get failed attempt count
     */
    function getFailedAttempts(bytes32 _messageId) external view returns (uint256) {
        return failedAttempts[_messageId];
    }
    
    // ============================================================================
    // Utility
    // ============================================================================
    
    /**
     * @notice Calculate message hash
     */
    function getMessageHash(
        uint256 _sourceChain,
        uint256 _destChain,
        address _sender,
        address _recipient,
        bytes calldata _data,
        uint256 _amount,
        uint256 _nonce
    ) external pure returns (bytes32) {
        return keccak256(abi.encodePacked(
            _sourceChain,
            _destChain,
            _sender,
            _recipient,
            _data,
            _amount,
            _nonce
        ));
    }
    
    /**
     * @notice Get current nonce
     */
    function getNonce() external view returns (uint256) {
        return nextNonce;
    }
}