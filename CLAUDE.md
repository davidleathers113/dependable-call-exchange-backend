# CLAUDE.md

This file provides guidance to Claude Code when working with the Dependable Call Exchange Backend codebase.

# Core Documentation
See @README.md for project overview and @go.mod for dependencies and Go version requirements.

# Development Workflow
- Make commands: @Makefile
- Git workflow: @docs/git-instructions.md
- Quick reference: @QUICK_REFERENCE.md

# Testing & Quality
- Testing guide: @TESTING.md
- API best practices: @API_AUDIT_REPORT.md

# Personal Instructions
- @~/.claude/my-project-instructions.md

## Project Overview

A high-performance Pay Per Call exchange platform built with Go 1.24, implementing real-time call routing, intelligent bidding, and comprehensive compliance management.

**Key Performance Requirements:**
- Call routing decisions: < 1ms latency
- Bid processing: 100K bids/second throughput
- API responses: < 50ms p99

## Architecture

**Modular Monolith with DDD** - Five core domains:
- `account/` - Buyers and sellers
- `bid/` - Auctions and bidding  
- `call/` - Call lifecycle
- `compliance/` - TCPA, GDPR, DNC rules
- `financial/` - Transactions and billing

**Service Layer** (`internal/service/`):
- Orchestrates domains and infrastructure only
- No business logic (belongs in domains)
- Maximum 5 dependencies per service

## Essential Commands

See `QUICK_REFERENCE.md` for the most frequently used commands.

```bash
# Development
make dev-watch              # Hot reload development
make ci                     # Run all checks before committing

# Debugging (CRITICAL)
go build -gcflags="-e" ./...  # Show ALL compilation errors (not just first 10)
```

## Key Conventions

### Domain Constructors
All validation in domain constructors, not services:
```go
func NewCall(fromNumber, toNumber string) (*Call, error) {
    // Validate and create value objects
    from, err := values.NewPhoneNumber(fromNumber)
    // ...
}
```

### Error Handling
Use custom `AppError` type:
```go
errors.NewValidationError("INVALID_PHONE", "phone must be E.164 format")
```

### Testing
- Property-based tests: 1000+ iterations for invariants
- Synctest: Deterministic concurrent testing
- Fixtures: Use `testutil/fixtures/` builders

## Important Notes

- **ALWAYS** check compilation errors with `go build -gcflags="-e" ./...` before fixing
- Use fixture builders in `internal/testutil/fixtures/` for test data
- Check context7 MCP server for Go documentation
- Never use sed commands for file modifications

## Related Documentation

- **Quick Reference**: See `QUICK_REFERENCE.md` for daily commands
- **Domain Model**: See `docs/DOMAIN_MODEL_REFERENCE.md`
- **Technical Reference**: See `docs/TECHNICAL_REFERENCE.md`  
- **Testing Guide**: See `TESTING.md`
- **AST Analysis**: See `docs/AST_ANALYSIS.md`
- **API Documentation**: See `docs/api/`

## Subdirectory Context

This project uses nested CLAUDE.md files for area-specific guidance:

- `internal/api/CLAUDE.md` - API implementation patterns
- `internal/domain/CLAUDE.md` - Domain entities and issues
- `internal/infrastructure/CLAUDE.md` - Database patterns
- `internal/service/CLAUDE.md` - Service patterns and anti-patterns
- `test/CLAUDE.md` - Testing issues and patterns
- `cmd/CLAUDE.md` - CLI tools and migration status

## Anchor Comments

Look for these throughout the codebase:
- `AIDEV-NOTE:` - Important implementation details
- `AIDEV-TODO:` - Tasks to complete
- `AIDEV-QUESTION:` - Clarifications needed

Always grep for existing `AIDEV-*` comments before scanning files.

## Current Focus Areas

1. **Contract Testing**: OpenAPI validation with < 1ms overhead
2. **Property-Based Testing**: Using Go 1.24's enhanced testing features
3. **Performance**: Sub-millisecond routing decisions
4. **Compliance**: Real-time TCPA/DNC validation

## Efficiency Principles

- For maximum efficiency, whenever you need to perform multiple independent operations, invoke all relevant tools simultaneously rather than sequentially.

For detailed information, refer to the comprehensive documentation in the `docs/` directory.