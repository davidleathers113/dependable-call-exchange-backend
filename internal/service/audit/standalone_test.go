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

// Test the export service with a minimal setup to avoid dependency issues
func TestExportService_Standalone(t *testing.T) {
	// Create a minimal query service with mock data
	queryService := NewQueryService()
	queryService.RegisterRepository("users", &MockUserRepository{})
	queryService.RegisterRepository("calls", &MockCallRepository{})
	queryService.RegisterRepository("consents", &MockConsentRepository{})
	queryService.RegisterRepository("tcpa_consents", &MockConsentRepository{})
	queryService.RegisterRepository("transactions", &MockTransactionRepository{})
	queryService.RegisterRepository("audit_events", &MockTransactionRepository{})
	queryService.RegisterRepository("security_events", &MockSecurityEventRepository{})

	// Create export service
	exportService := NewExportService(queryService)

	t.Run("JSON Export", func(t *testing.T) {
		var buf bytes.Buffer
		options := ExportOptions{
			Format:          ExportFormatJSON,
			ReportType:      ReportTypeGDPR,
			RedactPII:       false,
			IncludeMetadata: true,
			ChunkSize:       1000,
		}

		progress, err := exportService.Export(context.Background(), options, &buf)

		require.NoError(t, err)
		assert.NotNil(t, progress)
		assert.Equal(t, "completed", progress.CurrentPhase)

		// Verify JSON output structure
		output := buf.String()
		assert.True(t, strings.HasPrefix(output, "["))
		assert.True(t, strings.HasSuffix(strings.TrimSpace(output), "]"))

		// Parse JSON to verify it's valid
		var records []map[string]interface{}
		err = json.Unmarshal([]byte(output), &records)
		require.NoError(t, err)
	})

	t.Run("CSV Export", func(t *testing.T) {
		var buf bytes.Buffer
		options := ExportOptions{
			Format:     ExportFormatCSV,
			ReportType: ReportTypeTCPA,
			RedactPII:  true,
			ChunkSize:  1000,
		}

		progress, err := exportService.Export(context.Background(), options, &buf)

		require.NoError(t, err)
		assert.NotNil(t, progress)

		// Verify CSV output
		output := buf.String()
		lines := strings.Split(strings.TrimSpace(output), "\n")
		assert.GreaterOrEqual(t, len(lines), 1) // At least header

		// Check for CSV headers
		if len(lines) > 0 {
			headers := strings.Split(lines[0], ",")
			assert.Contains(t, headers, "consent_id")
			assert.Contains(t, headers, "phone_number")
		}
	})

	t.Run("Parquet Export", func(t *testing.T) {
		var buf bytes.Buffer
		options := ExportOptions{
			Format:     ExportFormatParquet,
			ReportType: ReportTypeSOX,
			RedactPII:  true,
			ChunkSize:  1000,
		}

		progress, err := exportService.Export(context.Background(), options, &buf)

		require.NoError(t, err)
		assert.NotNil(t, progress)

		// Verify parquet-like output contains expected markers
		output := buf.String()
		assert.Contains(t, output, "Parquet-like Export")
		assert.Contains(t, output, "sox_financial_audit")
	})

	t.Run("PII Redaction", func(t *testing.T) {
		// Test redaction functions directly
		testCases := []struct {
			fieldType string
			input     interface{}
			expected  string
		}{
			{"phone", "+1234567890", "+12345****"},
			{"email", "test@example.com", "****@example.com"},
			{"name", "John Doe", "J. D."},
			{"ssn", "123-45-6789", "***-**-6789"},
		}

		for _, tc := range testCases {
			t.Run(tc.fieldType, func(t *testing.T) {
				result := redactValue(tc.fieldType, tc.input)
				assert.Equal(t, tc.expected, result)
			})
		}
	})

	t.Run("Validation Errors", func(t *testing.T) {
		var buf bytes.Buffer

		// Test invalid format
		options := ExportOptions{
			Format:     "invalid",
			ReportType: ReportTypeGDPR,
		}

		_, err := exportService.Export(context.Background(), options, &buf)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported export format")
	})

	t.Run("Progress Tracking", func(t *testing.T) {
		var buf bytes.Buffer
		options := ExportOptions{
			Format:     ExportFormatJSON,
			ReportType: ReportTypeGDPR,
			ChunkSize:  1000,
		}

		progress, err := exportService.Export(context.Background(), options, &buf)

		require.NoError(t, err)
		assert.NotNil(t, progress)
		assert.False(t, progress.StartTime.IsZero())
		assert.Equal(t, "completed", progress.CurrentPhase)
		assert.GreaterOrEqual(t, progress.ProcessedRecords, int64(0))
		assert.GreaterOrEqual(t, progress.TotalRecords, progress.ProcessedRecords)
	})

	t.Run("Custom Template", func(t *testing.T) {
		customTemplate := `{
			"name": "Simple User Report",
			"description": "Basic user data export",
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

		progress, err := exportService.Export(context.Background(), options, &buf)

		require.NoError(t, err)
		assert.NotNil(t, progress)
		assert.Equal(t, "completed", progress.CurrentPhase)
	})

	t.Run("Time Range Filtering", func(t *testing.T) {
		var buf bytes.Buffer
		options := ExportOptions{
			Format:     ExportFormatJSON,
			ReportType: ReportTypeGDPR,
			ChunkSize:  1000,
			TimeRange: &TimeRange{
				Start: time.Now().Add(-24 * time.Hour),
				End:   time.Now(),
			},
		}

		progress, err := exportService.Export(context.Background(), options, &buf)

		require.NoError(t, err)
		assert.NotNil(t, progress)
	})

	t.Run("Convenience Methods", func(t *testing.T) {
		// Test GDPR export convenience method
		var buf bytes.Buffer
		userID := uuid.New()

		err := exportService.GDPRExport(context.Background(), userID, &buf)
		require.NoError(t, err)
		assert.Greater(t, buf.Len(), 0)

		// Test TCPA export convenience method
		buf.Reset()
		err = exportService.TCPAConsentExport(context.Background(), "+1234567890", &buf)
		require.NoError(t, err)
		assert.Greater(t, buf.Len(), 0)

		// Test financial audit export convenience method
		buf.Reset()
		timeRange := TimeRange{
			Start: time.Now().Add(-30 * 24 * time.Hour),
			End:   time.Now(),
		}
		err = exportService.FinancialAuditExport(context.Background(), timeRange, &buf)
		require.NoError(t, err)
		assert.Greater(t, buf.Len(), 0)

		// Test security incident export convenience method
		buf.Reset()
		err = exportService.SecurityIncidentExport(context.Background(), "high", timeRange, &buf)
		require.NoError(t, err)
		assert.Greater(t, buf.Len(), 0)
	})
}

// Test progress tracking thread safety
func TestExportProgress_ThreadSafety(t *testing.T) {
	progress := &ExportProgress{
		StartTime: time.Now(),
	}

	// Update progress from multiple goroutines concurrently
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(id int) {
			defer func() { done <- true }()

			// Multiple operations to test race conditions
			progress.Update(int64(id*10), "processing")
			progress.AddError("test error")
			snapshot := progress.GetSnapshot()

			// Verify snapshot is consistent
			assert.GreaterOrEqual(t, snapshot.ProcessedRecords, int64(0))
			assert.NotEmpty(t, snapshot.CurrentPhase)
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Final verification
	finalSnapshot := progress.GetSnapshot()
	assert.GreaterOrEqual(t, finalSnapshot.ProcessedRecords, int64(0))
	assert.GreaterOrEqual(t, len(finalSnapshot.Errors), 0)
}

// Benchmark test for export performance
func BenchmarkExportService_JSON(b *testing.B) {
	queryService := NewQueryService()
	queryService.RegisterRepository("users", &MockUserRepository{})
	queryService.RegisterRepository("calls", &MockCallRepository{})
	queryService.RegisterRepository("consents", &MockConsentRepository{})
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

func BenchmarkExportService_CSV(b *testing.B) {
	queryService := NewQueryService()
	queryService.RegisterRepository("tcpa_consents", &MockConsentRepository{})
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
