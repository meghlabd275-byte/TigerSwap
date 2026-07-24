package config

import (
	"context"
	"fmt"
	"log"
	"os"
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

	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=require TimeZone=UTC",
		host, user, password, dbname, port,
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
