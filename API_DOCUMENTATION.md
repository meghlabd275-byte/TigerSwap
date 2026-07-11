# TigerSwap API Documentation

## Overview

TigerSwap API provides programmatic access to all trading, swapping, and wallet operations. All endpoints return JSON.

**Base URL:** `https://api.tigerswap.io`

**Authentication:** API Key or JWT Token

---

## Authentication

### Register Platform
```
POST /api/external-platform/register
Content-Type: application/json

{
  "name": "platform_name",
  "type": "cex|dex|wallet",
  "apiKey": "your_api_key",
  "tier": "free|basic|pro|enterprise"
}
```

**Response:**
```json
{
  "success": true,
  "data": {
    "id": "platform_123",
    "name": "platform_name",
    "tier": "basic",
    "isActive": true,
    "permissions": {
      "canTrade": true,
      "canSwap": true,
      "canAddLiquidity": false,
      "canBridge": false,
      "canCreateToken": false
    },
    "rateLimitPerMin": 300,
    "monthlyFeeUsd": 99
  }
}
```

### Login
```
POST /api/auth/login
Content-Type: application/json

{
  "email": "user@example.com",
  "password": "secure_password"
}
```

**Response:**
```json
{
  "success": true,
  "token": "jwt_token_here",
  "user": {
    "id": "user_123",
    "email": "user@example.com",
    "username": "username",
    "role": "admin"
  }
}
```

---

## Tier Configurations

### Get All Tiers
```
GET /api/external-platform/tiers
```

**Response:**
```json
{
  "success": true,
  "data": [
    {
      "name": "free",
      "monthlyFeeUsd": 0,
      "maxApiCallsPerMin": 60,
      "maxDailyVolume": 10000,
      "maxPositions": 3,
      "features": {
        "canTrade": true,
        "canSwap": false,
        "canAddLiquidity": false,
        "canBridge": false,
        "canCreateToken": false
      }
    },
    {
      "name": "basic",
      "monthlyFeeUsd": 99,
      "maxApiCallsPerMin": 300,
      "maxDailyVolume": 100000,
      "maxPositions": 10,
      "features": {
        "canTrade": true,
        "canSwap": true,
        "canAddLiquidity": false,
        "canBridge": false,
        "canCreateToken": false
      }
    },
    {
      "name": "pro",
      "monthlyFeeUsd": 299,
      "maxApiCallsPerMin": 1000,
      "maxDailyVolume": 1000000,
      "maxPositions": 50,
      "features": {
        "canTrade": true,
        "canSwap": true,
        "canAddLiquidity": true,
        "canBridge": true,
        "canCreateToken": false
      }
    },
    {
      "name": "enterprise",
      "monthlyFeeUsd": 999,
      "maxApiCallsPerMin": 5000,
      "maxDailyVolume": 10000000,
      "maxPositions": 200,
      "features": {
        "canTrade": true,
        "canSwap": true,
        "canAddLiquidity": true,
        "canBridge": true,
        "canCreateToken": true
      }
    }
  ]
}
```

---

## Trading

### Execute Trade
```
POST /api/external-platform/trade
Authorization: Bearer YOUR_API_KEY
Content-Type: application/json

{
  "platform": "platform_id",
  "symbol": "ETH/USDT",
  "side": "buy|sell",
  "type": "market|limit",
  "amount": "0.1",
  "price": "2500.00"
}
```

**Response:**
```json
{
  "success": true,
  "data": {
    "id": "order_123456",
    "platform": "platform_id",
    "symbol": "ETH/USDT",
    "side": "buy",
    "type": "limit",
    "amount": "0.1",
    "price": "2500.00",
    "status": "filled",
    "timestamp": 1700000000
  }
}
```

---

## Swapping

### Execute Swap
```
POST /api/external-platform/swap
Authorization: Bearer YOUR_API_KEY
Content-Type: application/json

{
  "platform": "platform_id",
  "chainId": 1,
  "tokenIn": "0x...WETH",
  "tokenOut": "0x...USDC",
  "amountIn": "1.0",
  "slippage": 0.5
}
```

**Response:**
```json
{
  "success": true,
  "data": {
    "id": "swap_123456",
    "platform": "platform_id",
    "chainId": 1,
    "tokenIn": "0x...WETH",
    "tokenOut": "0x...USDC",
    "amountIn": "1.0",
    "amountOut": "2450.00",
    "slippage": 0.5,
    "status": "completed",
    "timestamp": 1700000000
  }
}
```

---

## Liquidity

### Add Liquidity
```
POST /api/external-platform/liquidity
Authorization: Bearer YOUR_API_KEY
Content-Type: application/json

{
  "platform": "platform_id",
  "chainId": 1,
  "tokenA": "0x...WETH",
  "tokenB": "0x...USDC",
  "amountA": "1.0",
  "amountB": "2500.0"
}
```

**Response:**
```json
{
  "success": true,
  "data": {
    "id": "liq_123456",
    "platform": "platform_id",
    "chainId": 1,
    "tokenA": "0x...WETH",
    "tokenB": "0x...USDC",
    "amountA": "1.0",
    "amountB": "2500.0",
    "status": "added",
    "timestamp": 1700000000
  }
}
```

---

## Bot Subscription

### Get Bot Tiers
```
GET /api/bot-subscription/tiers
```

**Response:**
```json
{
  "success": true,
  "tiers": [
    {
      "id": "free",
      "name": "free",
      "displayName": "Free",
      "monthlyFeeUsd": 0,
      "perDexFeeUsd": 0,
      "perCexFeeUsd": 0,
      "maxBots": 1,
      "maxDEXs": 1,
      "maxCEXs": 0,
      "maxPositionUsd": 1000,
      "maxDailyVolume": 10000,
      "latencyTargetMs": 5000,
      "isActive": true
    },
    {
      "id": "basic",
      "name": "basic",
      "displayName": "Basic",
      "monthlyFeeUsd": 99,
      "perDexFeeUsd": 10,
      "perCexFeeUsd": 15,
      "maxBots": 3,
      "maxDEXs": 5,
      "maxCEXs": 3,
      "maxPositionUsd": 10000,
      "maxDailyVolume": 100000,
      "latencyTargetMs": 2000,
      "isActive": true
    },
    {
      "id": "pro",
      "name": "pro",
      "displayName": "Pro",
      "monthlyFeeUsd": 299,
      "perDexFeeUsd": 8,
      "perCexFeeUsd": 12,
      "maxBots": 10,
      "maxDEXs": 15,
      "maxCEXs": 10,
      "maxPositionUsd": 100000,
      "maxDailyVolume": 1000000,
      "latencyTargetMs": 500,
      "isActive": true
    },
    {
      "id": "enterprise",
      "name": "enterprise",
      "displayName": "Enterprise",
      "monthlyFeeUsd": 999,
      "perDexFeeUsd": 5,
      "perCexFeeUsd": 8,
      "maxBots": 50,
      "maxDEXs": 50,
      "maxCEXs": 30,
      "maxPositionUsd": 1000000,
      "maxDailyVolume": 10000000,
      "latencyTargetMs": 100,
      "isActive": true
    }
  ]
}
```

### Create Bot Instance
```
POST /api/bot-subscription/instances/create
Authorization: Bearer YOUR_JWT_TOKEN
Content-Type: application/json

{
  "userId": "user_123",
  "userEmail": "user@example.com",
  "botType": "grid|dca|arbitrage|momentum|mean_reversion|scalping|ai|signal|custom",
  "name": "My Trading Bot"
}
```

**Response:**
```json
{
  "success": true,
  "instance": {
    "id": "bot_123456",
    "userId": "user_123",
    "userEmail": "user@example.com",
    "botType": "grid",
    "name": "My Trading Bot",
    "status": "stopped",
    "connectedDEXs": 0,
    "connectedCEXs": 0,
    "totalPnL": 0,
    "totalVolume": 0,
    "totalOrders": 0,
    "avgLatencyUs": 0,
    "createdAt": "2024-01-01T00:00:00Z"
  }
}
```

### Start Bot
```
POST /api/bot-subscription/instances/start
Authorization: Bearer YOUR_JWT_TOKEN
Content-Type: application/json

{
  "id": "bot_123456"
}
```

### Stop Bot
```
POST /api/bot-subscription/instances/stop
Authorization: Bearer YOUR_JWT_TOKEN
Content-Type: application/json

{
  "id": "bot_123456"
}
```

---

## Admin

### Login
```
POST /api/admin/login
Content-Type: application/json

{
  "username": "super_admin",
  "password": "admin_password"
}
```

### Get Platform Stats
```
GET /api/admin/stats
Authorization: Bearer ADMIN_JWT_TOKEN
```

### Manage Blockchains
```
GET  /api/admin/blockchains
POST /api/admin/blockchains
POST /api/admin/blockchains/:id/pause
POST /api/admin/blockchains/:id/resume
```

### Manage Tokens
```
GET  /api/admin/tokens
POST /api/admin/tokens
POST /api/admin/tokens/:id/approve
POST /api/admin/tokens/:id/reject
```

### Manage Fees
```
GET  /api/admin/fees
POST /api/admin/fees
PUT  /api/admin/fees/:id
DELETE /api/admin/fees/:id
```

---

## White Label

### Get Clients
```
GET /api/white-label/clients
Authorization: Bearer ADMIN_JWT_TOKEN
```

### Approve Client
```
POST /api/white-label/clients/:id/approve
Authorization: Bearer ADMIN_JWT_TOKEN
Content-Type: application/json

{
  "apiKeyId": "key_123"
}
```

### Get Revenue
```
GET /api/white-label/revenue
Authorization: Bearer ADMIN_JWT_TOKEN
```

---

## Rate Limits

| Tier | Requests/Min | Daily Volume |
|------|-------------|--------------|
| Free | 60 | $10,000 |
| Basic | 300 | $100,000 |
| Pro | 1,000 | $1,000,000 |
| Enterprise | 5,000 | $10,000,000 |

### Check Rate Limit
```
GET /api/external-platform/rate-limit?platform=platform_id
```

---

## Error Codes

| Code | Description |
|------|-------------|
| 400 | Bad Request |
| 401 | Unauthorized |
| 403 | Forbidden |
| 404 | Not Found |
| 429 | Rate Limit Exceeded |
| 500 | Internal Server Error |

---

## Webhooks

Configure webhooks for real-time notifications:

```json
{
  "event": "trade.executed",
  "data": {
    "orderId": "order_123",
    "symbol": "ETH/USDT",
    "side": "buy",
    "amount": "0.1",
    "price": "2500.00"
  }
}
```

---

## SDK

### Go
```go
import "github.com/tigerswap/sdk-go"

client := tigerswap.NewClient("YOUR_API_KEY")
order, err := client.ExecuteTrade(tigerswap.TradeRequest{
    Platform: "platform_id",
    Symbol: "ETH/USDT",
    Side: "buy",
    Type: "market",
    Amount: "0.1",
})
```

### JavaScript
```javascript
import { TigerSwap } from '@tigerswap/sdk'

const client = new TigerSwap('YOUR_API_KEY')
const order = await client.executeTrade({
    platform: 'platform_id',
    symbol: 'ETH/USDT',
    side: 'buy',
    type: 'market',
    amount: '0.1'
})
```

---

## Support

- **Email:** api@tigerswap.io
- **Discord:** https://discord.gg/tigerswap