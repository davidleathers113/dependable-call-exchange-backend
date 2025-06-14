# Immutable Audit Logging Specification

## Overview

**Priority:** CRITICAL (Risk Score: 88/100)  
**Timeline:** Week 1 (Basic), Week 3-4 (Enhanced)  
**Team:** 1 Senior Engineer (Phase 0), 2 Engineers (Phase 1)  
**Revenue Impact:** Required for enterprise contracts ($2M)  
**Risk Mitigation:** Legal defense capability, compliance proof

## Business Context

### Problem Statement
The platform has NO audit trail capability, resulting in:
- Cannot prove compliance during audits or lawsuits
- No evidence for dispute resolution
- Unable to track compliance violations
- Missing chain of custody for consent/DNC checks
- Zero visibility into system decisions

### Success Criteria
- 100% of compliance decisions logged
- Immutable, tamper-proof audit records
- Sub-second query performance
- 7-year retention capability
- Legal admissibility standards met
- Complete reconstruction of any call's compliance journey

## Technical Specification

### Domain Model

```go
// internal/domain/audit/audit_log.go
package audit

type AuditLog struct {
    ID          uuid.UUID
    EventType   EventType
    EntityType  EntityType
    EntityID    string
    Actor       Actor
    Action      Action
    Result      Result
    Context     Context
    Timestamp   time.Time
    Hash        string // SHA-256 of previous entry + current data
    Signature   string // Digital signature for tamper detection
}

type EventType string
const (
    EventTypeComplianceCheck EventType = "compliance_check"
    EventTypeConsentAction   EventType = "consent_action"
    EventTypeDNCCheck       EventType = "dnc_check"
    EventTypeCallAttempt    EventType = "call_attempt"
    EventTypeDataAccess     EventType = "data_access"
    EventTypeConfiguration  EventType = "configuration"
)

type Actor struct {
    Type       ActorType // "system", "user", "api_client"
    ID         string
    IPAddress  string
    UserAgent  string
    SessionID  string
}

type Action struct {
    Type        string
    Description string
    Parameters  map[string]interface{}
}

type Result struct {
    Status      ResultStatus // "success", "failure", "blocked"
    Code        string
    Message     string
    Metadata    map[string]interface{}
}

type Context struct {
    CallID          *uuid.UUID
    PhoneNumber     string // Hashed for PII protection
    ComplianceRules []string
    TraceID         string
    SpanID          string
}
```

### Service Layer

```go
// internal/service/audit/service.go
package audit

type Service interface {
    // Core logging
    LogComplianceCheck(ctx context.Context, check ComplianceCheckEvent) error
    LogConsentAction(ctx context.Context, action ConsentActionEvent) error
    LogDNCCheck(ctx context.Context, check DNCCheckEvent) error
    LogCallAttempt(ctx context.Context, attempt CallAttemptEvent) error
    
    // Query operations
    GetAuditTrail(ctx context.Context, filter AuditFilter) ([]*AuditLog, error)
    GetCallCompliance(ctx context.Context, callID uuid.UUID) (*ComplianceJourney, error)
    VerifyIntegrity(ctx context.Context, startTime, endTime time.Time) (*IntegrityReport, error)
    
    // Export operations
    ExportAuditLogs(ctx context.Context, filter ExportFilter) (io.Reader, error)
    GenerateComplianceReport(ctx context.Context, dateRange DateRange) (*ComplianceReport, error)
}

type ComplianceCheckEvent struct {
    CallID       uuid.UUID
    PhoneNumber  string
    CheckType    string // "consent", "dnc", "tcpa", "state"
    Decision     bool
    Reason       string
    Rules        []string
    Latency      time.Duration
}

type AuditFilter struct {
    StartTime   time.Time
    EndTime     time.Time
    EventTypes  []EventType
    EntityID    string
    ActorID     string
    ResultStatus []ResultStatus
    Limit       int
    Offset      int
}
```

### Infrastructure Layer

```go
// internal/infrastructure/database/audit_repository.go
package database

type AuditRepository interface {
    // Write operations (append-only)
    Insert(ctx context.Context, log *domain.AuditLog) error
    BulkInsert(ctx context.Context, logs []*domain.AuditLog) error
    
    // Read operations
    GetByID(ctx context.Context, id uuid.UUID) (*domain.AuditLog, error)
    Query(ctx context.Context, filter domain.AuditFilter) ([]*domain.AuditLog, error)
    GetHashChain(ctx context.Context, startID, endID uuid.UUID) ([]string, error)
    
    // Never allow updates or deletes
    // Update() - NOT IMPLEMENTED
    // Delete() - NOT IMPLEMENTED
}

// Optimized read store for queries
type AuditIndexRepository interface {
    IndexLog(ctx context.Context, log *domain.AuditLog) error
    SearchByPhone(ctx context.Context, phoneHash string, dateRange DateRange) ([]*domain.AuditLog, error)
    SearchByCall(ctx context.Context, callID uuid.UUID) ([]*domain.AuditLog, error)
    GetStats(ctx context.Context, dateRange DateRange) (*AuditStats, error)
}
```

### Database Schema

```sql
-- Immutable audit log table (append-only)
CREATE TABLE audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_type VARCHAR(50) NOT NULL,
    entity_type VARCHAR(50) NOT NULL,
    entity_id VARCHAR(255) NOT NULL,
    actor_type VARCHAR(50) NOT NULL,
    actor_id VARCHAR(255) NOT NULL,
    actor_ip INET,
    actor_user_agent TEXT,
    actor_session_id VARCHAR(255),
    action_type VARCHAR(100) NOT NULL,
    action_description TEXT,
    action_parameters JSONB,
    result_status VARCHAR(20) NOT NULL,
    result_code VARCHAR(50),
    result_message TEXT,
    result_metadata JSONB,
    context_call_id UUID,
    context_phone_hash VARCHAR(64),
    context_compliance_rules TEXT[],
    context_trace_id VARCHAR(128),
    context_span_id VARCHAR(64),
    timestamp TIMESTAMPTZ NOT NULL,
    hash VARCHAR(64) NOT NULL,
    signature TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
) PARTITION BY RANGE (created_at);

-- Create monthly partitions
CREATE TABLE audit_logs_2024_01 PARTITION OF audit_logs
    FOR VALUES FROM ('2024-01-01') TO ('2024-02-01');

-- Indexes (on partitions)
CREATE INDEX idx_audit_timestamp ON audit_logs (timestamp);
CREATE INDEX idx_audit_entity ON audit_logs (entity_type, entity_id);
CREATE INDEX idx_audit_call_id ON audit_logs (context_call_id) WHERE context_call_id IS NOT NULL;
CREATE INDEX idx_audit_phone_hash ON audit_logs (context_phone_hash) WHERE context_phone_hash IS NOT NULL;
CREATE INDEX idx_audit_event_type ON audit_logs (event_type, timestamp);

-- Read-optimized materialized view for compliance queries
CREATE MATERIALIZED VIEW audit_compliance_summary AS
SELECT 
    date_trunc('hour', timestamp) as hour,
    event_type,
    result_status,
    COUNT(*) as count,
    AVG(CAST(result_metadata->>'latency_ms' AS FLOAT)) as avg_latency_ms
FROM audit_logs
WHERE event_type IN ('compliance_check', 'consent_action', 'dnc_check')
GROUP BY 1, 2, 3;

-- Hash chain verification table
CREATE TABLE audit_hash_chain (
    id SERIAL PRIMARY KEY,
    audit_log_id UUID NOT NULL REFERENCES audit_logs(id),
    previous_hash VARCHAR(64) NOT NULL,
    current_hash VARCHAR(64) NOT NULL,
    verified_at TIMESTAMPTZ,
    verification_status VARCHAR(20)
);

-- Archive table for old logs (after 90 days)
CREATE TABLE audit_logs_archive (
    LIKE audit_logs INCLUDING ALL
) PARTITION BY RANGE (created_at);
```

### Cryptographic Chain Implementation

```go
// Ensure immutability through hash chaining
func (s *Service) createAuditLog(event AuditEvent) (*AuditLog, error) {
    // Get previous log's hash
    previousHash, err := s.repo.GetLatestHash()
    if err != nil {
        return nil, err
    }
    
    log := &AuditLog{
        ID:        uuid.New(),
        EventType: event.Type,
        // ... other fields
        Timestamp: time.Now(),
    }
    
    // Calculate hash including previous hash
    data := fmt.Sprintf("%s|%s|%s|%v",
        previousHash,
        log.ID,
        log.EventType,
        log.Timestamp.Unix(),
    )
    
    hash := sha256.Sum256([]byte(data))
    log.Hash = hex.EncodeToString(hash[:])
    
    // Optional: Digital signature
    if s.signer != nil {
        signature, err := s.signer.Sign(hash[:])
        if err != nil {
            return nil, err
        }
        log.Signature = base64.StdEncoding.EncodeToString(signature)
    }
    
    return log, nil
}
```

## Implementation Plan

### Phase 0: Emergency Implementation (Days 1-2)

**Day 1: Core Infrastructure**
- [ ] Create audit domain model
- [ ] Implement basic audit table
- [ ] Simple append-only repository
- [ ] Basic service implementation

**Day 2: Integration**
- [ ] Integrate with consent checks
- [ ] Integrate with DNC checks
- [ ] Add call routing hooks
- [ ] Deploy to staging

### Phase 1: Enhanced Implementation (Week 3-4)

**Week 3: Advanced Features**
- [ ] Hash chain implementation
- [ ] Digital signatures
- [ ] Partitioning strategy
- [ ] Materialized views
- [ ] Query optimization

**Week 4: Tools & Compliance**
- [ ] Integrity verification tools
- [ ] Export functionality
- [ ] Compliance reports
- [ ] Legal hold support
- [ ] Archive strategy

## Integration Points

### Middleware Integration

```go
// Audit middleware for all API calls
func AuditMiddleware(auditService audit.Service) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            start := time.Now()
            
            // Capture response
            rw := &responseWriter{ResponseWriter: w}
            
            // Process request
            next.ServeHTTP(rw, r)
            
            // Log after completion
            auditService.LogAPICall(r.Context(), audit.APICallEvent{
                Method:     r.Method,
                Path:       r.URL.Path,
                StatusCode: rw.statusCode,
                Latency:    time.Since(start),
                ActorID:    getActorID(r),
                IPAddress:  getClientIP(r),
            })
        })
    }
}
```

### Event Streaming Integration

```go
// Publish audit events for real-time monitoring
func (s *Service) publishAuditEvent(log *AuditLog) error {
    event := &AuditLogCreatedEvent{
        LogID:     log.ID,
        EventType: log.EventType,
        Timestamp: log.Timestamp,
        Actor:     log.Actor,
        Result:    log.Result,
    }
    
    return s.eventBus.Publish("audit.log.created", event)
}
```

## Performance Requirements

- **Write Throughput:** 10,000 logs/second
- **Query Latency:** < 100ms for recent logs
- **Archive Query:** < 1s for historical data
- **Storage Efficiency:** Compression for archived data
- **Retention:** 7 years for compliance

## Query Patterns

### Common Queries

```sql
-- Get complete call compliance journey
SELECT * FROM audit_logs 
WHERE context_call_id = $1 
ORDER BY timestamp;

-- Find all consent actions for a phone number
SELECT * FROM audit_logs
WHERE event_type = 'consent_action'
AND context_phone_hash = $2
ORDER BY timestamp DESC;

-- Compliance summary for date range
SELECT 
    date_trunc('day', timestamp) as day,
    event_type,
    result_status,
    COUNT(*) as count
FROM audit_logs
WHERE timestamp BETWEEN $1 AND $2
GROUP BY 1, 2, 3;

-- Verify hash chain integrity
WITH RECURSIVE chain AS (
    SELECT id, hash, timestamp
    FROM audit_logs
    WHERE id = $1
    
    UNION ALL
    
    SELECT a.id, a.hash, a.timestamp
    FROM audit_logs a
    JOIN chain c ON a.previous_hash = c.hash
    WHERE a.timestamp > c.timestamp
)
SELECT * FROM chain;
```

## Monitoring & Alerting

### Key Metrics
- Write latency (target: < 10ms)
- Query performance by type
- Storage growth rate
- Hash chain verification status
- Failed audit attempts

### Critical Alerts
- Audit write failures
- Hash chain break detected
- Storage > 80% capacity
- Query performance degradation
- Suspicious access patterns

## Security Considerations

### Data Protection
- Phone numbers hashed before storage
- PII fields encrypted
- Access logging for audit logs
- Role-based query permissions
- Secure key management for signatures

### Tamper Prevention
- Append-only design
- Cryptographic hash chain
- Digital signatures
- Restricted database permissions
- Regular integrity checks

## Testing Strategy

### Unit Tests
- Hash calculation accuracy
- Signature verification
- Event serialization
- Query builders

### Integration Tests
- End-to-end audit trail
- Hash chain integrity
- Query performance
- Archive operations

### Compliance Tests
- Legal admissibility standards
- Retention policy compliance
- Data privacy regulations
- Export format validation

### Load Tests
- 10K writes/second sustained
- 1K concurrent queries
- Archive query performance
- Storage optimization

## Compliance & Legal

### Standards Compliance
- SOC 2 Type II requirements
- HIPAA audit requirements
- PCI DSS logging standards
- GDPR Article 30 (records of processing)

### Legal Admissibility
- Timestamp accuracy (NTP sync)
- Hash chain integrity
- Digital signatures
- Chain of custody documentation
- Expert witness preparation

## Archive Strategy

### 90-Day Active Storage
- All logs in primary partitions
- Full indexing
- Fast queries
- Real-time integrity checks

### 90 Days - 1 Year
- Move to archive partitions
- Compressed storage
- Reduced indexes
- Slower queries acceptable

### 1-7 Years
- Cold storage (S3/Glacier)
- Compliance retrieval SLA: 24 hours
- Annual integrity verification
- Legal hold support

## Success Metrics

### Week 1 (Emergency)
- ✅ Basic audit logging active
- ✅ All compliance checks logged
- ✅ 30-day retention working
- ✅ Simple queries functional

### Week 4 (Enhanced)
- ✅ Hash chain implemented
- ✅ < 10ms write latency
- ✅ < 100ms query latency
- ✅ Integrity verification tools
- ✅ Compliance reports automated

## Risk Mitigation

| Risk | Impact | Mitigation |
|------|--------|------------|
| Write failures | Critical | Multiple retry queues, fail-safe logging |
| Storage exhaustion | High | Automated archival, monitoring alerts |
| Query performance | Medium | Materialized views, query optimization |
| Hash chain corruption | High | Regular verification, backup chains |

## Cost Analysis

### Infrastructure
- Primary storage: 10GB/day × $0.10/GB = $30/month
- Archive storage: 300GB/month × $0.023/GB = $207/month
- Total: ~$250/month growing to $2K/month

### Development
- 1 engineer × 2 days (emergency)
- 2 engineers × 2 weeks (enhanced)
- Total: ~$25K

### ROI
- Enables $2M enterprise contracts
- Reduces legal costs by $500K/year
- Compliance certification value: $1M

## Dependencies

- PostgreSQL with partitioning support
- Time-series optimization
- Archive storage solution (S3)
- NTP time synchronization
- Digital signature infrastructure (Phase 1)

## References

- NIST 800-92: Guide to Computer Security Log Management
- PCI DSS v4.0 Requirement 10
- SOC 2 Logging Requirements
- Legal Electronic Evidence Standards

---

*Specification Version: 1.0*  
*Status: APPROVED FOR IMPLEMENTATION*  
*Last Updated: [Current Date]*