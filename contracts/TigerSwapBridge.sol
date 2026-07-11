// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/**
 * @title TigerSwapBridge
 * @notice Cross-chain bridge contract
 */
contract TigerSwapBridge {
    mapping(bytes32 => bool) public processedMessages;
    mapping(address => bool) public whitelistedTokens;
    mapping(uint256 => bool) public supportedChains;
    
    address public owner;
    uint256 public fee = 5; // 0.05%
    uint256 public minAmount = 100;
    
    event BridgeInitiated(address indexed sender, uint256 toChain, address token, uint256 amount, bytes32 messageId);
    event BridgeCompleted(address indexed receiver, uint256 fromChain, address token, uint256 amount, bytes32 messageId);
    event TokenWhitelisted(address indexed token, bool status);
    event ChainEnabled(uint256 indexed chainId, bool status);
    
    modifier onlyOwner() { require(msg.sender == owner, "NOT_OWNER"); _; }
    
    constructor() { owner = msg.sender; }
    
    function enableChain(uint256 chainId, bool status) external onlyOwner {
        supportedChains[chainId] = status;
        emit ChainEnabled(chainId, status);
    }
    
    function whitelistToken(address token, bool status) external onlyOwner {
        whitelistedTokens[token] = status;
        emit TokenWhitelisted(token, status);
    }
    
    function setFee(uint256 _fee) external onlyOwner {
        require(_fee <= 100, "FEE_TOO_HIGH");
        fee = _fee;
    }
    
    function setMinAmount(uint256 _minAmount) external onlyOwner {
        minAmount = _minAmount;
    }
    
    function bridge(
        uint256 toChain,
        address token,
        uint256 amount,
        address receiver
    ) external payable {
        require(supportedChains[toChain], "CHAIN_NOT_SUPPORTED");
        require(whitelistedTokens[token], "TOKEN_NOT_WHITELISTED");
        require(amount >= minAmount, "AMOUNT_TOO_LOW");
        
        uint256 bridgeFee = (amount * fee) / 10000;
        uint256 transferAmount = amount - bridgeFee;
        
        IERC20(token).transferFrom(msg.sender, address(this), transferAmount);
        
        bytes32 messageId = keccak256(abi.encodePacked(
            msg.sender, toChain, token, amount, receiver, block.timestamp
        ));
        
        emit BridgeInitiated(msg.sender, toChain, token, transferAmount, messageId);
    }
    
    function completeBridge(
        address receiver,
        address token,
        uint256 amount,
        bytes32 messageId,
        bytes memory proof
    ) external onlyOwner {
        require(!processedMessages[messageId], "MESSAGE_ALREADY_PROCESSED");
        
        processedMessages[messageId] = true;
        
        IERC20(token).transfer(receiver, amount);
        
        emit BridgeCompleted(receiver, 0, token, amount, messageId);
    }
    
    function rescueTokens(address token, uint256 amount) external onlyOwner {
        IERC20(token).transfer(owner, amount);
    }
    
    function rescueETH() external onlyOwner {
        payable(owner).transfer(address(this).balance);
    }
}

interface IERC20 {
    function transferFrom(address from, address to, uint256 value) external returns (bool);
    function transfer(address to, uint256 value) external returns (bool);
    function balanceOf(address account) external view returns (uint256);
}
