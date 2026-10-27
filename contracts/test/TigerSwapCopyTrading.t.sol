// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "forge-std/Test.sol";
import "../TigerSwapCopyTrading.sol";
import "./TestERC20.sol";
import "./TestPriceOracle.sol";

contract TigerSwapCopyTradingTest is Test {
    TigerSwapCopyTrading public copy;
    TestERC20 public rusd;
    TestPriceOracle public oracle;

    address public alice = address(0xA11CE);
    address public bob = address(0xB0B);
    address public trader = address(0x7AD);
    address public stranger = address(0x5BAD);

    uint256 constant PRICE = 60_000e18;
    uint256 constant DEPOSIT = 1_000e18;

    function setUp() public {
        rusd = new TestERC20("RUSD", "RUSD");
        oracle = new TestPriceOracle();
        oracle.setPrice(address(rusd), PRICE);

        copy = new TigerSwapCopyTrading(address(rusd), address(oracle));
        copy.setPriceAsset(address(rusd));

        for (uint256 i = 0; i < 3; i++) {
            address who = [alice, bob, trader][i];
            rusd.mint(who, 1_000_000e18);
            vm.prank(who);
            rusd.approve(address(copy), type(uint256).max);
        }

        vm.prank(trader);
        copy.registerTrader("Tiger", 1000); // 10% performance fee
    }

    function _deposit(address who, uint256 amount) internal {
        vm.prank(who);
        copy.deposit(amount);
    }

    /// @dev Follow the trader with DEPOSIT (which deposits it) and open one
    ///      position of `amount`, so the follower's whole balance is DEPOSIT.
    function _open(address who, uint256 amount, bool isLong) internal returns (bytes32 pos) {
        vm.prank(who);
        copy.followTrader(trader, DEPOSIT);
        vm.prank(who);
        pos = copy.openCopyTrade(who, trader, bytes32("orig"), amount, isLong);
    }

    function _setPrice(uint256 price) internal {
        oracle.setPrice(address(rusd), price);
    }

    // ---- Funds stay with their owner ---------------------------------------

    function testDepositIsOwnedByTheDepositor() public {
        _deposit(alice, DEPOSIT);

        assertEq(copy.balances(alice, address(rusd)), DEPOSIT, "balance credited");
        assertEq(copy.freeBalance(alice), DEPOSIT, "fully free");
        assertEq(rusd.balanceOf(address(copy)), DEPOSIT, "held by the contract");
    }

    function testWithdrawReturnsTheDeposit() public {
        _deposit(alice, DEPOSIT);

        uint256 before = rusd.balanceOf(alice);
        vm.prank(alice);
        copy.withdraw(DEPOSIT);

        assertEq(rusd.balanceOf(alice), before + DEPOSIT, "returned");
        assertEq(copy.freeBalance(alice), 0, "balance cleared");
    }

    function testCannotWithdrawMoreThanDeposited() public {
        _deposit(alice, DEPOSIT);
        _deposit(bob, DEPOSIT);

        // Alice must not be able to reach into bob's deposit.
        vm.prank(alice);
        vm.expectRevert("TigerSwap: INSUFFICIENT_BALANCE");
        copy.withdraw(DEPOSIT + 1);
    }

    function testOneFollowerCannotSpendAnothersBalance() public {
        _deposit(bob, DEPOSIT);

        vm.prank(alice);
        copy.followTrader(trader, DEPOSIT);
        vm.prank(alice);
        copy.openCopyTrade(alice, trader, bytes32("a"), 400e18, true);

        // Bob has no escrow, so his free balance is untouched by Alice.
        assertEq(copy.freeBalance(bob), DEPOSIT, "bob's balance intact");
        assertEq(copy.freeBalance(alice), DEPOSIT - 400e18, "alice committed her own funds");
    }

    function testEscrowedFundsCannotBeWithdrawn() public {
        vm.prank(alice);
        copy.followTrader(trader, DEPOSIT);
        vm.prank(alice);
        copy.openCopyTrade(alice, trader, bytes32("a"), 500e18, true);

        assertEq(copy.freeBalance(alice), 500e18, "half free");

        vm.prank(alice);
        vm.expectRevert("TigerSwap: INSUFFICIENT_BALANCE");
        copy.withdraw(500e18 + 1);
    }

    function testContractIsSolventAfterProfitableAndLosingTrades() public {
        bytes32 alicePos = _open(alice, DEPOSIT, true);
        bytes32 bobPos = _open(bob, DEPOSIT, true);

        // Alice wins, bob loses.
        _setPrice(PRICE * 11 / 10);
        vm.prank(alice);
        copy.closeCopyTrade(alicePos);

        _setPrice(PRICE * 9 / 10);
        vm.prank(bob);
        copy.closeCopyTrade(bobPos);

        // Every token the contract holds is owed to someone.
        uint256 owed = copy.balances(alice, address(rusd)) + copy.balances(bob, address(rusd))
            + copy.balances(trader, address(rusd)) + copy.platformFeesStored();
        assertGe(rusd.balanceOf(address(copy)), owed, "contract fully backs its credits");
    }

    function testLosingPositionNeverTakesAnotherFollowersFunds() public {
        // A short against a tripling price loses twice the principal, which is
        // capped at the principal so alice is wiped out but never goes negative.
        bytes32 alicePos = _open(alice, DEPOSIT, false);
        _deposit(bob, DEPOSIT);

        _setPrice(PRICE * 3);
        vm.prank(alice);
        copy.closeCopyTrade(alicePos);

        assertEq(copy.balances(alice, address(rusd)), 0, "alice wiped out");
        assertEq(copy.freeBalance(bob), DEPOSIT, "bob untouched");

        // Bob can still withdraw his whole deposit.
        uint256 before = rusd.balanceOf(bob);
        vm.prank(bob);
        copy.withdraw(DEPOSIT);
        assertEq(rusd.balanceOf(bob), before + DEPOSIT, "bob made whole");
    }

    // ---- Position accounting ----------------------------------------------

    function testOpenCopyTradeUsesTheOraclePrice() public {
        bytes32 pos = _open(alice, 100e18, true);

        (, , , , , uint256 entryPrice, , , ) = copy.copyPositions(pos);
        assertEq(entryPrice, PRICE, "entry from the oracle, not the caller");
    }

    function testOpenCopyTradeRejectsMoreThanFreeBalance() public {
        vm.prank(alice);
        copy.followTrader(trader, DEPOSIT);

        vm.prank(alice);
        vm.expectRevert("TigerSwap: INSUFFICIENT_BALANCE");
        copy.openCopyTrade(alice, trader, bytes32("x"), DEPOSIT + 1, true);
    }

    function testOpenCopyTradeRejectsNonFollower() public {
        _deposit(bob, DEPOSIT);

        vm.prank(bob);
        vm.expectRevert("TigerSwap: NOT_FOLLOWING");
        copy.openCopyTrade(bob, trader, bytes32("x"), 100e18, true);
    }

    function testStrangerCannotOpenAPositionForAFollower() public {
        vm.prank(alice);
        copy.followTrader(trader, DEPOSIT);

        vm.prank(stranger);
        vm.expectRevert("TigerSwap: NOT_EXECUTOR");
        copy.openCopyTrade(alice, trader, bytes32("x"), 100e18, true);
    }

    function testExecutorCanOpenOnBehalfOfAFollower() public {
        vm.prank(alice);
        copy.followTrader(trader, DEPOSIT);

        copy.setExecutor(stranger);
        vm.prank(stranger);
        bytes32 pos = copy.openCopyTrade(alice, trader, bytes32("x"), 100e18, true);

        assertEq(copy.escrowed(alice, address(rusd)), 100e18, "escrowed");
        (, , , , uint256 amount, , , , ) = copy.copyPositions(pos);
        assertEq(amount, 100e18, "amount recorded");
    }

    function testOnlyFollowerCanClose() public {
        bytes32 pos = _open(alice, 100e18, true);

        vm.prank(stranger);
        vm.expectRevert("TigerSwap: NOT_FOLLOWER");
        copy.closeCopyTrade(pos);
    }

    function testCannotCloseTwice() public {
        bytes32 pos = _open(alice, 100e18, true);

        vm.prank(alice);
        copy.closeCopyTrade(pos);

        vm.prank(alice);
        vm.expectRevert("TigerSwap: ALREADY_CLOSED");
        copy.closeCopyTrade(pos);
    }

    // ---- Profit and loss ---------------------------------------------------

    function testProfitableLongSplitsProfitCorrectly() public {
        bytes32 pos = _open(alice, DEPOSIT, true);

        // 10% up: profit = 100.
        _setPrice(PRICE * 11 / 10);

        vm.prank(alice);
        copy.closeCopyTrade(pos);

        // profit 100; trader 10% = 10; platform 0.5% = 0.5; alice keeps 89.5.
        uint256 traderFee = 100e18 * 1000 / 10_000;
        uint256 platformCut = 100e18 * 50 / 10_000;
        uint256 aliceProfit = 100e18 - traderFee - platformCut;

        assertEq(copy.balances(alice, address(rusd)), DEPOSIT + aliceProfit, "principal plus profit");
        assertEq(copy.balances(trader, address(rusd)), traderFee, "trader fee");
        assertEq(copy.platformFeesStored(), platformCut, "platform fee");
    }

    function testProfitableShortPaysOut() public {
        bytes32 pos = _open(alice, DEPOSIT, false);

        _setPrice(PRICE * 9 / 10);

        vm.prank(alice);
        copy.closeCopyTrade(pos);

        assertGt(copy.balances(alice, address(rusd)), DEPOSIT, "short profited");
    }

    function testLossIsCappedAtTheEscrowedPrincipal() public {
        // A short against a tripling price owes twice the principal, so the
        // loss must be clamped to the principal rather than underflowing.
        bytes32 pos = _open(alice, DEPOSIT, false);
        _setPrice(PRICE * 3);

        vm.prank(alice);
        copy.closeCopyTrade(pos);

        assertEq(copy.balances(alice, address(rusd)), 0, "loss capped at principal");
        assertEq(copy.escrowed(alice, address(rusd)), 0, "escrow released");
    }

    function testLongLossCannotExceedThePrincipal() public {
        bytes32 pos = _open(alice, DEPOSIT, true);

        // A long can never lose more than 100%, so a full crash returns zero.
        _setPrice(1e18);
        vm.prank(alice);
        copy.closeCopyTrade(pos);

        uint256 loss = DEPOSIT - copy.balances(alice, address(rusd));
        assertLe(loss, DEPOSIT, "loss never exceeds the principal");
        // The loss divides down, so the follower keeps the rounding remainder,
        // which is a few parts in 10^5 of the principal at most.
        assertLt(copy.balances(alice, address(rusd)), 2e16, "almost everything lost");
    }

    function testPartialLossReturnsTheRemainder() public {
        bytes32 pos = _open(alice, DEPOSIT, true);

        // 20% down: loss 200, so 800 comes back.
        _setPrice(PRICE * 8 / 10);

        vm.prank(alice);
        copy.closeCopyTrade(pos);

        assertEq(copy.balances(alice, address(rusd)), 800e18, "remainder returned");
        assertEq(copy.freeBalance(alice), 800e18, "and is withdrawable");
    }

    function testClosingAtTheEntryPriceReturnsThePrincipal() public {
        bytes32 pos = _open(alice, DEPOSIT, true);

        vm.prank(alice);
        copy.closeCopyTrade(pos);

        assertEq(copy.balances(alice, address(rusd)), DEPOSIT, "principal returned intact");
    }

    // ---- Trader registry ---------------------------------------------------

    function testUpdateTraderEnforcesTheFeeCap() public {
        vm.prank(trader);
        vm.expectRevert("TigerSwap: FEE_TOO_HIGH");
        copy.updateTrader("Tiger", 3001);
    }

    function testUpdateTraderAllowsAFeeWithinTheCap() public {
        vm.prank(trader);
        copy.updateTrader("Tiger Two", 2000);

        (, , , , , , uint256 fee, ) = copy.traders(trader);
        assertEq(fee, 2000, "fee updated");
    }

    function testRegisterTraderRejectsAnExcessiveFee() public {
        vm.prank(bob);
        vm.expectRevert("TigerSwap: FEE_TOO_HIGH");
        copy.registerTrader("Bob", 3001);
    }

    function testRegisterTraderRejectsDuplicates() public {
        vm.prank(trader);
        vm.expectRevert("TigerSwap: ALREADY_REGISTERED");
        copy.registerTrader("Again", 100);
    }

    function testFollowRequiresTheMinimumAmount() public {
        vm.prank(alice);
        vm.expectRevert("TigerSwap: AMOUNT_TOO_LOW");
        copy.followTrader(trader, 1e18); // minimum is 100e18
    }

    function testFollowRequiresAnActiveTrader() public {
        vm.prank(alice);
        vm.expectRevert("TigerSwap: TRADER_NOT_ACTIVE");
        copy.followTrader(stranger, DEPOSIT);
    }

    function testCannotFollowTwice() public {
        vm.prank(alice);
        copy.followTrader(trader, DEPOSIT);

        vm.prank(alice);
        vm.expectRevert("TigerSwap: ALREADY_FOLLOWING");
        copy.followTrader(trader, DEPOSIT);
    }

    function testUnfollowKeepsTheBalance() public {
        vm.prank(alice);
        copy.followTrader(trader, DEPOSIT);

        vm.prank(alice);
        copy.unfollowTrader(trader);

        assertEq(copy.freeBalance(alice), DEPOSIT, "funds retained");
        assertEq(copy.getFollowerCount(trader), 0, "follower removed");
    }

    function testUnfollowRequiresFollowing() public {
        vm.prank(alice);
        vm.expectRevert("TigerSwap: NOT_FOLLOWING");
        copy.unfollowTrader(trader);
    }

    // ---- Admin -------------------------------------------------------------

    function testOnlyOwnerCanSetPlatformFee() public {
        vm.prank(stranger);
        vm.expectRevert("TigerSwap: NOT_OWNER");
        copy.setPlatformFee(100);
    }

    function testPlatformFeeIsCapped() public {
        vm.expectRevert("TigerSwap: FEE_TOO_HIGH");
        copy.setPlatformFee(501);
    }

    function testWithdrawPlatformFees() public {
        bytes32 pos = _open(alice, DEPOSIT, true);
        _setPrice(PRICE * 11 / 10);

        vm.prank(alice);
        copy.closeCopyTrade(pos);

        uint256 accrued = copy.platformFeesStored();
        assertGt(accrued, 0, "fees accrued");

        uint256 before = rusd.balanceOf(stranger);
        copy.withdrawPlatformFees(stranger, accrued);

        assertEq(rusd.balanceOf(stranger), before + accrued, "fees withdrawn");
        assertEq(copy.platformFeesStored(), 0, "cleared");
    }

    function testWithdrawPlatformFeesRejectsOverdraw() public {
        vm.expectRevert("TigerSwap: INSUFFICIENT_FEES");
        copy.withdrawPlatformFees(stranger, 1e18);
    }

    function testTwoStepOwnershipTransfer() public {
        copy.commitTransferOwnership(stranger);
        assertEq(copy.futureOwner(), stranger, "pending owner");

        vm.prank(stranger);
        copy.acceptTransferOwnership();
        assertEq(copy.owner(), stranger, "ownership moved");
    }

    function testOnlyFutureOwnerCanAcceptOwnership() public {
        copy.commitTransferOwnership(stranger);
        vm.prank(bob);
        vm.expectRevert("TigerSwap: NOT_FUTURE_OWNER");
        copy.acceptTransferOwnership();
    }

    function testConstructorRejectsZeroAddresses() public {
        vm.expectRevert("TigerSwap: ZERO_TOKEN");
        new TigerSwapCopyTrading(address(0), address(oracle));

        vm.expectRevert("TigerSwap: ZERO_ORACLE");
        new TigerSwapCopyTrading(address(rusd), address(0));
    }

    function testTopTradersRanksByProfit() public {
        vm.prank(bob);
        copy.registerTrader("Bob", 500);

        // Give the trader a cumulative profit while bob stays at zero.
        bytes32 pos = _open(alice, DEPOSIT, true);
        _setPrice(PRICE * 11 / 10);
        vm.prank(alice);
        copy.closeCopyTrade(pos);

        address[] memory candidates = new address[](2);
        candidates[0] = bob;
        candidates[1] = trader;

        TigerSwapCopyTrading.Trader[] memory top = copy.getTopTraders(candidates, 2);
        assertEq(top[0].traderAddress, trader, "the profitable trader ranks first");
    }
}
