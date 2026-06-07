// SPDX-License-Identifier: MIT
pragma solidity ^0.8.19;

/**
 * TigerSwap Governance Token
 * 
 * @author TigerSwap
 * @version 1.0.0
 */

import "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import "@openzeppelin/contracts/token/ERC20/extensions/ERC20Votes.sol";
import "@openzeppelin/contracts/access/Ownable.sol";

/**
 * @title TigerToken
 * @dev Governance token for TigerSwap DAO
 */
contract TigerToken is ERC20, ERC20Votes, Ownable {
    
    uint256 public constant MAX_SUPPLY = 1000000000 * 1e18; // 1 billion tokens
    
    mapping(address => bool) public minters;
    
    event MinterAdded(address indexed minter);
    event MinterRemoved(address indexed minter);
    
    /**
     * @dev Constructor
     */
    constructor() ERC20("TigerSwap", "TIGER") Ownable() {
        _mint(msg.sender, 100000000 * 1e18); // Initial mint
    }
    
    /**
     * @dev Mint tokens
     */
    function mint(address to, uint256 amount) external onlyMinter {
        require(totalSupply() + amount <= MAX_SUPPLY, "Max supply exceeded");
        _mint(to, amount);
    }
    
    /**
     * @dev Burn tokens
     */
    function burn(uint256 amount) external {
        _burn(msg.sender, amount);
    }
    
    /**
     * @dev Add minter
     */
    function addMinter(address minter) external onlyOwner {
        minters[minter] = true;
        emit MinterAdded(minter);
    }
    
    /**
     * @dev Remove minter
     */
    function removeMinter(address minter) external onlyOwner {
        minters[minter] = false;
        emit MinterRemoved(minter);
    }
    
    /**
     * @dev Check if address is minter
     */
    modifier onlyMinter() {
        require(minters[msg.sender], "Not a minter");
        _;
    }
    
    // The following functions are overrides required by Solidity.
    
    function _afterTokenTransfer(
        address from,
        address to,
        uint256 amount
    ) internal override(ERC20, ERC20Votes) {
        super._afterTokenTransfer(from, to, amount);
    }
    
    function _mint(address to, uint256 amount) internal override(ERC20, ERC20Votes) {
        super._mint(to, amount);
    }
    
    function _burn(address account, uint256 amount) internal override(ERC20, ERC20Votes) {
        super._burn(account, amount);
    }
}

/**
 * @title Governor
 * @dev Governance contract for TigerSwap DAO
 */
contract TigerGovernor {
    
    struct Proposal {
        uint256 id;
        address proposer;
        string description;
        uint256 startBlock;
        uint256 endBlock;
        uint256 forVotes;
        uint256 againstVotes;
        bool executed;
        bool cancelled;
    }
    
    uint256 public proposalCount;
    uint256 public quorumVotes = 50000000e18; // 50 million tokens
    uint256 public votingDelay = 1 days;
    uint256 public votingPeriod = 3 days;
    
    mapping(uint256 => Proposal) public proposals;
    mapping(uint256 => mapping(address => bool)) public hasVoted;
    mapping(uint256 => mapping(address => uint256)) public voteAmount;
    
    event ProposalCreated(uint256 indexed id, address indexed proposer, string description);
    event VoteCast(address indexed voter, uint256 indexed proposalId, bool support, uint256 weight);
    event ProposalExecuted(uint256 indexed id);
    event ProposalCancelled(uint256 indexed id);
    
    /**
     * @dev Create proposal
     */
    function propose(address[] memory targets, uint256[] memory values, bytes[] memory calldatas, string memory description) 
        external returns (uint256) {
        require(targets.length == values.length, "Invalid proposal");
        require(targets.length > 0, "Empty proposal");
        
        proposalCount++;
        uint256 id = proposalCount;
        
        Proposal storage proposal = proposals[id];
        proposal.id = id;
        proposal.proposer = msg.sender;
        proposal.description = description;
        proposal.startBlock = block.timestamp + votingDelay;
        proposal.endBlock = block.timestamp + votingDelay + votingPeriod;
        
        emit ProposalCreated(id, msg.sender, description);
        
        return id;
    }
    
    /**
     * @dev Cast vote
     */
    function castVote(uint256 proposalId, bool support) external {
        Proposal storage proposal = proposals[proposalId];
        require(proposal.startBlock <= block.timestamp, "Voting not started");
        require(proposal.endBlock >= block.timestamp, "Voting ended");
        require(!hasVoted[proposalId][msg.sender], "Already voted");
        
        // Simplified - would normally use token balance
        uint256 weight = 1e18; 
        
        hasVoted[proposalId][msg.sender] = true;
        voteAmount[proposalId][msg.sender] = weight;
        
        if (support) {
            proposal.forVotes += weight;
        } else {
            proposal.againstVotes += weight;
        }
        
        emit VoteCast(msg.sender, proposalId, support, weight);
    }
    
    /**
     * @dev Execute proposal
     */
    function execute(uint256 proposalId) external {
        Proposal storage proposal = proposals[proposalId];
        require(proposal.endBlock < block.timestamp, "Voting not ended");
        require(proposal.forVotes > proposal.againstVotes, "Proposal failed");
        require(proposal.forVotes >= quorumVotes, "Quorum not reached");
        require(!proposal.executed, "Already executed");
        
        proposal.executed = true;
        
        emit ProposalExecuted(proposalId);
    }
    
    /**
     * @dev Cancel proposal
     */
    function cancel(uint256 proposalId) external {
        Proposal storage proposal = proposals[proposalId];
        require(proposal.proposer == msg.sender, "Not proposer");
        require(!proposal.executed, "Already executed");
        
        proposal.cancelled = true;
        
        emit ProposalCancelled(proposalId);
    }
    
    /**
     * @dev Get proposal state
     */
    function state(uint256 proposalId) external view returns (string memory) {
        Proposal storage proposal = proposals[proposalId];
        
        if (proposal.cancelled) return "Canceled";
        if (proposal.executed) return "Executed";
        if (proposal.endBlock >= block.timestamp) return "Active";
        if (proposal.forVotes > proposal.againstVotes && proposal.forVotes >= quorumVotes) return "Succeeded";
        return "Defeated";
    }
}

/**
 * @title Timelock
 * @dev Timelock controller for governance
 */
contract TigerTimelock {
    
    uint256 public constant MIN_DELAY = 2 days;
    uint256 public constant MAX_DELAY = 30 days;
    
    mapping(bytes32 => uint256) public queuedTransactions;
    
    event QueueTransaction(bytes32 indexed txHash, address indexed target, uint256 value, bytes data, uint256 eta);
    event ExecuteTransaction(bytes32 indexed txHash, address indexed target, uint256 value, bytes data, uint256 eta);
    event CancelTransaction(bytes32 indexed txHash, address indexed target, uint256 value, bytes data, uint256 eta);
    
    /**
     * @dev Queue transaction
     */
    function queueTransaction(
        address target,
        uint256 value,
        bytes calldata data,
        uint256 eta
    ) external returns (bytes32) {
        require(eta >= block.timestamp + MIN_DELAY, "Delay too short");
        require(eta <= block.timestamp + MAX_DELAY, "Delay too long");
        
        bytes32 txHash = keccak256(abi.encode(target, value, data, eta));
        require(!queuedTransactions[txHash], "Already queued");
        
        queuedTransactions[txHash] = eta;
        
        emit QueueTransaction(txHash, target, value, data, eta);
        
        return txHash;
    }
    
    /**
     * @dev Execute transaction
     */
    function executeTransaction(
        address target,
        uint256 value,
        bytes calldata data,
        uint256 eta
    ) external payable returns (bytes32) {
        bytes32 txHash = keccak256(abi.encode(target, value, data, eta));
        require(queuedTransactions[txHash] != 0, "Not queued");
        require(block.timestamp >= eta, "Too early");
        
        queuedTransactions[txHash] = 0;
        
        (bool success, ) = target.call{value: value}(data);
        require(success, "Execution failed");
        
        emit ExecuteTransaction(txHash, target, value, data, eta);
        
        return txHash;
    }
    
    /**
     * @dev Cancel transaction
     */
    function cancelTransaction(
        address target,
        uint256 value,
        bytes calldata data,
        uint256 eta
    ) external {
        bytes32 txHash = keccak256(abi.encode(target, value, data, eta));
        require(queuedTransactions[txHash] != 0, "Not queued");
        
        queuedTransactions[txHash] = 0;
        
        emit CancelTransaction(txHash, target, value, data, eta);
    }
}