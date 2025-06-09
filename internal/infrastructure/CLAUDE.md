# Infrastructure Layer Context

## Directory Structure
- `config/` - Koanf layered configuration management
- `database/` - PostgreSQL connections and migrations
- `repository/` - Data access implementations
- `cache/` - Redis caching layer (planned)
- `messaging/` - Kafka/NATS event streaming (planned)
- `telemetry/` - Logging and metrics

## Database Patterns

### Connection Management
- Use `pgx/v5` directly (not through database/sql)
- Connection pooling with configurable limits
- Prepared statements for repeated queries
- Context-based timeouts on all queries

### Schema Conventions
- UUID primary keys using `uuid-ossp` extension
- JSONB columns for flexible data (criteria, settings)
- PostgreSQL enum types for statuses
- `created_at` and `updated_at` on all tables
- Update triggers for automatic timestamp updates

### Repository Implementation
- One repository per aggregate root
- Use `sqlbuilder` for complex dynamic queries
- Return domain errors, not database errors
- Transaction support via `WithTx` pattern
- Batch operations for performance

## Configuration Hierarchy
1. Default values in code
2. `configs/config.yaml` file
3. Environment variables (DCE_ prefix)
4. Command-line flags (highest priority)

## When Implementing New Infrastructure

### Cache Layer
- Use Redis for session and frequently accessed data
- Implement cache-aside pattern
- Set appropriate TTLs based on data type
- Include cache warming strategies

### Message Queue
- Kafka for high-throughput event streaming
- NATS for request-reply patterns
- Implement retry logic with exponential backoff
- Dead letter queues for failed messages

### Observability
- OpenTelemetry for distributed tracing
- Prometheus metrics with custom collectors
- Structured logging with correlation IDs
- Health check endpoints for each component