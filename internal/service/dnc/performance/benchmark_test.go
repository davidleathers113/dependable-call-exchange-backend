package performance

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"go.uber.org/zap"
)

// BenchmarkPerformanceOptimizer tests the main optimizer performance
func BenchmarkPerformanceOptimizer_OptimizeQuery(b *testing.B) {
	logger, _ := zap.NewDevelopment()
	config := &OptimizerConfig{
		MaxConnections:     50,
		WorkerPoolSize:     25,
		L1CacheSize:        10000,
		BloomFilterEnabled: true,
	}
	
	optimizer := NewPerformanceOptimizer(config, nil, logger)
	ctx := context.Background()
	
	// Start optimizer
	err := optimizer.Start(ctx)
	if err != nil {
		b.Fatalf("Failed to start optimizer: %v", err)
	}
	defer optimizer.Stop(ctx)
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			phoneNumber, _ := values.NewPhoneNumber("+1555" + fmt.Sprintf("%07d", rand.Intn(10000000)))
			_, err := optimizer.OptimizeQuery(ctx, phoneNumber)
			if err != nil {
				b.Errorf("OptimizeQuery failed: %v", err)
			}
		}
	})
}

// BenchmarkL1Cache tests L1 cache performance
func BenchmarkL1Cache_Get(b *testing.B) {
	logger, _ := zap.NewDevelopment()
	config := &L1CacheConfig{
		Size: 10000,
		TTL:  time.Minute,
	}
	
	cache := NewL1Cache(config, logger)
	defer cache.Stop()
	
	// Pre-populate cache
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("phone_%d", i)
		cache.Set(key, fmt.Sprintf("result_%d", i), time.Minute)
	}
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			key := fmt.Sprintf("phone_%d", rand.Intn(1000))
			_, _ = cache.Get(key)
		}
	})
}

func BenchmarkL1Cache_Set(b *testing.B) {
	logger, _ := zap.NewDevelopment()
	config := &L1CacheConfig{
		Size: 10000,
		TTL:  time.Minute,
	}
	
	cache := NewL1Cache(config, logger)
	defer cache.Stop()
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("phone_%d", i)
			value := fmt.Sprintf("result_%d", i)
			cache.Set(key, value, time.Minute)
			i++
		}
	})
}

// BenchmarkBloomFilter tests bloom filter performance
func BenchmarkBloomFilter_Add(b *testing.B) {
	logger, _ := zap.NewDevelopment()
	config := &BloomFilterConfig{
		Size:   1000000,
		Hashes: 7,
	}
	
	bloomFilter := NewBloomFilter(config, logger)
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			phoneNumber := "+1555" + fmt.Sprintf("%07d", i)
			bloomFilter.Add(phoneNumber)
			i++
		}
	})
}

func BenchmarkBloomFilter_MayContain(b *testing.B) {
	logger, _ := zap.NewDevelopment()
	config := &BloomFilterConfig{
		Size:   1000000,
		Hashes: 7,
	}
	
	bloomFilter := NewBloomFilter(config, logger)
	
	// Pre-populate with some numbers
	for i := 0; i < 10000; i++ {
		phoneNumber := "+1555" + fmt.Sprintf("%07d", i)
		bloomFilter.Add(phoneNumber)
	}
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			phoneNumber := "+1555" + fmt.Sprintf("%07d", rand.Intn(20000))
			_ = bloomFilter.MayContain(phoneNumber)
		}
	})
}

// BenchmarkConnectionPool tests connection pool performance
func BenchmarkConnectionPool_GetConnection(b *testing.B) {
	logger, _ := zap.NewDevelopment()
	config := &ConnectionPoolConfig{
		MaxConnections:     50,
		MinIdleConnections: 10,
		ConnectionTimeout:  time.Second,
		PrewarmEnabled:     true,
		DSN:               "postgres://test:test@localhost/test",
	}
	
	pool := NewConnectionPool(config, logger)
	ctx := context.Background()
	
	// Note: This benchmark requires a test database
	// In a real scenario, you would use testcontainers
	b.Skip("Requires test database setup")
	
	err := pool.Start(ctx)
	if err != nil {
		b.Fatalf("Failed to start pool: %v", err)
	}
	defer pool.Stop(ctx)
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			conn, err := pool.GetConnection(ctx)
			if err != nil {
				b.Errorf("GetConnection failed: %v", err)
				continue
			}
			pool.ReturnConnection(conn)
		}
	})
}

// BenchmarkWorkerPool tests worker pool performance
func BenchmarkWorkerPool_SubmitTask(b *testing.B) {
	logger, _ := zap.NewDevelopment()
	config := &WorkerPoolConfig{
		PoolSize:  10,
		QueueSize: 1000,
	}
	
	pool := NewWorkerPool(config, logger)
	ctx := context.Background()
	
	err := pool.Start(ctx)
	if err != nil {
		b.Fatalf("Failed to start worker pool: %v", err)
	}
	defer pool.Stop(ctx)
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			task := Task{
				ID:      strconv.Itoa(i),
				Type:    TaskTypeDNCQuery,
				Context: ctx,
			}
			err := pool.SubmitTask(task)
			if err != nil {
				b.Errorf("SubmitTask failed: %v", err)
			}
			i++
		}
	})
}

// BenchmarkLatencyMonitor tests latency monitoring performance
func BenchmarkLatencyMonitor_RecordLatency(b *testing.B) {
	logger, _ := zap.NewDevelopment()
	config := &MonitorConfig{
		BufferSize: 10000,
	}
	
	monitor := NewLatencyMonitor(config, logger)
	ctx := context.Background()
	
	err := monitor.Start(ctx)
	if err != nil {
		b.Fatalf("Failed to start monitor: %v", err)
	}
	defer monitor.Stop(ctx)
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			duration := time.Duration(rand.Intn(10)) * time.Millisecond
			cacheHit := rand.Intn(2) == 1
			monitor.RecordLatency(OperationDNCQuery, duration, cacheHit)
		}
	})
}

// BenchmarkMemoryPool tests memory pool performance
func BenchmarkMemoryPool_GetPut(b *testing.B) {
	logger, _ := zap.NewDevelopment()
	config := &MemoryPoolConfig{
		PoolSize:  1000,
		BlockSize: 4096,
	}
	
	pool := NewMemoryPool(config, logger)
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			block := pool.Get()
			// Simulate some work with the block
			block.Data[0] = byte(rand.Intn(256))
			pool.Put(block)
		}
	})
}

// BenchmarkConcurrentCacheOperations tests cache under high concurrency
func BenchmarkConcurrentCacheOperations(b *testing.B) {
	logger, _ := zap.NewDevelopment()
	config := &L1CacheConfig{
		Size: 10000,
		TTL:  time.Minute,
	}
	
	cache := NewL1Cache(config, logger)
	defer cache.Stop()
	
	// Pre-populate cache
	for i := 0; i < 5000; i++ {
		key := fmt.Sprintf("phone_%d", i)
		cache.Set(key, fmt.Sprintf("result_%d", i), time.Minute)
	}
	
	b.ResetTimer()
	
	var wg sync.WaitGroup
	concurrency := 100
	
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(routineID int) {
			defer wg.Done()
			
			for j := 0; j < b.N/concurrency; j++ {
				if rand.Intn(2) == 0 {
					// Read operation
					key := fmt.Sprintf("phone_%d", rand.Intn(10000))
					_, _ = cache.Get(key)
				} else {
					// Write operation
					key := fmt.Sprintf("phone_%d_%d", routineID, j)
					value := fmt.Sprintf("result_%d_%d", routineID, j)
					cache.Set(key, value, time.Minute)
				}
			}
		}(i)
	}
	
	wg.Wait()
}

// BenchmarkOptimizationLatency measures optimization overhead
func BenchmarkOptimizationLatency(b *testing.B) {
	measurements := make([]time.Duration, b.N)
	
	logger, _ := zap.NewDevelopment()
	config := &OptimizerConfig{
		MaxConnections:     50,
		WorkerPoolSize:     25,
		L1CacheSize:        10000,
		BloomFilterEnabled: true,
	}
	
	optimizer := NewPerformanceOptimizer(config, nil, logger)
	ctx := context.Background()
	
	err := optimizer.Start(ctx)
	if err != nil {
		b.Fatalf("Failed to start optimizer: %v", err)
	}
	defer optimizer.Stop(ctx)
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		start := time.Now()
		
		phoneNumber, _ := values.NewPhoneNumber("+1555" + fmt.Sprintf("%07d", rand.Intn(10000000)))
		_, err := optimizer.OptimizeQuery(ctx, phoneNumber)
		if err != nil {
			b.Errorf("OptimizeQuery failed: %v", err)
		}
		
		measurements[i] = time.Since(start)
	}
	
	// Calculate percentiles
	sortedMeasurements := make([]time.Duration, len(measurements))
	copy(sortedMeasurements, measurements)
	
	// Simple bubble sort for small arrays
	for i := 0; i < len(sortedMeasurements); i++ {
		for j := i + 1; j < len(sortedMeasurements); j++ {
			if sortedMeasurements[i] > sortedMeasurements[j] {
				sortedMeasurements[i], sortedMeasurements[j] = sortedMeasurements[j], sortedMeasurements[i]
			}
		}
	}
	
	p50 := sortedMeasurements[len(sortedMeasurements)*50/100]
	p95 := sortedMeasurements[len(sortedMeasurements)*95/100]
	p99 := sortedMeasurements[len(sortedMeasurements)*99/100]
	
	b.Logf("Optimization latency P50: %v, P95: %v, P99: %v", p50, p95, p99)
	
	// Verify sub-millisecond P99 target
	if p99 > time.Millisecond {
		b.Errorf("P99 latency %v exceeds 1ms target", p99)
	}
}

// BenchmarkThroughputStress tests throughput under stress
func BenchmarkThroughputStress(b *testing.B) {
	logger, _ := zap.NewDevelopment()
	config := &OptimizerConfig{
		MaxConnections:     100,
		WorkerPoolSize:     50,
		L1CacheSize:        50000,
		BloomFilterEnabled: true,
	}
	
	optimizer := NewPerformanceOptimizer(config, nil, logger)
	ctx := context.Background()
	
	err := optimizer.Start(ctx)
	if err != nil {
		b.Fatalf("Failed to start optimizer: %v", err)
	}
	defer optimizer.Stop(ctx)
	
	concurrency := 100
	queriesPerWorker := b.N / concurrency
	
	var wg sync.WaitGroup
	start := time.Now()
	
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			
			for j := 0; j < queriesPerWorker; j++ {
				phoneNumber, _ := values.NewPhoneNumber("+1555" + fmt.Sprintf("%07d", rand.Intn(10000000)))
				_, err := optimizer.OptimizeQuery(ctx, phoneNumber)
				if err != nil {
					b.Errorf("OptimizeQuery failed: %v", err)
				}
			}
		}(i)
	}
	
	wg.Wait()
	duration := time.Since(start)
	
	throughput := float64(b.N) / duration.Seconds()
	b.Logf("Throughput: %.0f queries/second", throughput)
	
	// Verify 100K queries/second target
	if throughput < 100000 {
		b.Errorf("Throughput %.0f/sec below 100K target", throughput)
	}
}

// BenchmarkCacheHitRatio measures cache effectiveness
func BenchmarkCacheHitRatio(b *testing.B) {
	logger, _ := zap.NewDevelopment()
	config := &L1CacheConfig{
		Size: 10000,
		TTL:  time.Minute,
	}
	
	cache := NewL1Cache(config, logger)
	defer cache.Stop()
	
	// Simulate realistic phone number distribution
	phoneNumbers := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		phoneNumbers[i] = "+1555" + fmt.Sprintf("%07d", i)
	}
	
	// Pre-populate cache with 80% of numbers
	for i := 0; i < 800; i++ {
		cache.Set(phoneNumbers[i], fmt.Sprintf("result_%d", i), time.Minute)
	}
	
	var hits, misses int
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		phoneNumber := phoneNumbers[rand.Intn(1000)]
		if _, found := cache.Get(phoneNumber); found {
			hits++
		} else {
			misses++
		}
	}
	
	hitRatio := float64(hits) / float64(hits+misses) * 100
	b.Logf("Cache hit ratio: %.2f%%", hitRatio)
	
	// Expect reasonable hit ratio given the test setup
	if hitRatio < 75 {
		b.Errorf("Cache hit ratio %.2f%% below expected 75%%", hitRatio)
	}
}

// BenchmarkBloomFilterFalsePositiveRate measures bloom filter effectiveness
func BenchmarkBloomFilterFalsePositiveRate(b *testing.B) {
	logger, _ := zap.NewDevelopment()
	config := &BloomFilterConfig{
		Size:   1000000,
		Hashes: 7,
	}
	
	bloomFilter := NewBloomFilter(config, logger)
	
	// Add 100K known phone numbers
	knownNumbers := make(map[string]bool)
	for i := 0; i < 100000; i++ {
		phoneNumber := "+1555" + fmt.Sprintf("%07d", i)
		bloomFilter.Add(phoneNumber)
		knownNumbers[phoneNumber] = true
	}
	
	var falsePositives, trueNegatives int
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		// Test with unknown numbers
		phoneNumber := "+1666" + fmt.Sprintf("%07d", rand.Intn(1000000))
		
		if bloomFilter.MayContain(phoneNumber) {
			if !knownNumbers[phoneNumber] {
				falsePositives++
			}
		} else {
			trueNegatives++
		}
	}
	
	total := falsePositives + trueNegatives
	if total > 0 {
		falsePositiveRate := float64(falsePositives) / float64(total) * 100
		b.Logf("False positive rate: %.2f%%", falsePositiveRate)
		
		// Expect low false positive rate
		if falsePositiveRate > 5 {
			b.Errorf("False positive rate %.2f%% exceeds 5%% target", falsePositiveRate)
		}
	}
}

// Helper function to validate performance targets
func validatePerformanceTargets(b *testing.B, latencies []time.Duration) {
	if len(latencies) == 0 {
		return
	}
	
	// Sort latencies
	sorted := make([]time.Duration, len(latencies))
	copy(sorted, latencies)
	
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	
	p50 := sorted[len(sorted)*50/100]
	p95 := sorted[len(sorted)*95/100]
	p99 := sorted[len(sorted)*99/100]
	
	b.Logf("Performance targets - P50: %v, P95: %v, P99: %v", p50, p95, p99)
	
	// Validate against targets
	if p50 > 5*time.Millisecond {
		b.Errorf("P50 latency %v exceeds 5ms target", p50)
	}
	if p95 > 10*time.Millisecond {
		b.Errorf("P95 latency %v exceeds 10ms target", p95)
	}
	if p99 > 20*time.Millisecond {
		b.Errorf("P99 latency %v exceeds 20ms target", p99)
	}
}