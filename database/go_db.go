-- TigerSwap Database Connection and ORM for Go Backend
-- Database connection pooling and utilities

package database

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Config struct {
	Host     string
	Port     string
	User     string
	Password string
	Database string
	MaxConns int
}

func NewConfig() *Config {
	return &Config{
		Host:     getEnv("DB_HOST", "localhost"),
		Port:     getEnv("DB_PORT", "5432"),
		User:     getEnv("DB_USER", "tigerswap"),
		Password: getEnv("DB_PASSWORD", ""),
		Database: getEnv("DB_NAME", "tigerswap"),
		MaxConns: 100,
	}
}

func (c *Config) ConnectionString() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=require&pool_max_conns=%d",
		c.User, c.Password, c.Host, c.Port, c.Database, c.MaxConns,
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

// ============================================================================
// Query Helpers
// ============================================================================

type QueryOptions struct {
	Limit  int
	Offset int
	SortBy string
	Order  string
}

func (db *DB) Query(ctx context.Context, sql string, args ...interface{}) (*pgxpool.Row, error) {
	return nil, nil
}

func (db *DB) QueryAll(ctx context.Context, sql string, args ...interface{}) (*pgxpool.Rows, error) {
	return db.pool.Query(ctx, sql, args...)
}

func (db *DB) Exec(ctx context.Context, sql string, args ...interface{}) error {
	_, err := db.pool.Exec(ctx, sql, args...)
	return err
}

// ============================================================================
// User Operations
// ============================================================================

type User struct {
	ID            string    `json:"id"`
	WalletAddress string    `json:"wallet_address"`
	Email         string    `json:"email"`
	Username      string    `json:"username"`
	RiskScore     int       `json:"risk_score"`
	KYCStatus     string    `json:"kyc_status"`
	IsVerified    bool      `json:"is_verified"`
	IsAdmin       bool      `json:"is_admin"`
	TotalVolume   float64   `json:"total_volume_usd"`
	TotalPnL      float64   `json:"total_pnl"`
	CreatedAt     time.Time `json:"created_at"`
}

func (db *DB) CreateUser(ctx context.Context, user *User) error {
	sql := `
		INSERT INTO users (wallet_address, email, username, kyc_status, is_verified)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at
	`
	return db.pool.QueryRow(ctx, sql,
		user.WalletAddress, user.Email, user.Username, user.KYCStatus, user.IsVerified,
	).Scan(&user.ID, &user.CreatedAt)
}

func (db *DB) GetUserByWallet(ctx context.Context, wallet string) (*User, error) {
	user := &User{}
	sql := `
		SELECT id, wallet_address, email, username, risk_score, kyc_status, 
		       is_verified, is_admin, total_volume_usd, total_pnl, created_at
		FROM users WHERE wallet_address = $1
	`
	err := db.pool.QueryRow(ctx, sql, wallet).Scan(
		&user.ID, &user.WalletAddress, &user.Email, &user.Username,
		&user.RiskScore, &user.KYCStatus, &user.IsVerified, &user.IsAdmin,
		&user.TotalVolume, &user.TotalPnL, &user.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (db *DB) UpdateUserVolume(ctx context.Context, userID string, volumeDelta float64) error {
	sql := `UPDATE users SET total_volume_usd = total_volume_usd + $1 WHERE id = $2`
	_, err := db.pool.Exec(ctx, sql, volumeDelta, userID)
	return err
}

// ============================================================================
// Token & Pair Operations
// ============================================================================

type Token struct {
	ID              string    `json:"id"`
	Symbol          string    `json:"symbol"`
	Name            string    `json:"name"`
	ContractAddress string    `json:"contract_address"`
	ChainID         int       `json:"chain_id"`
	Decimals        int       `json:"decimals"`
	LogoURL         string    `json:"logo_url"`
	IsStablecoin    bool      `json:"is_stablecoin"`
	PriceUSD        float64   `json:"price_usd"`
}

type TradingPair struct {
	ID             string  `json:"id"`
	TokenAID       string  `json:"token_a_id"`
	TokenBID       string  `json:"token_b_id"`
	PairAddress    string  `json:"pair_address"`
	ChainID        int     `json:"chain_id"`
	IsStablePair   bool    `json:"is_stable_pair"`
	IsVerified     bool    `json:"is_verified"`
	MinOrderSize   float64 `json:"min_order_size"`
	FeeTierBps     int     `json:"fee_tier_bps"`
}

func (db *DB) CreateToken(ctx context.Context, token *Token) error {
	sql := `
		INSERT INTO tokens (symbol, name, contract_address, chain_id, decimals, is_stablecoin)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (chain_id, contract_address) DO UPDATE
		SET symbol = EXCLUDED.symbol, name = EXCLUDED.name
		RETURNING id
	`
	return db.pool.QueryRow(ctx, sql,
		token.Symbol, token.Name, token.ContractAddress,
		token.ChainID, token.Decimals, token.IsStablecoin,
	).Scan(&token.ID)
}

func (db *DB) GetTokenPrice(ctx context.Context, tokenID string) (float64, error) {
	var price float64
	sql := `SELECT price_usd FROM tokens WHERE id = $1`
	err := db.pool.QueryRow(ctx, sql, tokenID).Scan(&price)
	return price, err
}

func (db *DB) GetAllPairs(ctx context.Context, chainID int) ([]*TradingPair, error) {
	rows, err := db.pool.Query(ctx, `
		SELECT id, token_a_id, token_b_id, pair_address, chain_id, 
		       is_stable_pair, is_verified, min_order_size, fee_tier_bps
		FROM trading_pairs WHERE chain_id = $1 AND is_verified = true
	`, chainID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pairs []*TradingPair
	for rows.Next() {
		p := &TradingPair{}
		rows.Scan(&p.ID, &p.TokenAID, &p.TokenBID, &p.PairAddress,
			&p.ChainID, &p.IsStablePair, &p.IsVerified, &p.MinOrderSize, &p.FeeTierBps)
		pairs = append(pairs, p)
	}
	return pairs, rows.Err()
}

// ============================================================================
// Pool Operations
// ============================================================================

type Pool struct {
	ID             string  `json:"id"`
	DexID          string  `json:"dex_id"`
	PairID         string  `json:"pair_id"`
	PoolAddress    string  `json:"pool_address"`
	ReserveA       float64 `json:"reserve_a"`
	ReserveB       float64 `json:"reserve_b"`
	LiquidityUSD   float64 `json:"liquidity_usd"`
	FeeTierBps     int     `json:"fee_tier_bps"`
	TVLUSD         float64 `json:"tvl_usd"`
	Volume24hUSD   float64 `json:"volume_24h_usd"`
	APR            float64 `json:"apr"`
	IsActive       bool    `json:"is_active"`
}

func (db *DB) CreatePool(ctx context.Context, pool *Pool) error {
	sql := `
		INSERT INTO pools (dex_id, pair_id, pool_address, fee_tier_bps)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`
	return db.pool.QueryRow(ctx, sql,
		pool.DexID, pool.PairID, pool.PoolAddress, pool.FeeTierBps,
	).Scan(&pool.ID)
}

func (db *DB) UpdatePoolReserves(ctx context.Context, poolAddress string, reserveA, reserveB float64, liquidityUSD float64) error {
	sql := `
		UPDATE pools SET 
			reserve_a = $2, 
			reserve_b = $3, 
			liquidity_usd = $4,
			updated_at = NOW()
		WHERE pool_address = $1
	`
	_, err := db.pool.Exec(ctx, sql, poolAddress, reserveA, reserveB, liquidityUSD)
	return err
}

func (db *DB) GetTopPools(ctx context.Context, limit int) ([]*Pool, error) {
	rows, err := db.pool.Query(ctx, `
		SELECT id, dex_id, pair_id, pool_address, reserve_a, reserve_b,
		       liquidity_usd, fee_tier_bps, tvl_usd, volume_24h_usd, apr
		FROM pools 
		WHERE is_active = true 
		ORDER BY liquidity_usd DESC 
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pools []*Pool
	for rows.Next() {
		p := &Pool{}
		rows.Scan(&p.ID, &p.DexID, &p.PairID, &p.PoolAddress,
			&p.ReserveA, &p.ReserveB, &p.LiquidityUSD, &p.FeeTierBps,
			&p.TVLUSD, &p.Volume24hUSD, &p.APR)
		pools = append(pools, p)
	}
	return pools, rows.Err()
}

// ============================================================================
// Order Operations
// ============================================================================

type Order struct {
	ID          string    `json:"id"`
	OrderHash   string    `json:"order_hash"`
	UserID      string    `json:"user_id"`
	PairID      string    `json:"pair_id"`
	Side        string    `json:"side"`
	OrderType   string    `json:"order_type"`
	Price       float64   `json:"price"`
	Qty         float64   `json:"qty"`
	FilledQty   float64   `json:"filled_qty"`
	AvgFillPrice float64  `json:"avg_fill_price"`
	SlippageBps int       `json:"slippage_bps"`
	FeeUSD      float64   `json:"fee_usd"`
	Status      string    `json:"status"`
	ChainID     int       `json:"chain_id"`
	TxHash      string    `json:"tx_hash"`
	CreatedAt   time.Time `json:"created_at"`
}

func (db *DB) CreateOrder(ctx context.Context, order *Order) error {
	sql := `
		INSERT INTO orders (order_hash, user_id, pair_id, side, order_type, price, qty, slippage_bps, chain_id, tx_hash)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, created_at
	`
	return db.pool.QueryRow(ctx, sql,
		order.OrderHash, order.UserID, order.PairID, order.Side,
		order.OrderType, order.Price, order.Qty, order.SlippageBps,
		order.ChainID, order.TxHash,
	).Scan(&order.ID, &order.CreatedAt)
}

func (db *DB) UpdateOrderFilled(ctx context.Context, orderID string, filledQty, avgPrice float64) error {
	sql := `
		UPDATE orders SET 
			filled_qty = $2,
			avg_fill_price = $3,
			status = CASE WHEN filled_qty >= qty THEN 'filled' ELSE 'partial' END,
			updated_at = NOW()
		WHERE id = $1
	`
	_, err := db.pool.Exec(ctx, sql, orderID, filledQty, avgPrice)
	return err
}

func (db *DB) GetUserOrders(ctx context.Context, userID string, limit, offset int) ([]*Order, error) {
	rows, err := db.pool.Query(ctx, `
		SELECT id, order_hash, user_id, pair_id, side, order_type, price, qty,
		       filled_qty, avg_fill_price, slippage_bps, fee_usd, status, chain_id, created_at
		FROM orders 
		WHERE user_id = $1 
		ORDER BY created_at DESC 
		LIMIT $2 OFFSET $3
	`, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []*Order
	for rows.Next() {
		o := &Order{}
		rows.Scan(&o.ID, &o.OrderHash, &o.UserID, &o.PairID, &o.Side, &o.OrderType,
			&o.Price, &o.Qty, &o.FilledQty, &o.AvgFillPrice, &o.SlippageBps,
			&o.FeeUSD, &o.Status, &o.ChainID, &o.CreatedAt)
		orders = append(orders, o)
	}
	return orders, rows.Err()
}

// ============================================================================
// Bot Operations
// ============================================================================

type BotInstance struct {
	ID              string    `json:"id"`
	UserID          string    `json:"user_id"`
	BotType         string    `json:"bot_type"`
	Name            string    `json:"name"`
	Status          string    `json:"status"`
	MonthlyFeeUSD   float64   `json:"monthly_fee_usd"`
	PerExchangeFee  float64   `json:"per_exchange_fee_usd"`
	TotalPnL        float64   `json:"total_pnl"`
	TotalVolume     float64   `json:"total_volume"`
	TotalOrders     int       `json:"total_orders"`
	AvgLatencyUs    int       `json:"avg_latency_us"`
	CreatedAt       time.Time `json:"created_at"`
}

func (db *DB) CreateBot(ctx context.Context, bot *BotInstance) error {
	sql := `
		INSERT INTO bot_instances (user_id, bot_type, name, status, monthly_fee_usd, per_exchange_fee_usd)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at
	`
	return db.pool.QueryRow(ctx, sql,
		bot.UserID, bot.BotType, bot.Name, bot.Status,
		bot.MonthlyFeeUSD, bot.PerExchangeFee,
	).Scan(&bot.ID, &bot.CreatedAt)
}

func (db *DB) GetBotByID(ctx context.Context, botID string) (*BotInstance, error) {
	bot := &BotInstance{}
	sql := `
		SELECT id, user_id, bot_type, name, status, monthly_fee_usd, 
		       per_exchange_fee_usd, total_pnl, total_volume, total_orders, avg_latency_us, created_at
		FROM bot_instances WHERE id = $1
	`
	err := db.pool.QueryRow(ctx, sql, botID).Scan(
		&bot.ID, &bot.UserID, &bot.BotType, &bot.Name, &bot.Status,
		&bot.MonthlyFeeUSD, &bot.PerExchangeFee, &bot.TotalPnL,
		&bot.TotalVolume, &bot.TotalOrders, &bot.AvgLatencyUs, &bot.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return bot, nil
}

func (db *DB) UpdateBotStats(ctx context.Context, botID string, pnlDelta float64, volumeDelta float64) error {
	sql := `
		UPDATE bot_instances SET 
			total_pnl = total_pnl + $2,
			total_volume = total_volume + $3,
			total_orders = total_orders + 1,
			updated_at = NOW()
		WHERE id = $1
	`
	_, err := db.pool.Exec(ctx, sql, botID, pnlDelta, volumeDelta)
	return err
}

// ============================================================================
// Encryption Helpers
// ============================================================================

func Encrypt(data []byte, key string) ([]byte, error) {
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return nil, err
	}

	ciphertext := make([]byte, aes.BlockSize+len(data))
	iv := ciphertext[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}

	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], data)

	return ciphertext, nil
}

func Decrypt(data []byte, key string) ([]byte, error) {
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return nil, err
	}

	if len(data) < aes.BlockSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	iv := data[:aes.BlockSize]
	data = data[aes.BlockSize:]

	stream := cipher.NewCFBDecrypter(block, iv)
	stream.XORKeyStream(data, data)

	return data, nil
}

func EncryptAPIKey(apiKey, encryptionKey string) (string, error) {
	encrypted, err := Encrypt([]byte(apiKey), encryptionKey)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(encrypted), nil
}

func DecryptAPIKey(encryptedKey, encryptionKey string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(encryptedKey)
	if err != nil {
		return "", err
	}
	decrypted, err := Decrypt(data, encryptionKey)
	if err != nil {
		return "", err
	}
	return string(decrypted), nil
}

// ============================================================================
// Health Check
// ============================================================================

func (db *DB) Ping(ctx context.Context) error {
	return db.pool.Ping(ctx)
}

func (db *DB) Stats(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})
	
	var maxConns, idleConns, usedConns int
	db.pool.Stat().CopyTo(&struct {
		MaxConns  int
		IdleConns int
		UsedConns int
	}{MaxConns: maxConns, IdleConns: idleConns, UsedConns: usedConns})
	
	stats["max_connections"] = maxConns
	stats["idle_connections"] = idleConns
	stats["used_connections"] = usedConns
	
	return stats, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}