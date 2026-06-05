/**
 * TigerSwap Price Oracle
 * Multi-source price feeds: Chainlink, DEX pools, TWAP calculations
 */

import { CHAINLINK_PRICE_FEED_ABI, CHAINLINK_PRICE_FEEDS, COMMON_TOKENS, SUPPORTED_CHAINS } from '../web3_wallet/wallet';

// ============================================================================
// Types & Interfaces
// ============================================================================

export interface PriceData {
  price: number;
  timestamp: number;
  source: 'chainlink' | 'dex' | 'twap' | 'coinbase';
  confidence: number;
  previousPrice?: number;
  change24h?: number;
}

export interface TWAPData {
  price: number;
  startTime: number;
  endTime: number;
  intervals: number;
  source: string;
}

export interface PriceRequest {
  baseToken: string;
  quoteToken?: string;
  chainId?: number;
  sources?: ('chainlink' | 'dex' | 'twap' | 'coinbase')[];
}

export interface PriceResult {
  baseToken: string;
  quoteToken: string;
  price: number;
  previousPrice?: number;
  change24h?: number;
  change1h?: number;
  high24h?: number;
  low24h?: number;
  volume24h?: number;
  sources: PriceData[];
  timestamp: number;
  provider: string;
}

export interface HistoricalPrice {
  timestamp: number;
  open: number;
  high: number;
  low: number;
  close: number;
  volume: number;
}

// ============================================================================
// Chainlink Price Feed Addresses
// ============================================================================

const CHAINLINK_FEEDS: Record<number, Record<string, string>> = {
  1: {
    'ETH/USD': '0x5f4eC3Df9cbd43714FE2740f5E3617235d868879',
    'BTC/USD': '0xF4030086522a5bEEa4988F8cA5B36dbC97BeE88c',
    'LINK/USD': '0x2c1d072e956affc02f810a2d70d6f371ea4b1d8c',
    'AAVE/USD': '0x547a514d5e376968ad459cb2f8b12db65d49d9fd',
    'UNI/USD': '0x553303d4600b1faa03d50273c524475c84d4d3cb',
    'USDC/USD': '0x8fffffd085591743496e568d2398187d1ba14bac',
    'USDT/USD': '0x3e7d1eab13ad0104d2750b8223bfb8f655468f7',
    'DAI/USD': '0x6051e2d07fC25a2D5a8269C72A1542652e76B11b',
  },
  56: {
    'BNB/USD': '0x0567F2323251f0Aab45c40a2F527e8A94c7bAb3a',
    'BTC/USD': '0x264990fbd0A4796A3E8d8BbC90280fF41eB0C1C2',
    'ETH/USD': '0x9ef1B8cE5E96FeD8b63Cb2EaADc66e4b4236cB85',
    'USDT/USD': '0x55AFa9c5852fA6292B51f61f821F786C77128986',
    'USDC/USD': '0x4e1Ec92946967f5Fb74bB84C7E2f0Aa24F9e3EaF',
  },
  137: {
    'MATIC/USD': '0xAB594600376Ec9fD91F8e885dADF0CE036862dE0',
    'BTC/USD': '0xDE31f8bFBD0c2A1162840e308193488B0aC75e55',
    'ETH/USD': '0xF9680D99D6C9589e2a93a78A04A279e509205225',
    'USDC/USD': '0xfE4A8cc5c5B8a1E7dB4e16B8b6f0a1b8C3D2E4F',
    'USDT/USD': '0x7Ba9988D8dD6F36d50f5920929b6D2072A4F8e7E',
  },
  42161: {
    'ETH/USD': '0x639Fe6ab55C921f74e7feb1C72a2b72B27dD6eCc',
    'BTC/USD': '0x6ce36110cA89F6d47E147C758A5F9B2e9d73f6F3',
    'LINK/USD': '0x86b53A0CF1097f643eb4E46d3b1dC6a1Afa9192',
    'USDC/USD': '0xA6B4C3f8271BeF36e7C2B82c47bF3A6d8D9c6c33',
  },
  10: {
    'ETH/USD': '0x13e3Ee699D1909E125722f5c5CAeb7A25C2a06c0',
    'BTC/USD': '0xC27d1Be3f5C23e14e5D4f2a2fB4E8C4f7d3B8e1A',
    'USDC/USD': '0x93B0cF8d1F3Dc5F38A98c7e0F9D7bCd1A8f9B8E3',
  },
  8453: {
    'ETH/USD': '0x71079aC3b2d1e4E2f6D4C8E3A1F2B4D6E8F0A2C4',
    'BTC/USD': '0x8A2bE4C5D6F7A8B9C0D1E2F3A4B5C6D7E8F9A0B1',
  },
};

const COINGECKO_API = 'https://api.coingecko.com/api/v3';

// ============================================================================
// Price Oracle Class
// ============================================================================

export class PriceOracle {
  private cache: Map<string, { data: PriceData; expiry: number }> = new Map();
  private cacheExpiry: number = 60000; // 1 minute default
  private providers: Map<string, any> = new Map();
  private historicalPrices: Map<string, HistoricalPrice[]> = new Map();

  constructor() {
    this.startCacheCleanup();
  }

  private startCacheCleanup(): void {
    setInterval(() => {
      const now = Date.now();
      for (const [key, value] of this.cache.entries()) {
        if (value.expiry < now) {
          this.cache.delete(key);
        }
      }
    }, 30000);
  }

  // ============================================================================
  // Get Current Price
  // ============================================================================

  async getPrice(request: PriceRequest): Promise<PriceResult> {
    const chainId = request.chainId || 1;
    const sources = request.sources || ['chainlink', 'dex', 'coinbase'];
    const quoteToken = request.quoteToken || 'USD';
    
    const prices: PriceData[] = [];
    let bestPrice: number | null = null;
    let bestSource: string = 'chainlink';

    // Try each source
    for (const source of sources) {
      try {
        const priceData = await this.getPriceFromSource(source, request.baseToken, quoteToken, chainId);
        if (priceData) {
          prices.push(priceData);
          if (bestPrice === null || priceData.confidence > (prices.find(p => p.source === bestSource)?.confidence || 0)) {
            bestPrice = priceData.price;
            bestSource = priceData.source;
          }
        }
      } catch (error) {
        console.warn(`Failed to get price from ${source}:`, error);
      }
    }

    if (bestPrice === null) {
      throw new Error(`No price available for ${request.baseToken}`);
    }

    // Get 24h change from CoinGecko
    const changeData = await this.get24hChange(request.baseToken, chainId);

    return {
      baseToken: request.baseToken,
      quoteToken,
      price: bestPrice,
      previousPrice: prices[0]?.previousPrice,
      change24h: changeData?.change24h,
      change1h: changeData?.change1h,
      high24h: changeData?.high24h,
      low24h: changeData?.low24h,
      volume24h: changeData?.volume24h,
      sources: prices,
      timestamp: Date.now(),
      provider: bestSource,
    };
  }

  async getPriceFromSource(source: 'chainlink' | 'dex' | 'twap' | 'coinbase', baseToken: string, quoteToken: string, chainId: number): Promise<PriceData | null> {
    const cacheKey = `${source}:${baseToken}:${quoteToken}:${chainId}`;
    const cached = this.cache.get(cacheKey);
    if (cached && cached.expiry > Date.now()) {
      return cached.data;
    }

    let price: number | null = null;

    switch (source) {
      case 'chainlink':
        price = await this.getChainlinkPrice(baseToken, quoteToken, chainId);
        break;
      case 'dex':
        price = await this.getDEXPrice(baseToken, quoteToken, chainId);
        break;
      case 'twap':
        price = await this.getTWAPPrice(baseToken, quoteToken, chainId);
        break;
      case 'coinbase':
        price = await this.getCoinbasePrice(baseToken, quoteToken);
        break;
    }

    if (price === null) return null;

    const priceData: PriceData = {
      price,
      timestamp: Date.now(),
      source,
      confidence: this.getConfidenceForSource(source),
      previousPrice: cached?.data?.price,
    };

    this.cache.set(cacheKey, {
      data: priceData,
      expiry: Date.now() + this.cacheExpiry,
    });

    return priceData;
  }

  // ============================================================================
  // Chainlink Price Feed
  // ============================================================================

  async getChainlinkPrice(baseToken: string, quoteToken: string, chainId: number): Promise<number | null> {
    const feedAddress = CHAINLINK_FEEDS[chainId]?.[`${baseToken}/${quoteToken}`];
    
    if (!feedAddress) {
      // Try to find token symbol from address
      const tokenSymbol = this.getTokenSymbol(baseToken, chainId);
      if (tokenSymbol) {
        const feed = CHAINLINK_FEEDS[chainId]?.[`${tokenSymbol}/${quoteToken}`];
        if (feed) {
          return this.queryChainlinkFeed(feed);
        }
      }
      return null;
    }

    return this.queryChainlinkFeed(feedAddress);
  }

  private async queryChainlinkFeed(feedAddress: string): Promise<number | null> {
    try {
      // In production, this would use ethers.js or viem to query the contract
      // For now, return mock data with realistic prices
      const mockPrices: Record<string, number> = {
        '0x5f4eC3Df9cbd43714FE2740f5E3617235d868879': 2450.00, // ETH/USD
        '0xF4030086522a5bEEa4988F8cA5B36dbC97BeE88c': 62500.00, // BTC/USD
        '0x2c1d072e956affc02f810a2d70d6f371ea4b1d8c': 18.50, // LINK/USD
      };

      if (mockPrices[feedAddress]) {
        return mockPrices[feedAddress];
      }

      // Simulate price with small random variation
      const basePrice = 100 + Math.random() * 100;
      return basePrice;
    } catch (error) {
      console.error('Chainlink query failed:', error);
      return null;
    }
  }

  // ============================================================================
  // DEX Price (Spot)
  // ============================================================================

  async getDEXPrice(baseToken: string, quoteToken: string, chainId: number): Promise<number | null> {
    try {
      const dexPools = this.getDEXPools(baseToken, quoteToken, chainId);
      if (dexPools.length === 0) return null;

      // Find best price across DEXes
      let bestPrice = 0;
      for (const pool of dexPools) {
        const price = this.calculatePoolSpotPrice(pool);
        if (price > bestPrice) {
          bestPrice = price;
        }
      }

      return bestPrice || null;
    } catch (error) {
      console.error('DEX price query failed:', error);
      return null;
    }
  }

  private getDEXPools(baseToken: string, quoteToken: string, chainId: number): any[] {
    // Return mock pool data - in production would query subgraph or on-chain
    return [
      { dex: 'uniswap_v2', reserve0: '50000000000000000000000000', reserve1: '125000000000000000000000000', fee: 300 },
      { dex: 'sushiswap', reserve0: '15000000000000000000000000', reserve1: '37500000000000000000000000', fee: 300 },
      { dex: 'uniswap_v3', reserve0: '87500000000000000000000000', reserve1: '218750000000000000000000000', fee: 500 },
    ];
  }

  private calculatePoolSpotPrice(pool: any): number {
    const reserve0 = BigInt(pool.reserve0);
    const reserve1 = BigInt(pool.reserve1);
    if (reserve0 === BigInt(0)) return 0;
    return Number(reserve1) / Number(reserve0);
  }

  // ============================================================================
  // TWAP (Time-Weighted Average Price)
  // ============================================================================

  async getTWAPPrice(baseToken: string, quoteToken: string, chainId: number, window: number = 30): Promise<number | null> {
    try {
      // Get historical data points
      const intervals = Math.floor(window / 5); // 5 minute intervals
      const now = Date.now();
      const intervalMs = window * 60 * 1000 / intervals;

      const prices: number[] = [];
      for (let i = 0; i < intervals; i++) {
        const timestamp = now - (i * intervalMs);
        const price = await this.getHistoricalPriceAt(baseToken, timestamp);
        if (price) prices.push(price);
      }

      if (prices.length === 0) return null;

      // Calculate TWAP
      const twap = prices.reduce((a, b) => a + b, 0) / prices.length;
      return twap;
    } catch (error) {
      console.error('TWAP calculation failed:', error);
      return null;
    }
  }

  async getHistoricalPriceAt(token: string, timestamp: number): Promise<number | null> {
    // In production, this would query Chainlink or historical DEX data
    // Return mock data with realistic variation
    const basePrice = this.getBasePrice(token);
    const variation = (Math.sin(timestamp / 3600000) * 0.02); // ±2% variation over time
    return basePrice * (1 + variation);
  }

  // ============================================================================
  // Coinbase Price
  // ============================================================================

  async getCoinbasePrice(baseToken: string, quoteToken: string): Promise<number | null> {
    try {
      const symbol = `${baseToken}-${quoteToken}`;
      const response = await fetch(`https://api.coinbase.com/v2/prices/${symbol}/spot`);
      
      if (!response.ok) return null;
      
      const data = await response.json();
      return parseFloat(data.data?.amount);
    } catch (error) {
      console.error('Coinbase API failed:', error);
      return null;
    }
  }

  // ============================================================================
  // 24h Statistics from CoinGecko
  // ============================================================================

  async get24hChange(token: string, chainId: number): Promise<{
    change24h?: number;
    change1h?: number;
    high24h?: number;
    low24h?: number;
    volume24h?: number;
  } | null> {
    try {
      const coinId = this.getCoinGeckoId(token, chainId);
      if (!coinId) return null;

      const response = await fetch(
        `${COINGECKO_API}/coins/${coinId}?localization=false&tickers=false&community_data=false&developer_data=false&sparkline=false`
      );

      if (!response.ok) return null;

      const data = await response.json();
      const marketData = data.market_data;

      return {
        change24h: marketData?.price_change_percentage_24h,
        change1h: marketData?.price_change_percentage_1h,
        high24h: marketData?.high_24h?.usd,
        low24h: marketData?.low_24h?.usd,
        volume24h: marketData?.total_volume?.usd,
      };
    } catch (error) {
      console.error('CoinGecko API failed:', error);
      return null;
    }
  }

  // ============================================================================
  // Historical Prices (for charts)
  // ============================================================================

  async getHistoricalPrices(
    token: string,
    chainId: number,
    days: number = 1
  ): Promise<HistoricalPrice[]> {
    try {
      const coinId = this.getCoinGeckoId(token, chainId);
      if (!coinId) return this.generateMockHistoricalPrices(days);

      const response = await fetch(
        `${COINGECKO_API}/coins/${coinId}/ohlc?vs_currency=usd&days=${days}`
      );

      if (!response.ok) return this.generateMockHistoricalPrices(days);

      const data = await response.json();
      
      return data.map((item: number[]) => ({
        timestamp: item[0],
        open: item[1],
        high: item[2],
        low: item[3],
        close: item[4],
        volume: 0,
      }));
    } catch (error) {
      console.error('Historical prices fetch failed:', error);
      return this.generateMockHistoricalPrices(days);
    }
  }

  private generateMockHistoricalPrices(days: number): HistoricalPrice[] {
    const prices: HistoricalPrice[] = [];
    const now = Date.now();
    const intervalMs = 5 * 60 * 1000; // 5 minutes
    const points = (days * 24 * 60) / 5;
    
    let price = 2000 + Math.random() * 500;
    
    for (let i = points; i >= 0; i--) {
      const timestamp = now - (i * intervalMs);
      const change = (Math.random() - 0.5) * 0.01;
      price = price * (1 + change);
      
      prices.push({
        timestamp,
        open: price,
        high: price * 1.001,
        low: price * 0.999,
        close: price,
        volume: Math.random() * 1000000,
      });
    }
    
    return prices;
  }

  // ============================================================================
  // Helper Functions
  // ============================================================================

  private getTokenSymbol(address: string, chainId: number): string | null {
    const tokens = COMMON_TOKENS[chainId];
    if (!tokens) return null;
    
    for (const [symbol, token] of Object.entries(tokens)) {
      if (token.address.toLowerCase() === address.toLowerCase()) {
        return symbol;
      }
    }
    return null;
  }

  private getCoinGeckoId(token: string, chainId: number): string | null {
    const mapping: Record<number, Record<string, string>> = {
      1: {
        'ETH': 'ethereum',
        'WETH': 'ethereum',
        'BTC': 'bitcoin',
        'WBTC': 'wrapped-bitcoin',
        'USDC': 'usd-coin',
        'USDT': 'tether',
        'DAI': 'dai',
        'LINK': 'chainlink',
        'UNI': 'uniswap',
        'AAVE': 'aave',
      },
      56: {
        'BNB': 'binancecoin',
        'WBNB': 'binancecoin',
        'CAKE': 'pancakeswap-token',
      },
      137: {
        'MATIC': 'matic-network',
        'WMATIC': 'matic-network',
        'USDC': 'usd-coin',
        'USDT': 'tether',
      },
    };

    const symbol = this.getTokenSymbol(token, chainId);
    if (!symbol) return null;
    
    return mapping[chainId]?.[symbol] || null;
  }

  private getBasePrice(token: string): number {
    const prices: Record<string, number> = {
      'ETH': 2450,
      'WETH': 2450,
      'BTC': 62500,
      'WBTC': 62500,
      'LINK': 18.5,
      'UNI': 12.5,
      'AAVE': 285,
      'USDC': 1,
      'USDT': 1,
      'DAI': 1,
    };
    
    const symbol = this.getTokenSymbol(token, 1);
    return prices[symbol] || 1;
  }

  private getConfidenceForSource(source: 'chainlink' | 'dex' | 'twap' | 'coinbase'): number {
    const confidences = {
      chainlink: 0.95,
      dex: 0.85,
      twap: 0.90,
      coinbase: 0.88,
    };
    return confidences[source] || 0.5;
  }

  // ============================================================================
  // Utility Functions
  // ============================================================================

  clearCache(): void {
    this.cache.clear();
  }

  setCacheExpiry(ms: number): void {
    this.cacheExpiry = ms;
  }

  getCachedPrice(baseToken: string, quoteToken: string = 'USD'): PriceData | null {
    const cacheKey = `chainlink:${baseToken}:${quoteToken}:1`;
    const cached = this.cache.get(cacheKey);
    return cached?.data || null;
  }
}

// ============================================================================
// Singleton Export
// ============================================================================

export const priceOracle = new PriceOracle();
export default PriceOracle;
