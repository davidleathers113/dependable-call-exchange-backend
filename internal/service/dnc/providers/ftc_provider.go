package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/dnc"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/infrastructure/cache"
	"golang.org/x/time/rate"
)

// FTCProvider implements ProviderClient for the Federal Trade Commission DNC Registry
// Provides access to the National Do Not Call Registry via FTC APIs
type FTCProvider struct {
	config       FTCConfig
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
}

// FTCConfig contains configuration for the FTC provider
type FTCConfig struct {
	BaseURL          string        `json:"base_url"`
	APIKey           string        `json:"api_key"`
	APISecret        string        `json:"api_secret,omitempty"`
	Timeout          time.Duration `json:"timeout"`
	MaxRetries       int           `json:"max_retries"`
	RateLimitRPS     int           `json:"rate_limit_rps"`
	
	// Circuit breaker config
	CircuitConfig    CircuitConfig `json:"circuit_config"`
	
	// Cache settings
	CacheTTL         time.Duration `json:"cache_ttl"`
	CacheEnabled     bool          `json:"cache_enabled"`
	
	// FTC-specific settings
	Version          string        `json:"version"`
	Format           string        `json:"format"` // json, xml, csv
	IncludeMetadata  bool          `json:"include_metadata"`
	BatchSize        int           `json:"batch_size"`
}

// NewFTCProvider creates a new FTC provider instance
func NewFTCProvider(config FTCConfig, cacheInterface cache.Interface) *FTCProvider {
	if config.BaseURL == "" {
		config.BaseURL = "https://api.donotcall.gov"
	}
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.RateLimitRPS == 0 {
		config.RateLimitRPS = 10 // Conservative default
	}
	if config.BatchSize == 0 {
		config.BatchSize = 100
	}
	if config.Format == "" {
		config.Format = "json"
	}
	if config.Version == "" {
		config.Version = "v1"
	}
	if config.CacheTTL == 0 {
		config.CacheTTL = 4 * time.Hour // FTC data changes infrequently
	}
	
	// Set default circuit breaker config
	if config.CircuitConfig.FailureThreshold == 0 {
		config.CircuitConfig.FailureThreshold = 5
	}
	if config.CircuitConfig.RecoveryTimeout == 0 {
		config.CircuitConfig.RecoveryTimeout = 30 * time.Second
	}
	if config.CircuitConfig.SuccessThreshold == 0 {
		config.CircuitConfig.SuccessThreshold = 3
	}
	if config.CircuitConfig.HalfOpenMaxCalls == 0 {
		config.CircuitConfig.HalfOpenMaxCalls = 5
	}

	httpClient := &http.Client{
		Timeout: config.Timeout,
		Transport: &http.Transport{
			MaxIdleConns:        10,
			MaxIdleConnsPerHost: 5,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	return &FTCProvider{
		config:       config,
		client:       httpClient,
		rateLimiter:  rate.NewLimiter(rate.Limit(config.RateLimitRPS), config.RateLimitRPS*2), // 2x burst
		circuitState: CircuitClosed,
		cache:        cacheInterface,
		metrics: &ProviderMetrics{
			ProviderName: "FTC",
		},
	}
}

// GetProviderType returns the provider type
func (f *FTCProvider) GetProviderType() dnc.ProviderType {
	return dnc.ProviderTypeFederal
}

// GetProviderName returns the provider name
func (f *FTCProvider) GetProviderName() string {
	return "FTC"
}

// HealthCheck performs a health check against the FTC API
func (f *FTCProvider) HealthCheck(ctx context.Context) (*HealthCheckResult, error) {
	start := time.Now()
	result := &HealthCheckResult{
		LastUpdated: start,
		Metadata:    make(map[string]string),
	}

	// Check circuit breaker state
	if f.circuitState == CircuitOpen {
		result.IsHealthy = false
		result.Error = "Circuit breaker is open"
		result.Connectivity = false
		return result, nil
	}

	// Test endpoint availability
	healthURL := fmt.Sprintf("%s/%s/health", f.config.BaseURL, f.config.Version)
	req, err := http.NewRequestWithContext(ctx, "GET", healthURL, nil)
	if err != nil {
		result.IsHealthy = false
		result.Error = fmt.Sprintf("Failed to create request: %v", err)
		return result, err
	}

	f.addAuthHeaders(req)
	
	resp, err := f.client.Do(req)
	if err != nil {
		result.IsHealthy = false
		result.Error = fmt.Sprintf("Request failed: %v", err)
		result.Connectivity = false
		f.RecordFailure(err)
		return result, err
	}
	defer resp.Body.Close()

	result.ResponseTime = time.Since(start)
	result.StatusCode = resp.StatusCode
	result.Connectivity = true

	// Check authentication
	if resp.StatusCode == http.StatusUnauthorized {
		result.Authentication = false
		result.Error = "Authentication failed"
		err := &ProviderError{
			Code:     ErrCodeAuthenticationFailed,
			Message:  "FTC API authentication failed",
			Provider: "FTC",
			Retry:    false,
		}
		f.RecordFailure(err)
		return result, err
	}

	// Check for successful response
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		result.IsHealthy = true
		result.Authentication = true
		result.DataAvailable = true
		result.RateLimit = true
		result.Metadata["api_version"] = f.config.Version
		result.Metadata["format"] = f.config.Format
		f.RecordSuccess()
	} else {
		result.IsHealthy = false
		result.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
		if resp.StatusCode == 429 {
			result.RateLimit = false
		}
		err := &ProviderError{
			Code:     ErrCodeProviderUnavailable,
			Message:  fmt.Sprintf("FTC API returned HTTP %d", resp.StatusCode),
			Provider: "FTC",
			Retry:    resp.StatusCode >= 500,
		}
		f.RecordFailure(err)
		return result, err
	}

	f.lastHealth = result
	return result, nil
}

// CheckNumber checks if a phone number is on the FTC DNC registry
func (f *FTCProvider) CheckNumber(ctx context.Context, phoneNumber string) (*CheckResult, error) {
	// Check circuit breaker
	if f.circuitState == CircuitOpen {
		return nil, &ProviderError{
			Code:     ErrCodeProviderUnavailable,
			Message:  "Circuit breaker is open",
			Provider: "FTC",
			Retry:    true,
		}
	}

	// Normalize phone number
	normalizedNumber := f.normalizePhoneNumber(phoneNumber)
	if normalizedNumber == "" {
		return nil, &ProviderError{
			Code:     ErrCodeInvalidRequest,
			Message:  "Invalid phone number format",
			Provider: "FTC",
			Retry:    false,
		}
	}

	// Check cache first
	if f.config.CacheEnabled && f.cache != nil {
		cacheKey := fmt.Sprintf("ftc:check:%s", normalizedNumber)
		if cached, err := f.cache.Get(ctx, cacheKey); err == nil && cached != nil {
			var result CheckResult
			if err := json.Unmarshal(cached, &result); err == nil {
				f.metrics.RequestCount++
				return &result, nil
			}
		}
	}

	// Rate limiting
	if err := f.rateLimiter.Wait(ctx); err != nil {
		return nil, &ProviderError{
			Code:     ErrCodeRateLimitExceeded,
			Message:  "Rate limit exceeded",
			Provider: "FTC",
			Retry:    true,
		}
	}

	start := time.Now()
	f.metrics.RequestCount++

	// Build request URL
	checkURL := fmt.Sprintf("%s/%s/check", f.config.BaseURL, f.config.Version)
	params := url.Values{}
	params.Add("number", normalizedNumber)
	params.Add("format", f.config.Format)
	if f.config.IncludeMetadata {
		params.Add("metadata", "true")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", checkURL+"?"+params.Encode(), nil)
	if err != nil {
		f.RecordFailure(err)
		return nil, &ProviderError{
			Code:     ErrCodeInvalidRequest,
			Message:  fmt.Sprintf("Failed to create request: %v", err),
			Provider: "FTC",
			Retry:    false,
		}
	}

	f.addAuthHeaders(req)

	resp, err := f.client.Do(req)
	if err != nil {
		f.RecordFailure(err)
		return nil, &ProviderError{
			Code:     ErrCodeConnectionFailed,
			Message:  fmt.Sprintf("Request failed: %v", err),
			Provider: "FTC",
			Retry:    true,
		}
	}
	defer resp.Body.Close()

	duration := time.Since(start)
	f.updateResponseTimeMetrics(duration)

	// Handle error responses
	if resp.StatusCode != http.StatusOK {
		err := f.handleHTTPError(resp)
		f.RecordFailure(err)
		return nil, err
	}

	// Parse response
	var apiResponse FTCCheckResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		f.RecordFailure(err)
		return nil, &ProviderError{
			Code:     ErrCodeInvalidResponse,
			Message:  fmt.Sprintf("Failed to parse response: %v", err),
			Provider: "FTC",
			Retry:    false,
		}
	}

	// Convert to standard format
	result := &CheckResult{
		PhoneNumber: normalizedNumber,
		IsListed:    apiResponse.IsListed,
		ListSource:  "FTC",
		LastUpdated: time.Now(),
		Confidence:  1.0, // FTC is authoritative
		Metadata:    make(map[string]string),
	}

	if apiResponse.RegistrationDate != "" {
		if regDate, err := time.Parse("2006-01-02", apiResponse.RegistrationDate); err == nil {
			result.RegistrationDate = &regDate
		}
	}

	if apiResponse.ExpirationDate != "" {
		if expDate, err := time.Parse("2006-01-02", apiResponse.ExpirationDate); err == nil {
			result.ExpiresAt = &expDate
		}
	}

	result.Reason = apiResponse.Reason
	result.Metadata["provider_id"] = apiResponse.ID
	result.Metadata["list_type"] = apiResponse.ListType

	// Cache the result
	if f.config.CacheEnabled && f.cache != nil {
		cacheKey := fmt.Sprintf("ftc:check:%s", normalizedNumber)
		if data, err := json.Marshal(result); err == nil {
			f.cache.Set(ctx, cacheKey, data, f.config.CacheTTL)
		}
	}

	f.RecordSuccess()
	f.metrics.SuccessCount++
	return result, nil
}

// BatchCheckNumbers checks multiple phone numbers in a single request
func (f *FTCProvider) BatchCheckNumbers(ctx context.Context, phoneNumbers []string) ([]*CheckResult, error) {
	if len(phoneNumbers) == 0 {
		return []*CheckResult{}, nil
	}

	// Check circuit breaker
	if f.circuitState == CircuitOpen {
		return nil, &ProviderError{
			Code:     ErrCodeProviderUnavailable,
			Message:  "Circuit breaker is open",
			Provider: "FTC",
			Retry:    true,
		}
	}

	// Process in batches
	var allResults []*CheckResult
	for i := 0; i < len(phoneNumbers); i += f.config.BatchSize {
		end := i + f.config.BatchSize
		if end > len(phoneNumbers) {
			end = len(phoneNumbers)
		}

		batch := phoneNumbers[i:end]
		results, err := f.processBatch(ctx, batch)
		if err != nil {
			return allResults, err
		}

		allResults = append(allResults, results...)
	}

	return allResults, nil
}

// GetIncrementalUpdates retrieves incremental updates from the FTC registry
func (f *FTCProvider) GetIncrementalUpdates(ctx context.Context, since time.Time) (*SyncResult, error) {
	if f.circuitState == CircuitOpen {
		return nil, &ProviderError{
			Code:     ErrCodeProviderUnavailable,
			Message:  "Circuit breaker is open",
			Provider: "FTC",
			Retry:    true,
		}
	}

	start := time.Now()
	result := &SyncResult{
		ProviderName: "FTC",
		StartedAt:   start,
		Status:      "started",
	}

	// Build incremental sync request
	syncURL := fmt.Sprintf("%s/%s/updates", f.config.BaseURL, f.config.Version)
	params := url.Values{}
	params.Add("since", since.Format(time.RFC3339))
	params.Add("format", f.config.Format)
	params.Add("limit", strconv.Itoa(f.config.BatchSize))

	req, err := http.NewRequestWithContext(ctx, "GET", syncURL+"?"+params.Encode(), nil)
	if err != nil {
		result.Status = "failed"
		result.CompletedAt = time.Now()
		result.Duration = time.Since(start)
		result.Errors = append(result.Errors, SyncError{
			Code:      ErrCodeInvalidRequest,
			Message:   fmt.Sprintf("Failed to create request: %v", err),
			Timestamp: time.Now(),
		})
		f.RecordFailure(err)
		return result, err
	}

	f.addAuthHeaders(req)

	resp, err := f.client.Do(req)
	if err != nil {
		result.Status = "failed"
		result.CompletedAt = time.Now()
		result.Duration = time.Since(start)
		result.Errors = append(result.Errors, SyncError{
			Code:      ErrCodeConnectionFailed,
			Message:   fmt.Sprintf("Request failed: %v", err),
			Timestamp: time.Now(),
		})
		f.RecordFailure(err)
		return result, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err := f.handleHTTPError(resp)
		result.Status = "failed"
		result.CompletedAt = time.Now()
		result.Duration = time.Since(start)
		result.Errors = append(result.Errors, SyncError{
			Code:      err.(*ProviderError).Code,
			Message:   err.Error(),
			Timestamp: time.Now(),
		})
		f.RecordFailure(err)
		return result, err
	}

	var syncResponse FTCSyncResponse
	if err := json.NewDecoder(resp.Body).Decode(&syncResponse); err != nil {
		result.Status = "failed"
		result.CompletedAt = time.Now()
		result.Duration = time.Since(start)
		result.Errors = append(result.Errors, SyncError{
			Code:      ErrCodeInvalidResponse,
			Message:   fmt.Sprintf("Failed to parse response: %v", err),
			Timestamp: time.Now(),
		})
		f.RecordFailure(err)
		return result, err
	}

	// Process the sync data
	result.RecordsProcessed = len(syncResponse.Records)
	result.RecordsAdded = syncResponse.Added
	result.RecordsUpdated = syncResponse.Updated
	result.RecordsDeleted = syncResponse.Deleted
	result.DataVersion = syncResponse.Version
	result.Checkpoint = syncResponse.NextCheckpoint
	
	if syncResponse.NextSync != "" {
		if nextSync, err := time.Parse(time.RFC3339, syncResponse.NextSync); err == nil {
			result.NextSync = &nextSync
		}
	}

	result.Status = "success"
	result.CompletedAt = time.Now()
	result.Duration = time.Since(start)
	
	if result.Duration.Seconds() > 0 {
		result.ThroughputPerSecond = float64(result.RecordsProcessed) / result.Duration.Seconds()
	}

	f.RecordSuccess()
	return result, nil
}

// GetFullSnapshot retrieves a full snapshot of the FTC registry
func (f *FTCProvider) GetFullSnapshot(ctx context.Context) (*SyncResult, error) {
	// This would typically be a large operation that might use streaming
	// For now, we'll implement a simplified version
	return f.GetIncrementalUpdates(ctx, time.Time{})
}

// ValidateConfig validates the FTC provider configuration
func (f *FTCProvider) ValidateConfig(config map[string]string) error {
	required := []string{"api_key", "base_url"}
	for _, key := range required {
		if _, exists := config[key]; !exists {
			return &ProviderError{
				Code:     ErrCodeConfigurationError,
				Message:  fmt.Sprintf("Missing required configuration: %s", key),
				Provider: "FTC",
				Retry:    false,
			}
		}
	}

	// Validate URL format
	if baseURL, exists := config["base_url"]; exists {
		if _, err := url.Parse(baseURL); err != nil {
			return &ProviderError{
				Code:     ErrCodeConfigurationError,
				Message:  "Invalid base_url format",
				Provider: "FTC",
				Retry:    false,
			}
		}
	}

	return nil
}

// SetConfig updates the provider configuration
func (f *FTCProvider) SetConfig(config map[string]string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.ValidateConfig(config); err != nil {
		return err
	}

	// Update configuration
	if apiKey, exists := config["api_key"]; exists {
		f.config.APIKey = apiKey
	}
	if baseURL, exists := config["base_url"]; exists {
		f.config.BaseURL = baseURL
	}
	if timeout, exists := config["timeout"]; exists {
		if d, err := time.ParseDuration(timeout); err == nil {
			f.config.Timeout = d
			f.client.Timeout = d
		}
	}

	return nil
}

// GetRateLimit returns the current rate limit configuration
func (f *FTCProvider) GetRateLimit() RateLimit {
	return RateLimit{
		RequestsPerSecond: f.config.RateLimitRPS,
		RequestsPerMinute: f.config.RateLimitRPS * 60,
		RequestsPerHour:   f.config.RateLimitRPS * 3600,
		RequestsPerDay:    f.config.RateLimitRPS * 86400,
		BurstSize:         f.config.RateLimitRPS * 2,
	}
}

// GetQuotaStatus returns the current quota status (if applicable)
func (f *FTCProvider) GetQuotaStatus(ctx context.Context) (*QuotaStatus, error) {
	// FTC doesn't typically have quotas, but we can return a placeholder
	return &QuotaStatus{
		Used:      f.metrics.RequestCount,
		Limit:     -1, // Unlimited
		Remaining: -1, // Unlimited
		ResetTime: time.Now().Add(24 * time.Hour),
		Period:    "day",
	}, nil
}

// Connect establishes connection to the FTC provider
func (f *FTCProvider) Connect(ctx context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	health, err := f.HealthCheck(ctx)
	if err != nil {
		return err
	}

	if !health.IsHealthy {
		return &ProviderError{
			Code:     ErrCodeConnectionFailed,
			Message:  "Health check failed",
			Provider: "FTC",
			Retry:    true,
		}
	}

	f.connected = true
	return nil
}

// Disconnect disconnects from the FTC provider
func (f *FTCProvider) Disconnect(ctx context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.connected = false
	return nil
}

// IsConnected returns the connection status
func (f *FTCProvider) IsConnected() bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.connected
}

// Circuit breaker methods

// GetCircuitState returns the current circuit breaker state
func (f *FTCProvider) GetCircuitState() CircuitState {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.circuitState
}

// ResetCircuit resets the circuit breaker to closed state
func (f *FTCProvider) ResetCircuit() error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.circuitState = CircuitClosed
	f.failureCount = 0
	f.successCount = 0
	f.lastFailureTime = time.Time{}
	return nil
}

// RecordSuccess records a successful operation
func (f *FTCProvider) RecordSuccess() {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.successCount++
	f.metrics.SuccessCount++

	// Transition from half-open to closed if we have enough successes
	if f.circuitState == CircuitHalfOpen && f.successCount >= f.config.CircuitConfig.SuccessThreshold {
		f.circuitState = CircuitClosed
		f.failureCount = 0
	}
}

// RecordFailure records a failed operation
func (f *FTCProvider) RecordFailure(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.failureCount++
	f.lastFailureTime = time.Now()
	f.metrics.ErrorCount++

	// Transition to open if failure threshold is reached
	if f.circuitState == CircuitClosed && f.failureCount >= f.config.CircuitConfig.FailureThreshold {
		f.circuitState = CircuitOpen
		f.metrics.CircuitOpenCount++
	}

	// Transition from half-open back to open on any failure
	if f.circuitState == CircuitHalfOpen {
		f.circuitState = CircuitOpen
		f.metrics.CircuitOpenCount++
	}
}

// SetCircuitConfig updates the circuit breaker configuration
func (f *FTCProvider) SetCircuitConfig(config CircuitConfig) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.config.CircuitConfig = config
	return nil
}

// Helper methods

// normalizePhoneNumber normalizes a phone number to E.164 format
func (f *FTCProvider) normalizePhoneNumber(phoneNumber string) string {
	// Remove all non-digit characters
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, phoneNumber)

	// Handle US/Canada numbers
	if len(digits) == 10 {
		return "+1" + digits
	}
	if len(digits) == 11 && digits[0] == '1' {
		return "+" + digits
	}

	// Return as-is if it looks like international format
	if len(digits) > 7 {
		return "+" + digits
	}

	return "" // Invalid
}

// addAuthHeaders adds authentication headers to the request
func (f *FTCProvider) addAuthHeaders(req *http.Request) {
	if f.config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+f.config.APIKey)
	}
	req.Header.Set("User-Agent", "DCE-DNC-Client/1.0")
	req.Header.Set("Accept", "application/json")
}

// handleHTTPError converts HTTP errors to provider errors
func (f *FTCProvider) handleHTTPError(resp *http.Response) error {
	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return &ProviderError{
			Code:     ErrCodeAuthenticationFailed,
			Message:  "Authentication failed",
			Provider: "FTC",
			Retry:    false,
		}
	case http.StatusTooManyRequests:
		return &ProviderError{
			Code:     ErrCodeRateLimitExceeded,
			Message:  "Rate limit exceeded",
			Provider: "FTC",
			Retry:    true,
		}
	case http.StatusBadRequest:
		return &ProviderError{
			Code:     ErrCodeInvalidRequest,
			Message:  "Bad request",
			Provider: "FTC",
			Retry:    false,
		}
	case http.StatusServiceUnavailable:
		return &ProviderError{
			Code:     ErrCodeProviderUnavailable,
			Message:  "Service unavailable",
			Provider: "FTC",
			Retry:    true,
		}
	default:
		return &ProviderError{
			Code:     ErrCodeProviderUnavailable,
			Message:  fmt.Sprintf("HTTP %d", resp.StatusCode),
			Provider: "FTC",
			Retry:    resp.StatusCode >= 500,
		}
	}
}

// processBatch processes a batch of phone numbers
func (f *FTCProvider) processBatch(ctx context.Context, phoneNumbers []string) ([]*CheckResult, error) {
	start := time.Now()
	f.metrics.RequestCount++

	// Normalize numbers
	normalizedNumbers := make([]string, 0, len(phoneNumbers))
	for _, num := range phoneNumbers {
		if normalized := f.normalizePhoneNumber(num); normalized != "" {
			normalizedNumbers = append(normalizedNumbers, normalized)
		}
	}

	// Rate limiting
	if err := f.rateLimiter.Wait(ctx); err != nil {
		return nil, &ProviderError{
			Code:     ErrCodeRateLimitExceeded,
			Message:  "Rate limit exceeded",
			Provider: "FTC",
			Retry:    true,
		}
	}

	// Build batch request
	batchURL := fmt.Sprintf("%s/%s/batch-check", f.config.BaseURL, f.config.Version)
	requestBody := FTCBatchRequest{
		Numbers: normalizedNumbers,
		Format:  f.config.Format,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		f.RecordFailure(err)
		return nil, &ProviderError{
			Code:     ErrCodeInvalidRequest,
			Message:  fmt.Sprintf("Failed to marshal request: %v", err),
			Provider: "FTC",
			Retry:    false,
		}
	}

	req, err := http.NewRequestWithContext(ctx, "POST", batchURL, strings.NewReader(string(jsonData)))
	if err != nil {
		f.RecordFailure(err)
		return nil, &ProviderError{
			Code:     ErrCodeInvalidRequest,
			Message:  fmt.Sprintf("Failed to create request: %v", err),
			Provider: "FTC",
			Retry:    false,
		}
	}

	f.addAuthHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := f.client.Do(req)
	if err != nil {
		f.RecordFailure(err)
		return nil, &ProviderError{
			Code:     ErrCodeConnectionFailed,
			Message:  fmt.Sprintf("Request failed: %v", err),
			Provider: "FTC",
			Retry:    true,
		}
	}
	defer resp.Body.Close()

	duration := time.Since(start)
	f.updateResponseTimeMetrics(duration)

	if resp.StatusCode != http.StatusOK {
		err := f.handleHTTPError(resp)
		f.RecordFailure(err)
		return nil, err
	}

	var batchResponse FTCBatchResponse
	if err := json.NewDecoder(resp.Body).Decode(&batchResponse); err != nil {
		f.RecordFailure(err)
		return nil, &ProviderError{
			Code:     ErrCodeInvalidResponse,
			Message:  fmt.Sprintf("Failed to parse response: %v", err),
			Provider: "FTC",
			Retry:    false,
		}
	}

	// Convert results
	results := make([]*CheckResult, len(batchResponse.Results))
	for i, result := range batchResponse.Results {
		checkResult := &CheckResult{
			PhoneNumber: result.PhoneNumber,
			IsListed:    result.IsListed,
			ListSource:  "FTC",
			LastUpdated: time.Now(),
			Confidence:  1.0,
			Metadata:    make(map[string]string),
		}

		if result.RegistrationDate != "" {
			if regDate, err := time.Parse("2006-01-02", result.RegistrationDate); err == nil {
				checkResult.RegistrationDate = &regDate
			}
		}

		checkResult.Reason = result.Reason
		checkResult.Metadata["provider_id"] = result.ID
		checkResult.Metadata["list_type"] = result.ListType

		results[i] = checkResult
	}

	f.RecordSuccess()
	f.metrics.SuccessCount++
	return results, nil
}

// updateResponseTimeMetrics updates response time metrics
func (f *FTCProvider) updateResponseTimeMetrics(duration time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.metrics.MinResponseTime == 0 || duration < f.metrics.MinResponseTime {
		f.metrics.MinResponseTime = duration
	}
	if duration > f.metrics.MaxResponseTime {
		f.metrics.MaxResponseTime = duration
	}

	// Calculate running average
	totalRequests := f.metrics.RequestCount
	if totalRequests > 0 {
		f.metrics.AvgResponseTime = time.Duration(
			(int64(f.metrics.AvgResponseTime)*int64(totalRequests-1) + int64(duration)) / int64(totalRequests),
		)
	} else {
		f.metrics.AvgResponseTime = duration
	}

	f.metrics.LastRequestTime = time.Now()
}

// FTC API response types

// FTCCheckResponse represents a response from the FTC check API
type FTCCheckResponse struct {
	PhoneNumber      string `json:"phone_number"`
	IsListed         bool   `json:"is_listed"`
	RegistrationDate string `json:"registration_date,omitempty"`
	ExpirationDate   string `json:"expiration_date,omitempty"`
	Reason           string `json:"reason,omitempty"`
	ID               string `json:"id,omitempty"`
	ListType         string `json:"list_type,omitempty"`
}

// FTCBatchRequest represents a batch check request
type FTCBatchRequest struct {
	Numbers []string `json:"numbers"`
	Format  string   `json:"format"`
}

// FTCBatchResponse represents a batch check response
type FTCBatchResponse struct {
	Results []FTCCheckResponse `json:"results"`
}

// FTCSyncResponse represents a sync response from FTC
type FTCSyncResponse struct {
	Records        []map[string]interface{} `json:"records"`
	Added          int                      `json:"added"`
	Updated        int                      `json:"updated"`
	Deleted        int                      `json:"deleted"`
	Version        string                   `json:"version"`
	NextCheckpoint string                   `json:"next_checkpoint,omitempty"`
	NextSync       string                   `json:"next_sync,omitempty"`
}