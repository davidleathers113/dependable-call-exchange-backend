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
	repos := setupRepositories(testDB)
	
	// Use fixture builders for test data setup
	testData := fixtures.CreateCompleteTestSet(t, testDB)
	
	// Use call scenarios for better readability
	callScenarios := fixtures.NewCallScenarios(t)
	testCall := testData.InboundCall
	testCall.Status = call.StatusPending
	err := repos.callRepo.Update(ctx, testCall)
	require.NoError(t, err)
	
	// Use bid scenarios for creating test bids
	bidScenarios := fixtures.NewBidScenarios(t, testDB)
	
	// Create competing bids using scenario builders
	bid1 := bidScenarios.HighValueBid(testCall.ID)
	bid1.BuyerID = testData.SellerAccount1.ID
	bid1.Amount = 5.50
	err = repos.bidRepo.Create(ctx, bid1)
	require.NoError(t, err)
	
	bid2 := bidScenarios.LowValueBid(testCall.ID)
	bid2.BuyerID = testData.SellerAccount2.ID
	bid2.Amount = 4.75
	bid2.Quality = bid.QualityMetrics{
		ConversionRate:   0.95,
		AverageCallTime:  250,
		FraudScore:       0.005,
		HistoricalRating: 9.5,
	}
	err = repos.bidRepo.Create(ctx, bid2)
	require.NoError(t, err)
	
	// Test different routing algorithms using sub-tests
	t.Run("routing_algorithms", func(t *testing.T) {
		algorithms := []struct {
			name     string
			rules    *callrouting.RoutingRules
			validate func(t *testing.T, decision *callrouting.RoutingDecision)
		}{
			{
				name: "round_robin",
				rules: &callrouting.RoutingRules{
					Algorithm: "round-robin",
				},
				validate: validateRoundRobinDecision(testData),
			},
			{
				name: "skill_based",
				rules: &callrouting.RoutingRules{
					Algorithm: "skill-based",
				},
				validate: validateSkillBasedDecision(repos.bidRepo, ctx),
			},
			{
				name: "cost_based",
				rules: &callrouting.RoutingRules{
					Algorithm:      "cost-based",
					QualityWeight:  0.4,
					PriceWeight:    0.4,
					CapacityWeight: 0.2,
				},
				validate: validateCostBasedDecision,
			},
		}
		
		for _, algo := range algorithms {
			t.Run(algo.name, func(t *testing.T) {
				// Reset test state for each algorithm
				resetTestState(t, ctx, testCall, repos.callRepo, []*bid.Bid{bid1, bid2}, repos.bidRepo)
				
				// Create routing service
				svc := createRoutingService(repos, algo.rules)
				
				// Route the call
				decision, err := svc.RouteCall(ctx, testCall.ID)
				require.NoError(t, err)
				require.NotNil(t, decision)
				
				// Validate decision based on algorithm
				algo.validate(t, decision)
				
				// Common validations
				t.Run("post_routing_state", func(t *testing.T) {
					validatePostRoutingState(t, ctx, repos, testCall.ID, decision.BidID)
				})
			})
		}
	})
}

// TestCallLifecycle_Complete tests the complete call lifecycle with proper state transitions
func TestCallLifecycle_Complete(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	ctx := testutil.TestContext(t)
	
	// Use fixture builders
	testData := fixtures.CreateCompleteTestSet(t, testDB)
	testCall := testData.InboundCall
	
	// Use bid scenarios
	bidScenarios := fixtures.NewBidScenarios(t, testDB)
	testBid := bidScenarios.LowValueBid(testCall.ID)
	testBid.BuyerID = testData.SellerAccount1.ID
	
	// Save bid
	bidRepo := repository.NewBidRepository(testDB.DB())
	err := bidRepo.Create(ctx, testBid)
	require.NoError(t, err)
	
	// Get repository for updates
	callRepo := repository.NewCallRepository(testDB.DB())
	
	// Test status transitions using sub-tests
	t.Run("status_transitions", func(t *testing.T) {
		testStatusTransitions(t, ctx, callRepo, testCall)
	})
	
	t.Run("call_completion", func(t *testing.T) {
		testCallCompletion(t, ctx, callRepo, testCall)
	})
}

// TestConcurrentBidding tests concurrent bid creation and routing with proper synchronization
func TestConcurrentBidding(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	ctx := testutil.TestContext(t)
	
	// Setup repositories
	repos := setupRepositories(testDB)
	
	// Use fixtures for initial setup
	testData := fixtures.CreateCompleteTestSet(t, testDB)
	testCall := testData.InboundCall
	
	// Ensure call is in pending state
	testCall.Status = call.StatusPending
	err := repos.callRepo.Update(ctx, testCall)
	require.NoError(t, err)
	
	// Create multiple seller accounts using account scenarios
	numBidders := 10
	accountScenarios := fixtures.NewAccountScenarios(t, testDB)
	sellers := accountScenarios.MultipleSellerAccounts(numBidders)
	
	t.Run("concurrent_bid_creation", func(t *testing.T) {
		testConcurrentBidCreation(t, ctx, repos.bidRepo, testCall.ID, sellers)
	})
	
	t.Run("concurrent_routing_attempts", func(t *testing.T) {
		testConcurrentRoutingAttempts(t, ctx, repos, testCall.ID)
	})
}

// TestDatabaseConstraints validates database integrity constraints
func TestDatabaseConstraints(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	ctx := testutil.TestContext(t)
	
	repos := setupRepositories(testDB)
	
	t.Run("foreign_key_constraint", func(t *testing.T) {
		testForeignKeyConstraint(t, ctx, repos.bidRepo)
	})
	
	t.Run("unique_constraint", func(t *testing.T) {
		testUniqueConstraint(t, ctx, testDB, repos.callRepo)
	})
	
	t.Run("transaction_rollback", func(t *testing.T) {
		testTransactionRollback(t, ctx, testDB)
	})
}

// TestNegativeScenarios tests error cases and edge conditions
func TestNegativeScenarios(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	ctx := testutil.TestContext(t)
	
	// Setup repositories and service
	repos := setupRepositories(testDB)
	svc := createRoutingService(repos, &callrouting.RoutingRules{Algorithm: "round-robin"})
	
	t.Run("no_bids_available", func(t *testing.T) {
		testNoBidsAvailable(t, ctx, testDB, svc)
	})
	
	t.Run("call_not_routable", func(t *testing.T) {
		testCallNotRoutable(t, ctx, testDB, repos, svc)
	})
	
	t.Run("suspended_buyer_account", func(t *testing.T) {
		testSuspendedBuyerAccount(t, ctx, testDB, repos, svc)
	})
}

// Helper types and functions

type repositories struct {
	callRepo    repository.CallRepository
	bidRepo     repository.BidRepository
	accountRepo repository.AccountRepository
}

type routingResult struct {
	decision *callrouting.RoutingDecision
	err      error
	attempt  int
}

// setupRepositories creates and returns all repositories
func setupRepositories(testDB *testutil.TestDB) repositories {
	return repositories{
		callRepo:    repository.NewCallRepository(testDB.DB()),
		bidRepo:     repository.NewBidRepository(testDB.DB()),
		accountRepo: repository.NewAccountRepository(testDB.DB()),
	}
}

// createRoutingService creates a new routing service with the given rules
func createRoutingService(repos repositories, rules *callrouting.RoutingRules) *callrouting.Service {
	bidRepoAdapter := repository.NewCallRoutingBidRepository(repos.bidRepo)
	accountRepoAdapter := repository.NewCallRoutingAccountRepository(repos.accountRepo)
	return callrouting.NewService(repos.callRepo, bidRepoAdapter, accountRepoAdapter, nil, rules)
}

// Validation functions

func validateRoundRobinDecision(testData *fixtures.TestDataSet) func(t *testing.T, decision *callrouting.RoutingDecision) {
	return func(t *testing.T, decision *callrouting.RoutingDecision) {
		assert.Equal(t, "round-robin", decision.Algorithm)
		validBuyerIDs := []uuid.UUID{testData.SellerAccount1.ID, testData.SellerAccount2.ID}
		assert.Contains(t, validBuyerIDs, decision.BuyerID, 
			"Round-robin should select one of the available sellers")
	}
}

func validateSkillBasedDecision(bidRepo repository.BidRepository, ctx context.Context) func(t *testing.T, decision *callrouting.RoutingDecision) {
	return func(t *testing.T, decision *callrouting.RoutingDecision) {
		assert.Equal(t, "skill-based", decision.Algorithm)
		assert.NotNil(t, decision.BidID)
		assert.Greater(t, decision.Score, 0.0, "Skill-based routing should provide a score")
		
		// Verify the selected bid has appropriate criteria
		selectedBid, err := bidRepo.GetByID(ctx, decision.BidID)
		require.NoError(t, err)
		assert.Contains(t, selectedBid.Criteria.CallType, "inbound",
			"Selected bid should accept inbound calls")
	}
}

func validateCostBasedDecision(t *testing.T, decision *callrouting.RoutingDecision) {
	assert.Equal(t, "cost-based", decision.Algorithm)
	assert.Greater(t, decision.Score, 0.0, "Cost-based routing should provide a score")
	
	// Verify the decision contains metadata about the scoring
	metadata, ok := decision.Metadata["weight_breakdown"]
	assert.True(t, ok, "Cost-based routing should include weight breakdown in metadata")
	_ = metadata // Avoid unused variable warning
}

func validatePostRoutingState(t *testing.T, ctx context.Context, repos repositories, callID, winningBidID uuid.UUID) {
	// Verify call status update
	updatedCall, err := repos.callRepo.GetByID(ctx, callID)
	require.NoError(t, err)
	assert.Equal(t, call.StatusQueued, updatedCall.Status,
		"Call should be moved to queued status after routing")
	
	// Verify winning bid status
	winningBid, err := repos.bidRepo.GetByID(ctx, winningBidID)
	require.NoError(t, err)
	assert.Equal(t, bid.StatusWon, winningBid.Status,
		"Winning bid should be marked as won")
}

// Test helper functions

func testStatusTransitions(t *testing.T, ctx context.Context, callRepo repository.CallRepository, testCall *call.Call) {
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
}

func testCallCompletion(t *testing.T, ctx context.Context, callRepo repository.CallRepository, testCall *call.Call) {
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
}

func testConcurrentBidCreation(t *testing.T, ctx context.Context, bidRepo repository.BidRepository, callID uuid.UUID, sellers []*account.Account) {
	numBidders := len(sellers)
	var wg sync.WaitGroup
	bidChan := make(chan *bid.Bid, numBidders)
	errChan := make(chan error, numBidders)
	
	// Create bids concurrently
	for i := 0; i < numBidders; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			
			// Use bid scenarios for variety
			bidScenarios := fixtures.NewBidScenarios(t, nil)
			var newBid *bid.Bid
			
			// Alternate between high and low value bids
			if index%2 == 0 {
				newBid = bidScenarios.HighValueBid(callID)
			} else {
				newBid = bidScenarios.LowValueBid(callID)
			}
			
			newBid.BuyerID = sellers[index].ID
			newBid.Amount = float64(index) + 1.0
			
			err := bidRepo.Create(ctx, newBid)
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
}

func testConcurrentRoutingAttempts(t *testing.T, ctx context.Context, repos repositories, callID uuid.UUID) {
	// Setup routing service
	svc := createRoutingService(repos, &callrouting.RoutingRules{
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
			
			decision, err := svc.RouteCall(ctx, callID)
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
}

func testForeignKeyConstraint(t *testing.T, ctx context.Context, bidRepo repository.BidRepository) {
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
}

func testUniqueConstraint(t *testing.T, ctx context.Context, testDB *testutil.TestDB, callRepo repository.CallRepository) {
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
}

func testTransactionRollback(t *testing.T, ctx context.Context, testDB *testutil.TestDB) {
	// Use fixtures for setup
	testData := fixtures.CreateCompleteTestSet(t, testDB)
	
	// Count calls before transaction
	initialCount := testDB.GetRowCount(t, "calls")
	
	// Execute transaction that should rollback
	err := testDB.WithTx(ctx, func(tx *sql.Tx) error {
		// Create new call in transaction
		callScenarios := fixtures.NewCallScenarios(t)
		newCall := callScenarios.InboundCall()
		newCall.BuyerID = testData.BuyerAccount.ID
		
		// Use transactional repository
		txCallRepo := repository.NewCallRepositoryWithTx(tx)
		err := txCallRepo.Create(ctx, newCall)
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
}

func testNoBidsAvailable(t *testing.T, ctx context.Context, testDB *testutil.TestDB, svc *callrouting.Service) {
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
}

func testCallNotRoutable(t *testing.T, ctx context.Context, testDB *testutil.TestDB, repos repositories, svc *callrouting.Service) {
	testData := fixtures.CreateCompleteTestSet(t, testDB)
	testCall := testData.InboundCall
	
	// Set call to completed state
	testCall.Status = call.StatusCompleted
	err := repos.callRepo.Update(ctx, testCall)
	require.NoError(t, err)
	
	// Create a bid for the call
	bidScenarios := fixtures.NewBidScenarios(t, testDB)
	testBid := bidScenarios.LowValueBid(testCall.ID)
	testBid.BuyerID = testData.SellerAccount1.ID
	err = repos.bidRepo.Create(ctx, testBid)
	require.NoError(t, err)
	
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
}

func testSuspendedBuyerAccount(t *testing.T, ctx context.Context, testDB *testutil.TestDB, repos repositories, svc *callrouting.Service) {
	testData := fixtures.CreateCompleteTestSet(t, testDB)
	testCall := testData.InboundCall
	
	// Create suspended seller account using account scenarios
	accountScenarios := fixtures.NewAccountScenarios(t, testDB)
	suspendedSeller := accountScenarios.SuspendedSellerAccount()
	
	// Create bid from suspended seller
	bidScenarios := fixtures.NewBidScenarios(t, testDB)
	testBid := bidScenarios.LowValueBid(testCall.ID)
	testBid.BuyerID = suspendedSeller.ID
	err := repos.bidRepo.Create(ctx, testBid)
	require.NoError(t, err)
	
	// Attempt routing
	decision, err := svc.RouteCall(ctx, testCall.ID)
	
	// Should fail as no valid buyers
	require.Error(t, err)
	assert.Nil(t, decision)
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