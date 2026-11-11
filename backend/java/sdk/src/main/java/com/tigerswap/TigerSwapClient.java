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
import java.util.Collections;
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
     * Active liquidity-mining farms, each with its reward streams and live APR.
     *
     * {@code apr} is null when the node has no oracle price for the farm's
     * tokens; it is never reported as zero.
     */
    public List<Farm> getFarms() {
        Map<String, Object> root = Json.asObject(get("/farms", null), "response");
        List<Object> items = Json.asArray(Json.field(root, "farms"), "farms");
        List<Farm> farms = new ArrayList<>(items.size());
        for (Object item : items) {
            farms.add(Farm.fromJson(Json.asObject(item, "farm")));
        }
        return farms;
    }

    /** Airdrop campaigns currently open for claims. */
    public List<AirdropCampaign> getAirdrops() {
        Map<String, Object> root = Json.asObject(get("/airdrops", null), "response");
        List<Object> items = Json.asArray(Json.field(root, "campaigns"), "campaigns");
        List<AirdropCampaign> campaigns = new ArrayList<>(items.size());
        for (Object item : items) {
            campaigns.add(AirdropCampaign.fromJson(Json.asObject(item, "campaign")));
        }
        return campaigns;
    }

    /**
     * Active NFT collections, each with its minted supply.
     *
     * A collection starts paused; only an active one can mint. {@code totalSupply}
     * is the number of tokens minted so far, counted by the node.
     */
    public List<NFTCollection> getNFTCollections() {
        Map<String, Object> root = Json.asObject(get("/nft/collections", null), "response");
        List<Object> items = Json.asArray(Json.field(root, "collections"), "collections");
        List<NFTCollection> collections = new ArrayList<>(items.size());
        for (Object item : items) {
            collections.add(NFTCollection.fromJson(Json.asObject(item, "collection")));
        }
        return collections;
    }

    /**
     * Live NFT listings, each joined with its token and collection.
     *
     * A listing escrows the token, so {@code token.ownerUserId} is the escrow
     * sentinel until the listing settles or is cancelled.
     */
    public List<NFTListingView> getNFTListings() {
        Map<String, Object> root = Json.asObject(get("/nft/listings", null), "response");
        List<Object> items = Json.asArray(Json.field(root, "listings"), "listings");
        List<NFTListingView> listings = new ArrayList<>(items.size());
        for (Object item : items) {
            listings.add(NFTListingView.fromJson(Json.asObject(item, "listing")));
        }
        return listings;
    }

    /**
     * Live offers on one token, each joined with its token and collection.
     *
     * @param collectionId the collection UUID
     * @param tokenId      the token id within the collection
     */
    public List<NFTOfferView> getNFTOffers(String collectionId, long tokenId) {
        Map<String, String> query = new LinkedHashMap<>();
        query.put("collection_id", requireText(collectionId, "collectionId"));
        query.put("token_id", Long.toString(tokenId));
        Map<String, Object> root = Json.asObject(get("/nft/offers", query), "response");
        List<Object> items = Json.asArray(Json.field(root, "offers"), "offers");
        List<NFTOfferView> offers = new ArrayList<>(items.size());
        for (Object item : items) {
            offers.add(NFTOfferView.fromJson(Json.asObject(item, "offer")));
        }
        return offers;
    }

    /** The marketplace fee configuration. */
    public NFTMarketplaceConfig getNFTConfig() {
        return NFTMarketplaceConfig.fromJson(Json.asObject(get("/nft/config", null), "response"));
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
    // analytics
    // ------------------------------------------------------------------

    /**
     * The protocol dashboard snapshot.
     *
     * Every figure is aggregated by the node from rows TigerSwap itself wrote.
     * A value the node cannot compute (for example open interest with no price
     * source) comes back as zero, never as an invented number.
     */
    public ProtocolOverview getAnalyticsOverview() {
        return ProtocolOverview.fromJson(Json.asObject(get("/analytics/overview", null), "response"));
    }

    /**
     * A volume series. {@code interval} and {@code window} are in seconds;
     * {@code interval} must be a whole multiple of the node's rollup bucket. A
     * zero interval or window leaves the node's default in place.
     */
    public VolumeSeries getAnalyticsVolume(String pair, long chainId, int interval, int window) {
        Map<String, String> query = new LinkedHashMap<>();
        if (pair != null && !pair.isBlank()) {
            query.put("pair", pair);
        }
        if (chainId != 0) {
            query.put("chain_id", Long.toString(chainId));
        }
        if (interval > 0) {
            query.put("interval", Integer.toString(interval));
        }
        if (window > 0) {
            query.put("window", Integer.toString(window));
        }
        return VolumeSeries.fromJson(Json.asObject(get("/analytics/volume", query), "response"));
    }

    /** The per-market volume leaderboard over a window in seconds. */
    public List<PairAnalytics> getAnalyticsPairs(long chainId, int window, int limit) {
        Map<String, String> query = new LinkedHashMap<>();
        if (chainId != 0) {
            query.put("chain_id", Long.toString(chainId));
        }
        if (window > 0) {
            query.put("window", Integer.toString(window));
        }
        if (limit > 0) {
            query.put("limit", Integer.toString(limit));
        }
        Map<String, Object> root = Json.asObject(get("/analytics/pairs", query), "response");
        List<Object> items = Json.asArray(Json.field(root, "pairs"), "pairs");
        List<PairAnalytics> pairs = new ArrayList<>(items.size());
        for (Object item : items) {
            pairs.add(PairAnalytics.fromJson(Json.asObject(item, "pair")));
        }
        return pairs;
    }

    /** The realized-PnL leaderboard. A zero {@code window} means all time. */
    public List<TopTrader> getAnalyticsTraders(int window, int limit) {
        Map<String, String> query = new LinkedHashMap<>();
        if (window > 0) {
            query.put("window", Integer.toString(window));
        }
        if (limit > 0) {
            query.put("limit", Integer.toString(limit));
        }
        Map<String, Object> root = Json.asObject(get("/analytics/traders", query), "response");
        List<Object> items = Json.asArray(Json.field(root, "traders"), "traders");
        List<TopTrader> traders = new ArrayList<>(items.size());
        for (Object item : items) {
            traders.add(TopTrader.fromJson(Json.asObject(item, "trader")));
        }
        return traders;
    }

    // ------------------------------------------------------------------
    // portfolio
    // ------------------------------------------------------------------

    /**
     * The authenticated account's portfolio: holdings valued at the node's
     * oracle price, plus realized and unrealized PnL from the derivatives book.
     * The bearer token set on the client identifies the account, so the node
     * scopes the response to that account and never to a caller-supplied id.
     */
    public Portfolio getPortfolio() {
        return Portfolio.fromJson(Json.asObject(get("/user/portfolio", null), "response"));
    }

    /** The authenticated account's transaction history, newest first. */
    public List<UserTransaction> getTransactions() {
        List<Object> items = Json.asArray(get("/user/transactions", null), "transactions");
        List<UserTransaction> txs = new ArrayList<>(items.size());
        for (Object item : items) {
            txs.add(UserTransaction.fromJson(Json.asObject(item, "transaction")));
        }
        return txs;
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

    /** A liquidity-mining farm with its reward streams and live APR. */
    public static final class Farm {
        public final String id;
        public final String key;
        public final String name;
        public final String farmType;
        public final long chainId;
        public final String stakingTokenAddress;
        public final String stakingTokenSymbol;
        public final int stakingTokenDecimals;
        public final String totalStaked;
        public final String status;
        public final List<FarmReward> rewards;
        public final Double apr;

        private Farm(String id, String key, String name, String farmType, long chainId,
                     String stakingTokenAddress, String stakingTokenSymbol, int stakingTokenDecimals,
                     String totalStaked, String status, List<FarmReward> rewards, Double apr) {
            this.id = id;
            this.key = key;
            this.name = name;
            this.farmType = farmType;
            this.chainId = chainId;
            this.stakingTokenAddress = stakingTokenAddress;
            this.stakingTokenSymbol = stakingTokenSymbol;
            this.stakingTokenDecimals = stakingTokenDecimals;
            this.totalStaked = totalStaked;
            this.status = status;
            this.rewards = rewards;
            this.apr = apr;
        }

        static Farm fromJson(Map<String, Object> object) {
            Map<String, Object> inner = Json.asObject(Json.field(object, "farm"), "farm");
            List<Object> rewardItems = Json.asArray(Json.field(object, "rewards"), "rewards");
            List<FarmReward> rewards = new ArrayList<>(rewardItems.size());
            for (Object item : rewardItems) {
                rewards.add(FarmReward.fromJson(Json.asObject(item, "reward")));
            }
            Object apr = Json.field(object, "apr");
            Double aprValue = null;
            if (apr instanceof Number) {
                aprValue = ((Number) apr).doubleValue();
            } else if (apr != null) {
                throw new Json.JsonException("field 'apr' is neither a number nor null");
            }
            return new Farm(
                    Json.requiredString(inner, "id"),
                    Json.requiredString(inner, "key"),
                    Json.requiredString(inner, "name"),
                    Json.requiredString(inner, "farm_type"),
                    Json.requiredLong(inner, "chain_id"),
                    Json.requiredString(inner, "staking_token_address"),
                    Json.requiredString(inner, "staking_token_symbol"),
                    Json.requiredInt(inner, "staking_token_decimals"),
                    Json.requiredString(inner, "total_staked"),
                    Json.requiredString(inner, "status"),
                    rewards,
                    aprValue);
        }
    }

    /** One reward token stream of a farm. */
    public static final class FarmReward {
        public final String id;
        public final String farmId;
        public final String tokenAddress;
        public final String tokenSymbol;
        public final int tokenDecimals;
        public final String ratePerSecond;

        private FarmReward(String id, String farmId, String tokenAddress, String tokenSymbol,
                           int tokenDecimals, String ratePerSecond) {
            this.id = id;
            this.farmId = farmId;
            this.tokenAddress = tokenAddress;
            this.tokenSymbol = tokenSymbol;
            this.tokenDecimals = tokenDecimals;
            this.ratePerSecond = ratePerSecond;
        }

        static FarmReward fromJson(Map<String, Object> object) {
            return new FarmReward(
                    Json.requiredString(object, "id"),
                    Json.requiredString(object, "farm_id"),
                    Json.requiredString(object, "token_address"),
                    Json.requiredString(object, "token_symbol"),
                    Json.requiredInt(object, "token_decimals"),
                    Json.requiredString(object, "rate_per_second"));
        }
    }

    /** An admin-managed airdrop campaign. */
    public static final class AirdropCampaign {
        public final String id;
        public final String key;
        public final String name;
        public final long chainId;
        public final String tokenAddress;
        public final String tokenSymbol;
        public final int tokenDecimals;
        public final String totalAllocated;
        public final String funded;
        public final String totalClaimed;
        public final String status;

        private AirdropCampaign(String id, String key, String name, long chainId,
                                String tokenAddress, String tokenSymbol, int tokenDecimals,
                                String totalAllocated, String funded, String totalClaimed,
                                String status) {
            this.id = id;
            this.key = key;
            this.name = name;
            this.chainId = chainId;
            this.tokenAddress = tokenAddress;
            this.tokenSymbol = tokenSymbol;
            this.tokenDecimals = tokenDecimals;
            this.totalAllocated = totalAllocated;
            this.funded = funded;
            this.totalClaimed = totalClaimed;
            this.status = status;
        }

        static AirdropCampaign fromJson(Map<String, Object> object) {
            return new AirdropCampaign(
                    Json.requiredString(object, "id"),
                    Json.requiredString(object, "key"),
                    Json.requiredString(object, "name"),
                    Json.requiredLong(object, "chain_id"),
                    Json.requiredString(object, "token_address"),
                    Json.requiredString(object, "token_symbol"),
                    Json.requiredInt(object, "token_decimals"),
                    Json.requiredString(object, "total_allocated"),
                    Json.requiredString(object, "funded"),
                    Json.requiredString(object, "total_claimed"),
                    Json.requiredString(object, "status"));
        }
    }

    /** An admin-managed ERC-721 collection. */
    public static final class NFTCollection {
        public final String id;
        public final String key;
        public final String name;
        public final String symbol;
        public final long chainId;
        public final String contractAddress;
        public final String baseUri;
        public final long totalSupply;
        public final String status;

        private NFTCollection(String id, String key, String name, String symbol, long chainId,
                              String contractAddress, String baseUri, long totalSupply, String status) {
            this.id = id;
            this.key = key;
            this.name = name;
            this.symbol = symbol;
            this.chainId = chainId;
            this.contractAddress = contractAddress;
            this.baseUri = baseUri;
            this.totalSupply = totalSupply;
            this.status = status;
        }

        static NFTCollection fromJson(Map<String, Object> object) {
            return new NFTCollection(
                    Json.requiredString(object, "id"),
                    Json.requiredString(object, "key"),
                    Json.requiredString(object, "name"),
                    Json.requiredString(object, "symbol"),
                    Json.requiredLong(object, "chain_id"),
                    Json.requiredString(object, "contract_address"),
                    Json.requiredString(object, "base_uri"),
                    Json.requiredLong(object, "total_supply"),
                    Json.requiredString(object, "status"));
        }
    }

    /** A minted token belonging to one collection. */
    public static final class NFTToken {
        public final String id;
        public final String collectionId;
        public final long tokenId;
        public final String ownerUserId;
        public final String uri;
        public final String escrowListingId;

        private NFTToken(String id, String collectionId, long tokenId, String ownerUserId,
                         String uri, String escrowListingId) {
            this.id = id;
            this.collectionId = collectionId;
            this.tokenId = tokenId;
            this.ownerUserId = ownerUserId;
            this.uri = uri;
            this.escrowListingId = escrowListingId;
        }

        static NFTToken fromJson(Map<String, Object> object) {
            Object escrow = Json.field(object, "escrow_listing_id");
            String escrowListingId = escrow == null ? null : Json.asString(escrow, "escrow_listing_id");
            return new NFTToken(
                    Json.requiredString(object, "id"),
                    Json.requiredString(object, "collection_id"),
                    Json.requiredLong(object, "token_id"),
                    Json.requiredString(object, "owner_user_id"),
                    Json.requiredString(object, "uri"),
                    escrowListingId);
        }
    }

    /** A fixed-price listing. The price and fee are exact integer strings. */
    public static final class NFTListing {
        public final String id;
        public final String collectionId;
        public final long tokenId;
        public final String sellerUserId;
        public final String paymentTokenAddress;
        public final String paymentTokenSymbol;
        public final int paymentTokenDecimals;
        public final String price;
        public final int feeBps;
        public final String deadline;
        public final String status;

        private NFTListing(String id, String collectionId, long tokenId, String sellerUserId,
                           String paymentTokenAddress, String paymentTokenSymbol, int paymentTokenDecimals,
                           String price, int feeBps, String deadline, String status) {
            this.id = id;
            this.collectionId = collectionId;
            this.tokenId = tokenId;
            this.sellerUserId = sellerUserId;
            this.paymentTokenAddress = paymentTokenAddress;
            this.paymentTokenSymbol = paymentTokenSymbol;
            this.paymentTokenDecimals = paymentTokenDecimals;
            this.price = price;
            this.feeBps = feeBps;
            this.deadline = deadline;
            this.status = status;
        }

        static NFTListing fromJson(Map<String, Object> object) {
            return new NFTListing(
                    Json.requiredString(object, "id"),
                    Json.requiredString(object, "collection_id"),
                    Json.requiredLong(object, "token_id"),
                    Json.requiredString(object, "seller_user_id"),
                    Json.requiredString(object, "payment_token_address"),
                    Json.requiredString(object, "payment_token_symbol"),
                    Json.requiredInt(object, "payment_token_decimals"),
                    Json.requiredString(object, "price"),
                    Json.requiredInt(object, "fee_bps"),
                    Json.requiredString(object, "deadline"),
                    Json.requiredString(object, "status"));
        }
    }

    /** An escrowed bid on a token. */
    public static final class NFTOffer {
        public final String id;
        public final String collectionId;
        public final long tokenId;
        public final String buyerUserId;
        public final String paymentTokenAddress;
        public final String paymentTokenSymbol;
        public final int paymentTokenDecimals;
        public final String amount;
        public final int feeBps;
        public final String deadline;
        public final String status;

        private NFTOffer(String id, String collectionId, long tokenId, String buyerUserId,
                         String paymentTokenAddress, String paymentTokenSymbol, int paymentTokenDecimals,
                         String amount, int feeBps, String deadline, String status) {
            this.id = id;
            this.collectionId = collectionId;
            this.tokenId = tokenId;
            this.buyerUserId = buyerUserId;
            this.paymentTokenAddress = paymentTokenAddress;
            this.paymentTokenSymbol = paymentTokenSymbol;
            this.paymentTokenDecimals = paymentTokenDecimals;
            this.amount = amount;
            this.feeBps = feeBps;
            this.deadline = deadline;
            this.status = status;
        }

        static NFTOffer fromJson(Map<String, Object> object) {
            return new NFTOffer(
                    Json.requiredString(object, "id"),
                    Json.requiredString(object, "collection_id"),
                    Json.requiredLong(object, "token_id"),
                    Json.requiredString(object, "buyer_user_id"),
                    Json.requiredString(object, "payment_token_address"),
                    Json.requiredString(object, "payment_token_symbol"),
                    Json.requiredInt(object, "payment_token_decimals"),
                    Json.requiredString(object, "amount"),
                    Json.requiredInt(object, "fee_bps"),
                    Json.requiredString(object, "deadline"),
                    Json.requiredString(object, "status"));
        }
    }

    /** A live listing joined with its token and collection. */
    public static final class NFTListingView {
        public final NFTListing listing;
        public final NFTToken token;
        public final NFTCollection collection;
        public final boolean active;

        private NFTListingView(NFTListing listing, NFTToken token, NFTCollection collection, boolean active) {
            this.listing = listing;
            this.token = token;
            this.collection = collection;
            this.active = active;
        }

        static NFTListingView fromJson(Map<String, Object> object) {
            return new NFTListingView(
                    NFTListing.fromJson(Json.asObject(Json.field(object, "listing"), "listing")),
                    NFTToken.fromJson(Json.asObject(Json.field(object, "token"), "token")),
                    NFTCollection.fromJson(Json.asObject(Json.field(object, "collection"), "collection")),
                    Json.requiredBoolean(object, "active"));
        }
    }

    /** A live offer joined with its token and collection. */
    public static final class NFTOfferView {
        public final NFTOffer offer;
        public final NFTToken token;
        public final NFTCollection collection;
        public final boolean active;

        private NFTOfferView(NFTOffer offer, NFTToken token, NFTCollection collection, boolean active) {
            this.offer = offer;
            this.token = token;
            this.collection = collection;
            this.active = active;
        }

        static NFTOfferView fromJson(Map<String, Object> object) {
            return new NFTOfferView(
                    NFTOffer.fromJson(Json.asObject(Json.field(object, "offer"), "offer")),
                    NFTToken.fromJson(Json.asObject(Json.field(object, "token"), "token")),
                    NFTCollection.fromJson(Json.asObject(Json.field(object, "collection"), "collection")),
                    Json.requiredBoolean(object, "active"));
        }
    }

    /** The marketplace fee configuration. */
    public static final class NFTMarketplaceConfig {
        public final int feeBps;
        public final int maxFeeBps;
        public final String feeAccount;
        public final String escrowAccount;

        private NFTMarketplaceConfig(int feeBps, int maxFeeBps, String feeAccount, String escrowAccount) {
            this.feeBps = feeBps;
            this.maxFeeBps = maxFeeBps;
            this.feeAccount = feeAccount;
            this.escrowAccount = escrowAccount;
        }

        static NFTMarketplaceConfig fromJson(Map<String, Object> object) {
            return new NFTMarketplaceConfig(
                    Json.requiredInt(object, "fee_bps"),
                    Json.requiredInt(object, "max_fee_bps"),
                    Json.requiredString(object, "fee_account"),
                    Json.requiredString(object, "escrow_account"));
        }
    }

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

    /** One executable hop: a token pair and the pool family to trade it on. */
    public static final class SwapRouteHop {
        public final String tokenIn;
        public final String tokenOut;
        public final String family;
        public final String pool;

        private SwapRouteHop(String tokenIn, String tokenOut, String family, String pool) {
            this.tokenIn = tokenIn;
            this.tokenOut = tokenOut;
            this.family = family;
            this.pool = pool;
        }

        static SwapRouteHop fromJson(Map<String, Object> object) {
            return new SwapRouteHop(
                    Json.requiredString(object, "tokenIn"),
                    Json.requiredString(object, "tokenOut"),
                    Json.requiredString(object, "family"),
                    Json.requiredString(object, "pool"));
        }

        static List<SwapRouteHop> listFrom(Object value, String field) {
            if (value == null) {
                return Collections.emptyList();
            }
            List<Object> raw = Json.asArray(value, field);
            List<SwapRouteHop> hops = new ArrayList<>(raw.size());
            for (Object entry : raw) {
                hops.add(fromJson(Json.asObject(entry, field + " entry")));
            }
            return hops;
        }
    }

    /** One leg of a split order: a route and the input it carries. */
    public static final class SwapQuoteLeg {
        public final List<String> path;
        public final String family;
        public final String pool;
        public final String amountIn;
        public final String amountOut;
        public final List<SwapRouteHop> hops;

        private SwapQuoteLeg(List<String> path, String family, String pool,
                             String amountIn, String amountOut, List<SwapRouteHop> hops) {
            this.path = path;
            this.family = family;
            this.pool = pool;
            this.amountIn = amountIn;
            this.amountOut = amountOut;
            this.hops = hops;
        }

        static SwapQuoteLeg fromJson(Map<String, Object> object) {
            List<Object> rawPath = Json.asArray(Json.field(object, "path"), "path");
            List<String> path = new ArrayList<>(rawPath.size());
            for (Object entry : rawPath) {
                path.add(Json.asString(entry, "path entry"));
            }
            return new SwapQuoteLeg(
                    path,
                    Json.requiredString(object, "family"),
                    Json.requiredString(object, "pool"),
                    Json.requiredString(object, "amountIn"),
                    Json.requiredString(object, "amountOut"),
                    SwapRouteHop.listFrom(Json.field(object, "hops"), "hops"));
        }

        static List<SwapQuoteLeg> listFrom(Object value, String field) {
            if (value == null) {
                return Collections.emptyList();
            }
            List<Object> raw = Json.asArray(value, field);
            List<SwapQuoteLeg> legs = new ArrayList<>(raw.size());
            for (Object entry : raw) {
                legs.add(fromJson(Json.asObject(entry, field + " entry")));
            }
            return legs;
        }
    }

    /**
     * A swap quote. Amounts are decimal strings to preserve precision. RouteHops
     * is the executable plan for a single multi-hop route and legs is the plan
     * for a split order; a direct single-family quote carries neither.
     */
    public static final class SwapQuote {
        public final String amountIn;
        public final String amountOut;
        public final String amountOutMin;
        public final double priceImpact;
        public final List<String> route;
        public final long gasEstimate;
        public final List<SwapRouteHop> routeHops;
        public final List<SwapQuoteLeg> legs;

        private SwapQuote(String amountIn, String amountOut, String amountOutMin,
                          double priceImpact, List<String> route, long gasEstimate,
                          List<SwapRouteHop> routeHops, List<SwapQuoteLeg> legs) {
            this.amountIn = amountIn;
            this.amountOut = amountOut;
            this.amountOutMin = amountOutMin;
            this.priceImpact = priceImpact;
            this.route = route;
            this.gasEstimate = gasEstimate;
            this.routeHops = routeHops;
            this.legs = legs;
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
                    Json.requiredLong(object, "gasEstimate"),
                    SwapRouteHop.listFrom(Json.field(object, "routeHops"), "routeHops"),
                    SwapQuoteLeg.listFrom(Json.field(object, "legs"), "legs"));
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

    /** The protocol dashboard snapshot from {@code GET /analytics/overview}. */
    public static final class ProtocolOverview {
        public final double totalTvlUsd;
        public final double volume24hUsd;
        public final double fees24hUsd;
        public final double volume7dUsd;
        public final double fees7dUsd;
        public final long totalPools;
        public final long activePools;
        public final long totalFarms;
        public final long activeFarms;
        public final long totalTokens;
        public final long totalChains;
        public final long activeChains;
        public final long totalUsers;
        public final long totalTrades24h;
        public final double openInterestUsd;
        public final double marginBorrowedUsd;
        public final long nftListingsActive;
        public final String updatedAt;

        private ProtocolOverview(double totalTvlUsd, double volume24hUsd, double fees24hUsd,
                                 double volume7dUsd, double fees7dUsd, long totalPools,
                                 long activePools, long totalFarms, long activeFarms,
                                 long totalTokens, long totalChains, long activeChains,
                                 long totalUsers, long totalTrades24h, double openInterestUsd,
                                 double marginBorrowedUsd, long nftListingsActive, String updatedAt) {
            this.totalTvlUsd = totalTvlUsd;
            this.volume24hUsd = volume24hUsd;
            this.fees24hUsd = fees24hUsd;
            this.volume7dUsd = volume7dUsd;
            this.fees7dUsd = fees7dUsd;
            this.totalPools = totalPools;
            this.activePools = activePools;
            this.totalFarms = totalFarms;
            this.activeFarms = activeFarms;
            this.totalTokens = totalTokens;
            this.totalChains = totalChains;
            this.activeChains = activeChains;
            this.totalUsers = totalUsers;
            this.totalTrades24h = totalTrades24h;
            this.openInterestUsd = openInterestUsd;
            this.marginBorrowedUsd = marginBorrowedUsd;
            this.nftListingsActive = nftListingsActive;
            this.updatedAt = updatedAt;
        }

        static ProtocolOverview fromJson(Map<String, Object> object) {
            return new ProtocolOverview(
                    Json.requiredDouble(object, "total_tvl_usd"),
                    Json.requiredDouble(object, "volume_24h_usd"),
                    Json.requiredDouble(object, "fees_24h_usd"),
                    Json.requiredDouble(object, "volume_7d_usd"),
                    Json.requiredDouble(object, "fees_7d_usd"),
                    Json.requiredLong(object, "total_pools"),
                    Json.requiredLong(object, "active_pools"),
                    Json.requiredLong(object, "total_farms"),
                    Json.requiredLong(object, "active_farms"),
                    Json.requiredLong(object, "total_tokens"),
                    Json.requiredLong(object, "total_chains"),
                    Json.requiredLong(object, "active_chains"),
                    Json.requiredLong(object, "total_users"),
                    Json.requiredLong(object, "total_trades_24h"),
                    Json.requiredDouble(object, "open_interest_usd"),
                    Json.requiredDouble(object, "margin_borrowed_usd"),
                    Json.requiredLong(object, "nft_listings_active"),
                    Json.optionalString(object, "updated_at"));
        }
    }

    /** One point of a volume series. */
    public static final class VolumePoint {
        public final String timestamp;
        public final double volumeUsd;
        public final double feesUsd;
        public final long tradeCount;
        public final double takerBuyVolumeUsd;
        public final double takerSellVolumeUsd;

        private VolumePoint(String timestamp, double volumeUsd, double feesUsd, long tradeCount,
                            double takerBuyVolumeUsd, double takerSellVolumeUsd) {
            this.timestamp = timestamp;
            this.volumeUsd = volumeUsd;
            this.feesUsd = feesUsd;
            this.tradeCount = tradeCount;
            this.takerBuyVolumeUsd = takerBuyVolumeUsd;
            this.takerSellVolumeUsd = takerSellVolumeUsd;
        }

        static VolumePoint fromJson(Map<String, Object> object) {
            return new VolumePoint(
                    Json.requiredString(object, "timestamp"),
                    Json.requiredDouble(object, "volume_usd"),
                    Json.requiredDouble(object, "fees_usd"),
                    Json.requiredLong(object, "trade_count"),
                    Json.requiredDouble(object, "taker_buy_volume_usd"),
                    Json.requiredDouble(object, "taker_sell_volume_usd"));
        }
    }

    /** The envelope returned by {@code GET /analytics/volume}. */
    public static final class VolumeSeries {
        public final String pair;
        public final long chainId;
        public final int interval;
        public final List<VolumePoint> series;

        private VolumeSeries(String pair, long chainId, int interval, List<VolumePoint> series) {
            this.pair = pair;
            this.chainId = chainId;
            this.interval = interval;
            this.series = series;
        }

        static VolumeSeries fromJson(Map<String, Object> object) {
            List<Object> items = Json.asArray(Json.field(object, "series"), "series");
            List<VolumePoint> points = new ArrayList<>(items.size());
            for (Object item : items) {
                points.add(VolumePoint.fromJson(Json.asObject(item, "point")));
            }
            return new VolumeSeries(
                    Json.optionalString(object, "pair"),
                    Json.requiredLong(object, "chain_id"),
                    Json.requiredInt(object, "interval"),
                    points);
        }
    }

    /** Per-market activity over a window. */
    public static final class PairAnalytics {
        public final String pair;
        public final long chainId;
        public final double volumeUsd;
        public final double feesUsd;
        public final long tradeCount;
        public final double takerBuyVolumeUsd;
        public final double takerSellVolumeUsd;

        private PairAnalytics(String pair, long chainId, double volumeUsd, double feesUsd,
                              long tradeCount, double takerBuyVolumeUsd, double takerSellVolumeUsd) {
            this.pair = pair;
            this.chainId = chainId;
            this.volumeUsd = volumeUsd;
            this.feesUsd = feesUsd;
            this.tradeCount = tradeCount;
            this.takerBuyVolumeUsd = takerBuyVolumeUsd;
            this.takerSellVolumeUsd = takerSellVolumeUsd;
        }

        static PairAnalytics fromJson(Map<String, Object> object) {
            return new PairAnalytics(
                    Json.requiredString(object, "pair"),
                    Json.requiredLong(object, "chain_id"),
                    Json.requiredDouble(object, "volume_usd"),
                    Json.requiredDouble(object, "fees_usd"),
                    Json.requiredLong(object, "trade_count"),
                    Json.requiredDouble(object, "taker_buy_volume_usd"),
                    Json.requiredDouble(object, "taker_sell_volume_usd"));
        }
    }

    /** One row of the realized-PnL leaderboard. */
    public static final class TopTrader {
        public final String userId;
        public final double realizedPnl;
        public final double volumeUsd;
        public final long closedTrades;
        public final double winRate;
        public final long liquidations;

        private TopTrader(String userId, double realizedPnl, double volumeUsd, long closedTrades,
                          double winRate, long liquidations) {
            this.userId = userId;
            this.realizedPnl = realizedPnl;
            this.volumeUsd = volumeUsd;
            this.closedTrades = closedTrades;
            this.winRate = winRate;
            this.liquidations = liquidations;
        }

        static TopTrader fromJson(Map<String, Object> object) {
            return new TopTrader(
                    Json.requiredString(object, "user_id"),
                    Json.requiredDouble(object, "realized_pnl"),
                    Json.requiredDouble(object, "volume_usd"),
                    Json.requiredLong(object, "closed_trades"),
                    Json.requiredDouble(object, "win_rate"),
                    Json.requiredLong(object, "liquidations"));
        }
    }

    /** One holding in an account portfolio. */
    public static final class PortfolioAsset {
        public final long chainId;
        public final String tokenAddress;
        public final String symbol;
        public final int decimals;
        public final String balance;
        /** Null when the node had no oracle price for the token. */
        public final Double valueUsd;

        private PortfolioAsset(long chainId, String tokenAddress, String symbol, int decimals,
                               String balance, Double valueUsd) {
            this.chainId = chainId;
            this.tokenAddress = tokenAddress;
            this.symbol = symbol;
            this.decimals = decimals;
            this.balance = balance;
            this.valueUsd = valueUsd;
        }

        static PortfolioAsset fromJson(Map<String, Object> object) {
            return new PortfolioAsset(
                    Json.requiredLong(object, "chainId"),
                    Json.requiredString(object, "tokenAddress"),
                    Json.requiredString(object, "symbol"),
                    Json.requiredInt(object, "decimals"),
                    Json.requiredString(object, "balance"),
                    Json.optionalDouble(object, "valueUSD"));
        }
    }

    /** The account portfolio snapshot. */
    public static final class Portfolio {
        public final double totalValueUsd;
        public final List<PortfolioAsset> assets;
        public final double realizedPnlUsd;
        public final double unrealizedPnlUsd;
        public final int openPositions;
        public final String updatedAt;

        private Portfolio(double totalValueUsd, List<PortfolioAsset> assets, double realizedPnlUsd,
                          double unrealizedPnlUsd, int openPositions, String updatedAt) {
            this.totalValueUsd = totalValueUsd;
            this.assets = assets;
            this.realizedPnlUsd = realizedPnlUsd;
            this.unrealizedPnlUsd = unrealizedPnlUsd;
            this.openPositions = openPositions;
            this.updatedAt = updatedAt;
        }

        static Portfolio fromJson(Map<String, Object> object) {
            List<Object> items = Json.asArray(Json.field(object, "assets"), "assets");
            List<PortfolioAsset> assets = new ArrayList<>(items.size());
            for (Object item : items) {
                assets.add(PortfolioAsset.fromJson(Json.asObject(item, "asset")));
            }
            return new Portfolio(
                    Json.requiredDouble(object, "totalValueUSD"),
                    assets,
                    Json.requiredDouble(object, "realizedPnlUsd"),
                    Json.requiredDouble(object, "unrealizedPnlUsd"),
                    Json.requiredInt(object, "openPositions"),
                    Json.optionalString(object, "updatedAt"));
        }
    }

    /** One transaction in the account history. */
    public static final class UserTransaction {
        public final String hash;
        public final long chainId;
        public final String type;
        public final String status;
        public final String amount;
        public final String tokenSymbol;
        public final String createdAt;

        private UserTransaction(String hash, long chainId, String type, String status,
                                String amount, String tokenSymbol, String createdAt) {
            this.hash = hash;
            this.chainId = chainId;
            this.type = type;
            this.status = status;
            this.amount = amount;
            this.tokenSymbol = tokenSymbol;
            this.createdAt = createdAt;
        }

        static UserTransaction fromJson(Map<String, Object> object) {
            return new UserTransaction(
                    Json.requiredString(object, "hash"),
                    Json.requiredLong(object, "chainId"),
                    Json.requiredString(object, "type"),
                    Json.requiredString(object, "status"),
                    Json.requiredString(object, "amount"),
                    Json.requiredString(object, "tokenSymbol"),
                    Json.requiredString(object, "createdAt"));
        }
    }

}