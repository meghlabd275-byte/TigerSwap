package database

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// ============================================================================
// Config & Connection
// ============================================================================

type Config struct {
	Host     string
	Port     string
	User     string
	Password string
	Database string
	MaxConns int
	SSLMode  string
}

func NewConfig() *Config {
	return &Config{
		Host:     getEnv("DB_HOST", "localhost"),
		Port:     getEnv("DB_PORT", "5432"),
		User:     getEnv("DB_USER", "tigerswap"),
		Password: getEnv("DB_PASSWORD", ""),
		Database: getEnv("DB_NAME", "tigerswap"),
		MaxConns: 100,
		SSLMode:  getEnv("DB_SSL_MODE", "require"),
	}
}

func (c *Config) ConnectionString() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s&pool_max_conns=%d",
		c.User, c.Password, c.Host, c.Port, c.Database, c.SSLMode, c.MaxConns,
	)
}

type DB struct {
	pool   *pgxpool.Pool
	config *Config
}

func New(config *Config) (*DB, error) {
	poolConfig, err := pgxpool.ParseConfig(config.ConnectionString())
	if err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	poolConfig.MaxConns = int32(config.MaxConns)
	poolConfig.MinConns = 10
	poolConfig.MaxConnLifetime = time.Hour
	poolConfig.MaxConnIdleTime = 30 * time.Minute
	poolConfig.HealthCheckPeriod = time.Minute

	pool, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create pool: %w", err)
	}

	if err := pool.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DB{pool: pool, config: config}, nil
}

func (db *DB) Close() {
	db.pool.Close()
}

func (db *DB) Ping(ctx context.Context) error {
	return db.pool.Ping(ctx)
}

// ============================================================================
// Transaction Support
// ============================================================================

type Transaction struct {
	tx pgxTx
}

type pgxTx interface {
	Exec(ctx context.Context, sql string, args ...interface{}) (interface{}, error)
	QueryRow(ctx context.Context, sql string, args ...interface{}) interface{}
}

func (db *DB) Begin(ctx context.Context) (*sql.Tx, error) {
	return nil, nil
}

func (db *DB) Stats() map[string]interface{} {
	stats := db.pool.Stat()
	return map[string]interface{}{
		"max_conns":  stats.MaxConns,
		"idle_conns": stats.IdleConns,
		"total_conns": stats.TotalConns,
	}
}

// ============================================================================
// USER Operations
// ============================================================================

type User struct {
	ID            uuid.UUID       `json:"id"`
	WalletAddress string          `json:"wallet_address"`
	Email         string          `json:"email"`
	Username      string          `json:"username"`
	PasswordHash  string          `json:"-"`
	AvatarURL     string          `json:"avatar_url"`
	RiskScore     int             `json:"risk_score"`
	KYCStatus     string          `json:"kyc_status"`
	IsVerified    bool            `json:"is_verified"`
	IsAdmin       bool            `json:"is_admin"`
	TotalVolume   decimal.Decimal `json:"total_volume_usd"`
	TotalPnL      decimal.Decimal `json:"total_pnl"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
	LastActiveAt  time.Time       `json:"last_active_at"`
}

func (db *DB) CreateUser(ctx context.Context, user *User) error {
	user.ID = uuid.New()
	user.CreatedAt = time.Now()
	user.UpdatedAt = time.Now()
	user.RiskScore = 100
	user.KYCStatus = "none"
	user.IsVerified = false

	sql := `
		INSERT INTO users (id, wallet_address, email, username, password_hash, risk_score, kyc_status, is_verified, is_admin, total_volume_usd, total_pnl, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`
	_, err := db.pool.Exec(ctx, sql,
		user.ID, strings.ToLower(user.WalletAddress), user.Email, user.Username, user.PasswordHash,
		user.RiskScore, user.KYCStatus, user.IsVerified, user.IsAdmin, user.TotalVolume.String(), user.TotalPnL.String(), user.CreatedAt, user.UpdatedAt,
	)
	return err
}

func (db *DB) GetUserByWallet(ctx context.Context, wallet string) (*User, error) {
	user := &User{}
	sql := `SELECT id, wallet_address, email, username, risk_score, kyc_status, is_verified, is_admin, total_volume_usd, total_pnl, created_at, updated_at FROM users WHERE LOWER(wallet_address) = LOWER($1)`
	
	row := db.pool.QueryRow(ctx, sql, wallet)
	
	var totalVol, totalPnL string
	err := row.Scan(&user.ID, &user.WalletAddress, &user.Email, &user.Username, &user.RiskScore, &user.KYCStatus, &user.IsVerified, &user.IsAdmin, &totalVol, &totalPnL, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, err
	}
	user.TotalVolume, _ = decimal.NewFromString(totalVol)
	user.TotalPnL, _ = decimal.NewFromString(totalPnL)
	return user, nil
}

func (db *DB) GetUserByID(ctx context.Context, id uuid.UUID) (*User, error) {
	user := &User{}
	sql := `SELECT id, wallet_address, email, username, risk_score, kyc_status, is_verified, is_admin, total_volume_usd, total_pnl, created_at, updated_at FROM users WHERE id = $1`
	
	row := db.pool.QueryRow(ctx, sql, id)
	var totalVol, totalPnL string
	err := row.Scan(&user.ID, &user.WalletAddress, &user.Email, &user.Username, &user.RiskScore, &user.KYCStatus, &user.IsVerified, &user.IsAdmin, &totalVol, &totalPnL, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, err
	}
	user.TotalVolume, _ = decimal.NewFromString(totalVol)
	user.TotalPnL, _ = decimal.NewFromString(totalPnL)
	return user, nil
}

func (db *DB) UpdateUserVolume(ctx context.Context, userID uuid.UUID, volumeDelta decimal.Decimal) error {
	sql := `UPDATE users SET total_volume_usd = total_volume_usd + $1, updated_at = NOW() WHERE id = $2`
	_, err := db.pool.Exec(ctx, sql, volumeDelta.String(), userID)
	return err
}

func (db *DB) ListUsers(ctx context.Context, limit, offset int) ([]*User, int, error) {
	var total int
	db.pool.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&total)

	sql := `SELECT id, wallet_address, email, username, risk_score, kyc_status, is_verified, is_admin, total_volume_usd, total_pnl, created_at, updated_at FROM users ORDER BY created_at DESC LIMIT $1 OFFSET $2`
	rows, err := db.pool.Query(ctx, sql, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		u := &User{}
		var totalVol, totalPnL string
		rows.Scan(&u.ID, &u.WalletAddress, &u.Email, &u.Username, &u.RiskScore, &u.KYCStatus, &u.IsVerified, &u.IsAdmin, &totalVol, &totalPnL, &u.CreatedAt, &u.UpdatedAt)
		u.TotalVolume, _ = decimal.NewFromString(totalVol)
		u.TotalPnL, _ = decimal.NewFromString(totalPnL)
		users = append(users, u)
	}
	return users, total, rows.Err()
}

// ============================================================================
// TOKEN Operations
// ============================================================================

type Token struct {
	ID              uuid.UUID       `json:"id"`
	Symbol          string          `json:"symbol"`
	Name            string          `json:"name"`
	ContractAddress string          `json:"contract_address"`
	ChainID         int             `json:"chain_id"`
	Decimals        int             `json:"decimals"`
	LogoURL         string          `json:"logo_url"`
	IsStablecoin    bool            `json:"is_stablecoin"`
	PriceUSD        decimal.Decimal `json:"price_usd"`
	MarketCap       decimal.Decimal `json:"market_cap"`
	Volume24h       decimal.Decimal `json:"volume_24h"`
	CreatedAt       time.Time       `json:"created_at"`
}

func (db *DB) CreateToken(ctx context.Context, token *Token) error {
	token.ID = uuid.New()
	token.CreatedAt = time.Now()

	sql := `
		INSERT INTO tokens (id, symbol, name, contract_address, chain_id, decimals, logo_url, is_stablecoin, price_usd, market_cap, volume_24h, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (chain_id, contract_address) DO UPDATE SET symbol = EXCLUDED.symbol, name = EXCLUDED.name
		RETURNING id
	`
	return db.pool.QueryRow(ctx, sql,
		token.ID, token.Symbol, token.Name, strings.ToLower(token.ContractAddress), token.ChainID,
		token.Decimals, token.LogoURL, token.IsStablecoin, token.PriceUSD.String(), token.MarketCap.String(), token.Volume24h.String(), token.CreatedAt,
	).Scan(&token.ID)
}

func (db *DB) GetTokenByAddress(ctx context.Context, chainID int, address string) (*Token, error) {
	token := &Token{}
	sql := `SELECT id, symbol, name, contract_address, chain_id, decimals, logo_url, is_stablecoin, price_usd, market_cap, volume_24h, created_at FROM tokens WHERE chain_id = $1 AND LOWER(contract_address) = LOWER($2)`
	
	row := db.pool.QueryRow(ctx, sql, chainID, address)
	var priceStr, marketCapStr, volStr string
	err := row.Scan(&token.ID, &token.Symbol, &token.Name, &token.ContractAddress, &token.ChainID, &token.Decimals, &token.LogoURL, &token.IsStablecoin, &priceStr, &marketCapStr, &volStr, &token.CreatedAt)
	if err != nil {
		return nil, err
	}
	token.PriceUSD, _ = decimal.NewFromString(priceStr)
	token.MarketCap, _ = decimal.NewFromString(marketCapStr)
	token.Volume24h, _ = decimal.NewFromString(volStr)
	return token, nil
}

func (db *DB) ListTokens(ctx context.Context, chainID int, limit, offset int) ([]*Token, int, error) {
	var total int
	db.pool.QueryRow(ctx, "SELECT COUNT(*) FROM tokens WHERE chain_id = $1", chainID).Scan(&total)

	sql := `SELECT id, symbol, name, contract_address, chain_id, decimals, logo_url, is_stablecoin, price_usd, market_cap, volume_24h, created_at FROM tokens WHERE chain_id = $1 ORDER BY market_cap DESC NULLS LAST LIMIT $2 OFFSET $3`
	rows, err := db.pool.Query(ctx, sql, chainID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var tokens []*Token
	for rows.Next() {
		t := &Token{}
		var priceStr, marketCapStr, volStr string
		rows.Scan(&t.ID, &t.Symbol, &t.Name, &t.ContractAddress, &t.ChainID, &t.Decimals, &t.LogoURL, &t.IsStablecoin, &priceStr, &marketCapStr, &volStr, &t.CreatedAt)
		t.PriceUSD, _ = decimal.NewFromString(priceStr)
		t.MarketCap, _ = decimal.NewFromString(marketCapStr)
		t.Volume24h, _ = decimal.NewFromString(volStr)
		tokens = append(tokens, t)
	}
	return tokens, total, rows.Err()
}

func (db *DB) UpdateTokenPrice(ctx context.Context, tokenID uuid.UUID, priceUSD decimal.Decimal) error {
	sql := `UPDATE tokens SET price_usd = $1 WHERE id = $2`
	_, err := db.pool.Exec(ctx, sql, priceUSD.String(), tokenID)
	return err
}

// ============================================================================
// POOL Operations
// ============================================================================

type Pool struct {
	ID           uuid.UUID       `json:"id"`
	DexID        uuid.UUID       `json:"dex_id"`
	PairID       uuid.UUID       `json:"pair_id"`
	PoolAddress  string          `json:"pool_address"`
	TokenAAddr   string          `json:"token_a_address"`
	TokenBAddr   string          `json:"token_b_address"`
	ReserveA     decimal.Decimal `json:"reserve_a"`
	ReserveB     decimal.Decimal `json:"reserve_b"`
	LiquidityUSD decimal.Decimal `json:"liquidity_usd"`
	FeeTierBps   int             `json:"fee_tier_bps"`
	TVLUSD       decimal.Decimal `json:"tvl_usd"`
	Volume24hUSD decimal.Decimal `json:"volume_24h_usd"`
	APR          decimal.Decimal `json:"apr"`
	IsActive     bool            `json:"is_active"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

func (db *DB) CreatePool(ctx context.Context, pool *Pool) error {
	pool.ID = uuid.New()
	pool.CreatedAt = time.Now()
	pool.UpdatedAt = time.Now()
	pool.IsActive = true

	sql := `
		INSERT INTO pools (id, dex_id, pair_id, pool_address, token_a_address, token_b_address, fee_tier_bps, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id
	`
	return db.pool.QueryRow(ctx, sql,
		pool.ID, pool.DexID, pool.PairID, strings.ToLower(pool.PoolAddress), strings.ToLower(pool.TokenAAddr), strings.ToLower(pool.TokenBAddr),
		pool.FeeTierBps, pool.IsActive, pool.CreatedAt, pool.UpdatedAt,
	).Scan(&pool.ID)
}

func (db *DB) GetPoolByAddress(ctx context.Context, poolAddress string) (*Pool, error) {
	pool := &Pool{}
	sql := `SELECT id, dex_id, pair_id, pool_address, token_a_address, token_b_address, reserve_a, reserve_b, liquidity_usd, fee_tier_bps, tvl_usd, volume_24h_usd, apr, is_active, created_at, updated_at FROM pools WHERE LOWER(pool_address) = LOWER($1)`
	
	row := db.pool.QueryRow(ctx, sql, poolAddress)
	var reserveAStr, reserveBStr, liqStr, tvlStr, volStr, aprStr string
	err := row.Scan(&pool.ID, &pool.DexID, &pool.PairID, &pool.PoolAddress, &pool.TokenAAddr, &pool.TokenBAddr, &reserveAStr, &reserveBStr, &liqStr, &pool.FeeTierBps, &tvlStr, &volStr, &aprStr, &pool.IsActive, &pool.CreatedAt, &pool.UpdatedAt)
	if err != nil {
		return nil, err
	}
	pool.ReserveA, _ = decimal.NewFromString(reserveAStr)
	pool.ReserveB, _ = decimal.NewFromString(reserveBStr)
	pool.LiquidityUSD, _ = decimal.NewFromString(liqStr)
	pool.TVLUSD, _ = decimal.NewFromString(tvlStr)
	pool.Volume24hUSD, _ = decimal.NewFromString(volStr)
	pool.APR, _ = decimal.NewFromString(aprStr)
	return pool, nil
}

func (db *DB) UpdatePoolReserves(ctx context.Context, poolAddress string, reserveA, reserveB, liquidityUSD decimal.Decimal) error {
	sql := `UPDATE pools SET reserve_a = $2, reserve_b = $3, liquidity_usd = $4, updated_at = NOW() WHERE LOWER(pool_address) = LOWER($1)`
	_, err := db.pool.Exec(ctx, sql, poolAddress, reserveA.String(), reserveB.String(), liquidityUSD.String())
	return err
}

func (db *DB) GetTopPools(ctx context.Context, chainID int, limit int) ([]*Pool, error) {
	sql := `SELECT id, dex_id, pair_id, pool_address, token_a_address, token_b_address, reserve_a, reserve_b, liquidity_usd, fee_tier_bps, tvl_usd, volume_24h_usd, apr, is_active, created_at, updated_at FROM pools WHERE is_active = true ORDER BY liquidity_usd DESC LIMIT $1`
	
	rows, err := db.pool.Query(ctx, sql, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pools []*Pool
	for rows.Next() {
		p := &Pool{}
		var reserveAStr, reserveBStr, liqStr, tvlStr, volStr, aprStr string
		rows.Scan(&p.ID, &p.DexID, &p.PairID, &p.PoolAddress, &p.TokenAAddr, &p.TokenBAddr, &reserveAStr, &reserveBStr, &liqStr, &p.FeeTierBps, &tvlStr, &volStr, &aprStr, &p.IsActive, &p.CreatedAt, &p.UpdatedAt)
		p.ReserveA, _ = decimal.NewFromString(reserveAStr)
		p.ReserveB, _ = decimal.NewFromString(reserveBStr)
		p.LiquidityUSD, _ = decimal.NewFromString(liqStr)
		p.TVLUSD, _ = decimal.NewFromString(tvlStr)
		p.Volume24hUSD, _ = decimal.NewFromString(volStr)
		p.APR, _ = decimal.NewFromString(aprStr)
		pools = append(pools, p)
	}
	return pools, rows.Err()
}

// ============================================================================
// ORDER Operations
// ============================================================================

type Order struct {
	ID           uuid.UUID       `json:"id"`
	OrderHash    string          `json:"order_hash"`
	UserID       uuid.UUID       `json:"user_id"`
	PairID       uuid.UUID       `json:"pair_id"`
	Side         string          `json:"side"`
	OrderType    string          `json:"order_type"`
	Price        decimal.Decimal `json:"price"`
	Qty          decimal.Decimal `json:"qty"`
	FilledQty    decimal.Decimal `json:"filled_qty"`
	SlippageBps  int             `json:"slippage_bps"`
	FeeUSD       decimal.Decimal `json:"fee_usd"`
	Status       string          `json:"status"`
	ChainID      int             `json:"chain_id"`
	TxHash       string          `json:"tx_hash"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

func (db *DB) CreateOrder(ctx context.Context, order *Order) error {
	order.ID = uuid.New()
	order.OrderHash = generateOrderHash(order.UserID.String(), order.PairID.String(), order.Side, order.Price.String())
	order.CreatedAt = time.Now()
	order.UpdatedAt = time.Now()
	order.Status = "pending"
	order.FilledQty = decimal.Zero

	sql := `
		INSERT INTO orders (id, order_hash, user_id, pair_id, side, order_type, price, qty, filled_qty, slippage_bps, fee_usd, status, chain_id, tx_hash, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
	`
	_, err := db.pool.Exec(ctx, sql,
		order.ID, order.OrderHash, order.UserID, order.PairID, order.Side, order.OrderType,
		order.Price.String(), order.Qty.String(), order.FilledQty.String(), order.SlippageBps,
		order.FeeUSD.String(), order.Status, order.ChainID, order.TxHash, order.CreatedAt, order.UpdatedAt,
	)
	return err
}

func (db *DB) GetOrderByHash(ctx context.Context, orderHash string) (*Order, error) {
	order := &Order{}
	sql := `SELECT id, order_hash, user_id, pair_id, side, order_type, price, qty, filled_qty, slippage_bps, fee_usd, status, chain_id, tx_hash, created_at, updated_at FROM orders WHERE order_hash = $1`
	
	row := db.pool.QueryRow(ctx, sql, orderHash)
	var priceStr, qtyStr, filledStr, feeStr string
	err := row.Scan(&order.ID, &order.OrderHash, &order.UserID, &order.PairID, &order.Side, &order.OrderType, &priceStr, &qtyStr, &filledStr, &order.SlippageBps, &feeStr, &order.Status, &order.ChainID, &order.TxHash, &order.CreatedAt, &order.UpdatedAt)
	if err != nil {
		return nil, err
	}
	order.Price, _ = decimal.NewFromString(priceStr)
	order.Qty, _ = decimal.NewFromString(qtyStr)
	order.FilledQty, _ = decimal.NewFromString(filledStr)
	order.FeeUSD, _ = decimal.NewFromString(feeStr)
	return order, nil
}

func (db *DB) UpdateOrderFilled(ctx context.Context, orderID uuid.UUID, filledQty, avgPrice decimal.Decimal, status string) error {
	sql := `UPDATE orders SET filled_qty = $2, price = $3, status = $4, updated_at = NOW() WHERE id = $1`
	_, err := db.pool.Exec(ctx, sql, orderID, filledQty.String(), avgPrice.String(), status)
	return err
}

func (db *DB) GetUserOrders(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*Order, int, error) {
	var total int
	db.pool.QueryRow(ctx, "SELECT COUNT(*) FROM orders WHERE user_id = $1", userID).Scan(&total)

	sql := `SELECT id, order_hash, user_id, pair_id, side, order_type, price, qty, filled_qty, slippage_bps, fee_usd, status, chain_id, tx_hash, created_at, updated_at FROM orders WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`
	rows, err := db.pool.Query(ctx, sql, userID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var orders []*Order
	for rows.Next() {
		o := &Order{}
		var priceStr, qtyStr, filledStr, feeStr string
		rows.Scan(&o.ID, &o.OrderHash, &o.UserID, &o.PairID, &o.Side, &o.OrderType, &priceStr, &qtyStr, &filledStr, &o.SlippageBps, &feeStr, &o.Status, &o.ChainID, &o.TxHash, &o.CreatedAt, &o.UpdatedAt)
		o.Price, _ = decimal.NewFromString(priceStr)
		o.Qty, _ = decimal.NewFromString(qtyStr)
		o.FilledQty, _ = decimal.NewFromString(filledStr)
		o.FeeUSD, _ = decimal.NewFromString(feeStr)
		orders = append(orders, o)
	}
	return orders, total, rows.Err()
}

// ============================================================================
// TRADE Operations
// ============================================================================

type Trade struct {
	ID         uuid.UUID       `json:"id"`
	OrderID    uuid.UUID       `json:"order_id"`
	PairID     uuid.UUID       `json:"pair_id"`
	UserID     uuid.UUID       `json:"user_id"`
	Side       string          `json:"side"`
	Price      decimal.Decimal `json:"price"`
	Qty        decimal.Decimal `json:"qty"`
	FeeUSD     decimal.Decimal `json:"fee_usd"`
	TxHash     string          `json:"tx_hash"`
	BlockNumber int64          `json:"block_number"`
	Timestamp  time.Time       `json:"timestamp"`
	Dex        string          `json:"dex"`
	CreatedAt  time.Time       `json:"created_at"`
}

func (db *DB) CreateTrade(ctx context.Context, trade *Trade) error {
	trade.ID = uuid.New()
	trade.CreatedAt = time.Now()

	sql := `
		INSERT INTO trades (id, order_id, pair_id, user_id, side, price, qty, fee_usd, tx_hash, block_number, timestamp, dex, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
	`
	_, err := db.pool.Exec(ctx, sql,
		trade.ID, trade.OrderID, trade.PairID, trade.UserID, trade.Side, trade.Price.String(),
		trade.Qty.String(), trade.FeeUSD.String(), trade.TxHash, trade.BlockNumber, trade.Timestamp, trade.Dex, trade.CreatedAt,
	)
	return err
}

func (db *DB) GetTradesByPair(ctx context.Context, pairID uuid.UUID, limit, offset int) ([]*Trade, int, error) {
	var total int
	db.pool.QueryRow(ctx, "SELECT COUNT(*) FROM trades WHERE pair_id = $1", pairID).Scan(&total)

	sql := `SELECT id, order_id, pair_id, user_id, side, price, qty, fee_usd, tx_hash, block_number, timestamp, dex, created_at FROM trades WHERE pair_id = $1 ORDER BY timestamp DESC LIMIT $2 OFFSET $3`
	rows, err := db.pool.Query(ctx, sql, pairID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var trades []*Trade
	for rows.Next() {
		t := &Trade{}
		var priceStr, qtyStr, feeStr string
		rows.Scan(&t.ID, &t.OrderID, &t.PairID, &t.UserID, &t.Side, &priceStr, &qtyStr, &feeStr, &t.TxHash, &t.BlockNumber, &t.Timestamp, &t.Dex, &t.CreatedAt)
		t.Price, _ = decimal.NewFromString(priceStr)
		t.Qty, _ = decimal.NewFromString(qtyStr)
		t.FeeUSD, _ = decimal.NewFromString(feeStr)
		trades = append(trades, t)
	}
	return trades, total, rows.Err()
}

// ============================================================================
// BOT Operations
// ============================================================================

type BotInstance struct {
	ID               uuid.UUID       `json:"id"`
	UserID           uuid.UUID       `json:"user_id"`
	BotType          string          `json:"bot_type"`
	Name             string          `json:"name"`
	Status           string          `json:"status"`
	ConnectedDexes   string          `json:"connected_dexes"`
	ConnectedCexes   string          `json:"connected_cexes"`
	MonthlyFeeUSD    decimal.Decimal `json:"monthly_fee_usd"`
	PerExchangeFee   decimal.Decimal `json:"per_exchange_fee_usd"`
	TotalPnL         decimal.Decimal `json:"total_pnl"`
	TotalVolume      decimal.Decimal `json:"total_volume"`
	TotalOrders      int             `json:"total_orders"`
	AvgLatencyUs     int             `json:"avg_latency_us"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
}

func (db *DB) CreateBot(ctx context.Context, bot *BotInstance) error {
	bot.ID = uuid.New()
	bot.CreatedAt = time.Now()
	bot.UpdatedAt = time.Now()
	bot.Status = "stopped"
	bot.TotalPnL = decimal.Zero
	bot.TotalVolume = decimal.Zero
	bot.TotalOrders = 0
	bot.AvgLatencyUs = 0

	dexesJSON, _ := json.Marshal(bot.ConnectedDexes)
	cexesJSON, _ := json.Marshal(bot.ConnectedCexes)

	sql := `
		INSERT INTO bot_instances (id, user_id, bot_type, name, status, connected_dexes, connected_cexes, monthly_fee_usd, per_exchange_fee_usd, total_pnl, total_volume, total_orders, avg_latency_us, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
	`
	_, err := db.pool.Exec(ctx, sql,
		bot.ID, bot.UserID, bot.BotType, bot.Name, bot.Status, string(dexesJSON), string(cexesJSON),
		bot.MonthlyFeeUSD.String(), bot.PerExchangeFee.String(), bot.TotalPnL.String(), bot.TotalVolume.String(),
		bot.TotalOrders, bot.AvgLatencyUs, bot.CreatedAt, bot.UpdatedAt,
	)
	return err
}

func (db *DB) GetUserBots(ctx context.Context, userID uuid.UUID) ([]*BotInstance, error) {
	sql := `SELECT id, user_id, bot_type, name, status, connected_dexes, connected_cexes, monthly_fee_usd, per_exchange_fee_usd, total_pnl, total_volume, total_orders, avg_latency_us, created_at, updated_at FROM bot_instances WHERE user_id = $1`
	rows, err := db.pool.Query(ctx, sql, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bots []*BotInstance
	for rows.Next() {
		b := &BotInstance{}
		var dexesJSON, cexesJSON string
		var monthlyFee, perExFee, totalPnL, totalVol string
		rows.Scan(&b.ID, &b.UserID, &b.BotType, &b.Name, &b.Status, &dexesJSON, &cexesJSON, &monthlyFee, &perExFee, &totalPnL, &totalVol, &b.TotalOrders, &b.AvgLatencyUs, &b.CreatedAt, &b.UpdatedAt)
		b.ConnectedDexes = dexesJSON
		b.ConnectedCexes = cexesJSON
		b.MonthlyFeeUSD, _ = decimal.NewFromString(monthlyFee)
		b.PerExchangeFee, _ = decimal.NewFromString(perExFee)
		b.TotalPnL, _ = decimal.NewFromString(totalPnL)
		b.TotalVolume, _ = decimal.NewFromString(totalVol)
		bots = append(bots, b)
	}
	return bots, rows.Err()
}

func (db *DB) UpdateBotStats(ctx context.Context, botID uuid.UUID, pnlDelta, volumeDelta decimal.Decimal, latencyUs int) error {
	sql := `UPDATE bot_instances SET total_pnl = total_pnl + $2, total_volume = total_volume + $3, total_orders = total_orders + 1, avg_latency_us = $4, updated_at = NOW() WHERE id = $1`
	_, err := db.pool.Exec(ctx, sql, botID, pnlDelta.String(), volumeDelta.String(), latencyUs)
	return err
}

func (db *DB) UpdateBotStatus(ctx context.Context, botID uuid.UUID, status string) error {
	sql := `UPDATE bot_instances SET status = $2, updated_at = NOW() WHERE id = $1`
	_, err := db.pool.Exec(ctx, sql, botID, status)
	return err
}

// ============================================================================
// ACTIVITY LOG
// ============================================================================

type ActivityLog struct {
	ID         uuid.UUID  `json:"id"`
	UserID     uuid.UUID  `json:"user_id"`
	Action     string     `json:"action"`
	EntityType string     `json:"entity_type"`
	EntityID   *uuid.UUID `json:"entity_id"`
	Details    string     `json:"details"`
	IPAddress  string     `json:"ip_address"`
	UserAgent  string     `json:"user_agent"`
	CreatedAt  time.Time  `json:"created_at"`
}

func (db *DB) LogActivity(ctx context.Context, log *ActivityLog) error {
	log.ID = uuid.New()
	log.CreatedAt = time.Now()

	sql := `INSERT INTO user_activity_log (id, user_id, action, entity_type, entity_id, details, ip_address, user_agent, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`
	_, err := db.pool.Exec(ctx, sql, log.ID, log.UserID, log.Action, log.EntityType, log.EntityID, log.Details, log.IPAddress, log.UserAgent, log.CreatedAt)
	return err
}

// ============================================================================
// HELPERS
// ============================================================================

func generateOrderHash(userID, pairID, side, price string) string {
	data := fmt.Sprintf("%s-%s-%s-%s-%d", userID, pairID, side, price, time.Now().UnixNano())
	hash := sha256.Sum256([]byte(data))
	return "0x" + hex.EncodeToString(hash[:])
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// ============================================================================
// ENCRYPTION
// ============================================================================

func EncryptAPIKey(apiKey, encryptionKey string) (string, error) {
	block, err := aes.NewCipher([]byte(sha256256(encryptionKey)))
	if err != nil {
		return "", err
	}

	ciphertext := make([]byte, aes.BlockSize+len(apiKey))
	iv := ciphertext[:aes.BlockSize]
	if _, err := rand.Read(iv); err != nil {
		return "", err
	}

	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], []byte(apiKey))

	return hex.EncodeToString(ciphertext), nil
}

func DecryptAPIKey(encryptedKey, encryptionKey string) (string, error) {
	data, err := hex.DecodeString(encryptedKey)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher([]byte(sha256256(encryptionKey)))
	if err != nil {
		return "", err
	}

	if len(data) < aes.BlockSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	iv := data[:aes.BlockSize]
	data = data[aes.BlockSize:]

	stream := cipher.NewCFBDecrypter(block, iv)
	stream.XORKeyStream(data, data)

	return string(data), nil
}

func sha256256(input string) string {
	hash := sha256.Sum256([]byte(input))
	return string(hash[:])
}