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
	domainErrors "github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/infrastructure/repository"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/service/callrouting"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/testutil"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/testutil/fixtures"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCallRouting_EndToEnd tests the complete call routing flow with real database
func TestCallRouting_EndToEnd(t *testing.T) {
	// Setup test database and context
	testDB := testutil.NewTestDB(t)
	ctx := testutil.TestContext(t)
	
	// Create repositories
	callRepo := repository.NewCallRepository(testDB.DB())
	bidRepo := repository.NewBidRepository(testDB.DB())
	accountRepo := repository.NewAccountRepository(testDB.DB())
	
	// Use fixture builders for test data setup
	testData := fixtures.CreateCompleteTestSet(t, testDB)
	
	// Get the test call and ensure it's in pending state
	testCall := testData.InboundCall
	testCall.Status = call.StatusPending
	err := callRepo.Update(ctx, testCall)
	require.NoError(t, err)
	
	// Create competing bids using builders
	bid1 := fixtures.NewBidBuilder(testDB).
		WithCallID(testCall.ID).
		WithBuyerID(testData.SellerAccount1.ID).
		WithAmount(5.50).
		WithQualityMetrics(0.85, 300, 0.01, 8.5).
		WithCriteria(bid.BidCriteria{
			CallType: []string{"inbound", "outbound"},
			Geography: bid.GeoCriteria{
				Countries: []string{"US"},
			},
			Keywords:  []string{"sales"},
			MaxBudget: 100.0,
		}).
		Build(t)
	
	bid2 := fixtures.NewBidBuilder(testDB).
		WithCallID(testCall.ID).
		WithBuyerID(testData.SellerAccount2.ID).
		WithAmount(4.75).
		WithQualityMetrics(0.95, 250, 0.005, 9.5).
		WithCriteria(bid.BidCriteria{
			CallType: []string{"inbound", "outbound"},
			Geography: bid.GeoCriteria{
				Countries: []string{"US"},
			},
			Keywords:  []string{"sales", "support"},
			MaxBudget: 150.0,
		}).
		Build(t)
	
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
				// Round-robin should select one of the available sellers
				validBuyerIDs := []uuid.UUID{testData.SellerAccount1.ID, testData.SellerAccount2.ID}
				assert.Contains(t, validBuyerIDs, decision.BuyerID, 
					"Round-robin should select one of the two sellers")
			},
		},
		{
			name: "skill-based routing",
			rules: &callrouting.RoutingRules{
				Algorithm: "skill-based",
			},
			validate: func(t *testing.T, decision *callrouting.RoutingDecision) {
				assert.Equal(t, "skill-based", decision.Algorithm)
				// With skill-based routing, bid2 should win due to higher quality metrics
				// and more keywords, but we test more robustly
				assert.NotNil(t, decision.BidID)
				assert.Greater(t, decision.Score, 0.0, "Skill-based routing should provide a score")
				
				// Verify the selected bid has appropriate criteria
				selectedBid, err := bidRepo.GetByID(ctx, decision.BidID)
				require.NoError(t, err)
				assert.Contains(t, selectedBid.Criteria.CallType, "inbound",
					"Selected bid should accept inbound calls")
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
				assert.Greater(t, decision.Score, 0.0, "Cost-based routing should provide a score")
				
				// Verify the decision contains metadata about the scoring
				metadata, ok := decision.Metadata["weight_breakdown"]
				assert.True(t, ok, "Cost-based routing should include weight breakdown in metadata")
				_ = metadata // Avoid unused variable warning
			},
		},
	}
	
	for _, algo := range algorithms {
		t.Run(algo.name, func(t *testing.T) {
			// Reset call and bid states for each algorithm test
			resetTestState(t, ctx, testCall, callRepo, []*bid.Bid{bid1, bid2}, bidRepo)
			
			// Create routing service with repositories
			bidRepoAdapter := repository.NewCallRoutingBidRepository(bidRepo)
			accountRepoAdapter := repository.NewCallRoutingAccountRepository(accountRepo)
			svc := callrouting.NewService(callRepo, bidRepoAdapter, accountRepoAdapter, nil, algo.rules)
			
			// Route the call
			decision, err := svc.RouteCall(ctx, testCall.ID)
			require.NoError(t, err)
			require.NotNil(t, decision)
			
			// Validate decision based on algorithm
			algo.validate(t, decision)
			
			// Common validations for all algorithms
			t.Run("updates_call_status", func(t *testing.T) {
				updatedCall, err := callRepo.GetByID(ctx, testCall.ID)
				require.NoError(t, err)
				assert.Equal(t, call.StatusQueued, updatedCall.Status,
					"Call should be moved to queued status after routing")
			})
			
			t.Run("marks_winning_bid", func(t *testing.T) {
				winningBid, err := bidRepo.GetByID(ctx, decision.BidID)
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
	
	// Create a bid for the call
	testBid := fixtures.NewBidBuilder(testDB).
		WithCallID(testCall.ID).
		WithBuyerID(testData.SellerAccount1.ID).
		WithAmount(5.00).
		WithQualityMetrics(0.85, 300, 0.01, 8.5).
		WithCriteria(bid.BidCriteria{
			CallType: []string{"inbound"},
			Geography: bid.GeoCriteria{
				Countries: []string{"US"},
			},
		}).
		Build(t)
	
	// Get repository for updates
	callRepo := repository.NewCallRepository(testDB.DB())
	
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
			// Ensure call is in the expected starting state
			testCall.Status = transition.fromStatus
			err := callRepo.Update(ctx, testCall)
			require.NoError(t, err)
			
			// Perform status update
			testCall.UpdateStatus(transition.toStatus)
			err = callRepo.Update(ctx, testCall)
			
			if transition.validTransition {
				require.NoError(t, err)
				
				// Verify status change persisted
				retrieved, err := callRepo.GetByID(ctx, testCall.ID)
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
		err := callRepo.Update(ctx, testCall)
		require.NoError(t, err)
		
		// Complete the call
		duration := 300 // 5 minutes
		cost := 15.50
		testCall.Complete(duration, cost)
		err = callRepo.Update(ctx, testCall)
		require.NoError(t, err)
		
		// Verify completion details
		completed, err := callRepo.GetByID(ctx, testCall.ID)
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
	callRepo := repository.NewCallRepository(testDB.DB())
	bidRepo := repository.NewBidRepository(testDB.DB())
	accountRepo := repository.NewAccountRepository(testDB.DB())
	
	// Use fixtures for initial setup
	testData := fixtures.CreateCompleteTestSet(t, testDB)
	testCall := testData.InboundCall
	
	// Ensure call is in pending state
	testCall.Status = call.StatusPending
	err := callRepo.Update(ctx, testCall)
	require.NoError(t, err)
	
	// Create multiple seller accounts for concurrent bidding
	numBidders := 10
	sellers := make([]*account.Account, numBidders)
	for i := 0; i < numBidders; i++ {
		seller := fixtures.NewAccountBuilder(testDB).
			WithType(account.AccountTypeSeller).
			WithEmail(fmt.Sprintf("seller%d@test.com", i)).
			WithCompany(fmt.Sprintf("Seller %d Inc", i)).
			WithBalance(500.00 + float64(i*50)).
			WithQualityScore(0.80 + float64(i)*0.01).
			Build(t)
		sellers[i] = seller
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
				
				// Use builder for concurrent bid creation
				newBid := fixtures.NewBidBuilder(testDB).
					WithCallID(testCall.ID).
					WithBuyerID(sellers[index].ID).
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
						MaxBudget: 100.0 + float64(index*10),
					}).
					BuildWithRepo(t, bidRepo, ctx)
				
				select {
				case bidChan <- newBid:
				case errChan <- fmt.Errorf("failed to create bid %d", index):
				}
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
		// Setup routing service
		bidRepoAdapter := repository.NewCallRoutingBidRepository(bidRepo)
		accountRepoAdapter := repository.NewCallRoutingAccountRepository(accountRepo)
		svc := callrouting.NewService(callRepo, bidRepoAdapter, accountRepoAdapter, nil,
			&callrouting.RoutingRules{
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
				
				decision, err := svc.RouteCall(ctx, testCall.ID)
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
	
	callRepo := repository.NewCallRepository(testDB.DB())
	bidRepo := repository.NewBidRepository(testDB.DB())
	
	t.Run("foreign_key_constraint", func(t *testing.T) {
		// Attempt to create bid with non-existent call ID
		invalidBid := &bid.Bid{
			ID:       uuid.New(),
			CallID:   uuid.New(), // Non-existent call
			BuyerID:  uuid.New(),
			Amount:   5.00,
			Status:   bid.StatusActive,
			Quality:  bid.QualityMetrics{},
			PlacedAt: time.Now(),
			ExpiresAt: time.Now().Add(5 * time.Minute),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		
		err := bidRepo.Create(ctx, invalidBid)
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
		err := callRepo.Create(ctx, &duplicateCall)
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
	callRepo := repository.NewCallRepository(testDB.DB())
	bidRepo := repository.NewBidRepository(testDB.DB())
	accountRepo := repository.NewAccountRepository(testDB.DB())
	
	// Setup routing service
	bidRepoAdapter := repository.NewCallRoutingBidRepository(bidRepo)
	accountRepoAdapter := repository.NewCallRoutingAccountRepository(accountRepo)
	svc := callrouting.NewService(callRepo, bidRepoAdapter, accountRepoAdapter, nil,
		&callrouting.RoutingRules{Algorithm: "round-robin"})
	
	t.Run("no_bids_available", func(t *testing.T) {
		// Create call with no bids
		testData := fixtures.CreateCompleteTestSet(t, testDB)
		testCall := testData.InboundCall
		
		// Attempt routing
		decision, err := svc.RouteCall(ctx, testCall.ID)
		
		// Should fail with appropriate error
		require.Error(t, err)
		assert.Nil(t, decision)
		
		var appErr *domainErrors.AppError
		if assert.ErrorAs(t, err, &appErr) {
			assert.Equal(t, domainErrors.ErrorTypeNotFound, appErr.Type,
				"No bids available should return NotFound error")
		}
	})
	
	t.Run("call_not_routable", func(t *testing.T) {
		testData := fixtures.CreateCompleteTestSet(t, testDB)
		testCall := testData.InboundCall
		
		// Set call to completed state
		testCall.Status = call.StatusCompleted
		err := callRepo.Update(ctx, testCall)
		require.NoError(t, err)
		
		// Create a bid for the call
		fixtures.NewBidBuilder(testDB).
			WithCallID(testCall.ID).
			WithBuyerID(testData.SellerAccount1.ID).
			BuildWithRepo(t, bidRepo, ctx)
		
		// Attempt routing
		decision, err := svc.RouteCall(ctx, testCall.ID)
		
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
		
		// Create suspended seller account
		suspendedSeller := fixtures.NewAccountBuilder(testDB).
			WithType(account.AccountTypeSeller).
			WithStatus(account.StatusSuspended).
			Build(t)
		
		// Create bid from suspended seller
		fixtures.NewBidBuilder(testDB).
			WithCallID(testCall.ID).
			WithBuyerID(suspendedSeller.ID).
			WithCriteria(bid.BidCriteria{
				CallType: []string{"inbound"},
			}).
			BuildWithRepo(t, bidRepo, ctx)
		
		// Attempt routing
		decision, err := svc.RouteCall(ctx, testCall.ID)
		
		// Should fail as no valid buyers
		require.Error(t, err)
		assert.Nil(t, decision)
	})
}

// Helper functions

type routingResult struct {
	decision *callrouting.RoutingDecision
	err      error
	attempt  int
}

func resetTestState(t *testing.T, ctx context.Context, testCall *call.Call,
	callRepo repository.CallRepository, bids []*bid.Bid, bidRepo repository.BidRepository) {
	t.Helper()
	
	// Reset call to pending
	testCall.Status = call.StatusPending
	err := callRepo.Update(ctx, testCall)
	require.NoError(t, err)
	
	// Reset all bids to active
	for _, bid := range bids {
		bid.Status = bid.StatusActive
		err := bidRepo.Update(ctx, bid)
		require.NoError(t, err)
	}
}