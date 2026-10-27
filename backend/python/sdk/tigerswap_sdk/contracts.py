"""
Contract ABIs and the TigerSwap deployment address book.

Every ABI here targets a TigerSwap contract deployed by this repository.
No third-party DEX router, factory or quoter is referenced.
"""

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

# Per-chain TigerSwap deployment. Addresses are filled in per network at
# deployment time; a chain absent from this table has no TigerSwap deployment
# and the SDK raises rather than silently using a third-party contract.
DEPLOYMENTS: Dict[int, Dict[str, str]] = {
    1: {
        "router": "0x0000000000000000000000000000000000000000",
        "factory": "0x0000000000000000000000000000000000000000",
        "staking": "0x0000000000000000000000000000000000000000",
        "perpetuals": "0x0000000000000000000000000000000000000000",
        "weth": "0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2",
    },
    56: {
        "router": "0x0000000000000000000000000000000000000000",
        "factory": "0x0000000000000000000000000000000000000000",
        "staking": "0x0000000000000000000000000000000000000000",
        "perpetuals": "0x0000000000000000000000000000000000000000",
        "weth": "0xbb4CdB9CBd36B01bD1cBaEBF2De08d9173bc095c",
    },
    137: {
        "router": "0x0000000000000000000000000000000000000000",
        "factory": "0x0000000000000000000000000000000000000000",
        "staking": "0x0000000000000000000000000000000000000000",
        "perpetuals": "0x0000000000000000000000000000000000000000",
        "weth": "0x0d500B1d8E8eF31E21C99d1Db9A6444d3ADf1270",
    },
}


class DeploymentError(RuntimeError):
    """Raised when a TigerSwap contract address is not configured."""


def get_deployment(chain_id: int) -> Dict[str, str]:
    """Return the TigerSwap address book for a chain."""
    deployment = DEPLOYMENTS.get(chain_id)
    if deployment is None:
        raise DeploymentError(
            f"TigerSwap is not deployed on chain {chain_id}. "
            "Add the deployment addresses to contracts.DEPLOYMENTS."
        )
    return deployment


def get_contract_address(chain_id: int, name: str) -> str:
    """Return a single contract address, validating that it is configured."""
    address = get_deployment(chain_id).get(name)
    if not address:
        raise DeploymentError(f"{name} is not configured on chain {chain_id}")
    if address == "0x0000000000000000000000000000000000000000":
        raise DeploymentError(
            f"{name} on chain {chain_id} is the zero address. "
            "Deploy TigerSwap and set the real address before use."
        )
    return address


def is_deployed(chain_id: int) -> bool:
    """Whether the TigerSwap router and factory are configured for a chain."""
    deployment = DEPLOYMENTS.get(chain_id)
    if not deployment:
        return False
    zero = "0x0000000000000000000000000000000000000000"
    return deployment.get("router") != zero and deployment.get("factory") != zero
