package services

// This file pins the wire format of the copy-trading leaderboard and the
// TigerBridge quote, the two shapes the frontend validators in
// `src/lib/types.ts` were written against. The captured payloads in
// `frontend/src/lib/serverContract.test.ts` were copied from this test's log
// output, so a field renamed here fails the frontend contract first.

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestCopyTraderResponseWireFormat(t *testing.T) {
	engine := NewCopyTradingEngine(nil, nil)

	status, body := callHandler(t, engine.RegisterTraderHandler, 1, http.MethodPost, "/copy-trading/traders",
		`{"address":"0x1111111111111111111111111111111111111111","name":"Alpha Whale","avatar":"https://example.com/a.png"}`,
		nil)
	if status != http.StatusOK {
		t.Fatalf("register trader: status %d body %s", status, body)
	}
	t.Logf("SERVER_COPY_TRADER = `%s`", body)

	assertKeys(t, body,
		"address", "name", "avatar", "win_rate", "total_profit", "weekly_profit",
		"monthly_profit", "total_trades", "followers", "total_volume",
		"avg_holding_time", "is_verified", "is_pro", "risk_score", "strategies",
		"last_active")

	// The leaderboard envelope must carry the trader under `traders` so the
	// frontend's listingArray helper unwraps it.
	status, body = callHandler(t, engine.GetTopTradersHandler, 1, http.MethodGet, "/copy-trading/traders", "", nil)
	if status != http.StatusOK {
		t.Fatalf("list traders: status %d body %s", status, body)
	}
	t.Logf("SERVER_COPY_TRADERS = `%s`", body)

	var envelope struct {
		Count   int      `json:"count"`
		Traders []Trader `json:"traders"`
	}
	if err := json.Unmarshal([]byte(body), &envelope); err != nil {
		t.Fatalf("leaderboard is not a traders envelope: %v (%s)", err, body)
	}
	if envelope.Count != 1 || len(envelope.Traders) != 1 {
		t.Fatalf("expected one trader, got count=%d len=%d", envelope.Count, len(envelope.Traders))
	}
	if envelope.Traders[0].Address != "0x1111111111111111111111111111111111111111" {
		t.Fatalf("trader address was not lower-cased: %s", envelope.Traders[0].Address)
	}
}

func TestBridgeQuoteResponseWireFormat(t *testing.T) {
	quote := BridgeQuote{
		SrcChain:      1,
		DstChain:      137,
		Token:         "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48",
		AmountIn:      "1000000",
		AmountOut:     "999000",
		Fee:           "1000",
		FeeUSD:        0.001,
		EstimatedTime: "15 minutes",
		Router:        "0x1234567890123456789012345678901234567890",
		PriceImpact:   0,
	}

	encoded, err := json.Marshal(quote)
	if err != nil {
		t.Fatalf("marshal quote: %v", err)
	}
	t.Logf("SERVER_BRIDGE_QUOTE = `%s`", encoded)

	assertKeys(t, string(encoded),
		"src_chain", "dst_chain", "token", "amount_in", "amount_out", "fee",
		"fee_usd", "estimated_time", "router", "price_impact")
}
