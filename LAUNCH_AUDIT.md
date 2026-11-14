# TigerSwap launch audit — global / commercial readiness

Scope: the repository as it stands at `main`. This is a
"what is working / what is broken / what is missing" review measured against the
bar set by venues a global launch would be compared to (Hyperliquid, dYdX,
Uniswap, PancakeSwap, Anyswap/Multiswap): settlement model, real order
execution, multichain, compliance, operations, and an audit.

"Working" = exercised by a test suite against a real database or a real chain
client, and it fails closed rather than falling back to mock data.
"Broken" = the code path exists and is reachable but does not do the job it
advertises. "Missing" = a launch needs it and it is not here.

An earlier revision of this file reviewed `1047ece`, then `dd4d1b7`. Every
finding below was re-checked against the current `main` by reading the code and
re-running all eight language suites; the state described here is the current
one, not a snapshot pinned to an old commit. Each earlier finding is marked
resolved or still open.

---

## 1. Executive summary

**Test suites:** all seven language layers are green (see §7 for commands).

| Layer | Result |
| --- | --- |
| Solidity (`forge test`) | green (34 suites, 812 tests) |
| Go (`backend/`) | build + vet + test green (several packages have no tests) |
| Go (`backend/go/api_gateway`, separate module) | build + test green |
| Rust (`backend/rust`) | build + test green |
| C++ (`backend/cpp`) | builds; test binary passes |
| Java SDK | `build.sh` (`-Xlint:all -Werror`) green |
| Python (`ml`, `sdk`) | green |
| Frontend | `tsc --noEmit` clean; validator suite green |

**The headline problem is unchanged in kind, smaller in size.** Green tests
prove each component does what its own test says, not that the system trades. The
largest remaining issues are operational and commercial, not "the feature is
wired to nothing":

0. **The derivatives ledger is now free of the read-before-lock races and the
   open-interest caps bind.** Defects found across this and earlier reviews are
   fixed on `main`, each with a regression test that fails on the pre-fix code:
   `Repay` reads its debt row inside the transaction under a `FOR UPDATE` lock;
   `Liquidate` computes health, the repayable amount and the seized value inside
   the transaction after locking the account; `AddBribe`/`ClaimBribe` move real
   ledger funds; the oracle prices the derivatives money paths parse are parsed
   exactly rather than through `float64`; the off-chain perp engine evaluates its
   open-interest cap under the same write lock that inserts the position, so two
   concurrent opens can no longer both slip past it; and the on-chain
   `TigerSwapPerpOrderBook` now tracks and enforces `maxOpenInterest`, which was
   previously stored but never checked (§3.2).
1. **Settlement of the derivatives book is custodial and off-chain.** Perp and
   margin positions live in Postgres and settle against an internal ledger. That
   is the exchange model, not the DEX model, and it is the opposite of
   Hyperliquid (on-chain book) and Uniswap (non-custodial AMM). This is a
   product decision that must be made explicit before launch, not a bug.
2. **No third-party security audit** of the contracts or the custody/withdrawal
   paths.
3. **No secrets management.** Custody keys and the HD seed are environment
   variables; there is no KMS/HSM integration.

**The docs are now in step with the repo.** `README.md` lists only the trees that
exist. The aspirational directory listing and the unimplemented MPC/paymaster/MEV
claims have been removed.

---

## 2. What is working (in detail)

### 2.1 Fund movement and the ledger

The strongest part of the repository.

- `services/ledger_service.go` is the single owner of balance mutation. Credit
  and debit run inside a DB transaction with a `SELECT ... FOR UPDATE` row lock,
  and the `*Within(tx, …)` helpers commit the ledger mutation with the business
  write, never after it.
- `services/deposit_watcher.go` is the only path that mints platform credit. It
  records each credited `(chain, tx hash, log index)` in `processed_deposits` in
  the same transaction as the credit — replay- and reorg-safe.
- `WalletService.Withdraw` debits under the row lock **before** signing; an
  unfunded caller is rejected with 409 and never reaches the signer.
- `services/withdrawal_reconciler.go` watches the receipt and flips the row to
  `confirmed`/`failed`, reversing the debit on a revert. A broadcast that is
  mined and reverts no longer leaves the user debited with a `pending` row.
- Ledger-coupled, transactional value paths confirmed by grep: `perpetual_engine`
  (escrow margin on open, credit payout on close, liquidator fee + trader payout
  on liquidate), `margin_engine`, `option_service`, `farm_service`,
  `airdrop_service`, `copy_trading` (escrow on open, credit on close), `launchpad`
  (escrow on participate, release on claim), NFT, OTC (escrow on create,
  settle/refund on take/cancel/expiry).
- Perp settlement clamps a negative payout to 0 (`settlementPayout`) — explicit
  bad-debt handling rather than a revert that would strand the position.

### 2.2 Spot orders and routing

- Spot limit/stop/take-profit orders execute **on-chain**. The off-chain path is
  not a live matcher: `OnChainOrdersService.Start` (`onchain_orders.go`) runs an
  executor that builds `BuildExecuteOrder` / `BuildTriggerOrder` calldata and
  submits it; the condition is re-verified on-chain against the price oracle, so
  the server can never assert a condition the contract would reject. Requires
  `TIGERSWAP_ORDERS_EXECUTOR_KEY`; without it the loop does not run.
- Spot routing is real: `backend/go/router` is a multi-hop, order-splitting
  router over TigerSwap's own pool graph, consumed by
  `services/price_aggregator.go` and `route_market.go`. `BuildTransaction`
  returns an unsigned tx for the client wallet to sign; the server holds no key
  on that path.
- TWAMM slice execution is on-chain (`TigerSwapTWAMM.executeVirtualOrders`); the
  API exposes calldata endpoints. There is no off-chain slice executor, which is
  correct for a permissionless `executeVirtualOrders` model.

### 2.3 Smart contracts

- Every contract has a matching `contracts/test/<Name>.t.sol` (34 suites) and
  the suite is green.
- `TigerSwapPriceOracle` is the sole price source; acting contracts read the
  price from the oracle, never from a caller argument.
- Per-account fund tracking (`TigerSwapCopyTrading.balances`,
  `TigerSwapPerpetuals.poolShares`) — no payout draws on the aggregate balance.
- Signature recovery range-checks `v` (27/28) and rejects high-`s`; the zero
  address from a bad `ecrecover` is rejected.
- ERC-20 transfers go through a call-based helper that accepts a zero-length
  return (USDT) and rejects an explicit `false`.
- `TigerSwapBridge` requires a validator supermajority; the Go relayer recomputes
  the inbound digest in `bridgeInboundDigest`, and both sides are pinned to the
  constant `0x9f5a44e1…` by a test on each side.
- Four pool families (xyK, concentrated, stable, weighted) via `familyPools`;
  router liquidity calls take a `family` argument.
- A deploy script exists (`contracts/script/Deploy.s.sol`) and writes a per-chain
  registry file (`contracts/deployments/<chainId>.json`).

### 2.4 Identity, auth and compliance

- **User-facing sign-in exists.** `Header.tsx` renders `AuthPanel`; the SIWE path
  (`wallet_auth.go`) issues a JWT after a nonce → signature check, and the nonce
  is a single-use replay guard.
- **Rotating refresh sessions.** `refresh_service.go` rotates on every use and
  burns the whole family on reuse detection (returns 401 "session reuse
  detected").
- **Brute-force protection.** `models.User.FailedLoginAttempts` / `LockedUntil`
  with a per-account lockout, plus a per-IP `AuthRateLimiter` on the credential
  endpoints.
- **Two-factor auth.** TOTP enrolment and a `TOTPRequired` gate on withdrawal.
- **Compliance.** KYC submission/approval, a sanctions denylist (address and
  country) and geo-restriction. Withdrawals screen the destination; the mutating
  trading paths run both `RequireVerifiedTrading` and `RequireAllowedJurisdiction`
  (`compliance_service.go`), so a verified account in a restricted jurisdiction
  is refused, and the check fails closed on a store error.
- **Withdrawal policy.** Allow-listed destinations, per-asset limits and an
  operator approval queue (`withdrawal_service.go`).

### 2.5 Ops surface that is actually good

- Explicit HTTP server timeouts, an explicit CORS allowlist, `TrustedProxies`
  disabled, and a strict security-header set (CSP, HSTS, `Cache-Control: no-store`).
- Rate limiting: a global per-IP limiter with an in-process fallback when Redis
  is unreachable, a tighter per-IP limiter on credential endpoints, and a
  per-account limiter (`AccountRateLimiter`) on withdrawal (5/min) and the
  order/swap-creation endpoints (60/min).
- JWT `alg` is pinned to HS256; `jti` revocation and a per-user "revoke all
  sessions" cutoff are checked on every validation.
- Dependency-free Prometheus metrics and liveness/readiness probes
  (`services/metrics.go`) wired at `/healthz`, `/readyz`, `/metrics`, with
  database, Redis and blockchain-client probes registered.
- Outbound webhooks (`services/webhook_service.go`): durable at-least-once
  delivery, HMAC-signed, SSRF-checked **at connect time** — the client dials
  through `safeDialContext`, so the address validated is the address connected
  to and a DNS-rebinding host cannot reach a private or metadata endpoint —
  enqueued in the same transaction as the
  business write.
- An oracle writer (`onchain_perps_oracle.go`) pushes mark prices on-chain from
  the keeper key, so a deployed perp market is tradable rather than
  deployed-but-empty.
- `docker-compose.yml` + both `Dockerfile`s build; the API migrates and answers
  `/health`; the web image serves the app.
- Nonce allocation is serialised per `(chain, sender)` so concurrent payouts
  cannot collide.

---

## 3. What is broken (reachable code that does not do its job)

### 3.1 Precision: derivatives money paths (resolved)

`perpetual_engine.go` carries `EntryPrice`, `MarkPrice`, `IndexPrice` and
`LiquidationPrice` as `float64`, but no money decision is made on them. The
health check is exact: `isLiquidatable` calls `liquidationRequired`, which parses
the stored size and collateral into `big.Rat`, converts the entry and mark prices
with `rationalFromFloat` (shortest-decimal, so `0.7` does not become
`0.6999...`), computes PnL with `notionalPNL` in `big.Rat`, and compares
`equity` against `maintenance * notional` exactly. `liquidationPriceRat` derives
the displayed liquidation level from the same relation, so the price shown to a
trader is the level that actually settles. The liquidation fee and the trader's
remaining margin are computed with `big.Rat` (`liquidationPayout`,
`settlementPayout`) and only then scaled to base units.

`TestLiquidationRequiredIsExactAtTheBoundary` pins this: a long whose equity is
exactly at its maintenance level is not liquidated, and one tick below is. A
short mirrors it. A position whose size or collateral cannot be parsed is never
liquidated.

`MarginEngine` is likewise exact: risk ratios (LTV, liquidation threshold,
bonus, close factor) are parsed once into `*big.Rat`, `computeHealth` values
collateral and debt in `big.Rat`, and the liquidation decision compares the
health factor against exactly 1 (`TestMarginHealthIsExact`).

The floats that remain are the stored and displayed price fields. They are
serialised with `strconv.FormatFloat(..., 'f', -1, 64)`, the shortest decimal
that round-trips, so persisting and re-reading a price is lossless. Converting
those columns to integer strings is a schema migration with no change to any
liquidation decision, so it is not a launch blocker.

### 3.2 The perp order book's open-interest cap was inert (resolved)

`TigerSwapPerpOrderBook` stored `maxOpenInterest` and let the owner set it
through `createMarket`/`setMarketParams`, but `market.openInterest` was only ever
decremented — in `liquidate` — and never incremented. The field was therefore
always zero and the cap was never checked, so a listing's open-interest limit was
silently inert: any size could be opened against it. `TigerSwapPerpetuals`
already enforces its cap, so the two perps venues disagreed.

`_applyFill` now tracks open interest as the sum of absolute position sizes: it
adds the net increase when a fill grows a position (including the surviving side
of a flip), releases the difference when a fill shrinks one, and requires the
resulting book to stay within `maxOpenInterest` when one is set. `_applyFill` is
the single choke point for every position change, so the cap cannot be bypassed
by a particular taker/maker split.
`testOpenInterestTracksPositionsAndCapBinds` and
`testOpenInterestUncappedMarketIsNotBlocked` pin it; both fail on the pre-fix
contract, where open interest stayed at zero.

### 3.3 Non-EVM chains can be enabled but cannot be watched (open, fails closed)

`deposit_watcher.go` / `BlockchainClient` are EVM-only (`ethclient`). A chain
row an operator marks with a non-EVM type is now refused by
`LoadChainRPCs` rather than dialled with an `ethclient`, so a non-EVM chain is
inert rather than misconfigured. "Multichain" today means "the same EVM contracts
on several EVM chains"; there is still no non-EVM deposit path.

### 3.4 Bridge relayer holds the whole validator set (open)

`TIGERBRIDGE_VALIDATOR_KEYS` is read in-process and the relayer signs the
supermajority itself. A supermajority one process can produce is not a
supermajority; a real bridge runs independent validators. This is a design gap,
not a coding error, but it is a launch blocker for a bridge with real value.

---

## 4. What is missing (needed for a global commercial launch)

### 4.1 Custody and key management (critical)

Keys and the wallet HD seed come from environment variables
(`WALLET_ENCRYPTION_KEY`, `PLATFORM_SIGNER_KEY`, `WALLET_HD_SEED`). There is no
KMS/HSM/secrets-manager integration. Custody of user funds rests on a single hot
environment-variable secret.

### 4.2 On-chain deployment and settlement

- **No committed address registry.** `contracts/script/Deploy.s.sol` writes
  `contracts/deployments/<chainId>.json`, but `contracts/deployments/` is not
  present on a clean checkout, so an operator must deploy and record addresses
  before the API can dial a chain.
- **Two derivative products ship side by side.** The off-chain engine
  (`perpetual_engine.go`, `margin_engine.go`) settles against the platform
  ledger; the on-chain product (`services/onchain_perp_book.go` over
  `TigerSwapPerpOrderBook`, plus `services/onchain_perps.go` over
  `TigerSwapPerpetuals`) settles on chain, with its own indexer, keeper and
  oracle writer, exposed under `/api/v1/onchain/perp-book/*` and
  `/api/v1/onchain/perps/*`. Both are real and both are tested. The gap is not
  that on-chain settlement is absent — it is that an operator has to choose
  which one is the venue, because a chain with no committed deployment runs the
  off-chain book only, and the off-chain book does not accrue funding. Document
  the choice, or make the on-chain book the only perps venue.

### 4.3 Production operations

- **No versioned migrations.** GORM `AutoMigrate` at startup only; no reviewable
  migration history and no rollback path.
- **No distributed tracing or alerting.** Metrics, probes and per-request
  correlation logging (`X-Request-ID` + one structured line per request) exist;
  there is no trace and no page-on-error integration (no PagerDuty/Slack-on-error).
- **No horizontal scale.** The WebSocket hub, perp engine registry and bots
  engine are in-process. A second API replica double-books, double-broadcasts and
  desynchronises the hub. Either a shared bus + leader/ownership model or an
  explicit single-replica constraint is needed.
- **No third-party security audit.** (Also in `GAP_ANALYSIS.md`.)

### 4.4 Test depth

- Fuzz and invariant tests now exist for the xyK pool
  (`contracts/test/TigerSwapPoolFuzz.t.sol`, `TigerSwapPoolInvariant.t.sol`) and
  fuzz coverage has been added for the stable, weighted and concentrated
  families (`TigerSwapStableFuzz.t.sol`, `TigerSwapWeightedFuzz.t.sol`,
  `TigerSwapConcentratedFuzz.t.sol`). Still absent: contract fork tests (a test
  that reads a live fork).
- No end-to-end test that boots the API against a chain and a DB and trades
  through the stack. Per-service integration tests run against a real database
  and a real in-process chain harness (`perps/perpstest`, `go/orders`).

### 4.5 Orphaned layers (built, tested, not consumed by the API)

- **C++ HFT matching engine** (`backend/cpp`) — builds and tests, but nothing
  links it into the Go server. It is a standalone artifact.
- **`backend/go/order_matching`** — a second CLOB; not imported by the API (the
  live order path is the on-chain executor in `services/onchain_orders.go`).
- **Rust `fast` and `security` crates** — not referenced by Go; only `bots` is
  linked (cdylib via the `rust` build tag). The `security` crate's secure-key
  storage is unused.
- **`backend/go/distributed/*`** (gateway, trading, cache, analytics) — separate
  binaries with no tests and no wiring into `cmd/api`.
- Standalone binaries with no in-process consumer: `oracle_service`, `keeper`,
  `indexer`, `dex_aggregator`, `dex_connectors`, `admin_service`, `api_gateway`,
  `cli`. These are deployable services, not dead code, but the API does not start
  them.

### 4.6 Multichain

The repo is EVM-only. There is no Solana/Cosmos/non-EVM client, no non-EVM
deposit watcher, and no cross-VM message layer.

---

## 5. Doc-vs-repo discrepancies

| Claim (docs) | Reality |
| --- | --- |
| ~18 platform directories (CEX connectors, mobile, browser extension, …) | Removed from `README.md`; they do not exist |
| MPC wallet, account abstraction, 24-word seed | Not present (claim removed) |
| Paymaster / gas station, gas in 6 tokens, sponsored swaps | Not present (claim removed) |
| MEV protection: private mempool + MEV blocker | Not present (claim removed) |
| "Smart router, not Uniswap's" | True — `backend/go/router` is real |
| "Not production-ready" | True — this audit agrees |

---

## 6. Prioritised remediation

**P0 — correctness of funds and orders:**
1. (Resolved) The derivatives money paths run on exact `big.Rat` arithmetic;
   only the stored/displayed price fields are floats, and they round-trip
   losslessly. Pinned by `TestLiquidationRequiredIsExactAtTheBoundary` (§3.1).
2. Decide and document the settlement model: custodial off-chain book vs the
   on-chain `TigerSwapPerpetuals`/`TigerSwapLending` contracts (§4.2).
3. Commit a per-chain address registry, or document the deploy step as a
   prerequisite (§4.2).

**P1 — commercial launch blockers:**
4. Third-party audit of contracts and custody/withdrawal paths.
5. Secrets management (KMS/HSM) for `WALLET_HD_SEED`, signer and bridge keys.
6. Independent validator set for the bridge (§3.3).

**P2 — scale and ops:**
7. Versioned migrations with rollback.
8. Tracing and alerting (metrics and probes already exist).
9. Horizontal-scale design (shared bus, leader/ownership) or an explicit
   single-replica constraint.
10. Non-EVM support, or state "EVM multichain only".
11. Fork tests and an end-to-end trade test (pool-family fuzz/invariant tests
    are in place).

---

## 7. How to re-verify this audit

```
# Go
export PATH=/tmp/goroot/bin:$PATH
cd backend && GOFLAGS=-mod=mod go build ./... && go vet ./... && go test ./... -count=1
cd go/api_gateway && go build ./... && go test ./...

# Frontend
cd frontend && npx tsc --noEmit && npm test

# Contracts
export PATH="$HOME/.foundry/bin:$PATH"
cd contracts && forge test

# Rust / C++ / Java / Python — see AGENTS.md

# Spot-order path is on-chain, not a Go matcher
grep -rn "order_matching" backend/cmd backend/services --include=*.go   # no hits
grep -rn "BuildExecuteOrder\|BuildTriggerOrder" backend/services/onchain_orders.go

# Compliance is enforced on trading
grep -n "RequireAllowedJurisdiction\|RequireVerifiedTrading" backend/cmd/api/routes.go

# Withdrawal reaches a terminal state
grep -rn "StartWithdrawalReconciler" backend/cmd/api/main.go
```

---
*This audit was produced by an AI agent (OpenHands) reviewing the repository on
behalf of the user. It is an engineering review, not a formal security audit.*
