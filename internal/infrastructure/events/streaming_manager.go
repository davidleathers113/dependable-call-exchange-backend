package events

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/audit"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// NewStreamingManager creates a new streaming manager
func NewStreamingManager(logger *zap.Logger, bufferSize int) *StreamingManager {
	return &StreamingManager{
		connections: make(map[string]*StreamingConnection),
		filters:     make(map[string]StreamingFilter),
		logger:      logger,
		bufferSize:  bufferSize,
	}
}

// AddConnection adds a new streaming connection
func (sm *StreamingManager) AddConnection(
	connectionID string,
	userID uuid.UUID,
	conn interface{},
	filters StreamingFilter,
) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	// Close existing connection if it exists
	if existing, exists := sm.connections[connectionID]; exists {
		close(existing.EventBuffer)
	}
	
	connection := &StreamingConnection{
		ID:          connectionID,
		UserID:      userID,
		Conn:        conn,
		LastPing:    time.Now(),
		Filters:     filters,
		BufferSize:  sm.bufferSize,
		EventBuffer: make(chan DNCDomainEvent, sm.bufferSize),
	}
	
	sm.connections[connectionID] = connection
	sm.filters[connectionID] = filters
	
	sm.logger.Info("Added streaming connection",
		zap.String("connection_id", connectionID),
		zap.String("user_id", userID.String()),
		zap.Int("event_types", len(filters.EventTypes)),
		zap.Int("buffer_size", sm.bufferSize),
	)
	
	return nil
}

// RemoveConnection removes a streaming connection
func (sm *StreamingManager) RemoveConnection(connectionID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	connection, exists := sm.connections[connectionID]
	if !exists {
		return errors.NewNotFoundError("streaming connection not found")
	}
	
	// Close the event buffer
	close(connection.EventBuffer)
	
	delete(sm.connections, connectionID)
	delete(sm.filters, connectionID)
	
	sm.logger.Info("Removed streaming connection",
		zap.String("connection_id", connectionID),
		zap.String("user_id", connection.UserID.String()),
	)
	
	return nil
}

// StreamEvent sends an event to all matching connections
func (sm *StreamingManager) StreamEvent(ctx context.Context, event DNCDomainEvent) error {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	for connectionID, connection := range sm.connections {
		// Check if event matches connection filters
		if !sm.eventMatchesFilter(event, connection.Filters) {
			continue
		}
		
		// Try to send event to connection buffer
		select {
		case connection.EventBuffer <- event:
			sm.logger.Debug("Event sent to streaming connection",
				zap.String("connection_id", connectionID),
				zap.String("event_id", event.GetEventID().String()),
			)
		default:
			// Buffer full, log warning
			sm.logger.Warn("Streaming connection buffer full, dropping event",
				zap.String("connection_id", connectionID),
				zap.String("event_id", event.GetEventID().String()),
			)
		}
	}
	
	return nil
}

// GetConnectionCount returns the number of active connections
func (sm *StreamingManager) GetConnectionCount() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	return len(sm.connections)
}

// CleanupStaleConnections removes connections that haven't pinged recently
func (sm *StreamingManager) CleanupStaleConnections() {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	staleTimeout := 5 * time.Minute
	cutoff := time.Now().Add(-staleTimeout)
	
	for connectionID, connection := range sm.connections {
		if connection.LastPing.Before(cutoff) {
			sm.logger.Info("Removing stale streaming connection",
				zap.String("connection_id", connectionID),
				zap.Time("last_ping", connection.LastPing),
			)
			
			close(connection.EventBuffer)
			delete(sm.connections, connectionID)
			delete(sm.filters, connectionID)
		}
	}
}

// UpdateLastPing updates the last ping time for a connection
func (sm *StreamingManager) UpdateLastPing(connectionID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	if connection, exists := sm.connections[connectionID]; exists {
		connection.LastPing = time.Now()
	}
}

// GetConnection returns a specific connection
func (sm *StreamingManager) GetConnection(connectionID string) (*StreamingConnection, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	connection, exists := sm.connections[connectionID]
	if !exists {
		return nil, errors.NewNotFoundError("streaming connection not found")
	}
	
	return connection, nil
}

// Close shuts down the streaming manager
func (sm *StreamingManager) Close() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	for connectionID, connection := range sm.connections {
		close(connection.EventBuffer)
		delete(sm.connections, connectionID)
		delete(sm.filters, connectionID)
	}
	
	sm.logger.Info("Streaming manager shut down")
	return nil
}

// Private methods

func (sm *StreamingManager) eventMatchesFilter(event DNCDomainEvent, filter StreamingFilter) bool {
	// Check event types filter
	if len(filter.EventTypes) > 0 {
		eventType := event.GetEventType()
		found := false
		for _, filterType := range filter.EventTypes {
			if filterType == eventType {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	
	// Check aggregate types filter
	if len(filter.AggregateTypes) > 0 {
		aggregateType := event.GetAggregateType()
		found := false
		for _, filterType := range filter.AggregateTypes {
			if filterType == aggregateType {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	
	// Check aggregate IDs filter
	if len(filter.AggregateIDs) > 0 {
		aggregateID := event.GetAggregateID()
		found := false
		for _, filterID := range filter.AggregateIDs {
			if filterID == aggregateID {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	
	// TODO: Implement UserIDs filter if events have user context
	
	return true
}

// WebSocketEventStreamer handles WebSocket-specific event streaming
type WebSocketEventStreamer struct {
	sm     *StreamingManager
	logger *zap.Logger
}

// NewWebSocketEventStreamer creates a new WebSocket event streamer
func NewWebSocketEventStreamer(sm *StreamingManager, logger *zap.Logger) *WebSocketEventStreamer {
	return &WebSocketEventStreamer{
		sm:     sm,
		logger: logger,
	}
}

// StartStreaming starts streaming events to a WebSocket connection
func (ws *WebSocketEventStreamer) StartStreaming(
	ctx context.Context,
	connectionID string,
) error {
	connection, err := ws.sm.GetConnection(connectionID)
	if err != nil {
		return err
	}
	
	ws.logger.Info("Starting WebSocket event streaming",
		zap.String("connection_id", connectionID),
		zap.String("user_id", connection.UserID.String()),
	)
	
	for {
		select {
		case event, ok := <-connection.EventBuffer:
			if !ok {
				// Channel closed, stop streaming
				ws.logger.Info("Event buffer closed, stopping stream",
					zap.String("connection_id", connectionID),
				)
				return nil
			}
			
			// Convert event to JSON message
			message, err := ws.createStreamMessage(event)
			if err != nil {
				ws.logger.Error("Failed to create stream message",
					zap.Error(err),
					zap.String("connection_id", connectionID),
					zap.String("event_id", event.GetEventID().String()),
				)
				continue
			}
			
			// Send message to WebSocket
			if err := ws.sendToWebSocket(connection.Conn, message); err != nil {
				ws.logger.Error("Failed to send message to WebSocket",
					zap.Error(err),
					zap.String("connection_id", connectionID),
				)
				
				// Remove connection on send failure
				ws.sm.RemoveConnection(connectionID)
				return err
			}
			
		case <-ctx.Done():
			ws.logger.Info("Context cancelled, stopping stream",
				zap.String("connection_id", connectionID),
			)
			return ctx.Err()
		}
	}
}

func (ws *WebSocketEventStreamer) createStreamMessage(event DNCDomainEvent) ([]byte, error) {
	message := StreamMessage{
		Type:      "event",
		EventID:   event.GetEventID(),
		EventType: event.GetEventType(),
		Timestamp: event.GetTimestamp(),
		Data:      event,
	}
	
	return json.Marshal(message)
}

func (ws *WebSocketEventStreamer) sendToWebSocket(conn interface{}, message []byte) error {
	// This would be implemented based on the actual WebSocket library used
	// For example, with gorilla/websocket:
	// if wsConn, ok := conn.(*websocket.Conn); ok {
	//     return wsConn.WriteMessage(websocket.TextMessage, message)
	// }
	
	ws.logger.Debug("Would send WebSocket message", zap.Int("message_size", len(message)))
	return nil
}

// StreamMessage represents a message sent over the stream
type StreamMessage struct {
	Type      string            `json:"type"`
	EventID   uuid.UUID         `json:"event_id"`
	EventType audit.EventType   `json:"event_type"`
	Timestamp time.Time         `json:"timestamp"`
	Data      interface{}       `json:"data"`
}