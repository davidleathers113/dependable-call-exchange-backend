package compliance

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/compliance"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/google/uuid"
)

// ComplianceReporter implements comprehensive compliance monitoring and reporting
type ComplianceReporter struct {
	logger            *zap.Logger
	complianceRepo    ComplianceRepository
	auditService      AuditService
	alertService      AlertService
	certificateService CertificateService
	
	// Reporter configuration
	config            ReporterConfig
	alertRules        map[string]AlertRule
	reportTemplates   map[string]ReportTemplate
	thresholds        map[string]ComplianceThreshold
}

// ReporterConfig holds compliance reporter configuration
type ReporterConfig struct {
	EnableRealTimeMonitoring   bool            `json:"enable_realtime_monitoring"`
	MonitoringInterval         time.Duration   `json:"monitoring_interval"`
	AlertCooldownPeriod        time.Duration   `json:"alert_cooldown_period"`
	TrendAnalysisEnabled       bool            `json:"trend_analysis_enabled"`
	PredictiveAnalyticsEnabled bool            `json:"predictive_analytics_enabled"`
	ReportRetentionDays        int             `json:"report_retention_days"`
	AutoCertificationEnabled   bool            `json:"auto_certification_enabled"`
	ComplianceThresholds       map[string]float64 `json:"compliance_thresholds"`
	NotificationChannels       []NotificationChannel `json:"notification_channels"`
	ExportFormats             []string        `json:"export_formats"`
}

type AlertRule struct {
	RuleID          string                 `json:"rule_id"`
	Name            string                 `json:"name"`
	Description     string                 `json:"description"`
	Regulation      RegulationType         `json:"regulation"`
	MetricType      string                 `json:"metric_type"`
	Threshold       float64                `json:"threshold"`
	Operator        string                 `json:"operator"` // greater_than, less_than, equals
	Severity        compliance.Severity    `json:"severity"`
	CooldownPeriod  time.Duration          `json:"cooldown_period"`
	NotificationChannels []NotificationChannel `json:"notification_channels"`
	Enabled         bool                   `json:"enabled"`
}

type ReportTemplate struct {
	TemplateID      string                 `json:"template_id"`
	Name            string                 `json:"name"`
	Description     string                 `json:"description"`
	ReportType      ComplianceReportType   `json:"report_type"`
	Regulations     []RegulationType       `json:"regulations"`
	Sections        []ReportSection        `json:"sections"`
	Formats         []string               `json:"formats"`
	DefaultPeriod   string                 `json:"default_period"` // daily, weekly, monthly, quarterly, yearly
}

type ReportSection struct {
	SectionID   string                 `json:"section_id"`
	Title       string                 `json:"title"`
	Type        string                 `json:"type"` // summary, chart, table, text
	DataSource  string                 `json:"data_source"`
	Parameters  map[string]interface{} `json:"parameters"`
	Required    bool                   `json:"required"`
}

type ComplianceThreshold struct {
	MetricName      string    `json:"metric_name"`
	MinValue        float64   `json:"min_value"`
	MaxValue        float64   `json:"max_value"`
	TargetValue     float64   `json:"target_value"`
	CriticalValue   float64   `json:"critical_value"`
	Unit            string    `json:"unit"`
	Description     string    `json:"description"`
}

// External service interfaces for reporting
type AlertService interface {
	SendAlert(ctx context.Context, alert ComplianceAlert) error
	GetAlertHistory(ctx context.Context, filters AlertFilters) ([]*AlertHistoryItem, error)
}

type CertificateService interface {
	GenerateCertificate(ctx context.Context, req CertificateRequest) (*ComplianceCertificate, error)
	ValidateCertificate(ctx context.Context, certificateID uuid.UUID) (*CertificateValidationResult, error)
}

// NewComplianceReporter creates a new compliance reporter
func NewComplianceReporter(
	logger *zap.Logger,
	complianceRepo ComplianceRepository,
	auditService AuditService,
	alertService AlertService,
	certificateService CertificateService,
	config ReporterConfig,
) *ComplianceReporter {
	reporter := &ComplianceReporter{
		logger:             logger,
		complianceRepo:     complianceRepo,
		auditService:       auditService,
		alertService:       alertService,
		certificateService: certificateService,
		config:             config,
		alertRules:         initializeAlertRules(),
		reportTemplates:    initializeReportTemplates(),
		thresholds:         initializeComplianceThresholds(),
	}
	
	return reporter
}

// GenerateComplianceReport generates comprehensive compliance reports
func (r *ComplianceReporter) GenerateComplianceReport(ctx context.Context, req ComplianceReportRequest) (*ComplianceReport, error) {
	startTime := time.Now()
	reportID := uuid.New()
	
	r.logger.Info("Generating compliance report",
		zap.String("report_id", reportID.String()),
		zap.String("report_type", string(req.ReportType)),
		zap.Time("start_date", req.StartDate),
		zap.Time("end_date", req.EndDate),
	)

	defer func() {
		r.logger.Debug("Compliance report generation completed",
			zap.String("report_id", reportID.String()),
			zap.Duration("duration", time.Since(startTime)),
		)
	}()

	// Get metrics for the period
	metricsReq := MetricsRequest{
		StartDate:   req.StartDate,
		EndDate:     req.EndDate,
		Regulations: req.Regulations,
		GroupBy:     "day",
	}

	metrics, err := r.complianceRepo.GetComplianceMetrics(ctx, metricsReq)
	if err != nil {
		r.logger.Error("Failed to get compliance metrics",
			zap.String("report_id", reportID.String()),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to get metrics: %w", err)
	}

	// Get violations for the period
	violationFilters := ViolationFilters{
		StartDate: &req.StartDate,
		EndDate:   &req.EndDate,
		Limit:     1000,
	}

	violations, err := r.complianceRepo.GetViolations(ctx, violationFilters)
	if err != nil {
		r.logger.Error("Failed to get violations",
			zap.String("report_id", reportID.String()),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to get violations: %w", err)
	}

	// Create report summary
	summary := r.createComplianceSummary(metrics, violations)

	// Create detailed report based on type
	var details interface{}
	switch req.ReportType {
	case ReportTypeViolations:
		details = r.createViolationsReport(violations)
	case ReportTypeConsent:
		details = r.createConsentReport(ctx, req)
	case ReportTypeAudit:
		details = r.createAuditReport(ctx, req)
	case ReportTypeTrends:
		details = r.createTrendsReport(ctx, req, metrics)
	case ReportTypeCertification:
		details = r.createCertificationReport(ctx, req, summary)
	default:
		details = map[string]interface{}{
			"metrics": metrics,
			"violations": violations,
		}
	}

	report := &ComplianceReport{
		ReportID:    reportID,
		ReportType:  req.ReportType,
		GeneratedAt: time.Now(),
		Period: ReportPeriod{
			StartDate: req.StartDate,
			EndDate:   req.EndDate,
		},
		Summary: summary,
		Details: details,
		Metadata: map[string]interface{}{
			"generation_time_ms": time.Since(startTime).Milliseconds(),
			"scope":              req.Scope,
			"format":             req.Format,
			"include_raw_data":   req.IncludeRaw,
			"regulations":        req.Regulations,
		},
	}

	// Log audit event
	auditEvent := ComplianceAuditEvent{
		EventType:   "compliance_report_generated",
		CallID:      uuid.New(),
		PhoneNumber: values.PhoneNumber{}, // No specific phone number
		Regulation:  RegulationType("all"),
		Result:      fmt.Sprintf("report_type=%s,violations=%d", req.ReportType, len(violations)),
		Timestamp:   time.Now(),
		ActorID:     uuid.New(),
		Metadata: map[string]interface{}{
			"report_id":       reportID.String(),
			"report_type":     req.ReportType,
			"period_days":     req.EndDate.Sub(req.StartDate).Hours() / 24,
			"violations_count": len(violations),
			"compliance_rate": summary.ComplianceRate,
		},
	}

	if err := r.auditService.LogComplianceEvent(ctx, auditEvent); err != nil {
		r.logger.Error("Failed to log report generation audit event",
			zap.Error(err),
		)
	}

	return report, nil
}

// DetectViolations detects compliance violations based on filters
func (r *ComplianceReporter) DetectViolations(ctx context.Context, filters ViolationFilters) ([]*compliance.ComplianceViolation, error) {
	startTime := time.Now()
	
	r.logger.Debug("Detecting compliance violations",
		zap.Any("filters", filters),
	)

	violations, err := r.complianceRepo.GetViolations(ctx, filters)
	if err != nil {
		r.logger.Error("Failed to detect violations",
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to get violations: %w", err)
	}

	// Apply real-time analysis and enrichment
	enrichedViolations := make([]*compliance.ComplianceViolation, 0, len(violations))
	for _, violation := range violations {
		// Enrich violation with additional context
		enrichedViolation := r.enrichViolation(ctx, violation)
		enrichedViolations = append(enrichedViolations, enrichedViolation)
	}

	r.logger.Debug("Violation detection completed",
		zap.Int("violations_found", len(enrichedViolations)),
		zap.Duration("duration", time.Since(startTime)),
	)

	return enrichedViolations, nil
}

// GenerateComplianceCertificate generates compliance certificates
func (r *ComplianceReporter) GenerateComplianceCertificate(ctx context.Context, req CertificateRequest) (*ComplianceCertificate, error) {
	if !r.config.AutoCertificationEnabled {
		return nil, fmt.Errorf("automatic certification is disabled")
	}

	startTime := time.Now()
	
	r.logger.Info("Generating compliance certificate",
		zap.Strings("regulations", []string(req.Regulations)),
		zap.String("scope", req.Scope),
		zap.Time("start_date", req.StartDate),
		zap.Time("end_date", req.EndDate),
	)

	certificate, err := r.certificateService.GenerateCertificate(ctx, req)
	if err != nil {
		r.logger.Error("Failed to generate compliance certificate",
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to generate certificate: %w", err)
	}

	// Log audit event
	auditEvent := ComplianceAuditEvent{
		EventType:   "compliance_certificate_generated",
		CallID:      uuid.New(),
		PhoneNumber: values.PhoneNumber{},
		Regulation:  RegulationType("all"),
		Result:      fmt.Sprintf("certificate_id=%s,status=%s", certificate.CertificateID.String(), certificate.Status),
		Timestamp:   time.Now(),
		ActorID:     uuid.New(),
		Metadata: map[string]interface{}{
			"certificate_id":     certificate.CertificateID.String(),
			"regulations":        req.Regulations,
			"scope":              req.Scope,
			"generation_time_ms": time.Since(startTime).Milliseconds(),
			"findings_count":     len(certificate.Findings),
		},
	}

	if err := r.auditService.LogComplianceEvent(ctx, auditEvent); err != nil {
		r.logger.Error("Failed to log certificate generation audit event",
			zap.Error(err),
		)
	}

	return certificate, nil
}

// MonitorRealTimeCompliance provides real-time compliance monitoring
func (r *ComplianceReporter) MonitorRealTimeCompliance(ctx context.Context, config MonitoringConfig) (<-chan ComplianceAlert, error) {
	if !r.config.EnableRealTimeMonitoring {
		return nil, fmt.Errorf("real-time monitoring is disabled")
	}

	r.logger.Info("Starting real-time compliance monitoring",
		zap.Duration("check_interval", config.CheckInterval),
		zap.Strings("regulations", []string(config.Regulations)),
	)

	alertChan := make(chan ComplianceAlert, 100)

	go r.runMonitoringLoop(ctx, config, alertChan)

	return alertChan, nil
}

// AnalyzeComplianceTrends analyzes compliance trends and patterns
func (r *ComplianceReporter) AnalyzeComplianceTrends(ctx context.Context, req TrendAnalysisRequest) (*ComplianceTrends, error) {
	if !r.config.TrendAnalysisEnabled {
		return nil, fmt.Errorf("trend analysis is disabled")
	}

	startTime := time.Now()
	
	r.logger.Info("Analyzing compliance trends",
		zap.Time("start_date", req.StartDate),
		zap.Time("end_date", req.EndDate),
		zap.String("granularity", req.Granularity),
		zap.Strings("metrics", req.Metrics),
	)

	// Get historical metrics
	metricsReq := MetricsRequest{
		StartDate:   req.StartDate,
		EndDate:     req.EndDate,
		Regulations: req.Regulations,
		GroupBy:     req.Granularity,
	}

	metrics, err := r.complianceRepo.GetComplianceMetrics(ctx, metricsReq)
	if err != nil {
		r.logger.Error("Failed to get metrics for trend analysis",
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to get metrics: %w", err)
	}

	// Generate trend data for each requested metric
	trendMetrics := make(map[string][]TrendDataPoint)
	for _, metric := range req.Metrics {
		trendData := r.generateTrendData(metrics, metric, req.Granularity)
		trendMetrics[metric] = trendData
	}

	// Generate insights
	insights := r.generateTrendInsights(trendMetrics)

	// Generate predictions if enabled
	var predictions []TrendPrediction
	if r.config.PredictiveAnalyticsEnabled {
		predictions = r.generateTrendPredictions(trendMetrics, req.EndDate)
	}

	trends := &ComplianceTrends{
		Period: ReportPeriod{
			StartDate: req.StartDate,
			EndDate:   req.EndDate,
		},
		Metrics:     trendMetrics,
		Insights:    insights,
		Predictions: predictions,
	}

	r.logger.Debug("Compliance trend analysis completed",
		zap.Duration("duration", time.Since(startTime)),
		zap.Int("insights_count", len(insights)),
		zap.Int("predictions_count", len(predictions)),
	)

	return trends, nil
}

// Helper methods

func (r *ComplianceReporter) createComplianceSummary(metrics *ComplianceMetrics, violations []*compliance.ComplianceViolation) ComplianceSummary {
	criticalViolations := int64(0)
	for _, violation := range violations {
		if violation.Severity == compliance.SeverityCritical {
			criticalViolations++
		}
	}

	return ComplianceSummary{
		TotalChecks:        metrics.TotalChecks,
		ApprovedChecks:     metrics.ApprovedChecks,
		ViolationCount:     metrics.ViolationCount,
		ComplianceRate:     metrics.ComplianceRate,
		CriticalViolations: criticalViolations,
	}
}

func (r *ComplianceReporter) createViolationsReport(violations []*compliance.ComplianceViolation) *ViolationsReportDetails {
	report := &ViolationsReportDetails{
		TotalViolations: len(violations),
		ByType:          make(map[string]int),
		BySeverity:      make(map[string]int),
		ByRegulation:    make(map[string]int),
		TopViolations:   []*ViolationDetail{},
		Timeline:        []*ViolationTimelineItem{},
	}

	// Group violations by type, severity, and regulation
	for _, violation := range violations {
		report.ByType[violation.ViolationType.String()]++
		report.BySeverity[violation.Severity.String()]++
		// Would need to add regulation field to violation
	}

	// Create timeline
	dailyViolations := make(map[string]int)
	for _, violation := range violations {
		day := violation.CreatedAt.Format("2006-01-02")
		dailyViolations[day]++
	}

	for day, count := range dailyViolations {
		timelineItem := &ViolationTimelineItem{
			Date:  day,
			Count: count,
		}
		report.Timeline = append(report.Timeline, timelineItem)
	}

	// Sort timeline by date
	sort.Slice(report.Timeline, func(i, j int) bool {
		return report.Timeline[i].Date < report.Timeline[j].Date
	})

	return report
}

func (r *ComplianceReporter) createConsentReport(ctx context.Context, req ComplianceReportRequest) *ConsentReportDetails {
	// Implementation would query consent data
	return &ConsentReportDetails{
		TotalConsents:    0,
		ActiveConsents:   0,
		WithdrawnConsents: 0,
		ConsentsByType:   make(map[string]int),
		ConsentsBySource: make(map[string]int),
	}
}

func (r *ComplianceReporter) createAuditReport(ctx context.Context, req ComplianceReportRequest) *AuditReportDetails {
	// Implementation would query audit trail data
	return &AuditReportDetails{
		TotalAuditEvents: 0,
		EventsByType:     make(map[string]int),
		ComplianceEvents: 0,
		ViolationEvents:  0,
	}
}

func (r *ComplianceReporter) createTrendsReport(ctx context.Context, req ComplianceReportRequest, metrics *ComplianceMetrics) *TrendsReportDetails {
	return &TrendsReportDetails{
		ComplianceRateTrend:    metrics.TrendData,
		ViolationTrend:         []*ViolationTrendItem{},
		SeasonalPatterns:       []*SeasonalPattern{},
		PredictiveInsights:     []*PredictiveInsight{},
	}
}

func (r *ComplianceReporter) createCertificationReport(ctx context.Context, req ComplianceReportRequest, summary ComplianceSummary) *CertificationReportDetails {
	return &CertificationReportDetails{
		ComplianceScore:    summary.ComplianceRate * 100,
		CertificationLevel: r.determineCertificationLevel(summary.ComplianceRate),
		FindingsSummary:    []*CertificationFinding{},
		Recommendations:    []string{},
	}
}

func (r *ComplianceReporter) enrichViolation(ctx context.Context, violation *compliance.ComplianceViolation) *compliance.ComplianceViolation {
	// Add additional context, risk scoring, etc.
	// For now, return as-is
	return violation
}

func (r *ComplianceReporter) runMonitoringLoop(ctx context.Context, config MonitoringConfig, alertChan chan<- ComplianceAlert) {
	ticker := time.NewTicker(config.CheckInterval)
	defer ticker.Stop()
	defer close(alertChan)

	for {
		select {
		case <-ctx.Done():
			r.logger.Info("Stopping compliance monitoring")
			return
		case <-ticker.C:
			r.performComplianceCheck(ctx, config, alertChan)
		}
	}
}

func (r *ComplianceReporter) performComplianceCheck(ctx context.Context, config MonitoringConfig, alertChan chan<- ComplianceAlert) {
	r.logger.Debug("Performing real-time compliance check")

	// Check current metrics against thresholds
	now := time.Now()
	endTime := now
	startTime := now.Add(-config.CheckInterval)

	metricsReq := MetricsRequest{
		StartDate:   startTime,
		EndDate:     endTime,
		Regulations: config.Regulations,
		GroupBy:     "hour",
	}

	metrics, err := r.complianceRepo.GetComplianceMetrics(ctx, metricsReq)
	if err != nil {
		r.logger.Error("Failed to get metrics for monitoring",
			zap.Error(err),
		)
		return
	}

	// Check against alert rules
	for _, rule := range r.alertRules {
		if !rule.Enabled {
			continue
		}

		// Check if this regulation is being monitored
		monitoringThisRegulation := false
		for _, reg := range config.Regulations {
			if reg == rule.Regulation {
				monitoringThisRegulation = true
				break
			}
		}
		if !monitoringThisRegulation && len(config.Regulations) > 0 {
			continue
		}

		alert := r.evaluateAlertRule(rule, metrics, config.AlertThresholds)
		if alert != nil {
			select {
			case alertChan <- *alert:
				r.logger.Info("Compliance alert generated",
					zap.String("alert_id", alert.AlertID.String()),
					zap.String("type", alert.Type),
					zap.String("severity", string(alert.Severity)),
				)
			default:
				r.logger.Warn("Alert channel full, dropping alert",
					zap.String("alert_id", alert.AlertID.String()),
				)
			}
		}
	}
}

func (r *ComplianceReporter) evaluateAlertRule(rule AlertRule, metrics *ComplianceMetrics, thresholds map[string]float64) *ComplianceAlert {
	var metricValue float64
	
	switch rule.MetricType {
	case "compliance_rate":
		metricValue = metrics.ComplianceRate
	case "violation_count":
		metricValue = float64(metrics.ViolationCount)
	case "total_checks":
		metricValue = float64(metrics.TotalChecks)
	default:
		return nil
	}

	threshold := rule.Threshold
	if configThreshold, exists := thresholds[rule.MetricType]; exists {
		threshold = configThreshold
	}

	var triggered bool
	switch rule.Operator {
	case "greater_than":
		triggered = metricValue > threshold
	case "less_than":
		triggered = metricValue < threshold
	case "equals":
		triggered = metricValue == threshold
	default:
		return nil
	}

	if !triggered {
		return nil
	}

	return &ComplianceAlert{
		AlertID:        uuid.New(),
		Type:           rule.Name,
		Severity:       rule.Severity,
		Message:        fmt.Sprintf("%s: %s %s %.2f (threshold: %.2f)", rule.Name, rule.MetricType, rule.Operator, metricValue, threshold),
		Timestamp:      time.Now(),
		Regulation:     rule.Regulation,
		ActionRequired: rule.Severity >= compliance.SeverityHigh,
		Metadata: map[string]interface{}{
			"rule_id":      rule.RuleID,
			"metric_type":  rule.MetricType,
			"metric_value": metricValue,
			"threshold":    threshold,
			"operator":     rule.Operator,
		},
	}
}

func (r *ComplianceReporter) generateTrendData(metrics *ComplianceMetrics, metricName, granularity string) []TrendDataPoint {
	trendData := make([]TrendDataPoint, 0)

	for _, dataPoint := range metrics.TrendData {
		var value float64
		switch metricName {
		case "violations":
			value = float64(dataPoint.Value)
		case "compliance_rate":
			// Calculate compliance rate from data point
			value = metrics.ComplianceRate // Simplified
		case "total_checks":
			value = float64(dataPoint.Value)
		}

		trendPoint := TrendDataPoint{
			Timestamp: dataPoint.Date,
			Value:     value,
			Metadata: map[string]interface{}{
				"type":        dataPoint.Type,
				"granularity": granularity,
			},
		}
		trendData = append(trendData, trendPoint)
	}

	return trendData
}

func (r *ComplianceReporter) generateTrendInsights(trendMetrics map[string][]TrendDataPoint) []TrendInsight {
	insights := make([]TrendInsight, 0)

	for metricName, dataPoints := range trendMetrics {
		if len(dataPoints) < 2 {
			continue
		}

		// Calculate trend direction
		firstValue := dataPoints[0].Value
		lastValue := dataPoints[len(dataPoints)-1].Value
		changePercent := ((lastValue - firstValue) / firstValue) * 100

		var trendType, impact string
		var confidence float64

		if math.Abs(changePercent) < 5 {
			trendType = "stable"
			impact = "neutral"
			confidence = 0.7
		} else if changePercent > 0 {
			if metricName == "violations" {
				trendType = "increasing"
				impact = "negative"
			} else {
				trendType = "increasing"
				impact = "positive"
			}
			confidence = 0.8
		} else {
			if metricName == "violations" {
				trendType = "decreasing"
				impact = "positive"
			} else {
				trendType = "decreasing"
				impact = "negative"
			}
			confidence = 0.8
		}

		insight := TrendInsight{
			Type:       trendType,
			Message:    fmt.Sprintf("%s trend is %s with %.1f%% change", metricName, trendType, changePercent),
			Confidence: confidence,
			Impact:     impact,
		}
		insights = append(insights, insight)
	}

	return insights
}

func (r *ComplianceReporter) generateTrendPredictions(trendMetrics map[string][]TrendDataPoint, lastDate time.Time) []TrendPrediction {
	predictions := make([]TrendPrediction, 0)

	for metricName, dataPoints := range trendMetrics {
		if len(dataPoints) < 3 {
			continue
		}

		// Simple linear regression prediction
		n := len(dataPoints)
		sumX, sumY, sumXY, sumX2 := 0.0, 0.0, 0.0, 0.0

		for i, point := range dataPoints {
			x := float64(i)
			y := point.Value
			sumX += x
			sumY += y
			sumXY += x * y
			sumX2 += x * x
		}

		slope := (float64(n)*sumXY - sumX*sumY) / (float64(n)*sumX2 - sumX*sumX)
		intercept := (sumY - slope*sumX) / float64(n)

		// Predict next value
		nextX := float64(n)
		predictedValue := slope*nextX + intercept

		confidence := 0.6 // Simplified confidence calculation

		prediction := TrendPrediction{
			Metric:         metricName,
			PredictedValue: predictedValue,
			Confidence:     confidence,
			Timestamp:      lastDate.AddDate(0, 0, 1), // Next day
		}
		predictions = append(predictions, prediction)
	}

	return predictions
}

func (r *ComplianceReporter) determineCertificationLevel(complianceRate float64) string {
	if complianceRate >= 0.98 {
		return "Gold"
	} else if complianceRate >= 0.95 {
		return "Silver"
	} else if complianceRate >= 0.90 {
		return "Bronze"
	} else {
		return "Non-Compliant"
	}
}

// Report detail types

type ViolationsReportDetails struct {
	TotalViolations int                        `json:"total_violations"`
	ByType          map[string]int             `json:"by_type"`
	BySeverity      map[string]int             `json:"by_severity"`
	ByRegulation    map[string]int             `json:"by_regulation"`
	TopViolations   []*ViolationDetail         `json:"top_violations"`
	Timeline        []*ViolationTimelineItem   `json:"timeline"`
}

type ViolationDetail struct {
	ID          uuid.UUID               `json:"id"`
	Type        compliance.ViolationType `json:"type"`
	Severity    compliance.Severity     `json:"severity"`
	Description string                  `json:"description"`
	Count       int                     `json:"count"`
	FirstSeen   time.Time               `json:"first_seen"`
	LastSeen    time.Time               `json:"last_seen"`
}

type ViolationTimelineItem struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

type ConsentReportDetails struct {
	TotalConsents     int            `json:"total_consents"`
	ActiveConsents    int            `json:"active_consents"`
	WithdrawnConsents int            `json:"withdrawn_consents"`
	ConsentsByType    map[string]int `json:"consents_by_type"`
	ConsentsBySource  map[string]int `json:"consents_by_source"`
}

type AuditReportDetails struct {
	TotalAuditEvents int            `json:"total_audit_events"`
	EventsByType     map[string]int `json:"events_by_type"`
	ComplianceEvents int            `json:"compliance_events"`
	ViolationEvents  int            `json:"violation_events"`
}

type TrendsReportDetails struct {
	ComplianceRateTrend []MetricDataPoint     `json:"compliance_rate_trend"`
	ViolationTrend      []*ViolationTrendItem `json:"violation_trend"`
	SeasonalPatterns    []*SeasonalPattern    `json:"seasonal_patterns"`
	PredictiveInsights  []*PredictiveInsight  `json:"predictive_insights"`
}

type ViolationTrendItem struct {
	Date  time.Time `json:"date"`
	Count int       `json:"count"`
	Type  string    `json:"type"`
}

type SeasonalPattern struct {
	Pattern     string  `json:"pattern"`
	Description string  `json:"description"`
	Strength    float64 `json:"strength"`
}

type PredictiveInsight struct {
	Insight    string  `json:"insight"`
	Confidence float64 `json:"confidence"`
	Timeframe  string  `json:"timeframe"`
}

type CertificationReportDetails struct {
	ComplianceScore    float64                   `json:"compliance_score"`
	CertificationLevel string                    `json:"certification_level"`
	FindingsSummary    []*CertificationFinding   `json:"findings_summary"`
	Recommendations    []string                  `json:"recommendations"`
}

type AlertHistoryItem struct {
	AlertID   uuid.UUID `json:"alert_id"`
	Type      string    `json:"type"`
	Severity  string    `json:"severity"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
	Resolved  bool      `json:"resolved"`
}

type AlertFilters struct {
	StartDate *time.Time `json:"start_date,omitempty"`
	EndDate   *time.Time `json:"end_date,omitempty"`
	Severity  []string   `json:"severity,omitempty"`
	Type      []string   `json:"type,omitempty"`
	Resolved  *bool      `json:"resolved,omitempty"`
}

type CertificateValidationResult struct {
	Valid       bool      `json:"valid"`
	ExpiresAt   time.Time `json:"expires_at"`
	Status      string    `json:"status"`
	Violations  []string  `json:"violations,omitempty"`
}

// Initialization functions

func initializeAlertRules() map[string]AlertRule {
	return map[string]AlertRule{
		"high_violation_rate": {
			RuleID:      "high_violation_rate",
			Name:        "High Violation Rate",
			Description: "Alert when violation rate exceeds threshold",
			Regulation:  RegulationType("all"),
			MetricType:  "violation_count",
			Threshold:   10,
			Operator:    "greater_than",
			Severity:    compliance.SeverityHigh,
			CooldownPeriod: 30 * time.Minute,
			NotificationChannels: []NotificationChannel{NotificationEmail, NotificationSlack},
			Enabled:     true,
		},
		"low_compliance_rate": {
			RuleID:      "low_compliance_rate",
			Name:        "Low Compliance Rate",
			Description: "Alert when compliance rate falls below threshold",
			Regulation:  RegulationType("all"),
			MetricType:  "compliance_rate",
			Threshold:   0.95,
			Operator:    "less_than",
			Severity:    compliance.SeverityCritical,
			CooldownPeriod: 15 * time.Minute,
			NotificationChannels: []NotificationChannel{NotificationEmail, NotificationSlack, NotificationWebhook},
			Enabled:     true,
		},
	}
}

func initializeReportTemplates() map[string]ReportTemplate {
	return map[string]ReportTemplate{
		"daily_compliance": {
			TemplateID:  "daily_compliance",
			Name:        "Daily Compliance Report",
			Description: "Daily summary of compliance metrics and violations",
			ReportType:  ReportTypeAudit,
			Regulations: []RegulationType{RegulationTCPA, RegulationGDPR},
			Sections: []ReportSection{
				{
					SectionID:  "summary",
					Title:      "Executive Summary",
					Type:       "summary",
					DataSource: "metrics",
					Required:   true,
				},
				{
					SectionID:  "violations",
					Title:      "Violations",
					Type:       "table",
					DataSource: "violations",
					Required:   true,
				},
			},
			Formats:       []string{"json", "pdf", "html"},
			DefaultPeriod: "daily",
		},
	}
}

func initializeComplianceThresholds() map[string]ComplianceThreshold {
	return map[string]ComplianceThreshold{
		"compliance_rate": {
			MetricName:    "compliance_rate",
			MinValue:      0.0,
			MaxValue:      1.0,
			TargetValue:   0.98,
			CriticalValue: 0.95,
			Unit:          "percentage",
			Description:   "Overall compliance rate across all regulations",
		},
		"violation_count": {
			MetricName:    "violation_count",
			MinValue:      0,
			MaxValue:      math.Inf(1),
			TargetValue:   0,
			CriticalValue: 10,
			Unit:          "count",
			Description:   "Number of compliance violations",
		},
	}
}

// DefaultReporterConfig returns a default reporter configuration
func DefaultReporterConfig() ReporterConfig {
	return ReporterConfig{
		EnableRealTimeMonitoring:   true,
		MonitoringInterval:         5 * time.Minute,
		AlertCooldownPeriod:        15 * time.Minute,
		TrendAnalysisEnabled:       true,
		PredictiveAnalyticsEnabled: true,
		ReportRetentionDays:        365,
		AutoCertificationEnabled:   true,
		ComplianceThresholds: map[string]float64{
			"compliance_rate":  0.95,
			"violation_count":  10,
			"response_time_ms": 5000,
		},
		NotificationChannels: []NotificationChannel{NotificationEmail, NotificationSlack},
		ExportFormats:        []string{"json", "csv", "pdf", "html"},
	}
}