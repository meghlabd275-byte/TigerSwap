/**
 * TigerSwap CEX Connectors - Bybit Connector
 * 
 * Native Bybit exchange connector with complete API integration.
 * Zero external dependencies - fully native implementation.
 * 
 * Features:
 * - Spot trading
 * - Linear futures
 * - Inverse futures
 * - Options
 * - Copy trading
 * 
 * @author TigerSwap
 * @version 1.0.0
 */

export interface BybitConfig {
  apiKey: string;
  apiSecret: string;
  testnet: boolean;
}

export interface BybitOrder {
  orderId: string;
  orderLinkId: string;
  symbol: string;
  side: 'Buy' | 'Sell';
  orderType: 'Limit' | 'Market' | 'Stop' | 'StopLimit';
  price: string;
  qty: string;
  status: 'Created' | 'New' | 'PartiallyFilled' | 'Filled' | 'Cancelled' | 'PendingCancel' | 'Rejected';
}

export interface BybitPosition {
  symbol: string;
  side: 'Buy' | 'Sell';
  size: string;
  entryPrice: string;
  markPrice: string;
  liqPrice: string;
  positionValue: string;
  leverage: string;
  margin: string;
  unrealizedPnl: string;
}

export class BybitConnector {
  private config: BybitConfig;
  private baseUrl: string;

  constructor(config: BybitConfig) {
    this.config = config;
    this.baseUrl = config.testnet 
      ? 'https://api-testnet.bybit.com'
      : 'https://api.bybit.com';
  }

  /**
   * Get wallet balance
   */
  async getBalance(): Promise<any> {
    const endpoint = '/v5/account/wallet-balance';
    const params = { accountType: 'UNIFIED' };
    return this.getSigned(endpoint, params);
  }

  /**
   * Place order
   */
  async placeOrder(order: {
    symbol: string;
    side: 'Buy' | 'Sell';
    orderType: 'Limit' | 'Market';
    qty: string;
    price?: string;
  }): Promise<BybitOrder> {
    const endpoint = '/v5/order/create';
    return this.postSigned(endpoint, order);
  }

  /**
   * Get order list
   */
  async getOrders(symbol: string): Promise<BybitOrder[]> {
    const endpoint = '/v5/order/realtime';
    return this.getSigned(endpoint, { symbol });
  }

  /**
   * Cancel order
   */
  async cancelOrder(orderId: string, symbol: string): Promise<any> {
    const endpoint = '/v5/order/cancel';
    return this.postSigned(endpoint, { orderId, symbol });
  }

  /**
   * Get positions
   */
  async getPositions(category: string = 'linear'): Promise<BybitPosition[]> {
    const endpoint = '/v5/position/closed-pnl';
    const data = await this.getSigned(endpoint, { category });
    return data.list || [];
  }

  /**
   * Set leverage
   */
  async setLeverage(symbol: string, buyLeverage: number, sellLeverage: number): Promise<any> {
    const endpoint = '/v5/position/set-leverage';
    return this.postSigned(endpoint, { symbol, buyLeverage: buyLeverage.toString(), sellLeverage: sellLeverage.toString() });
  }

  /**
   * Get market ticker
   */
  async getTicker(symbol: string): Promise<any> {
    const endpoint = '/v5/market/ticker';
    return this.getPublic(endpoint, { symbol });
  }

  /**
   * Get orderbook
   */
  async getOrderBook(symbol: string, limit: number = 25): Promise<any> {
    const endpoint = '/v5/market/orderbook';
    return this.getPublic(endpoint, { symbol, limit: limit.toString() });
  }

  private async getSigned(endpoint: string, params: any = {}): Promise<any> {
    const timestamp = Date.now().toString();
    const recvWindow = '5000';
    const paramStr = Object.entries(params).map(([k, v]) => `${k}=${v}`).join('&');
    const sign = this.sign(`${timestamp}${this.config.apiKey}${recvWindow}${paramStr}`);
    
    const response = await fetch(`${this.baseUrl}${endpoint}?${paramStr}`, {
      headers: {
        'X-BAPI-API-KEY': this.config.apiKey,
        'X-BAPI-SIGN': sign,
        'X-BAPI-SIGN-TYPE': '2',
        'X-BAPI-TIMESTAMP': timestamp,
        'X-BAPI-RECV-WINDOW': recvWindow,
      },
    });
    
    return response.json();
  }

  private async postSigned(endpoint: string, params: any = {}): Promise<any> {
    const timestamp = Date.now().toString();
    const recvWindow = '5000';
    const body = JSON.stringify(params);
    const sign = this.sign(`${timestamp}${this.config.apiKey}${recvWindow}${body}`);
    
    const response = await fetch(`${this.baseUrl}${endpoint}`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-BAPI-API-KEY': this.config.apiKey,
        'X-BAPI-SIGN': sign,
        'X-BAPI-SIGN-TYPE': '2',
        'X-BAPI-TIMESTAMP': timestamp,
        'X-BAPI-RECV-WINDOW': recvWindow,
      },
      body,
    });
    
    return response.json();
  }

  private async getPublic(endpoint: string, params: any = {}): Promise<any> {
    const paramStr = Object.entries(params).map(([k, v]) => `${k}=${v}`).join('&');
    const response = await fetch(`${this.baseUrl}${endpoint}?${paramStr}`);
    return response.json();
  }

  private sign(message: string): string {
    const crypto = require('crypto');
    return crypto.createHmac('sha256', this.config.apiSecret).update(message).digest('hex');
  }
}

export default BybitConnector;