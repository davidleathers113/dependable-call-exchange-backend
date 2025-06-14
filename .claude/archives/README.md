# DCE Archives

This directory contains archived versions of important planning and configuration files that are no longer actively used but may be valuable for historical reference.

## Archive Structure

```
archives/
├── context/           # Archived context files (execution queues, state snapshots)
├── planning/          # Archived planning documents (master plans, roadmaps)
└── scripts/           # Archived automation scripts (if any)
```

## Naming Convention

All archived files include a timestamp suffix in the format: `-YYYYMMDD`

Example: `master-plan-compliance-critical-20250115.md`

## Current Archives

### Context Files
- `execution-queue-enhanced-20250112.yaml` - Enhanced execution queue from parallel implementation

### Planning Files  
- `master-plan-compliance-critical-20250115.md` - Critical compliance master plan

## Archival Policy

Files are moved here when:
1. They are superseded by newer versions
2. The feature/plan they describe is completed
3. They are no longer relevant to current development
4. During major reorganizations (like moving from improve/ to optimization/)

## Accessing Archives

These files are read-only references. If you need to resurrect an archived approach:
1. Copy the file to the appropriate active directory
2. Remove the timestamp suffix
3. Update any outdated references
4. Document why it was restored in git commit