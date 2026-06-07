/**
 * TigerSwap MM Bot Platform - Grid Trading Strategy
 * 
 * Native grid trading strategy implementation.
 * Zero external dependencies - fully native implementation.
 * 
 * @author TigerSwap
 * @version 1.0.0
 */

export interface GridConfig {
  symbol: string;
  gridCount: number;
  gridSize: bigint;
  upperPrice: bigint;
  lowerPrice: bigint;
  positionSize: bigint;
  autoRebalance: boolean;
}

export interface GridOrder {
  id: number;
  side: 'buy' | 'sell';
  price: bigint;
  amount: bigint;
  filled: boolean;
}

export interface GridState {
  config: GridConfig;
  orders: GridOrder[];
  totalProfit: bigint;
  totalTrades: number;
  filledGrids: number;
}

export class GridStrategy {
  private config: GridConfig;
  private state: GridState;
  private gridPrices: bigint[];

  constructor(config: GridConfig) {
    this.config = config;
    this.gridPrices = this.calculateGridPrices();
    this.state = {
      config,
      orders: this.createGridOrders(),
      totalProfit: 0n,
      totalTrades: 0,
      filledGrids: 0,
    };
  }

  /**
   * Calculate grid prices
   */
  private calculateGridPrices(): bigint[] {
    const prices: bigint[] = [];
    const range = this.config.upperPrice - this.config.lowerPrice;
    const gridSize = range / BigInt(this.config.gridCount);

    for (let i = 0; i <= this.config.gridCount; i++) {
      prices.push(this.config.lowerPrice + gridSize * BigInt(i));
    }

    return prices;
  }

  /**
   * Create grid orders
   */
  private createGridOrders(): GridOrder[] {
    const orders: GridOrder[] = [];

    for (let i = 0; i < this.gridPrices.length - 1; i++) {
      // Buy order at lower price
      orders.push({
        id: i * 2,
        side: 'buy',
        price: this.gridPrices[i],
        amount: this.config.gridSize,
        filled: false,
      });

      // Sell order at upper price
      orders.push({
        id: i * 2 + 1,
        side: 'sell',
        price: this.gridPrices[i + 1],
        amount: this.config.gridSize,
        filled: false,
      });
    }

    return orders;
  }

  /**
   * Process price update
   */
  async processPriceUpdate(currentPrice: bigint): Promise<GridOrder | null> {
    // Find which grid we're in
    let gridIndex = -1;
    for (let i = 0; i < this.gridPrices.length - 1; i++) {
      if (currentPrice >= this.gridPrices[i] && currentPrice < this.gridPrices[i + 1]) {
        gridIndex = i;
        break;
      }
    }

    if (gridIndex === -1) {
      return null;
    }

    // Check if we should place an order
    const buyOrderId = gridIndex * 2;
    const sellOrderId = gridIndex * 2 + 1;

    const buyOrder = this.state.orders.find(o => o.id === buyOrderId);
    const sellOrder = this.state.orders.find(o => o.id === sellOrderId);

    // Place buy order if below mid-price and not filled
    if (currentPrice < (this.gridPrices[gridIndex] + this.gridPrices[gridIndex + 1]) / 2n) {
      if (buyOrder && !buyOrder.filled) {
        return buyOrder;
      }
    }

    // Place sell order if above mid-price and not filled
    if (currentPrice > (this.gridPrices[gridIndex] + this.gridPrices[gridIndex + 1]) / 2n) {
      if (sellOrder && !sellOrder.filled) {
        return sellOrder;
      }
    }

    return null;
  }

  /**
   * Execute order
   */
  async executeOrder(orderId: number): Promise<void> {
    const order = this.state.orders.find(o => o.id === orderId);
    if (!order) throw new Error('Order not found');

    // In production, execute actual trade
    order.filled = true;
    this.state.filledGrids++;

    // Calculate profit
    if (order.side === 'sell') {
      this.state.totalProfit += order.amount * order.price / 1000000000n;
    }

    this.state.totalTrades++;
  }

  /**
   * Rebalance grid
   */
  rebalance(newUpperPrice: bigint, newLowerPrice: bigint): void {
    this.config.upperPrice = newUpperPrice;
    this.config.lowerPrice = newLowerPrice;
    this.gridPrices = this.calculateGridPrices();
    this.state.orders = this.createGridOrders();
  }

  /**
   * Get state
   */
  getState(): GridState {
    return { ...this.state };
  }

  /**
   * Calculate profit
   */
  calculateProfit(): bigint {
    return this.state.totalProfit;
  }
}

export default GridStrategy;