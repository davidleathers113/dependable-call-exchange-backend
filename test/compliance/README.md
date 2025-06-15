# Compliance Audit Testing Framework

This directory contains comprehensive compliance validation tests for the Dependable Call Exchange Backend's IMMUTABLE_AUDIT feature.

## Overview

The compliance testing framework validates regulatory adherence across multiple jurisdictions and standards:

- **GDPR** (General Data Protection Regulation) - EU data protection
- **TCPA** (Telephone Consumer Protection Act) - US telemarketing regulations  
- **SOX** (Sarbanes-Oxley Act) - US financial audit requirements
- **CCPA** (California Consumer Privacy Act) - California privacy rights

## Test Architecture

### Core Test Suites

#### `audit_compliance_test.go`
Main test suite containing comprehensive compliance validation:

```go
type ImmutableAuditComplianceTestSuite struct {
    suite.Suite
    db       *testutil.TestDatabase
    ctx      context.Context
    fixtures *fixtures.ComplianceScenarios
}
```

**Test Categories:**
- GDPR compliance validation
- TCPA consent trail verification
- SOX audit trail integrity
- CCPA privacy controls testing
- Retention policy validation
- Cross-regulation compatibility

#### `audit_helpers_test.go`
Test helper utilities and data generators:

```go
type ComplianceAuditTestHelper struct {
    t        *testing.T
    db       *testutil.TestDatabase
    ctx      context.Context
    fixtures *fixtures.ComplianceScenarios
}
```

**Helper Functions:**
- Test data generation for each regulation
- Audit trail creation and validation
- Data quality assessment utilities
- Cross-regulation scenario builders

## Test Execution

### Running All Compliance Tests

```bash
# Run all compliance tests
go test -tags=compliance ./test/compliance/...

# Run with verbose output
go test -tags=compliance -v ./test/compliance/...

# Run specific test suite
go test -tags=compliance -run TestImmutableAuditComplianceTestSuite ./test/compliance/
```

### Running Specific Compliance Areas

```bash
# GDPR tests only
go test -tags=compliance -run TestGDPRComplianceValidation ./test/compliance/

# TCPA tests only
go test -tags=compliance -run TestTCPAComplianceValidation ./test/compliance/

# SOX tests only
go test -tags=compliance -run TestSOXAuditTrailVerification ./test/compliance/

# CCPA tests only  
go test -tags=compliance -run TestCCPAPrivacyControlsTesting ./test/compliance/
```

## Test Scenarios

### GDPR Compliance Tests

#### Data Subject Access Requests
- Complete personal data export
- Data portability validation
- Legal basis documentation
- Processing purpose limitation
- Retention period compliance

#### Right to Erasure (Right to be Forgotten)
- Complete data deletion
- Anonymization verification
- Third-party notification
- Audit trail preservation

#### Cross-Border Data Transfers
- Adequacy decision validation
- Standard contractual clauses
- Binding corporate rules
- Data subject safeguards

### TCPA Compliance Tests

#### Consent Trail Validation
- Written consent documentation
- Electronic signature verification
- Timestamp accuracy
- IP address logging
- Consent scope verification

#### Time Restriction Compliance
- Multi-timezone validation
- Allowed calling hours (8 AM - 9 PM)
- Holiday restrictions
- State-specific rules

#### Do Not Call (DNC) List Management
- National DNC registry checks
- Internal DNC list management
- State DNC compliance
- Real-time validation

### SOX Audit Trail Tests

#### Financial Transaction Integrity
- Immutable audit logging
- Transaction hash verification
- Digital signature validation
- Timestamp accuracy
- Access control verification

#### Internal Controls Testing
- Automated control validation
- Manual control evidence
- Control effectiveness testing
- Deficiency tracking
- Management assertions

#### Audit Trail Retention
- 7-year retention compliance
- Secure storage validation
- Access logging
- Legal hold functionality

### CCPA Privacy Controls Tests

#### Consumer Privacy Rights
- Right to know implementation
- Right to delete processing
- Right to opt-out validation
- Non-discrimination testing

#### Data Inventory Management
- Data category mapping
- Business purpose documentation
- Third-party disclosure tracking
- Retention period management

## Data Structures

### Key Test Data Types

```go
// GDPR Test Subject
type GDPRTestSubject struct {
    SubjectID          uuid.UUID
    PhoneNumber        string
    Nationality        string
    ConsentScope       []string
    ProcessingPurposes []string
    RetentionPeriods   map[string]time.Duration
}

// TCPA Test Scenario
type TCPATestScenario struct {
    PhoneNumber      string
    ConsentMethod    string
    ConsentDate      time.Time
    CallTime         time.Time
    ConsentDocument  TCPAConsentDocument
    DNCStatus        DNCListStatus
}

// SOX Test Transaction
type SOXTestTransaction struct {
    TransactionID    uuid.UUID
    Amount          string
    InternalControls []SOXInternalControlTest
    AuditTrail      []SOXAuditEvent
    DataIntegrity   SOXDataIntegrity
}

// CCPA Test Consumer
type CCPATestConsumer struct {
    ConsumerID            uuid.UUID
    CaliforniaResident    bool
    DataCategories        []CCPADataCategoryTest
    PrivacyRightsRequests []CCPAPrivacyRightRequest
}
```

### Immutable Audit Trail

```go
type ImmutableAuditTrail struct {
    TrailID             uuid.UUID
    AuditChain          []AuditChainLink
    ChainIntegrity      bool
    Immutable           bool
    Encrypted           bool
    ComplianceStandards []string
}

type AuditChainLink struct {
    LinkID       uuid.UUID
    EventData    string
    PreviousHash string
    CurrentHash  string
    BlockNumber  int
    Signature    string
}
```

## Validation Assertions

### GDPR Assertions
- `assertGDPRDataSubjectRights()` - Validates data subject access
- `assertGDPRDataMinimization()` - Checks data minimization principle
- `assertGDPRPurposeLimitation()` - Validates purpose limitation
- `assertDataErasureCompliance()` - Checks right to erasure

### TCPA Assertions
- `assertTCPAConsentTrailIntegrity()` - Validates consent documentation
- `assertTCPATimeComplianceAudit()` - Checks time restrictions
- `assertTCPACallComplianceDecision()` - Validates call approval

### SOX Assertions
- `assertSOXAuditTrailIntegrity()` - Checks financial audit trail
- `assertSOXDataImmutability()` - Validates data immutability
- `assertSOXControlEffectiveness()` - Tests internal controls

### CCPA Assertions
- `assertCCPAPrivacyRightCompliance()` - Validates privacy rights
- `assertCCPAOptOutCompliance()` - Checks opt-out processing
- `assertCCPADataCategoryMapping()` - Validates data inventory

## Cross-Regulation Testing

### Compatibility Scenarios
- EU citizen with US phone number
- California resident traveling in EU
- US company processing EU data
- Multiple jurisdiction data subjects

### Conflict Resolution
- Highest protection standard applied
- Regulation precedence rules
- Compliance matrix validation
- Legal requirement harmonization

## Performance Requirements

### Audit Trail Performance
- Audit log ingestion: < 10ms per event
- Compliance check latency: < 50ms
- Report generation: < 5 seconds
- Data retention enforcement: < 1 hour

### Data Quality Standards
- Data accuracy: > 95%
- Data completeness: > 90%
- Audit trail integrity: 100%
- Immutability verification: 100%

## Integration with DCE System

### Service Dependencies
- `ComplianceService` - Core compliance logic
- `AuditService` - Immutable audit trail management
- `ConsentService` - Consent management
- `RetentionService` - Data retention policies

### Database Schema
- `compliance_rules` - Regulatory rule definitions
- `audit_events` - Immutable event log
- `consent_records` - Consent trail storage
- `data_retention_policies` - Retention configurations

## Regulatory Updates

### Maintaining Compliance
- Regular regulation monitoring
- Test case updates for new requirements
- Compliance matrix adjustments
- Legal review integration

### Documentation Updates
- Regulatory change logs
- Test coverage reports
- Compliance status dashboards
- Audit finding documentation

## Best Practices

### Test Development
1. **Comprehensive Coverage** - Test all regulatory requirements
2. **Real-world Scenarios** - Use realistic test data
3. **Edge Case Testing** - Cover boundary conditions
4. **Performance Validation** - Ensure compliance doesn't impact performance
5. **Documentation** - Maintain clear test documentation

### Compliance Maintenance
1. **Regular Reviews** - Quarterly compliance assessments
2. **Automated Monitoring** - Continuous compliance checking
3. **Audit Preparation** - Maintain audit-ready documentation
4. **Legal Coordination** - Regular legal team consultation
5. **Training Updates** - Keep team updated on regulations

## Troubleshooting

### Common Issues
- **Test Database Setup** - Ensure testcontainers are running
- **Timezone Handling** - Verify timezone conversions
- **Audit Chain Integrity** - Check hash calculations
- **Cross-regulation Conflicts** - Review conflict resolution logic

### Debug Commands
```bash
# Check test database connectivity
go test -tags=compliance -run TestDatabaseSetup ./test/compliance/

# Validate audit chain integrity
go test -tags=compliance -run TestAuditTrailCompleteness ./test/compliance/

# Test specific regulation compliance
go test -tags=compliance -run TestGDPRComplianceValidation ./test/compliance/
```

## Contributing

### Adding New Compliance Tests
1. Identify regulatory requirement
2. Create test scenario in appropriate test suite
3. Add validation assertions
4. Update documentation
5. Verify with legal team

### Test Data Guidelines
- Use realistic but anonymized data
- Follow GDPR principles in test data
- Maintain data minimization
- Document data sources and purposes

## References

- [GDPR Official Text](https://gdpr-info.eu/)
- [TCPA FCC Rules](https://www.fcc.gov/consumers/guides/stop-unwanted-robocalls-and-texts)
- [SOX Requirements](https://www.sec.gov/about/laws/soa2002.pdf)
- [CCPA Official Text](https://oag.ca.gov/privacy/ccpa)
- [DCE Compliance Documentation](../../docs/compliance/)