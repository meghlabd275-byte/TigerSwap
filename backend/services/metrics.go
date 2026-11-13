package services

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
)

// Metrics is a small, dependency-free Prometheus text exposition endpoint.
//
// The service deliberately keeps no third-party metrics library: a counter and a
// gauge are a few atomics, and the exposition format is a documented text
// protocol. Two kinds of series are exposed:
//
//   - Process and request counters accumulated in memory.
//   - Readiness probes registered by the services that own a dependency, so the
//     readiness endpoint reports the real state of the database and the chain
//     client rather than a hard-coded "ok".
//
// A label value is never taken from a request path (which is unbounded); the
// route template is used instead, so a scanner cannot grow the metric set.
type Metrics struct {
	started time.Time

	requestsTotal   map[string]*uint64
	requestDuration map[string]*durationSum
	mu              sync.RWMutex

	inFlight int64

	// probes are named readiness checks. Each returns nil when ready.
	probesMu sync.RWMutex
	probes   map[string]func() error
}

type durationSum struct {
	totalNanos uint64
	count      uint64
}

// NewMetrics builds an empty registry stamped with the process start time.
func NewMetrics() *Metrics {
	return &Metrics{
		started:         time.Now(),
		requestsTotal:   make(map[string]*uint64),
		requestDuration: make(map[string]*durationSum),
		probes:          make(map[string]func() error),
	}
}

// counter returns the counter for a (method, route, status) key, creating it on
// first use. The map is guarded so concurrent requests do not race.
func (m *Metrics) counter(method, route, status string) *uint64 {
	key := method + "\x00" + route + "\x00" + status
	m.mu.RLock()
	c, ok := m.requestsTotal[key]
	m.mu.RUnlock()
	if ok {
		return c
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if c, ok := m.requestsTotal[key]; ok {
		return c
	}
	c = new(uint64)
	m.requestsTotal[key] = c
	return c
}

func (m *Metrics) duration(method, route string) *durationSum {
	key := method + "\x00" + route
	m.mu.RLock()
	d, ok := m.requestDuration[key]
	m.mu.RUnlock()
	if ok {
		return d
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if d, ok := m.requestDuration[key]; ok {
		return d
	}
	d = &durationSum{}
	m.requestDuration[key] = d
	return d
}

// Middleware records every request by route template. The route template is read
// after the handler runs, so an unmatched path collapses to a single series.
func (m *Metrics) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		atomic.AddInt64(&m.inFlight, 1)
		c.Next()
		atomic.AddInt64(&m.inFlight, -1)

		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		status := fmt.Sprintf("%d", c.Writer.Status())
		atomic.AddUint64(m.counter(c.Request.Method, route, status), 1)

		d := m.duration(c.Request.Method, route)
		m.mu.Lock()
		d.totalNanos += uint64(time.Since(start).Nanoseconds())
		d.count++
		m.mu.Unlock()
	}
}

// RegisterProbe attaches a named readiness check. A probe that returns an error
// makes the readiness endpoint report not-ready with the probe's message.
func (m *Metrics) RegisterProbe(name string, check func() error) {
	m.probesMu.Lock()
	defer m.probesMu.Unlock()
	m.probes[name] = check
}

// Handler serves the Prometheus text exposition.
func (m *Metrics) Handler(c *gin.Context) {
	var b strings.Builder

	writeHelp := func(name, help, kind string) {
		fmt.Fprintf(&b, "# HELP %s %s\n", name, help)
		fmt.Fprintf(&b, "# TYPE %s %s\n", name, kind)
	}
	writeHelp("tigerswap_uptime_seconds", "Seconds since the process started.", "gauge")
	fmt.Fprintf(&b, "tigerswap_uptime_seconds %d\n", int64(time.Since(m.started).Seconds()))

	writeHelp("tigerswap_http_requests_in_flight", "Requests currently being served.", "gauge")
	fmt.Fprintf(&b, "tigerswap_http_requests_in_flight %d\n", atomic.LoadInt64(&m.inFlight))

	writeHelp("tigerswap_http_requests_total", "Total HTTP requests by route, method and status.", "counter")
	writeHelp("tigerswap_http_request_duration_seconds", "Cumulative request duration by route and method.", "summary")

	// Deterministic order so the output is diffable and cacheable.
	m.mu.RLock()
	keys := make([]string, 0, len(m.requestsTotal))
	for key := range m.requestsTotal {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	type row struct {
		method, route, status string
		count                 uint64
		seconds               float64
		sumCount              uint64
	}
	rows := make([]row, 0, len(keys))
	for _, key := range keys {
		parts := strings.Split(key, "\x00")
		method, route, status := parts[0], parts[1], parts[2]
		r := row{method: method, route: route, status: status, count: atomic.LoadUint64(m.requestsTotal[key])}
		if d, ok := m.requestDuration[method+"\x00"+route]; ok {
			r.seconds = float64(d.totalNanos) / float64(time.Second)
			r.sumCount = d.count
		}
		rows = append(rows, r)
	}
	m.mu.RUnlock()

	for _, r := range rows {
		fmt.Fprintf(&b, "tigerswap_http_requests_total{method=%q,route=%q,status=%q} %d\n", r.method, r.route, r.status, r.count)
	}
	for _, r := range rows {
		if r.sumCount == 0 {
			continue
		}
		fmt.Fprintf(&b, "tigerswap_http_request_duration_seconds_sum{method=%q,route=%q} %g\n", r.method, r.route, r.seconds)
		fmt.Fprintf(&b, "tigerswap_http_request_duration_seconds_count{method=%q,route=%q} %d\n", r.method, r.route, r.sumCount)
	}

	writeHelp("tigerswap_readiness_probe_success", "1 when a named readiness probe passes, 0 otherwise.", "gauge")
	m.probesMu.RLock()
	names := make([]string, 0, len(m.probes))
	for name := range m.probes {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		value := 1
		if err := m.probes[name](); err != nil {
			value = 0
		}
		fmt.Fprintf(&b, "tigerswap_readiness_probe_success{probe=%q} %d\n", name, value)
	}
	m.probesMu.RUnlock()

	c.Data(http.StatusOK, "text/plain; version=0.0.4; charset=utf-8", []byte(b.String()))
}

// LivenessHandler reports that the process is running. It never touches a
// dependency, so a transient database outage does not restart a healthy process.
func (m *Metrics) LivenessHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "alive"})
}

// ReadinessHandler reports whether every registered dependency is usable. A
// failing probe is listed so an operator can see which one blocked readiness.
func (m *Metrics) ReadinessHandler(c *gin.Context) {
	m.probesMu.RLock()
	names := make([]string, 0, len(m.probes))
	for name := range m.probes {
		names = append(names, name)
	}
	sort.Strings(names)
	failures := make(map[string]string)
	for _, name := range names {
		if err := m.probes[name](); err != nil {
			failures[name] = err.Error()
		}
	}
	m.probesMu.RUnlock()

	if len(failures) > 0 {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready", "failures": failures})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ready"})
}
