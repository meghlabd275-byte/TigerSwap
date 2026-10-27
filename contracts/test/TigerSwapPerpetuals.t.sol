// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "forge-std/Test.sol";
import "../TigerSwapPerpetuals.sol";
import "./TestERC20.sol";

contract TigerSwapPerpetualsTest is Test {
    TigerSwapPerpetuals public perps;
    TestERC20 public rusd;

    address public trader = address(0x11);
    address public other = address(0x22);
    address public keeper = address(0x33);

    bytes32 constant BTC = keccak256("BTC/RUSD");
    bytes32 constant ETH = keccak256("ETH/RUSD");

    /// @dev Prices are 1e18 scaled, matching PRICE_PRECISION, so a notional is
    ///      directly comparable to collateral in the same units.
    uint256 constant BTC_PRICE = 60_000e18;
    uint256 constant ETH_PRICE = 3_000e18;
    uint256 constant INITIAL_MARGIN = 1e17; // 10%
    uint256 constant MAINT_MARGIN = 5e16; // 5%
    uint256 constant LIQ_FEE = 5e16; // 5%
    /// @dev Pool liquidity seeded in setUp, which the engine also holds.
    uint256 constant POOL_LIQUIDITY = 5_000_000e18;

    function setUp() public {
        rusd = new TestERC20("RUSD", "RUSD");
        perps = new TigerSwapPerpetuals();

        perps.setOracle(address(this));

        perps.createMarket(BTC, address(rusd), 10e18, LIQ_FEE, MAINT_MARGIN, INITIAL_MARGIN);
        perps.createMarket(ETH, address(rusd), 10e18, LIQ_FEE, MAINT_MARGIN, INITIAL_MARGIN);

        // Seed prices.
        perps.updatePrice(BTC, BTC_PRICE);
        perps.updatePrice(ETH, ETH_PRICE);

        rusd.mint(trader, 10_000_000e18);
        rusd.mint(other, 10_000_000e18);

        // Liquidity providers fund the pool that pays trader profits. Without
        // this the engine would be paying winners out of other traders' margin.
        rusd.mint(address(this), POOL_LIQUIDITY);
        rusd.approve(address(perps), type(uint256).max);
        perps.addLiquidity(address(rusd), POOL_LIQUIDITY);

        vm.prank(trader);
        rusd.approve(address(perps), type(uint256).max);
        vm.prank(other);
        rusd.approve(address(perps), type(uint256).max);
    }

    // ---- Admin -------------------------------------------------------------

    function testCreateMarketStoresConfig() public view {
        TigerSwapPerpetuals.MarketView memory m = perps.getMarket(BTC);
        assertEq(m.token, address(rusd), "token");
        assertEq(m.maxLeverage, 10e18, "max leverage");
        assertEq(m.liquidationFee, LIQ_FEE, "liquidation fee");
        assertEq(m.maintenanceMargin, MAINT_MARGIN, "maintenance margin");
        assertEq(m.initialMargin, INITIAL_MARGIN, "initial margin");
        assertTrue(m.isActive, "active");
    }

    function testCreateMarketRejectsDuplicate() public {
        vm.expectRevert("TigerSwap: MARKET_EXISTS");
        perps.createMarket(BTC, address(rusd), 10e18, LIQ_FEE, MAINT_MARGIN, INITIAL_MARGIN);
    }

    function testOnlyOwnerCanCreateMarket() public {
        vm.prank(trader);
        vm.expectRevert("TigerSwap: NOT_OWNER");
        perps.createMarket(keccak256("X"), address(rusd), 10e18, LIQ_FEE, MAINT_MARGIN, INITIAL_MARGIN);
    }

    function testOnlyOracleCanSetPrice() public {
        vm.prank(trader);
        vm.expectRevert("TigerSwap: NOT_ORACLE");
        perps.updatePrice(BTC, 61_000e18);
    }

    function testSetMarketActive() public {
        perps.setMarketActive(BTC, false);
        assertFalse(perps.getMarket(BTC).isActive, "paused");

        vm.prank(trader);
        vm.expectRevert("TigerSwap: MARKET_NOT_ACTIVE");
        perps.openPosition(BTC, 1e18, 1e22, true);
    }

    // ---- Opening -----------------------------------------------------------

    function testOpenPositionTransfersCollateral() public {
        uint256 collateral = 100_000e18;
        uint256 size = 1e18; // 1 BTC -> 60,000 notional

        vm.prank(trader);
        bytes32 id = perps.openPosition(BTC, size, collateral, true);

        assertEq(rusd.balanceOf(address(perps)), POOL_LIQUIDITY + collateral, "collateral held alongside the pool");
        (, address who, uint256 sz, uint256 col, uint256 entry, , bool isLong) = perps.getPosition(id);
        assertEq(who, trader, "trader");
        assertEq(sz, size, "size");
        assertEq(col, collateral, "collateral");
        assertEq(entry, BTC_PRICE, "entry price");
        assertTrue(isLong, "long");
    }

    function testTwoPositionsInOneBlockGetDistinctIds() public {
        vm.startPrank(trader);
        bytes32 first = perps.openPosition(BTC, 1e18, 100_000e18, true);
        bytes32 second = perps.openPosition(BTC, 2e18, 200_000e18, true);
        vm.stopPrank();

        assertTrue(first != second, "ids must differ");

        // Both positions must still exist with their own collateral.
        (, , uint256 size1, uint256 col1, , , ) = perps.getPosition(first);
        (, , uint256 size2, uint256 col2, , , ) = perps.getPosition(second);
        assertEq(size1, 1e18, "first size intact");
        assertEq(col1, 100_000e18, "first collateral intact");
        assertEq(size2, 2e18, "second size");
        assertEq(col2, 200_000e18, "second collateral");

        assertEq(rusd.balanceOf(address(perps)), POOL_LIQUIDITY + 300_000e18, "both collaterals held");
    }

    function testTraderPositionsAreListed() public {
        vm.startPrank(trader);
        bytes32 a = perps.openPosition(BTC, 1e18, 100_000e18, true);
        bytes32 b = perps.openPosition(ETH, 1e18, 100_000e18, true);
        vm.stopPrank();

        bytes32[] memory ids = perps.getTraderPositions(trader);
        assertEq(ids.length, 2, "two positions");
        assertEq(ids[0], a, "first id");
        assertEq(ids[1], b, "second id");

        bytes32[] memory open = perps.getOpenPositions(trader);
        assertEq(open.length, 2, "both open");

        vm.prank(trader);
        perps.closePosition(a);

        open = perps.getOpenPositions(trader);
        assertEq(open.length, 1, "one still open");
        assertEq(open[0], b, "remaining position");
    }

    function testOpenRejectsInsufficientMargin() public {
        // 1 BTC = 60,000 notional, 10% initial margin = 6,000.
        vm.prank(trader);
        vm.expectRevert("TigerSwap: INSUFFICIENT_MARGIN");
        perps.openPosition(BTC, 1e18, 5_999e18, true);
    }

    function testOpenRejectsExcessiveLeverage() public {
        // A market whose initial margin (5%) is below 1/maxLeverage (20%) so the
        // leverage cap is what actually binds.
        bytes32 sol = keccak256("SOL/RUSD");
        perps.createMarket(sol, address(rusd), 5e18, LIQ_FEE, 4e16, 5e16);
        perps.updatePrice(sol, 100e18);

        // 1 SOL at 100 is 100 notional; 5 collateral is 20x, over the 5x cap.
        vm.prank(trader);
        vm.expectRevert("TigerSwap: LEVERAGE_TOO_HIGH");
        perps.openPosition(sol, 1e18, 5e18, true);
    }

    function testOpenRejectsUnknownMarket() public {
        vm.prank(trader);
        vm.expectRevert("TigerSwap: MARKET_NOT_ACTIVE");
        perps.openPosition(keccak256("NOPE"), 1e18, 1e22, true);
    }

    // ---- PnL ---------------------------------------------------------------

    function testLongProfitPaysOut() public {
        uint256 collateral = 100_000e18;
        vm.prank(trader);
        bytes32 id = perps.openPosition(BTC, 1e18, collateral, true);

        // Price rises 10%: 6,000 profit on 1 BTC.
        perps.updatePrice(BTC, 66_000e18);

        uint256 before = rusd.balanceOf(trader);
        vm.prank(trader);
        perps.closePosition(id);

        assertEq(rusd.balanceOf(trader), before + collateral + 6_000e18, "collateral plus profit");
    }

    function testLongLossReducesPayout() public {
        uint256 collateral = 100_000e18;
        vm.prank(trader);
        bytes32 id = perps.openPosition(BTC, 1e18, collateral, true);

        // Price falls 10%: 6,000 loss.
        perps.updatePrice(BTC, 54_000e18);

        uint256 before = rusd.balanceOf(trader);
        vm.prank(trader);
        perps.closePosition(id);

        assertEq(rusd.balanceOf(trader), before + collateral - 6_000e18, "loss deducted");
    }

    function testShortProfitsWhenPriceFalls() public {
        uint256 collateral = 100_000e18;
        vm.prank(trader);
        bytes32 id = perps.openPosition(BTC, 1e18, collateral, false);

        perps.updatePrice(BTC, 54_000e18);

        uint256 before = rusd.balanceOf(trader);
        vm.prank(trader);
        perps.closePosition(id);

        assertEq(rusd.balanceOf(trader), before + collateral + 6_000e18, "short profits");
    }

    function testOnlyTraderCanClose() public {
        vm.prank(trader);
        bytes32 id = perps.openPosition(BTC, 1e18, 100_000e18, true);

        vm.prank(other);
        vm.expectRevert("TigerSwap: NOT_TRADER");
        perps.closePosition(id);
    }

    function testCloseRejectsUnknownPosition() public {
        vm.prank(trader);
        vm.expectRevert("TigerSwap: POSITION_NOT_EXISTS");
        perps.closePosition(keccak256("NOPE"));
    }

    function testClosingAWipedOutPositionPaysNothing() public {
        vm.prank(trader);
        bytes32 id = perps.openPosition(BTC, 1e18, 10_000e18, true);

        // Price crashes far past the collateral: the loss exceeds the margin.
        perps.updatePrice(BTC, 40_000e18);

        uint256 before = rusd.balanceOf(trader);
        vm.prank(trader);
        perps.closePosition(id);

        assertEq(rusd.balanceOf(trader), before, "no payout on a wiped-out position");
        (, , uint256 size, , , , ) = perps.getPosition(id);
        assertEq(size, 0, "position closed");
    }

    // ---- Liquidation -------------------------------------------------------

    function testCannotLiquidateHealthyPosition() public {
        vm.prank(trader);
        bytes32 id = perps.openPosition(BTC, 1e18, 100_000e18, true);

        vm.prank(keeper);
        vm.expectRevert("TigerSwap: CANNOT_LIQUIDATE");
        perps.liquidatePosition(id);
    }

    function testLiquidateUnhealthyPosition() public {
        // 1 BTC notional 60,000, collateral 10,000 -> 6x leverage.
        // A fall to 51,000 loses 9,000, leaving 1,000 equity on 51,000 notional
        // which is below the 5% maintenance margin.
        vm.prank(trader);
        bytes32 id = perps.openPosition(BTC, 1e18, 10_000e18, true);

        perps.updatePrice(BTC, 51_000e18);

        uint256 keeperBefore = rusd.balanceOf(keeper);
        vm.prank(keeper);
        perps.liquidatePosition(id);

        uint256 fee = 10_000e18 * LIQ_FEE / 1e18;
        assertEq(rusd.balanceOf(keeper), keeperBefore + fee, "liquidator paid the fee");
        (, , uint256 size, , , , ) = perps.getPosition(id);
        assertEq(size, 0, "position removed");
    }

    function testLiquidationFeeIsCappedAtEquity() public {
        vm.prank(trader);
        bytes32 id = perps.openPosition(BTC, 1e18, 10_000e18, true);

        // Loss of 9,900 leaves only 100 of equity, less than the 500 fee.
        perps.updatePrice(BTC, 50_100e18);

        uint256 keeperBefore = rusd.balanceOf(keeper);
        vm.prank(keeper);
        perps.liquidatePosition(id);

        uint256 paid = rusd.balanceOf(keeper) - keeperBefore;
        assertLe(paid, 100e18 + 1, "never pays more than the remaining equity");
        assertGt(paid, 0, "liquidator still paid");
    }

    function testLiquidatingUnknownPositionReverts() public {
        vm.prank(keeper);
        vm.expectRevert("TigerSwap: POSITION_NOT_EXISTS");
        perps.liquidatePosition(keccak256("NOPE"));
    }

    function testLiquidationPaysTraderTheResidual() public {
        vm.prank(trader);
        bytes32 id = perps.openPosition(BTC, 1e18, 10_000e18, true);

        perps.updatePrice(BTC, 51_000e18);

        uint256 traderBefore = rusd.balanceOf(trader);
        vm.prank(keeper);
        perps.liquidatePosition(id);

        // Equity 1,000, fee 500, so the trader keeps 500.
        assertEq(rusd.balanceOf(trader), traderBefore + 500e18, "residual returned to the trader");
    }

    // ---- Margin management -------------------------------------------------

    function testAddMarginIncreasesCollateral() public {
        vm.prank(trader);
        bytes32 id = perps.openPosition(BTC, 1e18, 100_000e18, true);

        vm.prank(trader);
        perps.addMargin(id, 50_000e18);

        (, , , uint256 col, , , ) = perps.getPosition(id);
        assertEq(col, 150_000e18, "collateral added");
    }

    function testAddMarginRejectsZero() public {
        vm.prank(trader);
        bytes32 id = perps.openPosition(BTC, 1e18, 100_000e18, true);

        vm.prank(trader);
        vm.expectRevert("TigerSwap: ZERO_AMOUNT");
        perps.addMargin(id, 0);
    }

    function testRemoveMarginChecksInitialMargin() public {
        vm.prank(trader);
        bytes32 id = perps.openPosition(BTC, 1e18, 100_000e18, true);

        // Removing down to below the 10% initial margin must be rejected.
        vm.prank(trader);
        vm.expectRevert("TigerSwap: MARGIN_TOO_LOW");
        perps.removeMargin(id, 95_000e18);
    }

    function testRemoveMarginSucceedsWithinLimits() public {
        vm.prank(trader);
        bytes32 id = perps.openPosition(BTC, 1e18, 100_000e18, true);

        vm.prank(trader);
        perps.removeMargin(id, 50_000e18);

        (, , , uint256 col, , , ) = perps.getPosition(id);
        assertEq(col, 50_000e18, "margin removed");
    }

    function testOnlyTraderCanRemoveMargin() public {
        vm.prank(trader);
        bytes32 id = perps.openPosition(BTC, 1e18, 100_000e18, true);

        vm.prank(other);
        vm.expectRevert("TigerSwap: NOT_TRADER");
        perps.removeMargin(id, 1_000e18);
    }

    // ---- Funding -----------------------------------------------------------

    function testFundingRateFollowsPriceDifference() public {
        perps.updateFundingRate(BTC, 61_000e18, 60_000e18);

        (int256 rate, uint256 updated, uint256 mark, uint256 index) = perps.getFundingInfo(BTC);
        assertGt(rate, 0, "positive funding when mark is above index");
        assertEq(updated, block.timestamp, "timestamp recorded");
        assertEq(mark, 61_000e18, "mark stored");
        assertEq(index, 60_000e18, "index stored");
    }

    function testNegativeFundingWhenMarkBelowIndex() public {
        perps.updateFundingRate(BTC, 59_000e18, 60_000e18);
        (int256 rate, , , ) = perps.getFundingInfo(BTC);
        assertLt(rate, 0, "negative funding");
    }

    function testOnlyOracleCanUpdateFunding() public {
        vm.prank(trader);
        vm.expectRevert("TigerSwap: NOT_ORACLE");
        perps.updateFundingRate(BTC, 61_000e18, 60_000e18);
    }

    // ---- Views and admin ---------------------------------------------------

    function testMarkPriceReadsOraclePrice() public {
        assertEq(perps.getMarkPrice(BTC), BTC_PRICE, "mark equals oracle");
        perps.updatePrice(BTC, 61_000e18);
        assertEq(perps.getMarkPrice(BTC), 61_000e18, "mark updated");
    }

    function testUpdatePriceRejectsZero() public {
        vm.expectRevert("TigerSwap: INVALID_PRICE");
        perps.updatePrice(BTC, 0);
    }

    function testSetOracleOnlyOwner() public {
        vm.prank(trader);
        vm.expectRevert("TigerSwap: NOT_OWNER");
        perps.setOracle(trader);
    }

    function testSetOracleRejectsZero() public {
        vm.expectRevert("TigerSwap: ZERO_ADDRESS");
        perps.setOracle(address(0));
    }

    function testTransferOwnership() public {
        perps.transferOwnership(other);
        assertEq(perps.owner(), other, "owner changed");

        vm.prank(other);
        perps.setOracle(other);
        assertEq(perps.priceOracle(), other, "new owner can administer");
    }

    // ---- Liquidity pool ----------------------------------------------------

    function testAddLiquidityMintsShares() public {
        address lp2 = address(0x55);
        rusd.mint(lp2, 1_000_000e18);
        vm.prank(lp2);
        rusd.approve(address(perps), type(uint256).max);
        vm.prank(lp2);
        uint256 shares = perps.addLiquidity(address(rusd), 1_000_000e18);

        // The pool already holds 5M, so 1M buys a sixth of the shares.
        assertEq(shares, POOL_LIQUIDITY / 5, "shares priced against the pool");
        assertEq(perps.poolLiquidity(address(rusd)), POOL_LIQUIDITY + 1_000_000e18, "liquidity grew");
    }

    function testRemoveLiquidityReturnsFunds() public {
        uint256 before = rusd.balanceOf(address(this));
        uint256 shares = perps.poolShares(address(rusd), address(this));
        uint256 out = perps.removeLiquidity(address(rusd), shares);

        assertEq(out, POOL_LIQUIDITY, "whole pool returned");
        assertEq(rusd.balanceOf(address(this)), before + out, "received");
    }

    function testRemoveLiquidityRejectsExcessShares() public {
        vm.expectRevert("TigerSwap: INSUFFICIENT_SHARES");
        perps.removeLiquidity(address(rusd), POOL_LIQUIDITY + 1);
    }

    function testProtocolFeesAccrueAndWithdraw() public {
        // A market with an explicit open fee.
        bytes32 sol = keccak256("SOL/RUSD");
        perps.createMarket(sol, address(rusd), 10e18, LIQ_FEE, MAINT_MARGIN, INITIAL_MARGIN);
        perps.updateMarket(sol, 10e18, LIQ_FEE, MAINT_MARGIN, INITIAL_MARGIN, 1e15, 0);
        perps.updatePrice(sol, 100e18);

        vm.prank(trader);
        perps.openPosition(sol, 1e18, 100e18, true);

        // Notional 100, fee 1e15/1e18 = 0.1% = 0.1, of which 20% is protocol.
        assertEq(perps.protocolFees(address(rusd)), 0.1e18 * 2e17 / 1e18, "protocol cut accrued");

        uint256 before = rusd.balanceOf(other);
        perps.withdrawProtocolFees(address(rusd), other);
        assertEq(rusd.balanceOf(other), before + 0.1e18 * 2e17 / 1e18, "withdrawn");
    }

    function testOnlyOwnerCanWithdrawProtocolFees() public {
        vm.prank(trader);
        vm.expectRevert("TigerSwap: NOT_OWNER");
        perps.withdrawProtocolFees(address(rusd), trader);
    }

    function testOpenRejectedWhenPoolCannotCoverPayout() public {
        // A market with a thin initial margin so the margin check passes but the
        // pool cannot cover the position's maximum payout.
        bytes32 big = keccak256("BIG/RUSD");
        perps.createMarket(big, address(rusd), 1000e18, LIQ_FEE, 1e14, 1e15);
        perps.updatePrice(big, 100e18);

        // 200,000 units at 100 is 20,000,000 notional. At a 0.1% initial margin
        // the margin check passes with 20,000 collateral, but the 30% max payout
        // of 6,000,000 exceeds the pool's 5,000,000, so coverage rejects it.
        vm.prank(trader);
        vm.expectRevert("TigerSwap: INSUFFICIENT_POOL_LIQUIDITY");
        perps.openPosition(big, 200_000e18, 20_000e18, true);
    }

    function testPoolCapsASingleWinnersPayout() public {
        // A large long that would win more than the cap allows.
        vm.prank(trader);
        bytes32 id = perps.openPosition(BTC, 10e18, 600_000e18, true);

        // Price doubles: the raw profit would be 600,000.
        perps.updatePrice(BTC, 120_000e18);

        uint256 poolBefore = perps.poolLiquidity(address(rusd));
        uint256 traderBefore = rusd.balanceOf(trader);

        vm.prank(trader);
        perps.closePosition(id);

        uint256 paid = rusd.balanceOf(trader) - traderBefore;
        uint256 poolAfter = perps.poolLiquidity(address(rusd));

        // The payout is collateral plus a profit bounded by the pool.
        assertGe(paid, 600_000e18, "collateral returned in full");
        assertLe(poolBefore - poolAfter, poolBefore, "pool never pays more than it holds");
        assertLe(poolAfter, poolBefore, "pool liquidity only falls");
    }

    function testLpCannotWithdrawReservedFees() public {
        bytes32 sol = keccak256("SOL/RUSD");
        perps.createMarket(sol, address(rusd), 10e18, LIQ_FEE, MAINT_MARGIN, INITIAL_MARGIN);
        perps.updateMarket(sol, 10e18, LIQ_FEE, MAINT_MARGIN, INITIAL_MARGIN, 1e15, 0);
        perps.updatePrice(sol, 100e18);

        vm.prank(trader);
        perps.openPosition(sol, 1e18, 100e18, true);

        // The protocol's cut is reserved, so withdrawing the whole pool fails.
        uint256 shares = perps.poolShares(address(rusd), address(this));
        vm.expectRevert("TigerSwap: LIQUIDITY_RESERVED");
        perps.removeLiquidity(address(rusd), shares);
    }

    function testOpenInterestIsTrackedAndReleased() public {
        vm.prank(trader);
        bytes32 id = perps.openPosition(BTC, 1e18, 100_000e18, true);
        assertEq(perps.getMarket(BTC).openInterest, 60_000e18, "open interest recorded");

        vm.prank(trader);
        perps.closePosition(id);
        assertEq(perps.getMarket(BTC).openInterest, 0, "open interest released");
    }

    function testOpenInterestCapIsEnforced() public {
        bytes32 sol = keccak256("SOL/RUSD");
        perps.createMarket(sol, address(rusd), 10e18, LIQ_FEE, MAINT_MARGIN, INITIAL_MARGIN);
        perps.updateMarket(sol, 10e18, LIQ_FEE, MAINT_MARGIN, INITIAL_MARGIN, 0, 50_000e18);
        perps.updatePrice(sol, 100e18);

        vm.prank(trader);
        perps.openPosition(sol, 400e18, 40_000e18, true); // 40,000 notional

        vm.prank(trader);
        vm.expectRevert("TigerSwap: OPEN_INTEREST_CAP");
        perps.openPosition(sol, 200e18, 20_000e18, true); // would reach 60,000
    }

    function testFreeLiquidityReflectsPool() public view {
        assertEq(perps.freeLiquidity(address(rusd)), POOL_LIQUIDITY, "full pool free");
    }

    function testClosingOnePositionLeavesTheOtherBacked() public {
        vm.startPrank(trader);
        bytes32 a = perps.openPosition(BTC, 1e18, 100_000e18, true);
        perps.openPosition(ETH, 1e18, 100_000e18, true);
        vm.stopPrank();

        // Both collaterals are held by the engine.
        (, , , uint256 colA, , , ) = perps.getPosition(a);
        assertEq(rusd.balanceOf(address(perps)), POOL_LIQUIDITY + 2 * colA, "both held");

        vm.prank(trader);
        perps.closePosition(a);

        // Closing at the entry price returns exactly one collateral.
        assertEq(rusd.balanceOf(address(perps)), POOL_LIQUIDITY + colA, "only the open one remains");
    }
}
