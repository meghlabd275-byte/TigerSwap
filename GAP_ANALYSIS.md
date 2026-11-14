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
runtime-validator suite (`npm test`, 137 tests) passes. Every API response is
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
   environment variables or a mounted secret file (`config.Secret`, via the
   `_FILE` sibling), but not from a secrets manager / KMS. Sourcing them from a
   managed vault rather than a plain `.env` or a mounted file is still open.
4. **No horizontal-scale story.** The API keeps in-process state (the WebSocket
   hub, the perp engine registry, the bots engine). Running more than one API
   replica needs a shared bus and a leader/ownership model that is not present.
5. **No committed on-chain address registry.** `contracts/script/Deploy.s.sol`
   writes `contracts/deployments/<chainId>.json`, but that directory is not
   present on a clean checkout. An operator must deploy and record addresses
   before the API can dial a chain.
6. **No observability stack beyond metrics.** Metrics and liveness/readiness
   probes exist, but there is no distributed tracing and no page-on-error
   alerting wired for a live deployment.

## Gaps (correctness or launch-blocking)

1. **Perp prices are still `float64` (margin is now exact).** The perp
   settlement arithmetic is exact (`perp_money.go` uses `big.Rat`), but
   `PerpetualPosition` prices (`EntryPrice`, `MarkPrice`, `IndexPrice`,
   `LiquidationPrice`) are still `float64`, and `notionalPNL` /
   `isLiquidatable` compare a float mark price against a float liquidation
   price. `MarginEngine` is now fully exact: health, LTV, liquidation
   threshold, bonus and close factor are `*big.Rat`, so a margin account whose
   health factor is exactly 1 is neither listed nor liquidated. The perp engine
   should be converted the same way; a liquidation threshold compared against a
   floating mark price can round at the boundary.
2. **Bridge relayer holds the entire validator key set.** `bridge_new.go` reads
   `TIGERBRIDGE_VALIDATOR_KEYS` in-process and signs the supermajority itself, so
   the supermajority is produced by one process. A production bridge needs
   independent validators.
3. **Non-EVM chains cannot be watched (fails closed).** `deposit_watcher.go` and
   `BlockchainClient` are EVM-only; `LoadChainRPCs` now refuses to register a
   chain whose `blockchains.type` is not EVM, so an admin marking a non-EVM chain
   active leaves it inert rather than dialling it with an `ethclient`. Multichain
   is EVM-multichain only.

## Previously recorded gaps now fixed on `main`

These were listed as gaps in an earlier revision and have since been closed;
they are kept here so the history is not lost.

- **Copy trading moves funds.** `copy_trading.go` now escrows on open
  (`debitWithin`) and credits on close (`creditWithin`) inside the position
  transaction.
- **Launchpad escrows.** `launchpad.go` debits the participant on `Participate`
  and releases on `ClaimTokens`, both within the allocation transaction.
- **Withdrawals reach a terminal state.** `withdrawal_reconciler.go` watches the
  receipt and confirms or reverses the debit.
- **`TradingEnabled` and maintenance mode are enforced.** `platform_gate.go`
  gates trading, withdrawals, deposits and registration.
- **Spot limit/stop orders execute on-chain** through the `onchain_orders.go`
  executor, with the condition re-verified on-chain.
- **Compliance exists and is enforced on trading**, not only withdrawal.
- **Governance bribes move funds.** `governance_service.go` debits the funder on
  `AddBribe` and credits the voter on `ClaimBribe`, both inside the transaction
  that records the mutation, with the claim capped at the remaining escrow. A
  bribe used to be recorded with an amount but never touched a balance, so a
  claim paid out of nothing.
- **veTGR locks escrow real balance.** `CreateLock` used to record a `VeLock`
  with an `Amount` but never debit the owner, so any account could mint voting
  power from a balance it did not hold; that power is spent against funded bribe
  escrow, so the free power could drain a bribe. `CreateLock` now debits the
  owner's TGR inside the transaction that inserts the lock (rejecting with 409
  when the balance is short), the lock stores the escrowed chain/token, and
  `WithdrawLock` refunds it under a `FOR UPDATE` row lock with a withdrawn
  re-check so a release is exactly-once. A lock created before the escrow
  columns existed (empty token) is never credited, so no balance is conjured.
- **A margin repayment reads its debt under the transaction.** `Repay` used to
  read the `MarginBorrow` row before opening its transaction, so two concurrent
  repayments each acted on the pre-payment debt. The row is now read inside the
  transaction under a `FOR UPDATE` lock.
- **A margin liquidation decides under the transaction.** `Liquidate` used to
  compute health, the repayable amount and the seized value from a snapshot read
  before the row lock, so two racing liquidators could each seize the same
  collateral. The accrual, the liquidatability decision and the amounts are now
  computed inside the transaction after the account row is locked.
- **A margin deposit and borrow cannot lose a write to a concurrent one.**
  `DepositCollateral` and `Borrow` used to read the collateral/principal row and
  write it back without a row lock or a unique constraint, so two requests on the
  same `(account, asset)` each wrote a total that omitted the other while both
  ledger movements stood. Both now lock the account row, take the debt row under
  a `FOR UPDATE` lock, and upsert on a composite unique index `(account_id,
  asset)` on `margin_collaterals` and `margin_borrows`.
- **A governance re-vote cannot double a weight.** `GovernanceService.Vote` could
  insert two `GaugeVote` rows for one `(user, gauge)`, and the pro-rata bribe
  split then counted that account's weight twice. A composite unique index
  `(user_id, gauge_id)` plus an upsert makes a re-vote update the row in place.

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
