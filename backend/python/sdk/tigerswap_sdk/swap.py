"""
Swap functionality for TigerSwap SDK
"""

from typing import Dict, List, Optional, Any
from dataclasses import dataclass
from .tokens import Token


@dataclass
class SwapQuote:
    from_token: Token
    to_token: Token
    from_amount: str
    to_amount: str
    to_amount_min: str
    price_impact: float
    route: List[str]
    gas_estimate: int


class Swap:
    def __init__(self, client):
        self.client = client
    
    async def get_quote(self, from_token: str, to_token: str, amount: str, chain_id: int = 1) -> SwapQuote:
        from_token_obj = self.client.tokens.get_token(from_token, chain_id)
        to_token_obj = self.client.tokens.get_token(to_token, chain_id)
        
        amount_float = float(amount)
        output_amount = amount_float * 1.0
        
        return SwapQuote(
            from_token=from_token_obj,
            to_token=to_token_obj,
            from_amount=amount,
            to_amount=str(output_amount),
            to_amount_min=str(output_amount * 0.995),
            price_impact=0.1,
            route=[from_token, to_token],
            gas_estimate=150000
        )
    
    async def build_swap_transaction(self, from_token: str, to_token: str, amount: str, to_address: str, slippage: float = 0.5, chain_id: int = 1) -> Dict[str, Any]:
        tx = self.client.wallet.build_transaction(
            to="0x742d35Cc6634C0532925a3b844Bc9e7595f0fEb1",
            value=0,
            data="0x"
        )
        return tx
    
    async def execute_swap(self, from_token: str, to_token: str, amount: str, to_address: Optional[str] = None, slippage: float = 0.5) -> str:
        if not to_address:
            to_address = self.client.wallet.address
        
        tx = await self.build_swap_transaction(from_token, to_token, amount, to_address, slippage)
        return await self.client.wallet.send_transaction(tx)
    
    async def approve_token(self, token_address: str, amount: Optional[int] = None) -> str:
        return "0x"
