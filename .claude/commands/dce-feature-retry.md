# DCE Feature Retry

Retry a failed wave with improvements, incorporating lessons learned from the failure.

## Usage

```bash
/dce-feature-retry <feature-id> --wave=<wave-number> [options]
```

## Options

- `<feature-id>` - The feature ID to retry (required)
- `--wave=<wave>` - Wave number to retry (1-5) (required)
- `--with-context=<message>` - Additional context about what to fix
- `--clean-artifacts` - Remove failed artifacts before retry
- `--alternative-approach` - Use alternative implementation approach

## Failure Analysis

Before retrying, the command analyzes:
- Compilation errors
- Test failures
- Integration issues
- Performance problems
- Architectural conflicts

## Common Retry Scenarios

### Domain Layer Failures (Wave 1)
```bash
# Retry with better validation logic
/dce-feature-retry consent_management --wave=1 \
  --with-context="Add phone number validation and GDPR compliance checks"
```

### Infrastructure Failures (Wave 2)
```bash
# Retry with performance optimization
/dce-feature-retry consent_management --wave=2 \
  --with-context="Add database indexes and optimize queries"
```

### Service Layer Failures (Wave 3)
```bash
# Retry with proper error handling
/dce-feature-retry consent_management --wave=3 \
  --with-context="Add comprehensive error handling and transaction management"
```

### API Failures (Wave 4)
```bash
# Retry with correct OpenAPI spec
/dce-feature-retry consent_management --wave=4 \
  --with-context="Fix OpenAPI contract validation errors"
```

### Quality Failures (Wave 5)
```bash
# Retry with better test coverage
/dce-feature-retry consent_management --wave=5 \
  --with-context="Increase test coverage to 85% and add integration tests"
```

## Retry Strategy

The retry command:
1. **Analyzes Failure**: Identifies root cause of wave failure
2. **Preserves Good Parts**: Keeps working code from failed attempt
3. **Applies Fixes**: Incorporates context and lessons learned
4. **Validates Changes**: Ensures fixes don't break other waves
5. **Updates Progress**: Records retry attempt and outcome

## Alternative Approaches

When `--alternative-approach` is used:
- Domain: Try different modeling patterns (aggregates vs entities)
- Infrastructure: Switch repository patterns (generic vs specific)
- Service: Change orchestration style (procedural vs event-driven)
- API: Try different API styles (REST vs GraphQL)
- Quality: Different testing strategies (unit vs integration focus)

## Implementation

The retry command:
1. Loads failure analysis from previous attempt
2. Applies corrective strategies based on failure type
3. Re-implements the wave with improvements
4. Validates against previous wave artifacts
5. Updates progress tracking with retry metadata