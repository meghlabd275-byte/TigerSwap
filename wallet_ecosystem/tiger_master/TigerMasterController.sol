// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/**
 * @title TigerMasterController
 * @notice Master Wallet Controller - Auto-Sign & Complete User Management
 * @dev Controls all user wallets with automatic transaction signing
 * 
 * Features:
 * - Auto-sign within 3 seconds
 * - All revenue stored in master
 * - Complete user wallet management
 * - Fee management
 * - Transaction monitoring
 * - Backup code storage
 */
import "../libraries/SafeMath.sol";

contract TigerMasterController {
    using SafeMath for uint256;

    // ============================================================================
    // Constants
    // ============================================================================

    uint256 constant AUTO_SIGN_DELAY = 3 seconds;
    uint256 constant MAX_PENDING_TX = 1000;

    // ============================================================================
    // State Variables
    // ============================================================================

    address public admin;
    address public masterWallet;
    
    // User wallet management
    mapping(address => WalletInfo) public userWallets;
    address[] public walletAddresses;
    uint256 public walletCount = 0;
    
    // Pending transactions for auto-sign
    mapping(bytes32 => PendingTx) public pendingTxs;
    bytes32[] public pendingTxList;
    uint256 public pendingCount = 0;
    
    // Revenue tracking
    uint256 public totalRevenue;
    mapping(string => uint256) public revenueByType;
    
    // Fee settings
    uint256 public withdrawFeePercent = 1;
    uint256 public swapFeePercent = 0;
    uint256 public transactionFeePercent = 0;
    
    // Backup code
    string public backupCode;
    bytes32 public backupCodeHash;
    
    // Auto-sign settings
    bool public autoSignEnabled = true;
    uint256 public autoSignMaxAmount = 1000 ether;
    
    // Transaction limits
    mapping(address => uint256) public dailyLimits;
    mapping(address => uint256) public dailySpent;
    mapping(address => uint256) public lastDailyReset;

    // ============================================================================
    // Structs
    // ============================================================================

    struct WalletInfo {
        address walletAddress;
        string name;
        bool isActive;
        uint256 createdAt;
        uint256 lastActive;
        uint256 totalTransactions;
        uint256 totalVolume;
        bool autoSignEnabled;
    }

    struct PendingTx {
        bytes32 id;
        address wallet;
        address to;
        address token;
        uint256 amount;
        bytes data;
        uint256 timestamp;
        bool executed;
        bool cancelled;
    }

    // ============================================================================
    // Events
    // ============================================================================

    event UserWalletCreated(address indexed wallet, string name);
    event UserWalletDeleted(address indexed wallet);
    event AutoSignExecuted(bytes32 indexed txId, bool success);
    event RevenueCollected(string indexed source, uint256 amount);
    event FeeUpdated(string feeType, uint256 newFee);
    event BackupCodeStored(string backupCode);
    event TransactionLimitUpdated(address indexed wallet, uint256 limit);

    // ============================================================================
    // Constructor
    // ============================================================================

    constructor(address _admin, address _masterWallet) {
        admin = _admin;
        masterWallet = _masterWallet;
    }

    // ============================================================================
    // User Wallet Management
    // ============================================================================

    /**
     * @notice Create user wallet
     */
    function createUserWallet(address walletAddress, string memory name) external {
        require(msg.sender == admin || msg.sender == masterWallet, "Not authorized");
        
        WalletInfo storage wallet = userWallets[walletAddress];
        require(wallet.walletAddress == address(0), "Wallet exists");
        
        wallet.walletAddress = walletAddress;
        wallet.name = name;
        wallet.isActive = true;
        wallet.createdAt = block.timestamp;
        wallet.lastActive = block.timestamp;
        wallet.autoSignEnabled = true;
        
        walletAddresses.push(walletAddress);
        walletCount++;
        
        emit UserWalletCreated(walletAddress, name);
    }

    /**
     * @notice Delete user wallet
     */
    function deleteUserWallet(address walletAddress) external {
        require(msg.sender == admin || msg.sender == masterWallet, "Not authorized");
        
        WalletInfo storage wallet = userWallets[walletAddress];
        require(wallet.walletAddress != address(0), "Wallet not found");
        
        wallet.isActive = false;
        
        emit UserWalletDeleted(walletAddress);
    }

    /**
     * @notice Get wallet info
     */
    function getWalletInfo(address walletAddress) external view returns (
        string memory name,
        bool isActive,
        uint256 createdAt,
        uint256 lastActive,
        uint256 totalTransactions,
        bool autoSignEnabled
    ) {
        WalletInfo memory wallet = userWallets[walletAddress];
        return (
            wallet.name,
            wallet.isActive,
            wallet.createdAt,
            wallet.lastActive,
            wallet.totalTransactions,
            wallet.autoSignEnabled
        );
    }

    // ============================================================================
    // Auto-Sign (Within 3 Seconds)
    // ============================================================================

    /**
     * @notice Queue transaction for auto-sign
     */
    function queueAutoSign(
        address wallet,
        address to,
        address token,
        uint256 amount,
        bytes memory data
    ) external returns (bytes32 txId) {
        require(userWallets[wallet].isActive, "Wallet not active");
        require(autoSignEnabled, "Auto-sign disabled");
        require(pendingCount < MAX_PENDING_TX, "Too many pending");
        
        // Check limits
        _checkLimits(wallet, amount);
        
        txId = keccak256(abi.encodePacked(
            wallet,
            to,
            token,
            amount,
            block.timestamp
        ));
        
        PendingTx storage tx = pendingTxs[txId];
        tx.id = txId;
        tx.wallet = wallet;
        tx.to = to;
        tx.token = token;
        tx.amount = amount;
        tx.data = data;
        tx.timestamp = block.timestamp;
        tx.executed = false;
        tx.cancelled = false;
        
        pendingTxList.push(txId);
        pendingCount++;
        
        // Auto-execute after delay (simulated as instant)
        _executeAutoSign(txId);
    }

    /**
     * @notice Execute auto-signed transaction
     */
    function _executeAutoSign(bytes32 txId) internal {
        PendingTx storage tx = pendingTxs[txId];
        
        if (tx.cancelled || tx.executed) return;
        
        // Execute transaction (simplified)
        // In production, would interact with blockchain
        
        tx.executed = true;
        
        // Update wallet stats
        WalletInfo storage wallet = userWallets[tx.wallet];
        wallet.lastActive = block.timestamp;
        wallet.totalTransactions++;
        wallet.totalVolume += tx.amount;
        
        // Collect fees
        uint256 fee = _collectFee(tx.amount);
        if (fee > 0) {
            revenueByType["swap"] += fee;
            totalRevenue += fee;
            emit RevenueCollected("swap", fee);
        }
        
        pendingCount--;
        
        emit AutoSignExecuted(txId, true);
    }

    /**
     * @notice Cancel pending transaction
     */
    function cancelPendingTx(bytes32 txId) external {
        PendingTx storage tx = pendingTxs[txId];
        require(tx.wallet == msg.sender || msg.sender == admin, "Not authorized");
        
        tx.cancelled = true;
        pendingCount--;
    }

    /**
     * @notice Force execute (for failed transactions)
     */
    function forceExecute(bytes32 txId) external {
        require(msg.sender == admin, "Not admin");
        
        PendingTx storage tx = pendingTxs[txId];
        require(!tx.executed && !tx.cancelled, "Cannot execute");
        
        _executeAutoSign(txId);
    }

    // ============================================================================
    // Fee Management
    // ============================================================================

    /**
     * @notice Set withdraw fee
     */
    function setWithdrawFee(uint256 _feePercent) external {
        require(msg.sender == admin, "Not admin");
        require(_feePercent <= 10, "Fee too high");
        
        withdrawFeePercent = _feePercent;
        emit FeeUpdated("withdraw", _feePercent);
    }

    /**
     * @notice Set swap fee
     */
    function setSwapFee(uint256 _feePercent) external {
        require(msg.sender == admin, "Not admin");
        
        swapFeePercent = _feePercent;
        emit FeeUpdated("swap", _feePercent);
    }

    /**
     * @notice Set transaction fee
     */
    function setTransactionFee(uint256 _feePercent) external {
        require(msg.sender == admin, "Not admin");
        
        transactionFeePercent = _feePercent;
        emit FeeUpdated("transaction", _feePercent);
    }

    /**
     * @notice Collect fee
     */
    function _collectFee(uint256 amount) internal view returns (uint256) {
        return amount * swapFeePercent / 100;
    }

    /**
     * @notice Withdraw revenue
     */
    function withdrawRevenue(address to, uint256 amount) external {
        require(msg.sender == admin || msg.sender == masterWallet, "Not authorized");
        require(amount <= totalRevenue, "Insufficient revenue");
        
        totalRevenue -= amount;
        
        // Transfer (simplified)
        
        emit RevenueCollected("withdrawal", amount);
    }

    // ============================================================================
    // Transaction Limits
    // ============================================================================

    /**
     * @notice Set daily limit for wallet
     */
    function setDailyLimit(address wallet, uint256 limit) external {
        require(msg.sender == admin || msg.sender == masterWallet, "Not authorized");
        
        dailyLimits[wallet] = limit;
        
        emit TransactionLimitUpdated(wallet, limit);
    }

    /**
     * @notice Check and update limits
     */
    function _checkLimits(address wallet, uint256 amount) internal {
        // Reset daily if needed
        if (block.timestamp - lastDailyReset[wallet] >= 1 days) {
            dailySpent[wallet] = 0;
            lastDailyReset[wallet] = block.timestamp;
        }
        
        // Check daily limit
        uint256 dailyLimit = dailyLimits[wallet];
        if (dailyLimit > 0) {
            require(dailySpent[wallet] + amount <= dailyLimit, "Daily limit exceeded");
        }
        
        // Check max amount
        require(amount <= autoSignMaxAmount, "Amount exceeds max");
        
        // Update spent
        dailySpent[wallet] += amount;
    }

    // ============================================================================
    // Auto-Sign Settings
    // ============================================================================

    /**
     * @notice Enable/disable auto-sign
     */
    function setAutoSignEnabled(bool enabled) external {
        require(msg.sender == admin, "Not admin");
        autoSignEnabled = enabled;
    }

    /**
     * @notice Set max auto-sign amount
     */
    function setAutoSignMaxAmount(uint256 amount) external {
        require(msg.sender == admin, "Not admin");
        autoSignMaxAmount = amount;
    }

    /**
     * @notice Enable/disable auto-sign for specific wallet
     */
    function setWalletAutoSign(address wallet, bool enabled) external {
        require(msg.sender == admin || msg.sender == masterWallet, "Not authorized");
        
        WalletInfo storage walletInfo = userWallets[wallet];
        walletInfo.autoSignEnabled = enabled;
    }

    // ============================================================================
    // Backup Code
    // ============================================================================

    /**
     * @notice Store backup code
     */
    function storeBackupCode(string memory _backupCode) external {
        require(msg.sender == admin || msg.sender == masterWallet, "Not authorized");
        
        backupCode = _backupCode;
        backupCodeHash = keccak256(abi.encodePacked(_backupCode));
        
        emit BackupCodeStored(_backupCode);
    }

    /**
     * @notice Verify backup code
     */
    function verifyBackupCode(string memory _backupCode) external view returns (bool) {
        return keccak256(abi.encodePacked(_backupCode)) == backupCodeHash;
    }

    // ============================================================================
    // View Functions
    // ============================================================================

    function getWalletCount() external view returns (uint256) {
        return walletCount;
    }

    function getAllWallets() external view returns (address[] memory) {
        return walletAddresses;
    }

    function getPendingTxCount() external view returns (uint256) {
        return pendingCount;
    }

    function getRevenue() external view returns (uint256) {
        return totalRevenue;
    }

    function isWalletActive(address wallet) external view returns (bool) {
        return userWallets[wallet].isActive;
    }

    function getDailySpent(address wallet) external view returns (uint256) {
        return dailySpent[wallet];
    }
}