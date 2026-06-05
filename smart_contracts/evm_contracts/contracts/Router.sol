// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/**
 * @title TigerSwapRouter
 * @notice Router contract for executing swaps and liquidity operations
 * @dev User-facing contract that interacts with pairs
 */
contract TigerSwapRouter {
    using SafeMath for uint256;

    // Factory address
    address public immutable factory;

    // WETH for wrapping ETH
    address public immutable WETH;

    // Security modifier for contracts
    modifier ensure(uint256 deadline) {
        require(deadline >= block.timestamp, "TigerSwapRouter: EXPIRED");
        _;
    }

    constructor(address _factory, address _WETH) {
        factory = _factory;
        WETH = _WETH;
    }

    // ==================== LIQUIDITY OPERATIONS ====================

    /**
     * @notice Add liquidity to a pool
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
        (amountA, amountB) = _addLiquidity(tokenA, tokenB, amountADesired, amountBDesired);
        
        // Transfer tokens from user
        TransferHelper.safeTransferFrom(tokenA, msg.sender, pairFor(tokenA, tokenB), amountA);
        TransferHelper.safeTransferFrom(tokenB, msg.sender, pairFor(tokenA, tokenB), amountB);

        // Mint LP tokens
        address pair = ITigerSwapFactory(factory).getPairAddress(tokenA, tokenB);
        liquidity = ITigerSwapPair(pair).mint(to);
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
        (amountToken, amountETH) = _addLiquidity(token, WETH, amountTokenDesired, msg.value);
        
        // Transfer tokens
        TransferHelper.safeTransferFrom(token, msg.sender, pairFor(token, WETH), amountToken);
        
        // Wrap ETH and transfer to pair
        IWETH(WETH).deposit{value: amountETH}();
        assert(IWETH(WETH).transfer(pairFor(token, WETH), amountETH));

        // Mint LP tokens
        address pair = ITigerSwapFactory(factory).getPairAddress(token, WETH);
        liquidity = ITigerSwapPair(pair).mint(to);
        
        // Refund excess ETH
        if (msg.value > amountETH) {
            TransferHelper.safeTransferETH(msg.sender, msg.value - amountETH);
        }
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
        address pair = ITigerSwapFactory(factory).getPairAddress(tokenA, tokenB);
        
        // Transfer LP tokens from user
        TransferHelper.safeTransferFrom(pair, msg.sender, pair, liquidity);
        
        // Burn and get back tokens
        (amountA, amountB) = ITigerSwapPair(pair).burn(to);
        
        require(amountA >= amountAMin, "TigerSwapRouter: INSUFFICIENT_A");
        require(amountB >= amountBMin, "TigerSwapRouter: INSUFFICIENT_B");
    }

    /**
     * @notice Remove liquidity with ETH
     */
    function removeLiquidityETH(
        address token,
        uint256 liquidity,
        uint256 amountTokenMin,
        uint256 amountETHMin,
        address to,
        uint256 deadline
    ) public ensure(deadline) returns (uint256 amountToken, uint256 amountETH) {
        (amountToken, amountETH) = removeLiquidity(
            token, WETH, liquidity, amountTokenMin, amountETHMin, address(this), deadline
        );
        
        // Transfer token to user
        TransferHelper.safeTransfer(token, to, amountToken);
        
        // Unwrap WETH and send ETH
        IWETH(WETH).withdraw(amountETH);
        TransferHelper.safeTransferETH(to, amountETH);
    }

    // ==================== SWAP OPERATIONS ====================

    /**
     * @notice Swap exact tokens for tokens
     */
    function swapExactTokensForTokens(
        uint256 amountIn,
        uint256 amountOutMin,
        address[] calldata path,
        address to,
        uint256 deadline
    ) external ensure(deadline) returns (uint256[] memory amounts) {
        amounts = getAmountsOut(amountIn, path);
        require(amounts[amounts.length - 1] >= amountOutMin, "TigerSwapRouter: INSUFFICIENT_OUTPUT");

        // Transfer from sender to first pair
        TransferHelper.safeTransferFrom(
            path[0], msg.sender, ITigerSwapFactory(factory).getPairAddress(path[0], path[1]), amounts[0]
        );

        // Execute swaps through path
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
        require(amounts[0] <= amountInMax, "TigerSwapRouter: EXCESSIVE_INPUT");

        TransferHelper.safeTransferFrom(
            path[0], msg.sender, ITigerSwapFactory(factory).getPairAddress(path[0], path[1]), amounts[0]
        );

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
        require(path[0] == WETH, "TigerSwapRouter: INVALID_PATH");
        amounts = getAmountsOut(msg.value, path);
        require(amounts[amounts.length - 1] >= amountOutMin, "TigerSwapRouter: INSUFFICIENT_OUTPUT");

        IWETH(WETH).deposit{value: amounts[0]}();
        assert(IWETH(WETH).transfer(ITigerSwapFactory(factory).getPairAddress(path[0], path[1]), amounts[0]));

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
        require(path[path.length - 1] == WETH, "TigerSwapRouter: INVALID_PATH");
        amounts = getAmountsIn(amountOut, path);
        require(amounts[0] <= amountInMax, "TigerSwapRouter: EXCESSIVE_INPUT");

        TransferHelper.safeTransferFrom(
            path[0], msg.sender, ITigerSwapFactory(factory).getPairAddress(path[0], path[1]), amounts[0]
        );
        _swap(amounts, path, address(this));

        IWETH(WETH).withdraw(amounts[amounts.length - 1]);
        TransferHelper.safeTransferETH(to, amounts[amounts.length - 1]);
    }

    /**
     * @notice Swap exact tokens for ETH
     */
    function swapExactTokensForETH(uint256 amountIn, uint256 amountOutMin, address[] calldata path, address to, uint256 deadline)
        external
        ensure(deadline)
        returns (uint256[] memory amounts)
    {
        require(path[path.length - 1] == WETH, "TigerSwapRouter: INVALID_PATH");
        amounts = getAmountsOut(amountIn, path);
        require(amounts[amounts.length - 1] >= amountOutMin, "TigerSwapRouter: INSUFFICIENT_OUTPUT");

        TransferHelper.safeTransferFrom(
            path[0], msg.sender, ITigerSwapFactory(factory).getPairAddress(path[0], path[1]), amounts[0]
        );
        _swap(amounts, path, address(this));

        IWETH(WETH).withdraw(amounts[amounts.length - 1]);
        TransferHelper.safeTransferETH(to, amounts[amounts.length - 1]);
    }

    /**
     * @notice Swap ETH for exact tokens
     */
    function swapETHForExactTokens(uint256 amountOut, address[] calldata path, address to, uint256 deadline)
        external
        payable
        ensure(deadline)
        returns (uint256[] memory amounts)
    {
        require(path[0] == WETH, "TigerSwapRouter: INVALID_PATH");
        amounts = getAmountsIn(amountOut, path);
        require(amounts[0] <= msg.value, "TigerSwapRouter: EXCESSIVE_INPUT");

        IWETH(WETH).deposit{value: amounts[0]}();
        assert(IWETH(WETH).transfer(ITigerSwapFactory(factory).getPairAddress(path[0], path[1]), amounts[0]));

        _swap(amounts, path, to);

        // Refund excess ETH
        if (msg.value > amounts[0]) {
            TransferHelper.safeTransferETH(msg.sender, msg.value - amounts[0]);
        }
    }

    // ==================== QUOTE FUNCTIONS ====================

    /**
     * @notice Get quote for adding liquidity
     */
    function quote(uint256 amountA, uint256 reserveA, uint256 reserveB) public pure returns (uint256 amountB) {
        return amountA * reserveB / reserveA;
    }

    /**
     * @notice Get amounts out for a swap
     */
    function getAmountOut(uint256 amountIn, uint256 reserveIn, uint256 reserveOut) public pure returns (uint256 amountOut) {
        require(amountIn > 0, "TigerSwapRouter: INSUFFICIENT_INPUT_AMOUNT");
        require(reserveIn > 0 && reserveOut > 0, "TigerSwapRouter: INSUFFICIENT_LIQUIDITY");
        
        uint256 amountInWithFee = amountIn * 997; // 0.3% fee
        uint256 numerator = amountInWithFee * reserveOut;
        uint256 denominator = reserveIn * 1000 + amountInWithFee;
        amountOut = numerator / denominator;
    }

    /**
     * @notice Get amounts in for a swap
     */
    function getAmountIn(uint256 amountOut, uint256 reserveIn, uint256 reserveOut) public pure returns (uint256 amountIn) {
        require(amountOut > 0, "TigerSwapRouter: INSUFFICIENT_OUTPUT_AMOUNT");
        require(reserveIn > 0 && reserveOut > 0, "TigerSwapRouter: INSUFFICIENT_LIQUIDITY");
        
        uint256 numerator = reserveIn * amountOut * 1000;
        uint256 denominator = (reserveOut - amountOut) * 997;
        amountIn = numerator / denominator + 1;
    }

    /**
     * @notice Get amounts for path
     */
    function getAmountsOut(uint256 amountIn, address[] memory path) public view returns (uint256[] memory amounts) {
        amounts = new uint256[](path.length);
        amounts[0] = amountIn;
        for (uint256 i = 0; i < path.length - 1; i++) {
            (uint112 reserve0, uint112 reserve1,) = ITigerSwapPair(ITigerSwapFactory(factory).getPairAddress(path[i], path[i+1])).getReserves();
            (uint112 reserveIn, uint112 reserveOut) = path[i] < path[i+1] ? (reserve0, reserve1) : (reserve1, reserve0);
            amounts[i+1] = getAmountOut(amounts[i], reserveIn, reserveOut);
        }
    }

    /**
     * @notice Get amounts in for path
     */
    function getAmountsIn(uint256 amountOut, address[] memory path) public view returns (uint256[] memory amounts) {
        amounts = new uint256[](path.length);
        amounts[amounts.length - 1] = amountOut;
        for (uint256 i = path.length - 1; i > 0; i--) {
            (uint112 reserve0, uint112 reserve1,) = ITigerSwapPair(ITigerSwapFactory(factory).getPairAddress(path[i-1], path[i])).getReserves();
            (uint112 reserveIn, uint112 reserveOut) = path[i-1] < path[i] ? (reserve0, reserve1) : (reserve1, reserve0);
            amounts[i-1] = getAmountIn(amounts[i], reserveIn, reserveOut);
        }
    }

    // ==================== INTERNAL FUNCTIONS ====================

    function _addLiquidity(
        address tokenA,
        address tokenB,
        uint256 amountADesired,
        uint256 amountBDesired
    ) internal view returns (uint256 amountA, uint256 amountB) {
        (uint256 reserveA, uint256 reserveB) = _getReserves(tokenA, tokenB);
        
        if (reserveA == 0 && reserveB == 0) {
            // New pool - use desired amounts
            (amountA, amountB) = (amountADesired, amountBDesired);
        } else {
            // Existing pool - calculate optimal amounts
            uint256 amountBOptimal = quote(amountADesired, reserveA, reserveB);
            if (amountBOptimal <= amountBDesired) {
                (amountA, amountB) = (amountADesired, amountBOptimal);
            } else {
                uint256 amountAOptimal = quote(amountBDesired, reserveB, reserveA);
                (amountA, amountB) = (amountAOptimal, amountBDesired);
            }
        }
    }

    function _getReserves(address tokenA, address tokenB) internal view returns (uint256 reserveA, uint256 reserveB) {
        address pair = ITigerSwapFactory(factory).getPairAddress(tokenA, tokenB);
        (reserveA, reserveB,) = ITigerSwapPair(pair).getReserves();
    }

    function _swap(uint256[] memory amounts, address[] memory path, address _to) internal {
        for (uint256 i = 0; i < path.length - 1; i++) {
            (uint112 reserve0, uint112 reserve1,) = ITigerSwapPair(ITigerSwapFactory(factory).getPairAddress(path[i], path[i+1])).getReserves();
            (uint256 input, uint256 output) = path[i] < path[i+1] 
                ? (amounts[i], amounts[i+1]) 
                : (amounts[i+1], amounts[i]);
            
            address to = i < path.length - 2 ? ITigerSwapFactory(factory).getPairAddress(path[i+1], path[i+2]) : _to;
            
            ITigerSwapPair(ITigerSwapFactory(factory).getPairAddress(path[i], path[i+1])).swap(
                path[i] < path[i+1] ? uint256(0) : output,
                path[i] < path[i+1] ? output : uint256(0),
                to
            );
        }
    }

    function pairFor(address tokenA, address tokenB) internal view returns (address) {
        return ITigerSwapFactory(factory).getPairAddress(tokenA, tokenB);
    }
}

// ==================== HELPER CONTRACTS ====================

/**
 * @title TransferHelper
 * @notice Safe token transfer utilities
 */
library TransferHelper {
    function safeTransferFrom(address token, address from, address to, uint256 value) internal {
        (bool success, bytes memory data) = token.call(
            abi.encodeWithSignature("transferFrom(address,address,uint256)", from, to, value)
        );
        require(success && (data.length == 0 || abi.decode(data, (bool))), "TransferHelper: TRANSFER_FAILED");
    }

    function safeTransfer(address token, address to, uint256 value) internal {
        (bool success, bytes memory data) = token.call(
            abi.encodeWithSignature("transfer(address,uint256)", to, value)
        );
        require(success && (data.length == 0 || abi.decode(data, (bool))), "TransferHelper: TRANSFER_FAILED");
    }

    function safeTransferETH(address to, uint256 value) internal {
        (bool success,) = to.call{value: value}(new bytes(0));
        require(success, "TransferHelper: ETH_TRANSFER_FAILED");
    }
}

/**
 * @title IWETH
 * @notice WETH interface
 */
interface IWETH {
    function deposit() external payable;
    function withdraw(uint256) external;
}

/**
 * @title ITigerSwapFactory
 */
interface ITigerSwapFactory {
    function getPairAddress(address, address) external view returns (address);
    function feeTo() external view returns (address);
}

/**
 * @title ITigerSwapPair
 */
interface ITigerSwapPair {
    function getReserves() external view returns (uint112, uint112, uint32);
    function mint(address) external returns (uint256);
    function burn(address) external returns (uint256, uint256);
    function swap(uint256, uint256, address) external;
}