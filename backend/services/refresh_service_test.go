package services

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"tigerswap/backend/models"
)

func newRefreshHarness(t *testing.T) (*AuthService, *gorm.DB) {
	t.Helper()
	t.Setenv("JWT_SECRET", "refresh-test-secret-0123456789abcdef")
	db := newListingTestDB(t)
	if err := db.AutoMigrate(&models.User{}, &models.RevokedToken{}, &models.RefreshToken{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return NewAuthService(db), db
}

// refreshPost issues a refresh request carrying the cookie and returns the
// recorder and the cookie the server set in response.
func refreshPost(t *testing.T, auth *AuthService, cookie *http.Cookie) (*httptest.ResponseRecorder, *http.Cookie) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/auth/refresh", auth.RefreshToken)
	request := httptest.NewRequest(http.MethodPost, "/auth/refresh", strings.NewReader(""))
	if cookie != nil {
		request.AddCookie(cookie)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	var set *http.Cookie
	for _, c := range recorder.Result().Cookies() {
		if c.Name == refreshCookieName {
			set = c
		}
	}
	return recorder, set
}

func seedRefreshUser(t *testing.T, db *gorm.DB) models.User {
	t.Helper()
	user := models.User{ID: uuid.New(), Username: "refreshuser", Role: models.RoleUser, IsActive: true}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return user
}

// issueViaHarness sets the refresh cookie for a user through a login-style flow.
func issueViaHarness(t *testing.T, auth *AuthService, userID uuid.UUID) *http.Cookie {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/issue", func(c *gin.Context) {
		if err := auth.setRefreshCookie(c, userID); err != nil {
			t.Errorf("setRefreshCookie: %v", err)
		}
		c.Status(http.StatusOK)
	})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/issue", nil)
	router.ServeHTTP(recorder, request)
	for _, c := range recorder.Result().Cookies() {
		if c.Name == refreshCookieName {
			return c
		}
	}
	t.Fatal("no refresh cookie was set")
	return nil
}

// TestRefreshRotatesToken proves a successful refresh issues a new cookie and
// revokes the presented one.
func TestRefreshRotatesToken(t *testing.T) {
	auth, db := newRefreshHarness(t)
	user := seedRefreshUser(t, db)
	cookie := issueViaHarness(t, auth, user.ID)

	resp, rotated := refreshPost(t, auth, cookie)
	if resp.Code != http.StatusOK {
		t.Fatalf("refresh: want 200, got %d (%s)", resp.Code, resp.Body.String())
	}
	if rotated == nil || rotated.Value == "" {
		t.Fatal("refresh must set a new cookie")
	}
	if rotated.Value == cookie.Value {
		t.Fatal("refresh must rotate the token, not reuse it")
	}

	var live int64
	db.Model(&models.RefreshToken{}).Where("user_id = ? AND revoked = ?", user.ID, false).Count(&live)
	if live != 1 {
		t.Fatalf("want exactly one live token after rotation, got %d", live)
	}
}

// TestRefreshReuseRevokesFamily proves presenting an already-rotated token is
// treated as theft: the whole family is revoked and a fresh sign-in is required.
func TestRefreshReuseRevokesFamily(t *testing.T) {
	auth, db := newRefreshHarness(t)
	user := seedRefreshUser(t, db)
	first := issueViaHarness(t, auth, user.ID)

	// Legitimate rotation: first -> second.
	resp, second := refreshPost(t, auth, first)
	if resp.Code != http.StatusOK {
		t.Fatalf("first refresh: want 200, got %d (%s)", resp.Code, resp.Body.String())
	}
	if second == nil {
		t.Fatal("first refresh did not rotate")
	}

	// Replay the already-rotated first token: reuse detection must fire.
	reuse, _ := refreshPost(t, auth, first)
	if reuse.Code != http.StatusUnauthorized {
		t.Fatalf("reused token must be 401, got %d (%s)", reuse.Code, reuse.Body.String())
	}

	// The whole family is revoked, so the second token is dead too.
	after, _ := refreshPost(t, auth, second)
	if after.Code != http.StatusUnauthorized {
		t.Fatalf("family must be revoked after reuse; second token returned %d", after.Code)
	}

	var live int64
	db.Model(&models.RefreshToken{}).Where("user_id = ? AND revoked = ?", user.ID, false).Count(&live)
	if live != 0 {
		t.Fatalf("reuse detection must revoke every token in the family, %d still live", live)
	}
}

// TestRefreshWithoutCookieIsUnauthorized proves an absent cookie is refused.
func TestRefreshWithoutCookieIsUnauthorized(t *testing.T) {
	auth, _ := newRefreshHarness(t)
	resp, _ := refreshPost(t, auth, nil)
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("refresh without a cookie must be 401, got %d (%s)", resp.Code, resp.Body.String())
	}
}

// TestRefreshRejectsUnknownToken proves a forged cookie is refused.
func TestRefreshRejectsUnknownToken(t *testing.T) {
	auth, _ := newRefreshHarness(t)
	resp, _ := refreshPost(t, auth, &http.Cookie{Name: refreshCookieName, Value: "not-a-real-token"})
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("unknown token must be 401, got %d (%s)", resp.Code, resp.Body.String())
	}
}

var _ = fmt.Sprintf
