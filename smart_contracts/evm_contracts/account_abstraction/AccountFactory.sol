// SPDX-License-Identifier: MIT
pragma solidity ^0.8.19;

/**
 * Account Factory
 * Creates smart contract accounts for users
 */

import "@openzeppelin/contracts/proxy/Clones.sol";
import "@openzeppelin/contracts/access/Ownable.sol";

/**
 * Simple Smart Contract Account
 */
contract SimpleAccount {
    address public owner;
    address public entryPoint;
    mapping(address => uint256) public nonces;
    
    event Executed(
        address indexed target,
        uint256 value,
        bytes data,
        bool success
    );
    
    constructor(address _entryPoint) {
        entryPoint = _entryPoint;
    }
    
    /**
     * Execute transaction through entry point
     */
    function execute(
        address target,
        uint256 value,
        bytes calldata data,
        bytes calldata signature
    ) external payable {
        require(msg.sender == entryPoint, "Only entry point");
        
        _execute(target, value, data);
    }
    
    /**
     * Execute batch
     */
    function executeBatch(
        address[] calldata targets,
        uint256[] calldata values,
        bytes[] calldata datas,
        bytes[] calldata signatures
    ) external payable {
        require(msg.sender == entryPoint, "Only entry point");
        require(targets.length == values.length, "Length mismatch");
        require(targets.length == datas.length, "Length mismatch");
        
        for (uint256 i = 0; i < targets.length; i++) {
            _execute(targets[i], values[i], datas[i]);
        }
    }
    
    /**
     * Internal execute
     */
    function _execute(
        address target,
        uint256 value,
        bytes memory data
    ) internal {
        (bool success, ) = target.call{value: value}(data);
        
        emit Executed(target, value, data, success);
    }
    
    /**
     * Validate user operation
     */
    function validateUserOp(
        address,
        bytes32,
        bytes calldata signature
    ) external pure returns (uint256) {
        // Simple signature validation
        if (signature.length == 64) {
            return 0;
        }
        return 1;
    }
    
    /**
     * Get nonce
     */
    function nonce() external view returns (uint256) {
        return nonces[msg.sender];
    }
    
    /**
     * Get chain ID
     */
    function getChainId() external view returns (uint256) {
        return block.chainid;
    }
    
    receive() external payable {}
}

/**
 * Account Factory
 */
contract AccountFactory is Ownable {
    using Clones for address;
    
    // Singleton implementation
    address public singletonTemplate;
    
    // Account mapping
    mapping(address => mapping(address => bool)) public isAccount;
    mapping(address => address[]) public accountsOf;
    
    // Events
    event AccountCreated(
        address indexed account,
        address indexed owner,
        uint256 salt
    );
    
    constructor(address _singletonTemplate) {
        require(_singletonTemplate != address(0), "Invalid template");
        singletonTemplate = _singletonTemplate;
    }
    
    /**
     * Create account with salt
     */
    function createAccount(
        address owner,
        uint256 salt
    ) public returns (address account) {
        // Get account address
        account = _getAccountAddress(owner, salt);
        
        // Deploy if not exists
        if (!isAccount[owner][account]) {
            SimpleAccount newAccount = SimpleAccount(
                Clones.clone(singletonTemplate)
            );
            
            // Initialize
            bytes memory initData = abi.encodeWithSelector(
                SimpleAccount(address(newAccount)).owner.selector,
                owner
            );
            
            (bool success, ) = address(newAccount).call(initData);
            require(success, "Init failed");
            
            // Register
            isAccount[owner][account] = true;
            accountsOf[owner].push(account);
            
            emit AccountCreated(account, owner, salt);
        }
    }
    
    /**
     * Get counterfactual account address
     */
    function getAccountAddress(
        address owner,
        uint256 salt
    ) external view returns (address account) {
        return _getAccountAddress(owner, salt);
    }
    
    /**
     * Internal account address calculation
     */
    function _getAccountAddress(
        address owner,
        uint256 salt
    ) internal view returns (address account) {
        bytes32 saltHash = keccak256(abi.encodePacked(owner, salt));
        
        bytes memory initCode = abi.encodePacked(
            singletonTemplate,
            abi.encodeWithSelector(SimpleAccount.owner.selector, owner)
        );
        
        bytes32 initCodeHash = keccak256(initCode);
        
        return address(
            uint160(
                uint256(
                    keccak256(
                        abi.encodePacked(
                            bytes1(0xff),
                            address(this),
                            saltHash,
                            initCodeHash
                        )
                    )
                )
            )
        );
    }
    
    /**
     * Get accounts for owner
     */
    function getAccounts(address owner)
        external view returns (address[] memory) {
        return accountsOf[owner];
    }
    
    /**
     * Update template
     */
    function updateTemplate(address newTemplate)
        external onlyOwner {
        require(newTemplate != address(0), "Invalid template");
        singletonTemplate = newTemplate;
    }
}

/**
 * Paymaster contract for sponsored transactions
 */
contract Paymaster is Ownable {
    using Clones for address;
    
    // Entry point
    address public entryPoint;
    
    // Stake management
    mapping(address => uint256) public deposits;
    mapping(address => uint256) public stakes;
    mapping(address => uint256) public unstakeDelay;
    
    // Configuration
    struct Config {
        uint256 minStake;
        uint256 minDeposit;
    }
    
    Config public config;
    
    // Whitelist
    mapping(address => bool) public whitelistedSenders;
    mapping(address => bool) public whitelistedPaymasters;
    
    // Events
    event Deposited(address indexed account, uint256 amount);
    event StakeLocked(address indexed account, uint256 stake);
    event StakeUnlocked(address indexed account);
    event Withdrawn(address indexed account, address recipient, uint256 amount);
    event WhitelistUpdated(address indexed account, bool status);
    
    constructor(address _entryPoint) {
        require(_entryPoint != address(0), "Invalid entry point");
        entryPoint = _entryPoint;
        
        // Default config
        config = Config({
            minStake: 0.1 ether,
            minDeposit: 0.05 ether
        });
    }
    
    /**
     * Deposit for sponsoring
     */
    function deposit() external payable {
        require(msg.value >= config.minDeposit, "Insufficient deposit");
        deposits[msg.sender] += msg.value;
        
        emit Deposited(msg.sender, msg.value);
    }
    
    /**
     * Add stake
     */
    function addStake(uint256 _unstakeDelay) external payable {
        require(msg.value >= config.minStake, "Insufficient stake");
        require(_unstakeDelay >= 12 hours, "Delay too short");
        
        stakes[msg.sender] += msg.value;
        unstakeDelay[msg.sender] = _unstakeDelay;
        
        emit StakeLocked(msg.sender, msg.value);
    }
    
    /**
     * Unlock stake
     */
    function unlockStake() external {
        require(stakes[msg.sender] > 0, "No stake");
        unstakeDelay[msg.sender] = block.timestamp + unstakeDelay[msg.sender];
        
        emit StakeUnlocked(msg.sender);
    }
    
    /**
     * Withdraw stake
     */
    function withdrawStake(address payable recipient) external {
        require(unstakeDelay[msg.sender] <= block.timestamp, "Stake locked");
        require(stakes[msg.sender] > 0, "No stake");
        
        uint256 amount = stakes[msg.sender];
        stakes[msg.sender] = 0;
        
        recipient.transfer(amount);
        
        emit Withdrawn(msg.sender, recipient, amount);
    }
    
    /**
     * Validate paymaster user operation
     */
    function validatePaymasterUserOp(
        UserOperation calldata op,
        bytes32,
        uint256
    ) external view returns (bytes memory) {
        // Check stake
        if (stakes[op.sender] < config.minStake) {
            return bytes("Insufficient stake");
        }
        
        // Check deposit
        if (deposits[op.sender] < config.minDeposit) {
            return bytes("Insufficient deposit");
        }
        
        return bytes("");
    }
    
    /**
     * Post operation hook
     */
    function postOp(
        bytes calldata context,
        uint256 actualGasCost
    ) external {
        // Refund gas used
    }
    
    /**
     * Update config
     */
    function setConfig(uint256 minStake_, uint256 minDeposit_)
        external onlyOwner {
        config = Config({
            minStake: minStake_,
            minDeposit: minDeposit_
        });
    }
    
    /**
     * Whitelist sender
     */
    function setWhitelistedSender(address sender, bool status)
        external onlyOwner {
        whitelistedSenders[sender] = status;
        emit WhitelistUpdated(sender, status);
    }
    
    /**
     * Whitelist paymaster
     */
    function setWhitelistedPaymaster(address paymaster, bool status)
        external onlyOwner {
        whitelistedPaymasters[paymaster] = status;
        emit WhitelistUpdated(paymaster, status);
    }
    
    receive() external payable {}
}

/**
 * Bundler - manages UserOperation bundles
 */
contract Bundler {
    // Entry point
    address public entryPoint;
    
    // Mempool
    struct UserOp {
        address sender;
        uint256 nonce;
        bytes initCode;
        bytes callData;
        uint256 callGasLimit;
        uint256 verificationGasLimit;
        uint256 preVerificationGas;
        uint256 maxFeePerGas;
        uint256 maxPriorityFeePerGas;
        address paymaster;
        bytes signature;
    }
    
    // Pending operations
    UserOp[] public pendingOps;
    
    // Events
    event UserOpReceived(address indexed sender, uint256 nonce);
    event UserOpBundled(address indexed bundler, uint256 count);
    event UserOpExecuted(address indexed sender, uint256 nonce, bool success);
    
    constructor(address _entryPoint) {
        require(_entryPoint != address(0), "Invalid entry point");
        entryPoint = _entryPoint;
    }
    
    /**
     * Add user operation to mempool
     */
    function addUserOp(UserOperation calldata op) external {
        pendingOps.push(op);
        
        emit UserOpReceived(op.sender, op.nonce);
    }
    
    /**
     * Bundle and send to entry point
     */
    function bundle() external {
        require(pendingOps.length > 0, "No pending ops");
        
        // Execute batch
        UserOperation[] memory ops = new UserOperation[](pendingOps.length);
        
        for (uint256 i = 0; i < pendingOps.length; i++) {
            ops[i] = pendingOps[i];
        }
        
        delete pendingOps;
        
        // Call entry point
        (bool success, ) = entryPoint.delegatecall(
            abi.encodeWithSelector(0x0, ops, new bytes[](0))
        );
        
        emit UserOpBundled(msg.sender, ops.length);
    }
    
    /**
     * Get pending count
     */
    function pendingCount() external view returns (uint256) {
        return pendingOps.length;
    }
    
    receive() external payable {}
}