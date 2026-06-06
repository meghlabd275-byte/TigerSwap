// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/**
 * @title TigerWalletConnector
 * @notice Multi-Chain Wallet Connector - Connect Any Wallet + HD Wallet Support
 * @dev Connect external wallets + Create/Import Multi-Chain HD Wallets
 */
contract TigerWalletConnector {
    
    // State Variables
    address public admin;
    address public tigerSwap;
    address public masterController;
    
    // Connected external wallets
    mapping(address => ConnectedWallet) public connectedWallets;
    address[] public connectedWalletAddresses;
    uint256 public connectedWalletCount = 0;
    
    // HD Wallets
    mapping(address => HDWallet) public hdWallets;
    address[] public hdWalletAddresses;
    uint256 public hdWalletCount = 0;
    
    // Chain support
    mapping(address => mapping(uint256 => bool)) public walletChainSupport;
    
    // Approved wallet types
    mapping(string => bool) public approvedWalletTypes;
    
    struct ConnectedWallet {
        address userAddress;
        string walletType;
        bytes signature;
        uint256 connectedAt;
        bool isActive;
    }
    
    struct HDWallet {
        address walletAddress;
        bytes32 rootHash;
        string name;
        uint256 createdAt;
        bool isActive;
    }
    
    event ExternalWalletConnected(address indexed user, string walletType);
    event ExternalWalletDisconnected(address indexed user);
    event HDWalletCreated(address indexed wallet, string name);
    event HDWalletImported(address indexed wallet, string name);
    event ChainEnabled(address indexed wallet, uint256 chainId);
    event SwapExecuted(address indexed wallet, uint256 chainId, address tokenIn, address tokenOut, uint256 amount);
    
    constructor(address _admin, address _tigerSwap, address _masterController) {
        admin = _admin;
        tigerSwap = _tigerSwap;
        masterController = _masterController;
        
        // Approve common wallet types
        approvedWalletTypes["metamask"] = true;
        approvedWalletTypes["trustwallet"] = true;
        approvedWalletTypes["coinbase"] = true;
        approvedWalletTypes["rainbow"] = true;
        approvedWalletTypes["phantom"] = true;
        approvedWalletTypes["keplr"] = true;
    }
    
    // Connect external wallet (MetaMask, Trust Wallet, etc.)
    function connectExternalWallet(string memory walletType, bytes memory signature) external returns (address) {
        require(approvedWalletTypes[walletType], "Wallet type not approved");
        
        ConnectedWallet storage wallet = connectedWallets[msg.sender];
        wallet.userAddress = msg.sender;
        wallet.walletType = walletType;
        wallet.signature = signature;
        wallet.connectedAt = block.timestamp;
        wallet.isActive = true;
        
        if (connectedWallets[msg.sender].connectedAt == 0) {
            connectedWalletAddresses.push(msg.sender);
            connectedWalletCount++;
        }
        
        // Enable default EVM chains
        walletChainSupport[msg.sender][1] = true;   // Ethereum
        walletChainSupport[msg.sender][56] = true;  // BSC
        walletChainSupport[msg.sender][137] = true; // Polygon
        
        emit ExternalWalletConnected(msg.sender, walletType);
        return msg.sender;
    }
    
    // Disconnect external wallet
    function disconnectExternalWallet() external {
        ConnectedWallet storage wallet = connectedWallets[msg.sender];
        require(wallet.isActive, "Not connected");
        wallet.isActive = false;
        emit ExternalWalletDisconnected(msg.sender);
    }
    
    // Add chain support
    function addChainSupport(uint256 chainId) external {
        require(connectedWallets[msg.sender].isActive, "Not connected");
        walletChainSupport[msg.sender][chainId] = true;
        emit ChainEnabled(msg.sender, chainId);
    }
    
    // Create HD wallet with 24-word seed (BIP44)
    function createHDWallet(string memory name, bytes32 rootHash) external returns (address walletAddress) {
        walletAddress = address(uint160(uint256(keccak256(abi.encodePacked(rootHash))));
        
        HDWallet storage wallet = hdWallets[walletAddress];
        require(wallet.walletAddress == address(0), "Wallet exists");
        
        wallet.walletAddress = walletAddress;
        wallet.rootHash = rootHash;
        wallet.name = name;
        wallet.createdAt = block.timestamp;
        wallet.isActive = true;
        
        // Enable all chains
        _enableAllChains(walletAddress);
        
        hdWalletAddresses.push(walletAddress);
        hdWalletCount++;
        
        emit HDWalletCreated(walletAddress, name);
    }
    
    // Import HD wallet
    function importHDWallet(string memory name, bytes32 rootHash) external returns (address) {
        return createHDWallet(name, rootHash);
    }
    
    // Import by private key
    function importWalletByPrivateKey(string memory name, bytes32 privateKey) external returns (address walletAddress) {
        walletAddress = address(uint160(uint256(keccak256(abi.encodePacked(privateKey))));
        
        HDWallet storage wallet = hdWallets[walletAddress];
        require(wallet.walletAddress == address(0), "Wallet exists");
        
        wallet.walletAddress = walletAddress;
        wallet.rootHash = keccak256(abi.encodePacked(privateKey));
        wallet.name = name;
        wallet.createdAt = block.timestamp;
        wallet.isActive = true;
        
        _enableAllChains(walletAddress);
        
        hdWalletAddresses.push(walletAddress);
        hdWalletCount++;
        
        emit HDWalletImported(walletAddress, name);
    }
    
    // Enable all BIP44 chains
    function _enableAllChains(address walletAddress) internal {
        // EVM chains
        walletChainSupport[walletAddress][1] = true;     // Ethereum
        walletChainSupport[walletAddress][56] = true;    // BSC
        walletChainSupport[walletAddress][137] = true;   // Polygon
        walletChainSupport[walletAddress][42161] = true;  // Arbitrum
        walletChainSupport[walletAddress][10] = true;    // Optimism
        walletChainSupport[walletAddress][8453] = true;  // Base
        walletChainSupport[walletAddress][43114] = true;  // Avalanche
        walletChainSupport[walletAddress][250] = true;   // Fantom
        
        // Non-EVM chains
        walletChainSupport[walletAddress][101] = true;    // Solana
        walletChainSupport[walletAddress][102] = true;    // Aptos
        walletChainSupport[walletAddress][103] = true;    // Sui
        walletChainSupport[walletAddress][0] = true;     // Ton
        walletChainSupport[walletAddress][100] = true;    // Near
    }
    
    // Execute swap
    function executeSwap(address wallet, uint256 chainId, address tokenIn, address tokenOut, uint256 amountIn) external returns (uint256) {
        require(connectedWallets[wallet].isActive || hdWallets[wallet].isActive, "Wallet not connected");
        require(walletChainSupport[wallet][chainId], "Chain not supported");
        
        emit SwapExecuted(wallet, chainId, tokenIn, tokenOut, amountIn);
        return amountIn;
    }
    
    // View functions
    function isWalletConnected(address user) external view returns (bool) {
        return connectedWallets[user].isActive;
    }
    
    function isHDWalletActive(address wallet) external view returns (bool) {
        return hdWallets[wallet].isActive;
    }
    
    function isChainSupported(address wallet, uint256 chainId) external view returns (bool) {
        return walletChainSupport[wallet][chainId];
    }
    
    function getConnectedWalletCount() external view returns (uint256) {
        return connectedWalletCount;
    }
    
    function getHDWalletCount() external view returns (uint256) {
        return hdWalletCount;
    }
    
    function getWalletType(address user) external view returns (string memory) {
        return connectedWallets[user].walletType;
    }
}
