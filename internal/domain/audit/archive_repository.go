package audit

import (
	"context"
	"io"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/google/uuid"
)

// ArchiveRepository defines the interface for audit data archival and cold storage operations
// Handles lifecycle management for aged audit data following compliance requirements
type ArchiveRepository interface {
	// Archive operations
	
	// ArchiveEvents moves events to cold storage based on retention policies
	// Events are compressed and stored in an immutable format
	ArchiveEvents(ctx context.Context, criteria ArchiveCriteria) (*ArchiveResult, error)
	
	// ArchiveBatch archives a specific batch of events atomically
	ArchiveBatch(ctx context.Context, eventIDs []uuid.UUID, archiveLocation string) (*ArchiveResult, error)
	
	// ArchiveByTimeRange archives all events within a time range
	ArchiveByTimeRange(ctx context.Context, start, end time.Time, location string) (*ArchiveResult, error)
	
	// ArchiveBySequenceRange archives events within a sequence number range
	ArchiveBySequenceRange(ctx context.Context, start, end values.SequenceNumber, location string) (*ArchiveResult, error)
	
	// Retrieval operations
	
	// RestoreEvents retrieves archived events back to active storage
	// Used for compliance investigations or data recovery
	RestoreEvents(ctx context.Context, archiveID string, eventIDs []uuid.UUID) (*RestoreResult, error)
	
	// GetArchivedEvent retrieves a single archived event without full restoration
	GetArchivedEvent(ctx context.Context, archiveID string, eventID uuid.UUID) (*Event, error)
	
	// GetArchivedEvents retrieves multiple archived events with filtering
	GetArchivedEvents(ctx context.Context, archiveID string, filter ArchiveFilter) (*ArchivedEventPage, error)
	
	// SearchArchivedEvents performs content search across archived data
	SearchArchivedEvents(ctx context.Context, criteria SearchCriteria) (*ArchiveSearchResult, error)
	
	// Export operations
	
	// ExportArchive exports archived data in specified format
	ExportArchive(ctx context.Context, archiveID string, format values.ExportFormat) (io.Reader, error)
	
	// ExportToStream streams archived data for large exports
	ExportToStream(ctx context.Context, archiveID string, format values.ExportFormat, writer io.Writer) error
	
	// CreateSnapshot creates a point-in-time snapshot of archived data
	CreateSnapshot(ctx context.Context, criteria SnapshotCriteria) (*SnapshotResult, error)
	
	// Archive management operations
	
	// ListArchives returns all available archives with metadata
	ListArchives(ctx context.Context, filter ArchiveListFilter) (*ArchiveList, error)
	
	// GetArchiveInfo returns detailed information about an archive
	GetArchiveInfo(ctx context.Context, archiveID string) (*ArchiveInfo, error)
	
	// ValidateArchive verifies the integrity of an archived dataset
	ValidateArchive(ctx context.Context, archiveID string) (*ArchiveValidationResult, error)
	
	// Lifecycle management operations
	
	// GetRetentionPolicy returns the current retention policy
	GetRetentionPolicy(ctx context.Context) (*RetentionPolicy, error)
	
	// UpdateRetentionPolicy updates the retention policy
	UpdateRetentionPolicy(ctx context.Context, policy *RetentionPolicy) error
	
	// ApplyRetentionPolicy processes events according to retention rules
	ApplyRetentionPolicy(ctx context.Context) (*RetentionResult, error)
	
	// GetExpiringEvents returns events approaching retention expiry
	GetExpiringEvents(ctx context.Context, days int) ([]*Event, error)
	
	// Storage operations
	
	// CompactArchive optimizes storage by recompressing or reorganizing
	CompactArchive(ctx context.Context, archiveID string) (*CompactionResult, error)
	
	// MigrateArchive moves archive to different storage tier or location
	MigrateArchive(ctx context.Context, archiveID string, targetLocation string) (*MigrationResult, error)
	
	// DeleteArchive permanently removes an archive (compliance permitting)
	DeleteArchive(ctx context.Context, archiveID string, reason string) error
	
	// GetStorageMetrics returns storage utilization metrics
	GetStorageMetrics(ctx context.Context) (*ArchiveStorageMetrics, error)
	
	// Compliance operations
	
	// GetComplianceReport generates archive compliance report
	GetComplianceReport(ctx context.Context, criteria ComplianceReportCriteria) (*ArchiveComplianceReport, error)
	
	// VerifyCompliance checks archive against compliance requirements
	VerifyCompliance(ctx context.Context, archiveID string, standards []string) (*ComplianceVerificationResult, error)
	
	// ApplyLegalHold places or removes legal hold on archives
	ApplyLegalHold(ctx context.Context, archiveID string, hold *LegalHold) error
	
	// GetLegalHolds returns all active legal holds
	GetLegalHolds(ctx context.Context) ([]*LegalHold, error)
}

// ArchiveCriteria defines criteria for archiving events
type ArchiveCriteria struct {
	// Time-based criteria
	OlderThan     *time.Time `json:"older_than,omitempty"`
	RetentionDays *int       `json:"retention_days,omitempty"`
	
	// Sequence-based criteria
	SequenceBefore *values.SequenceNumber `json:"sequence_before,omitempty"`
	
	// Event filtering
	EventTypes  []EventType `json:"event_types,omitempty"`
	Categories  []string    `json:"categories,omitempty"`
	Severities  []Severity  `json:"severities,omitempty"`
	
	// Compliance filtering
	ComplianceFlags []string `json:"compliance_flags,omitempty"`
	DataClasses     []string `json:"data_classes,omitempty"`
	
	// Archive configuration
	ArchiveLocation string                `json:"archive_location"`
	CompressionType string                `json:"compression_type,omitempty"` // gzip, lz4, zstd
	EncryptionKey   string                `json:"encryption_key,omitempty"`
	StorageClass    string                `json:"storage_class,omitempty"`    // STANDARD, COLD, GLACIER
	Format          values.ExportFormat   `json:"format"`
	
	// Processing options
	BatchSize       int           `json:"batch_size,omitempty"`
	MaxEvents       int64         `json:"max_events,omitempty"`
	Timeout         time.Duration `json:"timeout,omitempty"`
	VerifyIntegrity bool          `json:"verify_integrity"`
	CreateIndex     bool          `json:"create_index"`
}

// ArchiveResult represents the result of an archive operation
type ArchiveResult struct {
	ArchiveID       string    `json:"archive_id"`
	EventsArchived  int64     `json:"events_archived"`
	EventsSkipped   int64     `json:"events_skipped"`
	EventsFailed    int64     `json:"events_failed"`
	
	// Size and performance
	OriginalSize    int64         `json:"original_size"`
	CompressedSize  int64         `json:"compressed_size"`
	CompressionRatio float64      `json:"compression_ratio"`
	ProcessingTime  time.Duration `json:"processing_time"`
	
	// Archive metadata
	ArchiveLocation string              `json:"archive_location"`
	ArchiveFormat   values.ExportFormat `json:"archive_format"`
	EncryptionUsed  bool                `json:"encryption_used"`
	
	// Time information
	StartTime       time.Time `json:"start_time"`
	CompletedAt     time.Time `json:"completed_at"`
	
	// Integrity information
	IntegrityHash   string `json:"integrity_hash"`
	EventCount      int64  `json:"event_count"`
	SequenceRange   *SequenceRange `json:"sequence_range,omitempty"`
	
	// Error information
	Errors          []ArchiveError `json:"errors,omitempty"`
	Warnings        []string       `json:"warnings,omitempty"`
}

// SequenceRange represents a range of sequence numbers in an archive
type SequenceRange struct {
	Start values.SequenceNumber `json:"start"`
	End   values.SequenceNumber `json:"end"`
}

// ArchiveError represents an error during archiving
type ArchiveError struct {
	EventID uuid.UUID `json:"event_id"`
	Error   string    `json:"error"`
	Code    string    `json:"code"`
}

// RestoreResult represents the result of a restore operation
type RestoreResult struct {
	RestoreID       string        `json:"restore_id"`
	EventsRestored  int64         `json:"events_restored"`
	EventsFailed    int64         `json:"events_failed"`
	RestoredSize    int64         `json:"restored_size"`
	ProcessingTime  time.Duration `json:"processing_time"`
	CompletedAt     time.Time     `json:"completed_at"`
	
	// Error information
	Errors          []RestoreError `json:"errors,omitempty"`
}

// RestoreError represents an error during restoration
type RestoreError struct {
	EventID uuid.UUID `json:"event_id"`
	Error   string    `json:"error"`
	Code    string    `json:"code"`
}

// ArchiveFilter defines filtering for archived event queries
type ArchiveFilter struct {
	// Event identification
	EventIDs []uuid.UUID `json:"event_ids,omitempty"`
	
	// Event classification
	Types      []EventType `json:"types,omitempty"`
	Categories []string    `json:"categories,omitempty"`
	Severities []Severity  `json:"severities,omitempty"`
	
	// Actor/Target filters
	ActorIDs  []string `json:"actor_ids,omitempty"`
	TargetIDs []string `json:"target_ids,omitempty"`
	
	// Time filters
	StartTime *time.Time `json:"start_time,omitempty"`
	EndTime   *time.Time `json:"end_time,omitempty"`
	
	// Sequence filters
	SequenceStart *values.SequenceNumber `json:"sequence_start,omitempty"`
	SequenceEnd   *values.SequenceNumber `json:"sequence_end,omitempty"`
	
	// Compliance filters
	ComplianceFlags []string `json:"compliance_flags,omitempty"`
	DataClasses     []string `json:"data_classes,omitempty"`
	
	// Text search
	SearchText string `json:"search_text,omitempty"`
	
	// Pagination
	Limit  int    `json:"limit,omitempty"`
	Offset int    `json:"offset,omitempty"`
	Cursor string `json:"cursor,omitempty"`
	
	// Performance options
	IncludeMetadata bool `json:"include_metadata,omitempty"`
}

// ArchivedEventPage represents paginated archived events
type ArchivedEventPage struct {
	Events      []*Event  `json:"events"`
	TotalCount  int64     `json:"total_count"`
	HasMore     bool      `json:"has_more"`
	NextCursor  string    `json:"next_cursor,omitempty"`
	ArchiveID   string    `json:"archive_id"`
	QueryTime   time.Duration `json:"query_time"`
	
	// Archive metadata
	ArchiveInfo *ArchiveInfo `json:"archive_info,omitempty"`
}

// SearchCriteria defines criteria for cross-archive searches
type SearchCriteria struct {
	// Archive scope
	ArchiveIDs []string `json:"archive_ids,omitempty"` // Empty means search all
	
	// Time range
	StartTime *time.Time `json:"start_time,omitempty"`
	EndTime   *time.Time `json:"end_time,omitempty"`
	
	// Content search
	SearchText   string   `json:"search_text"`
	SearchFields []string `json:"search_fields,omitempty"` // Fields to search in
	
	// Event filters
	Types       []EventType `json:"types,omitempty"`
	Categories  []string    `json:"categories,omitempty"`
	ActorIDs    []string    `json:"actor_ids,omitempty"`
	TargetIDs   []string    `json:"target_ids,omitempty"`
	
	// Compliance filters
	ComplianceFlags []string `json:"compliance_flags,omitempty"`
	DataClasses     []string `json:"data_classes,omitempty"`
	
	// Performance limits
	MaxResults  int           `json:"max_results,omitempty"`
	Timeout     time.Duration `json:"timeout,omitempty"`
	
	// Result options
	IncludeMetadata bool `json:"include_metadata"`
	HighlightMatches bool `json:"highlight_matches"`
}

// ArchiveSearchResult represents search results across archives
type ArchiveSearchResult struct {
	Results     []*ArchiveSearchMatch `json:"results"`
	TotalCount  int64                 `json:"total_count"`
	SearchTime  time.Duration         `json:"search_time"`
	ArchivesSearched int              `json:"archives_searched"`
	
	// Query information
	Query       SearchCriteria `json:"query"`
	ExecutedAt  time.Time      `json:"executed_at"`
}

// ArchiveSearchMatch represents a single search match
type ArchiveSearchMatch struct {
	Event       *Event            `json:"event"`
	ArchiveID   string            `json:"archive_id"`
	Score       float64           `json:"score"`          // Relevance score
	Highlights  map[string]string `json:"highlights,omitempty"` // Field -> highlighted text
	Context     map[string]interface{} `json:"context,omitempty"`
}

// SnapshotCriteria defines criteria for creating archive snapshots
type SnapshotCriteria struct {
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	
	// Scope
	ArchiveIDs  []string   `json:"archive_ids,omitempty"`
	StartTime   *time.Time `json:"start_time,omitempty"`
	EndTime     *time.Time `json:"end_time,omitempty"`
	
	// Output configuration
	Format         values.ExportFormat `json:"format"`
	Compression    string              `json:"compression,omitempty"`
	Encryption     bool                `json:"encryption"`
	StorageClass   string              `json:"storage_class,omitempty"`
	
	// Metadata
	Tags        map[string]string `json:"tags,omitempty"`
	RetentionDays int             `json:"retention_days,omitempty"`
}

// SnapshotResult represents the result of snapshot creation
type SnapshotResult struct {
	SnapshotID     string                `json:"snapshot_id"`
	Name           string                `json:"name"`
	EventsIncluded int64                 `json:"events_included"`
	SnapshotSize   int64                 `json:"snapshot_size"`
	Format         values.ExportFormat   `json:"format"`
	CreatedAt      time.Time             `json:"created_at"`
	ProcessingTime time.Duration         `json:"processing_time"`
	Location       string                `json:"location"`
	IntegrityHash  string                `json:"integrity_hash"`
}

// ArchiveListFilter defines filtering for archive listing
type ArchiveListFilter struct {
	// Time filters
	CreatedAfter  *time.Time `json:"created_after,omitempty"`
	CreatedBefore *time.Time `json:"created_before,omitempty"`
	
	// Status filters
	Status []string `json:"status,omitempty"` // ACTIVE, ARCHIVED, COMPRESSED, DELETED
	
	// Storage filters
	StorageClass []string `json:"storage_class,omitempty"`
	Locations    []string `json:"locations,omitempty"`
	
	// Size filters
	MinSize *int64 `json:"min_size,omitempty"`
	MaxSize *int64 `json:"max_size,omitempty"`
	
	// Compliance filters
	LegalHold    *bool    `json:"legal_hold,omitempty"`
	ComplianceStandards []string `json:"compliance_standards,omitempty"`
	
	// Pagination
	Limit  int `json:"limit,omitempty"`
	Offset int `json:"offset,omitempty"`
	
	// Sorting
	OrderBy   string `json:"order_by,omitempty"`   // created_at, size, event_count
	OrderDesc bool   `json:"order_desc,omitempty"`
}

// ArchiveList represents a list of archives with metadata
type ArchiveList struct {
	Archives   []*ArchiveInfo `json:"archives"`
	TotalCount int64          `json:"total_count"`
	HasMore    bool           `json:"has_more"`
}

// ArchiveInfo provides detailed information about an archive
type ArchiveInfo struct {
	ArchiveID       string                `json:"archive_id"`
	Name            string                `json:"name,omitempty"`
	Description     string                `json:"description,omitempty"`
	
	// Content information
	EventCount      int64                 `json:"event_count"`
	SequenceRange   *SequenceRange        `json:"sequence_range,omitempty"`
	TimeRange       *TimeRange            `json:"time_range,omitempty"`
	
	// Storage information
	OriginalSize    int64                 `json:"original_size"`
	CompressedSize  int64                 `json:"compressed_size"`
	CompressionRatio float64              `json:"compression_ratio"`
	StorageClass    string                `json:"storage_class"`
	Location        string                `json:"location"`
	
	// Format information
	Format          values.ExportFormat   `json:"format"`
	CompressionType string                `json:"compression_type,omitempty"`
	EncryptionUsed  bool                  `json:"encryption_used"`
	
	// Status information
	Status          string                `json:"status"`
	Health          string                `json:"health"` // HEALTHY, DEGRADED, CORRUPTED
	LastVerified    *time.Time            `json:"last_verified,omitempty"`
	
	// Lifecycle information
	CreatedAt       time.Time             `json:"created_at"`
	ArchivedAt      time.Time             `json:"archived_at"`
	ExpiresAt       *time.Time            `json:"expires_at,omitempty"`
	
	// Compliance information
	RetentionPolicy *RetentionPolicy      `json:"retention_policy,omitempty"`
	LegalHolds      []*LegalHold          `json:"legal_holds,omitempty"`
	ComplianceFlags []string              `json:"compliance_flags,omitempty"`
	
	// Integrity information
	IntegrityHash   string                `json:"integrity_hash"`
	LastIntegrityCheck *time.Time         `json:"last_integrity_check,omitempty"`
	
	// Metadata
	Tags            map[string]string     `json:"tags,omitempty"`
	Creator         string                `json:"creator,omitempty"`
	
	// Access information
	LastAccessed    *time.Time            `json:"last_accessed,omitempty"`
	AccessCount     int64                 `json:"access_count"`
}

// TimeRange represents a time range in an archive
type TimeRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// RetentionPolicy defines how long different types of events should be retained
type RetentionPolicy struct {
	// Default retention
	DefaultRetentionDays int `json:"default_retention_days"`
	
	// Type-specific retention
	TypeRetention map[EventType]int `json:"type_retention,omitempty"`
	
	// Compliance-specific retention
	GDPRRetentionDays int `json:"gdpr_retention_days,omitempty"`
	TCPARetentionDays int `json:"tcpa_retention_days,omitempty"`
	
	// Category-specific retention
	CategoryRetention map[string]int `json:"category_retention,omitempty"`
	
	// Severity-specific retention
	SeverityRetention map[Severity]int `json:"severity_retention,omitempty"`
	
	// Legal hold override
	LegalHoldOverride bool `json:"legal_hold_override"`
	
	// Policy metadata
	Version     int       `json:"version"`
	EffectiveAt time.Time `json:"effective_at"`
	CreatedBy   string    `json:"created_by"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// RetentionResult represents the result of applying retention policy
type RetentionResult struct {
	ProcessingTime    time.Duration `json:"processing_time"`
	EventsProcessed   int64         `json:"events_processed"`
	EventsArchived    int64         `json:"events_archived"`
	EventsDeleted     int64         `json:"events_deleted"`
	EventsPreserved   int64         `json:"events_preserved"` // Due to legal hold
	
	ArchivesCreated   int           `json:"archives_created"`
	StorageFreed      int64         `json:"storage_freed"`
	
	Errors            []RetentionError `json:"errors,omitempty"`
	ExecutedAt        time.Time     `json:"executed_at"`
}

// RetentionError represents an error during retention processing
type RetentionError struct {
	EventID uuid.UUID `json:"event_id"`
	Error   string    `json:"error"`
	Code    string    `json:"code"`
}

// LegalHold represents a legal hold placed on archived data
type LegalHold struct {
	HoldID      string    `json:"hold_id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	
	// Scope
	ArchiveIDs  []string   `json:"archive_ids,omitempty"`
	EventTypes  []EventType `json:"event_types,omitempty"`
	ActorIDs    []string   `json:"actor_ids,omitempty"`
	TargetIDs   []string   `json:"target_ids,omitempty"`
	StartTime   *time.Time `json:"start_time,omitempty"`
	EndTime     *time.Time `json:"end_time,omitempty"`
	
	// Legal information
	CaseNumber  string    `json:"case_number,omitempty"`
	Authority   string    `json:"authority"`
	LegalBasis  string    `json:"legal_basis"`
	
	// Lifecycle
	CreatedAt   time.Time  `json:"created_at"`
	CreatedBy   string     `json:"created_by"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	Status      string     `json:"status"` // ACTIVE, EXPIRED, RELEASED
	
	// Contact information
	ContactEmail string    `json:"contact_email,omitempty"`
	ContactPhone string    `json:"contact_phone,omitempty"`
}

// Additional result types and storage metrics types would continue here...
// For brevity, I'll include the key ones needed for the interface