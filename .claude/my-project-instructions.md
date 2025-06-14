# Project-Specific Instructions for Dependable Call Exchange Backend

This document provides detailed, actionable guidance for working with the DCE codebase. It complements CLAUDE.md with specific patterns, examples, and solutions.

## 🎯 Performance Requirements & Benchmarks

### Critical Performance Paths
```go
// CRITICAL PATH: Call routing decision (target: < 1ms)
// File: internal/service/callrouting/service.go:RouteCall()
// - Avoid database queries in hot path
// - Use pre-loaded buyer profiles
// - Cache routing decisions for 5 seconds

// CRITICAL PATH: Bid evaluation (target: < 5ms)
// File: internal/service/bidding/auction.go:evaluateBids()
// - Process bids in parallel batches of 100
// - Use concurrent map for bid aggregation
// - Pre-allocate slices with capacity hints
```

### Database Query Patterns
```sql
-- ALWAYS use these indexes for call queries
-- idx_calls_created_at_status (created_at DESC, status)
-- idx_calls_buyer_id_status (buyer_id, status, created_at DESC)

-- GOOD: Uses index efficiently
SELECT * FROM calls 
WHERE buyer_id = $1 AND status = 'active' 
ORDER BY created_at DESC LIMIT 100;

-- BAD: Full table scan
SELECT * FROM calls WHERE metadata->>'source' = 'api';
```

### Cache Strategies by Domain
```go
// Account domain: Cache for 5 minutes (rarely changes)
accountCache := cache.New(5*time.Minute, 10*time.Minute)

// Bid domain: Cache for 10 seconds (frequently updates)
bidCache := cache.New(10*time.Second, 20*time.Second)

// Compliance: Cache DNC list for 1 hour (regulatory requirement)
dncCache := cache.New(1*time.Hour, 2*time.Hour)
```

## 🔨 Code Generation Patterns

### Domain Entity Constructor Template
```go
// Template for new domain entities
func New<Entity>(<params>) (*<Entity>, error) {
    // 1. Validate required fields
    if <requiredField> == "" {
        return nil, errors.NewValidationError("<ENTITY>_REQUIRED", 
            "<field> is required")
    }
    
    // 2. Create value objects
    <valueObject>, err := values.New<ValueType>(<param>)
    if err != nil {
        return nil, errors.NewValidationError("<ENTITY>_INVALID_<FIELD>", 
            "invalid <field>").WithCause(err)
    }
    
    // 3. Apply business rules
    if err := validate<BusinessRule>(<params>); err != nil {
        return nil, err
    }
    
    // 4. Initialize with safe defaults
    return &<Entity>{
        id:        uuid.New(),
        <field>:   <valueObject>,
        status:    <Entity>StatusPending,
        createdAt: time.Now().UTC(),
        updatedAt: time.Now().UTC(),
    }, nil
}
```

### Service Interface Pattern
```go
// Always define service interfaces with these sections
type <Domain>Service interface {
    // Command methods (state changes)
    Create<Entity>(ctx context.Context, <params>) (*<Entity>, error)
    Update<Entity>(ctx context.Context, id uuid.UUID, <params>) error
    
    // Query methods (read-only)
    Get<Entity>(ctx context.Context, id uuid.UUID) (*<Entity>, error)
    List<Entities>(ctx context.Context, filter <Filter>) ([]*<Entity>, error)
    
    // Business operations
    <BusinessOperation>(ctx context.Context, <params>) error
}

// Implementation must have exactly these fields
type <domain>Service struct {
    repo       <Domain>Repository  // Required
    logger     *slog.Logger       // Required
    metrics    *Metrics           // Required
    <dep3>     Interface3         // Optional (max 5 total)
    <dep4>     Interface4         // Optional (max 5 total)
}
```

### Repository Method Pattern
```go
// Repository methods follow this structure
func (r *<entity>Repository) Create(ctx context.Context, entity *domain.<Entity>) error {
    // 1. Start span for tracing
    ctx, span := r.tracer.Start(ctx, "repository.<entity>.create")
    defer span.End()
    
    // 2. Prepare SQL with named parameters
    query := `
        INSERT INTO <entities> (
            id, field1, field2, status, created_at, updated_at
        ) VALUES (
            :id, :field1, :field2, :status, :created_at, :updated_at
        )`
    
    // 3. Use value object adapters
    args := pgx.NamedArgs{
        "id":         entity.ID(),
        "field1":     adapters.PhoneNumberToDB(entity.Field1()),
        "field2":     adapters.MoneyToDB(entity.Field2()),
        "status":     entity.Status().String(),
        "created_at": entity.CreatedAt(),
        "updated_at": entity.UpdatedAt(),
    }
    
    // 4. Execute with proper error handling
    if _, err := r.db.ExecContext(ctx, query, args); err != nil {
        span.RecordError(err)
        return errors.NewInternalError("failed to create <entity>").
            WithCause(err).
            WithMetadata("entity_id", entity.ID())
    }
    
    return nil
}
```

### Test Fixture Builder Pattern
```go
// Builder for test data
type <Entity>Builder struct {
    entity *domain.<Entity>
}

func New<Entity>Builder() *<Entity>Builder {
    return &<Entity>Builder{
        entity: &domain.<Entity>{
            // Set sensible defaults
            id:     uuid.New(),
            status: domain.<Entity>StatusActive,
        },
    }
}

func (b *<Entity>Builder) With<Field>(value <Type>) *<Entity>Builder {
    b.entity.<field> = value
    return b
}

func (b *<Entity>Builder) Build() *domain.<Entity> {
    // Apply any final validation or computed fields
    b.entity.updatedAt = time.Now().UTC()
    return b.entity
}
```

## ⚠️ Common Pitfalls & Solutions

### 1. Foreign Key Constraint Order
```go
// WRONG: Will fail with foreign key violation
call := fixtures.NewCall().WithBuyerID(uuid.New()).Build()
db.Create(call) // Error: buyer doesn't exist

// CORRECT: Create in dependency order
buyer := fixtures.NewAccount().WithType(AccountTypeBuyer).Build()
db.Create(buyer)

call := fixtures.NewCall().WithBuyerID(buyer.ID).Build()
db.Create(call)

// For bulk operations, use this order:
// 1. Accounts (buyers/sellers)
// 2. Compliance profiles
// 3. Calls
// 4. Bids
// 5. Transactions
```

### 2. Enum String Conversions
```go
// WRONG: Direct insertion fails
db.Exec("INSERT INTO calls (status) VALUES (?)", call.StatusActive)

// CORRECT: Use String() method
db.Exec("INSERT INTO calls (status) VALUES (?)", call.StatusActive.String())

// For scanning from DB
var statusStr string
err := rows.Scan(&statusStr)
status := domain.CallStatusFromString(statusStr) // Use converter function
```

### 3. Transaction Boundaries
```go
// WRONG: Multiple transactions
func (s *service) ProcessCall(ctx context.Context, call *Call) error {
    s.repo.CreateCall(ctx, call)        // Transaction 1
    s.repo.CreateBid(ctx, bid)          // Transaction 2 - could fail
    s.repo.UpdateAccount(ctx, account)  // Transaction 3 - inconsistent state
}

// CORRECT: Single transaction
func (s *service) ProcessCall(ctx context.Context, call *Call) error {
    return s.repo.WithTransaction(ctx, func(tx Repository) error {
        if err := tx.CreateCall(ctx, call); err != nil {
            return err // Automatic rollback
        }
        if err := tx.CreateBid(ctx, bid); err != nil {
            return err // Automatic rollback
        }
        return tx.UpdateAccount(ctx, account)
    })
}
```

### 4. Concurrent Test Isolation
```go
// WRONG: Tests share state
var sharedCache = cache.New()

func TestFeature1(t *testing.T) {
    sharedCache.Set("key", "value1") // Affects other tests
}

// CORRECT: Isolated instances
func TestFeature1(t *testing.T) {
    cache := cache.New() // Local instance
    cache.Set("key", "value1")
}

// For database tests, use separate schemas
func TestWithDB(t *testing.T) {
    db := testutil.SetupTestDB(t) // Creates isolated schema
    defer testutil.TeardownTestDB(t, db)
}
```

### 5. Mock Service Creation
```go
// WRONG: Nil pointer panics
mock := &MockService{}
mock.On("Method").Return(nil) // Panics if mock not initialized

// CORRECT: Use testify properly
mock := new(MockService) // or &MockService{}
mock.On("Method", mock.Anything).Return(nil, nil)
defer mock.AssertExpectations(t)
```

## 🔍 Debugging Strategies

### Diagnose Compilation Errors
```bash
# ALWAYS use this to see ALL errors (not just first 10)
go build -gcflags="-e" ./...

# For specific package debugging
go build -gcflags="-e -N -l" ./internal/service/bidding

# With race detection
go build -race -gcflags="-e" ./...
```

### Tracing Slow Queries
```go
// Add query timing to context
ctx = database.WithQueryTiming(ctx, true)

// Check slow query log
tail -f logs/slow-queries.log | jq '. | select(.duration_ms > 100)'

// Enable PostgreSQL slow query logging
ALTER SYSTEM SET log_min_duration_statement = 100; -- Log queries > 100ms
```

### Debugging Concurrent Tests
```bash
# Run with synctest for deterministic execution
GOEXPERIMENT=synctest go test -race ./internal/service/callrouting

# Add debugging to specific test
go test -v -run TestConcurrentBidding -race -count=100

# Use delve for step debugging
dlv test ./internal/service/bidding -- -test.run TestAuction
```

### Performance Profiling Commands
```bash
# CPU profiling
go test -cpuprofile=cpu.prof -bench=. ./internal/service/callrouting
go tool pprof -http=:8080 cpu.prof

# Memory profiling  
go test -memprofile=mem.prof -bench=. ./internal/service/bidding
go tool pprof -http=:8080 mem.prof

# Trace execution
go test -trace=trace.out ./...
go tool trace trace.out
```

### Memory Leak Detection
```go
// Add to suspicious functions
defer func() {
    var m runtime.MemStats
    runtime.ReadMemStats(&m)
    if m.Alloc > 100*1024*1024 { // 100MB threshold
        panic(fmt.Sprintf("Potential memory leak: %d MB allocated", m.Alloc/1024/1024))
    }
}()
```

## 🧪 Testing Requirements

### Minimum Coverage Thresholds
- Domain entities: 95% coverage required
- Services: 85% coverage required  
- Repositories: 80% coverage required
- API handlers: 75% coverage required
- Infrastructure: 70% coverage required

### Required Test Types by Component
```yaml
Domain Entity:
  - Unit tests (constructor validation)
  - Property tests (invariants)
  - Example: internal/domain/call/call_test.go

Service:
  - Unit tests (mock dependencies)
  - Integration tests (real dependencies)
  - Example: internal/service/bidding/service_test.go

Repository:
  - Integration tests (real database)
  - Query performance tests
  - Example: internal/infrastructure/repository/call_repository_test.go

API Handler:
  - Unit tests (mock services)
  - Contract tests (OpenAPI validation)
  - Example: internal/api/rest/handlers_test.go
```

### Property Test Scenarios
```go
// Required property tests for each domain
func TestCallInvariants(t *testing.T) {
    quick.Check(func(from, to string) bool {
        call, err := domain.NewCall(from, to)
        if err != nil {
            return true // Invalid input, skip
        }
        
        // Invariant 1: Duration is never negative
        if call.Duration() < 0 {
            return false
        }
        
        // Invariant 2: Status transitions are valid
        if !call.CanTransitionTo(domain.CallStatusCompleted) {
            return false
        }
        
        return true
    }, nil)
}
```

### E2E Test Data Setup Order
```go
// Always follow this sequence
func setupE2ETestData(t *testing.T, db *sql.DB) {
    // 1. System configuration
    setupSystemConfig(t, db)
    
    // 2. Accounts (buyers and sellers)
    buyer := createTestBuyer(t, db)
    seller := createTestSeller(t, db)
    
    // 3. Compliance profiles
    createComplianceProfile(t, db, buyer.ID)
    
    // 4. Calls (requires valid buyer/seller)
    call := createTestCall(t, db, buyer.ID, seller.ID)
    
    // 5. Bids (requires valid call and account)
    createTestBid(t, db, call.ID, buyer.ID)
    
    // 6. Financial records (after bids)
    createTestTransaction(t, db, call.ID)
}
```

### Contract Test Requirements
```go
// Every API endpoint must have contract tests
func TestCreateCallContract(t *testing.T) {
    // 1. Load OpenAPI spec
    spec := loadOpenAPISpec(t)
    
    // 2. Validate request format
    req := httptest.NewRequest("POST", "/api/v1/calls", body)
    validateRequest(t, spec, req)
    
    // 3. Execute handler
    resp := executeHandler(req)
    
    // 4. Validate response format
    validateResponse(t, spec, resp)
    
    // 5. Verify performance (< 50ms)
    assert.Less(t, resp.Duration, 50*time.Millisecond)
}
```

## 🔌 Integration Points

### Redis Connection Patterns
```go
// Connection pooling configuration
redisClient := redis.NewClient(&redis.Options{
    Addr:         getEnv("REDIS_URL", "localhost:6379"),
    Password:     getEnv("REDIS_PASSWORD", ""),
    DB:           0,
    PoolSize:     100,              // Match expected concurrent operations
    MinIdleConns: 10,               // Maintain warm connections
    MaxRetries:   3,                // Retry failed operations
    DialTimeout:  5 * time.Second,  // Connection timeout
    ReadTimeout:  3 * time.Second,  // Operation timeout
    WriteTimeout: 3 * time.Second,
})

// Circuit breaker pattern
breaker := gobreaker.NewCircuitBreaker(gobreaker.Settings{
    Name:        "redis",
    MaxRequests: 100,
    Interval:    10 * time.Second,
    Timeout:     60 * time.Second,
    ReadyToTrip: func(counts gobreaker.Counts) bool {
        failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
        return counts.Requests >= 10 && failureRatio >= 0.5
    },
})

// Use with circuit breaker
result, err := breaker.Execute(func() (interface{}, error) {
    return redisClient.Get(ctx, key).Result()
})
```

### PostgreSQL Adapter Usage
```go
// Value object adapters for database operations
// File: internal/infrastructure/database/adapters/

// Phone number adapter
func PhoneNumberToDB(phone values.PhoneNumber) string {
    return phone.E164() // Always store in E.164 format
}

func PhoneNumberFromDB(e164 string) (values.PhoneNumber, error) {
    return values.NewPhoneNumber(e164)
}

// Money adapter (store as cents)
func MoneyToDB(money values.Money) map[string]interface{} {
    return map[string]interface{}{
        "amount":   money.Cents(),
        "currency": money.Currency().String(),
    }
}

// Quality metrics adapter (JSONB)
func QualityMetricsToDB(metrics values.QualityMetrics) ([]byte, error) {
    return json.Marshal(metrics)
}
```

### OpenTelemetry Instrumentation
```go
// Standard span creation pattern
func (s *service) ProcessCall(ctx context.Context, call *Call) error {
    ctx, span := s.tracer.Start(ctx, "service.call.process",
        trace.WithAttributes(
            attribute.String("call.id", call.ID.String()),
            attribute.String("call.status", call.Status.String()),
        ))
    defer span.End()
    
    // Record important events
    span.AddEvent("processing_started")
    
    // Handle errors properly
    if err := s.validateCall(ctx, call); err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, "validation failed")
        return err
    }
    
    span.SetStatus(codes.Ok, "processed successfully")
    return nil
}
```

### WebSocket Event Handling
```go
// Event types and handlers
type EventType string

const (
    EventCallStarted   EventType = "call.started"
    EventCallCompleted EventType = "call.completed"
    EventBidReceived   EventType = "bid.received"
    EventBidAccepted   EventType = "bid.accepted"
)

// Broadcasting pattern
func (h *wsHandler) broadcastEvent(event Event) {
    h.clientsMu.RLock()
    clients := make([]*Client, 0, len(h.clients))
    for _, client := range h.clients {
        if client.CanReceive(event.Type) {
            clients = append(clients, client)
        }
    }
    h.clientsMu.RUnlock()
    
    // Send in parallel with timeout
    var wg sync.WaitGroup
    for _, client := range clients {
        wg.Add(1)
        go func(c *Client) {
            defer wg.Done()
            
            ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
            defer cancel()
            
            if err := c.SendWithContext(ctx, event); err != nil {
                h.logger.Error("failed to send event", 
                    "client_id", c.ID,
                    "error", err)
                h.removeClient(c)
            }
        }(client)
    }
    wg.Wait()
}
```

## 🔒 Security & Compliance Checklist

### TCPA Validation Requirements
```go
// Required checks for every call
func (s *complianceService) ValidateTCPA(ctx context.Context, call *Call) error {
    // 1. Check calling hours (8 AM - 9 PM recipient's time zone)
    recipientTZ := s.getTimeZone(call.ToNumber)
    localTime := time.Now().In(recipientTZ)
    hour := localTime.Hour()
    
    if hour < 8 || hour >= 21 {
        return errors.NewComplianceError("TCPA_HOURS", 
            "calls prohibited outside 8 AM - 9 PM recipient time")
    }
    
    // 2. Check consent status
    consent, err := s.repo.GetConsent(ctx, call.ToNumber)
    if err != nil || !consent.IsValid() {
        return errors.NewComplianceError("TCPA_CONSENT", 
            "valid consent required")
    }
    
    // 3. Check call frequency (max 3 calls per day)
    count, err := s.repo.CountCallsToday(ctx, call.ToNumber)
    if err != nil {
        return err
    }
    if count >= 3 {
        return errors.NewComplianceError("TCPA_FREQUENCY", 
            "exceeded daily call limit")
    }
    
    return nil
}
```

### DNC List Checking
```go
// Real-time DNC validation with caching
func (s *complianceService) CheckDNC(ctx context.Context, phoneNumber string) error {
    // 1. Check cache first
    if cached, found := s.dncCache.Get(phoneNumber); found {
        if cached.(bool) {
            return errors.NewComplianceError("DNC_LISTED", 
                "number is on Do Not Call list")
        }
        return nil
    }
    
    // 2. Check internal DNC
    if listed, err := s.repo.IsInternalDNC(ctx, phoneNumber); err != nil {
        return err
    } else if listed {
        s.dncCache.Set(phoneNumber, true, 1*time.Hour)
        return errors.NewComplianceError("DNC_LISTED", 
            "number is on internal DNC list")
    }
    
    // 3. Check national DNC (with circuit breaker)
    result, err := s.dncBreaker.Execute(func() (interface{}, error) {
        return s.checkNationalDNC(ctx, phoneNumber)
    })
    if err != nil {
        // Log but don't fail on external service errors
        s.logger.Error("national DNC check failed", "error", err)
        // Fail open - allow call but flag for review
        s.flagForReview(ctx, phoneNumber, "dnc_check_failed")
        return nil
    }
    
    listed := result.(bool)
    s.dncCache.Set(phoneNumber, listed, 1*time.Hour)
    
    if listed {
        return errors.NewComplianceError("DNC_LISTED", 
            "number is on national DNC list")
    }
    
    return nil
}
```

### GDPR Data Handling
```go
// Personal data must be handled with care
type PersonalData struct {
    // Tag fields for automatic handling
    PhoneNumber string `gdpr:"pii,mask"`
    Email       string `gdpr:"pii,hash"`
    Name        string `gdpr:"pii,encrypt"`
    
    // Non-PII fields
    AccountID   uuid.UUID `gdpr:"metadata"`
    CallCount   int       `gdpr:"metadata"`
}

// Data retention policy
func (s *gdprService) ApplyRetentionPolicy(ctx context.Context) error {
    // Delete call recordings after 90 days
    if err := s.repo.DeleteOldRecordings(ctx, 90*24*time.Hour); err != nil {
        return err
    }
    
    // Anonymize completed calls after 180 days
    if err := s.repo.AnonymizeOldCalls(ctx, 180*24*time.Hour); err != nil {
        return err
    }
    
    // Archive financial records after 7 years (legal requirement)
    if err := s.repo.ArchiveOldTransactions(ctx, 7*365*24*time.Hour); err != nil {
        return err
    }
    
    return nil
}

// Right to erasure (GDPR Article 17)
func (s *gdprService) DeleteUserData(ctx context.Context, userID uuid.UUID) error {
    return s.repo.WithTransaction(ctx, func(tx Repository) error {
        // 1. Delete personal data
        if err := tx.DeleteUserPersonalData(ctx, userID); err != nil {
            return err
        }
        
        // 2. Anonymize historical records
        if err := tx.AnonymizeUserRecords(ctx, userID); err != nil {
            return err
        }
        
        // 3. Audit log the deletion
        return tx.LogDataDeletion(ctx, userID, "user_requested")
    })
}
```

### API Authentication Patterns
```go
// JWT validation middleware
func (h *Handlers) authenticateRequest(next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // 1. Extract token
        token := extractBearerToken(r)
        if token == "" {
            h.writeError(w, http.StatusUnauthorized, 
                errors.NewAuthError("missing token"))
            return
        }
        
        // 2. Validate token
        claims, err := h.authService.ValidateToken(r.Context(), token)
        if err != nil {
            h.writeError(w, http.StatusUnauthorized, 
                errors.NewAuthError("invalid token"))
            return
        }
        
        // 3. Check permissions
        if !h.authService.HasPermission(claims, r.URL.Path, r.Method) {
            h.writeError(w, http.StatusForbidden, 
                errors.NewAuthError("insufficient permissions"))
            return
        }
        
        // 4. Add claims to context
        ctx := context.WithValue(r.Context(), "claims", claims)
        next.ServeHTTP(w, r.WithContext(ctx))
    }
}

// API key validation for service-to-service
func (h *Handlers) validateAPIKey(next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        apiKey := r.Header.Get("X-API-Key")
        
        // Validate against hashed keys in database
        service, err := h.authService.ValidateAPIKey(r.Context(), apiKey)
        if err != nil {
            h.writeError(w, http.StatusUnauthorized, 
                errors.NewAuthError("invalid API key"))
            return
        }
        
        // Add service identity to context
        ctx := context.WithValue(r.Context(), "service", service)
        next.ServeHTTP(w, r.WithContext(ctx))
    }
}
```

### Rate Limiting Configurations
```go
// Different rate limits by endpoint and client type
var rateLimits = map[string]RateLimit{
    // Public endpoints
    "POST /api/v1/calls":      {Rate: 100, Burst: 200, Per: time.Minute},
    "GET /api/v1/calls":       {Rate: 1000, Burst: 2000, Per: time.Minute},
    
    // Authenticated endpoints  
    "POST /api/v1/bids":       {Rate: 1000, Burst: 5000, Per: time.Minute},
    "GET /api/v1/analytics":   {Rate: 60, Burst: 120, Per: time.Minute},
    
    // Admin endpoints
    "POST /api/v1/admin/*":    {Rate: 10, Burst: 20, Per: time.Minute},
    
    // WebSocket connections
    "WS /api/v1/events":       {Rate: 10, Burst: 10, Per: time.Hour},
}

// Apply rate limiting with client identification
func (h *Handlers) rateLimitMiddleware(next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // Identify client
        clientID := h.identifyClient(r)
        endpoint := fmt.Sprintf("%s %s", r.Method, r.URL.Path)
        
        // Get appropriate limit
        limit, exists := rateLimits[endpoint]
        if !exists {
            limit = defaultRateLimit
        }
        
        // Check limit
        limiter := h.getLimiter(clientID, endpoint, limit)
        if !limiter.Allow() {
            h.writeError(w, http.StatusTooManyRequests,
                errors.NewRateLimitError("rate limit exceeded"))
            return
        }
        
        next.ServeHTTP(w, r)
    }
}
```

## 📈 Performance Optimization Guide

### Query Optimization Techniques
```sql
-- Use EXPLAIN ANALYZE to verify index usage
EXPLAIN (ANALYZE, BUFFERS) 
SELECT c.*, b.* 
FROM calls c
LEFT JOIN bids b ON b.call_id = c.id
WHERE c.status = 'active' AND c.created_at > NOW() - INTERVAL '1 hour';

-- Create covering indexes for hot queries
CREATE INDEX idx_calls_status_created_covering 
ON calls(status, created_at) 
INCLUDE (buyer_id, seller_id, duration);

-- Use partial indexes for common filters
CREATE INDEX idx_active_calls 
ON calls(created_at DESC) 
WHERE status = 'active';

-- Optimize JSONB queries with GIN indexes
CREATE INDEX idx_calls_metadata 
ON calls USING gin(metadata);
```

### Caching Strategies by Domain
```go
// Account caching (low churn, high read)
type AccountCache struct {
    cache *ristretto.Cache
}

func NewAccountCache() *AccountCache {
    cache, _ := ristretto.NewCache(&ristretto.Config{
        NumCounters: 1e7,     // 10 million
        MaxCost:     1 << 30, // 1GB
        BufferItems: 64,
        Cost: func(value interface{}) int64 {
            return int64(unsafe.Sizeof(value))
        },
    })
    return &AccountCache{cache: cache}
}

// Bid caching (high churn, short TTL)
type BidCache struct {
    cache *ttlcache.Cache
}

func NewBidCache() *BidCache {
    cache := ttlcache.New(
        ttlcache.WithTTL[string, *Bid](10 * time.Second),
        ttlcache.WithCapacity[string, *Bid](10000),
    )
    go cache.Start() // Start expiration routine
    return &BidCache{cache: cache}
}

// Warm cache on startup
func (s *service) WarmCache(ctx context.Context) error {
    // Load frequently accessed data
    accounts, err := s.repo.GetActiveAccounts(ctx, 1000)
    if err != nil {
        return err
    }
    
    for _, account := range accounts {
        s.accountCache.Set(account.ID.String(), account)
    }
    
    return nil
}
```

### Connection Pooling Settings
```go
// PostgreSQL optimal settings
func NewDBPool(ctx context.Context, connString string) (*pgxpool.Pool, error) {
    config, err := pgxpool.ParseConfig(connString)
    if err != nil {
        return nil, err
    }
    
    // Connection pool settings
    config.MaxConns = 100                     // Max connections
    config.MinConns = 10                      // Keep warm
    config.MaxConnLifetime = 1 * time.Hour    // Prevent stale connections
    config.MaxConnIdleTime = 30 * time.Minute // Release idle connections
    config.HealthCheckPeriod = 1 * time.Minute // Health check frequency
    
    // Connection-level settings
    config.ConnConfig.ConnectTimeout = 5 * time.Second
    config.ConnConfig.RuntimeParams["application_name"] = "dce-backend"
    config.ConnConfig.RuntimeParams["statement_timeout"] = "30s"
    config.ConnConfig.RuntimeParams["lock_timeout"] = "10s"
    
    return pgxpool.NewWithConfig(ctx, config)
}

// Redis connection pooling
func NewRedisClient() *redis.Client {
    return redis.NewClient(&redis.Options{
        Addr:         "localhost:6379",
        PoolSize:     50 * runtime.GOMAXPROCS(0), // Scale with CPUs
        MinIdleConns: 10,
        MaxRetries:   3,
        DialTimeout:  5 * time.Second,
        ReadTimeout:  3 * time.Second,
        WriteTimeout: 3 * time.Second,
        PoolTimeout:  4 * time.Second,
        
        // Connection pool optimization
        PoolFIFO:    true, // Use FIFO for better load distribution
        MaxConnAge:  0,    // No maximum age
        IdleTimeout: 5 * time.Minute,
    })
}
```

### Batch Processing Patterns
```go
// Batch bid processing
func (s *biddingService) ProcessBidsBatch(ctx context.Context, bids []*Bid) error {
    const batchSize = 100
    
    // Process in parallel batches
    g, ctx := errgroup.WithContext(ctx)
    g.SetLimit(10) // Max 10 concurrent batches
    
    for i := 0; i < len(bids); i += batchSize {
        end := i + batchSize
        if end > len(bids) {
            end = len(bids)
        }
        
        batch := bids[i:end]
        g.Go(func() error {
            return s.processBatch(ctx, batch)
        })
    }
    
    return g.Wait()
}

// Efficient batch insertion
func (r *repository) CreateBidsBatch(ctx context.Context, bids []*Bid) error {
    // Use COPY for bulk insert
    columns := []string{"id", "call_id", "buyer_id", "amount", "created_at"}
    
    rows := make([][]interface{}, len(bids))
    for i, bid := range bids {
        rows[i] = []interface{}{
            bid.ID,
            bid.CallID,
            bid.BuyerID,
            bid.Amount.Cents(),
            bid.CreatedAt,
        }
    }
    
    _, err := r.db.CopyFrom(ctx, pgx.Identifier{"bids"}, columns, 
        pgx.CopyFromRows(rows))
    return err
}
```

### Concurrent Processing Limits
```go
// Semaphore for limiting concurrent operations
type ConcurrencyLimiter struct {
    sem chan struct{}
}

func NewConcurrencyLimiter(limit int) *ConcurrencyLimiter {
    return &ConcurrencyLimiter{
        sem: make(chan struct{}, limit),
    }
}

func (c *ConcurrencyLimiter) Acquire() {
    c.sem <- struct{}{}
}

func (c *ConcurrencyLimiter) Release() {
    <-c.sem
}

// Usage in service
func (s *service) ProcessCalls(ctx context.Context, calls []*Call) error {
    limiter := NewConcurrencyLimiter(50) // Max 50 concurrent
    
    var wg sync.WaitGroup
    errCh := make(chan error, len(calls))
    
    for _, call := range calls {
        wg.Add(1)
        go func(c *Call) {
            defer wg.Done()
            
            limiter.Acquire()
            defer limiter.Release()
            
            if err := s.processCall(ctx, c); err != nil {
                errCh <- err
            }
        }(call)
    }
    
    wg.Wait()
    close(errCh)
    
    // Collect any errors
    for err := range errCh {
        if err != nil {
            return err
        }
    }
    
    return nil
}
```

## 🚨 Emergency Procedures

### How to Rollback Migrations
```bash
# Check current migration version
go run cmd/migrate/main.go -action version

# Rollback last migration
go run cmd/migrate/main.go -action down -steps 1

# Rollback to specific version
go run cmd/migrate/main.go -action down -version 20240615120000

# Emergency rollback script
#!/bin/bash
# Save as scripts/emergency-rollback.sh

BACKUP_FILE="/backups/db-$(date +%Y%m%d-%H%M%S).sql"

# 1. Backup current state
pg_dump $DATABASE_URL > $BACKUP_FILE

# 2. Rollback migrations
go run cmd/migrate/main.go -action down -steps $1

# 3. Restart services
systemctl restart dce-api
systemctl restart dce-worker
```

### Circuit Breaker Patterns
```go
// Circuit breaker for external services
type ServiceBreaker struct {
    breaker *gobreaker.CircuitBreaker
}

func NewServiceBreaker(name string) *ServiceBreaker {
    settings := gobreaker.Settings{
        Name:        name,
        MaxRequests: 100,                      // Requests allowed in half-open
        Interval:    10 * time.Second,         // Reset counters interval
        Timeout:     60 * time.Second,         // Time before trying half-open
        ReadyToTrip: func(counts gobreaker.Counts) bool {
            failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
            return counts.Requests >= 10 && failureRatio >= 0.6
        },
        OnStateChange: func(name string, from, to gobreaker.State) {
            logger.Warn("circuit breaker state change",
                "service", name,
                "from", from.String(),
                "to", to.String())
            
            // Alert on circuit open
            if to == gobreaker.StateOpen {
                alerting.SendAlert(AlertPriorityHigh, 
                    fmt.Sprintf("Circuit breaker OPEN for %s", name))
            }
        },
    }
    
    return &ServiceBreaker{
        breaker: gobreaker.NewCircuitBreaker(settings),
    }
}

// Usage with fallback
func (s *service) CallExternalAPI(ctx context.Context, req Request) (*Response, error) {
    result, err := s.breaker.Execute(func() (interface{}, error) {
        return s.client.Call(ctx, req)
    })
    
    if err != nil {
        // Check if circuit is open
        if err == gobreaker.ErrOpenState {
            // Use fallback
            return s.fallbackResponse(req), nil
        }
        return nil, err
    }
    
    return result.(*Response), nil
}
```

### Graceful Degradation
```go
// Feature flags for degradation
type FeatureFlags struct {
    // Degradation flags
    DisableComplexRouting   bool `env:"DISABLE_COMPLEX_ROUTING"`
    DisableRealTimeBidding  bool `env:"DISABLE_REALTIME_BIDDING"`
    DisableFraudChecking    bool `env:"DISABLE_FRAUD_CHECKING"`
    UseSimplifiedAnalytics  bool `env:"USE_SIMPLIFIED_ANALYTICS"`
}

// Degraded routing algorithm
func (s *routingService) RouteCall(ctx context.Context, call *Call) (*Buyer, error) {
    if s.flags.DisableComplexRouting {
        // Fall back to round-robin
        return s.simpleRoundRobin(ctx, call)
    }
    
    // Normal complex routing
    return s.complexRouting(ctx, call)
}

// Health check with degradation status
func (h *Handlers) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
    status := HealthStatus{
        Status: "healthy",
        Services: map[string]ServiceStatus{},
    }
    
    // Check each service
    if h.flags.DisableComplexRouting {
        status.Services["routing"] = ServiceStatus{
            Status: "degraded",
            Mode:   "simple",
        }
    }
    
    if h.flags.DisableRealTimeBidding {
        status.Services["bidding"] = ServiceStatus{
            Status: "degraded", 
            Mode:   "batch",
        }
    }
    
    // Set appropriate status code
    code := http.StatusOK
    if len(status.Services) > 0 {
        code = http.StatusPartialContent
        status.Status = "degraded"
    }
    
    h.writeJSON(w, code, status)
}
```

### Error Recovery Strategies
```go
// Automatic retry with backoff
func (s *service) ProcessWithRetry(ctx context.Context, fn func() error) error {
    backoff := []time.Duration{
        100 * time.Millisecond,
        500 * time.Millisecond,
        2 * time.Second,
        5 * time.Second,
    }
    
    var lastErr error
    for i, delay := range backoff {
        if err := fn(); err != nil {
            lastErr = err
            
            // Check if retryable
            if !isRetryable(err) {
                return err
            }
            
            s.logger.Warn("operation failed, retrying",
                "attempt", i+1,
                "delay", delay,
                "error", err)
            
            select {
            case <-time.After(delay):
                continue
            case <-ctx.Done():
                return ctx.Err()
            }
        }
        
        return nil // Success
    }
    
    return fmt.Errorf("failed after %d retries: %w", len(backoff), lastErr)
}

// Dead letter queue for failed operations
type DeadLetterQueue struct {
    db *sql.DB
}

func (dlq *DeadLetterQueue) Store(ctx context.Context, op FailedOperation) error {
    query := `
        INSERT INTO dead_letter_queue (
            id, operation_type, payload, error, retry_count, created_at
        ) VALUES ($1, $2, $3, $4, $5, $6)`
    
    _, err := dlq.db.ExecContext(ctx, query,
        uuid.New(),
        op.Type,
        op.Payload,
        op.Error.Error(),
        op.RetryCount,
        time.Now().UTC())
    
    return err
}

// Periodic retry of dead letter items
func (dlq *DeadLetterQueue) RetryFailed(ctx context.Context) error {
    query := `
        SELECT id, operation_type, payload, retry_count
        FROM dead_letter_queue
        WHERE retry_count < 5
        AND last_retry < NOW() - INTERVAL '1 hour'
        ORDER BY created_at
        LIMIT 100`
    
    // Process and update retry counts...
}
```

## ✅ Code Review Checklist

### Before Committing
```bash
# 1. Run all checks
make ci

# 2. Check for compilation errors
go build -gcflags="-e" ./...

# 3. Run specific tests affected by changes
go test -run TestAffectedArea ./...

# 4. Check for AIDEV comments
grep -r "AIDEV-TODO" . --include="*.go" | grep -v "vendor"

# 5. Verify no secrets in code
gitleaks detect --source . --verbose

# 6. Format and lint
make fmt
make lint
```

### Performance Regression Indicators
- Query execution time > 100ms (check logs)
- API response time > 50ms p99 (check metrics)
- Memory allocation > 10MB per request
- Goroutine count > 10,000
- Database connection pool exhaustion
- Cache hit rate < 80%

### Security Vulnerability Patterns
```go
// ❌ SQL Injection vulnerable
query := fmt.Sprintf("SELECT * FROM users WHERE id = %s", userID)

// ✅ Parameterized query
query := "SELECT * FROM users WHERE id = $1"
db.QueryRow(query, userID)

// ❌ Path traversal vulnerable  
file := filepath.Join("/uploads", userInput)

// ✅ Sanitized path
file := filepath.Join("/uploads", filepath.Base(userInput))

// ❌ Unvalidated redirect
http.Redirect(w, r, r.URL.Query().Get("redirect"), 302)

// ✅ Validated redirect
redirect := r.URL.Query().Get("redirect")
if isValidRedirect(redirect) {
    http.Redirect(w, r, redirect, 302)
}
```

### Documentation Requirements
- All public functions must have godoc comments
- Complex algorithms need explanation comments
- API endpoints need OpenAPI annotations
- Performance-critical sections need benchmarks
- Breaking changes need migration guides

## 🔧 Tool-Specific Commands

### Delve Debugger
```bash
# Debug specific test
dlv test ./internal/service/bidding -- -test.run TestAuction

# Debug with breakpoint
dlv debug ./cmd/api/main.go
(dlv) break internal/service/callrouting/service.go:42
(dlv) continue
```

### pprof Profiling
```bash
# CPU profile
go test -cpuprofile cpu.prof -bench=. ./internal/service/callrouting
go tool pprof -http=:8080 cpu.prof

# Memory profile
go test -memprofile mem.prof -bench=. ./internal/service/bidding  
go tool pprof -top mem.prof | head -20
```

### Database Tools
```bash
# Analyze slow queries
psql $DATABASE_URL -c "SELECT * FROM pg_stat_statements ORDER BY mean_exec_time DESC LIMIT 10"

# Check index usage
psql $DATABASE_URL -c "SELECT * FROM pg_stat_user_indexes WHERE idx_scan = 0"

# Vacuum and analyze
psql $DATABASE_URL -c "VACUUM ANALYZE"
```

This comprehensive guide should be used alongside CLAUDE.md for effective development on the DCE project.

## Related Documentation

- **[docs/QUICKSTART.md](../docs/QUICKSTART.md)** - Getting started with DCE development
- **[COMMAND_REFERENCE.md](COMMAND_REFERENCE.md)** - Complete DCE command reference
- **[docs/TROUBLESHOOTING.md](docs/TROUBLESHOOTING.md)** - Solutions for DCE-specific issues