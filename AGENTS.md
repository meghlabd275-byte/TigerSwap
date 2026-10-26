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
