package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// OnChainPerpPosition is the indexed mirror of a position held by the deployed
// TigerSwapPerpetuals contract. The chain is the source of truth; this row is a
// queryable copy so the API can list a trader's positions without replaying
// every log on each request. Every field is written from a confirmed event, so
// a row can never describe a position the chain does not have.
type OnChainPerpPosition struct {
	ID      uuid.UUID `gorm:"type:uuid;primary_key" json:"id"`
	ChainID int64     `gorm:"uniqueIndex:idx_onchain_perp_pos;index" json:"chain_id"`
	// PositionID is the contract's positionId (bytes32 hex), unique per chain.
	PositionID string `gorm:"size:66;uniqueIndex:idx_onchain_perp_pos" json:"position_id"`
	MarketID   string `gorm:"size:66;index" json:"market_id"`
	Symbol     string `gorm:"size:40;index" json:"symbol"`
	// Trader is the on-chain address that owns the position.
	Trader string `gorm:"size:42;index" json:"trader"`
	// UserID is resolved from the trader address when the platform knows the
	// wallet; it is empty for a position opened by a wallet the platform does
	// not custody, and the API then keys the position by address.
	UserID string `gorm:"size:100;index" json:"user_id"`

	IsLong     bool   `gorm:"index" json:"is_long"`
	Size       string `gorm:"size:80" json:"size"`
	Collateral string `gorm:"size:80" json:"collateral"`
	EntryPrice string `gorm:"size:80" json:"entry_price"`
	MarkPrice  string `gorm:"size:80" json:"mark_price"`
	// RealizedPnL is set when the position closes or is liquidated. The column
	// name is explicit because GORM would otherwise split the trailing "PnL".
	RealizedPnL string `gorm:"column:realized_pnl;size:80" json:"realized_pnl"`

	Status string `gorm:"size:20;index;default:'open'" json:"status"` // open, closed, liquidated

	OpenedBlock uint64     `gorm:"index" json:"opened_block"`
	ClosedBlock uint64     `json:"closed_block"`
	OpenedAt    time.Time  `json:"opened_at"`
	ClosedAt    *time.Time `json:"closed_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

func (p *OnChainPerpPosition) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}

// OnChainPerpCursor tracks how far the perps event indexer has scanned a chain.
// It is per chain and per contract, so a contract upgrade or a redeploy can be
// indexed from a known block without rescanning genesis.
type OnChainPerpCursor struct {
	ChainID   int64     `gorm:"primary_key" json:"chain_id"`
	Contract  string    `gorm:"primary_key;size:42" json:"contract"`
	LastBlock uint64    `json:"last_block"`
	UpdatedAt time.Time `json:"updated_at"`
}

// OnChainPerpLiquidation records a liquidation the keeper submitted, so the
// operator has an audit trail and the keeper never re-submits a position it
// already liquidated.
type OnChainPerpLiquidation struct {
	ID          uuid.UUID `gorm:"type:uuid;primary_key" json:"id"`
	ChainID     int64     `gorm:"uniqueIndex:idx_onchain_perp_liq" json:"chain_id"`
	PositionID  string    `gorm:"size:66;uniqueIndex:idx_onchain_perp_liq" json:"position_id"`
	Trader      string    `gorm:"size:42;index" json:"trader"`
	TxHash      string    `gorm:"size:66" json:"tx_hash"`
	BlockNumber uint64    `json:"block_number"`
	CreatedAt   time.Time `json:"created_at"`
}

func (l *OnChainPerpLiquidation) BeforeCreate(tx *gorm.DB) error {
	if l.ID == uuid.Nil {
		l.ID = uuid.New()
	}
	return nil
}
