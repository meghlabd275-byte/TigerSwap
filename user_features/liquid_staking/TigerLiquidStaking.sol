// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import "@openzeppelin/contracts/token/ERC20/extensions/ERC20Burnable.sol";
import "@openzeppelin/contracts/access/AccessControl.sol";
import "@openzeppelin/contracts/security/ReentrancyGuard.sol";

/**
 * @title TigerLiquidStaking
 * @notice Liquid Staking with staked token (stTIGER)
 * @dev Users stake TIGER and receive stTIGER representing their share
 */

contract TigerLiquidStaking is ERC20, ERC20Burnable, AccessControl, ReentrancyGuard {
    bytes32 public constant WARDEN_ROLE = keccak256("WARDEN_ROLE");
    bytes32 public constant OPERATOR_ROLE = keccak256("OPERATOR_ROLE");
    
    // Precision
    uint256 public constant PRECISION = 1e18;
    uint256 public constant EPOCH_DURATION = 1 days;
    
    // Token
    ERC20 public immutable stakingToken;
    
    // State
    uint256 public totalStaked;
    uint256 public totalRewards;
    uint256 public rewardPerTokenStored;
    uint256 public lastUpdateTime;
    uint256 public epoch;
    uint256 public nextEpoch;
    
    // Rewards
    mapping(address => uint256) public userRewardPerTokenPaid;
    mapping(address => uint256) public rewards;
    mapping(address => uint256) public stakedAt;
    
    // Unstaking queue
    struct UnstakeRequest {
        address user;
        uint256 amount;
        uint256 requestTime;
        uint256 claimTime;
        bool claimed;
    }
    
    uint256 public requestCount;
    mapping(uint256 => UnstakeRequest) public unstakeRequests;
    mapping(address => uint256[]) public userUnstakeRequests;
    
    // Slashing
    mapping(address => uint256) public slashAmounts;
    uint256 public totalSlashed;
    
    // Events
    event Staked(address indexed user, uint256 amount, uint256 shares);
    event UnstakeRequested(address indexed user, uint256 amount, uint256 requestId);
    event UnstakeClaimed(address indexed user, uint256 amount, uint256 requestId);
    event RewardAdded(uint256 amount);
    event RewardClaimed(address indexed user, uint256 amount);
    event Slashed(address indexed validator, uint256 amount);
    
    modifier onlyWardens() {
        require(hasRole(WARDEN_ROLE, msg.sender), "Not warden");
        _;
    }
    
    modifier onlyOperators() {
        require(hasRole(OPERATOR_ROLE, msg.sender), "Not operator");
        _;
    }
    
    constructor(address _stakingToken, address _admin) ERC20("Staked TIGER", "stTIGER") {
        require(_stakingToken != address(0), "Invalid token");
        
        stakingToken = ERC20(_stakingToken);
        
        _grantRole(DEFAULT_ADMIN_ROLE, _admin);
        _grantRole(WARDEN_ROLE, _admin);
        _grantRole(OPERATOR_ROLE, _admin);
        
        lastUpdateTime = block.timestamp;
        epoch = block.timestamp / EPOCH_DURATION;
        nextEpoch = epoch + 1;
    }
    
    /**
     * @notice Stake tokens
     * @param amount Amount to stake
     */
    function stake(uint256 amount) external nonReentrant {
        require(amount > 0, "Cannot stake 0");
        
        // Update rewards
        _updateReward(msg.sender);
        
        // Transfer tokens
        require(stakingToken.transferFrom(msg.sender, address(this), amount), "Transfer failed");
        
        // Calculate shares
        uint256 shares = amount;
        if (totalStaked > 0) {
            shares = (amount * totalSupply()) / totalStaked;
        }
        
        // Mint shares
        _mint(msg.sender, shares);
        
        // Update state
        totalStaked += amount;
        stakedAt[msg.sender] = block.timestamp;
        
        emit Staked(msg.sender, amount, shares);
    }
    
    /**
     * @notice Request unstake
     * @param amount Amount to unstake
     */
    function requestUnstake(uint256 amount) external nonReentrant {
        require(amount > 0, "Cannot unstake 0");
        require(balanceOf(msg.sender) >= amount, "Insufficient balance");
        
        // Update rewards
        _updateReward(msg.sender);
        
        // Calculate underlying amount
        uint256 underlying = (amount * totalStaked) / totalSupply();
        
        // Burn shares
        _burn(msg.sender, amount);
        
        // Update state
        totalStaked -= underlying;
        
        // Create unstake request
        uint256 requestId = ++requestCount;
        unstakeRequests[requestId] = UnstakeRequest({
            user: msg.sender,
            amount: underlying,
            requestTime: block.timestamp,
            claimTime: block.timestamp + 14 days, // 14 day unstaking period
            claimed: false
        });
        
        userUnstakeRequests[msg.sender].push(requestId);
        
        emit UnstakeRequested(msg.sender, underlying, requestId);
    }
    
    /**
     * @notice Claim unstaked tokens
     * @param requestId Request ID
     */
    function claimUnstake(uint256 requestId) external nonReentrant {
        UnstakeRequest storage request = unstakeRequests[requestId];
        require(request.user == msg.sender, "Not owner");
        require(!request.claimed, "Already claimed");
        require(block.timestamp >= request.claimTime, "Not yet");
        
        request.claimed = true;
        
        // Transfer tokens
        require(stakingToken.transfer(msg.sender, request.amount), "Transfer failed");
        
        emit UnstakeClaimed(msg.sender, request.amount, requestId);
    }
    
    /**
     * @notice Claim rewards
     */
    function claimRewards() external nonReentrant {
        _updateReward(msg.sender);
        
        uint256 reward = rewards[msg.sender];
        require(reward > 0, "No rewards");
        
        rewards[msg.sender] = 0;
        
        require(stakingToken.transfer(msg.sender, reward), "Transfer failed");
        
        emit RewardClaimed(msg.sender, reward);
    }
    
    /**
     * @notice Add rewards to pool
     * @param amount Amount to add
     */
    function addRewards(uint256 amount) external onlyWardens {
        require(amount > 0, "Amount is 0");
        
        // Transfer from sender
        require(stakingToken.transferFrom(msg.sender, address(this), amount), "Transfer failed");
        
        // Update reward rate
        if (totalStaked > 0) {
            rewardPerTokenStored += (amount * PRECISION) / totalStaked;
        }
        
        totalRewards += amount;
        lastUpdateTime = block.timestamp;
        
        emit RewardAdded(amount);
    }
    
    /**
     * @notice Slash a validator
     * @param validator Validator address
     * @param amount Amount to slash
     */
    function slash(address validator, uint256 amount) external onlyWardens {
        require(amount > 0, "Amount is 0");
        
        slashAmounts[validator] += amount;
        totalSlashed += amount;
        
        emit Slashed(validator, amount);
    }
    
    /**
     * @notice Get pending rewards
     * @param user User address
     * @return Pending rewards
     */
    function pendingRewards(address user) external view returns (uint256) {
        uint256 rewardPerToken = rewardPerTokenStored;
        if (totalStaked > 0) {
            uint256 timeSinceUpdate = block.timestamp - lastUpdateTime;
            rewardPerToken += (timeSinceUpdate * PRECISION) / totalStaked;
        }
        
        uint256 userShares = balanceOf(user);
        uint256 pending = (userShares * rewardPerToken) / PRECISION;
        return pending - userRewardPerTokenPaid[user];
    }
    
    /**
     * @notice Get staked amount for user
     * @param user User address
     * @return Staked amount
     */
    function getStakedAmount(address user) external view returns (uint256) {
        if (totalSupply() == 0) return 0;
        return (balanceOf(user) * totalStaked) / totalSupply();
    }
    
    /**
     * @notice Get user unstake requests
     * @param user User address
     * @return Array of request IDs
     */
    function getUserUnstakeRequests(address user) external view returns (uint256[] memory) {
        return userUnstakeRequests[user];
    }
    
    /**
     * @notice Get exchange rate
     * @return stTIGER to TIGER exchange rate
     */
    function getExchangeRate() external view returns (uint256) {
        if (totalSupply() == 0) return PRECISION;
        return (totalStaked * PRECISION) / totalSupply();
    }
    
    // Internal functions
    
    function _updateReward(address user) internal {
        rewardPerTokenStored = rewardPerTokenPaid[user];
        if (totalStaked > 0) {
            uint256 timeSinceUpdate = block.timestamp - lastUpdateTime;
            uint256 reward = (timeSinceUpdate * totalRewards) / EPOCH_DURATION;
            rewardPerTokenStored += (reward * PRECISION) / totalStaked;
        }
        
        rewards[user] = (balanceOf(user) * rewardPerTokenStored) / PRECISION - userRewardPerTokenPaid[user];
        userRewardPerTokenPaid[user] = rewardPerTokenStored;
        lastUpdateTime = block.timestamp;
    }
    
    // Override transfer to prevent staking token transfers
    function _beforeTokenTransfer(
        address from,
        address to,
        uint256 amount
    ) internal override {
        super._beforeTokenTransfer(from, to, amount);
        
        if (from != address(0) && to != address(0)) {
            _updateReward(from);
            _updateReward(to);
        }
    }
}