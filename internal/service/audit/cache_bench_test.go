package audit

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/audit"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
)

// Cache performance benchmarks - validates cache efficiency and performance

// Mock cache implementation for benchmarking
type mockCache struct {
	hashResults     map[string]*audit.HashChainVerificationResult
	sequenceResults map[string]*audit.SequenceIntegrityResult
	events          map[string]*audit.Event
	hitDelay        time.Duration
	missDelay       time.Duration
	mu              sync.RWMutex
	hitCount        int64
	missCount       int64
}

func newMockCache(hitDelay, missDelay time.Duration) *mockCache {
	return &mockCache{
		hashResults:     make(map[string]*audit.HashChainVerificationResult),
		sequenceResults: make(map[string]*audit.SequenceIntegrityResult),
		events:          make(map[string]*audit.Event),
		hitDelay:        hitDelay,
		missDelay:       missDelay,
	}
}

func (m *mockCache) GetHashChainResult(ctx context.Context, start, end values.SequenceNumber) (*audit.HashChainVerificationResult, error) {
	key := fmt.Sprintf("hash:%d-%d", start.Value(), end.Value())
	
	m.mu.RLock()
	result, exists := m.hashResults[key]
	m.mu.RUnlock()

	if exists {
		m.hitCount++
		if m.hitDelay > 0 {
			time.Sleep(m.hitDelay)
		}
		return result, nil
	}

	m.missCount++
	if m.missDelay > 0 {
		time.Sleep(m.missDelay)
	}
	return nil, fmt.Errorf("cache miss")
}

func (m *mockCache) SetHashChainResult(ctx context.Context, start, end values.SequenceNumber, result *audit.HashChainVerificationResult, ttl time.Duration) error {
	key := fmt.Sprintf("hash:%d-%d", start.Value(), end.Value())
	
	m.mu.Lock()
	m.hashResults[key] = result
	m.mu.Unlock()
	
	return nil
}

func (m *mockCache) InvalidateHashChainRange(ctx context.Context, start, end values.SequenceNumber) error {
	// Invalidate all overlapping ranges
	m.mu.Lock()
	defer m.mu.Unlock()
	
	for key := range m.hashResults {
		// Simple invalidation logic for benchmark
		delete(m.hashResults, key)
	}
	
	return nil
}

func (m *mockCache) GetSequenceResult(ctx context.Context, criteria audit.SequenceIntegrityCriteria) (*audit.SequenceIntegrityResult, error) {
	key := fmt.Sprintf("seq:%s", criteria.String()) // Simplified key generation
	
	m.mu.RLock()
	result, exists := m.sequenceResults[key]
	m.mu.RUnlock()

	if exists {
		m.hitCount++
		if m.hitDelay > 0 {
			time.Sleep(m.hitDelay)
		}
		return result, nil
	}

	m.missCount++
	if m.missDelay > 0 {
		time.Sleep(m.missDelay)
	}
	return nil, fmt.Errorf("cache miss")
}

func (m *mockCache) SetSequenceResult(ctx context.Context, criteria audit.SequenceIntegrityCriteria, result *audit.SequenceIntegrityResult, ttl time.Duration) error {
	key := fmt.Sprintf("seq:%s", criteria.String())
	
	m.mu.Lock()
	m.sequenceResults[key] = result
	m.mu.Unlock()
	
	return nil
}

func (m *mockCache) GetEvent(ctx context.Context, eventID uuid.UUID) (*audit.Event, error) {
	key := eventID.String()
	
	m.mu.RLock()
	event, exists := m.events[key]
	m.mu.RUnlock()

	if exists {
		m.hitCount++
		if m.hitDelay > 0 {
			time.Sleep(m.hitDelay)
		}
		return event, nil
	}

	m.missCount++
	if m.missDelay > 0 {
		time.Sleep(m.missDelay)
	}
	return nil, fmt.Errorf("cache miss")
}

func (m *mockCache) SetEvent(ctx context.Context, event *audit.Event, ttl time.Duration) error {
	key := event.ID.String()
	
	m.mu.Lock()
	m.events[key] = event
	m.mu.Unlock()
	
	return nil
}

func (m *mockCache) GetStats() (int64, int64) {
	return m.hitCount, m.missCount
}

func (m *mockCache) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.hashResults = make(map[string]*audit.HashChainVerificationResult)
	m.sequenceResults = make(map[string]*audit.SequenceIntegrityResult)
	m.events = make(map[string]*audit.Event)
	m.hitCount = 0
	m.missCount = 0
}

// Cache hit ratio benchmark - validates cache efficiency
func BenchmarkCache_HitRatio(b *testing.B) {
	scenarios := []struct {
		name     string
		hitDelay time.Duration
		missDelay time.Duration
	}{
		{"fast_cache", time.Microsecond * 10, time.Millisecond * 5},
		{"slow_cache", time.Microsecond * 100, time.Millisecond * 20},
		{"network_cache", time.Millisecond * 2, time.Millisecond * 50},
	}

	for _, scenario := range scenarios {
		b.Run(scenario.name, func(b *testing.B) {
			cache := newMockCache(scenario.hitDelay, scenario.missDelay)
			ctx := context.Background()

			// Pre-populate cache with some results
			for i := 0; i < 1000; i++ {
				start := values.NewSequenceNumber(int64(i * 100))
				end := values.NewSequenceNumber(int64((i + 1) * 100 - 1))
				
				result := &audit.HashChainVerificationResult{
					StartSequence:  start,
					EndSequence:    end,
					TotalEvents:    100,
					VerifiedEvents: 100,
					IsValid:        true,
					IntegrityScore: 1.0,
				}
				
				cache.SetHashChainResult(ctx, start, end, result, time.Hour)
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				// 80% cache hits, 20% misses
				var start, end values.SequenceNumber
				if i%5 == 0 {
					// Cache miss - new range
					start = values.NewSequenceNumber(int64(1000 + i))
					end = values.NewSequenceNumber(int64(1000 + i + 99))
				} else {
					// Cache hit - existing range
					idx := rand.Intn(1000)
					start = values.NewSequenceNumber(int64(idx * 100))
					end = values.NewSequenceNumber(int64((idx + 1) * 100 - 1))
				}

				_, err := cache.GetHashChainResult(ctx, start, end)
				if err != nil && i%5 != 0 {
					// Should only error on cache miss attempts
					b.Logf("Unexpected cache miss: %v", err)
				}
			}

			b.StopTimer()

			hits, misses := cache.GetStats()
			total := hits + misses
			hitRatio := float64(hits) / float64(total) * 100

			b.ReportMetric(hitRatio, "hit_ratio_percent")
			b.ReportMetric(float64(hits), "cache_hits")
			b.ReportMetric(float64(misses), "cache_misses")

			// Validate expected hit ratio
			expectedHitRatio := 80.0 // Expecting ~80% hit rate
			if hitRatio < expectedHitRatio-5 {
				b.Logf("WARNING: Hit ratio %.1f%% below expected %.1f%%", hitRatio, expectedHitRatio)
			}

			b.Logf("Cache performance: %.1f%% hit ratio, %d hits, %d misses", hitRatio, hits, misses)
		})
	}
}

// Cache write performance benchmark
func BenchmarkCache_WritePerformance(b *testing.B) {
	cache := newMockCache(0, 0) // No artificial delays for write benchmark
	ctx := context.Background()

	// Test different data sizes
	dataSizes := []int{100, 1000, 10000}

	for _, dataSize := range dataSizes {
		b.Run(fmt.Sprintf("dataset_%dk", dataSize/1000), func(b *testing.B) {
			// Pre-generate test data
			results := make([]*audit.HashChainVerificationResult, dataSize)
			starts := make([]values.SequenceNumber, dataSize)
			ends := make([]values.SequenceNumber, dataSize)

			for i := 0; i < dataSize; i++ {
				starts[i] = values.NewSequenceNumber(int64(i * 100))
				ends[i] = values.NewSequenceNumber(int64((i + 1) * 100 - 1))
				
				results[i] = &audit.HashChainVerificationResult{
					StartSequence:  starts[i],
					EndSequence:    ends[i],
					TotalEvents:    100,
					VerifiedEvents: 100,
					IsValid:        true,
					IntegrityScore: 1.0,
				}
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				idx := i % dataSize
				
				err := cache.SetHashChainResult(ctx, starts[idx], ends[idx], results[idx], time.Hour)
				if err != nil {
					b.Fatalf("Cache write failed: %v", err)
				}
			}

			b.StopTimer()
			
			writesPerSec := float64(b.N) / b.Elapsed().Seconds()
			b.ReportMetric(writesPerSec, "writes/sec")

			// Cache writes should be fast
			minWritesPerSec := 10000.0
			if writesPerSec < minWritesPerSec {
				b.Logf("WARNING: Write performance %.0f writes/sec below %.0f minimum",
					writesPerSec, minWritesPerSec)
			}
		})
	}
}

// Cache read performance under different load conditions
func BenchmarkCache_ReadPerformance(b *testing.B) {
	cache := newMockCache(time.Microsecond*10, time.Millisecond*5)
	ctx := context.Background()

	// Pre-populate cache
	cacheSize := 10000
	for i := 0; i < cacheSize; i++ {
		start := values.NewSequenceNumber(int64(i * 100))
		end := values.NewSequenceNumber(int64((i + 1) * 100 - 1))
		
		result := &audit.HashChainVerificationResult{
			StartSequence:  start,
			EndSequence:    end,
			TotalEvents:    100,
			VerifiedEvents: 100,
			IsValid:        true,
			IntegrityScore: 1.0,
		}
		
		cache.SetHashChainResult(ctx, start, end, result, time.Hour)
	}

	// Test different read patterns
	patterns := []struct {
		name        string
		description string
		generator   func(i int) (values.SequenceNumber, values.SequenceNumber)
	}{
		{
			name: "sequential_reads",
			description: "Sequential access pattern",
			generator: func(i int) (values.SequenceNumber, values.SequenceNumber) {
				idx := i % cacheSize
				start := values.NewSequenceNumber(int64(idx * 100))
				end := values.NewSequenceNumber(int64((idx + 1) * 100 - 1))
				return start, end
			},
		},
		{
			name: "random_reads",
			description: "Random access pattern",
			generator: func(i int) (values.SequenceNumber, values.SequenceNumber) {
				idx := rand.Intn(cacheSize)
				start := values.NewSequenceNumber(int64(idx * 100))
				end := values.NewSequenceNumber(int64((idx + 1) * 100 - 1))
				return start, end
			},
		},
		{
			name: "hot_spot_reads",
			description: "80/20 hot spot pattern",
			generator: func(i int) (values.SequenceNumber, values.SequenceNumber) {
				var idx int
				if i%5 < 4 {
					// 80% access to first 20% of data
					idx = rand.Intn(cacheSize / 5)
				} else {
					// 20% access to remaining 80% of data
					idx = cacheSize/5 + rand.Intn(cacheSize*4/5)
				}
				start := values.NewSequenceNumber(int64(idx * 100))
				end := values.NewSequenceNumber(int64((idx + 1) * 100 - 1))
				return start, end
			},
		},
	}

	for _, pattern := range patterns {
		b.Run(pattern.name, func(b *testing.B) {
			cache.Clear()
			
			// Re-populate for this test
			for i := 0; i < cacheSize; i++ {
				start := values.NewSequenceNumber(int64(i * 100))
				end := values.NewSequenceNumber(int64((i + 1) * 100 - 1))
				
				result := &audit.HashChainVerificationResult{
					StartSequence:  start,
					EndSequence:    end,
					TotalEvents:    100,
					VerifiedEvents: 100,
					IsValid:        true,
					IntegrityScore: 1.0,
				}
				
				cache.SetHashChainResult(ctx, start, end, result, time.Hour)
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				start, end := pattern.generator(i)
				
				_, err := cache.GetHashChainResult(ctx, start, end)
				if err != nil {
					b.Logf("Cache read failed: %v", err)
				}
			}

			b.StopTimer()

			hits, misses := cache.GetStats()
			total := hits + misses
			hitRatio := float64(hits) / float64(total) * 100
			readsPerSec := float64(b.N) / b.Elapsed().Seconds()

			b.ReportMetric(hitRatio, "hit_ratio_percent")
			b.ReportMetric(readsPerSec, "reads/sec")

			b.Logf("%s: %.1f%% hit ratio, %.0f reads/sec", pattern.description, hitRatio, readsPerSec)
		})
	}
}

// Concurrent cache access benchmark
func BenchmarkCache_ConcurrentAccess(b *testing.B) {
	cache := newMockCache(time.Microsecond*5, time.Millisecond*2)
	ctx := context.Background()

	// Pre-populate cache
	cacheSize := 5000
	for i := 0; i < cacheSize; i++ {
		start := values.NewSequenceNumber(int64(i * 100))
		end := values.NewSequenceNumber(int64((i + 1) * 100 - 1))
		
		result := &audit.HashChainVerificationResult{
			StartSequence:  start,
			EndSequence:    end,
			TotalEvents:    100,
			VerifiedEvents: 100,
			IsValid:        true,
			IntegrityScore: 1.0,
		}
		
		cache.SetHashChainResult(ctx, start, end, result, time.Hour)
	}

	concurrencyLevels := []int{1, 5, 10, 20, 50}

	for _, concurrency := range concurrencyLevels {
		b.Run(fmt.Sprintf("concurrent_%d", concurrency), func(b *testing.B) {
			cache.Clear()
			
			// Re-populate
			for i := 0; i < cacheSize; i++ {
				start := values.NewSequenceNumber(int64(i * 100))
				end := values.NewSequenceNumber(int64((i + 1) * 100 - 1))
				
				result := &audit.HashChainVerificationResult{
					StartSequence:  start,
					EndSequence:    end,
					TotalEvents:    100,
					VerifiedEvents: 100,
					IsValid:        true,
					IntegrityScore: 1.0,
				}
				
				cache.SetHashChainResult(ctx, start, end, result, time.Hour)
			}

			opsPerWorker := b.N / concurrency
			if opsPerWorker < 1 {
				opsPerWorker = 1
			}

			var wg sync.WaitGroup
			totalOps := concurrency * opsPerWorker

			start := time.Now()
			b.ResetTimer()
			b.ReportAllocs()

			for w := 0; w < concurrency; w++ {
				wg.Add(1)
				go func(workerID int) {
					defer wg.Done()

					for i := 0; i < opsPerWorker; i++ {
						// Mix of reads and writes
						if i%10 < 8 {
							// 80% reads
							idx := rand.Intn(cacheSize)
							start := values.NewSequenceNumber(int64(idx * 100))
							end := values.NewSequenceNumber(int64((idx + 1) * 100 - 1))
							
							cache.GetHashChainResult(ctx, start, end)
						} else {
							// 20% writes
							idx := cacheSize + workerID*opsPerWorker + i
							start := values.NewSequenceNumber(int64(idx * 100))
							end := values.NewSequenceNumber(int64((idx + 1) * 100 - 1))
							
							result := &audit.HashChainVerificationResult{
								StartSequence:  start,
								EndSequence:    end,
								TotalEvents:    100,
								VerifiedEvents: 100,
								IsValid:        true,
								IntegrityScore: 1.0,
							}
							
							cache.SetHashChainResult(ctx, start, end, result, time.Hour)
						}
					}
				}(w)
			}

			wg.Wait()
			b.StopTimer()

			totalTime := time.Since(start)
			opsPerSec := float64(totalOps) / totalTime.Seconds()

			b.ReportMetric(opsPerSec, "ops/sec")
			b.ReportMetric(float64(concurrency), "workers")

			// Concurrent performance should scale reasonably
			expectedMinOpsPerSec := float64(concurrency) * 1000 // 1K ops/sec per worker minimum
			if opsPerSec < expectedMinOpsPerSec {
				b.Logf("WARNING: Concurrent performance %.0f ops/sec below expected %.0f for %d workers",
					opsPerSec, expectedMinOpsPerSec, concurrency)
			}
		})
	}
}

// Cache invalidation performance benchmark
func BenchmarkCache_InvalidationPerformance(b *testing.B) {
	cache := newMockCache(0, 0)
	ctx := context.Background()

	invalidationSizes := []int{100, 1000, 10000}

	for _, invSize := range invalidationSizes {
		b.Run(fmt.Sprintf("invalidate_%dk", invSize/1000), func(b *testing.B) {
			// Pre-populate cache before each benchmark iteration
			for i := 0; i < b.N; i++ {
				cache.Clear()
				
				// Populate cache
				for j := 0; j < invSize; j++ {
					start := values.NewSequenceNumber(int64(j * 100))
					end := values.NewSequenceNumber(int64((j + 1) * 100 - 1))
					
					result := &audit.HashChainVerificationResult{
						StartSequence:  start,
						EndSequence:    end,
						TotalEvents:    100,
						VerifiedEvents: 100,
						IsValid:        true,
						IntegrityScore: 1.0,
					}
					
					cache.SetHashChainResult(ctx, start, end, result, time.Hour)
				}

				// Measure invalidation time
				start := values.NewSequenceNumber(1)
				end := values.NewSequenceNumber(int64(invSize * 100))

				invalidateStart := time.Now()
				err := cache.InvalidateHashChainRange(ctx, start, end)
				invalidateTime := time.Since(invalidateStart)

				if err != nil {
					b.Fatalf("Cache invalidation failed: %v", err)
				}

				// Invalidation should be fast
				maxExpectedTime := time.Duration(invSize/1000) * time.Millisecond
				if maxExpectedTime < time.Millisecond {
					maxExpectedTime = time.Millisecond
				}

				if invalidateTime > maxExpectedTime {
					b.Logf("WARNING: Invalidation of %d entries took %v, expected < %v",
						invSize, invalidateTime, maxExpectedTime)
				}
			}

			invalidationsPerSec := float64(b.N) / b.Elapsed().Seconds()
			b.ReportMetric(invalidationsPerSec, "invalidations/sec")
			b.ReportMetric(float64(invSize), "entries_invalidated")
		})
	}
}

// Cache memory efficiency benchmark
func BenchmarkCache_MemoryEfficiency(b *testing.B) {
	cache := newMockCache(0, 0)
	ctx := context.Background()

	entrySizes := []int{1000, 10000, 100000}

	for _, entrySize := range entrySizes {
		b.Run(fmt.Sprintf("entries_%dk", entrySize/1000), func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				cache.Clear()

				// Add entries to cache
				for j := 0; j < entrySize; j++ {
					start := values.NewSequenceNumber(int64(j * 100))
					end := values.NewSequenceNumber(int64((j + 1) * 100 - 1))
					
					result := &audit.HashChainVerificationResult{
						StartSequence:   start,
						EndSequence:     end,
						TotalEvents:     100,
						VerifiedEvents:  100,
						IsValid:         true,
						IntegrityScore:  1.0,
						StartTime:       time.Now(),
						EndTime:         time.Now().Add(time.Millisecond),
					}
					
					err := cache.SetHashChainResult(ctx, start, end, result, time.Hour)
					if err != nil {
						b.Fatalf("Failed to set cache entry: %v", err)
					}
				}

				// Verify cache contains expected entries
				testStart := values.NewSequenceNumber(100)
				testEnd := values.NewSequenceNumber(199)
				_, err := cache.GetHashChainResult(ctx, testStart, testEnd)
				if err != nil {
					b.Logf("Cache verification failed: %v", err)
				}
			}

			b.ReportMetric(float64(entrySize), "cache_entries")
		})
	}
}

// Cache TTL and expiration benchmark
func BenchmarkCache_TTLPerformance(b *testing.B) {
	cache := newMockCache(0, 0)
	ctx := context.Background()

	// Test different TTL scenarios
	ttlScenarios := []struct {
		name string
		ttl  time.Duration
	}{
		{"short_ttl", time.Millisecond * 100},
		{"medium_ttl", time.Second * 5},
		{"long_ttl", time.Minute * 30},
	}

	for _, scenario := range ttlScenarios {
		b.Run(scenario.name, func(b *testing.B) {
			cache.Clear()

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				start := values.NewSequenceNumber(int64(i * 100))
				end := values.NewSequenceNumber(int64((i + 1) * 100 - 1))
				
				result := &audit.HashChainVerificationResult{
					StartSequence:  start,
					EndSequence:    end,
					TotalEvents:    100,
					VerifiedEvents: 100,
					IsValid:        true,
					IntegrityScore: 1.0,
				}

				// Set with TTL
				err := cache.SetHashChainResult(ctx, start, end, result, scenario.ttl)
				if err != nil {
					b.Fatalf("Failed to set cache entry with TTL: %v", err)
				}

				// Immediate read should succeed
				_, err = cache.GetHashChainResult(ctx, start, end)
				if err != nil {
					b.Fatalf("Failed to read cache entry: %v", err)
				}
			}

			b.ReportMetric(float64(scenario.ttl.Milliseconds()), "ttl_ms")
		})
	}
}

// Cache performance under different event sizes
func BenchmarkCache_EventSizePerformance(b *testing.B) {
	cache := newMockCache(time.Microsecond*5, 0)
	ctx := context.Background()

	// Test different event data sizes
	eventSizes := []struct {
		name     string
		metadata int // Number of metadata fields
	}{
		{"small_events", 5},
		{"medium_events", 50},
		{"large_events", 500},
	}

	for _, eventSize := range eventSizes {
		b.Run(eventSize.name, func(b *testing.B) {
			cache.Clear()

			// Generate events with different metadata sizes
			events := make([]*audit.Event, 1000)
			for i := 0; i < 1000; i++ {
				metadata := make(map[string]interface{})
				for j := 0; j < eventSize.metadata; j++ {
					metadata[fmt.Sprintf("key_%d", j)] = fmt.Sprintf("value_%d_%d", i, j)
				}

				events[i] = &audit.Event{
					ID:          uuid.New(),
					Type:        audit.EventTypeDataAccess,
					ActorID:     fmt.Sprintf("actor_%d", i),
					Action:      "test_action",
					Timestamp:   time.Now(),
					SequenceNum: int64(i),
					Metadata:    metadata,
				}
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				event := events[i%len(events)]
				
				// Cache the event
				err := cache.SetEvent(ctx, event, time.Hour)
				if err != nil {
					b.Fatalf("Failed to cache event: %v", err)
				}

				// Read it back
				_, err = cache.GetEvent(ctx, event.ID)
				if err != nil {
					b.Fatalf("Failed to read cached event: %v", err)
				}
			}

			b.ReportMetric(float64(eventSize.metadata), "metadata_fields")
		})
	}
}

// Cache performance regression test
func BenchmarkCache_PerformanceRegression(b *testing.B) {
	// Standard test conditions for regression detection
	cache := newMockCache(time.Microsecond*10, time.Millisecond*5)
	ctx := context.Background()

	// Pre-populate with standard dataset
	cacheSize := 10000
	for i := 0; i < cacheSize; i++ {
		start := values.NewSequenceNumber(int64(i * 100))
		end := values.NewSequenceNumber(int64((i + 1) * 100 - 1))
		
		result := &audit.HashChainVerificationResult{
			StartSequence:  start,
			EndSequence:    end,
			TotalEvents:    100,
			VerifiedEvents: 100,
			IsValid:        true,
			IntegrityScore: 1.0,
		}
		
		cache.SetHashChainResult(ctx, start, end, result, time.Hour)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Standard access pattern: 90% reads, 10% writes
		if i%10 < 9 {
			// Read existing entry
			idx := i % cacheSize
			start := values.NewSequenceNumber(int64(idx * 100))
			end := values.NewSequenceNumber(int64((idx + 1) * 100 - 1))
			
			_, err := cache.GetHashChainResult(ctx, start, end)
			if err != nil {
				b.Logf("Cache read failed: %v", err)
			}
		} else {
			// Write new entry
			idx := cacheSize + i
			start := values.NewSequenceNumber(int64(idx * 100))
			end := values.NewSequenceNumber(int64((idx + 1) * 100 - 1))
			
			result := &audit.HashChainVerificationResult{
				StartSequence:  start,
				EndSequence:    end,
				TotalEvents:    100,
				VerifiedEvents: 100,
				IsValid:        true,
				IntegrityScore: 1.0,
			}
			
			cache.SetHashChainResult(ctx, start, end, result, time.Hour)
		}
	}

	opsPerSec := float64(b.N) / b.Elapsed().Seconds()
	b.ReportMetric(opsPerSec, "ops/sec")

	hits, misses := cache.GetStats()
	total := hits + misses
	if total > 0 {
		hitRatio := float64(hits) / float64(total) * 100
		b.ReportMetric(hitRatio, "hit_ratio_percent")
	}

	// Performance baseline for regression detection
	minOpsPerSec := 50000.0 // 50K ops/sec minimum
	if opsPerSec < minOpsPerSec {
		b.Errorf("Cache performance regression: %.0f ops/sec below minimum %.0f ops/sec",
			opsPerSec, minOpsPerSec)
	}
}