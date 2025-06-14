package audit

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestNewAuditEventPublisher(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockLogger := &mockAuditLogger{}
	config := DefaultPublisherConfig()

	publisher := NewAuditEventPublisher(logger, mockLogger, config)
	defer publisher.Close()

	assert.NotNil(t, publisher)
	assert.Equal(t, config.WorkerCount, cap(publisher.workerPool))
	assert.Equal(t, config.QueueSize, cap(publisher.eventQueue))
}

func TestAuditEventPublisher_Publish(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockLogger := &mockAuditLogger{}
	config := DefaultPublisherConfig()

	publisher := NewAuditEventPublisher(logger, mockLogger, config)
	defer publisher.Close()

	// Create a test event
	event := NewCallInitiatedEvent("user123", uuid.New(), "+1234567890", "+9876543210")

	// Publish the event
	ctx := context.Background()
	err := publisher.Publish(ctx, event)
	require.NoError(t, err)

	// Verify the event was logged
	events := mockLogger.GetEvents()
	assert.Len(t, events, 1)
	
	loggedEvent := events[0]
	assert.Equal(t, EventCallInitiated, loggedEvent.Type)
	assert.Equal(t, event.GetActorID(), loggedEvent.ActorID)
	assert.Equal(t, event.GetTargetID(), loggedEvent.TargetID)
	assert.Equal(t, event.GetAction(), loggedEvent.Action)

	// Check metrics
	metrics := publisher.GetMetrics()
	assert.Equal(t, int64(1), metrics.EventsPublished)
	assert.Equal(t, int64(0), metrics.EventsFailed)
}

func TestAuditEventPublisher_PublishBatch(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockLogger := &mockAuditLogger{}
	config := DefaultPublisherConfig()

	publisher := NewAuditEventPublisher(logger, mockLogger, config)
	defer publisher.Close()

	// Create test events
	events := []DomainEvent{
		NewCallInitiatedEvent("user123", uuid.New(), "+1234567890", "+9876543210"),
		NewBidPlacedEvent("user456", uuid.New(), uuid.New(), uuid.New(), uuid.New(), "5.00"),
		NewConsentGrantedEvent("user789", uuid.New(), uuid.New(), "marketing", "email"),
	}

	// Publish batch
	ctx := context.Background()
	err := publisher.PublishBatch(ctx, events)
	require.NoError(t, err)

	// Verify all events were logged
	loggedEvents := mockLogger.GetEvents()
	assert.Len(t, loggedEvents, 3)

	// Check event types
	eventTypes := make([]EventType, len(loggedEvents))
	for i, event := range loggedEvents {
		eventTypes[i] = event.Type
	}
	assert.Contains(t, eventTypes, EventCallInitiated)
	assert.Contains(t, eventTypes, EventBidPlaced)
	assert.Contains(t, eventTypes, EventConsentGranted)

	// Check metrics
	metrics := publisher.GetMetrics()
	assert.Equal(t, int64(1), metrics.BatchesPublished)
	assert.Equal(t, int64(0), metrics.BatchesFailed)
}

func TestAuditEventPublisher_PublishAsync(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockLogger := &mockAuditLogger{}
	config := DefaultPublisherConfig()
	config.BatchTimeout = 50 * time.Millisecond // Shorter timeout for testing

	publisher := NewAuditEventPublisher(logger, mockLogger, config)
	defer publisher.Close()

	// Create test event
	event := NewCallInitiatedEvent("user123", uuid.New(), "+1234567890", "+9876543210")

	// Publish async
	ctx := context.Background()
	publisher.PublishAsync(ctx, event)

	// Wait for async processing
	time.Sleep(100 * time.Millisecond)

	// Verify the event was processed
	events := mockLogger.GetEvents()
	assert.Len(t, events, 1)
	assert.Equal(t, EventCallInitiated, events[0].Type)
}

func TestAuditEventPublisher_Subscribe(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockLogger := &mockAuditLogger{}
	config := DefaultPublisherConfig()

	publisher := NewAuditEventPublisher(logger, mockLogger, config)
	defer publisher.Close()

	// Create a test handler
	var handledEvents []DomainEvent
	var mu sync.Mutex
	
	handler := EventHandlerFunc(func(ctx context.Context, event DomainEvent) error {
		mu.Lock()
		defer mu.Unlock()
		handledEvents = append(handledEvents, event)
		return nil
	})

	// Subscribe to events
	subscription := publisher.Subscribe(handler)
	assert.NotEmpty(t, subscription.ID())

	// Publish an event
	event := NewCallInitiatedEvent("user123", uuid.New(), "+1234567890", "+9876543210")
	ctx := context.Background()
	err := publisher.Publish(ctx, event)
	require.NoError(t, err)

	// Wait for handler processing
	time.Sleep(10 * time.Millisecond)

	// Verify handler was called
	mu.Lock()
	assert.Len(t, handledEvents, 1)
	assert.Equal(t, event.GetEventID(), handledEvents[0].GetEventID())
	mu.Unlock()

	// Unsubscribe
	err = subscription.Unsubscribe()
	assert.NoError(t, err)

	// Publish another event
	event2 := NewBidPlacedEvent("user456", uuid.New(), uuid.New(), uuid.New(), uuid.New(), "5.00")
	err = publisher.Publish(ctx, event2)
	require.NoError(t, err)

	// Wait and verify handler wasn't called again
	time.Sleep(10 * time.Millisecond)
	mu.Lock()
	assert.Len(t, handledEvents, 1) // Still only 1 event
	mu.Unlock()
}

func TestAuditEventPublisher_Health(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockLogger := &mockAuditLogger{}
	config := DefaultPublisherConfig()

	publisher := NewAuditEventPublisher(logger, mockLogger, config)
	defer publisher.Close()

	// Health should be OK initially
	err := publisher.Health()
	assert.NoError(t, err)
}

func TestAuditEventPublisher_Close(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockLogger := &mockAuditLogger{}
	config := DefaultPublisherConfig()
	config.ShutdownTimeout = 100 * time.Millisecond

	publisher := NewAuditEventPublisher(logger, mockLogger, config)

	// Publish some async events
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		event := NewCallInitiatedEvent("user123", uuid.New(), "+1234567890", "+9876543210")
		publisher.PublishAsync(ctx, event)
	}

	// Close should complete within timeout
	start := time.Now()
	err := publisher.Close()
	duration := time.Since(start)

	assert.NoError(t, err)
	assert.Less(t, duration, 2*config.ShutdownTimeout)

	// Events should have been processed before shutdown
	events := mockLogger.GetEvents()
	assert.GreaterOrEqual(t, len(events), 1) // At least some events should be processed
}

func TestAuditEventPublisher_ConcurrentPublish(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockLogger := &mockAuditLogger{}
	config := DefaultPublisherConfig()

	publisher := NewAuditEventPublisher(logger, mockLogger, config)
	defer publisher.Close()

	// Publish events concurrently
	const numGoroutines = 10
	const eventsPerGoroutine = 5
	
	var wg sync.WaitGroup
	ctx := context.Background()

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < eventsPerGoroutine; j++ {
				event := NewCallInitiatedEvent("user123", uuid.New(), "+1234567890", "+9876543210")
				err := publisher.Publish(ctx, event)
				assert.NoError(t, err)
			}
		}(i)
	}

	wg.Wait()

	// Verify all events were published
	events := mockLogger.GetEvents()
	assert.Len(t, events, numGoroutines*eventsPerGoroutine)

	// Check metrics
	metrics := publisher.GetMetrics()
	assert.Equal(t, int64(numGoroutines*eventsPerGoroutine), metrics.EventsPublished)
	assert.Equal(t, int64(0), metrics.EventsFailed)
}

func TestAuditEventPublisher_InvalidEvent(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	mockLogger := &mockAuditLogger{}
	config := DefaultPublisherConfig()

	publisher := NewAuditEventPublisher(logger, mockLogger, config)
	defer publisher.Close()

	// Create an invalid event (will fail conversion)
	event := &invalidDomainEvent{}

	// Publish should fail
	ctx := context.Background()
	err := publisher.Publish(ctx, event)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to convert domain event")

	// Check metrics
	metrics := publisher.GetMetrics()
	assert.Equal(t, int64(0), metrics.EventsPublished)
	assert.Equal(t, int64(1), metrics.EventsFailed)
}

func TestDefaultPublisherConfig(t *testing.T) {
	config := DefaultPublisherConfig()

	assert.Equal(t, 5, config.WorkerCount)
	assert.Equal(t, 1000, config.QueueSize)
	assert.Equal(t, 10, config.BatchSize)
	assert.Equal(t, 100*time.Millisecond, config.BatchTimeout)
	assert.Equal(t, 3, config.MaxRetries)
	assert.Equal(t, 100*time.Millisecond, config.RetryDelay)
	assert.Equal(t, 2.0, config.BackoffFactor)
	assert.Equal(t, 5, config.FailureThreshold)
	assert.Equal(t, 30*time.Second, config.ResetTimeout)
	assert.Equal(t, 5*time.Second, config.PublishTimeout)
	assert.Equal(t, 10*time.Second, config.ShutdownTimeout)
}

func TestNewTestEventPublisher(t *testing.T) {
	publisher := NewTestEventPublisher()
	defer publisher.Close()

	assert.NotNil(t, publisher)

	// Test basic functionality
	event := NewCallInitiatedEvent("user123", uuid.New(), "+1234567890", "+9876543210")
	ctx := context.Background()
	err := publisher.Publish(ctx, event)
	assert.NoError(t, err)

	// Get the mock logger and verify event was logged
	mockLogger := publisher.auditLogger.(*mockAuditLogger)
	events := mockLogger.GetEvents()
	assert.Len(t, events, 1)
}

// Test helpers

type invalidDomainEvent struct{}

func (e *invalidDomainEvent) GetEventID() uuid.UUID       { return uuid.New() }
func (e *invalidDomainEvent) GetEventType() EventType     { return EventType("invalid") }
func (e *invalidDomainEvent) GetTimestamp() time.Time     { return time.Now() }
func (e *invalidDomainEvent) GetVersion() int             { return 1 }
func (e *invalidDomainEvent) GetActorID() string          { return "actor" }
func (e *invalidDomainEvent) GetActorType() string        { return "user" }
func (e *invalidDomainEvent) GetTargetID() string         { return "target" }
func (e *invalidDomainEvent) GetTargetType() string       { return "resource" }
func (e *invalidDomainEvent) GetAction() string           { return "action" }
func (e *invalidDomainEvent) GetResult() Result           { return ResultSuccess }
func (e *invalidDomainEvent) GetRequestID() string        { return "req123" }
func (e *invalidDomainEvent) GetSessionID() string        { return "sess123" }
func (e *invalidDomainEvent) GetCorrelationID() string    { return "corr123" }
func (e *invalidDomainEvent) GetComplianceFlags() map[string]bool { return nil }
func (e *invalidDomainEvent) GetDataClasses() []string    { return nil }
func (e *invalidDomainEvent) GetLegalBasis() string       { return "" }
func (e *invalidDomainEvent) GetMetadata() map[string]interface{} { return nil }

func (e *invalidDomainEvent) ToAuditEvent() (*Event, error) {
	return nil, assert.AnError // Force an error
}

// Benchmark tests

func BenchmarkAuditEventPublisher_Publish(b *testing.B) {
	logger, _ := zap.NewDevelopment()
	mockLogger := &mockAuditLogger{}
	config := DefaultPublisherConfig()

	publisher := NewAuditEventPublisher(logger, mockLogger, config)
	defer publisher.Close()

	event := NewCallInitiatedEvent("user123", uuid.New(), "+1234567890", "+9876543210")
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			err := publisher.Publish(ctx, event)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkAuditEventPublisher_PublishAsync(b *testing.B) {
	logger, _ := zap.NewDevelopment()
	mockLogger := &mockAuditLogger{}
	config := DefaultPublisherConfig()

	publisher := NewAuditEventPublisher(logger, mockLogger, config)
	defer publisher.Close()

	event := NewCallInitiatedEvent("user123", uuid.New(), "+1234567890", "+9876543210")
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			publisher.PublishAsync(ctx, event)
		}
	})
}