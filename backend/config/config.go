package config

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	DB    *gorm.DB
	Redis *redis.Client
	ctx   = context.Background()
)

func InitDB() (*gorm.DB, error) {
	// All database credentials must be provided via environment variables
	// No defaults for security - fail if not configured
	host := os.Getenv("DB_HOST")
	port := os.Getenv("DB_PORT")
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	dbname := os.Getenv("DB_NAME")

	// Validate required environment variables
	if host == "" {
		return nil, fmt.Errorf("DB_HOST environment variable is required")
	}
	if user == "" {
		return nil, fmt.Errorf("DB_USER environment variable is required")
	}
	if password == "" {
		return nil, fmt.Errorf("DB_PASSWORD environment variable is required")
	}
	if dbname == "" {
		return nil, fmt.Errorf("DB_NAME environment variable is required")
	}
	if port == "" {
		port = "5432" // Default only if explicitly empty
	}

	// TLS to Postgres is required by default. A single-node deployment whose
	// database is only reachable over a private network may set DB_SSLMODE to a
	// weaker value, but the default keeps a misconfigured deployment from
	// silently sending credentials in the clear.
	sslmode := os.Getenv("DB_SSLMODE")
	if sslmode == "" {
		sslmode = "require"
	}

	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s TimeZone=UTC",
		host, user, password, dbname, port, sslmode,
	)

	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})

	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying SQL database: %w", err)
	}

	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	log.Println("Database connection established successfully")
	return DB, nil
}

func InitRedis() *redis.Client {
	redisHost := getEnv("REDIS_HOST", "localhost")
	redisPort := getEnv("REDIS_PORT", "6379")
	redisPassword := getEnv("REDIS_PASSWORD", "")
	redisDB := 0

	Redis = redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%s", redisHost, redisPort),
		Password:     redisPassword,
		DB:           redisDB,
		PoolSize:     100,
		MinIdleConns: 10,
	})

	_, err := Redis.Ping(ctx).Result()
	if err != nil {
		log.Printf("Warning: Failed to connect to Redis: %v", err)
	} else {
		log.Println("Redis connection established successfully")
	}

	return Redis
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

// Secret returns the value of an environment variable, falling back to a file
// named by the variable's `_FILE` sibling. A deployment that mounts secrets
// (Docker secrets, Kubernetes mounted Secret volumes) exposes the value as a
// file rather than an environment variable, so a key set only as `KEY_FILE`
// would otherwise be invisible. The environment variable wins when both are
// set, so a local `.env` keeps working.
//
// The file contents are trimmed of a single trailing newline, because a mounted
// secret commonly ends in one and a signer key with a stray newline would fail
// to parse. A read error or an empty value is returned as the empty string; the
// caller reports the missing key, exactly as it does for an unset variable.
func Secret(key string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	path := os.Getenv(key + "_FILE")
	if path == "" {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		log.Printf("warning: cannot read secret file for %s: %v", key, err)
		return ""
	}
	return strings.TrimRight(string(data), "\r\n")
}
