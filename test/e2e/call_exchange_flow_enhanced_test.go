//go:build e2e

package e2e

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/account"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/bid"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/call"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/davidleathers/dependable-call-exchange-backend/test/e2e/infrastructure"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCallExchangeFlow_Enhanced(t *testing.T) {
	env := infrastructure.NewTestEnvironment(t)
	client := infrastructure.NewAPIClient(t, env.APIURL)
	
	t.Run("Complete Bid Profile Lifecycle", func(t *testing.T) {
		env.ResetDatabase()
		
		// Create seller account
		seller := createAccountInDB(t, env, "seller", account.TypeSeller, 0.00)
		sellerAuth := authenticateAccount(t, client, seller.Email.String())
		client.SetToken(sellerAuth.Token)
		
		// Create bid profile
		profileResp := client.Post("/api/v1/bid-profiles", map[string]interface{}{
			"criteria": map[string]interface{}{
				"geography": map[string]interface{}{
					"countries": []string{"US"},
					"states":    []string{"CA", "NY", "TX"},
				},
				"call_type":  []string{"sales", "support"},
				"max_budget": 150.00,
				"keywords":   []string{"insurance", "auto", "home"},
			},
			"active": true,
		})
		require.Equal(t, 201, profileResp.StatusCode)
		
		var profile bid.BidProfile
		client.DecodeResponse(profileResp, &profile)
		
		// Update bid profile
		updateResp := client.Put("/api/v1/bid-profiles/"+profile.ID.String(), map[string]interface{}{
			"criteria": map[string]interface{}{
				"max_budget": 200.00,
			},
		})
		assert.Equal(t, 200, updateResp.StatusCode)
		
		// List bid profiles
		listResp := client.Get("/api/v1/bid-profiles")
		assert.Equal(t, 200, listResp.StatusCode)
		
		var profiles []bid.BidProfile
		client.DecodeResponse(listResp, &profiles)
		assert.Len(t, profiles, 1)
		
		// Delete bid profile
		deleteResp := client.Delete("/api/v1/bid-profiles/" + profile.ID.String())
		assert.Equal(t, 204, deleteResp.StatusCode)
	})
	
	t.Run("Multi-Seller Auction Competition", func(t *testing.T) {
		env.ResetDatabase()
		
		// Create multiple sellers with different bid profiles
		sellers := []struct {
			email    string
			criteria bid.BidCriteria
		}{
			{
				email: "premium-seller@example.com",
				criteria: bid.BidCriteria{
					Geography: bid.GeoCriteria{Countries: []string{"US"}},
					CallType:  []string{"sales"},
					MaxBudget: values.MustNewMoneyFromFloat(50.00, values.USD),
				},
			},
			{
				email: "budget-seller@example.com",
				criteria: bid.BidCriteria{
					Geography: bid.GeoCriteria{Countries: []string{"US", "CA"}},
					CallType:  []string{"sales", "support"},
					MaxBudget: values.MustNewMoneyFromFloat(25.00, values.USD),
				},
			},
			{
				email: "specialized-seller@example.com",
				criteria: bid.BidCriteria{
					Geography: bid.GeoCriteria{
						Countries: []string{"US"},
						States:    []string{"CA", "NY"},
					},
					CallType:  []string{"sales"},
					Keywords:  []string{"insurance", "finance"},
					MaxBudget: values.MustNewMoneyFromFloat(75.00, values.USD),
				},
			},
		}
		
		// Create sellers and bid profiles
		for _, s := range sellers {
			sellerAccount := createAccountInDB(t, env, extractNameFromEmail(s.email), account.TypeSeller, 0.00)
			sellerAuth := authenticateAccount(t, client, sellerAccount.Email.String())
			client.SetToken(sellerAuth.Token)
			
			resp := client.Post("/api/v1/bid-profiles", map[string]interface{}{
				"criteria": s.criteria,
				"active":   true,
			})
			require.Equal(t, 201, resp.StatusCode)
		}
		
		// Create buyer and incoming call
		buyer := createAccountInDB(t, env, "buyer", account.TypeBuyer, 1000.00)
		buyerAuth := authenticateAccount(t, client, buyer.Email.String())
		client.SetToken(buyerAuth.Token)
		
		callResp := client.Post("/api/v1/calls", map[string]interface{}{
			"from_number": "+14155551234",
			"to_number":   "+18005551234",
			"direction":   "inbound",
		})
		require.Equal(t, 201, callResp.StatusCode)
		
		var incomingCall call.Call
		client.DecodeResponse(callResp, &incomingCall)
		
		// Create auction
		auctionResp := client.Post("/api/v1/auctions", map[string]interface{}{
			"call_id":       incomingCall.ID,
			"reserve_price": 1.00,
			"duration":      10, // 10 second auction
		})
		require.Equal(t, 201, auctionResp.StatusCode)
		
		var auction bid.Auction
		client.DecodeResponse(auctionResp, &auction)
		
		// Sellers place bids concurrently
		var wg sync.WaitGroup
		bidAmounts := []float64{15.50, 8.25, 22.00}
		bidResults := make(chan bidResult, len(sellers))
		
		for i, s := range sellers {
			wg.Add(1)
			go func(idx int, sellerEmail string, amount float64) {
				defer wg.Done()
				
				// Create new client for concurrent requests
				sellerClient := infrastructure.NewAPIClient(t, env.APIURL)
				sellerAccount := createAccountInDB(t, env, extractNameFromEmail(sellerEmail), account.TypeSeller, 0.00)
				sellerAuth := authenticateAccount(t, sellerClient, sellerAccount.Email.String())
				sellerClient.SetToken(sellerAuth.Token)
				
				bidResp := sellerClient.Post("/api/v1/bids", map[string]interface{}{
					"auction_id": auction.ID,
					"amount":     amount,
				})
				
				bidResults <- bidResult{
					statusCode: bidResp.StatusCode,
					amount:     amount,
					sellerID:   sellerAccount.ID,
				}
			}(i, s.email, bidAmounts[i])
		}
		
		wg.Wait()
		close(bidResults)
		
		// Collect and verify bids
		successfulBids := 0
		var highestBidAmount float64
		var winningSellerID uuid.UUID
		
		for result := range bidResults {
			if result.statusCode == 201 {
				successfulBids++
				if result.amount > highestBidAmount {
					highestBidAmount = result.amount
					winningSellerID = result.sellerID
				}
			}
		}
		
		assert.Equal(t, 3, successfulBids, "All sellers should successfully place bids")
		
		// Complete auction
		client.SetToken(buyerAuth.Token)
		completeResp := client.Post("/api/v1/auctions/"+auction.ID.String()+"/complete", nil)
		require.Equal(t, 200, completeResp.StatusCode)
		
		var completedAuction bid.Auction
		client.DecodeResponse(completeResp, &completedAuction)
		
		// Verify highest bidder won
		assert.Equal(t, bid.AuctionStatusCompleted, completedAuction.Status)
		assert.NotNil(t, completedAuction.WinningBid)
		
		// Route call to winner
		routeResp := client.Post("/api/v1/calls/"+incomingCall.ID.String()+"/route", nil)
		assert.Equal(t, 200, routeResp.StatusCode)
		
		var routedCall call.Call
		client.DecodeResponse(routeResp, &routedCall)
		assert.NotNil(t, routedCall.SellerID)
		assert.Equal(t, winningSellerID, *routedCall.SellerID)
		
		// Progress call through lifecycle
		statusUpdates := []string{"ringing", "in_progress"}
		for _, status := range statusUpdates {
			updateResp := client.Patch("/api/v1/calls/"+incomingCall.ID.String()+"/status", 
				map[string]interface{}{"status": status})
			assert.Equal(t, 200, updateResp.StatusCode)
			time.Sleep(100 * time.Millisecond) // Simulate real timing
		}
		
		// Complete call
		completeCallResp := client.Post("/api/v1/calls/"+incomingCall.ID.String()+"/complete", 
			map[string]interface{}{"duration": 240}) // 4 minutes
		assert.Equal(t, 200, completeCallResp.StatusCode)
		
		// Verify billing
		var completedCall call.Call
		client.DecodeResponse(completeCallResp, &completedCall)
		assert.Greater(t, completedCall.Cost.ToFloat64(), 0.0)
		
		// Check balances
		balanceResp := client.Get("/api/v1/account/balance")
		assert.Equal(t, 200, balanceResp.StatusCode)
		
		var balance map[string]float64
		client.DecodeResponse(balanceResp, &balance)
		assert.Less(t, balance["balance"], 1000.0) // Balance reduced
	})
}

func TestCallExchangeFlow_ErrorScenarios(t *testing.T) {
	env := infrastructure.NewTestEnvironment(t)
	client := infrastructure.NewAPIClient(t, env.APIURL)
	
	t.Run("Routing Without Bids", func(t *testing.T) {
		env.ResetDatabase()
		
		buyer := createAccountInDB(t, env, "buyer", account.TypeBuyer, 1000.00)
		buyerAuth := authenticateAccount(t, client, buyer.Email.String())
		client.SetToken(buyerAuth.Token)
		
		// Create call
		callResp := client.Post("/api/v1/calls", map[string]interface{}{
			"from_number": "+14155551234",
			"to_number":   "+18005551234",
		})
		require.Equal(t, 201, callResp.StatusCode)
		
		var call call.Call
		client.DecodeResponse(callResp, &call)
		
		// Try to route without any bids
		routeResp := client.Post("/api/v1/calls/"+call.ID.String()+"/route", nil)
		assert.Equal(t, 400, routeResp.StatusCode)
		assert.Contains(t, routeResp.Body.String(), "NO_BIDS_AVAILABLE")
	})
	
	t.Run("Complete Call Without Routing", func(t *testing.T) {
		env.ResetDatabase()
		
		buyer := createAccountInDB(t, env, "buyer", account.TypeBuyer, 1000.00)
		buyerAuth := authenticateAccount(t, client, buyer.Email.String())
		client.SetToken(buyerAuth.Token)
		
		// Create call
		callResp := client.Post("/api/v1/calls", map[string]interface{}{
			"from_number": "+14155551234",
			"to_number":   "+18005551234",
		})
		require.Equal(t, 201, callResp.StatusCode)
		
		var call call.Call
		client.DecodeResponse(callResp, &call)
		
		// Try to complete without routing
		completeResp := client.Post("/api/v1/calls/"+call.ID.String()+"/complete", 
			map[string]interface{}{"duration": 180})
		assert.Equal(t, 400, completeResp.StatusCode)
		assert.Contains(t, completeResp.Body.String(), "INVALID_STATE")
	})
	
	t.Run("Insufficient Balance", func(t *testing.T) {
		env.ResetDatabase()
		
		// Create buyer with low balance
		buyer := createAccountInDB(t, env, "buyer", account.TypeBuyer, 0.50) // Very low balance
		buyerAuth := authenticateAccount(t, client, buyer.Email.String())
		
		// Create seller with bid profile
		seller := createAccountInDB(t, env, "seller", account.TypeSeller, 0.00)
		sellerAuth := authenticateAccount(t, client, seller.Email.String())
		client.SetToken(sellerAuth.Token)
		
		createBidProfile(t, client, bid.BidCriteria{
			Geography: bid.GeoCriteria{Countries: []string{"US"}},
			CallType:  []string{"sales"},
			MaxBudget: values.MustNewMoneyFromFloat(100.00, values.USD),
		})
		
		// Create expensive call
		client.SetToken(buyerAuth.Token)
		callResp := client.Post("/api/v1/calls", map[string]interface{}{
			"from_number": "+14155551234",
			"to_number":   "+18005551234",
		})
		require.Equal(t, 201, callResp.StatusCode)
		
		var incomingCall call.Call
		client.DecodeResponse(callResp, &incomingCall)
		
		// Create auction with high reserve price
		auctionResp := client.Post("/api/v1/auctions", map[string]interface{}{
			"call_id":       incomingCall.ID,
			"reserve_price": 10.00, // Higher than buyer's balance
			"duration":      5,
		})
		// Should fail due to insufficient balance
		assert.Equal(t, 400, auctionResp.StatusCode)
		assert.Contains(t, auctionResp.Body.String(), "insufficient balance")
	})
	
	t.Run("Auction Timeout", func(t *testing.T) {
		env.ResetDatabase()
		
		buyer := createAccountInDB(t, env, "buyer", account.TypeBuyer, 1000.00)
		buyerAuth := authenticateAccount(t, client, buyer.Email.String())
		client.SetToken(buyerAuth.Token)
		
		callResp := client.Post("/api/v1/calls", map[string]interface{}{
			"from_number": "+14155551234",
			"to_number":   "+18005551234",
		})
		require.Equal(t, 201, callResp.StatusCode)
		
		var incomingCall call.Call
		client.DecodeResponse(callResp, &incomingCall)
		
		// Create very short auction
		auctionResp := client.Post("/api/v1/auctions", map[string]interface{}{
			"call_id":       incomingCall.ID,
			"reserve_price": 1.00,
			"duration":      1, // 1 second
		})
		require.Equal(t, 201, auctionResp.StatusCode)
		
		var auction bid.Auction
		client.DecodeResponse(auctionResp, &auction)
		
		// Wait for auction to timeout
		time.Sleep(2 * time.Second)
		
		// Try to complete auction after timeout
		completeResp := client.Post("/api/v1/auctions/"+auction.ID.String()+"/complete", nil)
		assert.Equal(t, 400, completeResp.StatusCode)
		assert.Contains(t, completeResp.Body.String(), "auction expired")
	})
}

// Helper types and functions
type bidResult struct {
	statusCode int
	amount     float64
	sellerID   uuid.UUID
}

func extractNameFromEmail(email string) string {
	// Extract name from email for account creation
	return email[:len(email)-12] // Remove "@example.com"
}

type AuthenticatedUser struct {
	Email        string
	Token        string
	RefreshToken string
}

func createAuthenticatedUser(t *testing.T, client *infrastructure.APIClient, email, userType string) *AuthenticatedUser {
	// Register user
	registerReq := map[string]interface{}{
		"email":    email,
		"password": "TestPass123!",
		"name":     "Test User",
		"type":     userType,
	}
	
	registerResp := client.Post("/api/v1/auth/register", registerReq)
	// Don't require success as user might already exist
	
	// Login
	loginReq := map[string]interface{}{
		"email":    email,
		"password": "TestPass123!",
	}
	
	loginResp := client.Post("/api/v1/auth/login", loginReq)
	require.Equal(t, 200, loginResp.StatusCode)
	
	var loginResult map[string]interface{}
	client.DecodeResponse(loginResp, &loginResult)
	
	return &AuthenticatedUser{
		Email:        email,
		Token:        loginResult["token"].(string),
		RefreshToken: loginResult["refresh_token"].(string),
	}
}