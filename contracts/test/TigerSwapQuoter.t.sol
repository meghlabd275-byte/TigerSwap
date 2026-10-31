// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "forge-std/Test.sol";
import "../TigerSwapConcentrated.sol";
import "../TigerSwapFactory.sol";
import "../TigerSwapQuoter.sol";
import "./TestERC20.sol";

/// @notice Verifies the quoter reproduces what the pool actually pays out.
contract TigerSwapQuoterTest is Test {
    uint24 constant FEE = 3000; // 0.3% in hundredths of a bip
    int24 constant SPACING = 60;
    uint128 constant LIQUIDITY = 1e18;

    TigerSwapFactory factory;
    TigerSwapConcentrated poolA; // token0 / token1
    TigerSwapConcentrated poolB; // token1 / token2
    TigerSwapQuoter quoter;

    TestERC20 token0;
    TestERC20 token1;
    TestERC20 token2;

    address lp = address(0x11);
    address trader = address(0x22);

    int24 tickLower = -600;
    int24 tickUpper = 600;

    function setUp() public {
        token0 = new TestERC20("TIGER", "TGR");
        token1 = new TestERC20("RUSD", "RUSD");
        token2 = new TestERC20("USDX", "USDX");

        factory = new TigerSwapFactory();

        poolA = new TigerSwapConcentrated(address(factory), address(token0), address(token1), FEE, SPACING);
        poolB = new TigerSwapConcentrated(address(factory), address(token1), address(token2), FEE, SPACING);

        factory.registerFamilyPool(address(token0), address(token1), 1, address(poolA));
        factory.registerFamilyPool(address(token1), address(token2), 1, address(poolB));

        quoter = new TigerSwapQuoter(address(factory));

        // The pool only accepts initialization from its factory.
        vm.startPrank(address(factory));
        poolA.initialize(uint160(uint256(1) << 96));
        poolB.initialize(uint160(uint256(1) << 96));
        vm.stopPrank();

        _fund(lp);
        _fund(trader);

        vm.startPrank(lp);
        token0.approve(address(poolA), type(uint256).max);
        token1.approve(address(poolA), type(uint256).max);
        token1.approve(address(poolB), type(uint256).max);
        token2.approve(address(poolB), type(uint256).max);
        vm.stopPrank();

        vm.startPrank(trader);
        token0.approve(address(poolA), type(uint256).max);
        token1.approve(address(poolA), type(uint256).max);
        token1.approve(address(poolB), type(uint256).max);
        token2.approve(address(poolB), type(uint256).max);
        vm.stopPrank();
    }

    function _fund(address who) internal {
        token0.mint(who, 1_000_000e18);
        token1.mint(who, 1_000_000e18);
        token2.mint(who, 1_000_000e18);
    }

    function _addLiquidity() internal {
        vm.startPrank(lp);
        poolA.mint(lp, tickLower, tickUpper, LIQUIDITY);
        poolB.mint(lp, tickLower, tickUpper, LIQUIDITY);
        vm.stopPrank();
    }

    function testQuoteMatchesExecutionZeroForOne() public {
        _addLiquidity();
        uint256 amountIn = 1e15;

        uint256 quoted = quoter.quoteExactInputSingle(address(token0), address(token1), FEE, amountIn);

        vm.prank(trader);
        (, int256 amount1) = poolA.swap(trader, true, int256(amountIn), 0);

        assertEq(quoted, uint256(-amount1), "quote must equal tokens actually received");
    }

    function testQuoteMatchesExecutionOneForZero() public {
        _addLiquidity();
        uint256 amountIn = 1e15;

        uint256 quoted = quoter.quoteExactInputSingle(address(token1), address(token0), FEE, amountIn);

        vm.prank(trader);
        (int256 amount0,) = poolA.swap(trader, false, int256(amountIn), 0);

        assertEq(quoted, uint256(-amount0), "quote must equal tokens actually received");
    }

    function testQuoteMatchesExecutionForLargeSwapAcrossTicks() public {
        _addLiquidity();
        // Large enough to walk several tick steps within the range.
        uint256 amountIn = 100e18;

        uint256 quoted = quoter.quoteExactInputSingle(address(token0), address(token1), FEE, amountIn);

        vm.prank(trader);
        (, int256 amount1) = poolA.swap(trader, true, int256(amountIn), 0);

        assertEq(quoted, uint256(-amount1));
    }

    function testQuoteIsPureAndDoesNotMoveThePool() public {
        _addLiquidity();

        uint160 priceBefore = poolA.sqrtPriceX96();
        int24 tickBefore = poolA.tick();
        uint256 trader0Before = token0.balanceOf(trader);

        quoter.quoteExactInputSingle(address(token0), address(token1), FEE, 1e15);

        assertEq(poolA.sqrtPriceX96(), priceBefore, "quote must not move price");
        assertEq(poolA.tick(), tickBefore, "quote must not move tick");
        assertEq(token0.balanceOf(trader), trader0Before, "quote must not move tokens");
    }

    function testQuoteIsMonotonicInAmountIn() public {
        _addLiquidity();

        uint256 small = quoter.quoteExactInputSingle(address(token0), address(token1), FEE, 1e15);
        uint256 large = quoter.quoteExactInputSingle(address(token0), address(token1), FEE, 1e18);

        assertGt(large, small, "a larger input must quote a larger output");
    }

    function testQuoteReflectsFee() public {
        _addLiquidity();

        uint256 amountIn = 1e15;
        uint256 quoted = quoter.quoteExactInputSingle(address(token0), address(token1), FEE, amountIn);

        // Output is the input less the fee, and less slippage on top of that.
        uint256 feeFree = amountIn - (amountIn * FEE) / 1_000_000;
        assertLt(quoted, feeFree, "the fee must be deducted");
        // At this depth the price impact stays well under 0.1%.
        assertGt(quoted, (feeFree * 999) / 1000, "slippage must be small at this depth");
    }

    function testQuoteExactInputMultiHopMatchesExecution() public {
        _addLiquidity();
        uint256 amountIn = 1e15;

        bytes memory path = abi.encodePacked(address(token0), FEE, address(token1), FEE, address(token2));
        uint256 quoted = quoter.quoteExactInput(path, amountIn);

        vm.prank(trader);
        (, int256 mid) = poolA.swap(trader, true, int256(amountIn), 0);
        uint256 hopOut = uint256(-mid);

        vm.prank(trader);
        (int256 amount1,) = poolB.swap(trader, false, int256(hopOut), 0);

        assertEq(quoted, uint256(-amount1), "multi-hop quote must equal execution");
    }

    function testQuoteExactInputSingleHopMatchesSingle() public {
        _addLiquidity();
        uint256 amountIn = 1e15;

        bytes memory path = abi.encodePacked(address(token0), FEE, address(token1));
        assertEq(
            quoter.quoteExactInput(path, amountIn),
            quoter.quoteExactInputSingle(address(token0), address(token1), FEE, amountIn)
        );
    }

    function testQuoteRevertsForUnknownPool() public {
        TestERC20 stranger = new TestERC20("X", "X");
        vm.expectRevert(abi.encodeWithSelector(TigerSwapQuoter.UnknownPool.selector, address(token0), address(stranger)));
        quoter.quoteExactInputSingle(address(token0), address(stranger), FEE, 1e15);
    }

    function testQuoteRevertsForWrongFeeTier() public {
        _addLiquidity();
        vm.expectRevert(abi.encodeWithSelector(TigerSwapQuoter.InvalidFee.selector, uint24(500)));
        quoter.quoteExactInputSingle(address(token0), address(token1), 500, 1e15);
    }

    function testQuoteRevertsForIdenticalTokens() public {
        vm.expectRevert(TigerSwapQuoter.IdenticalTokens.selector);
        quoter.quoteExactInputSingle(address(token0), address(token0), FEE, 1e15);
    }

    function testQuoteRevertsForZeroLiquidity() public {
        vm.expectRevert(TigerSwapQuoter.NoLiquidity.selector);
        quoter.quoteExactInputSingle(address(token0), address(token1), FEE, 1e15);
    }

    function testQuoteOfZeroIsZero() public {
        _addLiquidity();
        assertEq(quoter.quoteExactInputSingle(address(token0), address(token1), FEE, 0), 0);
    }

    function testMultiHopRejectsShortPath() public {
        vm.expectRevert(abi.encodeWithSelector(TigerSwapQuoter.UnknownPool.selector, address(0), address(0)));
        quoter.quoteExactInput(abi.encodePacked(address(token0)), 1e15);
    }

    function testQuoterRejectsZeroFactory() public {
        vm.expectRevert(TigerSwapQuoter.ZeroAddress.selector);
        new TigerSwapQuoter(address(0));
    }
}
