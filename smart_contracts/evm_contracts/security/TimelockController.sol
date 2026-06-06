// SPDX-License-Identifier: MIT
pragma solidity ^0.8.19;

// ============================================================================
// TIGERSWAP TIMELOCK CONTROLLER
// Time-delayed execution for governance actions
// ============================================================================

import "@openzeppelin/contracts/access/AccessControl.sol";

/// @title TigerTimelockController
/// @notice Timelock controller for delayed execution
contract TigerTimelockController is AccessControl {

    // ============================================================================
    // Constants
    // ============================================================================
    
    bytes32 public constant PROPOSER_ROLE = keccak256("PROPOSER_ROLE");
    bytes32 public constant EXECUTOR_ROLE = keccak256("EXECUTOR_ROLE");
    bytes32 public constant CANCELLER_ROLE = keccak256("CANCELLER_ROLE");
    
    // ============================================================================
    // State Variables
    // ============================================================================
    
    uint256 public minDelay = 86400; // 24 hours default
    
    struct Call {
        address target;
        uint256 value;
        bytes data;
        bool executed;
        bool cancelled;
    }
    
    mapping(bytes32 => Call) public calls;
    mapping(bytes32 => uint256) public timestamps;
    mapping(bytes32 => bool) public queuedCalls;
    
    // ============================================================================
    // Events
    // ============================================================================
    
    event CallQueued(
        bytes32 indexed id,
        address indexed target,
        uint256 value,
        bytes data,
        uint256 eta
    );
    event CallExecuted(bytes32 indexed id);
    event CallCancelled(bytes32 indexed id);
    event MinDelayUpdated(uint256 oldDelay, uint256 newDelay);
    
    // ============================================================================
    // Constructor
    // ============================================================================
    
    constructor(address[] memory proposers, address[] memory executors) {
        // Grant roles
        _grantRole(DEFAULT_ADMIN_ROLE, msg.sender);
        _grantRole(PROPOSER_ROLE, msg.sender);
        _grantRole(EXECUTOR_ROLE, msg.sender);
        _grantRole(CANCELLER_ROLE, msg.sender);
        
        for (uint i = 0; i < proposers.length; i++) {
            _grantRole(PROPOSER_ROLE, proposers[i]);
        }
        
        for (uint i = 0; i < executors.length; i++) {
            _grantRole(EXECUTOR_ROLE, executors[i]);
        }
    }
    
    // ============================================================================
    // Queue
    // ============================================================================
    
    /// @notice Queue a transaction
    function queue(
        address target,
        uint256 value,
        bytes calldata data,
        bytes32 predecessor,
        bytes32 salt
    ) external onlyRole(PROPOSER_ROLE) returns (bytes32) {
        bytes32 id = keccak256(abi.encode(target, value, data, predecessor, salt));
        uint256 eta = block.timestamp + minDelay;
        
        require(!calls[id].executed, "Already executed");
        require(!calls[id].cancelled, "Cancelled");
        require(!queuedCalls[id], "Already queued");
        
        calls[id] = Call({
            target: target,
            value: value,
            data: data,
            executed: false,
            cancelled: false
        });
        
        timestamps[id] = eta;
        queuedCalls[id] = true;
        
        emit CallQueued(id, target, value, data, eta);
        
        return id;
    }
    
    // ============================================================================
    // Execute
    // ============================================================================
    
    /// @notice Execute a queued transaction
    function execute(
        address target,
        uint256 value,
        bytes calldata data,
        bytes32 predecessor,
        bytes32 salt
    ) external payable onlyRole(EXECUTOR_ROLE) returns (bytes32) {
        bytes32 id = keccak256(abi.encode(target, value, data, predecessor, salt));
        
        require(queuedCalls[id], "Not queued");
        require(!calls[id].cancelled, "Cancelled");
        require(!calls[id].executed, "Already executed");
        require(block.timestamp >= timestamps[id], "Too early");
        
        calls[id].executed = true;
        queuedCalls[id] = false;
        
        (bool success, ) = target.call{value: value}(data);
        require(success, "Execution failed");
        
        emit CallExecuted(id);
        
        return id;
    }
    
    // ============================================================================
    // Cancel
    // ============================================================================
    
    /// @notice Cancel a queued transaction
    function cancel(bytes32 id) external onlyRole(CANCELLER_ROLE) {
        require(queuedCalls[id], "Not queued");
        require(!calls[id].executed, "Already executed");
        
        calls[id].cancelled = true;
        queuedCalls[id] = false;
        
        emit CallCancelled(id);
    }
    
    // ============================================================================
    // Admin
    // ============================================================================
    
    /// @notice Update minimum delay
    function updateMinDelay(uint256 newDelay) external onlyRole(DEFAULT_ADMIN_ROLE) {
        require(newDelay >= 1 hours, "Min delay 1 hour");
        require(newDelay <= 30 days, "Max delay 30 days");
        
        uint256 oldDelay = minDelay;
        minDelay = newDelay;
        
        emit MinDelayUpdated(oldDelay, newDelay);
    }
    
    // ============================================================================
    // View
    // ============================================================================
    
    /// @notice Get call info
    function getCallInfo(bytes32 id) external view returns (
        address target,
        uint256 value,
        bytes memory data,
        bool executed,
        bool cancelled,
        uint256 eta
    ) {
        Call storage call = calls[id];
        return (
            call.target,
            call.value,
            call.data,
            call.executed,
            call.cancelled,
            timestamps[id]
        );
    }
}