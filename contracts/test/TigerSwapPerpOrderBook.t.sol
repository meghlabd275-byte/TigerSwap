// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "forge-std/Test.sol";
import "../TigerSwapPerpOrderBook.sol";
import "./TestERC20.sol";

/// @dev A price source that always answers, so the order book can read a mark
///      without the real median oracle's reporter setup.
contract FixedOracle {
    mapping(address => uint256) public prices;

    function setPrice(address asset, uint256 price) external {
        prices[asset] = price;
    }

    function getPrice(address asset) external view returns (uint256) {
        uint256 price = prices[asset];
        require(price > 0, "FixedOracle: NO_PRICE");
        return price;
    }
}

contract TigerSwapPerpOrderBookTest is Test {
    // Mirrors TigerSwapPerpOrderBook.OrderFilled so the test can assert the
    // maker fill names the resting order it consumed.
    event OrderFilled(
        uint256 indexed orderId,
        bytes32 indexed marketId,
        address indexed trader,
        uint256 price,
        uint256 quantity,
        bool isMaker,
        uint256 fee
    );

    TigerSwapPerpOrderBook public book;
    FixedOracle public oracle;
    TestERC20 public usdc;

    address public alice = address(0xA11CE);
    address public bob = address(0xB0B);
    address public carol = address(0xCA401);
    address public liquidator = address(0x11D);

    bytes32 constant BTC = keccak256("BTC/RUSD");
    uint256 constant PRICE = 60_000e18;
    uint256 constant LOT = 1e15; // 0.001 base unit
    uint256 constant MIN_ORDER = 1e15;

    // 10x max leverage => 10% initial margin, 5% maintenance.
    uint256 constant INITIAL_MARGIN = 1e17;
    uint256 constant MAINT_MARGIN = 5e16;
    uint256 constant TAKER_FEE = 5e14; // 0.05%
    uint256 constant MAKER_FEE = 2e14; // 0.02%
    uint256 constant LIQ_FEE = 5e16; // 5% of remaining equity

    function setUp() public {
        usdc = new TestERC20("USD Coin", "USDC");
        oracle = new FixedOracle();
        book = new TigerSwapPerpOrderBook(address(this));

        oracle.setPrice(address(0xB7C), PRICE);

        book.createMarket(
            BTC,
            address(usdc),
            address(0xB7C),
            TAKER_FEE,
            MAKER_FEE,
            INITIAL_MARGIN,
            MAINT_MARGIN,
            LIQ_FEE,
            10e18,
            0,
            MIN_ORDER,
            LOT
        );
        book.setMarketActive(BTC, true);
        book.updatePrice(BTC, PRICE, PRICE);

        address[3] memory traders = [alice, bob, carol];
        for (uint256 i = 0; i < 3; i++) {
            usdc.mint(traders[i], 100_000_000e18);
            vm.prank(traders[i]);
            usdc.approve(address(book), type(uint256).max);
        }
        // Seed the pool that pays winners.
        usdc.mint(address(this), 100_000_000e18);
        usdc.approve(address(book), type(uint256).max);
        book.addLiquidity(BTC, 50_000_000e18);
    }

    function _deposit(address who, uint256 amount) internal {
        vm.prank(who);
        book.deposit(BTC, amount);
    }

    function _place(address who, uint256 qty, uint256 price, bool isLong, uint8 tif, bool ro)
        internal
        returns (uint256 id, uint256 filled)
    {
        vm.prank(who);
        return book.placeOrder(BTC, qty, price, isLong, tif, ro);
    }

    /// @dev Open a two-sided position: `who` trades against a fresh counterparty
    ///      so the resulting position is exactly `qty` on `isLong`.
    function _openPosition(address who, uint256 qty, bool isLong) internal {
        address counter = who == bob ? carol : bob;
        _deposit(who, 100_000e18);
        _deposit(counter, 100_000e18);
        if (isLong) {
            _place(counter, qty, PRICE, false, 0, false); // counter sells
            _place(who, qty, PRICE, true, 1, false); // who buys, IOC
        } else {
            _place(counter, qty, PRICE, true, 0, false); // counter buys
            _place(who, qty, PRICE, false, 1, false); // who sells, IOC
        }
    }

    /// @dev Open a position with a specific (small) margin, for liquidation tests.
    function _openLeveraged(address who, uint256 qty, bool isLong, uint256 margin) internal {
        address counter = who == bob ? carol : bob;
        _deposit(who, margin);
        _deposit(counter, 100_000e18);
        if (isLong) {
            _place(counter, qty, PRICE, false, 0, false);
            _place(who, qty, PRICE, true, 1, false);
        } else {
            _place(counter, qty, PRICE, true, 0, false);
            _place(who, qty, PRICE, false, 1, false);
        }
    }

    // ---- Admin ----

    function testCreateMarketStoresConfig() public view {
        TigerSwapPerpOrderBook.Market memory m = book.getMarket(BTC);
        assertEq(m.token, address(usdc), "token");
        assertEq(m.initialMargin, INITIAL_MARGIN, "initial margin");
        assertEq(m.maintenanceMargin, MAINT_MARGIN, "maintenance margin");
        assertTrue(m.isActive, "active");
    }

    function testOnlyOwnerCanCreateMarket() public {
        vm.prank(alice);
        vm.expectRevert("TigerSwap: NOT_OWNER");
        book.createMarket(keccak256("X"), address(usdc), address(0xB7C), 0, 0, 1e17, 5e16, 0, 10e18, 0, 1e15, 1e15);
    }

    function testCreateMarketRejectsDuplicate() public {
        vm.expectRevert("TigerSwap: MARKET_EXISTS");
        book.createMarket(BTC, address(usdc), address(0xB7C), 0, 0, 1e17, 5e16, 0, 10e18, 0, 1e15, 1e15);
    }

    function testCreateMarketRejectsBadMargin() public {
        vm.expectRevert("TigerSwap: BAD_MARGIN");
        book.createMarket(keccak256("Y"), address(usdc), address(0xB7C), 0, 0, 5e16, 1e17, 0, 10e18, 0, 1e15, 1e15);
    }

    function testOnlyOracleCanUpdatePrice() public {
        vm.prank(alice);
        vm.expectRevert("TigerSwap: NOT_ORACLE");
        book.updatePrice(BTC, PRICE, PRICE);
    }

    function testTradingHaltBlocksOrders() public {
        _deposit(alice, 10_000e18);
        book.setTradingEnabled(false);
        vm.prank(alice);
        vm.expectRevert("TigerSwap: TRADING_HALTED");
        book.placeOrder(BTC, 1e18, PRICE, true, 0, false);
    }

    function testTwoStepOwnership() public {
        book.commitTransferOwnership(alice);
        vm.prank(bob);
        vm.expectRevert("TigerSwap: NOT_PENDING_OWNER");
        book.acceptTransferOwnership();
        vm.prank(alice);
        book.acceptTransferOwnership();
        assertEq(book.owner(), alice, "owner transferred");
    }

    // ---- Deposit / withdraw ----

    function testDepositAndWithdraw() public {
        _deposit(alice, 1_000e18);
        assertEq(book.balances(BTC, alice), 1_000e18, "balance");
        vm.prank(alice);
        book.withdraw(BTC, 400e18);
        assertEq(book.balances(BTC, alice), 600e18, "balance after withdraw");
        assertEq(usdc.balanceOf(alice), 100_000_000e18 - 600e18, "token returned");
    }

    function testWithdrawBeyondBalanceReverts() public {
        _deposit(alice, 100e18);
        vm.prank(alice);
        vm.expectRevert("TigerSwap: INSUFFICIENT_BALANCE");
        book.withdraw(BTC, 101e18);
    }

    function testWithdrawCannotDropBelowInitialMargin() public {
        _openPosition(alice, 1e18, true); // long 1 BTC at 60k, initial margin 6k
        vm.prank(alice);
        vm.expectRevert("TigerSwap: MARGIN_TOO_LOW");
        book.withdraw(BTC, 95_000e18);
    }

    // ---- Resting + matching ----

    function testRestingOrderIsPlacedAndReadable() public {
        _deposit(alice, 10_000e18);
        (uint256 id, uint256 filled) = _place(alice, 1e18, PRICE, true, 0, false);
        assertGt(id, 0, "order id");
        assertEq(filled, 0, "nothing filled");
        (uint256 bidPrice, uint256 bidQty, , ) = book.bestBidAsk(BTC);
        assertEq(bidPrice, PRICE, "best bid");
        assertEq(bidQty, 1e18, "best bid qty");
    }

    function testTakerCrossesRestingOrder() public {
        _deposit(alice, 10_000e18);
        _deposit(bob, 10_000e18);
        _place(alice, 1e18, PRICE, false, 0, false); // alice sells (short)
        (, uint256 filled) = _place(bob, 1e18, PRICE, true, 0, false); // bob buys
        assertEq(filled, 1e18, "bob fully filled");
        (int256 aliceSize, , , , ) = book.getPosition(BTC, alice);
        (int256 bobSize, , , , ) = book.getPosition(BTC, bob);
        assertEq(aliceSize, -1e18, "alice short");
        assertEq(bobSize, 1e18, "bob long");
    }

    /// @dev A maker fill must name the resting order it consumed, so an indexer
    ///      can attribute the fill to that order. The taker side carries id 0.
    function testMakerFillCarriesRestingOrderId() public {
        _deposit(alice, 10_000e18);
        _deposit(bob, 10_000e18);
        (uint256 aliceId, ) = _place(alice, 1e18, PRICE, false, 0, false);

        // Maker fee is 0.02% of the 60,000 notional = 12e18.
        vm.expectEmit(true, true, true, true, address(book));
        emit OrderFilled(aliceId, BTC, alice, PRICE, 1e18, true, 12e18);
        _place(bob, 1e18, PRICE, true, 0, false);
    }

    function testPriceTimePriority() public {
        _deposit(alice, 10_000e18);
        _deposit(bob, 10_000e18);
        _deposit(carol, 10_000e18);
        (uint256 aId, ) = _place(alice, 1e18, PRICE, false, 0, false);
        (uint256 bId, ) = _place(bob, 1e18, PRICE, false, 0, false);
        assertLt(aId, bId, "alice earlier");
        _place(carol, 1e18, PRICE, true, 0, false);
        (int256 aliceSize, , , , ) = book.getPosition(BTC, alice);
        (int256 bobSize, , , , ) = book.getPosition(BTC, bob);
        assertEq(aliceSize, -1e18, "alice filled first");
        assertEq(bobSize, 0, "bob untouched");
    }

    function testPartialFillRestsRemainder() public {
        _deposit(alice, 30_000e18);
        _deposit(bob, 30_000e18);
        _place(alice, 2e18, PRICE, false, 0, false);
        (, uint256 filled) = _place(bob, 1e18, PRICE, true, 0, false);
        assertEq(filled, 1e18, "filled one");
        (, , uint256 askPrice, uint256 askQty) = book.bestBidAsk(BTC);
        assertEq(askPrice, PRICE, "ask price");
        assertEq(askQty, 1e18, "ask remainder");
    }

    function testTakerPartialThenRests() public {
        _deposit(alice, 30_000e18);
        _deposit(bob, 30_000e18);
        _place(alice, 1e18, PRICE, false, 0, false);
        // Bob bids for 2 at the ask; 1 fills, 1 rests.
        (uint256 id, uint256 filled) = _place(bob, 2e18, PRICE, true, 0, false);
        assertEq(filled, 1e18, "one filled");
        assertGt(id, 0, "remainder rests");
        (uint256 bidPrice, uint256 bidQty, , ) = book.bestBidAsk(BTC);
        assertEq(bidPrice, PRICE, "resting bid price");
        assertEq(bidQty, 1e18, "resting bid qty");
    }

    function testIocCancelsRemainder() public {
        _deposit(alice, 10_000e18);
        _deposit(bob, 10_000e18);
        _place(alice, 1e18, PRICE, false, 0, false);
        (uint256 id, uint256 filled) = _place(bob, 2e18, PRICE, true, 1, false); // IOC
        assertEq(filled, 1e18, "one filled");
        assertEq(id, 0, "no remainder rests");
        (uint256 bidPrice, , , ) = book.bestBidAsk(BTC);
        assertEq(bidPrice, 0, "no resting bid");
    }

    function testFokAllOrNothing() public {
        _deposit(alice, 10_000e18);
        _deposit(bob, 10_000e18);
        _place(alice, 1e18, PRICE, false, 0, false);
        vm.prank(bob);
        vm.expectRevert("TigerSwap: FOK_NOT_FILLED");
        book.placeOrder(BTC, 2e18, PRICE, true, 2, false);
    }

    function testFokFillsWhenFullLiquidityExists() public {
        _deposit(alice, 30_000e18);
        _deposit(bob, 30_000e18);
        _place(alice, 2e18, PRICE, false, 0, false);
        (, uint256 filled) = _place(bob, 2e18, PRICE, true, 2, false);
        assertEq(filled, 2e18, "fok filled");
    }

    function testPostOnlyRejectsCrossing() public {
        _deposit(alice, 10_000e18);
        _deposit(bob, 10_000e18);
        _place(alice, 1e18, PRICE, false, 0, false);
        vm.prank(bob);
        vm.expectRevert("TigerSwap: POST_ONLY_WOULD_CROSS");
        book.placeOrder(BTC, 1e18, PRICE, true, 3, false);
    }

    function testPostOnlyRestsWhenNotCrossing() public {
        _deposit(bob, 10_000e18);
        (uint256 id, uint256 filled) = _place(bob, 1e18, PRICE - 100e18, true, 3, false);
        assertGt(id, 0, "rests");
        assertEq(filled, 0, "no fill");
    }

    function testCancelOrder() public {
        _deposit(alice, 10_000e18);
        (uint256 id, ) = _place(alice, 1e18, PRICE, true, 0, false);
        vm.prank(alice);
        book.cancelOrder(id);
        (uint256 bidPrice, , , ) = book.bestBidAsk(BTC);
        assertEq(bidPrice, 0, "book empty");
    }

    function testCannotCancelOthersOrder() public {
        _deposit(alice, 10_000e18);
        (uint256 id, ) = _place(alice, 1e18, PRICE, true, 0, false);
        vm.prank(bob);
        vm.expectRevert("TigerSwap: NOT_OWNER");
        book.cancelOrder(id);
    }

    function testSelfTradePrevented() public {
        _deposit(alice, 10_000e18);
        _place(alice, 1e18, PRICE, false, 0, false);
        (uint256 id, uint256 filled) = _place(alice, 1e18, PRICE, true, 0, false);
        assertEq(filled, 0, "no self fill");
        (int256 aliceSize, , , , ) = book.getPosition(BTC, alice);
        assertEq(aliceSize, 0, "no position");
        assertGt(id, 0, "rests instead");
    }

    function testBookStaysUncrossed() public {
        _deposit(alice, 10_000e18);
        _deposit(bob, 10_000e18);
        _place(alice, 1e18, PRICE, false, 0, false);
        _place(bob, 1e18, PRICE - 100e18, true, 0, false);
        (uint256 bidPrice, , uint256 askPrice, ) = book.bestBidAsk(BTC);
        assertLt(bidPrice, askPrice, "best bid below best ask");
    }

    // ---- Margin enforcement ----

    function testOrderRejectedWithoutMargin() public {
        _deposit(alice, 100e18); // far too little for 1 BTC at 10x
        vm.prank(alice);
        vm.expectRevert("TigerSwap: INSUFFICIENT_MARGIN");
        book.placeOrder(BTC, 1e18, PRICE, true, 0, false);
    }

    function testMaxLeverageRespected() public {
        _openPosition(alice, 1e18, true);
        (int256 size, , , , ) = book.getPosition(BTC, alice);
        assertEq(size, 1e18, "opened at max leverage");
    }

    function testRestingOrderNeedsMargin() public {
        _deposit(alice, 100e18);
        vm.prank(alice);
        vm.expectRevert("TigerSwap: INSUFFICIENT_MARGIN");
        book.placeOrder(BTC, 1e18, PRICE - 1_000e18, true, 0, false);
    }

    function testReduceOnlyCannotFlip() public {
        _openPosition(alice, 1e18, true); // alice long 1
        _deposit(bob, 10_000e18);
        _place(bob, 1e18, PRICE, true, 0, false); // bob bids 1
        _place(alice, 2e18, PRICE, false, 0, true); // alice reduce-only sells 2
        (int256 size, , , , ) = book.getPosition(BTC, alice);
        assertEq(size, 0, "reduce-only never flips");
    }

    function testReduceOnlyWithNoPositionDoesNotOpen() public {
        _deposit(alice, 10_000e18);
        _deposit(bob, 10_000e18);
        _place(bob, 1e18, PRICE, false, 0, false);
        (uint256 id, uint256 filled) = _place(alice, 1e18, PRICE, true, 1, true); // IOC reduce-only
        assertEq(filled, 0, "nothing filled");
        (int256 size, , , , ) = book.getPosition(BTC, alice);
        assertEq(size, 0, "still flat");
        assertEq(id, 0, "ioc cancelled");
    }

    // ---- PnL ----

    function testProfitPaidFromPool() public {
        _openPosition(alice, 1e18, true); // alice long 1 at 60k
        _deposit(bob, 10_000e18);
        book.updatePrice(BTC, 61_000e18, 61_000e18);
        _place(bob, 1e18, 61_000e18, true, 0, false); // bob bid
        uint256 aliceBefore = book.balances(BTC, alice);
        _place(alice, 1e18, 61_000e18, false, 0, true); // alice closes at 61k
        (int256 size, , , , ) = book.getPosition(BTC, alice);
        assertEq(size, 0, "flat");
        uint256 fee = 61_000e18 * TAKER_FEE / 1e18;
        assertEq(book.balances(BTC, alice), aliceBefore + 1_000e18 - fee, "profit credited");
    }

    function testLossMovesToPool() public {
        _openPosition(alice, 1e18, true);
        _deposit(bob, 10_000e18);
        uint256 poolBefore = book.poolLiquidity(BTC);
        book.updatePrice(BTC, 59_000e18, 59_000e18);
        _place(bob, 1e18, 59_000e18, true, 0, false);
        _place(alice, 1e18, 59_000e18, false, 0, true); // alice closes at -1k
        assertGt(book.poolLiquidity(BTC), poolBefore, "pool gained the loss + fees");
    }

    function testProfitBeyondPoolIsRecordedUnpaid() public {
        book.removeLiquidity(BTC, book.poolLiquidity(BTC));
        _openPosition(alice, 1e18, true);
        _deposit(bob, 10_000e18);
        book.updatePrice(BTC, 61_000e18, 61_000e18);
        _place(bob, 1e18, 61_000e18, true, 0, false);
        _place(alice, 1e18, 61_000e18, false, 0, true);
        assertGt(book.unpaidProfits(BTC), 0, "unpaid profit recorded");
    }

    function testDeferredProfitIsPaidWhenLiquidityReturns() public {
        book.removeLiquidity(BTC, book.poolLiquidity(BTC));
        _openPosition(alice, 1e18, true);
        _deposit(bob, 10_000e18);
        book.updatePrice(BTC, 61_000e18, 61_000e18);
        _place(bob, 1e18, 61_000e18, true, 0, false);
        _place(alice, 1e18, 61_000e18, false, 0, true); // alice closes at +1k

        uint256 owed = book.unpaidProfits(BTC);
        assertGt(owed, 0, "profit deferred, not lost");

        // An LP funds the pool, then alice claims what she was owed.
        book.addLiquidity(BTC, owed * 3);
        uint256 before = book.balances(BTC, alice);
        vm.prank(alice);
        uint256 paid = book.claimUnpaid(BTC);
        assertGt(paid, 0, "deferred profit paid");
        assertEq(book.balances(BTC, alice), before + paid, "balance credited");
        assertLt(book.unpaidProfits(BTC), owed, "obligation reduced");
    }

    function testClaimUnpaidRejectsNothingOwed() public {
        _deposit(alice, 1_000e18);
        vm.prank(alice);
        vm.expectRevert(bytes("TigerSwap: NOTHING_OWED"));
        book.claimUnpaid(BTC);
    }

    function testClaimUnpaidPaysFromLaterFees() public {
        book.removeLiquidity(BTC, book.poolLiquidity(BTC));
        _openPosition(alice, 1e18, true);
        _deposit(bob, 10_000e18);
        book.updatePrice(BTC, 61_000e18, 61_000e18);
        _place(bob, 1e18, 61_000e18, true, 0, false);
        _place(alice, 1e18, 61_000e18, false, 0, true);
        uint256 owed = book.unpaidProfits(BTC);
        assertGt(owed, 0, "profit deferred");

        // Another trader's round trip pays fees into the pool, which the
        // creditor can then draw on.
        _deposit(carol, 10_000e18);
        _place(carol, 1e18, 61_000e18, true, 1, false);
        _place(carol, 1e18, 61_000e18, false, 1, false);

        vm.prank(alice);
        uint256 paid = book.claimUnpaid(BTC);
        assertGt(paid, 0, "paid from later fees");
    }

    // ---- Funding ----

    function testFundingChargesLongs() public {
        _openPosition(alice, 1e18, true); // long
        uint256 aliceBefore = book.balances(BTC, alice);
        vm.warp(block.timestamp + 1 hours);
        book.accrueFunding(BTC, 61_000e18, 60_000e18);
        _deposit(alice, 1e18); // touching settles funding
        assertLt(book.balances(BTC, alice), aliceBefore + 1e18, "long paid funding");
    }

    function testFundingPaysShorts() public {
        _openPosition(alice, 1e18, false); // short
        uint256 aliceBefore = book.balances(BTC, alice);
        vm.warp(block.timestamp + 1 hours);
        book.accrueFunding(BTC, 61_000e18, 60_000e18);
        _deposit(alice, 1e18);
        assertGt(book.balances(BTC, alice), aliceBefore + 1e18, "short received funding");
    }

    /// A price update carries the same mark/index spread as an accrual, so it
    /// must fold the elapsed window into the funding index rather than advance
    /// the clock and forgive it. Otherwise a keeper that calls updatePrice
    /// between accruals silently drops that window of funding.
    function testUpdatePriceAccruesElapsedFunding() public {
        _openPosition(alice, 1e18, true); // long
        vm.warp(block.timestamp + 1 hours);
        int256 before = book.getFundingInfo(BTC).cumulativeIndex;
        book.updatePrice(BTC, 61_000e18, 60_000e18);
        int256 afterIndex = book.getFundingInfo(BTC).cumulativeIndex;
        assertGt(afterIndex, before, "updatePrice folded the window into funding");
    }

    // ---- Liquidation ----

    function testLiquidationOfUnderwaterPosition() public {
        _openLeveraged(alice, 1e18, true, 6_100e18);
        book.updatePrice(BTC, 56_500e18, 56_500e18);
        (, , , , int256 equity) = book.getPosition(BTC, alice);
        assertLt(equity, int256(56_500e18 * MAINT_MARGIN / 1e18), "underwater");

        vm.prank(liquidator);
        book.liquidate(BTC, alice);
        (int256 size, , , , ) = book.getPosition(BTC, alice);
        assertEq(size, 0, "position closed");
        assertEq(book.balances(BTC, alice), 0, "margin cleared");
    }

    function testCannotLiquidateHealthyPosition() public {
        _openPosition(alice, 1e18, true);
        vm.prank(liquidator);
        vm.expectRevert("TigerSwap: CANNOT_LIQUIDATE");
        book.liquidate(BTC, alice);
    }

    function testLiquidationPaysLiquidator() public {
        _openLeveraged(alice, 1e18, true, 6_100e18);
        book.updatePrice(BTC, 56_500e18, 56_500e18);
        uint256 before = usdc.balanceOf(liquidator);
        vm.prank(liquidator);
        book.liquidate(BTC, alice);
        assertGt(usdc.balanceOf(liquidator), before, "liquidator paid a fee");
    }

    function testBadDebtAbsorbedByInsuranceFirst() public {
        book.depositInsurance(BTC, 100_000e18);
        _openLeveraged(alice, 1e18, true, 6_100e18);
        book.updatePrice(BTC, 50_000e18, 50_000e18); // equity negative
        uint256 insuranceBefore = book.insuranceFund(BTC);
        vm.prank(liquidator);
        book.liquidate(BTC, alice);
        assertLt(book.insuranceFund(BTC), insuranceBefore, "insurance absorbed the debt");
    }

    function testLiquidationReadsOracleNotCaller() public {
        _openPosition(alice, 1e18, true);
        vm.prank(liquidator);
        vm.expectRevert("TigerSwap: CANNOT_LIQUIDATE");
        book.liquidate(BTC, alice);
    }

    /// A liquidated position's unrealized loss must reach the pool, exactly as a
    /// normal close does; otherwise the pool bleeds on every liquidation while
    /// the trader keeps margin that is owed to it.
    function testLiquidationSettlesLossToPool() public {
        _openLeveraged(alice, 1e18, true, 6_100e18);
        book.updatePrice(BTC, 56_500e18, 56_500e18);
        uint256 poolBefore = book.poolLiquidity(BTC);
        vm.prank(liquidator);
        book.liquidate(BTC, alice);
        // Loss is (60_000 - 56_500) * 1e18 / 1e18 = 3_500e18.
        assertEq(book.poolLiquidity(BTC) - poolBefore, 3_500e18, "pool collected exactly the loss");
    }

    /// A liquidated trader with margin left over keeps the remainder after the
    /// liquidator fee, so the margin is not stranded in the contract.
    function testLiquidationReturnsRemainderToTrader() public {
        _openLeveraged(alice, 1e18, true, 6_100e18);
        book.updatePrice(BTC, 56_500e18, 56_500e18);
        uint256 traderBefore = usdc.balanceOf(alice);
        vm.prank(liquidator);
        book.liquidate(BTC, alice);
        assertGt(usdc.balanceOf(alice), traderBefore, "remainder returned to trader");
        assertEq(book.balances(BTC, alice), 0, "margin cleared");
    }

    // ---- Liquidity ----

    function testAddRemoveLiquidity() public {
        uint256 before = book.poolLiquidity(BTC);
        book.addLiquidity(BTC, 1_000e18);
        assertEq(book.poolLiquidity(BTC), before + 1_000e18, "added");
        book.removeLiquidity(BTC, 500e18);
        assertEq(book.poolLiquidity(BTC), before + 500e18, "removed");
    }

    function testProtocolFeesAccrueAndWithdraw() public {
        _deposit(alice, 10_000e18);
        _deposit(bob, 10_000e18);
        _place(alice, 1e18, PRICE, false, 0, false);
        _place(bob, 1e18, PRICE, true, 0, false);
        assertGt(book.protocolFees(BTC), 0, "protocol fee accrued");
        uint256 amount = book.withdrawProtocolFees(BTC, address(this));
        assertGt(amount, 0, "withdrawn");
        assertEq(book.protocolFees(BTC), 0, "cleared");
    }

    function testLotSizeEnforced() public {
        _deposit(alice, 10_000e18);
        vm.prank(alice);
        vm.expectRevert("TigerSwap: BAD_LOT");
        book.placeOrder(BTC, LOT + 1, PRICE, true, 0, false);
    }

    function testDepthAggregates() public {
        _deposit(alice, 30_000e18);
        _deposit(bob, 30_000e18);
        _place(alice, 1e18, PRICE, false, 0, false);
        _place(alice, 2e18, PRICE + 100e18, false, 0, false);
        _place(bob, 1e18, PRICE - 100e18, true, 0, false);
        (uint256[] memory bp, uint256[] memory bq, uint256[] memory ap, uint256[] memory aq) = book.depth(BTC, 5);
        assertEq(bp[0], PRICE - 100e18, "best bid");
        assertEq(bq[0], 1e18, "best bid qty");
        assertEq(ap[0], PRICE, "best ask");
        assertEq(aq[0], 1e18, "best ask qty");
        assertEq(ap[1], PRICE + 100e18, "second ask");
        assertEq(aq[1], 2e18, "second ask qty");
    }
}
