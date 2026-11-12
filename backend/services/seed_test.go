package services

import (
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"tigerswap/backend/models"
)

var seedTestDBSeq uint64

func newSeedTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s-%d?mode=memory&cache=shared", t.Name(), atomic.AddUint64(&seedTestDBSeq, 1))
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

	if err := db.AutoMigrate(&models.Blockchain{}, &models.Token{}); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}
	return db
}

// TestSeedChainsIsIdempotentAndReachesExistingDeployments pins the upgrade
// behaviour: the seed must not depend on an empty table, so a chain added to the
// registry (TigerSmartChain, 6666) lands on a database that already holds other
// chains, and a second boot does not duplicate or error.
func TestSeedChainsIsIdempotentAndReachesExistingDeployments(t *testing.T) {
	db := newSeedTestDB(t)

	// Simulate an existing deployment: one chain is already present, so the table
	// is not empty and the old count==0 guard would have skipped the whole seed.
	if err := db.Create(&models.Blockchain{ChainID: 1, Name: "Ethereum", Symbol: "ETH", Type: "evm", IsActive: true}).Error; err != nil {
		t.Fatalf("pre-existing chain: %v", err)
	}

	seedChains(db)

	var tsc models.Blockchain
	if err := db.Where("chain_id = ?", 6666).First(&tsc).Error; err != nil {
		t.Fatalf("TigerSmartChain missing after seed on populated db: %v", err)
	}
	if tsc.Symbol != "TGR" || tsc.Type != "evm" {
		t.Fatalf("unexpected TigerSmartChain row: %+v", tsc)
	}

	// A second boot must converge on the same rows, not duplicate or fail.
	seedChains(db)

	var count int64
	if err := db.Model(&models.Blockchain{}).Where("chain_id = ?", 6666).Count(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected exactly one TigerSmartChain row, got %d", count)
	}
}

// TestSeedTokensIsIdempotentAndReachesExistingDeployments pins the same
// behaviour for tokens, including the operator-declared TigerSmartChain ERC-20s.
func TestSeedTokensIsIdempotentAndReachesExistingDeployments(t *testing.T) {
	db := newSeedTestDB(t)

	if err := db.Create(&models.Token{ChainID: 1, Address: "0x0000000000000000000000000000000000000000", Name: "Ethereum", Symbol: "ETH", Decimals: 18, IsActive: true}).Error; err != nil {
		t.Fatalf("pre-existing token: %v", err)
	}

	t.Setenv("TIGERSWAP_TOKEN_6666_RUSD", "0x00000000000000000000000000000000000000a1")
	seedTokens(db)

	var rusd models.Token
	if err := db.Where("chain_id = ? AND address = ?", 6666, "0x00000000000000000000000000000000000000a1").First(&rusd).Error; err != nil {
		t.Fatalf("operator-declared RUSD missing after seed on populated db: %v", err)
	}
	if rusd.Symbol != "RUSD" {
		t.Fatalf("unexpected RUSD row: %+v", rusd)
	}

	seedTokens(db)

	var count int64
	if err := db.Model(&models.Token{}).Where("chain_id = ? AND address = ?", 6666, "0x00000000000000000000000000000000000000a1").Count(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected exactly one RUSD row, got %d", count)
	}
}
