// SPDX-License-Identifier: MIT
pragma solidity ^0.8.24;

import "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import "@openzeppelin/contracts/token/ERC20/extensions/ERC20Burnable.sol";
import "@openzeppelin/contracts/token/ERC20/extensions/ERC20Snapshot.sol";
import "@openzeppelin/contracts/access/AccessControl.sol";
import "@openzeppelin/contracts/security/Pausable.sol";
import "@openzeppelin/contracts/utils/Address.sol";

/**
 * @title TIGER Governance Token
 * @notice TigerSwap Protocol Token with governance capabilities
 * @dev Implements ERC-20 with governance, voting, and delegation features
 */
contract TIGERToken is ERC20, ERC20Burnable, ERC20Snapshot, AccessControl, Pausable {
    bytes32 public constant MINTER_ROLE = keccak256("MINTER_ROLE");
    bytes32 public constant SNAPSHOT_MANAGER_ROLE = keccak256("SNAPSHOT_MANAGER_ROLE");
    
    // Maximum supply: 1 billion tokens
    uint256 public constant MAX_SUPPLY = 1_000_000_000 * 1e18;
    
    // Domain separators for EIP-712
    bytes32 public constant DOMAIN_TYPEHASH = keccak256("EIP712Domain(string name,uint256 chainId,address verifyingContract)");
    bytes32 public constant DELEGATION_TYPEHASH = keccak256("Delegation(address delegatee,uint256 nonce,uint256 expiry)");
    bytes32 public constant VOTE_TYPEHASH = keccak256("Vote(address voter,address delegatee,bool support,uint256 nonce)");
    
    // Chain id for domain separator
    uint256 public chainId;
    
    // Mapping for delegated votes
    mapping(address => address) public delegates;
    mapping(address => mapping(uint256 => Checkpoint)) public checkpoints;
    mapping(address => uint256) public checkpointCounts;
    
    // Nonces for EIP-712 signatures
    mapping(address => uint256) public nonces;
    
    // Proposal voting powers
    mapping(address => uint256) public proposalVotes;
    
    // Treasury address
    address public treasury;
    
    // Emission rate (per second)
    uint256 public emissionRate;
    uint256 public lastUpdateTime;
    
    // Events
    event DelegateChanged(address indexed delegator, address indexed fromDelegate, address indexed toDelegate);
    event DelegateVotesChanged(address indexed delegate, uint256 previousVotes, uint256 newVotes);
    event ProposalVoteCast(address indexed voter, uint256 proposalId, bool support, uint256 weight);
    event TreasuryUpdated(address indexed newTreasury);
    event EmissionRateUpdated(uint256 newRate);
    event SnapshotCreated(uint256 indexed snapshotId);
    
    struct Checkpoint {
        uint256 fromBlock;
        uint256 votes;
    }
    
    modifier onlyMinter() {
        require(hasRole(MINTER_ROLE, msg.sender), "Caller is not a minter");
        _;
    }
    
    modifier onlySnapshotManager() {
        require(hasRole(SNAPSHOT_MANAGER_ROLE, msg.sender), "Caller is not a snapshot manager");
        _;
    }
    
    constructor(
        string memory name_,
        string memory symbol_,
        address _treasury,
        address _admin
    ) ERC20(name_, symbol_) {
        require(_treasury != address(0), "Invalid treasury address");
        
        treasury = _treasury;
        chainId = block.chainid;
        
        // Grant admin role to deployer
        _grantRole(DEFAULT_ADMIN_ROLE, _admin);
        _grantRole(MINTER_ROLE, _admin);
        _grantRole(SNAPSHOT_MANAGER_ROLE, _admin);
        
        // Initial supply to treasury (10% for team, 90% for protocol)
        uint256 initialSupply = MAX_SUPPLY;
        _mint(treasury, initialSupply);
        
        // Set initial emission rate
        emissionRate = 0; // No automatic emission
        lastUpdateTime = block.timestamp;
    }
    
    /**
     * @notice Delegate votes to another address
     * @param delegatee The address to delegate votes to
     */
    function delegate(address delegatee) external {
        return _delegate(msg.sender, delegatee);
    }
    
    /**
     * @notice Delegate votes using EIP-712 signature
     * @param delegatee The address to delegate votes to
     * @param nonce The nonce for the signature
     * @param expiry The expiry time for the signature
     * @param v The recovery byte of the signature
     * @param r Half of the ECDSA signature
     * @param s Half of the ECDSA signature
     */
    function delegateBySig(
        address delegatee,
        uint256 nonce,
        uint256 expiry,
        uint8 v,
        bytes32 r,
        bytes32 s
    ) external {
        bytes32 domainSeparator = keccak256(
            abi.encode(
                DOMAIN_TYPEHASH,
                keccak256(bytes(name())),
                chainId,
                address(this)
            )
        );
        
        bytes32 structHash = keccak256(
            abi.encode(
                DELEGATION_TYPEHASH,
                delegatee,
                nonce,
                expiry
            )
        );
        
        bytes32 digest = keccak256(abi.encodePacked("\x19\x01", domainSeparator, structHash));
        
        address signatory = ecrecover(digest, v, r, s);
        require(signatory != address(0), "Invalid signature");
        require(nonce == nonces[signatory]++, "Invalid nonce");
        require(block.timestamp <= expiry, "Signature expired");
        
        return _delegate(signatory, delegatee);
    }
    
    /**
     * @notice Get the current votes for an account
     * @param account The account to check
     * @return The current votes for the account
     */
    function getVotes(address account) public view returns (uint256) {
        uint256 checkpointCount = checkpointCounts[account];
        if (checkpointCount == 0) {
            return 0;
        }
        
        // Get the most recent checkpoint
        Checkpoint memory checkpoint = checkpoints[account][checkpointCount];
        return checkpoint.votes;
    }
    
    /**
     * @notice Get the prior votes for an account at a specific block
     * @param account The account to check
     * @param blockNumber The block to check at
     * @return The votes at that block
     */
    function getPriorVotes(address account, uint256 blockNumber) external view returns (uint256) {
        require(blockNumber < block.number, "Block not yet determined");
        
        uint256 checkpointCount = checkpointCounts[account];
        if (checkpointCount == 0) {
            return 0;
        }
        
        // Find the checkpoint at or before the block
        uint256 index = checkpointCount;
        for (uint256 i = checkpointCount; i > 0; i--) {
            Checkpoint memory checkpoint = checkpoints[account][i];
            if (checkpoint.fromBlock <= blockNumber) {
                return checkpoint.votes;
            }
            index--;
        }
        
        return 0;
    }
    
    /**
     * @notice Cast a vote on a proposal
     * @param proposalId The proposal to vote on
     * @param support Whether to support the proposal
     */
    function castVote(uint256 proposalId, bool support) external {
        uint256 weight = getVotes(msg.sender);
        require(weight > 0, "No voting power");
        
        if (support) {
            proposalVotes[msg.sender] += weight;
        }
        
        emit ProposalVoteCast(msg.sender, proposalId, support, weight);
    }
    
    /**
     * @notice Mint new tokens
     * @param to The address to mint to
     * @param amount The amount to mint
     */
    function mint(address to, uint256 amount) external onlyMinter whenNotPaused {
        require(totalSupply() + amount <= MAX_SUPPLY, "Exceeds max supply");
        _mint(to, amount);
    }
    
    /**
     * @notice Burn tokens from an account
     * @param from The address to burn from
     * @param amount The amount to burn
     */
    function burnFrom(address from, uint256 amount) external whenNotPaused {
        _spendAllowance(from, msg.sender, amount);
        _burn(from, amount);
    }
    
    /**
     * @notice Create a snapshot
     * @return The snapshot ID
     */
    function snapshot() external onlySnapshotManager whenNotPaused returns (uint256) {
        uint256 id = _snapshot();
        emit SnapshotCreated(id);
        return id;
    }
    
    /**
     * @notice Update treasury address
     * @param newTreasury The new treasury address
     */
    function setTreasury(address newTreasury) external onlyRole(DEFAULT_ADMIN_ROLE) {
        require(newTreasury != address(0), "Invalid address");
        treasury = newTreasury;
        emit TreasuryUpdated(newTreasury);
    }
    
    /**
     * @notice Update emission rate
     * @param newRate The new emission rate
     */
    function setEmissionRate(uint256 newRate) external onlyRole(DEFAULT_ADMIN_ROLE) {
        emissionRate = newRate;
        lastUpdateTime = block.timestamp;
        emit EmissionRateUpdated(newRate);
    }
    
    /**
     * @notice Batch transfer
     * @param recipients The addresses to transfer to
     * @param amounts The amounts to transfer
     */
    function batchTransfer(address[] calldata recipients, uint256[] calldata amounts) external {
        require(recipients.length == amounts.length, "Length mismatch");
        require(recipients.length > 0, "Empty array");
        
        for (uint256 i = 0; i < recipients.length; i++) {
            _transfer(msg.sender, recipients[i], amounts[i]);
        }
    }
    
    /**
     * @notice Emergency pause
     */
    function pause() external onlyRole(DEFAULT_ADMIN_ROLE) {
        _pause();
    }
    
    /**
     * @notice Unpause
     */
    function unpause() external onlyRole(DEFAULT_ADMIN_ROLE) {
        _unpause();
    }
    
    function _delegate(address delegator, address delegatee) internal {
        address currentDelegate = delegates[delegator];
        delegates[delegator] = delegatee;
        
        emit DelegateChanged(delegator, currentDelegate, delegatee);
        
        // Update voting power
        uint256 amount = balanceOf(delegator);
        _moveDelegates(currentDelegate, delegatee, amount);
    }
    
    function _moveDelegates(address from, address to, uint256 amount) internal {
        if (from == to) {
            return;
        }
        
        // Reduce votes from delegator
        if (from != address(0)) {
            uint256 fromCount = checkpointCounts[from];
            uint256 fromVotes = fromCount == 0 ? 0 : checkpoints[from][fromCount].votes;
            
            if (amount > fromVotes) {
                // Should not happen in normal circumstances
                fromVotes = 0;
            } else {
                fromVotes -= amount;
            }
            
            if (fromCount == 0 || checkpoints[from][fromCount].fromBlock != block.number) {
                checkpoints[from][fromCount + 1] = Checkpoint(block.number, fromVotes);
                checkpointCounts[from] = fromCount + 1;
            } else {
                checkpoints[from][fromCount] = Checkpoint(block.number, fromVotes);
            }
            
            emit DelegateVotesChanged(from, fromVotes + amount, fromVotes);
        }
        
        // Add votes to delegatee
        if (to != address(0)) {
            uint256 toCount = checkpointCounts[to];
            uint256 toVotes = toCount == 0 ? 0 : checkpoints[to][toCount].votes;
            toVotes += amount;
            
            if (toCount == 0 || checkpoints[to][toCount].fromBlock != block.number) {
                checkpoints[to][toCount + 1] = Checkpoint(block.number, toVotes);
                checkpointCounts[to] = toCount + 1;
            } else {
                checkpoints[to][toCount] = Checkpoint(block.number, toVotes);
            }
            
            emit DelegateVotesChanged(to, toVotes - amount, toVotes);
        }
    }
    
    function _beforeTokenTransfer(
        address from,
        address to,
        uint256 amount
    ) internal override(ERC20, ERC20Snapshot) whenNotPaused {
        super._beforeTokenTransfer(from, to, amount);
        
        // Update delegates when tokens are transferred
        if (from != address(0) && from != to) {
            _moveDelegates(delegates[from], delegates[to], amount);
        }
    }
    
    // EIP-712 support
    function _domainSeparator() internal view returns (bytes32) {
        if (block.chainid == chainId) {
            return _cachedDomainSeparator;
        } else {
            return _buildDomainSeparator();
        }
    }
    
    bytes32 private _cachedDomainSeparator = 0x9c2355f2171b98214cb1f69e1dc71b0e5c3a4f1b2e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1e1;
    
    function _buildDomainSeparator() internal view returns (bytes32) {
        return keccak256(
            abi.encode(
                DOMAIN_TYPEHASH,
                keccak256(bytes(name())),
                chainId,
                address(this)
            )
        );
    }
    
    function _ecrecover(bytes32 hash, uint8 v, bytes32 r, bytes32 s) internal pure returns (address) {
        // EIP-2 signature
        bytes32 prefix = "\x19Ethereum Signed Message:\n32";
        bytes32 prefixedHash = keccak256(abi.encodePacked(prefix, hash));
        
        return ecrecover(prefixedHash, v, r, s);
    }
}