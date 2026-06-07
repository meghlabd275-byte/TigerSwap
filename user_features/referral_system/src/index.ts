/**
 * TigerSwap User Features - Referral System Module
 * 
 * Native referral and rewards system.
 * Zero external dependencies - fully native implementation.
 * 
 * @author TigerSwap
 * @version 1.0.0
 */

export interface ReferralCode {
  code: string;
  referrer: string;
  createdAt: number;
  usedCount: number;
}

export interface ReferralTiers {
  tier: number;
  discount: number;
  rebate: number;
  volumeRequired: bigint;
}

export interface ReferralReward {
  id: string;
  referee: string;
  referrer: string;
  amount: bigint;
  type: 'discount' | 'rebate';
  timestamp: number;
}

export class ReferralSystem {
  private referralCodes: Map<string, ReferralCode>;
  private referrals: Map<string, string>; // referee -> referrer
  private rewards: ReferralReward[];
  private tiers: ReferralTiers[];

  constructor() {
    this.referralCodes = new Map();
    this.referrals = new Map();
    this.rewards = [];
    this.tiers = this.initializeTiers();
  }

  /**
   * Initialize tiers
   */
  private initializeTiers(): ReferralTiers[] {
    return [
      { tier: 1, discount: 5, rebate: 5, volumeRequired: 0n },
      { tier: 2, discount: 10, rebate: 10, volumeRequired: 1000000n },
      { tier: 3, discount: 15, rebate: 15, volumeRequired: 10000000n },
      { tier: 4, discount: 20, rebate: 20, volumeRequired: 100000000n },
      { tier: 5, discount: 25, rebate: 25, volumeRequired: 1000000000n },
    ];
  }

  /**
   * Generate referral code
   */
  generateCode(referrer: string): string {
    const code = this.generateRandomCode();
    const referralCode: ReferralCode = {
      code,
      referrer,
      createdAt: Date.now(),
      usedCount: 0,
    };
    this.referralCodes.set(code, referralCode);
    return code;
  }

  /**
   * Use referral code
   */
  useCode(referee: string, code: string): boolean {
    const referralCode = this.referralCodes.get(code);
    if (!referralCode) return false;
    if (this.referrals.has(referee)) return false;

    // Record referral
    this.referrals.set(referee, referralCode.referrer);
    referralCode.usedCount++;

    return true;
  }

  /**
   * Calculate reward
   */
  calculateReward(referrer: string, volume: bigint): bigint {
    const tier = this.getTier(volume);
    const tierInfo = this.tiers[tier];
    
    // Calculate rebate based on tier
    return (volume * BigInt(tierInfo.rebate)) / 1000n;
  }

  /**
   * Distribute rewards
   */
  distributeReward(referee: string, amount: bigint): void {
    const referrer = this.referrals.get(referee);
    if (!referrer) return;

    const reward: ReferralReward = {
      id: `reward_${Date.now()}`,
      referee,
      referrer,
      amount,
      type: 'rebate',
      timestamp: Date.now(),
    };

    this.rewards.push(reward);
  }

  /**
   * Get tier
   */
  private getTier(volume: bigint): number {
    for (let i = this.tiers.length - 1; i >= 0; i--) {
      if (volume >= this.tiers[i].volumeRequired) {
        return i;
      }
    }
    return 0;
  }

  /**
   * Get referrer
   */
  getReferrer(referee: string): string | null {
    return this.referrals.get(referee) || null;
  }

  /**
   * Get referral stats
   */
  getStats(referrer: string): { totalReferrals: number; totalRewards: bigint } {
    const refereeList = Array.from(this.referrals.entries())
      .filter(([_, r]) => r === referrer);
    
    const totalRewards = this.rewards
      .filter(r => r.referrer === referrer)
      .reduce((sum, r) => sum + r.amount, 0n);

    return {
      totalReferrals: refereeList.length,
      totalRewards,
    };
  }

  private generateRandomCode(): string {
    const chars = 'ABCDEFGHJKLMNPQRSTUVWXYZ23456789';
    let code = '';
    for (let i = 0; i < 8; i++) {
      code += chars[Math.floor(Math.random() * chars.length)];
    }
    return code;
  }
}

export default ReferralSystem;