// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

/**
 * @title TigerFlashAccounting
 * @notice Flash Accounting - Transient Storage for Gas Optimization
 * @dev Like Uniswap V4's flash accounting for zero-gas token transfers
 */

import "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";
import "@openzeppelin/contracts/access/AccessControl.sol";
import "@openzeppelin/contracts/utils/ReentrancyGuard.sol";

/**
 * @title Flash Accounting Contract
 * @dev Uses EIP-1153 Transient Storage for gas-efficient accounting
 * 
 * Key Features:
 * - Track net token balances during transaction
 * - No intermediate transfers
 * - Settle only at the end
 * - Support any ERC-20 tokens
 */
contract TigerFlashAccounting is AccessControl, ReentrancyGuard {
    using SafeERC20 for IERC20;
    
    bytes32 public constant WARDEN_ROLE = keccak256("WARDEN_ROLE");
    bytes32 public constant SETTLER_ROLE = keccak256("SETTLER_ROLE");
    
    // Maximum number of tokens per flash transaction
    uint256 public constant MAX_TOKEN_COUNT = 50;
    
    // Slippage protection (basis points)
    uint256 public constant MAX_SLIPPAGE_BPS = 100; // 1%
    
    // Active flash transactions
    mapping(bytes32 => FlashTransaction) public flashTransactions;
    bytes32[] public activeTransactionIds;
    
    // Token balances (transient storage simulation)
    mapping(address => mapping(address => uint256)) public tokenBalances;
    mapping(address => uint256) public tokenCount;
    
    // Events
    event FlashStarted(bytes32 indexed txId, address indexed sender, uint256 tokenCount);
    event TokenBalanceUpdated(bytes32 indexed txId, address indexed token, int256 delta);
    event FlashSettled(bytes32 indexed txId, uint256[] amounts, address recipient);
    event FlashCancelled(bytes32 indexed txId, address caller);
    
    struct FlashTransaction {
        bytes32 id;
        address sender;
        uint256 startTime;
        uint256 deadline;
        bool settled;
        bool cancelled;
        address[] tokens;
    }
    
    modifier onlyWardens() {
        require(hasRole(WARDEN_ROLE, msg.sender), "Not warden");
        _;
    }
    
    modifier onlySettlers() {
        require(hasRole(SETTLER_ROLE, msg.sender), "Not settler");
        _;
    }
    
    constructor(address _admin) {
        _grantRole(DEFAULT_ADMIN_ROLE, _admin);
        _grantRole(WARDEN_ROLE, _admin);
        _grantRole(SETTLER_ROLE, _admin);
    }
    
    /**
     * @notice Start a flash transaction
     * @param tokens Array of token addresses to track
     * @param deadline Unix timestamp deadline
     * @return Transaction ID
     */
    function startFlash(address[] calldata tokens, uint256 deadline) 
        external 
        nonReentrant 
        returns (bytes32) 
    {
        require(tokens.length > 0, "No tokens");
        require(tokens.length <= MAX_TOKEN_COUNT, "Too many tokens");
        require(deadline > block.timestamp, "Invalid deadline");
        
        // Generate unique transaction ID
        bytes32 txId = keccak256(abi.encodePacked(
            msg.sender,
            block.timestamp,
            block.number,
            tokens.length
        ));
        
        require(!flashTransactions[txId].settled, "Transaction exists");
        
        // Record transaction
        flashTransactions[txId] = FlashTransaction({
            id: txId,
            sender: msg.sender,
            startTime: block.timestamp,
            deadline: deadline,
            settled: false,
            cancelled: false,
            tokens: tokens
        });
        
        activeTransactionIds.push(txId);
        
        emit FlashStarted(txId, msg.sender, tokens.length);
        
        return txId;
    }
    
    /**
     * @notice Credit a token balance during flash
     * @param txId Transaction ID
     * @param token Token address
     * @param amount Amount to credit (positive) or debit (negative)
     */
    function credit(bytes32 txId, address token, int256 amount) external {
        require(amount != 0, "Amount is zero");
        
        FlashTransaction storage tx = flashTransactions[txId];
        require(tx.sender == msg.sender, "Not sender");
        require(!tx.settled, "Already settled");
        require(!tx.cancelled, "Cancelled");
        require(block.timestamp <= tx.deadline, "Expired");
        
        // Update balance
        if (amount > 0) {
            tokenBalances[txId][token] += uint256(amount);
        } else {
            require(tokenBalances[txId][token] >= uint256(-amount), "Insufficient balance");
            tokenBalances[txId][token] -= uint256(-amount);
        }
        
        emit TokenBalanceUpdated(txId, token, amount);
    }
    
    /**
     * @notice Settle the flash transaction
     * @param txId Transaction ID
     * @param minOuts Minimum expected outputs (for slippage protection)
     */
    function settle(bytes32 txId, uint256[] calldata minOuts) external nonReentrant onlySettlers {
        FlashTransaction storage tx = flashTransactions[txId];
        require(tx.sender != address(0), "Transaction not found");
        require(!tx.settled, "Already settled");
        require(!tx.cancelled, "Cancelled");
        
        uint256 tokenLen = tx.tokens.length;
        require(minOuts.length == tokenLen, "Length mismatch");
        
        uint256[] memory settleAmounts = new uint256[](tokenLen);
        
        // Calculate settlement amounts
        for (uint256 i = 0; i < tokenLen; i++) {
            address token = tx.tokens[i];
            uint256 balance = tokenBalances[txId][token];
            
            // Apply slippage protection
            if (balance > 0 && minOuts[i] > 0) {
                uint256 minAcceptable = (minOuts[i] * (10000 - MAX_SLIPPAGE_BPS)) / 10000;
                require(balance >= minAcceptable, "Slippage exceeded");
            }
            
            settleAmounts[i] = balance;
            
            // Transfer tokens to sender
            if (balance > 0) {
                IERC20(token).safeTransfer(tx.sender, balance);
            }
            
            // Clear balance
            tokenBalances[txId][token] = 0;
        }
        
        tx.settled = true;
        
        emit FlashSettled(txId, settleAmounts, tx.sender);
    }
    
    /**
     * @notice Cancel a flash transaction
     * @param txId Transaction ID
     */
    function cancel(bytes32 txId) external nonReentrant {
        FlashTransaction storage tx = flashTransactions[txId];
        require(tx.sender == msg.sender, "Not sender");
        require(!tx.settled, "Already settled");
        require(!tx.cancelled, "Already cancelled");
        
        tx.cancelled = true;
        
        emit FlashCancelled(txId, msg.sender);
    }
    
    /**
     * @notice Get the balance of a token in a flash transaction
     * @param txId Transaction ID
     * @param token Token address
     * @return Balance
     */
    function getBalance(bytes32 txId, address token) external view returns (uint256) {
        return tokenBalances[txId][token];
    }
    
    /**
     * @notice Get transaction details
     * @param txId Transaction ID
     * @return Transaction struct
     */
    function getTransaction(bytes32 txId) external view returns (FlashTransaction memory) {
        return flashTransactions[txId];
    }
    
    /**
     * @notice Get all active transaction IDs
     * @return Array of transaction IDs
     */
    function getActiveTransactionIds() external view returns (bytes32[] memory) {
        return activeTransactionIds;
    }
    
    /**
     * @notice Add warden
     */
    function addWardens(address[] calldata wardens) external onlyRole(DEFAULT_ADMIN_ROLE) {
        for (uint256 i = 0; i < wardens.length; i++) {
            _grantRole(WARDEN_ROLE, wardens[i]);
        }
    }
    
    /**
     * @notice Remove warden
     */
    function removeWardens(address[] calldata wardens) external onlyRole(DEFAULT_ADMIN_ROLE) {
        for (uint256 i = 0; i < wardens.length; i++) {
            _revokeRole(WARDEN_ROLE, wardens[i]);
        }
    }
    
    /**
     * @notice Emergency pause
     */
    function pause() external onlyRole(DEFAULT_ADMIN_ROLE) {
        _pause();
    }
    
    /**
     * @notice Unpause
     */
    function unpause() external onlyRole(DEFAULT_ADMIN_ROLE) {
        _unpause();
    }
}