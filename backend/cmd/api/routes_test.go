package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestRouterRegistersBotsOtcTwammRoutes pins the routes added for the bots,
// OTC and TWAMM services. These handlers existed but were unreachable; a
// regression that drops one of them leaves the feature inaccessible again, and
// gin panics at startup on a route conflict, so this also guards the tree.
func TestRouterRegistersBotsOtcTwammRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := setupRouter(apiDeps{})

	registered := make(map[string]bool, len(router.Routes()))
	for _, route := range router.Routes() {
		registered[route.Method+" "+route.Path] = true
	}

	want := []string{
		"GET /api/v1/bots",
		"GET /api/v1/bots/summary",
		"GET /api/v1/bots/strategies",
		"GET /api/v1/bots/:id",
		"GET /api/v1/bots/:id/orders",
		"GET /api/v1/bots/:id/performance",
		"POST /api/v1/bots",
		"PUT /api/v1/bots/:id/status",
		"DELETE /api/v1/bots/:id",

		"GET /api/v1/otc/orders",
		"GET /api/v1/otc/orders/open",
		"POST /api/v1/otc/orders",
		"POST /api/v1/otc/orders/:id/take",
		"DELETE /api/v1/otc/orders/:id",

		"GET /api/v1/twamm/orders",
		"POST /api/v1/twamm/orders",
		"DELETE /api/v1/twamm/orders/:id",

		"GET /api/v1/farms",
		"GET /api/v1/farms/all",
		"GET /api/v1/farms/stakes",
		"GET /api/v1/farms/:id",
		"POST /api/v1/farms/:id/stake",
		"POST /api/v1/farms/:id/unstake",
		"POST /api/v1/farms/:id/claim",
		"POST /api/v1/farms/:id/fund",
		"GET /api/v1/admin/farms",
		"POST /api/v1/admin/farms",
		"PUT /api/v1/admin/farms/:id",
		"DELETE /api/v1/admin/farms/:id",

		"GET /api/v1/airdrops",
		"GET /api/v1/airdrops/all",
		"GET /api/v1/airdrops/allocations",
		"POST /api/v1/airdrops/:id/claim",
		"GET /api/v1/admin/airdrops",
		"POST /api/v1/admin/airdrops",
		"PUT /api/v1/admin/airdrops/:id",
		"DELETE /api/v1/admin/airdrops/:id",
		"POST /api/v1/admin/airdrops/:id/allocations",
		"GET /api/v1/admin/airdrops/:id/allocations",
		"POST /api/v1/admin/airdrops/:id/fund",
		"GET /api/v1/options/series",
		"GET /api/v1/options/series/all",
		"GET /api/v1/options/series/:id",
		"GET /api/v1/options/positions",
		"POST /api/v1/options/series/:id/buy",
		"POST /api/v1/options/series/:id/collateral",
		"POST /api/v1/options/series/:id/collateral/remove",
		"POST /api/v1/options/series/:id/exercise",
		"POST /api/v1/options/series/:id/withdraw",
		"GET /api/v1/admin/options/series",
		"POST /api/v1/admin/options/series",
		"GET /api/v1/admin/options/series/:id",
		"PUT /api/v1/admin/options/series/:id",
		"DELETE /api/v1/admin/options/series/:id",
		"POST /api/v1/admin/options/series/:id/settle",
		"POST /api/v1/admin/options/series/:id/refresh-price",
	}

	for _, route := range want {
		if !registered[route] {
			t.Errorf("route %q is not registered", route)
		}
	}
}

// TestRouterProtectedRoutesRejectAnonymousCallers sends an unauthenticated
// request to each newly exposed protected route and expects the auth middleware
// to stop it. A route that answered without a token would read the caller's
// identity from an empty context.
func TestRouterProtectedRoutesRejectAnonymousCallers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := setupRouter(apiDeps{})

	protected := []struct{ method, path string }{
		{"GET", "/api/v1/bots"},
		{"POST", "/api/v1/bots"},
		{"GET", "/api/v1/bots/1/orders"},
		{"GET", "/api/v1/otc/orders"},
		{"POST", "/api/v1/otc/orders"},
		{"DELETE", "/api/v1/otc/orders/1"},
		{"GET", "/api/v1/twamm/orders"},
		{"POST", "/api/v1/twamm/orders"},
		{"GET", "/api/v1/farms/stakes"},
		{"POST", "/api/v1/farms/1/stake"},
		{"POST", "/api/v1/farms/1/claim"},
		{"GET", "/api/v1/airdrops/allocations"},
		{"POST", "/api/v1/airdrops/1/claim"},
		{"GET", "/api/v1/options/positions"},
		{"POST", "/api/v1/options/series/1/buy"},
		{"POST", "/api/v1/options/series/1/collateral"},
		{"POST", "/api/v1/options/series/1/exercise"},
	}

	for _, route := range protected {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(route.method, route.path, nil)
		router.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusUnauthorized {
			t.Errorf("%s %s answered %d without a token, want 401",
				route.method, route.path, recorder.Code)
		}
	}
}
