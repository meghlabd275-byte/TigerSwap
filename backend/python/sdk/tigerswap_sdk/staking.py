"""
Staking for TigerSwap SDK.

Reads and writes go to the TigerSwap staking contract
(contracts/TigerSwapStaking.sol). No values are invented locally.
"""

from typing import Any, Dict

from .contracts import ERC20_ABI, STAKING_ABI, get_contract_address


class Staking:
    """Staking interface for the TigerSwap staking contract."""

    def __init__(self, client):
        self.client = client

    def _staking(self, chain_id: int):
        address = get_contract_address(chain_id, "staking")
        return self.client.w3.eth.contract(
            address=self.client.w3.to_checksum_address(address), abi=STAKING_ABI
        )

    async def stake(self, amount: int, chain_id: int = 1) -> str:
        """Stake `amount` of the staking token (raw units)."""
        if self.client.wallet is None:
            raise ValueError("No wallet configured")

        contract = self._staking(chain_id)
        tx = contract.functions.stake(amount).build_transaction(
            {
                "from": self.client.wallet.address,
                "nonce": self.client.w3.eth.get_transaction_count(self.client.wallet.address),
                "chainId": chain_id,
            }
        )
        return await self.client.wallet.send_transaction(tx)

    async def withdraw(self, amount: int, chain_id: int = 1) -> str:
        """Withdraw `amount` of staked tokens (raw units)."""
        if self.client.wallet is None:
            raise ValueError("No wallet configured")

        contract = self._staking(chain_id)
        tx = contract.functions.withdraw(amount).build_transaction(
            {
                "from": self.client.wallet.address,
                "nonce": self.client.w3.eth.get_transaction_count(self.client.wallet.address),
                "chainId": chain_id,
            }
        )
        return await self.client.wallet.send_transaction(tx)

    async def claim_rewards(self, chain_id: int = 1) -> str:
        """Claim accrued staking rewards."""
        if self.client.wallet is None:
            raise ValueError("No wallet configured")

        contract = self._staking(chain_id)
        tx = contract.functions.claimRewards().build_transaction(
            {
                "from": self.client.wallet.address,
                "nonce": self.client.w3.eth.get_transaction_count(self.client.wallet.address),
                "chainId": chain_id,
            }
        )
        return await self.client.wallet.send_transaction(tx)

    async def get_staked_amount(self, address: str, chain_id: int = 1) -> int:
        """Staked balance of an address in raw units."""
        contract = self._staking(chain_id)
        return contract.functions.balanceOf(
            self.client.w3.to_checksum_address(address)
        ).call()

    async def get_pending_rewards(self, address: str, chain_id: int = 1) -> int:
        """Accrued but unclaimed rewards for an address in raw units."""
        contract = self._staking(chain_id)
        return contract.functions.earned(self.client.w3.to_checksum_address(address)).call()

    async def get_reward_rate(self, chain_id: int = 1) -> int:
        """Current reward rate in tokens per second."""
        return self._staking(chain_id).functions.rewardRate().call()

    async def get_position(self, address: str, chain_id: int = 1) -> Dict[str, Any]:
        """Combined staked balance and pending rewards for an address."""
        return {
            "address": address,
            "staked": await self.get_staked_amount(address, chain_id),
            "pending_rewards": await self.get_pending_rewards(address, chain_id),
        }

    async def approve(self, token_address: str, amount: int, chain_id: int = 1) -> str:
        """Approve the staking contract to pull the staking token."""
        if self.client.wallet is None:
            raise ValueError("No wallet configured")

        spender = get_contract_address(chain_id, "staking")
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
