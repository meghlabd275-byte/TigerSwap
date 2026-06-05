"""
TigerSwap Real-Time Trading Service
WebSocket-based price feeds, order book, and trade updates
"""

import asyncio
import json
import time
import hashlib
from dataclasses import dataclass, field
from typing import Dict, List, Optional, Set, Callable
from enum import Enum
from autobahn.asyncio.websocket import WebSocketServerProtocol, WebSocketServerFactory
from threading import Thread
import redis.asyncio as redis

@dataclass
class PriceTick:
    pair: str  # e.g., "ETH_USDC"
    price: float
    volume_24h: float
    change_24h: float
    high_24h: float
    low_24h: float
    timestamp: int
    source: str  # "uniswap", "binance", "aggregated"

@dataclass
class OrderBookLevel:
    price: float
    amount: float
    total: float  # Cumulative amount

@dataclass
class OrderBook:
    pair: str
    bids: List[OrderBookLevel]  # Buy orders (sorted desc by price)
    asks: List[OrderBookLevel]  # Sell orders (sorted asc by price)
    spread: float
    spread_bps: float
    timestamp: int
    last_update_id: int

@dataclass
class Trade:
    id: str
    pair: str
    side: str  # "buy" or "sell"
    price: float
    amount: float
    timestamp: int
    tx_hash: str
    dex: str

@dataclass
class Ticker:
    pair: str
    price: float
    volume_24h: float
    change_24h: float
    high_24h: float
    low_24h: float
    bid: float
    ask: float
    last_update: int

class SubscriptionType(Enum):
    TRADES = "trades"
    TICKER = "ticker"
    ORDERBOOK = "orderbook"
    PRICE = "price"

@dataclass
class WSClient:
    client_id: str
    subscriptions: Set[str] = field(default_factory=set)  # "pair:channel"
    subscribed_pairs: Set[str] = field(default_factory=set)
    protocol: Optional['TradingWebSocketProtocol'] = None

class TradingWebSocketProtocol(WebSocketServerProtocol):
    """
    WebSocket protocol for TigerSwap real-time trading data.
    Handles subscriptions and streaming of:
    - Price ticks
    - Order book updates
    - Trade executions
    - Ticker data
    """
    
    MAX_SUBSCRIPTIONS = 50
    PING_INTERVAL = 30
    
    def __init__(self):
        super().__init__()
        self.client_id = None
        self.subscriptions: Set[str] = set()
        self.subscribed_pairs: Set[str] = set()
        
    def onConnect(self, request):
        """Handle new WebSocket connection"""
        self.client_id = self.factory.generate_client_id()
        print(f"Client {self.client_id} connecting from {request.peer}")
        
        # Register with factory
        self.factory.register_client(self)
        
    def onOpen(self):
        """WebSocket connection opened"""
        print(f"Client {self.client_id} connection opened")
        
        # Send welcome message
        self.sendMessage(json.dumps({
            "type": "welcome",
            "client_id": self.client_id,
            "server_time": int(time.time()),
            "subscriptions": []
        }).encode())
        
    def onMessage(self, payload, isBinary):
        """Handle incoming messages"""
        if isBinary:
            return
            
        try:
            message = json.loads(payload.decode())
            self._handle_message(message)
        except Exception as e:
            print(f"Error handling message: {e}")
            self.sendMessage(json.dumps({
                "type": "error",
                "message": str(e)
            }).encode())
    
    def _handle_message(self, message: dict):
        """Process subscription/unsubscription messages"""
        action = message.get("action")
        
        if action == "subscribe":
            self._handle_subscribe(message)
        elif action == "unsubscribe":
            self._handle_unsubscribe(message)
        elif action == "ping":
            self.sendMessage(json.dumps({"type": "pong", "timestamp": int(time.time())}).encode())
        elif action == "get_orderbook":
            self._handle_get_orderbook(message)
        elif action == "get_ticker":
            self._handle_get_ticker(message)
        elif action == "get_recent_trades":
            self._handle_get_recent_trades(message)
        else:
            self.sendMessage(json.dumps({
                "type": "error",
                "message": f"Unknown action: {action}"
            }).encode())
    
    def _handle_subscribe(self, message: dict):
        """Handle subscription request"""
        channels = message.get("channels", [])
        pair = message.get("pair", "")
        
        # Check subscription limit
        if len(self.subscriptions) + len(channels) > self.MAX_SUBSCRIPTIONS:
            self.sendMessage(json.dumps({
                "type": "error",
                "message": f"Too many subscriptions. Max: {self.MAX_SUBSCRIPTIONS}"
            }).encode())
            return
        
        subscribed = []
        for channel in channels:
            if channel not in [s.value for s in SubscriptionType]:
                continue
                
            sub_key = f"{pair}:{channel}"
            self.subscriptions.add(sub_key)
            self.subscribed_pairs.add(pair)
            subscribed.append(sub_key)
            
            # Add to global subscription index
            self.factory.add_subscription(self.client_id, sub_key)
        
        # Confirm subscription
        self.sendMessage(json.dumps({
            "type": "subscribed",
            "channels": subscribed,
            "pair": pair
        }).encode())
        
        # Send initial data
        for sub_key in subscribed:
            _, channel = sub_key.split(":")
            if channel == "orderbook":
                self._send_orderbook_snapshot(pair)
            elif channel == "ticker":
                self._send_ticker(pair)
            elif channel == "trades":
                self._send_recent_trades(pair)
    
    def _handle_unsubscribe(self, message: dict):
        """Handle unsubscription request"""
        channels = message.get("channels", [])
        pair = message.get("pair", "")
        
        unsubscribed = []
        for channel in channels:
            sub_key = f"{pair}:{channel}"
            if sub_key in self.subscriptions:
                self.subscriptions.remove(sub_key)
                self.subscribed_pairs.discard(pair)
                unsubscribed.append(sub_key)
                
                # Remove from global index
                self.factory.remove_subscription(self.client_id, sub_key)
        
        self.sendMessage(json.dumps({
            "type": "unsubscribed",
            "channels": unsubscribed,
            "pair": pair
        }).encode())
    
    def _handle_get_orderbook(self, message: dict):
        """Handle orderbook snapshot request"""
        pair = message.get("pair")
        depth = message.get("depth", 20)
        
        orderbook = self.factory.get_orderbook(pair, depth)
        if orderbook:
            self.sendMessage(json.dumps({
                "type": "orderbook_snapshot",
                "pair": pair,
                "data": self._orderbook_to_dict(orderbook)
            }).encode())
    
    def _handle_get_ticker(self, message: dict):
        """Handle ticker request"""
        pair = message.get("pair")
        ticker = self.factory.get_ticker(pair)
        if ticker:
            self.sendMessage(json.dumps({
                "type": "ticker",
                "pair": pair,
                "data": self._ticker_to_dict(ticker)
            }).encode())
    
    def _handle_get_recent_trades(self, message: dict):
        """Handle recent trades request"""
        pair = message.get("pair")
        limit = message.get("limit", 50)
        
        trades = self.factory.get_recent_trades(pair, limit)
        self.sendMessage(json.dumps({
            "type": "recent_trades",
            "pair": pair,
            "trades": [self._trade_to_dict(t) for t in trades]
        }).encode())
    
    def _send_orderbook_snapshot(self, pair: str):
        """Send orderbook snapshot"""
        orderbook = self.factory.get_orderbook(pair, 20)
        if orderbook:
            self.sendMessage(json.dumps({
                "type": "orderbook_snapshot",
                "pair": pair,
                "data": self._orderbook_to_dict(orderbook)
            }).encode())
    
    def _send_ticker(self, pair: str):
        """Send ticker update"""
        ticker = self.factory.get_ticker(pair)
        if ticker:
            self.sendMessage(json.dumps({
                "type": "ticker",
                "pair": pair,
                "data": self._ticker_to_dict(ticker)
            }).encode())
    
    def _send_recent_trades(self, pair: str):
        """Send recent trades"""
        trades = self.factory.get_recent_trades(pair, 20)
        self.sendMessage(json.dumps({
            "type": "recent_trades",
            "pair": pair,
            "trades": [self._trade_to_dict(t) for t in trades]
        }).encode())
    
    def _orderbook_to_dict(self, ob: OrderBook) -> dict:
        return {
            "pair": ob.pair,
            "bids": [{"price": b.price, "amount": b.amount, "total": b.total} for b in ob.bids],
            "asks": [{"price": a.price, "amount": a.amount, "total": a.total} for a in ob.asks],
            "spread": ob.spread,
            "spread_bps": ob.spread_bps,
            "timestamp": ob.timestamp
        }
    
    def _ticker_to_dict(self, t: Ticker) -> dict:
        return {
            "pair": t.pair,
            "price": t.price,
            "volume_24h": t.volume_24h,
            "change_24h": t.change_24h,
            "high_24h": t.high_24h,
            "low_24h": t.low_24h,
            "bid": t.bid,
            "ask": t.ask,
            "last_update": t.last_update
        }
    
    def _trade_to_dict(self, t: Trade) -> dict:
        return {
            "id": t.id,
            "pair": t.pair,
            "side": t.side,
            "price": t.price,
            "amount": t.amount,
            "timestamp": t.timestamp,
            "tx_hash": t.tx_hash,
            "dex": t.dex
        }
    
    def onClose(self, wasClean, code, reason):
        """Handle WebSocket close"""
        print(f"Client {self.client_id} disconnected: {reason}")
        self.factory.unregister_client(self)
    
    def onError(self, reason):
        """Handle WebSocket error"""
        print(f"Client {self.client_id} error: {reason}")


class TradingWebSocketFactory(WebSocketServerFactory):
    """
    WebSocket factory managing all trading data streams.
    Broadcasts price updates, order book changes, and trades.
    """
    
    def __init__(self, host: str = "0.0.0.0", port: int = 8080):
        super().__init__()
        self.host = host
        self.port = port
        
        # Connected clients
        self.clients: Dict[str, WSClient] = {}
        
        # Global subscription index: "pair:channel" -> set of client_ids
        self.subscription_index: Dict[str, Set[str]] = {}
        
        # Data stores
        self.orderbooks: Dict[str, OrderBook] = {}
        self.tickers: Dict[str, Ticker] = {}
        self.recent_trades: Dict[str, List[Trade]] = {}
        self.price_cache: Dict[str, PriceTick] = {}
        
        # Redis for pub/sub
        self.redis: Optional[redis.Redis] = None
        
        # Update counters
        self.update_counters: Dict[str, int] = {}
        
    def generate_client_id(self) -> str:
        """Generate unique client ID"""
        return hashlib.sha256(str(time.time()).encode()).hexdigest()[:16]
    
    def register_client(self, protocol: TradingWebSocketProtocol):
        """Register new client"""
        client = WSClient(
            client_id=protocol.client_id,
            protocol=protocol
        )
        self.clients[protocol.client_id] = client
    
    def unregister_client(self, protocol: TradingWebSocketProtocol):
        """Unregister client and clean up subscriptions"""
        if protocol.client_id in self.clients:
            client = self.clients[protocol.client_id]
            
            # Remove from subscription index
            for sub_key in client.subscriptions:
                self.remove_subscription(protocol.client_id, sub_key)
            
            del self.clients[protocol.client_id]
    
    def add_subscription(self, client_id: str, sub_key: str):
        """Add subscription to global index"""
        if sub_key not in self.subscription_index:
            self.subscription_index[sub_key] = set()
        self.subscription_index[sub_key].add(client_id)
    
    def remove_subscription(self, client_id: str, sub_key: str):
        """Remove subscription from global index"""
        if sub_key in self.subscription_index:
            self.subscription_index[sub_key].discard(client_id)
            if not self.subscription_index[sub_key]:
                del self.subscription_index[sub_key]
    
    async def initialize(self):
        """Initialize factory with data connectors"""
        # Connect to Redis
        try:
            self.redis = redis.from_url("redis://localhost:6379", decode_responses=True)
            await self.redis.ping()
            print("Connected to Redis")
        except Exception as e:
            print(f"Redis connection failed: {e}")
            self.redis = None
        
        # Initialize with base trading pairs
        self._initialize_pairs()
        
        # Start background tasks
        asyncio.create_task(self._price_update_loop())
        asyncio.create_task(self._orderbook_update_loop())
        
    def _initialize_pairs(self):
        """Initialize data for trading pairs"""
        pairs = [
            "ETH_USDC", "ETH_USDT", "ETH_WBTC",
            "WBTC_USDC", "WBTC_USDT",
            "LINK_ETH", "UNI_ETH", "AAVE_ETH"
        ]
        
        for pair in pairs:
            self.orderbooks[pair] = self._generate_orderbook(pair)
            self.tickers[pair] = self._generate_ticker(pair)
            self.recent_trades[pair] = []
    
    def _generate_orderbook(self, pair: str, depth: int = 20) -> OrderBook:
        """Generate simulated orderbook"""
        # Get base price
        base_price = self._get_base_price(pair)
        
        bids = []
        asks = []
        
        cumulative = 0.0
        for i in range(depth):
            # Bids below market
            bid_price = base_price * (1 - 0.0001 * (i + 1))
            bid_amount = 1.0 + (depth - i) * 0.5
            cumulative += bid_amount
            bids.append(OrderBookLevel(bid_price, bid_amount, cumulative))
            
            # Asks above market
            ask_price = base_price * (1 + 0.0001 * (i + 1))
            ask_amount = 1.0 + (depth - i) * 0.5
            asks.append(OrderBookLevel(ask_price, ask_amount, cumulative))
        
        spread = asks[0].price - bids[0].price
        spread_bps = (spread / base_price) * 10000 if base_price > 0 else 0
        
        return OrderBook(
            pair=pair,
            bids=bids,
            asks=asks,
            spread=spread,
            spread_bps=spread_bps,
            timestamp=int(time.time()),
            last_update_id=0
        )
    
    def _generate_ticker(self, pair: str) -> Ticker:
        """Generate simulated ticker"""
        base_price = self._get_base_price(pair)
        change = (hashlib.md5(pair.encode()).hexdigest()[0:2], int(hashlib.md5(pair.encode()).hexdigest()[0:2], 16) % 20 - 10)
        change_pct = change[1] / 10.0
        
        return Ticker(
            pair=pair,
            price=base_price,
            volume_24h=base_price * 1000000,
            change_24h=change_pct,
            high_24h=base_price * 1.05,
            low_24h=base_price * 0.95,
            bid=base_price * 0.9999,
            ask=base_price * 1.0001,
            last_update=int(time.time())
        )
    
    def _get_base_price(self, pair: str) -> float:
        """Get base price for pair"""
        prices = {
            "ETH_USDC": 2450.0,
            "ETH_USDT": 2450.0,
            "ETH_WBTC": 0.04,
            "WBTC_USDC": 62500.0,
            "WBTC_USDT": 62500.0,
            "LINK_ETH": 0.0075,
            "UNI_ETH": 0.005,
            "AAVE_ETH": 0.12,
        }
        return prices.get(pair, 1.0)
    
    def get_orderbook(self, pair: str, depth: int = 20) -> Optional[OrderBook]:
        """Get orderbook snapshot"""
        if pair not in self.orderbooks:
            return None
        ob = self.orderbooks[pair]
        return OrderBook(
            pair=ob.pair,
            bids=ob.bids[:depth],
            asks=ob.asks[:depth],
            spread=ob.spread,
            spread_bps=ob.spread_bps,
            timestamp=ob.timestamp,
            last_update_id=ob.last_update_id
        )
    
    def get_ticker(self, pair: str) -> Optional[Ticker]:
        """Get ticker for pair"""
        return self.tickers.get(pair)
    
    def get_recent_trades(self, pair: str, limit: int = 50) -> List[Trade]:
        """Get recent trades for pair"""
        return self.recent_trades.get(pair, [])[:limit]
    
    async def _price_update_loop(self):
        """Background loop updating prices"""
        while True:
            try:
                for pair in self.orderbooks.keys():
                    # Simulate price movement
                    base_price = self._get_base_price(pair)
                    volatility = 0.0002  # 0.02% per tick
                    change = (hashlib.md5(str(time.time()).encode()).hexdigest()[0:4], 
                             int(hashlib.md5(str(time.time()).encode()).hexdigest()[0:4], 16) % 1000 - 500)
                    price_change = base_price * volatility * (change[1] / 500)
                    
                    new_price = base_price + price_change
                    
                    # Update ticker
                    ticker = self.tickers[pair]
                    ticker.price = new_price
                    ticker.bid = new_price * 0.9999
                    ticker.ask = new_price * 1.0001
                    ticker.last_update = int(time.time())
                    
                    # Update orderbook
                    self.orderbooks[pair] = self._generate_orderbook(pair)
                    
                    # Broadcast update
                    await self._broadcast_price_update(pair, new_price)
                    
            except Exception as e:
                print(f"Price update error: {e}")
            
            await asyncio.sleep(1)  # Update every second
    
    async def _orderbook_update_loop(self):
        """Background loop updating orderbook"""
        while True:
            try:
                for pair in self.orderbooks.keys():
                    # Update orderbook
                    self.orderbooks[pair] = self._generate_orderbook(pair)
                    self.orderbooks[pair].last_update_id += 1
                    self.orderbooks[pair].timestamp = int(time.time())
                    
                    # Broadcast update
                    await self._broadcast_orderbook_update(pair)
                    
            except Exception as e:
                print(f"Orderbook update error: {e}")
            
            await asyncio.sleep(2)  # Update every 2 seconds
    
    async def _broadcast_price_update(self, pair: str, price: float):
        """Broadcast price update to subscribers"""
        sub_key = f"{pair}:price"
        if sub_key not in self.subscription_index:
            return
            
        message = json.dumps({
            "type": "price_update",
            "pair": pair,
            "price": price,
            "timestamp": int(time.time())
        }).encode()
        
        for client_id in list(self.subscription_index[sub_key]):
            if client_id in self.clients:
                try:
                    self.clients[client_id].protocol.sendMessage(message)
                except Exception:
                    pass  # Client may be disconnected
    
    async def _broadcast_orderbook_update(self, pair: str):
        """Broadcast orderbook update to subscribers"""
        sub_key = f"{pair}:orderbook"
        if sub_key not in self.subscription_index:
            return
        
        ob = self.orderbooks[pair]
        message = json.dumps({
            "type": "orderbook_update",
            "pair": pair,
            "bids": [[b.price, b.amount] for b in ob.bids[:10]],
            "asks": [[a.price, a.amount] for a in ob.asks[:10]],
            "last_update_id": ob.last_update_id,
            "timestamp": ob.timestamp
        }).encode()
        
        for client_id in list(self.subscription_index[sub_key]):
            if client_id in self.clients:
                try:
                    self.clients[client_id].protocol.sendMessage(message)
                except Exception:
                    pass
    
    async def start(self):
        """Start the WebSocket server"""
        self.server = await self._start_server()
        print(f"Trading WebSocket server started on {self.host}:{self.port}")
    
    async def _start_server(self):
        """Internal server start"""
        try:
            from autobahn.asyncio.websocket import WebSocketServerFactory
            
            factory = self
            
            loop = asyncio.get_event_loop()
            server = await loop.create_server(
                lambda: TradingWebSocketProtocol(),
                self.host,
                self.port
            )
            return server
        except ImportError:
            print("autobahn not installed, using mock server")
            return None


class TradingService:
    """
    Main trading service orchestrating all real-time data.
    """
    
    def __init__(self, config: dict = None):
        self.config = config or {}
        self.host = self.config.get("host", "0.0.0.0")
        self.port = self.config.get("port", 8080)
        
        self.ws_factory = TradingWebSocketFactory(self.host, self.port)
        self.running = False
        
    async def start(self):
        """Start the trading service"""
        await self.ws_factory.initialize()
        await self.ws_factory.start()
        self.running = True
        
        print(f"TigerSwap Trading Service started on {self.host}:{self.port}")
        
        # Keep running
        while self.running:
            await asyncio.sleep(1)
    
    async def stop(self):
        """Stop the trading service"""
        self.running = False
        print("TigerSwap Trading Service stopped")
    
    def broadcast_trade(self, pair: str, trade: Trade):
        """Broadcast a new trade to subscribers"""
        # Add to recent trades
        if pair not in self.ws_factory.recent_trades:
            self.ws_factory.recent_trades[pair] = []
        
        self.ws_factory.recent_trades[pair].insert(0, trade)
        self.ws_factory.recent_trades[pair] = self.ws_factory.recent_trades[pair][:100]
        
        # Broadcast
        sub_key = f"{pair}:trades"
        if sub_key in self.ws_factory.subscription_index:
            message = json.dumps({
                "type": "trade",
                "data": {
                    "id": trade.id,
                    "pair": trade.pair,
                    "side": trade.side,
                    "price": trade.price,
                    "amount": trade.amount,
                    "timestamp": trade.timestamp,
                    "tx_hash": trade.tx_hash,
                    "dex": trade.dex
                }
            }).encode()
            
            for client_id in list(self.ws_factory.subscription_index[sub_key]):
                if client_id in self.ws_factory.clients:
                    try:
                        self.ws_factory.clients[client_id].protocol.sendMessage(message)
                    except Exception:
                        pass


async def main():
    """Test the trading service"""
    print("Starting TigerSwap Trading Service...")
    
    service = TradingService({"host": "0.0.0.0", "port": 8080})
    
    try:
        await service.start()
    except KeyboardInterrupt:
        await service.stop()


if __name__ == "__main__":
    asyncio.run(main())
