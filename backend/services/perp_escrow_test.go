package services

import (
	"context"
	"fmt"
	"math/big"
	"sync/atomic"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"tigerswap/backend/models"
)

var perpEscrowDBSeq uint64

// rusdTestAddress is the settlement token used by the escrow tests.
const rusdTestAddress = "0x00000000000000000000000000000000000000aa"

// newEscrowPerpEngine builds an engine backed by a real database and ledger, so
// the escrow and settlement paths run against actual balance rows rather than a
// stub. The settlement asset is a fixed RUSD registry on chain 1 with 8
// decimals, which keeps the expected unit counts easy to read.
func newEscrowPerpEngine(t *testing.T, prices map[string]float64) (*PerpetualEngine, *LedgerService) {
	t.Helper()
	dsn := fmt.Sprintf("file:perp-escrow-%s-%d?mode=memory&cache=shared", t.Name(), atomic.AddUint64(&perpEscrowDBSeq, 1))
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

	if err := db.AutoMigrate(&models.AccountBalance{}, &models.PerpetualPosition{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	ledger := NewLedgerService(db)
	engine := newMarkedPerpEngine(prices)
	engine.db = db
	engine.SetLedger(ledger, 1)
	engine.SetSettlementResolver(func(string) (perpSettlement, error) {
		return perpSettlement{Symbol: "RUSD", ChainID: 1, Address: rusdTestAddress, Decimals: 8}, nil
	})
	return engine, ledger
}

// fundTrader credits a trader a whole-number RUSD balance so an open can escrow
// against it.
func fundTrader(t *testing.T, ledger *LedgerService, userID string, whole int64) {
	t.Helper()
	amount := new(big.Int).Mul(big.NewInt(whole), pow10(8))
	if err := ledger.Credit(userID, 1, rusdTestAddress, "RUSD", 8, amount); err != nil {
		t.Fatalf("fund trader: %v", err)
	}
}

// balanceUnits returns the trader's available balance in base units.
func balanceUnits(t *testing.T, ledger *LedgerService, userID string) *big.Int {
	t.Helper()
	available, err := ledger.Available(userID, 1, rusdTestAddress)
	if err != nil {
		t.Fatalf("available: %v", err)
	}
	return available
}

// TestOpenPositionEscrowsMargin is the regression test for the fund-safety hole
// where a position could be opened without any balance being escrowed, letting a
// trader take on exposure the platform had no funds to settle.
func TestOpenPositionEscrowsMargin(t *testing.T) {
	engine, ledger := newEscrowPerpEngine(t, map[string]float64{"BTC": 100})
	fundTrader(t, ledger, "0xabc", 5000)

	before := balanceUnits(t, ledger, "0xabc")

	position, err := engine.OpenPosition(context.Background(), "0xabc", 1, "BTC/RUSD", "long", "10000", "2000", 10)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if position.Status != "open" {
		t.Fatalf("expected an open position, got %q", position.Status)
	}

	after := balanceUnits(t, ledger, "0xabc")
	escrowed := new(big.Int).Sub(before, after)
	want := new(big.Int).Mul(big.NewInt(2000), pow10(8))
	if escrowed.Cmp(want) != 0 {
		t.Fatalf("escrowed %s units, want %s", escrowed, want)
	}
}

// TestOpenPositionRefusesUnfundedTrader verifies a trader with no balance cannot
// open a position, so exposure is always backed by real funds.
func TestOpenPositionRefusesUnfundedTrader(t *testing.T) {
	engine, _ := newEscrowPerpEngine(t, map[string]float64{"BTC": 100})

	if _, err := engine.OpenPosition(context.Background(), "0xabc", 1, "BTC/RUSD", "long", "10000", "2000", 10); err == nil {
		t.Fatal("an unfunded trader must not be able to open a position")
	}
	if len(engine.positions) != 0 {
		t.Fatalf("a refused open must not leave a position on the book, got %d", len(engine.positions))
	}
}

// TestOpenPositionRefusesWithoutLedger verifies the engine refuses to open a
// position when no ledger is wired, rather than opening one that moves no funds.
func TestOpenPositionRefusesWithoutLedger(t *testing.T) {
	engine := newMarkedPerpEngine(map[string]float64{"BTC": 100})

	if _, err := engine.OpenPosition(context.Background(), "0xabc", 1, "BTC/RUSD", "long", "10000", "2000", 10); err == nil {
		t.Fatal("an engine with no ledger must refuse to open a position")
	}
}

// TestClosePositionReturnsMarginPlusProfit verifies closing a winning long
// credits the escrowed margin back plus the realised profit.
func TestClosePositionReturnsMarginPlusProfit(t *testing.T) {
	engine, ledger := newEscrowPerpEngine(t, map[string]float64{"BTC": 100})
	fundTrader(t, ledger, "0xabc", 5000)

	if _, err := engine.OpenPosition(context.Background(), "0xabc", 1, "BTC/RUSD", "long", "10000", "2000", 10); err != nil {
		t.Fatalf("open: %v", err)
	}

	// The position is marked at 110 on close, so a 10,000 notional long gains 1,000.
	engine.priceAgg.(*stubPriceAggregator).prices["BTC"] = 110
	position, err := engine.ClosePosition(context.Background(), "0xabc", enginePositionID(engine))
	if err != nil {
		t.Fatalf("close: %v", err)
	}
	if position.Status != "closed" {
		t.Fatalf("expected a closed position, got %q", position.Status)
	}

	// 5,000 funded, 2,000 escrowed, then 2,000 margin + 1,000 profit returned.
	got := balanceUnits(t, ledger, "0xabc")
	want := new(big.Int).Mul(big.NewInt(6000), pow10(8))
	if got.Cmp(want) != 0 {
		t.Fatalf("balance after close = %s, want %s", got, want)
	}
}

// TestClosePositionFloorsLossAtMargin verifies a loss larger than the margin
// returns nothing rather than a negative payout that would debit the trader.
func TestClosePositionFloorsLossAtMargin(t *testing.T) {
	engine, ledger := newEscrowPerpEngine(t, map[string]float64{"BTC": 100})
	fundTrader(t, ledger, "0xabc", 5000)

	if _, err := engine.OpenPosition(context.Background(), "0xabc", 1, "BTC/RUSD", "long", "10000", "2000", 10); err != nil {
		t.Fatalf("open: %v", err)
	}

	// A collapse to 1 loses 9,900 against a 2,000 margin: nothing is returned.
	engine.priceAgg.(*stubPriceAggregator).prices["BTC"] = 1
	if _, err := engine.ClosePosition(context.Background(), "0xabc", enginePositionID(engine)); err != nil {
		t.Fatalf("close: %v", err)
	}

	got := balanceUnits(t, ledger, "0xabc")
	want := new(big.Int).Mul(big.NewInt(3000), pow10(8))
	if got.Cmp(want) != 0 {
		t.Fatalf("balance after an underwater close = %s, want %s", got, want)
	}
}

// TestLiquidatePositionPaysTraderAndFeeAccount verifies a liquidation moves real
// funds: the trader's remaining margin is returned and the fee is credited to
// the platform fee account.
func TestLiquidatePositionPaysTraderAndFeeAccount(t *testing.T) {
	engine, ledger := newEscrowPerpEngine(t, map[string]float64{"BTC": 100})
	engine.markets["BTC/RUSD"].LiquidationFeeRate = 0.005
	fundTrader(t, ledger, "0xabc", 5000)

	if _, err := engine.OpenPosition(context.Background(), "0xabc", 1, "BTC/RUSD", "long", "10000", "2000", 10); err != nil {
		t.Fatalf("open: %v", err)
	}
	id := enginePositionID(engine)
	// The mark falls to 90, crossing the position's liquidation price, and the
	// risk engine flags it for settlement.
	engine.priceAgg.(*stubPriceAggregator).prices["BTC"] = 90
	engine.positions[id].Status = "liquidatable"

	fee, err := engine.LiquidatePosition(context.Background(), id)
	if err != nil {
		t.Fatalf("liquidate: %v", err)
	}

	// At mark 90 a 10,000 long loses 1,000: remaining = 2,000 - 1,000 = 1,000.
	// Fee = 1,000 * 0.5% = 5; the trader receives 995, the fee account 5. The
	// trader's balance is the 3,000 left after escrow plus the 995 payout.
	remaining := new(big.Rat).Sub(new(big.Rat).SetInt64(1000), fee)
	traderPayout := scaleToBaseUnits(remaining, 8)
	traderWant := new(big.Int).Add(new(big.Int).Mul(big.NewInt(3000), pow10(8)), traderPayout)
	feeWant := scaleToBaseUnits(fee, 8)

	if got := balanceUnits(t, ledger, "0xabc"); got.Cmp(traderWant) != 0 {
		t.Fatalf("trader balance = %s, want %s", got, traderWant)
	}
	feeAcct, err := ledger.Available(perpFeeAccount, 1, rusdTestAddress)
	if err != nil {
		t.Fatalf("fee account: %v", err)
	}
	if feeAcct.Cmp(feeWant) != 0 {
		t.Fatalf("fee account = %s, want %s", feeAcct, feeWant)
	}
	if feeAcct.Sign() <= 0 {
		t.Fatal("the liquidator fee must be a positive credit, not zero")
	}
}

// TestSettlementResolverRefusesUnlistedAsset verifies an open is refused when the
// listing's quote asset cannot be resolved, rather than settling in a guessed
// asset.
func TestSettlementResolverRefusesUnlistedAsset(t *testing.T) {
	engine, ledger := newEscrowPerpEngine(t, map[string]float64{"BTC": 100})
	fundTrader(t, ledger, "0xabc", 5000)
	engine.SetSettlementResolver(func(pair string) (perpSettlement, error) {
		return perpSettlement{}, fmt.Errorf("no settlement asset for %s", pair)
	})

	if _, err := engine.OpenPosition(context.Background(), "0xabc", 1, "BTC/RUSD", "long", "10000", "2000", 10); err == nil {
		t.Fatal("an unresolvable settlement asset must refuse the open")
	}
}

// enginePositionID returns the id of the single position on the book.
func enginePositionID(engine *PerpetualEngine) string {
	for id := range engine.positions {
		return id
	}
	return ""
}
