package services

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"tigerswap/backend/models"

	"github.com/gin-gonic/gin"
)

// newLiquidatablePerpEngine builds an engine with one listed market whose mark
// has already crossed a long position's liquidation price, so the liquidation
// entry point has real work to do.
func newLiquidatablePerpEngine() *PerpetualEngine {
	engine := newMarkedPerpEngine(map[string]float64{"BTC": 90})
	engine.markets["BTC/RUSD"].LiquidationFeeRate = 0.005
	engine.positions["p1"] = &PerpetualPosition{
		ID:               "p1",
		UserAddress:      "0xabc",
		Pair:             "BTC/RUSD",
		Side:             "long",
		Size:             "10000",
		Collateral:       "2000",
		EntryPrice:       100,
		LiquidationPrice: 95,
		Status:           "liquidatable",
	}
	return engine
}

// TestClosePositionDoesNotDeadlock is the regression test for a self-deadlock:
// ClosePosition took the write lock and then called getMarkPrice, which takes
// the read lock through market(). Every real close hung forever.
func TestClosePositionDoesNotDeadlock(t *testing.T) {
	engine := newMarkedPerpEngine(map[string]float64{"BTC": 110})
	engine.positions["p1"] = &PerpetualPosition{
		ID: "p1", UserAddress: "0xabc", Pair: "BTC/RUSD", Side: "long",
		Size: "10000", Collateral: "2000", EntryPrice: 100, LiquidationPrice: 50, Status: "open",
	}

	done := make(chan error, 1)
	go func() {
		_, err := engine.ClosePosition(context.Background(), "0xabc", "p1")
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("close: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ClosePosition did not return; it deadlocked on the engine lock")
	}

	if engine.positions["p1"].Status != "closed" {
		t.Fatalf("position should be closed, got %q", engine.positions["p1"].Status)
	}
}

// TestLiquidatePositionDoesNotDeadlock is the same regression for the
// settlement path, which called market() and getMarkPrice() under the write
// lock.
func TestLiquidatePositionDoesNotDeadlock(t *testing.T) {
	engine := newLiquidatablePerpEngine()

	done := make(chan error, 1)
	go func() {
		_, err := engine.LiquidatePosition(context.Background(), "p1")
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("liquidate: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("LiquidatePosition did not return; it deadlocked on the engine lock")
	}

	if engine.positions["p1"].Status != "liquidated" {
		t.Fatalf("position should be settled, got %q", engine.positions["p1"].Status)
	}
}

// TestLiquidatePositionHandlerSettlesFlaggedPosition verifies the operator
// endpoint settles a genuinely liquidatable position and reports the fee.
func TestLiquidatePositionHandlerSettlesFlaggedPosition(t *testing.T) {
	engine := newLiquidatablePerpEngine()

	status, body := callHandler(t, engine.LiquidatePositionHandler, "1", http.MethodPost,
		"/perps/positions/p1/liquidate", "", gin.Params{{Key: "id", Value: "p1"}})
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", status, body)
	}
	if engine.positions["p1"].Status != "liquidated" {
		t.Fatalf("position should be settled, got status %q", engine.positions["p1"].Status)
	}
	var resp map[string]any
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["liquidation_fee"] == nil || resp["liquidation_fee"] == "" {
		t.Fatalf("response must report the liquidation fee: %s", body)
	}
}

// TestLiquidatePositionHandlerRefusesHealthyPosition verifies the endpoint
// cannot be used to close a healthy position and seize its margin.
func TestLiquidatePositionHandlerRefusesHealthyPosition(t *testing.T) {
	engine := newMarkedPerpEngine(map[string]float64{"BTC": 110})
	engine.positions["p1"] = &PerpetualPosition{
		ID: "p1", UserAddress: "0xabc", Pair: "BTC/RUSD", Side: "long",
		Size: "10000", EntryPrice: 100, LiquidationPrice: 50, Status: "open",
	}

	status, _ := callHandler(t, engine.LiquidatePositionHandler, "1", http.MethodPost,
		"/perps/positions/p1/liquidate", "", gin.Params{{Key: "id", Value: "p1"}})
	if status != http.StatusBadRequest {
		t.Fatalf("a healthy position must not be liquidated, got %d", status)
	}
	if engine.positions["p1"].Status != "open" {
		t.Fatalf("a refused liquidation must leave the position untouched, got %q", engine.positions["p1"].Status)
	}
}

// TestListLiquidatablePositionsHandlerReportsFlaggedOnly verifies the operator
// list contains exactly the positions the risk engine has flagged.
func TestListLiquidatablePositionsHandlerReportsFlaggedOnly(t *testing.T) {
	engine := newLiquidatablePerpEngine()

	status, body := callHandler(t, engine.ListLiquidatablePositionsHandler, "1", http.MethodGet,
		"/perps/liquidatable", "", nil)
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", status, body)
	}
	var resp struct {
		Count     int `json:"count"`
		Positions []struct {
			ID string `json:"id"`
		} `json:"positions"`
	}
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Count != 1 || len(resp.Positions) != 1 || resp.Positions[0].ID != "p1" {
		t.Fatalf("expected only position p1, got %s", body)
	}
}

// TestLiquidateAccountHandlerRejectsMalformedID verifies a non-UUID account id
// is rejected before any database work happens.
func TestLiquidateAccountHandlerRejectsMalformedID(t *testing.T) {
	env := newMarginTestEnv(t, map[string]float64{"BTC": 60000, "RUSD": 1})

	status, _ := callHandler(t, env.engine.LiquidateAccountHandler, "1", http.MethodPost,
		"/margin/accounts/not-a-uuid/liquidate", "", gin.Params{{Key: "id", Value: "not-a-uuid"}})
	if status != http.StatusBadRequest {
		t.Fatalf("a malformed account id must be rejected, got %d", status)
	}
}

// TestLiquidateAccountHandlerRefusesHealthyAccount verifies the operator
// endpoint refuses a solvent account.
func TestLiquidateAccountHandlerRefusesHealthyAccount(t *testing.T) {
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

	status, _ := callHandler(t, env.engine.LiquidateAccountHandler, "1", http.MethodPost,
		"/margin/accounts/x/liquidate", "", gin.Params{{Key: "id", Value: account.ID.String()}})
	if status != http.StatusBadRequest {
		t.Fatalf("a solvent account must not be liquidated, got %d", status)
	}
}

// TestLiquidateAccountHandlerSettlesUnderCollateralizedAccount verifies the
// endpoint drives a real liquidation when the account is genuinely underwater.
func TestLiquidateAccountHandlerSettlesUnderCollateralizedAccount(t *testing.T) {
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

	prices["BTC"] = 30000

	status, body := callHandler(t, env.engine.LiquidateAccountHandler, "1", http.MethodPost,
		"/margin/accounts/x/liquidate", "", gin.Params{{Key: "id", Value: account.ID.String()}})
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", status, body)
	}
	if !strings.Contains(body, `"success":true`) {
		t.Fatalf("response must report success: %s", body)
	}
}

// TestExpireStaleOrdersHandlerReportsCount verifies the operator endpoint
// expires open orders past their deadline and reports how many.
func TestExpireStaleOrdersHandlerReportsCount(t *testing.T) {
	db := newBotsTestDB(t)
	service := NewOtcService(db)

	stale := models.OtcOrder{
		MakerID: "1", Symbol: "BTC/RUSD", Side: "sell", Quantity: "1", Price: "60000",
		MinFill: "1", Status: "open", ExpiresAt: time.Now().UTC().Add(-time.Minute),
	}
	if err := db.Create(&stale).Error; err != nil {
		t.Fatalf("seed stale: %v", err)
	}

	status, body := callHandler(t, service.ExpireStaleOrdersHandler, "1", http.MethodPost,
		"/otc/expire", "", nil)
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", status, body)
	}
	var resp struct {
		Expired int64 `json:"expired"`
	}
	if err := json.Unmarshal([]byte(body), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Expired != 1 {
		t.Fatalf("expected 1 expired order, got %d (%s)", resp.Expired, body)
	}
}
