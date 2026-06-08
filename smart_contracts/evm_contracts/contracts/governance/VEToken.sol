// SPDX-License-Identifier: MIT
pragma solidity ^0.8.19;

/**
 * @title VEToken
 * @notice Curve-style veToken governance contract
 * @dev Lock tokens to receive voting power and governance rights
 */
contract VEToken {
    string public constant name = "TigerSwap Voting Escrow";
    string public constant symbol = "veTIGER";
    string public constant version = "1.0.0";
    uint8 public constant decimals = 18;

    /// @notice Chain ID
    uint256 public chainId;

    /// @notice Total supply
    uint256 public totalSupply;

    /// @notice Token address
    address public immutable token;

    /// @notice Lock data
    mapping(address => LockData[]) public locks;

    /// @notice User voting power
    mapping(address => uint256) public votingPower;

    /// @notice Boosted voting power
    mapping(address => uint256) public boostedVotingPower;

    /// @notice Total voting power
    uint256 public totalVotingPower;

    /// @notice Lock structure
    struct LockData {
        uint256 amount;
        uint256 lockStart;
        uint256 lockEnd;
        uint256 votingPower;
        bool isNFT;
        uint256 nftId;
    }

    /// @notice NFT data for tokenized locks
    struct NFTData {
        address owner;
        uint256 lockId;
    }

    /// @notice NFT ID to data
    mapping(uint256 => NFTData) public nftData;

    /// @notice Next NFT ID
    uint256 public nextNFTId = 1;

    /// @notice Events
    event LockCreated(address indexed user, uint256 amount, uint256 lockEnd, uint256 votingPower);
    event LockExtended(address indexed user, uint256 oldEnd, uint256 newEnd);
    event LockWithdrawn(address indexed user, uint256 amount);
    event VotingPowerUpdated(address indexed user, uint256 oldPower, uint256 newPower);
    event NFTMinted(address indexed owner, uint256 nftId);

    /// @notice Errors
    error InvalidAmount();
    error LockPeriodTooShort();
    error LockPeriodTooLong();
    error LockNotExpired();
    error InsufficientBalance();
    error ZeroVotingPower();
    error InvalidNFT();
    error NotOwner();

    /// @notice Constants
    uint256 public constant MIN_LOCK_PERIOD = 7 days;
    uint256 public constant MAX_LOCK_PERIOD = 4 * 365 days;
    uint256 public constant MAX_BOOST = 25000; // 2.5x

    constructor(address _token, uint256 _chainId) {
        token = _token;
        chainId = _chainId;
    }

    /**
     * @notice Create a new lock
     * @param _amount Amount of tokens to lock
     * @param _lockDuration Lock duration in seconds
     */
    function createLock(uint256 _amount, uint256 _lockDuration) external {
        if (_amount == 0) revert InvalidAmount();
        if (_lockDuration < MIN_LOCK_PERIOD) revert LockPeriodTooShort();
        if (_lockDuration > MAX_LOCK_PERIOD) revert LockPeriodTooLong();

        // Transfer tokens from user
        IERC20(token).transferFrom(msg.sender, address(this), _amount);

        // Calculate voting power
        uint256 timeFactor = _lockDuration / (365 days);
        uint256 _votingPower = _amount * timeFactor;

        if (_votingPower == 0) revert ZeroVotingPower();

        // Calculate boost based on lock duration
        uint256 boost = 10000;
        if (_lockDuration >= 4 * 365 days) {
            boost = MAX_BOOST;
        } else if (_lockDuration >= 2 * 365 days) {
            boost = 20000;
        } else if (_lockDuration >= 365 days) {
            boost = 15000;
        }

        uint256 _boostedPower = _votingPower * boost / 10000;

        // Create lock
        locks[msg.sender].push(LockData({
            amount: _amount,
            lockStart: block.timestamp,
            lockEnd: block.timestamp + _lockDuration,
            votingPower: _votingPower,
            isNFT: false,
            nftId: 0
        }));

        // Update voting power
        votingPower[msg.sender] += _votingPower;
        boostedVotingPower[msg.sender] += _boostedPower;
        totalVotingPower += _votingPower;

        totalSupply += _amount;

        emit LockCreated(msg.sender, _amount, block.timestamp + _lockDuration, _votingPower);
    }

    /**
     * @notice Extend lock duration
     * @param _lockIndex Index of the lock to extend
     * @param _newDuration New lock duration
     */
    function extendLock(uint256 _lockIndex, uint256 _newDuration) external {
        LockData[] storage userLocks = locks[msg.sender];
        if (_lockIndex >= userLocks.length) revert InvalidAmount();

        LockData storage _lock = userLocks[_lockIndex];
        if (_lock.lockEnd > block.timestamp) revert LockNotExpired();

        uint256 oldEnd = _lock.lockEnd;
        uint256 newEnd = block.timestamp + _newDuration;
        if (newEnd > block.timestamp + MAX_LOCK_PERIOD) revert LockPeriodTooLong();

        // Recalculate voting power
        uint256 timeFactor = newEnd / (365 days);
        uint256 newPower = _lock.amount * timeFactor;

        // Update boost
        uint256 boost = 10000;
        if (newEnd >= 4 * 365 days) {
            boost = MAX_BOOST;
        } else if (newEnd >= 2 * 365 days) {
            boost = 20000;
        } else if (newEnd >= 365 days) {
            boost = 15000;
        }

        uint256 newBoostedPower = newPower * boost / 10000;

        // Update storage
        _lock.lockEnd = newEnd;
        _lock.votingPower = newPower;

        // Update global voting power
        votingPower[msg.sender] = votingPower[msg.sender] - _lock.votingPower + newPower;
        boostedVotingPower[msg.sender] = boostedVotingPower[msg.sender] - (_lock.votingPower * boost / 10000) + newBoostedPower;
        totalVotingPower = totalVotingPower - _lock.votingPower + newPower;

        emit LockExtended(msg.sender, oldEnd, newEnd);
    }

    /**
     * @notice Withdraw after lock expires
     * @param _lockIndex Index of the lock to withdraw
     */
    function withdraw(uint256 _lockIndex) external {
        LockData[] storage userLocks = locks[msg.sender];
        if (_lockIndex >= userLocks.length) revert InvalidAmount();

        LockData storage _lock = userLocks[_lockIndex];
        if (_lock.lockEnd <= block.timestamp) revert LockNotExpired();

        uint256 amount = _lock.amount;
        if (amount == 0) revert InvalidAmount();

        // Update voting power
        votingPower[msg.sender] -= _lock.votingPower;
        boostedVotingPower[msg.sender] -= _lock.votingPower;
        totalVotingPower -= _lock.votingPower;

        // Clear lock
        _lock.amount = 0;
        _lock.votingPower = 0;

        // Transfer tokens
        IERC20(token).transfer(msg.sender, amount);

        totalSupply -= amount;

        emit LockWithdrawn(msg.sender, amount);
    }

    /**
     * @notice Get current voting power with decay
     * @param _user User address
     * @return Current voting power
     */
    function getCurrentVotingPower(address _user) external view returns (uint256) {
        LockData[] storage userLocks = locks[_user];
        uint256 totalPower;

        for (uint256 i = 0; i < userLocks.length; i++) {
            LockData storage _lock = userLocks[i];
            if (_lock.amount == 0) continue;

            uint256 timeRemaining = _lock.lockEnd > block.timestamp 
                ? _lock.lockEnd - block.timestamp 
                : 0;

            if (timeRemaining == 0) continue;

            uint256 totalDuration = _lock.lockEnd - _lock.lockStart;
            if (totalDuration == 0) continue;

            uint256 remainingPower = _lock.votingPower * timeRemaining / totalDuration;
            totalPower += remainingPower;
        }

        return totalPower;
    }

    /**
     * @notice Get user lock count
     * @param _user User address
     * @return Lock count
     */
    function getLockCount(address _user) external view returns (uint256) {
        return locks[_user].length;
    }

    /**
     * @notice Get user lock data
     * @param _user User address
     * @param _index Lock index
     * @return Lock data tuple
     */
    function getLock(address _user, uint256 _index) external view returns (
        uint256 amount,
        uint256 lockStart,
        uint256 lockEnd,
        uint256 _votingPower,
        bool isNFT,
        uint256 nftId
    ) {
        LockData storage _lock = locks[_user][_index];
        return (
            _lock.amount,
            _lock.lockStart,
            _lock.lockEnd,
            _lock.votingPower,
            _lock.isNFT,
            _lock.nftId
        );
    }
}

/**
 * @title IVEToken
 * @notice Minimal IVEToken interface
 */
interface IERC20 {
    function transferFrom(address from, address to, uint256 amount) external returns (bool);
    function transfer(address to, uint256 amount) external returns (bool);
    function balanceOf(address account) external view returns (uint256);
}