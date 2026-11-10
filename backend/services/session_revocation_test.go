package services

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const sessionTestSecret = "register-validation-test-secret-0123456789"

// TestDeactivationRevokesExistingSessions proves that revoking a user's sessions
// invalidates a token that was already issued, that the revocation survives a
// service restart (i.e. it is persisted), and that a token minted afterwards is
// still accepted.
func TestDeactivationRevokesExistingSessions(t *testing.T) {
	auth, db := newAuthHarness(t)

	userID := uuid.New()
	token, err := auth.generateToken(userID, "trader@tigerswap.local", "user")
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	if _, err := auth.ValidateToken(token); err != nil {
		t.Fatalf("a freshly issued token must validate: %v", err)
	}

	if err := auth.RevokeUserSessions(userID.String()); err != nil {
		t.Fatalf("revoke sessions: %v", err)
	}
	if _, err := auth.ValidateToken(token); err == nil {
		t.Fatalf("a revoked session must no longer validate")
	}

	// The revocation is persisted, so a restarted service still refuses it.
	restarted := NewAuthService(db)
	if _, err := restarted.ValidateToken(token); err == nil {
		t.Fatalf("the revocation must survive a service restart")
	}

	// A token minted after the revocation carries a later iat and stays valid.
	future := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"jti":     uuid.NewString(),
		"user_id": userID.String(),
		"email":   "trader@tigerswap.local",
		"role":    "user",
		"iat":     time.Now().Add(time.Hour).Unix(),
		"exp":     time.Now().Add(2 * time.Hour).Unix(),
	})
	signed, err := future.SignedString([]byte(sessionTestSecret))
	if err != nil {
		t.Fatalf("sign future token: %v", err)
	}
	if _, err := restarted.ValidateToken(signed); err != nil {
		t.Fatalf("a token issued after the revocation must validate: %v", err)
	}
}
