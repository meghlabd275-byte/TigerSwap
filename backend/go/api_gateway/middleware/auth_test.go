package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"tigerswap/backend/go/api_gateway/config"
)

const testJWTSecret = "test-secret-that-is-long-enough-for-hs256"

// mintToken signs an HS256 token with the supplied claims.
func mintToken(t *testing.T, claims jwt.MapClaims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(testJWTSecret))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}
	return signed
}

// The auth service puts the account id in a `user_id` claim and never sets
// `sub`. The gateway used to read only `claims.Subject`, so every token minted
// by the platform produced an empty identity in context and the account-scoped
// handlers rejected it. This pins the claim the gateway must honour.
func TestAuthResolvesUserIDClaim(t *testing.T) {
	gin.SetMode(gin.TestMode)

	token := mintToken(t, jwt.MapClaims{
		"user_id": "11111111-1111-1111-1111-111111111111",
		"role":    "user",
		"exp":     time.Now().Add(time.Hour).Unix(),
	})

	r := gin.New()
	r.Use(Auth(&config.Config{JWT: config.JWTConfig{Secret: testJWTSecret}}))
	r.GET("/whoami", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"user_id": c.GetString("user_id"), "role": c.GetString("role")})
	})

	req, _ := http.NewRequest("GET", "/whoami", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "11111111-1111-1111-1111-111111111111") {
		t.Fatalf("expected the user_id claim in context, got %s", body)
	}
}

// A token whose only identity is `sub` still resolves, so a token minted by a
// component that uses the registered claim is not rejected.
func TestAuthFallsBackToSubject(t *testing.T) {
	gin.SetMode(gin.TestMode)

	token := mintToken(t, jwt.MapClaims{
		"sub": "22222222-2222-2222-2222-222222222222",
		"exp": time.Now().Add(time.Hour).Unix(),
	})

	r := gin.New()
	r.Use(Auth(&config.Config{JWT: config.JWTConfig{Secret: testJWTSecret}}))
	r.GET("/whoami", func(c *gin.Context) {
		c.String(http.StatusOK, c.GetString("user_id"))
	})

	req, _ := http.NewRequest("GET", "/whoami", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "22222222-2222-2222-2222-222222222222") {
		t.Fatalf("expected the sub claim to be used, got %s", w.Body.String())
	}
}

// A validly signed token that carries no identity at all must be refused rather
// than admitted with an empty user id.
func TestAuthRejectsTokenWithoutIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)

	token := mintToken(t, jwt.MapClaims{"exp": time.Now().Add(time.Hour).Unix()})

	r := gin.New()
	r.Use(Auth(&config.Config{JWT: config.JWTConfig{Secret: testJWTSecret}}))
	r.GET("/whoami", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req, _ := http.NewRequest("GET", "/whoami", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}
