package com.tigerswap;

import java.io.IOException;
import java.net.URI;
import java.net.URLEncoder;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.nio.charset.StandardCharsets;
import java.time.Duration;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

/**
 * TigerSwap Java SDK.
 *
 * Talks to a TigerSwap node over its REST API. The client depends only on the
 * JDK: HTTP comes from {@link HttpClient} and JSON from {@link Json}. No
 * third-party DEX or exchange endpoint is referenced, and no response is ever
 * guessed at - a body that does not match the documented shape raises
 * {@link ApiException} rather than returning a partially filled object.
 */
public final class TigerSwapClient {

    /** API version prefix, matching the server's mounted routes. */
    private static final String API_PREFIX = "/api/v1";

    private final String baseURL;
    private final String apiKey;
    private final HttpClient httpClient;

    /**
     * @param baseURL server root, for example {@code https://node.tigerswap.example}
     * @param apiKey  bearer token, or an empty string for public endpoints
     */
    public TigerSwapClient(String baseURL, String apiKey) {
        if (baseURL == null || baseURL.isBlank()) {
            throw new IllegalArgumentException("baseURL is required");
        }
        String trimmed = baseURL.endsWith("/")
                ? baseURL.substring(0, baseURL.length() - 1)
                : baseURL;
        if (!trimmed.startsWith("http://") && !trimmed.startsWith("https://")) {
            throw new IllegalArgumentException("baseURL must be an http(s) URL");
        }
        this.baseURL = trimmed;
        this.apiKey = apiKey == null ? "" : apiKey;
        this.httpClient = HttpClient.newBuilder()
                .connectTimeout(Duration.ofSeconds(10))
                .followRedirects(HttpClient.Redirect.NEVER)
                .build();
    }

    // ------------------------------------------------------------------
    // endpoints
    // ------------------------------------------------------------------

    /** Every chain the node serves. */
    public List<Chain> getChains() {
        List<Object> items = Json.asArray(get("/chains", null), "response");
        List<Chain> chains = new ArrayList<>(items.size());
        for (Object item : items) {
            chains.add(Chain.fromJson(Json.asObject(item, "chain")));
        }
        return chains;
    }

    /** Every token the node knows on {@code chainId}. */
    public List<Token> getTokens(long chainId) {
        Map<String, String> query = new LinkedHashMap<>();
        query.put("chainId", Long.toString(chainId));
        List<Object> items = Json.asArray(get("/tokens", query), "response");
        List<Token> tokens = new ArrayList<>(items.size());
        for (Object item : items) {
            tokens.add(Token.fromJson(Json.asObject(item, "token")));
        }
        return tokens;
    }

    /** Pools known to the node, with their reserves and fee tier. */
    public List<Pool> getPools() {
        List<Object> items = Json.asArray(get("/swap/pairs", null), "response");
        List<Pool> pools = new ArrayList<>(items.size());
        for (Object item : items) {
            pools.add(Pool.fromJson(Json.asObject(item, "pool")));
        }
        return pools;
    }

    /** Aggregate market statistics. */
    public MarketStats getMarketStats() {
        return MarketStats.fromJson(Json.asObject(get("/market/stats", null), "response"));
    }

    /**
     * Quote a swap.
     *
     * @param slippageBps allowed slippage in basis points, 50 for 0.5%
     */
    public SwapQuote getQuote(String tokenIn, String tokenOut, String amountIn,
                              long chainId, int slippageBps) {
        Map<String, Object> payload = new LinkedHashMap<>();
        payload.put("token_in", requireText(tokenIn, "tokenIn"));
        payload.put("token_out", requireText(tokenOut, "tokenOut"));
        payload.put("amount_in", requireText(amountIn, "amountIn"));
        payload.put("chain_id", chainId);
        payload.put("slippage_bps", requireSlippage(slippageBps));
        return SwapQuote.fromJson(Json.asObject(post("/swap/quote", payload), "response"));
    }

    /**
     * Build an unsigned swap transaction.
     *
     * The server returns calldata and a target; signing happens in the caller's
     * wallet, so no private key is ever sent to the node.
     */
    public UnsignedTransaction buildSwap(String tokenIn, String tokenOut, String amountIn,
                                         long chainId, String recipient, int slippageBps) {
        Map<String, Object> payload = new LinkedHashMap<>();
        payload.put("token_in", requireText(tokenIn, "tokenIn"));
        payload.put("token_out", requireText(tokenOut, "tokenOut"));
        payload.put("amount_in", requireText(amountIn, "amountIn"));
        payload.put("chain_id", chainId);
        payload.put("recipient", requireText(recipient, "recipient"));
        payload.put("slippage_bps", requireSlippage(slippageBps));
        return UnsignedTransaction.fromJson(Json.asObject(post("/swap/build", payload), "response"));
    }

    // ------------------------------------------------------------------
    // transport
    // ------------------------------------------------------------------

    private Object get(String path, Map<String, String> query) {
        return send(HttpRequest.newBuilder()
                .uri(URI.create(baseURL + API_PREFIX + path + encodeQuery(query)))
                .timeout(Duration.ofSeconds(15))
                .header("Accept", "application/json")
                .GET()
                .build());
    }

    private Object post(String path, Map<String, Object> payload) {
        String body = Json.write(payload);
        HttpRequest request = HttpRequest.newBuilder()
                .uri(URI.create(baseURL + API_PREFIX + path))
                .timeout(Duration.ofSeconds(15))
                .header("Accept", "application/json")
                .header("Content-Type", "application/json")
                .POST(HttpRequest.BodyPublishers.ofString(body, StandardCharsets.UTF_8))
                .build();
        return send(request);
    }

    private Object send(HttpRequest request) {
        if (!apiKey.isEmpty()) {
            request = HttpRequest.newBuilder(request, (name, value) -> true)
                    .header("Authorization", "Bearer " + apiKey)
                    .build();
        }

        HttpResponse<String> response;
        try {
            response = httpClient.send(request, HttpResponse.BodyHandlers.ofString(StandardCharsets.UTF_8));
        } catch (IOException e) {
            throw new ApiException(0, "request to " + request.uri() + " failed: " + e.getMessage(), e);
        } catch (InterruptedException e) {
            Thread.currentThread().interrupt();
            throw new ApiException(0, "request to " + request.uri() + " was interrupted", e);
        }

        int status = response.statusCode();
        String body = response.body() == null ? "" : response.body();
        if (status < 200 || status >= 300) {
            throw new ApiException(status, "HTTP " + status + ": " + truncate(body));
        }
        if (body.isBlank()) {
            throw new ApiException(status, "empty response body");
        }
        try {
            return Json.parse(body);
        } catch (Json.JsonException e) {
            throw new ApiException(status, "malformed response body: " + e.getMessage(), e);
        }
    }

    private static String encodeQuery(Map<String, String> query) {
        if (query == null || query.isEmpty()) {
            return "";
        }
        StringBuilder out = new StringBuilder("?");
        boolean first = true;
        for (Map.Entry<String, String> entry : query.entrySet()) {
            if (!first) {
                out.append('&');
            }
            first = false;
            out.append(URLEncoder.encode(entry.getKey(), StandardCharsets.UTF_8))
                    .append('=')
                    .append(URLEncoder.encode(entry.getValue(), StandardCharsets.UTF_8));
        }
        return out.toString();
    }

    private static String requireText(String value, String name) {
        if (value == null || value.isBlank()) {
            throw new IllegalArgumentException(name + " is required");
        }
        return value;
    }

    private static int requireSlippage(int slippageBps) {
        if (slippageBps < 0 || slippageBps > 10_000) {
            throw new IllegalArgumentException("slippageBps must be between 0 and 10000");
        }
        return slippageBps;
    }

    private static String truncate(String body) {
        int limit = 512;
        return body.length() <= limit ? body : body.substring(0, limit) + "...";
    }

    // ------------------------------------------------------------------
    // types
    // ------------------------------------------------------------------

    /** Raised when the node returns an error or an unusable body. */
    public static final class ApiException extends RuntimeException {
        private static final long serialVersionUID = 1L;

        private final int statusCode;

        ApiException(int statusCode, String message) {
            super(message);
            this.statusCode = statusCode;
        }

        ApiException(int statusCode, String message, Throwable cause) {
            super(message, cause);
            this.statusCode = statusCode;
        }

        /** HTTP status, or 0 when the request never completed. */
        public int getStatusCode() {
            return statusCode;
        }
    }

    /** A chain served by the node. */
    public static final class Chain {
        public final long id;
        public final String name;
        public final String chainType;
        public final String nativeSymbol;
        public final String explorerURL;
        public final boolean isActive;

        private Chain(long id, String name, String chainType, String nativeSymbol,
                      String explorerURL, boolean isActive) {
            this.id = id;
            this.name = name;
            this.chainType = chainType;
            this.nativeSymbol = nativeSymbol;
            this.explorerURL = explorerURL;
            this.isActive = isActive;
        }

        static Chain fromJson(Map<String, Object> object) {
            Object active = Json.field(object, "isActive");
            if (!(active instanceof Boolean)) {
                throw new Json.JsonException("missing or non-boolean field 'isActive'");
            }
            return new Chain(
                    Json.requiredLong(object, "id"),
                    Json.requiredString(object, "name"),
                    Json.requiredString(object, "chainType"),
                    Json.requiredString(object, "nativeSymbol"),
                    Json.optionalString(object, "explorerURL"),
                    (Boolean) active);
        }
    }

    /** A token listed on a chain. */
    public static final class Token {
        public final String address;
        public final String symbol;
        public final String name;
        public final int decimals;
        public final long chainId;
        public final String logoURI;
        public final Double priceUSD;

        private Token(String address, String symbol, String name, int decimals,
                      long chainId, String logoURI, Double priceUSD) {
            this.address = address;
            this.symbol = symbol;
            this.name = name;
            this.decimals = decimals;
            this.chainId = chainId;
            this.logoURI = logoURI;
            this.priceUSD = priceUSD;
        }

        static Token fromJson(Map<String, Object> object) {
            Object price = Json.field(object, "priceUSD");
            Double priceUSD = null;
            if (price instanceof Number) {
                priceUSD = ((Number) price).doubleValue();
            } else if (price != null) {
                throw new Json.JsonException("field 'priceUSD' is neither a number nor null");
            }
            return new Token(
                    Json.requiredString(object, "address"),
                    Json.requiredString(object, "symbol"),
                    Json.requiredString(object, "name"),
                    Json.requiredInt(object, "decimals"),
                    Json.requiredLong(object, "chainId"),
                    Json.optionalString(object, "logoURI"),
                    priceUSD);
        }
    }

    /** A liquidity pool. */
    public static final class Pool {
        public final String pairAddress;
        public final String token0;
        public final String token1;
        public final String symbol0;
        public final String symbol1;
        public final double reserve0;
        public final double reserve1;
        public final int feeBps;

        private Pool(String pairAddress, String token0, String token1, String symbol0,
                     String symbol1, double reserve0, double reserve1, int feeBps) {
            this.pairAddress = pairAddress;
            this.token0 = token0;
            this.token1 = token1;
            this.symbol0 = symbol0;
            this.symbol1 = symbol1;
            this.reserve0 = reserve0;
            this.reserve1 = reserve1;
            this.feeBps = feeBps;
        }

        static Pool fromJson(Map<String, Object> object) {
            return new Pool(
                    Json.requiredString(object, "pairAddress"),
                    Json.requiredString(object, "token0"),
                    Json.requiredString(object, "token1"),
                    Json.requiredString(object, "symbol0"),
                    Json.requiredString(object, "symbol1"),
                    Json.requiredDouble(object, "reserve0"),
                    Json.requiredDouble(object, "reserve1"),
                    Json.requiredInt(object, "feeBps"));
        }
    }

    /** A swap quote. Amounts are decimal strings to preserve precision. */
    public static final class SwapQuote {
        public final String amountIn;
        public final String amountOut;
        public final String amountOutMin;
        public final double priceImpact;
        public final List<String> route;
        public final long gasEstimate;

        private SwapQuote(String amountIn, String amountOut, String amountOutMin,
                          double priceImpact, List<String> route, long gasEstimate) {
            this.amountIn = amountIn;
            this.amountOut = amountOut;
            this.amountOutMin = amountOutMin;
            this.priceImpact = priceImpact;
            this.route = route;
            this.gasEstimate = gasEstimate;
        }

        static SwapQuote fromJson(Map<String, Object> object) {
            List<Object> rawRoute = Json.asArray(Json.field(object, "route"), "route");
            List<String> route = new ArrayList<>(rawRoute.size());
            for (Object hop : rawRoute) {
                if (!(hop instanceof String)) {
                    throw new Json.JsonException("route entries must be strings");
                }
                route.add((String) hop);
            }
            return new SwapQuote(
                    Json.requiredString(object, "amountIn"),
                    Json.requiredString(object, "amountOut"),
                    Json.requiredString(object, "amountOutMin"),
                    Json.requiredDouble(object, "priceImpact"),
                    route,
                    Json.requiredLong(object, "gasEstimate"));
        }
    }

    /** An unsigned transaction, ready for the caller's wallet to sign. */
    public static final class UnsignedTransaction {
        public final String to;
        public final String data;
        public final String value;
        public final String gasLimit;

        private UnsignedTransaction(String to, String data, String value, String gasLimit) {
            this.to = to;
            this.data = data;
            this.value = value;
            this.gasLimit = gasLimit;
        }

        static UnsignedTransaction fromJson(Map<String, Object> object) {
            return new UnsignedTransaction(
                    Json.requiredString(object, "to"),
                    Json.requiredString(object, "data"),
                    Json.requiredString(object, "value"),
                    Json.optionalString(object, "gas_limit"));
        }
    }

    /** Aggregate market statistics. */
    public static final class MarketStats {
        public final double totalTVL;
        public final double volume24h;
        public final long totalPairs;
        public final long totalUsers;
        public final String updatedAt;

        private MarketStats(double totalTVL, double volume24h, long totalPairs,
                            long totalUsers, String updatedAt) {
            this.totalTVL = totalTVL;
            this.volume24h = volume24h;
            this.totalPairs = totalPairs;
            this.totalUsers = totalUsers;
            this.updatedAt = updatedAt;
        }

        static MarketStats fromJson(Map<String, Object> object) {
            return new MarketStats(
                    Json.requiredDouble(object, "totalTVL"),
                    Json.requiredDouble(object, "volume24h"),
                    Json.requiredLong(object, "totalPairs"),
                    Json.requiredLong(object, "totalUsers"),
                    Json.optionalString(object, "updatedAt"));
        }
    }
}
