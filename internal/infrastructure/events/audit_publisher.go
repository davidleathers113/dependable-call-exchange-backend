package events

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/audit"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// Transport protocol types
type TransportType string

const (
	TransportWebSocket TransportType = "websocket"
	TransportKafka     TransportType = "kafka"
	TransportGRPC      TransportType = "grpc"
	TransportHTTP      TransportType = "http"
)

// AuditEventPublisher implements real-time streaming of audit events
type AuditEventPublisher struct {
	logger     *zap.Logger
	transports map[TransportType]EventTransport
	
	// Subscription management
	subscriptions    map[string]*Subscription
	subscriptionsMu  sync.RWMutex
	
	// Event filtering and routing
	router          *EventRouter
	
	// Performance monitoring
	metrics         *PublisherMetrics
	tracer          trace.Tracer
	
	// Configuration
	config          PublisherConfig
	
	// Lifecycle management
	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup
	
	// Event queues for async processing
	eventQueue      chan *audit.Event
	criticalQueue   chan *audit.Event
	
	// Backpressure handling
	backpressure    *BackpressureController
}

// EventTransport defines interface for different transport protocols
type EventTransport interface {
	// Send sends a single event
	Send(ctx context.Context, event *audit.Event, subscribers []string) error
	
	// SendBatch sends multiple events
	SendBatch(ctx context.Context, events []*audit.Event, subscribers []string) error
	
	// GetProtocol returns the transport protocol type
	GetProtocol() TransportType
	
	// IsHealthy checks transport health
	IsHealthy() bool
	
	// Close gracefully shuts down the transport
	Close() error
}

// Subscription represents an event subscription
type Subscription struct {
	ID          string
	UserID      uuid.UUID
	Transport   TransportType
	Filters     EventFilters
	CreatedAt   time.Time
	LastEventAt time.Time
	
	// Delivery tracking
	DeliveryStats DeliveryStats
	
	// Connection-specific data
	ConnectionData interface{}
}

// EventFilters defines filtering criteria for events
type EventFilters struct {
	EventTypes    []audit.EventType
	Severity      []audit.Severity
	EntityTypes   []string
	EntityIDs     []uuid.UUID
	UserIDs       []uuid.UUID
	TimeRange     *TimeRange
	CustomFilters map[string]interface{}
}

// TimeRange defines a time-based filter
type TimeRange struct {
	Start time.Time
	End   time.Time
}

// DeliveryStats tracks delivery statistics per subscription
type DeliveryStats struct {
	EventsSent      int64
	EventsFailed    int64
	LastSuccess     time.Time
	LastFailure     time.Time
	AvgLatency      time.Duration
	mu              sync.RWMutex
}

// PublisherConfig configures the audit event publisher
type PublisherConfig struct {
	// Queue sizes
	EventQueueSize    int
	CriticalQueueSize int
	
	// Worker configuration
	WorkerCount       int
	CriticalWorkers   int
	
	// Batching
	BatchSize         int
	BatchTimeout      time.Duration
	
	// Retry configuration
	MaxRetries        int
	RetryDelay        time.Duration
	RetryBackoff      float64
	
	// Backpressure
	MaxQueueDepth     int
	BackpressureDelay time.Duration
	
	// Timeouts
	SendTimeout       time.Duration
	ShutdownTimeout   time.Duration
}

// DefaultPublisherConfig returns default configuration
func DefaultPublisherConfig() PublisherConfig {
	return PublisherConfig{
		EventQueueSize:    10000,
		CriticalQueueSize: 1000,
		WorkerCount:       10,
		CriticalWorkers:   5,
		BatchSize:         100,
		BatchTimeout:      50 * time.Millisecond,
		MaxRetries:        3,
		RetryDelay:        100 * time.Millisecond,
		RetryBackoff:      2.0,
		MaxQueueDepth:     5000,
		BackpressureDelay: 10 * time.Millisecond,
		SendTimeout:       5 * time.Second,
		ShutdownTimeout:   30 * time.Second,
	}
}

// PublisherMetrics tracks publisher performance metrics
type PublisherMetrics struct {
	// Counters
	eventsPublished  metric.Int64Counter
	eventsFiltered   metric.Int64Counter
	eventsFailed     metric.Int64Counter
	eventsDropped    metric.Int64Counter
	
	// Histograms
	publishLatency   metric.Float64Histogram
	queueDepth       metric.Int64Histogram
	batchSize        metric.Int64Histogram
	
	// Gauges
	activeSubscriptions metric.Int64ObservableGauge
	queueSize          metric.Int64ObservableGauge
	
	// Internal stats
	stats struct {
		mu               sync.RWMutex
		totalPublished   int64
		totalFailed      int64
		totalDropped     int64
		currentQueueSize int64
	}
}

// NewAuditEventPublisher creates a new audit event publisher
func NewAuditEventPublisher(
	ctx context.Context,
	logger *zap.Logger,
	config PublisherConfig,
	transports map[TransportType]EventTransport,
) (*AuditEventPublisher, error) {
	ctx, cancel := context.WithCancel(ctx)
	
	publisher := &AuditEventPublisher{
		logger:        logger,
		transports:    transports,
		subscriptions: make(map[string]*Subscription),
		config:        config,
		ctx:           ctx,
		cancel:        cancel,
		eventQueue:    make(chan *audit.Event, config.EventQueueSize),
		criticalQueue: make(chan *audit.Event, config.CriticalQueueSize),
		tracer:        otel.Tracer("audit.publisher"),
	}
	
	// Initialize components
	publisher.router = NewEventRouter(logger)
	publisher.backpressure = NewBackpressureController(config.MaxQueueDepth, config.BackpressureDelay)
	
	// Initialize metrics
	if err := publisher.initMetrics(); err != nil {
		return nil, fmt.Errorf("failed to initialize metrics: %w", err)
	}
	
	// Start workers
	publisher.startWorkers()
	
	logger.Info("Audit event publisher initialized",
		zap.Int("transports", len(transports)),
		zap.Int("workers", config.WorkerCount),
		zap.Int("critical_workers", config.CriticalWorkers),
	)
	
	return publisher, nil
}

// Publish publishes an audit event to all matching subscribers
func (p *AuditEventPublisher) Publish(ctx context.Context, event *audit.Event) error {
	ctx, span := p.tracer.Start(ctx, "AuditEventPublisher.Publish",
		trace.WithAttributes(
			attribute.String("event.id", event.ID.String()),
			attribute.String("event.type", string(event.Type)),
			attribute.String("event.severity", string(event.Severity)),
		),
	)
	defer span.End()
	
	// Apply backpressure if needed
	if err := p.backpressure.Wait(ctx); err != nil {
		span.RecordError(err)
		p.recordEventDropped("backpressure")
		return errors.NewInternalError("backpressure limit reached").WithCause(err)
	}
	
	// Route critical events to priority queue
	if event.Severity == audit.SeverityCritical {
		select {
		case p.criticalQueue <- event:
			span.SetAttributes(attribute.Bool("queued.critical", true))
		case <-ctx.Done():
			span.RecordError(ctx.Err())
			return ctx.Err()
		default:
			// Critical queue full, try regular queue
			select {
			case p.eventQueue <- event:
				span.SetAttributes(attribute.Bool("queued.regular", true))
			case <-ctx.Done():
				span.RecordError(ctx.Err())
				return ctx.Err()
			default:
				p.recordEventDropped("queue_full")
				return errors.NewInternalError("event queues full")
			}
		}
	} else {
		// Regular events go to normal queue
		select {
		case p.eventQueue <- event:
			span.SetAttributes(attribute.Bool("queued.regular", true))
		case <-ctx.Done():
			span.RecordError(ctx.Err())
			return ctx.Err()
		default:
			p.recordEventDropped("queue_full")
			return errors.NewInternalError("event queue full")
		}
	}
	
	p.updateQueueMetrics()
	return nil
}

// Subscribe creates a new event subscription
func (p *AuditEventPublisher) Subscribe(
	ctx context.Context,
	userID uuid.UUID,
	transport TransportType,
	filters EventFilters,
	connectionData interface{},
) (*Subscription, error) {
	ctx, span := p.tracer.Start(ctx, "AuditEventPublisher.Subscribe",
		trace.WithAttributes(
			attribute.String("user.id", userID.String()),
			attribute.String("transport", string(transport)),
			attribute.Int("filters.event_types", len(filters.EventTypes)),
		),
	)
	defer span.End()
	
	// Validate transport exists
	if _, ok := p.transports[transport]; !ok {
		return nil, errors.NewValidationError("INVALID_TRANSPORT", 
			fmt.Sprintf("transport %s not available", transport))
	}
	
	subscription := &Subscription{
		ID:             uuid.New().String(),
		UserID:         userID,
		Transport:      transport,
		Filters:        filters,
		CreatedAt:      time.Now(),
		ConnectionData: connectionData,
	}
	
	p.subscriptionsMu.Lock()
	p.subscriptions[subscription.ID] = subscription
	p.subscriptionsMu.Unlock()
	
	p.router.AddSubscription(subscription)
	
	p.logger.Info("New audit event subscription created",
		zap.String("subscription_id", subscription.ID),
		zap.String("user_id", userID.String()),
		zap.String("transport", string(transport)),
		zap.Int("filter_count", p.countFilters(filters)),
	)
	
	span.SetAttributes(attribute.String("subscription.id", subscription.ID))
	p.recordSubscriptionChange(1)
	
	return subscription, nil
}

// Unsubscribe removes an event subscription
func (p *AuditEventPublisher) Unsubscribe(ctx context.Context, subscriptionID string) error {
	p.subscriptionsMu.Lock()
	subscription, exists := p.subscriptions[subscriptionID]
	if !exists {
		p.subscriptionsMu.Unlock()
		return errors.NewNotFoundError("subscription not found")
	}
	delete(p.subscriptions, subscriptionID)
	p.subscriptionsMu.Unlock()
	
	p.router.RemoveSubscription(subscriptionID)
	
	p.logger.Info("Audit event subscription removed",
		zap.String("subscription_id", subscriptionID),
		zap.String("user_id", subscription.UserID.String()),
	)
	
	p.recordSubscriptionChange(-1)
	return nil
}

// GetSubscriptionStats returns statistics for a subscription
func (p *AuditEventPublisher) GetSubscriptionStats(subscriptionID string) (*DeliveryStats, error) {
	p.subscriptionsMu.RLock()
	subscription, exists := p.subscriptions[subscriptionID]
	p.subscriptionsMu.RUnlock()
	
	if !exists {
		return nil, errors.NewNotFoundError("subscription not found")
	}
	
	subscription.DeliveryStats.mu.RLock()
	defer subscription.DeliveryStats.mu.RUnlock()
	
	// Return a copy of the stats
	stats := &DeliveryStats{
		EventsSent:   subscription.DeliveryStats.EventsSent,
		EventsFailed: subscription.DeliveryStats.EventsFailed,
		LastSuccess:  subscription.DeliveryStats.LastSuccess,
		LastFailure:  subscription.DeliveryStats.LastFailure,
		AvgLatency:   subscription.DeliveryStats.AvgLatency,
	}
	
	return stats, nil
}

// GetMetrics returns current publisher metrics
func (p *AuditEventPublisher) GetMetrics() map[string]interface{} {
	p.metrics.stats.mu.RLock()
	defer p.metrics.stats.mu.RUnlock()
	
	p.subscriptionsMu.RLock()
	subscriptionCount := len(p.subscriptions)
	p.subscriptionsMu.RUnlock()
	
	return map[string]interface{}{
		"events_published":     p.metrics.stats.totalPublished,
		"events_failed":        p.metrics.stats.totalFailed,
		"events_dropped":       p.metrics.stats.totalDropped,
		"queue_size":           len(p.eventQueue),
		"critical_queue_size":  len(p.criticalQueue),
		"active_subscriptions": subscriptionCount,
		"transports":           len(p.transports),
	}
}

// Health checks the health of the publisher and its transports
func (p *AuditEventPublisher) Health() error {
	// Check if publisher is running
	select {
	case <-p.ctx.Done():
		return errors.NewInternalError("publisher is shut down")
	default:
	}
	
	// Check transport health
	unhealthyTransports := []string{}
	for transportType, transport := range p.transports {
		if !transport.IsHealthy() {
			unhealthyTransports = append(unhealthyTransports, string(transportType))
		}
	}
	
	if len(unhealthyTransports) > 0 {
		return errors.NewInternalError(fmt.Sprintf("unhealthy transports: %v", unhealthyTransports))
	}
	
	// Check queue depth
	queueDepth := len(p.eventQueue)
	if queueDepth > p.config.EventQueueSize*9/10 { // 90% full
		return errors.NewInternalError(fmt.Sprintf("event queue nearly full: %d/%d", 
			queueDepth, p.config.EventQueueSize))
	}
	
	return nil
}

// Close gracefully shuts down the publisher
func (p *AuditEventPublisher) Close() error {
	p.logger.Info("Shutting down audit event publisher")
	
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
	case <-time.After(p.config.ShutdownTimeout):
		p.logger.Warn("Shutdown timeout reached, some events may be lost",
			zap.Int("pending_events", len(p.eventQueue)),
			zap.Int("pending_critical", len(p.criticalQueue)),
		)
	}
	
	// Close all transports
	for transportType, transport := range p.transports {
		if err := transport.Close(); err != nil {
			p.logger.Error("Failed to close transport",
				zap.String("transport", string(transportType)),
				zap.Error(err),
			)
		}
	}
	
	return nil
}

// Private methods

func (p *AuditEventPublisher) startWorkers() {
	// Start regular event workers
	for i := 0; i < p.config.WorkerCount; i++ {
		p.wg.Add(1)
		go p.eventWorker(i, p.eventQueue, false)
	}
	
	// Start critical event workers
	for i := 0; i < p.config.CriticalWorkers; i++ {
		p.wg.Add(1)
		go p.eventWorker(i, p.criticalQueue, true)
	}
	
	// Start batch processor
	p.wg.Add(1)
	go p.batchProcessor()
}

func (p *AuditEventPublisher) eventWorker(id int, queue <-chan *audit.Event, isCritical bool) {
	defer p.wg.Done()
	
	workerType := "regular"
	if isCritical {
		workerType = "critical"
	}
	
	p.logger.Info("Event worker started",
		zap.Int("worker_id", id),
		zap.String("type", workerType),
	)
	
	for {
		select {
		case event := <-queue:
			p.processEvent(event)
			
		case <-p.ctx.Done():
			p.logger.Info("Event worker shutting down",
				zap.Int("worker_id", id),
				zap.String("type", workerType),
			)
			return
		}
	}
}

func (p *AuditEventPublisher) processEvent(event *audit.Event) {
	start := time.Now()
	ctx, span := p.tracer.Start(context.Background(), "processEvent",
		trace.WithAttributes(
			attribute.String("event.id", event.ID.String()),
			attribute.String("event.type", string(event.Type)),
		),
	)
	defer span.End()
	
	// Find matching subscriptions
	subscriptions := p.router.Route(event)
	if len(subscriptions) == 0 {
		p.recordEventFiltered("no_subscribers")
		return
	}
	
	// Group subscriptions by transport
	transportGroups := p.groupByTransport(subscriptions)
	
	// Send to each transport
	var wg sync.WaitGroup
	for transportType, subs := range transportGroups {
		transport, ok := p.transports[transportType]
		if !ok {
			continue
		}
		
		wg.Add(1)
		go func(t EventTransport, subscribers []*Subscription) {
			defer wg.Done()
			
			subIDs := make([]string, len(subscribers))
			for i, sub := range subscribers {
				subIDs[i] = sub.ID
			}
			
			sendCtx, cancel := context.WithTimeout(ctx, p.config.SendTimeout)
			defer cancel()
			
			if err := t.Send(sendCtx, event, subIDs); err != nil {
				p.logger.Error("Failed to send event",
					zap.String("transport", string(t.GetProtocol())),
					zap.Error(err),
					zap.String("event_id", event.ID.String()),
				)
				p.recordEventFailed(string(t.GetProtocol()))
				
				// Update delivery stats for failed subscribers
				for _, sub := range subscribers {
					p.updateDeliveryStats(sub, false, 0)
				}
			} else {
				// Update delivery stats for successful subscribers
				latency := time.Since(start)
				for _, sub := range subscribers {
					p.updateDeliveryStats(sub, true, latency)
				}
			}
		}(transport, subs)
	}
	
	wg.Wait()
	
	p.recordEventPublished(time.Since(start))
}

func (p *AuditEventPublisher) batchProcessor() {
	defer p.wg.Done()
	
	ticker := time.NewTicker(p.config.BatchTimeout)
	defer ticker.Stop()
	
	batch := make([]*audit.Event, 0, p.config.BatchSize)
	
	for {
		select {
		case event := <-p.eventQueue:
			batch = append(batch, event)
			
			if len(batch) >= p.config.BatchSize {
				p.processBatch(batch)
				batch = make([]*audit.Event, 0, p.config.BatchSize)
			}
			
		case <-ticker.C:
			if len(batch) > 0 {
				p.processBatch(batch)
				batch = make([]*audit.Event, 0, p.config.BatchSize)
			}
			
		case <-p.ctx.Done():
			// Process remaining batch
			if len(batch) > 0 {
				p.processBatch(batch)
			}
			return
		}
	}
}

func (p *AuditEventPublisher) processBatch(events []*audit.Event) {
	if len(events) == 0 {
		return
	}
	
	ctx, span := p.tracer.Start(context.Background(), "processBatch",
		trace.WithAttributes(
			attribute.Int("batch.size", len(events)),
		),
	)
	defer span.End()
	
	// Route all events and group by transport
	transportBatches := make(map[TransportType]map[string][]*audit.Event)
	
	for _, event := range events {
		subscriptions := p.router.Route(event)
		
		for _, sub := range subscriptions {
			if _, ok := transportBatches[sub.Transport]; !ok {
				transportBatches[sub.Transport] = make(map[string][]*audit.Event)
			}
			transportBatches[sub.Transport][sub.ID] = append(
				transportBatches[sub.Transport][sub.ID], event)
		}
	}
	
	// Send batches to each transport
	for transportType, subBatches := range transportBatches {
		transport, ok := p.transports[transportType]
		if !ok {
			continue
		}
		
		// Flatten events for batch send
		allEvents := make([]*audit.Event, 0)
		allSubscribers := make([]string, 0)
		
		for subID, events := range subBatches {
			allEvents = append(allEvents, events...)
			for range events {
				allSubscribers = append(allSubscribers, subID)
			}
		}
		
		sendCtx, cancel := context.WithTimeout(ctx, p.config.SendTimeout)
		err := transport.SendBatch(sendCtx, allEvents, allSubscribers)
		cancel()
		
		if err != nil {
			p.logger.Error("Failed to send batch",
				zap.String("transport", string(transportType)),
				zap.Error(err),
				zap.Int("event_count", len(allEvents)),
			)
		}
	}
	
	p.recordBatchProcessed(len(events))
}

func (p *AuditEventPublisher) groupByTransport(subscriptions []*Subscription) map[TransportType][]*Subscription {
	groups := make(map[TransportType][]*Subscription)
	
	for _, sub := range subscriptions {
		groups[sub.Transport] = append(groups[sub.Transport], sub)
	}
	
	return groups
}

func (p *AuditEventPublisher) updateDeliveryStats(sub *Subscription, success bool, latency time.Duration) {
	sub.DeliveryStats.mu.Lock()
	defer sub.DeliveryStats.mu.Unlock()
	
	if success {
		sub.DeliveryStats.EventsSent++
		sub.DeliveryStats.LastSuccess = time.Now()
		
		// Update average latency (simple moving average)
		if sub.DeliveryStats.AvgLatency == 0 {
			sub.DeliveryStats.AvgLatency = latency
		} else {
			sub.DeliveryStats.AvgLatency = (sub.DeliveryStats.AvgLatency + latency) / 2
		}
	} else {
		sub.DeliveryStats.EventsFailed++
		sub.DeliveryStats.LastFailure = time.Now()
	}
	
	sub.LastEventAt = time.Now()
}

func (p *AuditEventPublisher) countFilters(filters EventFilters) int {
	count := len(filters.EventTypes) + len(filters.EntityTypes) + 
		len(filters.EntityIDs) + len(filters.UserIDs) + len(filters.CustomFilters)
	
	if filters.TimeRange != nil {
		count++
	}
	
	return count
}

// Metric recording methods

func (p *AuditEventPublisher) initMetrics() error {
	meter := otel.Meter("audit.publisher")
	
	// Create counters
	eventsPublished, err := meter.Int64Counter("audit.events.published",
		metric.WithDescription("Total number of audit events published"))
	if err != nil {
		return err
	}
	p.metrics.eventsPublished = eventsPublished
	
	eventsFiltered, err := meter.Int64Counter("audit.events.filtered",
		metric.WithDescription("Total number of audit events filtered out"))
	if err != nil {
		return err
	}
	p.metrics.eventsFiltered = eventsFiltered
	
	eventsFailed, err := meter.Int64Counter("audit.events.failed",
		metric.WithDescription("Total number of audit events that failed to publish"))
	if err != nil {
		return err
	}
	p.metrics.eventsFailed = eventsFailed
	
	eventsDropped, err := meter.Int64Counter("audit.events.dropped",
		metric.WithDescription("Total number of audit events dropped"))
	if err != nil {
		return err
	}
	p.metrics.eventsDropped = eventsDropped
	
	// Create histograms
	publishLatency, err := meter.Float64Histogram("audit.publish.latency",
		metric.WithDescription("Latency of publishing audit events"),
		metric.WithUnit("ms"))
	if err != nil {
		return err
	}
	p.metrics.publishLatency = publishLatency
	
	batchSize, err := meter.Int64Histogram("audit.batch.size",
		metric.WithDescription("Size of event batches processed"))
	if err != nil {
		return err
	}
	p.metrics.batchSize = batchSize
	
	// Create observable gauges
	activeSubscriptions, err := meter.Int64ObservableGauge("audit.subscriptions.active",
		metric.WithDescription("Number of active audit event subscriptions"))
	if err != nil {
		return err
	}
	p.metrics.activeSubscriptions = activeSubscriptions
	
	queueSize, err := meter.Int64ObservableGauge("audit.queue.size",
		metric.WithDescription("Current size of the event queue"))
	if err != nil {
		return err
	}
	p.metrics.queueSize = queueSize
	
	// Register callbacks for observable gauges
	meter.RegisterCallback(func(ctx context.Context, observer metric.Observer) error {
		p.subscriptionsMu.RLock()
		subscriptionCount := int64(len(p.subscriptions))
		p.subscriptionsMu.RUnlock()
		
		observer.ObserveInt64(activeSubscriptions, subscriptionCount)
		observer.ObserveInt64(queueSize, int64(len(p.eventQueue)))
		
		return nil
	}, activeSubscriptions, queueSize)
	
	return nil
}

func (p *AuditEventPublisher) recordEventPublished(latency time.Duration) {
	p.metrics.eventsPublished.Add(context.Background(), 1)
	p.metrics.publishLatency.Record(context.Background(), float64(latency.Milliseconds()))
	
	p.metrics.stats.mu.Lock()
	p.metrics.stats.totalPublished++
	p.metrics.stats.mu.Unlock()
}

func (p *AuditEventPublisher) recordEventFiltered(reason string) {
	p.metrics.eventsFiltered.Add(context.Background(), 1,
		metric.WithAttributes(attribute.String("reason", reason)))
}

func (p *AuditEventPublisher) recordEventFailed(transport string) {
	p.metrics.eventsFailed.Add(context.Background(), 1,
		metric.WithAttributes(attribute.String("transport", transport)))
	
	p.metrics.stats.mu.Lock()
	p.metrics.stats.totalFailed++
	p.metrics.stats.mu.Unlock()
}

func (p *AuditEventPublisher) recordEventDropped(reason string) {
	p.metrics.eventsDropped.Add(context.Background(), 1,
		metric.WithAttributes(attribute.String("reason", reason)))
	
	p.metrics.stats.mu.Lock()
	p.metrics.stats.totalDropped++
	p.metrics.stats.mu.Unlock()
}

func (p *AuditEventPublisher) recordBatchProcessed(size int) {
	p.metrics.batchSize.Record(context.Background(), int64(size))
}

func (p *AuditEventPublisher) recordSubscriptionChange(delta int) {
	// Subscription count is tracked via observable gauge
}

func (p *AuditEventPublisher) updateQueueMetrics() {
	p.metrics.stats.mu.Lock()
	p.metrics.stats.currentQueueSize = int64(len(p.eventQueue))
	p.metrics.stats.mu.Unlock()
}