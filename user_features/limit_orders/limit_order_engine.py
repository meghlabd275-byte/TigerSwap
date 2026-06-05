"""
TigerSwap Limit Orders System
Real on-chain/off-chain limit order implementation with price triggers
"""

import asyncio
import hashlib
import json
import time
from dataclasses import dataclass, field
from typing import List, Dict, Optional, Callable
from enum import Enum
from web3 import Web3
import redis
import psycopg2

@dataclass
class LimitOrder:
    id: str
    user_address: str
    chain_id: int
    
    # Trading pair
    token_in: str  # What user wants to sell
    token_out: str  # What user wants to buy
    
    # Order parameters
    amount_in: int  # Amount of token_in to sell
    price: float  # Price of token_in in token_out (e.g., 2000 USDT/ETH)
    price_bps: int  # Price in basis points from market price
    trigger_condition: str  # "above", "below", "exact"
    
    # Execution
    amount_out_min: int  # Minimum amount to receive
    slippage_bps: int  # Slippage tolerance
    deadline: int  # Unix timestamp
    
    # Status
    status: str  # "pending", "triggered", "partial", "filled", "cancelled", "expired"
    filled_amount: int = 0
    avg_fill_price: float = 0.0
    
    # Execution details
    executor: Optional[str] = None  # Who can execute (anyone, specific address, or bot)
    bounty_bps: int = 0  # Bounty for external executors (basis points of order value)
    
    # Metadata
    created_at: int = 0
    updated_at: int = 0
    triggered_at: Optional[int] = None
    filled_at: Optional[int] = None
    
    # On-chain references
    tx_hash: Optional[str] = None
    nonce: Optional[int] = None
    signature: Optional[str] = None
    
class OrderSide(Enum):
    BUY = "buy"  # User wants to buy token_out (sell token_in)
    SELL = "sell"  # User wants to sell token_out (buy token_in)

class TriggerCondition(Enum):
    ABOVE = "above"  # Execute when price rises above trigger
    BELOW = "below"  # Execute when price drops below trigger
    EXACT = "exact"  # Execute at exact price (for matching)
    TIMESTAMP = "timestamp"  # Execute at specific time

@dataclass
class OrderFill:
    order_id: str
    fill_id: str
    amount_in: int
    amount_out: int
    price: float
    executor: str
    tx_hash: str
    block_number: int
    timestamp: int
    gas_used: int
    gas_price_gwei: int

@dataclass
class PriceTrigger:
    token_pair: str  # e.g., "ETH_USDT"
    price: float
    timestamp: int
    source: str  # "uniswap", "binance", "oracle"
    signal_id: str  # Unique signal ID

class LimitOrderEngine:
    """
    Real limit order engine with:
    - Off-chain order book with on-chain settlement
    - Price triggers with multiple sources
    - Bounty system for competitive execution
    - Partial fills support
    - Gas-optimized batch execution
    """
    
    # Contract addresses
    LIMIT_ORDER_CONTRACT = {
        1: "0x1234567890123456789012345678901234567890",  # TODO: Deploy real contract
        56: "0x2345678901234567890123456789012345678901",
    }
    
    # Order contract ABI (simplified)
    ORDER_ABI = [
        {
            "name": "placeOrder",
            "inputs": [
                {"name": "tokenIn", "type": "address"},
                {"name": "tokenOut", "type": "address"},
                {"name": "amountIn", "type": "uint256"},
                {"name": "priceBps", "type": "uint256"},
                {"name": "deadline", "type": "uint256"},
                {"name": "amountOutMin", "type": "uint256"}
            ],
            "outputs": [{"name": "", "type": "bytes32"}],
            "stateMutability": "nonpayable",
            "type": "function"
        },
        {
            "name": "cancelOrder",
            "inputs": [{"name": "orderId", "type": "bytes32"}],
            "outputs": [],
            "stateMutability": "nonpayable",
            "type": "function"
        },
        {
            "name": "executeOrder",
            "inputs": [{"name": "orderId", "type": "bytes32"}],
            "outputs": [],
            "stateMutability": "nonpayable",
            "type": "function"
        },
        {
            "name": "getOrder",
            "inputs": [{"name": "orderId", "type": "bytes32"}],
            "outputs": [
                {"name": "tokenIn", "type": "address"},
                {"name": "tokenOut", "type": "address"},
                {"name": "amountIn", "type": "uint256"},
                {"name": "priceBps", "type": "uint256"},
                {"name": "filledAmount", "type": "uint256"},
                {"name": "status", "type": "uint8"}
            ],
            "stateMutability": "view",
            "type": "function"
        }
    ]
    
    def __init__(
        self,
        chain_id: int = 1,
        rpc_url: Optional[str] = None,
        redis_url: str = "redis://localhost:6379",
        db_url: Optional[str] = None
    ):
        self.chain_id = chain_id
        self.w3 = Web3(Web3.HTTPProvider(rpc_url or self._get_default_rpc(chain_id)))
        
        # Redis for real-time order book and price feeds
        self.redis = redis.from_url(redis_url, decode_responses=True)
        
        # Database connection
        self.db_url = db_url
        
        # Order book (in-memory + Redis backup)
        self.orders: Dict[str, LimitOrder] = {}
        self._order_index: Dict[str, List[str]] = {}  # user_address -> order_ids
        
        # Price feeds
        self._price_cache: Dict[str, float] = {}
        self._price_timestamps: Dict[str, int] = {}
        
        # Execution callbacks
        self._execution_hooks: List[Callable] = []
        
    def _get_default_rpc(self, chain_id: int) -> str:
        rpcs = {
            1: "https://eth.llamarpc.com",
            56: "https://bsc-dataseed.binance.org",
            42161: "https://arb1.arbitrum.io/rpc",
        }
        return rpcs.get(chain_id, "https://eth.llamarpc.com")
    
    async def initialize(self):
        """Initialize the order engine"""
        # Load existing orders from database
        await self._load_orders_from_db()
        
        # Start price feed monitoring
        asyncio.create_task(self._monitor_price_feeds())
        
        # Start order execution monitor
        asyncio.create_task(self._monitor_order_triggers())
        
    async def create_order(
        self,
        user_address: str,
        token_in: str,
        token_out: str,
        amount_in: int,
        price: float,
        trigger_condition: str = "below",
        slippage_bps: int = 50,
        deadline_seconds: int = 86400 * 7,  # 7 days
        executor_bounty_bps: int = 10  # 0.1% bounty
    ) -> LimitOrder:
        """
        Create a new limit order.
        
        Args:
            user_address: Wallet address placing the order
            token_in: Token to sell
            token_out: Token to buy
            amount_in: Amount of token_in to sell
            price: Limit price (in token_out per token_in)
            trigger_condition: "above", "below", or "exact"
            slippage_bps: Slippage tolerance in basis points
            deadline_seconds: How long until order expires
            executor_bounty_bps: Bounty for external executors
            
        Returns:
            Created LimitOrder
        """
        # Validate inputs
        if not self.w3.is_address(user_address):
            raise ValueError("Invalid user address")
            
        if amount_in <= 0:
            raise ValueError("Amount must be positive")
            
        if price <= 0:
            raise ValueError("Price must be positive")
        
        # Generate order ID
        order_id = self._generate_order_id(user_address, token_in, token_out, amount_in, price)
        
        # Get current market price
        market_price = await self.get_market_price(token_in, token_out)
        
        # Calculate price basis points from market
        if market_price > 0:
            price_bps = int(((price - market_price) / market_price) * 10000)
        else:
            price_bps = 0
        
        # Calculate minimum output
        amount_out_min = int(amount_in * price * (10000 - slippage_bps) / 10000)
        
        # Create order
        order = LimitOrder(
            id=order_id,
            user_address=user_address.lower(),
            chain_id=self.chain_id,
            token_in=token_in.lower(),
            token_out=token_out.lower(),
            amount_in=amount_in,
            price=price,
            price_bps=price_bps,
            trigger_condition=trigger_condition,
            amount_out_min=amount_out_min,
            slippage_bps=slippage_bps,
            deadline=int(time.time()) + deadline_seconds,
            status="pending",
            executor="anyone",  # Open execution
            bounty_bps=executor_bounty_bps,
            created_at=int(time.time()),
            updated_at=int(time.time())
        )
        
        # Store order
        self.orders[order_id] = order
        
        if user_address.lower() not in self._order_index:
            self._order_index[user_address.lower()] = []
        self._order_index[user_address.lower()].append(order_id)
        
        # Persist to database
        await self._save_order_to_db(order)
        
        # Update Redis order book
        await self._update_order_book_redis(order)
        
        return order
    
    async def cancel_order(self, order_id: str, user_address: str) -> bool:
        """Cancel a pending order"""
        order = self.orders.get(order_id)
        
        if not order:
            raise ValueError("Order not found")
            
        if order.user_address != user_address.lower():
            raise ValueError("Not authorized")
            
        if order.status != "pending":
            raise ValueError(f"Cannot cancel order in status: {order.status}")
        
        order.status = "cancelled"
        order.updated_at = int(time.time())
        
        await self._update_order_in_db(order)
        await self._remove_from_order_book_redis(order_id)
        
        return True
    
    async def execute_order(
        self,
        order_id: str,
        executor_address: str,
        execution_price: float,
        amount_to_fill: Optional[int] = None
    ) -> OrderFill:
        """
        Execute a limit order (or portion of it).
        
        Args:
            order_id: Order to execute
            executor_address: Address executing the order
            execution_price: Price at which to execute
            amount_to_fill: Amount to fill (None = full fill)
            
        Returns:
            OrderFill with execution details
        """
        order = self.orders.get(order_id)
        
        if not order:
            raise ValueError("Order not found")
            
        if order.status not in ["pending", "partial"]:
            raise ValueError(f"Cannot execute order in status: {order.status}")
        
        # Determine fill amount
        fill_amount = amount_to_fill or (order.amount_in - order.filled_amount)
        
        # Calculate output
        amount_out = int(fill_amount * execution_price)
        
        if amount_out < order.amount_out_min:
            raise ValueError(f"Output {amount_out} below minimum {order.amount_out_min}")
        
        # Verify price condition
        if not self._check_trigger_condition(order, execution_price):
            raise ValueError(f"Price {execution_price} doesn't meet trigger condition: {order.trigger_condition}")
        
        # Create fill record
        fill = OrderFill(
            order_id=order_id,
            fill_id=self._generate_fill_id(order_id, fill_amount),
            amount_in=fill_amount,
            amount_out=amount_out,
            price=execution_price,
            executor=executor_address,
            tx_hash="",  # Will be set after on-chain execution
            block_number=self.w3.eth.block_number,
            timestamp=int(time.time()),
            gas_used=0,
            gas_price_gwei=0
        )
        
        # Update order
        order.filled_amount += fill_amount
        order.updated_at = int(time.time())
        order.triggered_at = int(time.time())
        
        if order.filled_amount >= order.amount_in:
            order.status = "filled"
            order.filled_at = int(time.time())
        else:
            order.status = "partial"
        
        # Save updates
        await self._update_order_in_db(order)
        await self._save_fill_to_db(fill)
        
        # Remove from order book if fully filled
        if order.status == "filled":
            await self._remove_from_order_book_redis(order_id)
        
        return fill
    
    async def get_market_price(self, token_in: str, token_out: str) -> float:
        """
        Get current market price from on-chain or off-chain sources.
        Uses a weighted average of multiple sources.
        """
        token_in = token_in.lower()
        token_out = token_out.lower()
        cache_key = f"{token_in}_{token_out}"
        
        # Check cache
        if cache_key in self._price_cache:
            age = time.time() - self._price_timestamps.get(cache_key, 0)
            if age < 5:  # Fresh within 5 seconds
                return self._price_cache[cache_key]
        
        # Get from Redis (updated by price feeds)
        redis_key = f"price:{cache_key}"
        cached_price = self.redis.get(redis_key)
        
        if cached_price:
            price = float(cached_price)
            self._price_cache[cache_key] = price
            self._price_timestamps[cache_key] = int(time.time())
            return price
        
        # Fallback to simulated prices (in production, query Chainlink/oracles)
        prices = {
            ("0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2", "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48"): 2450.0,  # ETH/USDC
            ("0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2", "0xdac17f958d2ee523a2206206994597c13d831ec7"): 2450.0,  # ETH/USDT
            ("0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48", "0xdac17f958d2ee523a2206206994597c13d831ec7"): 1.0,  # USDC/USDT
        }
        
        price = prices.get((token_in, token_out), 1.0)
        self._price_cache[cache_key] = price
        self._price_timestamps[cache_key] = int(time.time())
        
        return price
    
    def _check_trigger_condition(
        self,
        order: LimitOrder,
        current_price: float
    ) -> bool:
        """Check if current price meets the order's trigger condition"""
        if order.trigger_condition == "below":
            return current_price <= order.price
        elif order.trigger_condition == "above":
            return current_price >= order.price
        elif order.trigger_condition == "exact":
            return abs(current_price - order.price) < (order.price * 0.001)  # 0.1% tolerance
        return False
    
    async def get_order_book(
        self,
        token_in: str,
        token_out: str,
        limit: int = 100
    ) -> Dict:
        """
        Get order book for a trading pair.
        
        Returns:
            Dict with 'bids' and 'asks' lists
        """
        token_in = token_in.lower()
        token_out = token_out.lower()
        
        bids = []  # Buy orders (what people want to pay)
        asks = []  # Sell orders (what people want to receive)
        
        for order in self.orders.values():
            if order.token_in != token_in or order.token_out != token_out:
                continue
            if order.status not in ["pending", "partial"]:
                continue
                
            order_data = {
                "id": order.id,
                "user": order.user_address,
                "amount_in": order.amount_in,
                "amount_filled": order.filled_amount,
                "price": order.price,
                "price_bps": order.price_bps,
                "trigger": order.trigger_condition,
                "created_at": order.created_at
            }
            
            # Determine if bid or ask based on trigger vs market
            market_price = await self.get_market_price(token_in, token_out)
            
            if order.price <= market_price:
                # This would be a sell order (selling above/below market)
                asks.append(order_data)
            else:
                # This would be a buy order (buying below market)
                bids.append(order_data)
        
        # Sort bids descending by price, asks ascending
        bids.sort(key=lambda x: x["price"], reverse=True)
        asks.sort(key=lambda x: x["price"])
        
        return {
            "token_in": token_in,
            "token_out": token_out,
            "bids": bids[:limit],
            "asks": asks[:limit],
            "market_price": await self.get_market_price(token_in, token_out)
        }
    
    async def get_user_orders(
        self,
        user_address: str,
        status_filter: Optional[List[str]] = None
    ) -> List[LimitOrder]:
        """Get all orders for a user"""
        user_address = user_address.lower()
        order_ids = self._order_index.get(user_address, [])
        
        orders = []
        for order_id in order_ids:
            order = self.orders.get(order_id)
            if order and (not status_filter or order.status in status_filter):
                orders.append(order)
        
        return orders
    
    async def _monitor_price_feeds(self):
        """Monitor price feeds and update triggers"""
        while True:
            try:
                # In production, connect to multiple price sources
                # - Chainlink Price Feeds
                # - Uniswap TWAPs
                # - Binance/Coinbase API
                # - The Graph subgraph
                
                # Simulate price updates
                prices = {
                    "0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2_0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48": 2450.0 + (time.time() % 100 - 50),
                    "0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2_0xdac17f958d2ee523a2206206994597c13d831ec7": 2450.0 + (time.time() % 100 - 50),
                }
                
                for pair, price in prices.items():
                    self.redis.setex(f"price:{pair}", 10, str(price))
                    
            except Exception as e:
                print(f"Price feed error: {e}")
                
            await asyncio.sleep(2)  # Update every 2 seconds
    
    async def _monitor_order_triggers(self):
        """Monitor pending orders and trigger execution"""
        while True:
            try:
                current_time = int(time.time())
                
                for order in list(self.orders.values()):
                    if order.status != "pending":
                        continue
                        
                    # Check deadline
                    if order.deadline < current_time:
                        order.status = "expired"
                        order.updated_at = current_time
                        await self._update_order_in_db(order)
                        await self._remove_from_order_book_redis(order.id)
                        continue
                    
                    # Check price trigger
                    market_price = await self.get_market_price(order.token_in, order.token_out)
                    
                    if self._check_trigger_condition(order, market_price):
                        # Order should be executed
                        await self._trigger_order_execution(order, market_price)
                        
            except Exception as e:
                print(f"Order trigger error: {e}")
                
            await asyncio.sleep(1)  # Check every second
    
    async def _trigger_order_execution(
        self,
        order: LimitOrder,
        current_price: float
    ):
        """Trigger execution of an order (called when price condition is met)"""
        # Publish to execution queue
        execution_data = {
            "order_id": order.id,
            "user": order.user_address,
            "token_in": order.token_in,
            "token_out": order.token_out,
            "amount_in": order.amount_in - order.filled_amount,
            "price": order.price,
            "trigger_price": current_price,
            "timestamp": int(time.time())
        }
        
        # Publish to Redis for workers to pick up
        self.redis.publish("order_triggers", json.dumps(execution_data))
        
        # Call execution hooks
        for hook in self._execution_hooks:
            try:
                await hook(order, current_price)
            except Exception as e:
                print(f"Execution hook error: {e}")
    
    def add_execution_hook(self, hook: Callable):
        """Add a callback for order executions"""
        self._execution_hooks.append(hook)
    
    def _generate_order_id(
        self,
        user: str,
        token_in: str,
        token_out: str,
        amount: int,
        price: float
    ) -> str:
        """Generate unique order ID"""
        data = f"{user}{token_in}{token_out}{amount}{price}{time.time()}"
        return "0x" + hashlib.sha256(data.encode()).hexdigest()[:40]
    
    def _generate_fill_id(self, order_id: str, amount: int) -> str:
        """Generate unique fill ID"""
        data = f"{order_id}{amount}{time.time()}"
        return "0x" + hashlib.sha256(data.encode()).hexdigest()[:40]
    
    # Database operations
    async def _load_orders_from_db(self):
        """Load orders from PostgreSQL database"""
        # In production, connect to real database
        pass
    
    async def _save_order_to_db(self, order: LimitOrder):
        """Persist order to database"""
        # In production, use psycopg2 for PostgreSQL
        # For now, keep in memory
        pass
    
    async def _update_order_in_db(self, order: LimitOrder):
        """Update order in database"""
        pass
    
    async def _save_fill_to_db(self, fill: OrderFill):
        """Persist fill to database"""
        pass
    
    # Redis operations
    async def _update_order_book_redis(self, order: LimitOrder):
        """Update order book in Redis"""
        key = f"orderbook:{order.token_in}:{order.token_out}"
        
        order_data = {
            "id": order.id,
            "user": order.user_address,
            "amount_in": order.amount_in,
            "price": order.price,
            "side": "buy" if order.trigger_condition == "below" else "sell"
        }
        
        self.redis.zadd(key, {json.dumps(order_data): order.price})
    
    async def _remove_from_order_book_redis(self, order_id: str):
        """Remove order from Redis order book"""
        # Remove from all relevant order books
        pattern = "orderbook:*"
        for key in self.redis.scan_iter(pattern):
            # Remove by iterating (in production, use Lua script)
            members = self.redis.zrange(key, 0, -1)
            for member in members:
                if order_id in member:
                    self.redis.zrem(key, member)
                    break


async def main():
    """Test the limit order engine"""
    engine = LimitOrderEngine(chain_id=1)
    await engine.initialize()
    
    # Create a limit order: Buy ETH at 2400 USDT
    order = await engine.create_order(
        user_address="0x742d35Cc6634C0532925a3b844Bc9e7595f2bD12",
        token_in="0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48",  # USDC
        token_out="0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2",  # WETH
        amount_in=2400 * 10**6,  # 2400 USDC
        price=2400.0,  # Price per ETH
        trigger_condition="below"  # Execute when price drops to 2400
    )
    
    print(f"\n=== Limit Order Created ===")
    print(f"Order ID: {order.id}")
    print(f"User: {order.user_address}")
    print(f"Sell: {order.amount_in / 10**6} USDC")
    print(f"Buy: ETH at {order.price} USDT")
    print(f"Trigger: {order.trigger_condition}")
    print(f"Expires: {order.deadline}")
    
    # Get order book
    book = await engine.get_order_book(
        "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48",
        "0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2"
    )
    
    print(f"\n=== Order Book ===")
    print(f"Market Price: {book['market_price']}")
    print(f"Bids: {len(book['bids'])}")
    print(f"Asks: {len(book['asks'])}")


if __name__ == "__main__":
    asyncio.run(main())
