// SPDX-License-Identifier: MIT
pragma solidity ^0.8.19;

/**
 * TigerSwap Vault Contract
 * 
 * @author TigerSwap
 * @version 1.0.0
 */

/**
 * @title TigerVault
 * @dev Vault for managing protocol funds
 */
contract TigerVault {
    
    address public governor;
    address public pendingGovernor;
    
    mapping(address => bool) public guardians;
    mapping(address => bool) public whitelistedTokens;
    
    uint256 public constant DENOMINATOR = 10000;
    uint256 public withdrawalFee = 10; // 0.1%
    
    event GovernorChanged(address indexed oldGovernor, address indexed newGovernor);
    event GuardianAdded(address indexed guardian);
    event GuardianRemoved(address indexed guardian);
    event Deposit(address indexed user, address token, uint256 amount);
    event Withdrawal(address indexed user, address token, uint256 amount, uint256 fee);
    event TokenWhitelisted(address indexed token);
    
    constructor() {
        governor = msg.sender;
    }
    
    /**
     * @dev Set pending governor
     */
    function setPendingGovernor(address _pendingGovernor) external {
        require(msg.sender == governor, "Not governor");
        pendingGovernor = _pendingGovernor;
    }
    
    /**
     * @dev Accept governor
     */
    function acceptGovernor() external {
        require(msg.sender == pendingGovernor, "Not pending governor");
        address oldGovernor = governor;
        governor = msg.sender;
        pendingGovernor = address(0);
        emit GovernorChanged(oldGovernor, msg.sender);
    }
    
    /**
     * @dev Add guardian
     */
    function addGuardian(address _guardian) external {
        require(msg.sender == governor, "Not governor");
        guardians[_guardian] = true;
        emit GuardianAdded(_guardian);
    }
    
    /**
     * @dev Remove guardian
     */
    function removeGuardian(address _guardian) external {
        require(msg.sender == governor, "Not governor");
        guardians[_guardian] = false;
        emit GuardianRemoved(_guardian);
    }
    
    /**
     * @dev Whitelist token
     */
    function whitelistToken(address _token) external {
        require(msg.sender == governor || guardians[msg.sender], "Not authorized");
        whitelistedTokens[_token] = true;
        emit TokenWhitelisted(_token);
    }
    
    /**
     * @dev Deposit
     */
    function deposit(address _token, uint256 _amount) external {
        require(whitelistedTokens[_token], "Token not whitelisted");
        
        IERC20(_token).transferFrom(msg.sender, address(this), _amount);
        
        emit Deposit(msg.sender, _token, _amount);
    }
    
    /**
     * @dev Withdraw
     */
    function withdraw(address _token, uint256 _amount, address _recipient) external {
        require(msg.sender == governor || guardians[msg.sender], "Not authorized");
        
        uint256 fee = (_amount * withdrawalFee) / DENOMINATOR;
        uint256 amountAfterFee = _amount - fee;
        
        IERC20(_token).transfer(_recipient, amountAfterFee);
        
        emit Withdrawal(_recipient, _token, _amount, fee);
    }
    
    /**
     * @dev Set withdrawal fee
     */
    function setWithdrawalFee(uint256 _fee) external {
        require(msg.sender == governor, "Not governor");
        require(_fee <= DENOMINATOR / 10, "Fee too high");
        withdrawalFee = _fee;
    }
    
    /**
     * @dev Rescue funds
     */
    function rescueFunds(address _token, address _recipient, uint256 _amount) external {
        require(msg.sender == governor, "Not governor");
        IERC20(_token).transfer(_recipient, _amount);
    }
}

interface IERC20 {
    function transfer(address to, uint256 amount) external returns (bool);
    function transferFrom(address from, address to, uint256 amount) external returns (bool);
}