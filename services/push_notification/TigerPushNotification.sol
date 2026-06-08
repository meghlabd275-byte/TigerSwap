// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";
import "@openzeppelin/contracts/access/AccessControl.sol";

/**
 * @title TigerPushNotification
 * @notice Push notification service for mobile and web
 */

contract TigerPushNotification is AccessControl {
    using SafeERC20 for IERC20;
    
    bytes32 public constant WARDEN_ROLE = keccak256("WARDEN_ROLE");
    bytes32 public constant OPERATOR_ROLE = keccak256("OPERATOR_ROLE");
    
    // Notification types
    enum NotificationType { 
        SwapCompleted, 
        OrderFilled, 
        PriceAlert, 
        Liquidation, 
        Governance,
        Security,
        System
    }
    
    // Subscription
    struct Subscription {
        address user;
        bool enabled;
        bool swapNotifications;
        bool orderNotifications;
        bool priceAlerts;
        bool governanceNotifications;
        bool securityAlerts;
    }
    
    // State
    uint256 public subscriptionCount;
    uint256 public notificationCount;
    mapping(address => Subscription) public subscriptions;
    mapping(address => uint256[]) public userNotifications;
    mapping(uint256 => Notification) public notifications;
    
    // Events
    event SubscriptionCreated(address indexed user);
    event SubscriptionUpdated(address indexed user);
    event NotificationSent(uint256 indexed id, address indexed user, NotificationType notifType);
    event NotificationRead(uint256 indexed id, address indexed user);
    event PriceAlertTriggered(address indexed user, string tokenPair, uint256 price);
    
    struct Notification {
        uint256 id;
        address user;
        NotificationType notifType;
        string title;
        string body;
        string data;
        uint256 timestamp;
        bool read;
    }
    
    modifier onlyOperators() {
        require(hasRole(OPERATOR_ROLE, msg.sender), "Not operator");
        _;
    }
    
    constructor(address _admin) {
        _grantRole(DEFAULT_ADMIN_ROLE, _admin);
        _grantRole(WARDEN_ROLE, _admin);
        _grantRole(OPERATOR_ROLE, _admin);
    }
    
    /**
     * @notice Create subscription
     */
    function createSubscription() external {
        Subscription storage sub = subscriptions[msg.sender];
        require(sub.user == address(0), "Already subscribed");
        
        sub.user = msg.sender;
        sub.enabled = true;
        sub.swapNotifications = true;
        sub.orderNotifications = true;
        sub.priceAlerts = true;
        sub.governanceNotifications = true;
        sub.securityAlerts = true;
        
        subscriptionCount++;
        
        emit SubscriptionCreated(msg.sender);
    }
    
    /**
     * @notice Update subscription preferences
     * @param _swap Enable swap notifications
     * @param _order Enable order notifications
     * @param _price Enable price alerts
     * @param _governance Enable governance notifications
     * @param _security Enable security notifications
     */
    function updateSubscription(
        bool _swap,
        bool _order,
        bool _price,
        bool _governance,
        bool _security
    ) external {
        Subscription storage sub = subscriptions[msg.sender];
        require(sub.user != address(0), "Not subscribed");
        
        sub.swapNotifications = _swap;
        sub.orderNotifications = _order;
        sub.priceAlerts = _price;
        sub.governanceNotifications = _governance;
        sub.securityAlerts = _security;
        
        emit SubscriptionUpdated(msg.sender);
    }
    
    /**
     * @notice Send swap notification
     * @param user User address
     * @param fromToken From token
     * @param toToken To token
     * @param amount Amount swapped
     */
    function sendSwapNotification(
        address user,
        string memory fromToken,
        string memory toToken,
        string memory amount
    ) external onlyOperators {
        Subscription storage sub = subscriptions[user];
        if (!sub.enabled || !sub.swapNotifications) return;
        
        _sendNotification(
            user,
            NotificationType.SwapCompleted,
            "Swap Completed",
            string(abi.encodePacked("Swapped ", amount, " ", fromToken, " for ", toToken)),
            ""
        );
    }
    
    /**
     * @notice Send order notification
     * @param user User address
     * @param orderType Order type
     * @param pair Trading pair
     * @param price Price
     */
    function sendOrderNotification(
        address user,
        string memory orderType,
        string memory pair,
        string memory price
    ) external onlyOperators {
        Subscription storage sub = subscriptions[user];
        if (!sub.enabled || !sub.orderNotifications) return;
        
        _sendNotification(
            user,
            NotificationType.OrderFilled,
            string(abi.encodePacked(orderType, " Order Filled")),
            string(abi.encodePacked(pair, " at ", price)),
            ""
        );
    }
    
    /**
     * @notice Send price alert
     * @param user User address
     * @param tokenPair Token pair
     * @param price Current price
     * @param direction Above or below target
     */
    function sendPriceAlert(
        address user,
        string memory tokenPair,
        string memory price,
        string memory direction
    ) external onlyOperators {
        Subscription storage sub = subscriptions[user];
        if (!sub.enabled || !sub.priceAlerts) return;
        
        _sendNotification(
            user,
            NotificationType.PriceAlert,
            string(abi.encodePacked(tokenPair, " Price Alert")),
            string(abi.encodePacked(tokenPair, " is now ", direction, " at ", price)),
            tokenPair
        );
    }
    
    /**
     * @notice Send governance notification
     * @param user User address
     * @param title Notification title
     * @param body Notification body
     */
    function sendGovernanceNotification(
        address user,
        string memory title,
        string memory body
    ) external onlyOperators {
        Subscription storage sub = subscriptions[user];
        if (!sub.enabled || !sub.governanceNotifications) return;
        
        _sendNotification(user, NotificationType.Governance, title, body, "");
    }
    
    /**
     * @notice Send security notification
     * @param user User address
     * @param title Notification title
     * @param body Notification body
     */
    function sendSecurityNotification(
        address user,
        string memory title,
        string memory body
    ) external onlyOperators {
        Subscription storage sub = subscriptions[user];
        if (!sub.enabled || !sub.securityAlerts) return;
        
        _sendNotification(user, NotificationType.Security, title, body, "");
    }
    
    /**
     * @notice Mark notification as read
     * @param id Notification ID
     */
    function markAsRead(uint256 id) external {
        Notification storage notif = notifications[id];
        require(notif.user == msg.sender, "Not owner");
        
        notif.read = true;
        
        emit NotificationRead(id, msg.sender);
    }
    
    /**
     * @notice Get user notification count
     * @param user User address
     * @return Number of notifications
     */
    function getUserNotificationCount(address user) external view returns (uint256) {
        return userNotifications[user].length;
    }
    
    /**
     * @notice Get subscription status
     * @param user User address
     * @return Enabled status
     */
    function getSubscriptionStatus(address user) external view returns (bool) {
        return subscriptions[user].enabled;
    }
    
    // Internal
    
    function _sendNotification(
        address user,
        NotificationType notifType,
        string memory title,
        string memory body,
        string memory data
    ) internal {
        uint256 id = ++notificationCount;
        
        notifications[id] = Notification({
            id: id,
            user: user,
            notifType: notifType,
            title: title,
            body: body,
            data: data,
            timestamp: block.timestamp,
            read: false
        });
        
        userNotifications[user].push(id);
        
        emit NotificationSent(id, user, notifType);
    }
}