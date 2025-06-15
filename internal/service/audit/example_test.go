package audit

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
)

// ExampleExportService_GDPRReport demonstrates generating a GDPR data subject report
func ExampleExportService_GDPRReport() {
	// Create query service with mock data
	queryService := SetupMockQueryService()

	// Create export service
	exportService := NewExportService(queryService)

	// Generate GDPR export for a user
	var buf bytes.Buffer
	userID := uuid.New()

	err := exportService.GDPRExport(context.Background(), userID, &buf)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("GDPR export generated: %d bytes\n", buf.Len())
	// Output: GDPR export generated: 1000+ bytes (approximate)
}

// ExampleExportService_TCPAConsentTrail demonstrates generating a TCPA consent trail
func ExampleExportService_TCPAConsentTrail() {
	queryService := SetupMockQueryService()
	exportService := NewExportService(queryService)

	var buf bytes.Buffer
	phoneNumber := "+1234567890"

	err := exportService.TCPAConsentExport(context.Background(), phoneNumber, &buf)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("TCPA consent trail generated for %s\n", phoneNumber)
	// Output: TCPA consent trail generated for +1234567890
}

// ExampleExportService_FinancialAudit demonstrates SOX financial audit export
func ExampleExportService_FinancialAudit() {
	queryService := SetupMockQueryService()
	exportService := NewExportService(queryService)

	var buf bytes.Buffer
	timeRange := TimeRange{
		Start: time.Now().Add(-30 * 24 * time.Hour),
		End:   time.Now(),
	}

	err := exportService.FinancialAuditExport(context.Background(), timeRange, &buf)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Financial audit export completed for last 30 days\n")
	// Output: Financial audit export completed for last 30 days
}

// ExampleExportService_SecurityIncidents demonstrates security incident reporting
func ExampleExportService_SecurityIncidents() {
	queryService := SetupMockQueryService()
	exportService := NewExportService(queryService)

	var buf bytes.Buffer
	timeRange := TimeRange{
		Start: time.Now().Add(-7 * 24 * time.Hour),
		End:   time.Now(),
	}

	err := exportService.SecurityIncidentExport(context.Background(), "high", timeRange, &buf)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("High-severity security incidents exported\n")
	// Output: High-severity security incidents exported
}

// ExampleExportService_CustomReport demonstrates custom report generation
func ExampleExportService_CustomReport() {
	queryService := SetupMockQueryService()
	exportService := NewExportService(queryService)

	// Define custom template
	customTemplate := `{
		"name": "User Activity Report",
		"description": "Summary of user activity for compliance review",
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
			},
			{
				"name": "last_login",
				"type": "timestamp",
				"required": false,
				"source_path": "last_login_at"
			}
		],
		"queries": [
			{
				"name": "active_users",
				"entity": "users",
				"filter": "status = 'active'",
				"sort": "created_at DESC"
			}
		]
	}`

	var buf bytes.Buffer
	options := ExportOptions{
		Format:          ExportFormatJSON,
		ReportType:      ReportTypeCustom,
		CustomTemplate:  customTemplate,
		RedactPII:       true,
		IncludeMetadata: true,
		ChunkSize:       1000,
	}

	progress, err := exportService.Export(context.Background(), options, &buf)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Custom report generated: %d records processed\n", progress.ProcessedRecords)
	// Output: Custom report generated: 2+ records processed (approximate)
}

// ExampleExportService_WithProgressTracking demonstrates progress tracking
func ExampleExportService_WithProgressTracking() {
	queryService := SetupMockQueryService()
	exportService := NewExportService(queryService)

	var buf bytes.Buffer
	options := ExportOptions{
		Format:          ExportFormatCSV,
		ReportType:      ReportTypeTCPA,
		RedactPII:       false,
		IncludeMetadata: true,
		ChunkSize:       100, // Small chunks to see progress
	}

	// Start export
	progress, err := exportService.Export(context.Background(), options, &buf)
	if err != nil {
		log.Fatal(err)
	}

	// Show final progress
	fmt.Printf("Export completed: %d/%d records in %v\n",
		progress.ProcessedRecords,
		progress.TotalRecords,
		time.Since(progress.StartTime))

	// Output: Export completed: 2/2 records in 1ms+ (approximate)
}

// ExampleExportService_MultipleFormats demonstrates different export formats
func ExampleExportService_MultipleFormats() {
	queryService := SetupMockQueryService()
	exportService := NewExportService(queryService)

	baseOptions := ExportOptions{
		ReportType:      ReportTypeSecurityAudit,
		RedactPII:       true,
		IncludeMetadata: false,
		ChunkSize:       1000,
		TimeRange: &TimeRange{
			Start: time.Now().Add(-24 * time.Hour),
			End:   time.Now(),
		},
	}

	formats := []ExportFormat{
		ExportFormatJSON,
		ExportFormatCSV,
		ExportFormatParquet,
	}

	for _, format := range formats {
		var buf bytes.Buffer
		options := baseOptions
		options.Format = format

		_, err := exportService.Export(context.Background(), options, &buf)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Printf("Generated %s export: %d bytes\n", format, buf.Len())
	}

	// Output:
	// Generated json export: 500+ bytes
	// Generated csv export: 300+ bytes
	// Generated parquet export: 800+ bytes
}

// ExampleExportService_DataSanitization demonstrates PII redaction
func ExampleExportService_DataSanitization() {
	queryService := SetupMockQueryService()
	exportService := NewExportService(queryService)

	// Export with PII redaction enabled
	var redactedBuf bytes.Buffer
	redactedOptions := ExportOptions{
		Format:     ExportFormatJSON,
		ReportType: ReportTypeGDPR,
		RedactPII:  true, // Enable PII redaction
		ChunkSize:  1000,
	}

	_, err := exportService.Export(context.Background(), redactedOptions, &redactedBuf)
	if err != nil {
		log.Fatal(err)
	}

	// Export without PII redaction
	var fullBuf bytes.Buffer
	fullOptions := ExportOptions{
		Format:     ExportFormatJSON,
		ReportType: ReportTypeGDPR,
		RedactPII:  false, // Disable PII redaction
		ChunkSize:  1000,
	}

	_, err = exportService.Export(context.Background(), fullOptions, &fullBuf)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Redacted export: %d bytes\n", redactedBuf.Len())
	fmt.Printf("Full export: %d bytes\n", fullBuf.Len())

	// Output:
	// Redacted export: 800+ bytes
	// Full export: 1000+ bytes
}

// ExampleExportService_StreamingLargeDataset demonstrates handling large datasets
func ExampleExportService_StreamingLargeDataset() {
	queryService := SetupMockQueryService()
	exportService := NewExportService(queryService)

	var buf bytes.Buffer
	options := ExportOptions{
		Format:     ExportFormatCSV,
		ReportType: ReportTypeSOX,
		RedactPII:  true,
		ChunkSize:  50, // Small chunks for demonstration
		TimeRange: &TimeRange{
			Start: time.Now().Add(-365 * 24 * time.Hour), // Full year
			End:   time.Now(),
		},
	}

	// This would stream data in chunks, processing 50 records at a time
	// to avoid loading large datasets into memory
	progress, err := exportService.Export(context.Background(), options, &buf)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Streamed %d records in chunks of %d\n",
		progress.ProcessedRecords, options.ChunkSize)

	// Output: Streamed 2+ records in chunks of 50
}

// ExampleExportService_ComplianceReports demonstrates different compliance report types
func ExampleExportService_ComplianceReports() {
	queryService := SetupMockQueryService()
	exportService := NewExportService(queryService)

	reportTypes := []struct {
		name       string
		reportType ReportType
		format     ExportFormat
	}{
		{"GDPR Data Subject", ReportTypeGDPR, ExportFormatJSON},
		{"TCPA Consent Trail", ReportTypeTCPA, ExportFormatCSV},
		{"SOX Financial Audit", ReportTypeSOX, ExportFormatParquet},
		{"Security Incidents", ReportTypeSecurityAudit, ExportFormatJSON},
	}

	for _, report := range reportTypes {
		var buf bytes.Buffer
		options := ExportOptions{
			Format:          report.format,
			ReportType:      report.reportType,
			RedactPII:       true,
			IncludeMetadata: true,
			ChunkSize:       1000,
		}

		_, err := exportService.Export(context.Background(), options, &buf)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Printf("Generated %s report (%s): %d bytes\n",
			report.name, report.format, buf.Len())
	}

	// Output:
	// Generated GDPR Data Subject report (json): 1000+ bytes
	// Generated TCPA Consent Trail report (csv): 500+ bytes
	// Generated SOX Financial Audit report (parquet): 800+ bytes
	// Generated Security Incidents report (json): 600+ bytes
}
