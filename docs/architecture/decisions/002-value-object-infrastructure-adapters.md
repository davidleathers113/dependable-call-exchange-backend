# ADR-002: Value Object Infrastructure Adapters

## Status
Accepted

## Context
The domain value objects (`Email`, `Money`, `PhoneNumber`, `QualityMetrics`) in the `internal/domain/values/` package were directly implementing database interfaces (`sql.Scanner` and `driver.Valuer`) from the `database/sql/driver` package. This created a violation of clean architecture principles:

1. **Domain Infrastructure Leakage**: Domain objects had direct dependencies on infrastructure concerns (database drivers)
2. **Coupling**: The domain layer was tightly coupled to specific database implementation details
3. **Testing Complexity**: Domain tests required database driver imports and mocking
4. **Architectural Boundaries**: The separation between domain and infrastructure layers was compromised

### Problems Identified
- Value objects imported `database/sql/driver` package
- Domain layer had knowledge of database storage mechanisms
- Difficulty in changing database implementations without affecting domain objects
- Violation of Dependency Inversion Principle (DIP)

## Decision
Create dedicated infrastructure adapters in `internal/infrastructure/database/adapters/` package that handle the conversion between pure domain value objects and database representations.

### Implementation Details

1. **Create Adapter Layer**: New package `internal/infrastructure/database/adapters/` contains:
   - `EmailAdapter` - handles Email value object database conversion
   - `MoneyAdapter` - handles Money value object database conversion
   - `PhoneAdapter` - handles PhoneNumber value object database conversion
   - `QualityMetricsAdapter` - handles QualityMetrics value object database conversion

2. **Remove Infrastructure Dependencies**: Remove all database-related methods and imports from domain value objects:
   - Remove `Scan(value interface{}) error` methods
   - Remove `Value() (driver.Value, error)` methods
   - Remove `database/sql/driver` imports

3. **Adapter Interface Pattern**: Each adapter implements:
   ```go
   // Scanner interface for reading from database
   Scan(dest *ValueObject, value interface{}) error
   
   // Valuer interface for writing to database
   Value(src ValueObject) (driver.Value, error)
   
   // Nullable variants for optional fields
   ScanNullable(dest **ValueObject, value interface{}) error
   ValueNullable(src *ValueObject) (driver.Value, error)
   ```

4. **Repository Updates**: Update repositories to use adapters instead of calling value object methods directly

5. **Convenience Functions**: Provide package-level convenience functions for common operations

## Consequences

### Positive
- **Clean Architecture**: Domain objects are now pure business logic without infrastructure concerns
- **Better Testability**: Domain tests don't require database driver dependencies
- **Flexibility**: Can easily change database implementation without affecting domain
- **Clear Separation**: Explicit boundary between domain and infrastructure layers
- **Reusability**: Adapters can be reused across different repositories
- **Type Safety**: Adapters provide compile-time type safety for database operations

### Negative
- **Additional Complexity**: New adapter layer adds some architectural complexity
- **More Code**: Additional adapter classes and interfaces to maintain
- **Indirection**: One more layer of indirection when working with database operations

### Migration Strategy
1. Create all adapter classes with comprehensive tests
2. Remove database methods from value objects
3. Update repository implementations to use adapters
4. Update existing tests to use new patterns
5. Add integration tests to verify end-to-end functionality

## Alternatives Considered

### 1. Keep Current Implementation
- **Pros**: Simpler, fewer files, direct integration
- **Cons**: Violates clean architecture, tight coupling, harder to test

### 2. Generic Adapter Pattern
- **Pros**: Single adapter for all value objects
- **Cons**: Loss of type safety, complex implementation, harder to customize per type

### 3. Repository-Level Adapters
- **Pros**: Localized to specific repositories
- **Cons**: Code duplication, inconsistent patterns across repositories

## Implementation Files
- `internal/infrastructure/database/adapters/adapters.go` - Main package interface
- `internal/infrastructure/database/adapters/email_adapter.go` - Email conversion
- `internal/infrastructure/database/adapters/money_adapter.go` - Money conversion
- `internal/infrastructure/database/adapters/phone_adapter.go` - Phone conversion
- `internal/infrastructure/database/adapters/quality_metrics_adapter.go` - QualityMetrics conversion
- `internal/infrastructure/database/adapters/adapters_test.go` - Comprehensive tests

## Usage Example

### Before (Domain Infrastructure Leakage)
```go
// In domain value object
func (e *Email) Scan(value interface{}) error { ... }
func (e Email) Value() (driver.Value, error) { ... }

// In repository
err := row.Scan(&account.Email, ...)
```

### After (Clean Architecture)
```go
// In infrastructure adapter
func (a *EmailAdapter) Scan(dest *values.Email, value interface{}) error { ... }
func (a *EmailAdapter) Value(src values.Email) (driver.Value, error) { ... }

// In repository
emailAdapter := adapters.NewEmailAdapter()
var emailStr string
err := row.Scan(&emailStr, ...)
err = emailAdapter.Scan(&account.Email, emailStr)
```

## References
- Clean Architecture by Robert C. Martin
- Domain-Driven Design by Eric Evans
- Go database/sql package documentation
- [ADR-001: Orchestrator Dependency Exception](001-orchestrator-dependency-exception.md)