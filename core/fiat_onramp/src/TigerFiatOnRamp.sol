// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/**
 * @title TigerFiatOnRamp
 * @notice Production Fiat On-Ramp
 * @dev Crypto purchasing with fiat currency
 * 
 * Features:
 * - Card payments
 * - Bank transfers
 * - Multiple fiat currencies
 * - KYC integration
 * - Limits management
 * - Order tracking
 * 
 * @author TigerSwap
 * @version 1.0.0 Production
 */

import "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";
import "@openzeppelin/contracts/utils/ReentrancyGuard.sol";
import "@openzeppelin/contracts/access/Ownable.sol";
import "@openzeppelin/contracts/access/AccessControl.sol";

/**
 * @title FiatOnRamp Types
 */
library FiatTypes {
    uint256 constant MIN_PURCHASE = 30; // $30
    uint256 constant MAX_PURCHASE = 50000; // $50,000
    uint256 constant KYC_TIER_1 = 1000; // $1,000
    uint256 constant KYC_TIER_2 = 10000; // $10,000
    uint256 constant KYC_TIER_3 = 50000; // $50,000
}

/**
 * @title TigerFiatOnRamp
 * @dev Main fiat on-ramp contract
 */
contract TigerFiatOnRamp is ReentrancyGuard, Ownable, AccessControl {
    using SafeERC20 for IERC20;

    // ============ Roles ============
    bytes32 public constant PARTNER = keccak256("PARTNER");
    bytes32 public constant KYC_ADMIN = keccak256("KYC_ADMIN");
    bytes32 public constant COMPLIANCE = keccak256("COMPLIANCE");

    // ============ Constants ============
    uint256 constant MIN_PURCHASE = 30e18; // $30
    uint256 constant MAX_PURCHASE = 50000e18; // $50,000

    // ============ State Variables ============
    
    // Supported tokens
    mapping(address => bool) public supportedTokens;
    address[] public tokenList;
    
    // Supported fiat currencies
    mapping(string => bool) public supportedFiat;
    string[] public fiatList;
    
    // Payment providers
    mapping(address => bool) public providers;
    address[] public providerList;
    
    // Orders
    uint256 public orderCount;
    mapping(uint256 => Order) public orders;
    mapping(address => uint256[]) public userOrders;
    
    // KYC
    mapping(address => KYCStatus) public kycStatus;
    mapping(address => uint256) public kycLimit;
    
    // Limits
    uint256 public dailyLimit;
    uint256 public monthlyLimit;
    mapping(address => uint256) public dailySpent;
    mapping(address => uint256) public monthlySpent;
    uint256 public lastResetTime;

    // ============ Enums ============
    
    enum OrderStatus {
        PENDING,
        PROCESSING,
        COMPLETED,
        FAILED,
        CANCELLED,
        REFUNDED
    }
    
    enum PaymentMethod {
        CARD,
        BANK_TRANSFER,
        SEPA,
        SWIFT,
        PIX
    }
    
    enum KYCLevel {
        NONE,
        BASIC,
        INTERMEDIATE,
        FULL
    }

    // ============ Structs ============
    
    struct Order {
        uint256 orderId;
        address user;
        address token;
        string fiatCurrency;
        uint256 fiatAmount;
        uint256 tokenAmount;
        uint256 rate;
        PaymentMethod paymentMethod;
        string providerOrderId;
        OrderStatus status;
        uint256 createdAt;
        uint256 completedAt;
        address partner;
    }
    
    struct KYCStatus {
        KYCLevel level;
        uint256 limit;
        bool suspended;
        uint256 verifiedAt;
        string provider;
    }

    // ============ Events ============
    event OrderCreated(
        uint256 indexed orderId,
        address indexed user,
        address token,
        uint256 fiatAmount,
        uint256 tokenAmount
    );
    event OrderCompleted(uint256 indexed orderId, uint256 tokenAmount);
    event OrderFailed(uint256 indexed orderId, string reason);
    event OrderCancelled(uint256 indexed orderId);
    event OrderRefunded(uint256 indexed orderId, uint256 amount);
    event KYCSubmitted(address indexed user, KYCLevel level);
    event KYCApproved(address indexed user, KYCLevel level, uint256 limit);
    event KYCSuspended(address indexed user, string reason);
    event ProviderAdded(address indexed provider);
    event ProviderRemoved(address indexed provider);
    event TokenAdded(address indexed token);
    event TokenRemoved(address indexed token);
    event LimitsUpdated(uint256 dailyLimit, uint256 monthlyLimit);

    // ============ Constructor ============
    
    constructor(address _owner) Ownable(_owner) {
        _grantRole(DEFAULT_ADMIN_ROLE, _owner);
        
        // Initialize limits
        dailyLimit = 100000e18; // $100,000/day
        monthlyLimit = 1000000e18; // $1,000,000/month
        lastResetTime = block.timestamp;
        
        // Add default supported tokens
        // In production: add USDC, USDT, etc.
    }

    // ============ Create Order ============

    /**
     * @notice Create a purchase order
     */
    function createOrder(
        address _token,
        string calldata _fiatCurrency,
        uint256 _fiatAmount,
        PaymentMethod _paymentMethod,
        string calldata _providerOrderId
    ) external nonReentrant returns (uint256) {
        require(supportedTokens[_token], "Token not supported");
        require(supportedFiat[_fiatCurrency], "Fiat not supported");
        require(_fiatAmount >= MIN_PURCHASE, "Below minimum");
        require(_fiatAmount <= MAX_PURCHASE, "Above maximum");
        
        // Check KYC
        KYCStatus memory kyc = kycStatus[msg.sender];
        require(kyc.level != KYCLevel.NONE, "KYC required");
        require(!kyc.suspended, "KYC suspended");
        
        // Check limits
        _checkLimits(msg.sender, _fiatAmount);
        
        // Calculate token amount (simplified rate)
        uint256 rate = _getRate(_token, _fiatCurrency);
        uint256 tokenAmount = (_fiatAmount * 1e18) / rate;
        
        // Create order
        uint256 orderId = ++orderCount;
        
        orders[orderId] = Order({
            orderId: orderId,
            user: msg.sender,
            token: _token,
            fiatCurrency: _fiatCurrency,
            fiatAmount: _fiatAmount,
            tokenAmount: tokenAmount,
            rate: rate,
            paymentMethod: _paymentMethod,
            providerOrderId: _providerOrderId,
            status: OrderStatus.PENDING,
            createdAt: block.timestamp,
            completedAt: 0,
            partner: address(0)
        });
        
        userOrders[msg.sender].push(orderId);
        
        // Update spent
        dailySpent[msg.sender] += _fiatAmount;
        monthlySpent[msg.sender] += _fiatAmount;
        
        emit OrderCreated(orderId, msg.sender, _token, _fiatAmount, tokenAmount);
        
        return orderId;
    }

    // ============ Order Fulfillment ============

    /**
     * @notice Complete order (called by provider/oracle)
     */
    function completeOrder(
        uint256 _orderId,
        string calldata _providerTxId
    ) external onlyRole(PARTNER) nonReentrant {
        Order storage order = orders[_orderId];
        
        require(order.status == OrderStatus.PENDING, "Not pending");
        require(order.user != address(0), "Invalid order");
        
        // Mark as completed
        order.status = OrderStatus.COMPLETED;
        order.completedAt = block.timestamp;
        
        // Transfer tokens
        IERC20(order.token).safeTransfer(order.user, order.tokenAmount);
        
        emit OrderCompleted(_orderId, order.tokenAmount);
    }

    /**
     * @notice Mark order as failed
     */
    function failOrder(uint256 _orderId, string calldata _reason) 
        external 
        onlyRole(PARTNER) 
    {
        Order storage order = orders[_orderId];
        
        require(order.status == OrderStatus.PENDING, "Not pending");
        
        order.status = OrderStatus.FAILED;
        
        emit OrderFailed(_orderId, _reason);
    }

    /**
     * @notice Cancel order
     */
    function cancelOrder(uint256 _orderId) external {
        Order storage order = orders[_orderId];
        
        require(order.user == msg.sender, "Not owner");
        require(order.status == OrderStatus.PENDING, "Cannot cancel");
        
        order.status = OrderStatus.CANCELLED;
        
        // Refund limits
        dailySpent[msg.sender] -= order.fiatAmount;
        monthlySpent[msg.sender] -= order.fiatAmount;
        
        emit OrderCancelled(_orderId);
    }

    // ============ KYC ============

    /**
     * @notice Submit KYC
     */
    function submitKYC(string calldata _provider, KYCLevel _level) external {
        require(uint8(_level) > 0, "Invalid level");
        
        KYCStatus storage kyc = kycStatus[msg.sender];
        
        kyc.level = _level;
        kyc.provider = _provider;
        kyc.verifiedAt = block.timestamp;
        
        // Set limit based on level
        if (_level == KYCLevel.BASIC) {
            kycLimit[msg.sender] = 1000e18;
        } else if (_level == KYCLevel.INTERMEDIATE) {
            kycLimit[msg.sender] = 10000e18;
        } else if (_level == KYCLevel.FULL) {
            kycLimit[msg.sender] = 50000e18;
        }
        
        emit KYCSubmitted(msg.sender, _level);
    }

    /**
     * @notice Approve KYC (admin)
     */
    function approveKYC(address _user, KYCLevel _level, uint256 _limit) 
        external 
        onlyRole(KYC_ADMIN) 
    {
        KYCStatus storage kyc = kycStatus[_user];
        
        kyc.level = _level;
        kyc.limit = _limit;
        kyc.verifiedAt = block.timestamp;
        
        emit KYCApproved(_user, _level, _limit);
    }

    /**
     * @notice Suspend KYC
     */
    function suspendKYC(address _user, string calldata _reason) 
        external 
        onlyRole(COMPLIANCE) 
    {
        KYCStatus storage kyc = kycStatus[_user];
        
        kyc.suspended = true;
        
        emit KYCSuspended(_user, _reason);
    }

    // ============ Limits Management ============

    /**
     * @dev Check spending limits
     */
    function _checkLimits(address _user, uint256 _amount) internal view {
        // Daily limit
        require(dailySpent[_user] + _amount <= dailyLimit, "Daily limit exceeded");
        
        // Monthly limit
        require(monthlySpent[_user] + _amount <= monthlyLimit, "Monthly limit exceeded");
        
        // KYC limit
        require(kycLimit[_user] > 0, "KYC limit not set");
        require(kycLimit[_user] >= _amount, "KYC limit exceeded");
    }

    /**
     * @notice Reset daily limits
     */
    function resetDailyLimits() external {
        if (block.timestamp - lastResetTime >= 1 days) {
            // In production: iterate through all users
            // For now: just reset time
            lastResetTime = block.timestamp;
        }
    }

    // ============ Admin Functions ============

    /**
     * @notice Add supported token
     */
    function addToken(address _token) external onlyOwner {
        require(_token != address(0), "Invalid token");
        
        if (!supportedTokens[_token]) {
            supportedTokens[_token] = true;
            tokenList.push(_token);
            
            emit TokenAdded(_token);
        }
    }

    /**
     * @notice Remove supported token
     */
    function removeToken(address _token) external onlyOwner {
        require(supportedTokens[_token], "Token not supported");
        
        supportedTokens[_token] = false;
        
        emit TokenRemoved(_token);
    }

    /**
     * @notice Add supported fiat
     */
    function addFiat(string calldata _currency) external onlyOwner {
        if (!supportedFiat[_currency]) {
            supportedFiat[_currency] = true;
            fiatList.push(_currency);
        }
    }

    /**
     * @notice Add payment provider
     */
    function addProvider(address _provider) external onlyOwner {
        require(_provider != address(0), "Invalid provider");
        
        if (!providers[_provider]) {
            providers[_provider] = true;
            providerList.push(_provider);
            
            emit ProviderAdded(_provider);
        }
    }

    /**
     * @notice Set limits
     */
    function setLimits(uint256 _dailyLimit, uint256 _monthlyLimit) 
        external 
        onlyOwner 
    {
        dailyLimit = _dailyLimit;
        monthlyLimit = _monthlyLimit;
        
        emit LimitsUpdated(_dailyLimit, _monthlyLimit);
    }

    // ============ View Functions ============

    /**
     * @notice Get order details
     */
    function getOrder(uint256 _orderId) external view returns (Order memory) {
        return orders[_orderId];
    }

    /**
     * @notice Get user orders
     */
    function getUserOrders(address _user) external view returns (uint256[] memory) {
        return userOrders[_user];
    }

    /**
     * @notice Get supported tokens
     */
    function getTokenList() external view returns (address[] memory) {
        return tokenList;
    }

    /**
     * @notice Get supported fiat currencies
     */
    function getFiatList() external view returns (string[] memory) {
        return fiatList;
    }

    /**
     * @notice Get rate for token/fiat pair
     */
    function getRate(address _token, string calldata _fiatCurrency) 
        external 
        view 
        returns (uint256) 
    {
        return _getRate(_token, _fiatCurrency);
    }

    /**
     * @dev Get rate (simplified)
     */
    function _getRate(address _token, string calldata _fiatCurrency) 
        internal 
        view 
        returns (uint256) 
    {
        // In production: integrate with price oracles
        // For now: return mock rates
        
        // ETH at $3000
        if (_token == 0xEeeeeEeeeEeEeeEeEeEeeEEEeeeeEeeeeeeeEEeE) {
            return 3000e18;
        }
        
        // USDC at $1
        return 1e18;
    }

    /**
     * @notice Get KYC status
     */
    function getKYC(address _user) external view returns (
        KYCLevel level,
        uint256 limit,
        bool suspended
    ) {
        KYCStatus storage kyc = kycStatus[_user];
        return (kyc.level, kyc.limit, kyc.suspended);
    }
}
