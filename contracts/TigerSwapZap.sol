// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./TigerSwapInterfaces.sol";

/// @notice Minimal view of a TigerSwap V2 pool, which is also its LP token.
interface ITigerSwapPoolView {
    function getReserves() external view returns (uint112 reserve0, uint112 reserve1, uint32 blockTimestampLast);
    function token0() external view returns (address);
    function token1() external view returns (address);
    function totalSupply() external view returns (uint256);
    function balanceOf(address account) external view returns (uint256);
    function transfer(address to, uint256 value) external returns (bool);
    function transferFrom(address from, address to, uint256 value) external returns (bool);
    function approve(address spender, uint256 value) external returns (bool);
}

/// @notice Router surface used by the zap.
interface ITigerSwapRouterZap {
    function WETH() external view returns (address);

    function addLiquidity(
        address tokenA,
        address tokenB,
        uint256 amountADesired,
        uint256 amountBDesired,
        uint256 amountAMin,
        uint256 amountBMin,
        address to,
        uint256 deadline
    ) external returns (uint256 amountA, uint256 amountB, uint256 liquidity);

    function removeLiquidity(
        address tokenA,
        address tokenB,
        uint256 liquidity,
        uint256 amountAMin,
        uint256 amountBMin,
        address to,
        uint256 deadline
    ) external returns (uint256 amountA, uint256 amountB);

    function swapExactTokensForTokens(
        uint256 amountIn,
        uint256 amountOutMin,
        address[] calldata path,
        address to,
        uint256 deadline
    ) external returns (uint256[] memory amounts);

    function getAmountsOut(uint256 amountIn, address[] memory path) external view returns (uint256[] memory amounts);
}

/**
 * @title TigerSwapZap
 * @notice Single-sided liquidity entry and exit.
 *
 * `zapIn` takes one token and returns a two-sided LP position: it swaps the
 * part of the input that balances the deposit against the pool ratio, adds the
 * liquidity, forwards the LP tokens to the user, and refunds any dust.
 *
 * `zapOut` burns LP tokens and returns a single token, swapping the unwanted
 * side back.
 *
 * Both ETH variants wrap and unwrap through the router's WETH.
 */
contract TigerSwapZap {
    ITigerSwapRouterZap public immutable router;
    ITigerSwapFactory public immutable factory;
    address public immutable WETH;

    address public owner;

    event ZappedIn(
        address indexed user,
        address indexed tokenIn,
        address indexed tokenOut,
        uint256 amountIn,
        uint256 lpOut
    );
    event ZappedOut(address indexed user, address indexed tokenIn, address indexed tokenOut, uint256 lpIn, uint256 amountOut);
    event DustRefunded(address indexed user, address indexed token, uint256 amount);
    event OwnershipTransferred(address indexed previousOwner, address indexed newOwner);

    modifier onlyOwner() {
        require(msg.sender == owner, "not owner");
        _;
    }

    uint256 private _status = 1;

    modifier nonReentrant() {
        require(_status == 1, "reentrant call");
        _status = 2;
        _;
        _status = 1;
    }

    constructor(address _router, address _factory) {
        require(_router != address(0) && _factory != address(0), "zero address");
        router = ITigerSwapRouterZap(_router);
        factory = ITigerSwapFactory(_factory);
        WETH = ITigerSwapRouterZap(_router).WETH();
        owner = msg.sender;
        emit OwnershipTransferred(address(0), msg.sender);
    }

    receive() external payable {
        // Only the router's unwrap path may send ETH here.
        require(msg.sender == WETH, "only WETH");
    }

    // ---- Math --------------------------------------------------------------

    function _sqrt(uint256 y) internal pure returns (uint256 z) {
        if (y > 3) {
            z = y;
            uint256 x = y / 2 + 1;
            while (x < z) {
                z = x;
                x = (y / x + x) / 2;
            }
        } else if (y != 0) {
            z = 1;
        }
    }

    /// @dev Input amount to swap so the remaining input and the swap output sit
    /// in the post-swap reserve ratio. With reserveIn == reserveOut == R that
    /// balance condition reduces to s^2 + 2sR - A*R = 0, so s = sqrt(R^2 + A*R) - R.
    /// The same closed form is a close approximation for skewed pools, and any
    /// residual imbalance is absorbed by addLiquidity and refunded as dust.
    function _optimalSwapAmount(uint256 _amountIn, uint256 _reserveIn) public pure returns (uint256) {
        uint256 root = _sqrt(_reserveIn * _reserveIn + _amountIn * _reserveIn);
        if (root <= _reserveIn) {
            return 0;
        }
        uint256 swapAmount = root - _reserveIn;
        // Never swap the whole balance; something must remain to pair with.
        if (swapAmount >= _amountIn) {
            swapAmount = _amountIn / 2;
        }
        return swapAmount;
    }

    // ---- Zap in ------------------------------------------------------------

    /// @notice Swap part of `_tokenIn` into `_tokenOut` and add both as liquidity.
    /// @dev The caller must approve this contract for `_amountIn` of `_tokenIn`.
    function zapIn(address _tokenIn, address _tokenOut, uint256 _amountIn, uint256 _minLp, uint256 _deadline)
        external
        nonReentrant
        returns (uint256 liquidity)
    {
        require(_tokenIn != _tokenOut, "identical tokens");
        require(_amountIn > 0, "amount required");
        require(block.timestamp <= _deadline, "expired");

        address pool = factory.getPoolAddress(_tokenIn, _tokenOut);
        require(pool != address(0), "pool does not exist");

        uint256 reserveIn = _reserveOf(pool, _tokenIn);
        uint256 reserveOut = _reserveOf(pool, _tokenOut);
        require(reserveIn > 0 && reserveOut > 0, "pool has no liquidity");

        require(IERC20(_tokenIn).transferFrom(msg.sender, address(this), _amountIn), "transfer in failed");

        uint256 swapAmount = _optimalSwapAmount(_amountIn, reserveIn);
        uint256 amountOut = _swap(_tokenIn, _tokenOut, swapAmount, _deadline);

        uint256 amountInRemaining = _amountIn - swapAmount;
        liquidity = _addLiquidity(_tokenIn, _tokenOut, amountInRemaining, amountOut, _deadline);

        require(liquidity >= _minLp, "slippage exceeded");
        require(IERC20(pool).transfer(msg.sender, liquidity), "lp transfer failed");

        _refundDust(msg.sender, _tokenIn, _tokenOut);

        emit ZappedIn(msg.sender, _tokenIn, _tokenOut, _amountIn, liquidity);
    }

    /// @notice Zap ETH into an ETH-pair LP position.
    function zapInETH(address _tokenOut, uint256 _minLp, uint256 _deadline)
        external
        payable
        nonReentrant
        returns (uint256 liquidity)
    {
        require(_tokenOut != WETH, "use zapIn for WETH");
        require(msg.value > 0, "value required");
        require(block.timestamp <= _deadline, "expired");

        address pool = factory.getPoolAddress(WETH, _tokenOut);
        require(pool != address(0), "pool does not exist");

        uint256 reserveIn = _reserveOf(pool, WETH);
        uint256 reserveOut = _reserveOf(pool, _tokenOut);
        require(reserveIn > 0 && reserveOut > 0, "pool has no liquidity");

        IWETH(WETH).deposit{value: msg.value}();

        uint256 swapAmount = _optimalSwapAmount(msg.value, reserveIn);
        uint256 amountOut = _swap(WETH, _tokenOut, swapAmount, _deadline);

        liquidity = _addLiquidity(WETH, _tokenOut, msg.value - swapAmount, amountOut, _deadline);
        require(liquidity >= _minLp, "slippage exceeded");
        require(IERC20(pool).transfer(msg.sender, liquidity), "lp transfer failed");

        _refundDustETH(msg.sender, _tokenOut);

        emit ZappedIn(msg.sender, WETH, _tokenOut, msg.value, liquidity);
    }

    // ---- Zap out -----------------------------------------------------------

    /// @notice Burn LP tokens and receive only `_tokenOut`.
    /// @dev The caller must approve this contract for `_lpAmount` of the LP token.
    function zapOut(address _tokenIn, address _tokenOut, uint256 _lpAmount, uint256 _minAmountOut, uint256 _deadline)
        external
        nonReentrant
        returns (uint256 amountOut)
    {
        require(_tokenIn != _tokenOut, "identical tokens");
        require(_lpAmount > 0, "amount required");
        require(block.timestamp <= _deadline, "expired");

        address pool = factory.getPoolAddress(_tokenIn, _tokenOut);
        require(pool != address(0), "pool does not exist");

        require(ITigerSwapPoolView(pool).transferFrom(msg.sender, address(this), _lpAmount), "lp transfer failed");
        // removeLiquidity pulls the LP tokens through the router, so it needs an
        // allowance from this contract.
        require(ITigerSwapPoolView(pool).approve(address(router), _lpAmount), "lp approve failed");

        (uint256 amountIn, uint256 amountSide) = router.removeLiquidity(_tokenIn, _tokenOut, _lpAmount, 0, 0, address(this), _deadline);

        // Convert the unwanted side into the requested token.
        uint256 swapped = _swap(_tokenOut, _tokenIn, amountSide, _deadline);
        amountOut = amountIn + swapped;

        require(amountOut >= _minAmountOut, "slippage exceeded");
        require(IERC20(_tokenIn).transfer(msg.sender, amountOut), "payout failed");

        emit ZappedOut(msg.sender, _tokenIn, _tokenOut, _lpAmount, amountOut);
    }

    /// @notice Burn LP tokens and receive only ETH.
    function zapOutETH(address _tokenIn, uint256 _lpAmount, uint256 _minETH, uint256 _deadline)
        external
        nonReentrant
        returns (uint256 amountOut)
    {
        require(_tokenIn != WETH, "use zapOut for WETH");
        require(_lpAmount > 0, "amount required");
        require(block.timestamp <= _deadline, "expired");

        address pool = factory.getPoolAddress(_tokenIn, WETH);
        require(pool != address(0), "pool does not exist");

        require(ITigerSwapPoolView(pool).transferFrom(msg.sender, address(this), _lpAmount), "lp transfer failed");
        require(ITigerSwapPoolView(pool).approve(address(router), _lpAmount), "lp approve failed");

        (uint256 amountToken, uint256 amountWeth) =
            router.removeLiquidity(_tokenIn, WETH, _lpAmount, 0, 0, address(this), _deadline);

        uint256 swapped = _swap(_tokenIn, WETH, amountToken, _deadline);
        uint256 totalWeth = amountWeth + swapped;

        require(totalWeth >= _minETH, "slippage exceeded");
        IWETH(WETH).withdraw(totalWeth);
        amountOut = totalWeth;

        (bool ok,) = msg.sender.call{value: totalWeth}("");
        require(ok, "eth payout failed");

        emit ZappedOut(msg.sender, _tokenIn, WETH, _lpAmount, amountOut);
    }

    // ---- Internal helpers --------------------------------------------------

    function _reserveOf(address _pool, address _token) internal view returns (uint256) {
        (uint112 r0, uint112 r1,) = ITigerSwapPoolView(_pool).getReserves();
        return ITigerSwapPoolView(_pool).token0() == _token ? uint256(r0) : uint256(r1);
    }

    function _swap(address _tokenIn, address _tokenOut, uint256 _amountIn, uint256 _deadline)
        internal
        returns (uint256 amountOut)
    {
        if (_amountIn == 0) {
            return 0;
        }
        IERC20(_tokenIn).approve(address(router), _amountIn);

        address[] memory path = new address[](2);
        path[0] = _tokenIn;
        path[1] = _tokenOut;

        uint256[] memory amounts = router.swapExactTokensForTokens(_amountIn, 0, path, address(this), _deadline);
        amountOut = amounts[amounts.length - 1];
    }

    function _addLiquidity(
        address _tokenA,
        address _tokenB,
        uint256 _amountA,
        uint256 _amountB,
        uint256 _deadline
    ) internal returns (uint256 liquidity) {
        IERC20(_tokenA).approve(address(router), _amountA);
        IERC20(_tokenB).approve(address(router), _amountB);

        (, , liquidity) = router.addLiquidity(_tokenA, _tokenB, _amountA, _amountB, 0, 0, address(this), _deadline);
    }

    /// @dev Send any balance left in this contract back to the user.
    function _refundDust(address _user, address _tokenIn, address _tokenOut) internal {
        uint256 dustIn = IERC20(_tokenIn).balanceOf(address(this));
        if (dustIn > 0) {
            require(IERC20(_tokenIn).transfer(_user, dustIn), "dust refund failed");
            emit DustRefunded(_user, _tokenIn, dustIn);
        }
        uint256 dustOut = IERC20(_tokenOut).balanceOf(address(this));
        if (dustOut > 0) {
            require(IERC20(_tokenOut).transfer(_user, dustOut), "dust refund failed");
            emit DustRefunded(_user, _tokenOut, dustOut);
        }
    }

    function _refundDustETH(address _user, address _tokenOut) internal {
        uint256 dustWeth = IERC20(WETH).balanceOf(address(this));
        if (dustWeth > 0) {
            IWETH(WETH).withdraw(dustWeth);
        }
        uint256 dustToken = IERC20(_tokenOut).balanceOf(address(this));
        if (dustToken > 0) {
            require(IERC20(_tokenOut).transfer(_user, dustToken), "dust refund failed");
            emit DustRefunded(_user, _tokenOut, dustToken);
        }
        uint256 balance = address(this).balance;
        if (balance > 0) {
            (bool ok,) = _user.call{value: balance}("");
            require(ok, "eth dust refund failed");
            emit DustRefunded(_user, WETH, balance);
        }
    }

    // ---- Admin -------------------------------------------------------------

    /// @notice Sweep tokens that were sent here by mistake. Never touches user funds
    /// mid-operation because every zap path is atomic and guarded.
    function sweep(address _token, address _to, uint256 _amount) external onlyOwner {
        require(_to != address(0), "zero address");
        require(IERC20(_token).transfer(_to, _amount), "sweep failed");
    }

    function transferOwnership(address _newOwner) external onlyOwner {
        require(_newOwner != address(0), "zero address");
        owner = _newOwner;
        emit OwnershipTransferred(msg.sender, _newOwner);
    }
}
