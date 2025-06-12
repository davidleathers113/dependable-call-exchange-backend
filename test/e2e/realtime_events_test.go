//go:build e2e

package e2e

import (
	"sync"
	"testing"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/account"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/call"
	"github.com/davidleathers/dependable-call-exchange-backend/test/e2e/infrastructure"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRealTimeEvents_WebSocket(t *testing.T) {
	env := infrastructure.NewTestEnvironment(t)
	apiClient := infrastructure.NewAPIClient(t, env.APIURL)
	
	t.Run("Real-time Bidding Updates", func(t *testing.T) {
		env.ResetDatabase()
		
		// Create test accounts
		buyer := createAccountInDB(t, env, "buyer", account.TypeBuyer, 1000.00)
		seller1 := createAccountInDB(t, env, "seller1", account.TypeSeller, 0.00)
		seller2 := createAccountInDB(t, env, "seller2", account.TypeSeller, 0.00)
		
		// Authenticate accounts
		buyerAuth := authenticateAccount(t, apiClient, buyer.Email.String())
		seller1Auth := authenticateAccount(t, apiClient, seller1.Email.String())
		seller2Auth := authenticateAccount(t, apiClient, seller2.Email.String())
		
		// Connect WebSocket clients for both sellers
		ws1 := infrastructure.NewWebSocketClient(t, env.WSURL+"/ws/bidding")
		err := ws1.Connect(seller1.ID.String())
		require.NoError(t, err)
		defer ws1.Close()
		
		ws2 := infrastructure.NewWebSocketClient(t, env.WSURL+"/ws/bidding")
		err = ws2.Connect(seller2.ID.String())
		require.NoError(t, err)
		defer ws2.Close()
		
		// Subscribe to auction events
		subscribeToAuction(t, ws1, "all")
		subscribeToAuction(t, ws2, "all")
		
		// Create incoming call and start auction
		apiClient.SetToken(buyerAuth.Token)
		incomingCall := simulateIncomingCall(t, apiClient, "+14155551234", "+18005551234")
		auction := startAuction(t, apiClient, incomingCall.ID)
		
		// Both sellers should receive auction started event
		event1 := readWebSocketEvent(t, ws1)
		assert.Equal(t, "auction.started", event1.Type)
		assert.Equal(t, auction.ID.String(), event1.Data["auction_id"])
		
		event2 := readWebSocketEvent(t, ws2)
		assert.Equal(t, "auction.started", event2.Type)
		assert.Equal(t, auction.ID.String(), event2.Data["auction_id"])
		
		// Seller 1 places bid
		apiClient.SetToken(seller1Auth.Token)
		bid1 := placeBid(t, apiClient, auction.ID, 5.50)
		
		// Both sellers should receive bid placed event
		bidEvent1 := readWebSocketEvent(t, ws1)
		assert.Equal(t, "bid.placed", bidEvent1.Type)
		assert.Equal(t, bid1.ID.String(), bidEvent1.Data["bid_id"])
		
		bidEvent2 := readWebSocketEvent(t, ws2)
		assert.Equal(t, "bid.placed", bidEvent2.Type)
		
		// Seller 2 places higher bid
		apiClient.SetToken(seller2Auth.Token)
		bid2 := placeBid(t, apiClient, auction.ID, 6.25)
		
		// Seller 1 should receive outbid notification
		outbidEvent := readWebSocketEvent(t, ws1)
		assert.Equal(t, "bid.outbid", outbidEvent.Type)
		assert.Equal(t, bid1.ID.String(), outbidEvent.Data["previous_bid_id"])
		assert.Equal(t, bid2.ID.String(), outbidEvent.Data["new_bid_id"])
		
		// Complete auction
		apiClient.SetToken(buyerAuth.Token)
		result := completeAuction(t, apiClient, auction.ID)
		
		// Winner should receive won event
		wonEvent := readWebSocketEvent(t, ws2)
		assert.Equal(t, "auction.won", wonEvent.Type)
		assert.Equal(t, result.ID.String(), wonEvent.Data["auction_id"])
		assert.Equal(t, bid2.ID.String(), wonEvent.Data["winning_bid_id"])
		
		// Loser should receive lost event
		lostEvent := readWebSocketEvent(t, ws1)
		assert.Equal(t, "auction.lost", lostEvent.Type)
		assert.Equal(t, result.ID.String(), lostEvent.Data["auction_id"])
	})
	
	t.Run("Call State Updates", func(t *testing.T) {
		env.ResetDatabase()
		
		buyer := createAccountInDB(t, env, "buyer", account.TypeBuyer, 1000.00)
		seller := createAccountInDB(t, env, "seller", account.TypeSeller, 0.00)
		
		buyerAuth := authenticateAccount(t, apiClient, buyer.Email.String())
		sellerAuth := authenticateAccount(t, apiClient, seller.Email.String())
		
		// Connect WebSocket for call events
		wsCall := infrastructure.NewWebSocketClient(t, env.WSURL+"/ws/calls")
		err := wsCall.Connect(seller.ID.String())
		require.NoError(t, err)
		defer wsCall.Close()
		
		// Create and route call
		apiClient.SetToken(buyerAuth.Token)
		incomingCall := simulateIncomingCall(t, apiClient, "+14155551234", "+18005551234")
		auction := startAuction(t, apiClient, incomingCall.ID)
		
		apiClient.SetToken(sellerAuth.Token)
		placeBid(t, apiClient, auction.ID, 5.00)
		
		apiClient.SetToken(buyerAuth.Token)
		completeAuction(t, apiClient, auction.ID)
		routedCall := routeCall(t, apiClient, incomingCall.ID)
		
		// Subscribe to call updates
		subscribeToCall(t, wsCall, routedCall.ID)
		
		// Update call status and verify events
		statuses := []call.Status{
			call.StatusRinging,
			call.StatusInProgress,
		}
		
		for _, status := range statuses {
			updateCallStatus(t, apiClient, routedCall.ID, status)
			
			event := readWebSocketEvent(t, wsCall)
			assert.Equal(t, "call.status_changed", event.Type)
			assert.Equal(t, routedCall.ID.String(), event.Data["call_id"])
			assert.Equal(t, status.String(), event.Data["status"])
		}
		
		// Complete call
		completedCall := completeCall(t, apiClient, routedCall.ID, 180)
		
		// Verify completion event
		completeEvent := readWebSocketEvent(t, wsCall)
		assert.Equal(t, "call.completed", completeEvent.Type)
		assert.Equal(t, completedCall.ID.String(), completeEvent.Data["call_id"])
		assert.Equal(t, float64(180), completeEvent.Data["duration"])
	})
	
	t.Run("Multiple Concurrent Connections", func(t *testing.T) {
		env.ResetDatabase()
		
		// Test that system handles many concurrent WebSocket connections
		numClients := 50
		clients := make([]*infrastructure.WebSocketClient, numClients)
		
		// Connect all clients
		var wg sync.WaitGroup
		for i := 0; i < numClients; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				
				sellerID := uuid.New()
				ws := infrastructure.NewWebSocketClient(t, env.WSURL+"/ws/bidding")
				err := ws.Connect(sellerID.String())
				require.NoError(t, err)
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
		buyer := createAccountInDB(t, env, "buyer", account.TypeBuyer, 1000.00)
		buyerAuth := authenticateAccount(t, apiClient, buyer.Email.String())
		
		apiClient.SetToken(buyerAuth.Token)
		incomingCall := simulateIncomingCall(t, apiClient, "+14155551234", "+18005551234")
		_ = startAuction(t, apiClient, incomingCall.ID)
		
		// Verify all clients receive the event
		received := make(chan bool, numClients)
		for i, ws := range clients {
			go func(idx int, conn *infrastructure.WebSocketClient) {
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

func TestRealTimeEvents_Heartbeat(t *testing.T) {
	env := infrastructure.NewTestEnvironment(t)
	
	t.Run("Heartbeat Keeps Connection Alive", func(t *testing.T) {
		sellerID := uuid.New()
		ws := infrastructure.NewWebSocketClient(t, env.WSURL+"/ws/bidding")
		err := ws.Connect(sellerID.String())
		require.NoError(t, err)
		defer ws.Close()
		
		// Send periodic pings
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		
		pongReceived := make(chan bool)
		ws.Conn.SetPongHandler(func(string) error {
			pongReceived <- true
			return nil
		})
		
		// Send ping and expect pong
		for i := 0; i < 3; i++ {
			err := ws.Conn.WriteMessage(websocket.PingMessage, []byte{})
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
		ws := infrastructure.NewWebSocketClient(t, env.WSURL+"/ws/bidding")
		err := ws.Connect(sellerID.String())
		require.NoError(t, err)
		defer ws.Close()
		
		// Don't send any heartbeats
		// Connection should timeout after configured period
		
		// Try to read, should eventually get close message
		ws.Conn.SetReadDeadline(time.Now().Add(30 * time.Second))
		
		for {
			_, _, err := ws.Conn.ReadMessage()
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
func subscribeToAuction(t *testing.T, ws *infrastructure.WebSocketClient, filter string) {
	msg := map[string]interface{}{
		"action": "subscribe",
		"type":   "auction",
		"filter": filter,
	}
	
	err := ws.Send(msg)
	require.NoError(t, err)
}

func subscribeToCall(t *testing.T, ws *infrastructure.WebSocketClient, callID uuid.UUID) {
	msg := map[string]interface{}{
		"action":  "subscribe",
		"type":    "call",
		"call_id": callID.String(),
	}
	
	err := ws.Send(msg)
	require.NoError(t, err)
}

type WebSocketEvent struct {
	Type      string                 `json:"type"`
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
}

func readWebSocketEvent(t *testing.T, ws *infrastructure.WebSocketClient) WebSocketEvent {
	var event WebSocketEvent
	
	// Set read deadline to avoid hanging forever
	ws.Conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	
	err := ws.Receive(&event)
	require.NoError(t, err)
	
	return event
}
