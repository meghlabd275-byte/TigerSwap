// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";
import "@openzeppelin/contracts/access/AccessControl.sol";
import "@openzeppelin/contracts/utils/ReentrancyGuard.sol";

/**
 * @title TigerBotMarketplace
 * @notice Bot Trading Marketplace
 * @dev Users can buy, sell, and rent trading bots
 */

contract TigerBotMarketplace is AccessControl, ReentrancyGuard {
    using SafeERC20 for IERC20;
    
    bytes32 public constant WARDEN_ROLE = keccak256("WARDEN_ROLE");
    bytes32 public constant APPROVER_ROLE = keccak256("APPROVER_ROLE");
    
    // Bot types
    enum BotType { Grid, DCA, Arbitrage, Sniper, Liquidity, Custom }
    enum ListingType { Sale, Rent, Subscription }
    enum BotStatus { Pending, Active, Paused, Deprecated }
    
    // Fee
    uint256 public constant PLATFORM_FEE_BPS = 250; // 2.5%
    
    // State
    IERC20 public paymentToken;
    uint256 public platformFees;
    uint256 public botCount;
    uint256 public listingCount;
    bool public paused;
    
    // Bot definitions
    mapping(uint256 => Bot) public bots;
    mapping(address => uint256[]) public creatorBots;
    
    // Listings
    mapping(uint256 => Listing) public listings;
    mapping(address => uint256[]) public userListings;
    mapping(uint256 => uint256[]) public botListings;
    
    // Purchases
    mapping(uint256 => Purchase) public purchases;
    mapping(address => uint256[]) public userPurchases;
    uint256 public purchaseCount;
    
    // Ratings
    mapping(uint256 => uint256[]) public botRatings;
    mapping(uint256 => uint256) public botRatingSums;
    
    // Events
    event BotCreated(uint256 indexed botId, address indexed creator, string name, BotType botType);
    event BotUpdated(uint256 indexed botId);
    event ListingCreated(uint256 indexed listingId, uint256 indexed botId, ListingType listingType, uint256 price);
    event BotPurchased(uint256 indexed purchaseId, uint256 indexed listingId, address indexed buyer, uint256 price);
    event SubscriptionRenewed(uint256 indexed purchaseId, uint256 newExpiry);
    event RatingSubmitted(uint256 indexed botId, address indexed user, uint256 rating);
    event PlatformFeeWithdrawn(uint256 amount);
    
    struct Bot {
        uint256 id;
        address creator;
        string name;
        string description;
        string strategyHash; // IPFS hash of strategy code
        BotType botType;
        BotStatus status;
        uint256 createdAt;
        uint256 updatedAt;
        uint256 totalSales;
        uint256 totalRevenue;
    }
    
    struct Listing {
        uint256 id;
        uint256 botId;
        address seller;
        ListingType listingType;
        uint256 price;
        uint256 subscriptionDuration; // in seconds
        bool active;
    }
    
    struct Purchase {
        uint256 id;
        uint256 listingId;
        address buyer;
        uint256 price;
        uint256 boughtAt;
        uint256 expiresAt;
        bool active;
    }
    
    modifier onlyWardens() {
        require(hasRole(WARDEN_ROLE, msg.sender), "Not warden");
        _;
    }
    
    modifier onlyApprovers() {
        require(hasRole(APPROVER_ROLE, msg.sender), "Not approver");
        _;
    }
    
    modifier whenNotPaused() {
        require(!paused, "Paused");
        _;
    }
    
    constructor(address _paymentToken, address _admin) {
        require(_paymentToken != address(0), "Invalid token");
        
        paymentToken = IERC20(_paymentToken);
        
        _grantRole(DEFAULT_ADMIN_ROLE, _admin);
        _grantRole(WARDEN_ROLE, _admin);
        _grantRole(APPROVER_ROLE, _admin);
    }
    
    /**
     * @notice Create a new bot
     * @param name Bot name
     * @param description Bot description
     * @param strategyHash IPFS hash of strategy
     * @param botType Bot type
     */
    function createBot(
        string memory name,
        string memory description,
        string memory strategyHash,
        BotType botType
    ) external whenNotPaused returns (uint256) {
        require(bytes(name).length > 0, "Empty name");
        require(bytes(strategyHash).length > 0, "Empty strategy");
        
        uint256 botId = ++botCount;
        
        bots[botId] = Bot({
            id: botId,
            creator: msg.sender,
            name: name,
            description: description,
            strategyHash: strategyHash,
            botType: botType,
            status: BotStatus.Pending,
            createdAt: block.timestamp,
            updatedAt: block.timestamp,
            totalSales: 0,
            totalRevenue: 0
        });
        
        creatorBots[msg.sender].push(botId);
        
        emit BotCreated(botId, msg.sender, name, botType);
        
        return botId;
    }
    
    /**
     * @notice Update bot
     * @param botId Bot ID
     * @param name New name
     * @param description New description
     */
    function updateBot(uint256 botId, string memory name, string memory description) external whenNotPaused {
        Bot storage bot = bots[botId];
        require(bot.creator == msg.sender, "Not creator");
        require(bot.status != BotStatus.Deprecated, "Deprecated");
        
        bot.name = name;
        bot.description = description;
        bot.updatedAt = block.timestamp;
        
        emit BotUpdated(botId);
    }
    
    /**
     * @notice Approve bot (by approver)
     * @param botId Bot ID
     */
    function approveBot(uint256 botId) external onlyApprovers whenNotPaused {
        Bot storage bot = bots[botId];
        require(bot.creator != address(0), "Bot not found");
        require(bot.status == BotStatus.Pending, "Not pending");
        
        bot.status = BotStatus.Active;
        bot.updatedAt = block.timestamp;
    }
    
    /**
     * @notice Create listing
     * @param botId Bot ID
     * @param listingType Sale, Rent, or Subscription
     * @param price Price
     * @param subscriptionDuration Duration for rentals/subscriptions
     */
    function createListing(
        uint256 botId,
        ListingType listingType,
        uint256 price,
        uint256 subscriptionDuration
    ) external whenNotPaused returns (uint256) {
        Bot storage bot = bots[botId];
        require(bot.creator == msg.sender, "Not creator");
        require(bot.status == BotStatus.Active, "Not active");
        require(price > 0, "Price is 0");
        
        uint256 listingId = ++listingCount;
        
        listings[listingId] = Listing({
            id: listingId,
            botId: botId,
            seller: msg.sender,
            listingType: listingType,
            price: price,
            subscriptionDuration: subscriptionDuration,
            active: true
        });
        
        userListings[msg.sender].push(listingId);
        botListings[botId].push(listingId);
        
        emit ListingCreated(listingId, botId, listingType, price);
        
        return listingId;
    }
    
    /**
     * @notice Purchase bot
     * @param listingId Listing ID
     */
    function purchase(uint256 listingId) external nonReentrant whenNotPaused {
        Listing storage listing = listings[listingId];
        require(listing.active, "Not active");
        require(listing.seller != msg.sender, "Cannot buy own");
        
        uint256 price = listing.price;
        
        // Calculate fees
        uint256 platformFee = (price * PLATFORM_FEE_BPS) / 10000;
        uint256 sellerAmount = price - platformFee;
        
        // Transfer payment
        paymentToken.safeTransferFrom(msg.sender, address(this), price);
        paymentToken.safeTransfer(listing.seller, sellerAmount);
        
        // Update platform fees
        platformFees += platformFee;
        
        // Create purchase
        uint256 purchaseId = ++purchaseCount;
        uint256 expiresAt = listing.listingType == ListingType.Sale 
            ? uint256(-1) 
            : block.timestamp + listing.subscriptionDuration;
        
        purchases[purchaseId] = Purchase({
            id: purchaseId,
            listingId: listingId,
            buyer: msg.sender,
            price: price,
            boughtAt: block.timestamp,
            expiresAt: expiresAt,
            active: true
        });
        
        userPurchases[msg.sender].push(purchaseId);
        
        // Update bot stats
        Bot storage bot = bots[listing.botId];
        bot.totalSales += 1;
        bot.totalRevenue += price;
        
        emit BotPurchased(purchaseId, listingId, msg.sender, price);
    }
    
    /**
     * @notice Renew subscription
     * @param purchaseId Purchase ID
     */
    function renewSubscription(uint256 purchaseId) external nonReentrant whenNotPaused {
        Purchase storage purchase = purchases[purchaseId];
        require(purchase.buyer == msg.sender, "Not buyer");
        require(purchase.expiresAt != uint256(-1), "Not subscription");
        
        Listing storage listing = listings[purchase.listingId];
        
        // Transfer payment
        paymentToken.safeTransferFrom(msg.sender, listing.seller, listing.price);
        
        // Extend subscription
        purchase.expiresAt += listing.subscriptionDuration;
        
        emit SubscriptionRenewed(purchaseId, purchase.expiresAt);
    }
    
    /**
     * @notice Rate a bot
     * @param botId Bot ID
     * @param rating Rating (1-5)
     */
    function rateBot(uint256 botId, uint256 rating) external whenNotPaused {
        require(rating >= 1 && rating <= 5, "Invalid rating");
        
        botRatings[botId].push(rating);
        botRatingSums[botId] += rating;
        
        emit RatingSubmitted(botId, msg.sender, rating);
    }
    
    /**
     * @notice Withdraw platform fees
     */
    function withdrawPlatformFees() external onlyRole(DEFAULT_ADMIN_ROLE) {
        uint256 amount = platformFees;
        require(amount > 0, "No fees");
        
        platformFees = 0;
        paymentToken.safeTransfer(msg.sender, amount);
        
        emit PlatformFeeWithdrawn(amount);
    }
    
    /**
     * @notice Get bot average rating
     * @param botId Bot ID
     * @return Average rating
     */
    function getBotRating(uint256 botId) external view returns (uint256) {
        uint256[] storage ratings = botRatings[botId];
        if (ratings.length == 0) return 0;
        return botRatingSums[botId] / ratings.length;
    }
    
    /**
     * @notice Get creator bots
     * @param creator Creator address
     * @return Array of bot IDs
     */
    function getCreatorBots(address creator) external view returns (uint256[] memory) {
        return creatorBots[creator];
    }
    
    /**
     * @notice Get bot listings
     * @param botId Bot ID
     * @return Array of listing IDs
     */
    function getBotListings(uint256 botId) external view returns (uint256[] memory) {
        return botListings[botId];
    }
    
    /**
     * @notice Get user purchases
     * @param user User address
     * @return Array of purchase IDs
     */
    function getUserPurchases(address user) external view returns (uint256[] memory) {
        return userPurchases[user];
    }
    
    /**
     * @notice Pause/unpause
     */
    function setPaused(bool _paused) external onlyRole(DEFAULT_ADMIN_ROLE) {
        paused = _paused;
    }
}