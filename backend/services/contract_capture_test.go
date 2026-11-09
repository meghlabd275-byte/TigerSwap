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
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
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

	status, body := callHandler(t, service.CreateBot, "7", http.MethodPost, "/bots",
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

	status, body = callHandler(t, service.BotsSummary, "7", http.MethodGet, "/bots/summary", "", nil)
	if status != http.StatusOK {
		t.Fatalf("summary: status %d body %s", status, body)
	}
	t.Logf("SERVER_BOTS_SUMMARY = `%s`", body)
	assertKeys(t, body, "by_strategy", "by_status", "total")

	order := models.BotOrder{
		BotID: 1, UserID: "7", Symbol: "BTC/RUSD", Side: "buy", OrderKind: "limit",
		Price: "60000", Quantity: "0.5", Status: "pending",
	}
	if err := db.Create(&order).Error; err != nil {
		t.Fatalf("seed order: %v", err)
	}
	status, body = callHandler(t, service.ListBotOrders, "7", http.MethodGet, "/bots/1/orders", "", idParam("1"))
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
	status, body = callHandler(t, service.BotPerformanceSummary, "7", http.MethodGet, "/bots/1/performance", "", idParam("1"))
	if status != http.StatusOK {
		t.Fatalf("bot performance: status %d body %s", status, body)
	}
	t.Logf("SERVER_BOT_PERFORMANCE = `%s`", body)
}

func TestOtcOrderWireFormat(t *testing.T) {
	db := newBotsTestDB(t)
	service := NewOtcService(db)

	status, body := callHandler(t, service.CreateOrder, "7", http.MethodPost, "/otc/orders",
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

	status, body := callHandler(t, service.CreateOrder, "7", http.MethodPost, "/twamm/orders",
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
		UserID:        "7",
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

// TestOptionResponseWireFormat pins the option series, position, buy and
// exercise payloads. The frontend fixtures for the options panel are copied
// from this test's log output, so a field rename fails here first.
func TestOptionResponseWireFormat(t *testing.T) {
	env := newOptionTestEnv(t)
	expiry := time.Now().Add(30 * 24 * time.Hour)
	seriesID := seedOptionSeries(t, env, true, "2000", "2000", "0.8", expiry)

	_, listBody := callUserHandler(t, env.engine.GetActiveSeries, "", http.MethodGet,
		"/options/series", "", nil)
	t.Logf("SERVER_OPTION_SERIES = `%s`", listBody)

	_, detailBody := callUserHandler(t, env.engine.GetSeriesDetail, "", http.MethodGet,
		"/options/series/"+seriesID, "", gin.Params{{Key: "id", Value: seriesID}})
	t.Logf("SERVER_OPTION_SERIES_DETAIL = `%s`", detailBody)
	assertKeys(t, detailBody, "series", "success")

	creditCollateral(t, env, "lp-1", "1000000000000000000000000")
	callUserHandler(t, env.engine.AddCollateral, "lp-1", http.MethodPost,
		"/options/series/"+seriesID+"/collateral", `{"amount":"1000000000000000000000000"}`,
		gin.Params{{Key: "id", Value: seriesID}})

	creditCollateral(t, env, "trader-1", "10000000000000000000000")
	_, buyBody := callUserHandler(t, env.engine.Buy, "trader-1", http.MethodPost,
		"/options/series/"+seriesID+"/buy", `{"size":"1","max_premium":"1000000000000000000000"}`,
		gin.Params{{Key: "id", Value: seriesID}})
	t.Logf("SERVER_OPTION_BUY = `%s`", buyBody)
	assertKeys(t, buyBody, "premium", "size", "success")

	_, positionsBody := callUserHandler(t, env.engine.GetMyPositions, "trader-1", http.MethodGet,
		"/options/positions", "", nil)
	t.Logf("SERVER_OPTION_POSITIONS = `%s`", positionsBody)
	assertKeys(t, positionsBody, "positions", "success")

	// Settle above the strike so the call finishes in the money and the
	// exercise payload carries a real payout.
	env.db.Model(&models.OptionSeries{}).Where("id = ?", seriesID).
		Update("expiry", time.Now().Add(-time.Hour))
	callUserHandler(t, env.engine.SettleSeries, "admin-1", http.MethodPost,
		"/options/series/"+seriesID+"/settle", `{"settlement_price":"2500"}`,
		gin.Params{{Key: "id", Value: seriesID}})
	_, exerciseBody := callUserHandler(t, env.engine.Exercise, "trader-1", http.MethodPost,
		"/options/series/"+seriesID+"/exercise", "", gin.Params{{Key: "id", Value: seriesID}})
	t.Logf("SERVER_OPTION_EXERCISE = `%s`", exerciseBody)
	assertKeys(t, exerciseBody, "payout", "success")
}

// TestNFTResponseWireFormat pins the NFT marketplace payloads. The frontend
// fixtures for the NFT panel are copied from this test's log output, so a field
// rename fails here first.
func TestNFTResponseWireFormat(t *testing.T) {
	env := newNFTTestEnv(t)
	collectionID := nftCollectionID(t, env)
	tokenID := nftMint(t, env, collectionID, "seller-1")

	_, collectionsBody := callUserHandler(t, env.engine.GetActiveCollections, "", http.MethodGet,
		"/nft/collections", "", nil)
	t.Logf("SERVER_NFT_COLLECTIONS = `%s`", collectionsBody)
	assertKeys(t, collectionsBody, "collections", "success")

	_, configBody := callUserHandler(t, env.engine.GetMarketplaceConfig, "", http.MethodGet,
		"/nft/config", "", nil)
	t.Logf("SERVER_NFT_CONFIG = `%s`", configBody)
	assertKeys(t, configBody, "escrow_account", "fee_account", "fee_bps", "max_fee_bps", "success")

	_, mineBody := callUserHandler(t, env.engine.GetMyNFTs, "seller-1", http.MethodGet,
		"/nft/tokens/mine", "", nil)
	t.Logf("SERVER_NFT_TOKENS_MINE = `%s`", mineBody)
	assertKeys(t, mineBody, "success", "tokens")

	fund(t, env, "buyer-1", 1_000_000)
	listingID := listNFT(t, env, collectionID, tokenID, "seller-1", "100000")

	_, listingsBody := callUserHandler(t, env.engine.GetActiveListings, "", http.MethodGet,
		"/nft/listings", "", nil)
	t.Logf("SERVER_NFT_LISTINGS = `%s`", listingsBody)
	assertKeys(t, listingsBody, "listings", "success")

	_, buyBody := callUserHandler(t, env.engine.BuyNFT, "buyer-1", http.MethodPost,
		"/nft/listings/"+listingID+"/buy", "", gin.Params{{Key: "id", Value: listingID}})
	t.Logf("SERVER_NFT_BUY = `%s`", buyBody)
	assertKeys(t, buyBody, "listing", "success")

	// Offers: mint a fresh token so it is not escrowed by the listing above.
	offerTokenID := nftMint(t, env, collectionID, "seller-2")
	fund(t, env, "buyer-2", 1_000_000)
	offerBody := fmt.Sprintf(`{"collection_id":%q,"token_id":%d,"payment_token_address":%q,`+
		`"payment_token_symbol":"RUSD","amount":"50000","deadline":%q}`,
		collectionID, offerTokenID, nftPaymentAddress, time.Now().Add(24*time.Hour).Format(time.RFC3339))
	_, makeOfferBody := callUserHandler(t, env.engine.MakeOffer, "buyer-2", http.MethodPost,
		"/nft/offers", offerBody, nil)
	t.Logf("SERVER_NFT_OFFER = `%s`", makeOfferBody)
	assertKeys(t, makeOfferBody, "offer", "success")

	offerURL := "/nft/offers?collection_id=" + collectionID + fmt.Sprintf("&token_id=%d", offerTokenID)
	_, offersBody := callUserHandler(t, env.engine.GetOffers, "", http.MethodGet, offerURL, "", nil)
	t.Logf("SERVER_NFT_OFFERS = `%s`", offersBody)
	assertKeys(t, offersBody, "offers", "success")
}
