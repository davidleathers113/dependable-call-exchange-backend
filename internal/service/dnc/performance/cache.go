package performance

import (
	"container/list"
	"crypto/sha256"
	"fmt"
	"hash"
	"math"
	"sync"
	"time"

	"go.uber.org/zap"
)

// L1Cache implements a high-performance in-memory cache with LRU eviction
type L1Cache struct {
	logger *zap.Logger
	config *L1CacheConfig
	
	// Cache storage
	items  map[string]*list.Element
	lru    *list.List
	mutex  sync.RWMutex
	
	// Statistics
	stats     *L1CacheStats
	statsMutex sync.RWMutex
	
	// Cleanup
	stopCleanup chan struct{}
	cleanupDone chan struct{}
}

// L1CacheConfig configures the L1 cache
type L1CacheConfig struct {
	Size           int
	TTL            time.Duration
	CleanupInterval time.Duration
	EvictionPolicy  EvictionPolicy
}

// EvictionPolicy defines cache eviction strategies
type EvictionPolicy int

const (
	EvictionPolicyLRU EvictionPolicy = iota
	EvictionPolicyLFU
	EvictionPolicyTTL
	EvictionPolicyRandom
)

// cacheItem represents an item in the cache
type cacheItem struct {
	key        string
	value      interface{}
	expiry     time.Time
	accessCount int64
	lastAccess time.Time
}

// NewL1Cache creates a new L1 cache
func NewL1Cache(config *L1CacheConfig, logger *zap.Logger) *L1Cache {
	cache := &L1Cache{
		logger:      logger,
		config:      config,
		items:       make(map[string]*list.Element),
		lru:         list.New(),
		stats:       &L1CacheStats{MaxSize: config.Size},
		stopCleanup: make(chan struct{}),
		cleanupDone: make(chan struct{}),
	}
	
	// Start cleanup goroutine
	go cache.runCleanup()
	
	return cache
}

// Get retrieves a value from the cache
func (c *L1Cache) Get(key string) (interface{}, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	
	element, exists := c.items[key]
	if !exists {
		c.updateStats(false, 0)
		return nil, false
	}
	
	item := element.Value.(*cacheItem)
	
	// Check expiry
	if time.Now().After(item.expiry) {
		c.updateStats(false, 0)
		// Remove expired item (will be cleaned up later)
		return nil, false
	}
	
	// Update access statistics
	item.accessCount++
	item.lastAccess = time.Now()
	
	// Move to front for LRU
	c.lru.MoveToFront(element)
	
	c.updateStats(true, time.Since(item.lastAccess))
	return item.value, true
}

// Set stores a value in the cache
func (c *L1Cache) Set(key string, value interface{}, ttl time.Duration) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	now := time.Now()
	expiry := now.Add(ttl)
	if ttl == 0 {
		expiry = now.Add(c.config.TTL)
	}
	
	// Check if item already exists
	if element, exists := c.items[key]; exists {
		// Update existing item
		item := element.Value.(*cacheItem)
		item.value = value
		item.expiry = expiry
		item.lastAccess = now
		c.lru.MoveToFront(element)
		return
	}
	
	// Create new item
	item := &cacheItem{
		key:        key,
		value:      value,
		expiry:     expiry,
		accessCount: 1,
		lastAccess: now,
	}
	
	// Check if we need to evict
	if len(c.items) >= c.config.Size {
		c.evictOne()
	}
	
	// Add new item
	element := c.lru.PushFront(item)
	c.items[key] = element
	
	c.statsMutex.Lock()
	c.stats.Size++
	c.statsMutex.Unlock()
}

// Delete removes an item from the cache
func (c *L1Cache) Delete(key string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	if element, exists := c.items[key]; exists {
		c.removeElement(element)
	}
}

// Clear removes all items from the cache
func (c *L1Cache) Clear() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	c.items = make(map[string]*list.Element)
	c.lru.Init()
	
	c.statsMutex.Lock()
	c.stats.Size = 0
	c.statsMutex.Unlock()
}

// Cleanup removes expired items
func (c *L1Cache) Cleanup() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	now := time.Now()
	var toRemove []*list.Element
	
	// Find expired items
	for element := c.lru.Back(); element != nil; element = element.Prev() {
		item := element.Value.(*cacheItem)
		if now.After(item.expiry) {
			toRemove = append(toRemove, element)
		}
	}
	
	// Remove expired items
	for _, element := range toRemove {
		c.removeElement(element)
	}
	
	c.statsMutex.Lock()
	c.stats.EvictionCount += int64(len(toRemove))
	c.statsMutex.Unlock()
	
	if len(toRemove) > 0 {
		c.logger.Debug("Cleaned up expired cache items",
			zap.Int("removed", len(toRemove)),
			zap.Int("remaining", len(c.items)),
		)
	}
}

// GetStats returns cache statistics
func (c *L1Cache) GetStats() *L1CacheStats {
	c.statsMutex.RLock()
	defer c.statsMutex.RUnlock()
	
	stats := *c.stats
	stats.Size = len(c.items)
	
	if stats.TotalQueries > 0 {
		stats.HitRate = float64(stats.TotalHits) / float64(stats.TotalQueries) * 100.0
	}
	
	return &stats
}

// Stop shuts down the cache cleanup goroutine
func (c *L1Cache) Stop() {
	close(c.stopCleanup)
	<-c.cleanupDone
}

// evictOne removes one item based on eviction policy
func (c *L1Cache) evictOne() {
	if len(c.items) == 0 {
		return
	}
	
	var elementToRemove *list.Element
	
	switch c.config.EvictionPolicy {
	case EvictionPolicyLRU:
		elementToRemove = c.lru.Back()
	case EvictionPolicyLFU:
		elementToRemove = c.findLFU()
	case EvictionPolicyTTL:
		elementToRemove = c.findEarliestExpiry()
	case EvictionPolicyRandom:
		elementToRemove = c.findRandom()
	default:
		elementToRemove = c.lru.Back() // Default to LRU
	}
	
	if elementToRemove != nil {
		c.removeElement(elementToRemove)
		c.statsMutex.Lock()
		c.stats.EvictionCount++
		c.statsMutex.Unlock()
	}
}

// removeElement removes an element from both the map and list
func (c *L1Cache) removeElement(element *list.Element) {
	item := element.Value.(*cacheItem)
	delete(c.items, item.key)
	c.lru.Remove(element)
	
	c.statsMutex.Lock()
	c.stats.Size--
	c.statsMutex.Unlock()
}

// findLFU finds the least frequently used item
func (c *L1Cache) findLFU() *list.Element {
	var lfuElement *list.Element
	var minCount int64 = -1
	
	for element := c.lru.Back(); element != nil; element = element.Prev() {
		item := element.Value.(*cacheItem)
		if minCount == -1 || item.accessCount < minCount {
			minCount = item.accessCount
			lfuElement = element
		}
	}
	
	return lfuElement
}

// findEarliestExpiry finds the item with earliest expiry
func (c *L1Cache) findEarliestExpiry() *list.Element {
	var earliestElement *list.Element
	var earliestTime time.Time
	
	for element := c.lru.Back(); element != nil; element = element.Prev() {
		item := element.Value.(*cacheItem)
		if earliestElement == nil || item.expiry.Before(earliestTime) {
			earliestTime = item.expiry
			earliestElement = element
		}
	}
	
	return earliestElement
}

// findRandom finds a random item for eviction
func (c *L1Cache) findRandom() *list.Element {
	if c.lru.Len() == 0 {
		return nil
	}
	
	// Simple random selection - return middle element
	count := c.lru.Len() / 2
	element := c.lru.Front()
	for i := 0; i < count && element != nil; i++ {
		element = element.Next()
	}
	
	return element
}

// updateStats updates cache statistics
func (c *L1Cache) updateStats(hit bool, latency time.Duration) {
	c.statsMutex.Lock()
	defer c.statsMutex.Unlock()
	
	c.stats.TotalQueries++
	if hit {
		c.stats.TotalHits++
	} else {
		c.stats.TotalMisses++
	}
	
	// Update average latency
	if c.stats.AverageLatency == 0 {
		c.stats.AverageLatency = latency
	} else {
		c.stats.AverageLatency = (c.stats.AverageLatency + latency) / 2
	}
}

// runCleanup runs periodic cleanup
func (c *L1Cache) runCleanup() {
	defer close(c.cleanupDone)
	
	interval := c.config.CleanupInterval
	if interval == 0 {
		interval = time.Minute
	}
	
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	
	for {
		select {
		case <-c.stopCleanup:
			return
		case <-ticker.C:
			c.Cleanup()
		}
	}
}

// BloomFilter implements a space-efficient probabilistic data structure
type BloomFilter struct {
	logger *zap.Logger
	config *BloomFilterConfig
	
	// Bloom filter storage
	bitArray []uint64
	size     uint
	hashFuncs uint
	mutex    sync.RWMutex
	
	// Statistics
	stats     *BloomFilterStats
	statsMutex sync.RWMutex
	
	// Hash functions
	hashers []hash.Hash
}

// BloomFilterConfig configures the bloom filter
type BloomFilterConfig struct {
	Size   uint // Size of bit array
	Hashes uint // Number of hash functions
}

// NewBloomFilter creates a new bloom filter
func NewBloomFilter(config *BloomFilterConfig, logger *zap.Logger) *BloomFilter {
	// Calculate optimal size for bit array (in uint64 words)
	arraySize := (config.Size + 63) / 64
	
	bf := &BloomFilter{
		logger:    logger,
		config:    config,
		bitArray:  make([]uint64, arraySize),
		size:      config.Size,
		hashFuncs: config.Hashes,
		stats: &BloomFilterStats{
			Size:          config.Size,
			HashFunctions: config.Hashes,
		},
		hashers: make([]hash.Hash, config.Hashes),
	}
	
	// Initialize hash functions
	for i := uint(0); i < config.Hashes; i++ {
		bf.hashers[i] = sha256.New()
	}
	
	return bf
}

// Add adds an item to the bloom filter
func (bf *BloomFilter) Add(item string) {
	bf.mutex.Lock()
	defer bf.mutex.Unlock()
	
	data := []byte(item)
	
	for i := uint(0); i < bf.hashFuncs; i++ {
		bf.hashers[i].Reset()
		bf.hashers[i].Write(data)
		bf.hashers[i].Write([]byte{byte(i)}) // Salt with index
		
		hash := bf.hashers[i].Sum(nil)
		hashValue := bf.bytesToUint64(hash) % uint64(bf.size)
		
		wordIndex := hashValue / 64
		bitIndex := hashValue % 64
		
		bf.bitArray[wordIndex] |= 1 << bitIndex
	}
	
	bf.statsMutex.Lock()
	bf.stats.TotalAdds++
	bf.stats.EstimatedItems++
	bf.statsMutex.Unlock()
}

// MayContain checks if an item might be in the set
func (bf *BloomFilter) MayContain(item string) bool {
	bf.mutex.RLock()
	defer bf.mutex.RUnlock()
	
	data := []byte(item)
	
	for i := uint(0); i < bf.hashFuncs; i++ {
		bf.hashers[i].Reset()
		bf.hashers[i].Write(data)
		bf.hashers[i].Write([]byte{byte(i)}) // Salt with index
		
		hash := bf.hashers[i].Sum(nil)
		hashValue := bf.bytesToUint64(hash) % uint64(bf.size)
		
		wordIndex := hashValue / 64
		bitIndex := hashValue % 64
		
		if bf.bitArray[wordIndex]&(1<<bitIndex) == 0 {
			bf.updateStats(false)
			return false
		}
	}
	
	bf.updateStats(true)
	return true
}

// Clear resets the bloom filter
func (bf *BloomFilter) Clear() {
	bf.mutex.Lock()
	defer bf.mutex.Unlock()
	
	for i := range bf.bitArray {
		bf.bitArray[i] = 0
	}
	
	bf.statsMutex.Lock()
	bf.stats.EstimatedItems = 0
	bf.stats.TotalAdds = 0
	bf.stats.TotalChecks = 0
	bf.statsMutex.Unlock()
}

// GetStats returns bloom filter statistics
func (bf *BloomFilter) GetStats() *BloomFilterStats {
	bf.statsMutex.RLock()
	defer bf.statsMutex.RUnlock()
	
	stats := *bf.stats
	
	// Calculate false positive rate
	if stats.EstimatedItems > 0 {
		k := float64(bf.hashFuncs)
		n := float64(stats.EstimatedItems)
		m := float64(bf.size)
		
		// Formula: (1 - e^(-kn/m))^k
		exponent := -k * n / m
		stats.FalsePositiveRate = math.Pow(1.0-math.Exp(exponent), k)
	}
	
	// Estimate memory usage
	stats.MemoryUsage = int64(len(bf.bitArray) * 8) // 8 bytes per uint64
	
	return &stats
}

// bytesToUint64 converts hash bytes to uint64
func (bf *BloomFilter) bytesToUint64(data []byte) uint64 {
	var result uint64
	for i := 0; i < 8 && i < len(data); i++ {
		result |= uint64(data[i]) << (i * 8)
	}
	return result
}

// updateStats updates bloom filter statistics
func (bf *BloomFilter) updateStats(mayContain bool) {
	bf.statsMutex.Lock()
	defer bf.statsMutex.Unlock()
	
	bf.stats.TotalChecks++
	// Note: We can't distinguish true positives from false positives
	// without additional information
}

// MemoryPool manages reusable memory blocks
type MemoryPool struct {
	logger *zap.Logger
	config *MemoryPoolConfig
	
	// Pool storage
	pool   sync.Pool
	blocks []*MemoryBlock
	mutex  sync.RWMutex
	
	// Statistics
	stats      *MemoryPoolStats
	statsMutex sync.RWMutex
}

// MemoryPoolConfig configures the memory pool
type MemoryPoolConfig struct {
	PoolSize  int
	BlockSize int
}

// MemoryBlock represents a reusable memory block
type MemoryBlock struct {
	Data     []byte
	InUse    bool
	Created  time.Time
	LastUsed time.Time
}

// NewMemoryPool creates a new memory pool
func NewMemoryPool(config *MemoryPoolConfig, logger *zap.Logger) *MemoryPool {
	mp := &MemoryPool{
		logger: logger,
		config: config,
		blocks: make([]*MemoryBlock, 0, config.PoolSize),
		stats: &MemoryPoolStats{
			Total: config.PoolSize,
		},
	}
	
	// Initialize sync.Pool
	mp.pool = sync.Pool{
		New: func() interface{} {
			return &MemoryBlock{
				Data:    make([]byte, config.BlockSize),
				Created: time.Now(),
			}
		},
	}
	
	// Pre-allocate blocks
	for i := 0; i < config.PoolSize; i++ {
		block := mp.pool.Get().(*MemoryBlock)
		mp.blocks = append(mp.blocks, block)
		mp.pool.Put(block)
	}
	
	return mp
}

// Get retrieves a memory block from the pool
func (mp *MemoryPool) Get() *MemoryBlock {
	block := mp.pool.Get().(*MemoryBlock)
	block.InUse = true
	block.LastUsed = time.Now()
	
	mp.statsMutex.Lock()
	mp.stats.Allocated++
	mp.stats.TotalAllocs++
	mp.statsMutex.Unlock()
	
	return block
}

// Put returns a memory block to the pool
func (mp *MemoryPool) Put(block *MemoryBlock) {
	if block == nil {
		return
	}
	
	block.InUse = false
	mp.pool.Put(block)
	
	mp.statsMutex.Lock()
	mp.stats.Allocated--
	mp.stats.TotalFrees++
	mp.statsMutex.Unlock()
}

// GetStats returns memory pool statistics
func (mp *MemoryPool) GetStats() *MemoryPoolStats {
	mp.statsMutex.RLock()
	defer mp.statsMutex.RUnlock()
	
	stats := *mp.stats
	stats.Available = stats.Total - stats.Allocated
	stats.BytesInUse = int64(stats.Allocated * mp.config.BlockSize)
	stats.BytesTotal = int64(stats.Total * mp.config.BlockSize)
	
	return &stats
}