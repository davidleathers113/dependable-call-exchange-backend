package events_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/audit"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/infrastructure/events"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestAuditEventPublisher_Publish(t *testing.T) {
	ctx := context.Background()
	logger, _ := zap.NewDevelopment()
	
	// Create mock transport
	mockTransport := NewMockEventTransport()
	transports := map[events.TransportType]events.EventTransport{
		events.TransportWebSocket: mockTransport,
	}
	
	// Create publisher
	config := events.DefaultPublisherConfig()
	config.WorkerCount = 2
	config.EventQueueSize = 100
	
	publisher, err := events.NewAuditEventPublisher(ctx, logger, config, transports)
	require.NoError(t, err)
	defer publisher.Close()
	
	// Create test event
	event := &audit.Event{
		ID:         uuid.New(),
		Type:       audit.EventTypeCallCreated,
		Severity:   audit.SeverityInfo,
		Timestamp:  time.Now(),
		UserID:     uuid.New(),
		EntityType: "call",
		EntityID:   uuid.New(),
		Action:     "create",
		Result:     audit.ResultSuccess,
		Metadata: map[string]interface{}{
			"from_number": "+1234567890",
			"to_number":   "+0987654321",
		},
	}
	
	// Create subscription
	userID := uuid.New()
	filters := events.EventFilters{
		EventTypes: []audit.EventType{audit.EventTypeCallCreated},
	}
	
	subscription, err := publisher.Subscribe(ctx, userID, events.TransportWebSocket, filters, nil)
	require.NoError(t, err)
	
	// Publish event
	err = publisher.Publish(ctx, event)
	assert.NoError(t, err)
	
	// Wait for async processing
	time.Sleep(100 * time.Millisecond)
	
	// Verify event was sent
	sentEvents := mockTransport.GetSentEvents()
	assert.Len(t, sentEvents, 1)
	assert.Equal(t, event.ID, sentEvents[0].Event.ID)
	assert.Contains(t, sentEvents[0].Subscribers, subscription.ID)
	
	// Get metrics
	metrics := publisher.GetMetrics()
	assert.Equal(t, int64(1), metrics["events_published"])
	assert.Equal(t, int64(0), metrics["events_failed"])
	assert.Equal(t, 1, metrics["active_subscriptions"])
}

func TestAuditEventPublisher_Filtering(t *testing.T) {
	ctx := context.Background()
	logger, _ := zap.NewDevelopment()
	
	// Create mock transport
	mockTransport := NewMockEventTransport()
	transports := map[events.TransportType]events.EventTransport{
		events.TransportWebSocket: mockTransport,
	}
	
	// Create publisher
	config := events.DefaultPublisherConfig()
	publisher, err := events.NewAuditEventPublisher(ctx, logger, config, transports)
	require.NoError(t, err)
	defer publisher.Close()
	
	// Create subscriptions with different filters
	userID1 := uuid.New()
	filters1 := events.EventFilters{
		EventTypes: []audit.EventType{audit.EventTypeCallCreated},
		Severity:   []audit.Severity{audit.SeverityInfo},
	}
	sub1, err := publisher.Subscribe(ctx, userID1, events.TransportWebSocket, filters1, nil)
	require.NoError(t, err)
	
	userID2 := uuid.New()
	filters2 := events.EventFilters{
		EventTypes: []audit.EventType{audit.EventTypeBidCreated},
		Severity:   []audit.Severity{audit.SeverityHigh},
	}
	sub2, err := publisher.Subscribe(ctx, userID2, events.TransportWebSocket, filters2, nil)
	require.NoError(t, err)
	
	// Publish events that match different filters
	callEvent := &audit.Event{
		ID:         uuid.New(),
		Type:       audit.EventTypeCallCreated,
		Severity:   audit.SeverityInfo,
		Timestamp:  time.Now(),
		UserID:     uuid.New(),
		EntityType: "call",
		EntityID:   uuid.New(),
	}
	
	bidEvent := &audit.Event{
		ID:         uuid.New(),
		Type:       audit.EventTypeBidCreated,
		Severity:   audit.SeverityHigh,
		Timestamp:  time.Now(),
		UserID:     uuid.New(),
		EntityType: "bid",
		EntityID:   uuid.New(),
	}
	
	// Publish events
	err = publisher.Publish(ctx, callEvent)
	require.NoError(t, err)
	
	err = publisher.Publish(ctx, bidEvent)
	require.NoError(t, err)
	
	// Wait for async processing
	time.Sleep(100 * time.Millisecond)
	
	// Verify correct routing
	sentEvents := mockTransport.GetSentEvents()
	assert.Len(t, sentEvents, 2)
	
	// Check first event went to sub1
	callDelivery := findEventDelivery(sentEvents, callEvent.ID)
	require.NotNil(t, callDelivery)
	assert.Contains(t, callDelivery.Subscribers, sub1.ID)
	assert.NotContains(t, callDelivery.Subscribers, sub2.ID)
	
	// Check second event went to sub2
	bidDelivery := findEventDelivery(sentEvents, bidEvent.ID)
	require.NotNil(t, bidDelivery)
	assert.Contains(t, bidDelivery.Subscribers, sub2.ID)
	assert.NotContains(t, bidDelivery.Subscribers, sub1.ID)
}

func TestAuditEventPublisher_CriticalEvents(t *testing.T) {
	ctx := context.Background()
	logger, _ := zap.NewDevelopment()
	
	// Create mock transport
	mockTransport := NewMockEventTransport()
	transports := map[events.TransportType]events.EventTransport{
		events.TransportWebSocket: mockTransport,
	}
	
	// Create publisher with small queues to test prioritization
	config := events.DefaultPublisherConfig()
	config.EventQueueSize = 10
	config.CriticalQueueSize = 5
	config.WorkerCount = 1
	config.CriticalWorkers = 1
	
	publisher, err := events.NewAuditEventPublisher(ctx, logger, config, transports)
	require.NoError(t, err)
	defer publisher.Close()
	
	// Subscribe to all events
	userID := uuid.New()
	filters := events.EventFilters{} // No filters = match all
	_, err = publisher.Subscribe(ctx, userID, events.TransportWebSocket, filters, nil)
	require.NoError(t, err)
	
	// Publish mix of regular and critical events
	var publishedEvents []*audit.Event
	
	// Regular events
	for i := 0; i < 5; i++ {
		event := &audit.Event{
			ID:        uuid.New(),
			Type:      audit.EventTypeCallCreated,
			Severity:  audit.SeverityInfo,
			Timestamp: time.Now(),
			UserID:    uuid.New(),
		}
		publishedEvents = append(publishedEvents, event)
		err = publisher.Publish(ctx, event)
		require.NoError(t, err)
	}
	
	// Critical event (should be processed with priority)
	criticalEvent := &audit.Event{
		ID:        uuid.New(),
		Type:      audit.EventTypeSecurityBreach,
		Severity:  audit.SeverityCritical,
		Timestamp: time.Now(),
		UserID:    uuid.New(),
		Metadata: map[string]interface{}{
			"breach_type": "unauthorized_access",
			"ip_address":  "192.168.1.100",
		},
	}
	publishedEvents = append(publishedEvents, criticalEvent)
	err = publisher.Publish(ctx, criticalEvent)
	require.NoError(t, err)
	
	// Wait for processing
	time.Sleep(200 * time.Millisecond)
	
	// Verify all events were sent
	sentEvents := mockTransport.GetSentEvents()
	assert.Len(t, sentEvents, 6)
	
	// Verify critical event was included
	criticalDelivery := findEventDelivery(sentEvents, criticalEvent.ID)
	require.NotNil(t, criticalDelivery)
}

func TestAuditEventPublisher_Backpressure(t *testing.T) {
	ctx := context.Background()
	logger, _ := zap.NewDevelopment()
	
	// Create slow transport that simulates backpressure
	slowTransport := NewSlowMockTransport(50 * time.Millisecond)
	transports := map[events.TransportType]events.EventTransport{
		events.TransportWebSocket: slowTransport,
	}
	
	// Create publisher with small queue
	config := events.DefaultPublisherConfig()
	config.EventQueueSize = 5
	config.WorkerCount = 1
	config.MaxQueueDepth = 3
	config.BackpressureDelay = 10 * time.Millisecond
	
	publisher, err := events.NewAuditEventPublisher(ctx, logger, config, transports)
	require.NoError(t, err)
	defer publisher.Close()
	
	// Subscribe to all events
	userID := uuid.New()
	filters := events.EventFilters{}
	_, err = publisher.Subscribe(ctx, userID, events.TransportWebSocket, filters, nil)
	require.NoError(t, err)
	
	// Try to publish many events quickly
	errors := make([]error, 10)
	for i := 0; i < 10; i++ {
		event := &audit.Event{
			ID:        uuid.New(),
			Type:      audit.EventTypeCallCreated,
			Severity:  audit.SeverityInfo,
			Timestamp: time.Now(),
			UserID:    uuid.New(),
		}
		errors[i] = publisher.Publish(ctx, event)
	}
	
	// Some publishes should fail due to backpressure
	failCount := 0
	for _, err := range errors {
		if err != nil {
			failCount++
		}
	}
	
	assert.Greater(t, failCount, 0, "Expected some failures due to backpressure")
	
	// Check metrics
	metrics := publisher.GetMetrics()
	assert.Greater(t, metrics["events_dropped"], int64(0))
}

func TestAuditEventPublisher_MultipleTransports(t *testing.T) {
	ctx := context.Background()
	logger, _ := zap.NewDevelopment()
	
	// Create multiple mock transports
	wsTransport := NewMockEventTransport()
	kafkaTransport := NewMockEventTransport()
	
	transports := map[events.TransportType]events.EventTransport{
		events.TransportWebSocket: wsTransport,
		events.TransportKafka:     kafkaTransport,
	}
	
	// Create publisher
	config := events.DefaultPublisherConfig()
	publisher, err := events.NewAuditEventPublisher(ctx, logger, config, transports)
	require.NoError(t, err)
	defer publisher.Close()
	
	// Create subscriptions on different transports
	userID1 := uuid.New()
	filters := events.EventFilters{
		EventTypes: []audit.EventType{audit.EventTypeCallCreated},
	}
	
	wsSub, err := publisher.Subscribe(ctx, userID1, events.TransportWebSocket, filters, nil)
	require.NoError(t, err)
	
	userID2 := uuid.New()
	kafkaSub, err := publisher.Subscribe(ctx, userID2, events.TransportKafka, filters, nil)
	require.NoError(t, err)
	
	// Publish event
	event := &audit.Event{
		ID:         uuid.New(),
		Type:       audit.EventTypeCallCreated,
		Severity:   audit.SeverityInfo,
		Timestamp:  time.Now(),
		UserID:     uuid.New(),
		EntityType: "call",
		EntityID:   uuid.New(),
	}
	
	err = publisher.Publish(ctx, event)
	require.NoError(t, err)
	
	// Wait for processing
	time.Sleep(100 * time.Millisecond)
	
	// Verify event was sent to both transports
	wsEvents := wsTransport.GetSentEvents()
	assert.Len(t, wsEvents, 1)
	assert.Contains(t, wsEvents[0].Subscribers, wsSub.ID)
	
	kafkaEvents := kafkaTransport.GetSentEvents()
	assert.Len(t, kafkaEvents, 1)
	assert.Contains(t, kafkaEvents[0].Subscribers, kafkaSub.ID)
}

// Mock implementations for testing

type MockEventTransport struct {
	sentEvents []EventDelivery
	mu         sync.Mutex
	healthy    bool
}

type EventDelivery struct {
	Event       *audit.Event
	Subscribers []string
	Timestamp   time.Time
}

func NewMockEventTransport() *MockEventTransport {
	return &MockEventTransport{
		sentEvents: make([]EventDelivery, 0),
		healthy:    true,
	}
}

func (m *MockEventTransport) Send(ctx context.Context, event *audit.Event, subscribers []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.sentEvents = append(m.sentEvents, EventDelivery{
		Event:       event,
		Subscribers: subscribers,
		Timestamp:   time.Now(),
	})
	
	return nil
}

func (m *MockEventTransport) SendBatch(ctx context.Context, events []*audit.Event, subscribers []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	for _, event := range events {
		m.sentEvents = append(m.sentEvents, EventDelivery{
			Event:       event,
			Subscribers: subscribers,
			Timestamp:   time.Now(),
		})
	}
	
	return nil
}

func (m *MockEventTransport) GetProtocol() events.TransportType {
	return events.TransportWebSocket
}

func (m *MockEventTransport) IsHealthy() bool {
	return m.healthy
}

func (m *MockEventTransport) Close() error {
	return nil
}

func (m *MockEventTransport) GetSentEvents() []EventDelivery {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	result := make([]EventDelivery, len(m.sentEvents))
	copy(result, m.sentEvents)
	return result
}

// SlowMockTransport simulates a slow transport for testing backpressure
type SlowMockTransport struct {
	*MockEventTransport
	delay time.Duration
}

func NewSlowMockTransport(delay time.Duration) *SlowMockTransport {
	return &SlowMockTransport{
		MockEventTransport: NewMockEventTransport(),
		delay:              delay,
	}
}

func (s *SlowMockTransport) Send(ctx context.Context, event *audit.Event, subscribers []string) error {
	time.Sleep(s.delay)
	return s.MockEventTransport.Send(ctx, event, subscribers)
}

// Helper functions

func findEventDelivery(deliveries []EventDelivery, eventID uuid.UUID) *EventDelivery {
	for _, d := range deliveries {
		if d.Event.ID == eventID {
			return &d
		}
	}
	return nil
}