package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"tigerswap/backend/go/order_matching/orderbook"
)

func newTestRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	engine = orderbook.NewEngine()
	if err := engine.RegisterPair(orderbook.PairConfig{Pair: "BTC/RUSD", TakerFeeBps: 2}); err != nil {
		t.Fatalf("register pair: %v", err)
	}
	cfg = Config{AdminKey: "test-admin-key", RateLimit: 10000}

	r := gin.New()
	api := r.Group("/api/v1")
	{
		api.POST("/orders", submitOrderHandler)
		api.DELETE("/orders/:id", cancelOrderHandler)
		api.GET("/orders/:id", getOrderHandler)
		api.GET("/orders/trader/:address", getTraderOrdersHandler)
		api.GET("/market", getMarketDataHandler)
		api.GET("/market/depth", getDepthHandler)
		api.GET("/market/trades", getTradesHandler)
		api.GET("/pairs", listPairsHandler)
		api.GET("/stats", getStatsHandler)
		api.GET("/health", healthHandler)

		admin := api.Group("/admin", adminAuth)
		{
			admin.POST("/pairs", registerPairHandler)
			admin.DELETE("/pairs", unregisterPairHandler)
		}
	}
	return r
}

func doJSON(t *testing.T, r *gin.Engine, method, path string, body any, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// TestSubmitAndFillThroughAPI proves an order placed over HTTP rests, a
// crossing order fills it, and the tape reflects the trade.
func TestSubmitAndFillThroughAPI(t *testing.T) {
	r := newTestRouter(t)

	w := doJSON(t, r, http.MethodPost, "/api/v1/orders", map[string]any{
		"trader": "maker", "pair": "BTC/RUSD", "side": "sell", "type": "limit",
		"price": 60000, "quantity": 1,
	}, nil)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var placed struct {
		Order struct {
			OrderID uint64  `json:"order_id"`
			Status  string  `json:"status"`
			Leaves  float64 `json:"leaves"`
		} `json:"order"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &placed); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if placed.Order.Status != "open" || placed.Order.Leaves != 1 {
		t.Fatalf("unexpected resting order: %+v", placed.Order)
	}

	// Crossing buy fills it.
	w = doJSON(t, r, http.MethodPost, "/api/v1/orders", map[string]any{
		"trader": "taker", "pair": "BTC/RUSD", "side": "buy", "type": "limit",
		"price": 61000, "quantity": 1,
	}, nil)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var filled struct {
		Order struct {
			Status string `json:"status"`
		} `json:"order"`
		Trades []struct {
			Price    float64 `json:"price"`
			Quantity float64 `json:"quantity"`
		} `json:"trades"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &filled); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if filled.Order.Status != "filled" {
		t.Fatalf("expected filled, got %q", filled.Order.Status)
	}
	if len(filled.Trades) != 1 || filled.Trades[0].Price != 60000 {
		t.Fatalf("expected one trade at 60000, got %+v", filled.Trades)
	}

	// The tape now reports the trade.
	w = doJSON(t, r, http.MethodGet, "/api/v1/market/trades?pair=BTC/RUSD", nil, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var tape struct {
		Count int `json:"count"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &tape); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if tape.Count != 1 {
		t.Fatalf("expected 1 trade on the tape, got %d", tape.Count)
	}
}

// TestUnlistedPairReturns404 proves the API refuses a pair that is not listed.
func TestUnlistedPairReturns404(t *testing.T) {
	r := newTestRouter(t)

	w := doJSON(t, r, http.MethodPost, "/api/v1/orders", map[string]any{
		"trader": "a", "pair": "DOGE/RUSD", "side": "buy", "type": "limit",
		"price": 1, "quantity": 1,
	}, nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// TestCancelRequiresTraderAndOwnership proves the API requires a trader and
// that a different trader cannot cancel someone else's order.
func TestCancelRequiresTraderAndOwnership(t *testing.T) {
	r := newTestRouter(t)

	w := doJSON(t, r, http.MethodPost, "/api/v1/orders", map[string]any{
		"trader": "alice", "pair": "BTC/RUSD", "side": "buy", "type": "limit",
		"price": 60000, "quantity": 1,
	}, nil)
	var placed struct {
		Order struct {
			OrderID uint64 `json:"order_id"`
		} `json:"order"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &placed); err != nil {
		t.Fatalf("decode: %v", err)
	}
	path := "/api/v1/orders/" + itoa(placed.Order.OrderID)

	// No trader supplied.
	if w := doJSON(t, r, http.MethodDelete, path, map[string]any{}, nil); w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 without a trader, got %d", w.Code)
	}

	// Wrong trader.
	if w := doJSON(t, r, http.MethodDelete, path, map[string]any{"trader": "mallory"}, nil); w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for a non-owner, got %d: %s", w.Code, w.Body.String())
	}

	// Owner succeeds.
	if w := doJSON(t, r, http.MethodDelete, path, map[string]any{"trader": "alice"}, nil); w.Code != http.StatusOK {
		t.Fatalf("expected 200 for the owner, got %d: %s", w.Code, w.Body.String())
	}
}

// TestAdminPairManagementRequiresKey proves the admin surface is protected and
// that a listed pair can be added and removed.
func TestAdminPairManagementRequiresKey(t *testing.T) {
	r := newTestRouter(t)

	body := map[string]any{"pair": "ETH/RUSD", "maker_fee_bps": 0, "taker_fee_bps": 2}

	if w := doJSON(t, r, http.MethodPost, "/api/v1/admin/pairs", body, nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without a key, got %d", w.Code)
	}
	if w := doJSON(t, r, http.MethodPost, "/api/v1/admin/pairs", body, map[string]string{"X-Admin-Key": "wrong"}); w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 with a wrong key, got %d", w.Code)
	}

	w := doJSON(t, r, http.MethodPost, "/api/v1/admin/pairs", body, map[string]string{"X-Admin-Key": "test-admin-key"})
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// The new pair is now tradable.
	if w := doJSON(t, r, http.MethodPost, "/api/v1/orders", map[string]any{
		"trader": "a", "pair": "ETH/RUSD", "side": "buy", "type": "limit", "price": 2000, "quantity": 1,
	}, nil); w.Code != http.StatusCreated {
		t.Fatalf("expected the new pair to accept orders, got %d: %s", w.Code, w.Body.String())
	}

	if w := doJSON(t, r, http.MethodDelete, "/api/v1/admin/pairs?pair=ETH/RUSD", nil, map[string]string{"X-Admin-Key": "test-admin-key"}); w.Code != http.StatusOK {
		t.Fatalf("expected 200 removing the pair, got %d: %s", w.Code, w.Body.String())
	}
}

// TestAdminDisabledWithoutKey proves the admin surface is closed, not open,
// when no key is configured.
func TestAdminDisabledWithoutKey(t *testing.T) {
	r := newTestRouter(t)
	cfg = Config{} // no admin key

	w := doJSON(t, r, http.MethodPost, "/api/v1/admin/pairs", map[string]any{"pair": "X/RUSD"}, nil)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when admin is disabled, got %d", w.Code)
	}
}

// TestStatsReflectRealActivity proves the stats endpoint reports counters that
// come from actual orders rather than constants.
func TestStatsReflectRealActivity(t *testing.T) {
	r := newTestRouter(t)

	doJSON(t, r, http.MethodPost, "/api/v1/orders", map[string]any{
		"trader": "maker", "pair": "BTC/RUSD", "side": "sell", "type": "limit", "price": 60000, "quantity": 1,
	}, nil)
	doJSON(t, r, http.MethodPost, "/api/v1/orders", map[string]any{
		"trader": "taker", "pair": "BTC/RUSD", "side": "buy", "type": "limit", "price": 60000, "quantity": 1,
	}, nil)

	w := doJSON(t, r, http.MethodGet, "/api/v1/stats", nil, nil)
	var stats struct {
		Submitted   uint64 `json:"total_orders"`
		Trades      uint64 `json:"total_trades"`
		ListedPairs int    `json:"listed_pairs"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &stats); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if stats.Submitted != 2 || stats.Trades != 1 || stats.ListedPairs != 1 {
		t.Fatalf("stats do not reflect activity: %+v", stats)
	}
}

// TestRejectsBadInputThroughAPI proves malformed orders are refused with 400.
func TestRejectsBadInputThroughAPI(t *testing.T) {
	r := newTestRouter(t)

	cases := []map[string]any{
		{"trader": "a", "pair": "BTC/RUSD", "side": "sideways", "type": "limit", "price": 1, "quantity": 1},
		{"trader": "a", "pair": "BTC/RUSD", "side": "buy", "type": "limit", "price": 0, "quantity": 1},
		{"trader": "a", "pair": "BTC/RUSD", "side": "buy", "type": "limit", "price": 1, "quantity": 0},
		{"pair": "BTC/RUSD", "side": "buy", "type": "limit", "price": 1, "quantity": 1}, // no trader
	}
	for i, body := range cases {
		if w := doJSON(t, r, http.MethodPost, "/api/v1/orders", body, nil); w.Code != http.StatusBadRequest {
			t.Fatalf("case %d: expected 400, got %d: %s", i, w.Code, w.Body.String())
		}
	}
}

// TestLoadPairSpecsRejectsBadConfig proves the pair registry fails closed.
func TestLoadPairSpecsRejectsBadConfig(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{"empty", ""},
		{"not json", "not json"},
		{"empty array", "[]"},
		{"no pair name", `[{"maker_fee_bps":1}]`},
		{"duplicate", `[{"pair":"A/B"},{"pair":"A/B"}]`},
		{"negative fee", `[{"pair":"A/B","taker_fee_bps":-1}]`},
		{"bad range", `[{"pair":"A/B","min_quantity":10,"max_quantity":5}]`},
		{"unknown field", `[{"pair":"A/B","bogus":1}]`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := loadPairSpecs(tc.raw); err == nil {
				t.Fatalf("expected %q to be rejected", tc.raw)
			}
		})
	}

	specs, err := loadPairSpecs(`[{"pair":"BTC/RUSD","taker_fee_bps":2},{"pair":"ETH/RUSD"}]`)
	if err != nil {
		t.Fatalf("expected a valid list to parse: %v", err)
	}
	if len(specs) != 2 {
		t.Fatalf("expected 2 specs, got %d", len(specs))
	}
}

// TestRateLimiterBlocksBurst proves the limiter refuses traffic past its limit.
func TestRateLimiterBlocksBurst(t *testing.T) {
	rl := NewRateLimiter(3, time.Second)
	defer rl.Stop()

	for i := 0; i < 3; i++ {
		if !rl.Allow("client") {
			t.Fatalf("request %d should be allowed", i)
		}
	}
	if rl.Allow("client") {
		t.Fatal("the fourth request should be blocked")
	}
	// A different client is unaffected.
	if !rl.Allow("other") {
		t.Fatal("a different client should be allowed")
	}
}

// TestSameHostMatching verifies the WebSocket origin fallback.
func TestSameHostMatching(t *testing.T) {
	if !sameHost("https://app.tigerswap.io", "app.tigerswap.io") {
		t.Fatal("expected the same host to match")
	}
	if !sameHost("http://localhost:3000", "localhost:3000") {
		t.Fatal("expected the same host with a port to match")
	}
	if sameHost("https://evil.example", "app.tigerswap.io") {
		t.Fatal("a different host must not match")
	}
}

func itoa(v uint64) string {
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}
