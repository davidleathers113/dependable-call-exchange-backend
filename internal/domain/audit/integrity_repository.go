package audit

import (
	"context"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/google/uuid"
)

// IntegrityRepository defines the interface for audit data integrity verification and management
// Provides cryptographic verification, hash chain validation, and corruption detection
type IntegrityRepository interface {
	// Hash chain operations
	
	// VerifyHashChain verifies the cryptographic hash chain for a range of events
	VerifyHashChain(ctx context.Context, start, end values.SequenceNumber) (*HashChainVerificationResult, error)
	
	// VerifyHashChainIncremental performs incremental hash chain verification
	// More efficient for continuous verification of new events
	VerifyHashChainIncremental(ctx context.Context, fromSequence values.SequenceNumber) (*HashChainVerificationResult, error)
	
	// RepairHashChain attempts to repair broken hash chains where possible
	RepairHashChain(ctx context.Context, start, end values.SequenceNumber) (*HashChainRepairResult, error)
	
	// GetHashChainHead returns the head of the hash chain (latest event hash)
	GetHashChainHead(ctx context.Context) (*HashChainHead, error)
	
	// ValidateHashChainContinuity checks for gaps or overlaps in the hash chain
	ValidateHashChainContinuity(ctx context.Context) (*ChainContinuityResult, error)
	
	// Individual event integrity operations
	
	// VerifyEventHash verifies the cryptographic hash of a single event
	VerifyEventHash(ctx context.Context, eventID uuid.UUID) (*EventHashVerificationResult, error)
	
	// VerifyEventHashes verifies hashes for multiple events in batch
	VerifyEventHashes(ctx context.Context, eventIDs []uuid.UUID) (*BatchHashVerificationResult, error)
	
	// RecomputeEventHash recalculates and updates the hash for an event
	// Used for corruption repair (requires proper authorization)
	RecomputeEventHash(ctx context.Context, eventID uuid.UUID) (*HashRecomputeResult, error)
	
	// ValidateEventIntegrity performs comprehensive integrity check on a single event
	ValidateEventIntegrity(ctx context.Context, eventID uuid.UUID) (*EventIntegrityResult, error)
	
	// Sequence integrity operations
	
	// VerifySequenceIntegrity checks for sequence number integrity issues
	VerifySequenceIntegrity(ctx context.Context, criteria SequenceIntegrityCriteria) (*SequenceIntegrityResult, error)
	
	// DetectSequenceGaps identifies missing sequence numbers
	DetectSequenceGaps(ctx context.Context, start, end values.SequenceNumber) (*SequenceGapReport, error)
	
	// DetectDuplicateSequences identifies duplicate sequence numbers
	DetectDuplicateSequences(ctx context.Context, start, end values.SequenceNumber) (*DuplicateSequenceReport, error)
	
	// ValidateSequenceOrder ensures events are in correct chronological order
	ValidateSequenceOrder(ctx context.Context, criteria SequenceOrderCriteria) (*SequenceOrderResult, error)
	
	// Corruption detection and analysis
	
	// DetectCorruption scans for various types of data corruption
	DetectCorruption(ctx context.Context, criteria CorruptionDetectionCriteria) (*CorruptionReport, error)
	
	// AnalyzeCorruption provides detailed analysis of detected corruption
	AnalyzeCorruption(ctx context.Context, corruptionID string) (*CorruptionAnalysis, error)
	
	// GetCorruptionHistory returns history of corruption detection and repair
	GetCorruptionHistory(ctx context.Context, filter CorruptionHistoryFilter) (*CorruptionHistory, error)
	
	// Integrity monitoring and alerting
	
	// SetupIntegrityMonitoring configures continuous integrity monitoring
	SetupIntegrityMonitoring(ctx context.Context, config *IntegrityMonitoringConfig) error
	
	// GetIntegrityMonitoringStatus returns current monitoring status
	GetIntegrityMonitoringStatus(ctx context.Context) (*IntegrityMonitoringStatus, error)
	
	// GetIntegrityAlerts returns active integrity alerts
	GetIntegrityAlerts(ctx context.Context, filter IntegrityAlertFilter) (*IntegrityAlerts, error)
	
	// AcknowledgeIntegrityAlert marks an alert as acknowledged
	AcknowledgeIntegrityAlert(ctx context.Context, alertID string, acknowledgedBy string) error
	
	// Performance and optimization
	
	// OptimizeIntegrityChecks analyzes and optimizes integrity check performance
	OptimizeIntegrityChecks(ctx context.Context) (*IntegrityOptimizationResult, error)
	
	// GetIntegrityCheckStats returns performance statistics for integrity operations
	GetIntegrityCheckStats(ctx context.Context, timeRange TimeRange) (*IntegrityStats, error)
	
	// ScheduleIntegrityCheck schedules automated integrity verification
	ScheduleIntegrityCheck(ctx context.Context, schedule *IntegrityCheckSchedule) (string, error)
	
	// Cryptographic operations
	
	// ValidateDigitalSignatures verifies digital signatures on events (if used)
	ValidateDigitalSignatures(ctx context.Context, eventIDs []uuid.UUID) (*SignatureValidationResult, error)
	
	// GetCryptographicInfo returns cryptographic metadata for events
	GetCryptographicInfo(ctx context.Context, eventID uuid.UUID) (*CryptographicInfo, error)
	
	// RotateIntegrityKeys manages cryptographic key rotation for integrity
	RotateIntegrityKeys(ctx context.Context, keyRotationConfig *KeyRotationConfig) (*KeyRotationResult, error)
	
	// Backup and restoration verification
	
	// VerifyBackupIntegrity validates integrity of backup data
	VerifyBackupIntegrity(ctx context.Context, backupID string) (*BackupIntegrityResult, error)
	
	// VerifyRestoreIntegrity validates integrity after data restoration
	VerifyRestoreIntegrity(ctx context.Context, restoreID string) (*RestoreIntegrityResult, error)
	
	// CrossValidateWithArchive compares active data with archived data integrity
	CrossValidateWithArchive(ctx context.Context, archiveID string, criteria CrossValidationCriteria) (*CrossValidationResult, error)
	
	// Reporting and compliance
	
	// GenerateIntegrityReport creates comprehensive integrity report
	GenerateIntegrityReport(ctx context.Context, criteria IntegrityReportCriteria) (*ComprehensiveIntegrityReport, error)
	
	// GetIntegrityComplianceStatus returns compliance status for integrity requirements
	GetIntegrityComplianceStatus(ctx context.Context, standards []string) (*IntegrityComplianceStatus, error)
	
	// ExportIntegrityEvidence exports cryptographic evidence for legal purposes
	ExportIntegrityEvidence(ctx context.Context, criteria EvidenceExportCriteria) (*IntegrityEvidence, error)
}

// HashChainVerificationResult represents the result of hash chain verification
type HashChainVerificationResult struct {
	// Verification scope
	StartSequence     values.SequenceNumber `json:"start_sequence"`
	EndSequence       values.SequenceNumber `json:"end_sequence"`
	EventsVerified    int64                 `json:"events_verified"`
	
	// Overall result
	IsValid           bool                  `json:"is_valid"`
	IntegrityScore    float64               `json:"integrity_score"` // 0.0 to 1.0
	
	// Chain analysis
	ChainComplete     bool                  `json:"chain_complete"`
	HashesValid       int64                 `json:"hashes_valid"`
	HashesInvalid     int64                 `json:"hashes_invalid"`
	HashesMissing     int64                 `json:"hashes_missing"`
	
	// Broken chain information
	BrokenChains      []*BrokenChain        `json:"broken_chains,omitempty"`
	FirstBrokenAt     *values.SequenceNumber `json:"first_broken_at,omitempty"`
	
	// Performance metrics
	VerificationTime  time.Duration         `json:"verification_time"`
	EventsPerSecond   float64               `json:"events_per_second"`
	
	// Detailed issues
	Issues            []*ChainIntegrityIssue `json:"issues,omitempty"`
	Warnings          []string              `json:"warnings,omitempty"`
	
	// Verification metadata
	VerifiedAt        time.Time             `json:"verified_at"`
	VerificationID    string                `json:"verification_id"`
	Method            string                `json:"method"` // full, incremental, sample
}

// BrokenChain represents a break in the hash chain
type BrokenChain struct {
	StartSequence     values.SequenceNumber `json:"start_sequence"`
	EndSequence       values.SequenceNumber `json:"end_sequence"`
	BreakType         string                `json:"break_type"` // hash_mismatch, missing_event, invalid_hash
	ExpectedHash      string                `json:"expected_hash"`
	ActualHash        string                `json:"actual_hash"`
	AffectedEvents    []uuid.UUID           `json:"affected_events"`
	Severity          string                `json:"severity"` // low, medium, high, critical
	RepairPossible    bool                  `json:"repair_possible"`
	RepairEstimate    time.Duration         `json:"repair_estimate,omitempty"`
}

// ChainIntegrityIssue represents a specific integrity issue in the chain
type ChainIntegrityIssue struct {
	IssueID           string                `json:"issue_id"`
	Type              string                `json:"type"`
	Severity          string                `json:"severity"`
	EventID           uuid.UUID             `json:"event_id"`
	Sequence          values.SequenceNumber `json:"sequence"`
	Description       string                `json:"description"`
	Impact            string                `json:"impact"`
	Recommendation    string                `json:"recommendation"`
	AutoRepair        bool                  `json:"auto_repair"`
}

// HashChainRepairResult represents the result of hash chain repair
type HashChainRepairResult struct {
	RepairID          string                `json:"repair_id"`
	RepairScope       SequenceRange         `json:"repair_scope"`
	
	// Repair results
	EventsRepaired    int64                 `json:"events_repaired"`
	EventsSkipped     int64                 `json:"events_skipped"`
	EventsFailed      int64                 `json:"events_failed"`
	
	// Hash operations
	HashesRecalculated int64                `json:"hashes_recalculated"`
	ChainLinksRepaired int64                `json:"chain_links_repaired"`
	
	// Repair details
	RepairActions     []*RepairAction       `json:"repair_actions"`
	UnrepairableIssues []*UnrepairableIssue `json:"unrepairable_issues,omitempty"`
	
	// Performance metrics
	RepairTime        time.Duration         `json:"repair_time"`
	
	// Verification after repair
	PostRepairVerification *HashChainVerificationResult `json:"post_repair_verification,omitempty"`
	
	// Metadata
	RepairedAt        time.Time             `json:"repaired_at"`
	RepairedBy        string                `json:"repaired_by"`
	RepairReason      string                `json:"repair_reason"`
}

// RepairAction represents a single repair operation
type RepairAction struct {
	ActionType        string                `json:"action_type"` // recalculate_hash, rebuild_chain, skip_event
	EventID           uuid.UUID             `json:"event_id"`
	Sequence          values.SequenceNumber `json:"sequence"`
	OldHash           string                `json:"old_hash,omitempty"`
	NewHash           string                `json:"new_hash,omitempty"`
	Success           bool                  `json:"success"`
	Error             string                `json:"error,omitempty"`
}

// UnrepairableIssue represents an issue that couldn't be automatically repaired
type UnrepairableIssue struct {
	EventID           uuid.UUID             `json:"event_id"`
	Sequence          values.SequenceNumber `json:"sequence"`
	IssueType         string                `json:"issue_type"`
	Reason            string                `json:"reason"`
	RequiresManualAction bool               `json:"requires_manual_action"`
	Recommendation    string                `json:"recommendation"`
}

// HashChainHead represents the head of the hash chain
type HashChainHead struct {
	LatestSequence    values.SequenceNumber `json:"latest_sequence"`
	LatestEventID     uuid.UUID             `json:"latest_event_id"`
	LatestHash        string                `json:"latest_hash"`
	PreviousHash      string                `json:"previous_hash"`
	ChainLength       int64                 `json:"chain_length"`
	LastUpdated       time.Time             `json:"last_updated"`
	IsHealthy         bool                  `json:"is_healthy"`
	HealthCheckTime   time.Time             `json:"health_check_time"`
}

// SequenceIntegrityCriteria defines criteria for sequence integrity verification
type SequenceIntegrityCriteria struct {
	// Scope
	StartSequence     *values.SequenceNumber `json:"start_sequence,omitempty"`
	EndSequence       *values.SequenceNumber `json:"end_sequence,omitempty"`
	TimeRange         *TimeRange            `json:"time_range,omitempty"`
	
	// Check types
	CheckGaps         bool                  `json:"check_gaps"`
	CheckDuplicates   bool                  `json:"check_duplicates"`
	CheckOrder        bool                  `json:"check_order"`
	CheckContinuity   bool                  `json:"check_continuity"`
	
	// Performance options
	SampleSize        int                   `json:"sample_size,omitempty"` // For large datasets
	MaxTime           time.Duration         `json:"max_time,omitempty"`
	ParallelCheck     bool                  `json:"parallel_check"`
}

// SequenceIntegrityResult represents sequence integrity verification results
type SequenceIntegrityResult struct {
	// Verification scope
	Scope             SequenceIntegrityCriteria `json:"scope"`
	SequencesChecked  int64                     `json:"sequences_checked"`
	
	// Overall status
	IsValid           bool                      `json:"is_valid"`
	IntegrityScore    float64                   `json:"integrity_score"`
	
	// Specific findings
	GapsFound         int64                     `json:"gaps_found"`
	DuplicatesFound   int64                     `json:"duplicates_found"`
	OrderViolations   int64                     `json:"order_violations"`
	
	// Detailed results
	Gaps              []*SequenceGap            `json:"gaps,omitempty"`
	Duplicates        []*DuplicateSequence      `json:"duplicates,omitempty"`
	OrderIssues       []*SequenceOrderIssue     `json:"order_issues,omitempty"`
	
	// Performance metrics
	CheckTime         time.Duration             `json:"check_time"`
	CheckedAt         time.Time                 `json:"checked_at"`
	
	// Recommendations
	Recommendations   []string                  `json:"recommendations,omitempty"`
}

// SequenceGap represents a gap in sequence numbers
type SequenceGap struct {
	GapID             string                `json:"gap_id"`
	StartSequence     values.SequenceNumber `json:"start_sequence"`
	EndSequence       values.SequenceNumber `json:"end_sequence"`
	GapSize           int64                 `json:"gap_size"`
	ExpectedEvents    int64                 `json:"expected_events"`
	Severity          string                `json:"severity"`
	PossibleCause     string                `json:"possible_cause"`
	RepairAction      string                `json:"repair_action,omitempty"`
}

// DuplicateSequence represents duplicate sequence numbers
type DuplicateSequence struct {
	Sequence          values.SequenceNumber `json:"sequence"`
	EventIDs          []uuid.UUID           `json:"event_ids"`
	Timestamps        []time.Time           `json:"timestamps"`
	Severity          string                `json:"severity"`
	Resolution        string                `json:"resolution"`
}

// SequenceOrderIssue represents chronological ordering issues
type SequenceOrderIssue struct {
	EventID           uuid.UUID             `json:"event_id"`
	Sequence          values.SequenceNumber `json:"sequence"`
	EventTimestamp    time.Time             `json:"event_timestamp"`
	ExpectedAfter     *time.Time            `json:"expected_after,omitempty"`
	ExpectedBefore    *time.Time            `json:"expected_before,omitempty"`
	Severity          string                `json:"severity"`
	Impact            string                `json:"impact"`
}

// CorruptionDetectionCriteria defines criteria for corruption detection
type CorruptionDetectionCriteria struct {
	// Scope
	StartTime         *time.Time            `json:"start_time,omitempty"`
	EndTime           *time.Time            `json:"end_time,omitempty"`
	StartSequence     *values.SequenceNumber `json:"start_sequence,omitempty"`
	EndSequence       *values.SequenceNumber `json:"end_sequence,omitempty"`
	
	// Detection types
	CheckHashes       bool                  `json:"check_hashes"`
	CheckMetadata     bool                  `json:"check_metadata"`
	CheckReferences   bool                  `json:"check_references"`
	CheckEncoding     bool                  `json:"check_encoding"`
	CheckConsistency  bool                  `json:"check_consistency"`
	
	// Detection sensitivity
	DeepScan          bool                  `json:"deep_scan"`
	StatisticalAnalysis bool                `json:"statistical_analysis"`
	PatternAnalysis   bool                  `json:"pattern_analysis"`
	
	// Performance options
	MaxEvents         int64                 `json:"max_events,omitempty"`
	SampleRate        float64               `json:"sample_rate,omitempty"`
	Timeout           time.Duration         `json:"timeout,omitempty"`
}

// CorruptionReport represents detected corruption issues
type CorruptionReport struct {
	ReportID          string                `json:"report_id"`
	Criteria          CorruptionDetectionCriteria `json:"criteria"`
	
	// Overall status
	CorruptionFound   bool                  `json:"corruption_found"`
	CorruptionLevel   string                `json:"corruption_level"` // none, low, medium, high, severe
	EventsScanned     int64                 `json:"events_scanned"`
	EventsCorrupted   int64                 `json:"events_corrupted"`
	
	// Corruption details
	Corruptions       []*CorruptionInstance `json:"corruptions"`
	
	// Analysis summary
	CorruptionTypes   map[string]int64      `json:"corruption_types"`
	SeverityBreakdown map[string]int64      `json:"severity_breakdown"`
	
	// Performance metrics
	ScanTime          time.Duration         `json:"scan_time"`
	ScannedAt         time.Time             `json:"scanned_at"`
	
	// Recommendations
	ImmediateActions  []string              `json:"immediate_actions,omitempty"`
	Recommendations   []string              `json:"recommendations,omitempty"`
}

// CorruptionInstance represents a single instance of corruption
type CorruptionInstance struct {
	CorruptionID      string                `json:"corruption_id"`
	EventID           uuid.UUID             `json:"event_id"`
	Sequence          values.SequenceNumber `json:"sequence"`
	CorruptionType    string                `json:"corruption_type"`
	Severity          string                `json:"severity"`
	Field             string                `json:"field,omitempty"`
	ExpectedValue     string                `json:"expected_value,omitempty"`
	ActualValue       string                `json:"actual_value,omitempty"`
	DetectedAt        time.Time             `json:"detected_at"`
	Impact            string                `json:"impact"`
	RepairPossible    bool                  `json:"repair_possible"`
	RepairComplexity  string                `json:"repair_complexity,omitempty"`
}

// IntegrityMonitoringConfig configures continuous integrity monitoring
type IntegrityMonitoringConfig struct {
	// Monitoring scope
	MonitorAll        bool                  `json:"monitor_all"`
	MonitorTypes      []EventType           `json:"monitor_types,omitempty"`
	MonitorCategories []string              `json:"monitor_categories,omitempty"`
	
	// Check intervals
	HashChainInterval    time.Duration      `json:"hash_chain_interval"`
	SequenceInterval     time.Duration      `json:"sequence_interval"`
	CorruptionInterval   time.Duration      `json:"corruption_interval"`
	
	// Alert thresholds
	HashFailureThreshold    float64         `json:"hash_failure_threshold"`
	SequenceGapThreshold    int64           `json:"sequence_gap_threshold"`
	CorruptionThreshold     float64         `json:"corruption_threshold"`
	
	// Alert configuration
	EnableAlerts         bool               `json:"enable_alerts"`
	AlertChannels        []string           `json:"alert_channels"` // email, slack, webhook
	AlertSeverities      []string           `json:"alert_severities"`
	
	// Performance configuration
	MaxConcurrentChecks  int                `json:"max_concurrent_checks"`
	CheckTimeout         time.Duration      `json:"check_timeout"`
	SampleSize           int                `json:"sample_size,omitempty"`
	
	// Retention
	HistoryRetentionDays int                `json:"history_retention_days"`
	AlertRetentionDays   int                `json:"alert_retention_days"`
}

// Additional types would continue here for completeness...
// Including IntegrityMonitoringStatus, IntegrityAlerts, etc.