package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// VolumeBucket is one rolled-up slice of confirmed swap volume for a
// (pair, chain, interval) key. The rollup job is idempotent: it replaces the
// bucket for a window from the confirmed transactions in that window, so
// re-running it after a reorg or a restart converges on the same numbers
// instead of double-counting. The unique index is what makes the replace safe.
//
// Amounts are stored as decimal strings for the same reason balances are: a
// notional that must reconcile against the ledger cannot be a float. The USD
// columns are float64 because they are a derived valuation for display, not a
// balance anyone can withdraw.
type VolumeBucket struct {
	ID uuid.UUID `gorm:"type:uuid;primary_key" json:"id"`

	// Pair is the market symbol (e.g. "TGR/RUSD"). ChainID scopes the bucket so
	// the same pair on two chains does not collide.
	Pair    string `gorm:"size:40;uniqueIndex:idx_volume_bucket,priority:2" json:"pair"`
	ChainID int64  `gorm:"uniqueIndex:idx_volume_bucket,priority:3" json:"chain_id"`

	// BucketStart is the inclusive start of the window, truncated to the
	// interval. BucketSeconds is the window length, so a reader knows how wide
	// the bucket is without guessing.
	BucketStart   time.Time `gorm:"uniqueIndex:idx_volume_bucket,priority:1;index" json:"bucket_start"`
	BucketSeconds int       `gorm:"uniqueIndex:idx_volume_bucket,priority:4" json:"bucket_seconds"`

	// Volume is the summed notional in quote terms, as an exact decimal string.
	Volume string `gorm:"size:78" json:"volume"`

	// VolumeUSD and FeesUSD are derived valuations for display.
	VolumeUSD float64 `json:"volume_usd"`
	FeesUSD   float64 `json:"fees_usd"`

	// TradeCount counts confirmed swaps folded into this bucket.
	TradeCount int64 `json:"trade_count"`

	// TakerVolumeUSD splits the flow by aggressor side so a reader can show buy
	// versus sell pressure without re-reading every transaction.
	TakerBuyVolumeUSD  float64 `json:"taker_buy_volume_usd"`
	TakerSellVolumeUSD float64 `json:"taker_sell_volume_usd"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (b *VolumeBucket) BeforeCreate(tx *gorm.DB) error {
	if b.ID == uuid.Nil {
		b.ID = uuid.New()
	}
	return nil
}
