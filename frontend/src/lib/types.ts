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
        record.projects;
      if (items !== undefined) {
        return arrayLenient(element)(items, `${path}.items`);
      }
    }
    return arrayLenient(element)(value, path);
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
