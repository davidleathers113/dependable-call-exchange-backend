package dnc

import (
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/dnc"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/dnc/types"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Request types

// AddSuppressionRequest represents a request to add a phone number to suppression
type AddSuppressionRequest struct {
	PhoneNumber   *values.PhoneNumber    `json:"phone_number" validate:"required"`
	ListSource    values.ListSource      `json:"list_source" validate:"required"`
	SuppressReason values.SuppressReason `json:"suppress_reason" validate:"required"`
	ExpiresAt     *time.Time             `json:"expires_at,omitempty"`
	AddedBy       uuid.UUID              `json:"added_by" validate:"required"`
	Notes         string                 `json:"notes,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// UpdateSuppressionRequest represents a request to update a suppression entry
type UpdateSuppressionRequest struct {
	ID             uuid.UUID              `json:"id" validate:"required"`
	SuppressReason values.SuppressReason  `json:"suppress_reason,omitempty"`
	ExpiresAt      *time.Time             `json:"expires_at,omitempty"`
	Notes          string                 `json:"notes,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
	UpdatedBy      uuid.UUID              `json:"updated_by" validate:"required"`
}

// UpdateProviderRequest represents a request to update provider configuration
type UpdateProviderRequest struct {
	ID              uuid.UUID              `json:"id" validate:"required"`
	Name            string                 `json:"name,omitempty"`
	URL             string                 `json:"url,omitempty"`
	AuthType        string                 `json:"auth_type,omitempty"`
	Configuration   map[string]interface{} `json:"configuration,omitempty"`
	UpdateFrequency time.Duration          `json:"update_frequency,omitempty"`
	Priority        int                    `json:"priority,omitempty"`
	Active          *bool                  `json:"active,omitempty"`
	UpdatedBy       uuid.UUID              `json:"updated_by" validate:"required"`
}

// CallValidationRequest represents a request to validate a call
type CallValidationRequest struct {
	FromNumber        *values.PhoneNumber `json:"from_number" validate:"required"`
	ToNumber          *values.PhoneNumber `json:"to_number" validate:"required"`
	CallTime          time.Time           `json:"call_time" validate:"required"`
	CallType          string              `json:"call_type,omitempty"`
	ComplianceLevel   string              `json:"compliance_level,omitempty" default:"standard"`
	RequiresConsent   bool                `json:"requires_consent,omitempty"`
	BypassTimeChecks  bool                `json:"bypass_time_checks,omitempty"`
}

// ComplianceReportCriteria represents criteria for generating compliance reports
type ComplianceReportCriteria struct {
	PhoneNumbers    []*values.PhoneNumber `json:"phone_numbers,omitempty"`
	ListSources     []values.ListSource   `json:"list_sources,omitempty"`
	SuppressReasons []values.SuppressReason `json:"suppress_reasons,omitempty"`
	StartDate       *time.Time            `json:"start_date,omitempty"`
	EndDate         *time.Time            `json:"end_date,omitempty"`
	IncludeExpired  bool                  `json:"include_expired,omitempty"`
	Format          string                `json:"format,omitempty" default:"json"`
	IncludeDetails  bool                  `json:"include_details,omitempty"`
	Limit           int                   `json:"limit,omitempty" default:"1000"`
	Offset          int                   `json:"offset,omitempty"`
}

// SearchCriteria represents search criteria for suppression entries
type SearchCriteria struct {
	PhoneNumberPattern string                 `json:"phone_number_pattern,omitempty"`
	ListSources        []values.ListSource    `json:"list_sources,omitempty"`
	SuppressReasons    []values.SuppressReason `json:"suppress_reasons,omitempty"`
	Active             *bool                  `json:"active,omitempty"`
	StartDate          *time.Time             `json:"start_date,omitempty"`
	EndDate            *time.Time             `json:"end_date,omitempty"`
	AddedBy            *uuid.UUID             `json:"added_by,omitempty"`
	SortBy             string                 `json:"sort_by,omitempty" default:"added_at"`
	SortOrder          string                 `json:"sort_order,omitempty" default:"desc"`
	Limit              int                    `json:"limit,omitempty" default:"100"`
	Offset             int                    `json:"offset,omitempty"`
}

// CallContext represents context for risk assessment
type CallContext struct {
	CallType        string                 `json:"call_type"`
	Campaign        string                 `json:"campaign,omitempty"`
	CallerID        *values.PhoneNumber    `json:"caller_id,omitempty"`
	CallCenter      string                 `json:"call_center,omitempty"`
	TimeZone        string                 `json:"time_zone,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// Response types

// DNCCheckResponse represents the response from a DNC check
type DNCCheckResponse struct {
	PhoneNumber     *values.PhoneNumber   `json:"phone_number"`
	IsBlocked       bool                  `json:"is_blocked"`
	BlockReasons    []BlockReason         `json:"block_reasons,omitempty"`
	ComplianceLevel string                `json:"compliance_level"`
	RiskScore       float64               `json:"risk_score"`
	CheckedAt       time.Time             `json:"checked_at"`
	CachedResult    bool                  `json:"cached_result"`
	TTL             time.Duration         `json:"ttl"`
	HighestSeverity string                `json:"highest_severity,omitempty"`
	Sources         []CheckSource         `json:"sources"`
	CanCall         bool                  `json:"can_call"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// BlockReason represents a reason why a number is blocked
type BlockReason struct {
	Source      values.ListSource      `json:"source"`
	Reason      values.SuppressReason  `json:"reason"`
	Severity    string                 `json:"severity"`
	AddedAt     time.Time              `json:"added_at"`
	ExpiresAt   *time.Time             `json:"expires_at,omitempty"`
	Description string                 `json:"description"`
	ComplianceCode string              `json:"compliance_code,omitempty"`
}

// CheckSource represents a source checked during DNC verification
type CheckSource struct {
	Source     values.ListSource `json:"source"`
	Checked    bool              `json:"checked"`
	Found      bool              `json:"found"`
	Duration   time.Duration     `json:"duration"`
	Error      string            `json:"error,omitempty"`
	CacheHit   bool              `json:"cache_hit"`
}

// SuppressionResponse represents a suppression entry response
type SuppressionResponse struct {
	ID             uuid.UUID              `json:"id"`
	PhoneNumber    *values.PhoneNumber    `json:"phone_number"`
	ListSource     values.ListSource      `json:"list_source"`
	SuppressReason values.SuppressReason  `json:"suppress_reason"`
	AddedAt        time.Time              `json:"added_at"`
	ExpiresAt      *time.Time             `json:"expires_at,omitempty"`
	AddedBy        uuid.UUID              `json:"added_by"`
	UpdatedBy      *uuid.UUID             `json:"updated_by,omitempty"`
	UpdatedAt      *time.Time             `json:"updated_at,omitempty"`
	Active         bool                   `json:"active"`
	Notes          string                 `json:"notes,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

// SyncResponse represents the response from a provider sync operation
type SyncResponse struct {
	StartedAt       time.Time                 `json:"started_at"`
	CompletedAt     time.Time                 `json:"completed_at"`
	Duration        time.Duration             `json:"duration"`
	TotalProviders  int                       `json:"total_providers"`
	SuccessCount    int                       `json:"success_count"`
	FailureCount    int                       `json:"failure_count"`
	ProviderResults []ProviderSyncResponse    `json:"provider_results"`
	TotalRecords    int                       `json:"total_records"`
	NewRecords      int                       `json:"new_records"`
	UpdatedRecords  int                       `json:"updated_records"`
	Errors          []string                  `json:"errors,omitempty"`
}

// ProviderSyncResponse represents the response from syncing a specific provider
type ProviderSyncResponse struct {
	ProviderID      uuid.UUID     `json:"provider_id"`
	ProviderName    string        `json:"provider_name"`
	Success         bool          `json:"success"`
	StartedAt       time.Time     `json:"started_at"`
	CompletedAt     time.Time     `json:"completed_at"`
	Duration        time.Duration `json:"duration"`
	RecordsProcessed int          `json:"records_processed"`
	RecordsAdded    int           `json:"records_added"`
	RecordsUpdated  int           `json:"records_updated"`
	RecordsSkipped  int           `json:"records_skipped"`
	ErrorCount      int           `json:"error_count"`
	Error           string        `json:"error,omitempty"`
	NextSyncAt      time.Time     `json:"next_sync_at"`
}

// ProviderResponse represents a provider configuration response
type ProviderResponse struct {
	ID              uuid.UUID              `json:"id"`
	Name            string                 `json:"name"`
	Type            string                 `json:"type"`
	URL             string                 `json:"url"`
	AuthType        string                 `json:"auth_type"`
	UpdateFrequency time.Duration          `json:"update_frequency"`
	Priority        int                    `json:"priority"`
	Active          bool                   `json:"active"`
	CreatedAt       time.Time              `json:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at"`
	LastSyncAt      *time.Time             `json:"last_sync_at,omitempty"`
	NextSyncAt      time.Time              `json:"next_sync_at"`
	HealthStatus    string                 `json:"health_status"`
	ErrorCount      int                    `json:"error_count"`
	SuccessRate     float64                `json:"success_rate"`
	Configuration   map[string]interface{} `json:"configuration,omitempty"`
}

// ProviderStatusResponse represents provider status information
type ProviderStatusResponse struct {
	Provider        ProviderResponse       `json:"provider"`
	SyncStatus      string                 `json:"sync_status"`
	LastSyncResult  string                 `json:"last_sync_result"`
	RecordsManaged  int                    `json:"records_managed"`
	PerformanceMetrics PerformanceMetrics  `json:"performance_metrics"`
	RecentErrors    []string               `json:"recent_errors,omitempty"`
}

// PerformanceMetrics represents performance metrics for providers
type PerformanceMetrics struct {
	AverageResponseTime time.Duration `json:"average_response_time"`
	SuccessRate         float64       `json:"success_rate"`
	ErrorRate           float64       `json:"error_rate"`
	TotalRequests       int64         `json:"total_requests"`
	SuccessfulRequests  int64         `json:"successful_requests"`
	FailedRequests      int64         `json:"failed_requests"`
	LastResponseTime    time.Duration `json:"last_response_time"`
	LastUpdated         time.Time     `json:"last_updated"`
}

// ComplianceReportResponse represents a compliance report
type ComplianceReportResponse struct {
	GeneratedAt     time.Time              `json:"generated_at"`
	Criteria        ComplianceReportCriteria `json:"criteria"`
	Summary         ComplianceSummary      `json:"summary"`
	Entries         []SuppressionResponse  `json:"entries,omitempty"`
	Statistics      ComplianceStatistics   `json:"statistics"`
	Format          string                 `json:"format"`
	ExportURL       string                 `json:"export_url,omitempty"`
}

// ComplianceSummary represents a summary of compliance data
type ComplianceSummary struct {
	TotalEntries        int                       `json:"total_entries"`
	ActiveEntries       int                       `json:"active_entries"`
	ExpiredEntries      int                       `json:"expired_entries"`
	SourceBreakdown     map[string]int            `json:"source_breakdown"`
	ReasonBreakdown     map[string]int            `json:"reason_breakdown"`
	RiskDistribution    map[string]int            `json:"risk_distribution"`
	RecentActivity      int                       `json:"recent_activity"`
}

// ComplianceStatistics represents detailed compliance statistics
type ComplianceStatistics struct {
	BySource        map[values.ListSource]int      `json:"by_source"`
	ByReason        map[values.SuppressReason]int  `json:"by_reason"`
	ByMonth         map[string]int                 `json:"by_month"`
	ByRiskLevel     map[string]int                 `json:"by_risk_level"`
	ComplianceScore float64                        `json:"compliance_score"`
	TrendDirection  string                         `json:"trend_direction"`
}

// CallValidationResponse represents call validation results
type CallValidationResponse struct {
	FromNumber       *values.PhoneNumber    `json:"from_number"`
	ToNumber         *values.PhoneNumber    `json:"to_number"`
	CallTime         time.Time              `json:"call_time"`
	Allowed          bool                   `json:"allowed"`
	DNCResult        DNCCheckResponse       `json:"dnc_result"`
	TCPACompliant    bool                   `json:"tcpa_compliant"`
	CallingHours     bool                   `json:"calling_hours"`
	ConsentRequired  bool                   `json:"consent_required"`
	ConsentPresent   bool                   `json:"consent_present"`
	RiskAssessment   RiskAssessmentResponse `json:"risk_assessment"`
	Violations       []ComplianceViolation  `json:"violations,omitempty"`
	Recommendations  []string               `json:"recommendations,omitempty"`
	ValidatedAt      time.Time              `json:"validated_at"`
}

// ComplianceViolation represents a compliance violation
type ComplianceViolation struct {
	Type        string    `json:"type"`
	Severity    string    `json:"severity"`
	Description string    `json:"description"`
	Regulation  string    `json:"regulation"`
	Penalty     *decimal.Decimal `json:"penalty,omitempty"`
	Required    []string  `json:"required,omitempty"`
}

// RiskAssessmentResponse represents risk assessment results
type RiskAssessmentResponse struct {
	PhoneNumber     *values.PhoneNumber    `json:"phone_number"`
	RiskScore       float64                `json:"risk_score"`
	RiskLevel       string                 `json:"risk_level"`
	Factors         []RiskFactor           `json:"factors"`
	PenaltyEstimate *decimal.Decimal       `json:"penalty_estimate,omitempty"`
	Recommendations []string               `json:"recommendations"`
	CallHistory     CallHistorySummary     `json:"call_history"`
	AssessedAt      time.Time              `json:"assessed_at"`
}

// RiskFactor represents a factor contributing to risk
type RiskFactor struct {
	Factor      string  `json:"factor"`
	Weight      float64 `json:"weight"`
	Score       float64 `json:"score"`
	Description string  `json:"description"`
}

// CallHistorySummary represents call history summary for risk assessment
type CallHistorySummary struct {
	TotalCalls      int       `json:"total_calls"`
	RecentCalls     int       `json:"recent_calls"`
	Violations      int       `json:"violations"`
	LastCallAt      *time.Time `json:"last_call_at,omitempty"`
	LastViolationAt *time.Time `json:"last_violation_at,omitempty"`
	CallFrequency   float64   `json:"call_frequency"`
}

// SearchResponse represents search results
type SearchResponse struct {
	Results    []SuppressionResponse `json:"results"`
	Total      int                   `json:"total"`
	Offset     int                   `json:"offset"`
	Limit      int                   `json:"limit"`
	HasMore    bool                  `json:"has_more"`
	SearchedAt time.Time             `json:"searched_at"`
}

// CacheStatsResponse represents cache statistics
type CacheStatsResponse struct {
	HitRate           float64              `json:"hit_rate"`
	MissRate          float64              `json:"miss_rate"`
	TotalRequests     int64                `json:"total_requests"`
	CacheHits         int64                `json:"cache_hits"`
	CacheMisses       int64                `json:"cache_misses"`
	AverageLatency    time.Duration        `json:"average_latency"`
	MemoryUsage       int64                `json:"memory_usage"`
	KeyCount          int64                `json:"key_count"`
	ExpiredKeys       int64                `json:"expired_keys"`
	EvictedKeys       int64                `json:"evicted_keys"`
	BloomFilterStats  BloomFilterStats     `json:"bloom_filter_stats"`
	DetailedStats     map[string]interface{} `json:"detailed_stats,omitempty"`
	CollectedAt       time.Time            `json:"collected_at"`
}

// BloomFilterStats represents bloom filter statistics
type BloomFilterStats struct {
	EstimatedItems   int     `json:"estimated_items"`
	FalsePositiveRate float64 `json:"false_positive_rate"`
	BitArraySize     int     `json:"bit_array_size"`
	HashFunctionCount int    `json:"hash_function_count"`
	LastResetAt      *time.Time `json:"last_reset_at,omitempty"`
}

// CacheStats represents internal cache statistics
type CacheStats struct {
	Hits          int64         `json:"hits"`
	Misses        int64         `json:"misses"`
	Errors        int64         `json:"errors"`
	AverageLatency time.Duration `json:"average_latency"`
	MemoryUsage   int64         `json:"memory_usage"`
	KeyCount      int64         `json:"key_count"`
}

// HealthResponse represents service health status
type HealthResponse struct {
	Status       string                 `json:"status"`
	Version      string                 `json:"version"`
	CheckedAt    time.Time              `json:"checked_at"`
	Dependencies []DependencyHealth     `json:"dependencies"`
	Metrics      HealthMetrics          `json:"metrics"`
	Warnings     []string               `json:"warnings,omitempty"`
	Errors       []string               `json:"errors,omitempty"`
}

// DependencyHealth represents health status of a dependency
type DependencyHealth struct {
	Name         string        `json:"name"`
	Status       string        `json:"status"`
	ResponseTime time.Duration `json:"response_time"`
	LastCheck    time.Time     `json:"last_check"`
	Error        string        `json:"error,omitempty"`
	Details      map[string]interface{} `json:"details,omitempty"`
}

// HealthMetrics represents service health metrics
type HealthMetrics struct {
	RequestsPerSecond    float64 `json:"requests_per_second"`
	AverageResponseTime  time.Duration `json:"average_response_time"`
	ErrorRate            float64 `json:"error_rate"`
	CacheHitRate         float64 `json:"cache_hit_rate"`
	ActiveConnections    int     `json:"active_connections"`
	MemoryUsagePercent   float64 `json:"memory_usage_percent"`
	CPUUsagePercent      float64 `json:"cpu_usage_percent"`
}

// External API result types

// FederalDNCResult represents result from federal DNC API
type FederalDNCResult struct {
	PhoneNumber *values.PhoneNumber `json:"phone_number"`
	Listed      bool                `json:"listed"`
	AddedDate   *time.Time          `json:"added_date,omitempty"`
	Source      string              `json:"source"`
	Confidence  float64             `json:"confidence"`
}

// StateDNCResult represents result from state DNC API
type StateDNCResult struct {
	PhoneNumber *values.PhoneNumber `json:"phone_number"`
	State       string              `json:"state"`
	Listed      bool                `json:"listed"`
	AddedDate   *time.Time          `json:"added_date,omitempty"`
	Source      string              `json:"source"`
	Confidence  float64             `json:"confidence"`
}

// Audit request types

// DNCCheckAuditRequest represents audit information for DNC checks
type DNCCheckAuditRequest struct {
	PhoneNumber  *values.PhoneNumber `json:"phone_number"`
	Result       DNCCheckResponse    `json:"result"`
	UserID       uuid.UUID           `json:"user_id"`
	RequestID    string              `json:"request_id"`
	IPAddress    string              `json:"ip_address,omitempty"`
	UserAgent    string              `json:"user_agent,omitempty"`
	SessionID    string              `json:"session_id,omitempty"`
	CheckedAt    time.Time           `json:"checked_at"`
}

// SuppressionAuditRequest represents audit information for suppression changes
type SuppressionAuditRequest struct {
	Action      string              `json:"action"`
	EntryID     uuid.UUID           `json:"entry_id"`
	PhoneNumber *values.PhoneNumber `json:"phone_number"`
	Before      *SuppressionResponse `json:"before,omitempty"`
	After       *SuppressionResponse `json:"after,omitempty"`
	UserID      uuid.UUID           `json:"user_id"`
	Reason      string              `json:"reason,omitempty"`
	Timestamp   time.Time           `json:"timestamp"`
}

// ProviderSyncAuditRequest represents audit information for provider sync operations
type ProviderSyncAuditRequest struct {
	ProviderID   uuid.UUID            `json:"provider_id"`
	ProviderName string               `json:"provider_name"`
	SyncResult   ProviderSyncResponse `json:"sync_result"`
	UserID       *uuid.UUID           `json:"user_id,omitempty"`
	Scheduled    bool                 `json:"scheduled"`
	Timestamp    time.Time            `json:"timestamp"`
}