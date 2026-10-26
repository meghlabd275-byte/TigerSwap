// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./TigerSwapInterfaces.sol";

/**
 * @title TigerSwapVoteEscrow
 * @notice Vote-escrowed TGR (veTGR) with gauge voting and a bribe market.
 *
 * Voting power decays linearly to zero over the lock duration. The global
 * supply is tracked with a bias/slope checkpoint plus a table of slope changes
 * registered at week boundaries, so `totalSupply` at any timestamp equals the
 * sum of every holder's `balanceOf` without iterating over holders.
 *
 * Both `balanceOf` and `totalSupply` use the same truncated slope
 * (`amount / MAXTIME`), so the two always agree exactly.
 */
contract TigerSwapVoteEscrow {
    IERC20 public immutable token;

    uint256 public constant MAXTIME = 4 * 365 * 86400; // 4 years
    uint256 public constant WEEK = 86400 * 7;
    uint256 public constant MIN_LOCK_TIME = WEEK;
    uint256 public constant MAX_ITERATIONS = 255; // MAXTIME / WEEK + 1

    struct Point {
        int128 bias;
        int128 slope;
        uint256 ts;
    }

    struct LockedBalance {
        int128 amount;
        uint256 start;
        uint256 end;
    }

    Point public pointHistory;
    mapping(uint256 => int128) public slopeChanges;
    mapping(address => LockedBalance) public locked;

    // ---- Gauge voting ------------------------------------------------------
    mapping(address => bool) public isGauge;
    address[] public gaugeList;
    mapping(address => uint256) public gaugeWeight; // gauge => current total weight
    mapping(address => mapping(address => uint256)) public userGaugeWeight; // user => gauge => weight
    mapping(address => uint256) public userTotalWeight;

    // Weights frozen per epoch (epoch = timestamp / WEEK) for bribe accounting.
    mapping(address => mapping(uint256 => uint256)) public epochGaugeWeight; // gauge => epoch => weight
    mapping(address => mapping(address => mapping(uint256 => uint256))) public epochUserWeight; // user => gauge => epoch

    // ---- Bribe market ------------------------------------------------------
    mapping(address => mapping(uint256 => address[])) private bribeTokenList;
    mapping(address => mapping(uint256 => mapping(address => bool))) private bribeTokenSeen;
    mapping(address => mapping(uint256 => mapping(address => uint256))) public bribeTotal; // gauge => epoch => token => amount
    mapping(address => mapping(address => mapping(uint256 => mapping(address => bool)))) public bribeClaimed;

    address public owner;
    address public pendingOwner;

    event LockCreated(address indexed user, uint256 amount, uint256 unlockTime);
    event LockIncreased(address indexed user, uint256 amount, uint256 unlockTime);
    event UnlockTimeIncreased(address indexed user, uint256 unlockTime);
    event Withdrawn(address indexed user, uint256 amount);
    event GaugeAdded(address indexed gauge);
    event Voted(address indexed user, address indexed gauge, uint256 weight, uint256 epoch);
    event BribeAdded(address indexed gauge, uint256 indexed epoch, address indexed token, uint256 amount, address funder);
    event BribeClaimed(address indexed user, address indexed gauge, uint256 indexed epoch, address token, uint256 amount);
    event OwnershipTransferStarted(address indexed previousOwner, address indexed newOwner);
    event OwnershipTransferred(address indexed previousOwner, address indexed newOwner);

    modifier onlyOwner() {
        require(msg.sender == owner, "not owner");
        _;
    }

    constructor(address _token) {
        require(_token != address(0), "token required");
        token = IERC20(_token);
        owner = msg.sender;
        emit OwnershipTransferred(address(0), msg.sender);
    }

    // ---- Reentrancy guard --------------------------------------------------

    uint256 private _status = 1;

    modifier nonReentrant() {
        require(_status == 1, "reentrant call");
        _status = 2;
        _;
        _status = 1;
    }

    // ---- Voting power ------------------------------------------------------

    /// @notice Voting power of `_addr`, decaying linearly from the lock start.
    function balanceOf(address _addr) public view returns (uint256) {
        LockedBalance memory l = locked[_addr];
        uint256 t = block.timestamp;
        if (l.end <= t || l.amount <= 0) {
            return 0;
        }
        uint256 amount = uint256(uint128(l.amount));
        uint256 slope = amount / MAXTIME;
        uint256 bias = slope * (l.end - l.start);
        uint256 decayed = slope * (t - l.start);
        return decayed >= bias ? 0 : bias - decayed;
    }

    /// @notice Total voting power across all holders at the current timestamp.
    function totalSupply() external view returns (uint256) {
        return uint256(uint128(_supplyAt(block.timestamp)));
    }

    function _supplyAt(uint256 _ts) internal view returns (int128) {
        Point memory last = pointHistory;
        if (last.ts == 0) {
            return 0;
        }
        uint256 lastWeek = (last.ts / WEEK) * WEEK;
        for (uint256 i = 0; i < MAX_ITERATIONS; i++) {
            lastWeek += WEEK;
            if (lastWeek > _ts) {
                lastWeek = _ts;
            }
            int128 slopeChange = slopeChanges[lastWeek];
            int128 bias = last.bias - last.slope * int128(uint128(lastWeek - last.ts));
            if (bias < 0) {
                bias = 0;
            }
            int128 slope = last.slope + slopeChange;
            if (slope < 0) {
                slope = 0;
            }
            last.bias = bias;
            last.slope = slope;
            last.ts = lastWeek;
            if (lastWeek == _ts) {
                break;
            }
        }
        return last.bias;
    }

    /// @notice Advance the global checkpoint to the current block.
    function checkpoint() public {
        Point memory last = pointHistory;
        uint256 t = block.timestamp;
        if (last.ts == 0) {
            last.ts = t;
            pointHistory = last;
            return;
        }
        uint256 lastWeek = (last.ts / WEEK) * WEEK;
        for (uint256 i = 0; i < MAX_ITERATIONS; i++) {
            lastWeek += WEEK;
            if (lastWeek > t) {
                lastWeek = t;
            }
            int128 slopeChange = slopeChanges[lastWeek];
            int128 bias = last.bias - last.slope * int128(uint128(lastWeek - last.ts));
            if (bias < 0) {
                bias = 0;
            }
            int128 slope = last.slope + slopeChange;
            if (slope < 0) {
                slope = 0;
            }
            last.bias = bias;
            last.slope = slope;
            last.ts = lastWeek;
            if (lastWeek == t) {
                break;
            }
        }
        pointHistory = last;
    }

    // ---- Locking -----------------------------------------------------------

    /// @notice Create a lock. `_unlockTime` is floored to a week boundary.
    function createLock(uint256 _amount, uint256 _unlockTime) external nonReentrant {
        require(locked[msg.sender].amount == 0, "existing lock");
        _createLock(_amount, _unlockTime);
    }

    /// @notice Add TGR to an existing lock.
    function increaseAmount(uint256 _amount) external nonReentrant {
        LockedBalance memory old = locked[msg.sender];
        require(old.amount > 0, "no existing lock");
        require(old.end > block.timestamp, "lock expired");
        _depositFor(msg.sender, _amount, old.end, old, _typeIncreaseAmount());
    }

    /// @notice Extend an existing lock's unlock time.
    function increaseUnlockTime(uint256 _unlockTime) external nonReentrant {
        LockedBalance memory old = locked[msg.sender];
        require(old.amount > 0, "no existing lock");
        require(old.end > block.timestamp, "lock expired");
        uint256 unlockTime = _roundUpToWeek(_unlockTime);
        require(unlockTime > old.end, "only increase");
        _depositFor(msg.sender, 0, unlockTime, old, _typeIncreaseUnlockTime());
    }

    /// @notice Withdraw the full locked amount after expiry.
    function withdraw() external nonReentrant {
        LockedBalance memory old = locked[msg.sender];
        require(old.amount > 0, "no lock");
        require(old.end <= block.timestamp, "lock not expired");

        uint256 amount = uint256(uint128(old.amount));
        locked[msg.sender] = LockedBalance(0, 0, 0);

        _checkpoint(msg.sender, old, LockedBalance(0, 0, 0));

        require(token.transfer(msg.sender, amount), "transfer failed");
        emit Withdrawn(msg.sender, amount);
    }

    function _typeCreateLock() internal pure returns (uint8) {
        return 0;
    }

    function _typeIncreaseAmount() internal pure returns (uint8) {
        return 1;
    }

    function _typeIncreaseUnlockTime() internal pure returns (uint8) {
        return 2;
    }

    function _createLock(uint256 _amount, uint256 _unlockTime) internal {
        uint256 unlockTime = _roundUpToWeek(_unlockTime);
        require(_amount > 0, "amount required");
        require(unlockTime > block.timestamp, "unlock in past");
        require(unlockTime >= block.timestamp + MIN_LOCK_TIME, "lock too short");

        _depositFor(msg.sender, _amount, unlockTime, LockedBalance(0, 0, 0), _typeCreateLock());
    }

    function _depositFor(
        address _addr,
        uint256 _amount,
        uint256 _unlockTime,
        LockedBalance memory _old,
        uint8 _lockType
    ) internal {
        // `_old` must not be aliased: a memory-to-memory struct assignment in
        // Solidity copies the reference, so mutating `next` would also mutate
        // `_old` and collapse the checkpoint deltas to zero.
        LockedBalance memory next;
        next.amount = _old.amount;
        next.start = _old.start;
        next.end = _old.end;

        if (_lockType == _typeCreateLock()) {
            next.amount = int128(uint128(_amount));
            next.start = block.timestamp;
            next.end = _unlockTime;
            emit LockCreated(_addr, _amount, _unlockTime);
        } else if (_lockType == _typeIncreaseAmount()) {
            next.amount = _old.amount + int128(uint128(_amount));
            emit LockIncreased(_addr, _amount, next.end);
        } else {
            next.end = _unlockTime;
            emit UnlockTimeIncreased(_addr, _unlockTime);
        }

        locked[_addr] = next;

        if (_amount > 0) {
            require(token.transferFrom(msg.sender, address(this), _amount), "transferFrom failed");
        }

        _checkpoint(_addr, _old, next);
    }

    /// @dev Round a requested unlock time up to the next week boundary, and
    /// clamp so a maximum-duration request still fits inside MAXTIME.
    function _roundUpToWeek(uint256 _ts) internal view returns (uint256) {
        uint256 floored = (_ts / WEEK) * WEEK;
        uint256 rounded = floored == _ts ? floored : floored + WEEK;
        uint256 maxEnd = ((block.timestamp + MAXTIME) / WEEK) * WEEK;
        return rounded > maxEnd ? maxEnd : rounded;
    }

    /// @dev Apply the change between `_old` and `_new` to the global checkpoint.
    /// The global bias must move by the same delta as the user's own bias,
    /// otherwise `totalSupply` drifts away from the sum of all balances.
    function _checkpoint(address _addr, LockedBalance memory _old, LockedBalance memory _new) internal {
        Point memory last = pointHistory;
        uint256 t = block.timestamp;

        if (last.ts == 0) {
            last.ts = t;
        }

        int128 oldSlope;
        int128 oldBias;
        if (_old.end > t && _old.amount > 0) {
            oldSlope = _old.amount / int128(uint128(MAXTIME));
            oldBias = oldSlope * int128(uint128(_old.end - _old.start));
        }
        int128 newSlope;
        int128 newBias;
        if (_new.end > t && _new.amount > 0) {
            newSlope = _new.amount / int128(uint128(MAXTIME));
            newBias = newSlope * int128(uint128(_new.end - _new.start));
        }

        // Advance the checkpoint to now, then apply both deltas.
        last.bias -= last.slope * int128(uint128(t - last.ts));
        if (last.bias < 0) {
            last.bias = 0;
        }
        last.bias += newBias - oldBias;
        if (last.bias < 0) {
            last.bias = 0;
        }
        last.slope += newSlope - oldSlope;
        if (last.slope < 0) {
            last.slope = 0;
        }
        last.ts = t;
        pointHistory = last;

        if (_old.end > t && oldSlope > 0) {
            slopeChanges[(_old.end / WEEK) * WEEK] -= oldSlope;
        }
        if (_new.end > t && newSlope > 0) {
            slopeChanges[(_new.end / WEEK) * WEEK] += newSlope;
        }

        emit Checkpoint(_addr, last.bias, last.slope);
    }

    event Checkpoint(address indexed user, int128 bias, int128 slope);

    // ---- Gauges ------------------------------------------------------------

    function addGauge(address _gauge) external onlyOwner {
        require(_gauge != address(0), "gauge required");
        require(!isGauge[_gauge], "gauge exists");
        isGauge[_gauge] = true;
        gaugeList.push(_gauge);
        emit GaugeAdded(_gauge);
    }

    function gaugeCount() external view returns (uint256) {
        return gaugeList.length;
    }

    /// @notice Set this caller's vote weight for a gauge.
    function vote(address _gauge, uint256 _weight) external {
        require(isGauge[_gauge], "not a gauge");

        uint256 power = balanceOf(msg.sender);
        uint256 previous = userGaugeWeight[msg.sender][_gauge];
        uint256 newTotal = userTotalWeight[msg.sender] - previous + _weight;
        require(newTotal <= power, "insufficient voting power");

        userGaugeWeight[msg.sender][_gauge] = _weight;
        userTotalWeight[msg.sender] = newTotal;
        gaugeWeight[_gauge] = gaugeWeight[_gauge] - previous + _weight;

        uint256 epoch = block.timestamp / WEEK;
        uint256 previousThisEpoch = epochUserWeight[msg.sender][_gauge][epoch];
        epochGaugeWeight[_gauge][epoch] = epochGaugeWeight[_gauge][epoch] - previousThisEpoch + _weight;
        epochUserWeight[msg.sender][_gauge][epoch] = _weight;

        emit Voted(msg.sender, _gauge, _weight, epoch);
    }

    // ---- Bribes ------------------------------------------------------------

    /// @notice Fund a bribe for the current epoch of a gauge.
    function addBribe(address _gauge, address _token, uint256 _amount) external nonReentrant {
        require(isGauge[_gauge], "not a gauge");
        require(_token != address(0), "token required");
        require(_amount > 0, "amount required");

        uint256 epoch = block.timestamp / WEEK;
        if (!bribeTokenSeen[_gauge][epoch][_token]) {
            bribeTokenSeen[_gauge][epoch][_token] = true;
            bribeTokenList[_gauge][epoch].push(_token);
        }
        bribeTotal[_gauge][epoch][_token] += _amount;

        require(IERC20(_token).transferFrom(msg.sender, address(this), _amount), "bribe transfer failed");

        emit BribeAdded(_gauge, epoch, _token, _amount, msg.sender);
    }

    function bribeTokenCount(address _gauge, uint256 _epoch) external view returns (uint256) {
        return bribeTokenList[_gauge][_epoch].length;
    }

    function bribeTokenAt(address _gauge, uint256 _epoch, uint256 _index) external view returns (address) {
        return bribeTokenList[_gauge][_epoch][_index];
    }

    /// @notice Claim a pro-rata share of an epoch's bribe for a gauge.
    function claimBribe(address _gauge, uint256 _epoch, address _token) external nonReentrant returns (uint256) {
        require(!bribeClaimed[msg.sender][_gauge][_epoch][_token], "already claimed");

        uint256 totalWeight = epochGaugeWeight[_gauge][_epoch];
        uint256 userWeight = epochUserWeight[msg.sender][_gauge][_epoch];
        uint256 total = bribeTotal[_gauge][_epoch][_token];
        require(total > 0 && totalWeight > 0 && userWeight > 0, "nothing to claim");

        uint256 share = (total * userWeight) / totalWeight;
        require(share > 0, "share is zero");

        bribeClaimed[msg.sender][_gauge][_epoch][_token] = true;
        require(IERC20(_token).transfer(msg.sender, share), "bribe payout failed");

        emit BribeClaimed(msg.sender, _gauge, _epoch, _token, share);
        return share;
    }

    // ---- Boost -------------------------------------------------------------

    /// @notice Farm boost multiplier in basis points (10000 = 1x, 25000 = 2.5x).
    /// @dev 1x + 1.5x * (userVeTGR / userStaked) capped at 2.5x.
    function boostBps(address _user, uint256 _stakedAmount) external view returns (uint256) {
        if (_stakedAmount == 0) {
            return 10000;
        }
        uint256 ve = balanceOf(_user);
        uint256 bonus = (15000 * ve) / _stakedAmount;
        uint256 total = 10000 + bonus;
        return total > 25000 ? 25000 : total;
    }

    // ---- Ownership ---------------------------------------------------------

    function transferOwnership(address _newOwner) external onlyOwner {
        require(_newOwner != address(0), "zero address");
        pendingOwner = _newOwner;
        emit OwnershipTransferStarted(owner, _newOwner);
    }

    function acceptOwnership() external {
        require(msg.sender == pendingOwner, "not pending owner");
        address previous = owner;
        owner = pendingOwner;
        pendingOwner = address(0);
        emit OwnershipTransferred(previous, owner);
    }
}
