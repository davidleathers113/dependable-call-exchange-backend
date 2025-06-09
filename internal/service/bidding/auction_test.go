package bidding

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/bid"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/testutil/mocks"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestAuctionEngine_RunAuction(t *testing.T) {
	ctx := context.Background()

	callID := uuid.New() // Use same ID for all tests
	
	tests := []struct {
		name          string
		setupMocks    func(*mocks.BidRepository, *mocks.CallRepository)
		expectedError bool
		errorContains string
		validate      func(*testing.T, *AuctionResult)
	}{
		{
			name: "successful auction start with existing bids",
			setupMocks: func(br *mocks.BidRepository, cr *mocks.CallRepository) {
				bids := []*bid.Bid{
					{
						ID:      uuid.New(),
						CallID:  callID,
						BuyerID: uuid.New(),
						Amount:  5.0,
						Status:  bid.StatusActive,
					},
					{
						ID:      uuid.New(),
						CallID:  callID,
						BuyerID: uuid.New(),
						Amount:  6.0,
						Status:  bid.StatusActive,
					},
				}
				
				br.On("GetActiveBidsForCall", ctx, callID).Return(bids, nil)
			},
			expectedError: false,
			validate: func(t *testing.T, result *AuctionResult) {
				assert.NotNil(t, result)
				assert.Equal(t, 2, result.Participants)
				assert.NotZero(t, result.StartTime)
			},
		},
		{
			name: "auction start with no bids",
			setupMocks: func(br *mocks.BidRepository, cr *mocks.CallRepository) {
				br.On("GetActiveBidsForCall", ctx, callID).Return([]*bid.Bid{}, nil)
			},
			expectedError: false,
			validate: func(t *testing.T, result *AuctionResult) {
				assert.NotNil(t, result)
				assert.Equal(t, 0, result.Participants)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mocks
			bidRepo := new(mocks.BidRepository)
			callRepo := new(mocks.CallRepository)
			notifier := new(mocks.NotificationService)
			metrics := new(mocks.MetricsCollector)
			
			// Setup mocks
			tt.setupMocks(bidRepo, callRepo)
			
			// Create engine with short durations for testing
			engine := &auctionEngine{
				bidRepo:     bidRepo,
				callRepo:    callRepo,
				notifier:    notifier,
				metrics:     metrics,
				minDuration: 100 * time.Millisecond,
				maxDuration: 500 * time.Millisecond,
				closeDelay:  50 * time.Millisecond,
				auctions:    make(map[uuid.UUID]*activeAuction),
			}
			
			// Execute
			result, err := engine.RunAuction(ctx, callID)
			
			// Validate
			if tt.expectedError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
			} else {
				require.NoError(t, err)
				tt.validate(t, result)
			}
			
			// Cleanup
			engine.mu.Lock()
			if auction, exists := engine.auctions[callID]; exists {
				auction.mu.Lock()
				if auction.closeTimer != nil {
					auction.closeTimer.Stop()
				}
				auction.mu.Unlock()
				delete(engine.auctions, callID)
			}
			engine.mu.Unlock()
			
			// Assert expectations
			bidRepo.AssertExpectations(t)
			callRepo.AssertExpectations(t)
		})
	}
}

func TestAuctionEngine_GetAuctionStatus(t *testing.T) {
	ctx := context.Background()
	
	t.Run("get status of running auction", func(t *testing.T) {
		bidRepo := new(mocks.BidRepository)
		callID := uuid.New()
		
		// Setup active bids
		bids := []*bid.Bid{
			{
				ID:      uuid.New(),
				CallID:  callID,
				BuyerID: uuid.New(),
				Amount:  10.0,
				Status:  bid.StatusActive,
			},
			{
				ID:      uuid.New(),
				CallID:  callID,
				BuyerID: uuid.New(),
				Amount:  8.0,
				Status:  bid.StatusActive,
			},
		}
		
		bidRepo.On("GetActiveBidsForCall", ctx, callID).Return(bids, nil)
		
		// Create engine with active auction
		engine := &auctionEngine{
			bidRepo:     bidRepo,
			maxDuration: 5 * time.Minute,
			closeDelay:  5 * time.Second,
			auctions:    make(map[uuid.UUID]*activeAuction),
		}
		
		// Add active auction
		engine.auctions[callID] = &activeAuction{
			callID:    callID,
			startTime: time.Now(),
			status:    "open",
		}
		
		// Get status
		status, err := engine.GetAuctionStatus(ctx, callID)
		require.NoError(t, err)
		
		// Validate
		assert.Equal(t, callID, status.CallID)
		assert.Equal(t, "open", status.Status)
		assert.Equal(t, 2, status.BidCount)
		assert.Equal(t, 10.0, status.TopBidAmount)
		assert.Greater(t, status.TimeLeft.Seconds(), 0.0)
		
		// Cleanup
		delete(engine.auctions, callID)
	})
	
	t.Run("auction not found", func(t *testing.T) {
		engine := &auctionEngine{
			auctions: make(map[uuid.UUID]*activeAuction),
		}
		
		status, err := engine.GetAuctionStatus(ctx, uuid.New())
		require.Error(t, err)
		assert.Nil(t, status)
		assert.Contains(t, err.Error(), "auction not found")
	})
}

func TestAuctionEngine_FinalizeAuction(t *testing.T) {
	ctx := context.Background()
	
	t.Run("finalize with winner", func(t *testing.T) {
		bidRepo := new(mocks.BidRepository)
		notifier := new(mocks.NotificationService)
		metrics := new(mocks.MetricsCollector)
		
		callID := uuid.New()
		winnerID := uuid.New()
		loserID := uuid.New()
		
		// Create bids
		winningBid := &bid.Bid{
			ID:      uuid.New(),
			CallID:  callID,
			BuyerID: winnerID,
			Amount:  15.0,
			Status:  bid.StatusActive,
		}
		
		losingBid := &bid.Bid{
			ID:      uuid.New(),
			CallID:  callID,
			BuyerID: loserID,
			Amount:  10.0,
			Status:  bid.StatusActive,
		}
		
		bids := []*bid.Bid{winningBid, losingBid}
		
		// Setup expectations
		bidRepo.On("GetActiveBidsForCall", ctx, callID).Return(bids, nil)
		
		// Winning bid update
		bidRepo.On("Update", ctx, mock.MatchedBy(func(b *bid.Bid) bool {
			return b.ID == winningBid.ID && b.Status == bid.StatusWon
		})).Return(nil)
		
		// Losing bid update
		bidRepo.On("Update", ctx, mock.MatchedBy(func(b *bid.Bid) bool {
			return b.ID == losingBid.ID && b.Status == bid.StatusLost
		})).Return(nil)
		
		// Notifications (called in goroutines, so may not execute before test ends)
		notifier.On("NotifyBidWon", mock.Anything, mock.MatchedBy(func(b *bid.Bid) bool {
			return b.ID == winningBid.ID
		})).Return(nil).Maybe()
		
		notifier.On("NotifyBidLost", mock.Anything, mock.MatchedBy(func(b *bid.Bid) bool {
			return b.ID == losingBid.ID
		})).Return(nil).Maybe()
		
		// Metrics
		metrics.On("RecordAuctionDuration", ctx, callID, mock.AnythingOfType("time.Duration"))
		
		// Create engine
		engine := &auctionEngine{
			bidRepo:  bidRepo,
			notifier: notifier,
			metrics:  metrics,
			auctions: make(map[uuid.UUID]*activeAuction),
		}
		
		// Create auction
		auction := &activeAuction{
			callID:    callID,
			startTime: time.Now().Add(-30 * time.Second),
			status:    "open",
		}
		engine.auctions[callID] = auction
		
		// Execute
		err := engine.finalizeAuction(ctx, auction)
		require.NoError(t, err)
		
		// Verify auction was removed
		engine.mu.RLock()
		_, exists := engine.auctions[callID]
		engine.mu.RUnlock()
		assert.False(t, exists)
		
		// Assert expectations
		bidRepo.AssertExpectations(t)
		notifier.AssertExpectations(t)
		metrics.AssertExpectations(t)
	})
	
	t.Run("finalize with no bids", func(t *testing.T) {
		bidRepo := new(mocks.BidRepository)
		metrics := new(mocks.MetricsCollector)
		
		callID := uuid.New()
		
		// No bids
		bidRepo.On("GetActiveBidsForCall", ctx, callID).Return([]*bid.Bid{}, nil)
		
		// Metrics
		metrics.On("RecordAuctionDuration", ctx, callID, mock.AnythingOfType("time.Duration"))
		
		// Create engine
		engine := &auctionEngine{
			bidRepo:  bidRepo,
			metrics:  metrics,
			auctions: make(map[uuid.UUID]*activeAuction),
		}
		
		// Create auction
		auction := &activeAuction{
			callID:    callID,
			startTime: time.Now(),
			status:    "open",
		}
		engine.auctions[callID] = auction
		
		// Execute
		err := engine.finalizeAuction(ctx, auction)
		require.NoError(t, err)
		
		// Verify auction was removed
		engine.mu.RLock()
		_, exists := engine.auctions[callID]
		engine.mu.RUnlock()
		assert.False(t, exists)
		
		// Assert expectations
		bidRepo.AssertExpectations(t)
	})
}

func TestAuctionEngine_ConcurrentAuctions(t *testing.T) {
	ctx := context.Background()
	
	// Setup
	bidRepo := new(mocks.BidRepository)
	callRepo := new(mocks.CallRepository)
	
	numAuctions := 10
	callIDs := make([]uuid.UUID, numAuctions)
	
	// Setup expectations for each auction
	for i := 0; i < numAuctions; i++ {
		callIDs[i] = uuid.New()
		
		// Initial bid check
		bidRepo.On("GetActiveBidsForCall", ctx, callIDs[i]).Return([]*bid.Bid{
			{
				ID:      uuid.New(),
				CallID:  callIDs[i],
				BuyerID: uuid.New(),
				Amount:  float64(i) + 1.0,
				Status:  bid.StatusActive,
			},
		}, nil).Maybe()
	}
	
	// Create engine
	engine := &auctionEngine{
		bidRepo:     bidRepo,
		callRepo:    callRepo,
		minDuration: 100 * time.Millisecond,
		maxDuration: 200 * time.Millisecond,
		closeDelay:  50 * time.Millisecond,
		auctions:    make(map[uuid.UUID]*activeAuction),
	}
	
	// Start auctions concurrently
	var wg sync.WaitGroup
	errors := make(chan error, numAuctions)
	
	for i := 0; i < numAuctions; i++ {
		wg.Add(1)
		go func(callID uuid.UUID) {
			defer wg.Done()
			_, err := engine.RunAuction(ctx, callID)
			errors <- err
		}(callIDs[i])
	}
	
	// Wait for all to complete
	wg.Wait()
	close(errors)
	
	// Check results
	for err := range errors {
		require.NoError(t, err)
	}
	
	// Verify all auctions were created
	engine.mu.RLock()
	assert.Equal(t, numAuctions, len(engine.auctions))
	engine.mu.RUnlock()
	
	// Cleanup
	engine.mu.Lock()
	for _, auction := range engine.auctions {
		auction.mu.Lock()
		if auction.closeTimer != nil {
			auction.closeTimer.Stop()
		}
		auction.mu.Unlock()
	}
	engine.auctions = make(map[uuid.UUID]*activeAuction)
	engine.mu.Unlock()
}

func TestAuctionEngine_HandleNewBid(t *testing.T) {
	ctx := context.Background()
	
	t.Run("new bid extends auction closing time", func(t *testing.T) {
		bidRepo := new(mocks.BidRepository)
		callRepo := new(mocks.CallRepository)
		
		callID := uuid.New()
		
		// Setup initial auction start
		bidRepo.On("GetActiveBidsForCall", ctx, callID).Return([]*bid.Bid{}, nil)
		
		// Create engine
		engine := &auctionEngine{
			bidRepo:     bidRepo,
			callRepo:    callRepo,
			minDuration: 100 * time.Millisecond,
			maxDuration: 5 * time.Second,
			closeDelay:  100 * time.Millisecond,
			auctions:    make(map[uuid.UUID]*activeAuction),
		}
		
		// Create auction manually
		auction := &activeAuction{
			callID:    callID,
			startTime: time.Now(),
			status:    "open",
		}
		engine.auctions[callID] = auction
		
		// Create new bid
		newBid := &bid.Bid{
			ID:      uuid.New(),
			CallID:  callID,
			BuyerID: uuid.New(),
			Amount:  10.0,
			Status:  bid.StatusActive,
		}
		
		// Handle new bid
		err := engine.HandleNewBid(ctx, newBid)
		require.NoError(t, err)
		
		// Verify auction is now closing
		auction.mu.RLock()
		assert.True(t, auction.closing)
		assert.NotNil(t, auction.closeTimer)
		auction.mu.RUnlock()
		
		// Cleanup
		auction.mu.Lock()
		if auction.closeTimer != nil {
			auction.closeTimer.Stop()
		}
		auction.mu.Unlock()
		delete(engine.auctions, callID)
	})
	
	t.Run("new bid starts auction if not exists", func(t *testing.T) {
		bidRepo := new(mocks.BidRepository)
		callRepo := new(mocks.CallRepository)
		
		callID := uuid.New()
		
		// Setup for new auction
		bidRepo.On("GetActiveBidsForCall", ctx, callID).Return([]*bid.Bid{}, nil)
		
		// Create engine
		engine := &auctionEngine{
			bidRepo:     bidRepo,
			callRepo:    callRepo,
			minDuration: 100 * time.Millisecond,
			maxDuration: 500 * time.Millisecond,
			closeDelay:  50 * time.Millisecond,
			auctions:    make(map[uuid.UUID]*activeAuction),
		}
		
		// Create new bid
		newBid := &bid.Bid{
			ID:      uuid.New(),
			CallID:  callID,
			BuyerID: uuid.New(),
			Amount:  10.0,
			Status:  bid.StatusActive,
		}
		
		// Handle new bid
		err := engine.HandleNewBid(ctx, newBid)
		require.NoError(t, err)
		
		// Verify auction was created
		engine.mu.RLock()
		_, exists := engine.auctions[callID]
		engine.mu.RUnlock()
		assert.True(t, exists)
		
		// Cleanup
		engine.mu.Lock()
		if auction, exists := engine.auctions[callID]; exists {
			auction.mu.Lock()
			if auction.closeTimer != nil {
				auction.closeTimer.Stop()
			}
			auction.mu.Unlock()
			delete(engine.auctions, callID)
		}
		engine.mu.Unlock()
	})
}

func BenchmarkAuctionEngine_ProcessBids(b *testing.B) {
	ctx := context.Background()
	bidRepo := new(mocks.BidRepository)
	
	// Create many bids
	numBids := 100
	callID := uuid.New()
	bids := make([]*bid.Bid, numBids)
	
	for i := 0; i < numBids; i++ {
		bids[i] = &bid.Bid{
			ID:      uuid.New(),
			CallID:  callID,
			BuyerID: uuid.New(),
			Amount:  float64(i) + 1.0,
			Status:  bid.StatusActive,
		}
	}
	
	bidRepo.On("GetActiveBidsForCall", ctx, callID).Return(bids, nil)
	
	engine := &auctionEngine{
		bidRepo:  bidRepo,
		auctions: make(map[uuid.UUID]*activeAuction),
	}
	
	auction := &activeAuction{
		callID:    callID,
		startTime: time.Now(),
		status:    "open",
	}
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		// Simulate finalizing auction
		_ = engine.finalizeAuction(ctx, auction)
		
		// Reset auction state
		auction.status = "open"
	}
}