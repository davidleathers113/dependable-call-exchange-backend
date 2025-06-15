package audit

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/audit"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/infrastructure/cache"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/infrastructure/database"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/infrastructure/events"
)

// LoggerConfig configures the audit logging service
type LoggerConfig struct {
	// Worker configuration
	WorkerPoolSize int // Number of async workers (default: 10)
	BatchWorkers   int // Number of batch processing workers (default: 5)

	// Batch processing
	BatchSize    int           // Maximum batch size (default: 100)
	BatchTimeout time.Duration // Batch flush timeout (default: 1s)

	// Performance tuning
	BufferSize   int           // Event buffer size (default: 10000)
	WriteTimeout time.Duration // Database write timeout (default: 5s)

	// Hash chain
	HashChainEnabled bool   // Enable hash chain validation (default: true)
	HashSecretKey    []byte // Secret key for hash chain (required)

	// Circuit breaker
	FailureThreshold int           // Circuit breaker threshold (default: 5)
	CircuitTimeout   time.Duration // Circuit breaker reset timeout (default: 30s)

	// Event enrichment
	EnrichmentEnabled bool // Enable event enrichment (default: true)
	IPGeoEnabled      bool // Enable IP geolocation (default: true)
	UserAgentParsing  bool // Enable user agent parsing (default: true)

	// Graceful degradation
	GracefulDegradation bool   // Enable graceful degradation (default: true)
	MaxMemoryUsage      int64  // Max memory usage in bytes (default: 100MB)
	DropPolicy          string // Drop policy: "oldest", "newest", "random" (default: "oldest")
}

// DefaultLoggerConfig returns default configuration
func DefaultLoggerConfig() LoggerConfig {
	return LoggerConfig{
		WorkerPoolSize:      10,
		BatchWorkers:        5,
		BatchSize:           100,
		BatchTimeout:        1 * time.Second,
		BufferSize:          10000,
		WriteTimeout:        5 * time.Second,
		HashChainEnabled:    true,
		FailureThreshold:    5,
		CircuitTimeout:      30 * time.Second,
		EnrichmentEnabled:   true,
		IPGeoEnabled:        true,
		UserAgentParsing:    true,
		GracefulDegradation: true,
		MaxMemoryUsage:      100 * 1024 * 1024, // 100MB
		DropPolicy:          "oldest",
	}
}

// Logger provides high-performance async audit logging with hash chain validation
type Logger struct {
	config LoggerConfig
	logger *zap.Logger

	// Dependencies (max 5 per DCE patterns)
	repository    AuditRepository
	cache         AuditCache
	publisher     AuditPublisher
	domainService DomainService
	enricher      EventEnricher

	// Async processing
	eventBuffer  chan *audit.Event
	batchBuffer  chan []*audit.Event
	workers      []*worker
	batchWorkers []*batchWorker

	// Circuit breaker
	circuitBreaker *CircuitBreaker

	// Metrics and monitoring
	metrics *LoggerMetrics
	tracer  trace.Tracer

	// Lifecycle management
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	running int32

	// Performance monitoring
	lastFlush     time.Time
	totalEvents   int64
	droppedEvents int64

	// Hash chain state
	chainMutex   sync.RWMutex
	lastHash     string
	lastSequence int64
}

// Dependencies interfaces following DCE patterns
type AuditRepository interface {
	Store(ctx context.Context, event *audit.Event) error
	StoreBatch(ctx context.Context, events []*audit.Event) error
	GetLatestSequenceNumber(ctx context.Context) (values.SequenceNumber, error)
}

type AuditCache interface {
	SetLatestHash(ctx context.Context, hash string, sequenceNum int64) error
	GetLatestHash(ctx context.Context) (string, int64, error)
	SetEvents(ctx context.Context, events []*audit.Event) error
}

type AuditPublisher interface {
	Publish(ctx context.Context, event *audit.Event) error
}

type DomainService interface {
	ValidateEvent(event *audit.Event) error
	ComputeHash(event *audit.Event, previousHash string) (string, error)
}

type EventEnricher interface {
	Enrich(ctx context.Context, event *audit.Event, request *http.Request) error
}

// NewLogger creates a new high-performance audit logger
func NewLogger(
	ctx context.Context,
	config LoggerConfig,
	logger *zap.Logger,
	repository AuditRepository,
	cache AuditCache,
	publisher AuditPublisher,
	domainService DomainService,
	enricher EventEnricher,
) (*Logger, error) {
	// Validate dependencies
	if repository == nil {
		return nil, errors.NewValidationError("MISSING_REPOSITORY", "audit repository is required")
	}
	if cache == nil {
		return nil, errors.NewValidationError("MISSING_CACHE", "audit cache is required")
	}
	if domainService == nil {
		return nil, errors.NewValidationError("MISSING_DOMAIN_SERVICE", "domain service is required")
	}

	// Validate hash chain configuration
	if config.HashChainEnabled && len(config.HashSecretKey) < 32 {
		return nil, errors.NewValidationError("WEAK_HASH_KEY",
			"hash secret key must be at least 32 bytes")
	}

	ctx, cancel := context.WithCancel(ctx)

	auditLogger := &Logger{
		config:        config,
		logger:        logger,
		repository:    repository,
		cache:         cache,
		publisher:     publisher,
		domainService: domainService,
		enricher:      enricher,
		eventBuffer:   make(chan *audit.Event, config.BufferSize),
		batchBuffer:   make(chan []*audit.Event, config.BatchWorkers*2),
		ctx:           ctx,
		cancel:        cancel,
		tracer:        otel.Tracer("audit.logger"),
		lastFlush:     time.Now(),
	}

	// Initialize circuit breaker
	auditLogger.circuitBreaker = NewCircuitBreaker(
		config.FailureThreshold,
		config.CircuitTimeout,
		logger,
	)

	// Initialize metrics
	if err := auditLogger.initMetrics(); err != nil {
		return nil, fmt.Errorf("failed to initialize metrics: %w", err)
	}

	// Initialize hash chain state
	if err := auditLogger.initHashChain(ctx); err != nil {
		logger.Warn("Failed to initialize hash chain, starting fresh", zap.Error(err))
	}

	// Start workers
	auditLogger.startWorkers()

	logger.Info("Audit logger initialized",
		zap.Int("workers", config.WorkerPoolSize),
		zap.Int("batch_workers", config.BatchWorkers),
		zap.Int("buffer_size", config.BufferSize),
		zap.Bool("hash_chain_enabled", config.HashChainEnabled),
		zap.Bool("enrichment_enabled", config.EnrichmentEnabled),
	)

	return auditLogger, nil
}

// LogEvent logs an audit event asynchronously with < 5ms latency
// Performance target: < 5ms write latency with graceful degradation
func (l *Logger) LogEvent(ctx context.Context, eventType audit.EventType,
	actorID, targetID, action, result string, metadata map[string]interface{}) error {

	ctx, span := l.tracer.Start(ctx, "Logger.LogEvent",
		trace.WithAttributes(
			attribute.String("event.type", string(eventType)),
			attribute.String("actor.id", actorID),
			attribute.String("action", action),
		),
	)
	defer span.End()

	start := time.Now()

	// Check if logger is running
	if atomic.LoadInt32(&l.running) == 0 {
		span.RecordError(errors.NewInternalError("logger not running"))
		return errors.NewInternalError("audit logger not running")
	}

	// Create audit event
	event := &audit.Event{
		ID:            uuid.New(),
		Type:          eventType,
		Severity:      l.determineSeverity(eventType, result),
		ActorID:       actorID,
		TargetID:      targetID,
		Action:        action,
		Result:        result,
		Metadata:      metadata,
		Timestamp:     time.Now().UTC(),
		TimestampNano: time.Now().UTC().UnixNano(),
	}

	// Enrich event from context
	if l.config.EnrichmentEnabled && l.enricher != nil {
		if request := extractHTTPRequest(ctx); request != nil {
			if err := l.enricher.Enrich(ctx, event, request); err != nil {
				l.logger.Warn("Failed to enrich event", zap.Error(err))
				// Continue without enrichment - graceful degradation
			}
		}
	}

	// Validate event
	if err := l.domainService.ValidateEvent(event); err != nil {
		span.RecordError(err)
		l.recordEventError("validation")
		return errors.NewValidationError("INVALID_EVENT", "event validation failed").WithCause(err)
	}

	// Try to send to buffer (non-blocking for performance)
	select {
	case l.eventBuffer <- event:
		// Success - record metrics
		latency := time.Since(start)
		l.recordEventLatency(latency)
		l.recordEventProcessed("buffered")

		// Target: < 5ms latency
		if latency > 5*time.Millisecond {
			l.logger.Warn("Event logging latency exceeded target",
				zap.Duration("latency", latency),
				zap.String("event_id", event.ID.String()),
			)
		}

		span.SetAttributes(attribute.Bool("buffered", true))
		return nil

	default:
		// Buffer full - apply drop policy for graceful degradation
		if l.config.GracefulDegradation {
			dropped := atomic.AddInt64(&l.droppedEvents, 1)
			l.recordEventDropped("buffer_full")

			l.logger.Warn("Event buffer full, event dropped",
				zap.String("event_id", event.ID.String()),
				zap.Int64("total_dropped", dropped),
			)

			span.SetAttributes(attribute.Bool("dropped", true))
			return nil // Graceful degradation - don't fail the operation
		}

		// No graceful degradation - return error
		span.RecordError(errors.NewInternalError("event buffer full"))
		return errors.NewInternalError("audit event buffer full")
	}
}

// LogEventWithRequest logs an audit event with HTTP request context for enrichment
func (l *Logger) LogEventWithRequest(ctx context.Context, request *http.Request,
	eventType audit.EventType, actorID, targetID, action, result string,
	metadata map[string]interface{}) error {

	// Store request in context for enrichment
	ctx = withHTTPRequest(ctx, request)
	return l.LogEvent(ctx, eventType, actorID, targetID, action, result, metadata)
}

// FlushEvents forces immediate processing of buffered events
func (l *Logger) FlushEvents(ctx context.Context) error {
	ctx, span := l.tracer.Start(ctx, "Logger.FlushEvents")
	defer span.End()

	// Signal workers to flush immediately
	flushSignal := make(chan struct{})
	close(flushSignal)

	// Wait for flush to complete with timeout
	done := make(chan struct{})
	go func() {
		defer close(done)
		// Process remaining events in buffer
		timeout := time.After(l.config.WriteTimeout)
		for {
			select {
			case <-l.eventBuffer:
				// Event processed
			case <-timeout:
				return
			case <-done:
				return
			}
		}
	}()

	select {
	case <-done:
		l.lastFlush = time.Now()
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(l.config.WriteTimeout):
		return errors.NewInternalError("flush timeout exceeded")
	}
}

// GetStats returns current logger statistics
func (l *Logger) GetStats() LoggerStats {
	return LoggerStats{
		TotalEvents:        atomic.LoadInt64(&l.totalEvents),
		DroppedEvents:      atomic.LoadInt64(&l.droppedEvents),
		BufferSize:         len(l.eventBuffer),
		BufferCapacity:     cap(l.eventBuffer),
		WorkersActive:      len(l.workers),
		BatchWorkersActive: len(l.batchWorkers),
		LastFlush:          l.lastFlush,
		CircuitState:       l.circuitBreaker.GetState(),
		IsRunning:          atomic.LoadInt32(&l.running) == 1,
	}
}

// Health checks the health of the audit logger
func (l *Logger) Health() error {
	// Check if running
	if atomic.LoadInt32(&l.running) == 0 {
		return errors.NewInternalError("audit logger not running")
	}

	// Check circuit breaker
	if l.circuitBreaker.GetState() == CircuitStateOpen {
		return errors.NewInternalError("audit circuit breaker open")
	}

	// Check buffer capacity
	bufferUsage := float64(len(l.eventBuffer)) / float64(cap(l.eventBuffer))
	if bufferUsage > 0.9 {
		return errors.NewInternalError(fmt.Sprintf("event buffer nearly full: %.1f%%", bufferUsage*100))
	}

	// Check memory usage
	if l.config.GracefulDegradation {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		if int64(m.Alloc) > l.config.MaxMemoryUsage {
			return errors.NewInternalError(fmt.Sprintf("memory usage exceeded: %d bytes", m.Alloc))
		}
	}

	return nil
}

// Close gracefully shuts down the audit logger
func (l *Logger) Close() error {
	l.logger.Info("Shutting down audit logger")

	// Signal shutdown
	atomic.StoreInt32(&l.running, 0)
	l.cancel()

	// Wait for workers to finish with timeout
	done := make(chan struct{})
	go func() {
		l.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		l.logger.Info("Audit logger shut down gracefully")
	case <-time.After(l.config.WriteTimeout * 2):
		l.logger.Warn("Audit logger shutdown timeout, some events may be lost",
			zap.Int("pending_events", len(l.eventBuffer)),
		)
	}

	return nil
}

// Private methods

func (l *Logger) startWorkers() {
	atomic.StoreInt32(&l.running, 1)

	// Start event workers
	l.workers = make([]*worker, l.config.WorkerPoolSize)
	for i := 0; i < l.config.WorkerPoolSize; i++ {
		worker := &worker{
			id:     i,
			logger: l,
		}
		l.workers[i] = worker

		l.wg.Add(1)
		go worker.run()
	}

	// Start batch workers
	l.batchWorkers = make([]*batchWorker, l.config.BatchWorkers)
	for i := 0; i < l.config.BatchWorkers; i++ {
		batchWorker := &batchWorker{
			id:     i,
			logger: l,
		}
		l.batchWorkers[i] = batchWorker

		l.wg.Add(1)
		go batchWorker.run()
	}

	// Start batch coordinator
	l.wg.Add(1)
	go l.batchCoordinator()
}

func (l *Logger) batchCoordinator() {
	defer l.wg.Done()

	ticker := time.NewTicker(l.config.BatchTimeout)
	defer ticker.Stop()

	batch := make([]*audit.Event, 0, l.config.BatchSize)

	for {
		select {
		case event := <-l.eventBuffer:
			batch = append(batch, event)

			if len(batch) >= l.config.BatchSize {
				l.sendBatch(batch)
				batch = make([]*audit.Event, 0, l.config.BatchSize)
			}

		case <-ticker.C:
			if len(batch) > 0 {
				l.sendBatch(batch)
				batch = make([]*audit.Event, 0, l.config.BatchSize)
			}

		case <-l.ctx.Done():
			// Process remaining batch
			if len(batch) > 0 {
				l.sendBatch(batch)
			}
			return
		}
	}
}

func (l *Logger) sendBatch(events []*audit.Event) {
	if len(events) == 0 {
		return
	}

	// Copy batch to avoid race conditions
	batchCopy := make([]*audit.Event, len(events))
	copy(batchCopy, events)

	select {
	case l.batchBuffer <- batchCopy:
		// Batch queued successfully
	default:
		// Batch buffer full - process synchronously as fallback
		l.processBatchDirect(batchCopy)
	}
}

func (l *Logger) processBatchDirect(events []*audit.Event) {
	ctx, cancel := context.WithTimeout(l.ctx, l.config.WriteTimeout)
	defer cancel()

	if err := l.circuitBreaker.Execute(func() error {
		return l.processBatch(ctx, events)
	}); err != nil {
		l.logger.Error("Failed to process batch directly",
			zap.Error(err),
			zap.Int("batch_size", len(events)),
		)
		l.recordBatchError("direct_processing")
	}
}

func (l *Logger) processBatch(ctx context.Context, events []*audit.Event) error {
	ctx, span := l.tracer.Start(ctx, "Logger.processBatch",
		trace.WithAttributes(
			attribute.Int("batch.size", len(events)),
		),
	)
	defer span.End()

	start := time.Now()

	// Compute hash chain for batch
	if l.config.HashChainEnabled {
		if err := l.computeBatchHashChain(events); err != nil {
			span.RecordError(err)
			return fmt.Errorf("failed to compute hash chain: %w", err)
		}
	}

	// Store batch in database
	if err := l.repository.StoreBatch(ctx, events); err != nil {
		span.RecordError(err)
		l.recordBatchError("database")
		return fmt.Errorf("failed to store batch: %w", err)
	}

	// Cache events for fast retrieval
	if l.cache != nil {
		if err := l.cache.SetEvents(ctx, events); err != nil {
			l.logger.Warn("Failed to cache events", zap.Error(err))
			// Continue - cache failure is not critical
		}
	}

	// Publish events for real-time notifications
	if l.publisher != nil {
		for _, event := range events {
			if err := l.publisher.Publish(ctx, event); err != nil {
				l.logger.Warn("Failed to publish event",
					zap.Error(err),
					zap.String("event_id", event.ID.String()),
				)
				// Continue - publishing failure is not critical
			}
		}
	}

	// Update metrics
	atomic.AddInt64(&l.totalEvents, int64(len(events)))
	l.recordBatchProcessed(len(events), time.Since(start))

	span.SetAttributes(
		attribute.Int64("events.processed", int64(len(events))),
		attribute.Int64("processing.duration_ms", time.Since(start).Milliseconds()),
	)

	return nil
}

func (l *Logger) computeBatchHashChain(events []*audit.Event) error {
	l.chainMutex.Lock()
	defer l.chainMutex.Unlock()

	previousHash := l.lastHash

	for _, event := range events {
		// Assign sequence number
		l.lastSequence++
		event.SequenceNum = l.lastSequence

		// Compute hash with previous hash
		hash, err := l.domainService.ComputeHash(event, previousHash)
		if err != nil {
			return fmt.Errorf("failed to compute hash for event %s: %w", event.ID, err)
		}

		event.EventHash = hash
		event.PreviousHash = previousHash
		previousHash = hash
	}

	// Update chain state
	if len(events) > 0 {
		l.lastHash = previousHash

		// Update cache with latest hash
		if l.cache != nil {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			l.cache.SetLatestHash(ctx, l.lastHash, l.lastSequence)
		}
	}

	return nil
}

func (l *Logger) initHashChain(ctx context.Context) error {
	// Try to load from cache first
	if l.cache != nil {
		hash, seq, err := l.cache.GetLatestHash(ctx)
		if err == nil && hash != "" {
			l.lastHash = hash
			l.lastSequence = seq
			l.logger.Info("Hash chain state loaded from cache",
				zap.String("hash", hash[:8]+"..."),
				zap.Int64("sequence", seq),
			)
			return nil
		}
	}

	// Load from database
	if l.repository != nil {
		latestSeq, err := l.repository.GetLatestSequenceNumber(ctx)
		if err == nil {
			l.lastSequence = int64(latestSeq.Value())
			l.logger.Info("Hash chain sequence loaded from database",
				zap.Int64("sequence", l.lastSequence),
			)
		}
	}

	return nil
}

func (l *Logger) determineSeverity(eventType audit.EventType, result string) audit.Severity {
	// Determine severity based on event type and result
	switch eventType {
	case audit.EventTypeSystemFailure, audit.EventTypeSecurityIncident:
		return audit.SeverityCritical
	case audit.EventTypeComplianceViolation, audit.EventTypeDataAccess:
		if strings.Contains(strings.ToLower(result), "fail") ||
			strings.Contains(strings.ToLower(result), "error") {
			return audit.SeverityHigh
		}
		return audit.SeverityMedium
	case audit.EventTypeUserLogin, audit.EventTypeUserLogout:
		if strings.Contains(strings.ToLower(result), "fail") {
			return audit.SeverityMedium
		}
		return audit.SeverityLow
	default:
		return audit.SeverityLow
	}
}

// Context helpers for HTTP request enrichment
type contextKey string

const httpRequestKey contextKey = "http_request"

func withHTTPRequest(ctx context.Context, request *http.Request) context.Context {
	return context.WithValue(ctx, httpRequestKey, request)
}

func extractHTTPRequest(ctx context.Context) *http.Request {
	if request, ok := ctx.Value(httpRequestKey).(*http.Request); ok {
		return request
	}
	return nil
}
