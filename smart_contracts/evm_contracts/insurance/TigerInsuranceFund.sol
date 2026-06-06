// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "../libraries/SafeMath.sol";

/**
 * @title TigerInsuranceFund
 * @notice Protocol Insurance Fund
 * @dev Protects users from smart contract failures and hacks
 * 
 * Features:
 * - Insurance coverage for users
 * - Coverage claims processing
 * - Premium collection
 * - Fund management
 * - Coverage limits
 * - Emergency withdrawal
 */
contract TigerInsuranceFund {
    using SafeMath for uint256;

    // ============================================================================
    // Constants
    // ============================================================================
    
    uint256 constant COVERAGE_DENOMINATOR = 10000;
    uint256 constant MIN_COVERAGE = 1000e18;
    uint256 constant MAX_COVERAGE = 1000000e18;
    uint256 constant CLAIM_EXPIRY = 90 days;
    uint256 constant PREMIUM_DENOMINATOR = 10000;
    
    // Coverage Types
    uint8 constant COVERAGE_SWAP = 1;
    uint8 constant COVERAGE_LIQUIDITY = 2;
    uint8 constant COVERAGE_BRIDGE = 3;
    uint8 constant COVERAGE_STAKING = 4;
    
    // Claim Status
    uint8 constant STATUS_PENDING = 1;
    uint8 constant STATUS_APPROVED = 2;
    uint8 constant STATUS_REJECTED = 3;
    uint8 constant STATUS_PAID = 4;
    
    // ============================================================================
    // State Variables
    // ============================================================================
    
    // Governance
    address public governance;
    address public pendingGovernance;
    address public claimSigner;
    
    // Fund Management
    uint256 public totalDeposits;
    uint256 public totalClaimsPaid;
    uint256 public totalPremiumCollected;
    uint256 public currentBalance;
    
    // Coverage Configuration
    mapping(uint8 => CoverageConfig) public coverageConfigs;
    mapping(address => UserCoverage) public userCoverages;
    
    // Claims
    mapping(bytes32 => Claim) public claims;
    bytes32[] public claimIds;
    mapping(address => bytes32[]) public userClaims;
    
    // Premium
    mapping(address => uint256) public userPremiums;
    uint256 public premiumRate = 50; // 0.5%
    
    // Emergency
    bool public emergencyMode;
    bool public claimsPaused;
    
    // Events
    event Deposit(address indexed user, uint256 amount);
    event CoverageUpdated(address indexed user, uint256 coverage, uint8 coverageType);
    event ClaimSubmitted(bytes32 indexed claimId, address indexed user, uint256 amount);
    event ClaimApproved(bytes32 indexed claimId, uint256 amount);
    event ClaimRejected(bytes32 indexed claimId, string reason);
    event ClaimPaid(bytes32 indexed claimId, address indexed user, uint256 amount);
    event PremiumCollected(address indexed user, uint256 amount);
    event EmergencyMode(bool enabled);
    
    // ============== Structs ==============
    
    struct CoverageConfig {
        uint256 minCoverage;
        uint256 maxCoverage;
        uint256 premiumRate;
        uint256 maxAnnualPayout;
        uint256 currentPayout;
        bool active;
    }
    
    struct UserCoverage {
        address user;
        uint8 coverageType;
        uint256 coverageAmount;
        uint256 coverageStart;
        uint256 coverageEnd;
        bool active;
    }
    
    struct Claim {
        bytes32 id;
        address claimant;
        uint8 coverageType;
        uint256 amount;
        string description;
        bytes evidence;
        uint256 incidentTime;
        uint256 submitTime;
        uint8 status;
        uint256 approvedAmount;
        address approver;
        uint256 payTime;
    }
    
    // ============== Modifier ==============
    
    modifier onlyGovernance() {
        require(msg.sender == governance, "ONLY_GOVERNANCE");
        _;
    }
    
    modifier onlyClaimSigner() {
        require(msg.sender == claimSigner, "ONLY_CLAIM_SIGNER");
        _;
    }
    
    modifier whenNotPaused() {
        require(!claimsPaused, "CLAIMS_PAUSED");
        _;
    }
    
    // ============== Constructor ==============
    
    constructor() {
        governance = msg.sender;
        claimSigner = msg.sender;
        
        // Initialize coverage configs
        coverageConfigs[COVERAGE_SWAP] = CoverageConfig({
            minCoverage: MIN_COVERAGE,
            maxCoverage: MAX_COVERAGE,
            premiumRate: 50, // 0.5%
            maxAnnualPayout: 10000000e18,
            currentPayout: 0,
            active: true
        });
        
        coverageConfigs[COVERAGE_LIQUIDITY] = CoverageConfig({
            minCoverage: MIN_COVERAGE,
            maxCoverage: MAX_COVERAGE * 2,
            premiumRate: 75, // 0.75%
            maxAnnualPayout: 20000000e18,
            currentPayout: 0,
            active: true
        });
        
        coverageConfigs[COVERAGE_BRIDGE] = CoverageConfig({
            minCoverage: MIN_COVERAGE,
            maxCoverage: MAX_COVERAGE * 5,
            premiumRate: 100, // 1%
            maxAnnualPayout: 50000000e18,
            currentPayout: 0,
            active: true
        });
        
        coverageConfigs[COVERAGE_STAKING] = CoverageConfig({
            minCoverage: MIN_COVERAGE,
            maxCoverage: MAX_COVERAGE,
            premiumRate: 50, // 0.5%
            maxAnnualPayout: 10000000e18,
            currentPayout: 0,
            active: true
        });
    }
    
    // ============================================================================
    // Fund Management
    // ============================================================================
    
    /**
     * @notice Deposit funds to insurance
     */
    function deposit() external payable {
        require(msg.value > 0, "INVALID_AMOUNT");
        
        totalDeposits += msg.value;
        currentBalance += msg.value;
        
        emit Deposit(msg.sender, msg.value);
    }
    
    /**
     * @notice Withdraw funds (Governance only)
     */
    function withdraw(uint256 _amount) external onlyGovernance {
        require(_amount <= currentBalance, "INSUFFICIENT_BALANCE");
        
        currentBalance -= _amount;
        
        payable(msg.sender).transfer(_amount);
    }
    
    /**
     * @notice Get current balance
     */
    function getBalance() external view returns (uint256) {
        return currentBalance;
    }
    
    // ============================================================================
    // Coverage Management
    // ============================================================================
    
    /**
     * @notice Purchase coverage
     */
    function purchaseCoverage(uint8 _coverageType, uint256 _coverageAmount) external payable {
        CoverageConfig memory config = coverageConfigs[_coverageType];
        require(config.active, "COVERAGE_NOT_ACTIVE");
        require(_coverageAmount >= config.minCoverage, "BELOW_MIN");
        require(_coverageAmount <= config.maxCoverage, "ABOVE_MAX");
        
        // Calculate premium
        uint256 premium = _coverageAmount * config.premiumRate / PREMIUM_DENOMINATOR;
        require(msg.value >= premium, "INSUFFICIENT_PREMIUM");
        
        // Update user coverage
        userCoverages[msg.sender] = UserCoverage({
            user: msg.sender,
            coverageType: _coverageType,
            coverageAmount: _coverageAmount,
            coverageStart: block.timestamp,
            coverageEnd: block.timestamp + 365 days,
            active: true
        });
        
        // Record premium
        userPremiums[msg.sender] += premium;
        totalPremiumCollected += premium;
        
        // Refund excess
        if (msg.value > premium) {
            payable(msg.sender).transfer(msg.value - premium);
        }
        
        emit CoverageUpdated(msg.sender, _coverageAmount, _coverageType);
        emit PremiumCollected(msg.sender, premium);
    }
    
    /**
     * @notice Renew coverage
     */
    function renewCoverage() external payable {
        UserCoverage storage coverage = userCoverages[msg.sender];
        require(coverage.active, "NO_COVERAGE");
        require(block.timestamp > coverage.coverageEnd, "NOT_EXPIRED");
        
        CoverageConfig memory config = coverageConfigs[coverage.coverageType];
        uint256 premium = coverage.coverageAmount * config.premiumRate / PREMIUM_DENOMINATOR;
        require(msg.value >= premium, "INSUFFICIENT_PREMIUM");
        
        coverage.coverageStart = block.timestamp;
        coverage.coverageEnd = block.timestamp + 365 days;
        
        userPremiums[msg.sender] += premium;
        totalPremiumCollected += premium;
        
        emit PremiumCollected(msg.sender, premium);
    }
    
    /**
     * @notice Cancel coverage
     */
    function cancelCoverage() external {
        UserCoverage storage coverage = userCoverages[msg.sender];
        require(coverage.active, "NO_COVERAGE");
        
        coverage.active = false;
        
        emit CoverageUpdated(msg.sender, 0, coverage.coverageType);
    }
    
    // ============================================================================
    // Claims Management
    // ==========================================================================
    
    /**
     * @notice Submit a claim
     */
    function submitClaim(
        uint8 _coverageType,
        uint256 _amount,
        string calldata _description,
        bytes calldata _evidence,
        uint256 _incidentTime
    ) external whenNotPaused {
        require(!emergencyMode, "EMERGENCY_MODE");
        
        UserCoverage memory coverage = userCoverages[msg.sender];
        require(coverage.active, "NO_COVERAGE");
        require(coverage.coverageType == _coverageType, "WRONG_COVERAGE_TYPE");
        require(_incidentTime > coverage.coverageStart, "INCIDENT_BEFORE_COVERAGE");
        require(_incidentTime < block.timestamp, "INCIDENT_IN_FUTURE");
        require(_amount <= coverage.coverageAmount, "EXCEEDS_COVERAGE");
        
        // Verify incident time is recent
        require(block.timestamp - _incidentTime <= CLAIM_EXPIRY, "CLAIM_EXPIRED");
        
        bytes32 claimId = keccak256(abi.encodePacked(
            msg.sender,
            _coverageType,
            _amount,
            _incidentTime,
            block.timestamp,
            claimIds.length
        ));
        
        claims[claimId] = Claim({
            id: claimId,
            claimant: msg.sender,
            coverageType: _coverageType,
            amount: _amount,
            description: _description,
            evidence: _evidence,
            incidentTime: _incidentTime,
            submitTime: block.timestamp,
            status: STATUS_PENDING,
            approvedAmount: 0,
            approver: address(0),
            payTime: 0
        });
        
        claimIds.push(claimId);
        userClaims[msg.sender].push(claimId);
        
        emit ClaimSubmitted(claimId, msg.sender, _amount);
    }
    
    /**
     * @notice Approve claim (Claim signer only)
     */
    function approveClaim(bytes32 _claimId, uint256 _approvedAmount) external onlyClaimSigner {
        Claim storage claim = claims[_claimId];
        require(claim.status == STATUS_PENDING, "NOT_PENDING");
        require(_approvedAmount <= claim.amount, "EXCEEDS_CLAIM");
        
        CoverageConfig storage config = coverageConfigs[claim.coverageType];
        require(config.currentPayout + _approvedAmount <= config.maxAnnualPayout, "EXCEEDS_ANNUAL_LIMIT");
        require(_approvedAmount <= currentBalance, "INSUFFICIENT_FUNDS");
        
        claim.status = STATUS_APPROVED;
        claim.approvedAmount = _approvedAmount;
        claim.approver = msg.sender;
        
        config.currentPayout += _approvedAmount;
        
        emit ClaimApproved(_claimId, _approvedAmount);
    }
    
    /**
     * @notice Reject claim (Claim signer only)
     */
    function rejectClaim(bytes32 _claimId, string calldata _reason) external onlyClaimSigner {
        Claim storage claim = claims[_claimId];
        require(claim.status == STATUS_PENDING, "NOT_PENDING");
        
        claim.status = STATUS_REJECTED;
        
        emit ClaimRejected(_claimId, _reason);
    }
    
    /**
     * @notice Pay claim to claimant
     */
    function payClaim(bytes32 _claimId) external onlyGovernance {
        Claim storage claim = claims[_claimId];
        require(claim.status == STATUS_APPROVED, "NOT_APPROVED");
        require(claim.payTime == 0, "ALREADY_PAID");
        
        uint256 payoutAmount = claim.approvedAmount;
        require(payoutAmount <= currentBalance, "INSUFFICIENT_FUNDS");
        
        claim.status = STATUS_PAID;
        claim.payTime = block.timestamp;
        
        currentBalance -= payoutAmount;
        totalClaimsPaid += payoutAmount;
        
        payable(claim.claimant).transfer(payoutAmount);
        
        emit ClaimPaid(_claimId, claim.claimant, payoutAmount);
    }
    
    // ============================================================================
    // Governance Functions
    // ==========================================================================
    
    /**
     * @notice Set claim signer
     */
    function setClaimSigner(address _signer) external onlyGovernance {
        claimSigner = _signer;
    }
    
    /**
     * @notice Set coverage config
     */
    function setCoverageConfig(
        uint8 _coverageType,
        uint256 _minCoverage,
        uint256 _maxCoverage,
        uint256 _premiumRate,
        uint256 _maxAnnualPayout
    ) external onlyGovernance {
        CoverageConfig storage config = coverageConfigs[_coverageType];
        config.minCoverage = _minCoverage;
        config.maxCoverage = _maxCoverage;
        config.premiumRate = _premiumRate;
        config.maxAnnualPayout = _maxAnnualPayout;
    }
    
    /**
     * @notice Enable/disable coverage
     */
    function setCoverageActive(uint8 _coverageType, bool _active) external onlyGovernance {
        coverageConfigs[_coverageType].active = _active;
    }
    
    /**
     * @notice Enable emergency mode
     */
    function enableEmergencyMode() external onlyGovernance {
        emergencyMode = true;
        claimsPaused = true;
        
        emit EmergencyMode(true);
    }
    
    /**
     * @notice Disable emergency mode
     */
    function disableEmergencyMode() external onlyGovernance {
        emergencyMode = false;
        
        emit EmergencyMode(false);
    }
    
    /**
     * @notice Pause claims
     */
    function pauseClaims() external onlyGovernance {
        claimsPaused = true;
    }
    
    /**
     * @notice Resume claims
     */
    function resumeClaims() external onlyGovernance {
        claimsPaused = false;
    }
    
    /**
     * @notice Transfer governance
     */
    function transferGovernance(address _newGovernance) external onlyGovernance {
        pendingGovernance = _newGovernance;
    }
    
    /**
     * @notice Accept governance
     */
    function acceptGovernance() external {
        require(msg.sender == pendingGovernance, "NOT_PENDING");
        governance = msg.sender;
        delete pendingGovernance;
    }
    
    // ============================================================================
    // View Functions
    // ==========================================================================
    
    /**
     * @notice Get user coverage
     */
    function getUserCoverage(address _user) external view returns (
        uint8 coverageType,
        uint256 coverageAmount,
        uint256 coverageEnd,
        bool active
    ) {
        UserCoverage memory coverage = userCoverages[_user];
        return (
            coverage.coverageType,
            coverage.coverageAmount,
            coverage.coverageEnd,
            coverage.active
        );
    }
    
    /**
     * @notice Get claim details
     */
    function getClaim(bytes32 _claimId) external view returns (
        address claimant,
        uint8 coverageType,
        uint256 amount,
        uint256 approvedAmount,
        uint8 status,
        uint256 submitTime,
        uint256 payTime
    ) {
        Claim memory claim = claims[_claimId];
        return (
            claim.claimant,
            claim.coverageType,
            claim.amount,
            claim.approvedAmount,
            claim.status,
            claim.submitTime,
            claim.payTime
        );
    }
    
    /**
     * @notice Get user claims
     */
    function getUserClaims(address _user) external view returns (bytes32[] memory) {
        return userClaims[_user];
    }
    
    /**
     * @notice Get coverage config
     */
    function getCoverageConfig(uint8 _coverageType) external view returns (
        uint256 minCoverage,
        uint256 maxCoverage,
        uint256 premiumRate,
        uint256 maxAnnualPayout,
        bool active
    ) {
        CoverageConfig memory config = coverageConfigs[_coverageType];
        return (
            config.minCoverage,
            config.maxCoverage,
            config.premiumRate,
            config.maxAnnualPayout,
            config.active
        );
    }
    
    /**
     * @notice Get fund statistics
     */
    function getFundStats() external view returns (
        uint256 balance,
        uint256 totalDeposits_,
        uint256 totalClaimsPaid_,
        uint256 totalPremiumCollected_
    ) {
        return (
            currentBalance,
            totalDeposits,
            totalClaimsPaid,
            totalPremiumCollected
        );
    }
}