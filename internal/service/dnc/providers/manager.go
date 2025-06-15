package providers

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/dnc"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/infrastructure/cache"
	"go.uber.org/zap"
)

// ProviderManager orchestrates multiple DNC providers with load balancing,
// health checking, failover, and provider discovery capabilities
type ProviderManager struct {
	logger           *zap.Logger
	config           ManagerConfig
	providers        map[dnc.ProviderType][]ProviderClient
	primaryProvider  map[dnc.ProviderType]ProviderClient
	fallbackProvider map[dnc.ProviderType]ProviderClient
	cache            cache.Interface
	mu               sync.RWMutex

	// Load balancing state
	roundRobinIndex map[dnc.ProviderType]int
	lastUsed        map[dnc.ProviderType]time.Time

	// Health monitoring
	healthCheckers   map[string]*HealthChecker
	healthStatus     map[string]*HealthCheckResult
	healthMu         sync.RWMutex

	// Circuit breaker for provider types
	circuitBreakers  map[dnc.ProviderType]*MultiProviderCircuitBreaker

	// Metrics and monitoring
	metrics          *ManagerMetrics
	alertThresholds  AlertThresholds

	// Lifecycle management
	running          bool
	stopCh           chan struct{}
	wg               sync.WaitGroup
}

// ManagerConfig contains configuration for the provider manager
type ManagerConfig struct {
	// Provider discovery
	ProviderDiscovery    ProviderDiscoveryConfig `json:"provider_discovery"`
	AutoDiscovery        bool                    `json:"auto_discovery"`
	DiscoveryInterval    time.Duration           `json:"discovery_interval"`

	// Load balancing
	LoadBalanceStrategy  LoadBalanceStrategy     `json:"load_balance_strategy"`
	FailoverEnabled      bool                    `json:"failover_enabled"`
	FailoverTimeout      time.Duration           `json:"failover_timeout"`

	// Health checking
	HealthCheckInterval  time.Duration           `json:"health_check_interval"`
	HealthCheckTimeout   time.Duration           `json:"health_check_timeout"`
	UnhealthyThreshold   int                     `json:"unhealthy_threshold"`
	HealthyThreshold     int                     `json:"healthy_threshold"`

	// Rate limiting per provider type
	RateLimits          map[dnc.ProviderType]RateLimitConfig `json:"rate_limits"`

	// Circuit breaker configuration
	CircuitConfig       CircuitConfig           `json:"circuit_config"`

	// Cache configuration
	CacheEnabled        bool                    `json:"cache_enabled"`
	CacheTTL            time.Duration           `json:"cache_ttl"`
	CacheKeyPrefix      string                  `json:"cache_key_prefix"`

	// Performance monitoring
	MetricsEnabled      bool                    `json:"metrics_enabled"`
	SlowQueryThreshold  time.Duration           `json:"slow_query_threshold"`

	// Provider priorities
	ProviderPriorities  map[dnc.ProviderType]int `json:"provider_priorities"`
}

// ProviderDiscoveryConfig contains provider discovery settings
type ProviderDiscoveryConfig struct {
	Enabled          bool              `json:"enabled"`
	DiscoveryURL     string            `json:"discovery_url"`
	Credentials      map[string]string `json:"credentials"`
	RefreshInterval  time.Duration     `json:"refresh_interval"`
	TimeoutSeconds   int               `json:"timeout_seconds"`
}

// LoadBalanceStrategy defines load balancing approaches
type LoadBalanceStrategy string

const (
	LoadBalanceRoundRobin   LoadBalanceStrategy = "round_robin"
	LoadBalanceWeighted     LoadBalanceStrategy = "weighted"
	LoadBalanceLeastLatency LoadBalanceStrategy = "least_latency"
	LoadBalanceRandom       LoadBalanceStrategy = "random"
	LoadBalancePriority     LoadBalanceStrategy = "priority"
)

// HealthChecker manages health checking for a provider
type HealthChecker struct {
	provider        ProviderClient
	logger          *zap.Logger
	checkInterval   time.Duration
	timeout         time.Duration
	unhealthyCount  int
	healthyCount    int
	lastCheck       time.Time
	lastResult      *HealthCheckResult
	mu              sync.RWMutex
}

// ManagerMetrics tracks provider manager performance
type ManagerMetrics struct {
	TotalRequests       int64                           `json:"total_requests"`
	SuccessfulRequests  int64                           `json:"successful_requests"`
	FailedRequests      int64                           `json:"failed_requests"`
	CacheHits           int64                           `json:"cache_hits"`
	CacheMisses         int64                           `json:"cache_misses"`
	ProviderMetrics     map[string]*ProviderMetrics     `json:"provider_metrics"`
	LatencyPercentiles  map[string]time.Duration        `json:"latency_percentiles"`
	FailoverEvents      int64                           `json:"failover_events"`
	CircuitBreakerTrips int64                           `json:"circuit_breaker_trips"`
	LastUpdated         time.Time                       `json:"last_updated"`
	mu                  sync.RWMutex
}

// AlertThresholds defines when to trigger alerts
type AlertThresholds struct {
	MaxFailureRate     float64       `json:"max_failure_rate"`
	MaxLatency         time.Duration `json:"max_latency"`
	MinSuccessRate     float64       `json:"min_success_rate"`
	CircuitBreakerTrips int64        `json:"circuit_breaker_trips"`
}

// MultiProviderCircuitBreaker manages circuit breakers for provider types
type MultiProviderCircuitBreaker struct {
	breakers map[string]*CircuitBreaker
	config   CircuitConfig
	mu       sync.RWMutex
}

// NewProviderManager creates a new DNC provider manager
func NewProviderManager(
	logger *zap.Logger,
	config ManagerConfig,
	cacheInterface cache.Interface,
) *ProviderManager {
	if config.LoadBalanceStrategy == "" {
		config.LoadBalanceStrategy = LoadBalanceRoundRobin
	}
	if config.HealthCheckInterval == 0 {
		config.HealthCheckInterval = 30 * time.Second
	}
	if config.HealthCheckTimeout == 0 {
		config.HealthCheckTimeout = 10 * time.Second
	}
	if config.UnhealthyThreshold == 0 {
		config.UnhealthyThreshold = 3
	}
	if config.HealthyThreshold == 0 {
		config.HealthyThreshold = 2
	}
	if config.CacheTTL == 0 {
		config.CacheTTL = 5 * time.Minute
	}
	if config.SlowQueryThreshold == 0 {
		config.SlowQueryThreshold = 2 * time.Second
	}

	return &ProviderManager{
		logger:           logger,
		config:           config,
		providers:        make(map[dnc.ProviderType][]ProviderClient),
		primaryProvider:  make(map[dnc.ProviderType]ProviderClient),
		fallbackProvider: make(map[dnc.ProviderType]ProviderClient),
		cache:            cacheInterface,
		roundRobinIndex:  make(map[dnc.ProviderType]int),
		lastUsed:         make(map[dnc.ProviderType]time.Time),
		healthCheckers:   make(map[string]*HealthChecker),
		healthStatus:     make(map[string]*HealthCheckResult),
		circuitBreakers:  make(map[dnc.ProviderType]*MultiProviderCircuitBreaker),
		metrics:          &ManagerMetrics{ProviderMetrics: make(map[string]*ProviderMetrics)},
		alertThresholds:  AlertThresholds{
			MaxFailureRate:     0.05, // 5%
			MaxLatency:         5 * time.Second,
			MinSuccessRate:     0.95, // 95%
			CircuitBreakerTrips: 5,
		},
		stopCh:           make(chan struct{}),
	}
}

// RegisterProvider registers a provider with the manager
func (pm *ProviderManager) RegisterProvider(provider ProviderClient) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	providerType := provider.GetProviderType()
	providerName := provider.GetProviderName()

	pm.logger.Info("Registering DNC provider",
		zap.String("provider_type", string(providerType)),
		zap.String("provider_name", providerName),
	)

	// Add to providers list
	if pm.providers[providerType] == nil {
		pm.providers[providerType] = make([]ProviderClient, 0)
	}
	pm.providers[providerType] = append(pm.providers[providerType], provider)

	// Set as primary if it's the first of this type
	if pm.primaryProvider[providerType] == nil {
		pm.primaryProvider[providerType] = provider
	}

	// Set up health checker
	healthChecker := &HealthChecker{
		provider:      provider,
		logger:        pm.logger.Named("health_checker"),
		checkInterval: pm.config.HealthCheckInterval,
		timeout:       pm.config.HealthCheckTimeout,
	}
	pm.healthCheckers[providerName] = healthChecker

	// Initialize circuit breaker for provider type if not exists
	if pm.circuitBreakers[providerType] == nil {
		pm.circuitBreakers[providerType] = &MultiProviderCircuitBreaker{
			breakers: make(map[string]*CircuitBreaker),
			config:   pm.config.CircuitConfig,
		}
	}

	// Initialize provider metrics
	pm.metrics.mu.Lock()
	pm.metrics.ProviderMetrics[providerName] = &ProviderMetrics{
		ProviderName: providerName,
		ProviderType: string(providerType),
	}
	pm.metrics.mu.Unlock()

	return nil
}

// UnregisterProvider removes a provider from the manager
func (pm *ProviderManager) UnregisterProvider(providerType dnc.ProviderType, providerName string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.logger.Info("Unregistering DNC provider",
		zap.String("provider_type", string(providerType)),
		zap.String("provider_name", providerName),
	)

	// Remove from providers list
	providers := pm.providers[providerType]
	for i, provider := range providers {
		if provider.GetProviderName() == providerName {
			pm.providers[providerType] = append(providers[:i], providers[i+1:]...)
			break
		}
	}

	// Update primary provider if necessary
	if pm.primaryProvider[providerType] != nil && pm.primaryProvider[providerType].GetProviderName() == providerName {
		if len(pm.providers[providerType]) > 0 {
			pm.primaryProvider[providerType] = pm.providers[providerType][0]
		} else {
			delete(pm.primaryProvider, providerType)
		}
	}

	// Clean up health checker
	delete(pm.healthCheckers, providerName)
	delete(pm.healthStatus, providerName)

	// Clean up metrics
	pm.metrics.mu.Lock()
	delete(pm.metrics.ProviderMetrics, providerName)
	pm.metrics.mu.Unlock()

	return nil
}

// CheckNumber performs a DNC check using the best available provider
func (pm *ProviderManager) CheckNumber(ctx context.Context, phoneNumber values.PhoneNumber, providerTypes ...dnc.ProviderType) (*CheckResult, error) {
	startTime := time.Now()
	defer func() {
		pm.updateMetrics(time.Since(startTime), true)
	}()

	// Determine which provider types to check
	typesToCheck := providerTypes
	if len(typesToCheck) == 0 {
		// Default order: Federal -> State -> Internal
		typesToCheck = []dnc.ProviderType{
			dnc.ProviderTypeFederal,
			dnc.ProviderTypeState,
			dnc.ProviderTypeInternal,
		}
	}

	// Check cache first if enabled
	if pm.config.CacheEnabled {
		cacheKey := pm.generateCacheKey(phoneNumber, typesToCheck)
		if cachedResult, found := pm.getFromCache(cacheKey); found {
			pm.metrics.mu.Lock()
			pm.metrics.CacheHits++
			pm.metrics.mu.Unlock()
			return cachedResult, nil
		}
		pm.metrics.mu.Lock()
		pm.metrics.CacheMisses++
		pm.metrics.mu.Unlock()
	}

	// Try each provider type in order
	var lastError error
	for _, providerType := range typesToCheck {
		result, err := pm.checkWithProviderType(ctx, phoneNumber, providerType)
		if err == nil {
			// Cache successful result
			if pm.config.CacheEnabled {
				cacheKey := pm.generateCacheKey(phoneNumber, []dnc.ProviderType{providerType})
				pm.cacheResult(cacheKey, result)
			}
			return result, nil
		}
		lastError = err
		pm.logger.Warn("Provider check failed, trying next",
			zap.String("provider_type", string(providerType)),
			zap.String("phone_number", phoneNumber.String()),
			zap.Error(err),
		)
	}

	pm.updateMetrics(time.Since(startTime), false)
	return nil, fmt.Errorf("all providers failed, last error: %w", lastError)
}

// BatchCheckNumbers performs batch DNC checks
func (pm *ProviderManager) BatchCheckNumbers(ctx context.Context, phoneNumbers []values.PhoneNumber, providerTypes ...dnc.ProviderType) ([]*CheckResult, error) {
	startTime := time.Now()
	defer func() {
		pm.updateMetrics(time.Since(startTime), true)
	}()

	results := make([]*CheckResult, 0, len(phoneNumbers))

	// Group numbers by best provider
	providerGroups := make(map[ProviderClient][]values.PhoneNumber)

	typesToCheck := providerTypes
	if len(typesToCheck) == 0 {
		typesToCheck = []dnc.ProviderType{dnc.ProviderTypeFederal, dnc.ProviderTypeState}
	}

	for _, phoneNumber := range phoneNumbers {
		for _, providerType := range typesToCheck {
			provider := pm.selectProvider(providerType)
			if provider != nil {
				providerGroups[provider] = append(providerGroups[provider], phoneNumber)
				break
			}
		}
	}

	// Execute batch checks in parallel
	var wg sync.WaitGroup
	resultsCh := make(chan []*CheckResult, len(providerGroups))
	errorsCh := make(chan error, len(providerGroups))

	for provider, numbers := range providerGroups {
		wg.Add(1)
		go func(p ProviderClient, nums []values.PhoneNumber) {
			defer wg.Done()

			batchResults, err := p.BatchCheckNumbers(ctx, nums)
			if err != nil {
				errorsCh <- err
				return
			}
			resultsCh <- batchResults
		}(provider, numbers)
	}

	go func() {
		wg.Wait()
		close(resultsCh)
		close(errorsCh)
	}()

	// Collect results
	for batchResult := range resultsCh {
		results = append(results, batchResult...)
	}

	// Check for errors
	select {
	case err := <-errorsCh:
		if err != nil {
			pm.updateMetrics(time.Since(startTime), false)
			return results, fmt.Errorf("batch check failed: %w", err)
		}
	default:
	}

	return results, nil
}

// HealthCheck performs health checks on all providers
func (pm *ProviderManager) HealthCheck(ctx context.Context) (map[string]*HealthCheckResult, error) {
	pm.healthMu.RLock()
	defer pm.healthMu.RUnlock()

	results := make(map[string]*HealthCheckResult)
	for providerName, status := range pm.healthStatus {
		results[providerName] = status
	}

	return results, nil
}

// GetProviderStatus returns status for all providers
func (pm *ProviderManager) GetProviderStatus(ctx context.Context) (*ProviderStatusResponse, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	status := &ProviderStatusResponse{
		Providers:       make(map[string]*ProviderStatus),
		LastUpdated:     time.Now(),
		TotalProviders:  0,
		HealthyProviders: 0,
	}

	for providerType, providers := range pm.providers {
		for _, provider := range providers {
			providerName := provider.GetProviderName()
			status.TotalProviders++

			pm.healthMu.RLock()
			healthResult := pm.healthStatus[providerName]
			pm.healthMu.RUnlock()

			providerStatus := &ProviderStatus{
				ProviderName:   providerName,
				ProviderType:   string(providerType),
				Healthy:        healthResult != nil && healthResult.Healthy,
				LastHealthCheck: time.Now(),
				CircuitState:   "closed",
			}

			if healthResult != nil {
				providerStatus.LastHealthCheck = healthResult.Timestamp
				providerStatus.ResponseTime = healthResult.ResponseTime
				providerStatus.ErrorMessage = healthResult.Error
			}

			if providerStatus.Healthy {
				status.HealthyProviders++
			}

			status.Providers[providerName] = providerStatus
		}
	}

	return status, nil
}

// GetMetrics returns manager performance metrics
func (pm *ProviderManager) GetMetrics() *ManagerMetrics {
	pm.metrics.mu.RLock()
	defer pm.metrics.mu.RUnlock()

	// Create a copy to avoid concurrent access issues
	metricsCopy := &ManagerMetrics{
		TotalRequests:       pm.metrics.TotalRequests,
		SuccessfulRequests:  pm.metrics.SuccessfulRequests,
		FailedRequests:      pm.metrics.Failed_requests,
		CacheHits:           pm.metrics.CacheHits,
		CacheMisses:         pm.metrics.CacheMisses,
		ProviderMetrics:     make(map[string]*ProviderMetrics),
		LatencyPercentiles:  make(map[string]time.Duration),
		FailoverEvents:      pm.metrics.FailoverEvents,
		CircuitBreakerTrips: pm.metrics.CircuitBreakerTrips,
		LastUpdated:         pm.metrics.LastUpdated,
	}

	// Copy provider metrics
	for name, metrics := range pm.metrics.ProviderMetrics {
		metricsCopy.ProviderMetrics[name] = metrics
	}

	// Copy latency percentiles
	for percentile, latency := range pm.metrics.LatencyPercentiles {
		metricsCopy.LatencyPercentiles[percentile] = latency
	}

	return metricsCopy
}

// Start begins provider manager operations
func (pm *ProviderManager) Start(ctx context.Context) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.running {
		return fmt.Errorf("provider manager already running")
	}

	pm.logger.Info("Starting DNC provider manager")

	// Start health checkers
	for _, healthChecker := range pm.healthCheckers {
		pm.wg.Add(1)
		go pm.runHealthChecker(healthChecker)
	}

	// Start provider discovery if enabled
	if pm.config.AutoDiscovery && pm.config.ProviderDiscovery.Enabled {
		pm.wg.Add(1)
		go pm.runProviderDiscovery()
	}

	// Start metrics collection
	if pm.config.MetricsEnabled {
		pm.wg.Add(1)
		go pm.runMetricsCollection()
	}

	pm.running = true
	return nil
}

// Stop gracefully shuts down the provider manager
func (pm *ProviderManager) Stop(ctx context.Context) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if !pm.running {
		return nil
	}

	pm.logger.Info("Stopping DNC provider manager")

	// Signal all goroutines to stop
	close(pm.stopCh)

	// Wait for all goroutines to finish
	done := make(chan struct{})
	go func() {
		pm.wg.Wait()
		close(done)
	}()

	// Wait for shutdown or timeout
	select {
	case <-done:
		pm.logger.Info("Provider manager stopped successfully")
	case <-ctx.Done():
		pm.logger.Warn("Provider manager stop timed out")
		return ctx.Err()
	}

	pm.running = false
	return nil
}

// Helper methods

func (pm *ProviderManager) checkWithProviderType(ctx context.Context, phoneNumber values.PhoneNumber, providerType dnc.ProviderType) (*CheckResult, error) {
	provider := pm.selectProvider(providerType)
	if provider == nil {
		return nil, fmt.Errorf("no available provider for type %s", providerType)
	}

	// Check circuit breaker
	circuitBreaker := pm.circuitBreakers[providerType]
	if circuitBreaker != nil {
		providerName := provider.GetProviderName()
		if state := circuitBreaker.GetProviderState(providerName); state == CircuitOpen {
			return nil, fmt.Errorf("circuit breaker open for provider %s", providerName)
		}
	}

	return provider.CheckNumber(ctx, phoneNumber)
}

func (pm *ProviderManager) selectProvider(providerType dnc.ProviderType) ProviderClient {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	providers := pm.providers[providerType]
	if len(providers) == 0 {
		return nil
	}

	// Filter healthy providers
	healthyProviders := make([]ProviderClient, 0)
	for _, provider := range providers {
		pm.healthMu.RLock()
		status := pm.healthStatus[provider.GetProviderName()]
		pm.healthMu.RUnlock()

		if status != nil && status.Healthy {
			healthyProviders = append(healthyProviders, provider)
		}
	}

	if len(healthyProviders) == 0 {
		// Fallback to any available provider
		healthyProviders = providers
	}

	// Apply load balancing strategy
	switch pm.config.LoadBalanceStrategy {
	case LoadBalanceRoundRobin:
		return pm.selectRoundRobin(providerType, healthyProviders)
	case LoadBalanceRandom:
		return pm.selectRandom(healthyProviders)
	case LoadBalancePriority:
		return pm.selectByPriority(providerType, healthyProviders)
	case LoadBalanceLeastLatency:
		return pm.selectLeastLatency(healthyProviders)
	default:
		return healthyProviders[0]
	}
}

func (pm *ProviderManager) selectRoundRobin(providerType dnc.ProviderType, providers []ProviderClient) ProviderClient {
	if len(providers) == 0 {
		return nil
	}

	index := pm.roundRobinIndex[providerType]
	provider := providers[index%len(providers)]
	pm.roundRobinIndex[providerType] = (index + 1) % len(providers)
	return provider
}

func (pm *ProviderManager) selectRandom(providers []ProviderClient) ProviderClient {
	if len(providers) == 0 {
		return nil
	}
	return providers[rand.Intn(len(providers))]
}

func (pm *ProviderManager) selectByPriority(providerType dnc.ProviderType, providers []ProviderClient) ProviderClient {
	if len(providers) == 0 {
		return nil
	}

	priorities := pm.config.ProviderPriorities
	if len(priorities) == 0 {
		return providers[0]
	}

	// Sort by priority
	sort.Slice(providers, func(i, j int) bool {
		priI := priorities[providers[i].GetProviderType()]
		priJ := priorities[providers[j].GetProviderType()]
		return priI < priJ // Lower number = higher priority
	})

	return providers[0]
}

func (pm *ProviderManager) selectLeastLatency(providers []ProviderClient) ProviderClient {
	if len(providers) == 0 {
		return nil
	}

	// For now, return first provider
	// In a real implementation, you'd track latency metrics
	return providers[0]
}

func (pm *ProviderManager) generateCacheKey(phoneNumber values.PhoneNumber, providerTypes []dnc.ProviderType) string {
	key := fmt.Sprintf("%s:dnc:%s", pm.config.CacheKeyPrefix, phoneNumber.String())
	for _, pt := range providerTypes {
		key += ":" + string(pt)
	}
	return key
}

func (pm *ProviderManager) getFromCache(key string) (*CheckResult, bool) {
	if pm.cache == nil {
		return nil, false
	}

	data, found := pm.cache.Get(key)
	if !found {
		return nil, false
	}

	result, ok := data.(*CheckResult)
	return result, ok
}

func (pm *ProviderManager) cacheResult(key string, result *CheckResult) {
	if pm.cache == nil {
		return
	}

	pm.cache.Set(key, result, pm.config.CacheTTL)
}

func (pm *ProviderManager) updateMetrics(latency time.Duration, success bool) {
	pm.metrics.mu.Lock()
	defer pm.metrics.mu.Unlock()

	pm.metrics.TotalRequests++
	if success {
		pm.metrics.SuccessfulRequests++
	} else {
		pm.metrics.FailedRequests++
	}
	pm.metrics.LastUpdated = time.Now()

	// Update latency metrics (simplified)
	if latency > pm.config.SlowQueryThreshold {
		pm.logger.Warn("Slow DNC query detected",
			zap.Duration("latency", latency),
			zap.Duration("threshold", pm.config.SlowQueryThreshold),
		)
	}
}

func (pm *ProviderManager) runHealthChecker(checker *HealthChecker) {
	defer pm.wg.Done()

	ticker := time.NewTicker(checker.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-pm.stopCh:
			return
		case <-ticker.C:
			pm.performHealthCheck(checker)
		}
	}
}

func (pm *ProviderManager) performHealthCheck(checker *HealthChecker) {
	ctx, cancel := context.WithTimeout(context.Background(), checker.timeout)
	defer cancel()

	startTime := time.Now()
	result, err := checker.provider.HealthCheck(ctx)
	responseTime := time.Since(startTime)

	checker.mu.Lock()
	checker.lastCheck = time.Now()

	if err != nil || !result.Healthy {
		checker.unhealthyCount++
		checker.healthyCount = 0
		result = &HealthCheckResult{
			Healthy:      false,
			Error:        err.Error(),
			ResponseTime: responseTime,
			Timestamp:    time.Now(),
		}
	} else {
		checker.healthyCount++
		checker.unhealthyCount = 0
		result.ResponseTime = responseTime
		result.Timestamp = time.Now()
	}

	checker.lastResult = result
	checker.mu.Unlock()

	// Update global health status
	pm.healthMu.Lock()
	pm.healthStatus[checker.provider.GetProviderName()] = result
	pm.healthMu.Unlock()

	// Log health status changes
	if err != nil {
		pm.logger.Warn("Provider health check failed",
			zap.String("provider", checker.provider.GetProviderName()),
			zap.Error(err),
			zap.Duration("response_time", responseTime),
		)
	}
}

func (pm *ProviderManager) runProviderDiscovery() {
	defer pm.wg.Done()

	ticker := time.NewTicker(pm.config.DiscoveryInterval)
	defer ticker.Stop()

	for {
		select {
		case <-pm.stopCh:
			return
		case <-ticker.C:
			pm.performProviderDiscovery()
		}
	}
}

func (pm *ProviderManager) performProviderDiscovery() {
	// Implementation would discover new providers from a registry
	// For now, this is a placeholder
	pm.logger.Debug("Performing provider discovery")
}

func (pm *ProviderManager) runMetricsCollection() {
	defer pm.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-pm.stopCh:
			return
		case <-ticker.C:
			pm.collectMetrics()
		}
	}
}

func (pm *ProviderManager) collectMetrics() {
	// Update latency percentiles and other derived metrics
	pm.metrics.mu.Lock()
	defer pm.metrics.mu.Unlock()

	// Calculate success rate
	total := pm.metrics.TotalRequests
	if total > 0 {
		successRate := float64(pm.metrics.SuccessfulRequests) / float64(total)
		if successRate < pm.alertThresholds.MinSuccessRate {
			pm.logger.Warn("Low success rate detected",
				zap.Float64("success_rate", successRate),
				zap.Float64("threshold", pm.alertThresholds.MinSuccessRate),
			)
		}
	}
}

// Response types

type ProviderStatusResponse struct {
	Providers        map[string]*ProviderStatus `json:"providers"`
	LastUpdated      time.Time                  `json:"last_updated"`
	TotalProviders   int                        `json:"total_providers"`
	HealthyProviders int                        `json:"healthy_providers"`
}

type ProviderStatus struct {
	ProviderName    string        `json:"provider_name"`
	ProviderType    string        `json:"provider_type"`
	Healthy         bool          `json:"healthy"`
	LastHealthCheck time.Time     `json:"last_health_check"`
	ResponseTime    time.Duration `json:"response_time"`
	ErrorMessage    string        `json:"error_message,omitempty"`
	CircuitState    string        `json:"circuit_state"`
}

// DefaultManagerConfig returns a sensible default configuration
func DefaultManagerConfig() ManagerConfig {
	return ManagerConfig{
		ProviderDiscovery: ProviderDiscoveryConfig{
			Enabled:         false,
			RefreshInterval: 1 * time.Hour,
			TimeoutSeconds:  30,
		},
		AutoDiscovery:       false,
		DiscoveryInterval:   1 * time.Hour,
		LoadBalanceStrategy: LoadBalanceRoundRobin,
		FailoverEnabled:     true,
		FailoverTimeout:     5 * time.Second,
		HealthCheckInterval: 30 * time.Second,
		HealthCheckTimeout:  10 * time.Second,
		UnhealthyThreshold:  3,
		HealthyThreshold:    2,
		RateLimits: map[dnc.ProviderType]RateLimitConfig{
			dnc.ProviderTypeFederal: {
				RequestsPerSecond: 100,
				BurstSize:         200,
			},
			dnc.ProviderTypeState: {
				RequestsPerSecond: 50,
				BurstSize:         100,
			},
		},
		CircuitConfig: CircuitConfig{
			FailureThreshold:     5,
			SuccessThreshold:     3,
			Timeout:              60 * time.Second,
			ResetTimeout:         300 * time.Second,
			MaxRequests:          10,
			FailureRateThreshold: 0.5,
			MinRequests:          10,
		},
		CacheEnabled:       true,
		CacheTTL:           5 * time.Minute,
		CacheKeyPrefix:     "dnc",
		MetricsEnabled:     true,
		SlowQueryThreshold: 2 * time.Second,
		ProviderPriorities: map[dnc.ProviderType]int{
			dnc.ProviderTypeFederal:  1, // Highest priority
			dnc.ProviderTypeState:    2,
			dnc.ProviderTypeInternal: 3, // Lowest priority
		},
	}
}