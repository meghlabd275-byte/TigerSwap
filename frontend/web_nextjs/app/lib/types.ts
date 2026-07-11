/**
 * TigerSwap Type Definitions
 * Production TypeScript types for the DEX
 * 
 * @author TigerSwap
 * @version 1.0.0 Production
 */

// Token types
export interface Token {
  address: string;
  symbol: string;
  name: string;
  decimals: number;
  logoUrl?: string;
  price: number;
  priceChange24h: number;
  volume24h: number;
  totalSupply?: string;
  isNative?: boolean;
}

export interface TokenBalance {
  token: Token;
  balance: string;
  balanceUSD: number;
  allowance?: string;
}

// Swap types
export interface Quote {
  fromToken: string;
  toToken: string;
  amountIn: bigint;
  amountOut: bigint;
  priceImpact: number;
  gasEstimate: bigint;
  gasFeeUSD: number;
  route: SwapRoute;
  txData?: string;
  txHash?: string;
  validUntil: number;
}

export interface SwapRoute {
  pools: Pool[];
  path: Token[];
  inputAmount: bigint;
  outputAmount: bigint;
  priceImpact: number;
}

export interface Pool {
  dex: string;
  poolAddress: string;
  tokenA: Token;
  tokenB: Token;
  reserveA: bigint;
  reserveB: bigint;
  liquidityUSD: number;
  fee: number;
}

export interface SwapParams {
  fromToken: string;
  toToken: string;
  amountIn: string;
  amountOutMinimum?: string;
  slippage: number;
  referrer?: string;
}

// Pool types
export interface Pool {
  address: string;
  token0: Token;
  token1: Token;
  reserve0: bigint;
  reserve1: bigint;
  totalSupply: bigint;
  tvl: number;
  volume24h: number;
  fee24h: number;
  apr: number;
  isStable: boolean;
}

// Order types
export interface Order {
  id: string;
  user: string;
  type: 'limit' | 'market';
  side: 'buy' | 'sell';
  fromToken: string;
  toToken: string;
  amountIn: string;
  amountOut: string;
  price?: string;
  status: 'pending' | 'filled' | 'cancelled' | 'expired';
  createdAt: number;
  filledAt?: number;
  expiresAt: number;
}

// Transaction types
export interface Transaction {
  hash: string;
  from: string;
  to: string;
  value: string;
  data: string;
  gasLimit: string;
  gasPrice: string;
  nonce: number;
  status: 'pending' | 'confirmed' | 'failed';
  timestamp: number;
  blockNumber: number;
}

// User types
export interface User {
  id: string;
  address: string;
  portfolio: Portfolio;
  orders: Order[];
  positions: Position[];
  transactions: Transaction[];
}

export interface Portfolio {
  totalValue: number;
  tokens: TokenBalance[];
  positions: Position[];
  pnl24h: number;
  pnl24hPercent: number;
}

export interface Position {
  pool: Pool;
  token0Balance: string;
  token1Balance: string;
  lpBalance: string;
  value: number;
  sharePercent: number;
  feesEarned: number;
  apr: number;
}

// Analytics types
export interface Analytics {
  volume24h: number;
  volume7d: number;
  volume30d: number;
  tvl: number;
  fees24h: number;
  trades24h: number;
  uniqueUsers24h: number;
}

export interface ChartData {
  timestamp: number;
  value: number;
}

export interface VolumeData extends ChartData {}
export interface TVLData extends ChartData {}
export interface PriceData extends ChartData {}

// Chain types
export interface Chain {
  id: number;
  name: string;
  symbol: string;
  logoUrl: string;
  rpcUrl: string;
  explorerUrl: string;
  isActive: boolean;
}

// API Response types
export interface APIResponse<T> {
  success: boolean;
  data: T;
  error?: string;
  timestamp: number;
}

export interface PaginatedResponse<T> {
  items: T[];
  page: number;
  pageSize: number;
  total: number;
  hasMore: boolean;
}

// Error types
export interface SwapError {
  code: string;
  message: string;
  details?: Record<string, unknown>;
}

export interface ValidationError {
  field: string;
  message: string;
}

// Config types
export interface SwapConfig {
  chains: Chain[];
  defaultSlippage: number;
  maxSlippage: number;
  deadline: number;
  enableMultihop: boolean;
  enableMEVProtection: boolean;
}

// Wallet types
export interface WalletState {
  address: string | null;
  chainId: number | null;
  isConnected: boolean;
  isConnecting: boolean;
  balance: string | null;
}

// WebSocket types
export interface WSMESSAGE {
  type: 'price' | 'trade' | 'pool' | 'order' | 'notification';
  data: unknown;
  timestamp: number;
}

export interface PriceUpdate {
  token: string;
  price: number;
  priceChange24h: number;
  volume24h: number;
  timestamp: number;
}

export interface TradeUpdate {
  hash: string;
  fromToken: string;
  toToken: string;
  fromAmount: string;
  toAmount: string;
  from: string;
  timestamp: number;
}
