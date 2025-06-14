# Rate Limiting and Security Hardening Specification

## Executive Summary

### Problem Statement
The current implementation contains a **stub rate limiting middleware** with no actual enforcement, leaving the API vulnerable to:
- DDoS attacks that could overwhelm the system
- API abuse and resource exhaustion
- Brute force attacks on authentication endpoints
- Data scraping and unauthorized bulk operations

Additionally, multiple security gaps exist including:
- No request body size limits (memory exhaustion risk)
- Missing CSRF protection
- Incomplete input validation
- No SQL injection prevention beyond basic ORM usage
- Lack of PII encryption at rest

### Business Risks
- **Service Availability**: Unprotected endpoints can be overwhelmed, causing downtime
- **Financial Impact**: Resource exhaustion leads to increased cloud costs
- **Compliance Violations**: TCPA and GDPR require data protection measures
- **Reputation Damage**: Security breaches erode customer trust
- **Data Theft**: Unprotected APIs enable bulk data extraction

### Proposed Solution
Implement a comprehensive rate limiting system with Redis-backed distributed counters and multi-layered security hardening including request validation, encryption, and monitoring.

## Rate Limiting Strategy

### Per-Endpoint Limits
Different endpoints require different rate limits based on their computational cost and business value:

```yaml
rate_limits:
  endpoints:
    # High-frequency operations
    - path: /api/v1/bids
      limit: 1000/second
      burst: 2000
      window: sliding
      
    - path: /api/v1/calls/*/status
      limit: 500/second
      burst: 1000
      window: sliding
      
    # Moderate frequency
    - path: /api/v1/calls
      limit: 100/second
      burst: 200
      window: fixed
      
    - path: /api/v1/analytics/*
      limit: 50/second
      burst: 100
      window: fixed
      
    # Low frequency / expensive operations
    - path: /api/v1/accounts
      limit: 10/second
      burst: 20
      window: fixed
      
    - path: /api/v1/compliance/dnc/bulk
      limit: 5/minute
      burst: 10
      window: fixed
      
    # Webhooks (external callbacks)
    - path: /webhook/*
      limit: 100/second
      burst: 200
      window: sliding
      
    # Authentication endpoints
    - path: /api/v1/auth/login
      limit: 5/minute
      burst: 10
      window: fixed
      by_ip: true  # Rate limit by IP, not API key
```

### Per-User/API Key Limits
Implement tiered rate limits based on account type:

```yaml
account_tiers:
  trial:
    global_limit: 100/minute
    burst_multiplier: 1.5
    
  standard:
    global_limit: 1000/minute
    burst_multiplier: 2.0
    
  premium:
    global_limit: 10000/minute
    burst_multiplier: 2.5
    
  enterprise:
    global_limit: custom
    burst_multiplier: 3.0
```

### Geographic Rate Limiting
Implement country-based rate limits for compliance and abuse prevention:

```yaml
geographic_limits:
  # Countries with stricter limits due to fraud patterns
  high_risk_countries:
    - code: XX
      multiplier: 0.1  # 10% of normal rate
    - code: YY
      multiplier: 0.2  # 20% of normal rate
      
  # GDPR countries require different handling
  gdpr_countries:
    data_retention_days: 365
    require_explicit_consent: true
```

### Burst Allowances
Allow temporary bursts for legitimate traffic spikes:

```go
type BurstConfig struct {
    Multiplier   float64       // How much to multiply base rate
    Duration     time.Duration // How long burst is allowed
    CooldownTime time.Duration // Time before next burst
}

burstConfigs := map[string]BurstConfig{
    "standard": {
        Multiplier:   2.0,
        Duration:     10 * time.Second,
        CooldownTime: 60 * time.Second,
    },
    "premium": {
        Multiplier:   2.5,
        Duration:     30 * time.Second,
        CooldownTime: 30 * time.Second,
    },
}
```

### Graceful Degradation
When rate limits are approached, implement progressive degradation:

1. **80% threshold**: Add `X-RateLimit-Warning` header
2. **90% threshold**: Increase cache TTLs, reduce data detail
3. **100% threshold**: Return 429 with retry-after header
4. **Sustained overload**: Temporary blacklist (5 minutes)

## Implementation Approach

### Token Bucket Algorithm
Primary algorithm for rate limiting with smooth traffic flow:

```go
type TokenBucket struct {
    capacity    int64         // Maximum tokens
    tokens      int64         // Current tokens
    refillRate  int64         // Tokens per second
    lastRefill  time.Time     // Last refill timestamp
    mu          sync.RWMutex  // Thread safety
}

func (tb *TokenBucket) Allow(tokens int64) bool {
    tb.mu.Lock()
    defer tb.mu.Unlock()
    
    // Refill tokens based on time elapsed
    now := time.Now()
    elapsed := now.Sub(tb.lastRefill)
    newTokens := int64(elapsed.Seconds()) * tb.refillRate
    
    tb.tokens = min(tb.capacity, tb.tokens+newTokens)
    tb.lastRefill = now
    
    // Check if enough tokens available
    if tb.tokens >= tokens {
        tb.tokens -= tokens
        return true
    }
    
    return false
}
```

### Redis-Backed Counters
Distributed rate limiting across multiple API servers:

```go
type RedisRateLimiter struct {
    client *redis.Client
    prefix string
}

func (r *RedisRateLimiter) CheckLimit(key string, limit int64, window time.Duration) (bool, error) {
    pipe := r.client.Pipeline()
    
    now := time.Now()
    windowStart := now.Add(-window).Unix()
    
    // Remove old entries
    pipe.ZRemRangeByScore(context.Background(), key, "-inf", fmt.Sprint(windowStart))
    
    // Count current entries
    count := pipe.ZCard(context.Background(), key)
    
    // Add current request
    pipe.ZAdd(context.Background(), key, &redis.Z{
        Score:  float64(now.Unix()),
        Member: uuid.New().String(),
    })
    
    // Set expiry
    pipe.Expire(context.Background(), key, window+time.Minute)
    
    _, err := pipe.Exec(context.Background())
    if err != nil {
        return false, err
    }
    
    return count.Val() < limit, nil
}
```

### Sliding Window Counters
More accurate rate limiting for critical endpoints:

```go
type SlidingWindowLimiter struct {
    store       Store
    windowSize  time.Duration
    bucketCount int
}

func (s *SlidingWindowLimiter) RecordRequest(key string) error {
    now := time.Now()
    bucketKey := s.getBucketKey(key, now)
    
    return s.store.Increment(bucketKey, 1, s.windowSize)
}

func (s *SlidingWindowLimiter) GetRequestCount(key string) (int64, error) {
    now := time.Now()
    count := int64(0)
    
    // Sum all buckets in the window
    for i := 0; i < s.bucketCount; i++ {
        bucketTime := now.Add(-time.Duration(i) * s.getBucketDuration())
        bucketKey := s.getBucketKey(key, bucketTime)
        
        bucketCount, _ := s.store.Get(bucketKey)
        count += bucketCount
    }
    
    return count, nil
}
```

## Security Hardening

### Request Size Limits
Prevent memory exhaustion attacks:

```go
func RequestSizeLimitMiddleware(maxBytes int64) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
            
            // Also check Content-Length header
            if r.ContentLength > maxBytes {
                http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
                return
            }
            
            next.ServeHTTP(w, r)
        })
    }
}

// Apply different limits per endpoint
mux.Handle("/api/v1/upload", RequestSizeLimitMiddleware(10<<20)(uploadHandler))    // 10MB
mux.Handle("/api/v1/", RequestSizeLimitMiddleware(1<<20)(apiHandler))              // 1MB default
```

### CSRF Protection
Implement double-submit cookie pattern:

```go
type CSRFMiddleware struct {
    tokenGenerator TokenGenerator
    cookieName     string
    headerName     string
}

func (c *CSRFMiddleware) Handler(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Skip for safe methods
        if r.Method == "GET" || r.Method == "HEAD" || r.Method == "OPTIONS" {
            next.ServeHTTP(w, r)
            return
        }
        
        // Get token from cookie
        cookie, err := r.Cookie(c.cookieName)
        if err != nil {
            http.Error(w, "CSRF token missing", http.StatusForbidden)
            return
        }
        
        // Compare with header
        headerToken := r.Header.Get(c.headerName)
        if !c.tokenGenerator.Verify(cookie.Value, headerToken) {
            http.Error(w, "CSRF token invalid", http.StatusForbidden)
            return
        }
        
        next.ServeHTTP(w, r)
    })
}
```

### SQL Injection Prevention
Beyond ORM, implement prepared statement validation:

```go
type QueryValidator struct {
    patterns []*regexp.Regexp
}

func NewQueryValidator() *QueryValidator {
    return &QueryValidator{
        patterns: []*regexp.Regexp{
            regexp.MustCompile(`(?i)(union\s+select|select\s+\*|drop\s+table|insert\s+into|delete\s+from)`),
            regexp.MustCompile(`(?i)(script|javascript|vbscript|onload|onerror|onclick)`),
            regexp.MustCompile(`[';]--`),
        },
    }
}

func (q *QueryValidator) IsSafe(input string) bool {
    for _, pattern := range q.patterns {
        if pattern.MatchString(input) {
            return false
        }
    }
    return true
}
```

### XSS Protection
Content Security Policy and output encoding:

```go
func SecurityHeadersMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Content Security Policy
        w.Header().Set("Content-Security-Policy", 
            "default-src 'self'; "+
            "script-src 'self' 'strict-dynamic'; "+
            "style-src 'self' 'unsafe-inline'; "+
            "img-src 'self' data: https:; "+
            "font-src 'self'; "+
            "connect-src 'self'; "+
            "frame-ancestors 'none'; "+
            "base-uri 'self'; "+
            "form-action 'self'")
        
        // Other security headers
        w.Header().Set("X-Content-Type-Options", "nosniff")
        w.Header().Set("X-Frame-Options", "DENY")
        w.Header().Set("X-XSS-Protection", "1; mode=block")
        w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
        w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
        
        next.ServeHTTP(w, r)
    })
}
```

### Input Validation Framework
Comprehensive validation using struct tags:

```go
type CreateBidRequest struct {
    CallID   uuid.UUID `json:"call_id" validate:"required,uuid"`
    Amount   float64   `json:"amount" validate:"required,gt=0,lte=10000"`
    Duration int       `json:"duration" validate:"required,min=1,max=3600"`
    Criteria struct {
        States []string `json:"states" validate:"required,dive,len=2,alpha"`
        Radius int      `json:"radius_miles" validate:"omitempty,min=1,max=500"`
    } `json:"criteria" validate:"required"`
}

func ValidateRequest(v *validator.Validate, req interface{}) error {
    if err := v.Struct(req); err != nil {
        var invalidFields []string
        for _, err := range err.(validator.ValidationErrors) {
            invalidFields = append(invalidFields, fmt.Sprintf(
                "field '%s' failed '%s' validation", 
                err.Field(), 
                err.Tag(),
            ))
        }
        return fmt.Errorf("validation failed: %s", strings.Join(invalidFields, ", "))
    }
    return nil
}
```

## Infrastructure Components

### Redis Cluster for Counters
High-availability Redis setup:

```yaml
redis_cluster:
  topology: cluster
  nodes: 6
  replicas: 1
  persistence: aof
  maxmemory_policy: allkeys-lru
  
  sentinel:
    quorum: 2
    down_after_milliseconds: 5000
    failover_timeout: 10000
```

### Rate Limit Middleware
Centralized middleware implementation:

```go
type RateLimitMiddleware struct {
    limiter      RateLimiter
    keyExtractor KeyExtractor
    responder    RateLimitResponder
}

func (m *RateLimitMiddleware) Handler(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Extract rate limit key
        key := m.keyExtractor.Extract(r)
        
        // Check limit
        result, err := m.limiter.CheckLimit(r.Context(), key)
        if err != nil {
            // Fail open on errors
            log.Error("Rate limit check failed", "error", err)
            next.ServeHTTP(w, r)
            return
        }
        
        // Add rate limit headers
        w.Header().Set("X-RateLimit-Limit", strconv.FormatInt(result.Limit, 10))
        w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(result.Remaining, 10))
        w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(result.ResetAt.Unix(), 10))
        
        if !result.Allowed {
            m.responder.TooManyRequests(w, result)
            return
        }
        
        next.ServeHTTP(w, r)
    })
}
```

### WAF Rules
Web Application Firewall configuration:

```yaml
waf_rules:
  # OWASP Core Rule Set
  - rule_set: OWASP-CRS-3.3
    mode: blocking
    
  # Custom rules for telephony
  - name: block_suspicious_phone_patterns
    condition: |
      request.uri_path contains "/api/v1/calls" AND
      request.body contains_regex "\\+1[0-9]{10}.*\\+1[0-9]{10}.*\\+1[0-9]{10}"
    action: block
    message: "Too many phone numbers in single request"
    
  # Geographic restrictions
  - name: geo_restrictions
    condition: |
      client.geo.country in ["XX", "YY", "ZZ"]
    action: block
    message: "Service not available in your region"
```

### IP Reputation Service
Integration with threat intelligence:

```go
type IPReputationService struct {
    providers []ReputationProvider
    cache     *cache.Cache
}

func (s *IPReputationService) CheckIP(ip string) (*Reputation, error) {
    // Check cache first
    if rep, found := s.cache.Get(ip); found {
        return rep.(*Reputation), nil
    }
    
    // Query providers
    var scores []float64
    for _, provider := range s.providers {
        score, err := provider.GetScore(ip)
        if err == nil {
            scores = append(scores, score)
        }
    }
    
    // Calculate aggregate score
    reputation := &Reputation{
        IP:         ip,
        Score:      calculateMedian(scores),
        CheckedAt:  time.Now(),
        IsThreat:   calculateMedian(scores) < 0.3,
    }
    
    // Cache result
    s.cache.Set(ip, reputation, 1*time.Hour)
    
    return reputation, nil
}
```

## PII Protection

### Encryption at Rest
Field-level encryption for sensitive data:

```go
type EncryptedPhoneNumber struct {
    Encrypted []byte `db:"phone_encrypted"`
    Hash      string `db:"phone_hash"` // For searching
}

func (e *EncryptedPhoneNumber) SetValue(phone string, key []byte) error {
    // Normalize phone number
    normalized := normalizePhoneNumber(phone)
    
    // Create searchable hash
    h := hmac.New(sha256.New, key)
    h.Write([]byte(normalized))
    e.Hash = hex.EncodeToString(h.Sum(nil))
    
    // Encrypt actual value
    block, err := aes.NewCipher(key)
    if err != nil {
        return err
    }
    
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return err
    }
    
    nonce := make([]byte, gcm.NonceSize())
    if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
        return err
    }
    
    e.Encrypted = gcm.Seal(nonce, nonce, []byte(normalized), nil)
    return nil
}
```

### Phone Number Masking
Display masking for logs and responses:

```go
func MaskPhoneNumber(phone string) string {
    if len(phone) < 10 {
        return "***"
    }
    
    // Show only last 4 digits
    return fmt.Sprintf("%s****%s", phone[:3], phone[len(phone)-4:])
}

// Custom JSON marshaler
func (p PhoneNumber) MarshalJSON() ([]byte, error) {
    masked := MaskPhoneNumber(string(p))
    return json.Marshal(masked)
}
```

### Audit Trail Encryption
Secure audit logging:

```go
type EncryptedAuditLog struct {
    ID        uuid.UUID
    Timestamp time.Time
    EventType string
    UserID    uuid.UUID
    IPAddress string // Hashed
    Details   []byte // Encrypted JSON
}

func (a *AuditLogger) Log(event AuditEvent) error {
    // Encrypt sensitive details
    details, err := a.encryptor.Encrypt(event.Details)
    if err != nil {
        return err
    }
    
    log := &EncryptedAuditLog{
        ID:        uuid.New(),
        Timestamp: time.Now(),
        EventType: event.Type,
        UserID:    event.UserID,
        IPAddress: a.hashIP(event.IPAddress),
        Details:   details,
    }
    
    return a.store.Save(log)
}
```

### Key Rotation
Automated key rotation strategy:

```go
type KeyRotationService struct {
    currentKey  *EncryptionKey
    previousKey *EncryptionKey
    rotationAge time.Duration
}

func (k *KeyRotationService) RotateIfNeeded() error {
    if time.Since(k.currentKey.CreatedAt) > k.rotationAge {
        // Generate new key
        newKey, err := GenerateEncryptionKey()
        if err != nil {
            return err
        }
        
        // Start re-encryption job
        if err := k.startReencryption(k.currentKey, newKey); err != nil {
            return err
        }
        
        // Update keys
        k.previousKey = k.currentKey
        k.currentKey = newKey
        
        // Schedule old key deletion
        k.scheduleKeyDeletion(k.previousKey, 30*24*time.Hour)
    }
    
    return nil
}
```

## API Security

### API Key Rotation
Automated key rotation with zero downtime:

```go
type APIKeyRotation struct {
    primary   string
    secondary string
    expiresAt time.Time
}

func (a *APIKeyService) RotateKeys(accountID uuid.UUID) (*APIKeyRotation, error) {
    // Generate new key
    newKey := generateSecureAPIKey()
    
    // Get current key
    currentKey, err := a.store.GetPrimaryKey(accountID)
    if err != nil {
        return nil, err
    }
    
    // Create rotation
    rotation := &APIKeyRotation{
        primary:   newKey,
        secondary: currentKey,
        expiresAt: time.Now().Add(30 * 24 * time.Hour),
    }
    
    // Save rotation
    if err := a.store.SaveRotation(accountID, rotation); err != nil {
        return nil, err
    }
    
    // Notify customer
    a.notifier.SendKeyRotationNotice(accountID, rotation)
    
    return rotation, nil
}
```

### OAuth 2.0 Support
Standard OAuth implementation:

```go
type OAuth2Config struct {
    ClientID     string
    ClientSecret string
    RedirectURL  string
    Scopes       []string
}

type OAuth2Handler struct {
    config *OAuth2Config
    store  TokenStore
}

func (h *OAuth2Handler) HandleCallback(w http.ResponseWriter, r *http.Request) {
    // Verify state parameter
    state := r.FormValue("state")
    if !h.verifyState(state) {
        http.Error(w, "Invalid state", http.StatusBadRequest)
        return
    }
    
    // Exchange code for token
    code := r.FormValue("code")
    token, err := h.exchangeCode(code)
    if err != nil {
        http.Error(w, "Token exchange failed", http.StatusBadRequest)
        return
    }
    
    // Store token securely
    if err := h.store.SaveToken(token); err != nil {
        http.Error(w, "Token storage failed", http.StatusInternalServerError)
        return
    }
    
    // Redirect to success page
    http.Redirect(w, r, "/dashboard", http.StatusFound)
}
```

### Webhook Signatures
HMAC-based webhook verification:

```go
type WebhookSigner struct {
    secret []byte
}

func (w *WebhookSigner) Sign(payload []byte) string {
    h := hmac.New(sha256.New, w.secret)
    h.Write(payload)
    
    timestamp := time.Now().Unix()
    h.Write([]byte(strconv.FormatInt(timestamp, 10)))
    
    signature := hex.EncodeToString(h.Sum(nil))
    return fmt.Sprintf("t=%d,v1=%s", timestamp, signature)
}

func (w *WebhookSigner) Verify(payload []byte, signatureHeader string) bool {
    parts := strings.Split(signatureHeader, ",")
    if len(parts) != 2 {
        return false
    }
    
    // Extract timestamp
    timestamp, err := strconv.ParseInt(strings.TrimPrefix(parts[0], "t="), 10, 64)
    if err != nil {
        return false
    }
    
    // Check timestamp is recent (5 minutes)
    if time.Now().Unix()-timestamp > 300 {
        return false
    }
    
    // Verify signature
    expectedSig := w.Sign(payload)
    return hmac.Equal([]byte(signatureHeader), []byte(expectedSig))
}
```

### Certificate Pinning
TLS certificate validation:

```go
type CertificatePinner struct {
    pins map[string][]string // domain -> SHA256 pins
}

func (c *CertificatePinner) VerifyPeerCertificate(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
    if len(verifiedChains) == 0 {
        return errors.New("no verified chains")
    }
    
    // Get leaf certificate
    cert := verifiedChains[0][0]
    
    // Check if we have pins for this domain
    pins, ok := c.pins[cert.Subject.CommonName]
    if !ok {
        return nil // No pinning for this domain
    }
    
    // Calculate pin for current certificate
    h := sha256.Sum256(cert.RawSubjectPublicKeyInfo)
    currentPin := base64.StdEncoding.EncodeToString(h[:])
    
    // Check against pins
    for _, pin := range pins {
        if pin == currentPin {
            return nil
        }
    }
    
    return errors.New("certificate pin mismatch")
}
```

## Monitoring & Alerts

### Rate Limit Violations
Track and alert on violations:

```yaml
alerts:
  - name: HighRateLimitViolations
    condition: |
      sum(rate(rate_limit_violations_total[5m])) > 100
    severity: warning
    notification:
      - security-team
      - on-call
    
  - name: SustainedRateLimitAttack
    condition: |
      sum(rate(rate_limit_violations_total[15m])) > 1000
    severity: critical
    notification:
      - security-team
      - on-call
      - management
    actions:
      - enable_ddos_mode
      - increase_cache_ttl
```

### Suspicious Patterns
ML-based anomaly detection:

```go
type AnomalyDetector struct {
    baseline *BaselineModel
    window   time.Duration
}

func (a *AnomalyDetector) DetectAnomalies(metrics []Metric) []Anomaly {
    var anomalies []Anomaly
    
    for _, metric := range metrics {
        // Calculate z-score
        zscore := (metric.Value - a.baseline.Mean) / a.baseline.StdDev
        
        if math.Abs(zscore) > 3.0 {
            anomalies = append(anomalies, Anomaly{
                Metric:    metric,
                ZScore:    zscore,
                Severity:  a.calculateSeverity(zscore),
                Timestamp: time.Now(),
            })
        }
    }
    
    return anomalies
}
```

### Failed Authentication Attempts
Track authentication failures:

```go
type AuthFailureTracker struct {
    store     Store
    threshold int
    window    time.Duration
}

func (a *AuthFailureTracker) RecordFailure(userID, ip string) error {
    key := fmt.Sprintf("auth_failure:%s:%s", userID, ip)
    
    count, err := a.store.Increment(key, 1, a.window)
    if err != nil {
        return err
    }
    
    if count >= a.threshold {
        // Trigger alert
        alert := &SecurityAlert{
            Type:      "AUTH_FAILURE_THRESHOLD",
            UserID:    userID,
            IP:        ip,
            Count:     count,
            Window:    a.window,
            Timestamp: time.Now(),
        }
        
        a.alertManager.Send(alert)
        
        // Temporary block
        a.blockService.BlockIP(ip, 15*time.Minute)
    }
    
    return nil
}
```

### Security Events
Centralized security event logging:

```go
type SecurityEventLogger struct {
    output io.Writer
    level  LogLevel
}

func (s *SecurityEventLogger) LogEvent(event SecurityEvent) {
    entry := map[string]interface{}{
        "timestamp":   event.Timestamp,
        "type":        event.Type,
        "severity":    event.Severity,
        "user_id":     event.UserID,
        "ip_address":  event.IPAddress,
        "user_agent":  event.UserAgent,
        "request_id":  event.RequestID,
        "details":     event.Details,
        "stack_trace": event.StackTrace,
    }
    
    // Log to SIEM
    json.NewEncoder(s.output).Encode(entry)
    
    // Critical events trigger immediate alerts
    if event.Severity >= SeverityCritical {
        s.alertManager.SendImmediate(event)
    }
}
```

## Performance Considerations

### Caching Strategy
Minimize Redis lookups:

```go
type CachedRateLimiter struct {
    redis      *redis.Client
    localCache *lru.Cache
    ttl        time.Duration
}

func (c *CachedRateLimiter) CheckLimit(key string, limit int64) (bool, error) {
    // Check local cache first
    if val, ok := c.localCache.Get(key); ok {
        cached := val.(*CachedLimit)
        if time.Since(cached.Timestamp) < c.ttl {
            return cached.Remaining > 0, nil
        }
    }
    
    // Fall back to Redis
    remaining, err := c.checkRedis(key, limit)
    if err != nil {
        return false, err
    }
    
    // Update local cache
    c.localCache.Add(key, &CachedLimit{
        Remaining: remaining,
        Timestamp: time.Now(),
    })
    
    return remaining > 0, nil
}
```

### Connection Pooling
Optimize Redis connections:

```yaml
redis:
  pool:
    max_idle: 50
    max_active: 100
    idle_timeout: 300s
    max_conn_lifetime: 3600s
  
  cluster:
    read_preference: nearest
    retry_attempts: 3
    retry_delay: 100ms
```

## Effort Estimate

### Development Timeline (4-5 developer days)

**Day 1: Core Rate Limiting (8 hours)**
- Token bucket implementation (2h)
- Redis integration (2h)
- Basic middleware (2h)
- Unit tests (2h)

**Day 2: Advanced Features (8 hours)**
- Sliding window counters (2h)
- Per-user/tier limits (2h)
- Burst handling (2h)
- Integration tests (2h)

**Day 3: Security Hardening (8 hours)**
- Request size limits (1h)
- CSRF protection (2h)
- Input validation framework (2h)
- Security headers (1h)
- XSS prevention (2h)

**Day 4: PII & API Security (8 hours)**
- Field encryption (2h)
- Phone masking (1h)
- API key rotation (2h)
- Webhook signatures (1h)
- OAuth 2.0 setup (2h)

**Day 5: Monitoring & Testing (8 hours)**
- Metrics and alerts (2h)
- Security event logging (2h)
- Load testing (2h)
- Documentation (2h)

### Resource Requirements

**Team**
- 1 Senior Backend Engineer (lead)
- 1 Security Engineer (consultation)
- 1 DevOps Engineer (Redis setup)

**Infrastructure**
- Redis Cluster (6 nodes)
- WAF deployment
- SIEM integration
- Monitoring stack updates

## Success Criteria

1. **Performance**
   - Rate limit checks < 1ms latency
   - No impact on p99 API response times
   - Support 100K+ concurrent rate limit checks

2. **Security**
   - Pass OWASP security scan
   - Zero SQL injection vulnerabilities
   - All PII encrypted at rest
   - Successful DDoS simulation defense

3. **Reliability**
   - 99.99% rate limiter availability
   - Graceful Redis failure handling
   - No false positive blocks for legitimate traffic

4. **Compliance**
   - TCPA compliant request filtering
   - GDPR compliant data protection
   - SOC 2 audit ready logging

## Next Steps

1. **Review and approve** specification with security team
2. **Set up Redis cluster** in development environment
3. **Implement core rate limiting** with tests
4. **Security hardening** implementation
5. **Load test** with realistic traffic patterns
6. **Security audit** by external firm
7. **Production rollout** with feature flags