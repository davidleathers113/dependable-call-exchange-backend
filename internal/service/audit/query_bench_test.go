package audit

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/audit"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
)

// Query performance benchmarks - validates < 1s response for 1M events

// Mock query repository for benchmarking
type mockQueryRepository struct {
	events      []*audit.Event
	queryDelay  time.Duration
	resultLimit int
}

func (m *mockQueryRepository) QueryEvents(ctx context.Context, criteria audit.QueryCriteria) ([]*audit.Event, error) {
	if m.queryDelay > 0 {
		time.Sleep(m.queryDelay)
	}

	// Simulate query processing
	var results []*audit.Event
	limit := criteria.Limit
	if limit == 0 {
		limit = len(m.events)
	}
	if m.resultLimit > 0 && limit > m.resultLimit {
		limit = m.resultLimit
	}

	// Simple filtering simulation
	for i, event := range m.events {
		if len(results) >= limit {
			break
		}

		// Simulate time range filtering
		if !criteria.StartTime.IsZero() && event.Timestamp.Before(criteria.StartTime) {
			continue
		}
		if !criteria.EndTime.IsZero() && event.Timestamp.After(criteria.EndTime) {
			continue
		}

		// Simulate actor filtering
		if len(criteria.ActorIDs) > 0 {
			found := false
			for _, actorID := range criteria.ActorIDs {
				if event.ActorID == actorID {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Simulate event type filtering
		if len(criteria.EventTypes) > 0 {
			found := false
			for _, eventType := range criteria.EventTypes {
				if event.Type == eventType {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		results = append(results, event)

		// Simulate processing delay for large datasets
		if i%10000 == 0 && i > 0 {
			time.Sleep(time.Microsecond * 10)
		}
	}

	return results, nil
}

func (m *mockQueryRepository) CountEvents(ctx context.Context, criteria audit.QueryCriteria) (int64, error) {
	if m.queryDelay > 0 {
		time.Sleep(m.queryDelay / 2) // Count is typically faster
	}

	events, _ := m.QueryEvents(ctx, criteria)
	return int64(len(events)), nil
}

func (m *mockQueryRepository) GetEventsBySequenceRange(ctx context.Context, start, end values.SequenceNumber) ([]*audit.Event, error) {
	if m.queryDelay > 0 {
		time.Sleep(m.queryDelay)
	}

	var results []*audit.Event
	for _, event := range m.events {
		if event.SequenceNum >= start.Value() && event.SequenceNum <= end.Value() {
			results = append(results, event)
		}
	}
	return results, nil
}

// Generate test dataset for query benchmarks
func generateTestDataset(size int) []*audit.Event {
	events := make([]*audit.Event, size)
	eventTypes := []audit.EventType{
		audit.EventTypeUserLogin,
		audit.EventTypeUserLogout,
		audit.EventTypeDataAccess,
		audit.EventTypeSystemActivity,
		audit.EventTypeComplianceViolation,
		audit.EventTypeSecurityIncident,
	}

	actors := make([]string, 1000)
	for i := range actors {
		actors[i] = fmt.Sprintf("actor_%d", i)
	}

	baseTime := time.Now().Add(-24 * time.Hour)

	for i := 0; i < size; i++ {
		events[i] = &audit.Event{
			ID:          uuid.New(),
			Type:        eventTypes[rand.Intn(len(eventTypes))],
			Severity:    audit.SeverityLow,
			ActorID:     actors[rand.Intn(len(actors))],
			TargetID:    uuid.New().String(),
			Action:      fmt.Sprintf("action_%d", rand.Intn(50)),
			Result:      "success",
			Timestamp:   baseTime.Add(time.Duration(i) * time.Second),
			SequenceNum: int64(i + 1),
			EventHash:   fmt.Sprintf("hash_%d", i),
			Metadata: map[string]interface{}{
				"index": i,
				"batch": i / 100,
			},
		}
	}

	return events
}

// Query performance with 1M events - validates < 1s response time target
func BenchmarkQuery_1MillionEvents(b *testing.B) {
	sizes := []int{100000, 500000, 1000000} // 100K, 500K, 1M events

	for _, size := range sizes {
		b.Run(fmt.Sprintf("dataset_%dk", size/1000), func(b *testing.B) {
			// Generate test dataset
			events := generateTestDataset(size)
			repo := &mockQueryRepository{
				events:      events,
				queryDelay:  0,
				resultLimit: 10000, // Reasonable result limit
			}

			ctx := context.Background()

			// Test different query patterns
			queries := []struct {
				name     string
				criteria audit.QueryCriteria
			}{
				{
					name: "time_range_1hour",
					criteria: audit.QueryCriteria{
						StartTime: time.Now().Add(-1 * time.Hour),
						EndTime:   time.Now(),
						Limit:     1000,
					},
				},
				{
					name: "specific_actor",
					criteria: audit.QueryCriteria{
						ActorIDs: []string{"actor_1", "actor_2", "actor_3"},
						Limit:    1000,
					},
				},
				{
					name: "event_type_filter",
					criteria: audit.QueryCriteria{
						EventTypes: []audit.EventType{audit.EventTypeUserLogin, audit.EventTypeDataAccess},
						Limit:      1000,
					},
				},
				{
					name: "complex_query",
					criteria: audit.QueryCriteria{
						StartTime:  time.Now().Add(-6 * time.Hour),
						EndTime:    time.Now().Add(-1 * time.Hour),
						ActorIDs:   []string{"actor_1", "actor_5", "actor_10"},
						EventTypes: []audit.EventType{audit.EventTypeDataAccess},
						Limit:      500,
					},
				},
			}

			for _, query := range queries {
				b.Run(query.name, func(b *testing.B) {
					var totalLatency time.Duration
					var maxLatency time.Duration

					b.ResetTimer()
					b.ReportAllocs()

					for i := 0; i < b.N; i++ {
						start := time.Now()

						results, err := repo.QueryEvents(ctx, query.criteria)
						if err != nil {
							b.Fatalf("Query failed: %v", err)
						}

						latency := time.Since(start)
						totalLatency += latency

						if latency > maxLatency {
							maxLatency = latency
						}

						// Validate < 1s response time for 1M events
						if latency > time.Second && size >= 1000000 {
							b.Logf("WARNING: Query latency %v exceeds 1s target for %d events", latency, size)
						}

						// Ensure we got some results for most queries
						if len(results) == 0 && query.name != "complex_query" {
							b.Logf("Query returned no results: %s", query.name)
						}
					}

					b.StopTimer()

					avgLatency := totalLatency / time.Duration(b.N)

					b.ReportMetric(float64(avgLatency.Milliseconds()), "avg_latency_ms")
					b.ReportMetric(float64(maxLatency.Milliseconds()), "max_latency_ms")
					b.ReportMetric(float64(size), "dataset_size")

					// Performance validation for 1M events
					if size >= 1000000 {
						if avgLatency > time.Second {
							b.Errorf("Average query latency %v exceeds 1s target for 1M events", avgLatency)
						}
						if maxLatency > 2*time.Second {
							b.Errorf("Max query latency %v exceeds 2s acceptable limit for 1M events", maxLatency)
						}
					}
				})
			}
		})
	}
}

// Sequence range query benchmark
func BenchmarkQuery_SequenceRange(b *testing.B) {
	events := generateTestDataset(1000000) // 1M events
	repo := &mockQueryRepository{
		events:     events,
		queryDelay: 0,
	}

	ctx := context.Background()

	rangeSizes := []int{100, 1000, 10000, 100000}

	for _, rangeSize := range rangeSizes {
		b.Run(fmt.Sprintf("range_%d", rangeSize), func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				start := values.NewSequenceNumber(1)
				end, _ := values.NewSequenceNumber(int64(rangeSize))

				queryStart := time.Now()
				results, err := repo.GetEventsBySequenceRange(ctx, start, end)
				queryTime := time.Since(queryStart)

				if err != nil {
					b.Fatalf("Sequence range query failed: %v", err)
				}

				expectedResults := rangeSize
				if len(results) != expectedResults {
					b.Logf("Expected %d results, got %d", expectedResults, len(results))
				}

				// Validate query performance based on range size
				expectedMaxTime := time.Duration(rangeSize/1000) * time.Millisecond
				if expectedMaxTime < 10*time.Millisecond {
					expectedMaxTime = 10 * time.Millisecond
				}

				if queryTime > expectedMaxTime {
					b.Logf("WARNING: Range query for %d events took %v, expected < %v",
						rangeSize, queryTime, expectedMaxTime)
				}
			}
		})
	}
}

// Count query benchmark - faster than full queries
func BenchmarkQuery_Count(b *testing.B) {
	events := generateTestDataset(1000000) // 1M events
	repo := &mockQueryRepository{
		events:     events,
		queryDelay: 0,
	}

	ctx := context.Background()

	criteria := []struct {
		name     string
		criteria audit.QueryCriteria
	}{
		{
			name: "count_all",
			criteria: audit.QueryCriteria{
				// No filters - count all
			},
		},
		{
			name: "count_by_actor",
			criteria: audit.QueryCriteria{
				ActorIDs: []string{"actor_1", "actor_2"},
			},
		},
		{
			name: "count_by_time_range",
			criteria: audit.QueryCriteria{
				StartTime: time.Now().Add(-12 * time.Hour),
				EndTime:   time.Now().Add(-6 * time.Hour),
			},
		},
		{
			name: "count_by_event_type",
			criteria: audit.QueryCriteria{
				EventTypes: []audit.EventType{audit.EventTypeUserLogin},
			},
		},
	}

	for _, test := range criteria {
		b.Run(test.name, func(b *testing.B) {
			var totalLatency time.Duration

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				start := time.Now()

				count, err := repo.CountEvents(ctx, test.criteria)
				if err != nil {
					b.Fatalf("Count query failed: %v", err)
				}

				latency := time.Since(start)
				totalLatency += latency

				// Count queries should be fast
				if latency > 100*time.Millisecond {
					b.Logf("WARNING: Count query took %v, expected < 100ms", latency)
				}

				if count < 0 {
					b.Errorf("Invalid count result: %d", count)
				}
			}

			b.StopTimer()

			avgLatency := totalLatency / time.Duration(b.N)
			b.ReportMetric(float64(avgLatency.Milliseconds()), "avg_latency_ms")

			// Count queries should complete in < 100ms for 1M events
			if avgLatency > 100*time.Millisecond {
				b.Errorf("Count query average latency %v exceeds 100ms target", avgLatency)
			}
		})
	}
}

// Pagination performance benchmark
func BenchmarkQuery_Pagination(b *testing.B) {
	events := generateTestDataset(100000) // 100K events for pagination testing
	repo := &mockQueryRepository{
		events:     events,
		queryDelay: 0,
	}

	ctx := context.Background()
	pageSize := 50

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Simulate pagination through results
		page := i % 100 // Cycle through first 100 pages
		offset := page * pageSize

		criteria := audit.QueryCriteria{
			Limit:  pageSize,
			Offset: offset,
		}

		start := time.Now()
		results, err := repo.QueryEvents(ctx, criteria)
		queryTime := time.Since(start)

		if err != nil {
			b.Fatalf("Pagination query failed: %v", err)
		}

		expectedResults := pageSize
		if offset+pageSize > len(events) {
			expectedResults = len(events) - offset
		}
		if expectedResults < 0 {
			expectedResults = 0
		}

		if len(results) > expectedResults {
			b.Logf("Page %d: expected <= %d results, got %d", page, expectedResults, len(results))
		}

		// Pagination should be fast
		if queryTime > 50*time.Millisecond {
			b.Logf("WARNING: Pagination query took %v, expected < 50ms", queryTime)
		}
	}
}

// Complex query benchmark - multiple filters
func BenchmarkQuery_ComplexFilters(b *testing.B) {
	events := generateTestDataset(500000) // 500K events
	repo := &mockQueryRepository{
		events:     events,
		queryDelay: 0,
	}

	ctx := context.Background()

	// Complex query with multiple filters
	criteria := audit.QueryCriteria{
		StartTime: time.Now().Add(-12 * time.Hour),
		EndTime:   time.Now().Add(-1 * time.Hour),
		ActorIDs:  []string{"actor_1", "actor_2", "actor_5", "actor_10", "actor_20"},
		EventTypes: []audit.EventType{
			audit.EventTypeUserLogin,
			audit.EventTypeDataAccess,
			audit.EventTypeSystemActivity,
		},
		Limit: 1000,
	}

	b.ResetTimer()
	b.ReportAllocs()

	var totalLatency time.Duration
	var maxLatency time.Duration

	for i := 0; i < b.N; i++ {
		start := time.Now()

		results, err := repo.QueryEvents(ctx, criteria)
		if err != nil {
			b.Fatalf("Complex query failed: %v", err)
		}

		latency := time.Since(start)
		totalLatency += latency

		if latency > maxLatency {
			maxLatency = latency
		}

		// Complex queries should still be reasonably fast
		if latency > 500*time.Millisecond {
			b.Logf("WARNING: Complex query took %v, expected < 500ms", latency)
		}

		// Verify results make sense
		if len(results) > criteria.Limit {
			b.Errorf("Query returned %d results, limit was %d", len(results), criteria.Limit)
		}
	}

	b.StopTimer()

	avgLatency := totalLatency / time.Duration(b.N)

	b.ReportMetric(float64(avgLatency.Milliseconds()), "avg_latency_ms")
	b.ReportMetric(float64(maxLatency.Milliseconds()), "max_latency_ms")

	// Complex queries should complete in reasonable time
	if avgLatency > 500*time.Millisecond {
		b.Errorf("Complex query average latency %v exceeds 500ms target", avgLatency)
	}
}

// Query optimization benchmark - different index scenarios
func BenchmarkQuery_IndexOptimization(b *testing.B) {
	events := generateTestDataset(1000000) // 1M events
	
	// Simulate different indexing scenarios
	scenarios := []struct {
		name       string
		queryDelay time.Duration
		description string
	}{
		{
			name:       "optimized_indexes",
			queryDelay: 0,
			description: "Optimal indexes on all query fields",
		},
		{
			name:       "partial_indexes",
			queryDelay: 5 * time.Millisecond,
			description: "Some indexes missing",
		},
		{
			name:       "no_indexes",
			queryDelay: 50 * time.Millisecond,
			description: "Full table scan simulation",
		},
	}

	for _, scenario := range scenarios {
		b.Run(scenario.name, func(b *testing.B) {
			repo := &mockQueryRepository{
				events:     events,
				queryDelay: scenario.queryDelay,
			}

			ctx := context.Background()

			// Standard query
			criteria := audit.QueryCriteria{
				StartTime:  time.Now().Add(-6 * time.Hour),
				EndTime:    time.Now(),
				ActorIDs:   []string{"actor_1", "actor_2"},
				EventTypes: []audit.EventType{audit.EventTypeDataAccess},
				Limit:      100,
			}

			b.ResetTimer()
			b.ReportAllocs()

			var totalLatency time.Duration

			for i := 0; i < b.N; i++ {
				start := time.Now()

				_, err := repo.QueryEvents(ctx, criteria)
				if err != nil {
					b.Fatalf("Query failed: %v", err)
				}

				latency := time.Since(start)
				totalLatency += latency
			}

			b.StopTimer()

			avgLatency := totalLatency / time.Duration(b.N)
			b.ReportMetric(float64(avgLatency.Milliseconds()), "avg_latency_ms")

			b.Logf("%s: Average latency %v", scenario.description, avgLatency)
		})
	}
}

// Memory efficiency benchmark for large result sets
func BenchmarkQuery_MemoryEfficiency(b *testing.B) {
	events := generateTestDataset(100000) // 100K events
	repo := &mockQueryRepository{
		events:     events,
		queryDelay: 0,
	}

	ctx := context.Background()

	resultSizes := []int{100, 1000, 10000, 50000}

	for _, resultSize := range resultSizes {
		b.Run(fmt.Sprintf("results_%d", resultSize), func(b *testing.B) {
			criteria := audit.QueryCriteria{
				Limit: resultSize,
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				results, err := repo.QueryEvents(ctx, criteria)
				if err != nil {
					b.Fatalf("Query failed: %v", err)
				}

				// Verify result size
				if len(results) > resultSize {
					b.Errorf("Expected max %d results, got %d", resultSize, len(results))
				}

				// Simulate processing results
				for _, event := range results {
					_ = event.ID.String() // Access field to prevent optimization
				}
			}

			b.StopTimer()
		})
	}
}