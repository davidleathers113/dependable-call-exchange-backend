# Real-time TCPA Validation Specification

## Overview

**Priority:** CRITICAL (Risk Score: 85/100)  
**Timeline:** Week 1-2 (Integrated with other compliance features)  
**Team:** 2 Senior Engineers  
**Revenue Impact:** Prevents $500-$1,500 per call violations  
**Risk Mitigation:** $3M+ annual violation prevention

## Business Context

### Problem Statement
The platform has NO TCPA validation, exposing every call to:
- Calling time violations ($500-$1,500 per call)
- Frequency violations (harassment claims)
- Prior express consent violations
- State-specific timing restrictions
- No calling window enforcement

### Success Criteria
- 100% of calls validated for TCPA compliance
- < 5ms validation latency
- Support for all state-specific rules
- Real-time timezone detection
- Complete audit trail of decisions
- Zero time-based violations

## Technical Specification

### Domain Model

```go
// internal/domain/compliance/tcpa.go
package compliance

type TCPAValidator struct {
    ID           uuid.UUID
    PhoneNumber  values.PhoneNumber
    Timezone     *time.Location
    State        string
    CallTime     time.Time
    Rules        []TCPARule
    Decision     TCPADecision
}

type TCPARule struct {
    ID          uuid.UUID
    Name        string
    Type        RuleType
    Scope       RuleScope // "federal", "state", "custom"
    Conditions  []RuleCondition
    Priority    int
    EffectiveAt time.Time
    ExpiresAt   *time.Time
}

type RuleType string
const (
    RuleTypeTimeWindow    RuleType = "time_window"
    RuleTypeFrequency     RuleType = "frequency"
    RuleTypeConsentAge    RuleType = "consent_age"
    RuleTypeChannelLimit  RuleType = "channel_limit"
    RuleTypeQuietPeriod   RuleType = "quiet_period"
)

type RuleCondition struct {
    Field    string
    Operator string // "between", "less_than", "equals", etc.
    Value    interface{}
}

type TCPADecision struct {
    Allowed      bool
    Reasons      []string
    AppliedRules []string
    NextAllowed  *time.Time // When calling would be allowed
    Metadata     map[string]interface{}
}

type CallingWindow struct {
    StartTime    time.Time // In recipient's timezone
    EndTime      time.Time
    DaysOfWeek   []time.Weekday
    Holidays     []Holiday
    Restrictions []WindowRestriction
}
```

### Service Layer

```go
// internal/service/tcpa/service.go
package tcpa

type Service interface {
    // Core validation
    ValidateCall(ctx context.Context, req ValidateCallRequest) (*TCPADecision, error)
    GetCallingWindow(ctx context.Context, phoneNumber string, date time.Time) (*CallingWindow, error)
    CheckFrequencyLimits(ctx context.Context, phoneNumber string) (*FrequencyStatus, error)
    
    // Rule management
    CreateRule(ctx context.Context, rule *TCPARule) error
    UpdateRule(ctx context.Context, id uuid.UUID, updates RuleUpdate) error
    GetRules(ctx context.Context, scope RuleScope) ([]*TCPARule, error)
    
    // Analytics
    GetViolationRisk(ctx context.Context, phoneNumber string) (*RiskAssessment, error)
    GetComplianceStats(ctx context.Context, dateRange DateRange) (*TCPAStats, error)
}

type ValidateCallRequest struct {
    PhoneNumber   string
    CallTime      time.Time
    CallType      string // "sales", "service", "survey"
    Channel       string // "voice", "sms", "robocall"
    ConsentDate   *time.Time
    LastCallTime  *time.Time
    CallCount24h  int
    CallCount7d   int
}

type FrequencyStatus struct {
    CallsToday     int
    CallsThisWeek  int
    CallsThisMonth int
    NextAllowed    time.Time
    LimitReached   bool
    LimitType      string
}
```

### Infrastructure Layer

```go
// internal/infrastructure/tcpa/timezone_service.go
package tcpa

type TimezoneService interface {
    GetTimezone(ctx context.Context, phoneNumber string) (*time.Location, error)
    GetState(ctx context.Context, phoneNumber string) (string, error)
    IsHoliday(ctx context.Context, date time.Time, state string) (bool, *Holiday, error)
}

// internal/infrastructure/database/tcpa_repository.go
package database

type TCPARuleRepository interface {
    GetActiveRules(ctx context.Context, scope RuleScope, effectiveAt time.Time) ([]*domain.TCPARule, error)
    GetByID(ctx context.Context, id uuid.UUID) (*domain.TCPARule, error)
    Create(ctx context.Context, rule *domain.TCPARule) error
    Update(ctx context.Context, rule *domain.TCPARule) error
}

type CallHistoryRepository interface {
    GetCallCount(ctx context.Context, phoneNumber string, since time.Time) (int, error)
    GetLastCallTime(ctx context.Context, phoneNumber string) (*time.Time, error)
    RecordCall(ctx context.Context, phoneNumber string, callTime time.Time) error
}
```

### Rule Engine Implementation

```go
// internal/service/tcpa/rule_engine.go
package tcpa

type RuleEngine struct {
    rules []TCPARule
}

func (e *RuleEngine) Evaluate(ctx context.Context, input ValidationInput) (*TCPADecision, error) {
    decision := &TCPADecision{
        Allowed: true,
        Reasons: []string{},
    }
    
    // Sort rules by priority
    sort.Slice(e.rules, func(i, j int) bool {
        return e.rules[i].Priority > e.rules[j].Priority
    })
    
    for _, rule := range e.rules {
        result := e.evaluateRule(rule, input)
        if !result.Passed {
            decision.Allowed = false
            decision.Reasons = append(decision.Reasons, result.Reason)
            decision.AppliedRules = append(decision.AppliedRules, rule.Name)
            
            // Calculate next allowed time
            if nextTime := e.calculateNextAllowed(rule, input); nextTime != nil {
                if decision.NextAllowed == nil || nextTime.Before(*decision.NextAllowed) {
                    decision.NextAllowed = nextTime
                }
            }
        }
    }
    
    return decision, nil
}

func (e *RuleEngine) evaluateTimeWindow(rule TCPARule, input ValidationInput) RuleResult {
    // Get recipient's local time
    localTime := input.CallTime.In(input.Timezone)
    hour := localTime.Hour()
    minute := localTime.Minute()
    
    // Federal TCPA: 8 AM - 9 PM local time
    if rule.Scope == "federal" {
        if hour < 8 || hour >= 21 {
            return RuleResult{
                Passed: false,
                Reason: fmt.Sprintf("Outside federal calling hours (8AM-9PM local time). Current time: %s",
                    localTime.Format("3:04 PM")),
            }
        }
    }
    
    // State-specific rules
    if rule.Scope == "state" {
        // Example: California has stricter rules
        if input.State == "CA" && (hour < 9 || hour >= 20) {
            return RuleResult{
                Passed: false,
                Reason: "Outside California calling hours (9AM-8PM)",
            }
        }
    }
    
    return RuleResult{Passed: true}
}
```

### Database Schema

```sql
-- TCPA rules configuration
CREATE TABLE tcpa_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    type VARCHAR(50) NOT NULL,
    scope VARCHAR(20) NOT NULL,
    conditions JSONB NOT NULL,
    priority INTEGER NOT NULL DEFAULT 0,
    effective_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ,
    created_by VARCHAR(255),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_tcpa_rules_active ON tcpa_rules(scope, effective_at) 
    WHERE expires_at IS NULL OR expires_at > NOW();

-- Call frequency tracking
CREATE TABLE call_frequency (
    phone_number_hash VARCHAR(64) NOT NULL,
    call_date DATE NOT NULL,
    hour INTEGER NOT NULL,
    call_count INTEGER NOT NULL DEFAULT 1,
    last_call_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (phone_number_hash, call_date, hour)
);

CREATE INDEX idx_call_frequency_lookup ON call_frequency(phone_number_hash, call_date);

-- TCPA violations log
CREATE TABLE tcpa_violations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    phone_number_hash VARCHAR(64) NOT NULL,
    violation_type VARCHAR(50) NOT NULL,
    rule_name VARCHAR(255),
    attempted_at TIMESTAMPTZ NOT NULL,
    local_time TIMESTAMPTZ NOT NULL,
    timezone VARCHAR(50),
    state VARCHAR(2),
    details JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_tcpa_violations_phone ON tcpa_violations(phone_number_hash);
CREATE INDEX idx_tcpa_violations_time ON tcpa_violations(attempted_at);

-- Timezone cache
CREATE TABLE timezone_cache (
    phone_prefix VARCHAR(10) PRIMARY KEY,
    timezone VARCHAR(50) NOT NULL,
    state VARCHAR(2),
    confidence FLOAT NOT NULL,
    source VARCHAR(50),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Holiday calendar
CREATE TABLE holiday_calendar (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    date DATE NOT NULL,
    name VARCHAR(255) NOT NULL,
    type VARCHAR(50) NOT NULL, -- "federal", "state", "religious"
    scope VARCHAR(50) NOT NULL, -- "national", "CA", "NY", etc.
    calling_restricted BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_holidays_date ON holiday_calendar(date, scope);
```

## Implementation Plan

### Phase 0: Core TCPA Engine (Week 1)

**Days 1-3: Rule Engine**
- [ ] TCPA domain model
- [ ] Basic time window validation
- [ ] Federal rules implementation
- [ ] Timezone detection service

**Days 4-5: Integration**
- [ ] Integrate with call routing
- [ ] Call frequency tracking
- [ ] Violation logging
- [ ] Deploy with other compliance

### Phase 1: Advanced Features (Week 2)

**State-Specific Rules**
- [ ] California strict timing
- [ ] Texas requirements
- [ ] Florida regulations
- [ ] New York rules

**Enhanced Features**
- [ ] Holiday detection
- [ ] Consent age validation
- [ ] Channel-specific limits
- [ ] Quiet period support

## Default Rule Set

```yaml
Federal Rules:
  - name: "Federal Calling Hours"
    type: "time_window"
    scope: "federal"
    conditions:
      - field: "local_hour"
        operator: "between"
        value: [8, 21]
    priority: 100

  - name: "Federal Frequency Limit"
    type: "frequency"
    scope: "federal"
    conditions:
      - field: "calls_per_day"
        operator: "less_than"
        value: 3
    priority: 90

State Rules:
  - name: "California Calling Hours"
    type: "time_window"
    scope: "state"
    state: "CA"
    conditions:
      - field: "local_hour"
        operator: "between"
        value: [9, 20]
    priority: 110

  - name: "Sunday Restrictions"
    type: "time_window"
    scope: "federal"
    conditions:
      - field: "day_of_week"
        operator: "equals"
        value: "Sunday"
      - field: "local_hour"
        operator: "between"
        value: [12, 21]
    priority: 95
```

## Integration with Call Flow

```go
// Call routing middleware
func TCPAMiddleware(tcpaService tcpa.Service) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            call := getCallFromContext(r.Context())
            
            // Validate TCPA compliance
            decision, err := tcpaService.ValidateCall(r.Context(), tcpa.ValidateCallRequest{
                PhoneNumber: call.ToNumber,
                CallTime:    time.Now(),
                CallType:    call.Type,
                Channel:     "voice",
            })
            
            if err != nil || !decision.Allowed {
                // Log violation attempt
                // Return 403 with next allowed time
                resp := map[string]interface{}{
                    "error": "TCPA_VIOLATION",
                    "reasons": decision.Reasons,
                }
                if decision.NextAllowed != nil {
                    resp["next_allowed"] = decision.NextAllowed.Format(time.RFC3339)
                }
                writeJSON(w, http.StatusForbidden, resp)
                return
            }
            
            next.ServeHTTP(w, r)
        })
    }
}
```

## Performance Optimization

### Caching Strategy
```go
// Cache timezone lookups
type TimezoneCache struct {
    redis *redis.Client
    ttl   time.Duration
}

func (c *TimezoneCache) GetTimezone(ctx context.Context, phoneNumber string) (*time.Location, error) {
    // Use area code + prefix for cache key
    prefix := phoneNumber[:6] // +1NPANXX
    
    cached, err := c.redis.Get(ctx, fmt.Sprintf("tz:%s", prefix)).Result()
    if err == nil {
        return time.LoadLocation(cached)
    }
    
    // Lookup from service
    tz, err := c.lookupTimezone(ctx, phoneNumber)
    if err != nil {
        return nil, err
    }
    
    // Cache for 24 hours
    c.redis.Set(ctx, fmt.Sprintf("tz:%s", prefix), tz.String(), c.ttl)
    
    return tz, nil
}
```

### Pre-computation
```go
// Pre-compute next calling windows
func (s *Service) PrecomputeWindows(ctx context.Context) error {
    // For each timezone, calculate today's windows
    for _, tz := range commonTimezones {
        window := s.calculateWindow(time.Now().In(tz))
        key := fmt.Sprintf("window:%s:%s", tz.String(), time.Now().Format("2006-01-02"))
        s.cache.Set(ctx, key, window, 24*time.Hour)
    }
    return nil
}
```

## Monitoring & Alerting

### Key Metrics
- TCPA validation latency (target: < 5ms)
- Violation attempt rate
- Rules applied per call
- Timezone detection accuracy
- Holiday detection accuracy

### Critical Alerts
- High violation attempt rate
- Rule evaluation failures
- Timezone service errors
- Unusual calling patterns
- Missing holiday data

## Testing Strategy

### Unit Tests
- Time window calculations
- Timezone conversions
- Rule evaluation logic
- Frequency counters

### Integration Tests
- End-to-end validation
- Multi-rule scenarios
- State-specific rules
- Holiday handling

### Compliance Tests
- All state regulations
- Federal requirements
- Edge cases (DST, etc.)
- International considerations

## Success Metrics

### Week 1
- ✅ Federal TCPA rules active
- ✅ < 10ms validation latency
- ✅ Basic timezone detection
- ✅ Violation logging active

### Week 2
- ✅ State rules implemented (top 5)
- ✅ < 5ms p99 latency
- ✅ Holiday calendar integrated
- ✅ Advanced frequency limits
- ✅ 100% compliant calling

## Risk Mitigation

| Risk | Impact | Mitigation |
|------|--------|------------|
| Timezone detection errors | High | Multiple data sources, manual overrides |
| Rule conflicts | Medium | Priority system, clear precedence |
| Performance impact | Medium | Caching, pre-computation |
| Holiday data gaps | Low | Multiple calendars, manual updates |

## Cost Analysis

### Infrastructure
- Timezone API: $100/month
- Additional compute: $50/month
- Total: $150/month

### Development
- 2 engineers × 2 weeks
- Total: ~$20K

### ROI
- Prevent 10 violations/month
- $5K-15K saved/month
- Break-even: < 1 month

## Dependencies

- Timezone detection service
- Holiday calendar data
- Call frequency tracking (shared with DNC)
- Audit logging system

## References

- 47 CFR § 64.1200 - TCPA Rules
- FCC TCPA Compliance Guide
- State Attorney General Guidelines
- Timezone Database (IANA)

---

*Specification Version: 1.0*  
*Status: APPROVED FOR IMPLEMENTATION*  
*Last Updated: [Current Date]*