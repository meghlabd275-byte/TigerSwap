// SPDX-License-Identifier: MIT
pragma solidity ^0.8.19;

/**
 * TigerSwap Router Contract
 * 
 * @author TigerSwap
 * @version 1.0.0
 */

/**
 * @title TigerRouter
 * @dev Main router for swaps
 */
contract TigerRouter {
    
    address public factory;
    address public WETH;
    
    modifier ensure(uint256 deadline) {
        require(block.timestamp <= deadline, "TigerRouter: EXPIRED");
        _;
    }
    
    constructor(address _factory, address _WETH) {
        factory = _factory;
        WETH = _WETH;
    }
    
    /**
     * @dev Swap exact tokens for tokens
     */
    function swapExactTokensForTokens(
        uint256 amountIn,
        uint256 amountOutMin,
        address[] calldata path,
        address to,
        uint256 deadline
    ) external ensure(deadline) returns (uint256[] memory amounts) {
        amounts = getAmountsOut(amountIn, path);
        require(amounts[amounts.length - 1] >= amountOutMin, "TigerRouter: INSUFFICIENT_OUTPUT_AMOUNT");
        
        // Transfer tokens
        IERC20(path[0]).transferFrom(msg.sender, factory, amountIn);
        
        // Execute swap (simplified)
        _swap(amounts, path, to);
        
        return amounts;
    }
    
    /**
     * @dev Swap tokens for exact tokens
     */
    function swapTokensForExactTokens(
        uint256 amountOut,
        uint256 amountInMax,
        address[] calldata path,
        address to,
        uint256 deadline
    ) external ensure(deadline) returns (uint256[] memory amounts) {
        amounts = getAmountsIn(amountOut, path);
        require(amounts[0] <= amountInMax, "TigerRouter: EXCESSIVE_INPUT_AMOUNT");
        
        IERC20(path[0]).transferFrom(msg.sender, factory, amounts[0]);
        
        _swap(amounts, path, to);
        
        return amounts;
    }
    
    /**
     * @dev Swap exact ETH for tokens
     */
    function swapExactETHForTokens(uint256 amountOutMin, address[] calldata path, address to, uint256 deadline)
        external
        payable
        ensure(deadline)
        returns (uint256[] memory amounts)
    {
        require(path[0] == WETH, "TigerRouter: INVALID_PATH");
        amounts = getAmountsOut(msg.value, path);
        require(amounts[amounts.length - 1] >= amountOutMin, "TigerRouter: INSUFFICIENT_OUTPUT_AMOUNT");
        
        IWETH(WETH).deposit{value: amounts[0]}();
        assert(IWETH(WETH).transfer(factory, amounts[0]));
        
        _swap(amounts, path, to);
        
        return amounts;
    }
    
    /**
     * @dev Swap tokens for exact ETH
     */
    function swapTokensForExactETH(uint256 amountOut, uint256 amountInMax, address[] calldata path, address to, uint256 deadline)
        external
        ensure(deadline)
        returns (uint256[] memory amounts)
    {
        require(path[path.length - 1] == WETH, "TigerRouter: INVALID_PATH");
        
        amounts = getAmountsIn(amountOut, path);
        require(amounts[0] <= amountInMax, "TigerRouter: EXCESSIVE_INPUT_AMOUNT");
        
        IERC20(path[0]).transferFrom(msg.sender, factory, amounts[0]);
        
        _swap(amounts, path, address(this));
        
        IWETH(WETH).withdraw(amounts[amounts.length - 1]);
        payable(to).transfer(amounts[amounts.length - 1]);
        
        return amounts;
    }
    
    /**
     * @dev Get amounts out
     */
    function getAmountsOut(uint256 amountIn, address[] memory path) public view returns (uint256[] memory amounts) {
        require(path.length >= 2, "TigerRouter: INVALID_PATH");
        amounts = new uint256[](path.length);
        amounts[0] = amountIn;
        
        for (uint256 i; i < path.length - 1; i++) {
            (uint256 reserveIn, uint256 reserveOut) = getReserves(path[i], path[i + 1]);
            amounts[i + 1] = getAmountOut(amounts[i], reserveIn, reserveOut);
        }
    }
    
    /**
     * @dev Get amounts in
     */
    function getAmountsIn(uint256 amountOut, address[] memory path) public view returns (uint256[] memory amounts) {
        require(path.length >= 2, "TigerRouter: INVALID_PATH");
        amounts = new uint256[](path.length);
        amounts[amounts.length - 1] = amountOut;
        
        for (uint256 i = path.length - 1; i > 0; i--) {
            (uint256 reserveIn, uint256 reserveOut) = getReserves(path[i - 1], path[i]);
            amounts[i - 1] = getAmountIn(amounts[i], reserveIn, reserveOut);
        }
    }
    
    /**
     * @dev Get amount out
     */
    function getAmountOut(uint256 amountIn, uint256 reserveIn, uint256 reserveOut) internal pure returns (uint256 amountOut) {
        require(amountIn > 0, "TigerRouter: INSUFFICIENT_INPUT_AMOUNT");
        require(reserveIn > 0 && reserveOut > 0, "TigerRouter: INSUFFICIENT_LIQUIDITY");
        
        uint256 amountInWithFee = amountIn * 997;
        uint256 numerator = amountInWithFee * reserveOut;
        uint256 denominator = reserveIn * 1000 + amountInWithFee;
        
        amountOut = numerator / denominator;
    }
    
    /**
     * @dev Get amount in
     */
    function getAmountIn(uint256 amountOut, uint256 reserveIn, uint256 reserveOut) internal pure returns (uint256 amountIn) {
        require(amountOut > 0, "TigerRouter: INSUFFICIENT_OUTPUT_AMOUNT");
        require(reserveIn > 0 && reserveOut > 0, "TigerRouter: INSUFFICIENT_LIQUIDITY");
        
        uint256 numerator = reserveIn * amountOut * 1000;
        uint256 denominator = (reserveOut - amountOut) * 997;
        
        amountIn = numerator / denominator + 1;
    }
    
    /**
     * @dev Get reserves
     */
    function getReserves(address tokenA, address tokenB) internal view returns (uint256 reserveA, uint256 reserveB) {
        // Simplified - would normally query factory
        (reserveA, reserveB) = (1000000e18, 1000000e18);
    }
    
    /**
     * @dev Swap
     */
    function _swap(uint256[] memory amounts, address[] memory path, address _to) internal {
        for (uint256 i; i < path.length - 1; i++) {
            (address input, address output) = (path[i], path[i + 1]);
            (uint256 amountOut) = amounts[i + 1];
            address to = i < path.length - 2 ? factory : _to;
            
            // Simplified swap logic
            IERC20(output).transfer(to, amountOut);
        }
    }
}

interface IERC20 {
    function transfer(address to, uint256 amount) external returns (bool);
    function transferFrom(address from, address to, uint256 amount) external returns (bool);
}

interface IWETH {
    function deposit() external payable;
    function transfer(address to, uint256 amount) external returns (bool);
    function withdraw(uint256 amount) external;
}

/**
 * @title TigerFactory
 * @dev Factory for creating pairs
 */
contract TigerFactory {
    bytes32 public constant INIT_CODE_PAIR_HASH = keccak256(abi.encodePacked(type(TigerPair).creationCode));
    
    mapping(address => mapping(address => address)) public getPair;
    address[] public allPairs;
    
    event PairCreated(address indexed token0, address indexed token1, address pair, uint256);
    
    function createPair(address tokenA, address tokenB) external returns (address pair) {
        require(tokenA != tokenB, "TigerFactory: IDENTICAL_ADDRESSES");
        
        (address token0, address token1) = tokenA < tokenB ? (tokenA, tokenB) : (tokenB, tokenA);
        require(getPair[token0][token1] == address(0), "TigerFactory: PAIR_EXISTS");
        
        // Simplified - would deploy actual pair contract
        pair = address(new TigerPair());
        
        getPair[token0][token1] = pair;
        getPair[token1][token0] = pair;
        allPairs.push(pair);
        
        emit PairCreated(token0, token1, pair, allPairs.length);
    }
}

/**
 * @title TigerPair
 * @dev Pair contract
 */
contract TigerPair {
    address public factory;
    address public token0;
    address public token1;
    
    uint256 public reserve0;
    uint256 public reserve1;
    
    constructor() {
        factory = msg.sender;
    }
    
    function initialize(address _token0, address _token0, address _token1) external {
        require(msg.sender == factory, "TigerPair: FORBIDDEN");
        token0 = _token0;
        token1 = _token1;
    }
}