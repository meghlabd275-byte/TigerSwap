"""
Wallet management for TigerSwap SDK
"""

import os
from typing import Optional, Dict, Any
from eth_account import Account
from eth_typing import ChecksumAddress
from web3 import Web3
from web3.eth import Eth
from web3.contract import ContractFunction


class Wallet:
    """
    Wallet management for transactions and signing
    """
    
    def __init__(self, private_key: str, w3: Web3):
        if private_key.startswith("0x"):
            private_key = private_key[2:]
        
        self.account = Account.from_key(f"0x{private_key}")
        self.w3 = w3
        self.address = self.account.address
    
    @property
    def checksum_address(self) -> ChecksumAddress:
        return self.w3.to_checksum_address(self.address)
    
    def sign_transaction(self, transaction: Dict) -> Dict:
        return self.account.sign_transaction(transaction)
    
    async def send_transaction(self, transaction: Dict) -> str:
        signed_tx = self.sign_transaction(transaction)
        tx_hash = self.w3.eth.send_raw_transaction(signed_tx.raw_transaction)
        return tx_hash.hex()
    
    def build_transaction(self, to: str, value: int = 0, data: str = "0x", gas: Optional[int] = None) -> Dict:
        tx = {
            "from": self.checksum_address,
            "to": self.w3.to_checksum_address(to),
            "value": value,
            "data": data,
            "chainId": self.w3.eth.chain_id,
            "nonce": self.w3.eth.get_transaction_count(self.checksum_address),
            "gasPrice": self.w3.eth.gas_price,
        }
        
        if gas:
            tx["gas"] = gas
        
        return tx
    
    def build_contract_transaction(
        self,
        contract: Any,
        function: str,
        args: list,
        value: int = 0
    ) -> Dict:
        func = getattr(contract.functions, function)(*args)
        tx = func.build_transaction({
            "from": self.checksum_address,
            "value": value,
            "chainId": self.w3.eth.chain_id,
            "nonce": self.w3.eth.get_transaction_count(self.checksum_address),
            "gasPrice": self.w3.eth.gas_price,
        })
        
        return tx
    
    async def call_contract(
        self,
        contract: Any,
        function: str,
        args: list
    ) -> Any:
        func = getattr(contract.functions, function)(*args)
        return func.call({
            "from": self.checksum_address
        })
    
    def sign_message(self, message: str) -> str:
        return self.account.sign_message(message).signature.hex()
    
    @staticmethod
    def create_wallet() -> Dict:
        account = Account.create()
        return {
            "address": account.address,
            "private_key": account.key.hex()
        }
    
    @staticmethod
    def from_mnemonic(mnemonic: str, index: int = 0) -> "Wallet":
        account = Account.from_mnemonic(mnemonic, account_path=f"m/44'/60'/0'/0/{index}")
        return account
    
    def __repr__(self) -> str:
        return f"Wallet(address={self.address[:10]}...)"
