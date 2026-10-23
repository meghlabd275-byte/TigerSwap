"""
Swap functionality for TigerSwap SDK.

Quotes come from the on-chain TigerSwap router (getAmountsOut) and swaps are
executed against the same router. No third-party aggregator is used.
"""

from dataclasses import dataclass, field
from typing import Any, Dict, List, Optional

from .contracts import ERC20_ABI, ROUTER_ABI, DeploymentError, get_contract_address
from .tokens import Token

DEFAULT_SLIPPAGE_BPS = 50  # 0.5%
MAX_SLIPPAGE_BPS = 500  # 5%


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
    amounts: List[int] = field(default_factory=list)


class Swap:
    def __init__(self, client):
        self.client = client

    def _router(self, chain_id: int):
        address = get_contract_address(chain_id, "router")
        return self.client.w3.eth.contract(
            address=self.client.w3.to_checksum_address(address), abi=ROUTER_ABI
        )

    def _resolve_path(self, from_token: str, to_token: str, chain_id: int) -> List[str]:
        if from_token.lower().startswith("0x") and to_token.lower().startswith("0x"):
            path = [from_token, to_token]
        else:
            token_in = self.client.tokens.get_token_by_symbol(from_token, chain_id)
            token_out = self.client.tokens.get_token_by_symbol(to_token, chain_id)
            if token_in is None or token_out is None:
                raise ValueError(
                    f"unknown token symbol: {from_token} -> {to_token} on chain {chain_id}"
                )
            path = [token_in.address, token_out.address]
        return [self.client.w3.to_checksum_address(a) for a in path]

    async def get_quote(
        self,
        from_token: str,
        to_token: str,
        amount: str,
        chain_id: int = 1,
        slippage_bps: int = DEFAULT_SLIPPAGE_BPS,
    ) -> SwapQuote:
        """Quote a swap by calling the TigerSwap router on-chain."""
        if slippage_bps < 0 or slippage_bps > MAX_SLIPPAGE_BPS:
            raise ValueError(f"slippage_bps must be between 0 and {MAX_SLIPPAGE_BPS}")

        path = self._resolve_path(from_token, to_token, chain_id)
        token_in = self.client.tokens.get_token(path[0], chain_id)
        token_out = self.client.tokens.get_token(path[1], chain_id)
        if token_in is None or token_out is None:
            raise ValueError("token metadata not found for the swap path")

        amount_in = int(amount)
        router = self._router(chain_id)
        amounts = router.functions.getAmountsOut(amount_in, path).call()
        amount_out = int(amounts[-1])
        amount_out_min = amount_out * (10_000 - slippage_bps) // 10_000

        # Spot price before the trade, used for price-impact estimation.
        probe = max(amount_in // 1000, 1)
        spot_amounts = router.functions.getAmountsOut(probe, path).call()
        spot_out = int(spot_amounts[-1])
        price_impact = 0.0
        if spot_out > 0:
            expected = spot_out * amount_in / probe
            if expected > 0:
                price_impact = max(0.0, (expected - amount_out) / expected * 100)

        try:
            gas_estimate = router.functions.swapExactTokensForTokens(
                amount_in, amount_out_min, path, self.client.wallet.address if self.client.wallet else path[-1], 2**64 - 1
            ).estimate_gas({"from": self.client.wallet.address}) if self.client.wallet else 0
        except Exception:
            gas_estimate = 0

        return SwapQuote(
            from_token=token_in,
            to_token=token_out,
            from_amount=str(amount_in),
            to_amount=str(amount_out),
            to_amount_min=str(amount_out_min),
            price_impact=round(price_impact, 4),
            route=path,
            gas_estimate=gas_estimate,
            amounts=[int(a) for a in amounts],
        )

    async def approve_token(self, token_address: str, amount: Optional[int] = None, chain_id: int = 1) -> str:
        """Approve the TigerSwap router to spend an ERC-20 token."""
        if self.client.wallet is None:
            raise ValueError("No wallet configured")
        if amount is None:
            amount = 2**256 - 1
        spender = get_contract_address(chain_id, "router")
        token = self.client.w3.eth.contract(
            address=self.client.w3.to_checksum_address(token_address), abi=ERC20_ABI
        )
        tx = token.functions.approve(
            self.client.w3.to_checksum_address(spender), amount
        ).build_transaction(
            {
                "from": self.client.wallet.address,
                "nonce": self.client.w3.eth.get_transaction_count(self.client.wallet.address),
                "chainId": chain_id,
            }
        )
        return await self.client.wallet.send_transaction(tx)

    async def build_swap_transaction(
        self,
        from_token: str,
        to_token: str,
        amount: str,
        to_address: str,
        slippage: float = 0.5,
        chain_id: int = 1,
        deadline: Optional[int] = None,
    ) -> Dict[str, Any]:
        """Build an unsigned TigerSwap router swap transaction."""
        if self.client.wallet is None:
            raise ValueError("No wallet configured")
        path = self._resolve_path(from_token, to_token, chain_id)
        quote = await self.get_quote(
            from_token, to_token, amount, chain_id, int(slippage * 100)
        )
        router = self._router(chain_id)
        tx = router.functions.swapExactTokensForTokens(
            int(quote.from_amount),
            int(quote.to_amount_min),
            path,
            self.client.w3.to_checksum_address(to_address),
            deadline if deadline is not None else self.client.w3.eth.get_block("latest")["timestamp"] + 1200,
        ).build_transaction(
            {
                "from": self.client.wallet.address,
                "nonce": self.client.w3.eth.get_transaction_count(self.client.wallet.address),
                "chainId": chain_id,
            }
        )
        return tx

    async def execute_swap(
        self,
        from_token: str,
        to_token: str,
        amount: str,
        to_address: Optional[str] = None,
        slippage: float = 0.5,
    ) -> str:
        """Execute a swap through the TigerSwap router."""
        if self.client.wallet is None:
            raise ValueError("No wallet configured")
        recipient = to_address or self.client.wallet.address
        tx = await self.build_swap_transaction(
            from_token, to_token, amount, recipient, slippage, self.client.chain_id
        )
        return await self.client.wallet.send_transaction(tx)
