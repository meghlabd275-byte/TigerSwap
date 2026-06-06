// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/**
 * @title TigerChainRegistry
 * @notice Complete Multi-Chain Registry for ALL EVM and Non-EVM Blockchains
 * @dev Supports adding ANY blockchain dynamically
 * 
 * Features:
 * - Add/remove EVM chains dynamically
 * - Add/remove Non-EVM chains dynamically
 * - Chain-specific token support
 * - Chain-specific fee configuration
 * - Cross-chain bridge integration
 * - Full admin management
 */
import "./libraries/SafeMath.sol";

contract TigerChainRegistry {
    using SafeMath for uint256;

    // ============================================================================
    // Constants
    // ============================================================================

    uint256 constant MAX_CHAINS = 100;
    uint256 constant MAX_TOKENS_PER_CHAIN = 10000;

    // ============================================================================
    // Enums
    // ============================================================================

    enum ChainType {
        EVM,
        NonEVM_Solana,
        NonEVM_Aptos,
        NonEVM_Sui,
        NonEVM_Ton,
        NonEVM_Near,
        NonEVM_Cosmos,
        NonEVM_Polkadot,
        NonEVM_Cardano,
        NonEVM_Algorand,
        NonEVM_Flow,
        NonEVM_Avalanche,
        NonEVM_Near
    }

    enum ChainStatus {
        Active,
        Suspended,
        Deprecated
    }

    // ============================================================================
    // State Variables
    // ============================================================================

    // Admin management
    address public admin;
    address public pendingAdmin;
    address public feeManager;
    address public emergencyGuardian;

    // Chain management
    mapping(uint256 => ChainInfo) public chains;
    uint256[] public chainIds;
    uint256 public chainCount = 0;

    // Token registry per chain
    mapping(uint256 => mapping(address => TokenInfo)) public chainTokens;
    mapping(uint256 => address[]) public chainTokenAddresses;

    // Fee configuration per chain
    mapping(uint256 => ChainFeeConfig) public chainFeeConfigs;

    // Bridge support
    mapping(uint256 => bool) public bridgeSupported;
    mapping(uint256 => mapping(uint256 => bool)) public crossChainRoutes; // fromChain -> toChain -> supported

    // ============================================================================
    // Structs
    // ============================================================================

    struct ChainInfo {
        uint256 chainId;
        string name;
        string symbol;
        ChainType chainType;
        string chainIdHex;
        string rpcUrl;
        string explorerUrl;
        string logoUrl;
        address priceOracle;
        uint256 nativeTokenDecimals;
        uint256 avgGasPrice;
        ChainStatus status;
        uint256 minConfirmations;
        uint256 blockTime;
        bool isTestnet;
        string network;
    }

    struct TokenInfo {
        address tokenAddress;
        string symbol;
        string name;
        uint8 decimals;
        uint256 chainId;
        bool isNative;
        bool isWrapped;
        address wrappedToken;
        uint256 minTransfer;
        uint256 maxTransfer;
        bool isPaused;
        bool isWhitelisted;
    }

    struct ChainFeeConfig {
        uint256 swapFeeBps;
        uint256 withdrawFeeMin;
        uint256 withdrawFeeMax;
        uint256 depositFeeMin;
        uint256 depositFeeMax;
        uint256 crossChainFee;
        uint256 listingFee;
        bool isDynamic;
    }

    // ============================================================================
    // Events
    // ============================================================================

    event ChainAdded(uint256 indexed chainId, string name, string symbol, uint8 chainType);
    event ChainUpdated(uint256 indexed chainId, string name);
    event ChainSuspended(uint256 indexed chainId, string reason);
    event ChainActivated(uint256 indexed chainId);
    event TokenAdded(uint256 indexed chainId, address indexed token, string symbol);
    event TokenPaused(uint256 indexed chainId, address indexed token);
    event TokenActivated(uint256 indexed chainId, address indexed token);
    event BridgeEnabled(uint256 indexed fromChain, uint256 indexed toChain);
    event BridgeDisabled(uint256 indexed fromChain, uint256 indexed toChain);
    event FeeConfigUpdated(uint256 indexed chainId, uint256 swapFee, uint256 withdrawFee);
    event CrossChainTransfer(uint256 indexed fromChain, uint256 indexed toChain, address indexed user, uint256 amount);

    // ============================================================================
    // Constructor
    // ============================================================================

    constructor(address _admin, address _feeManager, address _emergencyGuardian) {
        admin = _admin;
        feeManager = _feeManager;
        emergencyGuardian = _emergencyGuardian;
        
        // Pre-register default EVM chains
        _addDefaultEVMChains();
        
        // Pre-register default Non-EVM chains
        _addDefaultNonEVMChains();
    }

    // ============================================================================
    // Default Chains
    // ============================================================================

    function _addDefaultEVMChains() internal {
        // Ethereum
        _registerChain(1, "Ethereum", "ETH", ChainType.EVM, "0x1", "https://eth-mainnet.alchemyapi.io", 
            "https://etherscan.io", "https://icons.coinbase.com/eth.png", 18, 2000000000, 1, 12);
        
        // BSC
        _registerChain(56, "BNB Smart Chain", "BNB", ChainType.EVM, "0x38", "https://bsc-dataseed.binance.org",
            "https://bscscan.com", "https://icons.coinbase.com/bnb.png", 18, 5000000000, 1, 3);
        
        // Polygon
        _registerChain(137, "Polygon", "MATIC", ChainType.EVM, "0x89", "https://polygon-rpc.com",
            "https://polygonscan.com", "https://cryptologos.cc/matic.png", 18, 50000000000, 1, 2);
        
        // Arbitrum One
        _registerChain(42161, "Arbitrum One", "ETH", ChainType.EVM, "0xa4b1", "https://arb1.arbitrum.io",
            "https://arbiscan.io", "https://cryptologos.cc/arbitrum.png", 18, 100000000, 1, 15);
        
        // Optimism
        _registerChain(10, "Optimism", "ETH", ChainType.EVM, "0xa", "https://mainnet.optimism.io",
            "https://optimistic.etherscan.io", "https://cryptologos.cc/optimism.png", 18, 1000000, 1, 15);
        
        // Base
        _registerChain(8453, "Base", "ETH", ChainType.EVM, "0x2105", "https://mainnet.base.org",
            "https://basescan.org", "https://cryptologos.cc/base.png", 18, 1000000, 1, 15);
        
        // Avalanche C-Chain
        _registerChain(43114, "Avalanche", "AVAX", ChainType.EVM, "0xa86a", "https://api.avax.network/ext/bc/C/rpc",
            "https://snowtrace.io", "https://cryptologos.cc/avalanche.png", 18, 25000000000, 1, 2);
        
        // Fantom
        _registerChain(250, "Fantom", "FTM", ChainType.EVM, "0xfa", "https://rpc.fantom.network",
            "https://ftmscan.com", "https://cryptologos.cc/fantom.png", 18, 500000000000, 1, 2);
        
        // Cronos
        _registerChain(25, "Cronos", "CRO", ChainType.EVM, "0x19", "https://evm.cronos.org",
            "https://cronoscan.com", "https://cryptologos.cc/cronos.png", 18, 5000000000000, 1, 5);
        
        // Celo
        _registerChain(42220, "Celo", "CELO", ChainType.EVM, "0xa4ec", "https://forno.celo.org",
            "https://explorer.celo.org", "https://cryptologos.cc/celo.png", 18, 1000000000, 1, 1);
        
        // Gnosis
        _registerChain(100, "Gnosis", "GNO", ChainType.EVM, "0x64", "https://rpc.gnosischain.com",
            "https://gnosisscan.io", "https://cryptologos.cc/gnosis.png", 18, 1000000000, 1, 1);
        
        // Moonbeam
        _registerChain(1284, "Moonbeam", "GLMR", ChainType.EVM, "0x504", "https://rpc.api.moonbeam.network",
            "https://moonscan.io", "https://cryptologos.cc/moonbeam.png", 18, 1000000000, 1, 1);
        
        // Aurora
        _registerChain(1313161554, "Aurora", "ETH", ChainType.EVM, "0x4e454152", "https://mainnet.aurora.dev",
            "https://explorer.aurora.dev", "https://cryptologos.cc/aurora.png", 18, 100000000000, 1, 1);
        
        // Harmony
        _registerChain(1666600000, "Harmony", "ONE", ChainType.EVM, "0xa", "https://api.harmony.one",
            "https://explorer.harmony.one", "https://cryptologos.cc/harmony.png", 18, 1000000000, 1, 2);
        
        // Klaytn
        _registerChain(8217, "Klaytn", "KLAY", ChainType.EVM, "0x2019", "https://klaytn-mainnet-rpc.allthatnode.com",
            "https://scope.klaytn.com", "https://cryptologos.cc/klaytn.png", 18, 250000000000, 1, 1);
        
        // zkSync Era
        _registerChain(324, "zkSync Era", "ETH", ChainType.EVM, "0x144", "https://mainnet.era.zksync.io",
            "https://explorer.zksync.io", "https://cryptologos.cc/zksync.png", 18, 100000000, 1, 1);
        
        // Linea
        _registerChain(59144, "Linea", "ETH", ChainType.EVM, "0xe708", "https://rpc.linea.build",
            "https://lineascan.build", "https://cryptologos.cc/linea.png", 18, 100000000, 1, 1);
        
        // Scroll
        _registerChain(534352, "Scroll", "ETH", ChainType.EVM, "0x82750", "https://scroll.io",
            "https://scrollscan.com", "https://cryptologos.cc/scroll.png", 18, 100000000, 1, 1);
        
        // Mantle
        _registerChain(5000, "Mantle", "MNT", ChainType.EVM, "0x1388", "https://rpc.mantle.xyz",
            "https://mantlescan.org", "https://cryptologos.cc/mantle.png", 18, 100000000, 1, 1);
        
        // Blast
        _registerChain(81457, "Blast", "ETH", ChainType.EVM, "0x13f31", "https://rpc.blast.io",
            "https://blastscan.io", "https://cryptologos.cc/blast.png", 18, 100000000, 1, 1);
    }

    function _addDefaultNonEVMChains() internal {
        // Solana
        _registerChain(101, "Solana", "SOL", ChainType.NonEVM_Solana, "", "https://api.mainnet-beta.solana.com",
            "https://solscan.io", "https://cryptologos.cc/solana.png", 9, 0, 1, 0.4);
        
        // Aptos
        _registerChain(101, "Aptos", "APT", ChainType.NonEVM_Aptos, "", "https://fullnode.mainnet.aptoslabs.com",
            "https://explorer.aptoslabs.com", "https://cryptologos.cc/aptos.png", 8, 0, 1, 1);
        
        // Sui
        _registerChain(101, "Sui", "SUI", ChainType.NonEVM_Sui, "", "https://fullnode.mainnet.sui.io",
            "https://suiscan.xyz", "https://cryptologos.cc/sui.png", 9, 0, 1, 1);
        
        // Toncoin
        _registerChain(0, "Toncoin", "TON", ChainType.NonEVM_Ton, "", "https://toncenter.com/api/v2",
            "https://tonscan.org", "https://cryptologos.cc/ton.png", 9, 0, 1, 5);
        
        // Near
        _registerChain(0, "Near", "NEAR", ChainType.NonEVM_Near, "", "https://rpc.mainnet.near.org",
            "https://explorer.near.org", "https://cryptologos.cc/near.png", 24, 0, 1, 1);
        
        // Cosmos Hub
        _registerChain(0, "Cosmos Hub", "ATOM", ChainType.NonEVM_Cosmos, "", "https://rpc.cosmoshub.io",
            "https://mintscan.io/cosmos-hub", "https://cryptologos.cc/cosmos.png", 6, 0, 1, 6);
        
        // Polkadot
        _registerChain(0, "Polkadot", "DOT", ChainType.NonEVM_Polkadot, "", "https://rpc.polkadot.io",
            "https://polkadot.subscan.io", "https://cryptologos.cc/polkadot.png", 10, 0, 1, 12);
        
        // Cardano
        _registerChain(0, "Cardano", "ADA", ChainType.NonEVM_Cardano, "", "https://cardano-mainnet.blockfrost.io",
            "https://cardanoscan.io", "https://cryptologos.cc/cardano.png", 6, 0, 1, 20);
        
        // Algorand
        _registerChain(0, "Algorand", "ALGO", ChainType.NonEVM_Algorand, "", "https://mainnet-algorand.api.purestake.io",
            "https://algoexplorer.io", "https://cryptologos.cc/algorand.png", 6, 0, 1, 3);
        
        // Flow
        _registerChain(0, "Flow", "FLOW", ChainType.NonEVM_Flow, "", "https://flow-mainnet.gateway.web3api.io",
            "https://flowscan.org", "https://cryptologos.cc/flow.png", 8, 0, 1, 2);
        
        // Avalanche (Non-EVM X-Chain)
        _registerChain(0, "Avalanche X-Chain", "AVAX", ChainType.NonEVM_Avalanche, "", "https://api.avax.network",
            "https://snowtrace.io", "https://cryptologos.cc/avalanche.png", 18, 0, 1, 2);
        
        // Near (Testnet)
        _registerChain(0, "Near Testnet", "NEAR", ChainType.NonEVM_Near, "", "https://rpc.testnet.near.org",
            "https://explorer.testnet.near.org", "https://cryptologos.cc/near.png", 24, 0, 1, 1);
    }

    // ============================================================================
    // Chain Registration
    // ============================================================================

    function _registerChain(
        uint256 chainId,
        string memory name,
        string memory symbol,
        ChainType chainType,
        string memory chainIdHex,
        string memory rpcUrl,
        string memory explorerUrl,
        string memory logoUrl,
        uint256 nativeDecimals,
        uint256 avgGasPrice,
        uint256 minConfirmations,
        uint256 blockTime
    ) internal {
        ChainInfo memory chain = ChainInfo({
            chainId: chainId,
            name: name,
            symbol: symbol,
            chainType: chainType,
            chainIdHex: chainIdHex,
            rpcUrl: rpcUrl,
            explorerUrl: explorerUrl,
            logoUrl: logoUrl,
            priceOracle: address(0),
            nativeTokenDecimals: nativeDecimals,
            avgGasPrice: avgGasPrice,
            status: ChainStatus.Active,
            minConfirmations: minConfirmations,
            blockTime: blockTime,
            isTestnet: false,
            network: "mainnet"
        });
        
        chains[chainId] = chain;
        chainIds.push(chainId);
        chainCount++;
        
        emit ChainAdded(chainId, name, symbol, uint8(chainType));
    }

    /**
     * @notice Admin adds new EVM chain
     */
    function addEVMChain(
        uint256 chainId,
        string memory name,
        string memory symbol,
        string memory chainIdHex,
        string memory rpcUrl,
        string memory explorerUrl,
        string memory logoUrl,
        uint256 nativeDecimals,
        uint256 avgGasPrice
    ) external {
        require(msg.sender == admin, "Not admin");
        
        ChainInfo memory chain = ChainInfo({
            chainId: chainId,
            name: name,
            symbol: symbol,
            chainType: ChainType.EVM,
            chainIdHex: chainIdHex,
            rpcUrl: rpcUrl,
            explorerUrl: explorerUrl,
            logoUrl: logoUrl,
            priceOracle: address(0),
            nativeTokenDecimals: nativeDecimals,
            avgGasPrice: avgGasPrice,
            status: ChainStatus.Active,
            minConfirmations: 1,
            blockTime: 12,
            isTestnet: false,
            network: "mainnet"
        });
        
        chains[chainId] = chain;
        chainIds.push(chainId);
        chainCount++;
        
        emit ChainAdded(chainId, name, symbol, uint8(ChainType.EVM));
    }

    /**
     * @notice Admin adds new Non-EVM chain
     */
    function addNonEVMChain(
        uint256 chainId,
        string memory name,
        string memory symbol,
        uint8 chainType, // 1=Solana, 2=Aptos, 3=Sui, 4=Ton, 5=Near, etc.
        string memory rpcUrl,
        string memory explorerUrl,
        string memory logoUrl,
        uint256 nativeDecimals
    ) external {
        require(msg.sender == admin, "Not admin");
        
        ChainInfo memory chain = ChainInfo({
            chainId: chainId,
            name: name,
            symbol: symbol,
            chainType: ChainType(chainType),
            chainIdHex: "",
            rpcUrl: rpcUrl,
            explorerUrl: explorerUrl,
            logoUrl: logoUrl,
            priceOracle: address(0),
            nativeTokenDecimals: nativeDecimals,
            avgGasPrice: 0,
            status: ChainStatus.Active,
            minConfirmations: 1,
            blockTime: 2,
            isTestnet: false,
            network: "mainnet"
        });
        
        chains[chainId] = chain;
        chainIds.push(chainId);
        chainCount++;
        
        emit ChainAdded(chainId, name, symbol, chainType);
    }

    /**
     * @notice Update chain info
     */
    function updateChain(
        uint256 chainId,
        string memory rpcUrl,
        string memory explorerUrl,
        uint256 avgGasPrice,
        address priceOracle
    ) external {
        require(msg.sender == admin, "Not admin");
        require(chains[chainId].chainId == chainId, "Chain not found");
        
        ChainInfo storage chain = chains[chainId];
        chain.rpcUrl = rpcUrl;
        chain.explorerUrl = explorerUrl;
        chain.avgGasPrice = avgGasPrice;
        chain.priceOracle = priceOracle;
        
        emit ChainUpdated(chainId, chain.name);
    }

    /**
     * @notice Suspend chain
     */
    function suspendChain(uint256 chainId, string memory reason) external {
        require(msg.sender == admin || msg.sender == emergencyGuardian, "Not authorized");
        
        ChainInfo storage chain = chains[chainId];
        chain.status = ChainStatus.Suspended;
        
        emit ChainSuspended(chainId, reason);
    }

    /**
     * @notice Activate suspended chain
     */
    function activateChain(uint256 chainId) external {
        require(msg.sender == admin, "Not admin");
        
        ChainInfo storage chain = chains[chainId];
        chain.status = ChainStatus.Active;
        
        emit ChainActivated(chainId);
    }

    // ============================================================================
    // Token Management
    // ============================================================================

    /**
     * @notice Add token to chain
     */
    function addToken(
        uint256 chainId,
        address tokenAddress,
        string memory symbol,
        string memory name,
        uint8 decimals,
        bool isNative,
        bool isWrapped,
        address wrappedToken
    ) external {
        require(msg.sender == admin, "Not admin");
        require(chains[chainId].chainId == chainId, "Chain not found");
        
        TokenInfo memory token = TokenInfo({
            tokenAddress: tokenAddress,
            symbol: symbol,
            name: name,
            decimals: decimals,
            chainId: chainId,
            isNative: isNative,
            isWrapped: isWrapped,
            wrappedToken: wrappedToken,
            minTransfer: 0,
            maxTransfer: type(uint256).max,
            isPaused: false,
            isWhitelisted: true
        });
        
        chainTokens[chainId][tokenAddress] = token;
        chainTokenAddresses[chainId].push(tokenAddress);
        
        emit TokenAdded(chainId, tokenAddress, symbol);
    }

    /**
     * @notice Pause token
     */
    function pauseToken(uint256 chainId, address token) external {
        require(msg.sender == admin, "Not admin");
        
        TokenInfo storage t = chainTokens[chainId][token];
        require(t.tokenAddress == token, "Token not found");
        
        t.isPaused = true;
        
        emit TokenPaused(chainId, token);
    }

    /**
     * @notice Activate token
     */
    function activateToken(uint256 chainId, address token) external {
        require(msg.sender == admin, "Not admin");
        
        TokenInfo storage t = chainTokens[chainId][token];
        require(t.tokenAddress == token, "Token not found");
        
        t.isPaused = false;
        
        emit TokenActivated(chainId, token);
    }

    // ============================================================================
    // Fee Configuration
    // ============================================================================

    /**
     * @notice Set chain fee configuration
     */
    function setChainFeeConfig(
        uint256 chainId,
        uint256 swapFeeBps,
        uint256 withdrawFeeMin,
        uint256 withdrawFeeMax,
        uint256 depositFeeMin,
        uint256 depositFeeMax,
        uint256 crossChainFee,
        uint256 listingFee,
        bool isDynamic
    ) external {
        require(msg.sender == feeManager, "Not fee manager");
        
        ChainFeeConfig memory config = ChainFeeConfig({
            swapFeeBps: swapFeeBps,
            withdrawFeeMin: withdrawFeeMin,
            withdrawFeeMax: withdrawFeeMax,
            depositFeeMin: depositFeeMin,
            depositFeeMax: depositFeeMax,
            crossChainFee: crossChainFee,
            listingFee: listingFee,
            isDynamic: isDynamic
        });
        
        chainFeeConfigs[chainId] = config;
        
        emit FeeConfigUpdated(chainId, swapFeeBps, withdrawFeeMin);
    }

    // ============================================================================
    // Bridge Management
    // ============================================================================

    /**
     * @notice Enable cross-chain bridge
     */
    function enableBridge(uint256 fromChain, uint256 toChain) external {
        require(msg.sender == admin, "Not admin");
        
        crossChainRoutes[fromChain][toChain] = true;
        
        emit BridgeEnabled(fromChain, toChain);
    }

    /**
     * @notice Disable cross-chain bridge
     */
    function disableBridge(uint256 fromChain, uint256 toChain) external {
        require(msg.sender == admin, "Not admin");
        
        crossChainRoutes[fromChain][toChain] = false;
        
        emit BridgeDisabled(fromChain, toChain);
    }

    /**
     * @notice Check if bridge is supported
     */
    function isBridgeSupported(uint256 fromChain, uint256 toChain) external view returns (bool) {
        return crossChainRoutes[fromChain][toChain];
    }

    // ============================================================================
    // Admin Management
    // ============================================================================

    function setAdmin(address newAdmin) external {
        require(msg.sender == admin, "Not admin");
        pendingAdmin = newAdmin;
    }

    function acceptAdmin() external {
        require(msg.sender == pendingAdmin, "Not pending admin");
        admin = pendingAdmin;
        pendingAdmin = address(0);
    }

    function setFeeManager(address newFeeManager) external {
        require(msg.sender == admin, "Not admin");
        feeManager = newFeeManager;
    }

    // ============================================================================
    // View Functions
    // ============================================================================

    function getChain(uint256 chainId) external view returns (
        string memory name,
        string memory symbol,
        uint8 chainType,
        string memory rpcUrl,
        string memory explorerUrl,
        uint8 status,
        bool isTestnet
    ) {
        ChainInfo memory chain = chains[chainId];
        return (
            chain.name,
            chain.symbol,
            uint8(chain.chainType),
            chain.rpcUrl,
            chain.explorerUrl,
            uint8(chain.status),
            chain.isTestnet
        );
    }

    function getToken(uint256 chainId, address token) external view returns (
        string memory symbol,
        string memory name,
        uint8 decimals,
        bool isPaused,
        bool isWhitelisted
    ) {
        TokenInfo memory t = chainTokens[chainId][token];
        return (
            t.symbol,
            t.name,
            t.decimals,
            t.isPaused,
            t.isWhitelisted
        );
    }

    function getChainTokenCount(uint256 chainId) external view returns (uint256) {
        return chainTokenAddresses[chainId].length;
    }

    function getChainIds() external view returns (uint256[] memory) {
        return chainIds;
    }

    function getFeeConfig(uint256 chainId) external view returns (
        uint256 swapFee,
        uint256 withdrawFeeMin,
        uint256 withdrawFeeMax,
        bool isDynamic
    ) {
        ChainFeeConfig memory config = chainFeeConfigs[chainId];
        return (
            config.swapFeeBps,
            config.withdrawFeeMin,
            config.withdrawFeeMax,
            config.isDynamic
        );
    }

    function isChainActive(uint256 chainId) external view returns (bool) {
        return chains[chainId].status == ChainStatus.Active;
    }

    function getChainCount() external view returns (uint256) {
        return chainCount;
    }
}