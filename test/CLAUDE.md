# Test Context

## Test Organization
- `e2e/` - End-to-end user journey tests
- `integration/` - Cross-service integration tests
- Unit tests colocated with source files (`*_test.go`)
- Property tests use `*_property_test.go` suffix
- Concurrent tests use `*_synctest.go` suffix

## Testing Infrastructure

### Database Testing
- Use testcontainers for isolated PostgreSQL instances
- Automatic schema setup via migrations
- Transaction rollback for test isolation
- Parallel test support with separate databases

### Test Data Creation
**IMPORTANT: Respect foreign key constraints**
1. Create accounts first (buyers and sellers)
2. Create calls referencing valid account IDs
3. Create bids referencing valid call and account IDs
4. Use `.String()` method on enums for database insertion

### Test Builders
- Use fluent builders for test data creation
- Example: `NewCallBuilder().WithStatus(CallStatusActive).WithBuyerID(buyerID).Build()`
- Builders handle default values and validation
- Located in `internal/testutil/fixtures/`

## Testing Patterns

### Unit Tests
- Table-driven tests for multiple scenarios
- Mock external dependencies
- Focus on single unit of functionality
- Fast execution (< 100ms per test)

### Integration Tests
- Test service interactions
- Use real database with testcontainers
- Include API endpoint testing
- Test transaction boundaries

### Property-Based Tests
- Test invariants with random inputs
- 1000+ iterations per property
- Use `testing/quick` package
- Focus on edge cases and boundaries

### Concurrent Tests (Go 1.24)
- Enable with `GOEXPERIMENT=synctest`
- Eliminates timing-based flakiness
- Test race conditions deterministically
- Use `testing/synctest` package

## Common Test Helpers
- `testutil.SetupTestDB()` - Creates test database
- `testutil.TruncateTables()` - Cleans data between tests
- `testutil.LoadFixtures()` - Loads test data
- `testutil.AssertErrorCode()` - Validates error types

## Running Tests
```bash
# All tests
make test

# With race detection
make test-race

# With deterministic concurrency
make test-synctest

# Integration tests only
make test-integration

# Property tests
make test-property
```