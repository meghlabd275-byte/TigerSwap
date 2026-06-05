/**
 * TigerSwap Order Book / CLOB - Complete Native Implementation
 * Built from scratch - no dependencies on dYdX, Hyperliquid or other CLOB
 */

export interface Order {
  id: string;
  side: 'bid' | 'ask';
  price: bigint;
  quantity: bigint;
  filled: bigint;
  owner: string;
  timestamp: number;
  expiresAt: number;
  orderType: 'limit' | 'market' | 'stop' | 'stop_limit';
  status: 'pending' | 'partial' | 'filled' | 'cancelled' | 'expired';
  metadata?: Record<string, any>;
}

export interface Trade {
  id: string;
  price: bigint;
  quantity: bigint;
  side: 'buy' | 'sell';
  makerOrderId: string;
  takerOrderId: string;
  maker: string;
  taker: string;
  timestamp: number;
  fee: bigint;
  feeToken: string;
}

export interface OrderBookState {
  bids: PriceLevel[];
  asks: PriceLevel[];
  lastTradePrice: bigint;
  spread: bigint;
  depth: { bids: bigint; asks: bigint };
}

export interface PriceLevel {
  price: bigint;
  quantity: bigint;
  orders: number;
}

export interface MatchResult {
  trades: Trade[];
  matchedQuantity: bigint;
  remainingQuantity: bigint;
  price: bigint;
}

export class PriceComparator {
  static compare(a: bigint, b: bigint): number {
    if (a < b) return -1;
    if (a > b) return 1;
    return 0;
  }

  static isBidBetter(price1: bigint, price2: bigint): boolean {
    return price1 > price2;
  }

  static isAskBetter(price1: bigint, price2: bigint): boolean {
    return price1 < price2;
  }
}

export class OrderBook {
  private bids: Map<string, Order> = new Map();
  private asks: Map<string, Order> = new Map();
  private bidPrices: Map<string, bigint> = new Map(); // price key -> order id
  private askPrices: Map<string, bigint> = new Map();
  private trades: Trade[] = [];
  private orderCounter: number = 0;
  private feeRate: bigint = BigInt(10); // 0.1% = 10 bps

  constructor(private minOrderSize: bigint = BigInt(1), private tickSize: bigint = BigInt(1)) {}

  /**
   * Add an order to the book
   */
  addOrder(order: Omit<Order, 'id' | 'filled' | 'status'>): Order {
    const id = this.generateOrderId(order.owner);
    const newOrder: Order = {
      ...order,
      id,
      filled: BigInt(0),
      status: 'pending',
    };

    if (order.side === 'bid') {
      this.bids.set(id, newOrder);
      const priceKey = order.price.toString();
      this.bidPrices.set(priceKey, id);
    } else {
      this.asks.set(id, newOrder);
      const priceKey = order.price.toString();
      this.askPrices.set(priceKey, id);
    }

    return newOrder;
  }

  /**
   * Cancel an order
   */
  cancelOrder(orderId: string, owner: string): boolean {
    const bidOrder = this.bids.get(orderId);
    const askOrder = this.asks.get(orderId);
    const order = bidOrder || askOrder;

    if (!order || order.owner !== owner) {
      return false;
    }

    if (order.status === 'filled') {
      return false;
    }

    order.status = 'cancelled';

    if (order.side === 'bid') {
      this.bids.delete(orderId);
    } else {
      this.asks.delete(orderId);
    }

    return true;
  }

  /**
   * Modify an order
   */
  modifyOrder(orderId: string, owner: string, newPrice?: bigint, newQuantity?: bigint): Order | null {
    const bidOrder = this.bids.get(orderId);
    const askOrder = this.asks.get(orderId);
    const order = bidOrder || askOrder;

    if (!order || order.owner !== owner) {
      return null;
    }

    if (order.status === 'filled' || order.status === 'cancelled') {
      return null;
    }

    // If changing price, remove and re-add
    if (newPrice !== undefined && newPrice !== order.price) {
      this.cancelOrder(orderId, owner);
      return this.addOrder({
        ...order,
        price: newPrice,
        quantity: newQuantity !== undefined ? newQuantity : order.quantity,
        timestamp: Date.now(),
        expiresAt: order.expiresAt,
      });
    }

    // Just update quantity
    if (newQuantity !== undefined) {
      order.quantity = newQuantity;
    }

    return order;
  }

  /**
   * Match orders (bid/ask crossing)
   */
  matchOrders(bidOrderId: string, askOrderId: string, quantity: bigint): MatchResult {
    const bidOrder = this.bids.get(bidOrderId);
    const askOrder = this.asks.get(askOrderId);

    if (!bidOrder || !askOrder) {
      throw new Error('Order not found');
    }

    // Determine match price (limit price of the older order)
    const price = bidOrder.timestamp < askOrder.timestamp 
      ? bidOrder.price 
      : askOrder.price;

    const trade: Trade = {
      id: this.generateTradeId(),
      price,
      quantity,
      side: 'buy',
      makerOrderId: bidOrder.timestamp < askOrder.timestamp ? bidOrderId : askOrderId,
      takerOrderId: bidOrder.timestamp < askOrder.timestamp ? askOrderId : bidOrderId,
      maker: bidOrder.timestamp < askOrder.timestamp ? bidOrder.owner : askOrder.owner,
      taker: bidOrder.timestamp < askOrder.timestamp ? askOrder.owner : bidOrder.owner,
      timestamp: Date.now(),
      fee: (quantity * price * this.feeRate) / BigInt(1000000),
      feeToken: 'native',
    };

    this.trades.push(trade);

    // Update orders
    bidOrder.filled += quantity;
    askOrder.filled += quantity;

    if (bidOrder.filled >= bidOrder.quantity) {
      bidOrder.status = 'filled';
      this.bids.delete(bidOrderId);
    } else {
      bidOrder.status = 'partial';
    }

    if (askOrder.filled >= askOrder.quantity) {
      askOrder.status = 'filled';
      this.asks.delete(askOrderId);
    } else {
      askOrder.status = 'partial';
    }

    return {
      trades: [trade],
      matchedQuantity: quantity,
      remainingQuantity: BigInt(0),
      price,
    };
  }

  /**
   * Get best bid and ask
   */
  getBestPrices(): { bestBid: bigint | null; bestAsk: bigint | null; spread: bigint } {
    let bestBid: bigint | null = null;
    let bestAsk: bigint | null = null;

    for (const order of this.bids.values()) {
      if (order.status === 'pending' || order.status === 'partial') {
        if (bestBid === null || order.price > bestBid) {
          bestBid = order.price;
        }
      }
    }

    for (const order of this.asks.values()) {
      if (order.status === 'pending' || order.status === 'partial') {
        if (bestAsk === null || order.price < bestAsk) {
          bestAsk = order.price;
        }
      }
    }

    const spread = (bestBid !== null && bestAsk !== null) 
      ? bestAsk - bestBid 
      : BigInt(0);

    return { bestBid, bestAsk, spread };
  }

  /**
   * Get order book state
   */
  getState(levels: number = 10): OrderBookState {
    const bidMap = new Map<string, PriceLevel>();
    const askMap = new Map<string, PriceLevel>();

    for (const order of this.bids.values()) {
      if (order.status !== 'pending' && order.status !== 'partial') continue;
      
      const priceKey = order.price.toString();
      const level = bidMap.get(priceKey) || {
        price: order.price,
        quantity: BigInt(0),
        orders: 0,
      };
      
      level.quantity += order.quantity - order.filled;
      level.orders++;
      bidMap.set(priceKey, level);
    }

    for (const order of this.asks.values()) {
      if (order.status !== 'pending' && order.status !== 'partial') continue;
      
      const priceKey = order.price.toString();
      const level = askMap.get(priceKey) || {
        price: order.price,
        quantity: BigInt(0),
        orders: 0,
      };
      
      level.quantity += order.quantity - order.filled;
      level.orders++;
      askMap.set(priceKey, level);
    }

    const bids = Array.from(bidMap.values())
      .sort((a, b) => PriceComparator.compare(b.price, a.price))
      .slice(0, levels);

    const asks = Array.from(askMap.values())
      .sort((a, b) => PriceComparator.compare(a.price, b.price))
      .slice(0, levels);

    const { bestBid, bestAsk } = this.getBestPrices();

    return {
      bids,
      asks,
      lastTradePrice: this.trades.length > 0 ? this.trades[this.trades.length - 1].price : BigInt(0),
      spread: bestAsk !== null && bestBid !== null ? bestAsk - bestBid : BigInt(0),
      depth: {
        bids: bids.reduce((sum, l) => sum + l.quantity, BigInt(0)),
        asks: asks.reduce((sum, l) => sum + l.quantity, BigInt(0)),
      },
    };
  }

  /**
   * Get orders for a specific owner
   */
  getOrdersByOwner(owner: string): Order[] {
    const result: Order[] = [];
    for (const order of this.bids.values()) {
      if (order.owner === owner) result.push(order);
    }
    for (const order of this.asks.values()) {
      if (order.owner === owner) result.push(order);
    }
    return result;
  }

  /**
   * Get recent trades
   */
  getRecentTrades(limit: number = 50): Trade[] {
    return this.trades.slice(-limit);
  }

  /**
   * Execute market order
   */
  executeMarketOrder(side: 'bid' | 'ask', quantity: bigint, owner: string): MatchResult {
    if (quantity < this.minOrderSize) {
      throw new Error('Order too small');
    }

    const trades: Trade[] = [];
    let remainingQty = quantity;
    const oppositeBook = side === 'bid' ? this.asks : this.bids;

    // Sort by best price
    const sortedOrders = Array.from(oppositeBook.values())
      .filter(o => o.status === 'pending' || o.status === 'partial')
      .sort((a, b) => {
        return side === 'bid' 
          ? PriceComparator.compare(a.price, b.price) 
          : PriceComparator.compare(b.price, a.price);
      });

    for (const order of sortedOrders) {
      if (remainingQty <= BigInt(0)) break;

      const available = order.quantity - order.filled;
      const matchQty = available < remainingQty ? available : remainingQty;
      const matchPrice = order.price;

      const trade: Trade = {
        id: this.generateTradeId(),
        price: matchPrice,
        quantity: matchQty,
        side: side === 'bid' ? 'buy' : 'sell',
        makerOrderId: order.id,
        takerOrderId: this.generateOrderId(owner),
        maker: order.owner,
        taker: owner,
        timestamp: Date.now(),
        fee: (matchQty * matchPrice * this.feeRate) / BigInt(1000000),
        feeToken: 'native',
      };

      trades.push(trade);
      order.filled += matchQty;

      if (order.filled >= order.quantity) {
        order.status = 'filled';
        oppositeBook.delete(order.id);
      } else {
        order.status = 'partial';
      }

      remainingQty -= matchQty;
    }

    return {
      trades,
      matchedQuantity: quantity - remainingQty,
      remainingQuantity: remainingQty,
      price: trades.length > 0 ? trades[trades.length - 1].price : BigInt(0),
    };
  }

  /**
   * Set fee rate
   */
  setFeeRate(feeBps: bigint): void {
    this.feeRate = feeBps;
  }

  private generateOrderId(owner: string): string {
    this.orderCounter++;
    return `${owner.slice(0, 10)}-${Date.now()}-${this.orderCounter}`;
  }

  private generateTradeId(): string {
    return `trade-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
  }
}

export default { OrderBook };