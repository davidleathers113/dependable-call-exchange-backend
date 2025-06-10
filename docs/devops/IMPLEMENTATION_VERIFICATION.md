# DevOps Implementation Verification Guide

This document provides verification commands, quick start guides, and troubleshooting tips for all implemented DevOps tools in the Dependable Call Exchange backend.

## 📋 Tool Implementation Status

### ✅ Implemented Tools

| Tool | Purpose | Verification Command | Status |
|------|---------|---------------------|---------|
| Testcontainers | Integration testing with real databases | `go test ./internal/infrastructure/database -run TestWithContainer` | ✅ Implemented |
| golangci-lint | Multi-linter aggregator | `make lint` | ✅ Implemented |
| gosec | Security vulnerability scanner | `make security` | ✅ Implemented |
| govulncheck | Go vulnerability database checker | `make security` | ✅ Implemented |
| go-migrate | Database migration management | `go run cmd/migrate/main.go -action status` | ✅ Implemented |
| air | Hot reload for development | `make dev-watch` | ✅ Implemented |
| synctest | Deterministic concurrent testing | `make test-synctest` | ✅ Implemented |
| property testing | Randomized invariant testing | `make test-property` | ✅ Implemented |

### 🚧 Pending Tools (from TODO.md)

| Tool | Purpose | Implementation Priority |
|------|---------|------------------------|
| Prometheus | Metrics collection | High |
| Grafana | Metrics visualization | High |
| Jaeger | Distributed tracing | Medium |
| gomock | Mock generation | Medium |
| go-swagger | API documentation | Medium |
| dlv | Go debugger | Low |
| pprof | Performance profiling | Low |

## 🚀 Quick Start Guides

### 1. Testcontainers

**Purpose**: Run integration tests with real PostgreSQL containers

```bash
# Run all integration tests with containers
make test-integration

# Run specific test with container
go test -v ./internal/infrastructure/database -run TestDatabaseIntegration

# Run with custom PostgreSQL version
POSTGRES_VERSION=15 go test ./... -tags integration
```

**Verification**:
```bash
# Check if Docker is running
docker ps

# Verify testcontainers in tests
grep -r "testcontainers" internal/testutil/
```

### 2. golangci-lint

**Purpose**: Comprehensive code quality checks

```bash
# Run all linters
make lint

# Run specific linter
golangci-lint run --enable-only=gofmt

# Auto-fix issues
golangci-lint run --fix

# Check configuration
cat .golangci.yml
```

**Common Linters**:
- `gofmt`: Code formatting
- `govet`: Suspicious constructs
- `ineffassign`: Ineffectual assignments
- `staticcheck`: Static analysis
- `gosec`: Security issues

### 3. Security Tools (gosec & govulncheck)

**Purpose**: Identify security vulnerabilities

```bash
# Run both security tools
make security

# Run gosec only
gosec -fmt=json -out=security-report.json ./...

# Run govulncheck only
govulncheck ./...

# Check specific package
gosec ./internal/api/...
```

**Common Security Issues**:
- SQL injection risks
- Hardcoded credentials
- Weak crypto algorithms
- Path traversal vulnerabilities

### 4. Database Migrations (go-migrate)

**Purpose**: Version-controlled database schema changes

```bash
# Check migration status
go run cmd/migrate/main.go -action status

# Apply all pending migrations
go run cmd/migrate/main.go -action up

# Rollback last migration
go run cmd/migrate/main.go -action down -steps 1

# Create new migration
go run cmd/migrate/main.go -action create -name "add_user_preferences"

# Force specific version
go run cmd/migrate/main.go -action force -version 20250608000002
```

**Migration Files Location**: `migrations/`

### 5. Hot Reload (air)

**Purpose**: Automatic rebuild on file changes

```bash
# Start with hot reload
make dev-watch

# Check air configuration
cat .air.toml

# Run with custom config
air -c .air.dev.toml

# Debug air issues
air -d
```

### 6. Synctest (Go 1.24)

**Purpose**: Deterministic concurrent testing

```bash
# Run tests with synctest
make test-synctest

# Run specific synctest
GOEXPERIMENT=synctest go test -v ./internal/service/callrouting -run TestConcurrent

# Generate synctest coverage
make coverage-synctest
```

**Example Synctest Pattern**:
```go
func TestConcurrentBidding(t *testing.T) {
    t.Run("deterministic", func(t *testing.T) {
        testing.Synctest(t)
        // Concurrent test logic here
    })
}
```

### 7. Property-Based Testing

**Purpose**: Find edge cases with randomized inputs

```bash
# Run all property tests
make test-property

# Run specific property test
go test -v ./internal/domain/values -run TestMoneyProperty

# Run with more iterations
go test -quick.count=10000 ./...

# Benchmark property tests
make bench-property
```

## 🔧 CI/CD Integration

### GitHub Actions Workflow

```yaml
name: CI Pipeline

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.24'
          
      - name: Install tools
        run: make install-tools
        
      - name: Run linters
        run: make lint
        
      - name: Security scan
        run: make security
        
      - name: Unit tests
        run: make test
        
      - name: Integration tests
        run: make test-integration
        
      - name: Property tests
        run: make test-property
        
      - name: Coverage report
        run: make coverage
```

### Local CI Simulation

```bash
# Run full CI pipeline locally
make ci

# Individual CI steps
make lint && make security && make test && make test-integration
```

## 🐛 Troubleshooting

### Testcontainers Issues

**Problem**: "Cannot connect to Docker"
```bash
# Solution 1: Start Docker Desktop
open -a Docker  # macOS

# Solution 2: Check Docker socket
ls -la /var/run/docker.sock

# Solution 3: Use Docker in Docker
export TESTCONTAINERS_RYUK_DISABLED=true
```

**Problem**: "Container startup timeout"
```bash
# Increase timeout
export TESTCONTAINERS_POSTGRES_WAIT_TIME=120s
```

### Linting Issues

**Problem**: "File is not gofmt-ed"
```bash
# Auto-fix formatting
make fmt
# or
gofmt -w .
```

**Problem**: "Linter timeout"
```bash
# Increase timeout in .golangci.yml
timeout: 5m
```

### Migration Issues

**Problem**: "Migration already applied"
```bash
# Check current version
go run cmd/migrate/main.go -action status

# Force version if needed
go run cmd/migrate/main.go -action force -version <version>
```

**Problem**: "Migration checksum mismatch"
```bash
# Dangerous: Only in development
go run cmd/migrate/main.go -action dirty
```

### Security Scan Issues

**Problem**: "G104: Unhandled errors"
```bash
# Fix by handling errors
if err != nil {
    return fmt.Errorf("operation failed: %w", err)
}
```

**Problem**: "G601: Implicit memory aliasing"
```bash
# Fix by creating explicit copy
for i := range items {
    item := items[i]  // Create copy
    go process(&item)
}
```

## 📊 Success Metrics Mapping

Mapping to TODO.md Phase 5 success metrics:

### 1. Code Quality (Target: >90% coverage, A rating)
```bash
# Current coverage
make coverage  # Output: coverage.html

# Quality score
make lint  # 0 errors = A rating
```

### 2. Security (Target: 0 critical vulnerabilities)
```bash
# Security audit
make security | grep -c "Severity: HIGH\|CRITICAL"  # Should be 0
```

### 3. Performance (Target: <1ms routing, <5ms bid processing)
```bash
# Run benchmarks
make bench | grep -E "BenchmarkRouting|BenchmarkBidding"

# Expected output:
# BenchmarkRouting-8    1000000    980 ns/op     # <1ms ✅
# BenchmarkBidding-8     300000   4200 ns/op     # <5ms ✅
```

### 4. Reliability (Target: 99.9% uptime)
```bash
# Test reliability
make test-race  # No race conditions
make test-integration  # All passing
```

### 5. Scalability (Target: 10k calls/sec)
```bash
# Load test (requires pending tools)
# TODO: Implement with Vegeta or k6
```

## 🔄 Daily Verification Checklist

```bash
#!/bin/bash
# save as scripts/verify-devops.sh

echo "🔍 DevOps Tool Verification"
echo "=========================="

# 1. Check Docker
echo -n "Docker: "
docker --version >/dev/null 2>&1 && echo "✅" || echo "❌"

# 2. Check Go version
echo -n "Go 1.24: "
go version | grep -q "go1.24" && echo "✅" || echo "❌"

# 3. Check tools
echo -n "golangci-lint: "
which golangci-lint >/dev/null && echo "✅" || echo "❌"

echo -n "gosec: "
which gosec >/dev/null && echo "✅" || echo "❌"

echo -n "govulncheck: "
which govulncheck >/dev/null && echo "✅" || echo "❌"

echo -n "air: "
which air >/dev/null && echo "✅" || echo "❌"

# 4. Run quick tests
echo -e "\n📋 Running Quick Tests..."
make lint >/dev/null 2>&1 && echo "Linting: ✅" || echo "Linting: ❌"
make test-race >/dev/null 2>&1 && echo "Race Detection: ✅" || echo "Race Detection: ❌"

echo -e "\n✨ Verification Complete!"
```

## 📚 Additional Resources

- [Testcontainers Go Documentation](https://golang.testcontainers.org/)
- [golangci-lint Configuration](https://golangci-lint.run/usage/configuration/)
- [gosec Rule Set](https://securego.io/docs/rules/rule-intro.html)
- [go-migrate CLI Usage](https://github.com/golang-migrate/migrate/tree/master/cmd/migrate)
- [air Configuration Guide](https://github.com/cosmtrek/air/blob/master/air_example.toml)
- [Go 1.24 Synctest Design](https://go.dev/issue/67434)

## 🎯 Next Steps

1. **Immediate**: Verify all implemented tools are working
2. **This Week**: Integrate security scanning into pre-commit hooks
3. **Next Sprint**: Implement Prometheus/Grafana monitoring stack
4. **Future**: Add distributed tracing with Jaeger

---

*Last Updated: January 2025*
*Maintainer: DevOps Team*