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

    /// @notice Administrator allowed to fund rewards and set the duration.
    address public owner;
    address public futureOwner;

    /// @dev Reward funding is the only privileged action; everything else is
    ///      open, so anyone may stake, withdraw or claim their own funds.
    modifier onlyOwner() {
        require(msg.sender == owner, "TigerSwap: NOT_OWNER");
        _;
    }

    uint256 private _locked = 1;

    modifier nonReentrant() {
        require(_locked == 1, "TigerSwap: REENTRANT");
        _locked = 2;
        _;
        _locked = 1;
    }
    
    // Events
    event Staked(address indexed user, uint256 amount);
    event Withdrawn(address indexed user, uint256 amount);
    event RewardClaimed(address indexed user, uint256 reward);
    event RewardAdded(uint256 reward);
    event OwnershipTransferred(address indexed previousOwner, address indexed newOwner);

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
        require(_stakingToken != address(0) && _rewardToken != address(0), "TigerSwap: ZERO_ADDRESS");
        stakingToken = IERC20(_stakingToken);
        rewardToken = IERC20(_rewardToken);
        owner = msg.sender;
        emit OwnershipTransferred(address(0), msg.sender);
    }

    /// @notice Begin a two-step ownership transfer.
    function commitTransferOwnership(address newOwner) external onlyOwner {
        require(newOwner != address(0), "TigerSwap: ZERO_ADDRESS");
        futureOwner = newOwner;
    }

    /// @notice Complete a two-step ownership transfer.
    function acceptTransferOwnership() external {
        require(msg.sender == futureOwner, "TigerSwap: NOT_FUTURE_OWNER");
        emit OwnershipTransferred(owner, futureOwner);
        owner = futureOwner;
        futureOwner = address(0);
    }

    /**
     * @notice Stake tokens
     * @param _amount Amount to stake
     */
    function stake(uint256 _amount) external nonReentrant updateReward(msg.sender) {
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
    function withdraw(uint256 _amount) external nonReentrant updateReward(msg.sender) {
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
    function getReward() external nonReentrant updateReward(msg.sender) {
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
    function setRewardsDuration(uint256 _duration) external onlyOwner {
        require(finishAt < block.timestamp, "TigerSwap: REWARD_NOT_FINISHED");
        require(_duration > 0, "TigerSwap: ZERO_DURATION");
        duration = _duration;
    }

    /**
     * @notice Notify about new reward
     * @param _reward Reward amount
     */
    function notifyRewardAmount(uint256 _reward) external onlyOwner nonReentrant updateReward(address(0)) {
        require(_reward > 0, "TigerSwap: ZERO_REWARD");
        require(duration > 0, "TigerSwap: DURATION_NOT_SET");
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
