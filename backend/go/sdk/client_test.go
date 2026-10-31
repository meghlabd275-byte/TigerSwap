package tigerswap

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// These tests drive the SDK against a server that answers exactly like the real
// TigerSwap API: bare camelCase arrays, camelCase quote fields, and the
// `amount_in` / `chainId` request spellings. They exist because the SDK and the
// API drifted apart once already, and the failure was silent - the SDK decoded
// an envelope key that no longer existed and returned an empty slice.

func TestGetChainsDecodesBareArray(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
                        {"id":1,"name":"Ethereum","chainType":"evm","nativeSymbol":"ETH","explorerURL":"https://etherscan.io","isActive":true},
                        {"id":101,"name":"Solana","chainType":"solana","nativeSymbol":"SOL","explorerURL":null,"isActive":true}
                ]`))
	}))
	defer server.Close()

	chains, err := NewClient(server.URL, "").GetChains(context.Background())
	if err != nil {
		t.Fatalf("GetChains: %v", err)
	}
	if len(chains) != 2 {
		t.Fatalf("expected 2 chains, got %d", len(chains))
	}
	if chains[0].ID != 1 || chains[0].ChainType != "evm" || chains[0].NativeSymbol != "ETH" {
		t.Errorf("chain[0] = %+v", chains[0])
	}
	if chains[1].ExplorerURL != nil {
		t.Errorf("chain[1].ExplorerURL should be nil for null, got %v", *chains[1].ExplorerURL)
	}
}

func TestGetTokensDecodesBareArray(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("chainId"); got != "1" {
			t.Errorf("expected chainId=1, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
                        {"address":"0x1","symbol":"TGR","name":"TigerSwap","decimals":18,"chainId":1,"logoURI":null,"priceUSD":null}
                ]`))
	}))
	defer server.Close()

	tokens, err := NewClient(server.URL, "").GetTokens(context.Background(), 1)
	if err != nil {
		t.Fatalf("GetTokens: %v", err)
	}
	if len(tokens) != 1 || tokens[0].Symbol != "TGR" || tokens[0].Decimals != 18 {
		t.Fatalf("unexpected tokens: %+v", tokens)
	}
	if tokens[0].PriceUSD != nil {
		t.Errorf("priceUSD should be nil for null, got %v", *tokens[0].PriceUSD)
	}
}

func TestGetPoolsDecodesBareArray(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
                        {"pairAddress":"0x3","token0":"0x1","token1":"0x2","symbol0":"TGR","symbol1":"RUSD","reserve0":1000.5,"reserve1":2000.25,"feeBps":30}
                ]`))
	}))
	defer server.Close()

	pools, err := NewClient(server.URL, "").GetPools(context.Background())
	if err != nil {
		t.Fatalf("GetPools: %v", err)
	}
	if len(pools) != 1 {
		t.Fatalf("expected 1 pool, got %d", len(pools))
	}
	if pools[0].FeeBps != 30 || pools[0].Reserve1 != 2000.25 {
		t.Errorf("pool = %+v", pools[0])
	}
}

func TestGetQuoteSendsAndDecodesContractFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
		}
		// The API binds `amount_in`; sending `amount` yields a 400.
		if _, ok := body["amount_in"]; !ok {
			t.Errorf("request is missing amount_in: %v", body)
		}
		if _, ok := body["amount"]; ok {
			t.Errorf("request still sends the legacy `amount` key: %v", body)
		}
		// chain_id is a JSON number in the contract, not a string.
		if _, ok := body["chain_id"].(float64); !ok {
			t.Errorf("chain_id must be a JSON number, got %T", body["chain_id"])
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
                        "amountIn":"1000000000000000000",
                        "amountOut":"2000000000000000000",
                        "amountOutMin":"1990000000000000000",
                        "priceImpact":0.12,
                        "route":["0x1","0x2"],
                        "gasEstimate":150000
                }`))
	}))
	defer server.Close()

	quote, err := NewClient(server.URL, "").GetQuote(context.Background(), "0x1", "0x2", "1000000000000000000", 1)
	if err != nil {
		t.Fatalf("GetQuote: %v", err)
	}
	if quote.AmountOut != "2000000000000000000" || quote.AmountOutMin != "1990000000000000000" {
		t.Errorf("quote amounts = %+v", quote)
	}
	if quote.PriceImpact != 0.12 || quote.GasEstimate != 150000 {
		t.Errorf("quote impact/gas = %+v", quote)
	}
	if len(quote.Route) != 2 {
		t.Errorf("quote route = %v", quote.Route)
	}
}

func TestGetMarketStatsDecodesCamelCase(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"totalTVL":500000.5,"volume24h":15000,"totalPairs":7,"totalUsers":42,"updatedAt":"2026-10-31T04:04:03Z"}`))
	}))
	defer server.Close()

	stats, err := NewClient(server.URL, "").GetMarketStats(context.Background())
	if err != nil {
		t.Fatalf("GetMarketStats: %v", err)
	}
	if stats.TotalTVL != 500000.5 || stats.Volume24h != 15000 {
		t.Errorf("stats = %+v", stats)
	}
	if stats.TotalPairs != 7 || stats.TotalUsers != 42 {
		t.Errorf("stats counts = %+v", stats)
	}
}
