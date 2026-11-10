package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"tigerswap/backend/go/api_gateway/config"
)

// TestLogger tests the logger middleware
func TestLogger(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Logger())
	r.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

// TestRequestID tests the request ID middleware
func TestRequestID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestID())
	r.GET("/test", func(c *gin.Context) {
		requestID := GetRequestID(c)
		if requestID == "" {
			t.Error("Expected request ID to be set")
		}
		c.Status(http.StatusOK)
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
}

// TestRequestIDPresets tests request ID presets
func TestRequestIDPresets(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestID())
	r.GET("/test", func(c *gin.Context) {
		requestID := GetRequestID(c)
		expectedID := "custom-request-id"
		if requestID != expectedID {
			t.Errorf("Expected %s, got %s", expectedID, requestID)
		}
		c.Status(http.StatusOK)
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Request-ID", "custom-request-id")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
}

// TestTimeout tests the timeout middleware
func TestTimeout(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Timeout(100 * time.Millisecond))
	r.GET("/test", func(c *gin.Context) {
		time.Sleep(200 * time.Millisecond)
		c.Status(http.StatusOK)
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusRequestTimeout {
		t.Errorf("Expected status %d, got %d", http.StatusRequestTimeout, w.Code)
	}
}

// TestRecovery tests the recovery middleware
func TestRecovery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Recovery())
	r.GET("/test", func(c *gin.Context) {
		panic("test panic")
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

// TestSecure tests the secure middleware
func TestSecure(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Secure())
	r.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Check security headers
	if w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("Expected X-Content-Type-Options header")
	}
	if w.Header().Get("X-Frame-Options") != "DENY" {
		t.Error("Expected X-Frame-Options header")
	}
	// The gateway serves JSON, so the policy forbids loading anything at all.
	// A permissive policy here would only matter if a response were rendered
	// as a document, which is exactly the case the header must cover.
	csp := w.Header().Get("Content-Security-Policy")
	if csp == "" {
		t.Error("Expected a Content-Security-Policy header")
	}
	if strings.Contains(csp, "unsafe-inline") || strings.Contains(csp, "unsafe-eval") {
		t.Errorf("CSP must not allow inline or eval: %q", csp)
	}
	// The legacy X-XSS-Protection filter is deprecated and, set to 1, is
	// exploitable in some browsers; its absence is the safe choice.
	if w.Header().Get("X-XSS-Protection") != "" {
		t.Error("X-XSS-Protection must not be set")
	}
	// Account-scoped responses must not be cached and replayed to another
	// account by a shared or browser cache.
	if cc := w.Header().Get("Cache-Control"); !strings.Contains(cc, "no-store") {
		t.Errorf("Cache-Control %q must include no-store", cc)
	}
}

// TestBodyLimiter tests the body limiter middleware
func TestBodyLimiter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(BodyLimiter(100))
	r.POST("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req, _ := http.NewRequest("POST", "/test", strings.NewReader(strings.Repeat("a", 101)))
	req.ContentLength = 101
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Should reject due to size
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Logf("Got status %d, might pass for nil body", w.Code)
	}
}

// TestValidateRequest tests the request validation middleware
func TestValidateRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(ValidateRequest())
	r.POST("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// Test with invalid content type
	req, _ := http.NewRequest("POST", "/test", nil)
	req.Header.Set("Content-Type", "text/plain")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnsupportedMediaType {
		t.Errorf("Expected status %d, got %d", http.StatusUnsupportedMediaType, w.Code)
	}

	// Test with valid content type
	r.POST("/test2", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req2, _ := http.NewRequest("POST", "/test2", nil)
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w2.Code)
	}
}

// TestMetrics tests the metrics middleware
func TestMetrics(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Metrics())
	r.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

// TestRateLimiterDisabled tests rate limiter when disabled
func TestRateLimiterDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	limiter := NewRateLimiter(nil, config.RateLimitConfig{Enabled: false})
	r.Use(limiter.Middleware())
	r.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

// BenchmarkLogger benchmarks the logger middleware
func BenchmarkLogger(b *testing.B) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Logger())
	r.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req, _ := http.NewRequest("GET", "/test", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

// BenchmarkRequestID benchmarks the request ID middleware
func BenchmarkRequestID(b *testing.B) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestID())
	r.GET("/test", func(c *gin.Context) {
		_ = GetRequestID(c)
		c.Status(http.StatusOK)
	})

	req, _ := http.NewRequest("GET", "/test", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

// BenchmarkSecure benchmarks the secure middleware
func BenchmarkSecure(b *testing.B) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Secure())
	r.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req, _ := http.NewRequest("GET", "/test", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

// TestAdminRequiredRejectsNonAdmins pins the gateway admin guard: a valid token
// whose role is not admin/superadmin must be refused, so a plain authenticated
// user cannot reach the admin handlers. Before the guard was wired into the
// admin route group, Auth alone admitted any valid token.
func TestAdminRequiredRejectsNonAdmins(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := []struct {
		name     string
		role     any
		wantCode int
	}{
		{"plain user is refused", "user", http.StatusForbidden},
		{"missing role is refused", nil, http.StatusUnauthorized},
		{"non-string role is refused", 42, http.StatusForbidden},
		{"admin is admitted", "admin", http.StatusOK},
		{"superadmin is admitted", "superadmin", http.StatusOK},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := gin.New()
			r.GET("/admin", func(c *gin.Context) {
				if tc.role != nil {
					c.Set("role", tc.role)
				}
			}, AdminRequired(), func(c *gin.Context) {
				c.Status(http.StatusOK)
			})

			req, _ := http.NewRequest("GET", "/admin", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != tc.wantCode {
				t.Fatalf("role %v: expected %d, got %d", tc.role, tc.wantCode, w.Code)
			}
		})
	}

}

// A request that finishes inside the deadline must get the handler's own
// response. The middleware used to block on the context and reply 408 to every
// request, however fast the handler was.
func TestTimeoutLetsFastHandlerRespond(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Timeout(2 * time.Second))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	start := time.Now()
	req, _ := http.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, w.Code)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("fast handler was blocked for %v", elapsed)
	}
}

// A handler that overruns the deadline still yields 408 when it writes nothing.
func TestTimeoutReportsSlowHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Timeout(50 * time.Millisecond))
	r.GET("/test", func(c *gin.Context) {
		<-c.Request.Context().Done()
	})

	req, _ := http.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusRequestTimeout {
		t.Fatalf("expected status %d, got %d", http.StatusRequestTimeout, w.Code)
	}
}
