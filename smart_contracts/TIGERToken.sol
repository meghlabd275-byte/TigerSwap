// SPDX-License-Identifier: MIT
pragma solidity ^0.8.19;

/**
 * @title TIGERToken
 * @dev TigerSwap Governance Token
 * 
 * Features:
 * - Governance voting
 * - Delegate votes
 * - Proposal creation
 * - Quorum requirements
 * - Time-lock before execution
 */

import "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import "@openzeppelin/contracts/token/ERC20/extensions/ERC20Burnable.sol";
import "@openzeppelin/contracts/access/AccessControl.sol";
import "@openzeppelin/contracts/utils/ReentrancyGuard.sol";

/**
 * @dev Governor configuration
 */
contract TIGERToken is ERC20, ERC20Burnable, AccessControl, ReentrancyGuard {
    
    bytes32 public constant MINTER_ROLE = keccak256("MINTER_ROLE");
    bytes32 public constant GOVERNOR_ROLE = keccak256("GOVERNOR_ROLE");
    
    // Token configuration
    uint256 public constant MAX_SUPPLY = 1_000_000_000 * 1e18; // 1B tokens
    
    // Delegation
    mapping(address => address) public delegates;
    mapping(address => uint256) public checkpointedVotes;
    mapping(address => mapping(uint256 => Checkpoint)) public checkpoints;
    mapping(address => uint256) public numCheckpoints;
    
    struct Checkpoint {
        uint256 fromBlock;
        uint256 votes;
    }
    
    // Events
    event DelegateChanged(address indexed delegator, address indexed fromDelegate, address indexed toDelegate);
    event DelegateVotesChanged(address indexed delegate, uint256 previousVotes, uint256 newVotes);
    event MinterChanged(address indexed minter, bool status);
    
    constructor() ERC20("TigerSwap", "TIGER") {
        _grantRole(DEFAULT_ADMIN_ROLE, msg.sender);
        _grantRole(MINTER_ROLE, msg.sender);
        _grantRole(GOVERNOR_ROLE, msg.sender);
    }
    
    // ==================== MINTING ====================
    
    function mint(address to, uint256 amount) external onlyRole(MINTER_ROLE) {
        require(totalSupply() + amount <= MAX_SUPPLY, "Max supply exceeded");
        _mint(to, amount);
    }
    
    function mintBatch(address[] calldata recipients, uint256[] calldata amounts) external onlyRole(MINTER_ROLE) {
        require(recipients.length == amounts.length, "Length mismatch");
        
        uint256 total;
        for (uint256 i = 0; i < amounts.length; i++) {
            total += amounts[i];
        }
        require(totalSupply() + total <= MAX_SUPPLY, "Max supply exceeded");
        
        for (uint256 i = 0; i < recipients.length; i++) {
            _mint(recipients[i], amounts[i]);
        }
    }
    
    // ==================== DELEGATION ====================
    
    function delegate(address delegatee) external {
        address currentDelegate = delegates[msg.sender];
        
        if (delegatee != address(0) && delegatee != msg.sender) {
            // Delegate to someone
            uint256 amount = balanceOf(msg.sender);
            _moveDelegates(currentDelegate, delegatee, amount);
            delegates[msg.sender] = delegatee;
            
            emit DelegateChanged(msg.sender, currentDelegate, delegatee);
        } else if (delegatee == address(0)) {
            // Undelegate
            uint256 amount = balanceOf(msg.sender);
            _moveDelegates(currentDelegate, address(0), amount);
            delegates[msg.sender] = address(0);
            
            emit DelegateChanged(msg.sender, currentDelegate, address(0));
        }
    }
    
    function getVotes(address account) external view returns (uint256) {
        return checkpointedVotes[account];
    }
    
    function getPastVotes(address account, uint256 blockNumber) external view returns (uint256) {
        require(blockNumber < block.number, "Not determined yet");
        
        uint256 nCheckpoints = numCheckpoints[account];
        if (nCheckpoints == 0) return 0;
        
        // First checkpoint
        Checkpoint memory first = checkpoints[account][0];
        if (first.fromBlock > blockNumber) return 0;
        
        // Last checkpoint
        if (nCheckpoints == 1) return first.votes;
        
        Checkpoint memory last = checkpoints[account][nCheckpoints - 1];
        if (last.fromBlock <= blockNumber) return last.votes;
        
        // Binary search
        uint256 low = 0;
        uint256 high = nCheckpoints - 1;
        
        while (low < high) {
            uint256 mid = (low + high + 1) / 2;
            Checkpoint memory cp = checkpoints[account][mid];
            if (cp.fromBlock <= blockNumber) {
                low = mid;
            } else {
                high = mid - 1;
            }
        }
        
        return checkpoints[account][low].votes;
    }
    
    function _moveDelegates(address src, address dst, uint256 amount) internal {
        if (src != dst && amount > 0) {
            if (src != address(0)) {
                uint256 srcVotes = checkpointedVotes[src];
                uint256 newSrcVotes = srcVotes - amount;
                checkpointedVotes[src] = newSrcVotes;
                
                emit DelegateVotesChanged(src, srcVotes, newSrcVotes);
            }
            
            if (dst != address(0)) {
                uint256 dstVotes = checkpointedVotes[dst];
                uint256 newDstVotes = dstVotes + amount;
                checkpointedVotes[dst] = newDstVotes;
                
                emit DelegateVotesChanged(dst, dstVotes, newDstVotes);
            }
        }
    }
    
    function _afterTokenTransfer(address from, address to, uint256 amount) internal override {
        super._afterTokenTransfer(from, to, amount);
        
        // Auto-delegate on transfer
        if (from != address(0)) {
            address fromDelegate = delegates[from];
            if (fromDelegate != address(0)) {
                _moveDelegates(fromDelegate, fromDelegate, amount);
            }
        }
        
        if (to != address(0)) {
            address toDelegate = delegates[to];
            if (toDelegate != address(0)) {
                _moveDelegates(toDelegate, toDelegate, amount);
            }
        }
    }
    
    // ==================== PROPOSALS ====================
    
    struct Proposal {
        uint256 id;
        address proposer;
        address[] targets;
        uint256[] values;
        bytes[] signatures;
        bytes[] calldatas;
        uint256 startBlock;
        uint256 endBlock;
        string description;
        bool executed;
        bool cancelled;
        uint256 forVotes;
        uint256 againstVotes;
        uint256 abstainVotes;
    }
    
    mapping(uint256 => Proposal) public proposals;
    uint256 public proposalCount;
    uint256 public constant PROPOSAL_THRESHOLD = 100_000e18; // 100K tokens to propose
    uint256 public constant VOTING_DELAY = 1 blocks;
    uint256 public constant VOTING_PERIOD = 12960 blocks; // ~2 days
    uint256 public constant EXECUTION_DELAY = 2 days;
    uint256 public constant PROPOSAL_MAX_OPERATIONS = 10;
    
    // Timelock
    address public timelock;
    
    event ProposalCreated(uint256 indexed id, address proposer, string description);
    event ProposalExecuted(uint256 indexed id);
    event ProposalCancelled(uint256 indexed id);
    event VoteCast(address indexed voter, uint256 indexed proposalId, bool support, uint256 weight);
    
    modifier onlyTimelock() {
        require(msg.sender == timelock, "Not timelock");
        _;
    }
    
    function setTimelock(address _timelock) external onlyRole(DEFAULT_ADMIN_ROLE) {
        timelock = _timelock;
    }
    
    function propose(
        address[] calldata targets,
        uint256[] calldata values,
        bytes[] calldata signatures,
        bytes[] calldata calldatas,
        string calldata description
    ) external returns (uint256) {
        // Check proposer has enough votes
        require(getVotes(msg.sender) >= PROPOSAL_THRESHOLD, "Below threshold");
        
        // Check targets length
        require(targets.length == values.length, "Length mismatch");
        require(targets.length == signatures.length, "Length mismatch");
        require(targets.length == calldatas.length, "Length mismatch");
        require(targets.length > 0, "No targets");
        require(targets.length <= PROPOSAL_MAX_OPERATIONS, "Too many targets");
        
        uint256 proposalId = proposalCount;
        proposalCount++;
        
        Proposal storage proposal = proposals[proposalId];
        proposal.id = proposalId;
        proposal.proposer = msg.sender;
        proposal.targets = targets;
        proposal.values = values;
        proposal.signatures = signatures;
        proposal.calldatas = calldatas;
        proposal.startBlock = block.number + VOTING_DELAY;
        proposal.endBlock = block.number + VOTING_DELAY + VOTING_PERIOD;
        proposal.description = description;
        
        emit ProposalCreated(proposalId, msg.sender, description);
        
        return proposalId;
    }
    
    function execute(uint256 proposalId) external payable onlyTimelock returns (bytes[] memory) {
        Proposal storage proposal = proposals[proposalId];
        
        require(!proposal.executed, "Already executed");
        require(!proposal.cancelled, "Cancelled");
        require(block.number >= proposal.endBlock, "Not ended");
        
        // Check quorum
        uint256 totalVotes = proposal.forVotes + proposal.againstVotes + proposal.abstainVotes;
        uint256 quorum = totalSupply() / 10; // 10% quorum
        require(totalVotes >= quorum, "Below quorum");
        require(proposal.forVotes > proposal.againstVotes, "Not passed");
        
        proposal.executed = true;
        
        // Execute
        bytes[] memory results = new bytes[](proposal.targets.length);
        
        for (uint256 i = 0; i < proposal.targets.length; i++) {
            (bool success, bytes memory result) = proposal.targets[i].call{value: proposal.values[i]}(
                proposal.calldatas[i]
            );
            require(success, "Call failed");
            results[i] = result;
        }
        
        emit ProposalExecuted(proposalId);
        
        return results;
    }
    
    function cancel(uint256 proposalId) external {
        Proposal storage proposal = proposals[proposalId];
        require(msg.sender == proposal.proposer, "Not proposer");
        
        proposal.cancelled = true;
        emit ProposalCancelled(proposalId);
    }
    
    function castVote(uint256 proposalId, bool support) external {
        Proposal storage proposal = proposals[proposalId];
        require(!proposal.executed, "Already executed");
        require(!proposal.cancelled, "Cancelled");
        require(block.number >= proposal.startBlock, "Not started");
        require(block.number < proposal.endBlock, "Ended");
        
        uint256 weight = getVotes(msg.sender);
        
        if (support) {
            proposal.forVotes += weight;
        } else {
            proposal.againstVotes += weight;
        }
        
        emit VoteCast(msg.sender, proposalId, support, weight);
    }
    
    function getProposal(uint256 proposalId) external view returns (Proposal memory) {
        return proposals[proposalId];
    }
    
    // ==================== ADMIN ====================
    
    function grantMinterRole(address minter) external onlyRole(DEFAULT_ADMIN_ROLE) {
        _grantRole(MINTER_ROLE, minter);
        emit MinterChanged(minter, true);
    }
    
    function revokeMinterRole(address minter) external onlyRole(DEFAULT_ADMIN_ROLE) {
        _revokeRole(MINTER_ROLE, minter);
        emit MinterChanged(minter, false);
    }
}

// ==================== VETOKEN (VOTE ESCROW) ====================

/**
 * @title TIGERVotingEscrow
 * @dev Vote-escrowed token for governance
 */

contract TIGERVotingEscrow is AccessControl, ReentrancyGuard {
    
    bytes32 public constant ADMIN_ROLE = keccak256("ADMIN_ROLE");
    
    struct LockedBalance {
        uint256 amount;
        uint256 end;
    }
    
    struct Point {
        int128 bias;
        int128 slope;
        uint256 ts;
    }
    
    mapping(address => LockedBalance) public locked;
    mapping(address => mapping(uint256 => Point)) public pointHistory;
    mapping(address => uint256) public pointEpoch;
    mapping(address => address) public delegation;
    
    uint256 public totalSupply;
    uint256 public constant MAXTIME = 4 * 365 * 86400; // 4 years
    uint256 public constant MULTIPLIER = 1;
    
    address public tigerToken;
    uint256 public epoch;
    Point[] public pointHistoryGlobal;
    
    // Slopes
    mapping(address => mapping(uint256 => int128)) public slopeChanges;
    
    event Deposit(address indexed user, uint256 amount, uint256 locktime);
    event Withdraw(address indexed user);
    event Delegate(address indexed from, address indexed to);
    
    constructor(address _tigerToken) {
        tigerToken = _tigerToken;
        _grantRole(DEFAULT_ADMIN_ROLE, msg.sender);
        
        // Initial point
        pointHistoryGlobal.push(Point({
            bias: 0,
            slope: 0,
            ts: block.timestamp
        }));
    }
    
    function createLock(uint256 amount, uint256 lockDuration) external nonReentrant {
        require(lockDuration <= MAXTIME, "Too long");
        require(lockDuration >= 1 weeks, "Too short");
        
        address user = msg.sender;
        
        // Transfer TIGER to contract
        IERC20(tigerToken).transferFrom(user, address(this), amount);
        
        LockedBalance storage lock = locked[user];
        
        if (lock.amount > 0) {
            // Extend existing lock
            require(lock.end > block.timestamp, "Expired");
            lock.amount += amount;
            lock.end = max(lock.end, block.timestamp + lockDuration);
            
            _checkpoint(user, lock.amount, lock.end);
        } else {
            // New lock
            lock.amount = amount;
            lock.end = block.timestamp + lockDuration;
            
            _checkpoint(user, amount, lock.end);
        }
        
        totalSupply += amount;
        
        emit Deposit(user, amount, lock.end);
    }
    
    function increaseAmount(uint256 amount) external nonReentrant {
        address user = msg.sender;
        LockedBalance storage lock = locked[user];
        
        require(lock.amount > 0, "No lock");
        require(lock.end > block.timestamp, "Expired");
        
        IERC20(tigerToken).transferFrom(user, address(this), amount);
        lock.amount += amount;
        totalSupply += amount;
        
        _checkpoint(user, lock.amount, lock.end);
        
        emit Deposit(user, lock.amount, lock.end);
    }
    
    function increaseUnlockTime(uint256 newDuration) external nonReentrant {
        address user = msg.sender;
        LockedBalance storage lock = locked[user];
        
        require(lock.amount > 0, "No lock");
        require(lock.end > block.timestamp, "Expired");
        require(newDuration <= MAXTIME, "Too long");
        require(newDuration > lock.end - block.timestamp, "Cannot reduce");
        
        lock.end = block.timestamp + newDuration;
        
        _checkpoint(user, lock.amount, lock.end);
        
        emit Deposit(user, lock.amount, lock.end);
    }
    
    function withdraw() external nonReentrant {
        address user = msg.sender;
        LockedBalance storage lock = locked[user];
        
        require(lock.amount > 0, "No lock");
        require(lock.end <= block.timestamp, "Locked");
        
        uint256 amount = lock.amount;
        
        // Clear lock
        lock.amount = 0;
        lock.end = 0;
        
        _checkpoint(user, 0, 0);
        totalSupply -= amount;
        
        // Transfer back
        IERC20(tigerToken).transfer(user, amount);
        
        emit Withdraw(user);
    }
    
    function delegateVote(address delegatee) external {
        require(locked[msg.sender].amount > 0, "No lock");
        
        delegation[msg.sender] = delegatee;
        
        emit Delegate(msg.sender, delegatee);
    }
    
    function getVotes(address account) external view returns (uint256) {
        LockedBalance storage lock = locked[account];
        if (lock.amount == 0) return 0;
        
        uint256 bias = lock.amount * (lock.end - block.timestamp) / MAXTIME;
        return bias;
    }
    
    function _checkpoint(address user, uint256 amount, uint256 end) internal {
        uint256 dt = end - block.timestamp;
        int128 slope = int128(amount * dt / MAXTIME);
        
        uint256 epoch = pointEpoch[user];
        Point memory lastPoint = pointHistoryGlobal[epoch];
        
        // Update global history
        if (block.timestamp > lastPoint.ts) {
            pointHistoryGlobal.push(Point({
                bias: lastPoint.bias + int128(amount) * int128(dt) / int128(MAXTIME),
                slope: lastPoint.slope + slope,
                ts: block.timestamp
            }));
            epoch++;
        }
        
        pointEpoch[user] = epoch;
    }
    
    function max(uint256 a, uint256 b) internal pure returns (uint256) {
        return a >= b ? a : b;
    }
}

// ==================== GAUGE VOTING ====================

/**
 * @title TIGERGaugeVoting
 * @dev Gauge voting for liquidity incentives
 */

contract TIGERGaugeVoting is AccessControl {
    
    bytes32 public constant ADMIN_ROLE = keccak256("ADMIN_ROLE");
    
    struct Gauge {
        address pool;
        uint256 weight;
        uint256 rewardRate;
        bool active;
    }
    
    mapping(address => Gauge) public gauges;
    address[] public gaugeList;
    
    mapping(address => mapping(address => uint256)) public votes;
    mapping(address => uint256) public voteWeights;
    mapping(address => uint256) public usedWeights;
    
    uint256 public totalWeight;
    
    event GaugeAdded(address indexed gauge, uint256 weight);
    event GaugeUpdated(address indexed gauge, uint256 weight);
    event VoteCast(address indexed voter, address indexed gauge, uint256 weight);
    
    constructor() {
        _grantRole(DEFAULT_ADMIN_ROLE, msg.sender);
    }
    
    function addGauge(address pool, uint256 weight, uint256 rewardRate) external onlyRole(ADMIN_ROLE) {
        require(!gauges[pool].active, "Exists");
        
        gauges[pool] = Gauge({
            pool: pool,
            weight: weight,
            rewardRate: rewardRate,
            active: true
        });
        
        gaugeList.push(pool);
        
        emit GaugeAdded(pool, weight);
    }
    
    function updateGaugeWeight(address pool, uint256 weight) external onlyRole(ADMIN_ROLE) {
        require(gauges[pool].active, "Not found");
        
        gauges[pool].weight = weight;
        
        emit GaugeUpdated(pool, weight);
    }
    
    function vote(address gauge, uint256 weight) external {
        require(gauges[gauge].active, "Invalid gauge");
        
        address voter = msg.sender;
        
        // Remove old votes
        if (votes[voter][gauge] > 0) {
            voteWeights[voter] -= votes[voter][gauge];
            totalWeight -= votes[voter][gauge];
        }
        
        // Add new votes
        votes[voter][gauge] = weight;
        voteWeights[voter] += weight;
        totalWeight += weight;
        
        emit VoteCast(voter, gauge, weight);
    }
    
    function getGaugeWeight(address gauge) external view returns (uint256) {
        if (totalWeight == 0) return 0;
        
        uint256 voteWeight = voteWeights[gauges[gauge].pool];
        return (voteWeight * gauges[gauge].weight) / totalWeight;
    }
    
    function getGaugeList() external view returns (address[] memory) {
        return gaugeList;
    }
}

// ==================== FEE DISTRIBUTION ====================

/**
 * @title TIGERFeeDistributor
 * @dev Distributes trading fees toveTIGER holders
 */

contract TIGERFeeDistributor is AccessControl, ReentrancyGuard {
    
    bytes32 public constant ADMIN_ROLE = keccak256("ADMIN_ROLE");
    
    address public token;
    uint256 public totalDistributed;
    uint256 public currentPeriod;
    
    mapping(uint256 => uint256) public periodReward;
    mapping(uint256 => mapping(address => uint256)) public claimable;
    
    event RewardClaimed(address indexed user, uint256 amount);
    
    constructor(address _token) {
        token = _token;
        _grantRole(DEFAULT_ADMIN_ROLE, msg.sender);
    }
    
    function notifyReward(uint256 amount) external onlyRole(ADMIN_ROLE) {
        require(IERC20(token).transferFrom(msg.sender, address(this), amount), "Transfer failed");
        
        currentPeriod++;
        periodReward[currentPeriod] = amount;
    }
    
    function claim(uint256 period) external nonReentrant {
        uint256 reward = claimable[period][msg.sender];
        require(reward > 0, "Nothing to claim");
        
        claimable[period][msg.sender] = 0;
        totalDistributed += reward;
        
        IERC20(token).transfer(msg.sender, reward);
        
        emit RewardClaimed(msg.sender, reward);
    }
}