package events

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/audit"
	dncevents "github.com/davidleathers/dependable-call-exchange-backend/internal/domain/dnc/events"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// DNCDomainEvent defines the interface for all DNC domain events
type DNCDomainEvent interface {
	GetEventType() audit.EventType
	GetEventVersion() string
	GetEventID() uuid.UUID
	GetTimestamp() time.Time
	GetAggregateID() string
	GetAggregateType() string
	ToAuditEvent() (*audit.Event, error)
	Validate() error
}

// DNCEventPublisher handles domain event publishing for DNC operations
// Integrates with existing audit system and provides reliable delivery guarantees
type DNCEventPublisher struct {
	logger *zap.Logger
	
	// Event storage and integration
	auditPublisher  *AuditEventPublisher
	eventStore      DNCEventStore
	deadLetterQueue DeadLetterQueue
	
	// Event versioning and serialization
	serializer      EventSerializer
	versionRegistry *EventVersionRegistry
	
	// Delivery guarantees
	deliveryGuarantee DeliveryGuaranteeType
	retryPolicy       RetryPolicy
	
	// Real-time streaming
	webhookManager    *WebhookManager
	streamingManager  *StreamingManager
	
	// Cache invalidation
	cacheInvalidator  CacheInvalidator
	
	// Monitoring and observability
	metrics *DNCEventMetrics
	tracer  trace.Tracer
	
	// Configuration
	config DNCEventPublisherConfig
	
	// Lifecycle management
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	
	// Event ordering and deduplication
	orderingManager   *EventOrderingManager
	deduplicator      *EventDeduplicator
	
	// Event replay capabilities
	replayManager     *EventReplayManager
}

// DNCEventStore provides persistent storage for DNC events
type DNCEventStore interface {
	Store(ctx context.Context, event DNCDomainEvent) error
	Get(ctx context.Context, eventID uuid.UUID) (DNCDomainEvent, error)
	GetByAggregateID(ctx context.Context, aggregateID string, fromVersion int) ([]DNCDomainEvent, error)
	GetEventStream(ctx context.Context, fromTimestamp time.Time) (<-chan DNCDomainEvent, error)
	Close() error
}

// DeadLetterQueue handles failed events that couldn't be processed
type DeadLetterQueue interface {
	Add(ctx context.Context, event DNCDomainEvent, reason string, attempts int) error
	GetFailed(ctx context.Context, limit int) ([]FailedEvent, error)
	Retry(ctx context.Context, eventID uuid.UUID) error
	Remove(ctx context.Context, eventID uuid.UUID) error
}

// FailedEvent represents an event that failed processing
type FailedEvent struct {
	Event     DNCDomainEvent
	Reason    string
	Attempts  int
	FirstFail time.Time
	LastFail  time.Time
}

// EventSerializer handles event serialization and versioning
type EventSerializer interface {
	Serialize(event DNCDomainEvent) ([]byte, error)
	Deserialize(data []byte, eventType audit.EventType, version string) (DNCDomainEvent, error)
	GetSupportedVersions(eventType audit.EventType) []string
}

// EventVersionRegistry manages event schema versions
type EventVersionRegistry struct {
	versions map[audit.EventType]map[string]EventSchema
	mu       sync.RWMutex
}

// EventSchema defines the structure and validation for event versions
type EventSchema struct {
	Version     string
	Schema      interface{}
	Deserializer func([]byte) (DNCDomainEvent, error)
}

// DeliveryGuaranteeType defines the level of delivery guarantee
type DeliveryGuaranteeType string

const (
	DeliveryAtLeastOnce  DeliveryGuaranteeType = "at_least_once"
	DeliveryAtMostOnce   DeliveryGuaranteeType = "at_most_once"
	DeliveryExactlyOnce  DeliveryGuaranteeType = "exactly_once"
)

// RetryPolicy configures retry behavior for failed events
type RetryPolicy struct {
	MaxAttempts     int
	InitialDelay    time.Duration
	MaxDelay        time.Duration
	BackoffFactor   float64
	RetryableErrors []string
}

// WebhookManager handles webhook notifications for external systems
type WebhookManager struct {
	endpoints map[string]WebhookEndpoint
	client    WebhookClient
	mu        sync.RWMutex
}

// WebhookEndpoint represents a webhook configuration
type WebhookEndpoint struct {
	URL             string
	Secret          string
	EventFilters    []audit.EventType
	RetryPolicy     RetryPolicy
	TimeoutDuration time.Duration
	Enabled         bool
}

// WebhookClient handles HTTP delivery to webhook endpoints
type WebhookClient interface {
	Send(ctx context.Context, endpoint WebhookEndpoint, event DNCDomainEvent) error
}

// StreamingManager handles real-time event streaming via WebSocket
type StreamingManager struct {
	connections map[string]*StreamingConnection
	filters     map[string]StreamingFilter
	mu          sync.RWMutex
}

// StreamingConnection represents a WebSocket connection for event streaming
type StreamingConnection struct {
	ID          string
	UserID      uuid.UUID
	Conn        interface{} // WebSocket connection
	LastPing    time.Time
	Filters     StreamingFilter
	BufferSize  int
	EventBuffer chan DNCDomainEvent
}

// StreamingFilter defines filtering criteria for real-time streams
type StreamingFilter struct {
	EventTypes     []audit.EventType
	AggregateTypes []string
	AggregateIDs   []string
	UserIDs        []uuid.UUID
}

// CacheInvalidator handles event-driven cache invalidation
type CacheInvalidator interface {
	InvalidateOnEvent(ctx context.Context, event DNCDomainEvent) error
	RegisterInvalidationRule(eventType audit.EventType, rule InvalidationRule)
}

// InvalidationRule defines cache invalidation logic for specific events
type InvalidationRule struct {
	CacheKeys    []string
	CachePattern string
	TTLReset     bool
}

// EventOrderingManager ensures proper event ordering per aggregate
type EventOrderingManager struct {
	pendingEvents map[string][]PendingEvent
	nextSequence  map[string]int64
	mu            sync.RWMutex
}

// PendingEvent represents an event waiting for ordering
type PendingEvent struct {
	Event      DNCDomainEvent
	Sequence   int64
	ReceivedAt time.Time
}

// EventDeduplicator prevents duplicate event processing
type EventDeduplicator struct {
	processedEvents map[uuid.UUID]time.Time
	ttl             time.Duration
	mu              sync.RWMutex
}

// EventReplayManager handles event replay for debugging and recovery
type EventReplayManager struct {
	eventStore DNCEventStore
	logger     *zap.Logger
}

// DNCEventPublisherConfig configures the DNC event publisher
type DNCEventPublisherConfig struct {
	// Core settings
	DeliveryGuarantee DeliveryGuaranteeType
	RetryPolicy       RetryPolicy
	
	// Event storage
	EventStoreBatchSize   int
	EventStoreTimeout     time.Duration
	
	// Dead letter queue
	DeadLetterQueueSize   int
	DeadLetterRetention   time.Duration
	
	// Webhooks
	WebhookTimeout        time.Duration
	WebhookRetryAttempts  int
	
	// Streaming
	StreamingBufferSize   int
	StreamingTimeout      time.Duration
	
	// Cache invalidation
	CacheInvalidationEnabled bool
	CacheInvalidationTimeout time.Duration
	
	// Event ordering
	OrderingEnabled       bool
	OrderingTimeout       time.Duration
	
	// Deduplication
	DeduplicationEnabled  bool
	DeduplicationWindow   time.Duration
	
	// Performance
	WorkerPoolSize        int
	EventBatchSize        int
	EventBatchTimeout     time.Duration
}

// DefaultDNCEventPublisherConfig returns default configuration
func DefaultDNCEventPublisherConfig() DNCEventPublisherConfig {
	return DNCEventPublisherConfig{
		DeliveryGuarantee:         DeliveryAtLeastOnce,
		RetryPolicy: RetryPolicy{
			MaxAttempts:   3,
			InitialDelay:  100 * time.Millisecond,
			MaxDelay:      5 * time.Second,
			BackoffFactor: 2.0,
			RetryableErrors: []string{
				"connection_error",
				"timeout",
				"server_error",
			},
		},
		EventStoreBatchSize:       100,
		EventStoreTimeout:         5 * time.Second,
		DeadLetterQueueSize:       1000,
		DeadLetterRetention:       7 * 24 * time.Hour,
		WebhookTimeout:            10 * time.Second,
		WebhookRetryAttempts:      3,
		StreamingBufferSize:       1000,
		StreamingTimeout:          30 * time.Second,
		CacheInvalidationEnabled:  true,
		CacheInvalidationTimeout:  1 * time.Second,
		OrderingEnabled:           true,
		OrderingTimeout:           10 * time.Second,
		DeduplicationEnabled:      true,
		DeduplicationWindow:       5 * time.Minute,
		WorkerPoolSize:            5,
		EventBatchSize:            50,
		EventBatchTimeout:         100 * time.Millisecond,
	}
}

// DNCEventMetrics tracks DNC event publishing metrics
type DNCEventMetrics struct {
	// Counters
	eventsPublished      metric.Int64Counter
	eventsStored         metric.Int64Counter
	eventsFailed         metric.Int64Counter
	eventsDropped        metric.Int64Counter
	webhooksDelivered    metric.Int64Counter
	webhooksFailed       metric.Int64Counter
	cacheInvalidations   metric.Int64Counter
	
	// Histograms
	publishLatency       metric.Float64Histogram
	storeLatency         metric.Float64Histogram
	webhookLatency       metric.Float64Histogram
	eventSize           metric.Int64Histogram
	
	// Gauges
	deadLetterQueueSize  metric.Int64ObservableGauge
	activeStreams        metric.Int64ObservableGauge
	pendingEvents        metric.Int64ObservableGauge
	
	// Internal stats
	stats struct {
		mu                    sync.RWMutex
		totalPublished        int64
		totalStored           int64
		totalFailed           int64
		totalDropped          int64
		currentDeadLetterSize int64
		activeStreamCount     int64
		pendingEventCount     int64
	}
}

// NewDNCEventPublisher creates a new DNC event publisher
func NewDNCEventPublisher(
	ctx context.Context,
	logger *zap.Logger,
	auditPublisher *AuditEventPublisher,
	eventStore DNCEventStore,
	config DNCEventPublisherConfig,
) (*DNCEventPublisher, error) {
	ctx, cancel := context.WithCancel(ctx)
	
	publisher := &DNCEventPublisher{
		logger:         logger,
		auditPublisher: auditPublisher,
		eventStore:     eventStore,
		config:         config,
		ctx:            ctx,
		cancel:         cancel,
		tracer:         otel.Tracer("dnc.event.publisher"),
	}
	
	// Initialize components
	if err := publisher.initializeComponents(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to initialize components: %w", err)
	}
	
	// Initialize metrics
	if err := publisher.initMetrics(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to initialize metrics: %w", err)
	}
	
	// Start background workers
	publisher.startWorkers()
	
	logger.Info("DNC event publisher initialized",
		zap.String("delivery_guarantee", string(config.DeliveryGuarantee)),
		zap.Int("worker_pool_size", config.WorkerPoolSize),
		zap.Bool("ordering_enabled", config.OrderingEnabled),
		zap.Bool("deduplication_enabled", config.DeduplicationEnabled),
	)
	
	return publisher, nil
}

// PublishNumberSuppressed publishes a number suppressed event
func (p *DNCEventPublisher) PublishNumberSuppressed(
	ctx context.Context,
	event *dncevents.NumberSuppressedEvent,
) error {
	return p.publishEvent(ctx, event, "number_suppressed")
}

// PublishNumberReleased publishes a number released event
func (p *DNCEventPublisher) PublishNumberReleased(
	ctx context.Context,
	event *dncevents.NumberReleasedEvent,
) error {
	return p.publishEvent(ctx, event, "number_released")
}

// PublishDNCCheckPerformed publishes a DNC check performed event
func (p *DNCEventPublisher) PublishDNCCheckPerformed(
	ctx context.Context,
	event *dncevents.DNCCheckPerformedEvent,
) error {
	return p.publishEvent(ctx, event, "dnc_check_performed")
}

// PublishDNCListSynced publishes a DNC list synced event
func (p *DNCEventPublisher) PublishDNCListSynced(
	ctx context.Context,
	event *dncevents.DNCListSyncedEvent,
) error {
	return p.publishEvent(ctx, event, "dnc_list_synced")
}

// publishEvent handles the core event publishing logic
func (p *DNCEventPublisher) publishEvent(
	ctx context.Context,
	event DNCDomainEvent,
	eventCategory string,
) error {
	ctx, span := p.tracer.Start(ctx, "DNCEventPublisher.publishEvent",
		trace.WithAttributes(
			attribute.String("event.id", event.GetEventID().String()),
			attribute.String("event.type", string(event.GetEventType())),
			attribute.String("event.category", eventCategory),
			attribute.String("aggregate.id", event.GetAggregateID()),
			attribute.String("aggregate.type", event.GetAggregateType()),
		),
	)
	defer span.End()
	
	start := time.Now()
	
	// Validate event
	if err := event.Validate(); err != nil {
		span.RecordError(err)
		p.recordEventFailed("validation_error")
		return errors.NewValidationError("INVALID_EVENT", 
			"event validation failed").WithCause(err)
	}
	
	// Check for duplicates if deduplication is enabled
	if p.config.DeduplicationEnabled {
		if isDuplicate := p.deduplicator.IsDuplicate(event.GetEventID()); isDuplicate {
			p.logger.Debug("Duplicate event detected, skipping",
				zap.String("event_id", event.GetEventID().String()),
			)
			span.SetAttributes(attribute.Bool("event.duplicate", true))
			return nil
		}
	}
	
	// Handle event ordering if enabled
	if p.config.OrderingEnabled {
		if err := p.orderingManager.ProcessEvent(ctx, event); err != nil {
			span.RecordError(err)
			p.recordEventFailed("ordering_error")
			return errors.NewInternalError("event ordering failed").WithCause(err)
		}
	}
	
	// Store event for exactly-once semantics
	if p.config.DeliveryGuarantee == DeliveryExactlyOnce {
		storeCtx, storeCancel := context.WithTimeout(ctx, p.config.EventStoreTimeout)
		defer storeCancel()
		
		if err := p.eventStore.Store(storeCtx, event); err != nil {
			span.RecordError(err)
			p.recordEventFailed("store_error")
			return errors.NewInternalError("failed to store event").WithCause(err)
		}
		p.recordEventStored(time.Since(start))
	}
	
	// Convert to audit event for integration with existing system
	auditEvent, err := event.ToAuditEvent()
	if err != nil {
		span.RecordError(err)
		p.recordEventFailed("audit_conversion_error")
		return errors.NewInternalError("failed to convert to audit event").WithCause(err)
	}
	
	// Publish to audit system
	if err := p.auditPublisher.Publish(ctx, auditEvent); err != nil {
		span.RecordError(err)
		p.recordEventFailed("audit_publish_error")
		
		// Add to dead letter queue for retry
		if dlqErr := p.deadLetterQueue.Add(ctx, event, "audit_publish_failed", 1); dlqErr != nil {
			p.logger.Error("Failed to add event to dead letter queue",
				zap.Error(dlqErr),
				zap.String("event_id", event.GetEventID().String()),
			)
		}
		
		return errors.NewInternalError("failed to publish to audit system").WithCause(err)
	}
	
	// Handle real-time streaming
	if err := p.streamingManager.StreamEvent(ctx, event); err != nil {
		p.logger.Warn("Failed to stream event",
			zap.Error(err),
			zap.String("event_id", event.GetEventID().String()),
		)
		// Non-critical failure, don't fail the entire operation
	}
	
	// Handle webhook notifications
	if err := p.webhookManager.NotifyWebhooks(ctx, event); err != nil {
		p.logger.Warn("Failed to notify webhooks",
			zap.Error(err),
			zap.String("event_id", event.GetEventID().String()),
		)
		// Non-critical failure, don't fail the entire operation
	}
	
	// Handle cache invalidation
	if p.config.CacheInvalidationEnabled {
		if err := p.cacheInvalidator.InvalidateOnEvent(ctx, event); err != nil {
			p.logger.Warn("Failed to invalidate cache",
				zap.Error(err),
				zap.String("event_id", event.GetEventID().String()),
			)
			// Non-critical failure, don't fail the entire operation
		}
	}
	
	// Mark as processed for deduplication
	if p.config.DeduplicationEnabled {
		p.deduplicator.MarkProcessed(event.GetEventID())
	}
	
	p.recordEventPublished(time.Since(start))
	
	p.logger.Info("DNC event published successfully",
		zap.String("event_id", event.GetEventID().String()),
		zap.String("event_type", string(event.GetEventType())),
		zap.String("aggregate_id", event.GetAggregateID()),
		zap.Duration("latency", time.Since(start)),
	)
	
	return nil
}

// ReplayEvents replays events from a specific timestamp for debugging/recovery
func (p *DNCEventPublisher) ReplayEvents(
	ctx context.Context,
	fromTimestamp time.Time,
	toTimestamp *time.Time,
	eventTypes []audit.EventType,
) error {
	return p.replayManager.ReplayEvents(ctx, fromTimestamp, toTimestamp, eventTypes)
}

// GetEventStream returns a stream of events for real-time processing
func (p *DNCEventPublisher) GetEventStream(
	ctx context.Context,
	fromTimestamp time.Time,
) (<-chan DNCDomainEvent, error) {
	return p.eventStore.GetEventStream(ctx, fromTimestamp)
}

// AddWebhookEndpoint adds a new webhook endpoint for notifications
func (p *DNCEventPublisher) AddWebhookEndpoint(
	endpoint WebhookEndpoint,
) error {
	return p.webhookManager.AddEndpoint(endpoint)
}

// RemoveWebhookEndpoint removes a webhook endpoint
func (p *DNCEventPublisher) RemoveWebhookEndpoint(url string) error {
	return p.webhookManager.RemoveEndpoint(url)
}

// AddStreamingConnection adds a WebSocket connection for real-time events
func (p *DNCEventPublisher) AddStreamingConnection(
	connectionID string,
	userID uuid.UUID,
	conn interface{},
	filters StreamingFilter,
) error {
	return p.streamingManager.AddConnection(connectionID, userID, conn, filters)
}

// RemoveStreamingConnection removes a WebSocket connection
func (p *DNCEventPublisher) RemoveStreamingConnection(connectionID string) error {
	return p.streamingManager.RemoveConnection(connectionID)
}

// GetMetrics returns current publisher metrics
func (p *DNCEventPublisher) GetMetrics() map[string]interface{} {
	p.metrics.stats.mu.RLock()
	defer p.metrics.stats.mu.RUnlock()
	
	return map[string]interface{}{
		"events_published":        p.metrics.stats.totalPublished,
		"events_stored":           p.metrics.stats.totalStored,
		"events_failed":           p.metrics.stats.totalFailed,
		"events_dropped":          p.metrics.stats.totalDropped,
		"dead_letter_queue_size":  p.metrics.stats.currentDeadLetterSize,
		"active_streams":          p.metrics.stats.activeStreamCount,
		"pending_events":          p.metrics.stats.pendingEventCount,
	}
}

// Health checks the health of the publisher and its dependencies
func (p *DNCEventPublisher) Health() error {
	// Check if publisher is running
	select {
	case <-p.ctx.Done():
		return errors.NewInternalError("DNC event publisher is shut down")
	default:
	}
	
	// Check audit publisher health
	if err := p.auditPublisher.Health(); err != nil {
		return errors.NewInternalError("audit publisher unhealthy").WithCause(err)
	}
	
	// Check event store health
	storeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	// Simple health check by attempting to get non-existent event
	_, err := p.eventStore.Get(storeCtx, uuid.New())
	if err != nil && !errors.IsNotFoundError(err) {
		return errors.NewInternalError("event store unhealthy").WithCause(err)
	}
	
	return nil
}

// Close gracefully shuts down the publisher
func (p *DNCEventPublisher) Close() error {
	p.logger.Info("Shutting down DNC event publisher")
	
	// Signal shutdown
	p.cancel()
	
	// Wait for workers with timeout
	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		p.logger.Info("All workers shut down gracefully")
	case <-time.After(30 * time.Second):
		p.logger.Warn("Shutdown timeout reached")
	}
	
	// Close components
	if err := p.eventStore.Close(); err != nil {
		p.logger.Error("Failed to close event store", zap.Error(err))
	}
	
	if err := p.webhookManager.Close(); err != nil {
		p.logger.Error("Failed to close webhook manager", zap.Error(err))
	}
	
	if err := p.streamingManager.Close(); err != nil {
		p.logger.Error("Failed to close streaming manager", zap.Error(err))
	}
	
	return nil
}

// Private methods

func (p *DNCEventPublisher) initializeComponents() error {
	// Initialize version registry
	p.versionRegistry = NewEventVersionRegistry()
	p.registerEventVersions()
	
	// Initialize serializer
	p.serializer = NewJSONEventSerializer(p.versionRegistry)
	
	// Initialize dead letter queue
	p.deadLetterQueue = NewMemoryDeadLetterQueue(p.config.DeadLetterQueueSize, p.logger)
	
	// Initialize webhook manager
	p.webhookManager = NewWebhookManager(p.logger, p.config.WebhookTimeout)
	
	// Initialize streaming manager
	p.streamingManager = NewStreamingManager(p.logger, p.config.StreamingBufferSize)
	
	// Initialize cache invalidator
	p.cacheInvalidator = NewRedisCacheInvalidator(p.logger)
	
	// Initialize ordering manager
	if p.config.OrderingEnabled {
		p.orderingManager = NewEventOrderingManager(p.config.OrderingTimeout)
	}
	
	// Initialize deduplicator
	if p.config.DeduplicationEnabled {
		p.deduplicator = NewEventDeduplicator(p.config.DeduplicationWindow)
	}
	
	// Initialize replay manager
	p.replayManager = NewEventReplayManager(p.eventStore, p.logger)
	
	return nil
}

func (p *DNCEventPublisher) registerEventVersions() {
	// Register version 1.0 for all DNC events
	p.versionRegistry.Register(audit.EventDNCNumberSuppressed, "1.0", EventSchema{
		Version: "1.0",
		Deserializer: func(data []byte) (DNCDomainEvent, error) {
			var event dncevents.NumberSuppressedEvent
			if err := json.Unmarshal(data, &event); err != nil {
				return nil, err
			}
			return &event, nil
		},
	})
	
	p.versionRegistry.Register(audit.EventDNCNumberReleased, "1.0", EventSchema{
		Version: "1.0",
		Deserializer: func(data []byte) (DNCDomainEvent, error) {
			var event dncevents.NumberReleasedEvent
			if err := json.Unmarshal(data, &event); err != nil {
				return nil, err
			}
			return &event, nil
		},
	})
	
	p.versionRegistry.Register(audit.EventDNCCheckPerformed, "1.0", EventSchema{
		Version: "1.0",
		Deserializer: func(data []byte) (DNCDomainEvent, error) {
			var event dncevents.DNCCheckPerformedEvent
			if err := json.Unmarshal(data, &event); err != nil {
				return nil, err
			}
			return &event, nil
		},
	})
	
	p.versionRegistry.Register(audit.EventDNCListSynced, "1.0", EventSchema{
		Version: "1.0",
		Deserializer: func(data []byte) (DNCDomainEvent, error) {
			var event dncevents.DNCListSyncedEvent
			if err := json.Unmarshal(data, &event); err != nil {
				return nil, err
			}
			return &event, nil
		},
	})
}

func (p *DNCEventPublisher) startWorkers() {
	// Start dead letter queue processor
	p.wg.Add(1)
	go p.deadLetterProcessor()
	
	// Start metrics collector
	p.wg.Add(1)
	go p.metricsCollector()
	
	// Start cleanup worker
	p.wg.Add(1)
	go p.cleanupWorker()
}

func (p *DNCEventPublisher) deadLetterProcessor() {
	defer p.wg.Done()
	
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			p.processDeadLetterQueue()
		case <-p.ctx.Done():
			return
		}
	}
}

func (p *DNCEventPublisher) processDeadLetterQueue() {
	ctx, cancel := context.WithTimeout(p.ctx, 10*time.Second)
	defer cancel()
	
	failedEvents, err := p.deadLetterQueue.GetFailed(ctx, 100)
	if err != nil {
		p.logger.Error("Failed to get dead letter queue events", zap.Error(err))
		return
	}
	
	for _, failedEvent := range failedEvents {
		if failedEvent.Attempts >= p.config.RetryPolicy.MaxAttempts {
			continue
		}
		
		if err := p.deadLetterQueue.Retry(ctx, failedEvent.Event.GetEventID()); err != nil {
			p.logger.Error("Failed to retry event",
				zap.Error(err),
				zap.String("event_id", failedEvent.Event.GetEventID().String()),
			)
		}
	}
}

func (p *DNCEventPublisher) metricsCollector() {
	defer p.wg.Done()
	
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			p.updateMetrics()
		case <-p.ctx.Done():
			return
		}
	}
}

func (p *DNCEventPublisher) updateMetrics() {
	// Update dead letter queue size
	ctx, cancel := context.WithTimeout(p.ctx, 5*time.Second)
	defer cancel()
	
	failedEvents, err := p.deadLetterQueue.GetFailed(ctx, 0) // Get count only
	if err == nil {
		p.metrics.stats.mu.Lock()
		p.metrics.stats.currentDeadLetterSize = int64(len(failedEvents))
		p.metrics.stats.mu.Unlock()
	}
	
	// Update streaming connection count
	p.metrics.stats.mu.Lock()
	p.metrics.stats.activeStreamCount = int64(p.streamingManager.GetConnectionCount())
	p.metrics.stats.mu.Unlock()
}

func (p *DNCEventPublisher) cleanupWorker() {
	defer p.wg.Done()
	
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			p.performCleanup()
		case <-p.ctx.Done():
			return
		}
	}
}

func (p *DNCEventPublisher) performCleanup() {
	// Clean up expired deduplication entries
	if p.config.DeduplicationEnabled {
		p.deduplicator.Cleanup()
	}
	
	// Clean up stale streaming connections
	p.streamingManager.CleanupStaleConnections()
	
	// Clean up old dead letter queue entries
	ctx, cancel := context.WithTimeout(p.ctx, 30*time.Second)
	defer cancel()
	
	// Remove dead letter entries older than retention period
	cutoff := time.Now().Add(-p.config.DeadLetterRetention)
	_ = cutoff // TODO: Implement cleanup in dead letter queue interface
	_ = ctx
}

// Metric recording methods

func (p *DNCEventPublisher) initMetrics() error {
	meter := otel.Meter("dnc.event.publisher")
	
	// Create counters
	eventsPublished, err := meter.Int64Counter("dnc.events.published",
		metric.WithDescription("Total number of DNC events published"))
	if err != nil {
		return err
	}
	p.metrics.eventsPublished = eventsPublished
	
	eventsStored, err := meter.Int64Counter("dnc.events.stored",
		metric.WithDescription("Total number of DNC events stored"))
	if err != nil {
		return err
	}
	p.metrics.eventsStored = eventsStored
	
	eventsFailed, err := meter.Int64Counter("dnc.events.failed",
		metric.WithDescription("Total number of DNC events that failed to publish"))
	if err != nil {
		return err
	}
	p.metrics.eventsFailed = eventsFailed
	
	// Create histograms
	publishLatency, err := meter.Float64Histogram("dnc.publish.latency",
		metric.WithDescription("Latency of publishing DNC events"),
		metric.WithUnit("ms"))
	if err != nil {
		return err
	}
	p.metrics.publishLatency = publishLatency
	
	storeLatency, err := meter.Float64Histogram("dnc.store.latency",
		metric.WithDescription("Latency of storing DNC events"),
		metric.WithUnit("ms"))
	if err != nil {
		return err
	}
	p.metrics.storeLatency = storeLatency
	
	// Create observable gauges
	deadLetterQueueSize, err := meter.Int64ObservableGauge("dnc.dead_letter_queue.size",
		metric.WithDescription("Current size of the dead letter queue"))
	if err != nil {
		return err
	}
	p.metrics.deadLetterQueueSize = deadLetterQueueSize
	
	// Register callbacks for observable gauges
	meter.RegisterCallback(func(ctx context.Context, observer metric.Observer) error {
		p.metrics.stats.mu.RLock()
		defer p.metrics.stats.mu.RUnlock()
		
		observer.ObserveInt64(deadLetterQueueSize, p.metrics.stats.currentDeadLetterSize)
		
		return nil
	}, deadLetterQueueSize)
	
	return nil
}

func (p *DNCEventPublisher) recordEventPublished(latency time.Duration) {
	p.metrics.eventsPublished.Add(context.Background(), 1)
	p.metrics.publishLatency.Record(context.Background(), float64(latency.Milliseconds()))
	
	p.metrics.stats.mu.Lock()
	p.metrics.stats.totalPublished++
	p.metrics.stats.mu.Unlock()
}

func (p *DNCEventPublisher) recordEventStored(latency time.Duration) {
	p.metrics.eventsStored.Add(context.Background(), 1)
	p.metrics.storeLatency.Record(context.Background(), float64(latency.Milliseconds()))
	
	p.metrics.stats.mu.Lock()
	p.metrics.stats.totalStored++
	p.metrics.stats.mu.Unlock()
}

func (p *DNCEventPublisher) recordEventFailed(reason string) {
	p.metrics.eventsFailed.Add(context.Background(), 1,
		metric.WithAttributes(attribute.String("reason", reason)))
	
	p.metrics.stats.mu.Lock()
	p.metrics.stats.totalFailed++
	p.metrics.stats.mu.Unlock()
}