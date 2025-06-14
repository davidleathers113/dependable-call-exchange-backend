package consent

import (
	"context"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/api/websocket"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/consent"
	"go.uber.org/zap"
)

// WebSocketEventPublisher implements EventPublisher interface using WebSocket hub
type WebSocketEventPublisher struct {
	logger *zap.Logger
	hub    *websocket.ConsentEventHub
}

// NewWebSocketEventPublisher creates a new WebSocket event publisher
func NewWebSocketEventPublisher(logger *zap.Logger, hub *websocket.ConsentEventHub) *WebSocketEventPublisher {
	return &WebSocketEventPublisher{
		logger: logger,
		hub:    hub,
	}
}

// PublishConsentGranted publishes a consent granted event
func (p *WebSocketEventPublisher) PublishConsentGranted(ctx context.Context, event consent.ConsentCreatedEvent) error {
	p.logger.Info("Publishing consent granted event",
		zap.String("consent_id", event.ConsentID.String()),
		zap.String("consumer_id", event.ConsumerID.String()),
		zap.String("business_id", event.BusinessID.String()),
		zap.String("purpose", string(event.Purpose)),
	)
	
	p.hub.BroadcastConsentGranted(event)
	return nil
}

// PublishConsentRevoked publishes a consent revoked event
func (p *WebSocketEventPublisher) PublishConsentRevoked(ctx context.Context, event consent.ConsentRevokedEvent) error {
	p.logger.Info("Publishing consent revoked event",
		zap.String("consent_id", event.ConsentID.String()),
		zap.String("consumer_id", event.ConsumerID.String()),
		zap.String("business_id", event.BusinessID.String()),
		zap.String("reason", event.Reason),
	)
	
	p.hub.BroadcastConsentRevoked(event)
	return nil
}

// PublishConsentUpdated publishes a consent updated event
func (p *WebSocketEventPublisher) PublishConsentUpdated(ctx context.Context, event consent.ConsentUpdatedEvent) error {
	p.logger.Info("Publishing consent updated event",
		zap.String("consent_id", event.ConsentID.String()),
		zap.String("consumer_id", event.ConsumerID.String()),
		zap.String("business_id", event.BusinessID.String()),
		zap.String("updated_by", event.UpdatedBy.String()),
	)
	
	p.hub.BroadcastConsentUpdated(event)
	return nil
}