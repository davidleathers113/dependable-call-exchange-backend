# Compliance Platform Specification

## Executive Summary

### Problem Statement
Current compliance implementation stands at only 40%, creating critical legal and operational risks for the Dependable Call Exchange platform. The existing implementation has:
- Basic TCPA structures without orchestration
- No GDPR implementation beyond entity definitions
- Missing DNC list integration
- No audit trail system
- Incomplete consent management

### Business Risks
- **Legal Exposure**: Non-compliance with TCPA can result in fines up to $1,500 per violation
- **GDPR Penalties**: Up to 4% of annual revenue or €20 million (whichever is higher)
- **Operational Shutdown**: Regulatory bodies can cease operations for non-compliance
- **Reputation Damage**: Non-compliance incidents severely impact trust
- **Partner Risk**: Buyers/sellers may face liability for platform violations

### Solution Overview
Implement a comprehensive compliance platform that:
- Provides real-time compliance validation (< 2ms)
- Manages consent lifecycle with full audit trail
- Integrates with federal and state DNC lists
- Implements GDPR data subject rights
- Orchestrates compliance across all platform operations

## Compliance Requirements

### TCPA (Telephone Consumer Protection Act)
- **Calling Hours**: 8 AM - 9 PM in called party's time zone
- **Prior Express Written Consent**: Required for marketing calls to mobile numbers
- **Opt-Out Mechanism**: Honor requests within 30 days
- **Caller ID**: Must transmit accurate caller ID information
- **Do Not Call Registry**: Check federal and state lists
- **Record Retention**: Maintain consent records for 5 years

### GDPR (General Data Protection Regulation)
- **Lawful Basis**: Document legal basis for processing (consent, legitimate interest)
- **Right to Access**: Provide data export within 30 days
- **Right to Erasure**: Delete personal data upon request
- **Data Portability**: Export data in machine-readable format
- **Consent Management**: Clear, granular, withdrawable consent
- **Privacy by Design**: Data minimization and purpose limitation

### DNC (Do Not Call) Requirements
- **Federal DNC**: Check against FTC registry
- **State DNC Lists**: Check applicable state registries
- **Internal Suppression**: Maintain company-specific opt-out list
- **Grace Period**: 31 days to honor new registrations
- **Safe Harbor**: Document compliance procedures
- **Update Frequency**: Monthly updates minimum

### State-Specific Regulations
- **California (CCPA)**: Additional privacy rights and opt-out requirements
- **Florida**: State-specific DNC list with 5-year retention
- **Texas**: Specific consent requirements for automated calls
- **New York**: Enhanced caller ID requirements
- **Other States**: Configurable rule engine for state requirements

## Domain Model

```go
package compliance

import (
    "time"
    "github.com/google/uuid"
)

// Core Compliance Entities

type Consent struct {
    ID             uuid.UUID
    PhoneNumber    PhoneNumber
    ConsentType    ConsentType
    ConsentMethod  ConsentMethod
    ConsentText    string
    ConsentedAt    time.Time
    ExpiresAt      *time.Time
    IPAddress      string
    UserAgent      string
    RecordedBy     uuid.UUID // User/System that recorded consent
    SourceURL      string
    Status         ConsentStatus
    Revocations    []ConsentRevocation
    CreatedAt      time.Time
    UpdatedAt      time.Time
}

type ConsentType string

const (
    ConsentTypeTCPA            ConsentType = "TCPA"
    ConsentTypeMarketing       ConsentType = "MARKETING"
    ConsentTypeTransactional   ConsentType = "TRANSACTIONAL"
    ConsentTypeDataProcessing  ConsentType = "DATA_PROCESSING"
    ConsentTypeThirdPartyShare ConsentType = "THIRD_PARTY_SHARE"
)

type ConsentMethod string

const (
    ConsentMethodWebForm      ConsentMethod = "WEB_FORM"
    ConsentMethodAPI          ConsentMethod = "API"
    ConsentMethodPhone        ConsentMethod = "PHONE"
    ConsentMethodSMS          ConsentMethod = "SMS"
    ConsentMethodImported     ConsentMethod = "IMPORTED"
)

type ConsentStatus string

const (
    ConsentStatusActive   ConsentStatus = "ACTIVE"
    ConsentStatusRevoked  ConsentStatus = "REVOKED"
    ConsentStatusExpired  ConsentStatus = "EXPIRED"
)

type ConsentRevocation struct {
    RevokedAt     time.Time
    RevocationMethod string
    Reason        string
    ProcessedBy   uuid.UUID
}

type SuppressionList struct {
    ID            uuid.UUID
    ListType      SuppressionListType
    Provider      string // e.g., "FTC", "FL_DNC", "INTERNAL"
    PhoneNumbers  []PhoneNumber
    LastUpdated   time.Time
    NextUpdate    time.Time
    Version       string
    RecordCount   int
}

type SuppressionListType string

const (
    SuppressionListTypeFederalDNC SuppressionListType = "FEDERAL_DNC"
    SuppressionListTypeStateDNC   SuppressionListType = "STATE_DNC"
    SuppressionListTypeInternal   SuppressionListType = "INTERNAL"
    SuppressionListTypeLitigation SuppressionListType = "LITIGATION"
)

type ComplianceRule struct {
    ID              uuid.UUID
    RuleType        ComplianceRuleType
    Jurisdiction    string // "US", "CA", "FL", etc.
    Name            string
    Description     string
    Conditions      []RuleCondition
    Actions         []RuleAction
    Priority        int
    EffectiveDate   time.Time
    ExpirationDate  *time.Time
    Enabled         bool
}

type ComplianceRuleType string

const (
    RuleTypeCallingHours   ComplianceRuleType = "CALLING_HOURS"
    RuleTypeConsentRequired ComplianceRuleType = "CONSENT_REQUIRED"
    RuleTypeDNCCheck       ComplianceRuleType = "DNC_CHECK"
    RuleTypeDataRetention  ComplianceRuleType = "DATA_RETENTION"
)

type RuleCondition struct {
    Field    string
    Operator string // "eq", "ne", "gt", "lt", "in", "between"
    Value    interface{}
}

type RuleAction struct {
    Type   string // "block", "require_consent", "add_disclosure"
    Params map[string]interface{}
}

type ComplianceCheck struct {
    ID              uuid.UUID
    CallID          uuid.UUID
    PhoneNumber     PhoneNumber
    CheckType       string
    RulesEvaluated  []uuid.UUID
    Result          ComplianceResult
    Violations      []ComplianceViolation
    ProcessingTime  time.Duration
    CheckedAt       time.Time
}

type ComplianceResult string

const (
    ComplianceResultPass      ComplianceResult = "PASS"
    ComplianceResultFail      ComplianceResult = "FAIL"
    ComplianceResultConditional ComplianceResult = "CONDITIONAL"
)

type ComplianceViolation struct {
    RuleID      uuid.UUID
    ViolationType string
    Description string
    Severity    string // "CRITICAL", "HIGH", "MEDIUM", "LOW"
    Remediation string
}

type AuditLog struct {
    ID            uuid.UUID
    EntityType    string // "consent", "suppression", "compliance_check"
    EntityID      uuid.UUID
    Action        string // "created", "updated", "deleted", "accessed"
    PerformedBy   uuid.UUID
    PerformedAt   time.Time
    IPAddress     string
    UserAgent     string
    Changes       map[string]interface{} // Old vs New values
    ComplianceRelevant bool
}

type DataSubjectRequest struct {
    ID              uuid.UUID
    RequestType     DSRType
    SubjectPhone    PhoneNumber
    SubjectEmail    *string
    SubmittedAt     time.Time
    ProcessedAt     *time.Time
    Status          DSRStatus
    ProcessedBy     *uuid.UUID
    ExportData      *string // JSON export for access requests
    DeletionLog     []DeletionRecord
    VerificationMethod string
    Notes           string
}

type DSRType string

const (
    DSRTypeAccess     DSRType = "ACCESS"
    DSRTypeErasure    DSRType = "ERASURE"
    DSRTypeRectification DSRType = "RECTIFICATION"
    DSRTypePortability DSRType = "PORTABILITY"
    DSRTypeRestriction DSRType = "RESTRICTION"
)

type DSRStatus string

const (
    DSRStatusPending    DSRStatus = "PENDING"
    DSRStatusProcessing DSRStatus = "PROCESSING"
    DSRStatusCompleted  DSRStatus = "COMPLETED"
    DSRStatusRejected   DSRStatus = "REJECTED"
)

type DeletionRecord struct {
    TableName    string
    RecordID     string
    DeletedAt    time.Time
    Confirmed    bool
}

// Value Objects

type PhoneNumber struct {
    Number      string
    CountryCode string
    IsValid     bool
}

type TimeWindow struct {
    StartTime time.Time
    EndTime   time.Time
    TimeZone  string
}

// Compliance Aggregates

type ComplianceProfile struct {
    PhoneNumber     PhoneNumber
    Consents        []Consent
    Suppressions    []SuppressionEntry
    LastChecked     time.Time
    ComplianceScore float64
}

type SuppressionEntry struct {
    ListType    SuppressionListType
    Provider    string
    AddedDate   time.Time
    ExpiryDate  *time.Time
    Reason      string
}
```

## Service Implementation

### 1. ComplianceOrchestrationService (Missing - Critical)
```go
type ComplianceOrchestrationService interface {
    // Pre-call compliance check - must complete in < 2ms
    ValidateCallCompliance(ctx context.Context, from, to PhoneNumber, callType CallType) (*ComplianceDecision, error)
    
    // Batch compliance check for multiple numbers
    BatchValidateCompliance(ctx context.Context, requests []ComplianceRequest) ([]ComplianceDecision, error)
    
    // Real-time compliance monitoring
    MonitorActiveCall(ctx context.Context, callID uuid.UUID) error
    
    // Compliance reporting
    GenerateComplianceReport(ctx context.Context, startDate, endDate time.Time) (*ComplianceReport, error)
}

type ComplianceDecision struct {
    Allowed       bool
    Reason        string
    Requirements  []string // e.g., "requires_disclosure", "record_call"
    Restrictions  []string // e.g., "no_marketing", "transactional_only"
    ValidUntil    time.Time
}
```

### 2. ConsentManagementService
```go
type ConsentManagementService interface {
    // Record new consent
    RecordConsent(ctx context.Context, consent *Consent) error
    
    // Check consent status
    GetConsentStatus(ctx context.Context, phoneNumber PhoneNumber, consentType ConsentType) (*ConsentStatus, error)
    
    // Revoke consent
    RevokeConsent(ctx context.Context, phoneNumber PhoneNumber, consentType ConsentType, reason string) error
    
    // Bulk consent import
    ImportConsents(ctx context.Context, consents []Consent) (*ImportResult, error)
    
    // Consent audit trail
    GetConsentHistory(ctx context.Context, phoneNumber PhoneNumber) ([]ConsentAuditEntry, error)
    
    // Consent expiration management
    ProcessExpiredConsents(ctx context.Context) error
}
```

### 3. DNCIntegrationService
```go
type DNCIntegrationService interface {
    // Check number against all applicable DNC lists
    CheckDNCStatus(ctx context.Context, phoneNumber PhoneNumber) (*DNCStatus, error)
    
    // Update DNC lists from providers
    UpdateDNCLists(ctx context.Context) error
    
    // Add to internal suppression
    AddToInternalDNC(ctx context.Context, phoneNumber PhoneNumber, reason string) error
    
    // Remove from internal suppression
    RemoveFromInternalDNC(ctx context.Context, phoneNumber PhoneNumber) error
    
    // Get suppression details
    GetSuppressionDetails(ctx context.Context, phoneNumber PhoneNumber) (*SuppressionDetails, error)
    
    // Validate DNC safe harbor compliance
    ValidateSafeHarbor(ctx context.Context) (*SafeHarborStatus, error)
}

type DNCStatus struct {
    OnFederalDNC    bool
    OnStateDNC      map[string]bool // state -> on list
    OnInternalDNC   bool
    LastChecked     time.Time
    NextCheckDue    time.Time
}
```

### 4. GDPRComplianceService
```go
type GDPRComplianceService interface {
    // Process data subject requests
    ProcessAccessRequest(ctx context.Context, request *DataSubjectRequest) (*DataExport, error)
    ProcessErasureRequest(ctx context.Context, request *DataSubjectRequest) (*ErasureResult, error)
    ProcessPortabilityRequest(ctx context.Context, request *DataSubjectRequest) (*PortableData, error)
    
    // Consent management
    RecordGDPRConsent(ctx context.Context, consent *GDPRConsent) error
    WithdrawGDPRConsent(ctx context.Context, dataSubjectID string, purposes []string) error
    
    // Data inventory
    GetDataInventory(ctx context.Context, phoneNumber PhoneNumber) (*DataInventory, error)
    
    // Right to be forgotten
    ExecuteDataErasure(ctx context.Context, phoneNumber PhoneNumber) (*ErasureLog, error)
}
```

### 5. AuditService
```go
type AuditService interface {
    // Log compliance-relevant actions
    LogComplianceAction(ctx context.Context, entry *AuditLog) error
    
    // Query audit logs
    QueryAuditLogs(ctx context.Context, filters AuditFilters) ([]AuditLog, error)
    
    // Generate audit reports
    GenerateAuditReport(ctx context.Context, startDate, endDate time.Time) (*AuditReport, error)
    
    // Compliance attestation
    GenerateComplianceAttestation(ctx context.Context, period string) (*Attestation, error)
}
```

## Infrastructure Components

### 1. DNC List Provider Integration
```go
package infrastructure

type DNCProviderClient interface {
    // Federal DNC
    DownloadFederalDNCList(ctx context.Context) (io.Reader, error)
    CheckFederalDNC(ctx context.Context, phoneNumbers []string) (map[string]bool, error)
    
    // State DNC
    DownloadStateDNCList(ctx context.Context, state string) (io.Reader, error)
    CheckStateDNC(ctx context.Context, state string, phoneNumbers []string) (map[string]bool, error)
    
    // Provider health check
    HealthCheck(ctx context.Context) error
}

// Implementation for each provider
type FTCDNCClient struct {
    apiKey     string
    httpClient *http.Client
    baseURL    string
}

type StateDNCProvider struct {
    state      string
    apiKey     string
    httpClient *http.Client
}
```

### 2. Consent Database Schema
```sql
-- Consent management tables
CREATE TABLE consents (
    id UUID PRIMARY KEY,
    phone_number VARCHAR(20) NOT NULL,
    consent_type VARCHAR(50) NOT NULL,
    consent_method VARCHAR(50) NOT NULL,
    consent_text TEXT,
    consented_at TIMESTAMP WITH TIME ZONE NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE,
    ip_address INET,
    user_agent TEXT,
    recorded_by UUID,
    source_url TEXT,
    status VARCHAR(20) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    INDEX idx_phone_consent (phone_number, consent_type),
    INDEX idx_consent_status (status, expires_at),
    INDEX idx_consented_at (consented_at)
);

CREATE TABLE consent_revocations (
    id UUID PRIMARY KEY,
    consent_id UUID REFERENCES consents(id),
    revoked_at TIMESTAMP WITH TIME ZONE NOT NULL,
    revocation_method VARCHAR(50),
    reason TEXT,
    processed_by UUID,
    
    INDEX idx_consent_revocation (consent_id)
);

-- Audit trail with partitioning for performance
CREATE TABLE audit_logs (
    id UUID PRIMARY KEY,
    entity_type VARCHAR(50) NOT NULL,
    entity_id UUID NOT NULL,
    action VARCHAR(50) NOT NULL,
    performed_by UUID,
    performed_at TIMESTAMP WITH TIME ZONE NOT NULL,
    ip_address INET,
    user_agent TEXT,
    changes JSONB,
    compliance_relevant BOOLEAN DEFAULT FALSE,
    
    INDEX idx_entity (entity_type, entity_id),
    INDEX idx_performed_at (performed_at),
    INDEX idx_compliance (compliance_relevant, performed_at)
) PARTITION BY RANGE (performed_at);

-- Create monthly partitions
CREATE TABLE audit_logs_2025_01 PARTITION OF audit_logs
    FOR VALUES FROM ('2025-01-01') TO ('2025-02-01');
```

### 3. Suppression List Caching (Redis)
```go
type SuppressionCache struct {
    client *redis.Client
    ttl    time.Duration
}

func (c *SuppressionCache) CheckNumber(ctx context.Context, phoneNumber string) (bool, error) {
    key := fmt.Sprintf("dnc:%s", phoneNumber)
    exists, err := c.client.Exists(ctx, key).Result()
    if err != nil {
        return false, err
    }
    return exists > 0, nil
}

func (c *SuppressionCache) BulkLoad(ctx context.Context, numbers []string, listType string) error {
    pipe := c.client.Pipeline()
    for _, number := range numbers {
        key := fmt.Sprintf("dnc:%s", number)
        pipe.Set(ctx, key, listType, c.ttl)
    }
    _, err := pipe.Exec(ctx)
    return err
}

// Bloom filter for memory efficiency
type BloomFilterCache struct {
    filter *bloom.Filter
    mutex  sync.RWMutex
}
```

### 4. Compliance Rules Engine
```go
type RulesEngine struct {
    rules []ComplianceRule
    cache map[string]*CompiledRule
}

type CompiledRule struct {
    rule       ComplianceRule
    conditions []func(context.Context, EvaluationContext) bool
    actions    []func(context.Context, EvaluationContext) error
}

func (e *RulesEngine) Evaluate(ctx context.Context, input EvaluationContext) (*RuleResult, error) {
    results := []RuleEvaluation{}
    
    for _, compiled := range e.cache {
        if !e.shouldEvaluate(compiled.rule, input) {
            continue
        }
        
        passed := true
        for _, condition := range compiled.conditions {
            if !condition(ctx, input) {
                passed = false
                break
            }
        }
        
        if passed {
            for _, action := range compiled.actions {
                if err := action(ctx, input); err != nil {
                    return nil, err
                }
            }
        }
        
        results = append(results, RuleEvaluation{
            RuleID: compiled.rule.ID,
            Passed: passed,
        })
    }
    
    return &RuleResult{
        Evaluations: results,
        Decision:    e.makeDecision(results),
    }, nil
}
```

## API Endpoints

### Consent Management
```yaml
# Record consent
POST /api/v1/compliance/consent
Request:
  {
    "phone_number": "+14155551234",
    "consent_type": "TCPA",
    "consent_method": "WEB_FORM",
    "consent_text": "I agree to receive calls...",
    "ip_address": "192.168.1.1",
    "source_url": "https://example.com/signup"
  }
Response: 201 Created
  {
    "consent_id": "550e8400-e29b-41d4-a716-446655440000",
    "status": "ACTIVE",
    "expires_at": "2026-01-15T10:00:00Z"
  }

# Get consent status
GET /api/v1/compliance/consent/{phone}?type=TCPA
Response: 200 OK
  {
    "phone_number": "+14155551234",
    "consents": [
      {
        "type": "TCPA",
        "status": "ACTIVE",
        "consented_at": "2025-01-15T10:00:00Z",
        "expires_at": "2026-01-15T10:00:00Z"
      }
    ]
  }

# Revoke consent
DELETE /api/v1/compliance/consent/{phone}?type=TCPA
Request:
  {
    "reason": "User requested opt-out",
    "method": "SMS"
  }
Response: 204 No Content
```

### DNC Checking
```yaml
# Check DNC status
POST /api/v1/compliance/dnc/check
Request:
  {
    "phone_numbers": ["+14155551234", "+14155551235"],
    "check_federal": true,
    "check_states": ["CA", "FL"]
  }
Response: 200 OK
  {
    "results": [
      {
        "phone_number": "+14155551234",
        "on_federal_dnc": false,
        "on_state_dnc": {"CA": false, "FL": true},
        "on_internal_dnc": false,
        "can_call": true
      }
    ]
  }

# Bulk DNC check
POST /api/v1/compliance/dnc/check/bulk
Request: multipart/form-data with CSV file
Response: 202 Accepted
  {
    "job_id": "550e8400-e29b-41d4-a716-446655440000",
    "status_url": "/api/v1/compliance/jobs/550e8400-e29b-41d4-a716-446655440000"
  }
```

### GDPR Operations
```yaml
# Data access request
POST /api/v1/compliance/gdpr/access
Request:
  {
    "phone_number": "+14155551234",
    "email": "user@example.com",
    "verification_code": "ABC123"
  }
Response: 202 Accepted
  {
    "request_id": "550e8400-e29b-41d4-a716-446655440000",
    "status": "PROCESSING",
    "estimated_completion": "2025-01-16T10:00:00Z"
  }

# Data erasure request
POST /api/v1/compliance/gdpr/erasure
Request:
  {
    "phone_number": "+14155551234",
    "email": "user@example.com",
    "verification_code": "ABC123",
    "confirmation": "I understand this will permanently delete my data"
  }
Response: 202 Accepted

# Data portability
POST /api/v1/compliance/gdpr/portability
Response: 200 OK
  {
    "format": "JSON",
    "download_url": "https://secure.example.com/downloads/data-export-12345.json",
    "expires_at": "2025-01-16T10:00:00Z"
  }
```

### Compliance Validation
```yaml
# Pre-call compliance check
POST /api/v1/compliance/validate
Request:
  {
    "from_number": "+14155551234",
    "to_number": "+14155551235",
    "call_type": "MARKETING",
    "scheduled_time": "2025-01-15T14:00:00-05:00"
  }
Response: 200 OK
  {
    "compliant": true,
    "checks_performed": [
      {"type": "TCPA_HOURS", "passed": true},
      {"type": "CONSENT", "passed": true},
      {"type": "DNC", "passed": true}
    ],
    "requirements": ["RECORD_CALL", "PLAY_DISCLOSURE"],
    "valid_until": "2025-01-15T21:00:00-05:00"
  }
```

### Audit Trail
```yaml
# Get audit trail
GET /api/v1/compliance/audit?phone=+14155551234&start_date=2025-01-01
Response: 200 OK
  {
    "entries": [
      {
        "timestamp": "2025-01-15T10:00:00Z",
        "action": "CONSENT_RECORDED",
        "entity_type": "consent",
        "entity_id": "550e8400-e29b-41d4-a716-446655440000",
        "performed_by": "user-123",
        "details": {
          "consent_type": "TCPA",
          "method": "WEB_FORM"
        }
      }
    ],
    "total": 42,
    "page": 1
  }
```

## Real-time Validation

### Pre-call Compliance Check Flow
```go
func (s *ComplianceOrchestrationService) ValidateCallCompliance(
    ctx context.Context, 
    from, to PhoneNumber, 
    callType CallType,
) (*ComplianceDecision, error) {
    // Create trace for monitoring
    ctx, span := tracer.Start(ctx, "compliance.validate_call")
    defer span.End()
    
    // Parallel compliance checks
    var (
        consentResult   *ConsentStatus
        dncResult       *DNCStatus
        timeResult      *TimeWindowCheck
        ruleResult      *RuleEvaluation
        wg              sync.WaitGroup
        mu              sync.Mutex
        errors          []error
    )
    
    // Check consent (if required)
    wg.Add(1)
    go func() {
        defer wg.Done()
        if callType.RequiresConsent() {
            status, err := s.consentService.GetConsentStatus(ctx, to, ConsentTypeTCPA)
            mu.Lock()
            consentResult = status
            if err != nil {
                errors = append(errors, err)
            }
            mu.Unlock()
        }
    }()
    
    // Check DNC status
    wg.Add(1)
    go func() {
        defer wg.Done()
        status, err := s.dncService.CheckDNCStatus(ctx, to)
        mu.Lock()
        dncResult = status
        if err != nil {
            errors = append(errors, err)
        }
        mu.Unlock()
    }()
    
    // Check calling hours
    wg.Add(1)
    go func() {
        defer wg.Done()
        check, err := s.checkCallingHours(ctx, to)
        mu.Lock()
        timeResult = check
        if err != nil {
            errors = append(errors, err)
        }
        mu.Unlock()
    }()
    
    // Evaluate rules
    wg.Add(1)
    go func() {
        defer wg.Done()
        eval, err := s.rulesEngine.Evaluate(ctx, EvaluationContext{
            From:     from,
            To:       to,
            CallType: callType,
        })
        mu.Lock()
        ruleResult = eval
        if err != nil {
            errors = append(errors, err)
        }
        mu.Unlock()
    }()
    
    // Wait with timeout
    done := make(chan struct{})
    go func() {
        wg.Wait()
        close(done)
    }()
    
    select {
    case <-done:
        // All checks completed
    case <-time.After(2 * time.Millisecond):
        return &ComplianceDecision{
            Allowed: false,
            Reason:  "Compliance check timeout",
        }, ErrComplianceTimeout
    }
    
    // Compile decision
    decision := s.compileDecision(consentResult, dncResult, timeResult, ruleResult)
    
    // Log compliance check
    s.auditService.LogComplianceAction(ctx, &AuditLog{
        EntityType: "compliance_check",
        EntityID:   decision.ID,
        Action:     "validated",
        Changes: map[string]interface{}{
            "from":     from,
            "to":       to,
            "decision": decision.Allowed,
            "reason":   decision.Reason,
        },
    })
    
    return decision, nil
}
```

### Performance Optimization
```go
// Cache warming for frequently checked numbers
type ComplianceCache struct {
    consentCache *ristretto.Cache
    dncCache     *ristretto.Cache
    mutex        sync.RWMutex
}

func (c *ComplianceCache) WarmCache(ctx context.Context, phoneNumbers []PhoneNumber) {
    // Bulk load consent status
    consents, _ := c.consentService.BulkGetConsentStatus(ctx, phoneNumbers)
    for phone, status := range consents {
        c.consentCache.Set(phone, status, 1)
    }
    
    // Bulk load DNC status
    dncStatuses, _ := c.dncService.BulkCheckDNC(ctx, phoneNumbers)
    for phone, status := range dncStatuses {
        c.dncCache.Set(phone, status, 1)
    }
}
```

## Testing Requirements

### 1. Compliance Scenario Testing
```go
func TestTCPAComplianceScenarios(t *testing.T) {
    tests := []struct {
        name         string
        callTime     time.Time
        timezone     string
        hasConsent   bool
        onDNC        bool
        expectAllow  bool
    }{
        {
            name:        "valid call with consent",
            callTime:    time.Date(2025, 1, 15, 14, 0, 0, 0, time.UTC),
            timezone:    "America/New_York",
            hasConsent:  true,
            onDNC:       false,
            expectAllow: true,
        },
        {
            name:        "call outside hours",
            callTime:    time.Date(2025, 1, 15, 22, 0, 0, 0, time.UTC),
            timezone:    "America/New_York",
            hasConsent:  true,
            onDNC:       false,
            expectAllow: false,
        },
        {
            name:        "call to DNC number",
            callTime:    time.Date(2025, 1, 15, 14, 0, 0, 0, time.UTC),
            timezone:    "America/New_York",
            hasConsent:  true,
            onDNC:       true,
            expectAllow: false,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test implementation
        })
    }
}
```

### 2. Time Zone Handling Tests
```go
func TestTimeZoneCompliance(t *testing.T) {
    // Test all US time zones
    timezones := []string{
        "America/New_York",
        "America/Chicago",
        "America/Denver",
        "America/Los_Angeles",
        "America/Anchorage",
        "Pacific/Honolulu",
    }
    
    for _, tz := range timezones {
        t.Run(tz, func(t *testing.T) {
            loc, _ := time.LoadLocation(tz)
            
            // Test morning boundary (8 AM)
            morning := time.Date(2025, 1, 15, 8, 0, 0, 0, loc)
            decision := service.CheckCallingHours(morning, tz)
            assert.True(t, decision.Allowed)
            
            // Test evening boundary (9 PM)
            evening := time.Date(2025, 1, 15, 21, 0, 0, 0, loc)
            decision = service.CheckCallingHours(evening, tz)
            assert.False(t, decision.Allowed)
        })
    }
}
```

### 3. DNC Integration Tests
```go
func TestDNCIntegration(t *testing.T) {
    t.Run("federal DNC check", func(t *testing.T) {
        // Mock DNC provider
        mockProvider := &MockDNCProvider{
            Numbers: map[string]bool{
                "+14155551234": true,
                "+14155551235": false,
            },
        }
        
        service := NewDNCService(mockProvider)
        
        // Test batch check
        results, err := service.CheckNumbers(ctx, []string{
            "+14155551234",
            "+14155551235",
        })
        
        assert.NoError(t, err)
        assert.True(t, results["+14155551234"])
        assert.False(t, results["+14155551235"])
    })
    
    t.Run("cache performance", func(t *testing.T) {
        // Test cache hit performance < 0.1ms
        start := time.Now()
        for i := 0; i < 1000; i++ {
            service.CheckCached(ctx, "+14155551234")
        }
        elapsed := time.Since(start)
        assert.Less(t, elapsed/1000, 100*time.Microsecond)
    })
}
```

### 4. GDPR Workflow Tests
```go
func TestGDPRDataSubjectRequests(t *testing.T) {
    t.Run("data export request", func(t *testing.T) {
        // Create test data
        phone := "+14155551234"
        testData := createTestUserData(phone)
        
        // Submit export request
        request := &DataSubjectRequest{
            RequestType:  DSRTypeAccess,
            SubjectPhone: phone,
        }
        
        export, err := service.ProcessAccessRequest(ctx, request)
        assert.NoError(t, err)
        
        // Verify export contains all data
        assert.Contains(t, export.CallHistory, testData.Calls)
        assert.Contains(t, export.Consents, testData.Consents)
        assert.Contains(t, export.BillingRecords, testData.Billing)
    })
    
    t.Run("data erasure request", func(t *testing.T) {
        // Submit erasure request
        request := &DataSubjectRequest{
            RequestType:  DSRTypeErasure,
            SubjectPhone: phone,
        }
        
        result, err := service.ProcessErasureRequest(ctx, request)
        assert.NoError(t, err)
        
        // Verify data is deleted
        for _, record := range result.DeletionLog {
            assert.True(t, record.Confirmed)
        }
        
        // Verify audit trail preserved
        audit, _ := service.GetAuditTrail(ctx, phone)
        assert.NotEmpty(t, audit)
    })
}
```

### 5. Performance Benchmarks
```go
func BenchmarkComplianceCheck(b *testing.B) {
    service := setupComplianceService()
    ctx := context.Background()
    
    b.Run("single check", func(b *testing.B) {
        for i := 0; i < b.N; i++ {
            service.ValidateCallCompliance(ctx,
                "+14155551234",
                "+14155551235",
                CallTypeMarketing,
            )
        }
    })
    
    b.Run("parallel checks", func(b *testing.B) {
        b.RunParallel(func(pb *testing.PB) {
            for pb.Next() {
                service.ValidateCallCompliance(ctx,
                    "+14155551234",
                    "+14155551235",
                    CallTypeMarketing,
                )
            }
        })
    })
}
```

## Integration Points

### 1. Call Routing Integration
```go
// In CallRoutingService
func (s *CallRoutingService) RouteCall(ctx context.Context, call *Call) (*RoutingDecision, error) {
    // First, check compliance
    compliance, err := s.complianceService.ValidateCallCompliance(
        ctx,
        call.FromNumber,
        call.ToNumber,
        call.Type,
    )
    
    if err != nil {
        return nil, fmt.Errorf("compliance check failed: %w", err)
    }
    
    if !compliance.Allowed {
        // Log blocked call
        s.metricsClient.IncrementCounter("calls.blocked.compliance", 
            map[string]string{"reason": compliance.Reason})
        
        return &RoutingDecision{
            Action: ActionBlock,
            Reason: compliance.Reason,
        }, nil
    }
    
    // Apply compliance requirements
    routingOptions := s.applyComplianceRequirements(call, compliance)
    
    // Continue with normal routing
    return s.performRouting(ctx, call, routingOptions)
}
```

### 2. Billing Integration
```go
// Track consent in billing records
type BillingRecord struct {
    ID            uuid.UUID
    CallID        uuid.UUID
    ConsentID     *uuid.UUID // Link to consent record
    ComplianceLog ComplianceAudit
    // ... other fields
}

func (s *BillingService) CreateBillingRecord(ctx context.Context, call *Call) error {
    // Get compliance audit for the call
    audit, err := s.complianceService.GetCallComplianceAudit(ctx, call.ID)
    if err != nil {
        return err
    }
    
    record := &BillingRecord{
        CallID:        call.ID,
        ConsentID:     audit.ConsentID,
        ComplianceLog: audit,
    }
    
    return s.repo.CreateBillingRecord(ctx, record)
}
```

### 3. Analytics Integration
```go
// GDPR-compliant analytics
func (s *AnalyticsService) ProcessCallMetrics(ctx context.Context, call *Call) error {
    // Check if user has opted out of analytics
    consent, _ := s.complianceService.GetConsentStatus(
        ctx, 
        call.ToNumber,
        ConsentTypeDataProcessing,
    )
    
    if consent == nil || consent.Status != ConsentStatusActive {
        // Only process anonymized metrics
        return s.processAnonymizedMetrics(ctx, call)
    }
    
    // Process full metrics with consent
    return s.processFullMetrics(ctx, call)
}
```

## Migration Plan

### Phase 1: Foundation (Day 1-2)
1. Implement domain models
2. Create database schema and migrations
3. Set up DNC provider integrations
4. Implement consent management service

### Phase 2: Core Services (Day 3-4)
1. Build ComplianceOrchestrationService
2. Implement rules engine
3. Create caching layer
4. Add audit service

### Phase 3: API & Integration (Day 5)
1. Implement all API endpoints
2. Integrate with call routing
3. Add billing integration
4. Complete GDPR workflows

### Phase 4: Testing & Optimization (Day 6)
1. Comprehensive testing
2. Performance optimization
3. Load testing
4. Documentation

## Performance Considerations

### Caching Strategy
- Redis for DNC lookups (< 0.1ms)
- In-memory consent cache with TTL
- Bloom filters for initial DNC screening
- Warm cache for high-volume numbers

### Database Optimization
- Partitioned audit tables by month
- Indexed consent lookups
- Read replicas for queries
- Connection pooling

### Concurrency
- Parallel compliance checks
- Non-blocking audit logging
- Batch processing for bulk operations
- Circuit breakers for external services

## Security Considerations

1. **Data Encryption**
   - Encrypt PII at rest
   - TLS for all external communications
   - Secure key management

2. **Access Control**
   - Role-based permissions
   - API authentication required
   - Audit all access

3. **Data Retention**
   - Automated data expiration
   - Secure deletion procedures
   - Compliance with retention laws

## Monitoring & Alerts

### Key Metrics
- Compliance check latency (target < 2ms)
- DNC cache hit rate (target > 95%)
- Consent verification success rate
- GDPR request processing time

### Critical Alerts
- Compliance check failures
- DNC provider downtime
- Consent expiration warnings
- Audit trail gaps

## Effort Estimate

| Component | Days | Priority |
|-----------|------|----------|
| Domain Models | 0.5 | Critical |
| Database Schema | 0.5 | Critical |
| Consent Service | 1.0 | Critical |
| DNC Integration | 1.0 | Critical |
| Compliance Orchestration | 1.0 | Critical |
| GDPR Implementation | 1.0 | High |
| API Endpoints | 0.5 | Critical |
| Testing & Integration | 1.0 | Critical |
| Performance Optimization | 0.5 | High |
| **Total** | **6.0** | - |

## Success Criteria

1. **Functional Requirements**
   - All TCPA rules enforced
   - DNC lists checked in real-time
   - GDPR requests processed within legal timeframes
   - Complete audit trail maintained

2. **Performance Requirements**
   - Pre-call compliance check < 2ms (p99)
   - DNC cache hit rate > 95%
   - Consent lookup < 1ms
   - Bulk operations < 10ms per record

3. **Compliance Requirements**
   - 100% TCPA compliance
   - GDPR Article 15-22 implementation
   - State-specific rules supported
   - Safe harbor documentation

4. **Integration Requirements**
   - Seamless call routing integration
   - Billing record compliance tracking
   - Analytics respects consent
   - No impact on existing performance

This comprehensive compliance platform will transform the current 40% implementation into a robust, legally compliant system that protects the business while enabling growth. The modular design allows for future expansion as regulations evolve.