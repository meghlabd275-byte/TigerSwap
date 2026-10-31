package services

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"tigerswap/backend/models"
)

var botsTestDBSeq uint64

// newBotsTestDB opens an in-memory database with the bot tables migrated.
func newBotsTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s-%d?mode=memory&cache=shared", t.Name(), atomic.AddUint64(&botsTestDBSeq, 1))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db handle: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { sqlDB.Close() })

	if err := db.AutoMigrate(
		&models.BotInstance{},
		&models.BotOrder{},
		&models.BotPerformance{},
		&models.OtcOrder{},
		&models.TwammOrder{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// callHandler runs a gin handler with an authenticated user and a JSON body,
// returning the recorded status and response.
func callHandler(
	t *testing.T,
	handler gin.HandlerFunc,
	userID uint,
	method, target, body string,
	params gin.Params,
) (int, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Params = params
	context.Request = httptest.NewRequest(method, target, strings.NewReader(body))
	context.Request.Header.Set("Content-Type", "application/json")
	context.Set("user_id", userID)

	handler(context)
	return recorder.Code, recorder.Body.String()
}

func idParam(id string) gin.Params {
	return gin.Params{{Key: "id", Value: id}}
}

// ------------------------------------------------------- handler wiring

func TestCreateBotRejectsUnvalidatedConfig(t *testing.T) {
	db := newBotsTestDB(t)
	service := NewBotsService(db)

	body := `{
		"name":"mm",
		"strategy":"market_maker",
		"symbol":"BTC/RUSD",
		"config":{"spread_bps":10,"bogus":1},
		"max_order_size":"1",
		"max_daily_loss":"0",
		"max_open_orders":5
	}`
	status, response := callHandler(t, service.CreateBot, 1, http.MethodPost, "/bots", body, nil)
	if status != http.StatusBadRequest {
		t.Fatalf("a config with an unknown key should be rejected, got %d: %s", status, response)
	}
}

func TestCreateBotRejectsMalformedSymbol(t *testing.T) {
	db := newBotsTestDB(t)
	service := NewBotsService(db)

	body := `{
		"name":"mm",
		"strategy":"market_maker",
		"symbol":"BTC/RUSD' OR 1=1--",
		"config":{"spread_bps":10},
		"max_order_size":"1",
		"max_daily_loss":"0",
		"max_open_orders":5
	}`
	status, _ := callHandler(t, service.CreateBot, 1, http.MethodPost, "/bots", body, nil)
	if status != http.StatusBadRequest {
		t.Fatalf("an injection-shaped symbol should be rejected, got %d", status)
	}
}

func TestCreateBotStoresAValidConfiguration(t *testing.T) {
	db := newBotsTestDB(t)
	service := NewBotsService(db)

	body := `{
		"name":"mm-btc",
		"strategy":"market_maker",
		"symbol":"BTC/RUSD",
		"config":{"spread_bps":10,"target_depth":"1.5"},
		"max_order_size":"1",
		"max_daily_loss":"500",
		"max_open_orders":5
	}`
	status, response := callHandler(t, service.CreateBot, 7, http.MethodPost, "/bots", body, nil)
	if status != http.StatusCreated {
		t.Fatalf("a valid bot should be created, got %d: %s", status, response)
	}
	if !strings.Contains(response, `"strategy":"market_maker"`) {
		t.Fatalf("response should carry the strategy: %s", response)
	}
	if !strings.Contains(response, `"target_depth":"1.5"`) {
		t.Fatalf("config should be returned parsed: %s", response)
	}

	var stored models.BotInstance
	if err := db.Where("user_id = ?", 7).First(&stored).Error; err != nil {
		t.Fatalf("bot should be persisted: %v", err)
	}
	if stored.UserID != 7 {
		t.Fatalf("bot should belong to user 7, got %d", stored.UserID)
	}
}

func TestCreateBotRequiresAuthentication(t *testing.T) {
	db := newBotsTestDB(t)
	service := NewBotsService(db)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/bots", strings.NewReader("{}"))
	service.CreateBot(context)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("an unauthenticated call should be refused, got %d", recorder.Code)
	}
}

func TestGetBotDoesNotLeakAnotherUsersBot(t *testing.T) {
	db := newBotsTestDB(t)
	service := NewBotsService(db)

	bot := models.BotInstance{
		UserID: 1, Name: "mm", Strategy: models.StrategyMarketMaker, Symbol: "BTC/RUSD",
		Status: models.BotStatusActive, Config: "{}",
		MaxOrderSize: "1", MaxDailyLoss: "0", MaxOpenOrders: 5,
	}
	if err := db.Create(&bot).Error; err != nil {
		t.Fatalf("seed bot: %v", err)
	}

	// A different user asking for the same id must get a 404, not the bot.
	status, _ := callHandler(t, service.GetBot, 2, http.MethodGet, "/bots/1", "", idParam("1"))
	if status != http.StatusNotFound {
		t.Fatalf("another user's bot must not be readable, got %d", status)
	}

	status, _ = callHandler(t, service.GetBot, 1, http.MethodGet, "/bots/1", "", idParam("1"))
	if status != http.StatusOK {
		t.Fatalf("the owner should read their own bot, got %d", status)
	}
}

func TestUpdateBotStatusRejectsAnUnknownStatus(t *testing.T) {
	db := newBotsTestDB(t)
	service := NewBotsService(db)

	bot := models.BotInstance{
		UserID: 1, Name: "mm", Strategy: models.StrategyMarketMaker, Symbol: "BTC/RUSD",
		Status: models.BotStatusActive, Config: "{}",
		MaxOrderSize: "1", MaxDailyLoss: "0", MaxOpenOrders: 5,
	}
	if err := db.Create(&bot).Error; err != nil {
		t.Fatalf("seed bot: %v", err)
	}

	status, _ := callHandler(t, service.UpdateBotStatus, 1, http.MethodPut, "/bots/1/status",
		`{"status":"exploded"}`, idParam("1"))
	if status != http.StatusBadRequest {
		t.Fatalf("an unknown status must be rejected, got %d", status)
	}
}

func TestDeleteBotRefusesAnActiveBot(t *testing.T) {
	db := newBotsTestDB(t)
	service := NewBotsService(db)

	bot := models.BotInstance{
		UserID: 1, Name: "mm", Strategy: models.StrategyMarketMaker, Symbol: "BTC/RUSD",
		Status: models.BotStatusActive, Config: "{}",
		MaxOrderSize: "1", MaxDailyLoss: "0", MaxOpenOrders: 5,
	}
	if err := db.Create(&bot).Error; err != nil {
		t.Fatalf("seed bot: %v", err)
	}

	status, _ := callHandler(t, service.DeleteBot, 1, http.MethodDelete, "/bots/1", "", idParam("1"))
	if status != http.StatusConflict {
		t.Fatalf("an active bot must be paused before deletion, got %d", status)
	}
}

func TestDeleteBotRemovesPendingOrders(t *testing.T) {
	db := newBotsTestDB(t)
	service := NewBotsService(db)

	bot := models.BotInstance{
		UserID: 1, Name: "mm", Strategy: models.StrategyMarketMaker, Symbol: "BTC/RUSD",
		Status: models.BotStatusPaused, Config: "{}",
		MaxOrderSize: "1", MaxDailyLoss: "0", MaxOpenOrders: 5,
	}
	if err := db.Create(&bot).Error; err != nil {
		t.Fatalf("seed bot: %v", err)
	}
	if err := db.Create(&models.BotOrder{
		BotID: bot.ID, UserID: 1, Symbol: "BTC/RUSD", Side: "buy", OrderKind: "limit",
		Price: "60000", Quantity: "0.5", Source: "market_maker", Status: "pending",
	}).Error; err != nil {
		t.Fatalf("seed order: %v", err)
	}

	status, _ := callHandler(t, service.DeleteBot, 1, http.MethodDelete, "/bots/1", "", idParam("1"))
	if status != http.StatusOK {
		t.Fatalf("a paused bot should delete, got %d", status)
	}

	var remaining int64
	if err := db.Model(&models.BotOrder{}).Where("bot_id = ?", bot.ID).Count(&remaining).Error; err != nil {
		t.Fatalf("count orders: %v", err)
	}
	if remaining != 0 {
		t.Fatalf("deleting a bot must remove its pending orders, %d remain", remaining)
	}
}

func TestListBotOrdersRejectsAnInvalidLimit(t *testing.T) {
	db := newBotsTestDB(t)
	service := NewBotsService(db)

	bot := models.BotInstance{
		UserID: 1, Name: "mm", Strategy: models.StrategyMarketMaker, Symbol: "BTC/RUSD",
		Status: models.BotStatusActive, Config: "{}",
		MaxOrderSize: "1", MaxDailyLoss: "0", MaxOpenOrders: 5,
	}
	if err := db.Create(&bot).Error; err != nil {
		t.Fatalf("seed bot: %v", err)
	}

	status, _ := callHandler(t, service.ListBotOrders, 1, http.MethodGet, "/bots/1/orders?limit=99999", "", idParam("1"))
	if status != http.StatusBadRequest {
		t.Fatalf("an out-of-range limit must be rejected, got %d", status)
	}
}

func TestListBotOrdersRejectsANonNumericId(t *testing.T) {
	db := newBotsTestDB(t)
	service := NewBotsService(db)

	status, _ := callHandler(t, service.ListBotOrders, 1, http.MethodGet, "/bots/abc/orders", "", idParam("abc"))
	if status != http.StatusBadRequest {
		t.Fatalf("a non-numeric id must be rejected, got %d", status)
	}
}

func TestCreateOtcOrderRequiresAValidExpiry(t *testing.T) {
	db := newBotsTestDB(t)
	service := NewOtcService(db)

	base := `{"symbol":"BTC/RUSD","side":"sell","quantity":"1","price":"60000","min_fill":"1","expires_in_seconds":%d}`

	// Zero and an over-long expiry are both refused.
	for _, seconds := range []int{0, -1, 31 * 24 * 3600} {
		body := strings.Replace(base, "%d", strconv.Itoa(seconds), 1)
		status, _ := callHandler(t, service.CreateOrder, 1, http.MethodPost, "/otc/orders", body, nil)
		if status != http.StatusBadRequest {
			t.Fatalf("expiry %d should be rejected, got %d", seconds, status)
		}
	}

	body := strings.Replace(base, "%d", "3600", 1)
	status, response := callHandler(t, service.CreateOrder, 1, http.MethodPost, "/otc/orders", body, nil)
	if status != http.StatusCreated {
		t.Fatalf("a valid otc order should be created, got %d: %s", status, response)
	}
}

func TestCreateOtcOrderRejectsMinFillAboveQuantity(t *testing.T) {
	db := newBotsTestDB(t)
	service := NewOtcService(db)

	body := `{"symbol":"BTC/RUSD","side":"sell","quantity":"1","price":"60000","min_fill":"2","expires_in_seconds":3600}`
	status, _ := callHandler(t, service.CreateOrder, 1, http.MethodPost, "/otc/orders", body, nil)
	if status != http.StatusBadRequest {
		t.Fatalf("min_fill above quantity must be rejected, got %d", status)
	}
}

func TestTakeOtcOrderRefusesASelfTake(t *testing.T) {
	db := newBotsTestDB(t)
	service := NewOtcService(db)

	order := models.OtcOrder{
		MakerID: 1, Symbol: "BTC/RUSD", Side: "sell", Quantity: "1", Price: "60000",
		MinFill: "1", Status: "open", ExpiresAt: time.Now().UTC().Add(time.Hour),
	}
	if err := db.Create(&order).Error; err != nil {
		t.Fatalf("seed order: %v", err)
	}

	status, response := callHandler(t, service.TakeOrder, 1, http.MethodPost,
		"/otc/orders/1/take", `{"quantity":"1"}`, idParam(strconv.FormatUint(uint64(order.ID), 10)))
	if status != http.StatusBadRequest {
		t.Fatalf("taking your own otc order must be refused, got %d: %s", status, response)
	}
}

func TestTakeOtcOrderRefusesBelowMinFill(t *testing.T) {
	db := newBotsTestDB(t)
	service := NewOtcService(db)

	order := models.OtcOrder{
		MakerID: 1, Symbol: "BTC/RUSD", Side: "sell", Quantity: "10", Price: "60000",
		MinFill: "5", Status: "open", ExpiresAt: time.Now().UTC().Add(time.Hour),
	}
	if err := db.Create(&order).Error; err != nil {
		t.Fatalf("seed order: %v", err)
	}

	status, _ := callHandler(t, service.TakeOrder, 2, http.MethodPost,
		"/otc/orders/1/take", `{"quantity":"1"}`, idParam(strconv.FormatUint(uint64(order.ID), 10)))
	if status != http.StatusBadRequest {
		t.Fatalf("a fill below min_fill must be refused, got %d", status)
	}
}

func TestCreateTwammOrderRequiresAMinimumDuration(t *testing.T) {
	db := newBotsTestDB(t)
	service := NewTwammService(db)

	// One hour is below the five-minute minimum? No: 0 hours is.
	body := `{"symbol":"BTC/RUSD","side":"sell","total_quantity":"10","limit_price":"0","duration_hours":0}`
	status, _ := callHandler(t, service.CreateOrder, 1, http.MethodPost, "/twamm/orders", body, nil)
	if status != http.StatusBadRequest {
		t.Fatalf("a zero-hour twamm order must be rejected, got %d", status)
	}

	body = `{"symbol":"BTC/RUSD","side":"sell","total_quantity":"10","limit_price":"0","duration_hours":1}`
	status, response := callHandler(t, service.CreateOrder, 1, http.MethodPost, "/twamm/orders", body, nil)
	if status != http.StatusCreated {
		t.Fatalf("a one-hour twamm order should be created, got %d: %s", status, response)
	}
}

func TestCreateTwammOrderDefaultsAnEmptyLimitPriceToZero(t *testing.T) {
	db := newBotsTestDB(t)
	service := NewTwammService(db)

	body := `{"symbol":"BTC/RUSD","side":"buy","total_quantity":"10","duration_hours":1}`
	status, response := callHandler(t, service.CreateOrder, 1, http.MethodPost, "/twamm/orders", body, nil)
	if status != http.StatusCreated {
		t.Fatalf("a twamm order without a limit price should be created, got %d: %s", status, response)
	}

	var stored models.TwammOrder
	if err := db.Where("user_id = ?", 1).First(&stored).Error; err != nil {
		t.Fatalf("load twamm order: %v", err)
	}
	if stored.LimitPrice != "0" {
		t.Fatalf("an absent limit price should default to 0, got %s", stored.LimitPrice)
	}
}

func TestCancelTwammOrderRefusesAnotherUsersOrder(t *testing.T) {
	db := newBotsTestDB(t)
	service := NewTwammService(db)

	order := models.TwammOrder{
		UserID: 1, Symbol: "BTC/RUSD", Side: "sell", TotalQuantity: "10",
		FilledQuantity: "0", LimitPrice: "0",
		StartTime: time.Now().UTC(), EndTime: time.Now().UTC().Add(time.Hour),
		Status: "active",
	}
	if err := db.Create(&order).Error; err != nil {
		t.Fatalf("seed twamm: %v", err)
	}

	status, _ := callHandler(t, service.CancelOrder, 2, http.MethodPost,
		"/twamm/orders/1/cancel", "", idParam(strconv.FormatUint(uint64(order.ID), 10)))
	if status != http.StatusNotFound {
		t.Fatalf("another user's twamm order must not be cancellable, got %d", status)
	}
}

// ------------------------------------------------------- config validation

func TestValidateStrategyConfigAcceptsAValidMarketMaker(t *testing.T) {
	raw := json.RawMessage(`{"spread_bps":10,"target_depth":"1.5","rebalance_threshold_bps":10}`)
	canonical, err := validateStrategyConfig(models.StrategyMarketMaker, raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(canonical), &parsed); err != nil {
		t.Fatalf("canonical config is not valid JSON: %v", err)
	}
	if parsed["target_depth"] != "1.5" {
		t.Fatalf("target_depth should round-trip as a string, got %#v", parsed["target_depth"])
	}
}

func TestValidateStrategyConfigRejectsAnUnknownKey(t *testing.T) {
	raw := json.RawMessage(`{"spread_bps":10,"not_a_real_key":5}`)
	_, err := validateStrategyConfig(models.StrategyMarketMaker, raw)
	if err == nil {
		t.Fatal("an unknown config key must be rejected")
	}
	if !strings.Contains(err.Error(), "not_a_real_key") {
		t.Fatalf("the error should name the offending key, got %v", err)
	}
}

func TestValidateStrategyConfigRejectsAKeyFromAnotherStrategy(t *testing.T) {
	// levels belongs to the grid strategy, not the market maker.
	raw := json.RawMessage(`{"levels":5}`)
	if _, err := validateStrategyConfig(models.StrategyMarketMaker, raw); err == nil {
		t.Fatal("a key from another strategy must be rejected")
	}
}

func TestValidateStrategyConfigRejectsABadDecimal(t *testing.T) {
	for _, bad := range []string{`"1.2.3"`, `"abc"`, `"-1"`, `"1e5"`, `"1,000"`} {
		raw := json.RawMessage(`{"target_depth":` + bad + `}`)
		if _, err := validateStrategyConfig(models.StrategyMarketMaker, raw); err == nil {
			t.Fatalf("target_depth %s must be rejected", bad)
		}
	}
}

func TestValidateStrategyConfigRejectsATooPreciseDecimal(t *testing.T) {
	// 19 decimal places exceeds the 18-place scale.
	raw := json.RawMessage(`{"target_depth":"0.0000000000000000001"}`)
	if _, err := validateStrategyConfig(models.StrategyMarketMaker, raw); err == nil {
		t.Fatal("a value with more than 18 decimals must be rejected")
	}
}

func TestValidateStrategyConfigRejectsABadSide(t *testing.T) {
	raw := json.RawMessage(`{"total_quantity":"10","side":"sideways"}`)
	if _, err := validateStrategyConfig(models.StrategyTWAP, raw); err == nil {
		t.Fatal("side must be buy or sell")
	}
}

func TestValidateStrategyConfigRejectsABadAddress(t *testing.T) {
	for _, bad := range []string{`"0xnothex"`, `"nothex"`, `"0x"`, `"0xzzzz"`} {
		raw := json.RawMessage(`{"trader_address":` + bad + `}`)
		if _, err := validateStrategyConfig(models.StrategyCopy, raw); err == nil {
			t.Fatalf("address %s must be rejected", bad)
		}
	}
}

func TestValidateStrategyConfigLowercasesAnAddress(t *testing.T) {
	raw := json.RawMessage(`{"trader_address":"0xABCDEF0123456789ABCDEF0123456789ABCDEF01"}`)
	canonical, err := validateStrategyConfig(models.StrategyCopy, raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(canonical, "ABCDEF") {
		t.Fatalf("address should be lowercased, got %s", canonical)
	}
}

func TestValidateStrategyConfigAcceptsEmptyConfig(t *testing.T) {
	canonical, err := validateStrategyConfig(models.StrategyLiquidation, nil)
	if err != nil {
		t.Fatalf("an empty config should be accepted: %v", err)
	}
	if canonical != "{}" {
		t.Fatalf("empty config should canonicalise to {}, got %s", canonical)
	}
}

func TestValidateStrategyConfigRejectsUnknownStrategy(t *testing.T) {
	if _, err := validateStrategyConfig("not_a_strategy", nil); err == nil {
		t.Fatal("an unknown strategy must be rejected")
	}
}

// ------------------------------------------------------- decimal parsing

func TestParseDecimalStringAcceptsPlainDecimals(t *testing.T) {
	cases := map[string]string{
		"0":      "0",
		"1":      "1000000000000000000",
		"1.5":    "1500000000000000000",
		"0.000000000000000001": "1",
		".5":     "500000000000000000",
		"100":    "100000000000000000000",
	}
	for input, want := range cases {
		got, err := parseDecimalString(input)
		if err != nil {
			t.Fatalf("%q: unexpected error %v", input, err)
		}
		if got.String() != want {
			t.Fatalf("%q: got %s, want %s", input, got.String(), want)
		}
	}
}

func TestParseDecimalStringRejectsMalformedInput(t *testing.T) {
	for _, bad := range []string{"", "abc", "1.2.3", "-1", "1e5", "1,000", "+1", " 1 2 "} {
		if _, err := parseDecimalString(bad); err == nil {
			t.Fatalf("%q must be rejected", bad)
		}
	}
}

func TestValidatePositiveDecimalRejectsZero(t *testing.T) {
	if _, err := validatePositiveDecimal("quantity", "0"); err == nil {
		t.Fatal("zero must be rejected as a positive quantity")
	}
	if _, err := validatePositiveDecimal("quantity", "0.000000000000000001"); err != nil {
		t.Fatalf("the smallest representable value is positive: %v", err)
	}
}

// ------------------------------------------------------- symbol validation

func TestValidateMarketSymbolAcceptsValidSymbols(t *testing.T) {
	for _, symbol := range []string{"BTC/RUSD", "ETH/USDT", "BTC", "TGR-USD", "BTC.ETH"} {
		if err := validateMarketSymbol(symbol); err != nil {
			t.Fatalf("%q should be valid: %v", symbol, err)
		}
	}
}

func TestValidateMarketSymbolRejectsInjectionAttempts(t *testing.T) {
	// These are the shapes that would matter if the value were echoed into a
	// response or a query without parameterisation.
	for _, symbol := range []string{
		"",
		"BTC/RUSD' OR 1=1--",
		"<script>alert(1)</script>",
		"BTC/RUSD;DROP TABLE pools",
		"BTC/../../etc/passwd",
		strings.Repeat("A", 42),
		"BTC/RUSD/RUSD",
	} {
		if err := validateMarketSymbol(symbol); err == nil {
			t.Fatalf("%q must be rejected", symbol)
		}
	}
}

// ------------------------------------------------------- OTC

func TestOtcExpireStaleOrdersMarksOnlyExpiredOnes(t *testing.T) {
	db := newBotsTestDB(t)
	service := NewOtcService(db)

	expired := models.OtcOrder{
		MakerID: 1, Symbol: "BTC/RUSD", Side: "sell", Quantity: "1", Price: "60000",
		MinFill: "1", Status: "open", ExpiresAt: time.Now().UTC().Add(-time.Minute),
	}
	live := models.OtcOrder{
		MakerID: 1, Symbol: "BTC/RUSD", Side: "sell", Quantity: "1", Price: "60000",
		MinFill: "1", Status: "open", ExpiresAt: time.Now().UTC().Add(time.Hour),
	}
	if err := db.Create(&expired).Error; err != nil {
		t.Fatalf("seed expired: %v", err)
	}
	if err := db.Create(&live).Error; err != nil {
		t.Fatalf("seed live: %v", err)
	}

	affected, err := service.ExpireStaleOrders()
	if err != nil {
		t.Fatalf("expire: %v", err)
	}
	if affected != 1 {
		t.Fatalf("exactly one order should expire, got %d", affected)
	}

	var reloaded models.OtcOrder
	if err := db.First(&reloaded, live.ID).Error; err != nil {
		t.Fatalf("reload live: %v", err)
	}
	if reloaded.Status != "open" {
		t.Fatalf("a live order must stay open, got %s", reloaded.Status)
	}
}

// ------------------------------------------------------- TWAMM schedule

func TestTwammSliceIsLinearInElapsedTime(t *testing.T) {
	db := newBotsTestDB(t)
	service := NewTwammService(db)

	start := time.Now().UTC()
	order := models.TwammOrder{
		UserID:         1,
		Symbol:         "BTC/RUSD",
		Side:           "sell",
		TotalQuantity:  "100",
		FilledQuantity: "0",
		LimitPrice:     "0",
		StartTime:      start,
		EndTime:        start.Add(time.Hour),
		Status:         "active",
	}

	// Before the start there is nothing to trade.
	slice, err := service.ExecutableSlice(order, start.Add(-time.Minute))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if slice.Sign() != 0 {
		t.Fatalf("nothing should be executable before the start, got %s", slice)
	}

	// Halfway through, half the order has been released.
	slice, err = service.ExecutableSlice(order, start.Add(30*time.Minute))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if slice.String() != "50000000000000000000" { // 50 * 10^18
		t.Fatalf("halfway slice should be 50, got %s", slice)
	}

	// At the end, the whole order is released.
	slice, err = service.ExecutableSlice(order, start.Add(time.Hour))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if slice.String() != "100000000000000000000" {
		t.Fatalf("final slice should be 100, got %s", slice)
	}
}

func TestTwammSliceAccountsForAlreadyFilledQuantity(t *testing.T) {
	db := newBotsTestDB(t)
	service := NewTwammService(db)

	start := time.Now().UTC()
	order := models.TwammOrder{
		UserID:         1,
		Symbol:         "BTC/RUSD",
		Side:           "sell",
		TotalQuantity:  "100",
		FilledQuantity: "30",
		LimitPrice:     "0",
		StartTime:      start,
		EndTime:        start.Add(time.Hour),
		Status:         "active",
	}

	// Halfway, 50 is released and 30 has been filled, so 20 remains.
	slice, err := service.ExecutableSlice(order, start.Add(30*time.Minute))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if slice.String() != "20000000000000000000" {
		t.Fatalf("slice should be 20, got %s", slice)
	}
}

func TestTwammSliceNeverGoesNegative(t *testing.T) {
	db := newBotsTestDB(t)
	service := NewTwammService(db)

	start := time.Now().UTC()
	order := models.TwammOrder{
		UserID:         1,
		Symbol:         "BTC/RUSD",
		Side:           "sell",
		TotalQuantity:  "100",
		// More filled than the schedule has released, which can happen if a
		// slice filled at the start of a period.
		FilledQuantity: "60",
		LimitPrice:     "0",
		StartTime:      start,
		EndTime:        start.Add(time.Hour),
		Status:         "active",
	}

	slice, err := service.ExecutableSlice(order, start.Add(30*time.Minute))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if slice.Sign() != 0 {
		t.Fatalf("a negative slice must clamp to zero, got %s", slice)
	}
}

// ------------------------------------------------------- bot order limits

func TestRecordBotOrderEnforcesMaxOrderSize(t *testing.T) {
	db := newBotsTestDB(t)
	service := NewBotsService(db)

	bot := models.BotInstance{
		UserID:        1,
		Name:          "mm",
		Strategy:      models.StrategyMarketMaker,
		Symbol:        "BTC/RUSD",
		Status:        models.BotStatusActive,
		Config:        "{}",
		MaxOrderSize:  "1",
		MaxDailyLoss:  "0",
		MaxOpenOrders: 5,
	}
	if err := db.Create(&bot).Error; err != nil {
		t.Fatalf("seed bot: %v", err)
	}

	if _, err := service.RecordBotOrder(bot.ID, "BTC/RUSD", "buy", "limit", "60000", "2", "market_maker", false); err == nil {
		t.Fatal("an order above max_order_size must be rejected")
	}
	if _, err := service.RecordBotOrder(bot.ID, "BTC/RUSD", "buy", "limit", "60000", "0.5", "market_maker", false); err != nil {
		t.Fatalf("an order within the limit should be accepted: %v", err)
	}
}

func TestRecordBotOrderEnforcesMaxOpenOrders(t *testing.T) {
	db := newBotsTestDB(t)
	service := NewBotsService(db)

	bot := models.BotInstance{
		UserID: 1, Name: "grid", Strategy: models.StrategyGrid, Symbol: "BTC/RUSD",
		Status: models.BotStatusActive, Config: "{}",
		MaxOrderSize: "1", MaxDailyLoss: "0", MaxOpenOrders: 1,
	}
	if err := db.Create(&bot).Error; err != nil {
		t.Fatalf("seed bot: %v", err)
	}

	if _, err := service.RecordBotOrder(bot.ID, "BTC/RUSD", "buy", "limit", "60000", "0.5", "grid", false); err != nil {
		t.Fatalf("first order should be accepted: %v", err)
	}
	if _, err := service.RecordBotOrder(bot.ID, "BTC/RUSD", "buy", "limit", "60000", "0.5", "grid", false); err == nil {
		t.Fatal("the second order must be rejected once max_open_orders is reached")
	}
}

func TestRecordBotOrderRejectsInactiveBot(t *testing.T) {
	db := newBotsTestDB(t)
	service := NewBotsService(db)

	bot := models.BotInstance{
		UserID: 1, Name: "mm", Strategy: models.StrategyMarketMaker, Symbol: "BTC/RUSD",
		Status: models.BotStatusPaused, Config: "{}",
		MaxOrderSize: "1", MaxDailyLoss: "0", MaxOpenOrders: 5,
	}
	if err := db.Create(&bot).Error; err != nil {
		t.Fatalf("seed bot: %v", err)
	}

	if _, err := service.RecordBotOrder(bot.ID, "BTC/RUSD", "buy", "limit", "60000", "0.5", "market_maker", false); err == nil {
		t.Fatal("a paused bot must not place orders")
	}
}

func TestRecordBotOrderRejectsBadSideAndKind(t *testing.T) {
	db := newBotsTestDB(t)
	service := NewBotsService(db)

	bot := models.BotInstance{
		UserID: 1, Name: "mm", Strategy: models.StrategyMarketMaker, Symbol: "BTC/RUSD",
		Status: models.BotStatusActive, Config: "{}",
		MaxOrderSize: "1", MaxDailyLoss: "0", MaxOpenOrders: 5,
	}
	if err := db.Create(&bot).Error; err != nil {
		t.Fatalf("seed bot: %v", err)
	}

	if _, err := service.RecordBotOrder(bot.ID, "BTC/RUSD", "sideways", "limit", "60000", "0.5", "mm", false); err == nil {
		t.Fatal("an invalid side must be rejected")
	}
	if _, err := service.RecordBotOrder(bot.ID, "BTC/RUSD", "buy", "iceberg", "60000", "0.5", "mm", false); err == nil {
		t.Fatal("an invalid order kind must be rejected")
	}
	// A limit order without a price is not a limit order.
	if _, err := service.RecordBotOrder(bot.ID, "BTC/RUSD", "buy", "limit", "", "0.5", "mm", false); err == nil {
		t.Fatal("a limit order must carry a positive price")
	}
}
