package services

import (
	"encoding/json"
	"testing"
)

// TestPerpMarketViewJSONContract locks the wire format of the public
// /perpetuals/markets payload. The frontend validator reads these exact
// camelCase keys, so a rename here would silently break the client.
func TestPerpMarketViewJSONContract(t *testing.T) {
	view := PerpMarketView{
		MarketID:          "BTC-USD",
		Symbol:            "BTC-USD",
		MarkPrice:         65000,
		IndexPrice:        65000,
		FundingRate:       0.0001,
		MaxLeverage:       100,
		OpenInterestLong:  1000,
		OpenInterestShort: 250,
		IsActive:          true,
	}

	raw, err := json.Marshal(view)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	required := []string{
		"marketId", "symbol", "markPrice", "indexPrice", "fundingRate",
		"maxLeverage", "openInterestLong", "openInterestShort", "isActive",
	}
	for _, key := range required {
		if _, ok := decoded[key]; !ok {
			t.Errorf("payload is missing required key %q; got %s", key, raw)
		}
	}
	if len(decoded) != len(required) {
		t.Errorf("payload has unexpected keys: %s", raw)
	}
}

// TestPerpMarketsEnvelopeShape locks the `{ "markets": [...] }` envelope the
// frontend's list validator unwraps.
func TestPerpMarketsEnvelopeShape(t *testing.T) {
	markets := []PerpMarketView{
		{MarketID: "BTC-USD", Symbol: "BTC-USD", MaxLeverage: maxPerpLeverage},
	}

	raw, err := json.Marshal(struct {
		Markets []PerpMarketView `json:"markets"`
	}{Markets: markets})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded struct {
		Markets []map[string]interface{} `json:"markets"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(decoded.Markets) != 1 {
		t.Fatalf("expected 1 market, got %d", len(decoded.Markets))
	}

	// An unpriced market must still serialise every field, with isActive false
	// so a client never treats a stale price as tradable.
	market := decoded.Markets[0]
	if market["isActive"] != false {
		t.Errorf("expected isActive false for an unpriced market, got %v", market["isActive"])
	}
	if market["markPrice"] != float64(0) {
		t.Errorf("expected markPrice 0, got %v", market["markPrice"])
	}
	if market["maxLeverage"] != float64(maxPerpLeverage) {
		t.Errorf("expected maxLeverage %d, got %v", maxPerpLeverage, market["maxLeverage"])
	}
}
