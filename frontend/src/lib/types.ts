/**
 * Domain models and validators for data returned by the TigerSwap API.
 *
 * Every validator here is applied to the raw `unknown` body before the data
 * reaches a component, so the UI only ever handles checked values.
 */

import {
  address,
  array,
  arrayLenient,
  boolean,
  hex,
  integer,
  nonNegative,
  nullable,
  number,
  object,
  oneOf,
  string,
  symbol,
  timestamp,
  withDefault,
} from "./validation";

/** A tradable token as returned by `/tokens`. */
export interface Token {
  address: string;
  symbol: string;
  name: string;
  decimals: number;
  chainId: number;
  logoURI: string | null;
  priceUSD: number | null;
}

export const tokenValidator = object<Token>({
  address,
  symbol,
  name: string,
  decimals: integer,
  chainId: integer,
  logoURI: nullable(string),
  priceUSD: nullable(nonNegative),
});

export const tokenListValidator = arrayLenient(tokenValidator);

/** A blockchain network as returned by `/chains`. */
export interface Chain {
  id: number;
  name: string;
  chainType: string;
  nativeSymbol: string;
  explorerURL: string | null;
  isActive: boolean;
}

export const chainValidator = object<Chain>({
  id: integer,
  name: string,
  chainType: string,
  nativeSymbol: string,
  explorerURL: nullable(string),
  isActive: boolean,
});

export const chainListValidator = arrayLenient(chainValidator);

/** Aggregate market statistics from `/market/stats`. */
export interface MarketStats {
  totalTVL: number;
  volume24h: number;
  totalPairs: number;
  totalUsers: number;
  updatedAt: string | null;
}

export const marketStatsValidator = object<MarketStats>(
  {
    totalTVL: nonNegative,
    volume24h: nonNegative,
    totalPairs: integer,
    totalUsers: integer,
    updatedAt: nullable(timestamp),
  },
  { optional: ["updatedAt"] },
);

/** A liquidity pool from `/swap/pairs`. */
export interface Pool {
  pairAddress: string;
  token0: string;
  token1: string;
  symbol0: string;
  symbol1: string;
  reserve0: number;
  reserve1: number;
  feeBps: number;
}

export const poolValidator = object<Pool>({
  pairAddress: string,
  token0: string,
  token1: string,
  symbol0: symbol,
  symbol1: symbol,
  reserve0: nonNegative,
  reserve1: nonNegative,
  feeBps: integer,
});

export const poolListValidator = arrayLenient(poolValidator);

/** A quote from `/swap/quote`. */
export interface SwapQuote {
  amountIn: string;
  amountOut: string;
  amountOutMin: string;
  priceImpact: number;
  route: string[];
  gasEstimate: number;
}

export const swapQuoteValidator = object<SwapQuote>({
  amountIn: string,
  amountOut: string,
  amountOutMin: string,
  priceImpact: nonNegative,
  route: array(string),
  gasEstimate: nonNegative,
});

/** A recent trade from `/market/trades`. */
export interface Trade {
  id: string;
  pair: string;
  side: "buy" | "sell";
  price: number;
  quantity: number;
  timestamp: string;
}

export const tradeValidator = object<Trade>({
  id: string,
  pair: string,
  side: oneOf(["buy", "sell"] as const),
  price: nonNegative,
  quantity: nonNegative,
  timestamp,
});

export const tradeListValidator = arrayLenient(tradeValidator);

/** A price point from `/prices`. */
export interface PriceEntry {
  symbol: string;
  priceUSD: number;
  change24h: number;
}

export const priceValidator = object<PriceEntry>({
  symbol,
  priceUSD: nonNegative,
  change24h: number,
});

export const priceListValidator = arrayLenient(priceValidator);

/** A user's on-chain portfolio from `/user/portfolio`. */
export interface PortfolioAsset {
  chainId: number;
  tokenAddress: string;
  symbol: string;
  decimals: number;
  balance: string;
  valueUSD: number | null;
}

export const portfolioAssetValidator = object<PortfolioAsset>({
  chainId: integer,
  tokenAddress: string,
  symbol,
  decimals: integer,
  balance: string,
  valueUSD: nullable(nonNegative),
});

export interface Portfolio {
  totalValueUSD: number;
  assets: PortfolioAsset[];
  updatedAt: string | null;
}

export const portfolioValidator = object<Portfolio>(
  {
    totalValueUSD: nonNegative,
    assets: arrayLenient(portfolioAssetValidator),
    updatedAt: nullable(timestamp),
  },
  { optional: ["updatedAt"] },
);

/** A transaction from `/user/transactions`. */
export interface UserTransaction {
  hash: string;
  chainId: number;
  type: string;
  status: string;
  amount: string;
  tokenSymbol: string;
  createdAt: string;
}

export const userTransactionValidator = object<UserTransaction>({
  hash: string,
  chainId: integer,
  type: string,
  status: string,
  amount: string,
  tokenSymbol: symbol,
  createdAt: timestamp,
});

export const userTransactionListValidator = arrayLenient(userTransactionValidator);

/** A perpetual market from the perp engine. */
export interface PerpMarket {
  marketId: string;
  symbol: string;
  markPrice: number;
  indexPrice: number;
  fundingRate: number;
  maxLeverage: number;
  openInterestLong: number;
  openInterestShort: number;
  isActive: boolean;
}

export const perpMarketValidator = object<PerpMarket>({
  marketId: hex,
  symbol,
  markPrice: nonNegative,
  indexPrice: nonNegative,
  fundingRate: number,
  maxLeverage: nonNegative,
  openInterestLong: nonNegative,
  openInterestShort: nonNegative,
  isActive: boolean,
});

export const perpMarketListValidator = arrayLenient(perpMarketValidator);

/** The authenticated user's profile. */
export interface UserProfile {
  id: string;
  email: string | null;
  role: string;
  createdAt: string;
}

export const userProfileValidator = object<UserProfile>(
  {
    id: string,
    email: nullable(string),
    role: oneOf(["user", "admin", "superadmin"] as const),
    createdAt: withDefault(timestamp, new Date(0).toISOString()),
  },
  { optional: ["createdAt"] },
);

/** An admin dashboard summary from `/admin/dashboard`. */
export interface AdminDashboard {
  totalUsers: number;
  totalTransactions: number;
  totalVolume: number;
  totalRevenue: number;
  activePools: number;
  updatedAt: string | null;
}

export const adminDashboardValidator = object<AdminDashboard>(
  {
    totalUsers: integer,
    totalTransactions: integer,
    totalVolume: nonNegative,
    totalRevenue: nonNegative,
    activePools: integer,
    updatedAt: nullable(timestamp),
  },
  { optional: ["activePools", "updatedAt"] },
);

/** A generic paginated envelope. */
export interface Page<T> {
  items: T[];
  total: number;
  page: number;
  pageSize: number;
}

export function pageValidator<T>(item: (value: unknown, path?: string) => T): (value: unknown, path?: string) => Page<T> {
  return object<Page<T>>({
    items: arrayLenient(item),
    total: integer,
    page: integer,
    pageSize: integer,
  });
}
