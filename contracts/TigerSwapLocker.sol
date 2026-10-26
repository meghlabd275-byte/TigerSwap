// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./TigerSwapInterfaces.sol";

/**
 * @title TigerSwapLocker
 * @notice Locks LP tokens for 1 week to 4 years to earn boosted TGR rewards.
 *
 * Rewards accrue against a boost that scales linearly with the remaining lock
 * time, from 1x for a minimum-term lock up to 4x for a maximum-term lock. A
 * deposit keeps its own unlock time and boost, so several locks can coexist
 * per account without overwriting one another.
 */
contract TigerSwapLocker {
    IERC20 public immutable lpToken;
    IERC20 public immutable rewardToken;

    uint256 public constant WEEK = 86400 * 7;
    uint256 public constant MIN_LOCK = WEEK;
    uint256 public constant MAX_LOCK = 4 * 365 * 86400;
    uint256 public constant MIN_BOOST_BPS = 10000; // 1x
    uint256 public constant MAX_BOOST_BPS = 40000; // 4x

    struct Lock {
        uint256 amount;
        uint256 unlockTime;
        uint256 boostBps;
        uint256 rewardDebt;
        bool withdrawn;
    }

    mapping(address => Lock[]) private locks;
    mapping(address => uint256) public lockedBalance;

    uint256 public accRewardPerShare; // scaled by ACC_PRECISION
    uint256 public constant ACC_PRECISION = 1e18;
    uint256 public totalLocked;
    /// @dev Sum of boost-weighted amounts. Rewards are funded against this, not
    /// against raw `totalLocked`, so a boosted claim can never exceed the pool.
    uint256 public totalWeighted;

    address public owner;
    address public pendingOwner;

    event Locked(address indexed user, uint256 indexed lockId, uint256 amount, uint256 unlockTime, uint256 boostBps);
    event Withdrawn(address indexed user, uint256 indexed lockId, uint256 amount);
    event RewardsClaimed(address indexed user, uint256 indexed lockId, uint256 amount);
    event RewardNotified(uint256 amount, uint256 newAccRewardPerShare);
    event Migrated(address indexed user, uint256 indexed lockId, uint256 amount);
    event OwnershipTransferStarted(address indexed previousOwner, address indexed newOwner);
    event OwnershipTransferred(address indexed previousOwner, address indexed newOwner);

    modifier onlyOwner() {
        require(msg.sender == owner, "not owner");
        _;
    }

    uint256 private _status = 1;

    modifier nonReentrant() {
        require(_status == 1, "reentrant call");
        _status = 2;
        _;
        _status = 1;
    }

    constructor(address _lpToken, address _rewardToken) {
        require(_lpToken != address(0) && _rewardToken != address(0), "token required");
        lpToken = IERC20(_lpToken);
        rewardToken = IERC20(_rewardToken);
        owner = msg.sender;
        emit OwnershipTransferred(address(0), msg.sender);
    }

    // ---- Boost -------------------------------------------------------------

    /// @notice Boost in basis points for a given lock duration.
    function boostFor(uint256 _duration) public pure returns (uint256) {
        if (_duration <= MIN_LOCK) {
            return MIN_BOOST_BPS;
        }
        if (_duration >= MAX_LOCK) {
            return MAX_BOOST_BPS;
        }
        // Linear interpolation between 1x at MIN_LOCK and 4x at MAX_LOCK.
        uint256 span = MAX_LOCK - MIN_LOCK;
        uint256 progress = _duration - MIN_LOCK;
        return MIN_BOOST_BPS + ((MAX_BOOST_BPS - MIN_BOOST_BPS) * progress) / span;
    }

    // ---- Locking -----------------------------------------------------------

    /// @notice Lock LP tokens for `_duration` seconds (floored to whole weeks).
    function lock(uint256 _amount, uint256 _duration) external nonReentrant returns (uint256 lockId) {
        require(_amount > 0, "amount required");
        require(_duration >= MIN_LOCK, "lock too short");
        require(_duration <= MAX_LOCK, "lock too long");

        // Week-align the term, but let a maximum-duration request land exactly
        // on MAX_LOCK so it earns the full 4x boost (MAX_LOCK is not itself a
        // whole number of weeks).
        uint256 duration;
        if (_duration >= MAX_LOCK) {
            duration = MAX_LOCK;
        } else {
            duration = (_duration / WEEK) * WEEK;
            if (duration < MIN_LOCK) {
                duration = MIN_LOCK;
            }
        }

        uint256 boostBps = boostFor(duration);

        lockId = locks[msg.sender].length;
        locks[msg.sender].push(
            Lock({
                amount: _amount,
                unlockTime: block.timestamp + duration,
                boostBps: boostBps,
                rewardDebt: (_amount * boostBps * accRewardPerShare) / (MIN_BOOST_BPS * ACC_PRECISION),
                withdrawn: false
            })
        );

        lockedBalance[msg.sender] += _amount;
        totalLocked += _amount;
        totalWeighted += (_amount * boostBps) / MIN_BOOST_BPS;

        require(lpToken.transferFrom(msg.sender, address(this), _amount), "transferFrom failed");
        emit Locked(msg.sender, lockId, _amount, block.timestamp + duration, boostBps);
    }

    function lockCount(address _user) external view returns (uint256) {
        return locks[_user].length;
    }

    function getLock(address _user, uint256 _lockId) external view returns (Lock memory) {
        return locks[_user][_lockId];
    }

    /// @notice Pending rewards for one lock, weighted by its boost.
    function pendingReward(address _user, uint256 _lockId) public view returns (uint256) {
        Lock memory l = locks[_user][_lockId];
        if (l.withdrawn || l.amount == 0) {
            return 0;
        }
        uint256 weighted = (l.amount * l.boostBps) / MIN_BOOST_BPS;
        uint256 accumulated = (weighted * accRewardPerShare) / ACC_PRECISION;
        return accumulated > l.rewardDebt ? accumulated - l.rewardDebt : 0;
    }

    function claimRewards(uint256 _lockId) external nonReentrant returns (uint256) {
        require(_lockId < locks[msg.sender].length, "unknown lock");
        uint256 reward = pendingReward(msg.sender, _lockId);
        require(reward > 0, "nothing to claim");

        Lock storage l = locks[msg.sender][_lockId];
        uint256 weighted = (l.amount * l.boostBps) / MIN_BOOST_BPS;
        l.rewardDebt = (weighted * accRewardPerShare) / ACC_PRECISION;

        require(rewardToken.transfer(msg.sender, reward), "reward transfer failed");
        emit RewardsClaimed(msg.sender, _lockId, reward);
        return reward;
    }

    /// @notice Withdraw a matured lock.
    function withdraw(uint256 _lockId) external nonReentrant returns (uint256) {
        require(_lockId < locks[msg.sender].length, "unknown lock");
        Lock storage l = locks[msg.sender][_lockId];
        require(!l.withdrawn, "already withdrawn");
        require(l.unlockTime <= block.timestamp, "lock not expired");

        uint256 amount = l.amount;
        l.withdrawn = true;
        l.amount = 0;
        l.rewardDebt = 0;

        lockedBalance[msg.sender] -= amount;
        totalLocked -= amount;
        totalWeighted -= (amount * l.boostBps) / MIN_BOOST_BPS;

        require(lpToken.transfer(msg.sender, amount), "withdraw failed");
        emit Withdrawn(msg.sender, _lockId, amount);
        return amount;
    }

    // ---- Rewards funding ---------------------------------------------------

    /// @notice Fund rewards. The caller must have transferred `_amount` first.
    function notifyReward(uint256 _amount) external onlyOwner nonReentrant {
        require(_amount > 0, "amount required");
        require(totalWeighted > 0, "no locked liquidity");

        // Pull the rewards in first so the contract always holds enough to pay
        // every claim.
        require(rewardToken.transferFrom(msg.sender, address(this), _amount), "reward funding failed");

        // Rewards are distributed proportionally to boost-weighted liquidity.
        accRewardPerShare += (_amount * ACC_PRECISION) / totalWeighted;
        emit RewardNotified(_amount, accRewardPerShare);
    }

    // ---- Migration ---------------------------------------------------------

    /// @notice Withdraw a matured lock and forward the LP tokens to a migration
    /// target in a single call, for V2 to V3 position migration.
    function migrate(uint256 _lockId, address _target) external nonReentrant returns (uint256) {
        require(_target != address(0), "target required");
        require(_lockId < locks[msg.sender].length, "unknown lock");

        Lock storage l = locks[msg.sender][_lockId];
        require(!l.withdrawn, "already withdrawn");
        require(l.unlockTime <= block.timestamp, "lock not expired");

        uint256 amount = l.amount;
        l.withdrawn = true;
        l.amount = 0;
        l.rewardDebt = 0;

        lockedBalance[msg.sender] -= amount;
        totalLocked -= amount;
        totalWeighted -= (amount * l.boostBps) / MIN_BOOST_BPS;

        require(lpToken.transfer(_target, amount), "migration failed");
        emit Migrated(msg.sender, _lockId, amount);
        return amount;
    }

    // ---- Ownership ---------------------------------------------------------

    function transferOwnership(address _newOwner) external onlyOwner {
        require(_newOwner != address(0), "zero address");
        pendingOwner = _newOwner;
        emit OwnershipTransferStarted(owner, _newOwner);
    }

    function acceptOwnership() external {
        require(msg.sender == pendingOwner, "not pending owner");
        address previous = owner;
        owner = pendingOwner;
        pendingOwner = address(0);
        emit OwnershipTransferred(previous, owner);
    }
}
