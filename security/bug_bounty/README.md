# TigerSwap Bug Bounty Program

## Overview

TigerSwap is committed to ensuring the security of our protocol and user funds. We welcome the security research community to help identify and report vulnerabilities through our bug bounty program.

## Program Scope

### In-Scope Assets

| Category | Description | Severity Rewards |
|----------|-------------|-----------------|
| **Smart Contracts** | All contracts in `smart_contracts/` | Critical: $50,000 - $250,000 |
| **Core Protocol** | Rust trading engine, AMM, Orderbook | Critical: $25,000 - $100,000 |
| **Frontend** | Web interface, Wallet integration | High: $5,000 - $25,000 |
| **Mobile Apps** | iOS/Android apps | High: $5,000 - $25,000 |
| **Backend Services** | API, Indexer, Relayer | High: $5,000 - $25,000 |
| **Browser Extension** | Wallet extension | Medium: $1,000 - $5,000 |

### Vulnerability Severity Classification

| Severity | Criteria | Reward Range |
|----------|----------|--------------|
| **Critical (P1)** | Fund loss, contract exploitation, system compromise | $50,000 - $250,000 |
| **High (P2)** | Significant functionality impact, data exposure | $10,000 - $50,000 |
| **Medium (P3)** | Limited impact, workaround available | $2,500 - $10,000 |
| **Low (P4)** | Minor issues, documentation errors | $100 - $2,500 |

## Out of Scope

The following are NOT eligible for rewards:

- Social engineering attacks
- Denial of Service attacks (unless severe)
- Issues in third-party dependencies
- Vulnerabilities in testnets only
- Self-inflicted issues (your own contracts)
- Attacks while having >10% governance power

## Reporting Process

### 1. Submit Report

Submit findings to: **security@tigerswap.com**

Required format:

```
## Vulnerability Details
- Title:
- Severity:
- Category:
- Description:
- Impact:

## Technical Details
- Smart Contract:
- Function:
- Attack Vector:

## Reproduction Steps
1.
2.
3.

## Fix Recommendations
```

### 2. Response Timeline

| Action | Timeline |
|--------|----------|
| Initial Response | 24 hours |
| Severity Assessment | 3 days |
| Fix Timeline | 30 days (critical) |
| Bounty Payment | 7 days after fix |

### 3. Disclosure Policy

- **Private Disclosure**: 90-day grace period before public disclosure
- **Acknowledgment**: Public recognition (optional) in Hall of Fame
- **Duplicate Reports**: First valid report receives the reward

## Reward Structure

### Smart Contract Vulnerabilities

| Issue Type | Critical | High | Medium | Low |
|------------|----------|------|--------|-----|
| Reentrancy | $100K | $25K | $5K | $1K |
| Access Control | $100K | $25K | $5K | $1K |
| Integer Overflow | $75K | $20K | $4K | $500 |
| Front-Running | $50K | $15K | $3K | $500 |
| Oracle Manipulation | $75K | $20K | $4K | $500 |

### DeFi Specific Issues

| Issue Type | Critical | High | Medium | Low |
|------------|----------|------|--------|-----|
| Flash Loan Attack | $150K | $30K | $7K | $1K |
| Price Oracle Exploit | $100K | $25K | $5K | $1K |
| Liquidation Manipulation | $75K | $20K | $4K | $500 |
| Vault Exploit | $100K | $25K | $5K | $1K |

## Security Best Practices for Researchers

### Prohibited Actions

- ❌ Do NOT attack mainnet or testnet infrastructure
- ❌ Do NOT attempt to steal user funds
- ❌ Do NOT compromise user privacy
- ❌ Do NOT test with real funds
- ❌ Do NOT execute external calls to third-party contracts

### Allowed Actions

- ✅ Analyze contract code
- ✅ Test on testnet (if available)
- ✅ Submit detailed reports
- ✅ Ask clarifying questions

## Hall of Fame

We appreciate our security contributors:

| Researcher | Vulnerabilities | Total Bounty |
|------------|----------------|--------------|
| [Name] | [Issues] | $[Amount] |

## Contact

- **Security Email**: security@tigerswap.com
- **Emergency**: security@tigerswap.com (urgent)
- **PGP Key**: Available on website

## Legal

TigerSwap reserves the right to modify this program. By participating, you agree to our terms and conditions.

---

**Last Updated**: June 2026
**Version**: 1.0