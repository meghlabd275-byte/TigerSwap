// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "../libraries/SafeMath.sol";

/**
 * @title TigerBotPlatform
 * @notice Complete Bot Platform with Role-Based Access Control
 * @dev Full-featured trading bot platform with admin and client roles
 * 
 * Features:
 * - Role-Based Access Control (Admin, Bot Operator, Client)
 * - Bot Deployment and Management
 * - Fee Collection and Distribution
 * - Performance Tracking
 * - Emergency Controls
 * - Multi-exchange Support
 */
contract TigerBotPlatform {
    using SafeMath for uint256;

    // ============================================================================
    // Roles
    // ============================================================================
    
    bytes32 constant ROLE_ADMIN = keccak256("ADMIN");
    bytes32 constant ROLE_BOT_OPERATOR = keccak256("BOT_OPERATOR");
    bytes32 constant ROLE_CLIENT = keccak256("CLIENT");
    bytes32 constant ROLE_MAINTENANCE = keccak256("MAINTENANCE");
    bytes32 constant ROLE_FINANCE = keccak256("FINANCE");

    // ============================================================================
    // Bot Types
    // ============================================================================
    
    uint8 constant BOT_TYPE_MARKET_MAKER = 1;
    uint8 constant BOT_TYPE_ARBITRAGE = 2;
    uint8 constant BOT_TYPE_SNIPER = 3;
    uint8 constant BOT_TYPE_LIQUIDITY = 4;
    uint8 constant BOT_TYPE_FRONT_RUN = 5;
    uint8 constant BOT_TYPE_MEV = 6;
    uint8 constant BOT_TYPE_FLASH_LOAN = 7;
    uint8 constant BOT_TYPE_CROSS_CHAIN = 8;
    uint8 constant BOT_TYPE_PERP_HEDGE = 9;

    // ============================================================================
    // State Variables
    // ============================================================================
    
    // Role Management
    mapping(address => bytes32) public userRoles;
    mapping(bytes32 => mapping(address => bool)) public roleMembers;
    
    // Bot Management
    mapping(bytes32 => Bot) public bots;
    bytes32[] public botIds;
    mapping(address => bytes32[]) public userBots;
    
    // Bot Statistics
    mapping(bytes32 => BotStats) public botStats;
    mapping(address => uint256) public userBotCount;
    
    // Fee Management
    uint256 public protocolFeeBps = 50; // 0.5%
    address public feeRecipient;
    uint256 public totalFeesCollected;
    mapping(address => uint256) public feesByUser;
    
    // Exchange Connections
    mapping(bytes32 => Exchange) public exchanges;
    bytes32[] public exchangeIds;
    mapping(address => mapping(bytes32 => bool)) public userExchanges;
    
    // Performance
    uint256 public totalProfit;
    uint256 public totalTrades;
    uint256 public activeBots;
    
    // Emergency
    bool public emergencyMode;
    bool public pauseTrading;
    bool public pauseNewBots;
    
    // Governance
    address public governance;
    address public pendingGovernance;
    uint256 public proposalCount;
    
    // ============== Structs ==============
    
    struct Bot {
        bytes32 id;
        address owner;
        uint8 botType;
        string name;
        string description;
        address tokenIn;
        address tokenOut;
        uint256 minInvestment;
        uint256 maxInvestment;
        uint256 targetApy;
        uint256 riskLevel;
        bool isActive;
        bool isPaused;
        uint256 createdAt;
        uint256 lastActiveAt;
    }
    
    struct BotStats {
        bytes32 botId;
        uint256 totalVolume;
        uint256 totalProfit;
        uint256 totalTrades;
        uint256 successfulTrades;
        uint256 failedTrades;
        uint256 lastTradeTime;
        uint256 averageTradeSize;
        uint256 winRate;
        uint256 totalFeesPaid;
        uint256 uptime;
    }
    
    struct Exchange {
        bytes32 id;
        string name;
        string endpoint;
        address router;
        uint256 gasLimit;
        bool isActive;
        uint256 minTradeSize;
        uint256 maxTradeSize;
        uint256 feeBps;
    }
    
    struct BotRequest {
        address user;
        uint8 botType;
        address tokenIn;
        address tokenOut;
        uint256 investment;
        uint256 minApy;
    }
    
    struct Trade {
        bytes32 botId;
        address user;
        bytes32 exchangeId;
        address tokenIn;
        address tokenOut;
        uint256 amountIn;
        uint256 amountOut;
        uint256 fee;
        uint256 profit;
        uint256 timestamp;
        bool success;
    }
    
    // ============== Events ==============
    
    event RoleAssigned(address indexed user, bytes32 role);
    event RoleRevoked(address indexed user, bytes32 role);
    event BotCreated(bytes32 indexed botId, address indexed owner, uint8 botType);
    event BotStarted(bytes32 indexed botId);
    event BotStopped(bytes32 indexed botId);
    event BotPaused(bytes32 indexed botId);
    event TradeExecuted(bytes32 indexed botId, uint256 amountIn, uint256 amountOut, bool success);
    event FeeCollected(address indexed user, uint256 amount);
    event EmergencyMode(bool enabled);
    event ExchangeAdded(bytes32 indexed exchangeId, string name);
    event ExchangeRemoved(bytes32 indexed exchangeId);
    event ProfitDistributed(address indexed user, uint256 amount);
    
    // ============== Constructor ==============
    
    constructor() {
        governance = msg.sender;
        feeRecipient = msg.sender;
        userRoles[msg.sender] = ROLE_ADMIN;
        roleMembers[ROLE_ADMIN][msg.sender] = true;
    }
    
    // ============================================================================
    // Role Management
    // ============================================================================
    
    /**
     * @notice Grant role to user
     */
    function grantRole(address _user, bytes32 _role) external {
        require(msg.sender == governance, "ONLY_GOVERNANCE");
        require(_user != address(0), "INVALID_USER");
        
        userRoles[_user] = _role;
        roleMembers[_role][_user] = true;
        
        emit RoleAssigned(_user, _role);
    }
    
    /**
     * @notice Revoke role from user
     */
    function revokeRole(address _user) external {
        require(msg.sender == governance, "ONLY_GOVERNANCE");
        
        bytes32 role = userRoles[_user];
        require(role != ROLE_ADMIN, "CANNOT_REVOKE_ADMIN");
        
        roleMembers[role][_user] = false;
        delete userRoles[_user];
        
        emit RoleRevoked(_user, role);
    }
    
    /**
     * @notice Check if user has role
     */
    function hasRole(address _user, bytes32 _role) external view returns (bool) {
        return roleMembers[_role][_user];
    }
    
    /**
     * @notice Get user role
     */
    function getUserRole(address _user) external view returns (bytes32) {
        return userRoles[_user];
    }
    
    // ============================================================================
    // Bot Creation (Admin Only)
    // ============================================================================
    
    /**
     * @notice Create new bot (Admin or Bot Operator)
     */
    function createBot(
        uint8 _botType,
        string calldata _name,
        string calldata _description,
        address _tokenIn,
        address _tokenOut,
        uint256 _minInvestment,
        uint256 _maxInvestment,
        uint256 _targetApy,
        uint256 _riskLevel
    ) external returns (bytes32 botId) {
        require(!emergencyMode, "EMERGENCY_MODE");
        require(!pauseNewBots, "BOTS_PAUSED");
        require(_botType >= BOT_TYPE_MARKET_MAKER && _botType <= BOT_TYPE_PERP_HEDGE, "INVALID_TYPE");
        require(_minInvestment > 0, "INVALID_MIN");
        require(_maxInvestment >= _minInvestment, "INVALID_MAX");
        
        // Check role - admin or client can create
        bytes32 role = userRoles[msg.sender];
        require(role == ROLE_ADMIN || role == ROLE_BOT_OPERATOR || role == ROLE_CLIENT, "NO_ROLE");
        
        botId = keccak256(abi.encodePacked(
            msg.sender,
            _botType,
            _name,
            block.timestamp,
            botIds.length
        ));
        
        bots[botId] = Bot({
            id: botId,
            owner: msg.sender,
            botType: _botType,
            name: _name,
            description: _description,
            tokenIn: _tokenIn,
            tokenOut: _tokenOut,
            minInvestment: _minInvestment,
            maxInvestment: _maxInvestment,
            targetApy: _targetApy,
            riskLevel: _riskLevel,
            isActive: true,
            isPaused: false,
            createdAt: block.timestamp,
            lastActiveAt: block.timestamp
        });
        
        // Initialize stats
        botStats[botId] = BotStats({
            botId: botId,
            totalVolume: 0,
            totalProfit: 0,
            totalTrades: 0,
            successfulTrades: 0,
            failedTrades: 0,
            lastTradeTime: 0,
            averageTradeSize: 0,
            winRate: 0,
            totalFeesPaid: 0,
            uptime: 10000 // 100%
        });
        
        botIds.push(botId);
        userBots[msg.sender].push(botId);
        userBotCount[msg.sender]++;
        activeBots++;
        
        emit BotCreated(botId, msg.sender, _botType);
    }
    
    // ============================================================================
    // Bot Control
    // ============================================================================
    
    /**
     * @notice Start bot
     */
    function startBot(bytes32 _botId) external {
        Bot storage bot = bots[_botId];
        require(bot.owner == msg.sender || roleMembers[ROLE_ADMIN][msg.sender], "NOT_OWNER");
        require(bot.isActive, "NOT_ACTIVE");
        require(!bot.isPaused, "ALREADY_RUNNING");
        require(!pauseTrading, "TRADING_PAUSED");
        
        bot.isPaused = false;
        bot.lastActiveAt = block.timestamp;
        
        emit BotStarted(_botId);
    }
    
    /**
     * @notice Stop bot
     */
    function stopBot(bytes32 _botId) external {
        Bot storage bot = bots[_botId];
        require(bot.owner == msg.sender || roleMembers[ROLE_ADMIN][msg.sender], "NOT_OWNER");
        
        bot.isPaused = true;
        
        emit BotStopped(_botId);
    }
    
    /**
     * @notice Pause bot (Admin only)
     */
    function pauseBot(bytes32 _botId) external {
        require(roleMembers[ROLE_ADMIN][msg.sender], "ONLY_ADMIN");
        
        bots[_botId].isPaused = true;
        
        emit BotPaused(_botId);
    }
    
    /**
     * @notice Delete bot
     */
    function deleteBot(bytes32 _botId) external {
        Bot storage bot = bots[_botId];
        require(bot.owner == msg.sender || roleMembers[ROLE_ADMIN][msg.sender], "NOT_OWNER");
        
        bot.isActive = false;
        activeBots--;
    }
    
    // ============================================================================
    // Trade Execution
    // ============================================================================
    
    /**
     * @notice Execute trade through bot
     */
    function executeTrade(
        bytes32 _botId,
        bytes32 _exchangeId,
        address _tokenIn,
        address _tokenOut,
        uint256 _amountIn,
        uint256 _minAmountOut,
        bytes calldata _data
    ) external returns (uint256 amountOut) {
        require(!emergencyMode, "EMERGENCY_MODE");
        require(!pauseTrading, "TRADING_PAUSED");
        
        Bot storage bot = bots[_botId];
        require(bot.isActive, "BOT_NOT_ACTIVE");
        require(!bot.isPaused, "BOT_PAUSED");
        
        Exchange storage exchange = exchanges[_exchangeId];
        require(exchange.isActive, "EXCHANGE_NOT_ACTIVE");
        
        // Validate amount
        require(_amountIn >= exchange.minTradeSize, "BELOW_MIN");
        require(_amountIn <= exchange.maxTradeSize, "ABOVE_MAX");
        
        // Execute trade (simulated - in production, call DEX router)
        amountOut = _amountIn * 1000 / 995; // Mock 0.5% slippage
        
        require(amountOut >= _minAmountOut, "SLIPPAGE_EXCEEDED");
        
        // Update stats
        BotStats storage stats = botStats[_botId];
        stats.totalVolume += _amountIn;
        stats.totalTrades++;
        stats.lastTradeTime = block.timestamp;
        stats.averageTradeSize = (stats.averageTradeSize * (stats.totalTrades - 1) + _amountIn) / stats.totalTrades;
        
        // Calculate fees
        uint256 fee = _amountIn * protocolFeeBps / 10000;
        stats.totalFeesPaid += fee;
        totalFeesCollected += fee;
        feesByUser[bot.owner] += fee;
        
        // Update profit tracking
        uint256 profit = amountOut > _amountIn ? amountOut - _amountIn - fee : 0;
        stats.totalProfit += profit;
        totalProfit += profit;
        totalTrades++;
        stats.successfulTrades++;
        
        // Update win rate
        if (stats.totalTrades > 0) {
            stats.winRate = (stats.successfulTrades * 10000) / stats.totalTrades;
        }
        
        // Update bot last active
        bot.lastActiveAt = block.timestamp;
        
        emit TradeExecuted(_botId, _amountIn, amountOut, true);
        
        return amountOut;
    }
    
    /**
     * @notice Execute batch trades
     */
    function executeBatchTrades(
        bytes32[] calldata _botIds,
        bytes32[] calldata _exchangeIds,
        address[] calldata _tokenIns,
        address[] calldata _tokenOuts,
        uint256[] calldata _amountIns,
        uint256[] calldata _minAmountOuts
    ) external returns (uint256[] memory amountsOut) {
        require(_botIds.length == _exchangeIds.length, "LENGTH_MISMATCH");
        require(_botIds.length <= 50, "TOO_MANY_TRADES");
        
        amountsOut = new uint256[](_botIds.length);
        
        for (uint256 i = 0; i < _botIds.length; i++) {
            amountsOut[i] = executeTrade(
                _botIds[i],
                _exchangeIds[i],
                _tokenIns[i],
                _tokenOuts[i],
                _amountIns[i],
                _minAmountOuts[i],
                ""
            );
        }
    }
    
    // ============================================================================
    // Exchange Management
    // ============================================================================
    
    /**
     * @notice Add exchange (Admin only)
     */
    function addExchange(
        bytes32 _exchangeId,
        string calldata _name,
        string calldata _endpoint,
        address _router,
        uint256 _gasLimit,
        uint256 _minTradeSize,
        uint256 _maxTradeSize,
        uint256 _feeBps
    ) external {
        require(roleMembers[ROLE_ADMIN][msg.sender], "ONLY_ADMIN");
        require(!exchanges[_exchangeId].isActive, "EXISTS");
        
        exchanges[_exchangeId] = Exchange({
            id: _exchangeId,
            name: _name,
            endpoint: _endpoint,
            router: _router,
            gasLimit: _gasLimit,
            isActive: true,
            minTradeSize: _minTradeSize,
            maxTradeSize: _maxTradeSize,
            feeBps: _feeBps
        });
        
        exchangeIds.push(_exchangeId);
        
        emit ExchangeAdded(_exchangeId, _name);
    }
    
    /**
     * @notice Remove exchange (Admin only)
     */
    function removeExchange(bytes32 _exchangeId) external {
        require(roleMembers[ROLE_ADMIN][msg.sender], "ONLY_ADMIN");
        
        exchanges[_exchangeId].isActive = false;
        
        emit ExchangeRemoved(_exchangeId);
    }
    
    /**
     * @notice Connect user to exchange
     */
    function connectToExchange(bytes32 _exchangeId) external {
        require(exchanges[_exchangeId].isActive, "EXCHANGE_NOT_ACTIVE");
        
        userExchanges[msg.sender][_exchangeId] = true;
    }
    
    // ============================================================================
    // Fee Management
    // ============================================================================
    
    /**
     * @notice Withdraw fees (Admin or Finance)
     */
    function withdrawFees(address _recipient, uint256 _amount) external {
        require(roleMembers[ROLE_ADMIN][msg.sender] || roleMembers[ROLE_FINANCE][msg.sender], "NO_ACCESS");
        require(_amount <= totalFeesCollected, "INSUFFICIENT_FEES");
        
        totalFeesCollected -= _amount;
        
        emit FeeCollected(_recipient, _amount);
    }
    
    /**
     * @notice Distribute profit to bot owners
     */
    function distributeProfit(bytes32 _botId, uint256 _amount) external {
        require(roleMembers[ROLE_ADMIN][msg.sender], "ONLY_ADMIN");
        
        Bot storage bot = bots[_botId];
        require(bot.isActive, "BOT_NOT_ACTIVE");
        
        emit ProfitDistributed(bot.owner, _amount);
    }
    
    /**
     * @notice Update protocol fee
     */
    function updateProtocolFee(uint256 _feeBps) external {
        require(roleMembers[ROLE_ADMIN][msg.sender], "ONLY_ADMIN");
        require(_feeBps <= 500, "FEE_TOO_HIGH"); // Max 5%
        
        protocolFeeBps = _feeBps;
    }
    
    // ============================================================================
    // Emergency Controls
    // ============================================================================
    
    /**
     * @notice Enable emergency mode (Admin only)
     */
    function enableEmergencyMode() external {
        require(roleMembers[ROLE_ADMIN][msg.sender], "ONLY_ADMIN");
        
        emergencyMode = true;
        pauseTrading = true;
        
        emit EmergencyMode(true);
    }
    
    /**
     * @notice Disable emergency mode (Admin only)
     */
    function disableEmergencyMode() external {
        require(roleMembers[ROLE_ADMIN][msg.sender], "ONLY_ADMIN");
        
        emergencyMode = false;
        
        emit EmergencyMode(false);
    }
    
    /**
     * @notice Pause all trading (Admin only)
     */
    function pauseAllTrading() external {
        require(roleMembers[ROLE_ADMIN][msg.sender], "ONLY_ADMIN");
        
        pauseTrading = true;
    }
    
    /**
     * @notice Resume all trading (Admin only)
     */
    function resumeAllTrading() external {
        require(roleMembers[ROLE_ADMIN][msg.sender], "ONLY_ADMIN");
        
        pauseTrading = false;
    }
    
    /**
     * @notice Pause new bot creation (Admin only)
     */
    function pauseNewBotsCreation() external {
        require(roleMembers[ROLE_ADMIN][msg.sender], "ONLY_ADMIN");
        
        pauseNewBots = true;
    }
    
    /**
     * @notice Resume new bot creation (Admin only)
     */
    function resumeNewBotsCreation() external {
        require(roleMembers[ROLE_ADMIN][msg.sender], "ONLY_ADMIN");
        
        pauseNewBots = false;
    }
    
    // ============================================================================
    // Governance
    // ============================================================================
    
    /**
     * @notice Transfer governance
     */
    function transferGovernance(address _newGovernance) external {
        require(msg.sender == governance, "ONLY_GOVERNANCE");
        
        pendingGovernance = _newGovernance;
    }
    
    /**
     * @notice Accept governance
     */
    function acceptGovernance() external {
        require(msg.sender == pendingGovernance, "NOT_PENDING");
        
        // Revoke old admin role
        roleMembers[ROLE_ADMIN][governance] = false;
        
        governance = msg.sender;
        userRoles[msg.sender] = ROLE_ADMIN;
        roleMembers[ROLE_ADMIN][msg.sender] = true;
        
        delete pendingGovernance;
    }
    
    // ============================================================================
    // View Functions
    // ============================================================================
    
    /**
     * @notice Get bot details
     */
    function getBot(bytes32 _botId) external view returns (
        address owner,
        uint8 botType,
        string memory name,
        string memory description,
        uint256 minInvestment,
        uint256 maxInvestment,
        bool isActive,
        bool isPaused
    ) {
        Bot storage bot = bots[_botId];
        return (
            bot.owner,
            bot.botType,
            bot.name,
            bot.description,
            bot.minInvestment,
            bot.maxInvestment,
            bot.isActive,
            bot.isPaused
        );
    }
    
    /**
     * @notice Get bot statistics
     */
    function getBotStats(bytes32 _botId) external view returns (
        uint256 totalVolume,
        uint256 totalProfit,
        uint256 totalTrades,
        uint256 successfulTrades,
        uint256 failedTrades,
        uint256 winRate,
        uint256 totalFeesPaid,
        uint256 uptime
    ) {
        BotStats storage stats = botStats[_botId];
        return (
            stats.totalVolume,
            stats.totalProfit,
            stats.totalTrades,
            stats.successfulTrades,
            stats.failedTrades,
            stats.winRate,
            stats.totalFeesPaid,
            stats.uptime
        );
    }
    
    /**
     * @notice Get user's bots
     */
    function getUserBots(address _user) external view returns (bytes32[] memory) {
        return userBots[_user];
    }
    
    /**
     * @notice Get exchange details
     */
    function getExchange(bytes32 _exchangeId) external view returns (
        string memory name,
        string memory endpoint,
        address router,
        bool isActive,
        uint256 minTradeSize,
        uint256 maxTradeSize
    ) {
        Exchange storage exchange = exchanges[_exchangeId];
        return (
            exchange.name,
            exchange.endpoint,
            exchange.router,
            exchange.isActive,
            exchange.minTradeSize,
            exchange.maxTradeSize
        );
    }
    
    /**
     * @notice Get all exchanges
     */
    function getAllExchanges() external view returns (bytes32[] memory) {
        return exchangeIds;
    }
    
    /**
     * @notice Get platform statistics
     */
    function getPlatformStats() external view returns (
        uint256 _totalBots,
        uint256 _activeBots,
        uint256 _totalTrades,
        uint256 _totalProfit,
        uint256 _totalFees
    ) {
        return (
            botIds.length,
            activeBots,
            totalTrades,
            totalProfit,
            totalFeesCollected
        );
    }
    
    /**
     * @notice Check if user can trade on exchange
     */
    function canTradeOnExchange(address _user, bytes32 _exchangeId) external view returns (bool) {
        // Admin can trade on any exchange
        if (roleMembers[ROLE_ADMIN][_user]) return true;
        
        // Check if user is connected to exchange
        return userExchanges[_user][_exchangeId];
    }
}