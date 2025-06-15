package archive

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/audit"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/testutil"
)

// MockS3Client is a mock implementation of S3 client for testing
type MockS3Client struct {
	mock.Mock
}

func (m *MockS3Client) HeadBucket(ctx context.Context, input *s3.HeadBucketInput, opts ...func(*s3.Options)) (*s3.HeadBucketOutput, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*s3.HeadBucketOutput), args.Error(1)
}

func (m *MockS3Client) CreateBucket(ctx context.Context, input *s3.CreateBucketInput, opts ...func(*s3.Options)) (*s3.CreateBucketOutput, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*s3.CreateBucketOutput), args.Error(1)
}

func (m *MockS3Client) PutObject(ctx context.Context, input *s3.PutObjectInput, opts ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*s3.PutObjectOutput), args.Error(1)
}

func (m *MockS3Client) GetObject(ctx context.Context, input *s3.GetObjectInput, opts ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*s3.GetObjectOutput), args.Error(1)
}

func (m *MockS3Client) ListObjectsV2(ctx context.Context, input *s3.ListObjectsV2Input, opts ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*s3.ListObjectsV2Output), args.Error(1)
}

func (m *MockS3Client) DeleteObject(ctx context.Context, input *s3.DeleteObjectInput, opts ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	args := m.Called(ctx, input)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*s3.DeleteObjectOutput), args.Error(1)
}

func TestS3Archiver_ArchiveBatch(t *testing.T) {
	// Create test events
	events := createTestEvents(100)

	// Create mock S3 client
	mockS3 := &MockS3Client{}

	// Set up expectations
	mockS3.On("HeadBucket", mock.Anything, mock.Anything).Return(&s3.HeadBucketOutput{}, nil)
	mockS3.On("PutObject", mock.Anything, mock.MatchedBy(func(input *s3.PutObjectInput) bool {
		return strings.HasSuffix(*input.Key, ".parquet")
	})).Return(&s3.PutObjectOutput{}, nil)
	mockS3.On("PutObject", mock.Anything, mock.MatchedBy(func(input *s3.PutObjectInput) bool {
		return strings.HasSuffix(*input.Key, ".manifest.json")
	})).Return(&s3.PutObjectOutput{}, nil)

	// Create archiver
	archiver := &S3Archiver{
		s3Client: mockS3,
		config: ArchiveConfig{
			BucketName:      "test-bucket",
			Region:          "us-east-1",
			BatchSize:       100,
			CompressionType: "snappy",
			RetentionDays:   2555, // 7 years
		},
		logger: testutil.NewTestLogger(),
	}

	// Archive batch
	result, err := archiver.ArchiveBatch(context.Background(), events)
	
	// Assertions
	require.NoError(t, err)
	assert.Equal(t, int64(100), result.EventCount)
	assert.Equal(t, events[0].SequenceNum, int64(result.StartSequence.Value()))
	assert.Equal(t, events[99].SequenceNum, int64(result.EndSequence.Value()))
	assert.True(t, result.HashChainValid)
	assert.Greater(t, result.CompressionRatio, 1.0)
	assert.Contains(t, result.S3Location, "s3://test-bucket/")

	// Verify mock expectations
	mockS3.AssertExpectations(t)
}

func TestS3Archiver_VerifyArchiveIntegrity(t *testing.T) {
	archiveID := "audit_" + uuid.New().String()
	events := createTestEvents(10)

	// Create manifest
	manifest := &ArchiveManifest{
		ArchiveID:        archiveID,
		Version:          "1.0",
		CreatedAt:        time.Now().UTC(),
		EventCount:       int64(len(events)),
		StartSequence:    values.MustNewSequenceNumber(uint64(events[0].SequenceNum)),
		EndSequence:      values.MustNewSequenceNumber(uint64(events[len(events)-1].SequenceNum)),
		StartTime:        events[0].Timestamp,
		EndTime:          events[len(events)-1].Timestamp,
		CompressedSize:   1024,
		UncompressedSize: 4096,
		CompressionType:  "snappy",
		HashChainInfo: HashChainInfo{
			FirstHash:  events[0].EventHash,
			LastHash:   events[len(events)-1].EventHash,
			ChainValid: true,
			Algorithm:  "SHA-256",
		},
	}

	manifestJSON, _ := json.Marshal(manifest)

	// Create mock S3 client
	mockS3 := &MockS3Client{}
	
	// Mock manifest retrieval
	mockS3.On("GetObject", mock.Anything, mock.MatchedBy(func(input *s3.GetObjectInput) bool {
		return strings.HasSuffix(*input.Key, ".manifest.json")
	})).Return(&s3.GetObjectOutput{
		Body: io.NopCloser(bytes.NewReader(manifestJSON)),
	}, nil)

	// Mock archive download (would return Parquet data)
	mockS3.On("ListObjectsV2", mock.Anything, mock.Anything).Return(&s3.ListObjectsV2Output{
		Contents: []types.Object{
			{
				Key:  aws.String(fmt.Sprintf("year=2024/month=01/day=01/%s.parquet", archiveID)),
				Size: aws.Int64(1024),
			},
		},
	}, nil)

	// Create archiver
	archiver := &S3Archiver{
		s3Client: mockS3,
		config: ArchiveConfig{
			BucketName: "test-bucket",
		},
		logger: testutil.NewTestLogger(),
	}

	// Verify integrity
	result, err := archiver.VerifyArchiveIntegrity(context.Background(), archiveID)
	
	// Assertions
	require.NoError(t, err)
	assert.Equal(t, archiveID, result.ArchiveID)
	assert.Equal(t, int64(10), result.EventCount)
	assert.True(t, result.MetadataValid)
}

func TestS3Archiver_QueryArchive(t *testing.T) {
	ctx := context.Background()
	
	// Create test query
	query := ArchiveQuery{
		StartTime: time.Now().UTC().Add(-30 * 24 * time.Hour),
		EndTime:   time.Now().UTC(),
		EventTypes: []audit.EventType{
			audit.EventCallInitiated,
			audit.EventCallCompleted,
		},
		Limit: 50,
	}

	// Create mock S3 client
	mockS3 := &MockS3Client{}
	
	// Mock listing archives
	mockS3.On("ListObjectsV2", mock.Anything, mock.Anything).Return(&s3.ListObjectsV2Output{
		Contents: []types.Object{
			{
				Key:          aws.String("year=2024/month=01/day=01/audit_test1.parquet"),
				Size:         aws.Int64(1024),
				LastModified: aws.Time(time.Now().Add(-7 * 24 * time.Hour)),
			},
			{
				Key:          aws.String("year=2024/month=01/day=02/audit_test2.parquet"),
				Size:         aws.Int64(2048),
				LastModified: aws.Time(time.Now().Add(-6 * 24 * time.Hour)),
			},
		},
		IsTruncated: aws.Bool(false),
	}, nil)

	// Mock head object for metadata
	mockS3.On("HeadObject", mock.Anything, mock.Anything).Return(&s3.HeadObjectOutput{
		Metadata: map[string]string{
			"event-count": "100",
			"start-time":  query.StartTime.Format(time.RFC3339),
			"end-time":    query.EndTime.Format(time.RFC3339),
		},
	}, nil)

	// Create archiver
	archiver := &S3Archiver{
		s3Client: mockS3,
		config: ArchiveConfig{
			BucketName: "test-bucket",
		},
		logger: testutil.NewTestLogger(),
	}

	// Query archive
	result, err := archiver.QueryArchive(ctx, query)
	
	// Assertions
	require.NoError(t, err)
	assert.Equal(t, 2, result.ArchivesQueried)
	assert.NotNil(t, result.QueryTime)
}

func TestS3Archiver_GetArchiveStats(t *testing.T) {
	ctx := context.Background()
	
	// Create test manifests
	manifest1 := &ArchiveManifest{
		ArchiveID:        "audit_1",
		EventCount:       1000,
		StartTime:        time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
		EndTime:          time.Date(2023, 1, 31, 23, 59, 59, 0, time.UTC),
		CompressedSize:   1024 * 1024,
		UncompressedSize: 4 * 1024 * 1024,
		ComplianceFlags: map[string]int64{
			"gdpr_relevant":  250,
			"tcpa_relevant":  100,
			"contains_pii":   300,
		},
	}
	
	manifest2 := &ArchiveManifest{
		ArchiveID:        "audit_2",
		EventCount:       2000,
		StartTime:        time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		EndTime:          time.Date(2024, 1, 31, 23, 59, 59, 0, time.UTC),
		CompressedSize:   2 * 1024 * 1024,
		UncompressedSize: 8 * 1024 * 1024,
		ComplianceFlags: map[string]int64{
			"gdpr_relevant":  500,
			"tcpa_relevant":  200,
		},
	}

	manifest1JSON, _ := json.Marshal(manifest1)
	manifest2JSON, _ := json.Marshal(manifest2)

	// Create mock S3 client
	mockS3 := &MockS3Client{}
	
	// Mock listing all objects
	mockS3.On("ListObjectsV2", mock.Anything, mock.Anything).Return(&s3.ListObjectsV2Output{
		Contents: []types.Object{
			{
				Key:  aws.String("year=2023/month=01/day=01/audit_1.parquet"),
				Size: aws.Int64(1024 * 1024),
			},
			{
				Key:  aws.String("audit_1.manifest.json"),
				Size: aws.Int64(512),
			},
			{
				Key:  aws.String("year=2024/month=01/day=01/audit_2.parquet"),
				Size: aws.Int64(2 * 1024 * 1024),
			},
			{
				Key:  aws.String("audit_2.manifest.json"),
				Size: aws.Int64(512),
			},
		},
		IsTruncated: aws.Bool(false),
	}, nil)

	// Mock manifest retrieval
	mockS3.On("GetObject", mock.Anything, mock.MatchedBy(func(input *s3.GetObjectInput) bool {
		return *input.Key == "audit_1.manifest.json"
	})).Return(&s3.GetObjectOutput{
		Body: io.NopCloser(bytes.NewReader(manifest1JSON)),
	}, nil)
	
	mockS3.On("GetObject", mock.Anything, mock.MatchedBy(func(input *s3.GetObjectInput) bool {
		return *input.Key == "audit_2.manifest.json"
	})).Return(&s3.GetObjectOutput{
		Body: io.NopCloser(bytes.NewReader(manifest2JSON)),
	}, nil)

	// Create archiver
	archiver := &S3Archiver{
		s3Client: mockS3,
		config: ArchiveConfig{
			BucketName: "test-bucket",
		},
		logger: testutil.NewTestLogger(),
	}

	// Get stats
	stats, err := archiver.GetArchiveStats(ctx)
	
	// Assertions
	require.NoError(t, err)
	assert.Equal(t, int64(2), stats.TotalArchives)
	assert.Equal(t, int64(3000), stats.TotalEvents)
	assert.Equal(t, int64(3*1024*1024), stats.TotalSize)
	assert.Equal(t, int64(750), stats.EventsByCompliance["gdpr_relevant"])
	assert.Equal(t, int64(300), stats.EventsByCompliance["tcpa_relevant"])
	assert.Equal(t, int64(300), stats.EventsByCompliance["contains_pii"])
	assert.Equal(t, int64(1), stats.ArchivesByYear[2023])
	assert.Equal(t, int64(1), stats.ArchivesByYear[2024])
	assert.Equal(t, float64(4), stats.CompressionRatio)
}

func TestS3Archiver_DeleteExpiredArchives(t *testing.T) {
	ctx := context.Background()
	archiveID := "audit_expired"
	
	// Create expired manifest
	manifest := &ArchiveManifest{
		ArchiveID:  archiveID,
		StartTime:  time.Now().UTC().AddDate(-8, 0, 0), // 8 years ago
		EndTime:    time.Now().UTC().AddDate(-8, 0, 0).Add(24 * time.Hour),
		RetentionPolicy: RetentionPolicy{
			RetentionDays: 2555,
			ExpiresAt:     time.Now().UTC().AddDate(-1, 0, 0), // Expired 1 year ago
			LegalHold:     false,
		},
	}

	manifestJSON, _ := json.Marshal(manifest)

	// Create mock S3 client
	mockS3 := &MockS3Client{}
	
	// Mock listing expired archives
	mockS3.On("ListObjectsV2", mock.Anything, mock.Anything).Return(&s3.ListObjectsV2Output{
		Contents: []types.Object{
			{
				Key:          aws.String(fmt.Sprintf("year=2016/month=01/day=01/%s.parquet", archiveID)),
				Size:         aws.Int64(1024),
				LastModified: aws.Time(time.Now().UTC().AddDate(-8, 0, 0)),
			},
		},
		IsTruncated: aws.Bool(false),
	}, nil).Once()

	// Mock metadata retrieval
	mockS3.On("HeadObject", mock.Anything, mock.Anything).Return(&s3.HeadObjectOutput{
		Metadata: map[string]string{
			"event-count": "100",
			"start-time":  manifest.StartTime.Format(time.RFC3339),
			"end-time":    manifest.EndTime.Format(time.RFC3339),
		},
	}, nil)

	// Mock manifest retrieval
	mockS3.On("GetObject", mock.Anything, mock.MatchedBy(func(input *s3.GetObjectInput) bool {
		return strings.HasSuffix(*input.Key, ".manifest.json")
	})).Return(&s3.GetObjectOutput{
		Body: io.NopCloser(bytes.NewReader(manifestJSON)),
	}, nil)

	// Mock finding archive for deletion
	mockS3.On("ListObjectsV2", mock.Anything, mock.Anything).Return(&s3.ListObjectsV2Output{
		Contents: []types.Object{
			{
				Key: aws.String(fmt.Sprintf("year=2016/month=01/day=01/%s.parquet", archiveID)),
			},
		},
		IsTruncated: aws.Bool(false),
	}, nil)

	// Mock deletion
	mockS3.On("DeleteObject", mock.Anything, mock.MatchedBy(func(input *s3.DeleteObjectInput) bool {
		return strings.HasSuffix(*input.Key, ".parquet")
	})).Return(&s3.DeleteObjectOutput{}, nil)
	
	mockS3.On("DeleteObject", mock.Anything, mock.MatchedBy(func(input *s3.DeleteObjectInput) bool {
		return strings.HasSuffix(*input.Key, ".manifest.json")
	})).Return(&s3.DeleteObjectOutput{}, nil)

	// Create archiver
	archiver := &S3Archiver{
		s3Client: mockS3,
		config: ArchiveConfig{
			BucketName:    "test-bucket",
			RetentionDays: 2555, // 7 years
		},
		logger: testutil.NewTestLogger(),
	}

	// Delete expired archives
	deletedCount, err := archiver.DeleteExpiredArchives(ctx)
	
	// Assertions
	require.NoError(t, err)
	assert.Equal(t, int64(1), deletedCount)
	
	// Verify all expected calls were made
	mockS3.AssertExpectations(t)
}

// Helper function to create test events
func createTestEvents(count int) []*audit.Event {
	events := make([]*audit.Event, count)
	previousHash := ""
	
	for i := 0; i < count; i++ {
		event, _ := audit.NewEvent(
			audit.EventCallInitiated,
			fmt.Sprintf("actor-%d", i),
			fmt.Sprintf("target-%d", i),
			"test-action",
		)
		
		event.SequenceNum = int64(i + 1)
		event.Timestamp = time.Now().UTC().Add(-time.Duration(count-i) * time.Hour)
		event.ComplianceFlags["gdpr_relevant"] = i%2 == 0
		event.ComplianceFlags["tcpa_relevant"] = i%3 == 0
		
		// Compute hash chain
		hash, _ := event.ComputeHash(previousHash)
		previousHash = hash
		
		events[i] = event
	}
	
	return events
}

// TestS3Archiver_RestoreArchive tests restoring archived events
func TestS3Archiver_RestoreArchive(t *testing.T) {
	ctx := context.Background()
	archiveID := "audit_test_restore"
	events := createTestEvents(10)
	
	// Create manifest
	manifest := &ArchiveManifest{
		ArchiveID:     archiveID,
		EventCount:    int64(len(events)),
		StartSequence: values.MustNewSequenceNumber(uint64(events[0].SequenceNum)),
		EndSequence:   values.MustNewSequenceNumber(uint64(events[len(events)-1].SequenceNum)),
		HashChainInfo: HashChainInfo{
			ChainValid: true,
		},
	}
	
	manifestJSON, _ := json.Marshal(manifest)
	
	// Create mock S3 client
	mockS3 := &MockS3Client{}
	
	// Mock manifest retrieval for integrity check
	mockS3.On("GetObject", mock.Anything, mock.MatchedBy(func(input *s3.GetObjectInput) bool {
		return strings.HasSuffix(*input.Key, ".manifest.json")
	})).Return(&s3.GetObjectOutput{
		Body: io.NopCloser(bytes.NewReader(manifestJSON)),
	}, nil)
	
	// Mock finding archive file
	mockS3.On("ListObjectsV2", mock.Anything, mock.Anything).Return(&s3.ListObjectsV2Output{
		Contents: []types.Object{
			{
				Key: aws.String(fmt.Sprintf("year=2024/month=01/day=01/%s.parquet", archiveID)),
			},
		},
		IsTruncated: aws.Bool(false),
	}, nil)
	
	// Create archiver with mock audit repository
	mockAuditRepo := &MockAuditRepository{}
	mockAuditRepo.On("StoreBatch", mock.Anything, mock.Anything).Return(nil)
	
	archiver := &S3Archiver{
		s3Client:  mockS3,
		auditRepo: mockAuditRepo,
		config: ArchiveConfig{
			BucketName: "test-bucket",
		},
		logger: testutil.NewTestLogger(),
	}
	
	// Restore archive
	result, err := archiver.RestoreArchive(ctx, archiveID)
	
	// Assertions
	require.NoError(t, err)
	assert.Equal(t, archiveID, result.ArchiveID)
	assert.Equal(t, "VALID", result.VerificationStatus)
	assert.Empty(t, result.Errors)
}

// MockAuditRepository for testing
type MockAuditRepository struct {
	mock.Mock
}

func (m *MockAuditRepository) StoreBatch(ctx context.Context, events []*audit.Event) error {
	args := m.Called(ctx, events)
	return args.Error(0)
}

func (m *MockAuditRepository) GetExpiredEvents(ctx context.Context, before time.Time, limit int) ([]*audit.Event, error) {
	args := m.Called(ctx, before, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*audit.Event), args.Error(1)
}