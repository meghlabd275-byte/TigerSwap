// SPDX-License-Identifier: MIT
pragma solidity ^0.8.19;

/**
 * TigerSwap Staking Contract
 * 
 * @author TigerSwap
 * @version 1.0.0
 */

/**
 * @title TigerStaking
 * @dev Staking contract with rewards
 */
contract TigerStaking {
    
    IERC20 public stakingToken;
    IERC20 public rewardToken;
    
    uint256 public rewardRate;
    uint256 public lastUpdateTime;
    uint256 public rewardPerTokenStored;
    
    mapping(address => uint256) public userRewardPerTokenPaid;
    mapping(address => uint256) public rewards;
    
    uint256 private _totalSupply;
    mapping(address => uint256) private _balances;
    
    event Staked(address indexed user, uint256 amount);
    event Withdrawn(address indexed user, uint256 amount);
    event RewardClaimed(address indexed user, uint256 reward);
    
    constructor(IERC20 _stakingToken, IERC20 _rewardToken) {
        stakingToken = _stakingToken;
        rewardToken = _rewardToken;
    }
    
    /**
     * @dev Update reward
     */
    modifier updateReward(address account) {
        rewardPerTokenStored = rewardPerTokenStored;
        lastUpdateTime = block.timestamp;
        
        if (account != address(0)) {
            rewards[account] = earned(account);
            userRewardPerTokenPaid[account] = rewardPerTokenStored;
        }
        _;
    }
    
    /**
     * @dev Stake tokens
     */
    function stake(uint256 amount) external updateReward(msg.sender) {
        require(amount > 0, "Cannot stake 0");
        
        _totalSupply += amount;
        _balances[msg.sender] += amount;
        
        stakingToken.transferFrom(msg.sender, address(this), amount);
        
        emit Staked(msg.sender, amount);
    }
    
    /**
     * @dev Withdraw tokens
     */
    function withdraw(uint256 amount) external updateReward(msg.sender) {
        require(amount > 0, "Cannot withdraw 0");
        
        _totalSupply -= amount;
        _balances[msg.sender] -= amount;
        
        stakingToken.transfer(msg.sender, amount);
        
        emit Withdrawn(msg.sender, amount);
    }
    
    /**
     * @dev Claim rewards
     */
    function getReward() external updateReward(msg.sender) {
        uint256 reward = rewards[msg.sender];
        
        if (reward > 0) {
            rewards[msg.sender] = 0;
            rewardToken.transfer(msg.sender, reward);
            
            emit RewardClaimed(msg.sender, reward);
        }
    }
    
    /**
     * @dev Earned rewards
     */
    function earned(address account) public view returns (uint256) {
        uint256 balance = _balances[account];
        uint256 rewardPerToken = rewardPerTokenStored;
        
        if (_totalSupply > 0) {
            rewardPerToken = rewardPerToken + ((block.timestamp - lastUpdateTime) * rewardRate * 1e18 / _totalSupply);
        }
        
        return (balance * rewardPerToken / 1e18) - userRewardPerTokenPaid[account] + rewards[account];
    }
    
    /**
     * @dev Set reward rate
     */
    function setRewardRate(uint256 _rewardRate) external updateReward(address(0)) {
        rewardRate = _rewardRate;
    }
    
    /**
     * @dev Total staked
     */
    function totalSupply() external view returns (uint256) {
        return _totalSupply;
    }
    
    /**
     * @dev Balance of
     */
    function balanceOf(address account) external view returns (uint256) {
        return _balances[account];
    }
}

interface IERC20 {
    function transfer(address to, uint256 amount) external returns (bool);
    function transferFrom(address from, address to, uint256 amount) external returns (bool);
    function balanceOf(address account) external view returns (uint256);
}