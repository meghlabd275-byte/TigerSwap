// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "forge-std/Test.sol";
import "../TigerSwapTWAMM.sol";
import "./TestERC20.sol";

contract TigerSwapTWAMMTest is Test {
    TigerSwapTWAMM public twamm;
    TestERC20 public token0;
    TestERC20 public token1;

    address public lp = address(0x11);
    address public seller = address(0x22);
    address public buyer = address(0x33);
    address public feeRecipient = address(0x44);

    uint256 constant FEE = 3000; // 0.3% of 1e6

    function setUp() public {
        token0 = new TestERC20("RUSD", "RUSD");
        token1 = new TestERC20("ETH", "ETH");
        twamm = new TigerSwapTWAMM(address(token0), address(token1), feeRecipient, FEE);

        _fund(lp);
        _fund(seller);
        _fund(buyer);

        // Seed a balanced pool at 1:1.
        vm.prank(lp);
        twamm.addLiquidity(1_000_000e18, 1_000_000e18, 0);

        // Advance past the constructor's timestamp so orders can start.
        vm.warp(block.timestamp + 1 hours);
    }

    /// @dev Mints the test balance and approves the pool on behalf of `who`.
    function _fund(address who) internal {
        token0.mint(who, 100_000_000e18);
        token1.mint(who, 100_000_000e18);
        vm.startPrank(who);
        token0.approve(address(twamm), type(uint256).max);
        token1.approve(address(twamm), type(uint256).max);
        vm.stopPrank();
    }

    /// @dev The first three fields of an order.
    function _orderHead(uint256 orderId) internal view returns (address, bool, uint256) {
        (address owner, bool isToken0, uint256 sellRate, , , , ) = twamm.orders(orderId);
        return (owner, isToken0, sellRate);
    }

    /// @dev The last four fields of an order.
    function _orderBody(uint256 orderId) internal view returns (uint256, uint256, uint256, bool) {
        (, , , uint256 expiration, uint256 escrow, uint256 proceeds, bool active) = twamm.orders(orderId);
        return (expiration, escrow, proceeds, active);
    }

    // ---- Construction ------------------------------------------------------

    function testConstructorRejectsZeroToken() public {
        vm.expectRevert("TigerSwap: ZERO_TOKEN");
        new TigerSwapTWAMM(address(0), address(token1), feeRecipient, FEE);
    }

    function testConstructorRejectsSameToken() public {
        vm.expectRevert("TigerSwap: SAME_TOKEN");
        new TigerSwapTWAMM(address(token0), address(token0), feeRecipient, FEE);
    }

    function testConstructorRejectsExcessiveFee() public {
        vm.expectRevert("TigerSwap: FEE_TOO_HIGH");
        new TigerSwapTWAMM(address(token0), address(token1), feeRecipient, FEE_DENOMINATOR());
    }

    function FEE_DENOMINATOR() internal pure returns (uint256) {
        return 1e6;
    }

    // ---- Square root -------------------------------------------------------

    function testSqrtMatchesKnownValues() public view {
        assertEq(twamm.sqrt(0), 0);
        assertEq(twamm.sqrt(1), 1);
        assertEq(twamm.sqrt(4), 2);
        assertEq(twamm.sqrt(9), 3);
        assertEq(twamm.sqrt(16), 4);
        assertEq(twamm.sqrt(1e18), 1e9);
        assertEq(twamm.sqrt(1e36), 1e18);
    }

    function testSqrtIsTheFloorForNonPerfectSquares() public view {
        assertEq(twamm.sqrt(2), 1);
        assertEq(twamm.sqrt(3), 1);
        assertEq(twamm.sqrt(8), 2);
        assertEq(twamm.sqrt(15), 3);
        assertEq(twamm.sqrt(1e18 - 1), 999999999);
    }

    function testSqrtOfLargeValues() public view {
        uint256 value = 123456789012345678901234567890;
        uint256 root = twamm.sqrt(value);
        assertLe(root * root, value);
        assertGt((root + 1) * (root + 1), value);
    }

    // ---- Liquidity ---------------------------------------------------------

    function testFirstDepositMintsGeometricMeanShares() public {
        TigerSwapTWAMM fresh = new TigerSwapTWAMM(address(token0), address(token1), feeRecipient, 0);
        vm.startPrank(lp);
        token0.approve(address(fresh), type(uint256).max);
        token1.approve(address(fresh), type(uint256).max);
        uint256 shares = fresh.addLiquidity(100e18, 400e18, 0);
        vm.stopPrank();

        // sqrt(100 * 400) = 200.
        assertEq(shares, 200e18);
        assertEq(fresh.totalSupply(), 200e18);
    }

    function testAddLiquidityRejectsZeroAmount() public {
        vm.prank(lp);
        vm.expectRevert("TigerSwap: ZERO_AMOUNT");
        twamm.addLiquidity(0, 1e18, 0);
    }

    function testAddLiquidityRejectsSlippage() public {
        vm.prank(lp);
        vm.expectRevert("TigerSwap: SLIPPAGE");
        twamm.addLiquidity(1e18, 1e18, type(uint256).max);
    }

    function testRemoveLiquidityReturnsProportionalShare() public {
        uint256 before = token0.balanceOf(lp);
        uint256 shares = twamm.balanceOf(lp) / 10;

        vm.prank(lp);
        (uint256 amount0, uint256 amount1) = twamm.removeLiquidity(shares, 0, 0);

        // A tenth of the pool, which is 1,000,000 seeded.
        assertApproxEqRel(amount0, 100_000e18, 1e15);
        assertApproxEqRel(amount1, 100_000e18, 1e15);
        assertEq(token0.balanceOf(lp) - before, amount0);
    }

    function testRemoveLiquidityRejectsInsufficientShares() public {
        // Read the supply before arming the revert expectation: an argument
        // expression that makes a call would itself become the expected call.
        uint256 tooMany = twamm.totalSupply() + 1;
        vm.prank(lp);
        vm.expectRevert("TigerSwap: BALANCE");
        twamm.removeLiquidity(tooMany, 0, 0);
    }

    // ---- Order creation ----------------------------------------------------

    function testCreateOrderEscrowsTheFullSellAmount() public {
        uint256 rate = 1e18; // 1 token per second
        uint256 duration = 1 hours;
        uint256 before = token0.balanceOf(seller);

        vm.prank(seller);
        uint256 orderId = twamm.createOrder(true, rate, block.timestamp + duration);

        assertEq(orderId, 1);
        assertEq(before - token0.balanceOf(seller), rate * duration);

        // Read the fields in two groups; binding all seven at once pushes the
        // test past the stack limit under the optimizer.
        (address owner, bool isToken0, uint256 sellRate) = _orderHead(orderId);
        assertEq(owner, seller);
        assertTrue(isToken0);
        assertEq(sellRate, rate);

        (uint256 expiration, uint256 escrow, uint256 proceeds, bool active) = _orderBody(orderId);
        assertEq(expiration, block.timestamp + duration);
        assertEq(escrow, rate * duration);
        assertEq(proceeds, 0);
        assertTrue(active);
    }

    function testCreateOrderRejectsShortDuration() public {
        vm.prank(seller);
        vm.expectRevert("TigerSwap: TOO_SHORT");
        twamm.createOrder(true, 1e18, block.timestamp + 1 minutes);
    }

    function testCreateOrderRejectsLongDuration() public {
        vm.prank(seller);
        vm.expectRevert("TigerSwap: TOO_LONG");
        twamm.createOrder(true, 1e18, block.timestamp + 400 days);
    }

    function testCreateOrderRejectsZeroRate() public {
        vm.prank(seller);
        vm.expectRevert("TigerSwap: ZERO_RATE");
        twamm.createOrder(true, 0, block.timestamp + 1 hours);
    }

    // ---- Execution ---------------------------------------------------------

    function testExecutionPreservesTheInvariant() public {
        vm.prank(seller);
        twamm.createOrder(true, 1e18, block.timestamp + 1 hours);

        uint256 kBefore = twamm.reserve0() * twamm.reserve1();

        vm.warp(block.timestamp + 30 minutes);
        twamm.executeVirtualOrders();

        uint256 kAfter = twamm.reserve0() * twamm.reserve1();

        // The closed form preserves x*y exactly; rounding in the square root
        // can only lose a few wei, never gain.
        assertLe(kAfter, kBefore);
        assertApproxEqRel(kAfter, kBefore, 1e12);
    }

    function testExecutionMovesThePriceAgainstTheSeller() public {
        uint256 priceBefore = twamm.price0In1();

        vm.prank(seller);
        twamm.createOrder(true, 1e18, block.timestamp + 1 hours);

        vm.warp(block.timestamp + 30 minutes);
        twamm.executeVirtualOrders();

        // Selling token0 makes token0 cheaper, so its price in token1 falls.
        assertLt(twamm.price0In1(), priceBefore);
    }

    function testSellerReceivesProceeds() public {
        vm.prank(seller);
        uint256 orderId = twamm.createOrder(true, 1e18, block.timestamp + 1 hours);

        vm.warp(block.timestamp + 30 minutes);
        twamm.executeVirtualOrders();

        (, , , , , uint256 proceeds, ) = twamm.orders(orderId);
        assertGt(proceeds, 0);
    }

    function testExecuteIsIdempotentWithinABlock() public {
        vm.prank(seller);
        twamm.createOrder(true, 1e18, block.timestamp + 1 hours);

        vm.warp(block.timestamp + 30 minutes);
        twamm.executeVirtualOrders();
        uint256 reserve0AfterFirst = twamm.reserve0();

        // A second call in the same block must do nothing.
        twamm.executeVirtualOrders();
        assertEq(twamm.reserve0(), reserve0AfterFirst);
    }

    /// @dev Seeds a fee-free pool and returns it, with the seller approved.
    function _freshPool() internal returns (TigerSwapTWAMM pool) {
        pool = new TigerSwapTWAMM(address(token0), address(token1), feeRecipient, 0);
        vm.startPrank(lp);
        token0.approve(address(pool), type(uint256).max);
        token1.approve(address(pool), type(uint256).max);
        pool.addLiquidity(1_000_000e18, 1_000_000e18, 0);
        vm.stopPrank();
        vm.startPrank(seller);
        token0.approve(address(pool), type(uint256).max);
        token1.approve(address(pool), type(uint256).max);
        vm.stopPrank();
    }

    /// @dev Sells `totalSell` of token0 over one hour in 60 keeper steps and
    ///      returns the proceeds.
    function _gradualProceeds(uint256 totalSell) internal returns (uint256 proceeds) {
        TigerSwapTWAMM pool = _freshPool();
        uint256 start = block.timestamp;

        vm.prank(seller);
        uint256 orderId = pool.createOrder(true, totalSell / 1 hours, start + 1 hours);

        for (uint256 i = 0; i < 60; ++i) {
            vm.warp(pool.lastVirtualOrderTimestamp() + 1 minutes);
            pool.executeVirtualOrders();
        }

        (, , , , , proceeds, ) = pool.orders(orderId);
    }

    function testGradualExecutionMatchesOneShotOutput() public {
        // Constant-product swaps are path independent: selling the same total
        // in sixty increments returns what one swap returns, up to rounding.
        // The TWAMM must not lose or invent value, so this is an exact check.
        uint256 totalSell = 100_000e18;
        uint256 gradualProceeds = _gradualProceeds(totalSell);

        uint256 x = 1_000_000e18;
        uint256 y = 1_000_000e18;
        uint256 oneShotProceeds = y - (x * y) / (x + totalSell);

        assertGt(gradualProceeds, 0);
        // Within a few wei of the single-swap result.
        assertApproxEqRel(gradualProceeds, oneShotProceeds, 1e12);
    }

    function testGradualExecutionMovesThePriceInSmallSteps() public {
        // The point of a TWAMM is that the pool price does not jump in one
        // step. Track the largest single-step move and compare it with the
        // total move over the whole order.
        TigerSwapTWAMM pool = _freshPool();
        uint256 start = block.timestamp;
        uint256 totalSell = 100_000e18;

        vm.prank(seller);
        pool.createOrder(true, totalSell / 1 hours, start + 1 hours);

        uint256 priceStart = pool.price0In1();
        uint256 previous = priceStart;
        uint256 largestStep;

        for (uint256 i = 0; i < 60; ++i) {
            vm.warp(pool.lastVirtualOrderTimestamp() + 1 minutes);
            pool.executeVirtualOrders();

            uint256 price = pool.price0In1();
            uint256 step = previous > price ? previous - price : price - previous;
            if (step > largestStep) {
                largestStep = step;
            }
            previous = price;
        }

        uint256 totalMove = priceStart - previous;

        assertGt(totalMove, 0, "price should fall while token0 is sold");
        // Sixty steps, so no single step should be more than a fifth of the
        // total move (it would be about a sixtieth without rounding noise).
        assertLt(largestStep, totalMove / 5, "no step should dominate the move");
    }

    function testBothSidesSellingPreservesValue() public {
        vm.prank(seller);
        twamm.createOrder(true, 1e18, block.timestamp + 1 hours);
        vm.prank(buyer);
        twamm.createOrder(false, 1e18, block.timestamp + 1 hours);

        uint256 kBefore = twamm.reserve0() * twamm.reserve1();

        vm.warp(block.timestamp + 30 minutes);
        twamm.executeVirtualOrders();

        uint256 kAfter = twamm.reserve0() * twamm.reserve1();
        assertApproxEqRel(kAfter, kBefore, 1e12);
    }

    function testTwoSellersShareOutputInProportionToSize() public {
        // Seller stakes 1/3 of the rate that buyer stakes, so should receive
        // about 1/3 of the output on that side.
        vm.prank(seller);
        uint256 smallId = twamm.createOrder(true, 1e18, block.timestamp + 1 hours);
        vm.prank(buyer);
        uint256 largeId = twamm.createOrder(true, 3e18, block.timestamp + 1 hours);

        vm.warp(block.timestamp + 30 minutes);
        twamm.executeVirtualOrders();

        (, , , , , uint256 smallProceeds, ) = twamm.orders(smallId);
        (, , , , , uint256 largeProceeds, ) = twamm.orders(largeId);

        assertGt(smallProceeds, 0);
        assertGt(largeProceeds, smallProceeds);
        // large should be about three times small.
        assertApproxEqRel(largeProceeds, smallProceeds * 3, 1e15);
    }

    function testOrderStopsSellingAtExpiration() public {
        vm.prank(seller);
        uint256 orderId = twamm.createOrder(true, 1e18, block.timestamp + 1 hours);

        // Jump well past expiration.
        vm.warp(block.timestamp + 10 hours);
        twamm.executeVirtualOrders();

        // Only one hour's worth of escrow should have been consumed.
        (, , , , uint256 escrow, , ) = twamm.orders(orderId);
        assertEq(escrow, 0);

        uint256 reserveAfter = twamm.reserve0();
        vm.warp(block.timestamp + 10 hours);
        twamm.executeVirtualOrders();
        // Nothing further to sell.
        assertEq(twamm.reserve0(), reserveAfter);
    }

    // ---- Withdrawal --------------------------------------------------------

    function testWithdrawOrderPaysProceedsAndRefundsEscrow() public {
        vm.prank(seller);
        uint256 orderId = twamm.createOrder(true, 1e18, block.timestamp + 1 hours);

        // Cancel early so some escrow remains.
        vm.warp(block.timestamp + 30 minutes);
        twamm.executeVirtualOrders();

        uint256 token1Before = token1.balanceOf(seller);
        uint256 token0Before = token0.balanceOf(seller);

        vm.prank(seller);
        (uint256 proceeds, uint256 refund) = twamm.withdrawOrder(orderId);

        assertGt(proceeds, 0);
        assertGt(refund, 0);
        assertEq(token1.balanceOf(seller) - token1Before, proceeds);
        assertEq(token0.balanceOf(seller) - token0Before, refund);
    }

    function testWithdrawOrderRejectsNonOwner() public {
        vm.prank(seller);
        uint256 orderId = twamm.createOrder(true, 1e18, block.timestamp + 1 hours);

        vm.prank(buyer);
        vm.expectRevert("TigerSwap: NOT_OWNER");
        twamm.withdrawOrder(orderId);
    }

    function testWithdrawOrderRejectsDoubleWithdrawal() public {
        vm.prank(seller);
        uint256 orderId = twamm.createOrder(true, 1e18, block.timestamp + 1 hours);

        vm.warp(block.timestamp + 1 hours);
        vm.prank(seller);
        twamm.withdrawOrder(orderId);

        vm.prank(seller);
        vm.expectRevert("TigerSwap: NOT_ACTIVE");
        twamm.withdrawOrder(orderId);
    }

    function testCancelOrderRefundsUnspentEscrow() public {
        vm.prank(seller);
        uint256 orderId = twamm.createOrder(true, 1e18, block.timestamp + 1 hours);

        vm.warp(block.timestamp + 10 minutes);
        uint256 before = token0.balanceOf(seller);

        vm.prank(seller);
        (uint256 proceeds, uint256 refund) = twamm.cancelOrder(orderId);

        assertGt(refund, 0);
        assertEq(token0.balanceOf(seller) - before, refund);
        // Half an hour of the hour is unspent after 10 minutes... 50 minutes.
        assertApproxEqRel(refund, 1e18 * 3000, 1e15);
        proceeds;
    }

    // ---- Fees --------------------------------------------------------------

    function testFeesAccrueAndAreCollectable() public {
        vm.prank(seller);
        twamm.createOrder(true, 1e18, block.timestamp + 1 hours);

        vm.warp(block.timestamp + 30 minutes);
        twamm.executeVirtualOrders();

        uint256 fees = twamm.feeReserve0();
        assertGt(fees, 0);
        // 0.3% of 1800 tokens sold.
        assertApproxEqRel(fees, (1800e18 * FEE) / 1e6, 1e15);

        uint256 recipientBefore = token0.balanceOf(feeRecipient);
        twamm.collectFees();
        assertEq(token0.balanceOf(feeRecipient) - recipientBefore, fees);
        assertEq(twamm.feeReserve0(), 0);
    }

    function testSetFeeRejectsExcessiveValue() public {
        vm.expectRevert("TigerSwap: FEE_TOO_HIGH");
        twamm.setFee(1e6);
    }

    function testSetFeeOnlyOwner() public {
        vm.prank(seller);
        vm.expectRevert("TigerSwap: NOT_OWNER");
        twamm.setFee(100);
    }

    // ---- Views -------------------------------------------------------------

    function testActiveSellRateReflectsOpenOrders() public {
        assertEq(twamm.activeSellRate(true), 0);

        vm.prank(seller);
        twamm.createOrder(true, 5e18, block.timestamp + 1 hours);

        assertEq(twamm.activeSellRate(true), 5e18);
        assertEq(twamm.activeSellRate(false), 0);

        // After expiration the rate is no longer active.
        vm.warp(block.timestamp + 2 hours);
        assertEq(twamm.activeSellRate(true), 0);
    }

    function testPriceReflectsReserveRatio() public {
        // A balanced pool prices token0 at 1 token1.
        assertEq(twamm.price0In1(), 1e18);
    }

    // ---- LP share token ----------------------------------------------------

    function testShareTokenTransferWorks() public {
        vm.prank(lp);
        twamm.transfer(seller, 1000e18);
        assertEq(twamm.balanceOf(seller), 1000e18);
    }

    function testShareTransferRejectsInsufficientBalance() public {
        uint256 tooMany = twamm.balanceOf(seller) + 1;
        vm.prank(seller);
        vm.expectRevert("TigerSwap: BALANCE");
        twamm.transfer(lp, tooMany);
    }

    function testShareTransferFromRequiresAllowance() public {
        vm.prank(lp);
        twamm.approve(buyer, 100e18);

        vm.prank(buyer);
        twamm.transferFrom(lp, buyer, 50e18);
        assertEq(twamm.balanceOf(buyer), 50e18);
        assertEq(twamm.allowance(lp, buyer), 50e18);

        vm.prank(buyer);
        vm.expectRevert("TigerSwap: ALLOWANCE");
        twamm.transferFrom(lp, buyer, 100e18);
    }

    // ---- Order cap ---------------------------------------------------------

    function testManyOrdersStillExecuteWithinTheCap() public {
        // Open more orders than one update can touch and confirm execution
        // still makes progress without reverting.
        for (uint256 i = 0; i < 70; ++i) {
            vm.prank(seller);
            twamm.createOrder(true, 1e15, block.timestamp + 1 hours);
        }
        assertEq(twamm.orderCount(), 70);

        vm.warp(block.timestamp + 30 minutes);
        twamm.executeVirtualOrders();
        assertGt(twamm.reserve0(), 0);
    }
}
