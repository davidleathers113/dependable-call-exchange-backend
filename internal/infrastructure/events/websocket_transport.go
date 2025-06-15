package events

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/audit"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// WebSocketTransport implements EventTransport for WebSocket connections
type WebSocketTransport struct {
	logger      *zap.Logger
	connections map[string]*WebSocketConnection
	connMu      sync.RWMutex
	
	// Configuration
	config WebSocketConfig
	
	// Health tracking
	healthCheck *HealthChecker
}

// WebSocketConnection represents a WebSocket client connection
type WebSocketConnection struct {
	ID         string
	Conn       *websocket.Conn
	Send       chan []byte
	UserID     string
	LastPing   time.Time
	mu         sync.Mutex
}

// WebSocketConfig configures the WebSocket transport
type WebSocketConfig struct {
	WriteTimeout    time.Duration
	PingInterval    time.Duration
	PongTimeout     time.Duration
	MaxMessageSize  int64
	SendBufferSize  int
}

// DefaultWebSocketConfig returns default WebSocket configuration
func DefaultWebSocketConfig() WebSocketConfig {
	return WebSocketConfig{
		WriteTimeout:    10 * time.Second,
		PingInterval:    30 * time.Second,
		PongTimeout:     60 * time.Second,
		MaxMessageSize:  1024 * 1024, // 1MB
		SendBufferSize:  256,
	}
}

// WebSocketMessage represents a message sent over WebSocket
type WebSocketMessage struct {
	Type      string          `json:"type"`
	Event     *audit.Event    `json:"event,omitempty"`
	Events    []*audit.Event  `json:"events,omitempty"`
	Timestamp time.Time       `json:"timestamp"`
}

// NewWebSocketTransport creates a new WebSocket transport
func NewWebSocketTransport(logger *zap.Logger, config WebSocketConfig) *WebSocketTransport {
	transport := &WebSocketTransport{
		logger:      logger,
		connections: make(map[string]*WebSocketConnection),
		config:      config,
		healthCheck: NewHealthChecker(5 * time.Minute),
	}
	
	// Start connection manager
	go transport.connectionManager()
	
	return transport
}

// Send sends a single event to specified subscribers
func (t *WebSocketTransport) Send(ctx context.Context, event *audit.Event, subscribers []string) error {
	message := WebSocketMessage{
		Type:      "audit_event",
		Event:     event,
		Timestamp: time.Now(),
	}
	
	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}
	
	t.connMu.RLock()
	defer t.connMu.RUnlock()
	
	var sendErrors []error
	for _, subID := range subscribers {
		conn, exists := t.connections[subID]
		if !exists {
			continue
		}
		
		select {
		case conn.Send <- data:
			// Successfully queued
		case <-time.After(t.config.WriteTimeout):
			sendErrors = append(sendErrors, fmt.Errorf("timeout sending to %s", subID))
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	
	if len(sendErrors) > 0 {
		return fmt.Errorf("failed to send to %d subscribers", len(sendErrors))
	}
	
	t.healthCheck.RecordSuccess()
	return nil
}

// SendBatch sends multiple events to specified subscribers
func (t *WebSocketTransport) SendBatch(ctx context.Context, events []*audit.Event, subscribers []string) error {
	message := WebSocketMessage{
		Type:      "audit_event_batch",
		Events:    events,
		Timestamp: time.Now(),
	}
	
	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal events: %w", err)
	}
	
	// Group subscribers to avoid duplicate sends
	uniqueSubs := make(map[string]bool)
	for _, subID := range subscribers {
		uniqueSubs[subID] = true
	}
	
	t.connMu.RLock()
	defer t.connMu.RUnlock()
	
	var sendErrors []error
	for subID := range uniqueSubs {
		conn, exists := t.connections[subID]
		if !exists {
			continue
		}
		
		select {
		case conn.Send <- data:
			// Successfully queued
		case <-time.After(t.config.WriteTimeout):
			sendErrors = append(sendErrors, fmt.Errorf("timeout sending to %s", subID))
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	
	if len(sendErrors) > 0 {
		return fmt.Errorf("failed to send to %d subscribers", len(sendErrors))
	}
	
	t.healthCheck.RecordSuccess()
	return nil
}

// GetProtocol returns the transport protocol type
func (t *WebSocketTransport) GetProtocol() TransportType {
	return TransportWebSocket
}

// IsHealthy checks if the transport is healthy
func (t *WebSocketTransport) IsHealthy() bool {
	return t.healthCheck.IsHealthy()
}

// Close gracefully shuts down the transport
func (t *WebSocketTransport) Close() error {
	t.logger.Info("Closing WebSocket transport")
	
	t.connMu.Lock()
	defer t.connMu.Unlock()
	
	// Close all connections
	for id, conn := range t.connections {
		close(conn.Send)
		conn.Conn.Close()
		delete(t.connections, id)
	}
	
	return nil
}

// AddConnection adds a new WebSocket connection
func (t *WebSocketTransport) AddConnection(id string, conn *websocket.Conn, userID string) {
	wsConn := &WebSocketConnection{
		ID:       id,
		Conn:     conn,
		Send:     make(chan []byte, t.config.SendBufferSize),
		UserID:   userID,
		LastPing: time.Now(),
	}
	
	t.connMu.Lock()
	t.connections[id] = wsConn
	t.connMu.Unlock()
	
	// Start goroutines for this connection
	go t.writePump(wsConn)
	go t.readPump(wsConn)
	
	t.logger.Info("WebSocket connection added",
		zap.String("connection_id", id),
		zap.String("user_id", userID),
	)
}

// RemoveConnection removes a WebSocket connection
func (t *WebSocketTransport) RemoveConnection(id string) {
	t.connMu.Lock()
	conn, exists := t.connections[id]
	if exists {
		close(conn.Send)
		delete(t.connections, id)
	}
	t.connMu.Unlock()
	
	if exists {
		t.logger.Info("WebSocket connection removed",
			zap.String("connection_id", id),
		)
	}
}

// GetConnectionCount returns the number of active connections
func (t *WebSocketTransport) GetConnectionCount() int {
	t.connMu.RLock()
	defer t.connMu.RUnlock()
	return len(t.connections)
}

// Private methods

func (t *WebSocketTransport) writePump(conn *WebSocketConnection) {
	ticker := time.NewTicker(t.config.PingInterval)
	defer func() {
		ticker.Stop()
		conn.Conn.Close()
		t.RemoveConnection(conn.ID)
	}()
	
	for {
		select {
		case message, ok := <-conn.Send:
			conn.mu.Lock()
			conn.Conn.SetWriteDeadline(time.Now().Add(t.config.WriteTimeout))
			
			if !ok {
				// Channel closed
				conn.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				conn.mu.Unlock()
				return
			}
			
			if err := conn.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
				conn.mu.Unlock()
				t.logger.Error("WebSocket write error",
					zap.Error(err),
					zap.String("connection_id", conn.ID),
				)
				return
			}
			
			// Send any queued messages in the same write
			n := len(conn.Send)
			for i := 0; i < n; i++ {
				if err := conn.Conn.WriteMessage(websocket.TextMessage, <-conn.Send); err != nil {
					conn.mu.Unlock()
					return
				}
			}
			
			conn.mu.Unlock()
			
		case <-ticker.C:
			conn.mu.Lock()
			conn.Conn.SetWriteDeadline(time.Now().Add(t.config.WriteTimeout))
			if err := conn.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				conn.mu.Unlock()
				return
			}
			conn.mu.Unlock()
		}
	}
}

func (t *WebSocketTransport) readPump(conn *WebSocketConnection) {
	defer func() {
		conn.Conn.Close()
		t.RemoveConnection(conn.ID)
	}()
	
	conn.Conn.SetReadLimit(t.config.MaxMessageSize)
	conn.Conn.SetReadDeadline(time.Now().Add(t.config.PongTimeout))
	conn.Conn.SetPongHandler(func(string) error {
		conn.LastPing = time.Now()
		conn.Conn.SetReadDeadline(time.Now().Add(t.config.PongTimeout))
		return nil
	})
	
	for {
		// Read message from client
		messageType, message, err := conn.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				t.logger.Error("WebSocket read error",
					zap.Error(err),
					zap.String("connection_id", conn.ID),
				)
			}
			break
		}
		
		// Handle control messages from client
		if messageType == websocket.TextMessage {
			t.handleClientMessage(conn, message)
		}
	}
}

func (t *WebSocketTransport) handleClientMessage(conn *WebSocketConnection, message []byte) {
	// Parse client messages (e.g., subscription updates, ping/pong)
	var msg map[string]interface{}
	if err := json.Unmarshal(message, &msg); err != nil {
		t.logger.Warn("Invalid client message",
			zap.Error(err),
			zap.String("connection_id", conn.ID),
		)
		return
	}
	
	// Handle different message types
	msgType, ok := msg["type"].(string)
	if !ok {
		return
	}
	
	switch msgType {
	case "ping":
		// Respond with pong
		pong := map[string]interface{}{
			"type":      "pong",
			"timestamp": time.Now(),
		}
		if data, err := json.Marshal(pong); err == nil {
			select {
			case conn.Send <- data:
			default:
				// Send buffer full, skip pong
			}
		}
		
	case "subscribe":
		// Handle subscription updates
		// This would typically update the subscription filters
		t.logger.Debug("Subscription update received",
			zap.String("connection_id", conn.ID),
			zap.Any("filters", msg["filters"]),
		)
		
	default:
		t.logger.Debug("Unknown message type",
			zap.String("type", msgType),
			zap.String("connection_id", conn.ID),
		)
	}
}

func (t *WebSocketTransport) connectionManager() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	
	for range ticker.C {
		t.cleanupStaleConnections()
	}
}

func (t *WebSocketTransport) cleanupStaleConnections() {
	t.connMu.Lock()
	defer t.connMu.Unlock()
	
	now := time.Now()
	staleTimeout := 2 * t.config.PongTimeout
	
	for id, conn := range t.connections {
		if now.Sub(conn.LastPing) > staleTimeout {
			t.logger.Warn("Removing stale connection",
				zap.String("connection_id", id),
				zap.Duration("last_ping_ago", now.Sub(conn.LastPing)),
			)
			
			close(conn.Send)
			conn.Conn.Close()
			delete(t.connections, id)
		}
	}
}

// HealthChecker tracks transport health
type HealthChecker struct {
	lastSuccess   time.Time
	lastFailure   time.Time
	failureCount  int
	mu            sync.RWMutex
	healthTimeout time.Duration
}

func NewHealthChecker(timeout time.Duration) *HealthChecker {
	return &HealthChecker{
		healthTimeout: timeout,
		lastSuccess:   time.Now(),
	}
}

func (h *HealthChecker) RecordSuccess() {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	h.lastSuccess = time.Now()
	h.failureCount = 0
}

func (h *HealthChecker) RecordFailure() {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	h.lastFailure = time.Now()
	h.failureCount++
}

func (h *HealthChecker) IsHealthy() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	// Healthy if we've had a success recently
	return time.Since(h.lastSuccess) < h.healthTimeout
}