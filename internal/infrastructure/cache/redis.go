package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/infrastructure/config"
)

// redisCache implements the Cache interface using Redis
type redisCache struct {
	client *redis.Client
	logger *zap.Logger
}

// NewRedisCache creates a new Redis cache instance with the given configuration
func NewRedisCache(cfg *config.RedisConfig, logger *zap.Logger) (Cache, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger is required")
	}

	if cfg == nil {
		return nil, fmt.Errorf("redis config is required")
	}

	opts := &redis.Options{
		Addr:         cfg.URL,
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
		MaxRetries:   cfg.MaxRetries,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	}

	client := redis.NewClient(opts)

	// Health check with timeout
	ctx, cancel := context.WithTimeout(context.Background(), cfg.DialTimeout)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis connection failed: %w", err)
	}

	logger.Info("redis cache initialized",
		zap.String("addr", cfg.URL),
		zap.Int("db", cfg.DB),
		zap.Int("pool_size", cfg.PoolSize))

	return &redisCache{
		client: client,
		logger: logger,
	}, nil
}

// Get retrieves a value by key
func (r *redisCache) Get(ctx context.Context, key string) (string, error) {
	result, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return "", ErrCacheKeyNotFound{Key: key}
		}
		r.logger.Error("redis get failed", zap.String("key", key), zap.Error(err))
		return "", fmt.Errorf("redis get failed: %w", err)
	}

	return result, nil
}

// Set stores a value with optional TTL
func (r *redisCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	err := r.client.Set(ctx, key, value, ttl).Err()
	if err != nil {
		r.logger.Error("redis set failed",
			zap.String("key", key),
			zap.Duration("ttl", ttl),
			zap.Error(err))
		return fmt.Errorf("redis set failed: %w", err)
	}

	return nil
}

// Delete removes a key
func (r *redisCache) Delete(ctx context.Context, key string) error {
	err := r.client.Del(ctx, key).Err()
	if err != nil {
		r.logger.Error("redis delete failed", zap.String("key", key), zap.Error(err))
		return fmt.Errorf("redis delete failed: %w", err)
	}

	return nil
}

// Exists checks if a key exists
func (r *redisCache) Exists(ctx context.Context, key string) (bool, error) {
	result, err := r.client.Exists(ctx, key).Result()
	if err != nil {
		r.logger.Error("redis exists check failed", zap.String("key", key), zap.Error(err))
		return false, fmt.Errorf("redis exists check failed: %w", err)
	}

	return result > 0, nil
}

// SetNX sets a value only if the key doesn't exist (atomic)
func (r *redisCache) SetNX(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error) {
	result, err := r.client.SetNX(ctx, key, value, ttl).Result()
	if err != nil {
		r.logger.Error("redis setnx failed",
			zap.String("key", key),
			zap.Duration("ttl", ttl),
			zap.Error(err))
		return false, fmt.Errorf("redis setnx failed: %w", err)
	}

	return result, nil
}

// Increment atomically increments a numeric value
func (r *redisCache) Increment(ctx context.Context, key string) (int64, error) {
	result, err := r.client.Incr(ctx, key).Result()
	if err != nil {
		r.logger.Error("redis increment failed", zap.String("key", key), zap.Error(err))
		return 0, fmt.Errorf("redis increment failed: %w", err)
	}

	return result, nil
}

// Expire sets TTL on an existing key
func (r *redisCache) Expire(ctx context.Context, key string, ttl time.Duration) error {
	result, err := r.client.Expire(ctx, key, ttl).Result()
	if err != nil {
		r.logger.Error("redis expire failed",
			zap.String("key", key),
			zap.Duration("ttl", ttl),
			zap.Error(err))
		return fmt.Errorf("redis expire failed: %w", err)
	}
	
	// Redis Expire returns false if key doesn't exist
	if !result {
		return ErrCacheKeyNotFound{Key: key}
	}

	return nil
}

// GetJSON retrieves and unmarshals JSON data
func (r *redisCache) GetJSON(ctx context.Context, key string, dest interface{}) error {
	data, err := r.Get(ctx, key)
	if err != nil {
		return err
	}

	if err := json.Unmarshal([]byte(data), dest); err != nil {
		r.logger.Error("json unmarshal failed",
			zap.String("key", key),
			zap.Error(err))
		return fmt.Errorf("json unmarshal failed: %w", err)
	}

	return nil
}

// SetJSON marshals and stores JSON data
func (r *redisCache) SetJSON(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		r.logger.Error("json marshal failed",
			zap.String("key", key),
			zap.Error(err))
		return fmt.Errorf("json marshal failed: %w", err)
	}

	return r.Set(ctx, key, data, ttl)
}

// Close closes the cache connection
func (r *redisCache) Close() error {
	if err := r.client.Close(); err != nil {
		r.logger.Error("redis close failed", zap.Error(err))
		return fmt.Errorf("redis close failed: %w", err)
	}

	r.logger.Info("redis cache connection closed")
	return nil
}