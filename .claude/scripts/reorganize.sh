#!/bin/bash
# Reorganization script for .claude directory
# Created: 2025-01-16

set -e  # Exit on error

echo "Starting .claude directory reorganization..."

# Create necessary directories
echo "Creating archive directories..."
mkdir -p .claude/archives/planning
mkdir -p .claude/archives/context
mkdir -p .claude/scripts
mkdir -p .claude/state

# Move files from improve to proper locations
echo "Moving misplaced files..."

# Move enhanced master plan
if [ -f ".claude/improve/dce-master-plan-enhanced.md" ]; then
    mv .claude/improve/dce-master-plan-enhanced.md .claude/planning/master-plan-enhanced.md
    echo "  ✓ Moved master-plan-enhanced.md to planning/"
fi

# Move conflict resolution protocol
if [ -f ".claude/improve/conflict-resolution-protocol.md" ]; then
    mv .claude/improve/conflict-resolution-protocol.md .claude/docs/
    echo "  ✓ Moved conflict-resolution-protocol.md to docs/"
fi

# Move progress tracker
if [ -f ".claude/improve/progress-tracker.yaml" ]; then
    mv .claude/improve/progress-tracker.yaml .claude/state/
    echo "  ✓ Moved progress-tracker.yaml to state/"
fi

# Move execution log
if [ -f ".claude/improve/execution-log.json" ]; then
    mv .claude/improve/execution-log.json .claude/state/
    echo "  ✓ Moved execution-log.json to state/"
fi

# Move bridge converter script
if [ -f ".claude/improve/bridge-converter.sh" ]; then
    mv .claude/improve/bridge-converter.sh .claude/scripts/
    echo "  ✓ Moved bridge-converter.sh to scripts/"
fi

# Move review report
if [ -f ".claude/improve/reviews/review-report.md" ]; then
    mv .claude/improve/reviews/review-report.md .claude/planning/reports/improvement-review-report.md
    echo "  ✓ Moved review-report.md to planning/reports/"
fi

# Archive duplicate files
echo "Archiving duplicate files..."

# Archive compliance-critical master plan
if [ -f ".claude/planning/master-plan-compliance-critical.md" ]; then
    mv .claude/planning/master-plan-compliance-critical.md .claude/archives/planning/
    echo "  ✓ Archived master-plan-compliance-critical.md"
fi

# Copy enhanced execution queue to archives (keep original for now)
if [ -f ".claude/improve/execution-queue-enhanced.yaml" ]; then
    cp .claude/improve/execution-queue-enhanced.yaml .claude/archives/context/
    rm .claude/improve/execution-queue-enhanced.yaml
    echo "  ✓ Archived execution-queue-enhanced.yaml"
fi

# Remove empty directories
echo "Removing empty directories..."
for dir in context metrics monitoring specs-implementation state reviews; do
    if [ -d ".claude/improve/$dir" ]; then
        rmdir ".claude/improve/$dir" 2>/dev/null && echo "  ✓ Removed empty improve/$dir/" || true
    fi
done

# Remove improve directory if empty
if [ -d ".claude/improve" ]; then
    if [ -z "$(ls -A .claude/improve)" ]; then
        rmdir .claude/improve
        echo "  ✓ Removed empty improve directory"
    else
        echo "  ⚠ improve/ directory not empty, keeping it"
        ls -la .claude/improve/
    fi
fi

# Make scripts executable
chmod +x .claude/scripts/*.sh 2>/dev/null || true

echo ""
echo "Reorganization complete! Summary:"
echo "--------------------------------"
echo "✓ Created archive directories"
echo "✓ Moved misplaced files to proper locations"
echo "✓ Archived duplicate files"
echo "✓ Removed empty directories"
echo ""
echo "New structure:"
find .claude -type d -name "improve" -prune -o -type d -print | grep -E "^\.claude/[^/]+/?$" | sort