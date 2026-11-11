package com.tigerswap;

import com.sun.net.httpserver.HttpExchange;
import com.sun.net.httpserver.HttpServer;

import java.io.ByteArrayOutputStream;
import java.io.IOException;
import java.io.InputStream;
import java.io.OutputStream;
import java.net.InetSocketAddress;
import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.List;
import java.util.concurrent.atomic.AtomicReference;

/**
 * Tests for the TigerSwap Java SDK.
 *
 * The SDK's own HTTP and JSON code runs for real: requests go to an HTTP server
 * bound to loopback on an ephemeral port, and the SDK parses the bytes that
 * server writes back. Nothing here replaces the code under test, so a passing
 * run exercises the same path a caller would take against a live node.
 */
public final class TigerSwapClientTest {

    private static int checks;
    private static final List<String> failures = new ArrayList<>();

    public static void main(String[] args) throws Exception {
        jsonParsesNestedStructures();
        jsonRejectsMalformedInput();
        jsonWritesEscapedStrings();
        jsonRejectsNonFiniteNumbers();

        chainsAreParsedFromAResponse();
        tokensUseAChainIdQueryParameter();
        quotePostsAStructuredBody();
        quoteEscapesCallerSuppliedText();
        quoteRejectsAnUnusableBody();
        buildSwapReturnsTheServerCalldata();
        serverErrorsSurfaceTheStatusCode();
        malformedBodyRaises();
        emptyBodyRaises();
        bearerTokenIsSent();
        noTokenMeansNoAuthorizationHeader();
        constructorValidatesItsInput();
        quoteValidatesItsInput();
        farmsAreDecodedWithTheirRewards();
        farmWithNoPriceHasNullApr();
        airdropsAreDecoded();
        nftCollectionsAreDecoded();
        nftListingsJoinTheirTokenAndCollection();
        nftOffersJoinTheirTokenAndCollection();
        nftConfigIsDecoded();
        analyticsOverviewIsDecoded();
        analyticsVolumeDecodesSeries();
        analyticsPairsDecodeEnvelope();
        analyticsTradersDecodeEnvelope();
        analyticsReadsRejectServerErrors();
        portfolioDecodesRealPayload();
        transactionsDecodeArrayPayload();

        System.out.println();
        System.out.println("checks run: " + checks);
        if (failures.isEmpty()) {
            System.out.println("all checks passed");
            return;
        }
        System.out.println("failures: " + failures.size());
        for (String failure : failures) {
            System.out.println("  - " + failure);
        }
        System.exit(1);
    }

    // ------------------------------------------------------------------
    // Json
    // ------------------------------------------------------------------

    private static void jsonParsesNestedStructures() {
        Object value = Json.parse("{\"a\":[1,2,{\"b\":\"c\"}],\"d\":true,\"e\":null,\"f\":-1.5e2}");
        var object = Json.asObject(value, "root");
        List<Object> array = Json.asArray(Json.field(object, "a"), "a");
        check("nested array length", array.size() == 3);
        check("nested object value",
                "c".equals(Json.field(Json.asObject(array.get(2), "a[2]"), "b")));
        check("boolean field", Boolean.TRUE.equals(Json.field(object, "d")));
        check("null field", Json.field(object, "e") == null);
        check("exponent number", Json.requiredDouble(object, "f") == -150.0);
    }

    private static void jsonRejectsMalformedInput() {
        for (String bad : new String[]{"", "{", "{\"a\":}", "[1,]", "{\"a\":1}trailing", "tru"}) {
            boolean threw = false;
            try {
                Json.parse(bad);
            } catch (Json.JsonException e) {
                threw = true;
            }
            check("rejects '" + bad + "'", threw);
        }
    }

    private static void jsonWritesEscapedStrings() {
        String hostile = "he said \"hi\"\\\n\u0007";
        String encoded = Json.write(java.util.Map.of("k", hostile));
        // The encoded form must round-trip to the original value.
        Object decoded = Json.parse(encoded);
        check("round-trips hostile string",
                hostile.equals(Json.requiredString(Json.asObject(decoded, "root"), "k")));
        check("escapes the quote", encoded.contains("\\\""));
        check("escapes the backslash", encoded.contains("\\\\"));
        check("escapes the newline", encoded.contains("\\n"));
        check("escapes the control char", encoded.contains("\\u0007"));
    }

    private static void jsonRejectsNonFiniteNumbers() {
        boolean threw = false;
        try {
            Json.write(java.util.Map.of("n", Double.NaN));
        } catch (Json.JsonException e) {
            threw = true;
        }
        check("rejects NaN", threw);
    }

    // ------------------------------------------------------------------
    // client against a real server
    // ------------------------------------------------------------------

    private static void chainsAreParsedFromAResponse() throws Exception {
        String body = "[{\"id\":1,\"name\":\"Ethereum\",\"chainType\":\"evm\","
                + "\"nativeSymbol\":\"ETH\",\"explorerURL\":\"https://etherscan.io\","
                + "\"isActive\":true}]";
        try (StubServer server = new StubServer()) {
            server.respond(200, body);
            TigerSwapClient client = new TigerSwapClient(server.baseURL(), "");
            List<TigerSwapClient.Chain> chains = client.getChains();

            check("one chain", chains.size() == 1);
            check("chain id", chains.get(0).id == 1);
            check("chain name", "Ethereum".equals(chains.get(0).name));
            check("chain active", chains.get(0).isActive);
            check("request path", "/api/v1/chains".equals(server.lastPath()));
        }
    }

    private static void tokensUseAChainIdQueryParameter() throws Exception {
        try (StubServer server = new StubServer()) {
            server.respond(200, "[]");
            TigerSwapClient client = new TigerSwapClient(server.baseURL(), "");
            client.getTokens(56);
            check("token query string", "/api/v1/tokens?chainId=56".equals(server.lastPath()));
        }
    }

    private static void quotePostsAStructuredBody() throws Exception {
        String body = "{\"amountIn\":\"1000\",\"amountOut\":\"999\",\"amountOutMin\":\"994\","
                + "\"priceImpact\":0.01,\"route\":[\"RUSD\",\"TGR\"],\"gasEstimate\":180000,"
                + "\"routeHops\":[{\"tokenIn\":\"0xaaa\",\"tokenOut\":\"0xbbb\","
                + "\"family\":\"tigerswap_v2\",\"pool\":\"0xpool\"}]}";
        try (StubServer server = new StubServer()) {
            server.respond(200, body);
            TigerSwapClient client = new TigerSwapClient(server.baseURL(), "");
            TigerSwapClient.SwapQuote quote =
                    client.getQuote("0xaaa", "0xbbb", "1000", 1, 50);

            check("quote amount out", "999".equals(quote.amountOut));
            check("quote route", quote.route.equals(List.of("RUSD", "TGR")));
            check("quote gas", quote.gasEstimate == 180000);
            check("quote routeHops decoded", quote.routeHops.size() == 1);
            check("quote routeHops family",
                    "tigerswap_v2".equals(quote.routeHops.get(0).family));
            check("quote legs empty when absent", quote.legs.isEmpty());
            check("method is POST", "POST".equals(server.lastMethod()));
            check("path is quote", "/api/v1/swap/quote".equals(server.lastPath()));

            // The server must have received well-formed JSON with the exact fields.
            var sent = Json.asObject(Json.parse(server.lastBody()), "request");
            check("token_in sent", "0xaaa".equals(Json.requiredString(sent, "token_in")));
            check("chain_id sent", Json.requiredLong(sent, "chain_id") == 1);
            check("slippage sent", Json.requiredLong(sent, "slippage_bps") == 50);
        }
    }

    private static void quoteEscapesCallerSuppliedText() throws Exception {
        // A value containing JSON metacharacters must not change the request shape.
        String hostile = "0x\" ,\"injected\":true,\"x\":\"";
        try (StubServer server = new StubServer()) {
            server.respond(200, "{\"amountIn\":\"1\",\"amountOut\":\"1\",\"amountOutMin\":\"1\","
                    + "\"priceImpact\":0,\"route\":[],\"gasEstimate\":1}");
            TigerSwapClient client = new TigerSwapClient(server.baseURL(), "");
            client.getQuote(hostile, "0xbbb", "1", 1, 50);

            var sent = Json.asObject(Json.parse(server.lastBody()), "request");
            check("hostile value preserved verbatim",
                    hostile.equals(Json.requiredString(sent, "token_in")));
            check("no injected key", Json.field(sent, "injected") == null);
        }
    }

    private static void quoteRejectsAnUnusableBody() throws Exception {
        try (StubServer server = new StubServer()) {
            server.respond(200, "{\"amountOut\":\"1\"}");
            TigerSwapClient client = new TigerSwapClient(server.baseURL(), "");
            boolean threw = false;
            try {
                client.getQuote("0xaaa", "0xbbb", "1", 1, 50);
            } catch (TigerSwapClient.ApiException | Json.JsonException e) {
                threw = true;
            }
            check("missing fields raise", threw);
        }
    }

    private static void buildSwapReturnsTheServerCalldata() throws Exception {
        String body = "{\"to\":\"0xrouter\",\"data\":\"0xdeadbeef\",\"value\":\"0\","
                + "\"gas_limit\":\"210000\"}";
        try (StubServer server = new StubServer()) {
            server.respond(200, body);
            TigerSwapClient client = new TigerSwapClient(server.baseURL(), "");
            TigerSwapClient.UnsignedTransaction tx =
                    client.buildSwap("0xaaa", "0xbbb", "1", 1, "0xrecipient", 100);

            check("calldata returned", "0xdeadbeef".equals(tx.data));
            check("gas limit returned", "210000".equals(tx.gasLimit));
            check("build path", "/api/v1/swap/build".equals(server.lastPath()));
        }
    }

    private static void serverErrorsSurfaceTheStatusCode() throws Exception {
        try (StubServer server = new StubServer()) {
            server.respond(503, "{\"error\":\"maintenance\"}");
            TigerSwapClient client = new TigerSwapClient(server.baseURL(), "");
            try {
                client.getChains();
                check("503 raises", false);
            } catch (TigerSwapClient.ApiException e) {
                check("503 raises", true);
                check("503 status carried", e.getStatusCode() == 503);
            }
        }
    }

    private static void malformedBodyRaises() throws Exception {
        try (StubServer server = new StubServer()) {
            server.respond(200, "not json at all");
            TigerSwapClient client = new TigerSwapClient(server.baseURL(), "");
            boolean threw = false;
            try {
                client.getChains();
            } catch (TigerSwapClient.ApiException e) {
                threw = true;
            }
            check("malformed body raises ApiException", threw);
        }
    }

    private static void emptyBodyRaises() throws Exception {
        try (StubServer server = new StubServer()) {
            server.respond(200, "");
            TigerSwapClient client = new TigerSwapClient(server.baseURL(), "");
            boolean threw = false;
            try {
                client.getChains();
            } catch (TigerSwapClient.ApiException e) {
                threw = true;
            }
            check("empty body raises", threw);
        }
    }

    private static void bearerTokenIsSent() throws Exception {
        try (StubServer server = new StubServer()) {
            server.respond(200, "[]");
            TigerSwapClient client = new TigerSwapClient(server.baseURL(), "secret-token");
            client.getChains();
            check("authorization header",
                    "Bearer secret-token".equals(server.lastHeader("Authorization")));
        }
    }

    private static void noTokenMeansNoAuthorizationHeader() throws Exception {
        try (StubServer server = new StubServer()) {
            server.respond(200, "[]");
            TigerSwapClient client = new TigerSwapClient(server.baseURL(), "");
            client.getChains();
            check("no authorization header", server.lastHeader("Authorization") == null);
        }
    }

    private static void constructorValidatesItsInput() {
        check("blank url rejected", throwsIllegalArgument(() -> new TigerSwapClient("", "")));
        check("scheme rejected", throwsIllegalArgument(() -> new TigerSwapClient("ftp://x", "")));
    }

    private static void quoteValidatesItsInput() throws Exception {
        try (StubServer server = new StubServer()) {
            TigerSwapClient client = new TigerSwapClient(server.baseURL(), "");
            check("blank tokenIn rejected",
                    throwsIllegalArgument(() -> client.getQuote("", "0xb", "1", 1, 50)));
            check("negative slippage rejected",
                    throwsIllegalArgument(() -> client.getQuote("0xa", "0xb", "1", 1, -1)));
            check("excess slippage rejected",
                    throwsIllegalArgument(() -> client.getQuote("0xa", "0xb", "1", 1, 10_001)));
            check("blank recipient rejected",
                    throwsIllegalArgument(() -> client.buildSwap("0xa", "0xb", "1", 1, "", 50)));
        }
    }

    private static void farmsAreDecodedWithTheirRewards() throws Exception {
        String body = "{\"farms\":[{\"farm\":{\"id\":\"f1\",\"key\":\"tgr-rusd\",\"name\":\"TGR/RUSD LP\","
                + "\"farm_type\":\"lp\",\"chain_id\":1,\"staking_token_address\":\"0x1\","
                + "\"staking_token_symbol\":\"LP\",\"staking_token_decimals\":18,"
                + "\"total_staked\":\"1000\",\"start_time\":\"2026-11-01T00:00:00Z\","
                + "\"end_time\":\"2026-12-01T00:00:00Z\",\"status\":\"active\"},"
                + "\"rewards\":[{\"id\":\"r1\",\"farm_id\":\"f1\",\"token_address\":\"0x2\","
                + "\"token_symbol\":\"TGR\",\"token_decimals\":18,\"rate_per_second\":\"10\"}],"
                + "\"apr\":12.5}],\"success\":true}";
        try (StubServer server = new StubServer()) {
            server.respond(200, body);
            List<TigerSwapClient.Farm> farms = new TigerSwapClient(server.baseURL(), "").getFarms();

            check("one farm", farms.size() == 1);
            check("farm key", "tgr-rusd".equals(farms.get(0).key));
            check("farm staked", "1000".equals(farms.get(0).totalStaked));
            check("farm reward stream", farms.get(0).rewards.size() == 1);
            check("reward symbol", "TGR".equals(farms.get(0).rewards.get(0).tokenSymbol));
            check("farm apr", farms.get(0).apr != null && farms.get(0).apr == 12.5);
            check("farms path", "/api/v1/farms".equals(server.lastPath()));
        }
    }

    private static void farmWithNoPriceHasNullApr() throws Exception {
        String body = "{\"farms\":[{\"farm\":{\"id\":\"f2\",\"key\":\"paused\",\"name\":\"Paused\","
                + "\"farm_type\":\"single\",\"chain_id\":1,\"staking_token_address\":\"0x3\","
                + "\"staking_token_symbol\":\"TGR\",\"staking_token_decimals\":18,"
                + "\"total_staked\":\"0\",\"start_time\":\"2026-11-01T00:00:00Z\","
                + "\"end_time\":\"2026-12-01T00:00:00Z\",\"status\":\"paused\"},"
                + "\"rewards\":[],\"apr\":null}],\"success\":true}";
        try (StubServer server = new StubServer()) {
            server.respond(200, body);
            List<TigerSwapClient.Farm> farms = new TigerSwapClient(server.baseURL(), "").getFarms();
            // A null APR means no oracle price; it must stay null, never become 0.
            check("null apr stays null", farms.get(0).apr == null);
            check("empty rewards", farms.get(0).rewards.isEmpty());
        }
    }

    private static void airdropsAreDecoded() throws Exception {
        String body = "{\"campaigns\":[{\"id\":\"c1\",\"key\":\"tgr-genesis\",\"name\":\"TGR Genesis\","
                + "\"chain_id\":1,\"token_address\":\"0x3\",\"token_symbol\":\"TGR\","
                + "\"token_decimals\":18,\"total_allocated\":\"750\",\"funded\":\"750\","
                + "\"total_claimed\":\"500\",\"start_time\":\"2026-11-01T00:00:00Z\","
                + "\"end_time\":\"2026-12-01T00:00:00Z\",\"status\":\"active\"}],\"success\":true}";
        try (StubServer server = new StubServer()) {
            server.respond(200, body);
            List<TigerSwapClient.AirdropCampaign> campaigns =
                    new TigerSwapClient(server.baseURL(), "").getAirdrops();

            check("one campaign", campaigns.size() == 1);
            check("campaign key", "tgr-genesis".equals(campaigns.get(0).key));
            check("campaign funded", "750".equals(campaigns.get(0).funded));
            check("campaign claimed", "500".equals(campaigns.get(0).totalClaimed));
            check("airdrops path", "/api/v1/airdrops".equals(server.lastPath()));
        }
    }

    private static void nftCollectionsAreDecoded() throws Exception {
        String body = "{\"collections\":[{\"id\":\"col-1\",\"key\":\"tiger-art\",\"name\":\"Tiger Art\","
                + "\"symbol\":\"TART\",\"chain_id\":1,\"contract_address\":\"0x5555555555555555555555555555555555555555\","
                + "\"base_uri\":\"https://cdn.tigerswap.example/nft/\",\"total_supply\":3,\"status\":\"active\","
                + "\"created_by\":\"admin-1\",\"created_at\":\"2026-11-05T13:20:05Z\",\"updated_at\":\"2026-11-05T13:20:05Z\"}],"
                + "\"success\":true}";
        try (StubServer server = new StubServer()) {
            server.respond(200, body);
            List<TigerSwapClient.NFTCollection> collections =
                    new TigerSwapClient(server.baseURL(), "").getNFTCollections();

            check("one collection", collections.size() == 1);
            check("collection key", "tiger-art".equals(collections.get(0).key));
            check("collection supply", collections.get(0).totalSupply == 3);
            check("collection status", "active".equals(collections.get(0).status));
            check("collections path", "/api/v1/nft/collections".equals(server.lastPath()));
        }
    }

    private static void nftListingsJoinTheirTokenAndCollection() throws Exception {
        String body = "{\"listings\":[{\"listing\":{\"id\":\"lst-1\",\"collection_id\":\"col-1\","
                + "\"token_id\":1,\"seller_user_id\":\"seller-1\","
                + "\"payment_token_address\":\"0x4444444444444444444444444444444444444444\","
                + "\"payment_token_symbol\":\"RUSD\",\"payment_token_decimals\":18,\"price\":\"100000\","
                + "\"fee_bps\":250,\"deadline\":\"2026-11-06T13:20:05Z\",\"status\":\"active\","
                + "\"created_at\":\"2026-11-05T13:20:05Z\",\"updated_at\":\"2026-11-05T13:20:05Z\"},"
                + "\"token\":{\"id\":\"tok-1\",\"collection_id\":\"col-1\",\"token_id\":1,"
                + "\"owner_user_id\":\"__nft_escrow__\",\"uri\":\"token.json\","
                + "\"escrow_listing_id\":\"lst-1\",\"minted_by\":\"admin-1\","
                + "\"created_at\":\"2026-11-05T13:20:05Z\",\"updated_at\":\"2026-11-05T13:20:05Z\"},"
                + "\"collection\":{\"id\":\"col-1\",\"key\":\"tiger-art\",\"name\":\"Tiger Art\","
                + "\"symbol\":\"TART\",\"chain_id\":1,\"contract_address\":\"\",\"base_uri\":\"\","
                + "\"total_supply\":1,\"status\":\"active\",\"created_by\":\"admin-1\","
                + "\"created_at\":\"2026-11-05T13:20:05Z\",\"updated_at\":\"2026-11-05T13:20:05Z\"},"
                + "\"active\":true}],\"success\":true}";
        try (StubServer server = new StubServer()) {
            server.respond(200, body);
            List<TigerSwapClient.NFTListingView> listings =
                    new TigerSwapClient(server.baseURL(), "").getNFTListings();

            check("one listing", listings.size() == 1);
            check("listing price", "100000".equals(listings.get(0).listing.price));
            check("listing fee", listings.get(0).listing.feeBps == 250);
            // The listing escrows the token, so the owner is the escrow sentinel.
            check("token owner is escrow", "__nft_escrow__".equals(listings.get(0).token.ownerUserId));
            check("token escrow id", "lst-1".equals(listings.get(0).token.escrowListingId));
            check("joined collection key", "tiger-art".equals(listings.get(0).collection.key));
            check("listing active", listings.get(0).active);
            check("listings path", "/api/v1/nft/listings".equals(server.lastPath()));
        }
    }

    private static void nftOffersJoinTheirTokenAndCollection() throws Exception {
        String body = "{\"offers\":[{\"offer\":{\"id\":\"off-1\",\"collection_id\":\"col-1\","
                + "\"token_id\":2,\"buyer_user_id\":\"buyer-2\","
                + "\"payment_token_address\":\"0x4444444444444444444444444444444444444444\","
                + "\"payment_token_symbol\":\"RUSD\",\"payment_token_decimals\":18,\"amount\":\"50000\","
                + "\"fee_bps\":250,\"deadline\":\"2026-11-06T13:20:05Z\",\"status\":\"active\","
                + "\"created_at\":\"2026-11-05T13:20:05Z\",\"updated_at\":\"2026-11-05T13:20:05Z\"},"
                + "\"token\":{\"id\":\"tok-2\",\"collection_id\":\"col-1\",\"token_id\":2,"
                + "\"owner_user_id\":\"seller-2\",\"uri\":\"token.json\",\"escrow_listing_id\":null,"
                + "\"minted_by\":\"admin-1\",\"created_at\":\"2026-11-05T13:20:05Z\","
                + "\"updated_at\":\"2026-11-05T13:20:05Z\"},"
                + "\"collection\":{\"id\":\"col-1\",\"key\":\"tiger-art\",\"name\":\"Tiger Art\","
                + "\"symbol\":\"TART\",\"chain_id\":1,\"contract_address\":\"\",\"base_uri\":\"\","
                + "\"total_supply\":2,\"status\":\"active\",\"created_by\":\"admin-1\","
                + "\"created_at\":\"2026-11-05T13:20:05Z\",\"updated_at\":\"2026-11-05T13:20:05Z\"},"
                + "\"active\":true}],\"success\":true}";
        try (StubServer server = new StubServer()) {
            server.respond(200, body);
            List<TigerSwapClient.NFTOfferView> offers =
                    new TigerSwapClient(server.baseURL(), "").getNFTOffers("col-1", 2);

            check("one offer", offers.size() == 1);
            check("offer amount", "50000".equals(offers.get(0).offer.amount));
            check("offer buyer", "buyer-2".equals(offers.get(0).offer.buyerUserId));
            // A token that is not escrowed has a null escrow id, not an empty one.
            check("token not escrowed", offers.get(0).token.escrowListingId == null);
            check("offer token owner", "seller-2".equals(offers.get(0).token.ownerUserId));
            check("offer joined collection", "tiger-art".equals(offers.get(0).collection.key));
            check("offers query",
                    "/api/v1/nft/offers?collection_id=col-1&token_id=2".equals(server.lastPath()));
        }
    }

    private static void nftConfigIsDecoded() throws Exception {
        String body = "{\"escrow_account\":\"__nft_escrow__\",\"fee_account\":\"__nft_fee__\","
                + "\"fee_bps\":250,\"max_fee_bps\":500,\"success\":true}";
        try (StubServer server = new StubServer()) {
            server.respond(200, body);
            TigerSwapClient.NFTMarketplaceConfig config =
                    new TigerSwapClient(server.baseURL(), "").getNFTConfig();

            check("fee bps", config.feeBps == 250);
            check("max fee bps", config.maxFeeBps == 500);
            check("escrow account", "__nft_escrow__".equals(config.escrowAccount));
            check("config path", "/api/v1/nft/config".equals(server.lastPath()));
        }
    }

    // ------------------------------------------------------------------
    // analytics
    // ------------------------------------------------------------------

    private static void analyticsOverviewIsDecoded() throws Exception {
        String body = "{\"total_tvl_usd\":0,\"volume_24h_usd\":1500,\"fees_24h_usd\":4.5,"
                + "\"volume_7d_usd\":1500,\"fees_7d_usd\":4.5,\"total_pools\":0,\"active_pools\":0,"
                + "\"total_farms\":0,\"active_farms\":0,\"total_tokens\":0,\"total_chains\":0,"
                + "\"active_chains\":0,\"total_users\":0,\"total_trades_24h\":2,"
                + "\"open_interest_usd\":2500,\"margin_borrowed_usd\":0,\"nft_listings_active\":0,"
                + "\"updated_at\":\"2026-11-08T03:30:30.928888113Z\"}";
        try (StubServer server = new StubServer()) {
            server.respond(200, body);
            TigerSwapClient.ProtocolOverview overview =
                    new TigerSwapClient(server.baseURL(), "").getAnalyticsOverview();

            check("overview volume 24h", overview.volume24hUsd == 1500.0);
            check("overview fees 24h", overview.fees24hUsd == 4.5);
            check("overview trades 24h", overview.totalTrades24h == 2);
            check("overview open interest", overview.openInterestUsd == 2500.0);
            check("overview path", "/api/v1/analytics/overview".equals(server.lastPath()));
        }
    }

    private static void analyticsVolumeDecodesSeries() throws Exception {
        String body = "{\"chain_id\":0,\"interval\":60,\"pair\":\"TGR/RUSD\",\"series\":["
                + "{\"timestamp\":\"2026-11-08T03:25:00Z\",\"volume_usd\":1000,\"fees_usd\":3,"
                + "\"trade_count\":1,\"taker_buy_volume_usd\":1000,\"taker_sell_volume_usd\":0}]}";
        try (StubServer server = new StubServer()) {
            server.respond(200, body);
            TigerSwapClient.VolumeSeries series =
                    new TigerSwapClient(server.baseURL(), "").getAnalyticsVolume("TGR/RUSD", 0, 60, 3600);

            check("volume pair", "TGR/RUSD".equals(series.pair));
            check("volume interval", series.interval == 60);
            check("volume points", series.series.size() == 1);
            check("volume first point", series.series.get(0).volumeUsd == 1000.0);
            check("volume buy side", series.series.get(0).takerBuyVolumeUsd == 1000.0);
            check("volume query", server.lastPath().contains("pair=TGR%2FRUSD"));
            check("volume window query", server.lastPath().contains("window=3600"));
        }
    }

    private static void analyticsPairsDecodeEnvelope() throws Exception {
        String body = "{\"pairs\":[{\"pair\":\"TGR/RUSD\",\"chain_id\":1,\"volume_usd\":1000,"
                + "\"fees_usd\":3,\"trade_count\":1,\"taker_buy_volume_usd\":1000,"
                + "\"taker_sell_volume_usd\":0},{\"pair\":\"ETH/RUSD\",\"chain_id\":1,"
                + "\"volume_usd\":500,\"fees_usd\":1.5,\"trade_count\":1,"
                + "\"taker_buy_volume_usd\":0,\"taker_sell_volume_usd\":500}],\"window\":3600}";
        try (StubServer server = new StubServer()) {
            server.respond(200, body);
            List<TigerSwapClient.PairAnalytics> pairs =
                    new TigerSwapClient(server.baseURL(), "").getAnalyticsPairs(0, 3600, 10);

            check("pairs count", pairs.size() == 2);
            check("pairs first", "TGR/RUSD".equals(pairs.get(0).pair));
            check("pairs second sell", pairs.get(1).takerSellVolumeUsd == 500.0);
        }
    }

    private static void analyticsTradersDecodeEnvelope() throws Exception {
        String body = "{\"traders\":[{\"user_id\":\"u1\",\"realized_pnl\":120,\"volume_usd\":2500,"
                + "\"closed_trades\":1,\"win_rate\":1,\"liquidations\":0}]}";
        try (StubServer server = new StubServer()) {
            server.respond(200, body);
            List<TigerSwapClient.TopTrader> traders =
                    new TigerSwapClient(server.baseURL(), "").getAnalyticsTraders(0, 10);

            check("traders count", traders.size() == 1);
            check("trader id", "u1".equals(traders.get(0).userId));
            check("trader pnl", traders.get(0).realizedPnl == 120.0);
            check("trader win rate", traders.get(0).winRate == 1.0);
        }
    }

    private static void analyticsReadsRejectServerErrors() throws Exception {
        try (StubServer server = new StubServer()) {
            server.respond(500, "{\"error\":\"boom\"}");
            TigerSwapClient client = new TigerSwapClient(server.baseURL(), "");

            boolean overviewThrew = false;
            try {
                client.getAnalyticsOverview();
            } catch (TigerSwapClient.ApiException e) {
                overviewThrew = e.getStatusCode() == 500;
            }
            check("overview surfaces 500", overviewThrew);

            boolean tradersThrew = false;
            try {
                client.getAnalyticsTraders(0, 10);
            } catch (TigerSwapClient.ApiException e) {
                tradersThrew = e.getStatusCode() == 500;
            }
            check("traders surfaces 500", tradersThrew);
        }
    }

    private static void portfolioDecodesRealPayload() throws Exception {
        String body = "{\"totalValueUSD\":4000,\"assets\":[{\"chainId\":1,"
                + "\"tokenAddress\":\"0x0000000000000000000000000000000000000001\","
                + "\"symbol\":\"ETH\",\"decimals\":18,\"balance\":\"2000000000000000000\","
                + "\"valueUSD\":4000}],\"realizedPnlUsd\":-40.25,\"unrealizedPnlUsd\":150.5,"
                + "\"openPositions\":1,\"updatedAt\":\"2026-11-08T12:20:36Z\"}";
        try (StubServer server = new StubServer()) {
            server.respond(200, body);
            TigerSwapClient.Portfolio portfolio =
                    new TigerSwapClient(server.baseURL(), "").getPortfolio();

            check("portfolio total", portfolio.totalValueUsd == 4000.0);
            check("portfolio assets", portfolio.assets.size() == 1);
            check("portfolio asset symbol", "ETH".equals(portfolio.assets.get(0).symbol));
            check("portfolio asset value", portfolio.assets.get(0).valueUsd == 4000.0);
            check("portfolio realized pnl", portfolio.realizedPnlUsd == -40.25);
            check("portfolio unrealized pnl", portfolio.unrealizedPnlUsd == 150.5);
            check("portfolio open positions", portfolio.openPositions == 1);
            check("portfolio path", "/api/v1/user/portfolio".equals(server.lastPath()));
        }
    }

    private static void transactionsDecodeArrayPayload() throws Exception {
        String body = "[{\"hash\":\"0xdeadbeef\",\"chainId\":1,\"type\":\"swap\","
                + "\"status\":\"confirmed\",\"amount\":\"1000000000000000000\","
                + "\"tokenSymbol\":\"ETH\",\"createdAt\":\"2026-11-08T12:20:36Z\"}]";
        try (StubServer server = new StubServer()) {
            server.respond(200, body);
            List<TigerSwapClient.UserTransaction> txs =
                    new TigerSwapClient(server.baseURL(), "").getTransactions();

            check("transactions count", txs.size() == 1);
            check("transaction hash", "0xdeadbeef".equals(txs.get(0).hash));
            check("transaction symbol", "ETH".equals(txs.get(0).tokenSymbol));
            check("transaction status", "confirmed".equals(txs.get(0).status));
            check("transactions path", "/api/v1/user/transactions".equals(server.lastPath()));
        }
    }

    // ------------------------------------------------------------------
    // helpers
    // ------------------------------------------------------------------

    private static boolean throwsIllegalArgument(Runnable action) {
        try {
            action.run();
            return false;
        } catch (IllegalArgumentException e) {
            return true;
        }
    }

    private static void check(String label, boolean condition) {
        checks++;
        if (!condition) {
            failures.add(label);
            System.out.println("FAIL " + label);
        }
    }

    /** A real HTTP server on loopback that records the last request it served. */
    private static final class StubServer implements AutoCloseable {
        private final HttpServer server;
        private final AtomicReference<String> path = new AtomicReference<>();
        private final AtomicReference<String> method = new AtomicReference<>();
        private final AtomicReference<String> body = new AtomicReference<>();
        private final AtomicReference<String> authorization = new AtomicReference<>();
        private volatile int status = 200;
        private volatile String responseBody = "[]";

        StubServer() throws IOException {
            server = HttpServer.create(new InetSocketAddress("127.0.0.1", 0), 0);
            server.createContext("/", this::handle);
            server.setExecutor(null);
            server.start();
        }

        void respond(int status, String body) {
            this.status = status;
            this.responseBody = body;
        }

        String baseURL() {
            return "http://127.0.0.1:" + server.getAddress().getPort();
        }

        String lastPath() {
            return path.get();
        }

        String lastMethod() {
            return method.get();
        }

        String lastBody() {
            return body.get();
        }

        String lastHeader(String name) {
            return "Authorization".equalsIgnoreCase(name) ? authorization.get() : null;
        }

        private void handle(HttpExchange exchange) throws IOException {
            path.set(exchange.getRequestURI().getPath()
                    + (exchange.getRequestURI().getRawQuery() == null
                        ? ""
                        : "?" + exchange.getRequestURI().getRawQuery()));
            method.set(exchange.getRequestMethod());
            authorization.set(exchange.getRequestHeaders().getFirst("Authorization"));
            body.set(readAll(exchange.getRequestBody()));

            byte[] payload = responseBody.getBytes(StandardCharsets.UTF_8);
            exchange.getResponseHeaders().add("Content-Type", "application/json");
            exchange.sendResponseHeaders(status, payload.length);
            try (OutputStream out = exchange.getResponseBody()) {
                out.write(payload);
            }
        }

        private static String readAll(InputStream in) throws IOException {
            ByteArrayOutputStream buffer = new ByteArrayOutputStream();
            byte[] chunk = new byte[4096];
            int read;
            while ((read = in.read(chunk)) != -1) {
                buffer.write(chunk, 0, read);
            }
            return buffer.toString(StandardCharsets.UTF_8);
        }

        @Override
        public void close() {
            server.stop(0);
        }
    }
}
