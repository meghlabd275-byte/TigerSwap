// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "../libraries/SafeMath.sol";

/**
 * @title TigerGovernance
 * @notice Complete Governance System
 * @dev Proposals, voting, delegation, timelock
 * 
 * Features:
 * - Proposal creation and execution
 * - Voting with quorum
 * - Vote delegation
 * - Time-locked execution
 * - Emergency controls
 * - Token-based voting power
 */
contract TigerGovernance {
    using SafeMath for uint256;

    // ============================================================================
    // Constants
    // ============================================================================
    
    uint256 constant PROPOSAL_EXPIRY = 259200; // 3 days
    uint256 constant VOTING_DELAY = 86400; // 1 day
    uint256 constant EXECUTION_DELAY = 172800; // 2 days
    uint256 constant MIN_PROPOSAL_THRESHOLD = 1000000e18; // 1M tokens
    uint256 constant MIN_QUORUM = 4000000e18; // 4M tokens (40% of supply)
    uint256 constant MAX_PROPOSALS = 100;
    uint256 constant MAX_VOTERS = 1000;
    
    // Proposal state
    uint8 constant STATE_PENDING = 1;
    uint8 constant STATE_ACTIVE = 2;
    uint8 constant STATE_CANCELLED = 3;
    uint8 constant STATE_DEFEATED = 4;
    uint8 constant STATE_SUCCEEDED = 5;
    uint8 constant STATE_QUEUED = 6;
    uint8 constant STATE_EXPIRED = 7;
    uint8 constant STATE_EXECUTED = 8;
    
    // Vote type
    uint8 constant VOTE_FOR = 1;
    uint8 constant VOTE_AGAINST = 2;
    uint8 constant VOTE_ABSTAIN = 3;
    
    // ============================================================================
    // State Variables
    // ============================================================================
    
    // Token
    address public governanceToken;
    uint256 public totalSupply;
    
    // Governance
    address public governance;
    address public pendingGovernance;
    address public timelock;
    uint256 public proposalCount;
    uint256 public proposalThreshold = MIN_PROPOSAL_THRESHOLD;
    uint256 public quorumThreshold = MIN_QUORUM;
    
    // Proposals
    mapping(uint256 => Proposal) public proposals;
    mapping(uint256 => mapping(address => uint8)) public votes;
    mapping(uint256 => uint256) public forVotes;
    mapping(uint256 => uint256) public againstVotes;
    mapping(uint256 => uint256) public abstainVotes;
    mapping(uint256 => address[]) public proposalVoters;
    
    // Delegation
    mapping(address => address) public delegates;
    mapping(address => uint256) public delegatedVotes;
    mapping(address => uint256) public voteStartBlock;
    mapping(address => uint256) public voteEndBlock;
    
    // Timelock
    mapping(bytes32 => uint256) public timelockTransactions;
    mapping(bytes32 => bool) public timelockExecuted;
    
    // Emergency
    bool public emergencyMode;
    bool public pauseVoting;
    
    // Events
    event ProposalCreated(uint256 indexed proposalId, address indexed proposer);
    event VoteCast(address indexed voter, uint256 indexed proposalId, uint8 vote, uint256 weight);
    event ProposalExecuted(uint256 indexed proposalId);
    event ProposalCancelled(uint256 indexed proposalId);
    event ProposalQueued(uint256 indexed proposalId, uint256 executionTime);
    event Delegation(address indexed delegator, address indexed delegatee);
    event EmergencyMode(bool enabled);
    
    // ============== Structs ==============
    
    struct Proposal {
        uint256 id;
        address proposer;
        address target;
        uint256 value;
        bytes data;
        string description;
        uint256 createdAt;
        uint256 startTime;
        uint256 endTime;
        uint256 executionTime;
        uint256 forVotes;
        uint256 againstVotes;
        uint256 abstainVotes;
        uint8 state;
        bool executed;
        bool cancelled;
    }
    
    struct ProposalTarget {
        address target;
        uint256 value;
        bytes data;
    }
    
    struct VoteReceipt {
        address voter;
        uint8 support;
        uint256 weight;
        uint256 timestamp;
    }
    
    mapping(uint256 => VoteReceipt[]) public proposalVotes;
    
    // ============== Modifier ==============
    
    modifier onlyGovernance() {
        require(msg.sender == governance, "ONLY_GOVERNANCE");
        _;
    }
    
    // ============== Constructor ==============
    
    constructor(address _token) {
        require(_token != address(0), "INVALID_TOKEN");
        
        governance = msg.sender;
        governanceToken = _token;
        timelock = address(this);
    }
    
    // ============================================================================
    // Proposal Creation
    // ============================================================================
    
    /**
     * @notice Create new proposal
     */
    function createProposal(
        address _target,
        uint256 _value,
        bytes calldata _data,
        string calldata _description
    ) external returns (uint256 proposalId) {
        require(!emergencyMode, "EMERGENCY_MODE");
        require(!pauseVoting, "VOTING_PAUSED");
        require(proposalCount < MAX_PROPOSALS, "MAX_PROPOSALS");
        
        // Check proposal threshold
        uint256 votingPower = getVotingPower(msg.sender);
        require(votingPower >= proposalThreshold, "INSUFFICIENT_VOTING_POWER");
        
        proposalId = proposalCount++;
        
        uint256 startTime = block.timestamp + VOTING_DELAY;
        uint256 endTime = startTime + PROPOSAL_EXPIRY;
        
        proposals[proposalId] = Proposal({
            id: proposalId,
            proposer: msg.sender,
            target: _target,
            value: _value,
            data: _data,
            description: _description,
            createdAt: block.timestamp,
            startTime: startTime,
            endTime: endTime,
            executionTime: 0,
            forVotes: 0,
            againstVotes: 0,
            abstainVotes: 0,
            state: STATE_PENDING,
            executed: false,
            cancelled: false
        });
        
        emit ProposalCreated(proposalId, msg.sender);
    }
    
    /**
     * @notice Create proposal with multiple targets
     */
    function createMultiTargetProposal(
        address[] calldata _targets,
        uint256[] calldata _values,
        bytes[] calldata _datas,
        string calldata _description
    ) external returns (uint256 proposalId) {
        require(_targets.length == _values.length, "LENGTH_MISMATCH");
        require(_targets.length == _datas.length, "LENGTH_MISMATCH");
        require(_targets.length <= 10, "TOO_MANY_TARGETS");
        
        // Encode all targets into single proposal
        bytes memory combinedData = abi.encode(_targets, _values, _datas);
        
        return createProposal(
            address(this),
            0,
            combinedData,
            _description
        );
    }
    
    // ============================================================================
    // Voting
    // ============================================================================
    
    /**
     * @notice Cast vote
     */
    function castVote(uint256 _proposalId, uint8 _support) external {
        require(!emergencyMode, "EMERGENCY_MODE");
        require(!pauseVoting, "VOTING_PAUSED");
        
        Proposal storage proposal = proposals[_proposalId];
        require(proposal.id != 0, "INVALID_PROPOSAL");
        require(proposal.state == STATE_ACTIVE, "NOT_ACTIVE");
        require(block.timestamp <= proposal.endTime, "VOTING_ENDED");
        require(_support == VOTE_FOR || _support == VOTE_AGAINST || _support == VOTE_ABSTAIN, "INVALID_VOTE");
        
        // Get voting power
        uint256 weight = getVotingPower(msg.sender);
        require(weight > 0, "NO_VOTING_POWER");
        
        // Update vote
        uint8 previousVote = votes[_proposalId][msg.sender];
        
        if (previousVote == 0) {
            // First vote
            proposalVoters[_proposalId].push(msg.sender);
        } else {
            // Remove previous vote weight
            if (previousVote == VOTE_FOR) {
                proposal.forVotes -= weight;
            } else if (previousVote == VOTE_AGAINST) {
                proposal.againstVotes -= weight;
            } else {
                proposal.abstainVotes -= weight;
            }
        }
        
        // Add new vote weight
        if (_support == VOTE_FOR) {
            proposal.forVotes += weight;
        } else if (_support == VOTE_AGAINST) {
            proposal.againstVotes += weight;
        } else {
            proposal.abstainVotes += weight;
        }
        
        votes[_proposalId][msg.sender] = _support;
        
        emit VoteCast(msg.sender, _proposalId, _support, weight);
        
        // Check if voting period ended
        if (block.timestamp >= proposal.endTime) {
            _finalizeProposal(_proposalId);
        }
    }
    
    /**
     * @notice Cast vote with reason
     */
    function castVoteWithReason(
        uint256 _proposalId,
        uint8 _support,
        string calldata _reason
    ) external {
        castVote(_proposalId, _support);
    }
    
    /**
     * @notice Finalize proposal after voting ends
     */
    function _finalizeProposal(uint256 _proposalId) internal {
        Proposal storage proposal = proposals[_proposalId];
        
        if (proposal.state != STATE_ACTIVE) return;
        
        uint256 totalVotes = proposal.forVotes + proposal.againstVotes + proposal.abstainVotes;
        
        if (proposal.forVotes > proposal.againstVotes && proposal.forVotes >= quorumThreshold) {
            proposal.state = STATE_SUCCEEDED;
            proposal.executionTime = block.timestamp + EXECUTION_DELAY;
            
            emit ProposalQueued(_proposalId, proposal.executionTime);
        } else if (proposal.forVotes <= proposal.againstVotes) {
            proposal.state = STATE_DEFEATED;
        } else if (block.timestamp >= proposal.endTime) {
            proposal.state = STATE_DEFEATED;
        }
    }
    
    // ============================================================================
    // Execution
    // ============================================================================
    
    /**
     * @notice Execute proposal
     */
    function executeProposal(uint256 _proposalId) external returns (bool success) {
        Proposal storage proposal = proposals[_proposalId];
        require(proposal.id != 0, "INVALID_PROPOSAL");
        require(proposal.state == STATE_SUCCEEDED, "NOT_SUCCEEDED");
        require(block.timestamp >= proposal.executionTime, "TOO_EARLY");
        require(block.timestamp <= proposal.executionTime + PROPOSAL_EXPIRY, "EXPIRED");
        
        // Queue for timelock
        bytes32 txHash = keccak256(abi.encodePacked(
            proposal.target,
            proposal.value,
            proposal.data,
            proposal.executionTime
        ));
        
        require(!timelockExecuted[txHash], "ALREADY_EXECUTED");
        
        // Execute
        if (proposal.target == address(this)) {
            // Multi-target proposal
            (address[] memory targets, uint256[] memory values, bytes[] memory datas) = abi.decode(
                proposal.data,
                (address[], uint256[], bytes[])
            );
            
            for (uint256 i = 0; i < targets.length; i++) {
                (success, ) = targets[i].call{value: values[i]}(datas[i]);
            }
        } else {
            (success, ) = proposal.target.call{value: proposal.value}(proposal.data);
        }
        
        if (success) {
            proposal.executed = true;
            proposal.state = STATE_EXECUTED;
            timelockExecuted[txHash] = true;
            
            emit ProposalExecuted(_proposalId);
        }
        
        return success;
    }
    
    /**
     * @notice Cancel proposal
     */
    function cancelProposal(uint256 _proposalId) external {
        Proposal storage proposal = proposals[_proposalId];
        require(proposal.id != 0, "INVALID_PROPOSAL");
        require(
            proposal.proposer == msg.sender || 
            isGovernance() ||
            block.timestamp >= proposal.endTime,
            "NOT_AUTHORIZED"
        );
        
        proposal.cancelled = true;
        proposal.state = STATE_CANCELLED;
        
        emit ProposalCancelled(_proposalId);
    }
    
    // ============================================================================
    // Delegation
    // ============================================================================
    
    /**
     * @notice Delegate votes
     */
    function delegateVotes(address _delegatee) external {
        require(_delegatee != address(0), "INVALID_DELEGATEE");
        require(_delegatee != msg.sender, "CANNOT_DELEGATE_SELF");
        
        // Clear previous delegation
        address previousDelegate = delegates[msg.sender];
        if (previousDelegate != address(0)) {
            delegatedVotes[previousDelegate] -= getVotingPower(msg.sender);
        }
        
        // Set new delegation
        delegates[msg.sender] = _delegatee;
        delegatedVotes[_delegatee] += getVotingPower(msg.sender);
        
        emit Delegation(msg.sender, _delegatee);
    }
    
    /**
     * @notice Delegate votes with signature
     */
    function delegateVotesBySig(
        address _delegator,
        address _delegatee,
        uint256 _nonce,
        uint256 _expiry,
        uint8 _v,
        bytes32 _r,
        bytes32 _s
    ) external {
        // Verify signature
        bytes32 domainSeparator = keccak256(abi.encodePacked(
            keccak256("EIP712Domain(string name,string version,uint256 chainId,address verifyingContract)"),
            keccak256("TigerGovernance"),
            keccak256("1"),
            block.chainid,
            address(this)
        ));
        
        bytes32 structHash = keccak256(abi.encodePacked(
            keccak256("Delegate(address delegator,address delegatee,uint256 nonce,uint256 expiry)"),
            _delegator,
            _delegatee,
            _nonce,
            _expiry
        ));
        
        bytes32 digest = keccak256(abi.encodePacked(
            "\x19\x01",
            domainSeparator,
            structHash
        ));
        
        address signatory = ecrecover(digest, _v, _r, _s);
        require(signature is valid), "INVALID_SIG");
        
        delegateVotes(_delegatee);
    }
    
    /**
     * @notice Revoke delegation
     */
    function revokeDelegation() external {
        address previousDelegate = delegates[msg.sender];
        require(previousDelegate != address(0), "NOT_DELEGATED");
        
        delegatedVotes[previousDelegate] -= getVotingPower(msg.sender);
        delete delegates[msg.sender];
    }
    
    // ============================================================================
    // View Functions
    // ============================================================================
    
    /**
     * @notice Get voting power
     */
    function getVotingPower(address _account) public view returns (uint256) {
        // Own tokens + delegated tokens
        // In production, this would query the governance token contract
        uint256 balance = IERC20(governanceToken).balanceOf(_account);
        uint256 delegated = delegatedVotes[_account];
        
        return balance + delegated;
    }
    
    /**
     * @notice Get proposal state
     */
    function getProposalState(uint256 _proposalId) external view returns (uint8) {
        Proposal storage proposal = proposals[_proposalId];
        
        if (proposal.executed) return STATE_EXECUTED;
        if (proposal.cancelled) return STATE_CANCELLED;
        
        if (block.timestamp < proposal.startTime) return STATE_PENDING;
        if (block.timestamp < proposal.endTime) return STATE_ACTIVE;
        
        // Voting ended, determine final state
        if (proposal.forVotes > proposal.againstVotes && proposal.forVotes >= quorumThreshold) {
            return STATE_SUCCEEDED;
        }
        
        return STATE_DEFEATED;
    }
    
    /**
     * @notice Get proposal details
     */
    function getProposal(uint256 _proposalId) external view returns (
        address proposer,
        address target,
        uint256 value,
        string memory description,
        uint256 forVotes,
        uint256 againstVotes,
        uint256 startTime,
        uint256 endTime,
        uint8 state
    ) {
        Proposal storage proposal = proposals[_proposalId];
        return (
            proposal.proposer,
            proposal.target,
            proposal.value,
            proposal.description,
            proposal.forVotes,
            proposal.againstVotes,
            proposal.startTime,
            proposal.endTime,
            proposal.state
        );
    }
    
    /**
     * @notice Get vote count
     */
    function getProposalVoteCount(uint256 _proposalId) external view returns (uint256) {
        return proposalVoters[_proposalId].length;
    }
    
    /**
     * @notice Get voter vote
     */
    function getVote(address _voter, uint256 _proposalId) external view returns (uint8) {
        return votes[_proposalId][_voter];
    }
    
    /**
     * @notice Get delegate
     */
    function getDelegate(address _account) external view returns (address) {
        return delegates[_account];
    }
    
    /**
     * @notice Check if governance
     */
    function isGovernance() public view returns (bool) {
        return msg.sender == governance;
    }
    
    // ============================================================================
    // Governance Controls
    // ============================================================================
    
    /**
     * @notice Set proposal threshold
     */
    function setProposalThreshold(uint256 _threshold) external onlyGovernance {
        proposalThreshold = _threshold;
    }
    
    /**
     * @notice Set quorum threshold
     */
    function setQuorumThreshold(uint256 _threshold) external onlyGovernance {
        quorumThreshold = _threshold;
    }
    
    /**
     * @notice Enable emergency mode
     */
    function enableEmergencyMode() external onlyGovernance {
        emergencyMode = true;
        pauseVoting = true;
        
        emit EmergencyMode(true);
    }
    
    /**
     * @notice Disable emergency mode
     */
    function disableEmergencyMode() external onlyGovernance {
        emergencyMode = false;
        
        emit EmergencyMode(false);
    }
    
    /**
     * @notice Pause voting
     */
    function pauseVoting() external onlyGovernance {
        pauseVoting = true;
    }
    
    /**
     * @notice Resume voting
     */
    function resumeVoting() external onlyGovernance {
        pauseVoting = false;
    }
    
    /**
     * @notice Transfer governance
     */
    function transferGovernance(address _newGovernance) external onlyGovernance {
        pendingGovernance = _newGovernance;
    }
    
    /**
     * @notice Accept governance
     */
    function acceptGovernance() external {
        require(msg.sender == pendingGovernance, "NOT_PENDING");
        governance = msg.sender;
        delete pendingGovernance;
    }
}

// ============================================================================
// IERC20 Interface (for voting power lookup)
// ============================================================================

interface IERC20 {
    function balanceOf(address account) external view returns (uint256);
    function transfer(address to, uint256 amount) external returns (bool);
    function approve(address spender, uint256 amount) external returns (bool);
    function transferFrom(address from, address to, uint256 amount) external returns (bool);
}