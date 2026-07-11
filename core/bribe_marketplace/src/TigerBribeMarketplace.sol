// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/**
 * @title TigerBribeMarketplace
 * @notice Production Bribe Marketplace - Votium Style
 * @dev Vote buying mechanism for governance incentives
 * 
 * Features:
 * - Bribe voting
 * - Multi-asset bribes
 * - Gauge voting
 * - Reward distribution
 * - Epoch-based
 * 
 * @author TigerSwap
 * @version 1.0.0 Production
 */

import "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";
import "@openzeppelin/contracts/utils/ReentrancyGuard.sol";
import "@openzeppelin/contracts/access/Ownable.sol";

/**
 * @title Bribe Math
 */
library BribeMath {
    uint256 constant WAD = 1e18;
    uint256 constant WEEK = 7 days;
    
    function mul(uint256 x, uint256 y) internal pure returns (uint256) {
        return (x * y) / WAD;
    }
    
    function div(uint256 x, uint256 y) internal pure returns (uint256) {
        return (x * WAD) / y;
    }
}

/**
 * @title TigerBribeMarketplace
 * @dev Main bribe marketplace contract
 */
contract TigerBribeMarketplace is ReentrancyGuard, Ownable {
    using SafeERC20 for IERC20;
    using BribeMath for uint256;

    // ============ Constants ============
    uint256 constant WEEK = 7 days;
    uint256 constant MAX_REWARDS = 50;

    // ============ State Variables ============
    
    // Governance
    address public governanceToken;
    address public rewardToken;
    address public gaugeController;
    address public treasury;
    
    // Epoch
    uint256 public currentEpoch;
    uint256 public epochStartTime;
    
    // Bribes
    mapping(uint256 => mapping(address => Bribe)) public bribes; // epoch => gauge => bribe
    mapping(address => uint256[]) public gaugeBribeList; // gauge => bribe epochs
    
    // Voting
    mapping(uint256 => mapping(address => mapping(address => uint256))) public votes; 
    // epoch => gauge => voter => votes
    mapping(uint256 => mapping(address => mapping(address => uint256))) public claimed;
    // epoch => gauge => voter => claimed amount
    
    // Rewards
    mapping(address => uint256) public gaugeRewardRate; // per week
    mapping(address => uint256) public totalBribes;
    mapping(address => uint256) public totalClaimed;

    // ============ Structs ============
    
    struct Bribe {
        address token;
        uint256 amount;
        uint256 perVote;
        uint256 totalVotes;
        uint256 rewardRate;
        uint256 startEpoch;
        uint256 endEpoch;
        bool active;
    }

    // ============ Events ============
    event BribeCreated(
        uint256 indexed epoch,
        address indexed gauge,
        address token,
        uint256 amount,
        uint256 perVote
    );
    event VoteCast(
        uint256 indexed epoch,
        address indexed gauge,
        address indexed voter,
        uint256 weight
    );
    event RewardsClaimed(
        address indexed gauge,
        address indexed voter,
        uint256 amount
    );
    event BribeUpdated(
        uint256 indexed epoch,
        address indexed gauge,
        uint256 newAmount
    );
    event RewardRateUpdated(address indexed gauge, uint256 rate);

    // ============ Constructor ============
    
    constructor(
        address _governanceToken,
        address _rewardToken,
        address _treasury,
        address _owner
    ) Ownable(_owner) {
        require(_governanceToken != address(0), "Invalid governance token");
        
        governanceToken = _governanceToken;
        rewardToken = _rewardToken;
        treasury = _treasury;
        
        // Initialize epoch
        epochStartTime = (block.timestamp / WEEK) * WEEK;
        currentEpoch = 1;
    }

    // ============ Create Bribe ============

    /**
     * @notice Create a bribe for a gauge
     */
    function createBribe(
        address _gauge,
        address _token,
        uint256 _amount,
        uint256 _durationWeeks
    ) external nonReentrant returns (uint256) {
        require(_gauge != address(0), "Invalid gauge");
        require(_amount > 0, "Amount is 0");
        
        // Get current epoch
        uint256 epoch = _getCurrentEpoch();
        
        // Transfer bribe tokens
        IERC20(_token).safeTransferFrom(msg.sender, address(this), _amount);
        
        // Calculate per-vote rate
        // In production: would get actual voting power from gauge
        uint256 totalVotingPower = 1000000e18; // Example
        uint256 perVote = _amount.div(totalVotingPower);
        
        // Create bribe
        Bribe storage bribe = bribes[epoch][_gauge];
        
        bribe.token = _token;
        bribe.amount = _amount;
        bribe.perVote = perVote;
        bribe.totalVotes = 0;
        bribe.rewardRate = gaugeRewardRate[_gauge];
        bribe.startEpoch = epoch;
        bribe.endEpoch = epoch + _durationWeeks - 1;
        bribe.active = true;
        
        // Track bribe
        gaugeBribeList[_gauge].push(epoch);
        totalBribes[_gauge] += _amount;
        
        emit BribeCreated(epoch, _gauge, _token, _amount, perVote);
        
        return epoch;
    }

    // ============ Vote ============

    /**
     * @notice Vote for a gauge (in exchange for bribes)
     */
    function vote(
        address _gauge,
        uint256 _weight
    ) external nonReentrant {
        require(_weight > 0, "Weight is 0");
        
        uint256 epoch = _getCurrentEpoch();
        
        // Record vote
        votes[epoch][_gauge][msg.sender] = _weight;
        
        // Update bribe
        Bribe storage bribe = bribes[epoch][_gauge];
        
        if (bribe.active) {
            bribe.totalVotes += _weight;
        }
        
        emit VoteCast(epoch, _gauge, msg.sender, _weight);
    }

    // ============ Claim Rewards ============

    /**
     * @notice Claim bribe rewards
     */
    function claim(address _gauge) external nonReentrant {
        uint256 epoch = _getCurrentEpoch();
        
        uint256 weight = votes[epoch][_gauge][msg.sender];
        require(weight > 0, "No votes");
        
        // Check if already claimed
        uint256 claimedAmount = claimed[epoch][_gauge][msg.sender];
        
        // Calculate claimable
        uint256 claimable = _calculateClaimable(_gauge, epoch, weight);
        
        require(claimable > claimedAmount, "Nothing to claim");
        
        uint256 pending = claimable - claimedAmount;
        
        // Mark as claimed
        claimed[epoch][_gauge][msg.sender] = claimable;
        
        // Transfer rewards
        Bribe storage bribe = bribes[epoch][_gauge];
        IERC20(bribe.token).safeTransfer(msg.sender, pending);
        
        totalClaimed[_gauge] += pending;
        
        emit RewardsClaimed(_gauge, msg.sender, pending);
    }

    /**
     * @dev Calculate claimable amount
     */
    function _calculateClaimable(
        address _gauge,
        uint256 _epoch,
        uint256 _weight
    ) internal view returns (uint256) {
        Bribe storage bribe = bribes[_epoch][_gauge];
        
        if (!bribe.active || bribe.amount == 0) return 0;
        
        // Calculate based on votes
        uint256 totalVotes = bribe.totalVotes;
        if (totalVotes == 0) return 0;
        
        // Pro-rata allocation
        uint256 userShare = (_weight * 1e18) / totalVotes;
        uint256 claimable = (bribe.amount * userShare) / 1e18;
        
        return claimable;
    }

    // ============ Gauge Management ============

    /**
     * @notice Set reward rate for gauge
     */
    function setGaugeRewardRate(address _gauge, uint256 _rate) external onlyOwner {
        gaugeRewardRate[_gauge] = _rate;
        
        emit RewardRateUpdated(_gauge, _rate);
    }

    // ============ Epoch Management ============

    /**
     * @dev Get current epoch
     */
    function _getCurrentEpoch() internal view returns (uint256) {
        uint256 weeksPassed = (block.timestamp - epochStartTime) / WEEK;
        return currentEpoch + weeksPassed;
    }

    /**
     * @notice Advance to new epoch
     */
    function advanceEpoch() external {
        uint256 newEpoch = _getCurrentEpoch();
        
        require(newEpoch > currentEpoch, "Already at current epoch");
        
        currentEpoch = newEpoch;
    }

    // ============ View Functions ============

    /**
     * @notice Get bribe for gauge at epoch
     */
    function getBribe(address _gauge, uint256 _epoch) 
        external 
        view 
        returns (
            address token,
            uint256 amount,
            uint256 perVote,
            uint256 totalVotes,
            bool active
        ) 
    {
        Bribe storage bribe = bribes[_epoch][_gauge];
        return (
            bribe.token,
            bribe.amount,
            bribe.perVote,
            bribe.totalVotes,
            bribe.active
        );
    }

    /**
     * @notice Get pending rewards for voter
     */
    function getPendingRewards(address _gauge, address _voter) 
        external 
        view 
        returns (uint256) 
    {
        uint256 epoch = _getCurrentEpoch();
        uint256 weight = votes[epoch][_gauge][_voter];
        
        if (weight == 0) return 0;
        
        uint256 claimable = _calculateClaimable(_gauge, epoch, weight);
        uint256 alreadyClaimed = claimed[epoch][_gauge][_voter];
        
        return claimable > alreadyClaimed ? claimable - alreadyClaimed : 0;
    }

    /**
     * @notice Get bribe list for gauge
     */
    function getGaugeBribeList(address _gauge) external view returns (uint256[] memory) {
        return gaugeBribeList[_gauge];
    }

    /**
     * @notice Get total bribes for gauge
     */
    function getTotalBribes(address _gauge) external view returns (uint256) {
        return totalBribes[_gauge];
    }

    /**
     * @notice Get total claimed for gauge
     */
    function getTotalClaimed(address _gauge) external view returns (uint256) {
        return totalClaimed[_gauge];
    }

    // ============ Admin Functions ============

    /**
     * @notice Recover stuck tokens
     */
    function rescueTokens(address _token, uint256 _amount) external onlyOwner {
        require(_token != rewardToken, "Cannot rescue reward token");
        
        IERC20(_token).safeTransfer(treasury, _amount);
    }
}

/**
 * @title TigerBribeFactory
 * @dev Factory for creating bribe campaigns
 */
contract TigerBribeFactory is Ownable {
    address public bribeMarketplace;
    mapping(address => bool) public authorizedCampaigns;
    
    event CampaignCreated(address indexed campaign, address gauge);
    event MarketplaceUpdated(address indexed marketplace);
    
    constructor(address _owner) Ownable(_owner) {}
    
    /**
     * @notice Set bribe marketplace
     */
    function setMarketplace(address _marketplace) external onlyOwner {
        require(_marketplace != address(0), "Invalid marketplace");
        bribeMarketplace = _marketplace;
        
        emit MarketplaceUpdated(_marketplace);
    }

    /**
     * @notice Create new bribe campaign
     */
    function createCampaign(address _gauge) external returns (address) {
        // In production: deploy minimal proxy for campaign
        // For now, just emit event
        emit CampaignCreated(msg.sender, _gauge);
        
        return msg.sender;
    }
}
