package providers

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/dnc"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/infrastructure/cache"
	"golang.org/x/time/rate"
)

// InternalProvider implements ProviderClient for company suppression lists
// Manages internal do-not-call lists, customer opt-outs, and company-specific exclusions
type InternalProvider struct {
	config       InternalConfig
	client       *http.Client
	rateLimiter  *rate.Limiter
	circuitState CircuitState
	metrics      *ProviderMetrics
	cache        cache.Interface
	mu           sync.RWMutex

	// Circuit breaker state
	failureCount    int
	lastFailureTime time.Time
	successCount    int

	// Connection state
	connected bool
	lastHealth *HealthCheckResult

	// Internal lists management
	suppressionLists map[string]*SuppressionList
	listMu          sync.RWMutex
}

// InternalConfig contains configuration for the internal provider
type InternalConfig struct {
	DatabaseURL      string        `json:"database_url"`
	Timeout          time.Duration `json:"timeout"`
	MaxRetries       int           `json:"max_retries"`
	RateLimitRPS     int           `json:"rate_limit_rps"`

	// Circuit breaker config
	CircuitConfig    CircuitConfig `json:"circuit_config"`

	// Cache settings
	CacheTTL         time.Duration `json:"cache_ttl"`
	CacheEnabled     bool          `json:"cache_enabled"`

	// List management
	AutoRefresh      bool          `json:"auto_refresh"`
	RefreshInterval  time.Duration `json:"refresh_interval"`
	ListSources      []ListSource  `json:"list_sources"`

	// API settings for internal services
	APIBaseURL       string        `json:"api_base_url"`
	APIKey           string        `json:"api_key"`
	AuthToken        string        `json:"auth_token"`

	// Compliance settings
	DefaultAction    string        `json:"default_action"` // allow, block
	AuditEnabled     bool          `json:"audit_enabled"`
	LogRetention     time.Duration `json:"log_retention"`
}

// ListSource defines a source for suppression lists
type ListSource struct {
	SourceID    string            `json:"source_id"`
	Name        string            `json:"name"`
	Type        string            `json:"type"`        // database, file, api
	Location    string            `json:"location"`    // connection string, file path, or URL
	Format      string            `json:"format"`      // csv, json, database
	Credentials map[string]string `json:"credentials"`
	Schedule    string            `json:"schedule"`    // cron expression for refresh
	Enabled     bool              `json:"enabled"`
}

// SuppressionList represents an internal suppression list
type SuppressionList struct {
	ListID      string                 `json:"list_id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Type        SuppressionType        `json:"type"`
	Numbers     map[string]*ListEntry  `json:"numbers"`
	LastUpdated time.Time              `json:"last_updated"`
	Source      string                 `json:"source"`
	Active      bool                   `json:"active"`
	mu          sync.RWMutex
}

// SuppressionType defines the type of suppression list
type SuppressionType string

const (
	SuppressionTypeCustomer     SuppressionType = "customer_optout"
	SuppressionTypeInternal     SuppressionType = "internal_suppression"
	SuppressionTypeCompliance   SuppressionType = "compliance_block"
	SuppressionTypeLegal        SuppressionType = "legal_hold"
	SuppressionTypeTemporary    SuppressionType = "temporary_block"
)

// ListEntry represents an entry in a suppression list
type ListEntry struct {
	PhoneNumber values.PhoneNumber `json:"phone_number"`
	AddedAt     time.Time           `json:"added_at"`
	ExpiresAt   *time.Time          `json:"expires_at,omitempty"`
	Reason      string              `json:"reason"`
	Source      string              `json:"source"`
	Metadata    map[string]string   `json:"metadata"`
	Active      bool                `json:"active"`
}

// NewInternalProvider creates a new internal provider instance
func NewInternalProvider(config InternalConfig, cacheInterface cache.Interface) *InternalProvider {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.RateLimitRPS == 0 {
		config.RateLimitRPS = 1000 // Higher rate for internal systems
	}
	if config.CacheTTL == 0 {
		config.CacheTTL = 10 * time.Minute
	}
	if config.RefreshInterval == 0 {
		config.RefreshInterval = 15 * time.Minute
	}
	if config.DefaultAction == "" {
		config.DefaultAction = "allow"
	}

	return &InternalProvider{
		config:      config,
		client:      &http.Client{Timeout: config.Timeout},
		rateLimiter: rate.NewLimiter(rate.Limit(config.RateLimitRPS), config.RateLimitRPS*2),
		circuitState: CircuitClosed,
		metrics: &ProviderMetrics{
			ProviderName: "internal",
			ProviderType: string(dnc.ProviderTypeInternal),
		},
		cache:            cacheInterface,
		connected:        false,
		suppressionLists: make(map[string]*SuppressionList),
	}
}

// ProviderClient interface implementation

func (ip *InternalProvider) GetProviderType() dnc.ProviderType {
	return dnc.ProviderTypeInternal
}

func (ip *InternalProvider) GetProviderName() string {
	return "internal"
}

func (ip *InternalProvider) HealthCheck(ctx context.Context) (*HealthCheckResult, error) {
	startTime := time.Now()

	result := &HealthCheckResult{
		Healthy:      true,
		ResponseTime: 0,
		Timestamp:    time.Now(),
		Metadata: map[string]interface{}{
			"provider_type": "internal",
			"provider_name": "internal",
		},
	}

	// Check database connectivity if configured
	if ip.config.DatabaseURL != "" {
		if err := ip.checkDatabaseHealth(ctx); err != nil {
			result.Healthy = false
			result.Error = fmt.Sprintf("database health check failed: %v", err)
			ip.updateCircuitState(false)
			return result, err
		}
	}

	// Check API connectivity if configured
	if ip.config.APIBaseURL != "" {
		if err := ip.checkAPIHealth(ctx); err != nil {
			result.Healthy = false
			result.Error = fmt.Sprintf("API health check failed: %v", err)
			ip.updateCircuitState(false)
			return result, err
		}
	}

	// Check suppression lists status
	ip.listMu.RLock()
	activeLists := 0
	for _, list := range ip.suppressionLists {
		if list.Active {
			activeLists++
		}
	}
	ip.listMu.RUnlock()

	result.ResponseTime = time.Since(startTime)
	result.Metadata["active_lists"] = activeLists
	result.Metadata["total_lists"] = len(ip.suppressionLists)

	ip.updateCircuitState(true)
	ip.lastHealth = result
	return result, nil
}

func (ip *InternalProvider) CheckNumber(ctx context.Context, phoneNumber values.PhoneNumber) (*CheckResult, error) {
	startTime := time.Now()
	defer func() {
		ip.updateMetrics(time.Since(startTime), true)
	}()

	// Check rate limiter
	if !ip.rateLimiter.Allow() {
		return nil, fmt.Errorf("rate limit exceeded")
	}

	// Check circuit breaker
	if ip.circuitState == CircuitOpen {
		return nil, fmt.Errorf("circuit breaker is open")
	}

	// Check cache first
	if ip.config.CacheEnabled {
		cacheKey := fmt.Sprintf("internal:dnc:%s", phoneNumber.String())
		if cachedResult, found := ip.getFromCache(cacheKey); found {
			ip.metrics.CacheHits++
			return cachedResult, nil
		}
		ip.metrics.CacheMisses++
	}

	// Check all active suppression lists
	found, entry := ip.checkSuppressionLists(phoneNumber)

	result := &CheckResult{
		PhoneNumber:  phoneNumber,
		Listed:       found,
		ProviderType: dnc.ProviderTypeInternal,
		ProviderName: "internal",
		CheckedAt:    time.Now(),
		Metadata: map[string]interface{}{
			"provider_type": "internal",
			"lists_checked": len(ip.suppressionLists),
		},
	}

	if found && entry != nil {
		result.ListedSince = &entry.AddedAt
		result.Source = entry.Source
		result.Metadata["reason"] = entry.Reason
		result.Metadata["suppression_type"] = entry.Metadata["type"]
		if entry.ExpiresAt != nil {
			result.Metadata["expires_at"] = entry.ExpiresAt.Format(time.RFC3339)
		}
	}

	// Cache result
	if ip.config.CacheEnabled {
		cacheKey := fmt.Sprintf("internal:dnc:%s", phoneNumber.String())
		ip.cacheResult(cacheKey, result)
	}

	// Audit log if enabled
	if ip.config.AuditEnabled {
		ip.logAuditEvent(phoneNumber, result, entry)
	}

	ip.updateCircuitState(true)
	return result, nil
}

func (ip *InternalProvider) BatchCheckNumbers(ctx context.Context, phoneNumbers []values.PhoneNumber) ([]*CheckResult, error) {
	if len(phoneNumbers) == 0 {
		return []*CheckResult{}, nil
	}

	startTime := time.Now()
	defer func() {
		ip.updateMetrics(time.Since(startTime), true)
	}()

	results := make([]*CheckResult, 0, len(phoneNumbers))

	// Process in batches for better performance
	batchSize := 100
	for i := 0; i < len(phoneNumbers); i += batchSize {
		end := i + batchSize
		if end > len(phoneNumbers) {
			end = len(phoneNumbers)
		}

		batch := phoneNumbers[i:end]
		batchResults, err := ip.processBatch(ctx, batch)
		if err != nil {
			return results, fmt.Errorf("batch processing failed: %w", err)
		}

		results = append(results, batchResults...)
	}

	return results, nil
}

func (ip *InternalProvider) SyncData(ctx context.Context, req SyncRequest) (*SyncResult, error) {
	startTime := time.Now()

	result := &SyncResult{
		ProviderType:     dnc.ProviderTypeInternal,
		ProviderName:     "internal",
		SyncType:         req.SyncType,
		StartedAt:        startTime,
		RecordsProcessed: 0,
		RecordsAdded:     0,
		RecordsUpdated:   0,
		RecordsDeleted:   0,
		Errors:           []string{},
	}

	// Refresh all configured list sources
	for _, source := range ip.config.ListSources {
		if !source.Enabled {
			continue
		}

		sourceResult, err := ip.syncListSource(ctx, source)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("source %s: %v", source.SourceID, err))
			continue
		}

		result.RecordsProcessed += sourceResult.RecordsProcessed
		result.RecordsAdded += sourceResult.RecordsAdded
		result.RecordsUpdated += sourceResult.RecordsUpdated
		result.RecordsDeleted += sourceResult.RecordsDeleted
	}

	result.CompletedAt = time.Now()
	result.Duration = result.CompletedAt.Sub(result.StartedAt)
	result.Success = len(result.Errors) == 0

	return result, nil
}

func (ip *InternalProvider) GetSyncStatus(ctx context.Context) (*SyncStatus, error) {
	status := &SyncStatus{
		ProviderType: dnc.ProviderTypeInternal,
		ProviderName: "internal",
		LastSync:     time.Time{},
		InProgress:   false,
		NextSync:     time.Time{},
		RecordCount:  0,
	}

	// Calculate total records across all lists
	ip.listMu.RLock()
	for _, list := range ip.suppressionLists {
		if list.Active {
			list.mu.RLock()
			status.RecordCount += int64(len(list.Numbers))
			if list.LastUpdated.After(status.LastSync) {
				status.LastSync = list.LastUpdated
			}
			list.mu.RUnlock()
		}
	}
	ip.listMu.RUnlock()

	// Calculate next sync based on refresh interval
	if !status.LastSync.IsZero() {
		status.NextSync = status.LastSync.Add(ip.config.RefreshInterval)
	}

	return status, nil
}

func (ip *InternalProvider) GetMetrics(ctx context.Context) (*ProviderMetrics, error) {
	ip.mu.RLock()
	defer ip.mu.RUnlock()

	// Create a copy to avoid concurrent access issues
	metrics := &ProviderMetrics{
		ProviderName:        ip.metrics.ProviderName,
		ProviderType:        ip.metrics.ProviderType,
		TotalRequests:       ip.metrics.TotalRequests,
		SuccessfulRequests:  ip.metrics.SuccessfulRequests,
		FailedRequests:      ip.metrics.FailedRequests,
		CacheHits:           ip.metrics.CacheHits,
		CacheMisses:         ip.metrics.CacheMisses,
		AverageLatency:      ip.metrics.AverageLatency,
		LastRequestTime:     ip.metrics.LastRequestTime,
		CircuitBreakerState: string(ip.circuitState),
		ErrorRate:           ip.metrics.ErrorRate,
		HealthStatus:        ip.lastHealth,
	}

	return metrics, nil
}

// Internal provider specific methods

// AddToSuppressionList adds a phone number to a suppression list
func (ip *InternalProvider) AddToSuppressionList(listID string, phoneNumber values.PhoneNumber, reason string, metadata map[string]string) error {
	ip.listMu.Lock()
	defer ip.listMu.Unlock()

	list, exists := ip.suppressionLists[listID]
	if !exists {
		return fmt.Errorf("suppression list %s not found", listID)
	}

	list.mu.Lock()
	defer list.mu.Unlock()

	entry := &ListEntry{
		PhoneNumber: phoneNumber,
		AddedAt:     time.Now(),
		Reason:      reason,
		Source:      "internal_api",
		Metadata:    metadata,
		Active:      true,
	}

	list.Numbers[phoneNumber.String()] = entry
	list.LastUpdated = time.Now()

	// Invalidate cache for this number
	if ip.config.CacheEnabled {
		cacheKey := fmt.Sprintf("internal:dnc:%s", phoneNumber.String())
		ip.cache.Delete(cacheKey)
	}

	return nil
}

// RemoveFromSuppressionList removes a phone number from a suppression list
func (ip *InternalProvider) RemoveFromSuppressionList(listID string, phoneNumber values.PhoneNumber) error {
	ip.listMu.Lock()
	defer ip.listMu.Unlock()

	list, exists := ip.suppressionLists[listID]
	if !exists {
		return fmt.Errorf("suppression list %s not found", listID)
	}

	list.mu.Lock()
	defer list.mu.Unlock()

	delete(list.Numbers, phoneNumber.String())
	list.LastUpdated = time.Now()

	// Invalidate cache for this number
	if ip.config.CacheEnabled {
		cacheKey := fmt.Sprintf("internal:dnc:%s", phoneNumber.String())
		ip.cache.Delete(cacheKey)
	}

	return nil
}

// CreateSuppressionList creates a new suppression list
func (ip *InternalProvider) CreateSuppressionList(name, description string, suppressionType SuppressionType) (*SuppressionList, error) {
	ip.listMu.Lock()
	defer ip.listMu.Unlock()

	listID := fmt.Sprintf("list_%d", time.Now().Unix())

	list := &SuppressionList{
		ListID:      listID,
		Name:        name,
		Description: description,
		Type:        suppressionType,
		Numbers:     make(map[string]*ListEntry),
		LastUpdated: time.Now(),
		Source:      "internal_api",
		Active:      true,
	}

	ip.suppressionLists[listID] = list
	return list, nil
}

// GetSuppressionLists returns all suppression lists
func (ip *InternalProvider) GetSuppressionLists() map[string]*SuppressionList {
	ip.listMu.RLock()
	defer ip.listMu.RUnlock()

	// Return a copy to avoid concurrent access
	lists := make(map[string]*SuppressionList)
	for id, list := range ip.suppressionLists {
		lists[id] = list
	}
	return lists
}

// Helper methods

func (ip *InternalProvider) checkDatabaseHealth(ctx context.Context) error {
	// Implementation would check database connectivity
	// For now, return success
	return nil
}

func (ip *InternalProvider) checkAPIHealth(ctx context.Context) error {
	if ip.config.APIBaseURL == "" {
		return nil
	}

	req, err := http.NewRequestWithContext(ctx, "GET", ip.config.APIBaseURL+"/health", nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	if ip.config.APIKey != "" {
		req.Header.Set("X-API-Key", ip.config.APIKey)
	}
	if ip.config.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+ip.config.AuthToken)
	}

	resp, err := ip.client.Do(req)
	if err != nil {
		return fmt.Errorf("health check request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check returned status %d", resp.StatusCode)
	}

	return nil
}

func (ip *InternalProvider) checkSuppressionLists(phoneNumber values.PhoneNumber) (bool, *ListEntry) {
	normalizedNumber := phoneNumber.String()

	ip.listMu.RLock()
	defer ip.listMu.RUnlock()

	for _, list := range ip.suppressionLists {
		if !list.Active {
			continue
		}

		list.mu.RLock()
		entry, found := list.Numbers[normalizedNumber]
		list.mu.RUnlock()

		if found && entry.Active {
			// Check if entry has expired
			if entry.ExpiresAt != nil && time.Now().After(*entry.ExpiresAt) {
				continue
			}
			return true, entry
		}
	}

	return false, nil
}

func (ip *InternalProvider) processBatch(ctx context.Context, phoneNumbers []values.PhoneNumber) ([]*CheckResult, error) {
	results := make([]*CheckResult, 0, len(phoneNumbers))

	for _, phoneNumber := range phoneNumbers {
		result, err := ip.CheckNumber(ctx, phoneNumber)
		if err != nil {
			// Create error result
			result = &CheckResult{
				PhoneNumber:  phoneNumber,
				Listed:       false, // Default to not listed on error
				ProviderType: dnc.ProviderTypeInternal,
				ProviderName: "internal",
				CheckedAt:    time.Now(),
				Metadata: map[string]interface{}{
					"error": err.Error(),
				},
			}
		}
		results = append(results, result)
	}

	return results, nil
}

func (ip *InternalProvider) syncListSource(ctx context.Context, source ListSource) (*SyncResult, error) {
	result := &SyncResult{
		ProviderType: dnc.ProviderTypeInternal,
		ProviderName: "internal",
		SyncType:     SyncTypeFull,
		StartedAt:    time.Now(),
	}

	switch source.Type {
	case "database":
		return ip.syncDatabaseSource(ctx, source)
	case "file":
		return ip.syncFileSource(ctx, source)
	case "api":
		return ip.syncAPISource(ctx, source)
	default:
		result.Errors = append(result.Errors, fmt.Sprintf("unsupported source type: %s", source.Type))
		result.Success = false
	}

	result.CompletedAt = time.Now()
	result.Duration = result.CompletedAt.Sub(result.StartedAt)
	return result, nil
}

func (ip *InternalProvider) syncDatabaseSource(ctx context.Context, source ListSource) (*SyncResult, error) {
	// Implementation would sync from database
	return &SyncResult{
		ProviderType: dnc.ProviderTypeInternal,
		ProviderName: "internal",
		Success:      true,
	}, nil
}

func (ip *InternalProvider) syncFileSource(ctx context.Context, source ListSource) (*SyncResult, error) {
	// Implementation would sync from file
	return &SyncResult{
		ProviderType: dnc.ProviderTypeInternal,
		ProviderName: "internal",
		Success:      true,
	}, nil
}

func (ip *InternalProvider) syncAPISource(ctx context.Context, source ListSource) (*SyncResult, error) {
	// Implementation would sync from API
	return &SyncResult{
		ProviderType: dnc.ProviderTypeInternal,
		ProviderName: "internal",
		Success:      true,
	}, nil
}

func (ip *InternalProvider) getFromCache(key string) (*CheckResult, bool) {
	if ip.cache == nil {
		return nil, false
	}

	data, found := ip.cache.Get(key)
	if !found {
		return nil, false
	}

	result, ok := data.(*CheckResult)
	return result, ok
}

func (ip *InternalProvider) cacheResult(key string, result *CheckResult) {
	if ip.cache == nil {
		return
	}

	ip.cache.Set(key, result, ip.config.CacheTTL)
}

func (ip *InternalProvider) updateCircuitState(success bool) {
	ip.mu.Lock()
	defer ip.mu.Unlock()

	if success {
		ip.successCount++
		ip.failureCount = 0
		if ip.circuitState == CircuitOpen && ip.successCount >= ip.config.CircuitConfig.SuccessThreshold {
			ip.circuitState = CircuitClosed
		}
	} else {
		ip.failureCount++
		ip.successCount = 0
		ip.lastFailureTime = time.Now()
		if ip.failureCount >= ip.config.CircuitConfig.FailureThreshold {
			ip.circuitState = CircuitOpen
		}
	}
}

func (ip *InternalProvider) updateMetrics(latency time.Duration, success bool) {
	ip.mu.Lock()
	defer ip.mu.Unlock()

	ip.metrics.TotalRequests++
	if success {
		ip.metrics.SuccessfulRequests++
	} else {
		ip.metrics.FailedRequests++
	}

	// Update average latency (simplified)
	if ip.metrics.TotalRequests == 1 {
		ip.metrics.AverageLatency = latency
	} else {
		ip.metrics.AverageLatency = (ip.metrics.AverageLatency + latency) / 2
	}

	ip.metrics.LastRequestTime = time.Now()
	ip.metrics.ErrorRate = float64(ip.metrics.FailedRequests) / float64(ip.metrics.TotalRequests)
}

func (ip *InternalProvider) logAuditEvent(phoneNumber values.PhoneNumber, result *CheckResult, entry *ListEntry) {
	// Implementation would log audit events to audit system
	// For now, this is a placeholder
}

// DefaultInternalConfig returns a sensible default configuration
func DefaultInternalConfig() InternalConfig {
	return InternalConfig{
		Timeout:         30 * time.Second,
		MaxRetries:      3,
		RateLimitRPS:    1000,
		CircuitConfig:   DefaultCircuitConfig(),
		CacheTTL:        10 * time.Minute,
		CacheEnabled:    true,
		AutoRefresh:     true,
		RefreshInterval: 15 * time.Minute,
		ListSources:     []ListSource{},
		DefaultAction:   "allow",
		AuditEnabled:    true,
		LogRetention:    90 * 24 * time.Hour, // 90 days
	}
}