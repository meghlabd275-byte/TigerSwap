package config

import (
	"os"
	"strconv"
)

type Config struct {
	// Server
	Port string

	// Database
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string

	// Redis
	RedisAddr     string
	RedisPassword string

	// JWT
	JWTSecret string

	// Stripe
	StripeAPIKey string
	StripeWHSecret string

	// Coinbase Commerce
	CoinbaseAPIKey string
	CoinbaseWHSecret string

	// MoonPay
	MoonPayAPIKey string
	MoonPaySecretKey string

	// Transak
	TransakAPIKey string

	// Fee configuration
	ServiceFeePercent float64
}

func Load() *Config {
	return &Config{
		Port:            getEnv("PORT", "8080"),
		DBHost:          getEnv("DB_HOST", "localhost"),
		DBPort:          getEnv("DB_PORT", "5432"),
		DBUser:          getEnv("DB_USER", "tigerwallet"),
		DBPassword:      getEnv("DB_PASSWORD", "tigerwallet"),
		DBName:          getEnv("DB_NAME", "tigerwallet_fiat"),
		RedisAddr:       getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:   getEnv("REDIS_PASSWORD", ""),
		JWTSecret:       getEnv("JWT_SECRET", "tigerwallet-secret-key"),
		StripeAPIKey:    getEnv("STRIPE_API_KEY", ""),
		StripeWHSecret:  getEnv("STRIPE_WH_SECRET", ""),
		CoinbaseAPIKey:  getEnv("COINBASE_API_KEY", ""),
		CoinbaseWHSecret: getEnv("COINBASE_WH_SECRET", ""),
		MoonPayAPIKey:   getEnv("MOONPAY_API_KEY", ""),
		MoonPaySecretKey: getEnv("MOONPAY_SECRET_KEY", ""),
		TransakAPIKey:   getEnv("TRANSAK_API_KEY", ""),
		ServiceFeePercent: getEnvFloat("SERVICE_FEE_PERCENT", 0.5),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvFloat(key string, defaultValue float64) float64 {
	if value := os.Getenv(key); value != "" {
		if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
			return floatValue
		}
	}
	return defaultValue
}
