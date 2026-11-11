// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./TigerSwapInterfaces.sol";
import "./TigerSwapSafeERC20.sol";

/**
 * @title TigerSwapRouter
 * @notice Router contract for swapping tokens through pools
 */
contract TigerSwapRouter {
    address public factory;
    address public WETH;

    uint256 constant MINIMUM_LIQUIDITY = 1000;
    uint256 constant FEE_DENOMINATOR = 10_000;

    event LiquidityAdded(address indexed provider, uint256 amount0, uint256 amount1);
    event LiquidityRemoved(address indexed provider, uint256 amount0, uint256 amount1);
    event Swap(address indexed sender, address indexed fromToken, address indexed toToken, uint256 amountIn, uint256 amountOut);

    modifier ensure(uint256 deadline) {
        require(deadline >= block.timestamp, "TigerSwap: EXPIRED");
        _;
    }

    /// @dev Guards the paths that hold tokens between hops, so a malicious pool or
    ///      token cannot reenter while the router is mid-swap.
    bool private _locked;
    modifier lock() {
        require(!_locked, "TigerSwap: REENTRANT");
        _locked = true;
        _;
        _locked = false;
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
        address pool = ITigerSwapFactory(factory).getPoolAddress(tokenA, tokenB);
        
        // Transfer tokens to pool
        TigerSwapSafeERC20.safeTransferFrom(tokenA, msg.sender, pool, amountA);
        TigerSwapSafeERC20.safeTransferFrom(tokenB, msg.sender, pool, amountB);
        
        // Mint LP tokens
        liquidity = ITigerSwapPool(pool).mint(to);
        
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
        address pool = ITigerSwapFactory(factory).getPoolAddress(token, WETH);
        
        TigerSwapSafeERC20.safeTransferFrom(token, msg.sender, pool, amountToken);
        IWETH(WETH).deposit{value: amountETH}();
        IWETH(WETH).transfer(pool, amountETH);
        
        liquidity = ITigerSwapPool(pool).mint(to);
        
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
        address pool = ITigerSwapFactory(factory).getPoolAddress(tokenA, tokenB);
        require(pool != address(0), "TigerSwap: POOL_NOT_FOUND");
        
        // Transfer LP tokens
        TigerSwapSafeERC20.safeTransferFrom(pool, msg.sender, pool, liquidity);
        
        // Burn and get amounts
        (amountA, amountB) = ITigerSwapPool(pool).burn(to);
        
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
        
        address pool = ITigerSwapFactory(factory).getPoolAddress(path[0], path[1]);
        require(pool != address(0), "TigerSwap: POOL_NOT_FOUND");
        TigerSwapSafeERC20.safeTransferFrom(path[0], msg.sender, pool, amounts[0]);
        _swap(amounts, path, to);
    }

    /**
     * @notice Swap exact tokens for tokens through a specific pool family.
     * @dev Supports the xyK (0), stable (2) and weighted (3) families, which are
     *      all pull-based and need no callback. Concentrated liquidity (1) is
     *      not supported here because it settles in a callback.
     * @param tokenIn Input token
     * @param tokenOut Output token
     * @param family Family id: 0 xyK, 2 stable, 3 weighted
     * @param amountIn Amount of input tokens
     * @param amountOutMin Minimum output tokens
     * @param to Recipient address
     * @param deadline Deadline timestamp
     * @return amountOut Amount of output tokens
     */
    function swapExactTokensForTokens(
        address tokenIn,
        address tokenOut,
        uint8 family,
        uint256 amountIn,
        uint256 amountOutMin,
        address to,
        uint256 deadline
    ) external ensure(deadline) returns (uint256 amountOut) {
        require(amountIn > 0, "TigerSwap: INSUFFICIENT_INPUT_AMOUNT");
        require(to != address(0), "TigerSwap: ZERO_ADDRESS");
        require(tokenIn != tokenOut, "TigerSwap: IDENTICAL_ADDRESSES");

        address pool = ITigerSwapFactory(factory).getPoolForFamily(tokenIn, tokenOut, family);
        require(pool != address(0), "TigerSwap: POOL_NOT_FOUND");

        if (family == 0) {
            uint256[] memory amounts = getAmountsOut(amountIn, _pairPath(tokenIn, tokenOut));
            amountOut = amounts[1];
            TigerSwapSafeERC20.safeTransferFrom(tokenIn, msg.sender, pool, amountIn);
            (address token0,) = sortTokens(tokenIn, tokenOut);
            (uint256 amount0Out, uint256 amount1Out) =
                tokenIn == token0 ? (uint256(0), amountOut) : (amountOut, uint256(0));
            ITigerSwapPool(pool).swap(amount0Out, amount1Out, to, new bytes(0));
        } else if (family == 2) {
            (int128 i, int128 j) = _stableIndices(pool, tokenIn, tokenOut);
            TigerSwapSafeERC20.safeTransferFrom(tokenIn, msg.sender, pool, amountIn);
            amountOut = ITigerSwapStablePool(pool).exchange(i, j, amountIn, amountOutMin);
        } else if (family == 3) {
            (uint256 i, uint256 j) = _weightedIndices(pool, tokenIn, tokenOut);
            TigerSwapSafeERC20.safeTransferFrom(tokenIn, msg.sender, pool, amountIn);
            amountOut = ITigerSwapWeightedPool(pool).swap(i, j, amountIn, amountOutMin);
        } else {
            revert("TigerSwap: UNSUPPORTED_FAMILY");
        }

        require(amountOut >= amountOutMin, "TigerSwap: INSUFFICIENT_OUTPUT_AMOUNT");
        emit Swap(msg.sender, tokenIn, tokenOut, amountIn, amountOut);
    }

    function _pairPath(address tokenIn, address tokenOut) private pure returns (address[] memory path) {
        path = new address[](2);
        path[0] = tokenIn;
        path[1] = tokenOut;
    }

    /**
     * @notice Swap an exact input along a multi-token path, one pool family per
     *         hop, atomically in a single transaction.
     * @dev A path step is (tokenIn, tokenOut, family). Intermediate output is
     *      held by this router between hops, because the stable and weighted
     *      families pay their own `msg.sender` and cannot forward to a pool
     *      directly. Every hop is slippage-checked against its own minimum, and
     *      the final minimum also bounds the whole path, so a bad hop reverts
     *      the entire swap instead of leaving a half-filled order.
     * @param amountIn Exact input amount
     * @param amountOutMin Minimum final output for the whole path
     * @param tokens Token path; length must equal steps.length + 1
     * @param families Family id per hop: 0 xyK, 1 concentrated, 2 stable, 3 weighted
     * @param minOutPerHop Minimum output for each hop, so an intermediate hop
     *                     cannot be sandwiched
     * @param to Recipient of the final output
     * @param deadline Deadline timestamp
     * @return amountOut Final output amount
     */
    function swapExactTokensForTokensMultihop(
        uint256 amountIn,
        uint256 amountOutMin,
        address[] calldata tokens,
        uint8[] calldata families,
        uint256[] calldata minOutPerHop,
        address to,
        uint256 deadline
    ) external ensure(deadline) lock returns (uint256 amountOut) {
        require(amountIn > 0, "TigerSwap: INSUFFICIENT_INPUT_AMOUNT");
        require(to != address(0), "TigerSwap: ZERO_ADDRESS");

        amountOut = _executePath(tokens, families, amountIn, minOutPerHop, to);

        require(amountOut >= amountOutMin, "TigerSwap: INSUFFICIENT_OUTPUT_AMOUNT");
        emit Swap(msg.sender, tokens[0], tokens[tokens.length - 1], amountIn, amountOut);
        return amountOut;
    }

    /**
     * @notice Swap an exact input split across several independent paths in a
     *         single transaction.
     * @dev The router's smart order router divides a large order across pools so
     *      no single pool takes the whole price impact. Each leg is a full path
     *      (tokens[l], families[l], minOutPerHop[l]) and the legs must be pool
     *      disjoint, which the off-chain router guarantees. Every leg is
     *      slippage-checked on its own, and the summed output must clear
     *      amountOutMin, so a leg that fills worse than quoted reverts the whole
     *      swap rather than leaving a partial fill.
     * @param amountIn Total exact input; must equal the sum of amountInPerLeg
     * @param amountOutMin Minimum total output across all legs
     * @param tokens Token path per leg
     * @param families Family id per hop, per leg
     * @param amountInPerLeg Input for each leg
     * @param minOutPerHop Minimum output per hop, per leg
     * @param to Recipient of the summed output
     * @param deadline Deadline timestamp
     * @return amountOut Summed output across all legs
     */
    function swapExactTokensForTokensSplit(
        uint256 amountIn,
        uint256 amountOutMin,
        address[][] calldata tokens,
        uint8[][] calldata families,
        uint256[] calldata amountInPerLeg,
        uint256[][] calldata minOutPerHop,
        address to,
        uint256 deadline
    ) external ensure(deadline) lock returns (uint256 amountOut) {
        require(to != address(0), "TigerSwap: ZERO_ADDRESS");
        uint256 legs = tokens.length;
        require(legs >= 1, "TigerSwap: NO_LEGS");
        require(
            families.length == legs && amountInPerLeg.length == legs && minOutPerHop.length == legs,
            "TigerSwap: LEG_LENGTH_MISMATCH"
        );

        uint256 totalIn;
        for (uint256 l = 0; l < legs; l++) {
            require(tokens[l].length >= 2, "TigerSwap: INVALID_PATH");
            require(families[l].length == tokens[l].length - 1, "TigerSwap: PATH_LENGTH_MISMATCH");
            require(minOutPerHop[l].length == families[l].length, "TigerSwap: PATH_LENGTH_MISMATCH");
            totalIn += amountInPerLeg[l];
        }
        require(totalIn == amountIn, "TigerSwap: INPUT_MISMATCH");

        uint256 totalOut;
        for (uint256 l = 0; l < legs; l++) {
            totalOut += _executePath(tokens[l], families[l], amountInPerLeg[l], minOutPerHop[l], to);
        }

        require(totalOut >= amountOutMin, "TigerSwap: INSUFFICIENT_OUTPUT_AMOUNT");
        emit Swap(msg.sender, tokens[0][0], tokens[0][tokens[0].length - 1], amountIn, totalOut);
        return totalOut;
    }

    /**
     * @dev Pull `amountIn` of the path's first token from the caller and trade it
     *      along `tokens`/`families`, one pool per hop, delivering the final
     *      output to `to`. Each hop's own `minOutPerHop` bound is enforced.
     */
    function _executePath(
        address[] memory tokens,
        uint8[] memory families,
        uint256 amountIn,
        uint256[] memory minOutPerHop,
        address to
    ) private returns (uint256 amountOut) {
        require(amountIn > 0, "TigerSwap: INSUFFICIENT_INPUT_AMOUNT");
        require(tokens.length >= 2, "TigerSwap: INVALID_PATH");
        require(families.length == tokens.length - 1, "TigerSwap: PATH_LENGTH_MISMATCH");
        require(minOutPerHop.length == families.length, "TigerSwap: PATH_LENGTH_MISMATCH");

        // Pull the full leg input from the caller up front.
        TigerSwapSafeERC20.safeTransferFrom(tokens[0], msg.sender, address(this), amountIn);

        uint256 current = amountIn;
        for (uint256 i = 0; i < families.length; i++) {
            address tokenIn = tokens[i];
            address tokenOut = tokens[i + 1];
            require(tokenIn != tokenOut, "TigerSwap: IDENTICAL_ADDRESSES");

            address pool = ITigerSwapFactory(factory).getPoolForFamily(tokenIn, tokenOut, families[i]);
            require(pool != address(0), "TigerSwap: POOL_NOT_FOUND");

            // The last hop pays the caller's recipient directly; an intermediate
            // hop is held here so the next hop can spend it.
            address hopRecipient = i == families.length - 1 ? to : address(this);

            current = _swapViaPool(pool, families[i], tokenIn, tokenOut, current, minOutPerHop[i], hopRecipient);
            require(current >= minOutPerHop[i], "TigerSwap: INSUFFICIENT_OUTPUT_AMOUNT");
        }
        return current;
    }

    /**
     * @dev Execute one hop through a family pool, moving `amountIn` of `tokenIn`
     *      from this router into the pool first, and delivering the output to
     *      `recipient`.
     *
     *      The xyK family reads its input as the pool's balance delta and pays
     *      its `to` argument directly, so the router sends the tokens in and the
     *      pool pays the recipient. The concentrated family also pays its own
     *      `recipient` argument. The stable and weighted families always pay
     *      their own `msg.sender`, so when the recipient is not this router the
     *      router receives the output and forwards it.
     *
     *      The stable, weighted and concentrated families pull from `msg.sender`
     *      with `transferFrom`, so the router approves exactly `amountIn` and
     *      clears the allowance after, so no standing approval survives the hop.
     */
    function _swapViaPool(
        address pool,
        uint8 family,
        address tokenIn,
        address tokenOut,
        uint256 amountIn,
        uint256 minOut,
        address recipient
    ) private returns (uint256 amountOut) {
        if (family == 0) {
            uint256[] memory amounts = getAmountsOut(amountIn, _pairPath(tokenIn, tokenOut));
            amountOut = amounts[1];
            TigerSwapSafeERC20.safeTransfer(tokenIn, pool, amountIn);
            (address token0,) = sortTokens(tokenIn, tokenOut);
            (uint256 amount0Out, uint256 amount1Out) =
                tokenIn == token0 ? (uint256(0), amountOut) : (amountOut, uint256(0));
            ITigerSwapPool(pool).swap(amount0Out, amount1Out, recipient, new bytes(0));
        } else if (family == 1) {
            TigerSwapSafeERC20.safeApprove(tokenIn, pool, amountIn);
            bool zeroForOne = tokenIn == ITigerSwapConcentratedPool(pool).token0();
            (int256 amount0, int256 amount1) =
                ITigerSwapConcentratedPool(pool).swap(recipient, zeroForOne, int256(amountIn), 0);
            TigerSwapSafeERC20.safeApprove(tokenIn, pool, 0);
            int256 delta = zeroForOne ? amount1 : amount0;
            require(delta < 0, "TigerSwap: V3_NO_OUTPUT");
            amountOut = uint256(-delta);
        } else if (family == 2) {
            (int128 i, int128 j) = _stableIndices(pool, tokenIn, tokenOut);
            TigerSwapSafeERC20.safeApprove(tokenIn, pool, amountIn);
            amountOut = ITigerSwapStablePool(pool).exchange(i, j, amountIn, minOut);
            TigerSwapSafeERC20.safeApprove(tokenIn, pool, 0);
            if (recipient != address(this)) {
                TigerSwapSafeERC20.safeTransfer(tokenOut, recipient, amountOut);
            }
        } else if (family == 3) {
            (uint256 i, uint256 j) = _weightedIndices(pool, tokenIn, tokenOut);
            TigerSwapSafeERC20.safeApprove(tokenIn, pool, amountIn);
            amountOut = ITigerSwapWeightedPool(pool).swap(i, j, amountIn, minOut);
            TigerSwapSafeERC20.safeApprove(tokenIn, pool, 0);
            if (recipient != address(this)) {
                TigerSwapSafeERC20.safeTransfer(tokenOut, recipient, amountOut);
            }
        } else {
            revert("TigerSwap: UNSUPPORTED_FAMILY");
        }
        require(amountOut >= minOut, "TigerSwap: INSUFFICIENT_OUTPUT_AMOUNT");
        return amountOut;
    }

    function _stableIndices(address pool, address tokenIn, address tokenOut)
        private
        view
        returns (int128 i, int128 j)
    {
        address coin0 = ITigerSwapStablePool(pool).coins(0);
        address coin1 = ITigerSwapStablePool(pool).coins(1);
        if (tokenIn == coin0 && tokenOut == coin1) return (0, 1);
        if (tokenIn == coin1 && tokenOut == coin0) return (1, 0);
        revert("TigerSwap: TOKEN_NOT_IN_POOL");
    }

    function _weightedIndices(address pool, address tokenIn, address tokenOut)
        private
        view
        returns (uint256 i, uint256 j)
    {
        uint256 count = ITigerSwapWeightedPool(pool).tokenCount();
        bool foundIn;
        bool foundOut;
        for (uint256 k = 0; k < count; k++) {
            address t = ITigerSwapWeightedPool(pool).tokens(k);
            if (t == tokenIn) {
                i = k;
                foundIn = true;
            }
            if (t == tokenOut) {
                j = k;
                foundOut = true;
            }
        }
        require(foundIn && foundOut, "TigerSwap: TOKEN_NOT_IN_POOL");
    }

    /**
     * @notice Add liquidity to a pool family. Supports xyK (0), stable (2) and
     *         weighted (3). Concentrated liquidity (1) is not handled here.
     * @dev Stable and weighted pools pull their tokens from msg.sender and mint
     *      LP to msg.sender, so the router cannot intermediate for them. This
     *      path is therefore restricted to the caller's own liquidity.
     * @return amountA Amount of tokenA used
     * @return amountB Amount of tokenB used
     * @return liquidity LP tokens minted
     */
    function addLiquidity(
        address tokenA,
        address tokenB,
        uint8 family,
        uint256 amountADesired,
        uint256 amountBDesired,
        uint256 amountAMin,
        uint256 amountBMin,
        address to,
        uint256 deadline
    ) external ensure(deadline) returns (uint256 amountA, uint256 amountB, uint256 liquidity) {
        address pool = ITigerSwapFactory(factory).getPoolForFamily(tokenA, tokenB, family);
        require(pool != address(0), "TigerSwap: POOL_NOT_FOUND");

        if (family == 0) {
            require(to != address(0), "TigerSwap: ZERO_ADDRESS");
            (amountA, amountB) = _addLiquidity(tokenA, tokenB, amountADesired, amountBDesired, amountAMin, amountBMin);
            TigerSwapSafeERC20.safeTransferFrom(tokenA, msg.sender, pool, amountA);
            TigerSwapSafeERC20.safeTransferFrom(tokenB, msg.sender, pool, amountB);
            liquidity = ITigerSwapPool(pool).mint(to);
        } else if (family == 2) {
            require(to == msg.sender, "TigerSwap: SELF_ONLY");
            (int128 i, int128 j) = _stableIndices(pool, tokenA, tokenB);
            uint256[2] memory amounts;
            amounts[uint256(uint128(i))] = amountADesired;
            amounts[uint256(uint128(j))] = amountBDesired;
            liquidity = ITigerSwapStablePool(pool).addLiquidity(amounts, 0);
            amountA = amountADesired;
            amountB = amountBDesired;
        } else if (family == 3) {
            require(to == msg.sender, "TigerSwap: SELF_ONLY");
            (uint256 i, uint256 j) = _weightedIndices(pool, tokenA, tokenB);
            uint256 count = ITigerSwapWeightedPool(pool).tokenCount();
            uint256[] memory amounts = new uint256[](count);
            amounts[i] = amountADesired;
            amounts[j] = amountBDesired;
            liquidity = ITigerSwapWeightedPool(pool).addLiquidity(amounts, 0);
            amountA = amountADesired;
            amountB = amountBDesired;
        } else {
            revert("TigerSwap: UNSUPPORTED_FAMILY");
        }

        emit LiquidityAdded(to, amountA, amountB);
    }

    /**
     * @notice Remove liquidity from a pool family. Supports xyK (0), stable (2)
     *         and weighted (3). Family pools burn LP from and pay out to
     *         msg.sender.
     */
    function removeLiquidity(
        address tokenA,
        address tokenB,
        uint8 family,
        uint256 liquidity,
        uint256 amountAMin,
        uint256 amountBMin,
        address to,
        uint256 deadline
    ) external ensure(deadline) returns (uint256 amountA, uint256 amountB) {
        address pool = ITigerSwapFactory(factory).getPoolForFamily(tokenA, tokenB, family);
        require(pool != address(0), "TigerSwap: POOL_NOT_FOUND");

        if (family == 0) {
            require(to != address(0), "TigerSwap: ZERO_ADDRESS");
            TigerSwapSafeERC20.safeTransferFrom(pool, msg.sender, pool, liquidity);
            (amountA, amountB) = ITigerSwapPool(pool).burn(to);
        } else if (family == 2) {
            require(to == msg.sender, "TigerSwap: SELF_ONLY");
            (int128 i, int128 j) = _stableIndices(pool, tokenA, tokenB);
            uint256[2] memory mins;
            mins[uint256(uint128(i))] = amountAMin;
            mins[uint256(uint128(j))] = amountBMin;
            uint256[2] memory amounts = ITigerSwapStablePool(pool).removeLiquidity(liquidity, mins);
            amountA = amounts[uint256(uint128(i))];
            amountB = amounts[uint256(uint128(j))];
        } else if (family == 3) {
            require(to == msg.sender, "TigerSwap: SELF_ONLY");
            (uint256 i, uint256 j) = _weightedIndices(pool, tokenA, tokenB);
            uint256 count = ITigerSwapWeightedPool(pool).tokenCount();
            uint256[] memory mins = new uint256[](count);
            mins[i] = amountAMin;
            mins[j] = amountBMin;
            uint256[] memory amounts = ITigerSwapWeightedPool(pool).removeLiquidity(liquidity, mins);
            amountA = amounts[i];
            amountB = amounts[j];
        } else {
            revert("TigerSwap: UNSUPPORTED_FAMILY");
        }

        require(amountA >= amountAMin, "TigerSwap: INSUFFICIENT_A");
        require(amountB >= amountBMin, "TigerSwap: INSUFFICIENT_B");
        emit LiquidityRemoved(to, amountA, amountB);
    }

    /**
     * @notice Get output amounts for input amount
     */
    function getAmountsOut(uint256 amountIn, address[] memory path) public view returns (uint256[] memory amounts) {
        require(path.length >= 2, "TigerSwap: INVALID_PATH");
        amounts = new uint256[](path.length);
        amounts[0] = amountIn;
        
        for (uint256 i = 0; i < path.length - 1; i++) {
            address pool = ITigerSwapFactory(factory).getPoolAddress(path[i], path[i + 1]);
            require(pool != address(0), "TigerSwap: POOL_NOT_FOUND");
            
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
            address pool = ITigerSwapFactory(factory).getPoolAddress(path[i - 1], path[i]);
            require(pool != address(0), "TigerSwap: POOL_NOT_FOUND");
            (uint256 reserve0, uint256 reserve1) = getReserves(pool, path[i - 1], path[i]);
            amounts[i - 1] = getAmountIn(amounts[i], reserve0, reserve1);
        }
    }

    /**
     * @notice Calculate output amount for input
     */
    function getAmountOut(uint256 amountIn, uint256 reserveIn, uint256 reserveOut) internal view returns (uint256) {
        require(amountIn > 0, "TigerSwap: INSUFFICIENT_INPUT_AMOUNT");
        require(reserveIn > 0 && reserveOut > 0, "TigerSwap: INSUFFICIENT_LIQUIDITY");
        
        uint256 fee = ITigerSwapFactory(factory).swapFee();
        require(fee < FEE_DENOMINATOR, "TigerSwap: FEE_TOO_HIGH");
        uint256 amountInWithFee = amountIn * (FEE_DENOMINATOR - fee);
        uint256 numerator = amountInWithFee * reserveOut;
        uint256 denominator = reserveIn * FEE_DENOMINATOR + amountInWithFee;
        
        return numerator / denominator;
    }

    /**
     * @notice Calculate input amount for output
     */
    function getAmountIn(uint256 amountOut, uint256 reserveIn, uint256 reserveOut) internal view returns (uint256) {
        require(amountOut > 0, "TigerSwap: INSUFFICIENT_OUTPUT_AMOUNT");
        require(reserveIn > 0 && reserveOut > 0, "TigerSwap: INSUFFICIENT_LIQUIDITY");
        
        uint256 fee = ITigerSwapFactory(factory).swapFee();
        require(fee < FEE_DENOMINATOR, "TigerSwap: FEE_TOO_HIGH");
        require(amountOut < reserveOut, "TigerSwap: INSUFFICIENT_LIQUIDITY");
        uint256 numerator = reserveIn * amountOut * FEE_DENOMINATOR;
        uint256 denominator = (reserveOut - amountOut) * (FEE_DENOMINATOR - fee);
        
        return numerator / denominator + 1;
    }

    /**
     * @notice Internal swap function
     */
    function _swap(uint256[] memory amounts, address[] memory path, address to) internal {
        for (uint256 i = 0; i < path.length - 1; i++) {
            (address input, address output) = (path[i], path[i + 1]);
            (address token0, ) = sortTokens(input, output);
            address pool = ITigerSwapFactory(factory).getPoolAddress(input, output);
            require(pool != address(0), "TigerSwap: POOL_NOT_FOUND");

            uint256 amountOut = amounts[i + 1];
            (uint256 amount0Out, uint256 amount1Out) =
                input == token0 ? (uint256(0), amountOut) : (amountOut, uint256(0));

            // Intermediate hops forward to the next pool; the final hop pays `to`.
            address recipient = i < path.length - 2
                ? ITigerSwapFactory(factory).getPoolAddress(output, path[i + 2])
                : to;
            require(recipient != address(0), "TigerSwap: POOL_NOT_FOUND");

            ITigerSwapPool(pool).swap(amount0Out, amount1Out, recipient, new bytes(0));
        }
    }

    /**
     * @notice Get reserves from pool
     */
    function getReserves(address pool, address tokenA, address tokenB) internal view returns (uint256 reserveA, uint256 reserveB) {
        (address token0, ) = sortTokens(tokenA, tokenB);
        (uint256 reserve0, uint256 reserve1, ) = ITigerSwapPool(pool).getReserves();
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
        address pool = ITigerSwapFactory(factory).getPoolAddress(tokenA, tokenB);
        
        if (pool == address(0)) {
            ITigerSwapFactory(factory).createPool(tokenA, tokenB);
            pool = ITigerSwapFactory(factory).getPoolAddress(tokenA, tokenB);
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
