package dnc

import (
	"context"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/google/uuid"
)

// DNCEntryRepository defines the interface for DNC entry persistence operations
// Performance expectations:
// - Save: < 10ms for single entry, < 100ms for batch operations
// - FindByPhone: < 5ms with proper indexing
// - BulkInsert: > 10K entries/second throughput
type DNCEntryRepository interface {
	// Core CRUD operations
	
	// Save creates or updates a DNC entry
	// Returns error if phone number format is invalid or duplicate entry exists
	Save(ctx context.Context, entry *DNCEntry) error
	
	// SaveWithTx saves a DNC entry within an existing transaction
	SaveWithTx(ctx context.Context, tx Transaction, entry *DNCEntry) error
	
	// GetByID retrieves a DNC entry by its unique identifier
	GetByID(ctx context.Context, id uuid.UUID) (*DNCEntry, error)
	
	// Delete removes a DNC entry (soft delete for audit trail)
	Delete(ctx context.Context, id uuid.UUID, deletedBy uuid.UUID) error
	
	// DeleteWithTx removes a DNC entry within an existing transaction
	DeleteWithTx(ctx context.Context, tx Transaction, id uuid.UUID, deletedBy uuid.UUID) error
	
	// Phone number lookup operations (performance critical)
	
	// FindByPhone retrieves DNC entries for a specific phone number
	// Expected latency: < 5ms with proper B-tree indexing
	FindByPhone(ctx context.Context, phoneNumber values.PhoneNumber) ([]*DNCEntry, error)
	
	// FindActiveByPhone retrieves only active (non-expired) DNC entries for a phone number
	// This is the most frequently called method - must be optimized
	FindActiveByPhone(ctx context.Context, phoneNumber values.PhoneNumber) ([]*DNCEntry, error)
	
	// CheckPhoneExists performs a fast existence check without retrieving full records
	// Used for bloom filter miss validation - must be sub-millisecond
	CheckPhoneExists(ctx context.Context, phoneNumber values.PhoneNumber) (bool, error)
	
	// Provider and source operations
	
	// FindByProvider retrieves all DNC entries from a specific provider
	FindByProvider(ctx context.Context, providerID uuid.UUID) ([]*DNCEntry, error)
	
	// FindBySource retrieves DNC entries by list source type
	FindBySource(ctx context.Context, source ListSource) ([]*DNCEntry, error)
	
	// FindBySourceAndProvider retrieves entries by both source and provider
	FindBySourceAndProvider(ctx context.Context, source ListSource, providerID uuid.UUID) ([]*DNCEntry, error)
	
	// Batch operations for sync performance
	
	// BulkInsert inserts multiple DNC entries in a single transaction
	// Expected throughput: > 10K entries/second
	// All entries succeed or all fail for data consistency
	BulkInsert(ctx context.Context, entries []*DNCEntry) error
	
	// BulkInsertWithTx performs bulk insert within an existing transaction
	BulkInsertWithTx(ctx context.Context, tx Transaction, entries []*DNCEntry) error
	
	// BulkUpdate updates multiple DNC entries efficiently
	BulkUpdate(ctx context.Context, entries []*DNCEntry) error
	
	// BulkDelete removes multiple entries (soft delete)
	BulkDelete(ctx context.Context, entryIDs []uuid.UUID, deletedBy uuid.UUID) error
	
	// Upsert creates or updates entries based on phone number uniqueness
	// Used for incremental provider syncs
	Upsert(ctx context.Context, entries []*DNCEntry) error
	
	// Query and filtering operations
	
	// Find searches for DNC entries based on filter criteria with pagination
	Find(ctx context.Context, filter DNCEntryFilter) (*DNCEntryPage, error)
	
	// Count returns the total number of entries matching the filter
	Count(ctx context.Context, filter DNCEntryFilter) (int64, error)
	
	// FindExpired retrieves entries that have expired before the given time
	FindExpired(ctx context.Context, before time.Time, limit int) ([]*DNCEntry, error)
	
	// FindExpiring retrieves entries expiring within the specified duration
	FindExpiring(ctx context.Context, within time.Duration, limit int) ([]*DNCEntry, error)
	
	// FindModifiedSince retrieves entries modified since a specific time
	// Used for incremental sync and cache invalidation
	FindModifiedSince(ctx context.Context, since time.Time) ([]*DNCEntry, error)
	
	// Maintenance operations
	
	// CleanupExpired removes expired entries older than the retention period
	// Returns the number of entries cleaned up
	CleanupExpired(ctx context.Context, retentionDays int) (int64, error)
	
	// Vacuum performs database maintenance operations (VACUUM, REINDEX)
	Vacuum(ctx context.Context) error
	
	// GetStats returns repository performance and usage statistics
	GetStats(ctx context.Context) (*DNCEntryStats, error)
	
	// Sync and consistency operations
	
	// GetSyncChecksum calculates a checksum for entries from a specific provider
	// Used to verify data consistency during sync operations
	GetSyncChecksum(ctx context.Context, providerID uuid.UUID) (string, error)
	
	// GetLastSyncTime returns the timestamp of the most recent entry from a provider
	GetLastSyncTime(ctx context.Context, providerID uuid.UUID) (*time.Time, error)
	
	// ValidateIntegrity performs integrity checks on the repository
	ValidateIntegrity(ctx context.Context) (*DNCIntegrityReport, error)
	
	// Transaction support
	
	// BeginTx starts a new database transaction
	BeginTx(ctx context.Context) (Transaction, error)
	
	// WithTx executes a function within a database transaction
	WithTx(ctx context.Context, fn func(tx Transaction) error) error
}

// DNCEntryFilter defines filtering options for DNC entry queries
type DNCEntryFilter struct {
	// Phone number filters
	PhoneNumbers    []values.PhoneNumber `json:"phone_numbers,omitempty"`
	PhonePattern    *string              `json:"phone_pattern,omitempty"` // SQL LIKE pattern
	
	// Source and provider filters
	Sources       []ListSource `json:"sources,omitempty"`
	ProviderIDs   []uuid.UUID  `json:"provider_ids,omitempty"`
	ProviderNames []string     `json:"provider_names,omitempty"`
	
	// Reason filters
	SuppressReasons []SuppressReason `json:"suppress_reasons,omitempty"`
	
	// Time range filters
	AddedAfter    *time.Time `json:"added_after,omitempty"`
	AddedBefore   *time.Time `json:"added_before,omitempty"`
	ExpiresAfter  *time.Time `json:"expires_after,omitempty"`
	ExpiresBefore *time.Time `json:"expires_before,omitempty"`
	
	// Status filters
	OnlyActive   *bool `json:"only_active,omitempty"`   // Only non-expired entries
	OnlyExpired  *bool `json:"only_expired,omitempty"`  // Only expired entries
	HasExpiry    *bool `json:"has_expiry,omitempty"`    // Only entries with/without expiration
	
	// User filters
	AddedBy   []uuid.UUID `json:"added_by,omitempty"`
	UpdatedBy []uuid.UUID `json:"updated_by,omitempty"`
	
	// Search
	SearchText *string `json:"search_text,omitempty"` // Search in notes and metadata
	
	// Metadata filters
	MetadataKeys   []string          `json:"metadata_keys,omitempty"`
	MetadataValues map[string]string `json:"metadata_values,omitempty"`
	
	// Pagination
	Limit  int    `json:"limit,omitempty"`
	Offset int    `json:"offset,omitempty"`
	Cursor string `json:"cursor,omitempty"` // For cursor-based pagination
	
	// Sorting
	OrderBy   string `json:"order_by,omitempty"`   // Field to sort by
	OrderDesc bool   `json:"order_desc,omitempty"` // Sort direction
	
	// Performance options
	IncludeMetadata bool `json:"include_metadata,omitempty"` // Include metadata in results
	CountOnly       bool `json:"count_only,omitempty"`       // Return count only for performance
}

// DNCEntryPage represents a paginated result set of DNC entries
type DNCEntryPage struct {
	Entries    []*DNCEntry `json:"entries"`
	TotalCount int64       `json:"total_count"`
	HasMore    bool        `json:"has_more"`
	NextCursor string      `json:"next_cursor,omitempty"`
	
	// Performance metadata
	QueryTime    time.Duration `json:"query_time"`
	DatabaseHits int           `json:"database_hits"`
	CacheHits    int           `json:"cache_hits"`
}

// DNCEntryStats provides performance and usage statistics
type DNCEntryStats struct {
	// Entry counts by source
	TotalEntries       int64            `json:"total_entries"`
	EntriesBySource    map[string]int64 `json:"entries_by_source"`
	EntriesByProvider  map[string]int64 `json:"entries_by_provider"`
	ActiveEntries      int64            `json:"active_entries"`
	ExpiredEntries     int64            `json:"expired_entries"`
	
	// Performance metrics
	AverageQueryTime   time.Duration `json:"average_query_time"`
	QueryThroughput    float64       `json:"query_throughput"` // Queries per second
	InsertThroughput   float64       `json:"insert_throughput"` // Inserts per second
	
	// Growth metrics
	EntriesThisMonth   int64 `json:"entries_this_month"`
	EntriesThisWeek    int64 `json:"entries_this_week"`
	EntriesThisDay     int64 `json:"entries_this_day"`
	
	// Cache statistics
	CacheHitRate      float64 `json:"cache_hit_rate"`
	CacheSize         int64   `json:"cache_size"`
	CacheEvictions    int64   `json:"cache_evictions"`
	
	// Index statistics
	IndexSize         int64   `json:"index_size_bytes"`
	IndexEfficiency   float64 `json:"index_efficiency"`
	
	// Data quality
	DuplicatePhones   int64 `json:"duplicate_phones"`
	InvalidFormats    int64 `json:"invalid_formats"`
	MissingMetadata   int64 `json:"missing_metadata"`
	
	// Collection timestamp
	CollectedAt       time.Time `json:"collected_at"`
}

// DNCIntegrityReport provides comprehensive integrity analysis
type DNCIntegrityReport struct {
	// Report metadata
	GeneratedAt time.Time `json:"generated_at"`
	
	// Overall status
	IsHealthy     bool   `json:"is_healthy"`
	OverallStatus string `json:"overall_status"` // HEALTHY, DEGRADED, CRITICAL
	
	// Data integrity
	TotalEntries      int64 `json:"total_entries"`
	ValidEntries      int64 `json:"valid_entries"`
	InvalidEntries    int64 `json:"invalid_entries"`
	DuplicateEntries  int64 `json:"duplicate_entries"`
	OrphanedEntries   int64 `json:"orphaned_entries"` // Entries without valid provider
	
	// Consistency checks
	IndexConsistency  bool              `json:"index_consistency"`
	ReferentialIntegrity bool           `json:"referential_integrity"`
	DataTypeConsistency bool            `json:"data_type_consistency"`
	
	// Performance metrics
	VerificationTime time.Duration      `json:"verification_time"`
	DatabaseQueries  int                `json:"database_queries"`
	
	// Issues found
	CriticalIssues []string             `json:"critical_issues,omitempty"`
	Warnings       []string             `json:"warnings,omitempty"`
	Recommendations []string            `json:"recommendations,omitempty"`
	
	// Detailed breakdown
	IssuesBySource map[string][]string  `json:"issues_by_source,omitempty"`
	IssuesByProvider map[string][]string `json:"issues_by_provider,omitempty"`
}

// Transaction represents a database transaction interface
// This abstraction allows for different transaction implementations
type Transaction interface {
	// Commit commits the transaction
	Commit() error
	
	// Rollback rolls back the transaction
	Rollback() error
	
	// Context returns the transaction context
	Context() context.Context
}

// ListSource represents the source of a DNC list (federal, state, internal, etc.)
// This is a placeholder type - the actual implementation should use the values package
type ListSource string

const (
	ListSourceFederal  ListSource = "federal"
	ListSourceState    ListSource = "state" 
	ListSourceInternal ListSource = "internal"
	ListSourceConsumer ListSource = "consumer"
	ListSourcePartner  ListSource = "partner"
)

// SuppressReason represents why a number is on the DNC list
// This is a placeholder type - the actual implementation should use the values package
type SuppressReason string

const (
	SuppressReasonConsumerRequest    SuppressReason = "consumer_request"
	SuppressReasonRegulatory         SuppressReason = "regulatory"
	SuppressReasonComplaint          SuppressReason = "complaint"
	SuppressReasonInternalPolicy     SuppressReason = "internal_policy"
	SuppressReasonPartnerRequest     SuppressReason = "partner_request"
	SuppressReasonFraudPrevention    SuppressReason = "fraud_prevention"
	SuppressReasonDataQualityIssue   SuppressReason = "data_quality_issue"
)