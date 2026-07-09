package services

import (
	"fmt"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/spf13/viper"
)

// NewDatabase creates a new database connection
func NewDatabase() *sqlx.DB {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		viper.GetString("database.host"),
		viper.GetInt("database.port"),
		viper.GetString("database.user"),
		viper.GetString("database.password"),
		viper.GetString("database.name"),
	)

	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		fmt.Printf("Warning: Database connection failed: %v\n", err)
		return nil
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)

	return db
}

// DatabaseService provides database operations
type DatabaseService struct {
	db *sqlx.DB
}

// NewDatabaseService creates a new database service
func NewDatabaseService(db *sqlx.DB) *DatabaseService {
	return &DatabaseService{db: db}
}

// InitSchema initializes the database schema
func (s *DatabaseService) InitSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id VARCHAR(64) PRIMARY KEY,
		email VARCHAR(255) UNIQUE NOT NULL,
		username VARCHAR(255) NOT NULL,
		password_hash VARCHAR(255) NOT NULL,
		created_at BIGINT NOT NULL,
		updated_at BIGINT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS wallets (
		id VARCHAR(64) PRIMARY KEY,
		user_id VARCHAR(64) NOT NULL REFERENCES users(id),
		chain_id INTEGER NOT NULL,
		address VARCHAR(255) NOT NULL,
		created_at BIGINT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS orders (
		id VARCHAR(64) PRIMARY KEY,
		user_id VARCHAR(64) NOT NULL REFERENCES users(id),
		market_id VARCHAR(32) NOT NULL,
		side VARCHAR(8) NOT NULL,
		order_type VARCHAR(16) NOT NULL,
		price VARCHAR(64) NOT NULL,
		quantity VARCHAR(64) NOT NULL,
		filled_quantity VARCHAR(64) NOT NULL DEFAULT '0',
		avg_fill_price VARCHAR(64) NOT NULL DEFAULT '0',
		status VARCHAR(16) NOT NULL,
		time_in_force VARCHAR(8) NOT NULL DEFAULT 'gtc',
		created_at BIGINT NOT NULL,
		updated_at BIGINT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS trades (
		id VARCHAR(64) PRIMARY KEY,
		order_id VARCHAR(64) NOT NULL REFERENCES orders(id),
		market_id VARCHAR(32) NOT NULL,
		side VARCHAR(8) NOT NULL,
		price VARCHAR(64) NOT NULL,
		quantity VARCHAR(64) NOT NULL,
		fee VARCHAR(64) NOT NULL,
		timestamp BIGINT NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_orders_user_id ON orders(user_id);
	CREATE INDEX IF NOT EXISTS idx_orders_market_id ON orders(market_id);
	CREATE INDEX IF NOT EXISTS idx_trades_market_id ON trades(market_id);
	`

	_, err := s.db.Exec(schema)
	return err
}

// Close closes the database connection
func (s *DatabaseService) Close() error {
	return s.db.Close()
}
