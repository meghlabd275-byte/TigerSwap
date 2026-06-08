// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/**
 * @title TigerSwapSonicChain
 * @notice Sonic (Fantom) Chain Integration
 * @dev 
 * - Native wrapper for Sonic EVM
 * - Fast consensus with Lachesis
 * - Low fees and high throughput
 */
contract TigerSwapSonicChain {
    // Chain ID
    uint256 public constant CHAIN_ID = 146;
    
    // Native token
    address public constant S = 0x5f5E9EDa1fa12A17E4Bf3FA4eC71d1b1E1EE1A41;
    
    // State
    mapping(address => uint256) public balances;
    mapping(address => mapping(address => uint256)) public allowances;
    
    // Events
    event Transfer(address indexed from, address indexed to, uint256 value);
    event Approval(address indexed owner, address indexed spender, uint256 value);
    
    // Token info
    string public name = "Sonic";
    string public symbol = "S";
    uint8 public decimals = 18;
    uint256 public totalSupply;
    
    constructor() {
        // Initialize with pre-mined supply for example
        totalSupply = 1000000000 * 10**18;
        balances[msg.sender] = totalSupply;
    }
    
    function balanceOf(address account) external view returns (uint256) {
        return balances[account];
    }
    
    function transfer(address to, uint256 amount) external returns (bool) {
        require(balances[msg.sender] >= amount, "Insufficient balance");
        balances[msg.sender] -= amount;
        balances[to] += amount;
        emit Transfer(msg.sender, to, amount);
        return true;
    }
    
    function approve(address spender, uint256 amount) external returns (bool) {
        allowances[msg.sender][spender] = amount;
        emit Approval(msg.sender, spender, amount);
        return true;
    }
    
    function transferFrom(address from, address to, uint256 amount) external returns (bool) {
        require(balances[from] >= amount, "Insufficient balance");
        require(allowances[from][msg.sender] >= amount, "Insufficient allowance");
        allowances[from][msg.sender] -= amount;
        balances[from] -= amount;
        balances[to] += amount;
        emit Transfer(from, to, amount);
        return true;
    }
    
    // Deposit native
    function deposit() external payable {
        balances[msg.sender] += msg.value;
        totalSupply += msg.value;
        emit Transfer(address(0), msg.sender, msg.value);
    }
    
    // Withdraw native
    function withdraw(uint256 amount) external {
        require(balances[msg.sender] >= amount, "Insufficient balance");
        balances[msg.sender] -= amount;
        totalSupply -= amount;
        payable(msg.sender).transfer(amount);
        emit Transfer(msg.sender, address(0), amount);
    }
    
    receive() external payable {
        deposit();
    }
}

// Sonic-specific DEX Router for TigerSwap
contract TigerSwapSonicRouter {
    uint256 public constant CHAIN_ID = 146;
    
    // Factory
    address public factory;
    
    // WETH (Sonic's wrapped token)
    address public WETH = 0x5f5E9EDa1fa12A17E4Bf3FA4eC71d1b1E1EE1A41;
    
    // Pair
    mapping(address => mapping(address => address)) public getPair;
    address[] public allPairs;
    
    event PairCreated(address indexed token0, address indexed token1, address pair, uint256);
    
    constructor(address _factory) {
        factory = _factory;
    }
    
    function createPair(address tokenA, address tokenB) external returns (address pair) {
        // Simplified pair creation
        require(tokenA != tokenB, "Identical addresses");
        require(getPair[tokenA][tokenB] == address(0), "Pair exists");
        
        // In production, would deploy actual pair contract
        pair = address(new TigerSwapSonicPair());
        getPair[tokenA][tokenB] = pair;
        getPair[tokenB][tokenA] = pair;
        allPairs.push(pair);
        
        emit PairCreated(tokenA, tokenB, pair, allPairs.length);
    }
    
    function swapExactETHForTokens(uint256 amountOutMin, address[] calldata path, address to, uint256 deadline) 
        external payable returns (uint256[] memory amounts) {
        require(path[0] == WETH, "Invalid path");
        amounts = new uint256[](path.length);
        
        // Simplified swap - in production would use actual DEXs
        amounts[path.length - 1] = msg.value;
        
        emit Transfer(msg.sender, to, amounts[path.length - 1]);
    }
}

// Simple pair contract for Sonic
contract TigerSwapSonicPair {
    address public token0;
    address public token1;
    uint112 private reserve0;
    uint112 private reserve1;
    uint32 private blockTimestampLast;
    
    function initialize(address _token0, address _token1) external {
        token0 = _token0;
        token1 = _token1;
    }
}