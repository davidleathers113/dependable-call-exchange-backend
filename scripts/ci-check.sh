#!/bin/bash

# Exit on any error
set -e

# Ensure we're in the project root
cd "$(dirname "$0")/.." || exit 1

echo "=== Running Code Smell Analysis ==="
echo "Working directory: $(pwd)"

# Set thresholds
MAX_CYCLOMATIC_COMPLEXITY=15
MAX_COGNITIVE_COMPLEXITY=20
MAX_CRITICAL_ISSUES=0
MAX_HIGH_ISSUES=10

# Create output directory
mkdir -p analysis/ci

# Run analysis
golangci-lint run --out-format json > analysis/ci/golangci-analysis.json || true

# Check if jq is installed
if ! command -v jq &> /dev/null; then
    echo "jq is not installed. Please install jq to parse JSON results."
    exit 1
fi

# Check critical issues
CRITICAL_COUNT=$(jq '[.Issues[] | select(.Severity == "error")] | length' analysis/ci/golangci-analysis.json 2>/dev/null || echo "0")
if [ "$CRITICAL_COUNT" -gt "$MAX_CRITICAL_ISSUES" ]; then
    echo "❌ FAILED: Found $CRITICAL_COUNT critical issues (max allowed: $MAX_CRITICAL_ISSUES)"
    exit 1
fi

# Check cyclomatic complexity
if command -v gocyclo &> /dev/null; then
    HIGH_COMPLEXITY=$(gocyclo -over $MAX_CYCLOMATIC_COMPLEXITY . | wc -l)
    if [ "$HIGH_COMPLEXITY" -gt 0 ]; then
        echo "⚠️  WARNING: Found $HIGH_COMPLEXITY functions with cyclomatic complexity > $MAX_CYCLOMATIC_COMPLEXITY"
    fi
else
    echo "⚠️  WARNING: gocyclo not installed, skipping complexity check"
fi

# Run architecture tests
echo "=== Running Architecture Tests ==="
if [ -d "test/architecture" ]; then
    go test ./test/architecture/... -v || {
        echo "❌ FAILED: Architecture tests failed"
        exit 1
    }
else
    echo "⚠️  WARNING: Architecture tests not found"
fi

echo "✅ Code smell analysis passed!"
