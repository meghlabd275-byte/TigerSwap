package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// DeveloperAPIKey is a programmatic credential a user issues to call the API
// without a session. Only a SHA-256 hash of the key is stored, so a database
// dump does not disclose a usable credential; the plaintext is shown once, at
// creation.
type DeveloperAPIKey struct {
	ID     uuid.UUID `gorm:"type:uuid;primary_key" json:"id"`
	UserID string    `gorm:"index;size:100" json:"user_id"`
	Name   string    `gorm:"size:100" json:"name"`
	// KeyHash is the hex-encoded SHA-256 of the plaintext key.
	KeyHash string `gorm:"size:64;uniqueIndex" json:"-"`
	// KeyPrefix is a short, non-secret prefix shown in listings so a user can
	// tell two keys apart without the full value.
	KeyPrefix string `gorm:"size:12" json:"key_prefix"`
	// Scopes is a comma-separated scope list, or "*" for full access.
	Scopes string `gorm:"size:255" json:"scopes"`
	// RateLimit is the per-minute request budget for the key. Zero means the
	// service default.
	RateLimit int        `gorm:"default:0" json:"rate_limit"`
	IsActive  bool       `gorm:"default:true" json:"is_active"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	LastUsed  *time.Time `json:"last_used,omitempty"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

func (k *DeveloperAPIKey) BeforeCreate(tx *gorm.DB) error {
	if k.ID == uuid.Nil {
		k.ID = uuid.New()
	}
	return nil
}
