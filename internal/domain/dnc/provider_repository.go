package dnc

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// DNCProviderRepository defines the interface for DNC provider persistence operations
// Performance expectations:
// - Save: < 20ms for single provider
// - FindByType: < 10ms with proper indexing
// - List: < 50ms for full provider list
type DNCProviderRepository interface {
	// Core CRUD operations
	
	// Save creates or updates a DNC provider
	// Returns error if provider name conflicts or URL is invalid
	Save(ctx context.Context, provider *DNCProvider) error
	
	// SaveWithTx saves a DNC provider within an existing transaction
	SaveWithTx(ctx context.Context, tx Transaction, provider *DNCProvider) error
	
	// GetByID retrieves a DNC provider by its unique identifier
	GetByID(ctx context.Context, id uuid.UUID) (*DNCProvider, error)
	
	// Update modifies an existing DNC provider
	Update(ctx context.Context, provider *DNCProvider) error
	
	// UpdateWithTx updates a DNC provider within an existing transaction
	UpdateWithTx(ctx context.Context, tx Transaction, provider *DNCProvider) error
	
	// Delete removes a DNC provider (soft delete to preserve audit trail)
	Delete(ctx context.Context, id uuid.UUID, deletedBy uuid.UUID) error
	
	// Query operations
	
	// FindByType retrieves providers by provider type
	// Used for sync orchestration by provider category
	FindByType(ctx context.Context, providerType ProviderType) ([]*DNCProvider, error)
	
	// FindByName retrieves a provider by its unique name
	FindByName(ctx context.Context, name string) (*DNCProvider, error)
	
	// List retrieves all providers with optional filtering
	List(ctx context.Context, filter DNCProviderFilter) ([]*DNCProvider, error)
	
	// Status and sync operations
	
	// FindActive retrieves all currently active (enabled) providers
	// This is frequently called for sync scheduling - must be optimized
	FindActive(ctx context.Context) ([]*DNCProvider, error)
	
	// FindByStatus retrieves providers by their operational status
	FindByStatus(ctx context.Context, status ProviderStatus) ([]*DNCProvider, error)
	
	// FindNeedingSync retrieves providers that need synchronization
	// Based on update frequency and last sync time
	FindNeedingSync(ctx context.Context) ([]*DNCProvider, error)
	
	// FindInError retrieves providers currently in error state
	FindInError(ctx context.Context) ([]*DNCProvider, error)
	
	// Sync tracking operations
	
	// UpdateSyncStatus updates the sync status and timestamps for a provider
	UpdateSyncStatus(ctx context.Context, providerID uuid.UUID, status ProviderStatus, 
		syncInfo *SyncInfo) error
	
	// RecordSyncAttempt records a sync attempt with outcome
	RecordSyncAttempt(ctx context.Context, providerID uuid.UUID, attempt *SyncAttempt) error
	
	// GetSyncHistory retrieves sync history for a provider
	GetSyncHistory(ctx context.Context, providerID uuid.UUID, limit int) ([]*SyncAttempt, error)
	
	// Configuration management
	
	// UpdateConfig updates provider configuration without affecting sync state
	UpdateConfig(ctx context.Context, providerID uuid.UUID, config map[string]string) error
	
	// UpdateAuth updates authentication credentials securely
	UpdateAuth(ctx context.Context, providerID uuid.UUID, authType AuthType, 
		credentials *string) error
	
	// Health and monitoring
	
	// GetHealthStatus retrieves health status for all providers
	GetHealthStatus(ctx context.Context) ([]*ProviderHealthStatus, error)
	
	// GetProviderMetrics retrieves performance metrics for a provider
	GetProviderMetrics(ctx context.Context, providerID uuid.UUID, 
		timeRange TimeRange) (*ProviderMetrics, error)
	
	// UpdateHealthCheck updates the health check status for a provider
	UpdateHealthCheck(ctx context.Context, providerID uuid.UUID, 
		health *HealthCheckResult) error
	
	// Batch operations
	
	// BulkUpdateStatus updates status for multiple providers
	BulkUpdateStatus(ctx context.Context, updates []ProviderStatusUpdate) error
	
	// BulkUpdateConfig updates configuration for multiple providers
	BulkUpdateConfig(ctx context.Context, updates []ProviderConfigUpdate) error
	
	// Administrative operations
	
	// GetStats returns repository performance and usage statistics
	GetStats(ctx context.Context) (*DNCProviderStats, error)
	
	// Vacuum performs database maintenance operations
	Vacuum(ctx context.Context) error
	
	// ValidateIntegrity performs integrity checks on provider data
	ValidateIntegrity(ctx context.Context) (*ProviderIntegrityReport, error)
	
	// Transaction support
	
	// BeginTx starts a new database transaction
	BeginTx(ctx context.Context) (Transaction, error)
	
	// WithTx executes a function within a database transaction
	WithTx(ctx context.Context, fn func(tx Transaction) error) error
}

// DNCProviderFilter defines filtering options for provider queries
type DNCProviderFilter struct {
	// Type filters
	Types     []ProviderType   `json:"types,omitempty"`
	Statuses  []ProviderStatus `json:"statuses,omitempty"`
	AuthTypes []AuthType       `json:"auth_types,omitempty"`
	
	// Name filters
	Names       []string `json:"names,omitempty"`
	NamePattern *string  `json:"name_pattern,omitempty"` // SQL LIKE pattern
	
	// Status filters
	OnlyActive    *bool `json:"only_active,omitempty"`
	OnlyEnabled   *bool `json:"only_enabled,omitempty"`
	OnlyInError   *bool `json:"only_in_error,omitempty"`
	NeedsSync     *bool `json:"needs_sync,omitempty"`
	
	// Time range filters
	CreatedAfter    *time.Time `json:"created_after,omitempty"`
	CreatedBefore   *time.Time `json:"created_before,omitempty"`
	LastSyncAfter   *time.Time `json:"last_sync_after,omitempty"`
	LastSyncBefore  *time.Time `json:"last_sync_before,omitempty"`
	
	// Performance filters
	MinSuccessRate     *float64 `json:"min_success_rate,omitempty"`
	MaxErrorCount      *int     `json:"max_error_count,omitempty"`
	MaxSyncDuration    *time.Duration `json:"max_sync_duration,omitempty"`
	
	// Priority filters
	MinPriority *int `json:"min_priority,omitempty"`
	MaxPriority *int `json:"max_priority,omitempty"`
	
	// User filters
	CreatedBy []uuid.UUID `json:"created_by,omitempty"`
	UpdatedBy []uuid.UUID `json:"updated_by,omitempty"`
	
	// Configuration filters
	HasConfig      []string          `json:"has_config,omitempty"`      // Must have these config keys
	ConfigValues   map[string]string `json:"config_values,omitempty"`   // Must match these values
	
	// Search
	SearchText *string `json:"search_text,omitempty"` // Search in name, base URL, config
	
	// Pagination
	Limit  int    `json:"limit,omitempty"`
	Offset int    `json:"offset,omitempty"`
	Cursor string `json:"cursor,omitempty"`
	
	// Sorting
	OrderBy   string `json:"order_by,omitempty"`   // Field to sort by
	OrderDesc bool   `json:"order_desc,omitempty"` // Sort direction
	
	// Performance options
	IncludeConfig    bool `json:"include_config,omitempty"`    // Include configuration data
	IncludeMetrics   bool `json:"include_metrics,omitempty"`   // Include performance metrics
	IncludeHealth    bool `json:"include_health,omitempty"`    // Include health status
}

// SyncInfo contains information about a sync operation
type SyncInfo struct {
	Duration    time.Duration `json:"duration"`
	RecordCount int           `json:"record_count"`
	ErrorCount  int           `json:"error_count"`
	StartedAt   time.Time     `json:"started_at"`
	CompletedAt time.Time     `json:"completed_at"`
	ErrorMsg    *string       `json:"error_msg,omitempty"`
	
	// Performance metrics
	ThroughputPerSecond float64 `json:"throughput_per_second"`
	MemoryUsedMB        float64 `json:"memory_used_mb"`
	NetworkBytesIn      int64   `json:"network_bytes_in"`
	NetworkBytesOut     int64   `json:"network_bytes_out"`
}

// SyncAttempt represents a single sync attempt with detailed information
type SyncAttempt struct {
	ID           uuid.UUID     `json:"id"`
	ProviderID   uuid.UUID     `json:"provider_id"`
	AttemptedAt  time.Time     `json:"attempted_at"`
	CompletedAt  *time.Time    `json:"completed_at,omitempty"`
	Status       string        `json:"status"` // started, completed, failed, timeout
	RecordsRead  int           `json:"records_read"`
	RecordsAdded int           `json:"records_added"`
	RecordsUpdated int         `json:"records_updated"`
	RecordsSkipped int         `json:"records_skipped"`
	Duration     *time.Duration `json:"duration,omitempty"`
	ErrorMsg     *string       `json:"error_msg,omitempty"`
	ErrorCode    *string       `json:"error_code,omitempty"`
	
	// Metadata
	TriggerType  string            `json:"trigger_type"` // scheduled, manual, retry
	Config       map[string]string `json:"config,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// ProviderHealthStatus contains health status information for a provider
type ProviderHealthStatus struct {
	ProviderID     uuid.UUID     `json:"provider_id"`
	ProviderName   string        `json:"provider_name"`
	OverallHealth  string        `json:"overall_health"` // healthy, degraded, unhealthy
	IsResponding   bool          `json:"is_responding"`
	LastCheckAt    time.Time     `json:"last_check_at"`
	ResponseTime   time.Duration `json:"response_time"`
	
	// Detailed checks
	ConnectivityOK bool          `json:"connectivity_ok"`
	AuthenticationOK bool        `json:"authentication_ok"`
	DataFormatOK   bool          `json:"data_format_ok"`
	RateLimitOK    bool          `json:"rate_limit_ok"`
	
	// Recent performance
	RecentSuccessRate float64     `json:"recent_success_rate"`
	RecentErrorCount  int         `json:"recent_error_count"`
	RecentAvgDuration time.Duration `json:"recent_avg_duration"`
	
	// Alerts
	ActiveAlerts   []string       `json:"active_alerts,omitempty"`
	WarningCount   int            `json:"warning_count"`
	ErrorCount     int            `json:"error_count"`
}

// ProviderMetrics contains performance metrics for a provider
type ProviderMetrics struct {
	ProviderID   uuid.UUID `json:"provider_id"`
	TimeRange    TimeRange `json:"time_range"`
	
	// Sync metrics
	TotalSyncs     int           `json:"total_syncs"`
	SuccessfulSyncs int          `json:"successful_syncs"`
	FailedSyncs    int           `json:"failed_syncs"`
	SuccessRate    float64       `json:"success_rate"`
	
	// Performance metrics
	AvgDuration    time.Duration `json:"avg_duration"`
	MinDuration    time.Duration `json:"min_duration"`
	MaxDuration    time.Duration `json:"max_duration"`
	Throughput     float64       `json:"throughput"` // Records per second
	
	// Data metrics
	TotalRecords   int64         `json:"total_records"`
	AddedRecords   int64         `json:"added_records"`
	UpdatedRecords int64         `json:"updated_records"`
	SkippedRecords int64         `json:"skipped_records"`
	
	// Error analysis
	TopErrors      []ErrorSummary `json:"top_errors,omitempty"`
	ErrorRate      float64        `json:"error_rate"`
	
	// Resource usage
	AvgMemoryMB    float64       `json:"avg_memory_mb"`
	MaxMemoryMB    float64       `json:"max_memory_mb"`
	NetworkBytesIn int64         `json:"network_bytes_in"`
	NetworkBytesOut int64        `json:"network_bytes_out"`
	
	// Trends
	DailyMetrics   []DailyMetric `json:"daily_metrics,omitempty"`
}

// TimeRange represents a time range for metrics queries
type TimeRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// ErrorSummary contains summarized error information
type ErrorSummary struct {
	ErrorCode   string `json:"error_code"`
	ErrorMsg    string `json:"error_msg"`
	Count       int    `json:"count"`
	LastOccurred time.Time `json:"last_occurred"`
}

// DailyMetric contains daily aggregated metrics
type DailyMetric struct {
	Date        time.Time `json:"date"`
	SyncCount   int       `json:"sync_count"`
	SuccessRate float64   `json:"success_rate"`
	AvgDuration time.Duration `json:"avg_duration"`
	RecordCount int64     `json:"record_count"`
}

// HealthCheckResult contains the result of a health check
type HealthCheckResult struct {
	CheckedAt      time.Time     `json:"checked_at"`
	IsHealthy      bool          `json:"is_healthy"`
	ResponseTime   time.Duration `json:"response_time"`
	StatusCode     int           `json:"status_code,omitempty"`
	ErrorMsg       *string       `json:"error_msg,omitempty"`
	
	// Detailed check results
	Connectivity   bool          `json:"connectivity"`
	Authentication bool          `json:"authentication"`
	DataAvailable  bool          `json:"data_available"`
	RateLimit      bool          `json:"rate_limit"`
	
	// Additional metadata
	Metadata       map[string]string `json:"metadata,omitempty"`
}

// ProviderStatusUpdate represents a batch status update
type ProviderStatusUpdate struct {
	ProviderID uuid.UUID     `json:"provider_id"`
	Status     ProviderStatus `json:"status"`
	UpdatedBy  uuid.UUID     `json:"updated_by"`
	Reason     *string       `json:"reason,omitempty"`
}

// ProviderConfigUpdate represents a batch configuration update
type ProviderConfigUpdate struct {
	ProviderID uuid.UUID         `json:"provider_id"`
	Config     map[string]string `json:"config"`
	UpdatedBy  uuid.UUID         `json:"updated_by"`
}

// DNCProviderStats provides performance and usage statistics
type DNCProviderStats struct {
	// Provider counts by type
	TotalProviders      int64            `json:"total_providers"`
	ProvidersByType     map[string]int64 `json:"providers_by_type"`
	ProvidersByStatus   map[string]int64 `json:"providers_by_status"`
	ActiveProviders     int64            `json:"active_providers"`
	HealthyProviders    int64            `json:"healthy_providers"`
	
	// Performance metrics
	AvgSyncDuration     time.Duration    `json:"avg_sync_duration"`
	AvgSuccessRate      float64          `json:"avg_success_rate"`
	TotalSyncsToday     int64            `json:"total_syncs_today"`
	FailedSyncsToday    int64            `json:"failed_syncs_today"`
	
	// System health
	ProvidersInError    int64            `json:"providers_in_error"`
	ProvidersOverdue    int64            `json:"providers_overdue"` // Missed sync window
	ProvidersThrottled  int64            `json:"providers_throttled"`
	
	// Growth metrics
	ProvidersThisMonth  int64            `json:"providers_this_month"`
	ProvidersThisWeek   int64            `json:"providers_this_week"`
	
	// Resource usage
	TotalSyncTime       time.Duration    `json:"total_sync_time"`
	TotalRecordsProcessed int64          `json:"total_records_processed"`
	
	// Collection timestamp
	CollectedAt         time.Time        `json:"collected_at"`
}

// ProviderIntegrityReport provides comprehensive integrity analysis for providers
type ProviderIntegrityReport struct {
	// Report metadata
	GeneratedAt    time.Time `json:"generated_at"`
	
	// Overall status
	IsHealthy      bool      `json:"is_healthy"`
	OverallStatus  string    `json:"overall_status"` // HEALTHY, DEGRADED, CRITICAL
	
	// Data integrity
	TotalProviders    int64   `json:"total_providers"`
	ValidProviders    int64   `json:"valid_providers"`
	InvalidProviders  int64   `json:"invalid_providers"`
	DuplicateNames    int64   `json:"duplicate_names"`
	
	// Configuration integrity
	InvalidURLs       int64   `json:"invalid_urls"`
	MissingAuth       int64   `json:"missing_auth"`
	InvalidConfig     int64   `json:"invalid_config"`
	
	// Operational integrity
	StaleProviders    int64   `json:"stale_providers"`    // Not synced recently
	ErrorProviders    int64   `json:"error_providers"`    // In persistent error state
	UnresponsiveProviders int64 `json:"unresponsive_providers"`
	
	// Performance metrics
	VerificationTime  time.Duration `json:"verification_time"`
	DatabaseQueries   int           `json:"database_queries"`
	
	// Issues found
	CriticalIssues    []string      `json:"critical_issues,omitempty"`
	Warnings          []string      `json:"warnings,omitempty"`
	Recommendations   []string      `json:"recommendations,omitempty"`
	
	// Detailed breakdown
	IssuesByType      map[string][]string `json:"issues_by_type,omitempty"`
	IssuesByProvider  map[string][]string `json:"issues_by_provider,omitempty"`
}