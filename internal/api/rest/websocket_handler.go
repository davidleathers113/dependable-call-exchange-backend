package rest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"log/slog"
)

// WebSocketHub manages all WebSocket connections
type WebSocketHub struct {
	clients      map[uuid.UUID]*WebSocketClient
	clientsMu    sync.RWMutex
	register     chan *WebSocketClient
	unregister   chan *WebSocketClient
	broadcast    chan *WebSocketMessage
	subscriptions map[string]map[uuid.UUID]bool // topic -> client IDs
	subMu        sync.RWMutex
	eventBus     EventBus
	authService  *AuthMiddleware
	logger       *slog.Logger
	tracer       trace.Tracer
	config       WebSocketConfig
}

// WebSocketConfig holds WebSocket configuration
type WebSocketConfig struct {
	WriteTimeout     time.Duration
	PongTimeout      time.Duration
	PingPeriod       time.Duration
	MaxMessageSize   int64
	ReadBufferSize   int
	WriteBufferSize  int
	CheckOrigin      func(r *http.Request) bool
	EnableCompression bool
}

// WebSocketClient represents a connected client
type WebSocketClient struct {
	ID           uuid.UUID
	UserID       uuid.UUID
	AccountType  string
	conn         *websocket.Conn
	send         chan []byte
	hub          *WebSocketHub
	pingTicker   *time.Ticker
	subscriptions map[string]bool
	subMu        sync.RWMutex
	lastActivity time.Time
	isClosing    bool
	closeMu      sync.Mutex
}

// WebSocketMessage represents a message
type WebSocketMessage struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`
	Event     string                 `json:"event,omitempty"`
	Topic     string                 `json:"topic,omitempty"`
	Data      interface{}            `json:"data,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// EventBus handles event distribution
type EventBus interface {
	Subscribe(ctx context.Context, topics ...string) (<-chan Event, error)
	Publish(ctx context.Context, topic string, event Event) error
}

// Event represents a system event
type Event struct {
	ID       string
	Type     string
	Topic    string
	Data     interface{}
	Metadata map[string]interface{}
}

// DefaultWebSocketConfig returns default configuration
func DefaultWebSocketConfig() WebSocketConfig {
	return WebSocketConfig{
		WriteTimeout:   10 * time.Second,
		PongTimeout:    60 * time.Second,
		PingPeriod:     54 * time.Second, // Must be less than PongTimeout
		MaxMessageSize: 512 * 1024,       // 512KB
		ReadBufferSize: 1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			// In production, implement proper origin checking
			return true
		},
		EnableCompression: true,
	}
}

// NewWebSocketHub creates a new WebSocket hub
func NewWebSocketHub(eventBus EventBus, authService *AuthMiddleware, logger *slog.Logger) *WebSocketHub {
	hub := &WebSocketHub{
		clients:       make(map[uuid.UUID]*WebSocketClient),
		register:      make(chan *WebSocketClient),
		unregister:    make(chan *WebSocketClient),
		broadcast:     make(chan *WebSocketMessage, 256),
		subscriptions: make(map[string]map[uuid.UUID]bool),
		eventBus:      eventBus,
		authService:   authService,
		logger:        logger,
		tracer:        otel.Tracer("api.rest.websocket"),
		config:        DefaultWebSocketConfig(),
	}

	go hub.run()
	return hub
}

// Run starts the hub's main loop
func (h *WebSocketHub) run() {
	for {
		select {
		case client := <-h.register:
			h.registerClient(client)

		case client := <-h.unregister:
			h.unregisterClient(client)

		case message := <-h.broadcast:
			h.broadcastMessage(message)
		}
	}
}

// ServeHTTP handles WebSocket upgrade requests
func (h *WebSocketHub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.tracer.Start(r.Context(), "websocket.connect")
	defer span.End()

	// Extract token from query params (WebSocket doesn't support custom headers)
	token := r.URL.Query().Get("token")
	if token == "" {
		h.logger.Error("missing token in WebSocket request")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Validate token
	claims, err := h.authService.validateToken(ctx, token)
	if err != nil {
		span.RecordError(err)
		h.logger.Error("invalid token", "error", err)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Configure upgrader
	upgrader := websocket.Upgrader{
		ReadBufferSize:    h.config.ReadBufferSize,
		WriteBufferSize:   h.config.WriteBufferSize,
		CheckOrigin:       h.config.CheckOrigin,
		EnableCompression: h.config.EnableCompression,
	}

	// Upgrade connection
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		span.RecordError(err)
		h.logger.Error("websocket upgrade failed", "error", err)
		return
	}

	// Create client
	client := &WebSocketClient{
		ID:           uuid.New(),
		UserID:       claims.UserID,
		AccountType:  claims.AccountType,
		conn:         conn,
		send:         make(chan []byte, 256),
		hub:          h,
		subscriptions: make(map[string]bool),
		lastActivity: time.Now(),
	}

	// Configure connection
	client.conn.SetReadLimit(h.config.MaxMessageSize)
	client.conn.SetReadDeadline(time.Now().Add(h.config.PongTimeout))
	client.conn.SetPongHandler(func(string) error {
		client.conn.SetReadDeadline(time.Now().Add(h.config.PongTimeout))
		client.lastActivity = time.Now()
		return nil
	})

	// Start client routines
	h.register <- client
	go client.writePump()
	go client.readPump()

	// Send welcome message
	welcome := &WebSocketMessage{
		ID:        uuid.New().String(),
		Type:      "system",
		Event:     "connected",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"client_id": client.ID,
			"user_id":   client.UserID,
		},
	}
	client.SendMessage(welcome)

	span.SetAttributes(
		attribute.String("client_id", client.ID.String()),
		attribute.String("user_id", client.UserID.String()),
		attribute.String("account_type", client.AccountType),
	)
}

// Client methods

// readPump handles incoming messages from the client
func (c *WebSocketClient) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	for {
		var message WebSocketMessage
		err := c.conn.ReadJSON(&message)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.hub.logger.Error("websocket read error", "error", err, "client_id", c.ID)
			}
			break
		}

		c.lastActivity = time.Now()
		c.handleMessage(&message)
	}
}

// writePump handles outgoing messages to the client
func (c *WebSocketClient) writePump() {
	ticker := time.NewTicker(c.hub.config.PingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(c.hub.config.WriteTimeout))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued messages to current write
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(c.hub.config.WriteTimeout))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// handleMessage processes incoming messages
func (c *WebSocketClient) handleMessage(message *WebSocketMessage) {
	ctx, span := c.hub.tracer.Start(context.Background(), "websocket.handle_message",
		trace.WithAttributes(
			attribute.String("message_type", message.Type),
			attribute.String("event", message.Event),
		),
	)
	defer span.End()

	switch message.Type {
	case "subscribe":
		c.handleSubscribe(ctx, message)
	case "unsubscribe":
		c.handleUnsubscribe(ctx, message)
	case "ping":
		c.handlePing(ctx, message)
	case "bid":
		c.handleBidMessage(ctx, message)
	case "call":
		c.handleCallMessage(ctx, message)
	default:
		c.hub.logger.Warn("unknown message type", "type", message.Type)
	}
}

// handleSubscribe handles subscription requests
func (c *WebSocketClient) handleSubscribe(ctx context.Context, message *WebSocketMessage) {
	topic := message.Topic
	if topic == "" {
		c.SendError("missing topic")
		return
	}

	// Check permissions
	if !c.canSubscribe(topic) {
		c.SendError("permission denied")
		return
	}

	// Add subscription
	c.subMu.Lock()
	c.subscriptions[topic] = true
	c.subMu.Unlock()

	// Update hub subscriptions
	c.hub.subMu.Lock()
	if c.hub.subscriptions[topic] == nil {
		c.hub.subscriptions[topic] = make(map[uuid.UUID]bool)
	}
	c.hub.subscriptions[topic][c.ID] = true
	c.hub.subMu.Unlock()

	// Send confirmation
	response := &WebSocketMessage{
		ID:        uuid.New().String(),
		Type:      "system",
		Event:     "subscribed",
		Topic:     topic,
		Timestamp: time.Now(),
	}
	c.SendMessage(response)

	c.hub.logger.Info("client subscribed", "client_id", c.ID, "topic", topic)
}

// handleUnsubscribe handles unsubscription requests
func (c *WebSocketClient) handleUnsubscribe(ctx context.Context, message *WebSocketMessage) {
	topic := message.Topic
	if topic == "" {
		return
	}

	// Remove subscription
	c.subMu.Lock()
	delete(c.subscriptions, topic)
	c.subMu.Unlock()

	// Update hub subscriptions
	c.hub.subMu.Lock()
	if clients, ok := c.hub.subscriptions[topic]; ok {
		delete(clients, c.ID)
		if len(clients) == 0 {
			delete(c.hub.subscriptions, topic)
		}
	}
	c.hub.subMu.Unlock()

	// Send confirmation
	response := &WebSocketMessage{
		ID:        uuid.New().String(),
		Type:      "system",
		Event:     "unsubscribed",
		Topic:     topic,
		Timestamp: time.Now(),
	}
	c.SendMessage(response)
}

// handlePing handles ping messages
func (c *WebSocketClient) handlePing(ctx context.Context, message *WebSocketMessage) {
	pong := &WebSocketMessage{
		ID:        message.ID,
		Type:      "pong",
		Timestamp: time.Now(),
	}
	c.SendMessage(pong)
}

// handleBidMessage handles bid-related messages
func (c *WebSocketClient) handleBidMessage(ctx context.Context, message *WebSocketMessage) {
	// In a real implementation, this would interact with the bidding service
	c.hub.logger.Info("received bid message", "client_id", c.ID, "event", message.Event)
}

// handleCallMessage handles call-related messages
func (c *WebSocketClient) handleCallMessage(ctx context.Context, message *WebSocketMessage) {
	// In a real implementation, this would interact with the call service
	c.hub.logger.Info("received call message", "client_id", c.ID, "event", message.Event)
}

// SendMessage sends a message to the client
func (c *WebSocketClient) SendMessage(message *WebSocketMessage) error {
	data, err := json.Marshal(message)
	if err != nil {
		return err
	}

	select {
	case c.send <- data:
		return nil
	default:
		return fmt.Errorf("client send buffer full")
	}
}

// SendError sends an error message to the client
func (c *WebSocketClient) SendError(errorMsg string) {
	message := &WebSocketMessage{
		ID:        uuid.New().String(),
		Type:      "error",
		Event:     "error",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"message": errorMsg,
		},
	}
	c.SendMessage(message)
}

// canSubscribe checks if client can subscribe to a topic
func (c *WebSocketClient) canSubscribe(topic string) bool {
	// Implement permission checking based on topic and user type
	switch {
	case strings.HasPrefix(topic, "calls."):
		return true // Both buyers and sellers can subscribe to calls
	case strings.HasPrefix(topic, "bids."):
		return c.AccountType == "seller" // Only sellers can subscribe to bids
	case strings.HasPrefix(topic, "auctions."):
		return c.AccountType == "seller" // Only sellers can subscribe to auctions
	case strings.HasPrefix(topic, "user."):
		// Users can only subscribe to their own events
		return strings.HasSuffix(topic, c.UserID.String())
	default:
		return false
	}
}

// Hub methods

// registerClient registers a new client
func (h *WebSocketHub) registerClient(client *WebSocketClient) {
	h.clientsMu.Lock()
	h.clients[client.ID] = client
	h.clientsMu.Unlock()

	h.logger.Info("client registered", 
		"client_id", client.ID,
		"user_id", client.UserID,
		"total_clients", len(h.clients),
	)
}

// unregisterClient removes a client
func (h *WebSocketHub) unregisterClient(client *WebSocketClient) {
	h.clientsMu.Lock()
	if _, ok := h.clients[client.ID]; ok {
		delete(h.clients, client.ID)
		close(client.send)
	}
	h.clientsMu.Unlock()

	// Remove from all subscriptions
	h.subMu.Lock()
	for topic := range client.subscriptions {
		if clients, ok := h.subscriptions[topic]; ok {
			delete(clients, client.ID)
			if len(clients) == 0 {
				delete(h.subscriptions, topic)
			}
		}
	}
	h.subMu.Unlock()

	h.logger.Info("client unregistered",
		"client_id", client.ID,
		"user_id", client.UserID,
		"total_clients", len(h.clients),
	)
}

// broadcastMessage sends a message to all subscribed clients
func (h *WebSocketHub) broadcastMessage(message *WebSocketMessage) {
	if message.Topic == "" {
		return
	}

	h.subMu.RLock()
	clients, ok := h.subscriptions[message.Topic]
	h.subMu.RUnlock()

	if !ok {
		return
	}

	data, err := json.Marshal(message)
	if err != nil {
		h.logger.Error("failed to marshal message", "error", err)
		return
	}

	h.clientsMu.RLock()
	defer h.clientsMu.RUnlock()

	for clientID := range clients {
		if client, ok := h.clients[clientID]; ok {
			select {
			case client.send <- data:
			default:
				// Client's send channel is full, skip
				h.logger.Warn("client send buffer full", "client_id", clientID)
			}
		}
	}
}

// BroadcastToTopic sends a message to all clients subscribed to a topic
func (h *WebSocketHub) BroadcastToTopic(topic string, event string, data interface{}) {
	message := &WebSocketMessage{
		ID:        uuid.New().String(),
		Type:      "event",
		Event:     event,
		Topic:     topic,
		Data:      data,
		Timestamp: time.Now(),
	}
	h.broadcast <- message
}

// SendToUser sends a message to a specific user
func (h *WebSocketHub) SendToUser(userID uuid.UUID, event string, data interface{}) error {
	h.clientsMu.RLock()
	defer h.clientsMu.RUnlock()

	sent := false
	for _, client := range h.clients {
		if client.UserID == userID {
			message := &WebSocketMessage{
				ID:        uuid.New().String(),
				Type:      "event",
				Event:     event,
				Data:      data,
				Timestamp: time.Now(),
			}
			if err := client.SendMessage(message); err != nil {
				h.logger.Error("failed to send message to user", "user_id", userID, "error", err)
			} else {
				sent = true
			}
		}
	}

	if !sent {
		return fmt.Errorf("user %s not connected", userID)
	}
	return nil
}

// GetConnectedClients returns the number of connected clients
func (h *WebSocketHub) GetConnectedClients() int {
	h.clientsMu.RLock()
	defer h.clientsMu.RUnlock()
	return len(h.clients)
}

// GetSubscriptionCount returns the number of clients subscribed to a topic
func (h *WebSocketHub) GetSubscriptionCount(topic string) int {
	h.subMu.RLock()
	defer h.subMu.RUnlock()
	if clients, ok := h.subscriptions[topic]; ok {
		return len(clients)
	}
	return 0
}