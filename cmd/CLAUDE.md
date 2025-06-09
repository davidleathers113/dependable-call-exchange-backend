# CLI Tools Context

## Directory Structure
- `cli/` - Main CLI interface (planned)
- `migrate/` - Database migration tool (implemented)
- `worker/` - Background job processors (planned)

## Migration Tool Commands
```bash
# Apply all pending migrations
go run cmd/migrate/main.go -action up

# Rollback last migration
go run cmd/migrate/main.go -action down -steps 1

# Check migration status
go run cmd/migrate/main.go -action status

# Create new migration file
go run cmd/migrate/main.go -action create -name "add_feature"
```

## When Implementing New CLI Tools
- Use cobra for command structure
- Follow existing patterns in `migrate/main.go`
- Include comprehensive help text
- Handle errors with proper exit codes
- Use the shared config package for consistency