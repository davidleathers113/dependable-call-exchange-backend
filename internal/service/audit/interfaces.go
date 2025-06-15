package audit

import (
	"context"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/audit"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
)

// IntegrityServiceInterface defines the contract for the integrity service
// This interface allows for easy testing and mocking
type IntegrityServiceInterface interface {
	// Service lifecycle
	Start(ctx context.Context) error
	Stop(ctx context.Context) error

	// Hash chain verification
	VerifyHashChain(ctx context.Context, start, end values.SequenceNumber) (*audit.HashChainVerificationResult, error)
	RepairChain(ctx context.Context, start, end values.SequenceNumber, options *RepairOptions) (*audit.HashChainRepairResult, error)

	// Sequence integrity
	VerifySequenceIntegrity(ctx context.Context, criteria audit.SequenceIntegrityCriteria) (*audit.SequenceIntegrityResult, error)

	// Comprehensive checks
	PerformIntegrityCheck(ctx context.Context, criteria audit.IntegrityCriteria) (*audit.IntegrityReport, error)

	// Corruption detection
	DetectCorruption(ctx context.Context, criteria audit.CorruptionDetectionCriteria) (*audit.CorruptionReport, error)

	// Scheduling
	ScheduleIntegrityCheck(ctx context.Context, schedule *audit.IntegrityCheckSchedule) (string, error)

	// Status and monitoring
	GetIntegrityStatus(ctx context.Context) (*IntegrityServiceStatus, error)
}

// WorkerPoolInterface defines the contract for the worker pool
type WorkerPoolInterface interface {
	Start() error
	Stop()
	SubmitTask(task IntegrityTask) bool
	GetStatus() *WorkerPoolStatus
}

// SchedulerInterface defines the contract for the integrity scheduler
type SchedulerInterface interface {
	Start() error
	Stop()
	AddSchedule(check *ScheduledCheck)
	RemoveSchedule(checkID string)
	GetSchedules() map[string]*ScheduledCheck
	GetStatus() *SchedulerStatus
	UpdateSchedule(checkID string, updates map[string]interface{}) error
	SetupDefaultSchedules()
}

// AlertManagerInterface defines the contract for the alert manager
type AlertManagerInterface interface {
	TriggerAlert(ctx context.Context, alert *IntegrityAlert)
	ResolveAlert(ctx context.Context, alertID string, resolvedBy string) error
	GetActiveAlerts(ctx context.Context) ([]*IntegrityAlert, error)
	GetAllAlerts(ctx context.Context) ([]*IntegrityAlert, error)
	GetAlert(ctx context.Context, alertID string) (*IntegrityAlert, error)
	GetAlertSummary(ctx context.Context) *AlertSummary
	CleanupOldAlerts(ctx context.Context, maxAge time.Duration)
	StartPeriodicCleanup()
}

// IntegrityMonitorInterface defines the contract for integrity monitoring
type IntegrityMonitorInterface interface {
	StartContinuousMonitoring(ctx context.Context) error
	StopContinuousMonitoring(ctx context.Context) error
	GetMonitoringStatus(ctx context.Context) (*MonitoringStatus, error)
	ConfigureMonitoring(ctx context.Context, config *audit.IntegrityMonitoringConfig) error
}

// IntegrityReportInterface defines the contract for integrity reporting
type IntegrityReportInterface interface {
	GenerateReport(ctx context.Context, criteria audit.IntegrityReportCriteria) (*audit.ComprehensiveIntegrityReport, error)
	ScheduleReport(ctx context.Context, schedule *ReportSchedule) (string, error)
	GetReportHistory(ctx context.Context, limit int) ([]*ReportHistoryItem, error)
}

// CacheInterface defines the contract for integrity caching
type CacheInterface interface {
	GetHashChainResult(ctx context.Context, start, end values.SequenceNumber) (*audit.HashChainVerificationResult, error)
	SetHashChainResult(ctx context.Context, start, end values.SequenceNumber, result *audit.HashChainVerificationResult, ttl time.Duration) error
	InvalidateHashChainRange(ctx context.Context, start, end values.SequenceNumber) error
	GetSequenceResult(ctx context.Context, criteria audit.SequenceIntegrityCriteria) (*audit.SequenceIntegrityResult, error)
	SetSequenceResult(ctx context.Context, criteria audit.SequenceIntegrityCriteria, result *audit.SequenceIntegrityResult, ttl time.Duration) error
}

// Additional supporting types

// MonitoringStatus represents the status of continuous integrity monitoring
type MonitoringStatus struct {
	IsEnabled        bool                             `json:"is_enabled"`
	LastCheck        time.Time                        `json:"last_check"`
	NextCheck        time.Time                        `json:"next_check"`
	CheckInterval    time.Duration                    `json:"check_interval"`
	HealthStatus     string                           `json:"health_status"`
	ActiveChecks     []string                         `json:"active_checks"`
	FailedChecks     int64                            `json:"failed_checks"`
	TotalChecks      int64                            `json:"total_checks"`
	AverageCheckTime time.Duration                    `json:"average_check_time"`
	Configuration    *audit.IntegrityMonitoringConfig `json:"configuration"`
}

// ReportSchedule represents a scheduled integrity report
type ReportSchedule struct {
	ID             string                        `json:"id"`
	Name           string                        `json:"name"`
	ReportType     string                        `json:"report_type"`
	Criteria       audit.IntegrityReportCriteria `json:"criteria"`
	CronExpression string                        `json:"cron_expression"`
	Recipients     []string                      `json:"recipients"`
	Format         string                        `json:"format"` // json, pdf, html
	IsEnabled      bool                          `json:"is_enabled"`
	CreatedAt      time.Time                     `json:"created_at"`
	LastRun        *time.Time                    `json:"last_run,omitempty"`
	NextRun        time.Time                     `json:"next_run"`
}

// ReportHistoryItem represents a historical report execution
type ReportHistoryItem struct {
	ReportID       string        `json:"report_id"`
	ScheduleID     string        `json:"schedule_id"`
	ReportType     string        `json:"report_type"`
	GeneratedAt    time.Time     `json:"generated_at"`
	Duration       time.Duration `json:"duration"`
	Status         string        `json:"status"` // success, failed, partial
	EventsAnalyzed int64         `json:"events_analyzed"`
	IssuesFound    int64         `json:"issues_found"`
	ReportSize     int64         `json:"report_size"`
	Location       string        `json:"location"` // file path or URL
	Error          string        `json:"error,omitempty"`
}

// IntegrityConfiguration represents complete integrity service configuration
type IntegrityConfiguration struct {
	Service    *IntegrityConfig                 `json:"service"`
	Monitoring *audit.IntegrityMonitoringConfig `json:"monitoring"`
	Cache      *CacheConfig                     `json:"cache"`
	Alerts     *AlertConfig                     `json:"alerts"`
	Reports    *ReportConfig                    `json:"reports"`
}

// CacheConfig represents cache configuration for integrity operations
type CacheConfig struct {
	Enabled         bool          `json:"enabled"`
	DefaultTTL      time.Duration `json:"default_ttl"`
	HashChainTTL    time.Duration `json:"hash_chain_ttl"`
	SequenceTTL     time.Duration `json:"sequence_ttl"`
	CorruptionTTL   time.Duration `json:"corruption_ttl"`
	MaxCacheSize    int64         `json:"max_cache_size"`
	EvictionPolicy  string        `json:"eviction_policy"` // lru, lfu, random
	RefreshInterval time.Duration `json:"refresh_interval"`
	PreloadOnStart  bool          `json:"preload_on_start"`
}

// AlertConfig represents alert configuration
type AlertConfig struct {
	Enabled              bool               `json:"enabled"`
	DefaultCooldown      time.Duration      `json:"default_cooldown"`
	SeverityThresholds   map[string]float64 `json:"severity_thresholds"`
	NotificationChannels []string           `json:"notification_channels"`
	EscalationRules      []EscalationRule   `json:"escalation_rules"`
	SuppressLowSeverity  bool               `json:"suppress_low_severity"`
	BatchNotifications   bool               `json:"batch_notifications"`
	BatchWindow          time.Duration      `json:"batch_window"`
	RetentionPeriod      time.Duration      `json:"retention_period"`
}

// EscalationRule defines when and how to escalate alerts
type EscalationRule struct {
	Condition string        `json:"condition"` // alert_age, alert_count, severity
	Threshold float64       `json:"threshold"`
	Action    string        `json:"action"` // email, page, escalate
	Target    string        `json:"target"` // email address, team, etc.
	Cooldown  time.Duration `json:"cooldown"`
}

// ReportConfig represents report configuration
type ReportConfig struct {
	Enabled            bool          `json:"enabled"`
	DefaultFormat      string        `json:"default_format"`
	StorageLocation    string        `json:"storage_location"`
	RetentionPeriod    time.Duration `json:"retention_period"`
	CompressionEnabled bool          `json:"compression_enabled"`
	EncryptionEnabled  bool          `json:"encryption_enabled"`
	MaxReportSize      int64         `json:"max_report_size"`
	ParallelGeneration bool          `json:"parallel_generation"`
	TemplateDirectory  string        `json:"template_directory"`
}

// DefaultIntegrityConfiguration returns a default configuration
func DefaultIntegrityConfiguration() *IntegrityConfiguration {
	return &IntegrityConfiguration{
		Service:    DefaultIntegrityConfig(),
		Monitoring: DefaultMonitoringConfig(),
		Cache:      DefaultCacheConfig(),
		Alerts:     DefaultAlertConfig(),
		Reports:    DefaultReportConfig(),
	}
}

// DefaultMonitoringConfig returns default monitoring configuration
func DefaultMonitoringConfig() *audit.IntegrityMonitoringConfig {
	return &audit.IntegrityMonitoringConfig{
		MonitorAll:           true,
		HashChainInterval:    5 * time.Minute,
		SequenceInterval:     10 * time.Minute,
		CorruptionInterval:   30 * time.Minute,
		HashFailureThreshold: 0.99,
		SequenceGapThreshold: 5,
		CorruptionThreshold:  0.01,
		EnableAlerts:         true,
		AlertChannels:        []string{"log", "metrics"},
		AlertSeverities:      []string{"warning", "error", "critical"},
		MaxConcurrentChecks:  5,
		CheckTimeout:         5 * time.Minute,
		HistoryRetentionDays: 30,
		AlertRetentionDays:   7,
	}
}

// DefaultCacheConfig returns default cache configuration
func DefaultCacheConfig() *CacheConfig {
	return &CacheConfig{
		Enabled:         true,
		DefaultTTL:      1 * time.Hour,
		HashChainTTL:    24 * time.Hour,
		SequenceTTL:     1 * time.Hour,
		CorruptionTTL:   30 * time.Minute,
		MaxCacheSize:    1024 * 1024 * 100, // 100MB
		EvictionPolicy:  "lru",
		RefreshInterval: 5 * time.Minute,
		PreloadOnStart:  false,
	}
}

// DefaultAlertConfig returns default alert configuration
func DefaultAlertConfig() *AlertConfig {
	return &AlertConfig{
		Enabled:         true,
		DefaultCooldown: 15 * time.Minute,
		SeverityThresholds: map[string]float64{
			"info":     0.0,
			"warning":  0.05,
			"error":    0.10,
			"critical": 0.20,
		},
		NotificationChannels: []string{"log", "metrics"},
		EscalationRules:      []EscalationRule{},
		SuppressLowSeverity:  false,
		BatchNotifications:   true,
		BatchWindow:          5 * time.Minute,
		RetentionPeriod:      7 * 24 * time.Hour,
	}
}

// DefaultReportConfig returns default report configuration
func DefaultReportConfig() *ReportConfig {
	return &ReportConfig{
		Enabled:            true,
		DefaultFormat:      "json",
		StorageLocation:    "/var/log/dce/integrity-reports",
		RetentionPeriod:    30 * 24 * time.Hour,
		CompressionEnabled: true,
		EncryptionEnabled:  false,
		MaxReportSize:      100 * 1024 * 1024, // 100MB
		ParallelGeneration: true,
		TemplateDirectory:  "/etc/dce/report-templates",
	}
}
