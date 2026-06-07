/**
 * TigerSwap CEX Connectors - Bitget Connector
 * 
 * Native Bitget exchange connector.
 * Zero external dependencies - fully native implementation.
 * 
 * @author TigerSwap
 * @version 1.0.0
 */

export interface BitgetConfig {
  apiKey: string;
  apiSecret: string;
  passphrase: string;
}

export class BitgetConnector {
  private config: BitgetConfig;
  private baseUrl: string = 'https://api.bitget.com';

  constructor(config: BitgetConfig) {
    this.config = config;
  }

  async getBalance(): Promise<any> {
    const endpoint = '/api/v2/spot/account/tokens';
    return this.getSigned(endpoint, { });
  }

  async placeOrder(order: { symbol: string; side: string; orderType: string; size: string; price?: string }): Promise<any> {
    const endpoint = '/api/v2/spot/order/place-order';
    return this.postSigned(endpoint, order);
  }

  async cancelOrder(orderId: string, symbol: string): Promise<any> {
    const endpoint = '/api/v2/spot/order/cancel-order';
    return this.postSigned(endpoint, { orderId, symbol });
  }

  async getOpenOrders(symbol?: string): Promise<any> {
    const endpoint = '/api/v2/spot/order/current-batch-orders';
    return this.getSigned(endpoint, { symbol: symbol || '' });
  }

  async getTicker(symbol: string): Promise<any> {
    const endpoint = '/api/v2/spot/market/ticker';
    return this.getPublic(endpoint, { symbol });
  }

  async getOrderBook(symbol: string, limit: number = 10): Promise<any> {
    const endpoint = '/api/v2/spot/market/depth';
    return this.getPublic(endpoint, { symbol, limit: limit.toString() });
  }

  private async getSigned(endpoint: string, params: any): Promise<any> {
    const timestamp = Date.now().toString();
    const query = new URLSearchParams(params).toString();
    const sign = this.sign('GET', endpoint, query, timestamp);
    const response = await fetch(`${this.baseUrl}${endpoint}?${query}`, {
      headers: {
        'Content-Type': 'application/json',
        'OK-ACCESS-KEY': this.config.apiKey,
        'OK-ACCESS-SIGN': sign,
        'OK-ACCESS-TIMESTAMP': timestamp,
        'OK-ACCESS-PASSPHRASE': this.config.passphrase,
      },
    });
    return response.json();
  }

  private async postSigned(endpoint: string, data: any): Promise<any> {
    const timestamp = Date.now().toString();
    const body = JSON.stringify(data);
    const sign = this.sign('POST', endpoint, body, timestamp);
    const response = await fetch(`${this.baseUrl}${endpoint}`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'OK-ACCESS-KEY': this.config.apiKey,
        'OK-ACCESS-SIGN': sign,
        'OK-ACCESS-TIMESTAMP': timestamp,
        'OK-ACCESS-PASSPHRASE': this.config.passphrase,
      },
      body,
    });
    return response.json();
  }

  private async getPublic(endpoint: string, params: any): Promise<any> {
    const query = new URLSearchParams(params).toString();
    const response = await fetch(`${this.baseUrl}${endpoint}?${query}`);
    return response.json();
  }

  private sign(method: string, endpoint: string, body: string, timestamp: string): string {
    const crypto = require('crypto');
    const message = timestamp + method + endpoint + body;
    const hmac = crypto.createHmac('sha256', this.config.apiSecret);
    return hmac.update(message).digest('base64');
  }
}

export default BitgetConnector;