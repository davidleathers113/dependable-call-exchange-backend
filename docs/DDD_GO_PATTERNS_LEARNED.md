# DDD Go Patterns Learned from Consent Management Implementation

This document captures specific patterns and lessons learned from implementing the consent management domain in a DDD-style Go application. These patterns can be applied to other domains in the codebase.

## Architecture Patterns Observed

### 1. Domain-Service Layer Interface Patterns

**Pattern**: Services expect more from domain objects than initially implemented.

**Common Gaps Found**:
- Domain enums missing String() methods needed for serialization
- Domain objects missing parser functions (ParseType, ParseStatus)
- Domain aggregates missing convenience methods (IsActive, GetCurrentStatus)
- Value objects missing validation helpers

**Lesson**: When creating domain objects, always implement:
```go
// For any custom type
type Status string

// Always add:
func (s Status) String() string { return string(s) }
func (s Status) IsValid() bool { /* validation */ }
func ParseStatus(s string) (Status, error) { /* parsing */ }
```

### 2. Event Patterns

**Pattern**: Events often have fewer fields than services expect.

**What Services Expected**:
```go
event.ConsentType    // Type of consent
event.OldStatus      // Previous status
event.NewStatus      // New status
event.ChangedFields  // What changed
```

**What Events Actually Had**:
```go
event.ConsentID
event.ConsumerID
event.BusinessID
event.Timestamp
```

**Lesson**: Design events with enough context for consumers:
- Include before/after states for update events
- Include entity type/category information
- Consider event versioning from the start

### 3. Repository Interface Evolution

**Pattern**: Repository interfaces evolve differently across layers.

**Domain Layer Expected**:
```go
type Repository interface {
    Save(ctx context.Context, aggregate *Aggregate) error
    GetByID(ctx context.Context, id uuid.UUID) (*Aggregate, error)
}
```

**Service Layer Needed**:
```go
type Repository interface {
    Save(ctx context.Context, aggregate *Aggregate) error
    GetByID(ctx context.Context, id uuid.UUID) (*Aggregate, error)
    GetByConsumerAndType(ctx context.Context, consumerID uuid.UUID, consentType Type) (*Aggregate, error)
    FindActiveByConsumer(ctx context.Context, consumerID uuid.UUID) ([]*Aggregate, error)
}
```

**Lesson**: Start with rich repository interfaces or use Query/Command separation:
```go
type WriteRepository interface {
    Save(ctx context.Context, aggregate *Aggregate) error
    Delete(ctx context.Context, id uuid.UUID) error
}

type ReadRepository interface {
    GetByID(ctx context.Context, id uuid.UUID) (*Aggregate, error)
    // Query methods...
}
```

### 4. DTO-Domain Mapping Complexity

**Pattern**: Direct mapping rarely works due to structural differences.

**Service DTOs**:
```go
type ConsentRequest struct {
    PhoneNumber string
    Email       string
    Channel     string  // Single channel
    Preferences map[string]string
}
```

**Domain Objects**:
```go
type ConsentAggregate struct {
    Versions []ConsentVersion  // Versioned
    // ...
}

type ConsentVersion struct {
    Channels []Channel  // Multiple channels
    // ...
}
```

**Lessons**:
1. Create explicit mapper functions
2. Handle version management in mappers
3. Don't expose domain complexity in DTOs
4. Use builder pattern for complex domain object creation

### 5. Value Object Patterns

**Pattern**: Services often need to work with both raw values and value objects.

**Common Conversions Needed**:
```go
// String to value object
phone, err := values.NewPhoneNumber("+1234567890")

// Value object to string
phoneStr := phone.String()

// Nil-safe conversions
var emailPtr *string
if req.Email != "" {
    emailPtr = &req.Email
}
```

**Lesson**: Provide convenient conversion methods:
```go
// In value object
func (p PhoneNumber) String() string { return string(p) }
func (p PhoneNumber) StringPtr() *string { s := string(p); return &s }

// Helper functions
func StringToPhonePtr(s string) (*PhoneNumber, error) {
    if s == "" {
        return nil, nil
    }
    p, err := NewPhoneNumber(s)
    return &p, err
}
```

### 6. Aggregate Method Patterns

**Pattern**: Aggregates need both command methods and query methods.

**What Was Missing**:
```go
// Services expected these query methods
aggregate.IsActive() bool
aggregate.GetCurrentStatus() Status
aggregate.GetCurrentVersion() *Version
aggregate.HasValidConsent() bool

// But aggregate only had command methods
aggregate.Grant(proof Proof) error
aggregate.Revoke(reason string) error
```

**Lesson**: Design aggregates with both commands and queries:
```go
type Aggregate struct {
    // fields...
}

// Commands (change state)
func (a *Aggregate) Grant(proof Proof) error { }
func (a *Aggregate) Revoke(reason string) error { }

// Queries (read state)
func (a *Aggregate) IsActive() bool { }
func (a *Aggregate) GetCurrentVersion() *Version { }
```

### 7. Error Handling Patterns

**Pattern**: Different layers need different error types.

**Domain Layer**:
```go
errors.NewValidationError("INVALID_PHONE", "invalid format")
errors.NewBusinessRuleError("ALREADY_REVOKED", "consent already revoked")
```

**Service Layer Additions**:
```go
errors.IsNotFound(err)     // For repository errors
errors.IsConflict(err)     // For uniqueness violations
errors.IsValidation(err)   // For input validation
```

**Lesson**: Create a rich error hierarchy with helper functions:
```go
// Base error types in domain/errors
type AppError struct {
    Code    string
    Message string
    Cause   error
    Type    ErrorType
}

// Helper functions
func IsNotFound(err error) bool
func IsValidation(err error) bool
func IsConflict(err error) bool
```

## Compilation Error Patterns Specific to DDD

### 1. Service-Domain Mismatch Pattern

**Symptoms**:
- Service creates domain objects with fields that don't exist
- Service calls methods that aren't implemented
- Service expects different return types

**Root Cause**: Services designed before domain is complete

**Prevention**: 
1. Design domain API first
2. Write service interfaces based on domain capabilities
3. Use TDD - write service tests first

### 2. Event Store Pattern Issues

**Symptoms**:
```go
// Service expects:
eventStore.Store(ctx, events)

// But interface has:
eventStore.SaveEvents(ctx, events)
```

**Root Cause**: Inconsistent naming conventions

**Prevention**: Establish naming conventions early:
- `Save` for persistence operations
- `Get` for single item retrieval  
- `Find` for queries returning multiple items
- `Store` only for cache/temporary storage

### 3. Aggregate vs Entity Confusion

**Pattern**: Services unsure whether to work with aggregates or entities.

**Example**:
```go
// Service might expect:
consent := GetConsent()  // Returns Consent entity

// But repository returns:
aggregate := GetConsent()  // Returns ConsentAggregate
consent := aggregate.GetCurrentVersion()  // Extra step needed
```

**Lesson**: Be explicit about aggregate boundaries:
- Repositories always return aggregates
- Services work with aggregates
- Only expose entities through aggregate methods

## Code Generation Opportunities

Based on patterns observed, these are prime candidates for code generation:

1. **String() methods for all custom types**
2. **Parser functions for enums**
3. **IsValid() methods for validation**
4. **Error type checking functions**
5. **DTO-Domain mappers**
6. **Repository interfaces from aggregate methods**
7. **Event definitions from aggregate commands**

## Testing Patterns

### Compilation Tests
```go
// Ensure interfaces match
var _ domain.Repository = (*infrastructure.PostgresRepository)(nil)
var _ service.Service = (*service.Implementation)(nil)
```

### Builder Pattern for Tests
```go
// Instead of complex setup
consent := fixtures.NewConsentBuilder().
    WithConsumer(consumerID).
    WithType(consent.TypeTCPA).
    WithChannel(consent.ChannelVoice).
    Build()
```

## Summary of Key Learnings

1. **Always implement String() on custom types** - It's always needed eventually
2. **Design rich domain objects** - Services will need more than you think
3. **Create mapper layers** - Direct DTO-domain mapping rarely works
4. **Use explicit interfaces** - Don't rely on implicit interface satisfaction
5. **Plan for evolution** - Events, repositories, and aggregates all grow over time
6. **Establish conventions early** - Naming, error handling, and patterns
7. **Generate boilerplate** - Many patterns are repetitive and can be generated

These patterns repeat across domains. When implementing a new domain (e.g., billing, shipping, inventory), apply these patterns from the start to avoid the same compilation errors.