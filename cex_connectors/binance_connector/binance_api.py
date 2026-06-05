"""
TigerSwap CEX Connector - Binance
Real Binance API integration for trading, balances, and market data
"""

import asyncio
import hashlib
import hmac
import time
import json
from dataclasses import dataclass, field
from typing import Dict, List, Optional, Tuple, Any
from enum import Enum
import httpx

@dataclass
class BinanceConfig:
    """Binance API configuration"""
    api_key: str
    api_secret: str
    testnet: bool = False
    recv_window: int = 5000
    
    # Rate limiting
    requests_per_second: int = 10
    requests_per_minute: int = 1200

@dataclass
class BinanceOrder:
    """Binance order structure"""
    symbol: str
    order_id: int
    client_order_id: str
    price: float
    orig_qty: float
    executed_qty: float
    status: str
    type: str
    side: str
    transact_time: int

@dataclass
class BinanceBalance:
    """Account balance"""
    asset: str
    free: float
    locked: float
    total: float

@dataclass
class BinanceTicker:
    """24hr ticker data"""
    symbol: str
    price_change: float
    price_change_percent: float
    last_price: float
    high_price: float
    low_price: float
    volume: float
    quote_volume: float

@dataclass
class BinanceTrade:
    """Trade execution"""
    id: int
    symbol: str
    price: float
    qty: float
    commission: float
    commission_asset: str
    time: int
    is_buyer: bool

class BinanceOrderSide(Enum):
    BUY = "BUY"
    SELL = "SELL"

class BinanceOrderType(Enum):
    LIMIT = "LIMIT"
    MARKET = "MARKET"
    STOP_LOSS = "STOP_LOSS"
    STOP_LOSS_LIMIT = "STOP_LOSS_LIMIT"
    TAKE_PROFIT = "TAKE_PROFIT"
    TAKE_PROFIT_LIMIT = "TAKE_PROFIT_LIMIT"
    LIMIT_MAKER = "LIMIT_MAKER"

class BinanceTimeInForce(Enum):
    GTC = "GTC"  # Good Till Cancel
    IOC = "IOC"  # Immediate Or Cancel
    FOK = "FOK"  # Fill Or Kill

class BinanceConnector:
    """
    Real Binance API connector for TigerSwap.
    Supports spot trading, margin trading, and market data.
    """
    
    # API Endpoints
    BASE_URL = "https://api.binance.com"
    TESTNET_URL = "https://testnet.binance.vision"
    MARGIN_URL = "https://api.binance.com"
    
    # Rate limits
    WEIGHT_LIMIT = 1200  # per minute
    ORDER_LIMIT = 10  # per second
    
    def __init__(self, config: BinanceConfig):
        self.config = config
        self.base_url = self.TESTNET_URL if config.testnet else self.BASE_URL
        self.http_client = httpx.AsyncClient(timeout=30.0)
        
        # Rate limiting
        self._request_times: List[float] = []
        self._last_weight_reset = time.time()
        self._current_weight = 0
        
        # Market cache
        self._exchange_info_cache: Optional[Dict] = None
        self._cache_time = 0
        self._cache_ttl = 3600  # 1 hour
    
    async def _generate_signature(self, params: Dict) -> str:
        """Generate HMAC SHA256 signature"""
        query_string = "&".join([f"{k}={v}" for k, v in params.items()])
        signature = hmac.new(
            self.config.api_secret.encode("utf-8"),
            query_string.encode("utf-8"),
            hashlib.sha256
        ).hexdigest()
        return signature
    
    async def _make_request(
        self,
        method: str,
        endpoint: str,
        params: Optional[Dict] = None,
        signed: bool = False,
        weight: int = 1
    ) -> Dict:
        """
        Make authenticated request to Binance API.
        Handles rate limiting and signature generation.
        """
        # Rate limiting
        await self._check_rate_limit(weight)
        
        # Build headers
        headers = {
            "X-MBX-APIKEY": self.config.api_key,
            "Content-Type": "application/x-www-form-urlencoded"
        }
        
        # Add timestamp and signature for signed requests
        if signed:
            params = params or {}
            params["timestamp"] = int(time.time() * 1000)
            params["recvWindow"] = self.config.recv_window
            params["signature"] = await self._generate_signature(params)
        
        url = f"{self.base_url}{endpoint}"
        
        try:
            if method == "GET":
                response = await self.http_client.get(url, headers=headers, params=params)
            elif method == "POST":
                response = await self.http_client.post(url, headers=headers, data=params)
            elif method == "PUT":
                response = await self.http_client.put(url, headers=headers, data=params)
            elif method == "DELETE":
                response = await self.http_client.delete(url, headers=headers, params=params)
            else:
                raise ValueError(f"Unknown HTTP method: {method}")
            
            # Check response
            if response.status_code != 200:
                error_data = response.json()
                raise BinanceAPIError(
                    code=error_data.get("code", response.status_code),
                    message=error_data.get("msg", "Unknown error")
                )
            
            return response.json()
            
        except httpx.HTTPError as e:
            raise BinanceAPIError(code=0, message=str(e))
    
    async def _check_rate_limit(self, weight: int):
        """Check and enforce rate limits"""
        current_time = time.time()
        
        # Reset weights every minute
        if current_time - self._last_weight_reset >= 60:
            self._current_weight = 0
            self._last_weight_reset = current_time
        
        # Check weight limit
        if self._current_weight + weight > self.WEIGHT_LIMIT:
            sleep_time = 60 - (current_time - self._last_weight_reset)
            if sleep_time > 0:
                await asyncio.sleep(sleep_time)
                self._current_weight = 0
                self._last_weight_reset = time.time()
        
        self._current_weight += weight
    
    # ==================== Market Data ====================
    
    async def get_ticker(self, symbol: str) -> BinanceTicker:
        """Get 24hr ticker price change statistics"""
        data = await self._make_request(
            "GET",
            "/api/v3/ticker/24hr",
            params={"symbol": symbol.upper()},
            weight=1
        )
        
        return BinanceTicker(
            symbol=data["symbol"],
            price_change=float(data["priceChange"]),
            price_change_percent=float(data["priceChangePercent"]),
            last_price=float(data["lastPrice"]),
            high_price=float(data["highPrice"]),
            low_price=float(data["lowPrice"]),
            volume=float(data["volume"]),
            quote_volume=float(data["quoteVolume"])
        )
    
    async def get_order_book(
        self,
        symbol: str,
        limit: int = 100
    ) -> Dict:
        """Get order book depth"""
        data = await self._make_request(
            "GET",
            "/api/v3/depth",
            params={"symbol": symbol.upper(), "limit": limit},
            weight=5
        )
        
        return {
            "last_update_id": data["lastUpdateId"],
            "bids": [[float(p), float(q)] for p, q in data["bids"]],
            "asks": [[float(p), float(q)] for p, q in data["asks"]]
        }
    
    async def get_recent_trades(self, symbol: str, limit: int = 100) -> List[Dict]:
        """Get recent trades"""
        data = await self._make_request(
            "GET",
            "/api/v3/trades",
            params={"symbol": symbol.upper(), "limit": limit},
            weight=5
        )
        
        return [
            {
                "id": t["id"],
                "price": float(t["price"]),
                "qty": float(t["qty"]),
                "time": t["time"],
                "is_buyer_maker": t["isBuyerMaker"]
            }
            for t in data
        ]
    
    async def get_exchange_info(self, force_refresh: bool = False) -> Dict:
        """Get exchange trading rules and symbol information"""
        current_time = time.time()
        
        if not force_refresh and self._exchange_info_cache and \
           current_time - self._cache_time < self._cache_ttl:
            return self._exchange_info_cache
        
        data = await self._make_request("GET", "/api/v3/exchangeInfo", weight=10)
        
        # Cache the result
        self._exchange_info_cache = data
        self._cache_time = current_time
        
        return data
    
    async def get_klines(
        self,
        symbol: str,
        interval: str = "1h",
        limit: int = 500
    ) -> List[Dict]:
        """Get candlestick data"""
        data = await self._make_request(
            "GET",
            "/api/v3/klines",
            params={
                "symbol": symbol.upper(),
                "interval": interval,
                "limit": limit
            },
            weight=5
        )
        
        return [
            {
                "open_time": k[0],
                "open": float(k[1]),
                "high": float(k[2]),
                "low": float(k[3]),
                "close": float(k[4]),
                "volume": float(k[5]),
                "close_time": k[6]
            }
            for k in data
        ]
    
    # ==================== Account Operations ====================
    
    async def get_account_info(self) -> Dict:
        """Get account information"""
        data = await self._make_request("GET", "/api/v3/account", signed=True, weight=10)
        
        return {
            "account_type": data.get("accountType", "SPOT"),
            "maker_commission": data.get("makerCommission", 0),
            "taker_commission": data.get("takerCommission", 0),
            "balances": [
                BinanceBalance(
                    asset=b["asset"],
                    free=float(b["free"]),
                    locked=float(b["locked"]),
                    total=float(b["free"]) + float(b["locked"])
                )
                for b in data.get("balances", [])
                if float(b["free"]) > 0 or float(b["locked"]) > 0
            ]
        }
    
    async def get_balance(self, asset: str) -> BinanceBalance:
        """Get balance for specific asset"""
        account = await self.get_account_info()
        
        for balance in account["balances"]:
            if balance.asset.upper() == asset.upper():
                return balance
        
        return BinanceBalance(asset=asset, free=0.0, locked=0.0, total=0.0)
    
    async def get_all_balances(self) -> List[BinanceBalance]:
        """Get all non-zero balances"""
        account = await self.get_account_info()
        return account["balances"]
    
    # ==================== Trading ====================
    
    async def create_order(
        self,
        symbol: str,
        side: BinanceOrderSide,
        order_type: BinanceOrderType,
        quantity: float,
        price: Optional[float] = None,
        time_in_force: Optional[BinanceTimeInForce] = None,
        stop_price: Optional[float] = None,
        client_order_id: Optional[str] = None
    ) -> BinanceOrder:
        """
        Create a new order.
        
        Args:
            symbol: Trading pair (e.g., "ETHUSDT")
            side: BUY or SELL
            order_type: Type of order
            quantity: Order quantity
            price: Order price (required for LIMIT orders)
            time_in_force: Time in force (GTC, IOC, FOK)
            stop_price: Stop price for STOP orders
            client_order_id: Optional client-provided order ID
        """
        params = {
            "symbol": symbol.upper(),
            "side": side.value,
            "type": order_type.value,
            "quantity": str(quantity)
        }
        
        if price:
            params["price"] = str(price)
        
        if time_in_force:
            params["timeInForce"] = time_in_force.value
        
        if stop_price:
            params["stopPrice"] = str(stop_price)
        
        if client_order_id:
            params["newClientOrderId"] = client_order_id
        
        data = await self._make_request("POST", "/api/v3/order", params, signed=True, weight=1)
        
        return BinanceOrder(
            symbol=data["symbol"],
            order_id=data["orderId"],
            client_order_id=data["clientOrderId"],
            price=float(data["price"]),
            orig_qty=float(data["origQty"]),
            executed_qty=float(data["executedQty"]),
            status=data["status"],
            type=data["type"],
            side=data["side"],
            transact_time=data["transactTime"]
        )
    
    async def get_order(
        self,
        symbol: str,
        order_id: Optional[int] = None,
        client_order_id: Optional[str] = None
    ) -> BinanceOrder:
        """Get order details"""
        params = {"symbol": symbol.upper()}
        
        if order_id:
            params["orderId"] = order_id
        if client_order_id:
            params["origClientOrderId"] = client_order_id
        
        data = await self._make_request("GET", "/api/v3/order", params, signed=True, weight=1)
        
        return BinanceOrder(
            symbol=data["symbol"],
            order_id=data["orderId"],
            client_order_id=data["clientOrderId"],
            price=float(data["price"]),
            orig_qty=float(data["origQty"]),
            executed_qty=float(data["executedQty"]),
            status=data["status"],
            type=data["type"],
            side=data["side"],
            transact_time=data["transactTime"]
        )
    
    async def cancel_order(
        self,
        symbol: str,
        order_id: Optional[int] = None,
        client_order_id: Optional[str] = None
    ) -> Dict:
        """Cancel an order"""
        params = {"symbol": symbol.upper()}
        
        if order_id:
            params["orderId"] = order_id
        if client_order_id:
            params["origClientOrderId"] = client_order_id
        
        return await self._make_request("DELETE", "/api/v3/order", params, signed=True, weight=1)
    
    async def get_open_orders(self, symbol: Optional[str] = None) -> List[BinanceOrder]:
        """Get all open orders"""
        params = {}
        if symbol:
            params["symbol"] = symbol.upper()
        
        data = await self._make_request("GET", "/api/v3/openOrders", params, signed=True, weight=1)
        
        return [
            BinanceOrder(
                symbol=o["symbol"],
                order_id=o["orderId"],
                client_order_id=o["clientOrderId"],
                price=float(o["price"]),
                orig_qty=float(o["origQty"]),
                executed_qty=float(o["executedQty"]),
                status=o["status"],
                type=o["type"],
                side=o["side"],
                transact_time=o["time"]
            )
            for o in data
        ]
    
    async def get_trade_history(
        self,
        symbol: str,
        limit: int = 500
    ) -> List[BinanceTrade]:
        """Get trade history for a symbol"""
        data = await self._make_request(
            "GET",
            "/api/v3/myTrades",
            params={"symbol": symbol.upper(), "limit": limit},
            signed=True,
            weight=5
        )
        
        return [
            BinanceTrade(
                id=t["id"],
                symbol=t["symbol"],
                price=float(t["price"]),
                qty=float(t["qty"]),
                commission=float(t["commission"]),
                commission_asset=t["commissionAsset"],
                time=t["time"],
                is_buyer=t["isBuyer"]
            )
            for t in data
        ]
    
    # ==================== USDT-Margined Futures ====================
    
    async def get_futures_ticker(self, symbol: str) -> Dict:
        """Get futures 24hr ticker"""
        data = await self._make_request(
            "GET",
            "/fapi/v1/ticker/24hr",
            params={"symbol": symbol.upper()},
            weight=1
        )
        
        return {
            "symbol": data["symbol"],
            "last_price": float(data["lastPrice"]),
            "mark_price": float(data.get("markPrice", data["lastPrice"])),
            "index_price": float(data.get("indexPrice", data["lastPrice"])),
            "funding_rate": float(data.get("fundingRate", 0)),
            "next_funding_time": data.get("nextFundingTime"),
            "open_price": float(data["openPrice"]),
            "high_price": float(data["highPrice"]),
            "low_price": float(data["lowPrice"]),
            "volume": float(data["volume"]),
            "quote_volume": float(data["quoteVolume"])
        }
    
    async def get_futures_position(self, symbol: Optional[str] = None) -> List[Dict]:
        """Get futures positions"""
        params = {}
        if symbol:
            params["symbol"] = symbol.upper()
        
        data = await self._make_request(
            "GET",
            "/fapi/v2/positionRisk",
            params=params,
            signed=True,
            weight=5
        )
        
        return [
            {
                "symbol": p["symbol"],
                "position_side": p["positionSide"],
                "position_amt": float(p["positionAmt"]),
                "entry_price": float(p["entryPrice"]),
                "mark_price": float(p["markPrice"]),
                "unrealized_pnl": float(p["unRealizedProfit"]),
                "margin": float(p["isolatedMargin"]),
                "leverage": int(p["leverage"]),
                "liquidation_price": float(p.get("liquidationPrice", 0))
            }
            for p in data
            if float(p["positionAmt"]) != 0
        ]
    
    async def create_futures_order(
        self,
        symbol: str,
        side: BinanceOrderSide,
        order_type: BinanceOrderType,
        quantity: float,
        price: Optional[float] = None,
        stop_price: Optional[float] = None,
        position_side: str = "BOTH"  # BOTH, LONG, SHORT
    ) -> Dict:
        """Create a futures order"""
        params = {
            "symbol": symbol.upper(),
            "side": side.value,
            "positionSide": position_side,
            "type": order_type.value,
            "quantity": str(quantity)
        }
        
        if price:
            params["price"] = str(price)
        
        if stop_price:
            params["stopPrice"] = str(stop_price)
        
        return await self._make_request("POST", "/fapi/v1/order", params, signed=True, weight=1)
    
    # ==================== WebSocket Streams ====================
    
    def get_websocket_url(self, streams: List[str]) -> str:
        """Get combined websocket URL"""
        stream_params = "/".join(streams)
        if self.config.testnet:
            return f"wss://testnet.binance.vision/ws/{stream_params}"
        return f"wss://stream.binance.com:9443/ws/{stream_params}"
    
    async def start_websocket(
        self,
        streams: List[str],
        handler: callable
    ):
        """Start websocket connection for streams"""
        url = self.get_websocket_url(streams)
        
        async with self.http_client.ws_connect(url) as ws:
            async for msg in ws:
                if msg.type == httpx.WSMsgType.TEXT:
                    data = json.loads(msg.data)
                    await handler(data)
    
    # ==================== Utilities ====================
    
    def normalize_symbol(self, base: str, quote: str) -> str:
        """Convert base/quote to Binance symbol format"""
        return f"{base.upper()}{quote.upper()}"
    
    async def close(self):
        """Close the HTTP client"""
        await self.http_client.aclose()


class BinanceAPIError(Exception):
    """Binance API Error"""
    def __init__(self, code: int, message: str):
        self.code = code
        self.message = message
        super().__init__(f"Binance API Error {code}: {message}")


# ============================================================================
# TigerSwap CEX Integration Layer
# ============================================================================

class CEXIntegrationManager:
    """
    Manages all CEX connections for TigerSwap.
    Provides unified interface for trading across multiple exchanges.
    """
    
    def __init__(self):
        self.connectors: Dict[str, BinanceConnector] = {}
        self.trading_pairs: Dict[str, Dict] = {}  # Unified pair mapping
        
    async def connect_binance(
        self,
        api_key: str,
        api_secret: str,
        testnet: bool = False
    ) -> BinanceConnector:
        """Connect to Binance"""
        config = BinanceConfig(
            api_key=api_key,
            api_secret=api_secret,
            testnet=testnet
        )
        
        connector = BinanceConnector(config)
        
        # Verify connection
        await connector.get_account_info()
        
        self.connectors["binance"] = connector
        return connector
    
    async def execute_cross_exchange_arbitrage(
        self,
        buy_exchange: str,
        sell_exchange: str,
        symbol: str,
        quantity: float
    ) -> Dict:
        """
        Execute arbitrage between exchanges.
        Buy on one exchange, sell on another.
        """
        if buy_exchange not in self.connectors or sell_exchange not in self.connectors:
            raise ValueError("Exchange not connected")
        
        buy_conn = self.connectors[buy_exchange]
        sell_conn = self.connectors[sell_exchange]
        
        # Get prices
        buy_ticker = await buy_conn.get_ticker(symbol)
        sell_ticker = await sell_conn.get_ticker(symbol)
        
        buy_price = buy_ticker.last_price
        sell_price = sell_ticker.last_price
        
        # Calculate profit
        buy_cost = buy_price * quantity
        sell_revenue = sell_price * quantity
        profit = sell_revenue - buy_cost
        profit_pct = (profit / buy_cost) * 100 if buy_cost > 0 else 0
        
        return {
            "buy_exchange": buy_exchange,
            "sell_exchange": sell_exchange,
            "symbol": symbol,
            "quantity": quantity,
            "buy_price": buy_price,
            "sell_price": sell_price,
            "buy_cost": buy_cost,
            "sell_revenue": sell_revenue,
            "profit": profit,
            "profit_pct": profit_pct,
            "timestamp": int(time.time())
        }
    
    async def get_best_price(
        self,
        symbol: str,
        side: str,
        quantity: float
    ) -> Dict:
        """Find best price across all connected exchanges"""
        best_price = 0
        best_exchange = None
        second_best = 0
        
        for exchange_name, connector in self.connectors.items():
            try:
                ticker = await connector.get_ticker(symbol)
                price = ticker.last_price
                
                if side.upper() == "BUY":
                    # Lower price is better for buying
                    if price < best_price or best_price == 0:
                        second_best = best_price
                        best_price = price
                        best_exchange = exchange_name
                else:
                    # Higher price is better for selling
                    if price > best_price:
                        second_best = best_price
                        best_price = price
                        best_exchange = exchange_name
                        
            except Exception as e:
                print(f"Error getting price from {exchange_name}: {e}")
        
        return {
            "symbol": symbol,
            "side": side,
            "quantity": quantity,
            "best_price": best_price,
            "best_exchange": best_exchange,
            "second_best_price": second_best,
            "improvement_bps": ((best_price - second_best) / second_best * 10000) if second_best > 0 else 0
        }


async def main():
    """Test Binance connector"""
    print("=== Binance Connector Test ===")
    
    # Note: In production, use real API keys
    # For testing, you would use testnet credentials
    
    config = BinanceConfig(
        api_key="your_api_key",
        api_secret="your_api_secret",
        testnet=True
    )
    
    connector = BinanceConnector(config)
    
    # Get ticker
    try:
        ticker = await connector.get_ticker("ETHUSDT")
        print(f"\nETH/USDT Ticker:")
        print(f"  Price: ${ticker.last_price:,.2f}")
        print(f"  24h Change: {ticker.price_change_percent:+.2f}%")
        print(f"  24h Volume: ${ticker.quote_volume:,.0f}")
    except Exception as e:
        print(f"Error: {e}")
    
    # Get order book
    try:
        book = await connector.get_order_book("ETHUSDT", limit=5)
        print(f"\nETH/USDT Order Book:")
        print(f"  Bids: {book['bids'][:3]}")
        print(f"  Asks: {book['asks'][:3]}")
    except Exception as e:
        print(f"Error: {e}")
    
    await connector.close()


if __name__ == "__main__":
    asyncio.run(main())
