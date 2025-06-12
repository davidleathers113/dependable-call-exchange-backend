#!/bin/bash

# Ensure we're in the project root
cd "$(dirname "$0")/.." || exit 1

OUTPUT_DIR="analysis/phase3/boundaries"
mkdir -p $OUTPUT_DIR

echo "=== Checking Domain Boundaries ==="
echo "Working directory: $(pwd)"

# Check for direct cross-domain imports
for domain in account bid call compliance financial; do
    echo "Checking $domain domain..."
    
    # Check if domain directory exists
    if [ -d "internal/domain/$domain" ]; then
        # Find imports from other domains
        find internal/domain/$domain -name "*.go" -exec grep -H "internal/domain/" {} \; | \
            grep -v "internal/domain/$domain" | \
            grep -v "internal/domain/values" > $OUTPUT_DIR/${domain}-violations.txt
    else
        echo "  Warning: Domain directory internal/domain/$domain not found"
    fi
done

# Check service layer dependencies
echo "Checking service layer dependencies..."
find internal/service -name "*.go" -exec grep -l "type.*Service.*struct" {} \; | while read file; do
    echo "$file:"
    grep -A 10 "type.*Service.*struct" "$file" | grep -E "Repository|Service|Client|Cache|Bus" || true
done > $OUTPUT_DIR/service-dependencies.txt

echo "Domain boundary check complete!"
