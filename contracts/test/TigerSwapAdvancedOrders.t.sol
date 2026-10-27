// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "forge-std/Test.sol";
import "../TigerSwapAdvancedOrders.sol";
import "./TestERC20.sol";
import "./TestPriceOracle.sol";

/// @notice Router stub that swaps 1:1 at a configurable rate. It exists so the
///         order tests can assert exactly what the contract passed down (the
///         minimum output in particular) without pulling in the real pools.
contract TestRouter {
    uint256 public rate = 1e18; // output per 1e18 input
    uint256 public lastAmountOutMin;
    uint256 public lastAmountIn;
    uint256 public calls;

    function setRate(uint256 r) external {
        rate = r;
    }

    function swapExactTokensForTokens(
        uint256 amountIn,
        uint256 amountOutMin,
        address[] calldata path,
        address to,
        uint256
    ) external returns (uint256[] memory amounts) {
        lastAmountIn = amountIn;
        lastAmountOutMin = amountOutMin;
        calls++;

        uint256 out = amountIn * rate / 1e18;
        require(out >= amountOutMin, "TestRouter: INSUFFICIENT_OUTPUT_AMOUNT");

        TestERC20(path[1]).mint(address(this), out);
        TestERC20(path[1]).transfer(to, out);

        amounts = new uint256[](2);
        amounts[0] = amountIn;
        amounts[1] = out;
    }
}

contract TigerSwapAdvancedOrdersTest is Test {
    TigerSwapAdvancedOrders public orders;
    TestRouter public router;
    TestPriceOracle public oracle;
    TestERC20 public tgr;
    TestERC20 public rusd;

    address public alice = address(0xA11CE);
    address public keeper = address(0x4EE9);
    address public stranger = address(0x5BAD);

    uint256 constant PRICE = 2e18;

    function setUp() public {
        tgr = new TestERC20("TGR", "TGR");
        rusd = new TestERC20("RUSD", "RUSD");
        router = new TestRouter();
        oracle = new TestPriceOracle();
        oracle.setPrice(address(tgr), PRICE);

        orders = new TigerSwapAdvancedOrders(address(oracle), address(router));
        orders.setPriceAsset(address(tgr));
        orders.setExecutor(keeper, true);

        tgr.mint(alice, 1_000_000e18);
        vm.prank(alice);
        tgr.approve(address(orders), type(uint256).max);
    }


    /// @dev Reads the fields the tests assert on, by name.
    function _order(uint256 id)
        internal
        view
        returns (uint256 amountIn, uint256 filled, uint256 amountOutReceived, TigerSwapAdvancedOrders.OrderStatus status)
    {
        TigerSwapAdvancedOrders.Order memory o = orders.getOrder(id);
        return (o.amountIn, o.filledAmount, o.amountOutReceived, o.status);
    }

    function _createLimit(uint256 amountIn, uint256 amountOutMin, uint256 price, uint256 expiresAt)
        internal
        returns (uint256)
    {
        vm.prank(alice);
        return orders.createLimitOrder(
            TigerSwapAdvancedOrders.OrderSide.BUY, address(tgr), address(rusd), amountIn, amountOutMin, price, expiresAt
        );
    }

    // ---- Oracle-driven conditions -----------------------------------------

    function testBuyLimitDoesNotFillAboveTheLimitPrice() public {
        // Limit at 1.5, oracle is at 2.0, so the buy must not fill.
        uint256 id = _createLimit(100e18, 0, 1.5e18, block.timestamp + 1 days);

        vm.prank(keeper);
        vm.expectRevert("TigerSwap: PRICE_NOT_MET");
        orders.executeOrder(id, 0);
    }

    function testBuyLimitFillsWhenTheOracleIsAtOrBelowTheLimit() public {
        uint256 id = _createLimit(100e18, 0, 2e18, block.timestamp + 1 days);

        vm.prank(keeper);
        uint256 out = orders.executeOrder(id, 0);
        assertEq(out, 99.7e18, "99.7 after the 0.3% order fee, at 1:1");
    }

    function testSellLimitRequiresTheOracleAtOrAboveTheLimit() public {
        // A sell below the market must not fill.
        vm.prank(alice);
        uint256 id = orders.createLimitOrder(
            TigerSwapAdvancedOrders.OrderSide.SELL,
            address(tgr),
            address(rusd),
            100e18,
            0,
            3e18,
            block.timestamp + 1 days
        );

        vm.prank(keeper);
        vm.expectRevert("TigerSwap: PRICE_NOT_MET");
        orders.executeOrder(id, 0);
    }

    function testKeeperCannotAssertAPriceToForceAFill() public {
        // The oracle is the only price source; the keeper's own view is ignored
        // because executeOrder no longer takes a price argument.
        uint256 id = _createLimit(100e18, 0, 1e18, block.timestamp + 1 days);

        vm.prank(keeper);
        vm.expectRevert("TigerSwap: PRICE_NOT_MET");
        orders.executeOrder(id, 0);
    }

    function testPriceChangeUnblocksAnOrder() public {
        uint256 id = _createLimit(100e18, 0, 1.5e18, block.timestamp + 1 days);

        vm.prank(keeper);
        vm.expectRevert("TigerSwap: PRICE_NOT_MET");
        orders.executeOrder(id, 0);

        // The market moves down to the limit.
        oracle.setPrice(address(tgr), 1.5e18);

        vm.prank(keeper);
        orders.executeOrder(id, 0);

        (,,, TigerSwapAdvancedOrders.OrderStatus status) = _order(id);
        assertEq(uint256(status), uint256(TigerSwapAdvancedOrders.OrderStatus.FILLED), "filled");
    }

    function testStopLossTriggersOnTheOraclePrice() public {
        vm.prank(alice);
        uint256 id = orders.createStopLossOrder(
            TigerSwapAdvancedOrders.OrderSide.SELL,
            address(tgr),
            address(rusd),
            100e18,
            0,
            1e18, // stop when the price falls to 1.0
            block.timestamp + 1 days
        );

        // Price is still 2.0, so the stop has not triggered.
        vm.prank(keeper);
        vm.expectRevert("TigerSwap: TRIGGER_NOT_MET");
        orders.triggerOrder(id);

        oracle.setPrice(address(tgr), 0.9e18);

        vm.prank(keeper);
        orders.triggerOrder(id);


        (,,, TigerSwapAdvancedOrders.OrderStatus status) = _order(id);
        assertEq(uint256(status), uint256(TigerSwapAdvancedOrders.OrderStatus.FILLED), "triggered");
    }

    function testTakeProfitTriggersOnTheOraclePrice() public {
        vm.prank(alice);
        uint256 id = orders.createTakeProfitOrder(
            TigerSwapAdvancedOrders.OrderSide.SELL,
            address(tgr),
            address(rusd),
            100e18,
            0,
            5e18,
            block.timestamp + 1 days
        );

        vm.prank(keeper);
        vm.expectRevert("TigerSwap: TRIGGER_NOT_MET");
        orders.triggerOrder(id);

        oracle.setPrice(address(tgr), 5e18);

        vm.prank(keeper);
        orders.triggerOrder(id);

        (,,, TigerSwapAdvancedOrders.OrderStatus status) = _order(id);
        assertEq(uint256(status), uint256(TigerSwapAdvancedOrders.OrderStatus.FILLED), "triggered");
    }

    // ---- Partial fills and TWAP -------------------------------------------

    function testPartialFillScalesTheMinimumOutput() public {
        // A 1000 order with a 990 floor: the first 100-unit fill must ask for
        // only 99, not 990, or the swap would revert every time.
        uint256 id = _createLimit(1000e18, 990e18, 2e18, block.timestamp + 1 days);

        vm.prank(keeper);
        orders.executeOrder(id, 100e18);

        assertEq(router.lastAmountIn(), 99.7e18, "99.7 after the 0.3% fee");
        assertEq(router.lastAmountOutMin(), 99e18, "floor scaled to the fill");
    }

    function testPartialFillLeavesTheOrderOpen() public {
        uint256 id = _createLimit(1000e18, 0, 2e18, block.timestamp + 1 days);

        vm.prank(keeper);
        orders.executeOrder(id, 100e18);

        (uint256 amountIn, uint256 filled,, TigerSwapAdvancedOrders.OrderStatus status) = _order(id);
        assertEq(amountIn, 1000e18, "full size recorded");
        assertEq(filled, 100e18, "partially filled");
        assertEq(uint256(status), uint256(TigerSwapAdvancedOrders.OrderStatus.PARTIALLY_FILLED), "still open");
    }

    function testOrderCompletesAfterTheLastPartialFill() public {
        uint256 id = _createLimit(200e18, 0, 2e18, block.timestamp + 1 days);

        vm.prank(keeper);
        orders.executeOrder(id, 100e18);
        vm.prank(keeper);
        orders.executeOrder(id, 100e18);

        (, uint256 filled,, TigerSwapAdvancedOrders.OrderStatus status) = _order(id);
        assertEq(filled, 200e18, "fully filled");
        assertEq(uint256(status), uint256(TigerSwapAdvancedOrders.OrderStatus.FILLED), "filled");
    }

    function testCannotFillMoreThanTheOrderSize() public {
        uint256 id = _createLimit(100e18, 0, 2e18, block.timestamp + 1 days);

        // Asking for more than the remaining size fills only the remainder.
        vm.prank(keeper);
        orders.executeOrder(id, 500e18);

        (, uint256 filled,, TigerSwapAdvancedOrders.OrderStatus status) = _order(id);
        assertEq(filled, 100e18, "clamped to the order size");
        assertEq(uint256(status), uint256(TigerSwapAdvancedOrders.OrderStatus.FILLED), "filled");
    }

    function testFilledOrderCannotBeFilledAgain() public {
        uint256 id = _createLimit(100e18, 0, 2e18, block.timestamp + 1 days);

        vm.prank(keeper);
        orders.executeOrder(id, 0);

        vm.prank(keeper);
        vm.expectRevert("TigerSwap: ORDER_NOT_OPEN");
        orders.executeOrder(id, 0);
    }

    function testTWAPFillsInParts() public {
        vm.prank(alice);
        uint256 id = orders.createTWAPOrder(
            TigerSwapAdvancedOrders.OrderSide.BUY,
            address(tgr),
            address(rusd),
            1000e18,
            0,
            10,
            1 hours,
            block.timestamp + 1 days
        );

        // The first part fills immediately.
        vm.prank(keeper);
        orders.executeOrder(id, 0);
        assertEq(router.lastAmountIn(), 99.7e18, "one tenth of the order");

        // A second part before the interval elapses must wait.
        vm.prank(keeper);
        vm.expectRevert("TigerSwap: TWAP_TOO_SOON");
        orders.executeOrder(id, 0);

        vm.warp(block.timestamp + 1 hours + 1);
        vm.prank(keeper);
        orders.executeOrder(id, 0);
        assertEq(router.lastAmountIn(), 99.7e18, "second part");
    }

    function testTWAPCompletesAfterAllParts() public {
        vm.prank(alice);
        uint256 id = orders.createTWAPOrder(
            TigerSwapAdvancedOrders.OrderSide.BUY,
            address(tgr),
            address(rusd),
            200e18,
            0,
            2,
            1 hours,
            block.timestamp + 10 days
        );

        vm.prank(keeper);
        orders.executeOrder(id, 0);

        vm.warp(block.timestamp + 1 hours + 1);
        vm.prank(keeper);
        orders.executeOrder(id, 0);

        (, uint256 filled,, TigerSwapAdvancedOrders.OrderStatus status) = _order(id);
        assertEq(filled, 200e18, "both parts filled");
        assertEq(uint256(status), uint256(TigerSwapAdvancedOrders.OrderStatus.FILLED), "filled");
    }

    // ---- Access control ----------------------------------------------------

    function testOnlyExecutorCanExecute() public {
        uint256 id = _createLimit(100e18, 0, 2e18, block.timestamp + 1 days);

        vm.prank(stranger);
        vm.expectRevert("TigerSwap: NOT_EXECUTOR");
        orders.executeOrder(id, 0);
    }

    function testOnlyExecutorCanTrigger() public {
        vm.prank(alice);
        uint256 id = orders.createStopLossOrder(
            TigerSwapAdvancedOrders.OrderSide.SELL,
            address(tgr),
            address(rusd),
            100e18,
            0,
            1e18,
            block.timestamp + 1 days
        );

        vm.prank(stranger);
        vm.expectRevert("TigerSwap: NOT_EXECUTOR");
        orders.triggerOrder(id);
    }

    function testOnlyOwnerCanManageExecutors() public {
        vm.prank(stranger);
        vm.expectRevert("TigerSwap: NOT_OWNER");
        orders.setExecutor(stranger, true);
    }

    function testSetExecutorRejectsZeroAddress() public {
        vm.expectRevert("TigerSwap: ZERO_ADDRESS");
        orders.setExecutor(address(0), true);
    }

    function testOnlyOwnerCanSetFee() public {
        vm.prank(stranger);
        vm.expectRevert("TigerSwap: NOT_OWNER");
        orders.setFee(100);

        vm.expectRevert("TigerSwap: FEE_TOO_HIGH");
        orders.setFee(501);
    }

    function testConstructorRejectsZeroAddresses() public {
        vm.expectRevert("TigerSwap: ZERO_ORACLE");
        new TigerSwapAdvancedOrders(address(0), address(router));

        vm.expectRevert("TigerSwap: ZERO_ROUTER");
        new TigerSwapAdvancedOrders(address(oracle), address(0));
    }

    // ---- Cancellation and expiry -------------------------------------------

    function testOwnerCanCancelAndRecoverEscrow() public {
        uint256 id = _createLimit(100e18, 0, 2e18, block.timestamp + 1 days);

        uint256 before = tgr.balanceOf(alice);
        vm.prank(alice);
        orders.cancelOrder(id);

        assertEq(tgr.balanceOf(alice), before + 100e18, "escrow returned");
        (,,, TigerSwapAdvancedOrders.OrderStatus status) = _order(id);
        assertEq(uint256(status), uint256(TigerSwapAdvancedOrders.OrderStatus.CANCELLED), "cancelled");
    }

    function testStrangerCannotCancel() public {
        uint256 id = _createLimit(100e18, 0, 2e18, block.timestamp + 1 days);

        vm.prank(stranger);
        vm.expectRevert("TigerSwap: NOT_OWNER");
        orders.cancelOrder(id);
    }

    function testExpiredOrderCannotBeExecuted() public {
        uint256 id = _createLimit(100e18, 0, 2e18, block.timestamp + 1 hours);

        vm.warp(block.timestamp + 2 hours);

        vm.prank(keeper);
        vm.expectRevert("TigerSwap: ORDER_EXPIRED");
        orders.executeOrder(id, 0);
    }

    function testExpiredOrderCanBeExpiredAndRefunded() public {
        uint256 id = _createLimit(100e18, 0, 2e18, block.timestamp + 1 hours);

        vm.warp(block.timestamp + 2 hours);

        uint256 before = tgr.balanceOf(alice);
        orders.expireOrder(id);

        assertEq(tgr.balanceOf(alice), before + 100e18, "escrow returned");
    }

    function testCreateOrderRejectsBadInput() public {
        vm.prank(alice);
        vm.expectRevert("TigerSwap: ZERO_AMOUNT");
        orders.createLimitOrder(
            TigerSwapAdvancedOrders.OrderSide.BUY, address(tgr), address(rusd), 0, 0, 2e18, block.timestamp + 1 days
        );

        vm.prank(alice);
        vm.expectRevert("TigerSwap: SAME_TOKEN");
        orders.createLimitOrder(
            TigerSwapAdvancedOrders.OrderSide.BUY, address(tgr), address(tgr), 1e18, 0, 2e18, block.timestamp + 1 days
        );

        vm.prank(alice);
        vm.expectRevert("TigerSwap: ZERO_TOKEN");
        orders.createLimitOrder(
            TigerSwapAdvancedOrders.OrderSide.BUY, address(0), address(rusd), 1e18, 0, 2e18, block.timestamp + 1 days
        );
    }

    function testOwnerCanWithdrawFees() public {
        uint256 id = _createLimit(100e18, 0, 2e18, block.timestamp + 1 days);

        vm.prank(keeper);
        orders.executeOrder(id, 0);

        assertEq(orders.feesCollected(address(tgr)), 0.3e18, "0.3% fee collected");

        orders.withdrawFees(address(tgr), stranger);
        assertEq(tgr.balanceOf(stranger), 0.3e18, "fees withdrawn");
    }

    function testWithdrawFeesRejectsEmpty() public {
        vm.expectRevert("TigerSwap: NO_FEES");
        orders.withdrawFees(address(tgr), stranger);
    }
}
