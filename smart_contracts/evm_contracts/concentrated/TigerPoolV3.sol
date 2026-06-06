// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "../libraries/SafeMath.sol";
import "../interfaces/ITigerSwapFactory.sol";

/**
 * @title TigerPoolV3
 * @notice Concentrated Liquidity Pool - Uniswap V3 Style Implementation
 * @dev Provides 4000x capital efficiency through concentrated liquidity positions
 * 
 * Key innovations:
 * - Tick-based liquidity tracking
 * - Position-based LP tracking (NFT-like tokens)
 * - Multiple fee tiers (0.01%, 0.05%, 0.3%, 1%)
 * - Range orders support
 */
contract TigerPoolV3 {
    using SafeMath for uint256;
    using SafeMath for uint160;

    // ============== Constants ==============
    
    // Special tick values
    int24 constant internal MIN_TICK = -887272;
    int24 constant internal MAX_TICK = 887272;
    
    // Max ticks that can be initialized in one call
    uint256 constant internal MAX_TICKS_INITIALIZED = 1308451;
    
    // Fee tiers in hundredths of a bip (1/10000)
    uint24 constant public FEE_TIER_LOW = 100;      // 0.01%
    uint24 constant public FEE_TIER_MEDIUM = 500; // 0.05%
    uint24 constant publicFEE_TIER_HIGH = 3000;    // 0.3%
    uint24 constant public FEE_TIER_MAX = 10000;    // 1%
    
    // ============== State Variables ==============
    
    // Pool configuration
    address public factory;
    address public token0;
    address public token1;
    uint24 public fee;
    int24 public tickSpacing;
    
    // Squared price (Q64.64) - sqrt(P) * 2^64
    uint160 public sqrtPriceX96;
    int24 public tick;
    
    // Protocol fees (in thousandths)
    uint16 public protocolFeeDenominator = 1000;
    uint16 public protocolFees0;
    uint16 public protocolFees1;
    
    // Global state
    uint128 public liquidity;
    uint256 public observationIndex;
    uint256 public observationCardinality;
    uint256 public observationCardinalityNext;
    
    // Fee growth
    uint256 public feeGrowthGlobal0;
    uint256 public feeGrowthGlobal1;
    
    // Tick mapping
    mapping(int24 => Tick) public ticks;
    mapping(int24 => uint256) public tickBitmap;
    
    // Position mapping: key => Position
    mapping(bytes32 => Position) public positions;
    
    // Tokenized positions (NFT)
    mapping(uint256 => PositionInfo) public positionInfo;
    uint256 public nextPositionId = 1;
    
    // ============== Structs ==============
    
    struct Tick {
        uint128 liquidityGross;
        int128 liquidityNet;
        uint256 feeGrowthOutside0;
        uint256 feeGrowthOutside1;
        uint256 rewardGrowthOutside;
        int24 previousTick;
        int24 nextTick;
        uint32 tickCumulativeOutside;
        uint32 secondsPerLiquidityOutside;
        uint32 secondsOutside;
        bool initialized;
    }
    
    struct Position {
        uint128 liquidity;
        uint256 feeGrowthInside0;
        uint256 feeGrowthInside1;
        uint128 tokensOwed0;
        uint128 tokensOwed1;
    }
    
    struct PositionInfo {
        address owner;
        address token0;
        address token1;
        int24 tickLower;
        int24 tickUpper;
        uint128 liquidity;
    }
    
    // ============== Events ==============
    
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
    event Collect(
        address sender,
        address owner,
        int24 tickLower,
        int24 tickUpper,
        uint256 amount0,
        uint256 amount1
    );
    event Swap(
        address sender,
        address recipient,
        int256 amount0,
        int256 amount1,
        uint160 sqrtPriceX96,
        int24 tick,
        uint128 liquidity
    );
    event Flash(
        address sender,
        address recipient,
        uint256 amount0,
        uint256 amount1,
        uint256 paid0,
        uint256 paid1
    );
    
    // ============== Initialization ==============
    
    constructor() {
        factory = msg.sender;
    }
    
    /**
     * @notice Initialize pool with starting price
     */
    function initialize(uint160 _sqrtPriceX96) external {
        require(msg.sender == factory, "FORBIDDEN");
        require(sqrtPriceX96 == 0, "ALREADY_INITIALIZED");
        
        require(_sqrtPriceX96 >= 4295128739 && _sqrtPriceX96 <= 79228162514264337593543950335, "INVALID_PRICE");
        
        // Set tick from sqrt price
        int24 _tick = getTickAtSqrtPriceX96(_sqrtPriceX96);
        
        observationIndex = 0;
        observationCardinality = 0;
        observationCardinalityNext = 0;
        
        sqrtPriceX96 = _sqrtPriceX96;
        tick = _tick;
        
        emit Initialize(_sqrtPriceX96, _tick);
    }
    
    // ============== Core AMM Functions ==============
    
    /**
     * @notice Add liquidity to the pool within a price range
     */
    function mint(
        address _owner,
        int24 _tickLower,
        int24 _tickUpper,
        uint128 _amount
    ) external returns (uint256 amount0, uint256 amount1) {
        require(_tickLower < _tickUpper, "INVALID_RANGE");
        require(_tickLower >= MIN_TICK, "TICK_LOW");
        require(_tickUpper <= MAX_TICK, "TICK_HIGH");
        require(_amount > 0, "ZERO_LIQUIDITY");
        
        // Update tick if needed
        (, int24 _tick, , ) = _getSlot0();
        require(_tickLower < _tick && _tick < _tickUpper, "INVALID_RANGE_FOR_PRICE");
        
        // Update position
        bytes32 _positionKey = _positionKeyFor(msg.sender, _tickLower, _tickUpper);
        Position storage position = positions[_positionKey];
        
        // Calculate tokens needed
        amount0 = _getAmount0For(_tickLower, _tickUpper, _amount);
        amount1 = _getAmount1For(_tickLower, _tickUpper, _amount);
        
        // Update position
        position.liquidity = position.liquidity + _amount;
        
        // Update ticks
        _updateTick(_tickLower, int128(_amount), true);
        _updateTick(_tickUpper, -int128(_amount), true);
        
        // Mint position NFT
        uint256 positionId = nextPositionId++;
        positionInfo[positionId] = PositionInfo({
            owner: _owner,
            token0: token0,
            token1: token1,
            tickLower: _tickLower,
            tickUpper: _tickUpper,
            liquidity: _amount
        });
        
        // Update global liquidity
        liquidity = liquidity + _amount;
        
        emit Mint(msg.sender, _owner, _tickLower, _tickUpper, _amount, amount0, amount1);
    }
    
    /**
     * @notice Remove liquidity from the pool
     */
    function burn(
        int24 _tickLower,
        int24 _tickUpper,
        uint128 _amount
    ) external returns (uint256 amount0, uint256 amount1) {
        require(_tickLower < _tickUpper, "INVALID_RANGE");
        
        Position memory position = _getAndUpdatePosition(msg.sender, _tickLower, _tickUpper);
        require(position.liquidity >= _amount, "INSUFFICIENT_LIQUIDITY");
        
        // Calculate tokens to receive
        amount0 = _getAmount0For(_tickLower, _tickUpper, _amount);
        amount1 = _getAmount1For(_tickLower, _tickUpper, _amount);
        
        // Update position
        bytes32 _positionKey = _positionKeyFor(msg.sender, _tickLower, _tickUpper);
        Position storage _position = positions[_positionKey];
        _position.liquidity = _position.liquidity - _amount;
        
        // Update ticks
        _updateTick(_tickLower, -int128(_amount), false);
        _updateTick(_tickUpper, int128(_amount), false);
        
        // Update global liquidity
        liquidity = liquidity - _amount;
        
        emit Burn(msg.sender, msg.sender, _tickLower, _tickUpper, _amount, amount0, amount1);
    }
    
    /**
     * @notice Collect fees from a position
     */
    function collect(
        address _recipient,
        int24 _tickLower,
        int24 _tickUpper,
        uint256 _amount0Min,
        uint256 _amount1Min
    ) external returns (uint256 amount0, uint256 amount1) {
        require(_tickLower < _tickUpper, "INVALID_RANGE");
        
        Position memory position = _getAndUpdatePosition(msg.sender, _tickLower, _tickUpper);
        
        amount0 = position.tokensOwed0;
        amount1 = position.tokensOwed1;
        
        require(amount0 >= _amount0Min && amount1 >= _amount1Min, "INSUFFICIENT_COLLECT");
        
        // Reset owed tokens
        bytes32 _positionKey = _positionKeyFor(msg.sender, _tickLower, _tickUpper);
        Position storage _position = positions[_positionKey];
        _position.tokensOwed0 = 0;
        _position.tokensOwed1 = 0;
        
        emit Collect(msg.sender, _recipient, _tickLower, _tickUpper, amount0, amount1);
    }
    
    /**
     * @notice Execute a swap
     */
    function swap(
        address _recipient,
        bool _zeroForOne,
        int256 _amountSpecified,
        uint160 _sqrtPriceLimitX96
    ) external returns (int256 amount0, int256 amount1) {
        require(_amountSpecified != 0, "ZERO_AMOUNT");
        
        (uint160 _sqrtPriceX96, int24 _tick, , ) = _getSlot0();
        
        // Initialize swap state
        SwapState memory state = SwapState({
            amountSpecifiedRemaining: _amountSpecified,
            amountCalculated: 0,
            sqrtPriceX96: _sqrtPriceX96,
            tick: _tick,
            liquidity: liquidity,
            feeGrowthGlobal0: feeGrowthGlobal0,
            feeGrowthGlobal1: feeGrowthGlobal1
        });
        
        // Execute swap in loop (for large swaps)
        if (_zeroForOne) {
            // Selling token0 for token1
            amount0 = -_amountSpecified;
            
            while (state.amountSpecifiedRemaining > 0) {
                (uint160 nextSqrtPriceX96, int24 nextTick, uint128 liquidityStart) = _nextTick(
                    state.tick,
                    _zeroForOne,
                    _sqrtPriceLimitX96
                );
                
                // Calculate swap step
                (uint256 amountIn, uint256 amountOut, uint256 fee) = _computeSwapStep(
                    state.sqrtPriceX96,
                    nextSqrtPriceX96,
                    state.liquidity,
                    state.amountSpecifiedRemaining,
                    fee
                );
                
                state.amountSpecifiedRemaining -= int256(amountIn + fee);
                state.amountCalculated += int256(amountOut);
                
                if (state.sqrtPriceX96 == nextSqrtPriceX96) {
                    // Cross tick
                    int24 nextTick2 = _zeroForOne ? nextTick - 1 : nextTick + 1;
                    (state.liquidity, , ) = _crossTick(nextTick2, state.liquidity);
                    state.tick = _zeroForOne ? nextTick - 1 : nextTick;
                } else {
                    state.sqrtPriceX96 = nextSqrtPriceX96;
                    break;
                }
            }
        } else {
            // Selling token1 for token0
            amount1 = -_amountSpecified;
            
            while (state.amountSpecifiedRemaining > 0) {
                (uint160 nextSqrtPriceX96, int24 nextTick, uint128 liquidityStart) = _nextTick(
                    state.tick,
                    _zeroForOne,
                    _sqrtPriceLimitX96
                );
                
                (uint256 amountIn, uint256 amountOut, uint256 fee) = _computeSwapStep(
                    state.sqrtPriceX96,
                    nextSqrtPriceX96,
                    state.liquidity,
                    state.amountSpecifiedRemaining,
                    fee
                );
                
                state.amountSpecifiedRemaining -= int256(amountIn + fee);
                state.amountCalculated += int256(amountOut);
                
                if (state.sqrtPriceX96 == nextSqrtPriceX96) {
                    int24 nextTick2 = _zeroForOne ? nextTick - 1 : nextTick + 1;
                    (state.liquidity, , ) = _crossTick(nextTick2, state.liquidity);
                    state.tick = _zeroForOne ? nextTick - 1 : nextTick;
                } else {
                    state.sqrtPriceX96 = nextSqrtPriceX96;
                    break;
                }
            }
        }
        
        // Update state
        (uint160 finalSqrtPriceX96, int24 finalTick) = (state.sqrtPriceX96, state.tick);
        sqrtPriceX96 = finalSqrtPriceX96;
        tick = finalTick;
        
        amount0 = _zeroForOne ? state.amountCalculated : amount0;
        amount1 = _zeroForOne ? amount1 : state.amountCalculated;
        
        emit Swap(
            msg.sender,
            _recipient,
            amount0,
            amount1,
            finalSqrtPriceX96,
            finalTick,
            liquidity
        );
    }
    
    // ============== Flash Loan Support ==============
    
    /**
     * @notice Flash loan - borrow tokens and callback
     */
    function flash(
        address _recipient,
        uint256 _amount0,
        uint256 _amount1,
        bytes calldata _data
    ) external {
        require(_amount0 > 0 || _amount1 > 0, "ZERO_AMOUNT");
        
        uint256 fee0 = _amount0 > 0 ? _flashFee0(_amount0) : 0;
        uint256 fee1 = _amount1 > 0 ? _flashFee1(_amount1) : 0;
        
        // Transfer borrowed tokens
        if (_amount0 > 0) {
            IERC20(token0).transfer(_recipient, _amount0);
        }
        if (_amount1 > 0) {
            IERC20(token1).transfer(_recipient, _amount1);
        }
        
        // Callback
        ITigerFlashCallback(msg.sender).tigerFlashCallback(
            msg.sender,
            _amount0,
            _amount1,
            fee0,
            fee1,
            _data
        );
        
        // Verify returned
        uint256 balance0 = IERC20(token0).balanceOf(address(this));
        uint256 balance1 = IERC20(token1).balanceOf(address(this));
        
        require(balance0 >= fee0 && balance1 >= fee1, "INSUFFICIENT_REPAY");
        
        // Protocol fee
        if (fee0 > 0) {
            uint256 protocolFee = fee0 * protocolFeeDenominator / 1000;
            if (protocolFee > 0) {
                fee0 -= protocolFee;
                protocolFees0 += uint16(protocolFee);
            }
        }
        
        if (fee1 > 0) {
            uint256 protocolFee = fee1 * protocolFeeDenominator / 1000;
            if (protocolFee > 0) {
                fee1 -= protocolFee;
                protocolFees1 += uint16(protocolFee);
            }
        }
        
        emit Flash(msg.sender, _recipient, _amount0, _amount1, fee0, fee1);
    }
    
    // ============== View Functions ==============
    
    /**
     * @notice Get current slot0 data
     */
    function _getSlot0() internal view returns (
        uint160 _sqrtPriceX96,
        int24 _tick,
        uint16 _observationIndex,
        uint16 _observationCardinality
    ) {
        _sqrtPriceX96 = sqrtPriceX96;
        _tick = tick;
        _observationIndex = uint16(observationIndex);
        _observationCardinality = uint16(observationCardinality);
    }
    
    /**
     * @notice Get tick at given sqrt price
     */
    function getTickAtSqrtPriceX96(uint160 _sqrtPriceX96) public pure returns (int24 tick) {
        require(_sqrtPriceX96 < 79228162514264337593543950336, "INVALID_PRICE");
        
        uint256 tickInt = int256(_sqrtPriceX96) >> 64;
        if (int256(_sqrtPriceX96 & ((1 << 64) - 1)) != 0) {
            tickInt++;
        }
        
        tick = int24(tickInt / 65537);
        require(tick >= MIN_TICK && tick <= MAX_TICK, "INVALID_TICK");
        
        return tick;
    }
    
    /**
     * @notice Get sqrt price at given tick
     */
    function getSqrtPriceAtTick(int24 _tick) public pure returns (uint160 sqrtPriceX96) {
        require(_tick >= MIN_TICK && _tick <= MAX_TICK, "INVALID_TICK");
        
        uint256 _tickSigned = uint256(int256(_tick + 28800));
        
        uint256 ratio = _tickSigned & 1 != 0 
            ? (79228162514264337593543950335 * 79228162514264337593543950335) / ((_tickSigned + 1) / 2)
            : (79228162514264337593543950335 * 79228162514264337593543950335) / ((_tickSigned + 2) / 2);
        
        sqrtPriceX96 = uint160(ratio);
    }
    
    /**
     * @notice Get position key
     */
    function _positionKeyFor(
        address _owner,
        int24 _tickLower,
        int24 _tickUpper
    ) internal pure returns (bytes32) {
        return keccak256(abi.encodePacked(_owner, _tickLower, _tickUpper));
    }
    
    /**
     * @notice Get and update position
     */
    function _getAndUpdatePosition(
        address _owner,
        int24 _tickLower,
        int24 _tickUpper
    ) internal returns (Position memory position) {
        bytes32 _positionKey = _positionKeyFor(_owner, _tickLower, _tickUpper);
        position = positions[_positionKey];
        
        (
            uint256 _feeGrowthInside0,
            uint256 _feeGrowthInside1
        ) = _getFeeGrowthInside(_tickLower, _tickUpper);
        
        position.feeGrowthInside0 = _feeGrowthInside0;
        position.feeGrowthInside1 = _feeGrowthInside1;
        
        // Calculate owed fees
        if (position.feeGrowthInside0 > position.feeGrowthInside1) {
            position.tokensOwed0 += uint128(
                (position.liquidity * (_feeGrowthInside0 - position.feeGrowthInside1)) / 1e36
            );
        }
        if (position.feeGrowthInside1 > position.feeGrowthInside1) {
            position.tokensOwed1 += uint128(
                (position.liquidity * (_feeGrowthInside1 - position.feeGrowthInside1)) / 1e36
            );
        }
        
        positions[_positionKey] = position;
    }
    
    /**
     * @notice Get fee growth inside a range
     */
    function _getFeeGrowthInside(
        int24 _tickLower,
        int24 _tickUpper
    ) internal view returns (uint256 feeGrowthInside0, uint256 feeGrowthInside1) {
        Tick storage lower = ticks[_tickLower];
        Tick storage upper = ticks[_tickUpper];
        
        uint256 feeGrowthBelow0 = lower.feeGrowthOutside0;
        uint256 feeGrowthBelow1 = lower.feeGrowthOutside1;
        
        if (tick >= _tickUpper) {
            feeGrowthBelow0 = upper.feeGrowthOutside0;
            feeGrowthBelow1 = upper.feeGrowthOutside1;
        }
        
        feeGrowthInside0 = feeGrowthGlobal0 - feeGrowthBelow0;
        feeGrowthInside1 = feeGrowthGlobal1 - feeGrowthBelow1;
    }
    
    /**
     * @notice Get amount0 needed for liquidity in range
     */
    function _getAmount0For(
        int24 _tickLower,
        int24 _tickUpper,
        uint128 _liquidity
    ) internal view returns (uint256 amount0) {
        uint160 sqrtPriceLowerX96 = getSqrtPriceAtTick(_tickLower);
        uint160 sqrtPriceUpperX96 = getSqrtPriceAtTick(_tickUpper);
        
        amount0 = _liquidity * (sqrtPriceUpperX96 - sqrtPriceLowerX96) / 1e64 * 1e18 / sqrtPriceLowerX96;
    }
    
    /**
     * @notice Get amount1 needed for liquidity in range
     */
    function _getAmount1For(
        int24 _tickLower,
        int24 _tickUpper,
        uint128 _liquidity
    ) internal view returns (uint256 amount1) {
        uint160 sqrtPriceLowerX96 = getSqrtPriceAtTick(_tickLower);
        uint160 sqrtPriceUpperX96 = getSqrtPriceAtTick(_tickUpper);
        
        amount1 = _liquidity * (sqrtPriceUpperX96 - sqrtPriceLowerX96) / 1e64;
    }
    
    /**
     * @notice Update tick with liquidity change
     */
    function _updateTick(
        int24 _tick,
        int128 _liquidityDelta,
        bool _maxLiquidity
    ) internal {
        Tick storage tick = ticks[_tick];
        
        if (_maxLiquidity) {
            require(tick.liquidityGross + uint128(_liquidityDelta) <= MAX_TICKS_INITIALIZED, "MAX_TICKS");
        }
        
        if (_liquidityDelta > 0) {
            tick.liquidityGross += uint128(_liquidityDelta);
        } else {
            tick.liquidityGross -= uint128(-_liquidityDelta);
        }
        
        if (_liquidityDelta < 0) {
            tick.liquidityNet -= _liquidityDelta;
        } else {
            tick.liquidityNet += _liquidityDelta;
        }
        
        if (tick.liquidityGross == 0) {
            tick.initialized = true;
        }
    }
    
    /**
     * @notice Cross a tick (price moves past it)
     */
    function _crossTick(
        int24 _tick,
        uint128 _liquidity
    ) internal returns (uint128 newLiquidity, int24 previousTick, int24 nextTick) {
        Tick storage tick = ticks[_tick];
        
        newLiquidity = uint128(int128(_liquidity) + tick.liquidityNet);
        previousTick = tick.previousTick;
        nextTick = tick.nextTick;
        
        if (newLiquidity == 0) {
            delete ticks[_tick];
        }
    }
    
    /**
     * @notice Get next tick in direction
     */
    function _nextTick(
        int24 _currentTick,
        bool _zeroForOne,
        uint160 _sqrtPriceLimitX96
    ) internal view returns (
        uint160 nextSqrtPriceX96,
        int24 nextTick,
        uint128 liquidity
    ) {
        liquidity = liquidity;
        
        // Simplified - find next initialized tick
        if (_zeroForOne) {
            // Find next lower tick
            int24 next = _currentTick - tickSpacing;
            while (next >= MIN_TICK && !ticks[next].initialized) {
                next -= tickSpacing;
            }
            nextTick = next;
        } else {
            // Find next higher tick
            int24 next = _currentTick + tickSpacing;
            while (next <= MAX_TICK && !ticks[next].initialized) {
                next += tickSpacing;
            }
            nextTick = next;
        }
        
        nextSqrtPriceX96 = getSqrtPriceAtTick(nextTick);
        
        if (_sqrtPriceLimitX96 > 0) {
            if (_zeroForOne && nextSqrtPriceX96 < _sqrtPriceLimitX96) {
                nextSqrtPriceX96 = _sqrtPriceLimitX96;
            } else if (!_zeroForOne && nextSqrtPriceX96 > _sqrtPriceLimitX96) {
                nextSqrtPriceX96 = _sqrtPriceLimitX96;
            }
        }
    }
    
    /**
     * @notice Compute swap step result
     */
    function _computeSwapStep(
        uint160 _sqrtPriceCurrentX96,
        uint160 _sqrtPriceTargetX96,
        uint128 _liquidity,
        int256 _amountRemaining,
        uint24 _fee
    ) internal pure returns (
        uint256 amountIn,
        uint256 amountOut,
        uint256 fee
    ) {
        uint256 sqrtPriceDiff = _sqrtPriceTargetX96 > _sqrtPriceCurrentX96
            ? _sqrtPriceTargetX96 - _sqrtPriceCurrentX96
            : _sqrtPriceCurrentX96 - _sqrtPriceTargetX96;
        
        // Calculate amount based on price movement
        if (sqrtPriceDiff > 0 && _amountRemaining > 0) {
            amountIn = _liquidity * sqrtPriceDiff / 1e64;
            amountOut = _liquidity * sqrtPriceDiff / 1e64;
            
            if (_amountRemaining > int256(amountIn)) {
                amountIn = uint256(_amountRemaining);
            }
            
            // Calculate output from amount in
            amountOut = amountIn * _sqrtPriceCurrentX96 / 1e64;
        }
        
        // Fee
        fee = amountIn * _fee / 1e6;
    }
    
    /**
     * @notice Calculate flash loan fee for token0
     */
    function _flashFee0(uint256 _amount) internal view returns (uint256) {
        return _amount * fee / 1e6;
    }
    
    /**
     * @notice Calculate flash loan fee for token1
     */
    function _flashFee1(uint256 _amount) internal view returns (uint256) {
        return _amount * fee / 1e6;
    }
    
    // ============== Swap State ==============
    
    struct SwapState {
        int256 amountSpecifiedRemaining;
        int256 amountCalculated;
        uint160 sqrtPriceX96;
        int24 tick;
        uint128 liquidity;
        uint256 feeGrowthGlobal0;
        uint256 feeGrowthGlobal1;
    }
}

// ============== Mock ERC20 Interface ==============

interface IERC20 {
    function transfer(address to, uint256 amount) external returns (bool);
    function transferFrom(address from, address to, uint256 amount) external returns (bool);
    function balanceOf(address account) external view returns (uint256);
}

// ============== Callback Interface ==============

interface ITigerFlashCallback {
    function tigerFlashCallback(
        address sender,
        uint256 amount0,
        uint256 amount1,
        uint256 fee0,
        uint256 fee1,
        bytes calldata data
    ) external;
}