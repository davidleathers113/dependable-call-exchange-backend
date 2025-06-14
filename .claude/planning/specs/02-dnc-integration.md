# DNC (Do Not Call) Integration Specification

## Overview

**Priority:** CRITICAL (Risk Score: 90/100)  
**Timeline:** Week 1 (Emergency), Week 3 (Enhanced)  
**Team:** 1 Senior Engineer (Phase 0), 2 Engineers (Phase 1)  
**Revenue Impact:** +$1.5M/year (violation prevention)  
**Risk Mitigation:** $40K+ per violation avoided

## Business Context

### Problem Statement
The platform has NO DNC checking capability, resulting in:
- Federal DNC violations at $40,654 per incident
- State DNC violations with additional penalties
- Immediate shutdown risk from regulators
- 100% of calls potentially violating DNC rules
- No internal suppression capability

### Success Criteria
- 100% of calls checked against DNC before routing
- < 10ms DNC lookup latency
- Daily synchronization with federal DNC
- Support for state DNC lists
- Internal suppression list management
- Complete audit trail of DNC checks

## Technical Specification

### Domain Model

```go
// internal/domain/compliance/dnc.go
package compliance

type DNCEntry struct {
    ID           uuid.UUID
    PhoneNumber  values.PhoneNumber
    ListType     DNCListType
    Source       DNCSource
    AddedAt      time.Time
    ExpiresAt    *time.Time
    Metadata     map[string]interface{}
}

type DNCListType string
const (
    DNCListFederal   DNCListType = "federal"
    DNCListState     DNCListType = "state"
    DNCListInternal  DNCListType = "internal"
    DNCListLitigation DNCListType = "litigation"
    DNCListWireless  DNCListType = "wireless"
)

type DNCSource struct {
    Provider   string // "ftc", "state_ca", "internal", etc.
    FileID     string
    ImportedAt time.Time
    RecordCount int64
}

type DNCCheckResult struct {
    PhoneNumber  string
    IsOnDNC      bool
    Lists        []DNCListType
    CheckedAt    time.Time
    CacheHit     bool
    Latency      time.Duration
}
```

### Service Layer

```go
// internal/service/dnc/service.go
package dnc

type Service interface {
    // Core DNC operations
    CheckNumber(ctx context.Context, phoneNumber string) (*DNCCheckResult, error)
    AddToInternalDNC(ctx context.Context, req AddToDNCRequest) error
    RemoveFromInternalDNC(ctx context.Context, phoneNumber string, reason string) error
    
    // Bulk operations
    ImportDNCList(ctx context.Context, listType DNCListType, reader io.Reader) (*ImportResult, error)
    SyncFederalDNC(ctx context.Context) error
    SyncStateDNC(ctx context.Context, state string) error
    
    // Management operations
    GetDNCStats(ctx context.Context) (*DNCStats, error)
    SearchDNC(ctx context.Context, query string) ([]*DNCEntry, error)
    ExportInternalDNC(ctx context.Context) (io.Reader, error)
}

type AddToDNCRequest struct {
    PhoneNumber string
    Reason      string
    Duration    *time.Duration
    AddedBy     string
    Metadata    map[string]interface{}
}

type DNCStats struct {
    TotalNumbers      int64
    ByList           map[DNCListType]int64
    LastFederalSync  time.Time
    LastStateSync    map[string]time.Time
    CacheHitRate     float64
    AverageLatency   time.Duration
}
```

### Infrastructure Layer

```go
// internal/infrastructure/database/dnc_repository.go
package database

type DNCRepository interface {
    // Write operations (PostgreSQL)
    BulkInsert(ctx context.Context, entries []*domain.DNCEntry) error
    Delete(ctx context.Context, phoneNumber string, listType domain.DNCListType) error
    DeleteExpired(ctx context.Context) (int64, error)
    
    // Read operations (Redis primarily)
    Exists(ctx context.Context, phoneNumber string) (bool, []domain.DNCListType, error)
    Search(ctx context.Context, pattern string, limit int) ([]*domain.DNCEntry, error)
    GetStats(ctx context.Context) (*DNCStats, error)
}

// Redis cache implementation
type DNCCache interface {
    // Bloom filter for quick negative lookups
    InitializeBloomFilter(ctx context.Context, capacity uint, errorRate float64) error
    AddToBloom(ctx context.Context, phoneNumber string) error
    CheckBloom(ctx context.Context, phoneNumber string) (bool, error)
    
    // Hash storage for positive lookups
    Set(ctx context.Context, phoneNumber string, lists []domain.DNCListType) error
    Get(ctx context.Context, phoneNumber string) ([]domain.DNCListType, bool, error)
    
    // Batch operations
    LoadBatch(ctx context.Context, numbers map[string][]domain.DNCListType) error
    Clear(ctx context.Context) error
}
```

### API Endpoints

```yaml
# REST API
GET    /api/v1/dnc/check/{phone}         # Check DNC status
POST   /api/v1/dnc/internal              # Add to internal DNC
DELETE /api/v1/dnc/internal/{phone}      # Remove from internal DNC
POST   /api/v1/dnc/import                # Import DNC list
GET    /api/v1/dnc/stats                 # Get DNC statistics
POST   /api/v1/dnc/sync/federal          # Trigger federal sync
POST   /api/v1/dnc/sync/state/{state}    # Trigger state sync

# Internal endpoints
GET    /internal/dnc/bloom/stats         # Bloom filter statistics
POST   /internal/dnc/cache/warm          # Warm cache from database
```

### Database Schema

```sql
-- DNC entries table (write-heavy, read from cache)
CREATE TABLE dnc_entries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    phone_number VARCHAR(20) NOT NULL,
    phone_hash VARCHAR(64) NOT NULL,
    list_type VARCHAR(20) NOT NULL,
    source_provider VARCHAR(50) NOT NULL,
    source_file_id VARCHAR(255),
    added_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes
CREATE INDEX idx_dnc_phone_hash ON dnc_entries(phone_hash);
CREATE INDEX idx_dnc_phone_list ON dnc_entries(phone_number, list_type);
CREATE INDEX idx_dnc_expires ON dnc_entries(expires_at) WHERE expires_at IS NOT NULL;
CREATE INDEX idx_dnc_list_type ON dnc_entries(list_type);

-- DNC sync history
CREATE TABLE dnc_sync_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    list_type VARCHAR(20) NOT NULL,
    source_provider VARCHAR(50) NOT NULL,
    sync_started_at TIMESTAMPTZ NOT NULL,
    sync_completed_at TIMESTAMPTZ,
    records_added BIGINT,
    records_removed BIGINT,
    status VARCHAR(20) NOT NULL,
    error_message TEXT,
    metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- DNC check audit log
CREATE TABLE dnc_check_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    phone_number_hash VARCHAR(64) NOT NULL,
    is_on_dnc BOOLEAN NOT NULL,
    lists TEXT[],
    cache_hit BOOLEAN NOT NULL,
    latency_ms INTEGER NOT NULL,
    checked_at TIMESTAMPTZ NOT NULL,
    call_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Partitioned by day for efficient cleanup
CREATE INDEX idx_dnc_check_log_created ON dnc_check_log(created_at);
```

## Implementation Plan

### Phase 0: Emergency Implementation (Days 1-3)

**Day 1: Infrastructure Setup**
- [ ] Redis cluster configuration
- [ ] Bloom filter initialization (10M capacity, 0.01% error rate)
- [ ] Basic database schema
- [ ] Federal DNC API integration

**Day 2: Core Service**
- [ ] DNC check service implementation
- [ ] Redis caching layer
- [ ] Manual federal DNC import tool
- [ ] Basic internal suppression

**Day 3: Integration**
- [ ] Call routing middleware
- [ ] Emergency federal list import
- [ ] Basic monitoring
- [ ] Deploy to staging

### Phase 1: Enhanced Implementation (Week 3)

**Enhanced Features:**
- [ ] State DNC integrations (CA, TX, FL priority)
- [ ] Automated daily sync jobs
- [ ] Wireless number detection
- [ ] Litigation scrub lists
- [ ] Advanced suppression rules
- [ ] DNC analytics dashboard

## Integration Points

### Call Routing Integration

```go
// Middleware for call routing
func DNCMiddleware(dncService dnc.Service) func(next http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            call := getCallFromContext(r.Context())
            
            result, err := dncService.CheckNumber(r.Context(), call.ToNumber.String())
            if err != nil {
                // Log error, fail closed (block call)
                return
            }
            
            if result.IsOnDNC {
                // Log DNC violation attempt
                // Return 403 Forbidden
                // Publish violation event
                return
            }
            
            // Add DNC check result to context
            ctx := context.WithValue(r.Context(), "dnc_check", result)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}
```

### Cache Warming Strategy

```go
// Warm cache on startup and periodically
func (s *Service) WarmCache(ctx context.Context) error {
    // 1. Load all DNC numbers from database
    // 2. Initialize bloom filter
    // 3. Load into Redis hash in batches
    // 4. Monitor memory usage
    
    const batchSize = 10000
    offset := 0
    
    for {
        numbers, err := s.repo.GetBatch(ctx, offset, batchSize)
        if err != nil {
            return err
        }
        
        if len(numbers) == 0 {
            break
        }
        
        // Add to bloom filter
        for _, num := range numbers {
            s.cache.AddToBloom(ctx, num.PhoneNumber)
        }
        
        // Add to Redis hash
        s.cache.LoadBatch(ctx, numbers)
        
        offset += batchSize
    }
    
    return nil
}
```

## Performance Optimization

### Bloom Filter Configuration
- **Capacity:** 10M numbers initially
- **Error Rate:** 0.01% false positive
- **Memory Usage:** ~14.4MB
- **Benefit:** Instant negative lookups

### Redis Architecture
```
1. Check bloom filter (1μs)
   ↓ (not in filter = not on DNC)
2. Check Redis hash (1ms)
   ↓ (cache miss)
3. Check PostgreSQL (5ms)
   ↓
4. Update cache
```

### Benchmarks Required
- 100K lookups/second
- < 10ms p99 latency
- 99%+ cache hit rate
- < 100MB memory overhead

## Sync Strategy

### Federal DNC Sync
```yaml
Schedule: Daily at 2 AM EST
Process:
  1. Download delta file from FTC
  2. Parse and validate entries
  3. Bulk insert new numbers
  4. Remove deleted numbers
  5. Update bloom filter
  6. Invalidate affected cache entries
  7. Log sync results
```

### State DNC Sync
```yaml
Schedule: Daily at 3 AM local time
States: CA, TX, FL, NY, PA (priority)
Process:
  1. Connect to state API/SFTP
  2. Download full or delta list
  3. Transform to standard format
  4. Merge with federal data
  5. Update caches
```

## Monitoring & Alerting

### Key Metrics
- DNC check latency (target: < 10ms)
- Cache hit rate (target: > 99%)
- Bloom filter false positive rate
- Sync success rate
- DNC list sizes by type

### Critical Alerts
- DNC check latency > 50ms
- Cache hit rate < 90%
- Sync failure (federal or state)
- Rapid growth in suppression list
- Memory usage > 80%

## Testing Strategy

### Unit Tests
- Bloom filter accuracy
- Cache operations
- Number formatting
- List priority logic

### Integration Tests
- End-to-end DNC check
- Sync job processing
- Cache warming
- API endpoints

### Load Tests
- 100K concurrent lookups
- Cache performance under load
- Bloom filter accuracy at scale
- Memory usage patterns

### Compliance Tests
- Federal DNC compliance
- State-specific rules
- Wireless number handling
- Audit trail completeness

## Migration Strategy

### Initial Load
1. Obtain federal DNC list
2. Validate data format
3. Bulk import (10M+ records)
4. Build bloom filter
5. Warm Redis cache
6. Verify accuracy

### Rollout
1. Shadow mode (log only)
2. 1% enforcement
3. 10% enforcement
4. 50% enforcement
5. 100% enforcement

## Success Metrics

### Day 3 (Emergency)
- ✅ Federal DNC checking active
- ✅ < 50ms lookup latency
- ✅ Basic suppression working
- ✅ Zero DNC violations

### Week 3 (Enhanced)
- ✅ State DNC integrated (5 states)
- ✅ < 10ms p99 latency
- ✅ 99%+ cache hit rate
- ✅ Automated daily syncs
- ✅ Full audit trail

## Risk Mitigation

| Risk | Impact | Mitigation |
|------|--------|------------|
| Sync failures | High | Multiple retry attempts, fallback to last good list |
| Cache corruption | Medium | Automated cache rebuild, health checks |
| Memory exhaustion | Medium | Bloom filter sizing, cache eviction policies |
| API rate limits | Low | Respect limits, implement backoff |

## Cost Analysis

### Infrastructure
- Redis cluster upgrade: $500/month
- Additional storage: $200/month
- Federal DNC subscription: $0 (free)
- State DNC APIs: $500/month total

### Development
- 1 engineer × 3 days (emergency)
- 2 engineers × 1 week (enhanced)
- Total: ~$10K

### ROI
- Avoid 1 violation = $40K saved
- Monthly violations prevented: ~5
- Annual savings: $2.4M

## Dependencies

- Redis cluster (must upgrade)
- Federal DNC access (apply immediately)
- State DNC agreements (legal review)
- Audit logging system (parallel track)

## References

- FTC DNC Registry: https://www.donotcall.gov
- TCPA DNC Requirements
- State DNC Regulations
- Redis Bloom Filter Documentation

---

*Specification Version: 1.0*  
*Status: APPROVED FOR IMPLEMENTATION*  
*Last Updated: [Current Date]*