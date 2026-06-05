// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "@openzeppelin/contracts/token/ERC20/IERC20.sol";

/**
 * @title TigerStaking
 * @notice Staking contract for TigerSwap token holders
 */
contract TigerStaking {
    struct StakeInfo {
        uint256 amount;
        uint256 startTime;
        uint256 rewardDebt;
        uint256 pendingRewards;
    }

    IERC20 public stakingToken;
    IERC20 public rewardToken;
    
    uint256 public accRewardPerShare;
    uint256 public lastRewardTime;
    uint256 public rewardPerSecond;
    uint256 public constant HALVING_INTERVAL = 365 days;
    uint256 public constant MIN_STAKE_DURATION = 1 days;
    
    uint256 public totalStaked;
    uint256 public minimumStake;
    uint256 public maximumStake;
    uint256 public earlyWithdrawPenalty = 500; // 5%

    mapping(address => StakeInfo) public stakes;
    mapping(address => uint256[]) public stakeTimestamps;
    mapping(address => uint256[]) public stakeAmounts;
    
    mapping(address => bool) public isWhitelisted;
    uint256 public whitelistDuration = 90 days;

    event Staked(address indexed user, uint256 amount, uint256 duration);
    event Unstaked(address indexed user, uint256 amount, uint256 penalty);
    event RewardClaimed(address indexed user, uint256 reward);
    event RewardAdded(uint256 amount);

    constructor(address _stakingToken, address _rewardToken) {
        stakingToken = IERC20(_stakingToken);
        rewardToken = IERC20(_rewardToken);
        minimumStake = 100 * 10**18;
        maximumStake = 1000000 * 10**18;
    }

    function stake(uint256 amount, uint256 duration) external {
        require(amount >= minimumStake, "TigerStaking: BELOW_MINIMUM");
        require(amount <= maximumStake, "TigerStaking: ABOVE_MAXIMUM");
        require(duration >= MIN_STAKE_DURATION, "TigerStaking: DURATION_TOO_SHORT");
        require(stakes[msg.sender].amount == 0, "TigerStaking: ALREADY_STAKED");

        uint256 multiplier = _getMultiplier(duration);
        uint256 actualAmount = (amount * multiplier) / 1e18;

        stakingToken.transferFrom(msg.sender, address(this), amount);
        
        stakes[msg.sender] = StakeInfo({
            amount: amount,
            startTime: block.timestamp,
            rewardDebt: actualAmount * accRewardPerShare / 1e18,
            pendingRewards: 0
        });
        
        stakeTimestamps[msg.sender].push(block.timestamp);
        stakeAmounts[msg.sender].push(amount);
        totalStaked += amount;

        emit Staked(msg.sender, amount, duration);
    }

    function unstake() external {
        StakeInfo storage stakeInfo = stakes[msg.sender];
        require(stakeInfo.amount > 0, "TigerStaking: NO_STAKE");
        
        uint256 stakeDuration = block.timestamp - stakeInfo.startTime;
        uint256 penalty = 0;
        
        if (stakeDuration < MIN_STAKE_DURATION) {
            penalty = stakeInfo.amount * earlyWithdrawPenalty / 10000;
        }
        
        uint256 pending = _pendingReward(msg.sender);
        uint256 amountToWithdraw = stakeInfo.amount - penalty;
        
        delete stakes[msg.sender];
        totalStaked -= stakeInfo.amount;
        
        stakingToken.transfer(msg.sender, amountToWithdraw);
        if (pending > 0) {
            rewardToken.transfer(msg.sender, pending);
            emit RewardClaimed(msg.sender, pending);
        }
        
        emit Unstaked(msg.sender, amountToWithdraw, penalty);
    }

    function claimReward() external {
        uint256 pending = _pendingReward(msg.sender);
        require(pending > 0, "TigerStaking: NO_REWARD");

        stakes[msg.sender].rewardDebt = stakes[msg.sender].amount * accRewardPerShare / 1e18;
        stakes[msg.sender].pendingRewards = 0;

        rewardToken.transfer(msg.sender, pending);
        emit RewardClaimed(msg.sender, pending);
    }

    function _pendingReward(address user) internal view returns (uint256) {
        StakeInfo storage stakeInfo = stakes[user];
        if (stakeInfo.amount == 0) return stakeInfo.pendingRewards;
        
        uint256 reward = stakeInfo.amount * accRewardPerShare / 1e18 - stakeInfo.rewardDebt;
        return stakeInfo.pendingRewards + reward;
    }

    function _getMultiplier(uint256 duration) internal pure returns (uint256) {
        if (duration >= 365 days) return 1500; // 1.5x for 1 year
        if (duration >= 180 days) return 1250; // 1.25x for 6 months
        if (duration >= 90 days) return 1100;  // 1.1x for 3 months
        return 1000; // 1x base
    }

    function updateRewardRate(uint256 newRate) external {
        accRewardPerShare += (block.timestamp - lastRewardTime) * rewardPerSecond * 1e18 / totalStaked;
        rewardPerSecond = newRate;
        lastRewardTime = block.timestamp;
    }

    function addReward(uint256 amount) external {
        rewardToken.transferFrom(msg.sender, address(this), amount);
        emit RewardAdded(amount);
    }

    function setMinimumStake(uint256 amount) external {
        minimumStake = amount;
    }

    function setMaximumStake(uint256 amount) external {
        maximumStake = amount;
    }
}