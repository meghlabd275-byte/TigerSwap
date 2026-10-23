// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./TigerSwapInterfaces.sol";

/**
 * @title TigerSwapBridge
 * @notice Lock-and-mint bridge for the TigerBridge relayer network.
 *
 * Outbound transfers lock tokens and emit a message that relayers observe.
 * Inbound transfers release tokens only when a supermajority of the validator
 * set has signed the exact message, so a single compromised relayer cannot
 * mint value.
 */
contract TigerSwapBridge {
    uint256 public constant MAX_FEE_BPS = 100; // 1%
    uint256 public constant BPS = 10_000;

    /// @notice Inbound payload signed by the relayer validator set.
    struct BridgeMessage {
        bytes32 messageId;
        uint256 fromChain;
        uint256 toChain;
        address token;
        uint256 amount;
        address receiver;
    }

    address public owner;
    uint256 public feeBps = 5; // 0.05%
    uint256 public minAmount = 100;
    uint256 public validatorThreshold;
    uint256 public chainId;

    mapping(uint256 => bool) public supportedChains;
    mapping(address => bool) public whitelistedTokens;
    mapping(address => bool) public isValidator;
    mapping(bytes32 => bool) public processedMessages;
    mapping(bytes32 => bool) public outboundMessages;
    mapping(address => uint256) public collectedFees;

    event BridgeInitiated(
        address indexed sender,
        uint256 indexed toChain,
        address token,
        uint256 amount,
        uint256 fee,
        address receiver,
        bytes32 messageId
    );
    event BridgeCompleted(
        address indexed receiver,
        uint256 indexed fromChain,
        address token,
        uint256 amount,
        bytes32 messageId
    );
    event TokenWhitelisted(address indexed token, bool status);
    event ChainEnabled(uint256 indexed chainId, bool status);
    event ValidatorUpdated(address indexed validator, bool status);
    event ThresholdUpdated(uint256 threshold);
    event FeeUpdated(uint256 feeBps);
    event FeesWithdrawn(address indexed token, address indexed to, uint256 amount);

    modifier onlyOwner() {
        require(msg.sender == owner, "TigerSwap: NOT_OWNER");
        _;
    }

    constructor(uint256 _chainId) {
        require(_chainId != 0, "TigerSwap: ZERO_CHAIN");
        owner = msg.sender;
        chainId = _chainId;
    }

    // ============ Admin ============

    function enableChain(uint256 _chainId, bool status) external onlyOwner {
        require(_chainId != 0, "TigerSwap: ZERO_CHAIN");
        require(_chainId != chainId, "TigerSwap: SELF_CHAIN");
        supportedChains[_chainId] = status;
        emit ChainEnabled(_chainId, status);
    }

    function whitelistToken(address token, bool status) external onlyOwner {
        require(token != address(0), "TigerSwap: ZERO_TOKEN");
        whitelistedTokens[token] = status;
        emit TokenWhitelisted(token, status);
    }

    function setValidator(address validator, bool status) external onlyOwner {
        require(validator != address(0), "TigerSwap: ZERO_ADDRESS");
        isValidator[validator] = status;
        emit ValidatorUpdated(validator, status);
    }

    function setValidatorThreshold(uint256 threshold) external onlyOwner {
        require(threshold > 0, "TigerSwap: ZERO_THRESHOLD");
        validatorThreshold = threshold;
        emit ThresholdUpdated(threshold);
    }

    function setFee(uint256 _feeBps) external onlyOwner {
        require(_feeBps <= MAX_FEE_BPS, "TigerSwap: FEE_TOO_HIGH");
        feeBps = _feeBps;
        emit FeeUpdated(_feeBps);
    }

    function setMinAmount(uint256 _minAmount) external onlyOwner {
        minAmount = _minAmount;
    }

    function transferOwnership(address newOwner) external onlyOwner {
        require(newOwner != address(0), "TigerSwap: ZERO_ADDRESS");
        owner = newOwner;
    }

    function withdrawFees(address token, address to) external onlyOwner {
        require(to != address(0), "TigerSwap: ZERO_ADDRESS");
        uint256 amount = collectedFees[token];
        require(amount > 0, "TigerSwap: NO_FEES");
        collectedFees[token] = 0;
        require(IERC20(token).transfer(to, amount), "TigerSwap: TRANSFER_FAILED");
        emit FeesWithdrawn(token, to, amount);
    }

    // ============ Outbound ============

    /// @notice Lock `amount` of `token` and publish a bridge message.
    function bridge(
        uint256 toChain,
        address token,
        uint256 amount,
        address receiver,
        uint256 nonce
    ) external returns (bytes32 messageId) {
        require(supportedChains[toChain], "TigerSwap: CHAIN_NOT_SUPPORTED");
        require(whitelistedTokens[token], "TigerSwap: TOKEN_NOT_WHITELISTED");
        require(receiver != address(0), "TigerSwap: ZERO_RECEIVER");
        require(amount >= minAmount, "TigerSwap: AMOUNT_TOO_LOW");

        uint256 fee = amount * feeBps / BPS;
        uint256 transferAmount = amount - fee;
        require(transferAmount > 0, "TigerSwap: AMOUNT_TOO_LOW");

        require(IERC20(token).transferFrom(msg.sender, address(this), amount), "TigerSwap: TRANSFER_FAILED");
        collectedFees[token] += fee;

        messageId = keccak256(abi.encodePacked(
            chainId, toChain, token, transferAmount, receiver, msg.sender, nonce
        ));
        require(!outboundMessages[messageId], "TigerSwap: MESSAGE_EXISTS");
        outboundMessages[messageId] = true;

        emit BridgeInitiated(msg.sender, toChain, token, transferAmount, fee, receiver, messageId);
    }

    // ============ Inbound ============

    /// @notice Release tokens for an inbound message once validators have signed it.
    /// @param message The exact payload signed by validators.
    /// @param signatures Concatenated 65-byte ECDSA signatures (r, s, v).
    function completeBridge(BridgeMessage calldata message, bytes calldata signatures) external {
        require(message.toChain == chainId, "TigerSwap: WRONG_CHAIN");
        require(supportedChains[message.fromChain], "TigerSwap: CHAIN_NOT_SUPPORTED");
        require(whitelistedTokens[message.token], "TigerSwap: TOKEN_NOT_WHITELISTED");
        require(message.receiver != address(0), "TigerSwap: ZERO_RECEIVER");
        require(message.amount > 0, "TigerSwap: ZERO_AMOUNT");
        require(!processedMessages[message.messageId], "TigerSwap: MESSAGE_ALREADY_PROCESSED");

        bytes32 payloadHash = keccak256(abi.encode(
            message.messageId,
            message.fromChain,
            message.toChain,
            message.token,
            message.amount,
            message.receiver
        ));
        bytes32 digest = keccak256(abi.encodePacked("\x19Ethereum Signed Message:\n32", payloadHash));

        require(_countValidSignatures(digest, signatures) >= validatorThreshold,
            "TigerSwap: INSUFFICIENT_SIGNATURES");

        // Set replay protection before the external transfer.
        processedMessages[message.messageId] = true;

        require(IERC20(message.token).transfer(message.receiver, message.amount), "TigerSwap: TRANSFER_FAILED");

        emit BridgeCompleted(message.receiver, message.fromChain, message.token, message.amount, message.messageId);
    }

    function _countValidSignatures(bytes32 digest, bytes calldata signatures)
        internal
        view
        returns (uint256 valid)
    {
        require(signatures.length % 65 == 0, "TigerSwap: BAD_SIGNATURE_LENGTH");

        uint256 count = signatures.length / 65;
        address[] memory seen = new address[](count);

        for (uint256 i = 0; i < count; i++) {
            bytes calldata sig = signatures[i * 65:(i + 1) * 65];
            address signer = _recover(digest, sig);
            if (signer == address(0) || !isValidator[signer]) continue;

            bool duplicate = false;
            for (uint256 j = 0; j < valid; j++) {
                if (seen[j] == signer) {
                    duplicate = true;
                    break;
                }
            }
            if (duplicate) continue;

            seen[valid] = signer;
            valid++;
        }
    }

    function _recover(bytes32 digest, bytes calldata sig) internal pure returns (address) {
        bytes32 r = bytes32(sig[0:32]);
        bytes32 s = bytes32(sig[32:64]);
        uint8 v = uint8(sig[64]);

        // Reject malleable signatures (EIP-2).
        if (uint256(s) > 0x7FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF5D576E7357A4501DDFE92F46681B20A0) {
            return address(0);
        }
        if (v != 27 && v != 28) {
            return address(0);
        }

        return ecrecover(digest, v, r, s);
    }

    // ============ Views ============

    /// @notice Recompute the message id for an outbound payload so relayers can verify.
    function computeMessageId(
        uint256 fromChain,
        uint256 toChain,
        address token,
        uint256 amount,
        address receiver,
        address sender,
        uint256 nonce
    ) external pure returns (bytes32) {
        return keccak256(abi.encodePacked(fromChain, toChain, token, amount, receiver, sender, nonce));
    }

    /// @notice Digest validators must sign for an inbound payload.
    function inboundDigest(BridgeMessage calldata message) external pure returns (bytes32) {
        bytes32 payloadHash = keccak256(abi.encode(
            message.messageId,
            message.fromChain,
            message.toChain,
            message.token,
            message.amount,
            message.receiver
        ));
        return keccak256(abi.encodePacked("\x19Ethereum Signed Message:\n32", payloadHash));
    }
}
