# AST Analysis with Tree-Sitter

This guide covers using Tree-Sitter for advanced code analysis and architecture validation in the Dependable Call Exchange Backend.

## Setup

```bash
# Initialize tree-sitter for the project
mcp__tree_sitter__register_project_tool --path . --name "DCE"

# Verify setup
mcp__tree_sitter__list_projects_tool
```

## Architecture Enforcement

### Domain Boundary Validation

```bash
# Find cross-layer imports (violations of DDD boundaries)
mcp__tree_sitter__run_query --project "DCE" --query '(import_spec path: (string) @import (#match? @import "internal/(domain|service|infrastructure)"))'

# Verify DDD boundaries - domain shouldn't import from infrastructure
mcp__tree_sitter__run_query --project "DCE" --query '(import_spec path: (string) @path (#match? @path "internal/domain") (#match? @file "internal/infrastructure"))'

# Find service layer violations (business logic in services)
mcp__tree_sitter__run_query --project "DCE" --query '(method_declaration receiver: (parameter_list (parameter type: (pointer_type (identifier) @type (#match? @type "Service$")))) body: (block (if_statement condition: (binary_expression left: (selector_expression field: (identifier) @field (#match? @field "Status|State|Valid")))))'

# Check for proper error domain usage
mcp__tree_sitter__run_query --project "DCE" --query '(import_spec path: (string) @path (#match? @path "errors") (#not-match? @path "internal/domain/errors"))'

# Find direct database access outside repositories
mcp__tree_sitter__run_query --project "DCE" --query '(import_spec path: (string) @path (#match? @path "database/sql|pgx") (#not-match? @file "repository|database"))'
```

### Repository Pattern Validation

```bash
# Find all repository interfaces
mcp__tree_sitter__run_query --project "DCE" --query '(interface_type name: (identifier) @name (#match? @name "Repository$"))'

# Detect repository pattern violations
mcp__tree_sitter__run_query --project "DCE" --query '(method_declaration receiver: (parameter_list (parameter type: (pointer_type (identifier) @type (#match? @type "Repository$")))) name: (identifier) @method (#not-match? @method "^(Find|Get|List|Create|Update|Delete)"))'

# Find repository implementations
mcp__tree_sitter__get_symbols --project "DCE" --file_path "internal/infrastructure/database/" --symbol_types ["types"]
```

## Code Quality Analysis

### Error Handling Patterns

```bash
# Find missing error wrapping
mcp__tree_sitter__run_query --project "DCE" --query '(return_statement (expression_list (identifier) @err (#eq? @err "err")) (#not-match? @parent "fmt.Errorf"))'

# Find ignored errors
mcp__tree_sitter__run_query --project "DCE" --query '(assignment_statement left: (identifier) @underscore (#eq? @underscore "_") right: (call_expression))'

# Find error handling in loops (potential issues)
mcp__tree_sitter__run_query --project "DCE" --query '(for_statement body: (block (if_statement condition: (binary_expression left: (identifier) @err (#eq? @err "err") operator: "!="))))'
```

### Performance Analysis

```bash
# Find all database queries for optimization
mcp__tree_sitter__run_query --project "DCE" --query '(call_expression function: (selector_expression field: (identifier) @method (#match? @method "^(Query|Exec|Get|Select)"))'

# Detect potential N+1 queries
mcp__tree_sitter__find_similar_code --project "DCE" --snippet "for _, _ := range" --context_lines 10 | grep -A5 -B5 "Query\|Get"

# Find all goroutine spawns for concurrency review
mcp__tree_sitter__run_query --project "DCE" --query '(go_statement)'

# Find all mutex locks for concurrency bottlenecks
mcp__tree_sitter__run_query --project "DCE" --query '(call_expression function: (selector_expression field: (identifier) @method (#match? @method "^(Lock|RLock)"))'

# Detect large struct copies (potential performance issues)
mcp__tree_sitter__run_query --project "DCE" --query '(assignment_statement left: (identifier) right: (identifier) @struct (#match? @struct "^[A-Z]"))'

# Analyze function complexity for performance hotspots
mcp__tree_sitter__analyze_complexity --project "DCE" --file_path "internal/service/callrouting/algorithms.go"
```

## Test Analysis

```bash
# Find all public functions without tests
mcp__tree_sitter__get_symbols --project "DCE" --file_path "internal/service/callrouting/service.go" --symbol_types ["functions"]
# Then check if corresponding test exists

# Find all test tables for consistency
mcp__tree_sitter__run_query --project "DCE" --query '(composite_literal type: (slice_type) @type (#match? @type "struct") (#match? @parent "tests.*:="))'

# Detect missing property tests
mcp__tree_sitter__list_files --project "DCE" --pattern "**/*_test.go" | grep -v "_property_test.go"

# Find untested error paths
mcp__tree_sitter__run_query --project "DCE" --query '(if_statement condition: (binary_expression left: (identifier) @err (#eq? @err "err") operator: "!="))'

# Analyze test complexity vs source complexity
mcp__tree_sitter__analyze_complexity --project "DCE" --file_path "internal/service/bidding/auction.go"
```

## Refactoring Support

### Value Object Migration

```bash
# Find all primitive money values to convert
mcp__tree_sitter__run_query --project "DCE" --query '(field name: (identifier) @field (#match? @field "(Amount|Price|Cost)") type: (identifier) @type (#match? @type "float64"))'

# Find all struct literals that need value object conversion
mcp__tree_sitter__find_similar_code --project "DCE" --snippet "Amount: " --threshold 0.7

# Find all places using primitive phone/email strings
mcp__tree_sitter__run_query --project "DCE" --query '(field name: (identifier) @field (#match? @field "(Phone|Email|Contact)") type: (identifier) @type (#eq? @type "string"))'

# Detect value object instantiation patterns for consistency
mcp__tree_sitter__find_similar_code --project "DCE" --snippet "values.MustNew" --threshold 0.8

# Analyze usage before refactoring
mcp__tree_sitter__find_usage --project "DCE" --symbol "QualityScore"
```

### Systematic Refactoring Process

1. **Find all occurrences**
   ```bash
   mcp__tree_sitter__find_usage --project "DCE" --symbol "TargetSymbol"
   ```

2. **Analyze dependencies**
   ```bash
   mcp__tree_sitter__get_dependencies --project "DCE" --file_path "path/to/file.go"
   ```

3. **Apply changes systematically**
   ```bash
   # Use tree-sitter queries to find specific patterns
   ```

4. **Verify changes**
   ```bash
   mcp__tree_sitter__find_usage --project "DCE" --symbol "NewSymbol"
   ```

## Project Analysis

### High-Level Structure

```bash
# Analyze overall project structure
mcp__tree_sitter__analyze_project --project "DCE" --scan_depth 3

# Find all AIDEV anchor comments
mcp__tree_sitter__find_text --project "DCE" --pattern "AIDEV-(NOTE|TODO|QUESTION):" --use_regex true

# Extract all TODO comments with context
mcp__tree_sitter__find_text --project "DCE" --pattern "TODO" --context_lines 2
```

### Dependency Analysis

```bash
# Find circular dependencies
mcp__tree_sitter__analyze_project --project "DCE" --scan_depth 5

# List all external dependencies
mcp__tree_sitter__run_query --project "DCE" --query '(import_spec path: (string) @import (#not-match? @import "^internal/"))'

# Find unused imports
mcp__tree_sitter__run_query --project "DCE" --query '(import_spec path: (string) @import alias: (identifier) @alias (#eq? @alias "_"))'
```

## Custom Queries

### Template for Custom Queries

```bash
# Basic structure matching
mcp__tree_sitter__run_query --project "DCE" --query '(PATTERN_TYPE field: (identifier) @capture)'

# With conditions
mcp__tree_sitter__run_query --project "DCE" --query '(PATTERN condition: (expression) @cond (#match? @cond "pattern"))'

# Multiple captures
mcp__tree_sitter__run_query --project "DCE" --query '(function_declaration name: (identifier) @name parameters: (parameter_list) @params)'
```

### Common Patterns

- `#match?` - Regex match
- `#eq?` - Exact match
- `#not-match?` - Negative regex match
- `@capture` - Capture node for output

## Integration with Development Workflow

### Pre-commit Checks

```bash
# Check for architecture violations
./scripts/check-architecture.sh

# Find code smells
make analyze-smells

# Extract TODOs
make find-todos
```

### CI/CD Integration

Add these checks to your CI pipeline:

```yaml
- name: Architecture Validation
  run: |
    mcp__tree_sitter__register_project_tool --path . --name "DCE"
    ./scripts/validate-ddd-boundaries.sh
```

## Best Practices

1. **Regular Analysis** - Run architecture checks weekly
2. **Before Refactoring** - Always analyze usage patterns
3. **Document Patterns** - Save useful queries in scripts
4. **Combine Tools** - Use with grep, awk for complex analysis
5. **Version Control** - Track query scripts in git

## Troubleshooting

### Common Issues

**Query Returns No Results**
- Check file patterns match your structure
- Verify syntax node names are correct
- Use simpler queries to debug

**Performance Issues**
- Reduce scan depth for large projects
- Use file patterns to limit scope
- Run queries on specific directories

**Parse Errors**
- Ensure Go files are syntactically valid
- Check for unsupported Go 1.24 syntax
- Update tree-sitter grammar if needed

For more advanced patterns, refer to:
- Tree-Sitter documentation
- Go grammar specification
- Project-specific query examples in `scripts/ast-queries/`