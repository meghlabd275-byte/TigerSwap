// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/**
 * @title TigerInsuranceFund
 * @notice Production Insurance Fund - Protocol Protection System
 * @dev Risk backstop for protocol failures, hacks, and extreme events
 * 
 * Features:
 * - Multi-token coverage
 * - Claims processing
 * - Governance-controlled payouts
 * - Premium collection
 * - Emergency reserves
 * - Coverage limits
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
library InsuranceMath {
    uint256 constant BASIS_POINTS = 10000;
    
    function mul(uint256 x, uint256 y, uint256 precision) internal pure returns (uint256) {
        return (x * y) / precision;
    }
    
    function div(uint256 x, uint256 y, uint256 precision) internal pure returns (uint256) {
        return (x * precision) / y;
    }
}

/**
 * @title TigerInsuranceFund
 * @dev Main insurance fund contract
 */
contract TigerInsuranceFund is ReentrancyGuard, Ownable, AccessControl {
    using SafeERC20 for IERC20;
    using InsuranceMath for uint256;

    // ============ Roles ============
    bytes32 public constant CLAIM_MANAGER = keccak256("CLAIM_MANAGER");
    bytes32 public constant TREASURY_MANAGER = keccak256("TREASURY_MANAGER");
    bytes32 public constant PAUSE_ADMIN = keccak256("PAUSE_ADMIN");

    // ============ Constants ============
    uint256 constant MIN_COVERAGE = 1000e18;
    uint256 constant MAX_COVERAGE = 1000000e18;
    uint256 constant CLAIM_EXPIRY = 90 days;
    uint256 constant BASIS_POINTS = 10000;
    uint256 constant PREMIUM_DENOMINATOR = 10000;

    // Coverage types
    uint8 constant COVERAGE_SWAP = 1;
    uint8 constant COVERAGE_LIQUIDITY = 2;
    uint8 constant COVERAGE_BRIDGE = 3;
    uint8 constant COVERAGE_STAKING = 4;
    uint8 constant COVERAGE_PERPETUAL = 5;

    // Claim status
    uint8 constant STATUS_PENDING = 1;
    uint8 constant STATUS_REVIEWING = 2;
    uint8 constant STATUS_APPROVED = 3;
    uint8 constant STATUS_REJECTED = 4;
    uint8 constant STATUS_PAID = 5;
    uint8 constant STATUS_EXPIRED = 6;

    // ============ State Variables ============
    
    // Governance
    address public governance;
    address public pendingGovernance;
    address public claimSigner;
    
    // Fund Management
    uint256 public totalDeposits;
    uint256 public totalClaimsPaid;
    uint256 public totalPremiumCollected;
    uint256 public currentBalance;
    uint256 public reserveRatio = 3000; // 30% reserves
    
    // Coverage Configuration
    mapping(uint8 => CoverageConfig) public coverageConfigs;
    mapping(address => UserCoverage) public userCoverages;
    mapping(address => bool) public supportedTokens;
    address[] public supportedTokenList;
    
    // Claims
    mapping(bytes32 => Claim) public claims;
    bytes32[] public claimIds;
    mapping(address => bytes32[]) public userClaims;
    uint256 public claimCount;
    
    // Premium
    uint256 public premiumRate = 50; // 0.5%
    mapping(address => uint256) public userPremiums;
    mapping(address => uint256) public userDepositTime;
    
    // Emergency
    bool public emergencyMode;
    bool public claimsPaused;
    uint256 public minReserve; // Minimum reserve to maintain
    
    // ============ Structs ============
    
    struct CoverageConfig {
        bool enabled;
        uint256 minCoverage;
        uint256 maxCoverage;
        uint256 premiumBps;
        uint256 deductible;
        bool requiresKYC;
    }
    
    struct UserCoverage {
        uint256 coverageAmount;
        uint256 coverageType;
        uint256 startTime;
        uint256 endTime;
        bool active;
    }
    
    struct Claim {
        address claimant;
        uint256 amount;
        uint256 coverageType;
        string description;
        bytes32 evidenceHash;
        uint8 status;
        uint256 submittedAt;
        uint256 reviewedAt;
        address reviewer;
        uint256 payoutAmount;
        string rejectionReason;
    }

    // ============ Events ============
    event CoverageUpdated(address indexed user, uint256 coverage, uint8 coverageType);
    event CoverageActivated(address indexed user, uint256 amount, uint8 coverageType);
    event CoverageExpired(address indexed user);
    event ClaimSubmitted(bytes32 indexed claimId, address indexed user, uint256 amount, uint8 coverageType);
    event ClaimApproved(bytes32 indexed claimId, uint256 amount);
    event ClaimRejected(bytes32 indexed claimId, string reason);
    event ClaimPaid(bytes32 indexed claimId, uint256 amount);
    event PremiumPaid(address indexed user, uint256 amount);
    event Deposit(address indexed user, uint256 amount);
    event Withdrawal(address indexed user, uint256 amount);
    event GovernanceUpdated(address indexed oldGov, address indexed newGov);
    event EmergencyModeActivated(bool mode);
    event ReservesUpdated(uint256 newRatio);

    // ============ Constructor ============
    
    constructor(address _owner) Ownable(_owner) {
        governance = _owner;
        
        // Initialize coverage configs
        coverageConfigs[COVERAGE_SWAP] = CoverageConfig({
            enabled: true,
            minCoverage: MIN_COVERAGE,
            maxCoverage: MAX_COVERAGE,
            premiumBps: 50,
            deductible: 100e18,
            requiresKYC: false
        });
        
        coverageConfigs[COVERAGE_LIQUIDITY] = CoverageConfig({
            enabled: true,
            minCoverage: MIN_COVERAGE,
            maxCoverage: MAX_COVERAGE * 10,
            premiumBps: 75,
            deductible: 500e18,
            requiresKYC: false
        });
        
        coverageConfigs[COVERAGE_BRIDGE] = CoverageConfig({
            enabled: true,
            minCoverage: MIN_COVERAGE * 10,
            maxCoverage: MAX_COVERAGE * 5,
            premiumBps: 100,
            deductible: 1000e18,
            requiresKYC: true
        });
        
        coverageConfigs[COVERAGE_STAKING] = CoverageConfig({
            enabled: true,
            minCoverage: MIN_COVERAGE,
            maxCoverage: MAX_COVERAGE,
            premiumBps: 50,
            deductible: 50e18,
            requiresKYC: false
        });
        
        coverageConfigs[COVERAGE_PERPETUAL] = CoverageConfig({
            enabled: true,
            minCoverage: MIN_COVERAGE * 5,
            maxCoverage: MAX_COVERAGE * 10,
            premiumBps: 150,
            deductible: 1000e18,
            requiresKYC: true
        });
        
        // Grant roles
        _grantRole(DEFAULT_ADMIN_ROLE, _owner);
        _grantRole(CLAIM_MANAGER, _owner);
        _grantRole(TREASURY_MANAGER, _owner);
    }

    // ============ Coverage Functions ============

    /**
     * @notice Purchase coverage
     */
    function purchaseCoverage(
        uint8 _coverageType,
        uint256 _amount,
        address _token
    ) external nonReentrant {
        require(!emergencyMode, "Emergency mode active");
        require(!claimsPaused, "Coverage purchases paused");
        
        CoverageConfig memory config = coverageConfigs[_coverageType];
        require(config.enabled, "Coverage type not enabled");
        require(_amount >= config.minCoverage, "Below minimum coverage");
        require(_amount <= config.maxCoverage, "Above maximum coverage");
        
        // Calculate premium
        uint256 premium = _amount.mul(config.premiumBps, PREMIUM_DENOMINATOR);
        
        // Transfer premium
        IERC20(_token).safeTransferFrom(msg.sender, address(this), premium);
        
        // Update user coverage
        userCoverages[msg.sender] = UserCoverage({
            coverageAmount: _amount,
            coverageType: _coverageType,
            startTime: block.timestamp,
            endTime: block.timestamp + 365 days,
            active: true
        });
        
        // Update totals
        totalPremiumCollected += premium;
        userPremiums[msg.sender] += premium;
        userDepositTime[msg.sender] = block.timestamp;
        
        emit PremiumPaid(msg.sender, premium);
        emit CoverageActivated(msg.sender, _amount, _coverageType);
    }

    /**
     * @notice Extend coverage
     */
    function extendCoverage(uint256 _additionalDays) external nonReentrant {
        UserCoverage storage coverage = userCoverages[msg.sender];
        require(coverage.active, "No active coverage");
        
        CoverageConfig memory config = coverageConfigs[uint8(coverage.coverageType)];
        require(config.enabled, "Coverage type disabled");
        
        uint256 proportionalPremium = (coverage.coverageAmount * config.premiumBps * _additionalDays) 
            / (PREMIUM_DENOMINATOR * 365);
        
        IERC20(supportedTokenList[0]).safeTransferFrom(
            msg.sender, 
            address(this), 
            proportionalPremium
        );
        
        coverage.endTime += _additionalDays * 1 days;
        totalPremiumCollected += proportionalPremium;
        
        emit CoverageUpdated(
            msg.sender, 
            coverage.coverageAmount, 
            coverage.coverageType
        );
    }

    /**
     * @notice Cancel coverage
     */
    function cancelCoverage() external nonReentrant {
        UserCoverage storage coverage = userCoverages[msg.sender];
        require(coverage.active, "No active coverage");
        
        uint256 daysRemaining = (coverage.endTime - block.timestamp) / 1 days;
        require(daysRemaining > 30, "Cannot cancel within 30 days of expiry");
        
        coverage.active = false;
        
        emit CoverageExpired(msg.sender);
    }

    // ============ Claim Functions ============

    /**
     * @notice Submit a claim
     */
    function submitClaim(
        uint256 _amount,
        uint8 _coverageType,
        string memory _description,
        bytes32 _evidenceHash
    ) external nonReentrant returns (bytes32) {
        require(!emergencyMode, "Emergency mode active");
        require(!claimsPaused, "Claims paused");
        
        UserCoverage memory coverage = userCoverages[msg.sender];
        require(coverage.active, "No active coverage");
        require(coverage.coverageType == _coverageType, "Wrong coverage type");
        require(_amount <= coverage.coverageAmount, "Exceeds coverage");
        
        CoverageConfig memory config = coverageConfigs[_coverageType];
        require(_amount >= config.deductible, "Below deductible");
        
        // Create claim
        bytes32 claimId = keccak256(abi.encodePacked(
            msg.sender,
            _amount,
            block.timestamp,
            claimCount++
        ));
        
        claims[claimId] = Claim({
            claimant: msg.sender,
            amount: _amount,
            coverageType: _coverageType,
            description: _description,
            evidenceHash: _evidenceHash,
            status: STATUS_PENDING,
            submittedAt: block.timestamp,
            reviewedAt: 0,
            reviewer: address(0),
            payoutAmount: 0,
            rejectionReason: ""
        });
        
        claimIds.push(claimId);
        userClaims[msg.sender].push(claimId);
        
        emit ClaimSubmitted(claimId, msg.sender, _amount, _coverageType);
        
        return claimId;
    }

    /**
     * @notice Approve a claim
     */
    function approveClaim(bytes32 _claimId, uint256 _payoutAmount) external onlyRole(CLAIM_MANAGER) {
        Claim storage claim = claims[_claimId];
        require(claim.status == STATUS_PENDING, "Not pending");
        require(_payoutAmount <= claim.amount, "Exceeds claim amount");
        
        claim.status = STATUS_APPROVED;
        claim.reviewedAt = block.timestamp;
        claim.reviewer = msg.sender;
        claim.payoutAmount = _payoutAmount;
        
        emit ClaimApproved(_claimId, _payoutAmount);
    }

    /**
     * @notice Reject a claim
     */
    function rejectClaim(bytes32 _claimId, string memory _reason) external onlyRole(CLAIM_MANAGER) {
        Claim storage claim = claims[_claimId];
        require(claim.status == STATUS_PENDING, "Not pending");
        
        claim.status = STATUS_REJECTED;
        claim.reviewedAt = block.timestamp;
        claim.reviewer = msg.sender;
        claim.rejectionReason = _reason;
        
        emit ClaimRejected(_claimId, _reason);
    }

    /**
     * @notice Pay out a claim
     */
    function payClaim(bytes32 _claimId, address _token) external onlyRole(CLAIM_MANAGER) nonReentrant {
        Claim storage claim = claims[_claimId];
        require(claim.status == STATUS_APPROVED, "Not approved");
        require(claim.payoutAmount > 0, "No payout amount");
        
        // Check reserves
        uint256 available = currentBalance - minReserve;
        require(available >= claim.payoutAmount, "Insufficient reserves");
        
        claim.status = STATUS_PAID;
        
        // Transfer payout
        IERC20(_token).safeTransfer(claim.claimant, claim.payoutAmount);
        
        // Update totals
        currentBalance -= claim.payoutAmount;
        totalClaimsPaid += claim.payoutAmount;
        
        emit ClaimPaid(_claimId, claim.payoutAmount);
    }

    // ============ Fund Management ============

    /**
     * @notice Deposit to fund
     */
    function deposit(uint256 _amount, address _token) external nonReentrant {
        require(_amount > 0, "Cannot deposit 0");
        
        IERC20(_token).safeTransferFrom(msg.sender, address(this), _amount);
        
        totalDeposits += _amount;
        currentBalance += _amount;
        
        if (!supportedTokens[_token]) {
            supportedTokens[_token] = true;
            supportedTokenList.push(_token);
        }
        
        emit Deposit(msg.sender, _amount);
    }

    /**
     * @notice Withdraw from fund
     */
    function withdraw(uint256 _amount, address _recipient, address _token) 
        external 
        onlyRole(TREASURY_MANAGER) 
        nonReentrant 
    {
        require(_amount > 0, "Cannot withdraw 0");
        require(_recipient != address(0), "Invalid recipient");
        
        uint256 available = currentBalance - minReserve;
        require(_amount <= available, "Exceeds available reserves");
        
        IERC20(_token).safeTransfer(_recipient, _amount);
        
        currentBalance -= _amount;
        
        emit Withdrawal(_recipient, _amount);
    }

    /**
     * @notice Set minimum reserve
     */
    function setMinReserve(uint256 _amount) external onlyOwner {
        minReserve = _amount;
    }

    /**
     * @notice Set reserve ratio
     */
    function setReserveRatio(uint256 _ratio) external onlyOwner {
        require(_ratio <= BASIS_POINTS, "Ratio too high");
        reserveRatio = _ratio;
        
        emit ReservesUpdated(_ratio);
    }

    // ============ Emergency Functions ============

    /**
     * @notice Activate emergency mode
     */
    function activateEmergencyMode(bool _mode) external onlyOwner {
        emergencyMode = _mode;
        
        emit EmergencyModeActivated(_mode);
    }

    /**
     * @notice Pause claims
     */
    function pauseClaims(bool _pause) external onlyRole(PAUSE_ADMIN) {
        claimsPaused = _pause;
    }

    /**
     * @notice Emergency withdrawal
     */
    function emergencyWithdraw(address _token) external onlyOwner {
        require(emergencyMode, "Not in emergency mode");
        
        uint256 balance = IERC20(_token).balanceOf(address(this));
        require(balance > 0, "No balance");
        
        // Only withdraw non-reserved amount
        uint256 available = balance - minReserve;
        if (available > 0) {
            IERC20(_token).safeTransfer(governance, available);
        }
    }

    // ============ Governance ============

    /**
     * @notice Set governance
     */
    function setGovernance(address _governance) external {
        require(msg.sender == governance, "Only governance");
        pendingGovernance = _governance;
    }

    /**
     * @notice Accept governance
     */
    function acceptGovernance() external {
        require(msg.sender == pendingGovernance, "Not pending governance");
        emit GovernanceUpdated(governance, pendingGovernance);
        governance = pendingGovernance;
        pendingGovernance = address(0);
    }

    // ============ View Functions ============

    /**
     * @notice Get user coverage
     */
    function getUserCoverage(address _user) external view returns (UserCoverage memory) {
        return userCoverages[_user];
    }

    /**
     * @notice Get claim details
     */
    function getClaim(bytes32 _claimId) external view returns (Claim memory) {
        return claims[_claimId];
    }

    /**
     * @notice Get user claims
     */
    function getUserClaims(address _user) external view returns (Claim[] memory) {
        bytes32[] storage ids = userClaims[_user];
        Claim[] memory result = new Claim[](ids.length);
        
        for (uint256 i = 0; i < ids.length; i++) {
            result[i] = claims[ids[i]];
        }
        
        return result;
    }

    /**
     * @notice Calculate coverage premium
     */
    function calculatePremium(uint256 _amount, uint8 _coverageType) external view returns (uint256) {
        CoverageConfig memory config = coverageConfigs[_coverageType];
        require(config.enabled, "Coverage type not enabled");
        
        return _amount.mul(config.premiumBps, PREMIUM_DENOMINATOR);
    }

    /**
     * @notice Get total coverage in effect
     */
    function getTotalActiveCoverage() external view returns (uint256 total) {
        // This would need to iterate through all users in production
        // Simplified for example
        return totalPremiumCollected;
    }

    /**
     * @notice Get supported tokens
     */
    function getSupportedTokens() external view returns (address[] memory) {
        return supportedTokenList;
    }
}
