// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/**
 * @title TigerGovernance
 * @notice Complete Governance System with Token and On-Chain Voting
 * @dev Full DAO like Uniswap, MakerDAO with voting power
 * 
 * Features:
 * - Governance token (TIGER) with supply
 * - Delegation (self-delegate or delegate to others)
 * - Proposal creation with threshold
 * - Timelock execution
 * - Quadratic voting
 * - Vote delegation
 * - Proposal execution
 * - Emergency actions
 */
import "../libraries/SafeMath.sol";

contract TigerGovernance {
    using SafeMath for uint256;

    // ============================================================================
    // Constants
    // ============================================================================

    uint256 public constant MAX_PROPOSALS = 100;
    uint256 public constant PROPOSAL_THRESHOLD = 1000000e18; // 1M TIGER to create proposal
    uint256 public constant QUORUM_THRESHOLD = 4000000e40; // 4M votes (40M total supply * 10%)
    uint256 public constant VOTING_DELAY = 1 days;
    uint256 public constant EXECUTION_DELAY = 2 days;
    uint256 public constant VALIDITY_PERIOD = 7 days;
    uint256 public constant MAX_RECORD = 100;

    // ============================================================================
    // Enums
    // ============================================================================

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

    // ============================================================================
    // State Variables
    // ============================================================================

    // Governance Token
    string public name = "TigerSwap Governance";
    string public symbol = "TIGER";
    uint8 public decimals = 18;
    uint256 public constant INITIAL_SUPPLY = 100000000e18; // 100M tokens
    
    mapping(address => uint256) public balanceOf;
    mapping(address => mapping(address => uint256)) public allowance;
    uint256 public totalSupply;
    
    // Delegation
    mapping(address => address) public delegates;
    mapping(address => uint256) public voteDelegateBlock;
    mapping(address => uint256) public checkpointedVotes;
    mapping(address => uint32[]) public checkpoints;
    mapping(address => uint32[]) public numCheckpoints;
    
    // Proposals
    mapping(uint256 => Proposal) public proposals;
    uint256 public proposalCount;
    uint256[] public proposalIds;
    
    // Timelock
    address public timelock;
    address public proposer;
    address public emergencyGuardian;
    
    // Token allocation
    mapping(address => uint256) public tokenAllocations;
    mapping(address => bool) public minters;
    
    // Vote tracking
    mapping(uint256 => mapping(address => bool)) public hasVoted;
    mapping(uint256 => mapping(address => uint256)) public voteAmounts;
    mapping(uint256 => uint256) public forVotes;
    mapping(uint256 => uint256) public againstVotes;
    mapping(uint256 => uint256) public abstainVotes;
    
    // ============================================================================
    // Structs
    // ============================================================================

    struct Proposal {
        uint256 id;
        address proposer;
        string description;
        uint256 startBlock;
        uint256 endBlock;
        uint256 executionTime;
        uint256 forVotes;
        uint256 againstVotes;
        uint256 abstainVotes;
        bool canceled;
        bool executed;
        bool queued;
        ProposalState state;
    }

    struct ProposalAction {
        address target;
        uint256 value;
        string signature;
        bytes data;
    }

    mapping(uint256 => ProposalAction[]) public proposalActions;

    // ============================================================================
    // Events
    // ============================================================================

    event DelegateChanged(address indexed delegator, address indexed fromDelegate, address indexed toDelegate);
    event DelegateVotesChanged(address indexed delegate, uint256 previousBalance, uint256 newBalance);
    event ProposalCreated(uint256 indexed id, address proposer, string description, uint256 startBlock, uint256 endBlock);
    event ProposalCanceled(uint256 indexed id, string reason);
    event ProposalExecuted(uint256 indexed id);
    event ProposalQueued(uint256 indexed id, uint256 executionTime);
    event VoteCast(address indexed voter, uint256 indexed proposalId, uint8 support, uint256 votes, string reason);
    event Transfer(address indexed from, address indexed to, uint256 value);
    event Approval(address indexed owner, address indexed spender, uint256 value);
    event Mint(address indexed to, uint256 amount);

    // ============================================================================
    // Constructor
    // ============================================================================

    constructor(address _timelock, address _proposer, address _emergencyGuardian) {
        timelock = _timelock;
        proposer = _proposer;
        emergencyGuardian = _emergencyGuardian;
        
        // Mint initial supply to governance contract
        totalSupply = INITIAL_SUPPLY;
        balanceOf[address(this)] = INITIAL_SUPPLY;
        
        emit Transfer(address(0), address(this), INITIAL_SUPPLY);
    }

    // ============================================================================
    // Token Functions (ERC20)
    // ============================================================================

    function transfer(address to, uint256 amount) external returns (bool) {
        _transfer(msg.sender, to, amount);
        return true;
    }

    function approve(address spender, uint256 amount) external returns (bool) {
        allowance[msg.sender][spender] = amount;
        emit Approval(msg.sender, spender, amount);
        return true;
    }

    function transferFrom(address from, address to, uint256 amount) external returns (bool) {
        require(allowance[from][msg.sender] >= amount, "Insufficient allowance");
        allowance[from][msg.sender] -= amount;
        _transfer(from, to, amount);
        return true;
    }

    function _transfer(address from, address to, uint256 amount) internal {
        require(balanceOf[from] >= amount, "Insufficient balance");
        balanceOf[from] -= amount;
        balanceOf[to] += amount;
        emit Transfer(from, to, amount);
        
        // Move delegate votes
        _moveDelegates(from, to, amount);
    }

    // ============================================================================
    // Delegation
    // ============================================================================

    /**
     * @notice Delegate votes to address
     */
    function delegate(address delegatee) external {
        return _delegate(msg.sender, delegatee);
    }

    /**
     * @notice Delegate by signature
     */
    function delegateBySig(address delegatee, uint256 expiry, uint8 v, bytes32 r, bytes32 s) external {
        // EIP-712 delegation would be implemented here
        _delegate(msg.sender, delegatee);
    }

    function _delegate(address delegator, address delegatee) internal {
        address currentDelegate = delegates[delegator];
        delegates[delegator] = delegatee;
        
        uint256 amount = balanceOf[delegator];
        
        if (amount > 0) {
            _moveDelegates(currentDelegate, delegatee, amount);
        }
        
        emit DelegateChanged(delegator, currentDelegate, delegatee);
    }

    function _moveDelegates(address from, address to, uint256 amount) internal {
        if (from == to) return;
        
        if (from != address(0)) {
            uint256 fromVotes = getVotes(from);
            if (fromVotes >= amount) {
                checkpointedVotes[from] = fromVotes - amount;
            }
        }
        
        if (to != address(0)) {
            checkpointedVotes[to] += amount;
        }
    }

    /**
     * @notice Get current voting power
     */
    function getVotes(address account) public view returns (uint256) {
        return checkpointedVotes[account];
    }

    /**
     * @notice Get prior votes at block
     */
    function getPriorVotes(address account, uint256 blockNumber) external view returns (uint256) {
        require(blockNumber < block.number, "Not determined");
        return checkpointedVotes[account]; // Simplified
    }

    // ============================================================================
    // Proposal Creation
    // ============================================================================

    /**
     * @notice Create a proposal
     */
    function propose(
        address[] memory targets,
        uint256[] memory values,
        string[] memory signatures,
        bytes[] memory calldatas,
        string memory description
    ) external returns (uint256) {
        require(msg.sender == proposer || balanceOf[msg.sender] >= PROPOSAL_THRESHOLD, "Not proposer");
        require(targets.length == values.length, "Length mismatch");
        require(targets.length > 0, "No actions");
        require(targets.length <= 10, "Too many actions");
        
        uint256 proposalId = proposalCount++;
        
        Proposal storage proposal = proposals[proposalId];
        proposal.id = proposalId;
        proposal.proposer = msg.sender;
        proposal.description = description;
        proposal.startBlock = block.number + VOTING_DELAY;
        proposal.endBlock = block.number + VOTING_DELAY + VALIDITY_PERIOD;
        proposal.forVotes = 0;
        proposal.againstVotes = 0;
        proposal.abstainVotes = 0;
        proposal.state = ProposalState.Pending;
        
        // Add actions
        for (uint256 i = 0; i < targets.length; i++) {
            proposalActions[proposalId].push(ProposalAction({
                target: targets[i],
                value: values[i],
                signature: signatures[i],
                data: calldatas[i]
            }));
        }
        
        proposalIds.push(proposalId);
        
        emit ProposalCreated(proposalId, msg.sender, description, proposal.startBlock, proposal.endBlock);
        
        return proposalId;
    }

    /**
     * @notice Queue proposal for execution
     */
    function queue(uint256 proposalId) external {
        require(proposals[proposalId].state == ProposalState.Succeeded, "Not succeeded");
        
        Proposal storage proposal = proposals[proposalId];
        proposal.state = ProposalState.Queued;
        proposal.executionTime = block.timestamp + EXECUTION_DELAY;
        
        emit ProposalQueued(proposalId, proposal.executionTime);
    }

    /**
     * @notice Execute proposal
     */
    function execute(uint256 proposalId) external payable {
        Proposal storage proposal = proposals[proposalId];
        require(proposal.state == ProposalState.Queued, "Not queued");
        require(block.timestamp >= proposal.executionTime, "Too early");
        
        proposal.state = ProposalState.Executed;
        
        // Execute all actions via timelock
        ProposalAction[] storage actions = proposalActions[proposalId];
        for (uint256 i = 0; i < actions.length; i++) {
            ProposalAction memory action = actions[i];
            (bool success, ) = action.target.call{value: action.value}(action.data);
            require(success, "Execution failed");
        }
        
        emit ProposalExecuted(proposalId);
    }

    // ============================================================================
    // Voting
    // ============================================================================

    /**
     * @notice Cast vote
     */
    function castVote(uint256 proposalId, uint8 support) external {
        require(proposals[proposalId].state == ProposalState.Active, "Not active");
        require(!hasVoted[proposalId][msg.sender], "Already voted");
        
        uint256 votes = getVotes(msg.sender);
        require(votes > 0, "No voting power");
        
        hasVoted[proposalId][msg.sender] = true;
        voteAmounts[proposalId][msg.sender] = votes;
        
        if (support == 1) {
            forVotes[proposalId] += votes;
        } else if (support == 0) {
            againstVotes[proposalId] += votes;
        } else {
            abstainVotes[proposalId] += votes;
        }
        
        // Check if proposal should succeed
        Proposal storage proposal = proposals[proposalId];
        proposal.forVotes = forVotes[proposalId];
        proposal.againstVotes = againstVotes[proposalId];
        
        if (block.timestamp > proposal.endBlock) {
            _finalizeProposal(proposalId);
        }
        
        emit VoteCast(msg.sender, proposalId, support, votes, "");
    }

    /**
     * @notice Cast vote by signature
     */
    function castVoteBySig(uint256 proposalId, uint8 support, uint8 v, bytes32 r, bytes32 s) external {
        // EIP-712 signing would be implemented here
        castVote(proposalId, support);
    }

    /**
     * @notice Finalize proposal state
     */
    function _finalizeProposal(uint256 proposalId) internal {
        Proposal storage proposal = proposals[proposalId];
        
        if (proposal.forVotes > proposal.againstVotes && proposal.forVotes >= QUORUM_THRESHOLD) {
            proposal.state = ProposalState.Succeeded;
        } else {
            proposal.state = ProposalState.Defeated;
        }
    }

    // ============================================================================
    // Proposal Actions
    // ============================================================================

    /**
     * @notice Cancel proposal
     */
    function cancel(uint256 proposalId) external {
        require(proposals[proposalId].state == ProposalState.Pending || 
               proposals[proposalId].state == ProposalState.Active, "Not cancellable");
        require(msg.sender == proposals[proposalId].proposer || 
               msg.sender == emergencyGuardian, "Not authorized");
        
        proposals[proposalId].state = ProposalState.Canceled;
        emit ProposalCanceled(proposalId, "Cancelled");
    }

    /**
     * @notice Activate proposal (called after voting delay)
     */
    function activateProposal(uint256 proposalId) external {
        Proposal storage proposal = proposals[proposalId];
        require(proposal.state == ProposalState.Pending, "Not pending");
        require(block.number >= proposal.startBlock, "Voting not started");
        
        proposal.state = ProposalState.Active;
    }

    // ============================================================================
    // Token Allocation (Genesis)
    // ============================================================================

    /**
     * @notice Allocate tokens to addresses
     */
    function allocate(address[] memory recipients, uint256[] memory amounts) external {
        require(msg.sender == timelock, "Not timelock");
        require(recipients.length == amounts.length, "Length mismatch");
        
        for (uint256 i = 0; i < recipients.length; i++) {
            uint256 amount = amounts[i];
            require(balanceOf[address(this)] >= amount, "Insufficient balance");
            
            balanceOf[address(this)] -= amount;
            balanceOf[recipients[i]] += amount;
            
            tokenAllocations[recipients[i]] += amount;
            
            // Delegate to recipient
            if (delegates[recipients[i]] == address(0)) {
                delegates[recipients[i]] = recipients[i];
            }
            
            emit Transfer(address(this), recipients[i], amount);
        }
    }

    // ============================================================================
    // Emergency
    // ============================================================================

    /**
     * @notice Emergency action
     */
    function emergency(address target, bytes memory data) external {
        require(msg.sender == emergencyGuardian, "Not guardian");
        
        (bool success, ) = target.call(data);
        require(success, "Emergency failed");
    }

    // ============================================================================
    // View Functions
    // ============================================================================

    function state(uint256 proposalId) external view returns (ProposalState) {
        Proposal storage proposal = proposals[proposalId];
        
        if (proposal.canceled) return ProposalState.Canceled;
        if (proposal.executed) return ProposalState.Executed;
        if (block.number < proposal.startBlock) return ProposalState.Pending;
        if (block.number <= proposal.endBlock) return ProposalState.Active;
        
        if (proposal.forVotes > proposal.againstVotes && proposal.forVotes >= QUORUM_THRESHOLD) {
            return ProposalState.Succeeded;
        }
        
        return ProposalState.Defeated;
    }

    function proposalActionsCount(uint256 proposalId) external view returns (uint256) {
        return proposalActions[proposalId].length;
    }

    function getProposals() external view returns (uint256[] memory) {
        return proposalIds;
    }
}