package cache

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/audit"
)

func setupTestAuditCache(t *testing.T) (*AuditCache, *miniredis.Miniredis) {
	// Create mini Redis instance
	s, err := miniredis.Run()
	require.NoError(t, err)

	// Create Redis client
	client := redis.NewClient(&redis.Options{
		Addr: s.Addr(),
	})

	// Create logger
	logger := zaptest.NewLogger(t)

	// Create audit cache with test config
	config := &AuditCacheConfig{
		MaxBatchSize:  10,
		WarmupSize:    100,
		LRUSize:       1000,
		TTLJitter:     1 * time.Second,
		EnableMetrics: true,
	}

	cache, err := NewAuditCache(client, logger, config)
	require.NoError(t, err)

	return cache, s
}

func createTestEvent(t *testing.T) *audit.Event {
	event, err := audit.NewEvent(
		audit.EventCallInitiated,
		"user-123",
		"call-456",
		"initiate_call",
	)
	require.NoError(t, err)

	// Set sequence number
	event.SequenceNum = 1

	// Compute hash
	hash, err := event.ComputeHash("previous-hash")
	require.NoError(t, err)
	require.NotEmpty(t, hash)

	return event
}

func TestNewAuditCache(t *testing.T) {
	logger := zaptest.NewLogger(t)

	t.Run("valid configuration", func(t *testing.T) {
		s, err := miniredis.Run()
		require.NoError(t, err)
		defer s.Close()

		client := redis.NewClient(&redis.Options{Addr: s.Addr()})
		cache, err := NewAuditCache(client, logger, nil)
		assert.NoError(t, err)
		assert.NotNil(t, cache)
	})

	t.Run("nil client", func(t *testing.T) {
		cache, err := NewAuditCache(nil, logger, nil)
		assert.Error(t, err)
		assert.Nil(t, cache)
	})

	t.Run("nil logger", func(t *testing.T) {
		s, err := miniredis.Run()
		require.NoError(t, err)
		defer s.Close()

		client := redis.NewClient(&redis.Options{Addr: s.Addr()})
		cache, err := NewAuditCache(client, nil, nil)
		assert.Error(t, err)
		assert.Nil(t, cache)
	})
}

func TestEventCaching(t *testing.T) {
	ctx := context.Background()

	t.Run("set and get event", func(t *testing.T) {
		cache, s := setupTestAuditCache(t)
		defer s.Close()

		event := createTestEvent(t)

		// Set event
		err := cache.SetEvent(ctx, event)
		assert.NoError(t, err)

		// Get event
		retrieved, err := cache.GetEvent(ctx, event.ID)
		assert.NoError(t, err)
		assert.NotNil(t, retrieved)
		assert.Equal(t, event.ID, retrieved.ID)
		assert.Equal(t, event.EventHash, retrieved.EventHash)
	})

	t.Run("cache miss", func(t *testing.T) {
		cache, s := setupTestAuditCache(t)
		defer s.Close()

		randomID := uuid.New()
		retrieved, err := cache.GetEvent(ctx, randomID)
		assert.NoError(t, err)
		assert.Nil(t, retrieved)
	})

	t.Run("batch get events", func(t *testing.T) {
		cache, s := setupTestAuditCache(t)
		defer s.Close()

		// Create and cache multiple events
		events := make([]*audit.Event, 5)
		eventIDs := make([]uuid.UUID, 5)
		for i := 0; i < 5; i++ {
			event := createTestEvent(t)
			event.SequenceNum = int64(i + 1)
			events[i] = event
			eventIDs[i] = event.ID
			
			err := cache.SetEvent(ctx, event)
			require.NoError(t, err)
		}

		// Batch get
		retrieved, err := cache.GetEvents(ctx, eventIDs)
		assert.NoError(t, err)
		assert.Len(t, retrieved, 5)

		for _, event := range events {
			assert.Contains(t, retrieved, event.ID)
		}
	})

	t.Run("batch set events", func(t *testing.T) {
		cache, s := setupTestAuditCache(t)
		defer s.Close()

		// Create multiple events
		events := make([]*audit.Event, 5)
		for i := 0; i < 5; i++ {
			event := createTestEvent(t)
			event.SequenceNum = int64(i + 1)
			events[i] = event
		}

		// Batch set
		err := cache.SetEvents(ctx, events)
		assert.NoError(t, err)

		// Verify all cached
		for _, event := range events {
			retrieved, err := cache.GetEvent(ctx, event.ID)
			assert.NoError(t, err)
			assert.NotNil(t, retrieved)
		}
	})

	t.Run("invalidate event", func(t *testing.T) {
		cache, s := setupTestAuditCache(t)
		defer s.Close()

		event := createTestEvent(t)

		// Set event
		err := cache.SetEvent(ctx, event)
		require.NoError(t, err)

		// Invalidate
		err = cache.InvalidateEvent(ctx, event.ID)
		assert.NoError(t, err)

		// Should be gone
		retrieved, err := cache.GetEvent(ctx, event.ID)
		assert.NoError(t, err)
		assert.Nil(t, retrieved)
	})

	t.Run("LRU eviction", func(t *testing.T) {
		cache, s := setupTestAuditCache(t)
		defer s.Close()

		// Set LRU size to 3 for testing
		cache.lruSize = 3

		// Add 5 events (should evict oldest 2)
		eventIDs := make([]uuid.UUID, 5)
		for i := 0; i < 5; i++ {
			event := createTestEvent(t)
			event.SequenceNum = int64(i + 1)
			eventIDs[i] = event.ID
			
			err := cache.SetEvent(ctx, event)
			require.NoError(t, err)
			
			// Small delay to ensure different timestamps
			time.Sleep(10 * time.Millisecond)
		}

		// Check LRU size
		lruKey := cache.lruKey()
		count, err := cache.client.ZCard(ctx, lruKey).Result()
		assert.NoError(t, err)
		assert.Equal(t, int64(3), count)
	})
}

func TestHashChainCaching(t *testing.T) {
	ctx := context.Background()

	t.Run("latest hash operations", func(t *testing.T) {
		cache, s := setupTestAuditCache(t)
		defer s.Close()

		// Set latest hash
		hash := "abcd1234567890"
		seq := int64(100)
		err := cache.SetLatestHash(ctx, hash, seq)
		assert.NoError(t, err)

		// Get latest hash
		retrievedHash, retrievedSeq, err := cache.GetLatestHash(ctx)
		assert.NoError(t, err)
		assert.Equal(t, hash, retrievedHash)
		assert.Equal(t, seq, retrievedSeq)
	})

	t.Run("hash chain operations", func(t *testing.T) {
		cache, s := setupTestAuditCache(t)
		defer s.Close()

		// Create hash chain
		chain := map[int64]string{
			1: "hash1",
			2: "hash2",
			3: "hash3",
			4: "hash4",
			5: "hash5",
		}

		// Set chain
		err := cache.SetHashChain(ctx, chain)
		assert.NoError(t, err)

		// Get chain range
		retrieved, err := cache.GetHashChain(ctx, 2, 4)
		assert.NoError(t, err)
		assert.Len(t, retrieved, 3)
		assert.Equal(t, "hash2", retrieved[2])
		assert.Equal(t, "hash3", retrieved[3])
		assert.Equal(t, "hash4", retrieved[4])
	})

	t.Run("empty hash validation", func(t *testing.T) {
		cache, s := setupTestAuditCache(t)
		defer s.Close()

		err := cache.SetLatestHash(ctx, "", 100)
		assert.Error(t, err)
	})
}

func TestSequenceOperations(t *testing.T) {
	ctx := context.Background()

	t.Run("sequence increment", func(t *testing.T) {
		cache, s := setupTestAuditCache(t)
		defer s.Close()

		// First increment
		seq1, err := cache.IncrementSequence(ctx)
		assert.NoError(t, err)
		assert.Equal(t, int64(1), seq1)

		// Second increment
		seq2, err := cache.IncrementSequence(ctx)
		assert.NoError(t, err)
		assert.Equal(t, int64(2), seq2)

		// Get current
		current, err := cache.GetSequenceNumber(ctx)
		assert.NoError(t, err)
		assert.Equal(t, int64(2), current)
	})

	t.Run("sequence gap tracking", func(t *testing.T) {
		cache, s := setupTestAuditCache(t)
		defer s.Close()

		// Track gaps
		err := cache.TrackSequenceGap(ctx, 10, 15)
		assert.NoError(t, err)

		err = cache.TrackSequenceGap(ctx, 20, 22)
		assert.NoError(t, err)

		// Get gaps
		gaps, err := cache.GetSequenceGaps(ctx, 10)
		assert.NoError(t, err)
		assert.Len(t, gaps, 2)
		assert.Equal(t, [2]int64{20, 22}, gaps[0]) // Most recent first
		assert.Equal(t, [2]int64{10, 15}, gaps[1])
	})
}

func TestCacheWarming(t *testing.T) {
	ctx := context.Background()

	t.Run("warm cache with events", func(t *testing.T) {
		cache, s := setupTestAuditCache(t)
		defer s.Close()

		// Create events
		events := make([]*audit.Event, 25)
		for i := 0; i < 25; i++ {
			event := createTestEvent(t)
			event.SequenceNum = int64(i + 1)
			events[i] = event
		}

		// Warm cache (should process in batches)
		err := cache.WarmCache(ctx, events)
		assert.NoError(t, err)

		// Verify some events are cached
		retrieved, err := cache.GetEvent(ctx, events[0].ID)
		assert.NoError(t, err)
		assert.NotNil(t, retrieved)
	})

	t.Run("warm cache with empty slice", func(t *testing.T) {
		cache, s := setupTestAuditCache(t)
		defer s.Close()

		err := cache.WarmCache(ctx, []*audit.Event{})
		assert.NoError(t, err)
	})
}

func TestCacheMetrics(t *testing.T) {
	ctx := context.Background()

	t.Run("track cache metrics", func(t *testing.T) {
		cache, s := setupTestAuditCache(t)
		defer s.Close()

		event := createTestEvent(t)

		// Generate some hits and misses
		cache.SetEvent(ctx, event)
		cache.GetEvent(ctx, event.ID)         // Hit
		cache.GetEvent(ctx, event.ID)         // Hit
		cache.GetEvent(ctx, uuid.New())       // Miss
		cache.GetEvent(ctx, uuid.New())       // Miss
		cache.GetEvent(ctx, uuid.New())       // Miss

		// Get stats
		stats, err := cache.GetCacheStats(ctx)
		assert.NoError(t, err)
		assert.Equal(t, int64(2), stats["hits"])
		assert.Equal(t, int64(3), stats["misses"])
		assert.Equal(t, float64(0.4), stats["hit_rate"])
	})
}

func TestConcurrentAccess(t *testing.T) {
	ctx := context.Background()

	t.Run("concurrent event operations", func(t *testing.T) {
		cache, s := setupTestAuditCache(t)
		defer s.Close()

		const concurrency = 10
		const eventsPerGoroutine = 10

		var wg sync.WaitGroup
		wg.Add(concurrency)

		// Concurrent writes and reads
		for i := 0; i < concurrency; i++ {
			go func(routineID int) {
				defer wg.Done()

				for j := 0; j < eventsPerGoroutine; j++ {
					event := createTestEvent(t)
					event.SequenceNum = int64(routineID*eventsPerGoroutine + j)

					// Set
					err := cache.SetEvent(ctx, event)
					assert.NoError(t, err)

					// Get
					retrieved, err := cache.GetEvent(ctx, event.ID)
					assert.NoError(t, err)
					assert.NotNil(t, retrieved)
				}
			}(i)
		}

		wg.Wait()
	})

	t.Run("concurrent sequence increment", func(t *testing.T) {
		cache, s := setupTestAuditCache(t)
		defer s.Close()

		const concurrency = 20
		sequences := make([]int64, concurrency)

		var wg sync.WaitGroup
		wg.Add(concurrency)

		for i := 0; i < concurrency; i++ {
			go func(idx int) {
				defer wg.Done()
				seq, err := cache.IncrementSequence(ctx)
				assert.NoError(t, err)
				sequences[idx] = seq
			}(i)
		}

		wg.Wait()

		// Check all sequences are unique
		seen := make(map[int64]bool)
		for _, seq := range sequences {
			assert.False(t, seen[seq], "duplicate sequence: %d", seq)
			seen[seq] = true
		}
	})
}

func TestPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	ctx := context.Background()
	cache, s := setupTestAuditCache(t)
	defer s.Close()

	t.Run("write latency < 5ms", func(t *testing.T) {
		event := createTestEvent(t)

		// Warm up
		cache.SetEvent(ctx, event)

		// Measure write latency
		iterations := 100
		start := time.Now()

		for i := 0; i < iterations; i++ {
			event := createTestEvent(t)
			err := cache.SetEvent(ctx, event)
			require.NoError(t, err)
		}

		elapsed := time.Since(start)
		avgLatency := elapsed / time.Duration(iterations)

		t.Logf("Average write latency: %v", avgLatency)
		assert.Less(t, avgLatency, 5*time.Millisecond, 
			"Write latency exceeds 5ms requirement")
	})

	t.Run("batch operations performance", func(t *testing.T) {
		// Create batch of events
		events := make([]*audit.Event, 100)
		for i := 0; i < 100; i++ {
			event := createTestEvent(t)
			event.SequenceNum = int64(i + 1)
			events[i] = event
		}

		// Measure batch set
		start := time.Now()
		err := cache.SetEvents(ctx, events)
		elapsed := time.Since(start)
		require.NoError(t, err)

		t.Logf("Batch set 100 events: %v", elapsed)
		assert.Less(t, elapsed, 50*time.Millisecond)

		// Measure batch get
		eventIDs := make([]uuid.UUID, len(events))
		for i, e := range events {
			eventIDs[i] = e.ID
		}

		start = time.Now()
		retrieved, err := cache.GetEvents(ctx, eventIDs)
		elapsed = time.Since(start)
		require.NoError(t, err)
		assert.Len(t, retrieved, 100)

		t.Logf("Batch get 100 events: %v", elapsed)
		assert.Less(t, elapsed, 20*time.Millisecond)
	})
}

func TestEdgeCases(t *testing.T) {
	ctx := context.Background()

	t.Run("nil event handling", func(t *testing.T) {
		cache, s := setupTestAuditCache(t)
		defer s.Close()

		err := cache.SetEvent(ctx, nil)
		assert.Error(t, err)
	})

	t.Run("invalid sequence range", func(t *testing.T) {
		cache, s := setupTestAuditCache(t)
		defer s.Close()

		chain, err := cache.GetHashChain(ctx, 10, 5)
		assert.Error(t, err)
		assert.Nil(t, chain)
	})

	t.Run("batch size limits", func(t *testing.T) {
		cache, s := setupTestAuditCache(t)
		defer s.Close()

		// Create more events than max batch size
		events := make([]*audit.Event, 20)
		for i := 0; i < 20; i++ {
			events[i] = createTestEvent(t)
		}

		// Should only process maxBatch (10)
		err := cache.SetEvents(ctx, events)
		assert.NoError(t, err)
	})

	t.Run("TTL with jitter", func(t *testing.T) {
		cache, s := setupTestAuditCache(t)
		defer s.Close()

		// Test jitter is applied
		ttl1 := cache.addJitter(EventCacheTTL)
		ttl2 := cache.addJitter(EventCacheTTL)

		// Should be different due to jitter
		assert.NotEqual(t, ttl1, ttl2)
		assert.Greater(t, ttl1, EventCacheTTL)
		assert.Greater(t, ttl2, EventCacheTTL)
	})
}

// Benchmark tests
func BenchmarkSetEvent(b *testing.B) {
	ctx := context.Background()
	cache, s := setupTestAuditCache(b)
	defer s.Close()

	event := createTestEvent(b)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.SetEvent(ctx, event)
	}
}

func BenchmarkGetEvent(b *testing.B) {
	ctx := context.Background()
	cache, s := setupTestAuditCache(b)
	defer s.Close()

	event := createTestEvent(b)
	cache.SetEvent(ctx, event)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.GetEvent(ctx, event.ID)
	}
}

func BenchmarkIncrementSequence(b *testing.B) {
	ctx := context.Background()
	cache, s := setupTestAuditCache(b)
	defer s.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.IncrementSequence(ctx)
	}
}

func BenchmarkBatchOperations(b *testing.B) {
	ctx := context.Background()
	cache, s := setupTestAuditCache(b)
	defer s.Close()

	// Create batch of 10 events
	events := make([]*audit.Event, 10)
	for i := 0; i < 10; i++ {
		events[i] = createTestEvent(b)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.SetEvents(ctx, events)
	}
}