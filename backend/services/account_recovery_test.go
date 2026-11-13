package services

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"tigerswap/backend/models"
)

func newRecoveryHarness(t *testing.T) (*AuthService, *gorm.DB) {
	t.Helper()
	t.Setenv("JWT_SECRET", "recovery-test-secret-0123456789abcdef")
	t.Setenv("WALLET_ENCRYPTION_KEY", "recovery-encryption-key-0123456789ab")
	db := newListingTestDB(t)
	if err := db.AutoMigrate(&models.User{}, &models.RevokedToken{}, &models.RefreshToken{}, &models.AccountToken{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return NewAuthService(db), db
}

func recoveryPost(t *testing.T, route string, handler gin.HandlerFunc, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST(route, handler)
	request := httptest.NewRequest(http.MethodPost, route, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

// TestPasswordResetRevokesOldSessions proves a reset changes the password and
// invalidates every existing session, which is the whole point of a reset.
func TestPasswordResetRevokesOldSessions(t *testing.T) {
	auth, db := newRecoveryHarness(t)
	oldHash, _ := bcrypt.GenerateFromPassword([]byte("Old-Passw0rd!2026"), bcrypt.DefaultCost)
	user := models.User{
		ID:           uuid.New(),
		Username:     "resetuser",
		Email:        "reset@example.com",
		PasswordHash: string(oldHash),
		Role:         models.RoleUser,
		IsActive:     true,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	// A live session that must be killed by the reset.
	oldSession, err := auth.generateToken(user.ID, user.Email, user.Role)
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	if _, err := auth.ValidateToken(oldSession); err != nil {
		t.Fatalf("the session must be valid before the reset: %v", err)
	}

	token, err := auth.issueAccountToken(user.ID, models.AccountTokenPasswordReset, passwordResetTTL)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	resp := recoveryPost(t, "/password/reset", auth.ResetPassword,
		`{"token":"`+token+`","new_password":"Brand-New-Passw0rd!2026"}`)
	if resp.Code != http.StatusOK {
		t.Fatalf("reset: want 200, got %d (%s)", resp.Code, resp.Body.String())
	}

	// The old password no longer works; the new one does.
	var stored models.User
	db.Where("id = ?", user.ID).First(&stored)
	if bcrypt.CompareHashAndPassword([]byte(stored.PasswordHash), []byte("Old-Passw0rd!2026")) == nil {
		t.Fatal("the old password must no longer authenticate")
	}
	if bcrypt.CompareHashAndPassword([]byte(stored.PasswordHash), []byte("Brand-New-Passw0rd!2026")) != nil {
		t.Fatal("the new password must authenticate")
	}

	// The pre-reset session is now revoked.
	if _, err := auth.ValidateToken(oldSession); err == nil {
		t.Fatal("a session issued before the reset must be revoked")
	}

	// The token is single-use.
	replay := recoveryPost(t, "/password/reset", auth.ResetPassword,
		`{"token":"`+token+`","new_password":"Another-Passw0rd!2026"}`)
	if replay.Code == http.StatusOK {
		t.Fatal("a reset token must not be redeemable twice")
	}
}

// TestResetTokenIsPurposeBound proves a token minted for email verification
// cannot be used to reset a password.
func TestResetTokenIsPurposeBound(t *testing.T) {
	auth, db := newRecoveryHarness(t)
	user := models.User{ID: uuid.New(), Username: "bounduser", Email: "bound@example.com", Role: models.RoleUser, IsActive: true}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	verifyToken, err := auth.issueAccountToken(user.ID, models.AccountTokenEmailVerification, emailVerificationTTL)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	resp := recoveryPost(t, "/password/reset", auth.ResetPassword,
		`{"token":"`+verifyToken+`","new_password":"Brand-New-Passw0rd!2026"}`)
	if resp.Code == http.StatusOK {
		t.Fatal("an email-verification token must not reset a password")
	}
}

// TestForgotPasswordDoesNotEnumerate proves the endpoint answers identically for
// a known and an unknown address.
func TestForgotPasswordDoesNotEnumerate(t *testing.T) {
	auth, db := newRecoveryHarness(t)
	user := models.User{ID: uuid.New(), Username: "enumuser", Email: "enum@example.com", Role: models.RoleUser, IsActive: true}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	known := recoveryPost(t, "/password/forgot", auth.RequestPasswordReset, `{"email":"enum@example.com"}`)
	unknown := recoveryPost(t, "/password/forgot", auth.RequestPasswordReset, `{"email":"nobody@example.com"}`)
	if known.Code != http.StatusOK || unknown.Code != http.StatusOK {
		t.Fatalf("both responses must be 200, got %d and %d", known.Code, unknown.Code)
	}
	if known.Body.String() != unknown.Body.String() {
		t.Fatalf("the response must not differ by account existence: %q vs %q", known.Body.String(), unknown.Body.String())
	}
}

// TestEmailVerificationFlow proves a verification token marks the account
// verified exactly once.
func TestEmailVerificationFlow(t *testing.T) {
	auth, db := newRecoveryHarness(t)
	user := models.User{ID: uuid.New(), Username: "verifyuser", Email: "verify@example.com", Role: models.RoleUser, IsActive: true}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	token, err := auth.issueAccountToken(user.ID, models.AccountTokenEmailVerification, emailVerificationTTL)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	resp := recoveryPost(t, "/email/verify", auth.VerifyEmail, `{"token":"`+token+`"}`)
	if resp.Code != http.StatusOK {
		t.Fatalf("verify: want 200, got %d (%s)", resp.Code, resp.Body.String())
	}
	var body struct {
		Verified bool `json:"verified"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil || !body.Verified {
		t.Fatalf("verify must report verified=true: %s", resp.Body.String())
	}
	var stored models.User
	db.Where("id = ?", user.ID).First(&stored)
	if !stored.IsVerified {
		t.Fatal("the account must be marked verified")
	}

	// A replay is refused.
	replay := recoveryPost(t, "/email/verify", auth.VerifyEmail, `{"token":"`+token+`"}`)
	if replay.Code == http.StatusOK {
		t.Fatal("a verification token must not be redeemable twice")
	}
}
