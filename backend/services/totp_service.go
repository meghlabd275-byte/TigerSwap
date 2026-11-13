package services

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"tigerswap/backend/models"
)

// TOTP two-factor authentication.
//
// Enrolment is two steps so a user can never lock themselves out: Setup stores a
// candidate secret but leaves the account un-enrolled, and only Confirm — which
// requires a valid code from that secret — flips TOTPEnabled on. The secret is
// sealed with AES-256-GCM under a key derived from WALLET_ENCRYPTION_KEY; the
// service refuses enrolment when that key is unset rather than storing a secret
// in the clear.

// totpIssuer is the label shown in the authenticator app.
const totpIssuer = "TigerSwap"

// deriveTOTPKey builds the AES-256 key for TOTP secrets from
// WALLET_ENCRYPTION_KEY. A missing key is an explicit error, not a silent
// fallback to a weaker key.
func deriveTOTPKey() ([]byte, error) {
	secret := os.Getenv("WALLET_ENCRYPTION_KEY")
	if len(secret) < 32 {
		return nil, errors.New("WALLET_ENCRYPTION_KEY must be set (32+ chars) to enrol two-factor authentication")
	}
	sum := sha256.Sum256([]byte(secret))
	return sum[:], nil
}

// encryptTOTPSecret seals a TOTP secret. It reuses the same construction as the
// wallet service (AES-256-GCM, nonce prefixed, hex-encoded).
func encryptTOTPSecret(plaintext string) (string, error) {
	key, err := deriveTOTPKey()
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	return hex.EncodeToString(gcm.Seal(nonce, nonce, []byte(plaintext), nil)), nil
}

// decryptTOTPSecret opens a value produced by encryptTOTPSecret.
func decryptTOTPSecret(encoded string) (string, error) {
	key, err := deriveTOTPKey()
	if err != nil {
		return "", err
	}
	raw, err := hex.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(raw) < gcm.NonceSize() {
		return "", errors.New("ciphertext too short")
	}
	nonce, ciphertext := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// verifyUserTOTP loads a user's secret and checks a code. A user with no secret
// configured never verifies.
func (s *AuthService) verifyUserTOTP(userID string, code string) (bool, error) {
	var user models.User
	if err := s.db.Where("id = ?", userID).First(&user).Error; err != nil {
		return false, err
	}
	if user.TOTPSecret == "" {
		return false, nil
	}
	secret, err := decryptTOTPSecret(user.TOTPSecret)
	if err != nil {
		return false, err
	}
	return verifyTOTP(secret, code), nil
}

// SetupTOTP begins enrolment. It returns the secret and the otpauth URI to scan;
// the account is not yet protected until ConfirmTOTP succeeds.
func (s *AuthService) SetupTOTP(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}

	var user models.User
	if err := s.db.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "account not found"})
		return
	}
	if user.TOTPEnabled {
		c.JSON(http.StatusConflict, gin.H{"error": "two-factor authentication is already enabled"})
		return
	}

	secret, err := generateTOTPSecret()
	if err != nil {
		respondInternalError(c, err)
		return
	}
	sealed, err := encryptTOTPSecret(secret)
	if err != nil {
		respondInternalError(c, err)
		return
	}
	if err := s.db.Model(&models.User{}).Where("id = ?", userID).Update("totp_secret", sealed).Error; err != nil {
		respondInternalError(c, err)
		return
	}

	account := user.Email
	if account == "" {
		account = user.Username
	}
	c.JSON(http.StatusOK, gin.H{
		"secret":       secret,
		"otpauth_uri":  totpProvisioningURI(totpIssuer, account, secret),
		"totp_enabled": false,
	})
}

// ConfirmTOTP completes enrolment by verifying a code from the candidate secret.
func (s *AuthService) ConfirmTOTP(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}
	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequestError(c, err)
		return
	}

	var user models.User
	if err := s.db.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "account not found"})
		return
	}
	if user.TOTPSecret == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "start two-factor enrolment first"})
		return
	}
	secret, err := decryptTOTPSecret(user.TOTPSecret)
	if err != nil {
		respondInternalError(c, err)
		return
	}
	if !verifyTOTP(secret, req.Code) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid two-factor code"})
		return
	}
	if err := s.db.Model(&models.User{}).Where("id = ?", userID).
		Update("totp_enabled", true).Error; err != nil {
		respondInternalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"totp_enabled": true})
}

// DisableTOTP turns two-factor off. A valid current code is required, so a
// stolen session alone cannot strip the second factor.
func (s *AuthService) DisableTOTP(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}
	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequestError(c, err)
		return
	}

	ok, err := s.verifyUserTOTP(userID, req.Code)
	if err != nil {
		respondInternalError(c, err)
		return
	}
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid two-factor code"})
		return
	}
	if err := s.db.Model(&models.User{}).Where("id = ?", userID).
		Updates(map[string]interface{}{"totp_enabled": false, "totp_secret": ""}).Error; err != nil {
		respondInternalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"totp_enabled": false})
}

// TOTPStatus reports whether the caller has two-factor enabled.
func (s *AuthService) TOTPStatus(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}
	var user models.User
	if err := s.db.Select("totp_enabled").Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "account not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"totp_enabled": user.TOTPEnabled})
}

// requireTOTPIfEnabled checks the X-TOTP-Code header for a caller whose account
// has two-factor enabled. It is a no-op for an account without two-factor, so a
// sensitive route can always be wrapped without breaking users who have not
// enrolled. It is exported for the middleware package.
func (s *AuthService) RequireTOTPIfEnabled(c *gin.Context) bool {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return false
	}
	var user models.User
	if err := s.db.Select("totp_enabled", "totp_secret").Where("id = ?", userID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "account not found"})
			return false
		}
		respondInternalError(c, err)
		return false
	}
	if !user.TOTPEnabled {
		return true
	}
	code := strings.TrimSpace(c.GetHeader("X-TOTP-Code"))
	if code == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error":         "two-factor code required",
			"totp_required": true,
		})
		return false
	}
	secret, err := decryptTOTPSecret(user.TOTPSecret)
	if err != nil {
		respondInternalError(c, err)
		return false
	}
	if !verifyTOTP(secret, code) {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"error":         "invalid two-factor code",
			"totp_required": true,
		})
		return false
	}
	return true
}

// TOTPRequired returns a middleware that enforces the second factor on a
// sensitive route for callers who have it enabled. It is a no-op for an account
// without two-factor, so wrapping a route never breaks users who have not
// enrolled. The code is carried in the X-TOTP-Code header so it never lands in a
// URL or a log line.
func (s *AuthService) TOTPRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !s.RequireTOTPIfEnabled(c) {
			c.Abort()
			return
		}
		c.Next()
	}
}

// verifyLoginTOTP is the login-time second-factor check. It returns an error
// message to render, or "" when the caller may proceed.
func (s *AuthService) verifyLoginTOTP(user *models.User, code string) string {
	if !user.TOTPEnabled {
		return ""
	}
	if strings.TrimSpace(code) == "" {
		return "two-factor code required"
	}
	secret, err := decryptTOTPSecret(user.TOTPSecret)
	if err != nil {
		return "two-factor verification is unavailable"
	}
	if !verifyTOTP(secret, code) {
		return "invalid two-factor code"
	}
	return ""
}
