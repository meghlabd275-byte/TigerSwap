"""
TigerSwap Concentrated Liquidity Engine
Uniswap V3 style concentrated liquidity implementation with range orders
"""

import math
import hashlib
import json
import time
from dataclasses import dataclass, field
from typing import Dict, List, Optional, Tuple, Set
from enum import Enum
from decimal import Decimal

@dataclass
class Position:
    """Represents a liquidity position in a concentrated range"""
    id: str
    owner: str
    pool: str
    
    # Token info
    token_a: str
    token_b: str
    token_a_decimals: int
    token_b_decimals: int
    
    # Position range (price in terms of token_a/token_b)
    tick_lower: int  # Lower price tick
    tick_upper: int  # Upper price tick
    
    # Liquidity amount
    liquidity: int
    
    # Accumulated fees
    fee_growth_inside_a: float = 0.0  # Fee growth inside range for token A
    fee_growth_inside_b: float = 0.0  # Fee growth inside range for token B
    fees_owed_a: int = 0
    Fees_owed_b: int = 0
    
    # Amounts (calculated on mint)
    amount_a_desired: int = 0
    amount_b_desired: int = 0
    amount_a_min: int = 0
    amount_b_min: int = 0
    
    # Status
    is_active: bool = True
    created_at: int = 0
    updated_at: int = 0
    
    # Transaction info
    tx_hash: Optional[str] = None
    block_number: Optional[int] = None

@dataclass
class Tick:
    """Represents a price tick with aggregated liquidity"""
    index: int
    liquidity_net: int  # Net liquidity change at this tick
    liquidity_gross: int  # Total liquidity at this tick
    
    # Fee growth
    fee_growth_outside_a: float = 0.0
    fee_growth_outside_b: float = 0.0
    
    # Tick markers
    initialized: bool = False

@dataclass
class Pool:
    """Concentrated liquidity pool (Uniswap V3 style)"""
    address: str
    token_a: str
    token_b: str
    token_a_decimals: int
    token_b_decimals: int
    
    # Fee tier (in basis points)
    fee: int  # e.g., 500 = 0.05%, 3000 = 0.3%, 10000 = 1%
    
    # Current state
    current_tick: int = 0
    sqrt_price_x96: int = 0  # Square root of current price in Q96 format
    
    # Liquidity
    liquidity: int = 0  # Virtual liquidity in the pool
    
    # Fee tracking
    fee_growth_global_a: float = 0.0  # Cumulative fee growth per unit liquidity
    fee_growth_global_b: float = 0.0
    
    # Tick table
    ticks: Dict[int, Tick] = field(default_factory=dict)
    
    # Position tracking
    positions: Dict[str, Position] = field(default_factory=dict)
    
    # Observation (for TWAP)
    observation_index: int = 0
    observation_cardinality: int = 1
    observation_time: int = 0
    observation_tick_cumulative: int = 0
    
    # Metadata
    max_liquidity_per_tick: int = 0  # Max liquidity that can be added to a tick
    created_at: int = 0

@dataclass
class MintParams:
    """Parameters for adding liquidity"""
    token_a: str
    token_b: str
    fee: int
    tick_lower: int
    tick_upper: int
    amount_a_desired: int
    amount_b_desired: int
    amount_a_min: int
    amount_b_min: int
    recipient: str
    deadline: int

@dataclass
class MintResult:
    """Result of adding liquidity"""
    token_id: str
    liquidity: int
    amount_a: int
    amount_b: int
    fees_owed_a: int
    fees_owed_b: int

@dataclass
class SwapParams:
    """Parameters for a swap"""
    token_in: str
    token_out: str
    fee: int
    recipient: str
    amount_in: int
    amount_out_min: int
    sqrt_price_limit_x96: Optional[int] = None
    hook_data: Optional[bytes] = None

@dataclass
class SwapResult:
    """Result of a swap"""
    amount_in: int
    amount_out: int
    sqrt_price_x96_after: int
    tick_after: int
    liquidity_after: int
    fee_amount: int

@dataclass 
class FeeGrowth:
    """Track fee growth for a position"""
    fee_growth_inside_last: float
    position_liquidity: int

# Math constants for Q96 format (96-bit fixed point)
Q96 = 1 << 96
Q128 = 1 << 128
Q192 = Q96 * Q96

class ConcentratedLiquidityEngine:
    """
    Uniswap V3 style concentrated liquidity engine.
    
    Key concepts:
    - Positions have price ranges [tick_lower, tick_upper]
    - Liquidity is "virtual" - concentrated within the position's range
    - Fees are accumulated based on time spent in range and trade volume
    - Swaps cross ticks, moving active liquidity
    """
    
    # Fee tier constants (basis points)
    FEE_LOW = 100    # 0.01% - stablecoin pairs
    FEE_MEDIUM = 500  # 0.05%
    FEE_HIGH = 3000  # 0.3% - volatile pairs
    FEE_MAX = 10000  # 1%
    
    # Max tick
    MAX_TICK = 887272  # Defined by Uniswap
    
    def __init__(self, chain_id: int = 1):
        self.chain_id = chain_id
        self.pools: Dict[str, Pool] = {}
        self.positions_by_owner: Dict[str, Set[str]] = {}
        
        # Events for tracking
        self.events: List[Dict] = []
    
    def create_pool(
        self,
        token_a: str,
        token_b: str,
        fee: int,
        sqrt_price_x96: Optional[int] = None,
        tick: Optional[int] = None
    ) -> Pool:
        """Create a new concentrated liquidity pool"""
        # Sort tokens (token_a < token_b by address)
        if token_a.lower() > token_b.lower():
            token_a, token_b = token_b, token_a
        
        pool_key = self._pool_key(token_a, token_b, fee)
        
        if pool_key in self.pools:
            raise ValueError("Pool already exists")
        
        # Initialize sqrt_price if not provided
        if sqrt_price_x96 is None:
            # Initial price of 1 (sqrt(1) in Q96)
            sqrt_price_x96 = Q96
        
        if tick is None:
            tick = self._sqrt_price_to_tick(sqrt_price_x96)
        
        pool = Pool(
            address=pool_key,
            token_a=token_a,
            token_b=token_b,
            token_a_decimals=18,  # Default
            token_b_decimals=6,  # Default
            fee=fee,
            current_tick=tick,
            sqrt_price_x96=sqrt_price_x96,
            max_liquidity_per_tick=self._calculate_max_liquidity_per_tick(fee),
            created_at=int(time.time())
        )
        
        # Initialize tick 0 (and current tick if different)
        pool.ticks[0] = Tick(
            index=0,
            liquidity_net=0,
            liquidity_gross=0,
            initialized=True
        )
        
        if tick != 0:
            pool.ticks[tick] = Tick(
                index=tick,
                liquidity_net=0,
                liquidity_gross=0,
                initialized=True
            )
        
        self.pools[pool_key] = pool
        
        self._emit_event("PoolCreated", {
            "pool": pool_key,
            "token_a": token_a,
            "token_b": token_b,
            "fee": fee,
            "sqrt_price_x96": sqrt_price_x96,
            "tick": tick
        })
        
        return pool
    
    def add_liquidity(self, params: MintParams) -> MintResult:
        """
        Add liquidity to a position (mint new position or increase existing).
        """
        pool_key = self._pool_key(params.token_a, params.token_b, params.fee)
        pool = self.pools.get(pool_key)
        
        if not pool:
            raise ValueError("Pool does not exist")
        
        # Validate tick range
        if params.tick_lower >= params.tick_upper:
            raise ValueError("tick_lower must be less than tick_upper")
        if params.tick_lower < -self.MAX_TICK or params.tick_upper > self.MAX_TICK:
            raise ValueError("Tick out of range")
        
        # Calculate liquidity based on amounts
        # For a new position, we calculate how much liquidity the amounts represent
        sqrt_price_lower = self._tick_to_sqrt_price(params.tick_lower)
        sqrt_price_upper = self._tick_to_sqrt_price(params.tick_upper)
        sqrt_price_current = pool.sqrt_price_x96
        
        # Calculate amounts based on liquidity formula
        amount_a = 0
        amount_b = 0
        
        if params.tick_lower <= pool.current_tick < params.tick_upper:
            # Position is in range - use full amounts
            amount_a = params.amount_a_desired
            amount_b = params.amount_b_desired
        else:
            # Position is out of range - only one token is used
            if pool.current_tick < params.tick_lower:
                # Below range - only token A
                amount_a = params.amount_a_desired
            else:
                # Above range - only token B
                amount_b = params.amount_b_desired
        
        # Check slippage
        if amount_a < params.amount_a_min or amount_b < params.amount_b_min:
            raise ValueError("Slippage check failed")
        
        # Calculate liquidity from amounts
        liquidity = self._calculate_liquidity(
            amount_a, amount_b,
            sqrt_price_lower, sqrt_price_upper, sqrt_price_current
        )
        
        # Generate position ID
        position_id = self._generate_position_id(
            params.recipient, pool_key,
            params.tick_lower, params.tick_upper
        )
        
        # Get or create position
        if position_id in pool.positions:
            position = pool.positions[position_id]
            # Increase liquidity
            position.liquidity += liquidity
            position.amount_a_desired += amount_a
            position.amount_b_desired += amount_b
        else:
            # New position
            position = Position(
                id=position_id,
                owner=params.recipient,
                pool=pool_key,
                token_a=params.token_a,
                token_b=params.token_b,
                token_a_decimals=pool.token_a_decimals,
                token_b_decimals=pool.token_b_decimals,
                tick_lower=params.tick_lower,
                tick_upper=params.tick_upper,
                liquidity=liquidity,
                amount_a_desired=amount_a,
                amount_b_desired=amount_b,
                amount_a_min=params.amount_a_min,
                amount_b_min=params.amount_b_min,
                created_at=int(time.time()),
                updated_at=int(time.time())
            )
            pool.positions[position_id] = position
        
        # Update tick liquidity
        self._update_tick_liquidity(pool, params.tick_lower, params.tick_upper, liquidity, True)
        
        # Update pool liquidity
        if params.tick_lower <= pool.current_tick < params.tick_upper:
            pool.liquidity += liquidity
        
        # Track by owner
        if params.recipient.lower() not in self.positions_by_owner:
            self.positions_by_owner[params.recipient.lower()] = set()
        self.positions_by_owner[params.recipient.lower()].add(position_id)
        
        position.updated_at = int(time.time())
        
        self._emit_event("LiquidityAdded", {
            "position_id": position_id,
            "pool": pool_key,
            "owner": params.recipient,
            "liquidity": liquidity,
            "amount_a": amount_a,
            "amount_b": amount_b,
            "tick_lower": params.tick_lower,
            "tick_upper": params.tick_upper
        })
        
        return MintResult(
            token_id=position_id,
            liquidity=liquidity,
            amount_a=amount_a,
            amount_b=amount_b,
            fees_owed_a=position.fees_owed_a,
            fees_owed_b=position.Fees_owed_b
        )
    
    def remove_liquidity(self, position_id: str, recipient: str) -> Tuple[int, int]:
        """
        Remove liquidity from a position (burn).
        Returns amounts of token_a and token_b received.
        """
        position = self._get_position_by_id(position_id)
        
        if not position:
            raise ValueError("Position not found")
        
        if position.owner != recipient.lower():
            raise ValueError("Not authorized")
        
        pool = self.pools.get(position.pool)
        if not pool:
            raise ValueError("Pool not found")
        
        # Calculate fees owed
        self._collect_fees(position, pool)
        
        # Get amounts
        amount_a, amount_b = self._calculate_token_amounts(
            position, pool, position.liquidity
        )
        
        # Decrease liquidity in ticks
        self._update_tick_liquidity(
            pool, position.tick_lower, position.tick_upper,
            position.liquidity, False
        )
        
        # Update pool liquidity
        if position.tick_lower <= pool.current_tick < position.tick_upper:
            pool.liquidity -= position.liquidity
        
        # Update position
        position.liquidity = 0
        position.is_active = False
        position.updated_at = int(time.time())
        
        self._emit_event("LiquidityRemoved", {
            "position_id": position_id,
            "owner": position.owner,
            "amount_a": amount_a,
            "amount_b": amount_b
        })
        
        return amount_a, amount_b
    
    def swap(self, params: SwapParams) -> SwapResult:
        """
        Execute a swap.
        Moves price within the pool, crossing ticks as needed.
        """
        pool_key = self._pool_key(params.token_in, params.token_out, params.fee)
        pool = self.pools.get(pool_key)
        
        if not pool:
            raise ValueError("Pool does not exist")
        
        # Determine direction
        zero_for_one = pool.token_a.lower() == params.token_in.lower()
        
        # Initialize swap state
        sqrt_price_limit = params.sqrt_price_limit_x96
        if sqrt_price_limit is None:
            if zero_for_one:
                sqrt_price_limit = self._tick_to_sqrt_price(pool.current_tick - self.MAX_TICK)
            else:
                sqrt_price_limit = self._tick_to_sqrt_price(pool.current_tick + self.MAX_TICK)
        
        # Validate price limit direction
        if zero_for_one:
            if sqrt_price_limit >= pool.sqrt_price_x96:
                raise ValueError("sqrt_price_limit too high for zero for one")
        else:
            if sqrt_price_limit <= pool.sqrt_price_x96:
                raise ValueError("sqrt_price_limit too low for one for zero")
        
        # Execute swap
        amount_remaining = params.amount_in
        amount_specified_remaining = params.amount_in
        amount_calculated = 0
        tick_crossed = 0
        fee_amount = 0
        
        while amount_specified_remaining > 0:
            # Get current tick's data
            tick = pool.current_tick
            tick_data = pool.ticks.get(tick, Tick(tick, 0, 0, False))
            
            # Calculate next tick to cross
            sqrt_price_next = self._get_next_sqrt_price(
                pool.sqrt_price_x96, amount_specified_remaining,
                zero_for_one, pool.liquidity, tick_data, pool.fee
            )
            
            # Calculate amount needed to cross tick
            amount_to_cross = self._calculate_amount_to_cross_tick(
                pool.sqrt_price_x96, sqrt_price_next,
                amount_specified_remaining, zero_for_one, pool.liquidity
            )
            
            if amount_to_cross <= amount_remaining:
                # Cross the tick
                amount_remaining -= amount_to_cross
                amount_specified_remaining -= amount_to_cross
                
                # Update fee growth
                fee_amount += self._calculate_fee_amount(amount_to_cross, pool.fee)
                
                # Move to next tick
                tick_crossed += 1
                pool.current_tick = self._get_adjacent_tick(tick, zero_for_one)
                pool.sqrt_price_x96 = sqrt_price_next
            else:
                # Complete the swap at current price
                sqrt_price_target = sqrt_price_limit
                amount_out = self._calc_amount_out(
                    pool.sqrt_price_x96, sqrt_price_target,
                    amount_remaining, zero_for_one, pool.liquidity
                )
                
                amount_calculated += amount_out
                amount_remaining = 0
                
                pool.sqrt_price_x96 = sqrt_price_target
                break
        
        # Calculate amounts
        amount_in = params.amount_in - amount_remaining
        amount_out = amount_calculated
        
        # Update fee tracking
        fee_growth = fee_amount / pool.liquidity if pool.liquidity > 0 else 0
        if zero_for_one:
            pool.fee_growth_global_b += fee_growth
        else:
            pool.fee_growth_global_a += fee_growth
        
        # Update position fees
        self._update_position_fees(pool, pool.current_tick)
        
        self._emit_event("Swap", {
            "pool": pool_key,
            "recipient": params.recipient,
            "amount_in": amount_in,
            "amount_out": amount_out,
            "fee_amount": fee_amount,
            "sqrt_price_after": pool.sqrt_price_x96,
            "tick_after": pool.current_tick
        })
        
        # Verify slippage
        if amount_out < params.amount_out_min:
            raise ValueError(f"Slippage check failed: {amount_out} < {params.amount_out_min}")
        
        return SwapResult(
            amount_in=amount_in,
            amount_out=amount_out,
            sqrt_price_x96_after=pool.sqrt_price_x96,
            tick_after=pool.current_tick,
            liquidity_after=pool.liquidity,
            fee_amount=fee_amount
        )
    
    def collect_fees(self, position_id: str, recipient: str) -> Tuple[int, int]:
        """Collect accumulated fees from a position"""
        position = self._get_position_by_id(position_id)
        
        if not position:
            raise ValueError("Position not found")
        
        if position.owner != recipient.lower():
            raise ValueError("Not authorized")
        
        pool = self.pools.get(position.pool)
        if not pool:
            raise ValueError("Pool not found")
        
        # Calculate and collect fees
        self._collect_fees(position, pool)
        
        fees_a = position.fees_owed_a
        fees_b = position.Fees_owed_b
        
        position.fees_owed_a = 0
        position.Fees_owed_b = 0
        
        self._emit_event("FeesCollected", {
            "position_id": position_id,
            "owner": recipient,
            "fees_a": fees_a,
            "fees_b": fees_b
        })
        
        return fees_a, fees_b
    
    def get_position(self, position_id: str) -> Optional[Dict]:
        """Get position details"""
        position = self._get_position_by_id(position_id)
        if not position:
            return None
        
        pool = self.pools.get(position.pool)
        
        # Calculate current values
        if pool:
            amount_a, amount_b = self._calculate_token_amounts(
                position, pool, position.liquidity
            )
        else:
            amount_a, amount_b = 0, 0
        
        return {
            "id": position.id,
            "owner": position.owner,
            "pool": position.pool,
            "tick_lower": position.tick_lower,
            "tick_upper": position.tick_upper,
            "liquidity": position.liquidity,
            "amount_a": amount_a,
            "amount_b": amount_b,
            "fees_owed_a": position.fees_owed_a,
            "fees_owed_b": position.Fees_owed_b,
            "is_active": position.is_active,
            "created_at": position.created_at
        }
    
    def get_pool_tvl(self, pool_key: str) -> int:
        """Get total value locked in a pool (in token amounts)"""
        pool = self.pools.get(pool_key)
        if not pool:
            return 0
        
        return pool.liquidity
    
    def get_tick_data(self, pool_key: str, tick: int) -> Optional[Dict]:
        """Get tick data for a pool"""
        pool = self.pools.get(pool_key)
        if not pool:
            return None
        
        tick_data = pool.ticks.get(tick)
        if not tick_data:
            return None
        
        return {
            "index": tick_data.index,
            "liquidity_net": tick_data.liquidity_net,
            "liquidity_gross": tick_data.liquidity_gross,
            "fee_growth_outside_a": tick_data.fee_growth_outside_a,
            "fee_growth_outside_b": tick_data.fee_growth_outside_b
        }
    
    # ==================== Helper Methods ====================
    
    def _pool_key(self, token_a: str, token_b: str, fee: int) -> str:
        """Generate pool key"""
        # Normalize and sort tokens
        tokens = sorted([token_a.lower(), token_b.lower()])
        return f"{tokens[0]}_{tokens[1]}_{fee}"
    
    def _generate_position_id(
        self, owner: str, pool_key: str,
        tick_lower: int, tick_upper: int
    ) -> str:
        """Generate unique position ID"""
        data = f"{owner.lower()}_{pool_key}_{tick_lower}_{tick_upper}_{time.time()}"
        return "0x" + hashlib.sha256(data.encode()).hexdigest()[:40]
    
    def _get_position_by_id(self, position_id: str) -> Optional[Position]:
        """Find position by ID across all pools"""
        for pool in self.pools.values():
            if position_id in pool.positions:
                return pool.positions[position_id]
        return None
    
    def _calculate_max_liquidity_per_tick(self, fee: int) -> int:
        """Calculate max liquidity per tick based on fee tier"""
        # Higher fee = more liquidity can be concentrated
        fee_factor = fee / 10000
        return int(Q128 / ((self.MAX_TICK * 2) * fee_factor)) if fee_factor > 0 else Q128
    
    def _sqrt_price_to_tick(self, sqrt_price_x96: int) -> int:
        """Convert sqrt price (Q96) to tick"""
        return int(math.log(sqrt_price_x96 ** 2 / Q96) / math.log(1.0001))
    
    def _tick_to_sqrt_price(self, tick: int) -> int:
        """Convert tick to sqrt price (Q96)"""
        # sqrt(1.0001^tick)
        return int((1.0001 ** (tick / 2)) * Q96)
    
    def _get_next_sqrt_price(
        self, sqrt_price_x96: int, amount_remaining: int,
        zero_for_one: bool, liquidity: int,
        tick_data: Tick, fee: int
    ) -> int:
        """Calculate next sqrt price when crossing a tick"""
        # Simplified - in production would use full math
        if amount_remaining == 0:
            return sqrt_price_x96
        
        # Calculate price after considering amount and liquidity
        if zero_for_one:
            # Moving left in price space
            return sqrt_price_x96 - int(amount_remaining * Q96 / liquidity)
        else:
            # Moving right in price space
            return sqrt_price_x96 + int(amount_remaining * Q96 / liquidity)
    
    def _calculate_amount_to_cross_tick(
        self, sqrt_price: int, sqrt_price_next: int,
        amount: int, zero_for_one: bool, liquidity: int
    ) -> int:
        """Calculate amount needed to move from sqrt_price to sqrt_price_next"""
        return amount
    
    def _calc_amount_out(
        self, sqrt_price_current: int, sqrt_price_target: int,
        amount: int, zero_for_one: bool, liquidity: int
    ) -> int:
        """Calculate amount out for a swap"""
        # Constant product formula with fee
        if zero_for_one:
            # In token A, out token B
            delta_sqrt = sqrt_price_current - sqrt_price_target
            return int(delta_sqrt * liquidity / sqrt_price_target)
        else:
            # In token B, out token A
            delta_sqrt = sqrt_price_target - sqrt_price_current
            return int(delta_sqrt * liquidity / sqrt_price_target)
    
    def _calculate_fee_amount(self, amount: int, fee: int) -> int:
        """Calculate fee amount from swap amount"""
        return int(amount * fee / 10000)
    
    def _calculate_liquidity(
        self, amount_a: int, amount_b: int,
        sqrt_price_lower: int, sqrt_price_upper: int,
        sqrt_price_current: int
    ) -> int:
        """Calculate liquidity from token amounts"""
        if sqrt_price_lower == sqrt_price_upper:
            return 0
        
        # Simplified liquidity calculation
        # In production uses full decimal math
        if sqrt_price_lower <= sqrt_price_current <= sqrt_price_upper:
            # In range - both amounts contribute
            return int(amount_a * sqrt_price_current / Q96 + amount_b * Q96 / sqrt_price_current)
        elif sqrt_price_current < sqrt_price_lower:
            # Below range - only token A
            return int(amount_a * sqrt_price_lower / Q96)
        else:
            # Above range - only token B
            return int(amount_b * Q96 / sqrt_price_upper)
    
    def _calculate_token_amounts(
        self, position: Position, pool: Pool, liquidity: int
    ) -> Tuple[int, int]:
        """Calculate actual token amounts for a position"""
        if liquidity == 0:
            return 0, 0
        
        sqrt_price_lower = self._tick_to_sqrt_price(position.tick_lower)
        sqrt_price_upper = self._tick_to_sqrt_price(position.tick_upper)
        
        if position.tick_lower <= pool.current_tick < position.tick_upper:
            # In range
            amount_a = int(liquidity * (pool.sqrt_price_x96 - sqrt_price_lower) / (sqrt_price_lower * pool.sqrt_price_x96 / Q96))
            amount_b = int(liquidity * (sqrt_price_upper - pool.sqrt_price_x96) / (sqrt_price_upper / pool.sqrt_price_x96 * Q96))
        elif pool.current_tick < position.tick_lower:
            # Below range - all in token A
            amount_a = int(liquidity * (sqrt_price_upper - sqrt_price_lower) / (sqrt_price_lower * sqrt_price_upper / Q96))
            amount_b = 0
        else:
            # Above range - all in token B
            amount_a = 0
            amount_b = int(liquidity * (sqrt_price_upper - sqrt_price_lower) / Q96)
        
        return amount_a, amount_b
    
    def _update_tick_liquidity(
        self, pool: Pool, tick_lower: int, tick_upper: int,
        liquidity_delta: int, add: bool
    ):
        """Update liquidity in ticks when adding/removing position"""
        if liquidity_delta == 0:
            return
        
        direction = liquidity_delta if add else -liquidity_delta
        
        # Update lower tick
        if tick_lower not in pool.ticks:
            pool.ticks[tick_lower] = Tick(tick_lower, 0, 0, False)
        pool.ticks[tick_lower].liquidity_net += direction
        pool.ticks[tick_lower].liquidity_gross += abs(direction)
        
        # Update upper tick
        if tick_upper not in pool.ticks:
            pool.ticks[tick_upper] = Tick(tick_upper, 0, 0, False)
        pool.ticks[tick_upper].liquidity_net -= direction
        pool.ticks[tick_upper].liquidity_gross += abs(direction)
    
    def _update_position_fees(self, pool: Pool, current_tick: int):
        """Update fee tracking for all positions in range"""
        for position in pool.positions.values():
            if position.tick_lower <= current_tick < position.tick_upper:
                # Calculate fee growth inside position's range
                # Simplified - in production would track properly
                fee_per_liquidity = pool.fee_growth_global_a + pool.fee_growth_global_b
                
                if position.fee_growth_inside_last < fee_per_liquidity:
                    uncollected = int(
                        position.liquidity * 
                        (fee_per_liquidity - position.fee_growth_inside_last) / Q128
                    )
                    position.fees_owed_a += uncollected
                    position.Fees_owed_b += uncollected
                    position.fee_growth_inside_last = fee_per_liquidity
    
    def _collect_fees(self, position: Position, pool: Pool):
        """Collect accumulated fees for a position"""
        # First update fees
        self._update_position_fees(pool, pool.current_tick)
    
    def _get_adjacent_tick(self, tick: int, zero_for_one: bool) -> int:
        """Get adjacent tick"""
        if zero_for_one:
            return tick - 1
        else:
            return tick + 1
    
    def _emit_event(self, event_type: str, data: Dict):
        """Emit event for tracking"""
        self.events.append({
            "type": event_type,
            "data": data,
            "timestamp": int(time.time())
        })


async def main():
    """Test concentrated liquidity engine"""
    print("=== Concentrated Liquidity Engine Test ===")
    
    engine = ConcentratedLiquidityEngine(chain_id=1)
    
    # Create a pool
    pool = engine.create_pool(
        token_a="0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2",  # WETH
        token_b="0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48",  # USDC
        fee=3000,  # 0.3%
        sqrt_price_x96=None,
        tick=200000  # ~$2450 ETH price
    )
    
    print(f"Pool created: {pool.address}")
    print(f"Initial tick: {pool.current_tick}")
    print(f"Initial sqrt price: {pool.sqrt_price_x96}")
    
    # Add liquidity
    mint_params = MintParams(
        token_a=pool.token_a,
        token_b=pool.token_b,
        fee=3000,
        tick_lower=190000,  # ~$2000
        tick_upper=210000,  # ~$2900
        amount_a_desired=10 * 10**18,  # 10 ETH
        amount_b_desired=25000 * 10**6,  # 25000 USDC
        amount_a_min=0,
        amount_b_min=0,
        recipient="0x742d35Cc6634C0532925a3b844Bc9e7595f2bD12",
        deadline=int(time.time()) + 3600
    )
    
    result = engine.add_liquidity(mint_params)
    
    print(f"\nLiquidity added:")
    print(f"  Position ID: {result.token_id}")
    print(f"  Liquidity: {result.liquidity}")
    print(f"  Amount A: {result.amount_a / 10**18} ETH")
    print(f"  Amount B: {result.amount_b / 10**6} USDC")
    
    # Get position
    position = engine.get_position(result.token_id)
    print(f"\nPosition details:")
    print(f"  Owner: {position['owner']}")
    print(f"  Tick range: {position['tick_lower']} - {position['tick_upper']}")
    print(f"  Liquidity: {position['liquidity']}")
    
    # Execute a swap
    swap_params = SwapParams(
        token_in=pool.token_a,  # WETH in
        token_out=pool.token_b,  # USDC out
        fee=3000,
        recipient="0x742d35Cc6634C0532925a3b844Bc9e7595f2bD12",
        amount_in=1 * 10**18,  # 1 ETH
        amount_out_min=0
    )
    
    swap_result = engine.swap(swap_params)
    
    print(f"\nSwap executed:")
    print(f"  Amount in: {swap_result.amount_in / 10**18} ETH")
    print(f"  Amount out: {swap_result.amount_out / 10**6} USDC")
    print(f"  Fee: {swap_result.fee_amount / 10**6} USDC")
    print(f"  Tick after: {swap_result.tick_after}")
    
    print(f"\nEvents: {len(engine.events)}")


if __name__ == "__main__":
    asyncio.run(main())
