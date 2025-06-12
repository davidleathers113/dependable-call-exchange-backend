# API Test Suite - Quick Start Guide

## 🚀 Quick Test Execution

### Run All Tests
```bash
# Complete test suite
make test

# With verbose output
make test ARGS="-v"

# With coverage
make coverage
```

### Run Specific Test Categories

```bash
# Unit tests only
go test -v ./internal/api/rest -run "TestUnit"

# Validation tests
go test -v ./internal/api/rest -run "TestValidation"

# Error handling tests
go test -v ./internal/api/rest -run "TestError"

# Integration tests
go test -v ./internal/api/rest -run "TestIntegration"

# Contract tests
go test -v ./internal/api/rest -run "TestContract"

# Security tests
go test -v ./internal/api/rest -run "TestSecurity"

# Performance tests (longer running)
go test -v ./internal/api/rest -run "TestPerformance" -timeout 30m
```

### Run Benchmarks
```bash
# All benchmarks
go test -bench=. ./internal/api/rest

# Specific benchmark with memory allocation info
go test -bench=BenchmarkHandlerRouteCall -benchmem ./internal/api/rest

# Run benchmarks for 30 seconds
go test -bench=. -benchtime=30s ./internal/api/rest
```

## 📊 Test Coverage Analysis

### Generate Coverage Report
```bash
# Generate coverage data
go test -coverprofile=coverage.out ./internal/api/rest

# View coverage in terminal
go tool cover -func=coverage.out

# View coverage in browser (recommended)
go tool cover -html=coverage.out
```

### Coverage by Function
```bash
# See uncovered lines
go test -coverprofile=coverage.out ./internal/api/rest
grep -E "0$" coverage.out | head -20
```

## 🏃 Performance Testing

### Quick Performance Check
```bash
# Run core performance tests
go test -v -run "TestPerformanceCallRouting|TestPerformanceAPIResponseTime" ./internal/api/rest
```

### Full Performance Suite
```bash
# All performance tests with extended timeout
go test -v -run "TestPerformance" -timeout 60m ./internal/api/rest
```

### Memory Profiling
```bash
# Generate memory profile
go test -memprofile=mem.prof -run "TestPerformanceMemoryUsage" ./internal/api/rest

# Analyze memory profile
go tool pprof mem.prof
```

## 🔒 Security Testing

### Run Security Tests
```bash
# All security tests
go test -v -run "TestSecurity" ./internal/api/rest

# Specific security aspects
go test -v -run "TestSecurityAuthentication" ./internal/api/rest
go test -v -run "TestSecurityInjection" ./internal/api/rest
go test -v -run "TestSecurityRateLimit" ./internal/api/rest
```

## 🐛 Debugging Failed Tests

### Verbose Output
```bash
# Run with detailed output
go test -v -run "TestName" ./internal/api/rest
```

### Run Single Test
```bash
# Run specific test by exact name
go test -v -run "^TestUnitCreateCall$" ./internal/api/rest
```

### Debug with Delve
```bash
# Install delve
go install github.com/go-delve/delve/cmd/dlv@latest

# Debug a test
dlv test ./internal/api/rest -- -test.run TestUnitCreateCall
```

## 📈 Continuous Monitoring

### Watch Mode (with external tool)
```bash
# Install goconvey or similar
go get github.com/smartystreets/goconvey

# Run in watch mode
goconvey -packages ./internal/api/rest
```

### Pre-commit Hook
```bash
# Add to .git/hooks/pre-commit
#!/bin/bash
echo "Running unit tests..."
go test -short ./internal/api/rest || exit 1
```

## 🎯 Test Selection Guidelines

### For Different Scenarios

**Quick Smoke Test** (< 1 minute)
```bash
go test -short ./internal/api/rest
```

**Pre-commit Check** (< 5 minutes)
```bash
make test-unit
```

**Full Regression** (< 30 minutes)
```bash
make test
```

**Complete Suite with Performance** (60+ minutes)
```bash
make test && make test-performance
```

## 📝 Common Issues and Solutions

### Issue: Tests Timeout
```bash
# Increase timeout
go test -timeout 30m ./internal/api/rest
```

### Issue: Race Conditions
```bash
# Run with race detector
go test -race ./internal/api/rest
```

### Issue: Flaky Tests
```bash
# Run multiple times to identify flaky tests
go test -count=10 -run "TestName" ./internal/api/rest
```

### Issue: Port Already in Use
```bash
# Find process using port
lsof -i :8080
# or
netstat -tlnp | grep 8080
```

## 🔧 Environment Setup

### Required Environment Variables
```bash
# Set test environment
export DCE_ENVIRONMENT=test
export DCE_LOG_LEVEL=error  # Reduce log noise during tests
```

### Docker Dependencies
```bash
# Start test dependencies
make docker-compose-test-up

# Stop after tests
make docker-compose-test-down
```

## 📊 Test Metrics

### Key Metrics to Monitor
- **Coverage**: Target 80% overall, 95% for critical paths
- **Execution Time**: Unit tests < 5s, Integration < 30s
- **Memory Usage**: No leaks, stable under load
- **Performance**: Meet SLA requirements

### Generate Test Report
```bash
# Run tests with JSON output
go test -json ./internal/api/rest > test-results.json

# Use go-junit-report for CI
go get -u github.com/jstemmer/go-junit-report
go test -v ./internal/api/rest | go-junit-report > test-results.xml
```

## 🚦 CI/CD Integration

### GitHub Actions Example
```yaml
- name: Run API Tests
  run: |
    make test-unit
    make test-integration
    make coverage
```

### Jenkins Pipeline
```groovy
stage('API Tests') {
    steps {
        sh 'make test'
        publishHTML([
            reportDir: 'coverage',
            reportFiles: 'index.html',
            reportName: 'Coverage Report'
        ])
    }
}
```

## 💡 Best Practices

1. **Run relevant tests frequently** - Unit tests after each change
2. **Use test caching** - Go caches test results for unchanged code
3. **Parallelize when possible** - Use `t.Parallel()` for independent tests
4. **Profile before optimizing** - Use benchmarks to identify bottlenecks
5. **Keep tests focused** - One concept per test
6. **Mock external dependencies** - Keep tests isolated and fast

---

For detailed documentation, see [API_TEST_DOCUMENTATION.md](./API_TEST_DOCUMENTATION.md)
