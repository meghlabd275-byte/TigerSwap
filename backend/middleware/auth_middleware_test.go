package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"

	"tigerswap/backend/models"
	"tigerswap/backend/services"
)

// signTestToken issues a token in the exact shape the AuthService produces, so
// the middleware is exercised against a real JWT rather than a hand-built
// context. The secret must match the one NewAuthService reads from the
// environment.
func signTestToken(t *testing.T, secret, userID, role string) string {
	t.Helper()
	claims := jwt.MapClaims{
		"jti":     "test-jti",
		"user_id": userID,
		"email":   "operator@example.com",
		"role":    role,
		"exp":     time.Now().Add(time.Hour).Unix(),
		"iat":     time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return signed
}

// newAuthServiceForTest builds an AuthService with no database. Revocation
// lookups degrade to the in-memory set, which is empty, so validation depends
// only on the signature and expiry.
func newAuthServiceForTest(t *testing.T, secret string) *services.AuthService {
	t.Helper()
	t.Setenv("JWT_SECRET", secret)
	return services.NewAuthService(nil)
}

// TestAdminRequiredAdmitsAdminAndSuperAdmin pins the role names the JWT actually
// carries. The token role is `superadmin`; before this was fixed the middleware
// compared against `admin` only, so a super admin was refused, and the
// super-admin guard compared against `super_admin`, which no token ever holds.
func TestAdminRequiredAdmitsAdminAndSuperAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const secret = "middleware-role-test-secret-0123456789"
	auth := newAuthServiceForTest(t, secret)

	router := gin.New()
	router.GET("/admin", AdminRequired(auth), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"role": c.GetString("role")})
	})

	for _, role := range []string{models.RoleAdmin, models.RoleSuperAdmin} {
		req := httptest.NewRequest(http.MethodGet, "/admin", nil)
		req.Header.Set("Authorization", "Bearer "+signTestToken(t, secret, "user-1", role))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("role %q should be admitted, got %d: %s", role, rec.Code, rec.Body.String())
		}
	}
}

func TestAdminRequiredRejectsPlainUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const secret = "middleware-role-test-secret-0123456789"
	auth := newAuthServiceForTest(t, secret)

	router := gin.New()
	router.GET("/admin", AdminRequired(auth), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.Header.Set("Authorization", "Bearer "+signTestToken(t, secret, "user-1", models.RoleUser))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("a plain user must be refused, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestSuperAdminRequiredMatchesTokenRole proves the super-admin guard accepts
// the role string a token actually carries and refuses an ordinary admin.
func TestSuperAdminRequiredMatchesTokenRole(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const secret = "middleware-role-test-secret-0123456789"
	auth := newAuthServiceForTest(t, secret)

	router := gin.New()
	router.GET("/super",
		AuthRequired(auth),
		SuperAdminRequired(),
		func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) },
	)

	superReq := httptest.NewRequest(http.MethodGet, "/super", nil)
	superReq.Header.Set("Authorization", "Bearer "+signTestToken(t, secret, "user-1", models.RoleSuperAdmin))
	superRec := httptest.NewRecorder()
	router.ServeHTTP(superRec, superReq)
	if superRec.Code != http.StatusOK {
		t.Fatalf("super admin should be admitted, got %d: %s", superRec.Code, superRec.Body.String())
	}

	adminReq := httptest.NewRequest(http.MethodGet, "/super", nil)
	adminReq.Header.Set("Authorization", "Bearer "+signTestToken(t, secret, "user-1", models.RoleAdmin))
	adminRec := httptest.NewRecorder()
	router.ServeHTTP(adminRec, adminReq)
	if adminRec.Code != http.StatusForbidden {
		t.Fatalf("an ordinary admin must not reach a super-admin route, got %d: %s", adminRec.Code, adminRec.Body.String())
	}
}

func TestAuthRequiredRejectsMissingAndMalformedHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const secret = "middleware-role-test-secret-0123456789"
	auth := newAuthServiceForTest(t, secret)

	router := gin.New()
	router.GET("/private", AuthRequired(auth), func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })

	cases := map[string]string{
		"missing":   "",
		"notBearer": "Token abc",
		"empty":     "Bearer ",
		"garbage":   "Bearer not-a-real-token",
	}
	for name, header := range cases {
		req := httptest.NewRequest(http.MethodGet, "/private", nil)
		if header != "" {
			req.Header.Set("Authorization", header)
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s header should yield 401, got %d: %s", name, rec.Code, rec.Body.String())
		}
	}
}
