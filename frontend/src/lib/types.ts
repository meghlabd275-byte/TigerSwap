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
  displayText,
  hex,
  httpURL,
  integer,
  literalTrue,
  marketSymbol,
  nonNegative,
  nullable,
  number,
  object,
  oneOf,
  string,
  symbol,
  timestamp,
  ValidationError,
  withDefault,
} from "./validation";
import type { Validator } from "./validation";

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
  nativeDecimals: number;
  rpcUrls: string[];
  explorerURL: string | null;
  isActive: boolean;
}

export const chainValidator = object<Chain>({
  id: integer,
  name: string,
  chainType: string,
  nativeSymbol: string,
  nativeDecimals: withDefault(integer, 18),
  rpcUrls: withDefault(arrayLenient(httpURL), []),
  explorerURL: nullable(httpURL),
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
  /** Present only when the router split the order across several routes. */
  legs?: SwapQuoteLeg[];
  /**
   * The executable hop plan for a single multi-hop route. Present only when the
   * winning route has more than one hop; a direct single-family quote omits it.
   */
  routeHops?: SwapRouteHop[];
}

/** One executable hop of a route: a token pair and the family to trade it on. */
export interface SwapRouteHop {
  tokenIn: string;
  tokenOut: string;
  family: string;
  pool: string;
}

/**
 * One leg of a split swap. The server sends it only when the router divided the
 * input across independent routes to reduce price impact.
 */
export interface SwapQuoteLeg {
  path: string[];
  family: string;
  pool: string;
  amountIn: string;
  amountOut: string;
  /** The full executable path for the leg; a multi-hop leg has more than one. */
  hops?: SwapRouteHop[];
}

export const swapRouteHopValidator = object<SwapRouteHop>({
  tokenIn: address,
  tokenOut: address,
  family: string,
  pool: address,
});

export const swapQuoteLegValidator = object<SwapQuoteLeg>(
  {
    path: array(address),
    family: string,
    pool: address,
    amountIn: string,
    amountOut: string,
    hops: array(swapRouteHopValidator),
  },
  { optional: ["hops"] },
);

export const swapQuoteValidator = object<SwapQuote>(
  {
    amountIn: string,
    amountOut: string,
    amountOutMin: string,
    priceImpact: nonNegative,
    route: array(address),
    gasEstimate: nonNegative,
    legs: array(swapQuoteLegValidator),
    routeHops: array(swapRouteHopValidator),
  },
  { optional: ["legs", "routeHops"] },
);

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
  /** Realized PnL from closed derivatives positions, in USD. May be negative. */
  realizedPnlUsd: number;
  /** Unrealized PnL on open derivatives positions, in USD. May be negative. */
  unrealizedPnlUsd: number;
  openPositions: number;
  updatedAt: string | null;
}

export const portfolioValidator = object<Portfolio>(
  {
    totalValueUSD: nonNegative,
    assets: arrayLenient(portfolioAssetValidator),
    realizedPnlUsd: number,
    unrealizedPnlUsd: number,
    openPositions: integer,
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
  marketId: string,
  symbol: marketSymbol,
  markPrice: nonNegative,
  indexPrice: nonNegative,
  fundingRate: number,
  maxLeverage: nonNegative,
  openInterestLong: nonNegative,
  openInterestShort: nonNegative,
  isActive: boolean,
});

export const perpMarketListValidator = listingArray(perpMarketValidator);

/**
 * An open perpetual position owned by the authenticated caller, as returned by
 * `/wallet/perpetual/positions`. Numeric fields arrive as decimal strings so no
 * precision is lost in transit.
 */
export interface PerpPosition {
  id: string;
  pair: string;
  side: 'long' | 'short';
  size: string;
  collateral: string;
  entry_price: string;
  mark_price: string;
  leverage: number;
  status: string;
  pnl: string;
  opened_at: string;
}

export const perpPositionValidator = object<PerpPosition>({
  id: string,
  pair: string,
  side: oneOf(['long', 'short'] as const),
  size: string,
  collateral: string,
  entry_price: string,
  mark_price: string,
  leverage: integer,
  status: string,
  pnl: withDefault(string, '0'),
  opened_at: withDefault(timestamp, new Date(0).toISOString()),
});

export const perpPositionListValidator = listingArray(perpPositionValidator);

/** Result of opening a perpetual position. */
export interface PerpOpenResult {
  success: boolean;
  position: PerpPosition;
}

export const perpOpenResultValidator = object<PerpOpenResult>({
  success: literalTrue,
  position: perpPositionValidator,
});

/**
 * A perpetual order owned by the authenticated caller, as returned by
 * `/perpetual/orders`. A pending limit or stop order rests until the mark price
 * meets its condition or its expiry passes.
 */
export interface PerpOrder {
  id: string;
  user_address: string;
  pair: string;
  side: 'long' | 'short';
  order_type: string;
  size: string;
  price: number;
  trigger_price: number;
  status: string;
  filled_size: string;
  created_at: string;
  expires_at: string;
}

export const perpOrderValidator = object<PerpOrder>({
  id: string,
  user_address: withDefault(string, ''),
  pair: string,
  side: oneOf(['long', 'short'] as const),
  order_type: string,
  size: string,
  price: withDefault(number, 0),
  trigger_price: withDefault(number, 0),
  status: string,
  filled_size: withDefault(string, '0'),
  created_at: withDefault(timestamp, new Date(0).toISOString()),
  expires_at: withDefault(timestamp, new Date(0).toISOString()),
});

export const perpOrderListValidator = listingArray(perpOrderValidator);

/** A margin account, as returned when one is opened. */
export interface MarginAccount {
  id: string;
  user_id: string;
  margin_mode: string;
  pair: string;
  status: string;
}

export const marginAccountValidator = object<MarginAccount>({
  id: string,
  user_id: string,
  margin_mode: string,
  pair: string,
  status: string,
});

/** The risk view of a margin account: collateral, debt and health factor. */
export interface MarginAccountView {
  id: string;
  margin_mode: string;
  pair: string;
  status: string;
  collateral_value_usd: number;
  debt_value_usd: number;
  equity_usd: number;
  health_factor: number;
  liquidation_value_usd: number;
}

export const marginAccountViewValidator = object<MarginAccountView>({
  id: string,
  margin_mode: string,
  pair: string,
  status: string,
  collateral_value_usd: nonNegative,
  debt_value_usd: nonNegative,
  equity_usd: number,
  health_factor: number,
  liquidation_value_usd: nonNegative,
});

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

/** A blockchain row as returned by the admin chains endpoint. */
export interface AdminChain {
  id: number;
  chain_id: number;
  name: string;
  symbol: string;
  icon: string;
  rpc_url: string;
  explorer_url: string;
  type: string;
  is_active: boolean;
  is_testnet: boolean;
}

export const adminChainValidator = object<AdminChain>({
  id: integer,
  chain_id: integer,
  name: string,
  symbol: string,
  icon: withDefault(string, ""),
  rpc_url: withDefault(string, ""),
  explorer_url: withDefault(string, ""),
  type: withDefault(string, ""),
  is_active: withDefault(boolean, true),
  is_testnet: withDefault(boolean, false),
});

export const adminChainListValidator = listingArray(adminChainValidator);

/** A token row as returned by the admin tokens endpoint. */
export interface AdminToken {
  id: number;
  address: string;
  chain_id: number;
  name: string;
  symbol: string;
  decimals: number;
  logo_uri: string;
  total_supply: string;
  is_active: boolean;
  is_verified: boolean;
  coingecko_id?: string;
}

export const adminTokenValidator = object<AdminToken>({
  id: integer,
  address: string,
  chain_id: integer,
  name: string,
  symbol,
  decimals: integer,
  logo_uri: withDefault(string, ""),
  total_supply: withDefault(string, "0"),
  is_active: withDefault(boolean, true),
  is_verified: withDefault(boolean, false),
  coingecko_id: withDefault(string, ""),
});

export const adminTokenListValidator = listingArray(adminTokenValidator);

/** Platform runtime switches. */
export interface AdminSettings {
  id: number;
  key: string;
  maintenance_mode: boolean;
  trading_enabled: boolean;
  withdrawal_enabled: boolean;
  deposit_enabled: boolean;
  new_user_registration_enabled: boolean;
  updated_at: string | null;
}

export const adminSettingsValidator = object<AdminSettings>({
  id: integer,
  key: withDefault(string, "default"),
  maintenance_mode: withDefault(boolean, false),
  trading_enabled: withDefault(boolean, true),
  withdrawal_enabled: withDefault(boolean, true),
  deposit_enabled: withDefault(boolean, true),
  new_user_registration_enabled: withDefault(boolean, true),
  updated_at: nullable(timestamp),
});

/** A fee configuration as stored by the fee collector. */
export interface FeeConfig {
  swap_fee_percent: number;
  swap_fee_fixed: number;
  bridge_fee_percent: number;
  bridge_fee_fixed: number;
  wallet_tx_fee_percent: number;
  wallet_tx_fee_fixed: number;
  deposit_fee_percent: number;
  deposit_fee_fixed: number;
  withdraw_fee_percent: number;
  withdraw_fee_fixed: number;
  launchpad_fee_percent: number;
  launchpad_fee_fixed: number;
  referral_fee_percent: number;
}

export const feeConfigValidator = object<FeeConfig>({
  swap_fee_percent: nonNegative,
  swap_fee_fixed: withDefault(nonNegative, 0),
  bridge_fee_percent: withDefault(nonNegative, 0),
  bridge_fee_fixed: withDefault(nonNegative, 0),
  wallet_tx_fee_percent: withDefault(nonNegative, 0),
  wallet_tx_fee_fixed: withDefault(nonNegative, 0),
  deposit_fee_percent: withDefault(nonNegative, 0),
  deposit_fee_fixed: withDefault(nonNegative, 0),
  withdraw_fee_percent: withDefault(nonNegative, 0),
  withdraw_fee_fixed: withDefault(nonNegative, 0),
  launchpad_fee_percent: withDefault(nonNegative, 0),
  launchpad_fee_fixed: withDefault(nonNegative, 0),
  referral_fee_percent: withDefault(nonNegative, 0),
});

/** An immutable admin audit-log entry. */
export interface AdminAuditLog {
  id: number;
  action: string;
  entity_type: string;
  entity_id: string;
  details: string;
  created_at: string;
}

export const adminAuditLogValidator = object<AdminAuditLog>({
  id: integer,
  action: string,
  entity_type: withDefault(string, ""),
  entity_id: withDefault(string, ""),
  details: withDefault(string, ""),
  created_at: withDefault(timestamp, new Date(0).toISOString()),
});

export const adminAuditLogListValidator = listingArray(adminAuditLogValidator);

/** A launchpad project as returned by the public launchpad endpoint. */
export interface LaunchpadProject {
  id: string;
  name: string;
  description: string;
  token_address: string;
  token_symbol: string;
  token_name: string;
  total_supply: string;
  price_per_token: string;
  payment_token: string;
  min_purchase: string;
  max_purchase: string;
  soft_cap: string;
  hard_cap: string;
  start_time: string;
  end_time: string;
  status: string;
  raised_amount: string;
  participants: number;
  website_url: string;
  whitepaper_url: string;
  logo_url: string;
}

export const launchpadProjectValidator = object<LaunchpadProject>({
  id: string,
  name: string,
  description: withDefault(string, ""),
  token_address: withDefault(string, ""),
  token_symbol: string,
  token_name: withDefault(string, ""),
  total_supply: withDefault(string, "0"),
  price_per_token: withDefault(string, "0"),
  payment_token: withDefault(string, ""),
  min_purchase: withDefault(string, "0"),
  max_purchase: withDefault(string, "0"),
  soft_cap: withDefault(string, "0"),
  hard_cap: withDefault(string, "0"),
  start_time: withDefault(timestamp, new Date(0).toISOString()),
  end_time: withDefault(timestamp, new Date(0).toISOString()),
  status: string,
  raised_amount: withDefault(string, "0"),
  participants: withDefault(integer, 0),
  website_url: withDefault(string, ""),
  whitepaper_url: withDefault(string, ""),
  logo_url: withDefault(string, ""),
});

export const launchpadProjectListValidator = listingArray(launchpadProjectValidator);

/**
 * Admin-managed listing models.
 *
 * These mirror the futures, perps and margin listing records the backend
 * exposes under `/admin/futures/markets`, `/admin/perps/markets` and
 * `/admin/margin/markets`. Every field is validated before it reaches the UI.
 */

export type ListingStatus = "active" | "paused" | "delisting" | "maintenance";

export const listingStatusValidator = oneOf([
  "active",
  "paused",
  "delisting",
  "maintenance",
] as const);

/** A futures contract listing. */
export interface FuturesMarket {
  id: string;
  symbol: string;
  contract_type: string;
  base_asset: string;
  quote_asset: string;
  settlement: string;
  contract_size: string;
  tick_size: string;
  max_leverage: number;
  min_order_size: string;
  funding_rate: string;
  funding_interval_hours: number;
  funding_rate_cap: string;
  oracle_sources: string;
  maintenance_margin_rate: number;
  initial_margin_rate: number;
  liquidation_fee_rate: number;
  insurance_fund_share: number;
  maker_fee_rate: number;
  taker_fee_rate: number;
  status: ListingStatus;
  listing_type: string;
  is_tradfi: boolean;
  created_at: string;
  updated_at: string;
}

export const futuresMarketValidator = object<FuturesMarket>(
  {
    id: string,
    symbol: marketSymbol,
    contract_type: string,
    base_asset: string,
    quote_asset: string,
    settlement: string,
    contract_size: string,
    tick_size: string,
    max_leverage: integer,
    min_order_size: withDefault(string, ""),
    funding_rate: string,
    funding_interval_hours: integer,
    funding_rate_cap: withDefault(string, ""),
    oracle_sources: withDefault(string, ""),
    maintenance_margin_rate: nonNegative,
    initial_margin_rate: nonNegative,
    liquidation_fee_rate: nonNegative,
    insurance_fund_share: nonNegative,
    maker_fee_rate: nonNegative,
    taker_fee_rate: nonNegative,
    status: listingStatusValidator,
    listing_type: string,
    is_tradfi: boolean,
    created_at: withDefault(timestamp, new Date(0).toISOString()),
    updated_at: withDefault(timestamp, new Date(0).toISOString()),
  },
  {
    optional: [
      "min_order_size",
      "funding_rate_cap",
      "oracle_sources",
      "created_at",
      "updated_at",
    ],
  },
);

/** A perpetual listing, priced either from an oracle or a CLOB order book. */
export interface PerpsListing {
  id: string;
  symbol: string;
  market_type: string;
  collateral_assets: string;
  max_open_interest: string;
  max_leverage: number;
  price_impact_rate: number;
  borrow_fee_per_hour: number;
  maker_fee_rate: number;
  taker_fee_rate: number;
  maintenance_margin_rate: number;
  liquidation_fee_rate: number;
  status: ListingStatus;
  created_at: string;
  updated_at: string;
}

export const perpsListingValidator = object<PerpsListing>(
  {
    id: string,
    symbol: marketSymbol,
    market_type: string,
    collateral_assets: withDefault(string, ""),
    max_open_interest: withDefault(string, "0"),
    max_leverage: integer,
    price_impact_rate: nonNegative,
    borrow_fee_per_hour: nonNegative,
    maker_fee_rate: nonNegative,
    taker_fee_rate: nonNegative,
    maintenance_margin_rate: nonNegative,
    liquidation_fee_rate: nonNegative,
    status: listingStatusValidator,
    created_at: withDefault(timestamp, new Date(0).toISOString()),
    updated_at: withDefault(timestamp, new Date(0).toISOString()),
  },
  {
    optional: [
      "collateral_assets",
      "max_open_interest",
      "created_at",
      "updated_at",
    ],
  },
);

/** A spot-margin listing with its risk parameters and interest curve. */
export interface MarginListing {
  id: string;
  pair: string;
  base_asset: string;
  quote_asset: string;
  margin_mode: string;
  max_leverage: number;
  collateral_assets: string;
  borrowable_assets: string;
  base_rate_apy: number;
  kink_utilization: number;
  jump_multiplier: number;
  ltv: number;
  liquidation_threshold: number;
  liquidation_bonus: number;
  close_factor: number;
  supply_cap: string;
  borrow_cap: string;
  risk_tier: string;
  status: ListingStatus;
  created_at: string;
  updated_at: string;
}

export const marginListingValidator = object<MarginListing>(
  {
    id: string,
    pair: marketSymbol,
    base_asset: string,
    quote_asset: string,
    margin_mode: string,
    max_leverage: integer,
    collateral_assets: withDefault(string, ""),
    borrowable_assets: withDefault(string, ""),
    base_rate_apy: nonNegative,
    kink_utilization: nonNegative,
    jump_multiplier: nonNegative,
    ltv: nonNegative,
    liquidation_threshold: nonNegative,
    liquidation_bonus: nonNegative,
    close_factor: nonNegative,
    supply_cap: withDefault(string, "0"),
    borrow_cap: withDefault(string, "0"),
    risk_tier: string,
    status: listingStatusValidator,
    created_at: withDefault(timestamp, new Date(0).toISOString()),
    updated_at: withDefault(timestamp, new Date(0).toISOString()),
  },
  {
    optional: [
      "collateral_assets",
      "borrowable_assets",
      "supply_cap",
      "borrow_cap",
      "created_at",
      "updated_at",
    ],
  },
);

/**
 * Extracts a listing array from a single-key envelope such as
 * `{ markets: [...] }`, `{ listings: [...] }` or `{ balances: [...] }`,
 * tolerating a bare array.
 */
function listingArray<T>(element: (value: unknown, path?: string) => T) {
  return (value: unknown, path = "value"): T[] => {
    if (Array.isArray(value)) {
      return arrayLenient(element)(value, path);
    }
    if (value && typeof value === "object") {
      const record = value as Record<string, unknown>;
      const items =
        record.markets ??
        record.listings ??
        record.balances ??
        record.items ??
        record.locks ??
        record.gauges ??
        record.bribes ??
        record.proposals ??
        record.chains ??
        record.tokens ??
        record.logs ??
        record.traders ??
        record.transfers ??
        record.positions ??
        record.projects ??
        record.farms ??
        record.stakes ??
        record.campaigns ??
        record.allocations ??
        record.series ??
        record.option_series ??
        record.collections ??
        record.offers ??
        record.pairs ??
        record.nft_listings;
      if (items !== undefined) {
        // A Go nil slice serialises as `null`; treat it as an empty list
        // rather than failing the whole response.
        return arrayLenient(element)(items ?? [], `${path}.items`);
      }
    }
    return arrayLenient(element)(value, path);
  };
}

/**
 * Unwraps a single-key envelope such as `{ series: {...} }` or
 * `{ campaign: {...} }` before validating the inner object. The key must be
 * named explicitly because a wrapper and its payload can share a field name:
 * `OptionSeriesView` has its own `series` field, so a generic unwrap would
 * strip the wrong level.
 */
function envelope<T>(key: string, inner: Validator<T>): Validator<T> {
  return (value: unknown, path = "value"): T => {
    if (value && typeof value === "object" && !Array.isArray(value)) {
      const record = value as Record<string, unknown>;
      if (record[key] !== undefined) {
        return inner(record[key], `${path}.${key}`);
      }
    }
    return inner(value, path);
  };
}

export const futuresMarketListValidator = listingArray(futuresMarketValidator);
export const perpsListingListValidator = listingArray(perpsListingValidator);
export const marginListingListValidator = listingArray(marginListingValidator);

/** Payload accepted when creating a futures listing. */
export interface FuturesMarketInput {
  symbol: string;
  contract_type: string;
  settlement: string;
  contract_size?: string;
  tick_size?: string;
  max_leverage: number;
  funding_rate?: string;
  funding_interval_hours?: number;
  funding_rate_cap?: string;
  oracle_sources?: string;
  maintenance_margin_rate?: number;
  initial_margin_rate?: number;
  liquidation_fee_rate?: number;
  insurance_fund_share?: number;
  maker_fee_rate?: number;
  taker_fee_rate?: number;
  listing_type?: string;
  is_tradfi?: boolean;
  status?: ListingStatus;
}

/** Payload accepted when creating a perpetual listing. */
export interface PerpsListingInput {
  symbol: string;
  market_type?: string;
  collateral_assets?: string;
  max_open_interest?: string;
  max_leverage: number;
  price_impact_rate?: number;
  borrow_fee_per_hour?: number;
  maker_fee_rate?: number;
  taker_fee_rate?: number;
  maintenance_margin_rate?: number;
  liquidation_fee_rate?: number;
  status?: ListingStatus;
}

/** Payload accepted when creating a spot-margin listing. */
export interface MarginListingInput {
  pair: string;
  margin_mode?: string;
  max_leverage: number;
  collateral_assets?: string;
  borrowable_assets?: string;
  base_rate_apy?: number;
  kink_utilization?: number;
  jump_multiplier?: number;
  ltv: number;
  liquidation_threshold: number;
  liquidation_bonus?: number;
  close_factor?: number;
  supply_cap?: string;
  borrow_cap?: string;
  risk_tier?: string;
  status?: ListingStatus;
}

/** A user's platform ledger balance for one token on one chain. */
export interface AccountBalance {
  user_id: string;
  chain_id: number;
  token_address: string;
  symbol: string;
  decimals: number;
  available: string;
  locked: string;
}

export const accountBalanceValidator = object<AccountBalance>({
  user_id: string,
  chain_id: integer,
  token_address: string,
  symbol: string,
  decimals: integer,
  available: string,
  locked: withDefault(string, "0"),
});

/**
 * Extracts the balances array from a `{ balances: [...] }` envelope, tolerating
 * a bare array.
 */
export const accountBalanceListValidator = listingArray(accountBalanceValidator);

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

/**
 * Governance: veTGR locks, gauge voting and the bribe market.
 *
 * Token amounts are base-10 strings end to end so no precision is lost
 * between the contract, the API and the UI.
 */

/** A veTGR lock with its live, decaying voting power. */
export interface VeLock {
  id: string;
  user_id: string;
  amount: string;
  start_time: string;
  unlock_time: string;
  withdrawn: boolean;
  voting_power: string;
  created_at: string;
  updated_at: string;
}

export const veLockValidator = object<VeLock>(
  {
    id: string,
    user_id: withDefault(string, ""),
    amount: string,
    start_time: withDefault(timestamp, new Date(0).toISOString()),
    unlock_time: timestamp,
    withdrawn: withDefault(boolean, false),
    voting_power: withDefault(string, "0"),
    created_at: withDefault(timestamp, new Date(0).toISOString()),
    updated_at: withDefault(timestamp, new Date(0).toISOString()),
  },
  { optional: ["user_id", "start_time", "withdrawn", "voting_power", "created_at", "updated_at"] },
);

export const veLockListValidator = listingArray(veLockValidator);

/** A gauge: an emission target that veTGR holders vote on. */
export interface Gauge {
  id: string;
  name: string;
  gauge_type: string;
  reference: string;
  weight: string;
  total_voted: string;
  active: boolean;
  created_at: string;
  updated_at: string;
}

export const gaugeValidator = object<Gauge>(
  {
    id: string,
    name: displayText,
    gauge_type: string,
    reference: withDefault(string, ""),
    weight: withDefault(string, "0"),
    total_voted: withDefault(string, "0"),
    active: withDefault(boolean, true),
    created_at: withDefault(timestamp, new Date(0).toISOString()),
    updated_at: withDefault(timestamp, new Date(0).toISOString()),
  },
  { optional: ["reference", "weight", "total_voted", "active", "created_at", "updated_at"] },
);

export const gaugeListValidator = listingArray(gaugeValidator);

/** A bribe funding a gauge's voters for one epoch. */
export interface Bribe {
  id: string;
  gauge_id: string;
  epoch: number;
  token: string;
  amount: string;
  total_weight: string;
  funder_id: string;
  created_at: string;
  updated_at: string;
}

export const bribeValidator = object<Bribe>(
  {
    id: string,
    gauge_id: string,
    epoch: integer,
    token: symbol,
    amount: string,
    total_weight: withDefault(string, "0"),
    funder_id: withDefault(string, ""),
    created_at: withDefault(timestamp, new Date(0).toISOString()),
    updated_at: withDefault(timestamp, new Date(0).toISOString()),
  },
  { optional: ["total_weight", "funder_id", "created_at", "updated_at"] },
);

export const bribeListValidator = listingArray(bribeValidator);

/** The result of a bribe claim. */
export interface BribeClaimResult {
  success: boolean;
  token: string;
  amount: string;
}

export const bribeClaimResultValidator = object<BribeClaimResult>({
  success: boolean,
  token: symbol,
  amount: string,
});

/** A user's farm boost derived from their veTGR balance. */
export interface GovernanceBoost {
  voting_power: string;
  staked_amount: string;
  boost_bps: string;
  boost_multiplier: string;
}

export const governanceBoostValidator = object<GovernanceBoost>({
  voting_power: withDefault(string, "0"),
  staked_amount: withDefault(string, "0"),
  boost_bps: withDefault(string, "10000"),
  boost_multiplier: withDefault(string, "1"),
});

/** A decimal integer formatted as a string, e.g. a balance or a PnL. */
const decimal = string;

/**
 * An identifier the API may serialise as a number or a string.
 *
 * Go emits a `uint` primary key as a JSON number while a UUID key is a string,
 * so both forms are accepted and normalised to a string.
 */
const identifier: Validator<string> = (value, path = "value") => {
  if (typeof value === "number") {
    if (!Number.isInteger(value) || !Number.isFinite(value)) {
      throw new ValidationError("expected an integer identifier", path);
    }
    return String(value);
  }
  return string(value, path);
};

/**
 * A configured trading bot.
 *
 * `config` is the strategy's validated parameters, held as JSON. It is
 * deliberately typed as `unknown`: each strategy has its own schema, so the UI
 * renders the known risk limits and never assumes a shape for `config`.
 */
export interface BotInstance {
  id: string;
  name: string;
  strategy: string;
  symbol: string;
  status: string;
  config: unknown;
  max_order_size: string;
  max_daily_loss: string;
  max_open_orders: number;
  realised_pnl: string;
  filled_orders: number;
  traded_volume: string;
  last_run_at: string;
  created_at: string;
  updated_at: string;
}

export const botValidator = object<BotInstance>({
  id: identifier,
  name: displayText,
  strategy: string,
  symbol: marketSymbol,
  status: string,
  config: withDefault<unknown>((value) => value, {}),
  max_order_size: decimal,
  max_daily_loss: withDefault(decimal, "0"),
  max_open_orders: withDefault(integer, 10),
  realised_pnl: withDefault(decimal, "0"),
  filled_orders: withDefault(integer, 0),
  traded_volume: withDefault(decimal, "0"),
  last_run_at: withDefault(timestamp, new Date(0).toISOString()),
  created_at: withDefault(timestamp, new Date(0).toISOString()),
  updated_at: withDefault(timestamp, new Date(0).toISOString()),
});

export const botListValidator = listingArray(botValidator);

/** An order a bot engine decided to place. */
export interface BotOrder {
  id: string;
  bot_id: string;
  user_id: string;
  symbol: string;
  side: string;
  order_kind: string;
  price: string;
  quantity: string;
  reduce_only: boolean;
  source: string;
  status: string;
  tx_hash: string | null;
  created_at: string;
  updated_at: string;
}

export const botOrderValidator = object<BotOrder>({
  id: identifier,
  bot_id: identifier,
  user_id: identifier,
  symbol: marketSymbol,
  side: oneOf(["buy", "sell", "long", "short"] as const),
  order_kind: string,
  price: decimal,
  quantity: decimal,
  reduce_only: withDefault(boolean, false),
  source: withDefault(string, ""),
  status: string,
  tx_hash: withDefault(nullable(string), null),
  created_at: withDefault(timestamp, new Date(0).toISOString()),
  updated_at: withDefault(timestamp, new Date(0).toISOString()),
});

export const botOrderListValidator = listingArray(botOrderValidator);

/** One day of a bot's realised performance. */
export interface BotPerformance {
  id: string;
  bot_id: string;
  day: string;
  orders_placed: number;
  orders_filled: number;
  volume: string;
  realised_pnl: string;
  fees_paid: string;
  max_drawdown: string;
}

export const botPerformanceValidator = object<BotPerformance>({
  id: identifier,
  bot_id: identifier,
  day: timestamp,
  orders_placed: withDefault(integer, 0),
  orders_filled: withDefault(integer, 0),
  volume: withDefault(decimal, "0"),
  realised_pnl: withDefault(decimal, "0"),
  fees_paid: withDefault(decimal, "0"),
  max_drawdown: withDefault(decimal, "0"),
});

export const botPerformanceListValidator = listingArray(botPerformanceValidator);

/** A strategy a client may configure, with the keys its config accepts. */
export interface BotStrategy {
  strategy: string;
  config_keys: Record<string, string>;
}

export const botStrategyValidator = object<BotStrategy>({
  strategy: string,
  config_keys: withDefault<Record<string, string>>((value) => {
    if (value === null || typeof value !== "object" || Array.isArray(value)) {
      return {};
    }
    return value as Record<string, string>;
  }, {}),
});

export const botStrategyListValidator = listingArray(botStrategyValidator);

/** Counts of the caller's bots, grouped by strategy and status. */
export interface BotsSummary {
  by_strategy: Record<string, number>;
  by_status: Record<string, number>;
  total: number;
}

export const botsSummaryValidator = object<BotsSummary>({
  by_strategy: withDefault<Record<string, number>>((value) => {
    if (value === null || typeof value !== "object" || Array.isArray(value)) {
      return {};
    }
    const out: Record<string, number> = {};
    for (const [key, raw] of Object.entries(value as Record<string, unknown>)) {
      const parsed = Number(raw);
      if (Number.isFinite(parsed)) {
        out[key] = parsed;
      }
    }
    return out;
  }, {}),
  by_status: withDefault<Record<string, number>>((value) => {
    if (value === null || typeof value !== "object" || Array.isArray(value)) {
      return {};
    }
    const out: Record<string, number> = {};
    for (const [key, raw] of Object.entries(value as Record<string, unknown>)) {
      const parsed = Number(raw);
      if (Number.isFinite(parsed)) {
        out[key] = parsed;
      }
    }
    return out;
  }, {}),
  total: withDefault(integer, 0),
});

/** A private OTC block trade. */
export interface OtcOrder {
  id: string;
  maker_id: string;
  taker_id: string | null;
  symbol: string;
  side: string;
  quantity: string;
  price: string;
  status: string;
  min_fill: string;
  counterparty: string | null;
  escrow_tx_hash: string | null;
  settle_tx_hash: string | null;
  expires_at: string;
  created_at: string;
  updated_at: string;
}

export const otcOrderValidator = object<OtcOrder>({
  id: identifier,
  maker_id: identifier,
  taker_id: withDefault(nullable(identifier), null),
  symbol: marketSymbol,
  side: oneOf(["buy", "sell"] as const),
  quantity: decimal,
  price: decimal,
  status: string,
  min_fill: withDefault(decimal, "0"),
  counterparty: withDefault(nullable(string), null),
  escrow_tx_hash: withDefault(nullable(string), null),
  settle_tx_hash: withDefault(nullable(string), null),
  expires_at: withDefault(timestamp, new Date(0).toISOString()),
  created_at: withDefault(timestamp, new Date(0).toISOString()),
  updated_at: withDefault(timestamp, new Date(0).toISOString()),
});

export const otcOrderListValidator = listingArray(otcOrderValidator);

/** A TWAMM order that executes in slices across its lifetime. */
export interface TwammOrder {
  id: string;
  user_id: string;
  symbol: string;
  side: string;
  total_quantity: string;
  filled_quantity: string;
  limit_price: string;
  start_time: string;
  end_time: string;
  status: string;
  created_at: string;
  updated_at: string;
}

export const twammOrderValidator = object<TwammOrder>({
  id: identifier,
  user_id: identifier,
  symbol: marketSymbol,
  side: oneOf(["buy", "sell"] as const),
  total_quantity: decimal,
  filled_quantity: withDefault(decimal, "0"),
  limit_price: withDefault(decimal, "0"),
  start_time: timestamp,
  end_time: timestamp,
  status: string,
  created_at: withDefault(timestamp, new Date(0).toISOString()),
  updated_at: withDefault(timestamp, new Date(0).toISOString()),
});

export const twammOrderListValidator = listingArray(twammOrderValidator);

/**
 * A trader on the copy-trading leaderboard, as returned by
 * `/copy-trading/traders` (shape: `{ traders: Trader[], count: number }`).
 */
export interface CopyTrader {
  address: string;
  name: string;
  avatar: string;
  win_rate: number;
  total_profit: string;
  weekly_profit: number;
  monthly_profit: number;
  total_trades: number;
  followers: number;
  total_volume: string;
  avg_holding_time: string;
  is_verified: boolean;
  is_pro: boolean;
  risk_score: string;
  strategies: string[];
  last_active: string;
}

export const copyTraderValidator = object<CopyTrader>({
  address,
  name: displayText,
  avatar: withDefault(string, ""),
  win_rate: nonNegative,
  total_profit: withDefault(decimal, "0"),
  weekly_profit: withDefault(number, 0),
  monthly_profit: withDefault(number, 0),
  total_trades: withDefault(nonNegative, 0),
  followers: withDefault(nonNegative, 0),
  total_volume: withDefault(decimal, "0"),
  avg_holding_time: withDefault(string, ""),
  is_verified: withDefault(boolean, false),
  is_pro: withDefault(boolean, false),
  risk_score: withDefault(string, "medium"),
  strategies: withDefault(array(string), []),
  last_active: withDefault(timestamp, new Date(0).toISOString()),
});

export const copyTraderListValidator = listingArray(copyTraderValidator);

/**
 * A TigerBridge quote from `POST /bridge/quote`. The amounts are decimal
 * strings in the bridged token's smallest unit.
 */
export interface BridgeQuote {
  src_chain: number;
  dst_chain: number;
  token: string;
  amount_in: string;
  amount_out: string;
  fee: string;
  fee_usd: number;
  estimated_time: string;
  router: string;
  price_impact: number;
}

export const bridgeQuoteValidator = object<BridgeQuote>({
  src_chain: integer,
  dst_chain: integer,
  token: string,
  amount_in: decimal,
  amount_out: decimal,
  fee: decimal,
  fee_usd: withDefault(number, 0),
  estimated_time: withDefault(string, ""),
  router: withDefault(string, ""),
  price_impact: withDefault(number, 0),
});

/**
 * A position the caller has copied from a trader, from
 * `/copy-trading/positions`.
 */
export interface CopyPosition {
  id: string;
  follower_address: string;
  trader_address: string;
  original_position: string;
  pair: string;
  side: string;
  size: string;
  entry_price: number;
  current_price: number;
  leverage: number;
  pnl: string;
  status: string;
  opened_at: string;
  closed_at: string | null;
  copied_at: string;
}

export const copyPositionValidator = object<CopyPosition>({
  id: identifier,
  follower_address: address,
  trader_address: address,
  original_position: withDefault(string, ""),
  pair: marketSymbol,
  side: oneOf(["buy", "sell", "long", "short"] as const),
  size: decimal,
  entry_price: nonNegative,
  current_price: nonNegative,
  leverage: nonNegative,
  pnl: withDefault(decimal, "0"),
  status: oneOf(["open", "closed"] as const),
  opened_at: timestamp,
  closed_at: nullable(timestamp),
  copied_at: withDefault(timestamp, new Date(0).toISOString()),
});

export const copyPositionListValidator = listingArray(copyPositionValidator);

/**
 * A TigerBridge transfer. `src_tx_hash` is set once the lock is mined and
 * `dst_tx_hash` once the destination mint confirms.
 */
export interface BridgeTransfer {
  id: string;
  src_chain: number;
  dst_chain: number;
  sender: string;
  recipient: string;
  token: string;
  amount: string;
  src_tx_hash: string;
  dst_tx_hash: string;
  nonce: number;
  status: string;
  fee: string;
  fee_usd: number;
  amount_received: string;
  created_at: string;
  confirmed_at: string | null;
  delivered_at: string | null;
}

export const bridgeTransferValidator = object<BridgeTransfer>({
  id: identifier,
  src_chain: integer,
  dst_chain: integer,
  sender: address,
  recipient: address,
  token: string,
  amount: decimal,
  src_tx_hash: withDefault(hex, ""),
  dst_tx_hash: withDefault(hex, ""),
  nonce: nonNegative,
  status: string,
  fee: withDefault(decimal, "0"),
  fee_usd: withDefault(number, 0),
  amount_received: withDefault(decimal, "0"),
  created_at: withDefault(timestamp, new Date(0).toISOString()),
  confirmed_at: nullable(timestamp),
  delivered_at: nullable(timestamp),
});

export const bridgeTransferListValidator = listingArray(bridgeTransferValidator);

/** The chain ids TigerBridge supports, from `GET /bridge/chains`. */
export const bridgeChainsValidator: Validator<number[]> = (value, path = "value") => {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    throw new ValidationError("expected a bridge chains envelope", path);
  }
  const chains = (value as Record<string, unknown>).chains;
  if (!Array.isArray(chains)) {
    throw new ValidationError("expected a chains array", `${path}.chains`);
  }
  return chains.map((entry, index) => integer(entry, `${path}.chains[${index}]`));
};

/**
 * One live market tick from the WebSocket feed at `/ws`.
 *
 * The server publishes real oracle prices on this channel, so the payload is
 * checked before it can reach a component: a malformed frame is dropped rather
 * than rendered as a price. `timestamp` is Unix seconds, matching the Go
 * producer.
 */
export interface MarketTick {
  type: "price";
  symbol: string;
  priceUSD: number;
  change24h: number;
  timestamp: number;
}

export const marketTickValidator = object<MarketTick>({
  type: oneOf(["price"] as const),
  symbol,
  priceUSD: nonNegative,
  change24h: number,
  timestamp: integer,
});

/* ------------------------------------------------------------------ farms */

/** A liquidity-mining farm listing from `GET /farms`. */
export interface FarmPool {
  id: string;
  key: string;
  name: string;
  farm_type: string;
  chain_id: number;
  staking_token_address: string;
  staking_token_symbol: string;
  staking_token_decimals: number;
  total_staked: string;
  start_time: string;
  end_time: string;
  status: string;
  created_at: string;
  updated_at: string;
}

export const farmPoolValidator = object<FarmPool>(
  {
    id: string,
    key: string,
    name: displayText,
    farm_type: string,
    chain_id: integer,
    staking_token_address: string,
    staking_token_symbol: string,
    staking_token_decimals: integer,
    total_staked: withDefault(decimal, "0"),
    start_time: withDefault(timestamp, new Date(0).toISOString()),
    end_time: withDefault(timestamp, new Date(0).toISOString()),
    status: string,
    created_at: withDefault(timestamp, new Date(0).toISOString()),
    updated_at: withDefault(timestamp, new Date(0).toISOString()),
  },
  { optional: ["total_staked", "start_time", "end_time", "created_at", "updated_at"] },
);

/** One reward stream of a farm. */
export interface FarmReward {
  id: string;
  farm_id: string;
  token_address: string;
  token_symbol: string;
  token_decimals: number;
  rate_per_second: string;
  acc_reward_per_share: string;
  total_reward: string;
  total_distributed: string;
}

export const farmRewardValidator = object<FarmReward>({
  id: string,
  farm_id: string,
  token_address: string,
  token_symbol: string,
  token_decimals: integer,
  rate_per_second: withDefault(decimal, "0"),
  acc_reward_per_share: withDefault(decimal, "0"),
  total_reward: withDefault(decimal, "0"),
  total_distributed: withDefault(decimal, "0"),
});

/**
 * A farm with its reward streams and APR.
 *
 * `apr` is null when the oracle price for a token is unavailable; the UI shows
 * a dash rather than a fabricated yield.
 */
export interface FarmView {
  farm: FarmPool;
  rewards: FarmReward[];
  apr: number | null;
}

export const farmViewValidator = object<FarmView>({
  farm: farmPoolValidator,
  rewards: withDefault(arrayLenient(farmRewardValidator), []),
  apr: nullable(nonNegative),
});

export const farmViewListValidator = listingArray(farmViewValidator);

/** One pending reward stream of a user's farm stake. */
export interface FarmPendingReward {
  token_address: string;
  symbol: string;
  decimals: number;
  pending: string;
}

export const farmPendingRewardValidator = object<FarmPendingReward>({
  token_address: string,
  symbol: string,
  decimals: integer,
  pending: withDefault(decimal, "0"),
});

/** A user's position in a farm, from `GET /farms/stakes`. */
export interface FarmStakeView {
  stake_id: string;
  farm_id: string;
  farm_key: string;
  farm_name: string;
  farm_type: string;
  status: string;
  chain_id: number;
  staking_token_address: string;
  staking_token_symbol: string;
  staked: string;
  end_time: string;
  pending_rewards: FarmPendingReward[];
  checked_at: string;
}

export const farmStakeViewValidator = object<FarmStakeView>({
  stake_id: string,
  farm_id: string,
  farm_key: withDefault(string, ""),
  farm_name: withDefault(displayText, ""),
  farm_type: withDefault(string, ""),
  status: withDefault(string, ""),
  chain_id: withDefault(integer, 0),
  staking_token_address: withDefault(string, ""),
  staking_token_symbol: withDefault(string, ""),
  staked: withDefault(decimal, "0"),
  end_time: withDefault(timestamp, new Date(0).toISOString()),
  pending_rewards: withDefault(arrayLenient(farmPendingRewardValidator), []),
  checked_at: withDefault(timestamp, new Date(0).toISOString()),
});

export const farmStakeViewListValidator = listingArray(farmStakeViewValidator);

/** The result of claiming farm rewards. */
export interface FarmClaimResult {
  success: boolean;
  claimed: Array<{ token_address?: string; symbol: string; amount: string }>;
}

export const farmClaimResultValidator = object<FarmClaimResult>({
  success: boolean,
  claimed: withDefault(arrayLenient(object<{ token_address?: string; symbol: string; amount: string }>({
    token_address: withDefault(string, ""),
    symbol: string,
    amount: decimal,
  })), []),
});

/* --------------------------------------------------------------- airdrops */

/** An airdrop campaign from `GET /airdrops`. */
export interface AirdropCampaign {
  id: string;
  key: string;
  name: string;
  chain_id: number;
  token_address: string;
  token_symbol: string;
  token_decimals: number;
  total_allocated: string;
  funded: string;
  total_claimed: string;
  start_time: string;
  end_time: string;
  status: string;
}

export const airdropCampaignValidator = object<AirdropCampaign>(
  {
    id: string,
    key: string,
    name: displayText,
    chain_id: integer,
    token_address: string,
    token_symbol: string,
    token_decimals: integer,
    total_allocated: withDefault(decimal, "0"),
    funded: withDefault(decimal, "0"),
    total_claimed: withDefault(decimal, "0"),
    start_time: withDefault(timestamp, new Date(0).toISOString()),
    end_time: withDefault(timestamp, new Date(0).toISOString()),
    status: string,
  },
  { optional: ["total_allocated", "funded", "total_claimed", "start_time", "end_time"] },
);

export const airdropCampaignListValidator = listingArray(airdropCampaignValidator);

/** A user's airdrop entitlement, from `GET /airdrops/allocations`. */
export interface AirdropAllocationView {
  allocation_id: string;
  campaign_id: string;
  campaign_key: string;
  campaign_name: string;
  token_address: string;
  symbol: string;
  amount: string;
  claimed: boolean;
  status: string;
  start_time: string;
  end_time: string;
}

export const airdropAllocationViewValidator = object<AirdropAllocationView>({
  allocation_id: string,
  campaign_id: string,
  campaign_key: withDefault(string, ""),
  campaign_name: withDefault(displayText, ""),
  token_address: withDefault(string, ""),
  symbol: withDefault(string, ""),
  amount: withDefault(decimal, "0"),
  claimed: withDefault(boolean, false),
  status: withDefault(string, ""),
  start_time: withDefault(timestamp, new Date(0).toISOString()),
  end_time: withDefault(timestamp, new Date(0).toISOString()),
});

export const airdropAllocationViewListValidator = listingArray(airdropAllocationViewValidator);

/** The result of claiming an airdrop. */
export interface AirdropClaimResult {
  success: boolean;
  campaign_id: string;
  token_address: string;
  symbol: string;
  amount: string;
}

export const airdropClaimResultValidator = object<AirdropClaimResult>({
  success: boolean,
  campaign_id: string,
  token_address: string,
  symbol: string,
  amount: decimal,
});

/* ---------------------------------------------------------------- options */

/**
 * An option series from `GET /options/series`.
 *
 * Every monetary field is an exact decimal string. `strike`, `base_iv` and
 * `mark_price` are plain decimals, while the pool's collateral and size fields
 * are integers in the settlement token's smallest unit.
 */
export interface OptionSeries {
  id: string;
  key: string;
  name: string;
  chain_id: number;
  underlying_symbol: string;
  collateral_token_address: string;
  collateral_token_symbol: string;
  collateral_token_decimals: number;
  strike: string;
  base_iv: string;
  expiry: string;
  is_call: boolean;
  mark_price: string;
  collateral: string;
  locked_collateral: string;
  premium_collected: string;
  total_long_size: string;
  settlement_price?: string;
  status: string;
  created_by: string;
  created_at: string;
  updated_at: string;
}

export const optionSeriesValidator = object<OptionSeries>(
  {
    id: string,
    key: string,
    name: displayText,
    chain_id: integer,
    underlying_symbol: string,
    collateral_token_address: withDefault(string, ""),
    collateral_token_symbol: string,
    collateral_token_decimals: withDefault(integer, 18),
    strike: decimal,
    base_iv: decimal,
    expiry: timestamp,
    is_call: boolean,
    mark_price: withDefault(decimal, "0"),
    collateral: withDefault(decimal, "0"),
    locked_collateral: withDefault(decimal, "0"),
    premium_collected: withDefault(decimal, "0"),
    total_long_size: withDefault(decimal, "0"),
    settlement_price: withDefault(decimal, "0"),
    status: string,
    created_by: withDefault(string, ""),
    created_at: withDefault(timestamp, new Date(0).toISOString()),
    updated_at: withDefault(timestamp, new Date(0).toISOString()),
  },
  {
    optional: [
      "collateral_token_address",
      "collateral_token_decimals",
      "mark_price",
      "collateral",
      "locked_collateral",
      "premium_collected",
      "total_long_size",
      "settlement_price",
      "created_by",
      "created_at",
      "updated_at",
    ],
  },
);

/**
 * A series with its live greeks and unit premium, from `GET /options/series`.
 *
 * `expired` is derived on the server so the UI does not have to compare clocks.
 */
export interface OptionSeriesView {
  series: OptionSeries;
  implied_volatility: string;
  delta: string;
  gamma: string;
  vega: string;
  theta: string;
  premium_per_unit: string;
  expired: boolean;
}

export const optionSeriesViewValidator = object<OptionSeriesView>({
  series: optionSeriesValidator,
  implied_volatility: withDefault(decimal, "0"),
  delta: withDefault(decimal, "0"),
  gamma: withDefault(decimal, "0"),
  vega: withDefault(decimal, "0"),
  theta: withDefault(decimal, "0"),
  premium_per_unit: withDefault(decimal, "0"),
  expired: withDefault(boolean, false),
});

export const optionSeriesViewListValidator = listingArray(optionSeriesViewValidator);

/** A holder's option position, from `GET /options/positions`. */
export interface OptionPositionView {
  position: {
    id: string;
    series_id: string;
    user_id: string;
    size: string;
    premium_paid: string;
    claimed_payout: string;
    created_at: string;
    updated_at: string;
  };
  series: OptionSeries;
  current_premium: string;
  payout_estimate: string;
}

export const optionPositionViewValidator = object<OptionPositionView>({
  position: object({
    id: string,
    series_id: string,
    user_id: withDefault(string, ""),
    size: withDefault(decimal, "0"),
    premium_paid: withDefault(decimal, "0"),
    claimed_payout: withDefault(decimal, "0"),
    created_at: withDefault(timestamp, new Date(0).toISOString()),
    updated_at: withDefault(timestamp, new Date(0).toISOString()),
  }),
  series: optionSeriesValidator,
  current_premium: withDefault(decimal, "0"),
  payout_estimate: withDefault(decimal, "0"),
});

export const optionPositionViewListValidator = listingArray(optionPositionViewValidator);

/** The result of buying an option. */
export interface OptionBuyResult {
  success: boolean;
  premium: string;
  size: string;
}

export const optionBuyResultValidator = object<OptionBuyResult>({
  success: boolean,
  premium: decimal,
  size: decimal,
});

/** The result of exercising a settled option. */
export interface OptionExerciseResult {
  success: boolean;
  payout: string;
}

export const optionExerciseResultValidator = object<OptionExerciseResult>({
  success: boolean,
  payout: decimal,
});

/** The admin create/update/settle/refresh endpoints wrap the view in `series`. */
export const optionSeriesEnvelopeValidator = envelope("series", optionSeriesViewValidator);

// ---------------------------------------------------------------------------
// Self-built NFT marketplace.
//
// A collection groups tokens; a token is owned by exactly one account and is
// moved to the escrow sentinel while a live listing holds it. A listing or an
// offer freezes its fee bps at creation, so the amount the seller receives is
// fixed even if the marketplace fee changes later.
// ---------------------------------------------------------------------------

/** A collection as returned by `/nft/collections`. */
export interface NFTCollection {
  id: string;
  key: string;
  name: string;
  symbol: string;
  chain_id: number;
  contract_address: string;
  base_uri: string;
  total_supply: number;
  status: string;
  created_by: string;
  created_at: string;
  updated_at: string;
}

export const nftCollectionValidator = object<NFTCollection>(
  {
    id: string,
    key: string,
    name: displayText,
    symbol: string,
    chain_id: integer,
    contract_address: withDefault(string, ""),
    base_uri: withDefault(string, ""),
    total_supply: withDefault(integer, 0),
    status: string,
    created_by: withDefault(string, ""),
    created_at: withDefault(timestamp, new Date(0).toISOString()),
    updated_at: withDefault(timestamp, new Date(0).toISOString()),
  },
  { optional: ["contract_address", "base_uri", "total_supply", "created_by", "created_at", "updated_at"] },
);

/** A token as returned by `/nft/tokens/mine`. */
export interface NFTToken {
  id: string;
  collection_id: string;
  token_id: number;
  owner_user_id: string;
  uri: string;
  escrow_listing_id?: string | null;
  minted_by: string;
  created_at: string;
  updated_at: string;
}

export const nftTokenValidator = object<NFTToken>(
  {
    id: string,
    collection_id: string,
    token_id: integer,
    owner_user_id: withDefault(string, ""),
    uri: withDefault(string, ""),
    escrow_listing_id: withDefault(nullable(string), null),
    minted_by: withDefault(string, ""),
    created_at: withDefault(timestamp, new Date(0).toISOString()),
    updated_at: withDefault(timestamp, new Date(0).toISOString()),
  },
  { optional: ["owner_user_id", "uri", "escrow_listing_id", "minted_by", "created_at", "updated_at"] },
);

/** A fixed-price listing, from `/nft/listings`. */
export interface NFTListing {
  id: string;
  collection_id: string;
  token_id: number;
  seller_user_id: string;
  payment_token_address: string;
  payment_token_symbol: string;
  payment_token_decimals: number;
  price: string;
  fee_bps: number;
  deadline: string;
  status: string;
  created_at: string;
  updated_at: string;
}

export const nftListingValidator = object<NFTListing>(
  {
    id: string,
    collection_id: string,
    token_id: integer,
    seller_user_id: withDefault(string, ""),
    payment_token_address: withDefault(string, ""),
    payment_token_symbol: string,
    payment_token_decimals: withDefault(integer, 18),
    price: decimal,
    fee_bps: withDefault(integer, 0),
    deadline: timestamp,
    status: string,
    created_at: withDefault(timestamp, new Date(0).toISOString()),
    updated_at: withDefault(timestamp, new Date(0).toISOString()),
  },
  {
    optional: [
      "seller_user_id",
      "payment_token_address",
      "payment_token_decimals",
      "fee_bps",
      "created_at",
      "updated_at",
    ],
  },
);

/** A live listing joined with its token and collection, from `GET /nft/listings`. */
export interface NFTListingView {
  listing: NFTListing;
  token: NFTToken;
  collection: NFTCollection;
  active: boolean;
}

export const nftListingViewValidator = object<NFTListingView>({
  listing: nftListingValidator,
  token: nftTokenValidator,
  collection: nftCollectionValidator,
  active: withDefault(boolean, true),
});

export const nftListingViewListValidator = listingArray(nftListingViewValidator);

/** An escrowed bid, from `/nft/offers`. */
export interface NFTOffer {
  id: string;
  collection_id: string;
  token_id: number;
  buyer_user_id: string;
  payment_token_address: string;
  payment_token_symbol: string;
  payment_token_decimals: number;
  amount: string;
  fee_bps: number;
  deadline: string;
  status: string;
  created_at: string;
  updated_at: string;
}

export const nftOfferValidator = object<NFTOffer>(
  {
    id: string,
    collection_id: string,
    token_id: integer,
    buyer_user_id: withDefault(string, ""),
    payment_token_address: withDefault(string, ""),
    payment_token_symbol: string,
    payment_token_decimals: withDefault(integer, 18),
    amount: decimal,
    fee_bps: withDefault(integer, 0),
    deadline: timestamp,
    status: string,
    created_at: withDefault(timestamp, new Date(0).toISOString()),
    updated_at: withDefault(timestamp, new Date(0).toISOString()),
  },
  {
    optional: [
      "buyer_user_id",
      "payment_token_address",
      "payment_token_decimals",
      "fee_bps",
      "created_at",
      "updated_at",
    ],
  },
);

/** A live offer joined with its token and collection, from `GET /nft/offers`. */
export interface NFTOfferView {
  offer: NFTOffer;
  token: NFTToken;
  collection: NFTCollection;
  active: boolean;
}

export const nftOfferViewValidator = object<NFTOfferView>({
  offer: nftOfferValidator,
  token: nftTokenValidator,
  collection: nftCollectionValidator,
  active: withDefault(boolean, true),
});

export const nftOfferViewListValidator = listingArray(nftOfferViewValidator);

/** The marketplace fee configuration, from `GET /nft/config`. */
export interface NFTMarketplaceConfig {
  escrow_account: string;
  fee_account: string;
  fee_bps: number;
  max_fee_bps: number;
}

export const nftMarketplaceConfigValidator = object<NFTMarketplaceConfig>({
  escrow_account: withDefault(string, ""),
  fee_account: withDefault(string, ""),
  fee_bps: withDefault(integer, 0),
  max_fee_bps: withDefault(integer, 500),
});

/** The result of buying a listing: the settled listing comes back. */
export const nftListingEnvelopeValidator = envelope("listing", nftListingValidator);

/** The result of opening an offer: the escrowed offer comes back. */
export const nftOfferEnvelopeValidator = envelope("offer", nftOfferValidator);

/** The admin create/mint endpoints wrap the collection or token. */
export const nftCollectionEnvelopeValidator = envelope("collection", nftCollectionValidator);
export const nftTokenEnvelopeValidator = envelope("token", nftTokenValidator);

export const nftCollectionListValidator = listingArray(nftCollectionValidator);
export const nftTokenListValidator = listingArray(nftTokenValidator);
export const nftListingListValidator = listingArray(nftListingValidator);
export const nftOfferListValidator = listingArray(nftOfferValidator);

/* ------------------------------------------------------------- analytics */

/**
 * The protocol dashboard snapshot from `GET /analytics/overview`. Every field
 * is a real count or sum computed from rows TigerSwap wrote; a field the server
 * could not value (for example open interest with no price source) comes back as
 * zero rather than a guess.
 */
export interface ProtocolOverview {
  total_tvl_usd: number;
  volume_24h_usd: number;
  fees_24h_usd: number;
  volume_7d_usd: number;
  fees_7d_usd: number;
  total_pools: number;
  active_pools: number;
  total_farms: number;
  active_farms: number;
  total_tokens: number;
  total_chains: number;
  active_chains: number;
  total_users: number;
  total_trades_24h: number;
  open_interest_usd: number;
  margin_borrowed_usd: number;
  nft_listings_active: number;
  updated_at: string | null;
}

export const protocolOverviewValidator = object<ProtocolOverview>({
  total_tvl_usd: nonNegative,
  volume_24h_usd: nonNegative,
  fees_24h_usd: nonNegative,
  volume_7d_usd: nonNegative,
  fees_7d_usd: nonNegative,
  total_pools: integer,
  active_pools: integer,
  total_farms: integer,
  active_farms: integer,
  total_tokens: integer,
  total_chains: integer,
  active_chains: integer,
  total_users: integer,
  total_trades_24h: integer,
  open_interest_usd: nonNegative,
  margin_borrowed_usd: nonNegative,
  nft_listings_active: integer,
  updated_at: nullable(timestamp),
});

/** One point of a volume series from `GET /analytics/volume`. */
export interface VolumePoint {
  timestamp: string;
  volume_usd: number;
  fees_usd: number;
  trade_count: number;
  taker_buy_volume_usd: number;
  taker_sell_volume_usd: number;
}

export const volumePointValidator = object<VolumePoint>({
  timestamp: timestamp,
  volume_usd: nonNegative,
  fees_usd: nonNegative,
  trade_count: integer,
  taker_buy_volume_usd: nonNegative,
  taker_sell_volume_usd: nonNegative,
});

/** The envelope returned by `/analytics/volume`: the series plus its params. */
export interface VolumeSeriesResponse {
  pair: string;
  chain_id: number;
  interval: number;
  series: VolumePoint[];
}

export const volumeSeriesValidator = object<VolumeSeriesResponse>({
  pair: withDefault(string, ""),
  chain_id: integer,
  interval: integer,
  series: array(volumePointValidator),
});

/** Per-market activity from `GET /analytics/pairs`. */
export interface PairAnalytics {
  pair: string;
  chain_id: number;
  volume_usd: number;
  fees_usd: number;
  trade_count: number;
  taker_buy_volume_usd: number;
  taker_sell_volume_usd: number;
}

export const pairAnalyticsValidator = object<PairAnalytics>({
  pair: string,
  chain_id: integer,
  volume_usd: nonNegative,
  fees_usd: nonNegative,
  trade_count: integer,
  taker_buy_volume_usd: nonNegative,
  taker_sell_volume_usd: nonNegative,
});

export const pairAnalyticsListValidator = listingArray(pairAnalyticsValidator);

/** One row of the PnL leaderboard from `GET /analytics/traders`. */
export interface TopTrader {
  user_id: string;
  realized_pnl: number;
  volume_usd: number;
  closed_trades: number;
  win_rate: number;
  liquidations: number;
}

export const topTraderValidator = object<TopTrader>({
  user_id: string,
  realized_pnl: number,
  volume_usd: nonNegative,
  closed_trades: integer,
  win_rate: nonNegative,
  liquidations: integer,
});

export const topTraderListValidator = listingArray(topTraderValidator);

/**
 * The result of an admin rollup from `POST /admin/analytics/rollup`. The window
 * is rebuilt rather than appended to, so the same call can be repeated after a
 * reorg without double-counting.
 */
export interface AnalyticsRollupResult {
  buckets_written: number;
  from: string;
  to: string;
}

export const analyticsRollupValidator = object<AnalyticsRollupResult>({
  buckets_written: integer,
  from: timestamp,
  to: timestamp,
});
