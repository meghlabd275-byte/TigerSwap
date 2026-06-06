// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/**
 * @title TigerTokenFactory
 * @notice Token Factory with Verification
 * @dev Create and verify ERC-20 tokens
 * 
 * Features:
 * - Token creation with presets
 * - KYC verification integration
 * - Minting rights management
 * - Token metadata
 * - Automatic listing
 * - Supply caps
 */
contract TigerTokenFactory {
    // ============================================================================
    // Constants
    // ============================================================================
    
    uint256 constant MAX_SUPPLY = 1e27; // Max token supply
    uint256 constant MAX_MINTERS = 20;
    uint256 constant MAX_BURNERS = 20;
    
    // Token type
    uint8 constant TOKEN_TYPE_STANDARD = 1;
    uint8 constant TOKEN_TYPE_MINTABLE = 2;
    uint8 constant TOKEN_TYPE_BURNABLE = 3;
    uint8 constant TOKEN_TYPE_PAUSEABLE = 4;
    uint8 constant TOKEN_TYPE_WHITELIST = 5;
    
    // Token status
    uint8 constant STATUS_PENDING = 1;
    uint8 constant STATUS_VERIFIED = 2;
    uint8 constant STATUS_SUSPENDED = 3;
    uint8 constant STATUS_LISTED = 4;
    
    // ============================================================================
    // State Variables
    // ============================================================================
    
    // Governance
    address public governance;
    address public pendingGovernance;
    
    // Token registry
    mapping(address => TokenInfo) public tokenInfo;
    address[] public tokens;
    mapping(address => bool) public isToken;
    
    // Verification
    mapping(address => bool) public verifiedTokens;
    mapping(address => mapping(address => bool)) public tokenVerifiers;
    mapping(address => uint256) public verificationLevel;
    
    // KYC
    mapping(address => bool) public kycProviders;
    mapping(address => mapping(address => bool)) public kycVerified;
    
    // Factory settings
    uint256 public protocolFee = 0.1 ether;
    uint256 public verificationFee = 0.05 ether;
    bool public requireVerification = true;
    
    // Events
    event TokenCreated(
        address indexed token,
        address indexed creator,
        string name,
        string symbol,
        uint256 supply
    );
    event TokenVerified(
        address indexed token,
        address indexed verifier
    );
    event TokenSuspended(
        address indexed token,
        address indexed suspender
    );
    event TokenListed(
        address indexed token,
        uint256 listingTime
    );
    event MinterAdded(
        address indexed token,
        address indexed minter
    );
    event MinterRemoved(
        address indexed token,
        address indexed minter
    );
    event KYCProviderAdded(address indexed provider);
    event KYCProviderRemoved(address indexed provider);
    
    // ============== Structs ==============
    
    struct TokenInfo {
        address token;
        address creator;
        string name;
        string symbol;
        uint8 decimals;
        uint256 totalSupply;
        uint8 tokenType;
        uint8 status;
        uint256 createdAt;
        bool isListed;
        bool isVerified;
        bytes32 ipfsHash;
    }
    
    struct TokenConfig {
        string name;
        string symbol;
        uint8 decimals;
        uint256 initialSupply;
        uint256 maxSupply;
        uint8 tokenType;
        bool mintable;
        bool burnable;
        bool pausable;
        bool whitelist;
        bool upgradeable;
    }
    
    struct TokenMetadata {
        string website;
        string description;
        string image;
        string telegram;
        string twitter;
        string discord;
        bytes32 ipfsHash;
    }
    
    mapping(address => TokenMetadata) public tokenMetadata;
    mapping(address => address[]) public tokenMinters;
    mapping(address => address[]) public tokenBurners;
    
    // ============== Modifier ==============
    
    modifier onlyGovernance() {
        require(msg.sender == governance, "ONLY_GOVERNANCE");
        _;
    }
    
    // ============== Constructor ==============
    
    constructor() {
        governance = msg.sender;
    }
    
    // ============================================================================
    // Token Creation
    // ============================================================================
    
    /**
     * @notice Create new token
     */
    function createToken(
        TokenConfig calldata _config,
        string calldata _website,
        string calldata _description,
        string calldata _image
    ) external payable returns (address token) {
        require(msg.value >= protocolFee, "INSUFFICIENT_FEE");
        
        // Validate config
        require(bytes(_config.name).length > 0, "INVALID_NAME");
        require(bytes(_config.symbol).length >= 2, "INVALID_SYMBOL");
        require(_config.decimals >= 1 && _config.decimals <= 18, "INVALID_DECIMALS");
        require(_config.maxSupply <= MAX_SUPPLY, "MAX_SUPPLY");
        
        // Create token based on type
        if (_config.tokenType == TOKEN_TYPE_MINTABLE) {
            token = address(new MintableToken(
                _config.name,
                _config.symbol,
                _config.decimals,
                _config.maxSupply,
                msg.sender
            ));
        } else if (_config.tokenType == TOKEN_TYPE_BURNABLE) {
            token = address(new BurnableToken(
                _config.name,
                _config.symbol,
                _config.decimals,
                _config.initialSupply,
                msg.sender
            ));
        } else if (_config.tokenType == TOKEN_TYPE_PAUSEABLE) {
            token = address(new PausableToken(
                _config.name,
                _config.symbol,
                _config.decimals,
                _config.initialSupply,
                msg.sender
            ));
        } else if (_config.tokenType == TOKEN_TYPE_WHITELIST) {
            token = address(new WhitelistToken(
                _config.name,
                _config.symbol,
                _config.decimals,
                _config.initialSupply,
                msg.sender
            ));
        } else {
            token = address(new StandardToken(
                _config.name,
                _config.symbol,
                _config.decimals,
                _config.initialSupply,
                msg.sender
            ));
        }
        
        // Register token
        isToken[token] = true;
        tokens.push(token);
        
        tokenInfo[token] = TokenInfo({
            token: token,
            creator: msg.sender,
            name: _config.name,
            symbol: _config.symbol,
            decimals: _config.decimals,
            totalSupply: _config.initialSupply,
            tokenType: _config.tokenType,
            status: STATUS_PENDING,
            createdAt: block.timestamp,
            isListed: false,
            isVerified: false,
            ipfsHash: bytes32(0)
        });
        
        // Set metadata
        tokenMetadata[token] = TokenMetadata({
            website: _website,
            description: _description,
            image: _image,
            telegram: "",
            twitter: "",
            discord: "",
            ipfsHash: bytes32(0)
        });
        
        emit TokenCreated(
            token,
            msg.sender,
            _config.name,
            _config.symbol,
            _config.initialSupply
        );
        
        // Auto-verify if not required
        if (!requireVerification) {
            verifiedTokens[token] = true;
            tokenInfo[token].isVerified = true;
            tokenInfo[token].status = STATUS_VERIFIED;
        }
        
        // Refund excess
        if (msg.value > protocolFee) {
            payable(msg.sender).transfer(msg.value - protocolFee);
        }
    }
    
    // ============================================================================
    // Token Verification
    // ============================================================================
    
    /**
     * @notice Verify token
     */
    function verifyToken(address _token) external {
        require(isToken[_token], "NOT_TOKEN");
        require(!verifiedTokens[_token], "ALREADY_VERIFIED");
        
        // Check if caller is KYC provider or governance
        if (kycProviders[msg.sender]) {
            verifiedTokens[_token] = true;
            tokenInfo[_token].isVerified = true;
            tokenInfo[_token].status = STATUS_VERIFIED;
            
            emit TokenVerified(_token, msg.sender);
        }
    }
    
    /**
     * @notice Batch verify tokens
     */
    function batchVerifyTokens(address[] calldata _tokens) external {
        for (uint256 i = 0; i < _tokens.length; i++) {
            verifyToken(_tokens[i]);
        }
    }
    
    // ============================================================================
    // Token Management
    // ============================================================================
    
    /**
     * @notice Add minter to token
     */
    function addMinter(address _token, address _minter) external {
        require(isToken[_token], "NOT_TOKEN");
        require(tokenInfo[_token].creator == msg.sender, "NOT_CREATOR");
        
        TokenInfo storage info = tokenInfo[_token];
        require(info.tokenType == TOKEN_TYPE_MINTABLE, "NOT_MINTABLE");
        
        // Add minter
        tokenMinters[_token].push(_minter);
        
        emit MinterAdded(_token, _minter);
    }
    
    /**
     * @notice Remove minter from token
     */
    function removeMinter(address _token, address _minter) external {
        require(isToken[_token], "NOT_TOKEN");
        require(tokenInfo[_token].creator == msg.sender, "NOT_CREATOR");
        
        emit MinterRemoved(_token, _minter);
    }
    
    /**
     * @notice List token on DEX
     */
    function listToken(address _token) external {
        require(isToken[_token], "NOT_TOKEN");
        TokenInfo storage info = tokenInfo[_token];
        require(info.creator == msg.sender, "NOT_CREATOR");
        require(!info.isListed, "ALREADY_LISTED");
        
        info.isListed = true;
        info.status = STATUS_LISTED;
        
        emit TokenListed(_token, block.timestamp);
    }
    
    /**
     * @notice Suspend token
     */
    function suspendToken(address _token) external onlyGovernance {
        require(isToken[_token], "NOT_TOKEN");
        
        tokenInfo[_token].status = STATUS_SUSPENDED;
        
        emit TokenSuspended(_token, msg.sender);
    }
    
    // ============================================================================
    // KYC Management
    // ============================================================================
    
    /**
     * @notice Add KYC provider
     */
    function addKYCProvider(address _provider) external onlyGovernance {
        require(_provider != address(0), "INVALID_PROVIDER");
        require(!kycProviders[_provider], "ALREADY_PROVIDER");
        
        kycProviders[_provider] = true;
        
        emit KYCProviderAdded(_provider);
    }
    
    /**
     * @notice Remove KYC provider
     */
    function removeKYCProvider(address _provider) external onlyGovernance {
        require(kycProviders[_provider], "NOT_PROVIDER");
        
        kycProviders[_provider] = false;
        
        emit KYCProviderRemoved(_provider);
    }
    
    /**
     * @notice Verify KYC for user on token
     */
    function verifyKYC(address _token, address _user) external {
        require(kycProviders[msg.sender], "NOT_KYC_PROVIDER");
        
        kycVerified[_token][_user] = true;
    }
    
    // ============================================================================
    // View Functions
    // ============================================================================
    
    /**
     * @notice Get token info
     */
    function getTokenInfo(address _token) external view returns (
        address creator,
        string memory name,
        string memory symbol,
        uint8 decimals,
        uint256 totalSupply,
        uint8 tokenType,
        uint8 status,
        bool isVerified,
        bool isListed
    ) {
        TokenInfo storage info = tokenInfo[_token];
        return (
            info.creator,
            info.name,
            info.symbol,
            info.decimals,
            info.totalSupply,
            info.tokenType,
            info.status,
            info.isVerified,
            info.isListed
        );
    }
    
    /**
     * @notice Get token metadata
     */
    function getTokenMetadata(address _token) external view returns (
        string memory website,
        string memory description,
        string memory image
    ) {
        TokenMetadata storage meta = tokenMetadata[_token];
        return (
            meta.website,
            meta.description,
            meta.image
        );
    }
    
    /**
     * @notice Get all tokens
     */
    function getAllTokens() external view returns (address[] memory) {
        return tokens;
    }
    
    /**
     * @notice Get token count
     */
    function getTokenCount() external view returns (uint256) {
        return tokens.length;
    }
    
    /**
     * @notice Check if token is verified
     */
    function isVerified(address _token) external view returns (bool) {
        return verifiedTokens[_token];
    }
    
    // ============================================================================
    // Governance
    // ============================================================================
    
    /**
     * @notice Set protocol fee
     */
    function setProtocolFee(uint256 _fee) external onlyGovernance {
        protocolFee = _fee;
    }
    
    /**
     * @notice Set verification fee
     */
    function setVerificationFee(uint256 _fee) external onlyGovernance {
        verificationFee = _fee;
    }
    
    /**
     * @notice Set require verification
     */
    function setRequireVerification(bool _required) external onlyGovernance {
        requireVerification = _required;
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
    }
}

// ============================================================================
// Token Implementations
// ============================================================================

contract StandardToken {
    string public name;
    string public symbol;
    uint8 public decimals;
    uint256 public totalSupply;
    mapping(address => uint256) public balanceOf;
    mapping(address => mapping(address => uint256)) public allowance;
    
    event Transfer(address indexed from, address indexed to, uint256 value);
    event Approval(address indexed owner, address indexed spender, uint256 value);
    
    constructor(
        string memory _name,
        string memory _symbol,
        uint8 _decimals,
        uint256 _supply,
        address _creator
    ) {
        name = _name;
        symbol = _symbol;
        decimals = _decimals;
        totalSupply = _supply;
        balanceOf[_creator] = _supply;
    }
    
    function transfer(address to, uint256 amount) external returns (bool) {
        require(balanceOf[msg.sender] >= amount, "INSUFFICIENT_BALANCE");
        balanceOf[msg.sender] -= amount;
        balanceOf[to] += amount;
        emit Transfer(msg.sender, to, amount);
        return true;
    }
    
    function approve(address spender, uint256 amount) external returns (bool) {
        allowance[msg.sender][spender] = amount;
        emit Approval(msg.sender, spender, amount);
        return true;
    }
    
    function transferFrom(address from, address to, uint256 amount) external returns (bool) {
        require(balanceOf[from] >= amount, "INSUFFICIENT_BALANCE");
        require(allowance[from][msg.sender] >= amount, "ALLOWANCE_EXCEEDED");
        balanceOf[from] -= amount;
        balanceOf[to] += amount;
        allowance[from][msg.sender] -= amount;
        emit Transfer(from, to, amount);
        return true;
    }
}

contract MintableToken is StandardToken {
    mapping(address => bool) public minters;
    uint256 public cap;
    bool public mintingEnabled = true;
    
    event Mint(address indexed to, uint256 amount);
    
    constructor(
        string memory _name,
        string memory _symbol,
        uint8 _decimals,
        uint256 _cap,
        address _creator
    ) StandardToken(_name, _symbol, _decimals, 0, _creator) {
        cap = _cap;
        minters[_creator] = true;
    }
    
    function mint(address to, uint256 amount) external {
        require(minters[msg.sender], "NOT_MINTER");
        require(mintingEnabled, "MINTING_DISABLED");
        require(totalSupply + amount <= cap, "CAP_EXCEEDED");
        
        totalSupply += amount;
        balanceOf[to] += amount;
        emit Mint(to, amount);
        emit Transfer(address(0), to, amount);
    }
}

contract BurnableToken is StandardToken {
    event Burn(address indexed from, uint256 amount);
    
    constructor(
        string memory _name,
        string memory _symbol,
        uint8 _decimals,
        uint256 _supply,
        address _creator
    ) StandardToken(_name, _symbol, _decimals, _supply, _creator) {}
    
    function burn(uint256 amount) external {
        require(balanceOf[msg.sender] >= amount, "INSUFFICIENT_BALANCE");
        balanceOf[msg.sender] -= amount;
        totalSupply -= amount;
        emit Burn(msg.sender, amount);
    }
}

contract PausableToken is StandardToken {
    bool public paused;
    mapping(address => bool) public minters;
    
    event Pause();
    event Unpause();
    
    constructor(
        string memory _name,
        string memory _symbol,
        uint8 _decimals,
        uint256 _supply,
        address _creator
    ) StandardToken(_name, _symbol, _decimals, _supply, _creator) {
        minters[_creator] = true;
    }
    
    function pause() external {
        require(minters[msg.sender], "NOT_MINTER");
        paused = true;
        emit Pause();
    }
    
    function unpause() external {
        require(minters[msg.sender], "NOT_MINTER");
        paused = false;
        emit Unpause();
    }
    
    function transfer(address to, uint256 amount) public override returns (bool) {
        require(!paused, "PAUSED");
        return super.transfer(to, amount);
    }
}

contract WhitelistToken is StandardToken {
    mapping(address => bool) public whitelist;
    mapping(address => bool) public operators;
    
    event WhitelistAdded(address indexed user);
    event WhitelistRemoved(address indexed user);
    
    constructor(
        string memory _name,
        string memory _symbol,
        uint8 _decimals,
        uint256 _supply,
        address _creator
    ) StandardToken(_name, _symbol, _decimals, _supply, _creator) {
        operators[_creator] = true;
        whitelist[_creator] = true;
    }
    
    function addToWhitelist(address _user) external {
        require(operators[msg.sender], "NOT_OPERATOR");
        whitelist[_user] = true;
        emit WhitelistAdded(_user);
    }
    
    function removeFromWhitelist(address _user) external {
        require(operators[msg.sender], "NOT_OPERATOR");
        whitelist[_user] = false;
        emit WhitelistRemoved(_user);
    }
    
    function transfer(address to, uint256 amount) public override returns (bool) {
        require(whitelist[msg.sender] && whitelist[to], "NOT_WHITELISTED");
        return super.transfer(to, amount);
    }
}