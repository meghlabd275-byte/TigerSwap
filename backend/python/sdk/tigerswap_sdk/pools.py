"""
Pools and liquidity for TigerSwap SDK.

Reserves, LP balances and quotes are read from the on-chain TigerSwap factory
and pool contracts. No reserve or position is fabricated locally.
"""

from dataclasses import dataclass
from typing import Any, Dict, List, Optional

from .contracts import (
    ERC20_ABI,
    FACTORY_ABI,
    POOL_ABI,
    ROUTER_ABI,
    get_contract_address,
)


@dataclass
class PoolInfo:
    """A TigerSwap liquidity pool and its reserves."""

    address: str
    token0: str
    token1: str
    reserve0: int
    reserve1: int
    total_supply: int
    block_timestamp_last: int

    @property
    def price_token1_per_token0(self) -> float:
        """Spot price of token0 quoted in token1, from the reserves."""
        if self.reserve0 == 0:
            return 0.0
        return self.reserve1 / self.reserve0

    @property
    def price_token0_per_token1(self) -> float:
        if self.reserve1 == 0:
            return 0.0
        return self.reserve0 / self.reserve1


class Pool:
    """Read and write access to TigerSwap pools."""

    def __init__(self, client):
        self.client = client

    def _factory(self, chain_id: int):
        address = get_contract_address(chain_id, "factory")
        return self.client.w3.eth.contract(
            address=self.client.w3.to_checksum_address(address), abi=FACTORY_ABI
        )

    def _router(self, chain_id: int):
        address = get_contract_address(chain_id, "router")
        return self.client.w3.eth.contract(
            address=self.client.w3.to_checksum_address(address), abi=ROUTER_ABI
        )

    def _pool_contract(self, pool_address: str):
        return self.client.w3.eth.contract(
            address=self.client.w3.to_checksum_address(pool_address), abi=POOL_ABI
        )

    def get_pool_address(self, token_a: str, token_b: str, chain_id: int = 1) -> str:
        """Look up the pool address for a token pair from the factory."""
        factory = self._factory(chain_id)
        address = factory.functions.getPoolAddress(
            self.client.w3.to_checksum_address(token_a),
            self.client.w3.to_checksum_address(token_b),
        ).call()
        if int(address, 16) == 0:
            raise ValueError(f"no TigerSwap pool for {token_a}/{token_b} on chain {chain_id}")
        return address

    async def get_pool(
        self, token_a: str, token_b: str, chain_id: int = 1
    ) -> PoolInfo:
        """Return a pool's tokens, reserves and LP supply."""
        pool_address = self.get_pool_address(token_a, token_b, chain_id)
        contract = self._pool_contract(pool_address)

        reserve0, reserve1, timestamp = contract.functions.getReserves().call()

        return PoolInfo(
            address=pool_address,
            token0=contract.functions.token0().call(),
            token1=contract.functions.token1().call(),
            reserve0=int(reserve0),
            reserve1=int(reserve1),
            total_supply=int(contract.functions.totalSupply().call()),
            block_timestamp_last=int(timestamp),
        )

    async def get_reserves(
        self, token_a: str, token_b: str, chain_id: int = 1
    ) -> Dict[str, int]:
        """Raw reserves of a pool."""
        pool = await self.get_pool(token_a, token_b, chain_id)
        return {
            "reserve0": pool.reserve0,
            "reserve1": pool.reserve1,
            "total_supply": pool.total_supply,
        }

    async def get_pool_count(self, chain_id: int = 1) -> int:
        """Number of pools created by the factory."""
        return self._factory(chain_id).functions.allPoolsLength().call()

    async def add_liquidity(
        self,
        token_a: str,
        token_b: str,
        amount_a_desired: int,
        amount_b_desired: int,
        amount_a_min: int = 0,
        amount_b_min: int = 0,
        chain_id: int = 1,
        deadline: Optional[int] = None,
    ) -> str:
        """Add liquidity to a TigerSwap pool through the router."""
        if self.client.wallet is None:
            raise ValueError("No wallet configured")

        if deadline is None:
            deadline = self.client.w3.eth.get_block("latest")["timestamp"] + 1200

        router = self._router(chain_id)
        tx = router.functions.addLiquidity(
            self.client.w3.to_checksum_address(token_a),
            self.client.w3.to_checksum_address(token_b),
            amount_a_desired,
            amount_b_desired,
            amount_a_min,
            amount_b_min,
            self.client.wallet.address,
            deadline,
        ).build_transaction(
            {
                "from": self.client.wallet.address,
                "nonce": self.client.w3.eth.get_transaction_count(self.client.wallet.address),
                "chainId": chain_id,
            }
        )
        return await self.client.wallet.send_transaction(tx)

    async def remove_liquidity(
        self,
        token_a: str,
        token_b: str,
        liquidity: int,
        amount_a_min: int = 0,
        amount_b_min: int = 0,
        chain_id: int = 1,
        deadline: Optional[int] = None,
    ) -> str:
        """Remove liquidity from a TigerSwap pool through the router."""
        if self.client.wallet is None:
            raise ValueError("No wallet configured")

        if deadline is None:
            deadline = self.client.w3.eth.get_block("latest")["timestamp"] + 1200

        router = self._router(chain_id)
        tx = router.functions.removeLiquidity(
            self.client.w3.to_checksum_address(token_a),
            self.client.w3.to_checksum_address(token_b),
            liquidity,
            amount_a_min,
            amount_b_min,
            self.client.wallet.address,
            deadline,
        ).build_transaction(
            {
                "from": self.client.wallet.address,
                "nonce": self.client.w3.eth.get_transaction_count(self.client.wallet.address),
                "chainId": chain_id,
            }
        )
        return await self.client.wallet.send_transaction(tx)


class Liquidity:
    """LP position queries."""

    def __init__(self, client):
        self.client = client
        self._pool = Pool(client)

    async def get_positions(
        self, owner: str, pairs: List[tuple], chain_id: int = 1
    ) -> List[Dict[str, Any]]:
        """Return LP balances for `owner` across the given token pairs.

        Each entry in `pairs` is a (token_a, token_b) address tuple. Pools with
        a zero balance are omitted.
        """
        holder = self.client.w3.to_checksum_address(owner)
        positions: List[Dict[str, Any]] = []

        for token_a, token_b in pairs:
            pool = await self._pool.get_pool(token_a, token_b, chain_id)
            contract = self.client.w3.eth.contract(
                address=self.client.w3.to_checksum_address(pool.address), abi=POOL_ABI
            )
            balance = int(contract.functions.balanceOf(holder).call())
            if balance == 0:
                continue

            # Integer maths keeps the underlying shares exact; the float is
            # only for display.
            if pool.total_supply == 0:
                token0_share = 0
                token1_share = 0
                share = 0.0
            else:
                token0_share = pool.reserve0 * balance // pool.total_supply
                token1_share = pool.reserve1 * balance // pool.total_supply
                share = balance / pool.total_supply

            positions.append(
                {
                    "pool": pool.address,
                    "token0": pool.token0,
                    "token1": pool.token1,
                    "lp_balance": balance,
                    "total_supply": pool.total_supply,
                    "share": round(share, 12),
                    "token0_share": token0_share,
                    "token1_share": token1_share,
                }
            )

        return positions

    async def get_position(
        self, owner: str, token_a: str, token_b: str, chain_id: int = 1
    ) -> Optional[Dict[str, Any]]:
        """Return one LP position, or None when the balance is zero."""
        positions = await self.get_positions(owner, [(token_a, token_b)], chain_id)
        return positions[0] if positions else None
