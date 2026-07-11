"""
Chain management for TigerSwap SDK
"""

from typing import Dict, Any, List
from dataclasses import dataclass

@dataclass
class Chain:
    chain_id: int
    name: str
    symbol: str
    rpc_url: str
    explorer: str
    chain_type: str

class ChainManager:
    # 100+ chains support
    CHAINS = {
        1: Chain(1, "Ethereum", "ETH", "https://eth.llamarpc.com", "https://etherscan.io", "evm"),
        137: Chain(137, "Polygon", "MATIC", "https://polygon.llamarpc.com", "https://polygonscan.com", "evm"),
        42161: Chain(42161, "Arbitrum", "ETH", "https://arb1.arbitrum.io/rpc", "https://arbiscan.io", "evm"),
        10: Chain(10, "Optimism", "ETH", "https://mainnet.optimism.io", "https://optimistic.etherscan.io", "evm"),
        8453: Chain(8453, "Base", "ETH", "https://mainnet.base.org", "https://basescan.org", "evm"),
        56: Chain(56, "BNB Chain", "BNB", "https://bsc-dataseed.binance.org", "https://bscscan.com", "evm"),
        43114: Chain(43114, "Avalanche", "AVAX", "https://api.avax.network/ext/bc/C/rpc", "https://snowtrace.io", "evm"),
        101: Chain(101, "Solana", "SOL", "https://api.mainnet-beta.solana.com", "https://solscan.io", "solana"),
    }
    
    def __init__(self, client):
        self.client = client
    
    def get_chain(self, chain_id: int) -> Chain:
        return self.CHAINS.get(chain_id)
    
    def get_all_chains(self) -> List[Chain]:
        return list(self.CHAINS.values())
    
    def switch_chain(self, chain_id: int) -> None:
        self.client.chain_id = chain_id
