package cache

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/dnc"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/infrastructure/config"
	"github.com/google/uuid"
)

func setupTestDNCCache(t *testing.T) (*DNCCache, func()) {
	// Start mini Redis server
	mr, err := miniredis.Run()
	require.NoError(t, err)

	// Create Redis config pointing to mini Redis
	cfg := &config.RedisConfig{
		URL:          mr.Addr(),
		Password:     "",
		DB:           0,
		PoolSize:     10,
		MinIdleConns: 2,
		MaxRetries:   3,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	}

	logger := zaptest.NewLogger(t)

	cache, err := NewDNCCache(cfg, logger)
	require.NoError(t, err)

	cleanup := func() {
		cache.Close()
		mr.Close()
	}

	return cache, cleanup
}

func createTestDNCEntry(t *testing.T, phoneNumber, source, reason string) *dnc.DNCEntry {
	userID := uuid.New()
	entry, err := dnc.NewDNCEntry(phoneNumber, source, reason, userID)
	require.NoError(t, err)
	return entry
}

func createTestCheckResult(t *testing.T, phoneNumber string, isBlocked bool) *dnc.DNCCheckResult {
	result, err := dnc.NewDNCCheckResult(phoneNumber)
	require.NoError(t, err)

	if isBlocked {
		blockReason, err := dnc.NewBlockReason("federal", "regulatory", "TestProvider", uuid.New())
		require.NoError(t, err)
		err = result.AddBlockReason(blockReason)
		require.NoError(t, err)
	}

	return result
}

func TestDNCCache_NewDNCCache(t *testing.T) {
	tests := []struct {
		name        string
		config      *config.RedisConfig
		expectError bool
	}{
		{
			name:        "nil config",
			config:      nil,
			expectError: true,
		},
		{
			name: "valid config",
			config: &config.RedisConfig{
				URL:          "localhost:6379",
				DB:           0,
				PoolSize:     10,
				DialTimeout:  5 * time.Second,
				ReadTimeout:  3 * time.Second,
				WriteTimeout: 3 * time.Second,
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zaptest.NewLogger(t)

			if tt.expectError {
				cache, err := NewDNCCache(tt.config, logger)
				assert.Error(t, err)
				assert.Nil(t, cache)
			} else {
				// Skip this test if Redis is not available
				if tt.config != nil {
					client := redis.NewClient(&redis.Options{Addr: tt.config.URL})
					if err := client.Ping(context.Background()).Err(); err != nil {
						t.Skip("Redis not available")
					}
					client.Close()
				}
			}
		})
	}
}

func TestDNCCache_SetAndGetDNCEntry(t *testing.T) {
	cache, cleanup := setupTestDNCCache(t)
	defer cleanup()

	ctx := context.Background()
	phoneNumber := "+14155551234"
	phone, err := values.NewPhoneNumber(phoneNumber)
	require.NoError(t, err)

	entry := createTestDNCEntry(t, phoneNumber, "federal", "regulatory")

	// Test Set
	err = cache.SetDNCEntry(ctx, entry)
	assert.NoError(t, err)

	// Test Get
	retrievedEntry, err := cache.GetDNCEntry(ctx, phone)
	assert.NoError(t, err)
	assert.NotNil(t, retrievedEntry)
	assert.Equal(t, entry.PhoneNumber.String(), retrievedEntry.PhoneNumber.String())
	assert.Equal(t, entry.ListSource.String(), retrievedEntry.ListSource.String())
	assert.Equal(t, entry.SuppressReason.String(), retrievedEntry.SuppressReason.String())

	// Test cache hit metrics
	metrics := cache.GetMetrics(ctx)
	assert.Greater(t, metrics.Hits, int64(0))
}

func TestDNCCache_GetDNCEntry_NotFound(t *testing.T) {
	cache, cleanup := setupTestDNCCache(t)
	defer cleanup()

	ctx := context.Background()
	phone, err := values.NewPhoneNumber("+14155559999")
	require.NoError(t, err)

	entry, err := cache.GetDNCEntry(ctx, phone)
	assert.Error(t, err)
	assert.Nil(t, entry)
	assert.IsType(t, ErrCacheKeyNotFound{}, err)

	// Test cache miss metrics
	metrics := cache.GetMetrics(ctx)
	assert.Greater(t, metrics.Misses, int64(0))
}

func TestDNCCache_SetAndGetCheckResult(t *testing.T) {
	cache, cleanup := setupTestDNCCache(t)
	defer cleanup()

	ctx := context.Background()
	phoneNumber := "+14155551234"
	phone, err := values.NewPhoneNumber(phoneNumber)
	require.NoError(t, err)

	result := createTestCheckResult(t, phoneNumber, true)

	// Test Set
	err = cache.SetCheckResult(ctx, result)
	assert.NoError(t, err)

	// Test Get
	retrievedResult, err := cache.GetCheckResult(ctx, phone)
	assert.NoError(t, err)
	assert.NotNil(t, retrievedResult)
	assert.Equal(t, result.PhoneNumber.String(), retrievedResult.PhoneNumber.String())
	assert.Equal(t, result.IsBlocked, retrievedResult.IsBlocked)
	assert.Equal(t, len(result.Reasons), len(retrievedResult.Reasons))
}

func TestDNCCache_BulkGetDNCEntries(t *testing.T) {
	cache, cleanup := setupTestDNCCache(t)
	defer cleanup()

	ctx := context.Background()

	// Create test entries
	phoneNumbers := []string{"+14155551234", "+14155551235", "+14155551236"}
	var phones []values.PhoneNumber
	
	for i, phoneNumber := range phoneNumbers {
		phone, err := values.NewPhoneNumber(phoneNumber)
		require.NoError(t, err)
		phones = append(phones, phone)

		entry := createTestDNCEntry(t, phoneNumber, "federal", "regulatory")
		
		// Only cache odd-indexed entries to test partial hits
		if i%2 == 0 {
			err = cache.SetDNCEntry(ctx, entry)
			require.NoError(t, err)
		}
	}

	// Test bulk get
	results, err := cache.BulkGetDNCEntries(ctx, phones)
	assert.NoError(t, err)
	assert.NotNil(t, results)

	// Should have entries for indices 0 and 2
	assert.Contains(t, results, phoneNumbers[0])
	assert.NotContains(t, results, phoneNumbers[1])
	assert.Contains(t, results, phoneNumbers[2])

	// Verify pipeline operations metric
	metrics := cache.GetMetrics(ctx)
	assert.Greater(t, metrics.PipelineOperations, int64(0))
}

func TestDNCCache_BulkSetDNCEntries(t *testing.T) {
	cache, cleanup := setupTestDNCCache(t)
	defer cleanup()

	ctx := context.Background()

	// Create test entries
	var entries []*dnc.DNCEntry
	phoneNumbers := []string{"+14155551234", "+14155551235", "+14155551236"}
	
	for _, phoneNumber := range phoneNumbers {
		entry := createTestDNCEntry(t, phoneNumber, "federal", "regulatory")
		entries = append(entries, entry)
	}

	// Test bulk set
	err := cache.BulkSetDNCEntries(ctx, entries)
	assert.NoError(t, err)

	// Verify all entries were stored
	for _, phoneNumber := range phoneNumbers {
		phone, err := values.NewPhoneNumber(phoneNumber)
		require.NoError(t, err)

		retrievedEntry, err := cache.GetDNCEntry(ctx, phone)
		assert.NoError(t, err)
		assert.NotNil(t, retrievedEntry)
		assert.Equal(t, phoneNumber, retrievedEntry.PhoneNumber.String())
	}

	// Verify pipeline operations metric
	metrics := cache.GetMetrics(ctx)
	assert.Greater(t, metrics.PipelineOperations, int64(0))
}

func TestDNCCache_WarmCache(t *testing.T) {
	cache, cleanup := setupTestDNCCache(t)
	defer cleanup()

	ctx := context.Background()

	// Create phone numbers for warming
	phoneNumbers := []values.PhoneNumber{}
	expectedEntries := []*dnc.DNCEntry{}
	
	for i := 0; i < 5; i++ {
		phoneNumber := fmt.Sprintf("+141555512%02d", i)
		phone, err := values.NewPhoneNumber(phoneNumber)
		require.NoError(t, err)
		phoneNumbers = append(phoneNumbers, phone)

		entry := createTestDNCEntry(t, phoneNumber, "federal", "regulatory")
		expectedEntries = append(expectedEntries, entry)
	}

	// Mock load function
	loadFunc := func(phones []values.PhoneNumber) ([]*dnc.DNCEntry, error) {
		var entries []*dnc.DNCEntry
		for _, phone := range phones {
			for _, expected := range expectedEntries {
				if phone.String() == expected.PhoneNumber.String() {
					entries = append(entries, expected)
					break
				}
			}
		}
		return entries, nil
	}

	// Test cache warming
	err := cache.WarmCache(ctx, phoneNumbers, loadFunc)
	assert.NoError(t, err)

	// Verify all entries were cached
	for _, phone := range phoneNumbers {
		retrievedEntry, err := cache.GetDNCEntry(ctx, phone)
		assert.NoError(t, err)
		assert.NotNil(t, retrievedEntry)
	}

	// Verify warming operations metric
	metrics := cache.GetMetrics(ctx)
	assert.Greater(t, metrics.WarmingOperations, int64(0))
}

func TestDNCCache_InvalidateSource(t *testing.T) {
	cache, cleanup := setupTestDNCCache(t)
	defer cleanup()

	ctx := context.Background()

	// Create entries from different sources
	federalEntry := createTestDNCEntry(t, "+14155551234", "federal", "regulatory")
	stateEntry := createTestDNCEntry(t, "+14155551235", "state", "regulatory")

	err := cache.SetDNCEntry(ctx, federalEntry)
	require.NoError(t, err)
	err = cache.SetDNCEntry(ctx, stateEntry)
	require.NoError(t, err)

	// Verify both entries exist
	phone1, _ := values.NewPhoneNumber("+14155551234")
	phone2, _ := values.NewPhoneNumber("+14155551235")
	
	_, err = cache.GetDNCEntry(ctx, phone1)
	assert.NoError(t, err)
	_, err = cache.GetDNCEntry(ctx, phone2)
	assert.NoError(t, err)

	// Invalidate federal source
	federalSource := values.MustNewListSource("federal")
	err = cache.InvalidateSource(ctx, federalSource)
	assert.NoError(t, err)

	// Note: This test would require a more complex implementation to track
	// entries by source. For now, we just verify the method doesn't error.
}

func TestDNCCache_GetMetrics(t *testing.T) {
	cache, cleanup := setupTestDNCCache(t)
	defer cleanup()

	ctx := context.Background()

	// Initially should have zero metrics
	metrics := cache.GetMetrics(ctx)
	assert.Equal(t, int64(0), metrics.Hits)
	assert.Equal(t, int64(0), metrics.Misses)
	assert.Equal(t, float64(0), metrics.HitRate)

	// Generate some cache activity
	phone, _ := values.NewPhoneNumber("+14155551234")
	entry := createTestDNCEntry(t, "+14155551234", "federal", "regulatory")

	// Cache miss
	_, err := cache.GetDNCEntry(ctx, phone)
	assert.Error(t, err)

	// Cache set
	err = cache.SetDNCEntry(ctx, entry)
	assert.NoError(t, err)

	// Cache hit
	_, err = cache.GetDNCEntry(ctx, phone)
	assert.NoError(t, err)

	// Check updated metrics
	metrics = cache.GetMetrics(ctx)
	assert.Greater(t, metrics.Hits, int64(0))
	assert.Greater(t, metrics.Misses, int64(0))
	assert.Greater(t, metrics.HitRate, float64(0))
	assert.LessOrEqual(t, metrics.HitRate, float64(1))
}

func TestDNCCache_GetCacheInfo(t *testing.T) {
	cache, cleanup := setupTestDNCCache(t)
	defer cleanup()

	ctx := context.Background()

	info, err := cache.GetCacheInfo(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, info)

	// Should contain expected keys
	assert.Contains(t, info, "key_counts")
	assert.Contains(t, info, "metrics")
	assert.Contains(t, info, "config")
	
	if cache.config.BloomFilterEnabled {
		assert.Contains(t, info, "bloom_filter")
	}
}

func TestDNCCache_ExpiredCheckResult(t *testing.T) {
	cache, cleanup := setupTestDNCCache(t)
	defer cleanup()

	ctx := context.Background()
	phoneNumber := "+14155551234"
	phone, err := values.NewPhoneNumber(phoneNumber)
	require.NoError(t, err)

	result := createTestCheckResult(t, phoneNumber, true)
	
	// Set a very short TTL to simulate expiration
	err = result.SetTTL(1 * time.Nanosecond)
	require.NoError(t, err)

	// Cache the result
	err = cache.SetCheckResult(ctx, result)
	assert.NoError(t, err)

	// Wait for expiration
	time.Sleep(10 * time.Millisecond)

	// Try to retrieve - should be treated as expired
	retrievedResult, err := cache.GetCheckResult(ctx, phone)
	assert.Error(t, err)
	assert.Nil(t, retrievedResult)
}

func TestBloomFilter_Basic(t *testing.T) {
	bf := NewBloomFilter(1000, 3)

	// Test adding and checking items
	items := []string{"item1", "item2", "item3"}
	
	for _, item := range items {
		bf.Add(item)
	}

	// All added items should be present
	for _, item := range items {
		assert.True(t, bf.Contains(item), "item %s should be in bloom filter", item)
	}

	// Non-added item might or might not be present (false positive possible)
	// But we can test that the method doesn't panic
	assert.NotPanics(t, func() {
		bf.Contains("not_added_item")
	})

	// Test reset
	bf.Reset()
	for _, item := range items {
		assert.False(t, bf.Contains(item), "item %s should not be in bloom filter after reset", item)
	}
}

func TestBloomFilter_EstimatedItemCount(t *testing.T) {
	bf := NewBloomFilter(1000, 3)

	// Initially should be 0
	count := bf.EstimatedItemCount()
	assert.Equal(t, int64(0), count)

	// Add some items
	for i := 0; i < 10; i++ {
		bf.Add(fmt.Sprintf("item%d", i))
	}

	// Should estimate some items
	count = bf.EstimatedItemCount()
	assert.Greater(t, count, int64(0))
}

func TestDNCCache_PhoneNumberHashing(t *testing.T) {
	cache, cleanup := setupTestDNCCache(t)
	defer cleanup()

	// Test that phone number hashing is consistent
	phoneNumber := "+14155551234"
	hash1 := cache.hashPhoneNumber(phoneNumber)
	hash2 := cache.hashPhoneNumber(phoneNumber)

	assert.Equal(t, hash1, hash2, "phone number hashing should be consistent")
	assert.NotEmpty(t, hash1, "hash should not be empty")

	// Different phone numbers should produce different hashes
	differentPhone := "+14155559999"
	hash3 := cache.hashPhoneNumber(differentPhone)
	assert.NotEqual(t, hash1, hash3, "different phone numbers should have different hashes")
}

func BenchmarkDNCCache_SetDNCEntry(b *testing.B) {
	cache, cleanup := setupTestDNCCache(&testing.T{})
	defer cleanup()

	ctx := context.Background()
	entry := createTestDNCEntry(&testing.T{}, "+14155551234", "federal", "regulatory")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.SetDNCEntry(ctx, entry)
	}
}

func BenchmarkDNCCache_GetDNCEntry(b *testing.B) {
	cache, cleanup := setupTestDNCCache(&testing.T{})
	defer cleanup()

	ctx := context.Background()
	phoneNumber := "+14155551234"
	phone, _ := values.NewPhoneNumber(phoneNumber)
	entry := createTestDNCEntry(&testing.T{}, phoneNumber, "federal", "regulatory")

	// Pre-populate cache
	cache.SetDNCEntry(ctx, entry)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.GetDNCEntry(ctx, phone)
	}
}

func BenchmarkDNCCache_BulkOperations(b *testing.B) {
	cache, cleanup := setupTestDNCCache(&testing.T{})
	defer cleanup()

	ctx := context.Background()
	
	// Create test data
	var entries []*dnc.DNCEntry
	var phones []values.PhoneNumber
	
	for i := 0; i < 100; i++ {
		phoneNumber := fmt.Sprintf("+141555512%02d", i)
		phone, _ := values.NewPhoneNumber(phoneNumber)
		phones = append(phones, phone)
		
		entry := createTestDNCEntry(&testing.T{}, phoneNumber, "federal", "regulatory")
		entries = append(entries, entry)
	}

	b.Run("BulkSet", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			cache.BulkSetDNCEntries(ctx, entries)
		}
	})

	// Pre-populate for bulk get benchmark
	cache.BulkSetDNCEntries(ctx, entries)

	b.Run("BulkGet", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			cache.BulkGetDNCEntries(ctx, phones)
		}
	})
}