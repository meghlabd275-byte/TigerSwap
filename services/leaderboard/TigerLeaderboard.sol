// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";
import "@openzeppelin/contracts/access/AccessControl.sol";
import "@openzeppelin/contracts/utils/ReentrancyGuard.sol";

/**
 * @title TigerLeaderboard
 * @notice Trading competition leaderboard system
 */

contract TigerLeaderboard is AccessControl, ReentrancyGuard {
    using SafeERC20 for IERC20;
    
    bytes32 public constant WARDEN_ROLE = keccak256("WARDEN_ROLE");
    bytes32 public constant JUDGLE_ROLE = keccak256("JUDGLE_ROLE");
    
    // Competition types
    enum CompetitionType { TradingVolume, PnL, MostTrades, BestROI }
    enum CompetitionStatus { Upcoming, Active, Completed }
    
    // State
    uint256 public competitionCount;
    uint256 public nextCompetitionId;
    IERC20 public rewardToken;
    uint256 public totalRewards;
    
    // Competitions
    mapping(uint256 => Competition) public competitions;
    mapping(uint256 => mapping(address => Competitor)) public competitors;
    mapping(uint256 => address[]) public competitorList;
    
    // Events
    event CompetitionCreated(uint256 indexed id, string name, CompetitionType compType, uint256 startTime, uint256 endTime);
    event CompetitionStarted(uint256 indexed id);
    event CompetitionEnded(uint256 indexed id);
    event CompetitorJoined(uint256 indexed compId, address indexed user, uint256 volume);
    event RankUpdated(uint256 indexed compId, address indexed user, uint256 newRank);
    event RewardClaimed(uint256 indexed compId, address indexed user, uint256 amount);
    event RewardsAdded(uint256 amount);
    
    struct Competition {
        uint256 id;
        string name;
        string description;
        CompetitionType compType;
        CompetitionStatus status;
        uint256 startTime;
        uint256 endTime;
        uint256 rewardPool;
        uint256 topN;
        bool rewardsDistributed;
    }
    
    struct Competitor {
        uint256 volume;      // Trading volume
        int256 pnl;        // Profit/Loss
        uint256 tradeCount;  // Number of trades
        uint256 volumeROI; // ROI calculation
        uint256 rank;
        uint256 score;
        bool claimed;
    }
    
    modifier onlyJudges() {
        require(hasRole(JUDGLE_ROLE, msg.sender), "Not judge");
        _;
    }
    
    constructor(address _admin, address _rewardToken) {
        require(_rewardToken != address(0), "Invalid reward token");
        
        rewardToken = IERC20(_rewardToken);
        
        _grantRole(DEFAULT_ADMIN_ROLE, _admin);
        _grantRole(WARDEN_ROLE, _admin);
        _grantRole(JUDGLE_ROLE, _admin);
    }
    
    /**
     * @notice Create new competition
     * @param name Competition name
     * @param description Competition description
     * @param compType Competition type
     * @param startTime Start timestamp
     * @param endTime End timestamp
     * @param rewardPool Total rewards
     * @param topN Number of top competitors to reward
     */
    function createCompetition(
        string memory name,
        string memory description,
        CompetitionType compType,
        uint256 startTime,
        uint256 endTime,
        uint256 rewardPool,
        uint256 topN
    ) external onlyRole(DEFAULT_ADMIN_ROLE) returns (uint256) {
        require(startTime > block.timestamp, "Start time passed");
        require(endTime > startTime, "Invalid end time");
        
        uint256 id = ++competitionCount;
        
        competitions[id] = Competition({
            id: id,
            name: name,
            description: description,
            compType: compType,
            status: CompetitionStatus.Upcoming,
            startTime: startTime,
            endTime: endTime,
            rewardPool: rewardPool,
            topN: topN,
            rewardsDistributed: false
        });
        
        emit CompetitionCreated(id, name, compType, startTime, endTime);
        
        return id;
    }
    
    /**
     * @notice Start competition
     * @param id Competition ID
     */
    function startCompetition(uint256 id) external onlyRole(DEFAULT_ADMIN_ROLE) {
        Competition storage comp = competitions[id];
        require(comp.status == CompetitionStatus.Upcoming, "Not upcoming");
        
        comp.status = CompetitionStatus.Active;
        
        emit CompetitionStarted(id);
    }
    
    /**
     * @notice End competition
     * @param id Competition ID
     */
    function endCompetition(uint256 id) external onlyRole(DEFAULT_ADMIN_ROLE) {
        Competition storage comp = competitions[id];
        require(comp.status == CompetitionStatus.Active, "Not active");
        require(block.timestamp >= comp.endTime, "Not ended");
        
        comp.status = CompetitionStatus.Completed;
        
        // Sort and rank competitors
        _sortAndRank(id);
        
        emit CompetitionEnded(id);
    }
    
    /**
     * @notice Join competition
     * @param id Competition ID
     */
    function joinCompetition(uint256 id) external {
        Competition storage comp = competitions[id];
        require(comp.status == CompetitionStatus.Active, "Not active");
        require(block.timestamp >= comp.startTime, "Not started");
        require(block.timestamp <= comp.endTime, "Ended");
        
        Competitor storage user = competitors[id][msg.sender];
        require(user.rank == 0, "Already joined");
        
        // Add to competitor list
        competitorList[id].push(msg.sender);
        
        // Initialize
        user.volume = 0;
        user.pnl = 0;
        user.tradeCount = 0;
        user.volumeROI = 0;
        user.rank = 0;
        user.score = 0;
        user.claimed = false;
        
        emit CompetitorJoined(id, msg.sender, 0);
    }
    
    /**
     * @notice Update competitor stats
     * @param id Competition ID
     * @param user User address
     * @param volume Trading volume
     * @param pnl Profit/Loss
     * @param tradeCount Number of trades
     */
    function updateStats(
        uint256 id,
        address user,
        uint256 volume,
        int256 pnl,
        uint256 tradeCount
    ) external onlyJudges {
        Competition storage comp = competitions[id];
        require(comp.status == CompetitionStatus.Active, "Not active");
        
        Competitor storage competitor = competitors[id][user];
        
        competitor.volume += volume;
        competitor.pnl += pnl;
        competitor.tradeCount += tradeCount;
        
        // Recalculate score based on type
        if (comp.compType == CompetitionType.TradingVolume) {
            competitor.score = competitor.volume;
        } else if (comp.compType == CompetitionType.PnL) {
            competitor.score = competitor.pnl > 0 ? uint256(competitor.pnl) : 0;
        } else if (comp.compType == CompetitionType.MostTrades) {
            competitor.score = competitor.tradeCount;
        } else if (comp.compType == CompetitionType.BestROI) {
            // ROI = (pnl / volume) * 1e18
            if (competitor.volume > 0) {
                competitor.volumeROI = (uint256(competitor.pnl > 0 ? competitor.pnl : -competitor.pnl) * 1e18) / competitor.volume;
            }
            competitor.score = competitor.volumeROI;
        }
    }
    
    /**
     * @notice Claim rewards
     * @param id Competition ID
     */
    function claimRewards(uint256 id) external nonReentrant {
        Competition storage comp = competitions[id];
        require(comp.status == CompetitionStatus.Completed, "Not completed");
        require(!comp.rewardsDistributed, "Rewards distributed");
        
        Competitor storage competitor = competitors[id][msg.sender];
        require(competitor.rank > 0 && competitor.rank <= comp.topN, "Not in top");
        require(!competitor.claimed, "Already claimed");
        
        // Calculate reward
        uint256 rewardPerPlace = comp.rewardPool / comp.topN;
        uint256 reward = rewardPerPlace;
        
        // Higher ranks get more
        if (competitor.rank == 1) {
            reward = (comp.rewardPool * 30) / 100; // Top 30%
        } else if (competitor.rank <= 3) {
            reward = (comp.rewardPool * 20) / 100; // Top 3 get 20%
        } else if (competitor.rank <= 10) {
            reward = (comp.rewardPool * 10) / 100; // Top 10 get 10%
        }
        
        competitor.claimed = true;
        
        // Transfer rewards
        rewardToken.safeTransfer(msg.sender, reward);
        
        emit RewardClaimed(id, msg.sender, reward);
    }
    
    /**
     * @notice Add rewards
     * @param amount Amount to add
     */
    function addRewards(uint256 amount) external onlyRole(DEFAULT_ADMIN_ROLE) {
        require(amount > 0, "Amount is 0");
        
        rewardToken.safeTransferFrom(msg.sender, address(this), amount);
        totalRewards += amount;
        
        emit RewardsAdded(amount);
    }
    
    /**
     * @notice Get competitor info
     * @param id Competition ID
     * @param user User address
     * @return Competitor struct
     */
    function getCompetitor(uint256 id, address user) external view returns (Competitor memory) {
        return competitors[id][user];
    }
    
    /**
     * @notice Get competition list size
     * @param id Competition ID
     * @return Number of competitors
     */
    function getCompetitorCount(uint256 id) external view returns (uint256) {
        return competitorList[id].length;
    }
    
    // Internal functions
    
    function _sortAndRank(uint256 id) internal {
        Competition storage comp = competitions[id];
        address[] storage list = competitorList[id];
        
        // Simple bubble sort (in production use proper sorting)
        for (uint256 i = 0; i < list.length; i++) {
            for (uint256 j = i + 1; j < list.length; j++) {
                Competitor storage a = competitors[id][list[i]];
                Competitor storage b = competitors[id][list[j]];
                
                if (a.score < b.score) {
                    // Swap
                    address temp = list[i];
                    list[i] = list[j];
                    list[j] = temp;
                }
            }
        }
        
        // Assign ranks
        for (uint256 i = 0; i < list.length; i++) {
            competitors[id][list[i]].rank = i + 1;
            emit RankUpdated(id, list[i], i + 1);
        }
    }
}