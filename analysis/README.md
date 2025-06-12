# Code Smell Analysis Directory

This directory contains the output from various code smell and quality analysis tools.

## Quick Start

1. **Install required tools:**
   ```bash
   make install-tools
   # or run the script directly:
   ./scripts/install-smell-tools.sh
   ```

2. **Run your first analysis:**
   ```bash
   # Quick check
   make smell-test-quick
   
   # Full analysis with report
   make smell-test-report
   ```

3. **View results:**
   - HTML Report: `analysis/reports/code-smell-report.html`
   - Anti-patterns: `analysis/phase2/antipatterns/`
   - Domain issues: `analysis/phase3/`

## Directory Structure

```
analysis/
├── baseline/           # Baseline reports for incremental analysis
├── ci/                # CI/CD specific analysis outputs
├── phase2/            # General code smell detection
│   └── antipatterns/  # Go anti-pattern detection results
├── phase3/            # Domain-Driven Design analysis
│   └── boundaries/    # Domain boundary violation reports
├── reports/           # Generated HTML and JSON reports
└── README.md          # This file
```

## Running Analysis

### Quick Analysis
```bash
make smell-test-quick
```

### Full Analysis
```bash
make smell-test-full
```

### Generate HTML Report
```bash
make smell-test-report
```

### Check Specific Aspects
```bash
make smell-test-antipatterns  # Detect Go anti-patterns
make smell-test-ddd           # Check DDD smells
make smell-test-boundaries    # Validate domain boundaries
```

### CI/CD Integration
```bash
make smell-test-ci
```

## Understanding Results

### Anti-patterns
- **empty-interfaces.txt**: Usage of `interface{}` (consider using specific types)
- **naked-returns.txt**: Functions with naked returns (harder to understand)
- **init-functions.txt**: Use of init() functions (can cause hidden dependencies)
- **panic-usage.txt**: Direct panic calls (should be rare in production code)
- **global-vars.txt**: Global variable declarations
- **long-params.txt**: Functions with >5 parameters
- **tech-debt-markers.txt**: TODO/FIXME/HACK comments

### DDD Smells
- **Anemic Models**: Domain objects with many fields but few methods
- **Fat Services**: Services with >5 dependencies
- **Domain Leakage**: Domain layer importing infrastructure

### Domain Boundaries
- **{domain}-violations.txt**: Cross-domain import violations for each domain

## Thresholds

Current configured thresholds:
- Cyclomatic Complexity: 15
- Cognitive Complexity: 20
- Max Service Dependencies: 5
- Max Function Length: 100 lines
- Max Function Statements: 50

## Integration with IDE

For VS Code, the golangci-lint extension will automatically use `.golangci.yml` configuration.

For GoLand/IntelliJ, configure golangci-lint in Settings > Tools > File Watchers.
