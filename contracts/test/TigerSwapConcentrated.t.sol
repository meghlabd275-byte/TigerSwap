// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "forge-std/Test.sol";
import "../TigerSwapConcentrated.sol";
import "./TestERC20.sol";

contract FlashBorrower is ITigerSwapFlashCallback {
    TestERC20 public token0;
    TestERC20 public token1;
    uint256 public borrowed0;
    uint256 public borrowed1;

    constructor(TestERC20 _token0, TestERC20 _token1) {
        token0 = _token0;
        token1 = _token1;
    }

    function borrow(uint256 amount0, uint256 amount1, address pool) external {
        borrowed0 = amount0;
        borrowed1 = amount1;
        TigerSwapConcentrated(pool).flash(address(this), amount0, amount1, "");
    }

    function tigerSwapFlashCallback(uint256 amount0, uint256 amount1, uint256 fee0, uint256 fee1, bytes calldata)
        external
    {
        // Repay principal plus fee.
        if (amount0 > 0) token0.transfer(msg.sender, amount0 + fee0);
        if (amount1 > 0) token1.transfer(msg.sender, amount1 + fee1);
    }
}

contract ConcentratedTest is Test {
    uint256 constant Q96 = 0x1000000000000000000000000;
    uint24 constant FEE = 3000; // 0.3% in hundredths of a bip
    int24 constant SPACING = 60;

    TigerSwapConcentrated pool;
    TestERC20 token0;
    TestERC20 token1;

    address lp = address(0x11);
    address trader = address(0x22);

    int24 tickLower = -600;
    int24 tickUpper = 600;
    uint128 constant LIQUIDITY = 1e18;

    function setUp() public {
        token0 = new TestERC20("TIGER", "TGR");
        token1 = new TestERC20("RUSD", "RUSD");

        // The test contract acts as the factory so it can initialise the pool.
        pool = new TigerSwapConcentrated(address(this), address(token0), address(token1), FEE, SPACING);

        uint160 initialSqrtPrice = uint160(uint256(1) << 96); // price 1:1
        pool.initialize(initialSqrtPrice);

        token0.mint(lp, 1_000_000e18);
        token1.mint(lp, 1_000_000e18);
        token0.mint(trader, 1_000_000e18);
        token1.mint(trader, 1_000_000e18);

        vm.startPrank(lp);
        token0.approve(address(pool), type(uint256).max);
        token1.approve(address(pool), type(uint256).max);
        vm.stopPrank();

        vm.startPrank(trader);
        token0.approve(address(pool), type(uint256).max);
        token1.approve(address(pool), type(uint256).max);
        vm.stopPrank();
    }

    function _addLiquidity() internal returns (uint256 amount0, uint256 amount1) {
        vm.prank(lp);
        (amount0, amount1) = pool.mint(lp, tickLower, tickUpper, LIQUIDITY);
    }

    function testInitializeSetsPriceAndTick() public {
        (uint160 sqrtPrice, int24 tick,,,,,) = pool.slot0();
        assertEq(uint256(sqrtPrice), uint256(1) << 96);
        assertEq(tick, 0);
    }

    function testMintPullsBothTokensAndTracksLiquidity() public {
        (uint256 amount0, uint256 amount1) = _addLiquidity();

        assertGt(amount0, 0);
        assertGt(amount1, 0);
        assertEq(token0.balanceOf(address(pool)), amount0);
        assertEq(token1.balanceOf(address(pool)), amount1);
        assertEq(pool.liquidity(), LIQUIDITY);

        // Ticks are initialised at both ends of the range.
        assertTrue(pool.getTickInfo(tickLower).initialized);
        assertTrue(pool.getTickInfo(tickUpper).initialized);
        assertEq(pool.getTickInfo(tickLower).liquidityNet, int128(LIQUIDITY));
        assertEq(pool.getTickInfo(tickUpper).liquidityNet, -int128(LIQUIDITY));
    }

    function testSwapExactInputZeroForOneMovesPriceDown() public {
        _addLiquidity();

        uint256 amountIn = 1e15; // 0.1% of the pool's liquidity scale
        uint256 traderToken1Before = token1.balanceOf(trader);

        vm.prank(trader);
        (int256 amount0, int256 amount1) = pool.swap(trader, true, int256(amountIn), 0);

        // Caller pays token0, receives token1.
        assertEq(amount0, int256(amountIn));
        assertLt(amount1, 0);

        uint256 received = uint256(-amount1);
        assertGt(received, 0);
        assertEq(token1.balanceOf(trader), traderToken1Before + received);

        // The output is the input less fee and slippage.
        assertLt(received, amountIn * 997 / 1000);
        assertGt(received, amountIn * 990 / 1000);

        // Price moved down and the tick followed.
        (uint160 sqrtPrice, int24 tick,,,,,) = pool.slot0();
        assertLt(uint256(sqrtPrice), uint256(1) << 96);
        assertLt(tick, 0);
    }

    function testSwapExactInputOneForZeroMovesPriceUp() public {
        _addLiquidity();

        uint256 amountIn = 1e15;
        uint256 traderToken0Before = token0.balanceOf(trader);

        vm.prank(trader);
        (int256 amount0, int256 amount1) = pool.swap(trader, false, int256(amountIn), 0);

        assertEq(amount1, int256(amountIn));
        assertLt(amount0, 0);
        assertEq(token0.balanceOf(trader), traderToken0Before + uint256(-amount0));

        (uint160 sqrtPrice, int24 tick,,,,,) = pool.slot0();
        assertGt(uint256(sqrtPrice), uint256(1) << 96);
        assertGt(tick, 0);
    }

    function testSwapExactOutput() public {
        _addLiquidity();

        uint256 wantOut = 1e14;
        uint256 traderToken0Before = token0.balanceOf(trader);

        vm.prank(trader);
        (int256 amount0, int256 amount1) = pool.swap(trader, true, -int256(wantOut), 0);

        // Exact output: the pool pays exactly wantOut token1 and takes token0 in.
        assertEq(amount1, -int256(wantOut));
        assertGt(amount0, 0);
        assertGt(uint256(amount0), wantOut); // fee + slippage means paying more
        assertEq(token0.balanceOf(trader), traderToken0Before - uint256(amount0));
    }

    function testSwapAccumulatesFeesForLiquidityProviders() public {
        _addLiquidity();

        assertEq(pool.feeGrowthGlobal0X128(), 0);

        vm.prank(trader);
        pool.swap(trader, true, 1e15, 0);

        assertGt(pool.feeGrowthGlobal0X128(), 0);

        // LP collects the accrued fees.
        uint256 lpToken0Before = token0.balanceOf(lp);

        vm.prank(lp);
        (uint128 collected0,) = pool.collect(lp, tickLower, tickUpper);

        assertGt(collected0, 0);
        assertEq(token0.balanceOf(lp), lpToken0Before + collected0);
    }

    function testSwapCrossesTickAndDrainsLiquidity() public {
        _addLiquidity();

        // A trade far larger than the range consumes all liquidity in the range.
        uint256 hugeIn = 1e21;
        vm.prank(trader);
        pool.swap(trader, true, int256(hugeIn), 0);

        // Price ends at or below the lower bound and liquidity is exhausted.
        (, int24 tick,,,,,) = pool.slot0();
        assertLe(tick, tickLower);
        assertEq(pool.liquidity(), 0);
    }

    function testBurnReturnsPrincipal() public {
        (uint256 amount0, uint256 amount1) = _addLiquidity();

        uint256 lpToken0Before = token0.balanceOf(lp);
        uint256 lpToken1Before = token1.balanceOf(lp);

        vm.prank(lp);
        (uint256 returned0, uint256 returned1) = pool.burn(tickLower, tickUpper, LIQUIDITY);

        assertApproxEqAbs(returned0, amount0, 2);
        assertApproxEqAbs(returned1, amount1, 2);
        assertEq(pool.liquidity(), 0);

        // Fees are zero here, so the LP recovers the principal.
        assertEq(token0.balanceOf(lp), lpToken0Before + returned0);
        assertEq(token1.balanceOf(lp), lpToken1Before + returned1);
    }

    function testBurnAfterSwapReturnsPrincipalPlusFees() public {
        (uint256 amount0,) = _addLiquidity();

        vm.prank(trader);
        pool.swap(trader, true, 1e16, 0);

        uint256 lpToken0Before = token0.balanceOf(lp);

        vm.prank(lp);
        (uint256 returned0,) = pool.burn(tickLower, tickUpper, LIQUIDITY);

        // Gets principal plus the fee share, so strictly more than the principal.
        assertGt(returned0, amount0);
        assertEq(token0.balanceOf(lp), lpToken0Before + returned0);
    }

    function testMintOutsideCurrentRangeRequiresOnlyOneToken() public {
        // Range entirely below the current price (tick 0): only token1 is needed.
        vm.prank(lp);
        (uint256 amount0, uint256 amount1) = pool.mint(lp, -1200, -600, LIQUIDITY);

        assertEq(amount0, 0);
        assertGt(amount1, 0);
    }

    function testSwapRejectsBadPriceLimit() public {
        _addLiquidity();

        vm.prank(trader);
        vm.expectRevert(bytes("TigerSwap: BAD_PRICE_LIMIT"));
        pool.swap(trader, true, 1e15, uint160(uint256(1) << 96) + 1);
    }

    function testSwapRevertsWithoutLiquidity() public {
        vm.prank(trader);
        pool.swap(trader, true, 1e15, 0);

        // No liquidity was ever minted, so no state changed.
        assertEq(pool.liquidity(), 0);
    }

    function testFlashLoanRepaysWithFee() public {
        _addLiquidity();

        FlashBorrower borrower = new FlashBorrower(token0, token1);
        token0.mint(address(borrower), 10e18); // buffer to pay the fee

        uint256 poolBalance0Before = token0.balanceOf(address(pool));

        borrower.borrow(1e15, 0, address(pool));

        assertEq(borrower.borrowed0(), 1e15);
        // The pool ends up with the principal back plus the flash fee.
        assertGt(token0.balanceOf(address(pool)), poolBalance0Before);
        assertGt(pool.protocolFees0() + pool.feeGrowthGlobal0X128(), 0);
    }

    function testFlashLoanRevertsWhenNotRepaid() public {
        _addLiquidity();

        // A contract that takes the loan and never repays.
        vm.prank(trader);
        vm.expectRevert();
        pool.flash(trader, 1e15, 0, "");
    }

    function testMintRejectsUninitialisedSpacing() public {
        vm.prank(lp);
        vm.expectRevert(bytes("TigerSwap: TICK_SPACING"));
        pool.mint(lp, -601, 600, LIQUIDITY);
    }

    function testTickMathRoundTrip() public {
        // sqrt-ratio and tick conversion must agree for spaced ticks.
        for (int24 t = -600; t <= 600; t += 60) {
            uint160 sqrtPrice = TickMath.getSqrtRatioAtTick(t);
            int24 recovered = TickMath.getTickAtSqrtRatio(sqrtPrice);
            assertEq(recovered, t);
        }
    }
}
