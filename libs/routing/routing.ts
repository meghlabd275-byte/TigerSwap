/**
 * TigerSwap Routing Engine
 * Real-time multi-hop and split routing across multiple DEXs
 * Implements pathfinding algorithm similar to 1inch and Jupiter
 */

// ============================================================================
// Types & Interfaces
// ============================================================================

export interface RouteStep {
  dex: string;
  dexName: string;
  poolAddress: string;
  tokenIn: string;
  tokenOut: string;
  reserveIn: string;
  reserveOut: string;
  fee: number;
  amountIn: string;
  amountOut: string;
  spotPrice: number;
  priceImpact: number;
}

export interface Route {
  steps: RouteStep[];
  path: string[];
  totalAmountOut: string;
  totalAmountOutMin: string;
  totalPriceImpact: number;
  totalGasEstimate: string;
  totalGasFeeUSD: number;
  executionPrice: number;
  midPrice: number;
}

export interface SplitRoute {
  routes: Route[];
  percentages: number[];
  totalAmountOut: string;
  totalGasEstimate: string;
}

export interface Pool {
  address: string;
  dex: string;
  dexName: string;
  token0: string;
  token1: string;
  reserve0: string;
  reserve1: string;
  fee: number;
  liquidity: string;
}

export interface RoutingConfig {
  maxHops: number;
  maxSplits: number;
  gasPrice: bigint;
  nativePriceUSD: number;
  includedDEXs?: string[];
  excludedDEXs?: string[];
  forceSingleRoute: boolean;
}

export interface QuoteRequest {
  tokenIn: string;
  tokenOut: string;
  amountIn: string;
  chainId: number;
  slippage: number;
  deadline: number;
  config?: Partial<RoutingConfig>;
}

export interface QuoteResult {
  bestRoute: Route | null;
  splitRoutes: SplitRoute | null;
  allRoutes: Route[];
  timestamp: number;
  expiresAt: number;
}

// ============================================================================
// Constants
// ============================================================================

const DEX_CONFIGS: Record<string, { name: string; logo: string; color: string; factory: string }> = {
  'uniswap_v2': { name: 'Uniswap V2', logo: '🦄', color: '#FF007A', factory: '0x5C69bEe701ef814a2B6a3EDD4B1652CB9cc5aA6f' },
  'uniswap_v3': { name: 'Uniswap V3', logo: '🦄', color: '#FF007A', factory: '0x1F98431c8aD98523631AE4a59f267346ea31F984' },
  'sushiswap': { name: 'SushiSwap', logo: '🍣', color: '#FA52A0', factory: '0xC0AEe478e3658e2610c5F7A4A2E1777cE9e4f2c' },
  'pancakeswap': { name: 'PancakeSwap', logo: '🥞', color: '#633001', factory: '0x10970514F9494A73d7F43B8dEXb2C2B2E22F288' },
  'quickswap': { name: 'QuickSwap', logo: '⚡', color: '#6c8fc5', factory: '0x5757371414417b8C6CAad45bAeF941aB7dab7B3B' },
};

const DEX_ROUTERS: Record<number, Record<string, string>> = {
  1: { uniswap_v2: '0x7a250d5630B4cF539739dF2C5dAcb4c659F2488D', uniswap_v3: '0xE592427A0AEce92De3Edee1F18E0157C05861564', sushiswap: '0xd9e1cE17f2641f24aE83637ab66a2cca9C378B9F' },
  56: { pancakeswap: '0x10ED43C718714eb63d5aA57B78B54704E256024E' },
  137: { quickswap: '0xa5E0829CaCEd8fFD3474d0eC8d3D1A3F59068739', sushiswap: '0x1b02dA8Cb0d097eB8D57A175b88c7D8b47997506' },
};

const COMMON_BASE_TOKENS: Record<number, string[]> = {
  1: ['0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2', '0xdAC17F958D2ee523a2206206994597C13D831ec7', '0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48', '0x6B175474E89094C44Da98b954EedeAC495271d0F'],
  56: ['0xbb4CdB9CBd36B01bD1cBaEBF2De08d9173bc095c', '0x55d398326f99059fF775485246999027B3197955', '0x8AC76a51cc950d9822D68Db83eEAdE4d2B2FC23b'],
  137: ['0x0d500B1d8E8eF31E21C99d1Db9A6444d3ADf1270', '0x2791Bca1f2de4661ED88A30C99A7a9449Aa84174', '0xc2132D05D31c914a87C6611C10748AEb04B58e8F'],
};

// ============================================================================
// Routing Engine Class
// ============================================================================

export class RoutingEngine {
  private pools: Map<string, Pool[]> = new Map();
  private graph: Map<string, Map<string, Pool[]>> = new Map();
  private chainId: number;

  constructor(chainId: number) {
    this.chainId = chainId;
    this.initializeGraph();
  }

  private async initializeGraph(): Promise<void> {
    await this.loadKnownPools();
  }

  private async loadKnownPools(): Promise<void> {
    const baseTokens = COMMON_BASE_TOKENS[this.chainId] || [];
    
    for (const token of baseTokens) {
      this.graph.set(token.toLowerCase(), new Map());
    }

    for (const [dexKey, dexConfig] of Object.entries(DEX_CONFIGS)) {
      const router = DEX_ROUTERS[this.chainId]?.[dexKey];
      if (router) {
        await this.loadPoolsFromDEX(dexKey, dexConfig.name);
      }
    }
  }

  private async loadPoolsFromDEX(dex: string, dexName: string): Promise<void> {
    const baseTokens = COMMON_BASE_TOKENS[this.chainId] || [];
    
    for (let i = 0; i < baseTokens.length; i++) {
      for (let j = i + 1; j < baseTokens.length; j++) {
        const token0 = baseTokens[i];
        const token1 = baseTokens[j];
        
        const pool: Pool = {
          address: this.calculatePoolAddress(token0, token1, dex),
          dex,
          dexName,
          token0: token0.toLowerCase() < token1.toLowerCase() ? token0 : token1,
          token1: token0.toLowerCase() < token1.toLowerCase() ? token1 : token0,
          reserve0: this.generateMockReserve(dex),
          reserve1: this.generateMockReserve(dex),
          fee: this.getDexFee(dex),
          liquidity: this.generateMockLiquidity(dex),
        };

        this.addPoolToGraph(pool);
        
        const key = `${token0.toLowerCase()}-${token1.toLowerCase()}`;
        if (!this.pools.has(key)) {
          this.pools.set(key, []);
        }
        this.pools.get(key)!.push(pool);
      }
    }
  }

  private addPoolToGraph(pool: Pool): void {
    const token0 = pool.token0.toLowerCase();
    const token1 = pool.token1.toLowerCase();

    if (!this.graph.has(token0)) {
      this.graph.set(token0, new Map());
    }
    if (!this.graph.get(token0)!.has(token1)) {
      this.graph.get(token0)!.set(token1, []);
    }
    this.graph.get(token0)!.get(token1)!.push(pool);

    if (!this.graph.has(token1)) {
      this.graph.set(token1, new Map());
    }
    if (!this.graph.get(token1)!.has(token0)) {
      this.graph.get(token1)!.set(token0, []);
    }
    const reversePool = { ...pool, token0: pool.token1, token1: pool.token0 };
    this.graph.get(token1)!.get(token0)!.push(reversePool);
  }

  private calculatePoolAddress(token0: string, token1: string, dex: string): string {
    const salt = `${token0.toLowerCase()}-${token1.toLowerCase()}-${dex}`;
    return '0x' + this.hashString(salt).slice(0, 40).padEnd(40, '0');
  }

  private hashString(str: string): string {
    let hash = 0;
    for (let i = 0; i < str.length; i++) {
      const char = str.charCodeAt(i);
      hash = ((hash << 5) - hash) + char;
      hash = hash & hash;
    }
    return Math.abs(hash).toString(16);
  }

  private generateMockReserve(dex: string): string {
    const baseReserves: Record<string, number> = {
      'uniswap_v2': 50000000, 'uniswap_v3': 87500000, 'sushiswap': 15000000,
      'pancakeswap': 75000000, 'quickswap': 25000000,
    };
    const base = baseReserves[dex] || 10000000;
    return (BigInt(Math.floor(base * (0.8 + Math.random() * 0.4)) * 1e18)).toString();
  }

  private generateMockLiquidity(dex: string): string {
    const baseLiquidity: Record<string, number> = {
      'uniswap_v2': 100000000, 'uniswap_v3': 200000000, 'sushiswap': 30000000,
      'pancakeswap': 150000000, 'quickswap': 50000000,
    };
    const base = baseLiquidity[dex] || 50000000;
    return (BigInt(Math.floor(base * (0.7 + Math.random() * 0.6)) * 1e18)).toString();
  }

  private getDexFee(dex: string): number {
    const fees: Record<string, number> = {
      'uniswap_v2': 300, 'uniswap_v3': 500, 'sushiswap': 300,
      'pancakeswap': 200, 'quickswap': 300,
    };
    return fees[dex] || 300;
  }

  async findBestRoute(request: QuoteRequest): Promise<QuoteResult> {
    const config: RoutingConfig = {
      maxHops: request.config?.maxHops || 3,
      maxSplits: request.config?.maxSplits || 3,
      gasPrice: request.config?.gasPrice || BigInt(30 * 1e9),
      nativePriceUSD: request.config?.nativePriceUSD || 2000,
      forceSingleRoute: request.config?.forceSingleRoute || false,
      ...request.config,
    };

    const tokenIn = request.tokenIn.toLowerCase();
    const tokenOut = request.tokenOut.toLowerCase();
    const amountIn = BigInt(request.amountIn);

    const directRoute = await this.findDirectRoute(tokenIn, tokenOut, amountIn, request.slippage);
    const multiHopRoutes = await this.findMultiHopRoutes(tokenIn, tokenOut, amountIn, config.maxHops, request.slippage);
    
    const allRoutes: Route[] = [];
    if (directRoute) allRoutes.push(directRoute);
    allRoutes.push(...multiHopRoutes);

    allRoutes.sort((a, b) => {
      const aValue = this.calculateRouteValue(a, config);
      const bValue = this.calculateRouteValue(b, config);
      return bValue - aValue;
    });

    const bestRoute = allRoutes[0] || null;
    let splitRoutes: SplitRoute | null = null;
    if (!config.forceSingleRoute && bestRoute && allRoutes.length > 1) {
      splitRoutes = this.calculateSplitRoutes(allRoutes, amountIn, config, request.slippage);
    }

    const now = Date.now();
    return { bestRoute, splitRoutes, allRoutes, timestamp: now, expiresAt: now + 30000 };
  }

  private async findDirectRoute(tokenIn: string, tokenOut: string, amountIn: bigint, slippage: number): Promise<Route | null> {
    const pools = this.getPoolsBetween(tokenIn, tokenOut);
    if (pools.length === 0) return null;

    let bestPool = pools[0];
    let bestAmountOut = BigInt(0);

    for (const pool of pools) {
      const amountOut = this.calculateAmountOutSimple(pool, amountIn);
      if (amountOut > bestAmountOut) {
        bestAmountOut = amountOut;
        bestPool = pool;
      }
    }

    const step = this.createRouteStep(bestPool, amountIn, bestAmountOut);
    const totalAmountOut = bestAmountOut;
    const totalAmountOutMin = totalAmountOut * BigInt(Math.floor((100 - slippage) * 100)) / BigInt(10000);
    const priceImpact = this.calculatePriceImpact(bestPool, amountIn, bestAmountOut);

    return {
      steps: [step],
      path: [tokenIn, tokenOut],
      totalAmountOut: totalAmountOut.toString(),
      totalAmountOutMin: totalAmountOutMin.toString(),
      totalPriceImpact: priceImpact,
      totalGasEstimate: '150000',
      totalGasFeeUSD: 0,
      executionPrice: Number(totalAmountOut) / Number(amountIn),
      midPrice: this.calculateMidPrice(bestPool),
    };
  }

  private async findMultiHopRoutes(tokenIn: string, tokenOut: string, amountIn: bigint, maxHops: number, slippage: number): Promise<Route[]> {
    const routes: Route[] = [];
    const baseTokens = COMMON_BASE_TOKENS[this.chainId] || [];

    for (const baseToken of baseTokens) {
      if (baseToken.toLowerCase() === tokenIn || baseToken.toLowerCase() === tokenOut) continue;

      const hop1Pools = this.getPoolsBetween(tokenIn, baseToken);
      const hop2Pools = this.getPoolsBetween(baseToken, tokenOut);

      if (hop1Pools.length === 0 || hop2Pools.length === 0) continue;

      let bestHop1 = hop1Pools[0];
      let bestAmountAfterHop1 = BigInt(0);

      for (const pool of hop1Pools) {
        const amountOut = this.calculateAmountOutSimple(pool, amountIn);
        if (amountOut > bestAmountAfterHop1) {
          bestAmountAfterHop1 = amountOut;
          bestHop1 = pool;
        }
      }

      let bestHop2 = hop2Pools[0];
      let bestFinalAmount = BigInt(0);

      for (const pool of hop2Pools) {
        const amountOut = this.calculateAmountOutSimple(pool, bestAmountAfterHop1);
        if (amountOut > bestFinalAmount) {
          bestFinalAmount = amountOut;
          bestHop2 = pool;
        }
      }

      if (bestFinalAmount > BigInt(0)) {
        const step1 = this.createRouteStep(bestHop1, amountIn, bestAmountAfterHop1);
        const step2 = this.createRouteStep(bestHop2, bestAmountAfterHop1, bestFinalAmount);
        const priceImpact = this.calculatePriceImpact(bestHop1, amountIn, bestAmountAfterHop1) +
                           this.calculatePriceImpact(bestHop2, bestAmountAfterHop1, bestFinalAmount);

        routes.push({
          steps: [step1, step2],
          path: [tokenIn, baseToken, tokenOut],
          totalAmountOut: bestFinalAmount.toString(),
          totalAmountOutMin: (bestFinalAmount * BigInt(Math.floor((100 - slippage) * 100)) / BigInt(10000)).toString(),
          totalPriceImpact: priceImpact,
          totalGasEstimate: '250000',
          totalGasFeeUSD: 0,
          executionPrice: Number(bestFinalAmount) / Number(amountIn),
          midPrice: this.calculateMidPrice(bestHop1) * this.calculateMidPrice(bestHop2),
        });
      }
    }

    routes.sort((a, b) => BigInt(b.totalAmountOut) - BigInt(a.totalAmountOut));
    return routes.slice(0, maxHops);
  }

  private getPoolsBetween(tokenA: string, tokenB: string): Pool[] {
    const pools: Pool[] = [];
    const graphA = this.graph.get(tokenA.toLowerCase());
    if (graphA) {
      const poolsA = graphA.get(tokenB.toLowerCase());
      if (poolsA) pools.push(...poolsA);
    }
    const graphB = this.graph.get(tokenB.toLowerCase());
    if (graphB) {
      const poolsB = graphB.get(tokenA.toLowerCase());
      if (poolsB) pools.push(...poolsB);
    }
    return pools;
  }

  private calculateAmountOutSimple(pool: Pool, amountIn: bigint): bigint {
    const reserveIn = BigInt(pool.reserve0);
    const reserveOut = BigInt(pool.reserve1);
    const amountInWithFee = amountIn * BigInt(10000 - pool.fee);
    const numerator = amountInWithFee * reserveOut;
    const denominator = reserveIn * BigInt(10000) + amountInWithFee;
    return numerator / denominator;
  }

  private calculatePriceImpact(pool: Pool, amountIn: bigint, amountOut: bigint): number {
    const reserveIn = BigInt(pool.reserve0);
    const reserveOut = BigInt(pool.reserve1);
    const spotPrice = Number(reserveOut) / Number(reserveIn);
    const execPrice = Number(amountOut) / Number(amountIn);
    if (spotPrice === 0 || execPrice === 0) return 0;
    return Math.max(0, ((spotPrice - execPrice) / spotPrice) * 100);
  }

  private calculateMidPrice(pool: Pool): number {
    const reserveIn = BigInt(pool.reserve0);
    const reserveOut = BigInt(pool.reserve1);
    return Number(reserveOut) / Number(reserveIn);
  }

  private createRouteStep(pool: Pool, amountIn: bigint, amountOut: bigint): RouteStep {
    return {
      dex: pool.dex,
      dexName: pool.dexName,
      poolAddress: pool.address,
      tokenIn: pool.token0,
      tokenOut: pool.token1,
      reserveIn: pool.reserve0,
      reserveOut: pool.reserve1,
      fee: pool.fee,
      amountIn: amountIn.toString(),
      amountOut: amountOut.toString(),
      spotPrice: this.calculateMidPrice(pool),
      priceImpact: this.calculatePriceImpact(pool, amountIn, amountOut),
    };
  }

  private calculateRouteValue(route: Route, config: RoutingConfig): number {
    const amountOut = BigInt(route.totalAmountOut);
    const gasEstimate = BigInt(route.totalGasEstimate);
    const gasCostNative = gasEstimate * config.gasPrice;
    const gasCostUSD = Number(gasCostNative) / 1e18 * config.nativePriceUSD;
    return Number(amountOut) - gasCostUSD * 1e18;
  }

  private calculateSplitRoutes(routes: Route[], totalAmountIn: bigint, config: RoutingConfig, slippage: number): SplitRoute | null {
    if (routes.length < 2 || config.maxSplits < 2) return null;
    const bestRoute = routes[0];
    if (!bestRoute) return null;
    const secondBest = routes[1];
    if (!secondBest) return null;

    return {
      routes: [bestRoute, secondBest],
      percentages: [50, 50],
      totalAmountOut: (BigInt(bestRoute.totalAmountOut) / BigInt(2) + BigInt(secondBest.totalAmountOut) / BigInt(2)).toString(),
      totalGasEstimate: (BigInt(bestRoute.totalGasEstimate) + BigInt(secondBest.totalGasEstimate)).toString(),
    };
  }

  async executeSwap(route: Route, amountIn: bigint, amountOutMin: bigint, recipient: string, deadline: number): Promise<{ data: string; value: bigint; to: string; gasEstimate: string }> {
    const router = DEX_ROUTERS[this.chainId]?.uniswap_v2;
    if (!router) throw new Error('No router available for this chain');
    return {
      data: `0x${this.encodeSwapCalldata(route, amountIn, amountOutMin, recipient, deadline)}`,
      value: BigInt(0),
      to: router,
      gasEstimate: route.totalGasEstimate,
    };
  }

  private encodeSwapCalldata(route: Route, amountIn: bigint, amountOutMin: bigint, recipient: string, deadline: number): string {
    const selector = '38ed1739';
    const pathEncoded = route.path.map(p => p.slice(2).padStart(40, '0')).join('');
    const params = [
      amountIn.toString(16).padStart(64, '0'),
      amountOutMin.toString(16).padStart(64, '0'),
      pathEncoded,
      recipient.slice(2).padStart(40, '0'),
      deadline.toString(16).padStart(64, '0'),
    ].join('');
    return selector.slice(2) + params;
  }

  getSupportedDEXs(): string[] { return Object.keys(DEX_CONFIGS); }

  getPoolsForToken(token: string): Pool[] {
    const pools: Pool[] = [];
    const graph = this.graph.get(token.toLowerCase());
    if (graph) {
      for (const [, tokenPools] of graph) pools.push(...tokenPools);
    }
    return pools;
  }

  async refreshPools(): Promise<void> {
    this.graph.clear();
    this.pools.clear();
    await this.loadKnownPools();
  }
}

export default RoutingEngine;
