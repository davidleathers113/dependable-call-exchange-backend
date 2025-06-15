package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/audit"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// AuditEventType represents the type of audit event for real-time streaming
type AuditEventType string

const (
	// Event notification types
	AuditEventPublished       AuditEventType = "audit.event.published"
	AuditEventSecurityAlert   AuditEventType = "audit.security.alert"
	AuditEventComplianceAlert AuditEventType = "audit.compliance.alert"
	AuditEventSystemAlert     AuditEventType = "audit.system.alert"

	// Connection management types
	AuditConnectionEstablished AuditEventType = "audit.connection.established"
	AuditConnectionPing        AuditEventType = "audit.connection.ping"
	AuditConnectionPong        AuditEventType = "audit.connection.pong"
)

// AuditStreamEvent represents a real-time audit event for WebSocket streaming
type AuditStreamEvent struct {
	ID         string                 `json:"id"`
	Type       AuditEventType         `json:"type"`
	AuditEvent *audit.Event           `json:"audit_event,omitempty"`
	Severity   string                 `json:"severity"`
	Timestamp  time.Time              `json:"timestamp"`
	Summary    string                 `json:"summary"`
	AlertData  interface{}            `json:"alert_data,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// AuditEventHub manages WebSocket connections for real-time audit event streaming
// Supports 1000+ concurrent connections with efficient filtering and broadcasting
type AuditEventHub struct {
	logger      *zap.Logger
	clients     map[uuid.UUID]*AuditClient
	clientsLock sync.RWMutex
	broadcast   chan *AuditStreamEvent
	register    chan *AuditClient
	unregister  chan *AuditClient
	done        chan struct{}

	// Performance metrics
	metrics AuditHubMetrics

	// Configuration
	config AuditHubConfig
}

// AuditClient represents a WebSocket client subscribed to audit events
type AuditClient struct {
	ID            uuid.UUID
	conn          *websocket.Conn
	send          chan *AuditStreamEvent
	hub           *AuditEventHub
	filters       AuditEventFilters
	authenticated bool
	userID        uuid.UUID
	role          string
	permissions   []string
	connectedAt   time.Time
	lastActivity  time.Time

	// Rate limiting
	rateLimiter *ClientRateLimiter

	// Connection health
	lastPong     time.Time
	pingCount    int64
	messageCount int64
	errorCount   int64
}

// AuditEventFilters defines comprehensive filters for audit events
type AuditEventFilters struct {
	EventTypes     []audit.EventType      `json:"event_types,omitempty"`
	Severities     []audit.Severity       `json:"severities,omitempty"`
	Categories     []string               `json:"categories,omitempty"`
	ActorIDs       []string               `json:"actor_ids,omitempty"`
	TargetIDs      []string               `json:"target_ids,omitempty"`
	Actions        []string               `json:"actions,omitempty"`
	Results        []string               `json:"results,omitempty"`
	Services       []string               `json:"services,omitempty"`
	Environments   []string               `json:"environments,omitempty"`
	TimeRange      *TimeRangeFilter       `json:"time_range,omitempty"`
	ComplianceOnly bool                   `json:"compliance_only,omitempty"`
	SecurityOnly   bool                   `json:"security_only,omitempty"`
	CustomFilters  map[string]interface{} `json:"custom_filters,omitempty"`
}

// TimeRangeFilter defines time-based filtering
type TimeRangeFilter struct {
	Start    time.Time `json:"start"`
	End      time.Time `json:"end"`
	Relative string    `json:"relative,omitempty"` // "1h", "24h", "7d"
}

// ClientRateLimiter manages per-client rate limiting
type ClientRateLimiter struct {
	maxEventsPerSecond int
	maxEventsPerMinute int
	windowSize         time.Duration
	events             []time.Time
	mutex              sync.Mutex
}

// AuditHubConfig configures the audit event hub
type AuditHubConfig struct {
	MaxClients          int           `json:"max_clients"`
	BroadcastBufferSize int           `json:"broadcast_buffer_size"`
	ClientBufferSize    int           `json:"client_buffer_size"`
	PingInterval        time.Duration `json:"ping_interval"`
	PongTimeout         time.Duration `json:"pong_timeout"`
	ReadTimeout         time.Duration `json:"read_timeout"`
	WriteTimeout        time.Duration `json:"write_timeout"`
	MaxMessageSize      int64         `json:"max_message_size"`
	RateLimitPerSecond  int           `json:"rate_limit_per_second"`
	RateLimitPerMinute  int           `json:"rate_limit_per_minute"`
	CleanupInterval     time.Duration `json:"cleanup_interval"`
	MetricsInterval     time.Duration `json:"metrics_interval"`
	EnableCompression   bool          `json:"enable_compression"`
	MaxFiltersPerClient int           `json:"max_filters_per_client"`
}

// AuditHubMetrics tracks performance metrics for the audit hub
type AuditHubMetrics struct {
	mu                    sync.RWMutex
	TotalConnections      int64
	ActiveConnections     int64
	TotalEventsPublished  int64
	TotalEventsFiltered   int64
	TotalEventsDropped    int64
	TotalBytesTransferred int64
	AverageLatency        time.Duration
	PeakConnections       int64
	ErrorCount            int64
	StartTime             time.Time
}

// DefaultAuditHubConfig returns default configuration optimized for 1000+ connections
func DefaultAuditHubConfig() AuditHubConfig {
	return AuditHubConfig{
		MaxClients:          2000,
		BroadcastBufferSize: 10000,
		ClientBufferSize:    256,
		PingInterval:        30 * time.Second,
		PongTimeout:         60 * time.Second,
		ReadTimeout:         60 * time.Second,
		WriteTimeout:        10 * time.Second,
		MaxMessageSize:      32 * 1024, // 32KB
		RateLimitPerSecond:  100,
		RateLimitPerMinute:  1000,
		CleanupInterval:     5 * time.Minute,
		MetricsInterval:     30 * time.Second,
		EnableCompression:   true,
		MaxFiltersPerClient: 50,
	}
}

// NewAuditEventHub creates a new audit event hub optimized for high-scale streaming
func NewAuditEventHub(logger *zap.Logger, config AuditHubConfig) *AuditEventHub {
	return &AuditEventHub{
		logger:     logger,
		clients:    make(map[uuid.UUID]*AuditClient),
		broadcast:  make(chan *AuditStreamEvent, config.BroadcastBufferSize),
		register:   make(chan *AuditClient),
		unregister: make(chan *AuditClient),
		done:       make(chan struct{}),
		config:     config,
		metrics: AuditHubMetrics{
			StartTime: time.Now(),
		},
	}
}

// Run starts the audit event hub with optimized goroutine management
func (h *AuditEventHub) Run(ctx context.Context) {
	h.logger.Info("Starting audit event hub",
		zap.Int("max_clients", h.config.MaxClients),
		zap.Int("broadcast_buffer", h.config.BroadcastBufferSize),
		zap.Duration("ping_interval", h.config.PingInterval),
	)

	// Start multiple goroutines for scalability
	go h.clientManager(ctx)
	go h.broadcastManager(ctx)
	go h.healthMonitor(ctx)
	go h.metricsCollector(ctx)
	go h.cleanupWorker(ctx)

	<-ctx.Done()
	h.shutdown()
}

// Stop gracefully shuts down the hub
func (h *AuditEventHub) Stop() {
	close(h.done)
}

// BroadcastAuditEvent broadcasts an audit event to all matching subscribers
func (h *AuditEventHub) BroadcastAuditEvent(auditEvent *audit.Event) {
	streamEvent := &AuditStreamEvent{
		ID:         uuid.New().String(),
		Type:       AuditEventPublished,
		AuditEvent: auditEvent,
		Severity:   string(auditEvent.Severity),
		Timestamp:  time.Now(),
		Summary:    h.generateEventSummary(auditEvent),
		Metadata: map[string]interface{}{
			"event_id":  auditEvent.ID.String(),
			"actor_id":  auditEvent.ActorID,
			"target_id": auditEvent.TargetID,
			"action":    auditEvent.Action,
			"result":    auditEvent.Result,
		},
	}

	h.broadcastEvent(streamEvent)
}

// BroadcastSecurityAlert broadcasts a security alert
func (h *AuditEventHub) BroadcastSecurityAlert(alertData interface{}, severity string, message string) {
	streamEvent := &AuditStreamEvent{
		ID:        uuid.New().String(),
		Type:      AuditEventSecurityAlert,
		Severity:  severity,
		Timestamp: time.Now(),
		Summary:   message,
		AlertData: alertData,
		Metadata: map[string]interface{}{
			"alert_type": "security",
		},
	}

	h.broadcastEvent(streamEvent)
}

// BroadcastComplianceAlert broadcasts a compliance alert
func (h *AuditEventHub) BroadcastComplianceAlert(alertData interface{}, severity string, message string) {
	streamEvent := &AuditStreamEvent{
		ID:        uuid.New().String(),
		Type:      AuditEventComplianceAlert,
		Severity:  severity,
		Timestamp: time.Now(),
		Summary:   message,
		AlertData: alertData,
		Metadata: map[string]interface{}{
			"alert_type": "compliance",
		},
	}

	h.broadcastEvent(streamEvent)
}

// RegisterClient registers a new WebSocket client with authentication and rate limiting
func (h *AuditEventHub) RegisterClient(client *AuditClient) error {
	// Check connection limits
	h.clientsLock.RLock()
	if len(h.clients) >= h.config.MaxClients {
		h.clientsLock.RUnlock()
		return errors.NewBusinessError("MAX_CLIENTS_REACHED",
			fmt.Sprintf("maximum clients reached: %d", h.config.MaxClients))
	}
	h.clientsLock.RUnlock()

	select {
	case h.register <- client:
		return nil
	case <-time.After(5 * time.Second):
		return errors.NewInternalError("client registration timeout")
	}
}

// UnregisterClient unregisters a WebSocket client
func (h *AuditEventHub) UnregisterClient(client *AuditClient) {
	select {
	case h.unregister <- client:
	default:
		// Non-blocking unregister
	}
}

// GetClientCount returns the current number of connected clients
func (h *AuditEventHub) GetClientCount() int {
	h.clientsLock.RLock()
	defer h.clientsLock.RUnlock()
	return len(h.clients)
}

// GetMetrics returns current hub metrics
func (h *AuditEventHub) GetMetrics() AuditHubMetrics {
	h.metrics.mu.RLock()
	defer h.metrics.mu.RUnlock()

	// Create a copy to avoid race conditions
	return AuditHubMetrics{
		TotalConnections:      h.metrics.TotalConnections,
		ActiveConnections:     h.metrics.ActiveConnections,
		TotalEventsPublished:  h.metrics.TotalEventsPublished,
		TotalEventsFiltered:   h.metrics.TotalEventsFiltered,
		TotalEventsDropped:    h.metrics.TotalEventsDropped,
		TotalBytesTransferred: h.metrics.TotalBytesTransferred,
		AverageLatency:        h.metrics.AverageLatency,
		PeakConnections:       h.metrics.PeakConnections,
		ErrorCount:            h.metrics.ErrorCount,
		StartTime:             h.metrics.StartTime,
	}
}

// GetConnectedClients returns information about connected clients
func (h *AuditEventHub) GetConnectedClients() []AuditClientInfo {
	h.clientsLock.RLock()
	defer h.clientsLock.RUnlock()

	clients := make([]AuditClientInfo, 0, len(h.clients))
	for _, client := range h.clients {
		info := AuditClientInfo{
			ClientID:     client.ID.String(),
			UserID:       client.userID.String(),
			Role:         client.role,
			ConnectedAt:  client.connectedAt,
			LastActivity: client.lastActivity,
			MessageCount: client.messageCount,
			FilterCount:  h.countClientFilters(client.filters),
			RateLimited:  h.isClientRateLimited(client),
		}
		clients = append(clients, info)
	}

	return clients
}

// Private methods

func (h *AuditEventHub) clientManager(ctx context.Context) {
	for {
		select {
		case client := <-h.register:
			h.registerClient(client)
		case client := <-h.unregister:
			h.unregisterClient(client)
		case <-ctx.Done():
			return
		}
	}
}

func (h *AuditEventHub) broadcastManager(ctx context.Context) {
	for {
		select {
		case event := <-h.broadcast:
			h.processEventBroadcast(event)
		case <-ctx.Done():
			return
		}
	}
}

func (h *AuditEventHub) healthMonitor(ctx context.Context) {
	ticker := time.NewTicker(h.config.PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			h.pingClients()
		case <-ctx.Done():
			return
		}
	}
}

func (h *AuditEventHub) metricsCollector(ctx context.Context) {
	ticker := time.NewTicker(h.config.MetricsInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			h.updateMetrics()
		case <-ctx.Done():
			return
		}
	}
}

func (h *AuditEventHub) cleanupWorker(ctx context.Context) {
	ticker := time.NewTicker(h.config.CleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			h.cleanupStaleConnections()
		case <-ctx.Done():
			return
		}
	}
}

func (h *AuditEventHub) registerClient(client *AuditClient) {
	h.clientsLock.Lock()
	defer h.clientsLock.Unlock()

	h.clients[client.ID] = client

	h.metrics.mu.Lock()
	h.metrics.TotalConnections++
	h.metrics.ActiveConnections = int64(len(h.clients))
	if h.metrics.ActiveConnections > h.metrics.PeakConnections {
		h.metrics.PeakConnections = h.metrics.ActiveConnections
	}
	h.metrics.mu.Unlock()

	h.logger.Info("Audit WebSocket client registered",
		zap.String("client_id", client.ID.String()),
		zap.String("user_id", client.userID.String()),
		zap.String("role", client.role),
		zap.Int("total_clients", len(h.clients)),
	)

	// Send welcome message
	welcome := &AuditStreamEvent{
		ID:        uuid.New().String(),
		Type:      AuditConnectionEstablished,
		Timestamp: time.Now(),
		Summary:   "Connected to audit event stream",
		Metadata: map[string]interface{}{
			"client_id":     client.ID.String(),
			"max_clients":   h.config.MaxClients,
			"rate_limit":    h.config.RateLimitPerSecond,
			"ping_interval": h.config.PingInterval.String(),
		},
	}

	select {
	case client.send <- welcome:
	default:
		// Client channel full on welcome - this is unusual but non-fatal
		h.logger.Warn("Client channel full on welcome message",
			zap.String("client_id", client.ID.String()),
		)
	}
}

func (h *AuditEventHub) unregisterClient(client *AuditClient) {
	h.clientsLock.Lock()
	defer h.clientsLock.Unlock()

	if _, exists := h.clients[client.ID]; exists {
		delete(h.clients, client.ID)
		close(client.send)

		h.metrics.mu.Lock()
		h.metrics.ActiveConnections = int64(len(h.clients))
		h.metrics.mu.Unlock()

		h.logger.Info("Audit WebSocket client unregistered",
			zap.String("client_id", client.ID.String()),
			zap.Int("remaining_clients", len(h.clients)),
		)
	}
}

func (h *AuditEventHub) broadcastEvent(event *AuditStreamEvent) {
	select {
	case h.broadcast <- event:
		// Event queued for broadcast
	default:
		// Broadcast buffer full - drop event and record metric
		h.metrics.mu.Lock()
		h.metrics.TotalEventsDropped++
		h.metrics.mu.Unlock()

		h.logger.Warn("Broadcast buffer full, dropping audit event",
			zap.String("event_id", event.ID),
			zap.String("event_type", string(event.Type)),
		)
	}
}

func (h *AuditEventHub) processEventBroadcast(event *AuditStreamEvent) {
	start := time.Now()

	h.clientsLock.RLock()
	defer h.clientsLock.RUnlock()

	sentCount := 0
	filteredCount := 0

	for _, client := range h.clients {
		if h.shouldClientReceiveEvent(client, event) {
			select {
			case client.send <- event:
				sentCount++
				client.messageCount++
				client.lastActivity = time.Now()
			default:
				// Client channel full - mark for disconnection
				h.logger.Warn("Client channel full, scheduling disconnection",
					zap.String("client_id", client.ID.String()),
				)
				go func(c *AuditClient) {
					h.unregister <- c
				}(client)
			}
		} else {
			filteredCount++
		}
	}

	// Update metrics
	h.metrics.mu.Lock()
	h.metrics.TotalEventsPublished++
	h.metrics.TotalEventsFiltered += int64(filteredCount)
	latency := time.Since(start)
	if h.metrics.AverageLatency == 0 {
		h.metrics.AverageLatency = latency
	} else {
		h.metrics.AverageLatency = (h.metrics.AverageLatency + latency) / 2
	}
	h.metrics.mu.Unlock()

	h.logger.Debug("Audit event broadcasted",
		zap.String("event_id", event.ID),
		zap.String("event_type", string(event.Type)),
		zap.Int("sent_to_clients", sentCount),
		zap.Int("filtered_out", filteredCount),
		zap.Duration("latency", latency),
	)
}

func (h *AuditEventHub) shouldClientReceiveEvent(client *AuditClient, event *AuditStreamEvent) bool {
	// Check authentication
	if !client.authenticated {
		return false
	}

	// Check rate limiting
	if h.isClientRateLimited(client) {
		return false
	}

	// Check role-based access for sensitive events
	if !h.hasPermissionForEvent(client, event) {
		return false
	}

	// Apply filters
	if event.AuditEvent != nil {
		return h.applyFilters(client.filters, event.AuditEvent)
	}

	// For non-audit events (alerts, system messages), check basic filters
	return h.applyBasicFilters(client.filters, event)
}

func (h *AuditEventHub) hasPermissionForEvent(client *AuditClient, event *AuditStreamEvent) bool {
	// Admin can see everything
	if client.role == "admin" {
		return true
	}

	// Security personnel can see security and compliance alerts
	if client.role == "security" {
		return event.Type == AuditEventSecurityAlert ||
			event.Type == AuditEventComplianceAlert ||
			event.Type == AuditEventPublished
	}

	// Compliance officers can see compliance alerts and audit events
	if client.role == "compliance" {
		return event.Type == AuditEventComplianceAlert ||
			event.Type == AuditEventPublished
	}

	// Operators can see system alerts and general audit events
	if client.role == "operator" {
		return event.Type == AuditEventSystemAlert ||
			event.Type == AuditEventPublished
	}

	// Auditors can see all audit events but not real-time alerts
	if client.role == "auditor" {
		return event.Type == AuditEventPublished
	}

	// Default: only connection management events
	return event.Type == AuditConnectionEstablished ||
		event.Type == AuditConnectionPing ||
		event.Type == AuditConnectionPong
}

func (h *AuditEventHub) applyFilters(filters AuditEventFilters, auditEvent *audit.Event) bool {
	// Event type filter
	if len(filters.EventTypes) > 0 {
		found := false
		for _, et := range filters.EventTypes {
			if et == auditEvent.Type {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Severity filter
	if len(filters.Severities) > 0 {
		found := false
		for _, severity := range filters.Severities {
			if severity == auditEvent.Severity {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Category filter
	if len(filters.Categories) > 0 {
		found := false
		for _, category := range filters.Categories {
			if category == auditEvent.Category {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Actor ID filter
	if len(filters.ActorIDs) > 0 {
		found := false
		for _, actorID := range filters.ActorIDs {
			if actorID == auditEvent.ActorID {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Target ID filter
	if len(filters.TargetIDs) > 0 {
		found := false
		for _, targetID := range filters.TargetIDs {
			if targetID == auditEvent.TargetID {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Action filter
	if len(filters.Actions) > 0 {
		found := false
		for _, action := range filters.Actions {
			if action == auditEvent.Action {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Result filter
	if len(filters.Results) > 0 {
		found := false
		for _, result := range filters.Results {
			if result == auditEvent.Result {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Service filter
	if len(filters.Services) > 0 {
		found := false
		for _, service := range filters.Services {
			if service == auditEvent.Service {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Environment filter
	if len(filters.Environments) > 0 {
		found := false
		for _, env := range filters.Environments {
			if env == auditEvent.Environment {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Time range filter
	if filters.TimeRange != nil {
		if !auditEvent.Timestamp.After(filters.TimeRange.Start) ||
			!auditEvent.Timestamp.Before(filters.TimeRange.End) {
			return false
		}
	}

	// Compliance only filter
	if filters.ComplianceOnly && !auditEvent.IsGDPRRelevant() && !auditEvent.IsTCPARelevant() {
		return false
	}

	// Security only filter
	if filters.SecurityOnly && auditEvent.Category != "security" {
		return false
	}

	return true
}

func (h *AuditEventHub) applyBasicFilters(filters AuditEventFilters, event *AuditStreamEvent) bool {
	// For non-audit events, apply basic filtering

	// Time range filter
	if filters.TimeRange != nil {
		if !event.Timestamp.After(filters.TimeRange.Start) ||
			!event.Timestamp.Before(filters.TimeRange.End) {
			return false
		}
	}

	// Severity filter for alerts
	if len(filters.Severities) > 0 && event.Severity != "" {
		found := false
		for _, severity := range filters.Severities {
			if string(severity) == event.Severity {
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

func (h *AuditEventHub) pingClients() {
	h.clientsLock.RLock()
	defer h.clientsLock.RUnlock()

	for _, client := range h.clients {
		if err := client.conn.WriteControl(
			websocket.PingMessage,
			nil,
			time.Now().Add(h.config.WriteTimeout),
		); err != nil {
			h.logger.Error("Failed to ping audit client",
				zap.String("client_id", client.ID.String()),
				zap.Error(err),
			)
			go func(c *AuditClient) {
				h.unregister <- c
			}(client)
		} else {
			client.pingCount++
		}
	}
}

func (h *AuditEventHub) cleanupStaleConnections() {
	h.clientsLock.RLock()
	staleClients := make([]*AuditClient, 0)

	for _, client := range h.clients {
		// Check for stale connections
		if time.Since(client.lastPong) > h.config.PongTimeout {
			staleClients = append(staleClients, client)
		}
	}
	h.clientsLock.RUnlock()

	// Remove stale clients
	for _, client := range staleClients {
		h.logger.Info("Removing stale audit client connection",
			zap.String("client_id", client.ID.String()),
			zap.Duration("last_pong", time.Since(client.lastPong)),
		)
		h.unregister <- client
	}
}

func (h *AuditEventHub) updateMetrics() {
	h.clientsLock.RLock()
	activeClients := int64(len(h.clients))
	h.clientsLock.RUnlock()

	h.metrics.mu.Lock()
	h.metrics.ActiveConnections = activeClients
	h.metrics.mu.Unlock()
}

func (h *AuditEventHub) isClientRateLimited(client *AuditClient) bool {
	if client.rateLimiter == nil {
		return false
	}
	return client.rateLimiter.IsLimited()
}

func (h *AuditEventHub) countClientFilters(filters AuditEventFilters) int {
	count := len(filters.EventTypes) + len(filters.Severities) + len(filters.Categories) +
		len(filters.ActorIDs) + len(filters.TargetIDs) + len(filters.Actions) +
		len(filters.Results) + len(filters.Services) + len(filters.Environments) +
		len(filters.CustomFilters)

	if filters.TimeRange != nil {
		count++
	}
	if filters.ComplianceOnly {
		count++
	}
	if filters.SecurityOnly {
		count++
	}

	return count
}

func (h *AuditEventHub) generateEventSummary(auditEvent *audit.Event) string {
	return fmt.Sprintf("%s: %s performed %s on %s with result %s",
		auditEvent.Type, auditEvent.ActorID, auditEvent.Action,
		auditEvent.TargetID, auditEvent.Result)
}

func (h *AuditEventHub) shutdown() {
	h.logger.Info("Shutting down audit event hub")

	h.clientsLock.Lock()
	defer h.clientsLock.Unlock()

	for _, client := range h.clients {
		close(client.send)
		client.conn.Close()
	}
	h.clients = make(map[uuid.UUID]*AuditClient)
}

// AuditClientInfo provides information about a connected client
type AuditClientInfo struct {
	ClientID     string    `json:"client_id"`
	UserID       string    `json:"user_id"`
	Role         string    `json:"role"`
	ConnectedAt  time.Time `json:"connected_at"`
	LastActivity time.Time `json:"last_activity"`
	MessageCount int64     `json:"message_count"`
	FilterCount  int       `json:"filter_count"`
	RateLimited  bool      `json:"rate_limited"`
}

// NewAuditClient creates a new audit WebSocket client with rate limiting
func NewAuditClient(
	conn *websocket.Conn,
	hub *AuditEventHub,
	userID uuid.UUID,
	role string,
	permissions []string,
) *AuditClient {
	return &AuditClient{
		ID:            uuid.New(),
		conn:          conn,
		send:          make(chan *AuditStreamEvent, hub.config.ClientBufferSize),
		hub:           hub,
		authenticated: true,
		userID:        userID,
		role:          role,
		permissions:   permissions,
		connectedAt:   time.Now(),
		lastActivity:  time.Now(),
		lastPong:      time.Now(),
		rateLimiter: &ClientRateLimiter{
			maxEventsPerSecond: hub.config.RateLimitPerSecond,
			maxEventsPerMinute: hub.config.RateLimitPerMinute,
			windowSize:         time.Minute,
			events:             make([]time.Time, 0),
		},
	}
}

// ReadPump pumps messages from the WebSocket connection to the hub
func (c *AuditClient) ReadPump() {
	defer func() {
		c.hub.UnregisterClient(c)
		c.conn.Close()
	}()

	c.conn.SetReadLimit(c.hub.config.MaxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(c.hub.config.ReadTimeout))
	c.conn.SetPongHandler(func(string) error {
		c.lastPong = time.Now()
		c.conn.SetReadDeadline(time.Now().Add(c.hub.config.ReadTimeout))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.hub.logger.Error("Audit WebSocket read error",
					zap.String("client_id", c.ID.String()),
					zap.Error(err),
				)
				c.errorCount++
			}
			break
		}

		c.lastActivity = time.Now()

		// Handle client messages (filter updates, ping/pong)
		var msg map[string]interface{}
		if err := json.Unmarshal(message, &msg); err != nil {
			c.hub.logger.Error("Failed to parse audit client message",
				zap.String("client_id", c.ID.String()),
				zap.Error(err),
			)
			c.errorCount++
			continue
		}

		c.handleClientMessage(msg)
	}
}

// WritePump pumps messages from the hub to the WebSocket connection
func (c *AuditClient) WritePump() {
	ticker := time.NewTicker(c.hub.config.PingInterval)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case event, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(c.hub.config.WriteTimeout))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			// Enable compression if configured
			if c.hub.config.EnableCompression {
				c.conn.EnableWriteCompression(true)
			}

			if err := c.conn.WriteJSON(event); err != nil {
				c.hub.logger.Error("Failed to write audit event to client",
					zap.String("client_id", c.ID.String()),
					zap.Error(err),
				)
				c.errorCount++
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

func (c *AuditClient) handleClientMessage(msg map[string]interface{}) {
	if msgType, ok := msg["type"].(string); ok {
		switch msgType {
		case "update_filters":
			c.updateFilters(msg)
		case "ping":
			// Respond with pong
			pong := &AuditStreamEvent{
				ID:        uuid.New().String(),
				Type:      AuditConnectionPong,
				Timestamp: time.Now(),
				Summary:   "pong",
			}
			select {
			case c.send <- pong:
			default:
			}
		case "get_stats":
			c.sendClientStats()
		}
	}
}

func (c *AuditClient) updateFilters(msg map[string]interface{}) {
	if filters, ok := msg["filters"].(map[string]interface{}); ok {
		// Validate filter count
		if c.hub.countClientFilters(c.filters) > c.hub.config.MaxFiltersPerClient {
			c.hub.logger.Warn("Client exceeded maximum filter count",
				zap.String("client_id", c.ID.String()),
				zap.Int("filter_count", c.hub.countClientFilters(c.filters)),
				zap.Int("max_allowed", c.hub.config.MaxFiltersPerClient),
			)
			return
		}

		// Update filters (implementation similar to consent_events.go but for audit filters)
		c.parseAndSetFilters(filters)

		c.hub.logger.Info("Audit client filters updated",
			zap.String("client_id", c.ID.String()),
			zap.Int("filter_count", c.hub.countClientFilters(c.filters)),
		)
	}
}

func (c *AuditClient) parseAndSetFilters(filters map[string]interface{}) {
	// Parse event types
	if eventTypes, ok := filters["event_types"].([]interface{}); ok {
		c.filters.EventTypes = make([]audit.EventType, 0, len(eventTypes))
		for _, et := range eventTypes {
			if strET, ok := et.(string); ok {
				c.filters.EventTypes = append(c.filters.EventTypes, audit.EventType(strET))
			}
		}
	}

	// Parse severities
	if severities, ok := filters["severities"].([]interface{}); ok {
		c.filters.Severities = make([]audit.Severity, 0, len(severities))
		for _, sev := range severities {
			if strSev, ok := sev.(string); ok {
				c.filters.Severities = append(c.filters.Severities, audit.Severity(strSev))
			}
		}
	}

	// Parse string array filters
	c.parseStringArrayFilter(filters, "categories", &c.filters.Categories)
	c.parseStringArrayFilter(filters, "actor_ids", &c.filters.ActorIDs)
	c.parseStringArrayFilter(filters, "target_ids", &c.filters.TargetIDs)
	c.parseStringArrayFilter(filters, "actions", &c.filters.Actions)
	c.parseStringArrayFilter(filters, "results", &c.filters.Results)
	c.parseStringArrayFilter(filters, "services", &c.filters.Services)
	c.parseStringArrayFilter(filters, "environments", &c.filters.Environments)

	// Parse boolean filters
	if complianceOnly, ok := filters["compliance_only"].(bool); ok {
		c.filters.ComplianceOnly = complianceOnly
	}

	if securityOnly, ok := filters["security_only"].(bool); ok {
		c.filters.SecurityOnly = securityOnly
	}

	// Parse time range filter
	if timeRange, ok := filters["time_range"].(map[string]interface{}); ok {
		c.filters.TimeRange = &TimeRangeFilter{}

		if start, ok := timeRange["start"].(string); ok {
			if startTime, err := time.Parse(time.RFC3339, start); err == nil {
				c.filters.TimeRange.Start = startTime
			}
		}

		if end, ok := timeRange["end"].(string); ok {
			if endTime, err := time.Parse(time.RFC3339, end); err == nil {
				c.filters.TimeRange.End = endTime
			}
		}

		if relative, ok := timeRange["relative"].(string); ok {
			c.filters.TimeRange.Relative = relative
			// Parse relative time and set Start/End accordingly
			c.parseRelativeTimeRange(relative)
		}
	}

	// Parse custom filters
	if customFilters, ok := filters["custom_filters"].(map[string]interface{}); ok {
		c.filters.CustomFilters = customFilters
	}
}

func (c *AuditClient) parseStringArrayFilter(filters map[string]interface{}, key string, target *[]string) {
	if values, ok := filters[key].([]interface{}); ok {
		*target = make([]string, 0, len(values))
		for _, value := range values {
			if strValue, ok := value.(string); ok {
				*target = append(*target, strValue)
			}
		}
	}
}

func (c *AuditClient) parseRelativeTimeRange(relative string) {
	now := time.Now()

	switch relative {
	case "1h":
		c.filters.TimeRange.Start = now.Add(-time.Hour)
		c.filters.TimeRange.End = now
	case "24h":
		c.filters.TimeRange.Start = now.Add(-24 * time.Hour)
		c.filters.TimeRange.End = now
	case "7d":
		c.filters.TimeRange.Start = now.Add(-7 * 24 * time.Hour)
		c.filters.TimeRange.End = now
	case "30d":
		c.filters.TimeRange.Start = now.Add(-30 * 24 * time.Hour)
		c.filters.TimeRange.End = now
	}
}

func (c *AuditClient) sendClientStats() {
	stats := &AuditStreamEvent{
		ID:        uuid.New().String(),
		Type:      "client.stats",
		Timestamp: time.Now(),
		Summary:   "Client statistics",
		Metadata: map[string]interface{}{
			"client_id":     c.ID.String(),
			"connected_at":  c.connectedAt,
			"last_activity": c.lastActivity,
			"message_count": c.messageCount,
			"ping_count":    c.pingCount,
			"error_count":   c.errorCount,
			"filter_count":  c.hub.countClientFilters(c.filters),
		},
	}

	select {
	case c.send <- stats:
	default:
	}
}

// ClientRateLimiter methods

func (rl *ClientRateLimiter) IsLimited() bool {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	now := time.Now()

	// Clean old events outside the window
	cutoff := now.Add(-rl.windowSize)
	validEvents := make([]time.Time, 0, len(rl.events))
	for _, eventTime := range rl.events {
		if eventTime.After(cutoff) {
			validEvents = append(validEvents, eventTime)
		}
	}
	rl.events = validEvents

	// Check limits
	eventsInLastSecond := 0
	secondCutoff := now.Add(-time.Second)

	for _, eventTime := range rl.events {
		if eventTime.After(secondCutoff) {
			eventsInLastSecond++
		}
	}

	return eventsInLastSecond >= rl.maxEventsPerSecond ||
		len(rl.events) >= rl.maxEventsPerMinute
}

func (rl *ClientRateLimiter) RecordEvent() {
	rl.mutex.Lock()
	defer rl.mutex.Unlock()

	rl.events = append(rl.events, time.Now())
}
