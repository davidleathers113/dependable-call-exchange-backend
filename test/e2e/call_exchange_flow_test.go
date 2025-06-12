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

func TestCallExchangeFlow_CompleteLifecycle(t *testing.T) {
	// Setup test environment with Testcontainers
	env := infrastructure.NewTestEnvironment(t)
	client := infrastructure.NewAPIClient(t, env.APIURL)
	
	t.Run("Complete Call Exchange Flow", func(t *testing.T) {
		env.ResetDatabase()
		
		// Step 1: Create buyer and seller accounts
		buyer := createAccountInDB(t, env, "buyer", account.TypeBuyer, 1000.00)
		seller1 := createAccountInDB(t, env, "seller1", account.TypeSeller, 0.00)
		seller2 := createAccountInDB(t, env, "seller2", account.TypeSeller, 0.00)
		
		// Authenticate sellers
		seller1Auth := authenticateAccount(t, client, seller1.Email.String())
		seller2Auth := authenticateAccount(t, client, seller2.Email.String())
		
		// Step 2: Sellers create bid profiles
		client.SetToken(seller1Auth.Token)
		_ = createBidProfile(t, client, bid.BidCriteria{
			Geography: bid.GeoCriteria{
				Countries: []string{"US"},
				States:    []string{"CA", "NY"},
			},
			MaxBudget: values.MustNewMoneyFromFloat(100.00, values.USD),
			CallType:  []string{"sales", "support"},
		})
		
		client.SetToken(seller2Auth.Token)
		_ = createBidProfile(t, client, bid.BidCriteria{
			Geography: bid.GeoCriteria{
				Countries: []string{"US"},
			},
			MaxBudget: values.MustNewMoneyFromFloat(150.00, values.USD),
			CallType:  []string{"sales"},
		})
		
		// Step 3: Incoming call arrives
		buyerAuth := authenticateAccount(t, client, buyer.Email.String())
		client.SetToken(buyerAuth.Token)
		
		incomingCall := simulateIncomingCall(t, client, "+14155551234", "+18005551234")
		assert.Equal(t, call.StatusPending, incomingCall.Status)
		
		// Step 4: Real-time auction begins
		auction := startAuction(t, client, incomingCall.ID)
		assert.Equal(t, bid.AuctionStatusActive, auction.Status)
		
		// Step 5: Sellers place bids
		client.SetToken(seller1Auth.Token)
		_ = placeBid(t, client, auction.ID, 5.50)
		
		client.SetToken(seller2Auth.Token)
		bid2 := placeBid(t, client, auction.ID, 6.25)
		
		// Step 6: Auction completes and winner is selected
		time.Sleep(100 * time.Millisecond) // Simulate auction duration
		
		client.SetToken(buyerAuth.Token)
		auctionResult := completeAuction(t, client, auction.ID)
		assert.Equal(t, bid.AuctionStatusCompleted, auctionResult.Status)
		assert.Equal(t, bid2.ID, *auctionResult.WinningBid)
		
		// Step 7: Call is routed to winning seller
		routedCall := routeCall(t, client, incomingCall.ID)
		assert.Equal(t, call.StatusQueued, routedCall.Status)
		assert.Equal(t, seller2.ID, *routedCall.SellerID)
		
		// Step 8: Call progresses through lifecycle
		updateCallStatus(t, client, routedCall.ID, call.StatusRinging)
		updateCallStatus(t, client, routedCall.ID, call.StatusInProgress)
		
		// Step 9: Call completes
		callDuration := 180 // 3 minutes
		completedCall := completeCall(t, client, routedCall.ID, callDuration)
		assert.Equal(t, call.StatusCompleted, completedCall.Status)
		assert.Equal(t, callDuration, *completedCall.Duration)
		
		// Step 10: Verify billing
		expectedCost := float64(callDuration/60) * bid2.Amount.ToFloat64()
		assert.InDelta(t, expectedCost, completedCall.Cost.ToFloat64(), 0.01)
		
		// Verify balances
		sellerBalance := getAccountBalance(t, client, seller2Auth.Token)
		assert.Greater(t, sellerBalance, 0.0)
		
		buyerBalance := getAccountBalance(t, client, buyerAuth.Token)
		assert.Less(t, buyerBalance, 1000.0)
	})
}

func TestCallExchangeFlow_WithCompliance(t *testing.T) {
	env := infrastructure.NewTestEnvironment(t)
	client := infrastructure.NewAPIClient(t, env.APIURL)
	
	t.Run("Call Blocked by DNC List", func(t *testing.T) {
		env.ResetDatabase()
		
		buyer := createAccountInDB(t, env, "buyer", account.TypeBuyer, 1000.00)
		buyerAuth := authenticateAccount(t, client, buyer.Email.String())
		client.SetToken(buyerAuth.Token)
		
		// Add number to DNC list
		adminAuth := createAuthenticatedUser(t, client, "admin@example.com", "admin")
		client.SetToken(adminAuth.Token)
		addToDNCList(t, client, "+14155551234")
		
		// Attempt to make call to DNC number
		client.SetToken(buyerAuth.Token)
		resp := client.Post("/api/v1/calls", map[string]interface{}{
			"from_number": "+18005551234",
			"to_number":   "+14155551234",
		})
		assert.Equal(t, 403, resp.StatusCode)
		
		var errResp map[string]string
		client.DecodeResponse(resp, &errResp)
		assert.Contains(t, errResp["error"], "DNC")
	})
	
	t.Run("TCPA Time Restrictions", func(t *testing.T) {
		env.ResetDatabase()
		
		buyer := createAccountInDB(t, env, "buyer", account.TypeBuyer, 1000.00)
		buyerAuth := authenticateAccount(t, client, buyer.Email.String())
		
		// Set restricted calling hours
		adminAuth := createAuthenticatedUser(t, client, "admin@example.com", "admin")
		client.SetToken(adminAuth.Token)
		setTCPARestrictions(t, client, "09:00", "20:00")
		
		// Simulate call outside allowed hours (would need time mocking)
		client.SetToken(buyerAuth.Token)
		// This test would need time mocking to work properly
		t.Skip("Skipping time-based test without time mocking")
	})
}

func TestCallExchangeFlow_FraudDetection(t *testing.T) {
	env := infrastructure.NewTestEnvironment(t)
	client := infrastructure.NewAPIClient(t, env.APIURL)
	
	t.Run("High Volume Fraud Detection", func(t *testing.T) {
		env.ResetDatabase()
		
		buyer := createAccountInDB(t, env, "buyer", account.TypeBuyer, 1000.00)
		buyerAuth := authenticateAccount(t, client, buyer.Email.String())
		client.SetToken(buyerAuth.Token)
		
		// Generate many calls in short timeframe
		fraudDetected := false
		for i := 0; i < 50; i++ {
			fromNumber := fmt.Sprintf("+1415555%04d", i)
			resp := client.Post("/api/v1/calls", map[string]interface{}{
				"from_number": fromNumber,
				"to_number":   "+18005551234",
			})
			
			if resp.StatusCode == 429 || resp.StatusCode == 403 {
				var errResp map[string]string
				client.DecodeResponse(resp, &errResp)
				if _, ok := errResp["fraud"]; ok {
					fraudDetected = true
					break
				}
			}
			resp.Body.Close()
		}
		
		assert.True(t, fraudDetected, "Fraud detection should trigger after suspicious volume")
	})
}

func TestCallExchangeFlow_RealTimeBidding(t *testing.T) {
	env := infrastructure.NewTestEnvironment(t)
	client := infrastructure.NewAPIClient(t, env.APIURL)
	
	t.Run("Multiple Concurrent Bidders", func(t *testing.T) {
		env.ResetDatabase()
		
		buyer := createAccountInDB(t, env, "buyer", account.TypeBuyer, 1000.00)
		buyerAuth := authenticateAccount(t, client, buyer.Email.String())
		
		// Create multiple sellers
		numSellers := 10
		sellers := make([]*account.Account, numSellers)
		sellerAuths := make([]*AuthenticatedUser, numSellers)
		
		for i := 0; i < numSellers; i++ {
			sellers[i] = createAccountInDB(t, env, fmt.Sprintf("seller%d", i), account.TypeSeller, 0.00)
			sellerAuths[i] = authenticateAccount(t, client, sellers[i].Email.String())
		}
		
		// Create call and start auction
		client.SetToken(buyerAuth.Token)
		incomingCall := simulateIncomingCall(t, client, "+14155551234", "+18005551234")
		auction := startAuction(t, client, incomingCall.ID)
		
		// All sellers bid concurrently
		var wg sync.WaitGroup
		bidChan := make(chan *bid.Bid, numSellers)
		errChan := make(chan error, numSellers)
		
		for i, auth := range sellerAuths {
			wg.Add(1)
			go func(idx int, sellerAuth *AuthenticatedUser) {
				defer wg.Done()
				
				localClient := infrastructure.NewAPIClient(t, env.APIURL)
				localClient.SetToken(sellerAuth.Token)
				
				amount := 3.00 + float64(idx)*0.50
				bid, err := placeBidConcurrent(localClient, auction.ID, amount)
				if err != nil {
					errChan <- err
					return
				}
				bidChan <- bid
			}(i, auth)
		}
		
		wg.Wait()
		close(bidChan)
		close(errChan)
		
		// Collect results
		var bids []*bid.Bid
		for bid := range bidChan {
			bids = append(bids, bid)
		}
		
		for err := range errChan {
			require.NoError(t, err)
		}
		
		// Complete auction
		client.SetToken(buyerAuth.Token)
		result := completeAuction(t, client, auction.ID)
		assert.NotNil(t, result.WinningBid)
		
		// Verify highest bidder won
		var maxBid *bid.Bid
		for _, b := range bids {
			if maxBid == nil || b.Amount.Compare(maxBid.Amount) > 0 {
				maxBid = b
			}
		}
		assert.Equal(t, maxBid.ID, *result.WinningBid)
	})
}

// Helper functions
func createAccountInDB(t *testing.T, env *infrastructure.TestEnvironment, name string, accType account.AccountType, balance float64) *account.Account {
	email, err := values.NewEmail(name + "@example.com")
	require.NoError(t, err)
	
	balanceMoney, err := values.NewMoneyFromFloat(balance, values.USD)
	require.NoError(t, err)
	
	acc := &account.Account{
		ID:        uuid.New(),
		Name:      name,
		Email:     email,
		Type:      accType,
		Status:    account.StatusActive,
		Balance:   balanceMoney,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	// Insert directly into database
	_, err = env.DB.Exec(
		`INSERT INTO accounts (id, name, email, type, status, balance, created_at, updated_at) 
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		acc.ID, acc.Name, acc.Email.String(), acc.Type, acc.Status, acc.Balance.ToFloat64(), acc.CreatedAt, acc.UpdatedAt,
	)
	require.NoError(t, err)
	
	return acc
}

func authenticateAccount(t *testing.T, client *infrastructure.APIClient, email string) *AuthenticatedUser {
	// First register if not exists
	registerReq := map[string]interface{}{
		"email":    email,
		"password": "TestPass123!",
		"name":     "Test User",
		"type":     "buyer",
	}
	
	// Try to register (might fail if already exists)
	client.Post("/api/v1/auth/register", registerReq)
	
	// Login
	loginReq := map[string]interface{}{
		"email":    email,
		"password": "TestPass123!",
	}
	
	resp := client.Post("/api/v1/auth/login", loginReq)
	require.Equal(t, 200, resp.StatusCode)
	
	var loginResp map[string]interface{}
	client.DecodeResponse(resp, &loginResp)
	
	return &AuthenticatedUser{
		Email:        email,
		Token:        loginResp["token"].(string),
		RefreshToken: loginResp["refresh_token"].(string),
	}
}

func createBidProfile(t *testing.T, client *infrastructure.APIClient, criteria bid.BidCriteria) *bid.BidProfile {
	resp := client.Post("/api/v1/bid-profiles", map[string]interface{}{
		"criteria": criteria,
		"active":   true,
	})
	require.Equal(t, 201, resp.StatusCode)
	
	var profile bid.BidProfile
	client.DecodeResponse(resp, &profile)
	return &profile
}

func simulateIncomingCall(t *testing.T, client *infrastructure.APIClient, from, to string) *call.Call {
	resp := client.Post("/api/v1/calls", map[string]interface{}{
		"from_number": from,
		"to_number":   to,
		"direction":   "inbound",
	})
	require.Equal(t, 201, resp.StatusCode)
	
	var result call.Call
	client.DecodeResponse(resp, &result)
	return &result
}

func startAuction(t *testing.T, client *infrastructure.APIClient, callID uuid.UUID) *bid.Auction {
	resp := client.Post("/api/v1/auctions", map[string]interface{}{
		"call_id":       callID,
		"reserve_price": 2.00,
		"duration":      30,
	})
	require.Equal(t, 201, resp.StatusCode)
	
	var result bid.Auction
	client.DecodeResponse(resp, &result)
	return &result
}

func placeBid(t *testing.T, client *infrastructure.APIClient, auctionID uuid.UUID, amount float64) *bid.Bid {
	resp := client.Post("/api/v1/bids", map[string]interface{}{
		"auction_id": auctionID,
		"amount":     amount,
	})
	require.Equal(t, 201, resp.StatusCode)
	
	var result bid.Bid
	client.DecodeResponse(resp, &result)
	return &result
}

func placeBidConcurrent(client *infrastructure.APIClient, auctionID uuid.UUID, amount float64) (*bid.Bid, error) {
	resp := client.Post("/api/v1/bids", map[string]interface{}{
		"auction_id": auctionID,
		"amount":     amount,
	})
	
	if resp.StatusCode != 201 {
		return nil, fmt.Errorf("failed to place bid: status %d", resp.StatusCode)
	}
	
	var result bid.Bid
	client.DecodeResponse(resp, &result)
	return &result, nil
}

func completeAuction(t *testing.T, client *infrastructure.APIClient, auctionID uuid.UUID) *bid.Auction {
	resp := client.Post(fmt.Sprintf("/api/v1/auctions/%s/complete", auctionID), nil)
	require.Equal(t, 200, resp.StatusCode)
	
	var result bid.Auction
	client.DecodeResponse(resp, &result)
	return &result
}

func routeCall(t *testing.T, client *infrastructure.APIClient, callID uuid.UUID) *call.Call {
	resp := client.Post(fmt.Sprintf("/api/v1/calls/%s/route", callID), nil)
	require.Equal(t, 200, resp.StatusCode)
	
	var result call.Call
	client.DecodeResponse(resp, &result)
	return &result
}

func updateCallStatus(t *testing.T, client *infrastructure.APIClient, callID uuid.UUID, status call.Status) {
	resp := client.Patch(fmt.Sprintf("/api/v1/calls/%s/status", callID), map[string]interface{}{
		"status": status.String(),
	})
	require.Equal(t, 200, resp.StatusCode)
}

func completeCall(t *testing.T, client *infrastructure.APIClient, callID uuid.UUID, duration int) *call.Call {
	resp := client.Post(fmt.Sprintf("/api/v1/calls/%s/complete", callID), map[string]interface{}{
		"duration": duration,
	})
	require.Equal(t, 200, resp.StatusCode)
	
	var result call.Call
	client.DecodeResponse(resp, &result)
	return &result
}

func getAccountBalance(t *testing.T, client *infrastructure.APIClient, token string) float64 {
	client.SetToken(token)
	resp := client.Get("/api/v1/account/balance")
	require.Equal(t, 200, resp.StatusCode)
	
	var result map[string]float64
	client.DecodeResponse(resp, &result)
	return result["balance"]
}

func addToDNCList(t *testing.T, client *infrastructure.APIClient, phoneNumber string) {
	resp := client.Post("/api/v1/compliance/dnc", map[string]interface{}{
		"phone_number": phoneNumber,
		"reason":       "consumer request",
	})
	require.Equal(t, 201, resp.StatusCode)
}

func setTCPARestrictions(t *testing.T, client *infrastructure.APIClient, startTime, endTime string) {
	resp := client.Put("/api/v1/compliance/tcpa/hours", map[string]interface{}{
		"start_time": startTime,
		"end_time":   endTime,
		"timezone":   "America/New_York",
	})
	require.Equal(t, 200, resp.StatusCode)
}
