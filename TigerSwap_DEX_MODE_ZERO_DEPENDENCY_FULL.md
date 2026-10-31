# TigerSwap — DEX Mode — Zero External Dependency

TigerSwap is TigerEx's DEX mode. It is a standalone DEX — its own AMM, its own
order book, its own perps and margin engine — not a wrapper around Uniswap,
dYdX, GMX or Aave.

## Principle

TigerSwap never depends on any other CEX or DEX. There is no Uniswap SDK, no
Pancake SDK, no 1inch API, no Jupiter API, no GMX API, no Hyperliquid API and no
Binance API anywhere in this repository. Everything below is built here: own AMM
V2/V3/V4/stable/weighted Solidity, own CLOB matching engine, own aggregator
routing, own bridge relayer, own nodes and own oracles.

## Scope reference

The feature set below was scoped against the top 30 DEXs — Uniswap V2/V3/V4,
PancakeSwap V3, Curve, Balancer, Sushi V3, TraderJoe, Orca, Raydium, Jupiter,
1inch, 0x, dYdX V4, GMX V2, Gains Network gTrade, Hyperliquid, Aster DEX, Drift,
Vertex, ApolloX, ApeX, WOOFi, QuickSwap, Velodrome, Aerodrome, Camelot, Thena,
DODO, Bancor, Kyber and Cetus — so that no user or admin capability present on
those venues is missing here.

## A. User and trader features

### 1. Wallet, chain and gas

- **Wallet connect (self-built).** MetaMask, Rabby, Phantom, Solflare, Trust,
  OKX, Bitget, TokenPocket, SafePal, Ledger, Trezor and WalletConnect V2 through
  an own connector. No Web3Modal dependency.
- **TigerWallet integration (self-built).** MPC + account abstraction + 24-word
  HD. Own wallet, not the Trust Wallet SDK.
- **Chain support (self-built).** EVM and non-EVM chains served from own
  self-hosted RPC nodes, not Alchemy or Infura.
- **Gas abstraction (self-built).** Own paymaster; gas payable in RUSD/USDT/USDC/
  TGR/RYUSD; gasless transfers; sponsored swaps; own gas station treasury.

### 2. Swap

- **V2 xyK (self-built).** Own factory and router Solidity, 0.3% fee.
- **V3 concentrated (self-built).** Concentrated ranges with 0.01%, 0.05%, 0.3%
  and 1% fee tiers; own position manager NFT. No Uniswap V3.
- **V4 hooks (self-built).** Dynamic fees, TWAMM, on-chain limit orders,
  auto-compounding and anti-MEV hooks. No Uniswap V4.
- **StableSwap (self-built).** Low-slippage pool for RUSD/USDT/USDC/RYUSD at
  0.01%. Own contract, not Curve.
- **Weighted pools (self-built).** 80/20 and 60/40 portfolio pools. Not Balancer.
- **Single-sided and zap (self-built).** Add a single asset and the pair is
  filled automatically. Not Bancor or TraderJoe.
- **Multi-hop routing (self-built).** Own smart router that finds and splits the
  best path across pools. Not the Uniswap smart router.
- **TigerAggregator (self-built).** Own aggregator that quotes TigerSwap's own
  V2/V3/V4/stable/weighted pools and splits across them for best price. No 1inch,
  no Jupiter, no 0x API, and no other DEX is ever called.
- **MEV protection (self-built).** Private mempool and MEV blocker with own
  builder. Not CowSwap.
- **Gasless swaps (self-built).** Sign a message, own solver network fills, gas
  paid in the output token. Not UniswapX or 1inch Fusion.
- **Limit orders (self-built).** Off-chain book with on-chain settlement. Not
  1inch Limit or Jupiter Limit.
- **DCA / TWAP / TWAMM (self-built).** Own scheduler for recurring buys,
  time-sliced large orders and time-weighted market making. Not Mean Finance or
  Jupiter DCA.
- **OTC and bridge+swap (self-built).** Private large-block OTC with no price
  impact; one-click cross-chain swap through own TigerBridge lock/mint relayer.
  Not Stargate or Across.

### 3. Liquidity

- **Add liquidity (self-built).** V2, V3, stable and weighted, with own LP token.
  Not Uniswap LP.
- **Zap in/out (self-built).** One-click entry and exit. Not Pancake Zap.
- **LP analytics (self-built).** APR, APY, impermanent-loss calculator, fees
  earned, volume and TVL per pool, served by own indexer. Not DexScreener API.
- **Liquidity lock and migration (self-built).** Lock LP from one week to four
  years for boosted rewards; migrate V2 to V3 in one click. Own locker contract,
  not Team Finance.

### 4. Farm, yield and staking

- **Farm V2/V3 (self-built).** Own MasterChef; stake LP to earn TGR and RYUSD.
  Not Pancake Farms.
- **Stable farm (self-built).** Low-IL farming for stable LP. Not Curve.
- **Single staking and multi-reward farm (self-built).** Own syrup pool; earn one
  or several reward tokens. Not Pancake.
- **Auto-compounding vaults (self-built).** One-click compounding with own
  strategy. Not Beefy or Yearn.
- **veToken model (self-built).** Lock TGR for veTGR, vote gauges, earn bribes,
  boost farm APR. Own ve contract. Not Curve veCRV or Velodrome veVELO.
- **Bribes market and gauge voting (self-built).** Projects bribe veTGR holders;
  holders vote on emissions each epoch. Own bribe marketplace. Not Votium or
  Hidden Hand.
- **Staking (self-built).** TGR, RUSD and RYUSD staking; LP staking; veTGR
  staking that earns a share of swap and perp fees plus voting, bribes and boost;
  liquid staking stTGR usable in DeFi while staked. Not GMX staking.

### 5. Perps and futures

- **Perps V2, GMX-style (self-built).** Oracle pricing, no borrow fee, GLP-style
  pool, 1000+ pairs across crypto, stocks, forex, commodities and indices,
  leverage up to 1001x, collateral in RUSD/USDT/USDC/RYUSD/BTC/ETH/TGR. Own
  engine, not GMX.
- **Perps order book (self-built).** On-chain CLOB with limit, market, stop,
  take-profit/stop-loss, trailing, TWAP, post-only and reduce-only orders;
  funding every 8 hours; maker 0% / taker 0.02%; own Rust matching engine. Not
  dYdX V4, Hyperliquid or Aster.
- **Perps aggregator (self-built).** Aggregates own oracle perps and own order
  book for best price and lowest fees. Depends on no other perps venue.
- **Funding rate, copy perps and perps bots (self-built).** Funding arbitrage
  dashboard across own markets, copy trading of top perps traders, and grid, DCA
  and TWAP bots. Not GMX copy.

### 6. Options and margin

- **Options AMM (self-built).** Buy calls and puts on BTC/RUSD, ETH/RUSD and
  TGR/RUSD, European and American, cash settled, with own Black-Scholes pricing.
  Not Lyra, Premia or Dopex.
- **Structured products (self-built).** Covered-call and put-selling vaults for
  BTC, ETH and TGR. Not Ribbon or Friktion.
- **Margin spot (self-built).** Borrow RUSD/USDT/USDC against BTC/ETH/TGR
  collateral; trade spot at 2x–10x; isolated and cross; own lending pools, own
  variable interest model and own liquidation. Not Aave or Compound.
- **Margin perps, lending pools, isolated lending and flash loans (self-built).**
  Cross-margin borrowing across spot, perps and lending; supply to earn interest;
  isolated pools for single-asset risk; flash loans repaid in one transaction.
  Not Aave, MarginFi or Drift.

### 7. Launchpad and NFT

- **IDO launchpad (self-built).** Stake TGR or veTGR for allocation. Own
  contracts. Not Pancake IFO.
- **Meme launchpad (self-built).** One-click launch on an own bonding curve that
  adds liquidity to TigerSwap once complete. Not Pump.fun.
- **Fair-launch LBP (self-built).** Liquidity bootstrapping pool. Not Fjord.
- **NFT marketplace (self-built).** Trade NFTs against RUSD/USDT/USDC/TGR. Own
  contracts. Not OpenSea or Blur.

### 8. Bots

- **Market maker bot (self-built).** Auto-provide liquidity to V2 and V3 pools,
  rebalance ranges when price leaves the range, auto-compound fees. Own Rust bot;
  no Arrakis, Gamma, Charm or Gelato API.
- **Grid bot (self-built).** Spot and perps grid. Not Pancake Grid or Pionex.
- **DCA bot (self-built).** Own scheduler. Not Mean Finance or Jupiter DCA.
- **Arbitrage bot (self-built).** Arbitrage across TigerSwap's own V2, V3, stable
  and weighted pools only. Not 1inch or Jupiter.
- **Sniping bot (self-built).** Own mempool monitor for new TigerSwap launches.
  Not Maestro, Banana Gun or Unibot.
- **Copy bot (self-built).** Copy top DEX traders on TigerSwap, swaps and perps.
  Own copy engine.
- **TWAP, rebalancing, yield and liquidation bots (self-built).** Time-sliced
  orders, portfolio rebalancing, auto-compounding and veTGR claiming, and
  liquidation of undercollateralized margin loans. All own Rust bots. Not
  Beefy or Yearn.
- **All CEX bots ported to DEX (self-built).** Spot grid, futures grid, DCA,
  martingale, infinity grid, smart rebalance and arbitrage, all running on
  TigerSwap liquidity.

### 9. Analytics and social

- **DEX analytics (self-built).** Prices, volume, liquidity, holders and
  transactions for all pairs from an own indexer. Not DexScreener or Birdeye.
- **Portfolio tracker (self-built).** Track wallets across chains with PnL and
  holdings per chain. Not Debank or Zerion.
- **Top traders and leaderboard (self-built).** Rank DEX traders by PnL. Not
  DexScreener Top Traders.
- **Social trading (self-built).** Follow, copy and chat. Not eToro.
- **On-chain signals (self-built).** Smart-money and whale alerts, new listings.
  Not Arkham.

## B. Admin features

### 1. Listing, pool and farm admin

- **Token listing admin (self-built).** Chain, contract address, symbol,
  decimals, logo, CoinGecko id, price feed, tags, verification badge and spam
  flag; approve or reject user requests. Own database. Not the Trust assets repo.
- **Pool creation admin (self-built).** Create V2, V3, stable and weighted pools
  with configurable fee tier, initial price and seeded liquidity from own
  treasury. Own factory contracts. Not the Uniswap factory.
- **Stable pool creation admin (self-built).** Configure the amplification
  parameter and fee for RUSD/USDT/USDC/RYUSD.
- **Concentrated and weighted pool admin (self-built).** Tick spacing, fee tier
  and weighted ratios.
- **Pool management (self-built).** Change fee tier, pause a pool, emergency
  withdraw, blacklist a token and set maximum slippage.
- **Liquidity management (self-built).** Add or remove admin seed liquidity from
  own treasury.
- **Farm creation and rewards management (self-built).** Create a farm for an LP
  token, set reward tokens per block, APR and duration, adjust TGR emissions,
  set gauge weights and manage bribes. Own MasterChef. Not Pancake.

### 2. Futures market and perps listing admin

- **Futures market listing admin (self-built).** Own `futures_markets_dex`
  table with add, edit, delete and pause.
  - Symbols across RUSD, USDT and USDC quotes, including BTC, ETH, TGR, TSLA and
    the rest of the 1000+ pair set; USDT and USDC futures contracts must exist.
  - Contract types: `RUSD_M`, `USDT_M`, `USDC_M`, `RYUSD_M`, `COIN_M`, `TGR_M`.
  - Quote assets: RUSD, USDT, USDC, RYUSD, USD, BTC, ETH, TGR.
  - Settlement in RUSD, USDT, USDC, RYUSD, BTC, ETH or TGR, by own engine.
  - Contract size, tick size, maximum leverage (default 125x, 200x for BTC/ETH,
    500x for alts, 1001x in high-leverage mode), funding rate and interval,
    oracle selection, and risk parameters (maintenance margin, initial margin,
    liquidation fee, insurance-fund allocation).
  - Status: active, paused, delisting, maintenance. Listing type: perpetual,
    quarterly, bi-quarterly, delivery.
  - TradFi perps for stocks, forex, commodities and indices, settled in
    RUSD/USDT/USDC, priced from own TradFi oracle.
- **Perps listing admin (self-built).** GMX-style oracle perps and order-book
  perps with the order types, fee schedule and own matching engine described
  above. Not GMX, dYdX V4, Hyperliquid or Aster.
- **Funding rate, mark price, insurance fund and liquidation admin
  (self-built).** Manual funding override; mark price from a median of
  Chainlink, Pyth and Tiger; pause a market when deviation exceeds 5%; allocate
  a share of fees to the insurance fund; set the liquidation fee split between
  liquidators and insurance; maintain the liquidation-bot whitelist.

### 3. Margin listing admin

- **Margin listing admin (self-built).** Own `margin_markets_dex` table.
  - Pairs such as BTC/RUSD, ETH/RUSD, TGR/RUSD and SOL/RUSD.
  - Margin mode: cross and isolated.
  - Maximum leverage, collateral list, borrowable assets, interest-rate model
    (base rate, kink, jump multiplier), LTV per collateral, liquidation
    threshold, liquidation bonus and risk tier. Status: active or paused.
- **Isolated lending pools admin (self-built).** Create isolated pools with their
  own LTV, threshold, bonus and interest model so risk does not contaminate
  other pools.
- **Borrow/lend, interest rate and liquidation engine admin (self-built).**
  Monitor positions and liquidate below a health factor of 1; adjust the
  interest-rate model; maintain the liquidation-bot whitelist; set close factor
  and per-asset supply and borrow caps.

### 4. Fees, revenue, treasury, buyback and veTGR admin

- **Fee management (self-built).** Swap fees per pool tier, perp maker/taker and
  open/close fees, borrow fee and funding, and margin borrow interest. Own fee
  engine.
- **Revenue split, treasury, buyback and veTGR distribution (self-built).**
  Configurable split of swap and perp fees between LPs, veTGR holders, treasury
  and insurance; multi-chain treasury balances; treasury-funded TGR buyback and
  burn; weekly fee distribution to veTGR holders. Own contracts.

### 5. Risk, security and emergency admin

- **Pause/unpause (self-built).** Emergency pause of swaps, perps, margin or a
  specific pool.
- **Blacklist and whitelist (self-built).** Block malicious tokens, wallets and
  pools; whitelist private pools.
- **Oracle admin (self-built).** Add oracle addresses and fallbacks; monitor
  deviation and pause a market past the threshold.
- **Price impact and slippage limits (self-built).** Maximum price impact, and
  default and maximum slippage.
- **MEV protection (self-built).** Private mempool, MEV blocker and fee share.
- **Insurance fund (self-built).** Per-chain target with automatic refill from
  fees.
- **Audit logs (self-built).** Immutable record of every admin action — who
  listed a pool, changed a fee or paused a market.

### 6. User, liquidity, farm, bridge and launchpad admin

- **User management (self-built).** View DEX users by wallet address, chain,
  balance and positions; ban or unban a wallet.
- **Liquidity management (self-built).** View TVL, volume, fees and APR per pool;
  add or remove admin seed liquidity.
- **Farm, bribe and airdrop management (self-built).** Create, edit and disable
  farms; manage gauge voting; review project bribes and veTGR claims; configure
  airdrop eligibility, amount and claim period.
- **Bridge and cross-chain management (self-built).** Manage the own TigerBridge
  relayer; add chains, set bridge fees and monitor liquidity. Not Stargate or
  Across.
- **Launchpad and meme management (self-built).** Approve or reject IDO and meme
  launches; configure bonding-curve parameters, treasury fee and burn; add
  liquidity to TigerSwap automatically once the curve completes. Not Pump.fun.

### 7. Analytics and dashboard admin

- **DEX dashboard (self-built).** Total TVL across chains, 24-hour volume, fees,
  active users, pools, farms, perps open interest and margin borrowed, from an
  own real-time indexer. Not DexScreener.
- **Pool, perps, margin and revenue analytics (self-built).** Per-pool TVL,
  volume, fees, APR and IL; perp volume, open interest, funding, liquidations and
  top-trader PnL; margin borrowed, utilization, health factor and liquidation
  history; fees by source and the LP/veTGR/treasury/insurance split.

## Summary

- **Swap.** Own V2 xyK, V3 concentrated, V4 hooks, stable, weighted, single-sided
  zap, multi-hop routing, TigerAggregator, MEV-protected private mempool, gasless
  swaps, limit orders, DCA/TWAP/TWAMM, OTC and bridge+swap. No external DEX.
- **Liquidity.** Own V2/V3/stable/weighted pools, zap, LP token, own analytics
  indexer, lock and migration.
- **Farm, staking, veTGR.** Own MasterChef variants, single and multi-reward
  farms, auto-compounding vaults, veTGR, bribes and gauges. No external protocol.
- **Perps and futures.** Own oracle perps, own order-book CLOB with a Rust
  matching engine, funding arbitrage, copy trading and bots. Futures, perps and
  USDT/USDC contracts are listed and settled by own engine. No external perps
  venue.
- **Options and margin.** Own options AMM, structured products, margin spot,
  margin perps, lending pools, isolated lending and flash loans. No external
  lending protocol.
- **Launchpad and NFT.** Own IDO, meme launchpad, LBP and NFT marketplace.
- **Bots.** Own market maker, grid, DCA, arbitrage, sniping, copy, TWAP,
  rebalancing, yield and liquidation bots. No external bot API.
- **Analytics and social.** Own DEX analytics, portfolio tracker, leaderboard,
  social trading and on-chain signals.
- **Admin.** Token listing, pool creation, farm creation, futures/perps/margin
  listing, fee and treasury management, risk and emergency controls, user and
  liquidity management, bridge and launchpad management, and analytics — all
  self-built, with no dependency on any other CEX or DEX.
