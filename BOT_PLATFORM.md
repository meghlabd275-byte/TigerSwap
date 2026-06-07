# TigerSwap Bot Platform

## Overview

TigerSwap provides 9 fully automated trading bots with role-based access control. Bots can connect to 20+ DEXs and 200+ CEXs.

---

## Bot Types

### 1. Grid Trading Bot

**Use Case:** Sideways markets with consistent volatility

**How It Works:**
- Set price range (lower = buy zone, upper = sell zone)
- Divide range into grid levels
- Auto-buy at lower levels, auto-sell at upper

**Configuration:**
- Price range (min/max)
- Grid levels (10-100)
- Order size per grid
- Take profit per trade

**Example:**
```
ETH price: $2,000 - $2,500
Grid levels: 10
Each level: $50 apart
Buy at $2,000 → Sell at $2,050 → Profit $50
Buy at $2,050 → Sell at $2,100 → Profit $50
```

---

### 2. DCA Bot (Dollar-Cost Averaging)

**Use Case:** Long-term accumulation, reduce average cost

**How It Works:**
- Regular purchases at set intervals
- Regardless of price
- Accumulate over time

**Configuration:**
- Purchase amount
- Interval (hourly/daily/weekly)
- Max positions
- Stop loss %

**Example:**
```
Buy $100 ETH every day for 30 days
Day 1: $2,000 → 0.05 ETH
Day 15: $1,800 → 0.055 ETH
Day 30: $2,200 → 0.045 ETH
Average: ~$2,000 regardless of volatility
```

---

### 3. Arbitrage Bot

**Use Case:** Profit from price differences between exchanges

**How It Works:**
- Monitor prices across multiple DEXs/CEXs
- Buy low on one exchange
- Sell high on another
- Capture spread as profit

**Configuration:**
- Min spread % (e.g., 0.5%)
- Max trade size
- Allowed exchanges
- Slippage tolerance

**Example:**
```
Binance: ETH $2,000
Coinbase: ETH $2,010
Buy 10 ETH on Binance → Sell on Coinbase
Profit: $100 (minus fees)
```

---

### 4. Momentum Bot

**Use Case:** Trend-following in trending markets

**How It Works:**
- Detect momentum (RSI, MACD, Moving Averages)
- Enter when momentum turns bullish
- Exit when momentum reverses

**Configuration:**
- Indicators (RSI/MACD/MA)
- Entry threshold
- Exit threshold
- Position size

**Example:**
```
RSI crosses above 30 → Buy
RSI crosses below 70 → Sell
Captures trending moves
```

---

### 5. Mean Reversion Bot

**Use Case:** Price returns to average

**How It Works:**
- Calculate moving average
- Buy when price below average
- Sell when price above average
- Profit from reversion

**Configuration:**
- Moving average period (e.g., 20, 50, 200)
- Deviation threshold %
- Position size

**Example:**
```
20-period MA: $2,000
Price drops to $1,800 (-10%) → Buy
Price returns to $2,000 → Sell
```

---

### 6. Scalping Bot

**Use Case:** Quick trades, small profits

**How It Works:**
- Very short timeframes (seconds/minutes)
- Small profit targets (0.1-1%)
- High frequency
- Low risk per trade

**Configuration:**
- Profit target %
- Max hold time
- Max daily trades
- Stop loss %

**Example:**
```
Buy at $2,000.50 → Sell at $2,001.00
Profit: $0.50 per trade
100 trades/day = $50 profit
```

---

### 7. AI Trading Bot

**Use Case:** ML-based trading decisions

**How It Works:**
- Machine learning models
- Pattern recognition
- Sentiment analysis
- Adaptive to market conditions

**Configuration:**
- Model type
- Risk level (low/medium/high)
- Max position
- Training data period

**Features:**
- Pattern recognition
- Anomaly detection
- Sentiment analysis
- Adaptive learning

---

### 8. Signal Bot

**Use Case:** Copy trading, follow experts

**How It Works:**
- Follow trading signals
- Replicate trades
- Risk-adjusted sizing

**Configuration:**
- Signal source
- Copy ratio
- Max delay
- Stop loss

**Example:**
```
Follow "ExpertTrader"
Expert buys 10 ETH → Bot buys 1 ETH (10% copy)
Risk management applied automatically
```

---

### 9. Custom Bot

**Use Case:** User-defined strategies

**How It Works:**
- Custom trading logic
- Python/JavaScript strategies
- Full control

**Configuration:**
- Strategy code
- Parameters
- Risk limits
- Execution mode

---

## Bot Tiers

| Tier | Monthly Fee | Max Bots | Max DEX | Max CEX | Latency |
|------|------------|---------|---------|---------|---------|
| Free | $0 | 1 | 1 | 0 | 5s |
| Basic | $99 | 3 | 5 | 3 | 2s |
| Pro | $299 | 10 | 15 | 10 | 500ms |
| Enterprise | $999 | 50 | 50 | 30 | 100ms |

---

## Role-Based Access

### Admin
- View all bots
- Manage all bots
- Configure fees
- Approve clients
- View analytics
- Manage white-label

### Operator
- View team bots
- Manage team bots
- View analytics
- Cannot manage fees

### Client
- View own bots
- Manage own bots
- View own analytics
- Cannot access admin

---

## Endpoints

### Create Bot
```
POST /api/bot-subscription/instances/create
{
  "userId": "user_123",
  "botType": "grid|dca|arbitrage|...",
  "name": "My Bot"
}
```

### Start Bot
```
POST /api/bot-subscription/instances/start
{
  "id": "bot_123"
}
```

### Stop Bot
```
POST /api/bot-subscription/instances/stop
{
  "id": "bot_123"
}
```

### Get Bots
```
GET /api/bot-subscription/instances?userId=user_123
```

### Get Stats
```
GET /api/bot-subscription/stats
```

---

## Integration

### DEX Connections
- Uniswap V2/V3
- SushiSwap
- PancakeSwap
- QuickSwap
- Curve
- Balancer
- Jupiter (Solana)
- Raydium (Solana)
- Orca (Solana)
- 20+ more

### CEX Connections
- Binance
- Coinbase
- Kraken
- KuCoin
- Bybit
- OKX
- Huobi
- Gate
- Bitget
- MEXC
- 200+ more

---

## Analytics

### Per Bot
- Total P&L
- Total Volume
- Total Orders
- Win Rate
- Avg Latency
- Drawdown

### Platform
- Total Bots
- Active Bots
- Total Volume
- Total P&L
- By Bot Type
- By Tier

---

## Security

- API key required
- Rate limiting
- Position limits
- Auto-stop on errors
- Monitoring & alerts
- Encrypted connections

---

## Getting Started

1. **Register/Login** → Get JWT token
2. **Choose Tier** → Select bot tier
3. **Create Bot** → Select bot type
4. **Configure** → Set parameters
5. **Start** → Activate bot
6. **Monitor** → Track performance

---

## Support

- **Email:** bots@tigerswap.io
- **Discord:** https://discord.gg/tigerswap
- **Documentation:** https://docs.tigerswap.io