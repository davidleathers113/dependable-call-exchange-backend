package providers

import (
	"context"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/dnc"
)

// ProviderClient defines the interface for DNC provider integrations
// Each provider client must implement these methods for external DNC data sources
type ProviderClient interface {
	// Provider metadata
	GetProviderType() dnc.ProviderType
	GetProviderName() string
	
	// Health and connectivity
	HealthCheck(ctx context.Context) (*HealthCheckResult, error)
	
	// Data retrieval operations
	CheckNumber(ctx context.Context, phoneNumber string) (*CheckResult, error)
	BatchCheckNumbers(ctx context.Context, phoneNumbers []string) ([]*CheckResult, error)
	
	// Synchronization operations
	GetIncrementalUpdates(ctx context.Context, since time.Time) (*SyncResult, error)
	GetFullSnapshot(ctx context.Context) (*SyncResult, error)
	
	// Authentication and configuration
	ValidateConfig(config map[string]string) error
	SetConfig(config map[string]string) error
	
	// Rate limiting and quotas
	GetRateLimit() RateLimit
	GetQuotaStatus(ctx context.Context) (*QuotaStatus, error)
	
	// Connection management
	Connect(ctx context.Context) error
	Disconnect(ctx context.Context) error
	IsConnected() bool
}

// SyncProvider defines extended synchronization capabilities
type SyncProvider interface {
	ProviderClient
	
	// Advanced sync operations
	GetSyncMetadata(ctx context.Context) (*SyncMetadata, error)
	SetSyncCheckpoint(ctx context.Context, checkpoint string) error
	GetSyncCheckpoint(ctx context.Context) (string, error)
	
	// Conflict resolution
	ResolveDuplicates(ctx context.Context, entries []*DNCEntry) ([]*DNCEntry, error)
	
	// Data transformation
	TransformEntry(entry *ExternalEntry) (*DNCEntry, error)
	ValidateEntry(entry *DNCEntry) error
}

// CircuitBreakerProvider wraps providers with circuit breaker functionality
type CircuitBreakerProvider interface {
	ProviderClient
	
	// Circuit breaker state
	GetCircuitState() CircuitState
	ResetCircuit() error
	
	// Failure tracking
	RecordSuccess()
	RecordFailure(err error)
	
	// Configuration
	SetCircuitConfig(config CircuitConfig) error
}

// ProviderManager manages multiple DNC provider instances
type ProviderManager interface {
	// Provider lifecycle
	RegisterProvider(name string, client ProviderClient) error
	UnregisterProvider(name string) error
	GetProvider(name string) (ProviderClient, error)
	ListProviders() []string
	
	// Provider discovery and health
	DiscoverProviders(ctx context.Context) ([]*ProviderInfo, error)
	HealthCheckAll(ctx context.Context) ([]*ProviderHealth, error)
	
	// Load balancing
	GetNextProvider(providerType dnc.ProviderType) (ProviderClient, error)
	RouteRequest(ctx context.Context, request *ProviderRequest) (*ProviderResponse, error)
	
	// Batch operations
	BulkCheck(ctx context.Context, phoneNumbers []string) ([]*CheckResult, error)
	BulkSync(ctx context.Context, providers []string) ([]*SyncResult, error)
	
	// Configuration management
	UpdateProviderConfig(name string, config map[string]string) error
	GetProviderConfig(name string) (map[string]string, error)
	
	// Monitoring and metrics
	GetProviderMetrics(name string) (*ProviderMetrics, error)
	GetAggregateMetrics() (*AggregateMetrics, error)
	
	// Cleanup and shutdown
	Close() error
}

// Data structures

// HealthCheckResult contains provider health check information
type HealthCheckResult struct {
	IsHealthy     bool              `json:"is_healthy"`
	ResponseTime  time.Duration     `json:"response_time"`
	StatusCode    int               `json:"status_code,omitempty"`
	Error         string            `json:"error,omitempty"`
	
	// Detailed checks
	Connectivity  bool              `json:"connectivity"`
	Authentication bool             `json:"authentication"`
	DataAvailable bool              `json:"data_available"`
	RateLimit     bool              `json:"rate_limit"`
	
	// Provider-specific data
	Metadata      map[string]string `json:"metadata,omitempty"`
	LastUpdated   time.Time         `json:"last_updated"`
}

// CheckResult contains DNC check result for a phone number
type CheckResult struct {
	PhoneNumber   string            `json:"phone_number"`
	IsListed      bool              `json:"is_listed"`
	ListSource    string            `json:"list_source"`
	LastUpdated   time.Time         `json:"last_updated"`
	ExpiresAt     *time.Time        `json:"expires_at,omitempty"`
	
	// Additional metadata
	RegistrationDate *time.Time       `json:"registration_date,omitempty"`
	Reason          string           `json:"reason,omitempty"`
	Confidence      float64          `json:"confidence"`
	Metadata        map[string]string `json:"metadata,omitempty"`
}

// SyncResult contains the result of a synchronization operation
type SyncResult struct {
	ProviderName    string           `json:"provider_name"`
	StartedAt       time.Time        `json:"started_at"`
	CompletedAt     time.Time        `json:"completed_at"`
	Duration        time.Duration    `json:"duration"`
	
	// Sync statistics
	RecordsProcessed int             `json:"records_processed"`
	RecordsAdded     int             `json:"records_added"`
	RecordsUpdated   int             `json:"records_updated"`
	RecordsSkipped   int             `json:"records_skipped"`
	RecordsDeleted   int             `json:"records_deleted"`
	
	// Status and error handling
	Status          string           `json:"status"` // success, partial, failed
	Errors          []SyncError      `json:"errors,omitempty"`
	Warnings        []string         `json:"warnings,omitempty"`
	
	// Sync metadata
	Checkpoint      string           `json:"checkpoint,omitempty"`
	NextSync        *time.Time       `json:"next_sync,omitempty"`
	DataVersion     string           `json:"data_version,omitempty"`
	
	// Performance metrics
	ThroughputPerSecond float64      `json:"throughput_per_second"`
	MemoryUsedMB       float64       `json:"memory_used_mb"`
	NetworkBytesIn     int64         `json:"network_bytes_in"`
	NetworkBytesOut    int64         `json:"network_bytes_out"`
}

// SyncError represents an error during synchronization
type SyncError struct {
	Code        string    `json:"code"`
	Message     string    `json:"message"`
	RecordIndex int       `json:"record_index,omitempty"`
	PhoneNumber string    `json:"phone_number,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
}

// RateLimit contains rate limiting information
type RateLimit struct {
	RequestsPerSecond int           `json:"requests_per_second"`
	RequestsPerMinute int           `json:"requests_per_minute"`
	RequestsPerHour   int           `json:"requests_per_hour"`
	RequestsPerDay    int           `json:"requests_per_day"`
	BurstSize         int           `json:"burst_size"`
}

// QuotaStatus contains current quota usage information
type QuotaStatus struct {
	Used         int64     `json:"used"`
	Limit        int64     `json:"limit"`
	Remaining    int64     `json:"remaining"`
	ResetTime    time.Time `json:"reset_time"`
	Period       string    `json:"period"` // minute, hour, day, month
}

// SyncMetadata contains metadata about synchronization state
type SyncMetadata struct {
	LastFullSync      time.Time `json:"last_full_sync"`
	LastIncrementalSync time.Time `json:"last_incremental_sync"`
	DataVersion       string    `json:"data_version"`
	TotalRecords      int64     `json:"total_records"`
	Checksum          string    `json:"checksum,omitempty"`
	
	// Provider-specific metadata
	ProviderMetadata  map[string]string `json:"provider_metadata,omitempty"`
}

// DNCEntry represents a standardized DNC entry
type DNCEntry struct {
	PhoneNumber      string            `json:"phone_number"`
	RegistrationDate time.Time         `json:"registration_date"`
	ExpirationDate   *time.Time        `json:"expiration_date,omitempty"`
	Source           string            `json:"source"`
	ListType         string            `json:"list_type"`
	Status           string            `json:"status"`
	Metadata         map[string]string `json:"metadata,omitempty"`
}

// ExternalEntry represents an entry from an external provider
type ExternalEntry struct {
	RawData   map[string]interface{} `json:"raw_data"`
	Format    string                 `json:"format"`
	Source    string                 `json:"source"`
	Timestamp time.Time              `json:"timestamp"`
}

// Circuit breaker types

// CircuitState represents the state of a circuit breaker
type CircuitState int

const (
	CircuitClosed CircuitState = iota
	CircuitOpen
	CircuitHalfOpen
)

func (s CircuitState) String() string {
	switch s {
	case CircuitClosed:
		return "closed"
	case CircuitOpen:
		return "open"
	case CircuitHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitConfig contains circuit breaker configuration
type CircuitConfig struct {
	FailureThreshold  int           `json:"failure_threshold"`
	RecoveryTimeout   time.Duration `json:"recovery_timeout"`
	SuccessThreshold  int           `json:"success_threshold"`
	HalfOpenMaxCalls  int           `json:"half_open_max_calls"`
}

// Provider management types

// ProviderInfo contains information about a discovered provider
type ProviderInfo struct {
	Name        string                 `json:"name"`
	Type        dnc.ProviderType       `json:"type"`
	BaseURL     string                 `json:"base_url"`
	Version     string                 `json:"version,omitempty"`
	Capabilities []string              `json:"capabilities,omitempty"`
	Config      map[string]string      `json:"config,omitempty"`
}

// ProviderHealth contains health status for a provider
type ProviderHealth struct {
	Name         string             `json:"name"`
	Type         dnc.ProviderType   `json:"type"`
	IsHealthy    bool               `json:"is_healthy"`
	LastCheck    time.Time          `json:"last_check"`
	ResponseTime time.Duration      `json:"response_time"`
	Error        string             `json:"error,omitempty"`
	Details      *HealthCheckResult `json:"details,omitempty"`
}

// ProviderRequest represents a request to be routed to a provider
type ProviderRequest struct {
	Type        string                 `json:"type"`        // check, sync, etc.
	Data        map[string]interface{} `json:"data"`
	Options     map[string]string      `json:"options,omitempty"`
	Timeout     time.Duration          `json:"timeout,omitempty"`
	RetryCount  int                    `json:"retry_count,omitempty"`
}

// ProviderResponse represents a response from a provider
type ProviderResponse struct {
	ProviderName string                 `json:"provider_name"`
	Success      bool                   `json:"success"`
	Data         map[string]interface{} `json:"data,omitempty"`
	Error        string                 `json:"error,omitempty"`
	Duration     time.Duration          `json:"duration"`
	Timestamp    time.Time              `json:"timestamp"`
}

// Metrics types

// ProviderMetrics contains performance metrics for a provider
type ProviderMetrics struct {
	ProviderName      string        `json:"provider_name"`
	RequestCount      int64         `json:"request_count"`
	SuccessCount      int64         `json:"success_count"`
	ErrorCount        int64         `json:"error_count"`
	SuccessRate       float64       `json:"success_rate"`
	AvgResponseTime   time.Duration `json:"avg_response_time"`
	MinResponseTime   time.Duration `json:"min_response_time"`
	MaxResponseTime   time.Duration `json:"max_response_time"`
	LastRequestTime   time.Time     `json:"last_request_time"`
	
	// Circuit breaker metrics
	CircuitState      CircuitState  `json:"circuit_state"`
	CircuitOpenCount  int64         `json:"circuit_open_count"`
	
	// Rate limiting metrics
	ThrottledRequests int64         `json:"throttled_requests"`
	QuotaExceeded     int64         `json:"quota_exceeded"`
	
	// Data metrics
	RecordsProcessed  int64         `json:"records_processed"`
	DataSyncCount     int64         `json:"data_sync_count"`
	LastSyncTime      time.Time     `json:"last_sync_time"`
}

// AggregateMetrics contains aggregated metrics across all providers
type AggregateMetrics struct {
	TotalProviders    int                            `json:"total_providers"`
	ActiveProviders   int                            `json:"active_providers"`
	HealthyProviders  int                            `json:"healthy_providers"`
	TotalRequests     int64                          `json:"total_requests"`
	TotalErrors       int64                          `json:"total_errors"`
	OverallSuccessRate float64                       `json:"overall_success_rate"`
	AvgResponseTime   time.Duration                  `json:"avg_response_time"`
	
	// Provider breakdown
	ProviderMetrics   map[string]*ProviderMetrics    `json:"provider_metrics"`
	ProviderTypes     map[dnc.ProviderType]int       `json:"provider_types"`
	
	// Health breakdown
	HealthyByType     map[dnc.ProviderType]int       `json:"healthy_by_type"`
	ErrorsByType      map[dnc.ProviderType]int64     `json:"errors_by_type"`
	
	// Time window
	TimeWindow        time.Duration                  `json:"time_window"`
	CollectedAt       time.Time                      `json:"collected_at"`
}

// Error definitions

// ProviderError represents provider-specific errors
type ProviderError struct {
	Code     string `json:"code"`
	Message  string `json:"message"`
	Provider string `json:"provider"`
	Retry    bool   `json:"retry"`
}

func (e *ProviderError) Error() string {
	return e.Message
}

// Standard error codes
const (
	ErrCodeConnectionFailed   = "CONNECTION_FAILED"
	ErrCodeAuthenticationFailed = "AUTH_FAILED"
	ErrCodeRateLimitExceeded  = "RATE_LIMIT_EXCEEDED"
	ErrCodeQuotaExceeded      = "QUOTA_EXCEEDED"
	ErrCodeInvalidRequest     = "INVALID_REQUEST"
	ErrCodeInvalidResponse    = "INVALID_RESPONSE"
	ErrCodeTimeout           = "TIMEOUT"
	ErrCodeProviderUnavailable = "PROVIDER_UNAVAILABLE"
	ErrCodeDataFormatError   = "DATA_FORMAT_ERROR"
	ErrCodeConfigurationError = "CONFIGURATION_ERROR"
)