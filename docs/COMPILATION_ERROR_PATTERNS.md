# Compilation Error Patterns Reference Guide

This document catalogs common compilation error patterns found in Go codebases following Domain-Driven Design (DDD) principles, particularly in the Dependable Call Exchange Backend. It serves as a reference for AI agents and developers to quickly identify and resolve compilation issues.

## Table of Contents

1. [Overview](#overview)
2. [Common Error Categories](#common-error-categories)
3. [Pattern Recognition Guide](#pattern-recognition-guide)
4. [Resolution Strategies](#resolution-strategies)
5. [Prevention Techniques](#prevention-techniques)
6. [Quick Reference Matrix](#quick-reference-matrix)

## Overview

Compilation errors in DDD-based Go projects typically fall into predictable patterns. Understanding these patterns enables faster resolution and helps prevent similar issues in future development.

### Key Principles
- **Always run `go build -gcflags="-e" ./...`** to see ALL compilation errors (not just the first 10)
- **Fix errors systematically** - start with missing types/methods, then imports, then type mismatches
- **Check interface definitions first** - most errors stem from interface-implementation mismatches
- **Verify domain model completeness** - services often expect domain features that don't exist

## Common Error Categories

### 1. Missing Methods on Domain Types

**Pattern**: `undefined (type X has no field or method Y)`

**Common Cases**:
```go
// Error: current.String undefined (type ConsentStatus has no field or method String)
// Solution: Add String() method to the type
func (s ConsentStatus) String() string {
    return string(s)
}
```

**Frequently Missing Methods**:
- `String()` on enum types (Status, Type, etc.)
- `IsValid()` for validation
- `ParseXXX()` for string-to-type conversion
- Getter methods for encapsulated fields

### 2. Interface Method Mismatches

**Pattern**: `does not implement interface: wrong type for method` or `undefined method`

**Common Cases**:
```go
// Repository interface expects:
SaveEvents(ctx context.Context, events []interface{}) error

// But implementation has:
Store(ctx context.Context, events []interface{}) error
```

**Resolution Checklist**:
1. Compare interface definition with implementation
2. Check method names match exactly
3. Verify parameter types and order
4. Ensure return types match
5. Check receiver type (pointer vs value)

### 3. Struct Field Mapping Errors

**Pattern**: `unknown field X in struct literal` or `cannot use X as Y value`

**Common Cases**:
```go
// Error: unknown field Channel in struct literal of type ConsentProof
// Domain struct has:
type ConsentProof struct {
    ID              uuid.UUID
    Type            ProofType
    Metadata        ProofMetadata
}

// Service tries to create:
ConsentProof{
    Channel:   req.Channel,    // Field doesn't exist!
    IPAddress: req.IPAddress,  // Should be in Metadata
}
```

**Resolution Strategy**:
1. Check actual domain struct definition
2. Map fields to correct nested structures
3. Create intermediate structs if needed
4. Use mapper functions for complex conversions

### 4. Type Conversion Issues

**Pattern**: `cannot use X (type Y) as type Z`

**Common Cases**:
```go
// Error: cannot use int64 as map[Type]int
// Service expects:
type ConsentMetrics struct {
    TotalGrants  map[Type]int
}

// But domain provides:
type ConsentMetrics struct {
    TotalGrants  int64
}
```

**Resolution Approaches**:
- Create conversion functions
- Use type assertions carefully
- Consider if types should match exactly
- Add TODO comments for complex conversions

### 5. Missing Imports

**Pattern**: `undefined: package.Type` or `undefined: function`

**Common Missing Imports**:
```go
"fmt"           // For Sprintf, Errorf
"time"          // For time.Time, time.Duration
"context"       // For context.Context
"github.com/google/uuid"  // For uuid.UUID
```

**Quick Fix**:
- Check if the undefined item is from a standard library
- Look for the import in similar files
- Use your IDE's auto-import feature

### 6. Pointer vs Value Confusion

**Pattern**: `invalid operation: cannot indirect X` or `cannot use &X as *Y`

**Common Cases**:
```go
// Error: invalid operation: cannot indirect req.StartDate
dateRange := consent.DateRange{
    Start: *req.StartDate,  // req.StartDate is not a pointer!
}

// Fix:
dateRange := consent.DateRange{
    Start: req.StartDate,   // Use value directly
}
```

### 7. Event Field References

**Pattern**: `event.Field undefined (type Event has no field Field)`

**Common Issue**:
```go
// Service expects:
event.ConsentType
event.OldStatus

// But event only has:
event.ConsentID
event.ConsumerID
```

**Resolution**:
- Check actual event struct definition
- Consider if field should exist or use different approach
- May need to enhance event with missing data

### 8. Missing Domain Constants/Functions

**Pattern**: `undefined: domain.Constant` or `undefined: domain.Function`

**Examples**:
```go
// Service uses:
consent.ProofTypeDigital      // Constant doesn't exist
consent.ParseChannel(string)  // Function doesn't exist
```

**Fix Process**:
1. Determine if constant/function should exist
2. Add to appropriate domain file
3. Follow existing naming patterns
4. Include validation if needed

## Pattern Recognition Guide

### Quick Diagnosis Flow

1. **Run Full Compilation Check**
   ```bash
   go build -gcflags="-e" ./...
   ```

2. **Categorize Errors**
   - Group by package
   - Identify patterns
   - Count occurrences

3. **Priority Order**
   1. Missing types/interfaces
   2. Missing methods
   3. Missing constants/functions
   4. Import issues
   5. Type mismatches
   6. Field mapping errors

### Red Flags to Watch For

- Multiple "undefined" errors in same package → Missing import
- "has no field or method" → Missing implementation
- "cannot use X as Y" → Type mismatch or conversion needed
- Repository method errors → Interface mismatch
- Event-related errors → Event structure mismatch

## Resolution Strategies

### 1. Systematic Approach

```bash
# Step 1: Get all errors
go build -gcflags="-e" ./... 2>&1 | tee build-errors.log

# Step 2: Count error types
grep "undefined:" build-errors.log | wc -l
grep "cannot use" build-errors.log | wc -l
grep "no field or method" build-errors.log | wc -l

# Step 3: Fix by category (in order)
# - Missing types/constants
# - Missing methods
# - Import issues
# - Type conversions
```

### 2. Domain-First Fixing

Always fix domain layer first:
1. Add missing types/constants to domain
2. Implement missing methods on domain objects
3. Then fix service layer usage
4. Finally fix infrastructure/API layers

### 3. Interface Alignment

For repository/service interfaces:
1. Document expected behavior in interface comments
2. Use consistent naming (Save, not Store)
3. Keep parameter order consistent
4. Use clear return types

### 4. Type Conversion Patterns

```go
// Pattern 1: Simple type conversion
stringValue := enumType.String()

// Pattern 2: Map type conversion with helper
func convertToStringMap(m map[string]interface{}) map[string]string {
    result := make(map[string]string, len(m))
    for k, v := range m {
        result[k] = fmt.Sprintf("%v", v)
    }
    return result
}

// Pattern 3: Nil-safe pointer conversion
var emailPtr *string
if email != "" {
    emailPtr = &email
}
```

## Prevention Techniques

### 1. Code Generation

Consider generating repetitive code:
- String() methods for enums
- Parser functions
- Mapper functions
- Test fixtures

### 2. Interface Contracts

```go
// Always define clear contracts
type Repository interface {
    // Save creates or updates an entity
    // Returns ErrNotFound if ID doesn't exist for update
    // Returns ErrConflict if unique constraint violated
    Save(ctx context.Context, entity *Entity) error
}
```

### 3. Consistent Patterns

- Always implement String() on custom types
- Always provide ParseXXX functions for types
- Use value objects consistently
- Follow builder pattern for complex objects

### 4. Compilation Checks in CI

```yaml
# .github/workflows/ci.yml
- name: Compilation Check
  run: |
    go build -gcflags="-e" ./... 2>&1 | tee build.log
    if [ -s build.log ]; then
      echo "Compilation errors found"
      cat build.log
      exit 1
    fi
```

## Quick Reference Matrix

| Error Pattern | Common Cause | Quick Fix | Prevention |
|--------------|--------------|-----------|------------|
| `undefined: Type` | Missing import | Add import | Use goimports |
| `no method String` | Missing String() impl | Add String() method | Code generation |
| `cannot use X as Y` | Type mismatch | Add conversion | Strong typing |
| `unknown field` | Struct mismatch | Check struct def | Use builders |
| `undefined: ParseX` | Missing parser | Add parser func | Generate parsers |
| `cannot indirect` | Pointer confusion | Check ptr vs value | Consistent APIs |
| `event.X undefined` | Event struct mismatch | Check event fields | Event versioning |

## Common Fixes Cookbook

### Fix Missing String() Method
```go
// Add to domain type
func (t TypeName) String() string {
    return string(t)
}
```

### Fix Missing Parser Function
```go
// Add to domain package
func ParseTypeName(s string) (TypeName, error) {
    switch s {
    case "value1":
        return TypeNameValue1, nil
    case "value2":
        return TypeNameValue2, nil
    default:
        return "", errors.NewValidationError("INVALID_TYPE", 
            fmt.Sprintf("invalid type: %s", s))
    }
}
```

### Fix Interface Mismatch
```go
// 1. Find interface definition
// 2. Update implementation to match exactly
// 3. Run tests to verify behavior unchanged
```

### Fix Type Conversion
```go
// For simple conversions
newType := TargetType(sourceValue)

// For complex conversions
func convertType(source SourceType) TargetType {
    // Custom conversion logic
}
```

## Testing Compilation Fixes

### 1. Incremental Testing
```bash
# After each fix, test the specific package
go build ./internal/service/consent/...

# Then test dependent packages
go build ./internal/api/...
```

### 2. Regression Prevention
```go
// Add test to prevent regression
func TestCompilation(t *testing.T) {
    // This will fail to compile if types don't match
    var _ Repository = (*PostgresRepository)(nil)
}
```

## Summary

Most compilation errors follow predictable patterns. By understanding these patterns and applying systematic fixes, you can resolve compilation issues quickly and prevent them from recurring. Always:

1. See all errors with `-gcflags="-e"`
2. Fix domain layer first
3. Align interfaces carefully
4. Use consistent patterns
5. Add tests to prevent regressions

This guide will evolve as new patterns are discovered. When encountering a new pattern, document it here for future reference.