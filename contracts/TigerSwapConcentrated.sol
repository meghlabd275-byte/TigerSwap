// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./TigerSwapInterfaces.sol";

/**
 * @title TigerSwapConcentrated
 * @notice Concentrated liquidity pool (Uniswap V3 model): Q64.96 sqrt-price
 *         arithmetic, a tick bitmap, real tick crossing during swaps, and
 *         per-position fee growth.
 *
 * The swap loop is the canonical algorithm: for each step it computes the
 * largest price move the current liquidity can absorb, charges the fee, and if
 * the step reaches an initialised tick it crosses it (flipping the fee-growth
 * accumulators and adding that tick's net liquidity) before continuing.
 */
contract TigerSwapConcentrated {
    uint256 internal constant Q96 = 0x1000000000000000000000000;
    uint128 internal constant MAX_LIQUIDITY = type(uint128).max;
    uint256 internal constant FEE_DENOMINATOR = 1_000_000; // fee in hundredths of a bip

    address public factory;
    address public token0;
    address public token1;
    uint24 public fee;
    int24 public tickSpacing;

    uint160 public sqrtPriceX96;
    int24 public tick;

    uint128 public liquidity;
    mapping(int24 => TickInfo) public ticks;
    mapping(int16 => uint256) public tickBitmap;
    mapping(bytes32 => PositionInfo) public positions;

    uint256 public feeGrowthGlobal0X128;
    uint256 public feeGrowthGlobal1X128;

    uint256 public protocolFees0;
    uint256 public protocolFees1;
    address public protocolFeeRecipient;
    uint8 public protocolFee;

    Observation[65535] public observations;
    uint16 public observationIndex;
    uint16 public observationCardinality;
    uint16 public observationCardinalityNext;

    struct TickInfo {
        uint128 liquidityGross;
        int128 liquidityNet;
        uint256 feeGrowthOutside0X128;
        uint256 feeGrowthOutside1X128;
        int56 tickCumulativeOutside;
        uint160 secondsPerLiquidityOutsideX128;
        uint32 secondsOutside;
        bool initialized;
    }

    struct PositionInfo {
        uint128 liquidity;
        uint256 feeGrowthInside0LastX128;
        uint256 feeGrowthInside1LastX128;
        uint128 tokensOwed0;
        uint128 tokensOwed1;
    }

    struct Observation {
        uint32 blockTimestamp;
        int56 tickCumulative;
        uint160 secondsPerLiquidityCumulativeX128;
        bool initialized;
    }

    event Initialize(uint160 sqrtPriceX96, int24 tick);
    event Mint(address sender, address owner, int24 tickLower, int24 tickUpper, uint128 amount, uint256 amount0, uint256 amount1);
    event Burn(address sender, address owner, int24 tickLower, int24 tickUpper, uint128 amount, uint256 amount0, uint256 amount1);
    event Collect(address owner, address recipient, int24 tickLower, int24 tickUpper, uint128 amount0, uint128 amount1);
    event Swap(address sender, address recipient, int256 amount0, int256 amount1, uint160 sqrtPriceX96, uint128 liquidity, int24 tick);
    event Flash(address sender, address recipient, uint256 amount0, uint256 amount1, uint256 paid0, uint256 paid1);

    modifier onlyFactory() {
        require(msg.sender == factory, "TigerSwap: NOT_FACTORY");
        _;
    }

    constructor(address _factory, address _token0, address _token1, uint24 _fee, int24 _tickSpacing) {
        require(_token0 != _token1, "TigerSwap: IDENTICAL_TOKENS");
        require(_tickSpacing > 0, "TigerSwap: TICK_SPACING");
        require(uint256(_fee) < FEE_DENOMINATOR, "TigerSwap: FEE_TOO_HIGH");
        factory = _factory;
        token0 = _token0;
        token1 = _token1;
        fee = _fee;
        tickSpacing = _tickSpacing;

        observations[0] = Observation(uint32(block.timestamp), 0, 0, true);
        observationCardinality = 1;
        observationCardinalityNext = 1;
    }

    // ============ Initialise ============

    function initialize(uint160 _sqrtPriceX96) external onlyFactory {
        require(sqrtPriceX96 == 0, "TigerSwap: ALREADY_INITIALIZED");
        require(
            _sqrtPriceX96 >= TickMath.MIN_SQRT_RATIO && _sqrtPriceX96 <= TickMath.MAX_SQRT_RATIO,
            "TigerSwap: SQRT_PRICE_OUT_OF_RANGE"
        );

        sqrtPriceX96 = _sqrtPriceX96;
        tick = TickMath.getTickAtSqrtRatio(_sqrtPriceX96);
        observations[0].blockTimestamp = uint32(block.timestamp);

        emit Initialize(_sqrtPriceX96, tick);
    }

    // ============ Liquidity ============

    function mint(address recipient, int24 tickLower, int24 tickUpper, uint128 amount)
        external
        returns (uint256 amount0, uint256 amount1)
    {
        require(amount > 0, "TigerSwap: ZERO_AMOUNT");
        _checkRange(tickLower, tickUpper);

        int256 amount0Delta;
        int256 amount1Delta;
        (amount0Delta, amount1Delta) = _modifyPosition(recipient, tickLower, tickUpper, int128(amount));

        amount0 = uint256(amount0Delta);
        amount1 = uint256(amount1Delta);

        if (amount0 > 0) {
            require(IERC20(token0).transferFrom(msg.sender, address(this), amount0), "TigerSwap: TRANSFER_FAILED");
        }
        if (amount1 > 0) {
            require(IERC20(token1).transferFrom(msg.sender, address(this), amount1), "TigerSwap: TRANSFER_FAILED");
        }

        emit Mint(msg.sender, recipient, tickLower, tickUpper, amount, amount0, amount1);
    }

    function burn(int24 tickLower, int24 tickUpper, uint128 amount)
        external
        returns (uint256 amount0, uint256 amount1)
    {
        require(amount > 0, "TigerSwap: ZERO_AMOUNT");
        _checkRange(tickLower, tickUpper);

        (int256 amount0Delta, int256 amount1Delta) =
            _modifyPosition(msg.sender, tickLower, tickUpper, -int128(amount));

        // A burn returns negative deltas meaning tokens are owed to the caller.
        amount0 = uint256(-amount0Delta);
        amount1 = uint256(-amount1Delta);

        bytes32 key = _positionKey(msg.sender, tickLower, tickUpper);
        PositionInfo storage position = positions[key];

        uint128 owed0 = position.tokensOwed0;
        uint128 owed1 = position.tokensOwed1;
        position.tokensOwed0 = 0;
        position.tokensOwed1 = 0;

        amount0 = amount0 + owed0;
        amount1 = amount1 + owed1;

        if (amount0 > 0) {
            require(IERC20(token0).transfer(msg.sender, amount0), "TigerSwap: TRANSFER_FAILED");
        }
        if (amount1 > 0) {
            require(IERC20(token1).transfer(msg.sender, amount1), "TigerSwap: TRANSFER_FAILED");
        }

        emit Burn(msg.sender, msg.sender, tickLower, tickUpper, amount, amount0, amount1);
    }

    /// @notice Collect accrued swap fees without removing liquidity.
    function collect(address recipient, int24 tickLower, int24 tickUpper)
        external
        returns (uint128 amount0, uint128 amount1)
    {
        _checkRange(tickLower, tickUpper);

        bytes32 key = _positionKey(msg.sender, tickLower, tickUpper);
        PositionInfo storage position = positions[key];

        (uint256 inside0, uint256 inside1) = _feeGrowthInside(tickLower, tickUpper);
        if (position.liquidity > 0) {
            if (inside0 >= position.feeGrowthInside0LastX128) {
                position.tokensOwed0 += uint128(
                    ((inside0 - position.feeGrowthInside0LastX128) * uint256(position.liquidity)) >> 128
                );
            }
            if (inside1 >= position.feeGrowthInside1LastX128) {
                position.tokensOwed1 += uint128(
                    ((inside1 - position.feeGrowthInside1LastX128) * uint256(position.liquidity)) >> 128
                );
            }
        }
        position.feeGrowthInside0LastX128 = inside0;
        position.feeGrowthInside1LastX128 = inside1;

        amount0 = position.tokensOwed0;
        amount1 = position.tokensOwed1;
        position.tokensOwed0 = 0;
        position.tokensOwed1 = 0;

        if (amount0 > 0) {
            require(IERC20(token0).transfer(recipient, amount0), "TigerSwap: TRANSFER_FAILED");
        }
        if (amount1 > 0) {
            require(IERC20(token1).transfer(recipient, amount1), "TigerSwap: TRANSFER_FAILED");
        }

        emit Collect(msg.sender, recipient, tickLower, tickUpper, amount0, amount1);
    }

    // ============ Swap ============

    /// @param amountSpecified Positive for exact input, negative for exact output.
    function swap(address recipient, bool zeroForOne, int256 amountSpecified, uint160 sqrtPriceLimitX96)
        external
        returns (int256 amount0, int256 amount1)
    {
        require(amountSpecified != 0, "TigerSwap: ZERO_AMOUNT");

        bool exactInput = amountSpecified > 0;

        if (sqrtPriceLimitX96 == 0) {
            sqrtPriceLimitX96 = zeroForOne ? TickMath.MIN_SQRT_RATIO + 1 : TickMath.MAX_SQRT_RATIO - 1;
        }
        if (zeroForOne) {
            require(
                sqrtPriceLimitX96 < sqrtPriceX96 && sqrtPriceLimitX96 > TickMath.MIN_SQRT_RATIO,
                "TigerSwap: BAD_PRICE_LIMIT"
            );
        } else {
            require(
                sqrtPriceLimitX96 > sqrtPriceX96 && sqrtPriceLimitX96 < TickMath.MAX_SQRT_RATIO,
                "TigerSwap: BAD_PRICE_LIMIT"
            );
        }

        SwapState memory state = SwapState({
            amountSpecifiedRemaining: amountSpecified,
            amountCalculated: 0,
            sqrtPriceX96: sqrtPriceX96,
            tick: tick,
            liquidity: liquidity,
            feeGrowthGlobalX128: zeroForOne ? feeGrowthGlobal0X128 : feeGrowthGlobal1X128
        });

        while (state.amountSpecifiedRemaining != 0 && state.sqrtPriceX96 != sqrtPriceLimitX96) {
            StepComputations memory step;
            step.sqrtPriceStartX96 = state.sqrtPriceX96;

            (step.tickNext, step.initialized) =
                _nextInitializedTickWithinOneWord(state.tick, tickSpacing, zeroForOne);

            if (step.tickNext < TickMath.MIN_TICK) step.tickNext = TickMath.MIN_TICK;
            else if (step.tickNext > TickMath.MAX_TICK) step.tickNext = TickMath.MAX_TICK;

            step.sqrtPriceNextX96 = TickMath.getSqrtRatioAtTick(step.tickNext);

            (state.sqrtPriceX96, step.amountIn, step.amountOut, step.feeAmount) = SwapMath.computeSwapStep(
                state.sqrtPriceX96,
                (zeroForOne ? step.sqrtPriceNextX96 < sqrtPriceLimitX96 : step.sqrtPriceNextX96 > sqrtPriceLimitX96)
                    ? sqrtPriceLimitX96
                    : step.sqrtPriceNextX96,
                state.liquidity,
                state.amountSpecifiedRemaining,
                fee
            );

            if (exactInput) {
                state.amountSpecifiedRemaining -= int256(step.amountIn + step.feeAmount);
                state.amountCalculated -= int256(step.amountOut);
            } else {
                state.amountSpecifiedRemaining += int256(step.amountOut);
                state.amountCalculated += int256(step.amountIn + step.feeAmount);
            }

            if (state.liquidity > 0) {
                uint256 protocolShare = step.feeAmount * uint256(protocolFee) / FEE_DENOMINATOR;
                if (zeroForOne) {
                    protocolFees0 += protocolShare;
                } else {
                    protocolFees1 += protocolShare;
                }
                state.feeGrowthGlobalX128 += ((step.feeAmount - protocolShare) << 128) / state.liquidity;
            }

            // Cross the tick when the price reached it.
            if (state.sqrtPriceX96 == step.sqrtPriceNextX96) {
                if (step.initialized) {
                    int128 liquidityNet = _cross(
                        step.tickNext,
                        zeroForOne ? state.feeGrowthGlobalX128 : feeGrowthGlobal0X128,
                        zeroForOne ? feeGrowthGlobal1X128 : state.feeGrowthGlobalX128
                    );
                    state.liquidity = zeroForOne
                        ? uint128(int128(state.liquidity) - liquidityNet)
                        : uint128(int128(state.liquidity) + liquidityNet);
                }
                state.tick = zeroForOne ? step.tickNext - 1 : step.tickNext;
            } else if (state.sqrtPriceX96 != step.sqrtPriceStartX96) {
                state.tick = TickMath.getTickAtSqrtRatio(state.sqrtPriceX96);
            }
        }

        if (zeroForOne) {
            feeGrowthGlobal0X128 = state.feeGrowthGlobalX128;
        } else {
            feeGrowthGlobal1X128 = state.feeGrowthGlobalX128;
        }

        if (state.tick != tick) {
            sqrtPriceX96 = state.sqrtPriceX96;
            tick = state.tick;
        } else {
            sqrtPriceX96 = state.sqrtPriceX96;
        }
        liquidity = state.liquidity;

        (amount0, amount1) = zeroForOne == exactInput
            ? (amountSpecified - state.amountSpecifiedRemaining, state.amountCalculated)
            : (state.amountCalculated, amountSpecified - state.amountSpecifiedRemaining);

        // Settle: positive means the caller owes the pool, negative means the pool owes the caller.
        if (amount0 > 0) {
            require(IERC20(token0).transferFrom(msg.sender, address(this), uint256(amount0)), "TigerSwap: TRANSFER_FAILED");
        }
        if (amount1 > 0) {
            require(IERC20(token1).transferFrom(msg.sender, address(this), uint256(amount1)), "TigerSwap: TRANSFER_FAILED");
        }
        if (amount0 < 0) {
            require(IERC20(token0).transfer(recipient, uint256(-amount0)), "TigerSwap: TRANSFER_FAILED");
        }
        if (amount1 < 0) {
            require(IERC20(token1).transfer(recipient, uint256(-amount1)), "TigerSwap: TRANSFER_FAILED");
        }

        _writeObservation();

        emit Swap(msg.sender, recipient, amount0, amount1, sqrtPriceX96, liquidity, tick);
    }

    struct SwapState {
        int256 amountSpecifiedRemaining;
        int256 amountCalculated;
        uint160 sqrtPriceX96;
        int24 tick;
        uint128 liquidity;
        uint256 feeGrowthGlobalX128;
    }

    struct StepComputations {
        uint160 sqrtPriceStartX96;
        int24 tickNext;
        bool initialized;
        uint160 sqrtPriceNextX96;
        uint256 amountIn;
        uint256 amountOut;
        uint256 feeAmount;
    }

    // ============ Position management ============

    function _modifyPosition(address owner, int24 tickLower, int24 tickUpper, int128 liquidityDelta)
        internal
        returns (int256 amount0, int256 amount1)
    {
        bytes32 key = _positionKey(owner, tickLower, tickUpper);
        PositionInfo storage position = positions[key];

        (uint256 inside0, uint256 inside1) = _feeGrowthInside(tickLower, tickUpper);

        if (position.liquidity > 0 || liquidityDelta < 0) {
            if (inside0 >= position.feeGrowthInside0LastX128) {
                position.tokensOwed0 += uint128(
                    ((inside0 - position.feeGrowthInside0LastX128) * uint256(position.liquidity)) >> 128
                );
            }
            if (inside1 >= position.feeGrowthInside1LastX128) {
                position.tokensOwed1 += uint128(
                    ((inside1 - position.feeGrowthInside1LastX128) * uint256(position.liquidity)) >> 128
                );
            }
        }
        position.feeGrowthInside0LastX128 = inside0;
        position.feeGrowthInside1LastX128 = inside1;

        if (liquidityDelta != 0) {
            uint128 liquidityBefore = liquidity;

            if (tick < tickLower) {
                amount0 = int256(SwapMath.getAmount0Delta(
                    TickMath.getSqrtRatioAtTick(tickLower), TickMath.getSqrtRatioAtTick(tickUpper), liquidityDelta, true
                ));
            } else if (tick < tickUpper) {
                uint160 current = sqrtPriceX96;
                amount0 = int256(SwapMath.getAmount0Delta(
                    current, TickMath.getSqrtRatioAtTick(tickUpper), liquidityDelta, true
                ));
                amount1 = int256(SwapMath.getAmount1Delta(
                    TickMath.getSqrtRatioAtTick(tickLower), current, liquidityDelta, true
                ));
            } else {
                amount1 = int256(SwapMath.getAmount1Delta(
                    TickMath.getSqrtRatioAtTick(tickLower), TickMath.getSqrtRatioAtTick(tickUpper), liquidityDelta, true
                ));
            }

            _updateTick(tickLower, liquidityDelta, false, liquidityBefore);
            _updateTick(tickUpper, liquidityDelta, true, liquidityBefore);

            position.liquidity = LiquidityMath.addDelta(position.liquidity, liquidityDelta);

            liquidity = LiquidityMath.addDelta(liquidityBefore, liquidityDelta);
            require(liquidity <= MAX_LIQUIDITY, "TigerSwap: LIQUIDITY_OVERFLOW");
        }

        // A burn reports the amounts the pool owes back to the caller.
        if (liquidityDelta < 0) {
            amount0 = -amount0;
            amount1 = -amount1;
        }
    }

    function _updateTick(int24 tickToUpdate, int128 liquidityDelta, bool upper, uint128 liquidityBefore) internal {
        TickInfo storage t = ticks[tickToUpdate];

        uint128 liquidityGrossBefore = t.liquidityGross;
        uint128 liquidityGrossAfter = LiquidityMath.addDelta(liquidityGrossBefore, liquidityDelta);
        require(liquidityGrossAfter <= MAX_LIQUIDITY, "TigerSwap: LIQUIDITY_OVERFLOW");

        if (liquidityGrossBefore == 0) {
            if (tickToUpdate <= tick) {
                t.feeGrowthOutside0X128 = feeGrowthGlobal0X128;
                t.feeGrowthOutside1X128 = feeGrowthGlobal1X128;
                t.secondsPerLiquidityOutsideX128 = _secondsPerLiquidityCumulative();
                t.tickCumulativeOutside = _tickCumulative();
                t.secondsOutside = uint32(block.timestamp);
            }
            t.initialized = true;
            _flipTick(tickToUpdate, true);
        } else if (liquidityGrossAfter == 0) {
            t.initialized = false;
            _flipTick(tickToUpdate, false);
        }

        t.liquidityGross = liquidityGrossAfter;
        t.liquidityNet = upper ? t.liquidityNet - liquidityDelta : t.liquidityNet + liquidityDelta;
    }

    function _cross(int24 tickToCross, uint256 global0X128, uint256 global1X128)
        internal
        returns (int128 liquidityNet)
    {
        TickInfo storage t = ticks[tickToCross];

        // Crossing flips each accumulator against its global.
        t.feeGrowthOutside0X128 = global0X128 - t.feeGrowthOutside0X128;
        t.feeGrowthOutside1X128 = global1X128 - t.feeGrowthOutside1X128;

        t.secondsPerLiquidityOutsideX128 = _secondsPerLiquidityCumulative() - t.secondsPerLiquidityOutsideX128;
        t.tickCumulativeOutside = _tickCumulative() - t.tickCumulativeOutside;
        t.secondsOutside = uint32(block.timestamp) - t.secondsOutside;

        liquidityNet = t.liquidityNet;
    }

    function _feeGrowthInside(int24 tickLower, int24 tickUpper)
        internal
        view
        returns (uint256 inside0, uint256 inside1)
    {
        TickInfo storage lower = ticks[tickLower];
        TickInfo storage upper = ticks[tickUpper];

        (uint256 below0, uint256 below1) = tick >= tickLower
            ? (lower.feeGrowthOutside0X128, lower.feeGrowthOutside1X128)
            : (feeGrowthGlobal0X128 - lower.feeGrowthOutside0X128, feeGrowthGlobal1X128 - lower.feeGrowthOutside1X128);

        (uint256 above0, uint256 above1) = tick < tickUpper
            ? (upper.feeGrowthOutside0X128, upper.feeGrowthOutside1X128)
            : (feeGrowthGlobal0X128 - upper.feeGrowthOutside0X128, feeGrowthGlobal1X128 - upper.feeGrowthOutside1X128);

        inside0 = feeGrowthGlobal0X128 - below0 - above0;
        inside1 = feeGrowthGlobal1X128 - below1 - above1;
    }

    function _checkRange(int24 tickLower, int24 tickUpper) internal view {
        require(tickLower < tickUpper, "TigerSwap: INVALID_RANGE");
        require(tickLower >= TickMath.MIN_TICK && tickUpper <= TickMath.MAX_TICK, "TigerSwap: TICK_OUT_OF_RANGE");
        require(tickLower % tickSpacing == 0 && tickUpper % tickSpacing == 0, "TigerSwap: TICK_SPACING");
    }

    function _positionKey(address owner, int24 tickLower, int24 tickUpper) internal pure returns (bytes32) {
        return keccak256(abi.encodePacked(owner, tickLower, tickUpper));
    }

    // ============ Tick bitmap ============

    function _flipTick(int24 tickToFlip, bool add) internal {
        int24 compressed = tickToFlip / tickSpacing;
        if (tickToFlip < 0 && tickToFlip % tickSpacing != 0) compressed--;

        int16 wordPos = int16(compressed >> 8);
        uint8 bitPos = uint8(uint24(compressed % 256));

        if (add) {
            tickBitmap[wordPos] |= (uint256(1) << bitPos);
        } else {
            tickBitmap[wordPos] &= ~(uint256(1) << bitPos);
        }
    }

    function _nextInitializedTickWithinOneWord(int24 tickToSearch, int24 spacing, bool lte)
        internal
        view
        returns (int24 next, bool initialized)
    {
        int24 compressed = tickToSearch / spacing;
        if (tickToSearch < 0 && tickToSearch % spacing != 0) compressed--;

        if (lte) {
            (int16 wordPos, uint8 bitPos) = _bitPosition(compressed);
            uint256 mask = (uint256(1) << bitPos) - 1 + (uint256(1) << bitPos);
            uint256 masked = tickBitmap[wordPos] & mask;

            initialized = masked != 0;
            next = initialized
                ? (compressed - int24(uint24(bitPos - uint8(BitMath.mostSignificantBit(masked))))) * spacing
                : (compressed - int24(uint24(bitPos))) * spacing;
        } else {
            (int16 wordPos, uint8 bitPos) = _bitPosition(compressed + 1);
            uint256 mask = ~((uint256(1) << bitPos) - 1);
            uint256 masked = tickBitmap[wordPos] & mask;

            initialized = masked != 0;
            next = initialized
                ? (compressed + 1 + int24(uint24(uint8(BitMath.leastSignificantBit(masked)) - bitPos))) * spacing
                : (compressed + 1 + int24(uint24(type(uint8).max - bitPos))) * spacing;
        }
    }

    function _bitPosition(int24 compressed) internal pure returns (int16 wordPos, uint8 bitPos) {
        wordPos = int16(compressed >> 8);
        bitPos = uint8(uint24(compressed % 256));
    }

    // ============ Observations / TWAP ============

    function increaseObservationCardinalityNext(uint16 cardinalityNext) external {
        uint16 old = observationCardinalityNext;
        observationCardinalityNext = old >= cardinalityNext ? old : cardinalityNext;
    }

    function _writeObservation() internal {
        (uint16 index, uint16 cardinality, uint16 cardinalityNext) =
            (observationIndex, observationCardinality, observationCardinalityNext);

        if (cardinalityNext > cardinality && cardinalityNext > 1) {
            uint16 newCardinality = cardinality + 1;
            observations[newCardinality - 1] = _transform(
                observations[(index + 1) % newCardinality], uint32(block.timestamp)
            );
            observationCardinality = newCardinality;
            observationIndex = newCardinality - 1;
            return;
        }

        uint16 newIndex = (index + 1) % cardinality;
        observations[newIndex] = _transform(observations[newIndex], uint32(block.timestamp));
        observationIndex = newIndex;
    }

    function _transform(Observation memory last, uint32 timestamp) internal view returns (Observation memory) {
        uint32 delta = timestamp - last.blockTimestamp;
        return Observation({
            blockTimestamp: timestamp,
            tickCumulative: last.tickCumulative + int56(tick) * int56(uint56(delta)),
            secondsPerLiquidityCumulativeX128: last.secondsPerLiquidityCumulativeX128
                + uint160((uint256(delta) << 128) / (liquidity > 0 ? liquidity : 1)),
            initialized: true
        });
    }

    function _tickCumulative() internal view returns (int56) {
        return observations[observationIndex].tickCumulative;
    }

    function _secondsPerLiquidityCumulative() internal view returns (uint160) {
        return observations[observationIndex].secondsPerLiquidityCumulativeX128;
    }

    /// @notice Cumulative tick and seconds-per-liquidity `secondsAgo` in the past.
    function observe(uint32[] calldata secondsAgos)
        external
        view
        returns (int56[] memory tickCumulatives, uint160[] memory secondsPerLiquidityCumulativeX128s)
    {
        tickCumulatives = new int56[](secondsAgos.length);
        secondsPerLiquidityCumulativeX128s = new uint160[](secondsAgos.length);

        for (uint256 i = 0; i < secondsAgos.length; i++) {
            (tickCumulatives[i], secondsPerLiquidityCumulativeX128s[i]) = _observeSingle(secondsAgos[i]);
        }
    }

    function _observeSingle(uint32 secondsAgo)
        internal
        view
        returns (int56 tickCumulative, uint160 secondsPerLiquidity)
    {
        Observation memory last = observations[observationIndex];

        if (secondsAgo == 0) {
            return (last.tickCumulative, last.secondsPerLiquidityCumulativeX128);
        }

        uint32 target = uint32(block.timestamp) - secondsAgo;
        uint16 index = observationIndex;
        uint16 cardinality = observationCardinality;

        for (uint16 i = 0; i < cardinality; i++) {
            uint16 prev = index == 0 ? cardinality - 1 : index - 1;
            Observation memory beforeOrAt = observations[prev];

            if (beforeOrAt.blockTimestamp <= target) {
                uint32 delta = target - beforeOrAt.blockTimestamp;
                return (
                    beforeOrAt.tickCumulative + int56(tick) * int56(uint56(delta)),
                    beforeOrAt.secondsPerLiquidityCumulativeX128
                        + uint160((uint256(delta) << 128) / (liquidity > 0 ? liquidity : 1))
                );
            }
            index = prev;
        }

        return (last.tickCumulative, last.secondsPerLiquidityCumulativeX128);
    }

    // ============ Flash loans ============

    function flash(address recipient, uint256 amount0, uint256 amount1, bytes calldata data) external {
        require(amount0 > 0 || amount1 > 0, "TigerSwap: ZERO_AMOUNT");

        uint256 balance0Before = IERC20(token0).balanceOf(address(this));
        uint256 balance1Before = IERC20(token1).balanceOf(address(this));

        if (amount0 > 0) {
            require(IERC20(token0).transfer(recipient, amount0), "TigerSwap: TRANSFER_FAILED");
        }
        if (amount1 > 0) {
            require(IERC20(token1).transfer(recipient, amount1), "TigerSwap: TRANSFER_FAILED");
        }

        uint256 fee0 = amount0 * uint256(fee) / FEE_DENOMINATOR;
        uint256 fee1 = amount1 * uint256(fee) / FEE_DENOMINATOR;

        ITigerSwapFlashCallback(msg.sender).tigerSwapFlashCallback(amount0, amount1, fee0, fee1, data);

        require(
            IERC20(token0).balanceOf(address(this)) >= balance0Before + fee0,
            "TigerSwap: FLASH_NOT_REPAID_0"
        );
        require(
            IERC20(token1).balanceOf(address(this)) >= balance1Before + fee1,
            "TigerSwap: FLASH_NOT_REPAID_1"
        );

        uint256 protocol0 = fee0 * uint256(protocolFee) / FEE_DENOMINATOR;
        uint256 protocol1 = fee1 * uint256(protocolFee) / FEE_DENOMINATOR;
        protocolFees0 += protocol0;
        protocolFees1 += protocol1;

        if (liquidity > 0) {
            feeGrowthGlobal0X128 += ((fee0 - protocol0) << 128) / uint256(liquidity);
            feeGrowthGlobal1X128 += ((fee1 - protocol1) << 128) / uint256(liquidity);
        }

        emit Flash(msg.sender, recipient, amount0, amount1, fee0, fee1);
    }

    // ============ Protocol fees ============

    function setProtocolFee(uint8 _protocolFee) external onlyFactory {
        require(uint256(_protocolFee) <= FEE_DENOMINATOR / 4, "TigerSwap: PROTOCOL_FEE_TOO_HIGH");
        protocolFee = _protocolFee;
    }

    function setProtocolFeeRecipient(address _recipient) external onlyFactory {
        require(_recipient != address(0), "TigerSwap: ZERO_ADDRESS");
        protocolFeeRecipient = _recipient;
    }

    function collectProtocolFees(uint256 amount0, uint256 amount1) external returns (uint256, uint256) {
        require(msg.sender == protocolFeeRecipient, "TigerSwap: NOT_RECIPIENT");
        require(amount0 <= protocolFees0 && amount1 <= protocolFees1, "TigerSwap: AMOUNT_TOO_HIGH");

        if (amount0 > 0) {
            protocolFees0 -= amount0;
            require(IERC20(token0).transfer(protocolFeeRecipient, amount0), "TigerSwap: TRANSFER_FAILED");
        }
        if (amount1 > 0) {
            protocolFees1 -= amount1;
            require(IERC20(token1).transfer(protocolFeeRecipient, amount1), "TigerSwap: TRANSFER_FAILED");
        }

        return (amount0, amount1);
    }

    // ============ Views ============

    function slot0()
        external
        view
        returns (uint160, int24, uint16, uint16, uint16, uint8, bool)
    {
        return (
            sqrtPriceX96,
            tick,
            observationIndex,
            observationCardinality,
            observationCardinalityNext,
            protocolFee,
            false
        );
    }

    function getPosition(address owner, int24 tickLower, int24 tickUpper)
        external
        view
        returns (uint128 positionLiquidity, uint256 fg0, uint256 fg1, uint128 owed0, uint128 owed1)
    {
        PositionInfo storage position = positions[_positionKey(owner, tickLower, tickUpper)];
        return (
            position.liquidity,
            position.feeGrowthInside0LastX128,
            position.feeGrowthInside1LastX128,
            position.tokensOwed0,
            position.tokensOwed1
        );
    }

    function getTickInfo(int24 tickToRead) external view returns (TickInfo memory) {
        return ticks[tickToRead];
    }

    function getQuoteAtTick(int24 tickToQuote) external pure returns (uint160) {
        return TickMath.getSqrtRatioAtTick(tickToQuote);
    }

    function getTickAtSqrtRatio(uint160 sqrtRatioX96) external pure returns (int24) {
        return TickMath.getTickAtSqrtRatio(sqrtRatioX96);
    }
}

interface ITigerSwapFlashCallback {
    function tigerSwapFlashCallback(uint256 amount0, uint256 amount1, uint256 fee0, uint256 fee1, bytes calldata data)
        external;
}

library LiquidityMath {
    function addDelta(uint128 x, int128 y) internal pure returns (uint128 z) {
        if (y < 0) {
            require((z = x - uint128(uint128(-y))) <= x, "TigerSwap: LIQUIDITY_UNDERFLOW");
        } else {
            require((z = x + uint128(uint128(y))) >= x, "TigerSwap: LIQUIDITY_OVERFLOW");
        }
    }
}

library BitMath {
    function mostSignificantBit(uint256 x) internal pure returns (uint8 r) {
        require(x > 0, "TigerSwap: ZERO");
        if (x >= 0x100000000000000000000000000000000) {
            x >>= 128;
            r += 128;
        }
        if (x >= 0x10000000000000000) {
            x >>= 64;
            r += 64;
        }
        if (x >= 0x100000000) {
            x >>= 32;
            r += 32;
        }
        if (x >= 0x10000) {
            x >>= 16;
            r += 16;
        }
        if (x >= 0x100) {
            x >>= 8;
            r += 8;
        }
        if (x >= 0x10) {
            x >>= 4;
            r += 4;
        }
        if (x >= 0x4) {
            x >>= 2;
            r += 2;
        }
        if (x >= 0x2) r += 1;
    }

    function leastSignificantBit(uint256 x) internal pure returns (uint8 r) {
        require(x > 0, "TigerSwap: ZERO");
        if (x & type(uint128).max == 0) {
            x >>= 128;
            r += 128;
        }
        if (x & type(uint64).max == 0) {
            x >>= 64;
            r += 64;
        }
        if (x & type(uint32).max == 0) {
            x >>= 32;
            r += 32;
        }
        if (x & type(uint16).max == 0) {
            x >>= 16;
            r += 16;
        }
        if (x & type(uint8).max == 0) {
            x >>= 8;
            r += 8;
        }
        if (x & 0xf == 0) {
            x >>= 4;
            r += 4;
        }
        if (x & 0x3 == 0) {
            x >>= 2;
            r += 2;
        }
        if (x & 0x1 == 0) r += 1;
    }
}

/// @notice Swap step math: how far the price moves and what is paid for it.
library SwapMath {
    uint256 internal constant Q96 = 0x1000000000000000000000000;
    uint256 internal constant FEE_DENOMINATOR = 1_000_000;

    function computeSwapStep(
        uint160 sqrtRatioCurrentX96,
        uint160 sqrtRatioTargetX96,
        uint128 liquidityAmount,
        int256 amountRemaining,
        uint24 feePips
    ) internal pure returns (uint160 sqrtRatioNextX96, uint256 amountIn, uint256 amountOut, uint256 feeAmount) {
        bool zeroForOne = sqrtRatioCurrentX96 >= sqrtRatioTargetX96;
        bool exactIn = amountRemaining >= 0;

        if (exactIn) {
            uint256 amountRemainingLessFee = FullMath.mulDiv(
                uint256(amountRemaining), FEE_DENOMINATOR - feePips, FEE_DENOMINATOR
            );
            amountIn = zeroForOne
                ? getAmount0Delta(sqrtRatioTargetX96, sqrtRatioCurrentX96, int128(liquidityAmount), true)
                : getAmount1Delta(sqrtRatioCurrentX96, sqrtRatioTargetX96, int128(liquidityAmount), true);

            if (amountRemainingLessFee >= amountIn) {
                sqrtRatioNextX96 = sqrtRatioTargetX96;
            } else {
                sqrtRatioNextX96 = getNextSqrtPriceFromInput(
                    sqrtRatioCurrentX96, liquidityAmount, amountRemainingLessFee, zeroForOne
                );
            }
        } else {
            amountOut = zeroForOne
                ? getAmount1Delta(sqrtRatioTargetX96, sqrtRatioCurrentX96, int128(liquidityAmount), false)
                : getAmount0Delta(sqrtRatioCurrentX96, sqrtRatioTargetX96, int128(liquidityAmount), false);

            if (uint256(-amountRemaining) >= amountOut) {
                sqrtRatioNextX96 = sqrtRatioTargetX96;
            } else {
                sqrtRatioNextX96 = getNextSqrtPriceFromOutput(
                    sqrtRatioCurrentX96, liquidityAmount, uint256(-amountRemaining), zeroForOne
                );
            }
        }

        bool max = sqrtRatioTargetX96 == sqrtRatioNextX96;

        if (zeroForOne) {
            amountIn = max && exactIn
                ? amountIn
                : getAmount0Delta(sqrtRatioNextX96, sqrtRatioCurrentX96, int128(liquidityAmount), true);
            amountOut = max && !exactIn
                ? amountOut
                : getAmount1Delta(sqrtRatioNextX96, sqrtRatioCurrentX96, int128(liquidityAmount), false);
        } else {
            amountIn = max && exactIn
                ? amountIn
                : getAmount1Delta(sqrtRatioCurrentX96, sqrtRatioNextX96, int128(liquidityAmount), true);
            amountOut = max && !exactIn
                ? amountOut
                : getAmount0Delta(sqrtRatioCurrentX96, sqrtRatioNextX96, int128(liquidityAmount), false);
        }

        if (!exactIn && amountOut > uint256(-amountRemaining)) {
            amountOut = uint256(-amountRemaining);
        }

        if (exactIn && sqrtRatioNextX96 != sqrtRatioTargetX96) {
            feeAmount = uint256(amountRemaining) - amountIn;
        } else {
            feeAmount = FullMath.mulDivRoundingUp(amountIn, feePips, FEE_DENOMINATOR - feePips);
        }
    }

    function getNextSqrtPriceFromInput(
        uint160 sqrtPX96,
        uint128 liquidityAmount,
        uint256 amountIn,
        bool zeroForOne
    ) internal pure returns (uint160) {
        require(sqrtPX96 > 0, "TigerSwap: ZERO_PRICE");
        return zeroForOne
            ? getNextSqrtPriceFromAmount0RoundingUp(sqrtPX96, liquidityAmount, amountIn, true)
            : getNextSqrtPriceFromAmount1RoundingDown(sqrtPX96, liquidityAmount, amountIn, true);
    }

    function getNextSqrtPriceFromOutput(
        uint160 sqrtPX96,
        uint128 liquidityAmount,
        uint256 amountOut,
        bool zeroForOne
    ) internal pure returns (uint160) {
        require(sqrtPX96 > 0, "TigerSwap: ZERO_PRICE");
        return zeroForOne
            ? getNextSqrtPriceFromAmount1RoundingDown(sqrtPX96, liquidityAmount, amountOut, false)
            : getNextSqrtPriceFromAmount0RoundingUp(sqrtPX96, liquidityAmount, amountOut, false);
    }

    function getNextSqrtPriceFromAmount0RoundingUp(
        uint160 sqrtPX96,
        uint128 liquidityAmount,
        uint256 amount,
        bool add
    ) internal pure returns (uint160) {
        if (amount == 0) return sqrtPX96;
        uint256 numerator1 = uint256(liquidityAmount) << 96;

        if (add) {
            uint256 productAdd = amount * sqrtPX96;
            if (productAdd / amount == sqrtPX96) {
                uint256 denominator0 = numerator1 + productAdd;
                if (denominator0 >= numerator1) {
                    return uint160(FullMath.mulDivRoundingUp(numerator1, sqrtPX96, denominator0));
                }
            }
            return uint160(divRoundingUp(numerator1, numerator1 / sqrtPX96 + amount));
        }

        uint256 productSub = amount * sqrtPX96;
        require(productSub / amount == sqrtPX96 && numerator1 > productSub, "TigerSwap: PRICE_MATH");
        uint256 denominator1 = numerator1 - productSub;
        return uint160(FullMath.mulDivRoundingUp(numerator1, sqrtPX96, denominator1));
    }

    function getNextSqrtPriceFromAmount1RoundingDown(
        uint160 sqrtPX96,
        uint128 liquidityAmount,
        uint256 amount,
        bool add
    ) internal pure returns (uint160) {
        if (add) {
            uint256 quotient = amount <= type(uint160).max
                ? (amount << 96) / liquidityAmount
                : FullMath.mulDiv(amount, Q96, liquidityAmount);
            return uint160(uint256(sqrtPX96) + quotient);
        }

        uint256 quotient = amount <= type(uint160).max
            ? divRoundingUp(amount << 96, liquidityAmount)
            : FullMath.mulDivRoundingUp(amount, Q96, liquidityAmount);
        require(sqrtPX96 > quotient, "TigerSwap: PRICE_MATH");
        return uint160(sqrtPX96 - quotient);
    }

    function getAmount0Delta(uint160 sqrtRatioAX96, uint160 sqrtRatioBX96, int128 liquidityAmount, bool roundUp)
        internal
        pure
        returns (uint256)
    {
        if (sqrtRatioAX96 > sqrtRatioBX96) {
            (sqrtRatioAX96, sqrtRatioBX96) = (sqrtRatioBX96, sqrtRatioAX96);
        }
        require(sqrtRatioAX96 > 0, "TigerSwap: ZERO_PRICE");

        uint128 liquidityAbs = liquidityAmount < 0 ? uint128(-liquidityAmount) : uint128(liquidityAmount);
        uint256 numerator1 = uint256(liquidityAbs) << 96;
        uint256 numerator2 = uint256(sqrtRatioBX96) - uint256(sqrtRatioAX96);
        require(sqrtRatioAX96 > 0, "TigerSwap: ZERO_PRICE");

        if (roundUp) {
            return FullMath.mulDivRoundingUp(
                FullMath.mulDivRoundingUp(numerator1, numerator2, sqrtRatioBX96), 1, sqrtRatioAX96
            );
        }
        return FullMath.mulDiv(numerator1, numerator2, sqrtRatioBX96) / uint256(sqrtRatioAX96);
    }

    function getAmount1Delta(uint160 sqrtRatioAX96, uint160 sqrtRatioBX96, int128 liquidityAmount, bool roundUp)
        internal
        pure
        returns (uint256)
    {
        if (sqrtRatioAX96 > sqrtRatioBX96) {
            (sqrtRatioAX96, sqrtRatioBX96) = (sqrtRatioBX96, sqrtRatioAX96);
        }
        uint128 liquidityAbs = liquidityAmount < 0 ? uint128(-liquidityAmount) : uint128(liquidityAmount);
        uint256 diff = uint256(sqrtRatioBX96) - uint256(sqrtRatioAX96);
        return roundUp
            ? FullMath.mulDivRoundingUp(uint256(liquidityAbs), diff, Q96)
            : FullMath.mulDiv(uint256(liquidityAbs), diff, Q96);
    }

    function divRoundingUp(uint256 a, uint256 b) internal pure returns (uint256) {
        require(b > 0, "TigerSwap: DIV_BY_ZERO");
        return a / b + (a % b == 0 ? 0 : 1);
    }
}

/// @notice Full-precision 512-bit multiply-then-divide.
library FullMath {
    function mulDiv(uint256 a, uint256 b, uint256 denominator) internal pure returns (uint256 result) {
        unchecked {
            uint256 prod0;
            uint256 prod1;
            assembly {
                let mm := mulmod(a, b, not(0))
                prod0 := mul(a, b)
                prod1 := sub(sub(mm, prod0), lt(mm, prod0))
            }

            if (prod1 == 0) {
                require(denominator > 0, "TigerSwap: DIV_BY_ZERO");
                return prod0 / denominator;
            }

            require(denominator > prod1, "TigerSwap: MULDIV_OVERFLOW");

            uint256 remainder;
            assembly {
                remainder := mulmod(a, b, denominator)
                prod1 := sub(prod1, gt(remainder, prod0))
                prod0 := sub(prod0, remainder)
            }

            uint256 twos = denominator & (~denominator + 1);
            assembly {
                denominator := div(denominator, twos)
                prod0 := div(prod0, twos)
                twos := add(div(sub(0, twos), twos), 1)
            }
            prod0 |= prod1 * twos;

            uint256 inverse = (3 * denominator) ^ 2;
            inverse *= 2 - denominator * inverse;
            inverse *= 2 - denominator * inverse;
            inverse *= 2 - denominator * inverse;
            inverse *= 2 - denominator * inverse;
            inverse *= 2 - denominator * inverse;
            inverse *= 2 - denominator * inverse;

            result = prod0 * inverse;
        }
    }

    function mulDivRoundingUp(uint256 a, uint256 b, uint256 denominator) internal pure returns (uint256 result) {
        result = mulDiv(a, b, denominator);
        if (mulmod(a, b, denominator) > 0) {
            require(result < type(uint256).max, "TigerSwap: MULDIV_OVERFLOW");
            result++;
        }
    }
}

/// @notice Q64.96 sqrt-ratio <-> tick conversion (Uniswap V3 algorithm).
library TickMath {
    int24 internal constant MIN_TICK = -887272;
    int24 internal constant MAX_TICK = 887272;
    uint160 internal constant MIN_SQRT_RATIO = 4295128739;
    uint160 internal constant MAX_SQRT_RATIO = 1461446703485210103287273052203988822378723970342;

    function getSqrtRatioAtTick(int24 tick) internal pure returns (uint160 sqrtPriceX96) {
        uint256 absTick = tick < 0 ? uint256(uint24(-tick)) : uint256(uint24(tick));
        require(absTick <= uint256(uint24(MAX_TICK)), "T");

        uint256 ratio = absTick & 0x1 != 0
            ? 0xfffcb933bd6fad37aa2d162d1a594001
            : 0x100000000000000000000000000000000;
        if (absTick & 0x2 != 0) ratio = (ratio * 0xfff97272373d413259a46990580e213a) >> 128;
        if (absTick & 0x4 != 0) ratio = (ratio * 0xfff2e50f5f656932ef12357cf3c7fdcc) >> 128;
        if (absTick & 0x8 != 0) ratio = (ratio * 0xffe5caca7e10e4e61c3624eaa0941cd0) >> 128;
        if (absTick & 0x10 != 0) ratio = (ratio * 0xffcb9843d60f6159c9db58835c926644) >> 128;
        if (absTick & 0x20 != 0) ratio = (ratio * 0xff973b41fa98c081472e6896dfb254c0) >> 128;
        if (absTick & 0x40 != 0) ratio = (ratio * 0xff2ea16466c96a3843ec78b326b52861) >> 128;
        if (absTick & 0x80 != 0) ratio = (ratio * 0xfe5dee046a99a2a811c461f1969c3053) >> 128;
        if (absTick & 0x100 != 0) ratio = (ratio * 0xfcbe86c7900a88aedcffc83b479aa3a4) >> 128;
        if (absTick & 0x200 != 0) ratio = (ratio * 0xf987a7253ac413176f2b074cf7815e54) >> 128;
        if (absTick & 0x400 != 0) ratio = (ratio * 0xf3392b0822b70005940c7a398e4b70f3) >> 128;
        if (absTick & 0x800 != 0) ratio = (ratio * 0xe7159475a2c29b7443b29c7fa6e889d9) >> 128;
        if (absTick & 0x1000 != 0) ratio = (ratio * 0xd097f3bdfd2022b8845ad8f792aa5825) >> 128;
        if (absTick & 0x2000 != 0) ratio = (ratio * 0xa9f746462d870fdf8a65dc1f90e061e5) >> 128;
        if (absTick & 0x4000 != 0) ratio = (ratio * 0x70d869a156d2a1b890bb3df62baf32f7) >> 128;
        if (absTick & 0x8000 != 0) ratio = (ratio * 0x31be135f97d08fd981231505542fcfa6) >> 128;
        if (absTick & 0x10000 != 0) ratio = (ratio * 0x9aa508b5b7a84e1c677de54f3e99bc9) >> 128;
        if (absTick & 0x20000 != 0) ratio = (ratio * 0x5d6af8dedb81196699c329225ee604) >> 128;
        if (absTick & 0x40000 != 0) ratio = (ratio * 0x2216e584f5fa1ea926041bedfe98) >> 128;
        if (absTick & 0x80000 != 0) ratio = (ratio * 0x48a170391f7dc42444e8fa2) >> 128;

        if (tick > 0) ratio = type(uint256).max / ratio;

        sqrtPriceX96 = uint160((ratio >> 32) + (ratio % (uint256(1) << 32) == 0 ? 0 : 1));
    }

    function getTickAtSqrtRatio(uint160 sqrtPriceX96) internal pure returns (int24 tick) {
        require(sqrtPriceX96 >= MIN_SQRT_RATIO && sqrtPriceX96 < MAX_SQRT_RATIO, "R");

        uint256 ratio = uint256(sqrtPriceX96) << 32;

        uint256 r = ratio;
        uint256 msb = 0;

        assembly {
            let f := shl(7, gt(r, 0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF))
            msb := or(msb, f)
            r := shr(f, r)
            f := shl(6, gt(r, 0xFFFFFFFFFFFFFFFF))
            msb := or(msb, f)
            r := shr(f, r)
            f := shl(5, gt(r, 0xFFFFFFFF))
            msb := or(msb, f)
            r := shr(f, r)
            f := shl(4, gt(r, 0xFFFF))
            msb := or(msb, f)
            r := shr(f, r)
            f := shl(3, gt(r, 0xFF))
            msb := or(msb, f)
            r := shr(f, r)
            f := shl(2, gt(r, 0xF))
            msb := or(msb, f)
            r := shr(f, r)
            f := shl(1, gt(r, 0x3))
            msb := or(msb, f)
            r := shr(f, r)
            f := gt(r, 0x1)
            msb := or(msb, f)
        }

        if (msb >= 128) r = ratio >> (msb - 127);
        else r = ratio << (127 - msb);

        int256 log_2 = (int256(msb) - 128) << 64;

        assembly {
            r := shr(127, mul(r, r))
            let f := shr(128, r)
            log_2 := or(log_2, shl(63, f))
            r := shr(f, r)

            r := shr(127, mul(r, r))
            f := shr(128, r)
            log_2 := or(log_2, shl(62, f))
            r := shr(f, r)

            r := shr(127, mul(r, r))
            f := shr(128, r)
            log_2 := or(log_2, shl(61, f))
            r := shr(f, r)

            r := shr(127, mul(r, r))
            f := shr(128, r)
            log_2 := or(log_2, shl(60, f))
            r := shr(f, r)

            r := shr(127, mul(r, r))
            f := shr(128, r)
            log_2 := or(log_2, shl(59, f))
            r := shr(f, r)

            r := shr(127, mul(r, r))
            f := shr(128, r)
            log_2 := or(log_2, shl(58, f))
            r := shr(f, r)

            r := shr(127, mul(r, r))
            f := shr(128, r)
            log_2 := or(log_2, shl(57, f))
            r := shr(f, r)

            r := shr(127, mul(r, r))
            f := shr(128, r)
            log_2 := or(log_2, shl(56, f))
            r := shr(f, r)

            r := shr(127, mul(r, r))
            f := shr(128, r)
            log_2 := or(log_2, shl(55, f))
            r := shr(f, r)

            r := shr(127, mul(r, r))
            f := shr(128, r)
            log_2 := or(log_2, shl(54, f))
            r := shr(f, r)

            r := shr(127, mul(r, r))
            f := shr(128, r)
            log_2 := or(log_2, shl(53, f))
            r := shr(f, r)

            r := shr(127, mul(r, r))
            f := shr(128, r)
            log_2 := or(log_2, shl(52, f))
            r := shr(f, r)

            r := shr(127, mul(r, r))
            f := shr(128, r)
            log_2 := or(log_2, shl(51, f))
            r := shr(f, r)

            r := shr(127, mul(r, r))
            f := shr(128, r)
            log_2 := or(log_2, shl(50, f))
        }

        int256 log_sqrt10001 = log_2 * 255738958999603826347141;

        int24 tickLow = int24((log_sqrt10001 - 3402992956809132418596140100660247210) >> 128);
        int24 tickHi = int24((log_sqrt10001 + 291339464771989622907027621153398088495) >> 128);

        tick = tickLow == tickHi ? tickLow : getSqrtRatioAtTick(tickHi) <= sqrtPriceX96 ? tickHi : tickLow;
    }
}
