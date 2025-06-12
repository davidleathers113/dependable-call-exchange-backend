package infrastructure

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/docker/go-connections/nat"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"
)

// TestEnvironment holds all test containers and connections
type TestEnvironment struct {
	// Add network manager
	networkManager *NetworkManager

	// Add internal URLs
	postgresInternalURL string
	redisInternalURL    string
	sipInternalURL      string

	// Existing fields...
	PostgresContainer testcontainers.Container
	RedisContainer    testcontainers.Container
	KamailioContainer testcontainers.Container
	APIContainer      testcontainers.Container

	DB          *sql.DB
	RedisClient *redis.Client

	PostgresURL string
	RedisURL    string
	SIPURL      string
	APIURL      string
	WSURL       string

	ctx context.Context
	t   *testing.T
	
	// Internal reference to the simple environment
	simple *SimpleTestEnvironment
}

// NewTestEnvironment creates a complete test environment with all containers
func NewTestEnvironment(t *testing.T) *TestEnvironment {
	// Use the simplified implementation for all tests
	simple := NewSimpleTestEnvironment(t)
	
	// Create a compatibility wrapper
	env := &TestEnvironment{
		ctx:         context.Background(),
		t:           t,
		DB:          simple.DB,
		RedisClient: simple.RedisClient,
		PostgresURL: simple.PostgresURL,
		RedisURL:    simple.RedisURL,
		APIURL:      simple.APIURL,
		WSURL:       simple.WSURL,
		// Internal URLs can be same as external for now
		postgresInternalURL: simple.PostgresURL,
		redisInternalURL:    simple.RedisURL,
		simple:              simple,
	}
	
	// If tests expect API to be running by default, start it
	// This maintains backward compatibility
	if err := env.StartAPI(); err != nil {
		t.Fatalf("Failed to start API: %v", err)
	}
	
	return env
}

// StartAPI starts the API container if not already running
func (env *TestEnvironment) StartAPI() error {
	if env.simple != nil {
		err := env.simple.StartAPI()
		if err == nil {
			env.APIURL = env.simple.APIURL
			env.WSURL = env.simple.WSURL
		}
		return err
	}
	return fmt.Errorf("simple environment not initialized")
}

// NewTestEnvironmentLegacy keeps the old implementation as a fallback
func NewTestEnvironmentLegacy(t *testing.T) *TestEnvironment {
	ctx := context.Background()
	env := &TestEnvironment{
		ctx: ctx,
		t:   t,
	}

	// Start containers sequentially for simplicity
	// PostgreSQL with TimescaleDB
	container, err := env.startPostgres(ctx)
	require.NoError(t, err, "Failed to start PostgreSQL")
	env.PostgresContainer = container

	// Redis
	container, err = env.startRedis(ctx)
	require.NoError(t, err, "Failed to start Redis")
	env.RedisContainer = container

	// Kamailio SIP Server (optional)
	container, err = env.startKamailio(ctx)
	if err != nil {
		t.Logf("Warning: Failed to start Kamailio container: %v", err)
		// Continue without Kamailio - it's optional for most tests
	} else {
		env.KamailioContainer = container
	}

	// Setup connections
	env.setupConnections(ctx)

	// Run migrations
	env.runMigrations()

	// Start API container after dependencies are ready
	env.startAPIContainerLegacy(ctx)

	// Cleanup on test completion
	t.Cleanup(func() {
		env.Cleanup()
	})

	return env
}

func (env *TestEnvironment) startInfrastructureContainers(ctx context.Context) {
	// Start containers in parallel for faster setup
	var wg sync.WaitGroup
	var mu sync.Mutex
	errors := make([]error, 0)

	// PostgreSQL
	wg.Add(1)
	go func() {
		defer wg.Done()
		container, err := env.startPostgresWithNetwork(ctx)
		mu.Lock()
		if err != nil {
			errors = append(errors, fmt.Errorf("postgres: %w", err))
		} else {
			env.PostgresContainer = container
		}
		mu.Unlock()
	}()

	// Redis
	wg.Add(1)
	go func() {
		defer wg.Done()
		container, err := env.startRedisWithNetwork(ctx)
		mu.Lock()
		if err != nil {
			errors = append(errors, fmt.Errorf("redis: %w", err))
		} else {
			env.RedisContainer = container
		}
		mu.Unlock()
	}()

	// Wait for all containers
	wg.Wait()

	// Check for errors
	if len(errors) > 0 {
		for _, err := range errors {
			env.t.Errorf("Container start error: %v", err)
		}
		env.t.FailNow()
	}
}

func (env *TestEnvironment) waitForInfrastructure(ctx context.Context) {
	env.t.Log("Waiting for infrastructure to be ready...")

	// Setup database connection
	db, err := sql.Open("pgx", env.PostgresURL)
	require.NoError(env.t, err, "Failed to connect to PostgreSQL")

	// Wait for database with retries
	maxRetries := 30
	for i := 0; i < maxRetries; i++ {
		if err := db.PingContext(ctx); err == nil {
			break
		}
		if i == maxRetries-1 {
			env.t.Fatalf("Database not ready after %d attempts", maxRetries)
		}
		time.Sleep(1 * time.Second)
	}

	env.DB = db

	// Setup Redis connection
	opt, err := redis.ParseURL(env.RedisURL)
	require.NoError(env.t, err, "Failed to parse Redis URL")

	env.RedisClient = redis.NewClient(opt)

	// Wait for Redis
	for i := 0; i < maxRetries; i++ {
		if _, err := env.RedisClient.Ping(ctx).Result(); err == nil {
			break
		}
		if i == maxRetries-1 {
			env.t.Fatalf("Redis not ready after %d attempts", maxRetries)
		}
		time.Sleep(1 * time.Second)
	}

	// Run migrations after infrastructure is ready
	env.runMigrations()

	env.t.Log("Infrastructure is ready!")
}

func (env *TestEnvironment) validateConnectivity(ctx context.Context) {
	validator := NewNetworkValidator(env)
	if err := validator.ValidateConnectivity(ctx); err != nil {
		env.t.Fatalf("Connectivity validation failed: %v", err)
	}
}

func (env *TestEnvironment) startPostgres(ctx context.Context) (testcontainers.Container, error) {
	container, err := postgres.Run(ctx,
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

	if err != nil {
		return nil, fmt.Errorf("failed to start postgres: %w", err)
	}

	// Get connection string
	host, err := container.Host(ctx)
	if err != nil {
		return nil, err
	}

	port, err := container.MappedPort(ctx, "5432")
	if err != nil {
		return nil, err
	}

	env.PostgresURL = fmt.Sprintf("postgres://test:test123@%s:%s/dce_test?sslmode=disable", host, port.Port())

	return container, nil
}

func (env *TestEnvironment) startRedis(ctx context.Context) (testcontainers.Container, error) {
	container, err := tcredis.Run(ctx,
		"redis:7-alpine",
		tcredis.WithSnapshotting(10, 1),
		tcredis.WithLogLevel(tcredis.LogLevelVerbose),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to start redis: %w", err)
	}

	// Get connection string
	host, err := container.Host(ctx)
	if err != nil {
		return nil, err
	}

	port, err := container.MappedPort(ctx, "6379")
	if err != nil {
		return nil, err
	}

	env.RedisURL = fmt.Sprintf("redis://%s:%s/0", host, port.Port())

	return container, nil
}

func (env *TestEnvironment) startKamailio(ctx context.Context) (testcontainers.Container, error) {
	// Create a simple mock SIP server
	req := testcontainers.ContainerRequest{
		Image: "alpine:3.18",
		Cmd: []string{
			"sh", "-c",
			`echo "Mock SIP server starting..." && \
			 apk add --no-cache socat && \
			 while true; do \
			   echo -e "SIP/2.0 200 OK\r\n\r\n" | socat -t 0.1 TCP-LISTEN:5060,reuseaddr,fork - & \
			   echo -e "SIP/2.0 200 OK\r\n\r\n" | socat -t 0.1 UDP-LISTEN:5060,reuseaddr,fork - & \
			   echo -e "HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\n\r\nOK" | socat -t 0.1 TCP-LISTEN:8080,reuseaddr,fork - & \
			   sleep 1; \
			 done`,
		},
		ExposedPorts: []string{"5060/udp", "5060/tcp", "8080/tcp"},
		WaitingFor: wait.ForAll(
			wait.ForListeningPort("5060/tcp").WithStartupTimeout(30*time.Second),
			wait.ForHTTP("/").WithPort("8080/tcp").WithStartupTimeout(30*time.Second),
		),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to start kamailio mock: %w", err)
	}

	// Get connection info
	host, err := container.Host(ctx)
	if err != nil {
		return nil, err
	}

	port, err := container.MappedPort(ctx, "5060")
	if err != nil {
		return nil, err
	}

	env.SIPURL = fmt.Sprintf("sip:%s:%s", host, port.Port())

	return container, nil
}

func (env *TestEnvironment) startPostgresWithNetwork(ctx context.Context) (testcontainers.Container, error) {
	networkName := env.networkManager.Name()

	req := testcontainers.ContainerRequest{
		Image: "timescale/timescaledb:latest-pg15",
		Networks: []string{networkName},
		NetworkAliases: map[string][]string{
			networkName: {"postgres-e2e"},
		},
		Env: map[string]string{
			"POSTGRES_DB":       "dce_test",
			"POSTGRES_USER":     "test",
			"POSTGRES_PASSWORD": "test123",
		},
		ExposedPorts: []string{"5432/tcp"},
		WaitingFor: wait.ForAll(
			wait.ForListeningPort("5432/tcp").WithStartupTimeout(30*time.Second),
			wait.ForSQL("5432/tcp", "pgx", func(host string, port nat.Port) string {
				return fmt.Sprintf("postgres://test:test123@%s:%s/dce_test?sslmode=disable", host, port.Port())
			}).WithStartupTimeout(60*time.Second).WithPollInterval(1*time.Second),
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to start postgres: %w", err)
	}

	// Store both external (for test access) and internal URLs (for container-to-container)
	host, err := container.Host(ctx)
	if err != nil {
		return nil, err
	}

	port, err := container.MappedPort(ctx, "5432")
	if err != nil {
		return nil, err
	}

	env.PostgresURL = fmt.Sprintf("postgres://test:test123@%s:%s/dce_test?sslmode=disable", host, port.Port())
	env.postgresInternalURL = "postgres://test:test123@postgres-e2e:5432/dce_test?sslmode=disable"

	env.t.Logf("PostgreSQL started - External: %s, Internal: %s", env.PostgresURL, env.postgresInternalURL)

	return container, nil
}

func (env *TestEnvironment) startRedisWithNetwork(ctx context.Context) (testcontainers.Container, error) {
	networkName := env.networkManager.Name()

	req := testcontainers.ContainerRequest{
		Image: "redis:7-alpine",
		Networks: []string{networkName},
		NetworkAliases: map[string][]string{
			networkName: {"redis-e2e"},
		},
		Cmd: []string{
			"redis-server",
			"--save", "10", "1",
			"--loglevel", "verbose",
		},
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor: wait.ForAll(
			wait.ForListeningPort("6379/tcp").WithStartupTimeout(30*time.Second),
			wait.ForLog("Ready to accept connections").WithStartupTimeout(30*time.Second),
		),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to start redis: %w", err)
	}

	// Store both external and internal URLs
	host, err := container.Host(ctx)
	if err != nil {
		return nil, err
	}

	port, err := container.MappedPort(ctx, "6379")
	if err != nil {
		return nil, err
	}

	env.RedisURL = fmt.Sprintf("redis://%s:%s/0", host, port.Port())
	env.redisInternalURL = "redis://redis-e2e:6379/0"

	env.t.Logf("Redis started - External: %s, Internal: %s", env.RedisURL, env.redisInternalURL)

	return container, nil
}

func (env *TestEnvironment) startAPIContainer(ctx context.Context) error {
	networkName := env.networkManager.Name()

	// Wait for network DNS to stabilize
	time.Sleep(2 * time.Second)

	// Get the project root
	_, filename, _, _ := runtime.Caller(0)
	projectRoot := filepath.Join(filepath.Dir(filename), "..", "..", "..")
	
	// Verify Dockerfile exists
	dockerfilePath := filepath.Join(projectRoot, "Dockerfile")
	if _, err := os.Stat(dockerfilePath); os.IsNotExist(err) {
		return fmt.Errorf("Dockerfile not found at %s", dockerfilePath)
	}
	env.t.Logf("Using Dockerfile at: %s", dockerfilePath)

	req := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    projectRoot,
			Dockerfile: "Dockerfile",
			BuildArgs: map[string]*string{
				"BUILD_ENV": stringPtr("test"),
			},
		},
		ExposedPorts: []string{"8080/tcp"},
		Env: map[string]string{
			// Use internal URLs for container-to-container communication
			"DCE_DATABASE_URL":                            env.postgresInternalURL,
			"DCE_REDIS_URL":                               env.redisInternalURL,
			"DCE_ENVIRONMENT":                             "test",
			"DCE_LOG_LEVEL":                               "debug",
			"DCE_SECURITY_JWT_SECRET":                     "test-secret-key-for-e2e-tests",
			"DCE_SECURITY_TOKEN_EXPIRY":                   "24h",
			"DCE_SECURITY_RATE_LIMIT_REQUESTS_PER_SECOND": "10000",
			"DCE_TELEMETRY_ENABLED":                       "false",
		},
		Networks: []string{networkName},
		NetworkAliases: map[string][]string{
			networkName: {"api-e2e"},
		},
		WaitingFor: wait.ForAll(
			wait.ForListeningPort("8080/tcp").WithStartupTimeout(120*time.Second),
			wait.ForHTTP("/health").
				WithPort("8080/tcp").
				WithStartupTimeout(120*time.Second).
				WithPollInterval(2*time.Second).
				WithResponseMatcher(func(body io.Reader) bool {
					data, _ := io.ReadAll(body)
					result := strings.Contains(string(data), "healthy") || strings.Contains(string(data), "ok")
					if !result {
						env.t.Logf("Health check response: %s", string(data))
					}
					return result
				}),
		),
	}

	// Add SIP proxy if available
	if env.sipInternalURL != "" {
		req.Env["DCE_TELEPHONY_SIP_PROXY"] = env.sipInternalURL
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})

	if err != nil {
		// Capture build logs for debugging
		if container != nil {
			logs, _ := container.Logs(ctx)
			if logs != nil {
				logBytes, _ := io.ReadAll(logs)
				env.t.Logf("API Container logs:\n%s", string(logBytes))
			}
		}
		return fmt.Errorf("failed to start API container: %w", err)
	}

	env.APIContainer = container

	// Get external URL for test access
	host, err := container.Host(ctx)
	if err != nil {
		return err
	}

	port, err := container.MappedPort(ctx, "8080")
	if err != nil {
		return err
	}

	env.APIURL = fmt.Sprintf("http://%s:%s", host, port.Port())
	env.WSURL = fmt.Sprintf("ws://%s:%s", host, port.Port())

	env.t.Logf("API Container started - External: %s", env.APIURL)

	return nil
}

func (env *TestEnvironment) startAPIContainerLegacy(ctx context.Context) {
	// Get the project root
	_, filename, _, _ := runtime.Caller(0)
	projectRoot := filepath.Join(filepath.Dir(filename), "..", "..", "..")

	// For macOS Docker Desktop, we need to use host.docker.internal
	// to connect from container to host services
	dbHost := "host.docker.internal"
	
	// Parse the PostgreSQL URL to replace the host
	_, postgresPort := env.parseHostPort(env.PostgresURL, "5432")
	_, redisPort := env.parseHostPort(env.RedisURL, "6379")
	
	// Use the actual mapped ports from host's perspective
	postgresURLForContainer := fmt.Sprintf("postgres://test:test123@%s:%s/dce_test?sslmode=disable", dbHost, postgresPort)
	redisURLForContainer := fmt.Sprintf("redis://%s:%s/0", dbHost, redisPort)

	req := testcontainers.ContainerRequest{
		FromDockerfile: testcontainers.FromDockerfile{
			Context:    projectRoot,
			Dockerfile: "Dockerfile",
			BuildArgs: map[string]*string{
				"BUILD_ENV": stringPtr("test"),
			},
		},
		ExposedPorts: []string{"8080/tcp"},
		Env: map[string]string{
			"DCE_DATABASE_URL":                            postgresURLForContainer,
			"DCE_REDIS_URL":                               redisURLForContainer,
			"DCE_ENVIRONMENT":                             "test",
			"DCE_LOG_LEVEL":                               "debug",
			"DCE_SECURITY_JWT_SECRET":                     "test-secret-key-for-e2e-tests",
			"DCE_SECURITY_TOKEN_EXPIRY":                   "24h",
			"DCE_SECURITY_RATE_LIMIT_REQUESTS_PER_SECOND": "10000",
			"DCE_TELEMETRY_ENABLED":                       "false",
		},
		WaitingFor: wait.ForAll(
			wait.ForHTTP("/health").WithPort("8080/tcp").WithStartupTimeout(60*time.Second),
			wait.ForLog("starting HTTP server").WithStartupTimeout(60*time.Second),
		),
	}

	// Add SIP proxy if Kamailio is available
	if env.SIPURL != "" {
		req.Env["DCE_TELEPHONY_SIP_PROXY"] = env.SIPURL
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})

	require.NoError(env.t, err, "Failed to start API container")

	env.APIContainer = container

	// Get API URL
	host, err := container.Host(ctx)
	require.NoError(env.t, err)

	apiPort, err := container.MappedPort(ctx, "8080")
	require.NoError(env.t, err)

	env.APIURL = fmt.Sprintf("http://%s:%s", host, apiPort.Port())
	env.WSURL = fmt.Sprintf("ws://%s:%s", host, apiPort.Port())
}

// parseHostPort extracts the port from a connection URL
func (env *TestEnvironment) parseHostPort(url string, defaultPort string) (string, string) {
	// Simple parsing - in production use proper URL parsing
	if url == "" {
		return "localhost", defaultPort
	}
	
	// Extract port from URLs like redis://localhost:49153/0
	start := 0
	if idx := len("redis://"); len(url) > idx && url[:idx] == "redis://" {
		start = idx
	} else if idx := len("postgres://"); len(url) > idx && url[:idx] == "postgres://" {
		// Find the @ symbol and start after it
		if atIdx := -1; atIdx < len(url) {
			for i := idx; i < len(url); i++ {
				if url[i] == '@' {
					start = i + 1
					break
				}
			}
		}
	}
	
	// Find the port
	colonIdx := -1
	for i := start; i < len(url); i++ {
		if url[i] == ':' && (i+1 < len(url) && url[i+1] >= '0' && url[i+1] <= '9') {
			colonIdx = i
			break
		}
	}
	
	if colonIdx == -1 {
		return "localhost", defaultPort
	}
	
	// Extract host and port
	host := url[start:colonIdx]
	portEnd := colonIdx + 1
	for portEnd < len(url) && url[portEnd] >= '0' && url[portEnd] <= '9' {
		portEnd++
	}
	port := url[colonIdx+1:portEnd]
	
	return host, port
}

func (env *TestEnvironment) setupConnections(ctx context.Context) {
	// Setup PostgreSQL connection
	db, err := sql.Open("pgx", env.PostgresURL)
	require.NoError(env.t, err, "Failed to connect to PostgreSQL")

	// Verify connection
	err = db.PingContext(ctx)
	require.NoError(env.t, err, "Failed to ping PostgreSQL")

	env.DB = db

	// Setup Redis connection
	opt, err := redis.ParseURL(env.RedisURL)
	require.NoError(env.t, err, "Failed to parse Redis URL")

	env.RedisClient = redis.NewClient(opt)

	// Verify Redis connection
	_, err = env.RedisClient.Ping(ctx).Result()
	require.NoError(env.t, err, "Failed to ping Redis")
}

func (env *TestEnvironment) runMigrations() {
	// Create the basic schema
	schema := `
	CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
	CREATE EXTENSION IF NOT EXISTS "timescaledb";
	
	CREATE TABLE IF NOT EXISTS accounts (
		id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
		type VARCHAR(20) NOT NULL,
		email VARCHAR(255) UNIQUE NOT NULL,
		company_name VARCHAR(255) NOT NULL,
		status VARCHAR(20) NOT NULL DEFAULT 'active',
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS calls (
		id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
		from_number VARCHAR(20) NOT NULL,
		to_number VARCHAR(20) NOT NULL,
		status VARCHAR(20) NOT NULL,
		direction VARCHAR(20) NOT NULL,
		buyer_id UUID REFERENCES accounts(id),
		seller_id UUID REFERENCES accounts(id),
		start_time TIMESTAMPTZ NOT NULL,
		end_time TIMESTAMPTZ,
		duration INTEGER,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS bids (
		id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
		call_id UUID NOT NULL REFERENCES calls(id),
		buyer_id UUID NOT NULL REFERENCES accounts(id),
		amount DECIMAL(10,2) NOT NULL,
		status VARCHAR(20) NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);

	-- Additional tables for auth tests
	CREATE TABLE IF NOT EXISTS users (
		id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
		email VARCHAR(255) UNIQUE NOT NULL,
		password_hash VARCHAR(255) NOT NULL,
		name VARCHAR(255),
		type VARCHAR(20) NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);

	CREATE TABLE IF NOT EXISTS api_keys (
		id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
		user_id UUID NOT NULL REFERENCES users(id),
		key_hash VARCHAR(255) NOT NULL,
		name VARCHAR(255) NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		expires_at TIMESTAMPTZ
	);

	CREATE INDEX IF NOT EXISTS idx_calls_buyer_id ON calls(buyer_id);
	CREATE INDEX IF NOT EXISTS idx_calls_seller_id ON calls(seller_id);
	CREATE INDEX IF NOT EXISTS idx_bids_call_id ON bids(call_id);
	`

	_, err := env.DB.Exec(schema)
	require.NoError(env.t, err, "Failed to run migrations")
}

// ResetDatabase clears all data from tables for test isolation
func (env *TestEnvironment) ResetDatabase() {
	truncateSQL := `
		TRUNCATE TABLE api_keys, users, bids, calls, accounts RESTART IDENTITY CASCADE;
	`
	_, err := env.DB.Exec(truncateSQL)
	require.NoError(env.t, err, "Failed to reset database")
}

// Cleanup stops all containers and closes connections
func (env *TestEnvironment) Cleanup() {
	env.t.Log("Starting cleanup...")

	// Collect logs before cleanup (helpful for debugging failures)
	if env.t.Failed() {
		env.collectDebugInfo()
	}

	// Close connections first
	if env.DB != nil {
		if err := env.DB.Close(); err != nil {
			env.t.Logf("Failed to close DB connection: %v", err)
		}
	}

	if env.RedisClient != nil {
		if err := env.RedisClient.Close(); err != nil {
			env.t.Logf("Failed to close Redis client: %v", err)
		}
	}

	// Stop containers
	containers := map[string]testcontainers.Container{
		"API":      env.APIContainer,
		"Kamailio": env.KamailioContainer,
		"Redis":    env.RedisContainer,
		"Postgres": env.PostgresContainer,
	}

	for name, container := range containers {
		if container != nil {
			env.t.Logf("Stopping %s container...", name)
			if err := container.Terminate(env.ctx); err != nil {
				env.t.Logf("Failed to terminate %s container: %v", name, err)
			}
		}
	}

	// Clean up network last
	if env.networkManager != nil {
		env.t.Log("Removing network...")
		if err := env.networkManager.Cleanup(); err != nil {
			env.t.Logf("Failed to cleanup network: %v", err)
		}
	}

	env.t.Log("Cleanup completed")
}

func (env *TestEnvironment) collectDebugInfo() {
	env.t.Log("Collecting debug information...")

	// Network info
	if env.networkManager != nil {
		env.t.Logf("Network name: %s", env.networkManager.Name())
	}

	// Container logs
	containers := map[string]testcontainers.Container{
		"API":      env.APIContainer,
		"Postgres": env.PostgresContainer,
		"Redis":    env.RedisContainer,
	}

	for name, container := range containers {
		if container != nil {
			env.t.Logf("=== %s Container Logs ===", name)

			logs, err := container.Logs(env.ctx)
			if err != nil {
				env.t.Logf("Failed to get logs: %v", err)
				continue
			}

			logBytes, err := io.ReadAll(logs)
			if err != nil {
				env.t.Logf("Failed to read logs: %v", err)
				continue
			}

			env.t.Logf("%s", string(logBytes))
		}
	}
}

// Helper function
func stringPtr(s string) *string {
	return &s
}
