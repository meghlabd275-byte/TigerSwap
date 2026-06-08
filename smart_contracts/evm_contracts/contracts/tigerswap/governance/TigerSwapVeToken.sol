// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import {IERC20} from "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import {SafeERC20} from "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";
import {ReentrancyGuard} from "@openzeppelin/contracts/utils/ReentrancyGuard.sol";
import {Ownable} from "@openzeppelin/contracts/access/Ownable.sol";
import {Math} from "@openzeppelin/contracts/utils/math/Math.sol";

/**
 * @title TigerSwapVeToken
 * @notice veToken (Vote-Escrow) Governance System - Similar to Curve/Velodrome/Aerodrome
 * @dev Implements:
 * - Token locking for voting power
 * - Gauge voting for liquidity emissions
 * - Bribe system for vote buying
 * - Time-locked withdrawals
 */
contract TigerSwapVeToken is ReentrancyGuard, Ownable {
    using SafeERC20 for IERC20;

    // ============ Constants ============
    uint256 public constant MAX_LOCK_TIME = 4 * 365 days; // 4 years max
    uint256 public constant MIN_LOCK_TIME = 1 weeks;
    uint256 public constant WEEK = 1 weeks;
    uint256 public constant MAX_BPS = 10000;

    // ============ State ============
    IERC20 public token;
    string public name;
    string public symbol;

    // Lock data
    struct LockedBalance {
        uint256 amount;
        uint256 end;
        uint256 multiplier;
    }

    mapping(address => LockedBalance) public locked;
    mapping(address => uint256) public balanceOf;
    mapping(address => mapping(uint256 => uint256)) public votes;
    mapping(address => uint256[]) public voteHistory;

    uint256 public totalSupply;
    uint256 public totalLocked;

    // Epoch tracking
    uint256 public epoch;

    // Gauge system
    struct Gauge {
        address pool;
        uint256 weight;
        uint256 earned;
        uint256 totalWeight;
        bool active;
    }

    mapping(address => Gauge) public gauges;
    address[] public gaugeList;
    uint256 public totalGaugeWeight;

    // Bribe system
    struct Bribe {
        address token;
        uint256 amount;
        uint256 epoch;
        uint256 totalVotes;
    }

    mapping(address => mapping(uint256 => Bribe)) public bribes;
    mapping(address => uint256[]) public bribeEpochs;

    // ============ Events ============
    event LockCreated(address indexed user, uint256 amount, uint256 lockTime, uint256 end);
    event LockExtended(address indexed user, uint256 newEnd);
    event LockWithdrawn(address indexed user, uint256 amount);
    event VoteCast(address indexed user, address indexed gauge, uint256 weight, uint256 epoch);
    event GaugeWeightUpdated(address indexed gauge, uint256 weight, uint256 totalWeight);
    event BribeDeposited(address indexed gauge, address indexed token, uint256 amount, uint256 epoch);
    event BribeClaimed(address indexed user, address indexed gauge, address indexed token, uint256 amount);

    // ============ Constructor ============
    constructor(address _token, string memory _name, string memory _symbol, address _owner) Ownable(_owner) {
        require(_token != address(0), "Invalid token");
        
        token = IERC20(_token);
        name = _name;
        symbol = _symbol;
    }

    // ============ Lock Functions ============
    function createLock(uint256 _amount, uint256 _lockTime) external nonReentrant {
        require(_amount > 0, "Amount is 0");
        require(_lockTime >= MIN_LOCK_TIME, "Lock too short");
        require(_lockTime <= MAX_LOCK_TIME, "Lock too long");
        
        LockedBalance storage lock = locked[msg.sender];
        require(lock.amount == 0, "Already locked");
        
        token.safeTransferFrom(msg.sender, address(this), _amount);
        
        uint256 end = ((block.timestamp + _lockTime) / WEEK) * WEEK;
        
        lock.amount = _amount;
        lock.end = end;
        lock.multiplier = _calculateMultiplier(_lockTime);
        
        _updateVotingPower(msg.sender);
        totalLocked += _amount;
        
        emit LockCreated(msg.sender, _amount, _lockTime, end);
    }

    function increaseLockAmount(uint256 _amount) external nonReentrant {
        require(_amount > 0, "Amount is 0");
        
        LockedBalance storage lock = locked[msg.sender];
        require(lock.amount > 0, "No lock");
        require(lock.end > block.timestamp, "Lock expired");
        
        token.safeTransferFrom(msg.sender, address(this), _amount);
        
        lock.amount += _amount;
        _updateVotingPower(msg.sender);
        totalLocked += _amount;
    }

    function extendLock(uint256 _newLockTime) external nonReentrant {
        require(_newLockTime >= MIN_LOCK_TIME, "Lock too short");
        require(_newLockTime <= MAX_LOCK_TIME, "Lock too long");
        
        LockedBalance storage lock = locked[msg.sender];
        require(lock.amount > 0, "No lock");
        
        uint256 newEnd = ((block.timestamp + _newLockTime) / WEEK) * WEEK;
        require(newEnd > lock.end, "Cannot shorten");
        
        lock.end = newEnd;
        lock.multiplier = _calculateMultiplier(_newLockTime);
        
        _updateVotingPower(msg.sender);
        
        emit LockExtended(msg.sender, newEnd);
    }

    function withdraw() external nonReentrant {
        LockedBalance storage lock = locked[msg.sender];
        require(lock.amount > 0, "No lock");
        require(lock.end <= block.timestamp, "Lock not expired");
        
        uint256 amount = lock.amount;
        
        lock.amount = 0;
        lock.end = 0;
        lock.multiplier = 0;
        
        _updateVotingPower(msg.sender);
        totalLocked -= amount;
        
        token.safeTransfer(msg.sender, amount);
        
        emit LockWithdrawn(msg.sender, amount);
    }

    // ============ Voting Functions ============
    function vote(address[] calldata _gauges, uint256[] calldata _weights) external nonReentrant {
        require(_gauges.length == _weights.length, "Length mismatch");
        
        uint256 totalWeight = 0;
        for (uint256 i = 0; i < _weights.length; i++) {
            totalWeight += _weights[i];
        }
        require(totalWeight == MAX_BPS, "Weights must sum to 10000");
        
        // Remove previous votes
        uint256[] storage history = voteHistory[msg.sender];
        for (uint256 i = 0; i < history.length; i++) {
            address gauge = address(uint160(history[i]));
            uint256 prevWeight = votes[msg.sender][gauge];
            if (prevWeight > 0) {
                gauges[gauge].weight -= prevWeight;
                totalGaugeWeight -= prevWeight;
            }
        }
        
        delete voteHistory[msg.sender];
        
        for (uint256 i = 0; i < _gauges.length; i++) {
            address gauge = _gauges[i];
            uint256 weight = _weights[i];
            
            require(gauges[gauge].active, "Gauge not active");
            
            votes[msg.sender][gauge] = weight;
            voteHistory[msg.sender].push(uint256(uint160(gauge)));
            
            gauges[gauge].weight += weight;
            totalGaugeWeight += weight;
            
            emit VoteCast(msg.sender, gauge, weight, epoch);
        }
    }

    // ============ Gauge Functions ============
    function addGauge(address _pool, uint256 _weight) external onlyOwner {
        require(_pool != address(0), "Invalid pool");
        require(!gauges[_pool].active, "Gauge exists");
        
        Gauge storage gauge = gauges[_pool];
        gauge.pool = _pool;
        gauge.weight = _weight;
        gauge.active = true;
        
        gaugeList.push(_pool);
        totalGaugeWeight += _weight;
        
        emit GaugeWeightUpdated(_pool, _weight, totalGaugeWeight);
    }

    function setGaugeWeight(address _gauge, uint256 _weight) external onlyOwner {
        require(gauges[_gauge].active, "Gauge not active");
        
        Gauge storage gauge = gauges[_gauge];
        
        if (gauge.weight > 0) {
            totalGaugeWeight -= gauge.weight;
        }
        
        gauge.weight = _weight;
        totalGaugeWeight += _weight;
        
        emit GaugeWeightUpdated(_gauge, _weight, totalGaugeWeight);
    }

    // ============ Bribe Functions ============
    function depositBribe(address _gauge, address _token, uint256 _amount, uint256 _epoch) external nonReentrant {
        require(_amount > 0, "Amount is 0");
        require(gauges[_gauge].active, "Gauge not active");
        
        if (_epoch == 0) {
            _epoch = (block.timestamp / WEEK) * WEEK;
        }
        
        IERC20(_token).safeTransferFrom(msg.sender, address(this), _amount);
        
        Bribe storage bribe = bribes[_gauge][_epoch];
        bribe.token = _token;
        bribe.amount += _amount;
        bribe.epoch = _epoch;
        bribe.totalVotes += gauges[_gauge].weight;
        
        bool found = false;
        for (uint256 i = 0; i < bribeEpochs[_gauge].length; i++) {
            if (bribeEpochs[_gauge][i] == _epoch) {
                found = true;
                break;
            }
        }
        if (!found) {
            bribeEpochs[_gauge].push(_epoch);
        }
        
        emit BribeDeposited(_gauge, _token, _amount, _epoch);
    }

    function claimBribes(address[] calldata _gauges, uint256[] calldata _epochs) external nonReentrant returns (uint256[] memory amounts) {
        require(_gauges.length == _epochs.length, "Length mismatch");
        
        amounts = new uint256[](_gauges.length);
        
        for (uint256 i = 0; i < _gauges.length; i++) {
            address gauge = _gauges[i];
            uint256 epoch = _epochs[i];
            
            Bribe storage bribe = bribes[gauge][epoch];
            require(bribe.amount > 0, "No bribe");
            
            uint256 userVotes = votes[msg.sender][gauge];
            require(userVotes > 0, "No votes");
            
            uint256 claimable = (bribe.amount * userVotes) / bribe.totalVotes;
            require(claimable > 0, "Nothing to claim");
            
            bribe.amount -= claimable;
            bribe.totalVotes -= userVotes;
            
            IERC20(bribe.token).safeTransfer(msg.sender, claimable);
            
            amounts[i] = claimable;
            
            emit BribeClaimed(msg.sender, gauge, bribe.token, claimable);
        }
        
        return amounts;
    }

    // ============ Internal Functions ============
    function _calculateMultiplier(uint256 _lockTime) internal pure returns (uint256) {
        return (MAX_BPS * _lockTime) / MAX_LOCK_TIME + MAX_BPS;
    }

    function _updateVotingPower(address _user) internal {
        LockedBalance storage lock = locked[_user];
        
        uint256 oldPower = balanceOf[_user];
        
        if (lock.amount > 0 && lock.end > block.timestamp) {
            balanceOf[_user] = (lock.amount * lock.multiplier) / MAX_BPS;
        } else {
            balanceOf[_user] = 0;
        }
        
        totalSupply = totalSupply - oldPower + balanceOf[_user];
    }

    // ============ View Functions ============
    function getVotes(address _user, address _gauge) external view returns (uint256) {
        return votes[_user][_gauge];
    }

    function getGaugeList() external view returns (address[] memory) {
        return gaugeList;
    }

    function getVotingPower(address _user) external view returns (uint256) {
        return balanceOf[_user];
    }

    function getLockInfo(address _user) external view returns (uint256 amount, uint256 end, uint256 multiplier, uint256 votingPower) {
        LockedBalance storage lock = locked[_user];
        return (lock.amount, lock.end, lock.multiplier, balanceOf[_user]);
    }

    function getEpoch() external view returns (uint256) {
        return (block.timestamp / WEEK) * WEEK;
    }
}