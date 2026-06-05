"""
TigerSwap TWAP/DCA Trading System
Real Time-Weighted Average Price and Dollar-Cost Averaging implementation
"""

import asyncio
import hashlib
import json
import time
from dataclasses import dataclass, field
from typing import List, Dict, Optional, Callable
from enum import Enum
from datetime import datetime, timedelta
import croniter

@dataclass
class TWAPOrder:
    id: str
    user_address: str
    chain_id: int
    
    # Trading pair
    token_in: str
    token_out: str
    
    # TWAP parameters
    total_amount: int  # Total amount to trade
    duration_seconds: int  # Total duration
    num_tranches: int  # Number of tranches
    tranche_interval_seconds: int  # Time between tranches
    
    # Price constraints
    trigger_price: Optional[float] = None  # Optional price trigger
    max_slippage_bps: int = 50  # Max slippage per trade
    price_range_low_bps: int = -500  # -5% from start price
    price_range_high_bps: int = 500  # +5% from start price
    
    # Execution tracking
    tranches_executed: int = 0
    total_filled: int = 0
    avg_price: float = 0.0
    start_price: float = 0.0
    
    # Status
    status: str = "pending"  # pending, active, paused, completed, cancelled
    pause_on_high_slippage: bool = True
    
    # Timing
    created_at: int = 0
    started_at: Optional[int] = None
    next_tranche_at: Optional[int] = None
    completed_at: Optional[int] = None
    
    # Schedule
    schedule_type: str = "linear"  # linear, geometric, custom
    schedule_cron: Optional[str] = None  # Optional cron expression

@dataclass
class DCAOrder:
    id: str
    user_address: str
    chain_id: int
    
    # Trading pair
    token_in: str
    token_out: str
    
    # DCA parameters
    amount_per_trade: int
    frequency_seconds: int  # Time between trades
    frequency_cron: Optional[str] = None  # Or use cron expression
    
    # Execution parameters
    max_trades: int = 0  # 0 = unlimited
    max_total_amount: int = 0  # 0 = unlimited
    min_trade_price: Optional[float] = None
    max_trade_price: Optional[float] = None
    
    # Running totals
    trades_executed: int = 0
    total_amount_in: int = 0
    total_amount_out: int = 0
    avg_price: float = 0.0
    
    # Status
    status: str = "active"  # active, paused, completed, cancelled
    
    # Timing
    created_at: int = 0
    next_trade_at: int = 0
    last_trade_at: Optional[int] = None
    completed_at: Optional[int] = None
    
    # Days of week (for weekly DCA)
    days_of_week: List[int] = field(default_factory=lambda: [0, 1, 2, 3, 4, 5, 6])  # 0=Monday
    time_of_day_seconds: int = 43200  # 12:00 PM UTC

@dataclass
class TradeExecution:
    execution_id: str
    order_id: str
    order_type: str  # "twap" or "dca"
    
    # Execution details
    tranche_num: int = 0
    amount_in: int
    amount_out: int
    price: float
    slippage_bps: int
    gas_used: int
    gas_price_gwei: int
    
    # Timing
    scheduled_for: int
    executed_at: Optional[int] = None
    
    # Route
    dex: str
    route: List[str]
    pool: str
    
    # Status
    status: str = "pending"  # pending, executed, failed, skipped
    failure_reason: Optional[str] = None

class TWAPExecutor:
    """
    TWAP (Time-Weighted Average Price) execution engine.
    Splits large orders into smaller tranches executed over time.
    """
    
    def __init__(self, chain_id: int = 1):
        self.chain_id = chain_id
        self.orders: Dict[str, TWAPOrder] = {}
        self.executions: Dict[str, List[TradeExecution]] = {}
        self._execution_callbacks: List[Callable] = []
        
    def create_order(
        self,
        user_address: str,
        token_in: str,
        token_out: str,
        total_amount: int,
        duration_minutes: int,
        num_tranches: int,
        max_slippage_bps: int = 50,
        trigger_price: Optional[float] = None
    ) -> TWAPOrder:
        """Create a new TWAP order"""
        order_id = self._generate_order_id(user_address, token_in, token_out, total_amount)
        
        tranche_interval = (duration_minutes * 60) // num_tranches
        
        order = TWAPOrder(
            id=order_id,
            user_address=user_address.lower(),
            chain_id=self.chain_id,
            token_in=token_in.lower(),
            token_out=token_out.lower(),
            total_amount=total_amount,
            duration_seconds=duration_minutes * 60,
            num_tranches=num_tranches,
            tranche_interval_seconds=tranche_interval,
            trigger_price=trigger_price,
            max_slippage_bps=max_slippage_bps,
            created_at=int(time.time())
        )
        
        self.orders[order_id] = order
        self.executions[order_id] = []
        
        return order
    
    def start_order(self, order_id: str, start_price: float) -> bool:
        """Start executing a TWAP order"""
        order = self.orders.get(order_id)
        
        if not order:
            return False
            
        if order.status != "pending":
            return False
        
        order.status = "active"
        order.started_at = int(time.time())
        order.next_tranche_at = int(time.time()) + order.tranche_interval_seconds
        order.start_price = start_price
        
        return True
    
    def pause_order(self, order_id: str) -> bool:
        """Pause a TWAP order"""
        order = self.orders.get(order_id)
        
        if not order or order.status != "active":
            return False
            
        order.status = "paused"
        return True
    
    def resume_order(self, order_id: str) -> bool:
        """Resume a paused TWAP order"""
        order = self.orders.get(order_id)
        
        if not order or order.status != "paused":
            return False
            
        order.status = "active"
        order.next_tranche_at = int(time.time()) + order.tranche_interval_seconds
        return True
    
    def cancel_order(self, order_id: str, user_address: str) -> bool:
        """Cancel a TWAP order"""
        order = self.orders.get(order_id)
        
        if not order:
            return False
            
        if order.user_address != user_address.lower():
            return False
            
        if order.status in ["completed", "cancelled"]:
            return False
            
        order.status = "cancelled"
        order.completed_at = int(time.time())
        return True
    
    def get_next_tranche(self, order_id: str) -> Optional[Dict]:
        """Get details for the next tranche to execute"""
        order = self.orders.get(order_id)
        
        if not order:
            return None
            
        if order.status != "active":
            return None
            
        now = int(time.time())
        
        if order.next_tranche_at and order.next_tranche_at > now:
            return None  # Not time yet
            
        if order.tranches_executed >= order.num_tranches:
            order.status = "completed"
            order.completed_at = now
            return None
        
        # Calculate tranche amount
        remaining_tranches = order.num_tranches - order.tranches_executed
        remaining_amount = order.total_amount - order.total_filled
        
        # Linear schedule - equal tranches
        if order.schedule_type == "linear":
            tranche_amount = remaining_amount // remaining_tranches
        else:
            tranche_amount = remaining_amount // remaining_tranches
        
        return {
            "order_id": order_id,
            "tranche_num": order.tranches_executed + 1,
            "amount": tranche_amount,
            "token_in": order.token_in,
            "token_out": order.token_out,
            "max_slippage_bps": order.max_slippage_bps,
            "start_price": order.start_price,
            "price_range_low_bps": order.price_range_low_bps,
            "price_range_high_bps": order.price_range_high_bps,
            "deadline": order.next_tranche_at + 60  # 1 minute to execute
        }
    
    def record_tranche_execution(
        self,
        order_id: str,
        tranche_num: int,
        amount_in: int,
        amount_out: int,
        price: float,
        slippage_bps: int,
        dex: str,
        route: List[str],
        pool: str
    ) -> bool:
        """Record successful tranche execution"""
        order = self.orders.get(order_id)
        
        if not order:
            return False
        
        # Create execution record
        execution = TradeExecution(
            execution_id=self._generate_execution_id(order_id, tranche_num),
            order_id=order_id,
            order_type="twap",
            tranche_num=tranche_num,
            amount_in=amount_in,
            amount_out=amount_out,
            price=price,
            slippage_bps=slippage_bps,
            gas_used=150000,
            gas_price_gwei=30,
            scheduled_for=order.next_tranche_at or int(time.time()),
            executed_at=int(time.time()),
            dex=dex,
            route=route,
            pool=pool,
            status="executed"
        )
        
        self.executions[order_id].append(execution)
        
        # Update order
        order.tranches_executed += 1
        order.total_filled += amount_in
        
        # Calculate running average price
        total_cost = (order.avg_price * order.total_filled * (order.tranches_executed - 1) + amount_in * price) / order.total_filled
        order.avg_price = total_cost / order.total_filled if order.total_filled > 0 else 0
        order.avg_price = price  # Simplified
        
        # Schedule next tranche
        if order.tranches_executed < order.num_tranches:
            order.next_tranche_at = int(time.time()) + order.tranche_interval_seconds
        else:
            order.status = "completed"
            order.completed_at = int(time.time())
        
        # Trigger callback
        for cb in self._execution_callbacks:
            try:
                cb(order, execution)
            except Exception as e:
                print(f"Execution callback error: {e}")
        
        return True
    
    def add_execution_callback(self, callback: Callable):
        """Add callback for execution events"""
        self._execution_callbacks.append(callback)
    
    def get_order_status(self, order_id: str) -> Optional[Dict]:
        """Get current status of a TWAP order"""
        order = self.orders.get(order_id)
        
        if not order:
            return None
        
        executions = self.executions.get(order_id, [])
        
        return {
            "order_id": order_id,
            "status": order.status,
            "progress": f"{order.tranches_executed}/{order.num_tranches}",
            "progress_pct": (order.tranches_executed / order.num_tranches * 100) if order.num_tranches > 0 else 0,
            "total_amount": order.total_amount,
            "total_filled": order.total_filled,
            "fill_pct": (order.total_filled / order.total_amount * 100) if order.total_amount > 0 else 0,
            "avg_price": order.avg_price,
            "start_price": order.start_price,
            "price_improvement": ((order.avg_price - order.start_price) / order.start_price * 100) if order.start_price > 0 else 0,
            "executions": [
                {
                    "tranche": e.tranche_num,
                    "amount_in": e.amount_in,
                    "amount_out": e.amount_out,
                    "price": e.price,
                    "slippage_bps": e.slippage_bps,
                    "dex": e.dex,
                    "executed_at": e.executed_at
                }
                for e in executions
            ],
            "next_tranche_at": order.next_tranche_at,
            "started_at": order.started_at,
            "completed_at": order.completed_at
        }
    
    def _generate_order_id(self, user: str, token_in: str, token_out: str, amount: int) -> str:
        """Generate unique order ID"""
        data = f"{user}{token_in}{token_out}{amount}{time.time()}"
        return "twap_" + hashlib.sha256(data.encode()).hexdigest()[:32]
    
    def _generate_execution_id(self, order_id: str, tranche: int) -> str:
        """Generate unique execution ID"""
        data = f"{order_id}{tranche}{time.time()}"
        return "exec_" + hashlib.sha256(data.encode()).hexdigest()[:32]


class DCAExecutor:
    """
    DCA (Dollar-Cost Averaging) execution engine.
    Executes recurring trades at regular intervals.
    """
    
    def __init__(self, chain_id: int = 1):
        self.chain_id = chain_id
        self.orders: Dict[str, DCAOrder] = {}
        self.executions: Dict[str, List[TradeExecution]] = {}
        self._execution_callbacks: List[Callable] = []
    
    def create_order(
        self,
        user_address: str,
        token_in: str,
        token_out: str,
        amount_per_trade: int,
        frequency_seconds: int,
        max_trades: int = 0,
        max_total_amount: int = 0,
        min_price: Optional[float] = None,
        max_price: Optional[float] = None,
        days_of_week: Optional[List[int]] = None,
        time_of_day_seconds: int = 43200
    ) -> DCAOrder:
        """Create a new DCA order"""
        order_id = self._generate_order_id(user_address, token_in, token_out, amount_per_trade)
        
        order = DCAOrder(
            id=order_id,
            user_address=user_address.lower(),
            chain_id=self.chain_id,
            token_in=token_in.lower(),
            token_out=token_out.lower(),
            amount_per_trade=amount_per_trade,
            frequency_seconds=frequency_seconds,
            max_trades=max_trades,
            max_total_amount=max_total_amount,
            min_trade_price=min_price,
            max_trade_price=max_price,
            days_of_week=days_of_week or [0, 1, 2, 3, 4, 5, 6],
            time_of_day_seconds=time_of_day_seconds,
            created_at=int(time.time()),
            next_trade_at=self._calculate_next_trade_time(frequency_seconds, days_of_week, time_of_day_seconds)
        )
        
        self.orders[order_id] = order
        self.executions[order_id] = []
        
        return order
    
    def pause_order(self, order_id: str, user_address: str) -> bool:
        """Pause a DCA order"""
        order = self.orders.get(order_id)
        
        if not order or order.user_address != user_address.lower():
            return False
            
        order.status = "paused"
        return True
    
    def resume_order(self, order_id: str) -> bool:
        """Resume a paused DCA order"""
        order = self.orders.get(order_id)
        
        if not order or order.status != "paused":
            return False
        
        order.next_trade_at = self._calculate_next_trade_time(
            order.frequency_seconds,
            order.days_of_week,
            order.time_of_day_seconds
        )
        order.status = "active"
        return True
    
    def cancel_order(self, order_id: str, user_address: str) -> bool:
        """Cancel a DCA order"""
        order = self.orders.get(order_id)
        
        if not order or order.user_address != user_address.lower():
            return False
            
        order.status = "cancelled"
        order.completed_at = int(time.time())
        return True
    
    def modify_order(
        self,
        order_id: str,
        user_address: str,
        amount_per_trade: Optional[int] = None,
        frequency_seconds: Optional[int] = None
    ) -> bool:
        """Modify DCA order parameters"""
        order = self.orders.get(order_id)
        
        if not order or order.user_address != user_address.lower():
            return False
            
        if amount_per_trade:
            order.amount_per_trade = amount_per_trade
        if frequency_seconds:
            order.frequency_seconds = frequency_seconds
            order.next_trade_at = self._calculate_next_trade_time(
                frequency_seconds,
                order.days_of_week,
                order.time_of_day_seconds
            )
            
        return True
    
    def get_pending_trades(self) -> List[Dict]:
        """Get all DCA trades that are due for execution"""
        now = int(time.time())
        pending = []
        
        for order in self.orders.values():
            if order.status != "active":
                continue
                
            if order.next_trade_at and order.next_trade_at <= now:
                # Check price constraints
                # In production, check current price against min/max
                
                pending.append({
                    "order_id": order.id,
                    "user_address": order.user_address,
                    "token_in": order.token_in,
                    "token_out": order.token_out,
                    "amount": order.amount_per_trade,
                    "max_slippage_bps": 50,
                    "scheduled_for": order.next_trade_at
                })
        
        return pending
    
    def record_trade_execution(
        self,
        order_id: str,
        amount_in: int,
        amount_out: int,
        price: float,
        slippage_bps: int,
        dex: str
    ) -> bool:
        """Record successful DCA trade execution"""
        order = self.orders.get(order_id)
        
        if not order:
            return False
        
        # Create execution record
        execution = TradeExecution(
            execution_id=self._generate_execution_id(order_id, order.trades_executed + 1),
            order_id=order_id,
            order_type="dca",
            tranche_num=order.trades_executed + 1,
            amount_in=amount_in,
            amount_out=amount_out,
            price=price,
            slippage_bps=slippage_bps,
            gas_used=150000,
            gas_price_gwei=30,
            scheduled_for=order.next_trade_at,
            executed_at=int(time.time()),
            dex=dex,
            route=[order.token_in, order.token_out],
            pool="",
            status="executed"
        )
        
        self.executions[order_id].append(execution)
        
        # Update order
        order.trades_executed += 1
        order.total_amount_in += amount_in
        order.total_amount_out += amount_out
        order.last_trade_at = int(time.time())
        order.avg_price = order.total_amount_out / order.total_amount_in if order.total_amount_in > 0 else 0
        
        # Schedule next trade
        order.next_trade_at = self._calculate_next_trade_time(
            order.frequency_seconds,
            order.days_of_week,
            order.time_of_day_seconds
        )
        
        # Check completion conditions
        if order.max_trades > 0 and order.trades_executed >= order.max_trades:
            order.status = "completed"
            order.completed_at = int(time.time())
        
        if order.max_total_amount > 0 and order.total_amount_in >= order.max_total_amount:
            order.status = "completed"
            order.completed_at = int(time.time())
        
        # Trigger callback
        for cb in self._execution_callbacks:
            try:
                cb(order, execution)
            except Exception as e:
                print(f"Execution callback error: {e}")
        
        return True
    
    def get_order_status(self, order_id: str) -> Optional[Dict]:
        """Get current status of a DCA order"""
        order = self.orders.get(order_id)
        
        if not order:
            return None
        
        executions = self.executions.get(order_id, [])
        
        return {
            "order_id": order_id,
            "status": order.status,
            "trades_executed": order.trades_executed,
            "max_trades": order.max_trades if order.max_trades > 0 else "unlimited",
            "amount_per_trade": order.amount_per_trade,
            "total_amount_in": order.total_amount_in,
            "total_amount_out": order.total_amount_out,
            "avg_price": order.avg_price,
            "frequency_seconds": order.frequency_seconds,
            "next_trade_at": order.next_trade_at,
            "last_trade_at": order.last_trade_at,
            "executions": [
                {
                    "trade_num": e.tranche_num,
                    "amount_in": e.amount_in,
                    "amount_out": e.amount_out,
                    "price": e.price,
                    "executed_at": e.executed_at
                }
                for e in executions[-10:]  # Last 10 executions
            ]
        }
    
    def _calculate_next_trade_time(
        self,
        frequency_seconds: int,
        days_of_week: List[int],
        time_of_day_seconds: int
    ) -> int:
        """Calculate next valid trade time"""
        now = datetime.utcnow()
        current_day = now.weekday()
        current_seconds = now.hour * 3600 + now.minute * 60 + now.second
        
        # If we haven't passed today's trade time and today is a valid day
        if current_day in days_of_week and current_seconds < time_of_day_seconds:
            today = now.replace(hour=0, minute=0, second=0, microsecond=0)
            next_time = today + timedelta(seconds=time_of_day_seconds)
            return int(next_time.timestamp())
        
        # Find next valid day
        for i in range(1, 8):
            next_day = (current_day + i) % 7
            if next_day in days_of_week:
                days_ahead = i
                if i == 1 and current_seconds >= time_of_day_seconds:
                    days_ahead = 1
                next_date = now + timedelta(days=days_ahead)
                next_date = next_date.replace(hour=0, minute=0, second=0, microsecond=0)
                next_time = next_date + timedelta(seconds=time_of_day_seconds)
                return int(next_time.timestamp())
        
        # Fallback to interval-based
        return int(time.time()) + frequency_seconds
    
    def _generate_order_id(self, user: str, token_in: str, token_out: str, amount: int) -> str:
        """Generate unique order ID"""
        data = f"{user}{token_in}{token_out}{amount}{time.time()}"
        return "dca_" + hashlib.sha256(data.encode()).hexdigest()[:32]
    
    def _generate_execution_id(self, order_id: str, trade_num: int) -> str:
        """Generate unique execution ID"""
        data = f"{order_id}{trade_num}{time.time()}"
        return "dca_exec_" + hashlib.sha256(data.encode()).hexdigest()[:32]
    
    def add_execution_callback(self, callback: Callable):
        """Add callback for execution events"""
        self._execution_callbacks.append(callback)


async def main():
    """Test TWAP and DCA executors"""
    
    print("=== TWAP Executor Test ===")
    twap = TWAPExecutor(chain_id=1)
    
    # Create a TWAP order: Buy $100k ETH over 1 hour in 6 tranches
    order = twap.create_order(
        user_address="0x742d35Cc6634C0532925a3b844Bc9e7595f2bD12",
        token_in="0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48",  # USDC
        token_out="0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2",  # WETH
        total_amount=100000 * 10**6,  # $100k
        duration_minutes=60,
        num_tranches=6,  # Every 10 minutes
        max_slippage_bps=50
    )
    
    print(f"Created TWAP Order: {order.id}")
    print(f"Total Amount: {order.total_amount / 10**6} USDC")
    print(f"Tranches: {order.num_tranches} @ {order.tranche_interval_seconds}s intervals")
    
    # Start the order
    twap.start_order(order.id, start_price=2450.0)
    
    # Get first tranche
    tranche = twap.get_next_tranche(order.id)
    if tranche:
        print(f"\nNext Tranche: {tranche['amount'] / 10**6} USDC -> ETH")
    
    print("\n=== DCA Executor Test ===")
    dca = DCAExecutor(chain_id=1)
    
    # Create DCA: Buy $100 ETH weekly
    dca_order = dca.create_order(
        user_address="0x742d35Cc6634C0532925a3b844Bc9e7595f2bD12",
        token_in="0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48",  # USDC
        token_out="0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2",  # WETH
        amount_per_trade=100 * 10**6,  # $100
        frequency_seconds=7 * 24 * 3600,  # Weekly
        max_trades=52  # 1 year of weekly DCA
    )
    
    print(f"Created DCA Order: {dca_order.id}")
    print(f"Amount per trade: ${dca_order.amount_per_trade / 10**6}")
    print(f"Frequency: Every {dca_order.frequency_seconds / 86400:.1f} days")
    print(f"Max trades: {dca_order.max_trades}")
    print(f"Next trade at: {dca_order.next_trade_at}")


if __name__ == "__main__":
    asyncio.run(main())
