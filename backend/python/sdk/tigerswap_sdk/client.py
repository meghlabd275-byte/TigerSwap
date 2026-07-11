"""
TigerSwap Client
Main client for interacting with the TigerSwap DEX
"""

import os
import asyncio
from typing import Optional, Dict, List, Any
from eth_account import Account
from web3 import Web3
from .wallet import Wallet
from .tokens import TokenList
from .swap import Swap
from .pools import Liquidity
from .perpetuals import Perpetuals
from .staking import Staking
from .chain import ChainManager


class TigerSwapClient:
    """
    Main client for TigerSwap DEX operations
    """
    
    def __init__(
        self,
        rpc_url: Optional[str] = None,
        private_key: Optional[str] = None,
        api_url: str = "https://api.tigerswap.io",
        chain_id: int = 1
    ):
        """
        Initialize TigerSwap client
        
        Args:
            rpc_url: RPC endpoint URL
            private_key: Private key for transactions
            api_url: API server URL
            chain_id: Chain ID (default: Ethereum mainnet)
        """
        self.rpc_url = rpc_url or os.getenv("RPC_URL", "https://eth.llamarpc.com")
        self.api_url = api_url
        self.chain_id = chain_id
        
        # Initialize Web3
        self.w3 = Web3(Web3.HTTPProvider(self.rpc_url))
        
        # Initialize wallet if private key provided
        self.wallet = None
        if private_key:
            self.wallet = Wallet(private_key, self.w3)
        
        # Initialize components
        self.tokens = TokenList(self)
        self.swap = Swap(self)
        self.liquidity = Liquidity(self)
        self.perpetuals = Perpetuals(self)
        self.staking = Staking(self)
        self.chains = ChainManager(self)
    
    async def get_balance(self, address: str) -> Dict[str, Any]:
        """
        Get native token balance
        
        Args:
            address: Wallet address
            
        Returns:
            Balance in wei and ETH
        """
        balance_wei = self.w3.eth.get_balance(address)
        return {
            "wei": balance_wei,
            "eth": self.w3.from_wei(balance_wei, "ether")
        }
    
    async def get_token_balance(self, token_address: str, address: str) -> Dict[str, Any]:
        """
        Get ERC20 token balance
        
        Args:
            token_address: Token contract address
            address: Wallet address
            
        Returns:
            Balance data
        """
        # This would interact with ERC20 contract
        pass
    
    async def get_gas_price(self) -> Dict[str, int]:
        """
        Get current gas prices
        
        Returns:
            Gas prices in wei
        """
        return {
            "low": self.w3.eth.gas_price,
            "medium": int(self.w3.eth.gas_price * 1.1),
            "high": int(self.w3.eth.gas_price * 1.2)
        }
    
    async def estimate_gas(self, transaction: Dict) -> int:
        """
        Estimate gas for transaction
        
        Args:
            transaction: Transaction dictionary
            
        Returns:
            Estimated gas limit
        """
        return self.w3.eth.estimate_gas(transaction)
    
    async def send_transaction(self, transaction: Dict) -> str:
        """
        Send a transaction
        
        Args:
            transaction: Transaction dictionary
            
        Returns:
            Transaction hash
        """
        if not self.wallet:
            raise ValueError("No wallet configured")
        
        return await self.wallet.send_transaction(transaction)
    
    async def get_transaction_receipt(self, tx_hash: str) -> Dict:
        """
        Get transaction receipt
        
        Args:
            tx_hash: Transaction hash
            
        Returns:
            Receipt data
        """
        return self.w3.eth.get_transaction_receipt(tx_hash)
    
    async def wait_for_transaction(self, tx_hash: str, timeout: int = 300) -> Dict:
        """
        Wait for transaction to be confirmed
        
        Args:
            tx_hash: Transaction hash
            timeout: Timeout in seconds
            
        Returns:
            Receipt data
        """
        return self.w3.eth.wait_for_transaction_receipt(tx_hash, timeout=timeout)
    
    def is_connected(self) -> bool:
        """
        Check if connected to node
        
        Returns:
            Connection status
        """
        return self.w3.is_connected()
    
    def get_chain_id(self) -> int:
        """
        Get current chain ID
        
        Returns:
            Chain ID
        """
        return self.w3.eth.chain_id
    
    def get_block_number(self) -> int:
        """
        Get current block number
        
        Returns:
            Block number
        """
        return self.w3.eth.block_number
    
    async def get_eth_price(self) -> float:
        """
        Get ETH price in USD
        
        Returns:
            ETH price
        """
        # This would call price oracle
        return 3450.0
    
    # Context manager support
    async def __aenter__(self):
        return self
    
    async def __aexit__(self, exc_type, exc_val, exc_tb):
        pass


def create_client(
    private_key: str,
    rpc_url: Optional[str] = None,
    chain_id: int = 1
) -> TigerSwapClient:
    """
    Factory function to create client
    
    Args:
        private_key: Private key
        rpc_url: RPC URL
        chain_id: Chain ID
        
    Returns:
        TigerSwapClient instance
    """
    return TigerSwapClient(
        private_key=private_key,
        rpc_url=rpc_url,
        chain_id=chain_id
    )
