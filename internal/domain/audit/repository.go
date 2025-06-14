package audit

import (
	"context"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/google/uuid"
)

// EventRepository defines the interface for audit event persistence
// Following DCE patterns: context support, domain-specific errors, performance optimization
type EventRepository interface {
	// Core CRUD operations
	
	// Store persists a single audit event with integrity checks
	// Returns error if sequence number conflicts or validation fails
	Store(ctx context.Context, event *Event) error
	
	// StoreBatch persists multiple events atomically for high throughput
	// All events succeed or all fail - maintains hash chain integrity
	StoreBatch(ctx context.Context, events []*Event) error
	
	// GetByID retrieves an event by its unique identifier
	GetByID(ctx context.Context, id uuid.UUID) (*Event, error)
	
	// GetBySequence retrieves an event by its sequence number
	// Sequence numbers are globally unique and monotonic
	GetBySequence(ctx context.Context, seq values.SequenceNumber) (*Event, error)
	
	// Sequence and chain operations
	
	// GetNextSequenceNumber returns the next available sequence number
	// Thread-safe and guarantees monotonic ordering
	GetNextSequenceNumber(ctx context.Context) (values.SequenceNumber, error)
	
	// GetLatestSequenceNumber returns the highest sequence number in use
	GetLatestSequenceNumber(ctx context.Context) (values.SequenceNumber, error)
	
	// GetSequenceRange retrieves events within a sequence number range
	// Useful for batch processing and integrity verification
	GetSequenceRange(ctx context.Context, start, end values.SequenceNumber) ([]*Event, error)
	
	// Query and filtering operations
	
	// GetEvents retrieves events based on filtering criteria with pagination
	GetEvents(ctx context.Context, filter EventFilter) (*EventPage, error)
	
	// Count returns the total number of events matching the filter
	Count(ctx context.Context, filter EventFilter) (int64, error)
	
	// GetEventsForActor retrieves all events for a specific actor
	GetEventsForActor(ctx context.Context, actorID string, filter EventFilter) (*EventPage, error)
	
	// GetEventsForTarget retrieves all events for a specific target
	GetEventsForTarget(ctx context.Context, targetID string, filter EventFilter) (*EventPage, error)
	
	// GetEventsByTimeRange retrieves events within a time range
	GetEventsByTimeRange(ctx context.Context, start, end time.Time, filter EventFilter) (*EventPage, error)
	
	// GetEventsByType retrieves events of specific types
	GetEventsByType(ctx context.Context, eventTypes []EventType, filter EventFilter) (*EventPage, error)
	
	// Compliance and retention operations
	
	// GetEventsForCompliance retrieves events relevant for compliance reporting
	GetEventsForCompliance(ctx context.Context, flags []string, filter EventFilter) (*EventPage, error)
	
	// GetExpiredEvents returns events that have exceeded their retention period
	GetExpiredEvents(ctx context.Context, before time.Time, limit int) ([]*Event, error)
	
	// GetGDPRRelevantEvents returns events containing PII or GDPR-relevant data
	GetGDPRRelevantEvents(ctx context.Context, dataSubject string, filter EventFilter) (*EventPage, error)
	
	// GetTCPARelevantEvents returns events relevant for TCPA compliance
	GetTCPARelevantEvents(ctx context.Context, phoneNumber string, filter EventFilter) (*EventPage, error)
	
	// Integrity and verification operations
	
	// VerifyEventIntegrity verifies the cryptographic hash of an event
	VerifyEventIntegrity(ctx context.Context, eventID uuid.UUID) (*IntegrityResult, error)
	
	// VerifyChainIntegrity verifies the hash chain for a range of events
	VerifyChainIntegrity(ctx context.Context, start, end values.SequenceNumber) (*ChainIntegrityResult, error)
	
	// GetIntegrityReport generates a comprehensive integrity report
	GetIntegrityReport(ctx context.Context, criteria IntegrityCriteria) (*IntegrityReport, error)
	
	// Performance and monitoring operations
	
	// GetStats returns repository performance statistics
	GetStats(ctx context.Context) (*RepositoryStats, error)
	
	// GetHealthCheck performs health check on the repository
	GetHealthCheck(ctx context.Context) (*HealthCheckResult, error)
	
	// Administrative operations
	
	// GetStorageInfo returns information about storage usage
	GetStorageInfo(ctx context.Context) (*StorageInfo, error)
	
	// Vacuum performs maintenance operations (index optimization, cleanup)
	Vacuum(ctx context.Context) error
}

// EventFilter defines filtering options for event queries
type EventFilter struct {
	// Event type filters
	Types []EventType `json:"types,omitempty"`
	
	// Severity filters
	Severities []Severity `json:"severities,omitempty"`
	
	// Category filters
	Categories []string `json:"categories,omitempty"`
	
	// Actor filters
	ActorIDs   []string `json:"actor_ids,omitempty"`
	ActorTypes []string `json:"actor_types,omitempty"`
	
	// Target filters
	TargetIDs   []string `json:"target_ids,omitempty"`
	TargetTypes []string `json:"target_types,omitempty"`
	
	// Action filters
	Actions []string `json:"actions,omitempty"`
	Results []string `json:"results,omitempty"`
	
	// Time range filters
	StartTime *time.Time `json:"start_time,omitempty"`
	EndTime   *time.Time `json:"end_time,omitempty"`
	
	// Sequence filters
	SequenceStart *values.SequenceNumber `json:"sequence_start,omitempty"`
	SequenceEnd   *values.SequenceNumber `json:"sequence_end,omitempty"`
	
	// Request correlation filters
	RequestIDs     []string `json:"request_ids,omitempty"`
	SessionIDs     []string `json:"session_ids,omitempty"`
	CorrelationIDs []string `json:"correlation_ids,omitempty"`
	
	// Service filters
	Environments []string `json:"environments,omitempty"`
	Services     []string `json:"services,omitempty"`
	
	// Compliance filters
	ComplianceFlags []string `json:"compliance_flags,omitempty"`
	DataClasses     []string `json:"data_classes,omitempty"`
	LegalBasis      []string `json:"legal_basis,omitempty"`
	
	// Text search (metadata, error messages, etc.)
	SearchText string `json:"search_text,omitempty"`
	
	// Tag filters
	Tags []string `json:"tags,omitempty"`
	
	// Error filters
	ErrorCodes    []string `json:"error_codes,omitempty"`
	HasErrors     *bool    `json:"has_errors,omitempty"`
	
	// Retention filters
	RetentionExpired *bool `json:"retention_expired,omitempty"`
	
	// Pagination
	Limit  int    `json:"limit,omitempty"`
	Offset int    `json:"offset,omitempty"`
	Cursor string `json:"cursor,omitempty"` // For cursor-based pagination
	
	// Sorting
	OrderBy   string `json:"order_by,omitempty"`   // Field to sort by
	OrderDesc bool   `json:"order_desc,omitempty"` // Sort direction
	
	// Performance options
	IncludeMetadata bool `json:"include_metadata,omitempty"` // Include full metadata in results
	HintsOnly       bool `json:"hints_only,omitempty"`       // Return minimal data for performance
}

// EventPage represents a paginated result set of events
type EventPage struct {
	Events     []*Event `json:"events"`
	TotalCount int64    `json:"total_count"`
	HasMore    bool     `json:"has_more"`
	NextCursor string   `json:"next_cursor,omitempty"`
	
	// Performance metadata
	QueryTime    time.Duration `json:"query_time"`
	DatabaseHits int           `json:"database_hits"`
	CacheHits    int           `json:"cache_hits"`
}

// IntegrityResult represents the result of an event integrity verification
type IntegrityResult struct {
	EventID   uuid.UUID `json:"event_id"`
	IsValid   bool      `json:"is_valid"`
	HashValid bool      `json:"hash_valid"`
	
	// Detailed validation results
	ComputedHash  string    `json:"computed_hash"`
	StoredHash    string    `json:"stored_hash"`
	PreviousHash  string    `json:"previous_hash"`
	VerifiedAt    time.Time `json:"verified_at"`
	
	// Error information
	Errors   []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

// ChainIntegrityResult represents the result of hash chain verification
type ChainIntegrityResult struct {
	StartSequence values.SequenceNumber `json:"start_sequence"`
	EndSequence   values.SequenceNumber `json:"end_sequence"`
	EventsChecked int64                 `json:"events_checked"`
	IsValid       bool                  `json:"is_valid"`
	
	// Broken chain information
	BrokenAt     *values.SequenceNumber `json:"broken_at,omitempty"`
	BrokenReason string                 `json:"broken_reason,omitempty"`
	
	// Performance metrics
	CheckTime   time.Duration `json:"check_time"`
	VerifiedAt  time.Time     `json:"verified_at"`
	
	// Error details
	Errors   []ChainError `json:"errors,omitempty"`
	Warnings []string     `json:"warnings,omitempty"`
}

// ChainError represents an error in the hash chain
type ChainError struct {
	Sequence values.SequenceNumber `json:"sequence"`
	EventID  uuid.UUID             `json:"event_id"`
	Type     string                `json:"type"`
	Message  string                `json:"message"`
}

// IntegrityCriteria defines criteria for integrity reporting
type IntegrityCriteria struct {
	// Time range for checking
	StartTime *time.Time `json:"start_time,omitempty"`
	EndTime   *time.Time `json:"end_time,omitempty"`
	
	// Sequence range for checking
	StartSequence *values.SequenceNumber `json:"start_sequence,omitempty"`
	EndSequence   *values.SequenceNumber `json:"end_sequence,omitempty"`
	
	// Verification depth
	DeepVerification bool `json:"deep_verification"` // Verify cryptographic signatures
	CheckReferences  bool `json:"check_references"`  // Verify referenced entities exist
	
	// Performance limits
	MaxEvents  int           `json:"max_events,omitempty"`
	Timeout    time.Duration `json:"timeout,omitempty"`
	
	// Specific checks
	CheckHashChain    bool `json:"check_hash_chain"`
	CheckSequencing   bool `json:"check_sequencing"`
	CheckMetadata     bool `json:"check_metadata"`
	CheckCompliance   bool `json:"check_compliance"`
}

// IntegrityReport provides comprehensive integrity analysis
type IntegrityReport struct {
	// Report metadata
	GeneratedAt time.Time             `json:"generated_at"`
	Criteria    IntegrityCriteria     `json:"criteria"`
	
	// Overall status
	OverallStatus string `json:"overall_status"` // HEALTHY, DEGRADED, CRITICAL
	IsHealthy     bool   `json:"is_healthy"`
	
	// Event statistics
	TotalEvents    int64 `json:"total_events"`
	VerifiedEvents int64 `json:"verified_events"`
	FailedEvents   int64 `json:"failed_events"`
	
	// Chain integrity
	ChainResult *ChainIntegrityResult `json:"chain_result,omitempty"`
	
	// Sequence analysis
	SequenceGaps    []SequenceGap    `json:"sequence_gaps,omitempty"`
	DuplicateEvents []DuplicateEvent `json:"duplicate_events,omitempty"`
	
	// Compliance status
	ComplianceIssues []ComplianceIssue `json:"compliance_issues,omitempty"`
	
	// Performance metrics
	VerificationTime time.Duration `json:"verification_time"`
	DatabaseQueries  int           `json:"database_queries"`
	
	// Recommendations
	Recommendations []string `json:"recommendations,omitempty"`
	
	// Error summary
	CriticalErrors []string `json:"critical_errors,omitempty"`
	Warnings       []string `json:"warnings,omitempty"`
}

// SequenceGap is defined in integrity_repository.go

// DuplicateEvent represents a duplicate sequence number
type DuplicateEvent struct {
	Sequence values.SequenceNumber `json:"sequence"`
	EventIDs []uuid.UUID           `json:"event_ids"`
}

// ComplianceIssue represents a compliance-related integrity issue
type ComplianceIssue struct {
	Type        string    `json:"type"`
	Severity    string    `json:"severity"`
	EventID     uuid.UUID `json:"event_id"`
	Sequence    values.SequenceNumber `json:"sequence"`
	Description string    `json:"description"`
	Impact      string    `json:"impact"`
}

// RepositoryStats provides performance and usage statistics
type RepositoryStats struct {
	// Event counts
	TotalEvents       int64 `json:"total_events"`
	EventsToday       int64 `json:"events_today"`
	EventsThisWeek    int64 `json:"events_this_week"`
	EventsThisMonth   int64 `json:"events_this_month"`
	
	// Performance metrics
	AverageInsertTime time.Duration `json:"average_insert_time"`
	AverageQueryTime  time.Duration `json:"average_query_time"`
	QueryThroughput   float64       `json:"query_throughput"` // Queries per second
	
	// Cache statistics
	CacheHitRate      float64 `json:"cache_hit_rate"`
	CacheSize         int64   `json:"cache_size"`
	CacheEvictions    int64   `json:"cache_evictions"`
	
	// Error rates
	ErrorRate         float64 `json:"error_rate"`
	IntegrityFailures int64   `json:"integrity_failures"`
	
	// Sequence information
	LatestSequence    values.SequenceNumber `json:"latest_sequence"`
	SequenceGaps      int64                 `json:"sequence_gaps"`
	
	// Compliance metrics
	GDPREvents        int64 `json:"gdpr_events"`
	TCPAEvents        int64 `json:"tcpa_events"`
	ExpiredEvents     int64 `json:"expired_events"`
	
	// Collection timestamp
	CollectedAt       time.Time `json:"collected_at"`
}

// HealthCheckResult represents the health status of the repository
type HealthCheckResult struct {
	Status      string    `json:"status"` // HEALTHY, DEGRADED, UNHEALTHY
	Healthy     bool      `json:"healthy"`
	CheckedAt   time.Time `json:"checked_at"`
	ResponseTime time.Duration `json:"response_time"`
	
	// Individual check results
	DatabaseHealth   bool `json:"database_health"`
	SequenceHealth   bool `json:"sequence_health"`
	IntegrityHealth  bool `json:"integrity_health"`
	PerformanceHealth bool `json:"performance_health"`
	
	// Error information
	Errors   []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
	
	// Detailed metrics
	Metrics map[string]interface{} `json:"metrics,omitempty"`
}

// StorageInfo provides information about repository storage usage
type StorageInfo struct {
	// Storage usage
	TotalSize       int64 `json:"total_size"`       // Total storage used in bytes
	IndexSize       int64 `json:"index_size"`       // Index storage in bytes
	DataSize        int64 `json:"data_size"`        // Data storage in bytes
	
	// Growth metrics
	DailyGrowth     int64 `json:"daily_growth"`     // Daily growth in bytes
	WeeklyGrowth    int64 `json:"weekly_growth"`    // Weekly growth in bytes
	MonthlyGrowth   int64 `json:"monthly_growth"`   // Monthly growth in bytes
	
	// Partitioning information (if applicable)
	Partitions      []PartitionInfo `json:"partitions,omitempty"`
	
	// Compression information
	CompressionRatio float64 `json:"compression_ratio,omitempty"`
	
	// Archival information
	ArchivedSize    int64 `json:"archived_size"`
	ArchiveLocation string `json:"archive_location,omitempty"`
	
	// Collection timestamp
	CollectedAt     time.Time `json:"collected_at"`
}

// PartitionInfo provides information about a storage partition
type PartitionInfo struct {
	Name        string    `json:"name"`
	StartTime   time.Time `json:"start_time"`
	EndTime     time.Time `json:"end_time"`
	Size        int64     `json:"size"`
	EventCount  int64     `json:"event_count"`
	Status      string    `json:"status"` // ACTIVE, ARCHIVED, COMPRESSED
}