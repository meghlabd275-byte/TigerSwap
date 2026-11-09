package services

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"tigerswap/backend/models"
)

func postLogin(t *testing.T, auth *AuthService, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/login", auth.Login)

	request := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func seedUser(t *testing.T, auth *AuthService, email, password string, active bool) {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	user := models.User{
		Email:        email,
		Username:     strings.SplitN(email, "@", 2)[0],
		PasswordHash: string(hash),
		Role:         models.RoleUser,
		IsActive:     true,
	}
	if err := auth.db.Create(&user).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	// IsActive carries a `default:true` tag, so a zero value is replaced by the
	// default on insert; flip it explicitly to model a deactivated account.
	if !active {
		if err := auth.db.Model(&models.User{}).Where("id = ?", user.ID).Update("is_active", false).Error; err != nil {
			t.Fatalf("deactivate user: %v", err)
		}
	}
}

// A deactivated account must not be able to obtain a fresh session, even with
// the correct password. The response is identical to a wrong password so the
// endpoint does not disclose whether the account exists.
func TestLoginRejectsDeactivatedAccount(t *testing.T) {
	auth, _ := newAuthHarness(t)
	seedUser(t, auth, "banned@tigerswap.local", "Str0ng-Passw0rd!2026", false)

	response := postLogin(t, auth, `{"email":"banned@tigerswap.local","password":"Str0ng-Passw0rd!2026"}`)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("a deactivated account must be refused, got %d: %s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), "token") {
		t.Fatalf("no token may be issued to a deactivated account: %s", response.Body.String())
	}
}

func TestLoginIssuesTokenForActiveAccount(t *testing.T) {
	auth, _ := newAuthHarness(t)
	seedUser(t, auth, "trader@tigerswap.local", "Str0ng-Passw0rd!2026", true)

	response := postLogin(t, auth, `{"email":"trader@tigerswap.local","password":"Str0ng-Passw0rd!2026"}`)
	if response.Code != http.StatusOK {
		t.Fatalf("an active account should log in, got %d: %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "token") {
		t.Fatalf("login must return a token: %s", response.Body.String())
	}
}
