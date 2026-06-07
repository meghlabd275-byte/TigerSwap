/**
 * TigerSwap User Features - Copy Trading Module
 * 
 * Native copy trading implementation.
 * Zero external dependencies - fully native implementation.
 * 
 * @author TigerSwap
 * @version 1.0.0
 */

export interface TraderProfile {
  id: string;
  address: string;
  name: string;
  winRate: number;
  totalTrades: number;
  pnl: bigint;
  followers: number;
  copiers: number;
}

export interface CopyTrade {
  id: string;
  trader: string;
  follower: string;
  token: string;
  amount: bigint;
  price: bigint;
  side: 'buy' | 'sell';
  timestamp: number;
  pnl?: bigint;
}

export interface CopySettings {
  copyRatio: number; // 0.1 to 1.0
  maxPosition: bigint;
  stopLoss?: bigint;
  takeProfit?: bigint;
  autoCopy: boolean;
}

export class CopyTrading {
  private traders: Map<string, TraderProfile>;
  private follows: Map<string, { trader: string; settings: CopySettings }>;
  private trades: Map<string, CopyTrade[]>;

  constructor() {
    this.traders = new Map();
    this.follows = new Map();
    this.trades = new Map();
  }

  /**
   * Register as trader
   */
  registerTrader(address: string, name: string): TraderProfile {
    const profile: TraderProfile = {
      id: `trader_${Date.now()}`,
      address,
      name,
      winRate: 0,
      totalTrades: 0,
      pnl: 0n,
      followers: 0,
      copiers: 0,
    };
    this.traders.set(profile.id, profile);
    return profile;
  }

  /**
   * Follow trader
   */
  follow(follower: string, traderId: string, settings: CopySettings): void {
    const key = `${follower}_${traderId}`;
    this.follows.set(key, { trader: traderId, settings });
    
    // Update follower count
    const trader = this.traders.get(traderId);
    if (trader) {
      trader.followers++;
    }
  }

  /**
   * Unfollow trader
   */
  unfollow(follower: string, traderId: string): void {
    const key = `${follower}_${traderId}`;
    this.follows.delete(key);
  }

  /**
   * Copy trade
   */
  async copyTrade(traderId: string, follower: string, trade: Omit<CopyTrade, 'follower'>): Promise<CopyTrade> {
    const follow = this.follows.get(`${follower}_${traderId}`);
    if (!follow) throw new Error('Not following trader');

    const copiedTrade: CopyTrade = {
      ...trade,
      id: `copy_${Date.now()}`,
      follower,
    };

    // Calculate copy amount based on settings
    const copyAmount = trade.amount * BigInt(Math.floor(follow.settings.copyRatio * 100)) / 100n;

    // Execute copy trade (simplified)
    const trades = this.trades.get(follower) || [];
    trades.push(copiedTrade);
    this.trades.set(follower, trades);

    return copiedTrade;
  }

  /**
   * Get top traders
   */
  getTopTraders(limit: number = 10): TraderProfile[] {
    return Array.from(this.traders.values())
      .sort((a, b) => Number(b.pnl - a.pnl))
      .slice(0, limit);
  }

  /**
   * Get trader profile
   */
  getTrader(traderId: string): TraderProfile | null {
    return this.traders.get(traderId) || null;
  }

  /**
   * Get following
   */
  getFollowing(follower: string): string[] {
    const following: string[] = [];
    for (const [key, value] of this.follows) {
      if (key.startsWith(follower + '_')) {
        following.push(value.trader);
      }
    }
    return following;
  }

  /**
   * Get copy history
   */
  getHistory(follower: string): CopyTrade[] {
    return this.trades.get(follower) || [];
  }
}

export default CopyTrading;