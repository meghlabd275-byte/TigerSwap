package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func newTestRouter(store *APIKeyStore) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/open", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	router.GET("/protected", APIKeyAuth(store), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	router.GET("/admin", APIKeyAuth(store), RequireScope("admin"), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	return router
}

func do(router *gin.Engine, path string, headers map[string]string) int {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec.Code
}

func TestValidKeyIsAccepted(t *testing.T) {
	store := NewAPIKeyStore()
	store.Add("secret-key-1234", "test", []string{"read"}, time.Hour)
	router := newTestRouter(store)

	if code := do(router, "/protected", map[string]string{"X-API-Key": "secret-key-1234"}); code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
}

func TestMissingKeyIsRejected(t *testing.T) {
	store := NewAPIKeyStore()
	store.Add("secret-key-1234", "test", []string{"read"}, time.Hour)
	router := newTestRouter(store)

	if code := do(router, "/protected", nil); code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", code)
	}
}

func TestUnknownKeyIsRejected(t *testing.T) {
	store := NewAPIKeyStore()
	store.Add("secret-key-1234", "test", []string{"read"}, time.Hour)
	router := newTestRouter(store)

	if code := do(router, "/protected", map[string]string{"X-API-Key": "wrong"}); code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", code)
	}
}

func TestBearerTokenIsAccepted(t *testing.T) {
	store := NewAPIKeyStore()
	store.Add("secret-key-1234", "test", []string{"read"}, time.Hour)
	router := newTestRouter(store)

	if code := do(router, "/protected", map[string]string{"Authorization": "Bearer secret-key-1234"}); code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
}

func TestExpiredKeyIsRejected(t *testing.T) {
	store := NewAPIKeyStore()
	store.Add("secret-key-1234", "test", []string{"read"}, time.Nanosecond)
	time.Sleep(2 * time.Millisecond)
	router := newTestRouter(store)

	if code := do(router, "/protected", map[string]string{"X-API-Key": "secret-key-1234"}); code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", code)
	}
}

func TestRevokedKeyIsRejected(t *testing.T) {
	store := NewAPIKeyStore()
	hash := store.Add("secret-key-1234", "test", []string{"read"}, time.Hour)
	store.Revoke(hash)
	router := newTestRouter(store)

	if code := do(router, "/protected", map[string]string{"X-API-Key": "secret-key-1234"}); code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", code)
	}
}

func TestScopeIsEnforced(t *testing.T) {
	store := NewAPIKeyStore()
	store.Add("reader", "reader", []string{"read"}, time.Hour)
	store.Add("admin-key", "admin", []string{"admin"}, time.Hour)
	router := newTestRouter(store)

	if code := do(router, "/admin", map[string]string{"X-API-Key": "reader"}); code != http.StatusForbidden {
		t.Fatalf("expected 403 for reader, got %d", code)
	}
	if code := do(router, "/admin", map[string]string{"X-API-Key": "admin-key"}); code != http.StatusOK {
		t.Fatalf("expected 200 for admin, got %d", code)
	}
}

func TestWildcardScopeGrantsEverything(t *testing.T) {
	store := NewAPIKeyStore()
	store.Add("root", "root", []string{"*"}, time.Hour)
	router := newTestRouter(store)

	if code := do(router, "/admin", map[string]string{"X-API-Key": "root"}); code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
}

func TestStoredKeyIsHashedNotPlaintext(t *testing.T) {
	store := NewAPIKeyStore()
	hash := store.Add("super-secret-value", "test", []string{"read"}, time.Hour)

	store.mu.RLock()
	entry := store.keys[hash]
	store.mu.RUnlock()

	if entry == nil {
		t.Fatal("key not stored")
	}
	if entry.Hash == HashKey("super-secret-value") {
		// The hash must be derived, never the plaintext.
		if string(entry.Hash[:]) == "super-secret-value" {
			t.Fatal("plaintext key was stored")
		}
	}
	if entry.Label != "test" {
		t.Fatalf("unexpected label %q", entry.Label)
	}
}

func TestMaskKeyDoesNotRevealFullKey(t *testing.T) {
	key := "abcdefghijklmnop"
	masked := MaskKey(key)

	if masked == key {
		t.Fatal("masked key equals the original")
	}
	if len(masked) >= len(key) {
		t.Fatalf("masked key is not shorter: %q", masked)
	}
}

func TestParseAPIKeyPrecedence(t *testing.T) {
	if got := ParseAPIKey("header-key", "Bearer auth-key", "query-key"); got != "header-key" {
		t.Fatalf("header should win, got %q", got)
	}
	if got := ParseAPIKey("", "Bearer auth-key", "query-key"); got != "auth-key" {
		t.Fatalf("bearer should be used, got %q", got)
	}
	if got := ParseAPIKey("", "", "query-key"); got != "query-key" {
		t.Fatalf("query should be used, got %q", got)
	}
	if got := ParseAPIKey("", "Basic abc", "query-key"); got != "query-key" {
		t.Fatalf("non-bearer auth should fall through, got %q", got)
	}
}
