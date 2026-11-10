package services

import (
	"strings"
	"testing"
)

// TestNewAuthServiceRejectsShortSecret proves a short JWT secret is refused at
// startup. A short secret is brute-forceable offline from one captured token,
// which would let an attacker mint a token for any account.
func TestNewAuthServiceRejectsShortSecret(t *testing.T) {
	t.Setenv("JWT_SECRET", "too-short")

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("NewAuthService accepted a short JWT secret")
		}
		if msg, ok := r.(string); !ok || !strings.Contains(msg, "at least") {
			t.Fatalf("unexpected panic value: %v", r)
		}
	}()

	_ = NewAuthService(nil)
}

// TestNewAuthServiceRejectsMissingSecret proves an unset secret still fails hard.
func TestNewAuthServiceRejectsMissingSecret(t *testing.T) {
	t.Setenv("JWT_SECRET", "")

	defer func() {
		if recover() == nil {
			t.Fatal("NewAuthService accepted an empty JWT secret")
		}
	}()

	_ = NewAuthService(nil)
}
