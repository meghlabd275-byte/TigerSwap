# TigerSwap launch audit — global / commercial readiness

Scope: the repository as it stands at `main` (`1047ece`). This is a
"what is working / what is broken / what is missing" review measured against the
bar set by venues a global launch would be compared to (Hyperliquid, dYdX,
Uniswap, PancakeSwap, Multiswap): non-custodial or explicitly custodial
settlement, real order execution, multichain, compliance, ops, and an audit.

"Working" = exercised by a test suite against a real database or a real chain
client, and it fails closed rather than falling back to mock data.
"Broken" = the code path exists and is reachable but does not do the job it
advertises. "Missing" = a launch needs it and it is not here.

This document is deliberately blunt. `GAP_ANALYSIS.md` already records the four
gaps the maintainers knew about; this audit adds the ones it does not, and it
separates "tests are green" from "the feature works".

---

## 1. Executive summary

**Test suites:** all seven language layers are green.

| Layer | Result |
| --- | --- |
| Solidity (`forge test`) | 729 tests across 28 suites, 0 failed |
| Go (`backend/`) | build + vet + test green (several packages have no tests) |
| Go (`backend/go/api_gateway`, separate module) | build + test green |
| Rust (`backend/rust`) | build + test green |
| C++ (`backend/cpp`) | builds; 252 checks, 0 failures |
| Java SDK | `build.sh` (`-Xlint:all -Werror`) → 124 checks passed |
| Python (`ml`, `sdk`) | 165 passed |
| Frontend | `tsc --noEmit` clean; validator suite green |

**The headline problem:** green tests here prove that each component does what
its own test says — not that the system trades. Three of the most commercially
important paths are wired to nothing:

1. **Spot limit/stop orders never fill.** `SwapService.CreateOrder` writes a
   `models.Order` row with `status:"pending"` and returns. Nothing advances it to
   `filled`. The only matcher in the repo (`backend/go/order_matching`, a CLOB
   with HTTP/WS) is referenced by **zero** non-test Go files — it is not wired
   into `cmd/api`, not in `docker-compose`, not in CI.
2. **TWAMM never executes.** `TwammService.ExecutableSlice` exists and is
   documented as the projection the engine runs on, but it has **no callers** and
   there is no background loop. A TWAMM order sits in the table forever.
3. **Copy trading and launchpad do not move money.** Both record rows
   (`CopyPosition`, `LaunchpadAllocation`) without debiting the follower's or
   participant's ledger balance. This is already in `GAP_ANALYSIS.md` but it is
   restated here because it is a funds-correctness bug, not a missing feature.

**The second problem: the docs describe a product that is not in the repo.**
`TigerSwap_DEX_MODE_ZERO_DEPENDENCY_FULL.md` claims a self-built MPC wallet with
account abstraction and a 24-word seed, a paymaster/gas station with gas payable
in six tokens and sponsored swaps, and an MEV-protecting private mempool. None of
that exists in the code. `README.md`'s directory listing names ~18 top-level
directories (`admin_platform`, `cex_connectors`, `cross_chain_protocol`,
`market_maker_platform`, `mobile`, `security_platform`, …) that are not present.
For a commercial launch these claims must be deleted or implemented — an auditor,
a listing partner or an exchange integrator will read them as facts.

**The third problem: settlement is fully custodial and off-chain.** Perp and
margin positions live in Postgres and settle against an internal ledger. This is
not automatically wrong — it is the exchange model, not the DEX model — but it is
the opposite of Hyperliquid (on-chain order book / vaults) and dYdX (own chain),
and it is a different trust and regulatory posture from Uniswap/PancakeSwap
(non-custodial AMM). The product is currently "a custodial CEX-shaped stack with
AMM contracts bolted on", and that mismatch needs an explicit decision before
launch, not after.

---

## 2. What is working (in detail)

### 2.1 Fund movement and the ledger

This is the strongest part of the repository and it is genuinely good.

- `services/ledger_service.go` is the single owner of balance mutation. Credit
  and debit run inside a DB transaction with a `SELECT ... FOR UPDATE` row lock,
  and the `*Within(tx, …)` helpers are used so the ledger mutation commits with
  the business write, never after it.
- `services/deposit_watcher.go` is the only path that mints platform credit. It
  records each credited `(chain, tx hash, log index)` in `processed_deposits` in
  the same transaction as the credit — replay- and reorg-safe.
- `WalletService.Withdraw` debits under the row lock **before** signing; an
  unfunded caller is rejected with 409 and never reaches the signer.
- Ledger-coupled, transactional value paths confirmed by grep:
  `perpetual_engine` (escrow margin on open, credit payout on close, liquidator
  fee + trader payout on liquidate), `margin_engine`, `option_service`,
  `farm_service`, `airdrop_service`, NFT, OTC (escrow on create, settle/refund on
  take/cancel/expiry).
- Perp settlement clamps a negative payout to 0 (`settlementPayout`) — explicit
  bad-debt handling rather than a revert that would strand the position.

### 2.2 Smart contracts

- Every contract has a matching `contracts/test/<Name>.t.sol` and the suite is
  green. Value-moving invariants from `AGENTS.md` hold in code:
  - `TigerSwapPriceOracle` is the sole price source; acting contracts read the
    price from the oracle, never from a caller argument, so a keeper cannot
    assert a limit/stop condition the contract would reject.
  - Per-account fund tracking (`TigerSwapCopyTrading.balances`,
    `TigerSwapPerpetuals.poolShares`) — no payout draws on the aggregate balance.
  - Signature recovery range-checks `v` (27/28) and rejects high-`s`; the zero
    address from a bad `ecrecover` is rejected.
  - ERC-20 transfers go through a call-based helper that accepts a zero-length
    return (USDT) and rejects an explicit `false`.
  - `TigerSwapBridge` requires a validator supermajority and the Go relayer
    recomputes the inbound digest in `bridgeInboundDigest`; both sides are pinned
    to the constant `0x9f5a44e1…` by a test on each side.
  - Four pool families (xyK, concentrated, stable, weighted) via
    `familyPools`; router liquidity calls take a `family` argument.

### 2.3 Routing

Unlike the order-book paths, spot **routing is real**: `backend/go/router` is a
genuine multi-hop, order-splitting router over TigerSwap's own pool graph, and it
is actually consumed by `services/price_aggregator.go` and `route_market.go`.
`GetQuote` and `BuildTransaction` go through it, and `BuildTransaction` returns
an unsigned tx for the client wallet to sign (the server holds no key on that
path). This is the one "aggregator/DEX" claim in the docs that holds up.

### 2.4 Frontend contract discipline

Every API response is parsed by a validator in `src/lib/types.ts` /
`validation.ts` before a component sees it, and `serverContract.test.ts` pins the
validators to payloads captured from a running server (not hand-written). The
`withDefault` marker and `marketSymbol` rules are real fixes to real bugs. The
WebSocket feed validates every frame.

### 2.5 Ops surface that is actually good

- Explicit HTTP server timeouts, an explicit CORS allowlist, `TrustedProxies`
  disabled.
- A global Redis rate limiter with an in-process fallback when Redis is
  unreachable (`middleware.RedisRateLimiter`).
- JWT `alg` is pinned to HS256; `jti` revocation and a per-user "revoke all
  sessions" cutoff are checked on every validation.
- `docker-compose.yml` + both `Dockerfile`s build; the API migrates and answers
  `/health`; the web image serves the app (verified this session).
- Nonce allocation is serialised per `(chain, sender)` so concurrent payouts
  cannot collide.

---

## 3. What is broken (reachable code that does not do its job)

### 3.1 Spot order book — orders never fill (critical)

`SwapService.CreateOrder` (`backend/services/services.go:679`) inserts
`models.Order{Status:"pending"}`. Grep for any transition to `"filled"` in
`services/services.go` returns nothing; the only matchers that set `filled` are
in `perpetual_engine.go` (perp path) and `bot_runner.go` (bot path).
`backend/go/order_matching` — a full CLOB with an HTTP/WS API and its own order
book — is imported by no production package and is absent from `docker-compose`
and CI. Net effect: **the advertised spot limit/stop-loss/take-profit order
types are inert.** A user can place an order and it will never execute or expire.

### 3.2 TWAMM never executes (critical for that feature)

`TwammService` creates, lists and cancels orders and defines `ExecutableSlice`
(`otc_service.go:757`), but nothing calls it and there is no sweeper. The
Solidity `TigerSwapTWAMM.sol` exists and is tested; the Go side is a CRUD shell.

### 3.3 Copy trading does not fund positions (funds bug)

`services/copy_trading.go` writes `CopyPosition` rows and computes PnL but never
debits a follower's ledger balance or opens a real perp position. A "copied"
size is a number in a table. Either wire it to the ledger + perp engine or label
it read-only tracking — do not ship it as funded copy trading.

### 3.4 Launchpad does not escrow (funds bug)

`Participate` records `allocated_amount` without debiting; `ClaimTokens` does not
move the purchased tokens. A raise on this launchpad would record commitments it
never collected.

### 3.5 Bridge / swap executor / fee collector have no ledger coupling

`bridge_new.go`, `swap_executor.go` and `fee_collector.go` reference the ledger
service **zero** times. `swap_executor`'s broadcast-only path is defensible (the
client signs); `bridge_new.go`'s outbound path signs with the holder's custody
key and the fee collector moves protocol revenue — those need ledger coupling or
an explicit "broadcast-only, no platform balance" statement.

### 3.6 Withdrawal never reaches a terminal state

`WalletService.Withdraw` broadcasts, records a `Transaction` with
`Status:"pending"`, and returns `status:"processing"`. There is no reconciler
that watches the receipt and flips the row to `confirmed`/`failed` or reverses
the debit on a revert. The debit is refunded only if `SendTransaction` itself
errors synchronously — a tx that is mined and reverts leaves the user debited
with a `pending` row forever.

### 3.7 Settings that are stored but never enforced

- `SystemSettings.TradingEnabled` is read/written by the admin settings handler
  (`services.go:1852-1923`) and enforced **nowhere** — swap, perp and margin do
  not consult it. The "halt trading" switch is decorative.
- `Blockchain.IsMaintenance` (and any maintenance flag) is not checked on any
  request path.

### 3.8 Non-EVM chains can be enabled but cannot be watched

`deposit_watcher.go` / `BlockchainClient` are EVM-only (`ethclient`). The chain
admin handlers can mark a non-EVM chain active, and the seeder can insert one,
but the watcher will never credit a deposit on it. Multichain is, in practice,
EVM-multichain only.

### 3.9 Precision: floats in money paths

`perp_money.go` / `perpetual_engine.go` carry `EntryPrice`, `MarkPrice`,
`UnrealizedPNL` as `float64`, and `MarginEngine.assetPriceUSD` returns `float64`.
Everything else in the repo is exact-integer decimal strings. On a venue where
PnL and liquidation thresholds are computed in `float64`, a 1-ULP rounding can
decide a liquidation. This contradicts the stated convention ("never use floats
for balances") and should be converted to `big.Rat`/`big.Int`.

---

## 4. What is missing (needed for a global commercial launch)

### 4.1 Identity, auth and the user-facing login (critical)

- **There is no user-facing login.** Protected REST endpoints require a JWT, but
  the only login form in the frontend is on the admin page
  (`frontend/src/app/admin/page.tsx`). `Header.tsx`, `page.tsx` and
  `wallet/page.tsx` have no sign-in. Components call `isAuthenticated()` and are
  gated, but there is no way for a normal user to authenticate.
- **Wallet connect is not linked to auth.** `wallet.ts` does `personal_sign` but
  there is no SIWE nonce → JWT issuance path in the backend. Wallet connect and
  session identity are two unconnected systems.
- **No refresh-cookie flow.** `frontend/src/lib/auth.ts` documents "restores a
  session from an httpOnly refresh cookie", but the backend never calls
  `SetCookie` (grep: zero hits). `AuthService.RefreshToken` just re-issues from
  the same still-valid bearer, with no rotation and no reuse detection.
- **No brute-force protection.** No login-attempt limit, lockout or password
  policy anywhere.
- **No KYC / AML / sanctions / geoblocking.** Zero references to `kyc`, `aml`,
  `sanction`, `ofac`, `geoblock` in the backend. There is no `compliance` role.
  Any venue with US or EU exposure needs sanctions screening and a geofence at
  minimum; a fiat ramp needs KYC/AML. This is the single largest *commercial*
  (as opposed to technical) blocker.

### 4.2 Custody and key management

- Keys and the wallet HD seed come from environment variables
  (`WALLET_ENCRYPTION_KEY`, `PLATFORM_SIGNER_KEY`, `WALLET_HD_SEED`). No
  KMS/HSM/secrets manager integration. Custody of user funds is a single hot
  environment-variable secret.
- The bridge relayer holds the **entire validator key set** in-process
  (`TIGERBRIDGE_VALIDATOR_KEYS`) and signs the supermajority itself. A
  supermajority that one process can produce is not a supermajority; a real
  bridge runs independent validators.
- The Rust `security`/`insurance` crates (key management, AES-256-GCM key
  storage, insurance fund with a daily spend limit) exist and are tested, but are
  **not consumed by the Go server** — the insurance fund is only touched via
  `insurance_fund_share` in `listing_service.go`. The intended secure-key layer
  is unused.

### 4.3 On-chain deployment and derivatives settlement

- **No deploy scripts and no recorded addresses** per chain. An operator must
  hand-deploy and record them before the API can dial a chain.
- **No oracle writer.** `TigerSwapPriceOracle` is a median-of-N reporter oracle,
  but no process in the repo submits prices (`setReporter`/`submitPrice` are
  never called from Go). On a clean deploy the oracle is empty and every path
  that needs a price (margin, perp mark, options) has no price source.
- **Perp/margin settlement is off-chain only.** `perpetual_engine.go` and
  `margin_engine.go` contain no reference to the on-chain `TigerSwapPerpetuals`
  / `TigerSwapLending` contracts; only `backend/go/keeper` reads the perps
  contract. So the on-chain perps/lending contracts and the trading UI are two
  different products. Decide which one is the product.
- **No liquidation/insurance backstop on the live path.** The keeper exists but
  reads its price from the orders contract's own oracle; nothing funds or draws
  the insurance fund for bad debt on the off-chain book.

### 4.4 Production operations

- **No versioned migrations.** GORM `AutoMigrate` at startup only; no reviewable
  migration history and no rollback path.
- **No observability.** Structured logs only — no metrics, no tracing, no
  alerting. There is no way to page on a stuck deposit watcher, a failed
  withdrawal, or a liquidation backlog.
- **No horizontal scale.** The WebSocket hub, perp engine registry and bots
  engine are in-process. A second API replica double-books, double-broadcasts and
  desynchronises the hub.
- **Rate limiting is global only** (100 req/min/IP), not per-endpoint and not
  per-account. Login, withdraw and order placement need their own limits.
- **No e2e/integration test** that boots the API against a chain and a DB and
  trades through the stack; no contract fork tests, no invariant tests, no fuzz
  tests (grep: zero `testFuzz`, zero `invariant_`, zero fork tests).
- **No third-party security audit.** (Already in `GAP_ANALYSIS.md`.)

### 4.5 Orphaned layers (built, tested, unused)

These are the most misleading part of the repo: they make the surface look far
larger than the product.

- **C++ HFT matching engine** (`backend/cpp`) — builds, 252 checks pass, exposes
  `Book.DepthExact`. Nothing links it into the Go server; no runtime consumer. It
  is a standalone artifact, not the venue's matcher.
- **`backend/go/order_matching`** — a second, independent CLOB; unreferenced.
- **Rust `fast` and `security` crates** — unreferenced by Go. Only `bots` is
  linked (cdylib via the `rust` build tag).
- **`backend/go/distributed/*`** (gateway, trading, cache, analytics) — separate
  binaries with no tests and no wiring into `cmd/api`; there is no distributed
  story despite the name.
- **`backend/go/admin_service`, `indexer`, `oracle_service`, `keeper`,
  `dex_connectors`, `cli`** — no test files.
- **~18 README-listed directories do not exist** (CEX connectors, mobile,
  browser extension, governance platform, security platform, …). There is no CEX
  connector, so "multiswap"/CEX-liquidity claims have no backing.

### 4.6 Multichain

The repo is EVM-only. There is no Solana/Cosmos/non-EVM client, no non-EVM
deposit watcher, and no cross-VM message layer. "Multichain" today means "the
same EVM contracts deployed on several EVM chains, once someone deploys them".

---

## 5. Doc-vs-repo discrepancies to fix before any external review

| Claim (docs) | Reality |
| --- | --- |
| MPC wallet, account abstraction, 24-word seed | Not present |
| Paymaster / gas station, gas in 6 tokens, gasless transfers, sponsored swaps | Not present |
| MEV protection: private mempool + MEV blocker | Not present |
| ~18 platform directories | Do not exist |
| "Smart router, not Uniswap's" | True — `backend/go/router` is real |
| "Not production-ready" | True — this audit agrees |

---

## 6. Prioritised remediation

**P0 — correctness of funds and orders (blockers even for a closed beta):**
1. Wire or remove copy trading and launchpad funding (ledger escrow on write).
2. Wire the spot order matcher (`order_matching` or the perp matcher) into
   `cmd/api`, or remove the limit/stop order types from the UI and API.
3. Add a withdrawal reconciler that confirms/reverses on the receipt.
4. Convert perp/margin money math from `float64` to exact integers.
5. Enforce `TradingEnabled` (and maintenance) on swap/perp/margin.

**P1 — commercial launch blockers:**
6. User-facing auth: SIWE → JWT, refresh-cookie rotation, login rate limit and
   lockout, password policy.
7. Compliance: sanctions screening, geofencing, and a KYC/AML path (or an
   explicit decision to exclude regulated jurisdictions).
8. Deploy scripts + recorded addresses per chain; an oracle reporter process.
9. Third-party audit of contracts and custody/withdrawal paths.
10. Secrets management (KMS/HSM) for `WALLET_HD_SEED`, signer and bridge keys.

**P2 — scale and ops:**
11. Versioned migrations with rollback.
12. Metrics/tracing/alerting; per-endpoint and per-account rate limits.
13. Horizontal-scale design (shared bus, leader/ownership) or an explicit
    single-replica constraint.
14. Non-EVM support, or state "EVM multichain only".
15. Fork/invariant/fuzz tests and an end-to-end trade test.
16. Delete or implement the doc claims in §5.

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
cd contracts && forge test          # 729 tests, 28 suites

# Rust / C++ / Java / Python — see AGENTS.md

# Spot-order dead end
grep -rn '"filled"' backend/services/services.go          # no hits
grep -rn "order_matching" backend --include=*.go | grep -v backend/go/order_matching   # no hits

# TWAMM dead end
grep -rn "ExecutableSlice" backend/services | grep -v _test  # definition only, no caller

# Trading gate not enforced
grep -rn "TradingEnabled" backend/services | grep -v _test   # read/written, never gated
```

---
*This audit was produced by an AI agent (OpenHands) reviewing the repository on
behalf of the user. It is an engineering review, not a formal security audit.*
