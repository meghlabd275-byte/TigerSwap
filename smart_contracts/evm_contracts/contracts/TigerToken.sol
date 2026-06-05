// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import "@openzeppelin/contracts/token/ERC20/extensions/ERC20Burnable.sol";
import "@openzeppelin/contracts/access/Ownable.sol";

/**
 * @title TigerToken
 * @notice TIGER token - Governance and utility token for TigerSwap
 */
contract TigerToken is ERC20, ERC20Burnable, Ownable {
    uint256 public constant MAX_SUPPLY = 1000000000 * 10**18; // 1 billion tokens
    
    mapping(address => bool) public minters;
    
    event MinterAdded(address indexed minter);
    event MinterRemoved(address indexed minter);
    
    constructor() ERC20("TigerSwap", "TIGER") Ownable(msg.sender) {
        // Initial supply: 10% to team, 5% to investors, 5% to marketing/treasury
        uint256 teamAllocation = MAX_SUPPLY * 100 / 1000; // 10%
        uint256 investorAllocation = MAX_SUPPLY * 50 / 1000; // 5%
        uint256 treasuryAllocation = MAX_SUPPLY * 50 / 1000; // 5%
        
        _mint(msg.sender, teamAllocation);
        _mint(0x8626f6940E2eb28930eFb4CeF49B2d1F2C9C1199, investorAllocation); // Example investor address
        _mint(0xdD2FD4581271e230360230F9337D5c0430Bf44C0, treasuryAllocation); // Example treasury address
    }
    
    modifier onlyMinter() {
        require(minters[msg.sender] || msg.sender == owner(), "TigerToken: NOT_MINTER");
        _;
    }
    
    function addMinter(address minter) external onlyOwner {
        minters[minter] = true;
        emit MinterAdded(minter);
    }
    
    function removeMinter(address minter) external onlyOwner {
        minters[minter] = false;
        emit MinterRemoved(minter);
    }
    
    function mint(address to, uint256 amount) external onlyMinter {
        require(totalSupply() + amount <= MAX_SUPPLY, "TigerToken: MAX_SUPPLY_EXCEEDED");
        _mint(to, amount);
    }
    
    // The following functions are overrides required by Solidity.
    function _update(address from, address to, uint256 value) internal override(ERC20) {
        super._update(from, to, value);
    }
}