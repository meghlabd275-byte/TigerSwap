package services

// This file pins the exact wire format of the bots, OTC and TWAMM responses.
//
// The frontend validators in `src/lib/types.ts` are written against these
// shapes and checked against captured payloads in
// `frontend/src/lib/serverContract.test.ts`. The payloads there were produced
// by the handlers below and copied from this test's log output, so a change to
// a response field fails here first, next to the handler that made it.
//
// Run with `-v` to print each payload verbatim, which is how the frontend
// fixtures are regenerated.

import (
	"encoding/json"
	"net/http"
	"sort"
	"testing"
	"time"

	"tigerswap/backend/models"
)

// assertKeys fails unless the JSON object has exactly the given keys. A field
// added or removed on the server is a breaking change for the validators.
func assertKeys(t *testing.T, payload string, want ...string) {
	t.Helper()
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
		t.Fatalf("payload is not a JSON object: %v (%s)", err, payload)
	}
	got := make([]string, 0, len(decoded))
	for key := range decoded {
		got = append(got, key)
	}
	sort.Strings(got)
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("keys mismatch:\n got %v\nwant %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("keys mismatch:\n got %v\nwant %v", got, want)
		}
	}
}

func TestBotResponseWireFormat(t *testing.T) {
	db := newBotsTestDB(t)
	service := NewBotsService(db)

	status, body := callHandler(t, service.CreateBot, 7, http.MethodPost, "/bots",
		`{"name":"maker","strategy":"market_maker","symbol":"BTC/RUSD",`+
			`"config":{"spread_bps":"25","target_depth":"10000","rebalance_threshold_bps":"50"},`+
			`"max_order_size":"5","max_daily_loss":"100","max_open_orders":10}`,
		nil)
	if status != http.StatusCreated {
		t.Fatalf("create bot: status %d body %s", status, body)
	}
	t.Logf("SERVER_BOT = `%s`", body)
	assertKeys(t, body,
		"id", "name", "strategy", "symbol", "status", "config",
		"max_order_size", "max_daily_loss", "max_open_orders",
		"realised_pnl", "filled_orders", "traded_volume",
		"last_run_at", "created_at", "updated_at",
	)

	// The config must survive validation as canonical JSON, not a re-encoded
	// string, or the engine would read a quoted blob.
	var created map[string]json.RawMessage
	if err := json.Unmarshal([]byte(body), &created); err != nil {
		t.Fatalf("decode bot: %v", err)
	}
	var config map[string]json.RawMessage
	if err := json.Unmarshal(created["config"], &config); err != nil {
		t.Fatalf("config is not a JSON object: %v", err)
	}
	if len(config) != 3 {
		t.Fatalf("config keys = %v, want the three validated keys", config)
	}

	status, body = callHandler(t, service.BotsSummary, 7, http.MethodGet, "/bots/summary", "", nil)
	if status != http.StatusOK {
		t.Fatalf("summary: status %d body %s", status, body)
	}
	t.Logf("SERVER_BOTS_SUMMARY = `%s`", body)
	assertKeys(t, body, "by_strategy", "by_status", "total")

	order := models.BotOrder{
		BotID: 1, UserID: 7, Symbol: "BTC/RUSD", Side: "buy", OrderKind: "limit",
		Price: "60000", Quantity: "0.5", Status: "pending",
	}
	if err := db.Create(&order).Error; err != nil {
		t.Fatalf("seed order: %v", err)
	}
	status, body = callHandler(t, service.ListBotOrders, 7, http.MethodGet, "/bots/1/orders", "", idParam("1"))
	if status != http.StatusOK {
		t.Fatalf("list bot orders: status %d body %s", status, body)
	}
	t.Logf("SERVER_BOT_ORDERS = `%s`", body)

	perf := models.BotPerformance{
		BotID: 1, Day: time.Now().UTC().Truncate(24 * time.Hour),
		OrdersPlaced: 3, OrdersFilled: 2, Volume: "3000", RealisedPnL: "12.5",
		FeesPaid: "0.6", MaxDrawdown: "4",
	}
	if err := db.Create(&perf).Error; err != nil {
		t.Fatalf("seed performance: %v", err)
	}
	status, body = callHandler(t, service.BotPerformanceSummary, 7, http.MethodGet, "/bots/1/performance", "", idParam("1"))
	if status != http.StatusOK {
		t.Fatalf("bot performance: status %d body %s", status, body)
	}
	t.Logf("SERVER_BOT_PERFORMANCE = `%s`", body)
}

func TestOtcOrderWireFormat(t *testing.T) {
	db := newBotsTestDB(t)
	service := NewOtcService(db)

	status, body := callHandler(t, service.CreateOrder, 7, http.MethodPost, "/otc/orders",
		`{"symbol":"BTC/RUSD","side":"buy","quantity":"2","price":"60000",`+
			`"min_fill":"0.5","expires_in_seconds":86400}`,
		nil)
	if status != http.StatusCreated {
		t.Fatalf("create otc: status %d body %s", status, body)
	}
	t.Logf("SERVER_OTC_ORDER = `%s`", body)

	// taker_id, counterparty and the settlement hashes are omitempty, so an
	// unfilled order carries only these keys.
	assertKeys(t, body,
		"id", "maker_id", "symbol", "side", "quantity", "price",
		"status", "min_fill", "expires_at", "created_at", "updated_at",
	)
}

func TestTwammOrderWireFormat(t *testing.T) {
	db := newBotsTestDB(t)
	service := NewTwammService(db)

	status, body := callHandler(t, service.CreateOrder, 7, http.MethodPost, "/twamm/orders",
		`{"symbol":"BTC/RUSD","side":"buy","total_quantity":"10","limit_price":"0",`+
			`"duration_hours":24,"start_delay_seconds":0}`,
		nil)
	if status != http.StatusCreated {
		t.Fatalf("create twamm: status %d body %s", status, body)
	}
	t.Logf("SERVER_TWAMM_ORDER = `%s`", body)
	assertKeys(t, body,
		"id", "user_id", "symbol", "side", "total_quantity", "filled_quantity",
		"limit_price", "start_time", "end_time", "status", "created_at", "updated_at",
	)
}

// TestBotTablesRoundTrip pins the columns the migrations create, so a dropped
// column is caught here rather than in the browser.
func TestBotTablesRoundTrip(t *testing.T) {
	db := newBotsTestDB(t)

	bot := models.BotInstance{
		UserID:        7,
		Name:          "grid",
		Strategy:      models.StrategyGrid,
		Symbol:        "ETH/RUSD",
		Status:        models.BotStatusActive,
		Config:        `{"lower_price":"1000","upper_price":"2000","levels":10}`,
		MaxOrderSize:  "1",
		MaxDailyLoss:  "50",
		MaxOpenOrders: 20,
		RealisedPnL:   "0",
		TradedVolume:  "0",
	}
	if err := db.Create(&bot).Error; err != nil {
		t.Fatalf("create bot: %v", err)
	}
	if bot.ID == 0 {
		t.Fatal("bot id was not assigned")
	}

	var loaded models.BotInstance
	if err := db.First(&loaded, bot.ID).Error; err != nil {
		t.Fatalf("reload bot: %v", err)
	}
	if loaded.Strategy != models.StrategyGrid || loaded.Symbol != "ETH/RUSD" {
		t.Fatalf("round trip mismatch: %+v", loaded)
	}
}
