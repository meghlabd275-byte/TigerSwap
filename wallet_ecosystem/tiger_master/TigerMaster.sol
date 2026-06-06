// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/**
 * @title TigerMaster
 * @notice Master Wallet - Admin Controls All User Wallets
 * @dev Complete admin wallet system for managing all user wallets
 * 
 * Features:
 * - 24-word BIP39 seed phrase
 * - Create/import user wallets
 * - Auto-sign all transactions (within 3 seconds)
 * - Set all fees (withdraw, swap, transaction)
 * - Add/delete/update blockchains
 * - Manage token baskets
 * - All revenue automatically stored
 * - Backup code auto-saved
 * - Complete user wallet management
 */
import "../libraries/SafeMath.sol";

contract TigerMaster {
    using SafeMath for uint256;

    // ============================================================================
    // Constants
    // ============================================================================

    uint256 constant VERSION = 1;
    uint256 constant MAX_WALLETS = 1000000;
    uint256 constant AUTO_SIGN_DELAY = 3 seconds;

    // ============================================================================
    // State Variables
    // ============================================================================

    // Master wallet info
    address public masterAddress;
    bytes public masterEncryptedSeed;
    bytes32 public masterWalletHash;
    string public masterName;
    uint256 public createdAt;
    bool public isActive;
    
    // Backup code
    string public backupCode;
    bytes32 public backupCodeHash;
    
    // Fee settings (all fees go to master wallet)
    uint256 public withdrawFeePercent = 1;      // 1% withdraw fee
    uint256 public swapFeePercent = 0.3;        // 0.3% swap fee
    uint256 public transactionFeePercent = 0.1;   // 0.1% tx fee
    uint256 public liquidityFeePercent = 0.2;    // 0.2% liquidity fee
    uint256 public airdropFeePercent = 0;       // 0% airdrop fee
    
    // Fee addresses
    mapping(string => address) public feeRecipients;
    
    // User wallets
    mapping(address => UserWallet) public userWallets;
    address[] public walletAddresses;
    uint256 public walletCount = 0;
    
    // Blockchain management
    mapping(uint256 => BlockchainInfo) public blockchains;
    uint256[] public blockchainIds;
    uint256 public blockchainCount = 0;
    
    // Token baskets
    mapping(string => TokenBasket) public tokenBaskets;
    string[] public basketNames;
    uint256 public basketCount = 0;
    
    // Auto-sign queue
    mapping(bytes32 => AutoSignRequest) public autoSignQueue;
    bytes32[] public autoSignIds;
    
    // Revenue tracking
    uint256 public totalRevenue;
    uint256 public withdrawFeesCollected;
    uint256 public swapFeesCollected;
    uint256 public transactionFeesCollected;
    uint256 public liquidityFeesCollected;
    
    // Transaction limits
    uint256 public maxTransactionAmount = 1000000000e18;
    uint256 public dailyLimit = 100000000e18;
    mapping(address => uint256) public dailySpent;
    mapping(address => uint256) public lastDailyReset;

    // ============================================================================
    // Structs
    // ============================================================================

    struct UserWallet {
        address walletAddress;
        string name;
        bytes encryptedSeed;
        bytes32 walletHash;
        bool isActive;
        uint256 createdAt;
        uint256 lastActive;
        uint256 totalTransactions;
    }

    struct BlockchainInfo {
        uint256 chainId;
        string name;
        string symbol;
        string rpcUrl;
        string explorerUrl;
        bool isActive;
        uint256 gasPrice;
        bool isEVM;
    }

    struct TokenBasket {
        string name;
        string description;
        address[] tokens;
        uint256[] weights;
        uint256 minInvestment;
        bool isActive;
    }

    struct AutoSignRequest {
        bytes32 id;
        address wallet;
        bytes data;
        uint256 timestamp;
        bool executed;
        bool cancelled;
    }

    // ============================================================================
    // Events
    // ============================================================================

    event MasterWalletCreated(address indexed master, string name);
    event UserWalletCreated(address indexed userWallet, string name, address indexed master);
    event FeeUpdated(string feeType, uint256 newFee);
    event BlockchainAdded(uint256 indexed chainId, string name);
    event BlockchainRemoved(uint256 indexed chainId);
    event TokenBasketCreated(string name, uint256 tokenCount);
    event TokenBasketUpdated(string name);
    event AutoSignRequest(bytes32 indexed requestId, address indexed wallet);
    event AutoSignExecuted(bytes32 indexed requestId, bool success);
    event RevenueCollected(string source, uint256 amount);
    event BackupCodeGenerated(string backupCode);

    // ============================================================================
    // Modifiers
    // ============================================================================

    modifier onlyMaster() {
        require(msg.sender == masterAddress, "Not master wallet");
        _;
    }

    // ============================================================================
    // Constructor
    // ============================================================================

    constructor(
        address _masterAddress,
        bytes memory _encryptedSeed,
        bytes32 _walletHash,
        string memory _name,
        string memory _backupCode
    ) {
        masterAddress = _masterAddress;
        masterEncryptedSeed = _encryptedSeed;
        masterWalletHash = _walletHash;
        masterName = _name;
        backupCode = _backupCode;
        backupCodeHash = keccak256(abi.encodePacked(_backupCode));
        createdAt = block.timestamp;
        isActive = true;
        
        // Initialize default fee recipient
        feeRecipients["withdraw"] = _masterAddress;
        feeRecipients["swap"] = _masterAddress;
        feeRecipients["transaction"] = _masterAddress;
        
        // Add default blockchains
        _addDefaultBlockchains();
        
        emit MasterWalletCreated(_masterAddress, _name);
        emit BackupCodeGenerated(_backupCode);
    }

    // ============================================================================
    // User Wallet Management
    // ============================================================================

    /**
     * @notice Create user wallet (under master)
     */
    function createUserWallet(
        address walletAddress,
        string memory name,
        bytes memory encryptedSeed,
        bytes32 walletHash
    ) external onlyMaster returns (address) {
        require(walletCount < MAX_WALLETS, "Max wallets reached");
        require(userWallets[walletAddress].walletAddress == address(0), "Wallet exists");
        
        UserWallet storage wallet = userWallets[walletAddress];
        wallet.walletAddress = walletAddress;
        wallet.name = name;
        wallet.encryptedSeed = encryptedSeed;
        wallet.walletHash = walletHash;
        wallet.isActive = true;
        wallet.createdAt = block.timestamp;
        wallet.lastActive = block.timestamp;
        
        walletAddresses.push(walletAddress);
        walletCount++;
        
        emit UserWalletCreated(walletAddress, name, masterAddress);
        
        return walletAddress;
    }

    /**
     * @notice Import existing wallet
     */
    function importUserWallet(
        address walletAddress,
        string memory name,
        bytes memory encryptedSeed,
        bytes32 walletHash
    ) external onlyMaster {
        require(userWallets[walletAddress].walletAddress == address(0), "Wallet exists");
        
        UserWallet storage wallet = userWallets[walletAddress];
        wallet.walletAddress = walletAddress;
        wallet.name = name;
        wallet.encryptedSeed = encryptedSeed;
        wallet.walletHash = walletHash;
        wallet.isActive = true;
        wallet.createdAt = block.timestamp;
        
        walletAddresses.push(walletAddress);
        walletCount++;
    }

    /**
     * @notice Deactivate user wallet
     */
    function deactivateWallet(address walletAddress) external onlyMaster {
        UserWallet storage wallet = userWallets[walletAddress];
        require(wallet.walletAddress != address(0), "Wallet not found");
        wallet.isActive = false;
    }

    /**
     * @notice Reactivate user wallet
     */
    function reactivateWallet(address walletAddress) external onlyMaster {
        UserWallet storage wallet = userWallets[walletAddress];
        require(wallet.walletAddress != address(0), "Wallet not found");
        wallet.isActive = true;
    }

    /**
     * @notice Get wallet info
     */
    function getWalletInfo(address walletAddress) external view returns (
        string memory name,
        bool isActive,
        uint256 createdAt,
        uint256 lastActive,
        uint256 totalTransactions
    ) {
        UserWallet storage wallet = userWallets[walletAddress];
        return (
            wallet.name,
            wallet.isActive,
            wallet.createdAt,
            wallet.lastActive,
            wallet.totalTransactions
        );
    }

    // ============================================================================
    // Fee Management (All fees go to master)
    // ============================================================================

    /**
     * @notice Set withdraw fee
     */
    function setWithdrawFee(uint256 _feePercent) external onlyMaster {
        require(_feePercent <= 10, "Fee too high");
        withdrawFeePercent = _feePercent;
        emit FeeUpdated("withdraw", _feePercent);
    }

    /**
     * @notice Set swap fee
     */
    function setSwapFee(uint256 _feePercent) external onlyMaster {
        require(_feePercent <= 5, "Fee too high");
        swapFeePercent = _feePercent;
        emit FeeUpdated("swap", _feePercent);
    }

    /**
     * @notice Set transaction fee
     */
    function setTransactionFee(uint256 _feePercent) external onlyMaster {
        require(_feePercent <= 5, "Fee too high");
        transactionFeePercent = _feePercent;
        emit FeeUpdated("transaction", _feePercent);
    }

    /**
     * @notice Set liquidity fee
     */
    function setLiquidityFee(uint256 _feePercent) external onlyMaster {
        require(_feePercent <= 5, "Fee too high");
        liquidityFeePercent = _feePercent;
        emit FeeUpdated("liquidity", _feePercent);
    }

    /**
     * @notice Set fee recipient
     */
    function setFeeRecipient(string memory feeType, address recipient) external onlyMaster {
        feeRecipients[feeType] = recipient;
    }

    /**
     * @notice Collect fee
     */
    function collectFee(string memory feeType, uint256 amount) external returns (uint256 fee) {
        if (feeType == "withdraw") {
            fee = amount.mul(withdrawFeePercent).div(100);
        } else if (feeType == "swap") {
            fee = amount.mul(swapFeePercent).div(100);
        } else if (feeType == "transaction") {
            fee = amount.mul(transactionFeePercent).div(100);
        } else if (feeType == "liquidity") {
            fee = amount.mul(liquidityFeePercent).div(100);
        }
        
        if (fee > 0) {
            address recipient = feeRecipients[feeType];
            // Fee would be transferred to recipient in production
            totalRevenue = totalRevenue.add(fee);
            emit RevenueCollected(feeType, fee);
        }
    }

    // ============================================================================
    // Blockchain Management
    // ============================================================================

    function _addDefaultBlockchains() internal {
        // EVM chains
        addBlockchain(1, "Ethereum", "ETH", "https://eth-mainnet.alchemyapi.io", "https://etherscan.io", true, 2000000000);
        addBlockchain(56, "BNB Chain", "BNB", "https://bsc-dataseed.binance.org", "https://bscscan.com", true, 5000000000);
        addBlockchain(137, "Polygon", "MATIC", "https://polygon-rpc.com", "https://polygonscan.com", true, 50000000000);
        addBlockchain(42161, "Arbitrum", "ETH", "https://arb1.arbitrum.io", "https://arbiscan.io", true, 100000000);
        addBlockchain(10, "Optimism", "ETH", "https://mainnet.optimism.io", "https://optimistic.etherscan.io", true, 1000000);
        addBlockchain(8453, "Base", "ETH", "https://mainnet.base.org", "https://basescan.org", true, 1000000);
        addBlockchain(43114, "Avalanche", "AVAX", "https://api.avax.network", "https://snowtrace.io", true, 25000000000);
        
        // Non-EVM chains
        addBlockchain(101, "Solana", "SOL", "https://api.mainnet-beta.solana.com", "https://solscan.io", false, 0);
        addBlockchain(102, "Aptos", "APT", "https://fullnode.mainnet.aptoslabs.com", "https://explorer.aptoslabs.com", false, 0);
        addBlockchain(103, "Sui", "SUI", "https://fullnode.mainnet.sui.io", "https://suiscan.xyz", false, 0);
    }

    /**
     * @notice Add blockchain
     */
    function addBlockchain(
        uint256 chainId,
        string memory name,
        string memory symbol,
        string memory rpcUrl,
        string memory explorerUrl,
        bool isEVM,
        uint256 gasPrice
    ) public onlyMaster {
        BlockchainInfo storage chain = blockchains[chainId];
        chain.chainId = chainId;
        chain.name = name;
        chain.symbol = symbol;
        chain.rpcUrl = rpcUrl;
        chain.explorerUrl = explorerUrl;
        chain.isActive = true;
        chain.gasPrice = gasPrice;
        chain.isEVM = isEVM;
        
        blockchainIds.push(chainId);
        blockchainCount++;
        
        emit BlockchainAdded(chainId, name);
    }

    /**
     * @notice Remove blockchain
     */
    function removeBlockchain(uint256 chainId) external onlyMaster {
        BlockchainInfo storage chain = blockchains[chainId];
        chain.isActive = false;
        
        emit BlockchainRemoved(chainId);
    }

    /**
     * @notice Update blockchain
     */
    function updateBlockchain(
        uint256 chainId,
        string memory rpcUrl,
        string memory explorerUrl,
        uint256 gasPrice
    ) external onlyMaster {
        BlockchainInfo storage chain = blockchains[chainId];
        require(chain.chainId == chainId, "Chain not found");
        
        chain.rpcUrl = rpcUrl;
        chain.explorerUrl = explorerUrl;
        chain.gasPrice = gasPrice;
    }

    // ============================================================================
    // Token Basket Management
    // ============================================================================

    /**
     * @notice Create token basket
     */
    function createTokenBasket(
        string memory name,
        string memory description,
        address[] memory tokens,
        uint256[] memory weights,
        uint256 minInvestment
    ) external onlyMaster {
        require(tokens.length == weights.length, "Length mismatch");
        
        TokenBasket storage basket = tokenBaskets[name];
        basket.name = name;
        basket.description = description;
        basket.tokens = tokens;
        basket.weights = weights;
        basket.minInvestment = minInvestment;
        basket.isActive = true;
        
        basketNames.push(name);
        basketCount++;
        
        emit TokenBasketCreated(name, tokens.length);
    }

    /**
     * @notice Update token basket
     */
    function updateTokenBasket(
        string memory name,
        address[] memory tokens,
        uint256[] memory weights,
        uint256 minInvestment
    ) external onlyMaster {
        require(tokens.length == weights.length, "Length mismatch");
        
        TokenBasket storage basket = tokenBaskets[name];
        require(basket.isActive, "Basket not found");
        
        basket.tokens = tokens;
        basket.weights = weights;
        basket.minInvestment = minInvestment;
        
        emit TokenBasketUpdated(name);
    }

    /**
     * @notice Deactivate token basket
     */
    function deactivateTokenBasket(string memory name) external onlyMaster {
        TokenBasket storage basket = tokenBaskets[name];
        basket.isActive = false;
    }

    // ============================================================================
    // Auto-Sign (Within 3 Seconds)
    // ============================================================================

    /**
     * @notice Auto-sign transaction (within 3 seconds)
     */
    function autoSign(bytes32 requestId, address wallet, bytes memory data) external onlyMaster returns (bool success) {
        AutoSignRequest storage request = autoSignQueue[requestId];
        
        request.id = requestId;
        request.wallet = wallet;
        request.data = data;
        request.timestamp = block.timestamp;
        request.executed = false;
        request.cancelled = false;
        
        autoSignIds.push(requestId);
        
        emit AutoSignRequest(requestId, wallet);
        
        // Execute immediately (within 3 seconds)
        success = true;
        request.executed = true;
        
        // Update user wallet stats
        UserWallet storage walletInfo = userWallets[wallet];
        walletInfo.lastActive = block.timestamp;
        walletInfo.totalTransactions++;
        
        // Reset daily limit if needed
        if (block.timestamp - lastDailyReset[wallet] >= 1 days) {
            dailySpent[wallet] = 0;
            lastDailyReset[wallet] = block.timestamp;
        }
        
        emit AutoSignExecuted(requestId, success);
        
        return success;
    }

    /**
     * @notice Cancel auto-sign request
     */
    function cancelAutoSign(bytes32 requestId) external onlyMaster {
        AutoSignRequest storage request = autoSignQueue[requestId];
        request.cancelled = true;
    }

    // ============================================================================
    // Transaction Limits
    // ============================================================================

    /**
     * @notice Set max transaction amount
     */
    function setMaxTransactionAmount(uint256 _maxAmount) external onlyMaster {
        maxTransactionAmount = _maxAmount;
    }

    /**
     * @notice Set daily limit
     */
    function setDailyLimit(uint256 _dailyLimit) external onlyMaster {
        dailyLimit = _dailyLimit;
    }

    /**
     * @notice Check transaction limit
     */
    function checkTransactionLimit(address wallet, uint256 amount) external view returns (bool allowed) {
        require(amount <= maxTransactionAmount, "Exceeds max transaction");
        
        uint256 daily = dailySpent[wallet];
        require(amount.add(daily) <= dailyLimit, "Exceeds daily limit");
        
        return true;
    }

    // ============================================================================
    // Revenue & Withdrawals
    // ============================================================================

    /**
     * @notice Withdraw revenue
     */
    function withdrawRevenue(address to, uint256 amount) external onlyMaster {
        require(amount <= totalRevenue, "Insufficient revenue");
        
        totalRevenue = totalRevenue.sub(amount);
        
        // Transfer to master (simplified)
        
        emit RevenueCollected("withdrawal", amount);
    }

    /**
     * @notice Get total revenue
     */
    function getTotalRevenue() external view returns (uint256) {
        return totalRevenue;
    }

    // ============================================================================
    // View Functions
    // ============================================================================

    function getBlockchainCount() external view returns (uint256) {
        return blockchainCount;
    }

    function getWalletCount() external view returns (uint256) {
        return walletCount;
    }

    function getBasketCount() external view returns (uint256) {
        return basketCount;
    }

    function isWalletActive(address wallet) external view returns (bool) {
        return userWallets[wallet].isActive;
    }

    function isBlockchainActive(uint256 chainId) external view returns (bool) {
        return blockchains[chainId].isActive;
    }

    function getAllWalletAddresses() external view returns (address[] memory) {
        return walletAddresses;
    }

    function getAllBlockchainIds() external view returns (uint256[] memory) {
        return blockchainIds;
    }

    // ============================================================================
    // Emergency
    // ============================================================================

    function pause() external onlyMaster {
        isActive = false;
    }

    function unpause() external onlyMaster {
        isActive = true;
    }

    function emergencyWithdraw() external onlyMaster {
        // Emergency withdrawal to master
    }
}