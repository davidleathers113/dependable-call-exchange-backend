# AI Agent Workflow for Fixing Compilation Errors

This document provides a systematic workflow for AI agents to efficiently diagnose and fix compilation errors in Go codebases.

## Executive Summary

When tasked with fixing compilation errors, follow this priority-ordered workflow:

1. **Discover** - Run `go build -gcflags="-e" ./...` to see ALL errors
2. **Categorize** - Group errors by type and package
3. **Prioritize** - Fix in order: types → methods → imports → conversions
4. **Implement** - Apply fixes systematically
5. **Verify** - Re-compile after each category of fixes

## Detailed Workflow

### Phase 1: Discovery and Analysis

```bash
# ALWAYS start with this command to see ALL errors
go build -gcflags="-e" ./... 2>&1 | tee compilation-errors.log

# Count error categories
grep -c "undefined:" compilation-errors.log
grep -c "no field or method" compilation-errors.log
grep -c "cannot use" compilation-errors.log
grep -c "unknown field" compilation-errors.log
```

**Key Insight**: The default `go build` only shows first 10 errors. Using `-gcflags="-e"` reveals all compilation errors, which is critical for understanding the full scope.

### Phase 2: Error Categorization

Group errors into these categories (fix in this order):

1. **Missing Types/Constants**
   - Pattern: `undefined: package.TypeName`
   - Pattern: `undefined: ConstantName`
   
2. **Missing Methods**
   - Pattern: `has no field or method MethodName`
   - Pattern: `.String undefined`

3. **Missing Functions**
   - Pattern: `undefined: ParseXXX`
   - Pattern: `undefined: NewXXX`

4. **Import Issues**
   - Pattern: `undefined: fmt`
   - Pattern: Multiple undefined in same file

5. **Type Mismatches**
   - Pattern: `cannot use X as Y`
   - Pattern: `invalid operation`

6. **Struct Field Issues**
   - Pattern: `unknown field X in struct literal`
   - Pattern: `cannot use field X`

### Phase 3: Systematic Resolution

#### Step 1: Fix Missing Types and Constants

```go
// If error: undefined: consent.ProofTypeDigital
// Add to domain/consent/consent.go:
const (
    ProofTypeDigital ProofType = "digital"
)

// If error: undefined: consent.ParseChannel
// Add parser function:
func ParseChannel(s string) (Channel, error) {
    switch s {
    case "voice":
        return ChannelVoice, nil
    // ... other cases
    default:
        return "", errors.NewValidationError("INVALID_CHANNEL", 
            fmt.Sprintf("invalid channel: %s", s))
    }
}
```

#### Step 2: Fix Missing Methods

```go
// If error: type ConsentStatus has no method String
// Add to the type definition:
func (s ConsentStatus) String() string {
    return string(s)
}
```

#### Step 3: Fix Import Issues

```go
// Check the specific undefined reference
// Common missing imports:
import (
    "fmt"        // For Sprintf, Errorf
    "time"       // For time.Time
    "context"    // For context.Context
    "github.com/google/uuid"  // For uuid.UUID
)
```

#### Step 4: Fix Type Conversions

```go
// Pattern 1: Simple conversion
// Error: cannot use stringValue as TypeName
typedValue := TypeName(stringValue)

// Pattern 2: Map conversion
// Error: cannot use map[string]interface{} as map[string]string
func convertMap(m map[string]interface{}) map[string]string {
    result := make(map[string]string)
    for k, v := range m {
        result[k] = fmt.Sprintf("%v", v)
    }
    return result
}

// Pattern 3: Pointer handling
// Error: cannot use value as *Type
var ptr *Type
if condition {
    temp := value
    ptr = &temp
}
```

### Phase 4: Verification

After each category of fixes:

```bash
# Re-run compilation to see remaining errors
go build -gcflags="-e" ./...

# Check specific package if working on focused area
go build -gcflags="-e" ./internal/service/consent/...
```

## Common Patterns Quick Reference

### Pattern: Interface Method Mismatch

**Symptom**: Repository implementation doesn't match interface

**Diagnosis**:
1. Find interface definition: `grep -n "interface {" *.go`
2. Find implementation: `grep -n "func.*RepoImpl" *.go`
3. Compare method signatures

**Fix**:
```go
// Interface expects:
Save(ctx context.Context, entity *Entity) error

// Implementation has:
Store(ctx context.Context, entity *Entity) error

// Fix: Rename Store to Save in implementation
```

### Pattern: Event Field Mismatch

**Symptom**: `event.FieldName undefined`

**Diagnosis**:
1. Check event struct definition
2. Determine if field should exist
3. Check if accessing wrong event type

**Fix Options**:
1. Add field to event (if it should exist)
2. Remove reference (if field not needed)
3. Use different approach to get data

### Pattern: Domain Object Method Missing

**Symptom**: Domain object missing expected method

**Fix Template**:
```go
// For status types
func (s StatusType) String() string {
    return string(s)
}

// For validation
func (t Type) IsValid() bool {
    switch t {
    case Type1, Type2, Type3:
        return true
    default:
        return false
    }
}

// For parsing
func ParseType(s string) (Type, error) {
    // Implementation
}
```

## Efficiency Tips

### 1. Batch Similar Fixes

Instead of fixing one error at a time:
- Fix ALL missing String() methods together
- Add ALL missing imports in one edit
- Create ALL missing constants together

### 2. Use Search Patterns

```bash
# Find all String() method implementations
grep -n "func.*String() string" internal/domain/**/*.go

# Find all type definitions
grep -n "^type.*string$" internal/domain/**/*.go

# Find interface definitions
grep -n "type.*interface {" internal/**/*.go
```

### 3. Understand Dependencies

Fix errors in dependency order:
1. Domain layer first
2. Service layer second  
3. Infrastructure layer third
4. API layer last

### 4. Create Helper Functions

When you see repetitive conversions, create helpers:

```go
// Instead of repeating this pattern
if value != nil {
    field = *value
} else {
    field = defaultValue
}

// Create
func derefOrDefault[T any](ptr *T, defaultVal T) T {
    if ptr != nil {
        return *ptr
    }
    return defaultVal
}
```

## AI-Specific Strategies

### 1. Context Window Management

- Focus on one package at a time
- Read only relevant files
- Use grep to find specific patterns instead of reading entire files

### 2. Efficient File Reading

```bash
# Instead of reading entire file
Read(file_path)

# Use grep first to find relevant sections
grep -n "type.*struct" file.go
grep -n "func.*Save" file.go

# Then read specific line ranges
Read(file_path, offset=100, limit=50)
```

### 3. Parallel Investigation

When investigating related errors:
- Open interface definition
- Open implementation
- Compare side-by-side
- Fix mismatches

### 4. Documentation Trail

As you fix errors, note patterns:
- Which methods are commonly missing?
- Which imports are frequently forgotten?
- Which type conversions recur?

Update this guide with new patterns discovered.

## Completion Checklist

- [ ] All compilation errors resolved
- [ ] No new errors introduced
- [ ] Code follows existing patterns
- [ ] Tests still pass (if any exist)
- [ ] Document any new patterns discovered

## Red Flags to Escalate

If you encounter these, consider asking for clarification:

1. **Massive Interface Changes** - If an interface has 10+ method mismatches
2. **Missing Core Types** - If fundamental domain types don't exist
3. **Circular Dependencies** - If fixes would create import cycles
4. **Breaking Changes** - If fixes would break existing functionality

## Summary

The key to efficiently fixing compilation errors is:
1. See ALL errors with `-gcflags="-e"`
2. Fix systematically by category
3. Start with domain layer
4. Batch similar fixes
5. Verify after each category

This systematic approach prevents wasted effort and ensures all related issues are addressed together.