package websocket

import (
	"context"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/audit"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/infrastructure/events"
	"go.uber.org/zap"
)

// AuditEventsIntegration demonstrates how to integrate the audit events WebSocket handler
// with the audit logging service and event publisher
type AuditEventsIntegration struct {
	handler   *Handler
	publisher *events.AuditEventPublisher
	logger    *zap.Logger
}

// NewAuditEventsIntegration creates a new integration instance
func NewAuditEventsIntegration(handler *Handler, publisher *events.AuditEventPublisher, logger *zap.Logger) *AuditEventsIntegration {
	return &AuditEventsIntegration{
		handler:   handler,
		publisher: publisher,
		logger:    logger,
	}
}

// StartEventStreaming sets up the connection between audit publisher and WebSocket hub
func (i *AuditEventsIntegration) StartEventStreaming(ctx context.Context) error {
	// Subscribe the WebSocket hub to audit events from the publisher
	_, err := i.publisher.Subscribe(
		ctx,
		// Use a system user ID for the WebSocket hub subscription
		auditSystemUserID(),
		events.TransportWebSocket,
		events.EventFilters{
			// Subscribe to all audit event types
			EventTypes: []audit.EventType{
				audit.EventConsentGranted,
				audit.EventConsentRevoked,
				audit.EventDataAccessed,
				audit.EventCallInitiated,
				audit.EventCallCompleted,
				audit.EventAuthSuccess,
				audit.EventAuthFailure,
				audit.EventBidPlaced,
				audit.EventPaymentProcessed,
				audit.EventComplianceViolation,
				audit.EventSecurityIncident,
				audit.EventSystemFailure,
			},
			// Subscribe to all severities
			Severity: []audit.Severity{
				audit.SeverityInfo,
				audit.SeverityWarning,
				audit.SeverityError,
				audit.SeverityCritical,
			},
		},
		// Pass the audit hub as connection data for the publisher to use
		i.handler.GetAuditEventHub(),
	)
	
	if err != nil {
		return err
	}

	i.logger.Info("Audit events WebSocket streaming started",
		zap.String("component", "audit_events_integration"),
	)

	return nil
}

// PublishTestEvent publishes a test audit event to demonstrate the integration
func (i *AuditEventsIntegration) PublishTestEvent(ctx context.Context) error {
	// Create a test audit event
	testEvent, err := audit.NewEvent(
		audit.EventSystemStartup,
		"system",
		"websocket-handler",
		"start_audit_streaming",
	)
	if err != nil {
		return err
	}

	// Set additional fields
	testEvent.Result = "success"
	testEvent.Metadata = map[string]interface{}{
		"component":     "audit_events_integration",
		"test_event":    true,
		"timestamp_str": time.Now().Format(time.RFC3339),
	}

	// Publish through the audit publisher
	if err := i.publisher.Publish(ctx, testEvent); err != nil {
		return err
	}

	// Also broadcast directly to WebSocket clients
	i.handler.GetAuditEventHub().BroadcastAuditEvent(testEvent)

	i.logger.Info("Test audit event published",
		zap.String("event_id", testEvent.ID.String()),
		zap.String("event_type", string(testEvent.Type)),
	)

	return nil
}

// PublishSecurityAlert demonstrates publishing a security alert
func (i *AuditEventsIntegration) PublishSecurityAlert(ctx context.Context, severity string, message string, details interface{}) {
	i.handler.GetAuditEventHub().BroadcastSecurityAlert(details, severity, message)
	
	i.logger.Info("Security alert broadcasted",
		zap.String("severity", severity),
		zap.String("message", message),
	)
}

// PublishComplianceAlert demonstrates publishing a compliance alert
func (i *AuditEventsIntegration) PublishComplianceAlert(ctx context.Context, severity string, message string, details interface{}) {
	i.handler.GetAuditEventHub().BroadcastComplianceAlert(details, severity, message)
	
	i.logger.Info("Compliance alert broadcasted",
		zap.String("severity", severity),
		zap.String("message", message),
	)
}

// GetConnectionStats returns statistics about WebSocket connections
func (i *AuditEventsIntegration) GetConnectionStats() map[string]interface{} {
	auditHub := i.handler.GetAuditEventHub()
	
	return map[string]interface{}{
		"audit_clients": map[string]interface{}{
			"total_count":        auditHub.GetClientCount(),
			"connected_clients":  auditHub.GetConnectedClients(),
			"hub_metrics":        auditHub.GetMetrics(),
		},
		"websocket_info": i.handler.GetWebSocketInfo(),
	}
}

// GetHealth checks the health of the audit events integration
func (i *AuditEventsIntegration) GetHealth() error {
	// Check WebSocket handler health
	if err := i.handler.HealthCheck(); err != nil {
		return err
	}

	// Check audit publisher health
	if err := i.publisher.Health(); err != nil {
		return err
	}

	return nil
}

// Helper function to generate a system user ID for audit subscriptions
func auditSystemUserID() interface{} {
	// In a real implementation, this would be a proper system user UUID
	// For now, return a placeholder that the publisher can handle
	return map[string]string{
		"type": "system",
		"name": "audit-websocket-hub",
	}
}

// Example usage patterns for the audit events WebSocket integration:

/*
// 1. Setting up the integration in your main application:

func main() {
	// Initialize logger, config, etc.
	logger := zap.NewProduction()
	
	// Create WebSocket handler
	wsHandler := websocket.NewHandler(logger)
	
	// Create audit publisher with WebSocket transport
	transports := map[events.TransportType]events.EventTransport{
		events.TransportWebSocket: websocket.NewWebSocketTransport(wsHandler.GetAuditEventHub()),
		// ... other transports
	}
	
	auditPublisher, err := events.NewAuditEventPublisher(
		ctx, logger, events.DefaultPublisherConfig(), transports)
	if err != nil {
		log.Fatal(err)
	}
	
	// Create integration
	integration := websocket.NewAuditEventsIntegration(wsHandler, auditPublisher, logger)
	
	// Start services
	go wsHandler.Start(ctx)
	go integration.StartEventStreaming(ctx)
	
	// Set up HTTP routes
	http.HandleFunc("/ws/audit", wsHandler.HandleAuditEvents)
	http.HandleFunc("/ws/consent", wsHandler.HandleConsentEvents)
	
	// ... start server
}

// 2. Publishing audit events from your business logic:

func (s *CallService) InitiateCall(ctx context.Context, req *CallRequest) (*Call, error) {
	// ... business logic
	
	// Log audit event
	auditEvent, _ := audit.NewEvent(
		audit.EventCallInitiated,
		req.ActorID,
		call.ID.String(),
		"initiate_call",
	)
	auditEvent.Result = "success"
	auditEvent.Metadata = map[string]interface{}{
		"from_number": req.FromNumber,
		"to_number":   req.ToNumber,
		"call_type":   req.CallType,
	}
	
	// This will automatically stream to WebSocket clients
	s.auditPublisher.Publish(ctx, auditEvent)
	
	return call, nil
}

// 3. Client-side JavaScript for consuming audit events:

const ws = new WebSocket('ws://localhost:8080/ws/audit');

ws.onopen = function() {
	console.log('Connected to audit event stream');
	
	// Set up filters
	ws.send(JSON.stringify({
		type: 'update_filters',
		filters: {
			event_types: ['audit.event.published'],
			severities: ['critical', 'error'],
			categories: ['security', 'compliance'],
			time_range: {
				relative: '24h'
			}
		}
	}));
};

ws.onmessage = function(event) {
	const auditEvent = JSON.parse(event.data);
	console.log('Audit event received:', auditEvent);
	
	// Handle different event types
	switch(auditEvent.type) {
		case 'audit.event.published':
			handleAuditEvent(auditEvent.audit_event);
			break;
		case 'audit.security.alert':
			handleSecurityAlert(auditEvent);
			break;
		case 'audit.compliance.alert':
			handleComplianceAlert(auditEvent);
			break;
	}
};

function handleAuditEvent(auditEvent) {
	// Display in audit log UI
	addToAuditLog({
		timestamp: auditEvent.timestamp,
		actor: auditEvent.actor_id,
		action: auditEvent.action,
		target: auditEvent.target_id,
		result: auditEvent.result,
		severity: auditEvent.severity
	});
}

function handleSecurityAlert(alert) {
	// Show security alert notification
	showNotification('security', alert.summary, alert.severity);
}

// 4. Role-based filtering example:

// Admin sees everything:
// {
//   "event_types": ["*"],
//   "severities": ["*"]
// }

// Security team sees security and compliance events:
// {
//   "categories": ["security", "compliance"],
//   "severities": ["warning", "error", "critical"]
// }

// Compliance team sees compliance-related events:
// {
//   "compliance_only": true,
//   "event_types": ["consent.*", "compliance.*", "data_access.*"]
// }

// Operations team sees system events:
// {
//   "categories": ["system", "call"],
//   "results": ["failure", "error"]
// }

*/