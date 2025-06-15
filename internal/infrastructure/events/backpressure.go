package events

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// BackpressureController manages flow control for event publishing
type BackpressureController struct {
	maxDepth      int64
	currentDepth  atomic.Int64
	delay         time.Duration
	
	// Circuit breaker state
	state         atomic.Int32 // 0=closed, 1=open, 2=half-open
	failures      atomic.Int64
	lastFailTime  atomic.Int64 // Unix timestamp
	
	// Configuration
	failureThreshold int64
	resetTimeout     time.Duration
	halfOpenLimit    int64
	
	// Metrics
	metrics BackpressureMetrics
}

// BackpressureMetrics tracks backpressure statistics
type BackpressureMetrics struct {
	TotalRequests    atomic.Int64
	ThrottledRequests atomic.Int64
	RejectedRequests  atomic.Int64
	CircuitOpens     atomic.Int64
	AverageWaitTime  atomic.Int64 // in nanoseconds
	
	mu sync.RWMutex
	waitTimes []time.Duration
}

const (
	circuitClosed = iota
	circuitOpen
	circuitHalfOpen
)

// NewBackpressureController creates a new backpressure controller
func NewBackpressureController(maxDepth int, delay time.Duration) *BackpressureController {
	return &BackpressureController{
		maxDepth:         int64(maxDepth),
		delay:            delay,
		failureThreshold: 10,
		resetTimeout:     30 * time.Second,
		halfOpenLimit:    5,
	}
}

// Wait applies backpressure if needed
func (b *BackpressureController) Wait(ctx context.Context) error {
	b.metrics.TotalRequests.Add(1)
	
	// Check circuit breaker state
	if !b.canProceed() {
		b.metrics.RejectedRequests.Add(1)
		return ErrCircuitOpen
	}
	
	// Check current depth
	currentDepth := b.currentDepth.Load()
	if currentDepth >= b.maxDepth {
		b.metrics.ThrottledRequests.Add(1)
		
		// Apply backpressure delay
		start := time.Now()
		select {
		case <-time.After(b.delay):
			b.recordWaitTime(time.Since(start))
			// Recheck after delay
			if b.currentDepth.Load() >= b.maxDepth {
				b.recordFailure()
				return ErrQueueFull
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	
	// Increment depth
	b.currentDepth.Add(1)
	
	// Return a completion function to decrement depth
	go func() {
		select {
		case <-ctx.Done():
			b.currentDepth.Add(-1)
		}
	}()
	
	return nil
}

// Release decrements the current depth
func (b *BackpressureController) Release() {
	b.currentDepth.Add(-1)
	b.recordSuccess()
}

// GetMetrics returns current backpressure metrics
func (b *BackpressureController) GetMetrics() map[string]interface{} {
	b.metrics.mu.RLock()
	defer b.metrics.mu.RUnlock()
	
	avgWaitNs := b.metrics.AverageWaitTime.Load()
	avgWaitMs := float64(avgWaitNs) / 1e6
	
	return map[string]interface{}{
		"total_requests":     b.metrics.TotalRequests.Load(),
		"throttled_requests": b.metrics.ThrottledRequests.Load(),
		"rejected_requests":  b.metrics.RejectedRequests.Load(),
		"circuit_opens":      b.metrics.CircuitOpens.Load(),
		"average_wait_ms":    avgWaitMs,
		"current_depth":      b.currentDepth.Load(),
		"max_depth":          b.maxDepth,
		"circuit_state":      b.getStateName(),
	}
}

// Private methods

func (b *BackpressureController) canProceed() bool {
	state := b.state.Load()
	
	switch state {
	case circuitClosed:
		return true
		
	case circuitOpen:
		// Check if we should transition to half-open
		lastFail := b.lastFailTime.Load()
		if time.Since(time.Unix(lastFail, 0)) > b.resetTimeout {
			if b.state.CompareAndSwap(circuitOpen, circuitHalfOpen) {
				// Reset failure count for half-open testing
				b.failures.Store(0)
			}
			return true
		}
		return false
		
	case circuitHalfOpen:
		// Allow limited requests in half-open state
		return b.failures.Load() < b.halfOpenLimit
		
	default:
		return false
	}
}

func (b *BackpressureController) recordFailure() {
	failures := b.failures.Add(1)
	b.lastFailTime.Store(time.Now().Unix())
	
	state := b.state.Load()
	
	// Check if we should open the circuit
	if state == circuitClosed && failures >= b.failureThreshold {
		if b.state.CompareAndSwap(circuitClosed, circuitOpen) {
			b.metrics.CircuitOpens.Add(1)
		}
	} else if state == circuitHalfOpen && failures >= b.halfOpenLimit {
		// Failed in half-open state, go back to open
		b.state.Store(circuitOpen)
		b.metrics.CircuitOpens.Add(1)
	}
}

func (b *BackpressureController) recordSuccess() {
	state := b.state.Load()
	
	if state == circuitHalfOpen {
		// Reset failures on success in half-open state
		// This gradually allows more traffic through
		b.failures.Store(0)
		
		// Consider closing the circuit after enough successes
		// For simplicity, we close immediately on first success
		b.state.CompareAndSwap(circuitHalfOpen, circuitClosed)
	}
}

func (b *BackpressureController) recordWaitTime(duration time.Duration) {
	b.metrics.mu.Lock()
	defer b.metrics.mu.Unlock()
	
	b.metrics.waitTimes = append(b.metrics.waitTimes, duration)
	
	// Keep only last 100 wait times for average calculation
	if len(b.metrics.waitTimes) > 100 {
		b.metrics.waitTimes = b.metrics.waitTimes[len(b.metrics.waitTimes)-100:]
	}
	
	// Calculate average
	var total time.Duration
	for _, d := range b.metrics.waitTimes {
		total += d
	}
	
	if len(b.metrics.waitTimes) > 0 {
		avg := total / time.Duration(len(b.metrics.waitTimes))
		b.metrics.AverageWaitTime.Store(avg.Nanoseconds())
	}
}

func (b *BackpressureController) getStateName() string {
	switch b.state.Load() {
	case circuitClosed:
		return "closed"
	case circuitOpen:
		return "open"
	case circuitHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// Errors

var (
	ErrCircuitOpen = &BackpressureError{Code: "BP001", Message: "Circuit breaker is open"}
	ErrQueueFull   = &BackpressureError{Code: "BP002", Message: "Queue is full"}
)

// BackpressureError represents a backpressure-specific error
type BackpressureError struct {
	Code    string
	Message string
}

func (e *BackpressureError) Error() string {
	return e.Code + ": " + e.Message
}