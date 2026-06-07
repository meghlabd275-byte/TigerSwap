# TigerSwap Installation Guide

## System Requirements

### Hardware Requirements
- **CPU:** 8+ cores (recommended 16+)
- **RAM:** 32GB minimum (recommended 64GB)
- **Storage:** 500GB SSD minimum (recommended 1TB NVMe)
- **Network:** 1Gbps bandwidth

### Software Requirements

#### Backend
- **Go:** 1.21+
- **PostgreSQL:** 14+
- **Redis:** 7.0+
- **Node.js:** 18+ (for frontend)
- **React:** 18+

#### Infrastructure
- **Docker:** 24.0+
- **Docker Compose:** 2.0+
- **Nginx:** for reverse proxy

### Supported Operating Systems
- Ubuntu 22.04 LTS (recommended)
- Debian 12+
- macOS 13+

---

## Quick Start

### 1. Clone Repository
```bash
git clone https://github.com/meghlabd275-byte/TigerSwap.git
cd TigerSwap
```

### 2. Database Setup
```bash
# Start PostgreSQL
docker run -d \
  --name tigerswap-db \
  -e POSTGRES_PASSWORD=your_secure_password \
  -e POSTGRES_DB=tigerswap \
  -v tigerswap-data:/var/lib/postgresql/data \
  -p 5432:5432 \
  postgres:14

# Run migrations
psql -h localhost -U postgres -d tigerswap -f database/schemas/extended_schema.sql
```

### 3. Configure Environment
```bash
cp .env.example .env
# Edit .env with your configuration
```

### 4. Build & Start
```bash
# Build backend
cd api_gateway && go build -o tigerswap-api .

# Start frontend
cd frontend/web_nextjs && npm install && npm run build

# Start services
docker-compose up -d
```

---

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|----------|
| `PORT` | API server port | `8080` |
| `DB_HOST` | PostgreSQL host | `localhost` |
| `DB_PORT` | PostgreSQL port | `5432` |
| `DB_USER` | Database user | `postgres` |
| `DB_PASSWORD` | Database password | - |
| `REDIS_HOST` | Redis host | `localhost` |
| `JWT_SECRET` | JWT signing key | - |
| `ENCRYPTION_KEY` | Encryption key | - |

### Blockchain Configuration

TigerSwap supports 40 blockchains:

**EVM (20):**
- Ethereum, BNB Chain, Polygon, Arbitrum, Optimism, Base, Avalanche, Cronos, Celo, Harmony, HECO, Klaytn, Oasis, IOTA, Aurora, Metis, Boba, Velas, Meter, Milkomeda

**Non-EVM (20):**
- Solana, Cosmos, Sui, Aptos, Tron, Bitcoin, Litecoin, Dogecoin, XRP, Near, Hedera, Algorand, MultiversX, Ton, Kava, Celestia, Injective, Sei, Osmosis, Dymension, Stargaze, Neutron

### Token Configuration

50+ tokens pre-installed including:
- **Stablecoins:** USDC, USDT, DAI, BUSD, TUSD, USDP
- **Native:** ETH, BNB, MATIC, AVAX, SOL, ATOM, TRX
- **Popular:** WBTC, LINK, UNI, AAVE, MKR, DOT, ADA, XRP

---

## API Endpoints

### Authentication
```
POST /api/auth/login        - Login
POST /api/auth/register     - Register
POST /api/auth/logout      - Logout
GET  /api/auth/me          - Get current user
```

### External Platform
```
GET  /api/external-platform/tiers     - Get tier configs
POST /api/external-platform/register - Register platform
POST /api/external-platform/trade   - Execute trade
POST /api/external-platform/swap  - Execute swap
GET  /api/external-platform/stats  - Platform stats
```

### Bot Subscription
```
GET  /api/bot-subscription/tiers       - Get bot tiers
POST /api/bot-subscription/subscriptions - Create subscription
GET  /api/bot-subscription/instances  - Get bot instances
POST /api/bot-subscription/instances/create - Create bot
POST /api/bot-subscription/instances/start  - Start bot
POST /api/bot-subscription/instances/stop   - Stop bot
```

### Master Wallet
```
GET  /api/master-wallet/wallets     - List wallets
POST /api/master-wallet/wallets     - Create wallet
POST /api/master-wallet/send       - Send transaction
POST /api/master-wallet/swap       - Swap tokens
GET  /api/master-wallet/transactions - Transaction history
```

### Admin
```
POST /api/admin/login              - Admin login
GET  /api/admin/stats             - Platform stats
GET  /api/admin/fees              - Fee configs
POST /api/admin/fees              - Create fee config
GET  /api/admin/blockchains       - List blockchains
POST /api/admin/blockchains     - Add blockchain
POST /api/admin/blockchains/:id/pause - Pause blockchain
POST /api/admin/blockchains/:id/resume - Resume blockchain
GET  /api/admin/tokens           - Token listings
POST /api/admin/tokens          - Add token
POST /api/admin/tokens/:id/approve - Approve token
```

### White Label
```
GET  /api/white-label/clients     - List clients
POST /api/white-label/clients    - Create client
POST /api/white-label/clients/:id/approve - Approve client
GET  /api/white-label/revenue    - Platform revenue
```

---

## Bot Platform

### Bot Tiers

| Tier | Monthly Fee | Max Bots | Max DEX | Max CEX |
|------|------------|---------|---------|---------|
| Free | $0 | 1 | 1 | 0 |
| Basic | $99 | 3 | 5 | 3 |
| Pro | $299 | 10 | 15 | 10 |
| Enterprise | $999 | 50 | 50 | 30 |

### Bot Types

1. **Grid Trading Bot** - Buy/sell at grid intervals
2. **DCA Bot** - Dollar-cost averaging
3. **Arbitrage Bot** - Cross-exchange profit
4. **Momentum Bot** - Trend following
5. **Mean Reversion Bot** - Price reversion
6. **Scalping Bot** - Quick trades
7. **AI Trading Bot** - ML-based
8. **Signal Bot** - Copy trading
9. **Custom Bot** - User-defined

### Bot Use Cases

#### Grid Trading
- Set price range with grid levels
- Auto-buy at lower levels, auto-sell at upper
- Profit from volatility

#### DCA (Dollar-Cost Averaging)
- Regular purchases regardless of price
- Reduce average cost over time
- Configurable intervals

#### Arbitrage
- Exploit price differences between exchanges
- Cross-DEX trading
- Cross-CEX trading

#### Momentum
- Follow trending assets
- Enter on momentum, exit on reversal
- Configurable indicators

---

## External Connections

### CEX Integration (200+)
- Binance, Coinbase, Kraken, KuCoin, Bybit, OKX, Huobi, Gate, Bitget, MEXC, and more

### DEX Integration (20+)
- Uniswap V2/V3, SushiSwap, PancakeSwap, QuickSwap, Curve, Balancer, Jupiter, Raydium, Orca

### Wallet Integration
- MetaMask, Trust Wallet, WalletConnect, Rainbow, Coinbase Wallet

---

## Security

### Features
- Rate limiting per tier
- API key authentication
- JWT tokens with expiry
- Encryption at rest and in transit
- Wallet risk scoring
- Transaction monitoring

### Tier Rate Limits

| Tier | Requests/Min | Daily Volume |
|------|-------------|--------------|
| Free | 60 | $10,000 |
| Basic | 300 | $100,000 |
| Pro | 1,000 | $1,000,000 |
| Enterprise | 5,000 | $10,000,000 |

---

## Deployment

### Production
```bash
# Build all services
docker-compose build

# Start with monitoring
docker-compose -f docker-compose.prod.yml up -d

# Check status
docker-compose ps
```

### Scaling
```bash
# Scale API instances
docker-compose up -d --scale api=3

# Load balance with Nginx
```

---

## Support

- **Documentation:** https://docs.tigerswap.io
- **API Reference:** https://api.tigerswap.io/docs
- **Discord:** https://discord.gg/tigerswap
- **Email:** support@tigerswap.io

---

## License

Proprietary - All rights reserved