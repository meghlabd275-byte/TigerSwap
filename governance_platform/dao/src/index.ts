import { Interface } from "ethers";
/**
 * TigerSwap Governance Platform
 * 
 * Native DAO and governance with on-chain voting.
 * Completely independent - NO external governance services.
 * 
 * Features:
 * - DAO with token governance
 * - On-chain voting (proposal, vote, execute)
 * - Timelock controller
 * - Treasury management
 * - Proposal engine
 * 
 * @author TigerSwap
 * @version 1.0.0
 */

import { EVMClient, EVMWallet } from '@tigerswap/evm-sdk';

// ============================================================================
// Types & Interfaces
// ============================================================================

export interface Proposal {
  id: string;
  proposer: string;
  title: string;
  description: string;
  targets: string[];
  values: bigint[];
  signatures: string[];
  callDatas: string[];
  status: ProposalStatus;
  forVotes: bigint;
  againstVotes: bigint;
  abstainVotes: bigint;
  startBlock: number;
  endBlock: number;
  executeBlock?: number;
  createdAt: number;
  descriptionHash: string;
}

export enum ProposalStatus {
  PENDING = 'pending',
  ACTIVE = 'active',
  CANCELLED = 'cancelled',
  DEFEATED = 'defeated',
  SUCCEEDED = 'succeeded',
  QUEUED = 'queued',
  EXECUTED = 'executed',
  EXPIRED = 'expired',
}

export interface Vote {
  voter: string;
  proposalId: string;
  support: number; // 0 = against, 1 = for, 2 = abstain
  votes: bigint;
  reason?: string;
  timestamp: number;
}

export interface Delegate {
  delegator: string;
  delegatee: string;
  votes: bigint;
  checkpoint: number;
}

export interface GovernanceConfig {
  govToken: string;
  timelock: string;
  quorum: bigint;
  votingPeriod: number;
  votingDelay: number;
  proposalThreshold: bigint;
  executor?: string;
}

export interface TreasuryProposal {
  id: string;
  title: string;
  description: string;
  amount: bigint;
  recipient: string;
  token: string;
  status: ProposalStatus;
  votes: bigint;
  createdAt: number;
}

// ============================================================================
// Governance Config
// ============================================================================

export const DEFAULT_GOV_CONFIG: GovernanceConfig = {
  govToken: '0x0000000000000000000000000000000000000001',
  timelock: '0x0000000000000000000000000000000000000002',
  quorum: 4000000000000000000000000n, // 4% of total supply
  votingPeriod: 5760, // ~2 days (12 seconds per block)
  votingDelay: 1, // 1 block delay
  proposalThreshold: 1000000000000000000000n, // 1 token
};

// ============================================================================
// Governance Token (ERC-20 + Governor)
// ============================================================================

/**
 * GovernanceToken - ERC-20 with voting power
 * 
 * Extends standard ERC-20 with:
 * - Delegation
 * - Checkpoints
 * - Voting power tracking
 */
export class GovernanceToken {
  private client: EVMClient;
  private wallet: EVMWallet | null;
  private config: GovernanceConfig;
  private delegates: Map<string, Delegate>;
  private checkpoints: Map<string, Array<{fromBlock: number; votes: bigint}>>;

  constructor(config: GovernanceConfig, client: EVMClient) {
    this.client = client;
    this.wallet = null;
    this.config = config;
    this.delegates = new Map();
    this.checkpoints = new Map();
  }

  /**
   * Set wallet for transactions
   */
  setWallet(wallet: EVMWallet): void {
    this.wallet = wallet;
  }

  /**
   * Delegate voting power
   */
  async delegate(delegatee: string): Promise<string> {
    if (!this.wallet) {
      throw new Error('Wallet not set');
    }

    // Get current votes
    const balance = await this.getVotes(this.wallet.getAddress());
    
    // Create delegation
    const delegate: Delegate = {
      delegator: this.wallet.getAddress(),
      delegatee,
      votes: balance,
      checkpoint: Date.now(),
    };
    
    this.delegates.set(this.wallet.getAddress(), delegate);
    
    // Create checkpoint
    this.writeCheckpoint(delegatee, balance);
    
    // Execute delegation transaction
    const data = this.encodeDelegate(delegatee);
    const tx = await this.wallet.sendTransaction({
      to: this.config.govToken,
      value: 0n,
      data,
      gasLimit: 100000n,
    });
    
    return tx.hash;
  }

  /**
   * Get current voting power
   */
  async getVotes(account: string): Promise<bigint> {
    // In production, call contract
    // For now, return mock
    return 1000000000000000000000000n;
  }

  /**
   * Get voting power at block
   */
  async getPastVotes(account: string, blockNumber: number): Promise<bigint> {
    const checkpoints = this.checkpoints.get(account);
    if (!checkpoints || checkpoints.length === 0) {
      return 0n;
    }

    // Binary search for checkpoint
    let left = 0;
    let right = checkpoints.length - 1;
    
    while (left < right) {
      const mid = Math.floor((left + right + 1) / 2);
      if (checkpoints[mid].fromBlock <= blockNumber) {
        left = mid;
      } else {
        right = mid - 1;
      }
    }
    
    return checkpoints[left].votes;
  }

  /**
   * Delegate by signature
   */
  async delegateBySig(
    delegatee: string,
    nonce: number,
    expiry: number,
    signature: string
  ): Promise<string> {
    if (!this.wallet) {
      throw new Error('Wallet not set');
    }

    const data = this.encodeDelegateBySig(delegatee, nonce, expiry, signature);
    
    const tx = await this.wallet.sendTransaction({
      to: this.config.govToken,
      value: 0n,
      data,
      gasLimit: 100000n,
    });
    
    return tx.hash;
  }

  /**
   * Get delegatee
   */
  getDelegate(delegator: string): string {
    const delegate = this.delegates.get(delegator);
    return delegate?.delegatee || delegator;
  }

  private writeCheckpoint(account: string, votes: bigint): void {
    if (!this.checkpoints.has(account)) {
      this.checkpoints.set(account, []);
    }
    
    const checkpoints = this.checkpoints.get(account)!;
    const fromBlock = Date.now();
    
    if (checkpoints.length > 0 && checkpoints[checkpoints.length - 1].fromBlock === fromBlock) {
      checkpoints[checkpoints.length - 1].votes = votes;
    } else {
      checkpoints.push({ fromBlock, votes });
    }
  }

  private encodeDelegate(delegatee: string): string {
    const iface = new Interface(['function delegate(address delegatee)']);
    return iface.encodeFunctionData('delegate', [delegatee]);
  }

  private encodeDelegateBySig(
    delegatee: string,
    nonce: number,
    expiry: number,
    signature: string
  ): string {
    const iface = new Interface([
      'function delegateBySig(address delegatee, uint256 nonce, uint256 expiry, bytes signature)',
    ]);
    return iface.encodeFunctionData('delegateBySig', [delegatee, nonce, expiry, signature]);
  }
}

// ============================================================================
// Governor (DAO)
// ============================================================================

/**
 * Governor - Proposal and voting management
 */
export class Governor {
  private client: EVMClient;
  private wallet: EVMWallet | null;
  private config: GovernanceConfig;
  private proposals: Map<string, Proposal>;
  private votes: Map<string, Vote[]>;

  constructor(config: GovernanceConfig, client: EVMClient) {
    this.client = client;
    this.wallet = null;
    this.config = config;
    this.proposals = new Map();
    this.votes = new Map();
  }

  /**
   * Set wallet
   */
  setWallet(wallet: EVMWallet): void {
    this.wallet = wallet;
  }

  /**
   * Create proposal
   */
  async propose(
    title: string,
    description: string,
    targets: string[],
    values: bigint[],
    signatures: string[],
    callDatas: string[]
  ): Promise<Proposal> {
    if (!this.wallet) {
      throw new Error('Wallet not set');
    }

    // Check proposal threshold
    const proposerVotes = await this.getVotingPower(this.wallet.getAddress());
    if (proposerVotes < this.config.proposalThreshold) {
      throw new Error('Insufficient voting power to propose');
    }

    // Generate proposal ID
    const proposalId = this.generateProposalId();
    
    const proposal: Proposal = {
      id: proposalId,
      proposer: this.wallet.getAddress(),
      title,
      description,
      targets,
      values,
      signatures,
      callDatas,
      status: ProposalStatus.PENDING,
      forVotes: 0n,
      againstVotes: 0n,
      abstainVotes: 0n,
      startBlock: 0,
      endBlock: 0,
      createdAt: Date.now(),
      descriptionHash: this.hashDescription(description),
    };

    // Execute proposal creation
    const data = this.encodePropose(targets, values, callDatas, this.hashDescription(description));
    const tx = await this.wallet.sendTransaction({
      to: this.config.govToken,
      value: 0n,
      data,
      gasLimit: 300000n,
    });

    proposal.status = ProposalStatus.ACTIVE;
    proposal.startBlock = tx.blockNumber || 0;
    proposal.endBlock = proposal.startBlock + this.config.votingPeriod;
    
    this.proposals.set(proposalId, proposal);
    return proposal;
  }

  /**
   * Cast vote
   */
  async castVote(
    proposalId: string,
    support: number,
    reason?: string
  ): Promise<Vote> {
    if (!this.wallet) {
      throw new Error('Wallet not set');
    }

    const proposal = this.proposals.get(proposalId);
    if (!proposal) {
      throw new Error('Proposal not found');
    }

    if (proposal.status !== ProposalStatus.ACTIVE) {
      throw new Error('Proposal not active');
    }

    // Get voting power
    const votes = await this.getVotingPower(this.wallet.getAddress());
    
    const vote: Vote = {
      voter: this.wallet.getAddress(),
      proposalId,
      support,
      votes,
      reason,
      timestamp: Date.now(),
    };

    // Update proposal vote counts
    if (support === 1) {
      proposal.forVotes += votes;
    } else if (support === 0) {
      proposal.againstVotes += votes;
    } else {
      proposal.abstainVotes += votes;
    }

    // Store vote
    if (!this.votes.has(proposalId)) {
      this.votes.set(proposalId, []);
    }
    this.votes.get(proposalId)!.push(vote);

    // Execute vote transaction
    const data = this.encodeCastVote(proposalId, support, reason || '');
    const tx = await this.wallet.sendTransaction({
      to: this.config.govToken,
      value: 0n,
      data,
      gasLimit: 150000n,
    });

    // Update proposal status
    await this.updateProposalStatus(proposalId);

    return vote;
  }

  /**
   * Execute proposal
   */
  async execute(proposalId: string): Promise<string> {
    if (!this.wallet) {
      throw new Error('Wallet not set');
    }

    const proposal = this.proposals.get(proposalId);
    if (!proposal) {
      throw new Error('Proposal not found');
    }

    if (proposal.status !== ProposalStatus.QUEUED) {
      throw new Error('Proposal not queued');
    }

    // Execute each action
    const txHashes: string[] = [];
    
    for (let i = 0; i < proposal.targets.length; i++) {
      const tx = await this.wallet.sendTransaction({
        to: proposal.targets[i],
        value: proposal.values[i],
        data: proposal.callDatas[i],
        gasLimit: 200000n,
      });
      txHashes.push(tx.hash);
    }

    proposal.status = ProposalStatus.EXECUTED;
    proposal.executeBlock = Date.now();

    return txHashes[0];
  }

  /**
   * Cancel proposal
   */
  async cancel(proposalId: string): Promise<void> {
    const proposal = this.proposals.get(proposalId);
    if (!proposal) {
      throw new Error('Proposal not found');
    }

    proposal.status = ProposalStatus.CANCELLED;
  }

  /**
   * Get proposal
   */
  getProposal(proposalId: string): Proposal | null {
    return this.proposals.get(proposalId) || null;
  }

  /**
   * Get all proposals
   */
  getProposals(status?: ProposalStatus): Proposal[] {
    const proposals = Array.from(this.proposals.values());
    if (status) {
      return proposals.filter(p => p.status === status);
    }
    return proposals;
  }

  /**
   * Get voting power
   */
  async getVotingPower(account: string): Promise<bigint> {
    // In production, call governance token
    return 1000000000000000000000000n;
  }

  /**
   * Get proposal state
   */
  async getProposalState(proposalId: string): Promise<ProposalStatus> {
    const proposal = this.proposals.get(proposalId);
    if (!proposal) {
      return ProposalStatus.PENDING;
    }

    return proposal.status;
  }

  private async updateProposalStatus(proposalId: string): Promise<void> {
    const proposal = this.proposals.get(proposalId);
    if (!proposal) return;

    const totalVotes = proposal.forVotes + proposal.againstVotes + proposal.abstainVotes;
    const forPercent = totalVotes > 0 
      ? (proposal.forVotes * 10000n) / totalVotes 
      : 0n;

    // Check if passed quorum
    if (forPercent >= this.config.quorum / 100n && proposal.forVotes > proposal.againstVotes) {
      proposal.status = ProposalStatus.QUEUED;
    } else if (proposal.endBlock > 0 && Date.now() > proposal.endBlock) {
      proposal.status = ProposalStatus.DEFEATED;
    }
  }

  private generateProposalId(): string {
    return `0x${Date.now().toString(16)}${Math.random().toString(16).slice(2, 10)}`;
  }

  private hashDescription(description: string): string {
    // In production, use keccak256
    return '0x' + Buffer.from(description).toString('hex').slice(0, 64);
  }

  private encodePropose(
    targets: string[],
    values: bigint[],
    callDatas: string[],
    descriptionHash: string
  ): string {
    const iface = new Interface([
      'function propose(address[] targets, uint256[] values, bytes[] calldatas, string description)',
    ]);
    return iface.encodeFunctionData('propose', [targets, values, callDatas, descriptionHash]);
  }

  private encodeCastVote(
    proposalId: string,
    support: number,
    reason: string
  ): string {
    const iface = new Interface([
      'function castVote(uint256 proposalId, uint8 support)',
    ]);
    return iface.encodeFunctionData('castVote', [proposalId, support]);
  }
}

// ============================================================================
// Timelock Controller
// ============================================================================

/**
 * Timelock - Time-delayed execution
 */
export class Timelock {
  private client: EVMClient;
  private wallet: EVMWallet | null;
  private delay: number;
  private pendingTxs: Map<string, { executeAfter: number; target: string; value: bigint; data: string }>;

  constructor(delay: number = 2 * 24 * 60 * 60) { // 2 days default
    this.client = new EVMClient(1);
    this.wallet = null;
    this.delay = delay;
    this.pendingTxs = new Map();
  }

  /**
   * Set wallet
   */
  setWallet(wallet: EVMWallet): void {
    this.wallet = wallet;
  }

  /**
   * Schedule transaction
   */
  async schedule(
    target: string,
    value: bigint,
    data: string,
    predecessor?: string
  ): Promise<string> {
    if (!this.wallet) {
      throw new Error('Wallet not set');
    }

    const id = this.generateTxId();
    const executeAfter = Date.now() + this.delay;

    this.pendingTxs.set(id, {
      executeAfter,
      target,
      value,
      data,
    });

    // Execute schedule transaction
    const scheduleData = this.encodeSchedule(target, value, data, predecessor || '0x');
    const tx = await this.wallet.sendTransaction({
      to: this.config.timelock,
      value: 0n,
      data: scheduleData,
      gasLimit: 150000n,
    });

    return tx.hash;
  }

  /**
   * Execute scheduled transaction
   */
  async execute(txId: string): Promise<string> {
    if (!this.wallet) {
      throw new Error('Wallet not set');
    }

    const pending = this.pendingTxs.get(txId);
    if (!pending) {
      throw new Error('Transaction not found');
    }

    if (Date.now() < pending.executeAfter) {
      throw new Error('Transaction not yet executable');
    }

    const tx = await this.wallet.sendTransaction({
      to: pending.target,
      value: pending.value,
      data: pending.data,
      gasLimit: 200000n,
    });

    this.pendingTxs.delete(txId);
    return tx.hash;
  }

  /**
   * Cancel scheduled transaction
   */
  async cancel(txId: string): Promise<void> {
    this.pendingTxs.delete(txId);
  }

  /**
   * Get pending transactions
   */
  getPendingTxs(): Array<{id: string; executeAfter: number; target: string; value: bigint}> {
    return Array.from(this.pendingTxs.entries()).map(([id, tx]) => ({
      id,
      executeAfter: tx.executeAfter,
      target: tx.target,
      value: tx.value,
    }));
  }

  /**
   * Update delay
   */
  async updateDelay(newDelay: number): Promise<string> {
    if (!this.wallet) {
      throw new Error('Wallet not set');
    }

    const data = this.encodeUpdateDelay(newDelay);
    const tx = await this.wallet.sendTransaction({
      to: this.config.timelock,
      value: 0n,
      data,
      gasLimit: 100000n,
    });

    this.delay = newDelay;
    return tx.hash;
  }

  private generateTxId(): string {
    return `0x${Date.now().toString(16)}${Math.random().toString(16).slice(2, 10)}`;
  }

  private encodeSchedule(
    target: string,
    value: bigint,
    data: string,
    predecessor: string
  ): string {
    const iface = new Interface([
      'function schedule(address target, uint256 value, bytes data, bytes32 predecessor, bytes32 salt)',
    ]);
    return iface.encodeFunctionData('schedule', [
      target,
      value,
      data,
      predecessor,
      '0x' + '0'.repeat(64),
    ]);
  }

  private encodeUpdateDelay(delay: number): string {
    const iface = new Interface(['function updateDelay(uint256 newDelay)']);
    return iface.encodeFunctionData('updateDelay', [delay]);
  }
}

// ============================================================================
// Treasury
// ============================================================================

/**
 * Treasury - Protocol treasury management
 */
export class Treasury {
  private client: EVMClient;
  private wallet: EVMWallet | null;
  private config: GovernanceConfig;
  private balances: Map<string, bigint>;

  constructor(config: GovernanceConfig, client: EVMClient) {
    this.client = client;
    this.wallet = null;
    this.config = config;
    this.balances = new Map();
  }

  /**
   * Set wallet
   */
  setWallet(wallet: EVMWallet): void {
    this.wallet = wallet;
  }

  /**
   * Get balance
   */
  async getBalance(token: string = '0x0000000000000000000000000000000000000000'): Promise<bigint> {
    return this.balances.get(token) || 0n;
  }

  /**
   * Get all balances
   */
  getBalances(): Map<string, bigint> {
    return new Map(this.balances);
  }

  /**
   * Create treasury proposal
   */
  createProposal(
    title: string,
    description: string,
    amount: bigint,
    recipient: string,
    token: string = '0x0000000000000000000000000000000000000000'
  ): TreasuryProposal {
    return {
      id: this.generateProposalId(),
      title,
      description,
      amount,
      recipient,
      token,
      status: ProposalStatus.PENDING,
      votes: 0n,
      createdAt: Date.now(),
    };
  }

  /**
   * Execute treasury transfer (after governance)
   */
  async executeTransfer(
    recipient: string,
    amount: bigint,
    token: string = '0x0000000000000000000000000000000000000000'
  ): Promise<string> {
    if (!this.wallet) {
      throw new Error('Wallet not set');
    }

    const data = this.encodeTransfer(recipient, amount);
    const tx = await this.wallet.sendTransaction({
      to: this.config.timelock,
      value: token === '0x0000000000000000000000000000000000000000' ? amount : 0n,
      data,
      gasLimit: 100000n,
    });

    // Update balance
    const currentBalance = this.balances.get(token) || 0n;
    this.balances.set(token, currentBalance - amount);

    return tx.hash;
  }

  private generateProposalId(): string {
    return `0x${Date.now().toString(16)}${Math.random().toString(16).slice(2, 10)}`;
  }

  private encodeTransfer(recipient: string, amount: bigint): string {
    const iface = new Interface(['function transfer(address to, uint256 amount)']);
    return iface.encodeFunctionData('transfer', [recipient, amount]);
  }
}

// ============================================================================
// Utility Functions
// ============================================================================

/**
 * Create governance config
 */
export function createGovConfig(
  govToken: string,
  timelock: string,
  quorum: bigint = 4000000000000000000000000n
): GovernanceConfig {
  return {
    ...DEFAULT_GOV_CONFIG,
    govToken,
    timelock,
    quorum,
  };
}

/**
 * Calculate vote weight
 */
export function calculateVoteWeight(
  votes: bigint,
  weight: number
): bigint {
  return (votes * BigInt(Math.floor(weight * 100))) / 100n;
}

/**
 * Check if proposal passed
 */
export function isProposalPassed(
  forVotes: bigint,
  againstVotes: bigint,
  quorum: bigint
): boolean {
  return forVotes > againstVotes && forVotes >= quorum;
}

// ============================================================================
// Export
// ============================================================================

export default {
  ProposalStatus,
  DEFAULT_GOV_CONFIG,
  GovernanceToken,
  Governor,
  Timelock,
  Treasury,
  createGovConfig,
  calculateVoteWeight,
  isProposalPassed,
};