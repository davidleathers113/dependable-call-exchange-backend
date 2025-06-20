package testutil

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// TestDB provides test database functionality
type TestDB struct {
	t            *testing.T
	db           *sql.DB
	dbName       string
	cleanup      func()
	container    *postgres.PostgresContainer
	useContainer bool
}

// NewTestDB creates a new test database using testcontainers by default
func NewTestDB(t *testing.T) *TestDB {
	t.Helper()

	// Check if we should use legacy approach (for CI, explicit flag, or no Docker)
	if os.Getenv("USE_DOCKER_COMPOSE") == "true" || os.Getenv("USE_LOCAL_POSTGRES") == "true" {
		return newLegacyTestDB(t)
	}

	// Try testcontainers first
	db, err := tryTestcontainerDB(t)
	if err != nil {
		t.Logf("Testcontainers failed (%v), falling back to local PostgreSQL", err)
		// Set environment variable to use legacy for subsequent tests in this run
		os.Setenv("USE_LOCAL_POSTGRES", "true")
		return newLegacyTestDB(t)
	}

	return db
}

// tryTestcontainerDB attempts to create a test database using testcontainers
func tryTestcontainerDB(t *testing.T) (db *TestDB, err error) {
	t.Helper()

	// Catch panics and convert to errors
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(error); ok {
				err = e
			} else {
				err = fmt.Errorf("panic: %v", r)
			}
		}
	}()

	return newTestcontainerDB(t), nil
}

// newTestcontainerDB creates a test database using testcontainers
func newTestcontainerDB(t *testing.T) *TestDB {
	t.Helper()

	ctx := context.Background()

	// Create PostgreSQL container
	pgContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("dce_test"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("postgres"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	require.NoError(t, err)

	// Get connection string
	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	// Connect to test database
	testDB, err := sql.Open("postgres", connStr)
	require.NoError(t, err)

	// Set connection pool settings for tests
	testDB.SetMaxOpenConns(10)
	testDB.SetMaxIdleConns(5)
	testDB.SetConnMaxLifetime(5 * time.Minute)

	// Verify connection
	err = testDB.Ping()
	require.NoError(t, err)

	tdb := &TestDB{
		t:            t,
		db:           testDB,
		dbName:       "dce_test",
		container:    pgContainer,
		useContainer: true,
	}

	// Setup cleanup
	tdb.cleanup = func() {
		testDB.Close()
		if err := pgContainer.Terminate(ctx); err != nil {
			t.Logf("failed to terminate container: %v", err)
		}
	}

	// Register cleanup
	t.Cleanup(tdb.cleanup)

	// Initialize schema
	tdb.InitSchema()

	return tdb
}

// newLegacyTestDB creates a test database using docker-compose (legacy)
func newLegacyTestDB(t *testing.T) *TestDB {
	t.Helper()

	// Connect to postgres database to create test database
	// Use localhost:5432 for local PostgreSQL (default Homebrew setup)
	host := "localhost"
	port := "5432"
	// Use localhost:5433 when using docker-compose test setup
	if os.Getenv("USE_DOCKER_COMPOSE") == "true" {
		port = "5433"
	}
	// Use postgres-test:5432 when running inside Docker
	if _, inDocker := os.LookupEnv("RUNNING_IN_DOCKER"); inDocker {
		host = "postgres-test"
		port = "5432"
	}
	adminDB, err := sql.Open("postgres", fmt.Sprintf("postgres://postgres:postgres@%s:%s/postgres?sslmode=disable", host, port))
	require.NoError(t, err)
	defer adminDB.Close()

	// Generate unique test database name
	dbName := fmt.Sprintf("test_dce_%d", time.Now().UnixNano())

	// Create test database
	_, err = adminDB.Exec(fmt.Sprintf("CREATE DATABASE %s", dbName))
	require.NoError(t, err)

	// Connect to test database
	testDB, err := sql.Open("postgres", fmt.Sprintf("postgres://postgres:postgres@%s:%s/%s?sslmode=disable", host, port, dbName))
	require.NoError(t, err)

	// Set connection pool settings for tests
	testDB.SetMaxOpenConns(10)
	testDB.SetMaxIdleConns(5)
	testDB.SetConnMaxLifetime(5 * time.Minute)

	// Verify connection
	err = testDB.Ping()
	require.NoError(t, err)

	tdb := &TestDB{
		t:            t,
		db:           testDB,
		dbName:       dbName,
		useContainer: false,
	}

	// Setup cleanup
	tdb.cleanup = func() {
		testDB.Close()
		adminDB, _ := sql.Open("postgres", fmt.Sprintf("postgres://postgres:postgres@%s:%s/postgres?sslmode=disable", host, port))
		defer adminDB.Close()
		adminDB.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s", dbName))
	}

	// Register cleanup
	t.Cleanup(tdb.cleanup)

	// Initialize schema
	tdb.InitSchema()

	return tdb
}

// T returns the testing.T instance
func (tdb *TestDB) T() *testing.T {
	return tdb.t
}

// ConnectionString returns the PostgreSQL connection string for this test database
func (tdb *TestDB) ConnectionString() string {
	if tdb.useContainer && tdb.container != nil {
		ctx := context.Background()
		connStr, err := tdb.container.ConnectionString(ctx, "sslmode=disable")
		if err != nil {
			tdb.t.Fatalf("failed to get connection string: %v", err)
		}
		return connStr
	}

	// Legacy path
	host := "localhost"
	port := "5432"
	if os.Getenv("USE_DOCKER_COMPOSE") == "true" {
		port = "5433"
	}
	if _, inDocker := os.LookupEnv("RUNNING_IN_DOCKER"); inDocker {
		host = "postgres-test"
		port = "5432"
	}
	return fmt.Sprintf("postgres://postgres:postgres@%s:%s/%s?sslmode=disable", host, port, tdb.dbName)
}

// GetTestDatabaseURL returns a test database URL for use in tests
func GetTestDatabaseURL() string {
	// This returns a URL for tests that don't need a specific test database
	// For integration tests, use NewTestDB().ConnectionString() instead
	host := "localhost"
	port := "5432"
	if os.Getenv("USE_DOCKER_COMPOSE") == "true" {
		port = "5433"
	}
	if _, inDocker := os.LookupEnv("RUNNING_IN_DOCKER"); inDocker {
		host = "postgres-test"
		port = "5432"
	}
	return fmt.Sprintf("postgres://postgres:postgres@%s:%s/postgres?sslmode=disable", host, port)
}

// DB returns the underlying database connection
func (tdb *TestDB) DB() *sql.DB {
	return tdb.db
}

// PgxPool returns a pgx connection pool for tests that need it
func (tdb *TestDB) PgxPool() *pgxpool.Pool {
	tdb.t.Helper()
	
	connStr := tdb.ConnectionString()
	pool, err := pgxpool.New(context.Background(), connStr)
	require.NoError(tdb.t, err)
	
	// Register cleanup to close the pool
	tdb.t.Cleanup(func() {
		pool.Close()
	})
	
	return pool
}

// InitSchema initializes the database schema
func (tdb *TestDB) InitSchema() {
	tdb.t.Helper()

	ctx := context.Background()

	// Create extensions
	_, err := tdb.db.ExecContext(ctx, `CREATE EXTENSION IF NOT EXISTS "uuid-ossp"`)
	require.NoError(tdb.t, err)

	// Create enums
	tdb.execMulti(ctx, `
		-- Call status enum
		CREATE TYPE call_status AS ENUM (
			'pending', 'queued', 'ringing', 'in_progress', 'completed', 'failed', 'no_answer'
		);
		
		-- Call direction enum
		CREATE TYPE call_direction AS ENUM (
			'inbound', 'outbound'
		);
		
		-- Bid status enum
		CREATE TYPE bid_status AS ENUM (
			'pending', 'active', 'winning', 'won', 'lost', 'expired', 'cancelled'
		);
		
		-- Account type enum
		CREATE TYPE account_type AS ENUM (
			'buyer', 'seller', 'admin'
		);
		
		-- Account status enum
		CREATE TYPE account_status AS ENUM (
			'pending', 'active', 'suspended', 'closed'
		);
		
		-- Compliance rule type enum
		CREATE TYPE compliance_rule_type AS ENUM (
			'tcpa', 'dnc', 'gdpr', 'state', 'custom'
		);
		
		-- Consent type enum
		CREATE TYPE consent_type AS ENUM (
			'express', 'implied', 'prior_business'
		);
		
		-- Consent status enum
		CREATE TYPE consent_status AS ENUM (
			'active', 'expired', 'revoked'
		);
	`)

	// Create tables
	tdb.execMulti(ctx, `
		-- Accounts table
		CREATE TABLE accounts (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			email VARCHAR(255) UNIQUE NOT NULL,
			name VARCHAR(255) NOT NULL,
			company VARCHAR(255),
			type account_type NOT NULL,
			status account_status NOT NULL DEFAULT 'pending',
			phone_number VARCHAR(20),
			balance DECIMAL(10,2) NOT NULL DEFAULT 0,
			credit_limit DECIMAL(10,2) NOT NULL DEFAULT 0,
			quality_score DECIMAL(3,2) NOT NULL DEFAULT 0.50,
			payment_terms INTEGER DEFAULT 30,
			tcpa_consent BOOLEAN NOT NULL DEFAULT true,
			gdpr_consent BOOLEAN NOT NULL DEFAULT true,
				compliance_flags TEXT[] DEFAULT '{}',
			fraud_score DECIMAL(3,2) NOT NULL DEFAULT 0.00,
			settings JSONB NOT NULL DEFAULT '{}',
			email_verified BOOLEAN NOT NULL DEFAULT false,
			phone_verified BOOLEAN NOT NULL DEFAULT false,
			compliance_verified BOOLEAN NOT NULL DEFAULT false,
				last_login_at TIMESTAMP WITH TIME ZONE,
			last_activity_at TIMESTAMP WITH TIME ZONE,
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
		);
		
		-- Calls table
		CREATE TABLE calls (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			from_number VARCHAR(20) NOT NULL,
			to_number VARCHAR(20) NOT NULL,
			status call_status NOT NULL DEFAULT 'pending',
			direction call_direction NOT NULL,
			buyer_id UUID REFERENCES accounts(id),
			seller_id UUID REFERENCES accounts(id),
			started_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
			ended_at TIMESTAMP WITH TIME ZONE,
			duration INTEGER,
			cost DECIMAL(10,2),
			metadata JSONB NOT NULL DEFAULT '{}',
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
		);
		
		-- Bids table
		CREATE TABLE bids (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			call_id UUID NOT NULL REFERENCES calls(id),
			buyer_id UUID NOT NULL REFERENCES accounts(id),
			seller_id UUID REFERENCES accounts(id),
			amount DECIMAL(10,2) NOT NULL,
			status bid_status NOT NULL DEFAULT 'active',
			auction_id UUID,
			rank INTEGER DEFAULT 0,
			criteria JSONB NOT NULL DEFAULT '{}',
			quality_metrics JSONB NOT NULL DEFAULT '{}',
			placed_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
			expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
			accepted_at TIMESTAMP WITH TIME ZONE,
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
		);
		
		-- Compliance rules table
		CREATE TABLE compliance_rules (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			type compliance_rule_type NOT NULL,
			name VARCHAR(255) NOT NULL,
			active BOOLEAN NOT NULL DEFAULT true,
			conditions JSONB NOT NULL DEFAULT '{}',
			actions JSONB NOT NULL DEFAULT '{}',
			priority INTEGER NOT NULL DEFAULT 100,
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
		);
		
		-- Consent records table
		CREATE TABLE consent_records (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			phone_number VARCHAR(20) NOT NULL,
			type consent_type NOT NULL,
			status consent_status NOT NULL DEFAULT 'active',
			source VARCHAR(100) NOT NULL,
			expires_at TIMESTAMP WITH TIME ZONE,
			recorded_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
			metadata JSONB NOT NULL DEFAULT '{}'
		);
		
		-- Account transactions table for audit trail
		CREATE TABLE account_transactions (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			account_id UUID NOT NULL REFERENCES accounts(id),
			amount DECIMAL(10,2) NOT NULL,
			balance_after DECIMAL(10,2) NOT NULL,
			transaction_type VARCHAR(20) NOT NULL,
			description TEXT,
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
		);
		
		-- Create indexes
		CREATE INDEX idx_calls_status ON calls(status);
		CREATE INDEX idx_calls_buyer_seller ON calls(buyer_id, seller_id);
		CREATE INDEX idx_calls_created_at ON calls(created_at);
		CREATE INDEX idx_bids_call_id ON bids(call_id);
		CREATE INDEX idx_bids_buyer_id ON bids(buyer_id);
		CREATE INDEX idx_bids_seller_id ON bids(seller_id) WHERE seller_id IS NOT NULL;
		CREATE INDEX idx_bids_status ON bids(status);
		CREATE INDEX idx_bids_auction_id ON bids(auction_id) WHERE auction_id IS NOT NULL;
		CREATE INDEX idx_consent_phone ON consent_records(phone_number);
		CREATE INDEX idx_consent_status ON consent_records(status);
		
		-- Create update trigger function
		CREATE OR REPLACE FUNCTION update_updated_at_column()
		RETURNS TRIGGER AS $$
		BEGIN
			NEW.updated_at = NOW();
			RETURN NEW;
		END;
		$$ language 'plpgsql';
		
		-- Add update triggers
		CREATE TRIGGER update_accounts_updated_at BEFORE UPDATE ON accounts
			FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
		CREATE TRIGGER update_calls_updated_at BEFORE UPDATE ON calls
			FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
		CREATE TRIGGER update_bids_updated_at BEFORE UPDATE ON bids
			FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
		CREATE TRIGGER update_compliance_rules_updated_at BEFORE UPDATE ON compliance_rules
			FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
	`)

	// Add consent management tables
	tdb.execMulti(ctx, `
		-- Create consent consumers table
		CREATE TABLE IF NOT EXISTS consent_consumers (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			phone_number VARCHAR(20),
			email VARCHAR(255),
			first_name VARCHAR(100) NOT NULL,
			last_name VARCHAR(100) NOT NULL,
			metadata JSONB DEFAULT '{}',
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
			CONSTRAINT check_contact CHECK (phone_number IS NOT NULL OR email IS NOT NULL)
		);

		-- Create consent aggregates table
		CREATE TABLE IF NOT EXISTS consent_aggregates (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			consumer_id UUID NOT NULL REFERENCES consent_consumers(id),
			business_id UUID NOT NULL,
			consent_type VARCHAR(20) NOT NULL DEFAULT 'tcpa',
			current_version INTEGER NOT NULL DEFAULT 1,
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
		);

		-- Create consent versions table
		CREATE TABLE IF NOT EXISTS consent_versions (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			consent_id UUID NOT NULL REFERENCES consent_aggregates(id) ON DELETE CASCADE,
			version_number INTEGER NOT NULL,
			status VARCHAR(20) NOT NULL CHECK (status IN ('pending', 'active', 'revoked', 'expired')),
			channels TEXT[] NOT NULL CHECK (array_length(channels, 1) > 0),
			purpose VARCHAR(50) NOT NULL,
			source VARCHAR(50) NOT NULL,
			source_details JSONB DEFAULT '{}',
			consented_at TIMESTAMP WITH TIME ZONE,
			expires_at TIMESTAMP WITH TIME ZONE,
			revoked_at TIMESTAMP WITH TIME ZONE,
			created_by UUID NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
			CONSTRAINT uk_consent_version UNIQUE (consent_id, version_number)
		);

		-- Create consent proofs table
		CREATE TABLE IF NOT EXISTS consent_proofs (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			consent_version_id UUID NOT NULL REFERENCES consent_versions(id) ON DELETE CASCADE,
			proof_type VARCHAR(50) NOT NULL,
			storage_location TEXT NOT NULL,
			hash VARCHAR(256) NOT NULL,
			algorithm VARCHAR(20) NOT NULL DEFAULT 'SHA256',
			metadata JSONB DEFAULT '{}',
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
		);

		-- Create indexes for consent tables
		CREATE INDEX IF NOT EXISTS idx_consent_consumers_phone ON consent_consumers(phone_number) WHERE phone_number IS NOT NULL;
		CREATE INDEX IF NOT EXISTS idx_consent_consumers_email ON consent_consumers(email) WHERE email IS NOT NULL;
		CREATE INDEX IF NOT EXISTS idx_consent_aggregates_consumer_id ON consent_aggregates(consumer_id);
		CREATE INDEX IF NOT EXISTS idx_consent_aggregates_business_id ON consent_aggregates(business_id);
		CREATE INDEX IF NOT EXISTS idx_consent_versions_consent_id ON consent_versions(consent_id);
		CREATE INDEX IF NOT EXISTS idx_consent_versions_status ON consent_versions(status);
		CREATE INDEX IF NOT EXISTS idx_consent_proofs_version_id ON consent_proofs(consent_version_id);

		-- Add consent triggers
		CREATE TRIGGER update_consent_consumers_updated_at BEFORE UPDATE ON consent_consumers
			FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
		CREATE TRIGGER update_consent_aggregates_updated_at BEFORE UPDATE ON consent_aggregates
			FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
	`)
}

// execMulti executes multiple SQL statements
func (tdb *TestDB) execMulti(ctx context.Context, sql string) {
	tdb.t.Helper()
	_, err := tdb.db.ExecContext(ctx, sql)
	require.NoError(tdb.t, err)
}

// TruncateTables truncates all tables for test isolation
func (tdb *TestDB) TruncateTables() {
	tdb.t.Helper()

	ctx := context.Background()
	tables := []string{
		"consent_proofs",
		"consent_versions", 
		"consent_aggregates",
		"consent_consumers",
		"consent_records",
		"compliance_rules",
		"bids",
		"calls",
		"accounts",
	}

	for _, table := range tables {
		_, err := tdb.db.ExecContext(ctx, fmt.Sprintf("TRUNCATE TABLE %s CASCADE", table))
		require.NoError(tdb.t, err)
	}
}

// Snapshot creates a database snapshot for fast restoration
func (tdb *TestDB) Snapshot(name string) error {
	if !tdb.useContainer {
		// Legacy mode doesn't support snapshots, just use TruncateTables
		return nil
	}

	// For now, we'll implement a simple snapshot by storing the current state
	// Real snapshot support requires additional container features
	return nil
}

// RestoreSnapshot quickly restores database to a snapshot state
func (tdb *TestDB) RestoreSnapshot(name string) error {
	// For now, just truncate tables as a simple restore mechanism
	tdb.TruncateTables()
	return nil
}

// RunInTransaction executes a function within a transaction that's always rolled back
func (tdb *TestDB) RunInTransaction(fn func(*sql.Tx) error) error {
	tx, err := tdb.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	defer func() {
		// Always rollback to ensure test isolation
		if rbErr := tx.Rollback(); rbErr != nil && rbErr != sql.ErrTxDone {
			tdb.t.Errorf("failed to rollback transaction: %v", rbErr)
		}
	}()

	return fn(tx)
}

// SeedData is a generic interface for seeding test data
type SeedData interface {
	// TableName returns the table name for this entity
	TableName() string
	// InsertQuery returns the INSERT SQL query
	InsertQuery() string
	// Values returns the values to insert
	Values() []any
}

// Seed inserts test data into the database
func (tdb *TestDB) Seed(data ...SeedData) {
	tdb.t.Helper()

	ctx := context.Background()
	for _, d := range data {
		_, err := tdb.db.ExecContext(ctx, d.InsertQuery(), d.Values()...)
		require.NoError(tdb.t, err, "failed to seed %s", d.TableName())
	}
}

// WithTx executes a function within a transaction
func (tdb *TestDB) WithTx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := tdb.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p)
		}
	}()

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("rollback failed: %v, original error: %w", rbErr, err)
		}
		return err
	}

	return tx.Commit()
}

// AssertRowCount asserts the number of rows in a table
func (tdb *TestDB) AssertRowCount(table string, expected int) {
	tdb.t.Helper()

	var count int
	err := tdb.db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", table)).Scan(&count)
	require.NoError(tdb.t, err)
	require.Equal(tdb.t, expected, count, "expected %d rows in %s, got %d", expected, table, count)
}

// GetRowCount returns the number of rows in a table
func (tdb *TestDB) GetRowCount(t *testing.T, table string) int {
	t.Helper()

	var count int
	err := tdb.db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", table)).Scan(&count)
	require.NoError(t, err, "failed to count rows in %s", table)
	return count
}
