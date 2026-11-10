package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestSecurityHeadersAreNonStorable pins the hardening headers that every
// response must carry. The Cache-Control header is the security-relevant part:
// the API returns account-scoped data (balances, positions, keys), so a shared
// or browser cache must never retain one account's payload and replay it to
// another.
func TestSecurityHeadersAreNonStorable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(SecurityHeaders())
	router.GET("/account", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"balance": "1"}) })

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/account", nil)
	router.ServeHTTP(rec, req)

	cacheControl := rec.Header().Get("Cache-Control")
	if cacheControl == "" {
		t.Fatal("Cache-Control header is missing; account data could be cached")
	}
	for _, directive := range []string{"no-store", "no-cache", "must-revalidate", "private"} {
		if !strings.Contains(cacheControl, directive) {
			t.Errorf("Cache-Control %q is missing %q", cacheControl, directive)
		}
	}

	required := map[string]string{
		"X-Content-Type-Options":       "nosniff",
		"X-Frame-Options":              "DENY",
		"Referrer-Policy":              "no-referrer",
		"Cross-Origin-Opener-Policy":   "same-origin",
		"Cross-Origin-Resource-Policy": "same-origin",
		"Pragma":                       "no-cache",
	}
	for header, want := range required {
		if got := rec.Header().Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}

	if csp := rec.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "default-src 'none'") {
		t.Errorf("Content-Security-Policy %q is not the strict JSON-only policy", csp)
	}
}
