import assert from "node:assert/strict";
import { test } from "node:test";

import {
  adminAuditLogListValidator,
  adminChainListValidator,
  adminDashboardValidator,
  adminSettingsValidator,
  adminTokenListValidator,
  chainListValidator,
  futuresMarketListValidator,
  marginListingListValidator,
  marketStatsValidator,
  perpMarketListValidator,
  perpsListingListValidator,
  poolListValidator,
  priceListValidator,
  swapQuoteValidator,
  tokenListValidator,
  tradeListValidator,
} from "./types";

// Every payload below was captured verbatim from a running TigerSwap API
// (`go run ./cmd/api` against PostgreSQL) with curl. The point of this file is
// to lock the wire format the backend actually emits to the validators the UI
// enforces. A change on either side that breaks the other fails here, in a
// unit test, instead of blanking a page in production.

const SERVER_CHAINS = `[{"id":1,"name":"Ethereum","chainType":"evm","nativeSymbol":"ETH","explorerURL":"https://etherscan.io","isActive":true},{"id":10,"name":"Optimism","chainType":"evm","nativeSymbol":"ETH","explorerURL":"https://optimistic.etherscan.io","isActive":true}]`;

const SERVER_TOKENS = `[{"address":"0x0000000000000000000000000000000000000000","symbol":"ETH","name":"Ethereum","decimals":18,"chainId":1,"logoURI":"https://assets.coingecko.com/coins/images/279/small/ethereum.png","priceUSD":null},{"address":"0x514910771AF9Ca656af840dff83E8264EcF986CA","symbol":"LINK","name":"Chainlink","decimals":18,"chainId":1,"logoURI":null,"priceUSD":12.5}]`;

const SERVER_STATS = `{"totalTVL":0,"volume24h":0,"totalPairs":0,"totalUsers":0,"updatedAt":"2026-10-31T04:04:03Z"}`;

const SERVER_PAIRS = `[{"pairAddress":"0x3333333333333333333333333333333333333333","token0":"0x1111111111111111111111111111111111111111","token1":"0x2222222222222222222222222222222222222222","symbol0":"TGR","symbol1":"RUSD","reserve0":1000.5,"reserve1":2000.25,"feeBps":30}]`;

const SERVER_TRADES = `[{"id":"1","pair":"TGR/RUSD","side":"buy","price":1.5,"quantity":100,"timestamp":"2026-10-31T04:04:03Z"}]`;

const SERVER_ADMIN_DASHBOARD = `{"totalUsers":1,"totalTransactions":0,"totalVolume":0,"totalRevenue":0,"activePools":0,"updatedAt":"2026-10-31T04:06:12Z"}`;

const SERVER_ADMIN_CHAINS = `{"chains":[{"id":1,"chain_id":1,"name":"Ethereum","symbol":"ETH","icon":"🦄","rpc_url":"","explorer_url":"https://etherscan.io","type":"evm","is_active":true,"is_testnet":false,"created_at":"2026-10-31T04:03:52.275683Z","updated_at":"2026-10-31T04:03:52.275683Z"}]}`;

const SERVER_ADMIN_TOKENS = `{"count":1,"tokens":[{"id":1,"address":"0x0000000000000000000000000000000000000000","chain_id":1,"name":"Ethereum","symbol":"ETH","decimals":18,"logo_uri":"https://assets.coingecko.com/coins/images/279/small/ethereum.png","total_supply":"","is_active":true,"is_verified":true,"price":0,"created_at":"2026-10-31T04:03:52.287995Z","updated_at":"2026-10-31T04:03:52.287995Z"}]}`;

const SERVER_ADMIN_SETTINGS = `{"id":1,"key":"default","maintenance_mode":false,"trading_enabled":true,"withdrawal_enabled":true,"deposit_enabled":true,"new_user_registration_enabled":true,"updated_at":"2026-10-31T04:06:13.449935363Z"}`;

const SERVER_ADMIN_LOGS = `{"count":0,"logs":[]}`;

const SERVER_PERP_MARKETS = `{"markets":[]}`;

const SERVER_FUTURES_LIST = `{"count":1,"markets":[{"id":"186c6bc6-619b-4d36-87a6-e9be2dfdced6","symbol":"BTC/RUSD","contract_type":"RUSD_M","base_asset":"BTC","quote_asset":"RUSD","settlement":"RUSD","contract_size":"1","tick_size":"0.1","max_leverage":200,"min_order_size":"1","funding_rate":"0.0001","funding_interval_hours":8,"funding_rate_cap":"0.01","oracle_sources":"","maintenance_margin_rate":0.005,"initial_margin_rate":0.008,"liquidation_fee_rate":0.005,"insurance_fund_share":0.1,"maker_fee_rate":0,"taker_fee_rate":0.0002,"status":"paused","listing_type":"perpetual","is_tradfi":false,"created_at":"2026-10-31T04:06:45.968928Z","updated_at":"2026-10-31T04:06:45.968928Z"}]}`;

// /perps/markets and /margin/markets share the futures envelope shape; these
// payloads mirror what the same handlers return for their own tables.
const SERVER_PERPS_LIST = `{"count":1,"markets":[{"id":"bbe4240e-bf10-443e-a295-1486d1e93f60","symbol":"BTC/RUSD","market_type":"oracle","collateral_assets":"RUSD,USDT,USDC","max_open_interest":"1000000","max_leverage":1001,"price_impact_rate":0.0005,"borrow_fee_per_hour":0.0001,"maker_fee_rate":0,"taker_fee_rate":0.0002,"maintenance_margin_rate":0.005,"liquidation_fee_rate":0.005,"status":"paused","created_at":"2026-10-31T04:07:50.265667Z","updated_at":"2026-10-31T04:07:50.265667Z"}]}`;

const SERVER_MARGIN_LIST = `{"count":1,"markets":[{"id":"2f373904-0de5-4eeb-a2b4-3cf54c585f5d","pair":"BTC/RUSD","base_asset":"BTC","quote_asset":"RUSD","margin_mode":"cross","max_leverage":10,"collateral_assets":"BTC,ETH,RUSD,USDT,USDC","borrowable_assets":"RUSD,USDT,USDC","base_rate_apy":0.05,"kink_utilization":0.8,"jump_multiplier":1,"ltv":0.7,"liquidation_threshold":0.75,"liquidation_bonus":0.05,"close_factor":0.5,"supply_cap":"10000000","borrow_cap":"10000000","risk_tier":"low","status":"paused","created_at":"2026-10-31T04:07:50.490874Z","updated_at":"2026-10-31T04:07:50.490874Z"}]}`;

const SERVER_QUOTE = `{"amountIn":"1000000000000000000","amountOut":"2000000000000000000","amountOutMin":"1990000000000000000","priceImpact":0.12,"route":["0x1111111111111111111111111111111111111111","0x2222222222222222222222222222222222222222"],"gasEstimate":150000}`;

function parse<T>(validator: (value: unknown, path?: string) => T, payload: string, label: string): T {
  try {
    return validator(JSON.parse(payload), label);
  } catch (error) {
    throw new Error(`${label} rejected the live server payload: ${(error as Error).message}`);
  }
}

test("live /chains payload satisfies chainListValidator", () => {
  const chains = parse(chainListValidator, SERVER_CHAINS, "chains");
  assert.equal(chains.length, 2);
  assert.equal(chains[0].id, 1);
  assert.equal(chains[0].chainType, "evm");
  assert.equal(chains[0].nativeSymbol, "ETH");
});

test("live /tokens payload satisfies tokenListValidator", () => {
  const tokens = parse(tokenListValidator, SERVER_TOKENS, "tokens");
  assert.equal(tokens.length, 2);
  assert.equal(tokens[0].symbol, "ETH");
  assert.equal(tokens[0].priceUSD, null);
  assert.equal(tokens[1].priceUSD, 12.5);
});

test("live /market/stats payload satisfies marketStatsValidator", () => {
  const stats = parse(marketStatsValidator, SERVER_STATS, "stats");
  assert.equal(stats.totalTVL, 0);
  assert.equal(stats.totalPairs, 0);
  assert.equal(typeof stats.updatedAt, "string");
});

test("live /swap/pairs payload satisfies poolListValidator", () => {
  const pools = parse(poolListValidator, SERVER_PAIRS, "pairs");
  assert.equal(pools.length, 1);
  assert.equal(pools[0].feeBps, 30);
  assert.equal(pools[0].symbol0, "TGR");
});

test("live /market/trades payload satisfies tradeListValidator", () => {
  const trades = parse(tradeListValidator, SERVER_TRADES, "trades");
  assert.equal(trades.length, 1);
  assert.equal(trades[0].side, "buy");
});

test("live /prices payload (empty array) satisfies priceListValidator", () => {
  assert.deepEqual(parse(priceListValidator, "[]", "prices"), []);
});

test("live /admin/dashboard payload satisfies adminDashboardValidator", () => {
  const dash = parse(adminDashboardValidator, SERVER_ADMIN_DASHBOARD, "dashboard");
  assert.equal(dash.totalUsers, 1);
  assert.equal(dash.activePools, 0);
});

test("live /admin/chains payload satisfies adminChainListValidator", () => {
  const chains = parse(adminChainListValidator, SERVER_ADMIN_CHAINS, "adminChains");
  assert.equal(chains.length, 1);
  assert.equal(chains[0].chain_id, 1);
});

test("live /admin/tokens payload satisfies adminTokenListValidator", () => {
  const tokens = parse(adminTokenListValidator, SERVER_ADMIN_TOKENS, "adminTokens");
  assert.equal(tokens.length, 1);
  assert.equal(tokens[0].symbol, "ETH");
});

test("live /admin/settings payload satisfies adminSettingsValidator", () => {
  const settings = parse(adminSettingsValidator, SERVER_ADMIN_SETTINGS, "adminSettings");
  assert.equal(settings.maintenance_mode, false);
  assert.equal(settings.trading_enabled, true);
});

test("live /admin/logs payload (empty) satisfies adminAuditLogListValidator", () => {
  assert.deepEqual(parse(adminAuditLogListValidator, SERVER_ADMIN_LOGS, "adminLogs"), []);
});

test("live /admin/futures/markets payload satisfies futuresMarketListValidator", () => {
  const markets = parse(futuresMarketListValidator, SERVER_FUTURES_LIST, "futures");
  assert.equal(markets.length, 1);
  assert.equal(markets[0].symbol, "BTC/RUSD");
  assert.equal(markets[0].contract_type, "RUSD_M");
  assert.equal(markets[0].max_leverage, 200);
  assert.equal(markets[0].status, "paused");
});

test("live /admin/perps/markets payload satisfies perpsListingListValidator", () => {
  const listings = parse(perpsListingListValidator, SERVER_PERPS_LIST, "perps");
  assert.equal(listings.length, 1);
  assert.equal(listings[0].max_leverage, 1001);
  assert.equal(listings[0].collateral_assets, "RUSD,USDT,USDC");
});

test("live /admin/margin/markets payload satisfies marginListingListValidator", () => {
  const listings = parse(marginListingListValidator, SERVER_MARGIN_LIST, "margin");
  assert.equal(listings.length, 1);
  assert.equal(listings[0].pair, "BTC/RUSD");
  assert.equal(listings[0].ltv, 0.7);
  assert.equal(listings[0].margin_mode, "cross");
});

test("live /perpetuals/markets payload satisfies perpMarketListValidator", () => {
  assert.deepEqual(parse(perpMarketListValidator, SERVER_PERP_MARKETS, "perpMarkets"), []);
});

test("live /swap/quote payload satisfies swapQuoteValidator", () => {
  const quote = parse(swapQuoteValidator, SERVER_QUOTE, "quote");
  assert.equal(quote.amountIn, "1000000000000000000");
  assert.equal(quote.amountOutMin, "1990000000000000000");
  assert.equal(quote.priceImpact, 0.12);
  assert.equal(quote.gasEstimate, 150000);
});
