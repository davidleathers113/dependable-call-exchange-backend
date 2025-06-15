package events

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/infrastructure/config"
	"go.uber.org/zap"
)

// Factory creates audit event publishers with configured transports
type Factory struct {
	logger *zap.Logger
	config *config.Config
}

// NewFactory creates a new audit publisher factory
func NewFactory(logger *zap.Logger, config *config.Config) *Factory {
	return &Factory{
		logger: logger,
		config: config,
	}
}

// CreateAuditPublisher creates a fully configured audit event publisher
func (f *Factory) CreateAuditPublisher(ctx context.Context) (*AuditEventPublisher, error) {
	// Create publisher configuration
	publisherConfig := f.createPublisherConfig()
	
	// Create transports
	transports, err := f.createTransports()
	if err != nil {
		return nil, fmt.Errorf("failed to create transports: %w", err)
	}
	
	if len(transports) == 0 {
		return nil, fmt.Errorf("no transports configured")
	}
	
	// Create publisher
	publisher, err := NewAuditEventPublisher(ctx, f.logger, publisherConfig, transports)
	if err != nil {
		// Close any created transports
		for _, transport := range transports {
			transport.Close()
		}
		return nil, fmt.Errorf("failed to create audit publisher: %w", err)
	}
	
	f.logger.Info("Audit event publisher created",
		zap.Int("transport_count", len(transports)),
		zap.String("transports", f.getTransportNames(transports)),
	)
	
	return publisher, nil
}

// CreateWebSocketTransport creates a WebSocket transport
func (f *Factory) CreateWebSocketTransport() (*WebSocketTransport, error) {
	wsConfig := f.createWebSocketConfig()
	return NewWebSocketTransport(f.logger, wsConfig), nil
}

// CreateKafkaTransport creates a Kafka transport
func (f *Factory) CreateKafkaTransport() (*KafkaTransport, error) {
	// Check if Kafka is configured
	if !f.config.IsSet("kafka.enabled") || !f.config.GetBool("kafka.enabled") {
		return nil, nil
	}
	
	kafkaConfig := f.createKafkaConfig()
	
	// Create Kafka producer
	producer, err := f.createKafkaProducer(kafkaConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kafka producer: %w", err)
	}
	
	return NewKafkaTransport(f.logger, producer, kafkaConfig), nil
}

// Private methods

func (f *Factory) createPublisherConfig() PublisherConfig {
	config := DefaultPublisherConfig()
	
	// Override with config values if available
	if f.config.IsSet("audit.publisher.event_queue_size") {
		config.EventQueueSize = f.config.GetInt("audit.publisher.event_queue_size")
	}
	
	if f.config.IsSet("audit.publisher.critical_queue_size") {
		config.CriticalQueueSize = f.config.GetInt("audit.publisher.critical_queue_size")
	}
	
	if f.config.IsSet("audit.publisher.worker_count") {
		config.WorkerCount = f.config.GetInt("audit.publisher.worker_count")
	}
	
	if f.config.IsSet("audit.publisher.critical_workers") {
		config.CriticalWorkers = f.config.GetInt("audit.publisher.critical_workers")
	}
	
	if f.config.IsSet("audit.publisher.batch_size") {
		config.BatchSize = f.config.GetInt("audit.publisher.batch_size")
	}
	
	if f.config.IsSet("audit.publisher.batch_timeout") {
		config.BatchTimeout = f.config.GetDuration("audit.publisher.batch_timeout")
	}
	
	if f.config.IsSet("audit.publisher.max_retries") {
		config.MaxRetries = f.config.GetInt("audit.publisher.max_retries")
	}
	
	if f.config.IsSet("audit.publisher.retry_delay") {
		config.RetryDelay = f.config.GetDuration("audit.publisher.retry_delay")
	}
	
	if f.config.IsSet("audit.publisher.retry_backoff") {
		config.RetryBackoff = f.config.GetFloat64("audit.publisher.retry_backoff")
	}
	
	if f.config.IsSet("audit.publisher.max_queue_depth") {
		config.MaxQueueDepth = f.config.GetInt("audit.publisher.max_queue_depth")
	}
	
	if f.config.IsSet("audit.publisher.backpressure_delay") {
		config.BackpressureDelay = f.config.GetDuration("audit.publisher.backpressure_delay")
	}
	
	if f.config.IsSet("audit.publisher.send_timeout") {
		config.SendTimeout = f.config.GetDuration("audit.publisher.send_timeout")
	}
	
	if f.config.IsSet("audit.publisher.shutdown_timeout") {
		config.ShutdownTimeout = f.config.GetDuration("audit.publisher.shutdown_timeout")
	}
	
	return config
}

func (f *Factory) createWebSocketConfig() WebSocketConfig {
	config := DefaultWebSocketConfig()
	
	if f.config.IsSet("websocket.write_timeout") {
		config.WriteTimeout = f.config.GetDuration("websocket.write_timeout")
	}
	
	if f.config.IsSet("websocket.ping_interval") {
		config.PingInterval = f.config.GetDuration("websocket.ping_interval")
	}
	
	if f.config.IsSet("websocket.pong_timeout") {
		config.PongTimeout = f.config.GetDuration("websocket.pong_timeout")
	}
	
	if f.config.IsSet("websocket.max_message_size") {
		config.MaxMessageSize = int64(f.config.GetInt("websocket.max_message_size"))
	}
	
	if f.config.IsSet("websocket.send_buffer_size") {
		config.SendBufferSize = f.config.GetInt("websocket.send_buffer_size")
	}
	
	return config
}

func (f *Factory) createKafkaConfig() KafkaConfig {
	config := DefaultKafkaConfig()
	
	if f.config.IsSet("kafka.brokers") {
		config.Brokers = f.config.GetStringSlice("kafka.brokers")
	}
	
	if f.config.IsSet("kafka.topic") {
		config.Topic = f.config.GetString("kafka.topic")
	}
	
	if f.config.IsSet("kafka.compression_type") {
		config.CompressionType = f.config.GetString("kafka.compression_type")
	}
	
	if f.config.IsSet("kafka.batch_size") {
		config.BatchSize = f.config.GetInt("kafka.batch_size")
	}
	
	if f.config.IsSet("kafka.linger_ms") {
		config.LingerMs = f.config.GetInt("kafka.linger_ms")
	}
	
	if f.config.IsSet("kafka.retry_max") {
		config.RetryMax = f.config.GetInt("kafka.retry_max")
	}
	
	if f.config.IsSet("kafka.required_acks") {
		config.RequiredAcks = f.config.GetInt("kafka.required_acks")
	}
	
	if f.config.IsSet("kafka.idempotent_writes") {
		config.IdempotentWrites = f.config.GetBool("kafka.idempotent_writes")
	}
	
	if f.config.IsSet("kafka.enable_topic_routing") {
		config.EnableTopicRouting = f.config.GetBool("kafka.enable_topic_routing")
	}
	
	if f.config.IsSet("kafka.topic_prefix") {
		config.TopicPrefix = f.config.GetString("kafka.topic_prefix")
	}
	
	// Security settings
	if f.config.IsSet("kafka.enable_tls") {
		config.EnableTLS = f.config.GetBool("kafka.enable_tls")
	}
	
	if f.config.IsSet("kafka.enable_sasl") {
		config.EnableSASL = f.config.GetBool("kafka.enable_sasl")
	}
	
	if f.config.IsSet("kafka.sasl_mechanism") {
		config.SASLMechanism = f.config.GetString("kafka.sasl_mechanism")
	}
	
	if f.config.IsSet("kafka.sasl_username") {
		config.SASLUsername = f.config.GetString("kafka.sasl_username")
	}
	
	if f.config.IsSet("kafka.sasl_password") {
		config.SASLPassword = f.config.GetString("kafka.sasl_password")
	}
	
	return config
}

func (f *Factory) createTransports() (map[TransportType]EventTransport, error) {
	transports := make(map[TransportType]EventTransport)
	
	// Always include WebSocket transport
	wsTransport, err := f.CreateWebSocketTransport()
	if err != nil {
		return nil, fmt.Errorf("failed to create WebSocket transport: %w", err)
	}
	transports[TransportWebSocket] = wsTransport
	
	// Add Kafka transport if configured
	kafkaTransport, err := f.CreateKafkaTransport()
	if err != nil {
		return nil, fmt.Errorf("failed to create Kafka transport: %w", err)
	}
	if kafkaTransport != nil {
		transports[TransportKafka] = kafkaTransport
	}
	
	// TODO: Add other transports (gRPC, HTTP) as needed
	
	return transports, nil
}

func (f *Factory) createKafkaProducer(config KafkaConfig) (KafkaProducer, error) {
	// In a real implementation, this would create an actual Kafka producer
	// For now, return a mock producer
	return NewMockKafkaProducer(), nil
}

func (f *Factory) getTransportNames(transports map[TransportType]EventTransport) string {
	names := ""
	for transportType := range transports {
		if names != "" {
			names += ", "
		}
		names += string(transportType)
	}
	return names
}

// MockKafkaProducer is a mock implementation for testing
type MockKafkaProducer struct {
	messages []ProducerMessage
	mu       sync.Mutex
	closed   bool
}

func NewMockKafkaProducer() *MockKafkaProducer {
	return &MockKafkaProducer{
		messages: make([]ProducerMessage, 0),
	}
}

func (p *MockKafkaProducer) SendMessage(topic string, key []byte, value []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	if p.closed {
		return fmt.Errorf("producer is closed")
	}
	
	p.messages = append(p.messages, ProducerMessage{
		Topic:     topic,
		Key:       key,
		Value:     value,
		Timestamp: time.Now(),
	})
	
	return nil
}

func (p *MockKafkaProducer) SendBatch(messages []ProducerMessage) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	if p.closed {
		return fmt.Errorf("producer is closed")
	}
	
	p.messages = append(p.messages, messages...)
	return nil
}

func (p *MockKafkaProducer) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	p.closed = true
	return nil
}

func (p *MockKafkaProducer) GetMessages() []ProducerMessage {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	result := make([]ProducerMessage, len(p.messages))
	copy(result, p.messages)
	return result
}