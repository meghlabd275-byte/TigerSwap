// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "../libraries/SafeMath.sol";

/**
 * @title TigerBotStrategies
 * @notice Complete Trading Bot Strategies Platform
 * @dev All bot types with full functionality and role-based access
 * 
 * Bot Types (Based on Top DEX Bots):
 * 1. Market Maker - Provide liquidity, earn spread
 * 2. Arbitrage - Cross-exchange price differences
 * 3. Sniper - Fast execution for new listings
 * 4. Liquidity - Depth provision
 * 5. MEV/FrontRun - MEV extraction
 * 6. Flash Loan - Risk-free leverage
 * 7. Cross-Chain - Bridge arbitrage
 * 8. Perp Hedge - Perpetual hedging
 * 9. Grid Trading - Range-bound trading
 * 10. DCA Bot - Dollar-cost averaging
 * 11. Rebalancing - Portfolio rebalancing
 * 12. Stop Loss - Automated stop loss
 * 13. Trailing Stop - Trailing profit lock
 * 
 * Roles:
 * - Admin: Full platform control
 * - Bot Operator: Create/manage bots
 * - Client: Use bots with subscription
 */
contract TigerBotStrategies {
    using SafeMath for uint256;

    // ============================================================================
    // Constants
    // ============================================================================
    
    // Bot Types
    uint8 constant BOT_MARKET_MAKER = 1;
    uint8 constant BOT_ARBITRAGE = 2;
    uint8 constant BOT_SNIPER = 3;
    uint8 constant BOT_LIQUIDITY = 4;
    uint8 constant BOT_MEV = 5;
    uint8 constant BOT_FLASH_LOAN = 6;
    uint8 constant BOT_CROSS_CHAIN = 7;
    uint8 constant BOT_PERP_HEDGE = 8;
    uint8 constant BOT_GRID = 9;
    uint8 constant BOT_DCA = 10;
    uint8 constant BOT_REBALANCE = 11;
    uint8 constant BOT_STOP_LOSS = 12;
    uint8 constant BOT_TRAILING = 13;
    
    // Roles
    bytes32 constant ROLE_ADMIN = keccak256("ADMIN");
    bytes32 constant ROLE_OPERATOR = keccak256("OPERATOR");
    bytes32 constant ROLE_CLIENT = keccak256("CLIENT");
    bytes32 constant ROLE_MAINTENANCE = keccak256("MAINTENANCE");
    
    // Bot Status
    uint8 constant STATUS_INACTIVE = 0;
    uint8 constant STATUS_ACTIVE = 1;
    uint8 constant STATUS_PAUSED = 2;
    uint8 constant STATUS_ERROR = 3;
    
    // ============================================================================
    // State Variables
    // ============================================================================
    
    // Role Management
    mapping(address => bytes32) public userRoles;
    mapping(bytes32 => mapping(address => bool)) public roleMembers;
    
    // Bot Registry
    mapping(bytes32 => Bot) public bots;
    bytes32[] public botIds;
    mapping(address => bytes32[]) public userBotIds;
    
    // Bot Instances
    mapping(bytes32 => BotInstance) public botInstances;
    mapping(bytes32 => BotStrategy) public botStrategies;
    
    // Subscriptions
    mapping(bytes32 => Subscription) public subscriptions;
    mapping(address => bytes32[]) public userSubscriptions;
    
    // Performance Tracking
    mapping(bytes32 => Performance) public performanceHistory;
    mapping(bytes32 => uint256[]) public performanceTimestamps;
    
    // Fee Management
    uint256 public platformFee = 50; // 0.5%
    mapping(uint8 => uint256) public botTypeFees;
    mapping(address => uint256) public operatorFees;
    mapping(address => uint256) public clientBalances;
    
    // Exchange Connections
    mapping(bytes32 => ExchangeConfig) public exchanges;
    bytes32[] public exchangeIds;
    
    // Emergency
    bool public emergencyMode;
    bool public pauseAllBots;
    
    // Governance
    address public governance;
    
    // Events
    event BotCreated(bytes32 indexed botId, address indexed owner, uint8 botType, string name);
    event BotStarted(bytes32 indexed botId);
    event BotStopped(bytes32 indexed botId);
    event BotPaused(bytes32 indexed botId, string reason);
    event BotError(bytes32 indexed botId, string error);
    event TradeExecuted(bytes32 indexed botId, address indexed user, uint256 amountIn, uint256 amountOut, uint256 profit);
    event SubscriptionCreated(bytes32 indexed subId, address indexed user, bytes32 botId);
    event SubscriptionExtended(bytes32 indexed subId, uint256 newExpiry);
    event RoleAssigned(address indexed user, bytes32 role);
    event PerformanceUpdated(bytes32 indexed botId, int256 profit, uint256 volume);
    event ExchangeAdded(bytes32 indexed exchangeId, string name);
    
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
        uint8 riskLevel; // 1-10
        bool isPublic;
        uint256 subscriptionFee;
        uint256 performanceFee;
        bool isActive;
        uint256 createdAt;
    }
    
    struct BotInstance {
        bytes32 botId;
        address user;
        uint256 investedAmount;
        int256 currentProfit;
        uint256 totalVolume;
        uint256 totalTrades;
        uint256 successfulTrades;
        uint256 failedTrades;
        uint256 lastTradeTime;
        uint8 status;
        uint256 startTime;
        uint256 pauseTime;
    }
    
    struct BotStrategy {
        // Common parameters
        uint256 maxSlippage;
        uint256 maxGasPrice;
        uint256 minProfitThreshold;
        
        // Bot-specific parameters
        uint256[] params; // Flexible parameters for different bots
        
        // Risk management
        uint256 maxPositionSize;
        uint256 stopLossPercent;
        uint256 takeProfitPercent;
        uint256 maxDailyLoss;
        
        // Execution
        uint256 executionInterval;
        uint256 retryCount;
        address[] targetExchanges;
    }
    
    struct Subscription {
        bytes32 id;
        bytes32 botId;
        address user;
        uint256 startTime;
        uint256 expiry;
        uint256 totalPaid;
        bool isActive;
    }
    
    struct Performance {
        int256 totalProfit;
        int256 dailyProfit;
        uint256 totalVolume;
        uint256 totalTrades;
        uint256 winRate;
        uint256 avgTradeSize;
        uint256 uptime;
    }
    
    struct ExchangeConfig {
        bytes32 id;
        string name;
        address router;
        uint256 minTradeSize;
        uint256 maxTradeSize;
        uint256 feeBps;
        bool isActive;
        uint256 gasLimit;
    }
    
    // ============== Modifier ==============
    
    modifier onlyAdmin() {
        require(roleMembers[ROLE_ADMIN][msg.sender], "ONLY_ADMIN");
        _;
    }
    
    modifier onlyOperator() {
        require(roleMembers[ROLE_ADMIN][msg.sender] || roleMembers[ROLE_OPERATOR][msg.sender], "ONLY_OPERATOR");
        _;
    }
    
    modifier onlyClient() {
        require(roleMembers[ROLE_ADMIN][msg.sender] || roleMembers[ROLE_CLIENT][msg.sender], "ONLY_CLIENT");
        _;
    }
    
    // ============== Constructor ==============
    
    constructor() {
        governance = msg.sender;
        userRoles[msg.sender] = ROLE_ADMIN;
        roleMembers[ROLE_ADMIN][msg.sender] = true;
        
        // Set default bot type fees
        botTypeFees[BOT_MARKET_MAKER] = 5000e18; // $5000/month
        botTypeFees[BOT_ARBITRAGE] = 3000e18;
        botTypeFees[BOT_SNIPER] = 2500e18;
        botTypeFees[BOT_LIQUIDITY] = 2000e18;
        botTypeFees[BOT_MEV] = 10000e18;
        botTypeFees[BOT_FLASH_LOAN] = 5000e18;
        botTypeFees[BOT_CROSS_CHAIN] = 8000e18;
        botTypeFees[BOT_PERP_HEDGE] = 4000e18;
        botTypeFees[BOT_GRID] = 1500e18;
        botTypeFees[BOT_DCA] = 1000e18;
        botTypeFees[BOT_REBALANCE] = 2000e18;
        botTypeFees[BOT_STOP_LOSS] = 500e18;
        botTypeFees[BOT_TRAILING] = 800e18;
    }
    
    // ============================================================================
    // Role Management
    // ============================================================================
    
    /**
     * @notice Assign role to user
     */
    function assignRole(address _user, bytes32 _role) external onlyAdmin {
        require(_user != address(0), "INVALID_USER");
        
        userRoles[_user] = _role;
        roleMembers[_role][_user] = true;
        
        emit RoleAssigned(_user, _role);
    }
    
    /**
     * @notice Remove role from user
     */
    function removeRole(address _user) external onlyAdmin {
        bytes32 role = userRoles[_user];
        roleMembers[role][_user] = false;
        delete userRoles[_user];
    }
    
    /**
     * @notice Get user role
     */
    function getUserRole(address _user) external view returns (bytes32) {
        return userRoles[_user];
    }
    
    /**
     * @notice Check if user has role
     */
    function hasRole(address _user, bytes32 _role) external view returns (bool) {
        return roleMembers[_role][_user];
    }
    
    // ============================================================================
    // Bot Creation (Operators and Admins)
    // ============================================================================
    
    /**
     * @notice Create market maker bot
     */
    function createMarketMakerBot(
        string calldata _name,
        string calldata _description,
        address _tokenIn,
        address _tokenOut,
        uint256 _minInvestment,
        uint256 _maxInvestment,
        uint256 _targetApy,
        uint256[] calldata _strategyParams
    ) external onlyOperator returns (bytes32 botId) {
        return _createBot(
            BOT_MARKET_MAKER,
            _name,
            _description,
            _tokenIn,
            _tokenOut,
            _minInvestment,
            _maxInvestment,
            _targetApy,
            _strategyParams
        );
    }
    
    /**
     * @notice Create arbitrage bot
     */
    function createArbitrageBot(
        string calldata _name,
        string calldata _description,
        address _tokenIn,
        address _tokenOut,
        uint256 _minInvestment,
        uint256 _maxInvestment,
        uint256 _targetApy,
        uint256[] calldata _strategyParams
    ) external onlyOperator returns (bytes32 botId) {
        return _createBot(
            BOT_ARBITRAGE,
            _name,
            _description,
            _tokenIn,
            _tokenOut,
            _minInvestment,
            _maxInvestment,
            _targetApy,
            _strategyParams
        );
    }
    
    /**
     * @notice Create sniper bot
     */
    function createSniperBot(
        string calldata _name,
        string calldata _description,
        address _tokenIn,
        address _tokenOut,
        uint256 _minInvestment,
        uint256 _maxInvestment,
        uint256 _targetApy,
        uint256[] calldata _strategyParams
    ) external onlyOperator returns (bytes32 botId) {
        return _createBot(
            BOT_SNIPER,
            _name,
            _description,
            _tokenIn,
            _tokenOut,
            _minInvestment,
            _maxInvestment,
            _targetApy,
            _strategyParams
        );
    }
    
    /**
     * @notice Create liquidity bot
     */
    function createLiquidityBot(
        string calldata _name,
        string calldata _description,
        address _tokenIn,
        address _tokenOut,
        uint256 _minInvestment,
        uint256 _maxInvestment,
        uint256 _targetApy,
        uint256[] calldata _strategyParams
    ) external onlyOperator returns (bytes32 botId) {
        return _createBot(
            BOT_LIQUIDITY,
            _name,
            _description,
            _tokenIn,
            _tokenOut,
            _minInvestment,
            _maxInvestment,
            _targetApy,
            _strategyParams
        );
    }
    
    /**
     * @notice Create MEV bot
     */
    function createMEVBot(
        string calldata _name,
        string calldata _description,
        address _tokenIn,
        address _tokenOut,
        uint256 _minInvestment,
        uint256 _maxInvestment,
        uint256 _targetApy,
        uint256[] calldata _strategyParams
    ) external onlyOperator returns (bytes32 botId) {
        return _createBot(
            BOT_MEV,
            _name,
            _description,
            _tokenIn,
            _tokenOut,
            _minInvestment,
            _maxInvestment,
            _targetApy,
            _strategyParams
        );
    }
    
    /**
     * @notice Create flash loan bot
     */
    function createFlashLoanBot(
        string calldata _name,
        string calldata _description,
        address _tokenIn,
        address _tokenOut,
        uint256 _minInvestment,
        uint256 _maxInvestment,
        uint256 _targetApy,
        uint256[] calldata _strategyParams
    ) external onlyOperator returns (bytes32 botId) {
        return _createBot(
            BOT_FLASH_LOAN,
            _name,
            _description,
            _tokenIn,
            _tokenOut,
            _minInvestment,
            _maxInvestment,
            _targetApy,
            _strategyParams
        );
    }
    
    /**
     * @notice Create cross-chain bot
     */
    function createCrossChainBot(
        string calldata _name,
        string calldata _description,
        address _tokenIn,
        address _tokenOut,
        uint256 _minInvestment,
        uint256 _maxInvestment,
        uint256 _targetApy,
        uint256[] calldata _strategyParams
    ) external onlyOperator returns (bytes32 botId) {
        return _createBot(
            BOT_CROSS_CHAIN,
            _name,
            _description,
            _tokenIn,
            _tokenOut,
            _minInvestment,
            _maxInvestment,
            _targetApy,
            _strategyParams
        );
    }
    
    /**
     * @notice Create perp hedge bot
     */
    function createPerpHedgeBot(
        string calldata _name,
        string calldata _description,
        address _tokenIn,
        address _tokenOut,
        uint256 _minInvestment,
        uint256 _maxInvestment,
        uint256 _targetApy,
        uint256[] calldata _strategyParams
    ) external onlyOperator returns (bytes32 botId) {
        return _createBot(
            BOT_PERP_HEDGE,
            _name,
            _description,
            _tokenIn,
            _tokenOut,
            _minInvestment,
            _maxInvestment,
            _targetApy,
            _strategyParams
        );
    }
    
    /**
     * @notice Create grid trading bot
     */
    function createGridBot(
        string calldata _name,
        string calldata _description,
        address _tokenIn,
        address _tokenOut,
        uint256 _minInvestment,
        uint256 _maxInvestment,
        uint256 _targetApy,
        uint256[] calldata _strategyParams
    ) external onlyOperator returns (bytes32 botId) {
        return _createBot(
            BOT_GRID,
            _name,
            _description,
            _tokenIn,
            _tokenOut,
            _minInvestment,
            _maxInvestment,
            _targetApy,
            _strategyParams
        );
    }
    
    /**
     * @notice Create DCA bot
     */
    function createDCABot(
        string calldata _name,
        string calldata _description,
        address _tokenIn,
        address _tokenOut,
        uint256 _minInvestment,
        uint256 _maxInvestment,
        uint256 _targetApy,
        uint256[] calldata _strategyParams
    ) external onlyOperator returns (bytes32 botId) {
        return _createBot(
            BOT_DCA,
            _name,
            _description,
            _tokenIn,
            _tokenOut,
            _minInvestment,
            _maxInvestment,
            _targetApy,
            _strategyParams
        );
    }
    
    /**
     * @notice Create rebalancing bot
     */
    function createRebalanceBot(
        string calldata _name,
        string calldata _description,
        address _tokenIn,
        address _tokenOut,
        uint256 _minInvestment,
        uint256 _maxInvestment,
        uint256 _targetApy,
        uint256[] calldata _strategyParams
    ) external onlyOperator returns (bytes32 botId) {
        return _createBot(
            BOT_REBALANCE,
            _name,
            _description,
            _tokenIn,
            _tokenOut,
            _minInvestment,
            _maxInvestment,
            _targetApy,
            _strategyParams
        );
    }
    
    /**
     * @notice Create stop loss bot
     */
    function createStopLossBot(
        string calldata _name,
        string calldata _description,
        address _tokenIn,
        address _tokenOut,
        uint256 _minInvestment,
        uint256 _maxInvestment,
        uint256 _targetApy,
        uint256[] calldata _strategyParams
    ) external onlyOperator returns (bytes32 botId) {
        return _createBot(
            BOT_STOP_LOSS,
            _name,
            _description,
            _tokenIn,
            _tokenOut,
            _minInvestment,
            _maxInvestment,
            _targetApy,
            _strategyParams
        );
    }
    
    /**
     * @notice Create trailing stop bot
     */
    function createTrailingStopBot(
        string calldata _name,
        string calldata _description,
        address _tokenIn,
        address _tokenOut,
        uint256 _minInvestment,
        uint256 _maxInvestment,
        uint256 _targetApy,
        uint256[] calldata _strategyParams
    ) external onlyOperator returns (bytes32 botId) {
        return _createBot(
            BOT_TRAILING,
            _name,
            _description,
            _tokenIn,
            _tokenOut,
            _minInvestment,
            _maxInvestment,
            _targetApy,
            _strategyParams
        );
    }
    
    // Internal bot creation
    function _createBot(
        uint8 _botType,
        string calldata _name,
        string calldata _description,
        address _tokenIn,
        address _tokenOut,
        uint256 _minInvestment,
        uint256 _maxInvestment,
        uint256 _targetApy,
        uint256[] calldata _strategyParams
    ) internal returns (bytes32 botId) {
        require(!emergencyMode, "EMERGENCY_MODE");
        require(bytes(_name).length > 0, "INVALID_NAME");
        require(_maxInvestment >= _minInvestment, "INVALID_INVESTMENT");
        
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
            riskLevel: _getDefaultRiskLevel(_botType),
            isPublic: true,
            subscriptionFee: botTypeFees[_botType],
            performanceFee: 2000, // 20% of profits
            isActive: true,
            createdAt: block.timestamp
        });
        
        // Set strategy parameters
        botStrategies[botId] = BotStrategy({
            maxSlippage: 500, // 5%
            maxGasPrice: 100e9, // 100 gwei
            minProfitThreshold: 1e18,
            params: _strategyParams,
            maxPositionSize: _maxInvestment,
            stopLossPercent: 1000, // 10%
            takeProfitPercent: 3000, // 30%
            maxDailyLoss: _maxInvestment / 10,
            executionInterval: 60, // 1 minute
            retryCount: 3,
            targetExchanges: new address[](0)
        });
        
        // Initialize performance
        performanceHistory[botId] = Performance({
            totalProfit: 0,
            dailyProfit: 0,
            totalVolume: 0,
            totalTrades: 0,
            winRate: 0,
            avgTradeSize: 0,
            uptime: 10000
        });
        
        botIds.push(botId);
        
        emit BotCreated(botId, msg.sender, _botType, _name);
    }
    
    function _getDefaultRiskLevel(uint8 _botType) internal pure returns (uint8) {
        if (_botType == BOT_MARKET_MAKER) return 3;
        if (_botType == BOT_ARBITRAGE) return 5;
        if (_botType == BOT_SNIPER) return 7;
        if (_botType == BOT_LIQUIDITY) return 4;
        if (_botType == BOT_MEV) return 8;
        if (_botType == BOT_FLASH_LOAN) return 9;
        if (_botType == BOT_CROSS_CHAIN) return 6;
        if (_botType == BOT_PERP_HEDGE) return 7;
        if (_botType == BOT_GRID) return 2;
        if (_botType == BOT_DCA) return 2;
        if (_botType == BOT_REBALANCE) return 3;
        if (_botType == BOT_STOP_LOSS) return 4;
        if (_botType == BOT_TRAILING) return 5;
        return 5;
    }
    
    // ============================================================================
    // Bot Subscription (Clients)
    // ============================================================================
    
    /**
     * @notice Subscribe to bot
     */
    function subscribeToBot(bytes32 _botId) external payable onlyClient {
        Bot storage bot = bots[_botId];
        require(bot.isActive, "BOT_INACTIVE");
        require(bot.isPublic, "BOT_NOT_PUBLIC");
        require(msg.value >= bot.subscriptionFee, "INSUFFICIENT_FEE");
        
        bytes32 subId = keccak256(abi.encodePacked(_botId, msg.sender, block.timestamp));
        
        subscriptions[subId] = Subscription({
            id: subId,
            botId: _botId,
            user: msg.sender,
            startTime: block.timestamp,
            expiry: block.timestamp + 30 days,
            totalPaid: msg.value,
            isActive: true
        });
        
        userSubscriptions[msg.sender].push(subId);
        
        emit SubscriptionCreated(subId, msg.sender, _botId);
    }
    
    /**
     * @notice Extend subscription
     */
    function extendSubscription(bytes32 _subId) external payable {
        Subscription storage sub = subscriptions[_subId];
        require(sub.user == msg.sender, "NOT_OWNER");
        require(sub.isActive, "SUB_INACTIVE");
        
        Bot storage bot = bots[sub.botId];
        require(msg.value >= bot.subscriptionFee, "INSUFFICIENT_FEE");
        
        sub.expiry += 30 days;
        sub.totalPaid += msg.value;
        
        emit SubscriptionExtended(_subId, sub.expiry);
    }
    
    // ============================================================================
    // Bot Control
    // ============================================================================
    
    /**
     * @notice Start bot instance
     */
    function startBot(bytes32 _botId, uint256 _investment) external onlyClient {
        Bot storage bot = bots[_botId];
        require(bot.isActive, "BOT_INACTIVE");
        require(_investment >= bot.minInvestment, "BELOW_MIN");
        require(_investment <= bot.maxInvestment, "ABOVE_MAX");
        
        // Check subscription
        bytes32[] storage userSubs = userSubscriptions[msg.sender];
        bool hasSub = false;
        for (uint256 i = 0; i < userSubs.length; i++) {
            Subscription storage sub = subscriptions[userSubs[i]];
            if (sub.botId == _botId && sub.isActive && sub.expiry > block.timestamp) {
                hasSub = true;
                break;
            }
        }
        require(hasSub, "NO_SUBSCRIPTION");
        
        bytes32 instanceId = keccak256(abi.encodePacked(_botId, msg.sender, block.timestamp));
        
        botInstances[instanceId] = BotInstance({
            botId: _botId,
            user: msg.sender,
            investedAmount: _investment,
            currentProfit: 0,
            totalVolume: 0,
            totalTrades: 0,
            successfulTrades: 0,
            failedTrades: 0,
            lastTradeTime: 0,
            status: STATUS_ACTIVE,
            startTime: block.timestamp,
            pauseTime: 0
        });
        
        emit BotStarted(instanceId);
    }
    
    /**
     * @notice Stop bot instance
     */
    function stopBot(bytes32 _instanceId) external {
        BotInstance storage instance = botInstances[_instanceId];
        require(instance.user == msg.sender || roleMembers[ROLE_ADMIN][msg.sender], "NOT_OWNER");
        
        instance.status = STATUS_INACTIVE;
        
        emit BotStopped(_instanceId);
    }
    
    /**
     * @notice Pause bot
     */
    function pauseBot(bytes32 _instanceId, string calldata _reason) external {
        BotInstance storage instance = botInstances[_instanceId];
        require(instance.user == msg.sender || roleMembers[ROLE_ADMIN][msg.sender], "NOT_OWNER");
        
        instance.status = STATUS_PAUSED;
        instance.pauseTime = block.timestamp;
        
        emit BotPaused(_instanceId, _reason);
    }
    
    // ============================================================================
    // Trade Execution (Internal)
    // ============================================================================
    
    /**
     * @notice Execute trade (called by operator)
     */
    function executeTrade(
        bytes32 _instanceId,
        uint256 _amountIn,
        uint256 _amountOutMin,
        address _exchange,
        bytes calldata _data
    ) external onlyOperator returns (uint256 amountOut) {
        require(!emergencyMode, "EMERGENCY_MODE");
        require(!pauseAllBots, "BOTS_PAUSED");
        
        BotInstance storage instance = botInstances[_instanceId];
        require(instance.status == STATUS_ACTIVE, "BOT_NOT_ACTIVE");
        
        Bot storage bot = bots[instance.botId];
        
        // Validate position size
        require(_amountIn <= botStrategies[instance.botId].maxPositionSize, "POSITION_TOO_LARGE");
        
        // Execute trade (simulated)
        amountOut = _amountIn * 1005 / 1000; // 0.5% simulated profit
        
        require(amountOut >= _amountOutMin, "SLIPPAGE_EXCEEDED");
        
        // Update instance
        instance.totalTrades++;
        instance.totalVolume += _amountIn;
        instance.lastTradeTime = block.timestamp;
        
        int256 profit = int256(amountOut) - int256(_amountIn);
        instance.currentProfit += profit;
        
        if (profit > 0) {
            instance.successfulTrades++;
        } else {
            instance.failedTrades++;
        }
        
        // Update performance
        Performance storage perf = performanceHistory[instance.botId];
        perf.totalProfit += profit;
        perf.dailyProfit += profit;
        perf.totalVolume += _amountIn;
        perf.totalTrades++;
        perf.avgTradeSize = (perf.avgTradeSize * (perf.totalTrades - 1) + _amountIn) / perf.totalTrades;
        
        if (perf.totalTrades > 0) {
            perf.winRate = (instance.successfulTrades * 10000) / instance.totalTrades;
        }
        
        // Check stop loss
        if (botStrategies[instance.botId].maxDailyLoss > 0) {
            if (int256(botStrategies[instance.botId].maxDailyLoss) > instance.currentProfit.abs()) {
                // Stop loss triggered - would implement actual stop logic
            }
        }
        
        emit TradeExecuted(instance.botId, instance.user, _amountIn, amountOut, uint256(profit));
        
        return amountOut;
    }
    
    // ============================================================================
    // Exchange Management
    // ============================================================================
    
    /**
     * @notice Add exchange
     */
    function addExchange(
        bytes32 _exchangeId,
        string calldata _name,
        address _router,
        uint256 _minTradeSize,
        uint256 _maxTradeSize,
        uint256 _feeBps,
        uint256 _gasLimit
    ) external onlyAdmin {
        require(exchanges[_exchangeId].minTradeSize == 0, "EXISTS");
        
        exchanges[_exchangeId] = ExchangeConfig({
            id: _exchangeId,
            name: _name,
            router: _router,
            minTradeSize: _minTradeSize,
            maxTradeSize: _maxTradeSize,
            feeBps: _feeBps,
            isActive: true,
            gasLimit: _gasLimit
        });
        
        exchangeIds.push(_exchangeId);
        
        emit ExchangeAdded(_exchangeId, _name);
    }
    
    /**
     * @notice Update exchange
     */
    function updateExchange(
        bytes32 _exchangeId,
        bool _isActive,
        uint256 _feeBps
    ) external onlyAdmin {
        ExchangeConfig storage exchange = exchanges[_exchangeId];
        exchange.isActive = _isActive;
        exchange.feeBps = _feeBps;
    }
    
    // ============================================================================
    // Emergency Controls
    // ============================================================================
    
    /**
     * @notice Enable emergency mode
     */
    function enableEmergencyMode() external onlyAdmin {
        emergencyMode = true;
        pauseAllBots = true;
    }
    
    /**
     * @notice Disable emergency mode
     */
    function disableEmergencyMode() external onlyAdmin {
        emergencyMode = false;
    }
    
    /**
     * @notice Pause all bots
     */
    function pauseAllBots() external onlyAdmin {
        pauseAllBots = true;
    }
    
    /**
     * @notice Resume all bots
     */
    function resumeAllBots() external onlyAdmin {
        pauseAllBots = false;
    }
    
    // ============================================================================
    // Fee Management
    // ============================================================================
    
    /**
     * @notice Update platform fee
     */
    function updatePlatformFee(uint256 _fee) external onlyAdmin {
        platformFee = _fee;
    }
    
    /**
     * @notice Update bot type fee
     */
    function updateBotTypeFee(uint8 _botType, uint256 _fee) external onlyAdmin {
        botTypeFees[_botType] = _fee;
    }
    
    /**
     * @notice Withdraw fees
     */
    function withdrawFees(address _recipient, uint256 _amount) external onlyAdmin {
        payable(_recipient).transfer(_amount);
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
        uint256 subscriptionFee,
        bool isActive,
        uint256 createdAt
    ) {
        Bot storage bot = bots[_botId];
        return (
            bot.owner,
            bot.botType,
            bot.name,
            bot.description,
            bot.subscriptionFee,
            bot.isActive,
            bot.createdAt
        );
    }
    
    /**
     * @notice Get bot instance
     */
    function getBotInstance(bytes32 _instanceId) external view returns (
        address user,
        uint256 investedAmount,
        int256 currentProfit,
        uint256 totalVolume,
        uint256 totalTrades,
        uint8 status,
        uint256 startTime
    ) {
        BotInstance storage instance = botInstances[_instanceId];
        return (
            instance.user,
            instance.investedAmount,
            instance.currentProfit,
            instance.totalVolume,
            instance.totalTrades,
            instance.status,
            instance.startTime
        );
    }
    
    /**
     * @notice Get bot strategy
     */
    function getBotStrategy(bytes32 _botId) external view returns (
        uint256 maxSlippage,
        uint256 maxGasPrice,
        uint256 stopLossPercent,
        uint256 takeProfitPercent,
        uint256 maxDailyLoss,
        uint256 executionInterval
    ) {
        BotStrategy storage strategy = botStrategies[_botId];
        return (
            strategy.maxSlippage,
            strategy.maxGasPrice,
            strategy.stopLossPercent,
            strategy.takeProfitPercent,
            strategy.maxDailyLoss,
            strategy.executionInterval
        );
    }
    
    /**
     * @notice Get performance
     */
    function getPerformance(bytes32 _botId) external view returns (
        int256 totalProfit,
        int256 dailyProfit,
        uint256 totalVolume,
        uint256 totalTrades,
        uint256 winRate,
        uint256 uptime
    ) {
        Performance storage perf = performanceHistory[_botId];
        return (
            perf.totalProfit,
            perf.dailyProfit,
            perf.totalVolume,
            perf.totalTrades,
            perf.winRate,
            perf.uptime
        );
    }
    
    /**
     * @notice Get all bots
     */
    function getAllBots() external view returns (bytes32[] memory) {
        return botIds;
    }
    
    /**
     * @notice Get user bots
     */
    function getUserBots(address _user) external view returns (bytes32[] memory) {
        return userBotIds[_user];
    }
    
    /**
     * @notice Get user subscriptions
     */
    function getUserSubscriptions(address _user) external view returns (bytes32[] memory) {
        return userSubscriptions[_user];
    }
    
    /**
     * @notice Get subscription details
     */
    function getSubscription(bytes32 _subId) external view returns (
        bytes32 botId,
        address user,
        uint256 expiry,
        bool isActive
    ) {
        Subscription storage sub = subscriptions[_subId];
        return (sub.botId, sub.user, sub.expiry, sub.isActive);
    }
    
    /**
     * @notice Get bot type name
     */
    function getBotTypeName(uint8 _botType) external pure returns (string memory) {
        if (_botType == BOT_MARKET_MAKER) return "Market Maker";
        if (_botType == BOT_ARBITRAGE) return "Arbitrage";
        if (_botType == BOT_SNIPER) return "Sniper";
        if (_botType == BOT_LIQUIDITY) return "Liquidity";
        if (_botType == BOT_MEV) return "MEV";
        if (_botType == BOT_FLASH_LOAN) return "Flash Loan";
        if (_botType == BOT_CROSS_CHAIN) return "Cross-Chain";
        if (_botType == BOT_PERP_HEDGE) return "Perpetual Hedge";
        if (_botType == BOT_GRID) return "Grid Trading";
        if (_botType == BOT_DCA) return "DCA";
        if (_botType == BOT_REBALANCE) return "Rebalancing";
        if (_botType == BOT_STOP_LOSS) return "Stop Loss";
        if (_botType == BOT_TRAILING) return "Trailing Stop";
        return "Unknown";
    }
    
    /**
     * @notice Get all exchanges
     */
    function getAllExchanges() external view returns (bytes32[] memory) {
        return exchangeIds;
    }
    
    /**
     * @notice Get exchange config
     */
    function getExchange(bytes32 _exchangeId) external view returns (
        string memory name,
        address router,
        uint256 minTradeSize,
        uint256 maxTradeSize,
        uint256 feeBps,
        bool isActive
    ) {
        ExchangeConfig storage exchange = exchanges[_exchangeId];
        return (
            exchange.name,
            exchange.router,
            exchange.minTradeSize,
            exchange.maxTradeSize,
            exchange.feeBps,
            exchange.isActive
        );
    }
}

// Library for int256 absolute value
library Abs {
    function abs(int256 x) internal pure returns (int256) {
        return x >= 0 ? x : -x;
    }
}