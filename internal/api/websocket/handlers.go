package websocket

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// Upgrader configuration for WebSocket connections
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// TODO: Implement proper origin checking
		return true
	},
}

// Handler manages WebSocket endpoints
type Handler struct {
	logger          *zap.Logger
	consentEventHub *ConsentEventHub
}

// NewHandler creates a new WebSocket handler
func NewHandler(logger *zap.Logger) *Handler {
	return &Handler{
		logger:          logger,
		consentEventHub: NewConsentEventHub(logger),
	}
}

// Start initializes the WebSocket handler
func (h *Handler) Start(ctx context.Context) {
	go h.consentEventHub.Run(ctx)
}

// Stop gracefully shuts down the WebSocket handler
func (h *Handler) Stop() {
	h.consentEventHub.Stop()
}

// GetConsentEventHub returns the consent event hub for publishing events
func (h *Handler) GetConsentEventHub() *ConsentEventHub {
	return h.consentEventHub
}

// HandleConsentEvents handles WebSocket connections for consent events
func (h *Handler) HandleConsentEvents(w http.ResponseWriter, r *http.Request) {
	// Extract user information from request context
	// In a real implementation, this would come from authentication middleware
	userID, _ := r.Context().Value("user_id").(uuid.UUID)
	role, _ := r.Context().Value("role").(string)
	
	// Default values for development/testing
	if userID == uuid.Nil {
		userID = uuid.New() // Generate a test user ID
		role = "buyer"       // Default role
	}

	// Upgrade HTTP connection to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("Failed to upgrade WebSocket connection", 
			zap.Error(err),
			zap.String("remote_addr", r.RemoteAddr),
		)
		return
	}

	// Create new client
	client := NewConsentClient(conn, h.consentEventHub, userID, role)

	// Register client with hub
	h.consentEventHub.RegisterClient(client)

	// Allow collection of memory referenced by the caller by doing all work in new goroutines
	go client.WritePump()
	go client.ReadPump()

	h.logger.Info("New WebSocket connection established",
		zap.String("client_id", client.ID.String()),
		zap.String("user_id", userID.String()),
		zap.String("role", role),
		zap.String("remote_addr", r.RemoteAddr),
	)
}

// WebSocketInfo provides information about active WebSocket connections
type WebSocketInfo struct {
	ActiveConnections int                    `json:"active_connections"`
	ConnectedClients  []ConnectedClientInfo  `json:"connected_clients"`
	ServerUptime      time.Duration          `json:"server_uptime"`
}

// ConnectedClientInfo provides information about a connected client
type ConnectedClientInfo struct {
	ClientID      string    `json:"client_id"`
	UserID        string    `json:"user_id"`
	Role          string    `json:"role"`
	ConnectedAt   time.Time `json:"connected_at"`
	FilterCount   int       `json:"filter_count"`
}

// GetWebSocketInfo returns information about active WebSocket connections
func (h *Handler) GetWebSocketInfo() WebSocketInfo {
	h.consentEventHub.clientsLock.RLock()
	defer h.consentEventHub.clientsLock.RUnlock()

	info := WebSocketInfo{
		ActiveConnections: len(h.consentEventHub.clients),
		ConnectedClients:  make([]ConnectedClientInfo, 0, len(h.consentEventHub.clients)),
	}

	for _, client := range h.consentEventHub.clients {
		clientInfo := ConnectedClientInfo{
			ClientID: client.ID.String(),
			UserID:   client.userID.String(),
			Role:     client.role,
			FilterCount: len(client.filters.ConsumerIDs) + 
				len(client.filters.ConsentTypes) + 
				len(client.filters.Channels) + 
				len(client.filters.EventTypes),
		}
		info.ConnectedClients = append(info.ConnectedClients, clientInfo)
	}

	return info
}

// HealthCheck verifies the WebSocket handler is functioning
func (h *Handler) HealthCheck() error {
	// Check if event hub is running
	select {
	case <-h.consentEventHub.done:
		return ErrEventHubNotRunning
	default:
		return nil
	}
}

// Errors
var (
	ErrEventHubNotRunning = &WebSocketError{Code: "WS001", Message: "Event hub is not running"}
)

// WebSocketError represents a WebSocket-specific error
type WebSocketError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *WebSocketError) Error() string {
	return e.Code + ": " + e.Message
}