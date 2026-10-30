package services

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"tigerswap/backend/models"
)

func newAuthHarness(t *testing.T) (*AuthService, *gorm.DB) {
	t.Helper()
	t.Setenv("JWT_SECRET", "register-validation-test-secret")
	db := newListingTestDB(t)
	if err := db.AutoMigrate(&models.User{}, &models.RevokedToken{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return NewAuthService(db), db
}

func postRegister(t *testing.T, auth *AuthService, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/register", auth.Register)

	request := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

// The username is echoed back to users, so markup must never be accepted.
func TestRegisterRejectsUnsafeUsername(t *testing.T) {
	auth, _ := newAuthHarness(t)

	for _, bad := range []string{
		"<script>alert(1)</script>",
		"user name",
		"user;drop",
		"ab",
		"\" onmouseover=\"x",
		"emoji_\U0001F600",
		"nul\x00byte",
		"user\nnewline",
	} {
		payload, _ := json.Marshal(map[string]string{
			"email":    "unsafe@tigerswap.local",
			"username": bad,
			"password": "Str0ng-Passw0rd!2026",
		})
		response := postRegister(t, auth, string(payload))
		if response.Code != http.StatusBadRequest {
			t.Fatalf("username %q must be rejected, got status %d", bad, response.Code)
		}
	}
}

func TestRegisterAcceptsSafeUsername(t *testing.T) {
	auth, _ := newAuthHarness(t)

	payload, _ := json.Marshal(map[string]string{
		"email":    "good@tigerswap.local",
		"username": "tiger_trader-01.v2",
		"password": "Str0ng-Passw0rd!2026",
	})
	response := postRegister(t, auth, string(payload))
	if response.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", response.Code, response.Body.String())
	}
}

// bcrypt silently truncates at 72 bytes, so longer passwords must be refused
// rather than accepted and quietly ignored past the limit.
func TestRegisterRejectsOverlongPassword(t *testing.T) {
	auth, _ := newAuthHarness(t)

	payload, _ := json.Marshal(map[string]string{
		"email":    "long@tigerswap.local",
		"username": "longpass",
		"password": strings.Repeat("a", 73),
	})
	response := postRegister(t, auth, string(payload))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("a 73-byte password must be rejected, got %d", response.Code)
	}
}

// A duplicate username must produce a clean 400, not a 500 from the unique index.
func TestRegisterRejectsDuplicateUsername(t *testing.T) {
	auth, _ := newAuthHarness(t)

	first, _ := json.Marshal(map[string]string{
		"email":    "first@tigerswap.local",
		"username": "duplicate",
		"password": "Str0ng-Passw0rd!2026",
	})
	if response := postRegister(t, auth, string(first)); response.Code != http.StatusCreated {
		t.Fatalf("first registration failed: %d %s", response.Code, response.Body.String())
	}

	second, _ := json.Marshal(map[string]string{
		"email":    "second@tigerswap.local",
		"username": "duplicate",
		"password": "Str0ng-Passw0rd!2026",
	})
	response := postRegister(t, auth, string(second))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("duplicate username must return 400, got %d: %s", response.Code, response.Body.String())
	}
}
