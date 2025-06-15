package archive

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/infrastructure/config"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/infrastructure/database"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/infrastructure/telemetry"
)

// Factory creates archive repository instances
type Factory struct {
	db        *pgxpool.Pool
	auditRepo *database.AuditRepository
	config    *config.Config
	logger    telemetry.Logger
}

// NewFactory creates a new archive repository factory
func NewFactory(db *pgxpool.Pool, auditRepo *database.AuditRepository, cfg *config.Config, logger telemetry.Logger) *Factory {
	return &Factory{
		db:        db,
		auditRepo: auditRepo,
		config:    cfg,
		logger:    logger,
	}
}

// CreateArchiver creates an archiver based on configuration
func (f *Factory) CreateArchiver(ctx context.Context) (ArchiverRepository, error) {
	// Get archive configuration from main config
	archiveConfig := f.getArchiveConfig()

	// Validate configuration
	if err := f.validateConfig(archiveConfig); err != nil {
		return nil, errors.NewValidationError("INVALID_CONFIG", 
			"archive configuration validation failed").WithCause(err)
	}

	// Currently only S3 is supported
	switch archiveConfig.Provider {
	case "s3", "aws":
		return NewS3Archiver(f.db, f.auditRepo, archiveConfig, f.logger)
	case "gcs":
		return nil, errors.NewNotImplementedError("GCS_NOT_IMPLEMENTED", 
			"Google Cloud Storage archiver not yet implemented")
	case "azure":
		return nil, errors.NewNotImplementedError("AZURE_NOT_IMPLEMENTED", 
			"Azure Blob Storage archiver not yet implemented")
	default:
		return nil, errors.NewValidationError("UNKNOWN_PROVIDER", 
			fmt.Sprintf("unknown archive provider: %s", archiveConfig.Provider))
	}
}

// getArchiveConfig extracts archive configuration from main config
func (f *Factory) getArchiveConfig() ArchiveConfig {
	// Default configuration
	cfg := ArchiveConfig{
		Provider:        "s3",
		BucketName:      fmt.Sprintf("dce-audit-archive-%s", f.config.Environment),
		Region:          "us-east-1",
		BatchSize:       1000,
		CompressionType: "snappy",
		RowGroupSize:    100000,
		RetentionDays:   2555, // 7 years
		MaxConcurrency:  10,
		UploadPartSize:  5 * 1024 * 1024, // 5MB
		Timeout:         5 * time.Minute,
		EnableLifecycle: true,
		TransitionDays:  90, // Move to Glacier after 90 days
		EnableEncryption: true,
	}

	// Override with config values if available
	if f.config != nil {
		// Archive section
		if archiveSection := f.config.Sub("archive"); archiveSection != nil {
			if provider := archiveSection.String("provider"); provider != "" {
				cfg.Provider = provider
			}
			if bucket := archiveSection.String("bucket"); bucket != "" {
				cfg.BucketName = bucket
			}
			if region := archiveSection.String("region"); region != "" {
				cfg.Region = region
			}
			if endpoint := archiveSection.String("endpoint"); endpoint != "" {
				cfg.Endpoint = endpoint // For testing with MinIO
			}
			if batchSize := archiveSection.Int("batch_size"); batchSize > 0 {
				cfg.BatchSize = batchSize
			}
			if compression := archiveSection.String("compression"); compression != "" {
				cfg.CompressionType = compression
			}
			if rowGroupSize := archiveSection.Int("row_group_size"); rowGroupSize > 0 {
				cfg.RowGroupSize = rowGroupSize
			}
			if retentionDays := archiveSection.Int("retention_days"); retentionDays > 0 {
				cfg.RetentionDays = retentionDays
			}
			if concurrency := archiveSection.Int("max_concurrency"); concurrency > 0 {
				cfg.MaxConcurrency = concurrency
			}
			if partSize := archiveSection.Int64("upload_part_size"); partSize > 0 {
				cfg.UploadPartSize = partSize
			}
			if timeout := archiveSection.Duration("timeout"); timeout > 0 {
				cfg.Timeout = timeout
			}
			cfg.EnableLifecycle = archiveSection.Bool("enable_lifecycle")
			if transitionDays := archiveSection.Int("transition_days"); transitionDays > 0 {
				cfg.TransitionDays = transitionDays
			}
			cfg.EnableEncryption = archiveSection.Bool("enable_encryption")
			if kmsKey := archiveSection.String("kms_key_id"); kmsKey != "" {
				cfg.KMSKeyID = kmsKey
			}
		}

		// AWS section for S3-specific settings
		if awsSection := f.config.Sub("aws"); awsSection != nil {
			if region := awsSection.String("region"); region != "" && cfg.Region == "us-east-1" {
				cfg.Region = region
			}
		}
	}

	return cfg
}

// validateConfig validates the archive configuration
func (f *Factory) validateConfig(cfg ArchiveConfig) error {
	if cfg.BucketName == "" {
		return fmt.Errorf("bucket name is required")
	}

	if cfg.Region == "" {
		return fmt.Errorf("region is required")
	}

	if cfg.BatchSize <= 0 {
		return fmt.Errorf("batch size must be positive")
	}

	if cfg.RetentionDays <= 0 {
		return fmt.Errorf("retention days must be positive")
	}

	// Validate compression type
	validCompressions := map[string]bool{
		"snappy": true,
		"gzip":   true,
		"zstd":   true,
		"none":   true,
	}
	if !validCompressions[cfg.CompressionType] {
		return fmt.Errorf("invalid compression type: %s", cfg.CompressionType)
	}

	// Validate lifecycle settings
	if cfg.EnableLifecycle && cfg.TransitionDays >= cfg.RetentionDays {
		return fmt.Errorf("transition days must be less than retention days")
	}

	return nil
}

// ArchiveConfig with provider field
type ArchiveConfig struct {
	// Provider (s3, gcs, azure)
	Provider string `json:"provider"`
	
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