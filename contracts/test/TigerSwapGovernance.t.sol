// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "forge-std/Test.sol";
import "../TigerSwapGovernance.sol";
import "./TestERC20.sol";

contract TigerSwapGovernanceTest is Test {
    TigerSwapGovernance public gov;
    TestERC20 public tgr;

    address public alice = address(0xA11CE);
    address public bob = address(0xB0B);
    address public carol = address(0xCA401);

    function setUp() public {
        tgr = new TestERC20("TGR", "TGR");
        gov = new TigerSwapGovernance(address(tgr));

        tgr.mint(alice, 1_000_000e18);
        tgr.mint(bob, 1_000_000e18);
        tgr.mint(carol, 1_000_000e18);
    }

    /// @dev Create a proposal and advance into the active voting window.
    function _activeProposal() internal returns (uint256 id) {
        address[] memory targets = new address[](1);
        targets[0] = address(tgr);
        uint256[] memory values = new uint256[](1);
        string[] memory signatures = new string[](1);
        signatures[0] = "transfer(address,uint256)";
        bytes[] memory calldatas = new bytes[](1);
        calldatas[0] = abi.encode(alice, uint256(1));

        vm.prank(alice);
        id = gov.propose(targets, values, signatures, calldatas, "Test proposal");

        // votingDelay is in seconds and converted to blocks at 12s each.
        vm.roll(block.number + (gov.votingDelay() / 12) + 1);
    }

    function testProposalIsCreated() public {
        uint256 id = _activeProposal();
        assertEq(id, 0, "first proposal id");

        TigerSwapGovernance.Proposal memory p = gov.getProposal(id);
        assertEq(p.proposer, alice, "proposer");
        assertEq(p.forVotes, 0, "no votes yet");
    }

    function testVoteIsCountedOnce() public {
        uint256 id = _activeProposal();

        vm.prank(alice);
        gov.castVote(id, true);

        TigerSwapGovernance.Proposal memory p = gov.getProposal(id);
        assertEq(p.forVotes, 1_000_000e18, "weight counted");
        assertTrue(gov.hasVoted(id, alice), "recorded as voted");
        assertEq(gov.voteAmount(id, alice), 1_000_000e18, "weight recorded");
    }

    function testCannotVoteTwice() public {
        uint256 id = _activeProposal();

        vm.prank(alice);
        gov.castVote(id, true);

        // A second vote would otherwise inflate the tally arbitrarily.
        vm.prank(alice);
        vm.expectRevert("TigerSwap: ALREADY_VOTED");
        gov.castVote(id, true);

        TigerSwapGovernance.Proposal memory p = gov.getProposal(id);
        assertEq(p.forVotes, 1_000_000e18, "tally unchanged by the second attempt");
    }

    function testCannotVoteTwiceWithReason() public {
        uint256 id = _activeProposal();

        vm.prank(alice);
        gov.castVoteWithReason(id, true, "first");

        vm.prank(alice);
        vm.expectRevert("TigerSwap: ALREADY_VOTED");
        gov.castVoteWithReason(id, false, "second");
    }

    function testCannotVoteBothWays() public {
        uint256 id = _activeProposal();

        vm.prank(alice);
        gov.castVote(id, true);

        vm.prank(alice);
        vm.expectRevert("TigerSwap: ALREADY_VOTED");
        gov.castVote(id, false);

        TigerSwapGovernance.Proposal memory p = gov.getProposal(id);
        assertEq(p.againstVotes, 0, "no opposing vote recorded");
    }

    function testVotesFromDifferentHoldersAccumulate() public {
        uint256 id = _activeProposal();

        vm.prank(alice);
        gov.castVote(id, true);
        vm.prank(bob);
        gov.castVote(id, true);

        TigerSwapGovernance.Proposal memory p = gov.getProposal(id);
        assertEq(p.forVotes, 2_000_000e18, "both counted");
    }

    function testOpposingVotesAreSeparate() public {
        uint256 id = _activeProposal();

        vm.prank(alice);
        gov.castVote(id, true);
        vm.prank(bob);
        gov.castVote(id, false);

        TigerSwapGovernance.Proposal memory p = gov.getProposal(id);
        assertEq(p.forVotes, 1_000_000e18, "for");
        assertEq(p.againstVotes, 1_000_000e18, "against");
    }

    function testVoteRequiresVotingPower() public {
        uint256 id = _activeProposal();

        address nobody = address(0xD00D);
        vm.prank(nobody);
        vm.expectRevert("TigerSwap: NO_VOTING_POWER");
        gov.castVote(id, true);
    }

    function testProposalSucceedsWithMoreForVotes() public {
        uint256 id = _activeProposal();

        vm.prank(alice);
        gov.castVote(id, true);

        vm.roll(block.number + gov.votingPeriod() / 12 + 1);
        assertEq(uint256(gov.state(id)), uint256(TigerSwapGovernance.ProposalState.Succeeded), "succeeded");
    }

    function testProposalIsDefeatedWithMoreAgainstVotes() public {
        uint256 id = _activeProposal();

        vm.prank(alice);
        gov.castVote(id, false);

        vm.roll(block.number + gov.votingPeriod() / 12 + 1);
        assertEq(uint256(gov.state(id)), uint256(TigerSwapGovernance.ProposalState.Defeated), "defeated");
    }

    function testProposalIsDefeatedOnATie() public {
        uint256 id = _activeProposal();

        vm.prank(alice);
        gov.castVote(id, true);
        vm.prank(bob);
        gov.castVote(id, false);

        vm.roll(block.number + gov.votingPeriod() / 12 + 1);
        assertEq(uint256(gov.state(id)), uint256(TigerSwapGovernance.ProposalState.Defeated), "tie is not a win");
    }

    function testVotingIsClosedAfterThePeriod() public {
        uint256 id = _activeProposal();

        vm.roll(block.number + gov.votingPeriod() / 12 + 2);
        vm.prank(alice);
        vm.expectRevert("TigerSwap: NOT_ACTIVE");
        gov.castVote(id, true);
    }

    function testVotingIsClosedBeforeItOpens() public {
        address[] memory targets = new address[](1);
        targets[0] = address(tgr);
        uint256[] memory values = new uint256[](1);
        string[] memory signatures = new string[](1);
        signatures[0] = "";
        bytes[] memory calldatas = new bytes[](1);
        calldatas[0] = "";

        vm.prank(alice);
        uint256 id = gov.propose(targets, values, signatures, calldatas, "Pending");

        // Still inside the delay window, so the proposal is Pending.
        assertEq(uint256(gov.state(id)), uint256(TigerSwapGovernance.ProposalState.Pending), "pending");

        vm.prank(alice);
        vm.expectRevert("TigerSwap: NOT_ACTIVE");
        gov.castVote(id, true);
    }

    function testVotesCannotBeReplayedAfterTheWindowCloses() public {
        uint256 id = _activeProposal();

        vm.prank(alice);
        gov.castVote(id, true);

        vm.roll(block.number + gov.votingPeriod() / 12 + 1);

        // The window is closed, so the recorded vote cannot be used again.
        vm.prank(alice);
        vm.expectRevert("TigerSwap: NOT_ACTIVE");
        gov.castVote(id, true);
    }

    // ---- Admin -------------------------------------------------------------

    function testOnlyOwnerCanChangeParameters() public {
        vm.prank(alice);
        vm.expectRevert("TigerSwap: NOT_OWNER");
        gov.setVotingDelay(100);

        vm.prank(alice);
        vm.expectRevert("TigerSwap: NOT_OWNER");
        gov.setVotingPeriod(100);

        vm.prank(alice);
        vm.expectRevert("TigerSwap: NOT_OWNER");
        gov.setProposalThreshold(100);
    }

    function testOwnerCanChangeParameters() public {
        gov.setVotingDelay(1000);
        assertEq(gov.votingDelay(), 1000, "voting delay");

        gov.setVotingPeriod(2000);
        assertEq(gov.votingPeriod(), 2000, "voting period");

        gov.setProposalThreshold(500);
        assertEq(gov.proposalThreshold(), 500, "threshold");
    }

    function testTransferOwnershipRejectsZeroAddress() public {
        vm.expectRevert("TigerSwap: ZERO_ADDRESS");
        gov.transferOwnership(address(0));
    }

    function testTransferOwnershipMovesAdmin() public {
        gov.transferOwnership(bob);
        assertEq(gov.owner(), bob, "owner moved");

        vm.prank(bob);
        gov.setVotingDelay(1234);
        assertEq(gov.votingDelay(), 1234, "new owner can administer");
    }
}
