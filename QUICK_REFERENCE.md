# Quick Reference Card

Essential commands and patterns for daily development on the Dependable Call Exchange Backend.

## 🚀 Most Used Commands

```bash
# Development
make dev-watch              # Hot reload development
make test                   # Run all tests
make ci                     # Pre-commit checks

# Database
go run cmd/migrate/main.go -action up           # Apply migrations
go run cmd/migrate/main.go -action create -name "feature"  # New migration

# Debugging
go build -gcflags="-e" ./...  # Show ALL compilation errors
```

## 📁 Key Directories

```
internal/
├── domain/        # Business entities & logic
├── service/       # Orchestration only
├── infrastructure/# External integrations
└── api/          # HTTP/gRPC/WebSocket handlers

test/
└── testutil/
    └── fixtures/  # Test data builders
```

## 🏗️ Common Patterns

### Domain Constructor
```go
func NewCall(fromNumber, toNumber string) (*Call, error) {
    from, err := values.NewPhoneNumber(fromNumber)
    if err != nil {
        return nil, errors.NewValidationError("INVALID_FROM", 
            "from number must be E.164").WithCause(err)
    }
    // ...
}
```

### Error Handling
```go
// Always wrap errors with context
return errors.NewInternalError("failed to process").WithCause(err)

// Domain-specific errors
return errors.NewComplianceError("TCPA_VIOLATION", "outside calling hours")
```

### Test Fixtures
```go
// Always use builders
call := fixtures.NewCall().
    WithBuyer(buyerID).
    WithStatus(call.StatusRinging).
    Build()
```

## 🧪 Testing Commands

```bash
make test-synctest     # Concurrent tests (Go 1.24)
make test-property     # Property-based testing
make test-race        # Race detection
make coverage         # Coverage report
```

## 🔍 Finding Things

```bash
# Find AIDEV comments
grep -r "AIDEV-" . --include="*.go"

# Find all repositories
find . -name "*repository.go" -type f

# Check imports
go list -f '{{.Imports}}' ./...
```

## 📊 Performance Targets

- Call Routing: < 1ms
- Bid Processing: < 5ms  
- API Response: < 50ms p99
- Compliance Check: < 2ms

## 🛠️ Environment Variables

```bash
DCE_ENVIRONMENT=development
DCE_DATABASE_URL=postgres://localhost:5432/dce_dev
DCE_REDIS_URL=localhost:6379
DCE_LOG_LEVEL=debug
```

## 📚 Key Documentation

- Domain Model: `docs/DOMAIN_MODEL_REFERENCE.md`
- Testing Guide: `TESTING.md`
- AST Analysis: `docs/AST_ANALYSIS.md`
- API Docs: `http://localhost:8080/swagger`

## 🚨 Important Rules

1. **All validation in domain constructors**
2. **Services orchestrate only - no business logic**
3. **Use fixture builders for tests**
4. **Check ALL compilation errors before fixing**
5. **Maximum 5 dependencies per service**

## 💡 Tips

- Run `make ci` before every commit
- Use `make dev-watch` for hot reload
- Check `AIDEV-*` comments for context
- Profile before optimizing
- Write tests first (TDD)