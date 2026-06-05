"""
TigerSwap Perpetual Trading Order Book
Off-chain order book with on-chain settlement (dYdX/Hyperliquid style)
"""

import asyncio
import hashlib
import heapq
import json
import time
from dataclasses import dataclass, field
from typing import Dict, List, Optional, Tuple, Set
from enum import Enum
from sortedcontainers import SortedDict

@dataclass
class Order:
    """Order in the perpetual trading system"""
    id: str
    user_address: str
    
    # Trading pair
    market: str  # e.g., "ETH-PERP"
    
    # Order side
    side: str  # "buy" or "sell" (long or short for perp)
    
    # Order type
    order_type: str  # "market", "limit", "stop", "stop_limit", "take_profit"
    
    # Price and size
    price: float  # Limit price
    trigger_price: Optional[float] = None  # For stop orders
    size: float  # Position size (in base currency)
    filled_size: float = 0.0
    
    # Execution
    remaining_size: float = 0.0
    avg_fill_price: float = 0.0
    
    # Leverage
    leverage: int = 1  # 1x, 2x, 5x, 10x, etc.
    
    # Reduced only (for modifying orders)
    reduce_only: bool = False  # Can only reduce position
    post_only: bool = False  # Cannot execute immediately
    
    # Time in force
    time_in_force: str = "GTC"  # GTC, IOC, FOK
    
    # Status
    status: str = "open"  # "open", "partial", "filled", "cancelled", "expired"
    
    # Metadata
    created_at: int = 0
    updated_at: int = 0
    expires_at: Optional[int] = None
    
    # Matching
    maker: bool = False  # Is this a maker order?
    tx_hash: Optional[str] = None  # On-chain settlement

@dataclass
class Position:
    """User's perpetual position"""
    id: str
    user_address: str
    market: str
    
    # Position data
    side: str  # "long" or "short"
    size: float  # Current position size
    entry_price: float  # Average entry price
    liquidation_price: float  # Price at which position is liquidated
    
    # Unrealized PnL
    unrealized_pnl: float = 0.0
    unrealized_pnl_pct: float = 0.0
    
    # Funding
    funding_accrued: float = 0.0  # Cumulative funding
    last_funding_time: int = 0
    
    # Margins
    margin: float  # Isolated margin amount
    margin_ratio: float  # margin / position_value
    
    # Status
    status: str = "open"  # "open", "liquidating", "liquidated", "closed"
    
    # History
    created_at: int = 0
    updated_at: int = 0

@dataclass
class OrderBookLevel:
    """Single level in the order book"""
    price: float
    size: float  # Total size at this price
    orders: int  # Number of orders at this level
    order_ids: List[str]  # IDs of orders at this level

@dataclass
class OrderBook:
    """Complete order book for a market"""
    market: str
    bids: Dict[float, OrderBookLevel] = field(default_factory=dict)  # Buy orders
    asks: Dict[float, OrderBookLevel] = field(default_factory=dict)  # Sell orders
    last_update_id: int = 0
    timestamp: int = 0
    
    # Sorted structures for efficient retrieval
    sorted_bids: List[float] = field(default_factory=list)  # Sorted bid prices
    sorted_asks: List[float] = field(default_factory=list)  # Sorted ask prices

@dataclass
class Trade:
    """Executed trade"""
    id: str
    market: str
    
    # Trade sides
    buy_order_id: str
    sell_order_id: str
    buyer: str
    seller: str
    
    # Execution
    price: float
    size: float
    side: str  # "buy" or "sell" (taker's side)
    
    # Fees
    maker_fee: float
    taker_fee: float
    
    # Timing
    timestamp: int
    block_number: Optional[int] = None
    tx_hash: Optional[str] = None

@dataclass
class Market:
    """Perpetual market configuration"""
    name: str  # "ETH-PERP"
    base_currency: str  # "ETH"
    quote_currency: str  # "USD"
    
    # Pricing
    mark_price: float  # Current market price
    index_price: float  # Index price (from oracle)
    last_funding_rate: float = 0.0  # Current funding rate
    
    # Configuration
    initial_margin_fraction: float  # e.g., 0.05 = 5% = 20x leverage max
    maintenance_margin_fraction: float  # e.g., 0.03 = 3%
    
    # Limits
    max_position_size: float  # Max position size
    max_open_interest: float  # Max total open interest
    
    # Fees
    maker_fee: float = -0.0002  # -0.02% (rebate)
    taker_fee: float = 0.0005  # 0.05%
    
    # Funding
    funding_rate: float = 0.0  # Current funding rate
    next_funding_time: int = 0
    
    # Oracle
    oracle_price: float = 0.0  # Price from Chainlink
    
    # Status
    status: str = "open"  # "open", "settling", "halted"
    
    # Volume
    volume_24h: float = 0.0
    high_24h: float = 0.0
    low_24h: float = 0.0
    
    updated_at: int = 0

@dataclass
class FundingPayment:
    """Funding payment record"""
    user: str
    market: str
    payment: float  # Positive = receive, negative = pay
    funding_rate: float
    mark_price: float
    position_size: float
    timestamp: int


class OrderBookEngine:
    """
    Perpetual trading order book with off-chain matching
    and on-chain settlement (similar to dYdX, Hyperliquid).
    """
    
    def __init__(self, chain_id: int = 1):
        self.chain_id = chain_id
        
        # Markets
        self.markets: Dict[str, Market] = {}
        
        # Order books
        self.order_books: Dict[str, OrderBook] = {}
        
        # Orders
        self.orders: Dict[str, Order] = {}
        self.user_orders: Dict[str, Set[str]] = {}  # user -> order_ids
        
        # Positions
        self.positions: Dict[str, Position] = {}  # position_id
        self.user_positions: Dict[str, List[str]] = {}  # user -> position_ids
        
        # Trades
        self.trades: List[Trade] = []
        
        # Price tracking
        self.last_prices: Dict[str, float] = {}
        
        # Funding
        self.funding_payments: List[FundingPayment] = []
        
        # Events
        self.events: List[Dict] = []
        
        # Initialize default markets
        self._initialize_markets()
    
    def _initialize_markets(self):
        """Initialize perpetual markets"""
        markets = [
            Market(
                name="ETH-PERP",
                base_currency="ETH",
                quote_currency="USD",
                mark_price=2450.0,
                index_price=2450.0,
                initial_margin_fraction=0.05,  # 5% = 20x max
                maintenance_margin_fraction=0.03,  # 3%
                max_position_size=1000.0,
                max_open_interest=100_000_000.0,
                next_funding_time=int(time.time()) + 28800  # Every 8 hours
            ),
            Market(
                name="BTC-PERP",
                base_currency="BTC",
                quote_currency="USD",
                mark_price=62500.0,
                index_price=62500.0,
                initial_margin_fraction=0.05,
                maintenance_margin_fraction=0.03,
                max_position_size=500.0,
                max_open_interest=200_000_000.0,
                next_funding_time=int(time.time()) + 28800
            ),
        ]
        
        for market in markets:
            self.markets[market.name] = market
            self.order_books[market.name] = OrderBook(market=market.name)
    
    async def create_order(
        self,
        user_address: str,
        market: str,
        side: str,
        order_type: str,
        size: float,
        price: Optional[float] = None,
        trigger_price: Optional[float] = None,
        leverage: int = 1,
        reduce_only: bool = False,
        post_only: bool = False,
        time_in_force: str = "GTC"
    ) -> Order:
        """Create a new order"""
        if market not in self.markets:
            raise ValueError(f"Market {market} does not exist")
        
        if price is None and order_type in ["limit", "stop_limit"]:
            raise ValueError("Price required for limit orders")
        
        market_config = self.markets[market]
        
        # Validate leverage
        max_leverage = int(1 / market_config.initial_margin_fraction)
        if leverage > max_leverage:
            raise ValueError(f"Leverage {leverage}x exceeds max {max_leverage}x")
        
        # Generate order ID
        order_id = self._generate_order_id(user_address, market, side, size, price)
        
        order = Order(
            id=order_id,
            user_address=user_address.lower(),
            market=market,
            side=side,
            order_type=order_type,
            price=price or 0.0,
            trigger_price=trigger_price,
            size=size,
            filled_size=0.0,
            remaining_size=size,
            leverage=leverage,
            reduce_only=reduce_only,
            post_only=post_only,
            time_in_force=time_in_force,
            created_at=int(time.time()),
            updated_at=int(time.time())
        )
        
        self.orders[order_id] = order
        
        if user_address.lower() not in self.user_orders:
            self.user_orders[user_address.lower()] = set()
        self.user_orders[user_address.lower()].add(order_id)
        
        # Add to order book
        await self._add_to_order_book(order)
        
        # If market order, execute immediately
        if order_type == "market":
            await self._execute_market_order(order)
        
        # If stop order, monitor trigger
        if order_type in ["stop", "stop_limit"]:
            # Trigger monitoring is handled asynchronously
            pass
        
        self._emit_event("OrderCreated", order.__dict__)
        
        return order
    
    async def cancel_order(self, order_id: str, user_address: str) -> bool:
        """Cancel an order"""
        order = self.orders.get(order_id)
        
        if not order:
            raise ValueError("Order not found")
        
        if order.user_address != user_address.lower():
            raise ValueError("Not authorized")
        
        if order.status in ["filled", "cancelled"]:
            raise ValueError(f"Cannot cancel order in status: {order.status}")
        
        # Remove from order book
        await self._remove_from_order_book(order)
        
        order.status = "cancelled"
        order.updated_at = int(time.time())
        
        self._emit_event("OrderCancelled", {"order_id": order_id, "user": user_address})
        
        return True
    
    async def modify_order(
        self,
        order_id: str,
        user_address: str,
        new_price: Optional[float] = None,
        new_size: Optional[float] = None
    ) -> Order:
        """Modify an existing order"""
        order = self.orders.get(order_id)
        
        if not order:
            raise ValueError("Order not found")
        
        if order.user_address != user_address.lower():
            raise ValueError("Not authorized")
        
        if order.status not in ["open", "partial"]:
            raise ValueError(f"Cannot modify order in status: {order.status}")
        
        # Remove from order book at old price
        await self._remove_from_order_book(order)
        
        # Update order
        if new_price is not None:
            order.price = new_price
        if new_size is not None:
            # Can only reduce size
            if new_size < order.remaining_size:
                order.size = order.remaining_size  # Reduce remaining
            else:
                order.size = new_size
        
        order.updated_at = int(time.time())
        
        # Re-add to order book
        await self._add_to_order_book(order)
        
        self._emit_event("OrderModified", {"order_id": order_id, "new_price": new_price, "new_size": new_size})
        
        return order
    
    async def _add_to_order_book(self, order: Order):
        """Add order to the order book"""
        if order.status not in ["open", "partial"]:
            return
        
        book = self.order_books[order.market]
        
        if order.side == "buy":
            if order.price not in book.bids:
                book.bids[order.price] = OrderBookLevel(
                    price=order.price,
                    size=0.0,
                    orders=0,
                    order_ids=[]
                )
                book.sorted_bids.append(order.price)
                book.sorted_bids.sort(reverse=True)  # Descending
            book.bids[order.price].size += order.remaining_size
            book.bids[order.price].orders += 1
            book.bids[order.price].order_ids.append(order.id)
        else:
            if order.price not in book.asks:
                book.asks[order.price] = OrderBookLevel(
                    price=order.price,
                    size=0.0,
                    orders=0,
                    order_ids=[]
                )
                book.asks[order.price].size += order.remaining_size
                book.asks[order.price].orders += 1
                book.asks[order.price].order_ids.append(order.id)
        
        book.last_update_id += 1
        book.timestamp = int(time.time())
    
    async def _remove_from_order_book(self, order: Order):
        """Remove order from the order book"""
        book = self.order_books.get(order.market)
        if not book:
            return
        
        levels = book.bids if order.side == "buy" else book.asks
        
        if order.price in levels:
            level = levels[order.price]
            level.size -= order.remaining_size
            level.orders -= 1
            level.order_ids.remove(order.id)
            
            # Remove empty levels
            if level.size <= 0:
                del levels[order.price]
                if order.price in book.sorted_bids:
                    book.sorted_bids.remove(order.price)
                elif order.price in book.sorted_asks:
                    book.sorted_asks.remove(order.price)
        
        book.last_update_id += 1
        book.timestamp = int(time.time())
    
    async def _execute_market_order(self, order: Order):
        """Execute a market order against the order book"""
        market = self.markets[order.market]
        book = self.order_books[order.market]
        
        remaining = order.remaining_size
        total_cost = 0.0
        fills = []
        
        # Get opposite side of book
        opposite_side = book.asks if order.side == "buy" else book.bids
        sorted_prices = book.sorted_asks if order.side == "buy" else book.sorted_bids
        
        if not sorted_prices:
            order.status = "expired"
            return
        
        # Walk through the book
        for price in sorted_prices:
            if remaining <= 0:
                break
            
            level = opposite_side.get(price)
            if not level:
                continue
            
            # Calculate fill at this price
            fill_size = min(remaining, level.size)
            fill_value = fill_size * price
            
            fills.append({
                "price": price,
                "size": fill_size,
                "value": fill_value
            })
            
            total_cost += fill_value
            remaining -= fill_size
        
        # Check if we could fill
        if fills:
            avg_price = total_cost / (order.size - remaining) if order.size > remaining else 0
            
            # Update order
            order.filled_size = order.size - remaining
            order.remaining_size = remaining
            order.avg_fill_price = avg_price
            order.status = "filled" if remaining == 0 else "partial"
            
            # Update market price
            market.mark_price = avg_price
            self.last_prices[order.market] = avg_price
            
            # Update position
            await self._update_position(order, fills)
            
            # Create trades
            await self._create_trades(order, fills)
        else:
            order.status = "expired"
    
    async def _update_position(self, order: Order, fills: List[Dict]):
        """Update or create position after trade"""
        user = order.user_address
        market = order.market
        
        # Find existing position
        position_id = self._get_user_position_id(user, market)
        
        if position_id and order.reduce_only:
            # Can only reduce existing position
            position = self.positions[position_id]
            # Handle position update
            pass
        elif position_id:
            # Update existing position
            position = self.positions[position_id]
            # Recalculate position
            pass
        else:
            # Create new position
            position_id = self._generate_position_id(user, market)
            position = Position(
                id=position_id,
                user_address=user,
                market=market,
                side="long" if order.side == "buy" else "short",
                size=order.filled_size,
                entry_price=sum(f["price"] * f["size"] for f in fills) / sum(f["size"] for f in fills),
                liquidation_price=0.0,
                margin=order.filled_size * order.price / order.leverage,
                created_at=int(time.time()),
                updated_at=int(time.time())
            )
            self.positions[position_id] = position
            
            if user not in self.user_positions:
                self.user_positions[user] = []
            self.user_positions[user].append(position_id)
        
        # Calculate liquidation price
        market_config = self.markets[market]
        if position.side == "long":
            position.liquidation_price = position.entry_price * (1 - market_config.initial_margin_fraction)
        else:
            position.liquidation_price = position.entry_price * (1 + market_config.initial_margin_fraction)
        
        # Update margin ratio
        position.margin_ratio = position.margin / (position.size * position.entry_price)
        
        # Check liquidation
        if position.margin_ratio < market_config.maintenance_margin_fraction:
            await self._liquidate_position(position)
        
        position.updated_at = int(time.time())
    
    async def _create_trades(self, order: Order, fills: List[Dict]):
        """Create trade records"""
        market = self.markets[order.market]
        
        for fill in fills:
            trade_id = self._generate_trade_id(order.id, fill["price"], fill["size"])
            
            trade = Trade(
                id=trade_id,
                market=order.market,
                buy_order_id=order.id if order.side == "buy" else "",
                sell_order_id=order.id if order.side == "sell" else "",
                buyer="buyer_address",  # Would be resolved from order book
                seller="seller_address",
                price=fill["price"],
                size=fill["size"],
                side=order.side,
                maker_fee=fill["size"] * fill["price"] * market.maker_fee,
                taker_fee=fill["size"] * fill["price"] * market.taker_fee,
                timestamp=int(time.time())
            )
            
            self.trades.append(trade)
    
    async def _liquidate_position(self, position: Position):
        """Liquidate an undercollateralized position"""
        position.status = "liquidating"
        
        # Liquidation logic
        # - Calculate bankruptcy price
        # - Liquidator takes over position
        # - Insurance fund covers shortfall
        
        self._emit_event("PositionLiquidated", {
            "position_id": position.id,
            "user": position.user_address,
            "size": position.size,
            "liquidation_price": position.liquidation_price
        })
        
        position.status = "liquidated"
    
    def get_order_book(self, market: str, depth: int = 20) -> Dict:
        """Get order book snapshot"""
        if market not in self.order_books:
            return {}
        
        book = self.order_books[market]
        
        bids = []
        for price in book.sorted_bids[:depth]:
            level = book.bids[price]
            bids.append({
                "price": level.price,
                "size": level.size,
                "orders": level.orders
            })
        
        asks = []
        for price in book.sorted_asks[:depth]:
            level = book.asks[price]
            asks.append({
                "price": level.price,
                "size": level.size,
                "orders": level.orders
            })
        
        # Calculate spread
        best_bid = book.sorted_bids[0] if book.sorted_bids else 0
        best_ask = book.sorted_asks[0] if book.sorted_asks else float('inf')
        spread = best_ask - best_bid if best_bid and best_ask != float('inf') else 0
        spread_bps = (spread / best_bid * 10000) if best_bid > 0 else 0
        
        return {
            "market": market,
            "bids": bids,
            "asks": asks,
            "spread": spread,
            "spread_bps": spread_bps,
            "last_update_id": book.last_update_id,
            "timestamp": book.timestamp
        }
    
    def get_user_positions(self, user_address: str) -> List[Dict]:
        """Get all positions for a user"""
        position_ids = self.user_positions.get(user_address.lower(), [])
        
        positions = []
        for pos_id in position_ids:
            pos = self.positions.get(pos_id)
            if pos and pos.status == "open":
                # Calculate current unrealized PnL
                market = self.markets.get(pos.market)
                if market:
                    if pos.side == "long":
                        pos.unrealized_pnl = (market.mark_price - pos.entry_price) * pos.size
                    else:
                        pos.unrealized_pnl = (pos.entry_price - market.mark_price) * pos.size
                    pos.unrealized_pnl_pct = pos.unrealized_pnl / (pos.entry_price * pos.size) * 100
                
                positions.append(pos.__dict__)
        
        return positions
    
    def get_user_orders(self, user_address: str) -> List[Dict]:
        """Get all orders for a user"""
        order_ids = self.user_orders.get(user_address.lower(), [])
        
        orders = []
        for order_id in order_ids:
            order = self.orders.get(order_id)
            if order:
                orders.append(order.__dict__)
        
        return orders
    
    def calculate_funding(self, market: str) -> List[FundingPayment]:
        """Calculate funding payments for a market"""
        market_config = self.markets.get(market)
        if not market_config:
            return []
        
        position_ids = self.user_positions.get(market, [])
        payments = []
        
        # Calculate funding rate based on mark vs index price
        premium = market_config.mark_price - market_config.index_price
        funding_rate = premium / market_config.index_price
        
        for pos_id in position_ids:
            position = self.positions.get(pos_id)
            if not position or position.status != "open":
                continue
            
            # Funding payment = position_size * funding_rate
            payment = position.size * funding_rate
            
            # Longs pay shorts (or vice versa based on funding direction)
            if position.side == "long":
                payment = -payment  # Longs pay funding
            else:
                payment = -payment  # Shorts receive funding (or pay)
            
            # Actually funding flows from longs to shorts when positive
            if funding_rate > 0:
                payment = -position.size * funding_rate if position.side == "long" else position.size * funding_rate
            
            payments.append(FundingPayment(
                user=position.user_address,
                market=market,
                payment=payment,
                funding_rate=funding_rate,
                mark_price=market_config.mark_price,
                position_size=position.size,
                timestamp=int(time.time())
            ))
        
        return payments
    
    def _get_user_position_id(self, user: str, market: str) -> Optional[str]:
        """Get user's position ID for a market"""
        position_ids = self.user_positions.get(user.lower(), [])
        for pos_id in position_ids:
            pos = self.positions.get(pos_id)
            if pos and pos.market == market and pos.status == "open":
                return pos_id
        return None
    
    def _generate_order_id(self, user: str, market: str, side: str, size: float, price: float) -> str:
        """Generate unique order ID"""
        data = f"{user}{market}{side}{size}{price}{time.time()}"
        return "ord_" + hashlib.sha256(data.encode()).hexdigest()[:32]
    
    def _generate_position_id(self, user: str, market: str) -> str:
        """Generate unique position ID"""
        data = f"{user}{market}{time.time()}"
        return "pos_" + hashlib.sha256(data.encode()).hexdigest()[:32]
    
    def _generate_trade_id(self, order_id: str, price: float, size: float) -> str:
        """Generate unique trade ID"""
        data = f"{order_id}{price}{size}{time.time()}"
        return "trd_" + hashlib.sha256(data.encode()).hexdigest()[:32]
    
    def _emit_event(self, event_type: str, data: Dict):
        """Emit event"""
        self.events.append({
            "type": event_type,
            "data": data,
            "timestamp": int(time.time())
        })


async def main():
    """Test perpetual trading"""
    print("=== Perpetual Trading Test ===")
    
    engine = OrderBookEngine()
    
    # Get order book
    book = engine.get_order_book("ETH-PERP")
    print(f"ETH-PERP Order Book:")
    print(f"  Bids: {len(book['bids'])} levels")
    print(f"  Asks: {len(book['asks'])} levels")
    print(f"  Spread: {book['spread_bps']:.2f} bps")
    
    # Create order
    order = await engine.create_order(
        user_address="0x742d35Cc6634C0532925a3b844Bc9e7595f2bD12",
        market="ETH-PERP",
        side="buy",
        order_type="limit",
        size=1.0,  # 1 ETH
        price=2400.0,
        leverage=10
    )
    
    print(f"\nOrder created: {order.id}")
    print(f"  Side: {order.side}")
    print(f"  Size: {order.size} ETH")
    print(f"  Price: ${order.price}")
    print(f"  Leverage: {order.leverage}x")
    
    # Get updated order book
    book = engine.get_order_book("ETH-PERP")
    print(f"\nUpdated Order Book:")
    print(f"  Bids: {len(book['bids'])} levels")
    if book['bids']:
        print(f"  Best Bid: ${book['bids'][0]['price']} x {book['bids'][0]['size']}")
    print(f"  Asks: {len(book['asks'])} levels")
    
    # Get user positions
    positions = engine.get_user_positions("0x742d35Cc6634C0532925a3b844Bc9e7595f2bD12")
    print(f"\nUser Positions: {len(positions)}")


if __name__ == "__main__":
    asyncio.run(main())
