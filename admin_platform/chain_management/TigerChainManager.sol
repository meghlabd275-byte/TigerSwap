// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "../libraries/SafeMath.sol";

/**
 * @title TigerChainManager
 * @notice Multi-Chain Management for EVM and Non-EVM Blockchains
 * @dev Complete chain management with deployment, monitoring, and governance
 * 
 * Features:
 * - EVM chain deployment (Ethereum, BSC, Polygon, Arbitrum, Optimism, Base, Avalanche, etc.)
 * - Non-EVM chain support (Solana, Aptos, Sui, TON, Cosmos)
 * - Chain configuration and monitoring
 * - Cross-chain communication
 * - Bridge management
 * - Node management
 * - Token deployment across chains
 * - Gas fee management
 * - Chain upgradeability
 */
contract TigerChainManager {
    using SafeMath for uint256;

    // ============================================================================
    // Constants
    // ============================================================================
    
    uint256 constant MAX_CHAINS = 100;
    uint256 constant MAX_VALIDATORS = 50;
    uint256 constant MIN_STAKE = 1000 ether;
    
    // Chain Types
    uint8 constant CHAIN_TYPE_EVM = 1;
    uint8 constant CHAIN_TYPE_SOLANA = 2;
    uint8 constant CHAIN_TYPE_APTOS = 3;
    uint8 constant CHAIN_TYPE_SUI = 4;
    uint8 constant CHAIN_TYPE_TON = 5;
    uint8 constant CHAIN_TYPE_COSMOS = 6;
    
    // Chain Status
    uint8 constant STATUS_INACTIVE = 0;
    uint8 constant STATUS_ACTIVE = 1;
    uint8 constant STATUS_PAUSED = 2;
    uint8 constant STATUS_UPGRADING = 3;
    uint8 constant STATUS_DEPRECATED = 4;
    
    // ============================================================================
    // State Variables
    // ============================================================================
    
    // Governance
    address public governance;
    address public pendingGovernance;
    address public admin;
    
    // Chain Registry
    mapping(bytes32 => Chain) public chains;
    bytes32[] public chainIds;
    mapping(uint8 => mapping(bytes32 => bool)) public chainsByType;
    
    // Chain Configuration
    mapping(bytes32 => ChainConfig) public chainConfigs;
    mapping(bytes32 => ChainState) public chainStates;
    
    // Validators
    mapping(address => Validator) public validators;
    address[] public validatorList;
    uint256 public validatorCount;
    mapping(bytes32 => mapping(address => bool)) public chainValidators;
    
    // Bridge Management
    mapping(bytes32 => Bridge) public bridges;
    bytes32[] public bridgeIds;
    
    // Token Deployment
    mapping(bytes32 => mapping(address => DeployedToken)) public deployedTokens;
    mapping(address => bytes32[]) public tokenChains;
    
    // Gas Fees
    mapping(bytes32 => GasConfig) public gasConfigs;
    
    // Monitoring
    mapping(bytes32 => ChainMetrics) public chainMetrics;
    uint256 public lastUpdateTime;
    
    // Events
    event ChainAdded(bytes32 indexed chainId, string name, uint8 chainType);
    event ChainUpdated(bytes32 indexed chainId, uint8 status);
    event ChainRemoved(bytes32 indexed chainId);
    event ValidatorAdded(address indexed validator, bytes32 indexed chainId);
    event ValidatorRemoved(address indexed validator, bytes32 indexed chainId);
    event BridgeAdded(bytes32 indexed bridgeId, string name, bytes32 sourceChain, bytes32 destChain);
    event TokenDeployed(bytes32 indexed chainId, address indexed token, address indexed deployer);
    event ChainMetricsUpdated(bytes32 indexed chainId, uint256 blockNumber, uint256 gasUsed);
    event GovernanceTransferred(address indexed oldGov, address indexed newGov);
    
    // ============== Structs ==============
    
    struct Chain {
        bytes32 id;
        string name;
        uint8 chainType;
        string rpcUrl;
        string explorerUrl;
        uint256 chainId;
        bytes32 nativeCurrency;
        uint256 decimals;
        uint8 status;
        uint256 minGasPrice;
        uint256 maxGasPrice;
        uint256 targetGasLimit;
        bool isTestnet;
        uint256 deployedAt;
    }
    
    struct ChainConfig {
        uint256 minConfirmations;
        uint256 maxConfirmations;
        uint256 blockTime;
        uint256 finalityTime;
        uint256 maxTxSize;
        uint256 gasPerDataByte;
        bool supportsEIP1559;
        bool supportsBatch;
        bool supportsDebug;
    }
    
    struct ChainState {
        uint256 blockNumber;
        uint256 timestamp;
        uint256 gasUsed;
        uint256 gasLimit;
        uint256 txCount;
        uint256 activeValidators;
        bool syncing;
    }
    
    struct Validator {
        address validator;
        string endpoint;
        uint256 stake;
        uint256 delegatedStake;
        uint256 reward;
        bool active;
        uint256 lastActive;
        uint256 performanceScore;
    }
    
    struct Bridge {
        bytes32 id;
        string name;
        bytes32 sourceChain;
        bytes32 destChain;
        address router;
        uint256 minAmount;
        uint256 maxAmount;
        uint256 feeBps;
        bool active;
    }
    
    struct DeployedToken {
        address token;
        bytes32 chainId;
        string name;
        string symbol;
        uint256 decimals;
        uint256 totalSupply;
        address deployer;
        uint256 deployedAt;
    }
    
    struct GasConfig {
        uint256 minGasPrice;
        uint256 maxGasPrice;
        uint256 targetGasPrice;
        uint256 gasTipCap;
        uint256 feeHistory;
    }
    
    struct ChainMetrics {
        bytes32 chainId;
        uint256 tps;
        uint256 avgGasUsed;
        uint256 avgBlockTime;
        uint256 uptime;
        uint256 totalTx;
        uint256 totalVolume;
        uint256 activeUsers;
    }
    
    // ============== Modifier ==============
    
    modifier onlyGovernance() {
        require(msg.sender == governance, "ONLY_GOVERNANCE");
        _;
    }
    
    modifier onlyAdmin() {
        require(msg.sender == admin || msg.sender == governance, "ONLY_ADMIN");
        _;
    }
    
    // ============== Constructor ==============
    
    constructor() {
        governance = msg.sender;
        admin = msg.sender;
    }
    
    // ============================================================================
    // Chain Management
    // ============================================================================
    
    /**
     * @notice Add new EVM chain
     */
    function addEVMChain(
        bytes32 _chainId,
        string calldata _name,
        uint256 _chainIdNum,
        string calldata _rpcUrl,
        string calldata _explorerUrl,
        bytes32 _nativeCurrency,
        uint256 _decimals,
        bool _isTestnet
    ) external onlyAdmin {
        require(chains[_chainId].deployedAt == 0, "CHAIN_EXISTS");
        
        chains[_chainId] = Chain({
            id: _chainId,
            name: _name,
            chainType: CHAIN_TYPE_EVM,
            rpcUrl: _rpcUrl,
            explorerUrl: _explorerUrl,
            chainId: _chainIdNum,
            nativeCurrency: _nativeCurrency,
            decimals: _decimals,
            status: STATUS_ACTIVE,
            minGasPrice: 1e9,
            maxGasPrice: 100e9,
            targetGasLimit: 30000000,
            isTestnet: _isTestnet,
            deployedAt: block.timestamp
        });
        
        // Default configuration
        chainConfigs[_chainId] = ChainConfig({
            minConfirmations: _isTestnet ? 1 : 12,
            maxConfirmations: _isTestnet ? 3 : 64,
            blockTime: 12,
            finalityTime: 96 * 12, // ~19 min for Ethereum
            maxTxSize: 128000,
            gasPerDataByte: 16,
            supportsEIP1559: true,
            supportsBatch: true,
            supportsDebug: true
        });
        
        chainIds.push(_chainId);
        chainsByType[CHAIN_TYPE_EVM][_chainId] = true;
        
        emit ChainAdded(_chainId, _name, CHAIN_TYPE_EVM);
    }
    
    /**
     * @notice Add Non-EVM chain (Solana, Aptos, Sui, TON, Cosmos)
     */
    function addNonEVMChain(
        bytes32 _chainId,
        string calldata _name,
        uint8 _chainType,
        string calldata _rpcUrl,
        string calldata _explorerUrl,
        bytes32 _nativeCurrency,
        uint256 _decimals,
        bool _isTestnet
    ) external onlyAdmin {
        require(chains[_chainId].deployedAt == 0, "CHAIN_EXISTS");
        require(_chainType >= CHAIN_TYPE_SOLANA && _chainType <= CHAIN_TYPE_COSMOS, "INVALID_TYPE");
        
        chains[_chainId] = Chain({
            id: _chainId,
            name: _name,
            chainType: _chainType,
            rpcUrl: _rpcUrl,
            explorerUrl: _explorerUrl,
            chainId: 0,
            nativeCurrency: _nativeCurrency,
            decimals: _decimals,
            status: STATUS_ACTIVE,
            minGasPrice: 0,
            maxGasPrice: 0,
            targetGasLimit: 0,
            isTestnet: _isTestnet,
            deployedAt: block.timestamp
        });
        
        chainIds.push(_chainId);
        chainsByType[_chainType][_chainId] = true;
        
        emit ChainAdded(_chainId, _name, _chainType);
    }
    
    /**
     * @notice Update chain status
     */
    function updateChainStatus(bytes32 _chainId, uint8 _status) external onlyAdmin {
        require(chains[_chainId].deployedAt > 0, "CHAIN_NOT_EXISTS");
        
        chains[_chainId].status = _status;
        
        emit ChainUpdated(_chainId, _status);
    }
    
    /**
     * @notice Remove chain
     */
    function removeChain(bytes32 _chainId) external onlyGovernance {
        require(chains[_chainId].deployedAt > 0, "CHAIN_NOT_EXISTS");
        
        chains[_chainId].status = STATUS_DEPRECATED;
        
        emit ChainRemoved(_chainId);
    }
    
    /**
     * @notice Update chain configuration
     */
    function updateChainConfig(
        bytes32 _chainId,
        uint256 _minConfirmations,
        uint256 _maxConfirmations,
        uint256 _blockTime,
        uint256 _finalityTime
    ) external onlyAdmin {
        ChainConfig storage config = chainConfigs[_chainId];
        config.minConfirmations = _minConfirmations;
        config.maxConfirmations = _maxConfirmations;
        config.blockTime = _blockTime;
        config.finalityTime = _finalityTime;
    }
    
    // ============================================================================
    // Validator Management
    // ============================================================================
    
    /**
     * @notice Add validator for chain
     */
    function addValidator(address _validator, bytes32 _chainId, string calldata _endpoint) external onlyAdmin {
        require(_validator != address(0), "INVALID_VALIDATOR");
        require(chains[_chainId].deployedAt > 0, "CHAIN_NOT_EXISTS");
        
        if (validators[_validator].validator == address(0)) {
            validators[_validator] = Validator({
                validator: _validator,
                endpoint: _endpoint,
                stake: 0,
                delegatedStake: 0,
                reward: 0,
                active: true,
                lastActive: block.timestamp,
                performanceScore: 10000
            });
            
            validatorList.push(_validator);
            validatorCount++;
        }
        
        chainValidators[_chainId][_validator] = true;
        
        emit ValidatorAdded(_validator, _chainId);
    }
    
    /**
     * @notice Remove validator from chain
     */
    function removeValidator(address _validator, bytes32 _chainId) external onlyAdmin {
        require(chainValidators[_chainId][_validator], "NOT_VALIDATOR");
        
        chainValidators[_chainId][_validator] = false;
        
        emit ValidatorRemoved(_validator, _chainId);
    }
    
    /**
     * @notice Update validator stake
     */
    function updateValidatorStake(address _validator, uint256 _stake) external {
        require(validators[_validator].validator == address(0), "NOT_VALIDATOR");
        
        validators[_validator].stake = _stake;
    }
    
    /**
     * @notice Activate validator
     */
    function activateValidator(address _validator) external onlyAdmin {
        require(validators[_validator].validator != address(0), "NOT_VALIDATOR");
        
        validators[_validator].active = true;
        validators[_validator].lastActive = block.timestamp;
    }
    
    /**
     * @notice Deactivate validator
     */
    function deactivateValidator(address _validator) external onlyAdmin {
        validators[_validator].active = false;
    }
    
    // ============================================================================
    // Bridge Management
    // ============================================================================
    
    /**
     * @notice Add bridge
     */
    function addBridge(
        bytes32 _bridgeId,
        string calldata _name,
        bytes32 _sourceChain,
        bytes32 _destChain,
        address _router,
        uint256 _minAmount,
        uint256 _maxAmount,
        uint256 _feeBps
    ) external onlyAdmin {
        require(chains[_sourceChain].deployedAt > 0, "SOURCE_NOT_EXISTS");
        require(chains[_destChain].deployedAt > 0, "DEST_NOT_EXISTS");
        
        bridges[_bridgeId] = Bridge({
            id: _bridgeId,
            name: _name,
            sourceChain: _sourceChain,
            destChain: _destChain,
            router: _router,
            minAmount: _minAmount,
            maxAmount: _maxAmount,
            feeBps: _feeBps,
            active: true
        });
        
        bridgeIds.push(_bridgeId);
        
        emit BridgeAdded(_bridgeId, _name, _sourceChain, _destChain);
    }
    
    /**
     * @notice Update bridge
     */
    function updateBridge(
        bytes32 _bridgeId,
        uint256 _minAmount,
        uint256 _maxAmount,
        uint256 _feeBps,
        bool _active
    ) external onlyAdmin {
        Bridge storage bridge = bridges[_bridgeId];
        bridge.minAmount = _minAmount;
        bridge.maxAmount = _maxAmount;
        bridge.feeBps = _feeBps;
        bridge.active = _active;
    }
    
    // ============================================================================
    // Token Deployment
    // ============================================================================
    
    /**
     * @notice Register deployed token
     */
    function registerDeployedToken(
        bytes32 _chainId,
        address _token,
        string calldata _name,
        string calldata _symbol,
        uint256 _decimals,
        uint256 _totalSupply
    ) external onlyAdmin {
        require(chains[_chainId].deployedAt > 0, "CHAIN_NOT_EXISTS");
        
        deployedTokens[_chainId][_token] = DeployedToken({
            token: _token,
            chainId: _chainId,
            name: _name,
            symbol: _symbol,
            decimals: _decimals,
            totalSupply: _totalSupply,
            deployer: msg.sender,
            deployedAt: block.timestamp
        });
        
        tokenChains[_token].push(_chainId);
        
        emit TokenDeployed(_chainId, _token, msg.sender);
    }
    
    /**
     * @notice Get token on chain
     */
    function getTokenChain(address _token, bytes32 _chainId) external view returns (
        address token,
        string memory name,
        string memory symbol,
        uint256 decimals,
        uint256 totalSupply
    ) {
        DeployedToken storage dt = deployedTokens[_chainId][_token];
        return (dt.token, dt.name, dt.symbol, dt.decimals, dt.totalSupply);
    }
    
    // ============================================================================
    // Gas Management
    // ============================================================================
    
    /**
     * @notice Set gas configuration for chain
     */
    function setGasConfig(
        bytes32 _chainId,
        uint256 _minGasPrice,
        uint256 _maxGasPrice,
        uint256 _targetGasPrice,
        uint256 _gasTipCap
    ) external onlyAdmin {
        gasConfigs[_chainId] = GasConfig({
            minGasPrice: _minGasPrice,
            maxGasPrice: _maxGasPrice,
            targetGasPrice: _targetGasPrice,
            gasTipCap: _gasTipCap,
            feeHistory: 0
        });
    }
    
    /**
     * @notice Get recommended gas price
     */
    function getRecommendedGas(bytes32 _chainId) external view returns (uint256) {
        GasConfig storage config = gasConfigs[_chainId];
        
        if (config.targetGasPrice > 0) {
            return config.targetGasPrice;
        }
        
        return config.minGasPrice;
    }
    
    // ============================================================================
    // Chain Metrics
    // ============================================================================
    
    /**
     * @notice Update chain metrics
     */
    function updateChainMetrics(
        bytes32 _chainId,
        uint256 _tps,
        uint256 _avgGasUsed,
        uint256 _avgBlockTime,
        uint256 _uptime,
        uint256 _totalTx,
        uint256 _totalVolume,
        uint256 _activeUsers
    ) external onlyAdmin {
        chainMetrics[_chainId] = ChainMetrics({
            chainId: _chainId,
            tps: _tps,
            avgGasUsed: _avgGasUsed,
            avgBlockTime: _avgBlockTime,
            uptime: _uptime,
            totalTx: _totalTx,
            totalVolume: _totalVolume,
            activeUsers: _activeUsers
        });
        
        lastUpdateTime = block.timestamp;
        
        emit ChainMetricsUpdated(_chainId, chainStates[_chainId].blockNumber, _avgGasUsed);
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
        
        address oldGov = governance;
        governance = msg.sender;
        admin = msg.sender;
        
        emit GovernanceTransferred(oldGov, msg.sender);
        
        delete pendingGovernance;
    }
    
    /**
     * @notice Set admin
     */
    function setAdmin(address _newAdmin) external onlyGovernance {
        admin = _newAdmin;
    }
    
    // ============================================================================
    // View Functions
    // ============================================================================
    
    /**
     * @notice Get chain details
     */
    function getChain(bytes32 _chainId) external view returns (
        string memory name,
        uint8 chainType,
        uint256 chainIdNum,
        uint8 status,
        bool isTestnet,
        uint256 deployedAt
    ) {
        Chain storage chain = chains[_chainId];
        return (
            chain.name,
            chain.chainType,
            chain.chainId,
            chain.status,
            chain.isTestnet,
            chain.deployedAt
        );
    }
    
    /**
     * @notice Get all chains
     */
    function getAllChains() external view returns (bytes32[] memory) {
        return chainIds;
    }
    
    /**
     * @notice Get chains by type
     */
    function getChainsByType(uint8 _chainType) external view returns (bytes32[] memory result) {
        uint256 count = 0;
        for (uint256 i = 0; i < chainIds.length; i++) {
            if (chainsByType[_chainType][chainIds[i]]) {
                count++;
            }
        }
        
        result = new bytes32[](count);
        count = 0;
        for (uint256 i = 0; i < chainIds.length; i++) {
            if (chainsByType[_chainType][chainIds[i]]) {
                result[count++] = chainIds[i];
            }
        }
    }
    
    /**
     * @notice Get chain config
     */
    function getChainConfig(bytes32 _chainId) external view returns (
        uint256 minConfirmations,
        uint256 maxConfirmations,
        uint256 blockTime,
        uint256 finalityTime,
        bool supportsEIP1559,
        bool supportsBatch
    ) {
        ChainConfig storage config = chainConfigs[_chainId];
        return (
            config.minConfirmations,
            config.maxConfirmations,
            config.blockTime,
            config.finalityTime,
            config.supportsEIP1559,
            config.supportsBatch
        );
    }
    
    /**
     * @notice Get chain metrics
     */
    function getChainMetrics(bytes32 _chainId) external view returns (
        uint256 tps,
        uint256 avgGasUsed,
        uint256 avgBlockTime,
        uint256 uptime,
        uint256 totalTx,
        uint256 totalVolume,
        uint256 activeUsers
    ) {
        ChainMetrics storage metrics = chainMetrics[_chainId];
        return (
            metrics.tps,
            metrics.avgGasUsed,
            metrics.avgBlockTime,
            metrics.uptime,
            metrics.totalTx,
            metrics.totalVolume,
            metrics.activeUsers
        );
    }
    
    /**
     * @notice Get validators for chain
     */
    function getChainValidators(bytes32 _chainId) external view returns (address[] memory) {
        uint256 count = 0;
        for (uint256 i = 0; i < validatorList.length; i++) {
            if (chainValidators[_chainId][validatorList[i]]) {
                count++;
            }
        }
        
        address[] memory result = new address[](count);
        count = 0;
        for (uint256 i = 0; i < validatorList.length; i++) {
            if (chainValidators[_chainId][validatorList[i]]) {
                result[count++] = validatorList[i];
            }
        }
    }
    
    /**
     * @notice Get all bridges
     */
    function getAllBridges() external view returns (bytes32[] memory) {
        return bridgeIds;
    }
    
    /**
     * @notice Get bridge details
     */
    function getBridge(bytes32 _bridgeId) external view returns (
        string memory name,
        bytes32 sourceChain,
        bytes32 destChain,
        address router,
        uint256 feeBps,
        bool active
    ) {
        Bridge storage bridge = bridges[_bridgeId];
        return (
            bridge.name,
            bridge.sourceChain,
            bridge.destChain,
            bridge.router,
            bridge.feeBps,
            bridge.active
        );
    }
    
    /**
     * @notice Get validator info
     */
    function getValidatorInfo(address _validator) external view returns (
        string memory endpoint,
        uint256 stake,
        uint256 delegatedStake,
        bool active,
        uint256 performanceScore
    ) {
        Validator storage v = validators[_validator];
        return (
            v.endpoint,
            v.stake,
            v.delegatedStake,
            v.active,
            v.performanceScore
        );
    }
    
    /**
     * @notice Get chain count
     */
    function getChainCount() external view returns (uint256) {
        return chainIds.length;
    }
    
    /**
     * @notice Get validator count
     */
    function getValidatorCount() external view returns (uint256) {
        return validatorList.length;
    }
    
    /**
     * @notice Check if chain is active
     */
    function isChainActive(bytes32 _chainId) external view returns (bool) {
        return chains[_chainId].status == STATUS_ACTIVE;
    }
    
    /**
     * @notice Check if chain is EVM
     */
    function isEVMChain(bytes32 _chainId) external view returns (bool) {
        return chains[_chainId].chainType == CHAIN_TYPE_EVM;
    }
    
    /**
     * @notice Get supported chain types
     */
    function getSupportedChainTypes() external pure returns (uint8[] memory) {
        uint8[] memory types = new uint8[](6);
        types[0] = CHAIN_TYPE_EVM;
        types[1] = CHAIN_TYPE_SOLANA;
        types[2] = CHAIN_TYPE_APTOS;
        types[3] = CHAIN_TYPE_SUI;
        types[4] = CHAIN_TYPE_TON;
        types[5] = CHAIN_TYPE_COSMOS;
        return types;
    }
}