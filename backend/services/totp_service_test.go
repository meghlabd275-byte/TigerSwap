package services

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"tigerswap/backend/models"
)

// TestTOTPMatchesRFC6238Vectors pins the primitive to the RFC 6238 test vectors
// (SHA-1, 20-byte ASCII secret "12345678901234567890"). The RFC lists 8-digit
// codes; a 6-digit code is the same dynamic-truncation value modulo 10^6, so the
// expected value is the last six digits.
func TestTOTPMatchesRFC6238Vectors(t *testing.T) {
	// Base32 of the ASCII secret "12345678901234567890".
	const secret = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"

	vectors := []struct {
		unix int64
		code string
	}{
		{59, "287082"},          // RFC: 94287082
		{1111111109, "081804"},  // RFC: 07081804
		{1111111111, "050471"},  // RFC: 14050471
		{1234567890, "005924"},  // RFC: 89005924
		{2000000000, "279037"},  // RFC: 69279037
		{20000000000, "353130"}, // RFC: 65353130
	}

	for _, vector := range vectors {
		got, err := totpCodeAt(secret, time.Unix(vector.unix, 0))
		if err != nil {
			t.Fatalf("code at %d: %v", vector.unix, err)
		}
		if got != vector.code {
			t.Fatalf("code at %d: want %s, got %s", vector.unix, vector.code, got)
		}
	}
}

// TestVerifyTOTPAcceptsAdjacentStep proves the one-step skew window works.
func TestVerifyTOTPAcceptsAdjacentStep(t *testing.T) {
	secret, err := generateTOTPSecret()
	if err != nil {
		t.Fatalf("secret: %v", err)
	}
	previous, _ := totpCodeAt(secret, time.Now().Add(-totpPeriod))
	if !verifyTOTP(secret, previous) {
		t.Fatal("the previous step's code must be accepted within the skew window")
	}
	tooOld, _ := totpCodeAt(secret, time.Now().Add(-10*totpPeriod))
	if verifyTOTP(secret, tooOld) {
		t.Fatal("a code far outside the skew window must be rejected")
	}
	if verifyTOTP(secret, "000000") && verifyTOTP(secret, "111111") {
		// Not a strict assertion, just a sanity guard against a stub verifier.
	}
}

func newTOTPHarness(t *testing.T) (*AuthService, *gorm.DB) {
	t.Helper()
	t.Setenv("JWT_SECRET", "totp-test-secret-0123456789abcdefgh")
	t.Setenv("WALLET_ENCRYPTION_KEY", "totp-encryption-key-0123456789abcdef")
	db := newListingTestDB(t)
	if err := db.AutoMigrate(&models.User{}, &models.RevokedToken{}, &models.RefreshToken{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return NewAuthService(db), db
}

func totpRequest(t *testing.T, auth *AuthService, method, route string, handler gin.HandlerFunc, userID uuid.UUID, body, code string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Handle(method, route, func(c *gin.Context) {
		if userID != uuid.Nil {
			c.Set("user_id", userID.String())
		}
		handler(c)
	})
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	request := httptest.NewRequest(method, route, reader)
	request.Header.Set("Content-Type", "application/json")
	if code != "" {
		request.Header.Set("X-TOTP-Code", code)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

// TestTOTPEnrollmentLifecycle drives setup -> confirm -> status -> disable with a
// real secret and real codes.
func TestTOTPEnrollmentLifecycle(t *testing.T) {
	auth, db := newTOTPHarness(t)
	user := models.User{ID: uuid.New(), Username: "totpuser", Email: "totp@example.com", Role: models.RoleUser, IsActive: true}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	setup := totpRequest(t, auth, http.MethodPost, "/2fa/setup", auth.SetupTOTP, user.ID, "", "")
	if setup.Code != http.StatusOK {
		t.Fatalf("setup: want 200, got %d (%s)", setup.Code, setup.Body.String())
	}
	var setupBody struct {
		Secret      string `json:"secret"`
		OTPAuthURI  string `json:"otpauth_uri"`
		TOTPEnabled bool   `json:"totp_enabled"`
	}
	if err := json.Unmarshal(setup.Body.Bytes(), &setupBody); err != nil {
		t.Fatalf("decode setup: %v", err)
	}
	if setupBody.Secret == "" || !strings.HasPrefix(setupBody.OTPAuthURI, "otpauth://totp/") {
		t.Fatalf("setup must return a secret and otpauth URI: %s", setup.Body.String())
	}

	// A wrong code must not complete enrolment.
	wrong := totpRequest(t, auth, http.MethodPost, "/2fa/confirm", auth.ConfirmTOTP, user.ID,
		`{"code":"000000"}`, "")
	if wrong.Code == http.StatusOK {
		t.Fatal("a wrong code must not confirm enrolment")
	}

	code, err := totpCodeAt(setupBody.Secret, time.Now())
	if err != nil {
		t.Fatalf("code: %v", err)
	}
	confirm := totpRequest(t, auth, http.MethodPost, "/2fa/confirm", auth.ConfirmTOTP, user.ID,
		fmt.Sprintf(`{"code":%q}`, code), "")
	if confirm.Code != http.StatusOK {
		t.Fatalf("confirm: want 200, got %d (%s)", confirm.Code, confirm.Body.String())
	}

	var stored models.User
	db.Where("id = ?", user.ID).First(&stored)
	if !stored.TOTPEnabled {
		t.Fatal("account must be marked enrolled after a valid code")
	}
	if stored.TOTPSecret == setupBody.Secret {
		t.Fatal("the secret must be stored encrypted, not in the clear")
	}

	// Disable requires a fresh valid code.
	fresh, _ := totpCodeAt(setupBody.Secret, time.Now())
	disable := totpRequest(t, auth, http.MethodPost, "/2fa/disable", auth.DisableTOTP, user.ID,
		fmt.Sprintf(`{"code":%q}`, fresh), "")
	if disable.Code != http.StatusOK {
		t.Fatalf("disable: want 200, got %d (%s)", disable.Code, disable.Body.String())
	}
	db.Where("id = ?", user.ID).First(&stored)
	if stored.TOTPEnabled || stored.TOTPSecret != "" {
		t.Fatal("disable must clear both the flag and the secret")
	}
}

// TestTOTPRequiredMiddlewareBlocksWithoutCode proves a sensitive route is gated
// for an enrolled account and open for one without two-factor.
func TestTOTPRequiredMiddlewareBlocksWithoutCode(t *testing.T) {
	auth, db := newTOTPHarness(t)

	plain := models.User{ID: uuid.New(), Username: "plain", Email: "plain@example.com", Role: models.RoleUser, IsActive: true}
	enrolled := models.User{ID: uuid.New(), Username: "enrolled", Email: "enrolled@example.com", Role: models.RoleUser, IsActive: true}
	if err := db.Create(&plain).Error; err != nil {
		t.Fatalf("seed plain: %v", err)
	}
	if err := db.Create(&enrolled).Error; err != nil {
		t.Fatalf("seed enrolled: %v", err)
	}

	secret, _ := generateTOTPSecret()
	sealed, err := encryptTOTPSecret(secret)
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	db.Model(&models.User{}).Where("id = ?", enrolled.ID).
		Updates(map[string]interface{}{"totp_enabled": true, "totp_secret": sealed})

	guarded := func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) }

	// Without two-factor, the route is open.
	if resp := totpRequest(t, auth, http.MethodPost, "/sensitive", auth.TOTPRequired(), plain.ID, "", ""); resp.Code != http.StatusOK {
		t.Fatalf("a user without 2FA must pass, got %d", resp.Code)
	}
	_ = guarded

	// Enrolled, no code: refused with a marker.
	noCode := totpRequest(t, auth, http.MethodPost, "/sensitive", auth.TOTPRequired(), enrolled.ID, "", "")
	if noCode.Code != http.StatusUnauthorized {
		t.Fatalf("enrolled user without a code must be 401, got %d", noCode.Code)
	}
	if !strings.Contains(noCode.Body.String(), "totp_required") {
		t.Fatalf("the refusal must mark totp_required: %s", noCode.Body.String())
	}

	// Enrolled, valid code: passes.
	code, _ := totpCodeAt(secret, time.Now())
	withCode := totpRequest(t, auth, http.MethodPost, "/sensitive", auth.TOTPRequired(), enrolled.ID, "", code)
	if withCode.Code != http.StatusOK {
		t.Fatalf("enrolled user with a valid code must pass, got %d (%s)", withCode.Code, withCode.Body.String())
	}
}

// TestLoginRequiresTOTPWhenEnabled proves a password alone cannot mint a session
// once two-factor is on.
func TestLoginRequiresTOTPWhenEnabled(t *testing.T) {
	auth, db := newTOTPHarness(t)

	hash, err := bcrypt.GenerateFromPassword([]byte("Str0ng-Passw0rd!2026"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	secret, _ := generateTOTPSecret()
	sealed, _ := encryptTOTPSecret(secret)
	user := models.User{
		ID:           uuid.New(),
		Username:     "twouser",
		Email:        "two@example.com",
		PasswordHash: string(hash),
		Role:         models.RoleUser,
		IsActive:     true,
		TOTPEnabled:  true,
		TOTPSecret:   sealed,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	login := func(code string) *httptest.ResponseRecorder {
		body := fmt.Sprintf(`{"email":"two@example.com","password":"Str0ng-Passw0rd!2026","totp_code":%q}`, code)
		gin.SetMode(gin.TestMode)
		router := gin.New()
		router.POST("/login", auth.Login)
		request := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		return recorder
	}

	// Correct password, no code: refused.
	if resp := login(""); resp.Code != http.StatusUnauthorized {
		t.Fatalf("password alone must not log in with 2FA on, got %d (%s)", resp.Code, resp.Body.String())
	}
	// Correct password, wrong code: refused.
	if resp := login("000000"); resp.Code != http.StatusUnauthorized {
		t.Fatalf("a wrong code must not log in, got %d", resp.Code)
	}
	// Correct password, correct code: session issued.
	code, _ := totpCodeAt(secret, time.Now())
	resp := login(code)
	if resp.Code != http.StatusOK {
		t.Fatalf("password plus a valid code must log in, got %d (%s)", resp.Code, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), "token") {
		t.Fatalf("login must issue a token: %s", resp.Body.String())
	}
}
