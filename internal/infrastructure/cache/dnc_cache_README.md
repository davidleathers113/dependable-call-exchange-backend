# DNC Cache Implementation

## Overview

The DNC (Do Not Call) Cache provides high-performance caching for DNC lookups with sub-millisecond cache hits and 99%+ hit rate targets. This implementation uses Redis with advanced optimization techniques including bloom filters, pipeline operations, compression, and distributed cache consistency mechanisms.

## Features

### Core Functionality
- **High-Performance Lookups**: Sub-millisecond cache hits for phone number DNC status
- **Bloom Filter**: Probabilistic negative lookups to reduce Redis load
- **Pipeline Operations**: Bulk operations for improved throughput
- **Compression**: Automatic compression for large payloads
- **TTL Management**: Sliding expiration and intelligent cache warming

### Caching Strategies
- **Write-Through**: DNC entries are immediately cached on write
- **Read-Through**: Check results are cached on read
- **Cache Warming**: Proactive loading of frequently accessed entries
- **Invalidation**: Provider and source-specific cache invalidation

### Performance Optimizations
- **Connection Pooling**: Optimized Redis connection management
- **Hash-Based Keys**: Consistent phone number hashing for distribution
- **Metrics Tracking**: Comprehensive performance monitoring
- **Failover Support**: Redis clustering and failover mechanisms

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      DNC Cache Layer                        │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐        │
│  │ Bloom Filter│  │  Pipeline   │  │ Compression │        │
│  │   (Negative │  │ Operations  │  │   Engine    │        │
│  │   Lookups)  │  │             │  │             │        │
│  └─────────────┘  └─────────────┘  └─────────────┘        │
├─────────────────────────────────────────────────────────────┤
│                    Redis Client Pool                        │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐        │
│  │DNC Entries  │  │Check Results│  │  Provider   │        │
│  │   Cache     │  │    Cache    │  │    Cache    │        │
│  └─────────────┘  └─────────────┘  └─────────────┘        │
└─────────────────────────────────────────────────────────────┘
```

## Key Components

### DNCCache
The main cache interface providing:
- `GetDNCEntry(ctx, phoneNumber)` - Retrieve cached DNC entry
- `SetDNCEntry(ctx, entry)` - Store DNC entry with write-through
- `GetCheckResult(ctx, phoneNumber)` - Retrieve cached check result
- `SetCheckResult(ctx, result)` - Store check result
- `BulkGetDNCEntries(ctx, phoneNumbers)` - Bulk retrieval operations
- `BulkSetDNCEntries(ctx, entries)` - Bulk storage operations
- `WarmCache(ctx, phoneNumbers, loadFunc)` - Proactive cache warming
- `InvalidateProvider(ctx, providerID)` - Provider-specific invalidation
- `InvalidateSource(ctx, source)` - Source-specific invalidation

### BloomFilter
Probabilistic data structure for negative lookups:
- Reduces Redis load for non-existent phone numbers
- Configurable false positive rate (default: 1%)
- 1M bit array supports ~100K phone numbers
- Thread-safe with RWMutex protection

### Cache Keys
Organized with consistent prefixes:
- `dce:dnc:entry:` - Individual DNC entries
- `dce:dnc:check:` - Check results
- `dce:dnc:bloom:` - Bloom filter data
- `dce:dnc:provider:` - Provider-specific caches
- `dce:dnc:source:` - Source-specific caches
- `dce:dnc:stats:` - Performance statistics

## Configuration

### DNCCacheConfig
```go
type DNCCacheConfig struct {
    BloomFilterEnabled    bool    // Enable bloom filter (default: true)
    BloomFilterSize       uint64  // Bit array size (default: 1M)
    BloomFilterHashCount  uint64  // Hash functions (default: 7)
    CompressionEnabled    bool    // Enable compression (default: true)
    CompressionThreshold  int     // Compress if > bytes (default: 1024)
    WarmingBatchSize      int     // Warming batch size (default: 1000)
    PipelineBatchSize     int     // Pipeline batch size (default: 100)
    SlidingExpirationRate float64 // TTL extension rate (default: 0.1)
}
```

### TTL Values
- `DNCEntryTTL`: 24 hours - DNC entries cache duration
- `DNCCheckTTL`: 6 hours - Check results cache duration
- `DNCBloomTTL`: 12 hours - Bloom filter refresh interval
- `DNCProviderTTL`: 2 hours - Provider data cache duration
- `DNCNegativeTTL`: 30 minutes - Negative lookup cache duration

## Performance Targets

| Metric | Target | Implementation |
|--------|--------|----------------|
| Cache Hit Latency | < 1ms | Pipeline operations, connection pooling |
| Cache Hit Rate | > 99% | Bloom filter, intelligent warming |
| Throughput | 100K+ ops/sec | Bulk operations, compression |
| Memory Efficiency | < 100MB/1M entries | Hash-based keys, compression |
| Failover Time | < 5s | Redis clustering, connection management |

## Usage Examples

### Basic Operations
```go
// Initialize cache
cache, err := NewDNCCache(redisConfig, logger)
if err != nil {
    return err
}
defer cache.Close()

// Store DNC entry
entry, _ := dnc.NewDNCEntry("+14155551234", "federal", "regulatory", userID)
err = cache.SetDNCEntry(ctx, entry)

// Retrieve DNC entry
phone, _ := values.NewPhoneNumber("+14155551234")
cachedEntry, err := cache.GetDNCEntry(ctx, phone)
if err != nil {
    // Handle cache miss
}

// Store check result
result, _ := dnc.NewDNCCheckResult("+14155551234")
err = cache.SetCheckResult(ctx, result)

// Retrieve check result
cachedResult, err := cache.GetCheckResult(ctx, phone)
```

### Bulk Operations
```go
// Bulk retrieval
phoneNumbers := []values.PhoneNumber{phone1, phone2, phone3}
entries, err := cache.BulkGetDNCEntries(ctx, phoneNumbers)

// Bulk storage
dncEntries := []*dnc.DNCEntry{entry1, entry2, entry3}
err = cache.BulkSetDNCEntries(ctx, dncEntries)
```

### Cache Warming
```go
// Define load function
loadFunc := func(phones []values.PhoneNumber) ([]*dnc.DNCEntry, error) {
    // Load from database or external API
    return repository.GetDNCEntries(ctx, phones)
}

// Warm cache with frequently accessed numbers
phoneNumbers := getFrequentlyAccessedNumbers()
err = cache.WarmCache(ctx, phoneNumbers, loadFunc)
```

### Cache Invalidation
```go
// Invalidate by provider
err = cache.InvalidateProvider(ctx, "provider-123")

// Invalidate by source
federalSource := values.MustNewListSource("federal")
err = cache.InvalidateSource(ctx, federalSource)
```

## Monitoring and Metrics

### Performance Metrics
The cache tracks comprehensive metrics:
```go
type DNCCacheMetrics struct {
    Hits                      int64   // Cache hits
    Misses                    int64   // Cache misses  
    Errors                    int64   // Operation errors
    HitRate                   float64 // Hit rate percentage
    AvgLatency                float64 // Average latency (ms)
    BloomFilterHits           int64   // Bloom filter hits
    BloomFilterFalsePositives int64   // False positives
    WarmingOperations         int64   // Cache warming ops
    CompressedWrites          int64   // Compressed writes
    PipelineOperations        int64   // Pipeline operations
}
```

### Accessing Metrics
```go
// Get current metrics
metrics := cache.GetMetrics(ctx)
fmt.Printf("Hit Rate: %.2f%%\n", metrics.HitRate*100)
fmt.Printf("Avg Latency: %.2f ms\n", metrics.AvgLatency)

// Get detailed cache info
info, err := cache.GetCacheInfo(ctx)
fmt.Printf("Total Keys: %v\n", info["key_counts"])
```

## Testing

### Unit Tests
- Cache operations (get/set/bulk)
- Bloom filter functionality
- Metrics tracking
- Error handling
- TTL and expiration

### Benchmark Tests
- Single operation latency
- Bulk operation throughput
- Memory usage patterns
- Concurrent access performance

### Integration Tests
- Redis connection handling
- Failover scenarios
- Cache warming strategies
- Invalidation mechanisms

### Running Tests
```bash
# Unit tests
go test ./internal/infrastructure/cache -v -run TestDNCCache

# Benchmarks  
go test ./internal/infrastructure/cache -bench=BenchmarkDNCCache -benchmem

# Integration tests with real Redis
REDIS_URL=localhost:6379 go test ./internal/infrastructure/cache -tags=integration
```

## Deployment Considerations

### Redis Configuration
```yaml
# Recommended Redis settings for DNC cache
redis:
  url: "redis-cluster:6379"
  password: "secure-password"  
  db: 1  # Dedicated DB for DNC cache
  pool_size: 100
  min_idle_conns: 20
  max_retries: 3
  dial_timeout: 5s
  read_timeout: 3s
  write_timeout: 3s
```

### Clustering Setup
- Use Redis Cluster for horizontal scaling
- Configure consistent hashing for phone numbers
- Set up monitoring for cluster health
- Implement backup and recovery procedures

### Memory Management
- Monitor Redis memory usage
- Configure appropriate TTL values
- Use compression for large payloads
- Implement cache eviction policies

### Security
- Use TLS for Redis connections
- Implement proper authentication
- Restrict network access to cache servers
- Encrypt sensitive phone number data

## Troubleshooting

### Common Issues

1. **High Cache Miss Rate**
   - Check bloom filter configuration
   - Verify cache warming strategy
   - Review TTL settings
   - Monitor cache invalidation patterns

2. **Slow Cache Operations**
   - Check Redis connection pool settings
   - Monitor network latency
   - Review pipeline batch sizes
   - Check for Redis memory pressure

3. **Memory Usage Issues**
   - Enable compression for large payloads
   - Adjust TTL values
   - Monitor key distribution
   - Check for memory leaks

### Debug Commands
```bash
# Check Redis connection
redis-cli -h localhost -p 6379 ping

# Monitor cache operations
redis-cli -h localhost -p 6379 monitor

# Check memory usage
redis-cli -h localhost -p 6379 info memory

# List DNC cache keys
redis-cli -h localhost -p 6379 keys "dce:dnc:*"
```

## Performance Tuning

### Optimization Guidelines

1. **Connection Pool Tuning**
   - Set pool_size to 2x expected concurrent connections
   - Configure min_idle_conns to handle baseline load
   - Adjust timeouts based on network conditions

2. **Batch Size Optimization**
   - Pipeline batch size: 50-200 operations
   - Warming batch size: 500-2000 entries
   - Monitor network utilization and adjust

3. **Bloom Filter Tuning**
   - Size: 8-10 bits per expected item
   - Hash count: ln(2) * (bits/items) ≈ 7 for 1% false positive
   - Monitor false positive rate

4. **Compression Settings**
   - Threshold: 512-2048 bytes (balance CPU vs network)
   - Algorithm: Consider gzip for better ratios
   - Monitor compression ratios

### Monitoring Alerts
```yaml
# Recommended alerting thresholds
alerts:
  - name: "DNC Cache Hit Rate Low"
    condition: "hit_rate < 95%"
    severity: "warning"
  
  - name: "DNC Cache Latency High" 
    condition: "avg_latency > 5ms"
    severity: "warning"
    
  - name: "DNC Cache Errors High"
    condition: "error_rate > 1%"
    severity: "critical"
```

## Future Enhancements

### Planned Features
- Multi-region cache replication
- Advanced compression algorithms
- Machine learning-based cache warming
- Distributed bloom filters
- Real-time cache analytics

### API Improvements
- Async operation support
- Stream-based bulk operations
- Custom serialization formats
- Advanced invalidation patterns

### Integration Features
- Prometheus metrics export
- Grafana dashboard templates
- OpenTelemetry tracing
- Circuit breaker patterns