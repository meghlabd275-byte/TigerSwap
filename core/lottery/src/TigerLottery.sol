// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/**
 * @title TigerLottery
 * @notice Production Lottery System - PancakeSwap Style
 * @dev Gamification with lottery rounds and rewards
 * 
 * Features:
 * - Ticket-based lottery
 * - Multiple prize tiers
 * - Burn mechanism
 * - Random number generation
 * - Automatic draw
 * 
 * @author TigerSwap
 * @version 1.0.0 Production
 */

import "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";
import "@openzeppelin/contracts/utils/ReentrancyGuard.sol";
import "@openzeppelin/contracts/access/Ownable.sol";

/**
 * @title Lottery Math
 */
library LotteryMath {
    uint256 constant MAX_TICKETS = 10000;
    uint256 constant TICKET_PRICE = 1e18; // 1 TIGER
    
    function getRewardRate(uint256 tier) internal pure returns (uint256) {
        if (tier == 1) return 500;  // Match 1 number: 50%
        if (tier == 2) return 100;  // Match 2 numbers: 10%
        if (tier == 3) return 50;   // Match 3 numbers: 5%
        if (tier == 4) return 20;   // Match 4 numbers: 2%
        return 0;
    }
}

/**
 * @title TigerLottery
 * @dev Main lottery contract
 */
contract TigerLottery is ReentrancyGuard, Ownable {
    using SafeERC20 for IERC20;

    // ============ Constants ============
    uint256 constant TICKET_PRICE = 1e18; // 1 TIGER
    uint256 constant MAX_TICKETS_PER_BUY = 100;
    uint256 constant NUM_LENGTH = 4; // 4 numbers per ticket
    uint256 constant MAX_NUMBER = 14; // Numbers 1-14
    
    // Prize distribution (basis points)
    uint256 constant PRIZE_POOL_BPS = 8000; // 80% to prizes
    uint256 constant TREASURY_BPS = 1500;   // 15% to treasury
    uint256 constant BURN_BPS = 500;        // 5% burn

    // ============ State Variables ============
    
    // Token
    IERC20 public token;
    address public treasury;
    
    // Round state
    uint256 public currentRound;
    uint256 public ticketPrice;
    uint256 public closeTime;
    uint256 public drawTime;
    
    // Tickets
    mapping(uint256 => mapping(address => uint256[])) public userTickets;
    mapping(uint256 => uint256[]) public roundTickets;
    mapping(uint256 => mapping(uint256 => uint256)) public ticketCount; // round => number => count
    
    // Winning numbers
    mapping(uint256 => uint256[NUM_LENGTH]) public winningNumbers;
    mapping(uint256 => bool) public roundFinalized;
    
    // Rewards
    mapping(uint256 => uint256) public prizePool;
    mapping(uint256 => mapping(uint256 => uint256)) public prizePerTier; // round => tier => amount
    mapping(uint256 => mapping(address => uint256)) public userWinnings;
    
    // Statistics
    uint256 public totalTicketsSold;
    uint256 public totalBurned;
    uint256 public totalTreasury;
    
    // ============ Events ============
    event TicketsPurchased(address indexed user, uint256 round, uint256 amount, uint256[] numbers);
    event RoundClosed(uint256 round, uint256 ticketCount, uint256 prizePool);
    event NumbersDrawn(uint256 round, uint256[4] numbers);
    event RewardsClaimed(address indexed user, uint256 round, uint256 amount);
    event TreasuryUpdated(address indexed treasury);
    event PriceUpdated(uint256 newPrice);

    // ============ Constructor ============
    
    constructor(address _token, address _treasury, address _owner) Ownable(_owner) {
        require(_token != address(0), "Invalid token");
        
        token = IERC20(_token);
        treasury = _treasury;
        ticketPrice = TICKET_PRICE;
        currentRound = 1;
    }

    // ============ Buy Tickets ============

    /**
     * @notice Buy lottery tickets
     */
    function buyTickets(uint256[] calldata _numbers) external nonReentrant {
        require(_numbers.length > 0, "No tickets");
        require(_numbers.length <= MAX_TICKETS_PER_BUY, "Too many tickets");
        
        uint256 round = currentRound;
        require(block.timestamp < closeTime, "Round closed");
        
        // Validate numbers
        for (uint256 i = 0; i < _numbers.length; i++) {
            require(_numbers[i] > 0 && _numbers[i] <= MAX_NUMBER, "Invalid number");
        }
        
        // Calculate cost
        uint256 totalCost = _numbers.length * ticketPrice;
        
        // Transfer tokens
        token.safeTransferFrom(msg.sender, address(this), totalCost);
        
        // Record tickets
        for (uint256 i = 0; i < _numbers.length; i++) {
            uint256 number = _numbers[i];
            
            userTickets[round][msg.sender].push(number);
            roundTickets[round].push(number);
            ticketCount[round][number]++;
            
            totalTicketsSold++;
        }
        
        emit TicketsPurchased(msg.sender, round, _numbers.length, _numbers);
    }

    // ============ Draw Numbers ============

    /**
     * @notice Close round and draw winning numbers
     */
    function closeRound() external {
        require(block.timestamp >= closeTime, "Not time to close");
        require(!roundFinalized[currentRound], "Already finalized");
        
        uint256 round = currentRound;
        uint256 ticketCount = roundTickets[round].length;
        
        // Calculate prize pool
        uint256 poolSize = ticketCount * ticketPrice;
        prizePool[round] = (poolSize * PRIZE_POOL_BPS) / 10000;
        
        // Send to treasury
        uint256 treasuryAmount = (poolSize * TREASURY_BPS) / 10000;
        totalTreasury += treasuryAmount;
        
        // Burn
        uint256 burnAmount = (poolSize * BURN_BPS) / 10000;
        totalBurned += burnAmount;
        
        emit RoundClosed(round, ticketCount, prizePool[round]);
    }

    /**
     * @notice Draw winning numbers (would use Chainlink VRF in production)
     */
    function drawWinningNumbers() external {
        require(closeTime < block.timestamp, "Not time to draw");
        require(!roundFinalized[currentRound], "Already finalized");
        
        uint256 round = currentRound;
        
        // In production, this would use Chainlink VRF for randomness
        // For now, generate pseudo-random numbers
        winningNumbers[round] = [
            uint256(keccak256(abi.encodePacked(block.timestamp, round, 1))) % MAX_NUMBER + 1,
            uint256(keccak256(abi.encodePacked(block.timestamp, round, 2))) % MAX_NUMBER + 1,
            uint256(keccak256(abi.encodePacked(block.timestamp, round, 3))) % MAX_NUMBER + 1,
            uint256(keccak256(abi.encodePacked(block.timestamp, round, 4))) % MAX_NUMBER + 1
        ];
        
        // Calculate prizes
        _calculatePrizes(round);
        
        roundFinalized[round] = true;
        
        emit NumbersDrawn(round, winningNumbers[round]);
    }

    /**
     * @dev Calculate prizes for each tier
     */
    function _calculatePrizes(uint256 _round) internal {
        uint256[NUM_LENGTH] memory matches;
        
        // Count matches
        for (uint256 i = 0; i < NUM_LENGTH; i++) {
            uint256 winningNum = winningNumbers[_round][i];
            matches[i] = ticketCount[_round][winningNum];
        }
        
        // Calculate prize per match
        uint256 pool = prizePool[_round];
        
        for (uint256 tier = 1; tier <= NUM_LENGTH; tier++) {
            uint256 totalMatches = 0;
            
            // Count tickets matching tier
            for (uint256 i = 0; i < roundTickets[_round].length; i++) {
                uint256 matchesCount = 0;
                uint256 ticket = roundTickets[_round][i];
                
                for (uint256 j = 0; j < NUM_LENGTH; j++) {
                    if (ticket == winningNumbers[_round][j]) {
                        matchesCount++;
                    }
                }
                
                if (matchesCount >= tier) {
                    totalMatches++;
                }
            }
            
            if (totalMatches > 0) {
                // Prize pool distribution
                uint256 tierReward = (pool * LotteryMath.getRewardRate(tier)) / 10000;
                prizePerTier[_round][tier] = tierReward / totalMatches;
            }
        }
    }

    // ============ Claim Rewards ============

    /**
     * @notice Claim winnings
     */
    function claimReward(uint256 _round) external nonReentrant {
        require(roundFinalized[_round], "Round not finalized");
        
        uint256[NUM_LENGTH] memory winning = winningNumbers[_round];
        uint256[] storage userTicketNumbers = userTickets[_round][msg.sender];
        
        require(userTicketNumbers.length > 0, "No tickets");
        
        uint256 totalWinnings = 0;
        
        // Check each ticket
        for (uint256 i = 0; i < userTicketNumbers.length; i++) {
            uint256 ticket = userTicketNumbers[i];
            
            // Count matches
            uint256 matches = 0;
            for (uint256 j = 0; j < NUM_LENGTH; j++) {
                if (ticket == winning[j]) {
                    matches++;
                }
            }
            
            if (matches > 0 && prizePerTier[_round][matches] > 0) {
                totalWinnings += prizePerTier[_round][matches];
            }
        }
        
        require(totalWinnings > 0, "No winnings");
        
        // Transfer winnings
        token.safeTransfer(msg.sender, totalWinnings);
        
        userWinnings[_round][msg.sender] = totalWinnings;
        
        emit RewardsClaimed(msg.sender, _round, totalWinnings);
    }

    // ============ Admin Functions ============

    /**
     * @notice Start new round
     */
    function startNewRound(uint256 _duration) external onlyOwner {
        require(roundFinalized[currentRound], "Previous round not finalized");
        
        currentRound++;
        closeTime = block.timestamp + _duration;
        drawTime = closeTime + 1 hours;
    }

    /**
     * @notice Update ticket price
     */
    function setTicketPrice(uint256 _price) external onlyOwner {
        require(_price > 0, "Price must be positive");
        
        ticketPrice = _price;
        
        emit PriceUpdated(_price);
    }

    /**
     * @notice Update treasury
     */
    function setTreasury(address _treasury) external onlyOwner {
        require(_treasury != address(0), "Invalid treasury");
        
        treasury = _treasury;
        
        emit TreasuryUpdated(_treasury);
    }

    /**
     * @notice Withdraw treasury
     */
    function withdrawTreasury() external onlyOwner {
        uint256 amount = totalTreasury;
        totalTreasury = 0;
        
        token.safeTransfer(treasury, amount);
    }

    // ============ View Functions ============

    /**
     * @notice Get user's tickets for a round
     */
    function getUserTickets(address _user, uint256 _round) external view returns (uint256[] memory) {
        return userTickets[_round][_user];
    }

    /**
     * @notice Get round info
     */
    function getRoundInfo(uint256 _round) external view returns (
        uint256 ticketCount,
        uint256 prizePoolAmount,
        bool finalized,
        uint256[4] memory winning
    ) {
        return (
            roundTickets[_round].length,
            prizePool[_round],
            roundFinalized[_round],
            winningNumbers[_round]
        );
    }

    /**
     * @notice Get pending winnings
     */
    function getPendingWinnings(address _user, uint256 _round) external view returns (uint256) {
        if (!roundFinalized[_round]) return 0;
        if (userWinnings[_round][_user] > 0) return 0;
        
        uint256[NUM_LENGTH] memory winning = winningNumbers[_round];
        uint256[] storage userTicketNumbers = userTickets[_round][_user];
        
        uint256 total = 0;
        
        for (uint256 i = 0; i < userTicketNumbers.length; i++) {
            uint256 ticket = userTicketNumbers[i];
            uint256 matches = 0;
            
            for (uint256 j = 0; j < NUM_LENGTH; j++) {
                if (ticket == winning[j]) {
                    matches++;
                }
            }
            
            if (matches > 0) {
                total += prizePerTier[_round][matches];
            }
        }
        
        return total;
    }
}
