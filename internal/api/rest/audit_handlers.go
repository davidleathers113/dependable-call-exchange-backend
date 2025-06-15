package rest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/audit"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
	"github.com/google/uuid"
	"github.com/go-playground/validator/v10"
)

// AuditHandler handles audit-related API endpoints
type AuditHandler struct {
	*BaseHandler
	queryService      QueryService
	exportService     ExportService
	complianceService ComplianceService
	logger            *slog.Logger
	validator         *validator.Validate
}

// NewAuditHandler creates a new audit handler
func NewAuditHandler(
	baseHandler *BaseHandler,
	queryService QueryService,
	exportService ExportService,
	complianceService ComplianceService,
	logger *slog.Logger,
) *AuditHandler {
	validator := validator.New()
	
	return &AuditHandler{
		BaseHandler:       baseHandler,
		queryService:      queryService,
		exportService:     exportService,
		complianceService: complianceService,
		logger:           logger,
		validator:        validator,
	}
}

// RegisterAuditRoutes registers all audit-related routes
func (h *AuditHandler) RegisterAuditRoutes(mux *http.ServeMux) {
	// Create middleware chain for protected endpoints
	authChain := NewMiddlewareChain(
		SecurityHeadersMiddleware(),
		RequestIDMiddleware(),
		RequestLoggingMiddleware(h.logger),
		MetricsMiddleware(),
		ContentNegotiationMiddleware(),
		// AuthMiddleware would be added here
	)

	// Rate limiter for audit operations (lower limits due to resource intensive operations)
	auditRateLimiter := NewRateLimiter(RateLimitConfig{
		RequestsPerSecond: 20,
		Burst:             40,
		ByUser:            true,
	})

	// Query audit events endpoints
	mux.Handle("GET /api/v1/audit/events", authChain.Then(
		auditRateLimiter.Middleware()(
			h.WrapHandler("GET", "/api/v1/audit/events", h.handleQueryEvents,
				WithCache(2*time.Minute),
				WithTimeout(30*time.Second),
			),
		),
	))

	mux.Handle("GET /api/v1/audit/events/{id}", authChain.Then(
		auditRateLimiter.Middleware()(
			h.WrapHandler("GET", "/api/v1/audit/events/{id}", h.handleGetEvent,
				WithCache(5*time.Minute),
				WithTimeout(10*time.Second),
			),
		),
	))

	// Advanced search endpoint
	mux.Handle("GET /api/v1/audit/search", authChain.Then(
		auditRateLimiter.Middleware()(
			h.WrapHandler("GET", "/api/v1/audit/search", h.handleAdvancedSearch,
				WithCache(1*time.Minute),
				WithTimeout(45*time.Second),
			),
		),
	))

	// Statistics and metrics endpoints
	mux.Handle("GET /api/v1/audit/stats", authChain.Then(
		auditRateLimiter.Middleware()(
			h.WrapHandler("GET", "/api/v1/audit/stats", h.handleGetStats,
				WithCache(5*time.Minute),
				WithTimeout(20*time.Second),
			),
		),
	))

	// Export endpoints with stricter rate limiting
	exportRateLimiter := NewRateLimiter(RateLimitConfig{
		RequestsPerSecond: 2, // Very strict for exports
		Burst:             5,
		ByUser:            true,
	})

	mux.Handle("GET /api/v1/audit/export/{type}", authChain.Then(
		exportRateLimiter.Middleware()(
			h.WrapHandler("GET", "/api/v1/audit/export/{type}", h.handleExportReport,
				WithoutValidation(), // Custom validation in handler
				WithTimeout(10*time.Minute), // Exports can take longer
			),
		),
	))

	// Streaming endpoints for large datasets
	mux.Handle("GET /api/v1/audit/stream", authChain.Then(
		exportRateLimiter.Middleware()(
			h.WrapHandler("GET", "/api/v1/audit/stream", h.handleStreamEvents,
				WithoutValidation(),
				WithTimeout(30*time.Minute), // Long-running streams
			),
		),
	))

	// Compliance-specific endpoints
	mux.Handle("GET /api/v1/audit/compliance/gdpr", authChain.Then(
		auditRateLimiter.Middleware()(
			h.WrapHandler("GET", "/api/v1/audit/compliance/gdpr", h.handleGDPRCompliance,
				WithCache(30*time.Minute),
				WithTimeout(60*time.Second),
			),
		),
	))

	mux.Handle("GET /api/v1/audit/compliance/tcpa", authChain.Then(
		auditRateLimiter.Middleware()(
			h.WrapHandler("GET", "/api/v1/audit/compliance/tcpa", h.handleTCPACompliance,
				WithCache(15*time.Minute),
				WithTimeout(45*time.Second),
			),
		),
	))

	// Integrity and monitoring endpoints
	mux.Handle("GET /api/v1/audit/integrity", authChain.Then(
		auditRateLimiter.Middleware()(
			h.WrapHandler("GET", "/api/v1/audit/integrity", h.handleIntegrityCheck,
				WithCache(5*time.Minute),
				WithTimeout(60*time.Second),
			),
		),
	))
}

// Handler implementations

func (h *AuditHandler) handleQueryEvents(ctx context.Context, r *http.Request) (interface{}, error) {
	// Parse query parameters
	filters, err := h.parseEventFilters(r)
	if err != nil {
		return nil, errors.NewValidationError("INVALID_FILTERS", "invalid query parameters").WithCause(err)
	}

	// Parse pagination
	pagination, err := h.parsePagination(r)
	if err != nil {
		return nil, errors.NewValidationError("INVALID_PAGINATION", "invalid pagination parameters").WithCause(err)
	}

	// Build query request
	queryReq := EventQueryRequest{
		Filters:    filters,
		Pagination: pagination,
		SortBy:     h.getQueryParam(r, "sort_by", "created_at"),
		SortOrder:  h.getQueryParam(r, "sort_order", "desc"),
	}

	// Execute query
	result, err := h.queryService.QueryEvents(ctx, queryReq)
	if err != nil {
		return nil, h.handleServiceError(err)
	}

	// Convert to API response
	response := &AuditEventListResponse{
		Events:     h.convertEventsToResponse(result.Events),
		Pagination: h.convertPaginationToResponse(result.Pagination),
		Metadata: map[string]interface{}{
			"total_duration_ms": result.QueryDurationMs,
			"cache_hit":         result.CacheHit,
		},
	}

	return response, nil
}

func (h *AuditHandler) handleGetEvent(ctx context.Context, r *http.Request) (interface{}, error) {
	eventID, err := h.parseUUIDParam(r, "id")
	if err != nil {
		return nil, errors.NewValidationError("INVALID_EVENT_ID", "event ID must be a valid UUID")
	}

	// Get single event
	event, err := h.queryService.GetEvent(ctx, eventID)
	if err != nil {
		return nil, h.handleServiceError(err)
	}

	return h.convertEventToResponse(event), nil
}

func (h *AuditHandler) handleAdvancedSearch(ctx context.Context, r *http.Request) (interface{}, error) {
	// Parse search request
	searchReq, err := h.parseSearchRequest(r)
	if err != nil {
		return nil, errors.NewValidationError("INVALID_SEARCH", "invalid search parameters").WithCause(err)
	}

	// Validate search query
	if err := h.validator.Struct(searchReq); err != nil {
		return nil, h.formatValidationError(err)
	}

	// Execute search
	result, err := h.queryService.AdvancedSearch(ctx, *searchReq)
	if err != nil {
		return nil, h.handleServiceError(err)
	}

	response := &AuditSearchResponse{
		Results:    h.convertEventsToResponse(result.Events),
		Pagination: h.convertPaginationToResponse(result.Pagination),
		Facets:     result.Facets,
		Highlights: result.Highlights,
		Metadata: map[string]interface{}{
			"search_time_ms": result.SearchTimeMs,
			"total_hits":     result.TotalHits,
		},
	}

	return response, nil
}

func (h *AuditHandler) handleGetStats(ctx context.Context, r *http.Request) (interface{}, error) {
	// Parse time range
	timeRange, err := h.parseTimeRange(r)
	if err != nil {
		return nil, errors.NewValidationError("INVALID_TIME_RANGE", "invalid time range parameters").WithCause(err)
	}

	// Get statistics
	stats, err := h.queryService.GetEventStatistics(ctx, StatsRequest{
		TimeRange: timeRange,
		GroupBy:   h.getQueryParam(r, "group_by", "hour"),
		Metrics:   h.getQueryParams(r, "metrics"),
	})
	if err != nil {
		return nil, h.handleServiceError(err)
	}

	return &AuditStatsResponse{
		TimeRange:    timeRange,
		TotalEvents:  stats.TotalEvents,
		EventsByType: stats.EventsByType,
		Timeline:     stats.Timeline,
		TopActors:    stats.TopActors,
		ErrorRate:    stats.ErrorRate,
		Metadata: map[string]interface{}{
			"computed_at":   time.Now(),
			"cache_status": stats.CacheStatus,
		},
	}, nil
}

func (h *AuditHandler) handleExportReport(ctx context.Context, r *http.Request) (interface{}, error) {
	reportType := r.PathValue("type")
	if reportType == "" {
		return nil, errors.NewValidationError("MISSING_REPORT_TYPE", "report type is required")
	}

	// Parse export options
	options, err := h.parseExportOptions(r, reportType)
	if err != nil {
		return nil, errors.NewValidationError("INVALID_EXPORT_OPTIONS", "invalid export options").WithCause(err)
	}

	// Check if this is a streaming response request
	if h.getQueryParam(r, "stream", "false") == "true" {
		return h.handleStreamingExport(ctx, r, reportType, options)
	}

	// Generate export
	export, err := h.exportService.GenerateReport(ctx, ExportRequest{
		ReportType: ReportType(reportType),
		Options:    *options,
		RequestID:  h.getRequestMeta(ctx).RequestID,
	})
	if err != nil {
		return nil, h.handleServiceError(err)
	}

	response := &AuditExportResponse{
		ExportID:     export.ID,
		ReportType:   reportType,
		Status:       export.Status,
		Format:       string(export.Format),
		Size:         export.Size,
		RecordCount:  export.RecordCount,
		GeneratedAt:  export.GeneratedAt,
		ExpiresAt:    export.ExpiresAt,
		DownloadURL:  export.DownloadURL,
		Checksum:     export.Checksum,
		Metadata:     export.Metadata,
	}

	return response, nil
}

func (h *AuditHandler) handleStreamEvents(ctx context.Context, r *http.Request) (interface{}, error) {
	// This would typically use Server-Sent Events or WebSocket
	// For now, we'll implement a simple chunked JSON response
	
	filters, err := h.parseEventFilters(r)
	if err != nil {
		return nil, errors.NewValidationError("INVALID_FILTERS", "invalid stream filters").WithCause(err)
	}

	chunkSize := h.getQueryParamInt(r, "chunk_size", 100)
	if chunkSize > 1000 {
		chunkSize = 1000 // Limit chunk size
	}

	// Start streaming
	stream, err := h.queryService.StreamEvents(ctx, StreamRequest{
		Filters:   filters,
		ChunkSize: chunkSize,
		Format:    ExportFormatJSON,
	})
	if err != nil {
		return nil, h.handleServiceError(err)
	}

	// Return stream metadata (actual streaming would be handled differently)
	return &AuditStreamResponse{
		StreamID:  stream.ID,
		Status:    stream.Status,
		ChunkSize: chunkSize,
		StartedAt: time.Now(),
		Metadata: map[string]interface{}{
			"estimated_total": stream.EstimatedTotal,
			"stream_type":     "audit_events",
		},
	}, nil
}

func (h *AuditHandler) handleGDPRCompliance(ctx context.Context, r *http.Request) (interface{}, error) {
	subjectID := h.getQueryParam(r, "subject_id", "")
	if subjectID == "" {
		return nil, errors.NewValidationError("MISSING_SUBJECT_ID", "GDPR subject ID is required")
	}

	// Parse time range for GDPR report
	timeRange, err := h.parseTimeRange(r)
	if err != nil {
		return nil, errors.NewValidationError("INVALID_TIME_RANGE", "invalid time range").WithCause(err)
	}

	// Generate GDPR compliance report
	report, err := h.complianceService.GenerateGDPRReport(ctx, GDPRRequest{
		SubjectID:      subjectID,
		TimeRange:      timeRange,
		IncludePII:     h.getQueryParam(r, "include_pii", "false") == "true",
		ExportFormat:   ExportFormat(h.getQueryParam(r, "format", "json")),
	})
	if err != nil {
		return nil, h.handleServiceError(err)
	}

	return &GDPRComplianceResponse{
		SubjectID:       subjectID,
		ReportID:        report.ID,
		GeneratedAt:     report.GeneratedAt,
		DataPoints:      report.DataPoints,
		ProcessingBases: report.ProcessingBases,
		RetentionPolicy: report.RetentionPolicy,
		RightsExercised: report.RightsExercised,
		ConsentHistory:  report.ConsentHistory,
		DataTransfers:   report.DataTransfers,
		Metadata:        report.Metadata,
	}, nil
}

func (h *AuditHandler) handleTCPACompliance(ctx context.Context, r *http.Request) (interface{}, error) {
	phoneNumber := h.getQueryParam(r, "phone_number", "")
	if phoneNumber == "" {
		return nil, errors.NewValidationError("MISSING_PHONE_NUMBER", "phone number is required for TCPA compliance")
	}

	// Parse time range
	timeRange, err := h.parseTimeRange(r)
	if err != nil {
		return nil, errors.NewValidationError("INVALID_TIME_RANGE", "invalid time range").WithCause(err)
	}

	// Generate TCPA compliance report
	report, err := h.complianceService.GenerateTCPAReport(ctx, TCPARequest{
		PhoneNumber: phoneNumber,
		TimeRange:   timeRange,
		Detailed:    h.getQueryParam(r, "detailed", "false") == "true",
	})
	if err != nil {
		return nil, h.handleServiceError(err)
	}

	return &TCPAComplianceResponse{
		PhoneNumber:       phoneNumber,
		ReportID:          report.ID,
		GeneratedAt:       report.GeneratedAt,
		ConsentStatus:     report.ConsentStatus,
		CallHistory:       report.CallHistory,
		ViolationHistory:  report.ViolationHistory,
		OptOutHistory:     report.OptOutHistory,
		CallingTimeChecks: report.CallingTimeChecks,
		Metadata:          report.Metadata,
	}, nil
}

func (h *AuditHandler) handleIntegrityCheck(ctx context.Context, r *http.Request) (interface{}, error) {
	// Parse integrity check parameters
	checkType := h.getQueryParam(r, "type", "full")
	timeRange, err := h.parseTimeRange(r)
	if err != nil {
		return nil, errors.NewValidationError("INVALID_TIME_RANGE", "invalid time range").WithCause(err)
	}

	// Perform integrity check
	result, err := h.queryService.PerformIntegrityCheck(ctx, IntegrityCheckRequest{
		CheckType: checkType,
		TimeRange: timeRange,
		Deep:      h.getQueryParam(r, "deep", "false") == "true",
	})
	if err != nil {
		return nil, h.handleServiceError(err)
	}

	return &IntegrityCheckResponse{
		CheckID:       result.ID,
		CheckType:     checkType,
		Status:        result.Status,
		StartedAt:     result.StartedAt,
		CompletedAt:   result.CompletedAt,
		EventsChecked: result.EventsChecked,
		IssuesFound:   result.IssuesFound,
		IntegrityScore: result.IntegrityScore,
		Issues:        result.Issues,
		Recommendations: result.Recommendations,
		Metadata:      result.Metadata,
	}, nil
}

// Helper methods for streaming export
func (h *AuditHandler) handleStreamingExport(ctx context.Context, r *http.Request, reportType string, options *ExportOptions) (interface{}, error) {
	// For HTTP streaming, we would set appropriate headers and write chunks
	// This is a simplified version that returns stream initiation info
	
	stream, err := h.exportService.StartStreamingExport(ctx, StreamingExportRequest{
		ReportType: ReportType(reportType),
		Options:    *options,
		ChunkSize:  h.getQueryParamInt(r, "chunk_size", 1000),
	})
	if err != nil {
		return nil, h.handleServiceError(err)
	}

	return &StreamingExportResponse{
		StreamID:        stream.ID,
		Status:         stream.Status,
		EstimatedTotal: stream.EstimatedTotal,
		ChunkSize:      stream.ChunkSize,
		StartedAt:      time.Now(),
		Metadata: map[string]interface{}{
			"stream_url": fmt.Sprintf("/api/v1/audit/export/stream/%s", stream.ID),
		},
	}, nil
}

// Helper methods

func (h *AuditHandler) parseEventFilters(r *http.Request) (map[string]interface{}, error) {
	filters := make(map[string]interface{})
	
	// Basic filters
	if actor := h.getQueryParam(r, "actor", ""); actor != "" {
		filters["actor"] = actor
	}
	if eventType := h.getQueryParam(r, "event_type", ""); eventType != "" {
		filters["event_type"] = eventType
	}
	if severity := h.getQueryParam(r, "severity", ""); severity != "" {
		filters["severity"] = severity
	}
	if resource := h.getQueryParam(r, "resource", ""); resource != "" {
		filters["resource"] = resource
	}
	
	// Time range filters
	if startTime := h.getQueryParam(r, "start_time", ""); startTime != "" {
		t, err := time.Parse(time.RFC3339, startTime)
		if err != nil {
			return nil, fmt.Errorf("invalid start_time format: %w", err)
		}
		filters["start_time"] = t
	}
	if endTime := h.getQueryParam(r, "end_time", ""); endTime != "" {
		t, err := time.Parse(time.RFC3339, endTime)
		if err != nil {
			return nil, fmt.Errorf("invalid end_time format: %w", err)
		}
		filters["end_time"] = t
	}
	
	// Status and outcome filters
	if status := h.getQueryParam(r, "status", ""); status != "" {
		filters["status"] = status
	}
	if outcome := h.getQueryParam(r, "outcome", ""); outcome != "" {
		filters["outcome"] = outcome
	}
	
	// IP address and location filters
	if ipAddress := h.getQueryParam(r, "ip_address", ""); ipAddress != "" {
		filters["ip_address"] = ipAddress
	}
	
	return filters, nil
}

func (h *AuditHandler) parsePagination(r *http.Request) (PaginationRequest, error) {
	page := h.getQueryParamInt(r, "page", 1)
	pageSize := h.getQueryParamInt(r, "page_size", 50)
	
	// Enforce limits
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 50
	}
	if pageSize > 500 { // Audit queries can be resource intensive
		pageSize = 500
	}
	
	return PaginationRequest{
		Page:     page,
		PageSize: pageSize,
	}, nil
}

func (h *AuditHandler) parseSearchRequest(r *http.Request) (*AdvancedSearchRequest, error) {
	req := &AdvancedSearchRequest{
		Query:     h.getQueryParam(r, "q", ""),
		Fields:    h.getQueryParams(r, "fields"),
		Filters:   make(map[string]interface{}),
		Facets:    h.getQueryParams(r, "facets"),
		Highlight: h.getQueryParam(r, "highlight", "true") == "true",
	}
	
	// Parse pagination
	pagination, err := h.parsePagination(r)
	if err != nil {
		return nil, err
	}
	req.Pagination = pagination
	
	// Parse filters
	filters, err := h.parseEventFilters(r)
	if err != nil {
		return nil, err
	}
	req.Filters = filters
	
	// Parse time range
	timeRange, err := h.parseTimeRange(r)
	if err != nil {
		return nil, err
	}
	req.TimeRange = timeRange
	
	return req, nil
}

func (h *AuditHandler) parseTimeRange(r *http.Request) (*TimeRange, error) {
	startStr := h.getQueryParam(r, "start_date", "")
	endStr := h.getQueryParam(r, "end_date", "")
	
	if startStr == "" && endStr == "" {
		// Default to last 24 hours
		end := time.Now()
		start := end.Add(-24 * time.Hour)
		return &TimeRange{
			Start: start,
			End:   end,
		}, nil
	}
	
	if startStr == "" || endStr == "" {
		return nil, fmt.Errorf("both start_date and end_date must be provided")
	}
	
	start, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		return nil, fmt.Errorf("invalid start_date format: %w", err)
	}
	
	end, err := time.Parse(time.RFC3339, endStr)
	if err != nil {
		return nil, fmt.Errorf("invalid end_date format: %w", err)
	}
	
	if start.After(end) {
		return nil, fmt.Errorf("start_date must be before end_date")
	}
	
	// Enforce maximum time range (90 days)
	if end.Sub(start) > 90*24*time.Hour {
		return nil, fmt.Errorf("time range cannot exceed 90 days")
	}
	
	return &TimeRange{
		Start: start,
		End:   end,
	}, nil
}

func (h *AuditHandler) parseExportOptions(r *http.Request, reportType string) (*ExportOptions, error) {
	options := &ExportOptions{
		Format:          ExportFormat(h.getQueryParam(r, "format", "json")),
		ReportType:      ReportType(reportType),
		RedactPII:       h.getQueryParam(r, "redact_pii", "true") == "true",
		IncludeMetadata: h.getQueryParam(r, "include_metadata", "true") == "true",
		ChunkSize:       h.getQueryParamInt(r, "chunk_size", 1000),
	}
	
	// Parse filters
	filters, err := h.parseEventFilters(r)
	if err != nil {
		return nil, err
	}
	options.Filters = filters
	
	// Parse time range
	timeRange, err := h.parseTimeRange(r)
	if err != nil {
		return nil, err
	}
	options.TimeRange = timeRange
	
	// Parse custom template if provided
	if template := h.getQueryParam(r, "template", ""); template != "" {
		options.CustomTemplate = template
	}
	
	return options, nil
}

func (h *AuditHandler) parseUUIDParam(r *http.Request, paramName string) (uuid.UUID, error) {
	value := r.PathValue(paramName)
	if value == "" {
		return uuid.Nil, fmt.Errorf("missing %s parameter", paramName)
	}

	parsed, err := uuid.Parse(value)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid %s format: %w", paramName, err)
	}

	return parsed, nil
}

func (h *AuditHandler) getQueryParam(r *http.Request, key, defaultValue string) string {
	if value := r.URL.Query().Get(key); value != "" {
		return value
	}
	return defaultValue
}

func (h *AuditHandler) getQueryParamInt(r *http.Request, key string, defaultValue int) int {
	if value := r.URL.Query().Get(key); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

func (h *AuditHandler) getQueryParams(r *http.Request, key string) []string {
	return r.URL.Query()[key]
}

func (h *AuditHandler) getRequestMeta(ctx context.Context) *RequestMeta {
	if meta, ok := ctx.Value(contextKeyRequestMeta).(*RequestMeta); ok {
		return meta
	}
	return &RequestMeta{RequestID: uuid.New().String()}
}

func (h *AuditHandler) handleServiceError(err error) error {
	// Convert service errors to API errors
	switch e := err.(type) {
	case *errors.AppError:
		return e
	default:
		// Log internal errors but don't expose details
		h.logger.Error("internal audit service error", "error", err)
		return errors.NewInternalError("an internal error occurred")
	}
}

func (h *AuditHandler) formatValidationError(err error) error {
	// Reuse the existing validation error formatting from BaseHandler
	return h.BaseHandler.formatValidationError(err)
}

// Response conversion helpers

func (h *AuditHandler) convertEventsToResponse(events []AuditEvent) []AuditEventResponse {
	responses := make([]AuditEventResponse, len(events))
	for i, event := range events {
		responses[i] = h.convertEventToResponse(&event)
	}
	return responses
}

func (h *AuditHandler) convertEventToResponse(event *AuditEvent) AuditEventResponse {
	return AuditEventResponse{
		ID:         event.ID,
		EventType:  event.EventType,
		Actor:      event.Actor,
		Resource:   event.Resource,
		Action:     event.Action,
		Outcome:    event.Outcome,
		Severity:   event.Severity,
		IPAddress:  event.IPAddress,
		UserAgent:  event.UserAgent,
		Timestamp:  event.Timestamp,
		Data:       event.Data,
		Metadata:   event.Metadata,
	}
}

func (h *AuditHandler) convertPaginationToResponse(pagination PaginationResponse) PaginationResponse {
	return PaginationResponse{
		Page:       pagination.Page,
		PageSize:   pagination.PageSize,
		TotalPages: pagination.TotalPages,
		TotalItems: pagination.TotalItems,
		HasNext:    pagination.HasNext,
		HasPrev:    pagination.HasPrev,
	}
}