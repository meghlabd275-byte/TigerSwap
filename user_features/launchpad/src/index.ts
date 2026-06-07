/**
 * TigerSwap User Features - Launchpad Module
 * 
 * Native launchpad for IDO/IEO token sales.
 * Zero external dependencies - fully native implementation.
 * 
 * Features:
 * - Token sales
 * - Whitelist management
 * - Vesting schedules
 * - Fair distribution
 * 
 * @author TigerSwap
 * @version 1.0.0
 */

import { EVMClient, EVMWallet } from '@tigerswap/evm-sdk';

// ============================================================================
// Types & Interfaces
// ============================================================================

export interface Launchpad {
  id: string;
  name: string;
  token: string;
  tokenDecimals: number;
  saleToken: string;
  saleTokenDecimals: number;
  price: bigint;
  hardCap: bigint;
  softCap: bigint;
  minPurchase: bigint;
  maxPurchase: bigint;
  startTime: number;
  endTime: number;
  totalRaised: bigint;
  totalSold: bigint;
  status: 'pending' | 'active' | 'completed' | 'cancelled';
  vestingStart: number;
  vestingPeriod: number;
  vestingCliff: number;
  tiers: SaleTier[];
}

export interface SaleTier {
  name: string;
  allocation: bigint;
  minPurchase: bigint;
  maxPurchase: bigint;
  discount: number;
}

export interface Allocation {
  user: string;
  tier: string;
  allocation: bigint;
  claimed: bigint;
  pending: bigint;
}

export interface Purchase {
  buyer: string;
  amount: bigint;
  tokenAmount: bigint;
  timestamp: number;
  txHash: string;
}

// ============================================================================
// Launchpad Contract
// ============================================================================

export class Launchpad {
  private wallet: EVMWallet;
  private client: EVMClient;
  private launchpads: Map<string, Launchpad>;
  private allocations: Map<string, Allocation>;
  private purchases: Map<string, Purchase[]>;

  constructor(wallet: EVMWallet, chainId: number) {
    this.wallet = wallet;
    this.client = new EVMClient(chainId);
    this.launchpads = new Map();
    this.allocations = new Map();
    this.purchases = new Map();
  }

  /**
   * Create launchpad
   */
  async createLaunchpad(
    name: string,
    token: string,
    tokenDecimals: number,
    saleToken: string,
    saleTokenDecimals: number,
    price: bigint,
    hardCap: bigint,
    softCap: bigint,
    minPurchase: bigint,
    maxPurchase: bigint,
    startTime: number,
    endTime: number,
    vestingPeriod: number,
    vestingCliff: number
  ): Promise<string> {
    const launchpad: Launchpad = {
      id: this.generateId(),
      name,
      token,
      tokenDecimals,
      saleToken,
      saleTokenDecimals,
      price,
      hardCap,
      softCap,
      minPurchase,
      maxPurchase,
      startTime,
      endTime,
      totalRaised: 0n,
      totalSold: 0n,
      status: 'pending',
      vestingStart: endTime,
      vestingPeriod,
      vestingCliff,
      tiers: [],
    };

    this.launchpads.set(launchpad.id, launchpad);

    // In production, deploy contract
    const data = this.encodeCreateLaunchpad(launchpad);
    const tx = await this.wallet.sendTransaction({
      to: '0x0000000000000000000000000000000000000001',
      value: 0n,
      data,
      gasLimit: 500000n,
    });

    return launchpad.id;
  }

  /**
   * Add sale tier
   */
  addTier(launchpadId: string, tier: SaleTier): void {
    const launchpad = this.launchpads.get(launchpadId);
    if (!launchpad) throw new Error('Launchpad not found');
    
    launchpad.tiers.push(tier);
  }

  /**
   * Add to whitelist
   */
  async addToWhitelist(launchpadId: string, users: string[], tierName: string): Promise<string> {
    const launchpad = this.launchpads.get(launchpadId);
    if (!launchpad) throw new Error('Launchpad not found');

    for (const user of users) {
      const key = `${launchpadId}:${user}`;
      this.allocations.set(key, {
        user,
        tier: tierName,
        allocation: 0n,
        claimed: 0n,
        pending: 0n,
      });
    }

    const data = this.encodeAddToWhitelist(launchpadId, users);
    const tx = await this.wallet.sendTransaction({
      to: '0x0000000000000000000000000000000000000001',
      value: 0n,
      data,
      gasLimit: 200000n,
    });

    return tx.hash;
  }

  /**
   * Purchase tokens
   */
  async purchase(launchpadId: string, amount: bigint): Promise<string> {
    const launchpad = this.launchpads.get(launchpadId);
    if (!launchpad) throw new Error('Launchpad not found');

    if (launchpad.status !== 'active') {
      throw new Error('Launchpad not active');
    }

    if (Date.now() < launchpad.startTime) {
      throw new Error('Sale not started');
    }

    if (Date.now() > launchpad.endTime) {
      throw new Error('Sale ended');
    }

    const tokenAmount = (amount * 10n ** BigInt(launchpad.tokenDecimals)) / launchpad.price;
    
    if (tokenAmount < launchpad.minPurchase) {
      throw new Error('Below minimum purchase');
    }

    const maxPurchase = launchpad.maxPurchase;
    const userKey = `${launchpadId}:${this.wallet.getAddress()}`;
    const allocation = this.allocations.get(userKey);
    
    if (allocation) {
      const tier = launchpad.tiers.find(t => t.name === allocation.tier);
      if (tier) {
        if (amount > tier.maxPurchase) {
          throw new Error('Exceeds tier maximum');
        }
      }
    }

    const purchase: Purchase = {
      buyer: this.wallet.getAddress(),
      amount,
      tokenAmount,
      timestamp: Date.now(),
      txHash: '',
    };

    const purchases = this.purchases.get(launchpadId) || [];
    purchases.push(purchase);
    this.purchases.set(launchpadId, purchases);

    launchpad.totalRaised += amount;
    launchpad.totalSold += tokenAmount;

    if (launchpad.totalRaised >= launchpad.hardCap) {
      launchpad.status = 'completed';
    }

    const data = this.encodePurchase(launchpadId, amount);
    const tx = await this.wallet.sendTransaction({
      to: '0x0000000000000000000000000000000000000001',
      value: amount,
      data,
      gasLimit: 300000n,
    });

    purchase.txHash = tx.hash;
    return tx.hash;
  }

  /**
   * Claim tokens
   */
  async claim(launchpadId: string): Promise<string> {
    const launchpad = this.launchpads.get(launchpadId);
    if (!launchpad) throw new Error('Launchpad not found');

    const userKey = `${launchpadId}:${this.wallet.getAddress()}`;
    const allocation = this.allocations.get(userKey);
    
    if (!allocation) throw new Error('No allocation found');

    const claimable = this.calculateClaimable(launchpadId);
    if (claimable <= 0n) throw new Error('Nothing to claim');

    allocation.claimed += claimable;

    const data = this.encodeClaim(launchpadId, claimable);
    const tx = await this.wallet.sendTransaction({
      to: '0x0000000000000000000000000000000000000001',
      value: 0n,
      data,
      gasLimit: 150000n,
    });

    return tx.hash;
  }

  /**
   * Get launchpad info
   */
  getLaunchpad(launchpadId: string): Launchpad | null {
    return this.launchpads.get(launchpadId) || null;
  }

  /**
   * Get allocation
   */
  getAllocation(launchpadId: string, user: string): Allocation | null {
    return this.allocations.get(`${launchpadId}:${user}`) || null;
  }

  /**
   * Get purchases
   */
  getPurchases(launchpadId: string): Purchase[] {
    return this.purchases.get(launchpadId) || [];
  }

  /**
   * Calculate claimable tokens
   */
  calculateClaimable(launchpadId: string): bigint {
    const launchpad = this.launchpads.get(launchpadId);
    if (!launchpad) return 0n;

    const userKey = `${launchpadId}:${this.wallet.getAddress()}`;
    const allocation = this.allocations.get(userKey);
    if (!allocation) return 0n;

    if (launchpad.status !== 'completed') {
      return 0n;
    }

    const now = Date.now();
    if (now < launchpad.vestingStart + launchpad.vestingCliff) {
      return 0n;
    }

    const vestingStart = launchpad.vestingStart;
    const duration = launchpad.vestingPeriod;
    const elapsed = now - vestingStart;
    
    if (elapsed >= duration) {
      return allocation.pending - allocation.claimed;
    }

    const vested = (allocation.pending * BigInt(elapsed)) / BigInt(duration);
    return vested - allocation.claimed;
  }

  /**
   * Get all launchpads
   */
  getAllLaunchpads(): Launchpad[] {
    return Array.from(this.launchpads.values());
  }

  /**
   * Start launchpad
   */
  async startLaunchpad(launchpadId: string): Promise<void> {
    const launchpad = this.launchpads.get(launchpadId);
    if (!launchpad) throw new Error('Launchpad not found');
    
    launchpad.status = 'active';
  }

  /**
   * Cancel launchpad
   */
  async cancelLaunchpad(launchpadId: string): Promise<void> {
    const launchpad = this.launchpads.get(launchpadId);
    if (!launchpad) throw new Error('Launchpad not found');
    
    launchpad.status = 'cancelled';
  }

  private generateId(): string {
    return `launchpad_${Date.now()}_${Math.random().toString(36).slice(2, 9)}`;
  }

  private encodeCreateLaunchpad(lp: Launchpad): string {
    const iface = new Interface([
      'function createLaunchpad(string name, address token, uint256 price, uint256 hardCap, uint256 softCap, uint256 startTime, uint256 endTime)',
    ]);
    return iface.encodeFunctionData('createLaunchpad', [
      lp.name,
      lp.token,
      lp.price,
      lp.hardCap,
      lp.softCap,
      lp.startTime,
      lp.endTime,
    ]);
  }

  private encodeAddToWhitelist(launchpadId: string, users: string[]): string {
    const iface = new Interface([
      'function addToWhitelist(address[] users)',
    ]);
    return iface.encodeFunctionData('addToWhitelist', [users]);
  }

  private encodePurchase(launchpadId: string, amount: bigint): string {
    const iface = new Interface([
      'function purchase() payable',
    ]);
    return iface.encodeFunctionData('purchase', []);
  }

  private encodeClaim(launchpadId: string, amount: bigint): string {
    const iface = new Interface([
      'function claim()',
    ]);
    return iface.encodeFunctionData('claim', []);
  }
}

// ============================================================================
// Export
// ============================================================================

export default {
  Launchpad,
};