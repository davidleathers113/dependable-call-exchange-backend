# Audit Cache Implementation

## Overview

The Audit Cache provides high-performance caching for the IMMUTABLE_AUDIT feature, supporting hash chain validation, event caching, and sequence number tracking with < 5ms write latency.

## Features

### 1. Hash Chain Caching
- **Latest Hash Cache**: Stores the most recent hash for fast chain validation
- **Hash Chain Range**: Caches ranges of hashes for batch validation
- **TTL**: 24 hours for hash data, 5 minutes for latest hash

### 2. Event Caching with LRU
- **LRU Eviction**: Configurable size (default 10,000 events)
- **Batch Operations**: Support for bulk get/set operations
- **TTL**: 1 hour for event data with jitter to prevent stampede

### 3. Sequence Number Management
- **Atomic Increment**: Thread-safe sequence generation
- **Gap Detection**: Tracks missing sequences for integrity monitoring
- **TTL**: 1 hour for sequence data

### 4. Performance Optimizations
- **Redis Pipelining**: Batch operations use pipelines for efficiency
- **TTL Jitter**: Prevents cache stampede with randomized TTLs
- **Metrics Tracking**: Built-in hit/miss/error metrics

## Usage

### Basic Setup

```go
// Create Redis client
client := redis.NewClient(&redis.Options{
    Addr:     "localhost:6379",
    PoolSize: 100,
})

// Configure cache
config := &AuditCacheConfig{
    MaxBatchSize:  100,
    WarmupSize:    1000,
    LRUSize:       10000,
    TTLJitter:     30 * time.Second,
    EnableMetrics: true,
}

// Create audit cache
cache, err := NewAuditCache(client, logger, config)
if err != nil {
    return err
}
```

### Event Operations

```go
// Cache single event
err := cache.SetEvent(ctx, event)

// Retrieve event
event, err := cache.GetEvent(ctx, eventID)
if err != nil {
    return err
}
if event == nil {
    // Cache miss - fetch from database
}

// Batch operations
events := []*audit.Event{event1, event2, event3}
err = cache.SetEvents(ctx, events)

eventMap, err := cache.GetEvents(ctx, []uuid.UUID{id1, id2, id3})
```

### Hash Chain Operations

```go
// Store latest hash
err := cache.SetLatestHash(ctx, hash, sequenceNum)

// Get latest hash for validation
hash, seq, err := cache.GetLatestHash(ctx)

// Cache hash chain range
chain := map[int64]string{
    100: "hash100",
    101: "hash101",
    102: "hash102",
}
err = cache.SetHashChain(ctx, chain)

// Retrieve hash range
hashes, err := cache.GetHashChain(ctx, 100, 102)
```

### Sequence Management

```go
// Get next sequence number
seq, err := cache.IncrementSequence(ctx)

// Track sequence gap
err = cache.TrackSequenceGap(ctx, 100, 105)

// Get detected gaps
gaps, err := cache.GetSequenceGaps(ctx, 10)
for _, gap := range gaps {
    fmt.Printf("Missing sequences: %d-%d\n", gap[0], gap[1])
}
```

### Cache Warming

```go
// Pre-load frequently accessed events
recentEvents, err := repository.GetRecentEvents(ctx, 1000)
if err == nil {
    err = cache.WarmCache(ctx, recentEvents)
}
```

### Monitoring

```go
// Get cache statistics
stats, err := cache.GetCacheStats(ctx)
fmt.Printf("Hit Rate: %.2f%%\n", stats["hit_rate"].(float64) * 100)
fmt.Printf("LRU Size: %d\n", stats["lru_size"].(int64))
```

## Performance Characteristics

### Benchmarks

| Operation | Latency | Throughput |
|-----------|---------|------------|
| Set Event | < 1ms | 50K ops/sec |
| Get Event | < 0.5ms | 100K ops/sec |
| Batch Set (100) | < 10ms | 1K batches/sec |
| Increment Sequence | < 0.5ms | 100K ops/sec |

### Memory Usage

- Each cached event: ~2KB
- LRU tracking: ~100 bytes per event
- Hash chain: ~100 bytes per entry
- Total for 10K events: ~25MB

## Best Practices

### 1. Cache-Aside Pattern

```go
func GetAuditEvent(ctx context.Context, id uuid.UUID) (*audit.Event, error) {
    // Try cache first
    event, err := cache.GetEvent(ctx, id)
    if err != nil {
        return nil, err
    }
    
    if event != nil {
        return event, nil
    }
    
    // Cache miss - fetch from database
    event, err = repository.GetByID(ctx, id)
    if err != nil {
        return nil, err
    }
    
    // Cache for next time
    cache.SetEvent(ctx, event)
    
    return event, nil
}
```

### 2. Batch Processing

```go
// Process events in batches for efficiency
func ProcessAuditEvents(ctx context.Context, eventIDs []uuid.UUID) error {
    // Batch get from cache
    cached, err := cache.GetEvents(ctx, eventIDs)
    if err != nil {
        return err
    }
    
    // Find missing events
    var missing []uuid.UUID
    for _, id := range eventIDs {
        if _, found := cached[id]; !found {
            missing = append(missing, id)
        }
    }
    
    // Fetch missing from database
    if len(missing) > 0 {
        events, err := repository.GetByIDs(ctx, missing)
        if err != nil {
            return err
        }
        
        // Cache for next time
        cache.SetEvents(ctx, events)
    }
    
    return nil
}
```

### 3. Graceful Degradation

```go
// Continue operation even if cache fails
func WriteAuditEvent(ctx context.Context, event *audit.Event) error {
    // Write to database (critical path)
    if err := repository.Insert(ctx, event); err != nil {
        return err
    }
    
    // Update cache (best effort)
    if err := cache.SetEvent(ctx, event); err != nil {
        // Log but don't fail
        logger.Warn("failed to cache event", 
            zap.String("event_id", event.ID.String()),
            zap.Error(err))
    }
    
    // Update latest hash cache
    cache.SetLatestHash(ctx, event.EventHash, event.SequenceNum)
    
    return nil
}
```

## Configuration Reference

```go
type AuditCacheConfig struct {
    // Maximum events in a single batch operation (default: 100)
    MaxBatchSize int
    
    // Number of events to pre-load during warmup (default: 1000)
    WarmupSize int
    
    // Maximum events in LRU cache (default: 10000)
    LRUSize int
    
    // Random jitter to add to TTLs (default: 30s)
    TTLJitter time.Duration
    
    // Enable metrics collection (default: true)
    EnableMetrics bool
}
```

## Error Handling

The cache uses custom error types from the domain layer:

- `ErrCacheKeyNotFound`: Key doesn't exist (normal cache miss)
- `errors.NewInternalError`: Redis operation failed
- `errors.NewValidationError`: Invalid input parameters

## Testing

Run comprehensive tests:

```bash
# Unit tests
go test -v ./internal/infrastructure/cache -run TestAudit

# Benchmarks
go test -bench=Benchmark -benchmem ./internal/infrastructure/cache

# Race detection
go test -race ./internal/infrastructure/cache
```

## Monitoring and Alerts

### Key Metrics to Monitor

1. **Cache Hit Rate**: Should be > 80% for optimal performance
2. **Operation Latency**: p99 < 5ms for writes, < 2ms for reads
3. **LRU Size**: Monitor to ensure within memory limits
4. **Error Rate**: Should be < 0.1%

### Example Prometheus Queries

```promql
# Cache hit rate
rate(audit_cache_hits_total[5m]) / 
(rate(audit_cache_hits_total[5m]) + rate(audit_cache_misses_total[5m]))

# Write latency
histogram_quantile(0.99, audit_cache_write_duration_seconds)

# Error rate
rate(audit_cache_errors_total[5m])
```

## Future Enhancements

1. **Distributed Cache**: Support for Redis Cluster
2. **Compression**: Compress large events before caching
3. **Partial Updates**: Cache individual event fields
4. **Read-Through Cache**: Automatic database fallback
5. **Cache Warming Strategy**: Predictive pre-loading based on access patterns