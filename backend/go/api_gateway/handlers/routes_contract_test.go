package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"tigerswap/backend/go/api_gateway/config"
)

// newAggregatorUpstream stands up an HTTP server that answers exactly like
// TigerAggregator: amounts are JSON numbers (Go encodes *big.Int that way) and
// the field names are snake_case. A stub that returned strings would hide the
// decoding bug these tests exist to catch.
func newAggregatorUpstream(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/quote":
			_, _ = w.Write([]byte(`{
                                "from_token":"0x1111111111111111111111111111111111111111",
                                "to_token":"0x2222222222222222222222222222222222222222",
                                "amount_in":1000000000000000000,
                                "amount_out":2000000000000000000,
                                "amount_out_min":1990000000000000000,
                                "price_impact":0.12,
                                "gas_estimate":150000,
                                "routes":[{"family":"v3","path":["0x1111111111111111111111111111111111111111","0x2222222222222222222222222222222222222222"],"amount_in":1000000000000000000,"amount_out":2000000000000000000,"fee_bps":5,"gas_estimate":150000}],
                                "best_route":{"family":"v3","path":["0x1111111111111111111111111111111111111111","0x2222222222222222222222222222222222222222"],"amount_in":1000000000000000000,"amount_out":2000000000000000000,"fee_bps":5,"gas_estimate":150000}
                        }`))
		case "/api/v1/quote/multihop":
			_, _ = w.Write([]byte(`{
                                "from_token":"0x1111111111111111111111111111111111111111",
                                "to_token":"0x2222222222222222222222222222222222222222",
                                "amount_in":1000000000000000000,
                                "amount_out":2000000000000000000,
                                "routes":[
                                        {"family":"v2","path":["0x1111111111111111111111111111111111111111","0x2222222222222222222222222222222222222222"],"amount_in":1000000000000000000,"amount_out":1990000000000000000,"fee_bps":30,"gas_estimate":120000},
                                        {"family":"v3","path":["0x1111111111111111111111111111111111111111","0x3333333333333333333333333333333333333333","0x2222222222222222222222222222222222222222"],"amount_in":1000000000000000000,"amount_out":2000000000000000000,"fee_bps":5,"gas_estimate":180000}
                                ],
                                "best_route":{"family":"v3","path":["0x1111111111111111111111111111111111111111","0x3333333333333333333333333333333333333333","0x2222222222222222222222222222222222222222"],"amount_in":1000000000000000000,"amount_out":2000000000000000000,"fee_bps":5,"gas_estimate":180000}
                        }`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func gatewayCfg(t *testing.T, aggregatorURL string) *config.Config {
	t.Helper()
	cfg := &config.Config{}
	cfg.Upstream = config.UpstreamConfig{DEXAggregator: aggregatorURL}
	return cfg
}

// TestQuoteDecodesAggregatorBigIntAmounts is the regression test for the
// decoding bug: the aggregator emits amounts as JSON numbers, so a string field
// fails to unmarshal and every quote returned a 500.
func TestQuoteDecodesAggregatorBigIntAmounts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ag := newAggregatorUpstream(t)
	defer ag.Close()

	r := gin.New()
	r.POST("/swap/quote", NewSwapHandler(gatewayCfg(t, ag.URL)).GetQuote)

	req, _ := http.NewRequest("POST", "/swap/quote", strings.NewReader(`{
                "chain_id":1,
                "from_token":"0x1111111111111111111111111111111111111111",
                "to_token":"0x2222222222222222222222222222222222222222",
                "amount_in":"1000000000000000000",
                "slippage_bps":50
        }`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", w.Code, w.Body.String())
	}

	var quote QuoteResponse
	if err := json.Unmarshal(w.Body.Bytes(), &quote); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if quote.AmountIn != "1000000000000000000" {
		t.Errorf("amountIn = %q, want the exact base-unit string", quote.AmountIn)
	}
	if quote.AmountOut != "2000000000000000000" {
		t.Errorf("amountOut = %q, want the exact base-unit string", quote.AmountOut)
	}
	if quote.AmountOutMin != "1990000000000000000" {
		t.Errorf("amountOutMin = %q, want the exact base-unit string", quote.AmountOutMin)
	}
	if quote.PriceImpact != 0.12 {
		t.Errorf("priceImpact = %v, want 0.12", quote.PriceImpact)
	}
	if quote.GasEstimate != 150000 {
		t.Errorf("gasEstimate = %v, want 150000", quote.GasEstimate)
	}
	if len(quote.Route) != 2 {
		t.Errorf("route = %v, want the best route's 2-hop path", quote.Route)
	}
}

// TestQuoteRejectsNonPositiveAmount checks the amount is validated before any
// upstream call, so a malformed request cannot reach the aggregator.
func TestQuoteRejectsNonPositiveAmount(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ag := newAggregatorUpstream(t)
	defer ag.Close()

	r := gin.New()
	r.POST("/swap/quote", NewSwapHandler(gatewayCfg(t, ag.URL)).GetQuote)

	for _, amount := range []string{"0", "-1", "abc", ""} {
		body := `{"chain_id":1,"from_token":"0x1","to_token":"0x2","amount_in":"` + amount + `"}`
		req, _ := http.NewRequest("POST", "/swap/quote", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("amount_in=%q: expected 400, got %d", amount, w.Code)
		}
	}
}

// TestGetRoutesReturnsRealAggregatorRoutes asserts the endpoint reports the
// routes the aggregator found on TigerSwap's own pools, and never names an
// external DEX.
func TestGetRoutesReturnsRealAggregatorRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ag := newAggregatorUpstream(t)
	defer ag.Close()

	r := gin.New()
	r.GET("/swap/routes", NewSwapHandler(gatewayCfg(t, ag.URL)).GetRoutes)

	req, _ := http.NewRequest("GET",
		"/swap/routes?chain_id=1&from=0x1111111111111111111111111111111111111111&to=0x2222222222222222222222222222222222222222&amount_in=1000000000000000000", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", w.Code, w.Body.String())
	}

	var body struct {
		Routes []routeInfo `json:"routes"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(body.Routes) != 2 {
		t.Fatalf("expected the aggregator's 2 routes, got %d (%s)", len(body.Routes), w.Body.String())
	}
	families := map[string]bool{}
	for _, route := range body.Routes {
		families[route.Family] = true
	}
	if !families["v2"] || !families["v3"] {
		t.Errorf("expected TigerSwap v2 and v3 routes, got %v", families)
	}

	// The response must never reference another venue.
	lower := strings.ToLower(w.Body.String())
	for _, external := range []string{"uniswap", "sushiswap", "pancake", "1inch", "curve", "balancer"} {
		if strings.Contains(lower, external) {
			t.Errorf("route response references external venue %q: %s", external, w.Body.String())
		}
	}
}

// TestGetRoutesRequiresChainID asserts a missing chain_id is rejected rather
// than silently defaulting to a chain the caller did not ask for.
func TestGetRoutesRequiresChainID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ag := newAggregatorUpstream(t)
	defer ag.Close()

	r := gin.New()
	r.GET("/swap/routes", NewSwapHandler(gatewayCfg(t, ag.URL)).GetRoutes)

	req, _ := http.NewRequest("GET", "/swap/routes?from=0x1&to=0x2", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 without chain_id, got %d", w.Code)
	}
}

// TestGetRoutesEmptyWhenNoRouteExists asserts a pair with no route yields an
// empty array, not a fabricated one.
func TestGetRoutesEmptyWhenNoRouteExists(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ag := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"routes":[]}`))
	}))
	defer ag.Close()

	r := gin.New()
	r.GET("/swap/routes", NewSwapHandler(gatewayCfg(t, ag.URL)).GetRoutes)

	req, _ := http.NewRequest("GET",
		"/swap/routes?chain_id=1&from=0x1111111111111111111111111111111111111111&to=0x2222222222222222222222222222222222222222&amount_in=1", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body struct {
		Routes []routeInfo `json:"routes"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(body.Routes) != 0 {
		t.Errorf("expected no routes, got %d", len(body.Routes))
	}
}
