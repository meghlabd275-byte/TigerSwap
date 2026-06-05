// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "@openzeppelin/contracts/token/ERC20/IERC20.sol";

/**
 * @title TigerFarming
 * @notice Yield farming contract with multiple reward pools
 */
contract TigerFarming {
    struct PoolInfo {
        IERC20 lpToken;
        IERC20 rewardToken;
        uint256 allocPoint;
        uint256 lastRewardTime;
        uint256 accRewardPerShare;
        uint256 totalDeposits;
    }

    struct UserInfo {
        uint256 amount;
        uint256 rewardDebt;
        uint256 pending;
    }

    PoolInfo[] public pools;
    mapping(uint256 => mapping(address => UserInfo)) public users;
    
    IERC20 public tigerToken;
    address public governance;
    uint256 public constant REWARD_PER_SECOND = 1e18;
    uint256 public constant PRECISION = 1e18;
    
    uint256 public totalAllocPoint;
    uint256 public poolLength;

    event Deposit(address indexed user, uint256 indexed pid, uint256 amount);
    event Withdraw(address indexed user, uint256 indexed pid, uint256 amount);
    event EmergencyWithdraw(address indexed user, uint256 indexed pid, uint256 amount);
    event PoolAdded(uint256 indexed pid, address lpToken, address rewardToken);
    event SetPoolAllocPoint(uint256 indexed pid, uint256 allocPoint);

    constructor(address _tigerToken) {
        tigerToken = IERC20(_tigerToken);
        governance = msg.sender;
    }

    function addPool(IERC20 _lpToken, IERC20 _rewardToken, uint256 _allocPoint) external {
        require(msg.sender == governance, "TigerFarming: FORBIDDEN");
        _massUpdatePools();
        
        uint256 lastRewardTime = block.timestamp;
        pools.push(PoolInfo({
            lpToken: _lpToken,
            rewardToken: _rewardToken,
            allocPoint: _allocPoint,
            lastRewardTime: lastRewardTime,
            accRewardPerShare: 0,
            totalDeposits: 0
        }));
        
        totalAllocPoint += _allocPoint;
        poolLength++;
        
        emit PoolAdded(poolLength - 1, address(_lpToken), address(_rewardToken));
    }

    function setPoolAllocPoint(uint256 _pid, uint256 _allocPoint) external {
        require(msg.sender == governance, "TigerFarming: FORBIDDEN");
        _massUpdatePools();
        
        totalAllocPoint = totalAllocPoint - pools[_pid].allocPoint + _allocPoint;
        pools[_pid].allocPoint = _allocPoint;
        
        emit SetPoolAllocPoint(_pid, _allocPoint);
    }

    function deposit(uint256 _pid, uint256 _amount) external {
        PoolInfo storage pool = pools[_pid];
        UserInfo storage user = users[_pid][msg.sender];
        
        _updatePool(_pid);
        
        if (user.amount > 0) {
            uint256 pending = (user.amount * pool.accRewardPerShare / PRECISION) - user.rewardDebt;
            user.pending += pending;
        }
        
        if (_amount > 0) {
            pool.lpToken.transferFrom(address(msg.sender), address(this), _amount);
            user.amount += _amount;
            pool.totalDeposits += _amount;
        }
        
        user.rewardDebt = user.amount * pool.accRewardPerShare / PRECISION;
        emit Deposit(msg.sender, _pid, _amount);
    }

    function withdraw(uint256 _pid, uint256 _amount) external {
        PoolInfo storage pool = pools[_pid];
        UserInfo storage user = users[_pid][msg.sender];
        
        require(user.amount >= _amount, "TigerFarming: INSUFFICIENT");
        
        _updatePool(_pid);
        
        uint256 pending = (user.amount * pool.accRewardPerShare / PRECISION) - user.rewardDebt;
        user.pending += pending;
        
        if (_amount > 0) {
            user.amount -= _amount;
            pool.totalDeposits -= _amount;
            pool.lpToken.transfer(address(msg.sender), _amount);
        }
        
        user.rewardDebt = user.amount * pool.accRewardPerShare / PRECISION;
        emit Withdraw(msg.sender, _pid, _amount);
    }

    function claim(uint256 _pid) external {
        _updatePool(_pid);
        PoolInfo storage pool = pools[_pid];
        UserInfo storage user = users[_pid][msg.sender];
        
        uint256 pending = (user.amount * pool.accRewardPerShare / PRECISION) - user.rewardDebt;
        pending += user.pending;
        
        require(pending > 0, "TigerFarming: NO_REWARD");
        
        user.pending = 0;
        user.rewardDebt = user.amount * pool.accRewardPerShare / PRECISION;
        
        pool.rewardToken.transfer(msg.sender, pending);
    }

    function emergencyWithdraw(uint256 _pid) external {
        PoolInfo storage pool = pools[_pid];
        UserInfo storage user = users[_pid][msg.sender];
        
        uint256 amount = user.amount;
        user.amount = 0;
        user.rewardDebt = 0;
        user.pending = 0;
        pool.totalDeposits -= amount;
        
        pool.lpToken.transfer(address(msg.sender), amount);
        emit EmergencyWithdraw(msg.sender, _pid, amount);
    }

    function _updatePool(uint256 _pid) internal {
        PoolInfo storage pool = pools[_pid];
        if (block.timestamp > pool.lastRewardTime) {
            if (pool.totalDeposits > 0) {
                uint256 time = block.timestamp - pool.lastRewardTime;
                uint256 reward = time * REWARD_PER_SECOND * pool.allocPoint / totalAllocPoint;
                pool.accRewardPerShare += reward * PRECISION / pool.totalDeposits;
            }
            pool.lastRewardTime = block.timestamp;
        }
    }

    function _massUpdatePools() internal {
        uint256 length = poolLength;
        for (uint256 pid = 0; pid < length; pid++) {
            _updatePool(pid);
        }
    }

    function pendingReward(uint256 _pid, address _user) external view returns (uint256) {
        PoolInfo storage pool = pools[_pid];
        UserInfo storage user = users[_pid][_user];
        
        uint256 accRewardPerShare = pool.accRewardPerShare;
        if (block.timestamp > pool.lastRewardTime && pool.totalDeposits > 0) {
            uint256 time = block.timestamp - pool.lastRewardTime;
            uint256 reward = time * REWARD_PER_SECOND * pool.allocPoint / totalAllocPoint;
            accRewardPerShare += reward * PRECISION / pool.totalDeposits;
        }
        
        return (user.amount * accRewardPerShare / PRECISION) - user.rewardDebt + user.pending;
    }
}