package models

import (
	"time"
)

// BotStrategy names a strategy the DEX can run. Every value here has a matching
// engine in the Rust `tigerswap-bots` crate; the row is the operator's
// configuration for that engine.
type BotStrategy string

const (
	StrategyMarketMaker BotStrategy = "market_maker"
	StrategyGrid        BotStrategy = "grid"
	StrategyDCA         BotStrategy = "dca"
	StrategyTWAP        BotStrategy = "twap"
	StrategyArbitrage   BotStrategy = "arbitrage"
	StrategySniper      BotStrategy = "sniper"
	StrategyRebalance   BotStrategy = "rebalance"
	StrategyLiquidation BotStrategy = "liquidation"
	StrategyCopy        BotStrategy = "copy"
)

// BotStatus is the lifecycle state of a bot instance.
type BotStatus string

const (
	BotStatusActive  BotStatus = "active"
	BotStatusPaused  BotStatus = "paused"
	BotStatusStopped BotStatus = "stopped"
)

// BotInstance is a configured, running bot. The configuration is stored as
// validated JSON so a new strategy can be added without a schema change, while
// the columns the API filters on stay indexed.
type BotInstance struct {
	ID        uint        `gorm:"primaryKey" json:"id"`
	UserID    uint        `gorm:"index;not null" json:"user_id"`
	Name      string      `gorm:"size:128;not null" json:"name"`
	Strategy  BotStrategy `gorm:"size:32;index;not null" json:"strategy"`
	Symbol    string      `gorm:"size:41;index;not null" json:"symbol"`
	Status    BotStatus   `gorm:"size:16;index;not null;default:active" json:"status"`
	Config    string      `gorm:"type:text;not null" json:"-"`

	// Risk limits the engine enforces before emitting any order.
	MaxOrderSize  string `gorm:"size:78;not null" json:"max_order_size"`
	MaxDailyLoss  string `gorm:"size:78;not null" json:"max_daily_loss"`
	MaxOpenOrders int    `gorm:"not null;default:10" json:"max_open_orders"`

	// Realised results, maintained from fills rather than estimated.
	RealisedPnL   string `gorm:"size:78;not null;default:0" json:"realised_pnl"`
	FilledOrders  int64  `gorm:"not null;default:0" json:"filled_orders"`
	TradedVolume  string `gorm:"size:78;not null;default:0" json:"traded_volume"`

	LastRunAt time.Time `json:"last_run_at"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (BotInstance) TableName() string {
	return "bot_instances"
}

// BotOrder is an order a bot engine decided to place. It is persisted so the
// bot's decisions are auditable and a restart does not lose in-flight work.
type BotOrder struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	BotID      uint      `gorm:"index;not null" json:"bot_id"`
	UserID     uint      `gorm:"index;not null" json:"user_id"`
	Symbol     string    `gorm:"size:41;index;not null" json:"symbol"`
	Side       string    `gorm:"size:8;not null" json:"side"`
	OrderKind  string    `gorm:"size:16;not null" json:"order_kind"`
	Price      string    `gorm:"size:78;not null" json:"price"`
	Quantity   string    `gorm:"size:78;not null" json:"quantity"`
	ReduceOnly bool      `gorm:"not null;default:false" json:"reduce_only"`
	Source     string    `gorm:"size:32;not null" json:"source"`
	// Status is pending, filled, cancelled or rejected.
	Status    string    `gorm:"size:16;index;not null;default:pending" json:"status"`
	TxHash    string    `gorm:"size:66" json:"tx_hash,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (BotOrder) TableName() string {
	return "bot_orders"
}

// BotPerformance is one realised performance record per bot per day.
type BotPerformance struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	BotID         uint      `gorm:"index:idx_bot_perf_day,unique;not null" json:"bot_id"`
	Day           time.Time `gorm:"index:idx_bot_perf_day,unique" json:"day"`
	OrdersPlaced  int64     `gorm:"not null;default:0" json:"orders_placed"`
	OrdersFilled  int64     `gorm:"not null;default:0" json:"orders_filled"`
	Volume        string    `gorm:"size:78;not null;default:0" json:"volume"`
	RealisedPnL   string    `gorm:"size:78;not null;default:0" json:"realised_pnl"`
	FeesPaid      string    `gorm:"size:78;not null;default:0" json:"fees_paid"`
	MaxDrawdown   string    `gorm:"size:78;not null;default:0" json:"max_drawdown"`
}

func (BotPerformance) TableName() string {
	return "bot_performance"
}

// OtcOrder is a private block trade. It never touches the public order book, so
// it produces no price impact; the two counterparties are matched directly and
// the settlement is recorded here.
type OtcOrder struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	MakerID        uint      `gorm:"index;not null" json:"maker_id"`
	TakerID        *uint     `gorm:"index" json:"taker_id,omitempty"`
	Symbol         string    `gorm:"size:41;index;not null" json:"symbol"`
	Side           string    `gorm:"size:8;not null" json:"side"`
	Quantity       string    `gorm:"size:78;not null" json:"quantity"`
	Price          string    `gorm:"size:78;not null" json:"price"`
	// Status is open, matched, settled or cancelled.
	Status         string    `gorm:"size:16;index;not null;default:open" json:"status"`
	MinFill        string    `gorm:"size:78;not null;default:0" json:"min_fill"`
	Counterparty   string    `gorm:"size:42" json:"counterparty,omitempty"`
	EscrowTxHash   string    `gorm:"size:66" json:"escrow_tx_hash,omitempty"`
	SettleTxHash   string    `gorm:"size:66" json:"settle_tx_hash,omitempty"`
	ExpiresAt      time.Time `gorm:"index" json:"expires_at"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (OtcOrder) TableName() string {
	return "otc_orders"
}

// TwammOrder is a long-running order that executes in slices between a start
// and an end time. The virtual order curve is what makes it un-sandwichable:
// there is no single transaction to front-run.
type TwammOrder struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	UserID         uint      `gorm:"index;not null" json:"user_id"`
	Symbol         string    `gorm:"size:41;index;not null" json:"symbol"`
	Side           string    `gorm:"size:8;not null" json:"side"`
	TotalQuantity  string    `gorm:"size:78;not null" json:"total_quantity"`
	FilledQuantity string    `gorm:"size:78;not null;default:0" json:"filled_quantity"`
	// LimitPrice is the worst price acceptable; zero means no limit.
	LimitPrice string    `gorm:"size:78;not null;default:0" json:"limit_price"`
	StartTime  time.Time `gorm:"index;not null" json:"start_time"`
	EndTime    time.Time `gorm:"index;not null" json:"end_time"`
	Status     string    `gorm:"size:16;index;not null;default:active" json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (TwammOrder) TableName() string {
	return "twamm_orders"
}
