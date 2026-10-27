"""
Perpetuals trading for TigerSwap SDK.

Every call targets the TigerSwap perpetual engine deployed by this repository
(contracts/TigerSwapPerpetuals.sol). Market configuration, mark price, funding
and position state are read on-chain; nothing is estimated locally.
"""

from dataclasses import dataclass
from typing import Any, Dict, Optional

from .contracts import ERC20_ABI, PERPETUALS_ABI, get_contract_address

# TigerSwap scales prices by 1e18 and leverage by 1e4 in the perp engine.
PRICE_PRECISION = 10**18
LEVERAGE_PRECISION = 10**4


@dataclass
class Position:
    """An open perpetual position as returned by the engine."""

    position_id: str
    market_id: str
    trader: str
    size: int
    collateral: int
    entry_price: int
    last_updated: int
    is_long: bool
    mark_price: int = 0

    @property
    def side(self) -> str:
        return "long" if self.is_long else "short"

    @property
    def notional(self) -> int:
        """Position notional at the entry price, in whole quote units.

        Size and price are both 1e18-scaled, so the product must be divided by
        the precision twice to return a human-scale number.
        """
        return self.size * self.entry_price // (PRICE_PRECISION * PRICE_PRECISION)

    @property
    def collateral_whole(self) -> int:
        """Collateral in whole units under the engine's 1e18 convention."""
        return self.collateral // PRICE_PRECISION

    @property
    def leverage(self) -> float:
        """Leverage implied by notional over collateral."""
        if self.collateral_whole == 0:
            return 0.0
        return round(self.notional / self.collateral_whole, 4)

    @property
    def unrealized_pnl(self) -> int:
        """Unrealised profit or loss at the current mark price, in whole units."""
        if self.mark_price == 0:
            return 0
        delta = self.mark_price - self.entry_price
        if not self.is_long:
            delta = -delta
        return self.size * delta // (PRICE_PRECISION * PRICE_PRECISION)


class Perpetuals:
    """Perpetuals interface for the TigerSwap perp engine."""

    def __init__(self, client):
        self.client = client

    def _engine(self, chain_id: int):
        address = get_contract_address(chain_id, "perpetuals")
        return self.client.w3.eth.contract(
            address=self.client.w3.to_checksum_address(address), abi=PERPETUALS_ABI
        )

    @staticmethod
    def market_id(symbol: str) -> bytes:
        """Derives a market id the same way the contract does: keccak of the symbol."""
        from eth_utils import keccak

        return keccak(text=symbol)

    async def get_market(self, symbol: str, chain_id: int = 1) -> Dict[str, Any]:
        """Return a market's configuration plus live funding and mark price."""
        engine = self._engine(chain_id)
        market_id = self.market_id(symbol)

        # getMarket returns a MarketView struct as a tuple: token, maxLeverage,
        # liquidationFee, maintenanceMargin, initialMargin, isActive, openFee,
        # maxOpenInterest, openInterest.
        market = engine.functions.getMarket(market_id).call()
        (
            token,
            max_leverage,
            liquidation_fee,
            maintenance_margin,
            initial_margin,
            active,
            open_fee,
            max_open_interest,
            open_interest,
        ) = market
        funding_rate, last_update, mark_price, index_price = engine.functions.getFundingInfo(
            market_id
        ).call()

        return {
            "market_id": market_id.hex(),
            "symbol": symbol,
            "collateral_token": token,
            "max_leverage": max_leverage,
            "liquidation_fee": liquidation_fee,
            "maintenance_margin": maintenance_margin,
            "initial_margin": initial_margin,
            "is_active": active,
            "open_fee": open_fee,
            "max_open_interest": max_open_interest,
            "open_interest": open_interest,
            "mark_price": mark_price,
            "index_price": index_price,
            "funding_rate": funding_rate,
            "funding_updated_at": last_update,
        }

    async def get_mark_price(self, symbol: str, chain_id: int = 1) -> int:
        """Mark price for a market, scaled by 1e18."""
        return self._engine(chain_id).functions.getMarkPrice(self.market_id(symbol)).call()

    async def get_funding_rate(self, symbol: str, chain_id: int = 1) -> int:
        """Last funding rate for a market, scaled by 1e18."""
        _, _, _, funding_rate = (
            self._engine(chain_id).functions.getFundingInfo(self.market_id(symbol)).call()
        )
        return funding_rate

    async def get_position(self, position_id: str, chain_id: int = 1) -> Optional[Position]:
        """Return a position by id, or None when it does not exist."""
        engine = self._engine(chain_id)
        identifier = self._to_bytes32(position_id)
        market_id, trader, size, collateral, entry_price, last_updated, is_long = (
            engine.functions.getPosition(identifier).call()
        )

        if size == 0:
            return None

        position = Position(
            position_id=identifier.hex(),
            market_id=market_id.hex(),
            trader=trader,
            size=size,
            collateral=collateral,
            entry_price=entry_price,
            last_updated=last_updated,
            is_long=is_long,
        )

        # Attach the mark price so PnL can be computed without another call.
        try:
            position.mark_price = engine.functions.getMarkPrice(market_id).call()
        except Exception:
            position.mark_price = 0

        return position

    async def open_position(
        self,
        symbol: str,
        size: int,
        collateral: int,
        is_long: bool,
        chain_id: int = 1,
    ) -> str:
        """Open a position. Collateral is pulled from the configured wallet."""
        if self.client.wallet is None:
            raise ValueError("No wallet configured")

        engine = self._engine(chain_id)
        tx = engine.functions.openPosition(
            self.market_id(symbol), size, collateral, is_long
        ).build_transaction(
            {
                "from": self.client.wallet.address,
                "nonce": self.client.w3.eth.get_transaction_count(self.client.wallet.address),
                "chainId": chain_id,
            }
        )
        return await self.client.wallet.send_transaction(tx)

    async def close_position(self, position_id: str, chain_id: int = 1) -> str:
        """Close a position owned by the configured wallet."""
        if self.client.wallet is None:
            raise ValueError("No wallet configured")

        engine = self._engine(chain_id)
        tx = engine.functions.closePosition(self._to_bytes32(position_id)).build_transaction(
            {
                "from": self.client.wallet.address,
                "nonce": self.client.w3.eth.get_transaction_count(self.client.wallet.address),
                "chainId": chain_id,
            }
        )
        return await self.client.wallet.send_transaction(tx)

    async def add_margin(self, position_id: str, amount: int, chain_id: int = 1) -> str:
        """Add collateral to an existing position."""
        if self.client.wallet is None:
            raise ValueError("No wallet configured")

        engine = self._engine(chain_id)
        tx = engine.functions.addMargin(
            self._to_bytes32(position_id), amount
        ).build_transaction(
            {
                "from": self.client.wallet.address,
                "nonce": self.client.w3.eth.get_transaction_count(self.client.wallet.address),
                "chainId": chain_id,
            }
        )
        return await self.client.wallet.send_transaction(tx)

    async def remove_margin(self, position_id: str, amount: int, chain_id: int = 1) -> str:
        """Remove collateral from an existing position."""
        if self.client.wallet is None:
            raise ValueError("No wallet configured")

        engine = self._engine(chain_id)
        tx = engine.functions.removeMargin(
            self._to_bytes32(position_id), amount
        ).build_transaction(
            {
                "from": self.client.wallet.address,
                "nonce": self.client.w3.eth.get_transaction_count(self.client.wallet.address),
                "chainId": chain_id,
            }
        )
        return await self.client.wallet.send_transaction(tx)

    async def liquidate_position(self, position_id: str, chain_id: int = 1) -> str:
        """Liquidate an under-collateralised position."""
        if self.client.wallet is None:
            raise ValueError("No wallet configured")

        engine = self._engine(chain_id)
        tx = engine.functions.liquidatePosition(
            self._to_bytes32(position_id)
        ).build_transaction(
            {
                "from": self.client.wallet.address,
                "nonce": self.client.w3.eth.get_transaction_count(self.client.wallet.address),
                "chainId": chain_id,
            }
        )
        return await self.client.wallet.send_transaction(tx)

    async def approve_collateral(
        self, token_address: str, amount: Optional[int] = None, chain_id: int = 1
    ) -> str:
        """Approve the perp engine to pull collateral."""
        if self.client.wallet is None:
            raise ValueError("No wallet configured")

        if amount is None:
            amount = 2**256 - 1

        spender = get_contract_address(chain_id, "perpetuals")
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

    def _to_bytes32(self, value: str) -> bytes:
        """Normalises a hex string or raw string into a 32-byte value."""
        if isinstance(value, bytes):
            raw = value
        elif value.startswith("0x") or value.startswith("0X"):
            # Anything hex-prefixed must be a complete 32-byte value. Falling
            # through to the symbol hash here would silently reinterpret a
            # truncated id as a different market.
            if len(value) != 66:
                raise ValueError(
                    f"hex identifier must be 32 bytes (66 characters), received {len(value)}"
                )
            try:
                raw = bytes.fromhex(value[2:])
            except ValueError as error:
                raise ValueError(f"identifier is not valid hex: {error}") from error
        else:
            from eth_utils import keccak

            raw = keccak(text=value)

        if len(raw) != 32:
            raise ValueError(f"expected 32 bytes, received {len(raw)}")
        return raw
