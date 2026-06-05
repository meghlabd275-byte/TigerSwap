// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/**
 * @title TigerDAO
 * @notice Governance contract for TigerSwap protocol
 */
contract TigerDAO {
    struct Proposal {
        address proposer;
        uint256 startTime;
        uint256 endTime;
        uint256 forVotes;
        uint256 againstVotes;
        bool executed;
        string description;
        bytes32 hash;
    }

    uint256 public constant VOTING_PERIOD = 3 days;
    uint256 public constant PROPOSAL_THRESHOLD = 1000000e18;
    uint256 public proposalCount;
    uint256 public constant QUORUM_THRESHOLD = 400; // 4%

    mapping(address => uint256) public votingPower;
    mapping(address => bool) public isDelegator;
    mapping(address => address) public delegates;
    mapping(uint256 => Proposal) public proposals;
    mapping(uint256 => mapping(address => bool)) public hasVoted;
    mapping(uint256 => mapping(address => uint256)) public voteAmount;
    mapping(address => bool) public isAdmin;

    event ProposalCreated(uint256 indexed id, address proposer, string description);
    event VoteCast(address indexed voter, uint256 indexed proposalId, bool support, uint256 weight);
    event ProposalExecuted(uint256 indexed proposalId);
    event VotingPowerUpdated(address indexed user, uint256 newPower);
    event DelegateChanged(address indexed delegator, address indexed fromDelegate, address indexed toDelegate);

    constructor() {
        isAdmin[msg.sender] = true;
    }

    function propose(string memory description, bytes32 hash) external returns (uint256) {
        require(votingPower[msg.sender] >= PROPOSAL_THRESHOLD, "TigerDAO: BELOW_THRESHOLD");
        
        uint256 proposalId = proposalCount++;
        uint256 startTime = block.timestamp;
        uint256 endTime = startTime + VOTING_PERIOD;

        proposals[proposalId] = Proposal({
            proposer: msg.sender,
            startTime: startTime,
            endTime: endTime,
            forVotes: 0,
            againstVotes: 0,
            executed: false,
            description: description,
            hash: hash
        });

        emit ProposalCreated(proposalId, msg.sender, description);
        return proposalId;
    }

    function castVote(uint256 proposalId, bool support) external {
        require(!hasVoted[proposalId][msg.sender], "TigerDAO: ALREADY_VOTED");
        require(block.timestamp < proposals[proposalId].endTime, "TigerDAO: VOTING_ENDED");
        
        uint256 weight = votingPower[msg.sender];
        require(weight > 0, "TigerDAO: NO_VOTING_POWER");

        hasVoted[proposalId][msg.sender] = true;
        voteAmount[proposalId][msg.sender] = weight;

        if (support) {
            proposals[proposalId].forVotes += weight;
        } else {
            proposals[proposalId].againstVotes += weight;
        }

        emit VoteCast(msg.sender, proposalId, support, weight);
    }

    function executeProposal(uint256 proposalId) external {
        Proposal storage proposal = proposals[proposalId];
        require(!proposal.executed, "TigerDAO: ALREADY_EXECUTED");
        require(block.timestamp >= proposal.endTime, "TigerDAO: VOTING_NOT_ENDED");
        
        uint256 totalVotes = proposal.forVotes + proposal.againstVotes;
        uint256 quorum = (totalVotes * 10000) / votingPower[address(0)]; // Total supply
        
        require(quorum >= QUORUM_THRESHOLD, "TigerDAO: NO_QUORUM");
        require(proposal.forVotes > proposal.againstVotes, "TigerDAO: NOT_PASSED");

        proposal.executed = true;
        emit ProposalExecuted(proposalId);
    }

    function delegate(address delegatee) external {
        address currentDelegate = delegates[msg.sender];
        delegates[msg.sender] = delegatee;
        emit DelegateChanged(msg.sender, currentDelegate, delegatee);
    }

    function updateVotingPower(address user, uint256 newPower) external {
        require(isAdmin[msg.sender], "TigerDAO: FORBIDDEN");
        votingPower[user] = newPower;
        emit VotingPowerUpdated(user, newPower);
    }

    function getProposal(uint256 proposalId) external view returns (Proposal memory) {
        return proposals[proposalId];
    }

    function hasVotedOnProposal(address user, uint256 proposalId) external view returns (bool) {
        return hasVoted[proposalId][user];
    }
}