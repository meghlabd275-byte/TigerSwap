/**
 * TigerSwap User Features - Perpetual Trading Module
 * 
 * Native perpetual futures with leverage, funding, and liquidation.
 * Zero external dependencies - fully native implementation.
 * 
 * Features:
 * - Long/Short positions
 * - Leverage (up to 100x)
 * - Funding payments
 * - Liquidation mechanism
 * - Funding rate oracle
 * 
 * @author TigerSwap
 * @version 1.0.0
 */

import { EVMClient, EVMWallet } from '@tigerswap/evm-sdk';

// ============================================================================
// Types & Interfaces
// ============================================================================

export interface PerpetualMarket {
  id: string;
  baseToken: string;
  quoteToken: string;
  priceFeed: string;
  initialMargin: number; // e.g., 1000 = 1%
  maintenanceMargin: number; // e.g., 500 = 0.5%
  maxLeverage: number;
  makerFee: number;
  takerFee: number;
  fundingRateCap: number;
  fundingPeriod: number; // seconds
}

export interface Position {
  id: string;
  trader: string;
  marketId: string;
  side: 'long' | 'short';
  size: bigint;
  entryPrice: bigint;
  leverage: number;
  margin: bigint;
  openNotional: bigint;
  unrealizedPnl: bigint;
  realizedPnl: bigint;
  fundingPayment: bigint;
  timestamp: number;
}

export interface Order {
  id: string;
  trader: string;
  marketId: string;
  side: 'long' | 'short';
  orderType: 'market' | 'limit';
  size: bigint;
  price?: bigint;
  triggerPrice?: bigint;
  margin: bigint;
  status: 'pending' | 'filled' | 'cancelled' | 'expired';
  timestamp: number;
}

export interface Liquidation {
  id: string;
  trader: string;
  marketId: string;
  side: 'long' | 'short';
  size: bigint;
  entryPrice: bigint;
  liquidationPrice: bigint;
  marginRemaining: bigint;
  timestamp: number;
}

export interface FundingRate {
  marketId: string;
  rate: number;
  timestamp: number;
  nextUpdate: number;
}

// ============================================================================
// Perpetual Trading Protocol
// ============================================================================

export class PerpetualTrading {
  private markets: Map<string, PerpetualMarket>;
  private positions: Map<string, Position>;
  private orders: Map<string, Order>;
  private liquidations: Liquidation[];
  private fundingRates: Map<string, FundingRate>;
  private priceOracles: Map<string, bigint>;

  constructor() {
    this.markets = new Map();
    this.positions = new Map();
    this.orders = new Map();
    this.liquidations = [];
    this.fundingRates = new Map();
    this.priceOracles = new Map();
  }

  /**
   * Create perpetual market
   */
  createMarket(
    baseToken: string,
    quoteToken: string,
    initialMargin: number,
    maintenanceMargin: number,
    maxLeverage: number,
    makerFee: number,
    takerFee: number
  ): PerpetualMarket {
    const market: PerpetualMarket = {
      id: `perp_${baseToken}_${Date.now()}`,
      baseToken,
      quoteToken,
      priceFeed: baseToken,
      initialMargin,
      maintenanceMargin,
      maxLeverage,
      makerFee,
      takerFee,
      fundingRateCap: 1000000, // 100% cap
      fundingPeriod: 28800, // 8 hours
    };

    this.markets.set(market.id, market);

    // Initialize funding rate
    this.fundingRates.set(market.id, {
      marketId: market.id,
      rate: 0,
      timestamp: Date.now(),
      nextUpdate: Date.now() + market.fundingPeriod * 1000,
    });

    return market;
  }

  /**
   * Open position
   */
  openPosition(
    trader: string,
    marketId: string,
    side: 'long' | 'short',
    size: bigint,
    leverage: number,
    margin: bigint
  ): Position {
    const market = this.markets.get(marketId);
    if (!market) throw new Error('Market not found');

    if (leverage > market.maxLeverage) {
      throw new Error('Leverage exceeds max');
    }

    const openNotional = margin * BigInt(leverage);
    const entryPrice = this.getPrice(market.baseToken);

    const position: Position = {
      id: `pos_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`,
      trader,
      marketId,
      side,
      size,
      entryPrice,
      leverage,
      margin,
      openNotional,
      unrealizedPnl: 0n,
      realizedPnl: 0n,
      fundingPayment: 0n,
      timestamp: Date.now(),
    };

    this.positions.set(position.id, position);

    return position;
  }

  /**
   * Close position
   */
  closePosition(positionId: string): { pnl: bigint; fees: bigint } {
    const position = this.positions.get(positionId);
    if (!position) throw new Error('Position not found');

    const market = this.markets.get(position.marketId);
    if (!market) throw new Error('Market not found');

    const currentPrice = this.getPrice(market.baseToken);

    // Calculate PnL
    let pnl: bigint;
    if (position.side === 'long') {
      pnl = (currentPrice - position.entryPrice) * position.size;
    } else {
      pnl = (position.entryPrice - currentPrice) * position.size;
    }

    // Calculate fees
    const fees = (position.openNotional * BigInt(market.takerFee)) / 1000000n;

    // Update position
    position.realizedPnl = pnl - fees;
    position.unrealizedPnl = pnl;

    return { pnl, fees };
  }

  /**
   * Add margin
   */
  addMargin(positionId: string, margin: bigint): void {
    const position = this.positions.get(positionId);
    if (!position) throw new Error('Position not found');

    position.margin += margin;
    position.openNotional = position.margin * BigInt(position.leverage);
  }

  /**
   * Remove margin
   */
  removeMargin(positionId: string, margin: bigint): void {
    const position = this.positions.get(positionId);
    if (!position) throw new Error('Position not found');

    const market = this.markets.get(position.marketId);
    if (!market) throw new Error('Market not found');

    // Check if margin will be too low
    const minMargin = (position.openNotional * BigInt(market.initialMargin)) / 1000000n;
    if (position.margin - margin < minMargin) {
      throw new Error('Margin too low');
    }

    position.margin -= margin;
    position.openNotional = position.margin * BigInt(position.leverage);
  }

  /**
   * Create order
   */
  createOrder(
    trader: string,
    marketId: string,
    side: 'long' | 'short',
    orderType: 'market' | 'limit',
    size: bigint,
    price?: bigint,
    triggerPrice?: bigint,
    margin: bigint
  ): Order {
    const market = this.markets.get(marketId);
    if (!market) throw new Error('Market not found');

    const order: Order = {
      id: `order_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`,
      trader,
      marketId,
      side,
      orderType,
      size,
      price,
      triggerPrice,
      margin,
      status: 'pending',
      timestamp: Date.now(),
    };

    this.orders.set(order.id, order);

    return order;
  }

  /**
   * Cancel order
   */
  cancelOrder(orderId: string): void {
    const order = this.orders.get(orderId);
    if (!order) throw new Error('Order not found');

    order.status = 'cancelled';
  }

  /**
   * Execute order
   */
  executeOrder(orderId: string): Position {
    const order = this.orders.get(orderId);
    if (!order) throw new Error('Order not found');

    if (order.status !== 'pending') {
      throw new Error('Order already processed');
    }

    const market = this.markets.get(order.marketId);
    if (!market) throw new Error('Market not found');

    // Get execution price
    let executionPrice: bigint;
    if (order.orderType === 'market') {
      executionPrice = this.getPrice(market.baseToken);
    } else {
      executionPrice = order.price || this.getPrice(market.baseToken);
    }

    // Create position
    const position = this.openPosition(
      order.trader,
      order.marketId,
      order.side,
      order.size,
      Number((order.openNotional || 0n) / order.margin),
      order.margin
    );

    position.entryPrice = executionPrice;
    order.status = 'filled';

    return position;
  }

  /**
   * Check liquidations
   */
  checkLiquidations(): Liquidation[] {
    const toLiquidate: Position[] = [];

    for (const position of this.positions.values()) {
      const market = this.markets.get(position.marketId);
      if (!market) continue;

      const currentPrice = this.getPrice(market.baseToken);
      const liqPrice = this.calculateLiquidationPrice(position, market);

      let shouldLiquidate = false;
      if (position.side === 'long' && currentPrice <= liqPrice) {
        shouldLiquidate = true;
      } else if (position.side === 'short' && currentPrice >= liqPrice) {
        shouldLiquidate = true;
      }

      if (shouldLiquidate) {
        toLiquidate.push(position);
      }
    }

    // Execute liquidations
    const results: Liquidation[] = [];
    for (const pos of toLiquidate) {
      const liquidation = this.liquidatePosition(pos);
      if (liquidation) {
        results.push(liquidation);
      }
    }

    return results;
  }

  /**
   * Liquidate position
   */
  private liquidatePosition(position: Position): Liquidation {
    const market = this.markets.get(position.marketId);
    if (!market) throw new Error('Market not found');

    const currentPrice = this.getPrice(market.baseToken);

    const liquidation: Liquidation = {
      id: `liq_${Date.now()}`,
      trader: position.trader,
      marketId: position.marketId,
      side: position.side,
      size: position.size,
      entryPrice: position.entryPrice,
      liquidationPrice: currentPrice,
      marginRemaining: 0n,
      timestamp: Date.now(),
    };

    this.liquidations.push(liquidation);

    // Remove position
    this.positions.delete(position.id);

    return liquidation;
  }

  /**
   * Calculate liquidation price
   */
  calculateLiquidationPrice(position: Position, market: PerpetualMarket): bigint {
    const maintenanceMargin = (position.openNotional * BigInt(market.maintenanceMargin)) / 1000000n;

    if (position.side === 'long') {
      const liqPrice = position.entryPrice - (maintenanceMargin * position.entryPrice) / position.margin;
      return liqPrice;
    } else {
      const liqPrice = position.entryPrice + (maintenanceMargin * position.entryPrice) / position.margin;
      return liqPrice;
    }
  }

  /**
   * Update funding rates
   */
  updateFundingRates(): void {
    for (const [marketId, fundingRate] of this.fundingRates) {
      const market = this.markets.get(marketId);
      if (!market) continue;

      if (Date.now() >= fundingRate.nextUpdate) {
        const newRate = this.calculateFundingRate(marketId);
        const cappedRate = Math.min(
          newRate,
          market.fundingRateCap
        );

        fundingRate.rate = cappedRate;
        fundingRate.timestamp = Date.now();
        fundingRate.nextUpdate = Date.now() + market.fundingPeriod * 1000;
      }
    }
  }

  /**
   * Calculate funding rate
   */
  calculateFundingRate(marketId: string): number {
    const market = this.markets.get(marketId);
    if (!market) return 0;

    let totalLongSize = 0n;
    let totalShortSize = 0n;

    for (const position of this.positions.values()) {
      if (position.marketId !== marketId) continue;

      if (position.side === 'long') {
        totalLongSize += position.size;
      } else {
        totalShortSize += position.size;
      }
    }

    if (totalLongSize === 0n || totalShortSize === 0n) {
      return 0;
    }

    const imbalance = Number(totalLongSize - totalShortSize) / Number(totalLongSize + totalShortSize);

    return Math.floor(imbalance * 100000); // 0.01% per funding period
  }

  /**
   * Get position
   */
  getPosition(positionId: string): Position | null {
    return this.positions.get(positionId) || null;
  }

  /**
   * Get positions for trader
   */
  getTraderPositions(trader: string): Position[] {
    return Array.from(this.positions.values()).filter(p => p.trader === trader);
  }

  /**
   * Get market
   */
  getMarket(marketId: string): PerpetualMarket | null {
    return this.markets.get(marketId) || null;
  }

  /**
   * Get all markets
   */
  getAllMarkets(): PerpetualMarket[] {
    return Array.from(this.markets.values());
  }

  /**
   * Set price oracle
   */
  setPrice(token: string, price: bigint): void {
    this.priceOracles.set(token, price);
  }

  /**
   * Get price
   */
  getPrice(token: string): bigint {
    return this.priceOracles.get(token) || 0n;
  }

  /**
   * Get unrealized PnL
   */
  getUnrealizedPnl(positionId: string): bigint {
    const position = this.positions.get(positionId);
    if (!position) return 0n;

    const market = this.markets.get(position.marketId);
    if (!market) return 0n;

    const currentPrice = this.getPrice(market.baseToken);

    let pnl: bigint;
    if (position.side === 'long') {
      pnl = (currentPrice - position.entryPrice) * position.size;
    } else {
      pnl = (position.entryPrice - currentPrice) * position.size;
    }

    return pnl;
  }

  /**
   * Get liquidation history
   */
  getLiquidations(limit: number = 100): Liquidation[] {
    return this.liquidations.slice(-limit);
  }
}

// ============================================================================
// Funding Calculator
// ============================================================================

export class FundingCalculator {
  /**
   * Calculate funding payment
   */
  static calculateFundingPayment(
    size: bigint,
    rate: number,
    timeDelta: number // seconds
  ): bigint {
    const timeFraction = timeDelta / 28800; // 8 hours
    const payment = (size * BigInt(Math.floor(rate * timeFraction))) / 1000000n;
    return payment;
  }

  /**
   * Calculate mark price
   */
  static calculateMarkPrice(
    indexPrice: bigint,
    fundingRate: number,
    timeSinceUpdate: number
  ): bigint {
    const funding = indexPrice * BigInt(Math.floor(fundingRate * timeSinceUpdate)) / 1000000n / 28800n;
    return indexPrice + funding;
  }
}

// ============================================================================
// Export
// ============================================================================

export default {
  PerpetualTrading,
  FundingCalculator,
};