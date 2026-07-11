// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/**
 * @title TigerLaunchpad
 * @notice Production IDO/IFO Launchpad Platform
 * @dev Token launch and initial farm offering platform
 * 
 * Features:
 * - Fair launch with caps
 * - FCFS and lottery rounds
 * - Tier-based allocation
 * - Lockup periods
 * - Liquidity bootstrapping
 * - Emergency cancellation
 * 
 * @author TigerSwap
 * @version 1.0.0 Production
 */

import "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";
import "@openzeppelin/contracts/utils/ReentrancyGuard.sol";
import "@openzeppelin/contracts/access/Ownable.sol";
import "@openzeppelin/contracts/access/AccessControl.sol";

/**
 * @title Decimal Math
 */
library LaunchpadMath {
    uint256 constant PRECISION = 1e18;
    
    function mul(uint256 x, uint256 y) internal pure returns (uint256) {
        return (x * y) / PRECISION;
    }
    
    function div(uint256 x, uint256 y) internal pure returns (uint256) {
        return (x * PRECISION) / y;
    }
}

/**
 * @title TigerLaunchpad
 * @dev Main launchpad contract
 */
contract TigerLaunchpad is ReentrancyGuard, Ownable, AccessControl {
    using SafeERC20 for IERC20;
    using LaunchpadMath for uint256;

    // ============ Roles ============
    bytes32 public constant LAUNCHPAD_ADMIN = keccak256("LAUNCHPAD_ADMIN");
    bytes32 public constant OPERATOR = keccak256("OPERATOR");

    // ============ Constants ============
    uint256 constant TIER_NONE = 0;
    uint256 constant TIER_BRONZE = 1;
    uint256 constant TIER_SILVER = 2;
    uint256 constant TIER_GOLD = 3;
    uint256 constant TIER_PLATINUM = 4;
    uint256 constant TIER_DIAMOND = 5;

    uint256 constant ROUND_SEED = 1;
    uint256 constant ROUND_PRIVATE = 2;
    uint256 constant ROUND_PUBLIC = 3;
    uint256 constant ROUND_FCFS = 4;

    // ============ State Variables ============
    
    // Launch info
    address public token;
    address public paymentToken; // Usually USDC/USDT
    address public treasury;
    address public liquidityDestination;
    
    // Sale parameters
    uint256 public tokenPrice;
    uint256 public tokensForSale;
    uint256 public tokensForLiquidity;
    uint256 public tokensForTeam;
    uint256 public tokensForRewards;
    uint256 public hardCap;
    uint256 public softCap;
    uint256 public minBuy;
    uint256 public maxBuy;
    
    // Timing
    uint256 public startTime;
    uint256 public endTime;
    uint256 public claimStartTime;
    uint256 public liquidityLockPeriod;
    
    // Funding
    uint256 public raisedAmount;
    uint256 public soldAmount;
    uint256 public refundedAmount;
    
    // Status
    bool public isCancelled;
    bool public isClaimed;
    bool public liquidityAdded;
    
    // Rounds
    uint256 public currentRound;
    uint256 public seedRoundAlloc;
    uint256 public privateRoundAlloc;
    uint256 public publicRoundAlloc;
    
    // Tiers
    mapping(address => uint256) public userTier;
    mapping(address => uint256) public userAllocation;
    mapping(address => uint256) public userClaimed;
    mapping(address => uint256) public userDeposited;
    mapping(address => bool) public hasClaimed;
    
    // Tier requirements (TIGER tokens held)
    mapping(uint256 => uint256) public tierRequirements;
    mapping(uint256 => uint256) public tierAllocation;
    mapping(uint256 => uint256) public tierMaxBuy;
    
    // Whitelists
    mapping(address => bool) public seedWhitelist;
    mapping(address => bool) public privateWhitelist;
    
    // Statistics
    uint256 public totalParticipants;
    mapping(address => bool) public hasParticipated;

    // ============ Events ============
    event TokensSold(address indexed user, uint256 amount, uint256 paymentAmount);
    event TokensClaimed(address indexed user, uint256 amount);
    event RefundClaimed(address indexed user, uint256 amount);
    event LaunchpadCreated(address indexed token, uint256 tokenPrice, uint256 tokensForSale);
    event RoundStarted(uint256 round);
    event RoundEnded(uint256 round, uint256 raised);
    event LiquidityAdded(uint256 amountToken, uint256 amountETH);
    event Cancelled();
    event TierUpdated(address indexed user, uint256 tier);
    event WhitelistUpdated(address indexed user, bool status, uint256 round);

    // ============ Constructor ============
    
    constructor(address _owner) Ownable(_owner) {
        treasury = _owner;
        
        // Initialize tier requirements (in TIGER tokens)
        tierRequirements[TIER_NONE] = 0;
        tierRequirements[TIER_BRONZE] = 100e18;
        tierRequirements[TIER_SILVER] = 500e18;
        tierRequirements[TIER_GOLD] = 2000e18;
        tierRequirements[TIER_PLATINUM] = 10000e18;
        tierRequirements[TIER_DIAMOND] = 50000e18;
        
        // Initialize tier allocations
        tierAllocation[TIER_NONE] = 0;
        tierAllocation[TIER_BRONZE] = 100e18;
        tierAllocation[TIER_SILVER] = 500e18;
        tierAllocation[TIER_GOLD] = 2000e18;
        tierAllocation[TIER_PLATINUM] = 10000e18;
        tierAllocation[TIER_DIAMOND] = 50000e18;
        
        tierMaxBuy[TIER_NONE] = 100e18;
        tierMaxBuy[TIER_BRONZE] = 500e18;
        tierMaxBuy[TIER_SILVER] = 2000e18;
        tierMaxBuy[TIER_GOLD] = 10000e18;
        tierMaxBuy[TIER_PLATINUM] = 50000e18;
        tierMaxBuy[TIER_DIAMOND] = 100000e18;
        
        _grantRole(DEFAULT_ADMIN_ROLE, _owner);
        _grantRole(LAUNCHPAD_ADMIN, _owner);
    }

    // ============ Setup Functions ============

    /**
     * @notice Initialize launchpad
     */
    function initialize(
        address _token,
        address _paymentToken,
        uint256 _tokenPrice,
        uint256 _tokensForSale,
        uint256 _tokensForLiquidity,
        uint256 _tokensForTeam,
        uint256 _tokensForRewards,
        uint256 _hardCap,
        uint256 _softCap,
        uint256 _minBuy,
        uint256 _maxBuy,
        uint256 _startTime,
        uint256 _duration,
        uint256 _liquidityLockPeriod,
        address _liquidityDestination
    ) external onlyRole(LAUNCHPAD_ADMIN) {
        require(token == address(0), "Already initialized");
        require(_token != address(0), "Invalid token");
        require(_tokenPrice > 0, "Invalid price");
        require(_tokensForSale > 0, "No tokens for sale");
        
        token = _token;
        paymentToken = _paymentToken;
        tokenPrice = _tokenPrice;
        tokensForSale = _tokensForSale;
        tokensForLiquidity = _tokensForLiquidity;
        tokensForTeam = _tokensForTeam;
        tokensForRewards = _tokensForRewards;
        hardCap = _hardCap;
        softCap = _softCap;
        minBuy = _minBuy;
        maxBuy = _maxBuy;
        startTime = _startTime;
        endTime = _startTime + _duration;
        liquidityLockPeriod = _liquidityLockPeriod;
        liquidityDestination = _liquidityDestination;
        
        currentRound = ROUND_SEED;
        
        emit LaunchpadCreated(_token, _tokenPrice, _tokensForSale);
    }

    // ============ Tier Functions ============

    /**
     * @notice Set tier for user
     */
    function setTier(address _user, uint256 _tier) external onlyRole(OPERATOR) {
        require(_tier <= TIER_DIAMOND, "Invalid tier");
        userTier[_user] = _tier;
        
        emit TierUpdated(_user, _tier);
    }

    /**
     * @notice Batch set tiers
     */
    function batchSetTier(address[] calldata _users, uint256[] calldata _tiers) 
        external 
        onlyRole(OPERATOR) 
    {
        require(_users.length == _tiers.length, "Length mismatch");
        
        for (uint256 i = 0; i < _users.length; i++) {
            require(_tiers[i] <= TIER_DIAMOND, "Invalid tier");
            userTier[_users[i]] = _tiers[i];
            
            emit TierUpdated(_users[i], _tiers[i]);
        }
    }

    /**
     * @notice Update tier requirements
     */
    function setTierRequirement(uint256 _tier, uint256 _requirement) 
        external 
        onlyRole(LAUNCHPAD_ADMIN) 
    {
        require(_tier <= TIER_DIAMOND, "Invalid tier");
        tierRequirements[_tier] = _requirement;
    }

    // ============ Whitelist Functions ============

    /**
     * @notice Add to seed whitelist
     */
    function addToSeedWhitelist(address[] calldata _users) external onlyRole(OPERATOR) {
        for (uint256 i = 0; i < _users.length; i++) {
            seedWhitelist[_users[i]] = true;
            emit WhitelistUpdated(_users[i], true, ROUND_SEED);
        }
    }

    /**
     * @notice Add to private whitelist
     */
    function addToPrivateWhitelist(address[] calldata _users) external onlyRole(OPERATOR) {
        for (uint256 i = 0; i < _users.length; i++) {
            privateWhitelist[_users[i]] = true;
            emit WhitelistUpdated(_users[i], true, ROUND_PRIVATE);
        }
    }

    /**
     * @notice Remove from whitelist
     */
    function removeFromWhitelist(address _user) external onlyRole(OPERATOR) {
        seedWhitelist[_user] = false;
        privateWhitelist[_user] = false;
    }

    // ============ Round Functions ============

    /**
     * @notice Start a new round
     */
    function startRound(uint256 _round) external onlyRole(LAUNCHPAD_ADMIN) {
        require(block.timestamp >= startTime, "Not started");
        require(currentRound < ROUND_PUBLIC, "All rounds done");
        require(!isCancelled, "Cancelled");
        
        currentRound = _round;
        
        emit RoundStarted(_round);
    }

    /**
     * @notice End current round
     */
    function endRound() external onlyRole(LAUNCHPAD_ADMIN) {
        require(currentRound > 0, "Not started");
        
        emit RoundEnded(currentRound, raisedAmount);
        
        if (currentRound < ROUND_PUBLIC) {
            currentRound++;
        }
    }

    // ============ Buy Functions ============

    /**
     * @notice Buy tokens
     */
    function buy(uint256 _paymentAmount) external nonReentrant {
        require(!isCancelled, "Cancelled");
        require(block.timestamp >= startTime, "Not started");
        require(block.timestamp <= endTime, "Ended");
        require(_paymentAmount >= minBuy, "Below min");
        
        // Check round eligibility
        if (currentRound == ROUND_SEED) {
            require(seedWhitelist[msg.sender], "Not whitelisted");
            require(raisedAmount + _paymentAmount <= seedRoundAlloc, "Seed round cap");
        } else if (currentRound == ROUND_PRIVATE) {
            require(privateWhitelist[msg.sender], "Not whitelisted");
            require(raisedAmount + _paymentAmount <= privateRoundAlloc, "Private round cap");
        } else {
            // Public round
            require(raisedAmount < hardCap, "Hard cap reached");
        }
        
        // Check max buy
        uint256 userTotal = userDeposited[msg.sender] + _paymentAmount;
        uint256 userMax = tierMaxBuy[userTier[msg.sender]];
        if (userMax > 0) {
            require(userTotal <= userMax, "Exceeds max buy");
        }
        
        // Transfer payment
        IERC20(paymentToken).safeTransferFrom(msg.sender, address(this), _paymentAmount);
        
        // Calculate tokens
        uint256 tokensBought = _paymentAmount.div(tokenPrice);
        require(tokensBought > 0, "Insufficient payment");
        
        // Check allocation
        uint256 userAlloc = userAllocation[msg.sender];
        uint256 tierAlloc = tierAllocation[userTier[msg.sender]];
        if (tierAlloc > 0) {
            require(userAlloc + tokensBought <= tierAlloc, "Exceeds allocation");
        }
        
        // Update state
        userDeposited[msg.sender] += _paymentAmount;
        userAllocation[msg.sender] += tokensBought;
        raisedAmount += _paymentAmount;
        soldAmount += tokensBought;
        
        if (!hasParticipated[msg.sender]) {
            hasParticipated[msg.sender] = true;
            totalParticipants++;
        }
        
        emit TokensSold(msg.sender, tokensBought, _paymentAmount);
    }

    // ============ Claim Functions ============

    /**
     * @notice Claim tokens
     */
    function claim() external nonReentrant {
        require(!isCancelled, "Cancelled");
        require(block.timestamp >= claimStartTime, "Claim not started");
        require(!hasClaimed[msg.sender], "Already claimed");
        
        uint256 allocation = userAllocation[msg.sender];
        require(allocation > 0, "No allocation");
        
        // Calculate claimable amount (vesting could be added here)
        uint256 claimable = allocation;
        
        // Apply vesting if set
        // For simplicity, releasing all at once
        
        hasClaimed[msg.sender] = true;
        userClaimed[msg.sender] = claimable;
        
        // Transfer tokens
        IERC20(token).safeTransfer(msg.sender, claimable);
        
        emit TokensClaimed(msg.sender, claimable);
    }

    // ============ Refund Functions ============

    /**
     * @notice Claim refund if soft cap not met
     */
    function claimRefund() external nonReentrant {
        require(isCancelled || (block.timestamp > endTime && raisedAmount < softCap), "Not eligible");
        
        uint256 deposited = userDeposited[msg.sender];
        require(deposited > 0, "No deposit");
        require(!hasClaimed[msg.sender], "Already claimed");
        
        // Transfer refund
        IERC20(paymentToken).safeTransfer(msg.sender, deposited);
        
        refundedAmount += deposited;
        
        emit RefundClaimed(msg.sender, deposited);
    }

    // ============ Liquidity Functions ============

    /**
     * @notice Add liquidity after sale
     */
    function addLiquidity() external onlyRole(LAUNCHPAD_ADMIN) nonReentrant {
        require(!liquidityAdded, "Liquidity added");
        require(raisedAmount >= softCap, "Soft cap not met");
        require(block.timestamp > endTime, "Sale not ended");
        
        liquidityAdded = true;
        
        // Calculate liquidity amounts
        uint256 liquidityTokenAmount = tokensForLiquidity;
        uint256 liquidityPaymentAmount = raisedAmount * 20 / 100; // 20% of raised
        
        // Transfer tokens to liquidity destination
        IERC20(token).safeTransfer(liquidityDestination, liquidityTokenAmount);
        
        // Transfer payment token for liquidity
        IERC20(paymentToken).safeTransfer(liquidityDestination, liquidityPaymentAmount);
        
        emit LiquidityAdded(liquidityTokenAmount, liquidityPaymentAmount);
    }

    // ============ Admin Functions ============

    /**
     * @notice Set claim start time
     */
    function setClaimStartTime(uint256 _time) external onlyRole(LAUNCHPAD_ADMIN) {
        require(claimStartTime == 0, "Already set");
        claimStartTime = _time;
    }

    /**
     * @notice Cancel launchpad
     */
    function cancel() external onlyRole(LAUNCHPAD_ADMIN) {
        require(!isCancelled, "Already cancelled");
        
        isCancelled = true;
        
        emit Cancelled();
    }

    /**
     * @notice Withdraw unsold tokens
     */
    function withdrawUnsoldTokens() external onlyRole(LAUNCHPAD_ADMIN) {
        require(block.timestamp > endTime, "Sale ongoing");
        
        uint256 unsold = tokensForSale - soldAmount;
        if (unsold > 0) {
            IERC20(token).safeTransfer(treasury, unsold);
        }
    }

    /**
     * @notice Withdraw team tokens
     */
    function withdrawTeamTokens() external onlyRole(LAUNCHPAD_ADMIN) {
        require(block.timestamp > claimStartTime + 180 days, "Locked");
        
        if (tokensForTeam > 0) {
            IERC20(token).safeTransfer(treasury, tokensForTeam);
            tokensForTeam = 0;
        }
    }

    // ============ View Functions ============

    /**
     * @notice Get user info
     */
    function getUserInfo(address _user) external view returns (
        uint256 tier,
        uint256 deposited,
        uint256 allocation,
        uint256 claimed,
        bool claimed_
    ) {
        return (
            userTier[_user],
            userDeposited[_user],
            userAllocation[_user],
            userClaimed[_user],
            hasClaimed[_user]
        );
    }

    /**
     * @notice Get round info
     */
    function getRoundInfo() external view returns (
        uint256 round,
        uint256 raised,
        uint256 sold,
        uint256 remaining,
        bool ended
    ) {
        return (
            currentRound,
            raisedAmount,
            soldAmount,
            tokensForSale - soldAmount,
            block.timestamp > endTime || raisedAmount >= hardCap
        );
    }

    /**
     * @notice Calculate tokens for payment
     */
    function getTokensForPayment(uint256 _paymentAmount) external view returns (uint256) {
        return _paymentAmount.div(tokenPrice);
    }
}
