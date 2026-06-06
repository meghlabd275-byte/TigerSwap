// SPDX-License-Identifier: MIT
pragma solidity ^0.8.19;

/**
 * @title EmergencyPause
 * @notice Emergency pause functionality for the protocol
 */
contract EmergencyPause {
    bool public paused;
    bool public emergencyActive;
    
    mapping(bytes32 => uint256) public pauseDelays;
    mapping(bytes32 => uint256) public pauseTimestamps;
    
    event Paused(string reason);
    event Unpaused();
    EmergencyActivated(string reason);
    
    modifier whenNotPaused() {
        require(!paused, "Paused");
        _;
    }
    
    modifier whenPaused() {
        require(paused, "Not paused");
        _;
    }
    
    function pause(string memory reason) external {
        paused = true;
        emit Paused(reason);
    }
    
    function unpause() external {
        require(!emergencyActive, "Emergency active");
        paused = false;
        emit Unpaused();
    }
    
    function activateEmergency(string memory reason) external {
        emergencyActive = true;
        paused = true;
        emit emergencyActivated(reason);
    }
    
    function deactivateEmergency() external {
        emergencyActive = false;
    }
    
    function setPauseDelay(bytes32 functionId, uint256 delay) external {
        pauseDelays[functionId] = delay;
    }
    
    function canExecute(bytes32 functionId) external view returns (bool) {
        if (paused && !emergencyActive) return false;
        if (pauseTimestamps[functionId] == 0) return true;
        return block.timestamp >= pauseTimestamps[functionId] + pauseDelays[functionId];
    }
}