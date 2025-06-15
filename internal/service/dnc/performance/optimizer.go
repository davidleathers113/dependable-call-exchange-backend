package performance

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/infrastructure/cache"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
)

// PerformanceOptimizer handles all performance optimizations for DNC checks
type PerformanceOptimizer struct {
	logger *zap.Logger
	
	// Connection pooling
	connectionPool *ConnectionPool
	
	// Memory management
	memoryPool *MemoryPool
	bufferPool *sync.Pool
	
	// Goroutine management
	workerPool *WorkerPool
	
	// Caching layers
	l1Cache *L1Cache      // In-memory cache
	l2Cache cache.Service // Redis cache
	bloomFilter *BloomFilter
	
	// Performance monitoring
	metrics *OptimizerMetrics
	
	// Configuration
	config *OptimizerConfig
	
	// State management
	running int32
	stopped chan struct{}
	wg      sync.WaitGroup
}

// OptimizerConfig contains all performance tuning parameters
type OptimizerConfig struct {
	// Connection pool settings
	MaxConnections        int           `yaml:"max_connections" default:"100"`
	MinIdleConnections   int           `yaml:"min_idle_connections" default:"10"`
	ConnectionTimeout    time.Duration `yaml:"connection_timeout" default:"5s"`
	ConnectionMaxAge     time.Duration `yaml:"connection_max_age" default:"30m"`
	PrewarmConnections   bool          `yaml:"prewarm_connections" default:"true"`
	
	// Memory pool settings
	MemoryPoolSize       int  `yaml:"memory_pool_size" default:"1000"`
	BufferPoolSize       int  `yaml:"buffer_pool_size" default:"64"`
	EnableMemoryOptimize bool `yaml:"enable_memory_optimize" default:"true"`
	
	// Worker pool settings
	WorkerPoolSize       int           `yaml:"worker_pool_size" default:"50"`
	QueueSize           int           `yaml:"queue_size" default:"10000"`
	WorkerIdleTimeout   time.Duration `yaml:"worker_idle_timeout" default:"30s"`
	
	// Cache settings
	L1CacheSize         int           `yaml:"l1_cache_size" default:"100000"`
	L1CacheTTL          time.Duration `yaml:"l1_cache_ttl" default:"5m"`
	L2CacheTTL          time.Duration `yaml:"l2_cache_ttl" default:"1h"`
	CacheWarmupEnabled  bool          `yaml:"cache_warmup_enabled" default:"true"`
	CacheWarmupInterval time.Duration `yaml:"cache_warmup_interval" default:"10m"`
	
	// Bloom filter settings
	BloomFilterSize     uint          `yaml:"bloom_filter_size" default:"1000000"`
	BloomFilterHashes   uint          `yaml:"bloom_filter_hashes" default:"7"`
	BloomFilterEnabled  bool          `yaml:"bloom_filter_enabled" default:"true"`
	
	// Performance targets
	TargetLatencyMs     float64 `yaml:"target_latency_ms" default:"10.0"`
	CacheHitLatencyMs   float64 `yaml:"cache_hit_latency_ms" default:"1.0"`
	TargetThroughput    int     `yaml:"target_throughput" default:"100000"`
}

// OptimizerMetrics tracks performance optimization metrics
type OptimizerMetrics struct {
	// Cache metrics
	L1CacheHits   prometheus.Counter
	L1CacheMisses prometheus.Counter
	L2CacheHits   prometheus.Counter
	L2CacheMisses prometheus.Counter
	BloomHits     prometheus.Counter
	BloomMisses   prometheus.Counter
	
	// Pool metrics
	ConnectionPoolActive   prometheus.Gauge
	ConnectionPoolIdle     prometheus.Gauge
	WorkerPoolActive       prometheus.Gauge
	WorkerPoolQueued       prometheus.Gauge
	MemoryPoolAllocated    prometheus.Gauge
	MemoryPoolAvailable    prometheus.Gauge
	
	// Performance metrics
	OptimizationDuration   prometheus.Histogram
	CacheWarmupDuration    prometheus.Histogram
	MemoryOptimizeDuration prometheus.Histogram
	
	// Resource utilization
	CPUUtilization    prometheus.Gauge
	MemoryUtilization prometheus.Gauge
	GCPause          prometheus.Histogram
}

// NewPerformanceOptimizer creates a new performance optimizer
func NewPerformanceOptimizer(
	config *OptimizerConfig,
	l2Cache cache.Service,
	logger *zap.Logger,
) *PerformanceOptimizer {
	if config == nil {
		config = &OptimizerConfig{} // Use defaults
	}
	
	optimizer := &PerformanceOptimizer{
		logger:  logger,
		config:  config,
		l2Cache: l2Cache,
		stopped: make(chan struct{}),
		metrics: createOptimizerMetrics(),
	}
	
	optimizer.initializePools()
	optimizer.initializeCaches()
	
	return optimizer
}

// Start begins all optimization processes
func (o *PerformanceOptimizer) Start(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&o.running, 0, 1) {
		return fmt.Errorf("optimizer already running")
	}
	
	o.logger.Info("Starting performance optimizer",
		zap.Int("connection_pool_size", o.config.MaxConnections),
		zap.Int("worker_pool_size", o.config.WorkerPoolSize),
		zap.Int("l1_cache_size", o.config.L1CacheSize),
		zap.Float64("target_latency_ms", o.config.TargetLatencyMs),
	)
	
	// Start connection pool
	if err := o.connectionPool.Start(ctx); err != nil {
		return fmt.Errorf("failed to start connection pool: %w", err)
	}
	
	// Start worker pool
	if err := o.workerPool.Start(ctx); err != nil {
		return fmt.Errorf("failed to start worker pool: %w", err)
	}
	
	// Start cache warming if enabled
	if o.config.CacheWarmupEnabled {
		o.wg.Add(1)
		go o.runCacheWarmer(ctx)
	}
	
	// Start memory optimization if enabled
	if o.config.EnableMemoryOptimize {
		o.wg.Add(1)
		go o.runMemoryOptimizer(ctx)
	}
	
	// Start metrics collection
	o.wg.Add(1)
	go o.runMetricsCollector(ctx)
	
	o.logger.Info("Performance optimizer started successfully")
	return nil
}

// Stop gracefully shuts down the optimizer
func (o *PerformanceOptimizer) Stop(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&o.running, 1, 0) {
		return nil
	}
	
	o.logger.Info("Stopping performance optimizer")
	
	close(o.stopped)
	
	// Stop pools
	o.connectionPool.Stop(ctx)
	o.workerPool.Stop(ctx)
	
	// Wait for background routines
	done := make(chan struct{})
	go func() {
		o.wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		o.logger.Info("Performance optimizer stopped gracefully")
	case <-ctx.Done():
		o.logger.Warn("Performance optimizer stop timed out")
		return ctx.Err()
	}
	
	return nil
}

// OptimizeQuery applies all optimizations to a DNC query
func (o *PerformanceOptimizer) OptimizeQuery(ctx context.Context, phoneNumber *values.PhoneNumber) (*QueryOptimization, error) {
	start := time.Now()
	defer func() {
		o.metrics.OptimizationDuration.Observe(time.Since(start).Seconds())
	}()
	
	optimization := &QueryOptimization{
		PhoneNumber: phoneNumber,
		Timestamp:   start,
	}
	
	// Check bloom filter first for negative lookups
	if o.config.BloomFilterEnabled && o.bloomFilter != nil {
		if !o.bloomFilter.MayContain(phoneNumber.String()) {
			o.metrics.BloomHits.Inc()
			optimization.BloomFilterResult = BloomFilterNegative
			optimization.CacheStrategy = CacheStrategySkip
			return optimization, nil
		}
		o.metrics.BloomMisses.Inc()
		optimization.BloomFilterResult = BloomFilterPositive
	}
	
	// Check L1 cache
	if result, found := o.l1Cache.Get(phoneNumber.String()); found {
		o.metrics.L1CacheHits.Inc()
		optimization.L1CacheHit = true
		optimization.CacheStrategy = CacheStrategyL1Hit
		optimization.Result = result
		return optimization, nil
	}
	o.metrics.L1CacheMisses.Inc()
	
	// Check L2 cache
	if o.l2Cache != nil {
		if result, err := o.l2Cache.Get(ctx, phoneNumber.String()); err == nil {
			o.metrics.L2CacheHits.Inc()
			optimization.L2CacheHit = true
			optimization.CacheStrategy = CacheStrategyL2Hit
			optimization.Result = result
			
			// Promote to L1 cache
			o.l1Cache.Set(phoneNumber.String(), result, o.config.L1CacheTTL)
			return optimization, nil
		}
		o.metrics.L2CacheMisses.Inc()
	}
	
	// No cache hit - will need database query
	optimization.CacheStrategy = CacheStrategyMiss
	optimization.RequiresDBQuery = true
	
	// Get optimized connection
	conn, err := o.connectionPool.GetConnection(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get optimized connection: %w", err)
	}
	optimization.Connection = conn
	
	// Get worker for async processing if needed
	if worker, err := o.workerPool.GetWorker(ctx); err == nil {
		optimization.Worker = worker
	}
	
	return optimization, nil
}

// CacheResult stores query result in appropriate cache layers
func (o *PerformanceOptimizer) CacheResult(ctx context.Context, phoneNumber string, result interface{}, isOnDNC bool) error {
	// Store in L1 cache
	o.l1Cache.Set(phoneNumber, result, o.config.L1CacheTTL)
	
	// Store in L2 cache if available
	if o.l2Cache != nil {
		if err := o.l2Cache.Set(ctx, phoneNumber, result, o.config.L2CacheTTL); err != nil {
			o.logger.Warn("Failed to store in L2 cache",
				zap.String("phone_number", phoneNumber),
				zap.Error(err),
			)
		}
	}
	
	// Update bloom filter for negative results
	if o.config.BloomFilterEnabled && o.bloomFilter != nil && !isOnDNC {
		o.bloomFilter.Add(phoneNumber)
	}
	
	return nil
}

// GetOptimizationStats returns current optimization statistics
func (o *PerformanceOptimizer) GetOptimizationStats() *OptimizationStats {
	return &OptimizationStats{
		ConnectionPool: o.connectionPool.GetStats(),
		WorkerPool:     o.workerPool.GetStats(),
		MemoryPool:     o.memoryPool.GetStats(),
		L1Cache:        o.l1Cache.GetStats(),
		BloomFilter:    o.bloomFilter.GetStats(),
		Timestamp:      time.Now(),
	}
}

// initializePools sets up all resource pools
func (o *PerformanceOptimizer) initializePools() {
	// Connection pool
	o.connectionPool = NewConnectionPool(&ConnectionPoolConfig{
		MaxConnections:     o.config.MaxConnections,
		MinIdleConnections: o.config.MinIdleConnections,
		ConnectionTimeout:  o.config.ConnectionTimeout,
		ConnectionMaxAge:   o.config.ConnectionMaxAge,
		PrewarmEnabled:     o.config.PrewarmConnections,
	}, o.logger)
	
	// Worker pool
	o.workerPool = NewWorkerPool(&WorkerPoolConfig{
		PoolSize:    o.config.WorkerPoolSize,
		QueueSize:   o.config.QueueSize,
		IdleTimeout: o.config.WorkerIdleTimeout,
	}, o.logger)
	
	// Memory pool
	o.memoryPool = NewMemoryPool(&MemoryPoolConfig{
		PoolSize: o.config.MemoryPoolSize,
	}, o.logger)
	
	// Buffer pool for reusing byte slices
	o.bufferPool = &sync.Pool{
		New: func() interface{} {
			return make([]byte, o.config.BufferPoolSize)
		},
	}
}

// initializeCaches sets up all caching layers
func (o *PerformanceOptimizer) initializeCaches() {
	// L1 in-memory cache
	o.l1Cache = NewL1Cache(&L1CacheConfig{
		Size: o.config.L1CacheSize,
		TTL:  o.config.L1CacheTTL,
	}, o.logger)
	
	// Bloom filter for negative lookups
	if o.config.BloomFilterEnabled {
		o.bloomFilter = NewBloomFilter(&BloomFilterConfig{
			Size:   o.config.BloomFilterSize,
			Hashes: o.config.BloomFilterHashes,
		}, o.logger)
	}
}

// runCacheWarmer periodically warms up caches
func (o *PerformanceOptimizer) runCacheWarmer(ctx context.Context) {
	defer o.wg.Done()
	
	ticker := time.NewTicker(o.config.CacheWarmupInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-o.stopped:
			return
		case <-ticker.C:
			o.warmupCaches(ctx)
		}
	}
}

// runMemoryOptimizer periodically optimizes memory usage
func (o *PerformanceOptimizer) runMemoryOptimizer(ctx context.Context) {
	defer o.wg.Done()
	
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-o.stopped:
			return
		case <-ticker.C:
			o.optimizeMemory(ctx)
		}
	}
}

// runMetricsCollector collects resource utilization metrics
func (o *PerformanceOptimizer) runMetricsCollector(ctx context.Context) {
	defer o.wg.Done()
	
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-o.stopped:
			return
		case <-ticker.C:
			o.collectMetrics()
		}
	}
}

// warmupCaches preloads frequently accessed data
func (o *PerformanceOptimizer) warmupCaches(ctx context.Context) {
	start := time.Now()
	defer func() {
		o.metrics.CacheWarmupDuration.Observe(time.Since(start).Seconds())
	}()
	
	o.logger.Debug("Starting cache warmup")
	
	// Implementation would load frequently accessed phone numbers
	// from analytics or usage patterns
	
	o.logger.Debug("Cache warmup completed",
		zap.Duration("duration", time.Since(start)),
	)
}

// optimizeMemory performs memory optimization
func (o *PerformanceOptimizer) optimizeMemory(ctx context.Context) {
	start := time.Now()
	defer func() {
		o.metrics.MemoryOptimizeDuration.Observe(time.Since(start).Seconds())
	}()
	
	// Force garbage collection if memory usage is high
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	memUsageMB := float64(m.Alloc) / 1024 / 1024
	if memUsageMB > 1000 { // > 1GB
		runtime.GC()
		o.logger.Debug("Forced garbage collection",
			zap.Float64("memory_usage_mb", memUsageMB),
		)
	}
	
	// Clean up cache entries if needed
	o.l1Cache.Cleanup()
}

// collectMetrics updates performance metrics
func (o *PerformanceOptimizer) collectMetrics() {
	// Connection pool metrics
	connStats := o.connectionPool.GetStats()
	o.metrics.ConnectionPoolActive.Set(float64(connStats.Active))
	o.metrics.ConnectionPoolIdle.Set(float64(connStats.Idle))
	
	// Worker pool metrics
	workerStats := o.workerPool.GetStats()
	o.metrics.WorkerPoolActive.Set(float64(workerStats.Active))
	o.metrics.WorkerPoolQueued.Set(float64(workerStats.Queued))
	
	// Memory metrics
	memStats := o.memoryPool.GetStats()
	o.metrics.MemoryPoolAllocated.Set(float64(memStats.Allocated))
	o.metrics.MemoryPoolAvailable.Set(float64(memStats.Available))
	
	// System resource metrics
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	o.metrics.MemoryUtilization.Set(float64(m.Alloc) / 1024 / 1024) // MB
	o.metrics.GCPause.Observe(float64(m.PauseNs[(m.NumGC+255)%256]) / 1e9) // seconds
}

// createOptimizerMetrics initializes Prometheus metrics
func createOptimizerMetrics() *OptimizerMetrics {
	return &OptimizerMetrics{
		L1CacheHits: promauto.NewCounter(prometheus.CounterOpts{
			Name: "dnc_optimizer_l1_cache_hits_total",
			Help: "Total number of L1 cache hits",
		}),
		L1CacheMisses: promauto.NewCounter(prometheus.CounterOpts{
			Name: "dnc_optimizer_l1_cache_misses_total",
			Help: "Total number of L1 cache misses",
		}),
		L2CacheHits: promauto.NewCounter(prometheus.CounterOpts{
			Name: "dnc_optimizer_l2_cache_hits_total",
			Help: "Total number of L2 cache hits",
		}),
		L2CacheMisses: promauto.NewCounter(prometheus.CounterOpts{
			Name: "dnc_optimizer_l2_cache_misses_total",
			Help: "Total number of L2 cache misses",
		}),
		BloomHits: promauto.NewCounter(prometheus.CounterOpts{
			Name: "dnc_optimizer_bloom_hits_total",
			Help: "Total number of bloom filter hits",
		}),
		BloomMisses: promauto.NewCounter(prometheus.CounterOpts{
			Name: "dnc_optimizer_bloom_misses_total",
			Help: "Total number of bloom filter misses",
		}),
		ConnectionPoolActive: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "dnc_optimizer_connection_pool_active",
			Help: "Number of active connections in pool",
		}),
		ConnectionPoolIdle: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "dnc_optimizer_connection_pool_idle",
			Help: "Number of idle connections in pool",
		}),
		WorkerPoolActive: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "dnc_optimizer_worker_pool_active",
			Help: "Number of active workers in pool",
		}),
		WorkerPoolQueued: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "dnc_optimizer_worker_pool_queued",
			Help: "Number of queued tasks in worker pool",
		}),
		MemoryPoolAllocated: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "dnc_optimizer_memory_pool_allocated",
			Help: "Number of allocated memory blocks",
		}),
		MemoryPoolAvailable: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "dnc_optimizer_memory_pool_available",
			Help: "Number of available memory blocks",
		}),
		OptimizationDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "dnc_optimizer_optimization_duration_seconds",
			Help:    "Duration of query optimization operations",
			Buckets: prometheus.ExponentialBuckets(0.0001, 2, 15), // 0.1ms to ~3s
		}),
		CacheWarmupDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "dnc_optimizer_cache_warmup_duration_seconds",
			Help:    "Duration of cache warmup operations",
			Buckets: prometheus.ExponentialBuckets(0.1, 2, 10), // 0.1s to ~100s
		}),
		MemoryOptimizeDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "dnc_optimizer_memory_optimize_duration_seconds",
			Help:    "Duration of memory optimization operations",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 10), // 1ms to ~1s
		}),
		CPUUtilization: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "dnc_optimizer_cpu_utilization_percent",
			Help: "CPU utilization percentage",
		}),
		MemoryUtilization: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "dnc_optimizer_memory_utilization_mb",
			Help: "Memory utilization in megabytes",
		}),
		GCPause: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "dnc_optimizer_gc_pause_seconds",
			Help:    "Garbage collection pause duration",
			Buckets: prometheus.ExponentialBuckets(0.0001, 2, 10), // 0.1ms to ~100ms
		}),
	}
}