# AGENTS.md

Repository knowledge for agents working on TigerSwap (TigerEx DEX mode).

## Branching and delivery

Work is committed and pushed **directly to `main`**. This repository is
owner-operated and the owner has explicitly overridden the usual
branch-plus-pull-request default: do not open pull requests here, and do not
push to a side branch and stop. After the change is built and tested, push the
commit straight to `main` so CI (`.github/workflows/ci.yml`, triggered on push
to `main`) runs against it.

## Layout

- `backend/` — Go API server (Gin + GORM). Entry point `backend/cmd/api/main.go`.
  - `services/` — all business logic and HTTP handlers.
  - `models/models.go`, `models/derivatives.go` — GORM models.
  - `middleware/middleware.go` — JWT auth. `AuthRequired` / `AdminRequired` set
    `user_id`, `email`, `role` in the Gin context.
  - `go/` — additional Go services (amm, order_matching, indexer, keeper, ...).
  - `cpp/`, `rust/`, `python/`, `java/`, `cli/` — other language layers.
- `frontend/` — Next.js App Router + TypeScript + Tailwind.
  - `src/lib/types.ts` — runtime validators (no `any`; every API response is parsed).
  - `src/lib/endpoints.ts` — the only place API paths are declared.
  - `src/components/` — feature components.
- `contracts/` — Solidity.

## Build and test

Go (toolchain lives at `/tmp/go/bin`):

```
export PATH=/tmp/go/bin:$PATH
cd backend
go build ./... && go vet ./...
GOFLAGS=-mod=mod go test ./... -race -count=1
```

Frontend:

```
cd frontend
npx tsc --noEmit
npx next build
```

Contracts (Foundry, at `~/.foundry/bin`):

```
export PATH="$HOME/.foundry/bin:$PATH"
cd contracts
forge build
forge test
```

The other language layers each have their own toolchain. All four suites must
stay green:

```
# Rust workspace (backend/rust): security, fast, insurance
export PATH="$HOME/.cargo/bin:$PATH"
cd backend/rust && cargo test --workspace

# C++ HFT engine (backend/cpp): needs cmake and libssl-dev
cd backend/cpp && cmake -S . -B build -DCMAKE_BUILD_TYPE=Release && cmake --build build -j
./build/tigerswap_tests

# Python (backend/python): ml is stdlib-only, sdk needs web3
cd backend/python/ml && python3 -m pytest tests -q
cd backend/python/sdk && python3 -m pytest tests -q
```

`cargo`, `cmake` and `libssl-dev` are not preinstalled; install them before
running those suites. What actually works in this container:

```
export PATH="$HOME/.cargo/bin:$PATH"     # cargo, rustc
export PATH="/tmp/goroot/bin:$PATH"      # go, not on PATH by default
pip install cmake                        # lands in ~/.local/bin, so add it to PATH
sudo apt-get update && sudo apt-get install -y libssl-dev   # update first, the
                                                            # package index starts stale
cd backend/python && PYTHONPATH="ml:sdk" python3 -m pytest ml/tests sdk/tests -q
```

The Python packages are not installed into site-packages, so `PYTHONPATH` has
to point at `ml` and `sdk` or collection fails with `ModuleNotFoundError`.

The Go module uses `GOFLAGS=-mod=mod` because `go.sum` is not fully vendored.

## Conventions

- No mocks, no simulated or demo logic. Every handler must do real work or
  return an explicit error.
- Never trust the client for identity. Always read the caller from the Gin
  context via `authenticatedUserID(c)`; ignore any `user_id` in the request body.
- Validate external input at the boundary: `common.IsHexAddress` for addresses,
  `big.Int` parsing for amounts, and reject non-positive amounts.
- Monetary values are exact integers in the token's smallest unit, stored as
  decimal strings. Never use floats for balances.
- API responses are validated on the frontend. When a handler's JSON shape
  changes, update the matching validator in `src/lib/types.ts` and the
  corresponding test (e.g. `services/perp_market_view_test.go`).

## Contracts

- Every contract has a matching `contracts/test/<Name>.t.sol`. A contract with
  no test has never been executed; treat that as a defect, not a gap.
- Owner admin uses two-step transfer where it exists (`commitTransferOwnership`
  then `acceptTransferOwnership`) and single-step `transferOwnership` elsewhere.
  Either way the new owner is checked for the zero address.
- `TigerSwapPriceOracle` is the price source. Contracts that act on a price take
  it from the oracle and never from a caller-supplied argument, so a keeper or
  relayer cannot assert that a limit or stop condition was met.
- Contracts that hold user funds track them per account (`TigerSwapCopyTrading`
  `balances`, `TigerSwapPerpetuals` `poolShares`). Never pay one user out of the
  contract's aggregate balance; a shared balance lets one user's deposit settle
  another's payout.
- `TigerSwapFactory` charges `swapFee` to LPs and, when `feeTo` is set, mints
  the protocol's share of swap fees as new LP tokens at `mint`/`burn` time from
  the growth in `sqrt(k)`. While `feeTo` is unset the protocol fee is inert.
- Liquidity and accounting math rounds in the protocol's favour: fee and share
  calculations divide last, and losses are clamped to the escrowed amount.
- `forge test` builds with the IR pipeline and the optimizer, and a contract
  that compiles on its own can still fail the whole build with
  `Yul exception: Variable ... is 1 too deep in the stack`. The IR optimizer
  runs out of reachable stack slots, and it does so on a contract that is only
  one slot over. Grouping locals into a single memory struct and sharing one
  helper between similar external functions is the fix; there is no flag to
  raise the limit. When this happens, suspect the last function you touched,
  not the whole file — removing unrelated functions will not make it compile.
  Note that `solc ... test/X.t.sol` is not a valid cross-check: without the
  import remappings it fails to find `forge-std` and exits successfully with an
  error string, which reads like a pass if you only grep for the Yul message.
- Signature recovery must range-check `v` (27/28) and reject `s` above
  `0x7FFF...B20A0` before calling `ecrecover`. A high-s signature is malleable
  and recovers to a different address; the precompile also returns the zero
  address on bad input, which must be rejected rather than treated as a signer.
- ERC-20 transfers to third-party tokens go through a call-based helper that
  accepts a zero-length return (USDT returns nothing) and rejects an explicit
  `false`. A direct `IERC20.transfer` reverts against the former.

## Ledger and funds

- `services/ledger_service.go` owns all balance mutations. Credit and debit run
  inside a transaction with a `SELECT ... FOR UPDATE` row lock.
- `normalizeTokenAddress` lower-cases addresses; the empty string is stored as
  the literal `"native"` for a chain's native asset.
- `services/deposit_watcher.go` is the only path that creates platform credit.
  It scans confirmed blocks, records each credited `(chain, tx hash, log index)`
  in `processed_deposits` in the same transaction as the credit, so replays and
  reorgs cannot double-credit.
- `WalletService.Withdraw` debits under the row lock before signing. A caller
  with no balance is rejected with 409 and never reaches the signer.
- Chain selection for the watcher comes from `DEPOSIT_WATCH_CHAINS`.

## Required environment variables

Database (`DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`) is required;
there are no defaults, by design. `WALLET_ENCRYPTION_KEY` enables custody
endpoints. `PLATFORM_SIGNER_KEY` signs withdrawals. `WALLET_HD_SEED` derives
deposit addresses.

## Admin listing management

Future, perps and margin listings are admin-managed and persisted in
`futures_markets`, `perps_markets` and `margin_markets`:

- `POST/GET/PUT/DELETE /api/v1/admin/futures/markets`
- `POST/GET/PUT/DELETE /api/v1/admin/perps/markets`
- `POST/GET/PUT/DELETE /api/v1/admin/margin/markets`

New listings are created with status `paused`; an operator activates them.
`DELETE` moves a listing to `delisting` rather than removing the row.

The perp and margin engines treat these tables as the source of truth, so
listing changes take effect without a restart:

- `PerpetualEngine.SetDB` loads the active futures/perps listings into a
  registry; `RefreshMarkets` rebuilds it. `ListingService.SetMarketsChangedHook`
  is wired to that refresh in `cmd/api/main.go`, so a create, update or delete
  reloads the registry. Only `active` listings are tradable; a listing's
  `max_leverage` is authoritative and clamped to `maxPerpLeverage`.
- `MarginEngine` reads LTV, liquidation threshold, close factor, collateral and
  borrowable asset lists and the interest curve from `margin_markets` on every
  operation. An unlisted or inactive pair is rejected. Collateral and debt are
  valued from the oracle; a missing or non-positive price is an error, never a
  default.

Margin accounts live in `margin_accounts`, with `margin_collaterals` and
`margin_borrows` per account. Borrow interest accrues on read from
`last_accrual`, so it survives restarts and does not depend on a background job.

## Pool families and on-chain ABIs

Every pool family is registered in `TigerSwapFactory.familyPools[family][t0][t1]`
and read through `getPoolForFamily(tokenA, tokenB, uint8 family)`. Family codes:

- `0` xyK, `1` concentrated, `2` stable, `3` weighted.

Each family exposes different state, so a Go client must not assume one ABI fits
all pools. The canonical getters are:

- xyK: `getReserves()`, `token0()`, `token1()`
- stable: `coins(i)`, `balances(i)`, `A()` (storage), `getDy(int128,int128,uint256)`, `exchange(int128,int128,uint256,uint256)`
- weighted: `tokens(i)`, `tokenCount()`, `getBalances()`, `getWeights()`, `swap(uint256,uint256,uint256,uint256)`
- concentrated: `token0()`, `token1()`, `liquidity()`, `sqrtPriceX96()`

`TigerSwapRouter.addLiquidity`/`removeLiquidity` take a `family` argument.
Stable and weighted pools pull from and pay out to `msg.sender`, so those two
families require `to == msg.sender`; xyK supports an arbitrary `to`.

Go clients live in `backend/go/tigerswap` (library), `backend/go/dex_connectors`
and `backend/go/dex_aggregator` (binaries). They read real pool state via
`loadState`; never reintroduce a single-shape ABI for all families.

## Keepers and on-chain sourcing

`backend/go/keeper` discovers work from chain events, not seeded data:
positions come from `PositionOpened` logs on the perps contract and orders from
`OrderCreated` logs on the orders contract. Liquidation health checks mirror
`TigerSwapPerpetuals.liquidatePosition`, and order trigger/price conditions
mirror `TigerSwapAdvancedOrders._checkPriceCondition`/`_checkTriggerCondition`.
The keeper reads its price from the orders contract's own oracle
(`priceOracle` + `priceAsset`), so a keeper can never assert a condition the
contract would reject. Every executed action is a real signed transaction; a
transaction hash is never synthesized. Required env: `TIGERSWAP_PERPS_ADDRESS`,
`TIGERSWAP_ORDERS_ADDRESS`, `KEEPER_PRIVATE_KEY`, `RPC_URL`.

## Building and testing

Each language lives in its own tree, and two Go modules exist, so a plain
`go build ./...` from `backend/` does **not** compile everything:

| Tree | Build | Test |
| --- | --- | --- |
| `contracts/` | `forge build` | `forge test` |
| `backend/` | `go build ./...` | `go test ./...` |
| `backend/go/api_gateway/` | `go build ./...` | `go test ./...` |
| `backend/rust/` | `cargo build --release` | `cargo test --release` |
| `backend/cpp/` | `cmake -S . -B build && cmake --build build` | `ctest --output-on-failure` |
| `backend/java/sdk/` | `./build.sh` | `./build.sh` (runs the suite) |
| `frontend/` | `npm run build` | `npx tsc --noEmit` |

`backend/go/api_gateway` is a **separate module** (its own `go.mod`). Go commands
run in `backend/` skip it entirely. Always build and test it explicitly; a
compile error there is invisible from the parent module.

`backend/java/sdk` has **no third-party dependencies**: it uses the JDK's
`HttpClient` and a small in-tree JSON codec (`Json.java`). `build.sh` compiles
with `-Xlint:all -Werror` and then runs the suite, so any warning fails the
build. Only a JDK is needed; there is no Maven or Gradle wrapper and none should
be added.

`contracts/lib/` is gitignored, so `forge-std` is not in the repository. On a
clean checkout, clone it before running `forge test`:

```sh
mkdir -p contracts/lib
git clone --depth 1 https://github.com/foundry-rs/forge-std.git contracts/lib/forge-std
```

The Solidity contracts themselves have no external dependencies; only the test
suite imports `forge-std`.

`foundryup` installs to `~/.config/.foundry/bin`, not `~/.foundry/bin`. Prefer the
`foundry-rs/foundry-toolchain` action in CI over a manual `curl | bash` install.

There is no `go.mod` at the repository root, so `actions/setup-go` needs an
explicit `cache-dependency-path` listing `backend/go.sum` and
`backend/go/api_gateway/go.sum`, or the cache restore fails.

## Matching engine invariants

`backend/cpp/include/tigerswap/order_book.hpp` keeps the book uncrossed: the best
bid is always below the best ask. Two rules enforce it and must both hold.

- An order may only rest when its price does not cross the opposite best. A
  remainder that still crosses could only be blocked by the submitting trader's
  own resting liquidity (self-trade prevention stops at it), so it is rejected
  as `SelfTradePrevented` instead of resting. Resting it would leave a crossed
  book.
- `fillable_quantity` must count only liquidity the taker may trade against.
  Self-owned orders are excluded because `match_at_level` stops at them; counting
  them lets fill-or-kill accept an order that can never fill.

Engine API notes: the bearer token is the configured secret itself, not
`id:secret`. Pair config entries are `symbol:tick:maker_bps:taker_bps`. Orders
need a `trader` field. The engine binds `TIGERSWAP_BIND`/`TIGERSWAP_PORT` and
writes its write-ahead log to `TIGERSWAP_WAL` (default
`/var/lib/tigerswap/engine.wal`, which does not exist on a clean machine — set it
to a writable path when running locally).

## Realtime market feed

The WebSocket at `/ws` is served by `services.WebSocketHub`. Two wiring rules
matter, and breaking either leaves the endpoint silently unusable.

- `services.StartWebSocketHub` must be called before the server serves traffic.
  The hub's `Run` goroutine owns the `register` channel; while it is not running
  a connection's registration blocks and the handshake never completes. The
  register/unregister channels are buffered so a handler cannot wedge on a
  stalled hub.
- Publish through `services.Broadcast`, never by sending on `wsHub.broadcast`
  directly. `Broadcast` is non-blocking: when the queue is full the tick is
  dropped instead of stalling the caller. `StartMarketFeed` publishes real
  oracle prices on an interval; `TrackedPriceSymbols` is the one symbol list
  shared with the REST `/prices` handlers, so the two cannot drift.

The frontend consumes the feed through `src/lib/marketFeed.ts` and
`useMarketFeed`; every frame is validated with `marketTickValidator` before a
component sees it. The socket URL comes from `NEXT_PUBLIC_WS_URL`, or is derived
from the API base (`wss:` when the API base is HTTPS, same-origin when it is
relative).

## Frontend API contract validators

`frontend/src/lib/validation.ts` and `types.ts` validate every API response
before a component sees it. Two rules matter when adding a validator.

- A field wrapped in `withDefault` may be absent from the payload. The `object`
  helper detects this via the marker `withDefault` sets, so such a field does
  not need to be listed in `optional` as well. Before that marker existed the
  required-field guard fired first and every default was dead code, which
  silently dropped whole rows instead of filling in a fallback.
- Market symbols (`BTC/RUSD`) must use `marketSymbol`, not `symbol`. The
  `symbol` validator is single-token only, so using it for a derivatives or
  margin listing drops every row and the UI renders empty.

`frontend/src/lib/serverContract.test.ts` pins these validators to payloads
captured from a running server. When a listing shape changes, capture the real
body from `127.0.0.1:8080` and update the fixture, rather than hand-writing the
JSON — a hand-written fixture is what let the `total_supply` field go missing.

To check a live server end to end, build the validators with the test tsconfig
and run each real response through its validator:

```
cd frontend && npm test            # compiles to .test-build and runs the suite
```

## Farming, staking and airdrops

`services/farm_service.go` and `services/airdrop_service.go` own the vertical.
Both escrow real ledger balance, so a farm reward or an airdrop claim can only
pay out what an operator funded.

- `FarmService` is a MasterChef accumulator: `FarmReward.AccRewardPerShare` is
  scaled by `accScale` (1e18) and only advances while the farm has stake. Every
  monetary value is an exact integer string in the token's smallest unit.
  `rewardStep` is the pure projection used by `GetStakes` to display pending
  reward without mutating the stream; `advanceReward` persists it.
- Emission is capped by the stream's remaining budget (`TotalReward -
  TotalDistributed`) and stops at the farm's `end_time`. A farm with no oracle
  price reports a null APR rather than an invented one.
- The veTGR boost comes from `GovernanceService.BoostBps`, the library form of
  the `GetBoost` handler. With no boost source wired the staker gets the 1x
  base; a boost is never fabricated.
- A stake, an unstake, a claim and a reward funding each run as one database
  transaction that includes the ledger mutation (`ledger.debitWithin` /
  `ledger.creditWithin`). The farm row is re-read under `FOR UPDATE` inside the
  transaction and its status and window are re-checked, so a concurrent admin
  pause cannot be raced into accepting a stake or paying a claim. Never credit
  after the transaction commits: that is what the within-transaction helpers
  exist to avoid.
- Farm and airdrop listings are created `paused`; an operator funds the reward
  stream or campaign escrow and then activates it. `DeleteFarm` refuses while
  anything is staked; a campaign that has paid out is closed rather than
  deleted so its audit trail survives.
- Airdrop campaigns keep explicit per-user allocations in `airdrop_allocations`.
  A replacement allocation set is refused once any allocation has been claimed,
  and a claim is refused unless `Funded` still covers the remaining allocation.
- Routes: public `GET /farms`, `/farms/all`, `/farms/:id`, `/airdrops`,
  `/airdrops/all`; protected `POST /farms/:id/{stake,unstake,claim,fund}`,
  `GET /farms/stakes`, `POST /airdrops/:id/claim`, `GET /airdrops/allocations`;
  admin CRUD under `/admin/farms` and `/admin/airdrops`.

## Options

`contracts/TigerSwapOptionMath.sol` and `contracts/TigerSwapOptions.sol` are the
pricing and settlement layer; `services/option_math.go` and
`services/option_service.go` mirror them on the Go side, and the panel lives in
`frontend/src/components/OptionsTrading.tsx`.

- `option_math.go` must agree with the Solidity library digit for digit. It
  keeps the intermediate `/WAD` truncations the contract performs rather than
  computing in floating point, so a premium the API quotes is the premium the
  chain would charge.
- `invSqrt` normalises its argument into `temp` but the Newton step divides by
  the original `x`. The update is `y * (3 - x*y^2) / 2`; when `x*y^2 >= 3*WAD`
  the numerator is negative and the unsigned subtraction reverts. A short-dated
  series (under roughly three months) produces exactly that argument through
  `WAD*WAD/T`, so both libraries guard the case by halving `y` and continuing.
  If you touch the loop, keep the guard: without it any series shorter than a
  quarter could not be priced at all, and the old tests only used 0.25-1 year,
  which is why the defect survived.
- A series is fully collateralised per pool. Buying pays the Black-Scholes
  premium out of the caller's ledger balance, an LP deposits settlement
  collateral, and a settled position pays its intrinsic value. `AddCollateral`
  and `RemoveCollateral` refuse while open positions need the collateral; the
  locked amount is reserved rather than moved.
- Settlement is admin-driven: `POST /admin/options/series/:id/settle` fixes the
  settlement price, after which `Exercise` pays intrinsic value and
  `WithdrawLiquidity` returns an LP's share. A position can be exercised once;
  the payout is recorded before it is credited so a replay cannot double-pay.
- The mark price is never caller-supplied. `RefreshMarkPrice` reads it from the
  configured price source, the same way the perp engine sources its oracle.
- Routes: public `GET /options/series`, `/options/series/all`,
  `/options/series/:id`; protected `POST /options/series/:id/{buy,collateral,
  collateral/remove,exercise,withdraw}`, `GET /options/positions`; admin CRUD
  and `settle`/`refresh-price` under `/admin/options/series`.

## Analytics

`services/analytics_service.go` and `models/analytics.go` are the protocol and
market analytics layer; the panel is `frontend/src/components/Analytics.tsx` and
the admin panel is `frontend/src/components/AdminAnalyticsPanel.tsx`.

- Every figure is aggregated from rows TigerSwap itself wrote: confirmed swap
  transactions, the derivatives book, the margin book, farms, listings and the
  marketplace. Nothing calls an external indexer, and a value the server cannot
  compute (for example open interest with no price source) comes back as zero
  rather than an estimate.
- `RollupVolume` folds confirmed swaps into `volume_buckets` keyed by the unique
  `(bucket_start, pair, chain_id, bucket_seconds)` index. It rebuilds a window
  delete-then-insert inside one transaction, so re-running it after a late
  confirmation or a reorg converges on the same numbers instead of
  double-counting.
- A swap only contributes USD volume when its quote asset is one of
  `analyticsVolumeSymbols` (RUSD/RYUSD/USDT/USDC); a pair with no unambiguous USD
  notional is counted in trade count only, never valued at a guessed price. Fees
  use each pool's own fee tier, looked up once per pair, falling back to the
  protocol default only when the pool is genuinely absent.
- Routes: public `GET /analytics/{overview,volume,pairs,traders}`; admin
  `POST /admin/analytics/rollup`. Query params that are present but malformed are
  rejected with 400 rather than silently defaulted.
- The Go SDK (`backend/go/sdk/analytics.go`) and the Java SDK
  (`TigerSwapClient.getAnalytics*`) read the same endpoints. When a handler's
  JSON shape changes, update the matching validator in `frontend/src/lib/types.ts`
  and the SDK decoders together; the contract fixtures in
  `frontend/src/lib/serverContract.test.ts` are captured from a running server,
  not hand-written.

## Portfolio and transaction history

`services/portfolio_service.go` answers `GET /user/portfolio` and
`GET /user/transactions`; the panel is `frontend/src/components/Portfolio.tsx`
and the wallet page reads the same hook.

- Holdings come from the caller's own `account_balances` rows. A balance is
  `available + locked`; a zero balance is omitted. Each holding is valued at the
  oracle price, and a token the oracle cannot price is reported with a null
  `valueUSD` and excluded from `totalValueUSD`, never valued at a guessed
  number. The PnL block totals realized PnL from closed derivatives positions
  and unrealized PnL from open ones; a row whose PnL column does not parse is
  skipped rather than treated as zero. Unrealized PnL and the open-position
  count come from the live `PerpetualEngine` (attached with
  `SetPositionSource`), because the engine re-marks positions against the oracle
  every few seconds while the persisted row keeps the value it was opened with;
  without an engine attached the summary falls back to the stored rows.
- Identity is never read from the request body. Both handlers resolve the caller
  with `authenticatedUserID(c)`, so one account can never read another's rows.
- The wire format is camelCase DTOs. `/user/transactions` returns a bare JSON
  array, because the frontend's list validator rejects an object wrapper. The
  Go SDK (`backend/go/sdk/portfolio.go`) and the Java SDK
  (`TigerSwapClient.getPortfolio` / `getTransactions`) decode the same shapes.
- Contract fixtures live in `frontend/src/lib/serverContract.test.ts` and are
  captured from the handlers by `TestPortfolioWireFormat` in
  `backend/services/portfolio_service_test.go` (run with `-v` to regenerate).
