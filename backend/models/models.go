package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// User represents a user account
type User struct {
	ID           uuid.UUID      `gorm:"type:uuid;primary_key" json:"id"`
	Email        string         `gorm:"uniqueIndex;size:255" json:"email"`
	Username     string         `gorm:"uniqueIndex;size:100" json:"username"`
	WalletAddress string        `gorm:"index;size:42" json:"wallet_address,omitempty"`
	Role         string         `gorm:"size:20;default:'user'" json:"role"` // user, admin
	IsVerified   bool           `gorm:"default:false" json:"is_verified"`
	IsActive     bool           `gorm:"default:true" json:"is_active"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`

	// Relations
	Tokens      []Token        `gorm:"foreignKey:CreatedBy" json:"-?"`
	Transactions []Transaction `gorm:"foreignKey:UserID" json:"-?"`
	Orders      []Order        `gorm:"foreignKey:UserID" json:"-?"`
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
	Tokens []Token `gorm:"foreignKey:ChainID" json:"-?"`
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
	ID                uint           `gorm:"primaryKey" json:"id"`
	TokenAAddress     string         `gorm:"size:42;index" json:"token_a_address"`
	TokenBAddress     string         `gorm:"size:42;index" json:"token_b_address"`
	ChainID           int64          `gorm:"index" json:"chain_id"`
	FactoryAddress    string         `gorm:"size:42" json:"factory_address"`
	PoolAddress       string         `gorm:"size:42;uniqueIndex" json:"pool_address"`
	ReserveA          string         `gorm:"size:50" json:"reserve_a"`
	ReserveB          string         `gorm:"size:50" json:"reserve_b"`
	TotalLiquidity    string         `gorm:"size:50" json:"total_liquidity"`
	Token0Price       string         `gorm:"size:50" json:"token_0_price"`
	Token1Price       string         `gorm:"size:50" json:"token_1_price"`
	Fee               int            `gorm:"default:30" json:"fee"` // 30 = 0.3%
	Volume24h         string         `gorm:"size:50" json:"volume_24h"`
	Volume7d          string         `gorm:"size:50" json:"volume_7d"`
	TxCount           uint64         `json:"tx_count"`
	IsActive          bool           `gorm:"default:true" json:"is_active"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
	DeletedAt         gorm.DeletedAt `gorm:"index" json:"-"`
}

// Transaction represents a swap transaction
type Transaction struct {
	ID            uuid.UUID      `gorm:"type:uuid;primary_key" json:"id"`
	UserID        uuid.UUID      `gorm:"type:uuid;index" json:"user_id"`
	Hash          string         `gorm:"size:66;uniqueIndex" json:"hash"`
	ChainID       int64          `gorm:"index" json:"chain_id"`
	Type          string         `gorm:"size:20" json:"type"` // swap, add_liquidity, remove_liquidity, bridge
	FromToken     string         `gorm:"size:42" json:"from_token"`
	ToToken       `gorm:"size:42" json:"to_token"`
	FromAmount    string         `gorm:"size:50" json:"from_amount"`
	ToAmount      string         `gorm:"size:50" json:"to_amount"`
	FromAddress   string         `gorm:"size:42" json:"from_address"`
	ToAddress     string         `gorm:"size:42" json:"to_address"`
	GasUsed       string         `gorm:"size:30" json:"gas_used"`
	GasPrice      string         `gorm:"size:30" json:"gas_price"`
	Status        string         `gorm:"size:20;default:'pending'" json:"status"` // pending, confirmed, failed
	BlockNumber   uint64         `json:"block_number"`
	Timestamp     time.Time      `json:"timestamp"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
}

func (t *Transaction) BeforeCreate(tx *gorm.DB) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	return nil
}

// Order represents a limit order
type Order struct {
	ID            uuid.UUID      `gorm:"type:uuid;primary_key" json:"id"`
	UserID        uuid.UUID      `gorm:"type:uuid;index" json:"user_id"`
	ChainID       int64          `gorm:"index" json:"chain_id"`
	OrderType     string         `gorm:"size:20" json:"order_type"` // limit, stop_loss, take_profit
	Side          string         `gorm:"size:10" json:"side"` // buy, sell
	TokenIn       string         `gorm:"size:42" json:"token_in"`
	TokenOut      string         `gorm:"size:42" json:"token_out"`
	AmountIn      string         `gorm:"size:50" json:"amount_in"`
	AmountOutMin  string         `gorm:"size:50" json:"amount_out_min"`
	Price         string         `gorm:"size:50" json:"price"` // For limit orders
	Status        string         `gorm:"size:20;default:'pending'" json:"status"` // pending, filled, cancelled, expired
	FilledAmount  string         `gorm:"size:50" json:"filled_amount"`
	GasUsed       string         `gorm:"size:30" json:"gas_used"`
	TxHash        string         `gorm:"size:66" json:"tx_hash,omitempty"`
	ExpiresAt     time.Time      `json:"expires_at"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
}

func (o *Order) BeforeCreate(tx *gorm.DB) error {
	if o.ID == uuid.Nil {
		o.ID = uuid.New()
	}
	return nil
}

// AdminLog represents admin actions
type AdminLog struct {
	ID         uint           `gorm:"primaryKey" json:"id"`
	AdminID    uuid.UUID      `gorm:"type:uuid;index" json:"admin_id"`
	Action     string         `gorm:"size:100" json:"action"`
	EntityType string         `gorm:"size:50" json:"entity_type"` // user, token, chain, pool
	EntityID   string         `gorm:"size:100" json:"entity_id"`
	Details    string         `gorm:"type:jsonb" json:"details"`
	IPAddress  string         `gorm:"size:45" json:"ip_address"`
	UserAgent  string         `gorm:"size:500" json:"user_agent"`
	CreatedAt  time.Time      `json:"created_at"`
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
	ID            uint      `gorm:"primaryKey" json:"id"`
	TotalTVL      string    `gorm:"size:50" json:"total_tvl"`
	Volume24h     string    `gorm:"size:50" json:"volume_24h"`
	Volume7d      string    `gorm:"size:50" json:"volume_7d"`
	Fees24h       string    `gorm:"size:50" json:"fees_24h"`
	UserCount     uint64    `json:"user_count"`
	PoolCount     uint64    `json:"pool_count"`
	TokenCount    uint64    `json:"token_count"`
	ChainCount    uint64    `json:"chain_count"`
	UpdatedAt     time.Time `json:"updated_at"`
}
