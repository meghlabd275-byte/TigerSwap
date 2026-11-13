package services

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"tigerswap/backend/models"
)

// refreshCookieName is the httpOnly cookie that carries the rotating refresh
// credential. The access token stays in the response body (held in memory by the
// client); only the long-lived refresh credential is a cookie, so it is never
// exposed to page scripts.
const refreshCookieName = "tigerswap_refresh"

// refreshTTL is the lifetime of a refresh family. A refresh past this age is
// refused and the user must sign in again.
const refreshTTL = 30 * 24 * time.Hour

// refreshCookieSecure reports whether the cookie is marked Secure. It defaults
// to true; a local http deployment sets AUTH_COOKIE_INSECURE=true to run without
// TLS. Insecure is never the default, so a misconfigured production deployment
// fails closed to a secure cookie rather than shipping one readable over http.
func refreshCookieSecure() bool {
	return !strings.EqualFold(os.Getenv("AUTH_COOKIE_INSECURE"), "true")
}

// refreshCookieDomain returns the optional cookie domain. Empty means host-only.
func refreshCookieDomain() string {
	return strings.TrimSpace(os.Getenv("AUTH_COOKIE_DOMAIN"))
}

// setRefreshCookie issues a fresh refresh token for the user, stores only its
// hash, and sets the httpOnly cookie. It is called on sign-in and on every
// successful rotation; the returned token value is never logged.
func (s *AuthService) setRefreshCookie(c *gin.Context, userID uuid.UUID) error {
	return s.issueRefreshToken(c, userID, uuid.New())
}

// issueRefreshToken creates one refresh token inside a family and sets the
// cookie. A rotation reuses the family id so a replay of the rotated token can
// be traced back to the family and revoked wholesale.
func (s *AuthService) issueRefreshToken(c *gin.Context, userID uuid.UUID, familyID uuid.UUID) error {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)

	row := models.RefreshToken{
		UserID:    userID,
		TokenHash: hashRefreshToken(token),
		FamilyID:  familyID,
		ExpiresAt: time.Now().Add(refreshTTL),
	}
	if err := s.db.Create(&row).Error; err != nil {
		return err
	}

	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(
		refreshCookieName,
		token,
		int(refreshTTL.Seconds()),
		"/api/v1/auth",
		refreshCookieDomain(),
		refreshCookieSecure(),
		true, // httpOnly
	)
	return nil
}

// clearRefreshCookie removes the refresh cookie.
func clearRefreshCookie(c *gin.Context) {
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(refreshCookieName, "", -1, "/api/v1/auth", refreshCookieDomain(), refreshCookieSecure(), true)
}

// revokeRefreshFamily marks every live token in a family revoked. It is the
// response to a detected reuse: if an attacker presents a rotated token, the
// whole family is burned so neither party can continue with it.
func (s *AuthService) revokeRefreshFamily(familyID uuid.UUID) error {
	return s.db.Model(&models.RefreshToken{}).
		Where("family_id = ? AND revoked = ?", familyID, false).
		Update("revoked", true).Error
}

// RefreshSession rotates the refresh cookie. It requires the httpOnly cookie,
// not a bearer token: the whole point is to mint a new access token after the
// old one has expired. A token that is unknown, expired, or already revoked is
// refused; presenting an already-rotated token revokes the family (reuse
// detection) and forces a fresh sign-in.
func (s *AuthService) RefreshSession(c *gin.Context) {
	raw, err := c.Cookie(refreshCookieName)
	if err != nil || raw == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "no active session"})
		return
	}

	var (
		row         models.RefreshToken
		user        models.User
		newAccess   string
		shouldIssue bool
		reuse       bool
		inactive    bool
	)
	now := time.Now()

	err = s.db.Transaction(func(tx *gorm.DB) error {
		// Lock the row so two concurrent refreshes with the same token cannot both
		// rotate successfully; the second sees revoked=true and triggers reuse
		// detection.
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("token_hash = ?", hashRefreshToken(raw)).First(&row).Error; err != nil {
			return err
		}

		if row.Revoked || now.After(row.ExpiresAt) {
			// A reused token means the family was compromised. Burn the family and
			// record the reuse; the revocation must be committed, so the transaction
			// returns nil and the caller answers 401 from the flags below.
			if err := tx.Model(&models.RefreshToken{}).
				Where("family_id = ? AND revoked = ?", row.FamilyID, false).
				Update("revoked", true).Error; err != nil {
				return err
			}
			if err := tx.Model(&models.RefreshToken{}).
				Where("id = ?", row.ID).
				Update("reused_at", now).Error; err != nil {
				return err
			}
			reuse = true
			return nil
		}

		if err := tx.Where("id = ?", row.UserID).First(&user).Error; err != nil {
			return err
		}
		if !user.IsActive {
			inactive = true
			return nil
		}

		// Rotate: revoke the presented token, then mint a replacement in the same
		// family.
		if err := tx.Model(&models.RefreshToken{}).
			Where("id = ?", row.ID).Update("revoked", true).Error; err != nil {
			return err
		}
		token, err := s.generateToken(user.ID, user.Email, user.Role)
		if err != nil {
			return err
		}
		newAccess = token
		shouldIssue = true
		return nil
	})

	switch {
	case err != nil:
		clearRefreshCookie(c)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "session is no longer valid"})
		return
	case reuse:
		clearRefreshCookie(c)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "session reuse detected; please sign in again"})
		return
	case inactive:
		clearRefreshCookie(c)
		c.JSON(http.StatusForbidden, gin.H{"error": "account is not active"})
		return
	}

	if shouldIssue {
		if err := s.issueRefreshToken(c, user.ID, row.FamilyID); err != nil {
			respondInternalError(c, err)
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"user": user, "token": newAccess})
}

// revokeUserRefreshTokens revokes every refresh token for a user, so a
// "sign out everywhere" or an account deactivation also ends the cookie session.
func (s *AuthService) revokeUserRefreshTokens(userID uuid.UUID) error {
	return s.db.Model(&models.RefreshToken{}).
		Where("user_id = ? AND revoked = ?", userID, false).
		Update("revoked", true).Error
}
