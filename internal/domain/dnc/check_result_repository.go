package dnc

import (
	"context"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/google/uuid"
)

// DNCCheckResultRepository defines the interface for DNC check result persistence operations
// Performance expectations:
// - Save: < 5ms for single result (hot path for call routing)
// - FindByPhone: < 2ms with proper caching integration
// - FindRecent: < 10ms for compliance audit trails
// - Cleanup: > 10K results/second for data retention
type DNCCheckResultRepository interface {
	// Core persistence operations
	
	// Save persists a DNC check result for caching and audit purposes
	// This is called on every DNC check - must be optimized for sub-5ms latency
	Save(ctx context.Context, result *DNCCheckResult) error
	
	// SaveWithTx saves a DNC check result within an existing transaction
	SaveWithTx(ctx context.Context, tx Transaction, result *DNCCheckResult) error
	
	// GetByID retrieves a check result by its unique identifier
	GetByID(ctx context.Context, id uuid.UUID) (*DNCCheckResult, error)
	
	// Phone number lookup operations (hot path for compliance)
	
	// FindByPhone retrieves recent check results for a specific phone number
	// Used for compliance auditing and debugging - expected latency < 2ms
	FindByPhone(ctx context.Context, phoneNumber values.PhoneNumber) ([]*DNCCheckResult, error)
	
	// FindLatestByPhone retrieves the most recent check result for a phone number
	// Used for cache validation and compliance verification
	FindLatestByPhone(ctx context.Context, phoneNumber values.PhoneNumber) (*DNCCheckResult, error)
	
	// FindValidCachedResult retrieves a cached result that is still within TTL
	// This is the primary cache lookup method - must be sub-millisecond
	FindValidCachedResult(ctx context.Context, phoneNumber values.PhoneNumber) (*DNCCheckResult, error)
	
	// Time-based queries for compliance and auditing
	
	// FindRecent retrieves check results within a specified time range
	// Used for compliance reporting and audit trails
	FindRecent(ctx context.Context, since time.Time, limit int) ([]*DNCCheckResult, error)
	
	// FindByTimeRange retrieves check results within a specific time window
	FindByTimeRange(ctx context.Context, start, end time.Time, filter DNCCheckFilter) ([]*DNCCheckResult, error)
	
	// FindBlockedCalls retrieves check results that resulted in blocked calls
	// Critical for compliance reporting and violation tracking
	FindBlockedCalls(ctx context.Context, timeRange TimeRange) ([]*DNCCheckResult, error)
	
	// FindByCompliance retrieves results filtered by compliance criteria
	FindByCompliance(ctx context.Context, filter ComplianceFilter) ([]*DNCCheckResult, error)
	
	// Cache management operations
	
	// GetCacheStats retrieves caching performance statistics
	GetCacheStats(ctx context.Context) (*CacheStats, error)
	
	// InvalidatePhoneCache invalidates all cached results for a phone number
	// Called when DNC list updates affect a specific number
	InvalidatePhoneCache(ctx context.Context, phoneNumber values.PhoneNumber) error
	
	// InvalidateProviderCache invalidates cached results from a specific provider
	// Called during provider sync operations
	InvalidateProviderCache(ctx context.Context, providerID uuid.UUID) error
	
	// RefreshCache rebuilds cache entries for performance optimization
	RefreshCache(ctx context.Context, phoneNumbers []values.PhoneNumber) error
	
	// Cleanup and maintenance operations
	
	// Cleanup removes expired check results based on TTL and retention policies
	// Expected throughput: > 10K results/second for efficient data retention
	Cleanup(ctx context.Context, retentionPolicy RetentionPolicy) (*CleanupResult, error)
	
	// CleanupExpired removes results that have exceeded their TTL
	CleanupExpired(ctx context.Context, before time.Time) (int64, error)
	
	// CleanupByAge removes results older than specified age
	CleanupByAge(ctx context.Context, maxAge time.Duration) (int64, error)
	
	// ArchiveOldResults moves old results to archive storage
	ArchiveOldResults(ctx context.Context, archivePolicy ArchivePolicy) (*ArchiveResult, error)
	
	// Bulk operations for performance
	
	// BulkSave saves multiple check results efficiently
	BulkSave(ctx context.Context, results []*DNCCheckResult) error
	
	// BulkDelete removes multiple check results
	BulkDelete(ctx context.Context, resultIDs []uuid.UUID) error
	
	// BulkInvalidate invalidates multiple cache entries
	BulkInvalidate(ctx context.Context, phoneNumbers []values.PhoneNumber) error
	
	// Query and filtering operations
	
	// Find searches for check results based on filter criteria with pagination
	Find(ctx context.Context, filter DNCCheckFilter) (*DNCCheckResultPage, error)
	
	// Count returns the total number of results matching the filter
	Count(ctx context.Context, filter DNCCheckFilter) (int64, error)
	
	// FindByRiskScore retrieves results by risk score range
	FindByRiskScore(ctx context.Context, minScore, maxScore float64, limit int) ([]*DNCCheckResult, error)
	
	// FindByDecision retrieves results by compliance decision
	FindByDecision(ctx context.Context, decision string, timeRange TimeRange) ([]*DNCCheckResult, error)
	
	// Analytics and reporting operations
	
	// GetCheckMetrics retrieves aggregated check result metrics
	GetCheckMetrics(ctx context.Context, timeRange TimeRange) (*CheckMetrics, error)
	
	// GetComplianceReport generates a compliance report for a time period
	GetComplianceReport(ctx context.Context, reportCriteria ComplianceReportCriteria) (*ComplianceReport, error)
	
	// GetPerformanceMetrics retrieves performance metrics for check operations
	GetPerformanceMetrics(ctx context.Context, timeRange TimeRange) (*PerformanceMetrics, error)
	
	// GetTrendAnalysis provides trend analysis for DNC check patterns
	GetTrendAnalysis(ctx context.Context, analysis TrendAnalysisRequest) (*TrendAnalysis, error)
	
	// Administrative operations
	
	// GetStats returns repository performance and usage statistics
	GetStats(ctx context.Context) (*DNCCheckResultStats, error)
	
	// Vacuum performs database maintenance operations
	Vacuum(ctx context.Context) error
	
	// ValidateIntegrity performs integrity checks on check result data
	ValidateIntegrity(ctx context.Context) (*CheckResultIntegrityReport, error)
	
	// Transaction support
	
	// BeginTx starts a new database transaction
	BeginTx(ctx context.Context) (Transaction, error)
	
	// WithTx executes a function within a database transaction
	WithTx(ctx context.Context, fn func(tx Transaction) error) error
}

// DNCCheckFilter defines filtering options for DNC check result queries
type DNCCheckFilter struct {
	// Phone number filters
	PhoneNumbers    []values.PhoneNumber `json:"phone_numbers,omitempty"`
	PhonePattern    *string              `json:"phone_pattern,omitempty"`
	
	// Result filters
	IsBlocked       *bool                `json:"is_blocked,omitempty"`
	Decisions       []string             `json:"decisions,omitempty"` // Compliance decisions
	RiskScoreMin    *float64             `json:"risk_score_min,omitempty"`
	RiskScoreMax    *float64             `json:"risk_score_max,omitempty"`
	
	// Source filters
	Sources         []ListSource         `json:"sources,omitempty"`
	ProviderIDs     []uuid.UUID          `json:"provider_ids,omitempty"`
	ComplianceLevels []string            `json:"compliance_levels,omitempty"`
	
	// Time range filters
	CheckedAfter    *time.Time           `json:"checked_after,omitempty"`
	CheckedBefore   *time.Time           `json:"checked_before,omitempty"`
	
	// Performance filters
	DurationMin     *time.Duration       `json:"duration_min,omitempty"`
	DurationMax     *time.Duration       `json:"duration_max,omitempty"`
	SourcesCountMin *int                 `json:"sources_count_min,omitempty"`
	SourcesCountMax *int                 `json:"sources_count_max,omitempty"`
	
	// Cache filters
	OnlyExpired     *bool                `json:"only_expired,omitempty"`
	OnlyValid       *bool                `json:"only_valid,omitempty"`
	TTLMin          *time.Duration       `json:"ttl_min,omitempty"`
	TTLMax          *time.Duration       `json:"ttl_max,omitempty"`
	
	// Block reason filters
	SuppressReasons []SuppressReason     `json:"suppress_reasons,omitempty"`
	Severities      []string             `json:"severities,omitempty"`
	ComplianceCodes []string             `json:"compliance_codes,omitempty"`
	
	// Metadata filters
	MetadataKeys    []string             `json:"metadata_keys,omitempty"`
	MetadataValues  map[string]string    `json:"metadata_values,omitempty"`
	
	// Search
	SearchText      *string              `json:"search_text,omitempty"`
	
	// Pagination
	Limit           int                  `json:"limit,omitempty"`
	Offset          int                  `json:"offset,omitempty"`
	Cursor          string               `json:"cursor,omitempty"`
	
	// Sorting
	OrderBy         string               `json:"order_by,omitempty"`
	OrderDesc       bool                 `json:"order_desc,omitempty"`
	
	// Performance options
	IncludeReasons  bool                 `json:"include_reasons,omitempty"`
	IncludeMetadata bool                 `json:"include_metadata,omitempty"`
}

// ComplianceFilter defines compliance-specific filtering options
type ComplianceFilter struct {
	// Regulatory compliance
	RequiresDocumentation *bool               `json:"requires_documentation,omitempty"`
	ComplianceCodes      []string             `json:"compliance_codes,omitempty"`
	ViolationTypes       []string             `json:"violation_types,omitempty"`
	
	// Risk assessment
	RiskLevels           []string             `json:"risk_levels,omitempty"`
	MinPenaltyAmount     *int                 `json:"min_penalty_amount,omitempty"`
	
	// Time-sensitive compliance
	TimeRange            TimeRange            `json:"time_range"`
	BusinessHours        *bool                `json:"business_hours,omitempty"`
	
	// Audit requirements
	AuditTrailRequired   *bool                `json:"audit_trail_required,omitempty"`
	RetentionRequired    *bool                `json:"retention_required,omitempty"`
}

// DNCCheckResultPage represents a paginated result set of check results
type DNCCheckResultPage struct {
	Results    []*DNCCheckResult `json:"results"`
	TotalCount int64             `json:"total_count"`
	HasMore    bool              `json:"has_more"`
	NextCursor string            `json:"next_cursor,omitempty"`
	
	// Performance metadata
	QueryTime    time.Duration `json:"query_time"`
	DatabaseHits int           `json:"database_hits"`
	CacheHits    int           `json:"cache_hits"`
	CacheMisses  int           `json:"cache_misses"`
}

// CacheStats provides caching performance statistics
type CacheStats struct {
	// Hit/miss statistics
	TotalRequests   int64     `json:"total_requests"`
	CacheHits       int64     `json:"cache_hits"`
	CacheMisses     int64     `json:"cache_misses"`
	HitRate         float64   `json:"hit_rate"`
	
	// Performance metrics
	AvgHitTime      time.Duration `json:"avg_hit_time"`
	AvgMissTime     time.Duration `json:"avg_miss_time"`
	
	// Cache size and efficiency
	TotalEntries    int64     `json:"total_entries"`
	ValidEntries    int64     `json:"valid_entries"`
	ExpiredEntries  int64     `json:"expired_entries"`
	MemoryUsedMB    float64   `json:"memory_used_mb"`
	
	// Eviction statistics
	EvictionsTotal  int64     `json:"evictions_total"`
	EvictionsHour   int64     `json:"evictions_hour"`
	
	// Time-based metrics
	TotalCacheTime  time.Duration `json:"total_cache_time"`
	AverageEntryAge time.Duration `json:"average_entry_age"`
	
	// Collection timestamp
	CollectedAt     time.Time `json:"collected_at"`
}

// RetentionPolicy defines data retention rules
type RetentionPolicy struct {
	// Age-based retention
	MaxAge              time.Duration `json:"max_age"`
	MaxAgeBlocked       time.Duration `json:"max_age_blocked"`   // Longer retention for blocked calls
	MaxAgeCompliance    time.Duration `json:"max_age_compliance"` // Extended for compliance
	
	// Count-based retention
	MaxRecordsPerPhone  int           `json:"max_records_per_phone"`
	MaxTotalRecords     int64         `json:"max_total_records"`
	
	// Condition-based retention
	RetainBlocked       bool          `json:"retain_blocked"`       // Keep blocked call results longer
	RetainHighRisk      bool          `json:"retain_high_risk"`     // Keep high risk scores longer
	RetainCompliance    bool          `json:"retain_compliance"`    // Keep compliance-relevant results
	
	// Execution preferences
	BatchSize           int           `json:"batch_size"`
	MaxExecutionTime    time.Duration `json:"max_execution_time"`
	PreserveReferences  bool          `json:"preserve_references"`  // Keep if referenced by audit
}

// CleanupResult contains the results of a cleanup operation
type CleanupResult struct {
	StartedAt        time.Time     `json:"started_at"`
	CompletedAt      time.Time     `json:"completed_at"`
	Duration         time.Duration `json:"duration"`
	
	// Records processed
	RecordsExamined  int64         `json:"records_examined"`
	RecordsDeleted   int64         `json:"records_deleted"`
	RecordsRetained  int64         `json:"records_retained"`
	RecordsArchived  int64         `json:"records_archived"`
	
	// Space reclaimed
	SpaceReclaimed   int64         `json:"space_reclaimed_bytes"`
	IndexesRebuilt   int           `json:"indexes_rebuilt"`
	
	// Error handling
	ErrorsEncountered int          `json:"errors_encountered"`
	PartialFailures  []string      `json:"partial_failures,omitempty"`
	
	// Performance metrics
	ThroughputPerSecond float64    `json:"throughput_per_second"`
	MemoryUsedMB       float64     `json:"memory_used_mb"`
}

// ArchivePolicy defines rules for archiving old data
type ArchivePolicy struct {
	// Archive criteria
	ArchiveAge          time.Duration `json:"archive_age"`
	ArchiveAfterCount   int64         `json:"archive_after_count"`
	
	// Archive destination
	ArchiveLocation     string        `json:"archive_location"`
	CompressionEnabled  bool          `json:"compression_enabled"`
	EncryptionEnabled   bool          `json:"encryption_enabled"`
	
	// Verification
	VerifyIntegrity     bool          `json:"verify_integrity"`
	CreateManifest      bool          `json:"create_manifest"`
	
	// Performance
	BatchSize           int           `json:"batch_size"`
	MaxExecutionTime    time.Duration `json:"max_execution_time"`
	ParallelWorkers     int           `json:"parallel_workers"`
}

// ArchiveResult contains the results of an archive operation
type ArchiveResult struct {
	StartedAt           time.Time     `json:"started_at"`
	CompletedAt         time.Time     `json:"completed_at"`
	Duration            time.Duration `json:"duration"`
	
	// Archive statistics
	RecordsArchived     int64         `json:"records_archived"`
	ArchiveSizeBytes    int64         `json:"archive_size_bytes"`
	CompressionRatio    float64       `json:"compression_ratio"`
	
	// Verification results
	IntegrityVerified   bool          `json:"integrity_verified"`
	ManifestCreated     bool          `json:"manifest_created"`
	ArchiveLocation     string        `json:"archive_location"`
	
	// Performance metrics
	ThroughputPerSecond float64       `json:"throughput_per_second"`
	NetworkBytesOut     int64         `json:"network_bytes_out"`
	
	// Error handling
	ErrorsEncountered   int           `json:"errors_encountered"`
	FailedRecords       []uuid.UUID   `json:"failed_records,omitempty"`
}

// CheckMetrics provides aggregated metrics for DNC check operations
type CheckMetrics struct {
	TimeRange           TimeRange     `json:"time_range"`
	
	// Volume metrics
	TotalChecks         int64         `json:"total_checks"`
	UniquePhones        int64         `json:"unique_phones"`
	BlockedCalls        int64         `json:"blocked_calls"`
	AllowedCalls        int64         `json:"allowed_calls"`
	BlockRate           float64       `json:"block_rate"`
	
	// Performance metrics
	AvgCheckDuration    time.Duration `json:"avg_check_duration"`
	MedianCheckDuration time.Duration `json:"median_check_duration"`
	P95CheckDuration    time.Duration `json:"p95_check_duration"`
	P99CheckDuration    time.Duration `json:"p99_check_duration"`
	
	// Cache performance
	CacheHitRate        float64       `json:"cache_hit_rate"`
	AvgSourcesChecked   float64       `json:"avg_sources_checked"`
	
	// Risk analysis
	AvgRiskScore        float64       `json:"avg_risk_score"`
	HighRiskChecks      int64         `json:"high_risk_checks"`
	CriticalChecks      int64         `json:"critical_checks"`
	
	// Compliance breakdown
	ComplianceByLevel   map[string]int64 `json:"compliance_by_level"`
	ViolationsByType    map[string]int64 `json:"violations_by_type"`
	SourceBreakdown     map[string]int64 `json:"source_breakdown"`
	
	// Trend data
	HourlyVolume        []HourlyMetric   `json:"hourly_volume,omitempty"`
	DailyVolume         []DailyMetric    `json:"daily_volume,omitempty"`
}

// HourlyMetric contains hourly aggregated metrics
type HourlyMetric struct {
	Hour        time.Time `json:"hour"`
	CheckCount  int64     `json:"check_count"`
	BlockCount  int64     `json:"block_count"`
	BlockRate   float64   `json:"block_rate"`
	AvgDuration time.Duration `json:"avg_duration"`
}

// ComplianceReport provides comprehensive compliance analysis
type ComplianceReport struct {
	ReportID            uuid.UUID     `json:"report_id"`
	GeneratedAt         time.Time     `json:"generated_at"`
	TimeRange           TimeRange     `json:"time_range"`
	Criteria            ComplianceReportCriteria `json:"criteria"`
	
	// Summary statistics
	TotalCallAttempts   int64         `json:"total_call_attempts"`
	CompliantCalls      int64         `json:"compliant_calls"`
	NonCompliantCalls   int64         `json:"non_compliant_calls"`
	ComplianceRate      float64       `json:"compliance_rate"`
	
	// Violation analysis
	ViolationsByType    map[string]ViolationSummary `json:"violations_by_type"`
	ViolationsBySeverity map[string]int64           `json:"violations_by_severity"`
	TotalPenaltyRisk    int64                      `json:"total_penalty_risk"`
	
	// DNC list analysis
	DNCSources          []DNCSummary  `json:"dnc_sources"`
	NewDNCEntries       int64         `json:"new_dnc_entries"`
	ExpiredDNCEntries   int64         `json:"expired_dnc_entries"`
	
	// Audit trail
	AuditEvents         int64         `json:"audit_events"`
	DocumentationGaps   []string      `json:"documentation_gaps,omitempty"`
	
	// Recommendations
	ComplianceGrade     string        `json:"compliance_grade"` // A, B, C, D, F
	Recommendations     []string      `json:"recommendations"`
	ActionItems         []ActionItem  `json:"action_items"`
	
	// Attestation
	ReportIntegrity     string        `json:"report_integrity"` // Hash for verification
	GeneratedBy         uuid.UUID     `json:"generated_by"`
	ReviewRequired      bool          `json:"review_required"`
}

// ComplianceReportCriteria defines criteria for compliance reporting
type ComplianceReportCriteria struct {
	TimeRange           TimeRange     `json:"time_range"`
	IncludeViolations   bool          `json:"include_violations"`
	IncludeAuditTrail   bool          `json:"include_audit_trail"`
	IncludeRecommendations bool       `json:"include_recommendations"`
	DetailLevel         string        `json:"detail_level"` // summary, detailed, comprehensive
	RegulationTypes     []string      `json:"regulation_types"` // TCPA, state, etc.
	
	// Filtering
	BusinessUnits       []string      `json:"business_units,omitempty"`
	CallTypes           []string      `json:"call_types,omitempty"`
	GeographicRegions   []string      `json:"geographic_regions,omitempty"`
}

// ViolationSummary contains summarized violation information
type ViolationSummary struct {
	ViolationType       string        `json:"violation_type"`
	Count               int64         `json:"count"`
	PenaltyRisk         int64         `json:"penalty_risk"`
	AverageRiskScore    float64       `json:"average_risk_score"`
	MostRecentViolation time.Time     `json:"most_recent_violation"`
	ExampleCases        []uuid.UUID   `json:"example_cases,omitempty"`
}

// DNCSummary contains DNC list summary information
type DNCSummary struct {
	Source              string        `json:"source"`
	ProviderName        string        `json:"provider_name"`
	EntriesChecked      int64         `json:"entries_checked"`
	BlocksTriggered     int64         `json:"blocks_triggered"`
	LastUpdated         time.Time     `json:"last_updated"`
	HealthStatus        string        `json:"health_status"`
}

// ActionItem represents a compliance action item
type ActionItem struct {
	ID                  uuid.UUID     `json:"id"`
	Priority            string        `json:"priority"` // critical, high, medium, low
	Description         string        `json:"description"`
	DueDate             *time.Time    `json:"due_date,omitempty"`
	AssignedTo          *uuid.UUID    `json:"assigned_to,omitempty"`
	Category            string        `json:"category"` // training, process, technology
	EstimatedEffort     string        `json:"estimated_effort"`
	RegulatoryImpact    string        `json:"regulatory_impact"`
}

// PerformanceMetrics provides performance analysis for DNC operations
type PerformanceMetrics struct {
	TimeRange           TimeRange     `json:"time_range"`
	
	// Response time metrics
	AvgResponseTime     time.Duration `json:"avg_response_time"`
	MedianResponseTime  time.Duration `json:"median_response_time"`
	P95ResponseTime     time.Duration `json:"p95_response_time"`
	P99ResponseTime     time.Duration `json:"p99_response_time"`
	MaxResponseTime     time.Duration `json:"max_response_time"`
	
	// Throughput metrics
	RequestsPerSecond   float64       `json:"requests_per_second"`
	PeakRPS             float64       `json:"peak_rps"`
	ConcurrentChecks    int           `json:"concurrent_checks"`
	
	// Cache performance
	CacheHitRate        float64       `json:"cache_hit_rate"`
	CacheResponseTime   time.Duration `json:"cache_response_time"`
	CacheMissResponseTime time.Duration `json:"cache_miss_response_time"`
	
	// Resource utilization
	DatabaseConnections int           `json:"database_connections"`
	DatabaseResponseTime time.Duration `json:"database_response_time"`
	MemoryUsageMB       float64       `json:"memory_usage_mb"`
	CPUUtilization      float64       `json:"cpu_utilization"`
	
	// Error rates
	ErrorRate           float64       `json:"error_rate"`
	TimeoutRate         float64       `json:"timeout_rate"`
	RetryRate           float64       `json:"retry_rate"`
	
	// Performance by provider
	ProviderMetrics     []ProviderPerformance `json:"provider_metrics"`
}

// ProviderPerformance contains performance metrics for a specific provider
type ProviderPerformance struct {
	ProviderID          uuid.UUID     `json:"provider_id"`
	ProviderName        string        `json:"provider_name"`
	ResponseTime        time.Duration `json:"response_time"`
	RequestCount        int64         `json:"request_count"`
	ErrorRate           float64       `json:"error_rate"`
	AvailabilityRate    float64       `json:"availability_rate"`
}

// TrendAnalysisRequest defines parameters for trend analysis
type TrendAnalysisRequest struct {
	TimeRange           TimeRange     `json:"time_range"`
	Granularity         string        `json:"granularity"` // hour, day, week, month
	Metrics             []string      `json:"metrics"`     // volume, block_rate, performance
	IncludePredictions  bool          `json:"include_predictions"`
	SegmentBy           []string      `json:"segment_by"`  // source, provider, compliance_level
}

// TrendAnalysis provides trend analysis results
type TrendAnalysis struct {
	Request             TrendAnalysisRequest `json:"request"`
	GeneratedAt         time.Time            `json:"generated_at"`
	
	// Trend data points
	DataPoints          []TrendDataPoint     `json:"data_points"`
	
	// Analysis results
	TrendDirection      string               `json:"trend_direction"` // increasing, decreasing, stable
	ChangeRate          float64              `json:"change_rate"`     // Percentage change
	Seasonality         *SeasonalityAnalysis `json:"seasonality,omitempty"`
	
	// Predictions (if requested)
	Predictions         []PredictionPoint    `json:"predictions,omitempty"`
	ConfidenceLevel     float64              `json:"confidence_level,omitempty"`
	
	// Insights
	KeyInsights         []string             `json:"key_insights"`
	Anomalies           []AnomalyPoint       `json:"anomalies,omitempty"`
	Recommendations     []string             `json:"recommendations"`
}

// TrendDataPoint represents a single data point in trend analysis
type TrendDataPoint struct {
	Timestamp           time.Time            `json:"timestamp"`
	Metrics             map[string]float64   `json:"metrics"`
	SegmentData         map[string]float64   `json:"segment_data,omitempty"`
}

// SeasonalityAnalysis contains seasonality pattern analysis
type SeasonalityAnalysis struct {
	HasSeasonality      bool                 `json:"has_seasonality"`
	Period              time.Duration        `json:"period"`
	Strength            float64              `json:"strength"`
	PeakTimes           []time.Time          `json:"peak_times"`
	LowTimes            []time.Time          `json:"low_times"`
}

// PredictionPoint represents a predicted data point
type PredictionPoint struct {
	Timestamp           time.Time            `json:"timestamp"`
	PredictedValue      float64              `json:"predicted_value"`
	ConfidenceInterval  ConfidenceInterval   `json:"confidence_interval"`
}

// ConfidenceInterval represents prediction confidence bounds
type ConfidenceInterval struct {
	Lower               float64              `json:"lower"`
	Upper               float64              `json:"upper"`
}

// AnomalyPoint represents an anomalous data point
type AnomalyPoint struct {
	Timestamp           time.Time            `json:"timestamp"`
	Value               float64              `json:"value"`
	ExpectedValue       float64              `json:"expected_value"`
	AnomalyScore        float64              `json:"anomaly_score"`
	Description         string               `json:"description"`
}

// DNCCheckResultStats provides performance and usage statistics
type DNCCheckResultStats struct {
	// Volume statistics
	TotalResults        int64                `json:"total_results"`
	ResultsToday        int64                `json:"results_today"`
	ResultsThisWeek     int64                `json:"results_this_week"`
	ResultsThisMonth    int64                `json:"results_this_month"`
	
	// Block statistics
	TotalBlocked        int64                `json:"total_blocked"`
	BlockRate           float64              `json:"block_rate"`
	BlocksBySource      map[string]int64     `json:"blocks_by_source"`
	BlocksByReason      map[string]int64     `json:"blocks_by_reason"`
	
	// Performance metrics
	AvgCheckTime        time.Duration        `json:"avg_check_time"`
	MedianCheckTime     time.Duration        `json:"median_check_time"`
	P95CheckTime        time.Duration        `json:"p95_check_time"`
	P99CheckTime        time.Duration        `json:"p99_check_time"`
	
	// Cache performance
	CacheHitRate        float64              `json:"cache_hit_rate"`
	CacheSize           int64                `json:"cache_size"`
	CacheEfficiency     float64              `json:"cache_efficiency"`
	
	// Data quality
	ValidResults        int64                `json:"valid_results"`
	InvalidResults      int64                `json:"invalid_results"`
	ExpiredResults      int64                `json:"expired_results"`
	
	// Storage metrics
	StorageSize         int64                `json:"storage_size_bytes"`
	IndexSize           int64                `json:"index_size_bytes"`
	CompressionRatio    float64              `json:"compression_ratio"`
	
	// Collection timestamp
	CollectedAt         time.Time            `json:"collected_at"`
}

// CheckResultIntegrityReport provides comprehensive integrity analysis
type CheckResultIntegrityReport struct {
	// Report metadata
	GeneratedAt         time.Time            `json:"generated_at"`
	
	// Overall status
	IsHealthy           bool                 `json:"is_healthy"`
	OverallStatus       string               `json:"overall_status"`
	
	// Data integrity
	TotalResults        int64                `json:"total_results"`
	ValidResults        int64                `json:"valid_results"`
	InvalidResults      int64                `json:"invalid_results"`
	CorruptedResults    int64                `json:"corrupted_results"`
	
	// Consistency checks
	PhoneNumberConsistency bool              `json:"phone_number_consistency"`
	TimestampConsistency   bool              `json:"timestamp_consistency"`
	ReferenceConsistency   bool              `json:"reference_consistency"`
	
	// Cache integrity
	CacheConsistency    bool                 `json:"cache_consistency"`
	CacheCorruption     int64                `json:"cache_corruption"`
	CacheOrphans        int64                `json:"cache_orphans"`
	
	// Performance impact
	VerificationTime    time.Duration        `json:"verification_time"`
	DatabaseQueries     int                  `json:"database_queries"`
	
	// Issues found
	CriticalIssues      []string             `json:"critical_issues,omitempty"`
	Warnings            []string             `json:"warnings,omitempty"`
	Recommendations     []string             `json:"recommendations,omitempty"`
	
	// Detailed breakdown
	IssuesByCategory    map[string][]string  `json:"issues_by_category,omitempty"`
	IssuesByTimeRange   map[string][]string  `json:"issues_by_time_range,omitempty"`
}