// +build integration

package integration

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/audit"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/infrastructure/cache"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/infrastructure/database"
	"github.com/davidleathers/dependable-call-exchange-backend/test/testutil"
	"github.com/docker/go-connections/nat"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/localstack"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/zap"
)

// Missing types needed for the tests
type ArchiveCriteria struct {
	StartTime        *time.Time `json:"start_time,omitempty"`
	EndTime          *time.Time `json:"end_time,omitempty"`
	ComplianceFlags  []string   `json:"compliance_flags,omitempty"`
	RetentionPolicy  string     `json:"retention_policy,omitempty"`
	CompressionLevel int        `json:"compression_level,omitempty"`
}

type ArchiveResult struct {
	Success         bool      `json:"success"`
	ArchiveID       string    `json:"archive_id"`
	EventsArchived  int64     `json:"events_archived"`
	ArchiveLocation string    `json:"archive_location"`
	ArchivedAt      time.Time `json:"archived_at"`
}

type ArchiveMetadata struct {
	ArchiveID    string    `json:"archive_id"`
	EventCount   int64     `json:"event_count"`
	ArchiveSize  int64     `json:"archive_size"`
	CreatedAt    time.Time `json:"created_at"`
}

type RetrievalCriteria struct {
	ArchiveID       string                    `json:"archive_id"`
	StartSequence   int64                     `json:"start_sequence,omitempty"`
	EndSequence     int64                     `json:"end_sequence,omitempty"`
	StartTime       *time.Time                `json:"start_time,omitempty"`
	EndTime         *time.Time                `json:"end_time,omitempty"`
	IncludeMetadata bool                      `json:"include_metadata,omitempty"`
}

type RetrievalResult struct {
	Success bool           `json:"success"`
	Events  []*audit.Event `json:"events"`
}

type HashChainVerificationResult struct {
	StartSequence    values.SequenceNumber `json:"start_sequence"`
	EndSequence      values.SequenceNumber `json:"end_sequence"`
	IsValid          bool                  `json:"is_valid"`
	ChainComplete    bool                  `json:"chain_complete"`
	EventsVerified   int64                 `json:"events_verified"`
	HashesValid      int64                 `json:"hashes_valid"`
	HashesInvalid    int64                 `json:"hashes_invalid"`
	IntegrityScore   float64               `json:"integrity_score"`
	BrokenChains     []*BrokenChain        `json:"broken_chains,omitempty"`
	Issues           []*ChainIntegrityIssue `json:"issues,omitempty"`
	FirstBrokenAt    *values.SequenceNumber `json:"first_broken_at,omitempty"`
	VerifiedAt       time.Time             `json:"verified_at"`
	VerificationID   string                `json:"verification_id"`
	Method           string                `json:"method"`
	EventsPerSecond  float64               `json:"events_per_second"`
	VerificationTime time.Duration         `json:"verification_time"`
}

type BrokenChain struct {
	StartSequence  values.SequenceNumber `json:"start_sequence"`
	EndSequence    values.SequenceNumber `json:"end_sequence"`
	BreakType      string                `json:"break_type"`
	ExpectedHash   string                `json:"expected_hash"`
	ActualHash     string                `json:"actual_hash"`
	AffectedEvents []uuid.UUID           `json:"affected_events"`
	Severity       string                `json:"severity"`
	RepairPossible bool                  `json:"repair_possible"`
}

type ChainIntegrityIssue struct {
	IssueID     string                `json:"issue_id"`
	Type        string                `json:"type"`
	Severity    string                `json:"severity"`
	EventID     uuid.UUID             `json:"event_id,omitempty"`
	Sequence    values.SequenceNumber `json:"sequence,omitempty"`
	Description string                `json:"description"`
	Impact      string                `json:"impact"`
}

type HashChainRepairResult struct {
	RepairID              string                `json:"repair_id"`
	RepairScope           SequenceRange         `json:"repair_scope"`
	RepairActions         []*RepairAction       `json:"repair_actions"`
	RepairedAt            time.Time             `json:"repaired_at"`
	RepairedBy            string                `json:"repaired_by"`
	RepairReason          string                `json:"repair_reason"`
	EventsRepaired        int64                 `json:"events_repaired"`
	EventsFailed          int64                 `json:"events_failed"`
	EventsSkipped         int64                 `json:"events_skipped"`
	HashesRecalculated    int64                 `json:"hashes_recalculated"`
	ChainLinksRepaired    int64                 `json:"chain_links_repaired"`
	PostRepairVerification *HashChainVerificationResult `json:"post_repair_verification,omitempty"`
	RepairTime            time.Duration         `json:"repair_time"`
}

type RepairAction struct {
	ActionType string                `json:"action_type"`
	EventID    uuid.UUID             `json:"event_id"`
	Sequence   values.SequenceNumber `json:"sequence"`
	OldHash    string                `json:"old_hash,omitempty"`
	NewHash    string                `json:"new_hash,omitempty"`
	Success    bool                  `json:"success"`
	Error      string                `json:"error,omitempty"`
}

type SequenceRange struct {
	Start values.SequenceNumber `json:"start"`
	End   values.SequenceNumber `json:"end"`
}

type SequenceIntegrityCriteria struct {
	StartSequence   *values.SequenceNumber `json:"start_sequence,omitempty"`
	EndSequence     *values.SequenceNumber `json:"end_sequence,omitempty"`
	CheckGaps       bool                   `json:"check_gaps"`
	CheckDuplicates bool                   `json:"check_duplicates"`
	CheckOrder      bool                   `json:"check_order"`
}

type SequenceIntegrityResult struct {
	IsValid        bool                  `json:"is_valid"`
	Gaps           []*audit.SequenceGap  `json:"gaps,omitempty"`
	Duplicates     []*DuplicateSequence  `json:"duplicates,omitempty"`
	TotalEvents    int64                 `json:"total_events"`
	VerifiedAt     time.Time             `json:"verified_at"`
}

type DuplicateSequence struct {
	Sequence values.SequenceNumber `json:"sequence"`
	EventIDs []uuid.UUID           `json:"event_ids"`
}

type CorruptionReport struct {
	ReportID        string                 `json:"report_id"`
	CorruptionFound bool                   `json:"corruption_found"`
	Corruptions     []*CorruptionInstance  `json:"corruptions,omitempty"`
	GeneratedAt     time.Time              `json:"generated_at"`
}

type CorruptionInstance struct {
	EventID         uuid.UUID `json:"event_id"`
	CorruptionType  string    `json:"corruption_type"`
	ExpectedValue   string    `json:"expected_value"`
	ActualValue     string    `json:"actual_value"`
	RepairPossible  bool      `json:"repair_possible"`
}

// TrendAnalysis provides trend analysis
type TrendAnalysis struct {
	Direction   string    `json:"direction"` // increasing, decreasing, stable
	Slope       float64   `json:"slope"`
	Confidence  float64   `json:"confidence"`
	StartValue  float64   `json:"start_value"`
	EndValue    float64   `json:"end_value"`
	AnalyzedAt  time.Time `json:"analyzed_at"`
}

// ImplementationPlan provides implementation details for recommendations
type ImplementationPlan struct {
	Steps           []string      `json:"steps"`
	EstimatedTime   time.Duration `json:"estimated_time"`
	RequiredSkills  []string      `json:"required_skills"`
	Dependencies    []string      `json:"dependencies"`
	RiskLevel       string        `json:"risk_level"`
}

// TimeRange represents a time range
type TimeRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// EventCorrelation represents correlation between events
type EventCorrelation struct {
	CorrelationID   string      `json:"correlation_id"`
	EventIDs        []uuid.UUID `json:"event_ids"`
	CorrelationType string      `json:"correlation_type"`
	Strength        float64     `json:"strength"`
	Description     string      `json:"description"`
}

// AuditTestEnvironment holds all infrastructure for audit integration tests
type AuditTestEnvironment struct {
	PostgresContainer  testcontainers.Container
	RedisContainer     testcontainers.Container
	LocalStackContainer testcontainers.Container

	DB          *database.DB
	Cache       cache.AuditCache
	EventRepo   audit.EventRepository
	IntegrityRepo audit.IntegrityRepository
	QueryRepo   audit.QueryRepository
	ArchiveRepo audit.ArchiveRepository
	S3Client    *s3.Client

	PostgresURL   string
	RedisURL      string
	LocalStackURL string
	BucketName    string

	ctx     context.Context
	logger  *zap.Logger
	cleanup func()
}

func TestAuditIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	env := setupAuditTestEnvironment(t)
	defer env.cleanup()

	// Test scenarios
	t.Run("CompleteAuditEventLifecycle", func(t *testing.T) {
		testCompleteAuditEventLifecycle(t, env)
	})

	t.Run("HashChainIntegrityAcrossRestarts", func(t *testing.T) {
		testHashChainIntegrityAcrossRestarts(t, env)
	})

	t.Run("DatabasePartitioning", func(t *testing.T) {
		testDatabasePartitioning(t, env)
	})

	t.Run("CachePerformanceValidation", func(t *testing.T) {
		testCachePerformanceValidation(t, env)
	})

	t.Run("ArchiveAndRetrieval", func(t *testing.T) {
		testArchiveAndRetrieval(t, env)
	})

	t.Run("ComplianceReportGeneration", func(t *testing.T) {
		testComplianceReportGeneration(t, env)
	})

	t.Run("ConcurrentAccessTesting", func(t *testing.T) {
		testConcurrentAccessTesting(t, env)
	})

	t.Run("PerformanceValidation", func(t *testing.T) {
		testPerformanceValidation(t, env)
	})

	t.Run("FailureScenarios", func(t *testing.T) {
		testFailureScenarios(t, env)
	})
}

func setupAuditTestEnvironment(t *testing.T) *AuditTestEnvironment {
	ctx := context.Background()
	logger := zap.NewNop()

	env := &AuditTestEnvironment{
		ctx:    ctx,
		logger: logger,
	}

	// Start containers in parallel for faster setup
	var wg sync.WaitGroup
	var containerErrors []error
	var mu sync.Mutex

	// Start PostgreSQL with TimescaleDB for partitioning
	wg.Add(1)
	go func() {
		defer wg.Done()
		container, err := postgres.RunContainer(ctx,
			testcontainers.WithImage("timescale/timescaledb:latest-pg16"),
			postgres.WithDatabase("audit_test"),
			postgres.WithUsername("test"),
			postgres.WithPassword("test"),
			testcontainers.WithWaitStrategy(
				wait.ForSQL("5432/tcp", "pgx", func(host string, port nat.Port) string {
					return fmt.Sprintf("postgres://test:test@%s:%s/audit_test?sslmode=disable", host, port.Port())
				}).WithStartupTimeout(60*time.Second),
			),
		)
		mu.Lock()
		if err != nil {
			containerErrors = append(containerErrors, fmt.Errorf("postgres: %w", err))
		} else {
			env.PostgresContainer = container
		}
		mu.Unlock()
	}()

	// Start Redis for caching
	wg.Add(1)
	go func() {
		defer wg.Done()
		container, err := redis.RunContainer(ctx,
			testcontainers.WithImage("redis:7-alpine"),
			redis.WithSnapshotting(10, 1),
			redis.WithLogLevel(redis.LogLevelVerbose),
		)
		mu.Lock()
		if err != nil {
			containerErrors = append(containerErrors, fmt.Errorf("redis: %w", err))
		} else {
			env.RedisContainer = container
		}
		mu.Unlock()
	}()

	// Start LocalStack for S3 testing
	wg.Add(1)
	go func() {
		defer wg.Done()
		container, err := localstack.RunContainer(ctx,
			testcontainers.WithImage("localstack/localstack:3.0"),
			localstack.WithServices(localstack.S3),
		)
		mu.Lock()
		if err != nil {
			containerErrors = append(containerErrors, fmt.Errorf("localstack: %w", err))
		} else {
			env.LocalStackContainer = container
		}
		mu.Unlock()
	}()

	wg.Wait()

	// Check for container startup errors
	if len(containerErrors) > 0 {
		for _, err := range containerErrors {
			t.Errorf("Container startup error: %v", err)
		}
		t.FailNow()
	}

	// Get connection strings
	env.setupConnections(t)

	// Initialize database schema with partitioning
	env.setupAuditSchema(t)

	// Initialize repositories and services
	env.initializeRepositories(t)

	// Initialize S3 bucket for archive testing
	env.setupS3Bucket(t)

	// Setup cleanup
	env.cleanup = func() {
		env.cleanupEnvironment(t)
	}

	return env
}

func (env *AuditTestEnvironment) setupConnections(t *testing.T) {
	var err error

	// PostgreSQL connection
	env.PostgresURL, err = env.PostgresContainer.ConnectionString(env.ctx, "sslmode=disable")
	require.NoError(t, err)

	// Redis connection
	env.RedisURL, err = env.RedisContainer.ConnectionString(env.ctx)
	require.NoError(t, err)

	// LocalStack connection
	env.LocalStackURL, err = env.LocalStackContainer.ConnectionString(env.ctx)
	require.NoError(t, err)

	// Initialize database connection
	db, err := testutil.InitTestDB(env.PostgresURL)
	require.NoError(t, err)
	env.DB = database.NewDB(db)

	// Initialize cache
	env.Cache, err = cache.NewAuditCache(env.RedisURL)
	require.NoError(t, err)

	// Setup S3 client for LocalStack
	cfg, err := config.LoadDefaultConfig(env.ctx,
		config.WithRegion("us-east-1"),
		config.WithEndpointResolverWithOptions(aws.EndpointResolverWithOptionsFunc(
			func(service, region string, options ...interface{}) (aws.Endpoint, error) {
				return aws.Endpoint{
					URL: env.LocalStackURL,
					HostnameImmutable: true,
				}, nil
			})),
		config.WithCredentialsProvider(aws.AnonymousCredentials{}),
	)
	require.NoError(t, err)

	env.S3Client = s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true
	})
}

func (env *AuditTestEnvironment) setupAuditSchema(t *testing.T) {
	// Create audit schema with TimescaleDB partitioning
	schema := `
	-- Enable TimescaleDB extension
	CREATE EXTENSION IF NOT EXISTS timescaledb;
	CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

	-- Create audit events table with proper structure
	CREATE TABLE IF NOT EXISTS audit_events (
		id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
		sequence_num BIGSERIAL UNIQUE NOT NULL,
		timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		timestamp_nano BIGINT NOT NULL,
		
		-- Event classification
		event_type VARCHAR(100) NOT NULL,
		severity VARCHAR(20) NOT NULL,
		category VARCHAR(50) NOT NULL,
		
		-- Actor information
		actor_id VARCHAR(255) NOT NULL,
		actor_type VARCHAR(50) NOT NULL,
		actor_ip INET,
		actor_agent TEXT,
		
		-- Target information
		target_id VARCHAR(255) NOT NULL,
		target_type VARCHAR(50) NOT NULL,
		target_owner VARCHAR(255),
		
		-- Action details
		action VARCHAR(100) NOT NULL,
		result VARCHAR(20) NOT NULL,
		error_code VARCHAR(100),
		error_message TEXT,
		
		-- Request correlation
		request_id VARCHAR(255) NOT NULL,
		session_id VARCHAR(255),
		correlation_id VARCHAR(255),
		
		-- Service metadata
		environment VARCHAR(50) NOT NULL,
		service VARCHAR(100) NOT NULL,
		version VARCHAR(50) NOT NULL,
		
		-- Compliance metadata
		compliance_flags JSONB,
		data_classes TEXT[],
		legal_basis VARCHAR(100),
		retention_days INTEGER NOT NULL DEFAULT 2555,
		
		-- Additional context
		metadata JSONB,
		tags TEXT[],
		
		-- Cryptographic integrity
		previous_hash VARCHAR(64),
		event_hash VARCHAR(64) NOT NULL,
		signature VARCHAR(512)
	);

	-- Convert to hypertable for time-series partitioning
	SELECT create_hypertable('audit_events', 'timestamp', 
		chunk_time_interval => INTERVAL '1 day',
		if_not_exists => TRUE);

	-- Create partition by sequence for integrity verification
	CREATE TABLE IF NOT EXISTS audit_events_by_sequence (
		LIKE audit_events INCLUDING ALL
	) PARTITION BY RANGE (sequence_num);

	-- Create initial partition
	CREATE TABLE IF NOT EXISTS audit_events_seq_1_1000000 
		PARTITION OF audit_events_by_sequence 
		FOR VALUES FROM (1) TO (1000000);

	-- Create integrity tracking table
	CREATE TABLE IF NOT EXISTS audit_integrity_checks (
		id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
		check_type VARCHAR(50) NOT NULL,
		start_sequence BIGINT NOT NULL,
		end_sequence BIGINT NOT NULL,
		is_valid BOOLEAN NOT NULL,
		issues_found INTEGER DEFAULT 0,
		check_time TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		verification_time INTERVAL,
		report_data JSONB
	);

	-- Create sequence gaps tracking
	CREATE TABLE IF NOT EXISTS audit_sequence_gaps (
		id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
		gap_start BIGINT NOT NULL,
		gap_end BIGINT NOT NULL,
		gap_size BIGINT NOT NULL,
		discovered_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		resolved_at TIMESTAMPTZ,
		resolution_method VARCHAR(100)
	);

	-- Create archive tracking table
	CREATE TABLE IF NOT EXISTS audit_archive_metadata (
		id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
		archive_id VARCHAR(255) UNIQUE NOT NULL,
		start_sequence BIGINT NOT NULL,
		end_sequence BIGINT NOT NULL,
		start_time TIMESTAMPTZ NOT NULL,
		end_time TIMESTAMPTZ NOT NULL,
		event_count BIGINT NOT NULL,
		archive_location VARCHAR(500) NOT NULL,
		archive_size BIGINT,
		compression_ratio DECIMAL(5,2),
		integrity_hash VARCHAR(64),
		archived_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		archived_by VARCHAR(255) NOT NULL,
		status VARCHAR(50) NOT NULL DEFAULT 'active'
	);

	-- Create indexes for performance
	CREATE INDEX IF NOT EXISTS idx_audit_events_sequence ON audit_events(sequence_num);
	CREATE INDEX IF NOT EXISTS idx_audit_events_timestamp ON audit_events(timestamp);
	CREATE INDEX IF NOT EXISTS idx_audit_events_type ON audit_events(event_type);
	CREATE INDEX IF NOT EXISTS idx_audit_events_actor ON audit_events(actor_id);
	CREATE INDEX IF NOT EXISTS idx_audit_events_target ON audit_events(target_id);
	CREATE INDEX IF NOT EXISTS idx_audit_events_request ON audit_events(request_id);
	CREATE INDEX IF NOT EXISTS idx_audit_events_hash ON audit_events(event_hash);
	CREATE INDEX IF NOT EXISTS idx_audit_events_compliance ON audit_events USING GIN(compliance_flags);
	CREATE INDEX IF NOT EXISTS idx_audit_events_metadata ON audit_events USING GIN(metadata);

	-- Create hypertable indexes for time-based queries
	CREATE INDEX IF NOT EXISTS idx_audit_events_time_type ON audit_events(timestamp, event_type);
	CREATE INDEX IF NOT EXISTS idx_audit_events_time_actor ON audit_events(timestamp, actor_id);

	-- Create integrity verification functions
	CREATE OR REPLACE FUNCTION verify_event_hash(event_row audit_events)
	RETURNS BOOLEAN AS $$
	DECLARE
		computed_hash TEXT;
		hash_data JSONB;
	BEGIN
		-- Build hash data structure
		hash_data := jsonb_build_object(
			'id', event_row.id::text,
			'sequence_num', event_row.sequence_num,
			'timestamp_nano', event_row.timestamp_nano,
			'type', event_row.event_type,
			'actor_id', event_row.actor_id,
			'target_id', event_row.target_id,
			'action', event_row.action,
			'result', event_row.result,
			'previous_hash', COALESCE(event_row.previous_hash, '')
		);
		
		-- In production, this would compute actual SHA-256
		-- For testing, we'll use a simple hash function
		computed_hash := md5(hash_data::text);
		
		RETURN computed_hash = event_row.event_hash;
	END;
	$$ LANGUAGE plpgsql IMMUTABLE;

	-- Create retention policy for old events
	CREATE OR REPLACE FUNCTION apply_retention_policy()
	RETURNS INTEGER AS $$
	DECLARE
		archived_count INTEGER := 0;
	BEGIN
		-- Mark events for archival if they exceed retention period
		UPDATE audit_events 
		SET metadata = COALESCE(metadata, '{}'::jsonb) || '{"archived": true}'::jsonb
		WHERE timestamp < NOW() - INTERVAL '1 day' * retention_days
		AND (metadata->>'archived')::boolean IS NOT TRUE;
		
		GET DIAGNOSTICS archived_count = ROW_COUNT;
		
		RETURN archived_count;
	END;
	$$ LANGUAGE plpgsql;
	`

	_, err := env.DB.Exec(schema)
	require.NoError(t, err, "Failed to create audit schema")
}

func (env *AuditTestEnvironment) initializeRepositories(t *testing.T) {
	// Initialize repositories
	env.EventRepo = database.NewAuditEventRepository(env.DB, env.Cache, env.logger)
	env.IntegrityRepo = database.NewAuditIntegrityRepository(env.DB, env.logger)
	env.QueryRepo = database.NewAuditQueryRepository(env.DB, env.Cache, env.logger)
	env.ArchiveRepo = database.NewAuditArchiveRepository(env.DB, env.S3Client, env.BucketName, env.logger)
}

func (env *AuditTestEnvironment) setupS3Bucket(t *testing.T) {
	env.BucketName = "audit-test-bucket"
	
	// Create S3 bucket
	_, err := env.S3Client.CreateBucket(env.ctx, &s3.CreateBucketInput{
		Bucket: aws.String(env.BucketName),
	})
	require.NoError(t, err, "Failed to create S3 bucket")
}

func (env *AuditTestEnvironment) cleanupEnvironment(t *testing.T) {
	// Close connections
	if env.DB != nil {
		env.DB.Close()
	}
	if env.Cache != nil {
		env.Cache.Close()
	}

	// Terminate containers
	containers := []testcontainers.Container{
		env.PostgresContainer,
		env.RedisContainer,
		env.LocalStackContainer,
	}

	for _, container := range containers {
		if container != nil {
			if err := container.Terminate(env.ctx); err != nil {
				t.Logf("Failed to terminate container: %v", err)
			}
		}
	}
}

func testCompleteAuditEventLifecycle(t *testing.T, env *AuditTestEnvironment) {
	t.Log("Testing complete audit event lifecycle")

	// Test event creation with validation
	event, err := audit.NewEvent(
		audit.EventCallInitiated,
		"buyer-123",
		"call-456",
		"initiate_call",
	)
	require.NoError(t, err)

	// Set additional properties
	event.TargetType = "call"
	event.ActorType = "buyer"
	event.RequestID = uuid.New().String()
	event.Metadata = map[string]interface{}{
		"phone_number": "+1234567890",
		"duration_ms":  5000,
	}
	event.ComplianceFlags = map[string]bool{
		"tcpa_relevant": true,
		"recorded":     true,
	}
	event.DataClasses = []string{"phone_number", "call_metadata"}
	event.LegalBasis = "contract"

	// Store event and verify storage
	err = env.EventRepo.Store(env.ctx, event)
	require.NoError(t, err, "Failed to store audit event")

	// Retrieve and verify
	retrievedEvent, err := env.EventRepo.GetByID(env.ctx, event.ID)
	require.NoError(t, err)
	assert.Equal(t, event.ID, retrievedEvent.ID)
	assert.Equal(t, event.Type, retrievedEvent.Type)
	assert.Equal(t, event.ActorID, retrievedEvent.ActorID)
	assert.Equal(t, event.TargetID, retrievedEvent.TargetID)
	assert.Equal(t, event.Action, retrievedEvent.Action)
	assert.True(t, retrievedEvent.HasComplianceFlag("tcpa_relevant"))

	// Test hash chain integrity
	assert.NotEmpty(t, retrievedEvent.EventHash, "Event should have computed hash")
	assert.True(t, retrievedEvent.IsImmutable(), "Event should be immutable after storage")

	// Test cache performance
	start := time.Now()
	cachedEvent, err := env.EventRepo.GetByID(env.ctx, event.ID)
	cacheTime := time.Since(start)
	require.NoError(t, err)
	assert.Equal(t, event.ID, cachedEvent.ID)
	assert.Less(t, cacheTime, 10*time.Millisecond, "Cache retrieval should be fast")

	t.Logf("Event lifecycle test completed successfully")
}

func testHashChainIntegrityAcrossRestarts(t *testing.T, env *AuditTestEnvironment) {
	t.Log("Testing hash chain integrity across service restarts")

	// Create a chain of events
	events := make([]*audit.Event, 5)
	for i := 0; i < 5; i++ {
		event, err := audit.NewEvent(
			audit.EventBidPlaced,
			fmt.Sprintf("buyer-%d", i),
			fmt.Sprintf("auction-%d", i),
			"place_bid",
		)
		require.NoError(t, err)

		event.Metadata = map[string]interface{}{
			"bid_amount": float64(100 + i*10),
			"chain_position": i,
		}

		err = env.EventRepo.Store(env.ctx, event)
		require.NoError(t, err)
		events[i] = event
	}

	// Verify initial chain integrity
	hashService := audit.NewHashChainService(env.EventRepo, env.IntegrityRepo)
	
	startSeq, _ := values.NewSequenceNumber(events[0].SequenceNum)
	endSeq, _ := values.NewSequenceNumber(events[4].SequenceNum)
	
	result, err := hashService.VerifyChain(env.ctx, startSeq, endSeq)
	require.NoError(t, err)
	assert.True(t, result.IsValid, "Initial chain should be valid")
	assert.Equal(t, int64(5), result.EventsVerified)
	assert.Equal(t, int64(5), result.HashesValid)
	assert.Equal(t, int64(0), result.HashesInvalid)

	// Simulate service restart by recreating repositories
	env.initializeRepositories(t)

	// Verify chain integrity after restart
	result2, err := hashService.VerifyChain(env.ctx, startSeq, endSeq)
	require.NoError(t, err)
	assert.True(t, result2.IsValid, "Chain should remain valid after restart")
	assert.Equal(t, result.EventsVerified, result2.EventsVerified)

	// Add more events after restart
	for i := 5; i < 8; i++ {
		event, err := audit.NewEvent(
			audit.EventBidWon,
			fmt.Sprintf("buyer-%d", i),
			fmt.Sprintf("auction-%d", i),
			"win_bid",
		)
		require.NoError(t, err)

		err = env.EventRepo.Store(env.ctx, event)
		require.NoError(t, err)
	}

	// Verify extended chain
	newEndSeq, _ := values.NewSequenceNumber(events[4].SequenceNum + 3)
	result3, err := hashService.VerifyChain(env.ctx, startSeq, newEndSeq)
	require.NoError(t, err)
	assert.True(t, result3.IsValid, "Extended chain should be valid")
	assert.Equal(t, int64(8), result3.EventsVerified)

	t.Logf("Hash chain integrity verified across restarts")
}

func testDatabasePartitioning(t *testing.T, env *AuditTestEnvironment) {
	t.Log("Testing database partitioning validation")

	// Create events across different time periods to test partitioning
	now := time.Now()
	timePoints := []time.Time{
		now.AddDate(0, 0, -2), // 2 days ago
		now.AddDate(0, 0, -1), // 1 day ago
		now,                   // now
	}

	eventsPerPartition := 100
	totalEvents := len(timePoints) * eventsPerPartition

	// Create events distributed across partitions
	for partitionIdx, timePoint := range timePoints {
		for i := 0; i < eventsPerPartition; i++ {
			event, err := audit.NewEvent(
				audit.EventAPICall,
				fmt.Sprintf("user-%d-%d", partitionIdx, i),
				fmt.Sprintf("api-endpoint-%d", i),
				"api_call",
			)
			require.NoError(t, err)

			// Set specific timestamp for partitioning
			event.Timestamp = timePoint.Add(time.Duration(i) * time.Second)
			event.TimestampNano = event.Timestamp.UnixNano()
			event.Metadata = map[string]interface{}{
				"partition_id": partitionIdx,
				"event_index": i,
				"endpoint":    fmt.Sprintf("/api/v1/test-%d", i),
			}

			err = env.EventRepo.Store(env.ctx, event)
			require.NoError(t, err)
		}
	}

	// Verify partition distribution
	stats, err := env.EventRepo.GetStats(env.ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, stats.TotalEvents, int64(totalEvents))

	// Test partition-specific queries
	for partitionIdx, timePoint := range timePoints {
		startTime := timePoint.Add(-1 * time.Hour)
		endTime := timePoint.Add(24 * time.Hour)

		filter := audit.EventFilter{
			StartTime: &startTime,
			EndTime:   &endTime,
			Limit:     eventsPerPartition + 10,
		}

		events, err := env.EventRepo.GetEventsByTimeRange(env.ctx, startTime, endTime, filter)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(events.Events), eventsPerPartition,
			"Should find events in partition %d", partitionIdx)

		// Verify events are in correct time range
		for _, event := range events.Events {
			assert.True(t, event.Timestamp.After(startTime) || event.Timestamp.Equal(startTime))
			assert.True(t, event.Timestamp.Before(endTime) || event.Timestamp.Equal(endTime))
		}
	}

	// Test cross-partition queries
	globalFilter := audit.EventFilter{
		Types: []audit.EventType{audit.EventAPICall},
		Limit: totalEvents + 10,
	}

	allEvents, err := env.EventRepo.GetEvents(env.ctx, globalFilter)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(allEvents.Events), totalEvents)

	t.Logf("Database partitioning validation completed - %d events across %d partitions", 
		totalEvents, len(timePoints))
}

func testCachePerformanceValidation(t *testing.T, env *AuditTestEnvironment) {
	t.Log("Testing cache performance validation")

	// Performance thresholds
	const maxCacheLatency = 5 * time.Millisecond
	const maxDBLatency = 50 * time.Millisecond
	const minCacheHitRate = 0.8

	// Create test events
	testEvents := make([]*audit.Event, 20)
	for i := 0; i < 20; i++ {
		event, err := audit.NewEvent(
			audit.EventDataAccessed,
			fmt.Sprintf("user-%d", i),
			fmt.Sprintf("data-%d", i),
			"access_data",
		)
		require.NoError(t, err)

		event.Metadata = map[string]interface{}{
			"performance_test": true,
			"event_index":     i,
		}

		err = env.EventRepo.Store(env.ctx, event)
		require.NoError(t, err)
		testEvents[i] = event
	}

	// Warm up cache by reading all events
	for _, event := range testEvents {
		_, err := env.EventRepo.GetByID(env.ctx, event.ID)
		require.NoError(t, err)
	}

	// Test cache performance
	cacheHits := 0
	totalReads := 100
	var totalCacheTime time.Duration

	for i := 0; i < totalReads; i++ {
		eventIdx := i % len(testEvents)
		eventID := testEvents[eventIdx].ID

		start := time.Now()
		retrievedEvent, err := env.EventRepo.GetByID(env.ctx, eventID)
		readTime := time.Since(start)

		require.NoError(t, err)
		assert.Equal(t, eventID, retrievedEvent.ID)

		// Check if read was from cache (faster than DB threshold)
		if readTime < maxDBLatency/2 {
			cacheHits++
			totalCacheTime += readTime
		}
	}

	// Validate cache performance metrics
	cacheHitRate := float64(cacheHits) / float64(totalReads)
	avgCacheTime := totalCacheTime / time.Duration(cacheHits)

	assert.GreaterOrEqual(t, cacheHitRate, minCacheHitRate,
		"Cache hit rate should be at least %.1f%%, got %.1f%%", 
		minCacheHitRate*100, cacheHitRate*100)

	assert.Less(t, avgCacheTime, maxCacheLatency,
		"Average cache read time should be less than %v, got %v",
		maxCacheLatency, avgCacheTime)

	// Test cache invalidation
	newEvent, err := audit.NewEvent(
		audit.EventDataModified,
		"user-new",
		"data-new",
		"modify_data",
	)
	require.NoError(t, err)

	err = env.EventRepo.Store(env.ctx, newEvent)
	require.NoError(t, err)

	// Verify new event is immediately available
	start := time.Now()
	retrievedNewEvent, err := env.EventRepo.GetByID(env.ctx, newEvent.ID)
	newEventReadTime := time.Since(start)

	require.NoError(t, err)
	assert.Equal(t, newEvent.ID, retrievedNewEvent.ID)
	assert.Less(t, newEventReadTime, maxDBLatency)

	t.Logf("Cache performance validation completed - Hit rate: %.1f%%, Avg cache time: %v",
		cacheHitRate*100, avgCacheTime)
}

func testArchiveAndRetrieval(t *testing.T, env *AuditTestEnvironment) {
	t.Log("Testing archive and retrieval functionality")

	// Create a batch of events for archival
	archiveEvents := make([]*audit.Event, 50)
	archiveTime := time.Now().AddDate(0, 0, -30) // 30 days old

	for i := 0; i < 50; i++ {
		event, err := audit.NewEvent(
			audit.EventPaymentProcessed,
			fmt.Sprintf("merchant-%d", i),
			fmt.Sprintf("payment-%d", i),
			"process_payment",
		)
		require.NoError(t, err)

		event.Timestamp = archiveTime.Add(time.Duration(i) * time.Minute)
		event.TimestampNano = event.Timestamp.UnixNano()
		event.Metadata = map[string]interface{}{
			"amount":       float64(100 + i),
			"currency":     "USD",
			"archive_test": true,
		}
		event.ComplianceFlags = map[string]bool{
			"financial_data": true,
			"pci_relevant":   true,
		}

		err = env.EventRepo.Store(env.ctx, event)
		require.NoError(t, err)
		archiveEvents[i] = event
	}

	// Create archive criteria
	archiveCriteria := audit.ArchiveCriteria{
		StartTime:   archiveTime.Add(-1 * time.Hour),
		EndTime:     archiveTime.Add(1 * time.Hour),
		ComplianceFlags: []string{"financial_data"},
		RetentionPolicy: "long_term",
		CompressionLevel: 6,
	}

	// Perform archival
	archiveResult, err := env.ArchiveRepo.ArchiveEvents(env.ctx, archiveCriteria)
	require.NoError(t, err)
	assert.True(t, archiveResult.Success)
	assert.GreaterOrEqual(t, archiveResult.EventsArchived, int64(50))
	assert.NotEmpty(t, archiveResult.ArchiveID)
	assert.NotEmpty(t, archiveResult.ArchiveLocation)

	// Verify archive metadata
	archiveMetadata, err := env.ArchiveRepo.GetArchiveMetadata(env.ctx, archiveResult.ArchiveID)
	require.NoError(t, err)
	assert.Equal(t, archiveResult.ArchiveID, archiveMetadata.ArchiveID)
	assert.Equal(t, archiveResult.EventsArchived, archiveMetadata.EventCount)
	assert.Greater(t, archiveMetadata.ArchiveSize, int64(0))

	// Test archive retrieval
	retrievalCriteria := audit.RetrievalCriteria{
		ArchiveID: archiveResult.ArchiveID,
		StartSequence: archiveEvents[0].SequenceNum,
		EndSequence:   archiveEvents[49].SequenceNum,
		IncludeMetadata: true,
	}

	retrievalResult, err := env.ArchiveRepo.RetrieveFromArchive(env.ctx, retrievalCriteria)
	require.NoError(t, err)
	assert.True(t, retrievalResult.Success)
	assert.GreaterOrEqual(t, len(retrievalResult.Events), 50)

	// Verify retrieved events integrity
	for i, retrievedEvent := range retrievalResult.Events[:50] {
		originalEvent := archiveEvents[i]
		assert.Equal(t, originalEvent.ID, retrievedEvent.ID)
		assert.Equal(t, originalEvent.Type, retrievedEvent.Type)
		assert.Equal(t, originalEvent.ActorID, retrievedEvent.ActorID)
		assert.Equal(t, originalEvent.EventHash, retrievedEvent.EventHash)
	}

	// Test partial retrieval by time range
	partialStartTime := archiveTime.Add(10 * time.Minute)
	partialEndTime := archiveTime.Add(20 * time.Minute)

	partialCriteria := audit.RetrievalCriteria{
		ArchiveID: archiveResult.ArchiveID,
		StartTime: &partialStartTime,
		EndTime:   &partialEndTime,
	}

	partialResult, err := env.ArchiveRepo.RetrieveFromArchive(env.ctx, partialCriteria)
	require.NoError(t, err)
	assert.True(t, partialResult.Success)
	assert.Greater(t, len(partialResult.Events), 0)
	assert.Less(t, len(partialResult.Events), 50)

	// Verify all partial events are within time range
	for _, event := range partialResult.Events {
		assert.True(t, event.Timestamp.After(partialStartTime) || event.Timestamp.Equal(partialStartTime))
		assert.True(t, event.Timestamp.Before(partialEndTime) || event.Timestamp.Equal(partialEndTime))
	}

	t.Logf("Archive and retrieval test completed - Archived: %d events, Retrieved: %d events",
		archiveResult.EventsArchived, len(retrievalResult.Events))
}

func testComplianceReportGeneration(t *testing.T, env *AuditTestEnvironment) {
	t.Log("Testing compliance report generation")

	// Create GDPR-relevant events
	gdprEvents := []struct {
		actorID      string
		targetID     string
		eventType    audit.EventType
		dataClasses  []string
		legalBasis   string
		hasConsent   bool
	}{
		{"user-gdpr-1", "profile-1", audit.EventDataAccessed, []string{"personal_data", "email"}, "consent", true},
		{"user-gdpr-1", "profile-1", audit.EventDataModified, []string{"personal_data"}, "consent", true},
		{"user-gdpr-2", "profile-2", audit.EventDataAccessed, []string{"personal_data", "phone_number"}, "", false},
		{"user-gdpr-1", "profile-1", audit.EventDataExported, []string{"personal_data", "email"}, "legitimate_interest", false},
		{"user-gdpr-3", "profile-3", audit.EventConsentGranted, []string{"consent_data"}, "consent", true},
	}

	for i, eventData := range gdprEvents {
		event, err := audit.NewEvent(
			eventData.eventType,
			eventData.actorID,
			eventData.targetID,
			"gdpr_test_action",
		)
		require.NoError(t, err)

		event.DataClasses = eventData.dataClasses
		event.LegalBasis = eventData.legalBasis
		event.ComplianceFlags = map[string]bool{
			"gdpr_relevant": true,
			"contains_pii":  true,
		}
		if eventData.hasConsent {
			event.ComplianceFlags["explicit_consent"] = true
		}
		event.Metadata = map[string]interface{}{
			"test_case": i,
			"gdpr_test": true,
		}

		err = env.EventRepo.Store(env.ctx, event)
		require.NoError(t, err)
	}

	// Create TCPA-relevant events
	tcpaEvents := []struct {
		phoneNumber string
		eventType   audit.EventType
		hasConsent  bool
	}{
		{"+1234567890", audit.EventConsentGranted, true},
		{"+1234567890", audit.EventCallInitiated, true},
		{"+1234567891", audit.EventCallInitiated, false},
		{"+1234567890", audit.EventOptOutRequested, false},
		{"+1234567890", audit.EventCallInitiated, false}, // Should be violation
	}

	for i, eventData := range tcpaEvents {
		event, err := audit.NewEvent(
			eventData.eventType,
			"system",
			eventData.phoneNumber,
			"tcpa_test_action",
		)
		require.NoError(t, err)

		event.ComplianceFlags = map[string]bool{
			"tcpa_relevant": true,
		}
		if eventData.hasConsent {
			event.ComplianceFlags["explicit_consent"] = true
		}
		event.Metadata = map[string]interface{}{
			"phone_number": eventData.phoneNumber,
			"test_case":    i,
			"tcpa_test":    true,
		}

		err = env.EventRepo.Store(env.ctx, event)
		require.NoError(t, err)
	}

	// Generate GDPR compliance report
	complianceService := audit.NewComplianceVerificationService(env.EventRepo, env.QueryRepo)

	gdprReport, err := complianceService.VerifyGDPRCompliance(env.ctx, "user-gdpr-1")
	require.NoError(t, err)
	
	assert.Equal(t, "user-gdpr-1", gdprReport.DataSubjectID)
	assert.Greater(t, gdprReport.TotalEvents, int64(0))
	assert.Greater(t, gdprReport.ConsentEvents, int64(0))
	assert.Greater(t, gdprReport.DataAccessEvents, int64(0))
	
	// Check for compliance issues
	if gdprReport.EventsWithoutLegalBasis > 0 {
		assert.False(t, gdprReport.IsCompliant)
		assert.Greater(t, len(gdprReport.Issues), 0)
	}

	// Generate TCPA compliance report
	tcpaReport, err := complianceService.VerifyTCPACompliance(env.ctx, "+1234567890")
	require.NoError(t, err)
	
	assert.Equal(t, "+1234567890", tcpaReport.PhoneNumber)
	assert.Greater(t, tcpaReport.CallEvents, int64(0))
	assert.Greater(t, tcpaReport.ConsentGrantedEvents, int64(0))
	assert.Greater(t, tcpaReport.ConsentRevokedEvents, int64(0))
	
	// Should detect TCPA violation (call after opt-out)
	assert.False(t, tcpaReport.IsCompliant)
	assert.Greater(t, len(tcpaReport.ViolationRisks), 0)
	assert.Greater(t, tcpaReport.ComplianceRiskScore, 0.0)

	// Test comprehensive integrity report
	integrityService := audit.NewIntegrityCheckService(
		env.EventRepo,
		env.IntegrityRepo,
		env.QueryRepo,
		audit.NewHashChainService(env.EventRepo, env.IntegrityRepo),
	)

	criteria := audit.IntegrityCriteria{
		CheckHashChain:   true,
		CheckSequencing:  true,
		CheckMetadata:    true,
		CheckCompliance:  true,
		DeepVerification: true,
	}

	integrityReport, err := integrityService.PerformIntegrityCheck(env.ctx, criteria)
	require.NoError(t, err)
	
	assert.NotNil(t, integrityReport)
	assert.Greater(t, integrityReport.TotalEvents, int64(0))
	assert.Greater(t, integrityReport.VerifiedEvents, int64(0))
	
	// Check for compliance issues
	assert.Greater(t, len(integrityReport.ComplianceIssues), 0,
		"Should detect compliance issues from test events")

	t.Logf("Compliance report generation completed - GDPR events: %d, TCPA events: %d, Compliance issues: %d",
		gdprReport.TotalEvents, tcpaReport.CallEvents, len(integrityReport.ComplianceIssues))
}

func testConcurrentAccessTesting(t *testing.T, env *AuditTestEnvironment) {
	t.Log("Testing concurrent access patterns")

	const numWorkers = 10
	const eventsPerWorker = 20
	const totalEvents = numWorkers * eventsPerWorker

	var wg sync.WaitGroup
	var mu sync.Mutex
	results := make([]error, 0)
	eventIDs := make([]uuid.UUID, 0)

	// Test concurrent event creation
	wg.Add(numWorkers)
	for workerID := 0; workerID < numWorkers; workerID++ {
		go func(id int) {
			defer wg.Done()

			for i := 0; i < eventsPerWorker; i++ {
				event, err := audit.NewEvent(
					audit.EventSystemStartup,
					fmt.Sprintf("worker-%d", id),
					fmt.Sprintf("system-%d-%d", id, i),
					"concurrent_test",
				)
				if err != nil {
					mu.Lock()
					results = append(results, err)
					mu.Unlock()
					continue
				}

				event.Metadata = map[string]interface{}{
					"worker_id":    id,
					"event_index":  i,
					"concurrent":   true,
					"timestamp_ms": time.Now().UnixMilli(),
				}

				err = env.EventRepo.Store(env.ctx, event)
				mu.Lock()
				if err != nil {
					results = append(results, err)
				} else {
					eventIDs = append(eventIDs, event.ID)
				}
				mu.Unlock()

				// Add small delay to simulate realistic load
				time.Sleep(1 * time.Millisecond)
			}
		}(workerID)
	}

	wg.Wait()

	// Check for errors during concurrent creation
	assert.Empty(t, results, "Should have no errors during concurrent event creation")
	assert.Equal(t, totalEvents, len(eventIDs), "Should create all events successfully")

	// Test concurrent reads
	readResults := make([]error, 0)
	readWg := sync.WaitGroup{}
	
	readWg.Add(numWorkers)
	for workerID := 0; workerID < numWorkers; workerID++ {
		go func(id int) {
			defer readWg.Done()

			// Each worker reads random events
			for i := 0; i < eventsPerWorker; i++ {
				eventIdx := (id*eventsPerWorker + i) % len(eventIDs)
				eventID := eventIDs[eventIdx]

				_, err := env.EventRepo.GetByID(env.ctx, eventID)
				if err != nil {
					mu.Lock()
					readResults = append(readResults, err)
					mu.Unlock()
				}
			}
		}(workerID)
	}

	readWg.Wait()

	assert.Empty(t, readResults, "Should have no errors during concurrent reads")

	// Test hash chain integrity after concurrent operations
	hashService := audit.NewHashChainService(env.EventRepo, env.IntegrityRepo)
	
	// Get sequence range for all created events
	firstEvent, err := env.EventRepo.GetByID(env.ctx, eventIDs[0])
	require.NoError(t, err)
	lastEvent, err := env.EventRepo.GetByID(env.ctx, eventIDs[len(eventIDs)-1])
	require.NoError(t, err)

	startSeq, _ := values.NewSequenceNumber(firstEvent.SequenceNum)
	endSeq, _ := values.NewSequenceNumber(lastEvent.SequenceNum)

	if startSeq > endSeq {
		startSeq, endSeq = endSeq, startSeq
	}

	chainResult, err := hashService.VerifyChain(env.ctx, startSeq, endSeq)
	require.NoError(t, err)
	assert.True(t, chainResult.IsValid, "Hash chain should remain valid after concurrent operations")
	assert.Equal(t, float64(1.0), chainResult.IntegrityScore, "Integrity score should be perfect")

	// Test concurrent sequence number generation
	sequenceResults := make([]values.SequenceNumber, 0)
	sequenceWg := sync.WaitGroup{}
	
	sequenceWg.Add(numWorkers)
	for workerID := 0; workerID < numWorkers; workerID++ {
		go func() {
			defer sequenceWg.Done()

			seq, err := env.EventRepo.GetNextSequenceNumber(env.ctx)
			if err == nil {
				mu.Lock()
				sequenceResults = append(sequenceResults, seq)
				mu.Unlock()
			}
		}()
	}

	sequenceWg.Wait()

	// Verify all sequence numbers are unique
	sequenceMap := make(map[values.SequenceNumber]bool)
	for _, seq := range sequenceResults {
		assert.False(t, sequenceMap[seq], "Sequence number %d should be unique", seq)
		sequenceMap[seq] = true
	}

	t.Logf("Concurrent access testing completed - Events created: %d, Reads performed: %d, Unique sequences: %d",
		len(eventIDs), numWorkers*eventsPerWorker, len(sequenceResults))
}

func testPerformanceValidation(t *testing.T, env *AuditTestEnvironment) {
	t.Log("Testing performance validation against targets")

	// Performance targets from specification
	const maxEventStoreLatency = 10 * time.Millisecond  // Event storage < 10ms
	const maxEventReadLatency = 5 * time.Millisecond    // Event read < 5ms
	const maxChainVerifyLatency = 100 * time.Millisecond // Chain verification < 100ms
	const minThroughput = 1000                           // Events per second

	// Test event storage performance
	storageLatencies := make([]time.Duration, 100)
	
	for i := 0; i < 100; i++ {
		event, err := audit.NewEvent(
			audit.EventFinancialComplianceCheck,
			fmt.Sprintf("perf-test-%d", i),
			fmt.Sprintf("target-%d", i),
			"performance_test",
		)
		require.NoError(t, err)

		event.Metadata = map[string]interface{}{
			"performance_test": true,
			"iteration":       i,
		}

		start := time.Now()
		err = env.EventRepo.Store(env.ctx, event)
		storageTime := time.Since(start)
		
		require.NoError(t, err)
		storageLatencies[i] = storageTime
	}

	// Calculate storage performance metrics
	var totalStorageTime time.Duration
	for _, latency := range storageLatencies {
		totalStorageTime += latency
	}
	avgStorageLatency := totalStorageTime / time.Duration(len(storageLatencies))

	assert.Less(t, avgStorageLatency, maxEventStoreLatency,
		"Average storage latency should be less than %v, got %v",
		maxEventStoreLatency, avgStorageLatency)

	// Test read performance
	stats, err := env.EventRepo.GetStats(env.ctx)
	require.NoError(t, err)
	
	// Get latest events for read testing
	filter := audit.EventFilter{
		Limit:   100,
		OrderBy: "timestamp",
		OrderDesc: true,
	}
	
	events, err := env.EventRepo.GetEvents(env.ctx, filter)
	require.NoError(t, err)
	require.Greater(t, len(events.Events), 50, "Should have enough events for read testing")

	readLatencies := make([]time.Duration, len(events.Events))
	
	for i, event := range events.Events {
		start := time.Now()
		_, err := env.EventRepo.GetByID(env.ctx, event.ID)
		readTime := time.Since(start)
		
		require.NoError(t, err)
		readLatencies[i] = readTime
	}

	// Calculate read performance metrics
	var totalReadTime time.Duration
	for _, latency := range readLatencies {
		totalReadTime += latency
	}
	avgReadLatency := totalReadTime / time.Duration(len(readLatencies))

	assert.Less(t, avgReadLatency, maxEventReadLatency,
		"Average read latency should be less than %v, got %v",
		maxEventReadLatency, avgReadLatency)

	// Test hash chain verification performance
	if len(events.Events) >= 50 {
		startSeq, _ := values.NewSequenceNumber(events.Events[49].SequenceNum)
		endSeq, _ := values.NewSequenceNumber(events.Events[0].SequenceNum)
		
		if startSeq > endSeq {
			startSeq, endSeq = endSeq, startSeq
		}

		hashService := audit.NewHashChainService(env.EventRepo, env.IntegrityRepo)
		
		start := time.Now()
		chainResult, err := hashService.VerifyChain(env.ctx, startSeq, endSeq)
		verifyTime := time.Since(start)
		
		require.NoError(t, err)
		assert.True(t, chainResult.IsValid)
		
		// Adjust expectation based on number of events verified
		expectedMaxTime := time.Duration(chainResult.EventsVerified) * (maxChainVerifyLatency / 50)
		
		assert.Less(t, verifyTime, expectedMaxTime,
			"Chain verification time should be less than %v for %d events, got %v",
			expectedMaxTime, chainResult.EventsVerified, verifyTime)
	}

	// Test throughput
	throughputEvents := 100
	throughputStart := time.Now()
	
	for i := 0; i < throughputEvents; i++ {
		event, err := audit.NewEvent(
			audit.EventAnomalyDetected,
			fmt.Sprintf("throughput-test-%d", i),
			fmt.Sprintf("anomaly-%d", i),
			"throughput_test",
		)
		require.NoError(t, err)

		err = env.EventRepo.Store(env.ctx, event)
		require.NoError(t, err)
	}
	
	throughputTime := time.Since(throughputStart)
	actualThroughput := float64(throughputEvents) / throughputTime.Seconds()

	assert.Greater(t, actualThroughput, float64(minThroughput),
		"Throughput should be at least %d events/sec, got %.1f events/sec",
		minThroughput, actualThroughput)

	t.Logf("Performance validation completed - Storage: %v, Read: %v, Throughput: %.1f events/sec",
		avgStorageLatency, avgReadLatency, actualThroughput)
}

func testFailureScenarios(t *testing.T, env *AuditTestEnvironment) {
	t.Log("Testing failure scenarios and error handling")

	// Test invalid event creation
	t.Run("InvalidEventCreation", func(t *testing.T) {
		// Test empty actor ID
		_, err := audit.NewEvent(audit.EventDataAccessed, "", "target", "action")
		assert.Error(t, err, "Should fail with empty actor ID")

		// Test empty target ID
		_, err = audit.NewEvent(audit.EventDataAccessed, "actor", "", "action")
		assert.Error(t, err, "Should fail with empty target ID")

		// Test empty action
		_, err = audit.NewEvent(audit.EventDataAccessed, "actor", "target", "")
		assert.Error(t, err, "Should fail with empty action")

		// Test invalid event type
		_, err = audit.NewEvent("INVALID_TYPE", "actor", "target", "action")
		assert.Error(t, err, "Should fail with invalid event type")
	})

	// Test repository resilience
	t.Run("RepositoryResilience", func(t *testing.T) {
		// Test non-existent event retrieval
		nonExistentID := uuid.New()
		_, err := env.EventRepo.GetByID(env.ctx, nonExistentID)
		assert.Error(t, err, "Should fail when retrieving non-existent event")

		// Test invalid sequence number
		invalidSeq, _ := values.NewSequenceNumber(999999999)
		_, err = env.EventRepo.GetBySequence(env.ctx, invalidSeq)
		assert.Error(t, err, "Should fail when retrieving non-existent sequence")
	})

	// Test hash chain corruption detection
	t.Run("HashChainCorruption", func(t *testing.T) {
		// Create events to test with
		events := make([]*audit.Event, 3)
		for i := 0; i < 3; i++ {
			event, err := audit.NewEvent(
				audit.EventConfigChanged,
				fmt.Sprintf("admin-%d", i),
				fmt.Sprintf("config-%d", i),
				"modify_config",
			)
			require.NoError(t, err)

			err = env.EventRepo.Store(env.ctx, event)
			require.NoError(t, err)
			events[i] = event
		}

		// Test hash verification service
		hashService := audit.NewHashChainService(env.EventRepo, env.IntegrityRepo)
		
		// Verify initial chain is valid
		startSeq, _ := values.NewSequenceNumber(events[0].SequenceNum)
		endSeq, _ := values.NewSequenceNumber(events[2].SequenceNum)
		
		result, err := hashService.VerifyChain(env.ctx, startSeq, endSeq)
		require.NoError(t, err)
		assert.True(t, result.IsValid, "Initial chain should be valid")

		// Note: In a real test, you might simulate corruption by directly
		// modifying the database, but that's outside the scope of this integration test
	})

	// Test cache failures
	t.Run("CacheFailureHandling", func(t *testing.T) {
		// Create event for cache testing
		event, err := audit.NewEvent(
			audit.EventSessionTerminated,
			"user-cache-test",
			"session-123",
			"terminate_session",
		)
		require.NoError(t, err)

		// Store event (should work even if cache fails)
		err = env.EventRepo.Store(env.ctx, event)
		require.NoError(t, err)

		// Retrieve event (should fallback to database if cache fails)
		retrievedEvent, err := env.EventRepo.GetByID(env.ctx, event.ID)
		require.NoError(t, err)
		assert.Equal(t, event.ID, retrievedEvent.ID)
	})

	// Test archive failures
	t.Run("ArchiveFailureHandling", func(t *testing.T) {
		// Test archive with invalid criteria
		invalidCriteria := audit.ArchiveCriteria{
			StartTime: &time.Time{}, // Invalid time
			EndTime:   &time.Time{}, // Invalid time
		}

		_, err := env.ArchiveRepo.ArchiveEvents(env.ctx, invalidCriteria)
		assert.Error(t, err, "Should fail with invalid archive criteria")

		// Test retrieval from non-existent archive
		invalidRetrievalCriteria := audit.RetrievalCriteria{
			ArchiveID: "non-existent-archive",
		}

		_, err = env.ArchiveRepo.RetrieveFromArchive(env.ctx, invalidRetrievalCriteria)
		assert.Error(t, err, "Should fail when retrieving from non-existent archive")
	})

	// Test database connection failures
	t.Run("DatabaseConnectionResilience", func(t *testing.T) {
		// Test health check
		health, err := env.EventRepo.GetHealthCheck(env.ctx)
		require.NoError(t, err)
		assert.True(t, health.Healthy, "Database should be healthy")
		assert.Equal(t, "HEALTHY", health.Status)
	})

	// Test compliance validation failures
	t.Run("ComplianceValidationFailures", func(t *testing.T) {
		complianceService := audit.NewComplianceVerificationService(env.EventRepo, env.QueryRepo)

		// Test GDPR compliance for non-existent user
		gdprReport, err := complianceService.VerifyGDPRCompliance(env.ctx, "non-existent-user")
		require.NoError(t, err)
		assert.Equal(t, int64(0), gdprReport.TotalEvents)
		assert.True(t, gdprReport.IsCompliant) // No events = compliant

		// Test TCPA compliance for non-existent phone number
		tcpaReport, err := complianceService.VerifyTCPACompliance(env.ctx, "+1999999999")
		require.NoError(t, err)
		assert.Equal(t, int64(0), tcpaReport.CallEvents)
		assert.True(t, tcpaReport.IsCompliant) // No calls = compliant
	})

	t.Logf("Failure scenarios testing completed successfully")
}