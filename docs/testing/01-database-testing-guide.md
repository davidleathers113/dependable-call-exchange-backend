# Database Testing Guide

**Version:** 1.0.0  
**Date:** June 9, 2025  
**Status:** Active

## Table of Contents
- [Introduction](#introduction)
- [Database Testing Strategy](#database-testing-strategy)
- [Test Infrastructure](#test-infrastructure)
- [Testing Patterns](#testing-patterns)
- [Repository Testing](#repository-testing)
- [Performance Testing](#performance-testing)
- [Best Practices](#best-practices)
- [Troubleshooting](#troubleshooting)

## Introduction

This guide covers comprehensive database testing strategies for the Dependable Call Exchange Backend. We use PostgreSQL as our primary database and employ modern testing techniques to ensure data integrity, query performance, and schema reliability.

## Database Testing Strategy

### Core Principles

1. **Test Against Real Databases**
   - Never mock database interactions at the repository level
   - Use actual PostgreSQL instances via Testcontainers
   - Validate actual SQL execution and constraints

2. **Fast Test Execution**
   - Database snapshots for instant resets
   - Container reuse between test runs
   - Parallel test execution with isolation

3. **Comprehensive Coverage**
   - CRUD operations
   - Complex queries and joins
   - Constraint validation
   - Transaction behavior
   - Performance characteristics

### Testing Layers

```
┌─────────────────────────────────────┐
│     Repository Integration Tests     │
│   (Real DB, full SQL validation)    │
├─────────────────────────────────────┤
│       Migration Tests               │
│  (Schema evolution, rollbacks)      │
├─────────────────────────────────────┤
│     Performance Benchmarks          │
│   (Query optimization, load)        │
└─────────────────────────────────────┘
```

## Test Infrastructure

### PostgreSQL Test Container

```go
// internal/testutil/containers/postgres.go
package containers

import (
    "context"
    "fmt"
    "time"
    
    "github.com/testcontainers/testcontainers-go"
    "github.com/testcontainers/testcontainers-go/modules/postgres"
    "github.com/testcontainers/testcontainers-go/wait"
)

type PostgresContainer struct {
    *postgres.PostgresContainer
    ConnectionString string
    DatabaseName     string
}

func NewPostgresContainer(ctx context.Context) (*PostgresContainer, error) {
    pgContainer, err := postgres.Run(ctx,
        "postgres:16-alpine",
        postgres.WithDatabase("dce_test"),
        postgres.WithUsername("postgres"),
        postgres.WithPassword("postgres"),
        postgres.WithInitScripts("../../migrations/001_initial_schema.sql"),
        postgres.WithWaitStrategy(
            wait.ForLog("database system is ready to accept connections").
                WithOccurrence(2).
                WithStartupTimeout(30*time.Second),
        ),
        testcontainers.WithReuse(true),
        postgres.WithSnapshotName("clean-state"),
    )
    
    if err != nil {
        return nil, fmt.Errorf("failed to start postgres: %w", err)
    }
    
    connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
    if err != nil {
        return nil, fmt.Errorf("failed to get connection string: %w", err)
    }
    
    return &PostgresContainer{
        PostgresContainer: pgContainer,
        ConnectionString:  connStr,
        DatabaseName:     "dce_test",
    }, nil
}
```

### Test Database Helper

```go
// internal/testutil/database_helper.go
package testutil

import (
    "database/sql"
    "testing"
    
    "github.com/stretchr/testify/require"
)

type DatabaseHelper struct {
    t  *testing.T
    db *sql.DB
}

func NewDatabaseHelper(t *testing.T, db *sql.DB) *DatabaseHelper {
    return &DatabaseHelper{t: t, db: db}
}

// AssertRowCount verifies the number of rows in a table
func (h *DatabaseHelper) AssertRowCount(table string, expected int) {
    var count int
    query := fmt.Sprintf("SELECT COUNT(*) FROM %s", table)
    err := h.db.QueryRow(query).Scan(&count)
    require.NoError(h.t, err)
    require.Equal(h.t, expected, count, 
        "table %s should have %d rows but has %d", table, expected, count)
}

// TruncateAllTables clears all data while preserving schema
func (h *DatabaseHelper) TruncateAllTables() {
    tables := []string{
        "bids", "calls", "accounts", "compliance_rules", "consent_records",
    }
    
    _, err := h.db.Exec("SET CONSTRAINTS ALL DEFERRED")
    require.NoError(h.t, err)
    
    for _, table := range tables {
        _, err := h.db.Exec(fmt.Sprintf("TRUNCATE TABLE %s CASCADE", table))
        require.NoError(h.t, err)
    }
}
```

## Testing Patterns

### Pattern 1: Table-Driven Repository Tests

```go
func TestCallRepository_CRUD(t *testing.T) {
    ctx := context.Background()
    container, err := containers.NewPostgresContainer(ctx)
    require.NoError(t, err)
    defer container.Terminate(ctx)
    
    db, err := sql.Open("postgres", container.ConnectionString)
    require.NoError(t, err)
    defer db.Close()
    
    repo := repository.NewCallRepository(db)
    helper := testutil.NewDatabaseHelper(t, db)
    
    tests := []struct {
        name     string
        setup    func()
        test     func(t *testing.T)
        validate func(t *testing.T)
    }{
        {
            name: "create and retrieve call",
            setup: func() {
                helper.TruncateAllTables()
            },
            test: func(t *testing.T) {
                call := &domain.Call{
                    ID:         uuid.New(),
                    FromNumber: "+15551234567",
                    ToNumber:   "+15559876543",
                    Status:     domain.CallStatusPending,
                    Type:       domain.CallTypeInbound,
                }
                
                err := repo.Create(ctx, call)
                require.NoError(t, err)
                
                retrieved, err := repo.GetByID(ctx, call.ID)
                require.NoError(t, err)
                assert.Equal(t, call.ID, retrieved.ID)
                assert.Equal(t, call.FromNumber, retrieved.FromNumber)
            },
            validate: func(t *testing.T) {
                helper.AssertRowCount("calls", 1)
            },
        },
        {
            name: "update call status",
            setup: func() {
                helper.TruncateAllTables()
                // Insert test call
                _, err := db.Exec(`
                    INSERT INTO calls (id, from_number, to_number, status, type)
                    VALUES ($1, $2, $3, $4, $5)`,
                    testCallID, "+15551234567", "+15559876543", 
                    "pending", "inbound",
                )
                require.NoError(t, err)
            },
            test: func(t *testing.T) {
                err := repo.UpdateStatus(ctx, testCallID, domain.CallStatusActive)
                require.NoError(t, err)
                
                call, err := repo.GetByID(ctx, testCallID)
                require.NoError(t, err)
                assert.Equal(t, domain.CallStatusActive, call.Status)
            },
            validate: func(t *testing.T) {
                var status string
                err := db.QueryRow(
                    "SELECT status FROM calls WHERE id = $1", 
                    testCallID,
                ).Scan(&status)
                require.NoError(t, err)
                assert.Equal(t, "active", status)
            },
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            tt.setup()
            tt.test(t)
            tt.validate(t)
        })
    }
}
```

### Pattern 2: Transactional Test Isolation

```go
func TestCallRepository_TransactionIsolation(t *testing.T) {
    ctx := context.Background()
    container, err := containers.NewPostgresContainer(ctx)
    require.NoError(t, err)
    defer container.Terminate(ctx)
    
    db, err := sql.Open("postgres", container.ConnectionString)
    require.NoError(t, err)
    defer db.Close()
    
    t.Run("rollback on error", func(t *testing.T) {
        tx, err := db.BeginTx(ctx, nil)
        require.NoError(t, err)
        defer tx.Rollback()
        
        repo := repository.NewCallRepositoryWithTx(tx)
        
        // This should succeed
        call1 := createTestCall()
        err = repo.Create(ctx, call1)
        require.NoError(t, err)
        
        // This should fail (duplicate ID)
        call2 := createTestCall()
        call2.ID = call1.ID
        err = repo.Create(ctx, call2)
        require.Error(t, err)
        
        // Rollback
        tx.Rollback()
        
        // Verify nothing was persisted
        var count int
        err = db.QueryRow("SELECT COUNT(*) FROM calls").Scan(&count)
        require.NoError(t, err)
        assert.Equal(t, 0, count)
    })
}
```

### Pattern 3: Bulk Operations Testing

```go
func TestCallRepository_BulkInsert(t *testing.T) {
    ctx := context.Background()
    container, err := containers.NewPostgresContainer(ctx)
    require.NoError(t, err)
    defer container.Terminate(ctx)
    
    db, err := sql.Open("postgres", container.ConnectionString)
    require.NoError(t, err)
    defer db.Close()
    
    repo := repository.NewCallRepository(db)
    
    // Create 1000 test calls
    calls := make([]*domain.Call, 1000)
    for i := range calls {
        calls[i] = &domain.Call{
            ID:         uuid.New(),
            FromNumber: fmt.Sprintf("+1555%07d", i),
            ToNumber:   fmt.Sprintf("+1666%07d", i),
            Status:     domain.CallStatusPending,
            Type:       domain.CallTypeInbound,
        }
    }
    
    // Measure bulk insert performance
    start := time.Now()
    err = repo.BulkInsert(ctx, calls)
    duration := time.Since(start)
    
    require.NoError(t, err)
    
    // Verify all inserted
    var count int
    err = db.QueryRow("SELECT COUNT(*) FROM calls").Scan(&count)
    require.NoError(t, err)
    assert.Equal(t, 1000, count)
    
    // Performance assertion
    assert.Less(t, duration, 5*time.Second, 
        "Bulk insert of 1000 calls should complete within 5 seconds")
}
```

## Repository Testing

### Testing Complex Queries

```go
func TestCallRepository_ComplexQueries(t *testing.T) {
    ctx := context.Background()
    container, err := containers.NewPostgresContainer(ctx)
    require.NoError(t, err)
    defer container.Terminate(ctx)
    
    db, err := sql.Open("postgres", container.ConnectionString)
    require.NoError(t, err)
    
    repo := repository.NewCallRepository(db)
    
    // Seed test data
    seedCallData(t, db)
    
    t.Run("find calls with filters", func(t *testing.T) {
        filters := repository.CallFilters{
            Status:    domain.CallStatusActive,
            DateFrom:  time.Now().Add(-24 * time.Hour),
            DateTo:    time.Now(),
            BuyerID:   &testBuyerID,
            Limit:     10,
            Offset:    0,
        }
        
        calls, total, err := repo.FindWithFilters(ctx, filters)
        require.NoError(t, err)
        
        assert.Len(t, calls, 10)
        assert.Greater(t, total, int64(10))
        
        // Verify all returned calls match filters
        for _, call := range calls {
            assert.Equal(t, domain.CallStatusActive, call.Status)
            assert.Equal(t, testBuyerID, *call.BuyerID)
            assert.True(t, call.CreatedAt.After(filters.DateFrom))
            assert.True(t, call.CreatedAt.Before(filters.DateTo))
        }
    })
    
    t.Run("aggregate call statistics", func(t *testing.T) {
        stats, err := repo.GetCallStatistics(ctx, testBuyerID, 
            time.Now().Add(-7*24*time.Hour), time.Now())
        require.NoError(t, err)
        
        assert.Greater(t, stats.TotalCalls, 0)
        assert.Greater(t, stats.TotalDuration, 0)
        assert.Greater(t, stats.TotalCost, 0.0)
        assert.Greater(t, stats.AverageDuration, 0.0)
    })
}
```

### Testing Database Constraints

```go
func TestCallRepository_Constraints(t *testing.T) {
    ctx := context.Background()
    container, err := containers.NewPostgresContainer(ctx)
    require.NoError(t, err)
    defer container.Terminate(ctx)
    
    db, err := sql.Open("postgres", container.ConnectionString)
    require.NoError(t, err)
    
    repo := repository.NewCallRepository(db)
    
    t.Run("foreign key constraint", func(t *testing.T) {
        call := &domain.Call{
            ID:         uuid.New(),
            FromNumber: "+15551234567",
            ToNumber:   "+15559876543",
            BuyerID:    uuid.New(), // Non-existent buyer
            Status:     domain.CallStatusPending,
        }
        
        err := repo.Create(ctx, call)
        require.Error(t, err)
        assert.Contains(t, err.Error(), "foreign key constraint")
    })
    
    t.Run("check constraint", func(t *testing.T) {
        // Assuming we have a check constraint on phone number format
        call := &domain.Call{
            ID:         uuid.New(),
            FromNumber: "invalid-phone",
            ToNumber:   "+15559876543",
            Status:     domain.CallStatusPending,
        }
        
        err := repo.Create(ctx, call)
        require.Error(t, err)
        assert.Contains(t, err.Error(), "check constraint")
    })
}
```

## Performance Testing

### Query Performance Benchmarks

```go
func BenchmarkCallRepository_FindByStatus(b *testing.B) {
    ctx := context.Background()
    container, _ := containers.NewPostgresContainer(ctx)
    defer container.Terminate(ctx)
    
    db, _ := sql.Open("postgres", container.ConnectionString)
    repo := repository.NewCallRepository(db)
    
    // Seed with realistic data volume
    seedLargeDataset(b, db, 100000) // 100k calls
    
    b.ResetTimer()
    
    for i := 0; i < b.N; i++ {
        calls, err := repo.FindByStatus(ctx, domain.CallStatusActive, 100, 0)
        if err != nil {
            b.Fatal(err)
        }
        if len(calls) == 0 {
            b.Fatal("expected results")
        }
    }
    
    b.ReportMetric(float64(100), "calls/op")
}

func BenchmarkCallRepository_BulkInsert(b *testing.B) {
    ctx := context.Background()
    container, _ := containers.NewPostgresContainer(ctx)
    defer container.Terminate(ctx)
    
    db, _ := sql.Open("postgres", container.ConnectionString)
    repo := repository.NewCallRepository(db)
    
    // Prepare test data
    calls := generateTestCalls(1000)
    
    b.ResetTimer()
    
    for i := 0; i < b.N; i++ {
        // Reset database
        container.Restore(ctx)
        
        err := repo.BulkInsert(ctx, calls)
        if err != nil {
            b.Fatal(err)
        }
    }
    
    b.ReportMetric(float64(len(calls)), "calls/op")
}
```

### Load Testing

```go
func TestCallRepository_ConcurrentLoad(t *testing.T) {
    ctx := context.Background()
    container, err := containers.NewPostgresContainer(ctx)
    require.NoError(t, err)
    defer container.Terminate(ctx)
    
    db, err := sql.Open("postgres", container.ConnectionString)
    require.NoError(t, err)
    
    // Configure connection pool for load testing
    db.SetMaxOpenConns(50)
    db.SetMaxIdleConns(25)
    
    repo := repository.NewCallRepository(db)
    
    // Run concurrent operations
    const numGoroutines = 20
    const operationsPerGoroutine = 100
    
    var wg sync.WaitGroup
    errors := make(chan error, numGoroutines*operationsPerGoroutine)
    
    for i := 0; i < numGoroutines; i++ {
        wg.Add(1)
        go func(workerID int) {
            defer wg.Done()
            
            for j := 0; j < operationsPerGoroutine; j++ {
                call := &domain.Call{
                    ID:         uuid.New(),
                    FromNumber: fmt.Sprintf("+1555%03d%04d", workerID, j),
                    ToNumber:   "+15559876543",
                    Status:     domain.CallStatusPending,
                }
                
                if err := repo.Create(ctx, call); err != nil {
                    errors <- err
                    return
                }
                
                // Immediately query it back
                if _, err := repo.GetByID(ctx, call.ID); err != nil {
                    errors <- err
                    return
                }
            }
        }(i)
    }
    
    wg.Wait()
    close(errors)
    
    // Check for errors
    var errorCount int
    for err := range errors {
        t.Errorf("concurrent operation failed: %v", err)
        errorCount++
    }
    
    assert.Equal(t, 0, errorCount, "all concurrent operations should succeed")
    
    // Verify total count
    var count int
    err = db.QueryRow("SELECT COUNT(*) FROM calls").Scan(&count)
    require.NoError(t, err)
    assert.Equal(t, numGoroutines*operationsPerGoroutine, count)
}
```

## Best Practices

### 1. Test Data Management

```go
// Use builders for consistent test data
type CallBuilder struct {
    call *domain.Call
}

func NewCallBuilder() *CallBuilder {
    return &CallBuilder{
        call: &domain.Call{
            ID:         uuid.New(),
            FromNumber: "+15551234567",
            ToNumber:   "+15559876543",
            Status:     domain.CallStatusPending,
            Type:       domain.CallTypeInbound,
            CreatedAt:  time.Now(),
            UpdatedAt:  time.Now(),
        },
    }
}

func (b *CallBuilder) WithStatus(status domain.CallStatus) *CallBuilder {
    b.call.Status = status
    return b
}

func (b *CallBuilder) WithBuyer(buyerID uuid.UUID) *CallBuilder {
    b.call.BuyerID = &buyerID
    return b
}

func (b *CallBuilder) Build() *domain.Call {
    return b.call
}
```

### 2. Database State Verification

```go
// Helper functions for common assertions
func AssertCallExists(t *testing.T, db *sql.DB, callID uuid.UUID) {
    var exists bool
    err := db.QueryRow(
        "SELECT EXISTS(SELECT 1 FROM calls WHERE id = $1)", 
        callID,
    ).Scan(&exists)
    require.NoError(t, err)
    assert.True(t, exists, "call %s should exist", callID)
}

func AssertCallStatus(t *testing.T, db *sql.DB, callID uuid.UUID, expected string) {
    var status string
    err := db.QueryRow(
        "SELECT status FROM calls WHERE id = $1", 
        callID,
    ).Scan(&status)
    require.NoError(t, err)
    assert.Equal(t, expected, status)
}
```

### 3. Test Organization

```go
// Group related tests in test suites
type CallRepositoryTestSuite struct {
    suite.Suite
    container *containers.PostgresContainer
    db        *sql.DB
    repo      *repository.CallRepository
}

func (s *CallRepositoryTestSuite) SetupSuite() {
    ctx := context.Background()
    container, err := containers.NewPostgresContainer(ctx)
    s.Require().NoError(err)
    s.container = container
    
    db, err := sql.Open("postgres", container.ConnectionString)
    s.Require().NoError(err)
    s.db = db
    
    s.repo = repository.NewCallRepository(db)
}

func (s *CallRepositoryTestSuite) TearDownSuite() {
    s.db.Close()
    s.container.Terminate(context.Background())
}

func (s *CallRepositoryTestSuite) SetupTest() {
    // Clean database before each test
    s.container.Restore(context.Background())
}

func TestCallRepositorySuite(t *testing.T) {
    suite.Run(t, new(CallRepositoryTestSuite))
}
```

### 4. Performance Considerations

- **Use snapshots**: Restore clean state instead of recreating containers
- **Reuse containers**: Enable container reuse for faster test runs
- **Parallel execution**: Run independent tests in parallel
- **Connection pooling**: Configure appropriate pool sizes for tests
- **Bulk operations**: Test with realistic data volumes

### 5. Error Handling

```go
// Test error scenarios explicitly
func TestCallRepository_ErrorHandling(t *testing.T) {
    ctx := context.Background()
    container, err := containers.NewPostgresContainer(ctx)
    require.NoError(t, err)
    defer container.Terminate(ctx)
    
    db, err := sql.Open("postgres", container.ConnectionString)
    require.NoError(t, err)
    
    repo := repository.NewCallRepository(db)
    
    t.Run("handle connection errors", func(t *testing.T) {
        // Close the database to simulate connection error
        db.Close()
        
        _, err := repo.GetByID(ctx, uuid.New())
        require.Error(t, err)
        assert.Contains(t, err.Error(), "database is closed")
    })
    
    t.Run("handle query timeout", func(t *testing.T) {
        // Create new connection
        db, _ = sql.Open("postgres", container.ConnectionString)
        repo = repository.NewCallRepository(db)
        
        // Use very short timeout
        ctx, cancel := context.WithTimeout(ctx, 1*time.Nanosecond)
        defer cancel()
        
        _, err := repo.GetByID(ctx, uuid.New())
        require.Error(t, err)
        assert.True(t, errors.Is(err, context.DeadlineExceeded))
    })
}
```

## Troubleshooting

### Common Issues

1. **Container startup failures**
   ```bash
   # Check Docker daemon
   docker ps
   
   # Check resource limits
   docker system df
   
   # Clean up old containers
   docker system prune
   ```

2. **Slow test execution**
   - Enable container reuse: `testcontainers.WithReuse(true)`
   - Use snapshots instead of recreation
   - Run tests in parallel where possible

3. **Database connection errors**
   ```go
   // Add retry logic for container readiness
   wait.ForLog("database system is ready to accept connections").
       WithOccurrence(2).
       WithStartupTimeout(60*time.Second)
   ```

4. **Inconsistent test results**
   - Ensure proper test isolation
   - Use deterministic test data
   - Clear database state between tests

### Debugging Database Tests

```go
// Enable query logging for debugging
func EnableQueryLogging(db *sql.DB) {
    db.Exec("SET log_statement = 'all'")
    db.Exec("SET log_duration = on")
}

// Capture and print query execution plans
func ExplainQuery(t *testing.T, db *sql.DB, query string, args ...interface{}) {
    rows, err := db.Query("EXPLAIN ANALYZE " + query, args...)
    require.NoError(t, err)
    defer rows.Close()
    
    t.Log("Query execution plan:")
    for rows.Next() {
        var plan string
        rows.Scan(&plan)
        t.Log(plan)
    }
}
```

### Performance Profiling

```go
// Profile slow queries during tests
func ProfileSlowQueries(t *testing.T, db *sql.DB) {
    rows, err := db.Query(`
        SELECT query, mean_exec_time, calls
        FROM pg_stat_statements
        WHERE mean_exec_time > 100
        ORDER BY mean_exec_time DESC
        LIMIT 10
    `)
    require.NoError(t, err)
    defer rows.Close()
    
    t.Log("Slow queries detected:")
    for rows.Next() {
        var query string
        var meanTime float64
        var calls int64
        rows.Scan(&query, &meanTime, &calls)
        t.Logf("Query: %s, Avg Time: %.2fms, Calls: %d", 
            query, meanTime, calls)
    }
}
```

## Version History

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2025-06-09 | Initial database testing guide |

## See Also

- [Testcontainers Guide](02-testcontainers-guide.md)
- [Migration Testing](03-migration-testing.md)
- [Testing Patterns](04-testing-patterns.md)
