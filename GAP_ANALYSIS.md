# TigerSwap Launch Readiness Tracker

## Status

TigerSwap is **not production-ready yet**. This document replaces the previous completion claims with a launch-readiness tracker that separates production-ready work from prototype, incomplete, untested, externally dependent, and planned work.

## Readiness categories

| Category | Meaning | Launch rule |
| --- | --- | --- |
| Production candidate | Builds, tests, has real integrations, has operational ownership, and is ready for audit/release review. | Can be included in a guarded testnet or mainnet release after sign-off. |
| Beta / testnet | Real implementation exists but coverage, monitoring, audit, or operational hardening is incomplete. | Testnet only; no uncapped TVL. |
| Prototype | Useful for UI or architecture validation but contains incomplete logic, placeholders, or missing production controls. | Must be disabled in production. |
| Incomplete connector | Interface exists, but live quotes, execution, error handling, or provider coverage is incomplete. | Must not route production traffic. |
| Untested module | Implementation exists without sufficient unit, integration, fork, fuzz, or end-to-end tests. | Must not be treated as launch-ready. |
| Externally dependent | Requires a third-party RPC, wallet, bridge, DEX, CEX, oracle, simulator, scanner, cloud service, or explorer. | Must document vendor, outage behavior, fallback, and key/secrets policy. |
| Planned | Not implemented or not validated. | Roadmap only. |

## Current blockers

| Area | Current status | Required before production launch |
| --- | --- | --- |
| Documentation | Previous docs overstated completion and denied mock/demo behavior. | Keep this tracker current and require evidence for every launch claim. |
| Root build | Build has been partially hardened, but all packages still need package-specific cleanup. | Root build and CI matrix must pass reliably. |
| Workspaces | Root package manager coverage is not representative of every service/module. | Expand package ownership, remove duplicate package names, and validate TS/Go/Rust/Solidity modules in CI. |
| Frontend | Next.js build blockers were identified in chain and swap pages. | Production frontend build, lint, accessibility, mobile, wallet lifecycle, and transaction lifecycle must pass. |
| Browser extension | Production path must never show fake transaction success or mock quotes. | Keep live API failures explicit and add transaction status polling. |
| DApp transaction signer | Simulation must use real provider calls. | Support provider-backed simulation, error reporting, fork/integration tests, and malicious dApp cases. |
| DEX connectors | Some connectors still contain mock pool/quote/execution fallbacks. | Production router must exclude connectors until live quote/execution tests pass. |
| EVM RPC configuration | Demo RPC URLs are not acceptable production defaults. | Require environment/provider configuration and fail closed when missing. |
| Wallet matrix | Hardware/browser support is incomplete. | Publish supported wallet/chain/signing matrix and test vectors. |
| Smart contracts | Test coverage is far below launch-grade. | Add unit, fork, fuzz, invariant, access-control, emergency, governance, bridge, and economic tests; complete audits. |
| Deployment | Network configs exist, but operational launch artifacts are incomplete. | Add manifests, address registry, ABI publishing, multisig/timelock ownership, runbooks, monitoring, and rollback plans. |
| Backend/API/indexing | API endpoints need production validation and operational controls. | Add versioned API schemas, auth, idempotency, transaction polling, indexer reconciliation, logs, metrics, and traces. |
| Security evidence | Internal claims are not sufficient. | Add audit reports, threat model, bug bounty, privileged role registry, dependency scans, static analysis, and gas/MEV analysis. |

## Release gates

### Phase 1: Honest and buildable
- [ ] Root TypeScript configuration has no invalid compiler options.
- [ ] Package-level TypeScript configs exist for buildable TS packages.
- [ ] Frontend production build passes.
- [ ] Root build script validates JS/TS, Go, Rust, and Solidity scopes that are ready for CI.
- [ ] This tracker is updated when a module changes readiness state.

### Phase 2: No unsafe demo behavior in production
- [ ] Browser extension never reports swap/transfer success unless the API returns a transaction hash.
- [ ] Quote failures are shown as failures, not local mock prices.
- [ ] Production connector execution paths never return fake transaction hashes.
- [ ] Demo/mock data is allowed only behind explicit local-development flags.
- [ ] RPC URLs and API URLs are environment/config driven.

### Phase 3: Protocol safety
- [ ] AMM/router/factory/pair tests cover normal and edge cases.
- [ ] Staking/farming/governance/bridge/vault tests are complete.
- [ ] Fuzz and invariant tests are in CI.
- [ ] Mainnet-fork tests cover common token edge cases.
- [ ] External audits are complete and findings are tracked to closure.
- [ ] Bug bounty is published before uncapped TVL.

### Phase 4: Testnet launch
- [ ] Contracts deployed and verified on testnets.
- [ ] Backend, indexers, and frontends deployed to staging/testnet.
- [ ] Closed beta validates quotes, execution, transaction failures, and latency.
- [ ] Incident drills and pause procedures are exercised.

### Phase 5: Guarded mainnet launch
- [ ] Limited initial chain/token set.
- [ ] TVL or transaction caps are active if risk warrants.
- [ ] Monitoring and alerting are live.
- [ ] Privileged roles are transferred to approved multisig/timelock.
- [ ] Post-launch expansion requires data-backed sign-off.
