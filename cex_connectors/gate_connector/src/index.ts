/**
 * TigerSwap CEX Connectors - Gate.io Connector
 * 
 * Native Gate.io exchange connector.
 * Zero external dependencies - fully native implementation.
 * 
 * Features:
 * - Spot trading
 * - Futures
 * - Delivery futures
 * - Options
 * 
 * @author TigerSwap
 * @version 1.0.0
 */

export interface GateConfig {
  apiKey: string;
  apiSecret: string;
}

export interface GateOrder {
  id: string;
  text: string;
  createTime: number;
  updateTime: number;
  side: 'buy' | 'sell';
  type: 'limit' | 'market';
  price: string;
  amount: string;
  account: string;
  status: 'open' | 'closed' | 'cancelled';
}

export class GateConnector {
  private config: GateConfig;
  private baseUrl: string = 'https://api.gateio.ws';

  constructor(config: GateConfig) {
    this.config = config;
  }

  /**
   * Get account balance
   */
  async getBalance(): Promise<any> {
    const endpoint = '/api/v4/spot/accounts';
    return this.getSigned(endpoint, {});
  }

  /**
   * Place order
   */
  async placeOrder(order: {
    currencyPair: string;
    side: 'buy' | 'sell';
    type: 'limit' | 'market';
    price?: string;
    amount: string;
  }): Promise<{ id: string }> {
    const endpoint = '/api/v4/spot/orders';
    return this.postSigned(endpoint, order);
  }

  /**
   * Cancel order
   */
  async cancelOrder(orderId: string, currencyPair: string): Promise<any> {
    const endpoint = `/api/v4/spot/orders/${orderId}`;
    return this.deleteSigned(endpoint, { currency_pair: currencyPair });
  }

  /**
   * Get order
   */
  async getOrder(orderId: string, currencyPair: string): Promise<GateOrder> {
    const endpoint = `/api/v4/spot/orders/${orderId}`;
    return this.getSigned(endpoint, { currency_pair: currencyPair });
  }

  /**
   * Get open orders
   */
  async getOpenOrders(currencyPair?: string): Promise<GateOrder[]> {
    const endpoint = '/api/v4/spot/orders';
    const params = currencyPair ? { currency_pair: currencyPair } : {};
    return this.getSigned(endpoint, params);
  }

  /**
   * Get my trades
   */
  async getMyTrades(currencyPair: string, limit: number = 100): Promise<any[]> {
    const endpoint = '/api/v4/spot/my_trades';
    return this.getSigned(endpoint, { currency_pair: currencyPair, limit: limit.toString() });
  }

  /**
   * Get tickers
   */
  async getTickers(currencyPair?: string): Promise<any> {
    const endpoint = '/api/v4/spot/tickers';
    const params = currencyPair ? { currency_pair: currencyPair } : {};
    return this.getPublic(endpoint, params);
  }

  /**
   * Get orderbook
   */
  async getOrderBook(currencyPair: string, limit: number = 10): Promise<any> {
    const endpoint = '/api/v4/spot/order_book';
    return this.getPublic(endpoint, { currency_pair: currencyPair, limit: limit.toString() });
  }

  /**
   * Get klines (candlesticks)
   */
  async getKlines(currencyPair: string, interval: string, limit: number = 100): Promise<any[]> {
    const endpoint = '/api/v4/spot/candlesticks';
    return this.getPublic(endpoint, { 
      currency_pair: currencyPair, 
      interval, 
      limit: limit.toString() 
    });
  }

  private async getSigned(endpoint: string, params: Record<string, string>): Promise<any> {
    const url = `${this.baseUrl}${endpoint}`;
    const query = new URLSearchParams(params).toString();
    const sign = this.sign('GET', endpoint, query);
    
    const response = await fetch(`${url}?${query}`, {
      headers: {
        'KEY': this.config.apiKey,
        'SIGN': sign,
      },
    });
    
    return response.json();
  }

  private async postSigned(endpoint: string, data: any): Promise<any> {
    const url = `${this.baseUrl}${endpoint}`;
    const body = JSON.stringify(data);
    const sign = this.sign('POST', endpoint, body);
    
    const response = await fetch(url, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'KEY': this.config.apiKey,
        'SIGN': sign,
      },
      body,
    });
    
    return response.json();
  }

  private async deleteSigned(endpoint: string, params: Record<string, string>): Promise<any> {
    const url = `${this.baseUrl}${endpoint}`;
    const query = new URLSearchParams(params).toString();
    const sign = this.sign('DELETE', endpoint, query);
    
    const response = await fetch(`${url}?${query}`, {
      method: 'DELETE',
      headers: {
        'KEY': this.config.apiKey,
        'SIGN': sign,
      },
    });
    
    return response.json();
  }

  private async getPublic(endpoint: string, params: Record<string, string>): Promise<any> {
    const query = new URLSearchParams(params).toString();
    const response = await fetch(`${this.baseUrl}${endpoint}?${query}`);
    return response.json();
  }

  private sign(method: string, endpoint: string, body: string): string {
    const crypto = require('crypto');
    const hash = crypto.createHash('sha512').update(body).digest('hex');
    const signature = crypto.createHmac('sha512', this.config.apiSecret)
      .update(method + '\n' + endpoint + '\n' + hash)
      .digest('hex');
    return signature;
  }
}

export default GateConnector;