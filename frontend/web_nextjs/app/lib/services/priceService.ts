// Price service for real-time token prices
// Uses CoinGecko API for price data

export interface PriceData {
  symbol: string;
  price: number;
  change24h: number;
  high24h: number;
  low24h: number;
  volume24h: number;
  marketCap: number;
  lastUpdated: Date;
}

export interface SwapQuote {
  fromToken: string;
  toToken: string;
  fromAmount: string;
  toAmount: string;
  priceImpact: number;
  route: string[];
  gasEstimate: string;
}

// Token to CoinGecko ID mapping
const TOKEN_TO_COINGECKO: Record<string, string> = {
  'ETH': 'ethereum',
  'USDT': 'tether',
  'USDC': 'usd-coin',
  'WBTC': 'wrapped-bitcoin',
  'BNB': 'binancecoin',
  'SOL': 'solana',
  'MATIC': 'matic-network',
  'DAI': 'dai',
  'ARB': 'arbitrum',
  'OPT': 'optimism',
  'AVAX': 'avalanche-2',
  'FTM': 'fantom',
  'CRV': 'curve-dao-token',
  'LINK': 'chainlink',
  'UNI': 'uniswap',
  'AAVE': 'aave',
  'MKR': 'maker',
  'SNX': 'havven',
  'LDO': 'lido-dao',
  'PEPE': 'pepe',
  'SHIB': 'shiba-inu',
  'DOGE': 'dogecoin',
};

// Real-time price cache
const priceCache: Map<string, PriceData> = new Map();
let lastFetchTime: number = 0;
const CACHE_DURATION = 30000; // 30 seconds

/**
 * Fetch real-time prices from CoinGecko
 */
export async function fetchPrices(): Promise<Map<string, PriceData>> {
  const now = Date.now();
  
  // Return cached prices if still valid
  if (now - lastFetchTime < CACHE_DURATION && priceCache.size > 0) {
    return priceCache;
  }

  try {
    const ids = Object.values(TOKEN_TO_COINGECKO).join(',');
    const response = await fetch(
      `https://api.coingecko.com/api/v3/simple/price?ids=${ids}&vs_currencies=usd&include_24hr_change=true&include_24hr_vol=true&include_market_cap=true`
    );

    if (!response.ok) {
      throw new Error(`CoinGecko API error: ${response.status}`);
    }

    const data = await response.json();
    
    // Update cache
    for (const [symbol, coingeckoId] of Object.entries(TOKEN_TO_COINGECKO)) {
      if (data[coingeckoId]) {
        const priceData: PriceData = {
          symbol: symbol.toUpperCase(),
          price: data[coingeckoId].usd || 0,
          change24h: data[coingeckoId].usd_24h_change || 0,
          high24h: data[coingeckoId].usd_24h_high || data[coingeckoId].usd || 0,
          low24h: data[coingeckoId].usd_24h_low || data[coingeckoId].usd || 0,
          volume24h: data[coingeckoId].usd_24h_vol || 0,
          marketCap: data[coingeckoId].usd_market_cap || 0,
          lastUpdated: new Date(),
        };
        priceCache.set(symbol.toUpperCase(), priceData);
      }
    }

    lastFetchTime = now;
    return priceCache;
  } catch (error) {
    console.error('Failed to fetch prices:', error);
    // Return cached data if available, otherwise return empty
    return priceCache;
  }
}

/**
 * Get price for a specific token
 */
export async function getTokenPrice(symbol: string): Promise<number> {
  const prices = await fetchPrices();
  const upperSymbol = symbol.toUpperCase();
  
  if (prices.has(upperSymbol)) {
    return prices.get(upperSymbol)!.price;
  }
  
  // Fallback to mock price for unknown tokens
  return 0;
}

/**
 * Calculate swap output based on real prices
 */
export async function calculateSwapOutput(
  fromSymbol: string,
  toSymbol: string,
  amount: string
): Promise<SwapQuote | null> {
  const prices = await fetchPrices();
  const fromUpper = fromSymbol.toUpperCase();
  const toUpper = toSymbol.toUpperCase();

  const fromPrice = prices.get(fromUpper);
  const toPrice = prices.get(toUpper);

  if (!fromPrice || !toPrice) {
    console.error(`Price not found for ${fromSymbol} or ${toSymbol}`);
    return null;
  }

  const fromAmountNum = parseFloat(amount);
  if (isNaN(fromAmountNum) || fromAmountNum <= 0) {
    return null;
  }

  // Calculate output based on real prices
  const rate = fromPrice.price / toPrice.price;
  const toAmountNum = fromAmountNum * rate;
  
  // Calculate price impact (simplified)
  const priceImpact = Math.abs((rate - 1) / 1) * 100;

  return {
    fromToken: fromUpper,
    toToken: toUpper,
    fromAmount: amount,
    toAmount: toAmountNum.toFixed(toPrice.price < 1 ? 8 : 2),
    priceImpact: Math.min(priceImpact, 5), // Cap at 5%
    route: [fromUpper, toUpper],
    gasEstimate: '0.005', // Estimated ETH gas
  };
}

/**
 * Get all available tokens with prices
 */
export async function getTokensWithPrices(): Promise<PriceData[]> {
  const prices = await fetchPrices();
  return Array.from(priceCache.values());
}

/**
 * Get market data for a trading pair
 */
export async function getMarketData(pair: string): Promise<PriceData | null> {
  const [baseToken] = pair.split('-');
  const prices = await fetchPrices();
  return prices.get(baseToken.toUpperCase()) || null;
}
