package services

import (
	"fmt"
	"math/big"
	"strings"
	"testing"
	"time"

	"tigerswap/backend/models"

	"gorm.io/gorm"
)

// otcTestAssetAddress is the ledger address for a symbol in the OTC test
// registry; it is derived from the symbol so two assets cannot collide.
func otcTestAssetAddress(symbol string) string {
	return "0x00000000000000000000000000000000000000" + fmt.Sprintf("%02x", len(symbol))
}

// newOtcTestService builds an OTC service wired to a real ledger and a fixed
// asset registry, so an order's escrow and settlement move real balances.
func newOtcTestService(t *testing.T, db *gorm.DB) *OtcService {
	t.Helper()
	service := NewOtcService(db)
	service.SetLedger(NewLedgerService(db), 1)
	service.SetAssetResolver(func(asset string) (perpSettlement, error) {
		symbol := strings.ToUpper(strings.TrimSpace(asset))
		switch symbol {
		case "BTC", "RUSD":
			return perpSettlement{
				Symbol:   symbol,
				ChainID:  1,
				Address:  otcTestAssetAddress(symbol),
				Decimals: 8,
			}, nil
		default:
			return perpSettlement{}, fmt.Errorf("asset %s is not in the test registry", asset)
		}
	})
	return service
}

// fundOtcUser credits a user whole units of an asset so an order can escrow it.
func fundOtcUser(t *testing.T, db *gorm.DB, user, asset string, whole int64) {
	t.Helper()
	ledger := NewLedgerService(db)
	units := new(big.Int).Mul(big.NewInt(whole), pow10(8))
	if err := ledger.Credit(user, 1, otcTestAssetAddress(asset), asset, 8, units); err != nil {
		t.Fatalf("fund %s %s: %v", user, asset, err)
	}
}

// otcBalance returns a user's available balance for a test asset.
func otcBalance(t *testing.T, db *gorm.DB, user, asset string) *big.Int {
	t.Helper()
	available, err := NewLedgerService(db).Available(user, 1, otcTestAssetAddress(asset))
	if err != nil {
		t.Fatalf("available %s: %v", asset, err)
	}
	return available
}

// TestOtcCreateEscrowsMakerFunds verifies a sell escrows the base asset and a
// buy escrows the quote cost, so a taker is always promised committed funds.
func TestOtcCreateEscrowsMakerFunds(t *testing.T) {
	db := newBotsTestDB(t)
	service := newOtcTestService(t, db)
	fundOtcUser(t, db, "maker", "BTC", 10)
	fundOtcUser(t, db, "maker", "RUSD", 1000000)

	sell := `{"symbol":"BTC/RUSD","side":"sell","quantity":"2","price":"60000","min_fill":"2","expires_in_seconds":3600}`
	if status, response := callHandler(t, service.CreateOrder, "maker", "POST", "/otc/orders", sell, nil); status != 201 {
		t.Fatalf("sell order should be created, got %d: %s", status, response)
	}
	if got := otcBalance(t, db, "maker", "BTC"); got.Cmp(new(big.Int).Mul(big.NewInt(8), pow10(8))) != 0 {
		t.Fatalf("a 2 BTC sell should escrow 2 BTC, leaving 8: got %s", got)
	}

	buy := `{"symbol":"BTC/RUSD","side":"buy","quantity":"1","price":"60000","min_fill":"1","expires_in_seconds":3600}`
	if status, response := callHandler(t, service.CreateOrder, "maker", "POST", "/otc/orders", buy, nil); status != 201 {
		t.Fatalf("buy order should be created, got %d: %s", status, response)
	}
	// 1 BTC at 60000 escrows 60000 RUSD out of 1000000.
	wantRUSD := new(big.Int).Mul(big.NewInt(940000), pow10(8))
	if got := otcBalance(t, db, "maker", "RUSD"); got.Cmp(wantRUSD) != 0 {
		t.Fatalf("a 1 BTC buy at 60000 should escrow 60000 RUSD: got %s, want %s", got, wantRUSD)
	}
}

// TestOtcCreateRefusesUnfundedMaker verifies an order whose escrow cannot be
// funded is rejected rather than published.
func TestOtcCreateRefusesUnfundedMaker(t *testing.T) {
	db := newBotsTestDB(t)
	service := newOtcTestService(t, db)

	body := `{"symbol":"BTC/RUSD","side":"sell","quantity":"1","price":"60000","min_fill":"1","expires_in_seconds":3600}`
	status, _ := callHandler(t, service.CreateOrder, "broke", "POST", "/otc/orders", body, nil)
	if status == 201 {
		t.Fatal("an order with no escrowed funds must not be created")
	}
	var count int64
	if err := db.Model(&models.OtcOrder{}).Count(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Fatalf("a refused order must not be persisted, got %d rows", count)
	}
}

// TestOtcTakeSettlesBothLegs verifies a take delivers the base asset to the taker
// and the quote cost to the maker, moving real ledger balances.
func TestOtcTakeSettlesBothLegs(t *testing.T) {
	db := newBotsTestDB(t)
	service := newOtcTestService(t, db)
	fundOtcUser(t, db, "maker", "BTC", 10)
	fundOtcUser(t, db, "taker", "RUSD", 1000000)

	sell := `{"symbol":"BTC/RUSD","side":"sell","quantity":"2","price":"60000","min_fill":"1","expires_in_seconds":3600}`
	if status, response := callHandler(t, service.CreateOrder, "maker", "POST", "/otc/orders", sell, nil); status != 201 {
		t.Fatalf("sell order should be created, got %d: %s", status, response)
	}
	var order models.OtcOrder
	if err := db.First(&order).Error; err != nil {
		t.Fatalf("load order: %v", err)
	}

	status, response := callHandler(t, service.TakeOrder, "taker", "POST", "/otc/orders/1/take",
		`{"quantity":"2"}`, idParam(fmt.Sprintf("%d", order.ID)))
	if status != 200 {
		t.Fatalf("take should succeed, got %d: %s", status, response)
	}

	// The taker receives the 2 BTC that were escrowed.
	if got := otcBalance(t, db, "taker", "BTC"); got.Cmp(new(big.Int).Mul(big.NewInt(2), pow10(8))) != 0 {
		t.Fatalf("taker should receive 2 BTC, got %s", got)
	}
	// The taker pays 120000 RUSD.
	wantTakerRUSD := new(big.Int).Mul(big.NewInt(880000), pow10(8))
	if got := otcBalance(t, db, "taker", "RUSD"); got.Cmp(wantTakerRUSD) != 0 {
		t.Fatalf("taker should pay 120000 RUSD, got %s", got)
	}
	// The maker receives the 120000 RUSD cost.
	wantMakerRUSD := new(big.Int).Mul(big.NewInt(120000), pow10(8))
	if got := otcBalance(t, db, "maker", "RUSD"); got.Cmp(wantMakerRUSD) != 0 {
		t.Fatalf("maker should receive 120000 RUSD, got %s", got)
	}
}

// TestOtcCancelRefundsEscrow verifies cancelling an open order returns the
// maker's escrow.
func TestOtcCancelRefundsEscrow(t *testing.T) {
	db := newBotsTestDB(t)
	service := newOtcTestService(t, db)
	fundOtcUser(t, db, "maker", "BTC", 10)

	sell := `{"symbol":"BTC/RUSD","side":"sell","quantity":"3","price":"60000","min_fill":"3","expires_in_seconds":3600}`
	if status, response := callHandler(t, service.CreateOrder, "maker", "POST", "/otc/orders", sell, nil); status != 201 {
		t.Fatalf("sell order should be created, got %d: %s", status, response)
	}
	var order models.OtcOrder
	if err := db.First(&order).Error; err != nil {
		t.Fatalf("load order: %v", err)
	}

	if status, response := callHandler(t, service.CancelOrder, "maker", "DELETE", "/otc/orders/1", "",
		idParam(fmt.Sprintf("%d", order.ID))); status != 200 {
		t.Fatalf("cancel should succeed, got %d: %s", status, response)
	}
	if got := otcBalance(t, db, "maker", "BTC"); got.Cmp(new(big.Int).Mul(big.NewInt(10), pow10(8))) != 0 {
		t.Fatalf("cancel should refund the full 3 BTC escrow, got %s", got)
	}
}

// TestOtcExpireRefundsEscrow verifies an expired order returns the maker's
// escrow rather than leaving it locked.
func TestOtcExpireRefundsEscrow(t *testing.T) {
	db := newBotsTestDB(t)
	service := newOtcTestService(t, db)
	fundOtcUser(t, db, "maker", "BTC", 10)

	sell := `{"symbol":"BTC/RUSD","side":"sell","quantity":"4","price":"60000","min_fill":"4","expires_in_seconds":3600}`
	if status, response := callHandler(t, service.CreateOrder, "maker", "POST", "/otc/orders", sell, nil); status != 201 {
		t.Fatalf("sell order should be created, got %d: %s", status, response)
	}

	// Push the order past its deadline, then expire it.
	if err := db.Model(&models.OtcOrder{}).Where("status = ?", "open").
		Update("expires_at", time.Now().UTC().Add(-time.Hour)).Error; err != nil {
		t.Fatalf("backdate expiry: %v", err)
	}
	expired, err := service.ExpireStaleOrders()
	if err != nil {
		t.Fatalf("expire: %v", err)
	}
	if expired != 1 {
		t.Fatalf("expected 1 expired order, got %d", expired)
	}
	if got := otcBalance(t, db, "maker", "BTC"); got.Cmp(new(big.Int).Mul(big.NewInt(10), pow10(8))) != 0 {
		t.Fatalf("expiry should refund the full 4 BTC escrow, got %s", got)
	}
}
