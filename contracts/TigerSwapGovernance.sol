// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/**
 * @title TigerSwapGovernance
 * @notice DAO governance contract
 */
contract TigerSwapGovernance {
    // Token for voting power
    IERC20 public governanceToken;
    
    // Proposal states
    enum ProposalState {
        Pending,
        Active,
        Canceled,
        Defeated,
        Succeeded,
        Queued,
        Expired,
        Executed
    }
    
    // Proposal structure
    struct Proposal {
        uint256 id;
        address proposer;
        address[] targets;
        uint256[] values;
        string[] signatures;
        bytes[] calldatas;
        uint256 startBlock;
        uint256 endBlock;
        uint256 forVotes;
        uint256 againstVotes;
        bool canceled;
        bool executed;
        string description;
    }
    
    // Vote recording
    mapping(uint256 => mapping(address => bool)) public hasVoted;
    mapping(uint256 => mapping(address => uint256)) public voteAmount;
    
    // Proposal queue
    mapping(uint256 => uint256) public proposalTimestamps;
    
    // State
    Proposal[] public proposals;
    address public owner;
    uint256 public votingDelay;
    uint256 public votingPeriod;
    uint256 public proposalThreshold;
    uint256 public queuePeriod;
    uint256 public gracePeriod;
    uint256 public proposalCount;
    
    // Events
    event ProposalCreated(uint256 id, address proposer, address[] targets, uint256[] values, string description);
    event VoteCast(address indexed voter, uint256 proposalId, bool support, uint256 weight);
    event ProposalCanceled(uint256 id);
    event ProposalExecuted(uint256 id);
    event ProposalQueued(uint256 id, uint256 executionTime);

    modifier onlyOwner() {
        require(msg.sender == owner, "TigerSwap: NOT_OWNER");
        _;
    }

    constructor(address _governanceToken) {
        governanceToken = IERC20(_governanceToken);
        owner = msg.sender;
        votingDelay = 1 days;
        votingPeriod = 3 days;
        proposalThreshold = 1000000e18; // 1M tokens
        queuePeriod = 2 days;
        gracePeriod = 5 days;
    }

    /**
     * @notice Create a new proposal
     * @param targets Target contracts
     * @param values ETH values
     * @param signatures Function signatures
     * @param calldatas Calldata
     * @param description Proposal description
     */
    function propose(
        address[] memory targets,
        uint256[] memory values,
        string[] memory signatures,
        bytes[] memory calldatas,
        string memory description
    ) public returns (uint256) {
        require(governanceToken.balanceOf(msg.sender) >= proposalThreshold, "TigerSwap: TOKENS_REQUIRED");
        require(targets.length == values.length, "TigerSwap: MISMATCH");
        require(targets.length == signatures.length, "TigerSwap: MISMATCH");
        require(targets.length == calldatas.length, "TigerSwap: MISMATCH");
        require(targets.length > 0, "TigerSwap: NO_TARGETS");
        
        uint256 proposalId = proposalCount++;
        
        Proposal memory proposal = Proposal({
            id: proposalId,
            proposer: msg.sender,
            targets: targets,
            values: values,
            signatures: signatures,
            calldatas: calldatas,
            startBlock: block.number + (votingDelay / 12), // ~12 sec blocks
            endBlock: block.number + (votingDelay / 12) + (votingPeriod / 12),
            forVotes: 0,
            againstVotes: 0,
            canceled: false,
            executed: false,
            description: description
        });
        
        proposals.push(proposal);
        
        emit ProposalCreated(proposalId, msg.sender, targets, values, description);
        
        return proposalId;
    }

    /**
     * @notice Cast vote on proposal
     * @param proposalId Proposal ID
     * @param support Support or oppose
     */
    function castVote(uint256 proposalId, bool support) public {
        castVoteWithReason(proposalId, support, "");
    }

    /**
     * @notice Cast vote with reason
     * @param proposalId Proposal ID
     * @param support Support or oppose
     * @param reason Reason for vote
     */
    function castVoteWithReason(uint256 proposalId, bool support, string memory reason) public {
        Proposal storage proposal = proposals[proposalId];
        require(proposal.state() == ProposalState.Active, "TigerSwap: NOT_ACTIVE");
        
        uint256 weight = governanceToken.balanceOf(msg.sender);
        require(weight > 0, "TigerSwap: NO_VOTING_POWER");
        
        if (support) {
            proposal.forVotes += weight;
        } else {
            proposal.againstVotes += weight;
        }
        
        hasVoted[proposalId][msg.sender] = true;
        voteAmount[proposalId][msg.sender] = weight;
        
        emit VoteCast(msg.sender, proposalId, support, weight);
    }

    /**
     * @notice Queue successful proposal for execution
     * @param proposalId Proposal ID
     */
    function queueProposal(uint256 proposalId) public {
        Proposal storage proposal = proposals[proposalId];
        require(proposal.state() == ProposalState.Succeeded, "TigerSwap: NOT_SUCCEEDED");
        
        proposalTimestamps[proposalId] = block.timestamp + queuePeriod;
        
        emit ProposalQueued(proposalId, proposalTimestamps[proposalId]);
    }

    /**
     * @notice Execute proposal
     * @param proposalId Proposal ID
     */
    function executeProposal(uint256 proposalId) public payable {
        Proposal storage proposal = proposals[proposalId];
        require(proposal.state() == ProposalState.Queued, "TigerSwap: NOT_QUEUED");
        require(block.timestamp >= proposalTimestamps[proposalId], "TigerSwap: TOO_EARLY");
        require(block.timestamp <= proposalTimestamps[proposalId] + gracePeriod, "TigerSwap: EXPIRED");
        
        proposal.executed = true;
        
        for (uint256 i = 0; i < proposal.targets.length; i++) {
            (bool success, ) = proposal.targets[i].call{value: proposal.values[i]}(
                abi.encodePacked(abi.encodeWithSignature(proposal.signatures[i], proposal.calldatas[i]))
            );
            require(success, "TigerSwap: CALL_FAILED");
        }
        
        emit ProposalExecuted(proposalId);
    }

    /**
     * @notice Cancel proposal
     * @param proposalId Proposal ID
     */
    function cancelProposal(uint256 proposalId) public {
        Proposal storage proposal = proposals[proposalId];
        require(msg.sender == proposal.proposer || msg.sender == owner, "TigerSwap: NOT_AUTHORIZED");
        require(proposal.state() == ProposalState.Pending || proposal.state() == ProposalState.Active, "TigerSwap: CANNOT_CANCEL");
        
        proposal.canceled = true;
        
        emit ProposalCanceled(proposalId);
    }

    /**
     * @notice Get proposal state
     */
    function state(uint256 proposalId) public view returns (ProposalState) {
        Proposal storage proposal = proposals[proposalId];
        
        if (proposal.canceled) {
            return ProposalState.Canceled;
        }
        
        if (block.number <= proposal.startBlock) {
            return ProposalState.Pending;
        }
        
        if (block.number <= proposal.endBlock) {
            return ProposalState.Active;
        }
        
        if (proposal.forVotes <= proposal.againstVotes) {
            return ProposalState.Defeated;
        }
        
        if (proposalTimestamps[proposalId] == 0) {
            return ProposalState.Succeeded;
        }
        
        if (block.timestamp > proposalTimestamps[proposalId] + gracePeriod) {
            return ProposalState.Expired;
        }
        
        if (proposal.executed) {
            return ProposalState.Executed;
        }
        
        return ProposalState.Queued;
    }

    /**
     * @notice Get proposal count
     */
    function proposalCount() public view returns (uint256) {
        return proposals.length;
    }

    /**
     * @notice Get proposal by ID
     */
    function getProposal(uint256 proposalId) public view returns (Proposal memory) {
        return proposals[proposalId];
    }

    // Admin functions

    function setVotingDelay(uint256 _votingDelay) external onlyOwner {
        votingDelay = _votingDelay;
    }

    function setVotingPeriod(uint256 _votingPeriod) external onlyOwner {
        votingPeriod = _votingPeriod;
    }

    function setProposalThreshold(uint256 _proposalThreshold) external onlyOwner {
        proposalThreshold = _proposalThreshold;
    }

    function setQueuePeriod(uint256 _queuePeriod) external onlyOwner {
        queuePeriod = _queuePeriod;
    }

    function setGracePeriod(uint256 _gracePeriod) external onlyOwner {
        gracePeriod = _gracePeriod;
    }

    function transferOwnership(address newOwner) external onlyOwner {
        owner = newOwner;
    }
}

interface IERC20 {
    function balanceOf(address account) external view returns (uint256);
    function transfer(address to, uint256 value) external returns (bool);
    function transferFrom(address from, address to, uint256 value) external returns (bool);
}
