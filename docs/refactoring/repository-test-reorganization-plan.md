# Repository Test Reorganization Plan

## Overview

This plan addresses the issue of large repository test files (1000+ lines) by splitting them according to the established testing patterns already documented in `docs/testing/TESTING.md`. This approach maintains consistency with the existing codebase while improving maintainability.

## Current State

- `bid_repository_test.go`: 1,056 lines (9 test functions)
- `call_repository_test.go`: 1,236 lines
- `account_repository_test.go`: 505 lines
- Test files mix different concerns: CRUD, complex queries, property-based tests, concurrency tests

## Established Testing Pattern

The codebase already follows a clear naming convention for different test types:
- `*_test.go` - Basic unit tests (CRUD operations, simple queries)
- `*_property_test.go` - Property-based tests with randomized inputs
- `*_synctest.go` - Concurrent tests using Go 1.24's synctest
- Feature-specific tests like `*_query_test.go` for complex queries

## Proposed Reorganization

### 1. Bid Repository Tests

Split `bid_repository_test.go` into:

```
bid_repository_test.go           # ~300 lines
  - TestBidRepository_Create
  - TestBidRepository_GetByID
  - TestBidRepository_Update
  - TestBidRepository_Delete

bid_repository_query_test.go     # ~400 lines
  - TestBidRepository_GetActiveBidsForCall
  - TestBidRepository_GetByBuyer
  - TestBidRepository_GetExpiredBids

bid_repository_property_test.go  # ~130 lines
  - TestBidRepository_PropertyBased

bid_repository_sync_test.go      # ~100 lines
  - TestBidRepository_Concurrency
```

### 2. Call Repository Tests

Split `call_repository_test.go` into:

```
call_repository_test.go           # ~400 lines
  - TestCallRepository_Create
  - TestCallRepository_GetByID
  - TestCallRepository_Update
  - TestCallRepository_UpdateStatus
  - TestCallRepository_Delete

call_repository_query_test.go     # ~500 lines
  - TestCallRepository_GetByStatus
  - TestCallRepository_GetActiveCallsForBuyer
  - TestCallRepository_GetRecentCalls
  - TestCallRepository_GetCallsWithFilters

call_repository_property_test.go  # ~200 lines
  - TestCallRepository_PropertyBased
  - TestCallRepository_StatusTransitionProperties

call_repository_sync_test.go      # ~150 lines
  - TestCallRepository_ConcurrentOperations
  - TestCallRepository_ConcurrentStatusUpdates
```

### 3. Account Repository Tests

Split `account_repository_test.go` into:

```
account_repository_test.go        # ~400 lines
  - TestAccountRepository_GetByID
  - TestAccountRepository_UpdateBalance
  - TestAccountRepository_GetBalance
  - TestAccountRepository_DatabaseConstraints

account_repository_property_test.go # ~100 lines
  - TestAccountRepository_PropertyBased
```

### 4. Shared Test Helpers

Create `repository_test_helpers.go` for common test utilities:

```go
package repository

// Test data creation helpers
func createTestAccountAndCall(t *testing.T, testDB *testutil.TestDB) (*account.Account, *call.Call)
func createBidTestAccountInDB(t *testing.T, testDB *testutil.TestDB, acc *account.Account) error
func createCallInDB(t *testing.T, testDB *testutil.TestDB, c *call.Call) error

// Common assertion helpers
func assertBidEquals(t *testing.T, expected, actual *bid.Bid)
func assertCallEquals(t *testing.T, expected, actual *call.Call)
func assertAccountEquals(t *testing.T, expected, actual *account.Account)
```

## Benefits

1. **Consistency**: Follows existing test organization patterns
2. **Maintainability**: Each file ~300-400 lines instead of 1000+
3. **Discoverability**: Easy to find specific test types
4. **Focused Testing**: Can run specific test categories
5. **Same Package**: Tests retain access to internal methods
6. **Clear Separation**: CRUD vs queries vs properties vs concurrency

## Implementation Steps

1. Create new test files following the naming convention
2. Move test functions to appropriate files based on their purpose
3. Extract shared helpers to `repository_test_helpers.go`
4. Update imports in each test file
5. Run all tests to ensure nothing breaks
6. Update any CI/CD scripts if they reference specific test files

## Test Execution Examples

After reorganization, tests can be run more selectively:

```bash
# Run only CRUD tests
go test -run "TestBidRepository_(Create|GetByID|Update|Delete)"

# Run only query tests
go test ./internal/infrastructure/repository -run "Query"

# Run only property-based tests
make test-property

# Run concurrent tests with synctest
GOEXPERIMENT=synctest go test ./internal/infrastructure/repository -run "Sync"

# Run all repository tests
go test ./internal/infrastructure/repository/...
```

## Migration Checklist

- [ ] Create `repository_test_helpers.go` with shared helpers
- [ ] Split `bid_repository_test.go` into 4 files
- [ ] Split `call_repository_test.go` into 4 files
- [ ] Split `account_repository_test.go` into 2 files
- [ ] Fix all imports in the new test files
- [ ] Run `go test ./internal/infrastructure/repository/...` to verify
- [ ] Update any documentation that references test files
- [ ] Commit with message: "refactor: split large repository test files by concern"

## Notes

- This reorganization does not change any test logic, only moves code
- All tests remain in the same package for internal access
- The pattern can be applied to other large test files in the codebase
- Consider applying similar patterns to service layer tests if needed