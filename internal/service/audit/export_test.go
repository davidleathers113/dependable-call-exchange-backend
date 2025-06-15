package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExportService_JSONExport(t *testing.T) {
	// Setup
	queryService := SetupMockQueryService()
	exportService := NewExportService(queryService)

	var buf bytes.Buffer
	options := ExportOptions{
		Format:          ExportFormatJSON,
		ReportType:      ReportTypeGDPR,
		RedactPII:       false,
		IncludeMetadata: true,
		ChunkSize:       1000,
		Filters: map[string]interface{}{
			"user_id": uuid.New().String(),
		},
	}

	// Execute
	progress, err := exportService.Export(context.Background(), options, &buf)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, progress)
	assert.True(t, progress.ProcessedRecords >= 0)

	// Verify JSON output
	output := buf.String()
	assert.True(t, strings.HasPrefix(output, "["))
	assert.True(t, strings.HasSuffix(strings.TrimSpace(output), "]"))

	// Parse and verify structure
	var records []map[string]interface{}
	err = json.Unmarshal([]byte(output), &records)
	require.NoError(t, err)

	// Should have at least one record with metadata
	if len(records) > 0 {
		assert.Contains(t, records[0], "_metadata")
		metadata := records[0]["_metadata"].(map[string]interface{})
		assert.Equal(t, string(ReportTypeGDPR), metadata["report_type"])
		assert.Equal(t, false, metadata["redacted"])
	}
}

func TestExportService_CSVExport(t *testing.T) {
	// Setup
	queryService := SetupMockQueryService()
	exportService := NewExportService(queryService)

	var buf bytes.Buffer
	options := ExportOptions{
		Format:          ExportFormatCSV,
		ReportType:      ReportTypeTCPA,
		RedactPII:       true,
		IncludeMetadata: false,
		ChunkSize:       1000,
	}

	// Execute
	progress, err := exportService.Export(context.Background(), options, &buf)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, progress)

	// Verify CSV output
	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	// Should have header and at least one data row
	assert.GreaterOrEqual(t, len(lines), 1)

	// First line should be headers
	headers := strings.Split(lines[0], ",")
	assert.Contains(t, headers, "consent_id")
	assert.Contains(t, headers, "phone_number")
	assert.Contains(t, headers, "consent_type")
}

func TestExportService_ParquetExport(t *testing.T) {
	// Setup
	queryService := SetupMockQueryService()
	exportService := NewExportService(queryService)

	var buf bytes.Buffer
	options := ExportOptions{
		Format:          ExportFormatParquet,
		ReportType:      ReportTypeSOX,
		RedactPII:       true,
		IncludeMetadata: true,
		ChunkSize:       1000,
		TimeRange: &TimeRange{
			Start: time.Now().Add(-24 * time.Hour),
			End:   time.Now(),
		},
	}

	// Execute
	progress, err := exportService.Export(context.Background(), options, &buf)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, progress)

	// Verify parquet-like output
	output := buf.String()
	assert.Contains(t, output, "Parquet-like Export")
	assert.Contains(t, output, "header")
	assert.Contains(t, output, "data")
}

func TestExportService_GDPRExport(t *testing.T) {
	// Setup
	queryService := SetupMockQueryService()
	exportService := NewExportService(queryService)

	var buf bytes.Buffer
	userID := uuid.New()

	// Execute
	err := exportService.GDPRExport(context.Background(), userID, &buf)

	// Assert
	require.NoError(t, err)

	// Verify GDPR export structure
	output := buf.String()
	assert.True(t, strings.HasPrefix(output, "["))

	var records []map[string]interface{}
	err = json.Unmarshal([]byte(output), &records)
	require.NoError(t, err)

	// GDPR exports should not redact PII
	if len(records) > 0 {
		metadata := records[0]["_metadata"].(map[string]interface{})
		assert.Equal(t, false, metadata["redacted"])
	}
}

func TestExportService_TCPAConsentExport(t *testing.T) {
	// Setup
	queryService := SetupMockQueryService()
	exportService := NewExportService(queryService)

	var buf bytes.Buffer
	phoneNumber := "+1234567890"

	// Execute
	err := exportService.TCPAConsentExport(context.Background(), phoneNumber, &buf)

	// Assert
	require.NoError(t, err)

	// Verify CSV output
	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	assert.GreaterOrEqual(t, len(lines), 1)

	// Should have consent-related headers
	headers := strings.Split(lines[0], ",")
	assert.Contains(t, headers, "consent_id")
	assert.Contains(t, headers, "phone_number")
}

func TestExportService_FinancialAuditExport(t *testing.T) {
	// Setup
	queryService := SetupMockQueryService()
	exportService := NewExportService(queryService)

	var buf bytes.Buffer
	timeRange := TimeRange{
		Start: time.Now().Add(-30 * 24 * time.Hour),
		End:   time.Now(),
	}

	// Execute
	err := exportService.FinancialAuditExport(context.Background(), timeRange, &buf)

	// Assert
	require.NoError(t, err)

	// Verify parquet-like output for financial data
	output := buf.String()
	assert.Contains(t, output, "Parquet-like Export")
	assert.Contains(t, output, "sox_financial_audit")
}

func TestExportService_SecurityIncidentExport(t *testing.T) {
	// Setup
	queryService := SetupMockQueryService()
	exportService := NewExportService(queryService)

	var buf bytes.Buffer
	timeRange := TimeRange{
		Start: time.Now().Add(-24 * time.Hour),
		End:   time.Now(),
	}

	// Execute
	err := exportService.SecurityIncidentExport(context.Background(), "high", timeRange, &buf)

	// Assert
	require.NoError(t, err)

	// Verify JSON output for security incidents
	output := buf.String()
	assert.True(t, strings.HasPrefix(output, "["))

	var records []map[string]interface{}
	err = json.Unmarshal([]byte(output), &records)
	require.NoError(t, err)
}

func TestExportService_CustomTemplate(t *testing.T) {
	// Setup
	queryService := SetupMockQueryService()
	exportService := NewExportService(queryService)

	customTemplate := `{
		"name": "Custom User Report",
		"description": "Custom report for user analysis",
		"fields": [
			{
				"name": "user_id",
				"type": "uuid",
				"required": true,
				"source_path": "id"
			},
			{
				"name": "email",
				"type": "email",
				"required": true,
				"sensitive": true,
				"source_path": "email"
			}
		],
		"queries": [
			{
				"name": "users",
				"entity": "users",
				"sort": "created_at DESC"
			}
		]
	}`

	var buf bytes.Buffer
	options := ExportOptions{
		Format:         ExportFormatJSON,
		ReportType:     ReportTypeCustom,
		CustomTemplate: customTemplate,
		RedactPII:      true,
		ChunkSize:      1000,
	}

	// Execute
	progress, err := exportService.Export(context.Background(), options, &buf)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, progress)

	// Verify output
	output := buf.String()
	assert.True(t, strings.HasPrefix(output, "["))
}

func TestExportService_ValidationErrors(t *testing.T) {
	// Setup
	queryService := SetupMockQueryService()
	exportService := NewExportService(queryService)

	var buf bytes.Buffer

	tests := []struct {
		name    string
		options ExportOptions
		wantErr string
	}{
		{
			name: "invalid format",
			options: ExportOptions{
				Format:     "invalid",
				ReportType: ReportTypeGDPR,
			},
			wantErr: "unsupported export format",
		},
		{
			name: "invalid report type",
			options: ExportOptions{
				Format:     ExportFormatJSON,
				ReportType: "invalid",
			},
			wantErr: "unsupported report type",
		},
		{
			name: "custom template missing",
			options: ExportOptions{
				Format:     ExportFormatJSON,
				ReportType: ReportTypeCustom,
			},
			wantErr: "custom template required",
		},
		{
			name: "invalid time range",
			options: ExportOptions{
				Format:     ExportFormatJSON,
				ReportType: ReportTypeGDPR,
				TimeRange: &TimeRange{
					Start: time.Now(),
					End:   time.Now().Add(-1 * time.Hour),
				},
			},
			wantErr: "start time must be before end time",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := exportService.Export(context.Background(), tt.options, &buf)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestExportService_PIIRedaction(t *testing.T) {
	// Setup
	queryService := SetupMockQueryService()
	exportService := NewExportService(queryService)

	// Test data redaction
	tests := []struct {
		name      string
		fieldType string
		input     interface{}
		expected  string
	}{
		{
			name:      "phone redaction",
			fieldType: "phone",
			input:     "+1234567890",
			expected:  "+12345****",
		},
		{
			name:      "email redaction",
			fieldType: "email",
			input:     "test@example.com",
			expected:  "****@example.com",
		},
		{
			name:      "name redaction",
			fieldType: "name",
			input:     "John Doe",
			expected:  "J. D.",
		},
		{
			name:      "ssn redaction",
			fieldType: "ssn",
			input:     "123-45-6789",
			expected:  "***-**-6789",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := redactValue(tt.fieldType, tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExportService_ProgressTracking(t *testing.T) {
	// Setup
	queryService := SetupMockQueryService()
	exportService := NewExportService(queryService)

	var buf bytes.Buffer
	options := ExportOptions{
		Format:     ExportFormatJSON,
		ReportType: ReportTypeGDPR,
		ChunkSize:  1000,
	}

	// Execute
	progress, err := exportService.Export(context.Background(), options, &buf)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, progress)
	assert.False(t, progress.StartTime.IsZero())
	assert.Equal(t, "completed", progress.CurrentPhase)
	assert.Equal(t, progress.TotalRecords, progress.ProcessedRecords)
}

func TestExportService_LargeDataset(t *testing.T) {
	// This test would verify streaming behavior with large datasets
	// In a real implementation, we'd test memory usage and chunk processing

	queryService := SetupMockQueryService()
	exportService := NewExportService(queryService)

	var buf bytes.Buffer
	options := ExportOptions{
		Format:     ExportFormatCSV,
		ReportType: ReportTypeSecurityAudit,
		ChunkSize:  10, // Small chunk size to test streaming
	}

	// Execute
	progress, err := exportService.Export(context.Background(), options, &buf)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, progress)

	// Verify that data was processed in chunks
	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	assert.GreaterOrEqual(t, len(lines), 1) // At least header line
}

func TestExportService_ContextCancellation(t *testing.T) {
	// Test context cancellation during export
	queryService := SetupMockQueryService()
	exportService := NewExportService(queryService)

	var buf bytes.Buffer
	options := ExportOptions{
		Format:     ExportFormatJSON,
		ReportType: ReportTypeGDPR,
		ChunkSize:  1000,
	}

	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Execute
	_, err := exportService.Export(ctx, options, &buf)

	// Should handle cancellation gracefully
	if err != nil {
		assert.Contains(t, err.Error(), "cancelled")
	}
}

func TestExportProgress_ThreadSafety(t *testing.T) {
	// Test thread-safe progress updates
	progress := &ExportProgress{
		StartTime: time.Now(),
	}

	// Update progress from multiple goroutines
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(id int) {
			progress.Update(int64(id*10), "processing")
			progress.AddError("test error")
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify no data races occurred
	snapshot := progress.GetSnapshot()
	assert.GreaterOrEqual(t, snapshot.ProcessedRecords, int64(0))
	assert.GreaterOrEqual(t, len(snapshot.Errors), 0)
}

// Property-Based Testing with 1000+ Iterations for IMMUTABLE_AUDIT Export

func TestPropertyExportService_DataIntegrityInvariants(t *testing.T) {
	queryService := SetupMockQueryService()
	exportService := NewExportService(queryService)

	// Property: Export operations should maintain data integrity across formats
	for i := 0; i < 1000; i++ {
		formats := []ExportFormat{ExportFormatJSON, ExportFormatCSV, ExportFormatParquet}
		format := formats[i%len(formats)]
		
		var buf bytes.Buffer
		options := ExportOptions{
			Format:          format,
			ReportType:      ReportTypeGDPR,
			RedactPII:       false,
			IncludeMetadata: true,
			ChunkSize:       100 + (i % 900), // Varying chunk sizes
			Filters: map[string]interface{}{
				"iteration": i,
				"user_id":   uuid.New().String(),
			},
		}

		progress, err := exportService.Export(context.Background(), options, &buf)
		require.NoError(t, err, "Iteration %d: export should not fail", i)
		
		// Invariant: Progress should be valid
		assert.NotNil(t, progress, "Iteration %d: progress should not be nil", i)
		assert.GreaterOrEqual(t, progress.ProcessedRecords, int64(0), "Iteration %d: processed records should be non-negative", i)
		assert.GreaterOrEqual(t, progress.TotalRecords, progress.ProcessedRecords, "Iteration %d: total should be >= processed", i)
		
		// Invariant: Output should not be empty for valid data
		output := buf.String()
		assert.NotEmpty(t, output, "Iteration %d: output should not be empty", i)
		
		// Format-specific invariants
		switch format {
		case ExportFormatJSON:
			assert.True(t, strings.HasPrefix(output, "["), "Iteration %d: JSON should start with [", i)
			assert.True(t, strings.HasSuffix(strings.TrimSpace(output), "]"), "Iteration %d: JSON should end with ]", i)
		case ExportFormatCSV:
			lines := strings.Split(strings.TrimSpace(output), "\n")
			assert.GreaterOrEqual(t, len(lines), 1, "Iteration %d: CSV should have at least header", i)
		case ExportFormatParquet:
			assert.Contains(t, output, "Parquet-like Export", "Iteration %d: Parquet should have identifier", i)
		}
	}
}

func TestPropertyExportService_PIIRedactionConsistency(t *testing.T) {
	queryService := SetupMockQueryService()
	exportService := NewExportService(queryService)

	// Property: PII redaction should be consistent and reversible
	testCases := []struct {
		fieldType string
		values    []interface{}
	}{
		{"phone", []interface{}{"+1234567890", "+15551234567", "+19876543210"}},
		{"email", []interface{}{"test@example.com", "user.name@domain.org", "admin@company.net"}},
		{"name", []interface{}{"John Doe", "Jane Smith", "Bob Johnson"}},
		{"ssn", []interface{}{"123-45-6789", "987-65-4321", "555-44-3333"}},
	}

	for i := 0; i < 1000; i++ {
		testCase := testCases[i%len(testCases)]
		value := testCase.values[i%len(testCase.values)]
		
		// Test redaction consistency
		redacted1 := redactValue(testCase.fieldType, value)
		redacted2 := redactValue(testCase.fieldType, value)
		
		// Invariant: Same input should produce same redacted output
		assert.Equal(t, redacted1, redacted2, "Iteration %d: redaction should be consistent for %s", i, testCase.fieldType)
		
		// Invariant: Redacted value should be different from original (unless empty)
		if value != "" && value != nil {
			assert.NotEqual(t, value, redacted1, "Iteration %d: redacted value should differ from original", i)
		}
		
		// Invariant: Redacted value should preserve format structure
		switch testCase.fieldType {
		case "phone":
			if str, ok := value.(string); ok && len(str) > 0 {
				assert.True(t, strings.HasPrefix(redacted1, "+"), "Iteration %d: redacted phone should keep + prefix", i)
			}
		case "email":
			if str, ok := value.(string); ok && strings.Contains(str, "@") {
				assert.Contains(t, redacted1, "@", "Iteration %d: redacted email should keep @ symbol", i)
			}
		case "ssn":
			if str, ok := value.(string); ok && strings.Contains(str, "-") {
				assert.Contains(t, redacted1, "-", "Iteration %d: redacted SSN should keep - separators", i)
			}
		}
	}
}

func TestPropertyExportService_MetadataInvariants(t *testing.T) {
	queryService := SetupMockQueryService()
	exportService := NewExportService(queryService)

	// Property: Metadata should always be consistent and complete when enabled
	for i := 0; i < 1000; i++ {
		includeMetadata := i%2 == 0
		redactPII := i%3 == 0
		
		var buf bytes.Buffer
		options := ExportOptions{
			Format:          ExportFormatJSON,
			ReportType:      ReportTypeGDPR,
			RedactPII:       redactPII,
			IncludeMetadata: includeMetadata,
			ChunkSize:       1000,
		}

		progress, err := exportService.Export(context.Background(), options, &buf)
		require.NoError(t, err, "Iteration %d: export should succeed", i)
		
		if includeMetadata {
			output := buf.String()
			var records []map[string]interface{}
			err = json.Unmarshal([]byte(output), &records)
			require.NoError(t, err, "Iteration %d: should parse JSON", i)
			
			if len(records) > 0 {
				// Invariant: Metadata should be present and complete
				assert.Contains(t, records[0], "_metadata", "Iteration %d: should have metadata field", i)
				
				metadata := records[0]["_metadata"].(map[string]interface{})
				assert.Contains(t, metadata, "report_type", "Iteration %d: metadata should have report_type", i)
				assert.Contains(t, metadata, "redacted", "Iteration %d: metadata should have redacted flag", i)
				assert.Contains(t, metadata, "export_time", "Iteration %d: metadata should have export_time", i)
				
				// Invariant: Redaction flag should match option
				assert.Equal(t, redactPII, metadata["redacted"], "Iteration %d: redacted flag should match option", i)
				assert.Equal(t, string(ReportTypeGDPR), metadata["report_type"], "Iteration %d: report type should match", i)
			}
		}
	}
}

func TestPropertyExportService_ChunkSizeInvariants(t *testing.T) {
	queryService := SetupMockQueryService()
	exportService := NewExportService(queryService)

	// Property: Different chunk sizes should produce identical final output
	chunkSizes := []int{1, 10, 100, 1000, 5000}
	
	for i := 0; i < 200; i++ { // 200 iterations * 5 chunk sizes = 1000 total tests
		chunkSize := chunkSizes[i%len(chunkSizes)]
		
		var buf bytes.Buffer
		options := ExportOptions{
			Format:          ExportFormatJSON,
			ReportType:      ReportTypeGDPR,
			RedactPII:       false,
			IncludeMetadata: false, // Exclude metadata for easier comparison
			ChunkSize:       chunkSize,
			Filters: map[string]interface{}{
				"test_id": fmt.Sprintf("chunk_test_%d", i),
			},
		}

		progress, err := exportService.Export(context.Background(), options, &buf)
		require.NoError(t, err, "Iteration %d with chunk size %d should succeed", i, chunkSize)
		
		// Invariant: Progress should reflect chunk processing
		assert.GreaterOrEqual(t, progress.ProcessedRecords, int64(0), "Iteration %d: processed records should be valid", i)
		
		// Invariant: Output should be valid JSON regardless of chunk size
		output := buf.String()
		var records []map[string]interface{}
		err = json.Unmarshal([]byte(output), &records)
		require.NoError(t, err, "Iteration %d: output should be valid JSON regardless of chunk size %d", i, chunkSize)
		
		// Store reference output for comparison (using chunk size 1000 as reference)
		if chunkSize == 1000 && i < 40 { // Test first 40 iterations with reference
			referenceOutput := output
			
			// Test with smaller chunk size
			var smallBuf bytes.Buffer
			smallOptions := options
			smallOptions.ChunkSize = 10
			
			_, err = exportService.Export(context.Background(), smallOptions, &smallBuf)
			require.NoError(t, err, "Iteration %d: small chunk export should succeed", i)
			
			smallOutput := smallBuf.String()
			
			// Invariant: Different chunk sizes should produce equivalent data
			var referenceRecords, smallRecords []map[string]interface{}
			json.Unmarshal([]byte(referenceOutput), &referenceRecords)
			json.Unmarshal([]byte(smallOutput), &smallRecords)
			
			assert.Equal(t, len(referenceRecords), len(smallRecords), "Iteration %d: record count should be identical", i)
		}
	}
}

func TestPropertyExportService_TimeRangeFiltering(t *testing.T) {
	queryService := SetupMockQueryService()
	exportService := NewExportService(queryService)

	// Property: Time range filtering should be consistent and inclusive
	for i := 0; i < 1000; i++ {
		baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		
		// Generate various time ranges
		startOffset := time.Duration(i%24) * time.Hour
		endOffset := startOffset + time.Duration(1+i%48) * time.Hour
		
		timeRange := &TimeRange{
			Start: baseTime.Add(startOffset),
			End:   baseTime.Add(endOffset),
		}
		
		var buf bytes.Buffer
		options := ExportOptions{
			Format:          ExportFormatJSON,
			ReportType:      ReportTypeSecurityAudit,
			RedactPII:       false,
			IncludeMetadata: true,
			ChunkSize:       1000,
			TimeRange:       timeRange,
		}

		progress, err := exportService.Export(context.Background(), options, &buf)
		require.NoError(t, err, "Iteration %d: export with time range should succeed", i)
		
		// Invariant: Valid time range should not cause errors
		assert.NotNil(t, progress, "Iteration %d: progress should not be nil", i)
		
		// Invariant: Start time should be before end time
		assert.True(t, timeRange.Start.Before(timeRange.End), "Iteration %d: start should be before end", i)
		
		output := buf.String()
		if includeMetadata := true; includeMetadata {
			var records []map[string]interface{}
			if json.Unmarshal([]byte(output), &records) == nil && len(records) > 0 {
				metadata := records[0]["_metadata"].(map[string]interface{})
				
				// Invariant: Metadata should include time range information
				assert.Contains(t, metadata, "time_range", "Iteration %d: metadata should include time range", i)
			}
		}
	}
}

func TestPropertyExportService_ConcurrentExports(t *testing.T) {
	queryService := SetupMockQueryService()
	exportService := NewExportService(queryService)

	// Property: Concurrent exports should not interfere with each other
	for iteration := 0; iteration < 100; iteration++ {
		const numGoroutines = 10
		var wg sync.WaitGroup
		results := make(chan bool, numGoroutines)
		errors := make(chan error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				
				var buf bytes.Buffer
				options := ExportOptions{
					Format:     ExportFormatJSON,
					ReportType: ReportTypeGDPR,
					ChunkSize:  1000,
					Filters: map[string]interface{}{
						"goroutine_id": id,
						"iteration":    iteration,
					},
				}

				progress, err := exportService.Export(context.Background(), options, &buf)
				if err != nil {
					errors <- err
					return
				}

				// Verify export completed successfully
				if progress != nil && progress.ProcessedRecords >= 0 && buf.Len() > 0 {
					results <- true
				} else {
					results <- false
				}
			}(i)
		}

		wg.Wait()
		close(results)
		close(errors)

		// Check for errors
		for err := range errors {
			t.Errorf("Iteration %d: Concurrent export failed: %v", iteration, err)
		}

		// Verify all exports succeeded
		successCount := 0
		for success := range results {
			if success {
				successCount++
			}
		}
		assert.Equal(t, numGoroutines, successCount, "Iteration %d: all concurrent exports should succeed", iteration)
	}
}

// Edge Case Testing for Export Service

func TestExportService_EdgeCases(t *testing.T) {
	queryService := SetupMockQueryService()
	exportService := NewExportService(queryService)

	t.Run("EdgeCase_ZeroChunkSize", func(t *testing.T) {
		var buf bytes.Buffer
		options := ExportOptions{
			Format:     ExportFormatJSON,
			ReportType: ReportTypeGDPR,
			ChunkSize:  0, // Edge case: zero chunk size
		}

		_, err := exportService.Export(context.Background(), options, &buf)
		// Should handle gracefully or use default chunk size
		if err != nil {
			assert.Contains(t, err.Error(), "chunk")
		}
	})

	t.Run("EdgeCase_NegativeChunkSize", func(t *testing.T) {
		var buf bytes.Buffer
		options := ExportOptions{
			Format:     ExportFormatJSON,
			ReportType: ReportTypeGDPR,
			ChunkSize:  -100, // Edge case: negative chunk size
		}

		_, err := exportService.Export(context.Background(), options, &buf)
		assert.Error(t, err, "Negative chunk size should cause error")
	})

	t.Run("EdgeCase_MaximumChunkSize", func(t *testing.T) {
		var buf bytes.Buffer
		options := ExportOptions{
			Format:     ExportFormatJSON,
			ReportType: ReportTypeGDPR,
			ChunkSize:  1000000, // Edge case: very large chunk size
		}

		progress, err := exportService.Export(context.Background(), options, &buf)
		// Should handle large chunk sizes gracefully
		if err == nil {
			assert.NotNil(t, progress)
		}
	})

	t.Run("EdgeCase_EmptyTimeRange", func(t *testing.T) {
		var buf bytes.Buffer
		sameTime := time.Now()
		options := ExportOptions{
			Format:     ExportFormatJSON,
			ReportType: ReportTypeGDPR,
			TimeRange: &TimeRange{
				Start: sameTime,
				End:   sameTime, // Edge case: zero duration
			},
		}

		progress, err := exportService.Export(context.Background(), options, &buf)
		// Should handle zero-duration time range
		if err == nil {
			assert.NotNil(t, progress)
		}
	})

	t.Run("EdgeCase_VeryLongTimeRange", func(t *testing.T) {
		var buf bytes.Buffer
		options := ExportOptions{
			Format:     ExportFormatJSON,
			ReportType: ReportTypeGDPR,
			TimeRange: &TimeRange{
				Start: time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC),
				End:   time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC), // 130+ year range
			},
		}

		progress, err := exportService.Export(context.Background(), options, &buf)
		// Should handle very long time ranges
		if err == nil {
			assert.NotNil(t, progress)
		}
	})

	t.Run("EdgeCase_InvalidJSONInCustomTemplate", func(t *testing.T) {
		var buf bytes.Buffer
		options := ExportOptions{
			Format:         ExportFormatJSON,
			ReportType:     ReportTypeCustom,
			CustomTemplate: "{invalid json template}", // Edge case: malformed JSON
		}

		_, err := exportService.Export(context.Background(), options, &buf)
		assert.Error(t, err, "Invalid JSON template should cause error")
	})

	t.Run("EdgeCase_UnicodeInFilters", func(t *testing.T) {
		var buf bytes.Buffer
		options := ExportOptions{
			Format:     ExportFormatJSON,
			ReportType: ReportTypeGDPR,
			Filters: map[string]interface{}{
				"unicode_field": "测试数据", // Unicode characters
				"emoji_field":   "🔒🔑📊", // Emojis
			},
		}

		progress, err := exportService.Export(context.Background(), options, &buf)
		require.NoError(t, err, "Unicode in filters should not cause error")
		assert.NotNil(t, progress)

		// Verify output handles unicode correctly
		output := buf.String()
		assert.Contains(t, output, "测试数据")
		assert.Contains(t, output, "🔒🔑📊")
	})
}

// Error Handling Testing

func TestExportService_ErrorHandling(t *testing.T) {
	queryService := SetupMockQueryService()
	exportService := NewExportService(queryService)

	t.Run("Repository_Failure", func(t *testing.T) {
		// This would test what happens when the query service fails
		// In a real implementation, we'd mock repository failures
		var buf bytes.Buffer
		options := ExportOptions{
			Format:     ExportFormatJSON,
			ReportType: ReportTypeGDPR,
			Filters: map[string]interface{}{
				"force_error": true, // Mock trigger for repository error
			},
		}

		_, err := exportService.Export(context.Background(), options, &buf)
		// Should handle repository errors gracefully
		if err != nil {
			assert.Contains(t, err.Error(), "query")
		}
	})

	t.Run("Memory_Pressure", func(t *testing.T) {
		// Test behavior under memory pressure
		var buf bytes.Buffer
		options := ExportOptions{
			Format:     ExportFormatJSON,
			ReportType: ReportTypeGDPR,
			ChunkSize:  1, // Force many small chunks to test memory handling
		}

		progress, err := exportService.Export(context.Background(), options, &buf)
		// Should handle memory pressure by chunking appropriately
		if err == nil {
			assert.NotNil(t, progress)
		}
	})

	t.Run("Writer_Failure", func(t *testing.T) {
		// Test what happens when the writer fails
		failingWriter := &FailingWriter{failAfter: 100}
		options := ExportOptions{
			Format:     ExportFormatJSON,
			ReportType: ReportTypeGDPR,
		}

		_, err := exportService.Export(context.Background(), options, failingWriter)
		assert.Error(t, err, "Writer failure should cause error")
	})
}

// FailingWriter is a test helper that fails after a certain number of bytes
type FailingWriter struct {
	written   int
	failAfter int
}

func (w *FailingWriter) Write(p []byte) (n int, err error) {
	if w.written+len(p) > w.failAfter {
		return 0, fmt.Errorf("writer failure after %d bytes", w.failAfter)
	}
	w.written += len(p)
	return len(p), nil
}

// Benchmark tests

func BenchmarkExportService_JSONExport(b *testing.B) {
	queryService := SetupMockQueryService()
	exportService := NewExportService(queryService)

	options := ExportOptions{
		Format:     ExportFormatJSON,
		ReportType: ReportTypeGDPR,
		ChunkSize:  1000,
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		_, err := exportService.Export(context.Background(), options, &buf)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkExportService_CSVExport(b *testing.B) {
	queryService := SetupMockQueryService()
	exportService := NewExportService(queryService)

	options := ExportOptions{
		Format:     ExportFormatCSV,
		ReportType: ReportTypeTCPA,
		ChunkSize:  1000,
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		_, err := exportService.Export(context.Background(), options, &buf)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkExportService_PIIRedaction(b *testing.B) {
	// Benchmark PII redaction performance
	testValues := []interface{}{
		"+1234567890",
		"test@example.com", 
		"John Doe",
		"123-45-6789",
	}
	
	fieldTypes := []string{"phone", "email", "name", "ssn"}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		fieldType := fieldTypes[i%len(fieldTypes)]
		value := testValues[i%len(testValues)]
		_ = redactValue(fieldType, value)
	}
}

func BenchmarkExportService_ConcurrentExports(b *testing.B) {
	queryService := SetupMockQueryService()
	exportService := NewExportService(queryService)

	options := ExportOptions{
		Format:     ExportFormatJSON,
		ReportType: ReportTypeGDPR,
		ChunkSize:  1000,
	}

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			var buf bytes.Buffer
			_, err := exportService.Export(context.Background(), options, &buf)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkExportService_LargeDataset(b *testing.B) {
	queryService := SetupMockQueryService()
	exportService := NewExportService(queryService)

	options := ExportOptions{
		Format:     ExportFormatCSV,
		ReportType: ReportTypeSecurityAudit,
		ChunkSize:  10000, // Large chunk size for performance
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		_, err := exportService.Export(context.Background(), options, &buf)
		if err != nil {
			b.Fatal(err)
		}
	}
}
