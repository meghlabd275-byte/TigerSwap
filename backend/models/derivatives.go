package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Listing status values shared by the futures, perps and margin listing tables.
const (
	ListingStatusActive      = "active"
	ListingStatusPaused      = "paused"
	ListingStatusDelisting   = "delisting"
	ListingStatusMaintenance = "maintenance"
)

// Settlement and contract-type values for futures markets. RUSD is the native
// settlement asset; USDT and USDC markets settle on the same self-hosted engine.
const (
	ContractTypeRUSDM  = "RUSD_M"
	ContractTypeUSDTM  = "USDT_M"
	ContractTypeUSDCM  = "USDC_M"
	ContractTypeRYUSDM = "RYUSD_M"
	ContractTypeCOINM  = "COIN_M"
	ContractTypeTGRM   = "TGR_M"
)

// Listing types supported by the futures engine.
const (
	ListingTypePerpetual = "perpetual"
	ListingTypeQuarterly = "quarterly"
	ListingTypeBiQuarter = "bi_quarterly"
	ListingTypeDelivery  = "delivery"
)

// FuturesMarket is an admin-managed futures contract listing. Rows are the
// single source of truth for the matching engine: a market is tradable only
// while its Status is "active".
type FuturesMarket struct {
	ID           uuid.UUID `gorm:"type:uuid;primary_key" json:"id"`
	Symbol       string    `gorm:"uniqueIndex;size:40" json:"symbol"` // e.g. BTC/RUSD
	ContractType string    `gorm:"size:16;index" json:"contract_type"`
	BaseAsset    string    `gorm:"size:20;index" json:"base_asset"`
	QuoteAsset   string    `gorm:"size:20" json:"quote_asset"`
	Settlement   string    `gorm:"size:20" json:"settlement"`

	ContractSize string `gorm:"size:40" json:"contract_size"`
	TickSize     string `gorm:"size:40" json:"tick_size"`
	MaxLeverage  int    `json:"max_leverage"`
	MinOrderSize string `gorm:"size:40" json:"min_order_size"`

	FundingRate     string `gorm:"size:40" json:"funding_rate"`
	FundingInterval int    `json:"funding_interval_hours"`
	FundingRateCap  string `gorm:"size:40" json:"funding_rate_cap"`

	OracleSources string `gorm:"type:text" json:"oracle_sources"`

	MaintenanceMarginRate float64 `json:"maintenance_margin_rate"`
	InitialMarginRate     float64 `json:"initial_margin_rate"`
	LiquidationFeeRate    float64 `json:"liquidation_fee_rate"`
	InsuranceFundShare    float64 `json:"insurance_fund_share"`
	MakerFeeRate          float64 `json:"maker_fee_rate"`
	TakerFeeRate          float64 `json:"taker_fee_rate"`

	Status      string `gorm:"size:20;index;default:'paused'" json:"status"`
	ListingType string `gorm:"size:20;default:'perpetual'" json:"listing_type"`
	IsTradFi    bool   `json:"is_tradfi"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (m *FuturesMarket) BeforeCreate(tx *gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return nil
}

// PerpsMarket is an admin-managed perpetual listing. Oracle markets are priced
// from the index feed with no order book; orderbook markets trade on the
// self-hosted CLOB.
type PerpsMarket struct {
	ID         uuid.UUID `gorm:"type:uuid;primary_key" json:"id"`
	Symbol     string    `gorm:"uniqueIndex;size:40" json:"symbol"`
	MarketType string    `gorm:"size:16;index;default:'oracle'" json:"market_type"` // oracle, orderbook

	CollateralAssets string  `gorm:"type:text" json:"collateral_assets"` // comma-separated
	MaxOpenInterest  string  `gorm:"size:60" json:"max_open_interest"`
	MaxLeverage      int     `json:"max_leverage"`
	PriceImpactRate  float64 `json:"price_impact_rate"`
	BorrowFeePerHour float64 `json:"borrow_fee_per_hour"`

	MakerFeeRate float64 `json:"maker_fee_rate"`
	TakerFeeRate float64 `json:"taker_fee_rate"`

	MaintenanceMarginRate float64 `json:"maintenance_margin_rate"`
	LiquidationFeeRate    float64 `json:"liquidation_fee_rate"`

	Status    string    `gorm:"size:20;index;default:'paused'" json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (m *PerpsMarket) BeforeCreate(tx *gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return nil
}

// Margin account status values.
const (
	MarginAccountActive     = "active"
	MarginAccountLiquidated = "liquidated"
	MarginAccountClosed     = "closed"
)

// MarginAccount is a margin trading account. A cross account pools all of its
// collateral and debt; an isolated account is scoped to a single pair, so a
// loss in one isolated account cannot touch another.
type MarginAccount struct {
	ID         uuid.UUID `gorm:"type:uuid;primary_key" json:"id"`
	UserID     string    `gorm:"index;size:100" json:"user_id"`
	MarginMode string    `gorm:"size:16;default:'cross'" json:"margin_mode"`
	Pair       string    `gorm:"size:40;index" json:"pair"`
	Status     string    `gorm:"size:20;index;default:'active'" json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (a *MarginAccount) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return nil
}

// MarginCollateral is one collateral asset held in a margin account.
type MarginCollateral struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key" json:"id"`
	AccountID uuid.UUID `gorm:"index" json:"account_id"`
	Asset     string    `gorm:"size:20;index" json:"asset"`
	Amount    string    `gorm:"size:60" json:"amount"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (c *MarginCollateral) BeforeCreate(tx *gorm.DB) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	return nil
}

// MarginBorrow is one borrowed asset within a margin account. Interest accrues
// from LastAccrual at the market's utilization-driven rate.
type MarginBorrow struct {
	ID              uuid.UUID `gorm:"type:uuid;primary_key" json:"id"`
	AccountID       uuid.UUID `gorm:"index" json:"account_id"`
	Asset           string    `gorm:"size:20;index" json:"asset"`
	Principal       string    `gorm:"size:60" json:"principal"`
	AccruedInterest string    `gorm:"size:60" json:"accrued_interest"`
	LastAccrual     time.Time `json:"last_accrual"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func (b *MarginBorrow) BeforeCreate(tx *gorm.DB) error {
	if b.ID == uuid.Nil {
		b.ID = uuid.New()
	}
	return nil
}

// MarginMarket is an admin-managed spot-margin listing with its own risk
// parameters and interest-rate curve.
type MarginMarket struct {
	ID         uuid.UUID `gorm:"type:uuid;primary_key" json:"id"`
	Pair       string    `gorm:"uniqueIndex;size:40" json:"pair"` // e.g. BTC/RUSD
	BaseAsset  string    `gorm:"size:20;index" json:"base_asset"`
	QuoteAsset string    `gorm:"size:20" json:"quote_asset"`
	MarginMode string    `gorm:"size:16;default:'cross'" json:"margin_mode"` // cross, isolated

	MaxLeverage int `json:"max_leverage"`

	CollateralAssets string `gorm:"type:text" json:"collateral_assets"`
	BorrowableAssets string `gorm:"type:text" json:"borrowable_assets"`

	BaseRateAPY     float64 `json:"base_rate_apy"`
	KinkUtilization float64 `json:"kink_utilization"`
	JumpMultiplier  float64 `json:"jump_multiplier"`

	LTV                  float64 `json:"ltv"`
	LiquidationThreshold float64 `json:"liquidation_threshold"`
	LiquidationBonus     float64 `json:"liquidation_bonus"`
	CloseFactor          float64 `json:"close_factor"`
	SupplyCap            string  `gorm:"size:60" json:"supply_cap"`
	BorrowCap            string  `gorm:"size:60" json:"borrow_cap"`

	RiskTier  string    `gorm:"size:16;default:'medium'" json:"risk_tier"` // low, medium, high
	Status    string    `gorm:"size:20;index;default:'paused'" json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (m *MarginMarket) BeforeCreate(tx *gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return nil
}
