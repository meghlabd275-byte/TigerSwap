// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./TigerSwapConcentrated.sol";
import "./TigerSwapInterfaces.sol";

/**
 * @title TigerSwapQuoter
 * @notice Read-only price quotes for TigerSwap concentrated-liquidity pools.
 *
 * The quote walks the same tick-by-tick swap loop the pool executes, but never
 * moves a balance: the pool's public state is copied into memory and the
 * simulation advances only that copy. A quote therefore costs no token
 * approval and cannot alter the pool, so it is safe to expose as a view.
 *
 * Quoting is only exact while the price stays inside the simulated range. The
 * loop has no real tick bitmap to cross, so it reports the amount that the
 * liquidity available at the current price can absorb and stops at the nearest
 * initialised tick, mirroring the pool's own boundary logic.
 */
contract TigerSwapQuoter {
    uint256 internal constant FEE_DENOMINATOR = 1_000_000;

    address public immutable factory;

    /// @notice Family id for concentrated-liquidity pools, matching the router.
    uint8 internal constant FAMILY_V3 = 1;

    error IdenticalTokens();
    error ZeroAddress();
    error InvalidFee(uint24 fee);
    error UnknownPool(address tokenIn, address tokenOut);
    error NoLiquidity();

    constructor(address _factory) {
        if (_factory == address(0)) revert ZeroAddress();
        factory = _factory;
    }

    /// @notice Quote an exact-input swap on one concentrated pool.
    /// @param tokenIn Token sold.
    /// @param tokenOut Token bought.
    /// @param fee Fee tier in hundredths of a bip; must match the pool's own fee.
    /// @param amountIn Exact amount of `tokenIn` to sell.
    /// @return amountOut Amount of `tokenOut` the pool would return.
    function quoteExactInputSingle(address tokenIn, address tokenOut, uint24 fee, uint256 amountIn)
        external
        view
        returns (uint256 amountOut)
    {
        if (tokenIn == tokenOut) revert IdenticalTokens();
        if (tokenIn == address(0) || tokenOut == address(0)) revert ZeroAddress();
        if (fee >= FEE_DENOMINATOR) revert InvalidFee(fee);
        if (amountIn == 0) return 0;

        address pool = ITigerSwapFactory(factory).getPoolForFamily(tokenIn, tokenOut, FAMILY_V3);
        if (pool == address(0)) revert UnknownPool(tokenIn, tokenOut);

        return _quotePool(pool, tokenIn, fee, amountIn);
    }

    /// @notice Quote an exact-input swap across a multi-hop path.
    /// @param path Packed hops of 23 bytes: tokenIn (20) | fee (3, big-endian).
    ///             The final token is appended as a bare 20-byte address, so the
    ///             encoded length is `23 * (hops - 1) + 20`.
    /// @param amountIn Exact amount of the first token to sell.
    /// @return amountOut Amount of the last token the path would return.
    function quoteExactInput(bytes calldata path, uint256 amountIn) external view returns (uint256 amountOut) {
        if (path.length < 43) revert UnknownPool(address(0), address(0));

        uint256 current = amountIn;
        uint256 offset = 0;

        // Each iteration consumes one hop: 20-byte token, 3-byte fee.
        while (offset + 23 <= path.length - 20) {
            address tokenIn = _toAddress(path[offset:offset + 20]);
            uint24 hopFee = uint24((uint256(uint8(path[offset + 20])) << 16)
                | (uint256(uint8(path[offset + 21])) << 8)
                | uint256(uint8(path[offset + 22])));
            address tokenOut = _toAddress(path[offset + 23:offset + 43]);

            if (tokenIn == tokenOut) revert IdenticalTokens();
            if (hopFee >= FEE_DENOMINATOR) revert InvalidFee(hopFee);

            address pool = ITigerSwapFactory(factory).getPoolForFamily(tokenIn, tokenOut, FAMILY_V3);
            if (pool == address(0)) revert UnknownPool(tokenIn, tokenOut);

            current = _quotePool(pool, tokenIn, hopFee, current);
            offset += 23;
        }

        return current;
    }

    /// @dev Simulates one hop. Reverts on a pool whose fee or token order does
    ///      not match the request rather than returning a wrong number.
    function _quotePool(address pool, address tokenIn, uint24 fee, uint256 amountIn)
        private
        view
        returns (uint256 amountOut)
    {
        TigerSwapConcentrated p = TigerSwapConcentrated(pool);

        if (p.fee() != fee) revert InvalidFee(fee);

        bool zeroForOne = p.token0() == tokenIn;
        if (!zeroForOne && p.token1() != tokenIn) revert UnknownPool(tokenIn, p.token0());

        uint128 poolLiquidity = p.liquidity();
        if (poolLiquidity == 0) revert NoLiquidity();

        uint160 sqrtPriceX96 = p.sqrtPriceX96();
        int24 tickCurrent = p.tick();
        int24 spacing = p.tickSpacing();

        // Mirror the pool: an empty price limit means "no limit" and is then
        // pinned one step inside the representable range.
        uint160 sqrtPriceLimitX96 = zeroForOne
            ? TickMath.MIN_SQRT_RATIO + 1
            : TickMath.MAX_SQRT_RATIO - 1;

        int256 amountRemaining = int256(amountIn);
        uint256 amountCalculated = 0;
        uint128 liquidity = poolLiquidity;

        while (amountRemaining > 0 && sqrtPriceX96 != sqrtPriceLimitX96) {
            (int24 tickNext, bool initialized) = _nextInitializedTickWithinOneWord(p, tickCurrent, spacing, zeroForOne);

            if (tickNext < TickMath.MIN_TICK) tickNext = TickMath.MIN_TICK;
            else if (tickNext > TickMath.MAX_TICK) tickNext = TickMath.MAX_TICK;

            uint160 sqrtPriceNextX96 = TickMath.getSqrtRatioAtTick(tickNext);

            uint160 targetX96 = (zeroForOne ? sqrtPriceNextX96 < sqrtPriceLimitX96 : sqrtPriceNextX96 > sqrtPriceLimitX96)
                ? sqrtPriceLimitX96
                : sqrtPriceNextX96;

            (uint160 sqrtRatioNextX96, uint256 stepIn, uint256 stepOut, uint256 stepFee) =
                SwapMath.computeSwapStep(sqrtPriceX96, targetX96, liquidity, amountRemaining, fee);

            // The pool charges the fee on top of the traded input.
            amountRemaining -= int256(stepIn + stepFee);
            amountCalculated += stepOut;

            if (sqrtRatioNextX96 == sqrtPriceNextX96) {
                if (initialized) {
                    int128 liquidityNet = p.getTickInfo(tickNext).liquidityNet;
                    liquidity = zeroForOne
                        ? uint128(int128(liquidity) - liquidityNet)
                        : uint128(int128(liquidity) + liquidityNet);
                }
                tickCurrent = zeroForOne ? tickNext - 1 : tickNext;
            } else if (sqrtRatioNextX96 != sqrtPriceX96) {
                tickCurrent = TickMath.getTickAtSqrtRatio(sqrtRatioNextX96);
            }

            sqrtPriceX96 = sqrtRatioNextX96;
        }

        amountOut = amountCalculated;
    }

    /// @dev Same word-walk the pool performs, reading the bitmap through the
    ///      pool's public getter.
    function _nextInitializedTickWithinOneWord(
        TigerSwapConcentrated p,
        int24 tickToSearch,
        int24 spacing,
        bool lte
    ) private view returns (int24 next, bool initialized) {
        int24 compressed = tickToSearch / spacing;
        if (tickToSearch < 0 && tickToSearch % spacing != 0) compressed--;

        if (lte) {
            (int16 wordPos, uint8 bitPos) = _bitPosition(compressed);
            uint256 mask = (uint256(1) << bitPos) - 1 + (uint256(1) << bitPos);
            uint256 masked = p.tickBitmap(wordPos) & mask;

            initialized = masked != 0;
            next = initialized
                ? (compressed - int24(uint24(bitPos - uint8(BitMath.mostSignificantBit(masked))))) * spacing
                : (compressed - int24(uint24(bitPos))) * spacing;
        } else {
            (int16 wordPos, uint8 bitPos) = _bitPosition(compressed + 1);
            uint256 mask = ~((uint256(1) << bitPos) - 1);
            uint256 masked = p.tickBitmap(wordPos) & mask;

            initialized = masked != 0;
            next = initialized
                ? (compressed + 1 + int24(uint24(uint8(BitMath.leastSignificantBit(masked)) - bitPos))) * spacing
                : (compressed + 1 + int24(uint24(type(uint8).max - bitPos))) * spacing;
        }
    }

    function _bitPosition(int24 compressed) private pure returns (int16 wordPos, uint8 bitPos) {
        wordPos = int16(compressed >> 8);
        bitPos = uint8(uint24(compressed % 256));
    }

    function _toAddress(bytes calldata raw) private pure returns (address result) {
        require(raw.length == 20, "TigerSwap: BAD_PATH");
        assembly {
            result := shr(96, calldataload(raw.offset))
        }
    }
}
