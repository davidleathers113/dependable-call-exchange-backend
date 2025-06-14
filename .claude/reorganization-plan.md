# .claude Directory Reorganization Plan

## Current Issues

1. **Empty directories** in `.claude/improve/` that serve no purpose
2. **Duplicate files** with similar content across different locations
3. **Misplaced files** that don't follow the established directory structure
4. **Inconsistent naming** between similar files

## Proposed Actions

### 1. Clean Up Empty Directories
Remove these empty directories from `.claude/improve/`:
```bash
rm -rf .claude/improve/context/
rm -rf .claude/improve/metrics/
rm -rf .claude/improve/monitoring/
rm -rf .claude/improve/specs-implementation/
rm -rf .claude/improve/state/
```

### 2. Consolidate Master Plan Files
- **Keep**: `.claude/planning/master-plan.md` (primary)
- **Keep**: `.claude/commands/dce-master-plan.md` (command definition)
- **Archive**: `.claude/planning/master-plan-compliance-critical.md` → `.claude/planning/archives/`
- **Move**: `.claude/improve/dce-master-plan-enhanced.md` → `.claude/planning/master-plan-enhanced.md`

### 3. Consolidate Execution Queue Files
- **Keep**: `.claude/context/execution-queue.yaml` (current active queue)
- **Archive**: `.claude/improve/execution-queue-enhanced.yaml` → `.claude/context/archives/execution-queue-enhanced.yaml`

### 4. Move Misplaced Files
```bash
# From .claude/improve/ to proper locations:
mv .claude/improve/conflict-resolution-protocol.md .claude/docs/
mv .claude/improve/progress-tracker.yaml .claude/state/
mv .claude/improve/reviews/review-report.md .claude/planning/reports/improvement-review-report.md
mv .claude/improve/execution-log.json .claude/state/
mv .claude/improve/bridge-converter.sh .claude/scripts/
```

### 5. Create Archive Structure
```bash
mkdir -p .claude/archives/planning
mkdir -p .claude/archives/context
mkdir -p .claude/archives/scripts
```

### 6. Final Structure
After reorganization:
```
.claude/
├── commands/           # Command definitions
├── context/           # Active execution context
│   └── archives/      # Historical contexts
├── docs/              # Documentation and guides
├── planning/          # Plans, specs, and reports
│   ├── analysis/      
│   ├── reports/       
│   ├── specs/         
│   ├── templates/     
│   └── archives/      # Historical plans
├── prompts/           # Specialist prompts
├── scripts/           # Utility scripts
├── state/             # Current system state
├── work-discovery/    # Work finding tools
└── archives/          # General archives

# Remove .claude/improve/ entirely after moving files
```

## Execution Script

Create `.claude/scripts/reorganize.sh`:
```bash
#!/bin/bash
# Reorganization script for .claude directory

# Create archive directories
mkdir -p .claude/archives/planning
mkdir -p .claude/archives/context
mkdir -p .claude/scripts

# Move files from improve to proper locations
mv .claude/improve/dce-master-plan-enhanced.md .claude/planning/
mv .claude/improve/conflict-resolution-protocol.md .claude/docs/
mv .claude/improve/progress-tracker.yaml .claude/state/
mv .claude/improve/execution-log.json .claude/state/
mv .claude/improve/bridge-converter.sh .claude/scripts/
mv .claude/improve/reviews/review-report.md .claude/planning/reports/improvement-review-report.md

# Archive duplicate files
mv .claude/planning/master-plan-compliance-critical.md .claude/archives/planning/
cp .claude/improve/execution-queue-enhanced.yaml .claude/archives/context/

# Remove empty directories
rm -rf .claude/improve/context/
rm -rf .claude/improve/metrics/
rm -rf .claude/improve/monitoring/
rm -rf .claude/improve/specs-implementation/
rm -rf .claude/improve/state/

# Remove improve directory if empty
rmdir .claude/improve/reviews/ 2>/dev/null
rmdir .claude/improve/ 2>/dev/null

echo "Reorganization complete!"
```

## Benefits After Reorganization

1. **Cleaner structure** - No empty directories
2. **No duplicates** - Archived older versions
3. **Logical organization** - Files in proper locations
4. **Easier navigation** - Consistent structure
5. **Historical tracking** - Archives preserve old versions