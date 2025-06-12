package infrastructure

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/docker/go-connections/nat"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/compose"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"
)

// SimpleTestEnvironment uses Docker Compose for container orchestration
type SimpleTestEnvironment struct {
	compose       *compose.DockerCompose
	DB            *sql.DB
	RedisClient   *redis.Client
	PostgresURL   string
	RedisURL      string
	APIURL        string
	WSURL         string
	apiContainer  testcontainers.Container
	ctx           context.Context
	t             *testing.T
}

// NewSimpleTestEnvironment creates a test environment using Docker Compose
func NewSimpleTestEnvironment(t *testing.T) *SimpleTestEnvironment {
	ctx := context.Background()
	
	// Create a temporary compose file for testing
	composeContent := `
services:
  postgres:
    image: timescale/timescaledb:latest-pg15
    environment:
      POSTGRES_DB: dce_test
      POSTGRES_USER: test
      POSTGRES_PASSWORD: test123
    ports:
      - "5432"
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U test -d dce_test"]
      interval: 5s
      timeout: 5s
      retries: 5

  redis:
    image: redis:7-alpine
    command: redis-server --save 60 1 --loglevel warning
    ports:
      - "6379"
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 5s
      timeout: 5s
      retries: 5
`
	
	// Write compose content to temporary file
	composeFile, err := os.CreateTemp("", "compose-test-*.yml")
	require.NoError(t, err)
	defer os.Remove(composeFile.Name())
	
	_, err = composeFile.WriteString(composeContent)
	require.NoError(t, err)
	require.NoError(t, composeFile.Close())
	
	// Create a unique identifier for this test stack
	identifier := fmt.Sprintf("e2e-%s-%d", strings.ToLower(strings.ReplaceAll(t.Name(), "/", "_")), time.Now().Unix())
	
	// Create compose stack for postgres and redis only
	composeStack, err := compose.NewDockerComposeWith(
		compose.WithStackFiles(composeFile.Name()),
		compose.StackIdentifier(identifier),
	)
	require.NoError(t, err, "Failed to create compose stack")
	
	// Start postgres and redis services only
	t.Log("Starting PostgreSQL and Redis services...")
	err = composeStack.Up(ctx, compose.Wait(true))
	require.NoError(t, err, "Failed to start compose stack")
	
	env := &SimpleTestEnvironment{
		compose: composeStack,
		ctx:     ctx,
		t:       t,
	}
	
	// Wait for services to be ready
	t.Log("Waiting for services to be ready...")
	
	// Get PostgreSQL connection
	postgresContainer, err := composeStack.ServiceContainer(ctx, "postgres")
	require.NoError(t, err, "Failed to get postgres container")
	
	postgresHost, err := postgresContainer.Host(ctx)
	require.NoError(t, err)
	
	postgresPort, err := postgresContainer.MappedPort(ctx, "5432")
	require.NoError(t, err)
	
	env.PostgresURL = fmt.Sprintf("postgres://test:test123@%s:%s/dce_test?sslmode=disable", 
		postgresHost, postgresPort.Port())
	
	// Connect to PostgreSQL
	env.DB, err = sql.Open("pgx", env.PostgresURL)
	require.NoError(t, err, "Failed to connect to PostgreSQL")
	
	// Get Redis connection
	redisContainer, err := composeStack.ServiceContainer(ctx, "redis")
	require.NoError(t, err, "Failed to get redis container")
	
	redisHost, err := redisContainer.Host(ctx)
	require.NoError(t, err)
	
	redisPort, err := redisContainer.MappedPort(ctx, "6379")
	require.NoError(t, err)
	
	env.RedisURL = fmt.Sprintf("redis://%s:%s/0", redisHost, redisPort.Port())
	
	// Connect to Redis
	opt, err := redis.ParseURL(env.RedisURL)
	require.NoError(t, err)
	env.RedisClient = redis.NewClient(opt)
	
	// API is not started by default - tests can call StartAPI() if needed
	// This keeps infrastructure tests fast
	env.APIURL = "" // Will be set when StartAPI is called
	env.WSURL = ""  // Will be set when StartAPI is called
	
	// Run migrations
	env.runMigrations(t)
	
	// Cleanup on test completion
	t.Cleanup(func() {
		env.Cleanup(ctx)
	})
	
	t.Log("Test environment ready!")
	return env
}

// Alternative: Use testcontainers modules for individual containers
func NewModularTestEnvironment(t *testing.T) *SimpleTestEnvironment {
	ctx := context.Background()
	env := &SimpleTestEnvironment{}
	
	// Use testcontainers postgres module
	postgresContainer, err := postgres.Run(ctx,
		"timescale/timescaledb:latest-pg15",
		postgres.WithDatabase("dce_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test123"),
		testcontainers.WithWaitStrategy(
			wait.ForSQL("5432/tcp", "pgx", func(host string, port nat.Port) string {
				return fmt.Sprintf("postgres://test:test123@%s:%s/dce_test?sslmode=disable", host, port.Port())
			}).WithStartupTimeout(60*time.Second),
		),
	)
	testcontainers.CleanupContainer(t, postgresContainer)
	require.NoError(t, err)
	
	// Use testcontainers redis module
	redisContainer, err := tcredis.Run(ctx,
		"redis:7-alpine",
		tcredis.WithSnapshotting(10, 1),
		tcredis.WithLogLevel(tcredis.LogLevelVerbose),
	)
	testcontainers.CleanupContainer(t, redisContainer)
	require.NoError(t, err)
	
	// Get connection strings using module methods
	env.PostgresURL, err = postgresContainer.ConnectionString(ctx)
	require.NoError(t, err)
	
	env.RedisURL, err = redisContainer.ConnectionString(ctx)
	require.NoError(t, err)
	
	// Connect to databases
	env.DB, err = sql.Open("pgx", env.PostgresURL)
	require.NoError(t, err)
	
	opt, err := redis.ParseURL(env.RedisURL)
	require.NoError(t, err)
	env.RedisClient = redis.NewClient(opt)
	
	// Run migrations
	env.runMigrations(t)
	
	// API container can be started by individual tests if needed
	// For now, we provide just the database and cache infrastructure
	
	return env
}

func (env *SimpleTestEnvironment) runMigrations(t *testing.T) {
	// Run simplified migrations for E2E tests
	schema := `
		CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
		CREATE EXTENSION IF NOT EXISTS "timescaledb";
		
		-- Account types
		DO $$ BEGIN
			CREATE TYPE account_type AS ENUM ('buyer', 'seller', 'admin');
		EXCEPTION
			WHEN duplicate_object THEN null;
		END $$;

		DO $$ BEGIN
			CREATE TYPE account_status AS ENUM ('active', 'suspended', 'closed');
		EXCEPTION
			WHEN duplicate_object THEN null;
		END $$;

		DO $$ BEGIN
			CREATE TYPE call_status AS ENUM ('pending', 'ringing', 'in_progress', 'completed', 'failed', 'no_answer');
		EXCEPTION
			WHEN duplicate_object THEN null;
		END $$;

		DO $$ BEGIN
			CREATE TYPE bid_status AS ENUM ('active', 'won', 'lost', 'expired');
		EXCEPTION
			WHEN duplicate_object THEN null;
		END $$;
		
		-- Accounts table
		CREATE TABLE IF NOT EXISTS accounts (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			type account_type NOT NULL,
			email VARCHAR(255) UNIQUE NOT NULL,
			company_name VARCHAR(255) NOT NULL,
			status account_status NOT NULL DEFAULT 'active',
			balance DECIMAL(15,4) DEFAULT 0.00 NOT NULL CHECK (balance >= 0),
			quality_score INTEGER DEFAULT 50 CHECK (quality_score BETWEEN 0 AND 100),
			settings JSONB DEFAULT '{}' NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		-- Users table (for authentication)
		CREATE TABLE IF NOT EXISTS users (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			account_id UUID REFERENCES accounts(id),
			email VARCHAR(255) UNIQUE NOT NULL,
			password_hash VARCHAR(255) NOT NULL,
			name VARCHAR(255) NOT NULL,
			type VARCHAR(20) NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		-- API Keys table
		CREATE TABLE IF NOT EXISTS api_keys (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			account_id UUID REFERENCES accounts(id),
			name VARCHAR(255) NOT NULL,
			key_hash VARCHAR(255) UNIQUE NOT NULL,
			last_used_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			revoked_at TIMESTAMPTZ
		);

		-- Calls table
		CREATE TABLE IF NOT EXISTS calls (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			from_number VARCHAR(20) NOT NULL,
			to_number VARCHAR(20) NOT NULL,
			status call_status NOT NULL DEFAULT 'pending',
			buyer_id UUID NOT NULL REFERENCES accounts(id),
			seller_id UUID REFERENCES accounts(id),
			start_time TIMESTAMPTZ,
			end_time TIMESTAMPTZ,
			duration INTEGER,
			total_cost DECIMAL(12,4),
			metadata JSONB DEFAULT '{}',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);

		-- Bids table
		CREATE TABLE IF NOT EXISTS bids (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
			call_id UUID NOT NULL,
			buyer_id UUID NOT NULL REFERENCES accounts(id),
			amount DECIMAL(12,4) NOT NULL CHECK (amount >= 0),
			status bid_status NOT NULL DEFAULT 'active',
			placed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			expires_at TIMESTAMPTZ NOT NULL
		);

		-- Create basic indexes
		CREATE INDEX IF NOT EXISTS idx_accounts_email ON accounts(email);
		CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
		CREATE INDEX IF NOT EXISTS idx_calls_buyer ON calls(buyer_id);
		CREATE INDEX IF NOT EXISTS idx_bids_call ON bids(call_id);
	`
	
	_, err := env.DB.Exec(schema)
	require.NoError(t, err, "Failed to run migrations")
}

// startAPIWithConnections can be implemented by tests that need the API
// For now, we keep the environment focused on database/cache services only

func (env *SimpleTestEnvironment) ResetDatabase() {
	truncateSQL := `TRUNCATE TABLE api_keys, users, bids, calls, accounts RESTART IDENTITY CASCADE;`
	_, err := env.DB.Exec(truncateSQL)
	if err != nil {
		panic(fmt.Sprintf("Failed to reset database: %v", err))
	}
}

// StartAPI starts the API container on demand
func (env *SimpleTestEnvironment) StartAPI() error {
	if env.apiContainer != nil {
		return nil // Already started
	}
	
	env.t.Log("Starting API container...")
	
	// Use the new API container helper with fast prebuilt binary
	opts := DefaultAPIContainerOptions()
	container, apiURL, err := StartAPIContainer(env.ctx, env.t, env.PostgresURL, env.RedisURL, opts)
	if err != nil {
		return fmt.Errorf("failed to start API container: %w", err)
	}
	
	env.apiContainer = container
	env.APIURL = apiURL
	env.WSURL = fmt.Sprintf("ws://%s", apiURL[7:]) // Convert http:// to ws://
	
	// Wait for API to be fully ready
	if err := WaitForAPIReady(env.ctx, apiURL, 30*time.Second); err != nil {
		// Get container logs for debugging
		logs, _ := container.Logs(env.ctx)
		if logs != nil {
			defer logs.Close()
			logBytes := make([]byte, 8192)
			n, _ := logs.Read(logBytes)
			env.t.Logf("API container logs:\n%s", string(logBytes[:n]))
		}
		return fmt.Errorf("API failed to become ready: %w", err)
	}
	
	env.t.Logf("API container ready at %s", apiURL)
	return nil
}

// RequireAPI starts the API if not already running and returns the URL
func (env *SimpleTestEnvironment) RequireAPI(t *testing.T) string {
	if env.APIURL == "" {
		err := env.StartAPI()
		require.NoError(t, err, "Failed to start API container")
	}
	return env.APIURL
}

func (env *SimpleTestEnvironment) Cleanup(ctx context.Context) {
	// Close connections
	if env.DB != nil {
		env.DB.Close()
	}
	if env.RedisClient != nil {
		env.RedisClient.Close()
	}
	
	// Stop API container if started
	if env.apiContainer != nil {
		_ = env.apiContainer.Terminate(ctx)
	}
	
	// Stop compose stack if used
	if env.compose != nil {
		// Down with volume removal for clean test state
		_ = env.compose.Down(ctx, compose.RemoveOrphans(true), compose.RemoveVolumes(true))
	}
}