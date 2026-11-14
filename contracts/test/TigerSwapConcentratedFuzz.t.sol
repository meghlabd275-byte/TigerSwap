// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "forge-std/Test.sol";
import "../TigerSwapConcentrated.sol";
import "./TestERC20.sol";

/// @title Fuzz tests for the concentrated pool's core invariants.
/// @notice A concentrated pool's safety property is that a swap is bounded by
///         the liquidity actually in range: it can never pay out more than the
///         pool holds, it never takes more input than the caller specified, the
///         price always moves in the direction the trade pushes it, and the fee
///         is retained so an LP's principal is never reduced by a swap. These
///         pin those across the whole input domain.
contract TigerSwapConcentratedFuzzTest is Test {
    uint256 constant Q96 = 0x1000000000000000000000000;
    uint24 constant FEE = 3000; // 0.3%
    int24 constant SPACING = 60;

    TigerSwapConcentrated pool;
    TestERC20 token0;
    TestERC20 token1;

    address lp = address(0x11);
    address trader = address(0x22);

    int24 tickLower = -600;
    int24 tickUpper = 600;
    uint128 constant LIQUIDITY = 1e18;

    /// @dev The narrow [-600, 600] range holds roughly 3e16 of either token
    ///      before the price reaches an edge. Keeping a swap under 1e16 keeps it
    ///      inside the range, where the input is consumed in full.
    uint256 constant MAX_IN_RANGE_SWAP = 1e16;

    function setUp() public {
        token0 = new TestERC20("TIGER", "TGR");
        token1 = new TestERC20("RUSD", "RUSD");

        pool = new TigerSwapConcentrated(address(this), address(token0), address(token1), FEE, SPACING);
        pool.initialize(uint160(uint256(1) << 96));

        token0.mint(lp, 1e30);
        token1.mint(lp, 1e30);
        token0.mint(trader, 1e30);
        token1.mint(trader, 1e30);
        token0.mint(address(this), 1e30);
        token1.mint(address(this), 1e30);

        vm.startPrank(lp);
        token0.approve(address(pool), type(uint256).max);
        token1.approve(address(pool), type(uint256).max);
        pool.mint(lp, tickLower, tickUpper, LIQUIDITY);
        vm.stopPrank();

        vm.startPrank(trader);
        token0.approve(address(pool), type(uint256).max);
        token1.approve(address(pool), type(uint256).max);
        vm.stopPrank();

        token0.approve(address(pool), type(uint256).max);
        token1.approve(address(pool), type(uint256).max);
    }

    /// @notice A zero-for-one swap can never pay out more token1 than the pool
    ///         holds, and the trader always receives a positive amount.
    function testFuzzSwapOutputIsBoundedByPoolBalance(uint256 amountIn) public {
        amountIn = bound(amountIn, 1e6, MAX_IN_RANGE_SWAP);

        uint256 poolBalance1 = token1.balanceOf(address(pool));

        vm.prank(trader);
        (int256 amount0, int256 amount1) = pool.swap(trader, true, int256(amountIn), 0);

        assertEq(amount0, int256(amountIn), "input amount was not taken in full");
        assertLt(amount1, 0, "a zero-for-one swap must pay out token1");
        assertLe(uint256(-amount1), poolBalance1, "payout exceeded the pool's token1 balance");
        assertGt(uint256(-amount1), 0, "payout was zero");
    }

    /// @notice A one-for-zero swap pays out token0, bounded by the pool balance.
    function testFuzzReverseSwapOutputIsBoundedByPoolBalance(uint256 amountIn) public {
        amountIn = bound(amountIn, 1e6, MAX_IN_RANGE_SWAP);

        uint256 poolBalance0 = token0.balanceOf(address(pool));

        vm.prank(trader);
        (int256 amount0, int256 amount1) = pool.swap(trader, false, int256(amountIn), 0);

        assertEq(amount1, int256(amountIn), "input amount was not taken in full");
        assertLt(amount0, 0, "a one-for-zero swap must pay out token0");
        assertLe(uint256(-amount0), poolBalance0, "payout exceeded the pool's token0 balance");
        assertGt(uint256(-amount0), 0, "payout was zero");
    }

    /// @notice Even a swap large enough to exhaust the range never takes more
    ///         input than the caller specified, and never pays out more than the
    ///         pool holds.
    function testFuzzLargeSwapNeverOverchargesOrOverpays(uint256 amountIn) public {
        amountIn = bound(amountIn, 1e6, 1e24);

        uint256 poolBalance1 = token1.balanceOf(address(pool));

        vm.prank(trader);
        (int256 amount0, int256 amount1) = pool.swap(trader, true, int256(amountIn), 0);

        assertLe(uint256(amount0), amountIn, "swap took more input than specified");
        if (amount1 < 0) {
            assertLe(uint256(-amount1), poolBalance1, "payout exceeded the pool's token1 balance");
        }
    }

    /// @notice A zero-for-one swap moves the price down; one-for-zero moves it
    ///         up. The direction is never inverted.
    function testFuzzSwapMovesPriceInTheTradedDirection(uint256 amountIn, bool zeroForOne) public {
        amountIn = bound(amountIn, 1e6, MAX_IN_RANGE_SWAP);

        (uint160 before,,,,,,) = pool.slot0();

        vm.prank(trader);
        pool.swap(trader, zeroForOne, int256(amountIn), 0);

        (uint160 after_,,,,,,) = pool.slot0();
        if (zeroForOne) {
            assertLt(uint256(after_), uint256(before), "zero-for-one swap did not lower the price");
        } else {
            assertGt(uint256(after_), uint256(before), "one-for-zero swap did not raise the price");
        }
    }

    /// @notice A swap retains a fee: the trader receives strictly less than the
    ///         input's fair value, so the fee is not paid out of LP principal.
    ///         With the price starting at 1:1 and staying near it, the output
    ///         must be below the input.
    function testFuzzSwapRetainsAFee(uint256 amountIn) public {
        // A small trade keeps the price close to 1:1, where a fee-free swap
        // would return approximately the input.
        amountIn = bound(amountIn, 1e6, 1e14);

        vm.prank(trader);
        (int256 amount0, int256 amount1) = pool.swap(trader, true, int256(amountIn), 0);

        assertEq(amount0, int256(amountIn));
        assertLt(uint256(-amount1), amountIn, "a swap returned more than it took in at parity");
    }

    /// @notice Adding then immediately removing the same liquidity cannot return
    ///         more than was deposited.
    function testFuzzMintBurnRoundTripIsNotProfitable(uint128 amount) public {
        amount = uint128(bound(amount, 1e6, 1e18));

        (uint256 amount0, uint256 amount1) = pool.mint(address(this), tickLower, tickUpper, amount);

        (uint256 back0, uint256 back1) = pool.burn(tickLower, tickUpper, amount);

        assertLe(back0, amount0, "burn returned more token0 than was minted");
        assertLe(back1, amount1, "burn returned more token1 than was minted");
    }
}
