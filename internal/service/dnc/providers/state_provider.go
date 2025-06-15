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

// StateProvider implements ProviderClient for state-specific DNC registries
// Supports multiple state DNC systems through a unified interface
type StateProvider struct {
	config       StateConfig
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
	
	// State-specific configuration
	stateEndpoints map[string]StateEndpointConfig
	apiKeys        map[string]string
}

// StateConfig contains configuration for the state provider
type StateConfig struct {
	DefaultBaseURL   string                           `json:"default_base_url"`
	Timeout          time.Duration                    `json:"timeout"`
	MaxRetries       int                              `json:"max_retries"`
	RateLimitRPS     int                              `json:"rate_limit_rps"`
	
	// Circuit breaker config
	CircuitConfig    CircuitConfig                    `json:"circuit_config"`
	
	// Cache settings
	CacheTTL         time.Duration                    `json:"cache_ttl"`
	CacheEnabled     bool                             `json:"cache_enabled"`
	
	// State-specific settings
	SupportedStates  []string                         `json:"supported_states"`
	StateEndpoints   map[string]StateEndpointConfig   `json:"state_endpoints"`
	DefaultFormat    string                           `json:"default_format"`
	BatchSize        int                              `json:"batch_size"`
	
	// Fallback configuration
	UseFallback      bool                             `json:"use_fallback"`
	FallbackTimeout  time.Duration                    `json:"fallback_timeout"`
}

// StateEndpointConfig contains configuration for a specific state
type StateEndpointConfig struct {
	BaseURL      string            `json:"base_url"`
	APIKey       string            `json:"api_key"`
	AuthType     string            `json:"auth_type"` // bearer, basic, query_param
	Format       string            `json:"format"`    // json, xml, csv
	Version      string            `json:"version"`
	Endpoints    StateEndpoints    `json:"endpoints"`
	RateLimit    int               `json:"rate_limit_rps"`
	Timeout      time.Duration     `json:"timeout"`
	Metadata     map[string]string `json:"metadata"`
}

// StateEndpoints contains specific endpoint paths for a state
type StateEndpoints struct {
	Check       string `json:"check"`
	Batch       string `json:"batch"`
	Sync        string `json:"sync"`
	Health      string `json:"health"`
	Register    string `json:"register,omitempty"`
	Unregister  string `json:"unregister,omitempty"`
}

// NewStateProvider creates a new state provider instance
func NewStateProvider(config StateConfig, cacheInterface cache.Interface) *StateProvider {
	if config.DefaultBaseURL == "" {
		config.DefaultBaseURL = "https://api.state-dnc.gov"
	}
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.RateLimitRPS == 0 {
		config.RateLimitRPS = 5 // Conservative for state systems
	}
	if config.BatchSize == 0 {
		config.BatchSize = 25 // Smaller batches for state systems
	}
	if config.DefaultFormat == "" {
		config.DefaultFormat = "json"
	}
	if config.CacheTTL == 0 {
		config.CacheTTL = 6 * time.Hour // State data changes less frequently
	}
	if config.FallbackTimeout == 0 {
		config.FallbackTimeout = 10 * time.Second
	}

	// Set default circuit breaker config
	if config.CircuitConfig.FailureThreshold == 0 {
		config.CircuitConfig.FailureThreshold = 3 // More sensitive for state systems
	}
	if config.CircuitConfig.RecoveryTimeout == 0 {
		config.CircuitConfig.RecoveryTimeout = 60 * time.Second
	}
	if config.CircuitConfig.SuccessThreshold == 0 {
		config.CircuitConfig.SuccessThreshold = 2
	}
	if config.CircuitConfig.HalfOpenMaxCalls == 0 {
		config.CircuitConfig.HalfOpenMaxCalls = 3
	}

	httpClient := &http.Client{
		Timeout: config.Timeout,
		Transport: &http.Transport{
			MaxIdleConns:        5,
			MaxIdleConnsPerHost: 2,
			IdleConnTimeout:     60 * time.Second,
		},
	}

	return &StateProvider{
		config:         config,
		client:         httpClient,
		rateLimiter:    rate.NewLimiter(rate.Limit(config.RateLimitRPS), config.RateLimitRPS*2),
		circuitState:   CircuitClosed,
		cache:          cacheInterface,
		stateEndpoints: config.StateEndpoints,
		apiKeys:        make(map[string]string),
		metrics: &ProviderMetrics{
			ProviderName: "State",
		},
	}
}

// GetProviderType returns the provider type
func (s *StateProvider) GetProviderType() dnc.ProviderType {
	return dnc.ProviderTypeState
}

// GetProviderName returns the provider name
func (s *StateProvider) GetProviderName() string {
	return "State"
}

// HealthCheck performs a health check against available state APIs
func (s *StateProvider) HealthCheck(ctx context.Context) (*HealthCheckResult, error) {
	start := time.Now()
	result := &HealthCheckResult{
		LastUpdated: start,
		Metadata:    make(map[string]string),
	}

	// Check circuit breaker state
	if s.circuitState == CircuitOpen {
		result.IsHealthy = false
		result.Error = "Circuit breaker is open"
		result.Connectivity = false
		return result, nil
	}

	// Test a subset of state endpoints
	healthyStates := 0
	totalStates := 0
	errors := make([]string, 0)

	for state, endpointConfig := range s.stateEndpoints {
		totalStates++
		
		// Create timeout context for individual state check
		stateCtx, cancel := context.WithTimeout(ctx, s.config.FallbackTimeout)
		
		healthy, err := s.checkStateHealth(stateCtx, state, endpointConfig)
		cancel()
		
		if healthy {
			healthyStates++
		} else if err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", state, err))
		}
	}

	result.ResponseTime = time.Since(start)
	result.Connectivity = healthyStates > 0
	result.Metadata["healthy_states"] = fmt.Sprintf("%d", healthyStates)
	result.Metadata["total_states"] = fmt.Sprintf("%d", totalStates)
	result.Metadata["supported_states"] = strings.Join(s.config.SupportedStates, ",")

	// Consider healthy if at least 50% of states are responsive
	if totalStates > 0 {
		healthRatio := float64(healthyStates) / float64(totalStates)
		result.IsHealthy = healthRatio >= 0.5
		result.DataAvailable = healthyStates > 0
		result.RateLimit = true // Assume rate limiting is working
		
		if healthRatio >= 0.8 {
			result.Authentication = true
		} else if len(errors) > 0 {
			result.Error = fmt.Sprintf("Some states unavailable: %s", strings.Join(errors, "; "))
		}
		
		if result.IsHealthy {
			s.RecordSuccess()
		} else {
			err := &ProviderError{
				Code:     ErrCodeProviderUnavailable,
				Message:  fmt.Sprintf("Only %d/%d states available", healthyStates, totalStates),
				Provider: "State",
				Retry:    true,
			}
			s.RecordFailure(err)
		}
	} else {
		result.IsHealthy = false
		result.Error = "No state endpoints configured"
	}

	s.lastHealth = result
	return result, nil
}

// CheckNumber checks if a phone number is on any state DNC registry
func (s *StateProvider) CheckNumber(ctx context.Context, phoneNumber string) (*CheckResult, error) {
	// Check circuit breaker
	if s.circuitState == CircuitOpen {
		return nil, &ProviderError{
			Code:     ErrCodeProviderUnavailable,
			Message:  "Circuit breaker is open",
			Provider: "State",
			Retry:    true,
		}
	}

	// Normalize phone number and determine state
	normalizedNumber := s.normalizePhoneNumber(phoneNumber)
	if normalizedNumber == "" {
		return nil, &ProviderError{
			Code:     ErrCodeInvalidRequest,
			Message:  "Invalid phone number format",
			Provider: "State",
			Retry:    false,
		}
	}

	// Determine which state(s) to check based on area code
	statesToCheck := s.determineStatesForNumber(normalizedNumber)
	if len(statesToCheck) == 0 {
		// No specific state mapping, try primary states
		statesToCheck = s.getPrimaryStates()
	}

	// Check cache first
	if s.config.CacheEnabled && s.cache != nil {
		cacheKey := fmt.Sprintf("state:check:%s", normalizedNumber)
		if cached, err := s.cache.Get(ctx, cacheKey); err == nil && cached != nil {
			var result CheckResult
			if err := json.Unmarshal(cached, &result); err == nil {
				s.metrics.RequestCount++
				return &result, nil
			}
		}
	}

	// Rate limiting
	if err := s.rateLimiter.Wait(ctx); err != nil {
		return nil, &ProviderError{
			Code:     ErrCodeRateLimitExceeded,
			Message:  "Rate limit exceeded",
			Provider: "State",
			Retry:    true,
		}
	}

	start := time.Now()
	s.metrics.RequestCount++

	// Check each relevant state
	var bestResult *CheckResult
	var errors []string

	for _, state := range statesToCheck {
		result, err := s.checkNumberInState(ctx, normalizedNumber, state)
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", state, err))
			continue
		}

		if result.IsListed {
			// Found a positive match, return immediately
			bestResult = result
			bestResult.Metadata["checked_states"] = strings.Join(statesToCheck, ",")
			break
		}

		// Keep the first negative result as fallback
		if bestResult == nil {
			bestResult = result
		}
	}

	duration := time.Since(start)
	s.updateResponseTimeMetrics(duration)

	if bestResult == nil {
		// All states failed
		s.RecordFailure(fmt.Errorf("all states failed: %s", strings.Join(errors, "; ")))
		return nil, &ProviderError{
			Code:     ErrCodeProviderUnavailable,
			Message:  fmt.Sprintf("All state checks failed: %s", strings.Join(errors, "; ")),
			Provider: "State",
			Retry:    true,
		}
	}

	// Set metadata about which states were checked
	if bestResult.Metadata == nil {
		bestResult.Metadata = make(map[string]string)
	}
	bestResult.Metadata["checked_states"] = strings.Join(statesToCheck, ",")
	bestResult.Metadata["total_states_checked"] = fmt.Sprintf("%d", len(statesToCheck))

	// Cache the result
	if s.config.CacheEnabled && s.cache != nil {
		cacheKey := fmt.Sprintf("state:check:%s", normalizedNumber)
		if data, err := json.Marshal(bestResult); err == nil {
			s.cache.Set(ctx, cacheKey, data, s.config.CacheTTL)
		}
	}

	s.RecordSuccess()
	s.metrics.SuccessCount++
	return bestResult, nil
}

// BatchCheckNumbers checks multiple phone numbers across state registries
func (s *StateProvider) BatchCheckNumbers(ctx context.Context, phoneNumbers []string) ([]*CheckResult, error) {
	if len(phoneNumbers) == 0 {
		return []*CheckResult{}, nil
	}

	// Check circuit breaker
	if s.circuitState == CircuitOpen {
		return nil, &ProviderError{
			Code:     ErrCodeProviderUnavailable,
			Message:  "Circuit breaker is open",
			Provider: "State",
			Retry:    true,
		}
	}

	// Group numbers by state for efficient batch processing
	numbersByState := s.groupNumbersByState(phoneNumbers)

	var allResults []*CheckResult
	var allErrors []string

	// Process each state's numbers in batches
	for state, numbers := range numbersByState {
		if len(numbers) == 0 {
			continue
		}

		// Process in smaller batches
		for i := 0; i < len(numbers); i += s.config.BatchSize {
			end := i + s.config.BatchSize
			if end > len(numbers) {
				end = len(numbers)
			}

			batch := numbers[i:end]
			results, err := s.processBatchForState(ctx, batch, state)
			if err != nil {
				allErrors = append(allErrors, fmt.Sprintf("%s: %v", state, err))
				continue
			}

			allResults = append(allResults, results...)
		}
	}

	if len(allResults) == 0 && len(allErrors) > 0 {
		return nil, &ProviderError{
			Code:     ErrCodeProviderUnavailable,
			Message:  fmt.Sprintf("All batch checks failed: %s", strings.Join(allErrors, "; ")),
			Provider: "State",
			Retry:    true,
		}
	}

	return allResults, nil
}

// GetIncrementalUpdates retrieves incremental updates from state registries
func (s *StateProvider) GetIncrementalUpdates(ctx context.Context, since time.Time) (*SyncResult, error) {
	if s.circuitState == CircuitOpen {
		return nil, &ProviderError{
			Code:     ErrCodeProviderUnavailable,
			Message:  "Circuit breaker is open",
			Provider: "State",
			Retry:    true,
		}
	}

	start := time.Now()
	result := &SyncResult{
		ProviderName: "State",
		StartedAt:   start,
		Status:      "started",
	}

	// Sync from all available state endpoints
	var totalProcessed, totalAdded, totalUpdated, totalDeleted int
	var allErrors []SyncError

	for state, endpointConfig := range s.stateEndpoints {
		stateResult, err := s.syncFromState(ctx, state, endpointConfig, since)
		if err != nil {
			allErrors = append(allErrors, SyncError{
				Code:      ErrCodeProviderUnavailable,
				Message:   fmt.Sprintf("State %s sync failed: %v", state, err),
				Timestamp: time.Now(),
			})
			continue
		}

		totalProcessed += stateResult.RecordsProcessed
		totalAdded += stateResult.RecordsAdded
		totalUpdated += stateResult.RecordsUpdated
		totalDeleted += stateResult.RecordsDeleted
	}

	result.RecordsProcessed = totalProcessed
	result.RecordsAdded = totalAdded
	result.RecordsUpdated = totalUpdated
	result.RecordsDeleted = totalDeleted
	result.Errors = allErrors

	if totalProcessed > 0 {
		result.Status = "success"
		if len(allErrors) > 0 {
			result.Status = "partial"
		}
	} else {
		result.Status = "failed"
	}

	result.CompletedAt = time.Now()
	result.Duration = time.Since(start)
	
	if result.Duration.Seconds() > 0 {
		result.ThroughputPerSecond = float64(result.RecordsProcessed) / result.Duration.Seconds()
	}

	if result.Status == "success" || result.Status == "partial" {
		s.RecordSuccess()
	} else {
		err := fmt.Errorf("sync failed for all states")
		s.RecordFailure(err)
		return result, &ProviderError{
			Code:     ErrCodeProviderUnavailable,
			Message:  "Sync failed for all states",
			Provider: "State",
			Retry:    true,
		}
	}

	return result, nil
}

// GetFullSnapshot retrieves a full snapshot from state registries
func (s *StateProvider) GetFullSnapshot(ctx context.Context) (*SyncResult, error) {
	// Full snapshots from states are typically not available
	// Return incremental from beginning of time
	return s.GetIncrementalUpdates(ctx, time.Time{})
}

// ValidateConfig validates the state provider configuration
func (s *StateProvider) ValidateConfig(config map[string]string) error {
	required := []string{"supported_states"}
	for _, key := range required {
		if _, exists := config[key]; !exists {
			return &ProviderError{
				Code:     ErrCodeConfigurationError,
				Message:  fmt.Sprintf("Missing required configuration: %s", key),
				Provider: "State",
				Retry:    false,
			}
		}
	}

	// Validate state list
	if states, exists := config["supported_states"]; exists {
		stateList := strings.Split(states, ",")
		for _, state := range stateList {
			state = strings.TrimSpace(state)
			if len(state) != 2 {
				return &ProviderError{
					Code:     ErrCodeConfigurationError,
					Message:  fmt.Sprintf("Invalid state code: %s", state),
					Provider: "State",
					Retry:    false,
				}
			}
		}
	}

	return nil
}

// SetConfig updates the provider configuration
func (s *StateProvider) SetConfig(config map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ValidateConfig(config); err != nil {
		return err
	}

	// Update configuration
	if states, exists := config["supported_states"]; exists {
		s.config.SupportedStates = strings.Split(states, ",")
		for i, state := range s.config.SupportedStates {
			s.config.SupportedStates[i] = strings.TrimSpace(state)
		}
	}

	if baseURL, exists := config["default_base_url"]; exists {
		s.config.DefaultBaseURL = baseURL
	}

	if timeout, exists := config["timeout"]; exists {
		if d, err := time.ParseDuration(timeout); err == nil {
			s.config.Timeout = d
			s.client.Timeout = d
		}
	}

	return nil
}

// GetRateLimit returns the current rate limit configuration
func (s *StateProvider) GetRateLimit() RateLimit {
	return RateLimit{
		RequestsPerSecond: s.config.RateLimitRPS,
		RequestsPerMinute: s.config.RateLimitRPS * 60,
		RequestsPerHour:   s.config.RateLimitRPS * 3600,
		RequestsPerDay:    s.config.RateLimitRPS * 86400,
		BurstSize:         s.config.RateLimitRPS * 2,
	}
}

// GetQuotaStatus returns the current quota status
func (s *StateProvider) GetQuotaStatus(ctx context.Context) (*QuotaStatus, error) {
	// State systems typically have daily quotas
	return &QuotaStatus{
		Used:      s.metrics.RequestCount,
		Limit:     5000, // Typical daily limit for state systems
		Remaining: 5000 - s.metrics.RequestCount,
		ResetTime: time.Now().Add(24 * time.Hour),
		Period:    "day",
	}, nil
}

// Connect establishes connection to the state provider
func (s *StateProvider) Connect(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	health, err := s.HealthCheck(ctx)
	if err != nil {
		return err
	}

	if !health.IsHealthy {
		return &ProviderError{
			Code:     ErrCodeConnectionFailed,
			Message:  "Health check failed",
			Provider: "State",
			Retry:    true,
		}
	}

	s.connected = true
	return nil
}

// Disconnect disconnects from the state provider
func (s *StateProvider) Disconnect(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.connected = false
	return nil
}

// IsConnected returns the connection status
func (s *StateProvider) IsConnected() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.connected
}

// Circuit breaker methods (similar to previous implementations)

func (s *StateProvider) GetCircuitState() CircuitState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.circuitState
}

func (s *StateProvider) ResetCircuit() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.circuitState = CircuitClosed
	s.failureCount = 0
	s.successCount = 0
	s.lastFailureTime = time.Time{}
	return nil
}

func (s *StateProvider) RecordSuccess() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.successCount++
	s.metrics.SuccessCount++

	if s.circuitState == CircuitHalfOpen && s.successCount >= s.config.CircuitConfig.SuccessThreshold {
		s.circuitState = CircuitClosed
		s.failureCount = 0
	}
}

func (s *StateProvider) RecordFailure(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.failureCount++
	s.lastFailureTime = time.Now()
	s.metrics.ErrorCount++

	if s.circuitState == CircuitClosed && s.failureCount >= s.config.CircuitConfig.FailureThreshold {
		s.circuitState = CircuitOpen
		s.metrics.CircuitOpenCount++
	}

	if s.circuitState == CircuitHalfOpen {
		s.circuitState = CircuitOpen
		s.metrics.CircuitOpenCount++
	}
}

func (s *StateProvider) SetCircuitConfig(config CircuitConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.config.CircuitConfig = config
	return nil
}

// Helper methods

// normalizePhoneNumber normalizes a phone number to E.164 format
func (s *StateProvider) normalizePhoneNumber(phoneNumber string) string {
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

// determineStatesForNumber determines which states to check based on area code
func (s *StateProvider) determineStatesForNumber(phoneNumber string) []string {
	// Extract area code
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, phoneNumber)

	if len(digits) < 10 {
		return []string{}
	}

	var areaCode string
	if len(digits) == 10 {
		areaCode = digits[:3]
	} else if len(digits) == 11 && digits[0] == '1' {
		areaCode = digits[1:4]
	} else {
		return []string{}
	}

	// Map area codes to states (simplified mapping)
	areaCodeMap := map[string][]string{
		"201": {"NJ"}, "202": {"DC"}, "203": {"CT"}, "205": {"AL"},
		"206": {"WA"}, "207": {"ME"}, "208": {"ID"}, "209": {"CA"},
		"210": {"TX"}, "212": {"NY"}, "213": {"CA"}, "214": {"TX"},
		"215": {"PA"}, "216": {"OH"}, "217": {"IL"}, "218": {"MN"},
		"219": {"IN"}, "224": {"IL"}, "225": {"LA"}, "228": {"MS"},
		"229": {"GA"}, "231": {"MI"}, "234": {"OH"}, "239": {"FL"},
		"240": {"MD"}, "248": {"MI"}, "251": {"AL"}, "252": {"NC"},
		"253": {"WA"}, "254": {"TX"}, "256": {"AL"}, "260": {"IN"},
		"262": {"WI"}, "267": {"PA"}, "269": {"MI"}, "270": {"KY"},
		"276": {"VA"}, "281": {"TX"}, "301": {"MD"}, "302": {"DE"},
		"303": {"CO"}, "304": {"WV"}, "305": {"FL"}, "307": {"WY"},
		"308": {"NE"}, "309": {"IL"}, "310": {"CA"}, "312": {"IL"},
		"313": {"MI"}, "314": {"MO"}, "315": {"NY"}, "316": {"KS"},
		"317": {"IN"}, "318": {"LA"}, "319": {"IA"}, "320": {"MN"},
		"321": {"FL"}, "323": {"CA"}, "325": {"TX"}, "330": {"OH"},
		"331": {"IL"}, "334": {"AL"}, "336": {"NC"}, "337": {"LA"},
		"339": {"MA"}, "340": {"VI"}, "347": {"NY"}, "351": {"MA"},
		"352": {"FL"}, "360": {"WA"}, "361": {"TX"}, "386": {"FL"},
		"401": {"RI"}, "402": {"NE"}, "404": {"GA"}, "405": {"OK"},
		"406": {"MT"}, "407": {"FL"}, "408": {"CA"}, "409": {"TX"},
		"410": {"MD"}, "412": {"PA"}, "413": {"MA"}, "414": {"WI"},
		"415": {"CA"}, "417": {"MO"}, "419": {"OH"}, "423": {"TN"},
		"424": {"CA"}, "425": {"WA"}, "430": {"TX"}, "432": {"TX"},
		"434": {"VA"}, "435": {"UT"}, "440": {"OH"}, "443": {"MD"},
		"458": {"OR"}, "469": {"TX"}, "470": {"GA"}, "475": {"CT"},
		"478": {"GA"}, "479": {"AR"}, "480": {"AZ"}, "484": {"PA"},
		"501": {"AR"}, "502": {"KY"}, "503": {"OR"}, "504": {"LA"},
		"505": {"NM"}, "507": {"MN"}, "508": {"MA"}, "509": {"WA"},
		"510": {"CA"}, "512": {"TX"}, "513": {"OH"}, "515": {"IA"},
		"516": {"NY"}, "517": {"MI"}, "518": {"NY"}, "520": {"AZ"},
		"530": {"CA"}, "540": {"VA"}, "541": {"OR"}, "551": {"NJ"},
		"559": {"CA"}, "561": {"FL"}, "562": {"CA"}, "563": {"IA"},
		"567": {"OH"}, "570": {"PA"}, "571": {"VA"}, "573": {"MO"},
		"574": {"IN"}, "575": {"NM"}, "580": {"OK"}, "585": {"NY"},
		"586": {"MI"}, "601": {"MS"}, "602": {"AZ"}, "603": {"NH"},
		"605": {"SD"}, "606": {"KY"}, "607": {"NY"}, "608": {"WI"},
		"609": {"NJ"}, "610": {"PA"}, "612": {"MN"}, "614": {"OH"},
		"615": {"TN"}, "616": {"MI"}, "617": {"MA"}, "618": {"IL"},
		"619": {"CA"}, "620": {"KS"}, "623": {"AZ"}, "626": {"CA"},
		"630": {"IL"}, "631": {"NY"}, "636": {"MO"}, "641": {"IA"},
		"646": {"NY"}, "650": {"CA"}, "651": {"MN"}, "660": {"MO"},
		"661": {"CA"}, "662": {"MS"}, "667": {"MD"}, "678": {"GA"},
		"682": {"TX"}, "701": {"ND"}, "702": {"NV"}, "703": {"VA"},
		"704": {"NC"}, "706": {"GA"}, "707": {"CA"}, "708": {"IL"},
		"712": {"IA"}, "713": {"TX"}, "714": {"CA"}, "715": {"WI"},
		"716": {"NY"}, "717": {"PA"}, "718": {"NY"}, "719": {"CO"},
		"720": {"CO"}, "724": {"PA"}, "727": {"FL"}, "731": {"TN"},
		"732": {"NJ"}, "734": {"MI"}, "737": {"TX"}, "740": {"OH"},
		"747": {"CA"}, "754": {"FL"}, "757": {"VA"}, "760": {"CA"},
		"763": {"MN"}, "765": {"IN"}, "770": {"GA"}, "772": {"FL"},
		"773": {"IL"}, "774": {"MA"}, "775": {"NV"}, "781": {"MA"},
		"785": {"KS"}, "786": {"FL"}, "801": {"UT"}, "802": {"VT"},
		"803": {"SC"}, "804": {"VA"}, "805": {"CA"}, "806": {"TX"},
		"808": {"HI"}, "810": {"MI"}, "812": {"IN"}, "813": {"FL"},
		"814": {"PA"}, "815": {"IL"}, "816": {"MO"}, "817": {"TX"},
		"818": {"CA"}, "828": {"NC"}, "830": {"TX"}, "831": {"CA"},
		"832": {"TX"}, "843": {"SC"}, "845": {"NY"}, "847": {"IL"},
		"848": {"NJ"}, "850": {"FL"}, "856": {"NJ"}, "857": {"MA"},
		"858": {"CA"}, "859": {"KY"}, "860": {"CT"}, "862": {"NJ"},
		"863": {"FL"}, "864": {"SC"}, "865": {"TN"}, "870": {"AR"},
		"872": {"IL"}, "878": {"PA"}, "901": {"TN"}, "903": {"TX"},
		"904": {"FL"}, "906": {"MI"}, "907": {"AK"}, "908": {"NJ"},
		"909": {"CA"}, "910": {"NC"}, "912": {"GA"}, "913": {"KS"},
		"914": {"NY"}, "915": {"TX"}, "916": {"CA"}, "917": {"NY"},
		"918": {"OK"}, "919": {"NC"}, "920": {"WI"}, "925": {"CA"},
		"928": {"AZ"}, "929": {"NY"}, "931": {"TN"}, "936": {"TX"},
		"937": {"OH"}, "940": {"TX"}, "941": {"FL"}, "947": {"MI"},
		"949": {"CA"}, "951": {"CA"}, "952": {"MN"}, "954": {"FL"},
		"956": {"TX"}, "970": {"CO"}, "971": {"OR"}, "972": {"TX"},
		"973": {"NJ"}, "978": {"MA"}, "979": {"TX"}, "980": {"NC"},
		"984": {"NC"}, "985": {"LA"}, "989": {"MI"},
	}

	if states, exists := areaCodeMap[areaCode]; exists {
		// Filter by supported states
		var result []string
		for _, state := range states {
			for _, supported := range s.config.SupportedStates {
				if state == supported {
					result = append(result, state)
					break
				}
			}
		}
		return result
	}

	return []string{}
}

// getPrimaryStates returns the primary states to check when no specific mapping is found
func (s *StateProvider) getPrimaryStates() []string {
	// Return first few supported states as fallback
	if len(s.config.SupportedStates) > 3 {
		return s.config.SupportedStates[:3]
	}
	return s.config.SupportedStates
}

// checkStateHealth checks the health of a specific state endpoint
func (s *StateProvider) checkStateHealth(ctx context.Context, state string, config StateEndpointConfig) (bool, error) {
	if config.Endpoints.Health == "" {
		return true, nil // Assume healthy if no health endpoint
	}

	healthURL := fmt.Sprintf("%s%s", config.BaseURL, config.Endpoints.Health)
	req, err := http.NewRequestWithContext(ctx, "GET", healthURL, nil)
	if err != nil {
		return false, err
	}

	s.addAuthHeaders(req, config)

	resp, err := s.client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	return resp.StatusCode >= 200 && resp.StatusCode < 300, nil
}

// checkNumberInState checks a number against a specific state registry
func (s *StateProvider) checkNumberInState(ctx context.Context, phoneNumber, state string) (*CheckResult, error) {
	config, exists := s.stateEndpoints[state]
	if !exists {
		return nil, &ProviderError{
			Code:     ErrCodeConfigurationError,
			Message:  fmt.Sprintf("No configuration for state: %s", state),
			Provider: "State",
			Retry:    false,
		}
	}

	checkURL := fmt.Sprintf("%s%s", config.BaseURL, config.Endpoints.Check)
	params := url.Values{}
	params.Add("phone", phoneNumber)
	params.Add("format", config.Format)

	req, err := http.NewRequestWithContext(ctx, "GET", checkURL+"?"+params.Encode(), nil)
	if err != nil {
		return nil, &ProviderError{
			Code:     ErrCodeInvalidRequest,
			Message:  fmt.Sprintf("Failed to create request for %s: %v", state, err),
			Provider: "State",
			Retry:    false,
		}
	}

	s.addAuthHeaders(req, config)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, &ProviderError{
			Code:     ErrCodeConnectionFailed,
			Message:  fmt.Sprintf("Request to %s failed: %v", state, err),
			Provider: "State",
			Retry:    true,
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, s.handleHTTPError(resp, state)
	}

	var apiResponse StateCheckResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResponse); err != nil {
		return nil, &ProviderError{
			Code:     ErrCodeInvalidResponse,
			Message:  fmt.Sprintf("Failed to parse response from %s: %v", state, err),
			Provider: "State",
			Retry:    false,
		}
	}

	// Convert to standard format
	result := &CheckResult{
		PhoneNumber: phoneNumber,
		IsListed:    apiResponse.IsListed,
		ListSource:  fmt.Sprintf("State-%s", state),
		LastUpdated: time.Now(),
		Confidence:  0.9, // State registries are generally reliable
		Metadata:    make(map[string]string),
	}

	if apiResponse.RegistrationDate != "" {
		if regDate, err := time.Parse("2006-01-02", apiResponse.RegistrationDate); err == nil {
			result.RegistrationDate = &regDate
		}
	}

	result.Reason = apiResponse.Reason
	result.Metadata["state"] = state
	result.Metadata["registry"] = apiResponse.Registry

	return result, nil
}

// groupNumbersByState groups phone numbers by their likely state
func (s *StateProvider) groupNumbersByState(phoneNumbers []string) map[string][]string {
	result := make(map[string][]string)

	for _, number := range phoneNumbers {
		normalized := s.normalizePhoneNumber(number)
		if normalized == "" {
			continue
		}

		states := s.determineStatesForNumber(normalized)
		if len(states) == 0 {
			// Use primary states as fallback
			states = s.getPrimaryStates()
		}

		// Add to first applicable state (could be enhanced for multi-state numbers)
		if len(states) > 0 {
			state := states[0]
			result[state] = append(result[state], normalized)
		}
	}

	return result
}

// processBatchForState processes a batch of numbers for a specific state
func (s *StateProvider) processBatchForState(ctx context.Context, phoneNumbers []string, state string) ([]*CheckResult, error) {
	config, exists := s.stateEndpoints[state]
	if !exists {
		return nil, &ProviderError{
			Code:     ErrCodeConfigurationError,
			Message:  fmt.Sprintf("No configuration for state: %s", state),
			Provider: "State",
			Retry:    false,
		}
	}

	// Some states don't support batch operations, fall back to individual checks
	if config.Endpoints.Batch == "" {
		var results []*CheckResult
		for _, number := range phoneNumbers {
			result, err := s.checkNumberInState(ctx, number, state)
			if err != nil {
				return results, err
			}
			results = append(results, result)
		}
		return results, nil
	}

	start := time.Now()
	s.metrics.RequestCount++

	// Build batch request
	batchURL := fmt.Sprintf("%s%s", config.BaseURL, config.Endpoints.Batch)
	requestBody := StateBatchRequest{
		Numbers: phoneNumbers,
		Format:  config.Format,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		s.RecordFailure(err)
		return nil, &ProviderError{
			Code:     ErrCodeInvalidRequest,
			Message:  fmt.Sprintf("Failed to marshal request for %s: %v", state, err),
			Provider: "State",
			Retry:    false,
		}
	}

	req, err := http.NewRequestWithContext(ctx, "POST", batchURL, strings.NewReader(string(jsonData)))
	if err != nil {
		s.RecordFailure(err)
		return nil, &ProviderError{
			Code:     ErrCodeInvalidRequest,
			Message:  fmt.Sprintf("Failed to create request for %s: %v", state, err),
			Provider: "State",
			Retry:    false,
		}
	}

	s.addAuthHeaders(req, config)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		s.RecordFailure(err)
		return nil, &ProviderError{
			Code:     ErrCodeConnectionFailed,
			Message:  fmt.Sprintf("Request to %s failed: %v", state, err),
			Provider: "State",
			Retry:    true,
		}
	}
	defer resp.Body.Close()

	duration := time.Since(start)
	s.updateResponseTimeMetrics(duration)

	if resp.StatusCode != http.StatusOK {
		err := s.handleHTTPError(resp, state)
		s.RecordFailure(err)
		return nil, err
	}

	var batchResponse StateBatchResponse
	if err := json.NewDecoder(resp.Body).Decode(&batchResponse); err != nil {
		s.RecordFailure(err)
		return nil, &ProviderError{
			Code:     ErrCodeInvalidResponse,
			Message:  fmt.Sprintf("Failed to parse response from %s: %v", state, err),
			Provider: "State",
			Retry:    false,
		}
	}

	// Convert results
	results := make([]*CheckResult, len(batchResponse.Results))
	for i, result := range batchResponse.Results {
		checkResult := &CheckResult{
			PhoneNumber: result.PhoneNumber,
			IsListed:    result.IsListed,
			ListSource:  fmt.Sprintf("State-%s", state),
			LastUpdated: time.Now(),
			Confidence:  0.9,
			Metadata:    make(map[string]string),
		}

		if result.RegistrationDate != "" {
			if regDate, err := time.Parse("2006-01-02", result.RegistrationDate); err == nil {
				checkResult.RegistrationDate = &regDate
			}
		}

		checkResult.Reason = result.Reason
		checkResult.Metadata["state"] = state
		checkResult.Metadata["registry"] = result.Registry

		results[i] = checkResult
	}

	s.RecordSuccess()
	s.metrics.SuccessCount++
	return results, nil
}

// syncFromState syncs data from a specific state registry
func (s *StateProvider) syncFromState(ctx context.Context, state string, config StateEndpointConfig, since time.Time) (*SyncResult, error) {
	start := time.Now()
	result := &SyncResult{
		ProviderName: fmt.Sprintf("State-%s", state),
		StartedAt:   start,
		Status:      "started",
	}

	if config.Endpoints.Sync == "" {
		result.Status = "skipped"
		result.CompletedAt = time.Now()
		result.Duration = time.Since(start)
		return result, nil
	}

	syncURL := fmt.Sprintf("%s%s", config.BaseURL, config.Endpoints.Sync)
	params := url.Values{}
	params.Add("since", since.Format(time.RFC3339))
	params.Add("format", config.Format)

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
		return result, err
	}

	s.addAuthHeaders(req, config)

	resp, err := s.client.Do(req)
	if err != nil {
		result.Status = "failed"
		result.CompletedAt = time.Now()
		result.Duration = time.Since(start)
		result.Errors = append(result.Errors, SyncError{
			Code:      ErrCodeConnectionFailed,
			Message:   fmt.Sprintf("Request failed: %v", err),
			Timestamp: time.Now(),
		})
		return result, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		result.Status = "failed"
		result.CompletedAt = time.Now()
		result.Duration = time.Since(start)
		result.Errors = append(result.Errors, SyncError{
			Code:      ErrCodeProviderUnavailable,
			Message:   fmt.Sprintf("HTTP %d from %s", resp.StatusCode, state),
			Timestamp: time.Now(),
		})
		return result, fmt.Errorf("HTTP %d from %s", resp.StatusCode, state)
	}

	var syncResponse StateSyncResponse
	if err := json.NewDecoder(resp.Body).Decode(&syncResponse); err != nil {
		result.Status = "failed"
		result.CompletedAt = time.Now()
		result.Duration = time.Since(start)
		result.Errors = append(result.Errors, SyncError{
			Code:      ErrCodeInvalidResponse,
			Message:   fmt.Sprintf("Failed to parse response: %v", err),
			Timestamp: time.Now(),
		})
		return result, err
	}

	// Process the sync data
	result.RecordsProcessed = len(syncResponse.Records)
	result.RecordsAdded = syncResponse.Added
	result.RecordsUpdated = syncResponse.Updated
	result.RecordsDeleted = syncResponse.Deleted
	result.DataVersion = syncResponse.Version

	result.Status = "success"
	result.CompletedAt = time.Now()
	result.Duration = time.Since(start)
	
	if result.Duration.Seconds() > 0 {
		result.ThroughputPerSecond = float64(result.RecordsProcessed) / result.Duration.Seconds()
	}

	return result, nil
}

// addAuthHeaders adds authentication headers based on state configuration
func (s *StateProvider) addAuthHeaders(req *http.Request, config StateEndpointConfig) {
	switch config.AuthType {
	case "bearer":
		if config.APIKey != "" {
			req.Header.Set("Authorization", "Bearer "+config.APIKey)
		}
	case "basic":
		if config.APIKey != "" {
			req.SetBasicAuth("api", config.APIKey)
		}
	case "query_param":
		if config.APIKey != "" {
			q := req.URL.Query()
			q.Add("api_key", config.APIKey)
			req.URL.RawQuery = q.Encode()
		}
	}
	
	req.Header.Set("User-Agent", "DCE-DNC-Client/1.0")
	req.Header.Set("Accept", "application/json")
}

// handleHTTPError converts HTTP errors to provider errors for a specific state
func (s *StateProvider) handleHTTPError(resp *http.Response, state string) error {
	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return &ProviderError{
			Code:     ErrCodeAuthenticationFailed,
			Message:  fmt.Sprintf("Authentication failed for %s", state),
			Provider: "State",
			Retry:    false,
		}
	case http.StatusTooManyRequests:
		return &ProviderError{
			Code:     ErrCodeRateLimitExceeded,
			Message:  fmt.Sprintf("Rate limit exceeded for %s", state),
			Provider: "State",
			Retry:    true,
		}
	case http.StatusBadRequest:
		return &ProviderError{
			Code:     ErrCodeInvalidRequest,
			Message:  fmt.Sprintf("Bad request to %s", state),
			Provider: "State",
			Retry:    false,
		}
	case http.StatusServiceUnavailable:
		return &ProviderError{
			Code:     ErrCodeProviderUnavailable,
			Message:  fmt.Sprintf("Service unavailable for %s", state),
			Provider: "State",
			Retry:    true,
		}
	default:
		return &ProviderError{
			Code:     ErrCodeProviderUnavailable,
			Message:  fmt.Sprintf("HTTP %d from %s", resp.StatusCode, state),
			Provider: "State",
			Retry:    resp.StatusCode >= 500,
		}
	}
}

// updateResponseTimeMetrics updates response time metrics
func (s *StateProvider) updateResponseTimeMetrics(duration time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.metrics.MinResponseTime == 0 || duration < s.metrics.MinResponseTime {
		s.metrics.MinResponseTime = duration
	}
	if duration > s.metrics.MaxResponseTime {
		s.metrics.MaxResponseTime = duration
	}

	totalRequests := s.metrics.RequestCount
	if totalRequests > 0 {
		s.metrics.AvgResponseTime = time.Duration(
			(int64(s.metrics.AvgResponseTime)*int64(totalRequests-1) + int64(duration)) / int64(totalRequests),
		)
	} else {
		s.metrics.AvgResponseTime = duration
	}

	s.metrics.LastRequestTime = time.Now()
}

// State API response types

// StateCheckResponse represents a response from a state check API
type StateCheckResponse struct {
	PhoneNumber      string `json:"phone_number"`
	IsListed         bool   `json:"is_listed"`
	RegistrationDate string `json:"registration_date,omitempty"`
	Reason           string `json:"reason,omitempty"`
	Registry         string `json:"registry,omitempty"`
}

// StateBatchRequest represents a batch check request to a state API
type StateBatchRequest struct {
	Numbers []string `json:"numbers"`
	Format  string   `json:"format"`
}

// StateBatchResponse represents a batch check response from a state API
type StateBatchResponse struct {
	Results []StateCheckResponse `json:"results"`
}

// StateSyncResponse represents a sync response from a state API
type StateSyncResponse struct {
	Records []map[string]interface{} `json:"records"`
	Added   int                      `json:"added"`
	Updated int                      `json:"updated"`
	Deleted int                      `json:"deleted"`
	Version string                   `json:"version"`
}