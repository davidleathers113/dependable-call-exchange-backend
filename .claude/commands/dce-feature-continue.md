# DCE Feature Continue

Continue feature implementation from a specific wave, useful when you need to skip completed waves or jump to a specific implementation phase.

## Usage

```bash
/dce-feature-continue <feature-id> --from-wave=<wave-number> [options]
```

## Options

- `<feature-id>` - The feature ID to continue (required)
- `--from-wave=<wave>` - Wave number to continue from (1-5) (required)
- `--skip-validation` - Skip validation of previous waves
- `--with-context=<file>` - Load additional context from file

## Waves Overview

1. **Wave 1: Domain Layer** - Entities, value objects, domain logic
2. **Wave 2: Infrastructure** - Repositories, database migrations
3. **Wave 3: Service Layer** - Business orchestration
4. **Wave 4: API Layer** - REST/gRPC endpoints
5. **Wave 5: Quality Assurance** - Tests, documentation, benchmarks

## Example Usage

```bash
# Continue from service layer (assuming domain and infra are done)
/dce-feature-continue consent_management --from-wave=3

# Continue from API layer with additional context
/dce-feature-continue consent_management --from-wave=4 --with-context=api-design.yaml

# Skip to quality assurance phase
/dce-feature-continue consent_management --from-wave=5 --skip-validation
```

## Wave Dependencies

Each wave depends on previous waves:
- Wave 2 requires Wave 1 artifacts (domain entities)
- Wave 3 requires Wave 2 artifacts (repositories)
- Wave 4 requires Wave 3 artifacts (services)
- Wave 5 can run partially parallel but needs all previous waves

## Context Preservation

The continue command preserves:
- Previous wave outputs
- Architectural decisions
- Naming conventions
- Code patterns
- Test strategies

## Implementation

The continue command:
1. Validates previous wave completion (unless skipped)
2. Loads context from completed waves
3. Starts implementation from specified wave
4. Updates progress tracking in real-time