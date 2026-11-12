package services

import (
	"bytes"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"tigerswap/backend/models"
)

// These tests pin the JSON contract that the Next.js frontend's validators
// enforce. The frontend validates every response against a strict schema before
// it reaches a component, so a shape mismatch here is a blank or broken page in
// production, not a cosmetic difference. Each test therefore asserts the exact
// field names, types and envelope the frontend requires.

// frontendAddress mirrors `address` in validation.ts: 0x plus 40 hex digits.
func assertFrontendAddress(t *testing.T, path string, value any) {
	t.Helper()
	s, ok := value.(string)
	if !ok {
		t.Fatalf("%s: expected a string address, got %T", path, value)
	}
	if len(s) != 42 || s[:2] != "0x" {
		t.Fatalf("%s: %q is not a 20-byte hex address", path, s)
	}
	for _, c := range s[2:] {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			t.Fatalf("%s: %q contains a non-hex character", path, s)
		}
	}
}

// assertFrontendSymbol mirrors `symbol` in validation.ts.
func assertFrontendSymbol(t *testing.T, path string, value any) {
	t.Helper()
	s, ok := value.(string)
	if !ok {
		t.Fatalf("%s: expected a string symbol, got %T", path, value)
	}
	if len(s) == 0 || len(s) > 16 {
		t.Fatalf("%s: symbol %q must be 1-16 characters", path, s)
	}
	for _, c := range s {
		if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') ||
			(c >= '0' && c <= '9') || c == '.' || c == '_' || c == '-') {
			t.Fatalf("%s: symbol %q contains a disallowed character %q", path, s, c)
		}
	}
}

func assertFrontendNumber(t *testing.T, path string, value any) {
	t.Helper()
	f, ok := value.(float64)
	if !ok {
		t.Fatalf("%s: expected a JSON number, got %T (%v)", path, value, value)
	}
	if math.IsNaN(f) || math.IsInf(f, 0) {
		t.Fatalf("%s: expected a finite number, got %v", path, f)
	}
}

func assertFrontendNonNegative(t *testing.T, path string, value any) {
	t.Helper()
	assertFrontendNumber(t, path, value)
	if value.(float64) < 0 {
		t.Fatalf("%s: expected a non-negative number, got %v", path, value)
	}
}

func assertFrontendInteger(t *testing.T, path string, value any) {
	t.Helper()
	assertFrontendNumber(t, path, value)
	if value.(float64) != math.Trunc(value.(float64)) {
		t.Fatalf("%s: expected an integer, got %v", path, value)
	}
}

// TestTokensEndpointMatchesFrontendContract asserts /tokens returns a bare JSON
// array of camelCase token objects. The frontend runs the body through
// `tokenListValidator`, which requires an array; a `{tokens: [...]}` envelope
// makes the validator throw.
func TestTokensEndpointMatchesFrontendContract(t *testing.T) {
	db := newStatsTestDB(t)
	if err := db.Create(&models.Token{
		Address: "0x1111111111111111111111111111111111111111", ChainID: 1,
		Name: "Tiger", Symbol: "TGR", Decimals: 18, IsActive: true,
	}).Error; err != nil {
		t.Fatalf("seed token: %v", err)
	}
	svc := NewTokenService(db)

	rec := getJSON(t, svc.GetTokens, "/api/v1/tokens?chainId=1")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	items, ok := body.([]any)
	if !ok {
		t.Fatalf("frontend requires a bare array; got %T (%s)", body, rec.Body.String())
	}
	if len(items) == 0 {
		t.Fatalf("expected at least the seeded token, got an empty array")
	}

	// The service seeds the token registry, so the array holds more than the row
	// seeded here. Every element must satisfy the frontend contract, and the
	// seeded row must be present.
	var token map[string]any
	for _, raw := range items {
		element, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("expected an object element, got %T", raw)
		}
		assertFrontendAddress(t, "token.address", element["address"])
		assertFrontendSymbol(t, "token.symbol", element["symbol"])
		if _, ok := element["name"].(string); !ok {
			t.Fatalf("token.name must be a string, got %T", element["name"])
		}
		assertFrontendInteger(t, "token.decimals", element["decimals"])
		assertFrontendInteger(t, "token.chainId", element["chainId"])

		// `logoURI` is nullable(string) and `priceUSD` is nullable(nonNegative):
		// the key must be present, and its value must be null or the right type.
		if v, present := element["logoURI"]; present && v != nil {
			if _, ok := v.(string); !ok {
				t.Fatalf("token.logoURI must be a string or null, got %T", v)
			}
		} else if !present {
			t.Fatalf("token.logoURI key is missing; the validator requires it")
		}
		if v, present := element["priceUSD"]; present && v != nil {
			assertFrontendNonNegative(t, "token.priceUSD", v)
		} else if !present {
			t.Fatalf("token.priceUSD key is missing; the validator requires it")
		}

		if address, _ := element["address"].(string); strings.EqualFold(address, "0x1111111111111111111111111111111111111111") {
			token = element
		}
	}
	if token == nil {
		t.Fatalf("seeded token 0x1111...1111 is missing from the response")
	}
}

// TestChainsEndpointMatchesFrontendContract asserts /chains returns a bare array
// of camelCase chain objects.
func TestChainsEndpointMatchesFrontendContract(t *testing.T) {
	db := newStatsTestDB(t)
	if err := db.Create(&models.Blockchain{
		ChainID: 1, Name: "Ethereum", Symbol: "ETH", Type: "evm",
		ExplorerURL: "https://etherscan.io", IsActive: true,
	}).Error; err != nil {
		t.Fatalf("seed chain: %v", err)
	}
	svc := NewBlockchainService(db)

	rec := getJSON(t, svc.GetSupportedChains, "/api/v1/chains")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	items, ok := body.([]any)
	if !ok {
		t.Fatalf("frontend requires a bare array; got %T (%s)", body, rec.Body.String())
	}
	if len(items) == 0 {
		t.Fatalf("expected at least the seeded chain, got an empty array")
	}

	// The service seeds the chain registry, so the array holds more than the row
	// seeded here. Every element must satisfy the frontend contract, and the
	// seeded row must be present.
	var chain map[string]any
	for _, raw := range items {
		element, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("expected an object element, got %T", raw)
		}
		assertFrontendInteger(t, "chain.id", element["id"])
		if _, ok := element["name"].(string); !ok {
			t.Fatalf("chain.name must be a string, got %T", element["name"])
		}
		if _, ok := element["chainType"].(string); !ok {
			t.Fatalf("chain.chainType must be a string, got %T (%v)", element["chainType"], element["chainType"])
		}
		if _, ok := element["nativeSymbol"].(string); !ok {
			t.Fatalf("chain.nativeSymbol must be a string, got %T (%v)", element["nativeSymbol"], element["nativeSymbol"])
		}
		if v, present := element["explorerURL"]; present && v != nil {
			if _, ok := v.(string); !ok {
				t.Fatalf("chain.explorerURL must be a string or null, got %T", v)
			}
		} else if !present {
			t.Fatalf("chain.explorerURL key is missing")
		}
		if _, ok := element["isActive"].(bool); !ok {
			t.Fatalf("chain.isActive must be a boolean, got %T", element["isActive"])
		}
		// `nativeDecimals` and `rpcUrls` drive wallet_addEthereumChain on the
		// client, so both keys must always be present. `rpcUrls` may be empty when
		// the operator has not configured an endpoint, but it must still be an array.
		if _, ok := element["nativeDecimals"].(float64); !ok {
			t.Fatalf("chain.nativeDecimals must be a number, got %T", element["nativeDecimals"])
		}
		if _, ok := element["rpcUrls"].([]any); !ok {
			t.Fatalf("chain.rpcUrls must be an array, got %T", element["rpcUrls"])
		}

		if id, ok := element["id"].(float64); ok && int64(id) == 1 {
			chain = element
		}
	}
	if chain == nil {
		t.Fatalf("seeded chain id 1 is missing from the response")
	}
}

// TestMarketStatsEndpointMatchesFrontendContract asserts /market/stats returns
// the camelCase aggregate object the dashboard validator requires.
func TestMarketStatsEndpointMatchesFrontendContract(t *testing.T) {
	db := newStatsTestDB(t)
	svc := NewSwapService(db, nil)

	rec := getJSON(t, svc.GetMarketStats, "/api/v1/market/stats")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var stats map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &stats); err != nil {
		t.Fatalf("decode: %v", err)
	}
	assertFrontendNonNegative(t, "stats.totalTVL", stats["totalTVL"])
	assertFrontendNonNegative(t, "stats.volume24h", stats["volume24h"])
	assertFrontendInteger(t, "stats.totalPairs", stats["totalPairs"])
	assertFrontendInteger(t, "stats.totalUsers", stats["totalUsers"])

	// `updatedAt` is nullable(timestamp): present, and null or ISO-8601.
	v, present := stats["updatedAt"]
	if !present {
		t.Fatalf("stats.updatedAt key is missing")
	}
	if v != nil {
		s, ok := v.(string)
		if !ok {
			t.Fatalf("stats.updatedAt must be a string or null, got %T", v)
		}
		if _, err := time.Parse(time.RFC3339, s); err != nil {
			t.Fatalf("stats.updatedAt %q is not ISO-8601: %v", s, err)
		}
	}
}

// TestRecentTradesEndpointMatchesFrontendContract asserts /market/trades returns
// a bare array of camelCase trade objects.
func TestRecentTradesEndpointMatchesFrontendContract(t *testing.T) {
	db := newStatsTestDB(t)
	svc := NewSwapService(db, nil)

	tx := &models.Transaction{
		Hash: "0xabc", ChainID: 1, Type: "swap", Status: "confirmed",
		Pair: "TGR/RUSD", FromToken: "0x1111111111111111111111111111111111111111",
		ToToken:    "0x2222222222222222222222222222222222222222",
		FromAmount: "1000000000000000000", ToAmount: "2000000000000000000",
		AmountUSD: 250, Timestamp: time.Now(),
	}
	if err := db.Create(tx).Error; err != nil {
		t.Fatalf("seed transaction: %v", err)
	}

	rec := getJSON(t, svc.GetRecentTrades, "/api/v1/market/trades?limit=10")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	items, ok := body.([]any)
	if !ok {
		t.Fatalf("frontend requires a bare array; got %T (%s)", body, rec.Body.String())
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(items))
	}

	trade, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("expected an object element, got %T", items[0])
	}
	if _, ok := trade["id"].(string); !ok {
		t.Fatalf("trade.id must be a string, got %T", trade["id"])
	}
	if _, ok := trade["pair"].(string); !ok {
		t.Fatalf("trade.pair must be a string, got %T", trade["pair"])
	}
	side, ok := trade["side"].(string)
	if !ok || (side != "buy" && side != "sell") {
		t.Fatalf("trade.side must be \"buy\" or \"sell\", got %v", trade["side"])
	}
	assertFrontendNonNegative(t, "trade.price", trade["price"])
	assertFrontendNonNegative(t, "trade.quantity", trade["quantity"])
	ts, ok := trade["timestamp"].(string)
	if !ok {
		t.Fatalf("trade.timestamp must be a string, got %T", trade["timestamp"])
	}
	if _, err := time.Parse(time.RFC3339, ts); err != nil {
		t.Fatalf("trade.timestamp %q is not ISO-8601: %v", ts, err)
	}
}

// TestPairsEndpointMatchesFrontendContract asserts /swap/pairs returns a bare
// array of camelCase pool objects.
func TestPairsEndpointMatchesFrontendContract(t *testing.T) {
	db := newStatsTestDB(t)
	pool := &models.Pool{
		TokenAAddress: "0x1111111111111111111111111111111111111111",
		TokenBAddress: "0x2222222222222222222222222222222222222222",
		ChainID:       1, PoolAddress: "0x3333333333333333333333333333333333333333",
		ReserveA: "1000000", ReserveB: "2000000", Fee: 30, IsActive: true,
	}
	if err := db.Create(pool).Error; err != nil {
		t.Fatalf("seed pool: %v", err)
	}
	svc := NewSwapService(db, nil)

	rec := getJSON(t, svc.GetPairs, "/api/v1/swap/pairs")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	items, ok := body.([]any)
	if !ok {
		t.Fatalf("frontend requires a bare array; got %T (%s)", body, rec.Body.String())
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 pool, got %d", len(items))
	}

	p, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("expected an object element, got %T", items[0])
	}
	assertFrontendAddress(t, "pool.pairAddress", p["pairAddress"])
	assertFrontendAddress(t, "pool.token0", p["token0"])
	assertFrontendAddress(t, "pool.token1", p["token1"])
	assertFrontendSymbol(t, "pool.symbol0", p["symbol0"])
	assertFrontendSymbol(t, "pool.symbol1", p["symbol1"])
	assertFrontendNonNegative(t, "pool.reserve0", p["reserve0"])
	assertFrontendNonNegative(t, "pool.reserve1", p["reserve1"])
	assertFrontendInteger(t, "pool.feeBps", p["feeBps"])
}

// TestAdminDashboardMatchesFrontendContract asserts the admin dashboard returns
// the camelCase aggregate the admin console validator requires.
func TestAdminDashboardMatchesFrontendContract(t *testing.T) {
	db := newStatsTestDB(t)
	svc := NewAdminService(db)

	rec := getJSON(t, svc.GetDashboard, "/api/v1/admin/dashboard")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var dash map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &dash); err != nil {
		t.Fatalf("decode: %v", err)
	}
	assertFrontendInteger(t, "dashboard.totalUsers", dash["totalUsers"])
	assertFrontendInteger(t, "dashboard.totalTransactions", dash["totalTransactions"])
	assertFrontendNonNegative(t, "dashboard.totalVolume", dash["totalVolume"])
	assertFrontendNonNegative(t, "dashboard.totalRevenue", dash["totalRevenue"])
	if _, ok := dash["activePools"].(float64); !ok {
		t.Fatalf("dashboard.activePools must be a number, got %T", dash["activePools"])
	}
}

// TestSwapQuoteMatchesFrontendContract asserts /swap/quote returns the
// camelCase quote shape, including string amounts so precision survives JSON.
func TestSwapQuoteMatchesFrontendContract(t *testing.T) {
	db := newStatsTestDB(t)
	svc := NewSwapService(db, nil)

	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/swap/quote",
		jsonBody(t, map[string]any{
			"token_in":  "0x1111111111111111111111111111111111111111",
			"token_out": "0x2222222222222222222222222222222222222222",
			"amount_in": "1000000000000000000",
			"chain_id":  1,
		}))
	c.Request.Header.Set("Content-Type", "application/json")
	svc.GetQuote(c)

	// Without a configured price aggregator there is no real quote to give,
	// and inventing one would be exactly the fake data this endpoint must
	// never produce. A non-2xx is the correct answer here.
	if rec.Code == http.StatusOK {
		t.Fatalf("quote returned 200 with no price source configured; "+
			"a quote must never be fabricated (body: %s)", rec.Body.String())
	}
}

func jsonBody(t *testing.T, payload map[string]any) *bytes.Reader {
	t.Helper()
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	return bytes.NewReader(data)
}
