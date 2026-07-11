// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/**
 * @title TigerVeToken
 * @notice Production ve(3,3) Style Governance - Curve/Aerodrome Style
 * @dev Token locking mechanism for governance rights and fee distribution
 * 
 * Features:
 * - Time-weighted voting power
 * - Lock extension without withdrawing
 * - Vote delegation
 * - Governance proposal voting
 * - Fee distribution to lockers
 * - Quadratic voting
 * 
 * @author TigerSwap
 * @version 1.0.0 Production
 */

import "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";
import "@openzeppelin/contracts/utils/ReentrancyGuard.sol";
import "@openzeppelin/contracts/access/Ownable.sol";

/**
 * @title Decimal Math for precision
 */
library VeDecimalMath {
    uint256 constant WEEK = 7 days;
    uint256 constant MAX_LOCK_TIME = 4 * 365 days; // 4 years
    uint256 constant MULTIPLIER = 1e18;
    
    function mul(uint256 x, uint256 y) internal pure returns (uint256) {
        return (x * y) / MULTIPLIER;
    }
    
    function div(uint256 x, uint256 y) internal pure returns (uint256) {
        return (x * MULTIPLIER) / y;
    }
}

/**
 * @title TigerVeToken
 * @dev Main veToken contract for governance
 */
contract TigerVeToken is ReentrancyGuard, Ownable {
    using SafeERC20 for IERC20;
    using VeDecimalMath for uint256;

    // ============ Constants ============
    uint256 constant WEEK = 7 days;
    uint256 constant MAX_LOCK_TIME = 4 * 365 days;
    uint256 constant MULTIPLIER = 1e18;
    uint256 constant VOTE_MULTIPLIER = 2; // Quadratic voting

    // ============ State Variables ============
    
    // Token configuration
    IERC20 public token;                    // TIGER token
    uint256 public supply;                  // Total ve tokens
    uint256 public lockedSupply;            // Total locked
    uint256 public totalLocked;             // Total tokens locked
    
    // Time
    uint256 public epoch;
    uint256 public startTime;
    
    // Supply tracking
    uint256 public totalSupplyAtEpoch;
    uint256 public totalLockedAtEpoch;
    
    // Point history (for checkpointing)
    struct Point {
        uint256 bias;
        uint256 slope;
        uint256 timestamp;
        uint256 blk;
    }
    Point[] public pointHistory;
    
    // User lock info
    mapping(address => LockedBalance) public locked;
    
    // Epoch checkpoints
    mapping(uint256 => uint256) public supplyAtEpoch;
    mapping(uint256 => uint256) public lockedAtEpoch;
    
    // Slope changes
    mapping(uint256 => int256) public slopeChanges;
    
    // User checkpoint
    mapping(address => uint256) public userPointEpoch;
    mapping(address => mapping(uint256 => Point)) public userPointHistory;
    mapping(address => mapping(uint256 => uint256)) public userPointTs;
    
    // Voting
    mapping(address => mapping(uint256 => bool)) public voted;
    mapping(address => uint256) public voteLock;
    mapping(address => address) public delegates;
    
    // Governance
    uint256 public proposalCount;
    mapping(uint256 => Proposal) public proposals;
    mapping(uint256 => mapping(address => bool)) public hasVoted;
    mapping(uint256 => mapping(address => uint256)) public voteAmount;
    
    // Rewards
    uint256 public rewardRate;
    uint256 public rewardPerTokenStored;
    mapping(address => uint256) public userRewardPerTokenPaid;
    mapping(address => uint256) public rewards;
    
    // ============ Events ============
    event Deposit(address indexed user, uint256 value, uint256 lockTime, uint256 ts);
    event Withdraw(address indexed user, uint256 value, uint256 ts);
    event SupplyUpdate(uint256 prevSupply, uint256 newSupply);
    event Vote(address indexed user, uint256 proposalId, bool support, uint256 weight);
    event ProposalCreated(uint256 id, address proposer, string description, uint256 startTime, uint256 endTime);
    event ProposalExecuted(uint256 id);
    event ProposalCancelled(uint256 id);
    event RewardAdded(uint256 reward);
    event RewardPaid(address indexed user, uint256 reward);
    event DelegateChanged(address indexed delegator, address indexed fromDelegate, address indexed toDelegate);

    // ============ Structs ============
    struct LockedBalance {
        uint256 amount;
        uint256 end;
        uint256 start;
    }
    
    struct Proposal {
        address proposer;
        string description;
        uint256 startTime;
        uint256 endTime;
        uint256 forVotes;
        uint256 againstVotes;
        bool executed;
        bool cancelled;
        address[] targets;
        uint256[] values;
        bytes[] calldatas;
    }

    // ============ Constructor ============
    
    constructor(address _token, address _owner) Ownable(_owner) {
        require(_token != address(0), "Invalid token");
        
        token = IERC20(_token);
        startTime = block.timestamp;
        epoch = 1;
        
        // Initialize point history
        pointHistory.push(Point({
            bias: 0,
            slope: 0,
            timestamp: block.timestamp,
            blk: block.number
        }));
    }

    // ============ Lock Functions ============

    /**
     * @notice Create a lock
     * @param _value Amount of tokens to lock
     * @param _lockDuration Duration of lock in seconds
     */
    function createLock(uint256 _value, uint256 _lockDuration) external nonReentrant {
        require(_value > 0, "Cannot lock 0");
        require(_lockDuration >= WEEK, "Min lock is 1 week");
        require(_lockDuration <= MAX_LOCK_TIME, "Max lock is 4 years");
        
        LockedBalance memory locked_ = locked[msg.sender];
        
        require(locked_.amount == 0, "Already locked");
        
        // Calculate unlock time (rounded down to week)
        uint256 unlockTime = ((block.timestamp + _lockDuration) / WEEK) * WEEK;
        
        // Transfer tokens
        token.safeTransferFrom(msg.sender, address(this), _value);
        
        // Update locked balance
        locked_[msg.sender] = LockedBalance({
            amount: _value,
            end: unlockTime,
            start: block.timestamp
        });
        
        totalLocked += _value;
        _checkpoint(msg.sender, locked_, LockedBalance({amount: 0, end: 0, start: 0}));
        
        emit Deposit(msg.sender, _value, _lockDuration, block.timestamp);
    }

    /**
     * @notice Increase lock amount
     */
    function increaseLockAmount(uint256 _value) external nonReentrant {
        require(_value > 0, "Cannot add 0");
        
        LockedBalance memory locked_ = locked[msg.sender];
        require(locked_.amount > 0, "No lock found");
        require(locked_.end > block.timestamp, "Lock expired");
        
        token.safeTransferFrom(msg.sender, address(this), _value);
        
        uint256 newAmount = locked_.amount + _value;
        locked_[msg.sender].amount = newAmount;
        
        totalLocked += _value;
        
        _checkpoint(msg.sender, locked_, LockedBalance({amount: locked_.amount - _value, end: locked_.end, start: locked_.start}));
        
        emit Deposit(msg.sender, _value, locked_.end - block.timestamp, block.timestamp);
    }

    /**
     * @notice Extend lock time
     */
    function extendLock(uint256 _lockDuration) external nonReentrant {
        require(_lockDuration >= WEEK, "Min lock is 1 week");
        require(_lockDuration <= MAX_LOCK_TIME, "Max lock is 4 years");
        
        LockedBalance memory locked_ = locked[msg.sender];
        require(locked_.amount > 0, "No lock found");
        
        uint256 newEnd = ((block.timestamp + _lockDuration) / WEEK) * WEEK;
        require(newEnd > locked_.end, "Can only extend");
        
        locked_[msg.sender].end = newEnd;
        
        _checkpoint(msg.sender, locked_, LockedBalance({amount: locked_.amount, end: locked_.end, start: block.timestamp}));
        
        emit Deposit(msg.sender, locked_.amount, _lockDuration, block.timestamp);
    }

    /**
     * @notice Withdraw tokens after lock expires
     */
    function withdraw() external nonReentrant {
        LockedBalance memory locked_ = locked[msg.sender];
        require(locked_.amount > 0, "No lock found");
        require(locked_.end <= block.timestamp, "Lock not expired");
        
        uint256 amount = locked_.amount;
        
        // Reset locked balance
        locked_[msg.sender] = LockedBalance({amount: 0, end: 0, start: 0});
        
        // Update totals
        totalLocked -= amount;
        
        // Checkpoint
        _checkpoint(msg.sender, LockedBalance({amount: 0, end: 0, start: 0}), locked_);
        
        // Transfer tokens
        token.safeTransfer(msg.sender, amount);
        
        emit Withdraw(msg.sender, amount, block.timestamp);
    }

    // ============ Voting Functions ============

    /**
     * @notice Vote on a proposal
     */
    function vote(uint256 _proposalId, bool _support, uint256 _weight) external {
        require(_weight > 0, "Weight must be positive");
        require(_weight <= balanceOf(msg.sender), "Weight exceeds balance");
        
        Proposal storage proposal = proposals[_proposalId];
        require(proposal.startTime <= block.timestamp, "Proposal not started");
        require(block.timestamp <= proposal.endTime, "Proposal ended");
        
        // Check voting power
        uint256 votingPower = balanceOf(msg.sender);
        
        // Apply quadratic voting
        uint256 quadraticWeight = _sqrt(_weight) * _sqrt(votingPower);
        
        if (_support) {
            proposal.forVotes += quadraticWeight;
        } else {
            proposal.againstVotes += quadraticWeight;
        }
        
        hasVoted[_proposalId][msg.sender] = true;
        voteAmount[_proposalId][msg.sender] = _weight;
        
        emit Vote(msg.sender, _proposalId, _support, _weight);
    }

    /**
     * @notice Delegate voting power
     */
    function delegate(address _delegatee) external {
        address currentDelegate = delegates[msg.sender];
        require(currentDelegate != _delegatee, "Already delegated");
        
        delegates[msg.sender] = _delegatee;
        
        emit DelegateChanged(msg.sender, currentDelegate, _delegatee);
    }

    // ============ Governance Functions ============

    /**
     * @notice Create a proposal
     */
    function propose(
        string memory _description,
        address[] memory _targets,
        uint256[] memory _values,
        bytes[] memory _calldatas,
        uint256 _votingDuration
    ) external returns (uint256) {
        require(_targets.length == _values.length, "Length mismatch");
        require(_targets.length == _calldatas.length, "Length mismatch");
        
        // Check proposer has enough voting power
        require(balanceOf(msg.sender) >= 1000e18, "Insufficient voting power");
        
        uint256 proposalId = ++proposalCount;
        uint256 startTime = block.timestamp;
        uint256 endTime = startTime + (_votingDuration > 0 ? _votingDuration : 3 days);
        
        proposals[proposalId] = Proposal({
            proposer: msg.sender,
            description: _description,
            startTime: startTime,
            endTime: endTime,
            forVotes: 0,
            againstVotes: 0,
            executed: false,
            cancelled: false,
            targets: _targets,
            values: _values,
            calldatas: _calldatas
        });
        
        emit ProposalCreated(proposalId, msg.sender, _description, startTime, endTime);
        
        return proposalId;
    }

    /**
     * @notice Execute a proposal
     */
    function execute(uint256 _proposalId) external {
        Proposal storage proposal = proposals[_proposalId];
        require(!proposal.executed, "Already executed");
        require(!proposal.cancelled, "Cancelled");
        require(block.timestamp > proposal.endTime, "Voting not ended");
        
        // Check if passed (simple majority)
        require(proposal.forVotes > proposal.againstVotes, "Proposal failed");
        
        proposal.executed = true;
        
        // Execute proposals
        for (uint256 i = 0; i < proposal.targets.length; i++) {
            (bool success, ) = proposal.targets[i].call{value: proposal.values[i]}(
                proposal.calldatas[i]
            );
            require(success, "Execution failed");
        }
        
        emit ProposalExecuted(_proposalId);
    }

    /**
     * @notice Cancel a proposal
     */
    function cancel(uint256 _proposalId) external {
        Proposal storage proposal = proposals[_proposalId];
        require(!proposal.executed, "Already executed");
        require(!proposal.cancelled, "Already cancelled");
        require(proposal.proposer == msg.sender || balanceOf(msg.sender) >= 10000e18, "Not authorized");
        
        proposal.cancelled = true;
        
        emit ProposalCancelled(_proposalId);
    }

    // ============ Reward Functions ============

    /**
     * @notice Claim rewards
     */
    function getReward() external nonReentrant {
        _updateReward(msg.sender);
        
        uint256 reward = rewards[msg.sender];
        if (reward > 0) {
            rewards[msg.sender] = 0;
            token.safeTransfer(msg.sender, reward);
            
            emit RewardPaid(msg.sender, reward);
        }
    }

    /**
     * @notice Add reward to be distributed
     */
    function notifyRewardAmount(uint256 _amount) external onlyOwner {
        require(_amount > 0, "Amount is 0");
        
        token.safeTransferFrom(msg.sender, address(this), _amount);
        
        uint256 tokenSupply = totalSupplyAtEpoch;
        if (tokenSupply > 0) {
            rewardRate = _amount / WEEK;
        } else {
            rewardRate = 0;
        }
        
        emit RewardAdded(_amount);
    }

    // ============ View Functions ============

    /**
     * @notice Get current voting power
     */
    function balanceOf(address _user) public view returns (uint256) {
        LockedBalance memory locked_ = locked[_user];
        if (locked_.amount == 0) return 0;
        
        return _balanceOfAt(_user, block.timestamp);
    }

    /**
     * @notice Get voting power at specific time
     */
    function balanceOfAt(address _user, uint256 _timestamp) external view returns (uint256) {
        return _balanceOfAt(_user, _timestamp);
    }

    // ============ Internal Functions ============

    /**
     * @dev Calculate voting power at timestamp
     */
    function _balanceOfAt(address _user, uint256 _timestamp) internal view returns (uint256) {
        LockedBalance memory locked_ = locked[_user];
        
        if (locked_.amount == 0) return 0;
        
        uint256 userEpoch = userPointEpoch[_user];
        if (userEpoch == 0) return 0;
        
        Point memory userPoint = userPointHistory[_user][userEpoch];
        
        // Calculate bias at timestamp
        int256 bias = int256(userPoint.slope * (locked_.end - _timestamp));
        if (bias < 0) return 0;
        
        return uint256(bias);
    }

    /**
     * @dev Checkpoint for user
     */
    function _checkpoint(
        address _user,
        LockedBalance memory _locked,
        LockedBalance memory _oldLocked
    ) internal {
        // Save old point
        uint256 uOld = userPointEpoch[_user];
        
        if (uOld != 0) {
            Point memory oldPoint = userPointHistory[_user][uOld];
            oldPoint.ts = block.timestamp;
            userPointHistory[_user][uOld] = oldPoint;
        }
        
        // Create new point
        uint256 newEpoch = uOld + 1;
        userPointEpoch[_user] = newEpoch;
        
        uint256 dBias = 0;
        uint256 dSlope = 0;
        
        if (_oldLocked.end > block.timestamp && _oldLocked.end > _oldLocked.start) {
            dSlope -= _oldLocked.amount / (_oldLocked.end - _oldLocked.start);
            dBias += _oldLocked.amount * (_oldLocked.end - _oldLocked.start);
        }
        
        if (_locked.end > block.timestamp && _locked.end > _locked.start) {
            dSlope += _locked.amount / (_locked.end - _locked.start);
            dBias += _locked.amount * (_locked.end - _locked.start);
        }
        
        Point memory newPoint = Point({
            bias: dBias > 0 ? dBias : 0,
            slope: dSlope,
            timestamp: block.timestamp,
            blk: block.number
        });
        
        userPointHistory[_user][newEpoch] = newPoint;
        
        // Update global checkpoint
        _globalCheckpoint();
    }

    /**
     * @dev Global checkpoint
     */
    function _globalCheckpoint() internal {
        uint256 t = (block.timestamp / WEEK) * WEEK;
        
        // Update slope changes
        int256 dSlope = slopeChanges[t];
        
        Point memory lastPoint = pointHistory[pointHistory.length - 1];
        
        if (lastPoint.timestamp != t) {
            uint256 bias = lastPoint.bias + lastPoint.slope * (t - lastPoint.timestamp);
            uint256 slope = lastPoint.slope + dSlope;
            
            pointHistory.push(Point({
                bias: bias,
                slope: slope,
                timestamp: t,
                blk: block.number
            }));
        }
        
        // Update supply
        totalSupplyAtEpoch = pointHistory[pointHistory.length - 1].bias;
        lockedAtEpoch = totalLocked;
    }

    /**
     * @dev Update reward for user
     */
    function _updateReward(address _user) internal {
        if (totalSupplyAtEpoch > 0) {
            uint256 period = block.timestamp - startTime;
            uint256 reward = period * rewardRate * balanceOf(_user) / totalSupplyAtEpoch;
            rewards[_user] += reward;
        }
    }

    /**
     * @dev Square root function
     */
    function _sqrt(uint256 x) internal pure returns (uint256) {
        uint256 z = (x + 1) / 2;
        uint256 y = x;
        while (z < y) {
            y = z;
            z = (x / z + z) / 2;
        }
        return y;
    }
}
