// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "forge-std/Test.sol";
import "../TigerSwapStaking.sol";
import "./TestERC20.sol";

contract TigerSwapStakingTest is Test {
    TigerSwapStaking public staking;
    TestERC20 public tgr;
    TestERC20 public reward;

    address public alice = address(0xA11CE);
    address public bob = address(0xB0B);
    address public stranger = address(0x5BAD);

    uint256 constant DURATION = 30 days;
    uint256 constant TOTAL_REWARD = 30_000e18;

    function setUp() public {
        tgr = new TestERC20("TGR", "TGR");
        reward = new TestERC20("RYUSD", "RYUSD");

        staking = new TigerSwapStaking(address(tgr), address(reward));

        tgr.mint(alice, 1_000_000e18);
        tgr.mint(bob, 1_000_000e18);

        vm.prank(alice);
        tgr.approve(address(staking), type(uint256).max);
        vm.prank(bob);
        tgr.approve(address(staking), type(uint256).max);

        // Fund the contract with the reward and start a campaign.
        reward.mint(address(this), TOTAL_REWARD * 10);
        reward.approve(address(staking), type(uint256).max);

        staking.setRewardsDuration(DURATION);
    }

    function _startRewards(uint256 amount) internal {
        reward.transfer(address(staking), amount);
        staking.notifyRewardAmount(amount);
    }

    // ---- Access control ----------------------------------------------------

    function testOnlyOwnerCanNotifyReward() public {
        reward.transfer(address(staking), TOTAL_REWARD);

        vm.prank(stranger);
        vm.expectRevert("TigerSwap: NOT_OWNER");
        staking.notifyRewardAmount(TOTAL_REWARD);
    }

    function testOnlyOwnerCanSetDuration() public {
        vm.prank(stranger);
        vm.expectRevert("TigerSwap: NOT_OWNER");
        staking.setRewardsDuration(7 days);
    }

    function testConstructorRejectsZeroAddresses() public {
        vm.expectRevert("TigerSwap: ZERO_ADDRESS");
        new TigerSwapStaking(address(0), address(reward));

        vm.expectRevert("TigerSwap: ZERO_ADDRESS");
        new TigerSwapStaking(address(tgr), address(0));
    }

    function testTwoStepOwnershipTransfer() public {
        staking.commitTransferOwnership(stranger);
        assertEq(staking.futureOwner(), stranger, "pending owner");

        vm.prank(stranger);
        staking.acceptTransferOwnership();
        assertEq(staking.owner(), stranger, "ownership moved");

        // The old owner can no longer administer.
        vm.expectRevert("TigerSwap: NOT_OWNER");
        staking.setRewardsDuration(1 days);
    }

    function testOnlyFutureOwnerCanAccept() public {
        staking.commitTransferOwnership(stranger);
        vm.prank(bob);
        vm.expectRevert("TigerSwap: NOT_FUTURE_OWNER");
        staking.acceptTransferOwnership();
    }

    function testNotifyRewardRejectsZero() public {
        vm.expectRevert("TigerSwap: ZERO_REWARD");
        staking.notifyRewardAmount(0);
    }

    function testNotifyRewardRequiresDuration() public {
        TigerSwapStaking fresh = new TigerSwapStaking(address(tgr), address(reward));
        reward.transfer(address(fresh), TOTAL_REWARD);

        vm.expectRevert("TigerSwap: DURATION_NOT_SET");
        fresh.notifyRewardAmount(TOTAL_REWARD);
    }

    function testNotifyRewardRejectsUnbackedAmount() public {
        // Only 1,000 in the contract but 30,000 requested.
        reward.transfer(address(staking), 1_000e18);
        vm.expectRevert("TigerSwap: REWARD_TOO_HIGH");
        staking.notifyRewardAmount(TOTAL_REWARD);
    }

    // ---- Staking -----------------------------------------------------------

    function testStakeMovesTokens() public {
        vm.prank(alice);
        staking.stake(100_000e18);

        assertEq(staking.balanceOf(alice), 100_000e18, "staked balance");
        assertEq(staking.totalSupply(), 100_000e18, "total staked");
        assertEq(tgr.balanceOf(address(staking)), 100_000e18, "tokens held");
    }

    function testStakeRejectsZero() public {
        vm.prank(alice);
        vm.expectRevert("TigerSwap: CANNOT_STAKE_ZERO");
        staking.stake(0);
    }

    function testWithdrawRejectsOverdraft() public {
        vm.prank(alice);
        staking.stake(1_000e18);

        vm.prank(alice);
        vm.expectRevert("TigerSwap: INSUFFICIENT_BALANCE");
        staking.withdraw(1_001e18);
    }

    function testWithdrawRejectsZero() public {
        vm.prank(alice);
        vm.expectRevert("TigerSwap: CANNOT_WITHDRAW_ZERO");
        staking.withdraw(0);
    }

    function testWithdrawReturnsTokens() public {
        vm.prank(alice);
        staking.stake(100_000e18);

        uint256 before = tgr.balanceOf(alice);
        vm.prank(alice);
        staking.withdraw(100_000e18);

        assertEq(tgr.balanceOf(alice), before + 100_000e18, "returned");
        assertEq(staking.totalSupply(), 0, "total cleared");
    }

    // ---- Rewards -----------------------------------------------------------

    function testSoleStakerEarnsTheWholeReward() public {
        _startRewards(TOTAL_REWARD);

        vm.prank(alice);
        staking.stake(100_000e18);

        vm.warp(block.timestamp + DURATION);
        assertApproxEqRel(staking.earned(alice), TOTAL_REWARD, 0.001e18, "alice earns it all");
    }

    function testTwoStakersSplitRewardsByWeight() public {
        _startRewards(TOTAL_REWARD);

        vm.prank(alice);
        staking.stake(100_000e18);
        vm.prank(bob);
        staking.stake(300_000e18);

        vm.warp(block.timestamp + DURATION);

        uint256 aliceEarned = staking.earned(alice);
        uint256 bobEarned = staking.earned(bob);

        // Bob staked three times as much, so he earns three times as much.
        assertApproxEqRel(bobEarned, aliceEarned * 3, 0.01e18, "weighted by stake");
        assertApproxEqRel(aliceEarned + bobEarned, TOTAL_REWARD, 0.01e18, "reward fully allocated");
    }

    function testLateStakerEarnsProportionallyLess() public {
        _startRewards(TOTAL_REWARD);

        vm.prank(alice);
        staking.stake(100_000e18);

        // Half the campaign elapses before bob joins.
        vm.warp(block.timestamp + DURATION / 2);
        vm.prank(bob);
        staking.stake(100_000e18);

        vm.warp(block.timestamp + DURATION / 2);

        // Alice was alone for half and shared for half, so she beats bob.
        assertGt(staking.earned(alice), staking.earned(bob), "early staker earns more");
    }

    function testGetRewardTransfersAndClears() public {
        _startRewards(TOTAL_REWARD);

        vm.prank(alice);
        staking.stake(100_000e18);
        vm.warp(block.timestamp + DURATION);

        uint256 expected = staking.earned(alice);
        uint256 before = reward.balanceOf(alice);

        vm.prank(alice);
        staking.getReward();

        assertEq(reward.balanceOf(alice), before + expected, "reward paid");
        assertEq(staking.earned(alice), 0, "cleared");
    }

    function testGetRewardRejectsWhenNothingEarned() public {
        vm.prank(stranger);
        vm.expectRevert("TigerSwap: NO_REWARD");
        staking.getReward();
    }

    function testRewardsDoNotAccrueAfterTheCampaignEnds() public {
        _startRewards(TOTAL_REWARD);

        vm.prank(alice);
        staking.stake(100_000e18);

        vm.warp(block.timestamp + DURATION);
        uint256 atEnd = staking.earned(alice);

        vm.warp(block.timestamp + 365 days);
        assertEq(staking.earned(alice), atEnd, "no rewards past the end");
    }

    function testSecondCampaignExtendsTheFirst() public {
        _startRewards(TOTAL_REWARD);

        vm.prank(alice);
        staking.stake(100_000e18);

        // A second campaign partway through must not discard unclaimed rewards.
        vm.warp(block.timestamp + DURATION / 2);
        _startRewards(TOTAL_REWARD);

        vm.warp(block.timestamp + DURATION);

        // Alice was the only staker throughout, so she earns both campaigns.
        assertApproxEqRel(staking.earned(alice), TOTAL_REWARD * 2, 0.05e18, "both campaigns");
    }

    function testStakingRewardsAreNotPaidTwice() public {
        _startRewards(TOTAL_REWARD);

        vm.prank(alice);
        staking.stake(100_000e18);
        vm.warp(block.timestamp + DURATION);

        vm.prank(alice);
        staking.getReward();
        uint256 first = reward.balanceOf(alice);

        // A second claim must revert rather than pay again.
        vm.prank(alice);
        vm.expectRevert("TigerSwap: NO_REWARD");
        staking.getReward();
        assertEq(reward.balanceOf(alice), first, "no double payment");
    }

    function testWithdrawDoesNotForfeitRewards() public {
        _startRewards(TOTAL_REWARD);

        vm.prank(alice);
        staking.stake(100_000e18);
        vm.warp(block.timestamp + DURATION / 2);

        vm.prank(alice);
        staking.withdraw(100_000e18);

        // Rewards accrued while staked must remain claimable.
        assertGt(staking.earned(alice), 0, "rewards kept after withdrawing");

        uint256 before = reward.balanceOf(alice);
        vm.prank(alice);
        staking.getReward();
        assertGt(reward.balanceOf(alice), before, "still claimable");
    }

    function testRewardRateIsSetWhenACampaignStarts() public {
        assertEq(staking.rewardRate(), 0, "no campaign yet");

        _startRewards(TOTAL_REWARD);

        // The whole reward is spread evenly across the duration.
        assertEq(staking.rewardRate(), TOTAL_REWARD / DURATION, "rate spread over the duration");
    }
}
