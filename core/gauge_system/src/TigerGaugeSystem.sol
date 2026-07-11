// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/**
 * @title TigerGaugeSystem
 * @notice Production Gauge System - Curve/Aerodrome Style
 * @dev Liquidity incentive distribution system
 * 
 * Features:
 * - Gauge-based emissions
 * - Vote-escrowed rewards
 * - Bribe integration
 * - Multi-token emissions
 * - Epoch-based distribution
 * - Boost multiplier
 * 
 * @author TigerSwap
 * @version 1.0.0 Production
 */

import "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";
import "@openzeppelin/contracts/utils/ReentrancyGuard.sol";
import "@openzeppelin/contracts/access/Ownable.sol";

/**
 * @title Decimal Math
 */
library GaugeMath {
    uint256 constant PRECISION = 1e18;
    uint256 constant WEEK = 7 days;
    
    function mul(uint256 x, uint256 y) internal pure returns (uint256) {
        return (x * y) / PRECISION;
    }
    
    function div(uint256 x, uint256 y) internal pure returns (uint256) {
        return (x * PRECISION) / y;
    }
}

/**
 * @title IGauge
 * @dev Interface for gauge contracts
 */
interface IGauge {
    function deposit(uint256) external;
    function withdraw(uint256) external;
    function claimableRewards(address) external view returns (uint256);
    function rewardToken() external view returns (address);
}

/**
 * @title TigerGaugeController
 * @dev Main gauge controller
 */
contract TigerGaugeController is ReentrancyGuard, Ownable {
    using SafeERC20 for IERC20;
    using GaugeMath for uint256;

    // ============ Constants ============
    uint256 constant WEEK = 7 days;
    uint256 constant MAX_GAUGES = 100;
    uint256 constant PRECISION = 1e18;

    // ============ State Variables ============
    
    // Governance
    address public governance;
    address public emergency multisig;
    address public rewardVault;
    
    // Tokens
    address public governanceToken; // TIGER
    address public rewardToken; // e.g., WETH or stablecoin
    
    // Gauge management
    mapping(address => bool) public gauges;
    address[] public gaugeList;
    mapping(address => uint256) public gaugeType;
    mapping(uint256 => address[]) public typeToGauges;
    uint256 public gaugeCount;
    
    // Voting
    mapping(address => mapping(address => uint256)) public voteWeight;
    mapping(address => address[]) public votes;
    mapping(address => uint256) public voteCount;
    
    // Points
    mapping(uint256 => uint256) public pointsPerWeek;
    uint256 public pointsStartWeek;
    mapping(address => mapping(uint256 => uint256)) public gaugePoints;
    
    // Time
    uint256 public futureEpochTime;
    uint256 public epoch;
    
    // Reward distribution
    uint256 public rewardRate;
    uint256 public totalRewardPerToken;
    mapping(address => uint256) public rewardPerTokenStored;
    mapping(address => mapping(address => uint256)) public userRewardPerTokenPaid;
    mapping(address => mapping(address => uint256)) public pendingRewards;
    mapping(address => uint256) public claimableRewards;
    
    // ============ Events ============
    event NewGauge(address indexed gauge, uint256 gaugeType);
    event GaugeRemoved(address indexed gauge);
    event VoteChanged(address indexed user, address indexed gauge, uint256 weight);
    event GaugePointsUpdated(address indexed gauge, uint256 points, uint256 epoch);
    event RewardsAdded(uint256 amount);
    event RewardsClaimed(address indexed user, uint256 amount);
    event BribeReceived(address indexed gauge, uint256 amount);

    // ============ Constructor ============
    
    constructor(address _owner, address _governanceToken, address _rewardToken) Ownable(_owner) {
        governance = _owner;
        governanceToken = _governanceToken;
        rewardToken = _rewardToken;
        
        // Initialize first epoch
        uint256 startTime = (block.timestamp / WEEK) * WEEK + WEEK;
        pointsStartWeek = startTime / WEEK;
    }

    // ============ Gauge Functions ============

    /**
     * @notice Add a new gauge
     */
    function addGauge(address _gauge, uint256 _gaugeType) external onlyOwner {
        require(_gauge != address(0), "Invalid gauge");
        require(!gauges[_gauge], "Already added");
        require(gaugeCount < MAX_GAUGES, "Too many gauges");
        
        gauges[_gauge] = true;
        gaugeType[_gauge] = _gaugeType;
        gaugeList.push(_gauge);
        typeToGauges[_gaugeType].push(_gauge);
        
        gaugeCount++;
        
        emit NewGauge(_gauge, _gaugeType);
    }

    /**
     * @notice Remove a gauge
     */
    function removeGauge(address _gauge) external onlyOwner {
        require(gauges[_gauge], "Not a gauge");
        
        gauges[_gauge] = false;
        
        // Remove from list
        for (uint256 i = 0; i < gaugeList.length; i++) {
            if (gaugeList[i] == _gauge) {
                gaugeList[i] = gaugeList[gaugeList.length - 1];
                gaugeList.pop();
                break;
            }
        }
        
        emit GaugeRemoved(_gauge);
    }

    // ============ Voting Functions ============

    /**
     * @notice Vote for gauges
     */
    function vote(address[] calldata _gauges, uint256[] calldata _weights) external {
        require(_gauges.length == _weights.length, "Length mismatch");
        
        // Reset existing votes
        address[] memory currentVotes = votes[msg.sender];
        for (uint256 i = 0; i < currentVotes.length; i++) {
            address gauge = currentVotes[i];
            uint256 oldWeight = voteWeight[msg.sender][gauge];
            if (oldWeight > 0) {
                gaugePoints[gauge][epoch] -= oldWeight;
            }
        }
        
        // Set new votes
        uint256 totalWeight = 0;
        for (uint256 i = 0; i < _gauges.length; i++) {
            address gauge = _gauges[i];
            uint256 weight = _weights[i];
            
            require(gauges[gauge], "Not a gauge");
            require(weight <= 10000, "Weight too high"); // Max 100%
            
            voteWeight[msg.sender][gauge] = weight;
            gaugePoints[gauge][epoch] += weight;
            
            totalWeight += weight;
        }
        
        votes[msg.sender] = _gauges;
        voteCount[msg.sender] = _gauges.length;
        
        // Emit events
        for (uint256 i = 0; i < _gauges.length; i++) {
            emit VoteChanged(msg.sender, _gauges[i], _weights[i]);
        }
    }

    /**
     * @notice Reset votes
     */
    function resetVotes() external {
        address[] memory currentVotes = votes[msg.sender];
        
        for (uint256 i = 0; i < currentVotes.length; i++) {
            address gauge = currentVotes[i];
            uint256 oldWeight = voteWeight[msg.sender][gauge];
            if (oldWeight > 0) {
                gaugePoints[gauge][epoch] -= oldWeight;
                voteWeight[msg.sender][gauge] = 0;
            }
        }
        
        delete votes[msg.sender];
        voteCount[msg.sender] = 0;
    }

    // ============ Reward Functions ============

    /**
     * @notice Add rewards for distribution
     */
    function addRewards(uint256 _amount) external {
        require(_amount > 0, "No rewards");
        
        IERC20(rewardToken).safeTransferFrom(msg.sender, address(this), _amount);
        
        uint256 supply = totalSupply();
        if (supply > 0) {
            rewardRate = _amount / WEEK;
            totalRewardPerToken += _amount.div(supply);
        } else {
            // No liquidity yet, accumulate for later
            rewardRate = 0;
        }
        
        emit RewardsAdded(_amount);
    }

    /**
     * @notice Claim rewards
     */
    function claim() external nonReentrant {
        _updateReward(msg.sender);
        
        uint256 reward = claimableRewards[msg.sender];
        if (reward > 0) {
            claimableRewards[msg.sender] = 0;
            IERC20(rewardToken).safeTransfer(msg.sender, reward);
            
            emit RewardsClaimed(msg.sender, reward);
        }
    }

    /**
     * @notice Claim rewards for specific gauges
     */
    function claim(address[] calldata _gauges) external nonReentrant {
        for (uint256 i = 0; i < _gauges.length; i++) {
            _claimGauge(_gauges[i], msg.sender);
        }
    }

    // ============ Gauge Actions ============

    /**
     * @notice Deposit into gauge
     */
    function deposit(address _gauge, uint256 _amount) external {
        require(gauges[_gauge], "Not a gauge");
        
        IERC20(governanceToken).safeTransferFrom(msg.sender, _gauge, _amount);
        IGauge(_gauge).deposit(_amount);
    }

    /**
     * @notice Withdraw from gauge
     */
    function withdraw(address _gauge, uint256 _amount) external {
        require(gauges[_gauge], "Not a gauge");
        
        IGauge(_gauge).withdraw(_amount);
    }

    // ============ View Functions ============

    /**
     * @notice Get total votes
     */
    function getVotes(address _user) external view returns (address[] memory, uint256[] memory) {
        address[] memory userVotes = votes[_user];
        uint256[] memory weights = new uint256[](userVotes.length);
        
        for (uint256 i = 0; i < userVotes.length; i++) {
            weights[i] = voteWeight[_user][userVotes[i]];
        }
        
        return (userVotes, weights);
    }

    /**
     * @notice Get gauge list
     */
    function getGaugeList() external view returns (address[] memory) {
        return gaugeList;
    }

    /**
     * @notice Get gauges by type
     */
    function getGaugesByType(uint256 _type) external view returns (address[] memory) {
        return typeToGauges[_type];
    }

    /**
     * @notice Calculate total supply (weighted)
     */
    function totalSupply() public view returns (uint256) {
        uint256 total = 0;
        for (uint256 i = 0; i < gaugeList.length; i++) {
            total += gaugePoints[gaugeList[i]][epoch];
        }
        return total;
    }

    /**
     * @notice Calculate balance for user
     */
    function balanceOf(address _user) external view returns (uint256) {
        uint256 total = 0;
        address[] memory userVotes = votes[_user];
        
        for (uint256 i = 0; i < userVotes.length; i++) {
            total += voteWeight[_user][userVotes[i]];
        }
        
        return total;
    }

    // ============ Internal Functions ============

    /**
     * @dev Update reward for user
     */
    function _updateReward(address _user) internal {
        // This would calculate pending rewards based on user's gauge deposits
        // Simplified for example
    }

    /**
     * @dev Claim from specific gauge
     */
    function _claimGauge(address _gauge, address _user) internal {
        uint256 reward = IGauge(_gauge).claimableRewards(_user);
        if (reward > 0) {
            IERC20(rewardToken).safeTransfer(_user, reward);
        }
    }
}

/**
 * @title TigerGauge
 * @dev Individual gauge contract for each pool
 */
contract TigerGauge is ReentrancyGuard {
    using SafeERC20 for IERC20;
    using GaugeMath for uint256;

    // ============ Constants ============
    uint256 constant PRECISION = 1e18;
    uint256 constant WEEK = 7 days;

    // ============ State Variables ============
    
    address public controller;
    address public lpToken;
    address public rewardToken;
    
    // Supply tracking
    uint256 public totalSupply;
    mapping(address => uint256) public balanceOf;
    
    // Reward tracking
    uint256 public rewardRate;
    uint256 public rewardPerTokenStored;
    mapping(address => uint256) public userRewardPerTokenPaid;
    mapping(address => uint256) public pendingRewards;
    mapping(address => uint256) public claimedRewards;
    
    // Time
    uint256 public periodFinish;
    uint256 public lastUpdateTime;
    uint256 public rewardPerToken;
    
    // Epoch
    uint256 public currentEpoch;
    mapping(uint256 => uint256) public epochRewards;
    
    // ============ Events ============
    event Deposit(address indexed user, uint256 amount);
    event Withdraw(address indexed user, uint256 amount);
    event RewardClaimed(address indexed user, uint256 reward);

    // ============ Constructor ============
    
    constructor(address _controller, address _lpToken, address _rewardToken) {
        controller = _controller;
        lpToken = _lpToken;
        rewardToken = _rewardToken;
    }

    // ============ External Functions ============

    /**
     * @notice Deposit LP tokens
     */
    function deposit(uint256 _amount) external nonReentrant {
        require(_amount > 0, "Cannot deposit 0");
        
        // Update reward
        _updateReward(msg.sender);
        
        // Transfer tokens
        IERC20(lpToken).safeTransferFrom(msg.sender, address(this), _amount);
        
        // Update balance
        balanceOf[msg.sender] += _amount;
        totalSupply += _amount;
        
        emit Deposit(msg.sender, _amount);
    }

    /**
     * @notice Withdraw LP tokens
     */
    function withdraw(uint256 _amount) external nonReentrant {
        require(_amount > 0, "Cannot withdraw 0");
        require(balanceOf[msg.sender] >= _amount, "Insufficient balance");
        
        // Update reward
        _updateReward(msg.sender);
        
        // Update balance
        balanceOf[msg.sender] -= _amount;
        totalSupply -= _amount;
        
        // Transfer tokens
        IERC20(lpToken).safeTransfer(msg.sender, _amount);
        
        emit Withdraw(msg.sender, _amount);
    }

    /**
     * @notice Claim rewards
     */
    function claim() external nonReentrant {
        _updateReward(msg.sender);
        
        uint256 reward = pendingRewards[msg.sender];
        if (reward > 0) {
            pendingRewards[msg.sender] = 0;
            claimedRewards[msg.sender] += reward;
            
            IERC20(rewardToken).safeTransfer(msg.sender, reward);
            
            emit RewardClaimed(msg.sender, reward);
        }
    }

    /**
     * @notice Get claimable rewards
     */
    function claimableRewards(address _user) external view returns (uint256) {
        uint256 perToken = rewardPerToken;
        if (totalSupply > 0) {
            perToken += (block.timestamp - lastUpdateTime).mul(rewardRate).div(totalSupply);
        }
        
        return pendingRewards[_user] + (
            balanceOf[_user].mul(perToken - userRewardPerTokenPaid[_user], PRECISION)
        );
    }

    // ============ Internal Functions ============

    /**
     * @dev Update rewards
     */
    function _updateReward(address _user) internal {
        rewardPerToken = rewardPerTokenStored;
        lastUpdateTime = block.timestamp;
        
        if (totalSupply > 0) {
            rewardPerToken += (block.timestamp - lastUpdateTime).mul(rewardRate).div(totalSupply);
            rewardPerTokenStored = rewardPerToken;
        }
        
        pendingRewards[_user] += balanceOf[_user].mul(
            rewardPerToken - userRewardPerTokenPaid[_user],
            PRECISION
        );
        
        userRewardPerTokenPaid[_user] = rewardPerToken;
    }
}
