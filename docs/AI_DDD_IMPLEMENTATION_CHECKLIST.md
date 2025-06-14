# AI Agent DDD Implementation Checklist

A quick-reference checklist for AI agents implementing new domains or services in DDD-style Go applications. Use this to avoid common compilation errors and ensure completeness.

## 🎯 Pre-Implementation Checklist

Before writing any code:
- [ ] Run `go build -gcflags="-e" ./...` to see existing compilation state
- [ ] Review existing domain patterns in similar bounded contexts
- [ ] Check for existing interfaces that must be satisfied
- [ ] Identify value objects that will be reused (PhoneNumber, Email, Money)

## 📦 Domain Layer Checklist

### For Each Custom Type (Status, Type, etc.)
- [ ] Define the type: `type StatusType string`
- [ ] Add constants: `const (StatusActive StatusType = "active")`
- [ ] Implement `String() string` method
- [ ] Implement `IsValid() bool` method  
- [ ] Create `ParseXXX(string) (Type, error)` function
- [ ] Add `MarshalJSON` and `UnmarshalJSON` if needed

### For Each Value Object
- [ ] Constructor with validation: `NewPhoneNumber(string) (PhoneNumber, error)`
- [ ] String() method for display
- [ ] Validation method
- [ ] Equals method for comparison
- [ ] Consider nil-safe pointer helpers

### For Each Entity
- [ ] ID field (usually uuid.UUID)
- [ ] Timestamps (CreatedAt, UpdatedAt)
- [ ] Constructor with validation
- [ ] State transition methods (e.g., Activate(), Suspend())
- [ ] Validation of invariants
- [ ] No direct field access - use methods

### for Each Aggregate
- [ ] Root entity with ID
- [ ] Collection of events (if event sourced)
- [ ] Version/revision tracking
- [ ] Command methods that return errors
- [ ] Query methods for state inspection:
  - [ ] `IsActive() bool`
  - [ ] `GetCurrentState() State`
  - [ ] `GetVersion() int`
- [ ] Event emission methods
- [ ] Invariant protection in all methods

### For Each Domain Event
- [ ] Event type with clear name (ConsentGrantedEvent)
- [ ] Include aggregate ID
- [ ] Include actor/user ID if relevant
- [ ] Include before/after state for updates
- [ ] Include timestamp
- [ ] Include enough context for consumers
- [ ] Consider event versioning

### For Repository Interfaces
- [ ] Save(ctx, aggregate) error
- [ ] GetByID(ctx, id) (*Aggregate, error)
- [ ] Delete(ctx, id) error (if needed)
- [ ] Domain-specific finders (GetByPhoneNumber, etc.)
- [ ] Return domain errors (not SQL errors)
- [ ] Consider separating read/write interfaces

## 🔧 Service Layer Checklist

### Service Structure
- [ ] Interface definition first
- [ ] Maximum 5 dependencies (constructor injection)
- [ ] Logger as first parameter
- [ ] Context as first method parameter
- [ ] Return domain objects or DTOs, not internals

### Service Methods
- [ ] Orchestration only - no business logic
- [ ] Transaction boundaries clearly defined
- [ ] Error wrapping with context
- [ ] Logging at appropriate levels
- [ ] Event publishing after state changes
- [ ] Metrics/telemetry hooks

### DTOs (Data Transfer Objects)
- [ ] Request DTOs with validation tags
- [ ] Response DTOs without domain complexity
- [ ] No domain objects in API responses
- [ ] Explicit mapper functions:
  - [ ] `MapDomainToResponse()`
  - [ ] `MapRequestToDomain()`
- [ ] Handle nil/empty cases in mappers

### Integration Points
- [ ] Define interfaces for external services
- [ ] Mock implementations for testing
- [ ] Error handling for external failures
- [ ] Timeout/retry configuration
- [ ] Circuit breaker pattern where appropriate

## 🏗️ Infrastructure Layer Checklist

### Repository Implementations
- [ ] Implement all interface methods
- [ ] Use exact method names from interface
- [ ] Transaction support
- [ ] Proper error wrapping:
  - [ ] Return `errors.NewNotFound()` for missing entities
  - [ ] Return `errors.NewConflict()` for uniqueness violations
- [ ] Connection pool management
- [ ] Query timeouts

### Event Publishing
- [ ] Event store interface implementation
- [ ] Event bus abstraction
- [ ] Retry logic for failed publishes
- [ ] Event ordering guarantees
- [ ] Dead letter queue handling

## 🌐 API Layer Checklist

### REST Endpoints
- [ ] Follow RESTful conventions
- [ ] Input validation before service calls
- [ ] Proper HTTP status codes:
  - [ ] 201 for successful creation
  - [ ] 404 for not found
  - [ ] 409 for conflicts
  - [ ] 422 for validation errors
- [ ] Consistent error response format
- [ ] Request/response logging
- [ ] OpenAPI documentation

### Common Compilation Error Prevention

Before committing:
- [ ] Run `go build -gcflags="-e" ./...` 
- [ ] All custom types have String() methods
- [ ] All parser functions exist
- [ ] All imports are present (especially `fmt`, `time`, `uuid`)
- [ ] Repository interfaces match implementations exactly
- [ ] Event fields match what consumers expect
- [ ] No direct struct field access across layers
- [ ] Type conversions are explicit

## 🧪 Testing Checklist

### Unit Tests
- [ ] Table-driven tests for domain logic
- [ ] Property-based tests for invariants
- [ ] Edge case coverage
- [ ] Error condition testing

### Integration Tests
- [ ] Repository tests with test database
- [ ] Service tests with mocked dependencies
- [ ] API tests with full stack

### Test Helpers
- [ ] Fixture builders for complex objects
- [ ] Test data generators
- [ ] Mock implementations
- [ ] Test-specific configurations

## 📋 Final Verification

Before marking implementation complete:

```bash
# No compilation errors
go build -gcflags="-e" ./...

# Tests pass
go test ./...

# Linting passes
golangci-lint run

# Check for common issues
grep -r "TODO" ./
grep -r "panic" ./
grep -r "fmt.Print" ./  # Should use logger
```

## 🚨 Red Flags to Address

If you see these patterns, fix immediately:

1. **Business logic in services** → Move to domain
2. **SQL in domain layer** → Move to infrastructure  
3. **Direct struct access** → Use methods
4. **Missing error handling** → Add proper error returns
5. **No validation** → Add to constructors
6. **Circular dependencies** → Restructure interfaces
7. **God objects** → Split into smaller aggregates
8. **Anemic domain models** → Add behavior to entities

## 📝 Documentation to Create

For each new domain:
- [ ] README with domain overview
- [ ] Architecture Decision Records (ADRs) for design choices
- [ ] API documentation
- [ ] Event catalog
- [ ] Error code reference
- [ ] Example usage code

Use this checklist when implementing new features to ensure consistency and prevent common compilation errors. Update it as new patterns emerge.