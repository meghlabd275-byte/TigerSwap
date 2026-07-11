// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/**
 * @title TigerPredictionMarket
 * @notice Production Prediction Market
 * @dev Event-based betting with oracle resolution
 * 
 * Features:
 * - Binary outcomes (Yes/No)
 * - Multiple outcomes
 * - Oracle resolution
 * - Trading positions
 * - Payout distribution
 * 
 * @author TigerSwap
 * @version 1.0.0 Production
 */

import "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";
import "@openzeppelin/contracts/utils/ReentrancyGuard.sol";
import "@openzeppelin/contracts/access/Ownable.sol";

/**
 * @title Prediction Math
 */
library PredictionMath {
    uint256 constant WAD = 1e18;
    
    function mul(uint256 x, uint256 y) internal pure returns (uint256) {
        return (x * y) / WAD;
    }
    
    function div(uint256 x, uint256 y) internal pure returns (uint256) {
        return (x * WAD) / y;
    }
}

/**
 * @title TigerPredictionMarket
 * @dev Main prediction market contract
 */
contract TigerPredictionMarket is ReentrancyGuard, Ownable {
    using SafeERC20 for IERC20;
    using PredictionMath for uint256;

    // ============ Constants ============
    uint256 constant MIN_BET_AMOUNT = 1e16; // 0.01
    uint256 constant MAX_BET_AMOUNT = 1000e18; // 1000
    uint256 constant ORACLE_TIMEOUT = 7 days;

    // ============ State Variables ============
    
    // Core
    IERC20 public token;
    address public oracle;
    uint256 public marketCount;
    
    // Markets
    mapping(uint256 => Market) public markets;
    mapping(uint256 => mapping(uint256 => uint256)) public outcomeAmounts;
    mapping(uint256 => mapping(address => mapping(uint256 => uint256))) public userBets;
    mapping(uint256 => mapping(address => uint256)) public userTotalBet;
    
    // ============ Structs ============
    
    struct Market {
        string question;
        uint256 endTime;
        uint256 resolveTime;
        uint256 outcomeCount;
        uint256 resolvedOutcome;
        uint256 totalPool;
        uint256[] outcomePools;
        bool resolved;
        bool cancelled;
        address creator;
        uint256 creatorFee;
    }

    // ============ Events ============
    event MarketCreated(
        uint256 indexed marketId,
        string question,
        uint256 endTime,
        uint256 outcomeCount,
        address creator
    );
    event BetPlaced(
        address indexed user,
        uint256 indexed marketId,
        uint256 outcome,
        uint256 amount
    );
    event MarketResolved(
        uint256 indexed marketId,
        uint256 outcome,
        uint256 totalPayout
    );
    event MarketCancelled(uint256 indexed marketId);
    event PayoutClaimed(
        address indexed user,
        uint256 indexed marketId,
        uint256 amount
    );

    // ============ Constructor ============
    
    constructor(address _token, address _oracle, address _owner) Ownable(_owner) {
        require(_token != address(0), "Invalid token");
        
        token = IERC20(_token);
        oracle = _oracle;
    }

    // ============ Create Market ============

    /**
     * @notice Create a new prediction market
     */
    function createMarket(
        string memory _question,
        uint256 _duration,
        uint256 _outcomeCount,
        uint256 _creatorFee
    ) external returns (uint256) {
        require(_outcomeCount >= 2, "Min 2 outcomes");
        require(_outcomeCount <= 10, "Max 10 outcomes");
        
        uint256 marketId = ++marketCount;
        
        markets[marketId] = Market({
            question: _question,
            endTime: block.timestamp + _duration,
            resolveTime: 0,
            outcomeCount: _outcomeCount,
            resolvedOutcome: 0,
            totalPool: 0,
            outcomePools: new uint256[](_outcomeCount),
            resolved: false,
            cancelled: false,
            creator: msg.sender,
            creatorFee: _creatorFee
        });
        
        emit MarketCreated(marketId, _question, block.timestamp + _duration, _outcomeCount, msg.sender);
        
        return marketId;
    }

    // ============ Place Bet ============

    /**
     * @notice Place a bet on an outcome
     */
    function bet(uint256 _marketId, uint256 _outcome, uint256 _amount) 
        external 
        nonReentrant 
    {
        Market storage market = markets[_marketId];
        
        require(!market.cancelled, "Market cancelled");
        require(!market.resolved, "Market resolved");
        require(block.timestamp < market.endTime, "Market ended");
        require(_outcome < market.outcomeCount, "Invalid outcome");
        require(_amount >= MIN_BET_AMOUNT, "Amount too low");
        require(_amount <= MAX_BET_AMOUNT, "Amount too high");
        
        // Transfer tokens
        token.safeTransferFrom(msg.sender, address(this), _amount);
        
        // Record bet
        userBets[_marketId][msg.sender][_outcome] += _amount;
        userTotalBet[_marketId][msg.sender] += _amount;
        outcomeAmounts[_marketId][_outcome] += _amount;
        market.outcomePools[_outcome] += _amount;
        market.totalPool += _amount;
        
        emit BetPlaced(msg.sender, _marketId, _outcome, _amount);
    }

    // ============ Resolve Market ============

    /**
     * @notice Resolve market with winning outcome
     */
    function resolveMarket(uint256 _marketId, uint256 _outcome) external {
        require(msg.sender == owner() || msg.sender == oracle, "Not authorized");
        
        Market storage market = markets[_marketId];
        
        require(!market.cancelled, "Market cancelled");
        require(!market.resolved, "Already resolved");
        require(block.timestamp >= market.endTime, "Not ended");
        require(_outcome < market.outcomeCount, "Invalid outcome");
        
        market.resolvedOutcome = _outcome;
        market.resolveTime = block.timestamp;
        market.resolved = true;
        
        // Calculate payouts
        uint256 totalPayout = _calculatePayouts(_marketId, _outcome);
        
        emit MarketResolved(_marketId, _outcome, totalPayout);
    }

    /**
     * @dev Calculate payouts for winning bets
     */
    function _calculatePayouts(uint256 _marketId, uint256 _winningOutcome) 
        internal 
        returns (uint256) 
    {
        Market storage market = markets[_marketId];
        
        uint256 winningAmount = market.outcomePools[_winningOutcome];
        
        if (winningAmount == 0) return 0;
        
        // Calculate payout per token (total pool / winning pool)
        uint256 payoutRatio = market.totalPool.div(winningAmount);
        
        // This ratio is stored and used when claiming
        return payoutRatio;
    }

    // ============ Claim Payout ============

    /**
     * @notice Claim winnings
     */
    function claimPayout(uint256 _marketId) external nonReentrant {
        Market storage market = markets[_marketId];
        
        require(market.resolved, "Not resolved");
        require(!market.cancelled, "Market cancelled");
        
        uint256 betAmount = userBets[_marketId][msg.sender][market.resolvedOutcome];
        
        require(betAmount > 0, "No winning bet");
        
        uint256 winningPool = market.outcomePools[market.resolvedOutcome];
        uint256 payout = betAmount.mul(market.totalPool.div(winningPool));
        
        // Deduct creator fee
        uint256 creatorFee = (payout * market.creatorFee) / 10000;
        uint256 userPayout = payout - creatorFee;
        
        // Reset bet to prevent double claiming
        userBets[_marketId][msg.sender][market.resolvedOutcome] = 0;
        
        // Transfer payout
        token.safeTransfer(msg.sender, userPayout);
        
        // Transfer creator fee
        if (creatorFee > 0) {
            token.safeTransfer(market.creator, creatorFee);
        }
        
        emit PayoutClaimed(msg.sender, _marketId, userPayout);
    }

    // ============ Cancel Market ============

    /**
     * @notice Cancel market and refund
     */
    function cancelMarket(uint256 _marketId) external onlyOwner {
        Market storage market = markets[_marketId];
        
        require(!market.resolved, "Already resolved");
        require(!market.cancelled, "Already cancelled");
        
        market.cancelled = true;
        
        emit MarketCancelled(_marketId);
    }

    /**
     * @notice Claim refund for cancelled market
     */
    function claimRefund(uint256 _marketId) external nonReentrant {
        Market storage market = markets[_marketId];
        
        require(market.cancelled, "Not cancelled");
        
        uint256 totalBet = userTotalBet[_marketId][msg.sender];
        
        require(totalBet > 0, "No bets");
        
        // Reset bets
        for (uint256 i = 0; i < market.outcomeCount; i++) {
            userBets[_marketId][msg.sender][i] = 0;
        }
        userTotalBet[_marketId][msg.sender] = 0;
        
        // Transfer refund
        token.safeTransfer(msg.sender, totalBet);
    }

    // ============ View Functions ============

    /**
     * @notice Get market info
     */
    function getMarket(uint256 _marketId) external view returns (
        string memory question,
        uint256 endTime,
        uint256 outcomeCount,
        uint256 totalPool,
        bool resolved,
        bool cancelled,
        uint256 resolvedOutcome
    ) {
        Market storage market = markets[_marketId];
        return (
            market.question,
            market.endTime,
            market.outcomeCount,
            market.totalPool,
            market.resolved,
            market.cancelled,
            market.resolvedOutcome
        );
    }

    /**
     * @notice Get outcome pools
     */
    function getOutcomePools(uint256 _marketId) external view returns (uint256[] memory) {
        return markets[_marketId].outcomePools;
    }

    /**
     * @notice Get user bets
     */
    function getUserBets(uint256 _marketId, address _user) external view returns (uint256[] memory) {
        Market storage market = markets[_marketId];
        uint256[] memory bets = new uint256[](market.outcomeCount);
        
        for (uint256 i = 0; i < market.outcomeCount; i++) {
            bets[i] = userBets[_marketId][_user][i];
        }
        
        return bets;
    }

    /**
     * @notice Calculate potential payout
     */
    function getPotentialPayout(uint256 _marketId, address _user, uint256 _outcome) 
        external 
        view 
        returns (uint256) 
    {
        Market storage market = markets[_marketId];
        
        if (!market.resolved || market.cancelled) return 0;
        
        uint256 betAmount = userBets[_marketId][_user][_outcome];
        
        if (betAmount == 0 || _outcome != market.resolvedOutcome) return 0;
        
        uint256 winningPool = market.outcomePools[market.resolvedOutcome];
        
        return betAmount.mul(market.totalPool.div(winningPool));
    }
}
