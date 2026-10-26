package services

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"tigerswap/backend/models"
)

// TestGenerateAndValidateTokenRoundTrip proves a freshly issued token validates
// and carries the identifiers the API depends on.
func TestGenerateAndValidateTokenRoundTrip(t *testing.T) {
	db := newListingTestDB(t)
	t.Setenv("JWT_SECRET", "test-secret-for-revocation-suite")
	svc := NewAuthService(db)

	userID := uuid.New()
	token, err := svc.generateToken(userID, "trader@example.com", "user")
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	claims, err := svc.ValidateToken(token)
	if err != nil {
		t.Fatalf("token should validate: %v", err)
	}
	if claims["user_id"] != userID.String() {
		t.Fatalf("unexpected user_id claim: %v", claims["user_id"])
	}
	if claims["jti"] == "" || claims["jti"] == nil {
		t.Fatalf("token must carry a jti claim")
	}
}

// TestLogoutRevokesToken proves a logged-out token can no longer be used, both
// immediately and after the in-memory cache is dropped and reloaded from the
// database (i.e. the revocation survives a restart).
func TestLogoutRevokesToken(t *testing.T) {
	db := newListingTestDB(t)
	if err := db.AutoMigrate(&models.RevokedToken{}); err != nil {
		t.Fatalf("migrate revoked tokens: %v", err)
	}
	t.Setenv("JWT_SECRET", "test-secret-for-revocation-suite")
	svc := NewAuthService(db)

	token, err := svc.generateToken(uuid.New(), "trader@example.com", "user")
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	c.Request.Header.Set("Authorization", "Bearer "+token)
	svc.Logout(c)

	if rec.Code != http.StatusOK {
		t.Fatalf("logout failed: %d %s", rec.Code, rec.Body.String())
	}

	if _, err := svc.ValidateToken(token); err == nil {
		t.Fatalf("revoked token must not validate")
	}

	// Simulate a process restart: a brand-new service instance reads the
	// revocation from the database rather than from memory.
	restarted := NewAuthService(db)
	if _, err := restarted.ValidateToken(token); err == nil {
		t.Fatalf("revocation must survive a restart")
	}
}

// TestExpiredRevocationIsPruned proves the store does not retain revocations
// past the token's own expiry.
func TestExpiredRevocationIsPruned(t *testing.T) {
	db := newListingTestDB(t)
	if err := db.AutoMigrate(&models.RevokedToken{}); err != nil {
		t.Fatalf("migrate revoked tokens: %v", err)
	}
	store := newRevocationStore(db)

	if err := store.revoke("expired-jti", time.Now().Add(-time.Minute)); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if store.isRevoked("expired-jti") {
		t.Fatalf("an expired revocation must not be reported as active")
	}

	if err := store.revoke("live-jti", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if !store.isRevoked("live-jti") {
		t.Fatalf("a live revocation must be reported as active")
	}
}
