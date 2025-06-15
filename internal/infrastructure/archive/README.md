# Audit Event Archive System

This package implements the S3-based archival system for long-term audit event storage with 7-year retention compliance.

## Overview

The archive system provides:
- Automatic archival of events older than 90 days
- Parquet format compression with configurable algorithms (snappy, gzip, zstd)
- Cryptographic integrity verification through hash chains
- Compliance query support on archived data
- S3 lifecycle policies for cost optimization
- Event restoration capabilities for audits

## Architecture

### Components

1. **S3Archiver** - Main implementation using AWS S3
   - Batch processing for efficient archival
   - Parquet file generation with compression
   - Manifest files for metadata and integrity
   - Query capabilities on archived data

2. **ArchiverRepository** - Interface for archive operations
   - Archive events by age
   - Query archived data
   - Verify integrity
   - Manage retention

3. **Factory** - Creates archiver instances based on configuration

### S3 Structure

```
dce-audit-archive-{env}/
├── year=2024/
│   ├── month=01/
│   │   ├── day=01/
│   │   │   ├── audit_123e4567-e89b-12d3-a456-426614174000.parquet
│   │   │   └── audit_223e4567-e89b-12d3-a456-426614174000.parquet
│   │   └── day=02/
│   │       └── audit_323e4567-e89b-12d3-a456-426614174000.parquet
│   └── month=02/
└── audit_123e4567-e89b-12d3-a456-426614174000.manifest.json
```

### File Formats

#### Parquet Schema
- Columnar storage for efficient queries
- Compression reduces storage by ~75%
- Schema evolution support
- Partitioned by date for query optimization

#### Manifest Files
- JSON format with archive metadata
- Hash chain information
- Compliance flags summary
- Retention policy details

## Configuration

```yaml
archive:
  provider: s3                    # Storage provider (s3, gcs, azure)
  bucket: dce-audit-archive-prod  # S3 bucket name
  region: us-east-1              # AWS region
  
  # Archive settings
  batch_size: 1000               # Events per batch
  compression: snappy            # Compression type
  row_group_size: 100000         # Parquet row group size
  retention_days: 2555           # 7 years
  
  # Performance
  max_concurrency: 10            # Parallel uploads
  upload_part_size: 5242880      # 5MB multipart size
  timeout: 5m                    # Operation timeout
  
  # Lifecycle
  enable_lifecycle: true         # Enable S3 lifecycle rules
  transition_days: 90            # Days before Glacier
  
  # Security
  enable_encryption: true        # Server-side encryption
  kms_key_id: ""                # Optional KMS key
```

## Usage

### Archiving Events

```go
// Create archiver
factory := archive.NewFactory(db, auditRepo, config, logger)
archiver, err := factory.CreateArchiver(ctx)

// Archive events older than 90 days
cutoffDate := time.Now().AddDate(0, 0, -90)
count, err := archiver.ArchiveEvents(ctx, cutoffDate, 1000)
```

### Querying Archives

```go
// Query archived events
query := archive.ArchiveQuery{
    StartTime: time.Now().AddDate(0, -6, 0),
    EndTime:   time.Now().AddDate(0, -3, 0),
    EventTypes: []audit.EventType{
        audit.EventCallInitiated,
        audit.EventConsentGranted,
    },
    ComplianceFlags: []string{"tcpa_relevant"},
    Limit: 100,
}

result, err := archiver.QueryArchive(ctx, query)
```

### Verifying Integrity

```go
// Verify archive integrity
integrityResult, err := archiver.VerifyArchiveIntegrity(ctx, archiveID)
if !integrityResult.IsValid {
    log.Error("Archive integrity check failed", "errors", integrityResult.Errors)
}
```

### Restoring Archives

```go
// Restore archived events for audit
restoreResult, err := archiver.RestoreArchive(ctx, archiveID)
if restoreResult.VerificationStatus != "VALID" {
    log.Error("Archive verification failed during restore")
}
```

## S3 Lifecycle Policies

The system automatically configures S3 lifecycle policies:

1. **Transition to Glacier** - After 90 days
2. **Expiration** - After 7 years (2555 days)
3. **Legal Hold** - Prevents deletion when set

## Performance Considerations

1. **Batch Size** - Larger batches improve compression but use more memory
2. **Compression** - Snappy is fastest, gzip/zstd compress better
3. **Concurrency** - Balance between speed and resource usage
4. **Query Performance** - Time-based partitioning enables efficient queries

## Security

1. **Encryption at Rest** - AES-256 or KMS encryption
2. **Encryption in Transit** - TLS for all S3 operations
3. **Hash Chain Integrity** - Cryptographic verification
4. **Access Control** - IAM policies restrict access

## Monitoring

Key metrics to monitor:
- Archive operation duration
- Compression ratios
- Failed archival attempts
- Query performance
- Storage growth rate
- Integrity check failures

## Testing

The package includes comprehensive tests:
- Unit tests with mocked S3 client
- Integration tests with MinIO
- Property-based tests for compression
- Integrity verification tests

Run tests:
```bash
go test ./internal/infrastructure/archive/...
```

## Future Enhancements

1. **Multi-Cloud Support** - GCS and Azure implementations
2. **Incremental Archival** - Archive as events arrive
3. **Archive Index** - Dedicated index for faster queries
4. **Compression Optimization** - Adaptive compression selection
5. **Parallel Query** - Query multiple archives concurrently

## Compliance Notes

- **GDPR** - Supports right to erasure through event filtering
- **TCPA** - Preserves consent records with full context
- **SOX** - Immutable audit trail with integrity verification
- **HIPAA** - Encryption and access controls for PHI

## Dependencies

- AWS SDK v2 for S3 operations
- xitongsys/parquet-go for Parquet files
- Standard compression libraries

Note: You'll need to add the Parquet dependency to go.mod:
```
go get github.com/xitongsys/parquet-go/parquet
go get github.com/xitongsys/parquet-go/writer
go get github.com/xitongsys/parquet-go/reader
```