# E2E Testing with Testcontainers

This directory contains end-to-end tests for the Dependable Call Exchange Backend using Testcontainers for infrastructure management.

## Overview

The E2E tests use Testcontainers to spin up real instances of:
- PostgreSQL with TimescaleDB
- Redis
- Kamailio (SIP server)
- The DCE API server

This provides a production-like environment for testing complete workflows.

## Requirements

- Docker Desktop or Docker Engine
- Go 1.24+
- Make

## Running Tests

### Run all E2E tests
```bash
make test-e2e
```

### Run specific test suites
```bash
make test-e2e-auth        # Authentication and authorization tests
make test-e2e-flow        # Call exchange flow tests
make test-e2e-financial   # Billing and financial tests
make test-e2e-realtime    # WebSocket real-time event tests
make test-e2e-performance # Performance and load tests
```

### Run tests in short mode (excludes performance tests)
```bash
make test-e2e-short
```

### Run tests with coverage
```bash
make test-e2e-coverage
```

### Run tests with race detector
```bash
make test-e2e-race
```

## Test Structure

```
test/e2e/
├── infrastructure/
│   ├── containers.go    # Testcontainers setup
│   └── helpers.go       # API and WebSocket clients
├── auth_test.go         # Authentication/authorization tests
├── call_exchange_flow_test.go  # Core business flow tests
├── financial_test.go    # Billing and payment tests
├── performance_test.go  # Load and latency tests
├── realtime_events_test.go  # WebSocket event tests
└── Makefile            # E2E-specific make targets
```

## Writing New Tests

### Basic Test Structure

```go
func TestFeature_Scenario(t *testing.T) {
    // Setup test environment
    env := infrastructure.NewTestEnvironment(t)
    client := infrastructure.NewAPIClient(t, env.APIURL)
    
    t.Run("Specific Test Case", func(t *testing.T) {
        // Reset database for test isolation
        env.ResetDatabase()
        
        // Test implementation
        // ...
    })
}
```

### Using the API Client

```go
// Make authenticated requests
user := createAuthenticatedUser(t, client, "test@example.com", "buyer")
client.SetToken(user.Token)

// Make API calls
resp := client.Post("/api/v1/calls", map[string]interface{}{
    "from_number": "+14155551234",
    "to_number":   "+18005551234",
})
assert.Equal(t, 201, resp.StatusCode)

// Decode response
var call call.Call
client.DecodeResponse(resp, &call)
```

### Using WebSocket Client

```go
// Connect WebSocket
ws := infrastructure.NewWebSocketClient(t, env.WSURL+"/ws/bidding")
err := ws.Connect(clientID.String())
require.NoError(t, err)
defer ws.Close()

// Send messages
ws.Send(map[string]interface{}{
    "action": "subscribe",
    "type":   "auction",
})

// Receive events
var event WebSocketEvent
ws.Receive(&event)
```

## Performance Testing

Performance tests are excluded by default in PR builds. To run them:

```bash
# Run full performance suite
make test-e2e-performance

# Run with custom parameters
go test -tags=e2e -timeout=60m -v -run TestPerformance ./test/e2e/
```

Performance targets:
- Call routing: < 1ms
- Bid processing: 100K/second
- API response: < 50ms P99
- WebSocket delivery: < 5ms

## Debugging

### View Container Logs

During test failures, container logs are preserved. Access them:

```go
// In your test
t.Logf("API Container Logs: %s", env.GetAPILogs())
```

### Keep Containers Running

For debugging, you can keep containers running after tests:

```bash
# Set environment variable
export DCE_E2E_KEEP_CONTAINERS=true
make test-e2e-auth

# Manually clean up later
make docker-clean
```

### Database Access

Connect to the test database:
```bash
# Get container ID
docker ps | grep dce_test_postgres

# Connect
docker exec -it <container_id> psql -U test -d dce_test
```

## CI/CD Integration

The E2E tests run automatically on:
- Pull requests (excluding performance tests)
- Merges to main
- Scheduled runs every 4 hours (full suite)

See `.github/workflows/e2e-tests.yml` for configuration.

## Troubleshooting

### "Cannot connect to Docker daemon"

Ensure Docker is running:
```bash
docker info
```

### "Container failed to start"

Check Docker resources:
```bash
docker system df
docker system prune
```

### "Timeout waiting for container"

Increase timeouts in `infrastructure/containers.go`:
```go
wait.ForSQL(...).WithStartupTimeout(120*time.Second)
```

### Test Flakiness

For flaky tests, use retry logic:
```go
require.Eventually(t, func() bool {
    // Check condition
    return condition == expected
}, 10*time.Second, 100*time.Millisecond)
```

## Contributing

When adding new E2E tests:

1. Place tests in appropriate files by domain
2. Use `env.ResetDatabase()` for test isolation
3. Create helper functions for common operations
4. Document any new infrastructure requirements
5. Update this README with new test scenarios

## Performance Considerations

- Tests run in parallel by default (use `-p 1` to disable)
- Each test suite gets its own container set
- Database is reset between test cases
- Containers are reused within a test suite
- Cleanup happens automatically via `t.Cleanup()`
