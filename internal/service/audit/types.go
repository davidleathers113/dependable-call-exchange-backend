package audit

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/audit"
)

// LoggerStats provides statistics about the audit logger performance
type LoggerStats struct {
	TotalEvents        int64        `json:"total_events"`
	DroppedEvents      int64        `json:"dropped_events"`
	BufferSize         int          `json:"buffer_size"`
	BufferCapacity     int          `json:"buffer_capacity"`
	WorkersActive      int          `json:"workers_active"`
	BatchWorkersActive int          `json:"batch_workers_active"`
	LastFlush          time.Time    `json:"last_flush"`
	CircuitState       CircuitState `json:"circuit_state"`
	IsRunning          bool         `json:"is_running"`
}

// CircuitState represents the state of the circuit breaker
type CircuitState string

const (
	CircuitStateClosed   CircuitState = "closed"
	CircuitStateOpen     CircuitState = "open"
	CircuitStateHalfOpen CircuitState = "half_open"
)

// CircuitBreaker implements circuit breaker pattern for graceful degradation
type CircuitBreaker struct {
	threshold int
	timeout   time.Duration
	logger    *zap.Logger

	mu          sync.RWMutex
	state       CircuitState
	failures    int32
	lastFailure time.Time
	nextRetry   time.Time
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(threshold int, timeout time.Duration, logger *zap.Logger) *CircuitBreaker {
	return &CircuitBreaker{
		threshold: threshold,
		timeout:   timeout,
		logger:    logger,
		state:     CircuitStateClosed,
	}
}

// Execute executes a function with circuit breaker protection
func (cb *CircuitBreaker) Execute(fn func() error) error {
	cb.mu.RLock()
	state := cb.state
	nextRetry := cb.nextRetry
	cb.mu.RUnlock()

	// Check if circuit is open and retry time hasn't passed
	if state == CircuitStateOpen && time.Now().Before(nextRetry) {
		return &CircuitBreakerError{State: state}
	}

	// Execute function
	err := fn()

	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		cb.failures++
		cb.lastFailure = time.Now()

		// Open circuit if threshold exceeded
		if cb.failures >= int32(cb.threshold) && cb.state == CircuitStateClosed {
			cb.state = CircuitStateOpen
			cb.nextRetry = time.Now().Add(cb.timeout)
			cb.logger.Warn("Circuit breaker opened",
				zap.Int("failures", int(cb.failures)),
				zap.Int("threshold", cb.threshold),
			)
		}

		return err
	}

	// Reset on success
	if cb.state == CircuitStateOpen || cb.state == CircuitStateHalfOpen {
		cb.state = CircuitStateClosed
		cb.failures = 0
		cb.logger.Info("Circuit breaker closed", zap.Int("previous_failures", int(cb.failures)))
	}

	return nil
}

// GetState returns the current circuit breaker state
func (cb *CircuitBreaker) GetState() CircuitState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// CircuitBreakerError represents a circuit breaker error
type CircuitBreakerError struct {
	State CircuitState
}

func (e *CircuitBreakerError) Error() string {
	return "circuit breaker " + string(e.State)
}

// worker represents an async event processing worker
type worker struct {
	id     int
	logger *Logger
}

func (w *worker) run() {
	defer w.logger.wg.Done()

	w.logger.logger.Info("Audit worker started", zap.Int("worker_id", w.id))

	for {
		select {
		case event := <-w.logger.eventBuffer:
			w.processEvent(event)

		case <-w.logger.ctx.Done():
			w.logger.logger.Info("Audit worker shutting down", zap.Int("worker_id", w.id))
			return
		}
	}
}

func (w *worker) processEvent(event *audit.Event) {
	ctx, cancel := context.WithTimeout(w.logger.ctx, w.logger.config.WriteTimeout)
	defer cancel()

	if err := w.logger.circuitBreaker.Execute(func() error {
		return w.logger.processBatch(ctx, []*audit.Event{event})
	}); err != nil {
		w.logger.logger.Error("Failed to process event",
			zap.Error(err),
			zap.String("event_id", event.ID.String()),
			zap.Int("worker_id", w.id),
		)
		w.logger.recordEventError("processing")
	}
}

// batchWorker represents a batch processing worker
type batchWorker struct {
	id     int
	logger *Logger
}

func (bw *batchWorker) run() {
	defer bw.logger.wg.Done()

	bw.logger.logger.Info("Batch worker started", zap.Int("worker_id", bw.id))

	for {
		select {
		case batch := <-bw.logger.batchBuffer:
			bw.processBatch(batch)

		case <-bw.logger.ctx.Done():
			bw.logger.logger.Info("Batch worker shutting down", zap.Int("worker_id", bw.id))
			return
		}
	}
}

func (bw *batchWorker) processBatch(events []*audit.Event) {
	ctx, cancel := context.WithTimeout(bw.logger.ctx, bw.logger.config.WriteTimeout)
	defer cancel()

	if err := bw.logger.circuitBreaker.Execute(func() error {
		return bw.logger.processBatch(ctx, events)
	}); err != nil {
		bw.logger.logger.Error("Failed to process batch",
			zap.Error(err),
			zap.Int("batch_size", len(events)),
			zap.Int("worker_id", bw.id),
		)
		bw.logger.recordBatchError("processing")
	}
}

// LoggerMetrics tracks audit logger performance metrics
type LoggerMetrics struct {
	// Counters
	eventsProcessed  metric.Int64Counter
	eventsDropped    metric.Int64Counter
	eventsErrors     metric.Int64Counter
	batchesProcessed metric.Int64Counter
	batchErrors      metric.Int64Counter

	// Histograms
	eventLatency metric.Float64Histogram
	batchSize    metric.Int64Histogram
	batchLatency metric.Float64Histogram

	// Gauges
	bufferSize    metric.Int64ObservableGauge
	activeWorkers metric.Int64ObservableGauge
	circuitState  metric.Int64ObservableGauge

	// Internal tracking
	logger *Logger
}

func (l *Logger) initMetrics() error {
	meter := otel.Meter("audit.logger")
	l.metrics = &LoggerMetrics{logger: l}

	// Create counters
	eventsProcessed, err := meter.Int64Counter("audit.events.processed",
		metric.WithDescription("Total number of audit events processed"))
	if err != nil {
		return err
	}
	l.metrics.eventsProcessed = eventsProcessed

	eventsDropped, err := meter.Int64Counter("audit.events.dropped",
		metric.WithDescription("Total number of audit events dropped"))
	if err != nil {
		return err
	}
	l.metrics.eventsDropped = eventsDropped

	eventsErrors, err := meter.Int64Counter("audit.events.errors",
		metric.WithDescription("Total number of audit event processing errors"))
	if err != nil {
		return err
	}
	l.metrics.eventsErrors = eventsErrors

	batchesProcessed, err := meter.Int64Counter("audit.batches.processed",
		metric.WithDescription("Total number of audit batches processed"))
	if err != nil {
		return err
	}
	l.metrics.batchesProcessed = batchesProcessed

	batchErrors, err := meter.Int64Counter("audit.batches.errors",
		metric.WithDescription("Total number of audit batch processing errors"))
	if err != nil {
		return err
	}
	l.metrics.batchErrors = batchErrors

	// Create histograms
	eventLatency, err := meter.Float64Histogram("audit.event.latency",
		metric.WithDescription("Latency of audit event processing"),
		metric.WithUnit("ms"))
	if err != nil {
		return err
	}
	l.metrics.eventLatency = eventLatency

	batchSize, err := meter.Int64Histogram("audit.batch.size",
		metric.WithDescription("Size of audit event batches"))
	if err != nil {
		return err
	}
	l.metrics.batchSize = batchSize

	batchLatency, err := meter.Float64Histogram("audit.batch.latency",
		metric.WithDescription("Latency of audit batch processing"),
		metric.WithUnit("ms"))
	if err != nil {
		return err
	}
	l.metrics.batchLatency = batchLatency

	// Create observable gauges
	bufferSize, err := meter.Int64ObservableGauge("audit.buffer.size",
		metric.WithDescription("Current size of the audit event buffer"))
	if err != nil {
		return err
	}
	l.metrics.bufferSize = bufferSize

	activeWorkers, err := meter.Int64ObservableGauge("audit.workers.active",
		metric.WithDescription("Number of active audit workers"))
	if err != nil {
		return err
	}
	l.metrics.activeWorkers = activeWorkers

	circuitState, err := meter.Int64ObservableGauge("audit.circuit.state",
		metric.WithDescription("Circuit breaker state (0=closed, 1=open, 2=half-open)"))
	if err != nil {
		return err
	}
	l.metrics.circuitState = circuitState

	// Register callback for observable gauges
	meter.RegisterCallback(func(ctx context.Context, observer metric.Observer) error {
		observer.ObserveInt64(bufferSize, int64(len(l.eventBuffer)))
		observer.ObserveInt64(activeWorkers, int64(len(l.workers)+len(l.batchWorkers)))

		state := l.circuitBreaker.GetState()
		var stateValue int64
		switch state {
		case CircuitStateClosed:
			stateValue = 0
		case CircuitStateOpen:
			stateValue = 1
		case CircuitStateHalfOpen:
			stateValue = 2
		}
		observer.ObserveInt64(circuitState, stateValue)

		return nil
	}, bufferSize, activeWorkers, circuitState)

	return nil
}

// Metric recording methods
func (l *Logger) recordEventProcessed(reason string) {
	l.metrics.eventsProcessed.Add(context.Background(), 1,
		metric.WithAttributes(attribute.String("reason", reason)))
}

func (l *Logger) recordEventDropped(reason string) {
	l.metrics.eventsDropped.Add(context.Background(), 1,
		metric.WithAttributes(attribute.String("reason", reason)))
}

func (l *Logger) recordEventError(reason string) {
	l.metrics.eventsErrors.Add(context.Background(), 1,
		metric.WithAttributes(attribute.String("reason", reason)))
}

func (l *Logger) recordEventLatency(latency time.Duration) {
	l.metrics.eventLatency.Record(context.Background(), float64(latency.Microseconds())/1000)
}

func (l *Logger) recordBatchProcessed(size int, latency time.Duration) {
	l.metrics.batchesProcessed.Add(context.Background(), 1)
	l.metrics.batchSize.Record(context.Background(), int64(size))
	l.metrics.batchLatency.Record(context.Background(), float64(latency.Milliseconds()))
}

func (l *Logger) recordBatchError(reason string) {
	l.metrics.batchErrors.Add(context.Background(), 1,
		metric.WithAttributes(attribute.String("reason", reason)))
}

// DefaultEventEnricher provides basic event enrichment from HTTP requests
type DefaultEventEnricher struct {
	logger *zap.Logger
}

// NewDefaultEventEnricher creates a new default event enricher
func NewDefaultEventEnricher(logger *zap.Logger) *DefaultEventEnricher {
	return &DefaultEventEnricher{logger: logger}
}

// Enrich enriches an audit event with HTTP request context
func (e *DefaultEventEnricher) Enrich(ctx context.Context, event *audit.Event, request *http.Request) error {
	if request == nil {
		return nil
	}

	// Extract IP address
	if ip := extractClientIP(request); ip != "" {
		event.ActorIP = ip
	}

	// Extract User-Agent
	if userAgent := request.Header.Get("User-Agent"); userAgent != "" {
		event.ActorAgent = userAgent
	}

	// Extract session information
	if sessionID := extractSessionID(request); sessionID != "" {
		event.SessionID = sessionID
	}

	// Extract correlation ID
	if correlationID := extractCorrelationID(request); correlationID != "" {
		event.CorrelationID = correlationID
	}

	// Add HTTP-specific metadata
	if event.Metadata == nil {
		event.Metadata = make(map[string]interface{})
	}

	event.Metadata["http_method"] = request.Method
	event.Metadata["http_path"] = request.URL.Path
	if request.Referer() != "" {
		event.Metadata["http_referer"] = request.Referer()
	}

	return nil
}

// Helper functions for request enrichment
func extractClientIP(request *http.Request) string {
	// Check X-Forwarded-For header first
	if xff := request.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP in the chain
		if ips := strings.Split(xff, ","); len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// Check X-Real-IP header
	if xri := request.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Use remote address as fallback
	if host, _, err := net.SplitHostPort(request.RemoteAddr); err == nil {
		return host
	}

	return request.RemoteAddr
}

func extractSessionID(request *http.Request) string {
	// Try session cookie first
	if cookie, err := request.Cookie("session_id"); err == nil {
		return cookie.Value
	}

	// Try authorization header
	if auth := request.Header.Get("Authorization"); auth != "" {
		// Extract from Bearer token (simplified)
		if strings.HasPrefix(auth, "Bearer ") {
			return auth[7:] // Remove "Bearer " prefix
		}
	}

	return ""
}

func extractCorrelationID(request *http.Request) string {
	// Check common correlation ID headers
	headers := []string{
		"X-Correlation-ID",
		"X-Request-ID",
		"X-Trace-ID",
		"Request-ID",
	}

	for _, header := range headers {
		if value := request.Header.Get(header); value != "" {
			return value
		}
	}

	return ""
}
