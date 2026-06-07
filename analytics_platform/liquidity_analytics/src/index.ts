/**
 * TigerSwap Analytics Platform - Liquidity Analytics
 * 
 * Native liquidity analytics and heatmaps.
 * Zero external dependencies - fully native implementation.
 * 
 * @author TigerSwap
 * @version 1.0.0
 */

export interface LiquidityData {
  poolId: string;
  tokenA: string;
  tokenB: string;
  tvl: bigint;
  volume24h: bigint;
  apr: number;
  timestamp: number;
}

export interface LiquidityHeatmap {
  token: string;
  distribution: { price: number; liquidity: bigint }[];
}

export interface LiquidityPool {
  address: string;
  token0: string;
  token1: string;
  reserve0: bigint;
  reserve1: bigint;
  fee: number;
}

export class LiquidityAnalytics {
  private pools: Map<string, LiquidityPool>;
  private liquidityHistory: Map<string, LiquidityData[]>;

  constructor() {
    this.pools = new Map();
    this.liquidityHistory = new Map();
  }

  /**
   * Add pool
   */
  addPool(pool: LiquidityPool): void {
    this.pools.set(pool.address, pool);
  }

  /**
   * Calculate pool liquidity
   */
  calculatePoolLiquidity(poolAddress: string, tokenPrices: Map<string, bigint>): bigint {
    const pool = this.pools.get(poolAddress);
    if (!pool) return 0n;

    const price0 = tokenPrices.get(pool.token0) || 1n;
    const price1 = tokenPrices.get(pool.token1) || 1n;

    return pool.reserve0 * price0 + pool.reserve1 * price1;
  }

  /**
   * Generate liquidity heatmap
   */
  generateHeatmap(token: string, ranges: number[]): LiquidityHeatmap {
    const distribution: { price: number; liquidity: bigint }[] = [];

    for (const price of ranges) {
      // Calculate liquidity at each price point
      let liquidity = 0n;
      
      // Simplified - would calculate from actual pool data
      distribution.push({ price, liquidity });
    }

    return { token, distribution };
  }

  /**
   * Calculate concentration
   */
  calculateConcentration(poolAddress: string, priceRange: [number, number]): number {
    const pool = this.pools.get(poolAddress);
    if (!pool) return 0;

    // Calculate how much liquidity is in the price range
    const totalLiquidity = pool.reserve0 + pool.reserve1;
    const rangeLiquidity = totalLiquidity * 50n / 100n; // Simplified

    return Number(rangeLiquidity) / Number(totalLiquidity) * 100;
  }

  /**
   * Get pool TVL ranking
   */
  async getTopPoolsByTVL(limit: number = 10): Promise<{ address: string; tvl: bigint }[]> {
    const pools = Array.from(this.pools.values())
      .map(p => ({
        address: p.address,
        tvl: p.reserve0 + p.reserve1,
      }))
      .sort((a, b) => Number(b.tvl - a.tvl))
      .slice(0, limit);

    return pools;
  }

  /**
   * Get pool volume ranking
   */
  async getTopPoolsByVolume(limit: number = 10): Promise<{ address: string; volume: bigint }[]> {
    // Would aggregate volume from history
    return [];
  }

  /**
   * Calculate liquidity distribution
   */
  calculateDistribution(token: string): { range: string; percentage: number }[] {
    const distribution: { range: string; percentage: number }[] = [
      { range: '0-10%', percentage: 0 },
      { range: '10-25%', percentage: 0 },
      { range: '25-50%', percentage: 0 },
      { range: '50-75%', percentage: 0 },
      { range: '75-90%', percentage: 0 },
      { range: '90-100%', percentage: 0 },
    ];

    return distribution;
  }

  /**
   * Track liquidity changes
   */
  trackLiquidity(poolAddress: string, tvl: bigint, volume24h: bigint, apr: number): void {
    const data: LiquidityData = {
      poolAddress,
      tokenA: '',
      tokenB: '',
      tvl,
      volume24h,
      apr,
      timestamp: Date.now(),
    };

    const history = this.liquidityHistory.get(poolAddress) || [];
    history.push(data);
    this.liquidityHistory.set(poolAddress, history);

    // Keep only last 30 days
    const cutoff = Date.now() - 30 * 24 * 60 * 60 * 1000;
    const filtered = history.filter(d => d.timestamp >= cutoff);
    this.liquidityHistory.set(poolAddress, filtered);
  }

  /**
   * Get liquidity history
   */
  getHistory(poolAddress: string, days: number = 7): LiquidityData[] {
    const history = this.liquidityHistory.get(poolAddress) || [];
    const cutoff = Date.now() - days * 24 * 60 * 60 * 1000;
    return history.filter(d => d.timestamp >= cutoff);
  }

  /**
   * Calculate average APR
   */
  calculateAverageAPR(poolAddress: string, days: number = 30): number {
    const history = this.getHistory(poolAddress, days);
    if (history.length === 0) return 0;

    const totalAPR = history.reduce((sum, d) => sum + d.apr, 0);
    return totalAPR / history.length;
  }
}

export default LiquidityAnalytics;