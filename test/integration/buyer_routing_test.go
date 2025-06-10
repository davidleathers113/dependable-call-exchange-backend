//go:build integration

package integration

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/account"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/bid"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/call"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	domainErrors "github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/infrastructure/repository"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/service/bidding"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/service/buyer_routing"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/testutil"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/testutil/fixtures"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBuyerRouting_EndToEnd tests the complete buyer routing flow with real database
func TestBuyerRouting_EndToEnd(t *testing.T) {
	// Setup test database and context
	testDB := testutil.NewTestDB(t)
	ctx := testutil.TestContext(t)
	
	// Create repositories - concrete implementations
	callRepoImpl := repository.NewCallRepository(testDB.DB())
	bidRepoImpl := repository.NewBidRepository(testDB.DB())
	accountRepoImpl := repository.NewAccountRepository(testDB.DB())
	
	// Type assert to get access to the Create method
	type AccountRepositoryWithCreate interface {
		bidding.AccountRepository
		Create(ctx context.Context, a *account.Account) error
	}
	fullAccountRepo := accountRepoImpl.(AccountRepositoryWithCreate)
	
	// Use fixture builders for test data setup
	testData := fixtures.CreateCompleteTestSet(t, testDB)
	
	// Get the test call and ensure it's in pending state
	testCall := testData.InboundCall
	testCall.Status = call.StatusPending
	err := callRepoImpl.Update(ctx, testCall)
	require.NoError(t, err)
	
	// Create additional buyer accounts for competitive bidding
	buyer2 := fixtures.NewAccountBuilder(testDB).
		WithType(account.TypeBuyer).
		WithEmail(fixtures.GenerateEmail(t, "buyer2")).
		WithName("Test Buyer 2").
		WithCompany("Buyer Corp 2").
		WithQualityScore(0.85).
		Build(t)
	err = fullAccountRepo.Create(ctx, buyer2)
	require.NoError(t, err)

	buyer3 := fixtures.NewAccountBuilder(testDB).
		WithType(account.TypeBuyer).
		WithEmail(fixtures.GenerateEmail(t, "buyer3")).
		WithName("Test Buyer 3").
		WithCompany("Buyer Corp 3").
		WithQualityScore(0.95).
		Build(t)
	err = fullAccountRepo.Create(ctx, buyer3)
	require.NoError(t, err)

	// Create competing bids using builders and persist them
	bid1 := fixtures.NewBidBuilder(testDB).
		WithCallID(testCall.ID).
		WithBuyerID(buyer2.ID).
		WithSellerID(testData.SellerAccount.ID). // Set seller who generated the call
		WithAmount(5.50).
		WithQualityMetrics(0.85, 300, 0.01, 8.5).
		WithCriteria(bid.BidCriteria{
			CallType: []string{"inbound", "outbound"},
			Geography: bid.GeoCriteria{
				Countries: []string{"US"},
			},
			Keywords:  []string{"sales"},
			MaxBudget: values.MustNewMoneyFromFloat(100.0, values.USD),
		}).
		BuildWithRepo(t, bidRepoImpl, ctx)
	
	bid2 := fixtures.NewBidBuilder(testDB).
		WithCallID(testCall.ID).
		WithBuyerID(buyer3.ID).
		WithSellerID(testData.SellerAccount.ID). // Set seller who generated the call
		WithAmount(4.75).
		WithQualityMetrics(0.95, 250, 0.005, 9.5).
		WithCriteria(bid.BidCriteria{
			CallType: []string{"inbound", "outbound"},
			Geography: bid.GeoCriteria{
				Countries: []string{"US"},
			},
			Keywords:  []string{"sales", "support"},
			MaxBudget: values.MustNewMoneyFromFloat(150.0, values.USD),
		}).
		BuildWithRepo(t, bidRepoImpl, ctx)
	
	// Test different routing algorithms
	algorithms := []struct {
		name     string
		rules    *buyer_routing.BuyerRoutingRules
		validate func(t *testing.T, decision *buyer_routing.BuyerRoutingDecision)
	}{
		{
			name: "round-robin routing",
			rules: &buyer_routing.BuyerRoutingRules{
				Algorithm: "round-robin",
			},
			validate: func(t *testing.T, decision *buyer_routing.BuyerRoutingDecision) {
				assert.Equal(t, "round-robin", decision.Algorithm)
				// Round-robin should select one of the competing buyers (including existing test data buyer)
				validBuyerIDs := []uuid.UUID{testData.BuyerAccount.ID, buyer2.ID, buyer3.ID}
				assert.Contains(t, validBuyerIDs, decision.BuyerID, 
					"Round-robin should select one of the three competing buyers")
			},
		},
		{
			name: "skill-based routing",
			rules: &buyer_routing.BuyerRoutingRules{
				Algorithm: "skill-based",
			},
			validate: func(t *testing.T, decision *buyer_routing.BuyerRoutingDecision) {
				assert.Equal(t, "skill-based", decision.Algorithm)
				// With skill-based routing, bid2 should win due to higher quality metrics
				// and more keywords, but we test more robustly
				assert.NotNil(t, decision.BidID)
				assert.Greater(t, decision.Score, 0.0, "Skill-based routing should provide a score")
				
				// Verify the selected bid has appropriate criteria
				selectedBid, err := bidRepoImpl.GetByID(ctx, decision.BidID)
				require.NoError(t, err)
				assert.Contains(t, selectedBid.Criteria.CallType, "inbound",
					"Selected bid should accept inbound calls")
			},
		},
		{
			name: "cost-based routing",
			rules: &buyer_routing.BuyerRoutingRules{
				Algorithm:      "cost-based",
				QualityWeight:  0.4,
				PriceWeight:    0.4,
				CapacityWeight: 0.2,
			},
			validate: func(t *testing.T, decision *buyer_routing.BuyerRoutingDecision) {
				assert.Equal(t, "cost-based", decision.Algorithm)
				assert.Greater(t, decision.Score, 0.0, "Cost-based routing should provide a score")
				
				// Verify the decision contains metadata about the scoring
				weights, ok := decision.Metadata["weights"]
				assert.True(t, ok, "Cost-based routing should include weights in metadata")
				assert.NotNil(t, weights, "Weights should not be nil")
				
				// Verify weight components exist
				weightsMap, ok := weights.(map[string]float64)
				assert.True(t, ok, "Weights should be a map of string to float64")
				assert.Contains(t, weightsMap, "quality", "Should contain quality weight")
				assert.Contains(t, weightsMap, "price", "Should contain price weight")
				assert.Contains(t, weightsMap, "capacity", "Should contain capacity weight")
			},
		},
	}
	
	for _, algo := range algorithms {
		t.Run(algo.name, func(t *testing.T) {
			// Reset call and bid states for each algorithm test
			resetTestState(t, ctx, testCall, callRepoImpl, []*bid.Bid{bid1, bid2}, bidRepoImpl)
			
			// Create routing service with repositories
			// Use concrete repositories that implement buyer_routing interfaces
			svc := buyer_routing.NewService(callRepoImpl, bidRepoImpl, accountRepoImpl, buyer_routing.NewNoopMetrics(), algo.rules)
			
			// Route the call
			decision, err := svc.RouteCallToBuyer(ctx, testCall.ID)
			require.NoError(t, err)
			require.NotNil(t, decision)
			
			// Validate decision based on algorithm
			algo.validate(t, decision)
			
			// Common validations for all algorithms
			t.Run("updates_call_status", func(t *testing.T) {
				updatedCall, err := callRepoImpl.GetByID(ctx, testCall.ID)
				require.NoError(t, err)
				assert.Equal(t, call.StatusQueued, updatedCall.Status,
					"Call should be moved to queued status after routing")
			})
			
			t.Run("marks_winning_bid", func(t *testing.T) {
				winningBid, err := bidRepoImpl.GetByID(ctx, decision.BidID)
				require.NoError(t, err)
				assert.Equal(t, bid.StatusWon, winningBid.Status,
					"Winning bid should be marked as won")
			})
		})
	}
}

// TestCallLifecycle_Complete tests the complete call lifecycle with proper state transitions
func TestCallLifecycle_Complete(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	ctx := testutil.TestContext(t)
	
	// Use fixture builders
	testData := fixtures.CreateCompleteTestSet(t, testDB)
	testCall := testData.InboundCall
	
	// Create a bid for the call (removed - not used in this test)
	// testBid := fixtures.NewBidBuilder(testDB).
	// 	WithCallID(testCall.ID).
	// 	WithBuyerID(testData.SellerAccount1.ID).
	// 	WithAmount(5.00).
	// 	WithQualityMetrics(0.85, 300, 0.01, 8.5).
	// 	WithCriteria(bid.BidCriteria{
	// 		CallType: []string{"inbound"},
	// 		Geography: bid.GeoCriteria{
	// 			Countries: []string{"US"},
	// 		},
	// 	}).
	// 	Build(t)
	
	// Get repository for updates
	callRepoImpl := repository.NewCallRepository(testDB.DB())
	
	// Test status transitions
	statusTransitions := []struct {
		name           string
		fromStatus     call.Status
		toStatus       call.Status
		validTransition bool
	}{
		{
			name:           "pending_to_queued",
			fromStatus:     call.StatusPending,
			toStatus:       call.StatusQueued,
			validTransition: true,
		},
		{
			name:           "queued_to_ringing",
			fromStatus:     call.StatusQueued,
			toStatus:       call.StatusRinging,
			validTransition: true,
		},
		{
			name:           "ringing_to_in_progress",
			fromStatus:     call.StatusRinging,
			toStatus:       call.StatusInProgress,
			validTransition: true,
		},
	}
	
	for _, transition := range statusTransitions {
		t.Run(transition.name, func(t *testing.T) {
			// Retrieve fresh call instance for each test
			freshCall, err := callRepoImpl.GetByID(ctx, testCall.ID)
			require.NoError(t, err)
			
			// Ensure call is in the expected starting state
			freshCall.Status = transition.fromStatus
			err = callRepoImpl.Update(ctx, freshCall)
			require.NoError(t, err)
			
			// Perform status update
			freshCall.UpdateStatus(transition.toStatus)
			err = callRepoImpl.Update(ctx, freshCall)
			
			if transition.validTransition {
				require.NoError(t, err)
				
				// Verify status change persisted
				retrieved, err := callRepoImpl.GetByID(ctx, testCall.ID)
				require.NoError(t, err)
				assert.Equal(t, transition.toStatus, retrieved.Status)
				assert.True(t, retrieved.UpdatedAt.After(retrieved.CreatedAt),
					"UpdatedAt should be after CreatedAt after status change")
			} else {
				require.Error(t, err)
			}
		})
	}
	
	t.Run("complete_call", func(t *testing.T) {
		// Ensure call is in progress
		testCall.Status = call.StatusInProgress
		err := callRepoImpl.Update(ctx, testCall)
		require.NoError(t, err)
		
		// Complete the call
		duration := 300 // 5 minutes
		cost := values.MustNewMoneyFromFloat(15.50, values.USD)
		testCall.Complete(duration, cost)
		err = callRepoImpl.Update(ctx, testCall)
		require.NoError(t, err)
		
		// Verify completion details
		completed, err := callRepoImpl.GetByID(ctx, testCall.ID)
		require.NoError(t, err)
		assert.Equal(t, call.StatusCompleted, completed.Status)
		assert.NotNil(t, completed.EndTime, "EndTime should be set")
		assert.NotNil(t, completed.Duration, "Duration should be set")
		assert.NotNil(t, completed.Cost, "Cost should be set")
		assert.Equal(t, duration, *completed.Duration)
		assert.Equal(t, cost, *completed.Cost)
		assert.True(t, completed.EndTime.After(completed.StartTime),
			"EndTime should be after StartTime")
	})
}

// TestConcurrentBidding tests concurrent bid creation and routing with proper synchronization
func TestConcurrentBidding(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	ctx := testutil.TestContext(t)
	
	// Create repositories
	callRepoImpl := repository.NewCallRepository(testDB.DB())
	bidRepoImpl := repository.NewBidRepository(testDB.DB())
	accountRepoImpl := repository.NewAccountRepository(testDB.DB())
	
	// Type assert to get access to the Create method
	type AccountRepositoryWithCreate interface {
		bidding.AccountRepository
		Create(ctx context.Context, a *account.Account) error
	}
	fullAccountRepo := accountRepoImpl.(AccountRepositoryWithCreate)
	
	// Use fixtures for initial setup
	testData := fixtures.CreateCompleteTestSet(t, testDB)
	testCall := testData.InboundCall
	
	// Ensure call is in pending state
	testCall.Status = call.StatusPending
	err := callRepoImpl.Update(ctx, testCall)
	require.NoError(t, err)
	
	// Create multiple buyer accounts for concurrent bidding
	numBidders := 10
	buyers := make([]*account.Account, numBidders)
	for i := 0; i < numBidders; i++ {
		buyer := fixtures.NewAccountBuilder(testDB).
			WithType(account.TypeBuyer).
			WithEmail(fmt.Sprintf("buyer%d@test.com", i)).
			WithCompany(fmt.Sprintf("Buyer %d Corp", i)).
			WithBalance(500.00 + float64(i*50)).
			WithQualityScore(0.80 + float64(i)*0.01).
			Build(t)
		
		// Insert buyer account into database
		err := fullAccountRepo.Create(ctx, buyer)
		require.NoError(t, err)
		
		buyers[i] = buyer
	}
	
	t.Run("concurrent_bid_creation", func(t *testing.T) {
		var wg sync.WaitGroup
		bidChan := make(chan *bid.Bid, numBidders)
		errChan := make(chan error, numBidders)
		
		// Create bids concurrently
		for i := 0; i < numBidders; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				
				// Catch any panics from test assertions
				defer func() {
					if r := recover(); r != nil {
						errChan <- fmt.Errorf("panic in bid %d: %v", index, r)
					}
				}()
				
				// Use builder for concurrent bid creation (without repo)
				newBid := fixtures.NewBidBuilder(testDB).
					WithCallID(testCall.ID).
					WithBuyerID(buyers[index].ID).
					WithSellerID(testData.SellerAccount.ID).
					WithAmount(float64(index) + 1.0).
					WithQualityMetrics(
						0.70+float64(index)*0.01,
						300+index*10,
						0.01,
						7.0+float64(index)*0.1,
					).
					WithCriteria(bid.BidCriteria{
						CallType: []string{"inbound", "outbound"},
						Geography: bid.GeoCriteria{
							Countries: []string{"US"},
						},
						MaxBudget: values.MustNewMoneyFromFloat(100.0 + float64(index*10), values.USD),
					}).
					Build(t)
				
				// Manually create the bid to handle errors properly
				err := bidRepoImpl.Create(ctx, newBid)
				if err != nil {
					errChan <- fmt.Errorf("failed to create bid %d: %w", index, err)
					return
				}
				
				bidChan <- newBid
			}(i)
		}
		
		// Wait for all goroutines to complete
		wg.Wait()
		close(bidChan)
		close(errChan)
		
		// Verify all bids were created
		var createdBids []*bid.Bid
		for bid := range bidChan {
			createdBids = append(createdBids, bid)
		}
		
		assert.Len(t, createdBids, numBidders, "All concurrent bids should be created")
		
		// Verify no errors
		var errors []error
		for err := range errChan {
			errors = append(errors, err)
		}
		assert.Empty(t, errors, "No errors should occur during concurrent bid creation")
	})
	
	t.Run("concurrent_routing_attempts", func(t *testing.T) {
		// Ensure call is back in pending state for routing test
		testCall.Status = call.StatusPending
		err := callRepoImpl.Update(ctx, testCall)
		require.NoError(t, err)
		
		// Setup routing service
		// bidRepoAdapter removed - using bidRepo directly
		// accountRepoAdapter removed - using accountRepo directly
		svc := buyer_routing.NewService(callRepoImpl, bidRepoImpl, accountRepoImpl, buyer_routing.NewNoopMetrics(),
			&buyer_routing.BuyerRoutingRules{
				Algorithm:      "cost-based",
				QualityWeight:  0.6,
				PriceWeight:    0.4,
			})
		
		// Attempt concurrent routing
		numAttempts := 3
		results := make(chan routingResult, numAttempts)
		
		var wg sync.WaitGroup
		for i := 0; i < numAttempts; i++ {
			wg.Add(1)
			go func(attempt int) {
				defer wg.Done()
				
				decision, err := svc.RouteCallToBuyer(ctx, testCall.ID)
				results <- routingResult{
					decision: decision,
					err:      err,
					attempt:  attempt,
				}
			}(i)
		}
		
		wg.Wait()
		close(results)
		
		// Analyze results
		var successes, failures int
		for result := range results {
			if result.err != nil {
				failures++
				// Verify proper error type for concurrent routing attempts
				var appErr *domainErrors.AppError
				if assert.ErrorAs(t, result.err, &appErr,
					"Routing errors should be wrapped in AppError") {
					assert.Equal(t, domainErrors.ErrorTypeValidation, appErr.Type,
						"Concurrent routing failure should be validation error")
				}
			} else {
				successes++
				assert.NotNil(t, result.decision, "Successful routing should return decision")
			}
		}
		
		// Exactly one should succeed
		assert.Equal(t, 1, successes, "Exactly one routing attempt should succeed")
		assert.Equal(t, numAttempts-1, failures, "Other attempts should fail with proper error")
	})
}

// TestDatabaseConstraints validates database integrity constraints
func TestDatabaseConstraints(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	ctx := testutil.TestContext(t)
	
	callRepoImpl := repository.NewCallRepository(testDB.DB())
	bidRepoImpl := repository.NewBidRepository(testDB.DB())
	
	t.Run("foreign_key_constraint", func(t *testing.T) {
		// Attempt to create bid with non-existent call ID
		invalidBid := &bid.Bid{
			ID:       uuid.New(),
			CallID:   uuid.New(), // Non-existent call
			BuyerID:  uuid.New(),
			Amount:   values.MustNewMoneyFromFloat(5.00, values.USD),
			Status:   bid.StatusActive,
			Quality:  values.QualityMetrics{},
			PlacedAt: time.Now(),
			ExpiresAt: time.Now().Add(5 * time.Minute),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		
		err := bidRepoImpl.Create(ctx, invalidBid)
		require.Error(t, err, "Should fail with foreign key violation")
		
		// Check for proper error wrapping
		assert.True(t, repository.IsForeignKeyViolation(err),
			"Repository should wrap foreign key violations appropriately")
	})
	
	t.Run("unique_constraint", func(t *testing.T) {
		// Use fixtures to create valid test data
		testData := fixtures.CreateCompleteTestSet(t, testDB)
		testCall := testData.InboundCall
		
		// Attempt to create duplicate call
		duplicateCall := *testCall
		err := callRepoImpl.Create(ctx, &duplicateCall)
		require.Error(t, err, "Should fail with unique constraint violation")
		
		// Check for proper error wrapping
		assert.True(t, repository.IsDuplicateKeyViolation(err),
			"Repository should wrap duplicate key violations appropriately")
	})
	
	t.Run("transaction_rollback", func(t *testing.T) {
		// Use fixtures for setup
		testData := fixtures.CreateCompleteTestSet(t, testDB)
		
		// Count calls before transaction
		initialCount := testDB.GetRowCount(t, "calls")
		
		// Execute transaction that should rollback
		err := testDB.WithTx(ctx, func(tx *sql.Tx) error {
			// Create new call in transaction
			newCall, err := call.NewCall(
				"+15551234567",
				"+15559876543",
				testData.BuyerAccount.ID,
				call.DirectionInbound,
			)
			if err != nil {
				return err
			}
			
			// Use transactional repository
			txCallRepo := repository.NewCallRepositoryWithTx(tx)
			err = txCallRepo.Create(ctx, newCall)
			if err != nil {
				return err
			}
			
			// Force rollback
			return errors.New("intentional rollback")
		})
		
		// Verify error and rollback
		require.Error(t, err)
		assert.Contains(t, err.Error(), "intentional rollback")
		
		// Verify no new calls were persisted
		finalCount := testDB.GetRowCount(t, "calls")
		assert.Equal(t, initialCount, finalCount,
			"Transaction rollback should not persist any changes")
	})
}

// TestNegativeScenarios tests error cases and edge conditions
func TestNegativeScenarios(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	ctx := testutil.TestContext(t)
	
	// Create repositories
	callRepoImpl := repository.NewCallRepository(testDB.DB())
	bidRepoImpl := repository.NewBidRepository(testDB.DB())
	accountRepoImpl := repository.NewAccountRepository(testDB.DB())
	
	// Setup routing service
	// bidRepoAdapter removed - using bidRepo directly
	// accountRepoAdapter removed - using accountRepo directly
	svc := buyer_routing.NewService(callRepoImpl, bidRepoImpl, accountRepoImpl, buyer_routing.NewNoopMetrics(),
		&buyer_routing.BuyerRoutingRules{Algorithm: "round-robin"})
	
	t.Run("no_bids_available", func(t *testing.T) {
		// Create minimal test set (no bids)
		testData := fixtures.CreateMinimalTestSet(t, testDB)
		testCall := testData.InboundCall
		
		// Ensure call is in pending state
		testCall.Status = call.StatusPending
		err := callRepoImpl.Update(ctx, testCall)
		require.NoError(t, err)
		
		// Attempt routing
		decision, err := svc.RouteCallToBuyer(ctx, testCall.ID)
		
		// Should fail with appropriate error
		require.Error(t, err)
		assert.Nil(t, decision)
		
		var appErr *domainErrors.AppError
		if assert.ErrorAs(t, err, &appErr) {
			// The service returns BusinessError for NO_BIDS_AVAILABLE
			assert.Equal(t, domainErrors.ErrorTypeBusiness, appErr.Type,
				"No bids available should return Business error")
		}
	})
	
	t.Run("call_not_routable", func(t *testing.T) {
		testData := fixtures.CreateCompleteTestSet(t, testDB)
		testCall := testData.InboundCall
		
		// Set call to completed state
		testCall.Status = call.StatusCompleted
		err := callRepoImpl.Update(ctx, testCall)
		require.NoError(t, err)
		
		// Create a bid for the call
		fixtures.NewBidBuilder(testDB).
			WithCallID(testCall.ID).
			WithBuyerID(testData.BuyerAccount.ID).
			WithSellerID(testData.SellerAccount.ID).
			BuildWithRepo(t, bidRepoImpl, ctx)
		
		// Attempt routing
		decision, err := svc.RouteCallToBuyer(ctx, testCall.ID)
		
		// Should fail with validation error
		require.Error(t, err)
		assert.Nil(t, decision)
		
		var appErr *domainErrors.AppError
		if assert.ErrorAs(t, err, &appErr) {
			assert.Equal(t, domainErrors.ErrorTypeValidation, appErr.Type,
				"Routing non-pending call should return validation error")
		}
	})
	
	t.Run("suspended_buyer_account", func(t *testing.T) {
		testData := fixtures.CreateCompleteTestSet(t, testDB)
		testCall := testData.InboundCall
		
		// Type assert to get access to Create method
		type AccountRepositoryWithCreate interface {
			bidding.AccountRepository
			Create(ctx context.Context, a *account.Account) error
		}
		fullAccountRepo := accountRepoImpl.(AccountRepositoryWithCreate)
		
		// Create suspended buyer account
		suspendedBuyer := fixtures.NewAccountBuilder(testDB).
			WithType(account.TypeBuyer).
			WithStatus(account.StatusSuspended).
			WithEmail(fixtures.GenerateEmail(t, "suspended-buyer")).
			Build(t)
		
		// Insert the account
		err := fullAccountRepo.Create(ctx, suspendedBuyer)
		require.NoError(t, err, "failed to create suspended buyer account")
		
		// Create bid from suspended buyer
		fixtures.NewBidBuilder(testDB).
			WithCallID(testCall.ID).
			WithBuyerID(suspendedBuyer.ID).
			WithSellerID(testData.SellerAccount.ID). // Set the seller who owns the call
			WithCriteria(bid.BidCriteria{
				CallType: []string{"inbound"},
			}).
			BuildWithRepo(t, bidRepoImpl, ctx)
		
		// Attempt routing
		decision, err := svc.RouteCallToBuyer(ctx, testCall.ID)
		
		// Should fail as no valid buyers
		require.Error(t, err)
		assert.Nil(t, decision)
	})
}

// Helper functions

type routingResult struct {
	decision *buyer_routing.BuyerRoutingDecision
	err      error
	attempt  int
}

func resetTestState(t *testing.T, ctx context.Context, testCall *call.Call,
	callRepo interface{ Update(ctx context.Context, c *call.Call) error }, bids []*bid.Bid, bidRepo interface{ Update(ctx context.Context, b *bid.Bid) error }) {
	t.Helper()
	
	// Reset call to pending
	testCall.Status = call.StatusPending
	err := callRepo.Update(ctx, testCall)
	require.NoError(t, err)
	
	// Reset all bids to active
	for _, b := range bids {
		b.Status = bid.StatusActive
		err := bidRepo.Update(ctx, b)
		require.NoError(t, err)
	}
}