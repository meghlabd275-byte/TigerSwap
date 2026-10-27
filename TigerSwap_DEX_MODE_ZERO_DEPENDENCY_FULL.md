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
- **V3 concentrated (self-built).** Own fee tiers (0.01%, 0.05%, 0.3%, 1%), own
  position manager NFT.
- **V4 hooks (self-built).** Dynamic fees, TWAMM, on-chain limit orders,
  auto-compounding and anti-MEV hooks.
- **StableSwap (self-built).** Low-slippage stable pool for
  RUSD/USDT/USDC/RYUSD. Own contract, not Curve.
- **Weighted pools (self-built).** 80/20 and 60/40 portfolio pools. Own
  contract, not Balancer.
- **Single-sided and zap (self-built).** Add one asset and let zap pair it.
- **Multi-hop routing (self-built).** Own smart router that finds and splits the
  best path.
- **TigerAggregator (self-built).** Own aggregator that quotes TigerSwap's own
  V2/V3/V4/stable/weighted pools and splits across them. Own algorithm; never
  calls another DEX.
- **MEV protection (self-built).** Own private mempool and MEV blocker.
- **Gasless swaps (self-built).** Sign a message, own solvers fill, gas paid in
  the output token.
- **Limit orders (self-built).** Off-chain book with on-chain settlement.
- **DCA / TWAP / TWAMM (self-built).** Own scheduler.
- **OTC and bridge+swap (self-built).** Private large-block OTC; one-click
  cross-chain via TigerBridge's own lock/mint relayer.

### 3. Liquidity

- **Add liquidity (self-built).** V2/V3/stable/weighted, own LP token.
- **Zap in/out (self-built).**
- **LP analytics (self-built).** APR, APY, impermanent-loss calculator, fees,
  volume and TVL per pool from an own indexer.
- **Lock and migration (self-built).** Own locker contract; one-click V2 to V3
  migration.

### 4. Farm, yield and staking

- **Farm V2/V3 (self-built).** Own MasterChef contract.
- **Stable farm (self-built).** Low-impermanent-loss stable LP farms.
- **Single staking and multi-reward farms (self-built).** Own syrup pool.
- **Auto-compounding vaults (self-built).** Own vault contracts and strategies.
- **veToken model (self-built).** Lock TGR for veTGR, vote gauges, earn bribes,
  boost farm APR. Own contract, not Curve or Velodrome.
- **Bribes market and gauge voting (self-built).** Own bribe marketplace.
- **Staking (self-built).** TGR, RUSD and RYUSD staking; LP staking; veTGR
  staking that earns protocol fees; liquid staking token usable in DeFi while
  staked.

### 5. Perps and futures

- **Perps V2, GMX-style (self-built).** Own oracle pricing, own liquidity pool,
  leverage, collateral in RUSD/USDT/USDC/RYUSD/BTC/ETH/TGR.
- **Perps order book (self-built).** On-chain CLOB with limit, market, stop,
  TP/SL, trailing, TWAP, post-only and reduce-only orders; funding every 8h;
  maker 0% / taker 0.02%; own matching engine.
- **Perps aggregator (self-built).** Aggregates TigerSwap's own GMX-style and
  order-book venues.
- **Funding rate, copy perps and bots (self-built).** Funding arbitrage
  dashboard, copy trading and grid/DCA/TWAP bots.

### 6. Options and margin

- **Options AMM (self-built).** Own Black-Scholes pricing and own options
  contracts. Not Lyra, Premia or Dopex.
- **Structured products (self-built).** Covered-call and put-selling vaults.
- **Spot margin (self-built).** Borrow RUSD/USDT/USDC against
  BTC/ETH/TGR collateral, 2x–10x, isolated and cross, own lending pools, own
  interest model and own liquidation.
- **Perps margin, lending pools, isolated lending and flash loans
  (self-built).** Own contracts, not Aave, Compound, MarginFi or Drift.

### 7. Launchpad and NFT

- **IDO launchpad (self-built).** Own launchpad contracts.
- **Meme launchpad (self-built).** Own bonding curve that seeds a TigerSwap pool
  after the curve completes.
- **Fair-launch LBP (self-built).** Own liquidity-bootstrapping pool.
- **NFT marketplace (self-built).** Own marketplace contracts.

### 8. Bots

- **Market-maker bot (self-built).** Auto-provide liquidity, auto-rebalance
  ranges, auto-compound fees. Own bot; not Arrakis, Gamma, Charm, Gelato or
  Panoptic.
- **Grid, DCA, arbitrage, sniping, copy, TWAP, rebalancing, yield and
  liquidation bots (self-built).** The arbitrage bot trades TigerSwap's own
  pools only; the sniping bot uses TigerSwap's own mempool monitor.
- **CEX bot parity (self-built).** Spot grid, futures grid, DCA, martingale,
  infinity grid, smart rebalance and arbitrage, all available in DEX mode
  against TigerSwap liquidity.

### 9. Analytics and social

- **DEX analytics (self-built).** Own indexer, not DexScreener or Birdeye.
- **Portfolio tracker (self-built).** Not Debank or Zerion.
- **Top traders and leaderboard (self-built).**
- **Social trading (self-built).** Follow, copy and chat.
- **On-chain signals (self-built).** Smart-money and whale alerts, new listings.

## B. Admin features

### 1. Listing, pool and farm admin

- **Token listing.** Chain, contract address, symbol, decimals, logo, price feed,
  tags, verification, spam flag; approve or reject user requests. Own database.
- **Pool creation.** V2/V3/stable/weighted, fee tiers, initial price and treasury
  liquidity seed. Own factory contracts.
- **Pool management.** Change fee tier, pause, emergency withdraw, blacklist a
  token, set max slippage.
- **Farm creation and rewards.** Create farms, set reward rates, duration and
  boost, adjust emissions, manage gauge weights and bribes. Own MasterChef.

### 2. Futures and perps listing admin

- **Futures market listing.** Own database. Symbol set includes RUSD, USDT and
  USDC quotes. Contract types RUSD_M, USDT_M, USDC_M, RYUSD_M, COIN_M and TGR_M.
  Base covers crypto, stocks, forex, commodities and indices. Configurable tick
  size, max leverage, funding rate and interval, oracle set, risk parameters
  (maintenance margin, initial margin, liquidation fee, insurance-fund
  allocation), status and listing type.
- **Perps listing.** Both GMX-style oracle perps and order-book perps, with own
  order books, own settlement and own matching.
- **Funding, mark price, insurance fund and liquidation management.** Manual
  funding override, mark-price source selection, deviation monitoring, insurance
  fund allocation and liquidation bot whitelisting.

### 3. Margin listing admin

- **Spot margin listing.** Own database. Margin mode (cross, isolated), max
  leverage, collateral list, borrowable assets, interest model (base rate, kink,
  jump multiplier, variable and stable), LTV, liquidation threshold, liquidation
  bonus, risk tier and status.
- **Isolated lending pools.** Create pools with their own collateral, LTV,
  threshold, bonus and interest model.
- **Borrow/lend, interest-rate and liquidation-engine management.** Health-factor
  monitoring, manual rate adjustment, liquidator whitelisting, close factor and
  supply/borrow caps.

### 4. Fees, revenue, treasury, buyback and veTGR admin

- **Fee management.** Swap fees per tier, perps maker/taker/open/close/borrow and
  funding, margin borrow interest.
- **Revenue split, treasury, buyback and veTGR distribution.** Configurable
  split, multi-chain treasury balances, buyback-and-burn, weekly fee
  distribution to veTGR holders. Own contracts.

### 5. Risk, security and emergency admin

- **Pause/unpause, blacklist/whitelist, oracle, price impact, MEV, insurance and
  audit logs.** Emergency pause across swaps, perps, margin and individual
  pools; token/wallet/pool blacklisting; private-pool whitelisting; oracle
  management with fallback and deviation monitoring; max price impact; default
  and maximum slippage; MEV protection settings; insurance fund monitoring and
  auto-refill; immutable audit log of every admin action.

### 6. User, liquidity, farm, bridge and launchpad admin

- **User management.** View users by wallet, chain, balances and positions; ban
  and unban.
- **Liquidity management.** View all pools by TVL, volume, fees and APR; seed
  new pools from the treasury.
- **Farm, bribe and airdrop management.**
- **Bridge and cross-chain management.** Manage the TigerBridge relayer, add
  chains, set fees, monitor liquidity.
- **Launchpad and meme management.** Approve or reject launches, set bonding
  curve parameters and fee split, auto-add liquidity after the curve.

### 7. Analytics and dashboard admin

- **DEX dashboard.** Total TVL, 24h volume, 24h fees, active users, pool and farm
  counts, perps open interest and margin borrowed, in real time from an own
  indexer.
- **Pool, perps, margin and revenue analytics.** Per-pool TVL, volume, fees, APR
  and impermanent loss; perps volume, open interest, funding, liquidations and
  top-trader PnL; margin borrow, utilization, health factor and liquidation
  history; fee breakdown by source and revenue split.

## Verification

The stack is verified green:

- Contracts: `forge build` succeeds and `forge test` passes 575 tests.
- Go: `gofmt`, `go build`, `go vet` clean; 91 tests pass.
- Rust: `cargo build --workspace` succeeds.
- Frontend: `tsc --noEmit` reports no type errors.
