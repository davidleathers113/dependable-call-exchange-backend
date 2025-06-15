package dnc

import (
	"context"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/dnc"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/dnc/services"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/dnc/types"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/google/uuid"
)

// Service defines the main DNC orchestration service interface
// This service coordinates between domain services, repositories, and infrastructure
type Service interface {
	// Core DNC Checking Operations
	
	// CheckDNC performs a comprehensive DNC check with sub-10ms latency
	// Returns cached results when available, performs fresh checks when needed
	CheckDNC(ctx context.Context, phoneNumber *values.PhoneNumber, callTime time.Time) (*DNCCheckResponse, error)
	
	// CheckDNCBulk performs DNC checks for multiple phone numbers efficiently
	// Uses batch operations and parallel processing for optimal performance
	CheckDNCBulk(ctx context.Context, phoneNumbers []*values.PhoneNumber, callTime time.Time) ([]*DNCCheckResponse, error)
	
	// Suppression List Management
	
	// AddToSuppressionList adds a phone number to internal suppression
	// Validates business rules and publishes events for audit trail
	AddToSuppressionList(ctx context.Context, req AddSuppressionRequest) (*SuppressionResponse, error)
	
	// RemoveFromSuppressionList removes a phone number from internal suppression
	// Validates permissions and maintains audit trail
	RemoveFromSuppressionList(ctx context.Context, phoneNumber *values.PhoneNumber, removedBy uuid.UUID, reason string) error
	
	// UpdateSuppressionEntry updates an existing suppression entry
	// Supports reason changes, expiration updates, and metadata modifications
	UpdateSuppressionEntry(ctx context.Context, req UpdateSuppressionRequest) (*SuppressionResponse, error)
	
	// Provider Management and Synchronization
	
	// SyncWithProviders synchronizes DNC data from all configured providers
	// Implements exponential backoff and handles rate limiting
	SyncWithProviders(ctx context.Context) (*SyncResponse, error)
	
	// SyncWithProvider synchronizes data from a specific provider
	// Used for targeted updates and error recovery
	SyncWithProvider(ctx context.Context, providerID uuid.UUID) (*ProviderSyncResponse, error)
	
	// UpdateProvider updates provider configuration and credentials
	// Validates configuration and tests connectivity
	UpdateProvider(ctx context.Context, req UpdateProviderRequest) (*ProviderResponse, error)
	
	// GetProviderStatus returns current status and health of providers
	// Includes sync status, error counts, and performance metrics
	GetProviderStatus(ctx context.Context, providerID uuid.UUID) (*ProviderStatusResponse, error)
	
	// Compliance and Reporting
	
	// GetComplianceReport generates comprehensive compliance reports
	// Supports various criteria and export formats
	GetComplianceReport(ctx context.Context, criteria ComplianceReportCriteria) (*ComplianceReportResponse, error)
	
	// ValidateCall performs full TCPA and DNC validation for a call
	// Includes time zone checking, wireless validation, and consent verification
	ValidateCall(ctx context.Context, req CallValidationRequest) (*CallValidationResponse, error)
	
	// GetRiskAssessment calculates violation risk and potential penalties
	// Uses historical data and machine learning models
	GetRiskAssessment(ctx context.Context, phoneNumber *values.PhoneNumber, callContext CallContext) (*RiskAssessmentResponse, error)
	
	// Administrative Operations
	
	// GetSuppressionEntry retrieves details of a specific suppression entry
	GetSuppressionEntry(ctx context.Context, id uuid.UUID) (*SuppressionResponse, error)
	
	// SearchSuppressions finds suppression entries matching criteria
	// Supports phone number patterns, date ranges, and reason filtering
	SearchSuppressions(ctx context.Context, criteria SearchCriteria) (*SearchResponse, error)
	
	// GetCacheStats returns DNC cache performance statistics
	// Used for monitoring and optimization
	GetCacheStats(ctx context.Context) (*CacheStatsResponse, error)
	
	// ClearCache invalidates DNC cache entries
	// Supports pattern-based clearing for provider updates
	ClearCache(ctx context.Context, pattern string) error
	
	// Health and Monitoring
	
	// HealthCheck validates service health and dependencies
	// Tests database, cache, and external provider connectivity
	HealthCheck(ctx context.Context) (*HealthResponse, error)
}

// Repository interfaces for dependency injection
type DNCEntryRepository interface {
	dnc.DNCEntryRepository
}

type DNCProviderRepository interface {
	dnc.DNCProviderRepository
}

type DNCCheckResultRepository interface {
	dnc.DNCCheckResultRepository
}

// Cache interface for DNC-specific caching operations
type DNCCache interface {
	// Phone number check result caching
	GetCheckResult(ctx context.Context, phoneNumber *values.PhoneNumber) (*dnc.DNCCheckResult, error)
	SetCheckResult(ctx context.Context, result *dnc.DNCCheckResult) error
	
	// Provider data caching
	InvalidateProvider(ctx context.Context, providerID uuid.UUID) error
	InvalidateSource(ctx context.Context, source values.ListSource) error
	
	// Cache management
	GetStats(ctx context.Context) (*CacheStats, error)
	Clear(ctx context.Context, pattern string) error
	WarmCache(ctx context.Context, phoneNumbers []*values.PhoneNumber) error
}

// Event publisher for domain events
type EventPublisher interface {
	// DNC check events
	PublishDNCCheckPerformed(ctx context.Context, event *dnc.DNCCheckPerformedEvent) error
	
	// Suppression events
	PublishNumberSuppressed(ctx context.Context, event *dnc.NumberSuppressedEvent) error
	PublishNumberReleased(ctx context.Context, event *dnc.NumberReleasedEvent) error
	
	// Provider sync events
	PublishDNCListSynced(ctx context.Context, event *dnc.DNCListSyncedEvent) error
}

// Circuit breaker for external provider calls
type CircuitBreaker interface {
	Execute(ctx context.Context, req func() (interface{}, error)) (interface{}, error)
	GetState() CircuitState
	Reset()
}

type CircuitState string

const (
	CircuitClosed   CircuitState = "closed"
	CircuitOpen     CircuitState = "open"
	CircuitHalfOpen CircuitState = "half_open"
)

// Domain services integration
type ComplianceService interface {
	CheckCompliance(ctx context.Context, phoneNumber *values.PhoneNumber, callTime time.Time) (*types.ComplianceResult, error)
	ValidateCall(ctx context.Context, fromNumber, toNumber *values.PhoneNumber, callTime time.Time) (*types.CallValidation, error)
	GetComplianceReport(ctx context.Context, phoneNumber *values.PhoneNumber) (*types.ComplianceReport, error)
}

type RiskAssessmentService interface {
	AssessRisk(ctx context.Context, phoneNumber *values.PhoneNumber, callContext types.CallContext) (*types.RiskAssessment, error)
	CalculatePenalty(ctx context.Context, scenario types.ViolationScenario) (*types.PenaltyCalculation, error)
	GetRiskScore(ctx context.Context, phoneNumber *values.PhoneNumber) (float64, error)
}

type ConflictResolver interface {
	ResolveConflicts(ctx context.Context, phoneNumber *values.PhoneNumber) (*types.ConflictResult, error)
	MergeResults(ctx context.Context, results []*dnc.DNCCheckResult) (*dnc.DNCCheckResult, error)
}

// Time zone service for TCPA compliance
type TimeZoneService interface {
	GetTimeZone(phoneNumber *values.PhoneNumber) (*time.Location, error)
	IsWithinCallingHours(phoneNumber *values.PhoneNumber, callTime time.Time) (bool, error)
}

// Call history service for risk assessment
type CallHistoryService interface {
	GetCallHistory(ctx context.Context, phoneNumber *values.PhoneNumber, days int) (*types.CallHistory, error)
	GetViolationHistory(ctx context.Context, phoneNumber *values.PhoneNumber) (*types.ViolationHistory, error)
}

// External API client interfaces for provider integration
type FederalDNCClient interface {
	CheckNumber(ctx context.Context, phoneNumber *values.PhoneNumber) (*FederalDNCResult, error)
	BulkCheck(ctx context.Context, phoneNumbers []*values.PhoneNumber) ([]*FederalDNCResult, error)
	GetLastUpdateTime(ctx context.Context) (time.Time, error)
}

type StateDNCClient interface {
	CheckNumber(ctx context.Context, phoneNumber *values.PhoneNumber, state string) (*StateDNCResult, error)
	BulkCheck(ctx context.Context, phoneNumbers []*values.PhoneNumber, state string) ([]*StateDNCResult, error)
	GetSupportedStates(ctx context.Context) ([]string, error)
}

// Audit service for compliance logging
type AuditService interface {
	LogDNCCheck(ctx context.Context, req DNCCheckAuditRequest) error
	LogSuppressionChange(ctx context.Context, req SuppressionAuditRequest) error
	LogProviderSync(ctx context.Context, req ProviderSyncAuditRequest) error
}

// Service configuration
type Config struct {
	// Performance settings
	CheckTimeoutMs           int           `yaml:"check_timeout_ms" default:"10"`
	BulkCheckTimeoutMs      int           `yaml:"bulk_check_timeout_ms" default:"50"`
	CacheDefaultTTL         time.Duration `yaml:"cache_default_ttl" default:"6h"`
	
	// Provider sync settings
	SyncIntervalMinutes     int           `yaml:"sync_interval_minutes" default:"60"`
	SyncTimeoutMinutes      int           `yaml:"sync_timeout_minutes" default:"30"`
	MaxRetryAttempts        int           `yaml:"max_retry_attempts" default:"3"`
	RetryBackoffSeconds     int           `yaml:"retry_backoff_seconds" default:"30"`
	
	// Circuit breaker settings
	CircuitBreakerEnabled   bool          `yaml:"circuit_breaker_enabled" default:"true"`
	FailureThreshold        int           `yaml:"failure_threshold" default:"5"`
	TimeoutThreshold        time.Duration `yaml:"timeout_threshold" default:"5s"`
	RecoveryTimeout         time.Duration `yaml:"recovery_timeout" default:"60s"`
	
	// Compliance settings
	StrictModeEnabled       bool          `yaml:"strict_mode_enabled" default:"true"`
	TCPAValidationEnabled   bool          `yaml:"tcpa_validation_enabled" default:"true"`
	WirelessValidationEnabled bool        `yaml:"wireless_validation_enabled" default:"true"`
	
	// Cache warming settings
	WarmCacheOnStartup      bool          `yaml:"warm_cache_on_startup" default:"false"`
	WarmCacheBatchSize      int           `yaml:"warm_cache_batch_size" default:"1000"`
	
	// Performance monitoring
	MetricsEnabled          bool          `yaml:"metrics_enabled" default:"true"`
	SlowQueryThresholdMs    int           `yaml:"slow_query_threshold_ms" default:"5"`
}