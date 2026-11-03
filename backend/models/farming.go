package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Farm types supported by the farm engine.
const (
	FarmTypeLP     = "lp"
	FarmTypeSingle = "single"
	FarmTypeStable = "stable"
)

// FarmPool status values.
const (
	FarmStatusActive = "active"
	FarmStatusPaused = "paused"
	FarmStatusClosed = "closed"
)

// Airdrop campaign status values.
const (
	AirdropStatusActive = "active"
	AirdropStatusPaused = "paused"
	AirdropStatusClosed = "closed"
)

// FarmPool is an admin-managed staking farm. A farm takes a single staking
// token (an LP token, a stable LP token or a single asset) and pays one or more
// reward tokens. The row is the source of truth for the engine: only an
// `active` farm accepts stake.
type FarmPool struct {
	ID   uuid.UUID `gorm:"type:uuid;primary_key" json:"id"`
	Key  string    `gorm:"uniqueIndex;size:120" json:"key"`
	Name string    `gorm:"size:160;not null" json:"name"`

	FarmType string `gorm:"size:16;index;default:'lp'" json:"farm_type"`
	ChainID  int64  `gorm:"index" json:"chain_id"`

	StakingTokenAddress  string `gorm:"size:42" json:"staking_token_address"`
	StakingTokenSymbol   string `gorm:"size:20" json:"staking_token_symbol"`
	StakingTokenDecimals int    `json:"staking_token_decimals"`

	// BoostGaugeID links the farm to a governance gauge so a veTGR boost can be
	// applied to the staker's share of emissions.
	BoostGaugeID *uuid.UUID `gorm:"type:uuid" json:"boost_gauge_id,omitempty"`

	// TotalStaked is the sum of every open stake, in the staking token's
	// smallest unit.
	TotalStaked string `gorm:"size:78;default:'0'" json:"total_staked"`

	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`

	Status    string         `gorm:"size:20;index;default:'paused'" json:"status"`
	CreatedBy string         `gorm:"size:100" json:"created_by"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (m *FarmPool) BeforeCreate(tx *gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return nil
}

// FarmReward is one reward token stream of a farm. Accrual uses the MasterChef
// accumulator: AccRewardPerShare is scaled by 1e18 and only advances while the
// farm has stake, so the share is well defined for every staker.
type FarmReward struct {
	ID     uuid.UUID `gorm:"type:uuid;primary_key" json:"id"`
	FarmID uuid.UUID `gorm:"index;not null" json:"farm_id"`

	TokenAddress  string `gorm:"size:42" json:"token_address"`
	TokenSymbol   string `gorm:"size:20" json:"token_symbol"`
	TokenDecimals int    `json:"token_decimals"`

	// RatePerSecond is the emission rate in the reward token's smallest unit.
	RatePerSecond string `gorm:"size:78;default:'0'" json:"rate_per_second"`

	// AccRewardPerShare is the cumulative reward per staked unit, scaled by 1e18.
	AccRewardPerShare string    `gorm:"size:78;default:'0'" json:"acc_reward_per_share"`
	LastUpdateTime    time.Time `json:"last_update_time"`

	// TotalReward is the amount funded into escrow for this stream;
	// TotalDistributed is what has been settled to stakers so far. The
	// difference is the remaining budget and is never exceeded.
	TotalReward      string `gorm:"size:78;default:'0'" json:"total_reward"`
	TotalDistributed string `gorm:"size:78;default:'0'" json:"total_distributed"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (m *FarmReward) BeforeCreate(tx *gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return nil
}

// FarmStake is a user's position in a farm.
type FarmStake struct {
	ID     uuid.UUID `gorm:"type:uuid;primary_key" json:"id"`
	FarmID uuid.UUID `gorm:"index:idx_farm_stake,unique;not null" json:"farm_id"`
	UserID string    `gorm:"index:idx_farm_stake,unique;size:100;not null" json:"user_id"`

	// Amount is the staked balance in the staking token's smallest unit.
	Amount string `gorm:"size:78;default:'0'" json:"amount"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (m *FarmStake) BeforeCreate(tx *gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return nil
}

// FarmStakeReward tracks one reward stream for one stake. Debt is the
// accumulator checkpoint; Accrued is the settled, unclaimed balance.
type FarmStakeReward struct {
	ID       uuid.UUID `gorm:"type:uuid;primary_key" json:"id"`
	StakeID  uuid.UUID `gorm:"index:idx_stake_reward,unique;not null" json:"stake_id"`
	RewardID uuid.UUID `gorm:"index:idx_stake_reward,unique;not null" json:"reward_id"`
	FarmID   uuid.UUID `gorm:"index;not null" json:"farm_id"`
	UserID   string    `gorm:"index;size:100;not null" json:"user_id"`

	Debt    string `gorm:"size:78;default:'0'" json:"debt"`
	Accrued string `gorm:"size:78;default:'0'" json:"accrued"`
	Claimed string `gorm:"size:78;default:'0'" json:"claimed"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (m *FarmStakeReward) BeforeCreate(tx *gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return nil
}

// AirdropCampaign is an admin-managed token distribution. Allocations are
// explicit per-user amounts; a user can claim at most their allocation once.
type AirdropCampaign struct {
	ID   uuid.UUID `gorm:"type:uuid;primary_key" json:"id"`
	Key  string    `gorm:"uniqueIndex;size:120" json:"key"`
	Name string    `gorm:"size:160;not null" json:"name"`

	ChainID       int64  `gorm:"index" json:"chain_id"`
	TokenAddress  string `gorm:"size:42" json:"token_address"`
	TokenSymbol   string `gorm:"size:20" json:"token_symbol"`
	TokenDecimals int    `json:"token_decimals"`

	// TotalAllocated is the sum of every allocation; Funded is the amount moved
	// into escrow. A claim is refused unless Funded covers the allocation.
	TotalAllocated string `gorm:"size:78;default:'0'" json:"total_allocated"`
	Funded         string `gorm:"size:78;default:'0'" json:"funded"`
	TotalClaimed   string `gorm:"size:78;default:'0'" json:"total_claimed"`

	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`

	Status    string         `gorm:"size:20;index;default:'paused'" json:"status"`
	CreatedBy string         `gorm:"size:100" json:"created_by"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (m *AirdropCampaign) BeforeCreate(tx *gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return nil
}

// AirdropAllocation is one user's entitlement in a campaign.
type AirdropAllocation struct {
	ID         uuid.UUID `gorm:"type:uuid;primary_key" json:"id"`
	CampaignID uuid.UUID `gorm:"index:idx_airdrop_allocation,unique;not null" json:"campaign_id"`
	UserID     string    `gorm:"index:idx_airdrop_allocation,unique;size:100;not null" json:"user_id"`

	Amount  string `gorm:"size:78;default:'0'" json:"amount"`
	Claimed bool   `gorm:"default:false" json:"claimed"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (m *AirdropAllocation) BeforeCreate(tx *gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return nil
}
