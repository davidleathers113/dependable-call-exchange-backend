package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/audit"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
)

// Key prefixes for audit cache
const (
	AuditEventPrefix     = "dce:audit:event:"
	AuditHashPrefix      = "dce:audit:hash:"
	AuditSequencePrefix  = "dce:audit:seq:"
	AuditLatestPrefix    = "dce:audit:latest:"
	AuditGapPrefix       = "dce:audit:gap:"
	AuditBatchPrefix     = "dce:audit:batch:"
	AuditStatsPrefix     = "dce:audit:stats:"
)

// TTL values for audit cache entries
const (
	EventCacheTTL      = 1 * time.Hour    // Recent events cached for 1 hour
	HashCacheTTL       = 24 * time.Hour   // Hash chain cached for 24 hours
	SequenceCacheTTL   = 1 * time.Hour    // Sequence numbers cached for 1 hour
	LatestCacheTTL     = 5 * time.Minute  // Latest hash cached for 5 minutes
	BatchCacheTTL      = 10 * time.Minute // Batch operations cached for 10 minutes
	StatsCacheTTL      = 5 * time.Minute  // Statistics cached for 5 minutes
)

// AuditCache provides high-performance caching for audit events and hash chains
// Following DCE patterns with cache-aside pattern and graceful degradation
type AuditCache struct {
	client     *redis.Client
	logger     *zap.Logger
	maxBatch   int                // Maximum batch size for operations
	warmupSize int                // Number of events to warm cache with
	lruSize    int                // Size of LRU cache for events
	ttlJitter  time.Duration      // Jitter to prevent cache stampede
	metrics    *AuditCacheMetrics // Cache performance metrics
}

// AuditCacheConfig holds configuration for the audit cache
type AuditCacheConfig struct {
	MaxBatchSize   int           // Maximum batch size (default: 100)
	WarmupSize     int           // Cache warmup size (default: 1000)
	LRUSize        int           // LRU cache size (default: 10000)
	TTLJitter      time.Duration // TTL jitter (default: 30s)
	EnableMetrics  bool          // Enable metrics collection
}

// AuditCacheMetrics tracks cache performance
type AuditCacheMetrics struct {
	hits   int64
	misses int64
	errors int64
}

// DefaultAuditCacheConfig returns default configuration
func DefaultAuditCacheConfig() *AuditCacheConfig {
	return &AuditCacheConfig{
		MaxBatchSize:  100,
		WarmupSize:    1000,
		LRUSize:       10000,
		TTLJitter:     30 * time.Second,
		EnableMetrics: true,
	}
}

// NewAuditCache creates a new audit cache instance
func NewAuditCache(client *redis.Client, logger *zap.Logger, config *AuditCacheConfig) (*AuditCache, error) {
	if client == nil {
		return nil, fmt.Errorf("redis client is required")
	}
	if logger == nil {
		return nil, fmt.Errorf("logger is required")
	}
	if config == nil {
		config = DefaultAuditCacheConfig()
	}

	cache := &AuditCache{
		client:     client,
		logger:     logger,
		maxBatch:   config.MaxBatchSize,
		warmupSize: config.WarmupSize,
		lruSize:    config.LRUSize,
		ttlJitter:  config.TTLJitter,
	}

	if config.EnableMetrics {
		cache.metrics = &AuditCacheMetrics{}
	}

	return cache, nil
}

// GetEvent retrieves an audit event from cache
func (ac *AuditCache) GetEvent(ctx context.Context, eventID uuid.UUID) (*audit.Event, error) {
	key := ac.eventKey(eventID)

	data, err := ac.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			ac.recordMiss()
			return nil, nil // Cache miss
		}
		ac.recordError()
		return nil, errors.NewInternalError("failed to get event from cache").WithCause(err)
	}

	var event audit.Event
	if err := json.Unmarshal(data, &event); err != nil {
		ac.recordError()
		return nil, errors.NewInternalError("failed to unmarshal cached event").WithCause(err)
	}

	ac.recordHit()
	return &event, nil
}

// SetEvent stores an audit event in cache with LRU eviction
func (ac *AuditCache) SetEvent(ctx context.Context, event *audit.Event) error {
	if event == nil {
		return errors.NewValidationError("INVALID_EVENT", "event cannot be nil")
	}

	key := ac.eventKey(event.ID)
	data, err := json.Marshal(event)
	if err != nil {
		ac.recordError()
		return errors.NewInternalError("failed to marshal event").WithCause(err)
	}

	ttl := ac.addJitter(EventCacheTTL)
	if err := ac.client.Set(ctx, key, data, ttl).Err(); err != nil {
		ac.recordError()
		return errors.NewInternalError("failed to cache event").WithCause(err)
	}

	// Update LRU tracking
	lruKey := ac.lruKey()
	if err := ac.client.ZAdd(ctx, lruKey, redis.Z{
		Score:  float64(time.Now().Unix()),
		Member: event.ID.String(),
	}).Err(); err != nil {
		// Log but don't fail
		ac.logger.Warn("failed to update LRU tracking", zap.Error(err))
	}

	// Trim LRU if needed
	if err := ac.client.ZRemRangeByRank(ctx, lruKey, 0, -int64(ac.lruSize)-1).Err(); err != nil {
		ac.logger.Warn("failed to trim LRU cache", zap.Error(err))
	}

	return nil
}

// GetEvents retrieves multiple events from cache (batch operation)
func (ac *AuditCache) GetEvents(ctx context.Context, eventIDs []uuid.UUID) (map[uuid.UUID]*audit.Event, error) {
	if len(eventIDs) == 0 {
		return make(map[uuid.UUID]*audit.Event), nil
	}

	// Limit batch size
	if len(eventIDs) > ac.maxBatch {
		eventIDs = eventIDs[:ac.maxBatch]
	}

	// Build keys
	keys := make([]string, len(eventIDs))
	for i, id := range eventIDs {
		keys[i] = ac.eventKey(id)
	}

	// Batch get
	results, err := ac.client.MGet(ctx, keys...).Result()
	if err != nil {
		ac.recordError()
		return nil, errors.NewInternalError("failed to batch get events").WithCause(err)
	}

	events := make(map[uuid.UUID]*audit.Event)
	for i, result := range results {
		if result == nil {
			ac.recordMiss()
			continue
		}

		data, ok := result.(string)
		if !ok {
			continue
		}

		var event audit.Event
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			ac.logger.Warn("failed to unmarshal cached event", 
				zap.String("event_id", eventIDs[i].String()),
				zap.Error(err))
			continue
		}

		events[eventIDs[i]] = &event
		ac.recordHit()
	}

	return events, nil
}

// SetEvents stores multiple events in cache (batch operation)
func (ac *AuditCache) SetEvents(ctx context.Context, events []*audit.Event) error {
	if len(events) == 0 {
		return nil
	}

	// Limit batch size
	if len(events) > ac.maxBatch {
		events = events[:ac.maxBatch]
	}

	// Use pipeline for efficiency
	pipe := ac.client.Pipeline()
	lruKey := ac.lruKey()
	now := float64(time.Now().Unix())
	ttl := ac.addJitter(EventCacheTTL)

	for _, event := range events {
		if event == nil {
			continue
		}

		key := ac.eventKey(event.ID)
		data, err := json.Marshal(event)
		if err != nil {
			ac.logger.Warn("failed to marshal event for batch set",
				zap.String("event_id", event.ID.String()),
				zap.Error(err))
			continue
		}

		pipe.Set(ctx, key, data, ttl)
		pipe.ZAdd(ctx, lruKey, redis.Z{Score: now, Member: event.ID.String()})
	}

	// Trim LRU
	pipe.ZRemRangeByRank(ctx, lruKey, 0, -int64(ac.lruSize)-1)

	if _, err := pipe.Exec(ctx); err != nil {
		ac.recordError()
		return errors.NewInternalError("failed to batch set events").WithCause(err)
	}

	return nil
}

// GetLatestHash retrieves the latest hash in the chain for fast validation
func (ac *AuditCache) GetLatestHash(ctx context.Context) (string, int64, error) {
	key := ac.latestHashKey()
	
	// Use MGET for atomic retrieval
	results, err := ac.client.MGet(ctx, key+":hash", key+":seq").Result()
	if err != nil {
		ac.recordError()
		return "", 0, errors.NewInternalError("failed to get latest hash").WithCause(err)
	}

	if results[0] == nil || results[1] == nil {
		ac.recordMiss()
		return "", 0, nil // Cache miss
	}

	hash, ok := results[0].(string)
	if !ok || hash == "" {
		return "", 0, nil
	}

	seqStr, ok := results[1].(string)
	if !ok {
		return "", 0, nil
	}

	seq, err := strconv.ParseInt(seqStr, 10, 64)
	if err != nil {
		return "", 0, errors.NewInternalError("invalid sequence number in cache").WithCause(err)
	}

	ac.recordHit()
	return hash, seq, nil
}

// SetLatestHash stores the latest hash and sequence number
func (ac *AuditCache) SetLatestHash(ctx context.Context, hash string, sequenceNum int64) error {
	if hash == "" {
		return errors.NewValidationError("INVALID_HASH", "hash cannot be empty")
	}

	key := ac.latestHashKey()
	ttl := ac.addJitter(LatestCacheTTL)

	// Use pipeline for atomic update
	pipe := ac.client.Pipeline()
	pipe.Set(ctx, key+":hash", hash, ttl)
	pipe.Set(ctx, key+":seq", strconv.FormatInt(sequenceNum, 10), ttl)
	
	if _, err := pipe.Exec(ctx); err != nil {
		ac.recordError()
		return errors.NewInternalError("failed to set latest hash").WithCause(err)
	}

	return nil
}

// GetSequenceNumber retrieves the current sequence number
func (ac *AuditCache) GetSequenceNumber(ctx context.Context) (int64, error) {
	key := ac.sequenceKey()
	
	val, err := ac.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			ac.recordMiss()
			return 0, nil // Start from 0
		}
		ac.recordError()
		return 0, errors.NewInternalError("failed to get sequence number").WithCause(err)
	}

	seq, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return 0, errors.NewInternalError("invalid sequence number").WithCause(err)
	}

	ac.recordHit()
	return seq, nil
}

// IncrementSequence atomically increments and returns the next sequence number
func (ac *AuditCache) IncrementSequence(ctx context.Context) (int64, error) {
	key := ac.sequenceKey()
	
	seq, err := ac.client.Incr(ctx, key).Result()
	if err != nil {
		ac.recordError()
		return 0, errors.NewInternalError("failed to increment sequence").WithCause(err)
	}

	// Set TTL on first use
	if seq == 1 {
		ac.client.Expire(ctx, key, ac.addJitter(SequenceCacheTTL))
	}

	return seq, nil
}

// GetHashChain retrieves a range of hashes for validation
func (ac *AuditCache) GetHashChain(ctx context.Context, fromSeq, toSeq int64) (map[int64]string, error) {
	if fromSeq > toSeq {
		return nil, errors.NewValidationError("INVALID_RANGE", "from sequence must be <= to sequence")
	}

	// Limit range size
	maxRange := int64(ac.maxBatch)
	if toSeq-fromSeq > maxRange {
		toSeq = fromSeq + maxRange
	}

	// Build keys
	keys := make([]string, 0, toSeq-fromSeq+1)
	for seq := fromSeq; seq <= toSeq; seq++ {
		keys = append(keys, ac.hashKey(seq))
	}

	// Batch get
	results, err := ac.client.MGet(ctx, keys...).Result()
	if err != nil {
		ac.recordError()
		return nil, errors.NewInternalError("failed to get hash chain").WithCause(err)
	}

	chain := make(map[int64]string)
	for i, result := range results {
		if result == nil {
			ac.recordMiss()
			continue
		}

		hash, ok := result.(string)
		if !ok || hash == "" {
			continue
		}

		seq := fromSeq + int64(i)
		chain[seq] = hash
		ac.recordHit()
	}

	return chain, nil
}

// SetHashChain stores a range of hashes
func (ac *AuditCache) SetHashChain(ctx context.Context, chain map[int64]string) error {
	if len(chain) == 0 {
		return nil
	}

	// Use pipeline for efficiency
	pipe := ac.client.Pipeline()
	ttl := ac.addJitter(HashCacheTTL)

	count := 0
	for seq, hash := range chain {
		if hash == "" {
			continue
		}

		key := ac.hashKey(seq)
		pipe.Set(ctx, key, hash, ttl)
		
		count++
		if count >= ac.maxBatch {
			break // Limit batch size
		}
	}

	if _, err := pipe.Exec(ctx); err != nil {
		ac.recordError()
		return errors.NewInternalError("failed to set hash chain").WithCause(err)
	}

	return nil
}

// TrackSequenceGap records a detected gap in sequence numbers
func (ac *AuditCache) TrackSequenceGap(ctx context.Context, startSeq, endSeq int64) error {
	key := ac.gapKey()
	member := fmt.Sprintf("%d-%d", startSeq, endSeq)
	score := float64(time.Now().Unix())

	if err := ac.client.ZAdd(ctx, key, redis.Z{
		Score:  score,
		Member: member,
	}).Err(); err != nil {
		ac.recordError()
		return errors.NewInternalError("failed to track sequence gap").WithCause(err)
	}

	// Set TTL
	ac.client.Expire(ctx, key, ac.addJitter(SequenceCacheTTL))

	return nil
}

// GetSequenceGaps retrieves tracked sequence gaps
func (ac *AuditCache) GetSequenceGaps(ctx context.Context, limit int) ([][2]int64, error) {
	key := ac.gapKey()
	
	// Get recent gaps
	results, err := ac.client.ZRevRange(ctx, key, 0, int64(limit-1)).Result()
	if err != nil {
		ac.recordError()
		return nil, errors.NewInternalError("failed to get sequence gaps").WithCause(err)
	}

	gaps := make([][2]int64, 0, len(results))
	for _, result := range results {
		var start, end int64
		if _, err := fmt.Sscanf(result, "%d-%d", &start, &end); err != nil {
			ac.logger.Warn("invalid gap format", zap.String("gap", result))
			continue
		}
		gaps = append(gaps, [2]int64{start, end})
	}

	return gaps, nil
}

// WarmCache pre-loads frequently accessed events into cache
func (ac *AuditCache) WarmCache(ctx context.Context, events []*audit.Event) error {
	if len(events) == 0 {
		return nil
	}

	// Batch process in chunks
	for i := 0; i < len(events); i += ac.maxBatch {
		end := i + ac.maxBatch
		if end > len(events) {
			end = len(events)
		}

		if err := ac.SetEvents(ctx, events[i:end]); err != nil {
			ac.logger.Warn("failed to warm cache batch",
				zap.Int("batch_start", i),
				zap.Int("batch_end", end),
				zap.Error(err))
			// Continue with other batches
		}
	}

	ac.logger.Info("cache warmed",
		zap.Int("total_events", len(events)),
		zap.Int("batch_size", ac.maxBatch))

	return nil
}

// InvalidateEvent removes an event from cache
func (ac *AuditCache) InvalidateEvent(ctx context.Context, eventID uuid.UUID) error {
	key := ac.eventKey(eventID)
	
	if err := ac.client.Del(ctx, key).Err(); err != nil {
		ac.recordError()
		return errors.NewInternalError("failed to invalidate event").WithCause(err)
	}

	// Remove from LRU tracking
	lruKey := ac.lruKey()
	ac.client.ZRem(ctx, lruKey, eventID.String())

	return nil
}

// InvalidateEvents removes multiple events from cache
func (ac *AuditCache) InvalidateEvents(ctx context.Context, eventIDs []uuid.UUID) error {
	if len(eventIDs) == 0 {
		return nil
	}

	// Build keys
	keys := make([]string, len(eventIDs))
	members := make([]interface{}, len(eventIDs))
	for i, id := range eventIDs {
		keys[i] = ac.eventKey(id)
		members[i] = id.String()
	}

	// Delete events
	if err := ac.client.Del(ctx, keys...).Err(); err != nil {
		ac.recordError()
		return errors.NewInternalError("failed to invalidate events").WithCause(err)
	}

	// Remove from LRU tracking
	lruKey := ac.lruKey()
	ac.client.ZRem(ctx, lruKey, members...)

	return nil
}

// GetCacheStats returns cache performance statistics
func (ac *AuditCache) GetCacheStats(ctx context.Context) (map[string]interface{}, error) {
	stats := map[string]interface{}{
		"hits":   ac.getHits(),
		"misses": ac.getMisses(),
		"errors": ac.getErrors(),
	}

	// Calculate hit rate
	total := stats["hits"].(int64) + stats["misses"].(int64)
	if total > 0 {
		stats["hit_rate"] = float64(stats["hits"].(int64)) / float64(total)
	} else {
		stats["hit_rate"] = 0.0
	}

	// Get Redis info
	info, err := ac.client.Info(ctx, "memory", "stats").Result()
	if err == nil {
		stats["redis_info"] = info
	}

	// Get LRU size
	lruKey := ac.lruKey()
	lruSize, err := ac.client.ZCard(ctx, lruKey).Result()
	if err == nil {
		stats["lru_size"] = lruSize
	}

	return stats, nil
}

// Key generation helpers
func (ac *AuditCache) eventKey(id uuid.UUID) string {
	return fmt.Sprintf("%s%s", AuditEventPrefix, id)
}

func (ac *AuditCache) hashKey(seq int64) string {
	return fmt.Sprintf("%s%d", AuditHashPrefix, seq)
}

func (ac *AuditCache) sequenceKey() string {
	return AuditSequencePrefix + "current"
}

func (ac *AuditCache) latestHashKey() string {
	return AuditLatestPrefix + "chain"
}

func (ac *AuditCache) gapKey() string {
	return AuditGapPrefix + "detected"
}

func (ac *AuditCache) lruKey() string {
	return AuditEventPrefix + "lru"
}

// Helper methods
func (ac *AuditCache) addJitter(ttl time.Duration) time.Duration {
	if ac.ttlJitter == 0 {
		return ttl
	}
	// Add random jitter up to configured amount
	jitter := time.Duration(time.Now().UnixNano() % int64(ac.ttlJitter))
	return ttl + jitter
}

// Metrics helpers
func (ac *AuditCache) recordHit() {
	if ac.metrics != nil {
		ac.metrics.hits++
	}
}

func (ac *AuditCache) recordMiss() {
	if ac.metrics != nil {
		ac.metrics.misses++
	}
}

func (ac *AuditCache) recordError() {
	if ac.metrics != nil {
		ac.metrics.errors++
	}
}

func (ac *AuditCache) getHits() int64 {
	if ac.metrics != nil {
		return ac.metrics.hits
	}
	return 0
}

func (ac *AuditCache) getMisses() int64 {
	if ac.metrics != nil {
		return ac.metrics.misses
	}
	return 0
}

func (ac *AuditCache) getErrors() int64 {
	if ac.metrics != nil {
		return ac.metrics.errors
	}
	return 0
}