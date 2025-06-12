#!/bin/bash

# Validate code smell testing setup

echo "=== Validating Code Smell Testing Setup ==="
echo

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check function
check() {
    if [ $1 -eq 0 ]; then
        echo -e "${GREEN}✓${NC} $2"
    else
        echo -e "${RED}✗${NC} $2"
        return 1
    fi
}

warn() {
    echo -e "${YELLOW}⚠${NC} $1"
}

errors=0

# Check configuration files
echo "Checking configuration files..."
[ -f ".golangci.yml" ] && check 0 ".golangci.yml exists" || { check 1 ".golangci.yml missing"; ((errors++)); }
[ -f ".pre-commit-config.yaml" ] && check 0 ".pre-commit-config.yaml exists" || { check 1 ".pre-commit-config.yaml missing"; ((errors++)); }

# Check scripts
echo
echo "Checking scripts..."
scripts=(
    "scripts/detect-antipatterns.sh"
    "scripts/detect-ddd-smells/detect-ddd-smells.go"
    "scripts/check-domain-boundaries.sh"
    "scripts/ci-check.sh"
    "scripts/generate-report/generate-report.go"
    "scripts/install-smell-tools.sh"
)

for script in "${scripts[@]}"; do
    if [ -f "$script" ]; then
        check 0 "$script exists"
        if [[ "$script" == *.sh ]]; then
            [ -x "$script" ] && check 0 "$script is executable" || warn "$script is not executable (run: chmod +x $script)"
        fi
    else
        check 1 "$script missing"
        ((errors++))
    fi
done

# Check test files
echo
echo "Checking test files..."
[ -f "test/architecture/architecture_test.go" ] && check 0 "Architecture tests exist" || { check 1 "Architecture tests missing"; ((errors++)); }

# Check analysis directory
echo
echo "Checking analysis directory..."
[ -d "analysis" ] && check 0 "analysis/ directory exists" || { check 1 "analysis/ directory missing"; ((errors++)); }

# Check required tools
echo
echo "Checking required tools..."
command -v golangci-lint >/dev/null 2>&1 && check 0 "golangci-lint installed" || warn "golangci-lint not installed"
command -v gocyclo >/dev/null 2>&1 && check 0 "gocyclo installed" || warn "gocyclo not installed"
command -v gocognit >/dev/null 2>&1 && check 0 "gocognit installed" || warn "gocognit not installed"
command -v jq >/dev/null 2>&1 && check 0 "jq installed (for CI)" || warn "jq not installed (needed for CI scripts)"

# Check Makefile targets
echo
echo "Checking Makefile targets..."
make_targets=(
    "smell-test"
    "smell-test-quick"
    "smell-test-full"
    "smell-test-report"
)

for target in "${make_targets[@]}"; do
    if make -n $target >/dev/null 2>&1; then
        check 0 "make $target available"
    else
        check 1 "make $target missing"
        ((errors++))
    fi
done

# Check project structure
echo
echo "Checking project structure..."
[ -d "internal/domain" ] && check 0 "internal/domain/ exists" || warn "internal/domain/ missing"
[ -d "internal/service" ] && check 0 "internal/service/ exists" || warn "internal/service/ missing"

# Summary
echo
echo "=== Summary ==="
if [ $errors -eq 0 ]; then
    echo -e "${GREEN}All critical components are in place!${NC}"
    echo
    echo "Next steps:"
    echo "1. Install any missing tools: make install-tools"
    echo "2. Run initial analysis: make smell-test-baseline"
    echo "3. Generate report: make smell-test-report"
else
    echo -e "${RED}Found $errors critical issues that need to be fixed.${NC}"
fi

# Show warnings about optional tools
echo
if ! command -v gocyclo >/dev/null 2>&1 || ! command -v gocognit >/dev/null 2>&1; then
    echo "To install missing analysis tools, run:"
    echo "  make install-tools"
    echo "  # or"
    echo "  ./scripts/install-smell-tools.sh"
fi
