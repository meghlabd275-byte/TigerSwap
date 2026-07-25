// Package config provides configuration for the API Gateway
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config represents the API Gateway configuration
type Config struct {
	// Server settings
	Host         string
	Port         int
	Mode         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration

	// Rate limiting
	RateLimit RateLimitConfig

	// Redis configuration
	Redis RedisConfig

	// CORS configuration
	CORS CORSConfig

	// TLS configuration
	TLS TLSConfig

	// Upstream services
	Upstream UpstreamConfig

	// Request settings
	RequestTimeout time.Duration

	// JWT settings
	JWT JWTConfig
}

// RateLimitConfig represents rate limiting configuration
type RateLimitConfig struct {
	RequestsPerSecond int
	Burst            int
	Enabled          bool
}

// RedisConfig represents Redis configuration
type RedisConfig struct {
	Host         string
	Port         int
	Password     string
	DB           int
	PoolSize     int
	MinIdleConns int
}

// CORSConfig represents CORS configuration
type CORSConfig struct {
	AllowOrigins     []string
	AllowMethods     []string
	AllowHeaders     []string
	ExposeHeaders    []string
	AllowCredentials bool
	MaxAge           time.Duration
}

// TLSConfig represents TLS configuration
type TLSConfig struct {
	Enabled  bool
	CertFile string
	KeyFile  string
}

// UpstreamConfig represents upstream service configuration
type UpstreamConfig struct {
	DEXAggregator string
	Blockchain    string
	PriceOracle  string
	Wallet       string
}

// JWTConfig represents JWT configuration
type JWTConfig struct {
	Secret          string
	ExpirationHours int
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	cfg := &Config{
		Host:         getEnv("API_HOST", "0.0.0.0"),
		Port:         getEnvAsInt("API_PORT", 8080),
		Mode:         getEnv("GIN_MODE", "debug"),
		ReadTimeout:  getEnvAsDuration("API_READ_TIMEOUT", 30*time.Second),
		WriteTimeout: getEnvAsDuration("API_WRITE_TIMEOUT", 30*time.Second),
		IdleTimeout:  getEnvAsDuration("API_IDLE_TIMEOUT", 60*time.Second),
	}

	// Rate limiting
	cfg.RateLimit = RateLimitConfig{
		RequestsPerSecond: getEnvAsInt("RATE_LIMIT_RPS", 1000),
		Burst:             getEnvAsInt("RATE_LIMIT_BURST", 2000),
		Enabled:           getEnvAsBool("RATE_LIMIT_ENABLED", true),
	}

	// Redis
	cfg.Redis = RedisConfig{
		Host:         getEnv("REDIS_HOST", "localhost"),
		Port:         getEnvAsInt("REDIS_PORT", 6379),
		Password:     getEnv("REDIS_PASSWORD", ""),
		DB:           getEnvAsInt("REDIS_DB", 0),
		PoolSize:     getEnvAsInt("REDIS_POOL_SIZE", 100),
		MinIdleConns: getEnvAsInt("REDIS_MIN_IDLE_CONNS", 10),
	}

	// CORS
	cfg.CORS = CORSConfig{
		AllowOrigins:     strings.Split(getEnv("CORS_ALLOW_ORIGINS", "*"), ","),
		AllowMethods:     strings.Split(getEnv("CORS_ALLOW_METHODS", "GET,POST,PUT,DELETE,OPTIONS"), ","),
		AllowHeaders:     strings.Split(getEnv("CORS_ALLOW_HEADERS", "Origin,Content-Type,Accept,Authorization,X-Request-ID"), ","),
		ExposeHeaders:    strings.Split(getEnv("CORS_EXPOSE_HEADERS", "X-Request-ID,X-RateLimit-Remaining,X-RateLimit-Reset"), ","),
		AllowCredentials: getEnvAsBool("CORS_ALLOW_CREDENTIALS", true),
		MaxAge:           getEnvAsDuration("CORS_MAX_AGE", 12*time.Hour),
	}

	// TLS
	cfg.TLS = TLSConfig{
		Enabled:  getEnvAsBool("TLS_ENABLED", false),
		CertFile: getEnv("TLS_CERT_FILE", ""),
		KeyFile:  getEnv("TLS_KEY_FILE", ""),
	}

	// Upstream services
	cfg.Upstream = UpstreamConfig{
		DEXAggregator: getEnv("UPSTREAM_DEX_AGGREGATOR", "http://localhost:8081"),
		Blockchain:    getEnv("UPSTREAM_BLOCKCHAIN", "http://localhost:8082"),
		PriceOracle:  getEnv("UPSTREAM_PRICE_ORACLE", "http://localhost:8083"),
		Wallet:       getEnv("UPSTREAM_WALLET", "http://localhost:8084"),
	}

	// Request timeout
	cfg.RequestTimeout = getEnvAsDuration("REQUEST_TIMEOUT", 30*time.Second)

	// JWT
	cfg.JWT = JWTConfig{
		Secret:          getEnv("JWT_SECRET", "tigerswap-secret-key-change-in-production"),
		ExpirationHours: getEnvAsInt("JWT_EXPIRATION_HOURS", 24),
	}

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value, exists := os.LookupEnv(key); exists {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvAsBool(key string, defaultValue bool) bool {
	if value, exists := os.LookupEnv(key); exists {
		return strings.ToLower(value) == "true" || value == "1"
	}
	return defaultValue
}

func getEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	if value, exists := os.LookupEnv(key); exists {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

// GetAddress returns the server address
func (c *Config) GetAddress() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}
