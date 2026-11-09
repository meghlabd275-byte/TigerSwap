package netutil

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestConfigureTrustedProxiesRejectsWildcard pins that the wildcard is refused
// rather than silently re-enabling gin's trust-everyone default.
func TestConfigureTrustedProxiesRejectsWildcard(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, raw := range []string{"*", "0.0.0.0/0", "::/0", "10.0.0.0/8, *"} {
		router := gin.New()
		if err := ConfigureTrustedProxies(router, raw); err == nil {
			t.Fatalf("ConfigureTrustedProxies(%q) should have failed", raw)
		}
	}
}

// TestConfigureTrustedProxiesIgnoresForwardedHeader proves a spoofed
// X-Forwarded-For does not change the client IP when no proxy is configured.
func TestConfigureTrustedProxiesIgnoresForwardedHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	if err := ConfigureTrustedProxies(router, ""); err != nil {
		t.Fatalf("configure: %v", err)
	}

	var seen []string
	router.GET("/", func(c *gin.Context) { seen = append(seen, c.ClientIP()) })

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "203.0.113.7:1234"
		req.Header.Set("X-Forwarded-For", "10.0.0."+strconv.Itoa(i))
		router.ServeHTTP(httptest.NewRecorder(), req)
	}

	for _, ip := range seen {
		if ip != "203.0.113.7" {
			t.Fatalf("client IP must come from the peer, got %q", ip)
		}
	}
}
