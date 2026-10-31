"""
Contract ABIs and the TigerSwap deployment address book.

Every ABI here targets a TigerSwap contract deployed by this repository.
No third-party DEX router, factory or quoter is referenced.
"""

import os
from typing import Dict, Optional

# Minimal ERC-20 interface used for balances, allowances and approvals.
ERC20_ABI = [
    {"constant": True, "inputs": [], "name": "name", "outputs": [{"name": "", "type": "string"}], "type": "function"},
    {"constant": True, "inputs": [], "name": "symbol", "outputs": [{"name": "", "type": "string"}], "type": "function"},
    {"constant": True, "inputs": [], "name": "decimals", "outputs": [{"name": "", "type": "uint8"}], "type": "function"},
    {"constant": True, "inputs": [], "name": "totalSupply", "outputs": [{"name": "", "type": "uint256"}], "type": "function"},
    {"constant": True, "inputs": [{"name": "account", "type": "address"}], "name": "balanceOf", "outputs": [{"name": "", "type": "uint256"}], "type": "function"},
    {"constant": True, "inputs": [{"name": "owner", "type": "address"}, {"name": "spender", "type": "address"}], "name": "allowance", "outputs": [{"name": "", "type": "uint256"}], "type": "function"},
    {"constant": False, "inputs": [{"name": "spender", "type": "address"}, {"name": "value", "type": "uint256"}], "name": "approve", "outputs": [{"name": "", "type": "bool"}], "type": "function"},
    {"constant": False, "inputs": [{"name": "to", "type": "address"}, {"name": "value", "type": "uint256"}], "name": "transfer", "outputs": [{"name": "", "type": "bool"}], "type": "function"},
]

# TigerSwapRouter: quoting, swaps, liquidity (contracts/TigerSwapRouter.sol).
ROUTER_ABI = [
    {"inputs": [{"internalType": "uint256", "name": "amountIn", "type": "uint256"}, {"internalType": "address[]", "name": "path", "type": "address[]"}], "name": "getAmountsOut", "outputs": [{"internalType": "uint256[]", "name": "amounts", "type": "uint256[]"}], "stateMutability": "view", "type": "function"},
    {"inputs": [{"internalType": "uint256", "name": "amountOut", "type": "uint256"}, {"internalType": "address[]", "name": "path", "type": "address[]"}], "name": "getAmountsIn", "outputs": [{"internalType": "uint256[]", "name": "amounts", "type": "uint256[]"}], "stateMutability": "view", "type": "function"},
    {"inputs": [{"internalType": "uint256", "name": "amountIn", "type": "uint256"}, {"internalType": "uint256", "name": "amountOutMin", "type": "uint256"}, {"internalType": "address[]", "name": "path", "type": "address[]"}, {"internalType": "address", "name": "to", "type": "address"}, {"internalType": "uint256", "name": "deadline", "type": "uint256"}], "name": "swapExactTokensForTokens", "outputs": [{"internalType": "uint256[]", "name": "amounts", "type": "uint256[]"}], "stateMutability": "nonpayable", "type": "function"},
    {"inputs": [{"internalType": "uint256", "name": "amountOutMin", "type": "uint256"}, {"internalType": "address[]", "name": "path", "type": "address[]"}, {"internalType": "address", "name": "to", "type": "address"}, {"internalType": "uint256", "name": "deadline", "type": "uint256"}], "name": "swapExactETHForTokens", "outputs": [{"internalType": "uint256[]", "name": "amounts", "type": "uint256[]"}], "stateMutability": "payable", "type": "function"},
    {"inputs": [{"internalType": "uint256", "name": "amountOut", "type": "uint256"}, {"internalType": "uint256", "name": "amountInMax", "type": "uint256"}, {"internalType": "address[]", "name": "path", "type": "address[]"}, {"internalType": "address", "name": "to", "type": "address"}, {"internalType": "uint256", "name": "deadline", "type": "uint256"}], "name": "swapTokensForExactETH", "outputs": [{"internalType": "uint256[]", "name": "amounts", "type": "uint256[]"}], "stateMutability": "nonpayable", "type": "function"},
    {"inputs": [{"internalType": "address", "name": "tokenA", "type": "address"}, {"internalType": "address", "name": "tokenB", "type": "address"}, {"internalType": "uint256", "name": "amountADesired", "type": "uint256"}, {"internalType": "uint256", "name": "amountBDesired", "type": "uint256"}, {"internalType": "uint256", "name": "amountAMin", "type": "uint256"}, {"internalType": "uint256", "name": "amountBMin", "type": "uint256"}, {"internalType": "address", "name": "to", "type": "address"}, {"internalType": "uint256", "name": "deadline", "type": "uint256"}], "name": "addLiquidity", "outputs": [{"internalType": "uint256", "name": "amountA", "type": "uint256"}, {"internalType": "uint256", "name": "amountB", "type": "uint256"}, {"internalType": "uint256", "name": "liquidity", "type": "uint256"}], "stateMutability": "nonpayable", "type": "function"},
    {"inputs": [{"internalType": "address", "name": "tokenA", "type": "address"}, {"internalType": "address", "name": "tokenB", "type": "address"}, {"internalType": "uint256", "name": "liquidity", "type": "uint256"}, {"internalType": "uint256", "name": "amountAMin", "type": "uint256"}, {"internalType": "uint256", "name": "amountBMin", "type": "uint256"}, {"internalType": "address", "name": "to", "type": "address"}, {"internalType": "uint256", "name": "deadline", "type": "uint256"}], "name": "removeLiquidity", "outputs": [{"internalType": "uint256", "name": "amountA", "type": "uint256"}, {"internalType": "uint256", "name": "amountB", "type": "uint256"}], "stateMutability": "nonpayable", "type": "function"},
]

# TigerSwapFactory: pool discovery (contracts/TigerSwapFactory.sol).
FACTORY_ABI = [
    {"inputs": [{"internalType": "address", "name": "tokenA", "type": "address"}, {"internalType": "address", "name": "tokenB", "type": "address"}], "name": "getPoolAddress", "outputs": [{"internalType": "address", "name": "", "type": "address"}], "stateMutability": "view", "type": "function"},
    {"inputs": [{"internalType": "address", "name": "tokenA", "type": "address"}, {"internalType": "address", "name": "tokenB", "type": "address"}], "name": "createPool", "outputs": [{"internalType": "address", "name": "pool", "type": "address"}], "stateMutability": "nonpayable", "type": "function"},
    {"inputs": [], "name": "allPoolsLength", "outputs": [{"internalType": "uint256", "name": "", "type": "uint256"}], "stateMutability": "view", "type": "function"},
]

# TigerSwapPool: reserves and LP token (contracts/TigerSwapFactory.sol).
POOL_ABI = [
    {"inputs": [], "name": "getReserves", "outputs": [{"internalType": "uint112", "name": "reserve0", "type": "uint112"}, {"internalType": "uint112", "name": "reserve1", "type": "uint112"}, {"internalType": "uint32", "name": "blockTimestampLast", "type": "uint32"}], "stateMutability": "view", "type": "function"},
    {"inputs": [], "name": "token0", "outputs": [{"internalType": "address", "name": "", "type": "address"}], "stateMutability": "view", "type": "function"},
    {"inputs": [], "name": "token1", "outputs": [{"internalType": "address", "name": "", "type": "address"}], "stateMutability": "view", "type": "function"},
    {"inputs": [], "name": "totalSupply", "outputs": [{"internalType": "uint256", "name": "", "type": "uint256"}], "stateMutability": "view", "type": "function"},
    {"inputs": [{"internalType": "address", "name": "account", "type": "address"}], "name": "balanceOf", "outputs": [{"internalType": "uint256", "name": "", "type": "uint256"}], "stateMutability": "view", "type": "function"},
]

# TigerSwapPerpetuals: oracle-priced perp engine
# (contracts/TigerSwapPerpetuals.sol).
PERPETUALS_ABI = [
    {"inputs": [{"internalType": "bytes32", "name": "marketId", "type": "bytes32"}], "name": "getMarket", "outputs": [{"components": [{"internalType": "address", "name": "token", "type": "address"}, {"internalType": "uint256", "name": "maxLeverage", "type": "uint256"}, {"internalType": "uint256", "name": "liquidationFee", "type": "uint256"}, {"internalType": "uint256", "name": "maintenanceMargin", "type": "uint256"}, {"internalType": "uint256", "name": "initialMargin", "type": "uint256"}, {"internalType": "bool", "name": "isActive", "type": "bool"}, {"internalType": "uint256", "name": "openFee", "type": "uint256"}, {"internalType": "uint256", "name": "maxOpenInterest", "type": "uint256"}, {"internalType": "uint256", "name": "openInterest", "type": "uint256"}], "internalType": "struct TigerSwapPerpetuals.MarketView", "name": "", "type": "tuple"}], "stateMutability": "view", "type": "function"},
    {"inputs": [{"internalType": "address", "name": "token", "type": "address"}, {"internalType": "uint256", "name": "amount", "type": "uint256"}], "name": "addLiquidity", "outputs": [{"internalType": "uint256", "name": "shares", "type": "uint256"}], "stateMutability": "nonpayable", "type": "function"},
    {"inputs": [{"internalType": "address", "name": "token", "type": "address"}, {"internalType": "uint256", "name": "shares", "type": "uint256"}], "name": "removeLiquidity", "outputs": [{"internalType": "uint256", "name": "amount", "type": "uint256"}], "stateMutability": "nonpayable", "type": "function"},
    {"inputs": [{"internalType": "address", "name": "token", "type": "address"}], "name": "poolLiquidity", "outputs": [{"internalType": "uint256", "name": "", "type": "uint256"}], "stateMutability": "view", "type": "function"},
    {"inputs": [{"internalType": "address", "name": "token", "type": "address"}], "name": "freeLiquidity", "outputs": [{"internalType": "uint256", "name": "", "type": "uint256"}], "stateMutability": "view", "type": "function"},
    {"inputs": [{"internalType": "address", "name": "token", "type": "address"}, {"internalType": "address", "name": "provider", "type": "address"}], "name": "poolShares", "outputs": [{"internalType": "uint256", "name": "", "type": "uint256"}], "stateMutability": "view", "type": "function"},
    {"inputs": [{"internalType": "address", "name": "trader", "type": "address"}], "name": "getTraderPositions", "outputs": [{"internalType": "bytes32[]", "name": "", "type": "bytes32[]"}], "stateMutability": "view", "type": "function"},
    {"inputs": [{"internalType": "address", "name": "trader", "type": "address"}], "name": "getOpenPositions", "outputs": [{"internalType": "bytes32[]", "name": "open", "type": "bytes32[]"}], "stateMutability": "view", "type": "function"},
    {"inputs": [{"internalType": "bytes32", "name": "marketId", "type": "bytes32"}], "name": "getMarkPrice", "outputs": [{"internalType": "uint256", "name": "", "type": "uint256"}], "stateMutability": "view", "type": "function"},
    {"inputs": [{"internalType": "bytes32", "name": "marketId", "type": "bytes32"}], "name": "getFundingInfo", "outputs": [{"internalType": "int256", "name": "lastFundingRate", "type": "int256"}, {"internalType": "uint256", "name": "lastUpdateTime", "type": "uint256"}, {"internalType": "uint256", "name": "markPrice", "type": "uint256"}, {"internalType": "uint256", "name": "indexPrice", "type": "uint256"}], "stateMutability": "view", "type": "function"},
    {"inputs": [{"internalType": "bytes32", "name": "positionId", "type": "bytes32"}], "name": "getPosition", "outputs": [{"internalType": "bytes32", "name": "marketId", "type": "bytes32"}, {"internalType": "address", "name": "trader", "type": "address"}, {"internalType": "uint256", "name": "size", "type": "uint256"}, {"internalType": "uint256", "name": "collateral", "type": "uint256"}, {"internalType": "uint256", "name": "entryPrice", "type": "uint256"}, {"internalType": "uint256", "name": "lastUpdated", "type": "uint256"}, {"internalType": "bool", "name": "isLong", "type": "bool"}], "stateMutability": "view", "type": "function"},
    {"inputs": [{"internalType": "bytes32", "name": "marketId", "type": "bytes32"}, {"internalType": "uint256", "name": "size", "type": "uint256"}, {"internalType": "uint256", "name": "collateral", "type": "uint256"}, {"internalType": "bool", "name": "isLong", "type": "bool"}], "name": "openPosition", "outputs": [{"internalType": "bytes32", "name": "positionId", "type": "bytes32"}], "stateMutability": "nonpayable", "type": "function"},
    {"inputs": [{"internalType": "bytes32", "name": "positionId", "type": "bytes32"}], "name": "closePosition", "outputs": [], "stateMutability": "nonpayable", "type": "function"},
    {"inputs": [{"internalType": "bytes32", "name": "positionId", "type": "bytes32"}, {"internalType": "uint256", "name": "amount", "type": "uint256"}], "name": "addMargin", "outputs": [], "stateMutability": "nonpayable", "type": "function"},
    {"inputs": [{"internalType": "bytes32", "name": "positionId", "type": "bytes32"}, {"internalType": "uint256", "name": "amount", "type": "uint256"}], "name": "removeMargin", "outputs": [], "stateMutability": "nonpayable", "type": "function"},
    {"inputs": [{"internalType": "bytes32", "name": "positionId", "type": "bytes32"}], "name": "liquidatePosition", "outputs": [], "stateMutability": "nonpayable", "type": "function"},
]

# TigerSwapStaking: stake/unstake/claim (contracts/TigerSwapStaking.sol).

STAKING_ABI = [
    {"inputs": [{"internalType": "uint256", "name": "amount", "type": "uint256"}], "name": "stake", "outputs": [], "stateMutability": "nonpayable", "type": "function"},
    {"inputs": [{"internalType": "uint256", "name": "amount", "type": "uint256"}], "name": "withdraw", "outputs": [], "stateMutability": "nonpayable", "type": "function"},
    {"inputs": [], "name": "claimRewards", "outputs": [], "stateMutability": "nonpayable", "type": "function"},
    {"inputs": [{"internalType": "address", "name": "account", "type": "address"}], "name": "balanceOf", "outputs": [{"internalType": "uint256", "name": "", "type": "uint256"}], "stateMutability": "view", "type": "function"},
    {"inputs": [{"internalType": "address", "name": "account", "type": "address"}], "name": "earned", "outputs": [{"internalType": "uint256", "name": "", "type": "uint256"}], "stateMutability": "view", "type": "function"},
    {"inputs": [], "name": "rewardRate", "outputs": [{"internalType": "uint256", "name": "", "type": "uint256"}], "stateMutability": "view", "type": "function"},
]

# The contracts a TigerSwap deployment is made of. Addresses are read from the
# operator's environment rather than baked in, so a release carries no address
# that is wrong on some network. A chain with no configured address is treated
# as undeployed and the SDK raises instead of silently using a third-party
# contract.
CONTRACT_NAMES = ("router", "factory", "staking", "perpetuals")

ZERO_ADDRESS = "0x0000000000000000000000000000000000000000"

# Canonical wrapped-native token per chain. These are the chains' own wrapped
# tokens, not a third-party service, and are only used when an operator has not
# configured one.
WRAPPED_NATIVE: Dict[int, str] = {
    1: "0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2",
    56: "0xbb4CdB9CBd36B01bD1cBaEBF2De08d9173bc095c",
    137: "0x0d500B1d8E8eF31E21C99d1Db9A6444d3ADf1270",
}


class DeploymentError(RuntimeError):
    """Raised when a TigerSwap contract address is not configured."""


def _env_address(chain_id: int, name: str) -> Optional[str]:
    """Read a contract address from the environment, rejecting non-addresses.

    Accepts `TIGERSWAP_<NAME>_<CHAIN_ID>`, so one process can serve several
    chains. A malformed value raises rather than being ignored, because a typo
    in an address must never be interpreted as "not configured".
    """
    raw = os.getenv(f"TIGERSWAP_{name.upper()}_{chain_id}")
    if raw is None or raw.strip() == "":
        return None
    value = raw.strip()
    if not _is_address(value):
        raise DeploymentError(
            f"TIGERSWAP_{name.upper()}_{chain_id} is not a 20-byte hex address"
        )
    return value


def _is_address(value: str) -> bool:
    """Whether `value` is a 20-byte hex address."""
    if not isinstance(value, str) or not value.startswith("0x") or len(value) != 42:
        return False
    try:
        int(value[2:], 16)
    except ValueError:
        return False
    return True


def get_deployment(chain_id: int) -> Dict[str, str]:
    """Return the TigerSwap address book for a chain.

    Only contracts the operator has configured are present, so a caller can use
    membership to tell whether an optional contract (staking, perpetuals) is
    available on that chain.
    """
    deployment: Dict[str, str] = {}
    for name in CONTRACT_NAMES:
        address = _env_address(chain_id, name)
        if address is not None:
            deployment[name] = address

    wrapped = os.getenv(f"TIGERSWAP_WETH_{chain_id}") or WRAPPED_NATIVE.get(chain_id)
    if wrapped:
        deployment["weth"] = wrapped

    return deployment


def get_contract_address(chain_id: int, name: str) -> str:
    """Return a single configured contract address.

    Raises when the contract is absent, so a caller can never send a
    transaction to the zero address.
    """
    if name not in CONTRACT_NAMES and name != "weth":
        raise DeploymentError(f"{name} is not a TigerSwap contract name")

    address = get_deployment(chain_id).get(name)
    if not address:
        raise DeploymentError(
            f"{name} is not configured on chain {chain_id}. "
            f"Set TIGERSWAP_{name.upper()}_{chain_id} to the deployed address."
        )
    if address == ZERO_ADDRESS:
        raise DeploymentError(
            f"{name} on chain {chain_id} is the zero address. "
            "Set the real deployed address before use."
        )
    return address


def is_deployed(chain_id: int) -> bool:
    """Whether the TigerSwap router and factory are configured for a chain."""
    try:
        router = get_contract_address(chain_id, "router")
        factory = get_contract_address(chain_id, "factory")
    except DeploymentError:
        return False
    return router != ZERO_ADDRESS and factory != ZERO_ADDRESS