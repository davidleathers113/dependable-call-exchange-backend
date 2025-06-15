package dnc

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"
)

// circuitBreaker implements a circuit breaker pattern for external provider calls
// It prevents cascade failures by opening the circuit when failure rate exceeds threshold
type circuitBreaker struct {
	config          CircuitBreakerConfig
	state           int32 // atomic: 0=closed, 1=open, 2=half-open
	lastFailureTime int64 // atomic: unix nano
	failureCount    int64 // atomic
	successCount    int64 // atomic
	totalCount      int64 // atomic
	mutex           sync.RWMutex
	onStateChange   func(from, to CircuitState)
}

// CircuitBreakerConfig configures circuit breaker behavior
type CircuitBreakerConfig struct {
	FailureThreshold   int           // Number of failures to open circuit
	SuccessThreshold   int           // Number of successes to close circuit from half-open
	Timeout            time.Duration // How long to wait before attempting half-open
	ResetTimeout       time.Duration // How long to wait before resetting failure count
	MaxRequests        int           // Max requests allowed in half-open state
	FailureRateThreshold float64     // Failure rate (0-1) to trigger opening
	MinRequests        int           // Min requests before failure rate is considered
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(config CircuitBreakerConfig) CircuitBreaker {
	if config.FailureThreshold <= 0 {
		config.FailureThreshold = 5
	}
	if config.SuccessThreshold <= 0 {
		config.SuccessThreshold = 3
	}
	if config.Timeout <= 0 {
		config.Timeout = 60 * time.Second
	}
	if config.ResetTimeout <= 0 {
		config.ResetTimeout = 300 * time.Second
	}
	if config.MaxRequests <= 0 {
		config.MaxRequests = 10
	}
	if config.FailureRateThreshold <= 0 {
		config.FailureRateThreshold = 0.5
	}
	if config.MinRequests <= 0 {
		config.MinRequests = 10
	}

	return &circuitBreaker{
		config: config,
		state:  0, // closed
	}
}

// Execute runs the request function through the circuit breaker
func (cb *circuitBreaker) Execute(ctx context.Context, req func() (interface{}, error)) (interface{}, error) {
	// Check if we should execute the request
	if !cb.allowRequest() {
		return nil, ErrCircuitBreakerOpen
	}

	// Execute the request
	result, err := req()

	// Record the result
	if err != nil {
		cb.recordFailure()
	} else {
		cb.recordSuccess()
	}

	return result, err
}

// GetState returns the current circuit state
func (cb *circuitBreaker) GetState() CircuitState {
	state := atomic.LoadInt32(&cb.state)
	switch state {
	case 0:
		return CircuitClosed
	case 1:
		return CircuitOpen
	case 2:
		return CircuitHalfOpen
	default:
		return CircuitClosed
	}
}

// Reset manually resets the circuit breaker to closed state
func (cb *circuitBreaker) Reset() {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	oldState := cb.GetState()
	atomic.StoreInt32(&cb.state, 0) // closed
	atomic.StoreInt64(&cb.failureCount, 0)
	atomic.StoreInt64(&cb.successCount, 0)
	atomic.StoreInt64(&cb.totalCount, 0)
	atomic.StoreInt64(&cb.lastFailureTime, 0)

	if cb.onStateChange != nil && oldState != CircuitClosed {
		cb.onStateChange(oldState, CircuitClosed)
	}
}

// SetStateChangeCallback sets a callback for state changes
func (cb *circuitBreaker) SetStateChangeCallback(callback func(from, to CircuitState)) {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()
	cb.onStateChange = callback
}

// GetStats returns circuit breaker statistics
func (cb *circuitBreaker) GetStats() CircuitBreakerStats {
	failures := atomic.LoadInt64(&cb.failureCount)
	successes := atomic.LoadInt64(&cb.successCount)
	total := atomic.LoadInt64(&cb.totalCount)

	var failureRate float64
	if total > 0 {
		failureRate = float64(failures) / float64(total)
	}

	return CircuitBreakerStats{
		State:               cb.GetState(),
		FailureCount:        failures,
		SuccessCount:        successes,
		TotalRequests:       total,
		FailureRate:         failureRate,
		LastFailureTime:     time.Unix(0, atomic.LoadInt64(&cb.lastFailureTime)),
		ConsecutiveFailures: failures, // Simplified
	}
}

// allowRequest determines if a request should be allowed through
func (cb *circuitBreaker) allowRequest() bool {
	state := atomic.LoadInt32(&cb.state)
	
	switch state {
	case 0: // closed
		return true
		
	case 1: // open
		// Check if we should transition to half-open
		lastFailure := atomic.LoadInt64(&cb.lastFailureTime)
		if time.Since(time.Unix(0, lastFailure)) > cb.config.Timeout {
			// Try to transition to half-open
			if atomic.CompareAndSwapInt32(&cb.state, 1, 2) {
				cb.notifyStateChange(CircuitOpen, CircuitHalfOpen)
			}
			return true
		}
		return false
		
	case 2: // half-open
		// Allow limited requests
		total := atomic.LoadInt64(&cb.totalCount)
		return total < int64(cb.config.MaxRequests)
		
	default:
		return false
	}
}

// recordFailure records a failed request
func (cb *circuitBreaker) recordFailure() {
	atomic.AddInt64(&cb.failureCount, 1)
	atomic.AddInt64(&cb.totalCount, 1)
	atomic.StoreInt64(&cb.lastFailureTime, time.Now().UnixNano())

	// Check if we should open the circuit
	cb.checkShouldOpen()
}

// recordSuccess records a successful request
func (cb *circuitBreaker) recordSuccess() {
	atomic.AddInt64(&cb.successCount, 1)
	atomic.AddInt64(&cb.totalCount, 1)

	// Check if we should close the circuit (from half-open)
	state := atomic.LoadInt32(&cb.state)
	if state == 2 { // half-open
		successes := atomic.LoadInt64(&cb.successCount)
		if successes >= int64(cb.config.SuccessThreshold) {
			if atomic.CompareAndSwapInt32(&cb.state, 2, 0) {
				cb.notifyStateChange(CircuitHalfOpen, CircuitClosed)
				// Reset counters
				atomic.StoreInt64(&cb.failureCount, 0)
				atomic.StoreInt64(&cb.successCount, 0)
				atomic.StoreInt64(&cb.totalCount, 0)
			}
		}
	}
}

// checkShouldOpen determines if the circuit should be opened
func (cb *circuitBreaker) checkShouldOpen() {
	state := atomic.LoadInt32(&cb.state)
	if state != 0 { // not closed
		return
	}

	failures := atomic.LoadInt64(&cb.failureCount)
	total := atomic.LoadInt64(&cb.totalCount)

	// Check failure count threshold
	if failures >= int64(cb.config.FailureThreshold) {
		if atomic.CompareAndSwapInt32(&cb.state, 0, 1) {
			cb.notifyStateChange(CircuitClosed, CircuitOpen)
		}
		return
	}

	// Check failure rate threshold
	if total >= int64(cb.config.MinRequests) {
		failureRate := float64(failures) / float64(total)
		if failureRate >= cb.config.FailureRateThreshold {
			if atomic.CompareAndSwapInt32(&cb.state, 0, 1) {
				cb.notifyStateChange(CircuitClosed, CircuitOpen)
			}
		}
	}
}

// notifyStateChange notifies about state changes
func (cb *circuitBreaker) notifyStateChange(from, to CircuitState) {
	if cb.onStateChange != nil {
		go cb.onStateChange(from, to)
	}
}

// CircuitBreakerStats represents circuit breaker statistics
type CircuitBreakerStats struct {
	State               CircuitState  `json:"state"`
	FailureCount        int64         `json:"failure_count"`
	SuccessCount        int64         `json:"success_count"`
	TotalRequests       int64         `json:"total_requests"`
	FailureRate         float64       `json:"failure_rate"`
	LastFailureTime     time.Time     `json:"last_failure_time"`
	ConsecutiveFailures int64         `json:"consecutive_failures"`
}

// Common errors
var (
	ErrCircuitBreakerOpen = errors.New("circuit breaker is open")
)

// Multi-provider circuit breaker that manages multiple individual circuit breakers
type multiProviderCircuitBreaker struct {
	breakers map[string]CircuitBreaker
	config   CircuitBreakerConfig
	mutex    sync.RWMutex
}

// NewMultiProviderCircuitBreaker creates a circuit breaker that manages multiple providers
func NewMultiProviderCircuitBreaker(config CircuitBreakerConfig) *multiProviderCircuitBreaker {
	return &multiProviderCircuitBreaker{
		breakers: make(map[string]CircuitBreaker),
		config:   config,
	}
}

// ExecuteForProvider executes a request for a specific provider
func (m *multiProviderCircuitBreaker) ExecuteForProvider(ctx context.Context, providerID string, req func() (interface{}, error)) (interface{}, error) {
	breaker := m.getBreakerForProvider(providerID)
	return breaker.Execute(ctx, req)
}

// GetProviderState returns the circuit state for a specific provider
func (m *multiProviderCircuitBreaker) GetProviderState(providerID string) CircuitState {
	breaker := m.getBreakerForProvider(providerID)
	return breaker.GetState()
}

// ResetProvider resets the circuit breaker for a specific provider
func (m *multiProviderCircuitBreaker) ResetProvider(providerID string) {
	breaker := m.getBreakerForProvider(providerID)
	breaker.Reset()
}

// GetProviderStats returns statistics for a specific provider
func (m *multiProviderCircuitBreaker) GetProviderStats(providerID string) CircuitBreakerStats {
	breaker := m.getBreakerForProvider(providerID)
	if cb, ok := breaker.(*circuitBreaker); ok {
		return cb.GetStats()
	}
	return CircuitBreakerStats{State: breaker.GetState()}
}

// GetAllStats returns statistics for all providers
func (m *multiProviderCircuitBreaker) GetAllStats() map[string]CircuitBreakerStats {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	stats := make(map[string]CircuitBreakerStats)
	for providerID, breaker := range m.breakers {
		if cb, ok := breaker.(*circuitBreaker); ok {
			stats[providerID] = cb.GetStats()
		} else {
			stats[providerID] = CircuitBreakerStats{State: breaker.GetState()}
		}
	}
	return stats
}

// getBreakerForProvider gets or creates a circuit breaker for a provider
func (m *multiProviderCircuitBreaker) getBreakerForProvider(providerID string) CircuitBreaker {
	m.mutex.RLock()
	breaker, exists := m.breakers[providerID]
	m.mutex.RUnlock()

	if exists {
		return breaker
	}

	// Create new breaker with write lock
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Double-check after acquiring write lock
	if breaker, exists := m.breakers[providerID]; exists {
		return breaker
	}

	// Create new circuit breaker
	breaker = NewCircuitBreaker(m.config)
	m.breakers[providerID] = breaker
	return breaker
}

// RemoveProvider removes a circuit breaker for a provider
func (m *multiProviderCircuitBreaker) RemoveProvider(providerID string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	delete(m.breakers, providerID)
}

// SetStateChangeCallback sets a callback for state changes on all breakers
func (m *multiProviderCircuitBreaker) SetStateChangeCallback(callback func(providerID string, from, to CircuitState)) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	for providerID, breaker := range m.breakers {
		if cb, ok := breaker.(*circuitBreaker); ok {
			// Capture providerID in closure
			id := providerID
			cb.SetStateChangeCallback(func(from, to CircuitState) {
				callback(id, from, to)
			})
		}
	}
}