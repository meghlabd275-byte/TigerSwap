package services

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"tigerswap/backend/models"
)

// Account recovery: password reset and email verification.
//
// Both flows issue a single-use token through the same machinery. A token is a
// random 32-byte value; only its SHA-256 hash is stored, so a database read
// cannot yield a usable link. A token is bound to a purpose, so one minted for
// email verification can never be redeemed to reset a password. Redemption is a
// conditional update on `used = false`, so a replayed link affects zero rows
// and is refused.

const (
	// passwordResetTTL is deliberately short: a reset link is the most sensitive
	// message the platform sends.
	passwordResetTTL = 30 * time.Minute
	// emailVerificationTTL is longer because a user may not read the mail
	// immediately.
	emailVerificationTTL = 24 * time.Hour
)

// accountResetBaseURL is the web origin a recovery link points at. It must be
// configured for production; the fallback only serves a local run.
func accountResetBaseURL() string {
	if base := strings.TrimSpace(os.Getenv("PUBLIC_WEB_URL")); base != "" {
		return strings.TrimRight(base, "/")
	}
	if domain := strings.TrimSpace(os.Getenv("AUTH_DOMAIN")); domain != "" {
		return "https://" + domain
	}
	return "http://localhost:3000"
}

// issueAccountToken mints a single-use token for a user and purpose and returns
// the raw token. The raw value is never persisted.
func (s *AuthService) issueAccountToken(userID uuid.UUID, purpose string, ttl time.Duration) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := hex.EncodeToString(raw)
	row := models.AccountToken{
		UserID:    userID,
		Purpose:   purpose,
		TokenHash: hashRefreshToken(token),
		ExpiresAt: time.Now().UTC().Add(ttl),
	}
	if err := s.db.Create(&row).Error; err != nil {
		return "", err
	}
	return token, nil
}

// consumeAccountToken redeems a token exactly once. It returns the owning user
// id, or an error when the token is unknown, expired, of the wrong purpose, or
// already used. The conditional update is what makes redemption single-use even
// under concurrent requests.
func (s *AuthService) consumeAccountToken(purpose, token string) (uuid.UUID, error) {
	var row models.AccountToken
	err := s.db.Where("token_hash = ? AND purpose = ? AND used = ? AND expires_at > ?",
		hashRefreshToken(token), purpose, false, time.Now().UTC()).First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return uuid.Nil, errors.New("token is invalid or has expired")
		}
		return uuid.Nil, err
	}
	consumed := s.db.Model(&models.AccountToken{}).
		Where("id = ? AND used = ?", row.ID, false).
		Updates(map[string]interface{}{"used": true, "used_at": time.Now().UTC()})
	if consumed.Error != nil {
		return uuid.Nil, consumed.Error
	}
	if consumed.RowsAffected != 1 {
		return uuid.Nil, errors.New("token has already been used")
	}
	return row.UserID, nil
}

// RequestPasswordReset starts the reset flow. It always answers 200 so the
// endpoint cannot be used to enumerate accounts, but it only sends mail for an
// address that exists. A wallet-only account (no password) is skipped.
func (s *AuthService) RequestPasswordReset(c *gin.Context) {
	var req struct {
		Email string `json:"email" binding:"required,email"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequestError(c, err)
		return
	}

	var user models.User
	found := s.db.Where("email = ?", req.Email).First(&user).Error == nil
	if found && user.PasswordHash != "" && user.IsActive {
		token, err := s.issueAccountToken(user.ID, models.AccountTokenPasswordReset, passwordResetTTL)
		if err != nil {
			respondInternalError(c, err)
			return
		}
		link := accountResetBaseURL() + "/reset-password?token=" + token
		body := "Someone requested a password reset for your TigerSwap account.\n\n" +
			"Reset your password: " + link + "\n\n" +
			"This link expires in 30 minutes and can be used once. " +
			"If you did not request this, you can ignore this message."
		if err := s.mailer.Send(user.Email, "Reset your TigerSwap password", body); err != nil {
			// A failure to deliver must be visible in the logs, but the response is
			// still a generic 200 so the endpoint cannot enumerate accounts.
			logMailFailure("password reset", user.Email, err)
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "If that address has an account, a reset link has been sent."})
}

// ResetPassword completes the reset flow with a token from the email.
func (s *AuthService) ResetPassword(c *gin.Context) {
	var req struct {
		Token       string `json:"token" binding:"required"`
		NewPassword string `json:"new_password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequestError(c, err)
		return
	}
	if len(req.NewPassword) < 8 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Password must be at least 8 characters"})
		return
	}
	if len(req.NewPassword) > 72 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Password must be at most 72 bytes"})
		return
	}

	userID, err := s.consumeAccountToken(models.AccountTokenPasswordReset, req.Token)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		respondInternalError(c, err)
		return
	}
	hashed := string(hashedBytes)
	if err != nil {
		respondInternalError(c, err)
		return
	}
	if err := s.db.Model(&models.User{}).Where("id = ?", userID).
		Updates(map[string]interface{}{"password_hash": hashed, "failed_login_attempts": 0, "locked_until": nil}).Error; err != nil {
		respondInternalError(c, err)
		return
	}

	// A password change must invalidate every existing session: the whole point
	// of a reset is that the old credential may be compromised.
	s.revokeAllSessions(userID)

	c.JSON(http.StatusOK, gin.H{"message": "Your password has been reset. Sign in with the new password."})
}

// RequestEmailVerification sends (or resends) a verification link to the caller.
func (s *AuthService) RequestEmailVerification(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}
	var user models.User
	if err := s.db.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "account not found"})
		return
	}
	if user.IsVerified {
		c.JSON(http.StatusOK, gin.H{"verified": true})
		return
	}
	if err := s.sendVerificationEmail(&user); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "email delivery is not configured; contact support"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "A verification link has been sent."})
}

// sendVerificationEmail issues a token and mails the link. It is shared by
// registration and the resend endpoint so the two paths cannot drift.
func (s *AuthService) sendVerificationEmail(user *models.User) error {
	token, err := s.issueAccountToken(user.ID, models.AccountTokenEmailVerification, emailVerificationTTL)
	if err != nil {
		return err
	}
	link := accountResetBaseURL() + "/verify-email?token=" + token
	body := "Confirm your TigerSwap email address.\n\n" +
		"Verify your email: " + link + "\n\n" +
		"This link expires in 24 hours and can be used once."
	if err := s.mailer.Send(user.Email, "Verify your TigerSwap email", body); err != nil {
		logMailFailure("email verification", user.Email, err)
		return err
	}
	return nil
}

// VerifyEmail completes email verification with a token from the mail.
func (s *AuthService) VerifyEmail(c *gin.Context) {
	var req struct {
		Token string `json:"token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequestError(c, err)
		return
	}
	userID, err := s.consumeAccountToken(models.AccountTokenEmailVerification, req.Token)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := s.db.Model(&models.User{}).Where("id = ?", userID).
		Update("is_verified", true).Error; err != nil {
		respondInternalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"verified": true})
}

// revokeAllSessions invalidates every access token (via the user-level cutoff)
// and every refresh token a user holds. It is used after a password reset.
func (s *AuthService) revokeAllSessions(userID uuid.UUID) {
	if s.revocations != nil {
		if err := s.revocations.revokeUser(userID.String()); err != nil {
			logMailFailure("session revocation", userID.String(), err)
		}
	}
	if err := s.revokeUserRefreshTokens(userID); err != nil {
		logMailFailure("refresh revocation", userID.String(), err)
	}
}

// logMailFailure records a delivery or revocation failure without leaking the
// address into a structured response. It uses the standard logger so it lands in
// the same stream as the rest of the service logs.
func logMailFailure(flow, target string, err error) {
	// The target is not logged: a reset flow should not write addresses to logs.
	_ = target
	log.Printf("%s: %v", flow, err)
}
