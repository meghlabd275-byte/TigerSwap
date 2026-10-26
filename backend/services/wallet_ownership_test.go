package services

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"tigerswap/backend/models"
)

// newWalletTestDB builds an in-memory database for wallet ownership tests.
func newWalletTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	return newListingTestDB(t)
}

// runAuthenticatedHandler invokes a handler with a session user id set, the way
// the auth middleware does, and an optional JSON body.
func runAuthenticatedHandler(t *testing.T, handler gin.HandlerFunc, sessionUserID string, body interface{}, params gin.Params) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)

	var reader *bytes.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", reader)
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = params
	if sessionUserID != "" {
		c.Set("user_id", sessionUserID)
	}
	handler(c)
	return rec
}

// TestClosePerpetualPositionEnforcesOwnership proves one account cannot close
// another account's position.
func TestClosePerpetualPositionEnforcesOwnership(t *testing.T) {
	db := newWalletTestDB(t)
	if err := db.AutoMigrate(&models.PerpetualPosition{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	svc := &WalletService{db: db}

	victim := models.PerpetualPosition{
		UserID: "victim-user",
		Pair:   "BTC/RUSD",
		Side:   "long",
		Status: "open",
	}
	if err := db.Create(&victim).Error; err != nil {
		t.Fatalf("seed position: %v", err)
	}

	rec := runAuthenticatedHandler(t, svc.ClosePerpetualPosition, "attacker-user", nil,
		gin.Params{{Key: "id", Value: victim.ID.String()}})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("attacker must not see or close another user's position, got %d: %s", rec.Code, rec.Body.String())
	}

	var stored models.PerpetualPosition
	db.Where("id = ?", victim.ID).First(&stored)
	if stored.Status != "open" {
		t.Fatalf("position must remain open, got %q", stored.Status)
	}
}

// TestGetPerpetualPositionsScopesToCaller proves the listing endpoint never
// returns another account's positions, even when a user_id query is supplied.
func TestGetPerpetualPositionsScopesToCaller(t *testing.T) {
	db := newWalletTestDB(t)
	if err := db.AutoMigrate(&models.PerpetualPosition{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	svc := &WalletService{db: db}

	db.Create(&models.PerpetualPosition{UserID: "user-a", Pair: "BTC/RUSD", Status: "open"})
	db.Create(&models.PerpetualPosition{UserID: "user-b", Pair: "ETH/RUSD", Status: "open"})

	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/?user_id=user-b", nil)
	c.Set("user_id", "user-a")
	svc.GetPerpetualPositions(c)

	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected status %d", rec.Code)
	}
	var payload struct {
		Positions []models.PerpetualPosition `json:"positions"`
		Count     int                        `json:"count"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload.Count != 1 || payload.Positions[0].UserID != "user-a" {
		t.Fatalf("expected only the caller's position, got %+v", payload.Positions)
	}
}

// TestGetCopyTradesScopesToCaller proves the copy-trade listing is scoped to
// the authenticated account rather than an arbitrary address.
func TestGetCopyTradesScopesToCaller(t *testing.T) {
	db := newWalletTestDB(t)
	if err := db.AutoMigrate(&models.CopyTrade{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	svc := &WalletService{db: db}

	db.Create(&models.CopyTrade{FollowerID: "user-a", TraderAddress: "0x1", Pair: "BTC/RUSD"})
	db.Create(&models.CopyTrade{FollowerID: "user-b", TraderAddress: "0x2", Pair: "ETH/RUSD"})

	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/?address=0x2", nil)
	c.Set("user_id", "user-a")
	svc.GetCopyTrades(c)

	var payload struct {
		Trades []models.CopyTrade `json:"trades"`
		Count  int                `json:"count"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload.Count != 1 || payload.Trades[0].FollowerID != "user-a" {
		t.Fatalf("expected only the caller's copy trades, got %+v", payload.Trades)
	}
}

// TestGetPerpetualPositionsRejectsAnonymousCaller proves an unauthenticated
// context is rejected rather than returning the whole table.
func TestGetPerpetualPositionsRejectsAnonymousCaller(t *testing.T) {
	db := newWalletTestDB(t)
	if err := db.AutoMigrate(&models.PerpetualPosition{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	svc := &WalletService{db: db}
	db.Create(&models.PerpetualPosition{UserID: "user-a", Pair: "BTC/RUSD", Status: "open"})

	rec := runAuthenticatedHandler(t, svc.GetPerpetualPositions, "", nil, nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without a session, got %d", rec.Code)
	}
}
