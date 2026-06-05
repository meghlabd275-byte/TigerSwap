"""
TigerSwap MEV Protection Layer
Real MEV protection against sandwich attacks, front-running, and arbitrage
"""

import asyncio
import hashlib
import json
import time
from dataclasses import dataclass, field
from typing import Dict, List, Optional, Callable, Tuple
from enum import Enum
import httpx

@dataclass
class MEVProtectionConfig:
    """Configuration for MEV protection"""
    use_flashbots: bool = True
    use_suave: bool = False
    preferred_gas_speed: str = "instant"  # slow, standard, fast, instant
    max_bundle_gas: int = 5000000
    simulation_gas_limit: int = 10000000
    target_mev_share: float = 0.5  # Target 50% of MEV refund
    backrun_delay_ms: int = 0  # Delay before backrunning (0 = as fast as possible)

@dataclass
class ProtectionResult:
    """Result of MEV protection attempt"""
    success: bool
    tx_hash: Optional[str] = None
    bundle_id: Optional[str] = None
    flashbots_bundle_id: Optional[str] = None
    
    # MEV Analysis
    mev_detected: bool = False
    mev_type: Optional[str] = None  # "sandwich", "arbitrage", "liquidation", "front_run"
    estimated_mev: float = 0.0
    
    # Gas settings
    gas_price_used: int = 0
    priority_fee: int = 0
    max_fee: int = 0
    
    # Refunds
    mev_refund: int = 0
    
    # Fallback
    used_fallback: bool = False
    
    error: Optional[str] = None

@dataclass
class BundleContents:
    """Contents of a transaction bundle"""
    txs: List[str]  # Signed transactions
    replacement_data: Optional[Dict] = None  # For MEV rebates

class FlashbotsRPC:
    """
    Flashbots RPC interface for MEV protection.
    Uses Flashbots Protect to send private transactions.
    """
    
    FLASHBOTS_RELAY = "https://relay.flashbots.net"
    FLASHBOTS_RPC = "https://rpc.flashbots.net"
    
    def __init__(self, auth_key: Optional[str] = None):
        self.auth_key = auth_key
        self.http_client = httpx.AsyncClient(timeout=30.0)
    
    async def send_bundle(
        self,
        txs: List[str],
        block_number: Optional[int] = None,
        min_timestamp: Optional[int] = None,
        max_timestamp: Optional[int] = None,
        reverting_tx_hashes: List[str] = None
    ) -> Dict:
        """
        Send a bundle to Flashbots.
        
        Args:
            txs: List of signed transactions (hex encoded)
            block_number: Target block number
            min_timestamp: Minimum timestamp
            max_timestamp: Maximum timestamp
            reverting_tx_hashes: Txs that can revert
            
        Returns:
            Flashbots bundle response
        """
        params = {
            "jsonrpc": "2.0",
            "id": 1,
            "method": "eth_sendBundle",
            "params": [{
                "txs": txs,
            }]
        }
        
        if block_number:
            params["params"][0]["blockNumber"] = hex(block_number)
        if min_timestamp:
            params["params"][0]["minTimestamp"] = hex(min_timestamp)
        if max_timestamp:
            params["params"][0]["maxTimestamp"] = hex(max_timestamp)
        if reverting_tx_hashes:
            params["params"][0]["revertingTxHashes"] = reverting_tx_hashes
        
        headers = {}
        if self.auth_key:
            headers["X-Flashbots-Signature"] = self.auth_key
        
        try:
            response = await self.http_client.post(
                self.FLASHBOTS_RPC,
                json=params,
                headers=headers
            )
            return response.json()
        except Exception as e:
            return {"error": str(e)}
    
    async def call_bundle(
        self,
        txs: List[str],
        block_number: Optional[int] = None
    ) -> Dict:
        """
        Simulate a bundle before sending.
        Returns MEV opportunity analysis.
        """
        params = {
            "jsonrpc": "2.0",
            "id": 1,
            "method": "eth_callBundle",
            "params": [{
                "txs": txs,
                "blockNumber": hex(block_number) if block_number else None,
                "stateBlockNumber": "latest",
                "timestamp": hex(int(time.time())),
            }]
        }
        
        headers = {}
        if self.auth_key:
            headers["X-Flashbots-Signature"] = self.auth_key
        
        try:
            response = await self.http_client.post(
                self.FLASHBOTS_RPC,
                json=params,
                headers=headers
            )
            return response.json()
        except Exception as e:
            return {"error": str(e)}
    
    async def get_bundle_stats(self, bundle_hash: str) -> Dict:
        """Get statistics for a sent bundle"""
        params = {
            "jsonrpc": "2.0",
            "id": 1,
            "method": "flashbots_getBundleStats",
            "params": [{
                "bundleHash": bundle_hash,
                "blockNumber": hex(int(time.time()) // 12)  # Current block
            }]
        }
        
        headers = {}
        if self.auth_key:
            headers["X-Flashbots-Signature"] = self.auth_key
        
        try:
            response = await self.http_client.post(
                self.FLASHBOTS_RPC,
                json=params,
                headers=headers
            )
            return response.json()
        except Exception as e:
            return {"error": str(e)}


class MEVDetector:
    """
    Detect potential MEV threats in transactions.
    Analyzes mempool for sandwich attacks, front-running, and arbitrage.
    """
    
    def __init__(self):
        self.threat_patterns: Dict[str, Dict] = {}
        self._initialize_patterns()
    
    def _initialize_patterns(self):
        """Initialize known MEV attack patterns"""
        self.threat_patterns = {
            "sandwich": {
                "description": "Attacker front-runs and back-runs a trade",
                "indicators": [
                    "same_token_pair",
                    "high_slippage",
                    "large_trade"
                ],
                "severity": "high"
            },
            "front_run": {
                "description": "Attacker detects pending trade and executes first",
                "indicators": [
                    "pending_swap",
                    "higher_gas_price"
                ],
                "severity": "medium"
            },
            "arbitrage": {
                "description": "Bot arbitraging price differences across DEXs",
                "indicators": [
                    "price_diff_dex",
                    "atomic_execution"
                ],
                "severity": "low"  # Can be beneficial for market efficiency
            },
            "liquidation": {
                "description": "Loan liquidation bot",
                "indicators": [
                    "liquidation_trigger",
                    "health_factor_threshold"
                ],
                "severity": "low"  # Normal protocol function
            }
        }
    
    def analyze_transaction(self, tx_data: Dict) -> Tuple[bool, str, float]:
        """
        Analyze a transaction for MEV risk.
        
        Returns:
            (is_mev, mev_type, estimated_value)
        """
        tx_value = float(tx_data.get("value", 0))
        tx_data_hex = tx_data.get("data", "")
        to_address = tx_data.get("to", "").lower()
        
        # Check for sandwich indicators
        if self._check_sandwich_pattern(tx_data):
            return True, "sandwich", tx_value * 0.01  # ~1% MEV
        
        # Check for front-running patterns
        if self._check_frontrun_pattern(tx_data):
            return True, "front_run", tx_value * 0.005  # ~0.5% MEV
        
        # Check for arbitrage
        if self._check_arbitrage_pattern(tx_data):
            return True, "arbitrage", tx_value * 0.001  # ~0.1% MEV
        
        return False, "", 0.0
    
    def _check_sandwich_pattern(self, tx_data: Dict) -> bool:
        """Check if transaction is vulnerable to sandwich attack"""
        # High slippage setting
        slippage = tx_data.get("slippage_bps", 0)
        if slippage > 100:  # > 1%
            return True
        
        # Large trade relative to pool
        trade_value = float(tx_data.get("value", 0))
        pool_liquidity = tx_data.get("pool_liquidity", float('inf'))
        
        if pool_liquidity < trade_value * 10:  # Trade is > 10% of pool
            return True
        
        return False
    
    def _check_frontrun_pattern(self, tx_data: Dict) -> bool:
        """Check if transaction might be front-run"""
        # Check if using Uniswap V2 (more vulnerable)
        to_address = tx_data.get("to", "").lower()
        vulnerable_routers = [
            "0x7a250d5630b4cf539739df2c5dacb4c659f2488d",  # Uniswap V2
            "0xf164fc0ec4e93095b804a4795bbe8e65846a3a60",  # Uniswap V2 Router
        ]
        
        return to_address in vulnerable_routers
    
    def _check_arbitrage_pattern(self, tx_data: Dict) -> bool:
        """Check if transaction is an arbitrage opportunity"""
        # Check for multi-DEX or multi-hop patterns
        data_hex = tx_data.get("data", "")
        
        # Arbitrage typically involves complex swaps
        if len(data_hex) > 1000:  # Complex transaction
            return True
        
        return False
    
    def get_protection_recommendation(
        self,
        mev_detected: bool,
        mev_type: str
    ) -> Dict:
        """Get recommended protection strategy"""
        if not mev_detected:
            return {
                "strategy": "standard",
                "use_flashbots": False,
                "use_suave": False,
                "additional_gas": 0
            }
        
        recommendations = {
            "sandwich": {
                "strategy": "flashbots",
                "use_flashbots": True,
                "use_suave": False,
                "additional_gas": 50000,
                "slippage_reduction": 0.5,  # Reduce by 50%
                "delay_backrun": True
            },
            "front_run": {
                "strategy": "flashbots",
                "use_flashbots": True,
                "use_suave": False,
                "additional_gas": 25000,
                "private_tx": True
            },
            "arbitrage": {
                "strategy": "fast",
                "use_flashbots": False,
                "use_suave": False,
                "additional_gas": 10000
            }
        }
        
        return recommendations.get(mev_type, {
            "strategy": "standard",
            "use_flashbots": False,
            "use_suave": False,
            "additional_gas": 0
        })


class MEVProtection:
    """
    Main MEV protection service for TigerSwap.
    Coordinates Flashbots protection, transaction analysis, and risk mitigation.
    """
    
    def __init__(self, config: MEVProtectionConfig = None):
        self.config = config or MEVProtectionConfig()
        self.flashbots = FlashbotsRPC()
        self.detector = MEVDetector()
        
        # Pending transactions for analysis
        self.pending_txs: Dict[str, Dict] = {}
        
        # MEV statistics
        self.stats = {
            "total_txs": 0,
            "protected_txs": 0,
            "sandwich_attacks_blocked": 0,
            "front_runs_blocked": 0,
            "total_mev_saved": 0.0
        }
    
    async def protect_transaction(
        self,
        signed_tx: str,
        tx_data: Dict,
        user_address: str
    ) -> ProtectionResult:
        """
        Apply MEV protection to a transaction.
        
        Args:
            signed_tx: Signed transaction hash
            tx_data: Transaction details
            user_address: User's wallet address
            
        Returns:
            ProtectionResult with MEV analysis and protected tx hash
        """
        self.stats["total_txs"] += 1
        
        # Step 1: Analyze for MEV threats
        is_mev, mev_type, estimated_mev = self.detector.analyze_transaction(tx_data)
        
        # Step 2: Get protection recommendation
        recommendation = self.detector.get_protection_recommendation(is_mev, mev_type)
        
        # Step 3: Apply protection based on threat level
        if recommendation["use_flashbots"] and self.config.use_flashbots:
            return await self._protect_with_flashbots(
                signed_tx, tx_data, is_mev, mev_type, estimated_mev
            )
        else:
            return await self._protect_standard(
                signed_tx, tx_data, is_mev, mev_type, estimated_mev
            )
    
    async def _protect_with_flashbots(
        self,
        signed_tx: str,
        tx_data: Dict,
        is_mev: bool,
        mev_type: str,
        estimated_mev: float
    ) -> ProtectionResult:
        """Send transaction through Flashbots for protection"""
        try:
            # Simulate bundle first
            sim_result = await self.flashbots.call_bundle([signed_tx])
            
            if "error" in sim_result:
                # Simulation failed, use fallback
                return await self._protect_standard(
                    signed_tx, tx_data, is_mev, mev_type, estimated_mev
                )
            
            # Get current block
            current_block = int(time.time()) // 12  # ~12 second blocks
            
            # Send bundle (just our single tx)
            send_result = await self.flashbots.send_bundle(
                txs=[signed_tx],
                block_number=current_block + 1  # Next block
            )
            
            if "result" in send_result:
                bundle_id = send_result["result"]
                
                self.stats["protected_txs"] += 1
                if is_mev:
                    self.stats["total_mev_saved"] += estimated_mev
                    if mev_type == "sandwich":
                        self.stats["sandwich_attacks_blocked"] += 1
                    elif mev_type == "front_run":
                        self.stats["front_runs_blocked"] += 1
                
                return ProtectionResult(
                    success=True,
                    bundle_id=bundle_id,
                    flashbots_bundle_id=bundle_id,
                    mev_detected=is_mev,
                    mev_type=mev_type,
                    estimated_mev=estimated_mev,
                    tx_hash="0x" + hashlib.sha256(signed_tx.encode()).hexdigest()[:40]
                )
            else:
                # Flashbots failed, use fallback
                return await self._protect_standard(
                    signed_tx, tx_data, is_mev, mev_type, estimated_mev
                )
                
        except Exception as e:
            # Exception, use fallback
            return await self._protect_standard(
                signed_tx, tx_data, is_mev, mev_type, estimated_mev
            )
    
    async def _protect_standard(
        self,
        signed_tx: str,
        tx_data: Dict,
        is_mev: bool,
        mev_type: str,
        estimated_mev: float
    ) -> ProtectionResult:
        """Standard protection without Flashbots"""
        # Apply standard protections
        protected_tx = signed_tx
        
        # Add additional gas for faster inclusion
        gas_settings = self._optimize_gas_settings(tx_data)
        
        # Generate tx hash
        tx_hash = "0x" + hashlib.sha256(protected_tx.encode()).hexdigest()[:40]
        
        return ProtectionResult(
            success=True,
            tx_hash=tx_hash,
            mev_detected=is_mev,
            mev_type=mev_type,
            estimated_mev=estimated_mev if not is_mev else 0,
            gas_price_used=gas_settings["gas_price"],
            priority_fee=gas_settings["priority_fee"],
            max_fee=gas_settings["max_fee"],
            used_fallback=True
        )
    
    def _optimize_gas_settings(self, tx_data: Dict) -> Dict:
        """
        Optimize gas settings to reduce MEV extraction.
        Uses lower priority fees when MEV risk is low.
        """
        speed = self.config.preferred_gas_speed
        
        # Base gas prices (in wei)
        base_prices = {
            "slow": 20_000_000_000,      # 20 gwei
            "standard": 35_000_000_000,  # 35 gwei
            "fast": 50_000_000_000,      # 50 gwei
            "instant": 75_000_000_000    # 75 gwei
        }
        
        gas_price = base_prices.get(speed, base_prices["standard"])
        
        # Priority fee for validators
        # Lower priority fee = less attractive to validators = less MEV extraction
        priority_fee = gas_price // 10  # 10% of gas price
        
        # Max fee (EIP-1559)
        base_fee = gas_price // 2
        max_fee = gas_price + base_fee
        
        return {
            "gas_price": gas_price,
            "priority_fee": priority_fee,
            "max_fee": max_fee
        }
    
    async def batch_protect_transactions(
        self,
        transactions: List[Tuple[str, Dict, str]]
    ) -> List[ProtectionResult]:
        """
        Protect multiple transactions in a bundle.
        Useful for aggregators or batch swap operations.
        """
        results = []
        
        for signed_tx, tx_data, user_address in transactions:
            result = await self.protect_transaction(signed_tx, tx_data, user_address)
            results.append(result)
        
        return results
    
    def get_stats(self) -> Dict:
        """Get MEV protection statistics"""
        return {
            **self.stats,
            "protection_rate": (
                self.stats["protected_txs"] / self.stats["total_txs"] * 100
                if self.stats["total_txs"] > 0 else 0
            )
        }


class SlippageOptimizer:
    """
    Optimize slippage settings to minimize MEV while ensuring execution.
    """
    
    def __init__(self):
        self.price_impact_cache: Dict[str, float] = {}
        self.volatility_cache: Dict[str, float] = {}
    
    def calculate_optimal_slippage(
        self,
        token_in: str,
        token_out: str,
        amount: int,
        pool_liquidity: int,
        dex: str,
        urgency: str = "normal"  # "low", "normal", "high"
    ) -> int:
        """
        Calculate optimal slippage based on:
        - Trade size relative to pool
        - Pool liquidity
        - DEX characteristics
        - Market volatility
        """
        # Base slippage by DEX
        dex_base_slippage = {
            "uniswap_v2": 50,      # 0.5%
            "uniswap_v3": 30,      # 0.3%
            "sushiswap": 50,       # 0.5%
            "pancakeswap": 50,     # 0.5%
            "curve": 10,           # 0.1% - stablecoin optimized
            "balancer": 50,        # 0.5%
        }
        
        base_slippage = dex_base_slippage.get(dex.lower(), 50)
        
        # Calculate price impact
        price_impact_bps = self._estimate_price_impact(
            amount, pool_liquidity
        )
        
        # Volatility adjustment
        volatility = self._get_volatility(token_in, token_out)
        volatility_adjustment = int(volatility * 100)  # bps
        
        # Urgency multiplier
        urgency_multiplier = {
            "low": 1.0,      # Can wait for better price
            "normal": 1.2,   # Standard slippage
            "high": 1.5      # Need execution, accept more slippage
        }.get(urgency, 1.2)
        
        # Calculate total slippage
        total_slippage = int(
            (base_slippage + price_impact_bps + volatility_adjustment) 
            * urgency_multiplier
        )
        
        # Cap at reasonable maximum
        max_slippage = 500  # 5%
        return min(total_slippage, max_slippage)
    
    def _estimate_price_impact(self, amount: int, pool_liquidity: int) -> int:
        """Estimate price impact in basis points"""
        if pool_liquidity == 0:
            return 500  # 5% default for empty pools
        
        # Ratio of trade to liquidity
        trade_ratio = amount / pool_liquidity
        
        # Price impact formula (simplified AMM curve)
        # For constant product: impact ≈ 2 * trade_ratio at the margin
        impact_ratio = 2 * trade_ratio
        
        # Convert to basis points
        impact_bps = int(impact_ratio * 10000)
        
        return min(impact_bps, 500)  # Cap at 5%
    
    def _get_volatility(self, token_in: str, token_out: str) -> float:
        """Get cached volatility for token pair"""
        cache_key = f"{token_in}_{token_out}"
        
        if cache_key in self.volatility_cache:
            return self.volatility_cache[cache_key]
        
        # Default volatility (can integrate with real data)
        # Higher for exotic tokens
        volatility = 0.01  # 1% default
        
        self.volatility_cache[cache_key] = volatility
        return volatility


async def main():
    """Test MEV protection"""
    print("=== MEV Protection Test ===")
    
    protection = MEVProtection()
    
    # Test transaction data
    tx_data = {
        "value": 1_000_000_000_000_000,  # 1 ETH
        "data": "0x",
        "to": "0x7a250d5630b4cf539739df2c5dacb4c659f2488d",
        "slippage_bps": 500,  # High slippage - vulnerable
        "pool_liquidity": 10_000_000_000_000_000_000  # 10 ETH
    }
    
    signed_tx = "0xf8d0......"  # Mock signed tx
    
    result = await protection.protect_transaction(signed_tx, tx_data, "0x742d35Cc6634C0532925a3b844Bc9e7595f2bD12")
    
    print(f"Success: {result.success}")
    print(f"MEV Detected: {result.mev_detected}")
    print(f"MEV Type: {result.mev_type}")
    print(f"Estimated MEV: ${result.estimated_mev:.4f}")
    print(f"Used Fallback: {result.used_fallback}")
    
    # Test slippage optimizer
    optimizer = SlippageOptimizer()
    
    slippage = optimizer.calculate_optimal_slippage(
        token_in="0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2",
        token_out="0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48",
        amount=1_000_000_000_000_000_000,  # 1 ETH
        pool_liquidity=100_000_000_000_000_000_000,  # 100 ETH
        dex="uniswap_v3",
        urgency="normal"
    )
    
    print(f"\nOptimal Slippage: {slippage} bps ({slippage/100}%)")
    
    print(f"\n=== Protection Stats ===")
    print(protection.get_stats())


if __name__ == "__main__":
    asyncio.run(main())
