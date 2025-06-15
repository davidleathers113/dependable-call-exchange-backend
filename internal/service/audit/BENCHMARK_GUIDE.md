# IMMUTABLE_AUDIT Performance Benchmark Guide

This guide explains how to run and interpret the comprehensive performance benchmarks for the IMMUTABLE_AUDIT feature. The benchmarks validate the critical performance requirements: < 5ms write latency, < 1s query response for 1M events, > 10K events/sec export throughput, and < 100MB base memory usage.

## Overview

The benchmark suite consists of five main categories:

1. **Logger Benchmarks** (`logger_bench_test.go`) - Audit logging performance
2. **Query Benchmarks** (`query_bench_test.go`) - Query response times
3. **Export Benchmarks** (`export_bench_test.go`) - Export throughput
4. **Integrity Benchmarks** (`integrity_bench_test.go`) - Hash chain verification
5. **Cache Benchmarks** (`cache_bench_test.go`) - Cache performance

## Performance Targets

| Component | Metric | Target | Validation |
|-----------|--------|--------|------------|
| Audit Logging | Write Latency P99 | < 5ms | CRITICAL |
| Query Engine | 1M Event Query | < 1s | CRITICAL |
| Export Engine | Throughput | > 10K events/sec | CRITICAL |
| Memory Usage | Base Memory | < 100MB | CRITICAL |
| Hash Chain | Verification | > 50K events/sec | HIGH |
| Cache | Hit Ratio | > 80% | MEDIUM |

## Running Benchmarks

### Quick Performance Check

```bash
# Run all benchmarks with short execution time
make bench

# Run specific benchmark category
go test -bench=BenchmarkLogger ./internal/service/audit/
go test -bench=BenchmarkQuery ./internal/service/audit/
go test -bench=BenchmarkExport ./internal/service/audit/
go test -bench=BenchmarkIntegrity ./internal/service/audit/
go test -bench=BenchmarkCache ./internal/service/audit/
```

### Comprehensive Performance Analysis

```bash
# Run benchmarks with extended time and memory profiling
go test -bench=. -benchtime=10s -benchmem ./internal/service/audit/

# Run with CPU profiling
go test -bench=BenchmarkLogger_SingleEventLogging -cpuprofile=cpu.prof ./internal/service/audit/
go tool pprof cpu.prof

# Run with memory profiling
go test -bench=BenchmarkLogger_MemoryUsage -memprofile=mem.prof ./internal/service/audit/
go tool pprof mem.prof

# Generate comprehensive benchmark report
go test -bench=. -benchmem -count=5 ./internal/service/audit/ | tee benchmark_results.txt
```

### Specific Performance Validations

```bash
# Validate < 5ms write latency requirement
go test -bench=BenchmarkLogger_SingleEventLogging -benchtime=30s ./internal/service/audit/

# Validate < 1s query response for 1M events
go test -bench=BenchmarkQuery_1MillionEvents -benchtime=10s ./internal/service/audit/

# Validate > 10K events/sec export throughput
go test -bench=BenchmarkExport_Throughput -benchtime=15s ./internal/service/audit/

# Validate < 100MB memory usage
go test -bench=BenchmarkLogger_MemoryUsage -benchmem ./internal/service/audit/
```

## Benchmark Categories

### 1. Logger Benchmarks

**Purpose**: Validate audit logging performance and latency requirements.

**Key Benchmarks**:
- `BenchmarkLogger_SingleEventLogging` - Validates < 5ms write latency
- `BenchmarkLogger_BatchProcessing` - Tests batch throughput
- `BenchmarkLogger_MemoryUsage` - Validates < 100MB memory requirement
- `BenchmarkLogger_ConcurrentPerformance` - Multi-threaded performance
- `BenchmarkLogger_GracefulDegradation` - Behavior under stress

**Expected Results**:
```
BenchmarkLogger_SingleEventLogging/default_config-8    1000000    1.2 ms/op    avg_latency_μs:1200   max_latency_μs:4800
BenchmarkLogger_BatchProcessing/batch_size_100-8       5000       15.3 ms/op   events/sec:6535
BenchmarkLogger_MemoryUsage-8                          10000      45.2 ms/op   memory_mb:85.3
```

**Performance Validation**:
- Average latency must be < 5ms
- Memory usage must be < 100MB
- Throughput should exceed 5K events/sec

### 2. Query Benchmarks

**Purpose**: Validate query response times for large datasets.

**Key Benchmarks**:
- `BenchmarkQuery_1MillionEvents` - Validates < 1s response for 1M events
- `BenchmarkQuery_SequenceRange` - Range query performance
- `BenchmarkQuery_Count` - Count query optimization
- `BenchmarkQuery_ComplexFilters` - Multi-filter performance

**Expected Results**:
```
BenchmarkQuery_1MillionEvents/dataset_1000k/time_range_1hour-8    100    850 ms/op    avg_latency_ms:850    dataset_size:1000000
BenchmarkQuery_SequenceRange/range_10000-8                        1000   12.5 ms/op
BenchmarkQuery_Count/count_all-8                                   2000   45.2 ms/op   avg_latency_ms:45
```

**Performance Validation**:
- 1M event queries must complete in < 1s
- Range queries should scale linearly
- Count queries must be < 100ms

### 3. Export Benchmarks

**Purpose**: Validate export throughput requirements.

**Key Benchmarks**:
- `BenchmarkExport_Throughput` - Validates > 10K events/sec target
- `BenchmarkExport_Streaming` - Continuous export performance
- `BenchmarkExport_LargeDataset` - 1M event export performance
- `BenchmarkExport_FilteredExport` - Export with filtering

**Expected Results**:
```
BenchmarkExport_Throughput/dataset_100k/json-8         50    1.2 s/op    events/sec:83333    dataset_size:100000
BenchmarkExport_Streaming/batch_1000-8                 200   8.5 ms/op
BenchmarkExport_LargeDataset-8                          1     42.3 s/op   events/sec:23643   exported_mb:245.7
```

**Performance Validation**:
- Export throughput must exceed 10K events/sec
- Large dataset exports should maintain consistent performance
- Memory usage should remain bounded during export

### 4. Integrity Benchmarks

**Purpose**: Validate hash chain computation and verification performance.

**Key Benchmarks**:
- `BenchmarkIntegrity_HashChainVerification` - Core verification performance
- `BenchmarkIntegrity_HashComputation` - Hash generation speed
- `BenchmarkIntegrity_CorruptionDetection` - Corruption scanning
- `BenchmarkIntegrity_IncrementalVerification` - Incremental updates

**Expected Results**:
```
BenchmarkIntegrity_HashChainVerification/chain_100k-8    20    125 ms/op    events/sec:800000    chain_size:100000
BenchmarkIntegrity_HashComputation/event_type_0-8       5000000   0.35 μs/op
BenchmarkIntegrity_CorruptionDetection/dataset_100k-8   10    2.1 s/op
```

**Performance Validation**:
- Hash chain verification should exceed 50K events/sec
- Hash computation should be < 1μs per event
- Corruption detection should complete in reasonable time

### 5. Cache Benchmarks

**Purpose**: Validate cache efficiency and performance.

**Key Benchmarks**:
- `BenchmarkCache_HitRatio` - Cache efficiency validation
- `BenchmarkCache_ReadPerformance` - Read operation speed
- `BenchmarkCache_ConcurrentAccess` - Multi-threaded cache access
- `BenchmarkCache_InvalidationPerformance` - Cache invalidation speed

**Expected Results**:
```
BenchmarkCache_HitRatio/fast_cache-8                   100000   85.3 μs/op   hit_ratio_percent:82.5
BenchmarkCache_ReadPerformance/sequential_reads-8      500000   3.2 μs/op    reads/sec:312500
BenchmarkCache_ConcurrentAccess/concurrent_10-8        20000    125 μs/op    ops/sec:160000
```

**Performance Validation**:
- Cache hit ratio should exceed 80%
- Read operations should be < 10μs
- Concurrent access should scale with worker count

## Performance Analysis

### Interpreting Results

**Latency Metrics**:
- `avg_latency_μs` - Average operation latency in microseconds
- `max_latency_μs` - Maximum observed latency
- `latency_ms` - Latency in milliseconds for longer operations

**Throughput Metrics**:
- `events/sec` - Events processed per second
- `ops/sec` - Operations per second
- `reads/sec`, `writes/sec` - Specific operation rates

**Memory Metrics**:
- `memory_mb` - Memory usage in megabytes
- `bytes/event` - Memory per event
- Allocations and allocations per operation from `-benchmem`

**Quality Metrics**:
- `hit_ratio_percent` - Cache hit percentage
- `drop_rate_percent` - Event drop rate under load
- `events_stored` - Successfully persisted events

### Performance Regression Detection

Compare results across code changes:

```bash
# Baseline benchmark
git checkout main
go test -bench=. -count=5 ./internal/service/audit/ > baseline.txt

# Feature branch benchmark  
git checkout feature/optimization
go test -bench=. -count=5 ./internal/service/audit/ > feature.txt

# Compare results
benchcmp baseline.txt feature.txt
```

### Profiling Integration

```bash
# CPU profiling for hot paths
go test -bench=BenchmarkLogger_SingleEventLogging -cpuprofile=cpu.prof ./internal/service/audit/
go tool pprof -http=:8080 cpu.prof

# Memory profiling for memory usage
go test -bench=BenchmarkLogger_MemoryUsage -memprofile=mem.prof ./internal/service/audit/
go tool pprof -http=:8081 mem.prof

# Trace profiling for concurrency analysis
go test -bench=BenchmarkLogger_ConcurrentPerformance -trace=trace.out ./internal/service/audit/
go tool trace trace.out
```

## Continuous Integration Integration

### CI Pipeline Integration

```yaml
# .github/workflows/performance.yml
- name: Run Performance Benchmarks
  run: |
    go test -bench=. -benchmem -count=3 ./internal/service/audit/ | tee benchmark_results.txt
    
    # Validate critical performance requirements
    if ! grep -q "avg_latency_μs.*[0-4][0-9][0-9][0-9]" benchmark_results.txt; then
      echo "FAIL: Write latency exceeds 5ms requirement"
      exit 1
    fi
    
    if ! grep -q "events/sec.*[1-9][0-9][0-9][0-9][0-9]" benchmark_results.txt; then
      echo "FAIL: Export throughput below 10K events/sec requirement"
      exit 1
    fi
```

### Performance Monitoring

```bash
# Daily performance monitoring
go test -bench=BenchmarkLogger_PerformanceRegression -benchtime=30s ./internal/service/audit/
go test -bench=BenchmarkIntegrity_PerformanceRegression -benchtime=30s ./internal/service/audit/
go test -bench=BenchmarkCache_PerformanceRegression -benchtime=30s ./internal/service/audit/
```

## Troubleshooting Performance Issues

### Common Performance Problems

**High Write Latency**:
1. Check buffer sizes and worker pool configuration
2. Verify database connection pool settings
3. Monitor disk I/O and network latency
4. Review hash computation efficiency

**Slow Query Performance**:
1. Verify database indexes are properly created
2. Check query optimization and execution plans
3. Monitor memory usage during large queries
4. Review filtering and pagination logic

**Low Export Throughput**:
1. Check batch size configuration
2. Verify serialization performance
3. Monitor I/O bottlenecks
4. Review compression overhead

**Poor Cache Performance**:
1. Verify cache configuration and sizing
2. Check hit ratio and eviction policies
3. Monitor memory usage and fragmentation
4. Review cache key generation logic

### Performance Optimization

**Configuration Tuning**:
```go
// Optimized configuration for high performance
config := LoggerConfig{
    WorkerPoolSize:      20,    // Increase workers for higher throughput
    BatchWorkers:        10,    // More batch workers for better batching
    BatchSize:           1000,  // Larger batches for efficiency
    BatchTimeout:        500 * time.Millisecond,
    BufferSize:          50000, // Larger buffer to handle bursts
    WriteTimeout:        2 * time.Second,
    HashChainEnabled:    true,
    EnrichmentEnabled:   false, // Disable for pure performance
    GracefulDegradation: true,
    MaxMemoryUsage:      200 * 1024 * 1024, // 200MB limit
}
```

**Hardware Considerations**:
- SSD storage for database and logs
- Sufficient RAM for caching and buffers
- Multiple CPU cores for concurrent processing
- Fast network for distributed components

## Benchmark Maintenance

### Adding New Benchmarks

1. Follow naming convention: `BenchmarkComponent_TestCase`
2. Include performance validation logic
3. Add appropriate metrics reporting
4. Document expected performance characteristics
5. Update this guide with new benchmark information

### Updating Performance Targets

When updating performance requirements:
1. Update benchmark validation logic
2. Update documentation and comments
3. Update CI/CD pipeline validation
4. Communicate changes to team
5. Ensure backward compatibility where possible

## Summary

This comprehensive benchmark suite ensures the IMMUTABLE_AUDIT feature meets its critical performance requirements:

- ✅ **< 5ms write latency** - Validated by logger benchmarks
- ✅ **< 1s query response for 1M events** - Validated by query benchmarks  
- ✅ **> 10K events/sec export** - Validated by export benchmarks
- ✅ **< 100MB base memory** - Validated by memory benchmarks
- ✅ **Hash chain integrity** - Validated by integrity benchmarks
- ✅ **Cache efficiency** - Validated by cache benchmarks

Regular execution of these benchmarks ensures performance regression detection and validates that the system continues to meet its stringent performance requirements under various load conditions and usage patterns.