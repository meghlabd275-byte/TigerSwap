/**
 * TigerSwap CEX Connectors - MEXC Connector
 * 
 * Native MEXC exchange connector.
 * Zero external dependencies - fully native implementation.
 * 
 * @author TigerSwap
 * @version 1.0.0
 */

export interface MEXCConfig {
  apiKey: string;
  apiSecret: string;
}

export class MEXCConnector {
  private config: MEXCConfig;
  private baseUrl: string = 'https://api.mexc.com';

  constructor(config: MEXCConfig) {
    this.config = config;
  }

  async getBalance(): Promise<any> {
    const endpoint = '/api/v3/account/balance';
    return this.getSigned(endpoint, {});
  }

  async placeOrder(order: { symbol: string; side: string; type: string; quantity: string; price?: string }): Promise<any> {
    const endpoint = '/api/v3/order';
    return this.postSigned(endpoint, order);
  }

  async cancelOrder(orderId: string, symbol: string): Promise<any> {
    const endpoint = '/api/v3/order';
    return this.deleteSigned(endpoint, { orderId, symbol });
  }

  async getOpenOrders(symbol?: string): Promise<any> {
    const endpoint = '/api/v3/openOrders';
    return this.getSigned(endpoint, { symbol: symbol || '' });
  }

  async getTicker(symbol: string): Promise<any> {
    const endpoint = '/api/v3/ticker/24hr';
    return this.getPublic(endpoint, { symbol });
  }

  async getOrderBook(symbol: string, limit: number = 10): Promise<any> {
    const endpoint = '/api/v3/depth';
    return this.getPublic(endpoint, { symbol, limit: limit.toString() });
  }

  private async getSigned(endpoint: string, params: any): Promise<any> {
    const timestamp = Date.now().toString();
    const query = new URLSearchParams({ ...params, timestamp }).toString();
    const signature = this.sign(query);
    const response = await fetch(`${this.baseUrl}${endpoint}?${query}&signature=${signature}`, {
      headers: { 'X-MEXC-APIKEY': this.config.apiKey },
    });
    return response.json();
  }

  private async postSigned(endpoint: string, data: any): Promise<any> {
    const timestamp = Date.now().toString();
    const body = JSON.stringify({ ...data, timestamp });
    const signature = this.sign(new URLSearchParams(data).toString() + timestamp);
    const response = await fetch(`${this.baseUrl}${endpoint}`, {
      method: 'POST',
      headers: { 
        'Content-Type': 'application/json',
        'X-MEXC-APIKEY': this.config.apiKey,
        'X-MEXC-SIGNATURE': signature,
      },
      body,
    });
    return response.json();
  }

  private async deleteSigned(endpoint: string, params: any): Promise<any> {
    const timestamp = Date.now().toString();
    const query = new URLSearchParams({ ...params, timestamp }).toString();
    const signature = this.sign(query);
    const response = await fetch(`${this.baseUrl}${endpoint}?${query}&signature=${signature}`, {
      method: 'DELETE',
      headers: { 'X-MEXC-APIKEY': this.config.apiKey },
    });
    return response.json();
  }

  private async getPublic(endpoint: string, params: any): Promise<any> {
    const query = new URLSearchParams(params).toString();
    const response = await fetch(`${this.baseUrl}${endpoint}?${query}`);
    return response.json();
  }

  private sign(message: string): string {
    const crypto = require('crypto');
    return crypto.createHmac('sha256', this.config.apiSecret).update(message).digest('hex');
  }
}

export default MEXCConnector;