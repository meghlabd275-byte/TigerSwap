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


def min_out_per_hop(amounts: List[int], slippage_bps: int) -> List[int]:
    """Slippage-bounded minimum output for every hop of a path.

    `amounts` is the router's `getAmountsOut` result, whose length is one more
    than the number of hops. The returned list has one bound per hop, so a
    multi-hop execution cannot be sandwiched at an intermediate step.
    """
    if slippage_bps < 0 or slippage_bps > MAX_SLIPPAGE_BPS:
        raise ValueError(f"slippage_bps must be between 0 and {MAX_SLIPPAGE_BPS}")
    if len(amounts) < 2:
        raise ValueError("amounts must contain at least one hop")
    keep = 10_000 - slippage_bps
    return [int(a) * keep // 10_000 for a in amounts[1:]]


def validate_hops(hops: List["SwapRouteHop"]) -> None:
    """Check a multi-hop route is well formed before it is encoded.

    A route needs at least two hops and each hop must end on the token the next
    hop starts from; a disconnected route would revert on-chain, so it is
    rejected here with a clear error instead.
    """
    if len(hops) < 2:
        raise ValueError("a multi-hop route needs at least two hops")
    for index in range(len(hops) - 1):
        if hops[index].token_out.lower() != hops[index + 1].token_in.lower():
            raise ValueError(f"hop {index} does not connect to hop {index + 1}")


@dataclass
class SwapRouteHop:
    """One executable hop: a token pair and the family to trade it on."""

    token_in: str
    token_out: str
    family: int
    pool: str


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

    async def build_multihop_swap_transaction(
        self,
        hops: List[SwapRouteHop],
        amount: str,
        to_address: str,
        slippage: float = 0.5,
        chain_id: int = 1,
        deadline: Optional[int] = None,
    ) -> Dict[str, Any]:
        """Build an unsigned multi-hop TigerSwap router transaction.

        Each hop carries its own pool family, so a route that spans the xyK,
        stable and weighted families executes atomically. The per-hop minimum is
        the slippage-bounded output of that hop, so an intermediate hop cannot be
        sandwiched.
        """
        if self.client.wallet is None:
            raise ValueError("No wallet configured")
        validate_hops(hops)
        slippage_bps = int(round(slippage * 100))
        if slippage_bps < 0 or slippage_bps > MAX_SLIPPAGE_BPS:
            raise ValueError(f"slippage must be between 0 and {MAX_SLIPPAGE_BPS / 100}")

        tokens = [self.client.w3.to_checksum_address(hops[0].token_in)]
        tokens.extend(self.client.w3.to_checksum_address(h.token_out) for h in hops)
        families = [h.family for h in hops]
        amount_in = int(amount)

        router = self._router(chain_id)
        amounts = router.functions.getAmountsOut(amount_in, tokens).call()
        mins = min_out_per_hop([int(a) for a in amounts], slippage_bps)
        amount_out_min = mins[-1]

        tx = router.functions.swapExactTokensForTokensMultihop(
            amount_in,
            amount_out_min,
            tokens,
            families,
            mins,
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

    async def build_split_swap_transaction(
        self,
        legs: List[List[SwapRouteHop]],
        leg_amounts: List[str],
        amount: str,
        to_address: str,
        slippage: float = 0.5,
        chain_id: int = 1,
        deadline: Optional[int] = None,
    ) -> Dict[str, Any]:
        """Build an unsigned split-order TigerSwap router transaction.

        The order is divided across independent legs so no single pool takes the
        whole price impact. The legs must be pool-disjoint (the router's job) and
        their inputs must sum to `amount`.
        """
        if self.client.wallet is None:
            raise ValueError("No wallet configured")
        if len(legs) != len(leg_amounts):
            raise ValueError("legs and leg_amounts must have the same length")
        if len(legs) < 2:
            raise ValueError("a split needs at least two legs")
        slippage_bps = int(round(slippage * 100))
        if slippage_bps < 0 or slippage_bps > MAX_SLIPPAGE_BPS:
            raise ValueError(f"slippage must be between 0 and {MAX_SLIPPAGE_BPS / 100}")

        token_paths: List[List[str]] = []
        family_paths: List[List[int]] = []
        min_paths: List[List[int]] = []
        inputs = [int(a) for a in leg_amounts]
        if sum(inputs) != int(amount):
            raise ValueError("leg inputs must sum to amount")

        router = self._router(chain_id)
        for leg in legs:
            validate_hops(leg)
            tokens = [self.client.w3.to_checksum_address(leg[0].token_in)]
            tokens.extend(self.client.w3.to_checksum_address(h.token_out) for h in leg)
            amounts = router.functions.getAmountsOut(inputs[len(token_paths)], tokens).call()
            token_paths.append(tokens)
            family_paths.append([h.family for h in leg])
            min_paths.append(min_out_per_hop([int(a) for a in amounts], slippage_bps))

        total_min = sum(mins[-1] for mins in min_paths)
        tx = router.functions.swapExactTokensForTokensSplit(
            int(amount),
            total_min,
            token_paths,
            family_paths,
            inputs,
            min_paths,
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
