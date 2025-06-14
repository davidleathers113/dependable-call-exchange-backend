# DNC List Integration Technical Specification

## Feature Overview

The DNC (Do Not Call) List Integration is a critical compliance feature that ensures the Dependable Call Exchange platform adheres to federal and state regulations by preventing calls to registered phone numbers. This feature will integrate with national and state DNC registries, maintain internal suppression lists, and perform real-time scrubbing with sub-100ms performance.

### Key Requirements
- **Compliance**: Federal DNC registry + state-specific lists
- **Performance**: < 100ms phone number checks
- **Accuracy**: 99.99% availability with fallback mechanisms
- **Auditability**: Complete compliance reporting and history
- **Scalability**: Handle 100K+ checks per second

### Risk Mitigation
- **Risk Score**: 90/100 (FTC fines up to $43,792 per violation)
- **Legal Exposure**: Direct liability for DNC violations
- **Business Impact**: License revocation, reputation damage

## Domain Model Design

### Core Aggregates

#### DNCRegistry Aggregate
```go
// internal/domain/compliance/dnc_registry.go
package compliance

import (
    "time"
    "github.com/google/uuid"
    "github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
)

// DNCRegistry represents a Do Not Call registry (federal or state)
type DNCRegistry struct {
    ID           uuid.UUID           `json:"id"`
    Name         string              `json:"name"`
    Type         DNCRegistryType     `json:"type"`
    Jurisdiction string              `json:"jurisdiction"` // "federal" or state code
    Status       DNCRegistryStatus   `json:"status"`
    
    // Sync tracking
    LastSyncAt   *time.Time         `json:"last_sync_at"`
    NextSyncAt   time.Time          `json:"next_sync_at"`
    SyncInterval time.Duration      `json:"sync_interval"`
    
    // API configuration
    APIEndpoint  string             `json:"api_endpoint"`
    APIKey       string             `json:"-"` // Encrypted
    RateLimit    int                `json:"rate_limit"` // requests per second
    
    // Statistics
    TotalNumbers int64              `json:"total_numbers"`
    LastUpdated  time.Time          `json:"last_updated"`
    Version      string             `json:"version"`
    
    CreatedAt    time.Time          `json:"created_at"`
    UpdatedAt    time.Time          `json:"updated_at"`
}

type DNCRegistryType int

const (
    DNCRegistryTypeFederal DNCRegistryType = iota
    DNCRegistryTypeState
    DNCRegistryTypeInternal
)

type DNCRegistryStatus int

const (
    DNCRegistryStatusActive DNCRegistryStatus = iota
    DNCRegistryStatusInactive
    DNCRegistryStatusSyncing
    DNCRegistryStatusError
)

// NewDNCRegistry creates a new DNC registry
func NewDNCRegistry(name string, registryType DNCRegistryType, jurisdiction string) (*DNCRegistry, error) {
    if name == "" {
        return nil, ErrInvalidRegistryName
    }
    
    now := time.Now()
    return &DNCRegistry{
        ID:           uuid.New(),
        Name:         name,
        Type:         registryType,
        Jurisdiction: jurisdiction,
        Status:       DNCRegistryStatusInactive,
        SyncInterval: 24 * time.Hour, // Default daily sync
        CreatedAt:    now,
        UpdatedAt:    now,
    }, nil
}

// MarkSyncCompleted updates sync tracking
func (r *DNCRegistry) MarkSyncCompleted(totalNumbers int64, version string) {
    now := time.Now()
    r.LastSyncAt = &now
    r.NextSyncAt = now.Add(r.SyncInterval)
    r.TotalNumbers = totalNumbers
    r.Version = version
    r.LastUpdated = now
    r.UpdatedAt = now
}

// IsActive checks if registry is active and ready
func (r *DNCRegistry) IsActive() bool {
    return r.Status == DNCRegistryStatusActive && r.LastSyncAt != nil
}

// NeedsSyncing checks if registry needs to be synced
func (r *DNCRegistry) NeedsSyncing() bool {
    return time.Now().After(r.NextSyncAt) || r.LastSyncAt == nil
}
```

#### DNCEntry Entity
```go
// internal/domain/compliance/dnc_entry.go
package compliance

import (
    "time"
    "github.com/google/uuid"
    "github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
)

// DNCEntry represents a phone number on a DNC list
type DNCEntry struct {
    ID              uuid.UUID         `json:"id"`
    PhoneNumber     values.PhoneNumber `json:"phone_number"`
    RegistryID      uuid.UUID         `json:"registry_id"`
    Source          DNCSource         `json:"source"`
    
    // Registration details
    RegisteredAt    time.Time         `json:"registered_at"`
    ExpiresAt       *time.Time        `json:"expires_at,omitempty"`
    
    // Additional metadata
    ConsumerName    string            `json:"consumer_name,omitempty"`
    Notes           string            `json:"notes,omitempty"`
    
    // Audit fields
    AddedBy         string            `json:"added_by"`
    AddedAt         time.Time         `json:"added_at"`
    LastVerifiedAt  time.Time         `json:"last_verified_at"`
}

type DNCSource int

const (
    DNCSourceFederalRegistry DNCSource = iota
    DNCSourceStateRegistry
    DNCSourceConsumerRequest
    DNCSourceInternalList
    DNCSourcePartnerList
)

// NewDNCEntry creates a new DNC entry
func NewDNCEntry(phoneNumber values.PhoneNumber, registryID uuid.UUID, source DNCSource) (*DNCEntry, error) {
    now := time.Now()
    return &DNCEntry{
        ID:             uuid.New(),
        PhoneNumber:    phoneNumber,
        RegistryID:     registryID,
        Source:         source,
        RegisteredAt:   now,
        AddedAt:        now,
        LastVerifiedAt: now,
    }, nil
}

// IsActive checks if the DNC entry is currently active
func (e *DNCEntry) IsActive() bool {
    if e.ExpiresAt == nil {
        return true
    }
    return time.Now().Before(*e.ExpiresAt)
}

// Verify updates the last verification timestamp
func (e *DNCEntry) Verify() {
    e.LastVerifiedAt = time.Now()
}
```

#### SuppressionRule Value Object
```go
// internal/domain/compliance/suppression_rule.go
package compliance

import (
    "fmt"
    "regexp"
    "github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
)

// SuppressionRule defines a pattern-based suppression
type SuppressionRule struct {
    Pattern     string              `json:"pattern"`
    Type        SuppressionType     `json:"type"`
    Description string              `json:"description"`
    regex       *regexp.Regexp
}

type SuppressionType int

const (
    SuppressionTypeAreaCode SuppressionType = iota
    SuppressionTypeExchange
    SuppressionTypePrefix
    SuppressionTypeRegex
)

// NewSuppressionRule creates a new suppression rule
func NewSuppressionRule(pattern string, ruleType SuppressionType, description string) (*SuppressionRule, error) {
    rule := &SuppressionRule{
        Pattern:     pattern,
        Type:        ruleType,
        Description: description,
    }
    
    if ruleType == SuppressionTypeRegex {
        regex, err := regexp.Compile(pattern)
        if err != nil {
            return nil, fmt.Errorf("invalid regex pattern: %w", err)
        }
        rule.regex = regex
    }
    
    return rule, nil
}

// Matches checks if a phone number matches the suppression rule
func (r *SuppressionRule) Matches(phone values.PhoneNumber) bool {
    switch r.Type {
    case SuppressionTypeAreaCode:
        return phone.AreaCode() == r.Pattern
    case SuppressionTypeExchange:
        return phone.Exchange() == r.Pattern
    case SuppressionTypePrefix:
        return phone.AreaCode() + phone.Exchange() == r.Pattern
    case SuppressionTypeRegex:
        return r.regex.MatchString(phone.E164())
    default:
        return false
    }
}
```

### Repository Interfaces
```go
// internal/domain/compliance/repositories.go
package compliance

import (
    "context"
    "time"
    "github.com/google/uuid"
    "github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
)

// DNCRegistryRepository handles DNC registry persistence
type DNCRegistryRepository interface {
    // Registry management
    Create(ctx context.Context, registry *DNCRegistry) error
    Update(ctx context.Context, registry *DNCRegistry) error
    GetByID(ctx context.Context, id uuid.UUID) (*DNCRegistry, error)
    GetByType(ctx context.Context, registryType DNCRegistryType) ([]*DNCRegistry, error)
    GetActiveRegistries(ctx context.Context) ([]*DNCRegistry, error)
    GetRegistriesNeedingSync(ctx context.Context) ([]*DNCRegistry, error)
}

// DNCEntryRepository handles DNC entry persistence
type DNCEntryRepository interface {
    // Bulk operations for sync
    BulkCreate(ctx context.Context, entries []*DNCEntry) error
    BulkDelete(ctx context.Context, registryID uuid.UUID, phoneNumbers []values.PhoneNumber) error
    
    // Query operations
    ExistsByPhoneNumber(ctx context.Context, phoneNumber values.PhoneNumber) (bool, error)
    GetByPhoneNumber(ctx context.Context, phoneNumber values.PhoneNumber) ([]*DNCEntry, error)
    GetByRegistryID(ctx context.Context, registryID uuid.UUID, limit, offset int) ([]*DNCEntry, error)
    
    // Maintenance
    DeleteExpired(ctx context.Context) (int64, error)
    GetStaleEntries(ctx context.Context, staleAfter time.Duration) ([]*DNCEntry, error)
}

// SuppressionRuleRepository handles suppression rules
type SuppressionRuleRepository interface {
    Create(ctx context.Context, rule *SuppressionRule) error
    Delete(ctx context.Context, pattern string) error
    GetAll(ctx context.Context) ([]*SuppressionRule, error)
}
```

## Service Layer Design

### DNCService - Core DNC Management
```go
// internal/service/compliance/dnc_service.go
package compliance

import (
    "context"
    "fmt"
    "sync"
    "time"
    
    "github.com/google/uuid"
    "github.com/davidleathers/dependable-call-exchange-backend/internal/domain/compliance"
    "github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
    "github.com/davidleathers/dependable-call-exchange-backend/internal/infrastructure/cache"
    "github.com/davidleathers/dependable-call-exchange-backend/internal/infrastructure/telemetry"
)

// DNCService manages DNC list operations
type DNCService struct {
    registryRepo     compliance.DNCRegistryRepository
    entryRepo        compliance.DNCEntryRepository
    suppressionRepo  compliance.SuppressionRuleRepository
    cache            cache.Client
    logger           telemetry.Logger
    metrics          telemetry.Metrics
    
    // In-memory suppression rules for performance
    suppressionRules []*compliance.SuppressionRule
    rulesMutex       sync.RWMutex
}

// NewDNCService creates a new DNC service
func NewDNCService(
    registryRepo compliance.DNCRegistryRepository,
    entryRepo compliance.DNCEntryRepository,
    suppressionRepo compliance.SuppressionRuleRepository,
    cache cache.Client,
    logger telemetry.Logger,
    metrics telemetry.Metrics,
) (*DNCService, error) {
    service := &DNCService{
        registryRepo:    registryRepo,
        entryRepo:       entryRepo,
        suppressionRepo: suppressionRepo,
        cache:           cache,
        logger:          logger,
        metrics:         metrics,
    }
    
    // Load suppression rules into memory
    if err := service.loadSuppressionRules(context.Background()); err != nil {
        return nil, fmt.Errorf("failed to load suppression rules: %w", err)
    }
    
    return service, nil
}

// CreateRegistry creates a new DNC registry
func (s *DNCService) CreateRegistry(ctx context.Context, req CreateRegistryRequest) (*compliance.DNCRegistry, error) {
    registry, err := compliance.NewDNCRegistry(req.Name, req.Type, req.Jurisdiction)
    if err != nil {
        return nil, err
    }
    
    registry.APIEndpoint = req.APIEndpoint
    registry.APIKey = req.APIKey // Should be encrypted
    registry.RateLimit = req.RateLimit
    
    if err := s.registryRepo.Create(ctx, registry); err != nil {
        return nil, fmt.Errorf("failed to create registry: %w", err)
    }
    
    s.logger.Info("created DNC registry", 
        "registry_id", registry.ID,
        "name", registry.Name,
        "type", registry.Type)
    
    return registry, nil
}

// AddToSuppressionList adds a phone number to internal suppression
func (s *DNCService) AddToSuppressionList(ctx context.Context, req AddSuppressionRequest) error {
    phoneNumber, err := values.NewPhoneNumber(req.PhoneNumber)
    if err != nil {
        return fmt.Errorf("invalid phone number: %w", err)
    }
    
    // Get internal registry
    registries, err := s.registryRepo.GetByType(ctx, compliance.DNCRegistryTypeInternal)
    if err != nil {
        return fmt.Errorf("failed to get internal registry: %w", err)
    }
    
    if len(registries) == 0 {
        return fmt.Errorf("no internal registry found")
    }
    
    // Create DNC entry
    entry, err := compliance.NewDNCEntry(
        phoneNumber,
        registries[0].ID,
        compliance.DNCSourceConsumerRequest,
    )
    if err != nil {
        return err
    }
    
    entry.ConsumerName = req.ConsumerName
    entry.Notes = req.Reason
    entry.AddedBy = req.AddedBy
    
    if req.ExpiresIn > 0 {
        expiresAt := time.Now().Add(req.ExpiresIn)
        entry.ExpiresAt = &expiresAt
    }
    
    // Save to database
    if err := s.entryRepo.BulkCreate(ctx, []*compliance.DNCEntry{entry}); err != nil {
        return fmt.Errorf("failed to create suppression entry: %w", err)
    }
    
    // Invalidate cache
    cacheKey := s.getCacheKey(phoneNumber)
    if err := s.cache.Delete(ctx, cacheKey); err != nil {
        s.logger.Warn("failed to invalidate cache", "key", cacheKey, "error", err)
    }
    
    s.metrics.Counter("dnc.suppression.added", 1, map[string]string{
        "source": "consumer_request",
    })
    
    return nil
}

// AddSuppressionRule adds a pattern-based suppression rule
func (s *DNCService) AddSuppressionRule(ctx context.Context, req AddSuppressionRuleRequest) error {
    rule, err := compliance.NewSuppressionRule(req.Pattern, req.Type, req.Description)
    if err != nil {
        return fmt.Errorf("invalid suppression rule: %w", err)
    }
    
    if err := s.suppressionRepo.Create(ctx, rule); err != nil {
        return fmt.Errorf("failed to create suppression rule: %w", err)
    }
    
    // Reload rules
    if err := s.loadSuppressionRules(ctx); err != nil {
        s.logger.Error("failed to reload suppression rules", "error", err)
    }
    
    return nil
}

// loadSuppressionRules loads rules into memory
func (s *DNCService) loadSuppressionRules(ctx context.Context) error {
    rules, err := s.suppressionRepo.GetAll(ctx)
    if err != nil {
        return err
    }
    
    s.rulesMutex.Lock()
    s.suppressionRules = rules
    s.rulesMutex.Unlock()
    
    s.logger.Info("loaded suppression rules", "count", len(rules))
    return nil
}

// getCacheKey generates cache key for phone number
func (s *DNCService) getCacheKey(phone values.PhoneNumber) string {
    return fmt.Sprintf("dnc:phone:%s", phone.E164())
}

// Request types
type CreateRegistryRequest struct {
    Name         string
    Type         compliance.DNCRegistryType
    Jurisdiction string
    APIEndpoint  string
    APIKey       string
    RateLimit    int
}

type AddSuppressionRequest struct {
    PhoneNumber  string
    ConsumerName string
    Reason       string
    AddedBy      string
    ExpiresIn    time.Duration
}

type AddSuppressionRuleRequest struct {
    Pattern     string
    Type        compliance.SuppressionType
    Description string
}
```

### DNCScrubbingService - Real-time Checking
```go
// internal/service/compliance/dnc_scrubbing_service.go
package compliance

import (
    "context"
    "fmt"
    "time"
    
    "github.com/davidleathers/dependable-call-exchange-backend/internal/domain/compliance"
    "github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
    "github.com/davidleathers/dependable-call-exchange-backend/internal/infrastructure/cache"
    "github.com/davidleathers/dependable-call-exchange-backend/internal/infrastructure/telemetry"
)

// DNCScrubbingService provides real-time DNC checking
type DNCScrubbingService struct {
    dncService  *DNCService
    entryRepo   compliance.DNCEntryRepository
    cache       cache.Client
    logger      telemetry.Logger
    metrics     telemetry.Metrics
    
    // Configuration
    cacheEnabled bool
    cacheTTL     time.Duration
}

// NewDNCScrubbingService creates a new scrubbing service
func NewDNCScrubbingService(
    dncService *DNCService,
    entryRepo compliance.DNCEntryRepository,
    cache cache.Client,
    logger telemetry.Logger,
    metrics telemetry.Metrics,
) *DNCScrubbingService {
    return &DNCScrubbingService{
        dncService:   dncService,
        entryRepo:    entryRepo,
        cache:        cache,
        logger:       logger,
        metrics:      metrics,
        cacheEnabled: true,
        cacheTTL:     24 * time.Hour,
    }
}

// CheckPhoneNumber checks if a phone number is on any DNC list
func (s *DNCScrubbingService) CheckPhoneNumber(ctx context.Context, phoneNumber string) (*DNCCheckResult, error) {
    start := time.Now()
    defer func() {
        s.metrics.Histogram("dnc.check.duration_ms", float64(time.Since(start).Milliseconds()))
    }()
    
    phone, err := values.NewPhoneNumber(phoneNumber)
    if err != nil {
        return nil, fmt.Errorf("invalid phone number: %w", err)
    }
    
    // Check cache first
    if s.cacheEnabled {
        if result, found := s.checkCache(ctx, phone); found {
            s.metrics.Counter("dnc.check.cache_hit", 1)
            return result, nil
        }
        s.metrics.Counter("dnc.check.cache_miss", 1)
    }
    
    result := &DNCCheckResult{
        PhoneNumber: phone.E164(),
        CheckedAt:   time.Now(),
        OnDNCList:   false,
        Reasons:     []string{},
    }
    
    // Check suppression rules first (in-memory, fast)
    if s.checkSuppressionRules(phone) {
        result.OnDNCList = true
        result.Reasons = append(result.Reasons, "matched_suppression_rule")
    }
    
    // Check database
    entries, err := s.entryRepo.GetByPhoneNumber(ctx, phone)
    if err != nil {
        s.logger.Error("failed to check DNC database", "error", err)
        // Don't fail the check, but log for monitoring
    } else if len(entries) > 0 {
        result.OnDNCList = true
        for _, entry := range entries {
            if entry.IsActive() {
                result.Reasons = append(result.Reasons, s.getReasonForEntry(entry))
                result.Sources = append(result.Sources, entry.Source.String())
            }
        }
    }
    
    // Cache the result
    if s.cacheEnabled {
        s.cacheResult(ctx, phone, result)
    }
    
    s.metrics.Counter("dnc.check.completed", 1, map[string]string{
        "on_list": fmt.Sprintf("%t", result.OnDNCList),
    })
    
    return result, nil
}

// BulkCheckPhoneNumbers checks multiple phone numbers efficiently
func (s *DNCScrubbingService) BulkCheckPhoneNumbers(ctx context.Context, phoneNumbers []string) (*BulkDNCCheckResult, error) {
    start := time.Now()
    defer func() {
        s.metrics.Histogram("dnc.bulk_check.duration_ms", float64(time.Since(start).Milliseconds()))
    }()
    
    result := &BulkDNCCheckResult{
        Results:   make(map[string]*DNCCheckResult),
        CheckedAt: time.Now(),
    }
    
    // Convert and validate phone numbers
    phones := make([]values.PhoneNumber, 0, len(phoneNumbers))
    for _, num := range phoneNumbers {
        phone, err := values.NewPhoneNumber(num)
        if err != nil {
            result.Results[num] = &DNCCheckResult{
                PhoneNumber: num,
                CheckedAt:   time.Now(),
                Error:       err.Error(),
            }
            continue
        }
        phones = append(phones, phone)
    }
    
    // Check cache for all numbers
    uncachedPhones := make([]values.PhoneNumber, 0)
    if s.cacheEnabled {
        for _, phone := range phones {
            if cached, found := s.checkCache(ctx, phone); found {
                result.Results[phone.E164()] = cached
            } else {
                uncachedPhones = append(uncachedPhones, phone)
            }
        }
    } else {
        uncachedPhones = phones
    }
    
    // Batch check uncached numbers
    if len(uncachedPhones) > 0 {
        // Check suppression rules
        for _, phone := range uncachedPhones {
            checkResult := &DNCCheckResult{
                PhoneNumber: phone.E164(),
                CheckedAt:   time.Now(),
                OnDNCList:   false,
                Reasons:     []string{},
            }
            
            if s.checkSuppressionRules(phone) {
                checkResult.OnDNCList = true
                checkResult.Reasons = append(checkResult.Reasons, "matched_suppression_rule")
            }
            
            result.Results[phone.E164()] = checkResult
        }
        
        // Batch database check
        for _, phone := range uncachedPhones {
            entries, err := s.entryRepo.GetByPhoneNumber(ctx, phone)
            if err != nil {
                result.Results[phone.E164()].Error = err.Error()
                continue
            }
            
            if len(entries) > 0 {
                checkResult := result.Results[phone.E164()]
                checkResult.OnDNCList = true
                for _, entry := range entries {
                    if entry.IsActive() {
                        checkResult.Reasons = append(checkResult.Reasons, s.getReasonForEntry(entry))
                        checkResult.Sources = append(checkResult.Sources, entry.Source.String())
                    }
                }
            }
            
            // Cache individual results
            if s.cacheEnabled {
                s.cacheResult(ctx, phone, result.Results[phone.E164()])
            }
        }
    }
    
    // Calculate summary
    for _, r := range result.Results {
        result.TotalChecked++
        if r.OnDNCList {
            result.TotalOnList++
        }
        if r.Error != "" {
            result.TotalErrors++
        }
    }
    
    return result, nil
}

// checkSuppressionRules checks in-memory suppression rules
func (s *DNCScrubbingService) checkSuppressionRules(phone values.PhoneNumber) bool {
    s.dncService.rulesMutex.RLock()
    defer s.dncService.rulesMutex.RUnlock()
    
    for _, rule := range s.dncService.suppressionRules {
        if rule.Matches(phone) {
            return true
        }
    }
    return false
}

// checkCache checks the cache for a phone number
func (s *DNCScrubbingService) checkCache(ctx context.Context, phone values.PhoneNumber) (*DNCCheckResult, bool) {
    var result DNCCheckResult
    cacheKey := s.dncService.getCacheKey(phone)
    
    err := s.cache.Get(ctx, cacheKey, &result)
    if err != nil {
        return nil, false
    }
    
    return &result, true
}

// cacheResult caches a check result
func (s *DNCScrubbingService) cacheResult(ctx context.Context, phone values.PhoneNumber, result *DNCCheckResult) {
    cacheKey := s.dncService.getCacheKey(phone)
    if err := s.cache.Set(ctx, cacheKey, result, s.cacheTTL); err != nil {
        s.logger.Warn("failed to cache DNC result", "key", cacheKey, "error", err)
    }
}

// getReasonForEntry generates a reason string for a DNC entry
func (s *DNCScrubbingService) getReasonForEntry(entry *compliance.DNCEntry) string {
    switch entry.Source {
    case compliance.DNCSourceFederalRegistry:
        return "federal_dnc_registry"
    case compliance.DNCSourceStateRegistry:
        return "state_dnc_registry"
    case compliance.DNCSourceConsumerRequest:
        return "consumer_opt_out"
    case compliance.DNCSourceInternalList:
        return "internal_suppression"
    case compliance.DNCSourcePartnerList:
        return "partner_list"
    default:
        return "unknown_source"
    }
}

// Result types
type DNCCheckResult struct {
    PhoneNumber string    `json:"phone_number"`
    OnDNCList   bool      `json:"on_dnc_list"`
    Reasons     []string  `json:"reasons,omitempty"`
    Sources     []string  `json:"sources,omitempty"`
    CheckedAt   time.Time `json:"checked_at"`
    Error       string    `json:"error,omitempty"`
}

type BulkDNCCheckResult struct {
    Results      map[string]*DNCCheckResult `json:"results"`
    TotalChecked int                        `json:"total_checked"`
    TotalOnList  int                        `json:"total_on_list"`
    TotalErrors  int                        `json:"total_errors"`
    CheckedAt    time.Time                  `json:"checked_at"`
}
```

### DNCImportService - Bulk Import and Sync
```go
// internal/service/compliance/dnc_import_service.go
package compliance

import (
    "context"
    "encoding/csv"
    "fmt"
    "io"
    "sync"
    "time"
    
    "github.com/google/uuid"
    "github.com/davidleathers/dependable-call-exchange-backend/internal/domain/compliance"
    "github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
    "github.com/davidleathers/dependable-call-exchange-backend/internal/infrastructure/telemetry"
)

// DNCImportService handles bulk DNC list imports and synchronization
type DNCImportService struct {
    registryRepo compliance.DNCRegistryRepository
    entryRepo    compliance.DNCEntryRepository
    logger       telemetry.Logger
    metrics      telemetry.Metrics
    
    // Import configuration
    batchSize    int
    workerCount  int
}

// NewDNCImportService creates a new import service
func NewDNCImportService(
    registryRepo compliance.DNCRegistryRepository,
    entryRepo compliance.DNCEntryRepository,
    logger telemetry.Logger,
    metrics telemetry.Metrics,
) *DNCImportService {
    return &DNCImportService{
        registryRepo: registryRepo,
        entryRepo:    entryRepo,
        logger:       logger,
        metrics:      metrics,
        batchSize:    1000,
        workerCount:  10,
    }
}

// ImportCSV imports a CSV file of phone numbers
func (s *DNCImportService) ImportCSV(ctx context.Context, registryID uuid.UUID, reader io.Reader, source compliance.DNCSource) (*ImportResult, error) {
    result := &ImportResult{
        RegistryID: registryID,
        StartedAt:  time.Now(),
    }
    
    // Get registry
    registry, err := s.registryRepo.GetByID(ctx, registryID)
    if err != nil {
        return nil, fmt.Errorf("failed to get registry: %w", err)
    }
    
    // Mark registry as syncing
    registry.Status = compliance.DNCRegistryStatusSyncing
    if err := s.registryRepo.Update(ctx, registry); err != nil {
        s.logger.Warn("failed to update registry status", "error", err)
    }
    
    // Create CSV reader
    csvReader := csv.NewReader(reader)
    
    // Process in batches
    batch := make([]*compliance.DNCEntry, 0, s.batchSize)
    entryChan := make(chan []*compliance.DNCEntry, s.workerCount)
    errorChan := make(chan error, s.workerCount)
    
    // Start workers
    var wg sync.WaitGroup
    for i := 0; i < s.workerCount; i++ {
        wg.Add(1)
        go s.importWorker(ctx, &wg, entryChan, errorChan)
    }
    
    // Read and process CSV
    lineNum := 0
    for {
        record, err := csvReader.Read()
        if err == io.EOF {
            break
        }
        if err != nil {
            result.Errors = append(result.Errors, fmt.Sprintf("line %d: %v", lineNum, err))
            result.Failed++
            continue
        }
        
        lineNum++
        if lineNum == 1 && s.isHeaderRow(record) {
            continue // Skip header
        }
        
        // Parse phone number
        if len(record) == 0 {
            continue
        }
        
        phoneStr := record[0]
        phone, err := values.NewPhoneNumber(phoneStr)
        if err != nil {
            result.Errors = append(result.Errors, fmt.Sprintf("line %d: invalid phone %s: %v", lineNum, phoneStr, err))
            result.Failed++
            continue
        }
        
        // Create entry
        entry, err := compliance.NewDNCEntry(phone, registryID, source)
        if err != nil {
            result.Errors = append(result.Errors, fmt.Sprintf("line %d: %v", lineNum, err))
            result.Failed++
            continue
        }
        
        // Add to batch
        batch = append(batch, entry)
        result.Processed++
        
        // Send batch if full
        if len(batch) >= s.batchSize {
            entryChan <- batch
            batch = make([]*compliance.DNCEntry, 0, s.batchSize)
        }
    }
    
    // Send remaining batch
    if len(batch) > 0 {
        entryChan <- batch
    }
    
    // Close channel and wait for workers
    close(entryChan)
    wg.Wait()
    close(errorChan)
    
    // Collect errors
    for err := range errorChan {
        result.Errors = append(result.Errors, err.Error())
        result.Failed++
    }
    
    // Update registry
    result.CompletedAt = time.Now()
    result.Duration = result.CompletedAt.Sub(result.StartedAt)
    result.Imported = result.Processed - result.Failed
    
    registry.MarkSyncCompleted(int64(result.Imported), fmt.Sprintf("import_%s", result.CompletedAt.Format("20060102_150405")))
    registry.Status = compliance.DNCRegistryStatusActive
    if err := s.registryRepo.Update(ctx, registry); err != nil {
        s.logger.Error("failed to update registry after import", "error", err)
    }
    
    s.metrics.Counter("dnc.import.completed", 1, map[string]string{
        "registry_id": registryID.String(),
        "source":      source.String(),
    })
    s.metrics.Histogram("dnc.import.duration_seconds", result.Duration.Seconds())
    s.metrics.Counter("dnc.import.processed", float64(result.Processed))
    s.metrics.Counter("dnc.import.failed", float64(result.Failed))
    
    return result, nil
}

// SyncRegistry synchronizes a registry with its external source
func (s *DNCImportService) SyncRegistry(ctx context.Context, registryID uuid.UUID) (*SyncResult, error) {
    result := &SyncResult{
        RegistryID: registryID,
        StartedAt:  time.Now(),
    }
    
    registry, err := s.registryRepo.GetByID(ctx, registryID)
    if err != nil {
        return nil, fmt.Errorf("failed to get registry: %w", err)
    }
    
    if !registry.NeedsSyncing() {
        result.Skipped = true
        result.Message = "Registry does not need syncing"
        return result, nil
    }
    
    // Implementation depends on registry type
    switch registry.Type {
    case compliance.DNCRegistryTypeFederal:
        return s.syncFederalRegistry(ctx, registry)
    case compliance.DNCRegistryTypeState:
        return s.syncStateRegistry(ctx, registry)
    default:
        return nil, fmt.Errorf("unsupported registry type: %v", registry.Type)
    }
}

// importWorker processes batches of DNC entries
func (s *DNCImportService) importWorker(ctx context.Context, wg *sync.WaitGroup, entryChan <-chan []*compliance.DNCEntry, errorChan chan<- error) {
    defer wg.Done()
    
    for batch := range entryChan {
        if err := s.entryRepo.BulkCreate(ctx, batch); err != nil {
            errorChan <- fmt.Errorf("failed to import batch: %w", err)
        }
        
        s.metrics.Counter("dnc.import.batch_processed", 1)
    }
}

// isHeaderRow checks if a CSV row looks like a header
func (s *DNCImportService) isHeaderRow(record []string) bool {
    if len(record) == 0 {
        return false
    }
    
    header := record[0]
    // Common header variations
    headers := []string{"phone", "phone_number", "number", "telephone", "mobile"}
    for _, h := range headers {
        if header == h {
            return true
        }
    }
    return false
}

// syncFederalRegistry syncs with federal DNC API
func (s *DNCImportService) syncFederalRegistry(ctx context.Context, registry *compliance.DNCRegistry) (*SyncResult, error) {
    // This would integrate with the actual federal DNC API
    // For now, return a placeholder
    return &SyncResult{
        RegistryID: registry.ID,
        Message:    "Federal DNC sync not yet implemented",
        Skipped:    true,
    }, nil
}

// syncStateRegistry syncs with state DNC API
func (s *DNCImportService) syncStateRegistry(ctx context.Context, registry *compliance.DNCRegistry) (*SyncResult, error) {
    // This would integrate with state-specific APIs
    // For now, return a placeholder
    return &SyncResult{
        RegistryID: registry.ID,
        Message:    "State DNC sync not yet implemented",
        Skipped:    true,
    }, nil
}

// Result types
type ImportResult struct {
    RegistryID  uuid.UUID     `json:"registry_id"`
    Processed   int           `json:"processed"`
    Imported    int           `json:"imported"`
    Failed      int           `json:"failed"`
    Errors      []string      `json:"errors,omitempty"`
    StartedAt   time.Time     `json:"started_at"`
    CompletedAt time.Time     `json:"completed_at"`
    Duration    time.Duration `json:"duration"`
}

type SyncResult struct {
    RegistryID  uuid.UUID     `json:"registry_id"`
    Added       int           `json:"added"`
    Removed     int           `json:"removed"`
    Updated     int           `json:"updated"`
    Errors      []string      `json:"errors,omitempty"`
    StartedAt   time.Time     `json:"started_at"`
    CompletedAt time.Time     `json:"completed_at"`
    Duration    time.Duration `json:"duration"`
    Skipped     bool          `json:"skipped"`
    Message     string        `json:"message,omitempty"`
}
```

## Integration Design

### Federal DNC API Client
```go
// internal/infrastructure/dnc/federal_client.go
package dnc

import (
    "context"
    "encoding/xml"
    "fmt"
    "io"
    "net/http"
    "time"
    
    "golang.org/x/time/rate"
)

// FederalDNCClient integrates with the federal DNC registry
type FederalDNCClient struct {
    httpClient   *http.Client
    apiKey       string
    baseURL      string
    rateLimiter  *rate.Limiter
    
    // Configuration
    timeout      time.Duration
    maxRetries   int
}

// NewFederalDNCClient creates a new federal DNC client
func NewFederalDNCClient(apiKey string) *FederalDNCClient {
    return &FederalDNCClient{
        httpClient: &http.Client{
            Timeout: 30 * time.Second,
        },
        apiKey:      apiKey,
        baseURL:     "https://api.donotcall.gov/v1", // Example URL
        rateLimiter: rate.NewLimiter(rate.Every(time.Second), 10), // 10 req/s
        timeout:     30 * time.Second,
        maxRetries:  3,
    }
}

// CheckNumber checks if a phone number is on the federal DNC list
func (c *FederalDNCClient) CheckNumber(ctx context.Context, phoneNumber string) (*FederalDNCResponse, error) {
    // Wait for rate limit
    if err := c.rateLimiter.Wait(ctx); err != nil {
        return nil, fmt.Errorf("rate limit error: %w", err)
    }
    
    url := fmt.Sprintf("%s/check/%s", c.baseURL, phoneNumber)
    
    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
    if err != nil {
        return nil, fmt.Errorf("failed to create request: %w", err)
    }
    
    req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
    req.Header.Set("Accept", "application/xml")
    
    var resp *http.Response
    var lastErr error
    
    // Retry logic
    for i := 0; i < c.maxRetries; i++ {
        resp, lastErr = c.httpClient.Do(req)
        if lastErr == nil && resp.StatusCode < 500 {
            break
        }
        
        if resp != nil {
            resp.Body.Close()
        }
        
        // Exponential backoff
        time.Sleep(time.Duration(i+1) * time.Second)
    }
    
    if lastErr != nil {
        return nil, fmt.Errorf("request failed after %d retries: %w", c.maxRetries, lastErr)
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return nil, fmt.Errorf("API error: status=%d body=%s", resp.StatusCode, string(body))
    }
    
    var dncResp FederalDNCResponse
    if err := xml.NewDecoder(resp.Body).Decode(&dncResp); err != nil {
        return nil, fmt.Errorf("failed to decode response: %w", err)
    }
    
    return &dncResp, nil
}

// DownloadList downloads the full DNC list (requires special authorization)
func (c *FederalDNCClient) DownloadList(ctx context.Context, lastSync time.Time) (io.ReadCloser, error) {
    // Implementation would handle incremental downloads
    // This is a placeholder
    return nil, fmt.Errorf("not implemented")
}

// Response types
type FederalDNCResponse struct {
    XMLName     xml.Name  `xml:"DNCCheckResponse"`
    PhoneNumber string    `xml:"PhoneNumber"`
    OnDNCList   bool      `xml:"OnDNCList"`
    RegisteredDate *time.Time `xml:"RegisteredDate,omitempty"`
    ExpirationDate *time.Time `xml:"ExpirationDate,omitempty"`
}
```

### State DNC Integration Interface
```go
// internal/infrastructure/dnc/state_client.go
package dnc

import (
    "context"
    "fmt"
    "io"
    "time"
)

// StateDNCClient interface for state-specific DNC registries
type StateDNCClient interface {
    CheckNumber(ctx context.Context, phoneNumber string) (*StateDNCResponse, error)
    DownloadList(ctx context.Context, lastSync time.Time) (io.ReadCloser, error)
    GetState() string
}

// StateDNCResponse represents a state DNC check response
type StateDNCResponse struct {
    PhoneNumber    string
    OnDNCList      bool
    RegisteredDate *time.Time
    ExpirationDate *time.Time
    State          string
}

// StateDNCClientFactory creates state-specific clients
type StateDNCClientFactory struct {
    clients map[string]StateDNCClient
}

// NewStateDNCClientFactory creates a new factory
func NewStateDNCClientFactory() *StateDNCClientFactory {
    return &StateDNCClientFactory{
        clients: make(map[string]StateDNCClient),
    }
}

// RegisterClient registers a state-specific client
func (f *StateDNCClientFactory) RegisterClient(state string, client StateDNCClient) {
    f.clients[state] = client
}

// GetClient gets a client for a specific state
func (f *StateDNCClientFactory) GetClient(state string) (StateDNCClient, error) {
    client, ok := f.clients[state]
    if !ok {
        return nil, fmt.Errorf("no DNC client registered for state: %s", state)
    }
    return client, nil
}

// Example: California DNC Client
type CaliforniaDNCClient struct {
    apiKey  string
    baseURL string
}

func NewCaliforniaDNCClient(apiKey string) *CaliforniaDNCClient {
    return &CaliforniaDNCClient{
        apiKey:  apiKey,
        baseURL: "https://ca.gov/dnc/api", // Example
    }
}

func (c *CaliforniaDNCClient) CheckNumber(ctx context.Context, phoneNumber string) (*StateDNCResponse, error) {
    // California-specific implementation
    return &StateDNCResponse{
        PhoneNumber: phoneNumber,
        OnDNCList:   false,
        State:       "CA",
    }, nil
}

func (c *CaliforniaDNCClient) DownloadList(ctx context.Context, lastSync time.Time) (io.ReadCloser, error) {
    // Implementation
    return nil, fmt.Errorf("not implemented")
}

func (c *CaliforniaDNCClient) GetState() string {
    return "CA"
}
```

## Caching Strategy

### Redis Cache Implementation
```go
// internal/infrastructure/cache/dnc_cache.go
package cache

import (
    "context"
    "encoding/json"
    "fmt"
    "time"
    
    "github.com/redis/go-redis/v9"
)

// DNCCache provides DNC-specific caching
type DNCCache struct {
    client *redis.Client
    prefix string
    ttl    time.Duration
}

// NewDNCCache creates a new DNC cache
func NewDNCCache(client *redis.Client) *DNCCache {
    return &DNCCache{
        client: client,
        prefix: "dnc:",
        ttl:    24 * time.Hour, // Default 24-hour TTL
    }
}

// SetPhoneResult caches a phone number check result
func (c *DNCCache) SetPhoneResult(ctx context.Context, phoneNumber string, result interface{}) error {
    key := c.phoneKey(phoneNumber)
    
    data, err := json.Marshal(result)
    if err != nil {
        return fmt.Errorf("failed to marshal result: %w", err)
    }
    
    return c.client.Set(ctx, key, data, c.ttl).Err()
}

// GetPhoneResult gets a cached phone number check result
func (c *DNCCache) GetPhoneResult(ctx context.Context, phoneNumber string, result interface{}) error {
    key := c.phoneKey(phoneNumber)
    
    data, err := c.client.Get(ctx, key).Bytes()
    if err == redis.Nil {
        return ErrCacheMiss
    }
    if err != nil {
        return fmt.Errorf("cache get error: %w", err)
    }
    
    return json.Unmarshal(data, result)
}

// WarmCache pre-loads frequently checked numbers
func (c *DNCCache) WarmCache(ctx context.Context, phoneNumbers []string) error {
    pipe := c.client.Pipeline()
    
    for _, phone := range phoneNumbers {
        // This would be populated from database
        result := map[string]interface{}{
            "on_dnc_list": true,
            "checked_at":  time.Now(),
        }
        
        data, _ := json.Marshal(result)
        pipe.Set(ctx, c.phoneKey(phone), data, c.ttl)
    }
    
    _, err := pipe.Exec(ctx)
    return err
}

// InvalidatePhone removes a phone number from cache
func (c *DNCCache) InvalidatePhone(ctx context.Context, phoneNumber string) error {
    return c.client.Del(ctx, c.phoneKey(phoneNumber)).Err()
}

// phoneKey generates cache key for phone number
func (c *DNCCache) phoneKey(phoneNumber string) string {
    return fmt.Sprintf("%sphone:%s", c.prefix, phoneNumber)
}

var ErrCacheMiss = fmt.Errorf("cache miss")
```

## API Endpoints

### REST API Implementation
```go
// internal/api/rest/dnc_handlers.go
package rest

import (
    "encoding/json"
    "net/http"
    "time"
    
    "github.com/gorilla/mux"
    "github.com/davidleathers/dependable-call-exchange-backend/internal/service/compliance"
)

// DNCHandlers handles DNC-related HTTP requests
type DNCHandlers struct {
    dncService      *compliance.DNCService
    scrubbingService *compliance.DNCScrubbingService
    importService   *compliance.DNCImportService
}

// NewDNCHandlers creates new DNC handlers
func NewDNCHandlers(
    dncService *compliance.DNCService,
    scrubbingService *compliance.DNCScrubbingService,
    importService *compliance.DNCImportService,
) *DNCHandlers {
    return &DNCHandlers{
        dncService:       dncService,
        scrubbingService: scrubbingService,
        importService:    importService,
    }
}

// RegisterRoutes registers DNC routes
func (h *DNCHandlers) RegisterRoutes(router *mux.Router) {
    // DNC checking
    router.HandleFunc("/api/v1/dnc/check", h.CheckNumber).Methods("POST")
    router.HandleFunc("/api/v1/dnc/check/bulk", h.BulkCheck).Methods("POST")
    
    // Suppression management
    router.HandleFunc("/api/v1/dnc/suppression", h.AddToSuppression).Methods("POST")
    router.HandleFunc("/api/v1/dnc/suppression/rules", h.AddSuppressionRule).Methods("POST")
    
    // Registry management
    router.HandleFunc("/api/v1/dnc/registries", h.CreateRegistry).Methods("POST")
    router.HandleFunc("/api/v1/dnc/registries/{id}/sync", h.SyncRegistry).Methods("POST")
    
    // Import
    router.HandleFunc("/api/v1/dnc/import", h.ImportCSV).Methods("POST")
    
    // Reports
    router.HandleFunc("/api/v1/dnc/reports/compliance", h.GetComplianceReport).Methods("GET")
}

// CheckNumber checks a single phone number
func (h *DNCHandlers) CheckNumber(w http.ResponseWriter, r *http.Request) {
    var req struct {
        PhoneNumber string `json:"phone_number" validate:"required"`
    }
    
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }
    
    result, err := h.scrubbingService.CheckPhoneNumber(r.Context(), req.PhoneNumber)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(result)
}

// BulkCheck checks multiple phone numbers
func (h *DNCHandlers) BulkCheck(w http.ResponseWriter, r *http.Request) {
    var req struct {
        PhoneNumbers []string `json:"phone_numbers" validate:"required,max=1000"`
    }
    
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }
    
    result, err := h.scrubbingService.BulkCheckPhoneNumbers(r.Context(), req.PhoneNumbers)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(result)
}

// AddToSuppression adds a number to suppression list
func (h *DNCHandlers) AddToSuppression(w http.ResponseWriter, r *http.Request) {
    var req compliance.AddSuppressionRequest
    
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }
    
    // Get user from context
    userID := r.Context().Value("user_id").(string)
    req.AddedBy = userID
    
    if err := h.dncService.AddToSuppressionList(r.Context(), req); err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(map[string]string{
        "message": "Phone number added to suppression list",
    })
}

// GetComplianceReport generates a compliance report
func (h *DNCHandlers) GetComplianceReport(w http.ResponseWriter, r *http.Request) {
    startDate := r.URL.Query().Get("start_date")
    endDate := r.URL.Query().Get("end_date")
    
    // Parse dates
    start, _ := time.Parse("2006-01-02", startDate)
    end, _ := time.Parse("2006-01-02", endDate)
    
    report := map[string]interface{}{
        "period": map[string]string{
            "start": start.Format("2006-01-02"),
            "end":   end.Format("2006-01-02"),
        },
        "total_checks":     1000000, // Would come from metrics
        "violations_found": 150,
        "cache_hit_rate":   0.95,
        "avg_check_time_ms": 12,
        "registries": []map[string]interface{}{
            {
                "name":         "Federal DNC",
                "last_sync":    "2024-12-10T00:00:00Z",
                "total_numbers": 250000000,
            },
        },
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(report)
}

// Additional handlers would be implemented similarly...
```

## Database Schema

```sql
-- DNC Registries table
CREATE TABLE dnc_registries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    type INTEGER NOT NULL,
    jurisdiction VARCHAR(50) NOT NULL,
    status INTEGER NOT NULL DEFAULT 0,
    
    -- Sync tracking
    last_sync_at TIMESTAMP,
    next_sync_at TIMESTAMP NOT NULL,
    sync_interval INTERVAL NOT NULL DEFAULT '24 hours',
    
    -- API configuration
    api_endpoint VARCHAR(500),
    api_key_encrypted TEXT,
    rate_limit INTEGER DEFAULT 10,
    
    -- Statistics
    total_numbers BIGINT DEFAULT 0,
    last_updated TIMESTAMP,
    version VARCHAR(50),
    
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT uk_dnc_registry_name UNIQUE (name),
    INDEX idx_dnc_registry_type (type),
    INDEX idx_dnc_registry_status (status),
    INDEX idx_dnc_registry_next_sync (next_sync_at)
);

-- DNC Entries table (partitioned by phone number hash)
CREATE TABLE dnc_entries (
    id UUID DEFAULT gen_random_uuid(),
    phone_number VARCHAR(20) NOT NULL,
    phone_hash VARCHAR(64) NOT NULL, -- SHA256 for partitioning
    registry_id UUID NOT NULL,
    source INTEGER NOT NULL,
    
    -- Registration details
    registered_at TIMESTAMP NOT NULL,
    expires_at TIMESTAMP,
    
    -- Additional metadata
    consumer_name VARCHAR(255),
    notes TEXT,
    
    -- Audit fields
    added_by VARCHAR(255),
    added_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_verified_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    PRIMARY KEY (phone_hash, id),
    CONSTRAINT fk_dnc_entry_registry FOREIGN KEY (registry_id) REFERENCES dnc_registries(id),
    INDEX idx_dnc_entry_phone (phone_number),
    INDEX idx_dnc_entry_registry (registry_id),
    INDEX idx_dnc_entry_expires (expires_at)
) PARTITION BY HASH (phone_hash);

-- Create partitions (16 partitions for distribution)
CREATE TABLE dnc_entries_0 PARTITION OF dnc_entries FOR VALUES WITH (modulus 16, remainder 0);
CREATE TABLE dnc_entries_1 PARTITION OF dnc_entries FOR VALUES WITH (modulus 16, remainder 1);
-- ... continue for all 16 partitions

-- Suppression rules table
CREATE TABLE dnc_suppression_rules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    pattern VARCHAR(255) NOT NULL,
    type INTEGER NOT NULL,
    description TEXT,
    created_by VARCHAR(255),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT uk_suppression_pattern UNIQUE (pattern),
    INDEX idx_suppression_type (type)
);

-- DNC check audit log (for compliance reporting)
CREATE TABLE dnc_check_log (
    id BIGSERIAL PRIMARY KEY,
    phone_number VARCHAR(20) NOT NULL,
    on_dnc_list BOOLEAN NOT NULL,
    reasons JSONB,
    sources JSONB,
    checked_by UUID,
    checked_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    response_time_ms INTEGER,
    
    INDEX idx_dnc_check_phone (phone_number),
    INDEX idx_dnc_check_time (checked_at),
    INDEX idx_dnc_check_result (on_dnc_list)
) PARTITION BY RANGE (checked_at);

-- Create monthly partitions for audit log
CREATE TABLE dnc_check_log_2024_12 PARTITION OF dnc_check_log 
    FOR VALUES FROM ('2024-12-01') TO ('2025-01-01');
```

## Implementation Timeline

### Week 1: Core Domain and Database (Days 1-7)
- **Day 1-2**: Implement domain models (DNCRegistry, DNCEntry, SuppressionRule)
- **Day 3-4**: Create database schema and migrations
- **Day 5-6**: Implement repository interfaces and PostgreSQL implementations
- **Day 7**: Unit tests for domain and repositories

### Week 2: Federal API Integration (Days 8-14)
- **Day 8-9**: Implement Federal DNC API client with rate limiting
- **Day 10-11**: Build DNCImportService for bulk operations
- **Day 12-13**: Create sync mechanism and error handling
- **Day 14**: Integration tests with mock API

### Week 3: Caching Layer (Days 15-21)
- **Day 15-16**: Implement Redis caching strategy
- **Day 17-18**: Build DNCScrubbingService with cache integration
- **Day 19-20**: Add cache warming and invalidation logic
- **Day 21**: Performance testing and optimization

### Week 4: Testing and Optimization (Days 22-28)
- **Day 22-23**: REST API implementation and documentation
- **Day 24-25**: End-to-end testing with real phone numbers
- **Day 26**: Performance benchmarking and tuning
- **Day 27**: Compliance reporting implementation
- **Day 28**: Production deployment preparation

## Performance Optimization

### Caching Strategy
- **L1 Cache**: In-memory suppression rules (zero latency)
- **L2 Cache**: Redis with 24-hour TTL
- **L3 Cache**: PostgreSQL with partitioned tables

### Query Optimization
```sql
-- Optimized phone lookup with covering index
CREATE INDEX idx_dnc_entry_phone_covering ON dnc_entries(phone_number) 
    INCLUDE (registry_id, source, expires_at);

-- Batch check optimization
WITH phone_list AS (
    SELECT unnest(ARRAY['+14155551234', '+14155551235']) AS phone
)
SELECT p.phone, e.id IS NOT NULL as on_list
FROM phone_list p
LEFT JOIN dnc_entries e ON e.phone_number = p.phone
    AND (e.expires_at IS NULL OR e.expires_at > NOW());
```

### Benchmarks
```go
// Benchmark targets
// BenchmarkSingleCheck: < 1ms
// BenchmarkBulkCheck1000: < 50ms
// BenchmarkCacheHit: < 0.1ms
// BenchmarkSuppressionRule: < 0.01ms
```

## Monitoring and Alerts

### Key Metrics
- `dnc.check.latency_ms` - Check response time
- `dnc.check.cache_hit_rate` - Cache effectiveness
- `dnc.sync.success_rate` - Registry sync health
- `dnc.violations.detected` - Compliance violations found

### Alerts
- Registry sync failures
- API rate limit exceeded
- Cache hit rate < 90%
- Check latency > 100ms

## Security Considerations

1. **API Key Encryption**: Use AES-256 for storing registry API keys
2. **PII Protection**: Hash phone numbers in logs
3. **Access Control**: Role-based access to suppression management
4. **Audit Trail**: Complete history of all DNC operations
5. **Data Retention**: Comply with data retention policies

## Future Enhancements

1. **Machine Learning**: Predict DNC registration patterns
2. **Real-time Sync**: WebSocket integration for instant updates
3. **Multi-tenant Support**: Per-client suppression lists
4. **International Support**: Global DNC list integration
5. **Blockchain Audit**: Immutable compliance proof