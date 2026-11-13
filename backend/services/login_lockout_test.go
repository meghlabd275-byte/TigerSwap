package services

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"tigerswap/backend/models"
)

// TestLoginLocksAccountAfterRepeatedFailures proves a password-guessing run is
// bounded per account: once the threshold is reached, even the correct password
// is refused until the lock expires.
func TestLoginLocksAccountAfterRepeatedFailures(t *testing.T) {
	auth, db := newAuthHarness(t)
	const email = "lockout@tigerswap.local"
	const password = "Str0ng-Passw0rd!2026"
	seedUser(t, auth, email, password, true)

	for i := 0; i < loginMaxFailures; i++ {
		code := postLogin(t, auth, `{"email":"`+email+`","password":"wrong-password"}`).Code
		if code != http.StatusUnauthorized {
			t.Fatalf("attempt %d: want 401, got %d", i+1, code)
		}
	}

	var stored models.User
	if err := db.Where("email = ?", email).First(&stored).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if stored.LockedUntil == nil {
		t.Fatal("account must be locked after the failure threshold")
	}

	// The correct password must not unlock the account while the lock stands.
	response := postLogin(t, auth, `{"email":"`+email+`","password":"`+password+`"}`)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("locked account must refuse the correct password, got %d", response.Code)
	}
	if strings.Contains(response.Body.String(), "token") {
		t.Fatalf("a locked account must not be issued a token: %s", response.Body.String())
	}
}

// TestLoginClearsFailuresOnSuccess proves a successful sign-in resets the counter,
// so an occasional typo does not accumulate toward a lockout.
func TestLoginClearsFailuresOnSuccess(t *testing.T) {
	auth, db := newAuthHarness(t)
	const email = "clears@tigerswap.local"
	const password = "Str0ng-Passw0rd!2026"
	seedUser(t, auth, email, password, true)

	if code := postLogin(t, auth, `{"email":"`+email+`","password":"wrong-password"}`).Code; code != http.StatusUnauthorized {
		t.Fatalf("wrong password must be 401, got %d", code)
	}
	if code := postLogin(t, auth, `{"email":"`+email+`","password":"`+password+`"}`).Code; code != http.StatusOK {
		t.Fatalf("correct password must sign in, got %d", code)
	}

	var stored models.User
	if err := db.Where("email = ?", email).First(&stored).Error; err != nil {
		t.Fatalf("reload user: %v", err)
	}
	if stored.FailedLoginAttempts != 0 || stored.LockedUntil != nil {
		t.Fatalf("success must clear the counter and lock, got attempts=%d locked=%v", stored.FailedLoginAttempts, stored.LockedUntil)
	}
}

// TestLoginAllowsSignInAfterLockExpires proves the lock is time-bounded: once it
// has passed, the correct password signs in again.
func TestLoginAllowsSignInAfterLockExpires(t *testing.T) {
	auth, db := newAuthHarness(t)
	const email = "expired@tigerswap.local"
	const password = "Str0ng-Passw0rd!2026"
	seedUser(t, auth, email, password, true)

	past := time.Now().Add(-time.Minute)
	if err := db.Model(&models.User{}).Where("email = ?", email).Update("locked_until", past).Error; err != nil {
		t.Fatalf("set expired lock: %v", err)
	}
	if code := postLogin(t, auth, `{"email":"`+email+`","password":"`+password+`"}`).Code; code != http.StatusOK {
		t.Fatalf("an expired lock must allow sign-in, got %d", code)
	}
}
