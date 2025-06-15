package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sync"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/audit"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/dnc/events"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// DNCEventType represents the type of DNC event for real-time streaming
type DNCEventType string

const (
	// DNC event notification types
	DNCEventNumberSuppressed      DNCEventType = "dnc.number.suppressed"
	DNCEventNumberReleased        DNCEventType = "dnc.number.released"
	DNCEventCheckPerformed        DNCEventType = "dnc.check.performed"
	DNCEventListSynced           DNCEventType = "dnc.list.synced"
	DNCEventComplianceViolation  DNCEventType = "dnc.compliance.violation"
	DNCEventProviderStatusChange DNCEventType = "dnc.provider.status_change"

	// Connection management types
	DNCConnectionEstablished DNCEventType = "dnc.connection.established"
	DNCConnectionPing        DNCEventType = "dnc.connection.ping"
	DNCConnectionPong        DNCEventType = "dnc.connection.pong"
)

// DNCStreamEvent represents a real-time DNC event for WebSocket streaming
type DNCStreamEvent struct {
	ID          string                 `json:"id"`
	Type        DNCEventType           `json:"type"`
	PhoneNumber string                 `json:"phone_number,omitempty"`
	Provider    string                 `json:"provider,omitempty"`
	ProviderID  string                 `json:"provider_id,omitempty"`
	Severity    string                 `json:"severity"`
	Timestamp   time.Time              `json:"timestamp"`
	Summary     string                 `json:"summary"`
	EventData   interface{}            `json:"event_data,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// DNCEventHub manages WebSocket connections for real-time DNC event streaming
// Supports 1000+ concurrent connections with efficient filtering and broadcasting
type DNCEventHub struct {
	logger      *zap.Logger
	clients     map[uuid.UUID]*DNCClient
	clientsLock sync.RWMutex
	broadcast   chan *DNCStreamEvent
	register    chan *DNCClient
	unregister  chan *DNCClient
	done        chan struct{}

	// Performance metrics
	metrics DNCHubMetrics

	// Configuration
	config DNCHubConfig
}

// DNCClient represents a WebSocket client subscribed to DNC events
type DNCClient struct {
	ID            uuid.UUID
	conn          *websocket.Conn
	send          chan *DNCStreamEvent
	hub           *DNCEventHub
	filters       DNCEventFilters
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

// DNCEventFilters defines comprehensive filters for DNC events
type DNCEventFilters struct {
	EventTypes        []DNCEventType         `json:"event_types,omitempty"`
	PhonePatterns     []string               `json:"phone_patterns,omitempty"`
	ProviderIDs       []string               `json:"provider_ids,omitempty"`
	Providers         []string               `json:"providers,omitempty"`
	Severities        []string               `json:"severities,omitempty"`
	CheckReasons      []string               `json:"check_reasons,omitempty"`
	SuppressReasons   []string               `json:"suppress_reasons,omitempty"`
	ReleaseReasons    []string               `json:"release_reasons,omitempty"`
	SyncTriggers      []string               `json:"sync_triggers,omitempty"`
	SyncStatuses      []string               `json:"sync_statuses,omitempty"`
	TimeRange         *TimeRangeFilter       `json:"time_range,omitempty"`
	ComplianceOnly    bool                   `json:"compliance_only,omitempty"`
	HighRiskOnly      bool                   `json:"high_risk_only,omitempty"`
	ErrorsOnly        bool                   `json:"errors_only,omitempty"`
	BlockedOnly       bool                   `json:"blocked_only,omitempty"`
	CustomFilters     map[string]interface{} `json:"custom_filters,omitempty"`
}

// DNCHubConfig configures the DNC event hub
type DNCHubConfig struct {
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

// DNCHubMetrics tracks performance metrics for the DNC hub
type DNCHubMetrics struct {
	mu                     sync.RWMutex
	TotalConnections       int64
	ActiveConnections      int64
	TotalEventsPublished   int64
	TotalEventsFiltered    int64
	TotalEventsDropped     int64
	TotalBytesTransferred  int64
	AverageLatency         time.Duration
	PeakConnections        int64
	ErrorCount             int64
	ComplianceViolations   int64
	HighRiskEvents         int64
	BlockedNumberEvents    int64
	StartTime              time.Time
}

// DefaultDNCHubConfig returns default configuration optimized for 1000+ connections
func DefaultDNCHubConfig() DNCHubConfig {
	return DNCHubConfig{
		MaxClients:          1500,
		BroadcastBufferSize: 8000,
		ClientBufferSize:    256,
		PingInterval:        30 * time.Second,
		PongTimeout:         60 * time.Second,
		ReadTimeout:         60 * time.Second,
		WriteTimeout:        10 * time.Second,
		MaxMessageSize:      32 * 1024, // 32KB
		RateLimitPerSecond:  50,
		RateLimitPerMinute:  500,
		CleanupInterval:     5 * time.Minute,
		MetricsInterval:     30 * time.Second,
		EnableCompression:   true,
		MaxFiltersPerClient: 25,
	}
}

// NewDNCEventHub creates a new DNC event hub optimized for high-scale streaming
func NewDNCEventHub(logger *zap.Logger, config DNCHubConfig) *DNCEventHub {
	return &DNCEventHub{
		logger:     logger,
		clients:    make(map[uuid.UUID]*DNCClient),
		broadcast:  make(chan *DNCStreamEvent, config.BroadcastBufferSize),
		register:   make(chan *DNCClient),
		unregister: make(chan *DNCClient),
		done:       make(chan struct{}),
		config:     config,
		metrics: DNCHubMetrics{
			StartTime: time.Now(),
		},
	}
}

// Run starts the DNC event hub with optimized goroutine management
func (h *DNCEventHub) Run(ctx context.Context) {
	h.logger.Info("Starting DNC event hub",
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
func (h *DNCEventHub) Stop() {
	close(h.done)
}

// BroadcastNumberSuppressed broadcasts a number suppressed event
func (h *DNCEventHub) BroadcastNumberSuppressed(event *events.NumberSuppressedEvent) {
	streamEvent := &DNCStreamEvent{
		ID:          uuid.New().String(),
		Type:        DNCEventNumberSuppressed,
		PhoneNumber: event.PhoneNumber.String(),
		Severity:    h.getSeverityFromReason(string(event.Reason)),
		Timestamp:   time.Now(),
		Summary:     fmt.Sprintf("Phone number %s suppressed: %s", event.PhoneNumber.MaskedString(), event.Reason),
		EventData:   event,
		Metadata: map[string]interface{}{
			"event_id":       event.EventID.String(),
			"phone_number":   event.PhoneNumber.String(),
			"reason":         string(event.Reason),
			"source":         string(event.Source),
			"suppressed_by":  event.SuppressedBy.String(),
			"suppressed_at":  event.SuppressedAt,
			"is_temporary":   event.IsTemporary(),
			"tcpa_relevant":  event.IsTCPARelevant(),
			"gdpr_relevant":  event.IsGDPRRelevant(),
			"dnc_entry_id":   event.DNCEntryID.String(),
		},
	}

	h.broadcastEvent(streamEvent)
}

// BroadcastNumberReleased broadcasts a number released event
func (h *DNCEventHub) BroadcastNumberReleased(event *events.NumberReleasedEvent) {
	streamEvent := &DNCStreamEvent{
		ID:          uuid.New().String(),
		Type:        DNCEventNumberReleased,
		PhoneNumber: event.PhoneNumber.String(),
		Severity:    h.getSeverityFromReleaseReason(string(event.Reason)),
		Timestamp:   time.Now(),
		Summary:     fmt.Sprintf("Phone number %s released: %s", event.PhoneNumber.MaskedString(), event.Reason),
		EventData:   event,
		Metadata: map[string]interface{}{
			"event_id":             event.EventID.String(),
			"phone_number":         event.PhoneNumber.String(),
			"reason":               string(event.Reason),
			"released_by":          event.ReleasedBy.String(),
			"released_at":          event.ReleasedAt,
			"previous_dnc_entry":   event.PreviousDNCEntryID.String(),
			"is_consumer_request":  event.IsConsumerRequested(),
			"is_admin_override":    event.IsAdminOverride(),
			"is_verified":          event.IsVerified(),
			"tcpa_relevant":        event.IsTCPARelevant(),
			"gdpr_relevant":        event.IsGDPRRelevant(),
		},
	}

	h.broadcastEvent(streamEvent)
}

// BroadcastDNCCheckPerformed broadcasts a DNC check performed event
func (h *DNCEventHub) BroadcastDNCCheckPerformed(event *events.DNCCheckPerformedEvent) {
	severity := "info"
	if event.IsNumberBlocked() {
		severity = "warning"
	}
	if event.IsError() || event.IsTimeout() {
		severity = "error"
	}
	if event.IsHighRisk() {
		severity = "high"
	}

	streamEvent := &DNCStreamEvent{
		ID:          uuid.New().String(),
		Type:        DNCEventCheckPerformed,
		PhoneNumber: event.PhoneNumber.String(),
		Severity:    severity,
		Timestamp:   time.Now(),
		Summary:     fmt.Sprintf("DNC check for %s: %s (%s)", event.PhoneNumber.MaskedString(), event.Result, event.CheckReason),
		EventData:   event,
		Metadata: map[string]interface{}{
			"event_id":           event.EventID.String(),
			"phone_number":       event.PhoneNumber.String(),
			"result":             string(event.Result),
			"is_blocked":         event.IsBlocked,
			"check_reason":       event.CheckReason,
			"check_type":         event.CheckType,
			"initiated_by":       event.InitiatedBy.String(),
			"sources_checked":    event.Sources,
			"latency_ms":         event.Latency.Milliseconds(),
			"success_rate":       event.GetSuccessRate(),
			"cache_hit_rate":     event.GetCacheHitRate(),
			"risk_score":         event.RiskScore,
			"confidence_score":   event.ConfidenceScore,
			"tcpa_relevant":      event.TCPARelevant,
			"gdpr_relevant":      event.GDPRRelevant,
			"is_high_risk":       event.IsHighRisk(),
			"has_warnings":       event.HasWarnings(),
		},
	}

	if event.CallID != nil {
		streamEvent.Metadata["call_id"] = event.CallID.String()
	}

	h.broadcastEvent(streamEvent)
}

// BroadcastDNCListSynced broadcasts a DNC list synced event
func (h *DNCEventHub) BroadcastDNCListSynced(event *events.DNCListSyncedEvent) {
	severity := "info"
	if event.IsFailed() || event.IsTimeout() {
		severity = "error"
	} else if event.IsPartial() || event.HasErrors() {
		severity = "warning"
	} else if event.IsSuccessful() {
		severity = "success"
	}

	streamEvent := &DNCStreamEvent{
		ID:         uuid.New().String(),
		Type:       DNCEventListSynced,
		Provider:   event.Provider,
		ProviderID: event.ProviderID.String(),
		Severity:   severity,
		Timestamp:  time.Now(),
		Summary:    fmt.Sprintf("DNC list sync for %s: %s (%d records processed)", event.Provider, event.SyncStatus, event.RecordsProcessed),
		EventData:  event,
		Metadata: map[string]interface{}{
			"event_id":             event.EventID.String(),
			"provider":             event.Provider,
			"provider_id":          event.ProviderID.String(),
			"sync_id":              event.SyncID.String(),
			"sync_status":          string(event.SyncStatus),
			"sync_trigger":         string(event.SyncTrigger),
			"sync_type":            event.SyncType,
			"sync_duration_ms":     event.SyncDuration.Milliseconds(),
			"initiated_by":         event.InitiatedBy.String(),
			"records_added":        event.RecordsAdded,
			"records_removed":      event.RecordsRemoved,
			"records_updated":      event.RecordsUpdated,
			"records_total":        event.RecordsTotal,
			"records_processed":    event.RecordsProcessed,
			"net_data_change":      event.GetNetDataChange(),
			"throughput_per_sec":   event.ThroughputPerSecond,
			"data_quality_score":   event.DataQualityScore,
			"efficiency_score":     event.GetEfficiencyScore(),
			"has_errors":           event.HasErrors(),
			"has_warnings":         event.HasWarnings(),
			"is_retry":             event.IsRetry(),
		},
	}

	h.broadcastEvent(streamEvent)
}

// BroadcastComplianceViolation broadcasts a compliance violation alert
func (h *DNCEventHub) BroadcastComplianceViolation(phoneNumber, violation, details string, metadata map[string]interface{}) {
	streamEvent := &DNCStreamEvent{
		ID:          uuid.New().String(),
		Type:        DNCEventComplianceViolation,
		PhoneNumber: phoneNumber,
		Severity:    "critical",
		Timestamp:   time.Now(),
		Summary:     fmt.Sprintf("Compliance violation: %s for %s", violation, phoneNumber),
		Metadata: map[string]interface{}{
			"violation_type": violation,
			"details":        details,
			"phone_number":   phoneNumber,
		},
	}

	// Merge additional metadata
	for k, v := range metadata {
		streamEvent.Metadata[k] = v
	}

	h.broadcastEvent(streamEvent)
}

// BroadcastProviderStatusChange broadcasts a provider status change event
func (h *DNCEventHub) BroadcastProviderStatusChange(providerID, provider, oldStatus, newStatus, reason string) {
	severity := "info"
	if newStatus == "offline" || newStatus == "error" {
		severity = "error"
	} else if newStatus == "degraded" {
		severity = "warning"
	}

	streamEvent := &DNCStreamEvent{
		ID:         uuid.New().String(),
		Type:       DNCEventProviderStatusChange,
		Provider:   provider,
		ProviderID: providerID,
		Severity:   severity,
		Timestamp:  time.Now(),
		Summary:    fmt.Sprintf("Provider %s status changed from %s to %s", provider, oldStatus, newStatus),
		Metadata: map[string]interface{}{
			"provider_id":  providerID,
			"provider":     provider,
			"old_status":   oldStatus,
			"new_status":   newStatus,
			"reason":       reason,
			"alert_type":   "provider_status",
		},
	}

	h.broadcastEvent(streamEvent)
}

// RegisterClient registers a new WebSocket client with authentication and rate limiting
func (h *DNCEventHub) RegisterClient(client *DNCClient) error {
	// Check connection limits
	h.clientsLock.RLock()
	if len(h.clients) >= h.config.MaxClients {
		h.clientsLock.RUnlock()
		return errors.NewBusinessError("MAX_CLIENTS_REACHED",
			fmt.Sprintf("maximum DNC clients reached: %d", h.config.MaxClients))
	}
	h.clientsLock.RUnlock()

	select {
	case h.register <- client:
		return nil
	case <-time.After(5 * time.Second):
		return errors.NewInternalError("DNC client registration timeout")
	}
}

// UnregisterClient unregisters a WebSocket client
func (h *DNCEventHub) UnregisterClient(client *DNCClient) {
	select {
	case h.unregister <- client:
	default:
		// Non-blocking unregister
	}
}

// GetClientCount returns the current number of connected clients
func (h *DNCEventHub) GetClientCount() int {
	h.clientsLock.RLock()
	defer h.clientsLock.RUnlock()
	return len(h.clients)
}

// GetMetrics returns current hub metrics
func (h *DNCEventHub) GetMetrics() DNCHubMetrics {
	h.metrics.mu.RLock()
	defer h.metrics.mu.RUnlock()

	// Create a copy to avoid race conditions
	return DNCHubMetrics{
		TotalConnections:      h.metrics.TotalConnections,
		ActiveConnections:     h.metrics.ActiveConnections,
		TotalEventsPublished:  h.metrics.TotalEventsPublished,
		TotalEventsFiltered:   h.metrics.TotalEventsFiltered,
		TotalEventsDropped:    h.metrics.TotalEventsDropped,
		TotalBytesTransferred: h.metrics.TotalBytesTransferred,
		AverageLatency:        h.metrics.AverageLatency,
		PeakConnections:       h.metrics.PeakConnections,
		ErrorCount:            h.metrics.ErrorCount,
		ComplianceViolations:  h.metrics.ComplianceViolations,
		HighRiskEvents:        h.metrics.HighRiskEvents,
		BlockedNumberEvents:   h.metrics.BlockedNumberEvents,
		StartTime:             h.metrics.StartTime,
	}
}

// GetConnectedClients returns information about connected clients
func (h *DNCEventHub) GetConnectedClients() []DNCClientInfo {
	h.clientsLock.RLock()
	defer h.clientsLock.RUnlock()

	clients := make([]DNCClientInfo, 0, len(h.clients))
	for _, client := range h.clients {
		info := DNCClientInfo{
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

func (h *DNCEventHub) clientManager(ctx context.Context) {
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

func (h *DNCEventHub) broadcastManager(ctx context.Context) {
	for {
		select {
		case event := <-h.broadcast:
			h.processEventBroadcast(event)
		case <-ctx.Done():
			return
		}
	}
}

func (h *DNCEventHub) healthMonitor(ctx context.Context) {
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

func (h *DNCEventHub) metricsCollector(ctx context.Context) {
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

func (h *DNCEventHub) cleanupWorker(ctx context.Context) {
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

func (h *DNCEventHub) registerClient(client *DNCClient) {
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

	h.logger.Info("DNC WebSocket client registered",
		zap.String("client_id", client.ID.String()),
		zap.String("user_id", client.userID.String()),
		zap.String("role", client.role),
		zap.Int("total_clients", len(h.clients)),
	)

	// Send welcome message
	welcome := &DNCStreamEvent{
		ID:        uuid.New().String(),
		Type:      DNCConnectionEstablished,
		Timestamp: time.Now(),
		Summary:   "Connected to DNC event stream",
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
		h.logger.Warn("DNC client channel full on welcome message",
			zap.String("client_id", client.ID.String()),
		)
	}
}

func (h *DNCEventHub) unregisterClient(client *DNCClient) {
	h.clientsLock.Lock()
	defer h.clientsLock.Unlock()

	if _, exists := h.clients[client.ID]; exists {
		delete(h.clients, client.ID)
		close(client.send)

		h.metrics.mu.Lock()
		h.metrics.ActiveConnections = int64(len(h.clients))
		h.metrics.mu.Unlock()

		h.logger.Info("DNC WebSocket client unregistered",
			zap.String("client_id", client.ID.String()),
			zap.Int("remaining_clients", len(h.clients)),
		)
	}
}

func (h *DNCEventHub) broadcastEvent(event *DNCStreamEvent) {
	// Update metrics for specific event types
	h.updateEventMetrics(event)

	select {
	case h.broadcast <- event:
		// Event queued for broadcast
	default:
		// Broadcast buffer full - drop event and record metric
		h.metrics.mu.Lock()
		h.metrics.TotalEventsDropped++
		h.metrics.mu.Unlock()

		h.logger.Warn("DNC broadcast buffer full, dropping event",
			zap.String("event_id", event.ID),
			zap.String("event_type", string(event.Type)),
		)
	}
}

func (h *DNCEventHub) processEventBroadcast(event *DNCStreamEvent) {
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
				h.logger.Warn("DNC client channel full, scheduling disconnection",
					zap.String("client_id", client.ID.String()),
				)
				go func(c *DNCClient) {
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

	h.logger.Debug("DNC event broadcasted",
		zap.String("event_id", event.ID),
		zap.String("event_type", string(event.Type)),
		zap.Int("sent_to_clients", sentCount),
		zap.Int("filtered_out", filteredCount),
		zap.Duration("latency", latency),
	)
}

func (h *DNCEventHub) shouldClientReceiveEvent(client *DNCClient, event *DNCStreamEvent) bool {
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
	return h.applyFilters(client.filters, event)
}

func (h *DNCEventHub) hasPermissionForEvent(client *DNCClient, event *DNCStreamEvent) bool {
	// Admin can see everything
	if client.role == "admin" {
		return true
	}

	// Compliance officers can see all DNC events
	if client.role == "compliance" {
		return true
	}

	// Security personnel can see compliance violations and high-risk events
	if client.role == "security" {
		return event.Type == DNCEventComplianceViolation ||
			event.Severity == "critical" ||
			event.Severity == "high"
	}

	// Operators can see operational events
	if client.role == "operator" {
		return event.Type != DNCEventComplianceViolation
	}

	// Regular users can only see basic events
	return event.Type == DNCConnectionEstablished ||
		event.Type == DNCConnectionPing ||
		event.Type == DNCConnectionPong
}

func (h *DNCEventHub) applyFilters(filters DNCEventFilters, event *DNCStreamEvent) bool {
	// Event type filter
	if len(filters.EventTypes) > 0 {
		found := false
		for _, et := range filters.EventTypes {
			if et == event.Type {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Phone pattern filter
	if len(filters.PhonePatterns) > 0 && event.PhoneNumber != "" {
		found := false
		for _, pattern := range filters.PhonePatterns {
			if matched, _ := regexp.MatchString(pattern, event.PhoneNumber); matched {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Provider filter
	if len(filters.Providers) > 0 && event.Provider != "" {
		found := false
		for _, provider := range filters.Providers {
			if provider == event.Provider {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Provider ID filter
	if len(filters.ProviderIDs) > 0 && event.ProviderID != "" {
		found := false
		for _, providerID := range filters.ProviderIDs {
			if providerID == event.ProviderID {
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
			if severity == event.Severity {
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
		if !event.Timestamp.After(filters.TimeRange.Start) ||
			!event.Timestamp.Before(filters.TimeRange.End) {
			return false
		}
	}

	// Compliance only filter
	if filters.ComplianceOnly && event.Type != DNCEventComplianceViolation {
		return false
	}

	// High risk only filter
	if filters.HighRiskOnly && event.Severity != "high" && event.Severity != "critical" {
		return false
	}

	// Errors only filter
	if filters.ErrorsOnly && event.Severity != "error" {
		return false
	}

	// Blocked only filter
	if filters.BlockedOnly {
		if blocked, ok := event.Metadata["is_blocked"].(bool); !ok || !blocked {
			return false
		}
	}

	return true
}

func (h *DNCEventHub) pingClients() {
	h.clientsLock.RLock()
	defer h.clientsLock.RUnlock()

	for _, client := range h.clients {
		if err := client.conn.WriteControl(
			websocket.PingMessage,
			nil,
			time.Now().Add(h.config.WriteTimeout),
		); err != nil {
			h.logger.Error("Failed to ping DNC client",
				zap.String("client_id", client.ID.String()),
				zap.Error(err),
			)
			go func(c *DNCClient) {
				h.unregister <- c
			}(client)
		} else {
			client.pingCount++
		}
	}
}

func (h *DNCEventHub) cleanupStaleConnections() {
	h.clientsLock.RLock()
	staleClients := make([]*DNCClient, 0)

	for _, client := range h.clients {
		// Check for stale connections
		if time.Since(client.lastPong) > h.config.PongTimeout {
			staleClients = append(staleClients, client)
		}
	}
	h.clientsLock.RUnlock()

	// Remove stale clients
	for _, client := range staleClients {
		h.logger.Info("Removing stale DNC client connection",
			zap.String("client_id", client.ID.String()),
			zap.Duration("last_pong", time.Since(client.lastPong)),
		)
		h.unregister <- client
	}
}

func (h *DNCEventHub) updateMetrics() {
	h.clientsLock.RLock()
	activeClients := int64(len(h.clients))
	h.clientsLock.RUnlock()

	h.metrics.mu.Lock()
	h.metrics.ActiveConnections = activeClients
	h.metrics.mu.Unlock()
}

func (h *DNCEventHub) updateEventMetrics(event *DNCStreamEvent) {
	h.metrics.mu.Lock()
	defer h.metrics.mu.Unlock()

	switch event.Type {
	case DNCEventComplianceViolation:
		h.metrics.ComplianceViolations++
	case DNCEventCheckPerformed:
		if event.Severity == "high" || event.Severity == "critical" {
			h.metrics.HighRiskEvents++
		}
		if blocked, ok := event.Metadata["is_blocked"].(bool); ok && blocked {
			h.metrics.BlockedNumberEvents++
		}
	}
}

func (h *DNCEventHub) isClientRateLimited(client *DNCClient) bool {
	if client.rateLimiter == nil {
		return false
	}
	return client.rateLimiter.IsLimited()
}

func (h *DNCEventHub) countClientFilters(filters DNCEventFilters) int {
	count := len(filters.EventTypes) + len(filters.PhonePatterns) + len(filters.ProviderIDs) +
		len(filters.Providers) + len(filters.Severities) + len(filters.CheckReasons) +
		len(filters.SuppressReasons) + len(filters.ReleaseReasons) + len(filters.SyncTriggers) +
		len(filters.SyncStatuses) + len(filters.CustomFilters)

	if filters.TimeRange != nil {
		count++
	}
	if filters.ComplianceOnly {
		count++
	}
	if filters.HighRiskOnly {
		count++
	}
	if filters.ErrorsOnly {
		count++
	}
	if filters.BlockedOnly {
		count++
	}

	return count
}

func (h *DNCEventHub) getSeverityFromReason(reason string) string {
	switch reason {
	case "regulatory", "consumer_request":
		return "high"
	case "internal_policy":
		return "medium"
	case "testing":
		return "low"
	default:
		return "medium"
	}
}

func (h *DNCEventHub) getSeverityFromReleaseReason(reason string) string {
	switch reason {
	case "admin_override", "data_correction":
		return "high"
	case "consumer_request":
		return "medium"
	case "expired", "system_cleanup":
		return "low"
	default:
		return "medium"
	}
}

func (h *DNCEventHub) shutdown() {
	h.logger.Info("Shutting down DNC event hub")

	h.clientsLock.Lock()
	defer h.clientsLock.Unlock()

	for _, client := range h.clients {
		close(client.send)
		client.conn.Close()
	}
	h.clients = make(map[uuid.UUID]*DNCClient)
}

// DNCClientInfo provides information about a connected client
type DNCClientInfo struct {
	ClientID     string    `json:"client_id"`
	UserID       string    `json:"user_id"`
	Role         string    `json:"role"`
	ConnectedAt  time.Time `json:"connected_at"`
	LastActivity time.Time `json:"last_activity"`
	MessageCount int64     `json:"message_count"`
	FilterCount  int       `json:"filter_count"`
	RateLimited  bool      `json:"rate_limited"`
}

// NewDNCClient creates a new DNC WebSocket client with rate limiting
func NewDNCClient(
	conn *websocket.Conn,
	hub *DNCEventHub,
	userID uuid.UUID,
	role string,
	permissions []string,
) *DNCClient {
	return &DNCClient{
		ID:            uuid.New(),
		conn:          conn,
		send:          make(chan *DNCStreamEvent, hub.config.ClientBufferSize),
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
func (c *DNCClient) ReadPump() {
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
				c.hub.logger.Error("DNC WebSocket read error",
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
			c.hub.logger.Error("Failed to parse DNC client message",
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
func (c *DNCClient) WritePump() {
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
				c.hub.logger.Error("Failed to write DNC event to client",
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

func (c *DNCClient) handleClientMessage(msg map[string]interface{}) {
	if msgType, ok := msg["type"].(string); ok {
		switch msgType {
		case "update_filters":
			c.updateFilters(msg)
		case "ping":
			// Respond with pong
			pong := &DNCStreamEvent{
				ID:        uuid.New().String(),
				Type:      DNCConnectionPong,
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

func (c *DNCClient) updateFilters(msg map[string]interface{}) {
	if filters, ok := msg["filters"].(map[string]interface{}); ok {
		// Validate filter count
		if c.hub.countClientFilters(c.filters) > c.hub.config.MaxFiltersPerClient {
			c.hub.logger.Warn("DNC client exceeded maximum filter count",
				zap.String("client_id", c.ID.String()),
				zap.Int("filter_count", c.hub.countClientFilters(c.filters)),
				zap.Int("max_allowed", c.hub.config.MaxFiltersPerClient),
			)
			return
		}

		// Update filters
		c.parseAndSetFilters(filters)

		c.hub.logger.Info("DNC client filters updated",
			zap.String("client_id", c.ID.String()),
			zap.Int("filter_count", c.hub.countClientFilters(c.filters)),
		)
	}
}

func (c *DNCClient) parseAndSetFilters(filters map[string]interface{}) {
	// Parse event types
	if eventTypes, ok := filters["event_types"].([]interface{}); ok {
		c.filters.EventTypes = make([]DNCEventType, 0, len(eventTypes))
		for _, et := range eventTypes {
			if strET, ok := et.(string); ok {
				c.filters.EventTypes = append(c.filters.EventTypes, DNCEventType(strET))
			}
		}
	}

	// Parse string array filters
	c.parseStringArrayFilter(filters, "phone_patterns", &c.filters.PhonePatterns)
	c.parseStringArrayFilter(filters, "provider_ids", &c.filters.ProviderIDs)
	c.parseStringArrayFilter(filters, "providers", &c.filters.Providers)
	c.parseStringArrayFilter(filters, "severities", &c.filters.Severities)
	c.parseStringArrayFilter(filters, "check_reasons", &c.filters.CheckReasons)
	c.parseStringArrayFilter(filters, "suppress_reasons", &c.filters.SuppressReasons)
	c.parseStringArrayFilter(filters, "release_reasons", &c.filters.ReleaseReasons)
	c.parseStringArrayFilter(filters, "sync_triggers", &c.filters.SyncTriggers)
	c.parseStringArrayFilter(filters, "sync_statuses", &c.filters.SyncStatuses)

	// Parse boolean filters
	if complianceOnly, ok := filters["compliance_only"].(bool); ok {
		c.filters.ComplianceOnly = complianceOnly
	}

	if highRiskOnly, ok := filters["high_risk_only"].(bool); ok {
		c.filters.HighRiskOnly = highRiskOnly
	}

	if errorsOnly, ok := filters["errors_only"].(bool); ok {
		c.filters.ErrorsOnly = errorsOnly
	}

	if blockedOnly, ok := filters["blocked_only"].(bool); ok {
		c.filters.BlockedOnly = blockedOnly
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

func (c *DNCClient) parseStringArrayFilter(filters map[string]interface{}, key string, target *[]string) {
	if values, ok := filters[key].([]interface{}); ok {
		*target = make([]string, 0, len(values))
		for _, value := range values {
			if strValue, ok := value.(string); ok {
				*target = append(*target, strValue)
			}
		}
	}
}

func (c *DNCClient) parseRelativeTimeRange(relative string) {
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

func (c *DNCClient) sendClientStats() {
	stats := &DNCStreamEvent{
		ID:        uuid.New().String(),
		Type:      "client.stats",
		Timestamp: time.Now(),
		Summary:   "DNC client statistics",
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