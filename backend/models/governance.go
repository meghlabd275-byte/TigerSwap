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
type VeLock struct {
	ID         uuid.UUID `gorm:"type:uuid;primary_key" json:"id"`
	UserID     string    `gorm:"index;size:100;not null" json:"user_id"`
	Amount     string    `gorm:"size:78;not null" json:"amount"`
	StartTime  time.Time `gorm:"not null" json:"start_time"`
	UnlockTime time.Time `gorm:"index;not null" json:"unlock_time"`
	Withdrawn  bool      `gorm:"default:false" json:"withdrawn"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
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

// GaugeVote is a user's current vote for a gauge.
type GaugeVote struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key" json:"id"`
	UserID    string    `gorm:"index;size:100;not null" json:"user_id"`
	GaugeID   uuid.UUID `gorm:"index;not null" json:"gauge_id"`
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
	ID          uuid.UUID `gorm:"type:uuid;primary_key" json:"id"`
	GaugeID     uuid.UUID `gorm:"index;not null" json:"gauge_id"`
	Epoch       int64     `gorm:"index;not null" json:"epoch"`
	Token       string    `gorm:"size:32;not null" json:"token"`
	Amount      string    `gorm:"size:78;not null" json:"amount"`
	TotalWeight string    `gorm:"size:78;not null" json:"total_weight"`
	FunderID    string    `gorm:"size:100" json:"funder_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
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
