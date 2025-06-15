package archive

import (
	"context"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/audit"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/google/uuid"
)

// ArchiverRepository defines the interface for archiving audit events
// Following DCE patterns: context support, domain-specific errors, performance optimization
type ArchiverRepository interface {
	// Archive operations
	
	// ArchiveEvents archives events older than the specified date
	// Returns the number of events archived and any error
	ArchiveEvents(ctx context.Context, olderThan time.Time, batchSize int) (int64, error)
	
	// ArchiveBatch archives a specific batch of events
	// Events are compressed into Parquet format and stored in S3
	ArchiveBatch(ctx context.Context, events []*audit.Event) (*ArchiveResult, error)
	
	// Query operations
	
	// QueryArchive queries archived events based on criteria
	// Supports compliance queries on archived data
	QueryArchive(ctx context.Context, query ArchiveQuery) (*ArchiveQueryResult, error)
	
	// GetArchivedEvent retrieves a specific archived event by ID
	GetArchivedEvent(ctx context.Context, eventID uuid.UUID) (*audit.Event, error)
	
	// GetArchivedEventBySequence retrieves an archived event by sequence number
	GetArchivedEventBySequence(ctx context.Context, seq values.SequenceNumber) (*audit.Event, error)
	
	// Verification operations
	
	// VerifyArchiveIntegrity verifies the integrity of archived data
	VerifyArchiveIntegrity(ctx context.Context, archiveID string) (*ArchiveIntegrityResult, error)
	
	// GetArchiveManifest retrieves metadata about an archive file
	GetArchiveManifest(ctx context.Context, archiveID string) (*ArchiveManifest, error)
	
	// Management operations
	
	// ListArchives lists all archive files within a time range
	ListArchives(ctx context.Context, startTime, endTime time.Time) ([]*ArchiveInfo, error)
	
	// GetArchiveStats returns statistics about the archive storage
	GetArchiveStats(ctx context.Context) (*ArchiveStats, error)
	
	// DeleteExpiredArchives removes archives past 7-year retention
	DeleteExpiredArchives(ctx context.Context) (int64, error)
	
	// RestoreArchive restores archived events back to main storage
	// Used for compliance audits or investigations
	RestoreArchive(ctx context.Context, archiveID string) (*RestoreResult, error)
}

// ArchiveResult represents the result of an archiving operation
type ArchiveResult struct {
	ArchiveID        string    `json:"archive_id"`
	EventCount       int64     `json:"event_count"`
	StartSequence    values.SequenceNumber `json:"start_sequence"`
	EndSequence      values.SequenceNumber `json:"end_sequence"`
	StartTime        time.Time `json:"start_time"`
	EndTime          time.Time `json:"end_time"`
	CompressedSize   int64     `json:"compressed_size"`
	UncompressedSize int64     `json:"uncompressed_size"`
	CompressionRatio float64   `json:"compression_ratio"`
	S3Location       string    `json:"s3_location"`
	HashChainValid   bool      `json:"hash_chain_valid"`
	CreatedAt        time.Time `json:"created_at"`
	ExpiresAt        time.Time `json:"expires_at"`
}

// ArchiveQuery defines criteria for querying archived events
type ArchiveQuery struct {
	// Time range
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	
	// Event filters (similar to audit.EventFilter)
	EventTypes      []audit.EventType `json:"event_types,omitempty"`
	ActorIDs        []string          `json:"actor_ids,omitempty"`
	TargetIDs       []string          `json:"target_ids,omitempty"`
	ComplianceFlags []string          `json:"compliance_flags,omitempty"`
	
	// Sequence range
	SequenceStart *values.SequenceNumber `json:"sequence_start,omitempty"`
	SequenceEnd   *values.SequenceNumber `json:"sequence_end,omitempty"`
	
	// Pagination
	Limit  int    `json:"limit,omitempty"`
	Offset int    `json:"offset,omitempty"`
	
	// Performance options
	IncludeMetadata bool `json:"include_metadata,omitempty"`
}

// ArchiveQueryResult represents the result of an archive query
type ArchiveQueryResult struct {
	Events         []*audit.Event `json:"events"`
	TotalCount     int64          `json:"total_count"`
	ArchivesQueried int            `json:"archives_queried"`
	QueryTime      time.Duration  `json:"query_time"`
	HasMore        bool           `json:"has_more"`
}

// ArchiveIntegrityResult represents the result of archive integrity verification
type ArchiveIntegrityResult struct {
	ArchiveID      string    `json:"archive_id"`
	IsValid        bool      `json:"is_valid"`
	EventCount     int64     `json:"event_count"`
	HashChainValid bool      `json:"hash_chain_valid"`
	MetadataValid  bool      `json:"metadata_valid"`
	ParquetValid   bool      `json:"parquet_valid"`
	VerifiedAt     time.Time `json:"verified_at"`
	Errors         []string  `json:"errors,omitempty"`
}

// ArchiveManifest contains metadata about an archive file
type ArchiveManifest struct {
	ArchiveID        string                `json:"archive_id"`
	Version          string                `json:"version"`
	CreatedAt        time.Time             `json:"created_at"`
	EventCount       int64                 `json:"event_count"`
	StartSequence    values.SequenceNumber `json:"start_sequence"`
	EndSequence      values.SequenceNumber `json:"end_sequence"`
	StartTime        time.Time             `json:"start_time"`
	EndTime          time.Time             `json:"end_time"`
	CompressedSize   int64                 `json:"compressed_size"`
	UncompressedSize int64                 `json:"uncompressed_size"`
	CompressionType  string                `json:"compression_type"`
	Schema           ParquetSchema         `json:"schema"`
	ComplianceFlags  map[string]int64      `json:"compliance_flags"` // Flag -> count
	HashChainInfo    HashChainInfo         `json:"hash_chain_info"`
	RetentionPolicy  RetentionPolicy       `json:"retention_policy"`
}

// ParquetSchema defines the schema for the Parquet file
type ParquetSchema struct {
	Version      string              `json:"version"`
	Fields       []ParquetField      `json:"fields"`
	Compression  string              `json:"compression"`
	RowGroupSize int                 `json:"row_group_size"`
}

// ParquetField defines a field in the Parquet schema
type ParquetField struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Required   bool   `json:"required"`
	Repeated   bool   `json:"repeated"`
	LogicalType string `json:"logical_type,omitempty"`
}

// HashChainInfo contains information about the hash chain in the archive
type HashChainInfo struct {
	FirstHash    string `json:"first_hash"`
	LastHash     string `json:"last_hash"`
	ChainValid   bool   `json:"chain_valid"`
	Algorithm    string `json:"algorithm"`
}

// RetentionPolicy defines the retention policy for the archive
type RetentionPolicy struct {
	RetentionDays int       `json:"retention_days"`
	ExpiresAt     time.Time `json:"expires_at"`
	LegalHold     bool      `json:"legal_hold"`
	ComplianceType string    `json:"compliance_type"` // GDPR, TCPA, etc.
}

// ArchiveInfo provides summary information about an archive
type ArchiveInfo struct {
	ArchiveID      string    `json:"archive_id"`
	S3Key          string    `json:"s3_key"`
	EventCount     int64     `json:"event_count"`
	StartTime      time.Time `json:"start_time"`
	EndTime        time.Time `json:"end_time"`
	Size           int64     `json:"size"`
	CreatedAt      time.Time `json:"created_at"`
	ExpiresAt      time.Time `json:"expires_at"`
	Status         string    `json:"status"` // ACTIVE, EXPIRED, DELETED
}

// ArchiveStats provides statistics about archive storage
type ArchiveStats struct {
	TotalArchives     int64         `json:"total_archives"`
	TotalEvents       int64         `json:"total_events"`
	TotalSize         int64         `json:"total_size"`
	OldestArchive     time.Time     `json:"oldest_archive"`
	NewestArchive     time.Time     `json:"newest_archive"`
	AverageSize       int64         `json:"average_size"`
	CompressionRatio  float64       `json:"compression_ratio"`
	ArchivesByYear    map[int]int64 `json:"archives_by_year"`
	EventsByCompliance map[string]int64 `json:"events_by_compliance"`
	CollectedAt       time.Time     `json:"collected_at"`
}

// RestoreResult represents the result of restoring archived events
type RestoreResult struct {
	ArchiveID       string        `json:"archive_id"`
	EventsRestored  int64         `json:"events_restored"`
	StartSequence   values.SequenceNumber `json:"start_sequence"`
	EndSequence     values.SequenceNumber `json:"end_sequence"`
	RestoreTime     time.Duration `json:"restore_time"`
	VerificationStatus string     `json:"verification_status"`
	Errors          []string      `json:"errors,omitempty"`
}

// ArchiveConfig defines configuration for the archiver
type ArchiveConfig struct {
	// S3 configuration
	BucketName      string `json:"bucket_name"`
	Region          string `json:"region"`
	Endpoint        string `json:"endpoint,omitempty"` // For testing with MinIO
	
	// Archive settings
	BatchSize       int           `json:"batch_size"`
	CompressionType string        `json:"compression_type"` // snappy, gzip, zstd
	RowGroupSize    int           `json:"row_group_size"`
	RetentionDays   int           `json:"retention_days"`
	
	// Performance settings
	MaxConcurrency  int           `json:"max_concurrency"`
	UploadPartSize  int64         `json:"upload_part_size"`
	Timeout         time.Duration `json:"timeout"`
	
	// Lifecycle policies
	EnableLifecycle bool          `json:"enable_lifecycle"`
	TransitionDays  int           `json:"transition_days"` // Days before moving to glacier
	
	// Security
	EnableEncryption bool   `json:"enable_encryption"`
	KMSKeyID         string `json:"kms_key_id,omitempty"`
}