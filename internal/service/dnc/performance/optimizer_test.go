package performance

import (
	"context"
	"testing"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestPerformanceOptimizer_Lifecycle(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := &OptimizerConfig{
		MaxConnections:     10,
		WorkerPoolSize:     5,
		L1CacheSize:        1000,
		BloomFilterEnabled: true,
	}
	
	optimizer := NewPerformanceOptimizer(config, nil, logger)
	ctx := context.Background()
	
	// Test Start
	err := optimizer.Start(ctx)
	require.NoError(t, err)
	
	// Test double start should fail
	err = optimizer.Start(ctx)
	assert.Error(t, err)
	
	// Test Stop
	err = optimizer.Stop(ctx)
	require.NoError(t, err)
	
	// Test double stop should not fail
	err = optimizer.Stop(ctx)
	assert.NoError(t, err)
}

func TestPerformanceOptimizer_OptimizeQuery(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := &OptimizerConfig{
		MaxConnections:     10,
		WorkerPoolSize:     5,
		L1CacheSize:        1000,
		BloomFilterEnabled: true,
	}
	
	optimizer := NewPerformanceOptimizer(config, nil, logger)
	ctx := context.Background()
	
	err := optimizer.Start(ctx)
	require.NoError(t, err)
	defer optimizer.Stop(ctx)
	
	phoneNumber, err := values.NewPhoneNumber("+15551234567")
	require.NoError(t, err)
	
	// Test query optimization
	optimization, err := optimizer.OptimizeQuery(ctx, phoneNumber)
	require.NoError(t, err)
	assert.NotNil(t, optimization)
	assert.Equal(t, phoneNumber, optimization.PhoneNumber)
	assert.NotZero(t, optimization.Timestamp)
}

func TestPerformanceOptimizer_CacheResult(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := &OptimizerConfig{
		L1CacheSize: 1000,
	}
	
	optimizer := NewPerformanceOptimizer(config, nil, logger)
	
	phoneNumber := "+15551234567"
	result := "test_result"
	
	err := optimizer.CacheResult(context.Background(), phoneNumber, result, false)
	assert.NoError(t, err)
	
	// Verify cache hit
	cached, found := optimizer.l1Cache.Get(phoneNumber)
	assert.True(t, found)
	assert.Equal(t, result, cached)
}

func TestPerformanceOptimizer_GetOptimizationStats(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := &OptimizerConfig{
		MaxConnections: 10,
		WorkerPoolSize: 5,
		L1CacheSize:    1000,
	}
	
	optimizer := NewPerformanceOptimizer(config, nil, logger)
	
	stats := optimizer.GetOptimizationStats()
	assert.NotNil(t, stats)
	assert.NotNil(t, stats.ConnectionPool)
	assert.NotNil(t, stats.WorkerPool)
	assert.NotNil(t, stats.MemoryPool)
	assert.NotNil(t, stats.L1Cache)
	assert.NotZero(t, stats.Timestamp)
}

func TestL1Cache_BasicOperations(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := &L1CacheConfig{
		Size: 100,
		TTL:  time.Minute,
	}
	
	cache := NewL1Cache(config, logger)
	defer cache.Stop()
	
	key := "test_key"
	value := "test_value"
	
	// Test Set and Get
	cache.Set(key, value, time.Minute)
	retrieved, found := cache.Get(key)
	assert.True(t, found)
	assert.Equal(t, value, retrieved)
	
	// Test Get non-existent key
	_, found = cache.Get("non_existent")
	assert.False(t, found)
	
	// Test Delete
	cache.Delete(key)
	_, found = cache.Get(key)
	assert.False(t, found)
}

func TestL1Cache_Expiration(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := &L1CacheConfig{
		Size: 100,
		TTL:  time.Millisecond * 10,
	}
	
	cache := NewL1Cache(config, logger)
	defer cache.Stop()
	
	key := "test_key"
	value := "test_value"
	
	// Set with short TTL
	cache.Set(key, value, time.Millisecond*10)
	
	// Should be available immediately
	retrieved, found := cache.Get(key)
	assert.True(t, found)
	assert.Equal(t, value, retrieved)
	
	// Wait for expiration
	time.Sleep(time.Millisecond * 15)
	
	// Should be expired
	_, found = cache.Get(key)
	assert.False(t, found)
}

func TestL1Cache_Eviction(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := &L1CacheConfig{
		Size:           3, // Small size to force eviction
		TTL:            time.Minute,
		EvictionPolicy: EvictionPolicyLRU,
	}
	
	cache := NewL1Cache(config, logger)
	defer cache.Stop()
	
	// Fill cache to capacity
	cache.Set("key1", "value1", time.Minute)
	cache.Set("key2", "value2", time.Minute)
	cache.Set("key3", "value3", time.Minute)
	
	// All should be present
	_, found := cache.Get("key1")
	assert.True(t, found)
	_, found = cache.Get("key2")
	assert.True(t, found)
	_, found = cache.Get("key3")
	assert.True(t, found)
	
	// Add one more to trigger eviction
	cache.Set("key4", "value4", time.Minute)
	
	// One of the original keys should be evicted
	stats := cache.GetStats()
	assert.Equal(t, 3, stats.Size)
	assert.Greater(t, stats.EvictionCount, int64(0))
}

func TestL1Cache_Stats(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := &L1CacheConfig{
		Size: 100,
		TTL:  time.Minute,
	}
	
	cache := NewL1Cache(config, logger)
	defer cache.Stop()
	
	// Test initial stats
	stats := cache.GetStats()
	assert.Equal(t, 0, stats.Size)
	assert.Equal(t, int64(0), stats.TotalHits)
	assert.Equal(t, int64(0), stats.TotalMisses)
	
	// Add some items and test
	cache.Set("key1", "value1", time.Minute)
	cache.Set("key2", "value2", time.Minute)
	
	// Test hits and misses
	_, _ = cache.Get("key1") // Hit
	_, _ = cache.Get("key3") // Miss
	
	stats = cache.GetStats()
	assert.Equal(t, 2, stats.Size)
	assert.Equal(t, int64(1), stats.TotalHits)
	assert.Equal(t, int64(1), stats.TotalMisses)
	assert.Equal(t, int64(2), stats.TotalQueries)
	assert.Equal(t, 50.0, stats.HitRate)
}

func TestBloomFilter_BasicOperations(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := &BloomFilterConfig{
		Size:   1000,
		Hashes: 3,
	}
	
	bf := NewBloomFilter(config, logger)
	
	item := "test_item"
	
	// Should not contain item initially
	assert.False(t, bf.MayContain(item))
	
	// Add item
	bf.Add(item)
	
	// Should contain item after adding
	assert.True(t, bf.MayContain(item))
	
	// Should not contain different item
	assert.False(t, bf.MayContain("different_item"))
}

func TestBloomFilter_FalsePositiveRate(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := &BloomFilterConfig{
		Size:   10000,
		Hashes: 7,
	}
	
	bf := NewBloomFilter(config, logger)
	
	// Add 1000 items
	addedItems := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		item := fmt.Sprintf("item_%d", i)
		bf.Add(item)
		addedItems[item] = true
	}
	
	// Test false positive rate with different items
	falsePositives := 0
	testCount := 1000
	
	for i := 0; i < testCount; i++ {
		item := fmt.Sprintf("test_item_%d", i)
		if bf.MayContain(item) && !addedItems[item] {
			falsePositives++
		}
	}
	
	falsePositiveRate := float64(falsePositives) / float64(testCount)
	
	// False positive rate should be reasonably low
	assert.Less(t, falsePositiveRate, 0.1, "False positive rate too high: %f", falsePositiveRate)
	
	// All added items should be found
	for item := range addedItems {
		assert.True(t, bf.MayContain(item), "Item %s should be found", item)
	}
}

func TestBloomFilter_Stats(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := &BloomFilterConfig{
		Size:   1000,
		Hashes: 3,
	}
	
	bf := NewBloomFilter(config, logger)
	
	// Test initial stats
	stats := bf.GetStats()
	assert.Equal(t, uint(1000), stats.Size)
	assert.Equal(t, uint(3), stats.HashFunctions)
	assert.Equal(t, int64(0), stats.TotalAdds)
	assert.Equal(t, int64(0), stats.TotalChecks)
	
	// Add items and check stats
	bf.Add("item1")
	bf.Add("item2")
	bf.MayContain("item1")
	bf.MayContain("item3")
	
	stats = bf.GetStats()
	assert.Equal(t, int64(2), stats.TotalAdds)
	assert.Equal(t, int64(2), stats.TotalChecks)
	assert.Greater(t, stats.MemoryUsage, int64(0))
}

func TestBloomFilter_Clear(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := &BloomFilterConfig{
		Size:   1000,
		Hashes: 3,
	}
	
	bf := NewBloomFilter(config, logger)
	
	// Add items
	bf.Add("item1")
	bf.Add("item2")
	
	// Verify items are present
	assert.True(t, bf.MayContain("item1"))
	assert.True(t, bf.MayContain("item2"))
	
	// Clear filter
	bf.Clear()
	
	// Items should no longer be present
	assert.False(t, bf.MayContain("item1"))
	assert.False(t, bf.MayContain("item2"))
	
	// Stats should be reset
	stats := bf.GetStats()
	assert.Equal(t, uint(0), stats.EstimatedItems)
	assert.Equal(t, int64(0), stats.TotalAdds)
}

func TestMemoryPool_BasicOperations(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := &MemoryPoolConfig{
		PoolSize:  10,
		BlockSize: 1024,
	}
	
	pool := NewMemoryPool(config, logger)
	
	// Test Get
	block := pool.Get()
	require.NotNil(t, block)
	assert.Len(t, block.Data, 1024)
	assert.True(t, block.InUse)
	
	// Test Put
	pool.Put(block)
	assert.False(t, block.InUse)
	
	// Test stats
	stats := pool.GetStats()
	assert.Equal(t, 10, stats.Total)
	assert.Greater(t, stats.TotalAllocs, int64(0))
	assert.Greater(t, stats.TotalFrees, int64(0))
}

func TestMemoryPool_Stats(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := &MemoryPoolConfig{
		PoolSize:  5,
		BlockSize: 512,
	}
	
	pool := NewMemoryPool(config, logger)
	
	// Get initial stats
	stats := pool.GetStats()
	assert.Equal(t, 5, stats.Total)
	assert.Equal(t, 0, stats.Allocated)
	assert.Equal(t, 5, stats.Available)
	assert.Equal(t, int64(0), stats.BytesInUse)
	assert.Equal(t, int64(5*512), stats.BytesTotal)
	
	// Allocate some blocks
	block1 := pool.Get()
	block2 := pool.Get()
	
	stats = pool.GetStats()
	assert.Equal(t, 2, stats.Allocated)
	assert.Equal(t, 3, stats.Available)
	assert.Equal(t, int64(2*512), stats.BytesInUse)
	
	// Return blocks
	pool.Put(block1)
	pool.Put(block2)
	
	stats = pool.GetStats()
	assert.Equal(t, 0, stats.Allocated)
	assert.Equal(t, 5, stats.Available)
	assert.Equal(t, int64(0), stats.BytesInUse)
}

func TestLoadBalancers(t *testing.T) {
	// Create test workers
	workers := make([]*Worker, 3)
	for i := 0; i < 3; i++ {
		workers[i] = &Worker{
			ID:     i,
			Active: false,
			Stats: WorkerStats{
				TasksCompleted: int64(i * 10),
				TasksFailed:    int64(i),
				TotalDuration:  time.Duration(i) * time.Millisecond,
			},
		}
	}
	
	task := Task{ID: "test_task"}
	
	t.Run("RoundRobin", func(t *testing.T) {
		balancer := NewRoundRobinBalancer()
		
		// Should cycle through workers
		worker1 := balancer.SelectWorker(workers, task)
		worker2 := balancer.SelectWorker(workers, task)
		worker3 := balancer.SelectWorker(workers, task)
		worker4 := balancer.SelectWorker(workers, task)
		
		assert.NotEqual(t, worker1.ID, worker2.ID)
		assert.NotEqual(t, worker2.ID, worker3.ID)
		assert.Equal(t, worker1.ID, worker4.ID) // Should cycle back
	})
	
	t.Run("LeastConnections", func(t *testing.T) {
		balancer := NewLeastConnectionsBalancer()
		
		// Should select worker with fewest tasks
		worker := balancer.SelectWorker(workers, task)
		assert.Equal(t, 0, worker.ID) // Worker 0 has 0 tasks
	})
	
	t.Run("WeightedRoundRobin", func(t *testing.T) {
		balancer := NewWeightedRoundRobinBalancer()
		
		worker := balancer.SelectWorker(workers, task)
		assert.NotNil(t, worker)
		assert.Equal(t, LoadBalancingWeightedRoundRobin, balancer.GetStrategy())
	})
	
	t.Run("LatencyBased", func(t *testing.T) {
		balancer := NewLatencyBasedBalancer()
		
		worker := balancer.SelectWorker(workers, task)
		assert.NotNil(t, worker)
		assert.Equal(t, LoadBalancingLatencyBased, balancer.GetStrategy())
	})
	
	t.Run("ResourceBased", func(t *testing.T) {
		balancer := NewResourceBasedBalancer()
		
		worker := balancer.SelectWorker(workers, task)
		assert.NotNil(t, worker)
		assert.Equal(t, LoadBalancingResourceBased, balancer.GetStrategy())
	})
}

func TestCacheEvictionPolicies(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	
	t.Run("LRU", func(t *testing.T) {
		config := &L1CacheConfig{
			Size:           2,
			TTL:            time.Minute,
			EvictionPolicy: EvictionPolicyLRU,
		}
		
		cache := NewL1Cache(config, logger)
		defer cache.Stop()
		
		// Fill cache
		cache.Set("key1", "value1", time.Minute)
		cache.Set("key2", "value2", time.Minute)
		
		// Access key1 to make it recently used
		_, _ = cache.Get("key1")
		
		// Add key3, should evict key2 (least recently used)
		cache.Set("key3", "value3", time.Minute)
		
		_, found1 := cache.Get("key1")
		_, found2 := cache.Get("key2")
		_, found3 := cache.Get("key3")
		
		assert.True(t, found1)  // Should be present
		assert.False(t, found2) // Should be evicted
		assert.True(t, found3)  // Should be present
	})
}

func TestConcurrentAccess(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	config := &L1CacheConfig{
		Size: 1000,
		TTL:  time.Minute,
	}
	
	cache := NewL1Cache(config, logger)
	defer cache.Stop()
	
	// Test concurrent read/write operations
	done := make(chan bool, 2)
	
	// Writer goroutine
	go func() {
		for i := 0; i < 100; i++ {
			key := fmt.Sprintf("key_%d", i)
			value := fmt.Sprintf("value_%d", i)
			cache.Set(key, value, time.Minute)
		}
		done <- true
	}()
	
	// Reader goroutine
	go func() {
		for i := 0; i < 100; i++ {
			key := fmt.Sprintf("key_%d", i%10)
			_, _ = cache.Get(key)
		}
		done <- true
	}()
	
	// Wait for both to complete
	<-done
	<-done
	
	// Verify no race conditions occurred
	stats := cache.GetStats()
	assert.Greater(t, stats.TotalQueries, int64(0))
}