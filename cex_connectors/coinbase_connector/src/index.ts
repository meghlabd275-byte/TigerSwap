/**
 * TigerSwap CEX Connectors - Coinbase Connector
 * 
 * Native Coinbase exchange connector.
 * Zero external dependencies - fully native implementation.
 * 
 * Features:
 * - Spot trading
 * - Coinbase Advanced Trade
 * - Prime
 * - Wallet
 * 
 * @author TigerSwap
 * @version 1.0.0
 */

export interface CoinbaseConfig {
  apiKey: string;
  apiSecret: string;
  passphrase: string;
  sandbox: boolean;
}

export interface CoinbaseOrder {
  orderId: string;
  productId: string;
  side: 'BUY' | 'SELL';
  orderType: 'market' | 'limit';
  price?: string;
  size: string;
  status: 'open' | 'filled' | 'cancelled' | 'pending';
  timeInForce: string;
}

export interface CoinbasePosition {
  productId: string;
  side: 'B' | 'S';
  size: string;
  avg_entry_price: string;
  mark_price: string;
  cost: string;
  open: boolean;
}

export class CoinbaseConnector {
  private config: CoinbaseConfig;
  private baseUrl: string;

  constructor(config: CoinbaseConfig) {
    this.config = config;
    this.baseUrl = config.sandbox 
      ? 'https://api-public.sandbox.pro.coinbase.com'
      : 'https://api.exchange.coinbase.com';
  }

  /**
   * Get account balance
   */
  async getBalance(accounts?: string[]): Promise<any[]> {
    const endpoint = '/accounts';
    return this.getPrivate(endpoint);
  }

  /**
   * Place order
   */
  async placeOrder(order: {
    productId: string;
    side: 'BUY' | 'SELL';
    orderType: 'market' | 'limit';
    size?: string;
    price?: string;
    funds?: string;
  }): Promise<CoinbaseOrder> {
    const endpoint = '/orders';
    return this.postPrivate(endpoint, order);
  }

  /**
   * Cancel order
   */
  async cancelOrder(orderId: string): Promise<{ id: string; status: string }> {
    const endpoint = `/orders/${orderId}`;
    return this.deletePrivate(endpoint);
  }

  /**
   * Get orders
   */
  async getOrders(status?: 'open' | 'filled' | 'all'): Promise<CoinbaseOrder[]> {
    const endpoint = `/orders?status=${status || 'all'}`;
    return this.getPrivate(endpoint);
  }

  /**
   * Get fills
   */
  async getFills(orderId?: string, productId?: string): Promise<any[]> {
    let endpoint = '/fills';
    const params = [];
    if (orderId) params.push(`order_id=${orderId}`);
    if (productId) params.push(`product_id=${productId}`);
    if (params.length) endpoint += '?' + params.join('&');
    return this.getPrivate(endpoint);
  }

  /**
   * Get positions (Advanced Trade)
   */
  async getPositions(): Promise<CoinbasePosition[]> {
    const endpoint = '/positions';
    return this.getPrivate(endpoint);
  }

  /**
   * Get products
   */
  async getProducts(): Promise<any[]> {
    const endpoint = '/products';
    return this.getPublic(endpoint);
  }

  /**
   * Get product ticker
   */
  async getTicker(productId: string): Promise<any> {
    const endpoint = `/products/${productId}/ticker`;
    return this.getPublic(endpoint);
  }

  /**
   * Get orderbook
   */
  async getOrderBook(productId: string, level: number = 2): Promise<any> {
    const endpoint = `/products/${productId}/book?level=${level}`;
    return this.getPublic(endpoint);
  }

  private async getPrivate(endpoint: string): Promise<any> {
    const timestamp = Date.now().toString();
    const signature = this.sign('GET', endpoint, timestamp, '');
    
    const response = await fetch(`${this.baseUrl}${endpoint}`, {
      headers: {
        'CB-ACCESS-KEY': this.config.apiKey,
        'CB-ACCESS-SIGN': signature,
        'CB-ACCESS-TIMESTAMP': timestamp,
        'CB-ACCESS-PASSPHRASE': this.config.passphrase,
      },
    });
    
    return response.json();
  }

  private async postPrivate(endpoint: string, data: any): Promise<any> {
    const timestamp = Date.now().toString();
    const body = JSON.stringify(data);
    const signature = this.sign('POST', endpoint, timestamp, body);
    
    const response = await fetch(`${this.baseUrl}${endpoint}`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'CB-ACCESS-KEY': this.config.apiKey,
        'CB-ACCESS-SIGN': signature,
        'CB-ACCESS-TIMESTAMP': timestamp,
        'CB-ACCESS-PASSPHRASE': this.config.passphrase,
      },
      body,
    });
    
    return response.json();
  }

  private async deletePrivate(endpoint: string): Promise<any> {
    const timestamp = Date.now().toString();
    const signature = this.sign('DELETE', endpoint, timestamp, '');
    
    const response = await fetch(`${this.baseUrl}${endpoint}`, {
      method: 'DELETE',
      headers: {
        'CB-ACCESS-KEY': this.config.apiKey,
        'CB-ACCESS-SIGN': signature,
        'CB-ACCESS-TIMESTAMP': timestamp,
        'CB-ACCESS-PASSPHRASE': this.config.passphrase,
      },
    });
    
    return response.json();
  }

  private async getPublic(endpoint: string): Promise<any> {
    const response = await fetch(`${this.baseUrl}${endpoint}`);
    return response.json();
  }

  private sign(method: string, endpoint: string, timestamp: string, body: string): string {
    const crypto = require('crypto');
    const message = timestamp + method + endpoint + body;
    return crypto.createHmac('sha256', this.config.apiSecret).update(message).digest('base64');
  }
}

export default CoinbaseConnector;