package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// User represents a user account
type User struct {
	ID            uuid.UUID      `gorm:"type:uuid;primary_key" json:"id"`
	Email         string         `gorm:"uniqueIndex;size:255" json:"email"`
	Username      string         `gorm:"uniqueIndex;size:100" json:"username"`
	PasswordHash  string         `json:"-" gorm:"size:255"`
	WalletAddress string         `gorm:"index;size:42" json:"wallet_address,omitempty"`
	Role          string         `gorm:"size:20;default:'user'" json:"role"` // user, admin
	IsVerified    bool           `gorm:"default:false" json:"is_verified"`
	IsActive      bool           `gorm:"default:true" json:"is_active"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`

	// Relations
	Tokens       []Token       `gorm:"foreignKey:CreatedBy" json:"-"`
	Transactions []Transaction `gorm:"foreignKey:UserID" json:"-"`
	Orders       []Order       `gorm:"foreignKey:UserID" json:"-"`
}

func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return nil
}

// Blockchain represents a supported blockchain
type Blockchain struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	ChainID     int64          `gorm:"uniqueIndex" json:"chain_id"`
	Name        string         `gorm:"size:100" json:"name"`
	Symbol      string         `gorm:"size:20" json:"symbol"`
	Icon        string         `gorm:"size:255" json:"icon"`
	RPCURL      string         `gorm:"size:500" json:"rpc_url"`
	ExplorerURL string         `gorm:"size:500" json:"explorer_url"`
	Type        string         `gorm:"size:20" json:"type"` // evm, solana, cosmos, etc.
	IsActive    bool           `gorm:"default:true" json:"is_active"`
	IsTestnet   bool           `gorm:"default:false" json:"is_testnet"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`

	// Relations
	Tokens []Token `gorm:"foreignKey:ChainID" json:"-"`
}

// Token represents a cryptocurrency token
type Token struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	Address     string         `gorm:"size:42;index" json:"address"`
	ChainID     int64          `gorm:"index" json:"chain_id"`
	Name        string         `gorm:"size:100" json:"name"`
	Symbol      string         `gorm:"size:20;index" json:"symbol"`
	Decimals    int            `json:"decimals"`
	LogoURI     string         `gorm:"size:500" json:"logo_uri"`
	TotalSupply string         `gorm:"size:50" json:"total_supply"`
	IsActive    bool           `gorm:"default:true" json:"is_active"`
	IsVerified  bool           `gorm:"default:false" json:"is_verified"`
	CoingeckoID string         `gorm:"size:100" json:"coingecko_id,omitempty"`
	Price       float64        `gorm:"-" json:"price"`
	CreatedBy   uuid.UUID      `gorm:"type:uuid" json:"created_by,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

// Pool represents a liquidity pool
type Pool struct {
	ID             uint           `gorm:"primaryKey" json:"id"`
	TokenAAddress  string         `gorm:"size:42;index" json:"token_a_address"`
	TokenBAddress  string         `gorm:"size:42;index" json:"token_b_address"`
	ChainID        int64          `gorm:"index" json:"chain_id"`
	FactoryAddress string         `gorm:"size:42" json:"factory_address"`
	PoolAddress    string         `gorm:"size:42;uniqueIndex" json:"pool_address"`
	ReserveA       string         `gorm:"size:50" json:"reserve_a"`
	ReserveB       string         `gorm:"size:50" json:"reserve_b"`
	TotalLiquidity string         `gorm:"size:50" json:"total_liquidity"`
	Token0Price    string         `gorm:"size:50" json:"token_0_price"`
	Token1Price    string         `gorm:"size:50" json:"token_1_price"`
	Fee            int            `gorm:"default:30" json:"fee"` // 30 = 0.3%
	Volume24h      string         `gorm:"size:50" json:"volume_24h"`
	Volume7d       string         `gorm:"size:50" json:"volume_7d"`
	TvlUSD         float64        `json:"tvl_usd"`
	TxCount        uint64         `json:"tx_count"`
	IsActive       bool           `gorm:"default:true" json:"is_active"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
}

// Transaction represents a swap transaction
type Transaction struct {
	ID          uuid.UUID      `gorm:"type:uuid;primary_key" json:"id"`
	UserID      uuid.UUID      `gorm:"type:uuid;index" json:"user_id"`
	Hash        string         `gorm:"size:66;uniqueIndex" json:"hash"`
	ChainID     int64          `gorm:"index" json:"chain_id"`
	Type        string         `gorm:"size:20" json:"type"` // swap, add_liquidity, remove_liquidity, bridge
	FromToken   string         `gorm:"size:42" json:"from_token"`
	ToToken     string         `gorm:"size:42" json:"to_token"`
	FromAmount  string         `gorm:"size:50" json:"from_amount"`
	ToAmount    string         `gorm:"size:50" json:"to_amount"`
	FromAddress string         `gorm:"size:42" json:"from_address"`
	ToAddress   string         `gorm:"size:42" json:"to_address"`
	Pair        string         `gorm:"size:40;index" json:"pair"`
	AmountUSD   float64        `gorm:"index" json:"amount_usd"`
	GasUsed     string         `gorm:"size:30" json:"gas_used"`
	GasPrice    string         `gorm:"size:30" json:"gas_price"`
	Status      string         `gorm:"size:20;default:'pending'" json:"status"` // pending, confirmed, failed
	BlockNumber uint64         `json:"block_number"`
	Timestamp   time.Time      `json:"timestamp"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

func (t *Transaction) BeforeCreate(tx *gorm.DB) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	return nil
}

// Order represents a limit order
type Order struct {
	ID           uuid.UUID      `gorm:"type:uuid;primary_key" json:"id"`
	UserID       uuid.UUID      `gorm:"type:uuid;index" json:"user_id"`
	ChainID      int64          `gorm:"index" json:"chain_id"`
	OrderType    string         `gorm:"size:20" json:"order_type"` // limit, stop_loss, take_profit
	Side         string         `gorm:"size:10" json:"side"`       // buy, sell
	TokenIn      string         `gorm:"size:42" json:"token_in"`
	TokenOut     string         `gorm:"size:42" json:"token_out"`
	AmountIn     string         `gorm:"size:50" json:"amount_in"`
	AmountOutMin string         `gorm:"size:50" json:"amount_out_min"`
	Price        string         `gorm:"size:50" json:"price"`                    // For limit orders
	Status       string         `gorm:"size:20;default:'pending'" json:"status"` // pending, filled, cancelled, expired
	FilledAmount string         `gorm:"size:50" json:"filled_amount"`
	GasUsed      string         `gorm:"size:30" json:"gas_used"`
	TxHash       string         `gorm:"size:66" json:"tx_hash,omitempty"`
	ExpiresAt    time.Time      `json:"expires_at"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

func (o *Order) BeforeCreate(tx *gorm.DB) error {
	if o.ID == uuid.Nil {
		o.ID = uuid.New()
	}
	return nil
}

// AdminLog represents admin actions
type AdminLog struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	AdminID    uuid.UUID `gorm:"type:uuid;index" json:"admin_id"`
	Action     string    `gorm:"size:100" json:"action"`
	EntityType string    `gorm:"size:50" json:"entity_type"` // user, token, chain, pool
	EntityID   string    `gorm:"size:100" json:"entity_id"`
	Details    string    `gorm:"type:jsonb" json:"details"`
	IPAddress  string    `gorm:"size:45" json:"ip_address"`
	UserAgent  string    `gorm:"size:500" json:"user_agent"`
	CreatedAt  time.Time `json:"created_at"`
}

// APIKey represents API keys for developers
type APIKey struct {
	ID          uuid.UUID      `gorm:"type:uuid;primary_key" json:"id"`
	UserID      uuid.UUID      `gorm:"type:uuid;index" json:"user_id"`
	Key         string         `gorm:"size:64;uniqueIndex" json:"key"`
	Name        string         `gorm:"size:100" json:"name"`
	Permissions string         `gorm:"type:jsonb" json:"permissions"`
	RateLimit   int            `gorm:"default:1000" json:"rate_limit"` // requests per hour
	IsActive    bool           `gorm:"default:true" json:"is_active"`
	LastUsed    *time.Time     `json:"last_used"`
	ExpiresAt   *time.Time     `json:"expires_at"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

func (a *APIKey) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return nil
}

// Webhook represents webhooks for notifications
type Webhook struct {
	ID          uuid.UUID      `gorm:"type:uuid;primary_key" json:"id"`
	UserID      uuid.UUID      `gorm:"type:uuid;index" json:"user_id"`
	URL         string         `gorm:"size:500" json:"url"`
	Events      string         `gorm:"type:jsonb" json:"events"` // swap_completed, order_filled, etc.
	Secret      string         `gorm:"size:64" json:"secret"`
	IsActive    bool           `gorm:"default:true" json:"is_active"`
	RetryCount  int            `gorm:"default:3" json:"retry_count"`
	LastSuccess *time.Time     `json:"last_success"`
	LastFailure *time.Time     `json:"last_failure"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

func (w *Webhook) BeforeCreate(tx *gorm.DB) error {
	if w.ID == uuid.Nil {
		w.ID = uuid.New()
	}
	return nil
}

// MarketStats represents market statistics
type MarketStats struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	TotalTVL   string    `gorm:"size:50" json:"total_tvl"`
	Volume24h  string    `gorm:"size:50" json:"volume_24h"`
	Volume7d   string    `gorm:"size:50" json:"volume_7d"`
	Fees24h    string    `gorm:"size:50" json:"fees_24h"`
	UserCount  uint64    `json:"user_count"`
	PoolCount  uint64    `json:"pool_count"`
	TokenCount uint64    `json:"token_count"`
	ChainCount uint64    `json:"chain_count"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// PerpetualPosition represents a perpetual trading position
type PerpetualPosition struct {
	ID          uuid.UUID  `gorm:"type:uuid;primary_key" json:"id"`
	UserID      string     `gorm:"index;size:100" json:"user_id"`
	Pair        string     `gorm:"index;size:40" json:"pair"`
	Side        string     `gorm:"size:10" json:"side"` // long, short
	Size        string     `gorm:"size:50" json:"size"`
	Collateral  string     `gorm:"size:50" json:"collateral"`
	EntryPrice  string     `gorm:"size:50" json:"entry_price"`
	MarkPrice   string     `gorm:"size:50" json:"mark_price"`
	Leverage    int        `json:"leverage"`
	Status      string     `gorm:"size:20;index" json:"status"` // open, closed, liquidated
	PNL         string     `gorm:"size:50" json:"pnl"`
	FundingPaid string     `gorm:"size:50" json:"funding_paid"`
	OpenedAt    time.Time  `json:"opened_at"`
	ClosedAt    *time.Time `json:"closed_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

func (p *PerpetualPosition) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}

// CopyTrade represents a copy trade relationship
type CopyTrade struct {
	ID              uuid.UUID `gorm:"type:uuid;primary_key" json:"id"`
	TraderAddress   string    `gorm:"index;size:42" json:"trader_address"`
	FollowerAddress string    `gorm:"index;size:42" json:"follower_address"`
	Pair            string    `gorm:"size:40" json:"pair"`
	Amount          string    `gorm:"size:50" json:"amount"`
	Side            string    `gorm:"size:10" json:"side"`
	Status          string    `gorm:"size:20" json:"status"`
	CopiedAt        time.Time `json:"copied_at"`
	CreatedAt       time.Time `json:"created_at"`
}

func (c *CopyTrade) BeforeCreate(tx *gorm.DB) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	return nil
}

// LaunchpadProject represents an IDO/IEO launchpad project
type LaunchpadProject struct {
	ID            uuid.UUID `gorm:"type:uuid;primary_key" json:"id"`
	Name          string    `gorm:"size:200" json:"name"`
	Description   string    `gorm:"type:text" json:"description"`
	TokenAddress  string    `gorm:"size:42" json:"token_address"`
	TokenSymbol   string    `gorm:"size:20" json:"token_symbol"`
	TokenName     string    `gorm:"size:100" json:"token_name"`
	TotalSupply   string    `gorm:"size:50" json:"total_supply"`
	PricePerToken string    `gorm:"size:50" json:"price_per_token"`
	PaymentToken  string    `gorm:"size:42" json:"payment_token"`
	MinPurchase   string    `gorm:"size:50" json:"min_purchase"`
	MaxPurchase   string    `gorm:"size:50" json:"max_purchase"`
	SoftCap       string    `gorm:"size:50" json:"soft_cap"`
	HardCap       string    `gorm:"size:50" json:"hard_cap"`
	StartTime     time.Time `json:"start_time"`
	EndTime       time.Time `json:"end_time"`
	Status        string    `gorm:"size:20;index" json:"status"`
	RaisedAmount  string    `gorm:"size:50" json:"raised_amount"`
	Participants  int       `json:"participants"`
	WebsiteURL    string    `gorm:"size:255" json:"website_url"`
	WhitepaperURL string    `gorm:"size:255" json:"whitepaper_url"`
	LogoURL       string    `gorm:"size:255" json:"logo_url"`
	CreatedBy     string    `gorm:"size:100" json:"created_by"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (p *LaunchpadProject) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}

// MasterWallet stores an encrypted admin master wallet.
type MasterWallet struct {
	ID              uuid.UUID `gorm:"type:uuid;primary_key" json:"id"`
	Address         string    `gorm:"uniqueIndex;size:42" json:"address"`
	EncryptedKey    string    `gorm:"type:text" json:"-"`
	SeedEncrypted   string    `gorm:"type:text" json:"-"`
	BackupCodeHash  string    `gorm:"size:255" json:"-"`
	SupportedChains []int64   `gorm:"serializer:json" json:"supported_chains"`
	AutoSignEnabled bool      `json:"auto_sign_enabled"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func (m *MasterWallet) BeforeCreate(tx *gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return nil
}

// FeeSettings stores protocol fee configuration (singleton row, key="default").
type FeeSettings struct {
	ID                    uint      `gorm:"primaryKey" json:"id"`
	Key                   string    `gorm:"uniqueIndex;size:32" json:"key"`
	WithdrawFeePercent    float64   `json:"withdraw_fee_percent"`
	SwapFeePercent        float64   `json:"swap_fee_percent"`
	TransactionFeePercent float64   `json:"transaction_fee_percent"`
	DepositFeePercent     float64   `json:"deposit_fee_percent"`
	LaunchpadFeePercent   float64   `json:"launchpad_fee_percent"`
	UpdatedAt             time.Time `json:"updated_at"`
}

// BridgeTransfer records a TigerBridge cross-chain lock/mint transfer.
type BridgeTransfer struct {
	ID             uuid.UUID  `gorm:"type:uuid;primary_key" json:"id"`
	TransferID     string     `gorm:"uniqueIndex;size:80" json:"transfer_id"`
	SrcChain       int64      `gorm:"index" json:"src_chain"`
	DstChain       int64      `gorm:"index" json:"dst_chain"`
	Sender         string     `gorm:"index;size:42" json:"sender"`
	Recipient      string     `gorm:"index;size:42" json:"recipient"`
	Token          string     `gorm:"size:42" json:"token"`
	Amount         string     `gorm:"size:80" json:"amount"`
	SrcTxHash      string     `gorm:"size:80" json:"src_tx_hash"`
	DstTxHash      string     `gorm:"size:80" json:"dst_tx_hash"`
	Nonce          uint64     `json:"nonce"`
	Status         string     `gorm:"size:20;index" json:"status"`
	Fee            string     `gorm:"size:80" json:"fee"`
	FeeUSD         float64    `json:"fee_usd"`
	AmountReceived string     `gorm:"size:80" json:"amount_received"`
	ConfirmedAt    *time.Time `json:"confirmed_at,omitempty"`
	DeliveredAt    *time.Time `json:"delivered_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

func (b *BridgeTransfer) BeforeCreate(tx *gorm.DB) error {
	if b.ID == uuid.Nil {
		b.ID = uuid.New()
	}
	return nil
}
