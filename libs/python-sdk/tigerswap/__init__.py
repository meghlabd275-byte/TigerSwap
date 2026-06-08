"""
TigerSwap Python SDK
Complete DEX functionality for trading, routing, orders, and more
"""

import asyncio
import json
from typing import Optional, List, Dict, Any
from dataclasses import dataclass
from enum import Enum
import aiohttp

class OrderType(str, Enum):
    LIMIT = "limit"
    STOP_LOSS = "stop_loss"
    TAKE_PROFIT = "take_profit"
    MARKET = "market"
    GTD = "gtd"
    IOC = "ioc"
    FOK = "fok"

class OrderStatus(str, Enum):
    PENDING = "pending"
    PARTIALLY_FILLED = "partially_filled"
    FILLED = "filled"
    CANCELLED = "cancelled"
    EXPIRED = "expired"

class Side(str, Enum):
    BUY = "buy"
    SELL = "sell"

@dataclass
class Token:
    address: str
    symbol: str
    name: str
    decimals: int
    chain_id: int
    logo_url: Optional[str] = None

@dataclass
class TokenPair:
    token_in: Token
    token_out: Token
    chain_id: int

@dataclass
class Quote:
    pair: TokenPair
    amount_in: str
    amount_out: str
    price_impact: str
    gas_estimate: str
    route: List[Dict[str, Any]]

@dataclass
class SwapRequest:
    token_in: str
    token_out: str
    amount_in: str
    amount_out_min: str
    recipient: Optional[str] = None
    slippage_tolerance: Optional[str] = None

@dataclass
class SwapResponse:
    tx_hash: str
    amount_in: str
    amount_out: str
    gas_used: str
    price_impact: str

@dataclass
class Order:
    id: str
    owner: str
    token_in: str
    token_out: str
    amount_in: str
    amount_out: str
    price: str
    stop_price: Optional[str] = None
    order_type: OrderType = OrderType.LIMIT
    side: Side = Side.BUY
    status: OrderStatus = OrderStatus.PENDING
    created_at: str = ""
    expires_at: Optional[str] = None
    filled_amount: str = "0"

@dataclass
class DCAPlan:
    id: str
    owner: str
    token_in: str
    token_out: str
    amount_per_execution: str
    interval_seconds: int
    executions_completed: int = 0
    max_executions: Optional[int] = None
    status: str = "active"
    next_execution: str = ""
    created_at: str = ""

@dataclass
class Position:
    id: str
    owner: str
    collateral_token: str
    index_token: str
    is_long: bool
    size: str
    collateral: str
    average_price: str
    unrealized_pnl: str
    liquidation_price: str
    status: str

@dataclass
class PoolInfo:
    address: str
    token0: str
    token1: str
    reserve0: str
    reserve1: str
    liquidity: str
    fee_tier: int

@dataclass
class TokenBalance:
    token: Token
    balance: str
    balance_raw: str
    allowance: str

@dataclass
class NetworkStatus:
    chain_id: int
    block_number: int
    synced: bool
    gas_price: str

class TigerSwapSDK:
    """TigerSwap Python SDK"""
    
    def __init__(self, base_url: str = "https://api.tigerswap.io", 
                 api_key: Optional[str] = None,
                 timeout: int = 30):
        self.base_url = base_url
        self.api_key = api_key
        self.timeout = timeout
        self._session: Optional[aiohttp.ClientSession] = None
    
    async def _request(self, method: str, endpoint: str, 
                      params: Optional[Dict] = None,
                      data: Optional[Dict] = None) -> Dict:
        url = f"{self.base_url}{endpoint}"
        headers = {"Content-Type": "application/json"}
        
        if self.api_key:
            headers["Authorization"] = f"Bearer {self.api_key}"
        
        async with aiohttp.ClientSession() as session:
            async with session.request(
                method, url, 
                params=params, 
                json=data,
                headers=headers,
                timeout=self.timeout
            ) as response:
                if response.status != 200:
                    raise Exception(f"API Error: {response.status}")
                
                result = await response.json()
                
                if not result.get("success"):
                    raise Exception(result.get("error", "Unknown error"))
                
                return result.get("data", {})
    
    # ============ Token & Balance ============
    
    async def get_balance(self, owner: str, token: str) -> TokenBalance:
        """Get token balance for an address"""
        data = await self._request("GET", f"/v1/balance/{owner}", 
                                params={"token": token})
        return TokenBalance(**data)
    
    async def get_balances(self, owner: str) -> List[TokenBalance]:
        """Get all token balances for an address"""
        data = await self._request("GET", f"/v1/balance/{owner}")
        return [TokenBalance(**b) for b in data]
    
    # ============ Quotes & Swap ============
    
    async def get_quote(self, token_in: str, token_out: str, 
                       amount_in: str) -> Quote:
        """Get swap quote"""
        data = await self._request("GET", "/v1/quote", 
                                params={
                                    "token_in": token_in,
                                    "token_out": token_out,
                                    "amount_in": amount_in
                                })
        return Quote(**data)
    
    async def swap(self, request: SwapRequest) -> SwapResponse:
        """Execute a swap"""
        data = await self._request("POST", "/v1/swap", 
                                data={
                                    "token_in": request.token_in,
                                    "token_out": request.token_out,
                                    "amount_in": request.amount_in,
                                    "amount_out_min": request.amount_out_min,
                                    "recipient": request.recipient,
                                    "slippage_tolerance": request.slippage_tolerance
                                })
        return SwapResponse(**data)
    
    # ============ Orders ============
    
    async def create_order(self, order: Order) -> Order:
        """Create a new order"""
        data = await self._request("POST", "/v1/orders", 
                                data=order.__dict__)
        return Order(**data)
    
    async def get_order(self, order_id: str) -> Order:
        """Get order by ID"""
        data = await self._request("GET", f"/v1/orders/{order_id}")
        return Order(**data)
    
    async def cancel_order(self, order_id: str) -> Order:
        """Cancel an order"""
        data = await self._request("DELETE", f"/v1/orders/{order_id}")
        return Order(**data)
    
    async def get_orders(self, owner: str) -> List[Order]:
        """Get all orders for an address"""
        data = await self._request("GET", "/v1/orders",
                                params={"owner": owner})
        return [Order(**o) for o in data]
    
    # ============ DCA ============
    
    async def create_dca_plan(self, plan: DCAPlan) -> DCAPlan:
        """Create a DCA plan"""
        data = await self._request("POST", "/v1/dca", 
                                data=plan.__dict__)
        return DCAPlan(**data)
    
    async def get_dca_plan(self, plan_id: str) -> DCAPlan:
        """Get DCA plan by ID"""
        data = await self._request("GET", f"/v1/dca/{plan_id}")
        return DCAPlan(**data)
    
    async def cancel_dca_plan(self, plan_id: str) -> DCAPlan:
        """Cancel a DCA plan"""
        data = await self._request("DELETE", f"/v1/dca/{plan_id}")
        return DCAPlan(**data)
    
    # ============ Perpetuals ============
    
    async def open_position(self, position: Position) -> Position:
        """Open a perpetual position"""
        data = await self._request("POST", "/v1/positions",
                                data=position.__dict__)
        return Position(**data)
    
    async def close_position(self, position_id: str) -> Position:
        """Close a perpetual position"""
        data = await self._request("DELETE", f"/v1/positions/{position_id}")
        return Position(**data)
    
    async def get_position(self, position_id: str) -> Position:
        """Get position by ID"""
        data = await self._request("GET", f"/v1/positions/{position_id}")
        return Position(**data)
    
    async def get_positions(self, owner: str) -> List[Position]:
        """Get all positions for an address"""
        data = await self._request("GET", "/v1/positions",
                                params={"owner": owner})
        return [Position(**p) for p in data]
    
    # ============ Pool Info ============
    
    async def get_pool(self, token_a: str, token_b: str) -> PoolInfo:
        """Get pool info"""
        data = await self._request("GET", f"/v1/pool/{token_a}/{token_b}")
        return PoolInfo(**data)
    
    # ============ Network ============
    
    async def get_network_status(self, chain_id: int) -> NetworkStatus:
        """Get network status"""
        data = await self._request("GET", f"/v1/network/{chain_id}")
        return NetworkStatus(**data)
    
    async def estimate_gas(self, request: SwapRequest) -> Dict:
        """Estimate gas for swap"""
        data = await self._request("POST", "/v1/gas/estimate",
                                data=request.__dict__)
        return data

# Sync wrapper
class SyncTigerSwapSDK:
    """Synchronous wrapper for TigerSwap SDK"""
    
    def __init__(self, *args, **kwargs):
        self._async_sdk = TigerSwapSDK(*args, **kwargs)
    
    def _run(self, coro):
        return asyncio.run(coro)
    
    def get_balance(self, owner: str, token: str) -> TokenBalance:
        return self._run(self._async_sdk.get_balance(owner, token))
    
    def get_quote(self, token_in: str, token_out: str, amount_in: str) -> Quote:
        return self._run(self._async_sdk.get_quote(token_in, token_out, amount_in))
    
    def swap(self, request: SwapRequest) -> SwapResponse:
        return self._run(self._async_sdk.swap(request))
    
    def create_order(self, order: Order) -> Order:
        return self._run(self._async_sdk.create_order(order))
    
    def get_order(self, order_id: str) -> Order:
        return self._run(self._async_sdk.get_order(order_id))
    
    def cancel_order(self, order_id: str) -> Order:
        return self._run(self._async_sdk.cancel_order(order_id))
    
    def create_dca_plan(self, plan: DCAPlan) -> DCAPlan:
        return self._run(self._async_sdk.create_dca_plan(plan))
    
    def get_pool(self, token_a: str, token_b: str) -> PoolInfo:
        return self._run(self._async_sdk.get_pool(token_a, token_b))

__all__ = [
    "TigerSwapSDK",
    "SyncTigerSwapSDK",
    "Token",
    "TokenPair", 
    "Quote",
    "SwapRequest",
    "SwapResponse",
    "Order",
    "DCAPlan",
    "Position",
    "PoolInfo",
    "TokenBalance",
    "NetworkStatus",
    "OrderType",
    "OrderStatus",
    "Side",
]