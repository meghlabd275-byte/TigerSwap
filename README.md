# TigerSwap - Multichain DEX Ecosystem

TigerSwap is an EVM-multichain exchange: a Next.js trading frontend, a Go API and
trading stack, EVM smart contracts, and SDKs. The repository is deliberately
small; it contains only the trees listed below.

> **Launch status:** not yet production-ready. `GAP_ANALYSIS.md` records the
> maintainers' known gaps and `LAUNCH_AUDIT.md` is the detailed
> working/broken/missing review. Both are kept current against `main`.

## Current engineering priorities

1. Keep every language layer green (Solidity, Go, Rust, C++, Java, Python, frontend).
2. Keep every value-moving path on the real ledger and free of mock/demo data.
3. Close the launch blockers tracked in `LAUNCH_AUDIT.md`.
4. Add contract fork/fuzz/invariant tests and an end-to-end trade test.
5. Add production operations: versioned migrations, KMS/HSM secrets, and a
   documented single-replica or shared-state scaling story.

## Technology stack

| Layer | Technology |
| --- | --- |
| Frontend | Next.js, React, TypeScript, Tailwind |
| Backend APIs | Go (Gin + GORM) |
| Smart contracts | Solidity (Foundry) |
| Routing / matching engines | Go (router, order book), C++ (HFT engine), Rust (bot strategies) |
| SDKs | Go, Java, Python |

## Repository structure

```text
TigerSwap/
├── backend/                    # Go API server and services
│   ├── cmd/api/                # API entry point and route table
│   ├── services/               # business logic and HTTP handlers
│   ├── models/                 # GORM models
│   ├── middleware/             # JWT auth, rate limiting, security headers
│   ├── config/                 # chain/token config and deployment registry
│   ├── go/                     # extra Go modules (amm, router, orders, perps,
│   │                           #   keeper, indexer, oracle_service, sdk,
│   │                           #   order_matching, api_gateway, distributed, ...)
│   ├── rust/                   # security, fast, insurance, bots crates
│   ├── cpp/                    # HFT matching engine
│   ├── java/sdk/               # dependency-free Java SDK
│   └── python/                 # ml (stdlib) and sdk (web3)
├── contracts/                  # Solidity contracts, script/ (deploy), test/
├── frontend/                   # Next.js app (App Router) and its validators
├── docker-compose.yml          # Postgres + API + web for a single host
├── AGENTS.md                   # repository knowledge for agents
├── GAP_ANALYSIS.md             # known gaps
└── LAUNCH_AUDIT.md             # detailed readiness review
```

## Quick start

```bash
cp .env.example .env     # fill in DB_*, JWT_SECRET and any signing keys
docker compose up --build
```

See `INSTALLATION.md` for per-language build and test commands.

## Deployment

The API and the web app each ship as a Docker image, and `docker-compose.yml`
brings up a complete stack (Postgres, API, web) for a single host.

```bash
cp .env.example .env     # fill in DB_*, JWT_SECRET and any signing keys
docker compose up --build
```

The API runs GORM `AutoMigrate` on startup, so the schema is created on the
first boot. For a production database, run a reviewed migration instead.

- `backend/Dockerfile` builds the API server (Go module at `backend/`). The
  default build links no cgo; the Rust strategy engine is opt-in via the `rust`
  build tag and a prebuilt `libtigerswap_bots.so` (see `AGENTS.md`).
- `frontend/Dockerfile` builds a Next.js standalone server. `NEXT_PUBLIC_API_URL`
  is baked in at build time, so pass it as a build argument.
- `DB_SSLMODE` selects the Postgres TLS mode and defaults to `require`; set it to
  `disable` only when the database is reached over a private network without TLS.

## Production policy

Production builds must fail closed when live dependencies are missing. They must not silently fall back to mock quotes, mock balances, fake transaction hashes, or fake transaction success messages.

## License

MIT
