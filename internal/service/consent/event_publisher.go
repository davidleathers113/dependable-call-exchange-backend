package consent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/consent"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
	"go.uber.org/zap"
)

// EventPublisherImpl implements the EventPublisher interface
type EventPublisherImpl struct {
	logger         *zap.Logger
	eventBus       EventBus
	eventStore     consent.EventStore
}

// EventBus interface for publishing events to message bus
type EventBus interface {
	Publish(ctx context.Context, topic string, event interface{}) error
	PublishBatch(ctx context.Context, topic string, events []interface{}) error
}

// NewEventPublisher creates a new event publisher
func NewEventPublisher(logger *zap.Logger, eventBus EventBus, eventStore consent.EventStore) EventPublisher {
	return &EventPublisherImpl{
		logger:     logger,
		eventBus:   eventBus,
		eventStore: eventStore,
	}
}

// PublishConsentGranted publishes a consent granted event
func (ep *EventPublisherImpl) PublishConsentGranted(ctx context.Context, event consent.ConsentCreatedEvent) error {
	logger := ep.logger.With(
		zap.String("consent_id", event.ConsentID.String()),
		zap.String("consumer_id", event.ConsumerID.String()),
		zap.String("business_id", event.BusinessID.String()),
	)

	// Store event first
	if err := ep.eventStore.SaveEvents(ctx, []interface{}{event}); err != nil {
		logger.Error("failed to store consent granted event", zap.Error(err))
		return errors.NewInternalError("failed to store event").WithCause(err)
	}

	// Publish to event bus
	topic := ep.getTopicForEvent("consent.granted")
	if err := ep.eventBus.Publish(ctx, topic, event); err != nil {
		logger.Error("failed to publish consent granted event", zap.Error(err))
		// Don't fail the operation if publishing fails - event is stored
		// Background process can retry publishing later
	}

	// Publish to type-specific topic for subscribers interested in specific consent types
	// Note: ConsentCreatedEvent doesn't have ConsentType field, using generic topic for now
	typeSpecificTopic := "consent.granted.generic"
	if err := ep.eventBus.Publish(ctx, typeSpecificTopic, event); err != nil {
		logger.Warn("failed to publish to type-specific topic", 
			zap.String("topic", typeSpecificTopic),
			zap.Error(err))
	}

	logger.Info("consent granted event published successfully")
	return nil
}

// PublishConsentRevoked publishes a consent revoked event
func (ep *EventPublisherImpl) PublishConsentRevoked(ctx context.Context, event consent.ConsentRevokedEvent) error {
	logger := ep.logger.With(
		zap.String("consent_id", event.ConsentID.String()),
		zap.String("consumer_id", event.ConsumerID.String()),
		zap.String("reason", event.Reason),
	)

	// Store event first
	if err := ep.eventStore.SaveEvents(ctx, []interface{}{event}); err != nil {
		logger.Error("failed to store consent revoked event", zap.Error(err))
		return errors.NewInternalError("failed to store event").WithCause(err)
	}

	// Publish to event bus
	topic := ep.getTopicForEvent("consent.revoked")
	if err := ep.eventBus.Publish(ctx, topic, event); err != nil {
		logger.Error("failed to publish consent revoked event", zap.Error(err))
		// Don't fail the operation if publishing fails
	}

	// Publish to type-specific topic
	typeSpecificTopic := "consent.revoked.generic"
	if err := ep.eventBus.Publish(ctx, typeSpecificTopic, event); err != nil {
		logger.Warn("failed to publish to type-specific topic", 
			zap.String("topic", typeSpecificTopic),
			zap.Error(err))
	}

	// Publish critical revocation event for immediate processing
	// This ensures systems stop using revoked consent immediately
	criticalTopic := "consent.critical.revoked"
	if err := ep.eventBus.Publish(ctx, criticalTopic, event); err != nil {
		logger.Error("failed to publish critical revocation event", zap.Error(err))
		// This is more serious but still don't fail the operation
	}

	logger.Info("consent revoked event published successfully")
	return nil
}

// PublishConsentUpdated publishes a consent updated event
func (ep *EventPublisherImpl) PublishConsentUpdated(ctx context.Context, event consent.ConsentUpdatedEvent) error {
	logger := ep.logger.With(
		zap.String("consent_id", event.ConsentID.String()),
		zap.String("consumer_id", event.ConsumerID.String()),
	)

	// Store event first
	if err := ep.eventStore.SaveEvents(ctx, []interface{}{event}); err != nil {
		logger.Error("failed to store consent updated event", zap.Error(err))
		return errors.NewInternalError("failed to store event").WithCause(err)
	}

	// Publish to event bus
	topic := ep.getTopicForEvent("consent.updated")
	if err := ep.eventBus.Publish(ctx, topic, event); err != nil {
		logger.Error("failed to publish consent updated event", zap.Error(err))
		// Don't fail the operation if publishing fails
	}

	// Publish to type-specific topic
	typeSpecificTopic := "consent.updated.generic"
	if err := ep.eventBus.Publish(ctx, typeSpecificTopic, event); err != nil {
		logger.Warn("failed to publish to type-specific topic", 
			zap.String("topic", typeSpecificTopic),
			zap.Error(err))
	}

	// Check if this is a significant update that requires immediate attention
	if ep.isSignificantUpdate(event) {
		significantTopic := "consent.significant.updated"
		if err := ep.eventBus.Publish(ctx, significantTopic, event); err != nil {
			logger.Warn("failed to publish significant update event", zap.Error(err))
		}
	}

	logger.Info("consent updated event published successfully")
	return nil
}

// Helper methods

func (ep *EventPublisherImpl) getTopicForEvent(eventType string) string {
	// Environment-specific topic naming
	// In production, this might include environment prefix
	return fmt.Sprintf("dce.compliance.%s", eventType)
}

func (ep *EventPublisherImpl) isSignificantUpdate(event consent.ConsentUpdatedEvent) bool {
	// Determine if update is significant enough to warrant special handling
	
	// Check if channels changed significantly
	if len(event.OldChannels) != len(event.NewChannels) {
		return true
	}
	
	// Check if any specific channels changed
	for i, oldChannel := range event.OldChannels {
		if i >= len(event.NewChannels) || oldChannel != event.NewChannels[i] {
			return true
		}
	}

	return false
}

// InMemoryEventBus is a simple in-memory implementation of EventBus for testing
type InMemoryEventBus struct {
	logger     *zap.Logger
	handlers   map[string][]func(interface{})
}

// NewInMemoryEventBus creates a new in-memory event bus
func NewInMemoryEventBus(logger *zap.Logger) EventBus {
	return &InMemoryEventBus{
		logger:   logger,
		handlers: make(map[string][]func(interface{})),
	}
}

// Publish publishes an event to a topic
func (bus *InMemoryEventBus) Publish(ctx context.Context, topic string, event interface{}) error {
	bus.logger.Debug("publishing event to topic",
		zap.String("topic", topic),
		zap.String("event_type", fmt.Sprintf("%T", event)),
	)

	// In a real implementation, this would publish to Kafka, RabbitMQ, etc.
	// For now, just log the event
	eventJSON, err := json.Marshal(event)
	if err != nil {
		return errors.NewInternalError("failed to marshal event").WithCause(err)
	}

	bus.logger.Info("event published",
		zap.String("topic", topic),
		zap.String("event", string(eventJSON)),
	)

	// Call any registered handlers (for testing)
	if handlers, ok := bus.handlers[topic]; ok {
		for _, handler := range handlers {
			handler(event)
		}
	}

	return nil
}

// PublishBatch publishes multiple events to a topic
func (bus *InMemoryEventBus) PublishBatch(ctx context.Context, topic string, events []interface{}) error {
	for _, event := range events {
		if err := bus.Publish(ctx, topic, event); err != nil {
			return err
		}
	}
	return nil
}

// Subscribe registers a handler for a topic (for testing)
func (bus *InMemoryEventBus) Subscribe(topic string, handler func(interface{})) {
	bus.handlers[topic] = append(bus.handlers[topic], handler)
}