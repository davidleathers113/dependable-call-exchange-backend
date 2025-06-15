package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/dnc"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/infrastructure/cache"
	"golang.org/x/time/rate"
)

// CTIAProvider implements ProviderClient for the CTIA Wireless DNC Registry
// Provides access to wireless-specific DNC data through CTIA's Short Code Registry
type CTIAProvider struct {
	config       CTIAConfig
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
	
	// CTIA-specific state
	sessionToken string
	tokenExpiry  time.Time
}

// CTIAConfig contains configuration for the CTIA provider
type CTIAConfig struct {
	BaseURL          string        `json:"base_url"`
	ClientID         string        `json:"client_id"`
	ClientSecret     string        `json:"client_secret"`
	Timeout          time.Duration `json:"timeout"`
	MaxRetries       int           `json:"max_retries"`
	RateLimitRPS     int           `json:"rate_limit_rps"`
	
	// Circuit breaker config
	CircuitConfig    CircuitConfig `json:"circuit_config"`
	
	// Cache settings
	CacheTTL         time.Duration `json:"cache_ttl"`
	CacheEnabled     bool          `json:"cache_enabled"`
	
	// CTIA-specific settings
	Version          string        `json:"version"`
	Environment      string        `json:"environment"` // sandbox, production
	IncludeCarrier   bool          `json:"include_carrier"`
	IncludeShortCode bool          `json:"include_short_code"`
	BatchSize        int           `json:"batch_size"`
	
	// OAuth settings
	TokenEndpoint    string        `json:"token_endpoint"`
	Scope            string        `json:"scope"`
}

// NewCTIAProvider creates a new CTIA provider instance
func NewCTIAProvider(config CTIAConfig, cacheInterface cache.Interface) *CTIAProvider {
	if config.BaseURL == "" {
		config.BaseURL = "https://api.ctia.org/wireless-dnc"
	}
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.RateLimitRPS == 0 {
		config.RateLimitRPS = 20 // CTIA typically allows higher rates
	}
	if config.BatchSize == 0 {
		config.BatchSize = 50 // Smaller batches for wireless
	}
	if config.Version == "" {
		config.Version = "v2"
	}
	if config.Environment == "" {
		config.Environment = "production"
	}
	if config.CacheTTL == 0 {
		config.CacheTTL = 2 * time.Hour // Wireless data changes more frequently
	}
	if config.TokenEndpoint == "" {
		config.TokenEndpoint = config.BaseURL + "/oauth/token"
	}
	if config.Scope == "" {
		config.Scope = "wireless-dnc:read"
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

	return &CTIAProvider{
		config:       config,
		client:       httpClient,
		rateLimiter:  rate.NewLimiter(rate.Limit(config.RateLimitRPS), config.RateLimitRPS*2),
		circuitState: CircuitClosed,
		cache:        cacheInterface,
		metrics: &ProviderMetrics{
			ProviderName: "CTIA",
		},
	}
}

// GetProviderType returns the provider type
func (c *CTIAProvider) GetProviderType() dnc.ProviderType {
	return dnc.ProviderTypeFederal // CTIA is federal-level wireless DNC
}

// GetProviderName returns the provider name
func (c *CTIAProvider) GetProviderName() string {
	return "CTIA"
}

// HealthCheck performs a health check against the CTIA API
func (c *CTIAProvider) HealthCheck(ctx context.Context) (*HealthCheckResult, error) {
	start := time.Now()
	result := &HealthCheckResult{
		LastUpdated: start,
		Metadata:    make(map[string]string),
	}

	// Check circuit breaker state
	if c.circuitState == CircuitOpen {
		result.IsHealthy = false
		result.Error = "Circuit breaker is open"
		result.Connectivity = false
		return result, nil
	}

	// Ensure we have a valid token
	if err := c.ensureValidToken(ctx); err != nil {
		result.IsHealthy = false
		result.Authentication = false
		result.Error = fmt.Sprintf("Token validation failed: %v", err)
		return result, err
	}

	// Test endpoint availability
	healthURL := fmt.Sprintf("%s/%s/health", c.config.BaseURL, c.config.Version)
	req, err := http.NewRequestWithContext(ctx, "GET", healthURL, nil)
	if err != nil {
		result.IsHealthy = false
		result.Error = fmt.Sprintf("Failed to create request: %v", err)
		return result, err
	}

	c.addAuthHeaders(req)
	
	resp, err := c.client.Do(req)
	if err != nil {
		result.IsHealthy = false
		result.Error = fmt.Sprintf("Request failed: %v", err)
		result.Connectivity = false
		c.RecordFailure(err)
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
		// Try to refresh token
		if err := c.refreshToken(ctx); err != nil {
			err := &ProviderError{
				Code:     ErrCodeAuthenticationFailed,
				Message:  "CTIA API authentication failed",
				Provider: "CTIA",
				Retry:    false,
			}
			c.RecordFailure(err)
			return result, err
		}
	}

	// Check for successful response
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		result.IsHealthy = true
		result.Authentication = true
		result.DataAvailable = true
		result.RateLimit = true
		result.Metadata["api_version"] = c.config.Version
		result.Metadata["environment"] = c.config.Environment
		result.Metadata["token_expires"] = c.tokenExpiry.Format(time.RFC3339)
		c.RecordSuccess()
	} else {
		result.IsHealthy = false
		result.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
		if resp.StatusCode == 429 {
			result.RateLimit = false
		}
		err := &ProviderError{
			Code:     ErrCodeProviderUnavailable,
			Message:  fmt.Sprintf("CTIA API returned HTTP %d", resp.StatusCode),
			Provider: "CTIA",
			Retry:    resp.StatusCode >= 500,
		}
		c.RecordFailure(err)
		return result, err
	}

	c.lastHealth = result
	return result, nil
}

// CheckNumber checks if a wireless number is on the CTIA DNC registry
func (c *CTIAProvider) CheckNumber(ctx context.Context, phoneNumber string) (*CheckResult, error) {
	// Check circuit breaker
	if c.circuitState == CircuitOpen {
		return nil, &ProviderError{
			Code:     ErrCodeProviderUnavailable,
			Message:  "Circuit breaker is open",
			Provider: "CTIA",
			Retry:    true,
		}
	}

	// Normalize phone number
	normalizedNumber := c.normalizePhoneNumber(phoneNumber)
	if normalizedNumber == "" {
		return nil, &ProviderError{
			Code:     ErrCodeInvalidRequest,
			Message:  "Invalid phone number format",
			Provider: "CTIA",
			Retry:    false,
		}
	}

	// Check cache first
	if c.config.CacheEnabled && c.cache != nil {
		cacheKey := fmt.Sprintf("ctia:check:%s", normalizedNumber)
		if cached, err := c.cache.Get(ctx, cacheKey); err == nil && cached != nil {
			var result CheckResult
			if err := json.Unmarshal(cached, &result); err == nil {
				c.metrics.RequestCount++
				return &result, nil
			}
		}
	}

	// Ensure valid token
	if err := c.ensureValidToken(ctx); err != nil {
		return nil, err
	}

	// Rate limiting
	if err := c.rateLimiter.Wait(ctx); err != nil {
		return nil, &ProviderError{
			Code:     ErrCodeRateLimitExceeded,
			Message:  "Rate limit exceeded",
			Provider: "CTIA",
			Retry:    true,
		}
	}

	start := time.Now()
	c.metrics.RequestCount++

	// Build request URL
	checkURL := fmt.Sprintf("%s/%s/wireless-check", c.config.BaseURL, c.config.Version)
	params := url.Values{}
	params.Add("msisdn", normalizedNumber)
	if c.config.IncludeCarrier {
		params.Add("include_carrier", "true")
	}
	if c.config.IncludeShortCode {
		params.Add("include_short_code", "true")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", checkURL+"?"+params.Encode(), nil)
	if err != nil {
		c.RecordFailure(err)
		return nil, &ProviderError{
			Code:     ErrCodeInvalidRequest,
			Message:  fmt.Sprintf("Failed to create request: %v", err),
			Provider: "CTIA",
			Retry:    false,
		}
	}

	c.addAuthHeaders(req)

	resp, err := c.client.Do(req)
	if err != nil {
		c.RecordFailure(err)
		return nil, &ProviderError{
			Code:     ErrCodeConnectionFailed,
			Message:  fmt.Sprintf("Request failed: %v", err),
			Provider: "CTIA",
			Retry:    true,
		}
	}
	defer resp.Body.Close()

	duration := time.Since(start)
	c.updateResponseTimeMetrics(duration)

	// Handle error responses
	if resp.StatusCode != http.StatusOK {
		err := c.handleHTTPError(resp)
		c.RecordFailure(err)
		return nil, err
	}

	// Parse response
	var apiResponse CTIACheckResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		c.RecordFailure(err)
		return nil, &ProviderError{
			Code:     ErrCodeInvalidResponse,
			Message:  fmt.Sprintf("Failed to parse response: %v", err),
			Provider: "CTIA",
			Retry:    false,
		}
	}

	// Convert to standard format
	result := &CheckResult{
		PhoneNumber: normalizedNumber,
		IsListed:    apiResponse.IsOptedOut,
		ListSource:  "CTIA",
		LastUpdated: time.Now(),
		Confidence:  0.95, // CTIA is authoritative for wireless but with some margin
		Metadata:    make(map[string]string),
	}

	if apiResponse.OptOutDate != "" {
		if optDate, err := time.Parse("2006-01-02T15:04:05Z", apiResponse.OptOutDate); err == nil {
			result.RegistrationDate = &optDate
		}
	}

	result.Reason = apiResponse.Reason
	result.Metadata["carrier"] = apiResponse.Carrier
	result.Metadata["short_code"] = apiResponse.ShortCode
	result.Metadata["wireless_type"] = apiResponse.WirelessType
	result.Metadata["opt_out_method"] = apiResponse.OptOutMethod

	// Cache the result
	if c.config.CacheEnabled && c.cache != nil {
		cacheKey := fmt.Sprintf("ctia:check:%s", normalizedNumber)
		if data, err := json.Marshal(result); err == nil {
			c.cache.Set(ctx, cacheKey, data, c.config.CacheTTL)
		}
	}

	c.RecordSuccess()
	c.metrics.SuccessCount++
	return result, nil
}

// BatchCheckNumbers checks multiple wireless numbers in a single request
func (c *CTIAProvider) BatchCheckNumbers(ctx context.Context, phoneNumbers []string) ([]*CheckResult, error) {
	if len(phoneNumbers) == 0 {
		return []*CheckResult{}, nil
	}

	// Check circuit breaker
	if c.circuitState == CircuitOpen {
		return nil, &ProviderError{
			Code:     ErrCodeProviderUnavailable,
			Message:  "Circuit breaker is open",
			Provider: "CTIA",
			Retry:    true,
		}
	}

	// Process in batches
	var allResults []*CheckResult
	for i := 0; i < len(phoneNumbers); i += c.config.BatchSize {
		end := i + c.config.BatchSize
		if end > len(phoneNumbers) {
			end = len(phoneNumbers)
		}

		batch := phoneNumbers[i:end]
		results, err := c.processBatch(ctx, batch)
		if err != nil {
			return allResults, err
		}

		allResults = append(allResults, results...)
	}

	return allResults, nil
}

// GetIncrementalUpdates retrieves incremental updates from the CTIA registry
func (c *CTIAProvider) GetIncrementalUpdates(ctx context.Context, since time.Time) (*SyncResult, error) {
	if c.circuitState == CircuitOpen {
		return nil, &ProviderError{
			Code:     ErrCodeProviderUnavailable,
			Message:  "Circuit breaker is open",
			Provider: "CTIA",
			Retry:    true,
		}
	}

	// Ensure valid token
	if err := c.ensureValidToken(ctx); err != nil {
		return nil, err
	}

	start := time.Now()
	result := &SyncResult{
		ProviderName: "CTIA",
		StartedAt:   start,
		Status:      "started",
	}

	// Build incremental sync request
	syncURL := fmt.Sprintf("%s/%s/wireless-updates", c.config.BaseURL, c.config.Version)
	params := url.Values{}
	params.Add("since", since.Format(time.RFC3339))
	params.Add("limit", fmt.Sprintf("%d", c.config.BatchSize))

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
		c.RecordFailure(err)
		return result, err
	}

	c.addAuthHeaders(req)

	resp, err := c.client.Do(req)
	if err != nil {
		result.Status = "failed"
		result.CompletedAt = time.Now()
		result.Duration = time.Since(start)
		result.Errors = append(result.Errors, SyncError{
			Code:      ErrCodeConnectionFailed,
			Message:   fmt.Sprintf("Request failed: %v", err),
			Timestamp: time.Now(),
		})
		c.RecordFailure(err)
		return result, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err := c.handleHTTPError(resp)
		result.Status = "failed"
		result.CompletedAt = time.Now()
		result.Duration = time.Since(start)
		result.Errors = append(result.Errors, SyncError{
			Code:      err.(*ProviderError).Code,
			Message:   err.Error(),
			Timestamp: time.Now(),
		})
		c.RecordFailure(err)
		return result, err
	}

	var syncResponse CTIASyncResponse
	if err := json.NewDecoder(resp.Body).Decode(&syncResponse); err != nil {
		result.Status = "failed"
		result.CompletedAt = time.Now()
		result.Duration = time.Since(start)
		result.Errors = append(result.Errors, SyncError{
			Code:      ErrCodeInvalidResponse,
			Message:   fmt.Sprintf("Failed to parse response: %v", err),
			Timestamp: time.Now(),
		})
		c.RecordFailure(err)
		return result, err
	}

	// Process the sync data
	result.RecordsProcessed = len(syncResponse.OptOuts)
	result.RecordsAdded = syncResponse.NewOptOuts
	result.RecordsUpdated = syncResponse.UpdatedOptOuts
	result.RecordsDeleted = syncResponse.RemovedOptOuts
	result.DataVersion = syncResponse.Version
	result.Checkpoint = syncResponse.NextCursor
	
	if syncResponse.NextUpdate != "" {
		if nextSync, err := time.Parse(time.RFC3339, syncResponse.NextUpdate); err == nil {
			result.NextSync = &nextSync
		}
	}

	result.Status = "success"
	result.CompletedAt = time.Now()
	result.Duration = time.Since(start)
	
	if result.Duration.Seconds() > 0 {
		result.ThroughputPerSecond = float64(result.RecordsProcessed) / result.Duration.Seconds()
	}

	c.RecordSuccess()
	return result, nil
}

// GetFullSnapshot retrieves a full snapshot of the CTIA registry
func (c *CTIAProvider) GetFullSnapshot(ctx context.Context) (*SyncResult, error) {
	// Full snapshots for wireless data are typically not available
	// Return incremental from beginning of time
	return c.GetIncrementalUpdates(ctx, time.Time{})
}

// ValidateConfig validates the CTIA provider configuration
func (c *CTIAProvider) ValidateConfig(config map[string]string) error {
	required := []string{"client_id", "client_secret", "base_url"}
	for _, key := range required {
		if _, exists := config[key]; !exists {
			return &ProviderError{
				Code:     ErrCodeConfigurationError,
				Message:  fmt.Sprintf("Missing required configuration: %s", key),
				Provider: "CTIA",
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
				Provider: "CTIA",
				Retry:    false,
			}
		}
	}

	return nil
}

// SetConfig updates the provider configuration
func (c *CTIAProvider) SetConfig(config map[string]string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.ValidateConfig(config); err != nil {
		return err
	}

	// Update configuration
	if clientID, exists := config["client_id"]; exists {
		c.config.ClientID = clientID
		// Invalidate existing token
		c.sessionToken = ""
		c.tokenExpiry = time.Time{}
	}
	if clientSecret, exists := config["client_secret"]; exists {
		c.config.ClientSecret = clientSecret
		// Invalidate existing token
		c.sessionToken = ""
		c.tokenExpiry = time.Time{}
	}
	if baseURL, exists := config["base_url"]; exists {
		c.config.BaseURL = baseURL
	}
	if timeout, exists := config["timeout"]; exists {
		if d, err := time.ParseDuration(timeout); err == nil {
			c.config.Timeout = d
			c.client.Timeout = d
		}
	}

	return nil
}

// GetRateLimit returns the current rate limit configuration
func (c *CTIAProvider) GetRateLimit() RateLimit {
	return RateLimit{
		RequestsPerSecond: c.config.RateLimitRPS,
		RequestsPerMinute: c.config.RateLimitRPS * 60,
		RequestsPerHour:   c.config.RateLimitRPS * 3600,
		RequestsPerDay:    c.config.RateLimitRPS * 86400,
		BurstSize:         c.config.RateLimitRPS * 2,
	}
}

// GetQuotaStatus returns the current quota status
func (c *CTIAProvider) GetQuotaStatus(ctx context.Context) (*QuotaStatus, error) {
	// CTIA typically has daily quotas
	return &QuotaStatus{
		Used:      c.metrics.RequestCount,
		Limit:     10000, // Typical daily limit
		Remaining: 10000 - c.metrics.RequestCount,
		ResetTime: time.Now().Add(24 * time.Hour),
		Period:    "day",
	}, nil
}

// Connect establishes connection to the CTIA provider
func (c *CTIAProvider) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Get authentication token
	if err := c.refreshToken(ctx); err != nil {
		return err
	}

	health, err := c.HealthCheck(ctx)
	if err != nil {
		return err
	}

	if !health.IsHealthy {
		return &ProviderError{
			Code:     ErrCodeConnectionFailed,
			Message:  "Health check failed",
			Provider: "CTIA",
			Retry:    true,
		}
	}

	c.connected = true
	return nil
}

// Disconnect disconnects from the CTIA provider
func (c *CTIAProvider) Disconnect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.connected = false
	c.sessionToken = ""
	c.tokenExpiry = time.Time{}
	return nil
}

// IsConnected returns the connection status
func (c *CTIAProvider) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected && c.sessionToken != "" && time.Now().Before(c.tokenExpiry)
}

// Circuit breaker methods (similar to FTC implementation)

func (c *CTIAProvider) GetCircuitState() CircuitState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.circuitState
}

func (c *CTIAProvider) ResetCircuit() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.circuitState = CircuitClosed
	c.failureCount = 0
	c.successCount = 0
	c.lastFailureTime = time.Time{}
	return nil
}

func (c *CTIAProvider) RecordSuccess() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.successCount++
	c.metrics.SuccessCount++

	if c.circuitState == CircuitHalfOpen && c.successCount >= c.config.CircuitConfig.SuccessThreshold {
		c.circuitState = CircuitClosed
		c.failureCount = 0
	}
}

func (c *CTIAProvider) RecordFailure(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.failureCount++
	c.lastFailureTime = time.Now()
	c.metrics.ErrorCount++

	if c.circuitState == CircuitClosed && c.failureCount >= c.config.CircuitConfig.FailureThreshold {
		c.circuitState = CircuitOpen
		c.metrics.CircuitOpenCount++
	}

	if c.circuitState == CircuitHalfOpen {
		c.circuitState = CircuitOpen
		c.metrics.CircuitOpenCount++
	}
}

func (c *CTIAProvider) SetCircuitConfig(config CircuitConfig) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.config.CircuitConfig = config
	return nil
}

// Helper methods

// normalizePhoneNumber normalizes a phone number for CTIA (MSISDN format)
func (c *CTIAProvider) normalizePhoneNumber(phoneNumber string) string {
	// Remove all non-digit characters
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, phoneNumber)

	// Handle US/Canada numbers for wireless
	if len(digits) == 10 {
		return "1" + digits // CTIA expects country code without +
	}
	if len(digits) == 11 && digits[0] == '1' {
		return digits
	}

	// Return as-is if it looks valid
	if len(digits) > 7 {
		return digits
	}

	return "" // Invalid
}

// ensureValidToken ensures we have a valid OAuth token
func (c *CTIAProvider) ensureValidToken(ctx context.Context) error {
	c.mu.RLock()
	hasValidToken := c.sessionToken != "" && time.Now().Before(c.tokenExpiry.Add(-5*time.Minute))
	c.mu.RUnlock()

	if !hasValidToken {
		return c.refreshToken(ctx)
	}
	return nil
}

// refreshToken obtains a new OAuth token
func (c *CTIAProvider) refreshToken(ctx context.Context) error {
	tokenURL := c.config.TokenEndpoint
	
	data := url.Values{}
	data.Set("grant_type", "client_credentials")
	data.Set("scope", c.config.Scope)

	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return &ProviderError{
			Code:     ErrCodeAuthenticationFailed,
			Message:  fmt.Sprintf("Failed to create token request: %v", err),
			Provider: "CTIA",
			Retry:    false,
		}
	}

	req.SetBasicAuth(c.config.ClientID, c.config.ClientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.client.Do(req)
	if err != nil {
		return &ProviderError{
			Code:     ErrCodeAuthenticationFailed,
			Message:  fmt.Sprintf("Token request failed: %v", err),
			Provider: "CTIA",
			Retry:    true,
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &ProviderError{
			Code:     ErrCodeAuthenticationFailed,
			Message:  fmt.Sprintf("Token request failed with HTTP %d", resp.StatusCode),
			Provider: "CTIA",
			Retry:    false,
		}
	}

	var tokenResponse CTIATokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResponse); err != nil {
		return &ProviderError{
			Code:     ErrCodeAuthenticationFailed,
			Message:  fmt.Sprintf("Failed to parse token response: %v", err),
			Provider: "CTIA",
			Retry:    false,
		}
	}

	c.mu.Lock()
	c.sessionToken = tokenResponse.AccessToken
	c.tokenExpiry = time.Now().Add(time.Duration(tokenResponse.ExpiresIn) * time.Second)
	c.mu.Unlock()

	return nil
}

// addAuthHeaders adds authentication headers to the request
func (c *CTIAProvider) addAuthHeaders(req *http.Request) {
	if c.sessionToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.sessionToken)
	}
	req.Header.Set("User-Agent", "DCE-DNC-Client/1.0")
	req.Header.Set("Accept", "application/json")
}

// handleHTTPError converts HTTP errors to provider errors
func (c *CTIAProvider) handleHTTPError(resp *http.Response) error {
	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return &ProviderError{
			Code:     ErrCodeAuthenticationFailed,
			Message:  "Authentication failed",
			Provider: "CTIA",
			Retry:    false,
		}
	case http.StatusTooManyRequests:
		return &ProviderError{
			Code:     ErrCodeRateLimitExceeded,
			Message:  "Rate limit exceeded",
			Provider: "CTIA",
			Retry:    true,
		}
	case http.StatusBadRequest:
		return &ProviderError{
			Code:     ErrCodeInvalidRequest,
			Message:  "Bad request",
			Provider: "CTIA",
			Retry:    false,
		}
	case http.StatusServiceUnavailable:
		return &ProviderError{
			Code:     ErrCodeProviderUnavailable,
			Message:  "Service unavailable",
			Provider: "CTIA",
			Retry:    true,
		}
	default:
		return &ProviderError{
			Code:     ErrCodeProviderUnavailable,
			Message:  fmt.Sprintf("HTTP %d", resp.StatusCode),
			Provider: "CTIA",
			Retry:    resp.StatusCode >= 500,
		}
	}
}

// processBatch processes a batch of phone numbers
func (c *CTIAProvider) processBatch(ctx context.Context, phoneNumbers []string) ([]*CheckResult, error) {
	start := time.Now()
	c.metrics.RequestCount++

	// Normalize numbers
	normalizedNumbers := make([]string, 0, len(phoneNumbers))
	for _, num := range phoneNumbers {
		if normalized := c.normalizePhoneNumber(num); normalized != "" {
			normalizedNumbers = append(normalizedNumbers, normalized)
		}
	}

	// Ensure valid token
	if err := c.ensureValidToken(ctx); err != nil {
		return nil, err
	}

	// Rate limiting
	if err := c.rateLimiter.Wait(ctx); err != nil {
		return nil, &ProviderError{
			Code:     ErrCodeRateLimitExceeded,
			Message:  "Rate limit exceeded",
			Provider: "CTIA",
			Retry:    true,
		}
	}

	// Build batch request
	batchURL := fmt.Sprintf("%s/%s/wireless-batch-check", c.config.BaseURL, c.config.Version)
	requestBody := CTIABatchRequest{
		MSISDNs: normalizedNumbers,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		c.RecordFailure(err)
		return nil, &ProviderError{
			Code:     ErrCodeInvalidRequest,
			Message:  fmt.Sprintf("Failed to marshal request: %v", err),
			Provider: "CTIA",
			Retry:    false,
		}
	}

	req, err := http.NewRequestWithContext(ctx, "POST", batchURL, strings.NewReader(string(jsonData)))
	if err != nil {
		c.RecordFailure(err)
		return nil, &ProviderError{
			Code:     ErrCodeInvalidRequest,
			Message:  fmt.Sprintf("Failed to create request: %v", err),
			Provider: "CTIA",
			Retry:    false,
		}
	}

	c.addAuthHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		c.RecordFailure(err)
		return nil, &ProviderError{
			Code:     ErrCodeConnectionFailed,
			Message:  fmt.Sprintf("Request failed: %v", err),
			Provider: "CTIA",
			Retry:    true,
		}
	}
	defer resp.Body.Close()

	duration := time.Since(start)
	c.updateResponseTimeMetrics(duration)

	if resp.StatusCode != http.StatusOK {
		err := c.handleHTTPError(resp)
		c.RecordFailure(err)
		return nil, err
	}

	var batchResponse CTIABatchResponse
	if err := json.NewDecoder(resp.Body).Decode(&batchResponse); err != nil {
		c.RecordFailure(err)
		return nil, &ProviderError{
			Code:     ErrCodeInvalidResponse,
			Message:  fmt.Sprintf("Failed to parse response: %v", err),
			Provider: "CTIA",
			Retry:    false,
		}
	}

	// Convert results
	results := make([]*CheckResult, len(batchResponse.Results))
	for i, result := range batchResponse.Results {
		checkResult := &CheckResult{
			PhoneNumber: result.MSISDN,
			IsListed:    result.IsOptedOut,
			ListSource:  "CTIA",
			LastUpdated: time.Now(),
			Confidence:  0.95,
			Metadata:    make(map[string]string),
		}

		if result.OptOutDate != "" {
			if optDate, err := time.Parse("2006-01-02T15:04:05Z", result.OptOutDate); err == nil {
				checkResult.RegistrationDate = &optDate
			}
		}

		checkResult.Reason = result.Reason
		checkResult.Metadata["carrier"] = result.Carrier
		checkResult.Metadata["short_code"] = result.ShortCode
		checkResult.Metadata["wireless_type"] = result.WirelessType

		results[i] = checkResult
	}

	c.RecordSuccess()
	c.metrics.SuccessCount++
	return results, nil
}

// updateResponseTimeMetrics updates response time metrics
func (c *CTIAProvider) updateResponseTimeMetrics(duration time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.metrics.MinResponseTime == 0 || duration < c.metrics.MinResponseTime {
		c.metrics.MinResponseTime = duration
	}
	if duration > c.metrics.MaxResponseTime {
		c.metrics.MaxResponseTime = duration
	}

	totalRequests := c.metrics.RequestCount
	if totalRequests > 0 {
		c.metrics.AvgResponseTime = time.Duration(
			(int64(c.metrics.AvgResponseTime)*int64(totalRequests-1) + int64(duration)) / int64(totalRequests),
		)
	} else {
		c.metrics.AvgResponseTime = duration
	}

	c.metrics.LastRequestTime = time.Now()
}

// CTIA API response types

// CTIATokenResponse represents an OAuth token response
type CTIATokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
	Scope       string `json:"scope"`
}

// CTIACheckResponse represents a response from the CTIA check API
type CTIACheckResponse struct {
	MSISDN        string `json:"msisdn"`
	IsOptedOut    bool   `json:"is_opted_out"`
	OptOutDate    string `json:"opt_out_date,omitempty"`
	Reason        string `json:"reason,omitempty"`
	Carrier       string `json:"carrier,omitempty"`
	ShortCode     string `json:"short_code,omitempty"`
	WirelessType  string `json:"wireless_type,omitempty"`
	OptOutMethod  string `json:"opt_out_method,omitempty"`
}

// CTIABatchRequest represents a batch check request
type CTIABatchRequest struct {
	MSISDNs []string `json:"msisdns"`
}

// CTIABatchResponse represents a batch check response
type CTIABatchResponse struct {
	Results []CTIACheckResponse `json:"results"`
}

// CTIASyncResponse represents a sync response from CTIA
type CTIASyncResponse struct {
	OptOuts        []map[string]interface{} `json:"opt_outs"`
	NewOptOuts     int                      `json:"new_opt_outs"`
	UpdatedOptOuts int                      `json:"updated_opt_outs"`
	RemovedOptOuts int                      `json:"removed_opt_outs"`
	Version        string                   `json:"version"`
	NextCursor     string                   `json:"next_cursor,omitempty"`
	NextUpdate     string                   `json:"next_update,omitempty"`
}