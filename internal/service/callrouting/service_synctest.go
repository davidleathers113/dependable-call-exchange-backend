//go:build synctest

package callrouting

import (
	"context"
	"testing"
	"testing/synctest"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/call"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/bid"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/testutil/mocks"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestService_RouteCall_WithTimeout tests routing with deterministic timing
func TestService_RouteCall_WithTimeout(t *testing.T) {
	synctest.Run(func() {
		ctx := context.Background()
		
		// Setup mocks
		callRepo := new(mocks.CallRepository)
		bidRepo := new(mocks.BidRepository)
		accountRepo := new(mocks.AccountRepository)
		
		callID := uuid.New()
		testCall := &call.Call{
			ID:     callID,
			Status: call.StatusPending,
		}
		
		bids := []*bid.Bid{
			{
				ID:           uuid.New(),
				CallID:       callID,
				BuyerID:      uuid.New(),
				Amount:       5.0,
				Status:       bid.StatusActive,
				QualityScore: 85,
			},
		}
		
		// Simulate slow database operations
		callRepo.On("GetByID", ctx, callID).Return(testCall, nil).After(100 * time.Millisecond)
		bidRepo.On("GetActiveBidsForCall", ctx, callID).Return(bids, nil).After(200 * time.Millisecond)
		callRepo.On("Update", ctx, testCall).Return(nil).After(50 * time.Millisecond)
		
		svc := NewService(callRepo, bidRepo, accountRepo, nil, &RoutingRules{Algorithm: "round-robin"})
		
		// Start routing with timeout
		timeoutCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
		defer cancel()
		
		start := time.Now()
		decision, err := svc.RouteCall(timeoutCtx, callID)
		elapsed := time.Since(start)
		
		// With synctest, timing is deterministic
		require.NoError(t, err)
		assert.NotNil(t, decision)
		assert.Equal(t, 350*time.Millisecond, elapsed) // 100+200+50ms exactly
		
		// Wait for all goroutines to complete
		synctest.Wait()
		
		callRepo.AssertExpectations(t)
		bidRepo.AssertExpectations(t)
	})
}

// TestService_ConcurrentRouting_Deterministic tests concurrent routing with synctest
func TestService_ConcurrentRouting_Deterministic(t *testing.T) {
	synctest.Run(func() {
		ctx := context.Background()
		
		// Setup mocks for multiple calls
		callRepo := new(mocks.CallRepository)
		bidRepo := new(mocks.BidRepository)
		accountRepo := new(mocks.AccountRepository)
		
		numCalls := 5
		callIDs := make([]uuid.UUID, numCalls)
		
		for i := 0; i < numCalls; i++ {
			callID := uuid.New()
			callIDs[i] = callID
			
			testCall := &call.Call{
				ID:     callID,
				Status: call.StatusPending,
			}
			
			bids := []*bid.Bid{
				{
					ID:           uuid.New(),
					CallID:       callID,
					BuyerID:      uuid.New(),
					Amount:       float64(i) + 1.0,
					Status:       bid.StatusActive,
					QualityScore: 85,
				},
			}
			
			// Each call takes different amounts of time
			delay := time.Duration(i+1) * 100 * time.Millisecond
			callRepo.On("GetByID", ctx, callID).Return(testCall, nil).After(delay)
			bidRepo.On("GetActiveBidsForCall", ctx, callID).Return(bids, nil).After(delay)
			callRepo.On("Update", ctx, testCall).Return(nil).After(delay/2)
		}
		
		svc := NewService(callRepo, bidRepo, accountRepo, nil, &RoutingRules{Algorithm: "round-robin"})
		
		// Start all routing operations concurrently
		results := make(chan struct {
			decision *RoutingDecision
			err      error
			index    int
		}, numCalls)
		
		start := time.Now()
		
		for i, callID := range callIDs {
			go func(index int, cid uuid.UUID) {
				decision, err := svc.RouteCall(ctx, cid)
				results <- struct {
					decision *RoutingDecision
					err      error
					index    int
				}{decision, err, index}
			}(i, callID)
		}
		
		// Wait for all goroutines to become idle
		synctest.Wait()
		
		// Collect all results
		completionTimes := make([]time.Duration, numCalls)
		for i := 0; i < numCalls; i++ {
			result := <-results
			require.NoError(t, result.err)
			assert.NotNil(t, result.decision)
			
			// With synctest, we can precisely measure when each completed
			completionTimes[result.index] = time.Since(start)
		}
		
		// Verify deterministic completion order
		// Call 0 should complete first (300ms), Call 1 next (600ms), etc.
		for i := 0; i < numCalls-1; i++ {
			assert.True(t, completionTimes[i] < completionTimes[i+1],
				"Call %d should complete before Call %d", i, i+1)
		}
		
		// Verify exact completion times (deterministic with synctest)
		expectedTimes := []time.Duration{
			250 * time.Millisecond, // Call 0: 100+100+50
			500 * time.Millisecond, // Call 1: 200+200+100
			750 * time.Millisecond, // Call 2: 300+300+150
			1000 * time.Millisecond, // Call 3: 400+400+200
			1250 * time.Millisecond, // Call 4: 500+500+250
		}
		
		for i, expected := range expectedTimes {
			assert.Equal(t, expected, completionTimes[i],
				"Call %d should complete at exactly %v", i, expected)
		}
		
		callRepo.AssertExpectations(t)
		bidRepo.AssertExpectations(t)
	})
}

// TestService_RoutingWithPeriodicCleanup tests routing with background cleanup tasks
func TestService_RoutingWithPeriodicCleanup(t *testing.T) {
	synctest.Run(func() {
		ctx := context.Background()
		
		callRepo := new(mocks.CallRepository)
		bidRepo := new(mocks.BidRepository)
		accountRepo := new(mocks.AccountRepository)
		
		callID := uuid.New()
		testCall := &call.Call{
			ID:     callID,
			Status: call.StatusPending,
		}
		
		bids := []*bid.Bid{
			{
				ID:           uuid.New(),
				CallID:       callID,
				BuyerID:      uuid.New(),
				Amount:       5.0,
				Status:       bid.StatusActive,
				QualityScore: 85,
			},
		}
		
		callRepo.On("GetByID", ctx, callID).Return(testCall, nil)
		bidRepo.On("GetActiveBidsForCall", ctx, callID).Return(bids, nil)
		callRepo.On("Update", ctx, testCall).Return(nil)
		
		// Mock cleanup operations
		bidRepo.On("CleanupExpiredBids", ctx).Return(nil)
		
		svc := NewService(callRepo, bidRepo, accountRepo, nil, &RoutingRules{Algorithm: "round-robin"})
		
		// Start periodic cleanup task
		cleanupTicker := time.NewTicker(1 * time.Second)
		defer cleanupTicker.Stop()
		
		cleanupDone := make(chan bool)
		go func() {
			defer close(cleanupDone)
			for {
				select {
				case <-ctx.Done():
					return
				case <-cleanupTicker.C:
					bidRepo.CleanupExpiredBids(ctx)
					return // For test, cleanup once
				}
			}
		}()
		
		// Route the call
		decision, err := svc.RouteCall(ctx, callID)
		require.NoError(t, err)
		assert.NotNil(t, decision)
		
		// Advance time to trigger cleanup
		time.Sleep(1 * time.Second)
		synctest.Wait()
		
		// Verify cleanup was called
		<-cleanupDone
		bidRepo.AssertExpectations(t)
	})
}

// TestService_RouteCall_ContextCancellation tests context cancellation behavior
func TestService_RouteCall_ContextCancellation(t *testing.T) {
	synctest.Run(func() {
		callRepo := new(mocks.CallRepository)
		bidRepo := new(mocks.BidRepository)
		accountRepo := new(mocks.AccountRepository)
		
		callID := uuid.New()
		
		// Setup slow database operation
		callRepo.On("GetByID").Return(nil, context.Canceled).After(2 * time.Second)
		
		svc := NewService(callRepo, bidRepo, accountRepo, nil, &RoutingRules{Algorithm: "round-robin"})
		
		// Create context that cancels after 1 second
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		
		start := time.Now()
		decision, err := svc.RouteCall(ctx, callID)
		elapsed := time.Since(start)
		
		// Should fail due to context cancellation
		require.Error(t, err)
		assert.Nil(t, decision)
		assert.Contains(t, err.Error(), "context")
		assert.Equal(t, 1*time.Second, elapsed) // Exactly 1 second with synctest
		
		synctest.Wait()
	})
}