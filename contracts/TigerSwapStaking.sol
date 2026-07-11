// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/**
 * @title TigerSwapStaking
 * @notice Staking contract for earning rewards
 */
contract TigerSwapStaking {
    // ERC20 token interface
    IERC20 public stakingToken;
    IERC20 public rewardToken;
    
    // Duration of rewards to be paid out (in seconds)
    uint256 public duration;
    
    // Timestamp of when the rewards finish
    uint256 public finishAt;
    
    // Minimum of last updated time and reward finish time
    uint256 public updatedAt;
    
    // Reward to be paid out per second
    uint256 public rewardRate;
    
    // Sum of (reward rate * dt)
    uint256 public rewardPerTokenStored;
    
    // User address => rewardPerTokenPaid
    mapping(address => uint256) public rewardPerTokenPaid;
    
    // User address => rewards to be claimed
    mapping(address => uint256) public rewards;
    
    // Total supply
    uint256 public totalSupply;
    
    // User address => staked amount
    mapping(address => uint256) public balanceOf;
    
    // Events
    event Staked(address indexed user, uint256 amount);
    event Withdrawn(address indexed user, uint256 amount);
    event RewardClaimed(address indexed user, uint256 reward);
    event RewardAdded(uint256 reward);

    modifier updateReward(address _account) {
        rewardPerTokenStored = rewardPerToken();
        updatedAt = lastTimeRewardApplicable();
        
        if (_account != address(0)) {
            rewards[_account] = earned(_account);
            rewardPerTokenPaid[_account] = rewardPerTokenStored;
        }
        _;
    }

    constructor(address _stakingToken, address _rewardToken) {
        stakingToken = IERC20(_stakingToken);
        rewardToken = IERC20(_rewardToken);
    }

    /**
     * @notice Stake tokens
     * @param _amount Amount to stake
     */
    function stake(uint256 _amount) external updateReward(msg.sender) {
        require(_amount > 0, "TigerSwap: CANNOT_STAKE_ZERO");
        
        stakingToken.transferFrom(msg.sender, address(this), _amount);
        balanceOf[msg.sender] += _amount;
        totalSupply += _amount;
        
        emit Staked(msg.sender, _amount);
    }

    /**
     * @notice Withdraw staked tokens
     * @param _amount Amount to withdraw
     */
    function withdraw(uint256 _amount) external updateReward(msg.sender) {
        require(_amount > 0, "TigerSwap: CANNOT_WITHDRAW_ZERO");
        require(balanceOf[msg.sender] >= _amount, "TigerSwap: INSUFFICIENT_BALANCE");
        
        balanceOf[msg.sender] -= _amount;
        totalSupply -= _amount;
        stakingToken.transfer(msg.sender, _amount);
        
        emit Withdrawn(msg.sender, _amount);
    }

    /**
     * @notice Claim earned rewards
     */
    function getReward() external updateReward(msg.sender) {
        uint256 reward = rewards[msg.sender];
        require(reward > 0, "TigerSwap: NO_REWARD");
        
        rewards[msg.sender] = 0;
        rewardToken.transfer(msg.sender, reward);
        
        emit RewardClaimed(msg.sender, reward);
    }

    /**
     * @notice Set rewards duration
     * @param _duration Duration in seconds
     */
    function setRewardsDuration(uint256 _duration) external {
        require(finishAt < block.timestamp, "TigerSwap: REWARD_NOT_FINISHED");
        duration = _duration;
    }

    /**
     * @notice Notify about new reward
     * @param _reward Reward amount
     */
    function notifyRewardAmount(uint256 _reward) external updateReward(address(0)) {
        if (block.timestamp >= finishAt) {
            rewardRate = _reward / duration;
        } else {
            uint256 remainingRewards = (finishAt - block.timestamp) * rewardRate;
            rewardRate = (_reward + remainingRewards) / duration;
        }
        
        require(rewardRate > 0, "TigerSwap: REWARD_RATE_ZERO");
        require(rewardRate * duration <= rewardToken.balanceOf(address(this)), "TigerSwap: REWARD_TOO_HIGH");
        
        finishAt = block.timestamp + duration;
        updatedAt = block.timestamp;
        
        emit RewardAdded(_reward);
    }

    /**
     * @notice Calculate earned rewards for user
     * @param _account User address
     * @return Earned rewards
     */
    function earned(address _account) public view returns (uint256) {
        uint256 currentRewardPerToken = rewardPerToken();
        uint256 storedRewardPerToken = rewardPerTokenPaid[_account];
        
        return ((balanceOf[_account] * (currentRewardPerToken - storedRewardPerToken)) / 1e18) + rewards[_account];
    }

    /**
     * @notice Get last time reward is applicable
     * @return Last applicable time
     */
    function lastTimeRewardApplicable() public view returns (uint256) {
        return min(finishAt, block.timestamp);
    }

    /**
     * @notice Get reward per token
     * @return Reward per token
     */
    function rewardPerToken() public view returns (uint256) {
        if (totalSupply == 0) {
            return rewardPerTokenStored;
        }
        
        return rewardPerTokenStored + (rewardRate * (lastTimeRewardApplicable() - updatedAt) * 1e18 / totalSupply);
    }

    /**
     * @notice Get user info
     * @param _user User address
     * @return Staked amount, earned rewards
     */
    function userInfo(address _user) external view returns (uint256, uint256) {
        return (balanceOf[_user], earned(_user));
    }

    function min(uint256 a, uint256 b) internal pure returns (uint256) {
        return a < b ? a : b;
    }
}

interface IERC20 {
    function transfer(address to, uint256 value) external returns (bool);
    function transferFrom(address from, address to, uint256 value) external returns (bool);
    function balanceOf(address account) external view returns (uint256);
    function approve(address spender, uint256 value) external returns (bool);
    function allowance(address owner, address spender) external view returns (uint256);
}
