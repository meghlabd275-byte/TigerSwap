// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/// @title TigerSwap shared interfaces
/// @notice Interfaces used across the TigerSwap AMM contracts.

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

interface IWETH {
    function deposit() external payable;
    function transfer(address to, uint256 value) external returns (bool);
    function withdraw(uint256) external;
}

interface ITigerSwapFactory {
    function getPoolAddress(address tokenA, address tokenB) external view returns (address);
    function getPoolForFamily(address tokenA, address tokenB, uint8 family) external view returns (address);
    function createPool(address tokenA, address tokenB) external returns (address pool);
    function swapFee() external view returns (uint256);
    function protocolFee() external view returns (uint256);
    function feeTo() external view returns (address);
}

/// @notice Concentrated-liquidity pool (family 1).
interface ITigerSwapConcentratedPool {
    function token0() external view returns (address);
    function token1() external view returns (address);
    function swap(address recipient, bool zeroForOne, int256 amountSpecified, uint160 sqrtPriceLimitX96)
        external
        returns (int256 amount0, int256 amount1);
}

/// @notice StableSwap pool (family 2). Two coins, indices 0 and 1.
interface ITigerSwapStablePool {
    function coins(uint256 index) external view returns (address);
    function exchange(int128 i, int128 j, uint256 dx, uint256 minDy) external returns (uint256 dy);
    function addLiquidity(uint256[2] calldata amounts, uint256 minMint) external returns (uint256 mint);
    function removeLiquidity(uint256 amount, uint256[2] calldata minAmounts)
        external
        returns (uint256[2] memory amounts);
}

/// @notice Weighted pool (family 3).
interface ITigerSwapWeightedPool {
    function tokens(uint256 index) external view returns (address);
    function tokenCount() external view returns (uint256);
    function swap(uint256 tokenIn, uint256 tokenOut, uint256 amountIn, uint256 minAmountOut)
        external
        returns (uint256 amountOut);
    function addLiquidity(uint256[] calldata amounts, uint256 minLp) external returns (uint256 lpMinted);
    function removeLiquidity(uint256 lpAmount, uint256[] calldata minAmounts)
        external
        returns (uint256[] memory amounts);
}

interface ITigerSwapPool {
    function token0() external view returns (address);
    function token1() external view returns (address);
    function getReserves() external view returns (uint112 reserve0, uint112 reserve1, uint32 blockTimestampLast);
    function mint(address to) external returns (uint256 liquidity);
    function burn(address to) external returns (uint256 amount0, uint256 amount1);
    function swap(uint256 amount0Out, uint256 amount1Out, address to, bytes calldata data) external;
    /// @notice Settle the protocol's share of accrued swap fees.
    function mintFee() external returns (uint256 liquidity);
}
