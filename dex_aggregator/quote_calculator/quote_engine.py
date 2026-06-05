"""
TigerSwap DEX Aggregator - Quote Calculator
Real-time quote calculation with slippage, gas, and price impact
"""

import asyncio
import json
import time
from dataclasses import dataclass, field
from typing import List, Dict, Optional, Tuple
from enum import Enum
from web3 import Web3

@dataclass
class QuoteRequest:
    token_in: str
    token_out: str
    amount_in: int
    slippage_bps: int = 50  # Default 0.5%
    gas_price_gwei: Optional[int] = None
    deadline_seconds: int = 300  # 5 minutes
    max_hops: int = 3
    excluded_dexes: List[str] = field(default_factory=list)
    
@dataclass
class QuoteResult:
    success: bool
    provider: str  # Which DEX or "aggregator"
    method: str  # "swap", "split", "split_hop"
    
    # Amounts
    amount_in: int
    amount_out: int
    amount_out_min: int  # Slippage protected
    
    # Pricing
    exchange_rate: float
    price_impact_bps: int
    route: List[Dict]
    
    # Fees
    trading_fee_bps: int
    trading_fee_usd: float
    gas_estimate: int
    gas_fee_eth: float
    gas_fee_usd: float
    total_cost_usd: float
    
    # Metadata
    valid_until: int
    block_number: int
    
    error: Optional[str] = None

@dataclass  
class GasPrice:
    slow_gwei: int
    standard_gwei: int
    fast_gwei: int
    instant_gwei: int
    base_fee: int
    
@dataclass
class TokenInfo:
    address: str
    symbol: str
    name: str
    decimals: int
    price_usd: float
    chain_id: int

class QuoteCalculator:
    """
    Real-time quote calculator for DEX swaps.
    Calculates accurate quotes including:
    - Slippage protection
    - Gas estimation
    - Price impact
    - Fee calculation
    """
    
    # Contract ABIs for on-chain queries
    PAIR_ABI = [
        {
            "name": "getReserves",
            "outputs": [
                {"type": "uint112", "name": "_reserve0"},
                {"type": "uint112", "name": "_reserve1"},
                {"type": "uint32", "name": "_blockTimestampLast"}
            ],
            "stateMutability": "view",
            "type": "function"
        },
        {
            "name": "token0",
            "outputs": [{"type": "address", "name": ""}],
            "stateMutability": "view",
            "type": "function"
        },
        {
            "name": "token1", 
            "outputs": [{"type": "address", "name": ""}],
            "stateMutability": "view",
            "type": "function"
        }
    ]
    
    # ERC20 ABI for decimals
    ERC20_ABI = [
        {
            "name": "decimals",
            "outputs": [{"type": "uint8", "name": ""}],
            "stateMutability": "view",
            "type": "function"
        }
    ]
    
    def __init__(self, chain_id: int = 1, rpc_url: Optional[str] = None):
        self.chain_id = chain_id
        self.w3 = Web3(Web3.HTTPProvider(rpc_url or self._get_default_rpc(chain_id)))
        self._tokens: Dict[str, TokenInfo] = {}
        self._gas_price: Optional[GasPrice] = None
        self._last_gas_update = 0
        self._cache_ttl = 15  # seconds
        
    def _get_default_rpc(self, chain_id: int) -> str:
        """Get default RPC URL by chain"""
        rpcs = {
            1: "https://eth.llamarpc.com",
            56: "https://bsc-dataseed.binance.org",
            42161: "https://arb1.arbitrum.io/rpc",
            137: "https://polygon-rpc.com",
            10: "https://mainnet.optimism.io",
            43114: "https://api.avax.network/ext/bc/C/rpc",
        }
        return rpcs.get(chain_id, "https://eth.llamarpc.com")
    
    async def get_quote(self, request: QuoteRequest) -> QuoteResult:
        """
        Calculate quote for a swap request.
        This is the main entry point.
        """
        start_time = time.time()
        
        try:
            # Get current gas price
            gas_price = await self._get_gas_price()
            
            # Get token info
            token_in_info = await self._get_token_info(request.token_in)
            token_out_info = await self._get_token_info(request.token_out)
            
            if not token_in_info or not token_out_info:
                return QuoteResult(
                    success=False,
                    provider="",
                    method="",
                    amount_in=request.amount_in,
                    amount_out=0,
                    amount_out_min=0,
                    exchange_rate=0,
                    price_impact_bps=0,
                    route=[],
                    trading_fee_bps=0,
                    trading_fee_usd=0,
                    gas_estimate=0,
                    gas_fee_eth=0,
                    gas_fee_usd=0,
                    total_cost_usd=0,
                    valid_until=0,
                    block_number=0,
                    error="Token not supported"
                )
            
            # Calculate output amount using constant product formula
            amount_out, price_impact, route = await self._calculate_swap(
                request.token_in,
                request.token_out,
                request.amount_in,
                request.max_hops
            )
            
            # Apply slippage protection
            slippage_factor = (10000 - request.slippage_bps) / 10000
            amount_out_min = int(amount_out * slippage_factor)
            
            # Calculate fees
            input_usd = (request.amount_in / (10 ** token_in_info.decimals)) * token_in_info.price_usd
            trading_fee_usd = input_usd * 0.003  # 0.3% fee
            trading_fee_bps = 30
            
            # Gas calculation
            gas_price_gwei = request.gas_price_gwei or gas_price.standard_gwei
            gas_estimate = 150000  # Base gas for swap
            gas_fee_eth = (gas_estimate * gas_price_gwei) / 1e9
            eth_price_usd = token_in_info.price_usd if token_in_info.symbol == "ETH" else 2500
            gas_fee_usd = gas_fee_eth * eth_price_usd
            
            # Exchange rate
            exchange_rate = amount_out / request.amount_in
            if token_in_info.decimals != token_out_info.decimals:
                rate_adjustment = 10 ** (token_out_info.decimals - token_in_info.decimals)
                exchange_rate *= rate_adjustment
            
            # Total cost (input value + gas)
            total_cost_usd = input_usd + gas_fee_usd
            
            return QuoteResult(
                success=True,
                provider="TigerSwap",
                method="swap",
                amount_in=request.amount_in,
                amount_out=amount_out,
                amount_out_min=amount_out_min,
                exchange_rate=exchange_rate,
                price_impact_bps=int(price_impact * 100),  # Convert to bps
                route=route,
                trading_fee_bps=trading_fee_bps,
                trading_fee_usd=trading_fee_usd,
                gas_estimate=gas_estimate,
                gas_fee_eth=gas_fee_eth,
                gas_fee_usd=gas_fee_usd,
                total_cost_usd=total_cost_usd,
                valid_until=int(time.time()) + 30,
                block_number=self.w3.eth.block_number
            )
            
        except Exception as e:
            return QuoteResult(
                success=False,
                provider="",
                method="",
                amount_in=request.amount_in,
                amount_out=0,
                amount_out_min=0,
                exchange_rate=0,
                price_impact_bps=0,
                route=[],
                trading_fee_bps=0,
                trading_fee_usd=0,
                gas_estimate=0,
                gas_fee_eth=0,
                gas_fee_usd=0,
                total_cost_usd=0,
                valid_until=0,
                block_number=0,
                error=str(e)
            )
    
    async def _calculate_swap(
        self,
        token_in: str,
        token_out: str,
        amount_in: int,
        max_hops: int
    ) -> Tuple[int, float, List[Dict]]:
        """
        Calculate optimal swap across DEXs.
        Returns (amount_out, price_impact_bps, route)
        """
        # Direct pools (would query from subgraph/contract in production)
        pools = await self._get_viable_pools(token_in, token_out)
        
        if not pools:
            # Try multi-hop
            return await self._calculate_multi_hop(token_in, token_out, amount_in, max_hops)
        
        best_output = 0
        best_impact = 0
        best_route = []
        
        for pool in pools[:10]:  # Check top 10 pools
            reserve_in, reserve_out, fee = pool['reserve_in'], pool['reserve_out'], pool['fee']
            
            # Calculate output
            amount_out = self._constant_product_swap(amount_in, reserve_in, reserve_out, fee)
            
            # Calculate price impact
            spot_price = reserve_out / reserve_in
            exec_price = amount_out / amount_in if amount_in > 0 else 0
            impact = ((spot_price - exec_price) / spot_price * 100) if spot_price > 0 else 0
            
            if amount_out > best_output:
                best_output = amount_out
                best_impact = impact
                best_route = [{
                    'dex': pool['dex'],
                    'pool': pool['address'],
                    'fee': fee,
                    'path': [token_in, token_out]
                }]
        
        return best_output, best_impact, best_route
    
    async def _calculate_multi_hop(
        self,
        token_in: str,
        token_out: str,
        amount_in: int,
        max_hops: int
    ) -> Tuple[int, float, List[Dict]]:
        """Calculate multi-hop path through intermediate tokens"""
        # Common intermediate tokens
        intermediates = [
            "0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2",  # WETH
            "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48",  # USDC
            "0xdAC17F958D2ee523a2206206994597C13D831ec7",  # USDT
        ]
        
        best_output = 0
        best_route = []
        best_impact = 0
        
        for intermediate in intermediates:
            if intermediate in [token_in, token_out]:
                continue
            
            # Get pools for both legs
            pools1 = await self._get_viable_pools(token_in, intermediate)
            pools2 = await self._get_viable_pools(intermediate, token_out)
            
            if not pools1 or not pools2:
                continue
            
            # Best pool for each leg
            pool1 = pools1[0]
            pool2 = pools2[0]
            
            # Calculate through both legs
            amount_mid = self._constant_product_swap(
                amount_in,
                pool1['reserve_in'],
                pool1['reserve_out'],
                pool1['fee']
            )
            
            if amount_mid > 0:
                amount_out = self._constant_product_swap(
                    amount_mid,
                    pool2['reserve_in'],
                    pool2['reserve_out'],
                    pool2['fee']
                )
                
                if amount_out > best_output:
                    best_output = amount_out
                    best_route = [
                        {
                            'dex': pool1['dex'],
                            'pool': pool1['address'],
                            'fee': pool1['fee'],
                            'path': [token_in, intermediate]
                        },
                        {
                            'dex': pool2['dex'],
                            'pool': pool2['address'],
                            'fee': pool2['fee'],
                            'path': [intermediate, token_out]
                        }
                    ]
                    best_impact = 0.1  # Small impact for hop
        
        return best_output, best_impact, best_route
    
    def _constant_product_swap(
        self,
        amount_in: int,
        reserve_in: int,
        reserve_out: int,
        fee_bps: int
    ) -> int:
        """
        Real constant product AMM formula: x*y=k
        With fee: amountOut = (amountIn * reserveOut * (10000 - fee)) / (reserveIn * 10000 + amountIn * (10000 - fee))
        """
        if reserve_in == 0 or reserve_out == 0:
            return 0
        
        fee_multiplier = 10000 - fee_bps
        numerator = amount_in * reserve_out * fee_multiplier
        denominator = reserve_in * 10000 + amount_in * fee_multiplier
        
        if denominator == 0:
            return 0
        
        return numerator // denominator
    
    async def _get_viable_pools(
        self,
        token_a: str,
        token_b: str
    ) -> List[Dict]:
        """
        Get viable pools for token pair.
        In production, this queries The Graph or on-chain.
        """
        # Simulated pool data - in production would be real data
        pools = []
        
        # Normalize token order
        token_low = token_a.lower()
        token_high = token_b.lower()
        
        # Common WETH pairs
        weth = "0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2"
        usdc = "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48"
        usdt = "0xdac17f958d2ee523a2206206994597c13d831ec7"
        
        # Simulate pools based on token pair
        if (token_low == weth and token_high == usdc) or \
           (token_low == usdc and token_high == weth):
            pools = [
                {
                    'dex': 'uniswap_v2',
                    'address': '0xB4e16d0168e52d35CaCD2c6185b44281Ec28C9Dc',
                    'reserve_in': 50000 * 10**18,
                    'reserve_out': 125000000 * 10**6,
                    'fee': 30,
                    'liquidity': 125_000_000
                },
                {
                    'dex': 'uniswap_v3',
                    'address': '0x88e6A0c2dDD26FEEb64F039a2c41296FcB3f5640',
                    'reserve_in': 35000 * 10**18,
                    'reserve_out': 87500000 * 10**6,
                    'fee': 500,
                    'liquidity': 87_500_000
                }
            ]
        elif (token_low == weth and token_high == usdt) or \
             (token_low == usdt and token_high == weth):
            pools = [
                {
                    'dex': 'uniswap_v2',
                    'address': '0x0d4a11d5EEaaC28EC3F61d100daF4d40471f1852',
                    'reserve_in': 40000 * 10**18,
                    'reserve_out': 100000000 * 10**6,
                    'fee': 30,
                    'liquidity': 100_000_000
                }
            ]
        
        return pools
    
    async def _get_gas_price(self) -> GasPrice:
        """Get current gas prices"""
        now = time.time()
        
        # Return cached if fresh
        if self._gas_price and (now - self._last_gas_update) < self._cache_ttl:
            return self._gas_price
        
        try:
            # Get from network
            if self.w3.is_connected():
                base_fee = self.w3.eth.get_block('latest').base_fee_per_gas
                gas_price = self.w3.eth.gas_price
                
                self._gas_price = GasPrice(
                    slow_gwei=int(gas_price * 0.8 / 1e9),
                    standard_gwei=int(gas_price / 1e9),
                    fast_gwei=int(gas_price * 1.2 / 1e9),
                    instant_gwei=int(gas_price * 1.5 / 1e9),
                    base_fee=base_fee or 0
                )
            else:
                # Fallback defaults
                self._gas_price = GasPrice(20, 35, 50, 75, 15)
        except:
            self._gas_price = GasPrice(20, 35, 50, 75, 15)
            
        self._last_gas_update = now
        return self._gas_price
    
    async def _get_token_info(self, address: str) -> Optional[TokenInfo]:
        """Get token information"""
        address = address.lower()
        
        if address in self._tokens:
            return self._tokens[address]
        
        # Known tokens
        token_data = {
            "0xc02aaa39b223fe8d0a0e5c4f27ead9083c756cc2": TokenInfo(
                address="0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2",
                symbol="WETH",
                name="Wrapped Ether",
                decimals=18,
                price_usd=2450.0,
                chain_id=1
            ),
            "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48": TokenInfo(
                address="0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48",
                symbol="USDC",
                name="USD Coin",
                decimals=6,
                price_usd=1.0,
                chain_id=1
            ),
            "0xdac17f958d2ee523a2206206994597c13d831ec7": TokenInfo(
                address="0xdAC17F958D2ee523a2206206994597C13D831ec7",
                symbol="USDT",
                name="Tether USD",
                decimals=6,
                price_usd=1.0,
                chain_id=1
            ),
            "0x2260fac5e5542a773aa44fcf0f1e3f9dcf128b5ce": TokenInfo(
                address="0x2260FAC5E5542a773Aa44fCF0F1E3F9Dcf128B5CE",
                symbol="WBTC",
                name="Wrapped Bitcoin",
                decimals=8,
                price_usd=62500.0,
                chain_id=1
            ),
        }
        
        info = token_data.get(address)
        if info:
            self._tokens[address] = info
            
        return info
    
    async def compare_dex_prices(
        self,
        token_in: str,
        token_out: str,
        amount_in: int
    ) -> List[Dict]:
        """
        Compare prices across all DEXs for a given swap.
        Returns sorted list of quotes from best to worst.
        """
        pools = await self._get_viable_pools(token_in, token_out)
        
        quotes = []
        for pool in pools:
            amount_out = self._constant_product_swap(
                amount_in,
                pool['reserve_in'],
                pool['reserve_out'],
                pool['fee']
            )
            
            quotes.append({
                'dex': pool['dex'],
                'pool': pool['address'],
                'amount_out': amount_out,
                'fee': pool['fee'],
                'liquidity': pool['liquidity'],
                'price_per_unit': amount_out / amount_in if amount_in > 0 else 0
            })
        
        # Sort by best price
        quotes.sort(key=lambda x: x['amount_out'], reverse=True)
        
        return quotes
    
    def calculate_price_impact(
        self,
        amount_in: int,
        reserve_in: int,
        reserve_out: int,
        fee_bps: int
    ) -> float:
        """Calculate price impact in percentage"""
        if reserve_in == 0:
            return 100.0
            
        spot_price = reserve_out / reserve_in
        amount_out = self._constant_product_swap(amount_in, reserve_in, reserve_out, fee_bps)
        
        if amount_in == 0 or amount_out == 0:
            return 0.0
            
        exec_price = amount_out / amount_in
        
        return max(0.0, ((spot_price - exec_price) / spot_price) * 100)


async def main():
    """Test the quote calculator"""
    calc = QuoteCalculator(chain_id=1)
    
    # WETH -> USDC
    request = QuoteRequest(
        token_in="0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2",
        token_out="0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48",
        amount_in=1 * 10**18,  # 1 ETH
        slippage_bps=50  # 0.5%
    )
    
    quote = await calc.get_quote(request)
    
    print(f"\n=== Quote Result ===")
    print(f"Success: {quote.success}")
    print(f"Provider: {quote.provider}")
    print(f"Input: {request.amount_in / 10**18} ETH")
    print(f"Output: {quote.amount_out / 10**6} USDC")
    print(f"Output Min: {quote.amount_out_min / 10**6} USDC")
    print(f"Price Impact: {quote.price_impact_bps / 100:.2f}%")
    print(f"Gas Estimate: {quote.gas_estimate}")
    print(f"Gas Fee: {quote.gas_fee_eth:.6f} ETH (${quote.gas_fee_usd:.2f})")
    print(f"Trading Fee: ${quote.trading_fee_usd:.2f}")
    print(f"Route: {[r['dex'] for r in quote.route]}")
    

if __name__ == "__main__":
    asyncio.run(main())
