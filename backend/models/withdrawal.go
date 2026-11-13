package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Withdrawal address allow-list and limit configuration.
//
// A withdrawal address is added by the account owner and only becomes usable
// after a cooldown, so a stolen session cannot add an attacker's address and
// drain the account in the same window. A per-asset limit caps the value that
// may leave the platform; a withdrawal above the limit is not rejected, it is
// held in an approval queue for an operator to release or refuse.

// WithdrawalAddress is an allow-listed destination for one account. Addresses
// are unique per (user, chain, address) so a duplicate add is a no-op.
type WithdrawalAddress struct {
	ID      uuid.UUID `gorm:"type:uuid;primary_key" json:"id"`
	UserID  string    `gorm:"index:idx_withdrawal_address,unique;size:100" json:"user_id"`
	ChainID int64     `gorm:"index:idx_withdrawal_address,unique" json:"chain_id"`
	Address string    `gorm:"index:idx_withdrawal_address,unique;size:42" json:"address"`
	Label   string    `gorm:"size:64" json:"label"`
	// ReadyAt is when the address may first be used. It is set to now + cooldown
	// at creation and never moved earlier.
	ReadyAt   time.Time      `json:"ready_at"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (w *WithdrawalAddress) BeforeCreate(tx *gorm.DB) error {
	if w.ID == uuid.Nil {
		w.ID = uuid.New()
	}
	return nil
}

// WithdrawalLimit caps the amount of one asset (chain + token) that a single
// withdrawal may move. A zero limit means the operator has not set one, and the
// withdrawal proceeds without an approval step.
type WithdrawalLimit struct {
	ID           uuid.UUID `gorm:"type:uuid;primary_key" json:"id"`
	ChainID      int64     `gorm:"index:idx_withdrawal_limit,unique" json:"chain_id"`
	TokenAddress string    `gorm:"index:idx_withdrawal_limit,unique;size:42" json:"token_address"`
	// MaxAmount is an exact integer string in the token's smallest unit.
	MaxAmount string    `gorm:"size:78" json:"max_amount"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (w *WithdrawalLimit) BeforeCreate(tx *gorm.DB) error {
	if w.ID == uuid.Nil {
		w.ID = uuid.New()
	}
	return nil
}

// Withdrawal request statuses.
const (
	WithdrawalStatusPending  = "pending"
	WithdrawalStatusApproved = "approved"
	WithdrawalStatusRefused  = "refused"
)

// WithdrawalRequest is a withdrawal that exceeded its asset limit and is
// waiting for an operator. The funds are debited when the request is created,
// so an approved request has the balance reserved and a refused one returns it.
type WithdrawalRequest struct {
	ID           uuid.UUID `gorm:"type:uuid;primary_key" json:"id"`
	UserID       string    `gorm:"index;size:100" json:"user_id"`
	ChainID      int64     `json:"chain_id"`
	TokenAddress string    `gorm:"size:42" json:"token_address"`
	ToAddress    string    `gorm:"size:42" json:"to_address"`
	// Amount is an exact integer string in the token's smallest unit.
	Amount string `gorm:"size:78" json:"amount"`
	Status string `gorm:"index;size:20;default:'pending'" json:"status"`
	// Reason records why an operator refused the request.
	Reason    string    `json:"reason,omitempty"`
	TxHash    string    `gorm:"size:66" json:"tx_hash,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (w *WithdrawalRequest) BeforeCreate(tx *gorm.DB) error {
	if w.ID == uuid.Nil {
		w.ID = uuid.New()
	}
	return nil
}
