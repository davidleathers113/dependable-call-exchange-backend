//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/bid"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/call"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/testutil"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRealTimeEvents_WebSocket tests real-time event streaming via WebSocket
func TestRealTimeEvents_WebSocket(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	ctx := testutil.TestContext(t)
	
	server := setupTestServer(t, testDB, false)
	defer server.Close()
	
	// Convert HTTP server URL to WebSocket URL
	wsURL := "ws" + server.URL[4:] + "/ws"
	
	t.Run("Real-time Bidding Updates", func(t *testing.T) {
		// Create test accounts
		buyer := createTestAccount(t, ctx, testDB, "buyer", account.TypeBuyer)
		seller1 := createTestAccount(t, ctx, testDB, "seller1", account.TypeSeller)
		seller2 := createTestAccount(t, ctx, testDB, "seller2", account.TypeSeller)
		
		// Connect WebSocket clients for both sellers
		ws1 := connectWebSocket(t, wsURL+"/bidding", seller1.ID)
		defer ws1.Close()
		
		ws2 := connectWebSocket(t, wsURL+"/bidding", seller2.ID)
		defer ws2.Close()
		
		// Subscribe to auction events
		subscribeToAuction(t, ws1, "all")
		subscribeToAuction(t, ws2, "all")
		
		// Create incoming call and start auction
		incomingCall := simulateIncomingCall(t, server, buyer.ID, "+14155551234", "+18005551234")
		auction := startAuction(t, server, incomingCall.ID)
		
		// Both sellers should receive auction started event
		event1 := readWebSocketEvent(t, ws1)
		assert.Equal(t, "auction.started", event1.Type)
		assert.Equal(t, auction.ID, event1.Data["auction_id"])
		
		event2 := readWebSocketEvent(t, ws2)
		assert.Equal(t, "auction.started", event2.Type)
		assert.Equal(t, auction.ID, event2.Data["auction_id"])
		
		// Seller 1 places bid
		bid1 := placeBid(t, server, auction.ID, seller1.ID, 5.50)
		
		// Both sellers should receive bid placed event
		bidEvent1 := readWebSocketEvent(t, ws1)
		assert.Equal(t, "bid.placed", bidEvent1.Type)
		assert.Equal(t, bid1.ID, bidEvent1.Data["bid_id"])
		
		bidEvent2 := readWebSocketEvent(t, ws2)
		assert.Equal(t, "bid.placed", bidEvent2.Type)
		
		// Seller 2 places higher bid
		bid2 := placeBid(t, server, auction.ID, seller2.ID, 6.25)
		
		// Both should receive outbid notification
		outbidEvent := readWebSocketEvent(t, ws1)
		assert.Equal(t, "bid.outbid", outbidEvent.Type)
		assert.Equal(t, bid1.ID, outbidEvent.Data["previous_bid_id"])
		assert.Equal(t, bid2.ID, outbidEvent.Data["new_bid_id"])
		
		// Complete auction
		result := completeAuction(t, server, auction.ID)
		
		// Winner should receive won event
		wonEvent := readWebSocketEvent(t, ws2)
		assert.Equal(t, "auction.won", wonEvent.Type)
		assert.Equal(t, result.ID, wonEvent.Data["auction_id"])
		assert.Equal(t, bid2.ID, wonEvent.Data["winning_bid_id"])
		
		// Loser should receive lost event
		lostEvent := readWebSocketEvent(t, ws1)
		assert.Equal(t, "auction.lost", lostEvent.Type)
		assert.Equal(t, result.ID, lostEvent.Data["auction_id"])
	})
	
	t.Run("Call State Updates", func(t *testing.T) {
		buyer := createTestAccount(t, ctx, testDB, "buyer", account.TypeBuyer)
		seller := createTestAccount(t, ctx, testDB, "seller", account.TypeSeller)
		
		// Connect WebSocket for call events
		wsCall := connectWebSocket(t, wsURL+"/calls", seller.ID)
		defer wsCall.Close()
		
		// Create and route call
		incomingCall := simulateIncomingCall(t, server, buyer.ID, "+14155551234", "+18005551234")
		auction := startAuction(t, server, incomingCall.ID)
		placeBid(t, server, auction.ID, seller.ID, 5.00)
		completeAuction(t, server, auction.ID)
		routedCall := routeCall(t, server, incomingCall.ID)
		
		// Subscribe to call updates
		subscribeToCall(t, wsCall, routedCall.ID)
		
		// Update call status and verify events
		statuses := []call.Status{
			call.StatusRinging,
			call.StatusInProgress,
		}
		
		for _, status := range statuses {
			updateCallStatus(t, server, routedCall.ID, status)
			
			event := readWebSocketEvent(t, wsCall)
			assert.Equal(t, "call.status_changed", event.Type)
			assert.Equal(t, routedCall.ID, event.Data["call_id"])
			assert.Equal(t, status.String(), event.Data["status"])
		}
		
		// Complete call
		completedCall := completeCall(t, server, routedCall.ID, 180)
		
		// Verify completion event
		completeEvent := readWebSocketEvent(t, wsCall)
		assert.Equal(t, "call.completed", completeEvent.Type)
		assert.Equal(t, completedCall.ID, completeEvent.Data["call_id"])
		assert.Equal(t, 180, int(completeEvent.Data["duration"].(float64)))
	})
	
	t.Run("Multiple Concurrent Connections", func(t *testing.T) {
		// Test that system handles many concurrent WebSocket connections
		numClients := 50
		clients := make([]*websocket.Conn, numClients)
		
		// Connect all clients
		var wg sync.WaitGroup
		for i := 0; i < numClients; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				
				sellerID := uuid.New()
				ws := connectWebSocket(t, wsURL+"/bidding", sellerID)
				clients[idx] = ws
				
				// Subscribe to events
				subscribeToAuction(t, ws, "all")
			}(i)
		}
		wg.Wait()
		
		// Cleanup connections
		defer func() {
			for _, ws := range clients {
				if ws != nil {
					ws.Close()
				}
			}
		}()
		
		// Create event that should broadcast to all
		buyer := createTestAccount(t, ctx, testDB, "buyer", account.TypeBuyer)
		incomingCall := simulateIncomingCall(t, server, buyer.ID, "+14155551234", "+18005551234")
		auction := startAuction(t, server, incomingCall.ID)
		
		// Verify all clients receive the event
		received := make(chan bool, numClients)
		for i, ws := range clients {
			go func(idx int, conn *websocket.Conn) {
				event := readWebSocketEvent(t, conn)
				if event.Type == "auction.started" {
					received <- true
				}
			}(i, ws)
		}
		
		// Wait for all to receive
		timeout := time.After(5 * time.Second)
		count := 0
		for count < numClients {
			select {
			case <-received:
				count++
			case <-timeout:
				t.Fatalf("Timeout: only %d/%d clients received event", count, numClients)
			}
		}
		
		assert.Equal(t, numClients, count)
	})
}

// TestRealTimeEvents_Heartbeat tests WebSocket heartbeat and reconnection
func TestRealTimeEvents_Heartbeat(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	server := setupTestServer(t, testDB, false)
	defer server.Close()
	
	wsURL := "ws" + server.URL[4:] + "/ws"
	
	t.Run("Heartbeat Keeps Connection Alive", func(t *testing.T) {
		sellerID := uuid.New()
		ws := connectWebSocket(t, wsURL+"/bidding", sellerID)
		defer ws.Close()
		
		// Send periodic pings
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		
		pongReceived := make(chan bool)
		ws.SetPongHandler(func(string) error {
			pongReceived <- true
			return nil
		})
		
		// Send ping and expect pong
		for i := 0; i < 3; i++ {
			err := ws.WriteMessage(websocket.PingMessage, []byte{})
			require.NoError(t, err)
			
			select {
			case <-pongReceived:
				// Success
			case <-time.After(2 * time.Second):
				t.Fatal("No pong received")
			}
		}
	})
	
	t.Run("Connection Timeout Without Heartbeat", func(t *testing.T) {
		sellerID := uuid.New()
		ws := connectWebSocket(t, wsURL+"/bidding", sellerID)
		defer ws.Close()
		
		// Don't send any heartbeats
		// Connection should timeout after configured period
		
		// Try to read, should eventually get close message
		ws.SetReadDeadline(time.Now().Add(30 * time.Second))
		
		for {
			_, _, err := ws.ReadMessage()
			if err != nil {
				// Verify it's a close error
				closeErr, ok := err.(*websocket.CloseError)
				assert.True(t, ok)
				assert.Equal(t, websocket.CloseGoingAway, closeErr.Code)
				break
			}
		}
	})
}

// Helper functions for WebSocket testing

func connectWebSocket(t *testing.T, wsURL string, clientID uuid.UUID) *websocket.Conn {
	u, err := url.Parse(wsURL)
	require.NoError(t, err)
	
	// Add auth token or client ID to query params
	q := u.Query()
	q.Set("client_id", clientID.String())
	u.RawQuery = q.Encode()
	
	ws, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	require.NoError(t, err)
	
	return ws
}

func subscribeToAuction(t *testing.T, ws *websocket.Conn, filter string) {
	msg := map[string]interface{}{
		"action": "subscribe",
		"type":   "auction",
		"filter": filter,
	}
	
	err := ws.WriteJSON(msg)
	require.NoError(t, err)
}

func subscribeToCall(t *testing.T, ws *websocket.Conn, callID uuid.UUID) {
	msg := map[string]interface{}{
		"action":  "subscribe",
		"type":    "call",
		"call_id": callID,
	}
	
	err := ws.WriteJSON(msg)
	require.NoError(t, err)
}

type WebSocketEvent struct {
	Type      string                 `json:"type"`
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
}

func readWebSocketEvent(t *testing.T, ws *websocket.Conn) WebSocketEvent {
	var event WebSocketEvent
	
	// Set read deadline to avoid hanging forever
	ws.SetReadDeadline(time.Now().Add(5 * time.Second))
	
	err := ws.ReadJSON(&event)
	require.NoError(t, err)
	
	return event
}