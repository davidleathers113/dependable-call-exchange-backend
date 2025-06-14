package audit

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// EventPublisher defines the interface for publishing domain events to audit logging
type EventPublisher interface {
	// Publish single domain event
	Publish(ctx context.Context, event DomainEvent) error
	
	// PublishBatch publishes multiple events efficiently
	PublishBatch(ctx context.Context, events []DomainEvent) error
	
	// PublishAsync publishes event asynchronously (fire-and-forget)
	PublishAsync(ctx context.Context, event DomainEvent)
	
	// Subscribe to published events (for testing and monitoring)
	Subscribe(handler EventHandler) Subscription
	
	// Health check for publisher
	Health() error
	
	// Close gracefully shuts down the publisher
	Close() error
}

// EventHandler processes published domain events
type EventHandler interface {
	Handle(ctx context.Context, event DomainEvent) error
}

// EventHandlerFunc adapter for functions
type EventHandlerFunc func(ctx context.Context, event DomainEvent) error

func (f EventHandlerFunc) Handle(ctx context.Context, event DomainEvent) error {
	return f(ctx, event)
}

// Subscription represents an event subscription
type Subscription interface {
	Unsubscribe() error
	ID() string
}

// AuditEventPublisher is the main implementation of EventPublisher
// Routes domain events to audit logger with error handling and retries
type AuditEventPublisher struct {
	logger       *zap.Logger
	auditLogger  AuditLogger
	subscribers  map[string]EventHandler
	subscriberMu sync.RWMutex
	
	// Configuration
	config PublisherConfig
	
	// Async processing
	eventQueue   chan DomainEvent
	batchQueue   chan []DomainEvent
	workerPool   chan struct{}
	shutdownCh   chan struct{}
	wg           sync.WaitGroup
	
	// Metrics
	metrics PublisherMetrics
}

// AuditLogger interface for the actual audit logging implementation
type AuditLogger interface {
	Log(ctx context.Context, event *Event) error
	LogBatch(ctx context.Context, events []*Event) error
	Health() error
}

// PublisherConfig configures the event publisher
type PublisherConfig struct {
	// Async processing
	WorkerCount    int           `json:"worker_count"`
	QueueSize      int           `json:"queue_size"`
	BatchSize      int           `json:"batch_size"`
	BatchTimeout   time.Duration `json:"batch_timeout"`
	
	// Retry configuration
	MaxRetries     int           `json:"max_retries"`
	RetryDelay     time.Duration `json:"retry_delay"`
	BackoffFactor  float64       `json:"backoff_factor"`
	
	// Circuit breaker
	FailureThreshold int           `json:"failure_threshold"`
	ResetTimeout     time.Duration `json:"reset_timeout"`
	
	// Timeouts
	PublishTimeout time.Duration `json:"publish_timeout"`
	ShutdownTimeout time.Duration `json:"shutdown_timeout"`
}

// DefaultPublisherConfig returns sensible defaults
func DefaultPublisherConfig() PublisherConfig {
	return PublisherConfig{
		WorkerCount:      5,
		QueueSize:        1000,
		BatchSize:        10,
		BatchTimeout:     100 * time.Millisecond,
		MaxRetries:       3,
		RetryDelay:       100 * time.Millisecond,
		BackoffFactor:    2.0,
		FailureThreshold: 5,
		ResetTimeout:     30 * time.Second,
		PublishTimeout:   5 * time.Second,
		ShutdownTimeout:  10 * time.Second,
	}
}

// PublisherMetrics tracks publisher performance
type PublisherMetrics struct {
	EventsPublished   int64
	EventsFailed      int64
	BatchesPublished  int64
	BatchesFailed     int64
	RetryAttempts     int64
	QueueSize         int64
	ProcessingLatency time.Duration
	
	mu sync.RWMutex
}

// NewAuditEventPublisher creates a new audit event publisher
func NewAuditEventPublisher(logger *zap.Logger, auditLogger AuditLogger, config PublisherConfig) *AuditEventPublisher {
	publisher := &AuditEventPublisher{
		logger:      logger,
		auditLogger: auditLogger,
		subscribers: make(map[string]EventHandler),
		config:      config,
		eventQueue:  make(chan DomainEvent, config.QueueSize),
		batchQueue:  make(chan []DomainEvent, config.QueueSize/config.BatchSize),
		workerPool:  make(chan struct{}, config.WorkerCount),
		shutdownCh:  make(chan struct{}),
	}
	
	// Start worker pool
	publisher.startWorkers()
	
	return publisher
}

// Publish publishes a single domain event synchronously
func (p *AuditEventPublisher) Publish(ctx context.Context, event DomainEvent) error {
	start := time.Now()
	defer func() {
		p.metrics.mu.Lock()
		p.metrics.ProcessingLatency = time.Since(start)
		p.metrics.mu.Unlock()
	}()
	
	// Convert domain event to audit event
	auditEvent, err := event.ToAuditEvent()
	if err != nil {
		p.logger.Error("Failed to convert domain event to audit event",
			zap.Error(err),
			zap.String("event_type", string(event.GetEventType())),
			zap.String("event_id", event.GetEventID().String()))
		p.incrementFailureCount()
		return fmt.Errorf("failed to convert domain event: %w", err)
	}
	
	// Publish with timeout
	publishCtx, cancel := context.WithTimeout(ctx, p.config.PublishTimeout)
	defer cancel()
	
	err = p.publishWithRetry(publishCtx, auditEvent)
	if err != nil {
		p.incrementFailureCount()
		return err
	}
	
	// Notify subscribers
	p.notifySubscribers(ctx, event)
	
	p.incrementSuccessCount()
	return nil
}

// PublishBatch publishes multiple events efficiently
func (p *AuditEventPublisher) PublishBatch(ctx context.Context, events []DomainEvent) error {
	if len(events) == 0 {
		return nil
	}
	
	start := time.Now()
	defer func() {
		p.metrics.mu.Lock()
		p.metrics.ProcessingLatency = time.Since(start)
		p.metrics.mu.Unlock()
	}()
	
	// Convert all domain events to audit events
	auditEvents := make([]*Event, 0, len(events))
	for _, domainEvent := range events {
		auditEvent, err := domainEvent.ToAuditEvent()
		if err != nil {
			p.logger.Error("Failed to convert domain event in batch",
				zap.Error(err),
				zap.String("event_type", string(domainEvent.GetEventType())),
				zap.String("event_id", domainEvent.GetEventID().String()))
			continue // Skip failed conversions but continue with others
		}
		auditEvents = append(auditEvents, auditEvent)
	}
	
	if len(auditEvents) == 0 {
		p.incrementBatchFailureCount()
		return errors.New("no valid events to publish in batch")
	}
	
	// Publish batch with timeout
	publishCtx, cancel := context.WithTimeout(ctx, p.config.PublishTimeout)
	defer cancel()
	
	err := p.publishBatchWithRetry(publishCtx, auditEvents)
	if err != nil {
		p.incrementBatchFailureCount()
		return err
	}
	
	// Notify subscribers for each event
	for _, event := range events {
		p.notifySubscribers(ctx, event)
	}
	
	p.incrementBatchSuccessCount()
	return nil
}

// PublishAsync publishes event asynchronously
func (p *AuditEventPublisher) PublishAsync(ctx context.Context, event DomainEvent) {
	select {
	case p.eventQueue <- event:
		p.updateQueueSize()
	case <-ctx.Done():
		p.logger.Warn("Failed to queue async event due to context cancellation",
			zap.String("event_type", string(event.GetEventType())),
			zap.String("event_id", event.GetEventID().String()))
	default:
		p.logger.Error("Event queue full, dropping event",
			zap.String("event_type", string(event.GetEventType())),
			zap.String("event_id", event.GetEventID().String()),
			zap.Int("queue_size", len(p.eventQueue)))
		p.incrementFailureCount()
	}
}

// Subscribe adds an event handler
func (p *AuditEventPublisher) Subscribe(handler EventHandler) Subscription {
	id := uuid.New().String()
	
	p.subscriberMu.Lock()
	p.subscribers[id] = handler
	p.subscriberMu.Unlock()
	
	return &subscription{
		id:        id,
		publisher: p,
	}
}

// Health checks the publisher health
func (p *AuditEventPublisher) Health() error {
	// Check audit logger health
	if err := p.auditLogger.Health(); err != nil {
		return fmt.Errorf("audit logger unhealthy: %w", err)
	}
	
	// Check queue sizes
	p.metrics.mu.RLock()
	queueSize := p.metrics.QueueSize
	p.metrics.mu.RUnlock()
	
	if queueSize > int64(p.config.QueueSize)*9/10 { // 90% full
		return fmt.Errorf("event queue nearly full: %d/%d", queueSize, p.config.QueueSize)
	}
	
	return nil
}

// Close gracefully shuts down the publisher
func (p *AuditEventPublisher) Close() error {
	p.logger.Info("Shutting down audit event publisher")
	
	// Signal shutdown
	close(p.shutdownCh)
	
	// Wait for workers to finish with timeout
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		p.logger.Info("All workers shut down gracefully")
	case <-time.After(p.config.ShutdownTimeout):
		p.logger.Warn("Shutdown timeout reached, some events may be lost")
	}
	
	return nil
}

// Private methods

func (p *AuditEventPublisher) startWorkers() {
	// Start async event workers
	for i := 0; i < p.config.WorkerCount; i++ {
		p.wg.Add(1)
		go p.asyncEventWorker()
	}
	
	// Start batch processor
	p.wg.Add(1)
	go p.batchProcessor()
}

func (p *AuditEventPublisher) asyncEventWorker() {
	defer p.wg.Done()
	
	batch := make([]DomainEvent, 0, p.config.BatchSize)
	ticker := time.NewTicker(p.config.BatchTimeout)
	defer ticker.Stop()
	
	for {
		select {
		case event := <-p.eventQueue:
			batch = append(batch, event)
			
			// Send batch if full
			if len(batch) >= p.config.BatchSize {
				p.sendBatch(batch)
				batch = make([]DomainEvent, 0, p.config.BatchSize)
				ticker.Reset(p.config.BatchTimeout)
			}
			
		case <-ticker.C:
			// Send partial batch on timeout
			if len(batch) > 0 {
				p.sendBatch(batch)
				batch = make([]DomainEvent, 0, p.config.BatchSize)
			}
			
		case <-p.shutdownCh:
			// Process remaining events
			if len(batch) > 0 {
				p.sendBatch(batch)
			}
			return
		}
	}
}

func (p *AuditEventPublisher) batchProcessor() {
	defer p.wg.Done()
	
	for {
		select {
		case batch := <-p.batchQueue:
			ctx, cancel := context.WithTimeout(context.Background(), p.config.PublishTimeout)
			err := p.PublishBatch(ctx, batch)
			cancel()
			
			if err != nil {
				p.logger.Error("Failed to process event batch",
					zap.Error(err),
					zap.Int("batch_size", len(batch)))
			}
			
		case <-p.shutdownCh:
			return
		}
	}
}

func (p *AuditEventPublisher) sendBatch(batch []DomainEvent) {
	select {
	case p.batchQueue <- batch:
		// Successfully queued
	default:
		p.logger.Error("Batch queue full, dropping batch",
			zap.Int("batch_size", len(batch)))
		p.incrementBatchFailureCount()
	}
}

func (p *AuditEventPublisher) publishWithRetry(ctx context.Context, event *Event) error {
	var lastErr error
	delay := p.config.RetryDelay
	
	for attempt := 0; attempt <= p.config.MaxRetries; attempt++ {
		if attempt > 0 {
			// Wait before retry
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return ctx.Err()
			}
			delay = time.Duration(float64(delay) * p.config.BackoffFactor)
		}
		
		err := p.auditLogger.Log(ctx, event)
		if err == nil {
			if attempt > 0 {
				p.incrementRetryCount()
			}
			return nil
		}
		
		lastErr = err
		p.logger.Warn("Audit log attempt failed",
			zap.Error(err),
			zap.Int("attempt", attempt+1),
			zap.String("event_id", event.ID.String()))
	}
	
	return fmt.Errorf("failed to publish event after %d attempts: %w", p.config.MaxRetries+1, lastErr)
}

func (p *AuditEventPublisher) publishBatchWithRetry(ctx context.Context, events []*Event) error {
	var lastErr error
	delay := p.config.RetryDelay
	
	for attempt := 0; attempt <= p.config.MaxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return ctx.Err()
			}
			delay = time.Duration(float64(delay) * p.config.BackoffFactor)
		}
		
		err := p.auditLogger.LogBatch(ctx, events)
		if err == nil {
			if attempt > 0 {
				p.incrementRetryCount()
			}
			return nil
		}
		
		lastErr = err
		p.logger.Warn("Audit log batch attempt failed",
			zap.Error(err),
			zap.Int("attempt", attempt+1),
			zap.Int("batch_size", len(events)))
	}
	
	return fmt.Errorf("failed to publish batch after %d attempts: %w", p.config.MaxRetries+1, lastErr)
}

func (p *AuditEventPublisher) notifySubscribers(ctx context.Context, event DomainEvent) {
	p.subscriberMu.RLock()
	defer p.subscriberMu.RUnlock()
	
	for id, handler := range p.subscribers {
		go func(id string, handler EventHandler) {
			if err := handler.Handle(ctx, event); err != nil {
				p.logger.Error("Subscriber failed to handle event",
					zap.Error(err),
					zap.String("subscriber_id", id),
					zap.String("event_type", string(event.GetEventType())))
			}
		}(id, handler)
	}
}

// Metrics methods
func (p *AuditEventPublisher) incrementSuccessCount() {
	p.metrics.mu.Lock()
	p.metrics.EventsPublished++
	p.metrics.mu.Unlock()
}

func (p *AuditEventPublisher) incrementFailureCount() {
	p.metrics.mu.Lock()
	p.metrics.EventsFailed++
	p.metrics.mu.Unlock()
}

func (p *AuditEventPublisher) incrementBatchSuccessCount() {
	p.metrics.mu.Lock()
	p.metrics.BatchesPublished++
	p.metrics.mu.Unlock()
}

func (p *AuditEventPublisher) incrementBatchFailureCount() {
	p.metrics.mu.Lock()
	p.metrics.BatchesFailed++
	p.metrics.mu.Unlock()
}

func (p *AuditEventPublisher) incrementRetryCount() {
	p.metrics.mu.Lock()
	p.metrics.RetryAttempts++
	p.metrics.mu.Unlock()
}

func (p *AuditEventPublisher) updateQueueSize() {
	p.metrics.mu.Lock()
	p.metrics.QueueSize = int64(len(p.eventQueue))
	p.metrics.mu.Unlock()
}

// GetMetrics returns current publisher metrics
func (p *AuditEventPublisher) GetMetrics() PublisherMetrics {
	p.metrics.mu.RLock()
	defer p.metrics.mu.RUnlock()
	
	// Return a copy
	return PublisherMetrics{
		EventsPublished:   p.metrics.EventsPublished,
		EventsFailed:      p.metrics.EventsFailed,
		BatchesPublished:  p.metrics.BatchesPublished,
		BatchesFailed:     p.metrics.BatchesFailed,
		RetryAttempts:     p.metrics.RetryAttempts,
		QueueSize:         p.metrics.QueueSize,
		ProcessingLatency: p.metrics.ProcessingLatency,
	}
}

// subscription implements the Subscription interface
type subscription struct {
	id        string
	publisher *AuditEventPublisher
}

func (s *subscription) ID() string {
	return s.id
}

func (s *subscription) Unsubscribe() error {
	s.publisher.subscriberMu.Lock()
	defer s.publisher.subscriberMu.Unlock()
	
	delete(s.publisher.subscribers, s.id)
	return nil
}

// Helper function to create a test publisher (for testing only)
func NewTestEventPublisher() *AuditEventPublisher {
	logger, _ := zap.NewDevelopment()
	mockAuditLogger := &mockAuditLogger{}
	config := DefaultPublisherConfig()
	config.WorkerCount = 1
	config.QueueSize = 10
	
	return NewAuditEventPublisher(logger, mockAuditLogger, config)
}

// mockAuditLogger for testing
type mockAuditLogger struct {
	events []*Event
	mu     sync.Mutex
}

func (m *mockAuditLogger) Log(ctx context.Context, event *Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, event)
	return nil
}

func (m *mockAuditLogger) LogBatch(ctx context.Context, events []*Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, events...)
	return nil
}

func (m *mockAuditLogger) Health() error {
	return nil
}

func (m *mockAuditLogger) GetEvents() []*Event {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]*Event, len(m.events))
	copy(result, m.events)
	return result
}