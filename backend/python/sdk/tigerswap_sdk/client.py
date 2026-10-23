"""
TigerSwap Client
Main client for interacting with the TigerSwap DEX.
"""

import os
from typing import Any, Dict, List, Optional

from web3 import Web3

from .chain import ChainManager
from .contracts import (
    ERC20_ABI,
    POOL_ABI,
    ROUTER_ABI,
    get_contract_address,
    is_deployed,
)
from .pools import Liquidity, Pool
from .staking import Staking
from .swap import Swap
from .tokens import TokenList
from .wallet import Wallet


class TigerSwapClient:
    """
    Main client for TigerSwap DEX operations
    """

    def __init__(
        self,
        rpc_url: Optional[str] = None,
        private_key: Optional[str] = None,
        api_url: str = "https://api.tigerswap.io",
        chain_id: int = 1,
    ):
        self.rpc_url = rpc_url or os.getenv("RPC_URL")
        if not self.rpc_url:
            raise ValueError(
                "an RPC URL is required: pass rpc_url or set the RPC_URL environment variable"
            )
        self.api_url = api_url
        self.chain_id = chain_id

        self.w3 = Web3(Web3.HTTPProvider(self.rpc_url))

        self.wallet = None
        if private_key:
            self.wallet = Wallet(private_key, self.w3)

        self.tokens = TokenList(self)
        self.swap = Swap(self)
        self.pool = Pool(self)
        self.liquidity = Liquidity(self)
        self.staking = Staking(self)
        self.chains = ChainManager(self)

    async def get_balance(self, address: str) -> Dict[str, Any]:
        """Get the native token balance of an address."""
        balance_wei = self.w3.eth.get_balance(self.w3.to_checksum_address(address))
        return {"wei": balance_wei, "eth": float(self.w3.from_wei(balance_wei, "ether"))}

    async def get_token_balance(
        self, token_address: str, address: str, chain_id: Optional[int] = None
    ) -> Dict[str, Any]:
        """Get the ERC-20 balance, symbol and decimals of an address."""
        chain_id = chain_id if chain_id is not None else self.chain_id
        token = self.w3.eth.contract(
            address=self.w3.to_checksum_address(token_address), abi=ERC20_ABI
        )
        holder = self.w3.to_checksum_address(address)
        raw = token.functions.balanceOf(holder).call()
        decimals = token.functions.decimals().call()
        symbol = token.functions.symbol().call()
        return {
            "raw": raw,
            "decimals": decimals,
            "symbol": symbol,
            "formatted": raw / (10**decimals) if decimals else float(raw),
        }

    async def get_gas_price(self) -> Dict[str, int]:
        """Get low/medium/high gas prices derived from the node's suggestion."""
        base = self.w3.eth.gas_price
        return {
            "low": base,
            "medium": int(base * 1.1),
            "high": int(base * 1.2),
        }

    async def estimate_gas(self, transaction: Dict) -> int:
        """Estimate the gas limit for a transaction."""
        return self.w3.eth.estimate_gas(transaction)

    async def send_transaction(self, transaction: Dict) -> str:
        """Sign and broadcast a transaction with the configured wallet."""
        if not self.wallet:
            raise ValueError("No wallet configured")
        return await self.wallet.send_transaction(transaction)

    async def get_transaction_receipt(self, tx_hash: str) -> Dict:
        """Fetch a transaction receipt."""
        return self.w3.eth.get_transaction_receipt(tx_hash)

    async def wait_for_transaction(self, tx_hash: str, timeout: int = 300) -> Dict:
        """Block until a transaction is confirmed or the timeout elapses."""
        return self.w3.eth.wait_for_transaction_receipt(tx_hash, timeout=timeout)

    def is_connected(self) -> bool:
        """Check if the RPC node is reachable."""
        return self.w3.is_connected()

    def get_chain_id(self) -> int:
        """Return the chain id reported by the node."""
        return self.w3.eth.chain_id

    def get_block_number(self) -> int:
        """Return the latest block number."""
        return self.w3.eth.block_number

    async def get_token_price_usd(
        self, token_address: str, quote_address: Optional[str] = None, chain_id: Optional[int] = None
    ) -> float:
        """Return the USD price of a token quoted through the TigerSwap router.

        `quote_address` defaults to the chain's USD stablecoin (USDC). The price
        is read on-chain; there is no hardcoded fallback.
        """
        chain_id = chain_id if chain_id is not None else self.chain_id
        if not is_deployed(chain_id):
            raise ValueError(f"TigerSwap is not deployed on chain {chain_id}")

        if quote_address is None:
            quote = self.tokens.get_token_by_symbol("USDC", chain_id)
            if quote is None:
                raise ValueError(
                    f"no default quote token for chain {chain_id}; pass quote_address"
                )
            quote_address = quote.address

        router = self.w3.eth.contract(
            address=self.w3.to_checksum_address(get_contract_address(chain_id, "router")),
            abi=ROUTER_ABI,
        )
        token = self.w3.eth.contract(
            address=self.w3.to_checksum_address(token_address), abi=ERC20_ABI
        )
        quote_token = self.w3.eth.contract(
            address=self.w3.to_checksum_address(quote_address), abi=ERC20_ABI
        )
        token_decimals = token.functions.decimals().call()
        quote_decimals = quote_token.functions.decimals().call()
        path = [
            self.w3.to_checksum_address(token_address),
            self.w3.to_checksum_address(quote_address),
        ]
        out = int(router.functions.getAmountsOut(10**token_decimals, path).call()[-1])
        return out / (10**quote_decimals)

    async def __aenter__(self):
        return self

    async def __aexit__(self, exc_type, exc_val, exc_tb):
        return None


def create_client(
    private_key: Optional[str] = None,
    rpc_url: Optional[str] = None,
    chain_id: int = 1,
) -> TigerSwapClient:
    """Factory function to create a client."""
    return TigerSwapClient(private_key=private_key, rpc_url=rpc_url, chain_id=chain_id)
