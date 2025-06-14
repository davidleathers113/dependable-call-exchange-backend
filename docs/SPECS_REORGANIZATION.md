# Specifications Directory Reorganization

## Issue Identified
Duplicate and inconsistent organization of specification files across multiple directories:
- `/docs/specs/` - Real production feature specifications
- `/planning/specs/` - Mixed examples, templates, and unrelated UI specs

## Reorganization Performed (June 12, 2025)

### Files Reorganized

#### Moved to Templates Directory:
**From:** `/planning/specs/`  
**To:** `/planning/templates/`

- `example.md` - Basic specification template
- `example-parallel-ready.md` - Parallel execution example

#### Removed (Out of Scope):
- `ui-comparison-test.md` - UI testing spec (doesn't belong in backend project)

#### Remained in Place (Correct Location):
**Location:** `/docs/specs/` ✅

- `03-immutable-audit-logging.md` - Production audit logging specification
- `consent-management-system.md` - Production consent management specification  
- `DNC_LIST_INTEGRATION_SPEC.md` - Production DNC integration specification
- `TCPA_VALIDATION_SPEC.md` - Production TCPA validation specification
- `example-spec.md` - Specification format example

### Final Directory Structure

```
docs/specs/                          # ✅ Real feature specifications
├── 03-immutable-audit-logging.md
├── consent-management-system.md
├── DNC_LIST_INTEGRATION_SPEC.md
├── TCPA_VALIDATION_SPEC.md
└── example-spec.md

planning/templates/                   # ✅ Specification templates
├── example.md
└── example-parallel-ready.md

planning/specs/                       # ❌ REMOVED (was duplicative)
```

## Benefits

1. **Eliminates Duplication**: No more confusing dual specs directories
2. **Clear Purpose**: Templates vs actual specifications are separated
3. **Standard Convention**: `/docs/specs/` follows industry standard for project documentation
4. **Focused Scope**: Removed unrelated UI specs from backend project
5. **Updated References**: Planning documentation now points to correct locations

## Updated References

- `planning/README.md` updated to reference `/docs/specs/` for actual specifications
- Command examples updated to use real specification paths
- Clear distinction between templates and production specifications

This creates a clean, logical organization where:
- **Real specifications** live in the standard `/docs/specs/` location
- **Templates and examples** live in `/planning/templates/`
- **No duplication or confusion** about where to find or create specifications