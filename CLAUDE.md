# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Essential Commands

### Building & Running
```bash
# Run the application
make dev                    # Run in development mode
make dev-watch             # Run with hot reload (requires air)
go run main.go             # Direct run

# Build binaries  
make build                 # Build for current OS
make build-linux          # Build for Linux (production)

# Docker operations
make docker-build         # Build Docker image
make docker-run          # Run Docker container
docker-compose up -d     # Start all services
docker-compose -f docker-compose.dev.yml up  # Development environment
```

### Testing & Quality
```bash
# ENHANCED 2025 TDD TESTING
make test                    # Basic unit tests
make test-race              # Race condition testing  
make test-synctest          # Go 1.24 deterministic concurrent tests (GOEXPERIMENT=synctest)
make test-integration       # End-to-end with real database
make test-property          # Property-based testing with randomized inputs
make coverage               # Coverage reports
make coverage-synctest      # Coverage with synctest
make bench                  # Performance benchmarks
make bench-property         # Property-based benchmarks

# Docker testing environment
docker-compose -f docker-compose.test.yml up --abort-on-container-exit

# Code quality
make lint                # Run golangci-lint
make fmt                 # Format code
make vet                 # Run go vet
make security            # Run gosec and govulncheck
make ci                  # Run full CI pipeline

# COMPILATION ERROR DEBUGGING (CRITICAL)
# Go compiler stops at 10 errors by default - use these to see ALL errors:
go build -gcflags="-e" ./...                    # Show all compilation errors across project
go test -run=xxx ./... 2>&1 | grep -E "(cannot use|undefined|unknown field)"  # Find test compilation errors
```

### Database Operations
```bash
# Migrations
go run cmd/migrate/main.go -action up              # Apply all migrations
go run cmd/migrate/main.go -action down -steps 1   # Rollback 1 migration
go run cmd/migrate/main.go -action status          # Check migration status
go run cmd/migrate/main.go -action create -name "add_feature"  # Create new migration
```

### Development Tools
```bash
make install-tools       # Install air, golangci-lint, gosec, govulncheck
make deps               # Download and verify dependencies
```

## High-Level Architecture

### Domain Structure (DDD)
The codebase follows Domain-Driven Design with clear bounded contexts:

- **Call Domain** (`internal/domain/call/`): Core call entity with lifecycle management. Handles status transitions, location tracking, and routing associations.

- **Bid Domain** (`internal/domain/bid/`): Auction and bidding logic. Implements real-time auction mechanics with criteria matching and quality metrics.

- **Account Domain** (`internal/domain/account/`): User management for buyers/sellers. Includes balance tracking, quality scores, and account settings.

- **Compliance Domain** (`internal/domain/compliance/`): TCPA/GDPR rule engine. Manages consent records, violation tracking, and time-based restrictions.

### Service Layer Pattern
Services in `internal/service/` orchestrate domain logic:
- **CallRouting**: Implements routing algorithms (Round Robin, Skill-Based, Cost-Based)
- **Bidding**: Real-time auction engine with millisecond bid processing
- **Fraud**: ML-based fraud detection pipeline
- **Telephony**: SIP/WebRTC protocol handling

### Infrastructure Separation
`internal/infrastructure/` isolates external dependencies:
- **Config**: Layered configuration using Koanf (defaults → YAML → env vars)
- **Database**: PostgreSQL repositories with migration tooling
- **Messaging**: Kafka/NATS event streaming
- **Cache**: Redis for session management and rate limiting

### API Layer Architecture
Three distinct API protocols in `internal/api/`:
- **REST**: Management operations, CRUD endpoints
- **gRPC**: High-performance internal service communication
- **WebSocket**: Real-time bidding and event streaming

### Configuration Hierarchy
1. Default values in code
2. `configs/config.yaml` for environment settings
3. Environment variables with `DCE_` prefix
4. Command-line flags (highest priority)

### Error Handling Pattern
Structured errors in `internal/domain/errors/`:
- Custom `AppError` type with error codes, HTTP status, and retry logic
- Error wrapping with `fmt.Errorf` and `%w` verb
- Domain-specific error types (ValidationError, ComplianceError, etc.)

### Database Schema
PostgreSQL with:
- UUID primary keys using `uuid-ossp` extension
- Enum types for statuses and types
- JSONB for flexible settings/criteria storage
- Comprehensive indexes for query performance
- Update triggers for `updated_at` timestamps

### Event-Driven Patterns
- Kafka for high-throughput event streaming
- Domain events for audit trails
- Asynchronous processing for non-critical paths
- Event sourcing preparation in architecture

### Testing Strategy (2025 Best Practices)
**COMPREHENSIVE TDD IMPLEMENTATION** - This project demonstrates cutting-edge Go testing:

1. **Unit Tests** (`*_test.go`): Fast, isolated tests alongside source files with table-driven patterns
2. **Property-Based Tests** (`*_property_test.go`): Randomized testing using `testing/quick` for edge case discovery  
3. **Concurrent Tests** (`*_synctest.go`): **NEW Go 1.24** - Deterministic concurrent testing with `testing/synctest`
4. **Integration Tests** (`test/integration/`): End-to-end workflows with real PostgreSQL database
5. **Enhanced Test Infrastructure**:
   - Fluent fixture builders (`CallBuilder`, `BidBuilder`) 
   - Rich mock infrastructure with expectation helpers
   - Automatic test database creation/cleanup
   - Transaction-based test isolation

**Key Testing Features**:
- **Synctest**: Eliminates flaky concurrent tests with deterministic timing
- **Property Testing**: 1000+ randomized inputs per property for invariant verification
- **Test Coverage**: >90% line coverage with branch analysis
- **Performance Testing**: Comprehensive benchmarks with memory profiling

See `TESTING.md` for detailed testing guide and TDD workflow.

## Environment Variables

Critical environment variables:
```bash
# Core services
DCE_DATABASE_URL          # PostgreSQL connection string
DCE_REDIS_URL            # Redis connection
DCE_KAFKA_BROKERS        # Comma-separated Kafka brokers
DCE_SECURITY_JWT_SECRET  # JWT signing key
DCE_ENVIRONMENT          # development/staging/production
DCE_LOG_LEVEL           # debug/info/warn/error

# Testing (2025 Enhanced)
GOEXPERIMENT=synctest      # Enable Go 1.24 deterministic concurrent testing
DCE_TEST_DATABASE_URL     # Test database connection (auto-created)
```

## Performance Targets

The system is designed for:
- < 1ms call routing decisions
- < 5ms bid processing
- 10,000+ calls/second throughput
- 100,000+ concurrent connections

## Development Workflow

1. Branch from `main` using conventional naming: `feat/`, `fix/`, `docs/`
2. Run `make ci` before committing
3. Use conventional commits: `feat:`, `fix:`, `docs:`
4. Ensure migrations are backward compatible
5. Update API documentation for endpoint changes

## Debugging & Troubleshooting

### Finding All Compilation Errors
**CRITICAL**: Go's compiler stops after 10 errors by default. For comprehensive error detection:

```bash
# Show ALL compilation errors (not just first 10)
go build -gcflags="-e" ./...

# Find test compilation errors without running tests  
go test -run=xxx ./... 2>&1 | grep -E "(cannot use|undefined|unknown field)"

# Alternative: Use go-compiles tool for comprehensive checking
# Checks both source AND test files across all packages
```

### Common Value Object Conversion Patterns
When updating code to use value objects, watch for these patterns:

```go
// OLD: Primitive obsession
Amount: 10.50
QualityScore: 85.5

// NEW: Value objects  
Amount: values.MustNewMoneyFromFloat(10.50, "USD")
QualityMetrics: values.QualityMetrics{QualityScore: 85.5}
```

### Test Database Functions
```go
// Current pattern in codebase
testDB := testutil.NewTestDB(t)        // Primary function
testDB := testutil.CreateTestDB(t)     // Alternative for some tests
```

## Memories

- Check context7 mcp server for Go's documentation often so that you can meet the standard of strict adherence to official Go docs.
- Remember to use the fixture builders internal/testutil/fixtures/ wherever we have manual data setup so that all of our tests can be cleaner and more robust. Also, use sub-tests where appropriate and always seek to reduce boilerplate code when there's an established best practice to follow that would naturally reduce boilerplate code.
- Use clearer naming conventions that distinguish between routing directions like "buyers" and "sellers"
- Never use sed commands to modify my files
- **ALWAYS use `go build -gcflags="-e" ./...` to find ALL compilation errors before starting fixes**

## Anchor Comments

Use these prefixes throughout the codebase for searchable inline knowledge:
- `AIDEV-NOTE:` - Important implementation details
- `AIDEV-TODO:` - Tasks to complete  
- `AIDEV-QUESTION:` - Clarifications needed

Always grep for existing `AIDEV-*` comments before scanning files.

## Subdirectory Context

This project uses nested CLAUDE.md files for area-specific guidance:

- **internal/api/CLAUDE.md**: API implementation status (currently empty)
- **internal/domain/CLAUDE.md**: Domain entities and known issues
- **internal/infrastructure/CLAUDE.md**: Database patterns and missing components
- **internal/service/CLAUDE.md**: Service anti-patterns and refactoring priorities
- **test/CLAUDE.md**: Current testing issues and patterns
- **cmd/CLAUDE.md**: CLI tools and migration status

Claude Code will automatically include these when working in those directories.