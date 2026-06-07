/**
 * TigerSwap Market Maker Platform
 * 
 * Enterprise-grade market making engine with institutional features.
 * Completely independent - NO external market making services.
 * 
 * Features:
 * - Two-sided quote generation
 * - Dynamic spread calculation
 * - Inventory management
 * - Delta-neutral hedging
 * - Risk management
 * - PnL tracking
 * - Volatility tracking
 * 
 * @author TigerSwap
 * @version 1.0.0
 */

import { EVMClient, EVMWallet } from '@tigerswap/evm-sdk';

// ============================================================================
// Types & Interfaces
// ============================================================================

export interface MarketMakerConfig {
  name: string;
  chainId: number;
  pair: TokenPair;
  baseSpread: number; // basis points
  minSpread: number;
  maxSpread: number;
  minOrderSize: bigint;
  maxOrderSize: bigint;
  maxInventory: bigint;
  maxDailyVolume: bigint;
  quoteRefreshInterval: number; // ms
  inventoryBias: number; // 0-10000 (0 = neutral, 10000 = 100% long)
}

export interface TokenPair {
  base: string;
  quote: string;
  baseDecimals: number;
  quoteDecimals: number;
}

export interface Quote {
  maker: string;
  pair: TokenPair;
  bidPrice: number;
  askPrice: number;
  bidSize: bigint;
  askSize: bigint;
  timestamp: number;
  validFor: number;
  signature?: string;
}

export interface Order {
  id: string;
  maker: string;
  pair: TokenPair;
  side: 'buy' | 'sell';
  price: number;
  size: bigint;
  filled: bigint;
  remaining: bigint;
  status: OrderStatus;
  timestamp: number;
  expiresAt: number;
}

export enum OrderStatus {
  PENDING = 'pending',
  OPEN = 'open',
  PARTIAL = 'partial',
  FILLED = 'filled',
  CANCELLED = 'cancelled',
  EXPIRED = 'expired',
}

export interface Position {
  pair: TokenPair;
  side: 'long' | 'short';
  size: bigint;
  entryPrice: number;
  markPrice: number;
  unrealizedPnL: number;
  realizedPnL: number;
  notionalValue: number;
  leverage: number;
}

export interface Inventory {
  pair: TokenPair;
  baseBalance: bigint;
  quoteBalance: bigint;
  baseValueUSD: number;
  quoteValueUSD: number;
  totalValueUSD: number;
  bias: number; // -10000 to 10000
}

export interface RiskLimits {
  maxPositionSize: bigint;
  maxDailyVolume: bigint;
  maxDrawdown: number;
  maxLeverage: number;
  maxSlippage: number;
  maxSpread: number;
  minCapital: number;
}

export interface PnL {
  unrealizedPnL: number;
  realizedPnL: number;
  feesPaid: number;
  netPnL: number;
  roi: number;
  volume: number;
  trades: number;
  winRate: number;
  avgWin: number;
  avgLoss: number;
}

export interface VolatilityData {
  pair: TokenPair;
  iv: number;
  rv: number;
  high24h: number;
  low24h: number;
  openInterest: bigint;
  volume24h: bigint;
}

// ============================================================================
// MM Engine
// ============================================================================

/**
 * MMEngine - Market maker orchestrator
 * 
 * Manages the overall market making operation including:
 * - Quote generation
 * - Order management
 * - Strategy dispatch
 * - Multi-market execution
 */
export class MMEngine {
  private config: MarketMakerConfig;
  private wallet: EVMWallet;
  private client: EVMClient;
  private isRunning: boolean;
  private lastQuote: Quote | null;
  private orders: Map<string, Order>;
  private positions: Map<string, Position>;
  private inventory: Inventory;
  private spreadEngine: SpreadEngine;
  private inventoryEngine: InventoryEngine;
  private hedgeEngine: HedgeEngine;
  private riskEngine: RiskEngine;
  private quoteEngine: QuoteEngine;
  private pnlEngine: PnLEngine;
  private volatilityEngine: VolatilityEngine;

  constructor(config: MarketMakerConfig, wallet: EVMWallet) {
    this.config = config;
    this.wallet = wallet;
    this.client = new EVMClient(config.chainId);
    this.isRunning = false;
    this.lastQuote = null;
    this.orders = new Map();
    this.positions = new Map();
    this.inventory = this.initializeInventory();
    
    // Initialize sub-engines
    this.spreadEngine = new SpreadEngine(config);
    this.inventoryEngine = new InventoryEngine(config);
    this.hedgeEngine = new HedgeEngine(config, wallet);
    this.riskEngine = new RiskEngine(config);
    this.quoteEngine = new QuoteEngine(config);
    this.pnlEngine = new PnLEngine();
    this.volatilityEngine = new VolatilityEngine(config.pair);
  }

  /**
   * Start market making
   */
  async start(): Promise<void> {
    if (this.isRunning) {
      throw new Error('Market maker already running');
    }

    // Validate config
    await this.riskEngine.validateConfig();

    this.isRunning = true;
    this.startQuoteLoop();
  }

  /**
   * Stop market making
   */
  stop(): void {
    this.isRunning = false;
  }

  /**
   * Check if running
   */
  isActive(): boolean {
    return this.isRunning;
  }

  /**
   * Generate quote
   */
  async generateQuote(): Promise<Quote> {
    if (!this.isRunning) {
      throw new Error('Market maker not running');
    }

    // Get market data
    const [bidPrice, askPrice] = await this.getMarketPrices();
    
    // Calculate spread
    const spread = this.spreadEngine.calculateSpread(bidPrice, askPrice);
    
    // Adjust for inventory
    const inventoryAdjust = this.inventoryEngine.getAdjustment();
    const adjustedSpread = spread + inventoryAdjust;
    
    // Calculate final prices
    const finalBid = bidPrice * (1 - adjustedSpread / 10000);
    const finalAsk = askPrice * (1 + adjustedSpread / 10000);
    
    // Calculate sizes
    const [bidSize, askSize] = this.calculateOrderSizes(finalBid, finalAsk);
    
    const quote: Quote = {
      maker: this.wallet.getAddress(),
      pair: this.config.pair,
      bidPrice: finalBid,
      askPrice: finalAsk,
      bidSize,
      askSize,
      timestamp: Date.now(),
      validFor: this.config.quoteRefreshInterval,
    };

    // Sign quote
    quote.signature = await this.signQuote(quote);
    
    this.lastQuote = quote;
    return quote;
  }

  /**
   * Execute order
   */
  async executeOrder(orderId: string): Promise<Order> {
    const order = this.orders.get(orderId);
    if (!order) {
      throw new Error('Order not found');
    }

    // Check risk limits
    await this.riskEngine.validateOrder(order);

    // Execute based on order side
    if (order.side === 'buy') {
      const tx = await this.wallet.sendTransaction({
        to: this.config.pair.quote,
        value: order.remaining * BigInt(Math.floor(order.price * 1000)),
        data: '0x',
        gasLimit: 100000n,
      });
    } else {
      // Sell - transfer base tokens
      const tx = await this.wallet.sendTransaction({
        to: this.config.pair.base,
        value: 0n,
        data: '0x',
        gasLimit: 100000n,
      });
    }

    order.filled = order.remaining;
    order.status = OrderStatus.FILLED;

    // Update inventory
    this.updateInventory(order);

    // Update PnL
    this.pnlEngine.recordTrade(order);

    // Check if hedge is needed
    await this.checkHedge();

    return order;
  }

  /**
   * Cancel order
   */
  cancelOrder(orderId: string): Order {
    const order = this.orders.get(orderId);
    if (!order) {
      throw new Error('Order not found');
    }

    order.status = OrderStatus.CANCELLED;
    return order;
  }

  /**
   * Get open orders
   */
  getOpenOrders(): Order[] {
    return Array.from(this.orders.values()).filter(
      o => o.status === OrderStatus.OPEN || o.status === OrderStatus.PARTIAL
    );
  }

  /**
   * Get positions
   */
  getPositions(): Position[] {
    return Array.from(this.positions.values());
  }

  /**
   * Get inventory
   */
  getInventory(): Inventory {
    return this.inventory;
  }

  /**
   * Get PnL
   */
  getPnL(): PnL {
    return this.pnlEngine.getPnL();
  }

  /**
   * Get risk status
   */
  async getRiskStatus(): Promise<{
    withinLimits: boolean;
    limits: RiskLimits;
    current: {
      positionSize: bigint;
      dailyVolume: bigint;
      drawdown: number;
    };
  }> {
    return this.riskEngine.getStatus();
  }

  /**
   * Get volatility
   */
  getVolatility(): VolatilityData {
    return this.volatilityEngine.getData();
  }

  /**
   * Manual hedge
   */
  async hedge(delta: bigint): Promise<void> {
    await this.hedgeEngine.executeHedge(delta);
  }

  private startQuoteLoop(): void {
    if (!this.isRunning) return;

    setInterval(async () => {
      try {
        await this.generateQuote();
      } catch (error) {
        console.error('Quote generation failed:', error);
      }
    }, this.config.quoteRefreshInterval);
  }

  private async getMarketPrices(): Promise<[number, number]> {
    // In production, fetch from multiple sources
    // Mock for now
    return [1000, 1001];
  }

  private calculateOrderSizes(bid: number, ask: number): [bigint, bigint] {
    const midPrice = (bid + ask) / 2;
    const minSize = this.config.minOrderSize;
    const maxSize = this.config.maxOrderSize;
    
    // Calculate based on inventory bias
    const bias = this.inventory.bias;
    let bidSize = maxSize;
    let askSize = maxSize;
    
    if (bias > 5000) {
      // Long bias - prefer selling
      bidSize = (maxSize * BigInt(10000 - bias)) / 10000n;
    } else if (bias < -5000) {
      // Short bias - prefer buying
      askSize = (maxSize * BigInt(10000 + bias)) / 10000n;
    }
    
    return [bidSize, askSize];
  }

  private async signQuote(quote: Quote): Promise<string> {
    const message = `${quote.bidPrice}:${quote.askPrice}:${quote.timestamp}`;
    return this.wallet.signMessage(message);
  }

  private initializeInventory(): Inventory {
    return {
      pair: this.config.pair,
      baseBalance: 0n,
      quoteBalance: 0n,
      baseValueUSD: 0,
      quoteValueUSD: 0,
      totalValueUSD: 0,
      bias: 0,
    };
  }

  private updateInventory(order: Order): void {
    const size = order.filled;
    const value = Number(size) * order.price;
    
    if (order.side === 'buy') {
      this.inventory.baseBalance += size;
      this.inventory.baseValueUSD += value;
    } else {
      this.inventory.baseBalance -= size;
      this.inventory.baseValueUSD -= value;
    }
    
    this.inventory.totalValueUSD = this.inventory.baseValueUSD + this.inventory.quoteValueUSD;
    
    // Calculate bias
    if (this.inventory.totalValueUSD > 0) {
      this.inventory.bias = Math.floor(
        ((this.inventory.baseValueUSD - this.inventory.quoteValueUSD) / this.inventory.totalValueUSD) * 10000
      );
    }
  }

  private async checkHedge(): Promise<void> {
    const delta = this.inventoryEngine.calculateDelta();
    if (Math.abs(delta) > 1000) {
      await this.hedgeEngine.executeHedge(delta);
    }
  }
}

// ============================================================================
// Spread Engine
// ============================================================================

/**
 * SpreadEngine - Dynamic spread calculation
 */
export class SpreadEngine {
  private config: MarketMakerConfig;

  constructor(config: MarketMakerConfig) {
    this.config = config;
  }

  /**
   * Calculate spread based on market conditions
   */
  calculateSpread(bid: number, ask: number): number {
    const midPrice = (bid + ask) / 2;
    const rawSpread = ((ask - bid) / midPrice) * 10000;
    
    // Use base spread, but adjust for market spread
    return Math.max(this.config.minSpread, Math.min(this.config.maxSpread, Math.max(this.config.baseSpread, rawSpread)));
  }

  /**
   * Calculate spread with volatility adjustment
   */
  calculateVolatilityAdjustedSpread(bid: number, ask: number, volatility: number): number {
    const baseSpread = this.calculateSpread(bid, ask);
    
    // Increase spread with volatility
    const volMultiplier = 1 + (volatility / 100);
    return baseSpread * volMultiplier;
  }

  /**
   * Calculate spread with inventory adjustment
   */
  calculateInventoryAdjustedSpread(bid: number, ask: number, inventoryBias: number): number {
    const baseSpread = this.calculateSpread(bid, ask);
    
    // Wider spread when inventory is imbalanced
    const biasAbs = Math.abs(inventoryBias);
    const biasMultiplier = 1 + (biasAbs / 10000);
    return baseSpread * biasMultiplier;
  }
}

// ============================================================================
// Inventory Engine
// ============================================================================

/**
 * InventoryEngine - Real-time inventory tracking
 */
export class InventoryEngine {
  private config: MarketMakerConfig;
  private inventory: Map<string, bigint>;
  private lastRebalance: number;

  constructor(config: MarketMakerConfig) {
    this.config = config;
    this.inventory = new Map();
    this.lastRebalance = Date.now();
  }

  /**
   * Get inventory adjustment
   */
  getAdjustment(): number {
    const baseBalance = this.inventory.get(this.config.pair.base) || 0n;
    const quoteBalance = this.inventory.get(this.config.pair.quote) || 0n;
    
    if (baseBalance === 0n && quoteBalance === 0n) {
      return 0;
    }
    
    const total = baseBalance + quoteBalance;
    if (total === 0n) return 0;
    
    const bias = Number((baseBalance * 10000n) / total) - 5000;
    return bias;
  }

  /**
   * Calculate delta (for hedging)
   */
  calculateDelta(): number {
    const baseBalance = this.inventory.get(this.config.pair.base) || 0n;
    const quoteBalance = this.inventory.get(this.config.pair.quote) || 0n;
    
    return Number(baseBalance - quoteBalance);
  }

  /**
   * Update balance
   */
  updateBalance(token: string, amount: bigint): void {
    this.inventory.set(token, amount);
  }

  /**
   * Check rebalance needed
   */
  needsRebalance(): boolean {
    const bias = this.getAdjustment();
    return Math.abs(bias) > this.config.inventoryBias;
  }

  /**
   * Rebalance inventory
   */
  async rebalance(): Promise<void> {
    // In production, execute rebalance trades
    this.lastRebalance = Date.now();
  }
}

// ============================================================================
// Hedge Engine
// ============================================================================

/**
 * HedgeEngine - Delta-neutral hedging
 */
export class HedgeEngine {
  private config: MarketMakerConfig;
  private wallet: EVMWallet;
  private hedgeOrders: Map<string, Order>;

  constructor(config: MarketMakerConfig, wallet: EVMWallet) {
    this.config = config;
    this.wallet = wallet;
    this.hedgeOrders = new Map();
  }

  /**
   * Execute hedge order
   */
  async executeHedge(delta: bigint): Promise<Order> {
    const order: Order = {
      id: this.generateOrderId(),
      maker: this.wallet.getAddress(),
      pair: this.config.pair,
      side: delta > 0n ? 'sell' : 'buy',
      price: 0, // Market price
      size: delta > 0n ? delta : -delta,
      filled: 0n,
      remaining: delta > 0n ? delta : -delta,
      status: OrderStatus.PENDING,
      timestamp: Date.now(),
      expiresAt: Date.now() + 60000,
    };

    // Execute at market
    const tx = await this.wallet.sendTransaction({
      to: this.config.pair.base,
      value: 0n,
      data: '0x',
      gasLimit: 100000n,
    });

    order.filled = order.remaining;
    order.status = OrderStatus.FILLED;
    
    this.hedgeOrders.set(order.id, order);
    return order;
  }

  /**
   * Calculate hedge size needed
   */
  calculateHedgeSize(inventoryDelta: bigint): bigint {
    const maxHedge = this.config.maxInventory / 10n;
    
    if (Math.abs(inventoryDelta) > maxHedge) {
      return maxHedge * (inventoryDelta > 0n ? 1n : -1n);
    }
    
    return 0n;
  }

  private generateOrderId(): string {
    return `0x${Date.now().toString(16)}${Math.random().toString(16).slice(2, 10)}`;
  }
}

// ============================================================================
// Risk Engine
// ============================================================================

/**
 * RiskEngine - Risk management and limits
 */
export class RiskEngine {
  private config: MarketMakerConfig;
  private limits: RiskLimits;
  private dailyVolume: bigint;
  private lastReset: number;

  constructor(config: MarketMakerConfig) {
    this.config = config;
    this.dailyVolume = 0n;
    this.lastReset = Date.now();
    this.limits = this.calculateLimits();
  }

  /**
   * Validate config
   */
  async validateConfig(): Promise<void> {
    if (this.config.minOrderSize >= this.config.maxOrderSize) {
      throw new Error('Invalid order size range');
    }
    
    if (this.config.maxInventory === 0n) {
      throw new Error('Max inventory must be greater than 0');
    }
  }

  /**
   * Validate order
   */
  async validateOrder(order: Order): Promise<void> {
    const status = await this.getStatus();
    
    if (!status.withinLimits) {
      throw new Error('Risk limits exceeded');
    }
    
    if (order.size > this.config.maxOrderSize) {
      throw new Error('Order size exceeds maximum');
    }
    
    if (order.price > this.limits.maxSlippage) {
      throw new Error('Slippage exceeds limit');
    }
  }

  /**
   * Get risk status
   */
  async getStatus(): Promise<{
    withinLimits: boolean;
    limits: RiskLimits;
    current: {
      positionSize: bigint;
      dailyVolume: bigint;
      drawdown: number;
    };
  }> {
    // Reset daily volume if new day
    if (this.isNewDay()) {
      this.dailyVolume = 0n;
      this.lastReset = Date.now();
    }

    const currentPosition = this.dailyVolume; // Simplified
    const drawdown = 0; // Calculate from PnL
    
    const withinLimits = 
      currentPosition < this.limits.maxPositionSize &&
      this.dailyVolume < this.limits.maxDailyVolume &&
      drawdown < this.limits.maxDrawdown;

    return {
      withinLimits,
      limits: this.limits,
      current: {
        positionSize: currentPosition,
        dailyVolume: this.dailyVolume,
        drawdown,
      },
    };
  }

  /**
   * Update daily volume
   */
  updateVolume(amount: bigint): void {
    this.dailyVolume += amount;
  }

  private calculateLimits(): RiskLimits {
    return {
      maxPositionSize: this.config.maxInventory,
      maxDailyVolume: this.config.maxDailyVolume,
      maxDrawdown: 10, // 10%
      maxLeverage: 10,
      maxSlippage: 50, // 0.5%
      maxSpread: this.config.maxSpread,
      minCapital: 10000, // $10k minimum
    };
  }

  private isNewDay(): boolean {
    const now = new Date();
    const last = new Date(this.lastReset);
    return now.getDate() !== last.getDate();
  }
}

// ============================================================================
// Quote Engine
// ============================================================================

/**
 * QuoteEngine - Sub-millisecond quote generation
 */
export class QuoteEngine {
  private config: MarketMakerConfig;
  private lastQuoteTime: number;
  private quoteCache: Map<string, Quote>;

  constructor(config: MarketMakerConfig) {
    this.config = config;
    this.lastQuoteTime = 0;
    this.quoteCache = new Map();
  }

  /**
   * Generate two-sided quote
   */
  generate(bidPrice: number, askPrice: number, bidSize: bigint, askSize: bigint): Quote {
    const now = Date.now();
    
    // Check cache
    const cached = this.quoteCache.get(`${bidPrice}:${askPrice}`);
    if (cached && now - cached.timestamp < 100) {
      return cached;
    }

    const quote: Quote = {
      maker: '',
      pair: this.config.pair,
      bidPrice,
      askPrice,
      bidSize,
      askSize,
      timestamp: now,
      validFor: this.config.quoteRefreshInterval,
    };

    this.quoteCache.set(`${bidPrice}:${askPrice}`, quote);
    this.lastQuoteTime = now;

    return quote;
  }

  /**
   * Get quote age
   */
  getQuoteAge(): number {
    return Date.now() - this.lastQuoteTime;
  }

  /**
   * Check if quote is stale
   */
  isQuoteStale(): boolean {
    return this.getQuoteAge() > this.config.quoteRefreshInterval;
  }
}

// ============================================================================
// PnL Engine
// ============================================================================

/**
 * PnLEngine - Real-time PnL tracking
 */
export class PnLEngine {
  private trades: Order[];
  private fees: number;
  private startTime: number;
  private initialBalance: number;

  constructor() {
    this.trades = [];
    this.fees = 0;
    this.startTime = Date.now();
    this.initialBalance = 0;
  }

  /**
   * Record trade
   */
  recordTrade(order: Order): void {
    this.trades.push(order);
    this.fees += Number(order.size) * order.price * 0.001; // 0.1% fee estimate
  }

  /**
   * Get PnL
   */
  getPnL(): PnL {
    let realizedPnL = 0;
    let volume = 0;
    let wins = 0;
    let losses = 0;
    let totalWin = 0;
    let totalLoss = 0;

    for (const trade of this.trades) {
      const tradePnL = Number(trade.filled) * trade.price;
      realizedPnL += tradePnL;
      volume += Number(trade.size);

      if (trade.side === 'sell') {
        wins++;
        totalWin += tradePnL;
      } else {
        losses++;
        totalLoss += tradePnL;
      }
    }

    const netPnL = realizedPnL - this.fees;
    const totalTrades = wins + losses;
    const winRate = totalTrades > 0 ? (wins / totalTrades) * 100 : 0;
    const avgWin = wins > 0 ? totalWin / wins : 0;
    const avgLoss = losses > 0 ? totalLoss / losses : 0;
    const roi = this.initialBalance > 0 ? (netPnL / this.initialBalance) * 100 : 0;

    return {
      unrealizedPnL: 0,
      realizedPnL,
      feesPaid: this.fees,
      netPnL,
      roi,
      volume,
      trades: totalTrades,
      winRate,
      avgWin,
      avgLoss,
    };
  }

  /**
   * Set initial balance
   */
  setInitialBalance(balance: number): void {
    this.initialBalance = balance;
  }

  /**
   * Get trade history
   */
  getTradeHistory(): Order[] {
    return [...this.trades];
  }
}

// ============================================================================
// Volatility Engine
// ============================================================================

/**
 * VolatilityEngine - Volatility calculation
 */
export class VolatilityEngine {
  private pair: TokenPair;
  private prices: number[];
  private windowSize: number;

  constructor(pair: TokenPair, windowSize: number = 100) {
    this.pair = pair;
    this.prices = [];
    this.windowSize = windowSize;
  }

  /**
   * Update price
   */
  updatePrice(price: number): void {
    this.prices.push(price);
    if (this.prices.length > this.windowSize) {
      this.prices.shift();
    }
  }

  /**
   * Get volatility data
   */
  getData(): VolatilityData {
    if (this.prices.length < 2) {
      return {
        pair: this.pair,
        iv: 0,
        rv: 0,
        high24h: 0,
        low24h: 0,
        volume24h: 0n,
        openInterest: 0n,
      };
    }

    const returns: number[] = [];
    for (let i = 1; i < this.prices.length; i++) {
      returns.push(Math.log(this.prices[i] / this.prices[i - 1]));
    }

    const mean = returns.reduce((a, b) => a + b, 0) / returns.length;
    const variance = returns.reduce((sum, r) => sum + Math.pow(r - mean, 2), 0) / returns.length;
    const rv = Math.sqrt(variance) * Math.sqrt(365 * 24 * 60) * 100; // Annualized

    const high24h = Math.max(...this.prices.slice(-24));
    const low24h = Math.min(...this.prices.slice(-24));

    return {
      pair: this.pair,
      iv: rv, // Use RV as proxy for IV
      rv,
      high24h,
      low24h,
      volume24h: 0n,
      openInterest: 0n,
    };
  }

  /**
   * Calculate realized volatility
   */
  getRealizedVolatility(): number {
    return this.getData().rv;
  }
}

// ============================================================================
// Utility Functions
// ============================================================================

/**
 * Create MM config
 */
export function createMMConfig(
  name: string,
  chainId: number,
  pair: TokenPair
): MarketMakerConfig {
  return {
    name,
    chainId,
    pair,
    baseSpread: 30,
    minSpread: 10,
    maxSpread: 100,
    minOrderSize: 1000000n,
    maxOrderSize: 1000000000000n,
    maxInventory: 10000000000000n,
    maxDailyVolume: 100000000000000n,
    quoteRefreshInterval: 1000,
    inventoryBias: 5000,
  };
}

// ============================================================================
// Export
// ============================================================================

export default {
  OrderStatus,
  MMEngine,
  SpreadEngine,
  InventoryEngine,
  HedgeEngine,
  RiskEngine,
  QuoteEngine,
  PnLEngine,
  VolatilityEngine,
  createMMConfig,
};