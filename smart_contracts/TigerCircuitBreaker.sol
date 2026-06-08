// SPDX-License-Identifier: MIT
pragma solidity ^0.8.19;

/**
 * @title TigerCircuitBreaker
 * @dev Emergency circuit breaker for TigerSwap protocol
 * 
 * This contract provides emergency pause functionality for the TigerSwap protocol.
 * It allows the admin to pause/unpause the protocol in case of critical vulnerabilities
 * or unusual market conditions.
 */

import "@openzeppelin/contracts/access/AccessControl.sol";
import "@openzeppelin/contracts/security/Pausable.sol";
import "@openzeppelin/contracts/security/ReentrancyGuard.sol";
import "@openzeppelin/contracts/proxy/Proxy.sol";
import "@openzeppelin/contracts/proxy/ERC1967/ERC1967Upgrade.sol";

/**
 * @dev Circuit Breaker States:
 * - OPERATIONAL: Normal operation, all functions enabled
 * - PAUSED: Protocol paused, only emergency functions enabled  
 * - HALTED: Full halt, requires governance to restart
 * - RECOVERY: Gradual re-enablement, rate limited
 */
enum CircuitState {
    OPERATIONAL,
    PAUSED,
    HALTED,
    RECOVERY
}

/**
 * @dev Alert levels for monitoring
 */
enum AlertLevel {
    INFO,       // Informational - no action needed
    WARNING,    // Warning - monitor closely
    CRITICAL,   // Critical - prepare to pause
    EMERGENCY    // Emergency - protocol should be paused
}

/**
 * @dev Price deviation threshold for circuit breaker
 */
struct PriceDeviation {
    uint256 tokenA;
    uint256 tokenB;
    uint256 lastPriceA;
    uint256 lastPriceB;
    uint256 deviationThreshold;  // Percentage in basis points (e.g., 500 = 5%)
    uint256 lastUpdateTime;
}

/**
 * @dev Alert event structure
 */
struct Alert {
    uint256 id;
    address caller;
    AlertLevel level;
    string description;
    uint256 timestamp;
    bool resolved;
}

/**
 * @title TigerCircuitBreaker
 * @dev Circuit breaker for TigerSwap protocol
 */
contract TigerCircuitBreaker is AccessControl, Pausable, ReentrancyGuard, ERC1967Upgrade {
    
    // ==================== ROLES ====================
    bytes32 public constant ADMIN_ROLE = keccak256("ADMIN_ROLE");
    bytes32 public constant EMERGENCY_ROLE = keccak256("EMERGENCY_ROLE");
    bytes32 public constant MONITOR_ROLE = keccak256("MONITOR_ROLE");
    bytes32 public constant GOVERNANCE_ROLE = keccak256("GOVERNANCE_ROLE");
    
    // ==================== STATE ====================
    CircuitState public circuitState;
    uint256 public lastStateChange;
    address public governance;
    
    // Price deviation tracking
    mapping(bytes32 => PriceDeviation) public priceDeviations;
    bytes32[] public trackedPairs;
    
    // Alert system
    Alert[] public alerts;
    uint256 public alertCount;
    mapping(address => bool) public authorizedAlerters;
    
    // Rate limiting during recovery
    uint256 public recoveryRateLimit;
    uint256 public lastRecoveryAction;
    mapping(address => uint256) public userLastAction;
    
    // Statistics
    uint256 public totalPauses;
    uint256 public lastPauseTime;
    string public pauseReason;
    
    // ==================== EVENTS ====================
    event CircuitStateChanged(
        CircuitState indexed oldState,
        CircuitState indexed newState,
        string reason,
        address indexed actor
    );
    
    event PriceDeviationAlert(
        bytes32 indexed pair,
        uint256 oldPrice,
        uint256 newPrice,
        uint256 deviation
    );
    
    event AlertCreated(
        uint256 indexed alertId,
        AlertLevel indexed level,
        address indexed caller,
        string description
    );
    
    event AlertResolved(
        uint256 indexed alertId,
        address indexed resolver
    );
    
    event RecoveryAction(
        address indexed user,
        uint256 timestamp
    );
    
    // ==================== MODIFIERS ====================
    modifier whenOperational() {
        require(
            circuitState == CircuitState.OPERATIONAL,
            "Circuit: Not operational"
        );
        _;
    }
    
    modifier whenPaused() {
        require(
            circuitState == CircuitState.PAUSED,
            "Circuit: Not paused"
        );
        _;
    }
    
    modifier whenHalted() {
        require(
            circuitState == CircuitState.HALTED,
            "Circuit: Not halted"
        );
        _;
    }
    
    modifier whenRecovering() {
        require(
            circuitState == CircuitState.RECOVERY,
            "Circuit: Not in recovery"
        );
        _;
    }
    
    modifier onlyEmergencyOrAdmin() {
        require(
            hasRole(EMERGENCY_ROLE, msg.sender) || hasRole(ADMIN_ROLE, msg.sender),
            "Circuit: Not authorized"
        );
        _;
    }
    
    modifier rateLimited() {
        // During recovery, limit actions per user
        if (circuitState == CircuitState.RECOVERY) {
            require(
                block.timestamp >= userLastAction[msg.sender] + recoveryRateLimit,
                "Circuit: Rate limited"
            );
        }
        _;
    }
    
    // ==================== CONSTRUCTOR ====================
    constructor() {
        _grantRole(DEFAULT_ADMIN_ROLE, msg.sender);
        _grantRole(ADMIN_ROLE, msg.sender);
        _grantRole(EMERGENCY_ROLE, msg.sender);
        _grantRole(GOVERERNANCE_ROLE, msg.sender);
        
        circuitState = CircuitState.OPERATIONAL;
        lastStateChange = block.timestamp;
        recoveryRateLimit = 1 hours;
        governance = msg.sender;
    }
    
    // ==================== STATE MANAGEMENT ====================
    
    /**
     * @dev Pause the protocol
     * Can only be called by emergency role or admin
     */
    function pause(string calldata reason) external onlyEmergencyOrAdmin whenNotPaused {
        _pause();
        _setCircuitState(CircuitState.PAUSED, reason);
    }
    
    /**
     * @dev Unpause the protocol
     * Can only be called by admin or governance
     */
    function unpause() external onlyRole(ADMIN_ROLE) whenPaused {
        _unpause();
        _setCircuitState(CircuitState.OPERATIONAL, "Resume normal operation");
    }
    
    /**
     * @dev Halt the protocol completely
     * Requires governance role
     */
    function halt(string calldata reason) external onlyRole(GOVERERNANCE_ROLE) {
        _pause();
        _setCircuitState(CircuitState.HALTED, reason);
    }
    
    /**
     * @dev Enter recovery mode
     * Allows gradual restart with rate limiting
     */
    function enterRecovery(uint256 rateLimit) external onlyRole(GOVERERNANCE_ROLE) {
        require(
            circuitState == CircuitState.HALTED,
            "Circuit: Must be halted first"
        );
        
        recoveryRateLimit = rateLimit;
        _setCircuitState(CircuitState.RECOVERY, "Enter recovery mode");
    }
    
    /**
     * @dev Exit recovery mode to operational
     */
    function exitRecovery() external onlyRole(GOVERERNANCE_ROLE) whenRecovering {
        _unpause();
        _setCircuitState(CircuitState.OPERATIONAL, "Recovery complete");
    }
    
    /**
     * @dev Set circuit state internally
     */
    function _setCircuitState(CircuitState newState, string memory reason) internal {
        CircuitState oldState = circuitState;
        circuitState = newState;
        lastStateChange = block.timestamp;
        
        if (newState == CircuitState.PAUSED || newState == CircuitState.HALTED) {
            totalPauses++;
            lastPauseTime = block.timestamp;
            pauseReason = reason;
        }
        
        emit CircuitStateChanged(oldState, newState, reason, msg.sender);
    }
    
    // ==================== PRICE MONITORING ====================
    
    /**
     * @dev Track a trading pair for price deviation
     */
    function trackPair(
        bytes32 pairId,
        uint256 tokenA,
        uint256 tokenB,
        uint256 initialPriceA,
        uint256 initialPriceB,
        uint256 deviationThresholdBps
    ) external onlyRole(ADMIN_ROLE) {
        require(
            priceDeviations[pairId].lastUpdateTime == 0,
            "Circuit: Pair already tracked"
        );
        
        priceDeviations[pairId] = PriceDeviation({
            tokenA: tokenA,
            tokenB: tokenB,
            lastPriceA: initialPriceA,
            lastPriceB: initialPriceB,
            deviationThreshold: deviationThresholdBps,
            lastUpdateTime: block.timestamp
        });
        
        trackedPairs.push(pairId);
    }
    
    /**
     * @dev Update price and check for deviation
     */
    function updatePrice(
        bytes32 pairId,
        uint256 newPriceA,
        uint256 newPriceB
    ) external onlyRole(MONITOR_ROLE) whenNotPaused {
        PriceDeviation storage pd = priceDeviations[pairId];
        require(pd.lastUpdateTime > 0, "Circuit: Pair not tracked");
        
        uint256 deviationA = _calculateDeviation(pd.lastPriceA, newPriceA);
        uint256 deviationB = _calculateDeviation(pd.lastPriceB, newPriceB);
        
        if (deviationA > pd.deviationThreshold || deviationB > pd.deviationThreshold) {
            emit PriceDeviationAlert(
                pairId,
                pd.lastPriceA,
                newPriceA,
                deviationA > deviationB ? deviationA : deviationB
            );
            
            // Auto-pause if deviation is critical (>50%)
            if (deviationA > 5000 || deviationB > 5000) {
                _createAlert(AlertLevel.CRITICAL, "Significant price deviation detected");
            }
        }
        
        pd.lastPriceA = newPriceA;
        pd.lastPriceB = newPriceB;
        pd.lastUpdateTime = block.timestamp;
    }
    
    /**
     * @dev Calculate percentage deviation in basis points
     */
    function _calculateDeviation(uint256 oldPrice, uint256 newPrice) internal pure returns (uint256) {
        if (oldPrice == 0) return 0;
        
        uint256 diff = oldPrice > newPrice ? oldPrice - newPrice : newPrice - oldPrice;
        return (diff * 10000) / oldPrice;
    }
    
    // ==================== ALERT SYSTEM ====================
    
    /**
     * @dev Create an alert
     */
    function createAlert(AlertLevel level, string calldata description) external {
        require(
            authorizedAlerters[msg.sender] || hasRole(MONITOR_ROLE, msg.sender),
            "Circuit: Not authorized"
        );
        
        _createAlert(level, description);
    }
    
    /**
     * @dev Internal alert creation
     */
    function _createAlert(AlertLevel level, string memory description) internal {
        Alert memory alert = Alert({
            id: alertCount,
            caller: msg.sender,
            level: level,
            description: description,
            timestamp: block.timestamp,
            resolved: false
        });
        
        alerts.push(alert);
        emit AlertCreated(alertCount, level, msg.sender, description);
        
        // Auto-create circuit breaker alert for critical/emergency
        if (level >= AlertLevel.CRITICAL) {
            createAlert(AlertLevel.WARNING, "Circuit breaker alert triggered");
        }
        
        alertCount++;
    }
    
    /**
     * @dev Resolve an alert
     */
    function resolveAlert(uint256 alertId) external onlyRole(ADMIN_ROLE) {
        require(alertId < alerts.length, "Circuit: Alert not found");
        require(!alerts[alertId].resolved, "Circuit: Already resolved");
        
        alerts[alertId].resolved = true;
        emit AlertResolved(alertId, msg.sender);
    }
    
    /**
     * @dev Get alert details
     */
    function getAlert(uint256 alertId) external view returns (Alert memory) {
        require(alertId < alerts.length, "Circuit: Alert not found");
        return alerts[alertId];
    }
    
    /**
     * @dev Get active alerts
     */
    function getActiveAlerts() external view returns (Alert[] memory) {
        uint256 count = 0;
        for (uint256 i = 0; i < alerts.length; i++) {
            if (!alerts[i].resolved) count++;
        }
        
        Alert[] memory active = new Alert[](count);
        uint256 idx = 0;
        for (uint256 i = 0; i < alerts.length; i++) {
            if (!alerts[i].resolved) {
                active[idx] = alerts[i];
                idx++;
            }
        }
        
        return active;
    }
    
    /**
     * @dev Authorize an alerter
     */
    function authorizeAlerter(address alerter) external onlyRole(ADMIN_ROLE) {
        authorizedAlerters[alerter] = true;
    }
    
    /**
     * @dev Remove alerter authorization
     */
    function removeAlerter(address alerter) external onlyRole(ADMIN_ROLE) {
        authorizedAlerters[alerter] = false;
    }
    
    // ==================== RECOVERY MODE ====================
    
    /**
     * @dev Execute a rate-limited action in recovery mode
     */
    function executeRecoveryAction() external nonReentrant rateLimited whenRecovering {
        userLastAction[msg.sender] = block.timestamp;
        emit RecoveryAction(msg.sender, block.timestamp);
    }
    
    // ==================== GOVERNANCE ====================
    
    /**
     * @dev Update governance address
     */
    function setGovernance(address newGovernance) external onlyRole(GOVERERNANCE_ROLE) {
        require(newGovernance != address(0), "Circuit: Invalid address");
        
        _revokeRole(GOVERERNANCE_ROLE, governance);
        _grantRole(GOVERVERNANCE_ROLE, newGovernance);
        governance = newGovernance;
    }
    
    /**
     * @dev Transfer admin role
     */
    function transferAdmin(address newAdmin) external onlyRole(DEFAULT_ADMIN_ROLE) {
        require(newAdmin != address(0), "Circuit: Invalid address");
        
        _revokeRole(ADMIN_ROLE, msg.sender);
        _grantRole(ADMIN_ROLE, newAdmin);
    }
    
    // ==================== VIEW FUNCTIONS ====================
    
    /**
     * @dev Get current circuit state
     */
    function getCircuitState() external view returns (CircuitState, uint256, string memory) {
        return (circuitState, lastStateChange, pauseReason);
    }
    
    /**
     * @dev Get all tracked pairs
     */
    function getTrackedPairs() external view returns (bytes32[] memory) {
        return trackedPairs;
    }
    
    /**
     * @dev Get price deviation for a pair
     */
    function getPriceDeviation(bytes32 pairId) external view returns (PriceDeviation memory) {
        return priceDeviations[pairId];
    }
    
    /**
     * @dev Check if protocol is operational
     */
    function isOperational() external view returns (bool) {
        return circuitState == CircuitState.OPERATIONAL && !paused();
    }
}