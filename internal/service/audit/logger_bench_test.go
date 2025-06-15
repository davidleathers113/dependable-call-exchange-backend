package audit

import (
	"context"
	"crypto/sha256"
	"fmt"
	"math/rand"
	"net/http"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/audit"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
)

// Performance benchmarks for audit logging with < 5ms latency validation
// Target: Write latency < 5ms p99, Query < 1s for 1M events, Export > 10K events/sec

// Mock implementations for benchmarking
type mockAuditRepository struct {
	delay       time.Duration
	events      []*audit.Event
	batchEvents [][]*audit.Event
	mu          sync.RWMutex
}

func (m *mockAuditRepository) Store(ctx context.Context, event *audit.Event) error {
	if m.delay > 0 {
		time.Sleep(m.delay)
	}
	m.mu.Lock()
	m.events = append(m.events, event)
	m.mu.Unlock()
	return nil
}

func (m *mockAuditRepository) StoreBatch(ctx context.Context, events []*audit.Event) error {
	if m.delay > 0 {
		time.Sleep(m.delay)
	}
	m.mu.Lock()
	m.batchEvents = append(m.batchEvents, events)
	for _, event := range events {
		m.events = append(m.events, event)
	}
	m.mu.Unlock()
	return nil
}

func (m *mockAuditRepository) GetLatestSequenceNumber(ctx context.Context) (values.SequenceNumber, error) {
	return values.NewSequenceNumber(int64(len(m.events)))
}

func (m *mockAuditRepository) EventCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.events)
}

type mockAuditCache struct {
	lastHash     string
	lastSequence int64
	events       map[string]*audit.Event
	mu           sync.RWMutex
}

func (m *mockAuditCache) SetLatestHash(ctx context.Context, hash string, sequenceNum int64) error {
	m.mu.Lock()
	m.lastHash = hash
	m.lastSequence = sequenceNum
	m.mu.Unlock()
	return nil
}

func (m *mockAuditCache) GetLatestHash(ctx context.Context) (string, int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastHash, m.lastSequence, nil
}

func (m *mockAuditCache) SetEvents(ctx context.Context, events []*audit.Event) error {
	m.mu.Lock()
	if m.events == nil {
		m.events = make(map[string]*audit.Event)
	}
	for _, event := range events {
		m.events[event.ID.String()] = event
	}
	m.mu.Unlock()
	return nil
}

type mockAuditPublisher struct{}

func (m *mockAuditPublisher) Publish(ctx context.Context, event *audit.Event) error {
	return nil
}

type mockDomainService struct{}

func (m *mockDomainService) ValidateEvent(event *audit.Event) error {
	if event.ID == uuid.Nil {
		return fmt.Errorf("event ID is required")
	}
	if event.ActorID == "" {
		return fmt.Errorf("actor ID is required")
	}
	return nil
}

func (m *mockDomainService) ComputeHash(event *audit.Event, previousHash string) (string, error) {
	h := sha256.New()
	h.Write([]byte(fmt.Sprintf("%s:%s:%s:%s", event.ID, event.ActorID, event.Action, previousHash)))
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

type mockEventEnricher struct{}

func (m *mockEventEnricher) Enrich(ctx context.Context, event *audit.Event, request *http.Request) error {
	event.ActorIP = "192.168.1.1"
	event.ActorAgent = "BenchmarkAgent/1.0"
	return nil
}

// Benchmark helpers
func setupLogger(b *testing.B, config LoggerConfig) (*Logger, *mockAuditRepository) {
	logger := zap.NewNop()
	repository := &mockAuditRepository{}
	cache := &mockAuditCache{}
	publisher := &mockAuditPublisher{}
	domainService := &mockDomainService{}
	enricher := &mockEventEnricher{}

	// Generate test hash key
	config.HashSecretKey = make([]byte, 32)
	for i := range config.HashSecretKey {
		config.HashSecretKey[i] = byte(i)
	}

	auditLogger, err := NewLogger(
		context.Background(),
		config,
		logger,
		repository,
		cache,
		publisher,
		domainService,
		enricher,
	)
	if err != nil {
		b.Fatalf("Failed to create logger: %v", err)
	}

	b.Cleanup(func() {
		auditLogger.Close()
	})

	return auditLogger, repository
}

func generateRandomEvent(actorID string) (audit.EventType, string, string, string, map[string]interface{}) {
	eventTypes := []audit.EventType{
		audit.EventTypeUserLogin,
		audit.EventTypeDataAccess,
		audit.EventTypeSystemActivity,
		audit.EventTypeComplianceViolation,
	}

	actions := []string{
		"login",
		"logout",
		"create_call",
		"update_bid",
		"delete_account",
		"view_data",
	}

	results := []string{"success", "failure", "pending", "error"}

	eventType := eventTypes[rand.Intn(len(eventTypes))]
	action := actions[rand.Intn(len(actions))]
	result := results[rand.Intn(len(results))]
	targetID := uuid.New().String()

	metadata := map[string]interface{}{
		"source":    "benchmark",
		"timestamp": time.Now().Unix(),
		"random":    rand.Int63(),
	}

	return eventType, actorID, targetID, action, result, metadata
}

// Single event logging benchmark - validates < 5ms write latency
func BenchmarkLogger_SingleEventLogging(b *testing.B) {
	tests := []struct {
		name   string
		config LoggerConfig
	}{
		{
			name:   "default_config",
			config: DefaultLoggerConfig(),
		},
		{
			name: "optimized_config",
			config: LoggerConfig{
				WorkerPoolSize:      20,
				BatchWorkers:        10,
				BatchSize:           50,
				BatchTimeout:        500 * time.Millisecond,
				BufferSize:          20000,
				WriteTimeout:        2 * time.Second,
				HashChainEnabled:    true,
				FailureThreshold:    10,
				CircuitTimeout:      10 * time.Second,
				EnrichmentEnabled:   false, // Disable for pure performance
				IPGeoEnabled:        false,
				UserAgentParsing:    false,
				GracefulDegradation: true,
				MaxMemoryUsage:      200 * 1024 * 1024, // 200MB
				DropPolicy:          "oldest",
			},
		},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			logger, repo := setupLogger(b, tt.config)

			// Pre-generate test data
			events := make([]struct {
				eventType audit.EventType
				actorID   string
				targetID  string
				action    string
				result    string
				metadata  map[string]interface{}
			}, b.N)

			for i := 0; i < b.N; i++ {
				events[i].eventType, events[i].actorID, events[i].targetID,
					events[i].action, events[i].result, events[i].metadata = generateRandomEvent(fmt.Sprintf("actor_%d", i%1000))
			}

			ctx := context.Background()
			var maxLatency time.Duration
			var totalLatency time.Duration

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				start := time.Now()

				err := logger.LogEvent(
					ctx,
					events[i].eventType,
					events[i].actorID,
					events[i].targetID,
					events[i].action,
					events[i].result,
					events[i].metadata,
				)

				latency := time.Since(start)
				totalLatency += latency

				if latency > maxLatency {
					maxLatency = latency
				}

				if err != nil {
					b.Fatalf("Failed to log event: %v", err)
				}

				// Validate < 5ms latency requirement
				if latency > 5*time.Millisecond {
					b.Logf("WARNING: Event %d exceeded 5ms latency: %v", i, latency)
				}
			}

			b.StopTimer()

			// Wait for async processing
			logger.FlushEvents(ctx)
			time.Sleep(100 * time.Millisecond)

			avgLatency := totalLatency / time.Duration(b.N)

			b.ReportMetric(float64(avgLatency.Microseconds()), "avg_latency_μs")
			b.ReportMetric(float64(maxLatency.Microseconds()), "max_latency_μs")
			b.ReportMetric(float64(repo.EventCount()), "events_stored")

			// Performance validation
			if avgLatency > 5*time.Millisecond {
				b.Errorf("Average latency %v exceeds 5ms target", avgLatency)
			}

			if maxLatency > 10*time.Millisecond {
				b.Errorf("Max latency %v exceeds 10ms acceptable limit", maxLatency)
			}
		})
	}
}

// Batch processing benchmark - validates throughput targets
func BenchmarkLogger_BatchProcessing(b *testing.B) {
	batchSizes := []int{10, 50, 100, 500, 1000}

	for _, batchSize := range batchSizes {
		b.Run(fmt.Sprintf("batch_size_%d", batchSize), func(b *testing.B) {
			config := DefaultLoggerConfig()
			config.BatchSize = batchSize
			config.BatchWorkers = 10
			config.WorkerPoolSize = 20
			config.BufferSize = batchSize * 100

			logger, repo := setupLogger(b, config)
			ctx := context.Background()

			// Generate batch events
			totalEvents := b.N * batchSize
			events := make([][]interface{}, b.N)

			for i := 0; i < b.N; i++ {
				events[i] = make([]interface{}, 6)
				events[i][0], events[i][1], events[i][2], events[i][3], events[i][4], events[i][5] = generateRandomEvent(fmt.Sprintf("batch_actor_%d", i))
			}

			start := time.Now()
			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				for j := 0; j < batchSize; j++ {
					err := logger.LogEvent(
						ctx,
						events[i][0].(audit.EventType),
						events[i][1].(string),
						events[i][2].(string),
						events[i][3].(string),
						events[i][4].(string),
						events[i][5].(map[string]interface{}),
					)
					if err != nil {
						b.Fatalf("Failed to log event: %v", err)
					}
				}
			}

			b.StopTimer()

			// Wait for processing
			logger.FlushEvents(ctx)
			time.Sleep(200 * time.Millisecond)

			processingTime := time.Since(start)
			throughput := float64(totalEvents) / processingTime.Seconds()

			b.ReportMetric(throughput, "events/sec")
			b.ReportMetric(float64(repo.EventCount()), "events_stored")
			b.ReportMetric(float64(processingTime.Milliseconds()), "total_time_ms")

			// Target: > 10K events/sec
			if throughput < 10000 {
				b.Logf("WARNING: Throughput %v events/sec below 10K target", throughput)
			}
		})
	}
}

// Hash chain computation benchmark
func BenchmarkLogger_HashChainComputation(b *testing.B) {
	tests := []struct {
		name         string
		chainEnabled bool
		batchSize    int
	}{
		{"no_hash_chain", false, 100},
		{"hash_chain_enabled", true, 100},
		{"hash_chain_large_batch", true, 1000},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			config := DefaultLoggerConfig()
			config.HashChainEnabled = tt.chainEnabled
			config.BatchSize = tt.batchSize

			logger, repo := setupLogger(b, config)
			ctx := context.Background()

			// Pre-generate events
			events := make([][]interface{}, b.N)
			for i := 0; i < b.N; i++ {
				events[i] = make([]interface{}, 6)
				events[i][0], events[i][1], events[i][2], events[i][3], events[i][4], events[i][5] = generateRandomEvent(fmt.Sprintf("hash_actor_%d", i))
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				err := logger.LogEvent(
					ctx,
					events[i][0].(audit.EventType),
					events[i][1].(string),
					events[i][2].(string),
					events[i][3].(string),
					events[i][4].(string),
					events[i][5].(map[string]interface{}),
				)
				if err != nil {
					b.Fatalf("Failed to log event: %v", err)
				}
			}

			b.StopTimer()

			logger.FlushEvents(ctx)
			time.Sleep(100 * time.Millisecond)

			b.ReportMetric(float64(repo.EventCount()), "events_stored")
		})
	}
}

// Memory usage benchmark - validates < 100MB base requirement
func BenchmarkLogger_MemoryUsage(b *testing.B) {
	config := DefaultLoggerConfig()
	config.BufferSize = 50000
	config.BatchSize = 1000
	config.MaxMemoryUsage = 100 * 1024 * 1024 // 100MB

	logger, repo := setupLogger(b, config)
	ctx := context.Background()

	// Measure initial memory
	runtime.GC()
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)

	b.ResetTimer()

	// Generate many events to test memory usage
	for i := 0; i < b.N; i++ {
		eventType, actorID, targetID, action, result, metadata := generateRandomEvent(fmt.Sprintf("memory_actor_%d", i))

		err := logger.LogEvent(ctx, eventType, actorID, targetID, action, result, metadata)
		if err != nil {
			b.Fatalf("Failed to log event: %v", err)
		}

		// Check memory usage periodically
		if i%1000 == 0 {
			runtime.GC()
			var m2 runtime.MemStats
			runtime.ReadMemStats(&m2)

			memoryUsed := m2.Alloc - m1.Alloc

			// Validate < 100MB base memory requirement
			if memoryUsed > 100*1024*1024 {
				b.Logf("WARNING: Memory usage %d bytes exceeds 100MB target", memoryUsed)
			}
		}
	}

	b.StopTimer()

	logger.FlushEvents(ctx)

	// Final memory measurement
	runtime.GC()
	var m3 runtime.MemStats
	runtime.ReadMemStats(&m3)

	totalMemoryUsed := m3.Alloc - m1.Alloc

	b.ReportMetric(float64(totalMemoryUsed), "memory_bytes")
	b.ReportMetric(float64(totalMemoryUsed)/(1024*1024), "memory_mb")
	b.ReportMetric(float64(repo.EventCount()), "events_stored")

	// Validate memory target
	if totalMemoryUsed > 100*1024*1024 {
		b.Errorf("Memory usage %d bytes exceeds 100MB target", totalMemoryUsed)
	}
}

// Concurrent performance testing - validates performance under load
func BenchmarkLogger_ConcurrentPerformance(b *testing.B) {
	workerCounts := []int{1, 5, 10, 20, 50}

	for _, workers := range workerCounts {
		b.Run(fmt.Sprintf("workers_%d", workers), func(b *testing.B) {
			config := DefaultLoggerConfig()
			config.WorkerPoolSize = workers
			config.BatchWorkers = workers / 2
			if config.BatchWorkers < 1 {
				config.BatchWorkers = 1
			}
			config.BufferSize = workers * 1000

			logger, repo := setupLogger(b, config)
			ctx := context.Background()

			var wg sync.WaitGroup
			eventsPerWorker := b.N / workers
			if eventsPerWorker < 1 {
				eventsPerWorker = 1
			}

			start := time.Now()
			b.ResetTimer()
			b.ReportAllocs()

			for w := 0; w < workers; w++ {
				wg.Add(1)
				go func(workerID int) {
					defer wg.Done()

					for i := 0; i < eventsPerWorker; i++ {
						eventType, actorID, targetID, action, result, metadata := generateRandomEvent(fmt.Sprintf("concurrent_actor_%d_%d", workerID, i))

						err := logger.LogEvent(ctx, eventType, actorID, targetID, action, result, metadata)
						if err != nil {
							b.Logf("Worker %d failed to log event: %v", workerID, err)
							return
						}
					}
				}(w)
			}

			wg.Wait()
			b.StopTimer()

			logger.FlushEvents(ctx)
			time.Sleep(200 * time.Millisecond)

			processingTime := time.Since(start)
			totalEvents := workers * eventsPerWorker
			throughput := float64(totalEvents) / processingTime.Seconds()

			b.ReportMetric(throughput, "events/sec")
			b.ReportMetric(float64(repo.EventCount()), "events_stored")
			b.ReportMetric(float64(workers), "concurrent_workers")

			// Performance validation for concurrent load
			expectedMinThroughput := float64(workers) * 1000 // 1K events/sec per worker minimum
			if throughput < expectedMinThroughput {
				b.Logf("WARNING: Throughput %v events/sec below expected %v for %d workers",
					throughput, expectedMinThroughput, workers)
			}
		})
	}
}

// Circuit breaker performance impact benchmark
func BenchmarkLogger_CircuitBreakerPerformance(b *testing.B) {
	tests := []struct {
		name      string
		withDelay bool
		delay     time.Duration
	}{
		{"normal_operation", false, 0},
		{"with_1ms_delay", true, time.Millisecond},
		{"with_5ms_delay", true, 5 * time.Millisecond},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			config := DefaultLoggerConfig()
			config.FailureThreshold = 5
			config.CircuitTimeout = 1 * time.Second

			logger, repo := setupLogger(b, config)

			// Set delay on mock repository if needed
			if mockRepo, ok := logger.repository.(*mockAuditRepository); ok {
				mockRepo.delay = tt.delay
			}

			ctx := context.Background()

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				eventType, actorID, targetID, action, result, metadata := generateRandomEvent(fmt.Sprintf("circuit_actor_%d", i))

				err := logger.LogEvent(ctx, eventType, actorID, targetID, action, result, metadata)
				if err != nil {
					b.Fatalf("Failed to log event: %v", err)
				}
			}

			b.StopTimer()

			logger.FlushEvents(ctx)
			time.Sleep(100 * time.Millisecond)

			b.ReportMetric(float64(repo.EventCount()), "events_stored")
		})
	}
}

// Graceful degradation benchmark - tests behavior under stress
func BenchmarkLogger_GracefulDegradation(b *testing.B) {
	config := DefaultLoggerConfig()
	config.BufferSize = 100          // Small buffer to trigger degradation
	config.GracefulDegradation = true
	config.DropPolicy = "oldest"

	logger, repo := setupLogger(b, config)
	ctx := context.Background()

	// Flood the logger to trigger graceful degradation
	totalSent := 0
	start := time.Now()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		eventType, actorID, targetID, action, result, metadata := generateRandomEvent(fmt.Sprintf("degradation_actor_%d", i))

		err := logger.LogEvent(ctx, eventType, actorID, targetID, action, result, metadata)
		// In graceful degradation mode, we don't fail on buffer full
		if err != nil {
			b.Logf("Unexpected error during graceful degradation: %v", err)
		}
		totalSent++

		// Small delay to prevent busy loop
		if i%100 == 0 {
			time.Sleep(time.Microsecond)
		}
	}

	b.StopTimer()

	logger.FlushEvents(ctx)
	time.Sleep(200 * time.Millisecond)

	processingTime := time.Since(start)
	stats := logger.GetStats()

	b.ReportMetric(float64(stats.TotalEvents), "total_events")
	b.ReportMetric(float64(stats.DroppedEvents), "dropped_events")
	b.ReportMetric(float64(repo.EventCount()), "events_stored")
	b.ReportMetric(float64(totalSent), "events_sent")
	b.ReportMetric(float64(processingTime.Milliseconds()), "processing_time_ms")

	// Calculate drop rate
	dropRate := float64(stats.DroppedEvents) / float64(totalSent) * 100
	b.ReportMetric(dropRate, "drop_rate_percent")

	b.Logf("Graceful degradation: %d sent, %d processed, %d dropped (%.2f%% drop rate)",
		totalSent, repo.EventCount(), stats.DroppedEvents, dropRate)
}

// Performance regression test - ensures consistent performance
func BenchmarkLogger_PerformanceRegression(b *testing.B) {
	// This benchmark serves as a regression test for performance
	// It should maintain consistent results across code changes

	config := DefaultLoggerConfig()
	logger, repo := setupLogger(b, config)
	ctx := context.Background()

	// Standard test conditions
	eventType := audit.EventTypeUserLogin
	actorID := "performance_test_actor"
	targetID := "performance_test_target"
	action := "performance_test"
	result := "success"
	metadata := map[string]interface{}{
		"benchmark": "regression",
		"timestamp": time.Now().Unix(),
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		err := logger.LogEvent(ctx, eventType, actorID, targetID, action, result, metadata)
		if err != nil {
			b.Fatalf("Failed to log event: %v", err)
		}
	}

	b.StopTimer()

	logger.FlushEvents(ctx)
	time.Sleep(100 * time.Millisecond)

	b.ReportMetric(float64(repo.EventCount()), "events_stored")

	// Performance baselines (adjust based on hardware)
	opsPerSec := float64(b.N) / b.Elapsed().Seconds()
	b.ReportMetric(opsPerSec, "ops/sec")

	// Minimum acceptable performance (adjust as needed)
	minOpsPerSec := 10000.0
	if opsPerSec < minOpsPerSec {
		b.Errorf("Performance regression: %.2f ops/sec below minimum %.2f ops/sec",
			opsPerSec, minOpsPerSec)
	}
}