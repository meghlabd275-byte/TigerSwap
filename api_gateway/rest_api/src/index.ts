/**
 * TigerSwap REST API - API Gateway
 * 
 * Enterprise-grade REST API for TigerSwap ecosystem.
 * Zero external dependencies - fully native implementation.
 * 
 * Features:
 * - Wallet API
 * - Swap API
 * - Bridge API
 * - Pool API
 * - Analytics API
 * - WebSocket support
 * 
 * @author TigerSwap
 * @version 1.0.0
 */

// ============================================================================
// Types & Interfaces
// ============================================================================

export interface APIConfig {
  port: number;
  host: string;
  cors: boolean;
  rateLimit: {
    windowMs: number;
    maxRequests: number;
  };
}

export interface SwapRequest {
  fromToken: string;
  toToken: string;
  amount: string;
  slippage?: number;
  gasPrice?: string;
}

export interface SwapResponse {
  hash: string;
  fromToken: string;
  toToken: string;
  fromAmount: string;
  toAmount: string;
  priceImpact: number;
  gasUsed: string;
}

export interface BridgeRequest {
  fromChain: number;
  toChain: number;
  token: string;
  amount: string;
  recipient: string;
}

export interface BridgeResponse {
  hash: string;
  bridgeId: string;
  status: 'pending' | 'deposited' | 'completed' | 'failed';
  estimatedTime: number;
}

export interface PoolRequest {
  tokenA: string;
  tokenB: string;
}

export interface PoolResponse {
  address: string;
  tokenA: string;
  tokenB: string;
  reserveA: string;
  reserveB: string;
  tvl: string;
  apr: string;
}

export interface WalletResponse {
  address: string;
  balance: string;
  chainId: number;
}

export interface TokenListResponse {
  tokens: Array<{
    address: string;
    symbol: string;
    name: string;
    decimals: number;
    logo: string;
    chainId: number;
  }>;
}

export interface MarketResponse {
  symbol: string;
  price: string;
  change24h: number;
  volume24h: string;
  high24h: string;
  low24h: string;
}

export interface AnalyticsResponse {
  tvl: string;
  volume24h: string;
  fees24h: string;
  users24h: number;
}

export interface ErrorResponse {
  code: string;
  message: string;
  details?: any;
}

// ============================================================================
// API Endpoints
// ============================================================================

export const API_ENDPOINTS = {
  // Wallet
  WALLET_BALANCE: '/api/v1/wallet/balance',
  WALLET_HISTORY: '/api/v1/wallet/history',
  
  // Swap
  SWAP_QUOTE: '/api/v1/swap/quote',
  SWAP_EXECUTE: '/api/v1/swap/execute',
  SWAP_STATUS: '/api/v1/swap/:hash',
  
  // Bridge
  BRIDGE_QUOTE: '/api/v1/bridge/quote',
  BRIDGE_EXECUTE: '/api/v1/bridge/execute',
  BRIDGE_STATUS: '/api/v1/bridge/:id',
  
  // Pool
  POOL_LIST: '/api/v1/pools',
  POOL_INFO: '/api/v1/pools/:address',
  POOL_ADD_LIQUIDITY: '/api/v1/pools/:address/liquidity',
  POOL_REMOVE_LIQUIDITY: '/api/v1/pools/:address/remove',
  
  // Tokens
  TOKEN_LIST: '/api/v1/tokens',
  TOKEN_INFO: '/api/v1/tokens/:address',
  TOKEN_PRICE: '/api/v1/tokens/:address/price',
  
  // Market
  MARKET_TICKER: '/api/v1/market/ticker',
  MARKET_TRADES: '/api/v1/market/trades',
  MARKET_ORDERBOOK: '/api/v1/market/orderbook',
  
  // Analytics
  ANALYTICS_OVERVIEW: '/api/v1/analytics/overview',
  ANALYTICS_VOLUME: '/api/v1/analytics/volume',
  ANALYTICS_TVL: '/api/v1/analytics/tvl',
  
  // User
  USER_PORTFOLIO: '/api/v1/user/portfolio',
  USER_POSITIONS: '/api/v1/user/positions',
  USER_HISTORY: '/api/v1/user/history',
};

// ============================================================================
// API Client
// ============================================================================

export class APIClient {
  private baseUrl: string;
  private apiKey: string;
  private timeout: number;

  constructor(baseUrl: string, apiKey?: string, timeout: number = 30000) {
    this.baseUrl = baseUrl;
    this.apiKey = apiKey || '';
    this.timeout = timeout;
  }

  /**
   * Set API key
   */
  setApiKey(key: string): void {
    this.apiKey = key;
  }

  /**
   * Get headers
   */
  private getHeaders(): Record<string, string> {
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
    };
    if (this.apiKey) {
      headers['Authorization'] = `Bearer ${this.apiKey}`;
    }
    return headers;
  }

  /**
   * Make request
   */
  private async request<T>(
    method: string,
    endpoint: string,
    body?: any
  ): Promise<T> {
    const url = `${this.baseUrl}${endpoint}`;
    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), this.timeout);

    try {
      const response = await fetch(url, {
        method,
        headers: this.getHeaders(),
        body: body ? JSON.stringify(body) : undefined,
        signal: controller.signal,
      });

      clearTimeout(timeoutId);

      if (!response.ok) {
        const error: ErrorResponse = await response.json();
        throw new Error(error.message || 'API request failed');
      }

      return response.json();
    } catch (error) {
      clearTimeout(timeoutId);
      throw error;
    }
  }

  /**
   * GET request
   */
  async get<T>(endpoint: string): Promise<T> {
    return this.request<T>('GET', endpoint);
  }

  /**
   * POST request
   */
  async post<T>(endpoint: string, body: any): Promise<T> {
    return this.request<T>('POST', endpoint, body);
  }

  /**
   * PUT request
   */
  async put<T>(endpoint: string, body: any): Promise<T> {
    return this.request<T>('PUT', endpoint, body);
  }

  /**
   * DELETE request
   */
  async delete<T>(endpoint: string): Promise<T> {
    return this.request<T>('DELETE', endpoint);
  }

  // ============================================================================
  // Wallet Endpoints
  // ============================================================================

  /**
   * Get wallet balance
   */
  async getBalance(address: string): Promise<WalletResponse> {
    return this.get<WalletResponse>(`${API_ENDPOINTS.WALLET_BALANCE}?address=${address}`);
  }

  /**
   * Get wallet history
   */
  async getWalletHistory(address: string, limit?: number): Promise<any[]> {
    const params = new URLSearchParams({ address });
    if (limit) params.append('limit', limit.toString());
    return this.get<any[]>(`${API_ENDPOINTS.WALLET_HISTORY}?${params}`);
  }

  // ============================================================================
  // Swap Endpoints
  // ============================================================================

  /**
   * Get swap quote
   */
  async getSwapQuote(request: SwapRequest): Promise<SwapResponse> {
    return this.post<SwapResponse>(API_ENDPOINTS.SWAP_QUOTE, request);
  }

  /**
   * Execute swap
   */
  async executeSwap(request: SwapRequest): Promise<SwapResponse> {
    return this.post<SwapResponse>(API_ENDPOINTS.SWAP_EXECUTE, request);
  }

  /**
   * Get swap status
   */
  async getSwapStatus(hash: string): Promise<SwapResponse> {
    return this.get<SwapResponse>(API_ENDPOINTS.SWAP_STATUS.replace(':hash', hash));
  }

  // ============================================================================
  // Bridge Endpoints
  // ============================================================================

  /**
   * Get bridge quote
   */
  async getBridgeQuote(request: BridgeRequest): Promise<any> {
    return this.post<any>(API_ENDPOINTS.BRIDGE_QUOTE, request);
  }

  /**
   * Execute bridge
   */
  async executeBridge(request: BridgeRequest): Promise<BridgeResponse> {
    return this.post<BridgeResponse>(API_ENDPOINTS.BRIDGE_EXECUTE, request);
  }

  /**
   * Get bridge status
   */
  async getBridgeStatus(bridgeId: string): Promise<BridgeResponse> {
    return this.get<BridgeResponse>(API_ENDPOINTS.BRIDGE_STATUS.replace(':id', bridgeId));
  }

  // ============================================================================
  // Pool Endpoints
  // ============================================================================

  /**
   * Get pool list
   */
  async getPools(tokenA?: string, tokenB?: string): Promise<PoolResponse[]> {
    const params = new URLSearchParams();
    if (tokenA) params.append('tokenA', tokenA);
    if (tokenB) params.append('tokenB', tokenB);
    return this.get<PoolResponse[]>(`${API_ENDPOINTS.POOL_LIST}?${params}`);
  }

  /**
   * Get pool info
   */
  async getPoolInfo(address: string): Promise<PoolResponse> {
    return this.get<PoolResponse>(API_ENDPOINTS.POOL_INFO.replace(':address', address));
  }

  // ============================================================================
  // Token Endpoints
  // ============================================================================

  /**
   * Get token list
   */
  async getTokens(chainId?: number): Promise<TokenListResponse> {
    const params = chainId ? `?chainId=${chainId}` : '';
    return this.get<TokenListResponse>(API_ENDPOINTS.TOKEN_LIST + params);
  }

  /**
   * Get token info
   */
  async getTokenInfo(address: string): Promise<any> {
    return this.get<any>(API_ENDPOINTS.TOKEN_INFO.replace(':address', address));
  }

  /**
   * Get token price
   */
  async getTokenPrice(address: string): Promise<{ price: string; change24h: number }> {
    return this.get<{ price: string; change24h: number }>(
      API_ENDPOINTS.TOKEN_PRICE.replace(':address', address)
    );
  }

  // ============================================================================
  // Market Endpoints
  // ============================================================================

  /**
   * Get market ticker
   */
  async getMarketTicker(symbol?: string): Promise<MarketResponse[]> {
    const params = symbol ? `?symbol=${symbol}` : '';
    return this.get<MarketResponse[]>(API_ENDPOINTS.MARKET_TICKER + params);
  }

  /**
   * Get market trades
   */
  async getMarketTrades(symbol: string, limit?: number): Promise<any[]> {
    const params = new URLSearchParams({ symbol });
    if (limit) params.append('limit', limit.toString());
    return this.get<any[]>(`${API_ENDPOINTS.MARKET_TRADES}?${params}`);
  }

  /**
   * Get orderbook
   */
  async getOrderBook(symbol: string, limit?: number): Promise<any> {
    const params = new URLSearchParams({ symbol });
    if (limit) params.append('limit', limit.toString());
    return this.get<any>(`${API_ENDPOINTS.MARKET_ORDERBOOK}?${params}`);
  }

  // ============================================================================
  // Analytics Endpoints
  // ============================================================================

  /**
   * Get analytics overview
   */
  async getAnalyticsOverview(): Promise<AnalyticsResponse> {
    return this.get<AnalyticsResponse>(API_ENDPOINTS.ANALYTICS_OVERVIEW);
  }

  /**
   * Get volume analytics
   */
  async getVolumeAnalytics(period: string = '24h'): Promise<any> {
    return this.get<any>(`${API_ENDPOINTS.ANALYTICS_VOLUME}?period=${period}`);
  }

  /**
   * Get TVL analytics
   */
  async getTVLAnalytics(): Promise<any> {
    return this.get<any>(API_ENDPOINTS.ANALYTICS_TVL);
  }

  // ============================================================================
  // User Endpoints
  // ============================================================================

  /**
   * Get user portfolio
   */
  async getUserPortfolio(address: string): Promise<any> {
    return this.get<any>(`${API_ENDPOINTS.USER_PORTFOLIO}?address=${address}`);
  }

  /**
   * Get user positions
   */
  async getUserPositions(address: string): Promise<any[]> {
    return this.get<any[]>(`${API_ENDPOINTS.USER_POSITIONS}?address=${address}`);
  }

  /**
   * Get user history
   */
  async getUserHistory(address: string, limit?: number): Promise<any[]> {
    const params = new URLSearchParams({ address });
    if (limit) params.append('limit', limit.toString());
    return this.get<any[]>(`${API_ENDPOINTS.USER_HISTORY}?${params}`);
  }
}

// ============================================================================
// WebSocket Client
// ============================================================================

export class WSClient {
  private ws: WebSocket | null;
  private url: string;
  private handlers: Map<string, Function[]>;
  private reconnectAttempts: number;
  private maxReconnectAttempts: number;

  constructor(url: string) {
    this.url = url;
    this.ws = null;
    this.handlers = new Map();
    this.reconnectAttempts = 0;
    this.maxReconnectAttempts = 5;
  }

  /**
   * Connect
   */
  connect(): void {
    this.ws = new WebSocket(this.url);

    this.ws.onopen = () => {
      console.log('WebSocket connected');
      this.reconnectAttempts = 0;
    };

    this.ws.onmessage = (event) => {
      const data = JSON.parse(event.data);
      const type = data.type || 'message';
      const callbacks = this.handlers.get(type) || [];
      callbacks.forEach(callback => callback(data));
    };

    this.ws.onerror = (error) => {
      console.error('WebSocket error:', error);
    };

    this.ws.onclose = () => {
      console.log('WebSocket closed');
      this.reconnect();
    };
  }

  /**
   * Subscribe
   */
  subscribe(channel: string, callback: Function): void {
    const handlers = this.handlers.get(channel) || [];
    handlers.push(callback);
    this.handlers.set(channel, handlers);

    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify({
        action: 'subscribe',
        channel,
      }));
    }
  }

  /**
   * Unsubscribe
   */
  unsubscribe(channel: string): void {
    this.handlers.delete(channel);

    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify({
        action: 'unsubscribe',
        channel,
      }));
    }
  }

  /**
   * Send message
   */
  send(message: any): void {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(message));
    }
  }

  /**
   * Close
   */
  close(): void {
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
  }

  private reconnect(): void {
    if (this.reconnectAttempts < this.maxReconnectAttempts) {
      this.reconnectAttempts++;
      setTimeout(() => {
        console.log(`Reconnecting... attempt ${this.reconnectAttempts}`);
        this.connect();
      }, 1000 * this.reconnectAttempts);
    }
  }
}

// ============================================================================
// Factory
// ============================================================================

export function createAPIClient(baseUrl: string, apiKey?: string): APIClient {
  return new APIClient(baseUrl, apiKey);
}

export function createWSClient(url: string): WSClient {
  return new WSClient(url);
}

// ============================================================================
// Export
// ============================================================================

export default {
  APIClient,
  WSClient,
  createAPIClient,
  createWSClient,
  API_ENDPOINTS,
};