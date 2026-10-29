package services

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"tigerswap/backend/models"
)

var adminWriteTestDBSeq uint64

// newAdminWriteTestDB migrates the tables the admin write endpoints touch.
func newAdminWriteTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s-%d?mode=memory&cache=shared", t.Name(), atomic.AddUint64(&adminWriteTestDBSeq, 1))
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

	if err := db.AutoMigrate(&models.Blockchain{}, &models.Token{}, &models.AdminLog{}); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}
	return db
}

// postJSON drives a handler with a JSON body and a test admin identity.
func postJSONBody(t *testing.T, handler gin.HandlerFunc, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	handler(c)
	return rec
}

// TestCreateChainRejectsDuplicateChainID verifies a repeated chain_id is
// reported as a conflict. The column is unique, so without the explicit check
// the caller sees an opaque 500 instead of an actionable message.
func TestCreateChainRejectsDuplicateChainID(t *testing.T) {
	db := newAdminWriteTestDB(t)
	svc := NewAdminService(db)

	first := postJSONBody(t, svc.CreateChain, "/api/v1/admin/chains",
		`{"chain_id":9004,"name":"TigerChain","symbol":"TGR","type":"evm","is_active":true}`)
	if first.Code != http.StatusCreated {
		t.Fatalf("expected first create to succeed, got %d: %s", first.Code, first.Body.String())
	}

	second := postJSONBody(t, svc.CreateChain, "/api/v1/admin/chains",
		`{"chain_id":9004,"name":"Duplicate","symbol":"DUP","type":"evm"}`)
	if second.Code != http.StatusConflict {
		t.Fatalf("expected 409 for duplicate chain_id, got %d: %s", second.Code, second.Body.String())
	}

	var count int64
	db.Model(&models.Blockchain{}).Where("chain_id = ?", 9004).Count(&count)
	if count != 1 {
		t.Fatalf("expected exactly one chain with chain_id 9004, got %d", count)
	}
}

// TestCreateTokenRejectsDuplicateAddressOnChain verifies the same address cannot
// be listed twice on one chain, while remaining valid on another chain.
func TestCreateTokenRejectsDuplicateAddressOnChain(t *testing.T) {
	db := newAdminWriteTestDB(t)
	svc := NewAdminService(db)

	const addr = "0x0000000000000000000000000000000000000002"
	first := postJSONBody(t, svc.CreateToken, "/api/v1/admin/tokens",
		`{"address":"`+addr+`","chain_id":1,"name":"Royal Tiger USD","symbol":"RUSD","decimals":18,"is_active":true}`)
	if first.Code != http.StatusCreated {
		t.Fatalf("expected first create to succeed, got %d: %s", first.Code, first.Body.String())
	}

	duplicate := postJSONBody(t, svc.CreateToken, "/api/v1/admin/tokens",
		`{"address":"`+addr+`","chain_id":1,"name":"Royal Tiger USD","symbol":"RUSD","decimals":18}`)
	if duplicate.Code != http.StatusConflict {
		t.Fatalf("expected 409 for duplicate token on the same chain, got %d: %s", duplicate.Code, duplicate.Body.String())
	}

	// The same address on a different chain is a different asset and must pass.
	otherChain := postJSONBody(t, svc.CreateToken, "/api/v1/admin/tokens",
		`{"address":"`+addr+`","chain_id":137,"name":"Royal Tiger USD","symbol":"RUSD","decimals":18}`)
	if otherChain.Code != http.StatusCreated {
		t.Fatalf("expected same address on another chain to succeed, got %d: %s", otherChain.Code, otherChain.Body.String())
	}
}

// TestLogAdminActionPersistsWithoutDetails guards the audit trail for actions
// that carry no payload. The details column is jsonb, which cannot store an
// empty string, so a nil payload must be written as SQL NULL.
func TestLogAdminActionPersistsWithoutDetails(t *testing.T) {
	db := newAdminWriteTestDB(t)
	svc := NewAdminService(db)

	postJSONBody(t, svc.CreateChain, "/api/v1/admin/chains",
		`{"chain_id":4242,"name":"TigerChain","symbol":"TGR","type":"evm","is_active":true}`)

	var logs []models.AdminLog
	if err := db.Find(&logs).Error; err != nil {
		t.Fatalf("load audit logs: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected the chain creation to be audited, got %d entries", len(logs))
	}
	if logs[0].Action != "create_chain" {
		t.Fatalf("expected action create_chain, got %q", logs[0].Action)
	}
	if logs[0].Details != nil {
		t.Fatalf("expected nil details for a payload-free action, got %q", *logs[0].Details)
	}
}

// TestLogAdminActionStoresDetails verifies structured payloads survive the
// round trip so the audit trail stays useful for forensics.
func TestLogAdminActionStoresDetails(t *testing.T) {
	db := newAdminWriteTestDB(t)
	svc := NewAdminService(db)

	rec := postJSONBody(t, svc.CreateChain, "/api/v1/admin/chains",
		`{"chain_id":4243,"name":"TigerChain","symbol":"TGR","type":"evm","is_active":true}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	gin.SetMode(gin.TestMode)
	rec2 := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec2)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/chains", nil)
	svc.logAdminAction(c, "custom_action", "chain", "TigerChain", map[string]any{"chain_id": 4243})

	var entry models.AdminLog
	if err := db.Where("action = ?", "custom_action").First(&entry).Error; err != nil {
		t.Fatalf("load audit entry: %v", err)
	}
	if entry.Details == nil {
		t.Fatal("expected details to be stored")
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(*entry.Details), &decoded); err != nil {
		t.Fatalf("details are not valid json: %v", err)
	}
	if decoded["chain_id"] != float64(4243) {
		t.Fatalf("expected chain_id 4243 in details, got %v", decoded["chain_id"])
	}
}
