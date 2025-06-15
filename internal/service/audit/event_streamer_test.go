package audit

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/audit"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
)

// Mock implementations for testing

type MockEventRepository struct {
	mock.Mock
}

func (m *MockEventRepository) GetLatestSequenceNumber(ctx context.Context) (values.SequenceNumber, error) {
	args := m.Called(ctx)
	return args.Get(0).(values.SequenceNumber), args.Error(1)
}

func (m *MockEventRepository) GetByID(ctx context.Context, id string) (*audit.Event, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(*audit.Event), args.Error(1)
}

func (m *MockEventRepository) GetSequenceRange(ctx context.Context, start, end values.SequenceNumber) ([]*audit.Event, error) {
	args := m.Called(ctx, start, end)
	return args.Get(0).([]*audit.Event), args.Error(1)
}

type MockMonitor struct {
	mock.Mock
}

func (m *MockMonitor) RecordCounter(name string, value float64, tags map[string]string) {
	m.Called(name, value, tags)
}

func (m *MockMonitor) RecordHistogram(name string, value float64, tags map[string]string) {
	m.Called(name, value, tags)
}

func (m *MockMonitor) RecordGauge(name string, value float64, tags map[string]string) {
	m.Called(name, value, tags)
}

// Test setup helper
func setupTestEventStreamer(t *testing.T) (*EventStreamer, *MockEventRepository, *MockMonitor) {
	logger := zaptest.NewLogger(t)
	mockEventRepo := &MockEventRepository{}
	mockMonitor := &MockMonitor{}

	config := DefaultStreamerConfig()
	config.MaxConnections = 10
	config.EventBufferSize = 100
	config.BatchSize = 5
	config.FlushInterval = 100 * time.Millisecond
	config.EnableEventFiltering = true
	config.RateLimitPerSecond = 10

	streamer := NewEventStreamer(mockEventRepo, mockMonitor, logger, config)
	return streamer, mockEventRepo, mockMonitor
}

func TestNewEventStreamer(t *testing.T) {
	streamer, _, _ := setupTestEventStreamer(t)

	assert.NotNil(t, streamer)
	assert.NotNil(t, streamer.config)
	assert.NotNil(t, streamer.connections)
	assert.NotNil(t, streamer.eventBuffer)
	assert.NotNil(t, streamer.metrics)
	assert.False(t, streamer.isRunning)
}

func TestEventStreamer_StartStop(t *testing.T) {
	streamer, _, _ := setupTestEventStreamer(t)
	ctx := context.Background()

	// Test start
	err := streamer.Start(ctx)
	require.NoError(t, err)
	assert.True(t, streamer.isRunning)

	// Test double start (should fail)
	err = streamer.Start(ctx)
	assert.Error(t, err)

	// Test stop
	err = streamer.Stop(ctx)
	require.NoError(t, err)
	assert.False(t, streamer.isRunning)

	// Test double stop (should succeed)
	err = streamer.Stop(ctx)
	assert.NoError(t, err)
}

func TestEventStreamer_WebSocketUpgrade(t *testing.T) {
	streamer, _, _ := setupTestEventStreamer(t)
	ctx := context.Background()

	// Start streamer
	err := streamer.Start(ctx)
	require.NoError(t, err)
	defer streamer.Stop(ctx)

	// Create test HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := "test-user"
		err := streamer.HandleWebSocketUpgrade(w, r, &userID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	// Convert HTTP URL to WebSocket URL
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	// Connect WebSocket client
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer ws.Close()

	// Read welcome message
	_, message, err := ws.ReadMessage()
	require.NoError(t, err)

	var welcomeMsg map[string]interface{}
	err = json.Unmarshal(message, &welcomeMsg)
	require.NoError(t, err)

	assert.Equal(t, "welcome", welcomeMsg["type"])
	assert.Contains(t, welcomeMsg, "data")

	// Verify connection is registered
	status := streamer.GetConnectionStatus()
	assert.Equal(t, int64(1), status.Metrics.ActiveConnections)
	assert.Len(t, status.Connections, 1)
	assert.Equal(t, "test-user", *status.Connections[0].UserID)
}

func TestEventStreamer_StreamEvent(t *testing.T) {
	streamer, _, mockMonitor := setupTestEventStreamer(t)
	ctx := context.Background()

	// Set up monitor expectations
	mockMonitor.On("RecordGauge", mock.AnythingOfType("string"), mock.AnythingOfType("float64"), mock.Anything).Maybe()
	mockMonitor.On("RecordCounter", mock.AnythingOfType("string"), mock.AnythingOfType("float64"), mock.Anything).Maybe()
	mockMonitor.On("RecordHistogram", mock.AnythingOfType("string"), mock.AnythingOfType("float64"), mock.Anything).Maybe()

	// Start streamer
	err := streamer.Start(ctx)
	require.NoError(t, err)
	defer streamer.Stop(ctx)

	// Create test event
	event := &audit.Event{
		ID:             "test-event-1",
		EventType:      "user.created",
		Actor:          "test-user",
		EntityType:     "user",
		EntityID:       "user-123",
		SequenceNumber: values.SequenceNumber(1),
		Timestamp:      time.Now(),
		Metadata:       map[string]interface{}{"test": "data"},
	}

	// Stream event
	err = streamer.StreamEvent(ctx, event)
	require.NoError(t, err)

	// Wait for event processing
	time.Sleep(200 * time.Millisecond)

	// Verify metrics updated
	status := streamer.GetConnectionStatus()
	assert.GreaterOrEqual(t, status.Metrics.EventsStreamed, int64(1))

	mockMonitor.AssertExpectations(t)
}

func TestEventStreamer_AddRemoveFilter(t *testing.T) {
	streamer, _, _ := setupTestEventStreamer(t)
	ctx := context.Background()

	// Start streamer
	err := streamer.Start(ctx)
	require.NoError(t, err)
	defer streamer.Stop(ctx)

	// Create a test connection
	conn := &StreamConnection{
		ID:          "test-conn",
		ConnectedAt: time.Now(),
		LastActive:  time.Now(),
		IsActive:    true,
		Filters:     make([]*StreamFilter, 0),
		rateLimiter: NewTokenBucket(10, 10),
	}

	// Register connection
	streamer.connMutex.Lock()
	streamer.connections[conn.ID] = conn
	streamer.connMutex.Unlock()

	// Test add filter
	filter := &StreamFilter{
		Name:       "test-filter",
		EventTypes: []string{"user.created", "user.updated"},
		IsEnabled:  true,
	}

	err = streamer.AddFilter(conn.ID, filter)
	require.NoError(t, err)

	// Verify filter was added
	assert.Len(t, conn.Filters, 1)
	assert.Equal(t, "test-filter", conn.Filters[0].Name)

	// Test remove filter
	err = streamer.RemoveFilter(conn.ID, "test-filter")
	require.NoError(t, err)

	// Verify filter was removed
	assert.Len(t, conn.Filters, 0)

	// Test remove non-existent filter
	err = streamer.RemoveFilter(conn.ID, "non-existent")
	assert.Error(t, err)
}

func TestEventStreamer_EventFiltering(t *testing.T) {
	streamer, _, _ := setupTestEventStreamer(t)

	// Create test event
	event := &audit.Event{
		ID:         "test-event",
		EventType:  "user.created",
		Actor:      "admin",
		EntityType: "user",
		Timestamp:  time.Now(),
		Metadata:   map[string]interface{}{"severity": "high"},
	}

	// Create test connection with filters
	conn := &StreamConnection{
		ID: "test-conn",
		Filters: []*StreamFilter{
			{
				Name:       "user-events",
				EventTypes: []string{"user.created", "user.updated"},
				IsEnabled:  true,
			},
			{
				Name:      "admin-actions",
				Actors:    []string{"admin"},
				IsEnabled: true,
			},
			{
				Name:      "high-severity",
				Custom:    map[string]interface{}{"severity": "high"},
				IsEnabled: true,
			},
		},
	}

	// Test event matches filters
	matches := streamer.eventMatchesFilters(event, conn)
	assert.True(t, matches, "Event should match user-events filter")

	// Test with non-matching event
	nonMatchingEvent := &audit.Event{
		ID:         "test-event-2",
		EventType:  "call.completed",
		Actor:      "user",
		EntityType: "call",
		Timestamp:  time.Now(),
	}

	matches = streamer.eventMatchesFilters(nonMatchingEvent, conn)
	assert.False(t, matches, "Event should not match any filters")

	// Test with disabled filters
	conn.Filters[0].IsEnabled = false
	conn.Filters[1].IsEnabled = false
	conn.Filters[2].IsEnabled = false

	matches = streamer.eventMatchesFilters(event, conn)
	assert.False(t, matches, "Event should not match disabled filters")
}

func TestEventStreamer_RateLimiting(t *testing.T) {
	// Test TokenBucket rate limiting
	bucket := NewTokenBucket(5, 1) // 5 tokens, refill 1 per second

	// Use all tokens
	for i := 0; i < 5; i++ {
		assert.True(t, bucket.Allow(), "Should allow request %d", i+1)
	}

	// Next request should be denied
	assert.False(t, bucket.Allow(), "Should deny request when bucket is empty")

	// Wait for refill (simulate 2 seconds)
	bucket.lastRefill = time.Now().Add(-2 * time.Second)

	// Should allow 2 more requests
	assert.True(t, bucket.Allow(), "Should allow after refill")
	assert.True(t, bucket.Allow(), "Should allow second after refill")
	assert.False(t, bucket.Allow(), "Should deny third after refill")
}

func TestEventStreamer_ConnectionCleanup(t *testing.T) {
	streamer, _, _ := setupTestEventStreamer(t)
	ctx := context.Background()

	// Start streamer
	err := streamer.Start(ctx)
	require.NoError(t, err)
	defer streamer.Stop(ctx)

	// Create expired connection
	expiredConn := &StreamConnection{
		ID:          "expired-conn",
		ConnectedAt: time.Now().Add(-2 * time.Hour),
		LastActive:  time.Now().Add(-2 * time.Hour),
		IsActive:    true,
		Filters:     make([]*StreamFilter, 0),
	}

	// Create active connection
	activeConn := &StreamConnection{
		ID:          "active-conn",
		ConnectedAt: time.Now(),
		LastActive:  time.Now(),
		IsActive:    true,
		Filters:     make([]*StreamFilter, 0),
	}

	// Register connections
	streamer.connMutex.Lock()
	streamer.connections[expiredConn.ID] = expiredConn
	streamer.connections[activeConn.ID] = activeConn
	streamer.connMutex.Unlock()

	// Run cleanup
	streamer.cleanupConnections()

	// Verify expired connection was removed
	streamer.connMutex.RLock()
	_, expiredExists := streamer.connections[expiredConn.ID]
	_, activeExists := streamer.connections[activeConn.ID]
	streamer.connMutex.RUnlock()

	assert.False(t, expiredExists, "Expired connection should be removed")
	assert.True(t, activeExists, "Active connection should remain")
}

func TestEventStreamer_GetConnectionStatus(t *testing.T) {
	streamer, _, _ := setupTestEventStreamer(t)
	ctx := context.Background()

	// Start streamer
	err := streamer.Start(ctx)
	require.NoError(t, err)
	defer streamer.Stop(ctx)

	// Add test connections
	conn1 := &StreamConnection{
		ID:           "conn1",
		UserID:       stringPtr("user1"),
		ConnectedAt:  time.Now(),
		LastActive:   time.Now(),
		IsActive:     true,
		MessagesSent: 5,
	}

	conn2 := &StreamConnection{
		ID:           "conn2",
		UserID:       stringPtr("user2"),
		ConnectedAt:  time.Now(),
		LastActive:   time.Now(),
		IsActive:     true,
		MessagesSent: 3,
	}

	streamer.connMutex.Lock()
	streamer.connections[conn1.ID] = conn1
	streamer.connections[conn2.ID] = conn2
	streamer.connMutex.Unlock()

	// Get status
	status := streamer.GetConnectionStatus()

	assert.True(t, status.IsRunning)
	assert.Len(t, status.Connections, 2)
	assert.Equal(t, int64(2), status.Metrics.ActiveConnections)
	assert.NotNil(t, status.Configuration)
	assert.NotZero(t, status.LastUpdated)
}

func TestEventStreamer_BatchProcessing(t *testing.T) {
	streamer, _, mockMonitor := setupTestEventStreamer(t)
	ctx := context.Background()

	// Set up monitor expectations
	mockMonitor.On("RecordGauge", mock.AnythingOfType("string"), mock.AnythingOfType("float64"), mock.Anything).Maybe()
	mockMonitor.On("RecordCounter", mock.AnythingOfType("string"), mock.AnythingOfType("float64"), mock.Anything).Maybe()
	mockMonitor.On("RecordHistogram", mock.AnythingOfType("string"), mock.AnythingOfType("float64"), mock.Anything).Maybe()

	// Start streamer
	err := streamer.Start(ctx)
	require.NoError(t, err)
	defer streamer.Stop(ctx)

	// Create multiple events to trigger batch processing
	events := make([]*audit.Event, 10)
	for i := 0; i < 10; i++ {
		events[i] = &audit.Event{
			ID:             fmt.Sprintf("event-%d", i),
			EventType:      "test.event",
			Actor:          "test-user",
			EntityType:     "test",
			EntityID:       fmt.Sprintf("entity-%d", i),
			SequenceNumber: values.SequenceNumber(i + 1),
			Timestamp:      time.Now(),
		}
	}

	// Stream events
	for _, event := range events {
		err := streamer.StreamEvent(ctx, event)
		require.NoError(t, err)
	}

	// Wait for batch processing
	time.Sleep(300 * time.Millisecond)

	// Verify events were processed
	status := streamer.GetConnectionStatus()
	assert.GreaterOrEqual(t, status.Metrics.EventsStreamed, int64(10))

	mockMonitor.AssertExpectations(t)
}

func TestEventStreamer_FilterValidation(t *testing.T) {
	streamer, _, _ := setupTestEventStreamer(t)

	tests := []struct {
		name    string
		filter  *StreamFilter
		wantErr bool
	}{
		{
			name: "valid filter",
			filter: &StreamFilter{
				Name:       "test-filter",
				EventTypes: []string{"user.created"},
				IsEnabled:  true,
			},
			wantErr: false,
		},
		{
			name: "missing name",
			filter: &StreamFilter{
				EventTypes: []string{"user.created"},
				IsEnabled:  true,
			},
			wantErr: true,
		},
		{
			name: "invalid time range",
			filter: &StreamFilter{
				Name: "test-filter",
				TimeRange: &audit.TimeRange{
					Start: time.Now(),
					End:   time.Now().Add(-1 * time.Hour),
				},
				IsEnabled: true,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := streamer.validateFilter(tt.filter)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestEventStreamer_MetricsUpdates(t *testing.T) {
	streamer, _, _ := setupTestEventStreamer(t)

	// Test metrics updates
	initialEvents := streamer.metrics.EventsStreamed

	streamer.updateMetrics(func(m *StreamingMetrics) {
		m.EventsStreamed += 5
		m.ActiveConnections = 3
	})

	snapshot := streamer.getMetricsSnapshot()
	assert.Equal(t, initialEvents+5, snapshot.EventsStreamed)
	assert.Equal(t, int64(3), snapshot.ActiveConnections)
}

// Benchmark tests

func BenchmarkEventStreamer_StreamEvent(b *testing.B) {
	streamer, _, _ := setupTestEventStreamer(&testing.T{})
	ctx := context.Background()

	// Start streamer
	streamer.Start(ctx)
	defer streamer.Stop(ctx)

	event := &audit.Event{
		ID:             "bench-event",
		EventType:      "test.event",
		Actor:          "test-user",
		EntityType:     "test",
		EntityID:       "entity-1",
		SequenceNumber: values.SequenceNumber(1),
		Timestamp:      time.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		streamer.StreamEvent(ctx, event)
	}
}

func BenchmarkEventStreamer_EventFiltering(b *testing.B) {
	streamer, _, _ := setupTestEventStreamer(&testing.T{})

	event := &audit.Event{
		ID:         "bench-event",
		EventType:  "user.created",
		Actor:      "admin",
		EntityType: "user",
		Timestamp:  time.Now(),
	}

	conn := &StreamConnection{
		ID: "bench-conn",
		Filters: []*StreamFilter{
			{
				Name:       "user-events",
				EventTypes: []string{"user.created", "user.updated"},
				IsEnabled:  true,
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		streamer.eventMatchesFilters(event, conn)
	}
}

func BenchmarkTokenBucket_Allow(b *testing.B) {
	bucket := NewTokenBucket(1000, 100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bucket.Allow()
	}
}

// Helper functions

func stringPtr(s string) *string {
	return &s
}

// Integration test with actual WebSocket connection
func TestEventStreamer_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	streamer, _, mockMonitor := setupTestEventStreamer(t)
	ctx := context.Background()

	// Set up monitor expectations
	mockMonitor.On("RecordGauge", mock.AnythingOfType("string"), mock.AnythingOfType("float64"), mock.Anything).Maybe()
	mockMonitor.On("RecordCounter", mock.AnythingOfType("string"), mock.AnythingOfType("float64"), mock.Anything).Maybe()
	mockMonitor.On("RecordHistogram", mock.AnythingOfType("string"), mock.AnythingOfType("float64"), mock.Anything).Maybe()

	// Start streamer
	err := streamer.Start(ctx)
	require.NoError(t, err)
	defer streamer.Stop(ctx)

	// Create test HTTP server with WebSocket endpoint
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := "integration-test-user"
		err := streamer.HandleWebSocketUpgrade(w, r, &userID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	// Convert HTTP URL to WebSocket URL
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	// Connect WebSocket client
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer ws.Close()

	// Read welcome message
	_, welcomeMessage, err := ws.ReadMessage()
	require.NoError(t, err)

	var welcomeMsg map[string]interface{}
	err = json.Unmarshal(welcomeMessage, &welcomeMsg)
	require.NoError(t, err)
	assert.Equal(t, "welcome", welcomeMsg["type"])

	// Send add filter message
	filterMsg := map[string]interface{}{
		"type": "add_filter",
		"filter": map[string]interface{}{
			"name":        "integration-filter",
			"event_types": []string{"integration.test"},
		},
	}

	filterData, err := json.Marshal(filterMsg)
	require.NoError(t, err)

	err = ws.WriteMessage(websocket.TextMessage, filterData)
	require.NoError(t, err)

	// Stream matching event
	event := &audit.Event{
		ID:             "integration-event",
		EventType:      "integration.test",
		Actor:          "test-user",
		EntityType:     "test",
		EntityID:       "test-entity",
		SequenceNumber: values.SequenceNumber(1),
		Timestamp:      time.Now(),
		Metadata:       map[string]interface{}{"test": "integration"},
	}

	err = streamer.StreamEvent(ctx, event)
	require.NoError(t, err)

	// Wait for message processing
	time.Sleep(200 * time.Millisecond)

	// Set read timeout
	ws.SetReadDeadline(time.Now().Add(1 * time.Second))

	// Read event message
	_, eventMessage, err := ws.ReadMessage()
	require.NoError(t, err)

	var eventMsg map[string]interface{}
	err = json.Unmarshal(eventMessage, &eventMsg)
	require.NoError(t, err)

	assert.Equal(t, "audit_event", eventMsg["type"])
	assert.Contains(t, eventMsg, "data")

	// Verify event data
	eventData := eventMsg["data"].(map[string]interface{})
	assert.Equal(t, "integration-event", eventData["id"])
	assert.Equal(t, "integration.test", eventData["event_type"])

	// Send ping message
	pingMsg := map[string]interface{}{
		"type": "ping",
	}

	pingData, err := json.Marshal(pingMsg)
	require.NoError(t, err)

	err = ws.WriteMessage(websocket.TextMessage, pingData)
	require.NoError(t, err)

	// Read pong response
	ws.SetReadDeadline(time.Now().Add(1 * time.Second))
	_, pongMessage, err := ws.ReadMessage()
	require.NoError(t, err)

	var pongMsg map[string]interface{}
	err = json.Unmarshal(pongMessage, &pongMsg)
	require.NoError(t, err)
	assert.Equal(t, "pong", pongMsg["type"])

	// Verify final state
	status := streamer.GetConnectionStatus()
	assert.True(t, status.IsRunning)
	assert.Equal(t, int64(1), status.Metrics.ActiveConnections)
	assert.GreaterOrEqual(t, status.Metrics.EventsStreamed, int64(1))

	mockMonitor.AssertExpectations(t)
}
