import assert from "node:assert/strict";
import { test } from "node:test";

import {
  adminAuditLogListValidator,
  adminChainListValidator,
  adminDashboardValidator,
  adminSettingsValidator,
  analyticsRollupValidator,
  adminTokenListValidator,
  botListValidator,
  botOrderListValidator,
  botPerformanceListValidator,
  botStrategyListValidator,
  botsSummaryValidator,
  bridgeChainsValidator,
  bridgeQuoteValidator,
  chainListValidator,
  copyPositionListValidator,
  copyTraderListValidator,
  futuresMarketListValidator,
  marginListingListValidator,
  marketStatsValidator,
  nftCollectionListValidator,
  nftCollectionEnvelopeValidator,
  nftListingEnvelopeValidator,
  nftListingListValidator,
  nftListingViewListValidator,
  nftMarketplaceConfigValidator,
  nftOfferEnvelopeValidator,
  nftOfferListValidator,
  nftOfferViewListValidator,
  nftTokenEnvelopeValidator,
  nftTokenListValidator,
  optionBuyResultValidator,
  optionExerciseResultValidator,
  optionPositionViewListValidator,
  optionSeriesEnvelopeValidator,
  optionSeriesViewListValidator,
  otcOrderListValidator,
  perpMarketListValidator,
  perpsListingListValidator,
  pairAnalyticsListValidator,
  poolListValidator,
  protocolOverviewValidator,
  priceListValidator,
  swapQuoteValidator,
  tokenListValidator,
  topTraderListValidator,
  tradeListValidator,
  volumeSeriesValidator,
  twammOrderListValidator,
  airdropAllocationViewListValidator,
  airdropCampaignListValidator,
  farmStakeViewListValidator,
  farmViewListValidator,
  portfolioValidator,
  userTransactionListValidator,
} from "./types";

// Every payload below was captured verbatim from a running TigerSwap API
// (`go run ./cmd/api` against PostgreSQL) with curl. The point of this file is
// to lock the wire format the backend actually emits to the validators the UI
// enforces. A change on either side that breaks the other fails here, in a
// unit test, instead of blanking a page in production.

// The portfolio and transaction payloads were captured by running the real
// handlers against an in-memory database in
// `backend/services/portfolio_service_test.go` and copying the logged JSON.
const SERVER_PORTFOLIO = `{"totalValueUSD":4000,"assets":[{"chainId":1,"tokenAddress":"0x0000000000000000000000000000000000000001","symbol":"ETH","decimals":18,"balance":"2000000000000000000","valueUSD":4000}],"realizedPnlUsd":0,"unrealizedPnlUsd":0,"openPositions":0,"updatedAt":"2026-11-08T12:20:36.201522492Z"}`;

const SERVER_TRANSACTIONS = `[{"hash":"0xdeadbeef","chainId":1,"type":"swap","status":"confirmed","amount":"1000000000000000000","tokenSymbol":"ETH","createdAt":"2026-11-08T12:20:36.201140162Z"}]`;

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

// The bots, OTC and TWAMM payloads below were captured by running the real
// handlers against an in-memory database in
// `backend/services/contract_capture_test.go` and copying the logged JSON.
const SERVER_BOT = `{"config":{"rebalance_threshold_bps":50,"spread_bps":25,"target_depth":"10000"},"created_at":"2026-10-31T16:17:02.420413229Z","filled_orders":0,"id":1,"last_run_at":"0001-01-01T00:00:00Z","max_daily_loss":"100","max_open_orders":10,"max_order_size":"5","name":"maker","realised_pnl":"0","status":"active","strategy":"market_maker","symbol":"BTC/RUSD","traded_volume":"0","updated_at":"2026-10-31T16:17:02.420413229Z"}`;

const SERVER_BOT_LIST = `[${SERVER_BOT}]`;

const SERVER_BOT_STRATEGIES = `[{"config_keys":{"max_spread_bps":"int","min_book_depth":"decimal","min_spread_bps":"int","rebalance_threshold_bps":"int","spread_bps":"int","target_depth":"decimal"},"strategy":"market_maker"},{"config_keys":{"interval_ms":"int","side":"side","slices":"int","total_quantity":"decimal"},"strategy":"twap"}]`;

const SERVER_BOTS_SUMMARY = `{"by_status":{"active":1},"by_strategy":{"market_maker":1},"total":1}`;

const SERVER_BOT_ORDERS = `[{"id":1,"bot_id":1,"user_id":"7","symbol":"BTC/RUSD","side":"buy","order_kind":"limit","price":"60000","quantity":"0.5","reduce_only":false,"source":"","status":"pending","created_at":"2026-10-31T16:17:16.218019274Z","updated_at":"2026-10-31T16:17:16.218019274Z"}]`;

const SERVER_BOT_PERFORMANCE = `[{"id":1,"bot_id":1,"day":"2026-10-31T00:00:00Z","orders_placed":3,"orders_filled":2,"volume":"3000","realised_pnl":"12.5","fees_paid":"0.6","max_drawdown":"4"}]`;

const SERVER_OTC_ORDER = `{"id":1,"maker_id":"7","symbol":"BTC/RUSD","side":"buy","quantity":"2","price":"60000","status":"open","min_fill":"0.5","expires_at":"2026-11-01T16:17:02.425322999Z","created_at":"2026-10-31T16:17:02.425348979Z","updated_at":"2026-10-31T16:17:02.425348979Z"}`;

const SERVER_OTC_ORDERS = `[${SERVER_OTC_ORDER}]`;

const SERVER_TWAMM_ORDER = `{"id":1,"user_id":"7","symbol":"BTC/RUSD","side":"buy","total_quantity":"10","filled_quantity":"0","limit_price":"0","start_time":"2026-10-31T16:17:02.428673919Z","end_time":"2026-11-01T16:17:02.428673919Z","status":"active","created_at":"2026-10-31T16:17:02.428691819Z","updated_at":"2026-10-31T16:17:02.428691819Z"}`;

const SERVER_TWAMM_ORDERS = `[${SERVER_TWAMM_ORDER}]`;

const SERVER_COPY_TRADER = `{"address":"0x1111111111111111111111111111111111111111","name":"Alpha Whale","avatar":"https://example.com/a.png","win_rate":0,"total_profit":"0","weekly_profit":0,"monthly_profit":0,"total_trades":0,"followers":0,"total_volume":"0","avg_holding_time":"","is_verified":false,"is_pro":false,"risk_score":"medium","strategies":null,"last_active":"2026-10-31T16:35:21.407518436Z"}`;

const SERVER_COPY_TRADERS = `{"count":1,"traders":[${SERVER_COPY_TRADER}]}`;

// Captured from `GET /copy-trading/positions` on a running server via
// `TestCopyPositionResponseWireFormat` in
// `backend/services/copy_trading_contract_test.go` (run with -v to regenerate).
const SERVER_COPY_POSITION = `{"id":"cp_1794312461797982574","follower_address":"0x1111111111111111111111111111111111111111","trader_address":"0x2222222222222222222222222222222222222222","original_position":"","pair":"BTC/RUSD","side":"long","size":"1000","entry_price":100,"current_price":100,"leverage":1,"pnl":"0","status":"open","opened_at":"2026-11-10T12:07:41.797983184Z","closed_at":null,"copied_at":"2026-11-10T12:07:41.797983234Z"}`;

const SERVER_COPY_POSITIONS = `{"count":1,"positions":[${SERVER_COPY_POSITION}]}`;

// Captured from `GET /bridge/chains` on a running server.
const SERVER_BRIDGE_CHAINS = `{"chains":[8453,43114,1,137,56,42161,10]}`;

// The quote handler emits this shape; captured verbatim from the handler's own
// JSON in `services/copy_trading_contract_test.go`.
const SERVER_BRIDGE_QUOTE = `{"src_chain":1,"dst_chain":137,"token":"0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48","amount_in":"1000000","amount_out":"999000","fee":"1000","fee_usd":0.001,"estimated_time":"15 minutes","router":"0x1234567890123456789012345678901234567890","price_impact":0}`;

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

test("live /bots payload satisfies botListValidator", () => {
  const bots = parse(botListValidator, SERVER_BOT_LIST, "bots");
  assert.equal(bots.length, 1);
  assert.equal(bots[0].strategy, "market_maker");
  assert.equal(bots[0].symbol, "BTC/RUSD");
  assert.equal(bots[0].max_open_orders, 10);
  assert.equal(bots[0].status, "active");
});

test("live /bots/strategies payload satisfies botStrategyListValidator", () => {
  const strategies = parse(botStrategyListValidator, SERVER_BOT_STRATEGIES, "botStrategies");
  assert.equal(strategies.length, 2);
  assert.equal(strategies[0].config_keys.spread_bps, "int");
});

test("live /bots/summary payload satisfies botsSummaryValidator", () => {
  const summary = parse(botsSummaryValidator, SERVER_BOTS_SUMMARY, "botsSummary");
  assert.equal(summary.total, 1);
  assert.equal(summary.by_status.active, 1);
  assert.equal(summary.by_strategy.market_maker, 1);
});

test("live /bots/:id/orders payload satisfies botOrderListValidator", () => {
  const orders = parse(botOrderListValidator, SERVER_BOT_ORDERS, "botOrders");
  assert.equal(orders.length, 1);
  assert.equal(orders[0].order_kind, "limit");
  assert.equal(orders[0].price, "60000");
});

test("live /bots/:id/performance payload satisfies botPerformanceListValidator", () => {
  const performance = parse(botPerformanceListValidator, SERVER_BOT_PERFORMANCE, "botPerformance");
  assert.equal(performance.length, 1);
  assert.equal(performance[0].realised_pnl, "12.5");
  assert.equal(performance[0].orders_filled, 2);
});

test("live /otc/orders payload satisfies otcOrderListValidator", () => {
  const orders = parse(otcOrderListValidator, SERVER_OTC_ORDERS, "otcOrders");
  assert.equal(orders.length, 1);
  assert.equal(orders[0].side, "buy");
  assert.equal(orders[0].min_fill, "0.5");
});

test("live /twamm/orders payload satisfies twammOrderListValidator", () => {
  const orders = parse(twammOrderListValidator, SERVER_TWAMM_ORDERS, "twammOrders");
  assert.equal(orders.length, 1);
  assert.equal(orders[0].total_quantity, "10");
  assert.equal(orders[0].status, "active");
});

test("live /copy-trading/traders payload satisfies copyTraderListValidator", () => {
  const traders = parse(copyTraderListValidator, SERVER_COPY_TRADERS, "copyTraders");
  assert.equal(traders.length, 1);
  assert.equal(traders[0].name, "Alpha Whale");
  assert.equal(traders[0].address, "0x1111111111111111111111111111111111111111");
  assert.deepEqual(traders[0].strategies, []);
});

test("live /copy-trading/positions payload satisfies copyPositionListValidator", () => {
  const positions = parse(copyPositionListValidator, SERVER_COPY_POSITIONS, "copyPositions");
  assert.equal(positions.length, 1);
  assert.equal(positions[0].follower_address, "0x1111111111111111111111111111111111111111");
  assert.equal(positions[0].trader_address, "0x2222222222222222222222222222222222222222");
  assert.equal(positions[0].pair, "BTC/RUSD");
  assert.equal(positions[0].side, "long");
  assert.equal(positions[0].status, "open");
  assert.equal(positions[0].closed_at, null);
});

test("live /bridge/chains payload satisfies bridgeChainsValidator", () => {
  const chains = parse(bridgeChainsValidator, SERVER_BRIDGE_CHAINS, "bridgeChains");
  assert.deepEqual(chains, [8453, 43114, 1, 137, 56, 42161, 10]);
});

test("live /bridge/quote payload satisfies bridgeQuoteValidator", () => {
  const quote = parse(bridgeQuoteValidator, SERVER_BRIDGE_QUOTE, "bridgeQuote");
  assert.equal(quote.src_chain, 1);
  assert.equal(quote.dst_chain, 137);
  assert.equal(quote.amount_in, "1000000");
  assert.equal(quote.amount_out, "999000");
  assert.equal(quote.estimated_time, "15 minutes");
});

// Farm and airdrop payloads captured from the running API, in the same way as
// the fixtures above. The farm view carries its reward streams and a nullable
// APR, and the stake view carries a nullable pending-reward list.

const SERVER_FARMS = `{"farms":[{"farm":{"id":"a84f971d-57f4-469f-a4d1-9c59bfd8561f","key":"tgr-rusd","name":"TGR/RUSD LP Farm","farm_type":"lp","chain_id":1,"staking_token_address":"0x1111111111111111111111111111111111111111","staking_token_symbol":"TGR-RUSD-LP","staking_token_decimals":18,"total_staked":"1000000000000000000000","start_time":"2026-11-01T00:00:00Z","end_time":"2026-12-01T00:00:00Z","status":"active","created_by":"","created_at":"2026-11-03T17:54:15.233625133Z","updated_at":"2026-11-03T17:54:15.233625133Z"},"rewards":[{"id":"b269f781-35ce-4cfc-a00d-e6db27bac226","farm_id":"a84f971d-57f4-469f-a4d1-9c59bfd8561f","token_address":"0x2222222222222222222222222222222222222222","token_symbol":"TGR","token_decimals":18,"rate_per_second":"1000000000000","acc_reward_per_share":"0","last_update_time":"2026-11-01T00:00:00Z","total_reward":"0","total_distributed":"0","created_at":"2026-11-03T17:54:15.233796723Z","updated_at":"2026-11-03T17:54:15.233796723Z"}],"apr":12.5}],"success":true}`;

const SERVER_FARMS_NO_PRICE = `{"farms":[{"farm":{"id":"a84f971d-57f4-469f-a4d1-9c59bfd8561f","key":"tgr-rusd","name":"TGR/RUSD LP Farm","farm_type":"lp","chain_id":1,"staking_token_address":"0x1111111111111111111111111111111111111111","staking_token_symbol":"TGR-RUSD-LP","staking_token_decimals":18,"total_staked":"0","start_time":"2026-11-01T00:00:00Z","end_time":"2026-12-01T00:00:00Z","status":"paused"},"rewards":[],"apr":null}],"success":true}`;

const SERVER_FARM_STAKES = `{"stakes":[{"chain_id":1,"checked_at":"2026-11-01T00:00:00Z","end_time":"2026-12-01T00:00:00Z","farm_id":"a84f971d-57f4-469f-a4d1-9c59bfd8561f","farm_key":"tgr-rusd","farm_name":"TGR/RUSD LP Farm","farm_type":"lp","pending_rewards":[{"decimals":18,"pending":"2500000000000","symbol":"TGR","token_address":"0x2222222222222222222222222222222222222222"}],"stake_id":"11111111-1111-1111-1111-111111111111","staked":"500000000000000000000","staking_token_address":"0x1111111111111111111111111111111111111111","staking_token_symbol":"TGR-RUSD-LP","status":"active"}],"success":true}`;

const SERVER_AIRDROPS = `{"campaigns":[{"id":"3e6aac81-848b-4585-9e85-0f8ab1634419","key":"tgr-genesis","name":"TGR Genesis","chain_id":1,"token_address":"0x3333333333333333333333333333333333333333","token_symbol":"TGR","token_decimals":18,"total_allocated":"750000000000000000000","funded":"750000000000000000000","total_claimed":"500000000000000000000","start_time":"2026-11-01T00:00:00Z","end_time":"2026-12-01T00:00:00Z","status":"active","created_by":"","created_at":"2026-11-03T17:54:15.234018093Z","updated_at":"2026-11-03T17:54:15.234018093Z"}],"success":true}`;

const SERVER_AIRDROP_ALLOCATIONS = `{"allocations":[{"allocation_id":"74796fa2-02c0-4cf6-9958-27bfe5d4c0f8","amount":"500000000000000000000","campaign_id":"3e6aac81-848b-4585-9e85-0f8ab1634419","campaign_key":"tgr-genesis","campaign_name":"TGR Genesis","claimed":true,"end_time":"2026-12-01T00:00:00Z","start_time":"2026-11-01T00:00:00Z","status":"active","symbol":"TGR","token_address":"0x3333333333333333333333333333333333333333"}],"success":true}`;

test("live /farms payload satisfies farmViewListValidator", () => {
  const farms = parse(farmViewListValidator, SERVER_FARMS, "farms");
  assert.equal(farms.length, 1);
  assert.equal(farms[0].farm.key, "tgr-rusd");
  assert.equal(farms[0].farm.total_staked, "1000000000000000000000");
  assert.equal(farms[0].rewards.length, 1);
  assert.equal(farms[0].rewards[0].token_symbol, "TGR");
  assert.equal(farms[0].apr, 12.5);
});

test("a farm with no oracle price has a null APR rather than a fabricated one", () => {
  const farms = parse(farmViewListValidator, SERVER_FARMS_NO_PRICE, "farms");
  assert.equal(farms.length, 1);
  assert.equal(farms[0].apr, null);
  assert.deepEqual(farms[0].rewards, []);
});

test("live /farms/stakes payload satisfies farmStakeViewListValidator", () => {
  const stakes = parse(farmStakeViewListValidator, SERVER_FARM_STAKES, "farmStakes");
  assert.equal(stakes.length, 1);
  assert.equal(stakes[0].staked, "500000000000000000000");
  assert.equal(stakes[0].pending_rewards.length, 1);
  assert.equal(stakes[0].pending_rewards[0].symbol, "TGR");
  assert.equal(stakes[0].pending_rewards[0].pending, "2500000000000");
});

test("live /airdrops payload satisfies airdropCampaignListValidator", () => {
  const campaigns = parse(airdropCampaignListValidator, SERVER_AIRDROPS, "airdrops");
  assert.equal(campaigns.length, 1);
  assert.equal(campaigns[0].key, "tgr-genesis");
  assert.equal(campaigns[0].funded, "750000000000000000000");
  assert.equal(campaigns[0].status, "active");
});

test("live /airdrops/allocations payload satisfies airdropAllocationViewListValidator", () => {
  const allocations = parse(airdropAllocationViewListValidator, SERVER_AIRDROP_ALLOCATIONS, "allocations");
  assert.equal(allocations.length, 1);
  assert.equal(allocations[0].amount, "500000000000000000000");
  assert.equal(allocations[0].claimed, true);
});

// The options payloads below were captured by running the real handlers in
// `backend/services/contract_capture_test.go` and copying the logged JSON.
const SERVER_OPTION_SERIES = `{"series":[{"series":{"id":"ffb3134c-45d0-4328-bacf-c982839a92c3","key":"btc-2000-20261204","name":"BTC RUSD option","chain_id":1,"underlying_symbol":"BTC","collateral_token_address":"0x3333333333333333333333333333333333333333","collateral_token_symbol":"RUSD","collateral_token_decimals":18,"strike":"2000","base_iv":"0.8","expiry":"2026-12-04T16:36:39Z","is_call":true,"mark_price":"2000","collateral":"0","locked_collateral":"0","premium_collected":"0","total_long_size":"0","status":"active","created_by":"admin-1","created_at":"2026-11-04T16:36:39.075177715Z","updated_at":"2026-11-04T16:36:39.075177715Z"},"implied_volatility":"0.8","delta":"0.552743034574114732","gamma":"0.000862101170526148","vega":"226.744329974896719742","theta":"-45.95666238517838708","premium_per_unit":"186.352821444661749979","expired":false}],"success":true}`;

const SERVER_OPTION_SERIES_DETAIL = `{"series":{"series":{"id":"ffb3134c-45d0-4328-bacf-c982839a92c3","key":"btc-2000-20261204","name":"BTC RUSD option","chain_id":1,"underlying_symbol":"BTC","collateral_token_address":"0x3333333333333333333333333333333333333333","collateral_token_symbol":"RUSD","collateral_token_decimals":18,"strike":"2000","base_iv":"0.8","expiry":"2026-12-04T16:36:39Z","is_call":true,"mark_price":"2000","collateral":"0","locked_collateral":"0","premium_collected":"0","total_long_size":"0","status":"active","created_by":"admin-1","created_at":"2026-11-04T16:36:39.075177715Z","updated_at":"2026-11-04T16:36:39.075177715Z"},"implied_volatility":"0.8","delta":"0.552743034574114732","gamma":"0.000862101170526148","vega":"226.744329974896719742","theta":"-45.95666238517838708","premium_per_unit":"186.352821444661749979","expired":false},"success":true}`;

const SERVER_OPTION_BUY = `{"premium":"186352821444661749979","size":"1","success":true}`;

const SERVER_OPTION_POSITIONS = `{"positions":[{"position":{"id":"35c86ad0-8c8e-47df-99a6-3ec7f5f7c005","series_id":"ffb3134c-45d0-4328-bacf-c982839a92c3","user_id":"trader-1","size":"1000000000000000000","premium_paid":"186352821444661749979","claimed_payout":"0","created_at":"2026-11-04T16:36:39.077671515Z","updated_at":"2026-11-04T16:36:39.077671515Z"},"series":{"id":"ffb3134c-45d0-4328-bacf-c982839a92c3","key":"btc-2000-20261204","name":"BTC RUSD option","chain_id":1,"underlying_symbol":"BTC","collateral_token_address":"0x3333333333333333333333333333333333333333","collateral_token_symbol":"RUSD","collateral_token_decimals":18,"strike":"2000","base_iv":"0.8","expiry":"2026-12-04T16:36:39Z","is_call":true,"mark_price":"2000","collateral":"1000186352821444661749979","locked_collateral":"2000000000000000000000","premium_collected":"186352821444661749979","total_long_size":"1000000000000000000","status":"active","created_by":"admin-1","created_at":"2026-11-04T16:36:39.075177715Z","updated_at":"2026-11-04T16:36:39.077558285Z"},"current_premium":"187.2595972548250677","payout_estimate":"0"}],"success":true}`;

const SERVER_OPTION_EXERCISE = `{"payout":"500000000000000000000","success":true}`;

test("live /options/series payload satisfies optionSeriesViewListValidator", () => {
  const series = parse(optionSeriesViewListValidator, SERVER_OPTION_SERIES, "options");
  assert.equal(series.length, 1);
  assert.equal(series[0].series.key, "btc-2000-20261204");
  assert.equal(series[0].series.is_call, true);
  assert.equal(series[0].premium_per_unit, "186.352821444661749979");
  assert.equal(series[0].delta, "0.552743034574114732");
  assert.equal(series[0].expired, false);
});

test("live /options/series/:id payload satisfies optionSeriesEnvelopeValidator", () => {
  const view = parse(optionSeriesEnvelopeValidator, SERVER_OPTION_SERIES_DETAIL, "optionSeries");
  assert.equal(view.series.strike, "2000");
  assert.equal(view.series.collateral_token_symbol, "RUSD");
  assert.equal(view.implied_volatility, "0.8");
});

test("live /options/series/:id/buy payload satisfies optionBuyResultValidator", () => {
  const result = parse(optionBuyResultValidator, SERVER_OPTION_BUY, "optionBuy");
  assert.equal(result.success, true);
  assert.equal(result.premium, "186352821444661749979");
  assert.equal(result.size, "1");
});

test("live /options/positions payload satisfies optionPositionViewListValidator", () => {
  const positions = parse(optionPositionViewListValidator, SERVER_OPTION_POSITIONS, "optionPositions");
  assert.equal(positions.length, 1);
  assert.equal(positions[0].position.size, "1000000000000000000");
  assert.equal(positions[0].current_premium, "187.2595972548250677");
  assert.equal(positions[0].series.key, "btc-2000-20261204");
});

test("live /options/series/:id/exercise payload satisfies optionExerciseResultValidator", () => {
  const result = parse(optionExerciseResultValidator, SERVER_OPTION_EXERCISE, "optionExercise");
  assert.equal(result.success, true);
  assert.equal(result.payout, "500000000000000000000");
});


// ---------------------------------------------------------------------------
// NFT marketplace fixtures, captured from the running server.
// ---------------------------------------------------------------------------

const SERVER_NFT_COLLECTIONS = `{"collections":[{"id":"cde0d70d-2ec6-4ea0-9760-1402d0d6a4e5","key":"tiger-art","name":"Tiger Art","symbol":"TART","chain_id":1,"contract_address":"0x5555555555555555555555555555555555555555","base_uri":"https://cdn.tigerswap.example/nft/","total_supply":1,"status":"active","created_by":"admin-1","created_at":"2026-11-05T13:20:05.561048696Z","updated_at":"2026-11-05T13:20:05.561651656Z"}],"success":true}`;

const SERVER_NFT_CONFIG = `{"escrow_account":"__nft_escrow__","fee_account":"__nft_fee__","fee_bps":250,"max_fee_bps":500,"success":true}`;

const SERVER_NFT_LISTINGS = `{"listings":[{"listing":{"id":"adeb0115-2c27-4b0c-94fa-921ae785f21b","collection_id":"cde0d70d-2ec6-4ea0-9760-1402d0d6a4e5","token_id":1,"seller_user_id":"seller-1","payment_token_address":"0x4444444444444444444444444444444444444444","payment_token_symbol":"RUSD","payment_token_decimals":18,"price":"100000","fee_bps":250,"deadline":"2026-11-06T13:20:05Z","status":"active","created_at":"2026-11-05T13:20:05.562507656Z","updated_at":"2026-11-05T13:20:05.562507656Z"},"token":{"id":"9933c5ed-5c6d-425e-a87b-5305847a98af","collection_id":"cde0d70d-2ec6-4ea0-9760-1402d0d6a4e5","token_id":1,"owner_user_id":"__nft_escrow__","uri":"token.json","escrow_listing_id":"adeb0115-2c27-4b0c-94fa-921ae785f21b","minted_by":"admin-1","created_at":"2026-11-05T13:20:05.561573566Z","updated_at":"2026-11-05T13:20:05.562606666Z"},"collection":{"id":"cde0d70d-2ec6-4ea0-9760-1402d0d6a4e5","key":"tiger-art","name":"Tiger Art","symbol":"TART","chain_id":1,"contract_address":"0x5555555555555555555555555555555555555555","base_uri":"https://cdn.tigerswap.example/nft/","total_supply":1,"status":"active","created_by":"admin-1","created_at":"2026-11-05T13:20:05.561048696Z","updated_at":"2026-11-05T13:20:05.561651656Z"},"active":true}],"success":true}`;

const SERVER_NFT_TOKENS_MINE = `{"success":true,"tokens":[{"id":"a85a83f9-5ed6-4630-8945-0e8ca3f115cd","collection_id":"f673b944-e119-428c-8cfc-702b7bf65677","token_id":1,"owner_user_id":"seller-1","uri":"token.json","minted_by":"admin-1","created_at":"2026-11-05T13:20:15.502600879Z","updated_at":"2026-11-05T13:20:15.502600879Z"}]}`;

const SERVER_NFT_BUY = `{"listing":{"id":"adeb0115-2c27-4b0c-94fa-921ae785f21b","collection_id":"cde0d70d-2ec6-4ea0-9760-1402d0d6a4e5","token_id":1,"seller_user_id":"seller-1","payment_token_address":"0x4444444444444444444444444444444444444444","payment_token_symbol":"RUSD","payment_token_decimals":18,"price":"100000","fee_bps":250,"deadline":"2026-11-06T13:20:05Z","status":"active","created_at":"2026-11-05T13:20:05.562507656Z","updated_at":"2026-11-05T13:20:05.562507656Z"},"success":true}`;

test("live /nft/collections payload satisfies nftCollectionListValidator", () => {
  const collections = parse(nftCollectionListValidator, SERVER_NFT_COLLECTIONS, "nftCollections");
  assert.equal(collections.length, 1);
  assert.equal(collections[0].key, "tiger-art");
  assert.equal(collections[0].total_supply, 1);
  assert.equal(collections[0].status, "active");
});

test("live /nft/config payload satisfies nftMarketplaceConfigValidator", () => {
  const config = parse(nftMarketplaceConfigValidator, SERVER_NFT_CONFIG, "nftConfig");
  assert.equal(config.fee_bps, 250);
  assert.equal(config.max_fee_bps, 500);
  assert.equal(config.escrow_account, "__nft_escrow__");
});

test("live /nft/listings payload satisfies nftListingViewListValidator", () => {
  const listings = parse(nftListingViewListValidator, SERVER_NFT_LISTINGS, "nftListings");
  assert.equal(listings.length, 1);
  assert.equal(listings[0].listing.price, "100000");
  assert.equal(listings[0].listing.payment_token_symbol, "RUSD");
  assert.equal(listings[0].token.owner_user_id, "__nft_escrow__");
  assert.equal(listings[0].collection.key, "tiger-art");
  assert.equal(listings[0].active, true);
});

test("live /nft/tokens/mine payload satisfies nftTokenListValidator", () => {
  const tokens = parse(nftTokenListValidator, SERVER_NFT_TOKENS_MINE, "nftTokens");
  assert.equal(tokens.length, 1);
  assert.equal(tokens[0].token_id, 1);
  assert.equal(tokens[0].owner_user_id, "seller-1");
});

test("live /nft/listings/:id/buy payload satisfies nftListingEnvelopeValidator", () => {
  const listing = parse(nftListingEnvelopeValidator, SERVER_NFT_BUY, "nftBuy");
  assert.equal(listing.price, "100000");
  assert.equal(listing.fee_bps, 250);
});

const SERVER_NFT_OFFER = `{"offer":{"id":"8e3feacd-9abe-4cef-84b0-e99a19267a22","collection_id":"832d78d4-2ee6-4f68-92f9-600fce6932c9","token_id":2,"buyer_user_id":"buyer-2","payment_token_address":"0x4444444444444444444444444444444444444444","payment_token_symbol":"RUSD","payment_token_decimals":18,"amount":"50000","fee_bps":250,"deadline":"2026-11-06T13:28:26Z","status":"active","created_at":"2026-11-05T13:28:26.715325729Z","updated_at":"2026-11-05T13:28:26.715325729Z"},"success":true}`;

const SERVER_NFT_OFFERS = `{"offers":[{"offer":{"id":"8e3feacd-9abe-4cef-84b0-e99a19267a22","collection_id":"832d78d4-2ee6-4f68-92f9-600fce6932c9","token_id":2,"buyer_user_id":"buyer-2","payment_token_address":"0x4444444444444444444444444444444444444444","payment_token_symbol":"RUSD","payment_token_decimals":18,"amount":"50000","fee_bps":250,"deadline":"2026-11-06T13:28:26Z","status":"active","created_at":"2026-11-05T13:28:26.715325729Z","updated_at":"2026-11-05T13:28:26.715325729Z"},"token":{"id":"b0c8cee3-9b3c-4533-ba54-af7dc2bcc738","collection_id":"832d78d4-2ee6-4f68-92f9-600fce6932c9","token_id":2,"owner_user_id":"seller-2","uri":"token.json","minted_by":"admin-1","created_at":"2026-11-05T13:28:26.714751739Z","updated_at":"2026-11-05T13:28:26.714751739Z"},"collection":{"id":"832d78d4-2ee6-4f68-92f9-600fce6932c9","key":"tiger-art","name":"Tiger Art","symbol":"TART","chain_id":1,"contract_address":"0x5555555555555555555555555555555555555555","base_uri":"https://cdn.tigerswap.example/nft/","total_supply":2,"status":"active","created_by":"admin-1","created_at":"2026-11-05T13:28:26.711929419Z","updated_at":"2026-11-05T13:28:26.714821839Z"},"active":true}],"success":true}`;

test("live /nft/offers payload satisfies nftOfferViewListValidator", () => {
  const offers = parse(nftOfferViewListValidator, SERVER_NFT_OFFERS, "nftOffers");
  assert.equal(offers.length, 1);
  assert.equal(offers[0].offer.amount, "50000");
  assert.equal(offers[0].offer.buyer_user_id, "buyer-2");
  // The offer view joins the token and collection, so a client can render a
  // card without a second round trip.
  assert.equal(offers[0].token.token_id, 2);
  assert.equal(offers[0].token.owner_user_id, "seller-2");
  assert.equal(offers[0].collection.key, "tiger-art");
});

test("live POST /nft/offers payload satisfies nftOfferEnvelopeValidator", () => {
  const offer = parse(nftOfferEnvelopeValidator, SERVER_NFT_OFFER, "nftOffer");
  assert.equal(offer.amount, "50000");
  assert.equal(offer.fee_bps, 250);
  assert.equal(offer.status, "active");
});

// ------------------------------------------------ analytics (Go handler capture)
//
// Captured verbatim from `TestAnalyticsResponseWireFormat` in
// backend/services/analytics_service_test.go, which seeds real swaps and
// positions and logs each handler's response.

const SERVER_ANALYTICS_OVERVIEW = `{"total_tvl_usd":0,"volume_24h_usd":1500,"fees_24h_usd":4.5,"volume_7d_usd":1500,"fees_7d_usd":4.5,"total_pools":0,"active_pools":0,"total_farms":0,"active_farms":0,"total_tokens":0,"total_chains":0,"active_chains":0,"total_users":0,"total_trades_24h":2,"open_interest_usd":2500,"margin_borrowed_usd":0,"nft_listings_active":0,"updated_at":"2026-11-08T03:30:30.928888113Z"}`;

const SERVER_ANALYTICS_VOLUME = `{"chain_id":0,"interval":60,"pair":"TGR/RUSD","series":[{"timestamp":"2026-11-08T03:25:00Z","volume_usd":1000,"fees_usd":3,"trade_count":1,"taker_buy_volume_usd":1000,"taker_sell_volume_usd":0}]}`;

const SERVER_ANALYTICS_PAIRS = `{"pairs":[{"pair":"TGR/RUSD","chain_id":1,"volume_usd":1000,"fees_usd":3,"trade_count":1,"taker_buy_volume_usd":1000,"taker_sell_volume_usd":0},{"pair":"ETH/RUSD","chain_id":1,"volume_usd":500,"fees_usd":1.5,"trade_count":1,"taker_buy_volume_usd":0,"taker_sell_volume_usd":500}],"window":3600}`;

const SERVER_ANALYTICS_TRADERS = `{"traders":[{"user_id":"u1","realized_pnl":120,"volume_usd":2500,"closed_trades":1,"win_rate":1,"liquidations":0}]}`;

const SERVER_ANALYTICS_ROLLUP = `{"buckets_written":2,"from":"2026-11-08T02:31:00Z","to":"2026-11-08T03:31:00Z"}`;

test("live /analytics/overview payload satisfies protocolOverviewValidator", () => {
  const overview = parse(protocolOverviewValidator, SERVER_ANALYTICS_OVERVIEW, "overview");
  assert.equal(overview.volume_24h_usd, 1500);
  assert.equal(overview.fees_24h_usd, 4.5);
  assert.equal(overview.total_trades_24h, 2);
  assert.equal(overview.open_interest_usd, 2500);
  // A value the protocol cannot compute is reported as zero, never invented.
  assert.equal(overview.margin_borrowed_usd, 0);
});

test("live /analytics/volume payload satisfies volumeSeriesValidator", () => {
  const series = parse(volumeSeriesValidator, SERVER_ANALYTICS_VOLUME, "volume");
  assert.equal(series.pair, "TGR/RUSD");
  assert.equal(series.interval, 60);
  assert.equal(series.series.length, 1);
  assert.equal(series.series[0].volume_usd, 1000);
  assert.equal(series.series[0].trade_count, 1);
  assert.equal(series.series[0].taker_buy_volume_usd, 1000);
});

test("live /analytics/pairs payload satisfies pairAnalyticsListValidator", () => {
  const pairs = parse(pairAnalyticsListValidator, SERVER_ANALYTICS_PAIRS, "pairs");
  assert.equal(pairs.length, 2);
  assert.equal(pairs[0].pair, "TGR/RUSD");
  assert.equal(pairs[1].pair, "ETH/RUSD");
  // The series carries the two sides so the UI can draw buy/sell pressure.
  assert.equal(pairs[1].taker_sell_volume_usd, 500);
});

test("live /analytics/traders payload satisfies topTraderListValidator", () => {
  const traders = parse(topTraderListValidator, SERVER_ANALYTICS_TRADERS, "traders");
  assert.equal(traders.length, 1);
  assert.equal(traders[0].user_id, "u1");
  assert.equal(traders[0].realized_pnl, 120);
  assert.equal(traders[0].win_rate, 1);
  assert.equal(traders[0].liquidations, 0);
});

test("live /admin/analytics/rollup payload satisfies its envelope", () => {
  const result = parse(analyticsRollupValidator, SERVER_ANALYTICS_ROLLUP, "rollup");
  assert.equal(result.buckets_written, 2);
  assert.equal(result.from, "2026-11-08T02:31:00Z");
});

test("live /user/portfolio payload satisfies portfolioValidator", () => {
  const portfolio = parse(portfolioValidator, SERVER_PORTFOLIO, "portfolio");
  assert.equal(portfolio.totalValueUSD, 4000);
  assert.equal(portfolio.assets.length, 1);
  assert.equal(portfolio.assets[0].symbol, "ETH");
  assert.equal(portfolio.assets[0].valueUSD, 4000);
  assert.equal(portfolio.realizedPnlUsd, 0);
  assert.equal(portfolio.openPositions, 0);
});

test("live /user/transactions payload satisfies userTransactionListValidator", () => {
  const txs = parse(userTransactionListValidator, SERVER_TRANSACTIONS, "transactions");
  assert.equal(txs.length, 1);
  assert.equal(txs[0].hash, "0xdeadbeef");
  assert.equal(txs[0].chainId, 1);
  assert.equal(txs[0].tokenSymbol, "ETH");
  assert.equal(txs[0].status, "confirmed");
});
