package cache

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// redisRateLimiter implements the RateLimiter interface using Redis sorted sets
// for sliding window rate limiting
type redisRateLimiter struct {
	client *redis.Client
	logger *zap.Logger
}

// NewRedisRateLimiter creates a new Redis-based rate limiter
func NewRedisRateLimiter(client *redis.Client, logger *zap.Logger) RateLimiter {
	return &redisRateLimiter{
		client: client,
		logger: logger,
	}
}

// Allow checks if a request is allowed under the rate limit using sliding window algorithm
func (r *redisRateLimiter) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error) {
	now := time.Now()
	windowStart := now.Add(-window)
	
	rateLimitKey := RateLimitPrefix + key
	
	// Use Redis pipeline for atomic operations
	pipe := r.client.Pipeline()
	
	// Remove expired entries
	pipe.ZRemRangeByScore(ctx, rateLimitKey, "-inf", strconv.FormatInt(windowStart.UnixNano(), 10))
	
	// Count current entries in window
	countCmd := pipe.ZCard(ctx, rateLimitKey)
	
	// Add current request timestamp
	requestID := fmt.Sprintf("%d-%d", now.UnixNano(), 
		// Add some randomness to avoid collisions
		now.Nanosecond()%1000)
	pipe.ZAdd(ctx, rateLimitKey, redis.Z{
		Score:  float64(now.UnixNano()),
		Member: requestID,
	})
	
	// Set expiration on the key
	pipe.Expire(ctx, rateLimitKey, window+time.Minute)
	
	// Execute pipeline
	_, err := pipe.Exec(ctx)
	if err != nil {
		r.logger.Error("rate limiter pipeline failed",
			zap.String("key", key),
			zap.Int("limit", limit),
			zap.Duration("window", window),
			zap.Error(err))
		return false, fmt.Errorf("rate limiter pipeline failed: %w", err)
	}
	
	// Get the count before adding current request
	currentCount := countCmd.Val()
	
	// Check if limit is exceeded
	allowed := currentCount < int64(limit)
	
	if !allowed {
		// Remove the request we just added since it's not allowed
		r.client.ZRem(ctx, rateLimitKey, requestID)
		
		r.logger.Debug("rate limit exceeded",
			zap.String("key", key),
			zap.Int64("current_count", currentCount),
			zap.Int("limit", limit),
			zap.Duration("window", window))
		
		return false, nil
	}
	
	r.logger.Debug("rate limit check",
		zap.String("key", key),
		zap.Int64("current_count", currentCount+1),
		zap.Int("limit", limit),
		zap.Duration("window", window),
		zap.Bool("allowed", allowed))
	
	return true, nil
}

// Count returns the current count for a rate limit key
func (r *redisRateLimiter) Count(ctx context.Context, key string, window time.Duration) (int, error) {
	now := time.Now()
	windowStart := now.Add(-window)
	
	rateLimitKey := RateLimitPrefix + key
	
	// Clean up expired entries first
	err := r.client.ZRemRangeByScore(ctx, rateLimitKey, "-inf", strconv.FormatInt(windowStart.UnixNano(), 10)).Err()
	if err != nil {
		r.logger.Error("rate limiter cleanup failed",
			zap.String("key", key),
			zap.Error(err))
		return 0, fmt.Errorf("rate limiter cleanup failed: %w", err)
	}
	
	// Count current entries
	count, err := r.client.ZCard(ctx, rateLimitKey).Result()
	if err != nil {
		r.logger.Error("rate limiter count failed",
			zap.String("key", key),
			zap.Error(err))
		return 0, fmt.Errorf("rate limiter count failed: %w", err)
	}
	
	return int(count), nil
}

// Reset clears the rate limit counter for a key
func (r *redisRateLimiter) Reset(ctx context.Context, key string) error {
	rateLimitKey := RateLimitPrefix + key
	
	err := r.client.Del(ctx, rateLimitKey).Err()
	if err != nil {
		r.logger.Error("rate limiter reset failed",
			zap.String("key", key),
			zap.Error(err))
		return fmt.Errorf("rate limiter reset failed: %w", err)
	}
	
	r.logger.Debug("rate limit reset", zap.String("key", key))
	return nil
}

// Remaining returns how many requests are remaining in the current window
func (r *redisRateLimiter) Remaining(ctx context.Context, key string, limit int, window time.Duration) (int, error) {
	count, err := r.Count(ctx, key, window)
	if err != nil {
		return 0, err
	}
	
	remaining := limit - count
	if remaining < 0 {
		remaining = 0
	}
	
	return remaining, nil
}

// CleanupExpiredKeys removes expired rate limit keys (should be called periodically)
func (r *redisRateLimiter) CleanupExpiredKeys(ctx context.Context) (int64, error) {
	pattern := RateLimitPrefix + "*"
	
	var cursor uint64
	var deletedCount int64
	
	for {
		keys, nextCursor, err := r.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			r.logger.Error("rate limiter cleanup scan failed", zap.Error(err))
			return deletedCount, fmt.Errorf("rate limiter cleanup scan failed: %w", err)
		}
		
		for _, key := range keys {
			// Check if key exists and has TTL
			ttl, err := r.client.TTL(ctx, key).Result()
			if err != nil {
				continue
			}
			
			// If TTL is -1, the key exists but has no expiration set
			// This shouldn't happen but we'll clean it up anyway
			if ttl == -1 {
				r.client.Del(ctx, key)
				deletedCount++
			}
		}
		
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	
	if deletedCount > 0 {
		r.logger.Info("rate limiter cleanup completed",
			zap.Int64("deleted_keys", deletedCount))
	}
	
	return deletedCount, nil
}