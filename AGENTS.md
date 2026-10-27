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
