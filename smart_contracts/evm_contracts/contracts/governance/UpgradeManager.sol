// SPDX-License-Identifier: MIT
pragma solidity ^0.8.19;

/**
 * @title UpgradeManager
 * @notice Manages contract upgrades
 */
contract UpgradeManager {
    struct Upgrade {
        bytes32 id;
        address implementation;
        uint256 votes;
        uint256 startTime;
        bool executed;
        bool cancelled;
    }
    
    mapping(bytes32 => Upgrade) public upgrades;
    mapping(bytes32 => mapping(address => bool)) public voters;
    
    uint256 public quorum;
    uint256 public votingPeriod;
    
    event UpgradeProposed(bytes32 indexed id, address implementation);
    event UpgradeVoted(bytes32 indexed id, address voter);
    event UpgradeExecuted(bytes32 indexed id);
    
    constructor() {
        quorum = 3;
        votingPeriod = 86400;
    }
    
    function proposeUpgrade(address implementation) external returns (bytes32) {
        bytes32 id = keccak256(abi.encode(implementation, block.timestamp));
        
        upgrades[id] = Upgrade({
            id: id,
            implementation: implementation,
            votes: 0,
            startTime: block.timestamp,
            executed: false,
            cancelled: false
        });
        
        emit UpgradeProposed(id, implementation);
        return id;
    }
    
    function vote(bytes32 upgradeId) external {
        Upgrade storage upgrade = upgrades[upgradeId];
        require(upgrade.startTime > 0, "Not found");
        require(!upgrade.executed, "Executed");
        require(!upgrade.cancelled, "Cancelled");
        require(!voters[upgradeId][msg.sender], "Already voted");
        
        voters[upgradeId][msg.sender] = true;
        upgrade.votes++;
        
        emit UpgradeVoted(upgradeId, msg.sender);
    }
    
    function executeUpgrade(bytes32 upgradeId) external {
        Upgrade storage upgrade = upgrades[upgradeId];
        require(upgrade.startTime > 0, "Not found");
        require(!upgrade.executed, "Already executed");
        require(block.timestamp >= upgrade.startTime + votingPeriod, "Voting ongoing");
        require(upgrade.votes >= quorum, "No quorum");
        
        upgrade.executed = true;
        
        emit UpgradeExecuted(upgradeId);
    }
    
    function setQuorum(uint256 _quorum) external {
        quorum = _quorum;
    }
}