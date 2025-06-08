//go:build integration

package integration

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/call"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/bid"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/infrastructure/repository"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/service/callrouting"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCallRouting_EndToEnd tests the complete call routing flow with real database
func TestCallRouting_EndToEnd(t *testing.T) {
	// Setup test database
	testDB := testutil.NewTestDB(t)
	ctx := testutil.TestContext(t)
	
	// Create repositories
	callRepo := repository.NewCallRepository(testDB.DB())
	bidRepo := repository.NewBidRepository(testDB.DB())
	accountRepo := repository.NewAccountRepository(testDB.DB())
	
	// Create test data
	buyerID := uuid.New()
	sellerID1 := uuid.New()
	sellerID2 := uuid.New()
	
	// Create call
	testCall := call.NewCall("+15551234567", "+15559876543", buyerID, call.DirectionInbound)
	err := callRepo.Create(ctx, testCall)
	require.NoError(t, err)
	
	// Create competing bids
	bid1 := &bid.Bid{
		ID:           uuid.New(),
		CallID:       testCall.ID,
		BuyerID:      sellerID1,
		Amount:       5.50,
		Status:       bid.StatusActive,
		QualityScore: 85.0,
		PlacedAt:     time.Now(),
		ExpiresAt:    time.Now().Add(5 * time.Minute),
		Criteria: map[string]interface{}{
			"available_capacity": 100.0,
			"skills":            []string{"sales"},
		},
	}
	
	bid2 := &bid.Bid{
		ID:           uuid.New(),
		CallID:       testCall.ID,
		BuyerID:      sellerID2,
		Amount:       4.75,
		Status:       bid.StatusActive,
		QualityScore: 95.0,
		PlacedAt:     time.Now(),
		ExpiresAt:    time.Now().Add(5 * time.Minute),
		Criteria: map[string]interface{}{
			"available_capacity": 150.0,
			"skills":            []string{"sales", "support"},
		},
	}
	
	err = bidRepo.Create(ctx, bid1)
	require.NoError(t, err)
	err = bidRepo.Create(ctx, bid2)
	require.NoError(t, err)
	
	// Test different routing algorithms
	algorithms := []struct {
		name     string
		rules    *callrouting.RoutingRules
		validate func(t *testing.T, decision *callrouting.RoutingDecision)
	}{
		{
			name: "round-robin routing",
			rules: &callrouting.RoutingRules{
				Algorithm: "round-robin",
			},
			validate: func(t *testing.T, decision *callrouting.RoutingDecision) {
				assert.Equal(t, "round-robin", decision.Algorithm)
				assert.Contains(t, []uuid.UUID{sellerID1, sellerID2}, decision.BuyerID)
			},
		},
		{
			name: "skill-based routing",
			rules: &callrouting.RoutingRules{
				Algorithm: "skill-based",
			},
			validate: func(t *testing.T, decision *callrouting.RoutingDecision) {
				assert.Equal(t, "skill-based", decision.Algorithm)
				// Should prefer bid2 (has both sales and support skills)
				assert.Equal(t, sellerID2, decision.BuyerID)
				assert.Equal(t, bid2.ID, decision.BidID)
			},
		},
		{
			name: "cost-based routing",
			rules: &callrouting.RoutingRules{
				Algorithm:      "cost-based",
				QualityWeight:  0.4,
				PriceWeight:    0.4,
				CapacityWeight: 0.2,
			},
			validate: func(t *testing.T, decision *callrouting.RoutingDecision) {
				assert.Equal(t, "cost-based", decision.Algorithm)
				// Should prefer bid2 (higher quality score despite lower price)
				assert.Equal(t, sellerID2, decision.BuyerID)
				assert.Greater(t, decision.Score, 0.7)
			},
		},
	}
	
	for _, algo := range algorithms {
		t.Run(algo.name, func(t *testing.T) {
			// Reset call status
			testCall.Status = call.StatusPending
			err := callRepo.Update(ctx, testCall)
			require.NoError(t, err)
			
			// Create routing service
			svc := callrouting.NewService(callRepo, bidRepo, accountRepo, nil, algo.rules)
			
			// Route the call
			decision, err := svc.RouteCall(ctx, testCall.ID)
			require.NoError(t, err)
			require.NotNil(t, decision)
			
			// Validate decision
			algo.validate(t, decision)
			
			// Verify call was updated in database
			updatedCall, err := callRepo.GetByID(ctx, testCall.ID)
			require.NoError(t, err)
			assert.Equal(t, call.StatusQueued, updatedCall.Status)
			
			// Verify winning bid exists
			winningBid, err := bidRepo.GetByID(ctx, decision.BidID)
			require.NoError(t, err)
			assert.Equal(t, bid.StatusWon, winningBid.Status)
		})
	}
}

// TestCallLifecycle_Complete tests the complete call lifecycle
func TestCallLifecycle_Complete(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	ctx := testutil.TestContext(t)
	
	callRepo := repository.NewCallRepository(testDB.DB())
	bidRepo := repository.NewBidRepository(testDB.DB())
	
	// Create and save call
	testCall := call.NewCall("+15551234567", "+15559876543", uuid.New(), call.DirectionInbound)
	err := callRepo.Create(ctx, testCall)
	require.NoError(t, err)
	
	// Create bid
	testBid := &bid.Bid{
		ID:           uuid.New(),
		CallID:       testCall.ID,
		BuyerID:      uuid.New(),
		Amount:       5.00,
		Status:       bid.StatusActive,
		QualityScore: 85.0,
		PlacedAt:     time.Now(),
		ExpiresAt:    time.Now().Add(5 * time.Minute),
	}
	err = bidRepo.Create(ctx, testBid)
	require.NoError(t, err)
	
	// Test complete lifecycle: Pending -> Queued -> Ringing -> InProgress -> Completed
	statuses := []call.Status{
		call.StatusQueued,
		call.StatusRinging,
		call.StatusInProgress,
	}
	
	for _, status := range statuses {
		testCall.UpdateStatus(status)
		err = callRepo.Update(ctx, testCall)
		require.NoError(t, err)
		
		// Verify status in database
		retrieved, err := callRepo.GetByID(ctx, testCall.ID)
		require.NoError(t, err)
		assert.Equal(t, status, retrieved.Status)
	}
	
	// Complete the call
	duration := 300 // 5 minutes
	cost := 15.50
	testCall.Complete(duration, cost)
	err = callRepo.Update(ctx, testCall)
	require.NoError(t, err)
	
	// Verify completion in database
	completed, err := callRepo.GetByID(ctx, testCall.ID)
	require.NoError(t, err)
	assert.Equal(t, call.StatusCompleted, completed.Status)
	assert.NotNil(t, completed.EndTime)
	assert.NotNil(t, completed.Duration)
	assert.NotNil(t, completed.Cost)
	assert.Equal(t, duration, *completed.Duration)
	assert.Equal(t, cost, *completed.Cost)
}

// TestConcurrentBidding tests concurrent bid creation and routing
func TestConcurrentBidding(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	ctx := testutil.TestContext(t)
	
	callRepo := repository.NewCallRepository(testDB.DB())
	bidRepo := repository.NewBidRepository(testDB.DB())
	accountRepo := repository.NewAccountRepository(testDB.DB())
	
	// Create test call
	testCall := call.NewCall("+15551234567", "+15559876543", uuid.New(), call.DirectionInbound)
	err := callRepo.Create(ctx, testCall)
	require.NoError(t, err)
	
	// Create multiple bidders concurrently
	numBidders := 10
	bidders := make([]uuid.UUID, numBidders)
	bids := make([]*bid.Bid, numBidders)
	
	// Create bids concurrently
	bidChannel := make(chan *bid.Bid, numBidders)
	errorChannel := make(chan error, numBidders)
	
	for i := 0; i < numBidders; i++ {
		go func(index int) {
			bidderID := uuid.New()
			bidders[index] = bidderID
			
			newBid := &bid.Bid{
				ID:           uuid.New(),
				CallID:       testCall.ID,
				BuyerID:      bidderID,
				Amount:       float64(index) + 1.0, // Different bid amounts
				Status:       bid.StatusActive,
				QualityScore: 70.0 + float64(index), // Different quality scores
				PlacedAt:     time.Now(),
				ExpiresAt:    time.Now().Add(5 * time.Minute),
				Criteria: map[string]interface{}{
					"available_capacity": float64(100 + index*10),
				},
			}
			
			err := bidRepo.Create(ctx, newBid)
			if err != nil {
				errorChannel <- err
				return
			}
			
			bidChannel <- newBid
		}(i)
	}
	
	// Collect results
	for i := 0; i < numBidders; i++ {
		select {
		case bid := <-bidChannel:
			bids[i] = bid
		case err := <-errorChannel:
			require.NoError(t, err)
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for bid creation")
		}
	}
	
	// Verify all bids were created
	allBids, err := bidRepo.GetActiveBidsForCall(ctx, testCall.ID)
	require.NoError(t, err)
	assert.Len(t, allBids, numBidders)
	
	// Test routing with multiple concurrent requests
	svc := callrouting.NewService(callRepo, bidRepo, accountRepo, nil, &callrouting.RoutingRules{
		Algorithm:      "cost-based",
		QualityWeight:  0.6,
		PriceWeight:    0.4,
	})
	
	// Only one routing should succeed due to call state
	decisions := make(chan *callrouting.RoutingDecision, 3)
	errors := make(chan error, 3)
	
	for i := 0; i < 3; i++ {
		go func() {
			decision, err := svc.RouteCall(ctx, testCall.ID)
			if err != nil {
				errors <- err
				return
			}
			decisions <- decision
		}()
	}
	
	// Collect results - only one should succeed
	successCount := 0
	errorCount := 0
	
	for i := 0; i < 3; i++ {
		select {
		case decision := <-decisions:
			require.NotNil(t, decision)
			successCount++
		case err := <-errors:
			assert.Contains(t, err.Error(), "not in pending state")
			errorCount++
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for routing results")
		}
	}
	
	// Exactly one should succeed, others should fail
	assert.Equal(t, 1, successCount)
	assert.Equal(t, 2, errorCount)
}

// TestDatabaseConstraints tests database constraints and transactions
func TestDatabaseConstraints(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	ctx := testutil.TestContext(t)
	
	callRepo := repository.NewCallRepository(testDB.DB())
	bidRepo := repository.NewBidRepository(testDB.DB())
	
	t.Run("foreign key constraints", func(t *testing.T) {
		// Try to create bid for non-existent call
		invalidBid := &bid.Bid{
			ID:           uuid.New(),
			CallID:       uuid.New(), // Non-existent call
			BuyerID:      uuid.New(),
			Amount:       5.00,
			Status:       bid.StatusActive,
			QualityScore: 85.0,
			PlacedAt:     time.Now(),
			ExpiresAt:    time.Now().Add(5 * time.Minute),
		}
		
		err := bidRepo.Create(ctx, invalidBid)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "foreign key")
	})
	
	t.Run("unique constraints", func(t *testing.T) {
		// Create call
		testCall := call.NewCall("+15551234567", "+15559876543", uuid.New(), call.DirectionInbound)
		err := callRepo.Create(ctx, testCall)
		require.NoError(t, err)
		
		// Try to create call with same ID
		duplicateCall := *testCall
		err = callRepo.Create(ctx, &duplicateCall)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "duplicate")
	})
	
	t.Run("transaction rollback", func(t *testing.T) {
		// Use transaction wrapper
		err := testDB.WithTx(ctx, func(tx *sql.Tx) error {
			// Create call in transaction
			testCall := call.NewCall("+15551234567", "+15559876543", uuid.New(), call.DirectionInbound)
			err := callRepo.Create(ctx, testCall)
			if err != nil {
				return err
			}
			
			// Intentionally cause error to trigger rollback
			return errors.New("intentional error")
		})
		
		require.Error(t, err)
		assert.Contains(t, err.Error(), "intentional error")
		
		// Verify call was not persisted
		testDB.AssertRowCount("calls", 0)
	})
}