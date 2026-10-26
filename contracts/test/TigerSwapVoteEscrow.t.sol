// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "forge-std/Test.sol";
import "../TigerSwapVoteEscrow.sol";

contract MockTGR {
    string public name = "TigerSwap";
    string public symbol = "TGR";
    uint8 public decimals = 18;
    uint256 public totalSupply;
    mapping(address => uint256) public balanceOf;
    mapping(address => mapping(address => uint256)) public allowance;

    event Transfer(address indexed from, address indexed to, uint256 value);
    event Approval(address indexed owner, address indexed spender, uint256 value);

    function mint(address to, uint256 amount) external {
        totalSupply += amount;
        balanceOf[to] += amount;
        emit Transfer(address(0), to, amount);
    }

    function approve(address spender, uint256 amount) external returns (bool) {
        allowance[msg.sender][spender] = amount;
        emit Approval(msg.sender, spender, amount);
        return true;
    }

    function transfer(address to, uint256 amount) external returns (bool) {
        balanceOf[msg.sender] -= amount;
        balanceOf[to] += amount;
        emit Transfer(msg.sender, to, amount);
        return true;
    }

    function transferFrom(address from, address to, uint256 amount) external returns (bool) {
        allowance[from][msg.sender] -= amount;
        balanceOf[from] -= amount;
        balanceOf[to] += amount;
        emit Transfer(from, to, amount);
        return true;
    }
}

contract VoteEscrowTest is Test {
    TigerSwapVoteEscrow ve;
    MockTGR tgr;

    address alice = address(0xA11CE);
    address bob = address(0xB0B);
    address gaugeA = address(0x6A06E1);
    address gaugeB = address(0x6A06E2);

    uint256 constant WEEK = 86400 * 7;
    uint256 constant MAXTIME = 4 * 365 * 86400;

    function setUp() public {
        tgr = new MockTGR();
        ve = new TigerSwapVoteEscrow(address(tgr));
        ve.addGauge(gaugeA);
        ve.addGauge(gaugeB);

        tgr.mint(alice, 10_000e18);
        tgr.mint(bob, 10_000e18);
        vm.prank(alice);
        tgr.approve(address(ve), type(uint256).max);
        vm.prank(bob);
        tgr.approve(address(ve), type(uint256).max);
    }

    function _lock(address who, uint256 amount, uint256 duration) internal {
        vm.prank(who);
        ve.createLock(amount, block.timestamp + duration);
    }

    // ---- Locking -----------------------------------------------------------

    function testCreateLockStoresAmountAndTime() public {
        _lock(alice, 4000e18, MAXTIME);
        (int128 amount, uint256 start, uint256 end) = ve.locked(alice);
        assertEq(uint256(uint128(amount)), 4000e18, "amount");
        assertEq(start, block.timestamp, "start");
        // A max-duration request is clamped to the last week boundary inside MAXTIME.
        assertApproxEqAbs(end, block.timestamp + MAXTIME, WEEK, "end within one week of max");
    }

    function testCreateLockMovesTokensIn() public {
        _lock(alice, 1000e18, MAXTIME);
        assertEq(tgr.balanceOf(address(ve)), 1000e18, "escrow holds the deposit");
        assertEq(tgr.balanceOf(alice), 9000e18, "alice debited");
    }

    function testFullLockGivesFullVotingPower() public {
        _lock(alice, 4000e18, MAXTIME);
        // The term is week-aligned, so power is marginally below the amount.
        uint256 power = ve.balanceOf(alice);
        assertApproxEqRel(power, 4000e18, 0.005e18, "full-term lock ~= amount");
    }

    function testVotingPowerDecaysLinearlyToZero() public {
        _lock(alice, 4000e18, MAXTIME);
        uint256 atStart = ve.balanceOf(alice);

        vm.warp(block.timestamp + MAXTIME / 2);
        uint256 atHalf = ve.balanceOf(alice);

        assertApproxEqRel(atHalf, atStart / 2, 0.02e18, "halfway ~= half power");

        vm.warp(block.timestamp + MAXTIME / 2 + WEEK);
        assertEq(ve.balanceOf(alice), 0, "fully decayed after expiry");
    }

    function testCannotCreateLockBelowMinimumDuration() public {
        vm.prank(alice);
        vm.expectRevert("lock too short");
        ve.createLock(100e18, block.timestamp + WEEK - 1);
    }

    function testLockBeyondMaxTimeIsClampedToMax() public {
        vm.prank(alice);
        ve.createLock(100e18, block.timestamp + MAXTIME * 2);
        (,, uint256 end) = ve.locked(alice);
        assertLe(end, block.timestamp + MAXTIME, "clamped within MAXTIME");
        assertGe(end, block.timestamp + MAXTIME - WEEK, "clamped near MAXTIME");
    }

    function testCannotCreateLockInThePast() public {
        vm.prank(alice);
        vm.expectRevert("unlock in past");
        ve.createLock(100e18, block.timestamp - 1);
    }

    function testCannotLockZeroAmount() public {
        vm.prank(alice);
        vm.expectRevert("amount required");
        ve.createLock(0, block.timestamp + MAXTIME);
    }

    function testCannotCreateSecondLock() public {
        _lock(alice, 100e18, MAXTIME);
        vm.prank(alice);
        vm.expectRevert("existing lock");
        ve.createLock(100e18, block.timestamp + MAXTIME);
    }

    // ---- Increasing --------------------------------------------------------

    function testIncreaseAmountAddsToLock() public {
        _lock(alice, 1000e18, MAXTIME);
        vm.prank(alice);
        ve.increaseAmount(500e18);

        (int128 amount,,) = ve.locked(alice);
        assertEq(uint256(uint128(amount)), 1500e18, "amount increased");
        assertEq(tgr.balanceOf(address(ve)), 1500e18, "escrow balance increased");
    }

    function testIncreaseAmountRequiresExistingLock() public {
        vm.prank(alice);
        vm.expectRevert("no existing lock");
        ve.increaseAmount(100e18);
    }

    function testIncreaseUnlockTimeExtends() public {
        _lock(alice, 1000e18, WEEK * 10);
        uint256 newEnd = ((block.timestamp + MAXTIME) / WEEK) * WEEK;

        vm.prank(alice);
        ve.increaseUnlockTime(block.timestamp + MAXTIME);

        (,, uint256 end) = ve.locked(alice);
        assertEq(end, newEnd, "unlock time extended");
    }

    function testIncreaseUnlockTimeCannotShorten() public {
        _lock(alice, 1000e18, MAXTIME);
        vm.prank(alice);
        vm.expectRevert("only increase");
        ve.increaseUnlockTime(block.timestamp + WEEK);
    }

    // ---- Withdraw ----------------------------------------------------------

    function testWithdrawAfterExpiryReturnsTokens() public {
        _lock(alice, 1000e18, WEEK);
        (,, uint256 end) = ve.locked(alice);
        vm.warp(end + 1);

        vm.prank(alice);
        ve.withdraw();

        assertEq(tgr.balanceOf(alice), 10_000e18, "tokens returned in full");
        assertEq(ve.balanceOf(alice), 0, "no voting power left");
    }

    function testWithdrawBeforeExpiryReverts() public {
        _lock(alice, 1000e18, MAXTIME);
        vm.prank(alice);
        vm.expectRevert("lock not expired");
        ve.withdraw();
    }

    function testWithdrawWithoutLockReverts() public {
        vm.prank(bob);
        vm.expectRevert("no lock");
        ve.withdraw();
    }

    function testCannotWithdrawTwice() public {
        _lock(alice, 1000e18, WEEK);
        (,, uint256 end) = ve.locked(alice);
        vm.warp(end + 1);

        vm.prank(alice);
        ve.withdraw();

        vm.prank(alice);
        vm.expectRevert("no lock");
        ve.withdraw();
    }

    // ---- totalSupply -------------------------------------------------------

    function testTotalSupplyMatchesSumOfHolders() public {
        _lock(alice, 4000e18, MAXTIME);
        _lock(bob, 2000e18, MAXTIME);

        ve.checkpoint();
        uint256 sum = ve.balanceOf(alice) + ve.balanceOf(bob);
        assertApproxEqRel(ve.totalSupply(), sum, 0.01e18, "supply == sum of holders");

        vm.warp(block.timestamp + MAXTIME / 2);
        ve.checkpoint();
        uint256 sumLater = ve.balanceOf(alice) + ve.balanceOf(bob);
        assertApproxEqRel(ve.totalSupply(), sumLater, 0.01e18, "supply tracks decay");
    }

    function testTotalSupplyZeroWithNoLocks() public {
        assertEq(ve.totalSupply(), 0, "no locks means no supply");
    }

    // ---- Gauge voting ------------------------------------------------------

    function testVoteAllocatesWeight() public {
        _lock(alice, 4000e18, MAXTIME);
        uint256 power = ve.balanceOf(alice);

        vm.prank(alice);
        ve.vote(gaugeA, power);

        assertEq(ve.userGaugeWeight(alice, gaugeA), power, "user weight recorded");
        assertEq(ve.gaugeWeight(gaugeA), power, "gauge weight recorded");
    }

    function testVoteCannotExceedVotingPower() public {
        _lock(alice, 1000e18, MAXTIME);
        uint256 power = ve.balanceOf(alice);

        vm.prank(alice);
        vm.expectRevert("insufficient voting power");
        ve.vote(gaugeA, power + 1e18);
    }

    function testVoteCanBeRedistributedBetweenGauges() public {
        _lock(alice, 4000e18, MAXTIME);
        uint256 power = ve.balanceOf(alice);

        vm.prank(alice);
        ve.vote(gaugeA, power);

        vm.prank(alice);
        ve.vote(gaugeA, power / 4);
        vm.prank(alice);
        ve.vote(gaugeB, power - power / 4);

        assertEq(ve.userGaugeWeight(alice, gaugeA), power / 4, "gaugeA reduced");
        assertEq(ve.userGaugeWeight(alice, gaugeB), power - power / 4, "gaugeB set");
        assertEq(ve.userTotalWeight(alice), power, "total unchanged");
    }

    function testVoteOnUnknownGaugeReverts() public {
        _lock(alice, 1000e18, MAXTIME);
        vm.prank(alice);
        vm.expectRevert("not a gauge");
        ve.vote(address(0xDEAD), 1);
    }

    // ---- Bribes ------------------------------------------------------------

    function testBribeIsSplitProRataByVoteWeight() public {
        _lock(alice, 4000e18, MAXTIME);
        _lock(bob, 1000e18, MAXTIME);

        uint256 alicePower = ve.balanceOf(alice);
        uint256 bobPower = ve.balanceOf(bob);

        vm.prank(alice);
        ve.vote(gaugeA, alicePower);
        vm.prank(bob);
        ve.vote(gaugeA, bobPower);

        // Funder adds a bribe of 1000 RUSD to gaugeA for this epoch.
        MockTGR bribeToken = new MockTGR();
        bribeToken.mint(address(this), 1000e18);
        bribeToken.approve(address(ve), type(uint256).max);
        ve.addBribe(gaugeA, address(bribeToken), 1000e18);

        uint256 epoch = block.timestamp / WEEK;
        uint256 aliceBefore = bribeToken.balanceOf(alice);
        uint256 bobBefore = bribeToken.balanceOf(bob);

        vm.prank(alice);
        uint256 aliceShare = ve.claimBribe(gaugeA, epoch, address(bribeToken));
        vm.prank(bob);
        uint256 bobShare = ve.claimBribe(gaugeA, epoch, address(bribeToken));

        assertEq(aliceShare + bobShare, 1000e18, "bribe fully distributed");
        // Alice has 4x Bob's weight, so she should receive ~4x the share.
        assertApproxEqRel(aliceShare, bobShare * 4, 0.01e18, "share is proportional to weight");
        assertEq(bribeToken.balanceOf(alice) - aliceBefore, aliceShare, "alice paid");
        assertEq(bribeToken.balanceOf(bob) - bobBefore, bobShare, "bob paid");
    }

    function testCannotClaimBribeTwice() public {
        _lock(alice, 1000e18, MAXTIME);
        uint256 power = ve.balanceOf(alice);
        vm.prank(alice);
        ve.vote(gaugeA, power);

        MockTGR bribeToken = new MockTGR();
        bribeToken.mint(address(this), 100e18);
        bribeToken.approve(address(ve), type(uint256).max);
        ve.addBribe(gaugeA, address(bribeToken), 100e18);

        uint256 epoch = block.timestamp / WEEK;
        vm.prank(alice);
        ve.claimBribe(gaugeA, epoch, address(bribeToken));

        vm.prank(alice);
        vm.expectRevert("already claimed");
        ve.claimBribe(gaugeA, epoch, address(bribeToken));
    }

    function testClaimBribeWithoutWeightReverts() public {
        _lock(alice, 1000e18, MAXTIME);
        MockTGR bribeToken = new MockTGR();
        bribeToken.mint(address(this), 100e18);
        bribeToken.approve(address(ve), type(uint256).max);
        ve.addBribe(gaugeA, address(bribeToken), 100e18);

        vm.prank(bob);
        vm.expectRevert("nothing to claim");
        ve.claimBribe(gaugeA, block.timestamp / WEEK, address(bribeToken));
    }

    // ---- Boost -------------------------------------------------------------

    function testBoostIsOneXWithoutVeTGR() public {
        assertEq(ve.boostBps(alice, 1000e18), 10000, "no veTGR means 1x");
    }

    function testBoostCapsAtTwoPointFiveX() public {
        _lock(alice, 10_000e18, MAXTIME);
        // veTGR far exceeds staked, so boost must saturate at 2.5x.
        assertEq(ve.boostBps(alice, 1e18), 25000, "capped at 2.5x");
    }

    function testBoostScalesWithVeTGRRatio() public {
        _lock(alice, 1000e18, MAXTIME);
        uint256 ve1 = ve.balanceOf(alice);
        // staked == veTGR => 1x + 1.5x*1 = 2.5x
        assertEq(ve.boostBps(alice, ve1), 25000, "equal ratio gives max boost");

        // staked == 3x veTGR => 1x + 1.5x/3 = 1.5x
        assertEq(ve.boostBps(alice, ve1 * 3), 15000, "one third ratio gives 1.5x");
    }

    function testBoostZeroStakeReturnsBaseline() public {
        _lock(alice, 1000e18, MAXTIME);
        assertEq(ve.boostBps(alice, 0), 10000, "no stake means baseline");
    }

    // ---- Gauge admin -------------------------------------------------------

    function testOnlyOwnerCanAddGauge() public {
        vm.prank(alice);
        vm.expectRevert("not owner");
        ve.addGauge(address(0x1234));
    }

    function testCannotAddDuplicateGauge() public {
        vm.expectRevert("gauge exists");
        ve.addGauge(gaugeA);
    }

    function testGaugeCountTracksAdditions() public {
        assertEq(ve.gaugeCount(), 2, "two gauges registered in setUp");
        ve.addGauge(address(0x9999));
        assertEq(ve.gaugeCount(), 3, "third gauge registered");
    }

    // ---- Ownership ---------------------------------------------------------

    function testOwnershipTransferIsTwoStep() public {
        ve.transferOwnership(bob);
        assertEq(ve.owner(), address(this), "owner unchanged until accepted");

        vm.prank(bob);
        ve.acceptOwnership();
        assertEq(ve.owner(), bob, "ownership accepted");
    }

    function testAcceptOwnershipRejectsNonPending() public {
        ve.transferOwnership(bob);
        vm.prank(alice);
        vm.expectRevert("not pending owner");
        ve.acceptOwnership();
    }

    function testCannotTransferOwnershipToZero() public {
        vm.expectRevert("zero address");
        ve.transferOwnership(address(0));
    }
}
