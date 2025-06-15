package cache

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/dnc"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/infrastructure/config"
)

// DNC cache key prefixes
const (
	DNCEntryPrefix      = "dce:dnc:entry:"      // Individual DNC entries by phone number
	DNCCheckPrefix      = "dce:dnc:check:"      // Check results cache
	DNCBloomPrefix      = "dce:dnc:bloom:"      // Bloom filter for negative lookups
	DNCProviderPrefix   = "dce:dnc:provider:"   // Provider-specific caches
	DNCSourcePrefix     = "dce:dnc:source:"     // Source-specific caches
	DNCStatsPrefix      = "dce:dnc:stats:"      // Cache statistics
	DNCWarmingPrefix    = "dce:dnc:warming:"    // Cache warming locks
	DNCConfigPrefix     = "dce:dnc:config:"     // Cache configuration
)

// DNC cache TTL values
const (
	DNCEntryTTL       = 24 * time.Hour  // DNC entries are cached for 24 hours
	DNCCheckTTL       = 6 * time.Hour   // Check results cached for 6 hours
	DNCBloomTTL       = 12 * time.Hour  // Bloom filter refreshed every 12 hours
	DNCProviderTTL    = 2 * time.Hour   // Provider data cached for 2 hours
	DNCNegativeTTL    = 30 * time.Minute // Negative lookups cached for 30 minutes
	DNCStatsTTL       = 5 * time.Minute  // Statistics refreshed every 5 minutes
	DNCWarmingLockTTL = 10 * time.Minute // Cache warming lock expires in 10 minutes
)

// Cache performance metrics
type DNCCacheMetrics struct {
	Hits             int64   `json:"hits"`
	Misses           int64   `json:"misses"`
	Errors           int64   `json:"errors"`
	HitRate          float64 `json:"hit_rate"`
	AvgLatency       float64 `json:"avg_latency_ms"`
	BloomFilterHits  int64   `json:"bloom_filter_hits"`
	BloomFilterFalsePositives int64 `json:"bloom_filter_false_positives"`
	WarmingOperations int64   `json:"warming_operations"`
	CompressedWrites int64   `json:"compressed_writes"`
	PipelineOperations int64  `json:"pipeline_operations"`
}

// DNCCacheConfig holds cache-specific configuration
type DNCCacheConfig struct {
	BloomFilterEnabled    bool          `json:"bloom_filter_enabled"`
	BloomFilterSize       uint64        `json:"bloom_filter_size"`
	BloomFilterHashCount  uint64        `json:"bloom_filter_hash_count"`
	CompressionEnabled    bool          `json:"compression_enabled"`
	CompressionThreshold  int           `json:"compression_threshold"`
	WarmingBatchSize      int           `json:"warming_batch_size"`
	PipelineBatchSize     int           `json:"pipeline_batch_size"`
	SlidingExpirationRate float64       `json:"sliding_expiration_rate"`
}

// DNCCache provides high-performance caching for DNC lookups
type DNCCache struct {
	client   *redis.Client
	logger   *zap.Logger
	config   DNCCacheConfig
	metrics  DNCCacheMetrics
	mu       sync.RWMutex
	bloomFilter *BloomFilter
}

// BloomFilter provides probabilistic set membership testing
type BloomFilter struct {
	bits      []byte
	size      uint64
	hashCount uint64
	mu        sync.RWMutex
}

// CachedDNCEntry represents a cached DNC entry with metadata
type CachedDNCEntry struct {
	Entry      *dnc.DNCEntry `json:"entry"`
	CachedAt   time.Time     `json:"cached_at"`
	LastAccess time.Time     `json:"last_access"`
	AccessCount int64        `json:"access_count"`
	Source     string        `json:"source"`
	Version    string        `json:"version"`
}

// CachedCheckResult represents a cached DNC check result
type CachedCheckResult struct {
	Result      *dnc.DNCCheckResult `json:"result"`
	CachedAt    time.Time           `json:"cached_at"`
	LastAccess  time.Time           `json:"last_access"`
	AccessCount int64               `json:"access_count"`
	Compressed  bool                `json:"compressed"`
}

// NewDNCCache creates a new DNC cache instance with optimized configuration
func NewDNCCache(cfg *config.RedisConfig, logger *zap.Logger) (*DNCCache, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger is required")
	}

	if cfg == nil {
		return nil, fmt.Errorf("redis config is required")
	}

	// Create Redis client with optimized settings for DNC operations
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
		
		// Optimized for high-throughput DNC operations
		MaxRetryBackoff: 100 * time.Millisecond,
		IdleTimeout:     5 * time.Minute,
		MaxConnAge:      30 * time.Minute,
	}

	client := redis.NewClient(opts)

	// Health check
	ctx, cancel := context.WithTimeout(context.Background(), cfg.DialTimeout)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis connection failed: %w", err)
	}

	// Default cache configuration optimized for DNC operations
	defaultConfig := DNCCacheConfig{
		BloomFilterEnabled:    true,
		BloomFilterSize:       1000000, // 1M bits for ~100K phone numbers with 1% false positive rate
		BloomFilterHashCount:  7,       // Optimal hash count for 1% false positive rate
		CompressionEnabled:    true,
		CompressionThreshold:  1024,    // Compress payloads > 1KB
		WarmingBatchSize:      1000,    // Process 1000 entries per warming batch
		PipelineBatchSize:     100,     // Pipeline 100 operations at once
		SlidingExpirationRate: 0.1,     // 10% chance of extending TTL on access
	}

	cache := &DNCCache{
		client:  client,
		logger:  logger,
		config:  defaultConfig,
		metrics: DNCCacheMetrics{},
	}

	// Initialize bloom filter
	if cache.config.BloomFilterEnabled {
		cache.bloomFilter = NewBloomFilter(cache.config.BloomFilterSize, cache.config.BloomFilterHashCount)
	}

	logger.Info("DNC cache initialized",
		zap.String("addr", cfg.URL),
		zap.Int("db", cfg.DB),
		zap.Bool("bloom_filter", cache.config.BloomFilterEnabled),
		zap.Bool("compression", cache.config.CompressionEnabled))

	return cache, nil
}

// GetDNCEntry retrieves a DNC entry from cache with performance optimizations
func (c *DNCCache) GetDNCEntry(ctx context.Context, phoneNumber values.PhoneNumber) (*dnc.DNCEntry, error) {
	start := time.Now()
	defer func() {
		latency := time.Since(start).Seconds() * 1000 // Convert to milliseconds
		c.updateLatencyMetric(latency)
	}()

	// Normalize phone number for consistent caching
	normalizedPhone := phoneNumber.String()
	key := DNCEntryPrefix + c.hashPhoneNumber(normalizedPhone)

	// Check bloom filter first for negative lookups
	if c.config.BloomFilterEnabled && c.bloomFilter != nil {
		if !c.bloomFilter.Contains(normalizedPhone) {
			c.incrementMetric("bloom_filter_hits")
			c.incrementMetric("misses")
			return nil, ErrCacheKeyNotFound{Key: key}
		}
	}

	// Get from Redis
	data, err := c.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			c.incrementMetric("misses")
			return nil, ErrCacheKeyNotFound{Key: key}
		}
		c.incrementMetric("errors")
		c.logger.Error("DNC entry cache get failed", 
			zap.String("phone", normalizedPhone), 
			zap.Error(err))
		return nil, fmt.Errorf("cache get failed: %w", err)
	}

	// Deserialize cached entry
	var cached CachedDNCEntry
	if err := json.Unmarshal([]byte(data), &cached); err != nil {
		c.incrementMetric("errors")
		c.logger.Error("DNC entry cache unmarshal failed", 
			zap.String("phone", normalizedPhone), 
			zap.Error(err))
		return nil, fmt.Errorf("cache unmarshal failed: %w", err)
	}

	// Update access tracking
	cached.LastAccess = time.Now()
	cached.AccessCount++

	// Sliding expiration: randomly extend TTL on access
	if c.shouldExtendTTL() {
		go c.extendTTL(ctx, key, DNCEntryTTL)
	}

	// Update cached entry asynchronously
	go c.updateCachedEntry(ctx, key, &cached)

	c.incrementMetric("hits")
	return cached.Entry, nil
}

// SetDNCEntry stores a DNC entry in cache with write-through strategy
func (c *DNCCache) SetDNCEntry(ctx context.Context, entry *dnc.DNCEntry) error {
	start := time.Now()
	defer func() {
		latency := time.Since(start).Seconds() * 1000
		c.updateLatencyMetric(latency)
	}()

	normalizedPhone := entry.PhoneNumber.String()
	key := DNCEntryPrefix + c.hashPhoneNumber(normalizedPhone)

	// Create cached entry wrapper
	cached := CachedDNCEntry{
		Entry:       entry,
		CachedAt:    time.Now(),
		LastAccess:  time.Now(),
		AccessCount: 0,
		Source:      "write_through",
		Version:     "1.0",
	}

	// Serialize entry
	data, err := json.Marshal(cached)
	if err != nil {
		c.incrementMetric("errors")
		c.logger.Error("DNC entry cache marshal failed", 
			zap.String("phone", normalizedPhone), 
			zap.Error(err))
		return fmt.Errorf("cache marshal failed: %w", err)
	}

	// Compress if enabled and data is large enough
	if c.config.CompressionEnabled && len(data) > c.config.CompressionThreshold {
		data = c.compressData(data)
		c.incrementMetric("compressed_writes")
	}

	// Store in Redis
	if err := c.client.Set(ctx, key, data, DNCEntryTTL).Err(); err != nil {
		c.incrementMetric("errors")
		c.logger.Error("DNC entry cache set failed", 
			zap.String("phone", normalizedPhone), 
			zap.Error(err))
		return fmt.Errorf("cache set failed: %w", err)
	}

	// Add to bloom filter if enabled
	if c.config.BloomFilterEnabled && c.bloomFilter != nil {
		c.bloomFilter.Add(normalizedPhone)
	}

	c.logger.Debug("DNC entry cached",
		zap.String("phone", normalizedPhone),
		zap.String("source", string(entry.ListSource)),
		zap.Bool("compressed", len(data) > c.config.CompressionThreshold))

	return nil
}

// GetCheckResult retrieves a cached DNC check result
func (c *DNCCache) GetCheckResult(ctx context.Context, phoneNumber values.PhoneNumber) (*dnc.DNCCheckResult, error) {
	start := time.Now()
	defer func() {
		latency := time.Since(start).Seconds() * 1000
		c.updateLatencyMetric(latency)
	}()

	normalizedPhone := phoneNumber.String()
	key := DNCCheckPrefix + c.hashPhoneNumber(normalizedPhone)

	data, err := c.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			c.incrementMetric("misses")
			return nil, ErrCacheKeyNotFound{Key: key}
		}
		c.incrementMetric("errors")
		return nil, fmt.Errorf("cache get failed: %w", err)
	}

	var cached CachedCheckResult
	if err := json.Unmarshal([]byte(data), &cached); err != nil {
		c.incrementMetric("errors")
		return nil, fmt.Errorf("cache unmarshal failed: %w", err)
	}

	// Check if result has expired based on its own TTL
	if cached.Result.IsExpired() {
		c.incrementMetric("misses")
		// Asynchronously delete expired entry
		go c.client.Del(context.Background(), key)
		return nil, ErrCacheKeyNotFound{Key: key}
	}

	// Update access tracking
	cached.LastAccess = time.Now()
	cached.AccessCount++

	// Sliding expiration
	if c.shouldExtendTTL() {
		go c.extendTTL(ctx, key, DNCCheckTTL)
	}

	go c.updateCachedCheckResult(ctx, key, &cached)

	c.incrementMetric("hits")
	return cached.Result, nil
}

// SetCheckResult stores a DNC check result in cache
func (c *DNCCache) SetCheckResult(ctx context.Context, result *dnc.DNCCheckResult) error {
	start := time.Now()
	defer func() {
		latency := time.Since(start).Seconds() * 1000
		c.updateLatencyMetric(latency)
	}()

	normalizedPhone := result.PhoneNumber.String()
	key := DNCCheckPrefix + c.hashPhoneNumber(normalizedPhone)

	cached := CachedCheckResult{
		Result:      result,
		CachedAt:    time.Now(),
		LastAccess:  time.Now(),
		AccessCount: 0,
		Compressed:  false,
	}

	data, err := json.Marshal(cached)
	if err != nil {
		c.incrementMetric("errors")
		return fmt.Errorf("cache marshal failed: %w", err)
	}

	// Use result's own TTL, but cap it at our maximum
	ttl := result.TTL
	if ttl > DNCCheckTTL {
		ttl = DNCCheckTTL
	}

	if err := c.client.Set(ctx, key, data, ttl).Err(); err != nil {
		c.incrementMetric("errors")
		return fmt.Errorf("cache set failed: %w", err)
	}

	c.logger.Debug("DNC check result cached",
		zap.String("phone", normalizedPhone),
		zap.Bool("blocked", result.IsBlocked),
		zap.Duration("ttl", ttl))

	return nil
}

// BulkGetDNCEntries retrieves multiple DNC entries using pipeline operations
func (c *DNCCache) BulkGetDNCEntries(ctx context.Context, phoneNumbers []values.PhoneNumber) (map[string]*dnc.DNCEntry, error) {
	if len(phoneNumbers) == 0 {
		return make(map[string]*dnc.DNCEntry), nil
	}

	start := time.Now()
	defer func() {
		latency := time.Since(start).Seconds() * 1000
		c.updateLatencyMetric(latency)
	}()

	// Create pipeline for bulk operations
	pipe := c.client.Pipeline()
	keyToPhone := make(map[string]string)
	
	// Add all gets to pipeline
	for _, phone := range phoneNumbers {
		normalizedPhone := phone.String()
		key := DNCEntryPrefix + c.hashPhoneNumber(normalizedPhone)
		keyToPhone[key] = normalizedPhone
		pipe.Get(ctx, key)
	}

	// Execute pipeline
	results, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		c.incrementMetric("errors")
		return nil, fmt.Errorf("pipeline execution failed: %w", err)
	}

	// Process results
	entries := make(map[string]*dnc.DNCEntry)
	hits := 0
	misses := 0

	for i, result := range results {
		cmd, ok := result.(*redis.StringCmd)
		if !ok {
			continue
		}

		data, err := cmd.Result()
		if err != nil {
			if err == redis.Nil {
				misses++
				continue
			}
			c.logger.Warn("bulk get result error", zap.Error(err))
			continue
		}

		var cached CachedDNCEntry
		if err := json.Unmarshal([]byte(data), &cached); err != nil {
			c.logger.Warn("bulk get unmarshal error", zap.Error(err))
			continue
		}

		phoneNumber := phoneNumbers[i].String()
		entries[phoneNumber] = cached.Entry
		hits++

		// Update access tracking asynchronously
		key := DNCEntryPrefix + c.hashPhoneNumber(phoneNumber)
		cached.LastAccess = time.Now()
		cached.AccessCount++
		go c.updateCachedEntry(ctx, key, &cached)
	}

	// Update metrics
	c.addMetric("hits", int64(hits))
	c.addMetric("misses", int64(misses))
	c.incrementMetric("pipeline_operations")

	c.logger.Debug("bulk DNC entries retrieved",
		zap.Int("requested", len(phoneNumbers)),
		zap.Int("hits", hits),
		zap.Int("misses", misses))

	return entries, nil
}

// BulkSetDNCEntries stores multiple DNC entries using pipeline operations
func (c *DNCCache) BulkSetDNCEntries(ctx context.Context, entries []*dnc.DNCEntry) error {
	if len(entries) == 0 {
		return nil
	}

	start := time.Now()
	defer func() {
		latency := time.Since(start).Seconds() * 1000
		c.updateLatencyMetric(latency)
	}()

	// Process in batches to avoid overwhelming Redis
	batchSize := c.config.PipelineBatchSize
	for i := 0; i < len(entries); i += batchSize {
		end := i + batchSize
		if end > len(entries) {
			end = len(entries)
		}

		if err := c.bulkSetBatch(ctx, entries[i:end]); err != nil {
			return fmt.Errorf("bulk set batch failed: %w", err)
		}
	}

	c.incrementMetric("pipeline_operations")
	c.logger.Debug("bulk DNC entries stored", zap.Int("count", len(entries)))

	return nil
}

// bulkSetBatch processes a single batch of entries
func (c *DNCCache) bulkSetBatch(ctx context.Context, entries []*dnc.DNCEntry) error {
	pipe := c.client.Pipeline()

	for _, entry := range entries {
		normalizedPhone := entry.PhoneNumber.String()
		key := DNCEntryPrefix + c.hashPhoneNumber(normalizedPhone)

		cached := CachedDNCEntry{
			Entry:       entry,
			CachedAt:    time.Now(),
			LastAccess:  time.Now(),
			AccessCount: 0,
			Source:      "bulk_write",
			Version:     "1.0",
		}

		data, err := json.Marshal(cached)
		if err != nil {
			c.logger.Warn("bulk set marshal error", 
				zap.String("phone", normalizedPhone), 
				zap.Error(err))
			continue
		}

		// Compress if needed
		if c.config.CompressionEnabled && len(data) > c.config.CompressionThreshold {
			data = c.compressData(data)
			c.incrementMetric("compressed_writes")
		}

		pipe.Set(ctx, key, data, DNCEntryTTL)

		// Add to bloom filter
		if c.config.BloomFilterEnabled && c.bloomFilter != nil {
			c.bloomFilter.Add(normalizedPhone)
		}
	}

	// Execute pipeline
	_, err := pipe.Exec(ctx)
	if err != nil {
		c.incrementMetric("errors")
		return fmt.Errorf("pipeline execution failed: %w", err)
	}

	return nil
}

// WarmCache pre-loads DNC entries into cache for improved performance
func (c *DNCCache) WarmCache(ctx context.Context, phoneNumbers []values.PhoneNumber, loadFunc func([]values.PhoneNumber) ([]*dnc.DNCEntry, error)) error {
	if len(phoneNumbers) == 0 {
		return nil
	}

	lockKey := DNCWarmingPrefix + "warming_lock"
	
	// Try to acquire warming lock
	locked, err := c.client.SetNX(ctx, lockKey, "warming", DNCWarmingLockTTL).Result()
	if err != nil {
		return fmt.Errorf("failed to acquire warming lock: %w", err)
	}
	if !locked {
		return fmt.Errorf("cache warming already in progress")
	}

	defer func() {
		c.client.Del(context.Background(), lockKey)
	}()

	c.logger.Info("starting DNC cache warming", zap.Int("phone_count", len(phoneNumbers)))

	// Process in batches
	batchSize := c.config.WarmingBatchSize
	warmed := 0
	errors := 0

	for i := 0; i < len(phoneNumbers); i += batchSize {
		end := i + batchSize
		if end > len(phoneNumbers) {
			end = len(phoneNumbers)
		}

		batch := phoneNumbers[i:end]
		
		// Load entries from source
		entries, err := loadFunc(batch)
		if err != nil {
			c.logger.Warn("cache warming batch load failed", 
				zap.Int("batch_start", i), 
				zap.Error(err))
			errors++
			continue
		}

		// Store in cache
		if err := c.BulkSetDNCEntries(ctx, entries); err != nil {
			c.logger.Warn("cache warming batch store failed", 
				zap.Int("batch_start", i), 
				zap.Error(err))
			errors++
			continue
		}

		warmed += len(entries)
		
		// Add small delay between batches to avoid overwhelming Redis
		time.Sleep(10 * time.Millisecond)
	}

	c.incrementMetric("warming_operations")
	c.logger.Info("DNC cache warming completed",
		zap.Int("warmed", warmed),
		zap.Int("errors", errors),
		zap.Int("total", len(phoneNumbers)))

	return nil
}

// InvalidateProvider removes all entries from a specific provider
func (c *DNCCache) InvalidateProvider(ctx context.Context, providerID string) error {
	pattern := DNCProviderPrefix + providerID + ":*"
	
	keys, err := c.scanKeys(ctx, pattern)
	if err != nil {
		return fmt.Errorf("failed to scan provider keys: %w", err)
	}

	if len(keys) == 0 {
		return nil
	}

	// Delete in batches
	batchSize := 1000
	for i := 0; i < len(keys); i += batchSize {
		end := i + batchSize
		if end > len(keys) {
			end = len(keys)
		}

		if err := c.client.Del(ctx, keys[i:end]...).Err(); err != nil {
			c.logger.Warn("provider invalidation batch failed", zap.Error(err))
		}
	}

	c.logger.Info("provider cache invalidated",
		zap.String("provider_id", providerID),
		zap.Int("keys_deleted", len(keys)))

	return nil
}

// InvalidateSource removes all entries from a specific source
func (c *DNCCache) InvalidateSource(ctx context.Context, source values.ListSource) error {
	pattern := DNCSourcePrefix + source.String() + ":*"
	
	keys, err := c.scanKeys(ctx, pattern)
	if err != nil {
		return fmt.Errorf("failed to scan source keys: %w", err)
	}

	if len(keys) == 0 {
		return nil
	}

	// Delete in batches
	batchSize := 1000
	for i := 0; i < len(keys); i += batchSize {
		end := i + batchSize
		if end > len(keys) {
			end = len(keys)
		}

		if err := c.client.Del(ctx, keys[i:end]...).Err(); err != nil {
			c.logger.Warn("source invalidation batch failed", zap.Error(err))
		}
	}

	// Reset bloom filter if invalidating significant portion
	if c.config.BloomFilterEnabled && len(keys) > 1000 {
		c.bloomFilter.Reset()
		c.logger.Info("bloom filter reset due to large invalidation")
	}

	c.logger.Info("source cache invalidated",
		zap.String("source", source.String()),
		zap.Int("keys_deleted", len(keys)))

	return nil
}

// GetMetrics returns current cache performance metrics
func (c *DNCCache) GetMetrics(ctx context.Context) DNCCacheMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()

	metrics := c.metrics
	
	// Calculate hit rate
	total := metrics.Hits + metrics.Misses
	if total > 0 {
		metrics.HitRate = float64(metrics.Hits) / float64(total)
	}

	return metrics
}

// GetCacheInfo returns detailed cache information for monitoring
func (c *DNCCache) GetCacheInfo(ctx context.Context) (map[string]interface{}, error) {
	info := make(map[string]interface{})

	// Get Redis info
	redisInfo, err := c.client.Info(ctx, "memory", "stats").Result()
	if err != nil {
		c.logger.Warn("failed to get redis info", zap.Error(err))
	} else {
		info["redis_info"] = redisInfo
	}

	// Get key counts by prefix
	keyCounts := make(map[string]int64)
	prefixes := []string{DNCEntryPrefix, DNCCheckPrefix, DNCProviderPrefix, DNCSourcePrefix}
	
	for _, prefix := range prefixes {
		count, err := c.countKeys(ctx, prefix+"*")
		if err != nil {
			c.logger.Warn("failed to count keys", zap.String("prefix", prefix), zap.Error(err))
			continue
		}
		keyCounts[prefix] = count
	}

	info["key_counts"] = keyCounts
	info["metrics"] = c.GetMetrics(ctx)
	info["config"] = c.config

	// Bloom filter info
	if c.config.BloomFilterEnabled && c.bloomFilter != nil {
		info["bloom_filter"] = map[string]interface{}{
			"size":        c.bloomFilter.size,
			"hash_count":  c.bloomFilter.hashCount,
			"estimated_items": c.bloomFilter.EstimatedItemCount(),
		}
	}

	return info, nil
}

// Close closes the DNC cache and cleans up resources
func (c *DNCCache) Close() error {
	if err := c.client.Close(); err != nil {
		c.logger.Error("DNC cache close failed", zap.Error(err))
		return fmt.Errorf("DNC cache close failed: %w", err)
	}

	c.logger.Info("DNC cache closed successfully")
	return nil
}

// Helper methods

// hashPhoneNumber creates a consistent hash for phone number caching
func (c *DNCCache) hashPhoneNumber(phoneNumber string) string {
	hash := md5.Sum([]byte(phoneNumber))
	return hex.EncodeToString(hash[:])
}

// shouldExtendTTL determines if TTL should be extended based on sliding expiration rate
func (c *DNCCache) shouldExtendTTL() bool {
	return time.Now().UnixNano()%1000 < int64(c.config.SlidingExpirationRate*1000)
}

// extendTTL extends the TTL of a key
func (c *DNCCache) extendTTL(ctx context.Context, key string, ttl time.Duration) {
	if err := c.client.Expire(ctx, key, ttl).Err(); err != nil {
		c.logger.Debug("failed to extend TTL", zap.String("key", key), zap.Error(err))
	}
}

// updateCachedEntry updates cached entry metadata
func (c *DNCCache) updateCachedEntry(ctx context.Context, key string, cached *CachedDNCEntry) {
	data, err := json.Marshal(cached)
	if err != nil {
		return
	}

	// Don't fail if this update fails - it's just metadata
	c.client.Set(ctx, key, data, DNCEntryTTL)
}

// updateCachedCheckResult updates cached check result metadata
func (c *DNCCache) updateCachedCheckResult(ctx context.Context, key string, cached *CachedCheckResult) {
	data, err := json.Marshal(cached)
	if err != nil {
		return
	}

	c.client.Set(ctx, key, data, DNCCheckTTL)
}

// compressData compresses data using a simple compression algorithm
// In production, you might want to use gzip or another compression library
func (c *DNCCache) compressData(data []byte) []byte {
	// Placeholder for compression - implement gzip or similar
	return data
}

// scanKeys scans for keys matching a pattern
func (c *DNCCache) scanKeys(ctx context.Context, pattern string) ([]string, error) {
	var keys []string
	iter := c.client.Scan(ctx, 0, pattern, 1000).Iterator()
	
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	
	return keys, iter.Err()
}

// countKeys counts keys matching a pattern
func (c *DNCCache) countKeys(ctx context.Context, pattern string) (int64, error) {
	keys, err := c.scanKeys(ctx, pattern)
	if err != nil {
		return 0, err
	}
	return int64(len(keys)), nil
}

// Metrics methods

func (c *DNCCache) incrementMetric(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	switch name {
	case "hits":
		c.metrics.Hits++
	case "misses":
		c.metrics.Misses++
	case "errors":
		c.metrics.Errors++
	case "bloom_filter_hits":
		c.metrics.BloomFilterHits++
	case "bloom_filter_false_positives":
		c.metrics.BloomFilterFalsePositives++
	case "warming_operations":
		c.metrics.WarmingOperations++
	case "compressed_writes":
		c.metrics.CompressedWrites++
	case "pipeline_operations":
		c.metrics.PipelineOperations++
	}
}

func (c *DNCCache) addMetric(name string, value int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	switch name {
	case "hits":
		c.metrics.Hits += value
	case "misses":
		c.metrics.Misses += value
	case "errors":
		c.metrics.Errors += value
	}
}

func (c *DNCCache) updateLatencyMetric(latencyMs float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Simple exponential moving average
	alpha := 0.1
	if c.metrics.AvgLatency == 0 {
		c.metrics.AvgLatency = latencyMs
	} else {
		c.metrics.AvgLatency = alpha*latencyMs + (1-alpha)*c.metrics.AvgLatency
	}
}

// Bloom Filter implementation

// NewBloomFilter creates a new bloom filter
func NewBloomFilter(size, hashCount uint64) *BloomFilter {
	return &BloomFilter{
		bits:      make([]byte, size/8+1),
		size:      size,
		hashCount: hashCount,
	}
}

// Add adds an item to the bloom filter
func (bf *BloomFilter) Add(item string) {
	bf.mu.Lock()
	defer bf.mu.Unlock()
	
	for i := uint64(0); i < bf.hashCount; i++ {
		hash := bf.hash(item, i)
		byteIndex := hash / 8
		bitIndex := hash % 8
		bf.bits[byteIndex] |= 1 << bitIndex
	}
}

// Contains checks if an item might be in the bloom filter
func (bf *BloomFilter) Contains(item string) bool {
	bf.mu.RLock()
	defer bf.mu.RUnlock()
	
	for i := uint64(0); i < bf.hashCount; i++ {
		hash := bf.hash(item, i)
		byteIndex := hash / 8
		bitIndex := hash % 8
		if bf.bits[byteIndex]&(1<<bitIndex) == 0 {
			return false
		}
	}
	return true
}

// Reset clears the bloom filter
func (bf *BloomFilter) Reset() {
	bf.mu.Lock()
	defer bf.mu.Unlock()
	
	for i := range bf.bits {
		bf.bits[i] = 0
	}
}

// EstimatedItemCount estimates the number of items in the bloom filter
func (bf *BloomFilter) EstimatedItemCount() int64 {
	bf.mu.RLock()
	defer bf.mu.RUnlock()
	
	setBits := 0
	for _, b := range bf.bits {
		for i := 0; i < 8; i++ {
			if b&(1<<i) != 0 {
				setBits++
			}
		}
	}
	
	// Estimate using bloom filter formula
	if setBits == 0 {
		return 0
	}
	
	ratio := float64(setBits) / float64(bf.size)
	if ratio >= 1.0 {
		return int64(bf.size) // Filter is full
	}
	
	// n = -m * ln(1 - X) / k
	// where m = bit array size, X = ratio of set bits, k = hash functions
	import "math"
	estimated := -float64(bf.size) * math.Log(1.0-ratio) / float64(bf.hashCount)
	return int64(estimated)
}

// hash computes hash for bloom filter
func (bf *BloomFilter) hash(item string, seed uint64) uint64 {
	// Simple hash function - in production use a better hash like murmur3
	hash := uint64(0)
	for i, c := range item {
		hash = hash*31 + uint64(c) + seed*uint64(i+1)
	}
	return hash % bf.size
}