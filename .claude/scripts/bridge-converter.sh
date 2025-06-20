#!/bin/bash
# Bridge Converter: Convert legacy planning outputs to new context format
# This handles backward compatibility for existing .claude/planning/specs/*.md files

set -euo pipefail

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Default paths
PLANNING_DIR="${PLANNING_DIR:-./.claude/planning}"
CONTEXT_DIR="${CONTEXT_DIR:-./.claude/context}"
SPECS_DIR="$PLANNING_DIR/specs"

# Create context directory if it doesn't exist
mkdir -p "$CONTEXT_DIR"

echo -e "${GREEN}🔗 DCE Bridge Converter - Planning to Context Format${NC}"
echo "Converting legacy planning outputs to new context format..."

# Check if planning directory exists
if [[ ! -d "$PLANNING_DIR" ]]; then
    echo -e "${RED}Error: Planning directory not found at $PLANNING_DIR${NC}"
    exit 1
fi

# Check if specs directory exists
if [[ ! -d "$SPECS_DIR" ]]; then
    echo -e "${RED}Error: Specs directory not found at $SPECS_DIR${NC}"
    exit 1
fi

# Function to extract feature metadata from spec file
extract_feature_metadata() {
    local spec_file="$1"
    local feature_id=""
    local feature_name=""
    local priority=""
    local effort=""
    
    # Extract from markdown headers and content
    feature_name=$(grep -m1 "^# " "$spec_file" | sed 's/^# //' || echo "Unknown Feature")
    
    # Try to extract ID from filename or content
    feature_id=$(basename "$spec_file" .md | sed 's/^[0-9]*-//' | tr '[:lower:]' '[:upper:]' | tr '-' '_')
    
    # Look for priority indicators
    if grep -qi "critical\|compliance" "$spec_file"; then
        priority="Critical"
    elif grep -qi "high\|important" "$spec_file"; then
        priority="High"
    else
        priority="Medium"
    fi
    
    # Look for effort estimates
    if grep -qi "weeks:\s*[0-9]" "$spec_file"; then
        effort=$(grep -i "weeks:\s*[0-9]" "$spec_file" | head -1 | sed 's/.*weeks:\s*\([0-9]*\).*/\1 weeks/')
    else
        effort="TBD"
    fi
    
    echo "id: \"$feature_id\""
    echo "name: \"$feature_name\""
    echo "priority: \"$priority\""
    echo "effort: \"$effort\""
    echo "source_spec: \"$spec_file\""
}

# Generate feature-context.yaml
echo -e "\n${YELLOW}Generating feature-context.yaml...${NC}"
cat > "$CONTEXT_DIR/feature-context.yaml" << 'EOF'
# Auto-generated by DCE Bridge Converter
# Converts .claude/planning/specs/* to implementation context

context_metadata:
  generated_by: "bridge-converter"
  generated_at: "$(date -Iseconds)"
  source: "legacy_planning_outputs"
  
features:
EOF

# Process each spec file
for spec_file in "$SPECS_DIR"/*.md; do
    if [[ -f "$spec_file" ]]; then
        echo -e "  Processing: $(basename "$spec_file")"
        echo "  - $(extract_feature_metadata "$spec_file" | sed 's/^/    /')" >> "$CONTEXT_DIR/feature-context.yaml"
    fi
done

# Generate implementation-plan.md
echo -e "\n${YELLOW}Generating implementation-plan.md...${NC}"
cat > "$CONTEXT_DIR/implementation-plan.md" << 'EOF'
# DCE Implementation Plan

*Auto-generated from planning outputs*

## Overview

This implementation plan was generated by converting the strategic planning outputs into an actionable implementation guide.

## Feature Queue

EOF

# Add features to implementation plan
for spec_file in "$SPECS_DIR"/*.md; do
    if [[ -f "$spec_file" ]]; then
        feature_name=$(grep -m1 "^# " "$spec_file" | sed 's/^# //' || echo "Unknown Feature")
        echo "### $(basename "$spec_file" .md): $feature_name" >> "$CONTEXT_DIR/implementation-plan.md"
        echo "" >> "$CONTEXT_DIR/implementation-plan.md"
        echo "- **Spec**: [$(basename "$spec_file")](../../planning/specs/$(basename "$spec_file"))" >> "$CONTEXT_DIR/implementation-plan.md"
        echo "- **Status**: Ready for implementation" >> "$CONTEXT_DIR/implementation-plan.md"
        echo "" >> "$CONTEXT_DIR/implementation-plan.md"
    fi
done

# Generate execution-queue.yaml
echo -e "\n${YELLOW}Generating execution-queue.yaml...${NC}"
cat > "$CONTEXT_DIR/execution-queue.yaml" << 'EOF'
# DCE Execution Queue
# Auto-generated from planning specs

queue_metadata:
  generated_by: "bridge-converter"
  generated_at: "$(date -Iseconds)"
  total_features: $(ls -1 "$SPECS_DIR"/*.md 2>/dev/null | wc -l)
  
queue_entries:
EOF

# Add queue entries
priority_counter=1
for spec_file in "$SPECS_DIR"/*.md; do
    if [[ -f "$spec_file" ]]; then
        feature_id=$(basename "$spec_file" .md | sed 's/^[0-9]*-//' | tr '[:lower:]' '[:upper:]' | tr '-' '_')
        feature_name=$(grep -m1 "^# " "$spec_file" | sed 's/^# //' || echo "Unknown Feature")
        
        cat >> "$CONTEXT_DIR/execution-queue.yaml" << EOF
  - id: "$feature_id"
    name: "$feature_name"
    priority: $priority_counter
    source_spec: "../../planning/specs/$(basename "$spec_file")"
    ready: true
    dependencies: []
    
EOF
        ((priority_counter++))
    fi
done

echo -e "\n${GREEN}✅ Bridge conversion complete!${NC}"
echo -e "Generated files:"
echo -e "  - $CONTEXT_DIR/feature-context.yaml"
echo -e "  - $CONTEXT_DIR/implementation-plan.md"
echo -e "  - $CONTEXT_DIR/execution-queue.yaml"

# Make the script executable
chmod +x "$0"