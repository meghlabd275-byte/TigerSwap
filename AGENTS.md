# AGENTS.md

Repository knowledge for agents working on TigerSwap (TigerEx DEX mode).

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

`cargo`, `cmake` and `libssl-dev` are not preinstalled; install them (rustup,
`apt-get install -y cmake libssl-dev`) before running those suites.

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
