// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import "@openzeppelin/contracts/access/AccessControl.sol";
import "@openzeppelin/contracts/security/ReentrancyGuard.sol";

/**
 * @title TigerCircuitBreaker
 * @notice Circuit breaker for emergency trading halt
 */

contract TigerCircuitBreaker is AccessControl, ReentrancyGuard {
    bytes32 public constant WARDEN_ROLE = keccak256("WARDEN_ROLE");
    bytes32 public constant OPERATOR_ROLE = keccak256("OPERATOR_ROLE");
    
    // Circuit breaker states
    enum CircuitState { Normal, Tripped, HalfOpen, Recovery }
    
    // Configuration
    uint256 public constant TRIGGER_THRESHOLD = 1e8; // 100% price drop
    uint256 public constant RECOVERY_DELAY = 1 hours;
    uint256 public constant TRIP_COOLDOWN = 15 minutes;
    
    // State
    CircuitState public circuitState;
    uint256 public lastTripTime;
    uint256 public lastResetTime;
    uint256 public consecutiveTrips;
    uint256 public autoResetTime;
    bool public manualOverride;
    
    // Price feeds
    mapping(address => uint256) public priceFeeds;
    mapping(address => uint256) public priceChangePercent;
    mapping(address => bool) public priceFeedsEnabled;
    
    // Events
    event CircuitTripped(string reason);
    event CircuitReset();
    event CircuitHalfOpen();
    event CircuitRecovery();
    event PriceFeedUpdated(address indexed feed, uint256 price, uint256 change);
    event ManualOverride(bool enabled);
    
    modifier onlyOperators() {
        require(hasRole(OPERATOR_ROLE, msg.sender), "Not operator");
        _;
    }
    
    constructor(address _admin) {
        _grantRole(DEFAULT_ADMIN_ROLE, _admin);
        _grantRole(WARDEN_ROLE, _admin);
        _grantRole(OPERATOR_ROLE, _admin);
        
        circuitState = CircuitState.Normal;
    }
    
    /**
     * @notice Update price feed
     * @param feed Price feed address
     * @param price Current price
     * @param change24h 24h change in percent (WAD)
     */
    function updatePriceFeed(
        address feed,
        uint256 price,
        uint256 change24h
    ) external onlyOperators nonReentrant {
        require(price > 0, "Price is 0");
        
        uint256 prevPrice = priceFeeds[feed];
        
        // Calculate change
        if (prevPrice > 0) {
            // Change = ((new - old) * 1e18) / old
            uint256 change = price >= prevPrice 
                ? ((price - prevPrice) * 1e18) / prevPrice
                : ((prevPrice - price) * 1e18) / prevPrice;
            
            priceChangePercent[feed] = change;
        }
        
        priceFeeds[feed] = price;
        
        // Check for circuit break
        if (change24h > TRIGGER_THRESHOLD) {
            _tripCircuit("Price drop exceeded");
        }
        
        emit PriceFeedUpdated(feed, price, change24h);
    }
    
    /**
     * @notice Enable price feed
     * @param feed Price feed address
     */
    function enablePriceFeed(address feed) external onlyRole(DEFAULT_ADMIN_ROLE) {
        priceFeedsEnabled[feed] = true;
    }
    
    /**
     * @notice Disable price feed
     * @param feed Price feed address
     */
    function disablePriceFeed(address feed) external onlyRole(DEFAULT_ADMIN_ROLE) {
        priceFeedsEnabled[feed] = false;
    }
    
    /**
     * @notice Manually trip circuit
     * @param reason Reason for trip
     */
    function tripCircuitManually(string calldata reason) external onlyRole(WARDEN_ROLE) nonReentrant {
        require(circuitState == CircuitState.Normal, "Not normal");
        
        _tripCircuit(reason);
    }
    
    /**
     * @notice Reset circuit manually
     */
    function resetCircuitManually() external onlyRole(DEFAULT_ADMIN_ROLE) nonReentrant {
        require(circuitState != CircuitState.Normal, "Already normal");
        require(!manualOverride, "Manual override active");
        
        _resetCircuit();
    }
    
    /**
     * @notice Set manual override
     * @param enabled Override state
     */
    function setManualOverride(bool enabled) external onlyRole(DEFAULT_ADMIN_ROLE) {
        manualOverride = enabled;
        emit ManualOverride(enabled);
    }
    
    /**
     * @notice Attempt auto-recovery
     */
    function attemptRecovery() external onlyOperators nonReentrant {
        require(circuitState == CircuitState.HalfOpen, "Not half open");
        require(block.timestamp >= autoResetTime, "Too early");
        
        // Allow recovery to normal
        circuitState = CircuitState.Recovery;
        emit CircuitRecovery();
        
        // Auto-reset after delay
        autoResetTime = block.timestamp + RECOVERY_DELAY;
    }
    
    /**
     * @notice Get current state
     * @return Current circuit state
     */
    function getState() external view returns (CircuitState) {
        // Auto-transition from recovery
        if (circuitState == CircuitState.Recovery && block.timestamp >= autoResetTime) {
            return CircuitState.Normal;
        }
        return circuitState;
    }
    
    /**
     * @notice Check if operation is allowed
     * @return True if allowed
     */
    function isOperational() external view returns (bool) {
        if (manualOverride) return true;
        
        if (circuitState == CircuitState.Normal) {
            return true;
        } else if (circuitState == CircuitState.HalfOpen) {
            return block.timestamp >= autoResetTime;
        }
        
        return false;
    }
    
    // Internal functions
    
    function _tripCircuit(string memory reason) internal {
        circuitState = CircuitState.Tripped;
        lastTripTime = block.timestamp;
        consecutiveTrips++;
        
        emit CircuitTripped(reason);
        
        // Auto-reset schedule
        if (consecutiveTrips >= 3) {
            // After 3 trips, longer cooldown
            autoResetTime = block.timestamp + 24 hours;
        } else {
            autoResetTime = block.timestamp + TRIP_COOLDOWN;
        }
    }
    
    function _resetCircuit() internal {
        circuitState = CircuitState.Normal;
        lastResetTime = block.timestamp;
        consecutiveTrips = 0;
        
        emit CircuitReset();
    }
}