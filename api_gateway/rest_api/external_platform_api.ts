/**
 * TigerSwap External Platform API
 * Complete API for external wallets, DEXs, and CEXs to connect to TigerSwap
 * Tier-based access with full management
 */

import { ethers, JsonRpcProvider, Wallet, Contract } from 'ethers';
import { ERC20_ABI, DEX_ROUTERS } from './constants';

// ============================================================================
// TYPES & INTERFACES
// ============================================================================

export interface ExternalPlatform {
  id: string;
  name: string;
  type: 'cex' | 'dex' | 'wallet' | 'protocol';
  apiKey: string;
  tier: 'free' | 'basic' | 'pro' | 'enterprise';
  isActive: boolean;
  permissions: {
    canTrade: boolean;
    canSwap: boolean;
    canAddLiquidity: boolean;
    canBridge: boolean;
    canCreateToken: boolean;
  };
  rateLimitPerMin: number;
  monthlyFeeUsd: number;
  createdAt: number;
}

export interface TradingRequest {
  platform: string;
  symbol: string;
  side: 'buy' | 'sell';
  type: 'market' | 'limit';
  amount: string;
  price?: string;
}

export interface SwapRequest {
  platform: string;
  chainId: number;
  tokenIn: string;
  tokenOut: string;
  amountIn: string;
  slippage: number;
}

export interface LiquidityRequest {
  platform: string;
  chainId: number;
  tokenA: string;
  tokenB: string;
  amountA: string;
  amountB: string;
}

export interface ApiResponse<T = any> {
  success: boolean;
  data?: T;
  error?: string;
  message?: string;
}

// ============================================================================
// TIER CONFIGURATIONS
// ============================================================================

const TIER_CONFIG = {
  free: {
    name: 'free',
    monthlyFeeUsd: 0,
    maxApiCallsPerMin: 60,
    maxDailyVolume: 10000,
    maxPositions: 3,
    features: {
      canTrade: true,
      canSwap: false,
      canAddLiquidity: false,
      canBridge: false,
      canCreateToken: false,
    },
  },
  basic: {
    name: 'basic',
    monthlyFeeUsd: 99,
    maxApiCallsPerMin: 300,
    maxDailyVolume: 100000,
    maxPositions: 10,
    features: {
      canTrade: true,
      canSwap: true,
      canAddLiquidity: false,
      canBridge: false,
      canCreateToken: false,
    },
  },
  pro: {
    name: 'pro',
    monthlyFeeUsd: 299,
    maxApiCallsPerMin: 1000,
    maxDailyVolume: 1000000,
    maxPositions: 50,
    features: {
      canTrade: true,
      canSwap: true,
      canAddLiquidity: true,
      canBridge: true,
      canCreateToken: false,
    },
  },
  enterprise: {
    name: 'enterprise',
    monthlyFeeUsd: 999,
    maxApiCallsPerMin: 10000,
    maxDailyVolume: 10000000,
    maxPositions: 200,
    features: {
      canTrade: true,
      canSwap: true,
      canAddLiquidity: true,
      canBridge: true,
      canCreateToken: true,
    },
  },
};

// ============================================================================
// EXTERNAL API CLIENT
// ============================================================================

export class TigerSwapExternalApi {
  private apiKey: string;
  private baseUrl: string;
  private tier: string = 'free';
  private apiCallsThisMinute: number = 0;
  private lastResetMinute: number = 0;

  constructor(apiKey: string, baseUrl: string = 'https://api.tigerswap.io') {
    this.apiKey = apiKey;
    this.baseUrl = baseUrl;
    this.lastResetMinute = Math.floor(Date.now() / 60000);
  }

  /**
   * Set tier
   */
  setTier(tier: string): void {
    if (TIER_CONFIG[tier as keyof typeof TIER_CONFIG]) {
      this.tier = tier;
    }
  }

  /**
   * Check rate limit
   */
  private checkRateLimit(): boolean {
    const currentMinute = Math.floor(Date.now() / 60000);
    if (currentMinute !== this.lastResetMinute) {
      this.apiCallsThisMinute = 0;
      this.lastResetMinute = currentMinute;
    }

    const tierConfig = TIER_CONFIG[this.tier as keyof typeof TIER_CONFIG];
    return this.apiCallsThisMinute < tierConfig.maxApiCallsPerMin;
  }

  /**
   * Make API request
   */
  private async request<T>(
    endpoint: string,
    method: string = 'GET',
    body?: any
  ): Promise<ApiResponse<T>> {
    if (!this.checkRateLimit()) {
      return {
        success: false,
        error: 'Rate limit exceeded',
      };
    }

    this.apiCallsThisMinute++;

    try {
      const response = await fetch(`${this.baseUrl}${endpoint}`, {
        method,
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${this.apiKey}`,
        },
        body: body ? JSON.stringify(body) : undefined,
      });

      const data = await response.json();
      return data;
    } catch (error: any) {
      return {
        success: false,
        error: error.message,
      };
    }
  }

  /**
   * Get account balance
   */
  async getBalance(chainId: number): Promise<ApiResponse<{ address: string; balance: string }>> {
    return this.request(`/v1/balance?chain_id=${chainId}`);
  }

  /**
   * Get token balance
   */
  async getTokenBalance(
    chainId: number,
    tokenAddress: string
  ): Promise<ApiResponse<{ balance: string }>> {
    return this.request(`/v1/token-balance?chain_id=${chainId}&token=${tokenAddress}`);
  }

  /**
   * Execute trade on external CEX
   */
  async executeTrade(request: TradingRequest): Promise<ApiResponse<{ orderId: string; txHash: string }>> {
    const tierConfig = TIER_CONFIG[this.tier as keyof typeof TIER_CONFIG];
    if (!tierConfig.features.canTrade) {
      return { success: false, error: 'Trading not permitted on this tier' };
    }

    return this.request('/v1/trading/execute', 'POST', request);
  }

  /**
   * Execute swap on external DEX
   */
  async executeSwap(request: SwapRequest): Promise<ApiResponse<{ txHash: string }>> {
    const tierConfig = TIER_CONFIG[this.tier as keyof typeof TIER_CONFIG];
    if (!tierConfig.features.canSwap) {
      return { success: false, error: 'Swapping not permitted on this tier' };
    }

    return this.request('/v1/swap/execute', 'POST', request);
  }

  /**
   * Add liquidity
   */
  async addLiquidity(request: LiquidityRequest): Promise<ApiResponse<{ lpTokenId: string }>> {
    const tierConfig = TIER_CONFIG[this.tier as keyof typeof TIER_CONFIG];
    if (!tierConfig.features.canAddLiquidity) {
      return { success: false, error: 'Adding liquidity not permitted on this tier' };
    }

    return this.request('/v1/liquidity/add', 'POST', request);
  }

  /**
   * Remove liquidity
   */
  async removeLiquidity(
    lpTokenId: string,
    amount: string
  ): Promise<ApiResponse<{ txHash: string }>> {
    return this.request('/v1/liquidity/remove', 'POST', { lpTokenId, amount });
  }

  /**
   * Get swap quote
   */
  async getSwapQuote(
    tokenIn: string,
    tokenOut: string,
    amountIn: string,
    chainId: number = 1
  ): Promise<ApiResponse<{
    outputAmount: string;
    priceImpact: number;
    route: string[];
    fee: number;
  }>> {
    return this.request('/v1/swap/quote', 'POST', {
      token_in: tokenIn,
      token_out: tokenOut,
      amount_in: amountIn,
      chain_id: chainId,
    });
  }

  /**
   * Get pool info
   */
  async getPoolInfo(
    tokenA: string,
    tokenB: string,
    chainId: number = 1
  ): Promise<ApiResponse<{
    liquidity: string;
    tokenABalance: string;
    tokenBBalance: string;
    apr: number;
  }>> {
    return this.request(`/v1/pool/info?token_a=${tokenA}&token_b=${tokenB}&chain_id=${chainId}`);
  }

  /**
   * Create new token
   */
  async createToken(
    name: string,
    symbol: string,
    totalSupply: string,
    decimals: number = 18
  ): Promise<ApiResponse<{ tokenAddress: string }>> {
    const tierConfig = TIER_CONFIG[this.tier as keyof typeof TIER_CONFIG];
    if (!tierConfig.features.canCreateToken) {
      return { success: false, error: 'Token creation not permitted on this tier' };
    }

    return this.request('/v1/token/create', 'POST', {
      name,
      symbol,
      total_supply: totalSupply,
      decimals,
    });
  }

  /**
   * Bridge tokens cross-chain
   */
  async bridge(
    fromChainId: number,
    toChainId: number,
    token: string,
    amount: string
  ): Promise<ApiResponse<{ txHash: string; bridgeTime: number }>> {
    const tierConfig = TIER_CONFIG[this.tier as keyof typeof TIER_CONFIG];
    if (!tierConfig.features.canBridge) {
      return { success: false, error: 'Bridging not permitted on this tier' };
    }

    return this.request('/v1/bridge/execute', 'POST', {
      from_chain_id: fromChainId,
      to_chain_id: toChainId,
      token,
      amount,
    });
  }

  /**
   * Get supported chains
   */
  async getSupportedChains(): Promise<ApiResponse<any[]>> {
    return this.request('/v1/chains');
  }

  /**
   * Get supported tokens
   */
  async getSupportedTokens(chainId: number): Promise<ApiResponse<any[]>> {
    return this.request(`/v1/tokens?chain_id=${chainId}`);
  }

  /**
   * Get supported DEXs
   */
  async getSupportedDEXs(): Promise<ApiResponse<any[]>> {
    return this.request('/v1/dexs');
  }

  /**
   * Get supported CEXs
   */
  async getSupportedCEXs(): Promise<ApiResponse<any[]>> {
    return this.request('/v1/cexs');
  }

  /**
   * Get transaction history
   */
  async getTransactionHistory(
    chainId?: number,
    limit: number = 50
  ): Promise<ApiResponse<any[]>> {
    return this.request(`/v1/history?chain_id=${chainId || ''}&limit=${limit}`);
  }

  /**
   * Get gas estimate
   */
  async getGasEstimate(chainId: number): Promise<ApiResponse<{
    slow: string;
    standard: string;
    fast: string;
  }>> {
    return this.request(`/v1/gas?chain_id=${chainId}`);
  }

  /**
   * Get account info
   */
  async getAccountInfo(): Promise<ApiResponse<{
    address: string;
    tier: string;
    permissions: any;
    usage: {
      apiCallsToday: number;
      dailyVolume: string;
      positions: number;
    };
  }>> {
    return this.request('/v1/account/info');
  }
}

// ============================================================================
// EXAMPLE: CONNECTING EXTERNAL WALLET TO TIGERSWAP
// ============================================================================

/**
 * Example: External wallet connects to TigerSwap
 */
async function exampleExternalWalletConnection() {
  // External wallet gets API key from TigerSwap admin
  const apiKey = 'tiger_xxxxxxxxxxxx';

  // Create TigerSwap API client
  const tigerApi = new TigerSwapExternalApi(apiKey);

  // Set tier (based on what user paid for)
  tigerApi.setTier('pro');

  // Get account info
  const accountInfo = await tigerApi.getAccountInfo();
  console.log('Account:', accountInfo);

  // Get swap quote
  const quote = await tigerApi.getSwapQuote(
    '0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48', // USDC
    '0xC02aaA39b223FE8D0A0e5C4F27eADf3F02f60fDD', // WETH
    '1000' // 1000 USDC
  );
  console.log('Quote:', quote);

  // Execute swap
  if (quote.success && quote.data) {
    const swapResult = await tigerApi.executeSwap({
      platform: 'tigerswap',
      chainId: 1,
      tokenIn: '0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48',
      tokenOut: '0xC02aaA39b223FE8D0A0e5C4F27eADf3F02f60fDD',
      amountIn: '1000',
      slippage: 0.5,
    });
    console.log('Swap:', swapResult);
  }

  // Add liquidity
  const liquidityResult = await tigerApi.addLiquidity({
    platform: 'tigerswap',
    chainId: 1,
    tokenA: '0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48',
    tokenB: '0xC02aaA39b223FE8D0A0e5C4F27eADf3F02f60fDD',
    amountA: '5000',
    amountB: '2',
  });
  console.log('Liquidity:', liquidityResult);
}

// ============================================================================
// EXAMPLE: CONNECTING TIGERSWAP TO EXTERNAL CEX
// ============================================================================

/**
 * Example: TigerSwap connects to external CEX using API keys
 */
async function exampleConnectToExternalCEX() {
  // In production, this would be called by admin API
  const response = await fetch('https://api.tigerswap.io/v1/admin/external/connections', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': 'Bearer $ADMIN_API_KEY',
    },
    body: JSON.stringify({
      platform_name: 'binance',
      platform_type: 'cex',
      can_trade: true,
      can_swap: false,
      can_add_liquidity: false,
      tier: 'enterprise',
    }),
  });

  const result = await response.json();
  console.log('Connection created:', result);

  // Use the API key to trade on Binance
  if (result.success && result.api_key) {
    const binanceApi = new TigerSwapExternalApi(result.api_key);

    // Execute trade on Binance
    const tradeResult = await binanceApi.executeTrade({
      platform: 'binance',
      symbol: 'BTC/USDT',
      side: 'buy',
      type: 'market',
      amount: '0.1',
    });
    console.log('Trade:', tradeResult);
  }
}

// ============================================================================
// EXAMPLE: CONNECTING TIGERSWAP TO EXTERNAL DEX
// ============================================================================

/**
 * Example: TigerSwap connects to external DEX
 */
async function exampleConnectToExternalDEX() {
  // Admin creates DEX connection
  const response = await fetch('https://api.tigerswap.io/v1/admin/external/connections', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Authorization': 'Bearer $ADMIN_API_KEY',
    },
    body: JSON.stringify({
      platform_name: 'uniswap',
      platform_type: 'dex',
      can_trade: false,
      can_swap: true,
      can_add_liquidity: true,
      tier: 'pro',
    }),
  });

  const result = await response.json();
  console.log('DEX connection:', result);

  // Use to swap on Uniswap
  if (result.success && result.api_key) {
    const dexApi = new TigerSwapExternalApi(result.api_key);

    const swapResult = await dexApi.executeSwap({
      platform: 'uniswap',
      chainId: 1,
      tokenIn: '0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48', // USDC
      tokenOut: '0xC02aaA39b223FE8D0A0e5C4F27eADf3F02f60fDD', // WETH
      amountIn: '10000',
      slippage: 0.5,
    });
    console.log('Uniswap swap:', swapResult);
  }
}

// ============================================================================
// EXPORTS
// ============================================================================

export default TigerSwapExternalApi;
export { TIER_CONFIG };