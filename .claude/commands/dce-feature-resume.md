# DCE Feature Resume

Resume interrupted feature implementation from where it left off, maintaining context and progress.

## Usage

```bash
/dce-feature-resume <feature-id> [options]
```

## Options

- `<feature-id>` - The feature ID to resume (e.g., "consent_management", "COMPLIANCE-001")
- `--from-wave=<wave>` - Resume from a specific wave (1-5)
- `--force` - Force resume even if feature appears complete
- `--status` - Check current status without resuming

## Execution Flow

1. **Load Progress State**: Read feature progress from `.claude/state/feature-progress.yaml`
2. **Identify Resume Point**: Determine which wave to resume from based on:
   - Last completed wave
   - Partial completion status
   - Any blockers resolved
3. **Restore Context**: Load:
   - Previous wave outputs
   - Codebase insights
   - Dependencies status
   - Quality metrics
4. **Continue Implementation**: Resume from the appropriate wave with full context

## Progress Tracking

The resume command tracks:
- Files created in each wave
- Tests written and coverage
- Compilation status
- Blockers encountered
- Dependencies resolved

## Example Usage

```bash
# Check status of a feature
/dce-feature-resume consent_management --status

# Resume from where it left off
/dce-feature-resume consent_management

# Resume from a specific wave
/dce-feature-resume consent_management --from-wave=3

# Force resume a completed feature
/dce-feature-resume consent_management --force
```

## Output Format

```yaml
resume_status:
  feature_id: "consent_management"
  overall_progress: 60%
  last_completed_wave: 2
  resuming_from_wave: 3
  
  previous_artifacts:
    wave_1:
      - "internal/domain/compliance/consent.go"
      - "internal/domain/compliance/consent_test.go"
    wave_2:
      - "migrations/006_create_consent_table.sql"
      - "internal/infrastructure/repository/consent_repository.go"
      
  blockers_resolved:
    - "database_connection_issue"
    
  next_steps:
    - "Implement ConsentService orchestration"
    - "Add service-level validation"
    - "Create service tests"
```

## Implementation

The resume command:
1. Reads current progress state
2. Identifies the optimal resume point
3. Loads all necessary context
4. Continues the dce-feature implementation from the appropriate wave