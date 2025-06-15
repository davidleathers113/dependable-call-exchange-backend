#!/bin/bash

# IMMUTABLE_AUDIT Performance Benchmark Runner
# Validates < 5ms write latency, < 1s query response, > 10K events/sec export, < 100MB memory

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
AUDIT_DIR="$PROJECT_ROOT/internal/service/audit"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Performance thresholds
MAX_WRITE_LATENCY_MS=5
MAX_QUERY_LATENCY_MS=1000
MIN_EXPORT_THROUGHPUT=10000
MAX_MEMORY_MB=100

echo -e "${BLUE}=== IMMUTABLE_AUDIT Performance Benchmark Suite ===${NC}"
echo "Validating performance requirements:"
echo "  - Write latency: < ${MAX_WRITE_LATENCY_MS}ms"
echo "  - Query response (1M events): < ${MAX_QUERY_LATENCY_MS}ms" 
echo "  - Export throughput: > ${MIN_EXPORT_THROUGHPUT} events/sec"
echo "  - Base memory: < ${MAX_MEMORY_MB}MB"
echo ""

cd "$PROJECT_ROOT"

# Create results directory
RESULTS_DIR="$PROJECT_ROOT/benchmark_results"
mkdir -p "$RESULTS_DIR"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
RESULTS_FILE="$RESULTS_DIR/audit_benchmark_${TIMESTAMP}.txt"

echo -e "${BLUE}Running benchmarks...${NC}"

# Function to run benchmark and capture results
run_benchmark() {
    local bench_name="$1"
    local bench_pattern="$2"
    local description="$3"
    
    echo -e "${YELLOW}Running $description...${NC}"
    
    # Run benchmark and capture output
    if go test -bench="$bench_pattern" -benchtime=10s -benchmem "$AUDIT_DIR" >> "$RESULTS_FILE" 2>&1; then
        echo -e "${GREEN}✓ $description completed${NC}"
        return 0
    else
        echo -e "${RED}✗ $description failed${NC}"
        return 1
    fi
}

# Function to validate performance requirements
validate_performance() {
    echo -e "${BLUE}Validating performance requirements...${NC}"
    
    local validation_failed=0
    
    # Check write latency requirement (< 5ms)
    echo -n "Checking write latency requirement (< ${MAX_WRITE_LATENCY_MS}ms)... "
    if grep -q "BenchmarkLogger_SingleEventLogging" "$RESULTS_FILE"; then
        # Extract average latency from benchmark results
        local avg_latency=$(grep -A 5 "BenchmarkLogger_SingleEventLogging" "$RESULTS_FILE" | grep -o "avg_latency_μs:[0-9]*" | cut -d: -f2 | head -1)
        if [ -n "$avg_latency" ] && [ "$avg_latency" -lt $((MAX_WRITE_LATENCY_MS * 1000)) ]; then
            echo -e "${GREEN}PASS${NC} (${avg_latency}μs)"
        else
            echo -e "${RED}FAIL${NC} (${avg_latency}μs > ${MAX_WRITE_LATENCY_MS}ms)"
            validation_failed=1
        fi
    else
        echo -e "${YELLOW}SKIP${NC} (benchmark not run)"
    fi
    
    # Check query latency requirement (< 1s for 1M events)
    echo -n "Checking query latency requirement (< ${MAX_QUERY_LATENCY_MS}ms for 1M events)... "
    if grep -q "BenchmarkQuery_1MillionEvents.*dataset_1000k" "$RESULTS_FILE"; then
        local query_latency=$(grep -A 5 "BenchmarkQuery_1MillionEvents.*dataset_1000k" "$RESULTS_FILE" | grep -o "avg_latency_ms:[0-9]*" | cut -d: -f2 | head -1)
        if [ -n "$query_latency" ] && [ "$query_latency" -lt "$MAX_QUERY_LATENCY_MS" ]; then
            echo -e "${GREEN}PASS${NC} (${query_latency}ms)"
        else
            echo -e "${RED}FAIL${NC} (${query_latency}ms > ${MAX_QUERY_LATENCY_MS}ms)"
            validation_failed=1
        fi
    else
        echo -e "${YELLOW}SKIP${NC} (benchmark not run)"
    fi
    
    # Check export throughput requirement (> 10K events/sec)
    echo -n "Checking export throughput requirement (> ${MIN_EXPORT_THROUGHPUT} events/sec)... "
    if grep -q "BenchmarkExport_Throughput" "$RESULTS_FILE"; then
        local throughput=$(grep -A 5 "BenchmarkExport_Throughput" "$RESULTS_FILE" | grep -o "events/sec:[0-9]*" | cut -d: -f2 | head -1)
        if [ -n "$throughput" ] && [ "$throughput" -gt "$MIN_EXPORT_THROUGHPUT" ]; then
            echo -e "${GREEN}PASS${NC} (${throughput} events/sec)"
        else
            echo -e "${RED}FAIL${NC} (${throughput} events/sec < ${MIN_EXPORT_THROUGHPUT})"
            validation_failed=1
        fi
    else
        echo -e "${YELLOW}SKIP${NC} (benchmark not run)"
    fi
    
    # Check memory usage requirement (< 100MB)
    echo -n "Checking memory usage requirement (< ${MAX_MEMORY_MB}MB)... "
    if grep -q "BenchmarkLogger_MemoryUsage" "$RESULTS_FILE"; then
        local memory_mb=$(grep -A 5 "BenchmarkLogger_MemoryUsage" "$RESULTS_FILE" | grep -o "memory_mb:[0-9]*" | cut -d: -f2 | head -1)
        if [ -n "$memory_mb" ] && [ "$memory_mb" -lt "$MAX_MEMORY_MB" ]; then
            echo -e "${GREEN}PASS${NC} (${memory_mb}MB)"
        else
            echo -e "${RED}FAIL${NC} (${memory_mb}MB > ${MAX_MEMORY_MB}MB)"
            validation_failed=1
        fi
    else
        echo -e "${YELLOW}SKIP${NC} (benchmark not run)"
    fi
    
    return $validation_failed
}

# Start benchmark execution
echo "=== AUDIT PERFORMANCE BENCHMARK RESULTS ===" > "$RESULTS_FILE"
echo "Timestamp: $(date)" >> "$RESULTS_FILE"
echo "Git Commit: $(git rev-parse HEAD 2>/dev/null || echo 'unknown')" >> "$RESULTS_FILE"
echo "Go Version: $(go version)" >> "$RESULTS_FILE"
echo "" >> "$RESULTS_FILE"

benchmark_failed=0

# Run logger benchmarks (critical for < 5ms write latency)
run_benchmark "logger" "BenchmarkLogger_SingleEventLogging" "Logger single event logging" || benchmark_failed=1
run_benchmark "logger_memory" "BenchmarkLogger_MemoryUsage" "Logger memory usage" || benchmark_failed=1
run_benchmark "logger_batch" "BenchmarkLogger_BatchProcessing" "Logger batch processing" || benchmark_failed=1
run_benchmark "logger_concurrent" "BenchmarkLogger_ConcurrentPerformance" "Logger concurrent performance" || benchmark_failed=1

# Run query benchmarks (critical for < 1s query response)
run_benchmark "query_1m" "BenchmarkQuery_1MillionEvents" "Query 1M events performance" || benchmark_failed=1
run_benchmark "query_range" "BenchmarkQuery_SequenceRange" "Query sequence range performance" || benchmark_failed=1
run_benchmark "query_count" "BenchmarkQuery_Count" "Query count performance" || benchmark_failed=1

# Run export benchmarks (critical for > 10K events/sec)
run_benchmark "export_throughput" "BenchmarkExport_Throughput" "Export throughput performance" || benchmark_failed=1
run_benchmark "export_large" "BenchmarkExport_LargeDataset" "Export large dataset performance" || benchmark_failed=1
run_benchmark "export_concurrent" "BenchmarkExport_ConcurrentExports" "Export concurrent performance" || benchmark_failed=1

# Run integrity benchmarks
run_benchmark "integrity_chain" "BenchmarkIntegrity_HashChainVerification" "Integrity hash chain verification" || benchmark_failed=1
run_benchmark "integrity_compute" "BenchmarkIntegrity_HashComputation" "Integrity hash computation" || benchmark_failed=1
run_benchmark "integrity_corruption" "BenchmarkIntegrity_CorruptionDetection" "Integrity corruption detection" || benchmark_failed=1

# Run cache benchmarks
run_benchmark "cache_hit" "BenchmarkCache_HitRatio" "Cache hit ratio performance" || benchmark_failed=1
run_benchmark "cache_read" "BenchmarkCache_ReadPerformance" "Cache read performance" || benchmark_failed=1
run_benchmark "cache_concurrent" "BenchmarkCache_ConcurrentAccess" "Cache concurrent access" || benchmark_failed=1

echo ""
echo -e "${BLUE}Benchmark execution completed.${NC}"
echo "Results saved to: $RESULTS_FILE"
echo ""

# Validate performance requirements
validate_performance
validation_result=$?

# Generate summary
echo ""
echo -e "${BLUE}=== BENCHMARK SUMMARY ===${NC}"
if [ $benchmark_failed -eq 0 ]; then
    echo -e "${GREEN}✓ All benchmarks executed successfully${NC}"
else
    echo -e "${RED}✗ Some benchmarks failed to execute${NC}"
fi

if [ $validation_result -eq 0 ]; then
    echo -e "${GREEN}✓ All performance requirements PASSED${NC}"
else
    echo -e "${RED}✗ Some performance requirements FAILED${NC}"
fi

echo ""
echo "Performance Requirements Summary:"
echo "  - Write latency < 5ms: $(grep -q "write.*latency.*PASS" <<< "$(cat "$RESULTS_FILE")" && echo "✓ PASS" || echo "✗ FAIL/SKIP")"
echo "  - Query 1M events < 1s: $(grep -q "query.*latency.*PASS" <<< "$(cat "$RESULTS_FILE")" && echo "✓ PASS" || echo "✗ FAIL/SKIP")"
echo "  - Export > 10K events/sec: $(grep -q "export.*throughput.*PASS" <<< "$(cat "$RESULTS_FILE")" && echo "✓ PASS" || echo "✗ FAIL/SKIP")"
echo "  - Memory < 100MB: $(grep -q "memory.*PASS" <<< "$(cat "$RESULTS_FILE")" && echo "✓ PASS" || echo "✗ FAIL/SKIP")"

echo ""
echo "View detailed results:"
echo "  cat $RESULTS_FILE"
echo ""
echo "Generate performance report:"
echo "  $SCRIPT_DIR/generate-performance-report.sh $RESULTS_FILE"

# Exit with error if any validation failed
if [ $benchmark_failed -ne 0 ] || [ $validation_result -ne 0 ]; then
    exit 1
fi

echo -e "${GREEN}IMMUTABLE_AUDIT performance validation completed successfully!${NC}"