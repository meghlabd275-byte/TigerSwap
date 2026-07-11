// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/**
 * @title TigerSwapRouter
 * @notice Router contract for swapping tokens through pools
 */
contract TigerSwapRouter {
    address public factory;
    address public WETH;

    uint256 constant MINIMUM_LIQUIDITY = 1000;

    event LiquidityAdded(address indexed provider, uint256 amount0, uint256 amount1);
    event LiquidityRemoved(address indexed provider, uint256 amount0, uint256 amount1);
    event Swap(address indexed sender, address indexed fromToken, address indexed toToken, uint256 amountIn, uint256 amountOut);

    modifier ensure(uint256 deadline) {
        require(deadline >= block.timestamp, "TigerSwap: EXPIRED");
        _;
    }

    constructor(address _factory, address _WETH) {
        factory = _factory;
        WETH = _WETH;
    }

    /**
     * @notice Add liquidity to a pool
     * @param tokenA Token A address
     * @param tokenB Token B address
     * @param amountADesired Desired amount of token A
     * @param amountBDesired Desired amount of token B
     * @param amountAMin Minimum amount of token A
     * @param amountBMin Minimum amount of token B
     * @param to Recipient address
     * @param deadline Deadline timestamp
     * @return amountA Amount of token A added
     * @return amountB Amount of token B added
     * @return liquidity LP tokens minted
     */
    function addLiquidity(
        address tokenA,
        address tokenB,
        uint256 amountADesired,
        uint256 amountBDesired,
        uint256 amountAMin,
        uint256 amountBMin,
        address to,
        uint256 deadline
    ) external ensure(deadline) returns (uint256 amountA, uint256 amountB, uint256 liquidity) {
        (amountA, amountB) = _addLiquidity(tokenA, tokenB, amountADesired, amountBDesired, amountAMin, amountBMin);
        address pool = TigerSwapFactory(factory).getPoolAddress(tokenA, tokenB);
        
        // Transfer tokens to pool
        IERC20(tokenA).transferFrom(msg.sender, pool, amountA);
        IERC20(tokenB).transferFrom(msg.sender, pool, amountB);
        
        // Mint LP tokens
        liquidity = TigerSwapPool(pool).mint(to);
        
        emit LiquidityAdded(to, amountA, amountB);
    }

    /**
     * @notice Add liquidity with ETH
     */
    function addLiquidityETH(
        address token,
        uint256 amountTokenDesired,
        uint256 amountTokenMin,
        uint256 amountETHMin,
        address to,
        uint256 deadline
    ) external payable ensure(deadline) returns (uint256 amountToken, uint256 amountETH, uint256 liquidity) {
        (amountToken, amountETH) = _addLiquidity(token, WETH, amountTokenDesired, msg.value, amountTokenMin, amountETHMin);
        address pool = TigerSwapFactory(factory).getPoolAddress(token, WETH);
        
        IERC20(token).transferFrom(msg.sender, pool, amountToken);
        IWETH(WETH).deposit{value: amountETH}();
        IWETH(WETH).transfer(pool, amountETH);
        
        liquidity = TigerSwapPool(pool).mint(to);
        
        // Return excess ETH
        if (msg.value > amountETH) {
            payable(msg.sender).transfer(msg.value - amountETH);
        }
        
        emit LiquidityAdded(to, amountToken, amountETH);
    }

    /**
     * @notice Remove liquidity
     */
    function removeLiquidity(
        address tokenA,
        address tokenB,
        uint256 liquidity,
        uint256 amountAMin,
        uint256 amountBMin,
        address to,
        uint256 deadline
    ) public ensure(deadline) returns (uint256 amountA, uint256 amountB) {
        address pool = TigerSwapFactory(factory).getPoolAddress(tokenA, tokenB);
        require(pool != address(0), "TigerSwap: POOL_NOT_FOUND");
        
        // Transfer LP tokens
        IERC20(pool).transferFrom(msg.sender, pool, liquidity);
        
        // Burn and get amounts
        (amountA, amountB) = TigerSwapPool(pool).burn(to);
        
        require(amountA >= amountAMin, "TigerSwap: INSUFFICIENT_A");
        require(amountB >= amountBMin, "TigerSwap: INSUFFICIENT_B");
        
        emit LiquidityRemoved(msg.sender, amountA, amountB);
    }

    /**
     * @notice Swap exact tokens for tokens
     * @param amountIn Amount of input tokens
     * @param amountOutMin Minimum output tokens
     * @param path Token path
     * @param to Recipient address
     * @param deadline Deadline timestamp
     * @return amounts Output amounts
     */
    function swapExactTokensForTokens(
        uint256 amountIn,
        uint256 amountOutMin,
        address[] calldata path,
        address to,
        uint256 deadline
    ) external ensure(deadline) returns (uint256[] memory amounts) {
        amounts = getAmountsOut(amountIn, path);
        require(amounts[amounts.length - 1] >= amountOutMin, "TigerSwap: INSUFFICIENT_OUTPUT_AMOUNT");
        
        IERC20(path[0]).transferFrom(msg.sender, factory, amounts[0]);
        _swap(amounts, path, to);
    }

    /**
     * @notice Swap tokens for exact tokens
     */
    function swapTokensForExactTokens(
        uint256 amountOut,
        uint256 amountInMax,
        address[] calldata path,
        address to,
        uint256 deadline
    ) external ensure(deadline) returns (uint256[] memory amounts) {
        amounts = getAmountsIn(amountOut, path);
        require(amounts[0] <= amountInMax, "TigerSwap: EXCESSIVE_INPUT_AMOUNT");
        
        IERC20(path[0]).transferFrom(msg.sender, factory, amounts[0]);
        _swap(amounts, path, to);
    }

    /**
     * @notice Swap exact ETH for tokens
     */
    function swapExactETHForTokens(uint256 amountOutMin, address[] calldata path, address to, uint256 deadline)
        external
        payable
        ensure(deadline)
        returns (uint256[] memory amounts)
    {
        require(path[0] == WETH, "TigerSwap: INVALID_PATH");
        amounts = getAmountsOut(msg.value, path);
        require(amounts[amounts.length - 1] >= amountOutMin, "TigerSwap: INSUFFICIENT_OUTPUT_AMOUNT");
        
        IWETH(WETH).deposit{value: amounts[0]}();
        IERC20(WETH).transfer(factory, amounts[0]);
        _swap(amounts, path, to);
    }

    /**
     * @notice Swap tokens for exact ETH
     */
    function swapTokensForExactETH(uint256 amountOut, uint256 amountInMax, address[] calldata path, address to, uint256 deadline)
        external
        ensure(deadline)
        returns (uint256[] memory amounts)
    {
        require(path[path.length - 1] == WETH, "TigerSwap: INVALID_PATH");
        amounts = getAmountsIn(amountOut, path);
        require(amounts[0] <= amountInMax, "TigerSwap: EXCESSIVE_INPUT_AMOUNT");
        
        IERC20(path[0]).transferFrom(msg.sender, factory, amounts[0]);
        _swap(amounts, path, address(this));
        
        IWETH(WETH).withdraw(amounts[amounts.length - 1]);
        payable(to).transfer(amounts[amounts.length - 1]);
    }

    /**
     * @notice Get output amounts for input amount
     */
    function getAmountsOut(uint256 amountIn, address[] memory path) public view returns (uint256[] memory amounts) {
        require(path.length >= 2, "TigerSwap: INVALID_PATH");
        amounts = new uint256[](path.length);
        amounts[0] = amountIn;
        
        for (uint256 i = 0; i < path.length - 1; i++) {
            address pool = TigerSwapFactory(factory).getPoolAddress(path[i], path[i + 1]);
            if (pool == address(0)) {
                amounts[i + 1] = amounts[i];
                continue;
            }
            
            (uint256 reserve0, uint256 reserve1) = getReserves(pool, path[i], path[i + 1]);
            amounts[i + 1] = getAmountOut(amounts[i], reserve0, reserve1);
        }
    }

    /**
     * @notice Get input amounts for output amount
     */
    function getAmountsIn(uint256 amountOut, address[] memory path) public view returns (uint256[] memory amounts) {
        require(path.length >= 2, "TigerSwap: INVALID_PATH");
        amounts = new uint256[](path.length);
        amounts[amounts.length - 1] = amountOut;
        
        for (uint256 i = path.length - 1; i > 0; i--) {
            (uint256 reserve0, uint256 reserve1) = getReserves(pool, path[i - 1], path[i]);
            amounts[i - 1] = getAmountIn(amounts[i], reserve0, reserve1);
        }
    }

    /**
     * @notice Calculate output amount for input
     */
    function getAmountOut(uint256 amountIn, uint256 reserveIn, uint256 reserveOut) internal pure returns (uint256) {
        require(amountIn > 0, "TigerSwap: INSUFFICIENT_INPUT_AMOUNT");
        require(reserveIn > 0 && reserveOut > 0, "TigerSwap: INSUFFICIENT_LIQUIDITY");
        
        uint256 amountInWithFee = amountIn * 997;
        uint256 numerator = amountInWithFee * reserveOut;
        uint256 denominator = reserveIn * 1000 + amountInWithFee;
        
        return numerator / denominator;
    }

    /**
     * @notice Calculate input amount for output
     */
    function getAmountIn(uint256 amountOut, uint256 reserveIn, uint256 reserveOut) internal pure returns (uint256) {
        require(amountOut > 0, "TigerSwap: INSUFFICIENT_OUTPUT_AMOUNT");
        require(reserveIn > 0 && reserveOut > 0, "TigerSwap: INSUFFICIENT_LIQUIDITY");
        
        uint256 numerator = reserveIn * amountOut * 1000;
        uint256 denominator = (reserveOut - amountOut) * 997;
        
        return numerator / denominator + 1;
    }

    /**
     * @notice Internal swap function
     */
    function _swap(uint256[] memory amounts, address[] memory path, address to) internal {
        for (uint256 i = 0; i < path.length - 1; i++) {
            (address input, address output) = (path[i], path[i + 1]);
            (uint256 reserve0, uint256 reserve1) = getReserves(pool, input, output);
            uint256 amountOut = amounts[i + 1];
            
            address pool = TigerSwapFactory(factory).getPoolAddress(input, output);
            if (to != address(this)) {
                IERC20(output).transfer(to, amountOut);
            }
        }
    }

    /**
     * @notice Get reserves from pool
     */
    function getReserves(address pool, address tokenA, address tokenB) internal view returns (uint256 reserveA, uint256 reserveB) {
        (address token0, ) = sortTokens(tokenA, tokenB);
        (uint256 reserve0, uint256 reserve1, , ) = TigerSwapPool(pool).getReserves();
        (reserveA, reserveB) = tokenA == token0 ? (reserve0, reserve1) : (reserve1, reserve0);
    }

    /**
     * @notice Add liquidity internal
     */
    function _addLiquidity(
        address tokenA,
        address tokenB,
        uint256 amountADesired,
        uint256 amountBDesired,
        uint256 amountAMin,
        uint256 amountBMin
    ) internal returns (uint256 amountA, uint256 amountB) {
        address pool = TigerSwapFactory(factory).getPoolAddress(tokenA, tokenB);
        
        if (pool == address(0)) {
            TigerSwapFactory(factory).createPool(tokenA, tokenB);
            pool = TigerSwapFactory(factory).getPoolAddress(tokenA, tokenB);
        }
        
        (uint256 reserveA, uint256 reserveB) = getReserves(pool, tokenA, tokenB);
        
        if (reserveA == 0 && reserveB == 0) {
            amountA = amountADesired;
            amountB = amountBDesired;
        } else {
            uint256 amountBOptimal = amountADesired * reserveB / reserveA;
            if (amountBOptimal <= amountBDesired) {
                require(amountBOptimal >= amountBMin, "TigerSwap: INSUFFICIENT_B_AMOUNT");
                amountA = amountADesired;
                amountB = amountBOptimal;
            } else {
                uint256 amountAOptimal = amountBDesired * reserveA / reserveB;
                assert(amountAOptimal <= amountADesired);
                require(amountAOptimal >= amountAMin, "TigerSwap: INSUFFICIENT_A_AMOUNT");
                amountA = amountAOptimal;
                amountB = amountBDesired;
            }
        }
    }

    /**
     * @notice Sort token addresses
     */
    function sortTokens(address tokenA, address tokenB) internal pure returns (address token0, address token1) {
        require(tokenA != tokenB, "TigerSwap: IDENTICAL_ADDRESSES");
        (token0, token1) = tokenA < tokenB ? (tokenA, tokenB) : (tokenB, tokenA);
        require(token0 != address(0), "TigerSwap: ZERO_ADDRESS");
    }

    // --- Receive ETH ---
    receive() external payable {
        assert(msg.sender == WETH);
    }
}

/**
 * @title IWETH
 * @notice Interface for WETH
 */
interface IWETH {
    function deposit() external payable;
    function transfer(address to, uint256 value) external returns (bool);
    function withdraw(uint256) external;
}

interface IERC20 {
    function totalSupply() external view returns (uint256);
    function balanceOf(address account) external view returns (uint256);
    function transfer(address to, uint256 value) external returns (bool);
    function allowance(address owner, address spender) external view returns (uint256);
    function approve(address spender, uint256 value) external returns (bool);
    function transferFrom(address from, address to, uint256 value) external returns (bool);
    event Transfer(address indexed from, address indexed to, uint256 value);
    event Approval(address indexed owner, address indexed spender, uint256 value);
}

interface TigerSwapFactory {
    function getPoolAddress(address tokenA, address tokenB) external view returns (address);
    function createPool(address tokenA, address tokenB) external returns (address pool);
}

interface TigerSwapPool {
    function mint(address to) external returns (uint256 liquidity);
    function burn(address to) external returns (uint256 amount0, uint256 amount1);
    function getReserves() external view returns (uint112 reserve0, uint112 reserve1, uint32 blockTimestampLast);
    function swap(uint256 amount0Out, uint256 amount1Out, address to, bytes calldata data) external;
}
