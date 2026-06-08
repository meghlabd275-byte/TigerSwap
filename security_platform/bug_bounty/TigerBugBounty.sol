// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";
import "@openzeppelin/contracts/access/AccessControl.sol";
import "@openzeppelin/contracts/utils/ReentrancyGuard.sol";

/**
 * @title TigerBugBounty
 * @notice Bug Bounty Program Contract
 * @dev Security vulnerability reward program
 */

contract TigerBugBounty is AccessControl, ReentrancyGuard {
    using SafeERC20 for IERC20;
    
    bytes32 public constant WARDEN_ROLE = keccak256("WARDEN_ROLE");
    bytes32 public constant JUDGE_ROLE = keccak256("JUDGE_ROLE");
    
    // Severity levels and rewards
    enum Severity { Low, Medium, High, Critical }
    
    // Reward tiers (in USDC equivalent)
    uint256 public constant REWARD_LOW = 1e6;      // $1,000
    uint256 public constant REWARD_MEDIUM = 5e6;    // $5,000
    uint256 public constant REWARD_HIGH = 25e6;     // $25,000
    uint256 public constant REWARD_CRITICAL = 100e6; // $100,000
    
    // State
    uint256 public bountyPool;
    bool public paused;
    address public usdc;
    
    // Submissions
    mapping(uint256 => BugSubmission) public submissions;
    uint256 public submissionCount;
    
    // Hacker rewards (to prevent double-claims)
    mapping(address => uint256) public claimedRewards;
    mapping(address => bool) public whitelistedHackers;
    
    // Events
    event SubmissionSubmitted(uint256 indexed id, address indexed hacker, Severity severity, string description);
    event SubmissionConfirmed(uint256 indexed id, Severity severity);
    event SubmissionRejected(uint256 indexed id, string reason);
    event RewardClaimed(uint256 indexed id, address indexed hacker, uint256 amount);
    event BountyAdded(uint256 amount);
    event HackerWhitelisted(address indexed hacker);
    
    struct BugSubmission {
        uint256 id;
        address hacker;
        string description;
        string mitigation;
        Severity severity;
        uint256 rewardAmount;
        bool resolved;
        bool rewarded;
        uint256 timestamp;
    }
    
    modifier onlyJudges() {
        require(hasRole(JUDGE_ROLE, msg.sender), "Not judge");
        _;
    }
    
    modifier whenNotPaused() {
        require(!paused, "Paused");
        _;
    }
    
    constructor(address _admin, address _usdc) {
        require(_usdc != address(0), "Invalid USDC");
        usdc = _usdc;
        
        _grantRole(DEFAULT_ADMIN_ROLE, _admin);
        _grantRole(WARDEN_ROLE, _admin);
        _grantRole(JUDGE_ROLE, _admin);
    }
    
    /**
     * @notice Submit a bug report
     * @param description Bug description (IPFS hash or description)
     * @param mitigation Suggested fix
     */
    function submit(string calldata description, string calldata mitigation) 
        external 
        whenNotPaused 
    {
        require(bytes(description).length > 0, "Empty description");
        
        uint256 id = ++submissionCount;
        
        submissions[id] = BugSubmission({
            id: id,
            hacker: msg.sender,
            description: description,
            mitigation: mitigation,
            severity: Severity.Low, // Default until judged
            rewardAmount: 0,
            resolved: false,
            rewarded: false,
            timestamp: block.timestamp
        });
        
        emit SubmissionSubmitted(id, msg.sender, Severity.Low, description);
    }
    
    /**
     * @notice Judge a submission
     * @param id Submission ID
     * @param severity Determined severity
     */
    function judge(uint256 id, Severity severity) external onlyJudges whenNotPaused {
        BugSubmission storage sub = submissions[id];
        require(sub.hacker != address(0), "Submission not found");
        require(!sub.resolved, "Already resolved");
        
        sub.severity = severity;
        
        // Calculate reward
        uint256 reward;
        if (severity == Severity.Low) {
            reward = REWARD_LOW;
        } else if (severity == Severity.Medium) {
            reward = REWARD_MEDIUM;
        } else if (severity == Severity.High) {
            reward = REWARD_HIGH;
        } else {
            reward = REWARD_CRITICAL;
        }
        
        sub.rewardAmount = reward;
        sub.resolved = true;
        
        emit SubmissionConfirmed(id, severity);
    }
    
    /**
     * @notice Reject a submission
     * @param id Submission ID
     * @param reason Rejection reason
     */
    function reject(uint256 id, string calldata reason) 
        external 
        onlyJudges 
    {
        BugSubmission storage sub = submissions[id];
        require(sub.hacker != address(0), "Submission not found");
        require(!sub.resolved, "Already resolved");
        
        sub.resolved = true;
        
        emit SubmissionRejected(id, reason);
    }
    
    /**
     * @notice Claim reward
     * @param id Submission ID
     */
    function claim(uint256 id) external nonReentrant whenNotPaused {
        BugSubmission storage sub = submissions[id];
        require(sub.hacker == msg.sender, "Not hacker");
        require(sub.resolved, "Not resolved");
        require(!sub.rewarded, "Already rewarded");
        require(sub.rewardAmount > 0, "No reward");
        require(bountyPool >= sub.rewardAmount, "Insufficient pool");
        
        sub.rewarded = true;
        claimedRewards[msg.sender] += sub.rewardAmount;
        bountyPool -= sub.rewardAmount;
        
        // Transfer reward
        IERC20(usdc).safeTransfer(msg.sender, sub.rewardAmount);
        
        emit RewardClaimed(id, msg.sender, sub.rewardAmount);
    }
    
    /**
     * @notice Add to bounty pool
     * @param amount Amount to add
     */
    function addBounty(uint256 amount) external onlyRole(DEFAULT_ADMIN_ROLE) {
        require(amount > 0, "Amount is 0");
        
        IERC20(usdc).safeTransferFrom(msg.sender, address(this), amount);
        bountyPool += amount;
        
        emit BountyAdded(amount);
    }
    
    /**
     * @notice Whitelist hacker (for special cases)
     * @param hacker Address to whitelist
     */
    function whitelistHacker(address hacker) external onlyRole(DEFAULT_ADMIN_ROLE) {
        whitelistedHackers[hacker] = true;
        emit HackerWhitelisted(hacker);
    }
    
    /**
     * @notice Get submission details
     * @param id Submission ID
     * @return Submission struct
     */
    function getSubmission(uint256 id) external view returns (BugSubmission memory) {
        return submissions[id];
    }
    
    /**
     * @notice Get total claimed by address
     * @param hacker Hacker address
     * @return Total claimed
     */
    function getClaimed(address hacker) external view returns (uint256) {
        return claimedRewards[hacker];
    }
    
    /**
     * @notice Pause program
     */
    function pause() external onlyRole(DEFAULT_ADMIN_ROLE) {
        paused = true;
    }
    
    /**
     * @notice Unpause program
     */
    function unpause() external onlyRole(DEFAULT_ADMIN_ROLE) {
        paused = false;
    }
    
    /**
     * @notice Rescue funds (emergency)
     * @param token Token to rescue
     * @param amount Amount
     */
    function rescue(address token, uint256 amount) external onlyRole(DEFAULT_ADMIN_ROLE) {
        if (token == address(0)) {
            payable(msg.sender).transfer(amount);
        } else {
            IERC20(token).safeTransfer(msg.sender, amount);
        }
    }
}