# Domain Layer Context

## Domain Structure
- `account/` - Buyers & sellers with balance tracking
- `bid/` - Auction mechanics, bid matching
- `call/` - Call lifecycle, status transitions
- `compliance/` - TCPA/GDPR rules engine
- `errors/` - AppError type with retry logic
- `financial/` - Transaction processing

## Domain-Driven Design Principles

### Entity Guidelines
- All validation logic must be in domain constructors
- Use value objects for complex fields (e.g., PhoneNumber, Money)
- Entities should protect their invariants
- State transitions must be explicit methods (e.g., `call.Answer()`, `call.Complete()`)

### Business Logic Placement
- **Call domain**: Status transitions, duration validation, location rules
- **Account domain**: Balance checks, credit limits, quality score calculations
- **Bid domain**: Amount validation, criteria matching, auction rules
- **Compliance domain**: Time restrictions, consent verification, jurisdiction rules

### Working with Enums
- Always implement `String()` method for database serialization
- Use `ParseXXXStatus()` functions for deserialization
- Define `IsValid()` methods for validation
- Example: `CallStatus.String()` returns "pending", "active", etc.

## Error Handling
- Use `AppError` for all domain errors
- Include error codes from defined constants
- Set appropriate HTTP status codes
- Include retry information when applicable

## Testing Domain Logic
- Property-based tests for invariants (1000+ iterations)
- Table-driven tests for state transitions
- Use builders from `testutil/fixtures/` for test data
- Test both valid and invalid scenarios