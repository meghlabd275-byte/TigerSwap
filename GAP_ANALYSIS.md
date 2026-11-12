# Gap analysis

This file records what is working, what is missing, and what is a genuine gap in
TigerSwap as a launch candidate. It is referenced from `README.md`. It describes
the repository as it stands, not an aspiration.

"Working" means the path is exercised by the test suite against a real database
or a real chain client, and it fails closed (an explicit error) rather than
falling back to mock data. "Gap" means a launch-blocking or correctness issue.

## What is working

### Contracts (`contracts/`)

Every contract has a matching `contracts/test/<Name>.t.sol` and the suite is
green under `forge test`. Value-moving paths enforce the invariants in
`AGENTS.md`:

- `TigerSwapPriceOracle` is the only price source; contracts never take a price
  from a caller, so a keeper cannot assert a limit or stop condition.
- `TigerSwapFactory` supports four pool families (xyK, concentrated, stable,
  weighted) through `familyPools`, charges `swapFee` to LPs, and mints the
  protocol fee from `sqrt(k)` growth when `feeTo` is set.
- `TigerSwapRouter.addLiquidity`/`removeLiquidity` take a `family` argument and
  enforce `to == msg.sender` for the families that pull from `msg.sender`.
- `TigerSwapPerpetuals` tracks `poolShares` per account and
  `TigerSwapCopyTrading` tracks `balances` per account, so a payout never draws
  on the contract's aggregate balance.
- Signature recovery range-checks `v` and rejects high-`s` before `ecrecover`.
- ERC-20 transfers use a call-based helper that accepts a zero-length return
  (USDT) and rejects an explicit `false`.

### Backend (`backend/`)

Go API (Gin + GORM) with a Postgres store. `go build ./...`, `go vet ./...` and
`go test ./...` are green, and `backend/go/api_gateway` (a separate module) is
built and tested explicitly.

Fund handling is real and centralised:

- `services/ledger_service.go` owns every balance mutation. Credit and debit run
  inside a transaction with a `SELECT ... FOR UPDATE` row lock.
- `services/deposit_watcher.go` is the only path that creates platform credit; it
  records each credited `(chain, tx hash, log index)` in `processed_deposits` in
  the same transaction as the credit, so a replay or reorg cannot double-credit.
- `WalletService.Withdraw` debits under the row lock before signing; a caller
  with no balance is rejected with 409 and never reaches the signer.
- Derivatives (`perpetual_engine`, `perp_money`), margin (`margin_engine`),
  options (`option_service`), farms/airdrops (`farm_service`,
  `airdrop_service`), NFT and OTC settlement all move real ledger balances.
- OTC private block trades escrow the maker's side on create and settle both legs
  on take; a cancel or expiry refunds the escrow transactionally.
- Margin and perp engines resolve bare asset symbols through the chain token
  registry, so a settlement asset is never invented.

Admin-managed listings (`futures_markets`, `perps_markets`, `margin_markets`)
are the source of truth; the engines reload them without a restart, so a listing
change takes effect immediately.

### Frontend (`frontend/`)

Next.js App Router + TypeScript + Tailwind. `npx tsc --noEmit` is clean and the
runtime-validator suite (`npm test`, 94 tests) passes. Every API response is
parsed by a validator in `src/lib/types.ts` / `validation.ts` before a component
sees it, and `serverContract.test.ts` pins the validators to payloads captured
from a running server. The WebSocket market feed is validated per frame.

### Other language layers

- `backend/rust/` (security, fast, insurance, bots) — `cargo test` green; the
  bots crate is a `cdylib` linked into the Go server behind the `rust` build tag.
- `backend/cpp/` (HFT matching engine) — builds and passes `ctest`; the book is
  kept uncrossed and exposes exact-string depth.
- `backend/java/sdk/` — no third-party dependencies; `build.sh` compiles with
  `-Xlint:all -Werror` and runs the suite.
- `backend/python/` (ml, sdk) — `pytest` green with the documented `PYTHONPATH`.

### Deployment

`backend/Dockerfile`, `frontend/Dockerfile` and `docker-compose.yml` bring up the
API, the web app and Postgres. Both images build, the API migrates its schema and
answers `/health`, and the web image serves the app. `docker compose up` reaches
a healthy stack. `.env.example` documents every variable the server reads.

## What is missing

These are not defects; they are things a global launch needs that the repository
does not yet contain.

1. **No reviewed database migrations.** The API runs GORM `AutoMigrate` at
   startup. That is fine for a first boot, but a production deployment needs
   versioned, reviewable migrations with a rollback path.
2. **No third-party security audit.** The contracts and the custody/withdrawal
   paths have not been independently audited. A global DEX launch requires this
   before real funds are at risk.
3. **No production secrets management.** Keys and the wallet seed are read from
   environment variables. A production deployment should source them from a
   secrets manager / KMS, not a plain `.env`.
4. **No horizontal-scale story.** The API keeps in-process state (the WebSocket
   hub, the perp engine registry, the bots engine). Running more than one API
   replica needs a shared bus and a leader/ownership model that is not present.
5. **No on-chain deployment scripts or addresses.** The contracts compile and
   test, but the repository carries no deploy scripts and no recorded addresses
   per chain. An operator must deploy and record them before the API can dial a
   chain.
6. **No observability stack.** There are structured logs but no metrics,
   tracing or alerting wired for a live deployment.

## Gaps (correctness or launch-blocking)

1. **Copy trading does not move funds.** `services/copy_trading.go` records
   `CopyPosition` rows and computes PnL, but it never escrows or debits a
   follower's ledger balance. A follower's copied size is a number in a table,
   not a funded position. This must either be wired to the ledger/perp engine or
   the feature must be presented as read-only tracking, never as funded copy
   trading.
2. **Launchpad participation does not escrow the payment.** `Participate`
   records an `allocated_amount` on `LaunchpadAllocation` but does not debit the
   participant's ledger balance, and `ClaimTokens` does not move the purchased
   tokens. A participant is recorded as having committed funds that were never
   taken. This must be wired to the ledger (escrow on participate, release on
   claim) before the launchpad can be used for a real raise.
3. **Bridge, swap executor and fee collector have no ledger coupling.**
   `bridge_new.go`, `swap_executor.go` and `fee_collector.go` do not reference
   the ledger service. Where these paths are expected to move platform balances
   (as opposed to broadcasting a client-signed transaction), that coupling is
   missing and must be added or the paths must be documented as
   broadcast-only.
4. **The README's launch gate is not implemented.** `README.md` states that the
   root build is intended to become the primary launch gate. There is no such
   root build; the gate is the per-language CI workflow.

## How to re-verify

```
# Go
export PATH=/tmp/goroot/bin:$PATH
cd backend && GOFLAGS=-mod=mod go build ./... && go vet ./... && go test ./... -count=1
cd go/api_gateway && GOFLAGS=-mod=mod go build ./... && go test ./...

# Frontend
cd frontend && npx tsc --noEmit && npm test

# Deployment
docker compose --env-file .env up --build
curl -s http://localhost:8080/health
```

The remaining language suites are documented in `AGENTS.md`.
