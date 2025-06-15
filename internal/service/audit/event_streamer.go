package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/audit"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
)

// EventStreamer provides real-time streaming of audit events
// Following DCE patterns: service orchestrates infrastructure and domains
type EventStreamer struct {
	// Infrastructure dependencies
	eventRepo audit.EventRepository
	// monitor     monitoring.Monitor // TODO: Add monitoring when infrastructure available
	logger *zap.Logger

	// Configuration
	config *StreamerConfig

	// Connection management
	connections map[string]*StreamConnection
	connMutex   sync.RWMutex
	eventBuffer chan *audit.Event
	upgrader    websocket.Upgrader

	// State management
	isRunning bool
	startTime time.Time
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup

	// Metrics
	metrics *StreamingMetrics
}

// StreamerConfig configures the event streamer
type StreamerConfig struct {
	// Connection settings
	MaxConnections    int           `json:"max_connections"`
	ConnectionTimeout time.Duration `json:"connection_timeout"`
	PingInterval      time.Duration `json:"ping_interval"`
	ReadBufferSize    int           `json:"read_buffer_size"`
	WriteBufferSize   int           `json:"write_buffer_size"`

	// Buffering settings
	EventBufferSize int           `json:"event_buffer_size"`
	BatchSize       int           `json:"batch_size"`
	FlushInterval   time.Duration `json:"flush_interval"`

	// Filtering settings
	EnableEventFiltering bool `json:"enable_event_filtering"`
	MaxFiltersPerConn    int  `json:"max_filters_per_connection"`

	// Security settings
	RequireAuth        bool     `json:"require_auth"`
	AllowedOrigins     []string `json:"allowed_origins"`
	RateLimitPerSecond int      `json:"rate_limit_per_second"`

	// Performance settings
	CompressionEnabled bool `json:"compression_enabled"`
	HeartbeatEnabled   bool `json:"heartbeat_enabled"`
	BinaryEncoding     bool `json:"binary_encoding"`
}

// DefaultStreamerConfig returns sensible defaults
func DefaultStreamerConfig() *StreamerConfig {
	return &StreamerConfig{
		MaxConnections:       1000,
		ConnectionTimeout:    30 * time.Second,
		PingInterval:         10 * time.Second,
		ReadBufferSize:       1024,
		WriteBufferSize:      1024,
		EventBufferSize:      10000,
		BatchSize:            100,
		FlushInterval:        1 * time.Second,
		EnableEventFiltering: true,
		MaxFiltersPerConn:    10,
		RequireAuth:          true,
		AllowedOrigins:       []string{"*"},
		RateLimitPerSecond:   100,
		CompressionEnabled:   true,
		HeartbeatEnabled:     true,
		BinaryEncoding:       false,
	}
}

// StreamConnection represents a client connection
type StreamConnection struct {
	ID          string              `json:"id"`
	UserID      *string             `json:"user_id,omitempty"`
	RemoteAddr  string              `json:"remote_addr"`
	ConnectedAt time.Time           `json:"connected_at"`
	LastActive  time.Time           `json:"last_active"`
	Filters     []*StreamFilter     `json:"filters"`
	Conn        *websocket.Conn     `json:"-"`
	SendChan    chan *StreamMessage `json:"-"`
	IsActive    bool                `json:"is_active"`

	// Rate limiting
	rateLimiter *TokenBucket `json:"-"`

	// Connection-specific metrics
	MessagesSent    int64 `json:"messages_sent"`
	MessagesDropped int64 `json:"messages_dropped"`
	BytesSent       int64 `json:"bytes_sent"`

	// Synchronization
	mu sync.RWMutex `json:"-"`
}

// StreamFilter defines filtering criteria for events
type StreamFilter struct {
	Name       string                 `json:"name"`
	EventTypes []string               `json:"event_types,omitempty"`
	Actors     []string               `json:"actors,omitempty"`
	Entities   []string               `json:"entities,omitempty"`
	TimeRange  *audit.TimeRange       `json:"time_range,omitempty"`
	Severity   []string               `json:"severity,omitempty"`
	Custom     map[string]interface{} `json:"custom,omitempty"`
	IsEnabled  bool                   `json:"is_enabled"`
}

// StreamMessage represents a message sent to clients
type StreamMessage struct {
	Type      string      `json:"type"`
	Timestamp time.Time   `json:"timestamp"`
	Data      interface{} `json:"data"`
	Sequence  int64       `json:"sequence,omitempty"`
	Checksum  string      `json:"checksum,omitempty"`
}

// StreamingMetrics tracks streaming performance
type StreamingMetrics struct {
	// Connection metrics
	TotalConnections   int64 `json:"total_connections"`
	ActiveConnections  int64 `json:"active_connections"`
	PeakConnections    int64 `json:"peak_connections"`
	ConnectionsDropped int64 `json:"connections_dropped"`

	// Message metrics
	EventsStreamed   int64 `json:"events_streamed"`
	MessagesDropped  int64 `json:"messages_dropped"`
	BytesTransferred int64 `json:"bytes_transferred"`

	// Performance metrics
	AverageLatency time.Duration `json:"average_latency"`
	TotalLatency   time.Duration `json:"total_latency"`

	// Error metrics
	ConnectionErrors int64 `json:"connection_errors"`
	StreamingErrors  int64 `json:"streaming_errors"`

	// Last update
	LastUpdated time.Time `json:"last_updated"`

	mu sync.RWMutex `json:"-"`
}

// TokenBucket implements simple rate limiting
type TokenBucket struct {
	capacity   int
	tokens     int
	refillRate int
	lastRefill time.Time
	mu         sync.Mutex
}

// NewEventStreamer creates a new event streamer
func NewEventStreamer(
	eventRepo audit.EventRepository,
	// monitor monitoring.Monitor, // TODO: Add monitoring when infrastructure available
	logger *zap.Logger,
	config *StreamerConfig,
) *EventStreamer {
	if config == nil {
		config = DefaultStreamerConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	upgrader := websocket.Upgrader{
		ReadBufferSize:  config.ReadBufferSize,
		WriteBufferSize: config.WriteBufferSize,
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			for _, allowed := range config.AllowedOrigins {
				if allowed == "*" || allowed == origin {
					return true
				}
			}
			return false
		},
	}

	if config.CompressionEnabled {
		upgrader.EnableCompression = true
	}

	return &EventStreamer{
		eventRepo: eventRepo,
		// monitor:     monitor, // TODO: Add monitoring when infrastructure available
		logger:      logger,
		config:      config,
		connections: make(map[string]*StreamConnection),
		eventBuffer: make(chan *audit.Event, config.EventBufferSize),
		upgrader:    upgrader,
		ctx:         ctx,
		cancel:      cancel,
		metrics:     &StreamingMetrics{},
	}
}

// Start initializes and starts the event streamer
func (s *EventStreamer) Start(ctx context.Context) error {
	s.connMutex.Lock()
	defer s.connMutex.Unlock()

	if s.isRunning {
		return errors.NewInternalError("event streamer already running")
	}

	s.logger.Info("Starting event streamer",
		zap.Int("max_connections", s.config.MaxConnections),
		zap.Int("event_buffer_size", s.config.EventBufferSize),
		zap.Bool("filtering_enabled", s.config.EnableEventFiltering))

	// Start background goroutines
	s.wg.Add(3)
	go s.eventProcessor()
	go s.connectionManager()
	go s.metricsCollector()

	s.isRunning = true
	s.startTime = time.Now()

	s.logger.Info("Event streamer started successfully")
	return nil
}

// Stop gracefully shuts down the event streamer
func (s *EventStreamer) Stop(ctx context.Context) error {
	s.connMutex.Lock()
	defer s.connMutex.Unlock()

	if !s.isRunning {
		return nil
	}

	s.logger.Info("Stopping event streamer...")

	// Cancel context to stop background goroutines
	s.cancel()

	// Close all connections
	for _, conn := range s.connections {
		s.closeConnection(conn, "server_shutdown")
	}

	// Wait for goroutines to finish
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		s.logger.Info("Event streamer stopped gracefully")
	case <-time.After(30 * time.Second):
		s.logger.Warn("Event streamer shutdown timeout")
	}

	s.isRunning = false
	return nil
}

// HandleWebSocketUpgrade handles WebSocket connection upgrades
func (s *EventStreamer) HandleWebSocketUpgrade(w http.ResponseWriter, r *http.Request, userID *string) error {
	if !s.isRunning {
		return errors.NewInternalError("event streamer not running")
	}

	// Check connection limit
	s.connMutex.RLock()
	connCount := len(s.connections)
	s.connMutex.RUnlock()

	if connCount >= s.config.MaxConnections {
		s.logger.Warn("Connection limit reached", zap.Int("connections", connCount))
		return errors.NewInternalError("connection limit reached")
	}

	// Upgrade connection
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Error("Failed to upgrade WebSocket connection", zap.Error(err))
		s.updateMetrics(func(m *StreamingMetrics) {
			m.ConnectionErrors++
		})
		return errors.NewInternalError("failed to upgrade connection").WithCause(err)
	}

	// Create stream connection
	streamConn := &StreamConnection{
		ID:          generateConnectionID(),
		UserID:      userID,
		RemoteAddr:  r.RemoteAddr,
		ConnectedAt: time.Now(),
		LastActive:  time.Now(),
		Conn:        conn,
		SendChan:    make(chan *StreamMessage, 100),
		IsActive:    true,
		rateLimiter: NewTokenBucket(s.config.RateLimitPerSecond, s.config.RateLimitPerSecond),
		Filters:     make([]*StreamFilter, 0),
	}

	// Register connection
	s.connMutex.Lock()
	s.connections[streamConn.ID] = streamConn
	s.connMutex.Unlock()

	// Update metrics
	s.updateMetrics(func(m *StreamingMetrics) {
		m.TotalConnections++
		m.ActiveConnections++
		if m.ActiveConnections > m.PeakConnections {
			m.PeakConnections = m.ActiveConnections
		}
	})

	// Start connection handlers
	s.wg.Add(2)
	go s.connectionReader(streamConn)
	go s.connectionWriter(streamConn)

	// Send welcome message
	welcomeMsg := &StreamMessage{
		Type:      "welcome",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"connection_id": streamConn.ID,
			"server_time":   time.Now().UTC(),
			"features": map[string]bool{
				"filtering":   s.config.EnableEventFiltering,
				"compression": s.config.CompressionEnabled,
				"heartbeat":   s.config.HeartbeatEnabled,
			},
		},
	}

	select {
	case streamConn.SendChan <- welcomeMsg:
	default:
		s.logger.Warn("Failed to send welcome message, channel full")
	}

	s.logger.Info("New WebSocket connection established",
		zap.String("connection_id", streamConn.ID),
		zap.String("remote_addr", streamConn.RemoteAddr),
		zap.Stringp("user_id", userID))

	return nil
}

// StreamEvent streams an audit event to connected clients
func (s *EventStreamer) StreamEvent(ctx context.Context, event *audit.Event) error {
	if !s.isRunning {
		return errors.NewInternalError("event streamer not running")
	}

	select {
	case s.eventBuffer <- event:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		s.logger.Warn("Event buffer full, dropping event",
			zap.String("event_id", event.ID),
			zap.String("event_type", event.EventType))

		s.updateMetrics(func(m *StreamingMetrics) {
			m.MessagesDropped++
		})

		return errors.NewInternalError("event buffer full")
	}
}

// AddFilter adds a filter to a connection
func (s *EventStreamer) AddFilter(connectionID string, filter *StreamFilter) error {
	s.connMutex.RLock()
	conn, exists := s.connections[connectionID]
	s.connMutex.RUnlock()

	if !exists {
		return errors.NewNotFoundError(fmt.Sprintf("connection %s", connectionID))
	}

	conn.mu.Lock()
	defer conn.mu.Unlock()

	if len(conn.Filters) >= s.config.MaxFiltersPerConn {
		return errors.NewValidationError("MAX_FILTERS_EXCEEDED",
			fmt.Sprintf("maximum %d filters per connection", s.config.MaxFiltersPerConn))
	}

	// Validate filter
	if err := s.validateFilter(filter); err != nil {
		return err
	}

	conn.Filters = append(conn.Filters, filter)
	conn.LastActive = time.Now()

	s.logger.Debug("Filter added to connection",
		zap.String("connection_id", connectionID),
		zap.String("filter_name", filter.Name))

	return nil
}

// RemoveFilter removes a filter from a connection
func (s *EventStreamer) RemoveFilter(connectionID, filterName string) error {
	s.connMutex.RLock()
	conn, exists := s.connections[connectionID]
	s.connMutex.RUnlock()

	if !exists {
		return errors.NewNotFoundError(fmt.Sprintf("connection %s", connectionID))
	}

	conn.mu.Lock()
	defer conn.mu.Unlock()

	for i, filter := range conn.Filters {
		if filter.Name == filterName {
			conn.Filters = append(conn.Filters[:i], conn.Filters[i+1:]...)
			conn.LastActive = time.Now()

			s.logger.Debug("Filter removed from connection",
				zap.String("connection_id", connectionID),
				zap.String("filter_name", filterName))

			return nil
		}
	}

	return errors.NewNotFoundError(fmt.Sprintf("filter %s", filterName))
}

// GetConnectionStatus returns the status of all connections
func (s *EventStreamer) GetConnectionStatus() *StreamerStatus {
	s.connMutex.RLock()
	defer s.connMutex.RUnlock()

	connections := make([]*StreamConnection, 0, len(s.connections))
	for _, conn := range s.connections {
		connCopy := *conn
		connections = append(connections, &connCopy)
	}

	metrics := s.getMetricsSnapshot()

	return &StreamerStatus{
		IsRunning:       s.isRunning,
		StartTime:       s.startTime,
		Connections:     connections,
		Metrics:         metrics,
		Configuration:   s.config,
		EventBufferSize: len(s.eventBuffer),
		LastUpdated:     time.Now(),
	}
}

// StreamerStatus represents the current status of the streamer
type StreamerStatus struct {
	IsRunning       bool                `json:"is_running"`
	StartTime       time.Time           `json:"start_time"`
	Connections     []*StreamConnection `json:"connections"`
	Metrics         *StreamingMetrics   `json:"metrics"`
	Configuration   *StreamerConfig     `json:"configuration"`
	EventBufferSize int                 `json:"event_buffer_size"`
	LastUpdated     time.Time           `json:"last_updated"`
}

// Background processing methods

// eventProcessor processes events from the buffer and distributes to connections
func (s *EventStreamer) eventProcessor() {
	defer s.wg.Done()

	batch := make([]*audit.Event, 0, s.config.BatchSize)
	flushTimer := time.NewTimer(s.config.FlushInterval)
	defer flushTimer.Stop()

	for {
		select {
		case <-s.ctx.Done():
			// Process remaining events in batch
			if len(batch) > 0 {
				s.processBatch(batch)
			}
			return

		case event := <-s.eventBuffer:
			batch = append(batch, event)

			// Process batch when full
			if len(batch) >= s.config.BatchSize {
				s.processBatch(batch)
				batch = batch[:0]

				// Reset timer
				if !flushTimer.Stop() {
					<-flushTimer.C
				}
				flushTimer.Reset(s.config.FlushInterval)
			}

		case <-flushTimer.C:
			// Process batch on timer
			if len(batch) > 0 {
				s.processBatch(batch)
				batch = batch[:0]
			}
			flushTimer.Reset(s.config.FlushInterval)
		}
	}
}

// processBatch processes a batch of events
func (s *EventStreamer) processBatch(events []*audit.Event) {
	startTime := time.Now()

	s.connMutex.RLock()
	activeConnections := make([]*StreamConnection, 0, len(s.connections))
	for _, conn := range s.connections {
		if conn.IsActive {
			activeConnections = append(activeConnections, conn)
		}
	}
	s.connMutex.RUnlock()

	if len(activeConnections) == 0 {
		return
	}

	for _, event := range events {
		streamMsg := &StreamMessage{
			Type:      "audit_event",
			Timestamp: time.Now(),
			Data:      event,
			Sequence:  int64(event.SequenceNumber),
		}

		// Send to matching connections
		for _, conn := range activeConnections {
			if s.eventMatchesFilters(event, conn) {
				if conn.rateLimiter.Allow() {
					select {
					case conn.SendChan <- streamMsg:
						conn.mu.Lock()
						conn.MessagesSent++
						conn.LastActive = time.Now()
						conn.mu.Unlock()
					default:
						conn.mu.Lock()
						conn.MessagesDropped++
						conn.mu.Unlock()

						s.logger.Warn("Connection send buffer full",
							zap.String("connection_id", conn.ID))
					}
				} else {
					conn.mu.Lock()
					conn.MessagesDropped++
					conn.mu.Unlock()
				}
			}
		}
	}

	// Update metrics
	processingTime := time.Since(startTime)
	s.updateMetrics(func(m *StreamingMetrics) {
		m.EventsStreamed += int64(len(events))
		m.TotalLatency += processingTime
		m.AverageLatency = m.TotalLatency / time.Duration(m.EventsStreamed)
	})
}

// connectionManager manages connection health and cleanup
func (s *EventStreamer) connectionManager() {
	defer s.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.cleanupConnections()
		}
	}
}

// cleanupConnections removes inactive connections
func (s *EventStreamer) cleanupConnections() {
	s.connMutex.Lock()
	defer s.connMutex.Unlock()

	now := time.Now()
	toRemove := make([]string, 0)

	for id, conn := range s.connections {
		conn.mu.RLock()
		inactive := now.Sub(conn.LastActive) > s.config.ConnectionTimeout
		conn.mu.RUnlock()

		if inactive || !conn.IsActive {
			toRemove = append(toRemove, id)
		}
	}

	for _, id := range toRemove {
		if conn, exists := s.connections[id]; exists {
			s.closeConnection(conn, "timeout")
			delete(s.connections, id)
		}
	}

	if len(toRemove) > 0 {
		s.logger.Debug("Cleaned up inactive connections", zap.Int("count", len(toRemove)))

		s.updateMetrics(func(m *StreamingMetrics) {
			m.ActiveConnections = int64(len(s.connections))
			m.ConnectionsDropped += int64(len(toRemove))
		})
	}
}

// metricsCollector periodically updates metrics
func (s *EventStreamer) metricsCollector() {
	defer s.wg.Done()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.collectMetrics()
		}
	}
}

// collectMetrics collects and reports metrics
func (s *EventStreamer) collectMetrics() {
	s.connMutex.RLock()
	activeConnections := int64(len(s.connections))
	s.connMutex.RUnlock()

	s.updateMetrics(func(m *StreamingMetrics) {
		m.ActiveConnections = activeConnections
		m.LastUpdated = time.Now()
	})

	// Report to monitoring system
	// TODO: Add monitoring when infrastructure available
	// if s.monitor != nil {
	//	metrics := s.getMetricsSnapshot()
	//
	//	s.monitor.RecordGauge("audit_streamer_active_connections", float64(metrics.ActiveConnections), nil)
	//	s.monitor.RecordCounter("audit_streamer_events_streamed", float64(metrics.EventsStreamed), nil)
	//	s.monitor.RecordCounter("audit_streamer_messages_dropped", float64(metrics.MessagesDropped), nil)
	//	s.monitor.RecordHistogram("audit_streamer_average_latency_ms", float64(metrics.AverageLatency.Milliseconds()), nil)
	// }
}

// Connection handling methods

// connectionReader handles incoming messages from a connection
func (s *EventStreamer) connectionReader(conn *StreamConnection) {
	defer s.wg.Done()
	defer s.closeConnection(conn, "reader_exit")

	conn.Conn.SetReadLimit(1024)
	conn.Conn.SetReadDeadline(time.Now().Add(s.config.ConnectionTimeout))
	conn.Conn.SetPongHandler(func(string) error {
		conn.Conn.SetReadDeadline(time.Now().Add(s.config.ConnectionTimeout))
		conn.mu.Lock()
		conn.LastActive = time.Now()
		conn.mu.Unlock()
		return nil
	})

	for {
		messageType, message, err := conn.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				s.logger.Error("WebSocket read error", zap.Error(err))
			}
			return
		}

		conn.mu.Lock()
		conn.LastActive = time.Now()
		conn.mu.Unlock()

		if messageType == websocket.TextMessage {
			s.handleClientMessage(conn, message)
		}
	}
}

// connectionWriter handles outgoing messages to a connection
func (s *EventStreamer) connectionWriter(conn *StreamConnection) {
	defer s.wg.Done()
	defer s.closeConnection(conn, "writer_exit")

	ticker := time.NewTicker(s.config.PingInterval)
	defer ticker.Stop()

	for {
		select {
		case msg, ok := <-conn.SendChan:
			conn.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				conn.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			data, err := json.Marshal(msg)
			if err != nil {
				s.logger.Error("Failed to marshal message", zap.Error(err))
				continue
			}

			if err := conn.Conn.WriteMessage(websocket.TextMessage, data); err != nil {
				s.logger.Error("Failed to write message", zap.Error(err))
				return
			}

			conn.mu.Lock()
			conn.BytesSent += int64(len(data))
			conn.mu.Unlock()

		case <-ticker.C:
			conn.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// handleClientMessage processes messages from clients
func (s *EventStreamer) handleClientMessage(conn *StreamConnection, message []byte) {
	var clientMsg map[string]interface{}
	if err := json.Unmarshal(message, &clientMsg); err != nil {
		s.logger.Warn("Invalid client message", zap.Error(err))
		return
	}

	msgType, ok := clientMsg["type"].(string)
	if !ok {
		s.logger.Warn("Missing message type")
		return
	}

	switch msgType {
	case "add_filter":
		s.handleAddFilter(conn, clientMsg)
	case "remove_filter":
		s.handleRemoveFilter(conn, clientMsg)
	case "ping":
		s.handlePing(conn)
	default:
		s.logger.Warn("Unknown message type", zap.String("type", msgType))
	}
}

// Helper methods

// eventMatchesFilters checks if an event matches connection filters
func (s *EventStreamer) eventMatchesFilters(event *audit.Event, conn *StreamConnection) bool {
	if !s.config.EnableEventFiltering {
		return true
	}

	conn.mu.RLock()
	filters := conn.Filters
	conn.mu.RUnlock()

	// No filters means accept all events
	if len(filters) == 0 {
		return true
	}

	// Event must match at least one enabled filter
	for _, filter := range filters {
		if filter.IsEnabled && s.matchesFilter(event, filter) {
			return true
		}
	}

	return false
}

// matchesFilter checks if an event matches a specific filter
func (s *EventStreamer) matchesFilter(event *audit.Event, filter *StreamFilter) bool {
	// Check event types
	if len(filter.EventTypes) > 0 {
		found := false
		for _, eventType := range filter.EventTypes {
			if event.EventType == eventType {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check actors
	if len(filter.Actors) > 0 {
		found := false
		for _, actor := range filter.Actors {
			if event.Actor == actor {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check entities
	if len(filter.Entities) > 0 {
		found := false
		for _, entity := range filter.Entities {
			if event.EntityType == entity {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check time range
	if filter.TimeRange != nil {
		if event.Timestamp.Before(filter.TimeRange.Start) || event.Timestamp.After(filter.TimeRange.End) {
			return false
		}
	}

	// Check custom filters
	for key, expectedValue := range filter.Custom {
		if actualValue, exists := event.Metadata[key]; !exists || actualValue != expectedValue {
			return false
		}
	}

	return true
}

// validateFilter validates a stream filter
func (s *EventStreamer) validateFilter(filter *StreamFilter) error {
	if filter.Name == "" {
		return errors.NewValidationError("INVALID_FILTER", "filter name is required")
	}

	if filter.TimeRange != nil {
		if filter.TimeRange.Start.After(filter.TimeRange.End) {
			return errors.NewValidationError("INVALID_TIME_RANGE", "start time must be before end time")
		}
	}

	return nil
}

// closeConnection closes a connection
func (s *EventStreamer) closeConnection(conn *StreamConnection, reason string) {
	conn.mu.Lock()
	if !conn.IsActive {
		conn.mu.Unlock()
		return
	}
	conn.IsActive = false
	conn.mu.Unlock()

	close(conn.SendChan)
	conn.Conn.Close()

	s.logger.Debug("Connection closed",
		zap.String("connection_id", conn.ID),
		zap.String("reason", reason))
}

// updateMetrics safely updates streaming metrics
func (s *EventStreamer) updateMetrics(updateFn func(*StreamingMetrics)) {
	s.metrics.mu.Lock()
	updateFn(s.metrics)
	s.metrics.mu.Unlock()
}

// getMetricsSnapshot returns a snapshot of current metrics
func (s *EventStreamer) getMetricsSnapshot() *StreamingMetrics {
	s.metrics.mu.RLock()
	defer s.metrics.mu.RUnlock()

	return &StreamingMetrics{
		TotalConnections:   s.metrics.TotalConnections,
		ActiveConnections:  s.metrics.ActiveConnections,
		PeakConnections:    s.metrics.PeakConnections,
		ConnectionsDropped: s.metrics.ConnectionsDropped,
		EventsStreamed:     s.metrics.EventsStreamed,
		MessagesDropped:    s.metrics.MessagesDropped,
		BytesTransferred:   s.metrics.BytesTransferred,
		AverageLatency:     s.metrics.AverageLatency,
		TotalLatency:       s.metrics.TotalLatency,
		ConnectionErrors:   s.metrics.ConnectionErrors,
		StreamingErrors:    s.metrics.StreamingErrors,
		LastUpdated:        s.metrics.LastUpdated,
	}
}

// Utility functions

// generateConnectionID generates a unique connection ID
func generateConnectionID() string {
	return fmt.Sprintf("conn_%d_%s", time.Now().UnixNano(), uuid.New().String()[:8])
}

// NewTokenBucket creates a new token bucket for rate limiting
func NewTokenBucket(capacity, refillRate int) *TokenBucket {
	return &TokenBucket{
		capacity:   capacity,
		tokens:     capacity,
		refillRate: refillRate,
		lastRefill: time.Now(),
	}
}

// Allow checks if an action is allowed under rate limiting
func (tb *TokenBucket) Allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(tb.lastRefill)

	// Refill tokens based on elapsed time
	tokensToAdd := int(elapsed.Seconds()) * tb.refillRate
	tb.tokens = min(tb.capacity, tb.tokens+tokensToAdd)
	tb.lastRefill = now

	if tb.tokens > 0 {
		tb.tokens--
		return true
	}

	return false
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Message handlers

// handleAddFilter handles add filter requests
func (s *EventStreamer) handleAddFilter(conn *StreamConnection, msg map[string]interface{}) {
	filterData, ok := msg["filter"].(map[string]interface{})
	if !ok {
		s.logger.Warn("Invalid filter data")
		return
	}

	filter := &StreamFilter{}

	// Parse filter from message data
	if name, ok := filterData["name"].(string); ok {
		filter.Name = name
	}

	if eventTypes, ok := filterData["event_types"].([]interface{}); ok {
		filter.EventTypes = make([]string, len(eventTypes))
		for i, et := range eventTypes {
			if etStr, ok := et.(string); ok {
				filter.EventTypes[i] = etStr
			}
		}
	}

	filter.IsEnabled = true

	if err := s.AddFilter(conn.ID, filter); err != nil {
		s.logger.Warn("Failed to add filter", zap.Error(err))
	}
}

// handleRemoveFilter handles remove filter requests
func (s *EventStreamer) handleRemoveFilter(conn *StreamConnection, msg map[string]interface{}) {
	filterName, ok := msg["filter_name"].(string)
	if !ok {
		s.logger.Warn("Missing filter name")
		return
	}

	if err := s.RemoveFilter(conn.ID, filterName); err != nil {
		s.logger.Warn("Failed to remove filter", zap.Error(err))
	}
}

// handlePing handles ping requests
func (s *EventStreamer) handlePing(conn *StreamConnection) {
	pongMsg := &StreamMessage{
		Type:      "pong",
		Timestamp: time.Now(),
		Data:      map[string]interface{}{"server_time": time.Now().UTC()},
	}

	select {
	case conn.SendChan <- pongMsg:
	default:
		s.logger.Warn("Failed to send pong message, channel full")
	}
}
