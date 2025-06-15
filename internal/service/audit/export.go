package audit

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// ExportFormat represents the supported export formats
type ExportFormat string

const (
	ExportFormatJSON    ExportFormat = "json"
	ExportFormatCSV     ExportFormat = "csv"
	ExportFormatParquet ExportFormat = "parquet"
)

// ReportType represents the type of compliance report
type ReportType string

const (
	ReportTypeGDPR          ReportType = "gdpr_data_subject"
	ReportTypeTCPA          ReportType = "tcpa_consent_trail"
	ReportTypeSOX           ReportType = "sox_financial_audit"
	ReportTypeSecurityAudit ReportType = "security_incident"
	ReportTypeCustom        ReportType = "custom_query"
)

// ExportOptions configures the export behavior
type ExportOptions struct {
	Format          ExportFormat
	ReportType      ReportType
	RedactPII       bool
	IncludeMetadata bool
	ChunkSize       int // For streaming
	Filters         map[string]interface{}
	TimeRange       *TimeRange
	CustomTemplate  string // For custom reports
}

// TimeRange defines a time period for filtering
type TimeRange struct {
	Start time.Time
	End   time.Time
}

// ExportProgress tracks the progress of an export operation
type ExportProgress struct {
	TotalRecords     int64
	ProcessedRecords int64
	StartTime        time.Time
	EstimatedTime    time.Duration
	CurrentPhase     string
	Errors           []string
	mu               sync.RWMutex
}

// Update safely updates the progress
func (p *ExportProgress) Update(processed int64, phase string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.ProcessedRecords = processed
	p.CurrentPhase = phase

	if p.ProcessedRecords > 0 && p.TotalRecords > 0 {
		elapsed := time.Since(p.StartTime)
		perRecord := elapsed / time.Duration(p.ProcessedRecords)
		remaining := p.TotalRecords - p.ProcessedRecords
		p.EstimatedTime = time.Duration(remaining) * perRecord
	}
}

// AddError adds an error to the progress
func (p *ExportProgress) AddError(err string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Errors = append(p.Errors, err)
}

// GetSnapshot returns a safe copy of the progress
func (p *ExportProgress) GetSnapshot() ExportProgress {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return ExportProgress{
		TotalRecords:     p.TotalRecords,
		ProcessedRecords: p.ProcessedRecords,
		StartTime:        p.StartTime,
		EstimatedTime:    p.EstimatedTime,
		CurrentPhase:     p.CurrentPhase,
		Errors:           append([]string(nil), p.Errors...),
	}
}

// ExportService handles compliance data exports
type ExportService struct {
	queryService *QueryService
	templates    map[ReportType]ReportTemplate
	sanitizers   map[string]DataSanitizer
	progressMap  sync.Map // map[string]*ExportProgress
}

// NewExportService creates a new export service
func NewExportService(queryService *QueryService) *ExportService {
	return &ExportService{
		queryService: queryService,
		templates:    initializeReportTemplates(),
		sanitizers:   initializeDataSanitizers(),
	}
}

// ReportTemplate defines the structure of a compliance report
type ReportTemplate struct {
	Name        string
	Description string
	Fields      []FieldDefinition
	Queries     []QueryDefinition
	Formatters  map[string]FieldFormatter
}

// FieldDefinition describes a field in the report
type FieldDefinition struct {
	Name        string
	Type        string
	Required    bool
	Sensitive   bool // Indicates PII
	Description string
	SourcePath  string // JSON path to source data
}

// QueryDefinition defines a query for the report
type QueryDefinition struct {
	Name   string
	Entity string
	Filter string
	Sort   string
	Limit  int
}

// FieldFormatter formats a field value
type FieldFormatter func(value interface{}) (string, error)

// DataSanitizer sanitizes sensitive data
type DataSanitizer func(value interface{}) interface{}

// Export generates a compliance export
func (s *ExportService) Export(ctx context.Context, options ExportOptions, writer io.Writer) (*ExportProgress, error) {
	// Validate options
	if err := s.validateOptions(options); err != nil {
		return nil, err
	}

	// Create progress tracker
	exportID := uuid.New().String()
	progress := &ExportProgress{
		StartTime:    time.Now(),
		CurrentPhase: "initializing",
	}
	s.progressMap.Store(exportID, progress)

	// Get report template
	template, err := s.getReportTemplate(options.ReportType, options.CustomTemplate)
	if err != nil {
		return nil, err
	}

	// Create appropriate exporter
	exporter, err := s.createExporter(options.Format, template, writer, options)
	if err != nil {
		return nil, err
	}
	defer exporter.Close()

	// Stream data
	err = s.streamData(ctx, template, options, exporter, progress)
	if err != nil {
		progress.AddError(fmt.Sprintf("Export failed: %v", err))
		return progress, err
	}

	progress.Update(progress.TotalRecords, "completed")
	return progress, nil
}

// GetProgress retrieves the progress of an export
func (s *ExportService) GetProgress(exportID string) (*ExportProgress, error) {
	value, ok := s.progressMap.Load(exportID)
	if !ok {
		return nil, errors.NewNotFoundError(fmt.Sprintf("export %s not found", exportID))
	}

	progress := value.(*ExportProgress)
	snapshot := progress.GetSnapshot()
	return &snapshot, nil
}

// validateOptions validates export options
func (s *ExportService) validateOptions(options ExportOptions) error {
	// Validate format
	switch options.Format {
	case ExportFormatJSON, ExportFormatCSV, ExportFormatParquet:
		// Valid formats
	default:
		return errors.NewValidationError("INVALID_FORMAT", "unsupported export format")
	}

	// Validate report type
	switch options.ReportType {
	case ReportTypeGDPR, ReportTypeTCPA, ReportTypeSOX, ReportTypeSecurityAudit:
		// Valid standard reports
	case ReportTypeCustom:
		if options.CustomTemplate == "" {
			return errors.NewValidationError("MISSING_TEMPLATE", "custom template required for custom reports")
		}
	default:
		return errors.NewValidationError("INVALID_REPORT_TYPE", "unsupported report type")
	}

	// Validate time range if provided
	if options.TimeRange != nil {
		if options.TimeRange.Start.After(options.TimeRange.End) {
			return errors.NewValidationError("INVALID_TIME_RANGE", "start time must be before end time")
		}
	}

	// Set default chunk size if not provided
	if options.ChunkSize <= 0 {
		options.ChunkSize = 1000
	}

	return nil
}

// getReportTemplate retrieves or creates a report template
func (s *ExportService) getReportTemplate(reportType ReportType, customTemplate string) (*ReportTemplate, error) {
	if reportType == ReportTypeCustom {
		// Parse custom template
		return s.parseCustomTemplate(customTemplate)
	}

	template, ok := s.templates[reportType]
	if !ok {
		return nil, errors.NewNotFoundError(fmt.Sprintf("template for report type %s not found", reportType))
	}

	return &template, nil
}

// parseCustomTemplate parses a custom report template
func (s *ExportService) parseCustomTemplate(templateStr string) (*ReportTemplate, error) {
	var template ReportTemplate

	// Parse JSON template
	if err := json.Unmarshal([]byte(templateStr), &template); err != nil {
		return nil, errors.NewValidationError("INVALID_TEMPLATE", "failed to parse custom template").WithCause(err)
	}

	// Validate template
	if template.Name == "" {
		return nil, errors.NewValidationError("INVALID_TEMPLATE", "template name is required")
	}

	if len(template.Fields) == 0 {
		return nil, errors.NewValidationError("INVALID_TEMPLATE", "at least one field is required")
	}

	return &template, nil
}

// Exporter interface for different export formats
type Exporter interface {
	WriteHeader() error
	WriteRecord(record map[string]interface{}) error
	Close() error
}

// createExporter creates the appropriate exporter based on format
func (s *ExportService) createExporter(format ExportFormat, template *ReportTemplate, writer io.Writer, options ExportOptions) (Exporter, error) {
	switch format {
	case ExportFormatJSON:
		return newJSONExporter(writer, template, options), nil
	case ExportFormatCSV:
		return newCSVExporter(writer, template, options), nil
	case ExportFormatParquet:
		return newParquetExporter(writer, template, options)
	default:
		return nil, errors.NewValidationError("UNSUPPORTED_FORMAT", fmt.Sprintf("format %s not supported", format))
	}
}

// JSONExporter exports data as JSON
type JSONExporter struct {
	writer   io.Writer
	template *ReportTemplate
	options  ExportOptions
	encoder  *json.Encoder
	first    bool
}

func newJSONExporter(writer io.Writer, template *ReportTemplate, options ExportOptions) *JSONExporter {
	return &JSONExporter{
		writer:   writer,
		template: template,
		options:  options,
		encoder:  json.NewEncoder(writer),
		first:    true,
	}
}

func (e *JSONExporter) WriteHeader() error {
	// Write opening bracket for JSON array
	_, err := e.writer.Write([]byte("[\n"))
	return err
}

func (e *JSONExporter) WriteRecord(record map[string]interface{}) error {
	// Add comma if not first record
	if !e.first {
		if _, err := e.writer.Write([]byte(",\n")); err != nil {
			return err
		}
	}
	e.first = false

	// Apply sanitization if needed
	if e.options.RedactPII {
		record = e.sanitizeRecord(record)
	}

	// Add metadata if requested
	if e.options.IncludeMetadata {
		record["_metadata"] = map[string]interface{}{
			"exported_at": time.Now().UTC(),
			"report_type": e.options.ReportType,
			"redacted":    e.options.RedactPII,
		}
	}

	return e.encoder.Encode(record)
}

func (e *JSONExporter) Close() error {
	// Write closing bracket
	_, err := e.writer.Write([]byte("\n]"))
	return err
}

func (e *JSONExporter) sanitizeRecord(record map[string]interface{}) map[string]interface{} {
	sanitized := make(map[string]interface{})

	for _, field := range e.template.Fields {
		value, exists := record[field.Name]
		if !exists {
			continue
		}

		if field.Sensitive && e.options.RedactPII {
			sanitized[field.Name] = redactValue(field.Type, value)
		} else {
			sanitized[field.Name] = value
		}
	}

	return sanitized
}

// CSVExporter exports data as CSV
type CSVExporter struct {
	writer   *csv.Writer
	template *ReportTemplate
	options  ExportOptions
	headers  []string
}

func newCSVExporter(writer io.Writer, template *ReportTemplate, options ExportOptions) *CSVExporter {
	csvWriter := csv.NewWriter(writer)

	// Extract headers from template
	headers := make([]string, 0, len(template.Fields))
	for _, field := range template.Fields {
		headers = append(headers, field.Name)
	}

	if options.IncludeMetadata {
		headers = append(headers, "_exported_at", "_report_type", "_redacted")
	}

	return &CSVExporter{
		writer:   csvWriter,
		template: template,
		options:  options,
		headers:  headers,
	}
}

func (e *CSVExporter) WriteHeader() error {
	return e.writer.Write(e.headers)
}

func (e *CSVExporter) WriteRecord(record map[string]interface{}) error {
	row := make([]string, 0, len(e.headers))

	// Process fields in order
	for _, field := range e.template.Fields {
		value, exists := record[field.Name]
		if !exists {
			row = append(row, "")
			continue
		}

		// Apply sanitization if needed
		if field.Sensitive && e.options.RedactPII {
			value = redactValue(field.Type, value)
		}

		// Format value
		formatted := formatValue(value)
		row = append(row, formatted)
	}

	// Add metadata if requested
	if e.options.IncludeMetadata {
		row = append(row,
			time.Now().UTC().Format(time.RFC3339),
			string(e.options.ReportType),
			fmt.Sprintf("%t", e.options.RedactPII),
		)
	}

	return e.writer.Write(row)
}

func (e *CSVExporter) Close() error {
	e.writer.Flush()
	return e.writer.Error()
}

// ParquetExporter exports data in a parquet-like columnar JSON format
// This is a simplified implementation - would need actual parquet library for production
type ParquetExporter struct {
	writer   io.Writer
	template *ReportTemplate
	options  ExportOptions
	encoder  *json.Encoder
	schema   map[string]interface{}
	data     []map[string]interface{}
}

func newParquetExporter(writer io.Writer, template *ReportTemplate, options ExportOptions) (*ParquetExporter, error) {
	// Create schema based on template fields
	schema := make(map[string]interface{})
	for _, field := range template.Fields {
		schema[field.Name] = map[string]interface{}{
			"type":        field.Type,
			"required":    field.Required,
			"sensitive":   field.Sensitive,
			"description": field.Description,
		}
	}

	return &ParquetExporter{
		writer:   writer,
		template: template,
		options:  options,
		encoder:  json.NewEncoder(writer),
		schema:   schema,
		data:     make([]map[string]interface{}, 0),
	}, nil
}

func (e *ParquetExporter) WriteHeader() error {
	// Write parquet-like metadata header
	header := map[string]interface{}{
		"format":     "parquet-json",
		"version":    "1.0",
		"schema":     e.schema,
		"created_at": time.Now().UTC(),
		"options":    e.options,
	}

	if _, err := e.writer.Write([]byte("# Parquet-like Export\n")); err != nil {
		return err
	}

	if err := e.encoder.Encode(map[string]interface{}{"header": header}); err != nil {
		return err
	}

	if _, err := e.writer.Write([]byte("# Data Records\n")); err != nil {
		return err
	}

	return nil
}

func (e *ParquetExporter) WriteRecord(record map[string]interface{}) error {
	// Apply sanitization if needed
	if e.options.RedactPII {
		for _, field := range e.template.Fields {
			if field.Sensitive {
				if value, exists := record[field.Name]; exists {
					record[field.Name] = redactValue(field.Type, value)
				}
			}
		}
	}

	// Add metadata if requested
	if e.options.IncludeMetadata {
		record["_metadata"] = map[string]interface{}{
			"exported_at": time.Now().UTC(),
			"report_type": e.options.ReportType,
			"redacted":    e.options.RedactPII,
		}
	}

	// Store for columnar output
	e.data = append(e.data, record)

	return nil
}

func (e *ParquetExporter) Close() error {
	// Write data in columnar format (simplified parquet-like structure)
	columnarData := e.convertToColumnar()

	output := map[string]interface{}{
		"data":         columnarData,
		"row_count":    len(e.data),
		"column_count": len(e.schema),
	}

	return e.encoder.Encode(output)
}

// convertToColumnar converts row-based data to columnar format
func (e *ParquetExporter) convertToColumnar() map[string][]interface{} {
	columnar := make(map[string][]interface{})

	// Initialize columns
	for _, field := range e.template.Fields {
		columnar[field.Name] = make([]interface{}, 0, len(e.data))
	}

	// Convert rows to columns
	for _, record := range e.data {
		for _, field := range e.template.Fields {
			value := record[field.Name]
			columnar[field.Name] = append(columnar[field.Name], value)
		}
	}

	return columnar
}

// streamData streams data from queries to the exporter
func (s *ExportService) streamData(ctx context.Context, template *ReportTemplate, options ExportOptions, exporter Exporter, progress *ExportProgress) error {
	// Write header
	if err := exporter.WriteHeader(); err != nil {
		return errors.NewInternalError("failed to write header").WithCause(err)
	}

	progress.Update(0, "querying data")

	// Execute queries defined in template
	for _, queryDef := range template.Queries {
		// Build query with filters
		query := s.buildQuery(queryDef, options)

		// Count total records
		countReq := QueryRequest{
			Entity: queryDef.Entity,
			Filter: query.Filter,
			Sort:   query.Sort,
		}
		count, err := s.queryService.Count(ctx, countReq)
		if err != nil {
			return errors.NewInternalError("failed to count records").WithCause(err)
		}

		progress.TotalRecords += count

		// Stream records in chunks
		offset := 0
		for {
			// Check context cancellation
			if err := ctx.Err(); err != nil {
				return errors.NewInternalError("export cancelled").WithCause(err)
			}

			// Fetch chunk
			records, err := s.queryService.Query(ctx, QueryRequest{
				Entity: queryDef.Entity,
				Filter: query.Filter,
				Sort:   query.Sort,
				Limit:  options.ChunkSize,
				Offset: offset,
			})
			if err != nil {
				return errors.NewInternalError("failed to query records").WithCause(err)
			}

			// Process records
			for _, record := range records {
				// Transform record based on template
				transformed := s.transformRecord(record, template)

				// Write record
				if err := exporter.WriteRecord(transformed); err != nil {
					return errors.NewInternalError("failed to write record").WithCause(err)
				}

				progress.Update(progress.ProcessedRecords+1, "exporting")
			}

			// Check if we've processed all records
			if len(records) < options.ChunkSize {
				break
			}

			offset += options.ChunkSize
		}
	}

	return nil
}

// buildQuery builds a query from definition and options
func (s *ExportService) buildQuery(queryDef QueryDefinition, options ExportOptions) QueryDefinition {
	query := queryDef

	// Apply time range filter
	if options.TimeRange != nil {
		timeFilter := fmt.Sprintf("created_at >= '%s' AND created_at <= '%s'",
			options.TimeRange.Start.Format(time.RFC3339),
			options.TimeRange.End.Format(time.RFC3339))

		if query.Filter != "" {
			query.Filter = fmt.Sprintf("(%s) AND (%s)", query.Filter, timeFilter)
		} else {
			query.Filter = timeFilter
		}
	}

	// Apply custom filters
	for key, value := range options.Filters {
		filter := fmt.Sprintf("%s = '%v'", key, value)
		if query.Filter != "" {
			query.Filter = fmt.Sprintf("%s AND %s", query.Filter, filter)
		} else {
			query.Filter = filter
		}
	}

	return query
}

// transformRecord transforms a raw record based on template
func (s *ExportService) transformRecord(record map[string]interface{}, template *ReportTemplate) map[string]interface{} {
	transformed := make(map[string]interface{})

	for _, field := range template.Fields {
		// Extract value using source path
		value := extractValue(record, field.SourcePath)

		// Apply formatter if available
		if formatter, exists := template.Formatters[field.Name]; exists {
			if formatted, err := formatter(value); err == nil {
				value = formatted
			}
		}

		transformed[field.Name] = value
	}

	return transformed
}

// Helper functions

// redactValue redacts sensitive data based on type
func redactValue(fieldType string, value interface{}) interface{} {
	if value == nil {
		return nil
	}

	switch fieldType {
	case "phone":
		// Keep area code, redact rest
		if phone, ok := value.(string); ok && len(phone) >= 10 {
			return phone[:6] + "****"
		}
	case "email":
		// Keep domain, redact local part
		if email, ok := value.(string); ok {
			parts := strings.Split(email, "@")
			if len(parts) == 2 {
				return "****@" + parts[1]
			}
		}
	case "ssn", "ein":
		// Show last 4 digits only
		if ssn, ok := value.(string); ok && len(ssn) >= 4 {
			return "***-**-" + ssn[len(ssn)-4:]
		}
	case "address":
		// Redact street address, keep city/state
		if addr, ok := value.(map[string]interface{}); ok {
			addr["street"] = "****"
			if suite, exists := addr["suite"]; exists && suite != nil {
				addr["suite"] = "****"
			}
			return addr
		}
	case "name":
		// Show initials only
		if name, ok := value.(string); ok {
			parts := strings.Split(name, " ")
			initials := ""
			for _, part := range parts {
				if len(part) > 0 {
					initials += string(part[0]) + ". "
				}
			}
			return strings.TrimSpace(initials)
		}
	case "creditcard":
		// Show last 4 digits only
		if cc, ok := value.(string); ok && len(cc) >= 4 {
			return "****-****-****-" + cc[len(cc)-4:]
		}
	}

	// Default: fully redact
	return "****"
}

// formatValue formats a value for string representation
func formatValue(value interface{}) string {
	if value == nil {
		return ""
	}

	switch v := value.(type) {
	case string:
		return v
	case int, int32, int64:
		return fmt.Sprintf("%d", v)
	case float32, float64:
		return fmt.Sprintf("%f", v)
	case bool:
		return fmt.Sprintf("%t", v)
	case time.Time:
		return v.Format(time.RFC3339)
	case decimal.Decimal:
		return v.String()
	case uuid.UUID:
		return v.String()
	default:
		// Try JSON encoding for complex types
		if data, err := json.Marshal(v); err == nil {
			return string(data)
		}
		return fmt.Sprintf("%v", v)
	}
}

// extractValue extracts a value from a record using a path
func extractValue(record map[string]interface{}, path string) interface{} {
	if path == "" {
		return nil
	}

	// Simple dot notation support
	parts := strings.Split(path, ".")
	current := record

	for i, part := range parts {
		// Handle array notation like "items[0]"
		if strings.Contains(part, "[") {
			// Simplified - would need proper parsing for production
			fieldName := strings.Split(part, "[")[0]
			if value, ok := current[fieldName]; ok {
				// Assume it's an array and take first element
				if arr, ok := value.([]interface{}); ok && len(arr) > 0 {
					if i == len(parts)-1 {
						return arr[0]
					}
					if next, ok := arr[0].(map[string]interface{}); ok {
						current = next
						continue
					}
				}
			}
			return nil
		}

		// Regular field access
		if value, ok := current[part]; ok {
			if i == len(parts)-1 {
				return value
			}
			if next, ok := value.(map[string]interface{}); ok {
				current = next
			} else {
				return nil
			}
		} else {
			return nil
		}
	}

	return nil
}

// Initialize report templates
func initializeReportTemplates() map[ReportType]ReportTemplate {
	return map[ReportType]ReportTemplate{
		ReportTypeGDPR: {
			Name:        "GDPR Data Subject Report",
			Description: "Complete data export for GDPR data subject access request",
			Fields: []FieldDefinition{
				{Name: "user_id", Type: "uuid", Required: true, SourcePath: "user.id"},
				{Name: "email", Type: "email", Required: true, Sensitive: true, SourcePath: "user.email"},
				{Name: "phone", Type: "phone", Required: false, Sensitive: true, SourcePath: "user.phone"},
				{Name: "created_at", Type: "timestamp", Required: true, SourcePath: "user.created_at"},
				{Name: "calls_made", Type: "integer", Required: false, SourcePath: "stats.calls_made"},
				{Name: "consents", Type: "array", Required: false, SourcePath: "consents"},
				{Name: "data_retention", Type: "object", Required: false, SourcePath: "retention"},
			},
			Queries: []QueryDefinition{
				{
					Name:   "user_data",
					Entity: "users",
					Filter: "id = :user_id",
				},
				{
					Name:   "call_records",
					Entity: "calls",
					Filter: "user_id = :user_id",
					Sort:   "created_at DESC",
				},
				{
					Name:   "consent_records",
					Entity: "consents",
					Filter: "user_id = :user_id",
					Sort:   "created_at DESC",
				},
			},
		},
		ReportTypeTCPA: {
			Name:        "TCPA Consent Trail Report",
			Description: "Complete consent history for TCPA compliance",
			Fields: []FieldDefinition{
				{Name: "consent_id", Type: "uuid", Required: true, SourcePath: "id"},
				{Name: "phone_number", Type: "phone", Required: true, Sensitive: true, SourcePath: "phone_number"},
				{Name: "consent_type", Type: "string", Required: true, SourcePath: "type"},
				{Name: "consent_status", Type: "string", Required: true, SourcePath: "status"},
				{Name: "consent_timestamp", Type: "timestamp", Required: true, SourcePath: "created_at"},
				{Name: "ip_address", Type: "string", Required: false, Sensitive: true, SourcePath: "metadata.ip_address"},
				{Name: "user_agent", Type: "string", Required: false, SourcePath: "metadata.user_agent"},
				{Name: "source", Type: "string", Required: true, SourcePath: "source"},
				{Name: "revoked_at", Type: "timestamp", Required: false, SourcePath: "revoked_at"},
			},
			Queries: []QueryDefinition{
				{
					Name:   "consent_history",
					Entity: "tcpa_consents",
					Sort:   "created_at DESC",
				},
			},
		},
		ReportTypeSOX: {
			Name:        "SOX Financial Audit Report",
			Description: "Financial transaction audit trail for SOX compliance",
			Fields: []FieldDefinition{
				{Name: "transaction_id", Type: "uuid", Required: true, SourcePath: "id"},
				{Name: "transaction_type", Type: "string", Required: true, SourcePath: "type"},
				{Name: "amount", Type: "decimal", Required: true, SourcePath: "amount"},
				{Name: "currency", Type: "string", Required: true, SourcePath: "currency"},
				{Name: "timestamp", Type: "timestamp", Required: true, SourcePath: "created_at"},
				{Name: "buyer_id", Type: "uuid", Required: false, SourcePath: "buyer_id"},
				{Name: "seller_id", Type: "uuid", Required: false, SourcePath: "seller_id"},
				{Name: "call_id", Type: "uuid", Required: false, SourcePath: "call_id"},
				{Name: "status", Type: "string", Required: true, SourcePath: "status"},
				{Name: "audit_user", Type: "string", Required: false, SourcePath: "audit.user"},
				{Name: "audit_action", Type: "string", Required: false, SourcePath: "audit.action"},
				{Name: "audit_timestamp", Type: "timestamp", Required: false, SourcePath: "audit.timestamp"},
			},
			Queries: []QueryDefinition{
				{
					Name:   "financial_transactions",
					Entity: "transactions",
					Sort:   "created_at DESC",
				},
				{
					Name:   "audit_events",
					Entity: "audit_events",
					Filter: "entity_type = 'transaction'",
					Sort:   "created_at DESC",
				},
			},
		},
		ReportTypeSecurityAudit: {
			Name:        "Security Incident Report",
			Description: "Security events and incidents for compliance reporting",
			Fields: []FieldDefinition{
				{Name: "event_id", Type: "uuid", Required: true, SourcePath: "id"},
				{Name: "event_type", Type: "string", Required: true, SourcePath: "type"},
				{Name: "severity", Type: "string", Required: true, SourcePath: "severity"},
				{Name: "timestamp", Type: "timestamp", Required: true, SourcePath: "created_at"},
				{Name: "user_id", Type: "uuid", Required: false, SourcePath: "user_id"},
				{Name: "ip_address", Type: "string", Required: false, Sensitive: true, SourcePath: "ip_address"},
				{Name: "action", Type: "string", Required: true, SourcePath: "action"},
				{Name: "resource", Type: "string", Required: false, SourcePath: "resource"},
				{Name: "result", Type: "string", Required: true, SourcePath: "result"},
				{Name: "details", Type: "object", Required: false, SourcePath: "details"},
			},
			Queries: []QueryDefinition{
				{
					Name:   "security_events",
					Entity: "security_events",
					Sort:   "created_at DESC",
				},
			},
		},
	}
}

// Initialize data sanitizers
func initializeDataSanitizers() map[string]DataSanitizer {
	return map[string]DataSanitizer{
		"phone": func(value interface{}) interface{} {
			if phone, ok := value.(string); ok && len(phone) >= 10 {
				return phone[:6] + "****"
			}
			return "****"
		},
		"email": func(value interface{}) interface{} {
			if email, ok := value.(string); ok {
				parts := strings.Split(email, "@")
				if len(parts) == 2 {
					return "****@" + parts[1]
				}
			}
			return "****"
		},
		"ip_address": func(value interface{}) interface{} {
			if ip, ok := value.(string); ok {
				parts := strings.Split(ip, ".")
				if len(parts) == 4 {
					return fmt.Sprintf("%s.%s.*.*", parts[0], parts[1])
				}
			}
			return "*.*.*.*"
		},
		"name": func(value interface{}) interface{} {
			if name, ok := value.(string); ok {
				parts := strings.Split(name, " ")
				initials := ""
				for _, part := range parts {
					if len(part) > 0 {
						initials += string(part[0]) + ". "
					}
				}
				return strings.TrimSpace(initials)
			}
			return "****"
		},
	}
}

// GDPRExport generates a GDPR data subject export
func (s *ExportService) GDPRExport(ctx context.Context, userID uuid.UUID, writer io.Writer) error {
	options := ExportOptions{
		Format:          ExportFormatJSON,
		ReportType:      ReportTypeGDPR,
		RedactPII:       false, // GDPR requires full data
		IncludeMetadata: true,
		ChunkSize:       1000,
		Filters: map[string]interface{}{
			"user_id": userID.String(),
		},
	}

	_, err := s.Export(ctx, options, writer)
	return err
}

// TCPAConsentExport generates a TCPA consent trail export
func (s *ExportService) TCPAConsentExport(ctx context.Context, phoneNumber string, writer io.Writer) error {
	options := ExportOptions{
		Format:          ExportFormatCSV,
		ReportType:      ReportTypeTCPA,
		RedactPII:       false,
		IncludeMetadata: true,
		ChunkSize:       1000,
		Filters: map[string]interface{}{
			"phone_number": phoneNumber,
		},
	}

	_, err := s.Export(ctx, options, writer)
	return err
}

// FinancialAuditExport generates a SOX compliance financial audit export
func (s *ExportService) FinancialAuditExport(ctx context.Context, timeRange TimeRange, writer io.Writer) error {
	options := ExportOptions{
		Format:          ExportFormatParquet,
		ReportType:      ReportTypeSOX,
		RedactPII:       true,
		IncludeMetadata: true,
		ChunkSize:       5000,
		TimeRange:       &timeRange,
	}

	_, err := s.Export(ctx, options, writer)
	return err
}

// SecurityIncidentExport generates a security incident report
func (s *ExportService) SecurityIncidentExport(ctx context.Context, severity string, timeRange TimeRange, writer io.Writer) error {
	options := ExportOptions{
		Format:          ExportFormatJSON,
		ReportType:      ReportTypeSecurityAudit,
		RedactPII:       true,
		IncludeMetadata: true,
		ChunkSize:       1000,
		TimeRange:       &timeRange,
		Filters: map[string]interface{}{
			"severity": severity,
		},
	}

	_, err := s.Export(ctx, options, writer)
	return err
}
