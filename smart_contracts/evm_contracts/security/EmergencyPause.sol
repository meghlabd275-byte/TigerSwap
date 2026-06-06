// SPDX-License-Identifier: MIT
pragma solidity ^0.8.19;

// ============================================================================
// TIGERSWAP EMERGENCY PAUSE SYSTEM
// Emergency pause for the entire protocol
// ============================================================================

import "@openzeppelin/contracts/access/AccessControl.sol";
import "@openzeppelin/contracts/security/Pausable.sol";

/// @title TigerEmergencyPause
/// @notice Emergency pause system for TigerSwap protocol
contract TigerEmergencyPause is AccessControl, Pauser {

    // ============================================================================
    // Roles
    // ============================================================================
    
    bytes32 public constant EMERGENCY_ADMIN_ROLE = keccak256("EMERGENCY_ADMIN_ROLE");
    bytes32 public constant PAUSE_ADMIN_ROLE = keccak256("PAUSE_ADMIN_ROLE");
    
    // ============================================================================
    // State Variables
    // ============================================================================
    
    // Pause reason tracking
    mapping(bytes32 => bool) public pausedFunctions;
    
    // Guardian multisig (3-of-5)
    address[5] public guardians;
    uint256 public guardianCount;
    mapping(address => bool) public isGuardian;
    mapping(uint256 => mapping(address => bool)) public guardianVoted;
    uint256 public emergencyProposalId;
    uint256 public guardianVoteCount;
    uint256 public constant GUARDIAN_THRESHOLD = 3;
    
    // Auto-unpause delay (seconds)
    uint256 public autoUnpauseDelay = 86400; // 24 hours
    uint256 public emergencyPauseTime;
    
    // ============================================================================
    // Events
    // ============================================================================
    
    event EmergencyPauseInitiated(address indexed initiator, string reason);
    event EmergencyPauseExecuted(address indexed executor, string reason);
    event EmergencyUnpauseExecuted(address indexed executor);
    event GuardianAdded(address indexed guardian);
    event GuardianRemoved(address indexed guardian);
    event FunctionPaused(bytes32 indexed functionId, string reason);
    event FunctionUnpaused(bytes32 indexed functionId);
    
    // ============================================================================
    // Modifiers
    // ============================================================================
    
    modifier onlyGuardian() {
        require(isGuardian[msg.sender], "Not guardian");
        _;
    }
    
    modifier onlyEmergencyAdmin() {
        require(hasRole(EMERGENCY_ADMIN_ROLE, msg.sender), "Not emergency admin");
        _;
    }
    
    // ============================================================================
    // Constructor
    // ============================================================================
    
    constructor(address[] memory _guardians) {
        require(_guardians.length >= 3, "Need at least 3 guardians");
        
        for (uint i = 0; i < _guardians.length && i < 5; i++) {
            guardians[i] = _guardians[i];
            isGuardian[_guardians[i]] = true;
            guardianCount++;
        }
        
        _grantRole(DEFAULT_ADMIN_ROLE, msg.sender);
        _grantRole(EMERGENCY_ADMIN_ROLE, msg.sender);
        _grantRole(PAUSE_ADMIN_ROLE, msg.sender);
    }
    
    // ============================================================================
    // Emergency Functions
    // ============================================================================
    
    /// @notice Initiate emergency pause (requires guardian approval)
    function initiateEmergencyPause(string calldata _reason) external onlyGuardian {
        emergencyProposalId++;
        guardianVoteCount = 0;
        
        emit EmergencyPauseInitiated(msg.sender, _reason);
    }
    
    /// @notice Vote for emergency pause
    function voteEmergencyPause() external onlyGuardian {
        require(!guardianVoted[emergencyProposalId][msg.sender], "Already voted");
        
        guardianVoted[emergencyProposalId][msg.sender] = true;
        guardianVoteCount++;
        
        if (guardianVoteCount >= GUARDIAN_THRESHOLD) {
            _pause();
            emergencyPauseTime = block.timestamp;
            emit EmergencyPauseExecuted(msg.sender, "Guardian threshold reached");
        }
    }
    
    /// @notice Execute emergency pause directly
    function emergencyPause(string calldata _reason) external onlyEmergencyAdmin {
        _pause();
        emergencyPauseTime = block.timestamp;
        emit EmergencyPauseExecuted(msg.sender, _reason);
    }
    
    /// @notice Unpause the protocol
    function emergencyUnpause() external onlyEmergencyAdmin {
        _unpause();
        emergencyPauseTime = 0;
        emit EmergencyUnpauseExecuted(msg.sender);
    }
    
    /// @notice Auto-unpause after delay
    function autoUnpause() external {
        require(paused(), "Not paused");
        require(block.timestamp >= emergencyPauseTime + autoUnpauseDelay, "Too early");
        _unpause();
        emergencyPauseTime = 0;
        emit EmergencyUnpauseExecuted(msg.sender);
    }
    
    // ============================================================================
    // Function-level Pausing
    // ============================================================================
    
    function pauseFunction(bytes32 _functionId, string calldata _reason) 
        external 
        onlyRole(PAUSE_ADMIN_ROLE) 
    {
        pausedFunctions[_functionId] = true;
        emit FunctionPaused(_functionId, _reason);
    }
    
    function unpauseFunction(bytes32 _functionId) external onlyRole(PAUSE_ADMIN_ROLE) {
        pausedFunctions[_functionId] = false;
        emit FunctionUnpaused(_functionId);
    }
    
    function isFunctionPaused(bytes32 _functionId) external view returns (bool) {
        return pausedFunctions[_functionId];
    }
    
    // ============================================================================
    // Guardian Management
    // ============================================================================
    
    function addGuardian(address _guardian) external onlyRole(DEFAULT_ADMIN_ROLE) {
        require(!isGuardian[_guardian], "Already guardian");
        require(guardianCount < 5, "Max guardians");
        
        isGuardian[_guardian] = true;
        guardians[guardianCount] = _guardian;
        guardianCount++;
        
        emit GuardianAdded(_guardian);
    }
    
    function removeGuardian(address _guardian) external onlyRole(DEFAULT_ADMIN_ROLE) {
        require(isGuardian[_guardian], "Not guardian");
        require(guardianCount > 3, "Min 3 guardians");
        
        isGuardian[_guardian] = false;
        guardianCount--;
        
        emit GuardianRemoved(_guardian);
    }
    
    // ============================================================================
    // Settings
    // ============================================================================
    
    function setAutoUnpauseDelay(uint256 _delay) external onlyRole(DEFAULT_ADMIN_ROLE) {
        autoUnpauseDelay = _delay;
    }
    
    // ============================================================================
    // View Functions
    // ============================================================================
    
    function getGuardianCount() external view returns (uint256) {
        return guardianCount;
    }
    
    function getGuardians() external view returns (address[5] memory) {
        return guardians;
    }
}