package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Option series status values.
const (
	OptionStatusActive  = "active"
	OptionStatusPaused  = "paused"
	OptionStatusExpired = "expired"
	OptionStatusSettled = "settled"
)

// OptionSeries collateral must be one of the settlement assets the platform
// ledger understands. The engine refuses any other symbol, so a series cannot be
// created that the settlement path could never pay out.
var optionCollateralSymbols = map[string]bool{
	"RUSD":  true,
	"USDT":  true,
	"USDC":  true,
	"RYUSD": true,
}

// IsSupportedOptionCollateral reports whether a symbol may collateralise a
// series.
func IsSupportedOptionCollateral(symbol string) bool {
	return optionCollateralSymbols[symbol]
}

// OptionSeries is an admin-managed option series: a strike, an expiry, a call or
// put flag and a base implied volatility. It mirrors the on-chain
// `TigerSwapOptions.Series` struct so an off-chain row and an on-chain series
// describe the same instrument.
//
// Every monetary field is an exact decimal string in the collateral token's
// smallest unit. No float is stored, because a premium or a payout that loses a
// unit to rounding is a real loss to a user.
type OptionSeries struct {
	ID      uuid.UUID `gorm:"type:uuid;primary_key" json:"id"`
	Key     string    `gorm:"uniqueIndex;size:120" json:"key"`
	Name    string    `gorm:"size:160;not null" json:"name"`
	ChainID int64     `gorm:"index" json:"chain_id"`

	// UnderlyingSymbol is the priced asset, for example BTC or ETH. The series
	// settles in the collateral token below.
	UnderlyingSymbol string `gorm:"size:20;index" json:"underlying_symbol"`

	// CollateralTokenAddress/Symbol/Decimals describe the settlement asset
	// (RUSD, USDT or USDC).
	CollateralTokenAddress  string `gorm:"size:42" json:"collateral_token_address"`
	CollateralTokenSymbol   string `gorm:"size:20" json:"collateral_token_symbol"`
	CollateralTokenDecimals int    `json:"collateral_token_decimals"`

	// Strike and BaseIV are 1e18 fixed-point decimals. BaseIV of 0.8 means 80%.
	Strike string `gorm:"size:60" json:"strike"`
	BaseIV string `gorm:"size:60" json:"base_iv"`

	Expiry time.Time `gorm:"index" json:"expiry"`
	IsCall bool      `json:"is_call"`

	// MarkPrice is the current oracle price of the underlying, refreshed by an
	// operator or the price job. It is required before the series can be priced.
	MarkPrice string `gorm:"size:60" json:"mark_price"`

	// Collateral and LockedCollateral track the pool's free and committed
	// collateral, exactly as the on-chain series does.
	Collateral       string `gorm:"size:78;default:'0'" json:"collateral"`
	LockedCollateral string `gorm:"size:78;default:'0'" json:"locked_collateral"`
	PremiumCollected string `gorm:"size:78;default:'0'" json:"premium_collected"`
	TotalLongSize    string `gorm:"size:78;default:'0'" json:"total_long_size"`

	// SettlementPrice is set when the series settles.
	SettlementPrice *string `gorm:"size:60" json:"settlement_price,omitempty"`

	Status    string         `gorm:"size:20;index;default:'paused'" json:"status"`
	CreatedBy string         `gorm:"size:100" json:"created_by"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (m *OptionSeries) BeforeCreate(tx *gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return nil
}

// OptionPosition is a trader's long position in one series. `Size` is in 1e18
// fixed point; the premium paid is escrowed in the collateral token so the
// position is fully paid for.
type OptionPosition struct {
	ID       uuid.UUID `gorm:"type:uuid;primary_key" json:"id"`
	SeriesID uuid.UUID `gorm:"index;not null" json:"series_id"`
	UserID   string    `gorm:"size:100;index;not null" json:"user_id"`

	Size          string `gorm:"size:78;default:'0'" json:"size"`
	PremiumPaid   string `gorm:"size:78;default:'0'" json:"premium_paid"`
	ClaimedPayout string `gorm:"size:78;default:'0'" json:"claimed_payout"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (m *OptionPosition) BeforeCreate(tx *gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return nil
}

// OptionLiquidity records one LP's collateral deposit into a series, so an LP
// can only ever withdraw the principal they actually put in.
type OptionLiquidity struct {
	ID       uuid.UUID `gorm:"type:uuid;primary_key" json:"id"`
	SeriesID uuid.UUID `gorm:"index;not null" json:"series_id"`
	UserID   string    `gorm:"size:100;index;not null" json:"user_id"`

	Principal string `gorm:"size:78;default:'0'" json:"principal"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (m *OptionLiquidity) BeforeCreate(tx *gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return nil
}
