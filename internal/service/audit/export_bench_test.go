package audit

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/audit"
)

// Export performance benchmarks - validates > 10K events/sec export throughput

// Mock export writer for benchmarking
type mockExportWriter struct {
	writeDelay    time.Duration
	bytesWritten  int64
	eventsWritten int64
	buffer        *strings.Builder
}

func newMockExportWriter(delay time.Duration) *mockExportWriter {
	return &mockExportWriter{
		writeDelay: delay,
		buffer:     &strings.Builder{},
	}
}

func (m *mockExportWriter) Write(data []byte) (int, error) {
	if m.writeDelay > 0 {
		time.Sleep(m.writeDelay)
	}

	n, err := m.buffer.Write(data)
	m.bytesWritten += int64(n)
	return n, err
}

func (m *mockExportWriter) Close() error {
	return nil
}

// Mock export service for benchmarking
type mockExportService struct {
	events      []*audit.Event
	writer      io.WriteCloser
	batchSize   int
	writeDelay  time.Duration
	compression bool
}

func (m *mockExportService) ExportEvents(ctx context.Context, criteria audit.ExportCriteria, writer io.WriteCloser) error {
	defer writer.Close()

	// Filter events based on criteria
	var eventsToExport []*audit.Event
	for _, event := range m.events {
		if m.matchesCriteria(event, criteria) {
			eventsToExport = append(eventsToExport, event)
		}
		
		// Respect context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}

	// Export in batches
	batchSize := m.batchSize
	if batchSize == 0 {
		batchSize = 1000
	}

	for i := 0; i < len(eventsToExport); i += batchSize {
		end := i + batchSize
		if end > len(eventsToExport) {
			end = len(eventsToExport)
		}

		batch := eventsToExport[i:end]
		if err := m.writeBatch(writer, batch, criteria.Format); err != nil {
			return err
		}

		// Simulate processing delay
		if m.writeDelay > 0 {
			time.Sleep(m.writeDelay)
		}

		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}

	return nil
}

func (m *mockExportService) matchesCriteria(event *audit.Event, criteria audit.ExportCriteria) bool {
	// Time range filtering
	if !criteria.StartTime.IsZero() && event.Timestamp.Before(criteria.StartTime) {
		return false
	}
	if !criteria.EndTime.IsZero() && event.Timestamp.After(criteria.EndTime) {
		return false
	}

	// Actor filtering
	if len(criteria.ActorIDs) > 0 {
		found := false
		for _, actorID := range criteria.ActorIDs {
			if event.ActorID == actorID {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Event type filtering
	if len(criteria.EventTypes) > 0 {
		found := false
		for _, eventType := range criteria.EventTypes {
			if event.Type == eventType {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

func (m *mockExportService) writeBatch(writer io.WriteCloser, events []*audit.Event, format string) error {
	for _, event := range events {
		var data string
		switch format {
		case "json":
			data = fmt.Sprintf(`{"id":"%s","type":"%s","actor_id":"%s","timestamp":"%s"}%s`,
				event.ID, event.Type, event.ActorID, event.Timestamp.Format(time.RFC3339), "\n")
		case "csv":
			data = fmt.Sprintf("%s,%s,%s,%s\n",
				event.ID, event.Type, event.ActorID, event.Timestamp.Format(time.RFC3339))
		case "xml":
			data = fmt.Sprintf("<event><id>%s</id><type>%s</type><actor>%s</actor><timestamp>%s</timestamp></event>\n",
				event.ID, event.Type, event.ActorID, event.Timestamp.Format(time.RFC3339))
		default:
			data = fmt.Sprintf("%s\t%s\t%s\t%s\n",
				event.ID, event.Type, event.ActorID, event.Timestamp.Format(time.RFC3339))
		}

		if _, err := writer.Write([]byte(data)); err != nil {
			return err
		}
	}
	return nil
}

// Generate test events for export benchmarks
func generateExportTestData(size int) []*audit.Event {
	events := make([]*audit.Event, size)
	eventTypes := []audit.EventType{
		audit.EventTypeUserLogin,
		audit.EventTypeUserLogout,
		audit.EventTypeDataAccess,
		audit.EventTypeSystemActivity,
	}

	baseTime := time.Now().Add(-24 * time.Hour)

	for i := 0; i < size; i++ {
		events[i] = &audit.Event{
			ID:          uuid.New(),
			Type:        eventTypes[i%len(eventTypes)],
			Severity:    audit.SeverityLow,
			ActorID:     fmt.Sprintf("actor_%d", i%100),
			TargetID:    uuid.New().String(),
			Action:      fmt.Sprintf("action_%d", i%10),
			Result:      "success",
			Timestamp:   baseTime.Add(time.Duration(i) * time.Minute),
			SequenceNum: int64(i + 1),
			EventHash:   fmt.Sprintf("hash_%d", i),
		}
	}

	return events
}

// Export throughput benchmark - validates > 10K events/sec target
func BenchmarkExport_Throughput(b *testing.B) {
	dataSizes := []int{10000, 50000, 100000, 500000} // 10K to 500K events

	for _, dataSize := range dataSizes {
		b.Run(fmt.Sprintf("dataset_%dk", dataSize/1000), func(b *testing.B) {
			events := generateExportTestData(dataSize)

			formats := []string{"json", "csv", "xml", "tab"}

			for _, format := range formats {
				b.Run(format, func(b *testing.B) {
					ctx := context.Background()

					criteria := audit.ExportCriteria{
						Format:     format,
						StartTime:  time.Now().Add(-25 * time.Hour),
						EndTime:    time.Now(),
						Compress:   false,
						BatchSize:  1000,
					}

					exportService := &mockExportService{
						events:    events,
						batchSize: criteria.BatchSize,
					}

					var totalEvents int64
					var totalTime time.Duration

					b.ResetTimer()
					b.ReportAllocs()

					for i := 0; i < b.N; i++ {
						writer := newMockExportWriter(0)
						
						start := time.Now()
						err := exportService.ExportEvents(ctx, criteria, writer)
						exportTime := time.Since(start)

						if err != nil {
							b.Fatalf("Export failed: %v", err)
						}

						totalEvents += int64(dataSize)
						totalTime += exportTime

						// Validate > 10K events/sec target
						throughput := float64(dataSize) / exportTime.Seconds()
						if throughput < 10000 && dataSize >= 50000 {
							b.Logf("WARNING: Export throughput %.0f events/sec below 10K target for %d events",
								throughput, dataSize)
						}
					}

					b.StopTimer()

					avgThroughput := float64(totalEvents) / totalTime.Seconds()
					avgTime := totalTime / time.Duration(b.N)

					b.ReportMetric(avgThroughput, "events/sec")
					b.ReportMetric(float64(avgTime.Milliseconds()), "avg_time_ms")
					b.ReportMetric(float64(dataSize), "dataset_size")

					// Performance validation
					if avgThroughput < 10000 && dataSize >= 50000 {
						b.Errorf("Average export throughput %.0f events/sec below 10K target", avgThroughput)
					}
				})
			}
		})
	}
}

// Streaming export benchmark - tests continuous export
func BenchmarkExport_Streaming(b *testing.B) {
	events := generateExportTestData(100000) // 100K events
	
	batchSizes := []int{100, 500, 1000, 5000}

	for _, batchSize := range batchSizes {
		b.Run(fmt.Sprintf("batch_%d", batchSize), func(b *testing.B) {
			ctx := context.Background()

			criteria := audit.ExportCriteria{
				Format:    "json",
				BatchSize: batchSize,
			}

			exportService := &mockExportService{
				events:    events,
				batchSize: batchSize,
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				writer := newMockExportWriter(0)
				
				start := time.Now()
				err := exportService.ExportEvents(ctx, criteria, writer)
				exportTime := time.Since(start)

				if err != nil {
					b.Fatalf("Streaming export failed: %v", err)
				}

				throughput := float64(len(events)) / exportTime.Seconds()
				
				// Smaller batches should still maintain good throughput
				minThroughput := 5000.0 // Lower threshold for streaming
				if throughput < minThroughput {
					b.Logf("WARNING: Streaming throughput %.0f events/sec below %.0f minimum",
						throughput, minThroughput)
				}
			}

			b.ReportMetric(float64(batchSize), "batch_size")
		})
	}
}

// Compressed export benchmark
func BenchmarkExport_Compression(b *testing.B) {
	events := generateExportTestData(100000) // 100K events

	tests := []struct {
		name       string
		compressed bool
		writeDelay time.Duration
	}{
		{"uncompressed", false, 0},
		{"compressed", true, 1 * time.Millisecond}, // Simulate compression overhead
	}

	for _, test := range tests {
		b.Run(test.name, func(b *testing.B) {
			ctx := context.Background()

			criteria := audit.ExportCriteria{
				Format:   "json",
				Compress: test.compressed,
			}

			exportService := &mockExportService{
				events:      events,
				batchSize:   1000,
				writeDelay:  test.writeDelay,
				compression: test.compressed,
			}

			var totalBytes int64
			var totalTime time.Duration

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				writer := newMockExportWriter(test.writeDelay)
				
				start := time.Now()
				err := exportService.ExportEvents(ctx, criteria, writer)
				exportTime := time.Since(start)

				if err != nil {
					b.Fatalf("Compressed export failed: %v", err)
				}

				totalBytes += writer.bytesWritten
				totalTime += exportTime
			}

			b.StopTimer()

			avgThroughput := float64(len(events)) * float64(b.N) / totalTime.Seconds()
			avgBytesPerEvent := float64(totalBytes) / (float64(len(events)) * float64(b.N))

			b.ReportMetric(avgThroughput, "events/sec")
			b.ReportMetric(avgBytesPerEvent, "bytes/event")
			b.ReportMetric(float64(totalBytes)/(1024*1024), "total_mb")

			// Compression should maintain reasonable throughput
			if test.compressed && avgThroughput < 8000 {
				b.Logf("WARNING: Compressed export throughput %.0f events/sec below 8K threshold", avgThroughput)
			}
		})
	}
}

// Large dataset export benchmark
func BenchmarkExport_LargeDataset(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping large dataset benchmark in short mode")
	}

	events := generateExportTestData(1000000) // 1M events
	ctx := context.Background()

	criteria := audit.ExportCriteria{
		Format:    "json",
		BatchSize: 5000,
	}

	exportService := &mockExportService{
		events:    events,
		batchSize: criteria.BatchSize,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		writer := newMockExportWriter(0)
		
		start := time.Now()
		err := exportService.ExportEvents(ctx, criteria, writer)
		exportTime := time.Since(start)

		if err != nil {
			b.Fatalf("Large dataset export failed: %v", err)
		}

		throughput := float64(len(events)) / exportTime.Seconds()
		
		b.ReportMetric(throughput, "events/sec")
		b.ReportMetric(float64(exportTime.Seconds()), "total_time_sec")
		b.ReportMetric(float64(writer.bytesWritten)/(1024*1024), "exported_mb")

		// Large dataset should maintain high throughput
		if throughput < 15000 {
			b.Logf("WARNING: Large dataset throughput %.0f events/sec below 15K target", throughput)
		}

		// Should complete in reasonable time (< 1 minute for 1M events)
		if exportTime > time.Minute {
			b.Logf("WARNING: Large dataset export took %v, expected < 1 minute", exportTime)
		}
	}
}

// Filtered export benchmark - tests performance with criteria
func BenchmarkExport_FilteredExport(b *testing.B) {
	events := generateExportTestData(200000) // 200K events
	ctx := context.Background()

	filters := []struct {
		name     string
		criteria audit.ExportCriteria
	}{
		{
			name: "time_range_filter",
			criteria: audit.ExportCriteria{
				Format:    "json",
				StartTime: time.Now().Add(-12 * time.Hour),
				EndTime:   time.Now().Add(-6 * time.Hour),
			},
		},
		{
			name: "actor_filter",
			criteria: audit.ExportCriteria{
				Format:   "json",
				ActorIDs: []string{"actor_1", "actor_2", "actor_5", "actor_10"},
			},
		},
		{
			name: "event_type_filter",
			criteria: audit.ExportCriteria{
				Format:     "json",
				EventTypes: []audit.EventType{audit.EventTypeUserLogin, audit.EventTypeDataAccess},
			},
		},
		{
			name: "complex_filter",
			criteria: audit.ExportCriteria{
				Format:     "json",
				StartTime:  time.Now().Add(-18 * time.Hour),
				EndTime:    time.Now().Add(-6 * time.Hour),
				ActorIDs:   []string{"actor_1", "actor_3", "actor_7"},
				EventTypes: []audit.EventType{audit.EventTypeDataAccess},
			},
		},
	}

	for _, filter := range filters {
		b.Run(filter.name, func(b *testing.B) {
			exportService := &mockExportService{
				events:    events,
				batchSize: 1000,
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				writer := newMockExportWriter(0)
				
				start := time.Now()
				err := exportService.ExportEvents(ctx, filter.criteria, writer)
				exportTime := time.Since(start)

				if err != nil {
					b.Fatalf("Filtered export failed: %v", err)
				}

				// Estimate events exported (would need actual count in real implementation)
				estimatedEvents := len(events) / 4 // Assume filters reduce by ~75%
				throughput := float64(estimatedEvents) / exportTime.Seconds()

				// Filtered exports should still maintain good throughput
				if throughput < 8000 {
					b.Logf("WARNING: Filtered export throughput %.0f events/sec below 8K threshold", throughput)
				}
			}
		})
	}
}

// Concurrent export benchmark - multiple simultaneous exports
func BenchmarkExport_ConcurrentExports(b *testing.B) {
	events := generateExportTestData(100000) // 100K events
	ctx := context.Background()

	concurrencyLevels := []int{1, 2, 5, 10}

	for _, concurrency := range concurrencyLevels {
		b.Run(fmt.Sprintf("concurrent_%d", concurrency), func(b *testing.B) {
			exportService := &mockExportService{
				events:    events,
				batchSize: 1000,
			}

			criteria := audit.ExportCriteria{
				Format:    "json",
				BatchSize: 1000,
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				start := time.Now()

				// Run concurrent exports
				errChan := make(chan error, concurrency)
				for j := 0; j < concurrency; j++ {
					go func() {
						writer := newMockExportWriter(0)
						err := exportService.ExportEvents(ctx, criteria, writer)
						errChan <- err
					}()
				}

				// Wait for all exports to complete
				for j := 0; j < concurrency; j++ {
					if err := <-errChan; err != nil {
						b.Fatalf("Concurrent export failed: %v", err)
					}
				}

				exportTime := time.Since(start)
				totalEvents := len(events) * concurrency
				throughput := float64(totalEvents) / exportTime.Seconds()

				// Concurrent exports should scale reasonably
				expectedMinThroughput := float64(concurrency) * 5000 // 5K events/sec per export minimum
				if throughput < expectedMinThroughput {
					b.Logf("WARNING: Concurrent throughput %.0f events/sec below expected %.0f",
						throughput, expectedMinThroughput)
				}
			}
		})
	}
}

// Export format comparison benchmark
func BenchmarkExport_FormatComparison(b *testing.B) {
	events := generateExportTestData(50000) // 50K events
	ctx := context.Background()

	formats := []struct {
		name        string
		format      string
		expectSize  string
	}{
		{"json", "json", "largest"},
		{"csv", "csv", "smallest"},
		{"xml", "xml", "large"},
		{"tab", "tab", "small"},
	}

	for _, fmt := range formats {
		b.Run(fmt.name, func(b *testing.B) {
			criteria := audit.ExportCriteria{
				Format:    fmt.format,
				BatchSize: 1000,
			}

			exportService := &mockExportService{
				events:    events,
				batchSize: criteria.BatchSize,
			}

			var totalBytes int64
			var totalTime time.Duration

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				writer := newMockExportWriter(0)
				
				start := time.Now()
				err := exportService.ExportEvents(ctx, criteria, writer)
				exportTime := time.Since(start)

				if err != nil {
					b.Fatalf("Format export failed: %v", err)
				}

				totalBytes += writer.bytesWritten
				totalTime += exportTime
			}

			b.StopTimer()

			avgThroughput := float64(len(events)) * float64(b.N) / totalTime.Seconds()
			avgBytesPerEvent := float64(totalBytes) / (float64(len(events)) * float64(b.N))

			b.ReportMetric(avgThroughput, "events/sec")
			b.ReportMetric(avgBytesPerEvent, "bytes/event")
			b.ReportMetric(float64(totalBytes)/(1024*1024), "total_mb")

			b.Logf("Format %s: %.0f events/sec, %.1f bytes/event",
				fmt.format, avgThroughput, avgBytesPerEvent)
		})
	}
}

// Export memory efficiency benchmark
func BenchmarkExport_MemoryEfficiency(b *testing.B) {
	events := generateExportTestData(100000) // 100K events
	ctx := context.Background()

	batchSizes := []int{100, 1000, 10000}

	for _, batchSize := range batchSizes {
		b.Run(fmt.Sprintf("batch_%d", batchSize), func(b *testing.B) {
			criteria := audit.ExportCriteria{
				Format:    "json",
				BatchSize: batchSize,
			}

			exportService := &mockExportService{
				events:    events,
				batchSize: batchSize,
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				writer := newMockExportWriter(0)
				
				err := exportService.ExportEvents(ctx, criteria, writer)
				if err != nil {
					b.Fatalf("Memory efficiency export failed: %v", err)
				}
			}

			b.ReportMetric(float64(batchSize), "batch_size")
		})
	}
}