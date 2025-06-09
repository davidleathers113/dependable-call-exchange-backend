# Testing Guide for Dependable Call Exchange Backend

This guide covers the comprehensive testing strategy implemented for the Dependable Call Exchange Backend, including the latest Go 1.24 testing features and TDD best practices.

## Testing Philosophy

Our testing approach follows modern TDD principles:
- **Test First**: Write failing tests before implementing features
- **Red-Green-Refactor**: Follow the TDD cycle religiously
- **Property-Based Testing**: Use randomized inputs to find edge cases
- **Deterministic Concurrency**: Use Go 1.24's synctest for reliable concurrent tests
- **Integration Testing**: Test complete workflows with real databases

## Test Types and Structure

### 1. Unit Tests (`*_test.go`)

Located alongside source files, these test individual functions and methods:

```go
// Example: internal/domain/call/call_test.go
func TestNewCall(t *testing.T) {
    call := NewCall("+15551234567", "+15559876543", uuid.New(), DirectionInbound)
    assert.Equal(t, StatusPending, call.Status)
}
```

**Run unit tests:**
```bash
make test
make test-race  # With race detection
```

### 2. Property-Based Tests (`*_property_test.go`)

Use Go's `testing/quick` package to test invariants with randomized inputs:

```go
// Example: internal/domain/call/call_property_test.go
func TestCall_PropertyInvariants(t *testing.T) {
    property := func(from, to string, direction Direction) bool {
        c := NewCall(from, to, uuid.New(), direction)
        return !c.UpdatedAt.Before(c.CreatedAt)
    }
    
    err := quick.Check(property, &quick.Config{MaxCount: 1000})
    require.NoError(t, err)
}
```

**Run property-based tests:**
```bash
make test-property
make bench-property
```

### 3. Concurrent Tests with Synctest (`*_synctest.go`)

**NEW in Go 1.24**: Use experimental `testing/synctest` for deterministic concurrent testing:

```go
//go:build synctest

func TestService_ConcurrentRouting_Deterministic(t *testing.T) {
    synctest.Run(func() {
        // Test concurrent operations with deterministic timing
        start := time.Now()
        
        // Multiple goroutines with precise timing control
        for i := 0; i < 5; i++ {
            go func(index int) {
                time.Sleep(time.Duration(index) * 100 * time.Millisecond)
                // Operations complete at exactly predictable times
            }(i)
        }
        
        synctest.Wait() // Wait for all goroutines to become idle
        elapsed := time.Since(start)
        
        // With synctest, timing is deterministic
        assert.Equal(t, 400*time.Millisecond, elapsed)
    })
}
```

**Enable and run synctest:**
```bash
make test-synctest
GOEXPERIMENT=synctest go test -v ./...
```

### 4. Integration Tests (`test/integration/*_test.go`)

Test complete workflows with real databases:

```go
//go:build integration

func TestCallRouting_EndToEnd(t *testing.T) {
    testDB := testutil.NewTestDB(t)
    // Test complete call routing flow with real database
}
```

**Run integration tests:**
```bash
make test-integration
docker-compose -f docker-compose.test.yml up --abort-on-container-exit
```

## Test Infrastructure

### Test Utilities (`internal/testutil/`)

- **`database.go`**: Test database setup and management
- **`helpers.go`**: Common test utilities and assertions  
- **`fixtures/`**: Test data builders and scenarios
- **`mocks/`**: Mock implementations for dependencies

### Test Database

Automatic test database management:

```go
func TestMyFeature(t *testing.T) {
    testDB := testutil.NewTestDB(t)  // Creates isolated test DB
    ctx := testutil.TestContext(t)   // Context with timeout
    
    // Test database is automatically cleaned up
    defer testDB.TruncateTables()
}
```

### Fixtures and Builders

Fluent test data creation:

```go
// Using CallBuilder
call := fixtures.NewCallBuilder(t).
    WithStatus(call.StatusPending).
    WithPhoneNumbers("+15551234567", "+15559876543").
    Build()

// Using scenarios  
scenarios := fixtures.NewCallScenarios(t)
activeCall := scenarios.ActiveCall()
completedCall := scenarios.CompletedCall()
```

### Enhanced Mocks

Rich mock infrastructure with builders:

```go
// Setup repository mocks
callRepo := new(mocks.CallRepository)
callRepo.ExpectCallLifecycle(ctx, callID)

// Fluent mock builder
mockCall := NewCallMockBuilder(callRepo, ctx).
    WithStatus(call.StatusPending).
    ExpectGet().
    ExpectUpdate().
    Build()
```

## Testing Commands

### Basic Testing
```bash
# Run all tests
make test

# Run with race detection
make test-race

# Run specific test
go test -v -run TestNewCall ./internal/domain/call/
```

### Go 1.24 Features
```bash
# Run synctest (deterministic concurrent tests)
make test-synctest

# Run property-based tests
make test-property

# Run integration tests
make test-integration
```

### Coverage
```bash
# Generate coverage report
make coverage

# Generate coverage with synctest
make coverage-synctest

# View coverage in browser
open coverage.html
```

### Benchmarks
```bash
# Run all benchmarks
make bench

# Run property-based benchmarks
make bench-property

# Benchmark specific function
go test -bench=BenchmarkCall_PropertyCreation ./internal/domain/call/
```

## TDD Workflow

### 1. Red: Write Failing Test

```go
func TestCallRouter_SkillBasedRouting(t *testing.T) {
    // Arrange
    router := NewSkillBasedRouter()
    call := fixtures.NewCallBuilder(t).
        WithRequiredSkills([]string{"sales", "support"}).
        Build()
    
    // Act
    decision, err := router.Route(call)
    
    // Assert
    require.NoError(t, err)
    assert.Contains(t, decision.MatchedSkills, "sales")
    assert.Contains(t, decision.MatchedSkills, "support")
}
```

### 2. Green: Make Test Pass

Implement minimal code to make the test pass:

```go
func (r *SkillBasedRouter) Route(call *Call) (*RoutingDecision, error) {
    // Minimal implementation
    return &RoutingDecision{
        MatchedSkills: []string{"sales", "support"},
    }, nil
}
```

### 3. Refactor: Improve Code Quality

Refactor while keeping tests green:

```go
func (r *SkillBasedRouter) Route(call *Call) (*RoutingDecision, error) {
    requiredSkills := call.GetRequiredSkills()
    availableBids := r.bidRepo.GetActiveBidsForCall(call.ID)
    
    bestMatch := r.findBestSkillMatch(requiredSkills, availableBids)
    if bestMatch == nil {
        return nil, ErrNoMatchingSkills
    }
    
    return &RoutingDecision{
        BidID:         bestMatch.ID,
        MatchedSkills: intersection(requiredSkills, bestMatch.Skills),
        Score:         r.calculateSkillScore(requiredSkills, bestMatch.Skills),
    }, nil
}
```

## Testing Best Practices

### 1. Test Naming
- Use descriptive test names: `TestCallRouter_SkillBasedRouting_WithMultipleMatches`
- Group related tests with subtests: `t.Run("invalid phone number", func(t *testing.T) { ... })`

### 2. Test Structure (AAA Pattern)
```go
func TestFeature(t *testing.T) {
    // Arrange
    setup := prepareTestData()
    
    // Act  
    result := systemUnderTest.DoSomething(setup.input)
    
    // Assert
    assert.Equal(t, expected, result)
}
```

### 3. Table-Driven Tests
```go
func TestCallValidation(t *testing.T) {
    tests := []struct {
        name        string
        fromNumber  string
        toNumber    string
        expectError bool
    }{
        {"valid US number", "+15551234567", "+15559876543", false},
        {"invalid format", "123", "456", true},
        {"missing country code", "5551234567", "+15559876543", true},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := ValidateCall(tt.fromNumber, tt.toNumber)
            if tt.expectError {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}
```

### 4. Test Isolation
- Each test should be independent
- Use `testutil.NewTestDB(t)` for database isolation
- Clean up resources: `t.Cleanup(cleanup)`

### 5. Concurrent Testing
- Use `testing/synctest` for deterministic concurrent tests
- Avoid `time.Sleep()` in tests - use `synctest.Wait()`
- Test race conditions explicitly with `-race` flag

## CI/CD Integration

Tests are automatically run in GitHub Actions:

```yaml
# .github/workflows/ci.yml
- name: Run tests
  env:
    DCE_DATABASE_URL: postgres://postgres:postgres@localhost:5432/dce_test?sslmode=disable
  run: go test -race -coverprofile=coverage.out -covermode=atomic ./...

- name: Run synctest
  run: GOEXPERIMENT=synctest go test -v ./...

- name: Run integration tests  
  run: go test -tags=integration ./test/...
```

## Performance Testing

### Benchmarks
```go
func BenchmarkCallRouting(b *testing.B) {
    router := setupRouter()
    call := setupTestCall()
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        router.Route(call)
    }
}
```

### Load Testing
```bash
# Run benchmarks with memory profiling
go test -bench=. -benchmem -memprofile=mem.prof ./...

# Analyze memory usage
go tool pprof mem.prof
```

## Troubleshooting

### Common Issues

1. **Flaky Tests**: Use `testing/synctest` for time-dependent tests
2. **Database Conflicts**: Ensure unique test database names
3. **Race Conditions**: Always run with `-race` flag
4. **Mock Setup**: Use builders for complex mock scenarios

### Debugging Tests
```bash
# Run with verbose output
go test -v ./...

# Run specific test with debugging
go test -v -run TestSpecificFunction ./package/

# Run with race detection and verbose output
go test -race -v ./...
```

### Environment Variables
```bash
# For integration tests
export DCE_DATABASE_URL="postgres://postgres:postgres@localhost:5432/dce_test?sslmode=disable"
export DCE_REDIS_URL="localhost:6379"

# For synctest
export GOEXPERIMENT=synctest
```

## Future Enhancements

- **Chaos Testing**: Introduce random failures to test resilience
- **Contract Testing**: Use Pact for API contract testing  
- **Performance Regression**: Automated benchmark comparison
- **Mutation Testing**: Test the quality of tests themselves
- **E2E Testing**: Browser-based testing for web interfaces

## Resources

- [Go Testing Package](https://pkg.go.dev/testing)
- [Go 1.24 Synctest](https://go.dev/blog/synctest)
- [Property-Based Testing in Go](https://pkg.go.dev/testing/quick)
- [Testify Documentation](https://github.com/stretchr/testify)
- [Learn Go with Tests](https://quii.gitbook.io/learn-go-with-tests)