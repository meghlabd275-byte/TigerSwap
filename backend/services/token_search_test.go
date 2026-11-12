package services

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"

	"tigerswap/backend/models"
)

// TestSearchTokensMatchesCaseInsensitively pins that the public token search
// matches regardless of case and does not depend on a dialect-specific operator.
//
// The query used to be `symbol ILIKE ?`, which only PostgreSQL understands, so
// the search could not run under the SQLite test driver at all and the handler
// had no test coverage. It now lower-cases both sides with LOWER(...) LIKE.
func TestSearchTokensMatchesCaseInsensitively(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := newListingTestDB(t)
	if err := db.AutoMigrate(&models.Token{}); err != nil {
		t.Fatalf("migrate tokens: %v", err)
	}

	// NewTokenService seeds the built-in registry, so the fixture uses a symbol
	// and name that cannot collide with it.
	svc := NewTokenService(db)
	if err := db.Create(&models.Token{
		Address: "0x00000000000000000000000000000000000000aa", ChainID: 4242,
		Name: "Zebra Tiger", Symbol: "ZZTGR", Decimals: 18, IsActive: true,
	}).Error; err != nil {
		t.Fatalf("seed token: %v", err)
	}

	search := func(query string) []map[string]any {
		t.Helper()
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodGet, "/api/v1/tokens/search?q="+url.QueryEscape(query), nil)
		svc.SearchTokens(c)
		if rec.Code != http.StatusOK {
			t.Fatalf("search %q: status %d body %s", query, rec.Code, rec.Body.String())
		}
		var rows []map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &rows); err != nil {
			t.Fatalf("search %q: decode: %v", query, err)
		}
		return rows
	}

	// A lower-case query must find an upper-case symbol.
	if got := search("zztgr"); len(got) != 1 || got[0]["symbol"] != "ZZTGR" {
		t.Fatalf("lower-case query must match the symbol: %v", got)
	}
	// And an upper-case query must find a mixed-case name.
	if got := search("ZEBRA TIGER"); len(got) != 1 || got[0]["symbol"] != "ZZTGR" {
		t.Fatalf("upper-case query must match the name: %v", got)
	}
	// A seeded token must be reachable through a lower-case query too.
	if got := search("wbtc"); len(got) != 1 || got[0]["symbol"] != "WBTC" {
		t.Fatalf("seeded symbol must match case-insensitively: %v", got)
	}

	// An absent query returns no rows rather than every row.
	if rows := search("no-such-token-zzz"); len(rows) != 0 {
		t.Fatalf("an absent query must return no rows, got %v", rows)
	}

	// The pattern is bound, not interpolated, so a caller cannot widen the
	// search by supplying wildcards: a bare `%` matches no literal text.
	if rows := search("%"); len(rows) != 0 {
		t.Fatalf("a wildcard in the query must be treated literally, got %v", rows)
	}
}
