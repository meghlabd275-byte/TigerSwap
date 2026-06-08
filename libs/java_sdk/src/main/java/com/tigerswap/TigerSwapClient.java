package com.tigerswap;

import java.io.*;
import java.net.*;
import java.nio.charset.StandardCharsets;
import java.time.Instant;
import java.util.*;
import com.fasterxml.jackson.databind.*;

/**
 * TigerSwap Java SDK
 * 
 * Enterprise-grade Java SDK for TigerSwap DEX
 */
public class TigerSwapClient {
    private final String apiKey;
    private final String baseUrl;
    private final OkHttpClient httpClient;
    private final ObjectMapper objectMapper;
    
    /**
     * Create a new TigerSwap client
     */
    public TigerSwapClient(String apiKey, String baseUrl) {
        this.apiKey = apiKey;
        this.baseUrl = baseUrl;
        this.httpClient = new OkHttpClient();
        this.objectMapper = new ObjectMapper();
    }
    
    /**
     * Get token quote
     */
    public Quote getQuote(String fromToken, String toToken, String amount) throws IOException {
        String url = String.format("%s/v1/quote?from=%s&to=%s&amount=%s", 
            baseUrl, fromToken, toToken, amount);
        
        Request request = new Request.Builder()
            .url(url)
            .addHeader("Authorization", "Bearer " + apiKey)
            .addHeader("Content-Type", "application/json")
            .get()
            .build();
        
        try (Response response = httpClient.newCall(request).execute()) {
            if (!response.isSuccessful()) {
                throw new IOException("Quote failed: " + response.code());
            }
            return objectMapper.readValue(response.body().string(), Quote.class);
        }
    }
    
    /**
     * Execute swap
     */
    public SwapResponse swap(SwapRequest request) throws IOException {
        String url = baseUrl + "/v1/swap";
        
        RequestBody body = RequestBody.create(
            objectMapper.writeValueAsString(request),
            MediaType.parse("application/json")
        );
        
        Request httpRequest = new Request.Builder()
            .url(url)
            .addHeader("Authorization", "Bearer " + apiKey)
            .addHeader("Content-Type", "application/json")
            .post(body)
            .build();
        
        try (Response response = httpClient.newCall(httpRequest).execute()) {
            if (!response.isSuccessful()) {
                throw new IOException("Swap failed: " + response.code());
            }
            return objectMapper.readValue(response.body().string(), SwapResponse.class);
        }
    }
    
    /**
     * Get supported tokens
     */
    public List<Token> getTokens(long chainId) throws IOException {
        String url = String.format("%s/v1/tokens?chain_id=%d", baseUrl, chainId);
        
        Request request = new Request.Builder()
            .url(url)
            .addHeader("Authorization", "Bearer " + apiKey)
            .get()
            .build();
        
        try (Response response = httpClient.newCall(request).execute()) {
            if (!response.isSuccessful()) {
                throw new IOException("Get tokens failed: " + response.code());
            }
            return objectMapper.readValue(response.body().string(), 
                objectMapper.getTypeFactory().constructCollectionType(List.class, Token.class));
        }
    }
    
    /**
     * Get order book
     */
    public OrderBook getOrderBook(String pair) throws IOException {
        String url = baseUrl + "/v1/orderbook/" + pair;
        
        Request request = new Request.Builder()
            .url(url)
            .addHeader("Authorization", "Bearer " + apiKey)
            .get()
            .build();
        
        try (Response response = httpClient.newCall(request).execute()) {
            if (!response.isSuccessful()) {
                throw new IOException("Get orderbook failed: " + response.code());
            }
            return objectMapper.readValue(response.body().string(), OrderBook.class);
        }
    }
    
    /**
     * Get market data
     */
    public MarketData getMarket(String pair) throws IOException {
        String url = baseUrl + "/v1/market/" + pair;
        
        Request request = new Request.Builder()
            .url(url)
            .addHeader("Authorization", "Bearer " + apiKey)
            .get()
            .build();
        
        try (Response response = httpClient.newCall(request).execute()) {
            if (!response.isSuccessful()) {
                throw new IOException("Get market failed: " + response.code());
            }
            return objectMapper.readValue(response.body().string(), MarketData.class);
        }
    }
    
    /**
     * Get user's orders
     */
    public List<Order> getOrders(String user) throws IOException {
        String url = baseUrl + "/v1/orders?user=" + user;
        
        Request request = new Request.Builder()
            .url(url)
            .addHeader("Authorization", "Bearer " + apiKey)
            .get()
            .build();
        
        try (Response response = httpClient.newCall(request).execute()) {
            if (!response.isSuccessful()) {
                throw new IOException("Get orders failed: " + response.code());
            }
            return objectMapper.readValue(response.body().string(),
                objectMapper.getTypeFactory().constructCollectionType(List.class, Order.class));
        }
    }
    
    /**
     * Create order
     */
    public Order createOrder(Order order) throws IOException {
        String url = baseUrl + "/v1/orders";
        
        RequestBody body = RequestBody.create(
            objectMapper.writeValueAsString(order),
            MediaType.parse("application/json")
        );
        
        Request request = new Request.Builder()
            .url(url)
            .addHeader("Authorization", "Bearer " + apiKey)
            .addHeader("Content-Type", "application/json")
            .post(body)
            .build();
        
        try (Response response = httpClient.newCall(request).execute()) {
            if (!response.isSuccessful()) {
                throw new IOException("Create order failed: " + response.code());
            }
            return objectMapper.readValue(response.body().string(), Order.class);
        }
    }
    
    /**
     * Cancel order
     */
    public void cancelOrder(String orderId) throws IOException {
        String url = baseUrl + "/v1/orders/" + orderId;
        
        Request request = new Request.Builder()
            .url(url)
            .addHeader("Authorization", "Bearer " + apiKey)
            .delete()
            .build();
        
        try (Response response = httpClient.newCall(request).execute()) {
            if (!response.isSuccessful()) {
                throw new IOException("Cancel order failed: " + response.code());
            }
        }
    }
    
    /**
     * Get positions
     */
    public List<Position> getPositions(String user) throws IOException {
        String url = baseUrl + "/v1/positions?user=" + user;
        
        Request request = new Request.Builder()
            .url(url)
            .addHeader("Authorization", "Bearer " + apiKey)
            .get()
            .build();
        
        try (Response response = httpClient.newCall(request).execute()) {
            if (!response.isSuccessful()) {
                throw new IOException("Get positions failed: " + response.code());
            }
            return objectMapper.readValue(response.body().string(),
                objectMapper.getTypeFactory().constructCollectionType(List.class, Position.class));
        }
    }
    
    /**
     * Get portfolio
     */
    public Portfolio getPortfolio(String user) throws IOException {
        String url = baseUrl + "/v1/portfolio?user=" + user;
        
        Request request = new Request.Builder()
            .url(url)
            .addHeader("Authorization", "Bearer " + apiKey)
            .get()
            .build();
        
        try (Response response = httpClient.newCall(request).execute()) {
            if (!response.isSuccessful()) {
                throw new IOException("Get portfolio failed: " + response.code());
            }
            return objectMapper.readValue(response.body().string(), Portfolio.class);
        }
    }
    
    /**
     * Get pools
     */
    public List<Pool> getPools() throws IOException {
        String url = baseUrl + "/v1/pools";
        
        Request request = new Request.Builder()
            .url(url)
            .addHeader("Authorization", "Bearer " + apiKey)
            .get()
            .build();
        
        try (Response response = httpClient.newCall(request).execute()) {
            if (!response.isSuccessful()) {
                throw new IOException("Get pools failed: " + response.code());
            }
            return objectMapper.readValue(response.body().string(),
                objectMapper.getTypeFactory().constructCollectionType(List.class, Pool.class));
        }
    }
    
    /**
     * Get supported chains
     */
    public List<Chain> getChains() throws IOException {
        String url = baseUrl + "/v1/chains";
        
        Request request = new Request.Builder()
            .url(url)
            .addHeader("Authorization", "Bearer " + apiKey)
            .get()
            .build();
        
        try (Response response = httpClient.newCall(request).execute()) {
            if (!response.isSuccessful()) {
                throw new IOException("Get chains failed: " + response.code());
            }
            return objectMapper.readValue(response.body().string(),
                objectMapper.getTypeFactory().constructCollectionType(List.class, Chain.class));
        }
    }
    
    // Inner classes for data models
    
    public static class Quote {
        public String fromToken;
        public String toToken;
        public String fromAmount;
        public String toAmount;
        public String price;
        public String priceImpact;
        public String gas;
    }
    
    public static class SwapRequest {
        public String fromToken;
        public String toToken;
        public String amount;
        public int slippage;
        public String to;
    }
    
    public static class SwapResponse {
        public String txHash;
        public String fromToken;
        public String toToken;
        public String fromAmount;
        public String toAmount;
        public String status;
    }
    
    public static class Token {
        public String address;
        public String symbol;
        public String name;
        public int decimals;
        public long chainId;
        public String logoUrl;
    }
    
    public static class OrderBook {
        public String pair;
        public List<OrderBookEntry> bids;
        public List<OrderBookEntry> asks;
        public long updatedAt;
    }
    
    public static class OrderBookEntry {
        public String price;
        public String quantity;
    }
    
    public static class MarketData {
        public String pair;
        public String price;
        public String price24hAgo;
        public String change24h;
        public String volume24h;
        public String liquidity;
        public String high24h;
        public String low24h;
    }
    
    public static class Order {
        public String id;
        public String user;
        public String pair;
        public String side;
        public String orderType;
        public String price;
        public String quantity;
        public String filled;
        public String status;
        public long createdAt;
        public long expiresAt;
    }
    
    public static class Position {
        public String id;
        public String user;
        public String pair;
        public String side;
        public String size;
        public String collateral;
        public String leverage;
        public String entryPrice;
        public String markPrice;
        public String pnl;
        public String roe;
        public String liquidationPrice;
        public String status;
    }
    
    public static class Portfolio {
        public String user;
        public String totalValue;
        public List<Position> positions;
        public List<TokenBalance> balances;
    }
    
    public static class TokenBalance {
        public Token token;
        public String balance;
        public String valueUsd;
    }
    
    public static class Pool {
        public String address;
        public String tokenA;
        public String tokenB;
        public String reserveA;
        public String reserveB;
        public String totalSupply;
        public String fee;
    }
    
    public static class Chain {
        public long id;
        public String name;
        public String symbol;
        public String rpcUrl;
        public String explorerUrl;
        public String nativeToken;
        public boolean isActive;
    }
}
