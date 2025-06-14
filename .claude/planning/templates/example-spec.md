---
feature: Enhanced Consent Management
domain: call
priority: critical
effort: large
type: enhancement
---

# Feature Specification: Enhanced Consent Management

## Overview
Implement comprehensive consent management for call recordings to ensure TCPA compliance and support state-specific two-party consent requirements. This feature will track, validate, and audit all consent interactions while maintaining sub-millisecond performance for consent checks during call routing.

## Business Requirements
- Support both one-party and two-party consent states
- Real-time consent validation during call routing
- Immutable audit trail for all consent events
- Granular consent types (recording, transcription, analytics)
- Consent expiration and renewal workflows
- GDPR-compliant consent withdrawal

## Technical Specification

### Domain Model Changes
```yaml
entities:
  - name: ConsentRecord
    changes:
      - Add field: ID uuid.UUID
      - Add field: CallID uuid.UUID
      - Add field: BuyerID uuid.UUID
      - Add field: ConsentType ConsentType
      - Add field: ConsentMethod ConsentMethod
      - Add field: ConsentedAt time.Time
      - Add field: ExpiresAt *time.Time
      - Add field: RevokedAt *time.Time
      - Add field: Metadata map[string]interface{}
      - Add method: IsValid() bool
      - Add method: Revoke(reason string) error
      - Add method: Renew(duration time.Duration) error
    
value_objects:
  - name: ConsentType
    purpose: Type-safe consent categories
    validation: Must be one of: Recording, Transcription, Analytics, Marketing
    
  - name: ConsentMethod
    purpose: How consent was obtained
    validation: Must be one of: Verbal, Written, Electronic, Implied
    
  - name: ConsentState
    purpose: Track consent lifecycle
    validation: Must be one of: Pending, Granted, Revoked, Expired
    
domain_events:
  - name: ConsentGranted
    fields:
      - ConsentID uuid.UUID
      - CallID uuid.UUID
      - BuyerID uuid.UUID
      - Type ConsentType
      - Method ConsentMethod
      
  - name: ConsentRevoked
    fields:
      - ConsentID uuid.UUID
      - Reason string
      - RevokedBy string
      
  - name: ConsentExpired
    fields:
      - ConsentID uuid.UUID
      - ExpiredAt time.Time
```

### Service Requirements
```yaml
services:
  - name: ConsentService
    operations:
      - GrantConsent: Record new consent with validation
      - CheckConsent: Fast consent verification for routing
      - RevokeConsent: Handle consent withdrawal
      - GetConsentHistory: Audit trail retrieval
      - BulkConsentCheck: Check multiple consents efficiently
    dependencies:
      - ConsentRepository
      - ComplianceService
      - CacheService
      - EventPublisher
      
  - name: ConsentValidationService
    operations:
      - ValidateForState: Check state-specific requirements
      - ValidateConsentMethod: Ensure method is acceptable
      - ValidateExpiration: Check consent validity period
    dependencies:
      - StateComplianceRules
      - ConsentRepository
```

### API Specification
```yaml
endpoints:
  - method: POST
    path: /api/v1/calls/{call_id}/consent
    request:
      type: GrantConsentRequest
      fields:
        - consent_type: string
        - consent_method: string
        - expires_in_days: int (optional)
        - metadata: object (optional)
    response:
      type: ConsentResponse
      fields:
        - consent_id: string
        - status: string
        - valid_until: string
    rate_limit: 1000/second
    auth: JWT required
    
  - method: GET
    path: /api/v1/calls/{call_id}/consent
    response:
      type: ConsentListResponse
      fields:
        - consents: array[ConsentResponse]
        - total: int
    rate_limit: 5000/second
    auth: JWT required
    
  - method: DELETE
    path: /api/v1/consent/{consent_id}
    request:
      type: RevokeConsentRequest
      fields:
        - reason: string
    response:
      type: ConsentResponse
    rate_limit: 100/second
    auth: JWT required
    
  - method: GET
    path: /api/v1/consent/check
    query_params:
      - call_id: string
      - buyer_id: string
      - consent_type: string
    response:
      type: ConsentCheckResponse
      fields:
        - is_valid: boolean
        - consent_id: string (if exists)
        - expires_at: string (if exists)
    rate_limit: 10000/second
    auth: JWT required
    cache: 60 seconds
```

### Repository Requirements
```yaml
repositories:
  - name: ConsentRepository
    operations:
      - Create(consent *ConsentRecord) error
      - GetByID(id uuid.UUID) (*ConsentRecord, error)
      - GetByCall(callID uuid.UUID) ([]*ConsentRecord, error)
      - GetValidConsent(callID, buyerID uuid.UUID, consentType ConsentType) (*ConsentRecord, error)
      - Update(consent *ConsentRecord) error
      - GetExpiring(within time.Duration) ([]*ConsentRecord, error)
    indexes:
      - (call_id, buyer_id, consent_type, revoked_at)
      - (expires_at) WHERE revoked_at IS NULL
      - (created_at) for audit queries
```

### Performance Requirements
- Consent check latency: < 1ms p99
- Bulk consent check: < 5ms for 100 records
- Write operations: < 10ms p99
- Cache hit rate: > 95%
- Database query time: < 2ms

### Compliance Requirements
- TCPA: 
  - Two-party consent for: CA, CT, FL, IL, MD, MA, MT, NV, NH, PA, WA
  - Consent must be clear and conspicuous
  - Written consent required for marketing calls
- GDPR:
  - Explicit consent required
  - Easy withdrawal mechanism
  - Data portability support
  - 30-day deletion after revocation
- Audit:
  - All consent events must be logged
  - Immutable audit trail
  - 7-year retention period

### Security Requirements
- Authentication: JWT required for all endpoints
- Authorization: 
  - Buyers can only manage their own consents
  - Admins can view all consents
  - Sellers can view consents for their calls
- Validation:
  - Validate UUIDs format
  - Sanitize metadata fields
  - Rate limit by IP and user

### Testing Strategy
- Unit Tests:
  - Domain logic: 100% coverage
  - Service layer: 95% coverage
  - Repository: 90% coverage
- Integration Tests:
  - Full consent lifecycle
  - State-specific validation
  - Cache invalidation
  - Event publishing
- Performance Tests:
  - 100K concurrent consent checks
  - Cache performance under load
  - Database query optimization
- Security Tests:
  - Authorization bypass attempts
  - SQL injection tests
  - Rate limit verification

### Migration Plan
- Database Changes:
  ```sql
  CREATE TABLE consent_records (
      id UUID PRIMARY KEY,
      call_id UUID NOT NULL REFERENCES calls(id),
      buyer_id UUID NOT NULL REFERENCES accounts(id),
      consent_type VARCHAR(50) NOT NULL,
      consent_method VARCHAR(50) NOT NULL,
      consented_at TIMESTAMPTZ NOT NULL,
      expires_at TIMESTAMPTZ,
      revoked_at TIMESTAMPTZ,
      revoked_reason TEXT,
      metadata JSONB,
      created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
      updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
  );
  
  CREATE INDEX idx_consent_lookup ON consent_records(call_id, buyer_id, consent_type, revoked_at);
  CREATE INDEX idx_consent_expiry ON consent_records(expires_at) WHERE revoked_at IS NULL;
  CREATE INDEX idx_consent_audit ON consent_records(created_at);
  ```
- Backward Compatibility: 
  - Existing calls without consent treated as "implied consent" for one-party states
  - Grace period for obtaining consent on existing relationships
- Rollback Strategy:
  - Feature flag: ENHANCED_CONSENT_ENABLED
  - Dual-write period for safe transition
  - Keep old consent table for 30 days

### Monitoring & Observability
```yaml
metrics:
  - consent_checks_total{result="hit|miss|error"}
  - consent_check_duration_seconds
  - consent_grants_total{type,method}
  - consent_revocations_total{reason}
  - consent_cache_hit_rate
  - consent_validation_errors_total{error_type}

logs:
  - Consent granted: INFO level with full context
  - Consent revoked: WARN level with reason
  - Consent expired: INFO level
  - Validation failures: ERROR level

traces:
  - Instrument all service methods
  - Include cache hit/miss in span attributes
  - Track database query time

alerts:
  - High consent check latency (> 5ms p99)
  - Low cache hit rate (< 90%)
  - Spike in validation errors (> 100/min)
  - Consent expiration backlog (> 1000)
```

### Dependencies
- Blocks: Enhanced call recording features
- Blocked By: None
- Related To: Call recording infrastructure, Compliance reporting

### Acceptance Criteria
1. ✓ All consent operations complete successfully
2. ✓ Consent checks perform under 1ms p99
3. ✓ Two-party consent states properly validated
4. ✓ Audit trail captures all events
5. ✓ GDPR withdrawal mechanism functional
6. ✓ Cache properly invalidated on changes
7. ✓ All security requirements met
8. ✓ 95%+ test coverage achieved
9. ✓ Performance benchmarks passed
10. ✓ Documentation complete