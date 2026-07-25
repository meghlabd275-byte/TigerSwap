// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/**
 * @title TigerSwapConcentrated
 * @notice Concentrated liquidity pool implementation (Uniswap V3 style)
 * @dev Implements concentrated liquidity with tick-based pricing
 */
contract TigerSwapConcentrated {
    // ============ Constants ============
    uint256 public constant MAX_FEE = 10000; // 100%
    uint256 public constant MAX_TICK = 887272; // Maximum tick for sqrtRatio
    
    // ============ State Variables ============
    address public factory;
    address public token0;
    address public token1;
    uint256 public fee;
    
    // Tick spacing (determined by fee tier)
    int24 public tickSpacing;
    
    // Current sqrt price (Q64.64)
    uint160 public sqrtPriceX96;
    int24 public tick;
    
    // Liquidity tracking
    uint128 public liquidity;
    mapping(int24 => Tick) public ticks;
    mapping(bytes32 => Position) public positions;
    
    // Observation data for TWAP
    Observation[] public observations;
    uint16 public observationIndex;
    uint16 public observationCardinality;
    uint16 public observationCardinalityNext;
    
    // Protocol fees
    uint256 public protocolFees0;
    uint256 public protocolFees1;
    address public protocolFeeRecipient;
    
    // ============ Data Structures ============
    struct Tick {
        uint128 liquidityGross;
        int128 liquidityNet;
        uint256 feeGrowthOutside0X128;
        uint256 feeGrowthOutside1X128;
        int56 tickCumulativeOutside;
        uint160 secondsPerLiquidityOutsideX128;
        uint32 secondsOutside;
        bool initialized;
    }
    
    struct Position {
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
    
    // ============ Events ============
    event Initialize(uint160 sqrtPriceX96, int24 tick);
    event Mint(
        address sender,
        address owner,
        int24 tickLower,
        int24 tickUpper,
        uint128 amount,
        uint256 amount0,
        uint256 amount1
    );
    event Burn(
        address sender,
        address owner,
        int24 tickLower,
        int24 tickUpper,
        uint128 amount,
        uint256 amount0,
        uint256 amount1
    );
    event Swap(
        address sender,
        address recipient,
        int256 amount0,
        int256 amount1,
        uint160 sqrtPriceX96,
        uint128 liquidity,
        int24 tick
    );
    event Flash(
        address sender,
        address recipient,
        uint256 amount0,
        uint256 amount1,
        uint256 paid0,
        uint256 paid1
    );
    
    // ============ Modifiers ============
    modifier onlyFactory() {
        require(msg.sender == factory, "Only factory");
        _;
    }
    
    // ============ Constructor ============
    constructor(
        address _factory,
        address _token0,
        address _token1,
        uint24 _fee,
        int24 _tickSpacing
    ) {
        factory = _factory;
        token0 = _token0;
        token1 = _token1;
        fee = _fee;
        tickSpacing = _tickSpacing;
        
        // Initialize observations
        observations.push(Observation({
            blockTimestamp: uint32(block.timestamp),
            tickCumulative: 0,
            secondsPerLiquidityCumulativeX128: 0,
            initialized: true
        }));
        observationCardinality = 1;
        observationCardinalityNext = 1;
    }
    
    // ============ Initialize ============
    function initialize(uint160 _sqrtPriceX96) external onlyFactory {
        require(sqrtPriceX96 == 0, "Already initialized");
        
        sqrtPriceX96 = _sqrtPriceX96;
        tick = TickMath.getTickAtSqrtRatioX96(_sqrtPriceX96);
        
        observations[0].blockTimestamp = uint32(block.timestamp);
        
        emit Initialize(_sqrtPriceX96, tick);
    }
    
    // ============ Mint (Add Liquidity) ============
    function mint(
        address recipient,
        int24 tickLower,
        int24 tickUpper,
        uint128 amount
    ) external returns (uint256 amount0, uint256 amount1) {
        require(amount > 0, "Invalid amount");
        require(tickLower < tickUpper, "Invalid tick range");
        require(tickLower >= -MAX_TICK && tickLower <= MAX_TICK, "Invalid tick lower");
        require(tickUpper >= -MAX_TICK && tickUpper <= MAX_TICK, "Invalid tick upper");
        
        // Check tick spacing
        require(
            tickLower % tickSpacing == 0 && tickUpper % tickSpacing == 0,
            "Invalid tick spacing"
        );
        
        // Update position
        bytes32 positionKey = keccak256(abi.encodePacked(recipient, tickLower, tickUpper));
        Position storage position = positions[positionKey];
        
        // Calculate tokens owed
        (uint256 tokensOwed0, uint256 tokensOwed1) = _updatePosition(
            recipient,
            tickLower,
            tickUpper,
            int256(amount)
        );
        
        // Calculate amounts based on current price
        (amount0, amount1) = _calculateMintAmounts(tickLower, tickUpper, amount);
        
        // Update position liquidity
        position.liquidity += uint128(amount);
        
        // Update ticks
        _updateTick(tickLower, int256(amount));
        _updateTick(tickUpper, -int256(amount));
        
        // Update global liquidity
        liquidity += amount;
        
        // Transfer tokens
        if (amount0 > 0) {
            IERC20(token0).transferFrom(msg.sender, address(this), amount0);
        }
        if (amount1 > 0) {
            IERC20(token1).transferFrom(msg.sender, address(this), amount1);
        }
        
        emit Mint(msg.sender, recipient, tickLower, tickUpper, amount, amount0, amount1);
    }
    
    // ============ Burn (Remove Liquidity) ============
    function burn(
        int24 tickLower,
        int24 tickUpper,
        uint128 amount
    ) external returns (uint256 amount0, uint256 amount1) {
        require(amount > 0, "Invalid amount");
        
        bytes32 positionKey = keccak256(abi.encodePacked(msg.sender, tickLower, tickUpper));
        Position storage position = positions[positionKey];
        
        require(position.liquidity >= amount, "Insufficient liquidity");
        
        // Calculate fees earned
        (uint256 fees0, uint256 fees1) = _collectFees(msg.sender, tickLower, tickUpper);
        
        // Update position
        (uint256 tokensOwed0, uint256 tokensOwed1) = _updatePosition(
            msg.sender,
            tickLower,
            tickUpper,
            -int256(amount)
        );
        
        // Calculate burn amounts
        (amount0, amount1) = _calculateMintAmounts(tickLower, tickUpper, amount);
        
        // Update position liquidity
        position.liquidity -= amount;
        
        // Update ticks
        _updateTick(tickLower, -int256(amount));
        _updateTick(tickUpper, int256(amount));
        
        // Update global liquidity
        liquidity -= amount;
        
        // Transfer tokens
        if (amount0 + fees0 > 0) {
            IERC20(token0).transfer(msg.sender, amount0 + fees0);
        }
        if (amount1 + fees1 > 0) {
            IERC20(token1).transfer(msg.sender, amount1 + fees1);
        }
        
        emit Burn(msg.sender, msg.sender, tickLower, tickUpper, amount, amount0, amount1);
    }
    
    // ============ Swap ============
    function swap(
        address recipient,
        bool zeroForOne,
        int256 amountSpecified,
        uint160 sqrtPriceLimitX96
    ) external returns (int256 amount0, int256 amount1) {
        require(amountSpecified != 0, "Invalid amount");
        
        // Get current state
        uint128 liquidityStart = liquidity;
        int24 tickStart = tick;
        uint160 sqrtPriceStartX96 = sqrtPriceX96;
        
        // Execute swap
        (uint160 sqrtPriceNextX96, int24 nextTick, int256 amountIn, int256 amountOut, uint256 fee) = 
            _swap(
                zeroForOne,
                amountSpecified,
                sqrtPriceLimitX96,
                liquidityStart
            );
        
        // Update state
        sqrtPriceX96 = sqrtPriceNextX96;
        tick = nextTick;
        
        // Handle amount calculations
        if (zeroForOne) {
            amount0 = amountSpecified;
            amount1 = -amountOut;
        } else {
            amount0 = -amountOut;
            amount1 = amountSpecified;
        }
        
        // Transfer tokens
        if (amount0 > 0) {
            IERC20(token0).transfer(recipient, uint256(amount0));
        }
        if (amount1 > 0) {
            IERC20(token1).transfer(recipient, uint256(amount1));
        }
        
        // Record observation
        _writeObservation();
        
        emit Swap(
            msg.sender,
            recipient,
            amount0,
            amount1,
            sqrtPriceX96,
            liquidity,
            tick
        );
    }
    
    // ============ Flash Loan ============
    function flash(
        address recipient,
        uint256 amount0,
        uint256 amount1,
        bytes calldata data
    ) external {
        require(amount0 > 0 || amount1 > 0, "Invalid amounts");
        
        uint256 balance0Before = IERC20(token0).balanceOf(address(this));
        uint256 balance1Before = IERC20(token1).balanceOf(address(this));
        
        // Transfer flash loan amounts
        if (amount0 > 0) {
            IERC20(token0).transfer(recipient, amount0);
        }
        if (amount1 > 0) {
            IERC20(token1).transfer(recipient, amount1);
        }
        
        // Calculate fees (0.3% by default)
        uint256 fee0 = amount0 > 0 ? (amount0 * fee) / MAX_FEE : 0;
        uint256 fee1 = amount1 > 0 ? (amount1 * fee) / MAX_FEE : 0;
        
        // Callback for custom logic
        ITigerSwapFlashCallback(msg.sender).tigerSwapFlashCallback(
            amount0,
            amount1,
            fee0,
            fee1,
            data
        );
        
        // Verify tokens returned with fees
        uint256 balance0After = IERC20(token0).balanceOf(address(this));
        uint256 balance1After = IERC20(token1).balanceOf(address(this));
        
        require(balance0After >= balance0Before + fee0, "Flash loan failed: token0");
        require(balance1After >= balance1Before + fee1, "Flash loan failed: token1");
        
        emit Flash(msg.sender, recipient, amount0, amount1, fee0, fee1);
    }
    
    // ============ Internal Functions ============
    
    function _swap(
        bool zeroForOne,
        int256 amountSpecified,
        uint160 sqrtPriceLimitX96,
        uint128 liquidityStart
    ) internal returns (
        uint160 sqrtPriceNextX96,
        int24 nextTick,
        int256 amountIn,
        int256 amountOut,
        uint256 fee
    ) {
        // Simplified swap logic
        // In production, this would include full tick crossing logic
        
        uint256 amountRemaining = uint256(amountSpecified);
        
        if (zeroForOne) {
            // Selling token0 for token1
            while (amountRemaining > 0) {
                uint160 sqrtPrice = sqrtPriceX96;
                
                // Calculate output
                uint256 amountOutNext = _calculateAmountOut(
                    amountRemaining,
                    sqrtPrice,
                    liquidityStart,
                    zeroForOne
                );
                
                // Check price limit
                if (sqrtPriceLimitX96 > 0 && 
                    (amountOutNext > 0 && sqrtPrice <= sqrtPriceLimitX96)) {
                    break;
                }
                
                // Update amounts
                amountIn += int256(amountOutNext);
                amountRemaining -= amountOutNext;
                
                // Update price (simplified)
                uint256 priceChange = (amountOutNext * 1e18) / liquidityStart;
                sqrtPriceNextX96 = sqrtPrice + uint160(priceChange);
                
                // Break if price limit hit
                if (sqrtPriceLimitX96 > 0 && sqrtPriceNextX96 >= sqrtPriceLimitX96) {
                    break;
                }
                
                amountOut += int256(amountOutNext);
            }
        } else {
            // Selling token1 for token0
            while (amountRemaining > 0) {
                uint160 sqrtPrice = sqrtPriceX96;
                
                uint256 amountOutNext = _calculateAmountOut(
                    amountRemaining,
                    sqrtPrice,
                    liquidityStart,
                    zeroForOne
                );
                
                if (sqrtPriceLimitX96 > 0 && 
                    (amountOutNext > 0 && sqrtPrice >= sqrtPriceLimitX96)) {
                    break;
                }
                
                amountIn += int256(amountOutNext);
                amountRemaining -= amountOutNext;
                
                uint256 priceChange = (amountOutNext * 1e18) / liquidityStart;
                sqrtPriceNextX96 = sqrtPrice - uint160(priceChange);
                
                if (sqrtPriceLimitX96 > 0 && sqrtPriceNextX96 <= sqrtPriceLimitX96) {
                    break;
                }
                
                amountOut += int256(amountOutNext);
            }
        }
        
        // Calculate fee
        fee = (uint256(amountIn) * fee) / MAX_FEE;
        
        nextTick = tick;
        sqrtPriceNextX96 = sqrtPriceX96;
    }
    
    function _calculateAmountOut(
        uint256 amountIn,
        uint160 sqrtPrice,
        uint128 liquidity,
        bool zeroForOne
    ) internal pure returns (uint256) {
        if (amountIn == 0 || liquidity == 0) return 0;
        
        uint256 numerator = amountIn * liquidity;
        uint256 denominator;
        
        if (zeroForOne) {
            denominator = sqrtPrice + (amountIn * sqrtPrice / 1e18);
        } else {
            denominator = (uint256(1) << 96) + (amountIn * (uint256(1) << 96) / sqrtPrice);
        }
        
        return numerator / denominator;
    }
    
    function _calculateMintAmounts(
        int24 tickLower,
        int24 tickUpper,
        uint128 amount
    ) internal view returns (uint256 amount0, uint256 amount1) {
        // Calculate amounts using sqrt price
        uint160 sqrtRatioAX96 = TickMath.getSqrtRatioAtTick(tickLower);
        uint160 sqrtRatioBX96 = TickMath.getSqrtRatioAtTick(tickUpper);
        
        uint128 liquidityAmount = amount;
        
        if (tick < tickLower) {
            amount0 = _calcAmount0Delta(
                sqrtRatioAX96,
                sqrtRatioBX96,
                liquidityAmount,
                true
            );
        } else if (tick < tickUpper) {
            uint160 sqrtRatioCurrentX96 = sqrtPriceX96;
            amount0 = _calcAmount0Delta(
                sqrtRatioCurrentX96,
                sqrtRatioBX96,
                liquidityAmount,
                true
            );
            amount1 = _calcAmount1Delta(
                sqrtRatioAX96,
                sqrtRatioCurrentX96,
                liquidityAmount,
                true
            );
        } else {
            amount1 = _calcAmount1Delta(
                sqrtRatioAX96,
                sqrtRatioBX96,
                liquidityAmount,
                true
            );
        }
    }
    
    function _calcAmount0Delta(
        uint160 sqrtRatioAX96,
        uint160 sqrtRatioBX96,
        uint128 liquidity,
        bool roundUp
    ) internal pure returns (uint256) {
        unchecked {
            return _diffRatio(sqrtRatioAX96, sqrtRatioBX96, liquidity, roundUp);
        }
    }
    
    function _calcAmount1Delta(
        uint160 sqrtRatioAX96,
        uint160 sqrtRatioBX96,
        uint128 liquidity,
        bool roundUp
    ) internal pure returns (uint256) {
        unchecked {
            return _diffRatio(sqrtRatioBX96, sqrtRatioAX96, liquidity, roundUp);
        }
    }
    
    function _diffRatio(
        uint160 sqrtRatioAX96,
        uint160 sqrtRatioBX96,
        uint128 liquidity,
        bool roundUp
    ) internal pure returns (uint256) {
        uint256 numerator = uint256(liquidity) << 96;
        uint256 denominator = sqrtRatioBX96 - sqrtRatioAX96;
        
        if (roundUp) {
            return (numerator + denominator - 1) / denominator;
        }
        return numerator / denominator;
    }
    
    function _updatePosition(
        address owner,
        int24 tickLower,
        int24 tickUpper,
        int256 liquidityDelta
    ) internal returns (uint256, uint256) {
        bytes32 positionKey = keccak256(abi.encodePacked(owner, tickLower, tickUpper));
        Position storage position = positions[positionKey];
        
        // Calculate fees owed
        uint256 feeGrowthInside0X128 = _getFeeGrowthInside(tickLower, tickUpper);
        uint256 feeGrowthInside1X128 = _getFeeGrowthInside(tickLower, tickUpper);
        
        uint128 tokensOwed0 = uint128(
            (uint256(position.feeGrowthInside0LastX128) > 0)
                ? (feeGrowthInside0X128 - position.feeGrowthInside0LastX128) * position.liquidity >> 96
                : 0
        );
        uint128 tokensOwed1 = uint128(
            (uint256(position.feeGrowthInside1LastX128) > 0)
                ? (feeGrowthInside1X128 - position.feeGrowthInside1LastX128) * position.liquidity >> 96
                : 0
        );
        
        // Update position
        if (liquidityDelta != 0) {
            position.liquidity = uint128(int256(position.liquidity) + liquidityDelta);
        }
        
        position.feeGrowthInside0LastX128 = feeGrowthInside0X128;
        position.feeGrowthInside1LastX128 = feeGrowthInside1X128;
        
        return (tokensOwed0, tokensOwed1);
    }
    
    function _updateTick(int24 tick, int256 liquidityDelta) internal {
        Tick storage t = ticks[tick];
        
        if (liquidityDelta > 0) {
            t.liquidityNet += int128(liquidityDelta);
            t.liquidityGross += uint128(liquidityDelta);
        } else {
            t.liquidityNet -= int128(-liquidityDelta);
            t.liquidityGross -= uint128(-liquidityDelta);
        }
        
        if (t.liquidityGross > 0) {
            t.initialized = true;
        }
    }
    
    function _getFeeGrowthInside(int24 tickLower, int24 tickUpper) internal view returns (uint256) {
        // Simplified - in production would use proper fee growth tracking
        return 0;
    }
    
    function _collectFees(
        address owner,
        int24 tickLower,
        int24 tickUpper
    ) internal returns (uint256, uint256) {
        bytes32 positionKey = keccak256(abi.encodePacked(owner, tickLower, tickUpper));
        Position storage position = positions[positionKey];
        
        return (position.tokensOwed0, position.tokensOwed1);
    }
    
    function _writeObservation() internal {
        (uint16 index, uint16 cardinality) = (observationIndex, observationCardinality);
        
        if (index == cardinality - 1) {
            // Expand cardinality
            if (observationCardinalityNext > cardinality) {
                observations.push(Observation({
                    blockTimestamp: uint32(block.timestamp),
                    tickCumulative: 0,
                    secondsPerLiquidityCumulativeX128: 0,
                    initialized: false
                }));
                observationCardinality = observationCardinalityNext;
            }
            index = 0;
        } else {
            index++;
        }
        
        observations[index] = _transform(
            observations[index],
            block.timestamp,
            tick,
            liquidity
        );
        observationIndex = index;
    }
    
    function _transform(
        Observation memory observation,
        uint32 timestamp,
        int24 _tick,
        uint128 _liquidity
    ) internal pure returns (Observation memory) {
        return Observation({
            blockTimestamp: timestamp,
            tickCumulative: observation.tickCumulative + int56(_tick) * int56(timestamp - observation.blockTimestamp),
            secondsPerLiquidityCumulativeX128: observation.secondsPerLiquidityCumulativeX128 +
                uint160((timestamp - observation.blockTimestamp) << 128) / (_liquidity > 0 ? _liquidity : 1),
            initialized: true
        });
    }
    
    // ============ View Functions ============
    function getQuoteAtTick(int24 _tick) external pure returns (uint160) {
        return TickMath.getSqrtRatioAtTick(_tick);
    }
    
    function getTickAtSqrtRatio(uint160 sqrtRatioX96) external pure returns (int24) {
        return TickMath.getTickAtSqrtRatioX96(sqrtRatioX96);
    }
    
    function getPosition(
        address owner,
        int24 tickLower,
        int24 tickUpper
    ) external view returns (
        uint128 liquidity,
        uint256 feeGrowthInside0LastX128,
        uint256 feeGrowthInside1LastX128,
        uint128 tokensOwed0,
        uint128 tokensOwed1
    ) {
        bytes32 positionKey = keccak256(abi.encodePacked(owner, tickLower, tickUpper));
        Position storage position = positions[positionKey];
        
        return (
            position.liquidity,
            position.feeGrowthInside0LastX128,
            position.feeGrowthInside1LastX128,
            position.tokensOwed0,
            position.tokensOwed1
        );
    }
    
    // ============ Protocol Fees ============
    function setProtocolFeeRecipient(address _recipient) external onlyFactory {
        protocolFeeRecipient = _recipient;
    }
    
    function collectProtocolFees(
        uint256 amount0,
        uint256 amount1
    ) external returns (uint256, uint256) {
        require(msg.sender == protocolFeeRecipient, "Not recipient");
        
        if (amount0 > 0) {
            protocolFees0 -= amount0;
            IERC20(token0).transfer(protocolFeeRecipient, amount0);
        }
        if (amount1 > 0) {
            protocolFees1 -= amount1;
            IERC20(token1).transfer(protocolFeeRecipient, amount1);
        }
        
        return (amount0, amount1);
    }
}

// ============ Library for Tick Math ============
library TickMath {
    int24 internal constant MIN_TICK = -887272;
    int24 internal constant MAX_TICK = 887272;
    uint160 internal constant MIN_SQRT_RATIO = 4295128739;
    uint160 internal constant MAX_SQRT_RATIO = 14614467034852101032872730522039888213587260339517;
    
    function getSqrtRatioAtTick(int24 tick) internal pure returns (uint160) {
        uint256 absTick = tick < 0 ? uint256(-tick) : uint256(tick);
        uint256 ratio = absTick & 0x1 != 0 ? 0xfffcb933bd6fad37b2b2c7292a5eef34 : 0x100000000000000000000000000000000000;
        
        if (absTick & 0x2 != 0) ratio = (ratio * 0xfff97272373d413259a46990580e213a) >> 128;
        if (absTick & 0x4 != 0) ratio = (ratio * 0xfff2e50f5f656932ef12357cee3f76ba) >> 128;
        if (absTick & 0x8 != 0) ratio = (ratio * 0xffe5caca7e10e4d61c3f72d8b3a0a48) >> 128;
        if (absTick & 0x10 != 0) ratio = (ratio * 0xffcb9843d60f6159c9f5881a2f0c582) >> 128;
        if (absTick & 0x20 != 0) ratio = (ratio * 0xff973c97a944e915e6e1b7df3c4b4e3) >> 128;
        if (absTick & 0x40 != 0) ratio = (ratio * 0xffee56b0ab3b15e0f3c74dbbc6f6e80) >> 128;
        if (absTick & 0x80 != 0) ratio = (ratio * 0xff6550c0b3d3a4a4c7e3d2f4a1f5e3) >> 128;
        if (absTick & 0x100 != 0) ratio = (ratio * 0xfedb9a41e8e6f47c3a6c6e1e3f0e0e) >> 128;
        if (absTick & 0x200 != 0) ratio = (ratio * 0xfc7c1a6ed2c9c3b5e1c3e7d8b8f4c1) >> 128;
        if (absTick & 0x400 != 0) ratio = (ratio * 0xfb0877b7e1b5e4c7c5c5e3f3e3d4e5) >> 128;
        if (absTick & 0x800 != 0) ratio = (ratio * 0xf9830e0c1e3c5c6c7c8c9cad0d1e2) >> 128;
        
        if (tick > 0) ratio = type(uint256).max / ratio;
        
        return uint160(ratio);
    }
    
    function getTickAtSqrtRatioX96(uint160 sqrtRatioX96) internal pure returns (int24) {
        require(sqrtRatioX96 >= MIN_SQRT_RATIO && sqrtRatioX96 < MAX_SQRT_RATIO, "Invalid ratio");
        
        int256 msb = 0;
        int256 x = sqrtRatioX96;
        
        if (x >= 0x100000000000000000000000000000000000) { x >>= 128; msb += 128; }
        if (x >= 0x1000000000000000000000000) { x >>= 64; msb += 64; }
        if (x >= 0x1000000000000) { x >>= 32; msb += 32; }
        if (x >= 0x10000) { x >>= 16; msb += 16; }
        if (x >= 0x100) { x >>= 8; msb += 8; }
        if (x >= 0x10) { x >>= 4; msb += 4; }
        if (x >= 0x4) { x >>= 2; msb += 2; }
        if (x >= 0x2) msb += 1;
        
        int256 lsbs = ((x & 0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF) * 0x100000000000000000000000000000001) >> (256 - msb);
        
        int24 tick = int24((msb << 128) + lsbs);
        
        if (sqrtRatioX96 < getSqrtRatioAtTick(tick)) {
            tick -= 1;
        } else if (sqrtRatioX96 >= getSqrtRatioAtTick(tick + 1)) {
            tick += 1;
        }
        
        return tick;
    }
}

// ============ Interface for Flash Loan Callback ============
interface ITigerSwapFlashCallback {
    function tigerSwapFlashCallback(
        uint256 amount0,
        uint256 amount1,
        uint256 fee0,
        uint256 fee1,
        bytes calldata data
    ) external;
}

// ============ Basic IERC20 Interface ============
interface IERC20 {
    function transfer(address to, uint256 amount) external returns (bool);
    function transferFrom(address from, address to, uint256 amount) external returns (bool);
    function balanceOf(address account) external view returns (uint256);
}
