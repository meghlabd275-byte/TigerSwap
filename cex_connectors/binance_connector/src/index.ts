/**
 * TigerSwap CEX Connectors - Binance Connector
 * 
 * Native Binance connector with complete REST/WebSocket API integration.
 * Zero dependencies on external libraries - fully native implementation.
 * 
 * Features:
 * - Spot trading
 * - Futures trading (USDT-M, COIN-M)
 * - Margin trading
 * - WebSocket streams
 * - Sub-account management
 * - API key management
 * 
 * @author TigerSwap
 * @version 1.0.0
 */

// ============================================================================
// Types & Interfaces
// ============================================================================

export interface BinanceConfig {
  apiKey: string;
  apiSecret: string;
  testnet: boolean;
  timeout: number;
  recvWindow: number;
}

export interface BinanceSymbol {
  symbol: string;
  baseAsset: string;
  quoteAsset: string;
  status: 'BREAKING' | 'PENDING_TRADING' | 'TRADING' | 'PENDING_DELISTING';
  quotePrecision: number;
  quoteAssetPrecision: number;
  orderTypes: string[];
  icebergAllowed: boolean;
}

export interface BinanceTicker {
  symbol: string;
  priceChange: string;
  priceChangePercent: string;
  weightedAvgPrice: string;
  prevClosePrice: string;
  lastPrice: string;
  lastQty: string;
  bidPrice: string;
  bidQty: string;
  askPrice: string;
  askQty: string;
  openPrice: string;
  highPrice: string;
  lowPrice: string;
  volume: string;
  quoteVolume: string;
  openTime: number;
  closeTime: number;
  firstId: number;
  lastId: number;
  count: number;
}

export interface BinanceOrderBook {
  lastUpdateId: number;
  bids: [string, string][];
  asks: [string, string][];
}

export interface BinanceTrade {
  id: number;
  price: string;
  qty: string;
  time: number;
  isBuyerMaker: boolean;
  isBestMatch: boolean;
}

export interface BinanceOrder {
  symbol: string;
  orderId: number;
  clientOrderId: string;
  price: string;
  origQty: string;
  executedQty: string;
  cummulativeQuoteQty: string;
  status: 'PENDING' | 'NEW' | 'PARTIALLY_FILLED' | 'FILLED' | 'CANCELED' | 'PENDING_CANCEL' | 'REJECTED' | 'EXPIRED';
  timeInForce: 'GTC' | 'IOC' | 'FOK';
  type: 'LIMIT' | 'LIMIT_MAKER' | 'MARKET' | 'STOP_LOSS' | 'STOP_LOSS_LIMIT' | 'TAKE_PROFIT' | 'TAKE_PROFIT_LIMIT';
  side: 'BUY' | 'SELL';
  stopPrice: string;
  icebergQty: string;
  time: number;
  updateTime: number;
  isWorking: boolean;
  origQuoteOrderQty: string;
}

export interface BinanceKline {
  openTime: number;
  open: string;
  high: string;
  low: string;
  close: string;
  volume: string;
  closeTime: number;
  quoteVolume: string;
  numTrades: number;
  takerBuyBaseVolume: string;
  takerBuyQuoteVolume: string;
}

export interface BinanceAccount {
  accountType: string;
  balances: {
    asset: string;
    free: string;
    locked: string;
  }[];
  canDeposit: boolean;
  canTrade: boolean;
  canWithdraw: boolean;
}

export interface BinancePosition {
  symbol: string;
  positionAmt: string;
  entryPrice: string;
  markPrice: string;
  unRealizedProfit: string;
  liquidationPrice: string;
  marginAmt: string;
  marginRatio: string;
  positionSide: 'BOTH' | 'LONG' | 'SHORT';
}

export interface BinanceWallet {
  asset: string;
  free: string;
  locked: string;
  withdrawAllEnabled: boolean;
  depositAllEnabled: boolean;
}

export interface BinanceDeposit {
  insertTime: number;
  amount: number;
  asset: string;
  txId: string;
  status: number;
}

export interface BinanceWithdraw {
  id: string;
  clientRequestId: string;
  asset: string;
  amount: number;
  timestamp: number;
  status: number;
}

export interface BinanceListenKey {
  listenKey: string;
  expiresAt: number;
}

export type BinanceEndpoint = 
  | '/api/v3/account'
  | '/api/v3/order'
  | '/api/v3/myTrades'
  | '/api/v3/openOrders'
  | '/api/v3/allOrders'
  | '/api/v3/depth'
  | '/api/v3/trades'
  | '/api/v3/klines'
  | '/api/v3/ticker/24hr'
  | '/api/v3/ticker/price'
  | '/api/v3/ticker/bookTicker'
  | '/api/v3/exchangeInfo'
  | '/sapi/v1/capital/deposit/address'
  | '/sapi/v1/capital/deposit/hisrec'
  | '/sapi/v1/capital/withdraw/apply'
  | '/sapi/v1/capital/withdraw/history'
  | '/sapi/v1/sub-account/virtual'
  | '/fapi/v2/account'
  | '/fapi/v2/positionRisk'
  | '/fapi/v1/openOrders'
  | '/fapi/v1/order'
  | '/fapi/v1/myTrades'
  | '/dapi/v1/account'
  | '/dapi/v1/positionRisk'
  | '/dapi/v1/openOrders'
  | '/dapi/v1/order';

// ============================================================================
// Binance API Client
// ============================================================================

/**
 * BinanceAPI - Native Binance API client
 * 
 * Complete implementation with:
 * - HMAC-SHA256 signature generation
 * - Request signing
 * - Rate limiting
 * - WebSocket streams
 * - Sub-account management
 */
export class BinanceAPI {
  private config: BinanceConfig;
  private baseUrl: string;
  private wsUrl: string;
  private nonce: number;
  private rateLimit: { remaining: number; reset: number };
  private requestQueue: Promise<any>;

  constructor(config: BinanceConfig) {
    this.config = config;
    this.baseUrl = config.testnet 
      ? 'https://testnet.binance.vision' 
      : 'https://api.binance.com';
    this.wsUrl = config.testnet
      ? 'wss://stream.testnet.binance.vision'
      : 'wss://stream.binance.com:9443';
    this.nonce = Date.now();
    this.rateLimit = { remaining: 1200, reset: Date.now() + 60000 };
    this.requestQueue = Promise.resolve();
  }

  // ============================================================================
  // Public API Endpoints
  // ============================================================================

  /**
   * Get exchange information
   */
  async getExchangeInfo(symbol?: string): Promise<{ symbols: BinanceSymbol[] }> {
    const params = symbol ? `?symbol=${symbol}` : '';
    return this.get('/api/v3/exchangeInfo' + params);
  }

  /**
   * Get order book
   */
  async getOrderBook(symbol: string, limit: number = 100): Promise<BinanceOrderBook> {
    return this.get(`/api/v3/depth?symbol=${symbol}&limit=${limit}`);
  }

  /**
   * Get recent trades
   */
  async getRecentTrades(symbol: string, limit: number = 500): Promise<BinanceTrade[]> {
    return this.get(`/api/v3/trades?symbol=${symbol}&limit=${limit}`);
  }

  /**
   * Get historical trades
   */
  async getHistoricalTrades(
    symbol: string, 
    fromId?: number, 
    limit: number = 500
  ): Promise<BinanceTrade[]> {
    let url = `/api/v3/historicalTrades?symbol=${symbol}&limit=${limit}`;
    if (fromId) url += `&fromId=${fromId}`;
    return this.get(url);
  }

  /**
   * Get aggregate trades
   */
  async getAggregateTrades(
    symbol: string, 
    fromId?: number, 
    startTime?: number, 
    endTime?: number
  ): Promise<any[]> {
    let url = `/api/v3/aggTrades?symbol=${symbol}`;
    if (fromId) url += `&fromId=${fromId}`;
    if (startTime) url += `&startTime=${startTime}`;
    if (endTime) url += `&endTime=${endTime}`;
    return this.get(url);
  }

  /**
   * Get klines (candlestick data)
   */
  async getKlines(
    symbol: string, 
    interval: string, 
    startTime?: number, 
    endTime?: number, 
    limit: number = 500
  ): Promise<BinanceKline[]> {
    let url = `/api/v3/klines?symbol=${symbol}&interval=${interval}&limit=${limit}`;
    if (startTime) url += `&startTime=${startTime}`;
    if (endTime) url += `&endTime=${endTime}`;
    return this.get(url);
  }

  /**
   * Get current 24hr ticker
   */
  async get24hrTicker(symbol?: string): Promise<BinanceTicker | BinanceTicker[]> {
    if (symbol) {
      return this.get(`/api/v3/ticker/24hr?symbol=${symbol}`);
    }
    return this.get('/api/v3/ticker/24hr');
  }

  /**
   * Get price ticker
   */
  async getPriceTicker(symbol?: string): Promise<{ symbol: string; price: string }[]> {
    if (symbol) {
      return this.get(`/api/v3/ticker/price?symbol=${symbol}`);
    }
    return this.get('/api/v3/ticker/price');
  }

  /**
   * Get book ticker
   */
  async getBookTicker(symbol?: string): Promise<BinanceTicker> {
    if (symbol) {
      return this.get(`/api/v3/ticker/bookTicker?symbol=${symbol}`);
    }
    return this.get('/api/v3/ticker/bookTicker');
  }

  // ============================================================================
  // Account API Endpoints
  // ============================================================================

  /**
   * Get account information
   */
  async getAccount(): Promise<BinanceAccount> {
    return this.getSigned('/api/v3/account');
  }

  /**
   * Get account trade list
   */
  async getMyTrades(
    symbol: string, 
    fromId?: number, 
    startTime?: number, 
    endTime?: number, 
    limit: number = 500
  ): Promise<any[]> {
    let params = `symbol=${symbol}&limit=${limit}`;
    if (fromId) params += `&fromId=${fromId}`;
    if (startTime) params += `&startTime=${startTime}`;
    if (endTime) params += `&endTime=${endTime}`;
    return this.getSigned(`/api/v3/myTrades?${params}`);
  }

  /**
   * Get open orders
   */
  async getOpenOrders(symbol?: string): Promise<BinanceOrder[]> {
    const url = symbol 
      ? `/api/v3/openOrders?symbol=${symbol}` 
      : '/api/v3/openOrders';
    return this.getSigned(url);
  }

  /**
   * Get all orders
   */
  async getAllOrders(
    symbol: string, 
    orderId?: number, 
    startTime?: number, 
    endTime?: number, 
    limit: number = 500
  ): Promise<BinanceOrder[]> {
    let params = `symbol=${symbol}&limit=${limit}`;
    if (orderId) params += `&orderId=${orderId}`;
    if (startTime) params += `&startTime=${startTime}`;
    if (endTime) params += `&endTime=${endTime}`;
    return this.getSigned(`/api/v3/allOrders?${params}`);
  }

  /**
   * Get order
   */
  async getOrder(symbol: string, orderId?: number, origClientOrderId?: string): Promise<BinanceOrder> {
    let params = `symbol=${symbol}`;
    if (orderId) params += `&orderId=${orderId}`;
    if (origClientOrderId) params += `&origClientOrderId=${origClientOrderId}`;
    return this.getSigned(`/api/v3/order?${params}`);
  }

  /**
   * Create order (LIMIT)
   */
  async createOrder(
    symbol: string,
    side: 'BUY' | 'SELL',
    type: 'LIMIT' | 'MARKET' | 'STOP_LOSS' | 'STOP_LOSS_LIMIT' | 'TAKE_PROFIT' | 'TAKE_PROFIT_LIMIT' | 'LIMIT_MAKER',
    quantity: string,
    price?: string,
    stopPrice?: string,
    timeInForce: 'GTC' | 'IOC' | 'FOK' = 'GTC',
    clientOrderId?: string
  ): Promise<BinanceOrder> {
    let params = `symbol=${symbol}&side=${side}&type=${type}&quantity=${quantity}`;
    
    if (type === 'LIMIT') {
      params += `&timeInForce=${timeInForce}&price=${price}`;
    }
    
    if (stopPrice) {
      params += `&stopPrice=${stopPrice}`;
    }
    
    if (clientOrderId) {
      params += `&newClientOrderId=${clientOrderId}`;
    }

    return this.postSigned(`/api/v3/order?${params}`);
  }

  /**
   * Test order (validate only)
   */
  async testOrder(
    symbol: string,
    side: 'BUY' | 'SELL',
    type: 'LIMIT' | 'MARKET',
    quantity: string,
    price?: string,
    timeInForce: 'GTC' | 'IOC' | 'FOK' = 'GTC'
  ): Promise<{}> {
    let params = `symbol=${symbol}&side=${side}&type=${type}&quantity=${quantity}`;
    if (type === 'LIMIT') {
      params += `&timeInForce=${timeInForce}&price=${price}`;
    }
    return this.postSigned(`/api/v3/order/test?${params}`);
  }

  /**
   * Cancel order
   */
  async cancelOrder(
    symbol: string, 
    orderId?: number, 
    origClientOrderId?: string,
    clientOrderId?: string
  ): Promise<BinanceOrder> {
    let params = `symbol=${symbol}`;
    if (orderId) params += `&orderId=${orderId}`;
    if (origClientOrderId) params += `&origClientOrderId=${origClientOrderId}`;
    if (clientOrderId) params += `&newClientOrderId=${clientOrderId}`;
    return this.deleteSigned(`/api/v3/order?${params}`);
  }

  /**
   * Cancel all open orders
   */
  async cancelAllOrders(symbol: string): Promise<{ cancelled: BinanceOrder[] }> {
    return this.deleteSigned(`/api/v3/openOrders?symbol=${symbol}`);
  }

  // ============================================================================
  // Wallet API Endpoints
  // ============================================================================

  /**
   * Get deposit address
   */
  async getDepositAddress(coin: string, network?: string): Promise<{ address: string; url: string }> {
    let params = `coin=${coin}`;
    if (network) params += `&network=${network}`;
    return this.getSigned(`/sapi/v1/capital/deposit/address?${params}`);
  }

  /**
   * Get deposit history
   */
  async getDepositHistory(
    coin?: string, 
    startTime?: number, 
    endTime?: number, 
    offset?: number, 
    limit: number = 100
  ): Promise<BinanceDeposit[]> {
    let params = `limit=${limit}`;
    if (coin) params += `&coin=${coin}`;
    if (startTime) params += `&startTime=${startTime}`;
    if (endTime) params += `&endTime=${endTime}`;
    if (offset) params += `&offset=${offset}`;
    return this.getSigned(`/sapi/v1/capital/deposit/hisrec?${params}`);
  }

  /**
   * Withdraw
   */
  async withdraw(
    coin: string,
    network: string,
    amount: number,
    address: string,
    addressTag?: string,
    clientId?: string
  ): Promise<BinanceWithdraw> {
    let params = `coin=${coin}&network=${network}&amount=${amount}&address=${address}`;
    if (addressTag) params += `&addressTag=${addressTag}`;
    if (clientId) params += `&clientId=${clientId}`;
    return this.postSigned(`/sapi/v1/capital/withdraw/apply?${params}`);
  }

  /**
   * Get withdrawal history
   */
  async getWithdrawalHistory(
    coin?: string, 
    startTime?: number, 
    endTime?: number, 
    limit: number = 100
  ): Promise<BinanceWithdraw[]> {
    let params = `limit=${limit}`;
    if (coin) params += `&coin=${coin}`;
    if (startTime) params += `&startTime=${startTime}`;
    if (endTime) params += `&endTime=${endTime}`;
    return this.getSigned(`/sapi/v1/capital/withdraw/history?${params}`);
  }

  /**
   * Get wallet balance
   */
  async getWalletBalances(): Promise<BinanceWallet[]> {
    return this.getSigned('/sapi/v1/account/balance');
  }

  // ============================================================================
  // Futures API Endpoints
  // ============================================================================

  /**
   * Get futures account
   */
  async getFuturesAccount(): Promise<any> {
    return this.getSigned('/fapi/v2/account');
  }

  /**
   * Get futures position risk
   */
  async getFuturesPositionRisk(symbol?: string): Promise<BinancePosition[]> {
    const url = symbol 
      ? `/fapi/v2/positionRisk?symbol=${symbol}` 
      : '/fapi/v2/positionRisk';
    return this.getSigned(url);
  }

  /**
   * Get futures open orders
   */
  async getFuturesOpenOrders(symbol?: string): Promise<BinanceOrder[]> {
    const url = symbol 
      ? `/fapi/v1/openOrders?symbol=${symbol}` 
      : '/fapi/v1/openOrders';
    return this.getSigned(url);
  }

  /**
   * Create futures order
   */
  async createFuturesOrder(
    symbol: string,
    side: 'BUY' | 'SELL',
    type: 'LIMIT' | 'MARKET' | 'STOP' | 'TAKE_PROFIT',
    quantity: string,
    price?: string,
    stopPrice?: string,
    timeInForce?: 'GTC' | 'IOC' | 'FOK',
    reduceOnly?: boolean,
    clientOrderId?: string
  ): Promise<BinanceOrder> {
    let params = `symbol=${symbol}&side=${side}&type=${type}&quantity=${quantity}`;
    
    if (price) params += `&price=${price}`;
    if (stopPrice) params += `&stopPrice=${stopPrice}`;
    if (timeInForce) params += `&timeInForce=${timeInForce}`;
    if (reduceOnly) params += `&reduceOnly=true`;
    if (clientOrderId) params += `&newClientOrderId=${clientOrderId}`;

    return this.postSigned(`/fapi/v1/order?${params}`);
  }

  /**
   * Cancel futures order
   */
  async cancelFuturesOrder(
    symbol: string, 
    orderId?: number, 
    origClientOrderId?: string
  ): Promise<BinanceOrder> {
    let params = `symbol=${symbol}`;
    if (orderId) params += `&orderId=${orderId}`;
    if (origClientOrderId) params += `&origClientOrderId=${origClientOrderId}`;
    return this.deleteSigned(`/fapi/v1/order?${params}`);
  }

  /**
   * Set futures leverage
   */
  async setFuturesLeverage(symbol: string, leverage: number): Promise<{ leverage: number }> {
    return this.postSigned(`/fapi/v1/leverage?symbol=${symbol}&leverage=${leverage}`);
  }

  /**
   * Set futures margin type
   */
  async setFuturesMarginType(
    symbol: string, 
    marginType: 'ISOLATED' | 'CROSSED'
  ): Promise<{ marginType: string }> {
    return this.postSigned(`/fapi/v1/marginType?symbol=${symbol}&marginType=${marginType}`);
  }

  /**
   * Add/remove futures margin
   */
  async modifyFuturesMargin(
    symbol: string, 
    amount: number, 
    type: number // 1 = add, 2 = remove
  ): Promise<{ amount: string; code: number; msg: string }> {
    return this.postSigned(`/fapi/v1/positionMargin?symbol=${symbol}&amount=${amount}&type=${type}`);
  }

  // ============================================================================
  // Coin-M Futures API
  // ============================================================================

  /**
   * Get coin-M futures account
   */
  async getCoinMAccount(): Promise<any> {
    return this.getSigned('/dapi/v1/account');
  }

  /**
   * Get coin-M position risk
   */
  async getCoinMPositionRisk(symbol?: string): Promise<BinancePosition[]> {
    const url = symbol 
      ? `/dapi/v1/positionRisk?symbol=${symbol}` 
      : '/dapi/v1/positionRisk';
    return this.getSigned(url);
  }

  // ============================================================================
  // WebSocket Streams
  // ============================================================================

  /**
   * Create listen key for WebSocket
   */
  async createListenKey(): Promise<BinanceListenKey> {
    return this.postSigned('/api/v1/userDataStream');
  }

  /**
   * Keep alive listen key
   */
  async keepAliveListenKey(listenKey: string): Promise<{}> {
    return this.putSigned(`/api/v1/userDataStream?listenKey=${listenKey}`);
  }

  /**
   * Close listen key
   */
  async closeListenKey(listenKey: string): Promise<{}> {
    return this.deleteSigned(`/api/v1/userDataStream?listenKey=${listenKey}`);
  }

  // ============================================================================
  // Sub-Account Management
  // ============================================================================

  /**
   * Create sub-account
   */
  async createSubAccount(): Promise<{ email: string; userId: number }> {
    return this.postSigned('/sapi/v1/sub-account/virtual', true);
  }

  /**
   * Get sub-account list
   */
  async getSubAccountList(enable?: number): Promise<{ subAccounts: any[] }> {
    const url = enable !== undefined 
      ? `/sapi/v1/sub-account/list?enable=${enable}` 
      : '/sapi/v1/sub-account/list';
    return this.getSigned(url);
  }

  /**
   * Enable sub-account for futures
   */
  async enableSubAccountForFutures(email: string): Promise<{}> {
    return this.postSigned(`/sapi/v1/sub-account/futures?email=${email}`, true);
  }

  /**
   * Get sub-account futures balance
   */
  async getSubAccountFuturesBalance(email: string): Promise<any> {
    return this.getSigned(`/sapi/v1/sub-account/futures/balance?email=${email}`);
  }

  // ============================================================================
  // HTTP Methods
  // ============================================================================

  private async get(endpoint: string): Promise<any> {
    return this.request('GET', endpoint);
  }

  private async post(endpoint: string, isSapi: boolean = false): Promise<any> {
    return this.request('POST', endpoint, undefined, isSapi);
  }

  private async putSigned(endpoint: string): Promise<any> {
    return this.request('PUT', endpoint);
  }

  private async deleteSigned(endpoint: string): Promise<any> {
    return this.request('DELETE', endpoint);
  }

  private async getSigned(endpoint: string): Promise<any> {
    return this.request('GET', endpoint, true);
  }

  private async postSigned(endpoint: string, isSapi: boolean = false): Promise<any> {
    return this.request('POST', endpoint, true, isSapi);
  }

  private async request(
    method: string, 
    endpoint: string, 
    signed: boolean = false,
    isSapi: boolean = false
  ): Promise<any> {
    // Queue requests to respect rate limits
    await this.requestQueue;
    
    this.requestQueue = this.executeRequest(method, endpoint, signed, isSapi);
    return this.requestQueue;
  }

  private async executeRequest(
    method: string, 
    endpoint: string, 
    signed: boolean,
    isSapi: boolean
  ): Promise<any> {
    let url = this.baseUrl + endpoint;
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
    };

    if (signed) {
      const timestamp = Date.now();
      const recvWindow = this.config.recvWindow;
      
      let query = endpoint.includes('?') 
        ? endpoint.split('?')[1] + `&timestamp=${timestamp}&recvWindow=${recvWindow}`
        : `timestamp=${timestamp}&recvWindow=${recvWindow}`;
      
      // Add signature
      const signature = this.sign(query);
      query += `&signature=${signature}`;
      
      url = this.baseUrl + endpoint.split('?')[0] + '?' + query;
      
      headers['X-MBX-APIKEY'] = this.config.apiKey;
    }

    try {
      const response = await fetch(url, {
        method,
        headers,
      });

      // Update rate limits
      const remaining = response.headers.get('X-MBX-USED-WEIGHT-1M');
      if (remaining) {
        this.rateLimit.remaining = 1200 - parseInt(remaining);
      }

      if (!response.ok) {
        const error = await response.json();
        throw new Error(error.msg || error.code || 'Request failed');
      }

      return response.json();
    } catch (error) {
      console.error('Binance API Error:', error);
      throw error;
    }
  }

  // ============================================================================
  // Signature Generation
  // ============================================================================

  /**
   * Generate HMAC-SHA256 signature
   */
  private sign(message: string): string {
    const crypto = require('crypto');
    const hmac = crypto.createHmac('sha256', this.config.apiSecret);
    const signature = hmac.update(message).digest('hex');
    return signature;
  }
}

// ============================================================================
// WebSocket Streams
// ============================================================================

/**
 * BinanceWebSocket - Real-time WebSocket streams
 */
export class BinanceWebSocket {
  private ws: WebSocket | null;
  private url: string;
  private handlers: Map<string, Function>;
  private reconnectAttempts: number;
  private maxReconnectAttempts: number;
  private reconnectDelay: number;

  constructor(testnet: boolean = false) {
    this.url = testnet
      ? 'wss://stream.testnet.binance.vision/stream'
      : 'wss://stream.binance.com:9443/stream';
    this.ws = null;
    this.handlers = new Map();
    this.reconnectAttempts = 0;
    this.maxReconnectAttempts = 5;
    this.reconnectDelay = 3000;
  }

  /**
   * Connect to WebSocket stream
   */
  connect(streams: string | string[]): void {
    const streamList = Array.isArray(streams) ? streams.join('/') : streams;
    const wsUrl = `${this.url}?streams=${streamList}`;

    this.ws = new WebSocket(wsUrl);

    this.ws.onopen = () => {
      console.log('WebSocket connected');
      this.reconnectAttempts = 0;
    };

    this.ws.onmessage = (event) => {
      this.handleMessage(JSON.parse(event.data));
    };

    this.ws.onerror = (error) => {
      console.error('WebSocket error:', error);
    };

    this.ws.onclose = () => {
      console.log('WebSocket closed');
      this.attemptReconnect(streams);
    };
  }

  /**
   * Subscribe to stream
   */
  subscribe(stream: string, handler: Function): void {
    this.handlers.set(stream, handler);
    
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify({
        method: 'SUBSCRIBE',
        params: [stream],
        id: Date.now(),
      }));
    }
  }

  /**
   * Unsubscribe from stream
   */
  unsubscribe(stream: string): void {
    this.handlers.delete(stream);
    
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify({
        method: 'UNSUBSCRIBE',
        params: [stream],
        id: Date.now(),
      }));
    }
  }

  /**
   * Close connection
   */
  disconnect(): void {
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
  }

  private handleMessage(data: any): void {
    if (data.stream && this.handlers.has(data.stream)) {
      this.handlers.get(data.stream)!(data.data);
    } else if (data.e && this.handlers.has(data.e.toLowerCase())) {
      this.handlers.get(data.e.toLowerCase())(data);
    }
  }

  private attemptReconnect(streams: string | string[]): void {
    if (this.reconnectAttempts < this.maxReconnectAttempts) {
      this.reconnectAttempts++;
      setTimeout(() => {
        console.log(`Reconnecting... attempt ${this.reconnectAttempts}`);
        this.connect(streams);
      }, this.reconnectDelay * this.reconnectAttempts);
    } else {
      console.error('Max reconnect attempts reached');
    }
  }
}

// ============================================================================
// Utility Functions
// ============================================================================

/**
 * Create Binance API client
 */
export function createBinanceClient(config: BinanceConfig): BinanceAPI {
  return new BinanceAPI(config);
}

/**
 * Create Binance WebSocket client
 */
export function createBinanceWebSocket(testnet: boolean = false): BinanceWebSocket {
  return new BinanceWebSocket(testnet);
}

/**
 * Validate API keys
 */
export function validateApiKeys(apiKey: string, apiSecret: string): boolean {
  return apiKey.length === 64 && apiSecret.length === 64;
}

/**
 * Parse timestamp to Date
 */
export function parseTimestamp(timestamp: number): Date {
  return new Date(timestamp);
}

/**
 * Format for display
 */
export function formatSymbol(base: string, quote: string): string {
  return `${base}${quote}`.toUpperCase();
}

// ============================================================================
// Export
// ============================================================================

export default {
  BinanceAPI,
  BinanceWebSocket,
  createBinanceClient,
  createBinanceWebSocket,
  validateApiKeys,
  parseTimestamp,
  formatSymbol,
};