// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/**
 * @title TigerWalletFactory
 * @notice Factory for creating user wallets and master wallet
 * @dev Creates and manages all TigerWallet and TigerMaster instances
 */
import "../libraries/SafeMath.sol";

contract TigerWalletFactory {
    using SafeMath for uint256;

    // ============================================================================
    // State Variables
    // ============================================================================

    address public admin;
    address public pendingAdmin;
    
    // Master wallet
    address public masterWallet;
    bool public masterInitialized;
    
    // User wallets
    mapping(address => bool) public isUserWallet;
    address[] public userWallets;
    uint256 public userWalletCount = 0;
    
    // Wallet implementation
    address public walletImplementation;
    address public masterImplementation;
    
    // Wallet creation fees
    uint256 public walletCreationFee = 0;
    
    // Events
    event MasterWalletCreated(address indexed master);
    event UserWalletCreated(address indexed wallet, address indexed owner);
    event ImplementationUpdated(address indexed oldImpl, address indexed newImpl);
    
    // ============================================================================
    // Constructor
    // ============================================================================

    constructor(address _admin) {
        admin = _admin;
    }

    // ============================================================================
    // Master Wallet
    // ============================================================================

    /**
     * @notice Create master wallet (only once)
     */
    function createMasterWallet(
        address masterAddress,
        bytes memory encryptedSeed,
        bytes32 walletHash,
        string memory name,
        string memory backupCode
    ) external returns (address) {
        require(!masterInitialized, "Master already initialized");
        require(msg.sender == admin, "Not authorized");
        
        // Deploy master wallet
        address master = address(new TigerMaster(
            masterAddress,
            encryptedSeed,
            walletHash,
            name,
            backupCode
        ));
        
        masterWallet = master;
        masterInitialized = true;
        
        emit MasterWalletCreated(master);
        
        return master;
    }

    /**
     * @notice Get master wallet
     */
    function getMasterWallet() external view returns (address) {
        return masterWallet;
    }

    // ============================================================================
    // User Wallet
    // ============================================================================

    /**
     * @notice Create user wallet
     */
    function createUserWallet(
        address walletOwner,
        bytes memory encryptedSeed,
        bytes32 walletHash,
        string memory name
    ) external returns (address) {
        require(isUserWallet[walletOwner] == false, "Wallet exists");
        
        // Deploy user wallet
        address wallet = address(new TigerWallet(
            masterWallet,
            address(this),
            encryptedSeed,
            walletHash,
            name
        ));
        
        isUserWallet[wallet] = true;
        userWallets.push(wallet);
        userWalletCount++;
        
        emit UserWalletCreated(wallet, walletOwner);
        
        return wallet;
    }

    /**
     * @notice Get all user wallets
     */
    function getAllUserWallets() external view returns (address[] memory) {
        return userWallets;
    }

    /**
     * @notice Get user wallet count
     */
    function getUserWalletCount() external view returns (uint256) {
        return userWalletCount;
    }

    // ============================================================================
    // Admin
    // ============================================================================

    function setAdmin(address newAdmin) external {
        require(msg.sender == admin, "Not admin");
        pendingAdmin = newAdmin;
    }

    function acceptAdmin() external {
        require(msg.sender == pendingAdmin, "Not pending admin");
        admin = pendingAdmin;
    }

    function setWalletImplementation(address _implementation) external {
        require(msg.sender == admin, "Not admin");
        address oldImpl = walletImplementation;
        walletImplementation = _implementation;
        emit ImplementationUpdated(oldImpl, _implementation);
    }

    function setMasterImplementation(address _implementation) external {
        require(msg.sender == admin, "Not admin");
        address oldImpl = masterImplementation;
        masterImplementation = _implementation;
        emit ImplementationUpdated(oldImpl, _implementation);
    }

    function setWalletCreationFee(uint256 _fee) external {
        require(msg.sender == admin, "Not admin");
        walletCreationFee = _fee;
    }
}

// Placeholder imports (actual contracts would be imported)
contract TigerWallet {
    constructor(address masterWallet, address walletFactory, bytes memory encryptedSeed, bytes32 walletHash, string memory name) {}
}

contract TigerMaster {
    constructor(address masterAddress, bytes memory encryptedSeed, bytes32 walletHash, string memory name, string memory backupCode) {}
}