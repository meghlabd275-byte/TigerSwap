package services

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"tigerswap/backend/models"
)

// APIKeyService issues and validates developer API keys.
//
// The plaintext key is shown exactly once, at creation; only its SHA-256 hash is
// stored. Validation is a hash lookup, so the database never holds a value that
// can be replayed, and an operator reading the table cannot recover a key.
type APIKeyService struct {
	db *gorm.DB
}

// apiKeyScopes is the closed set of scopes a key may hold. A typo is rejected at
// creation rather than producing a key that silently cannot do what was asked.
var apiKeyScopes = map[string]bool{
	"read":     true,
	"trade":    true,
	"withdraw": true,
	"*":        true,
}

// ErrInvalidAPIKey marks a key that is unknown, revoked or expired. The caller
// treats every case the same so a probe cannot tell which one applies.
var ErrInvalidAPIKey = errors.New("invalid api key")

// NewAPIKeyService builds the service over a database.
func NewAPIKeyService(db *gorm.DB) *APIKeyService {
	return &APIKeyService{db: db}
}

// generateAPIKey returns a random key and its display prefix. The value is
// 32 random bytes in hex, prefixed so it is recognisable in a secret scanner.
func generateAPIKey() (string, string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", fmt.Errorf("generate api key: %w", err)
	}
	key := "tsk_" + hex.EncodeToString(raw)
	return key, key[:12], nil
}

// hashAPIKey returns the storage hash for a presented key.
func hashAPIKey(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

// normalizeScopes validates and canonicalises a scope list. An empty list means
// read-only, which is the least privilege a key can hold.
func normalizeScopes(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "read", nil
	}
	if raw == "*" {
		return "*", nil
	}
	seen := map[string]bool{}
	var out []string
	for _, part := range strings.Split(raw, ",") {
		scope := strings.TrimSpace(part)
		if scope == "" {
			continue
		}
		if !apiKeyScopes[scope] {
			return "", fmt.Errorf("unknown scope %q", scope)
		}
		if seen[scope] {
			continue
		}
		seen[scope] = true
		out = append(out, scope)
	}
	if len(out) == 0 {
		return "read", nil
	}
	return strings.Join(out, ","), nil
}

// HasScope reports whether a stored key holds a scope.
func HasAPIKeyScope(key *models.DeveloperAPIKey, scope string) bool {
	if key == nil {
		return false
	}
	for _, held := range strings.Split(key.Scopes, ",") {
		held = strings.TrimSpace(held)
		if held == "*" || held == scope {
			return true
		}
	}
	return false
}

// Authenticate resolves a presented key. A key that is unknown, inactive,
// revoked or expired is rejected with ErrInvalidAPIKey. The lookup is by hash
// and the comparison is constant time, so timing does not disclose a stored key.
func (s *APIKeyService) Authenticate(presented string) (*models.DeveloperAPIKey, error) {
	presented = strings.TrimSpace(presented)
	if presented == "" {
		return nil, ErrInvalidAPIKey
	}

	var key models.DeveloperAPIKey
	if err := s.db.Where("key_hash = ?", hashAPIKey(presented)).First(&key).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvalidAPIKey
		}
		return nil, err
	}

	stored, err := hex.DecodeString(key.KeyHash)
	if err != nil {
		return nil, ErrInvalidAPIKey
	}
	candidate := sha256.Sum256([]byte(presented))
	if subtle.ConstantTimeCompare(stored, candidate[:]) != 1 {
		return nil, ErrInvalidAPIKey
	}

	if !key.IsActive || key.RevokedAt != nil {
		return nil, ErrInvalidAPIKey
	}
	if key.ExpiresAt != nil && time.Now().After(*key.ExpiresAt) {
		return nil, ErrInvalidAPIKey
	}

	now := time.Now()
	if err := s.db.Model(&models.DeveloperAPIKey{}).Where("id = ?", key.ID).Update("last_used", now).Error; err != nil {
		return nil, err
	}
	key.LastUsed = &now

	return &key, nil
}

// ByID loads a key row by id. It is used by the rate limiter to read the key's
// own budget without re-authenticating it.
func (s *APIKeyService) ByID(id string) (*models.DeveloperAPIKey, error) {
	var key models.DeveloperAPIKey
	if err := s.db.Where("id = ?", id).First(&key).Error; err != nil {
		return nil, err
	}
	return &key, nil
}

// CreateAPIKey issues a key for the caller. The plaintext is returned once and
// never stored.
func (s *APIKeyService) CreateAPIKey(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}

	var req struct {
		Name      string `json:"name"`
		Scopes    string `json:"scopes"`
		RateLimit int    `json:"rate_limit"`
		ExpiresAt string `json:"expires_at"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "a key name is required"})
		return
	}
	if len(req.Name) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "key name is too long"})
		return
	}
	if req.RateLimit < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "rate_limit cannot be negative"})
		return
	}

	scopes, err := normalizeScopes(req.Scopes)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var expiresAt *time.Time
	if strings.TrimSpace(req.ExpiresAt) != "" {
		parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(req.ExpiresAt))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "expires_at must be RFC3339"})
			return
		}
		if !parsed.After(time.Now()) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "expires_at must be in the future"})
			return
		}
		expiresAt = &parsed
	}

	plaintext, prefix, err := generateAPIKey()
	if err != nil {
		respondInternalError(c, err)
		return
	}

	key := models.DeveloperAPIKey{
		UserID:    userID,
		Name:      req.Name,
		KeyHash:   hashAPIKey(plaintext),
		KeyPrefix: prefix,
		Scopes:    scopes,
		RateLimit: req.RateLimit,
		IsActive:  true,
		ExpiresAt: expiresAt,
	}
	if err := s.db.Create(&key).Error; err != nil {
		respondInternalError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":         key.ID,
		"name":       key.Name,
		"key":        plaintext,
		"key_prefix": key.KeyPrefix,
		"scopes":     key.Scopes,
		"rate_limit": key.RateLimit,
		"expires_at": key.ExpiresAt,
		"created_at": key.CreatedAt,
		"message":    "store this key now; it is shown only once",
	})
}

// ListAPIKeys returns the caller's own keys. The hash is never serialised.
func (s *APIKeyService) ListAPIKeys(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}

	var keys []models.DeveloperAPIKey
	if err := s.db.Where("user_id = ?", userID).Order("created_at desc").Find(&keys).Error; err != nil {
		respondInternalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"keys": keys})
}

// RevokeAPIKey marks one of the caller's keys unusable. The row is kept so the
// audit trail of what was issued survives.
func (s *APIKeyService) RevokeAPIKey(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}

	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "a key id is required"})
		return
	}

	now := time.Now()
	res := s.db.Model(&models.DeveloperAPIKey{}).
		Where("id = ? AND user_id = ?", id, userID).
		Updates(map[string]interface{}{"is_active": false, "revoked_at": now})
	if res.Error != nil {
		respondInternalError(c, res.Error)
		return
	}
	if res.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "key not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "revoked", "id": id})
}
