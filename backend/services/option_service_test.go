package services

import (
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"tigerswap/backend/models"
)

var optionTestDBSeq uint64

// optionTestEnv bundles the database, ledger and option service under test.
type optionTestEnv struct {
	db     *gorm.DB
	ledger *LedgerService
	engine *OptionService
}

func newOptionTestEnv(t *testing.T) *optionTestEnv {
	t.Helper()
	dsn := fmt.Sprintf("file:option-%s-%d?mode=memory&cache=shared", t.Name(), atomic.AddUint64(&optionTestDBSeq, 1))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db handle: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { sqlDB.Close() })

	if err := db.AutoMigrate(
		&models.OptionSeries{}, &models.OptionPosition{}, &models.OptionLiquidity{},
		&models.AccountBalance{}, &models.AdminLog{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	ledger := NewLedgerService(db)
	return &optionTestEnv{db: db, ledger: ledger, engine: NewOptionService(db, ledger)}
}

// optionTestCollateralAddress is a valid hex address used as the settlement
// token in tests.
const optionTestCollateralAddress = "0x3333333333333333333333333333333333333333"

// seedOptionSeries creates an active series through the admin handler, so the
// test exercises the real creation path and its validation.
func seedOptionSeries(t *testing.T, env *optionTestEnv, isCall bool, strike, markPrice, baseIV string, expiry time.Time) string {
	t.Helper()
	body := fmt.Sprintf(`{
		"key":"btc-%s-%s",
		"name":"BTC RUSD option",
		"chain_id":1,
		"underlying_symbol":"BTC",
		"collateral_token_address":%q,
		"collateral_token_symbol":"RUSD",
		"collateral_token_decimals":18,
		"strike":%q,
		"base_iv":%q,
		"expiry":%q,
		"is_call":%t,
		"mark_price":%q,
		"status":"active"
	}`, strike, expiry.Format("20060102"), optionTestCollateralAddress, strike, baseIV,
		expiry.Format(time.RFC3339), isCall, markPrice)

	status, response := callUserHandler(t, env.engine.CreateSeries, "admin-1", http.MethodPost, "/admin/options/series", body, nil)
	if status != http.StatusCreated {
		t.Fatalf("create series: status %d body %s", status, response)
	}
	var parsed struct {
		Series struct {
			Series models.OptionSeries `json:"series"`
		} `json:"series"`
	}
	if err := json.Unmarshal([]byte(response), &parsed); err != nil {
		t.Fatalf("decode create series: %v (%s)", err, response)
	}
	return parsed.Series.Series.ID.String()
}

// creditCollateral gives a user a ledger balance in the settlement token.
func creditCollateral(t *testing.T, env *optionTestEnv, userID, amount string) {
	t.Helper()
	value, ok := new(big.Int).SetString(amount, 10)
	if !ok {
		t.Fatalf("bad test amount %q", amount)
	}
	if err := env.ledger.Credit(userID, 1, optionTestCollateralAddress, "RUSD", 18, value); err != nil {
		t.Fatalf("credit: %v", err)
	}
}

func optionBalance(t *testing.T, env *optionTestEnv, userID string) *big.Int {
	t.Helper()
	value, err := env.ledger.Available(userID, 1, optionTestCollateralAddress)
	if err != nil {
		t.Fatalf("available: %v", err)
	}
	return value
}

// TestOptionSeriesCreationValidatesInput checks the admin create path rejects
// the malformed inputs a caller could send.
func TestOptionSeriesCreationValidatesInput(t *testing.T) {
	env := newOptionTestEnv(t)
	expiry := time.Now().Add(30 * 24 * time.Hour).Format(time.RFC3339)

	cases := []struct {
		name string
		body string
	}{
		{"bad collateral symbol", fmt.Sprintf(`{"key":"a","name":"a","chain_id":1,"underlying_symbol":"BTC","collateral_token_symbol":"DOGE","strike":"2000","base_iv":"0.8","expiry":%q,"is_call":true,"mark_price":"2000"}`, expiry)},
		{"zero strike", fmt.Sprintf(`{"key":"a","name":"a","chain_id":1,"underlying_symbol":"BTC","collateral_token_symbol":"RUSD","strike":"0","base_iv":"0.8","expiry":%q,"is_call":true,"mark_price":"2000"}`, expiry)},
		{"iv above cap", fmt.Sprintf(`{"key":"a","name":"a","chain_id":1,"underlying_symbol":"BTC","collateral_token_symbol":"RUSD","strike":"2000","base_iv":"6","expiry":%q,"is_call":true,"mark_price":"2000"}`, expiry)},
		{"expiry in the past", fmt.Sprintf(`{"key":"a","name":"a","chain_id":1,"underlying_symbol":"BTC","collateral_token_symbol":"RUSD","strike":"2000","base_iv":"0.8","expiry":%q,"is_call":true,"mark_price":"2000"}`, time.Now().Add(-time.Hour).Format(time.RFC3339))},
		{"zero mark price", fmt.Sprintf(`{"key":"a","name":"a","chain_id":1,"underlying_symbol":"BTC","collateral_token_symbol":"RUSD","strike":"2000","base_iv":"0.8","expiry":%q,"is_call":true,"mark_price":"0"}`, expiry)},
		{"bad token address", fmt.Sprintf(`{"key":"a","name":"a","chain_id":1,"underlying_symbol":"BTC","collateral_token_address":"not-an-address","collateral_token_symbol":"RUSD","strike":"2000","base_iv":"0.8","expiry":%q,"is_call":true,"mark_price":"2000"}`, expiry)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, _ := callUserHandler(t, env.engine.CreateSeries, "admin-1", http.MethodPost, "/admin/options/series", tc.body, nil)
			if status != http.StatusBadRequest {
				t.Fatalf("want 400, got %d", status)
			}
		})
	}
}

// TestOptionBuyChargesPremiumAndLocksCollateral checks the full buy path: the
// premium leaves the buyer, the pool receives it, and the collateral backing the
// position is locked.
func TestOptionBuyChargesPremiumAndLocksCollateral(t *testing.T) {
	env := newOptionTestEnv(t)
	expiry := time.Now().Add(365 * 24 * time.Hour)
	seriesID := seedOptionSeries(t, env, true, "2000", "2000", "0.8", expiry)

	creditCollateral(t, env, "lp-1", "1000000000000000000000000") // 1,000,000 RUSD
	status, response := callUserHandler(t, env.engine.AddCollateral, "lp-1", http.MethodPost,
		"/options/series/x/collateral", `{"amount":"1000000000000000000000000"}`,
		gin.Params{{Key: "id", Value: seriesID}})
	if status != http.StatusOK {
		t.Fatalf("add collateral: %d %s", status, response)
	}

	creditCollateral(t, env, "trader-1", "10000000000000000000000") // 10,000 RUSD
	status, response = callUserHandler(t, env.engine.Buy, "trader-1", http.MethodPost,
		"/options/series/x/buy", `{"size":"1","max_premium":"1000000000000000000000"}`,
		gin.Params{{Key: "id", Value: seriesID}})
	if status != http.StatusOK {
		t.Fatalf("buy: %d %s", status, response)
	}

	var parsed struct {
		Premium string `json:"premium"`
	}
	if err := json.Unmarshal([]byte(response), &parsed); err != nil {
		t.Fatalf("decode buy: %v", err)
	}
	premium, ok := new(big.Int).SetString(parsed.Premium, 10)
	if !ok || premium.Sign() <= 0 {
		t.Fatalf("premium must be positive, got %q", parsed.Premium)
	}

	// The ATM one-year call is about 656 RUSD; the pool must have locked the
	// spot value, 2000 RUSD.
	locked, ok := new(big.Int).SetString("2000000000000000000000", 10)
	if !ok {
		t.Fatal("bad constant")
	}

	var series models.OptionSeries
	if err := env.db.First(&series, "id = ?", seriesID).Error; err != nil {
		t.Fatalf("load series: %v", err)
	}
	if seriesBigInt(series.LockedCollateral).Cmp(locked) != 0 {
		t.Fatalf("locked collateral = %s, want %s", series.LockedCollateral, locked.String())
	}
	// The pool holds the LP deposit plus the premium the buyer paid.
	expectedCollateral := new(big.Int).Add(mustBigParse(t, "1000000000000000000000000"), premium)
	if seriesBigInt(series.Collateral).Cmp(expectedCollateral) != 0 {
		t.Fatalf("pool collateral = %s, want %s", series.Collateral, expectedCollateral.String())
	}

	// The trader paid exactly the premium.
	wantTrader := new(big.Int).Sub(mustBigParse(t, "10000000000000000000000"), premium)
	if got := optionBalance(t, env, "trader-1"); got.Cmp(wantTrader) != 0 {
		t.Fatalf("trader balance = %s, want %s", got.String(), wantTrader.String())
	}
}

// TestOptionBuyRejectsPremiumAboveLimit verifies the slippage guard.
func TestOptionBuyRejectsPremiumAboveLimit(t *testing.T) {
	env := newOptionTestEnv(t)
	expiry := time.Now().Add(365 * 24 * time.Hour)
	seriesID := seedOptionSeries(t, env, true, "2000", "2000", "0.8", expiry)
	creditCollateral(t, env, "lp-1", "1000000000000000000000000")
	callUserHandler(t, env.engine.AddCollateral, "lp-1", http.MethodPost,
		"/options/series/x/collateral", `{"amount":"1000000000000000000000000"}`,
		gin.Params{{Key: "id", Value: seriesID}})
	creditCollateral(t, env, "trader-1", "10000000000000000000000")

	status, _ := callUserHandler(t, env.engine.Buy, "trader-1", http.MethodPost,
		"/options/series/x/buy", `{"size":"1","max_premium":"1"}`,
		gin.Params{{Key: "id", Value: seriesID}})
	if status != http.StatusConflict {
		t.Fatalf("want 409 for a premium above the limit, got %d", status)
	}
}

// TestOptionBuyRejectsWhenPoolCannotUnderwrite verifies that a pool with too
// little collateral refuses the trade rather than opening an uncollateralised
// position.
func TestOptionBuyRejectsWhenPoolCannotUnderwrite(t *testing.T) {
	env := newOptionTestEnv(t)
	expiry := time.Now().Add(365 * 24 * time.Hour)
	seriesID := seedOptionSeries(t, env, true, "2000", "2000", "0.8", expiry)
	creditCollateral(t, env, "lp-1", "100000000000000000000") // only 100 RUSD
	callUserHandler(t, env.engine.AddCollateral, "lp-1", http.MethodPost,
		"/options/series/x/collateral", `{"amount":"100000000000000000000"}`,
		gin.Params{{Key: "id", Value: seriesID}})
	creditCollateral(t, env, "trader-1", "10000000000000000000000")

	status, _ := callUserHandler(t, env.engine.Buy, "trader-1", http.MethodPost,
		"/options/series/x/buy", `{"size":"1","max_premium":"10000000000000000000000"}`,
		gin.Params{{Key: "id", Value: seriesID}})
	if status != http.StatusConflict {
		t.Fatalf("want 409 when the pool cannot underwrite, got %d", status)
	}
}

// TestOptionBuyRejectsInsufficientBalance verifies a buyer with no balance is
// stopped by the ledger.
func TestOptionBuyRejectsInsufficientBalance(t *testing.T) {
	env := newOptionTestEnv(t)
	expiry := time.Now().Add(365 * 24 * time.Hour)
	seriesID := seedOptionSeries(t, env, true, "2000", "2000", "0.8", expiry)
	creditCollateral(t, env, "lp-1", "1000000000000000000000000")
	callUserHandler(t, env.engine.AddCollateral, "lp-1", http.MethodPost,
		"/options/series/x/collateral", `{"amount":"1000000000000000000000000"}`,
		gin.Params{{Key: "id", Value: seriesID}})

	status, _ := callUserHandler(t, env.engine.Buy, "trader-1", http.MethodPost,
		"/options/series/x/buy", `{"size":"1","max_premium":"1000000000000000000000"}`,
		gin.Params{{Key: "id", Value: seriesID}})
	if status != http.StatusConflict {
		t.Fatalf("want 409 for insufficient balance, got %d", status)
	}
}

// TestOptionExercisePaysIntrinsicAndBlocksReplay verifies the settlement path:
// the holder receives the intrinsic payout, the position is zeroed, and a second
// exercise pays nothing.
func TestOptionExercisePaysIntrinsicAndBlocksReplay(t *testing.T) {
	env := newOptionTestEnv(t)
	expiry := time.Now().Add(30 * 24 * time.Hour)
	seriesID := seedOptionSeries(t, env, true, "2000", "2000", "0.8", expiry)

	creditCollateral(t, env, "lp-1", "1000000000000000000000000")
	callUserHandler(t, env.engine.AddCollateral, "lp-1", http.MethodPost,
		"/options/series/x/collateral", `{"amount":"1000000000000000000000000"}`,
		gin.Params{{Key: "id", Value: seriesID}})
	creditCollateral(t, env, "trader-1", "10000000000000000000000")
	callUserHandler(t, env.engine.Buy, "trader-1", http.MethodPost,
		"/options/series/x/buy", `{"size":"1","max_premium":"1000000000000000000000"}`,
		gin.Params{{Key: "id", Value: seriesID}})

	// Move the expiry into the past so the series can settle, then settle at a
	// price above the strike.
	if err := env.db.Model(&models.OptionSeries{}).Where("id = ?", seriesID).
		Update("expiry", time.Now().Add(-time.Hour)).Error; err != nil {
		t.Fatalf("expire series: %v", err)
	}
	status, response := callUserHandler(t, env.engine.SettleSeries, "admin-1", http.MethodPost,
		"/admin/options/series/x/settle", `{"settlement_price":"2500"}`,
		gin.Params{{Key: "id", Value: seriesID}})
	if status != http.StatusOK {
		t.Fatalf("settle: %d %s", status, response)
	}

	balanceBefore := optionBalance(t, env, "trader-1")
	status, response = callUserHandler(t, env.engine.Exercise, "trader-1", http.MethodPost,
		"/options/series/x/exercise", "", gin.Params{{Key: "id", Value: seriesID}})
	if status != http.StatusOK {
		t.Fatalf("exercise: %d %s", status, response)
	}
	var parsed struct {
		Payout string `json:"payout"`
	}
	if err := json.Unmarshal([]byte(response), &parsed); err != nil {
		t.Fatalf("decode exercise: %v", err)
	}
	// A call struck at 2000 settled at 2500 pays 500 per unit.
	if parsed.Payout != "500000000000000000000" {
		t.Fatalf("payout = %s, want 500e18", parsed.Payout)
	}
	balanceAfter := optionBalance(t, env, "trader-1")
	gained := new(big.Int).Sub(balanceAfter, balanceBefore)
	if gained.Cmp(mustBigParse(t, "500000000000000000000")) != 0 {
		t.Fatalf("balance gained %s, want 500e18", gained.String())
	}

	// A second exercise must not pay again.
	status, _ = callUserHandler(t, env.engine.Exercise, "trader-1", http.MethodPost,
		"/options/series/x/exercise", "", gin.Params{{Key: "id", Value: seriesID}})
	if status != http.StatusConflict {
		t.Fatalf("second exercise should conflict, got %d", status)
	}
	if got := optionBalance(t, env, "trader-1"); got.Cmp(balanceAfter) != 0 {
		t.Fatalf("balance changed on a replayed exercise: %s", got.String())
	}
}

// TestOptionExpiredOutOfTheMoneyPaysNothing verifies an out-of-the-money option
// settles at zero and the position is still closed.
func TestOptionExpiredOutOfTheMoneyPaysNothing(t *testing.T) {
	env := newOptionTestEnv(t)
	expiry := time.Now().Add(30 * 24 * time.Hour)
	seriesID := seedOptionSeries(t, env, true, "2000", "2000", "0.8", expiry)

	creditCollateral(t, env, "lp-1", "1000000000000000000000000")
	callUserHandler(t, env.engine.AddCollateral, "lp-1", http.MethodPost,
		"/options/series/x/collateral", `{"amount":"1000000000000000000000000"}`,
		gin.Params{{Key: "id", Value: seriesID}})
	creditCollateral(t, env, "trader-1", "10000000000000000000000")
	callUserHandler(t, env.engine.Buy, "trader-1", http.MethodPost,
		"/options/series/x/buy", `{"size":"1","max_premium":"1000000000000000000000"}`,
		gin.Params{{Key: "id", Value: seriesID}})

	if err := env.db.Model(&models.OptionSeries{}).Where("id = ?", seriesID).
		Update("expiry", time.Now().Add(-time.Hour)).Error; err != nil {
		t.Fatalf("expire series: %v", err)
	}
	callUserHandler(t, env.engine.SettleSeries, "admin-1", http.MethodPost,
		"/admin/options/series/x/settle", `{"settlement_price":"1500"}`,
		gin.Params{{Key: "id", Value: seriesID}})

	balanceBefore := optionBalance(t, env, "trader-1")
	status, response := callUserHandler(t, env.engine.Exercise, "trader-1", http.MethodPost,
		"/options/series/x/exercise", "", gin.Params{{Key: "id", Value: seriesID}})
	if status != http.StatusOK {
		t.Fatalf("exercise: %d %s", status, response)
	}
	var parsed struct {
		Payout string `json:"payout"`
	}
	if err := json.Unmarshal([]byte(response), &parsed); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if parsed.Payout != "0" {
		t.Fatalf("out-of-the-money payout = %s, want 0", parsed.Payout)
	}
	if got := optionBalance(t, env, "trader-1"); got.Cmp(balanceBefore) != 0 {
		t.Fatalf("balance changed for a worthless option: %s", got.String())
	}
}

// TestOptionRemoveCollateralRejectsLockedFunds verifies an LP cannot withdraw
// collateral that is backing an open position.
func TestOptionRemoveCollateralRejectsLockedFunds(t *testing.T) {
	env := newOptionTestEnv(t)
	expiry := time.Now().Add(365 * 24 * time.Hour)
	seriesID := seedOptionSeries(t, env, true, "2000", "2000", "0.8", expiry)

	creditCollateral(t, env, "lp-1", "1000000000000000000000000")
	callUserHandler(t, env.engine.AddCollateral, "lp-1", http.MethodPost,
		"/options/series/x/collateral", `{"amount":"1000000000000000000000000"}`,
		gin.Params{{Key: "id", Value: seriesID}})
	creditCollateral(t, env, "trader-1", "10000000000000000000000")
	callUserHandler(t, env.engine.Buy, "trader-1", http.MethodPost,
		"/options/series/x/buy", `{"size":"1","max_premium":"1000000000000000000000"}`,
		gin.Params{{Key: "id", Value: seriesID}})

	// Withdrawing the entire deposit would leave the pool unable to cover the
	// position, so it must be rejected.
	status, _ := callUserHandler(t, env.engine.RemoveCollateral, "lp-1", http.MethodPost,
		"/options/series/x/collateral/remove", `{"amount":"1000000000000000000000000"}`,
		gin.Params{{Key: "id", Value: seriesID}})
	if status != http.StatusConflict {
		t.Fatalf("want 409 when withdrawing locked collateral, got %d", status)
	}

	// A withdrawal that leaves the lock covered succeeds.
	status, response := callUserHandler(t, env.engine.RemoveCollateral, "lp-1", http.MethodPost,
		"/options/series/x/collateral/remove", `{"amount":"100000000000000000000"}`,
		gin.Params{{Key: "id", Value: seriesID}})
	if status != http.StatusOK {
		t.Fatalf("want 200 for a covered withdrawal, got %d %s", status, response)
	}
}

// TestOptionRemoveCollateralRejectsNonDepositor verifies a user who never
// deposited cannot withdraw from the pool.
func TestOptionRemoveCollateralRejectsNonDepositor(t *testing.T) {
	env := newOptionTestEnv(t)
	expiry := time.Now().Add(365 * 24 * time.Hour)
	seriesID := seedOptionSeries(t, env, true, "2000", "2000", "0.8", expiry)
	creditCollateral(t, env, "lp-1", "1000000000000000000000000")
	callUserHandler(t, env.engine.AddCollateral, "lp-1", http.MethodPost,
		"/options/series/x/collateral", `{"amount":"1000000000000000000000000"}`,
		gin.Params{{Key: "id", Value: seriesID}})

	status, _ := callUserHandler(t, env.engine.RemoveCollateral, "attacker-1", http.MethodPost,
		"/options/series/x/collateral/remove", `{"amount":"1"}`,
		gin.Params{{Key: "id", Value: seriesID}})
	if status != http.StatusConflict {
		t.Fatalf("want 409 for a non-depositor, got %d", status)
	}
}

// TestOptionWithdrawLiquidityReservesOpenPositions verifies an LP cannot
// withdraw collateral reserved for an unexercised position.
func TestOptionWithdrawLiquidityReservesOpenPositions(t *testing.T) {
	env := newOptionTestEnv(t)
	expiry := time.Now().Add(30 * 24 * time.Hour)
	seriesID := seedOptionSeries(t, env, true, "2000", "2000", "0.8", expiry)

	creditCollateral(t, env, "lp-1", "1000000000000000000000000")
	callUserHandler(t, env.engine.AddCollateral, "lp-1", http.MethodPost,
		"/options/series/x/collateral", `{"amount":"1000000000000000000000000"}`,
		gin.Params{{Key: "id", Value: seriesID}})
	creditCollateral(t, env, "trader-1", "10000000000000000000000")
	callUserHandler(t, env.engine.Buy, "trader-1", http.MethodPost,
		"/options/series/x/buy", `{"size":"1","max_premium":"1000000000000000000000"}`,
		gin.Params{{Key: "id", Value: seriesID}})

	if err := env.db.Model(&models.OptionSeries{}).Where("id = ?", seriesID).
		Update("expiry", time.Now().Add(-time.Hour)).Error; err != nil {
		t.Fatalf("expire series: %v", err)
	}
	callUserHandler(t, env.engine.SettleSeries, "admin-1", http.MethodPost,
		"/admin/options/series/x/settle", `{"settlement_price":"1500"}`,
		gin.Params{{Key: "id", Value: seriesID}})

	// The pool holds roughly the deposit plus the premium; the position still
	// reserves 2000 RUSD, so withdrawing the whole deposit is rejected.
	status, _ := callUserHandler(t, env.engine.WithdrawLiquidity, "lp-1", http.MethodPost,
		"/options/series/x/withdraw", `{"amount":"1000000000000000000000000"}`,
		gin.Params{{Key: "id", Value: seriesID}})
	if status != http.StatusConflict {
		t.Fatalf("want 409 when withdrawing reserved collateral, got %d", status)
	}
}

// TestOptionSettleRequiresExpiry verifies a live series cannot be settled.
func TestOptionSettleRequiresExpiry(t *testing.T) {
	env := newOptionTestEnv(t)
	expiry := time.Now().Add(30 * 24 * time.Hour)
	seriesID := seedOptionSeries(t, env, true, "2000", "2000", "0.8", expiry)

	status, _ := callUserHandler(t, env.engine.SettleSeries, "admin-1", http.MethodPost,
		"/admin/options/series/x/settle", `{"settlement_price":"2500"}`,
		gin.Params{{Key: "id", Value: seriesID}})
	if status != http.StatusConflict {
		t.Fatalf("want 409 settling a live series, got %d", status)
	}
}

// TestOptionBuyRejectsPausedSeries verifies a paused series is not tradable.
func TestOptionBuyRejectsPausedSeries(t *testing.T) {
	env := newOptionTestEnv(t)
	expiry := time.Now().Add(365 * 24 * time.Hour)
	seriesID := seedOptionSeries(t, env, true, "2000", "2000", "0.8", expiry)
	if err := env.db.Model(&models.OptionSeries{}).Where("id = ?", seriesID).
		Update("status", models.OptionStatusPaused).Error; err != nil {
		t.Fatalf("pause series: %v", err)
	}
	creditCollateral(t, env, "trader-1", "10000000000000000000000")

	status, _ := callUserHandler(t, env.engine.Buy, "trader-1", http.MethodPost,
		"/options/series/x/buy", `{"size":"1","max_premium":"1000000000000000000000"}`,
		gin.Params{{Key: "id", Value: seriesID}})
	if status != http.StatusConflict {
		t.Fatalf("want 409 for a paused series, got %d", status)
	}
}

// TestOptionPutPayoutIsStrikeMinusSettlement verifies put settlement.
func TestOptionPutPayoutIsStrikeMinusSettlement(t *testing.T) {
	env := newOptionTestEnv(t)
	expiry := time.Now().Add(30 * 24 * time.Hour)
	seriesID := seedOptionSeries(t, env, false, "2000", "2000", "0.8", expiry)

	creditCollateral(t, env, "lp-1", "1000000000000000000000000")
	callUserHandler(t, env.engine.AddCollateral, "lp-1", http.MethodPost,
		"/options/series/x/collateral", `{"amount":"1000000000000000000000000"}`,
		gin.Params{{Key: "id", Value: seriesID}})
	creditCollateral(t, env, "trader-1", "10000000000000000000000")
	callUserHandler(t, env.engine.Buy, "trader-1", http.MethodPost,
		"/options/series/x/buy", `{"size":"1","max_premium":"1000000000000000000000"}`,
		gin.Params{{Key: "id", Value: seriesID}})

	if err := env.db.Model(&models.OptionSeries{}).Where("id = ?", seriesID).
		Update("expiry", time.Now().Add(-time.Hour)).Error; err != nil {
		t.Fatalf("expire series: %v", err)
	}
	callUserHandler(t, env.engine.SettleSeries, "admin-1", http.MethodPost,
		"/admin/options/series/x/settle", `{"settlement_price":"1500"}`,
		gin.Params{{Key: "id", Value: seriesID}})

	status, response := callUserHandler(t, env.engine.Exercise, "trader-1", http.MethodPost,
		"/options/series/x/exercise", "", gin.Params{{Key: "id", Value: seriesID}})
	if status != http.StatusOK {
		t.Fatalf("exercise: %d %s", status, response)
	}
	var parsed struct {
		Payout string `json:"payout"`
	}
	if err := json.Unmarshal([]byte(response), &parsed); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if parsed.Payout != "500000000000000000000" {
		t.Fatalf("put payout = %s, want 500e18", parsed.Payout)
	}
}

// mustBigParse parses a base-10 integer or fails the test.
func mustBigParse(t *testing.T, raw string) *big.Int {
	t.Helper()
	value, ok := new(big.Int).SetString(raw, 10)
	if !ok {
		t.Fatalf("bad test integer %q", raw)
	}
	return value
}
