#!/bin/bash

# Run architecture tests

# Ensure we're in the project root
cd "$(dirname "$0")/.." || exit 1

echo "=== Running Architecture Tests ==="
echo "Working directory: $(pwd)"

# Run the tests
if [ -d "test/architecture" ]; then
    go test -v ./test/architecture/...
else
    echo "Error: Architecture test directory not found"
    exit 1
fi
