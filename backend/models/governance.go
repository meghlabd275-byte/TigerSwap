package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Gauge types supported by the emissions controller.
const (
	GaugeTypePool     = "pool"
	GaugeTypeVault    = "vault"
	GaugeTypeLending  = "lending"
	GaugeTypePerps    = "perps"
	GaugeTypeExternal = "external"
)

// VeLock records a veTGR lock. Voting power decays linearly from the lock
// start to UnlockTime, so it is derived rather than stored.
//
// The locked Amount is escrowed from the owner's ledger balance at creation and
// returned on withdrawal. The chain/token columns record which ledger asset was
// escrowed, so a withdrawal credits back exactly the asset that was debited
// rather than trusting a caller-supplied token.
type VeLock struct {
	ID           uuid.UUID `gorm:"type:uuid;primary_key" json:"id"`
	UserID       string    `gorm:"index;size:100;not null" json:"user_id"`
	Amount       string    `gorm:"size:78;not null" json:"amount"`
	ChainID      int64     `gorm:"index" json:"chain_id"`
	TokenAddress string    `gorm:"size:100" json:"token_address"`
	StartTime    time.Time `gorm:"not null" json:"start_time"`
	UnlockTime   time.Time `gorm:"index;not null" json:"unlock_time"`
	Withdrawn    bool      `gorm:"default:false" json:"withdrawn"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (m *VeLock) BeforeCreate(tx *gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return nil
}

// Gauge is an emission target. Each epoch the admin distributes TGR across
// gauges according to their recorded vote weight.
type Gauge struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key" json:"id"`
	Name      string    `gorm:"size:120;not null" json:"name"`
	GaugeType string    `gorm:"size:32;index;not null" json:"gauge_type"`
	// Reference is the pool, vault or market this gauge rewards.
	Reference  string    `gorm:"size:255;index" json:"reference"`
	Weight     string    `gorm:"size:78;default:'0'" json:"weight"`
	TotalVoted string    `gorm:"size:78;default:'0'" json:"total_voted"`
	Active     bool      `gorm:"default:true" json:"active"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (m *Gauge) BeforeCreate(tx *gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return nil
}

// GaugeVote is one account's vote on one gauge. The composite unique index on
// (user_id, gauge_id) makes a second vote from the same account a conflict the
// vote path resolves as an update, so a duplicate row can never double that
// account's weight in the bribe distribution.
type GaugeVote struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key" json:"id"`
	UserID    string    `gorm:"index;size:100;not null;uniqueIndex:idx_gauge_vote_user_gauge,priority:1" json:"user_id"`
	GaugeID   uuid.UUID `gorm:"index;not null;uniqueIndex:idx_gauge_vote_user_gauge,priority:2" json:"gauge_id"`
	Weight    string    `gorm:"size:78;not null" json:"weight"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (m *GaugeVote) BeforeCreate(tx *gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return nil
}

// Bribe is a reward offered to voters of a gauge for a specific epoch. Each
// user may claim at most once per (gauge, epoch, token).
type Bribe struct {
	ID      uuid.UUID `gorm:"type:uuid;primary_key" json:"id"`
	GaugeID uuid.UUID `gorm:"index;not null" json:"gauge_id"`
	Epoch   int64     `gorm:"index;not null" json:"epoch"`
	Token   string    `gorm:"size:32;not null" json:"token"`
	Amount  string    `gorm:"size:78;not null" json:"amount"`
	// Escrowed is the amount still held for unclaimed voters; it starts at Amount
	// and falls as claims are credited, so claims can never pay out more than the
	// funder deposited.
	Escrowed      string    `gorm:"size:78;not null;default:'0'" json:"escrowed"`
	TotalWeight   string    `gorm:"size:78;not null" json:"total_weight"`
	FunderID      string    `gorm:"size:100" json:"funder_id"`
	ChainID       int64     `gorm:"not null;default:0" json:"chain_id"`
	TokenAddress  string    `gorm:"size:64" json:"token_address"`
	TokenSymbol   string    `gorm:"size:32" json:"token_symbol"`
	TokenDecimals int       `gorm:"not null;default:18" json:"token_decimals"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (m *Bribe) BeforeCreate(tx *gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return nil
}

// BribeClaim records a completed bribe claim so it can never be paid twice.
type BribeClaim struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key" json:"id"`
	BribeID   uuid.UUID `gorm:"index:idx_bribe_user,unique;not null" json:"bribe_id"`
	UserID    string    `gorm:"index:idx_bribe_user,unique;size:100;not null" json:"user_id"`
	Amount    string    `gorm:"size:78;not null" json:"amount"`
	CreatedAt time.Time `json:"created_at"`
}

func (m *BribeClaim) BeforeCreate(tx *gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return nil
}
