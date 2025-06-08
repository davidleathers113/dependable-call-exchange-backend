package testutil

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"
	
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

// TestDB provides test database functionality
type TestDB struct {
	t       *testing.T
	db      *sql.DB
	dbName  string
	cleanup func()
}

// NewTestDB creates a new test database
func NewTestDB(t *testing.T) *TestDB {
	t.Helper()
	
	// Connect to postgres database to create test database
	adminDB, err := sql.Open("postgres", "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable")
	require.NoError(t, err)
	defer adminDB.Close()
	
	// Generate unique test database name
	dbName := fmt.Sprintf("test_dce_%d", time.Now().UnixNano())
	
	// Create test database
	_, err = adminDB.Exec(fmt.Sprintf("CREATE DATABASE %s", dbName))
	require.NoError(t, err)
	
	// Connect to test database
	testDB, err := sql.Open("postgres", fmt.Sprintf("postgres://postgres:postgres@localhost:5432/%s?sslmode=disable", dbName))
	require.NoError(t, err)
	
	// Set connection pool settings for tests
	testDB.SetMaxOpenConns(10)
	testDB.SetMaxIdleConns(5)
	testDB.SetConnMaxLifetime(5 * time.Minute)
	
	// Verify connection
	err = testDB.Ping()
	require.NoError(t, err)
	
	tdb := &TestDB{
		t:      t,
		db:     testDB,
		dbName: dbName,
	}
	
	// Setup cleanup
	tdb.cleanup = func() {
		testDB.Close()
		adminDB, _ := sql.Open("postgres", "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable")
		defer adminDB.Close()
		adminDB.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s", dbName))
	}
	
	// Register cleanup
	t.Cleanup(tdb.cleanup)
	
	// Initialize schema
	tdb.InitSchema()
	
	return tdb
}

// DB returns the underlying database connection
func (tdb *TestDB) DB() *sql.DB {
	return tdb.db
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
			'pending', 'routing', 'active', 'completed', 'failed', 'cancelled'
		);
		
		-- Call type enum
		CREATE TYPE call_type AS ENUM (
			'inbound', 'outbound'
		);
		
		-- Bid status enum
		CREATE TYPE bid_status AS ENUM (
			'active', 'won', 'lost', 'expired', 'cancelled'
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
			company VARCHAR(255) NOT NULL,
			type account_type NOT NULL,
			status account_status NOT NULL DEFAULT 'pending',
			balance DECIMAL(10,2) NOT NULL DEFAULT 0,
			credit_limit DECIMAL(10,2) NOT NULL DEFAULT 0,
			quality_score DECIMAL(3,2) NOT NULL DEFAULT 0.50,
			settings JSONB NOT NULL DEFAULT '{}',
			email_verified BOOLEAN NOT NULL DEFAULT false,
			phone_verified BOOLEAN NOT NULL DEFAULT false,
			compliance_verified BOOLEAN NOT NULL DEFAULT false,
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
			type call_type NOT NULL,
			buyer_id UUID REFERENCES accounts(id),
			seller_id UUID REFERENCES accounts(id),
			started_at TIMESTAMP WITH TIME ZONE,
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
			amount DECIMAL(10,2) NOT NULL,
			status bid_status NOT NULL DEFAULT 'active',
			criteria JSONB NOT NULL DEFAULT '{}',
			placed_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
			expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
			won_at TIMESTAMP WITH TIME ZONE,
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
		
		-- Create indexes
		CREATE INDEX idx_calls_status ON calls(status);
		CREATE INDEX idx_calls_buyer_seller ON calls(buyer_id, seller_id);
		CREATE INDEX idx_calls_created_at ON calls(created_at);
		CREATE INDEX idx_bids_call_id ON bids(call_id);
		CREATE INDEX idx_bids_buyer_id ON bids(buyer_id);
		CREATE INDEX idx_bids_status ON bids(status);
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

// SeedData is a generic interface for seeding test data
type SeedData interface {
	// TableName returns the table name for this entity
	TableName() string
	// InsertQuery returns the INSERT SQL query
	InsertQuery() string
	// Values returns the values to insert
	Values() []interface{}
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