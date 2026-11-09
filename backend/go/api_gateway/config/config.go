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

	// TrustedProxies is the list of CIDRs or IPs allowed to set the client IP
	// through a forwarding header. Empty means trust none.
	TrustedProxies []string

	// JWT settings
	JWT JWTConfig
}

// RateLimitConfig represents rate limiting configuration
type RateLimitConfig struct {
	RequestsPerSecond int
	Burst             int
	Enabled           bool
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
	PriceOracle   string
	Wallet        string
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

	// Trusted proxies: only a configured proxy may set the client IP. An
	// unset or wildcard value leaves the set empty, which trusts no peer and
	// stops a client rotating X-Forwarded-For to bypass the rate limiter.
	cfg.TrustedProxies = parseTrustedProxies(getEnv("TRUSTED_PROXIES", ""))

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
		AllowOrigins:     sanitizeCORSOrigins(strings.Split(getEnv("CORS_ALLOW_ORIGINS", "*"), ","), getEnvAsBool("CORS_ALLOW_CREDENTIALS", true)),
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
		PriceOracle:   getEnv("UPSTREAM_PRICE_ORACLE", "http://localhost:8083"),
		Wallet:        getEnv("UPSTREAM_WALLET", "http://localhost:8084"),
	}

	// Request timeout
	cfg.RequestTimeout = getEnvAsDuration("REQUEST_TIMEOUT", 30*time.Second)

	// JWT. There is deliberately no default secret: a well-known fallback would
	// let anyone mint a valid token for any user, so an unset JWT_SECRET must
	// fail closed at startup rather than run insecurely.
	secret := os.Getenv("JWT_SECRET")
	if len(secret) < minJWTSecretLength {
		return nil, fmt.Errorf(
			"JWT_SECRET must be set to at least %d characters", minJWTSecretLength,
		)
	}
	cfg.JWT = JWTConfig{
		Secret:          secret,
		ExpirationHours: getEnvAsInt("JWT_EXPIRATION_HOURS", 24),
	}

	return cfg, nil
}

// minJWTSecretLength is the shortest HS256 secret accepted at startup.
const minJWTSecretLength = 32

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

// parseTrustedProxies turns a comma-separated TRUSTED_PROXIES list into the set
// of peers whose forwarding header is honoured. A wildcard is ignored: trusting
// every peer is the default this exists to replace.
func parseTrustedProxies(raw string) []string {
	out := []string{}
	for _, part := range strings.Split(raw, ",") {
		entry := strings.TrimSpace(part)
		if entry == "" || entry == "*" || entry == "0.0.0.0/0" || entry == "::/0" {
			continue
		}
		out = append(out, entry)
	}
	return out
}

// sanitizeCORSOrigins makes the origin list safe to combine with credentials.
// gin-contrib/cors answers a wildcard with `Access-Control-Allow-Origin: *`; a
// browser rejects that together with `Access-Control-Allow-Credentials: true`,
// and echoing credentials to any origin would let an arbitrary site drive the
// authenticated API. So when credentials are enabled the wildcard is dropped and
// an explicit list (localhost by default) is used instead.
func sanitizeCORSOrigins(origins []string, allowCredentials bool) []string {
	cleaned := make([]string, 0, len(origins))
	for _, origin := range origins {
		origin = strings.TrimSpace(origin)
		if origin == "" {
			continue
		}
		if allowCredentials && origin == "*" {
			continue
		}
		cleaned = append(cleaned, origin)
	}
	if len(cleaned) == 0 {
		if allowCredentials {
			return []string{"http://localhost:3000"}
		}
		return []string{"*"}
	}
	return cleaned
}
