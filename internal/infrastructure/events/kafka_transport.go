package events

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/audit"
	"go.uber.org/zap"
)

// KafkaTransport implements EventTransport for Apache Kafka
type KafkaTransport struct {
	logger      *zap.Logger
	producer    KafkaProducer
	config      KafkaConfig
	
	// Topic management
	topicRouter *TopicRouter
	
	// Health tracking
	healthCheck *HealthChecker
	
	// Metrics
	metrics     *KafkaMetrics
}

// KafkaProducer interface for Kafka producer operations
type KafkaProducer interface {
	SendMessage(topic string, key []byte, value []byte) error
	SendBatch(messages []ProducerMessage) error
	Close() error
}

// ProducerMessage represents a Kafka message
type ProducerMessage struct {
	Topic     string
	Key       []byte
	Value     []byte
	Headers   map[string][]byte
	Timestamp time.Time
}

// KafkaConfig configures the Kafka transport
type KafkaConfig struct {
	Brokers          []string
	Topic            string
	CompressionType  string
	BatchSize        int
	LingerMs         int
	RetryMax         int
	RequiredAcks     int
	IdempotentWrites bool
	
	// Topic routing
	EnableTopicRouting bool
	TopicPrefix        string
	
	// Security
	EnableTLS        bool
	EnableSASL       bool
	SASLMechanism    string
	SASLUsername     string
	SASLPassword     string
}

// DefaultKafkaConfig returns default Kafka configuration
func DefaultKafkaConfig() KafkaConfig {
	return KafkaConfig{
		Brokers:            []string{"localhost:9092"},
		Topic:              "audit-events",
		CompressionType:    "snappy",
		BatchSize:          100,
		LingerMs:           10,
		RetryMax:           3,
		RequiredAcks:       1,
		IdempotentWrites:   true,
		EnableTopicRouting: true,
		TopicPrefix:        "audit",
	}
}

// KafkaMetrics tracks Kafka transport metrics
type KafkaMetrics struct {
	MessagesSent     int64
	MessagesFailed   int64
	BatchesSent      int64
	BatchesFailed    int64
	AvgBatchSize     float64
	AvgLatency       time.Duration
	mu               sync.RWMutex
}

// TopicRouter routes events to different Kafka topics
type TopicRouter struct {
	prefix     string
	severityMap map[audit.Severity]string
	typeMap     map[audit.EventType]string
}

// NewKafkaTransport creates a new Kafka transport
func NewKafkaTransport(logger *zap.Logger, producer KafkaProducer, config KafkaConfig) *KafkaTransport {
	transport := &KafkaTransport{
		logger:      logger,
		producer:    producer,
		config:      config,
		healthCheck: NewHealthChecker(5 * time.Minute),
		metrics:     &KafkaMetrics{},
	}
	
	if config.EnableTopicRouting {
		transport.topicRouter = NewTopicRouter(config.TopicPrefix)
	}
	
	return transport
}

// Send sends a single event to Kafka
func (t *KafkaTransport) Send(ctx context.Context, event *audit.Event, subscribers []string) error {
	// Determine topic
	topic := t.config.Topic
	if t.config.EnableTopicRouting && t.topicRouter != nil {
		topic = t.topicRouter.GetTopic(event)
	}
	
	// Create Kafka message
	message, err := t.createMessage(event, subscribers)
	if err != nil {
		return fmt.Errorf("failed to create message: %w", err)
	}
	
	// Send with timeout
	sendCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	
	done := make(chan error, 1)
	start := time.Now()
	
	go func() {
		done <- t.producer.SendMessage(topic, message.Key, message.Value)
	}()
	
	select {
	case err := <-done:
		if err != nil {
			t.recordFailure()
			t.healthCheck.RecordFailure()
			return fmt.Errorf("failed to send to Kafka: %w", err)
		}
		
		t.recordSuccess(time.Since(start))
		t.healthCheck.RecordSuccess()
		return nil
		
	case <-sendCtx.Done():
		t.recordFailure()
		return sendCtx.Err()
	}
}

// SendBatch sends multiple events to Kafka
func (t *KafkaTransport) SendBatch(ctx context.Context, events []*audit.Event, subscribers []string) error {
	if len(events) == 0 {
		return nil
	}
	
	// Create messages
	messages := make([]ProducerMessage, 0, len(events))
	
	for i, event := range events {
		// Get subscribers for this specific event
		eventSubs := []string{}
		if i < len(subscribers) {
			eventSubs = append(eventSubs, subscribers[i])
		}
		
		msg, err := t.createMessage(event, eventSubs)
		if err != nil {
			t.logger.Error("Failed to create message for batch",
				zap.Error(err),
				zap.String("event_id", event.ID.String()),
			)
			continue
		}
		
		// Determine topic for each event
		topic := t.config.Topic
		if t.config.EnableTopicRouting && t.topicRouter != nil {
			topic = t.topicRouter.GetTopic(event)
		}
		
		messages = append(messages, ProducerMessage{
			Topic:     topic,
			Key:       msg.Key,
			Value:     msg.Value,
			Headers:   msg.Headers,
			Timestamp: event.Timestamp,
		})
	}
	
	if len(messages) == 0 {
		return fmt.Errorf("no valid messages to send")
	}
	
	// Send batch with timeout
	sendCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	
	done := make(chan error, 1)
	start := time.Now()
	
	go func() {
		done <- t.producer.SendBatch(messages)
	}()
	
	select {
	case err := <-done:
		if err != nil {
			t.recordBatchFailure(len(messages))
			t.healthCheck.RecordFailure()
			return fmt.Errorf("failed to send batch to Kafka: %w", err)
		}
		
		t.recordBatchSuccess(len(messages), time.Since(start))
		t.healthCheck.RecordSuccess()
		return nil
		
	case <-sendCtx.Done():
		t.recordBatchFailure(len(messages))
		return sendCtx.Err()
	}
}

// GetProtocol returns the transport protocol type
func (t *KafkaTransport) GetProtocol() TransportType {
	return TransportKafka
}

// IsHealthy checks if the transport is healthy
func (t *KafkaTransport) IsHealthy() bool {
	return t.healthCheck.IsHealthy()
}

// Close gracefully shuts down the transport
func (t *KafkaTransport) Close() error {
	t.logger.Info("Closing Kafka transport")
	
	if t.producer != nil {
		return t.producer.Close()
	}
	
	return nil
}

// GetMetrics returns current Kafka metrics
func (t *KafkaTransport) GetMetrics() map[string]interface{} {
	t.metrics.mu.RLock()
	defer t.metrics.mu.RUnlock()
	
	return map[string]interface{}{
		"messages_sent":    t.metrics.MessagesSent,
		"messages_failed":  t.metrics.MessagesFailed,
		"batches_sent":     t.metrics.BatchesSent,
		"batches_failed":   t.metrics.BatchesFailed,
		"avg_batch_size":   t.metrics.AvgBatchSize,
		"avg_latency_ms":   t.metrics.AvgLatency.Milliseconds(),
	}
}

// Private methods

func (t *KafkaTransport) createMessage(event *audit.Event, subscribers []string) (*KafkaMessage, error) {
	// Create event envelope
	envelope := &AuditEventEnvelope{
		Event:       event,
		Subscribers: subscribers,
		Metadata: map[string]string{
			"transport":   "kafka",
			"producer_id": t.getProducerID(),
			"timestamp":   time.Now().Format(time.RFC3339),
		},
	}
	
	// Marshal to JSON
	value, err := json.Marshal(envelope)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal event: %w", err)
	}
	
	// Create key for partitioning
	key := t.createKey(event)
	
	// Create headers
	headers := map[string][]byte{
		"event_id":   []byte(event.ID.String()),
		"event_type": []byte(event.Type),
		"severity":   []byte(event.Severity),
		"user_id":    []byte(event.UserID.String()),
		"entity_id":  []byte(event.EntityID.String()),
	}
	
	return &KafkaMessage{
		Key:     key,
		Value:   value,
		Headers: headers,
	}, nil
}

func (t *KafkaTransport) createKey(event *audit.Event) []byte {
	// Use entity ID for partitioning to ensure ordered delivery per entity
	if event.EntityID != nil {
		return []byte(event.EntityID.String())
	}
	
	// Fallback to user ID
	if event.UserID != nil {
		return []byte(event.UserID.String())
	}
	
	// Fallback to event ID
	return []byte(event.ID.String())
}

func (t *KafkaTransport) getProducerID() string {
	// In a real implementation, this would be a unique producer ID
	return "audit-publisher-1"
}

func (t *KafkaTransport) recordSuccess(latency time.Duration) {
	t.metrics.mu.Lock()
	defer t.metrics.mu.Unlock()
	
	t.metrics.MessagesSent++
	
	// Update average latency
	if t.metrics.AvgLatency == 0 {
		t.metrics.AvgLatency = latency
	} else {
		t.metrics.AvgLatency = (t.metrics.AvgLatency + latency) / 2
	}
}

func (t *KafkaTransport) recordFailure() {
	t.metrics.mu.Lock()
	defer t.metrics.mu.Unlock()
	
	t.metrics.MessagesFailed++
}

func (t *KafkaTransport) recordBatchSuccess(size int, latency time.Duration) {
	t.metrics.mu.Lock()
	defer t.metrics.mu.Unlock()
	
	t.metrics.BatchesSent++
	t.metrics.MessagesSent += int64(size)
	
	// Update average batch size
	if t.metrics.AvgBatchSize == 0 {
		t.metrics.AvgBatchSize = float64(size)
	} else {
		t.metrics.AvgBatchSize = (t.metrics.AvgBatchSize + float64(size)) / 2
	}
	
	// Update average latency
	if t.metrics.AvgLatency == 0 {
		t.metrics.AvgLatency = latency
	} else {
		t.metrics.AvgLatency = (t.metrics.AvgLatency + latency) / 2
	}
}

func (t *KafkaTransport) recordBatchFailure(size int) {
	t.metrics.mu.Lock()
	defer t.metrics.mu.Unlock()
	
	t.metrics.BatchesFailed++
	t.metrics.MessagesFailed += int64(size)
}

// KafkaMessage represents a Kafka message with metadata
type KafkaMessage struct {
	Key     []byte
	Value   []byte
	Headers map[string][]byte
}

// AuditEventEnvelope wraps an audit event with metadata
type AuditEventEnvelope struct {
	Event       *audit.Event      `json:"event"`
	Subscribers []string          `json:"subscribers,omitempty"`
	Metadata    map[string]string `json:"metadata"`
}

// NewTopicRouter creates a new topic router
func NewTopicRouter(prefix string) *TopicRouter {
	return &TopicRouter{
		prefix: prefix,
		severityMap: map[audit.Severity]string{
			audit.SeverityCritical: "critical",
			audit.SeverityHigh:     "high",
			audit.SeverityMedium:   "medium",
			audit.SeverityLow:      "low",
			audit.SeverityInfo:     "info",
		},
		typeMap: map[audit.EventType]string{
			// Call events
			audit.EventTypeCallCreated:    "calls",
			audit.EventTypeCallRouted:     "calls",
			audit.EventTypeCallConnected:  "calls",
			audit.EventTypeCallCompleted:  "calls",
			audit.EventTypeCallFailed:     "calls",
			
			// Bid events
			audit.EventTypeBidCreated:     "bids",
			audit.EventTypeBidAccepted:    "bids",
			audit.EventTypeBidRejected:    "bids",
			audit.EventTypeBidExpired:     "bids",
			
			// Financial events
			audit.EventTypePaymentProcessed: "financial",
			audit.EventTypeRefundIssued:     "financial",
			audit.EventTypeInvoiceGenerated: "financial",
			
			// Compliance events
			audit.EventTypeComplianceViolation: "compliance",
			audit.EventTypeConsentGranted:      "compliance",
			audit.EventTypeConsentRevoked:      "compliance",
			
			// Security events
			audit.EventTypeLoginSuccess:        "security",
			audit.EventTypeLoginFailed:         "security",
			audit.EventTypePermissionDenied:    "security",
			audit.EventTypeSuspiciousActivity:  "security",
		},
	}
}

// GetTopic returns the appropriate Kafka topic for an event
func (r *TopicRouter) GetTopic(event *audit.Event) string {
	// Route by severity for critical events
	if event.Severity == audit.SeverityCritical {
		return fmt.Sprintf("%s.%s", r.prefix, r.severityMap[event.Severity])
	}
	
	// Route by event type
	if category, ok := r.typeMap[event.Type]; ok {
		return fmt.Sprintf("%s.%s", r.prefix, category)
	}
	
	// Default topic
	return fmt.Sprintf("%s.events", r.prefix)
}