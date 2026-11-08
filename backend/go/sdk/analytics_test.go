package tigerswap

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// These tests drive the SDK analytics methods against a server that answers
// exactly like the real API: the snake_case analytics fields and the
// `{ pairs: [...] }` / `{ traders: [...] }` envelopes. They exist because the
// SDK has drifted from the API before, silently decoding a key that no longer
// existed and returning an empty slice.

func TestGetAnalyticsOverviewDecodesRealPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/analytics/overview" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"total_tvl_usd":0,"volume_24h_usd":1500,"fees_24h_usd":4.5,` +
			`"volume_7d_usd":1500,"fees_7d_usd":4.5,"total_pools":0,"active_pools":0,` +
			`"total_farms":0,"active_farms":0,"total_tokens":0,"total_chains":0,"active_chains":0,` +
			`"total_users":0,"total_trades_24h":2,"open_interest_usd":2500,"margin_borrowed_usd":0,` +
			`"nft_listings_active":0,"updated_at":"2026-11-08T03:30:30.928888113Z"}`))
	}))
	defer server.Close()

	overview, err := NewClient(server.URL, "").GetAnalyticsOverview(context.Background())
	if err != nil {
		t.Fatalf("GetAnalyticsOverview: %v", err)
	}
	if overview.Volume24hUSD != 1500 {
		t.Errorf("Volume24hUSD = %v, want 1500", overview.Volume24hUSD)
	}
	if overview.Fees24hUSD != 4.5 {
		t.Errorf("Fees24hUSD = %v, want 4.5", overview.Fees24hUSD)
	}
	if overview.OpenInterestUSD != 2500 {
		t.Errorf("OpenInterestUSD = %v, want 2500", overview.OpenInterestUSD)
	}
	if overview.TotalTrades24h != 2 {
		t.Errorf("TotalTrades24h = %v, want 2", overview.TotalTrades24h)
	}
}

func TestGetAnalyticsVolumeSendsQueryAndDecodesSeries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		if query.Get("pair") != "TGR/RUSD" || query.Get("interval") != "60" || query.Get("window") != "3600" {
			t.Errorf("unexpected query: %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"chain_id":0,"interval":60,"pair":"TGR/RUSD","series":[` +
			`{"timestamp":"2026-11-08T03:25:00Z","volume_usd":1000,"fees_usd":3,` +
			`"trade_count":1,"taker_buy_volume_usd":1000,"taker_sell_volume_usd":0}]}`))
	}))
	defer server.Close()

	series, err := NewClient(server.URL, "").GetAnalyticsVolume(context.Background(), "TGR/RUSD", 0, 60, 3600)
	if err != nil {
		t.Fatalf("GetAnalyticsVolume: %v", err)
	}
	if series.Pair != "TGR/RUSD" || series.Interval != 60 {
		t.Fatalf("unexpected series header: %+v", series)
	}
	if len(series.Series) != 1 {
		t.Fatalf("expected 1 point, got %d", len(series.Series))
	}
	if series.Series[0].VolumeUSD != 1000 || series.Series[0].TradeCount != 1 {
		t.Errorf("unexpected point: %+v", series.Series[0])
	}
	if series.Series[0].TakerBuyVolumeUSD != 1000 {
		t.Errorf("TakerBuyVolumeUSD = %v, want 1000", series.Series[0].TakerBuyVolumeUSD)
	}
}

func TestGetAnalyticsPairsDecodesEnvelope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"pairs":[{"pair":"TGR/RUSD","chain_id":1,"volume_usd":1000,` +
			`"fees_usd":3,"trade_count":1,"taker_buy_volume_usd":1000,"taker_sell_volume_usd":0},` +
			`{"pair":"ETH/RUSD","chain_id":1,"volume_usd":500,"fees_usd":1.5,"trade_count":1,` +
			`"taker_buy_volume_usd":0,"taker_sell_volume_usd":500}],"window":3600}`))
	}))
	defer server.Close()

	pairs, err := NewClient(server.URL, "").GetAnalyticsPairs(context.Background(), 0, 3600, 10)
	if err != nil {
		t.Fatalf("GetAnalyticsPairs: %v", err)
	}
	if len(pairs) != 2 {
		t.Fatalf("expected 2 pairs, got %d", len(pairs))
	}
	if pairs[0].Pair != "TGR/RUSD" || pairs[1].Pair != "ETH/RUSD" {
		t.Errorf("unexpected pairs: %+v", pairs)
	}
	if pairs[1].TakerSellVolumeUSD != 500 {
		t.Errorf("TakerSellVolumeUSD = %v, want 500", pairs[1].TakerSellVolumeUSD)
	}
}

func TestGetAnalyticsTradersDecodesEnvelope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"traders":[{"user_id":"u1","realized_pnl":120,"volume_usd":2500,` +
			`"closed_trades":1,"win_rate":1,"liquidations":0}]}`))
	}))
	defer server.Close()

	traders, err := NewClient(server.URL, "").GetAnalyticsTraders(context.Background(), 0, 10)
	if err != nil {
		t.Fatalf("GetAnalyticsTraders: %v", err)
	}
	if len(traders) != 1 {
		t.Fatalf("expected 1 trader, got %d", len(traders))
	}
	if traders[0].UserID != "u1" || traders[0].RealizedPnL != 120 {
		t.Errorf("unexpected trader: %+v", traders[0])
	}
	if traders[0].WinRate != 1 {
		t.Errorf("WinRate = %v, want 1", traders[0].WinRate)
	}
}

// A non-200 must surface as an error, not a zero-valued result that reads as
// "no data" to the caller.
func TestAnalyticsReadsRejectNonOKStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"boom"}`, http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	if _, err := client.GetAnalyticsOverview(context.Background()); err == nil {
		t.Error("GetAnalyticsOverview should fail on 500")
	}
	if _, err := client.GetAnalyticsVolume(context.Background(), "", 0, 0, 0); err == nil {
		t.Error("GetAnalyticsVolume should fail on 500")
	}
	if _, err := client.GetAnalyticsPairs(context.Background(), 0, 0, 0); err == nil {
		t.Error("GetAnalyticsPairs should fail on 500")
	}
	if _, err := client.GetAnalyticsTraders(context.Background(), 0, 0); err == nil {
		t.Error("GetAnalyticsTraders should fail on 500")
	}
}
