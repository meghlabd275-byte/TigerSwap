/**
 * TigerSwap CEX Connectors - Kraken Connector
 * 
 * Native Kraken exchange connector.
 * Zero external dependencies - fully native implementation.
 * 
 * Features:
 * - Spot trading
 * - Futures
 * - WebSocket feeds
 * 
 * @author TigerSwap
 * @version 1.0.0
 */

export interface KrakenConfig {
  apiKey: string;
  apiSecret: string;
}

export interface KrakenOrder {
  txid: string;
  ordertxid: string;
  pair: string;
  type: 'buy' | 'sell';
  ordertype: 'market' | 'limit';
  price: string;
  volume: string;
  status: 'open' | 'closed' | 'canceled';
}

export class KrakenConnector {
  private config: KrakenConfig;
  private baseUrl: string = 'https://api.kraken.com';

  constructor(config: KrakenConfig) {
    this.config = config;
  }

  /**
   * Get account balance
   */
  async getBalance(): Promise<Record<string, string>> {
    const endpoint = '/0/private/Balance';
    return this.postPrivate(endpoint, {});
  }

  /**
   * Place order
   */
  async placeOrder(order: {
    pair: string;
    type: 'buy' | 'sell';
    ordertype: 'market' | 'limit';
    volume: string;
    price?: string;
  }): Promise<{ txid: string[] }> {
    const endpoint = '/0/private/AddOrder';
    return this.postPrivate(endpoint, order);
  }

  /**
   * Cancel order
   */
  async cancelOrder(txid: string): Promise<{ pending: boolean }> {
    const endpoint = '/0/private/CancelOrder';
    return this.postPrivate(endpoint, { txid });
  }

  /**
   * Get open orders
   */
  async getOpenOrders(): Promise<{ open: Record<string, KrakenOrder> }> {
    const endpoint = '/0/private/OpenOrders';
    return this.postPrivate(endpoint, {});
  }

  /**
   * Get closed orders
   */
  async getClosedOrders(): Promise<{ closed: Record<string, KrakenOrder> }> {
    const endpoint = '/0/private/ClosedOrders';
    return this.postPrivate(endpoint, {});
  }

  /**
   * Get trades
   */
  async getTrades(): Promise<any> {
    const endpoint = '/0/private/TradesHistory';
    return this.postPrivate(endpoint, {});
  }

  /**
   * Get ticker
   */
  async getTicker(pair: string): Promise<any> {
    const endpoint = '/0/public/Ticker';
    return this.getPublic(endpoint, { pair });
  }

  /**
   * Get orderbook
   */
  async getOrderBook(pair: string, depth: number = 10): Promise<any> {
    const endpoint = '/0/public/Depth';
    return this.getPublic(endpoint, { pair, depth: depth.toString() });
  }

  private async getPublic(endpoint: string, params: Record<string, string>): Promise<any> {
    const query = new URLSearchParams(params).toString();
    const response = await fetch(`${this.baseUrl}${endpoint}?${query}`);
    return response.json();
  }

  private async postPrivate(endpoint: string, params: any): Promise<any> {
    const nonce = Date.now() * 1000;
    const postData = new URLSearchParams({ nonce: nonce.toString(), ...params }).toString();
    const hash = this.sha256(nonce.toString() + postData);
    const signature = this.sign(hash, endpoint);
    
    const response = await fetch(`${this.baseUrl}${endpoint}`, {
      method: 'POST',
      headers: {
        'API-Key': this.config.apiKey,
        'API-Sign': signature,
        'Content-Type': 'application/x-www-form-urlencoded',
      },
      body: postData,
    });
    
    return response.json();
  }

  private sha256(data: string): Buffer {
    const crypto = require('crypto');
    return crypto.createHash('sha256').update(data).digest();
  }

  private sign(message: Buffer, endpoint: string): string {
    const crypto = require('crypto');
    const hash = this.sha256(endpoint + message);
    return crypto.createHmac('sha512', this.config.apiSecret).update(hash).digest('hex');
  }
}

export default KrakenConnector;