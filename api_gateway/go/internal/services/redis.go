package services

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
)

// NewRedisClient creates a new Redis client
func NewRedisClient() *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", viper.GetString("redis.host"), viper.GetInt("redis.port")),
		Password: viper.GetString("redis.password"),
		DB:       viper.GetInt("redis.db"),
	})

	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		fmt.Printf("Warning: Redis connection failed: %v\n", err)
	}

	return client
}

// RedisService provides Redis operations
type RedisService struct {
	client *redis.Client
}

// NewRedisService creates a new Redis service
func NewRedisService(client *redis.Client) *RedisService {
	return &RedisService{client: client}
}

// CachePrice caches a price
func (s *RedisService) CachePrice(ctx context.Context, symbol string, price float64) error {
	return s.client.Set(ctx, fmt.Sprintf("price:%s", symbol), price, 0).Err()
}

// GetPrice gets a cached price
func (s *RedisService) GetPrice(ctx context.Context, symbol string) (float64, error) {
	result, err := s.client.Get(ctx, fmt.Sprintf("price:%s", symbol)).Float64()
	if err != nil {
		return 0, err
	}
	return result, nil
}

// SetRateLimit sets rate limit for a key
func (s *RedisService) SetRateLimit(ctx context.Context, key string, limit int, window int) error {
	return s.client.Set(ctx, fmt.Sprintf("ratelimit:%s", key), limit, 0).Err()
}

// GetRateLimit gets rate limit for a key
func (s *RedisService) GetRateLimit(ctx context.Context, key string) (int, error) {
	result, err := s.client.Get(ctx, fmt.Sprintf("ratelimit:%s", key)).Int()
	if err != nil {
		return 0, err
	}
	return result, nil
}
