package audit

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/audit"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/infrastructure/cache"
)

// IntegrityService orchestrates audit integrity verification operations
// Following DCE patterns: service-layer orchestration without business logic
type IntegrityService struct {
	// Domain services (business logic)
	hashChainService  *audit.HashChainService
	integrityService  *audit.IntegrityCheckService
	complianceService *audit.ComplianceVerificationService
	recoveryService   *audit.ChainRecoveryService
	cryptoService     *audit.CryptoService

	// Infrastructure dependencies
	eventRepo     audit.EventRepository
	integrityRepo audit.IntegrityRepository
	auditCache    *cache.AuditCache
	logger        *zap.Logger

	// Performance and concurrency management
	workerPool   *WorkerPool
	scheduler    *IntegrityScheduler
	alertManager *AlertManager
	metrics      *IntegrityMetrics

	// Configuration
	config *IntegrityConfig

	// State management
	mu            sync.RWMutex
	isRunning     bool
	backgroundCtx context.Context
	cancelBg      context.CancelFunc
}

// IntegrityConfig configures the integrity service
type IntegrityConfig struct {
	// Monitoring intervals
	HashChainCheckInterval  time.Duration `json:"hash_chain_check_interval"`
	SequenceCheckInterval   time.Duration `json:"sequence_check_interval"`
	CorruptionScanInterval  time.Duration `json:"corruption_scan_interval"`
	ComplianceCheckInterval time.Duration `json:"compliance_check_interval"`

	// Performance settings
	MaxConcurrentChecks int           `json:"max_concurrent_checks"`
	BatchSize           int           `json:"batch_size"`
	CheckTimeout        time.Duration `json:"check_timeout"`

	// Chain verification settings
	IncrementalCheckSize int     `json:"incremental_check_size"`
	FullCheckThreshold   int64   `json:"full_check_threshold"`
	ChainRepairEnabled   bool    `json:"chain_repair_enabled"`
	AutoRepairThreshold  float64 `json:"auto_repair_threshold"`

	// Alert thresholds
	IntegrityScoreThreshold float64       `json:"integrity_score_threshold"`
	CorruptionThreshold     float64       `json:"corruption_threshold"`
	AlertCooldown           time.Duration `json:"alert_cooldown"`

	// Cache settings
	CacheEnabled         bool          `json:"cache_enabled"`
	CacheRefreshInterval time.Duration `json:"cache_refresh_interval"`

	// Background processing
	EnableBackgroundChecks  bool          `json:"enable_background_checks"`
	BackgroundCheckInterval time.Duration `json:"background_check_interval"`
}

// DefaultIntegrityConfig returns sensible defaults
func DefaultIntegrityConfig() *IntegrityConfig {
	return &IntegrityConfig{
		HashChainCheckInterval:  5 * time.Minute,
		SequenceCheckInterval:   10 * time.Minute,
		CorruptionScanInterval:  30 * time.Minute,
		ComplianceCheckInterval: 1 * time.Hour,
		MaxConcurrentChecks:     10,
		BatchSize:               1000,
		CheckTimeout:            5 * time.Minute,
		IncrementalCheckSize:    100,
		FullCheckThreshold:      10000,
		ChainRepairEnabled:      true,
		AutoRepairThreshold:     0.95,
		IntegrityScoreThreshold: 0.99,
		CorruptionThreshold:     0.01,
		AlertCooldown:           15 * time.Minute,
		CacheEnabled:            true,
		CacheRefreshInterval:    1 * time.Minute,
		EnableBackgroundChecks:  true,
		BackgroundCheckInterval: 1 * time.Minute,
	}
}

// WorkerPool manages concurrent integrity checks
type WorkerPool struct {
	workers    int
	taskChan   chan IntegrityTask
	resultChan chan IntegrityResult
	wg         sync.WaitGroup
	ctx        context.Context
	cancel     context.CancelFunc
}

// IntegrityTask represents a task for integrity verification
type IntegrityTask struct {
	TaskID    string                `json:"task_id"`
	TaskType  string                `json:"task_type"` // hash_chain, sequence, corruption, compliance
	StartSeq  values.SequenceNumber `json:"start_seq"`
	EndSeq    values.SequenceNumber `json:"end_seq"`
	Priority  int                   `json:"priority"`
	TimeRange *audit.TimeRange      `json:"time_range,omitempty"`
	Criteria  interface{}           `json:"criteria,omitempty"`
	CreatedAt time.Time             `json:"created_at"`
}

// IntegrityResult represents the result of an integrity task
type IntegrityResult struct {
	TaskID      string        `json:"task_id"`
	TaskType    string        `json:"task_type"`
	Success     bool          `json:"success"`
	Duration    time.Duration `json:"duration"`
	Result      interface{}   `json:"result"`
	Error       error         `json:"error,omitempty"`
	CompletedAt time.Time     `json:"completed_at"`
}

// IntegrityScheduler manages scheduled integrity checks
type IntegrityScheduler struct {
	service   *IntegrityService
	ticker    *time.Ticker
	schedules map[string]*ScheduledCheck
	mu        sync.RWMutex
}

// ScheduledCheck represents a scheduled integrity check
type ScheduledCheck struct {
	ID        string        `json:"id"`
	Name      string        `json:"name"`
	Type      string        `json:"type"`
	Interval  time.Duration `json:"interval"`
	LastRun   *time.Time    `json:"last_run,omitempty"`
	NextRun   time.Time     `json:"next_run"`
	IsEnabled bool          `json:"is_enabled"`
	Criteria  interface{}   `json:"criteria,omitempty"`
}

// AlertManager handles integrity violation alerts
type AlertManager struct {
	service   *IntegrityService
	alerts    map[string]*IntegrityAlert
	cooldowns map[string]time.Time
	mu        sync.RWMutex
}

// IntegrityAlert represents an integrity violation alert
type IntegrityAlert struct {
	AlertID     string      `json:"alert_id"`
	AlertType   string      `json:"alert_type"`
	Severity    string      `json:"severity"`
	Title       string      `json:"title"`
	Description string      `json:"description"`
	Details     interface{} `json:"details"`
	TriggeredAt time.Time   `json:"triggered_at"`
	IsResolved  bool        `json:"is_resolved"`
	ResolvedAt  *time.Time  `json:"resolved_at,omitempty"`
}

// IntegrityMetrics tracks integrity verification performance
type IntegrityMetrics struct {
	// Check counts
	TotalChecks      int64 `json:"total_checks"`
	SuccessfulChecks int64 `json:"successful_checks"`
	FailedChecks     int64 `json:"failed_checks"`

	// Performance metrics
	AverageCheckTime time.Duration `json:"average_check_time"`
	TotalCheckTime   time.Duration `json:"total_check_time"`

	// Chain metrics
	HashChainChecks int64 `json:"hash_chain_checks"`
	SequenceChecks  int64 `json:"sequence_checks"`
	CorruptionScans int64 `json:"corruption_scans"`

	// Alert metrics
	AlertsTriggered int64 `json:"alerts_triggered"`
	AlertsResolved  int64 `json:"alerts_resolved"`

	// Last update
	LastUpdated time.Time `json:"last_updated"`
}

// NewIntegrityService creates a new integrity verification service
func NewIntegrityService(
	hashChainService *audit.HashChainService,
	integrityService *audit.IntegrityCheckService,
	complianceService *audit.ComplianceVerificationService,
	recoveryService *audit.ChainRecoveryService,
	cryptoService *audit.CryptoService,
	eventRepo audit.EventRepository,
	integrityRepo audit.IntegrityRepository,
	auditCache *cache.AuditCache,
	logger *zap.Logger,
	config *IntegrityConfig,
) *IntegrityService {
	if config == nil {
		config = DefaultIntegrityConfig()
	}

	backgroundCtx, cancelBg := context.WithCancel(context.Background())

	service := &IntegrityService{
		hashChainService:  hashChainService,
		integrityService:  integrityService,
		complianceService: complianceService,
		recoveryService:   recoveryService,
		cryptoService:     cryptoService,
		eventRepo:         eventRepo,
		integrityRepo:     integrityRepo,
		auditCache:        auditCache,
		logger:            logger,
		config:            config,
		backgroundCtx:     backgroundCtx,
		cancelBg:          cancelBg,
		metrics:           &IntegrityMetrics{},
	}

	// Initialize components
	service.workerPool = NewWorkerPool(config.MaxConcurrentChecks, backgroundCtx)
	service.scheduler = NewIntegrityScheduler(service)
	service.alertManager = NewAlertManager(service)

	return service
}

// Start initializes and starts the integrity service
func (s *IntegrityService) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.isRunning {
		return errors.NewInternalError("integrity service already running")
	}

	s.logger.Info("Starting integrity service",
		zap.Bool("background_checks_enabled", s.config.EnableBackgroundChecks),
		zap.Duration("check_interval", s.config.BackgroundCheckInterval),
		zap.Int("max_concurrent_checks", s.config.MaxConcurrentChecks))

	// Start worker pool
	if err := s.workerPool.Start(); err != nil {
		return errors.NewInternalError("failed to start worker pool").WithCause(err)
	}

	// Start scheduler if background checks enabled
	if s.config.EnableBackgroundChecks {
		if err := s.scheduler.Start(); err != nil {
			s.workerPool.Stop()
			return errors.NewInternalError("failed to start scheduler").WithCause(err)
		}
	}

	// Start background processing
	go s.runBackgroundProcessing()

	s.isRunning = true
	s.logger.Info("Integrity service started successfully")

	return nil
}

// Stop gracefully shuts down the integrity service
func (s *IntegrityService) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isRunning {
		return nil
	}

	s.logger.Info("Stopping integrity service...")

	// Cancel background context
	s.cancelBg()

	// Stop scheduler
	if s.scheduler != nil {
		s.scheduler.Stop()
	}

	// Stop worker pool
	if s.workerPool != nil {
		s.workerPool.Stop()
	}

	s.isRunning = false
	s.logger.Info("Integrity service stopped")

	return nil
}

// VerifyHashChain performs hash chain verification for a sequence range
func (s *IntegrityService) VerifyHashChain(ctx context.Context, start, end values.SequenceNumber) (*audit.HashChainVerificationResult, error) {
	startTime := time.Now()
	defer func() {
		s.updateMetrics("hash_chain", time.Since(startTime), nil)
	}()

	s.logger.Debug("Verifying hash chain",
		zap.Int64("start_sequence", int64(start)),
		zap.Int64("end_sequence", int64(end)))

	// Check cache first if enabled
	if s.config.CacheEnabled && s.auditCache != nil {
		if result := s.getCachedHashChainResult(ctx, start, end); result != nil {
			s.logger.Debug("Using cached hash chain verification result")
			return result, nil
		}
	}

	// Perform verification using domain service
	result, err := s.hashChainService.VerifyChain(ctx, start, end)
	if err != nil {
		s.logger.Error("Hash chain verification failed",
			zap.Error(err),
			zap.Int64("start_sequence", int64(start)),
			zap.Int64("end_sequence", int64(end)))

		s.updateMetrics("hash_chain", time.Since(startTime), err)
		return nil, err
	}

	// Cache result if enabled
	if s.config.CacheEnabled && s.auditCache != nil {
		s.cacheHashChainResult(ctx, start, end, result)
	}

	// Check for integrity violations and trigger alerts
	s.checkIntegrityThresholds(ctx, result)

	s.logger.Info("Hash chain verification completed",
		zap.Int64("events_verified", result.EventsVerified),
		zap.Bool("is_valid", result.IsValid),
		zap.Float64("integrity_score", result.IntegrityScore),
		zap.Duration("verification_time", result.VerificationTime))

	return result, nil
}

// VerifySequenceIntegrity performs sequence integrity verification
func (s *IntegrityService) VerifySequenceIntegrity(ctx context.Context, criteria audit.SequenceIntegrityCriteria) (*audit.SequenceIntegrityResult, error) {
	startTime := time.Now()
	defer func() {
		s.updateMetrics("sequence", time.Since(startTime), nil)
	}()

	s.logger.Debug("Verifying sequence integrity", zap.Any("criteria", criteria))

	// Use infrastructure repository for sequence verification
	result, err := s.integrityRepo.VerifySequenceIntegrity(ctx, criteria)
	if err != nil {
		s.logger.Error("Sequence integrity verification failed", zap.Error(err))
		s.updateMetrics("sequence", time.Since(startTime), err)
		return nil, err
	}

	// Trigger alerts for sequence issues
	if !result.IsValid {
		s.triggerSequenceAlert(ctx, result)
	}

	s.logger.Info("Sequence integrity verification completed",
		zap.Int64("sequences_checked", result.SequencesChecked),
		zap.Bool("is_valid", result.IsValid),
		zap.Int64("gaps_found", result.GapsFound),
		zap.Int64("duplicates_found", result.DuplicatesFound))

	return result, nil
}

// PerformIntegrityCheck runs comprehensive integrity verification
func (s *IntegrityService) PerformIntegrityCheck(ctx context.Context, criteria audit.IntegrityCriteria) (*audit.IntegrityReport, error) {
	startTime := time.Now()
	defer func() {
		s.updateMetrics("comprehensive", time.Since(startTime), nil)
	}()

	s.logger.Info("Starting comprehensive integrity check", zap.Any("criteria", criteria))

	// Use domain service for comprehensive check
	report, err := s.integrityService.PerformIntegrityCheck(ctx, criteria)
	if err != nil {
		s.logger.Error("Comprehensive integrity check failed", zap.Error(err))
		s.updateMetrics("comprehensive", time.Since(startTime), err)
		return nil, err
	}

	// Process report and trigger alerts
	s.processIntegrityReport(ctx, report)

	s.logger.Info("Comprehensive integrity check completed",
		zap.String("overall_status", report.OverallStatus),
		zap.Bool("is_healthy", report.IsHealthy),
		zap.Int64("total_events", report.TotalEvents),
		zap.Int64("verified_events", report.VerifiedEvents),
		zap.Int64("failed_events", report.FailedEvents))

	return report, nil
}

// DetectCorruption scans for data corruption
func (s *IntegrityService) DetectCorruption(ctx context.Context, criteria audit.CorruptionDetectionCriteria) (*audit.CorruptionReport, error) {
	startTime := time.Now()
	defer func() {
		s.updateMetrics("corruption", time.Since(startTime), nil)
	}()

	s.logger.Debug("Detecting corruption", zap.Any("criteria", criteria))

	// Use infrastructure repository for corruption detection
	report, err := s.integrityRepo.DetectCorruption(ctx, criteria)
	if err != nil {
		s.logger.Error("Corruption detection failed", zap.Error(err))
		s.updateMetrics("corruption", time.Since(startTime), err)
		return nil, err
	}

	// Trigger alerts for detected corruption
	if report.CorruptionFound {
		s.triggerCorruptionAlert(ctx, report)
	}

	s.logger.Info("Corruption detection completed",
		zap.Bool("corruption_found", report.CorruptionFound),
		zap.String("corruption_level", report.CorruptionLevel),
		zap.Int64("events_scanned", report.EventsScanned),
		zap.Int64("events_corrupted", report.EventsCorrupted))

	return report, nil
}

// RepairChain attempts to repair broken hash chains
func (s *IntegrityService) RepairChain(ctx context.Context, start, end values.SequenceNumber, options *RepairOptions) (*audit.HashChainRepairResult, error) {
	if !s.config.ChainRepairEnabled {
		return nil, errors.NewValidationError("REPAIR_DISABLED", "chain repair is disabled in configuration")
	}

	startTime := time.Now()
	defer func() {
		s.updateMetrics("repair", time.Since(startTime), nil)
	}()

	s.logger.Info("Starting chain repair",
		zap.Int64("start_sequence", int64(start)),
		zap.Int64("end_sequence", int64(end)))

	// Use domain service for chain repair
	result, err := s.hashChainService.RepairChain(ctx, start, end)
	if err != nil {
		s.logger.Error("Chain repair failed", zap.Error(err))
		s.updateMetrics("repair", time.Since(startTime), err)
		return nil, err
	}

	// Invalidate cache for repaired range
	if s.config.CacheEnabled && s.auditCache != nil {
		s.invalidateHashChainCache(ctx, start, end)
	}

	s.logger.Info("Chain repair completed",
		zap.Int64("events_repaired", result.EventsRepaired),
		zap.Int64("events_failed", result.EventsFailed),
		zap.Int64("hashes_recalculated", result.HashesRecalculated),
		zap.Duration("repair_time", result.RepairTime))

	return result, nil
}

// RepairOptions provides options for chain repair
type RepairOptions struct {
	DryRun         bool `json:"dry_run"`
	ForceRepair    bool `json:"force_repair"`
	SkipValidation bool `json:"skip_validation"`
}

// ScheduleIntegrityCheck schedules a recurring integrity check
func (s *IntegrityService) ScheduleIntegrityCheck(ctx context.Context, schedule *audit.IntegrityCheckSchedule) (string, error) {
	s.logger.Info("Scheduling integrity check",
		zap.String("check_type", schedule.CheckType),
		zap.String("cron_expression", schedule.CronExpression))

	// Use infrastructure repository to persist schedule
	scheduleID, err := s.integrityRepo.ScheduleIntegrityCheck(ctx, schedule)
	if err != nil {
		s.logger.Error("Failed to schedule integrity check", zap.Error(err))
		return "", err
	}

	// Add to local scheduler
	s.scheduler.AddSchedule(&ScheduledCheck{
		ID:        scheduleID,
		Name:      fmt.Sprintf("Integrity Check %s", schedule.CheckType),
		Type:      schedule.CheckType,
		IsEnabled: schedule.IsEnabled,
		NextRun:   schedule.NextRun,
	})

	return scheduleID, nil
}

// GetIntegrityStatus returns current integrity monitoring status
func (s *IntegrityService) GetIntegrityStatus(ctx context.Context) (*IntegrityServiceStatus, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Get monitoring status from infrastructure
	monitoringStatus, err := s.integrityRepo.GetIntegrityMonitoringStatus(ctx)
	if err != nil {
		s.logger.Error("Failed to get monitoring status", zap.Error(err))
		return nil, err
	}

	// Get active alerts
	alerts, err := s.alertManager.GetActiveAlerts(ctx)
	if err != nil {
		s.logger.Warn("Failed to get active alerts", zap.Error(err))
	}

	status := &IntegrityServiceStatus{
		IsRunning:         s.isRunning,
		BackgroundEnabled: s.config.EnableBackgroundChecks,
		WorkerPoolStatus:  s.workerPool.GetStatus(),
		SchedulerStatus:   s.scheduler.GetStatus(),
		MonitoringStatus:  monitoringStatus,
		ActiveAlerts:      len(alerts),
		Metrics:           s.metrics,
		LastHealthCheck:   time.Now(),
	}

	return status, nil
}

// IntegrityServiceStatus represents the current status of the integrity service
type IntegrityServiceStatus struct {
	IsRunning         bool                             `json:"is_running"`
	BackgroundEnabled bool                             `json:"background_enabled"`
	WorkerPoolStatus  *WorkerPoolStatus                `json:"worker_pool_status"`
	SchedulerStatus   *SchedulerStatus                 `json:"scheduler_status"`
	MonitoringStatus  *audit.IntegrityMonitoringStatus `json:"monitoring_status"`
	ActiveAlerts      int                              `json:"active_alerts"`
	Metrics           *IntegrityMetrics                `json:"metrics"`
	LastHealthCheck   time.Time                        `json:"last_health_check"`
}

// WorkerPoolStatus represents the status of the worker pool
type WorkerPoolStatus struct {
	ActiveWorkers  int   `json:"active_workers"`
	QueuedTasks    int   `json:"queued_tasks"`
	CompletedTasks int64 `json:"completed_tasks"`
	FailedTasks    int64 `json:"failed_tasks"`
}

// SchedulerStatus represents the status of the scheduler
type SchedulerStatus struct {
	IsRunning        bool       `json:"is_running"`
	ScheduledChecks  int        `json:"scheduled_checks"`
	CompletedChecks  int64      `json:"completed_checks"`
	NextScheduledRun *time.Time `json:"next_scheduled_run,omitempty"`
}

// Helper methods

// runBackgroundProcessing runs continuous background integrity checks
func (s *IntegrityService) runBackgroundProcessing() {
	ticker := time.NewTicker(s.config.BackgroundCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.backgroundCtx.Done():
			return
		case <-ticker.C:
			s.performBackgroundCheck()
		}
	}
}

// performBackgroundCheck performs routine integrity checks
func (s *IntegrityService) performBackgroundCheck() {
	ctx, cancel := context.WithTimeout(s.backgroundCtx, s.config.CheckTimeout)
	defer cancel()

	// Get latest sequence number
	latestSeq, err := s.eventRepo.GetLatestSequenceNumber(ctx)
	if err != nil {
		s.logger.Error("Failed to get latest sequence number", zap.Error(err))
		return
	}

	// Perform incremental hash chain check
	if latestSeq > values.SequenceNumber(s.config.IncrementalCheckSize) {
		startSeq := latestSeq - values.SequenceNumber(s.config.IncrementalCheckSize)

		task := IntegrityTask{
			TaskID:    uuid.New().String(),
			TaskType:  "hash_chain",
			StartSeq:  startSeq,
			EndSeq:    latestSeq,
			Priority:  1,
			CreatedAt: time.Now(),
		}

		select {
		case s.workerPool.taskChan <- task:
			s.logger.Debug("Queued background hash chain check",
				zap.String("task_id", task.TaskID),
				zap.Int64("start_seq", int64(startSeq)),
				zap.Int64("end_seq", int64(latestSeq)))
		default:
			s.logger.Warn("Worker pool queue full, skipping background check")
		}
	}
}

// Cache helper methods

func (s *IntegrityService) getCachedHashChainResult(ctx context.Context, start, end values.SequenceNumber) *audit.HashChainVerificationResult {
	// Implementation would use audit cache to retrieve cached results
	// For now, return nil to indicate cache miss
	return nil
}

func (s *IntegrityService) cacheHashChainResult(ctx context.Context, start, end values.SequenceNumber, result *audit.HashChainVerificationResult) {
	// Implementation would cache the result using audit cache
}

func (s *IntegrityService) invalidateHashChainCache(ctx context.Context, start, end values.SequenceNumber) {
	// Implementation would invalidate cached results for the range
}

// Alert helper methods

func (s *IntegrityService) checkIntegrityThresholds(ctx context.Context, result *audit.HashChainVerificationResult) {
	if result.IntegrityScore < s.config.IntegrityScoreThreshold {
		s.alertManager.TriggerAlert(ctx, &IntegrityAlert{
			AlertID:     uuid.New().String(),
			AlertType:   "integrity_score_low",
			Severity:    "warning",
			Title:       "Low Integrity Score Detected",
			Description: fmt.Sprintf("Hash chain integrity score %.3f is below threshold %.3f", result.IntegrityScore, s.config.IntegrityScoreThreshold),
			Details:     result,
			TriggeredAt: time.Now(),
		})
	}
}

func (s *IntegrityService) triggerSequenceAlert(ctx context.Context, result *audit.SequenceIntegrityResult) {
	severity := "info"
	if result.GapsFound > 0 {
		severity = "warning"
	}
	if result.DuplicatesFound > 0 {
		severity = "error"
	}

	s.alertManager.TriggerAlert(ctx, &IntegrityAlert{
		AlertID:     uuid.New().String(),
		AlertType:   "sequence_integrity_issue",
		Severity:    severity,
		Title:       "Sequence Integrity Issues Detected",
		Description: fmt.Sprintf("Found %d gaps and %d duplicates in sequence integrity check", result.GapsFound, result.DuplicatesFound),
		Details:     result,
		TriggeredAt: time.Now(),
	})
}

func (s *IntegrityService) triggerCorruptionAlert(ctx context.Context, report *audit.CorruptionReport) {
	severity := "warning"
	switch report.CorruptionLevel {
	case "high", "severe":
		severity = "critical"
	case "medium":
		severity = "error"
	}

	s.alertManager.TriggerAlert(ctx, &IntegrityAlert{
		AlertID:     uuid.New().String(),
		AlertType:   "corruption_detected",
		Severity:    severity,
		Title:       "Data Corruption Detected",
		Description: fmt.Sprintf("Corruption level: %s, %d events affected", report.CorruptionLevel, report.EventsCorrupted),
		Details:     report,
		TriggeredAt: time.Now(),
	})
}

func (s *IntegrityService) processIntegrityReport(ctx context.Context, report *audit.IntegrityReport) {
	if !report.IsHealthy {
		s.alertManager.TriggerAlert(ctx, &IntegrityAlert{
			AlertID:     uuid.New().String(),
			AlertType:   "integrity_health_issue",
			Severity:    "warning",
			Title:       "Integrity Health Issue",
			Description: fmt.Sprintf("Overall status: %s", report.OverallStatus),
			Details:     report,
			TriggeredAt: time.Now(),
		})
	}
}

func (s *IntegrityService) updateMetrics(checkType string, duration time.Duration, err error) {
	s.metrics.TotalChecks++
	s.metrics.TotalCheckTime += duration
	s.metrics.AverageCheckTime = s.metrics.TotalCheckTime / time.Duration(s.metrics.TotalChecks)
	s.metrics.LastUpdated = time.Now()

	if err != nil {
		s.metrics.FailedChecks++
	} else {
		s.metrics.SuccessfulChecks++
	}

	switch checkType {
	case "hash_chain":
		s.metrics.HashChainChecks++
	case "sequence":
		s.metrics.SequenceChecks++
	case "corruption":
		s.metrics.CorruptionScans++
	}

	// Report metrics to monitoring system would be implemented here when available
}
