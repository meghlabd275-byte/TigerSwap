package com.tigerswap.data

import kotlinx.serialization.Serializable

@Serializable
data class Token(
    val address: String,
    val symbol: String,
    val name: String,
    val decimals: Int,
    val logoUrl: String = "",
    val price: Double = 0.0,
    val priceChange24h: Double = 0.0,
    val totalValueLocked: Double = 0.0,
    val volume24h: Double = 0.0
)

@Serializable
data class Pool(
    val id: String,
    val token0: Token,
    val token1: Token,
    val feeTier: Double,
    val liquidity: Double,
    val valueLockedUSD: Double,
    val volume24hUSD: Double,
    val apr: Double,
    val token0Price: Double = 0.0,
    val token1Price: Double = 0.0
)

@Serializable
data class SwapQuote(
    val fromToken: String,
    val toToken: String,
    val fromAmount: String,
    val toAmount: String,
    val priceImpact: Double,
    val route: List<String>,
    val gasEstimate: String
)

@Serializable
data class Portfolio(
    val totalValueUSD: Double = 0.0,
    val tokens: List<TokenBalance> = emptyList(),
    val pools: List<PoolBalance> = emptyList()
)

@Serializable
data class TokenBalance(
    val token: Token,
    val balance: String,
    val valueUSD: Double,
    val balanceRaw: String
)

@Serializable
data class PoolBalance(
    val pool: Pool,
    val liquidityTokenBalance: String,
    val valueUSD: Double,
    val feesEarnedUSD: Double,
    val poolToken0Balance: String,
    val poolToken1Balance: String
)

@Serializable
data class Transaction(
    val hash: String,
    val from: String,
    val to: String,
    val value: String,
    val timestamp: Long,
    val status: TransactionStatus,
    val type: TransactionType,
    val tokenIn: String? = null,
    val tokenOut: String? = null,
    val amountIn: String? = null,
    val amountOut: String? = null
)

@Serializable
enum class TransactionStatus {
    pending, confirmed, failed
}

@Serializable
enum class TransactionType {
    swap, addLiquidity, removeLiquidity, transfer, approve
}

@Serializable
data class User(
    val address: String,
    val chainId: Int,
    val portfolio: Portfolio = Portfolio()
)

@Serializable
data class Chain(
    val id: Int,
    val name: String,
    val symbol: String,
    val rpcUrl: String,
    val explorerUrl: String,
    val nativeToken: Token
)