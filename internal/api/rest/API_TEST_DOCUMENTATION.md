# Dependable Call Exchange Backend - API Test Suite Documentation

## Overview

This document provides comprehensive documentation for the API test suite of the Dependable Call Exchange Backend. The test suite is organized into five phases, each focusing on different aspects of API quality and reliability.

## Test Suite Structure

```
internal/api/rest/
├── handlers_test.go              # Base test infrastructure
├── handlers_unit_test.go         # Phase 2: Unit tests
├── handlers_validation_test.go   # Phase 2: Input validation tests
├── handlers_error_test.go        # Phase 2: Error handling tests
├── handlers_integration_test.go  # Phase 2: Integration tests
├── handlers_contract_test.go     # Phase 3: Contract/schema tests
├── handlers_security_test.go     # Phase 3: Security tests
├── handlers_performance_test.go  # Phase 4: Performance tests
└── mocks_test.go                # Mock service implementations
```

## Running the Tests

### Basic Commands

```bash
# Run all tests
make test

# Run unit tests only
make test-unit

# Run with coverage
make coverage

# Run specific test file
go test -v ./internal/api/rest -run TestUnit

# Run performance tests (excluded from short mode)
go test -v ./internal/api/rest -run TestPerformance

# Run with race detection
make test-race

# Run benchmarks
go test -bench=. ./internal/api/rest
```

### Test Categories

#### Unit Tests (`handlers_unit_test.go`)
Tests individual handler functions in isolation with mocked dependencies.

**Coverage:**
- Authentication endpoints (login, logout, refresh)
- Account management (CRUD operations)
- Call lifecycle (create, route, update status, complete)
- Bid profiles and auction management
- Compliance checks (DNC, TCPA)
- Financial operations (balance, transactions)

**Example:**
```go
func TestUnitCreateCall(t *testing.T) {
    // Tests call creation with various scenarios
}
```

#### Validation Tests (`handlers_validation_test.go`)
Comprehensive input validation testing for all data types.

**Coverage:**
- Phone number validation (E.164 format)
- Email validation (RFC 5322)
- Money/currency validation
- UUID format validation
- Date/time format validation
- Enum/status value validation
- Array/list validation
- String length constraints
- Nested object validation

**Example:**
```go
func TestValidationPhoneNumbers(t *testing.T) {
    // Tests E.164 phone number validation
}
```

#### Error Handling Tests (`handlers_error_test.go`)
Tests error scenarios and response consistency.

**Coverage:**
- Domain errors (NotFound, Validation, Business, Conflict)
- Infrastructure errors (timeouts, service unavailable)
- Request errors (bad JSON, invalid methods)
- Panic recovery
- Error response format consistency

**Example:**
```go
func TestErrorHandlingNotFound(t *testing.T) {
    // Tests 404 error responses
}
```

#### Integration Tests (`handlers_integration_test.go`)
Tests complete business flows across multiple endpoints.

**Coverage:**
- Call-to-auction flow
- Compliance verification flow
- Concurrent operations
- Transaction consistency
- Performance requirements validation

**Example:**
```go
func TestIntegrationCallToAuctionFlow(t *testing.T) {
    // Tests complete call lifecycle with auction
}
```

#### Contract Tests (`handlers_contract_test.go`)
Validates API responses against expected schemas.

**Coverage:**
- Response structure validation
- Field type verification
- Pagination response format
- Header validation
- API versioning
- Content negotiation
- Backward compatibility

**Example:**
```go
func TestContractResponseStructure(t *testing.T) {
    // Validates response schemas
}
```

#### Security Tests (`handlers_security_test.go`)
Tests security controls and vulnerability prevention.

**Coverage:**
- Authentication requirements
- Authorization (RBAC)
- Injection prevention (SQL, XSS, NoSQL)
- Rate limiting
- CSRF protection
- Security headers
- Input size limits
- Session management
- Password policy
- Audit logging

**Example:**
```go
func TestSecurityAuthentication(t *testing.T) {
    // Tests auth requirements on protected endpoints
}
```

#### Performance Tests (`handlers_performance_test.go`)
Validates performance requirements and benchmarks.

**Coverage:**
- Call routing latency (< 1ms requirement)
- Bid processing throughput (100K/s target)
- API response times (< 50ms p99)
- Concurrent load handling
- Memory usage and leak detection
- Pagination performance
- Benchmark tests

**Example:**
```go
func TestPerformanceCallRouting(t *testing.T) {
    // Verifies < 1ms routing decision
}
```

## Mock Services

The test suite uses comprehensive mock implementations for all service dependencies:

### SimpleMockServices
Central mock service container providing:
- Account service mocks
- Call service mocks
- Bid service mocks
- Auction service mocks
- Compliance service mocks
- Financial service mocks
- Analytics service mocks
- Fraud detection mocks

### Mock Patterns

```go
// Function-based mocking for flexibility
type MockAccountService struct {
    GetByIDFunc func(ctx context.Context, id uuid.UUID) (*account.Account, error)
    CreateFunc  func(ctx context.Context, req account.CreateRequest) (*account.Account, error)
    // ... other methods
}

// Usage in tests
mocks.accountSvc.GetByIDFunc = func(ctx context.Context, id uuid.UUID) (*account.Account, error) {
    // Custom behavior for this test
    return testAccount, nil
}
```

## Helper Functions

### Test Setup
```go
func setupHandler(t *testing.T) (*Handler, *SimpleMockServices)
```
Creates a handler instance with all mocked dependencies.

### Request Helpers
```go
func makeRequest(handler http.Handler, method, path string, body interface{}) *httptest.ResponseRecorder
```
Simplifies HTTP request creation in tests.

### Context Helpers
```go
func setUserContext(ctx context.Context, userID uuid.UUID, accountType string) context.Context
```
Sets authentication context for protected endpoints.

### Public Endpoint Check
```go
func isPublicEndpoint(path string) bool
```
Identifies endpoints that don't require authentication.

## Test Data

### Standard Test IDs
```go
var (
    testBuyerID  = uuid.MustParse("123e4567-e89b-12d3-a456-426614174000")
    testSellerID = uuid.MustParse("223e4567-e89b-12d3-a456-426614174001")
    testCallID   = uuid.MustParse("323e4567-e89b-12d3-a456-426614174002")
)
```

### Test Request Builders
- `createCallRequest()` - Valid call creation request
- `createBidRequest()` - Valid bid placement request
- `createAuctionRequest()` - Valid auction creation request

## Best Practices

### 1. Table-Driven Tests
Use table-driven tests for comprehensive coverage:
```go
tests := []struct {
    name     string
    input    interface{}
    expected int
    wantErr  bool
}{
    // Test cases
}

for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        // Test implementation
    })
}
```

### 2. Parallel Execution
Enable parallel execution where appropriate:
```go
t.Parallel()
```

### 3. Cleanup
Always cleanup resources:
```go
t.Cleanup(func() {
    // Cleanup code
})
```

### 4. Assertions
Use testify for clear assertions:
```go
assert.Equal(t, expected, actual)
require.NoError(t, err)
```

### 5. Context Handling
Always provide proper context:
```go
ctx := context.Background()
ctx = setUserContext(ctx, userID, "buyer")
```

## Performance Benchmarks

### Target Metrics
- Call routing: < 1ms
- Bid processing: 100K/second
- API response: < 50ms p99
- Concurrent users: 1000+

### Running Benchmarks
```bash
# Run all benchmarks
go test -bench=. ./internal/api/rest

# Run specific benchmark
go test -bench=BenchmarkHandlerRouteCall ./internal/api/rest

# Run with memory profiling
go test -bench=. -benchmem ./internal/api/rest

# Run for specific duration
go test -bench=. -benchtime=10s ./internal/api/rest
```

## Coverage Goals

### Minimum Requirements
- Overall coverage: 80%
- Critical paths: 95%
- Error handling: 90%
- Business logic: 85%

### Checking Coverage
```bash
# Generate coverage report
make coverage

# View coverage in browser
go tool cover -html=coverage.out

# Check specific package coverage
go test -cover ./internal/api/rest
```

## Troubleshooting

### Common Issues

1. **Mock Not Configured**
   ```
   panic: runtime error: invalid memory address or nil pointer dereference
   ```
   Solution: Ensure all required mock functions are set before test execution.

2. **Context Missing User ID**
   ```
   Error: unauthorized
   ```
   Solution: Use `setUserContext()` to add authentication context.

3. **Race Conditions**
   ```
   WARNING: DATA RACE
   ```
   Solution: Use mutex or atomic operations for shared state in concurrent tests.

4. **Timeout in Performance Tests**
   ```
   panic: test timed out after 10m0s
   ```
   Solution: Use `-timeout` flag or skip long tests with `-short`.

## Contributing

### Adding New Tests

1. **Identify test category** (unit, validation, integration, etc.)
2. **Add to appropriate file** or create new file if needed
3. **Follow existing patterns** for consistency
4. **Update documentation** if adding new test categories
5. **Ensure CI passes** before submitting

### Test Naming Convention
- Unit tests: `TestUnit{Feature}{Scenario}`
- Validation tests: `TestValidation{DataType}`
- Integration tests: `TestIntegration{Flow}`
- Performance tests: `TestPerformance{Metric}`
- Benchmarks: `Benchmark{Operation}`

### Mock Guidelines
- Keep mocks simple and focused
- Use function fields for flexibility
- Document complex mock behavior
- Reset mocks between tests if needed

## CI/CD Integration

### GitHub Actions
```yaml
- name: Run Tests
  run: make test
  
- name: Check Coverage
  run: make coverage
  
- name: Upload Coverage
  uses: codecov/codecov-action@v3
```

### Pre-commit Hooks
```bash
#!/bin/sh
# .git/hooks/pre-commit
make test-unit
```

## Future Enhancements

1. **WebSocket Testing** - Real-time event testing
2. **Load Testing** - Using k6 or similar tools
3. **Chaos Testing** - Fault injection and recovery
4. **API Fuzzing** - Security vulnerability detection
5. **Contract Testing** - OpenAPI validation
6. **E2E Testing** - Full system integration tests

## References

- [Go Testing Documentation](https://golang.org/pkg/testing/)
- [Testify Assertion Library](https://github.com/stretchr/testify)
- [HTTP Testing in Go](https://golang.org/pkg/net/http/httptest/)
- [Table-Driven Tests](https://dave.cheney.net/2019/05/07/prefer-table-driven-tests)
- [Go Benchmarking](https://dave.cheney.net/2013/06/30/how-to-write-benchmarks-in-go)
