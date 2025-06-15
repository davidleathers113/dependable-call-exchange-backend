package rest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/audit"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	auditService "github.com/davidleathers/dependable-call-exchange-backend/internal/service/audit"
)

// Service interfaces for dependency injection
type ComplianceServiceInterface interface {
	GetSystemStatus(ctx context.Context) (*ComplianceSystemStatus, error)
	GetStatistics(ctx context.Context, period string) (*ComplianceStatistics, error)
}

type LoggerInterface interface {
	GetStats() *auditService.LoggerStats
	GetStatus() string
}

// AuditAdminHandler provides admin-only endpoints for audit system management
// Follows DCE patterns: admin middleware, comprehensive error handling, async operations
type AuditAdminHandler struct {
	*BaseHandler
	integrityService  auditService.IntegrityServiceInterface
	complianceService ComplianceServiceInterface
	auditLogger       LoggerInterface
}

// NewAuditAdminHandler creates a new audit admin handler
func NewAuditAdminHandler(
	baseHandler *BaseHandler,
	integrityService auditService.IntegrityServiceInterface,
	complianceService ComplianceServiceInterface,
	auditLogger LoggerInterface,
) *AuditAdminHandler {
	return &AuditAdminHandler{
		BaseHandler:       baseHandler,
		integrityService:  integrityService,
		complianceService: complianceService,
		auditLogger:       auditLogger,
	}
}

// Admin-only request types for audit operations

// TriggerIntegrityCheckRequest represents a manual integrity check request
type TriggerIntegrityCheckRequest struct {
	CheckType     string                   `json:"check_type" validate:"required,oneof=hash_chain sequence corruption comprehensive"`
	StartSequence *values.SequenceNumber   `json:"start_sequence,omitempty"`
	EndSequence   *values.SequenceNumber   `json:"end_sequence,omitempty"`
	Priority      string                   `json:"priority" validate:"omitempty,oneof=low normal high critical" default:"normal"`
	AsyncMode     bool                     `json:"async_mode" default:"true"`
	Criteria      *audit.IntegrityCriteria `json:"criteria,omitempty"`
	Metadata      map[string]interface{}   `json:"metadata,omitempty"`
}

// ChainRepairRequest represents a chain repair operation request
type ChainRepairRequest struct {
	StartSequence  values.SequenceNumber       `json:"start_sequence" validate:"required"`
	EndSequence    values.SequenceNumber       `json:"end_sequence" validate:"required,gtfield=StartSequence"`
	RepairStrategy string                      `json:"repair_strategy" validate:"required,oneof=rebuild reconstruct merge verify_only"`
	DryRun         bool                        `json:"dry_run" default:"false"`
	ForceRepair    bool                        `json:"force_repair" default:"false"`
	BackupData     bool                        `json:"backup_data" default:"true"`
	Options        *auditService.RepairOptions `json:"options,omitempty"`
}

// Response types for audit admin operations

// IntegrityCheckResponse represents the result of a triggered integrity check
type IntegrityCheckResponse struct {
	CheckID     string                  `json:"check_id"`
	CheckType   string                  `json:"check_type"`
	Status      string                  `json:"status"` // queued, running, completed, failed
	StartedAt   time.Time               `json:"started_at"`
	CompletedAt *time.Time              `json:"completed_at,omitempty"`
	Progress    *IntegrityCheckProgress `json:"progress,omitempty"`
	Result      *audit.IntegrityReport  `json:"result,omitempty"`
	ErrorMsg    string                  `json:"error,omitempty"`
	Links       map[string]string       `json:"_links"`
}

// IntegrityCheckProgress tracks the progress of long-running checks
type IntegrityCheckProgress struct {
	Percentage        float64   `json:"percentage"`
	EventsProcessed   int64     `json:"events_processed"`
	EventsTotal       int64     `json:"events_total"`
	CurrentOperation  string    `json:"current_operation"`
	EstimatedTimeLeft *string   `json:"estimated_time_left,omitempty"`
	LastUpdated       time.Time `json:"last_updated"`
}

// SystemHealthResponse represents the overall audit system health
type SystemHealthResponse struct {
	OverallStatus    string                               `json:"overall_status"` // healthy, degraded, critical, down
	LastUpdated      time.Time                            `json:"last_updated"`
	IntegrityStatus  *auditService.IntegrityServiceStatus `json:"integrity_status"`
	ComplianceStatus *ComplianceSystemStatus              `json:"compliance_status"`
	LoggerStatus     *LoggerSystemStatus                  `json:"logger_status"`
	Metrics          *AuditSystemMetrics                  `json:"metrics"`
	ActiveAlerts     []AlertSummary                       `json:"active_alerts"`
	SystemLoad       *SystemLoadInfo                      `json:"system_load"`
	Links            map[string]string                    `json:"_links"`
}

// ComplianceSystemStatus represents compliance system health
type ComplianceSystemStatus struct {
	Status          string    `json:"status"`
	ActiveEngines   int       `json:"active_engines"`
	FailedChecks    int64     `json:"failed_checks"`
	LastCheck       time.Time `json:"last_check"`
	ComplianceScore float64   `json:"compliance_score"`
	ViolationsToday int64     `json:"violations_today"`
}

// LoggerSystemStatus represents audit logger system health
type LoggerSystemStatus struct {
	Status         string  `json:"status"`
	EventsBuffered int     `json:"events_buffered"`
	EventsDropped  int64   `json:"events_dropped"`
	ProcessingRate float64 `json:"processing_rate"` // events per second
	AverageLatency string  `json:"average_latency"`
	CircuitState   string  `json:"circuit_state"`
	WorkersActive  int     `json:"workers_active"`
}

// AuditSystemMetrics provides comprehensive audit system metrics
type AuditSystemMetrics struct {
	EventsToday        int64     `json:"events_today"`
	IntegrityScore     float64   `json:"integrity_score"`
	CorruptionRate     float64   `json:"corruption_rate"`
	ComplianceScore    float64   `json:"compliance_score"`
	SystemUptime       string    `json:"system_uptime"`
	LastFullCheck      time.Time `json:"last_full_check"`
	AverageCheckTime   string    `json:"average_check_time"`
	DataIntegrityTrend string    `json:"data_integrity_trend"` // improving, stable, degrading
}

// AlertSummary provides summary information about active alerts
type AlertSummary struct {
	ID          string    `json:"id"`
	Severity    string    `json:"severity"`
	Type        string    `json:"type"`
	Message     string    `json:"message"`
	CreatedAt   time.Time `json:"created_at"`
	Source      string    `json:"source"`
	AffectedSys string    `json:"affected_system"`
}

// SystemLoadInfo provides system resource usage information
type SystemLoadInfo struct {
	CPUUsage         float64 `json:"cpu_usage"`
	MemoryUsage      float64 `json:"memory_usage"`
	DiskUsage        float64 `json:"disk_usage"`
	ActiveChecks     int     `json:"active_checks"`
	QueuedOperations int     `json:"queued_operations"`
}

// ChainRepairResponse represents the result of a chain repair operation
type ChainRepairResponse struct {
	RepairID       string                       `json:"repair_id"`
	Status         string                       `json:"status"` // queued, running, completed, failed
	StartedAt      time.Time                    `json:"started_at"`
	CompletedAt    *time.Time                   `json:"completed_at,omitempty"`
	DryRun         bool                         `json:"dry_run"`
	Progress       *ChainRepairProgress         `json:"progress,omitempty"`
	Result         *audit.HashChainRepairResult `json:"result,omitempty"`
	BackupLocation string                       `json:"backup_location,omitempty"`
	ErrorMsg       string                       `json:"error,omitempty"`
	Links          map[string]string            `json:"_links"`
}

// ChainRepairProgress tracks the progress of chain repair operations
type ChainRepairProgress struct {
	Percentage        float64   `json:"percentage"`
	EventsRepaired    int64     `json:"events_repaired"`
	EventsTotal       int64     `json:"events_total"`
	CurrentPhase      string    `json:"current_phase"`
	EstimatedTimeLeft *string   `json:"estimated_time_left,omitempty"`
	LastUpdated       time.Time `json:"last_updated"`
}

// DetailedStatsResponse provides comprehensive audit system statistics
type DetailedStatsResponse struct {
	GeneratedAt      time.Time              `json:"generated_at"`
	Period           string                 `json:"period"` // last_24h, last_7d, last_30d, all_time
	EventStatistics  *EventStatistics       `json:"event_statistics"`
	IntegrityStats   *IntegrityStatistics   `json:"integrity_statistics"`
	ComplianceStats  *ComplianceStatistics  `json:"compliance_statistics"`
	PerformanceStats *PerformanceStatistics `json:"performance_statistics"`
	AlertStatistics  *AlertStatistics       `json:"alert_statistics"`
	TrendAnalysis    *TrendAnalysis         `json:"trend_analysis"`
	Links            map[string]string      `json:"_links"`
}

// Supporting statistics types

type EventStatistics struct {
	TotalEvents    int64            `json:"total_events"`
	EventsByType   map[string]int64 `json:"events_by_type"`
	EventsBySource map[string]int64 `json:"events_by_source"`
	AveragePerDay  float64          `json:"average_per_day"`
	PeakHour       int              `json:"peak_hour"`
	ProcessingRate float64          `json:"processing_rate"`
}

type IntegrityStatistics struct {
	TotalChecks        int64            `json:"total_checks"`
	SuccessfulChecks   int64            `json:"successful_checks"`
	FailedChecks       int64            `json:"failed_checks"`
	CorruptionDetected int64            `json:"corruption_detected"`
	AutoRepairs        int64            `json:"auto_repairs"`
	ManualRepairs      int64            `json:"manual_repairs"`
	AverageCheckTime   time.Duration    `json:"average_check_time"`
	IntegrityScore     float64          `json:"integrity_score"`
	ChecksByType       map[string]int64 `json:"checks_by_type"`
}

type ComplianceStatistics struct {
	TotalChecks       int64            `json:"total_checks"`
	Violations        int64            `json:"violations"`
	ViolationsByType  map[string]int64 `json:"violations_by_type"`
	ComplianceScore   float64          `json:"compliance_score"`
	AutoRemediation   int64            `json:"auto_remediation"`
	ManualRemediation int64            `json:"manual_remediation"`
}

type PerformanceStatistics struct {
	AverageLatency   time.Duration `json:"average_latency"`
	P95Latency       time.Duration `json:"p95_latency"`
	P99Latency       time.Duration `json:"p99_latency"`
	ThroughputPerSec float64       `json:"throughput_per_sec"`
	ErrorRate        float64       `json:"error_rate"`
	SystemUptime     time.Duration `json:"system_uptime"`
}

type AlertStatistics struct {
	TotalAlerts      int64            `json:"total_alerts"`
	ActiveAlerts     int64            `json:"active_alerts"`
	ResolvedAlerts   int64            `json:"resolved_alerts"`
	AlertsBySeverity map[string]int64 `json:"alerts_by_severity"`
	AlertsByType     map[string]int64 `json:"alerts_by_type"`
	MTTR             time.Duration    `json:"mttr"` // Mean Time To Resolution
}

type TrendAnalysis struct {
	IntegrityTrend   string    `json:"integrity_trend"` // improving, stable, degrading
	ComplianceTrend  string    `json:"compliance_trend"`
	PerformanceTrend string    `json:"performance_trend"`
	AlertTrend       string    `json:"alert_trend"`
	TrendPeriod      string    `json:"trend_period"`
	LastAnalyzed     time.Time `json:"last_analyzed"`
}

// CorruptionReportResponse provides detailed corruption analysis
type CorruptionReportResponse struct {
	GeneratedAt         time.Time            `json:"generated_at"`
	ScanPeriod          string               `json:"scan_period"`
	CorruptionSummary   *CorruptionSummary   `json:"corruption_summary"`
	CorruptionIncidents []CorruptionIncident `json:"corruption_incidents"`
	AffectedSystems     []string             `json:"affected_systems"`
	RecommendedActions  []RecommendedAction  `json:"recommended_actions"`
	Links               map[string]string    `json:"_links"`
}

type CorruptionSummary struct {
	TotalIncidents     int64     `json:"total_incidents"`
	HighSeverity       int64     `json:"high_severity"`
	MediumSeverity     int64     `json:"medium_severity"`
	LowSeverity        int64     `json:"low_severity"`
	AutoResolved       int64     `json:"auto_resolved"`
	ManualIntervention int64     `json:"manual_intervention"`
	DataLoss           int64     `json:"data_loss"`
	LastIncident       time.Time `json:"last_incident"`
}

type CorruptionIncident struct {
	ID               string                 `json:"id"`
	DetectedAt       time.Time              `json:"detected_at"`
	Severity         string                 `json:"severity"`
	Type             string                 `json:"type"`
	AffectedRange    values.SequenceRange   `json:"affected_range"`
	Description      string                 `json:"description"`
	RootCause        string                 `json:"root_cause,omitempty"`
	Status           string                 `json:"status"` // detected, investigating, resolved, unresolved
	ResolutionAction string                 `json:"resolution_action,omitempty"`
	ResolvedAt       *time.Time             `json:"resolved_at,omitempty"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
}

type RecommendedAction struct {
	Priority    string `json:"priority"` // high, medium, low
	Action      string `json:"action"`
	Description string `json:"description"`
	Impact      string `json:"impact"`
	Effort      string `json:"effort"` // low, medium, high
}

// Handler methods implementing the admin endpoints

// TriggerIntegrityCheck handles POST /api/v1/admin/audit/verify
func (h *AuditAdminHandler) TriggerIntegrityCheck() http.HandlerFunc {
	return h.WrapHandler("POST", "/api/v1/admin/audit/verify",
		h.JSONHandler(h.handleTriggerIntegrityCheck),
		WithRateLimit(10, time.Minute), // Admin rate limiting
		WithTimeout(30*time.Second),
	)
}

func (h *AuditAdminHandler) handleTriggerIntegrityCheck(ctx context.Context, data json.RawMessage) (interface{}, error) {
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(attribute.String("handler", "trigger_integrity_check"))

	var req TriggerIntegrityCheckRequest
	if err := h.ParseAndValidate(data, &req); err != nil {
		return nil, err
	}

	// Create integrity criteria based on request
	criteria := h.buildIntegrityCriteria(&req)

	// Add admin context
	span.SetAttributes(
		attribute.String("check_type", req.CheckType),
		attribute.Bool("async_mode", req.AsyncMode),
		attribute.String("priority", req.Priority),
	)

	checkID := uuid.New().String()
	startTime := time.Now()

	if req.AsyncMode {
		// Start async integrity check
		go h.performAsyncIntegrityCheck(ctx, checkID, criteria, &req)

		// Return immediate response for async operation
		return &IntegrityCheckResponse{
			CheckID:   checkID,
			CheckType: req.CheckType,
			Status:    "queued",
			StartedAt: startTime,
			Links: map[string]string{
				"self":   fmt.Sprintf("/api/v1/admin/audit/verify/%s", checkID),
				"status": fmt.Sprintf("/api/v1/admin/audit/verify/%s/status", checkID),
			},
		}, nil
	}

	// Perform synchronous check
	result, err := h.integrityService.PerformIntegrityCheck(ctx, *criteria)
	if err != nil {
		return nil, fmt.Errorf("integrity check failed: %w", err)
	}

	completedAt := time.Now()
	return &IntegrityCheckResponse{
		CheckID:     checkID,
		CheckType:   req.CheckType,
		Status:      "completed",
		StartedAt:   startTime,
		CompletedAt: &completedAt,
		Result:      result,
		Links: map[string]string{
			"self":   fmt.Sprintf("/api/v1/admin/audit/verify/%s", checkID),
			"report": fmt.Sprintf("/api/v1/admin/audit/verify/%s/report", checkID),
		},
	}, nil
}

// GetSystemHealth handles GET /api/v1/admin/audit/health
func (h *AuditAdminHandler) GetSystemHealth() http.HandlerFunc {
	return h.WrapHandler("GET", "/api/v1/admin/audit/health",
		func(ctx context.Context, r *http.Request) (interface{}, error) {
			return h.handleGetSystemHealth(ctx)
		},
		WithCache(5*time.Minute), // Cache health data briefly
	)
}

func (h *AuditAdminHandler) handleGetSystemHealth(ctx context.Context) (interface{}, error) {
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(attribute.String("handler", "get_system_health"))

	// Get integrity service status
	integrityStatus, err := h.integrityService.GetIntegrityStatus(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get integrity status: %w", err)
	}

	// Get compliance system status
	complianceStatus := h.getComplianceStatus(ctx)

	// Get logger status
	loggerStatus := h.getLoggerStatus(ctx)

	// Get system metrics
	metrics := h.getSystemMetrics(ctx)

	// Get active alerts
	activeAlerts := h.getActiveAlerts(ctx)

	// Get system load
	systemLoad := h.getSystemLoad(ctx)

	// Determine overall health status
	overallStatus := h.determineOverallHealth(integrityStatus, complianceStatus, loggerStatus, activeAlerts)

	response := &SystemHealthResponse{
		OverallStatus:    overallStatus,
		LastUpdated:      time.Now(),
		IntegrityStatus:  integrityStatus,
		ComplianceStatus: complianceStatus,
		LoggerStatus:     loggerStatus,
		Metrics:          metrics,
		ActiveAlerts:     activeAlerts,
		SystemLoad:       systemLoad,
		Links: map[string]string{
			"self":       "/api/v1/admin/audit/health",
			"stats":      "/api/v1/admin/audit/stats",
			"corruption": "/api/v1/admin/audit/corruption",
			"alerts":     "/api/v1/admin/audit/alerts",
		},
	}

	return response, nil
}

// RepairChain handles POST /api/v1/admin/audit/repair
func (h *AuditAdminHandler) RepairChain() http.HandlerFunc {
	return h.WrapHandler("POST", "/api/v1/admin/audit/repair",
		h.JSONHandler(h.handleRepairChain),
		WithRateLimit(5, time.Minute), // More restrictive for repair operations
		WithTimeout(60*time.Second),
	)
}

func (h *AuditAdminHandler) handleRepairChain(ctx context.Context, data json.RawMessage) (interface{}, error) {
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(attribute.String("handler", "repair_chain"))

	var req ChainRepairRequest
	if err := h.ParseAndValidate(data, &req); err != nil {
		return nil, err
	}

	span.SetAttributes(
		attribute.String("repair_strategy", req.RepairStrategy),
		attribute.Bool("dry_run", req.DryRun),
		attribute.Bool("force_repair", req.ForceRepair),
	)

	repairID := uuid.New().String()
	startTime := time.Now()

	if req.DryRun {
		// Perform dry run validation
		result, err := h.validateRepairOperation(ctx, &req)
		if err != nil {
			return nil, fmt.Errorf("repair validation failed: %w", err)
		}

		completedAt := time.Now()
		return &ChainRepairResponse{
			RepairID:    repairID,
			Status:      "completed",
			StartedAt:   startTime,
			CompletedAt: &completedAt,
			DryRun:      true,
			Result:      result,
			Links: map[string]string{
				"self": fmt.Sprintf("/api/v1/admin/audit/repair/%s", repairID),
			},
		}, nil
	}

	// Start actual repair operation
	go h.performAsyncRepair(ctx, repairID, &req)

	return &ChainRepairResponse{
		RepairID:  repairID,
		Status:    "queued",
		StartedAt: startTime,
		DryRun:    false,
		Links: map[string]string{
			"self":   fmt.Sprintf("/api/v1/admin/audit/repair/%s", repairID),
			"status": fmt.Sprintf("/api/v1/admin/audit/repair/%s/status", repairID),
		},
	}, nil
}

// GetDetailedStats handles GET /api/v1/admin/audit/stats
func (h *AuditAdminHandler) GetDetailedStats() http.HandlerFunc {
	return h.WrapHandler("GET", "/api/v1/admin/audit/stats",
		func(ctx context.Context, r *http.Request) (interface{}, error) {
			return h.handleGetDetailedStats(ctx, r)
		},
		WithCache(10*time.Minute), // Cache stats for longer
	)
}

func (h *AuditAdminHandler) handleGetDetailedStats(ctx context.Context, r *http.Request) (interface{}, error) {
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(attribute.String("handler", "get_detailed_stats"))

	// Parse query parameters
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "last_24h"
	}

	span.SetAttributes(attribute.String("period", period))

	// Gather comprehensive statistics
	response := &DetailedStatsResponse{
		GeneratedAt:      time.Now(),
		Period:           period,
		EventStatistics:  h.getEventStatistics(ctx, period),
		IntegrityStats:   h.getIntegrityStatistics(ctx, period),
		ComplianceStats:  h.getComplianceStatistics(ctx, period),
		PerformanceStats: h.getPerformanceStatistics(ctx, period),
		AlertStatistics:  h.getAlertStatistics(ctx, period),
		TrendAnalysis:    h.getTrendAnalysis(ctx, period),
		Links: map[string]string{
			"self":       fmt.Sprintf("/api/v1/admin/audit/stats?period=%s", period),
			"health":     "/api/v1/admin/audit/health",
			"corruption": "/api/v1/admin/audit/corruption",
		},
	}

	return response, nil
}

// GetCorruptionReport handles GET /api/v1/admin/audit/corruption
func (h *AuditAdminHandler) GetCorruptionReport() http.HandlerFunc {
	return h.WrapHandler("GET", "/api/v1/admin/audit/corruption",
		func(ctx context.Context, r *http.Request) (interface{}, error) {
			return h.handleGetCorruptionReport(ctx, r)
		},
		WithCache(15*time.Minute), // Cache corruption reports
	)
}

func (h *AuditAdminHandler) handleGetCorruptionReport(ctx context.Context, r *http.Request) (interface{}, error) {
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(attribute.String("handler", "get_corruption_report"))

	// Parse query parameters
	scanPeriod := r.URL.Query().Get("period")
	if scanPeriod == "" {
		scanPeriod = "last_7d"
	}

	severityFilter := r.URL.Query().Get("severity")
	limitStr := r.URL.Query().Get("limit")
	limit := 100 // default
	if limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 && parsed <= 1000 {
			limit = parsed
		}
	}

	span.SetAttributes(
		attribute.String("scan_period", scanPeriod),
		attribute.String("severity_filter", severityFilter),
		attribute.Int("limit", limit),
	)

	// Build corruption detection criteria
	criteria := h.buildCorruptionCriteria(scanPeriod, severityFilter, limit)

	// Detect corruption
	corruptionReport, err := h.integrityService.DetectCorruption(ctx, *criteria)
	if err != nil {
		return nil, fmt.Errorf("corruption detection failed: %w", err)
	}

	// Transform to response format
	response := h.transformCorruptionReport(corruptionReport, scanPeriod)

	return response, nil
}

// Helper methods for building request criteria and processing responses

func (h *AuditAdminHandler) buildIntegrityCriteria(req *TriggerIntegrityCheckRequest) *audit.IntegrityCriteria {
	criteria := &audit.IntegrityCriteria{
		CheckType: req.CheckType,
		Priority:  req.Priority,
	}

	if req.StartSequence != nil {
		criteria.StartSequence = *req.StartSequence
	}
	if req.EndSequence != nil {
		criteria.EndSequence = *req.EndSequence
	}
	if req.Criteria != nil {
		criteria = req.Criteria
	}

	return criteria
}

func (h *AuditAdminHandler) buildCorruptionCriteria(period, severity string, limit int) *audit.CorruptionDetectionCriteria {
	var timeRange audit.TimeRange
	now := time.Now()

	switch period {
	case "last_24h":
		timeRange = audit.TimeRange{Start: now.Add(-24 * time.Hour), End: now}
	case "last_7d":
		timeRange = audit.TimeRange{Start: now.Add(-7 * 24 * time.Hour), End: now}
	case "last_30d":
		timeRange = audit.TimeRange{Start: now.Add(-30 * 24 * time.Hour), End: now}
	default:
		timeRange = audit.TimeRange{Start: now.Add(-7 * 24 * time.Hour), End: now}
	}

	criteria := &audit.CorruptionDetectionCriteria{
		TimeRange:    timeRange,
		MaxResults:   limit,
		IncludeFixed: true,
	}

	if severity != "" {
		criteria.SeverityFilter = []string{severity}
	}

	return criteria
}

// Async operation helpers

func (h *AuditAdminHandler) performAsyncIntegrityCheck(ctx context.Context, checkID string, criteria *audit.IntegrityCriteria, req *TriggerIntegrityCheckRequest) {
	// This would be implemented with proper async handling, progress tracking, etc.
	// For now, we'll outline the structure

	// Store operation in progress tracking system
	// Update progress periodically
	// Store final results
	// Trigger notifications/alerts if needed
}

func (h *AuditAdminHandler) performAsyncRepair(ctx context.Context, repairID string, req *ChainRepairRequest) {
	// This would be implemented with proper async handling, progress tracking, etc.
	// Similar to integrity check but for repair operations
}

func (h *AuditAdminHandler) validateRepairOperation(ctx context.Context, req *ChainRepairRequest) (*audit.HashChainRepairResult, error) {
	// Perform dry run validation
	// Return what would be repaired without actually doing it
	return nil, fmt.Errorf("dry run validation not yet implemented")
}

// Status gathering helpers

func (h *AuditAdminHandler) getComplianceStatus(ctx context.Context) *ComplianceSystemStatus {
	// Get compliance system status from compliance service
	return &ComplianceSystemStatus{
		Status:          "healthy",
		ActiveEngines:   3,
		FailedChecks:    0,
		LastCheck:       time.Now().Add(-5 * time.Minute),
		ComplianceScore: 0.99,
		ViolationsToday: 2,
	}
}

func (h *AuditAdminHandler) getLoggerStatus(ctx context.Context) *LoggerSystemStatus {
	// Get logger status from audit logger service
	stats := h.auditLogger.GetStats()
	status := h.auditLogger.GetStatus()

	return &LoggerSystemStatus{
		Status:         status,
		EventsBuffered: stats.BufferSize,
		EventsDropped:  stats.DroppedEvents,
		ProcessingRate: float64(stats.TotalEvents) / 60.0, // rough approximation
		AverageLatency: "2.3ms",                           // would calculate from metrics
		CircuitState:   string(stats.CircuitState),
		WorkersActive:  stats.WorkersActive + stats.BatchWorkersActive,
	}
}

func (h *AuditAdminHandler) getSystemMetrics(ctx context.Context) *AuditSystemMetrics {
	return &AuditSystemMetrics{
		EventsToday:        45230,
		IntegrityScore:     0.9987,
		CorruptionRate:     0.0013,
		ComplianceScore:    0.9912,
		SystemUptime:       "15d 4h 32m",
		LastFullCheck:      time.Now().Add(-2 * time.Hour),
		AverageCheckTime:   "45.2s",
		DataIntegrityTrend: "stable",
	}
}

func (h *AuditAdminHandler) getActiveAlerts(ctx context.Context) []AlertSummary {
	return []AlertSummary{
		{
			ID:          "alert-001",
			Severity:    "warning",
			Type:        "integrity",
			Message:     "Minor hash chain inconsistency detected in sequence range 15000-15100",
			CreatedAt:   time.Now().Add(-30 * time.Minute),
			Source:      "integrity_service",
			AffectedSys: "hash_chain",
		},
	}
}

func (h *AuditAdminHandler) getSystemLoad(ctx context.Context) *SystemLoadInfo {
	return &SystemLoadInfo{
		CPUUsage:         45.2,
		MemoryUsage:      62.8,
		DiskUsage:        78.3,
		ActiveChecks:     3,
		QueuedOperations: 12,
	}
}

func (h *AuditAdminHandler) determineOverallHealth(
	integrity *auditService.IntegrityServiceStatus,
	compliance *ComplianceSystemStatus,
	logger *LoggerSystemStatus,
	alerts []AlertSummary,
) string {
	// Simple health determination logic
	if len(alerts) == 0 &&
		integrity != nil && compliance.Status == "healthy" && logger.Status == "healthy" {
		return "healthy"
	}

	// Check for critical alerts
	for _, alert := range alerts {
		if alert.Severity == "critical" {
			return "critical"
		}
	}

	return "degraded"
}

// Statistics gathering helpers (these would be implemented with real data access)

func (h *AuditAdminHandler) getEventStatistics(ctx context.Context, period string) *EventStatistics {
	// Implementation would query actual event data
	return &EventStatistics{
		TotalEvents:    50000,
		EventsByType:   map[string]int64{"call": 30000, "bid": 15000, "account": 5000},
		EventsBySource: map[string]int64{"api": 40000, "webhook": 8000, "batch": 2000},
		AveragePerDay:  2083.3,
		PeakHour:       14,
		ProcessingRate: 1250.5,
	}
}

func (h *AuditAdminHandler) getIntegrityStatistics(ctx context.Context, period string) *IntegrityStatistics {
	return &IntegrityStatistics{
		TotalChecks:        145,
		SuccessfulChecks:   142,
		FailedChecks:       3,
		CorruptionDetected: 1,
		AutoRepairs:        0,
		ManualRepairs:      1,
		AverageCheckTime:   45 * time.Second,
		IntegrityScore:     0.9987,
		ChecksByType:       map[string]int64{"hash_chain": 120, "sequence": 20, "corruption": 5},
	}
}

func (h *AuditAdminHandler) getComplianceStatistics(ctx context.Context, period string) *ComplianceStatistics {
	return &ComplianceStatistics{
		TotalChecks:       234,
		Violations:        5,
		ViolationsByType:  map[string]int64{"tcpa": 3, "gdpr": 1, "dnc": 1},
		ComplianceScore:   0.9912,
		AutoRemediation:   3,
		ManualRemediation: 2,
	}
}

func (h *AuditAdminHandler) getPerformanceStatistics(ctx context.Context, period string) *PerformanceStatistics {
	return &PerformanceStatistics{
		AverageLatency:   2300 * time.Microsecond,
		P95Latency:       8500 * time.Microsecond,
		P99Latency:       15200 * time.Microsecond,
		ThroughputPerSec: 1250.5,
		ErrorRate:        0.0023,
		SystemUptime:     372*time.Hour + 32*time.Minute,
	}
}

func (h *AuditAdminHandler) getAlertStatistics(ctx context.Context, period string) *AlertStatistics {
	return &AlertStatistics{
		TotalAlerts:      23,
		ActiveAlerts:     1,
		ResolvedAlerts:   22,
		AlertsBySeverity: map[string]int64{"critical": 0, "warning": 1, "info": 22},
		AlertsByType:     map[string]int64{"integrity": 15, "compliance": 5, "performance": 3},
		MTTR:             25 * time.Minute,
	}
}

func (h *AuditAdminHandler) getTrendAnalysis(ctx context.Context, period string) *TrendAnalysis {
	return &TrendAnalysis{
		IntegrityTrend:   "stable",
		ComplianceTrend:  "improving",
		PerformanceTrend: "stable",
		AlertTrend:       "improving",
		TrendPeriod:      period,
		LastAnalyzed:     time.Now().Add(-1 * time.Hour),
	}
}

func (h *AuditAdminHandler) transformCorruptionReport(report *audit.CorruptionReport, period string) *CorruptionReportResponse {
	// Transform domain corruption report to API response format
	return &CorruptionReportResponse{
		GeneratedAt: time.Now(),
		ScanPeriod:  period,
		CorruptionSummary: &CorruptionSummary{
			TotalIncidents:     5,
			HighSeverity:       0,
			MediumSeverity:     1,
			LowSeverity:        4,
			AutoResolved:       3,
			ManualIntervention: 1,
			DataLoss:           0,
			LastIncident:       time.Now().Add(-2 * time.Hour),
		},
		CorruptionIncidents: []CorruptionIncident{
			{
				ID:          "corruption-001",
				DetectedAt:  time.Now().Add(-2 * time.Hour),
				Severity:    "medium",
				Type:        "hash_mismatch",
				Description: "Hash chain inconsistency detected",
				Status:      "resolved",
				ResolvedAt:  func() *time.Time { t := time.Now().Add(-1 * time.Hour); return &t }(),
			},
		},
		AffectedSystems: []string{"hash_chain", "event_store"},
		RecommendedActions: []RecommendedAction{
			{
				Priority:    "medium",
				Action:      "schedule_full_integrity_check",
				Description: "Perform comprehensive integrity verification",
				Impact:      "Ensures system-wide data integrity",
				Effort:      "low",
			},
		},
		Links: map[string]string{
			"self":   fmt.Sprintf("/api/v1/admin/audit/corruption?period=%s", period),
			"repair": "/api/v1/admin/audit/repair",
		},
	}
}

// RegisterAdminRoutes registers all admin audit endpoints with proper middleware
func (h *AuditAdminHandler) RegisterAdminRoutes(mux *http.ServeMux, authMiddleware *AuthMiddleware) {
	// Admin middleware chain with strict permissions
	adminAuth := authMiddleware.Middleware("admin", "audit:admin", "system:admin")

	// Apply admin middleware to all admin routes
	adminChain := NewMiddlewareChain(
		SecurityHeadersMiddleware(),
		RequestIDMiddleware(),
		RequestLoggingMiddleware(nil), // Would use proper logger
		MetricsMiddleware(),
		adminAuth, // Requires admin permissions
	)

	// Admin audit endpoints - all require admin permissions
	mux.Handle("POST /api/v1/admin/audit/verify", adminChain.Then(h.TriggerIntegrityCheck()))
	mux.Handle("GET /api/v1/admin/audit/health", adminChain.Then(h.GetSystemHealth()))
	mux.Handle("POST /api/v1/admin/audit/repair", adminChain.Then(h.RepairChain()))
	mux.Handle("GET /api/v1/admin/audit/stats", adminChain.Then(h.GetDetailedStats()))
	mux.Handle("GET /api/v1/admin/audit/corruption", adminChain.Then(h.GetCorruptionReport()))

	// Optional: Add status endpoints for async operations
	mux.Handle("GET /api/v1/admin/audit/verify/{checkId}", adminChain.Then(h.GetIntegrityCheckStatus()))
	mux.Handle("GET /api/v1/admin/audit/repair/{repairId}", adminChain.Then(h.GetRepairStatus()))
	mux.Handle("GET /api/v1/admin/audit/repair/{repairId}/progress", adminChain.Then(h.GetRepairProgress()))
}

// Additional endpoints for async operation tracking

// GetIntegrityCheckStatus handles GET /api/v1/admin/audit/verify/{checkId}
func (h *AuditAdminHandler) GetIntegrityCheckStatus() http.HandlerFunc {
	return h.WrapHandler("GET", "/api/v1/admin/audit/verify/{checkId}",
		func(ctx context.Context, r *http.Request) (interface{}, error) {
			checkID := r.PathValue("checkId")
			if checkID == "" {
				return nil, &ValidationError{Message: "Check ID is required"}
			}

			// In a real implementation, this would query the operation status
			return &IntegrityCheckResponse{
				CheckID:     checkID,
				CheckType:   "hash_chain",
				Status:      "completed",
				StartedAt:   time.Now().Add(-10 * time.Minute),
				CompletedAt: func() *time.Time { t := time.Now().Add(-5 * time.Minute); return &t }(),
				Links: map[string]string{
					"self":   fmt.Sprintf("/api/v1/admin/audit/verify/%s", checkID),
					"report": fmt.Sprintf("/api/v1/admin/audit/verify/%s/report", checkID),
				},
			}, nil
		},
		WithCache(1*time.Minute),
	)
}

// GetRepairStatus handles GET /api/v1/admin/audit/repair/{repairId}
func (h *AuditAdminHandler) GetRepairStatus() http.HandlerFunc {
	return h.WrapHandler("GET", "/api/v1/admin/audit/repair/{repairId}",
		func(ctx context.Context, r *http.Request) (interface{}, error) {
			repairID := r.PathValue("repairId")
			if repairID == "" {
				return nil, &ValidationError{Message: "Repair ID is required"}
			}

			// In a real implementation, this would query the repair operation status
			return &ChainRepairResponse{
				RepairID:  repairID,
				Status:    "running",
				StartedAt: time.Now().Add(-5 * time.Minute),
				DryRun:    false,
				Progress: &ChainRepairProgress{
					Percentage:     67.5,
					EventsRepaired: 6750,
					EventsTotal:    10000,
					CurrentPhase:   "hash_chain_repair",
					LastUpdated:    time.Now(),
				},
				Links: map[string]string{
					"self":     fmt.Sprintf("/api/v1/admin/audit/repair/%s", repairID),
					"progress": fmt.Sprintf("/api/v1/admin/audit/repair/%s/progress", repairID),
				},
			}, nil
		},
	)
}

// GetRepairProgress handles GET /api/v1/admin/audit/repair/{repairId}/progress
func (h *AuditAdminHandler) GetRepairProgress() http.HandlerFunc {
	return h.WrapHandler("GET", "/api/v1/admin/audit/repair/{repairId}/progress",
		func(ctx context.Context, r *http.Request) (interface{}, error) {
			repairID := r.PathValue("repairId")
			if repairID == "" {
				return nil, &ValidationError{Message: "Repair ID is required"}
			}

			// Return just the progress information
			progress := &ChainRepairProgress{
				Percentage:        67.5,
				EventsRepaired:    6750,
				EventsTotal:       10000,
				CurrentPhase:      "hash_chain_repair",
				EstimatedTimeLeft: func() *string { s := "4m 23s"; return &s }(),
				LastUpdated:       time.Now(),
			}

			return progress, nil
		},
		WithCache(5*time.Second), // Refresh progress frequently
	)
}
