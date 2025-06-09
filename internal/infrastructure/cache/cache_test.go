package cache

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/infrastructure/config"
)

func setupTestRedis(t *testing.T) (*redisCache, *miniredis.Miniredis, func()) {
	// Start mini Redis server
	mr, err := miniredis.Run()
	require.NoError(t, err)

	// Create test configuration
	cfg := &config.RedisConfig{
		URL:          mr.Addr(),
		Password:     "",
		DB:           0,
		PoolSize:     5,
		MinIdleConns: 1,
		MaxRetries:   3,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	}

	logger := zaptest.NewLogger(t)

	// Create cache
	cache, err := NewRedisCache(cfg, logger)
	require.NoError(t, err)

	redisCache := cache.(*redisCache)

	cleanup := func() {
		cache.Close()
		mr.Close()
	}

	return redisCache, mr, cleanup
}

func TestNewRedisCache(t *testing.T) {
	t.Run("successful creation", func(t *testing.T) {
		cache, _, cleanup := setupTestRedis(t)
		defer cleanup()

		assert.NotNil(t, cache)
		assert.NotNil(t, cache.client)
		assert.NotNil(t, cache.logger)
	})

	t.Run("nil logger", func(t *testing.T) {
		cfg := &config.RedisConfig{URL: "localhost:6379"}
		_, err := NewRedisCache(cfg, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "logger is required")
	})

	t.Run("nil config", func(t *testing.T) {
		logger := zaptest.NewLogger(t)
		_, err := NewRedisCache(nil, logger)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "redis config is required")
	})

	t.Run("connection failure", func(t *testing.T) {
		cfg := &config.RedisConfig{
			URL:         "localhost:9999", // Non-existent port
			DialTimeout: 100 * time.Millisecond,
		}
		logger := zaptest.NewLogger(t)

		_, err := NewRedisCache(cfg, logger)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "redis connection failed")
	})
}

func TestRedisCache_BasicOperations(t *testing.T) {
	cache, _, cleanup := setupTestRedis(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("Set and Get", func(t *testing.T) {
		key := "test:key"
		value := "test_value"

		// Set
		err := cache.Set(ctx, key, value, time.Hour)
		require.NoError(t, err)

		// Get
		result, err := cache.Get(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, value, result)
	})

	t.Run("Get non-existent key", func(t *testing.T) {
		_, err := cache.Get(ctx, "non_existent_key")
		assert.Error(t, err)
		
		var notFoundErr ErrCacheKeyNotFound
		assert.ErrorAs(t, err, &notFoundErr)
		assert.Equal(t, "non_existent_key", notFoundErr.Key)
	})

	t.Run("Delete", func(t *testing.T) {
		key := "test:delete"
		value := "delete_me"

		// Set
		err := cache.Set(ctx, key, value, time.Hour)
		require.NoError(t, err)

		// Verify exists
		exists, err := cache.Exists(ctx, key)
		require.NoError(t, err)
		assert.True(t, exists)

		// Delete
		err = cache.Delete(ctx, key)
		require.NoError(t, err)

		// Verify deleted
		exists, err = cache.Exists(ctx, key)
		require.NoError(t, err)
		assert.False(t, exists)
	})

	t.Run("Exists", func(t *testing.T) {
		key := "test:exists"

		// Should not exist initially
		exists, err := cache.Exists(ctx, key)
		require.NoError(t, err)
		assert.False(t, exists)

		// Set value
		err = cache.Set(ctx, key, "value", time.Hour)
		require.NoError(t, err)

		// Should exist now
		exists, err = cache.Exists(ctx, key)
		require.NoError(t, err)
		assert.True(t, exists)
	})
}

func TestRedisCache_AtomicOperations(t *testing.T) {
	cache, mr, cleanup := setupTestRedis(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("SetNX", func(t *testing.T) {
		key := "test:setnx"
		value1 := "first_value"
		value2 := "second_value"

		// First SetNX should succeed
		success, err := cache.SetNX(ctx, key, value1, time.Hour)
		require.NoError(t, err)
		assert.True(t, success)

		// Second SetNX should fail (key exists)
		success, err = cache.SetNX(ctx, key, value2, time.Hour)
		require.NoError(t, err)
		assert.False(t, success)

		// Value should be the first one
		result, err := cache.Get(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, value1, result)
	})

	t.Run("Increment", func(t *testing.T) {
		key := "test:incr"

		// First increment (key doesn't exist)
		result, err := cache.Increment(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, int64(1), result)

		// Second increment
		result, err = cache.Increment(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, int64(2), result)

		// Third increment
		result, err = cache.Increment(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, int64(3), result)
	})

	t.Run("Expire", func(t *testing.T) {
		key := "test:expire"
		value := "expire_me"

		// Set value without TTL
		err := cache.Set(ctx, key, value, 0)
		require.NoError(t, err)

		// Set expiration
		err = cache.Expire(ctx, key, 1*time.Second)
		require.NoError(t, err)

		// Value should exist immediately
		result, err := cache.Get(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, value, result)

		// Fast forward time in miniredis to trigger expiration
		mr.FastForward(1100 * time.Millisecond)

		// Value should be expired
		_, err = cache.Get(ctx, key)
		assert.Error(t, err)
		var notFoundErr ErrCacheKeyNotFound
		assert.ErrorAs(t, err, &notFoundErr)
	})
}

func TestRedisCache_JSONOperations(t *testing.T) {
	cache, _, cleanup := setupTestRedis(t)
	defer cleanup()

	ctx := context.Background()

	type TestStruct struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
		Tags []string `json:"tags"`
	}

	t.Run("SetJSON and GetJSON", func(t *testing.T) {
		key := "test:json"
		original := TestStruct{
			ID:   123,
			Name: "test_object",
			Tags: []string{"tag1", "tag2"},
		}

		// Set JSON
		err := cache.SetJSON(ctx, key, original, time.Hour)
		require.NoError(t, err)

		// Get JSON
		var result TestStruct
		err = cache.GetJSON(ctx, key, &result)
		require.NoError(t, err)

		assert.Equal(t, original.ID, result.ID)
		assert.Equal(t, original.Name, result.Name)
		assert.Equal(t, original.Tags, result.Tags)
	})

	t.Run("GetJSON with invalid JSON", func(t *testing.T) {
		key := "test:invalid_json"

		// Set invalid JSON
		err := cache.Set(ctx, key, "invalid json", time.Hour)
		require.NoError(t, err)

		// Try to get as JSON
		var result TestStruct
		err = cache.GetJSON(ctx, key, &result)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "json unmarshal failed")
	})

	t.Run("SetJSON with invalid object", func(t *testing.T) {
		key := "test:invalid_object"

		// Try to marshal invalid object (circular reference)
		type CircularRef struct {
			Self *CircularRef `json:"self"`
		}
		circular := &CircularRef{}
		circular.Self = circular

		err := cache.SetJSON(ctx, key, circular, time.Hour)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "json marshal failed")
	})
}

func TestRedisCache_TTL(t *testing.T) {
	cache, mr, cleanup := setupTestRedis(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("key expires after TTL", func(t *testing.T) {
		key := "test:ttl"
		value := "expires_soon"
		ttl := 1 * time.Second

		// Set with short TTL
		err := cache.Set(ctx, key, value, ttl)
		require.NoError(t, err)

		// Should exist immediately
		result, err := cache.Get(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, value, result)

		// Fast forward time in miniredis to trigger expiration
		mr.FastForward(1100 * time.Millisecond)

		// Should be expired
		_, err = cache.Get(ctx, key)
		assert.Error(t, err)
		var notFoundErr ErrCacheKeyNotFound
		assert.ErrorAs(t, err, &notFoundErr)
	})

	t.Run("no TTL means no expiration", func(t *testing.T) {
		key := "test:no_ttl"
		value := "never_expires"

		// Set without TTL
		err := cache.Set(ctx, key, value, 0)
		require.NoError(t, err)

		// Should exist after some time
		time.Sleep(50 * time.Millisecond)
		result, err := cache.Get(ctx, key)
		require.NoError(t, err)
		assert.Equal(t, value, result)
	})
}

func TestRedisCache_ConcurrentAccess(t *testing.T) {
	cache, _, cleanup := setupTestRedis(t)
	defer cleanup()

	ctx := context.Background()
	
	t.Run("concurrent increments", func(t *testing.T) {
		key := "test:concurrent_incr"
		numGoroutines := 10
		incrementsPerGoroutine := 100

		done := make(chan struct{}, numGoroutines)

		// Start concurrent increments
		for i := 0; i < numGoroutines; i++ {
			go func() {
				defer func() { done <- struct{}{} }()
				
				for j := 0; j < incrementsPerGoroutine; j++ {
					_, err := cache.Increment(ctx, key)
					assert.NoError(t, err)
				}
			}()
		}

		// Wait for all goroutines to complete
		for i := 0; i < numGoroutines; i++ {
			<-done
		}

		// Check final value
		finalValue, err := cache.Get(ctx, key)
		require.NoError(t, err)

		assert.Equal(t, "1000", finalValue)
	})
}

func TestRedisCache_Close(t *testing.T) {
	cache, _, cleanup := setupTestRedis(t)
	defer cleanup()

	// Close should not error
	err := cache.Close()
	assert.NoError(t, err)

	// Second close may error but should not panic
	// (Redis client doesn't support idempotent close)
	cache.Close()
}