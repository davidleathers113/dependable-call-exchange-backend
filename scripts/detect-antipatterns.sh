#!/bin/bash

# Ensure we're in the project root
cd "$(dirname "$0")/.." || exit 1

OUTPUT_DIR="analysis/phase2/antipatterns"
mkdir -p $OUTPUT_DIR

echo "=== Detecting Go Anti-Patterns ==="
echo "Working directory: $(pwd)"

# 1. Empty Interface Usage (potential design smell)
echo "Checking for empty interface{} usage..."
grep -r "interface{}" --include="*.go" --exclude-dir=vendor . | \
  grep -v "_test.go" | \
  grep -v "// " > $OUTPUT_DIR/empty-interfaces.txt

# 2. Naked Returns (harder to understand)
echo "Checking for naked returns..."
grep -r "return$" --include="*.go" --exclude-dir=vendor . | \
  grep -v "_test.go" > $OUTPUT_DIR/naked-returns.txt

# 3. Init Functions (can cause hidden dependencies)
echo "Checking for init functions..."
grep -r "func init()" --include="*.go" --exclude-dir=vendor . > $OUTPUT_DIR/init-functions.txt

# 4. Panic Usage (should be rare)
echo "Checking for panic calls..."
grep -r "panic(" --include="*.go" --exclude-dir=vendor . | \
  grep -v "_test.go" | \
  grep -v "// " > $OUTPUT_DIR/panic-usage.txt

# 5. Global Variables
echo "Checking for global variables..."
grep -r "^var " --include="*.go" --exclude-dir=vendor . | \
  grep -v "_test.go" | \
  grep -v "const " > $OUTPUT_DIR/global-vars.txt

# 6. Long Parameter Lists (>5 parameters)
echo "Checking for long parameter lists..."
awk '/func.*\(.*,.*,.*,.*,.*,/ {print FILENAME":"FNR":"$0}' \
  $(find . -name "*.go" -not -path "./vendor/*") > $OUTPUT_DIR/long-params.txt

# 7. TODO/FIXME/HACK Comments
echo "Checking for technical debt markers..."
grep -r "TODO\|FIXME\|HACK\|XXX" --include="*.go" --exclude-dir=vendor . > $OUTPUT_DIR/tech-debt-markers.txt

echo "Anti-pattern detection complete!"
