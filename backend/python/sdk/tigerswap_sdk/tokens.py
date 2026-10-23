"""
Token management for TigerSwap SDK.

The static token list seeds metadata; live prices are read from the on-chain
TigerSwap router rather than a hardcoded table.
"""

from dataclasses import dataclass
from typing import Any, Dict, List, Optional

from .contracts import ROUTER_ABI, DeploymentError, get_contract_address, is_deployed


@dataclass
class Token:
    """Token information"""
    address: str
    symbol: str
    name: str
    decimals: int
    chain_id: int
    logo_uri: Optional[str] = None
    price: Optional[float] = None

    @property
    def formatted_address(self) -> str:
        return f"{self.address[:6]}...{self.address[-4:]}"


class TokenList:
    """Token list management"""

    # Seed list for the default chains. Live prices are resolved on-chain.
    DEFAULT_TOKENS = [
        Token("0x0000000000000000000000000000000000000000", "ETH", "Ethereum", 18, 1),
        Token("0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48", "USDC", "USD Coin", 6, 1),
        Token("0xdAC17F958D2ee523a2206206994597C13D831ec7", "USDT", "Tether USD", 6, 1),
        Token("0x2260FAC5E5542a773Aa44fBCfeDf7C193bc2C599", "WBTC", "Wrapped Bitcoin", 8, 1),
        Token("0x514910771AF9Ca656af840dff83E8264EcF986CA", "LINK", "Chainlink", 18, 1),
        Token("0x1f9840a85d5aF5bf1D1762F925BDADdC4201F984", "UNI", "Uniswap", 18, 1),
        Token("0x7Fc66500c84A76Ad7e9c93437bFc5Ac33E2DDaE9", "AAVE", "Aave", 18, 1),
        Token("0x6B175474E89094C44Da98b954EedeAC495271d0F", "DAI", "Dai Stablecoin", 18, 1),
        Token("0x7D1AfA7B718fb893dB30A3aBc0Cfc608AaCfeBB0", "MATIC", "Polygon", 18, 137),
        Token("0xbb4CdB9CBd36B01bD1cBaEBF2De08d9173bc095c", "BNB", "BNB", 18, 56),
    ]

    def __init__(self, client):
        self.client = client
        self._tokens: Dict[int, List[Token]] = {1: self.DEFAULT_TOKENS.copy()}

    def get_tokens(self, chain_id: int = 1) -> List[Token]:
        return self._tokens.get(chain_id, [])

    def get_token(self, address: str, chain_id: int = 1) -> Optional[Token]:
        for token in self._tokens.get(chain_id, []):
            if token.address.lower() == address.lower():
                return token
        return None

    def get_token_by_symbol(self, symbol: str, chain_id: int = 1) -> Optional[Token]:
        for token in self._tokens.get(chain_id, []):
            if token.symbol.upper() == symbol.upper():
                return token
        return None

    def add_token(self, token: Token) -> None:
        self._tokens.setdefault(token.chain_id, []).append(token)

    def search_tokens(self, query: str, chain_id: int = 1) -> List[Token]:
        query = query.lower()
        return [
            t
            for t in self._tokens.get(chain_id, [])
            if query in t.symbol.lower() or query in t.name.lower()
        ]

    def get_popular_tokens(self, chain_id: int = 1) -> List[Token]:
        return self._tokens.get(chain_id, [])[:10]

    async def fetch_token_prices(
        self, chain_id: int = 1, quote_symbol: str = "USDC"
    ) -> Dict[str, float]:
        """Return token prices denominated in `quote_symbol`.

        Prices are derived by quoting 1 whole unit of each token through the
        TigerSwap router against the quote token. Tokens without a TigerSwap
        pool are omitted rather than assigned a fabricated price.
        """
        if not is_deployed(chain_id):
            raise DeploymentError(
                f"TigerSwap is not deployed on chain {chain_id}; cannot read prices"
            )

        quote = self.get_token_by_symbol(quote_symbol, chain_id)
        if quote is None:
            raise ValueError(f"quote token {quote_symbol} not found on chain {chain_id}")

        router = self.client.w3.eth.contract(
            address=self.client.w3.to_checksum_address(get_contract_address(chain_id, "router")),
            abi=ROUTER_ABI,
        )
        quote_unit = 10**quote.decimals

        prices: Dict[str, float] = {}
        for token in self._tokens.get(chain_id, []):
            if token.symbol == quote_symbol:
                prices[token.symbol] = 1.0
                continue
            if token.address.lower() == quote.address.lower():
                continue
            try:
                path = [
                    self.client.w3.to_checksum_address(token.address),
                    self.client.w3.to_checksum_address(quote.address),
                ]
                out = int(router.functions.getAmountsOut(10**token.decimals, path).call()[-1])
            except Exception:
                continue
            if out > 0:
                prices[token.symbol] = out / quote_unit
        return prices
