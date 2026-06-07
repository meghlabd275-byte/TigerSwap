/**
 * TigerSwap User Features - Staking Module
 * 
 * Native staking implementation for token staking and rewards.
 * Zero external dependencies - fully native implementation.
 * 
 * Features:
 * - Token staking
 * - Reward distribution
 * - Lock periods
 * - Governance power
 * 
 * @author TigerSwap
 * @version 1.0.0
 */

import { EVMClient, EVMWallet } from '@tigerswap/evm-sdk';

// ============================================================================
// Types & Interfaces
// ============================================================================

export interface StakingPool {
  id: string;
  stakingToken: string;
  rewardToken: string;
  totalStaked: bigint;
  rewardRate: bigint;
  periodFinish: number;
  rewardPerTokenStored: bigint;
  lastUpdateTime: number;
  lockPeriod: number;
  minStake: bigint;
  maxStake: bigint;
}

export interface Stake {
  user: string;
  amount: bigint;
  rewardDebt: bigint;
  startTime: number;
  lockEndTime: number;
  claimed: bigint;
}

export interface RewardInfo {
  pending: bigint;
  earned: bigint;
  rewardRate: bigint;
  finishTime: number;
}

export interface StakingConfig {
  chainId: number;
  stakingContract: string;
  rewardToken: string;
  rewardRate: bigint;
  lockPeriod: number;
  minStake: bigint;
  maxStake: bigint;
}

// ============================================================================
// Staking Contract
// ============================================================================

export class StakingContract {
  private config: StakingConfig;
  private wallet: EVMWallet;
  private client: EVMClient;
  private pools: Map<string, StakingPool>;
  private stakes: Map<string, Stake>;

  constructor(config: StakingConfig, wallet: EVMWallet) {
    this.config = config;
    this.wallet = wallet;
    this.client = new EVMClient(config.chainId);
    this.pools = new Map();
    this.stakes = new Map();
  }

  /**
   * Create staking pool
   */
  async createPool(
    stakingToken: string,
    rewardToken: string,
    rewardRate: bigint,
    lockPeriod: number,
    minStake: bigint,
    maxStake: bigint
  ): Promise<string> {
    const pool: StakingPool = {
      id: this.generateId(),
      stakingToken,
      rewardToken,
      totalStaked: 0n,
      rewardRate,
      periodFinish: Date.now() + 365 * 24 * 60 * 60 * 1000,
      rewardPerTokenStored: 0n,
      lastUpdateTime: Date.now(),
      lockPeriod,
      minStake,
      maxStake,
    };

    this.pools.set(pool.id, pool);

    // In production, deploy actual contract
    const data = this.encodeCreatePool(pool);
    const tx = await this.wallet.sendTransaction({
      to: this.config.stakingContract,
      value: 0n,
      data,
      gasLimit: 500000n,
    });

    return tx.hash;
  }

  /**
   * Stake tokens
   */
  async stake(poolId: string, amount: bigint): Promise<string> {
    const pool = this.pools.get(poolId);
    if (!pool) throw new Error('Pool not found');

    if (amount < pool.minStake) {
      throw new Error('Amount below minimum stake');
    }

    if (pool.maxStake > 0n && amount > pool.maxStake) {
      throw new Error('Amount exceeds maximum stake');
    }

    const stake: Stake = {
      user: this.wallet.getAddress(),
      amount,
      rewardDebt: 0n,
      startTime: Date.now(),
      lockEndTime: Date.now() + pool.lockPeriod,
      claimed: 0n,
    };

    const key = `${poolId}:${this.wallet.getAddress()}`;
    this.stakes.set(key, stake);

    pool.totalStaked += amount;

    const data = this.encodeStake(poolId, amount);
    const tx = await this.wallet.sendTransaction({
      to: this.config.stakingContract,
      value: amount,
      data,
      gasLimit: 200000n,
    });

    return tx.hash;
  }

  /**
   * Unstake tokens
   */
  async unstake(poolId: string): Promise<string> {
    const key = `${poolId}:${this.wallet.getAddress()}`;
    const stake = this.stakes.get(key);
    if (!stake) throw new Error('No stake found');

    if (Date.now() < stake.lockEndTime) {
      throw new Error('Lock period not finished');
    }

    const pool = this.pools.get(poolId);
    if (!pool) throw new Error('Pool not found');

    pool.totalStaked -= stake.amount;

    const data = this.encodeUnstake(poolId, stake.amount);
    const tx = await this.wallet.sendTransaction({
      to: this.config.stakingContract,
      value: 0n,
      data,
      gasLimit: 200000n,
    });

    this.stakes.delete(key);

    return tx.hash;
  }

  /**
   * Claim rewards
   */
  async claimReward(poolId: string): Promise<string> {
    const info = await this.getRewardInfo(poolId);
    
    if (info.pending <= 0n) {
      throw new Error('No pending rewards');
    }

    const data = this.encodeClaimReward(poolId);
    const tx = await this.wallet.sendTransaction({
      to: this.config.stakingContract,
      value: 0n,
      data,
      gasLimit: 150000n,
    });

    return tx.hash;
  }

  /**
   * Get reward info
   */
  async getRewardInfo(poolId: string): Promise<RewardInfo> {
    const pool = this.pools.get(poolId);
    if (!pool) throw new Error('Pool not found');

    const key = `${poolId}:${this.wallet.getAddress()}`;
    const stake = this.stakes.get(key);

    if (!stake) {
      return {
        pending: 0n,
        earned: 0n,
        rewardRate: pool.rewardRate,
        finishTime: pool.periodFinish,
      };
    }

    // Calculate pending rewards
    const timePassed = Date.now() - pool.lastUpdateTime;
    const rewardAmount = (pool.rewardRate * BigInt(timePassed)) / 1000n;
    const pending = stake.amount * rewardAmount / pool.totalStaked;

    return {
      pending,
      earned: stake.claimed,
      rewardRate: pool.rewardRate,
      finishTime: pool.periodFinish,
    };
  }

  /**
   * Get stake info
   */
  getStakeInfo(poolId: string): Stake | null {
    const key = `${poolId}:${this.wallet.getAddress()}`;
    return this.stakes.get(key) || null;
  }

  /**
   * Get pool info
   */
  getPoolInfo(poolId: string): StakingPool | null {
    return this.pools.get(poolId) || null;
  }

  /**
   * Get all pools
   */
  getAllPools(): StakingPool[] {
    return Array.from(this.pools.values());
  }

  /**
   * Calculate APY
   */
  calculateAPY(poolId: string): number {
    const pool = this.pools.get(poolId);
    if (!pool || pool.totalStaked === 0n) return 0;

    const annualReward = pool.rewardRate * 365n * 24n * 60n * 60n * 1000n;
    const rewardValue = Number(annualReward);
    const stakedValue = Number(pool.totalStaked);

    return (rewardValue / stakedValue) * 100;
  }

  private generateId(): string {
    return `pool_${Date.now()}_${Math.random().toString(36).slice(2, 9)}`;
  }

  private encodeCreatePool(pool: StakingPool): string {
    const iface = new Interface([
      'function createPool(address stakingToken, address rewardToken, uint256 rewardRate, uint256 lockPeriod, uint256 minStake, uint256 maxStake)',
    ]);
    return iface.encodeFunctionData('createPool', [
      pool.stakingToken,
      pool.rewardToken,
      pool.rewardRate,
      pool.lockPeriod,
      pool.minStake,
      pool.maxStake,
    ]);
  }

  private encodeStake(poolId: string, amount: bigint): string {
    const iface = new Interface([
      'function stake(uint256 amount)',
    ]);
    return iface.encodeFunctionData('stake', [amount]);
  }

  private encodeUnstake(poolId: string, amount: bigint): string {
    const iface = new Interface([
      'function unstake(uint256 amount)',
    ]);
    return iface.encodeFunctionData('unstake', [amount]);
  }

  private encodeClaimReward(poolId: string): string {
    const iface = new Interface([
      'function getReward()',
    ]);
    return iface.encodeFunctionData('getReward', []);
  }
}

// ============================================================================
// Vesting Contract
// ============================================================================

export class VestingContract {
  private wallet: EVMWallet;
  private vestings: Map<string, {
    recipient: string;
    totalAmount: bigint;
    startTime: number;
    cliff: number;
    duration: number;
    released: bigint;
  }>;

  constructor(wallet: EVMWallet) {
    this.wallet = wallet;
    this.vestings = new Map();
  }

  /**
   * Create vesting schedule
   */
  async createVesting(
    recipient: string,
    totalAmount: bigint,
    startTime: number,
    cliff: number,
    duration: number
  ): Promise<string> {
    const id = this.generateId();
    
    this.vestings.set(id, {
      recipient,
      totalAmount,
      startTime,
      cliff,
      duration,
      released: 0n,
    });

    const data = this.encodeCreateVesting(recipient, totalAmount, startTime, cliff, duration);
    const tx = await this.wallet.sendTransaction({
      to: recipient,
      value: totalAmount,
      data,
      gasLimit: 200000n,
    });

    return tx.hash;
  }

  /**
   * Release vested tokens
   */
  async release(vestingId: string): Promise<string> {
    const vesting = this.vestings.get(vestingId);
    if (!vesting) throw new Error('Vesting not found');

    const releasable = this.calculateReleasable(vestingId);
    if (releasable <= 0n) throw new Error('No tokens to release');

    vesting.released += releasable;

    const data = this.encodeRelease(vestingId, releasable);
    const tx = await this.wallet.sendTransaction({
      to: vesting.recipient,
      value: 0n,
      data,
      gasLimit: 100000n,
    });

    return tx.hash;
  }

  /**
   * Calculate releasable amount
   */
  calculateReleasable(vestingId: string): bigint {
    const vesting = this.vestings.get(vestingId);
    if (!vesting) return 0n;

    const now = Date.now();
    
    if (now < vesting.startTime + vesting.cliff) {
      return 0n;
    }

    const vestedEnd = vesting.startTime + vesting.duration;
    const vestedTime = Math.min(now, vestedEnd);
    const vestedDuration = vestedTime - vesting.startTime;
    
    const vestedAmount = (vesting.totalAmount * BigInt(vestedDuration)) / BigInt(vesting.duration);
    return vestedAmount - vesting.released;
  }

  /**
   * Get vesting info
   */
  getVestingInfo(vestingId: string): typeof this.vestings extends Map<string, infer V> ? V : never | null {
    return this.vestings.get(vestingId) || null;
  }

  private generateId(): string {
    return `vest_${Date.now()}_${Math.random().toString(36).slice(2, 9)}`;
  }

  private encodeCreateVesting(
    recipient: string,
    totalAmount: bigint,
    startTime: number,
    cliff: number,
    duration: number
  ): string {
    const iface = new Interface([
      'function createVesting(address recipient, uint256 amount, uint256 startTime, uint256 cliff, uint256 duration)',
    ]);
    return iface.encodeFunctionData('createVesting', [recipient, totalAmount, startTime, cliff, duration]);
  }

  private encodeRelease(vestingId: string, amount: bigint): string {
    const iface = new Interface([
      'function release(uint256 amount)',
    ]);
    return iface.encodeFunctionData('release', [amount]);
  }
}

// ============================================================================
// Export
// ============================================================================

export default {
  StakingContract,
  VestingContract,
};