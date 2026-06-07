/**
 * TigerSwap User Features - Portfolio Tracker Module
 * 
 * Native portfolio tracking and analytics.
 * Zero external dependencies - fully native implementation.
 * 
 * @author TigerSwap
 * @version 1.0.0
 */

export interface PortfolioPosition {
  token: string;
  balance: bigint;
  value: bigint;
  price: bigint;
  change24h: number;
}

export interface PortfolioSnapshot {
  id: string;
  user: string;
  totalValue: bigint;
  positions: PortfolioPosition[];
  timestamp: number;
}

export interface Transaction {
  id: string;
  type: 'swap' | 'transfer' | 'stake' | 'unstake' | 'bridge';
  tokenIn?: string;
  tokenOut?: string;
  amountIn?: bigint;
  amountOut?: bigint;
  value: bigint;
  hash: string;
  timestamp: number;
}

export class PortfolioTracker {
  private snapshots: Map<string, PortfolioSnapshot[]>;
  private transactions: Map<string, Transaction[]>;
  private prices: Map<string, bigint>;

  constructor() {
    this.snapshots = new Map();
    this.transactions = new Map();
    this.prices = new Map();
  }

  /**
   * Update prices
   */
  setPrice(token: string, price: bigint): void {
    this.prices.set(token, price);
  }

  /**
   * Snapshot portfolio
   */
  snapshot(user: string, positions: PortfolioPosition[]): PortfolioSnapshot {
    const totalValue = positions.reduce((sum, p) => sum + p.value, 0n);
    
    const snapshot: PortfolioSnapshot = {
      id: `snap_${Date.now()}`,
      user,
      totalValue,
      positions: [...positions],
      timestamp: Date.now(),
    };

    const userSnapshots = this.snapshots.get(user) || [];
    userSnapshots.push(snapshot);
    this.snapshots.set(user, userSnapshots);

    return snapshot;
  }

  /**
   * Record transaction
   */
  recordTransaction(user: string, tx: Transaction): void {
    const userTxs = this.transactions.get(user) || [];
    userTxs.push(tx);
    this.transactions.set(user, userTxs);
  }

  /**
   * Get portfolio value
   */
  getPortfolioValue(user: string): bigint {
    const snapshots = this.snapshots.get(user);
    if (!snapshots || snapshots.length === 0) return 0n;
    return snapshots[snapshots.length - 1].totalValue;
  }

  /**
   * Get PnL
   */
  getPnL(user: string): { pnl: bigint; pnlPercent: number } {
    const snapshots = this.snapshots.get(user);
    if (!snapshots || snapshots.length < 2) return { pnl: 0n, pnlPercent: 0 };

    const first = snapshots[0].totalValue;
    const last = snapshots[snapshots.length - 1].totalValue;
    const pnl = last - first;
    const pnlPercent = first > 0n ? (Number(pnl) / Number(first)) * 100 : 0;

    return { pnl, pnlPercent };
  }

  /**
   * Get transactions
   */
  getTransactions(user: string, limit?: number): Transaction[] {
    const txs = this.transactions.get(user) || [];
    return limit ? txs.slice(-limit) : txs;
  }

  /**
   * Get history
   */
  getHistory(user: string, days: number = 30): PortfolioSnapshot[] {
    const since = Date.now() - days * 24 * 60 * 60 * 1000;
    const snapshots = this.snapshots.get(user) || [];
    return snapshots.filter(s => s.timestamp >= since);
  }
}

export default PortfolioTracker;