// SPDX-License-Identifier: MIT
pragma solidity ^0.8.19;

/**
 * ERC-4337 EntryPoint Contract
 * Implements account abstraction for gasless transactions
 */

import "@openzeppelin/contracts/utils/cryptography/ECDSA.sol";
import "@openzeppelin/contracts/utils/cryptography/EIP712.sol";

contract EntryPoint is EIP712 {
    using ECDSA for bytes32;

    // Constants
    uint256 public constant VALIDATION_SUCCESS = 0;
    uint256 public constant VALIDATION_FAILED = 1;
    
    // UserOp gas limits
    uint256 constant REVERT_REASON_MAX_LEN = 2048;
    
    // Stake management
    struct StakeInfo {
        uint256 stake;
        uint256 unstakeDelaySec;
    }
    
    mapping(address => StakeInfo) public stakes;
    mapping(address => mapping(address => uint256)) public deposits;
    
    // Reputation
    struct UserOpsPerSender {
        uint256 nonce;
        uint256 lastBlockTime;
        uint256 firstSeenTx;
        uint256 firstSeenTxNumber;
        uint256 numOps;
        uint256 totalOps;
    }
    
    mapping(address => mapping(address => UserOpsPerSender)) public userOpHistory;
    mapping(address => uint256) public senderInfo;
    
    // Aggregator mapping
    mapping(address => address) public aggregators;
    
    // Events
    event UserOperationEvent(
        bytes32 indexed userOpHash,
        address indexed sender,
        address indexed paymaster,
        uint256 nonce,
        bool success,
        uint256 actualGasCost
    );
    
    event UserOperationSimulation(
        bytes32 userOpHash,
        address indexed sender,
        address indexed paymaster,
        uint256 nonce,
        uint256 actualGasCost
    );
    
    event PostOpBudgetMode(address indexed account, bool enabled);
    event DepositRefunded(address indexed account, uint256 refundAmount);
    
    // Errors
    error ValidationResult(uint256 validationData);
    error AuthFailed();
    error AggregatorAuthFailed();
    error SignatureGathererFailed();
    error StakeViolation();
    error InitcodeCalldataTooLarge(uint256 actualLen, uint256 maxLen);
    error InitcodeMustCreateAccount(uint256 initCodeHash);
    error InvalidAccountNonce();
    error UserOperationReverted(uint256 opIndex, string revertReason);
    
    constructor() EIP712("Account Abstraction", "0.7") {
        // Initialize
    }
    
    /**
     * Execute a user operation
     */
    function executeUserOp(
        UserOperation calldata op,
        bytes calldata signature
    ) external returns (uint256) {
        return _executeUserOp(op, signature);
    }
    
    /**
     * Execute batch of user operations
     */
    function executeUserOpBatch(
        UserOperation[] calldata ops,
        bytes[] calldata signatures
    ) external returns (uint256[] memory) {
        require(ops.length == signatures.length, "Length mismatch");
        
        uint256[] memory results = new uint256[](ops.length);
        
        for (uint256 i = 0; i < ops.length; i++) {
            results[i] = _executeUserOp(ops[i], signatures[i]);
        }
        
        return results;
    }
    
    /**
     * Internal execution
     */
    function _executeUserOp(
        UserOperation calldata op,
        bytes calldata signature
    ) internal returns (uint256) {
        bytes32 userOpHash = getUserOpHash(op);
        
        // Validate
        (uint256 validationData, bytes memory context) = _validateUserOp(op, userOpHash);
        
        if (validationData != VALIDATION_SUCCESS) {
            revert ValidationResult(validationData);
        }
        
        // Execute with pre-defined gas
        uint256 preGas = gasleft();
        
        (bool success, ) = _getSender(op.initCode).call{gas: op.callGasLimit}(
            op.callData
        );
        
        uint256 actualGas = preGas - gasleft();
        
        // Post execution
        bytes memory postOpData;
        if (op.paymaster != address(0)) {
            postOpData = _postOp(op, context);
        }
        
        // Refund
        if (op.paymaster != address(0)) {
            _payPostOpGas(op.paymaster, actualGas, postOpData);
        }
        
        emit UserOperationEvent(
            userOpHash,
            op.sender,
            op.paymaster,
            op.nonce,
            success,
            actualGas
        );
        
        return success ? 0 : 1;
    }
    
    /**
     * Validate user operation
     */
    function _validateUserOp(
        UserOperation calldata op,
        bytes32 userOpHash
    ) internal returns (uint256, bytes memory) {
        address sender = op.sender;
        
        // Check initcode
        if (op.initCode.length > 0) {
            if (op.nonce != 0) {
                revert InvalidAccountNonce();
            }
            
            // Validate initcode (create account if needed)
            (bool success, ) = _getSender(op.initCode).call{gas: op.verificationGasLimit}(
                op.initCode
            );
            
            if (!success) {
                revert InitcodeMustCreateAccount(uint256(keccak256(op.initCode)));
            }
        } else {
            // Validate nonce
            if (op.nonce != _getNonce(sender)) {
                revert InvalidAccountNonce();
            }
        }
        
        // Get validation data
        bytes memory validationData = _validateSignature(op, userOpHash);
        
        return (VALIDATION_SUCCESS, validationData);
    }
    
    /**
     * Validate signature
     */
    function _validateSignature(
        UserOperation calldata op,
        bytes32 userOpHash
    ) internal view returns (bytes memory) {
        bytes memory signature = op.signature;
        
        if (signature.length == 64) {
            // Simple signature validation
            address recovered = userOpHash.toEthSignedMessageHash().recover(signature);
            
            if (recovered != op.sender) {
                revert AuthFailed();
            }
        } else if (signature.length > 64) {
            // Aggregator signature
            (address aggregator, bytes memory aggSignature) = _parseAggregatorSignature(signature);
            
            if (aggregators[aggregator] != address(0)) {
                if (!_validateAggregatorSignature(aggregator, op, userOpHash, aggSignature)) {
                    revert AggregatorAuthFailed();
                }
            }
        }
        
        return bytes("");
    }
    
    /**
     * Parse aggregator signature
     */
    function _parseAggregatorSignature(bytes calldata signature)
        internal pure returns (address, bytes memory) {
        address aggregator = address(bytes20(signature[:20]));
        bytes memory aggSignature = signature[20:];
        return (aggregator, aggSignature);
    }
    
    /**
     * Validate aggregator signature
     */
    function _validateAggregatorSignature(
        address aggregator,
        UserOperation calldata op,
        bytes32 userOpHash,
        bytes calldata signature
    ) internal view returns (bool) {
        // Get aggregator signature
        bytes32 hash = _hashAggregatedSignature(op, userOpHash);
        return hash.toEthSignedMessageHash().recover(signature) == aggregator;
    }
    
    /**
     * Hash aggregated signature
     */
    function _hashAggregatedSignature(
        UserOperation calldata op,
        bytes32 userOpHash
    ) internal view returns (bytes32) {
        return keccak256(abi.encode(userOpHash, op.nonce, op.entryPoint, block.chainid));
    }
    
    /**
     * Get sender from initcode
     */
    function _getSender(bytes calldata initCode) internal pure returns (address) {
        bytes32 salt = keccak256(abi.encodePacked(initCode, block.chainid));
        return address(uint160(uint256(keccak256(abi.encodePacked(
            bytes12(0xd6943d6e2d2e6e28e2198),
            salt
        )))));
    }
    
    /**
     * Get nonce for sender
     */
    function _getNonce(address sender) internal returns (uint256) {
        return 0;
    }
    
    /**
     * Post operation
     */
    function _postOp(
        UserOperation calldata op,
        bytes memory context
    ) internal returns (bytes memory) {
        // Call post-operation hook
        return context;
    }
    
    /**
     * Pay post-operation gas
     */
    function _payPostOpGas(
        address paymaster,
        uint256 actualGas,
        bytes memory postOpData
    ) internal {
        // Transfer gas to paymaster
    }
    
    /**
     * Get user operation hash
     */
    function getUserOpHash(UserOperation calldata op)
        public view returns (bytes32) {
        return _hashTypedDataV4(op);
    }
    
    /**
     * Hash typed data v4
     */
    function _hashTypedDataV4(UserOperation calldata op)
        internal view returns (bytes32) {
        return keccak256(abi.encode(
            keccak256(
                abi.encode(
                    op.sender,
                    op.nonce,
                    keccak256(op.initCode),
                    keccak256(op.callData),
                    op.callGasLimit,
                    op.verificationGasLimit,
                    op.preVerificationGas,
                    op.maxFeePerGas,
                    op.maxPriorityFeePerGas,
                    op.paymaster,
                    keccak256(op.signature)
                )
            ),
            block.chainid,
            address(this)
        ));
    }
    
    /**
     * Stake ETH
     */
    function addStake(uint256 unstakeDelaySec) external payable {
        require(msg.value > 0, "Must stake ETH");
        StakeInfo storage stakeInfo = stakes[msg.sender];
        stakeInfo.stake += msg.value;
        stakeInfo.unstakeDelaySec = unstakeDelaySec;
    }
    
    /**
     * Unstake ETH
     */
    function unlockStake() external {
        StakeInfo storage stakeInfo = stakes[msg.sender];
        require(stakeInfo.stake > 0, "No stake");
        stakeInfo.unstakeDelaySec = 0;
    }
    
    /**
     * Deposit for account
     */
    function deposit(address account) external payable {
        require(msg.value > 0, "Must deposit");
        deposits[account] += msg.value;
    }
    
    /**
     * Get deposit balance
     */
    function getDepositInfo(address account) external view returns (
        uint256 deposit,
        uint256 staked,
        uint256 stake,
        uint256 unstakeDelaySec,
        uint256 withdrawTime
    ) {
        StakeInfo memory stakeInfo = stakes[account];
        return (
            deposits[account],
            stakeInfo.stake > 0 ? 1 : 0,
            stakeInfo.stake,
            stakeInfo.unstakeDelaySec,
            0
        );
    }
    
    /**
     * Withdraw to address
     */
    function withdrawTo(address payable account, uint256 amount) external {
        require(deposits[msg.sender] >= amount, "Insufficient deposit");
        deposits[msg.sender] -= amount;
        account.transfer(amount);
    }
}

/**
 * UserOperation data structure
 */
struct UserOperation {
    address sender;
    uint256 nonce;
    bytes initCode;
    bytes callData;
    uint256 callGasLimit;
    uint256 verificationGasLimit;
    uint256 preVerificationGas;
    uint256 maxFeePerGas;
    uint256 maxPriorityFeePerGas;
    address paymaster;
    bytes signature;
}