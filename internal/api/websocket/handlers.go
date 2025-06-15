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
	auditEventHub   *AuditEventHub
	dncEventHub     *DNCEventHub
}

// NewHandler creates a new WebSocket handler
func NewHandler(logger *zap.Logger) *Handler {
	// Initialize audit hub with default config optimized for 1000+ connections
	auditConfig := DefaultAuditHubConfig()
	
	// Initialize DNC hub with default config optimized for 1000+ connections
	dncConfig := DefaultDNCHubConfig()
	
	return &Handler{
		logger:          logger,
		consentEventHub: NewConsentEventHub(logger),
		auditEventHub:   NewAuditEventHub(logger, auditConfig),
		dncEventHub:     NewDNCEventHub(logger, dncConfig),
	}
}

// Start initializes the WebSocket handler
func (h *Handler) Start(ctx context.Context) {
	go h.consentEventHub.Run(ctx)
	go h.auditEventHub.Run(ctx)
	go h.dncEventHub.Run(ctx)
}

// Stop gracefully shuts down the WebSocket handler
func (h *Handler) Stop() {
	h.consentEventHub.Stop()
	h.auditEventHub.Stop()
	h.dncEventHub.Stop()
}

// GetConsentEventHub returns the consent event hub for publishing events
func (h *Handler) GetConsentEventHub() *ConsentEventHub {
	return h.consentEventHub
}

// GetAuditEventHub returns the audit event hub for publishing events
func (h *Handler) GetAuditEventHub() *AuditEventHub {
	return h.auditEventHub
}

// GetDNCEventHub returns the DNC event hub for publishing events
func (h *Handler) GetDNCEventHub() *DNCEventHub {
	return h.dncEventHub
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

// HandleAuditEvents handles WebSocket connections for audit events
func (h *Handler) HandleAuditEvents(w http.ResponseWriter, r *http.Request) {
	// Extract user information from request context (set by authentication middleware)
	userID, _ := r.Context().Value("user_id").(uuid.UUID)
	role, _ := r.Context().Value("role").(string)
	permissions, _ := r.Context().Value("permissions").([]string)
	
	// Default values for development/testing
	if userID == uuid.Nil {
		userID = uuid.New() // Generate a test user ID
		role = "admin"      // Default role with full access
		permissions = []string{"audit:read", "audit:stream", "security:read", "compliance:read"}
	}

	// Validate role permissions for audit access
	if !h.hasAuditPermission(role, permissions) {
		h.logger.Warn("Unauthorized audit event access attempt",
			zap.String("user_id", userID.String()),
			zap.String("role", role),
			zap.Strings("permissions", permissions),
			zap.String("remote_addr", r.RemoteAddr),
		)
		http.Error(w, "Forbidden: Insufficient permissions for audit events", http.StatusForbidden)
		return
	}

	// Upgrade HTTP connection to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("Failed to upgrade audit WebSocket connection", 
			zap.Error(err),
			zap.String("remote_addr", r.RemoteAddr),
			zap.String("user_id", userID.String()),
		)
		return
	}

	// Create new audit client with enhanced configuration
	client := NewAuditClient(conn, h.auditEventHub, userID, role, permissions)

	// Register client with hub (includes connection limit checking)
	if err := h.auditEventHub.RegisterClient(client); err != nil {
		h.logger.Error("Failed to register audit client",
			zap.Error(err),
			zap.String("user_id", userID.String()),
			zap.String("role", role),
		)
		conn.Close()
		return
	}

	// Start client goroutines for bidirectional communication
	go client.WritePump()
	go client.ReadPump()

	h.logger.Info("New audit WebSocket connection established",
		zap.String("client_id", client.ID.String()),
		zap.String("user_id", userID.String()),
		zap.String("role", role),
		zap.Strings("permissions", permissions),
		zap.String("remote_addr", r.RemoteAddr),
		zap.Int("total_clients", h.auditEventHub.GetClientCount()),
	)
}

// HandleDNCEvents handles WebSocket connections for DNC (Do Not Call) events
func (h *Handler) HandleDNCEvents(w http.ResponseWriter, r *http.Request) {
	// Extract user information from request context (set by authentication middleware)
	userID, _ := r.Context().Value("user_id").(uuid.UUID)
	role, _ := r.Context().Value("role").(string)
	permissions, _ := r.Context().Value("permissions").([]string)
	
	// Default values for development/testing
	if userID == uuid.Nil {
		userID = uuid.New() // Generate a test user ID
		role = "compliance" // Default role with DNC access
		permissions = []string{"dnc:read", "dnc:stream", "compliance:read", "telephony:read"}
	}

	// Validate role permissions for DNC access
	if !h.hasDNCPermission(role, permissions) {
		h.logger.Warn("Unauthorized DNC event access attempt",
			zap.String("user_id", userID.String()),
			zap.String("role", role),
			zap.Strings("permissions", permissions),
			zap.String("remote_addr", r.RemoteAddr),
		)
		http.Error(w, "Forbidden: Insufficient permissions for DNC events", http.StatusForbidden)
		return
	}

	// Upgrade HTTP connection to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("Failed to upgrade DNC WebSocket connection", 
			zap.Error(err),
			zap.String("remote_addr", r.RemoteAddr),
			zap.String("user_id", userID.String()),
		)
		return
	}

	// Create new DNC client with enhanced configuration
	client := NewDNCClient(conn, h.dncEventHub, userID, role, permissions)

	// Register client with hub (includes connection limit checking)
	if err := h.dncEventHub.RegisterClient(client); err != nil {
		h.logger.Error("Failed to register DNC client",
			zap.Error(err),
			zap.String("user_id", userID.String()),
			zap.String("role", role),
		)
		conn.Close()
		return
	}

	// Start client goroutines for bidirectional communication
	go client.WritePump()
	go client.ReadPump()

	h.logger.Info("New DNC WebSocket connection established",
		zap.String("client_id", client.ID.String()),
		zap.String("user_id", userID.String()),
		zap.String("role", role),
		zap.Strings("permissions", permissions),
		zap.String("remote_addr", r.RemoteAddr),
		zap.Int("total_clients", h.dncEventHub.GetClientCount()),
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
	consentClients := make([]ConnectedClientInfo, 0, len(h.consentEventHub.clients))
	
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
		consentClients = append(consentClients, clientInfo)
	}
	h.consentEventHub.clientsLock.RUnlock()

	// Get audit clients
	auditClients := h.auditEventHub.GetConnectedClients()
	
	// Get DNC clients
	dncClients := h.dncEventHub.GetConnectedClients()
	
	// Combine all clients
	allClients := make([]ConnectedClientInfo, 0, len(consentClients)+len(auditClients)+len(dncClients))
	allClients = append(allClients, consentClients...)
	
	// Convert audit client info to ConnectedClientInfo format
	for _, auditClient := range auditClients {
		clientInfo := ConnectedClientInfo{
			ClientID:     auditClient.ClientID,
			UserID:       auditClient.UserID,
			Role:         auditClient.Role,
			ConnectedAt:  auditClient.ConnectedAt,
			FilterCount:  auditClient.FilterCount,
		}
		allClients = append(allClients, clientInfo)
	}
	
	// Convert DNC client info to ConnectedClientInfo format
	for _, dncClient := range dncClients {
		clientInfo := ConnectedClientInfo{
			ClientID:     dncClient.ClientID,
			UserID:       dncClient.UserID,
			Role:         dncClient.Role,
			ConnectedAt:  dncClient.ConnectedAt,
			FilterCount:  dncClient.FilterCount,
		}
		allClients = append(allClients, clientInfo)
	}

	info := WebSocketInfo{
		ActiveConnections: len(allClients),
		ConnectedClients:  allClients,
	}

	return info
}

// HealthCheck verifies the WebSocket handler is functioning
func (h *Handler) HealthCheck() error {
	// Check if consent event hub is running
	select {
	case <-h.consentEventHub.done:
		return ErrEventHubNotRunning
	default:
	}
	
	// Check if audit event hub is running
	select {
	case <-h.auditEventHub.done:
		return ErrAuditEventHubNotRunning
	default:
	}
	
	// Check if DNC event hub is running
	select {
	case <-h.dncEventHub.done:
		return ErrDNCEventHubNotRunning
	default:
		return nil
	}
}

// hasAuditPermission checks if the user has permission to access audit events
func (h *Handler) hasAuditPermission(role string, permissions []string) bool {
	// Admin and auditor roles have full access
	if role == "admin" || role == "auditor" {
		return true
	}
	
	// Security and compliance roles have limited access
	if role == "security" || role == "compliance" || role == "operator" {
		return true
	}
	
	// Check specific permissions
	for _, perm := range permissions {
		if perm == "audit:read" || perm == "audit:stream" {
			return true
		}
	}
	
	return false
}

// hasDNCPermission checks if the user has permission to access DNC events
func (h *Handler) hasDNCPermission(role string, permissions []string) bool {
	// Admin role has full access
	if role == "admin" {
		return true
	}
	
	// Compliance and telephony roles have DNC access
	if role == "compliance" || role == "telephony" || role == "operator" {
		return true
	}
	
	// Security role has limited DNC access
	if role == "security" {
		return true
	}
	
	// Check specific permissions
	for _, perm := range permissions {
		if perm == "dnc:read" || perm == "dnc:stream" || perm == "compliance:read" || perm == "telephony:read" {
			return true
		}
	}
	
	return false
}

// Errors
var (
	ErrEventHubNotRunning      = &WebSocketError{Code: "WS001", Message: "Event hub is not running"}
	ErrAuditEventHubNotRunning = &WebSocketError{Code: "WS002", Message: "Audit event hub is not running"}
	ErrDNCEventHubNotRunning   = &WebSocketError{Code: "WS003", Message: "DNC event hub is not running"}
)

// WebSocketError represents a WebSocket-specific error
type WebSocketError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *WebSocketError) Error() string {
	return e.Code + ": " + e.Message
}