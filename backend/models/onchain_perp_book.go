package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// OnChainBookPosition is the indexed mirror of a position held by the deployed
// TigerSwapPerpOrderBook contract. The chain is the source of truth; this row is
// a queryable copy so the API and the liquidation keeper can find a trader's
// position without replaying every log on each request. A position is keyed by
// (chain, market, trader) because the book is cross-margined per market.
type OnChainBookPosition struct {
	ID       uuid.UUID `gorm:"type:uuid;primary_key" json:"id"`
	ChainID  int64     `gorm:"uniqueIndex:idx_onchain_book_pos;index" json:"chain_id"`
	MarketID string    `gorm:"size:66;uniqueIndex:idx_onchain_book_pos;index" json:"market_id"`
	Trader   string    `gorm:"size:42;uniqueIndex:idx_onchain_book_pos;index" json:"trader"`
	Symbol   string    `gorm:"size:40;index" json:"symbol"`
	// UserID is resolved from the trader address when the platform knows the
	// wallet; empty for a wallet the platform does not custody.
	UserID string `gorm:"size:100;index" json:"user_id"`

	Size       string `gorm:"size:80" json:"size"`
	EntryPrice string `gorm:"size:80" json:"entry_price"`
	Margin     string `gorm:"size:80" json:"margin"`

	// Size is zero when the position is flat; the row is kept so a trader's
	// history is not lost and a re-index is idempotent.
	Status string `gorm:"size:20;index;default:'open'" json:"status"` // open, flat

	UpdatedBlock uint64     `gorm:"index" json:"updated_block"`
	ClosedBlock  uint64     `json:"closed_block"`
	OpenedAt     time.Time  `json:"opened_at"`
	ClosedAt     *time.Time `json:"closed_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

func (p *OnChainBookPosition) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}

// OnChainBookCursor tracks how far the order book event indexer has scanned a
// chain. It is per chain and per contract, so a contract upgrade or a redeploy
// can be indexed from a known block without rescanning genesis.
type OnChainBookCursor struct {
	ChainID   int64     `gorm:"primary_key" json:"chain_id"`
	Contract  string    `gorm:"primary_key;size:42" json:"contract"`
	LastBlock uint64    `json:"last_block"`
	UpdatedAt time.Time `json:"updated_at"`
}

// OnChainBookLiquidation records a liquidation the keeper submitted, so the
// operator has an audit trail and the keeper never re-submits a position it
// already liquidated.
type OnChainBookLiquidation struct {
	ID          uuid.UUID `gorm:"type:uuid;primary_key" json:"id"`
	ChainID     int64     `gorm:"uniqueIndex:idx_onchain_book_liq" json:"chain_id"`
	MarketID    string    `gorm:"size:66;uniqueIndex:idx_onchain_book_liq" json:"market_id"`
	Trader      string    `gorm:"size:42;uniqueIndex:idx_onchain_book_liq" json:"trader"`
	TxHash      string    `gorm:"size:66" json:"tx_hash"`
	BlockNumber uint64    `json:"block_number"`
	CreatedAt   time.Time `json:"created_at"`
}

func (l *OnChainBookLiquidation) BeforeCreate(tx *gorm.DB) error {
	if l.ID == uuid.Nil {
		l.ID = uuid.New()
	}
	return nil
}

// OnChainBookFill is the indexed mirror of an OrderFilled event on the deployed
// order book. The chain is the source of truth; this row lets the API show a
// trader's trade history without replaying every log. It is idempotent on
// (chain, tx hash, log index) so a replay or a reorg cannot double-count a fill.
type OnChainBookFill struct {
	ID       uuid.UUID `gorm:"type:uuid;primary_key" json:"id"`
	ChainID  int64     `gorm:"uniqueIndex:idx_onchain_book_fill;index" json:"chain_id"`
	TxHash   string    `gorm:"size:66;uniqueIndex:idx_onchain_book_fill" json:"tx_hash"`
	LogIndex uint      `gorm:"uniqueIndex:idx_onchain_book_fill" json:"log_index"`
	// OrderID is the resting order the fill consumed, or "0" for a marketable
	// taker fill that is not tied to a resting order.
	OrderID  string `gorm:"size:80;index" json:"order_id"`
	MarketID string `gorm:"size:66;index" json:"market_id"`
	Symbol   string `gorm:"size:40;index" json:"symbol"`
	Trader   string `gorm:"size:42;index" json:"trader"`
	IsMaker  bool   `json:"is_maker"`
	Price    string `gorm:"size:80" json:"price"`
	Quantity string `gorm:"size:80" json:"quantity"`
	Fee      string `gorm:"size:80" json:"fee"`

	BlockNumber uint64    `gorm:"index" json:"block_number"`
	CreatedAt   time.Time `json:"created_at"`
}

func (f *OnChainBookFill) BeforeCreate(tx *gorm.DB) error {
	if f.ID == uuid.Nil {
		f.ID = uuid.New()
	}
	return nil
}
