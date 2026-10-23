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

// testUpstreams stands up real HTTP servers that back the gateway handlers.
type testUpstreams struct {
	blockchain  *httptest.Server
	priceOracle *httptest.Server
	wallet      *httptest.Server
	aggregator  *httptest.Server
	cfg         *config.Config
}

func newTestUpstreams() *testUpstreams {
	jsonHandler := func(body string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(body))
		}
	}

	bc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/v1/tokens/search"):
			_, _ = w.Write([]byte(`[{"address":"0x1","symbol":"TGR","name":"TigerSwap","decimals":18,"chainId":1}]`))
		case strings.HasPrefix(r.URL.Path, "/api/v1/tokens/"):
			_, _ = w.Write([]byte(`{"address":"0x1","symbol":"TGR","name":"TigerSwap","decimals":18,"chainId":1}`))
		case r.URL.Path == "/api/v1/tokens":
			_, _ = w.Write([]byte(`[{"address":"0x1","symbol":"TGR","name":"TigerSwap","decimals":18,"chainId":1}]`))
		case r.URL.Path == "/api/v1/chains":
			_, _ = w.Write([]byte(`[{"chainId":1,"name":"Ethereum","symbol":"ETH"}]`))
		case strings.HasPrefix(r.URL.Path, "/api/v1/chains/"):
			_, _ = w.Write([]byte(`{"chainId":1,"name":"Ethereum","symbol":"ETH"}`))
		case r.URL.Path == "/api/v1/pools":
			_, _ = w.Write([]byte(`[{"token0":"0xa","token1":"0xb","fee":30}]`))
		default:
			_, _ = w.Write([]byte(`{}`))
		}
	}))

	po := httptest.NewServer(jsonHandler(`{"ETH":{"usd":"3000.00"}}`))
	wl := httptest.NewServer(jsonHandler(`{"address":"0x1","balance":"100"}`))
	ag := httptest.NewServer(jsonHandler(`{"amount_out":"990000000000000000"}`))

	cfg := &config.Config{}
	cfg.Upstream = config.UpstreamConfig{
		Blockchain:    bc.URL,
		PriceOracle:   po.URL,
		Wallet:        wl.URL,
		DEXAggregator: ag.URL,
	}

	return &testUpstreams{blockchain: bc, priceOracle: po, wallet: wl, aggregator: ag, cfg: cfg}
}

func (u *testUpstreams) close() {
	u.blockchain.Close()
	u.priceOracle.Close()
	u.wallet.Close()
	u.aggregator.Close()
}

func TestHealthCheck(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/health", HealthCheck())

	req, _ := http.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if response["status"] != "healthy" {
		t.Errorf("expected healthy, got %v", response["status"])
	}
}

func TestWalletRoundTrip(t *testing.T) {
	gin.SetMode(gin.TestMode)
	up := newTestUpstreams()
	defer up.close()

	r := gin.New()
	handler := NewWalletHandler(up.cfg)
	r.POST("/wallet/create", handler.CreateWallet)
	r.POST("/wallet/import", handler.ImportWallet)

	req, _ := http.NewRequest("POST", "/wallet/create", strings.NewReader(`{"chainId":1}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("create wallet: expected 200, got %d (%s)", w.Code, w.Body.String())
	}

	var created struct {
		Address    string `json:"address"`
		PrivateKey string `json:"privateKey"`
		Mnemonic   string `json:"mnemonic"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(created.Address) != 42 || created.Mnemonic == "" || len(created.PrivateKey) != 64 {
		t.Fatalf("wallet create returned incomplete data: %+v", created)
	}

	importBody, _ := json.Marshal(map[string]interface{}{"mnemonic": created.Mnemonic, "chainId": 1})
	req, _ = http.NewRequest("POST", "/wallet/import", strings.NewReader(string(importBody)))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("import wallet: expected 200, got %d (%s)", w.Code, w.Body.String())
	}
	var imported struct {
		Address string `json:"address"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &imported); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if imported.Address != created.Address {
		t.Errorf("imported address %s does not match created address %s", imported.Address, created.Address)
	}
}

// TestMnemonicDerivationVector checks BIP39/BIP44 derivation against the
// canonical "abandon ... about" test vector.
func TestMnemonicDerivationVector(t *testing.T) {
	mnemonic := "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"
	addr, err := deriveAddressFromMnemonic(mnemonic)
	if err != nil {
		t.Fatalf("derive: %v", err)
	}
	const want = "0x9858EfFD232B4033E47d90003D41EC34EcaEda94"
	if !strings.EqualFold(addr, want) {
		t.Errorf("derived %s, want %s", addr, want)
	}
}

func TestImportWalletRejectsInvalidMnemonic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewWalletHandler(&config.Config{})
	r := gin.New()
	r.POST("/wallet/import", handler.ImportWallet)

	req, _ := http.NewRequest("POST", "/wallet/import", strings.NewReader(`{"mnemonic":"not a valid mnemonic phrase"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid mnemonic, got %d", w.Code)
	}
}

func TestGetTokensProxiesUpstream(t *testing.T) {
	gin.SetMode(gin.TestMode)
	up := newTestUpstreams()
	defer up.close()

	r := gin.New()
	r.GET("/tokens", NewTokenHandler(up.cfg).GetTokens)

	req, _ := http.NewRequest("GET", "/tokens", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var tokens []Token
	if err := json.Unmarshal(w.Body.Bytes(), &tokens); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(tokens) != 1 || tokens[0].Symbol != "TGR" {
		t.Errorf("unexpected tokens: %+v", tokens)
	}
}

func TestGetTokensReportsUpstreamFailure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{}
	cfg.Upstream = config.UpstreamConfig{Blockchain: "http://127.0.0.1:1"}

	r := gin.New()
	r.GET("/tokens", NewTokenHandler(cfg).GetTokens)

	req, _ := http.NewRequest("GET", "/tokens", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadGateway {
		t.Errorf("expected 502 when upstream is down, got %d", w.Code)
	}
}

func TestGetChainsProxiesUpstream(t *testing.T) {
	gin.SetMode(gin.TestMode)
	up := newTestUpstreams()
	defer up.close()

	r := gin.New()
	r.GET("/chains", NewChainHandler(up.cfg).GetChains)

	req, _ := http.NewRequest("GET", "/chains", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var chains []Chain
	if err := json.Unmarshal(w.Body.Bytes(), &chains); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(chains) != 1 || chains[0].ChainID != 1 {
		t.Errorf("unexpected chains: %+v", chains)
	}
}

func TestGetBalanceValidatesAddress(t *testing.T) {
	gin.SetMode(gin.TestMode)
	up := newTestUpstreams()
	defer up.close()

	r := gin.New()
	r.GET("/wallet/:address/balance", NewWalletHandler(up.cfg).GetBalance)

	req, _ := http.NewRequest("GET", "/wallet/not-an-address/balance", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid address, got %d", w.Code)
	}
}

func BenchmarkHealthCheck(b *testing.B) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/health", HealthCheck())

	req, _ := http.NewRequest("GET", "/health", nil)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}
