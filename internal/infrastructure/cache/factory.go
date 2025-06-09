package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/infrastructure/config"
)

// CacheManager provides access to all cache-related services
type CacheManager struct {
	Cache        Cache
	RateLimiter  RateLimiter
	SessionStore SessionStore
	client       *redis.Client
	logger       *zap.Logger
}

// NewCacheManager creates a new cache manager with all cache services
func NewCacheManager(cfg *config.RedisConfig, logger *zap.Logger) (*CacheManager, error) {
	if cfg == nil {
		return nil, fmt.Errorf("redis config is required")
	}

	if logger == nil {
		return nil, fmt.Errorf("logger is required")
	}

	// Create Redis client
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

	// Create cache implementation
	cache, err := NewRedisCache(cfg, logger)
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to create redis cache: %w", err)
	}

	// Create rate limiter
	rateLimiter := NewRedisRateLimiter(client, logger)

	// Create session store
	sessionStore := NewRedisSessionStore(cache, client, logger)

	logger.Info("cache manager initialized",
		zap.String("addr", cfg.URL),
		zap.Int("db", cfg.DB),
		zap.Int("pool_size", cfg.PoolSize))

	return &CacheManager{
		Cache:        cache,
		RateLimiter:  rateLimiter,
		SessionStore: sessionStore,
		client:       client,
		logger:       logger,
	}, nil
}

// Close closes all cache connections and cleans up resources
func (cm *CacheManager) Close() error {
	var errors []error

	// Close cache
	if err := cm.Cache.Close(); err != nil {
		errors = append(errors, fmt.Errorf("cache close failed: %w", err))
	}

	// Close Redis client
	if err := cm.client.Close(); err != nil {
		errors = append(errors, fmt.Errorf("redis client close failed: %w", err))
	}

	if len(errors) > 0 {
		return fmt.Errorf("cache manager close errors: %v", errors)
	}

	cm.logger.Info("cache manager closed successfully")
	return nil
}

// HealthCheck verifies that all cache services are operational
func (cm *CacheManager) HealthCheck(ctx context.Context) error {
	// Check Redis connection
	if err := cm.client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("redis health check failed: %w", err)
	}

	// Test basic cache operations
	testKey := "health_check:test"
	testValue := time.Now().Unix()

	// Test Set
	if err := cm.Cache.Set(ctx, testKey, testValue, 10*time.Second); err != nil {
		return fmt.Errorf("cache set health check failed: %w", err)
	}

	// Test Get
	if _, err := cm.Cache.Get(ctx, testKey); err != nil {
		return fmt.Errorf("cache get health check failed: %w", err)
	}

	// Test Delete
	if err := cm.Cache.Delete(ctx, testKey); err != nil {
		return fmt.Errorf("cache delete health check failed: %w", err)
	}

	// Test rate limiter
	allowed, err := cm.RateLimiter.Allow(ctx, "health_check", 1, time.Minute)
	if err != nil {
		return fmt.Errorf("rate limiter health check failed: %w", err)
	}
	if !allowed {
		return fmt.Errorf("rate limiter health check unexpected result")
	}

	// Clean up rate limiter test
	if err := cm.RateLimiter.Reset(ctx, "health_check"); err != nil {
		cm.logger.Warn("failed to clean up rate limiter health check", zap.Error(err))
	}

	return nil
}

// GetStats returns cache statistics for monitoring
func (cm *CacheManager) GetStats(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Redis info
	info, err := cm.client.Info(ctx, "memory", "stats").Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get redis info: %w", err)
	}

	stats["redis_info"] = info

	// Connection pool stats
	poolStats := cm.client.PoolStats()
	stats["pool_stats"] = map[string]interface{}{
		"hits":        poolStats.Hits,
		"misses":      poolStats.Misses,
		"timeouts":    poolStats.Timeouts,
		"total_conns": poolStats.TotalConns,
		"idle_conns":  poolStats.IdleConns,
		"stale_conns": poolStats.StaleConns,
	}

	// Database size
	dbSize, err := cm.client.DBSize(ctx).Result()
	if err != nil {
		cm.logger.Warn("failed to get database size", zap.Error(err))
	} else {
		stats["db_size"] = dbSize
	}

	return stats, nil
}

// StartBackgroundCleanup starts background cleanup routines for sessions and rate limits
func (cm *CacheManager) StartBackgroundCleanup(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	
	go func() {
		defer ticker.Stop()
		
		for {
			select {
			case <-ctx.Done():
				cm.logger.Info("background cleanup stopped")
				return
			case <-ticker.C:
				cm.runCleanup(ctx)
			}
		}
	}()
	
	cm.logger.Info("background cleanup started", zap.Duration("interval", interval))
}

// runCleanup performs periodic cleanup of expired cache entries
func (cm *CacheManager) runCleanup(ctx context.Context) {
	cleanupCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Clean up sessions
	sessionsCleaned, err := cm.SessionStore.CleanupExpired(cleanupCtx)
	if err != nil {
		cm.logger.Error("session cleanup failed", zap.Error(err))
	} else if sessionsCleaned > 0 {
		cm.logger.Info("session cleanup completed", zap.Int64("cleaned", sessionsCleaned))
	}

	// Clean up rate limit keys if the rate limiter supports it
	if rateLimiter, ok := cm.RateLimiter.(*redisRateLimiter); ok {
		rateLimitsCleaned, err := rateLimiter.CleanupExpiredKeys(cleanupCtx)
		if err != nil {
			cm.logger.Error("rate limit cleanup failed", zap.Error(err))
		} else if rateLimitsCleaned > 0 {
			cm.logger.Info("rate limit cleanup completed", zap.Int64("cleaned", rateLimitsCleaned))
		}
	}
}