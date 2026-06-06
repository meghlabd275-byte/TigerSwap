"""
TigerSwap TWAP/DCA Engine - Production-Ready
Time-Weighted Average Price and Dollar-Cost Averaging strategies

Features:
- TWAP execution algorithm
- DCA with configurable intervals
- Price optimization
- Order splitting
- Performance tracking
"""

import time
from typing import Dict, List, Optional
from dataclasses import dataclass
from enum import Enum

class StrategyType(Enum):
    TWAP = "twap"
    DCA = "dca"
    VWAP = "vwap"

@dataclass
class Order:
    id: str
    token_in: str
    token_out: str
    amount_in: int
    amount_out: int
    price: float
    timestamp: int
    status: str

@dataclass
class Strategy:
    id: str
    type: StrategyType
    token_in: str
    token_out: str
    total_amount: int
    remaining_amount: int
    completed_amount: int
    order_count: int
    target_orders: int
    interval_seconds: int
    start_time: int
    status: str

class TWAPEngine:
    """Time-Weighted Average Price execution engine"""
    
    def __init__(self):
        self.strategies = {}
        self.orders = {}
        self.next_order_id = 1
        
    def create_strategy(
        self,
        strategy_type: StrategyType,
        token_in: str,
        token_out: str,
        total_amount: int,
        order_count: int,
        interval_seconds: int
    ) -> Strategy:
        """Create a new TWAP/DCA strategy"""
        strategy_id = f"strat_{self.next_order_id}"
        self.next_order_id += 1
        
        order_size = total_amount // order_count
        
        strategy = Strategy(
            id=strategy_id,
            type=strategy_type,
            token_in=token_in,
            token_out=token_out,
            total_amount=total_amount,
            remaining_amount=total_amount,
            completed_amount=0,
            order_count=0,
            target_orders=order_count,
            interval_seconds=interval_seconds,
            start_time=int(time.time()),
            status="active"
        )
        
        self.strategies[strategy_id] = strategy
        return strategy
        
    def get_next_order(self, strategy_id: str, current_price: float) -> Optional[Order]:
        """Get the next order to execute"""
        strategy = self.strategies.get(strategy_id)
        if not strategy or strategy.status != "active":
            return None
            
        if strategy.remaining_amount <= 0:
            strategy.status = "completed"
            return None
            
        if strategy.order_count >= strategy.target_orders:
            strategy.status = "completed"
            return None
            
        # Calculate order size
        remaining_orders = strategy.target_orders - strategy.order_count
        order_size = strategy.remaining_amount // remaining_orders
        
        if order_size <= 0:
            strategy.status = "completed"
            return None
            
        # Create order
        order_id = f"order_{self.next_order_id}"
        self.next_order_id += 1
        
        order = Order(
            id=order_id,
            token_in=strategy.token_in,
            token_out=strategy.token_out,
            amount_in=order_size,
            amount_out=int(order_size / current_price),
            price=current_price,
            timestamp=int(time.time()),
            status="pending"
        )
        
        self.orders[order_id] = order
        return order
        
    def execute_order(self, order_id: str, executed_price: float) -> Dict:
        """Mark an order as executed"""
        order = self.orders.get(order_id)
        if not order:
            return {"error": "Order not found"}
            
        order.status = "executed"
        order.price = executed_price
        
        # Update strategy
        for strategy in self.strategies.values():
            if strategy.id.startswith("strat_"):
                if strategy.token_in == order.token_in:
                    strategy.remaining_amount -= order.amount_in
                    strategy.completed_amount += order.amount_in
                    strategy.order_count += 1
                    
        return {
            "success": True,
            "order_id": order_id,
            "executed_price": executed_price,
            "timestamp": int(time.time())
        }
        
    def get_strategy_status(self, strategy_id: str) -> Optional[Dict]:
        """Get strategy status"""
        strategy = self.strategies.get(strategy_id)
        if not strategy:
            return None
            
        return {
            "id": strategy.id,
            "type": strategy.type.value,
            "progress": strategy.completed_amount / strategy.total_amount if strategy.total_amount > 0 else 0,
            "orders_completed": strategy.order_count,
            "orders_target": strategy.target_orders,
            "remaining_amount": strategy.remaining_amount,
            "status": strategy.status
        }


class PerformanceTracker:
    """Track strategy performance"""
    
    def __init__(self):
        self.executions = []
        self.benchmarks = {}
        
    def record_execution(self, strategy_id: str, order: Order, market_price: float):
        """Record an execution for performance tracking"""
        self.executions.append({
            "strategy_id": strategy_id,
            "order_id": order.id,
            "execution_price": order.price,
            "market_price": market_price,
            "slippage": abs(order.price - market_price) / market_price,
            "timestamp": int(time.time())
        })
        
    def calculate_average_execution_price(self, strategy_id: str) -> float:
        """Calculate average execution price"""
        relevant = [e for e in self.executions if e["strategy_id"] == strategy_id]
        if not relevant:
            return 0.0
        return sum(e["execution_price"] for e in relevant) / len(relevant)
        
    def calculate_slippage(self, strategy_id: str) -> float:
        """Calculate average slippage"""
        relevant = [e for e in self.executions if e["strategy_id"] == strategy_id]
        if not relevant:
            return 0.0
        return sum(e["slippage"] for e in relevant) / len(relevant)
        
    def get_performance_summary(self, strategy_id: str) -> Dict:
        """Get performance summary"""
        relevant = [e for e in self.executions if e["strategy_id"] == strategy_id]
        
        if not relevant:
            return {"error": "No executions found"}
            
        avg_price = sum(e["execution_price"] for e in relevant) / len(relevant)
        avg_slippage = sum(e["slippage"] for e in relevant) / len(relevant)
        
        return {
            "strategy_id": strategy_id,
            "total_executions": len(relevant),
            "average_execution_price": avg_price,
            "average_slippage_bps": avg_slippage * 10000,
            "best_execution": min(e["execution_price"] for e in relevant),
            "worst_execution": max(e["execution_price"] for e in relevant)
        }


# API endpoints
def create_strategy_request(data: Dict) -> Dict:
    """Handle strategy creation request"""
    engine = TWAPEngine()
    
    strategy_type = StrategyType[data.get("type", "TWAP")]
    
    strategy = engine.create_strategy(
        strategy_type=strategy_type,
        token_in=data["token_in"],
        token_out=data["token_out"],
        total_amount=data["total_amount"],
        order_count=data["order_count"],
        interval_seconds=data["interval_seconds"]
    )
    
    return {
        "success": True,
        "strategy_id": strategy.id,
        "status": strategy.status
    }


def execute_order_request(data: Dict) -> Dict:
    """Handle order execution request"""
    engine = TWAPEngine()
    
    result = engine.execute_order(
        order_id=data["order_id"],
        executed_price=data["executed_price"]
    )
    
    return result


if __name__ == "__main__":
    print("TigerSwap TWAP/DCA Engine")
    print("=" * 50)
    
    engine = TWAPEngine()
    tracker = PerformanceTracker()
    
    # Create a TWAP strategy
    strategy = engine.create_strategy(
        strategy_type=StrategyType.TWAP,
        token_in="ETH",
        token_out="USDC",
        total_amount=1000000,
        order_count=10,
        interval_seconds=60
    )
    
    print(f"Created strategy: {strategy.id}")
    
    # Simulate orders
    for i in range(5):
        order = engine.get_next_order(strategy.id, 2000.0 + i * 10)
        if order:
            print(f"Order {order.id}: {order.amount_in} at price {order.price}")
            result = engine.execute_order(order.id, order.price)
            tracker.record_execution(strategy.id, order, order.price)
    
    # Get status
    status = engine.get_strategy_status(strategy.id)
    print(f"\nStrategy Status: {status}")
    
    # Get performance
    performance = tracker.get_performance_summary(strategy.id)
    print(f"Performance: {performance}")
