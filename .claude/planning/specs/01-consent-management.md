# Consent Management System Specification

## Overview

**Priority:** CRITICAL (Risk Score: 95/100)  
**Timeline:** Week 1-2 (Emergency), Week 3-4 (Full Implementation)  
**Team:** 2 Senior Engineers  
**Revenue Impact:** +$3M/year (enables compliant accounts)  
**Risk Mitigation:** $2M/year (TCPA violation prevention)

## Business Context

### Problem Statement
The platform currently processes calls with ZERO consent verification, exposing the business to:
- TCPA violations at $500-$1,500 per call
- Class action lawsuits with $10M+ exposure
- Loss of 40% of enterprise clients requiring compliance
- Inability to operate in strict compliance states

### Success Criteria
- 100% of calls verified for consent before routing
- < 50ms consent lookup latency
- Zero false negatives (never allow unconsented calls)
- Complete audit trail of all consent actions
- Support for 10M+ consent records

## Technical Specification

### Domain Model

```go
// internal/domain/compliance/consent.go
package compliance

type Consent struct {
    ID           uuid.UUID
    PhoneNumber  values.PhoneNumber // E.164 format
    ConsentType  ConsentType
    Channel      ConsentChannel
    Status       ConsentStatus
    GrantedAt    time.Time
    ExpiresAt    *time.Time
    RevokedAt    *time.Time
    Source       ConsentSource
    IPAddress    *string
    UserAgent    *string
    RecordingURL *string // Voice consent recording
    Metadata     map[string]interface{}
}

type ConsentType string
const (
    ConsentTypeMarketing ConsentType = "marketing"
    ConsentTypeSales     ConsentType = "sales"
    ConsentTypeService   ConsentType = "service"
)

type ConsentChannel string
const (
    ConsentChannelVoice ConsentChannel = "voice"
    ConsentChannelSMS   ConsentChannel = "sms"
    ConsentChannelEmail ConsentChannel = "email"
    ConsentChannelWeb   ConsentChannel = "web"
)

type ConsentStatus string
const (
    ConsentStatusActive  ConsentStatus = "active"
    ConsentStatusExpired ConsentStatus = "expired"
    ConsentStatusRevoked ConsentStatus = "revoked"
)

type ConsentSource struct {
    Type       string // "api", "import", "manual", "ivr"
    Identifier string // API key, user ID, etc.
    Timestamp  time.Time
}
```

### Service Layer

```go
// internal/service/consent/service.go
package consent

type Service interface {
    // Core consent operations
    GrantConsent(ctx context.Context, req GrantConsentRequest) (*Consent, error)
    RevokeConsent(ctx context.Context, phoneNumber string, reason string) error
    CheckConsent(ctx context.Context, phoneNumber string, consentType ConsentType) (bool, error)
    
    // Bulk operations
    ImportConsents(ctx context.Context, consents []ImportConsentRequest) (ImportResult, error)
    ExportConsents(ctx context.Context, filter ExportFilter) (io.Reader, error)
    
    // Compliance operations
    GetConsentHistory(ctx context.Context, phoneNumber string) ([]ConsentEvent, error)
    PurgeExpiredConsents(ctx context.Context) (int64, error)
}

type GrantConsentRequest struct {
    PhoneNumber  string
    ConsentType  ConsentType
    Channel      ConsentChannel
    ExpiresIn    *time.Duration
    IPAddress    *string
    UserAgent    *string
    RecordingURL *string
    Metadata     map[string]interface{}
}
```

### Infrastructure Layer

```go
// internal/infrastructure/database/consent_repository.go
package database

type ConsentRepository interface {
    Create(ctx context.Context, consent *domain.Consent) error
    Update(ctx context.Context, consent *domain.Consent) error
    GetByPhoneNumber(ctx context.Context, phoneNumber string) ([]*domain.Consent, error)
    GetActive(ctx context.Context, phoneNumber string, consentType domain.ConsentType) (*domain.Consent, error)
    ListExpired(ctx context.Context, before time.Time) ([]*domain.Consent, error)
    Delete(ctx context.Context, id uuid.UUID) error
}

// Redis cache for performance
type ConsentCache interface {
    Set(ctx context.Context, phoneNumber string, consent *domain.Consent, ttl time.Duration) error
    Get(ctx context.Context, phoneNumber string, consentType domain.ConsentType) (*domain.Consent, error)
    Invalidate(ctx context.Context, phoneNumber string) error
}
```

### API Endpoints

```yaml
# REST API
POST   /api/v1/consent                    # Grant consent
DELETE /api/v1/consent/{phone}            # Revoke consent
GET    /api/v1/consent/{phone}            # Get consent status
GET    /api/v1/consent/{phone}/history    # Get consent history
POST   /api/v1/consent/import             # Bulk import
GET    /api/v1/consent/export             # Export consents

# Webhook Events
consent.granted
consent.revoked
consent.expired
consent.imported
```

### Database Schema

```sql
-- Consent records table
CREATE TABLE consents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    phone_number VARCHAR(20) NOT NULL,
    phone_hash VARCHAR(64) NOT NULL, -- For indexing
    consent_type VARCHAR(20) NOT NULL,
    channel VARCHAR(20) NOT NULL,
    status VARCHAR(20) NOT NULL,
    granted_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    source_type VARCHAR(20) NOT NULL,
    source_identifier VARCHAR(255),
    ip_address INET,
    user_agent TEXT,
    recording_url TEXT,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes for performance
CREATE INDEX idx_consent_phone_hash ON consents(phone_hash);
CREATE INDEX idx_consent_phone_type_status ON consents(phone_number, consent_type, status);
CREATE INDEX idx_consent_expires_at ON consents(expires_at) WHERE expires_at IS NOT NULL;
CREATE INDEX idx_consent_status ON consents(status);

-- Consent events for audit trail
CREATE TABLE consent_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    consent_id UUID NOT NULL REFERENCES consents(id),
    event_type VARCHAR(50) NOT NULL,
    actor_id VARCHAR(255),
    actor_type VARCHAR(50),
    reason TEXT,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_consent_events_consent_id ON consent_events(consent_id);
CREATE INDEX idx_consent_events_created_at ON consent_events(created_at);
```

## Implementation Plan

### Phase 0: Emergency Implementation (Week 1)

**Day 1-2: Core Domain & Database**
- [ ] Create consent domain entities
- [ ] Implement database schema
- [ ] Basic repository implementation
- [ ] Migration scripts

**Day 3-4: Service Layer**
- [ ] Implement consent service
- [ ] Add Redis caching layer
- [ ] Create consent checker middleware
- [ ] Basic import tool

**Day 5: Integration & Testing**
- [ ] Integrate with call routing
- [ ] Emergency consent import
- [ ] Basic testing
- [ ] Deploy to staging

### Phase 1: Full Implementation (Week 3-4)

**Week 3: Enhanced Features**
- [ ] Bulk import/export APIs
- [ ] Consent preference center
- [ ] Double opt-in workflows
- [ ] Webhook notifications
- [ ] Advanced expiration handling

**Week 4: Production Readiness**
- [ ] Performance optimization
- [ ] Comprehensive testing
- [ ] Monitoring & alerting
- [ ] Documentation
- [ ] Production deployment

## Integration Points

### Call Routing Integration
```go
// Middleware for call routing
func ConsentMiddleware(consentService consent.Service) func(next http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            call := getCallFromContext(r.Context())
            
            hasConsent, err := consentService.CheckConsent(
                r.Context(), 
                call.ToNumber.String(), 
                consent.ConsentTypeMarketing,
            )
            
            if err != nil || !hasConsent {
                // Log compliance violation attempt
                // Return 403 Forbidden
                return
            }
            
            next.ServeHTTP(w, r)
        })
    }
}
```

### Event Publishing
```go
// Publish consent events for audit trail
type ConsentGrantedEvent struct {
    ConsentID   uuid.UUID
    PhoneNumber string
    ConsentType ConsentType
    Channel     ConsentChannel
    GrantedAt   time.Time
    ExpiresAt   *time.Time
}

// Published to: consent.granted topic
```

## Performance Requirements

- **Consent Check Latency:** < 50ms (p99)
- **Write Throughput:** 1000 consents/second
- **Read Throughput:** 10,000 checks/second
- **Cache Hit Rate:** > 95%
- **Storage:** Support 100M+ consent records

## Security Considerations

- Phone numbers encrypted at rest
- PII access logged for audit
- API rate limiting per client
- Consent proof storage (recordings)
- GDPR-compliant data retention

## Monitoring & Alerting

### Key Metrics
- Consent check latency (target: < 50ms)
- Consent grant/revoke rate
- Cache hit rate (target: > 95%)
- Expired consent cleanup rate
- API error rates

### Alerts
- High latency (> 100ms)
- Failed consent checks
- Cache failures
- Database connection issues
- Unusual revocation patterns

## Testing Strategy

### Unit Tests
- Domain logic validation
- Service layer business rules
- Repository operations
- Cache operations

### Integration Tests
- End-to-end consent lifecycle
- Import/export functionality
- API endpoint validation
- Event publishing

### Performance Tests
- 10K concurrent consent checks
- Bulk import of 1M records
- Cache performance under load
- Database query optimization

### Compliance Tests
- TCPA compliance scenarios
- Consent expiration handling
- Audit trail completeness
- Data retention compliance

## Migration Strategy

### Initial Data Load
1. Export existing consent data (if any)
2. Transform to new schema format
3. Validate data integrity
4. Bulk import with verification
5. Reconciliation report

### Rollout Plan
1. Deploy to staging
2. Import test consent data
3. Shadow mode testing (log only)
4. Gradual rollout (1% → 10% → 50% → 100%)
5. Full enforcement

## Success Metrics

### Week 1 (Emergency)
- ✅ Basic consent checking active
- ✅ Zero unconsented calls processed
- ✅ < 100ms consent check latency
- ✅ Emergency consent data imported

### Week 4 (Full Implementation)
- ✅ Complete consent lifecycle management
- ✅ < 50ms p99 latency achieved
- ✅ 100% audit trail coverage
- ✅ Full API suite available
- ✅ Zero compliance violations

## Risk Mitigation

| Risk | Impact | Mitigation |
|------|--------|------------|
| Missing consent data | High | Amnesty period + re-consent campaign |
| Performance degradation | Medium | Redis caching + read replicas |
| Import failures | High | Validation + rollback capability |
| Cache inconsistency | Low | TTL + invalidation strategy |

## Dependencies

- Redis cluster (existing)
- PostgreSQL with encryption
- Event streaming system (Phase 1)
- Audit logging system (parallel track)

## References

- TCPA Compliance Guide
- FCC Regulations on Consent
- Industry Best Practices
- GDPR Article 7 (Consent)

---

*Specification Version: 1.0*  
*Status: APPROVED FOR IMPLEMENTATION*  
*Last Updated: [Current Date]*