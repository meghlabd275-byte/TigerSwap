// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/**
 * @title TigerSwapSeiChain
 * @notice Sei Chain Integration
 */
contract TigerSwapSeiChain {
    uint256 public constant CHAIN_ID = 1329;
    address public constant SEI = 0x0c5E02e47c69F82AD800c7B7D22aD9732DE10e57;
    
    mapping(address => uint256) public balances;
    mapping(address => mapping(address => uint256)) public allowances;
    
    string public name = "Sei";
    string public symbol = "SEI";
    uint8 public decimals = 18;
    uint256 public totalSupply;
    
    event Transfer(address indexed from, address indexed to, uint256 value);
    event Approval(address indexed owner, address indexed spender, uint256 value);
    
    constructor() {
        totalSupply = 1000000000 * 10**18;
        balances[msg.sender] = totalSupply;
    }
    
    function balanceOf(address account) external view returns (uint256) {
        return balances[account];
    }
    
    function transfer(address to, uint256 amount) external returns (bool) {
        require(balances[msg.sender] >= amount);
        balances[msg.sender] -= amount;
        balances[to] += amount;
        emit Transfer(msg.sender, to, amount);
        return true;
    }
}

contract TigerSwapSeiRouter {
    uint256 public constant CHAIN_ID = 1329;
    address public factory;
    address public WSEI = 0x0c5E02e47c69F82AD800c7B7D22aD9732DE10e57;
    
    function swapExactETHForTokens(uint256 amountOutMin, address[] calldata path, address to, uint256 deadline)
        external payable returns (uint256[] memory amounts) {
        amounts = new uint256[](path.length);
        amounts[path.length - 1] = msg.value;
    }
}