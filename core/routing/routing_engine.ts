/**
 * TigerSwap DEX Aggregator Routing Engine - Complete Native Implementation
 * Built from scratch - no dependencies on 1inch, Paraswap or other aggregators
 */

import { AMMFactory, PoolCore } from '../amm/concentrated_amm';

export interface RouteStep {
  pool: string;
  tokenIn: string;
  tokenOut: string;
  amountIn: bigint;
  amountOut: bigint;
  fee: number;
  priceImpact: number;
}

export interface Route {
  steps: RouteStep[];
  path: string[];
  totalAmountOut: bigint;
  totalPriceImpact: number;
  totalGasEstimate: bigint;
  executionPrice: number;
}

export interface SplitRoute {
  routes: Route[];
  percentages: number[];
  totalAmountOut: bigint;
  gasEstimate: bigint;
}

export interface QuoteRequest {
  tokenIn: string;
  tokenOut: string;
  amountIn: bigint;
  slippage: number;
  maxHops: number;
  gasPrice: bigint;
  nativePriceUSD: number;
}

export interface RoutingConfig {
  maxHops: number;
  maxSplits: number;
  gasPrice: bigint;
  nativePriceUSD: number;
  forceSingleRoute: boolean;
  includedDEXs?: string[];
  excludedDEXs?: string[];
}

const MAX_UINT256 = BigInt('115792089237316195423570985008687907853269984665640564039457584007913129639935');

export class RoutingEngine {
  private pools: Map<string, PoolCore> = new Map();
  private tokenDecimals: Map<string, number> = new Map();
  private config: RoutingConfig;

  constructor(config: Partial<RoutingConfig> = {}) {
    this.config = {
      maxHops: config.maxHops || 3,
      maxSplits: config.maxSplits || 3,
      gasPrice: config.gasPrice || BigInt(30 * 1e9),
      nativePriceUSD: config.nativePriceUSD || 2000,
      forceSingleRoute: config.forceSingleRoute || false,
      includedDEXs: config.includedDEXs,
      excludedDEXs: config.excludedDEXs,
    };
  }

  /**
   * Register a pool with the routing engine
   */
  registerPool(address: string, pool: PoolCore): void {
    this.pools.set(address, pool);
  }

  /**
   * Set token decimals
   */
  setTokenDecimals(token: string, decimals: number): void {
    this.tokenDecimals.set(token.toLowerCase(), decimals);
  }

  /**
   * Find the best route for a swap
   */
  findBestRoute(request: QuoteRequest): { route: Route; splitRoutes: SplitRoute | null } {
    const directRoutes = this.findDirectRoutes(request);
    const multiHopRoutes = this.findMultiHopRoutes(request);
    
    const allRoutes = [...directRoutes, ...multiHopRoutes];
    
    if (allRoutes.length === 0) {
      throw new Error('No route found');
    }

    // Sort by output amount (gas-adjusted)
    allRoutes.sort((a, b) => {
      const valueA = this.calculateRouteValue(a, request.gasPrice, request.nativePriceUSD);
      const valueB = this.calculateRouteValue(b, request.gasPrice, request.nativePriceUSD);
      return valueB > valueA ? 1 : -1;
    });

    const bestRoute = allRoutes[0];
    
    // Calculate split routes if beneficial
    let splitRoutes: SplitRoute | null = null;
    if (!this.config.forceSingleRoute && allRoutes.length > 1 && this.config.maxSplits > 1) {
      splitRoutes = this.calculateSplitRoutes(allRoutes, request.amountIn);
    }

    return { route: bestRoute, splitRoutes };
  }

  /**
   * Find direct routes (single pool)
   */
  private findDirectRoutes(request: QuoteRequest): Route[] {
    const routes: Route[] = [];

    for (const [address, pool] of this.pools) {
      const state = pool.getState();
      
      // Check if pool matches the token pair
      const tokenIn = request.tokenIn.toLowerCase();
      const tokenOut = request.tokenOut.toLowerCase();
      
      if ((state.token0.toLowerCase() === tokenIn && state.token1.toLowerCase() === tokenOut) ||
          (state.token0.toLowerCase() === tokenOut && state.token1.toLowerCase() === tokenIn)) {
        
        const isReversed = state.token0.toLowerCase() !== tokenIn;
        const amountOut = this.calculateAmountOut(pool, request.amountIn, isReversed);
        
        if (amountOut > BigInt(0)) {
          const priceImpact = this.calculatePriceImpact(pool, request.amountIn, amountOut, isReversed);
          
          routes.push({
            steps: [{
              pool: address,
              tokenIn: request.tokenIn,
              tokenOut: request.tokenOut,
              amountIn: request.amountIn,
              amountOut,
              fee: state.fee,
              priceImpact,
            }],
            path: [request.tokenIn, request.tokenOut],
            totalAmountOut: amountOut,
            totalPriceImpact: priceImpact,
            totalGasEstimate: BigInt(150000),
            executionPrice: Number(amountOut) / Number(request.amountIn),
          });
        }
      }
    }

    return routes;
  }

  /**
   * Find multi-hop routes
   */
  private findMultiHopRoutes(request: QuoteRequest): Route[] {
    const routes: Route[] = [];
    const visited = new Set<string>();

    this.findRoutesRecursive(
      request.tokenIn,
      request.tokenOut,
      request.amountIn,
      [],
      routes,
      visited,
      0
    );

    return routes;
  }

  /**
   * Recursive route finding
   */
  private findRoutesRecursive(
    tokenIn: string,
    tokenOut: string,
    amountIn: bigint,
    currentPath: RouteStep[],
    routes: Route[],
    visited: Set<string>,
    depth: number
  ): void {
    if (depth >= this.config.maxHops) return;

    for (const [address, pool] of this.pools) {
      if (visited.has(address)) continue;
      
      const state = pool.getState();
      const token0 = state.token0.toLowerCase();
      const token1 = state.token1.toLowerCase();

      // Check if this pool connects to current token
      let intermediateToken: string | null = null;
      let isReversed = false;

      if (token0 === tokenIn.toLowerCase()) {
        intermediateToken = token1;
        isReversed = false;
      } else if (token1 === tokenIn.toLowerCase()) {
        intermediateToken = token0;
        isReversed = true;
      }

      if (!intermediateToken) continue;

      // Check if this leads to the target
      if (intermediateToken.toLowerCase() === tokenOut.toLowerCase()) {
        // Found a complete route
        const amountOut = this.calculateAmountOut(pool, amountIn, isReversed);
        
        const step: RouteStep = {
          pool: address,
          tokenIn,
          tokenOut,
          amountIn,
          amountOut,
          fee: state.fee,
          priceImpact: this.calculatePriceImpact(pool, amountIn, amountOut, isReversed),
        };

        const fullPath = [...currentPath, step];
        
        routes.push({
          steps: fullPath,
          path: fullPath.map(s => s.tokenIn).concat([tokenOut]),
          totalAmountOut: amountOut,
          totalPriceImpact: fullPath.reduce((sum, s) => sum + s.priceImpact, 0),
          totalGasEstimate: BigInt(150000 * (depth + 1)),
          executionPrice: Number(amountOut) / Number(amountIn),
        });
      } else {
        // Continue searching
        const nextAmountOut = this.calculateAmountOut(pool, amountIn, isReversed);
        
        const step: RouteStep = {
          pool: address,
          tokenIn,
          tokenOut: intermediateToken,
          amountIn,
          amountOut: nextAmountOut,
          fee: state.fee,
          priceImpact: this.calculatePriceImpact(pool, amountIn, nextAmountOut, isReversed),
        };

        visited.add(address);
        this.findRoutesRecursive(
          intermediateToken,
          tokenOut,
          nextAmountOut,
          [...currentPath, step],
          routes,
          visited,
          depth + 1
        );
        visited.delete(address);
      }
    }
  }

  /**
   * Calculate amount out from a pool
   */
  private calculateAmountOut(pool: PoolCore, amountIn: bigint, isReversed: boolean): bigint {
    const state = pool.getState();
    const fee = BigInt(state.fee);
    const feeMultiplier = BigInt(1000000 - Number(fee));
    
    if (isReversed) {
      const reserveIn = pool.getReserve1();
      const reserveOut = pool.getReserve0();
      if (reserveIn === BigInt(0) || reserveOut === BigInt(0)) return BigInt(0);
      
      const amountInWithFee = (amountIn * feeMultiplier) / BigInt(1000000);
      const newReserveIn = reserveIn + amountInWithFee;
      const newReserveOut = (reserveIn * reserveOut) / newReserveIn;
      return reserveOut - newReserveOut;
    } else {
      const reserveIn = pool.getReserve0();
      const reserveOut = pool.getReserve1();
      if (reserveIn === BigInt(0) || reserveOut === BigInt(0)) return BigInt(0);
      
      const amountInWithFee = (amountIn * feeMultiplier) / BigInt(1000000);
      const newReserveIn = reserveIn + amountInWithFee;
      const newReserveOut = (reserveIn * reserveOut) / newReserveIn;
      return reserveOut - newReserveOut;
    }
  }

  /**
   * Calculate price impact
   */
  private calculatePriceImpact(pool: PoolCore, amountIn: bigint, amountOut: bigint, isReversed: boolean): number {
    const state = pool.getState();
    const spotPrice = pool.getCurrentPrice();
    
    if (isReversed) {
      const execPrice = 1 / (Number(amountOut) / Number(amountIn));
      return Math.max(0, ((spotPrice - execPrice) / spotPrice) * 100);
    } else {
      const execPrice = Number(amountOut) / Number(amountIn);
      return Math.max(0, ((spotPrice - execPrice) / spotPrice) * 100);
    }
  }

  /**
   * Calculate gas-adjusted route value
   */
  private calculateRouteValue(route: Route, gasPrice: bigint, nativePriceUSD: number): number {
    const amountOut = route.totalAmountOut;
    const gasEstimate = route.totalGasEstimate;
    
    const gasCostNative = gasEstimate * gasPrice;
    const gasCostUSD = Number(gasCostNative) / 1e18 * nativePriceUSD;
    
    const decimals = this.tokenDecimals.get(route.path[route.path.length - 1].toLowerCase()) || 18;
    const amountOutUSD = Number(amountOut) / Math.pow(10, decimals);
    
    return amountOutUSD - gasCostUSD;
  }

  /**
   * Calculate split routes
   */
  private calculateSplitRoutes(routes: Route[], totalAmountIn: bigint): SplitRoute | null {
    if (routes.length < 2) return null;

    const bestRoute = routes[0];
    const secondBest = routes[1];
    
    if (!bestRoute || !secondBest) return null;

    // Simple 50/50 split
    const splitAmount1 = totalAmountIn / BigInt(2);
    const splitAmount2 = totalAmountIn - splitAmount1;

    // Calculate amounts for split
    const amountOut1 = this.scaleRoute(bestRoute, splitAmount1);
    const amountOut2 = this.scaleRoute(secondBest, splitAmount2);

    const totalAmountOut = amountOut1 + amountOut2;

    // Only use split if it improves total output
    if (totalAmountOut <= bestRoute.totalAmountOut) {
      return null;
    }

    return {
      routes: [bestRoute, secondBest],
      percentages: [50, 50],
      totalAmountOut,
      gasEstimate: bestRoute.totalGasEstimate + secondBest.totalGasEstimate,
    };
  }

  /**
   * Scale a route to a different input amount
   */
  private scaleRoute(route: Route, amountIn: bigint): bigint {
    const originalAmountIn = route.steps[0]?.amountIn || BigInt(1);
    const scaleFactor = Number(amountIn) / Number(originalAmountIn);
    return BigInt(Math.floor(Number(route.totalAmountOut) * scaleFactor));
  }

  /**
   * Get all pools for a token
   */
  getPoolsForToken(token: string): PoolCore[] {
    const pools: PoolCore[] = [];
    const tokenLower = token.toLowerCase();
    
    for (const pool of this.pools.values()) {
      const state = pool.getState();
      if (state.token0.toLowerCase() === tokenLower || state.token1.toLowerCase() === tokenLower) {
        pools.push(pool);
      }
    }
    
    return pools;
  }

  /**
   * Get all registered pools
   */
  getAllPools(): Map<string, PoolCore> {
    return new Map(this.pools);
  }

  /**
   * Clear all pools
   */
  clearPools(): void {
    this.pools.clear();
  }
}

export default { RoutingEngine };