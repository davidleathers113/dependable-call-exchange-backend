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
# Testing
make test                 # Run all tests
make test-race           # Run tests with race detection
make coverage            # Generate coverage report
make bench               # Run benchmarks
go test ./internal/domain/call -v  # Test specific package

# Code quality
make lint                # Run golangci-lint
make fmt                 # Format code
make vet                 # Run go vet
make security            # Run gosec and govulncheck
make ci                  # Run full CI pipeline
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

### Testing Strategy
- Unit tests alongside code (not yet implemented)
- Integration tests in `test/` directory
- Table-driven test patterns
- Testify for assertions
- Mock generation with mockery (when needed)

## Environment Variables

Critical environment variables:
```bash
DCE_DATABASE_URL          # PostgreSQL connection string
DCE_REDIS_URL            # Redis connection
DCE_KAFKA_BROKERS        # Comma-separated Kafka brokers
DCE_SECURITY_JWT_SECRET  # JWT signing key
DCE_ENVIRONMENT          # development/staging/production
DCE_LOG_LEVEL           # debug/info/warn/error
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