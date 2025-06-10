# Semgrep Custom Rules for Dependable Call Exchange Backend

This directory contains custom Semgrep rules tailored for the telephony, compliance, and domain-specific patterns in this codebase.

## Rule Categories

### 1. Telephony Security (`rules/telephony-security.yml`)
- **phone-number-validation**: Ensures phone numbers use the `values.Phone` type
- **tcpa-time-restriction-check**: Verifies TCPA compliance checks before calls
- **pii-logging-prevention**: Prevents logging of unmasked PII data
- **call-recording-consent**: Enforces consent checks for call recording
- **phone-number-storage**: Requires encryption/hashing for stored phone numbers
- **caller-id-spoofing-prevention**: Validates caller ID authorization
- **dnc-list-check**: Enforces Do Not Call list verification
- **call-frequency-limit**: Checks for rate limiting on outbound calls

### 2. Domain Patterns (`rules/domain-patterns.yml`)
- **repository-transaction-handling**: Ensures repositories use transaction context
- **money-arithmetic-safety**: Enforces value object methods for money operations
- **value-object-creation**: Requires factory methods for value objects
- **domain-event-publishing**: Ensures events are published through event bus
- **aggregate-root-persistence**: Warns against persisting non-aggregate entities
- **repository-error-wrapping**: Requires context in error returns
- **bid-amount-validation**: Validates bid amounts before persistence
- **call-status-transition**: Enforces state machine for status changes
- **quality-score-bounds**: Validates quality scores are within 0-100

### 3. Compliance (`rules/compliance.yml`)
- **gdpr-data-retention**: Requires retention periods for personal data
- **gdpr-consent-tracking**: Enforces consent verification
- **gdpr-right-to-erasure**: Ensures deletion methods exist
- **data-anonymization**: Requires anonymization in analytics
- **cross-border-data-transfer**: Validates data transfer compliance
- **consent-expiry-check**: Checks consent validity, not just existence
- **data-portability-format**: Ensures machine-readable export formats
- **audit-log-completeness**: Requires audit logging for data access
- **encryption-at-rest**: Enforces encryption for sensitive fields

### 4. Performance (`rules/performance.yml`)
- **n-plus-one-query**: Detects and suggests batch loading
- **unbounded-query**: Requires LIMIT clauses on queries
- **missing-context-timeout**: Suggests timeouts for external calls
- **inefficient-string-concatenation**: Recommends strings.Builder
- **map-memory-leak**: Warns about unbounded maps
- **goroutine-leak**: Ensures goroutines have exit conditions
- **defer-in-loop**: Warns about defer in loops
- **unnecessary-allocation**: Reduces allocations in hot paths
- **channel-buffer-size**: Suggests buffered channels for high throughput
- **json-marshal-in-loop**: Warns about expensive operations in loops
- **regex-compilation-in-function**: Suggests package-level regex compilation

## Usage

### Run all rules:
```bash
make semgrep
```

### Run specific rule categories:
```bash
make semgrep-security      # Security and compliance rules
make semgrep-domain       # Domain pattern rules
make semgrep-performance  # Performance anti-patterns
```

### CI/CD Integration:
```bash
make semgrep-ci          # JSON output, exits on findings
make semgrep-sarif       # SARIF format for GitHub/GitLab
```

### Development:
```bash
make semgrep-autofix     # Apply automatic fixes
make semgrep-validate    # Validate rule syntax
```

## Configuration

The main configuration is in `.semgrep/config.yml` which:
- Organizes rules by category
- Defines severity levels (ERROR, WARNING, INFO)
- Sets performance parameters
- Configures output formatting

## Severity Levels

- **ERROR**: Must fix before merge (blocks CI)
- **WARNING**: Should fix, reviewed in PR
- **INFO**: Best practice suggestions

## Ignoring False Positives

To ignore specific findings:

1. **Inline suppression**:
   ```go
   // nosemgrep: rule-id
   problematic_code()
   ```

2. **File-level suppression** in `.semgrepignore`

3. **Project-level suppression** in `config.yml`

## Adding New Rules

1. Choose the appropriate category file in `rules/`
2. Add your rule following the existing pattern
3. Include meaningful `message` and `fix` suggestions
4. Test with: `semgrep --config=.semgrep/rules/your-file.yml .`
5. Validate syntax: `make semgrep-validate`

## Integration with CI

The Makefile includes several CI-friendly targets:
- `ci-fast`: Includes `semgrep-ci` for quick feedback
- `ci-security`: Comprehensive security scanning including Semgrep
- Results are output as JSON/SARIF for processing