// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import "@openzeppelin/contracts/token/ERC20/extensions/ERC20Burnable.sol";
import "@openzeppelin/contracts/access/AccessControl.sol";
import "@openzeppelin/contracts/utils/ReentrancyGuard.sol";

/**
 * @title Vote-Escrowed TIGER Token
 * @notice Implements vote-escrowed tokenomics for TigerSwap governance
 * @dev Lock TIGER to receive voting power and boosted rewards
 */
contract VeToken is ERC20, ERC20Burnable, AccessControl, ReentrancyGuard {
    bytes32 public constant WARDEN_ROLE = keccak256("WARDEN_ROLE");
    bytes32 public constant VOTING_MANAGER_ROLE = keccak256("VOTING_MANAGER_ROLE");
    
    // Maximum lock duration: 4 years (in seconds)
    uint256 public constant MAX_LOCK_TIME = 4 * 365 days;
    
    // Minimum lock duration: 1 week
    uint256 public constant MIN_LOCK_TIME = 7 days;
    
    // Governance token reference
    ERC20 public immutable tigerToken;
    
    // Locked balance information
    mapping(address => LockedBalance) public lockedBalances;
    mapping(address => uint256) public userPointEpoch;
    mapping(address => Point[]) public userPoints;
    
    // Global checkpoint
    Point[] public pointHistory;
    uint256 public pointEpoch;
    
    // Voting epoch (weekly)
    uint256 public epoch;
    uint256 public epochStartTime;
    
    // Gauge voting
    mapping(address => uint256) public gaugeVotes;
    mapping(address => address[]) public gaugeVoteList;
    mapping(address => uint256) public gaugeVoteCount;
    
    // Supply tracking
    uint256 public totalLockedSupply;
    uint256 public totalLockedSupplyAtLastEpoch;
    
    // Rewards
    mapping(address => uint256) public claimableRewards;
    mapping(address => uint256) public claimedRewards;
    uint256 public rewardsPerTokenStored;
    uint256 public lastUpdateTime;
    
    // Boost parameters
    uint256 public constant BOOST_BASE = 1e18;
    uint256 public constant MAX_BOOST = 2.5e18; // 2.5x max boost
    uint256 public constant WEEK = 7 days;
    
    // Events
    event LockCreated(address indexed user, uint256 amount, uint256 unlockTime);
    event LockExtended(address indexed user, uint256 newUnlockTime);
    event LockWithdrawn(address indexed user, uint256 amount);
    event VotesModified(address indexed user, uint256 oldWeight, uint256 newWeight);
    event GaugeVoteModified(address indexed user, address indexed gauge, uint256 weight);
    event RewardsClaimed(address indexed user, uint256 amount);
    event RewardsAdded(uint256 amount);
    
    struct LockedBalance {
        uint256 amount;
        uint256 unlockTime;
        bool isLocked;
    }
    
    struct Point {
        int256 bias;
        int256 slope;
        uint256 ts;
        uint256 blockNumber;
    }
    
    modifier onlyWarden() {
        require(hasRole(WARDEN_ROLE, msg.sender), "Caller is not a warden");
        _;
    }
    
    modifier onlyVotingManager() {
        require(hasRole(VOTING_MANAGER_ROLE, msg.sender), "Caller is not a voting manager");
        _;
    }
    
    constructor(address _tigerToken, address _admin) ERC20("Vote-Escrowed TIGER", "veTIGER") {
        require(_tigerToken != address(0), "Invalid TIGER address");
        tigerToken = ERC20(_tigerToken);
        
        // Grant roles
        _grantRole(DEFAULT_ADMIN_ROLE, _admin);
        _grantRole(WARDEN_ROLE, _admin);
        _grantRole(VOTING_MANAGER_ROLE, _admin);
        
        // Initialize checkpoint
        pointHistory.push(Point({bias: 0, slope: 0, ts: block.timestamp, blockNumber: block.number}));
        epochStartTime = block.timestamp;
    }
    
    /**
     * @notice Lock TIGER tokens to receive voting power
     * @param amount Amount of TIGER to lock
     * @param lockDuration Duration to lock for (in seconds)
     */
    function createLock(uint256 amount, uint256 lockDuration) external nonReentrant whenNotPaused {
        require(amount > 0, "Amount is 0");
        require(lockDuration >= MIN_LOCK_TIME, "Lock duration too short");
        require(lockDuration <= MAX_LOCK_TIME, "Lock duration too long");
        
        LockedBalance storage lock = lockedBalances[msg.sender];
        
        // If user has existing lock, it must be expired or withdrawn first
        if (lock.amount > 0) {
            require(lock.unlockTime <= block.timestamp, "Lock still active");
        }
        
        // Transfer TIGER from user
        require(tigerToken.transferFrom(msg.sender, address(this), amount), "Transfer failed");
        
        // Update lock
        uint256 unlockTime = block.timestamp + lockDuration;
        lock.amount = lock.amount + amount;
        lock.unlockTime = unlockTime;
        lock.isLocked = true;
        
        // Update global supply
        totalLockedSupply += amount;
        
        // Create checkpoint
        _checkpoint(msg.sender, lock.amount, unlockTime);
        
        emit LockCreated(msg.sender, amount, unlockTime);
    }
    
    /**
     * @notice Extend the lock duration
     * @param newLockDuration New duration to extend to
     */
    function extendLock(uint256 newLockDuration) external nonReentrant whenNotPaused {
        LockedBalance storage lock = lockedBalances[msg.sender];
        require(lock.amount > 0, "No lock found");
        require(lock.unlockTime > block.timestamp, "Lock expired");
        require(newLockDuration >= MIN_LOCK_TIME, "Lock duration too short");
        require(newLockDuration <= MAX_LOCK_TIME, "Lock duration too long");
        
        uint256 newUnlockTime = block.timestamp + newLockDuration;
        lock.unlockTime = newUnlockTime;
        
        // Create checkpoint
        _checkpoint(msg.sender, lock.amount, newUnlockTime);
        
        emit LockExtended(msg.sender, newUnlockTime);
    }
    
    /**
     * @notice Increase the amount of locked tokens
     * @param amount Amount to add
     */
    function increaseLockAmount(uint256 amount) external nonReentrant whenNotPaused {
        require(amount > 0, "Amount is 0");
        
        LockedBalance storage lock = lockedBalances[msg.sender];
        require(lock.amount > 0, "No lock found");
        require(lock.unlockTime > block.timestamp, "Lock expired");
        
        // Transfer TIGER from user
        require(tigerToken.transferFrom(msg.sender, address(this), amount), "Transfer failed");
        
        // Update lock
        lock.amount += amount;
        totalLockedSupply += amount;
        
        // Create checkpoint
        _checkpoint(msg.sender, lock.amount, lock.unlockTime);
    }
    
    /**
     * @notice Withdraw unlocked tokens
     */
    function withdraw() external nonReentrant whenNotPaused {
        LockedBalance storage lock = lockedBalances[msg.sender];
        require(lock.amount > 0, "No lock found");
        require(lock.unlockTime <= block.timestamp, "Lock not expired");
        
        uint256 amount = lock.amount;
        
        // Clear lock
        lock.amount = 0;
        lock.unlockTime = 0;
        lock.isLocked = false;
        
        // Update global supply
        totalLockedSupply -= amount;
        
        // Create checkpoint
        _checkpoint(msg.sender, 0, 0);
        
        // Transfer TIGER back to user
        require(tigerToken.transfer(msg.sender, amount), "Transfer failed");
        
        emit LockWithdrawn(msg.sender, amount);
    }
    
    /**
     * @notice Get the voting power of a user
     * @param user The user address
     * @return The voting power (with boost)
     */
    function votingPower(address user) public view returns (uint256) {
        LockedBalance memory lock = lockedBalances[user];
        if (lock.amount == 0 || lock.unlockTime <= block.timestamp) {
            return 0;
        }
        
        // Calculate time remaining
        uint256 timeRemaining = lock.unlockTime - block.timestamp;
        
        // Calculate boost based on lock time
        uint256 lockDuration = lock.unlockTime - (userPoints[user][userPointEpoch[user]].ts);
        uint256 boost = _calculateBoost(lockDuration, timeRemaining);
        
        return (lock.amount * boost) / BOOST_BASE;
    }
    
    /**
     * @notice Get the raw voting power without boost
     * @param user The user address
     * @return The raw voting power
     */
    function rawVotingPower(address user) external view returns (uint256) {
        LockedBalance memory lock = lockedBalances[user];
        if (lock.amount == 0 || lock.unlockTime <= block.timestamp) {
            return 0;
        }
        return lock.amount;
    }
    
    /**
     * @notice Vote for a gauge
     * @param gauges Array of gauge addresses
     * @param weights Array of weights (must sum to 10000)
     */
    function voteForGauges(address[] calldata gauges, uint256[] calldata weights) external whenNotPaused {
        require(gauges.length == weights.length, "Length mismatch");
        require(gauges.length > 0, "Empty array");
        
        uint256 totalWeight = 0;
        for (uint256 i = 0; i < weights.length; i++) {
            totalWeight += weights[i];
        }
        require(totalWeight == 10000, "Weights must sum to 10000");
        
        // Clear previous votes
        address[] storage prevList = gaugeVoteList[msg.sender];
        for (uint256 i = 0; i < prevList.length; i++) {
            address gauge = prevList[i];
            gaugeVotes[gauge] -= rawVotingPower(msg.sender);
        }
        delete gaugeVoteList[msg.sender];
        
        // Set new votes
        for (uint256 i = 0; i < gauges.length; i++) {
            address gauge = gauges[i];
            uint256 weight = weights[i];
            uint256 vp = rawVotingPower(msg.sender);
            
            if (vp > 0 && weight > 0) {
                uint256 gaugeWeight = (vp * weight) / 10000;
                gaugeVotes[gauge] += gaugeWeight;
                gaugeVoteList[msg.sender].push(gauge);
            }
        }
        
        gaugeVoteCount[msg.sender] = gauges.length;
    }
    
    /**
     * @notice Get the votes for a gauge
     * @param gauge The gauge address
     * @return The total votes for the gauge
     */
    function gaugeVotesFor(address gauge) external view returns (uint256) {
        return gaugeVotes[gauge];
    }
    
    /**
     * @notice Claim accumulated rewards
     */
    function claimRewards() external nonReentrant whenNotPaused {
        _updateRewards(msg.sender);
        
        uint256 claimable = claimableRewards[msg.sender] - claimedRewards[msg.sender];
        require(claimable > 0, "No rewards to claim");
        
        claimedRewards[msg.sender] = claimableRewards[msg.sender];
        
        // Transfer rewards (using TIGER as reward token)
        require(tigerToken.transfer(msg.sender, claimable), "Transfer failed");
        
        emit RewardsClaimed(msg.sender, claimable);
    }
    
    /**
     * @notice Add rewards to the distribution
     * @param amount Amount of rewards to add
     */
    function addRewards(uint256 amount) external onlyWarden whenNotPaused {
        require(amount > 0, "Amount is 0");
        
        // Transfer from sender
        require(tigerToken.transferFrom(msg.sender, address(this), amount), "Transfer failed");
        
        if (totalLockedSupply > 0) {
            rewardsPerTokenStored += (amount * 1e18) / totalLockedSupply;
        }
        
        lastUpdateTime = block.timestamp;
        
        emit RewardsAdded(amount);
    }
    
    /**
     * @notice Get the number of checkpoints for a user
     * @param user The user address
     * @return The number of checkpoints
     */
    function numUserPoints(address user) external view returns (uint256) {
        return userPointEpoch[user];
    }
    
    /**
     * @notice Get user point at specific index
     * @param user The user address
     * @param index The checkpoint index
     * @return The checkpoint data
     */
    function getUserPoint(address user, uint256 index) external view returns (Point memory) {
        return userPoints[user][index];
    }
    
    /**
     * @notice Get the total supply at a specific time
     * @param time The timestamp
     * @return The total supply
     */
    function totalSupplyAt(uint256 time) external view returns (uint256) {
        return _supplyAt(pointHistory, time);
    }
    
    /**
     * @notice Emergency pause
     */
    function pause() external onlyRole(DEFAULT_ADMIN_ROLE) {
        _pause();
    }
    
    /**
     * @notice Unpause
     */
    function unpause() external onlyRole(DEFAULT_ADMIN_ROLE) {
        _unpause();
    }
    
    // Internal functions
    
    function _checkpoint(address user, uint256 amount, uint256 unlockTime) internal {
        // Update user's point
        uint256 userEpoch = userPointEpoch[user];
        uint256 ts = block.timestamp;
        
        int256 newSlope = 0;
        if (amount > 0 && unlockTime > ts) {
            newSlope = int256(amount) / int256(unlockTime - ts);
        }
        
        Point memory newPoint = Point({
            bias: int256(amount),
            slope: newSlope,
            ts: ts,
            blockNumber: block.number
        });
        
        if (userEpoch == 0) {
            userPoints[user].push(newPoint);
            userPointEpoch[user] = 1;
        } else {
            Point memory lastPoint = userPoints[user][userEpoch];
            
            // If same timestamp, overwrite
            if (lastPoint.ts == ts) {
                userPoints[user][userEpoch] = newPoint;
            } else {
                userPoints[user].push(newPoint);
                userPointEpoch[user] = userEpoch + 1;
            }
        }
        
        // Update global point
        _globalCheckpoint(amount, newSlope);
    }
    
    function _globalCheckpoint(int256 amount, int256 slope) internal {
        uint256 ts = block.timestamp;
        uint256 epoch = ts / WEEK;
        
        if (pointEpoch == 0) {
            pointHistory.push(Point({
                bias: int256(totalLockedSupply),
                slope: 0,
                ts: ts,
                blockNumber: block.number
            }));
            pointEpoch = 1;
        } else {
            Point memory lastPoint = pointHistory[pointEpoch];
            
            int256 prevSlope = lastPoint.slope;
            int256 newSlope = slope;
            
            // Update bias
            int256 newBias = lastPoint.bias - (prevSlope * int256(WEEK)) + (newSlope * int256(WEEK));
            
            Point memory newGlobalPoint = Point({
                bias: newBias,
                slope: newSlope,
                ts: ts,
                blockNumber: block.number
            });
            
            pointHistory.push(newGlobalPoint);
            pointEpoch++;
        }
    }
    
    function _calculateBoost(uint256 lockDuration, uint256 timeRemaining) internal pure returns (uint256) {
        if (lockDuration == 0) return BOOST_BASE;
        
        uint256 ratio = (timeRemaining * BOOST_BASE) / lockDuration;
        
        // Linear interpolation from 1x to MAX_BOOST based on lock time
        uint256 boost = BOOST_BASE + ((MAX_BOOST - BOOST_BASE) * ratio) / BOOST_BASE;
        
        return boost;
    }
    
    function _supplyAt(Point[] storage points, uint256 time) internal view returns (uint256) {
        uint256 len = points.length;
        if (len == 0) return 0;
        
        // Find the point at or before the time
        for (uint256 i = len; i > 0; i--) {
            Point memory point = points[i - 1];
            if (point.ts <= time) {
                int256 slopeChange = int256(point.slope) * int256(time - point.ts);
                int256 bias = point.bias - slopeChange;
                return bias > 0 ? uint256(bias) : 0;
            }
        }
        
        return 0;
    }
    
    function _updateRewards(address user) internal {
        if (totalLockedSupply > 0) {
            uint256 duration = block.timestamp - lastUpdateTime;
            uint256 newRewards = (totalLockedSupply * duration * rewardsPerTokenStored) / 1e18;
            claimableRewards[user] += newRewards;
        }
        lastUpdateTime = block.timestamp;
    }
    
    function _beforeTokenTransfer(
        address from,
        address to,
        uint256 amount
    ) internal override whenNotPaused {
        super._beforeTokenTransfer(from, to, amount);
        
        // Cannot transfer veTIGER - only mint/burn by contract
        require(from == address(0) || to == address(0), "Transfer not allowed");
    }
}