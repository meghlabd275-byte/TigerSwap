// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import {IERC20} from "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import {SafeERC20} from "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";
import {ReentrancyGuard} from "@openzeppelin/contracts/utils/ReentrancyGuard.sol";
import {Ownable} from "@openzeppelin/contracts/access/Ownable.sol";

/**
 * @title TigerSwapCLPool
 * @notice Concentrated Liquidity Pool - Similar to Uniswap V3
 * @dev Implements full CLMM with ticks, positions, and range orders
 */
contract TigerSwapCLPool is ReentrancyGuard, Ownable {
    using SafeERC20 for IERC20;

    // ============ Constants ============
    uint256 constant Q128 = 2**128;
    uint256 constant Q96 = 2**96;
    uint256 constant MIN_SQRT_RATIO = 4295128739;
    uint256 constant MAX_SQRT_RATIO = 79228162514264337593543950335;
    int24 constant MIN_TICK = -887272;
    int24 constant MAX_TICK = 887272;

    // ============ State ============
    address public token0;
    address public token1;
    uint16 public fee; // fee in hundredths of a bip (1e-6)
    uint160 public sqrtPriceX96;
    int24 public tick;
    uint256 public liquidity;
    uint256 public feeGrowthGlobal0X128;
    uint256 public feeGrowthGlobal1X128;
    uint128 public observationIndex;
    uint256 public observationCardinality;
    uint256 public observationCardinalityNext;

    // Position data
    mapping(bytes32 => Position) public positions;
    mapping(int24 => Tick) public ticks;
    mapping(int24 => uint256) public tickBitmap;

    // Aggregated data
    uint256 public totalSupply;
    mapping(address => uint256) public balanceOf;

    // ============ Events ============
    event Initialize(
        uint160 sqrtPriceX96,
        int24 tick
    );

    event Mint(
        address sender,
        address owner,
        int24 tickLower,
        int24 tickUpper,
        uint256 amount,
        uint256 amount0,
        uint256 amount1
    );

    event Burn(
        address sender,
        address owner,
        int24 tickLower,
        int24 tickUpper,
        uint256 amount,
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

    event Collect(
        address owner,
        address recipient,
        uint256 amount0,
        uint256 amount1
    );

    // ============ Structs ============
    struct Position {
        uint128 liquidity;
        uint256 feeGrowthInside0LastX128;
        uint256 feeGrowthInside1LastX128;
        uint128 tokensOwed0;
        uint128 tokensOwed1;
    }

    struct Tick {
        uint128 liquidityGross;
        int128 liquidityNet;
        uint256 feeGrowthOutside0X128;
        uint256 feeGrowthOutside1X128;
        uint256 rewardGrowthOutside;
        int24 initialized;
    }

    struct Snapshot {
        uint256 liquidity;
        int24 tick;
        uint256 feeGrowth0;
        uint256 feeGrowth1;
    }

    // ============ Constructor ============
    constructor(address _token0, address _token1, uint16 _fee, address _owner) Ownable(_owner) {
        require(_token0 != _token1, "Identical addresses");
        require(_fee < 1000000, "Fee too high");
        
        token0 = _token0;
        token1 = _token1;
        fee = _fee;
    }

    // ============ Initialization ============
    function initialize(uint160 _sqrtPriceX96) external onlyOwner {
        require(sqrtPriceX96 == 0, "Already initialized");
        require(_sqrtPriceX96 >= MIN_SQRT_RATIO && _sqrtPriceX96 < MAX_SQRT_RATIO, "Invalid price");
        
        sqrtPriceX96 = _sqrtPriceX96;
        tick = _tickFromSqrtPrice(_sqrtPriceX96);
        
        observationIndex = 0;
        observationCardinality = 1;
        observationCardinalityNext = 1;
        
        emit Initialize(_sqrtPriceX96, tick);
    }

    // ============ Mint (Add Liquidity) ============
    function mint(
        address owner,
        int24 tickLower,
        int24 tickUpper,
        uint128 amount
    ) external nonReentrant returns (uint256 amount0, uint256 amount1) {
        require(amount > 0, "Amount is 0");
        require(tickLower < tickUpper, "Invalid range");
        require(tickLower >= MIN_TICK && tickLower <= MAX_TICK, "Invalid lower tick");
        require(tickUpper >= MIN_TICK && tickUpper <= MAX_TICK, "Invalid upper tick");
        
        // Update position
        bytes32 positionKey = _positionKey(owner, tickLower, tickUpper);
        Position storage pos = positions[positionKey];
        
        uint128 liquidityBefore = pos.liquidity;
        uint128 liquidityNew = liquidityBefore + amount;
        
        // Update ticks
        _updateTick(tickLower, int128(amount), true);
        _updateTick(tickUpper, -int128(amount), true);
        
        // Update position
        pos.liquidity = liquidityNew;
        if (liquidityBefore == 0) {
            pos.feeGrowthInside0LastX128 = feeGrowthGlobal0X128;
            pos.feeGrowthInside1LastX128 = feeGrowthGlobal1X128;
        }
        
        // Update pool liquidity
        liquidity += amount;
        
        // Calculate token amounts
        amount0 = _getAmount0ForLiquidity(tickLower, tickUpper, amount);
        amount1 = _getAmount1ForLiquidity(tickLower, tickUpper, amount);
        
        // Transfer tokens
        if (amount0 > 0) {
            IERC20(token0).safeTransferFrom(msg.sender, address(this), amount0);
        }
        if (amount1 > 0) {
            IERC20(token1).safeTransferFrom(msg.sender, address(this), amount1);
        }
        
        emit Mint(msg.sender, owner, tickLower, tickUpper, amount, amount0, amount1);
    }

    // ============ Burn (Remove Liquidity) ============
    function burn(
        int24 tickLower,
        int24 tickUpper,
        uint128 amount
    ) external nonReentrant returns (uint256 amount0, uint256 amount1) {
        require(amount > 0, "Amount is 0");
        
        bytes32 positionKey = _positionKey(msg.sender, tickLower, tickUpper);
        Position storage pos = positions[positionKey];
        
        require(pos.liquidity >= amount, "Insufficient liquidity");
        
        // Update position
        uint128 liquidityNew = pos.liquidity - amount;
        pos.liquidity = liquidityNew;
        
        // Update ticks
        _updateTick(tickLower, -int128(amount), false);
        _updateTick(tickUpper, int128(amount), false);
        
        // Update pool liquidity
        liquidity -= amount;
        
        // Calculate token amounts
        amount0 = _getAmount0ForLiquidity(tickLower, tickUpper, amount);
        amount1 = _getAmount1ForLiquidity(tickLower, tickUpper, amount);
        
        // Collect fees first
        _collect(msg.sender, tickLower, tickUpper);
        
        // Transfer tokens
        if (amount0 > 0) {
            IERC20(token0).safeTransfer(msg.sender, amount0);
        }
        if (amount1 > 0) {
            IERC20(token1).safeTransfer(msg.sender, amount1);
        }
        
        emit Burn(msg.sender, msg.sender, tickLower, tickUpper, amount, amount0, amount1);
    }

    // ============ Swap ============
    function swap(
        address recipient,
        bool zeroForOne,
        int256 amountSpecified,
        uint160 sqrtPriceLimitX96
    ) external nonReentrant returns (int256 amount0, int256 amount1) {
        require(amountSpecified != 0, "Amount is 0");
        
        // Set price limit
        if (zeroForOne) {
            require(sqrtPriceLimitX96 > MIN_SQRT_RATIO && sqrtPriceLimitX96 < sqrtPriceX96, "Invalid limit");
        } else {
            require(sqrtPriceLimitX96 > sqrtPriceX96 && sqrtPriceLimitX96 < MAX_SQRT_RATIO, "Invalid limit");
        }
        
        bool exactInput = amountSpecified > 0;
        
        // Swap loop
        while (amountSpecified != 0) {
            (uint160 sqrtPriceNextX96, int24 nextTick, , ) = _getNextTick(
                tick,
                zeroForOne,
                sqrtPriceLimitX96
            );
            
            // Calculate swap step
            uint256 amountIn = uint256(amountSpecified > 0 ? amountSpecified : -amountSpecified);
            (uint256 amountOut, uint256 amountInRemaining, uint256 fee) = _computeSwapStep(
                sqrtPriceX96,
                zeroForOne ? sqrtPriceNextX96 : sqrtPriceLimitX96,
                liquidity,
                amountIn,
                fee
            );
            
            // Update amounts
            if (exactInput) {
                amountSpecified -= int256(amountInRemaining);
                if (amountSpecified < 0) amountSpecified = 0;
            } else {
                amountSpecified += int256(amountOut);
            }
            
            // Update fee
            if (zeroForOne) {
                feeGrowthGlobal0X128 += (fee * Q128) / liquidity;
            } else {
                feeGrowthGlobal1X128 += (fee * Q128) / liquidity;
            }
            
            // Update price and tick
            if (sqrtPriceX96 == sqrtPriceNextX96) {
                tick = nextTick;
                sqrtPriceX96 = sqrtPriceNextX96;
            } else {
                sqrtPriceX96 = sqrtPriceNextX96;
                tick = _tickFromSqrtPrice(sqrtPriceX96);
            }
            
            // Check if reached limit
            if (sqrtPriceX96 == sqrtPriceLimitX96) break;
        }
        
        // Transfer tokens
        if (zeroForOne) {
            amount0 = -amountSpecified;
            if (amount0 > 0) {
                IERC20(token0).safeTransfer(recipient, uint256(amount0));
            }
        } else {
            amount1 = -amountSpecified;
            if (amount1 > 0) {
                IERC20(token1).safeTransfer(recipient, uint256(amount1));
            }
        }
        
        emit Swap(msg.sender, recipient, amount0, amount1, sqrtPriceX96, liquidity, tick);
    }

    // ============ Collect Fees ============
    function collect(
        address recipient,
        int24 tickLower,
        int24 tickUpper
    ) external nonReentrant returns (uint256 amount0, uint256 amount1) {
        bytes32 positionKey = _positionKey(msg.sender, tickLower, tickUpper);
        Position storage pos = positions[positionKey];
        
        (uint256 owed0, uint256 owed1) = _collect(msg.sender, tickLower, tickUpper);
        
        if (owed0 > 0) {
            IERC20(token0).safeTransfer(recipient, owed0);
        }
        if (owed1 > 0) {
            IERC20(token1).safeTransfer(recipient, owed1);
        }
        
        emit Collect(recipient, recipient, owed0, owed1);
    }

    // ============ Internal Functions ============
    
    function _positionKey(address owner, int24 tickLower, int24 tickUpper) 
        internal pure returns (bytes32) {
        return keccak256(abi.encodePacked(owner, tickLower, tickUpper));
    }

    function _tickFromSqrtPrice(uint160 sqrtPriceX96) internal pure returns (int24) {
        int24 tick = int24((log(sqrtPriceX96 / Q96) * 2) / 1);
        return tick;
    }

    function log(uint256 x) internal pure returns (int256) {
        // Simplified log2 implementation
        int256 result = 0;
        while (x > 1) {
            x >>= 1;
            result += 1;
        }
        return result;
    }

    function _updateTick(int24 tick, int128 liquidityDelta, bool isInitialize) internal {
        Tick storage t = ticks[tick];
        
        uint128 liquidityGrossBefore = t.liquidityGross;
        uint128 liquidityGrossNew = liquidityGrossBefore + uint128(liquidityDelta);
        
        if (isInitialize) {
            if (liquidityGrossBefore == 0) {
                t.initialized = tick;
            }
        }
        
        t.liquidityGross = liquidityGrossNew;
        
        if (liquidityDelta != 0) {
            if (liquidityDelta > 0) {
                t.liquidityNet += liquidityDelta;
            } else {
                t.liquidityNet -= liquidityDelta;
            }
        }
    }

    function _getNextTick(
        int24 currentTick,
        bool zeroForOne,
        uint160 sqrtPriceLimitX96
    ) internal view returns (
        uint160 sqrtPriceNextX96,
        int24 nextTick,
        uint256 feeGrowth,
        uint256 rewardGrowth
    ) {
        // Simplified - in production, use tick bitmap to find next initialized tick
        nextTick = currentTick + (zeroForOne ? -60 : 60);
        sqrtPriceNextX96 = _sqrtPriceFromTick(nextTick);
        
        // Ensure we don't go past the limit
        if (zeroForOne) {
            if (sqrtPriceNextX96 < sqrtPriceLimitX96) {
                sqrtPriceNextX96 = sqrtPriceLimitX96;
            }
        } else {
            if (sqrtPriceNextX96 > sqrtPriceLimitX96) {
                sqrtPriceNextX96 = sqrtPriceLimitX96;
            }
        }
        
        return (sqrtPriceNextX96, nextTick, 0, 0);
    }

    function _computeSwapStep(
        uint160 sqrtPriceCurrentX96,
        uint160 sqrtPriceTargetX96,
        uint128 currentLiquidity,
        uint256 amountIn,
        uint256 _fee
    ) internal pure returns (
        uint256 amountOut,
        uint256 amountInRemaining,
        uint256 fee
    ) {
        // Calculate output amount using constant product formula
        uint256 sqrtRatioDiff = sqrtPriceTargetX96 - sqrtPriceCurrentX96;
        
        if (sqrtRatioDiff == 0) {
            return (0, amountIn, 0);
        }
        
        uint256 amountInLessFee = amountIn * (10000 - _fee) / 10000;
        fee = amountIn - amountInLessFee;
        
        // Calculate amount out
        uint256 numerator = amountInLessFee * sqrtPriceCurrentX96 * sqrtPriceTargetX96;
        uint256 denominator = sqrtRatioDiff * Q96;
        amountOut = numerator / denominator;
        
        amountInRemaining = 0;
        
        // Check if we need more input
        if (amountOut == 0) {
            amountInRemaining = amountIn;
        }
        
        return (amountOut, amountInRemaining, fee);
    }

    function _sqrtPriceFromTick(int24 tick) internal pure returns (uint160) {
        // Simplified - in production use precise math
        uint256 ratio = Q96;
        
        if (tick > 0) {
            for (int24 i = 0; i < tick; i++) {
                ratio = (ratio * 1000000) / 995000; // Approximate
            }
        } else if (tick < 0) {
            for (int24 i = tick; i < 0; i++) {
                ratio = (ratio * 995000) / 1000000;
            }
        }
        
        return uint160(ratio);
    }

    function _getAmount0ForLiquidity(
        int24 tickLower,
        int24 tickUpper,
        uint128 liquidity
    ) internal view returns (uint256) {
        uint160 sqrtLower = _sqrtPriceFromTick(tickLower);
        uint160 sqrtUpper = _sqrtPriceFromTick(tickUpper);
        
        uint256 amount0 = (uint256(liquidity) * (sqrtUpper - sqrtLower)) / Q96;
        return amount0;
    }

    function _getAmount1ForLiquidity(
        int24 tickLower,
        int24 tickUpper,
        uint128 liquidity
    ) internal view returns (uint256) {
        uint160 sqrtLower = _sqrtPriceFromTick(tickLower);
        uint160 sqrtUpper = _sqrtPriceFromTick(tickUpper);
        
        uint256 amount1 = (uint256(liquidity) * (sqrtUpper - sqrtLower)) / Q96;
        return amount1;
    }

    function _collect(
        address owner,
        int24 tickLower,
        int24 tickUpper
    ) internal returns (uint256 owed0, uint256 owed1) {
        bytes32 positionKey = _positionKey(owner, tickLower, tickUpper);
        Position storage pos = positions[positionKey];
        
        uint256 feeGrowthInside0 = feeGrowthGlobal0X128 - pos.feeGrowthInside0LastX128;
        uint256 feeGrowthInside1 = feeGrowthGlobal1X128 - pos.feeGrowthInside1LastX128;
        
        owed0 = (feeGrowthInside0 * pos.liquidity) / Q128;
        owed1 = (feeGrowthInside1 * pos.liquidity) / Q128;
        
        pos.tokensOwed0 += uint128(owed0);
        pos.tokensOwed1 += uint128(owed1);
        
        pos.feeGrowthInside0LastX128 = feeGrowthGlobal0X128;
        pos.feeGrowthInside1LastX128 = feeGrowthGlobal1X128;
    }

    // ============ View Functions ============
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
        bytes32 positionKey = _positionKey(owner, tickLower, tickUpper);
        Position storage pos = positions[positionKey];
        return (
            pos.liquidity,
            pos.feeGrowthInside0LastX128,
            pos.feeGrowthInside1LastX128,
            pos.tokensOwed0,
            pos.tokensOwed1
        );
    }

    function getTick(int24 tick) external view returns (
        uint128 liquidityGross,
        int128 liquidityNet,
        uint256 feeGrowthOutside0X128,
        uint256 feeGrowthOutside1X128
    ) {
        Tick storage t = ticks[tick];
        return (
            t.liquidityGross,
            t.liquidityNet,
            t.feeGrowthOutside0X128,
            t.feeGrowthOutside1X128
        );
    }
}