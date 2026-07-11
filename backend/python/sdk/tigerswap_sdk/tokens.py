"""
Token management for TigerSwap SDK
"""

from typing import List, Dict, Any, Optional
from dataclasses import dataclass


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
    
    # Default token list - top 50 tokens
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
        if token.chain_id not in self._tokens:
            self._tokens[token.chain_id] = []
        self._tokens[token.chain_id].append(token)
    
    def search_tokens(self, query: str, chain_id: int = 1) -> List[Token]:
        query = query.lower()
        results = []
        for token in self._tokens.get(chain_id, []):
            if query in token.symbol.lower() or query in token.name.lower():
                results.append(token)
        return results
    
    async def fetch_token_prices(self, chain_id: int = 1) -> Dict[str, float]:
        prices = {}
        for token in self._tokens.get(chain_id, []):
            if token.symbol in ["ETH", "BTC", "USDC", "USDT", "DAI", "LINK", "UNI", "AAVE"]:
                prices[token.symbol] = self._get_mock_price(token.symbol)
        return prices
    
    def _get_mock_price(self, symbol: str) -> float:
        prices = {
            "ETH": 3450.0,
            "BTC": 68500.0,
            "USDC": 1.0,
            "USDT": 1.0,
            "DAI": 1.0,
            "LINK": 18.50,
            "UNI": 12.80,
            "AAVE": 285.0,
        }
        return prices.get(symbol, 0.0)
    
    def get_popular_tokens(self, chain_id: int = 1) -> List[Token]:
        return self._tokens.get(chain_id, [])[:10]
