/**
 * TigerSwap TypeScript SDK
 * Complete DEX functionality for trading, routing, orders, and more
 */

export interface Token {
  address: string;
  symbol: string;
  name: string;
  decimals: number;
  chainId: number;
  logoUrl?: string;
}

export interface TokenPair {
  tokenIn: Token;
  tokenOut: Token;
  chainId: number;
}

export interface Quote {
  pair: TokenPair;
  amountIn: string;
  amountOut: string;
  priceImpact: string;
  gasEstimate: string;
  route: RouteHop[];
}

export interface RouteHop {
  dex: string;
  fromToken: string;
  toToken: string;
  proportion: string;
}

export interface SwapRequest {
  tokenIn: string;
  tokenOut: string;
  amountIn: string;
  amountOutMin: string;
  recipient?: string;
  slippageTolerance?: string;
}

export interface SwapResponse {
  txHash: string;
  amountIn: string;
  amountOut: string;
  gasUsed: string;
  priceImpact: string;
}

export type OrderType = 'limit' | 'stop_loss' | 'take_profit' | 'market' | 'gtd' | 'ioc' | 'fok';
export type OrderStatus = 'pending' | 'partially_filled' | 'filled' | 'cancelled' | 'expired';
export type Side = 'buy' | 'sell';

export interface Order {
  id: string;
  owner: string;
  tokenIn: string;
  tokenOut: string;
  amountIn: string;
  amountOut: string;
  price: string;
  stopPrice?: string;
  orderType: OrderType;
  side: Side;
  status: OrderStatus;
  createdAt: string;
  expiresAt?: string;
  filledAmount: string;
}

export interface DCAPlan {
  id: string;
  owner: string;
  tokenIn: string;
  tokenOut: string;
  amountPerExecution: string;
  intervalSeconds: number;
  executionsCompleted: number;
  maxExecutions?: number;
  status: string;
  nextExecution: string;
  createdAt: string;
}

export interface Position {
  id: string;
  owner: string;
  collateralToken: string;
  indexToken: string;
  isLong: boolean;
  size: string;
  collateral: string;
  averagePrice: string;
  unrealizedPnl: string;
  liquidationPrice: string;
  status: string;
}

export interface PoolInfo {
  address: string;
  token0: string;
  token1: string;
  reserve0: string;
  reserve1: string;
  liquidity: string;
  feeTier: number;
}

export interface TokenBalance {
  token: Token;
  balance: string;
  balanceRaw: string;
  allowance: string;
}

export interface TransactionReceipt {
  txHash: string;
  blockNumber: number;
  status: boolean;
  gasUsed: string;
  logs: TransactionLog[];
}

export interface TransactionLog {
  address: string;
  topics: string[];
  data: string;
}

export interface GasEstimate {
  gasPrice: string;
  gasLimit: string;
  totalCost: string;
  token: string;
}

export interface NetworkStatus {
  chainId: number;
  blockNumber: number;
  synced: boolean;
  gasPrice: string;
}

export interface APIResponse<T> {
  success: boolean;
  data?: T;
  error?: string;
}

export class TigerSwapSDK {
  private baseUrl: string;
  private apiKey?: string;
  private timeout: number;

  constructor(config: SDKConfig = {}) {
    this.baseUrl = config.baseUrl || 'https://api.tigerswap.io';
    this.apiKey = config.apiKey;
    this.timeout = config.timeout || 30000;
  }

  private async request<T>(
    endpoint: string,
    options: RequestOptions = {}
  ): Promise<T> {
    const url = `${this.baseUrl}${endpoint}`;
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
    };

    if (this.apiKey) {
      headers['Authorization'] = `Bearer ${this.apiKey}`;
    }

    try {
      const response = await fetch(url, {
        method: options.method || 'GET',
        headers,
        body: options.body ? JSON.stringify(options.body) : undefined,
      });

      if (!response.ok) {
        throw new Error(`API Error: ${response.status}`);
      }

      const data: APIResponse<T> = await response.json();
      
      if (!data.success) {
        throw new Error(data.error || 'Unknown error');
      }

      return data.data as T;
    } catch (error) {
      throw new Error(`Request failed: ${error}`);
    }
  }

  // ============ Token & Balance ============

  async getBalance(owner: string, token: string): Promise<TokenBalance> {
    return this.request<TokenBalance>(`/v1/balance/${owner}`, {
      query: { token },
    });
  }

  async getBalances(owner: string): Promise<TokenBalance[]> {
    return this.request<TokenBalance[]>(`/v1/balance/${owner}`);
  }

  // ============ Quotes & Swap ============

  async getQuote(
    tokenIn: string,
    tokenOut: string,
    amountIn: string
  ): Promise<Quote> {
    return this.request<Quote>('/v1/quote', {
      query: { tokenIn, tokenOut, amountIn },
    });
  }

  async swap(request: SwapRequest): Promise<SwapResponse> {
    return this.request<SwapResponse>('/v1/swap', {
      method: 'POST',
      body: request,
    });
  }

  // ============ Orders ============

  async createOrder(order: Partial<Order>): Promise<Order> {
    return this.request<Order>('/v1/orders', {
      method: 'POST',
      body: order,
    });
  }

  async getOrder(orderId: string): Promise<Order> {
    return this.request<Order>(`/v1/orders/${orderId}`);
  }

  async cancelOrder(orderId: string): Promise<Order> {
    return this.request<Order>(`/v1/orders/${orderId}`, {
      method: 'DELETE',
    });
  }

  async getOrders(owner: string): Promise<Order[]> {
    return this.request<Order[]>(`/v1/orders`, {
      query: { owner },
    });
  }

  // ============ DCA ============

  async createDCAPlan(plan: Partial<DCAPlan>): Promise<DCAPlan> {
    return this.request<DCAPlan>('/v1/dca', {
      method: 'POST',
      body: plan,
    });
  }

  async getDCAPlan(planId: string): Promise<DCAPlan> {
    return this.request<DCAPlan>(`/v1/dca/${planId}`);
  }

  async cancelDCAPlan(planId: string): Promise<DCAPlan> {
    return this.request<DCAPlan>(`/v1/dca/${planId}`, {
      method: 'DELETE',
    });
  }

  // ============ Perpetuals ============

  async openPosition(position: Partial<Position>): Promise<Position> {
    return this.request<Position>('/v1/positions', {
      method: 'POST',
      body: position,
    });
  }

  async closePosition(positionId: string): Promise<Position> {
    return this.request<Position>(`/v1/positions/${positionId}`, {
      method: 'DELETE',
    });
  }

  async getPosition(positionId: string): Promise<Position> {
    return this.request<Position>(`/v1/positions/${positionId}`);
  }

  async getPositions(owner: string): Promise<Position[]> {
    return this.request<Position[]>(`/v1/positions`, {
      query: { owner },
    });
  }

  // ============ Pool Info ============

  async getPool(tokenA: string, tokenB: string): Promise<PoolInfo> {
    return this.request<PoolInfo>(`/v1/pool/${tokenA}/${tokenB}`);
  }

  // ============ Network ============

  async getNetworkStatus(chainId: number): Promise<NetworkStatus> {
    return this.request<NetworkStatus>(`/v1/network/${chainId}`);
  }

  async estimateGas(request: SwapRequest): Promise<GasEstimate> {
    return this.request<GasEstimate>('/v1/gas/estimate', {
      method: 'POST',
      body: request,
    });
  }
}

interface SDKConfig {
  baseUrl?: string;
  apiKey?: string;
  timeout?: number;
}

interface RequestOptions {
  method?: 'GET' | 'POST' | 'PUT' | 'DELETE';
  query?: Record<string, string>;
  body?: unknown;
}

export default TigerSwapSDK;