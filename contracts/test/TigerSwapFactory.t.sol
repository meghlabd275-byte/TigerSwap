// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "forge-std/Test.sol";
import "../TigerSwapFactory.sol";
import "../TigerSwapRouter.sol";
import "./TestERC20.sol";

contract TigerSwapFactoryTest is Test {
    TigerSwapFactory public factory;
    TigerSwapRouter public router;
    TestERC20 public tgr;
    TestERC20 public rusd;

    address public alice = address(0xA11CE);
    address public bob = address(0xB0B);
    address public treasury = address(0x7EA5);
    address public stranger = address(0x5BAD);

    uint256 constant LIQUIDITY = 1_000_000e18;

    function setUp() public {
        tgr = new TestERC20("TGR", "TGR");
        rusd = new TestERC20("RUSD", "RUSD");
        factory = new TigerSwapFactory();
        router = new TigerSwapRouter(address(factory), address(0));

        tgr.mint(alice, 10_000_000e18);
        rusd.mint(alice, 10_000_000e18);
        tgr.mint(bob, 10_000_000e18);
        rusd.mint(bob, 10_000_000e18);

        for (uint256 i = 0; i < 2; i++) {
            address who = [alice, bob][i];
            vm.prank(who);
            tgr.approve(address(router), type(uint256).max);
            vm.prank(who);
            rusd.approve(address(router), type(uint256).max);
        }

        factory.createPool(address(tgr), address(rusd));
    }

    function _pool() internal view returns (address) {
        return factory.getPoolAddress(address(tgr), address(rusd));
    }

    function _addLiquidity(address who, uint256 a, uint256 b) internal returns (uint256 liquidity) {
        vm.prank(who);
        (,, liquidity) = router.addLiquidity(
            address(tgr), address(rusd), a, b, 0, 0, who, block.timestamp + 1 days
        );
    }

    function _swap(address who, uint256 amountIn) internal returns (uint256 out) {
        address[] memory path = new address[](2);
        path[0] = address(tgr);
        path[1] = address(rusd);

        vm.prank(who);
        uint256[] memory amounts =
            router.swapExactTokensForTokens(amountIn, 0, path, who, block.timestamp + 1 days);
        out = amounts[1];
    }

    // ---- Pool creation -----------------------------------------------------

    function testPoolIsCreatedWithTokensSorted() public {
        address pool = _pool();
        assertTrue(pool != address(0), "pool exists");

        // token0 is the lower address, whichever order they were passed in.
        (address t0, address t1) = address(tgr) < address(rusd)
            ? (address(tgr), address(rusd))
            : (address(rusd), address(tgr));

        assertEq(TigerSwapPool(pool).token0(), t0, "token0 sorted");
        assertEq(TigerSwapPool(pool).token1(), t1, "token1 sorted");
    }

    function testCannotCreateTheSamePoolTwice() public {
        vm.expectRevert("TigerSwap: POOL_EXISTS");
        factory.createPool(address(tgr), address(rusd));

        // Reversed order is the same pool.
        vm.expectRevert("TigerSwap: POOL_EXISTS");
        factory.createPool(address(rusd), address(tgr));
    }

    function testCannotCreatePoolWithIdenticalOrZeroTokens() public {
        vm.expectRevert("TigerSwap: IDENTICAL_ADDRESSES");
        factory.createPool(address(tgr), address(tgr));

        vm.expectRevert("TigerSwap: ZERO_ADDRESS");
        factory.createPool(address(0), address(tgr));
    }

    function testPoolIsRegisteredInTheFactory() public {
        assertTrue(factory.isPool(_pool()), "registered");
        assertEq(factory.allPoolsLength(), 1, "one pool");
    }

    function testCreatePoolIsBlockedWhilePaused() public {
        TestERC20 other = new TestERC20("OTHER", "OTH");

        factory.pause();
        vm.expectRevert("TigerSwap: PAUSED");
        factory.createPool(address(tgr), address(other));

        factory.unpause();
        factory.createPool(address(tgr), address(other));
        assertTrue(factory.getPoolAddress(address(tgr), address(other)) != address(0), "created after unpause");
    }

    // ---- Liquidity ---------------------------------------------------------

    function testAddLiquidityMintsLpTokens() public {
        uint256 liquidity = _addLiquidity(alice, LIQUIDITY, LIQUIDITY);
        assertGt(liquidity, 0, "lp minted");
        assertEq(TigerSwapPool(_pool()).balanceOf(alice), liquidity, "credited to alice");
    }

    function testFirstLiquidityLocksTheMinimumSupply() public {
        _addLiquidity(alice, LIQUIDITY, LIQUIDITY);

        // Uniswap V2 locks the smallest unit forever so the pool can never be
        // drained to a zero total supply.
        assertEq(TigerSwapPool(_pool()).balanceOf(address(0)), 1000, "minimum locked");
    }

    function testReservesMatchTheDeposits() public {
        _addLiquidity(alice, LIQUIDITY, LIQUIDITY);

        (uint112 r0, uint112 r1,) = TigerSwapPool(_pool()).getReserves();
        (address t0,) = address(tgr) < address(rusd) ? (address(tgr), address(rusd)) : (address(rusd), address(tgr));

        if (t0 == address(tgr)) {
            assertEq(uint256(r0), LIQUIDITY, "reserve0");
            assertEq(uint256(r1), LIQUIDITY, "reserve1");
        } else {
            assertEq(uint256(r0), LIQUIDITY, "reserve0");
            assertEq(uint256(r1), LIQUIDITY, "reserve1");
        }
    }

    function testSecondLiquidityProviderGetsProportionalShares() public {
        uint256 aliceLp = _addLiquidity(alice, LIQUIDITY, LIQUIDITY);
        uint256 bobLp = _addLiquidity(bob, LIQUIDITY, LIQUIDITY);

        // Equal deposits into an equal pool earn equal shares.
        assertApproxEqRel(bobLp, aliceLp, 1e15, "proportional shares");
    }

    function testRemoveLiquidityReturnsTheUnderlyingTokens() public {
        uint256 liquidity = _addLiquidity(alice, LIQUIDITY, LIQUIDITY);

        address pool = _pool();
        vm.prank(alice);
        TigerSwapPool(pool).approve(address(router), liquidity);

        uint256 tgrBefore = tgr.balanceOf(alice);
        uint256 rusdBefore = rusd.balanceOf(alice);

        vm.prank(alice);
        router.removeLiquidity(
            address(tgr), address(rusd), liquidity, 0, 0, alice, block.timestamp + 1 days
        );

        assertGt(tgr.balanceOf(alice), tgrBefore, "tgr returned");
        assertGt(rusd.balanceOf(alice), rusdBefore, "rusd returned");
    }

    // ---- Swaps -------------------------------------------------------------

    function testSwapMovesTokensAndChargesTheFee() public {
        _addLiquidity(alice, LIQUIDITY, LIQUIDITY);

        uint256 out = _swap(alice, 1000e18);
        assertGt(out, 0, "output received");
        // With a 0.3% fee the output is slightly under the constant-product
        // ideal of 1000 * 1e6 / (1e6 + 1000) ~= 999.0.
        assertLt(out, 1000e18, "fee is taken");
    }

    function testSwapKeepsTheInvariant() public {
        _addLiquidity(alice, LIQUIDITY, LIQUIDITY);
        _swap(alice, 1000e18);

        (uint112 r0, uint112 r1,) = TigerSwapPool(_pool()).getReserves();
        uint256 k = uint256(r0) * uint256(r1);
        // The product only grows, because the fee stays in the pool.
        assertGe(k, LIQUIDITY * LIQUIDITY, "invariant never decreases");
    }

    function testSwapRevertsWithoutLiquidity() public {
        address[] memory path = new address[](2);
        path[0] = address(tgr);
        path[1] = address(rusd);

        // The pool exists but holds no reserves.
        vm.prank(alice);
        vm.expectRevert("TigerSwap: INSUFFICIENT_LIQUIDITY");
        router.swapExactTokensForTokens(1e18, 0, path, alice, block.timestamp + 1 days);
    }

    function testSwapRevertsForAnUnknownPool() public {
        TestERC20 other = new TestERC20("OTHER", "OTH");
        other.mint(alice, 1e18);
        vm.prank(alice);
        other.approve(address(router), type(uint256).max);

        address[] memory path = new address[](2);
        path[0] = address(tgr);
        path[1] = address(other);

        vm.prank(alice);
        vm.expectRevert("TigerSwap: POOL_NOT_FOUND");
        router.swapExactTokensForTokens(1e18, 0, path, alice, block.timestamp + 1 days);
    }

    function testSwapRespectsTheMinimumOutput() public {
        _addLiquidity(alice, LIQUIDITY, LIQUIDITY);

        address[] memory path = new address[](2);
        path[0] = address(tgr);
        path[1] = address(rusd);

        vm.prank(alice);
        vm.expectRevert("TigerSwap: INSUFFICIENT_OUTPUT_AMOUNT");
        router.swapExactTokensForTokens(1000e18, 1000e18, path, alice, block.timestamp + 1 days);
    }

    function testSwapRevertsAfterTheDeadline() public {
        _addLiquidity(alice, LIQUIDITY, LIQUIDITY);

        address[] memory path = new address[](2);
        path[0] = address(tgr);
        path[1] = address(rusd);

        vm.warp(block.timestamp + 2 days);
        vm.prank(alice);
        vm.expectRevert("TigerSwap: EXPIRED");
        router.swapExactTokensForTokens(1000e18, 0, path, alice, block.timestamp - 1);
    }

    // ---- Protocol fee ------------------------------------------------------

    function testNoProtocolFeeWhileFeeToIsUnset() public {
        _addLiquidity(alice, LIQUIDITY, LIQUIDITY);
        _swap(alice, 1000e18);

        assertEq(factory.feeTo(), address(0), "unset by default");
        assertEq(TigerSwapPool(_pool()).mintFee(), 0, "nothing minted");
    }

    function testProtocolFeeAccruesToFeeTo() public {
        _addLiquidity(alice, LIQUIDITY, LIQUIDITY);

        factory.setFeeTo(treasury);
        assertEq(factory.feeTo(), treasury, "fee recipient set");

        // Generate fees with swaps.
        _swap(alice, 10_000e18);
        _swap(alice, 10_000e18);

        address pool = _pool();
        vm.prank(stranger);
        uint256 minted = TigerSwapPool(pool).mintFee();

        assertGt(minted, 0, "protocol fee minted");
        assertEq(TigerSwapPool(pool).balanceOf(treasury), minted, "credited to the treasury");
    }

    function testProtocolFeeIsBoundedByTheAccruedFees() public {
        _addLiquidity(alice, LIQUIDITY, LIQUIDITY);
        factory.setFeeTo(treasury);

        _swap(alice, 10_000e18);

        address pool = _pool();
        uint256 supplyBefore = TigerSwapPool(pool).totalSupply();
        vm.prank(stranger);
        uint256 minted = TigerSwapPool(pool).mintFee();

        // The protocol's cut is a small fraction of LP growth, never a
        // meaningful share of the pool.
        assertLt(minted, supplyBefore / 100, "protocol share is small");
    }

    function testMintFeeIsIdempotentWhenNoNewFeesAccrued() public {
        _addLiquidity(alice, LIQUIDITY, LIQUIDITY);
        factory.setFeeTo(treasury);
        _swap(alice, 10_000e18);

        address pool = _pool();
        vm.prank(stranger);
        uint256 first = TigerSwapPool(pool).mintFee();

        vm.prank(stranger);
        uint256 second = TigerSwapPool(pool).mintFee();

        assertGt(first, 0, "first accrual");
        assertEq(second, 0, "nothing new to accrue");
    }

    function testLiquidityChangesDoNotCreateAProtocolFee() public {
        _addLiquidity(alice, LIQUIDITY, LIQUIDITY);
        factory.setFeeTo(treasury);

        // Adding liquidity grows the reserves but is not a swap fee, so the
        // fee baseline must be reset and no protocol share minted.
        _addLiquidity(bob, LIQUIDITY, LIQUIDITY);

        vm.prank(stranger);
        assertEq(TigerSwapPool(_pool()).mintFee(), 0, "no fee from a liquidity change");
    }

    function testProtocolFeeIsSkippedWhenConfiguredToZero() public {
        _addLiquidity(alice, LIQUIDITY, LIQUIDITY);
        factory.setFeeTo(treasury);
        factory.setProtocolFee(0);
        _swap(alice, 10_000e18);

        vm.prank(stranger);
        assertEq(TigerSwapPool(_pool()).mintFee(), 0, "zero protocol fee");
    }

    function testProtocolFeeIsCapped() public {
        vm.expectRevert("TigerSwap: FEE_TOO_HIGH");
        factory.setProtocolFee(101);
    }

    function testSwapFeeIsCapped() public {
        vm.expectRevert("TigerSwap: FEE_TOO_HIGH");
        factory.setSwapFee(1001);
    }

    // ---- Admin -------------------------------------------------------------

    function testOnlyOwnerCanAdminister() public {
        vm.prank(stranger);
        vm.expectRevert("TigerSwap: FORBIDDEN");
        factory.setSwapFee(10);

        vm.prank(stranger);
        vm.expectRevert("TigerSwap: FORBIDDEN");
        factory.setProtocolFee(1);

        vm.prank(stranger);
        vm.expectRevert("TigerSwap: FORBIDDEN");
        factory.setFeeTo(treasury);

        vm.prank(stranger);
        vm.expectRevert("TigerSwap: FORBIDDEN");
        factory.pause();
    }

    function testTransferOwnershipRejectsZeroAddress() public {
        vm.expectRevert("TigerSwap: ZERO_ADDRESS");
        factory.transferOwnership(address(0));
    }

    function testTransferOwnershipMovesAdmin() public {
        factory.transferOwnership(bob);
        assertEq(factory.owner(), bob, "owner moved");

        vm.prank(bob);
        factory.setSwapFee(10);
        assertEq(factory.swapFee(), 10, "new owner can administer");
    }
}
