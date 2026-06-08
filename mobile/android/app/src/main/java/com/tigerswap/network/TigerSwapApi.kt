package com.tigerswap.network

import com.tigerswap.data.*
import io.ktor.client.*
import io.ktor.client.call.*
import io.ktor.client.request.*
import io.ktor.http.*
import kotlinx.serialization.json.Json

class TigerSwapApi(
    private val baseUrl: String = "https://api.tigerswap.com/v1"
) {
    private val json = Json {
        ignoreUnknownKeys = true
        isLenient = true
    }
    
    private val client = HttpClient {
        install(io.ktor.client.plugins.contentnegotiation.ContentNegotiation) {
            json(json)
        }
    }
    
    // Get quote for swap
    suspend fun getQuote(
        fromToken: String,
        toToken: String,
        amount: String,
        slippage: Double = 0.5
    ): Result<SwapQuote> = runCatching {
        client.get("$baseUrl/quote") {
            parameter("fromToken", fromToken)
            parameter("toToken", toToken)
            parameter("amount", amount)
            parameter("slippage", slippage)
        }.body()
    }
    
    // Execute swap
    suspend fun executeSwap(
        fromToken: String,
        toToken: String,
        amount: String,
        slippage: Double = 0.5,
        fromAddress: String
    ): Result<Transaction> = runCatching {
        client.post("$baseUrl/swap") {
            contentType(ContentType.Application.Json)
            setBody(mapOf(
                "fromToken" to fromToken,
                "toToken" to toToken,
                "amount" to amount,
                "slippage" to slippage,
                "fromAddress" to fromAddress
            ))
        }.body()
    }
    
    // Get pools
    suspend fun getPools(
        token0: String? = null,
        token1: String? = null
    ): Result<List<Pool>> = runCatching {
        client.get("$baseUrl/pools") {
            token0?.let { parameter("token0", it) }
            token1?.let { parameter("token1", it) }
        }.body()
    }
    
    // Get tokens
    suspend fun getTokens(chainId: Int? = null): Result<List<Token>> = runCatching {
        client.get("$baseUrl/tokens") {
            chainId?.let { parameter("chainId", it) }
        }.body()
    }
    
    // Get portfolio
    suspend fun getPortfolio(address: String): Result<Portfolio> = runCatching {
        client.get("$baseUrl/portfolio/$address").body()
    }
    
    // Get transaction history
    suspend fun getTransactions(
        address: String,
        limit: Int = 50
    ): Result<List<Transaction>> = runCatching {
        client.get("$baseUrl/transactions") {
            parameter("address", address)
            parameter("limit", limit)
        }.body()
    }
    
    // Add liquidity
    suspend fun addLiquidity(
        token0: String,
        token1: String,
        amount0: String,
        amount1: String,
        fromAddress: String
    ): Result<Transaction> = runCatching {
        client.post("$baseUrl/liquidity/add") {
            contentType(ContentType.Application.Json)
            setBody(mapOf(
                "token0" to token0,
                "token1" to token1,
                "amount0" to amount0,
                "amount1" to amount1,
                "fromAddress" to fromAddress
            ))
        }.body()
    }
    
    // Remove liquidity
    suspend fun removeLiquidity(
        pool: String,
        liquidity: String,
        fromAddress: String
    ): Result<Transaction> = runCatching {
        client.post("$baseUrl/liquidity/remove") {
            contentType(ContentType.Application.Json)
            setBody(mapOf(
                "pool" to pool,
                "liquidity" to liquidity,
                "fromAddress" to fromAddress
            ))
        }.body()
    }
    
    // Get supported chains
    suspend fun getChains(): Result<List<Chain>> = runCatching {
        client.get("$baseUrl/chains").body()
    }
    
    fun close() {
        client.close()
    }
}