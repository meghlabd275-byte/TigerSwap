// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "forge-std/Test.sol";
import "../TigerSwapLocker.sol";

contract LockERC20 {
    string public name = "Token";
    string public symbol = "TKN";
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

contract LockerTest is Test {
    TigerSwapLocker locker;
    LockERC20 lp;
    LockERC20 tgr;

    address alice = address(0xA11CE);
    address bob = address(0xB0B);

    uint256 constant WEEK = 86400 * 7;
    uint256 constant MAX_LOCK = 4 * 365 * 86400;

    function setUp() public {
        lp = new LockERC20();
        tgr = new LockERC20();
        locker = new TigerSwapLocker(address(lp), address(tgr));

        lp.mint(alice, 1000e18);
        lp.mint(bob, 1000e18);
        vm.prank(alice);
        lp.approve(address(locker), type(uint256).max);
        vm.prank(bob);
        lp.approve(address(locker), type(uint256).max);
    }

    // ---- Boost schedule ----------------------------------------------------

    function testBoostStartsAtOneXForMinimumLock() public view {
        assertEq(locker.boostFor(WEEK), 10000, "minimum lock gives 1x");
    }

    function testBoostReachesFourXAtMaximumLock() public view {
        assertEq(locker.boostFor(MAX_LOCK), 40000, "maximum lock gives 4x");
    }

    function testBoostIsMonotonicInDuration() public view {
        uint256 previous = 0;
        for (uint256 i = 0; i <= 48; i++) {
            uint256 boost = locker.boostFor(WEEK + (i * (MAX_LOCK - WEEK)) / 48);
            assertGe(boost, previous, "boost never decreases with duration");
            previous = boost;
        }
    }

    function testBoostInterpolatesMidway() public view {
        uint256 mid = (MAX_LOCK + WEEK) / 2;
        uint256 boost = locker.boostFor(mid);
        // Halfway between 1x and 4x is 2.5x.
        assertApproxEqRel(boost, 25000, 0.02e18, "midpoint ~= 2.5x");
    }

    // ---- Locking -----------------------------------------------------------

    function testLockTransfersLpAndRecordsBoost() public {
        vm.prank(alice);
        uint256 id = locker.lock(100e18, MAX_LOCK);

        assertEq(id, 0, "first lock id");
        assertEq(lp.balanceOf(address(locker)), 100e18, "locker holds LP");
        assertEq(locker.lockedBalance(alice), 100e18, "locked balance tracked");
        assertEq(locker.totalLocked(), 100e18, "total tracked");

        TigerSwapLocker.Lock memory l = locker.getLock(alice, id);
        assertEq(l.amount, 100e18, "amount recorded");
        assertEq(l.boostBps, 40000, "boost recorded");
    }

    function testCannotLockBelowMinimum() public {
        vm.prank(alice);
        vm.expectRevert("lock too short");
        locker.lock(100e18, WEEK - 1);
    }

    function testCannotLockAboveMaximum() public {
        vm.prank(alice);
        vm.expectRevert("lock too long");
        locker.lock(100e18, MAX_LOCK + WEEK);
    }

    function testCannotLockZero() public {
        vm.prank(alice);
        vm.expectRevert("amount required");
        locker.lock(0, MAX_LOCK);
    }

    function testMultipleLocksCoexistIndependently() public {
        vm.startPrank(alice);
        uint256 shortId = locker.lock(100e18, WEEK);
        uint256 longId = locker.lock(200e18, MAX_LOCK);
        vm.stopPrank();

        assertEq(locker.lockCount(alice), 2, "two locks");
        assertEq(locker.getLock(alice, shortId).boostBps, 10000, "short lock 1x");
        assertEq(locker.getLock(alice, longId).boostBps, 40000, "long lock 4x");
        assertEq(locker.lockedBalance(alice), 300e18, "both counted");
    }

    // ---- Withdraw ----------------------------------------------------------

    function testWithdrawAfterExpiryReturnsLp() public {
        vm.prank(alice);
        uint256 id = locker.lock(100e18, WEEK);

        vm.warp(block.timestamp + WEEK + 1);
        vm.prank(alice);
        uint256 returned = locker.withdraw(id);

        assertEq(returned, 100e18, "full amount returned");
        assertEq(lp.balanceOf(alice), 1000e18, "LP restored");
        assertEq(locker.totalLocked(), 0, "total cleared");
    }

    function testWithdrawBeforeExpiryReverts() public {
        vm.prank(alice);
        uint256 id = locker.lock(100e18, MAX_LOCK);

        vm.prank(alice);
        vm.expectRevert("lock not expired");
        locker.withdraw(id);
    }

    function testCannotWithdrawSameLockTwice() public {
        vm.prank(alice);
        uint256 id = locker.lock(100e18, WEEK);
        vm.warp(block.timestamp + WEEK + 1);

        vm.prank(alice);
        locker.withdraw(id);

        vm.prank(alice);
        vm.expectRevert("already withdrawn");
        locker.withdraw(id);
    }

    function testCannotWithdrawUnknownLock() public {
        vm.prank(alice);
        vm.expectRevert("unknown lock");
        locker.withdraw(0);
    }

    // ---- Rewards -----------------------------------------------------------

    function testLongerLockEarnsMoreRewards() public {
        vm.prank(alice);
        uint256 shortId = locker.lock(100e18, WEEK);
        vm.prank(bob);
        uint256 longId = locker.lock(100e18, MAX_LOCK);

        tgr.mint(address(this), 1000e18);
        tgr.approve(address(locker), type(uint256).max);
        locker.notifyReward(1000e18);

        uint256 shortReward = locker.pendingReward(alice, shortId);
        uint256 longReward = locker.pendingReward(bob, longId);

        assertGt(shortReward, 0, "short lock earns something");
        assertGt(longReward, 0, "long lock earns something");
        assertApproxEqRel(longReward, shortReward * 4, 0.01e18, "4x boost earns 4x rewards");
    }

    function testRewardsScaleWithAmount() public {
        vm.prank(alice);
        uint256 aId = locker.lock(100e18, MAX_LOCK);
        vm.prank(bob);
        uint256 bId = locker.lock(200e18, MAX_LOCK);

        tgr.mint(address(this), 900e18);
        tgr.approve(address(locker), type(uint256).max);
        locker.notifyReward(900e18);

        assertApproxEqRel(locker.pendingReward(bob, bId), locker.pendingReward(alice, aId) * 2, 0.01e18, "double amount, double reward");
    }

    function testClaimRewardsPaysOutAndZeroesPending() public {
        vm.prank(alice);
        uint256 id = locker.lock(100e18, MAX_LOCK);

        tgr.mint(address(this), 500e18);
        tgr.approve(address(locker), type(uint256).max);
        locker.notifyReward(500e18);

        vm.prank(alice);
        uint256 claimed = locker.claimRewards(id);

        assertEq(claimed, 500e18, "sole locker receives the whole reward");
        assertEq(tgr.balanceOf(alice), 500e18, "tokens delivered");
        assertEq(locker.pendingReward(alice, id), 0, "nothing left pending");
    }

    function testClaimWithNothingPendingReverts() public {
        vm.prank(alice);
        uint256 id = locker.lock(100e18, MAX_LOCK);

        vm.prank(alice);
        vm.expectRevert("nothing to claim");
        locker.claimRewards(id);
    }

    function testCannotDoubleClaimSameReward() public {
        vm.prank(alice);
        uint256 id = locker.lock(100e18, MAX_LOCK);
        tgr.mint(address(this), 100e18);
        tgr.approve(address(locker), type(uint256).max);
        locker.notifyReward(100e18);

        vm.prank(alice);
        locker.claimRewards(id);

        vm.prank(alice);
        vm.expectRevert("nothing to claim");
        locker.claimRewards(id);
    }

    function testNotifyRewardRequiresLockedLiquidity() public {
        tgr.mint(address(this), 100e18);
        tgr.approve(address(locker), type(uint256).max);
        vm.expectRevert("no locked liquidity");
        locker.notifyReward(100e18);
    }

    function testOnlyOwnerCanNotifyReward() public {
        vm.prank(alice);
        locker.lock(100e18, MAX_LOCK);

        vm.prank(alice);
        vm.expectRevert("not owner");
        locker.notifyReward(1e18);
    }

    // ---- Migration ---------------------------------------------------------

    function testMigrateSendsLpToTarget() public {
        vm.prank(alice);
        uint256 id = locker.lock(100e18, WEEK);
        vm.warp(block.timestamp + WEEK + 1);

        address v3Manager = address(0x3A3A);
        vm.prank(alice);
        uint256 moved = locker.migrate(id, v3Manager);

        assertEq(moved, 100e18, "amount migrated");
        assertEq(lp.balanceOf(v3Manager), 100e18, "target received LP");
        assertEq(locker.lockedBalance(alice), 0, "position closed");
    }

    function testMigrateBeforeExpiryReverts() public {
        vm.prank(alice);
        uint256 id = locker.lock(100e18, MAX_LOCK);
        vm.prank(alice);
        vm.expectRevert("lock not expired");
        locker.migrate(id, address(0x3A3A));
    }

    function testMigrateRequiresTarget() public {
        vm.prank(alice);
        uint256 id = locker.lock(100e18, WEEK);
        vm.warp(block.timestamp + WEEK + 1);
        vm.prank(alice);
        vm.expectRevert("target required");
        locker.migrate(id, address(0));
    }

    // ---- Ownership ---------------------------------------------------------

    function testOwnershipTwoStep() public {
        locker.transferOwnership(bob);
        assertEq(locker.owner(), address(this), "not yet transferred");
        vm.prank(bob);
        locker.acceptOwnership();
        assertEq(locker.owner(), bob, "transferred");
    }

    function testOnlyPendingOwnerCanAccept() public {
        locker.transferOwnership(bob);
        vm.prank(alice);
        vm.expectRevert("not pending owner");
        locker.acceptOwnership();
    }
}
