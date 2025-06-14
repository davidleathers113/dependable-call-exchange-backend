package websocket

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/consent"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// ConsentEventType represents the type of consent event
type ConsentEventType string

const (
	ConsentEventGranted ConsentEventType = "consent.granted"
	ConsentEventRevoked ConsentEventType = "consent.revoked"
	ConsentEventUpdated ConsentEventType = "consent.updated"
	ConsentEventExpired ConsentEventType = "consent.expired"
)

// ConsentEvent represents a real-time consent event
type ConsentEvent struct {
	ID          string           `json:"id"`
	Type        ConsentEventType `json:"type"`
	ConsumerID  string           `json:"consumer_id"`
	ConsentID   string           `json:"consent_id"`
	ConsentType string           `json:"consent_type"`
	Channel     string           `json:"channel"`
	Status      string           `json:"status"`
	Timestamp   time.Time        `json:"timestamp"`
	Data        interface{}      `json:"data,omitempty"`
}

// ConsentEventHub manages WebSocket connections for consent events
type ConsentEventHub struct {
	logger      *zap.Logger
	clients     map[uuid.UUID]*ConsentClient
	clientsLock sync.RWMutex
	broadcast   chan *ConsentEvent
	register    chan *ConsentClient
	unregister  chan *ConsentClient
	done        chan struct{}
}

// ConsentClient represents a WebSocket client subscribed to consent events
type ConsentClient struct {
	ID            uuid.UUID
	conn          *websocket.Conn
	send          chan *ConsentEvent
	hub           *ConsentEventHub
	filters       ConsentEventFilters
	authenticated bool
	userID        uuid.UUID
	role          string
}

// ConsentEventFilters defines filters for consent events
type ConsentEventFilters struct {
	ConsumerIDs  []uuid.UUID              `json:"consumer_ids,omitempty"`
	ConsentTypes []consent.Type           `json:"consent_types,omitempty"`
	Channels     []consent.Channel        `json:"channels,omitempty"`
	EventTypes   []ConsentEventType       `json:"event_types,omitempty"`
}

// NewConsentEventHub creates a new consent event hub
func NewConsentEventHub(logger *zap.Logger) *ConsentEventHub {
	return &ConsentEventHub{
		logger:     logger,
		clients:    make(map[uuid.UUID]*ConsentClient),
		broadcast:  make(chan *ConsentEvent, 100),
		register:   make(chan *ConsentClient),
		unregister: make(chan *ConsentClient),
		done:       make(chan struct{}),
	}
}

// Run starts the event hub
func (h *ConsentEventHub) Run(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			h.shutdown()
			return
		case <-h.done:
			return
		case client := <-h.register:
			h.registerClient(client)
		case client := <-h.unregister:
			h.unregisterClient(client)
		case event := <-h.broadcast:
			h.broadcastEvent(event)
		case <-ticker.C:
			h.pingClients()
		}
	}
}

// Stop gracefully shuts down the hub
func (h *ConsentEventHub) Stop() {
	close(h.done)
}

// BroadcastConsentGranted broadcasts a consent granted event
func (h *ConsentEventHub) BroadcastConsentGranted(event consent.ConsentCreatedEvent) {
	consentChannel := ""
	if len(event.Channels) > 0 {
		consentChannel = string(event.Channels[0])
	}
	
	h.broadcast <- &ConsentEvent{
		ID:          uuid.New().String(),
		Type:        ConsentEventGranted,
		ConsumerID:  event.ConsumerID.String(),
		ConsentID:   event.ConsentID.String(),
		ConsentType: "", // Not available in ConsentCreatedEvent, would need to pass separately
		Channel:     consentChannel,
		Status:      "active",
		Timestamp:   event.CreatedAt,
		Data: map[string]interface{}{
			"business_id": event.BusinessID.String(),
			"purpose":     string(event.Purpose),
			"source":      string(event.Source),
		},
	}
}

// BroadcastConsentRevoked broadcasts a consent revoked event
func (h *ConsentEventHub) BroadcastConsentRevoked(event consent.ConsentRevokedEvent) {
	h.broadcast <- &ConsentEvent{
		ID:          uuid.New().String(),
		Type:        ConsentEventRevoked,
		ConsumerID:  event.ConsumerID.String(),
		ConsentID:   event.ConsentID.String(),
		ConsentType: "", // Not available in ConsentRevokedEvent, would need to pass separately
		Channel:     "", // Not available in ConsentRevokedEvent
		Status:      "revoked",
		Timestamp:   event.RevokedAt,
		Data: map[string]interface{}{
			"revoked_by":     event.RevokedBy.String(),
			"revoked_reason": event.Reason,
			"business_id":    event.BusinessID.String(),
		},
	}
}

// BroadcastConsentUpdated broadcasts a consent updated event
func (h *ConsentEventHub) BroadcastConsentUpdated(event consent.ConsentUpdatedEvent) {
	newChannel := ""
	if len(event.NewChannels) > 0 {
		newChannel = string(event.NewChannels[0])
	}
	
	// Build changes map from old/new channels comparison
	changes := map[string]interface{}{
		"old_channels": make([]string, len(event.OldChannels)),
		"new_channels": make([]string, len(event.NewChannels)),
	}
	
	// Convert old channels to strings
	for i, ch := range event.OldChannels {
		changes["old_channels"].([]string)[i] = string(ch)
	}
	
	// Convert new channels to strings
	for i, ch := range event.NewChannels {
		changes["new_channels"].([]string)[i] = string(ch)
	}
	
	h.broadcast <- &ConsentEvent{
		ID:          uuid.New().String(),
		Type:        ConsentEventUpdated,
		ConsumerID:  event.ConsumerID.String(),
		ConsentID:   event.ConsentID.String(),
		ConsentType: "", // Not available in ConsentUpdatedEvent, would need to pass separately
		Channel:     newChannel,
		Status:      "active",
		Timestamp:   event.UpdatedAt,
		Data: map[string]interface{}{
			"updated_by": event.UpdatedBy.String(),
			"changes":    changes,
		},
	}
}

// RegisterClient registers a new WebSocket client
func (h *ConsentEventHub) RegisterClient(client *ConsentClient) {
	h.register <- client
}

// UnregisterClient unregisters a WebSocket client
func (h *ConsentEventHub) UnregisterClient(client *ConsentClient) {
	h.unregister <- client
}

func (h *ConsentEventHub) registerClient(client *ConsentClient) {
	h.clientsLock.Lock()
	defer h.clientsLock.Unlock()

	h.clients[client.ID] = client
	h.logger.Info("WebSocket client registered",
		zap.String("client_id", client.ID.String()),
		zap.String("user_id", client.userID.String()),
		zap.String("role", client.role),
	)

	// Send welcome message
	welcome := &ConsentEvent{
		ID:        uuid.New().String(),
		Type:      "connection.established",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"client_id": client.ID.String(),
			"message":   "Connected to consent event stream",
		},
	}
	
	select {
	case client.send <- welcome:
	default:
		// Client send channel full
	}
}

func (h *ConsentEventHub) unregisterClient(client *ConsentClient) {
	h.clientsLock.Lock()
	defer h.clientsLock.Unlock()

	if _, exists := h.clients[client.ID]; exists {
		delete(h.clients, client.ID)
		close(client.send)
		h.logger.Info("WebSocket client unregistered",
			zap.String("client_id", client.ID.String()),
		)
	}
}

func (h *ConsentEventHub) broadcastEvent(event *ConsentEvent) {
	h.clientsLock.RLock()
	defer h.clientsLock.RUnlock()

	for _, client := range h.clients {
		if client.shouldReceiveEvent(event) {
			select {
			case client.send <- event:
			default:
				// Client send channel full, close connection
				h.logger.Warn("Client send channel full, closing connection",
					zap.String("client_id", client.ID.String()),
				)
				go func(c *ConsentClient) {
					h.unregister <- c
				}(client)
			}
		}
	}
}

func (h *ConsentEventHub) pingClients() {
	h.clientsLock.RLock()
	defer h.clientsLock.RUnlock()

	for _, client := range h.clients {
		if err := client.conn.WriteControl(
			websocket.PingMessage,
			nil,
			time.Now().Add(10*time.Second),
		); err != nil {
			h.logger.Error("Failed to ping client",
				zap.String("client_id", client.ID.String()),
				zap.Error(err),
			)
			go func(c *ConsentClient) {
				h.unregister <- c
			}(client)
		}
	}
}

func (h *ConsentEventHub) shutdown() {
	h.clientsLock.Lock()
	defer h.clientsLock.Unlock()

	for _, client := range h.clients {
		close(client.send)
		client.conn.Close()
	}
	h.clients = make(map[uuid.UUID]*ConsentClient)
}

// ConsentClient methods

// NewConsentClient creates a new consent WebSocket client
func NewConsentClient(conn *websocket.Conn, hub *ConsentEventHub, userID uuid.UUID, role string) *ConsentClient {
	return &ConsentClient{
		ID:            uuid.New(),
		conn:          conn,
		send:          make(chan *ConsentEvent, 10),
		hub:           hub,
		authenticated: true,
		userID:        userID,
		role:          role,
	}
}

// ReadPump pumps messages from the WebSocket connection to the hub
func (c *ConsentClient) ReadPump() {
	defer func() {
		c.hub.UnregisterClient(c)
		c.conn.Close()
	}()

	c.conn.SetReadLimit(512)
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.hub.logger.Error("WebSocket read error",
					zap.String("client_id", c.ID.String()),
					zap.Error(err),
				)
			}
			break
		}

		// Handle client messages (e.g., filter updates)
		var msg map[string]interface{}
		if err := json.Unmarshal(message, &msg); err != nil {
			c.hub.logger.Error("Failed to parse client message",
				zap.String("client_id", c.ID.String()),
				zap.Error(err),
			)
			continue
		}

		if msgType, ok := msg["type"].(string); ok {
			switch msgType {
			case "update_filters":
				c.updateFilters(msg)
			case "ping":
				// Respond with pong
				pong := &ConsentEvent{
					ID:        uuid.New().String(),
					Type:      "pong",
					Timestamp: time.Now(),
				}
				select {
				case c.send <- pong:
				default:
				}
			}
		}
	}
}

// WritePump pumps messages from the hub to the WebSocket connection
func (c *ConsentClient) WritePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case event, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteJSON(event); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *ConsentClient) shouldReceiveEvent(event *ConsentEvent) bool {
	// Check authentication
	if !c.authenticated {
		return false
	}

	// Check role-based access
	if c.role != "admin" && c.role != "buyer" {
		return false
	}

	// Apply filters
	if len(c.filters.EventTypes) > 0 {
		found := false
		for _, et := range c.filters.EventTypes {
			if et == event.Type {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	if len(c.filters.ConsumerIDs) > 0 {
		consumerID, _ := uuid.Parse(event.ConsumerID)
		found := false
		for _, id := range c.filters.ConsumerIDs {
			if id == consumerID {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	if len(c.filters.ConsentTypes) > 0 {
		found := false
		for _, ct := range c.filters.ConsentTypes {
			if string(ct) == event.ConsentType {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	if len(c.filters.Channels) > 0 {
		found := false
		for _, ch := range c.filters.Channels {
			if string(ch) == event.Channel {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

func (c *ConsentClient) updateFilters(msg map[string]interface{}) {
	if filters, ok := msg["filters"].(map[string]interface{}); ok {
		// Update consumer ID filters
		if consumerIDs, ok := filters["consumer_ids"].([]interface{}); ok {
			c.filters.ConsumerIDs = make([]uuid.UUID, 0, len(consumerIDs))
			for _, id := range consumerIDs {
				if strID, ok := id.(string); ok {
					if uid, err := uuid.Parse(strID); err == nil {
						c.filters.ConsumerIDs = append(c.filters.ConsumerIDs, uid)
					}
				}
			}
		}

		// Update event type filters
		if eventTypes, ok := filters["event_types"].([]interface{}); ok {
			c.filters.EventTypes = make([]ConsentEventType, 0, len(eventTypes))
			for _, et := range eventTypes {
				if strET, ok := et.(string); ok {
					c.filters.EventTypes = append(c.filters.EventTypes, ConsentEventType(strET))
				}
			}
		}

		// Update consent type filters
		if consentTypes, ok := filters["consent_types"].([]interface{}); ok {
			c.filters.ConsentTypes = make([]consent.Type, 0, len(consentTypes))
			for _, ct := range consentTypes {
				if strCT, ok := ct.(string); ok {
					c.filters.ConsentTypes = append(c.filters.ConsentTypes, consent.Type(strCT))
				}
			}
		}

		// Update channel filters
		if channels, ok := filters["channels"].([]interface{}); ok {
			c.filters.Channels = make([]consent.Channel, 0, len(channels))
			for _, ch := range channels {
				if strCH, ok := ch.(string); ok {
					c.filters.Channels = append(c.filters.Channels, consent.Channel(strCH))
				}
			}
		}

		c.hub.logger.Info("Client filters updated",
			zap.String("client_id", c.ID.String()),
			zap.Any("filters", c.filters),
		)
	}
}