# Testcontainers Guide

**Version:** 1.0.0  
**Date:** June 9, 2025  
**Status:** Active

## Table of Contents
- [Introduction](#introduction)
- [Installation and Setup](#installation-and-setup)
- [Core Concepts](#core-concepts)
- [PostgreSQL Module](#postgresql-module)
- [Container Management](#container-management)
- [Advanced Features](#advanced-features)
- [Integration Patterns](#integration-patterns)
- [Performance Optimization](#performance-optimization)
- [Troubleshooting](#troubleshooting)

## Introduction

Testcontainers is a testing library that provides lightweight, throwaway instances of databases, message brokers, and other services running in Docker containers. This guide covers how to effectively use Testcontainers-Go in the Dependable Call Exchange Backend project.

### Why Testcontainers?

1. **Real Services**: Test against actual PostgreSQL, Redis, Kafka instances
2. **Isolation**: Each test gets a clean environment
3. **Reproducibility**: Consistent behavior across environments
4. **CI/CD Friendly**: Works seamlessly in containerized pipelines
5. **Developer Experience**: No manual service setup required

## Installation and Setup

### Dependencies

Add to your `go.mod`:

```go
require (
    github.com/testcontainers/testcontainers-go v0.33.0
    github.com/testcontainers/testcontainers-go/modules/postgres v0.33.0
    github.com/testcontainers/testcontainers-go/modules/redis v0.33.0
    github.com/testcontainers/testcontainers-go/modules/kafka v0.33.0
)
```

### System Requirements

1. **Docker**: Docker Desktop or Docker Engine must be installed
2. **Resources**: Ensure adequate CPU and memory for containers
3. **Permissions**: User must have Docker socket access

### Basic Setup

```go
// internal/testutil/containers/setup.go
package containers

import (
    "context"
    "fmt"
    "os"
    
    "github.com/testcontainers/testcontainers-go"
)

func init() {
    // Configure Testcontainers
    if os.Getenv("TESTCONTAINERS_RYUK_DISABLED") == "" {
        os.Setenv("TESTCONTAINERS_RYUK_DISABLED", "false")
    }
    
    // Set custom Docker host if needed
    if dockerHost := os.Getenv("DOCKER_HOST"); dockerHost != "" {
        os.Setenv("DOCKER_HOST", dockerHost)
    }
}

// IsDockerAvailable checks if Docker is accessible
func IsDockerAvailable() bool {
    ctx := context.Background()
    provider, err := testcontainers.NewDockerProvider()
    if err != nil {
        return false
    }
    defer provider.Close()
    
    return provider.Health(ctx) == nil
}
```

## Core Concepts

### Container Lifecycle

```go
// 1. Container Request - Define what you need
req := testcontainers.ContainerRequest{
    Image:        "postgres:16-alpine",
    ExposedPorts: []string{"5432/tcp"},
    Env: map[string]string{
        "POSTGRES_USER":     "test",
        "POSTGRES_PASSWORD": "test",
        "POSTGRES_DB":       "testdb",
    },
    WaitingFor: wait.ForListeningPort("5432/tcp"),
}

// 2. Container Creation - Start the container
container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
    ContainerRequest: req,
    Started:          true,
})

// 3. Container Usage - Get connection details
host, err := container.Host(ctx)
port, err := container.MappedPort(ctx, "5432/tcp")

// 4. Container Cleanup - Always clean up
defer container.Terminate(ctx)
```

### Wait Strategies

Testcontainers provides various strategies to determine when a container is ready:

```go
// Wait for a log message
wait.ForLog("database system is ready to accept connections")

// Wait for a port to be listening
wait.ForListeningPort("5432/tcp")

// Wait for an HTTP endpoint
wait.ForHTTP("/health").WithPort("8080/tcp")

// Wait for a SQL query to succeed
wait.ForSQL("5432/tcp", "postgres", func(host string, port nat.Port) string {
    return fmt.Sprintf("postgres://user:pass@%s:%s/db", host, port.Port())
})

// Combine multiple strategies
wait.ForAll(
    wait.ForListeningPort("5432/tcp"),
    wait.ForLog("ready to accept connections"),
).WithDeadline(1 * time.Minute)
```

## PostgreSQL Module

### Basic PostgreSQL Container

```go
// internal/testutil/containers/postgres.go
package containers

import (
    "context"
    "database/sql"
    "fmt"
    "time"
    
    "github.com/testcontainers/testcontainers-go"
    "github.com/testcontainers/testcontainers-go/modules/postgres"
    "github.com/testcontainers/testcontainers-go/wait"
    _ "github.com/lib/pq"
)

type PostgresContainer struct {
    *postgres.PostgresContainer
    ConnectionString string
    DB              *sql.DB
}

func NewPostgresContainer(ctx context.Context, opts ...PostgresOption) (*PostgresContainer, error) {
    // Default configuration
    config := &postgresConfig{
        image:    "postgres:16-alpine",
        database: "dce_test",
        username: "postgres",
        password: "postgres",
        initScripts: []string{
            "../../migrations/001_initial_schema.sql",
        },
    }
    
    // Apply options
    for _, opt := range opts {
        opt(config)
    }
    
    // Create container
    pgContainer, err := postgres.Run(ctx,
        config.image,
        postgres.WithDatabase(config.database),
        postgres.WithUsername(config.username),
        postgres.WithPassword(config.password),
        postgres.WithInitScripts(config.initScripts...),
        postgres.WithWaitStrategy(
            wait.ForLog("database system is ready to accept connections").
                WithOccurrence(2).
                WithStartupTimeout(30*time.Second),
        ),
        testcontainers.WithReuse(true),
    )
    
    if err != nil {
        return nil, fmt.Errorf("failed to start postgres: %w", err)
    }
    
    // Get connection string
    connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
    if err != nil {
        return nil, fmt.Errorf("failed to get connection string: %w", err)
    }
    
    // Connect to database
    db, err := sql.Open("postgres", connStr)
    if err != nil {
        return nil, fmt.Errorf("failed to connect to postgres: %w", err)
    }
    
    // Verify connection
    if err := db.Ping(); err != nil {
        return nil, fmt.Errorf("failed to ping postgres: %w", err)
    }
    
    return &PostgresContainer{
        PostgresContainer: pgContainer,
        ConnectionString:  connStr,
        DB:               db,
    }, nil
}

// Configuration options
type postgresConfig struct {
    image       string
    database    string
    username    string
    password    string
    initScripts []string
}

type PostgresOption func(*postgresConfig)

func WithPostgresImage(image string) PostgresOption {
    return func(c *postgresConfig) {
        c.image = image
    }
}

### PostgreSQL with Snapshots

```go
// Advanced PostgreSQL container with snapshot support
func NewPostgresContainerWithSnapshot(ctx context.Context) (*PostgresContainer, error) {
    pgContainer, err := postgres.Run(ctx,
        "postgres:16-alpine",
        postgres.WithDatabase("dce_test"),
        postgres.WithUsername("postgres"),
        postgres.WithPassword("postgres"),
        postgres.WithInitScripts("../../migrations/schema.sql"),
        postgres.WithSnapshotName("clean-state"),
        postgres.WithWaitStrategy(
            wait.ForLog("database system is ready to accept connections").
                WithOccurrence(2),
        ),
        testcontainers.WithReuse(true),
    )
    
    if err != nil {
        return nil, err
    }
    
    // Create initial snapshot after initialization
    if err := pgContainer.Snapshot(ctx); err != nil {
        return nil, fmt.Errorf("failed to create snapshot: %w", err)
    }
    
    connStr, _ := pgContainer.ConnectionString(ctx, "sslmode=disable")
    db, _ := sql.Open("postgres", connStr)
    
    return &PostgresContainer{
        PostgresContainer: pgContainer,
        ConnectionString:  connStr,
        DB:               db,
    }, nil
}

// RestoreSnapshot restores the database to the snapshot state
func (p *PostgresContainer) RestoreSnapshot(ctx context.Context) error {
    return p.PostgresContainer.Restore(ctx)
}

// Usage in tests
func TestWithSnapshot(t *testing.T) {
    ctx := context.Background()
    container, err := NewPostgresContainerWithSnapshot(ctx)
    require.NoError(t, err)
    defer container.Terminate(ctx)
    
    tests := []struct {
        name string
        test func(t *testing.T, db *sql.DB)
    }{
        {
            name: "test one",
            test: func(t *testing.T, db *sql.DB) {
                // Modify database
                _, err := db.Exec("INSERT INTO users (name) VALUES ('test')")
                require.NoError(t, err)
            },
        },
        {
            name: "test two",
            test: func(t *testing.T, db *sql.DB) {
                // This test starts with clean database
                var count int
                err := db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
                require.NoError(t, err)
                assert.Equal(t, 0, count) // Clean state!
            },
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Restore to clean state before each test
            err := container.RestoreSnapshot(ctx)
            require.NoError(t, err)
            
            tt.test(t, container.DB)
        })
    }
}
```

## Container Management

### Redis Container

```go
// internal/testutil/containers/redis.go
package containers

import (
    "context"
    "fmt"
    
    "github.com/redis/go-redis/v9"
    "github.com/testcontainers/testcontainers-go"
    "github.com/testcontainers/testcontainers-go/modules/redis"
)

type RedisContainer struct {
    *redis.RedisContainer
    ConnectionString string
    Client          *redis.Client
}

func NewRedisContainer(ctx context.Context) (*RedisContainer, error) {
    redisContainer, err := redis.Run(ctx,
        "redis:7-alpine",
        redis.WithSnapshotting(10, 1),
        redis.WithLogLevel(redis.LogLevelVerbose),
        testcontainers.WithReuse(true),
    )
    
    if err != nil {
        return nil, fmt.Errorf("failed to start redis: %w", err)
    }
    
    connStr, err := redisContainer.ConnectionString(ctx)
    if err != nil {
        return nil, fmt.Errorf("failed to get redis connection string: %w", err)
    }
    
    // Create Redis client
    opt, err := redis.ParseURL(connStr)
    if err != nil {
        return nil, fmt.Errorf("failed to parse redis URL: %w", err)
    }
    
    client := redis.NewClient(opt)
    
    // Verify connection
    if err := client.Ping(ctx).Err(); err != nil {
        return nil, fmt.Errorf("failed to ping redis: %w", err)
    }
    
    return &RedisContainer{
        RedisContainer:   redisContainer,
        ConnectionString: connStr,
        Client:          client,
    }, nil
}

// FlushAll clears all Redis data
func (r *RedisContainer) FlushAll(ctx context.Context) error {
    return r.Client.FlushAll(ctx).Err()
}
```

### Kafka Container

```go
// internal/testutil/containers/kafka.go
package containers

import (
    "context"
    "fmt"
    
    "github.com/testcontainers/testcontainers-go"
    "github.com/testcontainers/testcontainers-go/modules/kafka"
)

type KafkaContainer struct {
    *kafka.KafkaContainer
    BootstrapServers string
}

func NewKafkaContainer(ctx context.Context) (*KafkaContainer, error) {
    kafkaContainer, err := kafka.Run(ctx,
        "confluentinc/confluent-local:7.5.0",
        kafka.WithClusterID("test-cluster"),
        testcontainers.WithReuse(true),
    )
    
    if err != nil {
        return nil, fmt.Errorf("failed to start kafka: %w", err)
    }
    
    brokers, err := kafkaContainer.Brokers(ctx)
    if err != nil {
        return nil, fmt.Errorf("failed to get kafka brokers: %w", err)
    }
    
    return &KafkaContainer{
        KafkaContainer:   kafkaContainer,
        BootstrapServers: brokers[0],
    }, nil
}
```

### Container Suite for Integration Tests

```go
// internal/testutil/containers/suite.go
package containers

import (
    "context"
    "testing"
    
    "github.com/stretchr/testify/require"
)

type TestSuite struct {
    Postgres *PostgresContainer
    Redis    *RedisContainer
    Kafka    *KafkaContainer
}

// SuiteOption configures which containers to start
type SuiteOption func(*suiteConfig)

type suiteConfig struct {
    postgres bool
    redis    bool
    kafka    bool
}

func WithPostgres() SuiteOption {
    return func(c *suiteConfig) {
        c.postgres = true
    }
}

func WithRedis() SuiteOption {
    return func(c *suiteConfig) {
        c.redis = true
    }
}

func WithKafka() SuiteOption {
    return func(c *suiteConfig) {
        c.kafka = true
    }
}

// NewTestSuite creates containers based on options
func NewTestSuite(t *testing.T, opts ...SuiteOption) *TestSuite {
    ctx := context.Background()
    config := &suiteConfig{}
    
    for _, opt := range opts {
        opt(config)
    }
    
    suite := &TestSuite{}
    
    if config.postgres {
        pg, err := NewPostgresContainerWithSnapshot(ctx)
        require.NoError(t, err)
        suite.Postgres = pg
    }
    
    if config.redis {
        redis, err := NewRedisContainer(ctx)
        require.NoError(t, err)
        suite.Redis = redis
    }
    
    if config.kafka {
        kafka, err := NewKafkaContainer(ctx)
        require.NoError(t, err)
        suite.Kafka = kafka
    }
    
    return suite
}

// Cleanup terminates all containers
func (s *TestSuite) Cleanup(ctx context.Context) {
    if s.Postgres != nil {
        s.Postgres.Terminate(ctx)
    }
    if s.Redis != nil {
        s.Redis.Terminate(ctx)
    }
    if s.Kafka != nil {
        s.Kafka.Terminate(ctx)
    }
}

// Reset restores all containers to clean state
func (s *TestSuite) Reset(ctx context.Context) error {
    if s.Postgres != nil {
        if err := s.Postgres.RestoreSnapshot(ctx); err != nil {
            return err
        }
    }
    if s.Redis != nil {
        if err := s.Redis.FlushAll(ctx); err != nil {
            return err
        }
    }
    // Kafka doesn't need reset for most tests
    return nil
}
```

## Advanced Features

### Container Reuse

Container reuse significantly speeds up test execution by reusing containers between test runs:

```go
// Enable reuse globally
testcontainers.WithReuse(true)

// Or per container
pgContainer, err := postgres.Run(ctx,
    "postgres:16-alpine",
    testcontainers.WithReuse(true),
)

// Containers are reused if:
// 1. Image matches
// 2. Environment variables match
// 3. Exposed ports match
// 4. Container is still running
```

### Custom Networks

Create isolated networks for multi-container tests:

```go
// Create custom network
network, err := testcontainers.GenericNetwork(ctx, testcontainers.GenericNetworkRequest{
    NetworkRequest: testcontainers.NetworkRequest{
        Name:   "test-network",
        Driver: "bridge",
    },
})

// Attach containers to network
pgContainer, err := postgres.Run(ctx,
    "postgres:16-alpine",
    network.WithNetwork([]string{"postgres"}),
)
```

### Parallel Container Creation

Speed up test initialization with parallel container creation:

```go
func NewTestSuiteParallel(ctx context.Context) (*TestSuite, error) {
    suite := &TestSuite{}
    eg, ctx := errgroup.WithContext(ctx)
    
    // Start PostgreSQL
    eg.Go(func() error {
        pg, err := NewPostgresContainer(ctx)
        if err != nil {
            return err
        }
        suite.Postgres = pg
        return nil
    })
    
    // Start Redis
    eg.Go(func() error {
        redis, err := NewRedisContainer(ctx)
        if err != nil {
            return err
        }
        suite.Redis = redis
        return nil
    })
    
    // Wait for all containers
    if err := eg.Wait(); err != nil {
        return nil, err
    }
    
    return suite, nil
}
```

### Resource Limits

Control container resource usage:

```go
pgContainer, err := postgres.Run(ctx,
    "postgres:16-alpine",
    testcontainers.WithResources(testcontainers.ContainerResources{
        Memory:     1024 * 1024 * 1024,    // 1GB
        MemorySwap: 1024 * 1024 * 1024,    // 1GB
        CPUShares:  512,                    // Half CPU
    }),
)
```

### File Operations

Copy files to/from containers:

```go
// Copy file into container
err := container.CopyFileToContainer(ctx, 
    "/local/path/data.sql", 
    "/container/path/data.sql", 
    0644,
)

// Copy directory
err := container.CopyDirToContainer(ctx,
    "/local/config",
    "/etc/app",
    0755,
)

// Read file from container
reader, err := container.CopyFileFromContainer(ctx, "/container/logs/app.log")
```

### Environment Variables

Pass configuration via environment:

```go
pgContainer, err := postgres.Run(ctx,
    "postgres:16-alpine",
    testcontainers.WithEnv(map[string]string{
        "POSTGRES_SHARED_BUFFERS": "256MB",
        "POSTGRES_WORK_MEM":       "4MB",
        "POSTGRES_MAX_CONNECTIONS": "200",
    }),
)
```

## Integration Patterns

### Pattern 1: Shared Container for Test Package

```go
package repository_test

var (
    pgContainer *containers.PostgresContainer
    testDB      *sql.DB
)

func TestMain(m *testing.M) {
    ctx := context.Background()
    
    // Start container once for all tests
    pg, err := containers.NewPostgresContainer(ctx)
    if err != nil {
        log.Fatal(err)
    }
    pgContainer = pg
    testDB = pg.DB
    
    // Run tests
    code := m.Run()
    
    // Cleanup
    pgContainer.Terminate(ctx)
    os.Exit(code)
}
```

### Pattern 2: Test Helper with Cleanup

```go
func setupTestDB(t *testing.T) (*sql.DB, func()) {
    ctx := context.Background()
    container, err := containers.NewPostgresContainer(ctx)
    require.NoError(t, err)
    
    cleanup := func() {
        container.Terminate(ctx)
    }
    
    // Register cleanup to run even if test panics
    t.Cleanup(cleanup)
    
    return container.DB, cleanup
}

// Usage
func TestRepository(t *testing.T) {
    db, cleanup := setupTestDB(t)
    defer cleanup()
    
    // Run tests with db
}
```

### Pattern 3: Test Fixtures with Containers

```go
type RepositoryTestFixture struct {
    Container *containers.PostgresContainer
    DB        *sql.DB
    Repo      *repository.CallRepository
}

func NewRepositoryTestFixture(t *testing.T) *RepositoryTestFixture {
    ctx := context.Background()
    container, err := containers.NewPostgresContainerWithSnapshot(ctx)
    require.NoError(t, err)
    
    t.Cleanup(func() {
        container.Terminate(ctx)
    })
    
    return &RepositoryTestFixture{
        Container: container,
        DB:        container.DB,
        Repo:      repository.NewCallRepository(container.DB),
    }
}

func (f *RepositoryTestFixture) Reset(ctx context.Context) {
    f.Container.RestoreSnapshot(ctx)
}
```

## Performance Optimization

### 1. Container Reuse Strategies

```go
// Global container reuse with proper labeling
func NewReusablePostgres(ctx context.Context) (*PostgresContainer, error) {
    return postgres.Run(ctx,
        "postgres:16-alpine",
        testcontainers.WithReuse(true),
        testcontainers.WithLabels(map[string]string{
            "project":     "dce-backend",
            "test-type":   "integration",
            "reuse-group": "postgres-main",
        }),
    )
}
```

### 2. Lazy Container Initialization

```go
var (
    postgresOnce      sync.Once
    postgresContainer *containers.PostgresContainer
    postgresErr       error
)

func GetSharedPostgres(ctx context.Context) (*containers.PostgresContainer, error) {
    postgresOnce.Do(func() {
        postgresContainer, postgresErr = containers.NewPostgresContainer(ctx)
    })
    return postgresContainer, postgresErr
}
```

### 3. Connection Pool Optimization

```go
func NewOptimizedPostgres(ctx context.Context) (*PostgresContainer, error) {
    container, err := NewPostgresContainer(ctx)
    if err != nil {
        return nil, err
    }
    
    // Optimize connection pool for tests
    container.DB.SetMaxOpenConns(25)
    container.DB.SetMaxIdleConns(10)
    container.DB.SetConnMaxLifetime(5 * time.Minute)
    container.DB.SetConnMaxIdleTime(1 * time.Minute)
    
    return container, nil
}
```

### 4. Parallel Test Execution

```go
func TestRepositoryParallel(t *testing.T) {
    ctx := context.Background()
    container, err := containers.NewPostgresContainerWithSnapshot(ctx)
    require.NoError(t, err)
    defer container.Terminate(ctx)
    
    tests := []struct {
        name string
        test func(t *testing.T)
    }{
        {"test1", testFunc1},
        {"test2", testFunc2},
        {"test3", testFunc3},
    }
    
    for _, tt := range tests {
        tt := tt // Capture range variable
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel() // Run tests in parallel
            
            // Each parallel test gets clean state
            require.NoError(t, container.RestoreSnapshot(ctx))
            tt.test(t)
        })
    }
}
```

### 5. Resource Cleanup Patterns

```go
// Ensure cleanup even on panic
func WithContainer(t *testing.T, fn func(*containers.PostgresContainer)) {
    ctx := context.Background()
    container, err := containers.NewPostgresContainer(ctx)
    require.NoError(t, err)
    
    defer func() {
        if r := recover(); r != nil {
            container.Terminate(ctx)
            panic(r)
        }
        container.Terminate(ctx)
    }()
    
    fn(container)
}
```

## Troubleshooting

### Common Issues and Solutions

#### 1. Container Startup Timeouts

```go
// Increase startup timeout
wait.ForLog("ready").WithStartupTimeout(2 * time.Minute)

// Add retry logic
var container *postgres.PostgresContainer
for i := 0; i < 3; i++ {
    container, err = postgres.Run(ctx, "postgres:16-alpine")
    if err == nil {
        break
    }
    time.Sleep(5 * time.Second)
}
```

#### 2. Docker Socket Permission Errors

```bash
# Linux: Add user to docker group
sudo usermod -aG docker $USER

# macOS: Ensure Docker Desktop is running
open -a Docker
```

#### 3. Resource Cleanup Issues

```go
// Force cleanup on test failure
func TestWithCleanup(t *testing.T) {
    ctx := context.Background()
    container, _ := containers.NewPostgresContainer(ctx)
    
    // Always clean up, even on test failure
    t.Cleanup(func() {
        if err := container.Terminate(ctx); err != nil {
            t.Logf("cleanup failed: %v", err)
        }
    })
    
    // Test code here
}
```

#### 4. Container Reuse Conflicts

```go
// Use unique labels to avoid conflicts
testcontainers.WithLabels(map[string]string{
    "test-id":    uuid.New().String(),
    "test-suite": t.Name(),
})
```

### Debugging Container Issues

```go
// Enable debug logging
func init() {
    if testing.Verbose() {
        testcontainers.Logger = log.New(os.Stderr, "testcontainers: ", log.LstdFlags)
    }
}

// Get container logs
logs, err := container.Logs(ctx)
if err == nil {
    buf := new(bytes.Buffer)
    buf.ReadFrom(logs)
    t.Logf("Container logs:\n%s", buf.String())
}

// Inspect container state
state, err := container.State(ctx)
t.Logf("Container state: %+v", state)
```

### CI/CD Considerations

1. **Docker-in-Docker**: Enable for GitLab CI/Kubernetes
2. **Resource Limits**: Set appropriate limits for CI environments
3. **Cleanup**: Ensure containers are terminated after tests
4. **Caching**: Use registry mirrors for faster image pulls

```yaml
# .github/workflows/test.yml
- name: Run Integration Tests
  env:
    TESTCONTAINERS_RYUK_DISABLED: "false"
    DOCKER_HOST: "unix:///var/run/docker.sock"
  run: |
    go test -tags=integration -v ./...
```

## Best Practices Summary

1. **Always Use Cleanup**: Register cleanup with `t.Cleanup()` or `defer`
2. **Enable Reuse**: Use `WithReuse(true)` for faster subsequent runs
3. **Use Snapshots**: For database containers, leverage snapshots
4. **Parallel Safety**: Ensure containers are isolated for parallel tests
5. **Resource Management**: Set appropriate limits and connection pools
6. **Error Handling**: Always check errors from container operations
7. **Logging**: Enable debug logging when troubleshooting

## Version History

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2025-06-09 | Initial Testcontainers guide |

## See Also

- [Database Testing Guide](01-database-testing-guide.md)
- [Testing Patterns](04-testing-patterns.md)
- [Testcontainers Documentation](https://golang.testcontainers.org/)
