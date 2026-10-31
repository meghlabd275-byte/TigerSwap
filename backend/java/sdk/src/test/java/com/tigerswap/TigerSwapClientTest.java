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
                + "\"priceImpact\":0.01,\"route\":[\"RUSD\",\"TGR\"],\"gasEstimate\":180000}";
        try (StubServer server = new StubServer()) {
            server.respond(200, body);
            TigerSwapClient client = new TigerSwapClient(server.baseURL(), "");
            TigerSwapClient.SwapQuote quote =
                    client.getQuote("0xaaa", "0xbbb", "1000", 1, 50);

            check("quote amount out", "999".equals(quote.amountOut));
            check("quote route", quote.route.equals(List.of("RUSD", "TGR")));
            check("quote gas", quote.gasEstimate == 180000);
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
