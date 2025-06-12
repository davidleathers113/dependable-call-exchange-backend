# Testing Guide

This comprehensive guide covers all testing strategies, patterns, and commands for the Dependable Call Exchange Backend.

## Testing Philosophy

We implement a comprehensive TDD approach leveraging Go 1.24's modern features:

- **Property-Based Testing**: Test invariants with thousands of random inputs
- **Synctest**: Deterministic testing of concurrent code  
- **Table-Driven Tests**: Comprehensive coverage of edge cases
- **Contract Testing**: OpenAPI validation with < 1ms overhead
- **Integration Tests**: Real database and service interactions

## Test Commands

### Core Testing Commands

```bash
# Basic Testing
make test                    # Run all unit tests
make test-race              # Race condition detection
make test-quick             # Fast unit tests only

# Advanced Testing (Go 1.24)
make test-synctest          # Deterministic concurrent tests (GOEXPERIMENT=synctest)
make test-property          # Property-based testing with 1000+ iterations
make test-contract          # OpenAPI contract testing
make test-contract-full     # Full contract validation with spec validation

# Integration Testing
make test-integration       # End-to-end with real database
docker-compose -f docker-compose.test.yml up --abort-on-container-exit

# Coverage & Analysis
make coverage               # Generate coverage report
make coverage-synctest      # Coverage with synctest
make bench                  # Performance benchmarks
make bench-property         # Property-based benchmarks
```

### Debugging Test Failures

```bash
# Show ALL compilation errors in tests
go test -run=xxx ./... 2>&1 | grep -E "(cannot use|undefined|unknown field)"

# Run specific test with verbose output
go test -v -run TestCallRouting ./internal/service/callrouting/...

# Debug race conditions
go test -race -v ./internal/domain/bid/...

# Profile test performance
go test -bench=. -cpuprofile=cpu.prof ./internal/service/bidding
go tool pprof cpu.prof
```

## Testing Patterns

### Property-Based Testing

Test invariants with randomized inputs:

```go
func TestMoneyOperationsNeverNegative(t *testing.T) {
    propertytest.Run(t, func(t *propertytest.T) {
        amount1 := t.Float64(0.01, 1000000)
        amount2 := t.Float64(0.01, amount1)
        
        money1, _ := values.NewMoneyFromFloat(amount1, "USD")
        money2, _ := values.NewMoneyFromFloat(amount2, "USD")
        
        result, err := money1.Subtract(money2)
        require.NoError(t, err)
        assert.False(t, result.IsNegative())
    })
}
```

### Synctest (Go 1.24)

Deterministic concurrent testing:

```go
func TestConcurrentBidProcessing(t *testing.T) {
    synctest.Run(t, func(t *synctest.T) {
        service := NewBiddingService(...)
        call := fixtures.NewCall()
        
        var wg sync.WaitGroup
        results := make([]error, 100)
        
        for i := 0; i < 100; i++ {
            wg.Add(1)
            go func(idx int) {
                defer wg.Done()
                bid := fixtures.NewBid()
                _, err := service.PlaceBid(ctx, call.ID, bid)
                results[idx] = err
            }(i)
        }
        
        wg.Wait()
        // Deterministic verification
    })
}
```

### Table-Driven Tests

Comprehensive test cases:

```go
func TestCallStatusTransitions(t *testing.T) {
    tests := []struct {
        name        string
        fromStatus  call.Status
        toStatus    call.Status
        shouldError bool
    }{
        {
            name:       "pending to ringing",
            fromStatus: call.StatusPending,
            toStatus:   call.StatusRinging,
            shouldError: false,
        },
        // ... more cases
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // test implementation
        })
    }
}
```

### Contract Testing

OpenAPI validation in tests:

```go
func TestAPIContract(t *testing.T) {
    // Load OpenAPI spec
    spec := contracttest.LoadSpec(t, "openapi.yaml")
    
    // Create test server with contract validation
    server := contracttest.NewServer(t, spec, handler)
    
    // Test endpoint
    resp := server.POST("/api/v1/bids").
        WithJSON(bidRequest).
        Expect(t).
        Status(http.StatusCreated)
    
    // Contract automatically validates request/response
}
```

## Test Infrastructure

### Fixture Builders

Always use fixture builders for test data:

```go
// Use this pattern
call := fixtures.NewCall().
    WithBuyer(buyerID).
    WithStatus(call.StatusRinging).
    Build()

// NOT this
call := &call.Call{
    ID: uuid.New(),
    // manual setup...
}
```

### Test Database

```go
// Automatic test database creation
testDB := testutil.NewTestDB(t)  // or CreateTestDB(t)

// Transaction-based isolation
tx := testDB.BeginTx(t)
defer tx.Rollback()
```

### Mocking

Interface-based mocking with expectations:

```go
mockRepo := mocks.NewCallRepository(t)
mockRepo.On("GetByID", ctx, callID).Return(call, nil)

service := NewCallService(mockRepo)
// ... test service
mockRepo.AssertExpectations(t)
```

## Performance Testing

### Benchmarks

```bash
# Run all benchmarks
make bench

# Run specific benchmarks
go test -bench=BenchmarkRouting -benchtime=10s ./internal/service/callrouting
go test -bench=BenchmarkBidding -benchmem ./internal/service/bidding

# Profile CPU usage
go test -cpuprofile=cpu.prof -bench=. ./internal/service/callrouting
go tool pprof cpu.prof

# Profile memory
go test -memprofile=mem.prof -bench=. ./internal/service/bidding
go tool pprof mem.prof
```

### Performance Targets

| Operation | Target | Test Threshold |
|-----------|--------|----------------|
| Call Routing | < 1ms | 2ms |
| Bid Processing | < 5ms | 10ms |
| Compliance Check | < 2ms | 5ms |
| API Response | < 50ms | 100ms |

## Test Organization

### Directory Structure

```
internal/
├── domain/
│   └── call/
│       ├── call.go
│       ├── call_test.go          # Unit tests
│       ├── call_property_test.go # Property tests
│       └── call_synctest.go      # Concurrent tests
├── service/
│   └── bidding/
│       ├── service.go
│       ├── service_test.go       # Unit tests
│       └── service_contract_test.go # Contract tests
test/
├── integration/                  # End-to-end tests
├── testutil/
│   └── fixtures/                # Test data builders
└── contract/                    # OpenAPI specs
```

### Test Naming Conventions

- Unit tests: `TestFunctionName`
- Property tests: `TestPropertyName`
- Table tests: `TestScenarioName`
- Integration: `TestE2EFlowName`
- Benchmarks: `BenchmarkOperationName`

## Coverage Requirements

- Minimum line coverage: 80%
- Critical paths: 95%+
- Domain logic: 90%+
- Service orchestration: 85%+

Generate coverage reports:

```bash
make coverage
open coverage.html
```

## Continuous Integration

Tests run automatically on:
- Pull requests
- Main branch commits
- Nightly builds

CI pipeline includes:
1. Compilation checks
2. Unit tests
3. Race detection
4. Integration tests
5. Contract validation
6. Coverage analysis
7. Performance benchmarks

## Best Practices

1. **Write tests first** - TDD approach
2. **Use fixtures** - Never manual test data
3. **Test behaviors** - Not implementation
4. **Isolate tests** - No shared state
5. **Clear names** - Describe what's tested
6. **Fast feedback** - Unit tests < 100ms
7. **Deterministic** - No flaky tests
8. **Document why** - Complex test logic

## Troubleshooting

### Common Issues

**Compilation Errors in Tests**
```bash
# See ALL errors
go build -gcflags="-e" ./...
```

**Flaky Tests**
- Use synctest for concurrent code
- Check for shared state
- Use deterministic time (testutil.Clock)

**Slow Tests**
- Profile with `-cpuprofile`
- Use t.Parallel() where safe
- Mock external dependencies

**Database Tests Failing**
- Check Docker is running
- Verify test database exists
- Look for connection pool exhaustion

For more details, see:
- Contract Testing: `docs/CONTRACT_TESTING.md`
- Performance Testing: `docs/PERFORMANCE_TESTING.md`
- Integration Testing: `docs/INTEGRATION_TESTING.md`