package services

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestMetricsHandlerExposesRequests proves a served request appears in the
// exposition with its route template, method and status.
func TestMetricsHandlerExposesRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	metrics := NewMetrics()

	router := gin.New()
	router.Use(metrics.Middleware())
	router.GET("/api/v1/ping", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	router.GET("/metrics", metrics.Handler)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/ping", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("ping: %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := rec.Body.String()

	if !strings.Contains(body, `tigerswap_http_requests_total{method="GET",route="/api/v1/ping",status="200"} 1`) {
		t.Fatalf("the request counter must be exposed, got:\n%s", body)
	}
	if !strings.Contains(body, "tigerswap_http_request_duration_seconds_count") {
		t.Fatalf("the duration summary must be exposed, got:\n%s", body)
	}
	if !strings.Contains(body, "tigerswap_uptime_seconds") {
		t.Fatalf("uptime must be exposed, got:\n%s", body)
	}
}

// TestMetricsRouteTemplateBoundsCardinality proves two different ids collapse to
// one series under the route template rather than growing the metric set.
func TestMetricsRouteTemplateBoundsCardinality(t *testing.T) {
	gin.SetMode(gin.TestMode)
	metrics := NewMetrics()

	router := gin.New()
	router.Use(metrics.Middleware())
	router.GET("/api/v1/item/:id", func(c *gin.Context) { c.Status(http.StatusOK) })
	router.GET("/metrics", metrics.Handler)

	for _, id := range []string{"1", "2", "3"} {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/item/"+id, nil))
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := rec.Body.String()

	if !strings.Contains(body, `route="/api/v1/item/:id"`) {
		t.Fatalf("the route template must label the series, got:\n%s", body)
	}
	if strings.Contains(body, `route="/api/v1/item/1"`) {
		t.Fatalf("a raw id must not become a label value, got:\n%s", body)
	}
	if !strings.Contains(body, `tigerswap_http_requests_total{method="GET",route="/api/v1/item/:id",status="200"} 3`) {
		t.Fatalf("three requests must aggregate into one series, got:\n%s", body)
	}
}

// TestMetricsUnmatchedPathIsOneSeries proves a scanner hitting random paths does
// not grow the metric set.
func TestMetricsUnmatchedPathIsOneSeries(t *testing.T) {
	gin.SetMode(gin.TestMode)
	metrics := NewMetrics()

	router := gin.New()
	router.Use(metrics.Middleware())
	router.GET("/metrics", metrics.Handler)

	for _, path := range []string{"/a", "/b", "/c"} {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := rec.Body.String()

	if !strings.Contains(body, `route="unmatched"`) {
		t.Fatalf("an unmatched path must collapse to one series, got:\n%s", body)
	}
	if !strings.Contains(body, `route="unmatched",status="404"} 3`) {
		t.Fatalf("three unmatched requests must aggregate, got:\n%s", body)
	}
}

// TestReadinessReflectsProbes proves a failing probe reports not-ready with the
// failing name, and a passing one reports ready.
func TestReadinessReflectsProbes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	metrics := NewMetrics()
	metrics.RegisterProbe("database", func() error { return nil })
	metrics.RegisterProbe("redis", func() error { return errors.New("connection refused") })

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	metrics.ReadinessHandler(c)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when a probe fails, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "redis") {
		t.Fatalf("the failing probe must be named, got %s", rec.Body.String())
	}

	// A passing set reports ready.
	metrics.RegisterProbe("redis", func() error { return nil })
	rec = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(rec)
	metrics.ReadinessHandler(c)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 when every probe passes, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestLivenessIgnoresProbes proves liveness stays green while a dependency is
// down, so a transient outage does not restart a healthy process.
func TestLivenessIgnoresProbes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	metrics := NewMetrics()
	metrics.RegisterProbe("database", func() error { return errors.New("down") })

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	metrics.LivenessHandler(c)
	if rec.Code != http.StatusOK {
		t.Fatalf("liveness must stay green while a dependency is down, got %d", rec.Code)
	}
}

// TestMetricsProbeSeries proves the readiness state is exported as a gauge.
func TestMetricsProbeSeries(t *testing.T) {
	gin.SetMode(gin.TestMode)
	metrics := NewMetrics()
	metrics.RegisterProbe("database", func() error { return nil })
	metrics.RegisterProbe("redis", func() error { return errors.New("down") })

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	metrics.Handler(c)
	body := rec.Body.String()

	if !strings.Contains(body, `tigerswap_readiness_probe_success{probe="database"} 1`) {
		t.Fatalf("a passing probe must export 1, got:\n%s", body)
	}
	if !strings.Contains(body, `tigerswap_readiness_probe_success{probe="redis"} 0`) {
		t.Fatalf("a failing probe must export 0, got:\n%s", body)
	}
}

// TestMetricsConcurrentRequests proves the counters are race-free under load.
func TestMetricsConcurrentRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	metrics := NewMetrics()

	router := gin.New()
	router.Use(metrics.Middleware())
	router.GET("/api/v1/hit", func(c *gin.Context) { c.Status(http.StatusOK) })
	router.GET("/metrics", metrics.Handler)

	done := make(chan struct{})
	for i := 0; i < 8; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			for j := 0; j < 25; j++ {
				rec := httptest.NewRecorder()
				router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/hit", nil))
			}
		}()
	}
	for i := 0; i < 8; i++ {
		<-done
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if !strings.Contains(rec.Body.String(), `route="/api/v1/hit",status="200"} 200`) {
		t.Fatalf("expected 200 aggregated requests, got:\n%s", rec.Body.String())
	}
}
