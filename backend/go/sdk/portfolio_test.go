package tigerswap

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// These tests drive the SDK portfolio methods against a server that answers
// exactly like the real API. They exist because the SDK has drifted from the
// API before, silently decoding a key that no longer existed and returning an
// empty or zeroed value.

func TestGetPortfolioDecodesRealPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/user/portfolio" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer session-token" {
			t.Errorf("Authorization = %q, want the session bearer", auth)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"totalValueUSD":4000,"assets":[` +
			`{"chainId":1,"tokenAddress":"0x0000000000000000000000000000000000000001",` +
			`"symbol":"ETH","decimals":18,"balance":"2000000000000000000","valueUSD":4000}],` +
			`"realizedPnlUsd":-40.25,"unrealizedPnlUsd":150.5,"openPositions":1,` +
			`"updatedAt":"2026-11-08T12:20:36.201522492Z"}`))
	}))
	defer server.Close()

	portfolio, err := NewClient(server.URL, "session-token").GetPortfolio(context.Background())
	if err != nil {
		t.Fatalf("GetPortfolio: %v", err)
	}
	if portfolio.TotalValueUSD != 4000 {
		t.Errorf("TotalValueUSD = %v, want 4000", portfolio.TotalValueUSD)
	}
	if len(portfolio.Assets) != 1 {
		t.Fatalf("assets = %d, want 1", len(portfolio.Assets))
	}
	if portfolio.Assets[0].Symbol != "ETH" || portfolio.Assets[0].ValueUSD == nil || *portfolio.Assets[0].ValueUSD != 4000 {
		t.Errorf("asset = %+v, want ETH valued at 4000", portfolio.Assets[0])
	}
	if portfolio.RealizedPnLUSD != -40.25 {
		t.Errorf("RealizedPnLUSD = %v, want -40.25", portfolio.RealizedPnLUSD)
	}
	if portfolio.UnrealizedPnLUSD != 150.5 {
		t.Errorf("UnrealizedPnLUSD = %v, want 150.5", portfolio.UnrealizedPnLUSD)
	}
	if portfolio.OpenPositions != 1 {
		t.Errorf("OpenPositions = %v, want 1", portfolio.OpenPositions)
	}
}

func TestGetPortfolioKeepsUnpricedAssetNull(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"totalValueUSD":0,"assets":[` +
			`{"chainId":1,"tokenAddress":"0x0000000000000000000000000000000000000009",` +
			`"symbol":"MYSTERY","decimals":18,"balance":"5000000000000000000","valueUSD":null}],` +
			`"realizedPnlUsd":0,"unrealizedPnlUsd":0,"openPositions":0,"updatedAt":"2026-11-08T12:20:36Z"}`))
	}))
	defer server.Close()

	portfolio, err := NewClient(server.URL, "").GetPortfolio(context.Background())
	if err != nil {
		t.Fatalf("GetPortfolio: %v", err)
	}
	if portfolio.Assets[0].ValueUSD != nil {
		t.Fatalf("unpriced asset should decode to nil, got %v", *portfolio.Assets[0].ValueUSD)
	}
}

func TestGetTransactionsDecodesArrayPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/user/transactions" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"hash":"0xdeadbeef","chainId":1,"type":"swap",` +
			`"status":"confirmed","amount":"1000000000000000000","tokenSymbol":"ETH",` +
			`"createdAt":"2026-11-08T12:20:36.201140162Z"}]`))
	}))
	defer server.Close()

	txs, err := NewClient(server.URL, "").GetTransactions(context.Background())
	if err != nil {
		t.Fatalf("GetTransactions: %v", err)
	}
	if len(txs) != 1 {
		t.Fatalf("transactions = %d, want 1", len(txs))
	}
	if txs[0].Hash != "0xdeadbeef" || txs[0].TokenSymbol != "ETH" || txs[0].Status != "confirmed" {
		t.Errorf("transaction = %+v", txs[0])
	}
}
