# Test Documentation

## Test Isolation Strategy

This document outlines the test isolation strategy using database transaction helpers to prevent flaky tests and ensure consistent test execution.

## Transaction Helpers

The transaction helpers in `internal/testutil/tx_helper.go` provide guaranteed test isolation by wrapping test execution in database transactions that are always rolled back.

### Basic Usage

#### WithTransaction
The primary helper for test isolation:

```go
func TestMyFeature(t *testing.T) {
    testDB := testutil.NewTestDB(t)
    
    testutil.WithTransaction(t, testDB.DB(), func(tx *sql.Tx) {
        // All database operations here are rolled back
        // Use tx instead of db for queries
        
        _, err := tx.Exec(`INSERT INTO accounts ...`)
        require.NoError(t, err)
        
        // Test assertions...
    })
    // Database is clean after this point
}
```

#### WithTransactionContext
For tests that need context support:

```go
func TestWithContext(t *testing.T) {
    testDB := testutil.NewTestDB(t)
    ctx := context.Background()
    
    testutil.WithTransactionContext(t, ctx, testDB.DB(), func(ctx context.Context, tx *sql.Tx) {
        // Use ctx for cancellation/timeout
        _, err := tx.ExecContext(ctx, `INSERT INTO accounts ...`)
        require.NoError(t, err)
    })
}
```

#### WithParallelTransactions
For running multiple isolated tests in parallel:

```go
func TestParallelOperations(t *testing.T) {
    testDB := testutil.NewTestDB(t)
    
    testutil.WithParallelTransactions(t, testDB.DB(),
        func(tx *sql.Tx) {
            // Test 1 - runs in parallel
        },
        func(tx *sql.Tx) {
            // Test 2 - runs in parallel
        },
        func(tx *sql.Tx) {
            // Test 3 - runs in parallel
        },
    )
}
```

### Integration Test Pattern

For integration tests, wrap repository creation with transaction support:

```go
func TestIntegration(t *testing.T) {
    testDB := testutil.NewTestDB(t)
    
    testutil.WithTransaction(t, testDB.DB(), func(tx *sql.Tx) {
        // Create repositories with transaction
        callRepo := repository.NewCallRepositoryWithTx(tx)
        bidRepo := repository.NewBidRepositoryWithTx(tx)
        accountRepo := repository.NewAccountRepositoryWithTx(tx)
        
        // Create test data
        testData := fixtures.CreateCompleteTestSet(t, testDB)
        
        // Run tests - all changes will be rolled back
        // ...
    })
}
```

### Test Suite Pattern

For organizing multiple related tests:

```go
func TestAccountSuite(t *testing.T) {
    testDB := testutil.NewTestDB(t)
    suite := testutil.NewTransactionTestSuite(t, testDB.DB())
    
    suite.AddTest("create account", func(t *testing.T, tx *sql.Tx) {
        // Test account creation
    })
    
    suite.AddTest("update account", func(t *testing.T, tx *sql.Tx) {
        // Test account updates
    })
    
    suite.AddTest("delete account", func(t *testing.T, tx *sql.Tx) {
        // Test account deletion
    })
    
    suite.RunParallel() // All tests run in parallel with isolation
}
```

## Benefits

1. **Complete Isolation**: Each test runs in its own transaction that's rolled back
2. **No Test Interdependence**: Tests can't affect each other's data
3. **Parallel Execution**: Tests can run concurrently without conflicts
4. **Fast Cleanup**: No need to manually clean up test data
5. **Consistent State**: Database always returns to clean state

## Best Practices

1. **Always use transaction helpers** for database tests
2. **Pass the transaction** to repositories, not the database connection
3. **Use context** for long-running operations that may need cancellation
4. **Run tests in parallel** when possible for faster execution
5. **Group related tests** using the test suite pattern

## Migration from Existing Tests

To migrate existing tests:

1. Replace `testDB.DB()` with transaction from helper
2. Wrap test logic in `WithTransaction` call
3. Update repository creation to use `WithTx` variants
4. Remove manual cleanup code (no longer needed)
5. Enable parallel execution where appropriate

## Performance Considerations

- Transaction rollback is much faster than `TRUNCATE` or `DELETE`
- Parallel execution reduces total test time
- No need for test data snapshots or restoration
- Reduced database connection overhead

## Troubleshooting

### Tests Still Seeing Other Test Data
- Ensure all repositories use the transaction, not the database
- Check for any direct `db.Query()` calls that should use `tx.Query()`
- Verify no commits are happening within the test

### Deadlocks in Parallel Tests
- Review transaction isolation levels
- Check for conflicting row locks
- Consider test data design to minimize conflicts

### Performance Issues
- Use `WithParallelTransactions` for independent tests
- Consider connection pool settings
- Monitor for long-running transactions