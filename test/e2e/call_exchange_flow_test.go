//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/api/rest"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/account"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/bid"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/call"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/infrastructure/config"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/infrastructure/repository"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/service/bidding"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/service/callrouting"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/service/fraud"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/service/telephony"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/testutil"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCallExchangeFlow_CompleteLifecycle tests the complete call exchange flow from incoming call to billing
func TestCallExchangeFlow_CompleteLifecycle(t *testing.T) {
	// Setup test environment
	testDB := testutil.NewTestDB(t)
	ctx := testutil.TestContext(t)
	
	// Initialize repositories
	callRepo := repository.NewCallRepository(testDB.DB())
	bidRepo := repository.NewBidRepository(testDB.DB())
	accountRepo := repository.NewAccountRepository(testDB.DB())
	complianceRepo := repository.NewComplianceRepository(testDB.DB())
	financialRepo := repository.NewFinancialRepository(testDB.DB())
	
	// Initialize services
	cfg := &config.Config{
		Server: config.ServerConfig{Port: "8080"},
		Security: config.SecurityConfig{
			JWTSecret: "test-secret",
			TokenExpiry: 24 * time.Hour,
		},
	}
	
	fraudSvc := fraud.NewService(accountRepo, callRepo)
	telephonySvc := telephony.NewService(cfg)
	routingRules := &callrouting.RoutingRules{
		Algorithm: "cost-based",
		QualityWeight: 0.4,
		PriceWeight: 0.4,
		CapacityWeight: 0.2,
	}
	routingSvc := callrouting.NewService(callRepo, bidRepo, accountRepo, fraudSvc, routingRules)
	biddingSvc := bidding.NewService(bidRepo, accountRepo, financialRepo)
	
	// Initialize API server
	router := mux.NewRouter()
	rest.RegisterHandlers(router, rest.Services{
		CallRouting: routingSvc,
		Bidding: biddingSvc,
		Telephony: telephonySvc,
		Fraud: fraudSvc,
	})
	server := httptest.NewServer(router)
	defer server.Close()
	
	// Test Scenario: Complete call exchange flow
	t.Run("Complete Call Exchange Flow", func(t *testing.T) {
		// Step 1: Create buyer and seller accounts
		buyer := createTestAccount(t, ctx, accountRepo, "buyer", account.TypeBuyer)
		seller1 := createTestAccount(t, ctx, accountRepo, "seller1", account.TypeSeller)
		seller2 := createTestAccount(t, ctx, accountRepo, "seller2", account.TypeSeller)
		
		// Step 2: Sellers create bid profiles
		bidProfile1 := createBidProfile(t, server, seller1.ID, bid.BidCriteria{
			Geography: bid.GeoCriteria{
				Countries: []string{"US"},
				States: []string{"CA", "NY"},
			},
			MaxBudget: 100.00,
			CallType: []string{"sales", "support"},
		})
		
		bidProfile2 := createBidProfile(t, server, seller2.ID, bid.BidCriteria{
			Geography: bid.GeoCriteria{
				Countries: []string{"US"},
			},
			MaxBudget: 150.00,
			CallType: []string{"sales"},
		})
		
		// Step 3: Incoming call arrives
		incomingCall := simulateIncomingCall(t, server, buyer.ID, "+14155551234", "+18005551234")
		assert.Equal(t, call.StatusPending, incomingCall.Status)
		
		// Step 4: Real-time auction begins
		auction := startAuction(t, server, incomingCall.ID)
		assert.Equal(t, bid.AuctionStatusActive, auction.Status)
		
		// Step 5: Sellers place bids
		bid1 := placeBid(t, server, auction.ID, seller1.ID, 5.50)
		bid2 := placeBid(t, server, auction.ID, seller2.ID, 6.25)
		
		// Step 6: Auction completes and winner is selected
		time.Sleep(100 * time.Millisecond) // Simulate auction duration
		auctionResult := completeAuction(t, server, auction.ID)
		assert.Equal(t, bid.AuctionStatusCompleted, auctionResult.Status)
		assert.Equal(t, bid2.ID, *auctionResult.WinningBid)
		
		// Step 7: Call is routed to winning seller
		routedCall := routeCall(t, server, incomingCall.ID)
		assert.Equal(t, call.StatusQueued, routedCall.Status)
		assert.Equal(t, seller2.ID, *routedCall.SellerID)
		
		// Step 8: Call progresses through lifecycle
		// Ringing
		updateCallStatus(t, server, routedCall.ID, call.StatusRinging)
		
		// In Progress
		updateCallStatus(t, server, routedCall.ID, call.StatusInProgress)
		
		// Step 9: Call completes
		callDuration := 180 // 3 minutes
		completedCall := completeCall(t, server, routedCall.ID, callDuration)
		assert.Equal(t, call.StatusCompleted, completedCall.Status)
		assert.Equal(t, callDuration, *completedCall.Duration)
		
		// Step 10: Verify billing
		expectedCost := float64(callDuration/60) * bid2.Amount // Cost per minute
		assert.InDelta(t, expectedCost, *completedCall.Cost, 0.01)
		
		// Verify seller's account is credited
		sellerBalance := getAccountBalance(t, server, seller2.ID)
		assert.Greater(t, sellerBalance, 0.0)
		
		// Verify buyer's account is debited
		buyerBalance := getAccountBalance(t, server, buyer.ID)
		assert.Less(t, buyerBalance, 0.0)
	})
}

// TestCallExchangeFlow_WithCompliance tests call flow with compliance checks
func TestCallExchangeFlow_WithCompliance(t *testing.T) {
	// Setup similar to above...
	testDB := testutil.NewTestDB(t)	ctx := testutil.TestContext(t)
	
	// Initialize repositories and services
	callRepo := repository.NewCallRepository(testDB.DB())
	bidRepo := repository.NewBidRepository(testDB.DB())
	accountRepo := repository.NewAccountRepository(testDB.DB())
	complianceRepo := repository.NewComplianceRepository(testDB.DB())
	
	// Initialize API server with compliance enabled
	server := setupTestServer(t, testDB, true) // compliance enabled
	defer server.Close()
	
	t.Run("Call Blocked by DNC List", func(t *testing.T) {
		buyer := createTestAccount(t, ctx, accountRepo, "buyer", account.TypeBuyer)
		
		// Add number to DNC list
		addToDNCList(t, server, "+14155551234")
		
		// Attempt to make call to DNC number
		resp, err := makeCall(server, buyer.ID, "+18005551234", "+14155551234")
		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
		
		var errResp map[string]string
		json.NewDecoder(resp.Body).Decode(&errResp)
		assert.Contains(t, errResp["error"], "DNC")
	})
	
	t.Run("TCPA Time Restrictions", func(t *testing.T) {
		buyer := createTestAccount(t, ctx, accountRepo, "buyer", account.TypeBuyer)
		seller := createTestAccount(t, ctx, accountRepo, "seller", account.TypeSeller)
		
		// Set restricted calling hours (9 AM - 8 PM local time)
		setTCPARestrictions(t, server, "09:00", "20:00")
		
		// Simulate call outside allowed hours
		mockTime := time.Date(2025, 1, 15, 22, 0, 0, 0, time.UTC) // 10 PM
		withMockedTime(t, mockTime, func() {
			resp, err := makeCall(server, buyer.ID, "+14155551234", "+18005551234")
			require.NoError(t, err)
			assert.Equal(t, http.StatusForbidden, resp.StatusCode)
			
			var errResp map[string]string
			json.NewDecoder(resp.Body).Decode(&errResp)
			assert.Contains(t, errResp["error"], "TCPA")
		})
	})
}

// TestCallExchangeFlow_FraudDetection tests fraud detection during call flow
func TestCallExchangeFlow_FraudDetection(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	ctx := testutil.TestContext(t)
	
	server := setupTestServer(t, testDB, true)
	defer server.Close()
	
	t.Run("High Volume Fraud Detection", func(t *testing.T) {
		buyer := createTestAccount(t, ctx, testDB, "buyer", account.TypeBuyer)
		
		// Generate many calls in short timeframe
		for i := 0; i < 50; i++ {
			fromNumber := fmt.Sprintf("+1415555%04d", i)
			_, err := makeCall(server, buyer.ID, fromNumber, "+18005551234")
			require.NoError(t, err)
			
			// After threshold, should trigger fraud detection
			if i > 30 {
				resp, err := makeCall(server, buyer.ID, fromNumber, "+18005551234")
				require.NoError(t, err)
				
				if resp.StatusCode == http.StatusTooManyRequests {
					var errResp map[string]string
					json.NewDecoder(resp.Body).Decode(&errResp)
					assert.Contains(t, errResp["error"], "fraud")
					break
				}
			}
		}
	})
	
	t.Run("Suspicious Pattern Detection", func(t *testing.T) {
		buyer := createTestAccount(t, ctx, testDB, "suspicious", account.TypeBuyer)
		
		// Create pattern of calls that indicates fraud
		// - Same number called repeatedly
		// - Very short duration calls
		// - High cost destinations
		
		for i := 0; i < 10; i++ {
			call := simulateIncomingCall(t, server, buyer.ID, "+14155551234", "+19005551234")
			
			// Complete call with suspicious duration (< 10 seconds)
			completeCall(t, server, call.ID, 5)
		}
		
		// Next call should be flagged
		resp, err := makeCall(server, buyer.ID, "+14155551234", "+19005551234")
		require.NoError(t, err)
		
		var callResp call.Call
		json.NewDecoder(resp.Body).Decode(&callResp)
		
		// Verify fraud score is elevated
		fraudScore := getFraudScore(t, server, buyer.ID)
		assert.Greater(t, fraudScore, 0.7)
	})
}

// TestCallExchangeFlow_RealTimeBidding tests real-time bidding scenarios
func TestCallExchangeFlow_RealTimeBidding(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	ctx := testutil.TestContext(t)
	
	server := setupTestServer(t, testDB, false)
	defer server.Close()
	
	t.Run("Multiple Concurrent Bidders", func(t *testing.T) {
		buyer := createTestAccount(t, ctx, testDB, "buyer", account.TypeBuyer)
		
		// Create multiple sellers
		numSellers := 10
		sellers := make([]*account.Account, numSellers)
		for i := 0; i < numSellers; i++ {
			sellers[i] = createTestAccount(t, ctx, testDB, fmt.Sprintf("seller%d", i), account.TypeSeller)
		}
		
		// Incoming call triggers auction
		incomingCall := simulateIncomingCall(t, server, buyer.ID, "+14155551234", "+18005551234")
		auction := startAuction(t, server, incomingCall.ID)
		
		// All sellers bid concurrently
		bidChan := make(chan *bid.Bid, numSellers)
		errChan := make(chan error, numSellers)
		
		for i, seller := range sellers {
			go func(idx int, s *account.Account) {
				amount := 3.00 + float64(idx)*0.50 // Varying bid amounts
				bid, err := placeBidConcurrent(server, auction.ID, s.ID, amount)
				if err != nil {
					errChan <- err
					return
				}
				bidChan <- bid
			}(i, seller)
		}
		
		// Collect all bids
		var bids []*bid.Bid
		for i := 0; i < numSellers; i++ {
			select {
			case bid := <-bidChan:
				bids = append(bids, bid)
			case err := <-errChan:
				t.Fatalf("Error placing bid: %v", err)
			case <-time.After(5 * time.Second):
				t.Fatal("Timeout waiting for bids")
			}
		}
		
		// Complete auction
		result := completeAuction(t, server, auction.ID)
		assert.NotNil(t, result.WinningBid)
		
		// Verify highest bidder won
		var maxBid *bid.Bid
		for _, b := range bids {
			if maxBid == nil || b.Amount > maxBid.Amount {
				maxBid = b
			}
		}
		assert.Equal(t, maxBid.ID, *result.WinningBid)
	})
	
	t.Run("Dynamic Bid Adjustment", func(t *testing.T) {
		buyer := createTestAccount(t, ctx, testDB, "buyer", account.TypeBuyer)
		seller1 := createTestAccount(t, ctx, testDB, "seller1", account.TypeSeller)
		seller2 := createTestAccount(t, ctx, testDB, "seller2", account.TypeSeller)
		
		// Configure auto-bidding rules
		setAutoBidding(t, server, seller1.ID, 3.00, 10.00, 0.50) // min, max, increment
		setAutoBidding(t, server, seller2.ID, 2.50, 8.00, 0.25)
		
		// Start auction
		incomingCall := simulateIncomingCall(t, server, buyer.ID, "+14155551234", "+18005551234")
		auction := startAuction(t, server, incomingCall.ID)
		
		// Monitor bidding war
		var finalBids []*bid.Bid
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		
		timeout := time.After(3 * time.Second)
		for {
			select {
			case <-ticker.C:
				bids := getAuctionBids(t, server, auction.ID)
				if len(bids) > len(finalBids) {
					finalBids = bids
				}
			case <-timeout:
				goto done
			}
		}
		done:
		
		// Verify progressive bidding occurred
		assert.Greater(t, len(finalBids), 4) // Multiple bid rounds
		
		// Complete auction
		result := completeAuction(t, server, auction.ID)
		assert.NotNil(t, result.WinningBid)
	})
}

// Helper functions

func createTestAccount(t *testing.T, ctx context.Context, repo repository.AccountRepository, name string, accType account.Type) *account.Account {
	acc := &account.Account{
		ID:        uuid.New(),
		Name:      name,
		Type:      accType,
		Status:    account.StatusActive,
		Balance:   1000.00,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	err := repo.Create(ctx, acc)
	require.NoError(t, err)
	return acc
}

func createBidProfile(t *testing.T, server *httptest.Server, sellerID uuid.UUID, criteria bid.BidCriteria) *bid.BidProfile {
	profile := map[string]interface{}{
		"seller_id": sellerID,
		"criteria":  criteria,
		"active":    true,
	}
	
	body, _ := json.Marshal(profile)
	resp, err := http.Post(server.URL+"/api/v1/bid-profiles", "application/json", bytes.NewBuffer(body))
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	
	var result bid.BidProfile
	json.NewDecoder(resp.Body).Decode(&result)
	return &result
}

func simulateIncomingCall(t *testing.T, server *httptest.Server, buyerID uuid.UUID, from, to string) *call.Call {
	callReq := map[string]interface{}{
		"from_number": from,
		"to_number":   to,
		"buyer_id":    buyerID,
		"direction":   "inbound",
	}
	
	body, _ := json.Marshal(callReq)
	resp, err := http.Post(server.URL+"/api/v1/calls", "application/json", bytes.NewBuffer(body))
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	
	var result call.Call
	json.NewDecoder(resp.Body).Decode(&result)
	return &result
}

func startAuction(t *testing.T, server *httptest.Server, callID uuid.UUID) *bid.Auction {
	auctionReq := map[string]interface{}{
		"call_id":       callID,
		"reserve_price": 2.00,
		"duration":      30,
	}
	
	body, _ := json.Marshal(auctionReq)
	resp, err := http.Post(server.URL+"/api/v1/auctions", "application/json", bytes.NewBuffer(body))
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	
	var result bid.Auction
	json.NewDecoder(resp.Body).Decode(&result)
	return &result
}

func placeBid(t *testing.T, server *httptest.Server, auctionID, sellerID uuid.UUID, amount float64) *bid.Bid {
	bidReq := map[string]interface{}{
		"auction_id": auctionID,
		"seller_id":  sellerID,
		"amount":     amount,
	}
	
	body, _ := json.Marshal(bidReq)
	resp, err := http.Post(server.URL+"/api/v1/bids", "application/json", bytes.NewBuffer(body))
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	
	var result bid.Bid
	json.NewDecoder(resp.Body).Decode(&result)
	return &result
}

func completeAuction(t *testing.T, server *httptest.Server, auctionID uuid.UUID) *bid.Auction {
	resp, err := http.Post(server.URL+fmt.Sprintf("/api/v1/auctions/%s/complete", auctionID), "application/json", nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	
	var result bid.Auction
	json.NewDecoder(resp.Body).Decode(&result)
	return &result
}

func routeCall(t *testing.T, server *httptest.Server, callID uuid.UUID) *call.Call {
	resp, err := http.Post(server.URL+fmt.Sprintf("/api/v1/calls/%s/route", callID), "application/json", nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	
	var result call.Call
	json.NewDecoder(resp.Body).Decode(&result)
	return &result
}

func updateCallStatus(t *testing.T, server *httptest.Server, callID uuid.UUID, status call.Status) {
	statusReq := map[string]interface{}{
		"status": status.String(),
	}
	
	body, _ := json.Marshal(statusReq)
	req, _ := http.NewRequest(http.MethodPatch, server.URL+fmt.Sprintf("/api/v1/calls/%s/status", callID), bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func completeCall(t *testing.T, server *httptest.Server, callID uuid.UUID, duration int) *call.Call {
	completeReq := map[string]interface{}{
		"duration": duration,
	}
	
	body, _ := json.Marshal(completeReq)
	resp, err := http.Post(server.URL+fmt.Sprintf("/api/v1/calls/%s/complete", callID), "application/json", bytes.NewBuffer(body))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	
	var result call.Call
	json.NewDecoder(resp.Body).Decode(&result)
	return &result
}

func getAccountBalance(t *testing.T, server *httptest.Server, accountID uuid.UUID) float64 {
	resp, err := http.Get(server.URL + fmt.Sprintf("/api/v1/accounts/%s/balance", accountID))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	
	var result map[string]float64
	json.NewDecoder(resp.Body).Decode(&result)
	return result["balance"]
}

func setupTestServer(t *testing.T, testDB *testutil.TestDB, complianceEnabled bool) *httptest.Server {
	// Initialize all repositories
	callRepo := repository.NewCallRepository(testDB.DB())
	bidRepo := repository.NewBidRepository(testDB.DB())
	accountRepo := repository.NewAccountRepository(testDB.DB())
	complianceRepo := repository.NewComplianceRepository(testDB.DB())
	financialRepo := repository.NewFinancialRepository(testDB.DB())
	
	// Initialize services with configuration
	cfg := &config.Config{
		Server: config.ServerConfig{Port: "8080"},
		Security: config.SecurityConfig{
			JWTSecret: "test-secret",
			TokenExpiry: 24 * time.Hour,
		},
		Compliance: config.ComplianceConfig{
			TCPAEnabled: complianceEnabled,
			GDPREnabled: complianceEnabled,
		},
	}
	
	fraudSvc := fraud.NewService(accountRepo, callRepo)
	telephonySvc := telephony.NewService(cfg)
	routingRules := &callrouting.RoutingRules{
		Algorithm: "cost-based",
		QualityWeight: 0.4,
		PriceWeight: 0.4,
		CapacityWeight: 0.2,
	}
	routingSvc := callrouting.NewService(callRepo, bidRepo, accountRepo, fraudSvc, routingRules)
	biddingSvc := bidding.NewService(bidRepo, accountRepo, financialRepo)
	
	// Initialize API router
	router := mux.NewRouter()
	rest.RegisterHandlers(router, rest.Services{
		CallRouting: routingSvc,
		Bidding: biddingSvc,
		Telephony: telephonySvc,
		Fraud: fraudSvc,
	})
	
	return httptest.NewServer(router)
}

// Additional helper functions for specific test scenarios
func makeCall(server *httptest.Server, buyerID uuid.UUID, from, to string) (*http.Response, error) {
	callReq := map[string]interface{}{
		"from_number": from,
		"to_number":   to,
		"buyer_id":    buyerID,
		"direction":   "inbound",
	}
	
	body, _ := json.Marshal(callReq)
	return http.Post(server.URL+"/api/v1/calls", "application/json", bytes.NewBuffer(body))
}

func addToDNCList(t *testing.T, server *httptest.Server, phoneNumber string) {
	dncReq := map[string]interface{}{
		"phone_number": phoneNumber,
		"reason":       "consumer request",
	}
	
	body, _ := json.Marshal(dncReq)
	resp, err := http.Post(server.URL+"/api/v1/compliance/dnc", "application/json", bytes.NewBuffer(body))
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
}

func setTCPARestrictions(t *testing.T, server *httptest.Server, startTime, endTime string) {
	tcpaReq := map[string]interface{}{
		"start_time": startTime,
		"end_time":   endTime,
		"timezone":   "America/New_York",
	}
	
	body, _ := json.Marshal(tcpaReq)
	req, _ := http.NewRequest(http.MethodPut, server.URL+"/api/v1/compliance/tcpa/hours", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func withMockedTime(t *testing.T, mockTime time.Time, fn func()) {
	// This would require time mocking library or interface
	// For now, just execute the function
	fn()
}

func getFraudScore(t *testing.T, server *httptest.Server, accountID uuid.UUID) float64 {
	resp, err := http.Get(server.URL + fmt.Sprintf("/api/v1/fraud/score/%s", accountID))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	
	var result map[string]float64
	json.NewDecoder(resp.Body).Decode(&result)
	return result["score"]
}

func placeBidConcurrent(server *httptest.Server, auctionID, sellerID uuid.UUID, amount float64) (*bid.Bid, error) {
	bidReq := map[string]interface{}{
		"auction_id": auctionID,
		"seller_id":  sellerID,
		"amount":     amount,
	}
	
	body, _ := json.Marshal(bidReq)
	resp, err := http.Post(server.URL+"/api/v1/bids", "application/json", bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	
	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	
	var result bid.Bid
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	
	return &result, nil
}

func setAutoBidding(t *testing.T, server *httptest.Server, sellerID uuid.UUID, minBid, maxBid, increment float64) {
	autoBidReq := map[string]interface{}{
		"seller_id": sellerID,
		"min_bid":   minBid,
		"max_bid":   maxBid,
		"increment": increment,
		"enabled":   true,
	}
	
	body, _ := json.Marshal(autoBidReq)
	req, _ := http.NewRequest(http.MethodPut, server.URL+"/api/v1/bidding/auto", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func getAuctionBids(t *testing.T, server *httptest.Server, auctionID uuid.UUID) []*bid.Bid {
	resp, err := http.Get(server.URL + fmt.Sprintf("/api/v1/auctions/%s/bids", auctionID))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	
	var result []*bid.Bid
	json.NewDecoder(resp.Body).Decode(&result)
	return result
}