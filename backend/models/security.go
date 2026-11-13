package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// WalletLoginNonce is a single-use challenge issued by /auth/wallet/nonce and
// consumed exactly once by /auth/wallet/verify. The row is the replay guard: a
// signature over a nonce that has already been used, or has expired, is refused.
type WalletLoginNonce struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key" json:"id"`
	Address   string    `gorm:"index;size:42" json:"address"`
	Nonce     string    `gorm:"uniqueIndex;size:64" json:"nonce"`
	Statement string    `gorm:"size:255" json:"statement"`
	ChainID   int64     `json:"chain_id"`
	Domain    string    `gorm:"size:255" json:"domain"`
	Used      bool      `json:"used"`
	ExpiresAt time.Time `gorm:"index" json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// BeforeCreate assigns the primary key so a nonce created without an explicit id
// still has one.
func (n *WalletLoginNonce) BeforeCreate(tx *gorm.DB) error {
	if n.ID == uuid.Nil {
		n.ID = uuid.New()
	}
	return nil
}

// RefreshToken is a rotating opaque refresh credential. Only the SHA-256 hash of
// the token is stored, so a database read cannot yield a usable credential. All
// tokens issued from one sign-in share a FamilyID: presenting a token that was
// already rotated (a revocation) means the family was stolen, and every token in
// the family is revoked.
type RefreshToken struct {
	ID        uuid.UUID  `gorm:"type:uuid;primary_key" json:"id"`
	UserID    uuid.UUID  `gorm:"index" json:"user_id"`
	TokenHash string     `gorm:"uniqueIndex;size:64" json:"-"`
	FamilyID  uuid.UUID  `gorm:"index" json:"family_id"`
	Revoked   bool       `json:"revoked"`
	ReusedAt  *time.Time `json:"reused_at,omitempty"`
	ExpiresAt time.Time  `gorm:"index" json:"expires_at"`
	CreatedAt time.Time  `json:"created_at"`
}

// BeforeCreate assigns the primary key so a token row created without an
// explicit id still has one.
func (r *RefreshToken) BeforeCreate(tx *gorm.DB) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	return nil
}
