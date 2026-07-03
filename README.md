# TigerSwap - Multichain DEX Ecosystem

TigerSwap is a multichain decentralized-exchange codebase that includes frontend apps, browser/mobile wallet surfaces, EVM smart contracts, blockchain SDKs, DEX/CEX connector modules, bot/market-maker modules, security modules, and backend/API components.

> **Launch status:** TigerSwap is **not production-ready yet**. See [`GAP_ANALYSIS.md`](./GAP_ANALYSIS.md) for the current launch-readiness tracker, blockers, and release gates.

## Current engineering priorities

1. Make the monorepo buildable and testable across TypeScript, Go, Rust, and Solidity packages.
2. Remove unsafe mock/demo behavior from production execution paths.
3. Add production-grade transaction lifecycle handling, provider-backed simulation, and clear failure states.
4. Expand smart-contract tests to include unit, integration, fork, fuzz, invariant, access-control, governance, bridge, and emergency scenarios.
5. Add deployment manifests, address registries, monitoring, incident runbooks, and security evidence before any uncapped mainnet launch.

## Technology stack

| Layer | Technology |
| --- | --- |
| Frontend | Next.js, React, TypeScript |
| Admin / dashboards | React, TypeScript |
| Backend APIs | Go, TypeScript |
| Smart contracts | Solidity, Hardhat |
| Routing / trading engines | Rust, TypeScript, C++/Go modules where present |
| Analytics / AI | Python, TypeScript |
| Mobile/browser wallet surfaces | React Native, browser extension JavaScript/TypeScript |

## Repository structure

```text
TigerSwap/
├── admin_platform/             # Admin and chain-management modules
├── ai_platform/                # Price/risk intelligence modules
├── analytics_platform/         # Dashboards and analytics surfaces
├── api_gateway/                # API gateway and REST modules
├── blockchain_layer/           # Chain SDKs and chain-specific integrations
├── browser_extension/          # Browser wallet/extension UI
├── cex_connectors/             # CEX connector modules
├── core/                       # Core trading/routing engines
├── cross_chain_protocol/       # Bridge/messaging modules
├── dapp_browser/               # DApp browser, permissions, wallet injector, signer
├── dex_aggregator/             # Aggregation/routing modules
├── dex_connectors/             # DEX connector modules
├── frontend/                   # Web, admin, and frontend SDK packages
├── governance/                 # Governance contracts/services
├── libs/                       # Shared SDKs/libraries
├── market_maker_platform/      # Market-maker modules
├── mm_bot_platform/            # Bot platform modules
├── mobile/                     # Mobile wallet app
├── security_platform/          # Security scanners, fraud/rate-limit/circuit-breaker modules
├── smart_contracts/            # EVM contracts and Hardhat project
└── user_features/              # User-facing feature modules
```

## Quick start

```bash
npm install
npm run build
```

The root build is intended to become the primary launch gate. Until every package is fully integrated, always review [`GAP_ANALYSIS.md`](./GAP_ANALYSIS.md) before relying on any module for production.

## Production policy

Production builds must fail closed when live dependencies are missing. They must not silently fall back to mock quotes, mock balances, fake transaction hashes, or fake transaction success messages.

## License

MIT
