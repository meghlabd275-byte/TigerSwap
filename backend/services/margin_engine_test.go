package services

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"tigerswap/backend/models"
)

var marginDBSeq uint64

// stubPriceAggregator serves prices from an in-memory map, exercising the
// engine's real valuation path without dialing a node. The map is read by the
// engine and written by tests between phases.
type stubPriceAggregator struct {
	prices map[string]float64
}

func (s *stubPriceAggregator) GetRealPrice(_ context.Context, symbol string) (*PriceData, error) {
	price, ok := s.prices[strings.ToUpper(strings.TrimSpace(symbol))]
	if !ok {
		return nil, fmt.Errorf("no price for %s", symbol)
	}
	if price <= 0 {
		return nil, fmt.Errorf("non-positive price for %s", symbol)
	}
	return &PriceData{Symbol: symbol, USD: price}, nil
}

// marginTestEnv bundles the database and engine under test.
type marginTestEnv struct {
	db     *gorm.DB
	engine *MarginEngine
}

func newMarginTestEnv(t *testing.T, prices map[string]float64) *marginTestEnv {
	t.Helper()
	dsn := fmt.Sprintf("file:margin-%s-%d?mode=memory&cache=shared", t.Name(), atomic.AddUint64(&marginDBSeq, 1))
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
		&models.MarginMarket{}, &models.MarginAccount{},
		&models.MarginCollateral{}, &models.MarginBorrow{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	return &marginTestEnv{db: db, engine: NewMarginEngine(db, &stubPriceAggregator{prices: prices})}
}

// seedMarginMarket inserts an active listing with the given risk parameters.
func (env *marginTestEnv) seedMarginMarket(t *testing.T, pair, base, quote string, ltv, threshold float64) {
	t.Helper()
	market := models.MarginMarket{
		Pair:                 pair,
		BaseAsset:            base,
		QuoteAsset:           quote,
		MarginMode:           "cross",
		MaxLeverage:          10,
		CollateralAssets:     base + ",RUSD",
		BorrowableAssets:     "RUSD",
		BaseRateAPY:          0.05,
		KinkUtilization:      0.8,
		JumpMultiplier:       1.0,
		LTV:                  ltv,
		LiquidationThreshold: threshold,
		LiquidationBonus:     0.05,
		CloseFactor:          0.5,
		RiskTier:             "medium",
		Status:               models.ListingStatusActive,
	}
	if err := env.db.Create(&market).Error; err != nil {
		t.Fatalf("seed margin market: %v", err)
	}
}

// TestMarginBorrowRespectsLTV verifies a borrow cannot push debt past the
// listing's LTV, and that a borrow within LTV succeeds.
func TestMarginBorrowRespectsLTV(t *testing.T) {
	ctx := context.Background()
	env := newMarginTestEnv(t, map[string]float64{"BTC": 60000, "RUSD": 1})
	env.seedMarginMarket(t, "BTC/RUSD", "BTC", "RUSD", 0.7, 0.75)

	account, err := env.engine.OpenAccount(ctx, "user-1", OpenAccountRequest{Pair: "BTC/RUSD"})
	if err != nil {
		t.Fatalf("open account: %v", err)
	}
	// 1 BTC of collateral at 60k = 60k USD; LTV 0.7 -> max debt 42k RUSD.
	if err := env.engine.DepositCollateral(ctx, "user-1", account.ID, "BTC", "1"); err != nil {
		t.Fatalf("deposit: %v", err)
	}

	if err := env.engine.Borrow(ctx, "user-1", account.ID, "RUSD", "42000"); err != nil {
		t.Fatalf("borrow within LTV should succeed: %v", err)
	}
	if err := env.engine.Borrow(ctx, "user-1", account.ID, "RUSD", "1"); !errors.Is(err, ErrMarginInsufficientCollateral) {
		t.Fatalf("borrow past LTV should be rejected, got %v", err)
	}
}

// TestMarginRejectsUnlistedMarketAndAsset verifies the listing is the source of
// truth for both tradability and which assets are allowed.
func TestMarginRejectsUnlistedMarketAndAsset(t *testing.T) {
	ctx := context.Background()
	env := newMarginTestEnv(t, map[string]float64{"BTC": 60000, "ETH": 3000, "RUSD": 1})
	env.seedMarginMarket(t, "BTC/RUSD", "BTC", "RUSD", 0.7, 0.75)

	if _, err := env.engine.OpenAccount(ctx, "user-1", OpenAccountRequest{Pair: "ETH/RUSD"}); !errors.Is(err, ErrMarginMarketNotListed) {
		t.Fatalf("unlisted market should be rejected, got %v", err)
	}

	account, err := env.engine.OpenAccount(ctx, "user-1", OpenAccountRequest{Pair: "BTC/RUSD"})
	if err != nil {
		t.Fatalf("open account: %v", err)
	}
	if err := env.engine.DepositCollateral(ctx, "user-1", account.ID, "ETH", "1"); !errors.Is(err, ErrMarginAssetNotAllowed) {
		t.Fatalf("ETH is not listed as collateral, got %v", err)
	}
	if err := env.engine.Borrow(ctx, "user-1", account.ID, "ETH", "1"); !errors.Is(err, ErrMarginAssetNotAllowed) {
		t.Fatalf("ETH is not borrowable, got %v", err)
	}
}

// TestMarginAccountIsolation verifies a user cannot touch another user's
// account, so a guessed account id is not enough to borrow or repay.
func TestMarginAccountIsolation(t *testing.T) {
	ctx := context.Background()
	env := newMarginTestEnv(t, map[string]float64{"BTC": 60000, "RUSD": 1})
	env.seedMarginMarket(t, "BTC/RUSD", "BTC", "RUSD", 0.7, 0.75)

	owner, err := env.engine.OpenAccount(ctx, "owner", OpenAccountRequest{Pair: "BTC/RUSD"})
	if err != nil {
		t.Fatalf("open account: %v", err)
	}
	if err := env.engine.DepositCollateral(ctx, "owner", owner.ID, "BTC", "1"); err != nil {
		t.Fatalf("deposit: %v", err)
	}

	if err := env.engine.Borrow(ctx, "attacker", owner.ID, "RUSD", "100"); !errors.Is(err, ErrMarginAccountNotFound) {
		t.Fatalf("attacker must not borrow from another account, got %v", err)
	}
	if err := env.engine.DepositCollateral(ctx, "attacker", owner.ID, "BTC", "1"); !errors.Is(err, ErrMarginAccountNotFound) {
		t.Fatalf("attacker must not deposit into another account, got %v", err)
	}
	if _, err := env.engine.GetAccountView(ctx, "attacker", owner.ID); !errors.Is(err, ErrMarginAccountNotFound) {
		t.Fatalf("attacker must not read another account, got %v", err)
	}
}

// TestMarginHealthAndLiquidation verifies an account becomes liquidatable when
// collateral value falls, and that liquidation reduces debt and seizes
// collateral.
func TestMarginHealthAndLiquidation(t *testing.T) {
	ctx := context.Background()
	prices := map[string]float64{"BTC": 60000, "RUSD": 1}
	env := newMarginTestEnv(t, prices)
	env.seedMarginMarket(t, "BTC/RUSD", "BTC", "RUSD", 0.7, 0.75)

	account, err := env.engine.OpenAccount(ctx, "user-1", OpenAccountRequest{Pair: "BTC/RUSD"})
	if err != nil {
		t.Fatalf("open account: %v", err)
	}
	if err := env.engine.DepositCollateral(ctx, "user-1", account.ID, "BTC", "1"); err != nil {
		t.Fatalf("deposit: %v", err)
	}
	if err := env.engine.Borrow(ctx, "user-1", account.ID, "RUSD", "40000"); err != nil {
		t.Fatalf("borrow: %v", err)
	}

	view, err := env.engine.GetAccountView(ctx, "user-1", account.ID)
	if err != nil {
		t.Fatalf("view: %v", err)
	}
	// health = 60000*0.75/40000 = 1.125
	if math.Abs(view.HealthFactor-1.125) > 1e-6 {
		t.Fatalf("expected health factor 1.125, got %v", view.HealthFactor)
	}
	if candidates, err := env.engine.LiquidatableAccounts(ctx); err != nil || len(candidates) != 0 {
		t.Fatalf("healthy account must not be liquidatable (err=%v, candidates=%d)", err, len(candidates))
	}

	// Collateral price collapses to 30k: health = 30000*0.75/40000 = 0.5625.
	prices["BTC"] = 30000

	candidates, err := env.engine.LiquidatableAccounts(ctx)
	if err != nil {
		t.Fatalf("liquidatable: %v", err)
	}
	if len(candidates) != 1 {
		t.Fatalf("expected 1 liquidatable account, got %d", len(candidates))
	}

	if _, err := env.engine.Liquidate(ctx, account.ID); err != nil {
		t.Fatalf("liquidate: %v", err)
	}

	after, err := env.engine.GetAccountView(ctx, "user-1", account.ID)
	if err != nil {
		t.Fatalf("view after: %v", err)
	}
	if after.DebtValueUSD >= view.DebtValueUSD {
		t.Fatalf("liquidation should reduce debt: before %v after %v", view.DebtValueUSD, after.DebtValueUSD)
	}
	if after.CollateralValueUSD >= view.CollateralValueUSD {
		t.Fatalf("liquidation should seize collateral: before %v after %v", view.CollateralValueUSD, after.CollateralValueUSD)
	}
}

// TestMarginHealthyAccountCannotBeLiquidated verifies the guard that stops a
// liquidation of a solvent account.
func TestMarginHealthyAccountCannotBeLiquidated(t *testing.T) {
	ctx := context.Background()
	env := newMarginTestEnv(t, map[string]float64{"BTC": 60000, "RUSD": 1})
	env.seedMarginMarket(t, "BTC/RUSD", "BTC", "RUSD", 0.7, 0.75)

	account, err := env.engine.OpenAccount(ctx, "user-1", OpenAccountRequest{Pair: "BTC/RUSD"})
	if err != nil {
		t.Fatalf("open account: %v", err)
	}
	if err := env.engine.DepositCollateral(ctx, "user-1", account.ID, "BTC", "1"); err != nil {
		t.Fatalf("deposit: %v", err)
	}
	if _, err := env.engine.Liquidate(ctx, account.ID); err == nil {
		t.Fatal("a solvent account must not be liquidatable")
	}
}

// TestMarginRepayClearsDebt verifies repayment clears interest first and
// deletes the borrow when fully repaid, and that overpayment is rejected.
func TestMarginRepayClearsDebt(t *testing.T) {
	ctx := context.Background()
	env := newMarginTestEnv(t, map[string]float64{"BTC": 60000, "RUSD": 1})
	env.seedMarginMarket(t, "BTC/RUSD", "BTC", "RUSD", 0.7, 0.75)

	account, err := env.engine.OpenAccount(ctx, "user-1", OpenAccountRequest{Pair: "BTC/RUSD"})
	if err != nil {
		t.Fatalf("open account: %v", err)
	}
	if err := env.engine.DepositCollateral(ctx, "user-1", account.ID, "BTC", "1"); err != nil {
		t.Fatalf("deposit: %v", err)
	}
	if err := env.engine.Borrow(ctx, "user-1", account.ID, "RUSD", "1000"); err != nil {
		t.Fatalf("borrow: %v", err)
	}

	if err := env.engine.Repay(ctx, "user-1", account.ID, "RUSD", "999999"); err == nil {
		t.Fatal("overpayment must be rejected")
	}
	if err := env.engine.Repay(ctx, "user-1", account.ID, "RUSD", "1000"); err != nil {
		t.Fatalf("full repay: %v", err)
	}

	var count int64
	if err := env.db.Model(&models.MarginBorrow{}).Where("account_id = ?", account.ID).Count(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected debt cleared, %d borrow rows remain", count)
	}
}

// TestMarginInterestAccrues verifies interest is charged over time and that the
// two-slope rate curve is continuous and increasing.
func TestMarginInterestAccrues(t *testing.T) {
	ctx := context.Background()
	env := newMarginTestEnv(t, map[string]float64{"BTC": 60000, "RUSD": 1})
	env.seedMarginMarket(t, "BTC/RUSD", "BTC", "RUSD", 0.7, 0.75)

	account, err := env.engine.OpenAccount(ctx, "user-1", OpenAccountRequest{Pair: "BTC/RUSD"})
	if err != nil {
		t.Fatalf("open account: %v", err)
	}
	if err := env.engine.DepositCollateral(ctx, "user-1", account.ID, "BTC", "1"); err != nil {
		t.Fatalf("deposit: %v", err)
	}
	if err := env.engine.Borrow(ctx, "user-1", account.ID, "RUSD", "10000"); err != nil {
		t.Fatalf("borrow: %v", err)
	}

	// Backdate the accrual clock by 30 days, then read the view, which accrues.
	past := time.Now().UTC().AddDate(0, 0, -30)
	if err := env.db.Model(&models.MarginBorrow{}).Where("account_id = ?", account.ID).
		Update("last_accrual", past).Error; err != nil {
		t.Fatalf("backdate: %v", err)
	}

	view, err := env.engine.GetAccountView(ctx, "user-1", account.ID)
	if err != nil {
		t.Fatalf("view: %v", err)
	}
	if view.DebtValueUSD <= 10000 {
		t.Fatalf("expected interest to increase debt above 10000, got %v", view.DebtValueUSD)
	}

	// At 5% APY over 30 days the debt should be close to 10000*(1.05)^(30/365).
	expected := 10000 * math.Pow(1.05, 30.0/365.0)
	if math.Abs(view.DebtValueUSD-expected) > 1 {
		t.Fatalf("expected debt near %.4f, got %.4f", expected, view.DebtValueUSD)
	}

	cfg := &marginMarketConfig{BaseRateAPY: 0.05, KinkUtilization: 0.8, JumpMultiplier: 1.0}
	atKink := borrowRateAPY(cfg, 0.8)
	if math.Abs(atKink-0.05) > 1e-9 {
		t.Errorf("rate at kink should equal the base rate, got %v", atKink)
	}
	if borrowRateAPY(cfg, 0.9) <= atKink {
		t.Error("rate past the kink must exceed the base rate")
	}
	if got := borrowRateAPY(cfg, 1.0); math.Abs(got-1.05) > 1e-9 {
		t.Errorf("rate at full utilization should be base+jump, got %v", got)
	}
}

// TestMarginPausedListingIsNotTradable verifies pausing a listing stops new
// accounts and borrows without deleting existing data.
func TestMarginPausedListingIsNotTradable(t *testing.T) {
	ctx := context.Background()
	env := newMarginTestEnv(t, map[string]float64{"BTC": 60000, "RUSD": 1})
	env.seedMarginMarket(t, "BTC/RUSD", "BTC", "RUSD", 0.7, 0.75)

	account, err := env.engine.OpenAccount(ctx, "user-1", OpenAccountRequest{Pair: "BTC/RUSD"})
	if err != nil {
		t.Fatalf("open account: %v", err)
	}
	if err := env.db.Model(&models.MarginMarket{}).Where("pair = ?", "BTC/RUSD").
		Update("status", models.ListingStatusPaused).Error; err != nil {
		t.Fatalf("pause: %v", err)
	}

	if _, err := env.engine.OpenAccount(ctx, "user-2", OpenAccountRequest{Pair: "BTC/RUSD"}); !errors.Is(err, ErrMarginMarketNotListed) {
		t.Fatalf("paused listing must not accept new accounts, got %v", err)
	}
	if err := env.engine.DepositCollateral(ctx, "user-1", account.ID, "BTC", "1"); !errors.Is(err, ErrMarginMarketNotListed) {
		t.Fatalf("paused listing must not accept collateral, got %v", err)
	}
}
