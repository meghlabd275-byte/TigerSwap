# TigerSwap Smart Contracts

## Overview

TigerSwap is a comprehensive decentralized exchange protocol with advanced features. This document provides a complete overview of all smart contracts.

## Contract Architecture

```
TigerSwap/
├── smart_contracts/evm_contracts/
│   ├── contracts/           # Core DEX Contracts
│   │   ├── TigerToken.sol
│   │   ├── Factory.sol
│   │   ├── Pair.sol
│   │   └── Router.sol
│   │
│   ├── concentrated/       # Concentrated Liquidity (V3-style)
│   │   └── TigerPoolV3.sol
│   │
│   ├── hooks/             # Hook System (V4-style)
│   │   └── TigerHooks.sol
│   │
│   ├── orderbook/          # Order Book CLOB
│   │   └── TigerOrderBook.sol
│   │
│   ├── advanced_orders/    # Advanced Orders
│   │   └── TigerAdvancedOrders.sol
│   │
│   ├── intent_routing/     # Intent-Based Routing
│   │   └── TigerIntentRouter.sol
│   │
│   ├── gas_optimization/    # Gas Optimization
│   │   └── TigerGasOptimizer.sol
│   │
│   ├── token_factory/      # Token Factory
│   │   └── TigerTokenFactory.sol
│   │
│   ├── insurance/          # Insurance Fund
│   │   └── TigerInsuranceFund.sol
│   │
│   ├── governance/         # Governance
│   │   └── TigerDAO.sol
│   │
│   ├── staking/            # Staking
│   │   └── TigerStaking.sol
│   │
│   ├── farming/           # Farming
│   │   └── TigerFarming.sol
│   │
│   ├── bridge/            # Cross-Chain Bridge
│   │   └── TigerBridge.sol
│   │
│   ├── vault/             # Vault
│   │   └── TigerVault.sol
│   │
│   └── treasury/           # Treasury
│       └── TigerTreasury.sol
│
├── mm_bot_platform/         # Bot Platform
│   ├── bot_admin/
│   │   └── TigerBotPlatform.sol
│   └── strategies/
│       └── TigerBotStrategies.sol
│
├── governance/              # Governance
│   └── proposals/
│       └── TigerGovernance.sol
│
├── wallet_ecosystem/        # Wallet
│   └── multisig/
│       └── TigerMultiSigWallet.sol
│
├── cross_chain_protocol/    # Cross-Chain
│   └── messaging/
│       └── TigerCrossChainMessenger.sol
│
└── admin_platform/         # Admin
    └── chain_management/
        └── TigerChainManager.sol
```

## Contract Descriptions

### Core DEX Contracts

| Contract | Description |
|----------|-------------|
| `TigerToken.sol` | ERC-20 token implementation |
| `Factory.sol` | Pool factory for creating trading pairs |
| `Pair.sol` | AMM pair contract with liquidity provision |
| `Router.sol` | Swap router for token exchanges |

### Advanced DEX Features

| Contract | Description |
|----------|-------------|
| `TigerPoolV3.sol` | Uniswap V3-style concentrated liquidity pools |
| `TigerHooks.sol` | Uniswap V4-style hook system for pool extensibility |
| `TigerOrderBook.sol` | dYdX/Hyperliquid-style order book for perps |
| `TigerAdvancedOrders.sol` | Stop-loss, take-profit, TWAP, trailing stop orders |
| `TigerIntentRouter.sol` | Intent-based routing (ERC-7683) |
| `TigerGasOptimizer.sol` | Gas optimization with batch execution |

### Ecosystem Contracts

| Contract | Description |
|----------|-------------|
| `TigerTokenFactory.sol` | Token creation with verification |
| `TigerInsuranceFund.sol` | Protocol insurance coverage |
| `TigerGovernance.sol` | DAO proposals and voting |
| `TigerStaking.sol` | Token staking rewards |
| `TigerFarming.sol` | Liquidity farming rewards |
| `TigerBridge.sol` | Cross-chain bridge |
| `TigerVault.sol` | Asset vault management |
| `TigerTreasury.sol` | Protocol treasury |

### Bot Platform

| Contract | Description |
|----------|-------------|
| `TigerBotPlatform.sol` | Bot platform with RBAC |
| `TigerBotStrategies.sol` | 13 trading bot strategies |

### Wallet & Cross-Chain

| Contract | Description |
|----------|-------------|
| `TigerMultiSigWallet.sol` | Gnosis Safe-style multi-sig |
| `TigerCrossChainMessenger.sol` | Cross-chain messaging |
| `TigerChainManager.sol` | Multi-chain management |

## Deployment

### Mainnet Addresses

| Network | Chain ID | Factory | Router |
|---------|----------|---------|--------|
| Ethereum | 1 | `0x...` | `0x...` |
| BSC | 56 | `0x...` | `0x...` |
| Polygon | 137 | `0x...` | `0x...` |
| Arbitrum | 42161 | `0x...` | `0x...` |
| Optimism | 10 | `0x...` | `0x...` |
| Base | 8453 | `0x...` | `0x...` |

### Deployment Steps

1. Deploy WETH (if needed)
2. Deploy Factory
3. Deploy Router
4. Set fee recipient
5. Configure parameters
6. Verify contracts

## Security

### Audits

- [ ] Security audit pending
- [ ] Bug bounty program
- [ ] Formal verification

### Security Features

- Reentrancy guards
- Access control
- Overflow protection (SafeMath)
- Emergency pause
- Timelock

## Gas Optimization

- Single-call swaps
- Batch transactions
- ERC-7683 intent standard
- Flashbots protection
- Priority fee optimization
