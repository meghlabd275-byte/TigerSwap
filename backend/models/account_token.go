package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Account token purposes. Each purpose is a distinct namespace so a token
// minted for one flow can never be redeemed in another.
const (
	AccountTokenPasswordReset     = "password_reset"
	AccountTokenEmailVerification = "email_verification"
)

// AccountToken is a single-use, hashed, expiring token backing the account
// recovery flows (password reset, email verification). Only the SHA-256 hash of
// the token is stored, so a database read cannot yield a usable link.
type AccountToken struct {
	ID        uuid.UUID  `gorm:"type:uuid;primary_key" json:"id"`
	UserID    uuid.UUID  `gorm:"index" json:"user_id"`
	Purpose   string     `gorm:"index;size:32" json:"purpose"`
	TokenHash string     `gorm:"uniqueIndex;size:64" json:"-"`
	Used      bool       `json:"used"`
	UsedAt    *time.Time `json:"used_at,omitempty"`
	ExpiresAt time.Time  `gorm:"index" json:"expires_at"`
	CreatedAt time.Time  `json:"created_at"`
}

// BeforeCreate assigns the primary key so a token row created without an
// explicit id still has one.
func (a *AccountToken) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return nil
}
