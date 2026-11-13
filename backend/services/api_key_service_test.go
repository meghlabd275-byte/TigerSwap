package services

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"tigerswap/backend/models"
)

func extractStringField(t *testing.T, body []byte, field string) string {
	t.Helper()
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	value, _ := payload[field].(string)
	return value
}

func ginParam(key, value string) gin.Params {
	return gin.Params{{Key: key, Value: value}}
}

func newAPIKeyEnv(t *testing.T) (*APIKeyService, *gorm.DB) {
	t.Helper()
	db := newListingTestDB(t)
	if err := db.AutoMigrate(&models.DeveloperAPIKey{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return NewAPIKeyService(db), db
}

// TestAPIKeyPlaintextIsNeverStored proves the database holds only a hash, so a
// dump cannot be replayed.
func TestAPIKeyPlaintextIsNeverStored(t *testing.T) {
	svc, db := newAPIKeyEnv(t)

	rec := runAuthenticatedHandler(t, svc.CreateAPIKey, "user-1", map[string]interface{}{
		"name":   "bot",
		"scopes": "read,trade",
	}, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	plaintext := extractStringField(t, rec.Body.Bytes(), "key")
	if plaintext == "" {
		t.Fatal("the created key must be returned once")
	}

	var stored models.DeveloperAPIKey
	if err := db.First(&stored).Error; err != nil {
		t.Fatalf("load: %v", err)
	}
	if stored.KeyHash == plaintext {
		t.Fatal("the plaintext must not be stored as the hash")
	}
	if stored.KeyHash == "" {
		t.Fatal("the hash must be stored")
	}

	// The plaintext authenticates; a wrong value does not.
	key, err := svc.Authenticate(plaintext)
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	if key.UserID != "user-1" {
		t.Fatalf("user id = %s, want user-1", key.UserID)
	}
	if _, err := svc.Authenticate("tsk_not-the-key"); err == nil {
		t.Fatal("an unknown key must be rejected")
	}
}

func TestAPIKeyScopeValidation(t *testing.T) {
	svc, _ := newAPIKeyEnv(t)

	rec := runAuthenticatedHandler(t, svc.CreateAPIKey, "user-1", map[string]interface{}{
		"name":   "bot",
		"scopes": "read,teleport",
	}, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("an unknown scope must be rejected, got %d: %s", rec.Code, rec.Body.String())
	}

	// An empty scope list defaults to read-only.
	rec = runAuthenticatedHandler(t, svc.CreateAPIKey, "user-1", map[string]interface{}{
		"name": "read-only",
	}, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	if got := extractStringField(t, rec.Body.Bytes(), "scopes"); got != "read" {
		t.Fatalf("default scopes = %q, want read", got)
	}
}

func TestAPIKeyExpiryIsHonoured(t *testing.T) {
	svc, db := newAPIKeyEnv(t)

	rec := runAuthenticatedHandler(t, svc.CreateAPIKey, "user-1", map[string]interface{}{
		"name": "expired",
	}, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	plaintext := extractStringField(t, rec.Body.Bytes(), "key")

	// An expiry in the past must not be accepted at creation.
	rec = runAuthenticatedHandler(t, svc.CreateAPIKey, "user-1", map[string]interface{}{
		"name":       "past",
		"expires_at": "2000-01-01T00:00:00Z",
	}, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("a past expiry must be rejected, got %d", rec.Code)
	}

	// Move the stored key's expiry into the past; authentication must fail.
	past := "2000-01-01T00:00:00Z"
	if err := db.Model(&models.DeveloperAPIKey{}).Where("key_hash <> ''").
		Update("expires_at", past).Error; err != nil {
		t.Fatalf("update: %v", err)
	}
	if _, err := svc.Authenticate(plaintext); err == nil {
		t.Fatal("an expired key must be rejected")
	}
}

func TestAPIKeyRevocationIsImmediate(t *testing.T) {
	svc, db := newAPIKeyEnv(t)

	rec := runAuthenticatedHandler(t, svc.CreateAPIKey, "user-1", map[string]interface{}{
		"name": "bot",
	}, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	plaintext := extractStringField(t, rec.Body.Bytes(), "key")
	id := extractStringField(t, rec.Body.Bytes(), "id")

	rec = runAuthenticatedHandler(t, svc.RevokeAPIKey, "user-1", nil, ginParam("id", id))
	if rec.Code != http.StatusOK {
		t.Fatalf("revoke: %d %s", rec.Code, rec.Body.String())
	}

	if _, err := svc.Authenticate(plaintext); err == nil {
		t.Fatal("a revoked key must be rejected")
	}
	_ = db
}

// TestAPIKeyCannotBeRevokedByAnotherUser proves the caller can only revoke their
// own keys.
func TestAPIKeyCannotBeRevokedByAnotherUser(t *testing.T) {
	svc, _ := newAPIKeyEnv(t)

	rec := runAuthenticatedHandler(t, svc.CreateAPIKey, "user-1", map[string]interface{}{
		"name": "bot",
	}, nil)
	id := extractStringField(t, rec.Body.Bytes(), "id")

	rec = runAuthenticatedHandler(t, svc.RevokeAPIKey, "user-2", nil, ginParam("id", id))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("another user must not revoke the key, got %d", rec.Code)
	}
}

func TestAPIKeyHasScope(t *testing.T) {
	wildcard := &models.DeveloperAPIKey{Scopes: "*"}
	if !HasAPIKeyScope(wildcard, "withdraw") {
		t.Fatal("a wildcard key must hold every scope")
	}
	readOnly := &models.DeveloperAPIKey{Scopes: "read"}
	if HasAPIKeyScope(readOnly, "trade") {
		t.Fatal("a read-only key must not hold trade")
	}
	if !HasAPIKeyScope(readOnly, "read") {
		t.Fatal("a read-only key must hold read")
	}
}
