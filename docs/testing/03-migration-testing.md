# Migration Testing Guide

**Version:** 1.0.0  
**Date:** June 9, 2025  
**Status:** Active

## Table of Contents
- [Introduction](#introduction)
- [Migration Testing Strategy](#migration-testing-strategy)
- [Test Infrastructure](#test-infrastructure)
- [Migration Test Patterns](#migration-test-patterns)
- [Schema Validation](#schema-validation)
- [Data Migration Testing](#data-migration-testing)
- [CI/CD Integration](#cicd-integration)
- [Best Practices](#best-practices)

## Introduction

Database migrations are critical operations that modify schema structure and data. This guide covers comprehensive testing strategies to ensure migrations are safe, reversible, and maintain data integrity.

### Why Test Migrations?

1. **Prevent Data Loss**: Ensure migrations don't accidentally drop data
2. **Maintain Integrity**: Validate constraints and relationships
3. **Ensure Reversibility**: Verify rollback procedures work
4. **Performance Impact**: Assess migration execution time
5. **Compatibility**: Ensure application compatibility during migration

## Migration Testing Strategy

### Core Testing Principles

1. **Test Both Directions**: Always test up and down migrations
2. **Use Production-Like Data**: Test with realistic data volumes
3. **Validate Schema**: Ensure schema matches expectations
4. **Check Performance**: Monitor migration execution time
5. **Test Idempotency**: Migrations should be safe to run multiple times

### Testing Layers

```
┌─────────────────────────────────────────┐
│        Schema Migration Tests           │
│    (Structure changes, constraints)     │
├─────────────────────────────────────────┤
│        Data Migration Tests             │
│    (Data transformation, integrity)     │
├─────────────────────────────────────────┤
│     Performance & Load Tests            │
│    (Large dataset migrations)           │
└─────────────────────────────────────────┘
```

## Test Infrastructure

### Migration Test Setup

```go
// internal/testutil/migration/helper.go
package migration

import (
    "context"
    "database/sql"
    "fmt"
    "testing"
    
    "github.com/golang-migrate/migrate/v4"
    "github.com/golang-migrate/migrate/v4/database/postgres"
    _ "github.com/golang-migrate/migrate/v4/source/file"
    "github.com/stretchr/testify/require"
    "github.com/testcontainers/testcontainers-go"
    postgresModule "github.com/testcontainers/testcontainers-go/modules/postgres"
)

type MigrationHelper struct {
    Container        *postgresModule.PostgresContainer
    DB              *sql.DB
    Migrate         *migrate.Migrate
    ConnectionString string
}

func NewMigrationHelper(t *testing.T) *MigrationHelper {
    ctx := context.Background()
    
    // Start clean PostgreSQL container
    container, err := postgresModule.Run(ctx,
        "postgres:16-alpine",
        postgresModule.WithDatabase("migration_test"),
        postgresModule.WithUsername("postgres"),
        postgresModule.WithPassword("postgres"),
        testcontainers.WithReuse(true),
    )
    require.NoError(t, err)
    
    connStr, err := container.ConnectionString(ctx, "sslmode=disable")
    require.NoError(t, err)
    
    db, err := sql.Open("postgres", connStr)
    require.NoError(t, err)
    
    // Create migrate instance
    driver, err := postgres.WithInstance(db, &postgres.Config{})
    require.NoError(t, err)
    
    m, err := migrate.NewWithDatabaseInstance(
        "file://../../migrations",
        "postgres", driver)
    require.NoError(t, err)
    
    helper := &MigrationHelper{
        Container:        container,
        DB:              db,
        Migrate:         m,
        ConnectionString: connStr,
    }
    
    t.Cleanup(func() {
        m.Close()
        db.Close()
        container.Terminate(ctx)
    })
    
    return helper
}

// GetVersion returns current migration version
func (h *MigrationHelper) GetVersion() (uint, bool, error) {
    return h.Migrate.Version()
}

// RunMigration executes a specific migration version
func (h *MigrationHelper) RunMigration(version int) error {
    return h.Migrate.Migrate(uint(version))
}
```

## Migration Test Patterns

### Pattern 1: Basic Migration Testing

```go
func TestMigrations_UpAndDown(t *testing.T) {
    helper := NewMigrationHelper(t)
    
    t.Run("all migrations up", func(t *testing.T) {
        // Run all migrations up
        err := helper.Migrate.Up()
        require.NoError(t, err)
        
        // Verify final version
        version, dirty, err := helper.GetVersion()
        require.NoError(t, err)
        require.False(t, dirty)
        require.Greater(t, version, uint(0))
    })
    
    t.Run("all migrations down", func(t *testing.T) {
        // Run all migrations down
        err := helper.Migrate.Down()
        require.NoError(t, err)
        
        // Verify database is empty
        var tableCount int
        err = helper.DB.QueryRow(`
            SELECT COUNT(*) 
            FROM information_schema.tables 
            WHERE table_schema = 'public'
        `).Scan(&tableCount)
        require.NoError(t, err)
        assert.Equal(t, 0, tableCount)
    })
}
```

### Pattern 2: Step-by-Step Migration Testing

```go
func TestMigrations_StepByStep(t *testing.T) {
    helper := NewMigrationHelper(t)
    
    // Get all migration files
    migrations := []struct {
        version uint
        name    string
        test    func(t *testing.T, db *sql.DB)
    }{
        {
            version: 1,
            name:    "initial schema",
            test: func(t *testing.T, db *sql.DB) {
                // Verify initial tables exist
                assertTableExists(t, db, "accounts")
                assertTableExists(t, db, "calls")
                assertTableExists(t, db, "bids")
            },
        },
        {
            version: 2,
            name:    "add indexes",
            test: func(t *testing.T, db *sql.DB) {
                // Verify indexes were created
                assertIndexExists(t, db, "idx_calls_created_at")
                assertIndexExists(t, db, "idx_bids_call_id")
            },
        },
    }
    
    for _, m := range migrations {
        t.Run(fmt.Sprintf("migration %d: %s", m.version, m.name), func(t *testing.T) {
            // Migrate to specific version
            err := helper.Migrate.Migrate(m.version)
            require.NoError(t, err)
            
            // Verify version
            version, dirty, err := helper.GetVersion()
            require.NoError(t, err)
            require.False(t, dirty)
            require.Equal(t, m.version, version)
            
            // Run version-specific tests
            m.test(t, helper.DB)
        })
    }
}
```

### Pattern 3: Idempotency Testing

```go
func TestMigrations_Idempotency(t *testing.T) {
    helper := NewMigrationHelper(t)
    
    t.Run("up migrations are idempotent", func(t *testing.T) {
        // Run migrations up
        err := helper.Migrate.Up()
        require.NoError(t, err)
        
        // Get current version
        version1, _, err := helper.GetVersion()
        require.NoError(t, err)
        
        // Try to run up again - should be no-op
        err = helper.Migrate.Up()
        require.Equal(t, migrate.ErrNoChange, err)
        
        // Version should not change
        version2, _, err := helper.GetVersion()
        require.NoError(t, err)
        require.Equal(t, version1, version2)
    })
}
```

### Pattern 4: Rollback Safety Testing

```go
func TestMigrations_RollbackSafety(t *testing.T) {
    helper := NewMigrationHelper(t)
    
    // Run all migrations up
    err := helper.Migrate.Up()
    require.NoError(t, err)
    
    finalVersion, _, _ := helper.GetVersion()
    
    // Test rolling back each migration
    for v := int(finalVersion); v > 0; v-- {
        t.Run(fmt.Sprintf("rollback from version %d", v), func(t *testing.T) {
            // Seed data at this version
            seedTestData(t, helper.DB, uint(v))
            
            // Rollback one step
            err := helper.Migrate.Steps(-1)
            require.NoError(t, err)
            
            // Verify data integrity after rollback
            if v > 1 {
                verifyDataIntegrity(t, helper.DB, uint(v-1))
            }
            
            // Roll forward again
            err = helper.Migrate.Steps(1)
            require.NoError(t, err)
            
            // Verify we can still use the schema
            verifySchemaUsable(t, helper.DB, uint(v))
        })
    }
}
```

## Schema Validation

### Table Structure Validation

```go
func assertTableStructure(t *testing.T, db *sql.DB, tableName string, expected map[string]string) {
    query := `
        SELECT column_name, data_type, is_nullable, column_default
        FROM information_schema.columns
        WHERE table_schema = 'public' AND table_name = $1
        ORDER BY ordinal_position
    `
    
    rows, err := db.Query(query, tableName)
    require.NoError(t, err)
    defer rows.Close()
    
    actual := make(map[string]string)
    for rows.Next() {
        var columnName, dataType, isNullable string
        var columnDefault sql.NullString
        
        err := rows.Scan(&columnName, &dataType, &isNullable, &columnDefault)
        require.NoError(t, err)
        
        actual[columnName] = dataType
    }
    
    // Compare with expected structure
    for col, expectedType := range expected {
        actualType, exists := actual[col]
        require.True(t, exists, "column %s should exist in table %s", col, tableName)
        assert.Contains(t, actualType, expectedType, 
            "column %s in table %s has unexpected type", col, tableName)
    }
}
```

### Constraint Validation

```go
func assertConstraintExists(t *testing.T, db *sql.DB, tableName, constraintName string) {
    var exists bool
    query := `
        SELECT EXISTS (
            SELECT 1 FROM information_schema.table_constraints
            WHERE table_schema = 'public'
            AND table_name = $1
            AND constraint_name = $2
        )
    `
    err := db.QueryRow(query, tableName, constraintName).Scan(&exists)
    require.NoError(t, err)
    require.True(t, exists, "constraint %s should exist on table %s", constraintName, tableName)
}

func assertForeignKeyExists(t *testing.T, db *sql.DB, tableName, columnName, refTable, refColumn string) {
    var exists bool
    query := `
        SELECT EXISTS (
            SELECT 1 
            FROM information_schema.key_column_usage kcu
            JOIN information_schema.referential_constraints rc 
                ON kcu.constraint_name = rc.constraint_name
            JOIN information_schema.key_column_usage kcu2 
                ON rc.unique_constraint_name = kcu2.constraint_name
            WHERE kcu.table_name = $1 
            AND kcu.column_name = $2
            AND kcu2.table_name = $3
            AND kcu2.column_name = $4
        )
    `
    err := db.QueryRow(query, tableName, columnName, refTable, refColumn).Scan(&exists)
    require.NoError(t, err)
    require.True(t, exists, "foreign key from %s.%s to %s.%s should exist", 
        tableName, columnName, refTable, refColumn)
}
```

### Index Validation

```go
func assertIndexExists(t *testing.T, db *sql.DB, indexName string) {
    var exists bool
    query := `
        SELECT EXISTS (
            SELECT 1 FROM pg_indexes
            WHERE schemaname = 'public'
            AND indexname = $1
        )
    `
    err := db.QueryRow(query, indexName).Scan(&exists)
    require.NoError(t, err)
    require.True(t, exists, "index %s should exist", indexName)
}

func getIndexDefinition(t *testing.T, db *sql.DB, indexName string) string {
    var definition string
    query := `
        SELECT indexdef FROM pg_indexes
        WHERE schemaname = 'public'
        AND indexname = $1
    `
    err := db.QueryRow(query, indexName).Scan(&definition)
    require.NoError(t, err)
    return definition
}
```

## Data Migration Testing

### Testing Data Transformations

```go
func TestDataMigration_TransformPhoneNumbers(t *testing.T) {
    helper := NewMigrationHelper(t)
    
    // Migrate to version before phone number format change
    err := helper.Migrate.Migrate(3)
    require.NoError(t, err)
    
    // Insert test data with old format
    testData := []struct {
        id         string
        oldFormat  string
        newFormat  string
    }{
        {"1", "555-123-4567", "+15551234567"},
        {"2", "(555) 987-6543", "+15559876543"},
        {"3", "555.111.2222", "+15551112222"},
    }
    
    for _, td := range testData {
        _, err := helper.DB.Exec(
            "INSERT INTO calls (id, from_number, to_number, status, type) VALUES ($1, $2, $3, $4, $5)",
            td.id, td.oldFormat, "555-000-0000", "pending", "inbound",
        )
        require.NoError(t, err)
    }
    
    // Run migration that transforms phone numbers
    err = helper.Migrate.Migrate(4)
    require.NoError(t, err)
    
    // Verify data was transformed correctly
    for _, td := range testData {
        var fromNumber string
        err := helper.DB.QueryRow(
            "SELECT from_number FROM calls WHERE id = $1", 
            td.id,
        ).Scan(&fromNumber)
        require.NoError(t, err)
        assert.Equal(t, td.newFormat, fromNumber)
    }
}
```

### Data Integrity During Migration

```go
func TestDataMigration_PreservesRelationships(t *testing.T) {
    helper := NewMigrationHelper(t)
    
    // Setup initial schema
    err := helper.Migrate.Migrate(5)
    require.NoError(t, err)
    
    // Create test data with relationships
    accountID := uuid.New()
    _, err = helper.DB.Exec(`
        INSERT INTO accounts (id, email, company, type, status)
        VALUES ($1, 'test@example.com', 'Test Co', 'buyer', 'active')
    `, accountID)
    require.NoError(t, err)
    
    callID := uuid.New()
    _, err = helper.DB.Exec(`
        INSERT INTO calls (id, from_number, to_number, buyer_id, status, type)
        VALUES ($1, '+15551234567', '+15559876543', $2, 'active', 'inbound')
    `, callID, accountID)
    require.NoError(t, err)
    
    // Count relationships before migration
    var callCount int
    err = helper.DB.QueryRow(
        "SELECT COUNT(*) FROM calls WHERE buyer_id = $1", 
        accountID,
    ).Scan(&callCount)
    require.NoError(t, err)
    require.Equal(t, 1, callCount)
    
    // Run migration that modifies schema
    err = helper.Migrate.Migrate(6)
    require.NoError(t, err)
    
    // Verify relationships still exist
    err = helper.DB.QueryRow(
        "SELECT COUNT(*) FROM calls WHERE buyer_id = $1", 
        accountID,
    ).Scan(&callCount)
    require.NoError(t, err)
    require.Equal(t, 1, callCount, "relationships should be preserved after migration")
}
```

### Performance Testing for Migrations

```go
func TestMigration_Performance(t *testing.T) {
    helper := NewMigrationHelper(t)
    
    // Setup base schema
    err := helper.Migrate.Up()
    require.NoError(t, err)
    
    // Seed large dataset
    seedLargeDataset(t, helper.DB, 100000) // 100k records
    
    // Measure migration performance
    start := time.Now()
    
    // Run a migration that adds index on large table
    err = helper.Migrate.Migrate(7)
    require.NoError(t, err)
    
    duration := time.Since(start)
    
    // Assert performance expectations
    assert.Less(t, duration, 30*time.Second, 
        "migration should complete within 30 seconds for 100k records")
    
    // Log performance metrics
    t.Logf("Migration completed in %v for 100k records", duration)
}
```

### Concurrent Migration Safety

```go
func TestMigration_ConcurrentExecution(t *testing.T) {
    ctx := context.Background()
    
    // Start two containers with same database
    container, err := postgresModule.Run(ctx, "postgres:16-alpine")
    require.NoError(t, err)
    defer container.Terminate(ctx)
    
    connStr, _ := container.ConnectionString(ctx, "sslmode=disable")
    
    // Create two migration instances
    m1, err := migrate.New("file://../../migrations", connStr)
    require.NoError(t, err)
    defer m1.Close()
    
    m2, err := migrate.New("file://../../migrations", connStr)
    require.NoError(t, err)
    defer m2.Close()
    
    // Try to run migrations concurrently
    var wg sync.WaitGroup
    errors := make(chan error, 2)
    
    for i, m := range []*migrate.Migrate{m1, m2} {
        wg.Add(1)
        go func(instance int, mig *migrate.Migrate) {
            defer wg.Done()
            if err := mig.Up(); err != nil && err != migrate.ErrNoChange {
                errors <- fmt.Errorf("instance %d: %w", instance, err)
            }
        }(i, m)
    }
    
    wg.Wait()
    close(errors)
    
    // Collect errors
    var migrationErrors []error
    for err := range errors {
        migrationErrors = append(migrationErrors, err)
    }
    
    // At least one should succeed, one might fail with lock error
    assert.LessOrEqual(t, len(migrationErrors), 1, 
        "at most one migration should fail")
    
    // Verify final state is correct
    version, dirty, err := m1.Version()
    require.NoError(t, err)
    require.False(t, dirty)
    require.Greater(t, version, uint(0))
}
```

## CI/CD Integration

### GitHub Actions Workflow

```yaml
# .github/workflows/migration-tests.yml
name: Migration Tests

on:
  pull_request:
    paths:
      - 'migrations/**'
      - 'internal/migration/**'

jobs:
  test-migrations:
    runs-on: ubuntu-latest
    
    services:
      postgres:
        image: postgres:16-alpine
        env:
          POSTGRES_PASSWORD: postgres
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
        ports:
          - 5432:5432

    steps:
      - uses: actions/checkout@v4
      
      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.24'
      
      - name: Run Migration Tests
        run: |
          go test -v -tags=migration ./internal/migration/...
        env:
          DATABASE_URL: postgres://postgres:postgres@localhost:5432/test?sslmode=disable
      
      - name: Validate Migration Files
        run: |
          # Check migration file naming
          for f in migrations/*.sql; do
            if ! [[ "$f" =~ ^migrations/[0-9]{3}_[a-z_]+\.(up|down)\.sql$ ]]; then
              echo "Invalid migration filename: $f"
              exit 1
            fi
          done
          
          # Ensure up/down migrations are paired
          for up in migrations/*up.sql; do
            down="${up/up.sql/down.sql}"
            if [ ! -f "$down" ]; then
              echo "Missing down migration for: $up"
              exit 1
            fi
          done
```

### Automated Migration Validation

```go
// cmd/validate-migrations/main.go
package main

import (
    "database/sql"
    "fmt"
    "log"
    "os"
    
    "github.com/golang-migrate/migrate/v4"
    _ "github.com/golang-migrate/migrate/v4/database/postgres"
    _ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
    dbURL := os.Getenv("DATABASE_URL")
    if dbURL == "" {
        log.Fatal("DATABASE_URL environment variable required")
    }
    
    // Test migrations up
    m, err := migrate.New("file://migrations", dbURL)
    if err != nil {
        log.Fatalf("Failed to create migrate instance: %v", err)
    }
    defer m.Close()
    
    // Run all migrations up
    if err := m.Up(); err != nil {
        log.Fatalf("Failed to run migrations up: %v", err)
    }
    
    version, dirty, err := m.Version()
    if err != nil {
        log.Fatalf("Failed to get version: %v", err)
    }
    
    if dirty {
        log.Fatal("Database is in dirty state")
    }
    
    fmt.Printf("Successfully migrated to version %d\n", version)
    
    // Test migrations down
    if err := m.Down(); err != nil {
        log.Fatalf("Failed to run migrations down: %v", err)
    }
    
    fmt.Println("All migrations validated successfully")
}
```

## Best Practices

### 1. Migration File Organization

```
migrations/
├── 001_initial_schema.up.sql
├── 001_initial_schema.down.sql
├── 002_add_indexes.up.sql
├── 002_add_indexes.down.sql
├── 003_add_quality_scores.up.sql
└── 003_add_quality_scores.down.sql
```

### 2. Migration Writing Guidelines

```sql
-- Good: Idempotent migration
-- 002_add_indexes.up.sql
CREATE INDEX IF NOT EXISTS idx_calls_created_at ON calls(created_at);
CREATE INDEX IF NOT EXISTS idx_calls_status ON calls(status);

-- Good: Safe column addition with default
ALTER TABLE accounts 
ADD COLUMN IF NOT EXISTS quality_score DECIMAL(3,2) DEFAULT 0.50;

-- Good: Transactional migration
BEGIN;
ALTER TABLE calls ADD COLUMN duration INTEGER;
UPDATE calls SET duration = 0 WHERE duration IS NULL;
ALTER TABLE calls ALTER COLUMN duration SET NOT NULL;
COMMIT;
```

### 3. Testing Checklist

- [ ] Test migrations up and down
- [ ] Verify schema structure after each migration
- [ ] Test with production-like data volumes
- [ ] Validate foreign key constraints
- [ ] Check index creation and performance
- [ ] Test concurrent migration handling
- [ ] Verify data integrity during transformations
- [ ] Measure migration execution time
- [ ] Test rollback scenarios
- [ ] Validate against application code

### 4. Common Pitfalls to Avoid

1. **Non-Reversible Migrations**
   ```sql
   -- Bad: Can't recover dropped column data
   ALTER TABLE users DROP COLUMN phone_number;
   
   -- Good: Rename and deprecate first
   ALTER TABLE users RENAME COLUMN phone_number TO phone_number_deprecated;
   ```

2. **Breaking Application Compatibility**
   ```sql
   -- Bad: Immediate breaking change
   ALTER TABLE calls ALTER COLUMN status TYPE VARCHAR(50);
   
   -- Good: Gradual migration
   -- Step 1: Add new column
   ALTER TABLE calls ADD COLUMN status_new VARCHAR(50);
   -- Step 2: Migrate data
   UPDATE calls SET status_new = status::VARCHAR(50);
   -- Step 3: Switch columns (in separate migration)
   ```

3. **Long-Running Migrations**
   ```sql
   -- Bad: Locks table for extended time
   CREATE INDEX idx_large_table ON large_table(column);
   
   -- Good: Create concurrently
   CREATE INDEX CONCURRENTLY idx_large_table ON large_table(column);
   ```

### 5. Migration Testing in Development

```bash
# Run migration tests locally
make test-migrations

# Test specific migration
go test -v -run TestMigration_003 ./internal/migration/...

# Validate all migrations
go run cmd/validate-migrations/main.go
```

## Troubleshooting

### Common Issues

1. **Dirty Database State**
   ```go
   // Force version reset (use with caution)
   m.Force(previousVersion)
   ```

2. **Migration Lock Issues**
   ```sql
   -- Check for locks
   SELECT * FROM pg_locks WHERE locktype = 'advisory';
   
   -- Release stuck migration lock
   SELECT pg_advisory_unlock(123456789);
   ```

3. **Performance Problems**
   ```sql
   -- Monitor migration progress
   SELECT query, state, wait_event_type, wait_event 
   FROM pg_stat_activity 
   WHERE query LIKE '%ALTER TABLE%';
   ```

### Migration Debugging

```go
// Enable verbose logging
m, err := migrate.New(
    "file://migrations",
    dbURL,
)
m.Log = &logger{t: t} // Custom logger implementation

// Check migration status
version, dirty, err := m.Version()
t.Logf("Current version: %d, Dirty: %v", version, dirty)
```

## Summary

Comprehensive migration testing ensures:

1. **Safety**: No data loss or corruption
2. **Reversibility**: Can rollback if needed
3. **Performance**: Migrations complete in reasonable time
4. **Compatibility**: Application continues to work
5. **Reliability**: Consistent behavior across environments

Remember: Every migration should be tested as thoroughly as application code. The database is the foundation of your application - treat it with respect.

## Version History

| Version | Date | Changes |
|---------|------|---------|
| 1.0.0 | 2025-06-09 | Initial migration testing guide |

## See Also

- [Database Testing Guide](01-database-testing-guide.md)
- [Testcontainers Guide](02-testcontainers-guide.md)
- [golang-migrate Documentation](https://github.com/golang-migrate/migrate)
