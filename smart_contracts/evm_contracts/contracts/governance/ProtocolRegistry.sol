// SPDX-License-Identifier: MIT
pragma solidity ^0.8.19;

/**
 * @title ProtocolRegistry
 * @notice Registry for protocol contracts and parameters
 */
contract ProtocolRegistry {
    struct ContractInfo {
        address implementation;
        uint256 version;
        uint256 deployedAt;
        bool active;
    }
    
    mapping(bytes32 => ContractInfo) public contracts;
    mapping(address => bool) public admins;
    
    event ContractRegistered(bytes32 indexed id, address implementation, uint256 version);
    event ContractUpdated(bytes32 indexed id, address implementation, uint256 version);
    event ContractDeprecated(bytes32 indexed id);
    
    modifier onlyAdmin() {
        require(admins[msg.sender], "Not admin");
        _;
    }
    
    constructor() {
        admins[msg.sender] = true;
    }
    
    function registerContract(bytes32 id, address implementation) external onlyAdmin {
        require(implementation != address(0), "Invalid implementation");
        
        uint256 version = contracts[id].version + 1;
        
        contracts[id] = ContractInfo({
            implementation: implementation,
            version: version,
            deployedAt: block.timestamp,
            active: true
        });
        
        emit ContractRegistered(id, implementation, version);
    }
    
    function updateContract(bytes32 id, address implementation) external onlyAdmin {
        require(contracts[id].implementation != address(0), "Not registered");
        require(implementation != address(0), "Invalid implementation");
        
        contracts[id].implementation = implementation;
        contracts[id].version++;
        
        emit ContractUpdated(id, implementation, contracts[id].version);
    }
    
    function deprecateContract(bytes32 id) external onlyAdmin {
        require(contracts[id].implementation != address(0), "Not registered");
        
        contracts[id].active = false;
        
        emit ContractDeprecated(id);
    }
    
    function getContract(bytes32 id) external view returns (address, uint256, bool) {
        ContractInfo memory info = contracts[id];
        return (info.implementation, info.version, info.active);
    }
    
    function setAdmin(address admin, bool status) external onlyAdmin {
        admins[admin] = status;
    }
}