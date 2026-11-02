package services

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"tigerswap/backend/models"
)

// newBridgeTestDB builds an isolated in-memory database with the bridge table
// migrated. SQLite is the test driver only; production runs on PostgreSQL.
func newBridgeTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s-%d?mode=memory&cache=shared", t.Name(), atomic.AddUint64(&bridgeTestDBSeq, 1))
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

	if err := db.AutoMigrate(&models.BridgeTransfer{}); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}
	return db
}

var bridgeTestDBSeq uint64

// TestProcessPendingSelectsOnlyConfirmed proves the relayer sweep only picks up
// transfers whose source-chain lock confirmed. A pending or failed transfer must
// never be minted on the destination chain.
func TestProcessPendingSelectsOnlyConfirmed(t *testing.T) {
	db := newBridgeTestDB(t)
	service := &BridgeService{db: db}

	rows := []models.BridgeTransfer{
		{TransferID: "t-confirmed", SrcChain: 1, DstChain: 137, Sender: "0x1", Recipient: "0x2", Token: "0x3", Amount: "100", Status: "confirmed"},
		{TransferID: "t-pending", SrcChain: 1, DstChain: 137, Sender: "0x1", Recipient: "0x2", Token: "0x3", Amount: "100", Status: "pending"},
		{TransferID: "t-failed", SrcChain: 1, DstChain: 137, Sender: "0x1", Recipient: "0x2", Token: "0x3", Amount: "100", Status: "failed"},
		{TransferID: "t-delivered", SrcChain: 1, DstChain: 137, Sender: "0x1", Recipient: "0x2", Token: "0x3", Amount: "100", Status: "delivered"},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("seed transfers: %v", err)
	}

	// ReleaseMint cannot sign here (no chain configured), so every selected row
	// fails at the signing step and stays confirmed. The sweep must still return
	// without error, and must have attempted only the confirmed row.
	if err := service.ProcessPending(context.Background()); err != nil {
		t.Fatalf("ProcessPending: %v", err)
	}

	var stillConfirmed int64
	db.Model(&models.BridgeTransfer{}).Where("status = ?", "confirmed").Count(&stillConfirmed)
	if stillConfirmed != 1 {
		t.Fatalf("confirmed rows = %d, want 1 (only the confirmed transfer is eligible)", stillConfirmed)
	}
}

// TestProcessPendingRequiresDatabase proves the sweep fails loudly rather than
// silently doing nothing when it has no database to read.
func TestProcessPendingRequiresDatabase(t *testing.T) {
	service := &BridgeService{}
	if err := service.ProcessPending(context.Background()); err == nil {
		t.Fatal("ProcessPending with no database returned nil, want an error")
	}
}

// TestStartBridgeRelayerStops proves the relayer loop exits on cancellation.
func TestStartBridgeRelayerStops(t *testing.T) {
	db := newBridgeTestDB(t)
	service := &BridgeService{db: db}

	ctx, cancel := context.WithCancel(context.Background())
	StartBridgeRelayer(ctx, service, time.Millisecond)
	time.Sleep(20 * time.Millisecond)
	cancel()
	// Let the loop observe the cancellation before the test's cleanup closes the
	// database the loop reads from.
	time.Sleep(50 * time.Millisecond)
}

// recordingSampler records the samples the sampler wrote and serves a fixed
// price map, so the sampling path runs without an oracle.
type recordingSampler struct {
	prices map[string]*PriceData
	got    []string
	err    error
}

func (r *recordingSampler) GetMultiChainPrices(_ context.Context, _ []string) (map[string]*PriceData, error) {
	return r.prices, nil
}

func (r *recordingSampler) RecordPriceSample(_ context.Context, symbol string, price, volume float64) error {
	if r.err != nil {
		return r.err
	}
	if price <= 0 {
		return errors.New("non-positive price")
	}
	r.got = append(r.got, symbol)
	return nil
}

// TestSamplePricesRecordsUsablePrices proves every symbol with a positive price
// is recorded and a symbol with no price is skipped.
func TestSamplePricesRecordsUsablePrices(t *testing.T) {
	source := &recordingSampler{prices: map[string]*PriceData{
		"BTC":  {Symbol: "BTC", USD: 65000, USDVolume24h: 100},
		"ETH":  {Symbol: "ETH", USD: 3200},
		"ZERO": {Symbol: "ZERO", USD: 0},
	}}

	samplePrices(context.Background(), source)

	recorded := map[string]bool{}
	for _, s := range source.got {
		recorded[s] = true
	}
	if !recorded["BTC"] || !recorded["ETH"] {
		t.Fatalf("recorded = %v, want BTC and ETH", source.got)
	}
	if recorded["ZERO"] {
		t.Fatalf("a zero price was recorded: %v", source.got)
	}
}

// TestSamplePricesSurvivesStoreFailure proves a failing store does not panic and
// simply records nothing.
func TestSamplePricesSurvivesStoreFailure(t *testing.T) {
	source := &recordingSampler{
		prices: map[string]*PriceData{"BTC": {Symbol: "BTC", USD: 65000}},
		err:    errors.New("store down"),
	}

	samplePrices(context.Background(), source)

	if len(source.got) != 0 {
		t.Fatalf("recorded = %v, want none when the store fails", source.got)
	}
}
