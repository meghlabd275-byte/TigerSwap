package services

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"tigerswap/backend/models"
)

var adminTestDBSeq uint64

// newAdminTestDB builds an isolated in-memory database migrated with the tables
// the admin read endpoints touch.
func newAdminTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s-%d?mode=memory&cache=shared", t.Name(), atomic.AddUint64(&adminTestDBSeq, 1))
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

	if err := db.AutoMigrate(&models.Blockchain{}, &models.Token{}, &models.SystemSettings{}, &models.AdminLog{}); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}
	return db
}

func TestGetChainsIncludesInactiveChains(t *testing.T) {
	db := newAdminTestDB(t)
	svc := NewAdminService(db)

	if err := db.Create(&models.Blockchain{ChainID: 1, Name: "Ethereum", Symbol: "ETH", Type: "evm", IsActive: true}).Error; err != nil {
		t.Fatalf("seed chain: %v", err)
	}
	if err := db.Create(&models.Blockchain{ChainID: 56, Name: "BSC", Symbol: "BNB", Type: "evm", IsActive: false}).Error; err != nil {
		t.Fatalf("seed chain: %v", err)
	}

	rec := getJSON(t, svc.GetChains, "/api/v1/admin/chains")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Chains []models.Blockchain `json:"chains"`
		Count  int                 `json:"count"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Count != 2 || len(payload.Chains) != 2 {
		t.Fatalf("expected both chains including the inactive one, got count=%d len=%d", payload.Count, len(payload.Chains))
	}
	if payload.Chains[0].ChainID != 1 || payload.Chains[1].ChainID != 56 {
		t.Fatalf("expected chains ordered by chain_id, got %+v", payload.Chains)
	}
}

func TestGetTokensIncludesInactiveTokens(t *testing.T) {
	db := newAdminTestDB(t)
	svc := NewAdminService(db)

	if err := db.Create(&models.Token{Address: "0xaaa", ChainID: 1, Name: "Tether", Symbol: "USDT", Decimals: 6, IsActive: true, TotalSupply: "1000"}).Error; err != nil {
		t.Fatalf("seed token: %v", err)
	}
	if err := db.Create(&models.Token{Address: "0xbbb", ChainID: 1, Name: "Scam", Symbol: "SCM", Decimals: 18, IsActive: false, TotalSupply: "0"}).Error; err != nil {
		t.Fatalf("seed token: %v", err)
	}

	rec := getJSON(t, svc.GetTokens, "/api/v1/admin/tokens")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Tokens []models.Token `json:"tokens"`
		Count  int            `json:"count"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Count != 2 {
		t.Fatalf("expected both tokens including the inactive one, got %d", payload.Count)
	}
}

func TestGetSettingsCreatesDefaultRowOnFirstUse(t *testing.T) {
	db := newAdminTestDB(t)
	svc := NewAdminService(db)

	rec := getJSON(t, svc.GetSettings, "/api/v1/admin/settings")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var settings models.SystemSettings
	if err := json.Unmarshal(rec.Body.Bytes(), &settings); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if settings.Key != "default" {
		t.Fatalf("expected the default settings key, got %q", settings.Key)
	}
	if !settings.TradingEnabled || !settings.WithdrawalEnabled || !settings.DepositEnabled || !settings.NewUserRegistration {
		t.Fatalf("expected permissive defaults, got %+v", settings)
	}
	if settings.MaintenanceMode {
		t.Fatalf("expected maintenance mode to be off by default")
	}

	var rows int64
	db.Model(&models.SystemSettings{}).Count(&rows)
	if rows != 1 {
		t.Fatalf("expected exactly one settings row, got %d", rows)
	}
}

func TestGetLogsReturnsNewestFirst(t *testing.T) {
	db := newAdminTestDB(t)
	svc := NewAdminService(db)

	for _, action := range []string{"create_chain", "update_config", "enable_maintenance"} {
		if err := db.Create(&models.AdminLog{Action: action, EntityType: "system"}).Error; err != nil {
			t.Fatalf("seed log: %v", err)
		}
	}

	rec := getJSON(t, svc.GetLogs, "/api/v1/admin/logs")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Logs  []models.AdminLog `json:"logs"`
		Count int               `json:"count"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Count != 3 {
		t.Fatalf("expected 3 logs, got %d", payload.Count)
	}
	if payload.Logs[0].Action != "enable_maintenance" {
		t.Fatalf("expected newest log first, got %q", payload.Logs[0].Action)
	}
}
