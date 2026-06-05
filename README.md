# TigerSwap - Enterprise-grade Multichain DEX Ecosystem

## 🚀 Complete Native Implementation - No Dependencies

TigerSwap is a **100% built-from-scratch** decentralized exchange ecosystem that does NOT depend on any third-party DEX protocols, aggregators, or wallet services.

### 🌐 Unlimited Blockchain Support
TigerSwap supports **unlimited EVM and Non-EVM blockchain networks** with full admin management (add, edit, update, remove chains dynamically).

#### EVM Chains (30+ supported)
- Ethereum, Sepolia, BNB Chain, Polygon, Mumbai, Arbitrum, Optimism, Base, Avalanche, Fantom, Cronos, Celo, Gnosis, Moonbeam, Kava, Linea, zkEVM, Scroll, Mantle, opBNB, Mode, Zora, Harmony, Metis, Shimmer, Core, and more...

#### Non-EVM Chains (15+ supported)
- **Solana**: Mainnet, Devnet, Testnet
- **Aptos**: Mainnet, Devnet, Testnet
- **Sui**: Mainnet, Devnet, Testnet
- **TON**: Mainnet
- **TRON**: Mainnet, Nile
- **Cosmos**: Cosmos Hub, Osmosis, Injective
- **NEAR**: Mainnet
- **Algorand**: Mainnet
- **Polkadot**: Polkadot, Kusama
- **Cardano**: Mainnet

### Native Implementations Built From Scratch:

| Component | Status | Description |
|-----------|--------|-------------|
| **Solana SDK** | ✅ Complete | Full RPC, SPL tokens, wallet adapters |
| **Aptos SDK** | ✅ Complete | Move language, BCS serialization |
| **TON SDK** | ✅ Complete | FunC contracts, Cell serialization |
| **Sui SDK** | ✅ Complete | Object model, Move execution |
| **TRON SDK** | ✅ Complete | TRC20, smart contracts |
| **Pi Network SDK** | ✅ Complete | Payment integration |
| **EVM Wallet Adapter** | ✅ Complete | MetaMask, WalletConnect v2, Coinbase |
| **Solana Wallet** | ✅ Complete | Phantom, Solflare, Backpack |
| **Aptos Wallet** | ✅ Complete | Martian, Sui Wallet |
| **TON Wallet** | ✅ Complete | Tonkeeper adapter |
| **Core AMM** | ✅ Complete | Concentrated liquidity, constant product |
| **MEV Protection** | ✅ Complete | Bundle builder, sandwich detector |
| **Order Book CLOB** | ✅ Complete | Limit orders, market orders |
| **DEX Aggregator** | ✅ Complete | Multi-hop routing, split routes |

## Technology Stack

| Layer | Technology |
|-------|------------|
| High Performance Routing | C++ |
| Wallet Core | Rust |
| Cryptography | Rust |
| Cross Chain Engine | Rust + Go |
| Backend APIs | Go |
| Enterprise Modules | Java |
| Smart Contracts | Solidity |
| Analytics / AI | Python |
| Internal Automation | Ruby |
| Frontend | TypeScript |
| Website | Next.js |
| UI Components | React |

## Supported Chains (All Native SDKs)

### EVM Chains
- Ethereum, BNB Chain, Polygon, Arbitrum, Optimism, Base, Avalanche, Fantom

### Non-EVM Chains (All Built From Scratch)
- **Solana** - SPL tokens, Serum/OpenBook DEX
- **Aptos** - Move language, fungible assets
- **TON** - Telegram Open Network, FunC contracts
- **Sui** - Object model, Move execution
- **TRON** - TRC20 tokens, smart contracts
- **Pi Network** - Payment integration
- **Bitcoin** - UTXO model

## Project Structure

```
TigerSwap/
├── blockchain_layer/           # Native blockchain SDKs
│   ├── solana_sdk/            # Solana RPC, SPL, AMM
│   ├── aptos_sdk/            # Aptos Move, BCS
│   ├── ton_sdk/              # TON Cell, FunC
│   ├── sui_sdk/              # Sui objects, Move
│   ├── tron_sdk/             # TRON TRC20
│   ├── pi_network_sdk/       # Pi payments
│   └── bitcoin_sdk/           # Bitcoin UTXO
├── core/                      # Core DEX engine
│   ├── amm/                  # Concentrated liquidity AMM
│   ├── orderbook/            # CLOB order book
│   ├── routing/              # DEX aggregator router
│   └── mev/                 # MEV protection
├── libs/                      # Libraries
│   ├── web3_wallet/          # EVM wallet adapters
│   └── routing/              # Routing engine
├── dex_aggregator/            # DEX aggregation
├── smart_contracts/           # EVM contracts
└── frontend/                  # UI applications
```

## Quick Start

```bash
# Install dependencies
npm install

# Build all packages
npm run build

# Run development
npm run dev
```

## License

MIT
Multichain Cryptocurrency Decentralised exchanges 
