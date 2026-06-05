// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/**
 * @title TigerRouter
 * @notice Main DEX aggregator router for TigerSwap
 * @dev Handles multi-hop swaps, multi-pool routing, and optimal path finding
 */
contract TigerRouter {
    struct Route {
        address[] path;
        address[] pools;
        uint256[] percentages;
    }

    struct SwapParams {
        address tokenIn;
        address tokenOut;
        uint256 amountIn;
        uint256 minAmountOut;
        uint256 deadline;
        Route[] routes;
        address referrer;
        uint256 fee;
    }

    address public factory;
    address public wrappedNative;
    uint256 public constant FEE_DENOMINATOR = 10000;
    uint256 public constant MAX_FEE = 100; // 1%

    mapping(address => mapping(address => address)) public getPair;
    mapping(address => bool) public isWhitelistedDEX;
    mapping(bytes32 => uint256) public bestPriceCache;
    uint256 public constant CACHE_TTL = 30 seconds;

    event Swap(
        address indexed user,
        address indexed tokenIn,
        address indexed tokenOut,
        uint256 amountIn,
        uint256 amountOut,
        address[] path,
        uint256 fee
    );

    modifier deadline(uint256 deadline_) {
        require(block.timestamp <= deadline_, "TigerRouter: EXPIRED");
        _;
    }

    constructor(address _factory, address _wrappedNative) {
        factory = _factory;
        wrappedNative = _wrappedNative;
    }

    function swapExactTokensForTokens(
        uint256 amountIn,
        uint256 minAmountOut,
        address[] calldata path,
        address[] calldata pools,
        address to,
        uint256 deadline_
    ) external deadline(deadline_) returns (uint256[] memory amounts) {
        return _swapExactTokens(amountIn, minAmountOut, path, pools, to);
    }

    function swapTokensForExactTokens(
        uint256 amountOut,
        uint256 maxAmountIn,
        address[] calldata path,
        address[] calldata pools,
        address to,
        uint256 deadline_
    ) external deadline(deadline_) returns (uint256[] memory amounts) {
        return _swapTokensForExact(amountOut, maxAmountIn, path, pools, to);
    }

    function swapExactETHForTokens(
        uint256 minAmountOut,
        address[] calldata path,
        address[] calldata pools,
        address to,
        uint256 deadline_
    ) external payable deadline(deadline_) returns (uint256[] memory amounts) {
        require(path[0] == wrappedNative, "TigerRouter: INVALID_PATH");
        amounts = _swapExactTokens(msg.value, minAmountOut, path, pools, to);
        if (msg.value > amounts[0]) {
            payable(msg.sender).transfer(msg.value - amounts[0]);
        }
    }

    function swapTokensForExactETH(
        uint256 amountOut,
        uint256 maxAmountIn,
        address[] calldata path,
        address[] calldata pools,
        address to,
        uint256 deadline_
    ) external deadline(deadline_) returns (uint256[] memory amounts) {
        require(path[path.length - 1] == wrappedNative, "TigerRouter: INVALID_PATH");
        amounts = _swapTokensForExact(amountOut, maxAmountIn, path, pools, address(this));
        require(address(this).balance >= amountOut, "TigerRouter: INSUFFICIENT_BALANCE");
        payable(to).transfer(amountOut);
    }

    function swapExactTokensForETH(
        uint256 amountIn,
        uint256 minAmountOut,
        address[] calldata path,
        address[] calldata pools,
        address to,
        uint256 deadline_
    ) external deadline(deadline_) returns (uint256[] memory amounts) {
        require(path[path.length - 1] == wrappedNative, "TigerRouter: INVALID_PATH");
        amounts = _swapExactTokens(amountIn, minAmountOut, path, pools, address(this));
        require(address(this).balance >= amounts[amounts.length - 1], "TigerRouter: INSUFFICIENT_BALANCE");
        payable(to).transfer(amounts[amounts.length - 1]);
    }

    function swapExactTokensForTokensSupportingFeeOnTransferTokens(
        uint256 amountIn,
        uint256 minAmountOut,
        address[] calldata path,
        address[] calldata pools,
        address to,
        uint256 deadline_
    ) external deadline(deadline_) {
        _swapSupportingFeeOnTransferTokens(amountIn, minAmountOut, path, pools, to);
    }

    function _swapExactTokens(
        uint256 amountIn,
        uint256 minAmountOut,
        address[] memory path,
        address[] memory pools,
        address to
    ) internal returns (uint256[] memory amounts) {
        amounts = new uint256[](path.length);
        amounts[0] = amountIn;
        
        for (uint256 i = 0; i < path.length - 1; i++) {
            (uint256 reserveIn, uint256 reserveOut) = _getReserves(path[i], path[i + 1], pools[i]);
            amounts[i + 1] = _getAmountOut(amounts[i], reserveIn, reserveOut);
        }

        require(amounts[amounts.length - 1] >= minAmountOut, "TigerRouter: INSUFFICIENT_OUTPUT");

        IERC20(path[0]).transferFrom(msg.sender, address(this), amountIn);
        _swap(amounts, path, pools, to);

        emit Swap(msg.sender, path[0], path[path.length - 1], amountIn, amounts[amounts.length - 1], path, 0);
    }

    function _swapTokensForExact(
        uint256 amountOut,
        uint256 maxAmountIn,
        address[] memory path,
        address[] memory pools,
        address to
    ) internal returns (uint256[] memory amounts) {
        amounts = new uint256[](path.length);
        amounts[amounts.length - 1] = amountOut;

        for (uint256 i = path.length - 1; i > 0; i--) {
            (uint256 reserveIn, uint256 reserveOut) = _getReserves(path[i - 1], path[i], pools[i - 1]);
            amounts[i - 1] = _getAmountIn(amounts[i], reserveIn, reserveOut);
        }

        require(amounts[0] <= maxAmountIn, "TigerRouter: EXCESSIVE_INPUT");
        IERC20(path[0]).transferFrom(msg.sender, address(this), amounts[0]);
        _swap(amounts, path, pools, to);

        emit Swap(msg.sender, path[0], path[path.length - 1], amounts[0], amountOut, path, 0);
    }

    function _swap(
        uint256[] memory amounts,
        address[] memory path,
        address[] memory pools,
        address to
    ) internal {
        for (uint256 i = 0; i < path.length - 1; i++) {
            (address tokenIn, address tokenOut) = (path[i], path[i + 1]);
            (uint256 reserveIn, uint256 reserveOut) = _getReserves(tokenIn, tokenOut, pools[i]);
            uint256 amountOut = amounts[i + 1];

            (uint256 amountIn, uint256 amountOutFinal) = (amounts[i], amounts[i + 1]);
            amountOutFinal = _getAmountOut(amountIn, reserveIn, reserveOut);

            (uint256 amount0Out, uint256 amount1Out) = tokenIn < tokenOut
                ? (uint256(0), amountOutFinal)
                : (amountOutFinal, uint256(0));

            IERC20(tokenIn).transfer(pools[i], amountIn);
            IPool(pools[i]).swap(amount0Out, amount1Out, to, new bytes(0));
        }
    }

    function _swapSupportingFeeOnTransferTokens(
        uint256 amountIn,
        uint256 minAmountOut,
        address[] memory path,
        address[] memory pools,
        address to
    ) internal {
        for (uint256 i = 0; i < path.length - 1; i++) {
            address tokenIn = path[i];
            address tokenOut = path[i + 1];
            uint256 balance = IERC20(tokenIn).balanceOf(address(this));
            (uint256 reserveIn, uint256 reserveOut) = _getReserves(tokenIn, tokenOut, pools[i]);
            uint256 amountOut = _getAmountOut(balance, reserveIn, reserveOut);
            require(amountOut >= minAmountOut, "TigerRouter: INSUFFICIENT_OUTPUT");

            (uint256 amount0Out, uint256 amount1Out) = tokenIn < tokenOut
                ? (uint256(0), amountOut)
                : (amountOut, uint256(0));

            IERC20(tokenIn).transfer(pools[i], balance);
            IPool(pools[i]).swap(amount0Out, amount1Out, to, new bytes(0));
        }
    }

    function _getReserves(address tokenA, address tokenB, address pool) internal view returns (uint256, uint256) {
        (address token0, address token1) = tokenA < tokenB ? (tokenA, tokenB) : (tokenB, tokenA);
        (uint256 reserve0, uint256 reserve1,,) = IPool(pool).getReserves();
        return tokenA == token0 ? (reserve0, reserve1) : (reserve1, reserve0);
    }

    function _getAmountOut(uint256 amountIn, uint256 reserveIn, uint256 reserveOut) internal pure returns (uint256) {
        require(amountIn > 0, "TigerRouter: INSUFFICIENT_INPUT");
        require(reserveIn > 0 && reserveOut > 0, "TigerRouter: INSUFFICIENT_LIQUIDITY");
        
        uint256 amountInWithFee = amountIn * 997;
        uint256 numerator = amountInWithFee * reserveOut;
        uint256 denominator = reserveIn * 1000 + amountInWithFee;
        return numerator / denominator;
    }

    function _getAmountIn(uint256 amountOut, uint256 reserveIn, uint256 reserveOut) internal pure returns (uint256) {
        require(amountOut > 0, "TigerRouter: INSUFFICIENT_OUTPUT");
        require(reserveIn > 0 && reserveOut > 0, "TigerRouter: INSUFFICIENT_LIQUIDITY");
        
        uint256 numerator = reserveIn * amountOut * 1000;
        uint256 denominator = (reserveOut - amountOut) * 997;
        return (numerator / denominator) + 1;
    }

    function getAmountsOut(uint256 amountIn, address[] memory path, address[] memory pools) external view returns (uint256[] memory) {
        uint256[] memory amounts = new uint256[](path.length);
        amounts[0] = amountIn;
        
        for (uint256 i = 0; i < path.length - 1; i++) {
            (uint256 reserveIn, uint256 reserveOut) = _getReserves(path[i], path[i + 1], pools[i]);
            amounts[i + 1] = _getAmountOut(amounts[i], reserveIn, reserveOut);
        }
        return amounts;
    }

    function getAmountsIn(uint256 amountOut, address[] memory path, address[] memory pools) external view returns (uint256[] memory) {
        uint256[] memory amounts = new uint256[](path.length);
        amounts[amounts.length - 1] = amountOut;

        for (uint256 i = path.length - 1; i > 0; i--) {
            (uint256 reserveIn, uint256 reserveOut) = _getReserves(path[i - 1], path[i], pools[i - 1]);
            amounts[i - 1] = _getAmountIn(amounts[i], reserveIn, reserveOut);
        }
        return amounts;
    }

    function addWhitelistedDEX(address dex) external {
        isWhitelistedDEX[dex] = true;
    }

    function removeWhitelistedDEX(address dex) external {
        isWhitelistedDEX[dex] = false;
    }
}

interface IERC20 {
    function balanceOf(address) external view returns (uint256);
    function transfer(address, uint256) external returns (bool);
    function transferFrom(address, address, uint256) external returns (bool);
}

interface IPool {
    function getReserves() external view returns (uint256, uint256, uint256, bool);
    function swap(uint256, uint256, address, bytes calldata) external;
}