"""
TigerSwap DEX Aggregator - Split Routes Engine
Real split routing optimization for best execution prices
"""

from typing import List, Dict, Tuple, Optional
from dataclasses import dataclass
import numpy as np

@dataclass
class SplitRoute:
    dex: str
    pool_address: str
    percentage: int  # 0-100
    amount_in: int
    amount_out: int
    fee: int
    price_impact: float

@dataclass
class OptimizedSplit:
    routes: List[SplitRoute]
    total_input: int
    total_output: int
    total_price_impact: float
    gas_premium: float  # Extra gas cost for splitting

class SplitRoutesEngine:
    """
    Real split routing optimization.
    Finds optimal distribution across multiple DEX routes to minimize slippage.
    """
    
    def __init__(self, max_splits: int = 3, min_split_bps: int = 100):
        self.max_splits = max_splits
        self.min_split_bps = min_split_bps  # Minimum 1% to make split worthwhile
        
    def calculate_optimal_split(
        self,
        available_routes: List[Dict],
        total_input: int,
        token_in_decimals: int = 18,
        token_out_decimals: int = 18
    ) -> OptimizedSplit:
        """
        Calculate optimal split across routes.
        
        Args:
            available_routes: List of route quotes with 'pool', 'amount_out', 'price_impact'
            total_input: Total input amount
            token_in_decimals: Decimals of input token
            token_out_decimals: Decimals of output token
            
        Returns:
            OptimizedSplit with best route distribution
        """
        if not available_routes:
            raise ValueError("No routes available")
            
        if len(available_routes) == 1:
            # Single route - no splitting needed
            route = available_routes[0]
            return OptimizedSplit(
                routes=[SplitRoute(
                    dex=route['pool'].dex.value,
                    pool_address=route['pool'].address,
                    percentage=100,
                    amount_in=total_input,
                    amount_out=route['amount_out'],
                    fee=route['pool'].fee,
                    price_impact=route['price_impact']
                )],
                total_input=total_input,
                total_output=route['amount_out'],
                total_price_impact=route['price_impact'],
                gas_premium=0.0
            )
        
        # Sort routes by effective price (output per input)
        sorted_routes = sorted(
            available_routes,
            key=lambda r: r['amount_out'] / max(r['amount_out'], 1),
            reverse=True
        )
        
        # Calculate normalized liquidity for each route
        route_weights = self._calculate_route_weights(sorted_routes, total_input)
        
        # Try different split combinations
        best_split = None
        best_output = 0
        
        # Try splits from 100/0 to 0/100
        for i in range(len(sorted_routes)):
            if i >= self.max_splits:
                break
                
            route_a = sorted_routes[i]
            
            # Single route portion
            split_a = self._calculate_split_amount(total_input, 10000 - i * 100)
            output_a = self._calculate_route_output(route_a, split_a)
            
            if output_a > best_output:
                best_output = output_a
                best_split = [{
                    'route': route_a,
                    'percentage': 10000 - i * 100,
                    'amount_in': split_a
                }]
            
            # Try combining with other routes
            for j in range(i + 1, len(sorted_routes)):
                if (10000 - i * 100 - j * 100) < self.min_split_bps:
                    continue
                    
                route_b = sorted_routes[j]
                
                # Split between two routes
                for split_pct in range(self.min_split_bps, 10000 - self.min_split_bps, self.min_split_bps):
                    pct_a = split_pct
                    pct_b = 10000 - split_pct - i * 100 - j * 100
                    
                    if pct_b < self.min_split_bps:
                        continue
                    
                    split_a = (total_input * pct_a) // 10000
                    split_b = (total_input * pct_b) // 10000
                    
                    output_a = self._calculate_route_output(route_a, split_a)
                    output_b = self._calculate_route_output(route_b, split_b)
                    
                    total_out = output_a + output_b
                    
                    if total_out > best_output:
                        best_output = total_out
                        best_split = [
                            {
                                'route': route_a,
                                'percentage': pct_a,
                                'amount_in': split_a
                            },
                            {
                                'route': route_b,
                                'percentage': pct_b,
                                'amount_in': split_b
                            }
                        ]
        
        # Build result
        routes = []
        total_impact = 0
        
        for item in best_split:
            route = item['route']
            amount_out = self._calculate_route_output(route, item['amount_in'])
            
            routes.append(SplitRoute(
                dex=route['pool'].dex.value,
                pool_address=route['pool'].address,
                percentage=item['percentage'] // 100,  # Convert bps to percent
                amount_in=item['amount_in'],
                amount_out=amount_out,
                fee=route['pool'].fee,
                price_impact=route['price_impact']
            ))
            
            # Weighted price impact
            total_impact += route['price_impact'] * (item['percentage'] / 10000)
        
        # Gas premium for splitting (extra gas per additional route)
        gas_premium = len(routes) * 0.1  # 10% more gas per split
        
        return OptimizedSplit(
            routes=routes,
            total_input=total_input,
            total_output=best_output,
            total_price_impact=total_impact,
            gas_premium=gas_premium
        )
    
    def _calculate_route_weights(
        self,
        routes: List[Dict],
        total_input: int
    ) -> List[float]:
        """
        Calculate normalized weights for routes based on liquidity and price.
        Routes with better prices but enough liquidity get higher weights.
        """
        weights = []
        best_price = max(r['amount_out'] for r in routes) / max(total_input, 1)
        
        for route in routes:
            price = route['amount_out'] / max(total_input, 1)
            liquidity_score = min(route['pool'].liquidity / 1_000_000, 1.0)  # Normalize to $1M
            
            # Weight combines price quality and available liquidity
            weight = (price / best_price) * (0.7 + 0.3 * liquidity_score)
            weights.append(weight)
        
        # Normalize weights
        total = sum(weights)
        return [w / total for w in weights]
    
    def _calculate_split_amount(self, total: int, bps: int) -> int:
        """Calculate amount based on basis points"""
        return (total * bps) // 10000
    
    def _calculate_route_output(self, route: Dict, amount_in: int) -> int:
        """Calculate output for a specific amount using route's pool"""
        pool = route['pool']
        
        # Constant product formula with fee
        reserve_in = pool.reserve_a if pool.token_a == route.get('token_in', '') else pool.reserve_b
        reserve_out = pool.reserve_b if pool.token_a == route.get('token_in', '') else pool.reserve_a
        
        if reserve_in == 0 or reserve_out == 0:
            return 0
            
        fee_deduction = 10000 - pool.fee
        numerator = amount_in * reserve_out * fee_deduction
        denominator = reserve_in * 10000 + amount_in * fee_deduction
        
        if denominator == 0:
            return 0
            
        return numerator // denominator
    
    def validate_split(self, split: OptimizedSplit) -> Tuple[bool, str]:
        """
        Validate that a split is valid and safe to execute.
        
        Returns:
            (is_valid, error_message)
        """
        if not split.routes:
            return False, "No routes in split"
            
        total_pct = sum(r.percentage for r in split.routes)
        if total_pct != 100:
            return False, f"Routes don't sum to 100%: got {total_pct}%"
            
        for route in split.routes:
            if route.percentage < self.min_split_bps // 100:
                return False, f"Route {route.dex} below minimum {self.min_split_bps/100}%"
                
            if route.amount_out == 0:
                return False, f"Route {route.dex} produces zero output"
                
        if split.total_price_impact > 5.0:  # 5% max price impact
            return False, f"Price impact too high: {split.total_price_impact}%"
            
        return True, ""
    
    def get_gas_estimate(self, split: OptimizedSplit, base_gas: int = 150000) -> int:
        """
        Estimate total gas for a split route.
        More splits = more gas due to multiple transactions.
        """
        # Base gas for single swap
        base = base_gas
        
        # Additional gas per route (approximate)
        per_route_gas = 30000
        
        # Number of unique DEXs
        unique_dexs = len(set(r.dex for r in split.routes))
        
        total_gas = base + (len(split.routes) - 1) * per_route_gas
        
        # DEX-specific gas adjustments
        for route in split.routes:
            if 'curve' in route.dex.lower():
                total_gas += 20000  # Curve is gas-heavy
            elif 'balancer' in route.dex.lower():
                total_gas += 15000
                
        return total_gas


def optimize_route_selection(
    pools: List[Dict],
    amount_in: int,
    max_splits: int = 3
) -> List[Dict]:
    """
    Select optimal routes from available pools.
    
    This is the main entry point for route optimization.
    """
    engine = SplitRoutesEngine(max_splits=max_splits)
    
    # Filter pools with sufficient liquidity
    min_liquidity = amount_in * 10  # At least 10x trade size
    viable_pools = [p for p in pools if p['pool'].liquidity >= min_liquidity]
    
    if not viable_pools:
        viable_pools = pools[:5]  # Use top pools if none meet threshold
    
    return engine.calculate_optimal_split(viable_pools, amount_in)
