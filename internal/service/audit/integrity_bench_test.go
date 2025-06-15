package audit

import (
	"context"
	"crypto/sha256"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/audit"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
)

// Integrity verification benchmarks - validates hash chain and verification performance

// Mock integrity repository for benchmarking
type mockIntegrityRepository struct {
	events       []*audit.Event
	verifyDelay  time.Duration
	computeDelay time.Duration
}

func (m *mockIntegrityRepository) GetEventsBySequenceRange(ctx context.Context, start, end values.SequenceNumber) ([]*audit.Event, error) {
	if m.verifyDelay > 0 {
		time.Sleep(m.verifyDelay)
	}

	var result []*audit.Event
	for _, event := range m.events {
		if event.SequenceNum >= start.Value() && event.SequenceNum <= end.Value() {
			result = append(result, event)
		}
	}
	return result, nil
}

func (m *mockIntegrityRepository) GetEventCount(ctx context.Context) (int64, error) {
	return int64(len(m.events)), nil
}

// Mock integrity service for benchmarking
type mockIntegrityService struct {
	repository   *mockIntegrityRepository
	hashDelay    time.Duration
	verifyDelay  time.Duration
	secretKey    []byte
}

func (m *mockIntegrityService) VerifyHashChain(ctx context.Context, start, end values.SequenceNumber) (*audit.HashChainVerificationResult, error) {
	events, err := m.repository.GetEventsBySequenceRange(ctx, start, end)
	if err != nil {
		return nil, err
	}

	result := &audit.HashChainVerificationResult{
		StartSequence:   start,
		EndSequence:     end,
		TotalEvents:     int64(len(events)),
		VerifiedEvents:  0,
		CorruptedEvents: 0,
		StartTime:       time.Now(),
	}

	var previousHash string
	for i, event := range events {
		// Simulate hash computation delay
		if m.hashDelay > 0 {
			time.Sleep(m.hashDelay)
		}

		// Compute expected hash
		expectedHash := m.computeEventHash(event, previousHash)
		
		// Verify hash
		if event.EventHash == expectedHash {
			result.VerifiedEvents++
		} else {
			result.CorruptedEvents++
			result.CorruptedSequences = append(result.CorruptedSequences, values.NewSequenceNumber(event.SequenceNum))
		}

		previousHash = event.EventHash

		// Simulate verification delay
		if m.verifyDelay > 0 && i%100 == 0 {
			time.Sleep(m.verifyDelay)
		}

		// Check context cancellation
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
	}

	result.EndTime = time.Now()
	result.IsValid = result.CorruptedEvents == 0
	result.IntegrityScore = float64(result.VerifiedEvents) / float64(result.TotalEvents)

	return result, nil
}

func (m *mockIntegrityService) computeEventHash(event *audit.Event, previousHash string) string {
	h := sha256.New()
	h.Write([]byte(fmt.Sprintf("%s:%s:%s:%s:%d:%s",
		event.ID, event.ActorID, event.Action, event.Timestamp.Format(time.RFC3339),
		event.SequenceNum, previousHash)))
	
	if len(m.secretKey) > 0 {
		h.Write(m.secretKey)
	}
	
	return fmt.Sprintf("%x", h.Sum(nil))
}

func (m *mockIntegrityService) DetectCorruption(ctx context.Context, criteria audit.CorruptionDetectionCriteria) (*audit.CorruptionReport, error) {
	// Simulate corruption detection logic
	time.Sleep(m.verifyDelay * 2) // Corruption detection is more intensive

	return &audit.CorruptionReport{
		ScanStartTime:      time.Now().Add(-m.verifyDelay * 2),
		ScanEndTime:        time.Now(),
		EventsScanned:      int64(len(m.repository.events)),
		CorruptionDetected: false,
		CorruptedRanges:    []audit.CorruptionRange{},
		IntegrityScore:     1.0,
		RecommendedActions: []string{"no_action_required"},
	}, nil
}

// Generate test events with proper hash chain
func generateIntegrityTestData(size int, secretKey []byte) []*audit.Event {
	events := make([]*audit.Event, size)
	baseTime := time.Now().Add(-24 * time.Hour)
	var previousHash string

	for i := 0; i < size; i++ {
		event := &audit.Event{
			ID:          uuid.New(),
			Type:        audit.EventTypeDataAccess,
			Severity:    audit.SeverityLow,
			ActorID:     fmt.Sprintf("actor_%d", i%100),
			TargetID:    uuid.New().String(),
			Action:      fmt.Sprintf("action_%d", i%10),
			Result:      "success",
			Timestamp:   baseTime.Add(time.Duration(i) * time.Minute),
			SequenceNum: int64(i + 1),
		}

		// Compute hash with previous hash
		h := sha256.New()
		h.Write([]byte(fmt.Sprintf("%s:%s:%s:%s:%d:%s",
			event.ID, event.ActorID, event.Action, event.Timestamp.Format(time.RFC3339),
			event.SequenceNum, previousHash)))
		
		if len(secretKey) > 0 {
			h.Write(secretKey)
		}
		
		eventHash := fmt.Sprintf("%x", h.Sum(nil))
		event.EventHash = eventHash
		event.PreviousHash = previousHash

		events[i] = event
		previousHash = eventHash
	}

	return events
}

// Hash chain verification benchmark - core integrity operation
func BenchmarkIntegrity_HashChainVerification(b *testing.B) {
	secretKey := make([]byte, 32)
	for i := range secretKey {
		secretKey[i] = byte(i)
	}

	chainSizes := []int{1000, 5000, 10000, 50000, 100000}

	for _, chainSize := range chainSizes {
		b.Run(fmt.Sprintf("chain_%dk", chainSize/1000), func(b *testing.B) {
			events := generateIntegrityTestData(chainSize, secretKey)
			
			repository := &mockIntegrityRepository{
				events:       events,
				verifyDelay:  0,
				computeDelay: 0,
			}

			service := &mockIntegrityService{
				repository:  repository,
				hashDelay:   time.Nanosecond * 100, // Minimal hash computation delay
				verifyDelay: 0,
				secretKey:   secretKey,
			}

			ctx := context.Background()
			start := values.NewSequenceNumber(1)
			end := values.NewSequenceNumber(int64(chainSize))

			var totalTime time.Duration
			var totalEvents int64

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				startTime := time.Now()

				result, err := service.VerifyHashChain(ctx, start, end)
				if err != nil {
					b.Fatalf("Hash chain verification failed: %v", err)
				}

				verifyTime := time.Since(startTime)
				totalTime += verifyTime
				totalEvents += result.TotalEvents

				// Validate results
				if result.TotalEvents != int64(chainSize) {
					b.Errorf("Expected %d events, got %d", chainSize, result.TotalEvents)
				}

				if !result.IsValid {
					b.Errorf("Hash chain verification failed, integrity score: %f", result.IntegrityScore)
				}

				// Performance validation - should complete quickly
				maxExpectedTime := time.Duration(chainSize/1000) * time.Millisecond * 10 // 10ms per 1K events
				if maxExpectedTime < 10*time.Millisecond {
					maxExpectedTime = 10 * time.Millisecond
				}

				if verifyTime > maxExpectedTime {
					b.Logf("WARNING: Verification of %d events took %v, expected < %v",
						chainSize, verifyTime, maxExpectedTime)
				}
			}

			b.StopTimer()

			avgTime := totalTime / time.Duration(b.N)
			eventsPerSec := float64(totalEvents) / totalTime.Seconds()

			b.ReportMetric(float64(avgTime.Milliseconds()), "avg_time_ms")
			b.ReportMetric(eventsPerSec, "events/sec")
			b.ReportMetric(float64(chainSize), "chain_size")

			b.Logf("Chain size %d: %.2f ms avg, %.0f events/sec", chainSize, avgTime.Seconds()*1000, eventsPerSec)
		})
	}
}

// Hash computation performance benchmark
func BenchmarkIntegrity_HashComputation(b *testing.B) {
	secretKey := make([]byte, 32)
	for i := range secretKey {
		secretKey[i] = byte(i)
	}

	service := &mockIntegrityService{
		secretKey: secretKey,
	}

	// Test different event sizes
	events := []*audit.Event{
		{
			ID:          uuid.New(),
			ActorID:     "simple_actor",
			Action:      "simple_action",
			Timestamp:   time.Now(),
			SequenceNum: 1,
		},
		{
			ID:          uuid.New(),
			ActorID:     "complex_actor_with_long_name",
			Action:      "complex_action_with_detailed_description",
			Timestamp:   time.Now(),
			SequenceNum: 1,
			Metadata: map[string]interface{}{
				"key1": "value1",
				"key2": 12345,
				"key3": []string{"a", "b", "c"},
			},
		},
	}

	for i, event := range events {
		b.Run(fmt.Sprintf("event_type_%d", i), func(b *testing.B) {
			previousHash := "previous_hash_example"

			b.ResetTimer()
			b.ReportAllocs()

			for j := 0; j < b.N; j++ {
				hash := service.computeEventHash(event, previousHash)
				if len(hash) != 64 { // SHA256 hex length
					b.Errorf("Invalid hash length: %d", len(hash))
				}
			}
		})
	}
}

// Corruption detection benchmark
func BenchmarkIntegrity_CorruptionDetection(b *testing.B) {
	secretKey := make([]byte, 32)
	for i := range secretKey {
		secretKey[i] = byte(i)
	}

	dataSizes := []int{10000, 50000, 100000}

	for _, dataSize := range dataSizes {
		b.Run(fmt.Sprintf("dataset_%dk", dataSize/1000), func(b *testing.B) {
			events := generateIntegrityTestData(dataSize, secretKey)
			
			// Introduce some corruption
			corruptionRate := 0.01 // 1% corruption
			corruptCount := int(float64(dataSize) * corruptionRate)
			for i := 0; i < corruptCount; i++ {
				idx := i * (dataSize / corruptCount)
				if idx < len(events) {
					events[idx].EventHash = "corrupted_hash"
				}
			}

			repository := &mockIntegrityRepository{
				events:      events,
				verifyDelay: time.Microsecond * 10, // Minimal delay per verification
			}

			service := &mockIntegrityService{
				repository:  repository,
				verifyDelay: time.Microsecond * 50, // Corruption detection overhead
				secretKey:   secretKey,
			}

			ctx := context.Background()
			criteria := audit.CorruptionDetectionCriteria{
				ScanDepth:      audit.ScanDepthFull,
				TimeRange:      24 * time.Hour,
				SampleRate:     1.0, // Full scan
				AlertThreshold: 0.01,
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				start := time.Now()

				report, err := service.DetectCorruption(ctx, criteria)
				if err != nil {
					b.Fatalf("Corruption detection failed: %v", err)
				}

				detectionTime := time.Since(start)

				// Validate scan completed
				if report.EventsScanned != int64(dataSize) {
					b.Logf("Expected to scan %d events, scanned %d", dataSize, report.EventsScanned)
				}

				// Performance validation
				maxExpectedTime := time.Duration(dataSize/10000) * time.Second // 1 second per 10K events
				if maxExpectedTime < time.Second {
					maxExpectedTime = time.Second
				}

				if detectionTime > maxExpectedTime {
					b.Logf("WARNING: Corruption detection for %d events took %v, expected < %v",
						dataSize, detectionTime, maxExpectedTime)
				}
			}
		})
	}
}

// Incremental verification benchmark - verifying only new events
func BenchmarkIntegrity_IncrementalVerification(b *testing.B) {
	secretKey := make([]byte, 32)
	for i := range secretKey {
		secretKey[i] = byte(i)
	}

	totalEvents := 100000
	events := generateIntegrityTestData(totalEvents, secretKey)

	repository := &mockIntegrityRepository{
		events:      events,
		verifyDelay: 0,
	}

	service := &mockIntegrityService{
		repository:  repository,
		hashDelay:   time.Nanosecond * 100,
		verifyDelay: 0,
		secretKey:   secretKey,
	}

	ctx := context.Background()

	// Test different incremental batch sizes
	incrementSizes := []int{100, 500, 1000, 5000}

	for _, incrementSize := range incrementSizes {
		b.Run(fmt.Sprintf("increment_%d", incrementSize), func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				// Simulate incremental verification - verify last batch of events
				startSeq := int64(totalEvents - incrementSize + 1)
				endSeq := int64(totalEvents)

				start := values.NewSequenceNumber(startSeq)
				end := values.NewSequenceNumber(endSeq)

				startTime := time.Now()
				result, err := service.VerifyHashChain(ctx, start, end)
				verifyTime := time.Since(startTime)

				if err != nil {
					b.Fatalf("Incremental verification failed: %v", err)
				}

				if result.TotalEvents != int64(incrementSize) {
					b.Errorf("Expected %d events, got %d", incrementSize, result.TotalEvents)
				}

				// Incremental verification should be very fast
				maxExpectedTime := time.Duration(incrementSize/100) * time.Millisecond
				if maxExpectedTime < time.Millisecond {
					maxExpectedTime = time.Millisecond
				}

				if verifyTime > maxExpectedTime {
					b.Logf("WARNING: Incremental verification of %d events took %v, expected < %v",
						incrementSize, verifyTime, maxExpectedTime)
				}
			}
		})
	}
}

// Concurrent verification benchmark - multiple parallel verification tasks
func BenchmarkIntegrity_ConcurrentVerification(b *testing.B) {
	secretKey := make([]byte, 32)
	for i := range secretKey {
		secretKey[i] = byte(i)
	}

	events := generateIntegrityTestData(50000, secretKey) // 50K events
	
	repository := &mockIntegrityRepository{
		events:      events,
		verifyDelay: time.Microsecond * 10,
	}

	service := &mockIntegrityService{
		repository:  repository,
		hashDelay:   time.Nanosecond * 100,
		verifyDelay: time.Microsecond * 5,
		secretKey:   secretKey,
	}

	concurrencyLevels := []int{1, 2, 5, 10}

	for _, concurrency := range concurrencyLevels {
		b.Run(fmt.Sprintf("concurrent_%d", concurrency), func(b *testing.B) {
			ctx := context.Background()
			chunkSize := len(events) / concurrency

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				start := time.Now()

				// Run concurrent verifications
				errChan := make(chan error, concurrency)
				for j := 0; j < concurrency; j++ {
					go func(chunkIndex int) {
						startSeq := int64(chunkIndex*chunkSize + 1)
						endSeq := int64((chunkIndex + 1) * chunkSize)
						if endSeq > int64(len(events)) {
							endSeq = int64(len(events))
						}

						start := values.NewSequenceNumber(startSeq)
						end := values.NewSequenceNumber(endSeq)

						_, err := service.VerifyHashChain(ctx, start, end)
						errChan <- err
					}(j)
				}

				// Wait for all verifications to complete
				for j := 0; j < concurrency; j++ {
					if err := <-errChan; err != nil {
						b.Fatalf("Concurrent verification failed: %v", err)
					}
				}

				verifyTime := time.Since(start)

				// Concurrent verification should scale well
				expectedSpeedup := float64(concurrency) * 0.7 // 70% efficiency
				maxExpectedTime := time.Duration(float64(len(events)) / (expectedSpeedup * 10000) * float64(time.Second))

				if verifyTime > maxExpectedTime {
					b.Logf("WARNING: Concurrent verification with %d workers took %v, expected < %v",
						concurrency, verifyTime, maxExpectedTime)
				}
			}
		})
	}
}

// Cache performance benchmark for integrity operations
func BenchmarkIntegrity_CachePerformance(b *testing.B) {
	secretKey := make([]byte, 32)
	for i := range secretKey {
		secretKey[i] = byte(i)
	}

	events := generateIntegrityTestData(20000, secretKey) // 20K events

	// Simulate cache hit/miss scenarios
	scenarios := []struct {
		name        string
		cacheHitRate float64
		cacheDelay   time.Duration
	}{
		{"no_cache", 0.0, 0},
		{"cache_50_percent", 0.5, time.Microsecond * 10},
		{"cache_90_percent", 0.9, time.Microsecond * 10},
		{"cache_perfect", 1.0, time.Microsecond * 5},
	}

	for _, scenario := range scenarios {
		b.Run(scenario.name, func(b *testing.B) {
			repository := &mockIntegrityRepository{
				events: events,
				verifyDelay: func() time.Duration {
					// Simulate cache hit/miss
					if scenario.cacheHitRate > 0 && b.N % int(1.0/scenario.cacheHitRate) == 0 {
						return scenario.cacheDelay // Cache hit
					}
					return time.Microsecond * 100 // Cache miss - full computation
				}(),
			}

			service := &mockIntegrityService{
				repository:  repository,
				hashDelay:   time.Nanosecond * 50,
				verifyDelay: time.Microsecond * 10,
				secretKey:   secretKey,
			}

			ctx := context.Background()
			start := values.NewSequenceNumber(1)
			end := values.NewSequenceNumber(1000) // Verify 1K events

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				_, err := service.VerifyHashChain(ctx, start, end)
				if err != nil {
					b.Fatalf("Cache verification failed: %v", err)
				}
			}

			b.ReportMetric(scenario.cacheHitRate*100, "cache_hit_percent")
		})
	}
}

// Memory efficiency benchmark for integrity operations
func BenchmarkIntegrity_MemoryEfficiency(b *testing.B) {
	secretKey := make([]byte, 32)
	for i := range secretKey {
		secretKey[i] = byte(i)
	}

	chainSizes := []int{1000, 10000, 100000}

	for _, chainSize := range chainSizes {
		b.Run(fmt.Sprintf("chain_%dk", chainSize/1000), func(b *testing.B) {
			events := generateIntegrityTestData(chainSize, secretKey)
			
			repository := &mockIntegrityRepository{
				events:      events,
				verifyDelay: 0,
			}

			service := &mockIntegrityService{
				repository:  repository,
				hashDelay:   time.Nanosecond * 50,
				verifyDelay: 0,
				secretKey:   secretKey,
			}

			ctx := context.Background()
			start := values.NewSequenceNumber(1)
			end := values.NewSequenceNumber(int64(chainSize))

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				result, err := service.VerifyHashChain(ctx, start, end)
				if err != nil {
					b.Fatalf("Memory efficiency verification failed: %v", err)
				}

				// Ensure we processed all events
				if result.TotalEvents != int64(chainSize) {
					b.Errorf("Expected %d events, got %d", chainSize, result.TotalEvents)
				}
			}

			// Memory usage should be proportional to chain size
			// but verification should not load entire chain into memory at once
		})
	}
}

// Performance regression test for integrity operations
func BenchmarkIntegrity_PerformanceRegression(b *testing.B) {
	// Standard test conditions for regression detection
	secretKey := make([]byte, 32)
	for i := range secretKey {
		secretKey[i] = byte(i)
	}

	events := generateIntegrityTestData(10000, secretKey) // 10K events
	
	repository := &mockIntegrityRepository{
		events:      events,
		verifyDelay: 0,
	}

	service := &mockIntegrityService{
		repository:  repository,
		hashDelay:   time.Nanosecond * 100,
		verifyDelay: 0,
		secretKey:   secretKey,
	}

	ctx := context.Background()
	start := values.NewSequenceNumber(1)
	end := values.NewSequenceNumber(int64(len(events)))

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		result, err := service.VerifyHashChain(ctx, start, end)
		if err != nil {
			b.Fatalf("Regression test verification failed: %v", err)
		}

		if !result.IsValid {
			b.Errorf("Hash chain verification failed in regression test")
		}
	}

	// Performance baseline (adjust based on hardware)
	opsPerSec := float64(b.N) / b.Elapsed().Seconds()
	eventsPerSec := float64(len(events)) * opsPerSec

	b.ReportMetric(opsPerSec, "verifications/sec")
	b.ReportMetric(eventsPerSec, "events_verified/sec")

	// Minimum acceptable performance for regression detection
	minEventsPerSec := 50000.0 // 50K events/sec minimum
	if eventsPerSec < minEventsPerSec {
		b.Errorf("Performance regression: %.0f events/sec below minimum %.0f events/sec",
			eventsPerSec, minEventsPerSec)
	}
}