package archive

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/xitongsys/parquet-go/parquet"
	"github.com/xitongsys/parquet-go/writer"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/audit"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/infrastructure/database"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/infrastructure/telemetry"
)

// S3Archiver implements the ArchiverRepository interface using AWS S3
// Following DCE patterns: S3 for long-term storage, Parquet for compression, metadata preservation
type S3Archiver struct {
	db          *pgxpool.Pool
	auditRepo   *database.AuditRepository
	s3Client    *s3.Client
	uploader    *manager.Uploader
	downloader  *manager.Downloader
	config      ArchiveConfig
	logger      telemetry.Logger
	mu          sync.RWMutex
}

// NewS3Archiver creates a new S3-based archiver
func NewS3Archiver(db *pgxpool.Pool, auditRepo *database.AuditRepository, cfg ArchiveConfig, logger telemetry.Logger) (*S3Archiver, error) {
	// Load AWS configuration
	awsCfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(cfg.Region),
	)
	if err != nil {
		return nil, errors.NewInternalError("failed to load AWS config").WithCause(err)
	}

	// Create S3 client
	s3Client := s3.NewFromConfig(awsCfg)
	if cfg.Endpoint != "" {
		// For testing with MinIO or LocalStack
		s3Client = s3.NewFromConfig(awsCfg, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
			o.UsePathStyle = true
		})
	}

	// Create uploader and downloader
	uploader := manager.NewUploader(s3Client, func(u *manager.Uploader) {
		u.PartSize = cfg.UploadPartSize
		u.Concurrency = cfg.MaxConcurrency
	})

	downloader := manager.NewDownloader(s3Client, func(d *manager.Downloader) {
		d.PartSize = cfg.UploadPartSize
		d.Concurrency = cfg.MaxConcurrency
	})

	archiver := &S3Archiver{
		db:         db,
		auditRepo:  auditRepo,
		s3Client:   s3Client,
		uploader:   uploader,
		downloader: downloader,
		config:     cfg,
		logger:     logger,
	}

	// Initialize bucket if needed
	if err := archiver.ensureBucketExists(context.Background()); err != nil {
		return nil, err
	}

	// Set up lifecycle policies if enabled
	if cfg.EnableLifecycle {
		if err := archiver.setupLifecyclePolicies(context.Background()); err != nil {
			logger.Warn(context.Background(), "Failed to setup lifecycle policies", 
				"error", err.Error())
		}
	}

	return archiver, nil
}

// ArchiveEvents archives events older than the specified date
func (a *S3Archiver) ArchiveEvents(ctx context.Context, olderThan time.Time, batchSize int) (int64, error) {
	a.logger.Info(ctx, "Starting event archival", 
		"older_than", olderThan.Format(time.RFC3339),
		"batch_size", batchSize)

	var totalArchived int64
	
	// Process in batches to avoid memory issues
	for {
		// Get batch of events to archive
		events, err := a.auditRepo.GetExpiredEvents(ctx, olderThan, batchSize)
		if err != nil {
			return totalArchived, errors.NewInternalError("failed to get expired events").WithCause(err)
		}

		if len(events) == 0 {
			break // No more events to archive
		}

		// Archive the batch
		result, err := a.ArchiveBatch(ctx, events)
		if err != nil {
			a.logger.Error(ctx, "Failed to archive batch", 
				"error", err.Error(),
				"batch_size", len(events))
			return totalArchived, err
		}

		// Mark events as archived in the database
		if err := a.markEventsAsArchived(ctx, events, result.ArchiveID); err != nil {
			a.logger.Error(ctx, "Failed to mark events as archived", 
				"error", err.Error(),
				"archive_id", result.ArchiveID)
			// Don't fail the whole operation, but log the error
		}

		totalArchived += result.EventCount
		
		a.logger.Info(ctx, "Archived batch", 
			"archive_id", result.ArchiveID,
			"event_count", result.EventCount,
			"total_archived", totalArchived)

		// If we got less than batch size, we're done
		if len(events) < batchSize {
			break
		}
	}

	a.logger.Info(ctx, "Completed event archival", 
		"total_archived", totalArchived)

	return totalArchived, nil
}

// ArchiveBatch archives a specific batch of events
func (a *S3Archiver) ArchiveBatch(ctx context.Context, events []*audit.Event) (*ArchiveResult, error) {
	if len(events) == 0 {
		return nil, errors.NewValidationError("EMPTY_BATCH", "no events to archive")
	}

	// Generate archive ID and S3 key
	archiveID := a.generateArchiveID()
	s3Key := a.generateS3Key(events[0].Timestamp, archiveID)

	// Create Parquet buffer
	buf := new(bytes.Buffer)
	
	// Write events to Parquet format
	parquetSize, err := a.writeParquetFile(buf, events)
	if err != nil {
		return nil, errors.NewInternalError("failed to write parquet file").WithCause(err)
	}

	// Calculate compression ratio
	uncompressedSize := a.calculateUncompressedSize(events)
	compressionRatio := float64(uncompressedSize) / float64(parquetSize)

	// Create manifest
	manifest := a.createManifest(archiveID, events, parquetSize, uncompressedSize)
	
	// Upload to S3
	if err := a.uploadToS3(ctx, s3Key, buf.Bytes(), manifest); err != nil {
		return nil, errors.NewInternalError("failed to upload to S3").WithCause(err)
	}

	// Create result
	result := &ArchiveResult{
		ArchiveID:        archiveID,
		EventCount:       int64(len(events)),
		StartSequence:    values.MustNewSequenceNumber(uint64(events[0].SequenceNum)),
		EndSequence:      values.MustNewSequenceNumber(uint64(events[len(events)-1].SequenceNum)),
		StartTime:        events[0].Timestamp,
		EndTime:          events[len(events)-1].Timestamp,
		CompressedSize:   parquetSize,
		UncompressedSize: uncompressedSize,
		CompressionRatio: compressionRatio,
		S3Location:       fmt.Sprintf("s3://%s/%s", a.config.BucketName, s3Key),
		HashChainValid:   a.verifyHashChain(events),
		CreatedAt:        time.Now().UTC(),
		ExpiresAt:        time.Now().UTC().AddDate(0, 0, a.config.RetentionDays),
	}

	return result, nil
}

// QueryArchive queries archived events based on criteria
func (a *S3Archiver) QueryArchive(ctx context.Context, query ArchiveQuery) (*ArchiveQueryResult, error) {
	// List relevant archive files based on time range
	archives, err := a.ListArchives(ctx, query.StartTime, query.EndTime)
	if err != nil {
		return nil, err
	}

	result := &ArchiveQueryResult{
		Events:          make([]*audit.Event, 0),
		ArchivesQueried: len(archives),
	}

	startTime := time.Now()

	// Query each relevant archive
	for _, archive := range archives {
		// Download and query the archive
		events, err := a.queryArchiveFile(ctx, archive.S3Key, query)
		if err != nil {
			a.logger.Warn(ctx, "Failed to query archive file", 
				"archive_id", archive.ArchiveID,
				"error", err.Error())
			continue
		}

		result.Events = append(result.Events, events...)
		
		// Check if we have enough results
		if query.Limit > 0 && len(result.Events) >= query.Limit {
			result.Events = result.Events[:query.Limit]
			result.HasMore = true
			break
		}
	}

	result.QueryTime = time.Since(startTime)
	result.TotalCount = int64(len(result.Events))

	return result, nil
}

// GetArchivedEvent retrieves a specific archived event by ID
func (a *S3Archiver) GetArchivedEvent(ctx context.Context, eventID uuid.UUID) (*audit.Event, error) {
	// First, check if we have an index or metadata about which archive contains this event
	archiveID, err := a.findArchiveForEvent(ctx, eventID)
	if err != nil {
		return nil, err
	}

	// Download and search the specific archive
	archive, err := a.downloadArchive(ctx, archiveID)
	if err != nil {
		return nil, err
	}

	// Find the event in the archive
	for _, event := range archive {
		if event.ID == eventID {
			return event, nil
		}
	}

	return nil, errors.NewNotFoundError("EVENT_NOT_FOUND", 
		fmt.Sprintf("event %s not found in archive", eventID))
}

// GetArchivedEventBySequence retrieves an archived event by sequence number
func (a *S3Archiver) GetArchivedEventBySequence(ctx context.Context, seq values.SequenceNumber) (*audit.Event, error) {
	// Find archive containing this sequence number
	archiveID, err := a.findArchiveForSequence(ctx, seq)
	if err != nil {
		return nil, err
	}

	// Download and search the specific archive
	archive, err := a.downloadArchive(ctx, archiveID)
	if err != nil {
		return nil, err
	}

	// Find the event in the archive
	for _, event := range archive {
		if event.SequenceNum == int64(seq.Value()) {
			return event, nil
		}
	}

	return nil, errors.NewNotFoundError("EVENT_NOT_FOUND", 
		fmt.Sprintf("event with sequence %s not found in archive", seq))
}

// VerifyArchiveIntegrity verifies the integrity of archived data
func (a *S3Archiver) VerifyArchiveIntegrity(ctx context.Context, archiveID string) (*ArchiveIntegrityResult, error) {
	manifest, err := a.GetArchiveManifest(ctx, archiveID)
	if err != nil {
		return nil, err
	}

	result := &ArchiveIntegrityResult{
		ArchiveID:     archiveID,
		IsValid:       true,
		EventCount:    manifest.EventCount,
		MetadataValid: true,
		VerifiedAt:    time.Now().UTC(),
		Errors:        make([]string, 0),
	}

	// Download the archive
	events, err := a.downloadArchive(ctx, archiveID)
	if err != nil {
		result.IsValid = false
		result.ParquetValid = false
		result.Errors = append(result.Errors, fmt.Sprintf("Failed to download archive: %v", err))
		return result, nil
	}

	// Verify event count
	if int64(len(events)) != manifest.EventCount {
		result.IsValid = false
		result.Errors = append(result.Errors, 
			fmt.Sprintf("Event count mismatch: expected %d, got %d", 
				manifest.EventCount, len(events)))
	}

	// Verify hash chain
	result.HashChainValid = a.verifyHashChain(events)
	if !result.HashChainValid {
		result.IsValid = false
		result.Errors = append(result.Errors, "Hash chain verification failed")
	}

	// Verify sequence range
	if len(events) > 0 {
		firstSeq := values.MustNewSequenceNumber(uint64(events[0].SequenceNum))
		lastSeq := values.MustNewSequenceNumber(uint64(events[len(events)-1].SequenceNum))
		
		if !firstSeq.Equal(manifest.StartSequence) || !lastSeq.Equal(manifest.EndSequence) {
			result.IsValid = false
			result.Errors = append(result.Errors, "Sequence range mismatch")
		}
	}

	result.ParquetValid = len(result.Errors) == 0

	return result, nil
}

// GetArchiveManifest retrieves metadata about an archive file
func (a *S3Archiver) GetArchiveManifest(ctx context.Context, archiveID string) (*ArchiveManifest, error) {
	// Construct manifest key
	manifestKey := fmt.Sprintf("%s.manifest.json", archiveID)
	
	// Download manifest from S3
	output, err := a.s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(a.config.BucketName),
		Key:    aws.String(manifestKey),
	})
	if err != nil {
		return nil, errors.NewInternalError("failed to get manifest").WithCause(err)
	}
	defer output.Body.Close()

	// Parse manifest
	var manifest ArchiveManifest
	if err := json.NewDecoder(output.Body).Decode(&manifest); err != nil {
		return nil, errors.NewInternalError("failed to parse manifest").WithCause(err)
	}

	return &manifest, nil
}

// ListArchives lists all archive files within a time range
func (a *S3Archiver) ListArchives(ctx context.Context, startTime, endTime time.Time) ([]*ArchiveInfo, error) {
	archives := make([]*ArchiveInfo, 0)

	// Generate prefix based on time range
	prefixes := a.generatePrefixesForTimeRange(startTime, endTime)

	for _, prefix := range prefixes {
		paginator := s3.NewListObjectsV2Paginator(a.s3Client, &s3.ListObjectsV2Input{
			Bucket: aws.String(a.config.BucketName),
			Prefix: aws.String(prefix),
		})

		for paginator.HasMorePages() {
			output, err := paginator.NextPage(ctx)
			if err != nil {
				return nil, errors.NewInternalError("failed to list objects").WithCause(err)
			}

			for _, obj := range output.Contents {
				// Skip manifest files
				if strings.HasSuffix(*obj.Key, ".manifest.json") {
					continue
				}

				// Parse archive info from key
				info, err := a.parseArchiveInfo(*obj.Key, obj)
				if err != nil {
					a.logger.Warn(ctx, "Failed to parse archive info", 
						"key", *obj.Key,
						"error", err.Error())
					continue
				}

				// Check if within time range
				if info.StartTime.After(endTime) || info.EndTime.Before(startTime) {
					continue
				}

				archives = append(archives, info)
			}
		}
	}

	return archives, nil
}

// GetArchiveStats returns statistics about the archive storage
func (a *S3Archiver) GetArchiveStats(ctx context.Context) (*ArchiveStats, error) {
	stats := &ArchiveStats{
		ArchivesByYear:     make(map[int]int64),
		EventsByCompliance: make(map[string]int64),
		CollectedAt:        time.Now().UTC(),
	}

	// List all archives
	paginator := s3.NewListObjectsV2Paginator(a.s3Client, &s3.ListObjectsV2Input{
		Bucket: aws.String(a.config.BucketName),
	})

	var totalCompressedSize int64
	var totalUncompressedSize int64

	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, errors.NewInternalError("failed to list objects").WithCause(err)
		}

		for _, obj := range output.Contents {
			// Skip manifest files
			if strings.HasSuffix(*obj.Key, ".manifest.json") {
				// Process manifest for statistics
				manifest, err := a.getManifestFromKey(ctx, *obj.Key)
				if err != nil {
					continue
				}

				stats.TotalEvents += manifest.EventCount
				totalCompressedSize += manifest.CompressedSize
				totalUncompressedSize += manifest.UncompressedSize

				// Count by year
				year := manifest.StartTime.Year()
				stats.ArchivesByYear[year]++

				// Count by compliance flags
				for flag, count := range manifest.ComplianceFlags {
					stats.EventsByCompliance[flag] += count
				}

				// Update oldest/newest
				if stats.OldestArchive.IsZero() || manifest.StartTime.Before(stats.OldestArchive) {
					stats.OldestArchive = manifest.StartTime
				}
				if manifest.EndTime.After(stats.NewestArchive) {
					stats.NewestArchive = manifest.EndTime
				}
			} else if strings.HasSuffix(*obj.Key, ".parquet") {
				stats.TotalArchives++
				stats.TotalSize += *obj.Size
			}
		}
	}

	// Calculate averages
	if stats.TotalArchives > 0 {
		stats.AverageSize = stats.TotalSize / stats.TotalArchives
	}
	if totalUncompressedSize > 0 {
		stats.CompressionRatio = float64(totalUncompressedSize) / float64(totalCompressedSize)
	}

	return stats, nil
}

// DeleteExpiredArchives removes archives past 7-year retention
func (a *S3Archiver) DeleteExpiredArchives(ctx context.Context) (int64, error) {
	cutoffDate := time.Now().UTC().AddDate(-7, 0, 0)
	var deletedCount int64

	// List archives older than 7 years
	archives, err := a.ListArchives(ctx, time.Time{}, cutoffDate)
	if err != nil {
		return 0, err
	}

	// Delete each expired archive
	for _, archive := range archives {
		// Check for legal hold
		manifest, err := a.GetArchiveManifest(ctx, archive.ArchiveID)
		if err != nil {
			a.logger.Warn(ctx, "Failed to get manifest for deletion", 
				"archive_id", archive.ArchiveID,
				"error", err.Error())
			continue
		}

		if manifest.RetentionPolicy.LegalHold {
			a.logger.Info(ctx, "Skipping archive with legal hold", 
				"archive_id", archive.ArchiveID)
			continue
		}

		// Delete archive and manifest
		if err := a.deleteArchive(ctx, archive.ArchiveID); err != nil {
			a.logger.Error(ctx, "Failed to delete archive", 
				"archive_id", archive.ArchiveID,
				"error", err.Error())
			continue
		}

		deletedCount++
	}

	a.logger.Info(ctx, "Deleted expired archives", 
		"count", deletedCount)

	return deletedCount, nil
}

// RestoreArchive restores archived events back to main storage
func (a *S3Archiver) RestoreArchive(ctx context.Context, archiveID string) (*RestoreResult, error) {
	startTime := time.Now()

	// Download archive
	events, err := a.downloadArchive(ctx, archiveID)
	if err != nil {
		return nil, err
	}

	result := &RestoreResult{
		ArchiveID:      archiveID,
		EventsRestored: int64(len(events)),
		Errors:         make([]string, 0),
	}

	if len(events) > 0 {
		result.StartSequence = values.MustNewSequenceNumber(uint64(events[0].SequenceNum))
		result.EndSequence = values.MustNewSequenceNumber(uint64(events[len(events)-1].SequenceNum))
	}

	// Verify integrity before restoration
	integrityResult, err := a.VerifyArchiveIntegrity(ctx, archiveID)
	if err != nil {
		result.VerificationStatus = "FAILED"
		result.Errors = append(result.Errors, fmt.Sprintf("Integrity verification failed: %v", err))
		return result, nil
	}

	if !integrityResult.IsValid {
		result.VerificationStatus = "INVALID"
		result.Errors = append(result.Errors, integrityResult.Errors...)
		return result, nil
	}

	result.VerificationStatus = "VALID"

	// Restore events to main database
	// This would typically involve:
	// 1. Creating a temporary table
	// 2. Bulk inserting the events
	// 3. Handling any conflicts
	// 4. Moving to main table
	
	// For now, we'll use the batch store method
	if err := a.auditRepo.StoreBatch(ctx, events); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Failed to restore events: %v", err))
		return result, nil
	}

	result.RestoreTime = time.Since(startTime)

	return result, nil
}

// Helper methods

func (a *S3Archiver) ensureBucketExists(ctx context.Context) error {
	_, err := a.s3Client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(a.config.BucketName),
	})
	
	if err != nil {
		// Try to create bucket
		_, createErr := a.s3Client.CreateBucket(ctx, &s3.CreateBucketInput{
			Bucket: aws.String(a.config.BucketName),
		})
		if createErr != nil {
			return errors.NewInternalError("failed to create bucket").WithCause(createErr)
		}

		// Enable versioning
		_, err = a.s3Client.PutBucketVersioning(ctx, &s3.PutBucketVersioningInput{
			Bucket: aws.String(a.config.BucketName),
			VersioningConfiguration: &types.VersioningConfiguration{
				Status: types.BucketVersioningStatusEnabled,
			},
		})
		if err != nil {
			a.logger.Warn(ctx, "Failed to enable versioning", "error", err.Error())
		}

		// Enable encryption
		if a.config.EnableEncryption {
			err = a.enableBucketEncryption(ctx)
			if err != nil {
				a.logger.Warn(ctx, "Failed to enable encryption", "error", err.Error())
			}
		}
	}

	return nil
}

func (a *S3Archiver) setupLifecyclePolicies(ctx context.Context) error {
	rules := []types.LifecycleRule{
		{
			ID:     aws.String("archive-retention"),
			Status: types.ExpirationStatusEnabled,
			Expiration: &types.LifecycleExpiration{
				Days: aws.Int32(int32(a.config.RetentionDays)),
			},
			Filter: &types.LifecycleRuleFilterMemberPrefix{
				Value: "year=",
			},
		},
	}

	if a.config.TransitionDays > 0 {
		rules = append(rules, types.LifecycleRule{
			ID:     aws.String("archive-transition"),
			Status: types.ExpirationStatusEnabled,
			Transitions: []types.Transition{
				{
					Days:         aws.Int32(int32(a.config.TransitionDays)),
					StorageClass: types.TransitionStorageClassGlacier,
				},
			},
			Filter: &types.LifecycleRuleFilterMemberPrefix{
				Value: "year=",
			},
		})
	}

	_, err := a.s3Client.PutBucketLifecycleConfiguration(ctx, &s3.PutBucketLifecycleConfigurationInput{
		Bucket: aws.String(a.config.BucketName),
		LifecycleConfiguration: &types.BucketLifecycleConfiguration{
			Rules: rules,
		},
	})

	return err
}

func (a *S3Archiver) enableBucketEncryption(ctx context.Context) error {
	encryptionConfig := &types.ServerSideEncryptionConfiguration{
		Rules: []types.ServerSideEncryptionRule{
			{
				ApplyServerSideEncryptionByDefault: &types.ServerSideEncryptionByDefault{
					SSEAlgorithm: types.ServerSideEncryptionAes256,
				},
			},
		},
	}

	if a.config.KMSKeyID != "" {
		encryptionConfig.Rules[0].ApplyServerSideEncryptionByDefault = &types.ServerSideEncryptionByDefault{
			SSEAlgorithm:   types.ServerSideEncryptionAwsKms,
			KMSMasterKeyID: aws.String(a.config.KMSKeyID),
		}
	}

	_, err := a.s3Client.PutBucketEncryption(ctx, &s3.PutBucketEncryptionInput{
		Bucket:                            aws.String(a.config.BucketName),
		ServerSideEncryptionConfiguration: encryptionConfig,
	})

	return err
}

func (a *S3Archiver) generateArchiveID() string {
	return fmt.Sprintf("audit_%s", uuid.New().String())
}

func (a *S3Archiver) generateS3Key(timestamp time.Time, archiveID string) string {
	// Path: /year={YYYY}/month={MM}/day={DD}/audit_{timestamp}.parquet
	return fmt.Sprintf("year=%d/month=%02d/day=%02d/%s.parquet",
		timestamp.Year(),
		timestamp.Month(),
		timestamp.Day(),
		archiveID)
}

func (a *S3Archiver) writeParquetFile(buf io.Writer, events []*audit.Event) (int64, error) {
	// Create Parquet writer
	pw, err := writer.NewParquetWriter(buf, new(ParquetEvent), 4)
	if err != nil {
		return 0, err
	}
	defer pw.WriteStop()

	// Configure compression
	pw.CompressionType = parquet.CompressionCodec_SNAPPY
	if a.config.CompressionType == "gzip" {
		pw.CompressionType = parquet.CompressionCodec_GZIP
	} else if a.config.CompressionType == "zstd" {
		pw.CompressionType = parquet.CompressionCodec_ZSTD
	}

	// Write events
	for _, event := range events {
		pe := convertToParquetEvent(event)
		if err := pw.Write(pe); err != nil {
			return 0, err
		}
	}

	// Get size
	if buffer, ok := buf.(*bytes.Buffer); ok {
		return int64(buffer.Len()), nil
	}

	return 0, nil
}

func (a *S3Archiver) calculateUncompressedSize(events []*audit.Event) int64 {
	var size int64
	for _, event := range events {
		// Estimate JSON size
		data, _ := json.Marshal(event)
		size += int64(len(data))
	}
	return size
}

func (a *S3Archiver) verifyHashChain(events []*audit.Event) bool {
	if len(events) <= 1 {
		return true
	}

	for i := 1; i < len(events); i++ {
		if events[i].PreviousHash != events[i-1].EventHash {
			return false
		}
	}

	return true
}

func (a *S3Archiver) createManifest(archiveID string, events []*audit.Event, compressedSize, uncompressedSize int64) *ArchiveManifest {
	manifest := &ArchiveManifest{
		ArchiveID:        archiveID,
		Version:          "1.0",
		CreatedAt:        time.Now().UTC(),
		EventCount:       int64(len(events)),
		StartSequence:    values.MustNewSequenceNumber(uint64(events[0].SequenceNum)),
		EndSequence:      values.MustNewSequenceNumber(uint64(events[len(events)-1].SequenceNum)),
		StartTime:        events[0].Timestamp,
		EndTime:          events[len(events)-1].Timestamp,
		CompressedSize:   compressedSize,
		UncompressedSize: uncompressedSize,
		CompressionType:  a.config.CompressionType,
		Schema:           a.getParquetSchema(),
		ComplianceFlags:  a.countComplianceFlags(events),
		HashChainInfo: HashChainInfo{
			FirstHash:  events[0].EventHash,
			LastHash:   events[len(events)-1].EventHash,
			ChainValid: a.verifyHashChain(events),
			Algorithm:  "SHA-256",
		},
		RetentionPolicy: RetentionPolicy{
			RetentionDays:  a.config.RetentionDays,
			ExpiresAt:      time.Now().UTC().AddDate(0, 0, a.config.RetentionDays),
			LegalHold:      false,
			ComplianceType: "STANDARD",
		},
	}

	return manifest
}

func (a *S3Archiver) getParquetSchema() ParquetSchema {
	return ParquetSchema{
		Version:      "1.0",
		Compression:  a.config.CompressionType,
		RowGroupSize: a.config.RowGroupSize,
		Fields: []ParquetField{
			{Name: "id", Type: "UTF8", Required: true},
			{Name: "sequence_num", Type: "INT64", Required: true},
			{Name: "timestamp", Type: "INT64", Required: true, LogicalType: "TIMESTAMP_MICROS"},
			{Name: "event_type", Type: "UTF8", Required: true},
			{Name: "severity", Type: "UTF8", Required: true},
			{Name: "actor_id", Type: "UTF8", Required: true},
			{Name: "target_id", Type: "UTF8", Required: true},
			{Name: "action", Type: "UTF8", Required: true},
			{Name: "result", Type: "UTF8", Required: true},
			{Name: "metadata", Type: "UTF8", Required: false},
			{Name: "compliance_flags", Type: "UTF8", Required: false},
			{Name: "event_hash", Type: "UTF8", Required: true},
			{Name: "previous_hash", Type: "UTF8", Required: true},
		},
	}
}

func (a *S3Archiver) countComplianceFlags(events []*audit.Event) map[string]int64 {
	counts := make(map[string]int64)
	
	for _, event := range events {
		for flag, enabled := range event.ComplianceFlags {
			if enabled {
				counts[flag]++
			}
		}
	}
	
	return counts
}

func (a *S3Archiver) uploadToS3(ctx context.Context, key string, data []byte, manifest *ArchiveManifest) error {
	// Upload Parquet file
	_, err := a.uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(a.config.BucketName),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String("application/octet-stream"),
		Metadata: map[string]string{
			"archive-id":    manifest.ArchiveID,
			"event-count":   fmt.Sprintf("%d", manifest.EventCount),
			"start-time":    manifest.StartTime.Format(time.RFC3339),
			"end-time":      manifest.EndTime.Format(time.RFC3339),
			"hash-chain":    fmt.Sprintf("%t", manifest.HashChainInfo.ChainValid),
		},
	})
	if err != nil {
		return err
	}

	// Upload manifest
	manifestData, err := json.Marshal(manifest)
	if err != nil {
		return err
	}

	manifestKey := fmt.Sprintf("%s.manifest.json", manifest.ArchiveID)
	_, err = a.uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(a.config.BucketName),
		Key:         aws.String(manifestKey),
		Body:        bytes.NewReader(manifestData),
		ContentType: aws.String("application/json"),
	})

	return err
}

func (a *S3Archiver) markEventsAsArchived(ctx context.Context, events []*audit.Event, archiveID string) error {
	// Update events in database to mark as archived
	// This would typically be done in a transaction
	
	tx, err := a.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	query := `UPDATE audit_events SET archived = true, archive_id = $1 WHERE id = $2`
	
	for _, event := range events {
		_, err := tx.Exec(ctx, query, archiveID, event.ID)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (a *S3Archiver) generatePrefixesForTimeRange(startTime, endTime time.Time) []string {
	prefixes := make([]string, 0)
	
	// Generate year/month prefixes for the range
	current := startTime
	for current.Before(endTime) || current.Equal(endTime) {
		prefix := fmt.Sprintf("year=%d/month=%02d/", current.Year(), current.Month())
		prefixes = append(prefixes, prefix)
		
		// Move to next month
		current = current.AddDate(0, 1, 0)
		if current.Day() != 1 {
			current = time.Date(current.Year(), current.Month(), 1, 0, 0, 0, 0, time.UTC)
		}
	}
	
	return prefixes
}

func (a *S3Archiver) parseArchiveInfo(key string, obj *types.Object) (*ArchiveInfo, error) {
	// Extract archive ID from key
	parts := strings.Split(key, "/")
	if len(parts) < 4 {
		return nil, fmt.Errorf("invalid key format: %s", key)
	}
	
	filename := parts[len(parts)-1]
	archiveID := strings.TrimSuffix(filename, ".parquet")
	
	info := &ArchiveInfo{
		ArchiveID: archiveID,
		S3Key:     key,
		Size:      *obj.Size,
		CreatedAt: *obj.LastModified,
		Status:    "ACTIVE",
	}
	
	// Try to get more info from metadata
	if metadata, err := a.getObjectMetadata(context.Background(), key); err == nil {
		if eventCount, ok := metadata["event-count"]; ok {
			fmt.Sscanf(eventCount, "%d", &info.EventCount)
		}
		if startTime, ok := metadata["start-time"]; ok {
			info.StartTime, _ = time.Parse(time.RFC3339, startTime)
		}
		if endTime, ok := metadata["end-time"]; ok {
			info.EndTime, _ = time.Parse(time.RFC3339, endTime)
		}
	}
	
	// Calculate expiry
	info.ExpiresAt = info.CreatedAt.AddDate(0, 0, a.config.RetentionDays)
	if time.Now().After(info.ExpiresAt) {
		info.Status = "EXPIRED"
	}
	
	return info, nil
}

func (a *S3Archiver) getObjectMetadata(ctx context.Context, key string) (map[string]string, error) {
	output, err := a.s3Client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(a.config.BucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	
	return output.Metadata, nil
}

func (a *S3Archiver) findArchiveForEvent(ctx context.Context, eventID uuid.UUID) (string, error) {
	// This would typically use an index or metadata store
	// For now, we'll need to search through manifests
	
	// This is inefficient and should be optimized with an index
	return "", errors.NewNotFoundError("ARCHIVE_NOT_FOUND", 
		"archive lookup not implemented")
}

func (a *S3Archiver) findArchiveForSequence(ctx context.Context, seq values.SequenceNumber) (string, error) {
	// This would typically use an index or metadata store
	// For now, we'll need to search through manifests
	
	// This is inefficient and should be optimized with an index
	return "", errors.NewNotFoundError("ARCHIVE_NOT_FOUND", 
		"archive lookup not implemented")
}

func (a *S3Archiver) downloadArchive(ctx context.Context, archiveID string) ([]*audit.Event, error) {
	// Find the S3 key for this archive
	key := ""
	
	// List objects to find the archive
	paginator := s3.NewListObjectsV2Paginator(a.s3Client, &s3.ListObjectsV2Input{
		Bucket: aws.String(a.config.BucketName),
	})
	
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		
		for _, obj := range output.Contents {
			if strings.Contains(*obj.Key, archiveID) && strings.HasSuffix(*obj.Key, ".parquet") {
				key = *obj.Key
				break
			}
		}
		
		if key != "" {
			break
		}
	}
	
	if key == "" {
		return nil, errors.NewNotFoundError("ARCHIVE_NOT_FOUND", 
			fmt.Sprintf("archive %s not found", archiveID))
	}
	
	// Download the file
	buf := &bytes.Buffer{}
	_, err := a.downloader.Download(ctx, buf, &s3.GetObjectInput{
		Bucket: aws.String(a.config.BucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	
	// Parse Parquet file
	return a.parseParquetFile(buf.Bytes())
}

func (a *S3Archiver) queryArchiveFile(ctx context.Context, key string, query ArchiveQuery) ([]*audit.Event, error) {
	// Download the file
	buf := &bytes.Buffer{}
	_, err := a.downloader.Download(ctx, buf, &s3.GetObjectInput{
		Bucket: aws.String(a.config.BucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	
	// Parse Parquet file
	events, err := a.parseParquetFile(buf.Bytes())
	if err != nil {
		return nil, err
	}
	
	// Filter events based on query
	filtered := make([]*audit.Event, 0)
	for _, event := range events {
		if a.matchesQuery(event, query) {
			filtered = append(filtered, event)
			
			if query.Limit > 0 && len(filtered) >= query.Limit {
				break
			}
		}
	}
	
	return filtered, nil
}

func (a *S3Archiver) matchesQuery(event *audit.Event, query ArchiveQuery) bool {
	// Check time range
	if event.Timestamp.Before(query.StartTime) || event.Timestamp.After(query.EndTime) {
		return false
	}
	
	// Check event types
	if len(query.EventTypes) > 0 {
		found := false
		for _, t := range query.EventTypes {
			if event.Type == t {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	
	// Check actor IDs
	if len(query.ActorIDs) > 0 {
		found := false
		for _, id := range query.ActorIDs {
			if event.ActorID == id {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	
	// Check target IDs
	if len(query.TargetIDs) > 0 {
		found := false
		for _, id := range query.TargetIDs {
			if event.TargetID == id {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	
	// Check compliance flags
	if len(query.ComplianceFlags) > 0 {
		for _, flag := range query.ComplianceFlags {
			if !event.HasComplianceFlag(flag) {
				return false
			}
		}
	}
	
	// Check sequence range
	if query.SequenceStart != nil && event.SequenceNum < int64(query.SequenceStart.Value()) {
		return false
	}
	if query.SequenceEnd != nil && event.SequenceNum > int64(query.SequenceEnd.Value()) {
		return false
	}
	
	return true
}

func (a *S3Archiver) parseParquetFile(data []byte) ([]*audit.Event, error) {
	// This would use a Parquet reader to parse the file
	// For now, return an error indicating implementation needed
	return nil, errors.NewInternalError("parquet parsing not fully implemented")
}

func (a *S3Archiver) getManifestFromKey(ctx context.Context, key string) (*ArchiveManifest, error) {
	output, err := a.s3Client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(a.config.BucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	defer output.Body.Close()
	
	var manifest ArchiveManifest
	if err := json.NewDecoder(output.Body).Decode(&manifest); err != nil {
		return nil, err
	}
	
	return &manifest, nil
}

func (a *S3Archiver) deleteArchive(ctx context.Context, archiveID string) error {
	// Delete Parquet file
	// Find the key first
	key := ""
	manifestKey := fmt.Sprintf("%s.manifest.json", archiveID)
	
	// List objects to find the archive
	paginator := s3.NewListObjectsV2Paginator(a.s3Client, &s3.ListObjectsV2Input{
		Bucket: aws.String(a.config.BucketName),
	})
	
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return err
		}
		
		for _, obj := range output.Contents {
			if strings.Contains(*obj.Key, archiveID) && strings.HasSuffix(*obj.Key, ".parquet") {
				key = *obj.Key
				break
			}
		}
		
		if key != "" {
			break
		}
	}
	
	// Delete both files
	if key != "" {
		_, err := a.s3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(a.config.BucketName),
			Key:    aws.String(key),
		})
		if err != nil {
			return err
		}
	}
	
	_, err := a.s3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(a.config.BucketName),
		Key:    aws.String(manifestKey),
	})
	
	return err
}

// ParquetEvent represents the structure for Parquet storage
type ParquetEvent struct {
	ID              string `parquet:"name=id, type=UTF8"`
	SequenceNum     int64  `parquet:"name=sequence_num, type=INT64"`
	Timestamp       int64  `parquet:"name=timestamp, type=INT64"`
	EventType       string `parquet:"name=event_type, type=UTF8"`
	Severity        string `parquet:"name=severity, type=UTF8"`
	ActorID         string `parquet:"name=actor_id, type=UTF8"`
	ActorType       string `parquet:"name=actor_type, type=UTF8"`
	TargetID        string `parquet:"name=target_id, type=UTF8"`
	TargetType      string `parquet:"name=target_type, type=UTF8"`
	Action          string `parquet:"name=action, type=UTF8"`
	Result          string `parquet:"name=result, type=UTF8"`
	Metadata        string `parquet:"name=metadata, type=UTF8"`
	ComplianceFlags string `parquet:"name=compliance_flags, type=UTF8"`
	EventHash       string `parquet:"name=event_hash, type=UTF8"`
	PreviousHash    string `parquet:"name=previous_hash, type=UTF8"`
}

func convertToParquetEvent(e *audit.Event) *ParquetEvent {
	metadataJSON, _ := json.Marshal(e.Metadata)
	complianceFlagsJSON, _ := json.Marshal(e.ComplianceFlags)
	
	return &ParquetEvent{
		ID:              e.ID.String(),
		SequenceNum:     e.SequenceNum,
		Timestamp:       e.Timestamp.UnixMicro(),
		EventType:       string(e.Type),
		Severity:        string(e.Severity),
		ActorID:         e.ActorID,
		ActorType:       e.ActorType,
		TargetID:        e.TargetID,
		TargetType:      e.TargetType,
		Action:          e.Action,
		Result:          e.Result,
		Metadata:        string(metadataJSON),
		ComplianceFlags: string(complianceFlagsJSON),
		EventHash:       e.EventHash,
		PreviousHash:    e.PreviousHash,
	}
}