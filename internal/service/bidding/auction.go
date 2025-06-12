package bidding

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/bid"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
	"github.com/google/uuid"
)

// auctionEngine implements the AuctionEngine interface
type auctionEngine struct {
	bidRepo  BidRepository
	callRepo CallRepository
	notifier NotificationService
	metrics  MetricsCollector

	// Auction configuration
	minDuration time.Duration
	maxDuration time.Duration
	closeDelay  time.Duration // Delay after last bid

	// Active auctions tracking
	mu       sync.RWMutex
	auctions map[uuid.UUID]*activeAuction
}

// activeAuction tracks a running auction
type activeAuction struct {
	callID      uuid.UUID
	startTime   time.Time
	lastBidTime time.Time
	status      string
	mu          sync.RWMutex
	closing     bool
	closeTimer  *time.Timer
}

// NewAuctionEngine creates a new auction engine
func NewAuctionEngine(
	bidRepo BidRepository,
	callRepo CallRepository,
	notifier NotificationService,
	metrics MetricsCollector,
) AuctionEngine {
	return &auctionEngine{
		bidRepo:     bidRepo,
		callRepo:    callRepo,
		notifier:    notifier,
		metrics:     metrics,
		minDuration: 30 * time.Second,
		maxDuration: 5 * time.Minute,
		closeDelay:  5 * time.Second,
		auctions:    make(map[uuid.UUID]*activeAuction),
	}
}

// RunAuction executes the auction for a call
func (e *auctionEngine) RunAuction(ctx context.Context, callID uuid.UUID) (*AuctionResult, error) {
	// Check if auction already exists
	e.mu.Lock()
	if _, exists := e.auctions[callID]; exists {
		e.mu.Unlock()
		return nil, errors.NewConflictError("auction already running for call")
	}

	// Create new auction
	auction := &activeAuction{
		callID:    callID,
		startTime: time.Now(),
		status:    "open",
	}
	e.auctions[callID] = auction
	e.mu.Unlock()

	// Start auction timer
	go e.monitorAuction(ctx, auction)

	// Get initial bids
	bids, err := e.bidRepo.GetActiveBidsForCall(ctx, callID)
	if err != nil {
		return nil, errors.NewInternalError("failed to get bids").WithCause(err)
	}

	// If we have bids, start close timer
	if len(bids) > 0 {
		e.resetCloseTimer(auction)
	}

	return &AuctionResult{
		CallID:       callID,
		StartTime:    auction.startTime,
		Participants: len(bids),
	}, nil
}

// GetAuctionStatus returns current auction state
func (e *auctionEngine) GetAuctionStatus(ctx context.Context, callID uuid.UUID) (*AuctionStatus, error) {
	e.mu.RLock()
	auction, exists := e.auctions[callID]
	e.mu.RUnlock()

	if !exists {
		return nil, errors.NewNotFoundError("auction")
	}

	// Get current bids
	bids, err := e.bidRepo.GetActiveBidsForCall(ctx, callID)
	if err != nil {
		return nil, errors.NewInternalError("failed to get bids").WithCause(err)
	}

	// Find top bid
	var topBid float64
	if len(bids) > 0 {
		sort.Slice(bids, func(i, j int) bool {
			return bids[i].Amount.Compare(bids[j].Amount) > 0
		})
		topBid = bids[0].Amount.ToFloat64()
	}

	auction.mu.RLock()
	status := auction.status
	closing := auction.closing
	auction.mu.RUnlock()

	// Calculate time left
	var timeLeft time.Duration
	if closing {
		timeLeft = e.closeDelay - time.Since(auction.lastBidTime)
		if timeLeft < 0 {
			timeLeft = 0
		}
	} else {
		elapsed := time.Since(auction.startTime)
		if elapsed < e.maxDuration {
			timeLeft = e.maxDuration - elapsed
		}
	}

	return &AuctionStatus{
		CallID:       callID,
		Status:       status,
		BidCount:     len(bids),
		TopBidAmount: topBid,
		TimeLeft:     timeLeft,
		LastUpdate:   time.Now(),
	}, nil
}

// CloseAuction finalizes the auction
func (e *auctionEngine) CloseAuction(ctx context.Context, callID uuid.UUID) error {
	e.mu.Lock()
	auction, exists := e.auctions[callID]
	if !exists {
		e.mu.Unlock()
		return errors.NewNotFoundError("auction")
	}
	e.mu.Unlock()

	return e.finalizeAuction(ctx, auction)
}

// monitorAuction manages auction lifecycle
func (e *auctionEngine) monitorAuction(ctx context.Context, auction *activeAuction) {
	// Set maximum auction duration
	maxTimer := time.NewTimer(e.maxDuration)
	defer maxTimer.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-maxTimer.C:
			// Maximum duration reached
			if err := e.finalizeAuction(ctx, auction); err != nil {
				// Log error
			}
			return
		}
	}
}

// resetCloseTimer resets the auction close timer after a new bid
func (e *auctionEngine) resetCloseTimer(auction *activeAuction) {
	auction.mu.Lock()
	defer auction.mu.Unlock()

	auction.lastBidTime = time.Now()
	auction.closing = true

	// Cancel existing timer
	if auction.closeTimer != nil {
		auction.closeTimer.Stop()
	}

	// Start new timer
	auction.closeTimer = time.AfterFunc(e.closeDelay, func() {
		ctx := context.Background()
		if err := e.finalizeAuction(ctx, auction); err != nil {
			// Log error
		}
	})
}

// finalizeAuction closes the auction and determines winner
func (e *auctionEngine) finalizeAuction(ctx context.Context, auction *activeAuction) error {
	// Mark as closing
	auction.mu.Lock()
	if auction.status == "closed" {
		auction.mu.Unlock()
		return nil // Already closed
	}
	auction.status = "closing"
	auction.mu.Unlock()

	// Get all bids
	bids, err := e.bidRepo.GetActiveBidsForCall(ctx, auction.callID)
	if err != nil {
		return errors.NewInternalError("failed to get bids").WithCause(err)
	}

	// Sort bids by amount (highest first)
	sort.Slice(bids, func(i, j int) bool {
		return bids[i].Amount.Compare(bids[j].Amount) > 0
	})

	endTime := time.Now()

	if len(bids) > 0 {
		// Winner is highest bid
		winner := bids[0]
		winner.Accept()

		// Update winning bid
		if err := e.bidRepo.Update(ctx, winner); err != nil {
			return errors.NewInternalError("failed to update winning bid").WithCause(err)
		}

		// Mark other bids as lost
		for i := 1; i < len(bids); i++ {
			bids[i].Reject()
			if err := e.bidRepo.Update(ctx, bids[i]); err != nil {
				// Log error but continue
			}

			// Notify loser
			go e.notifier.NotifyBidLost(context.Background(), bids[i])
		}

		// Notify winner
		go e.notifier.NotifyBidWon(context.Background(), winner)
	}

	// Record metrics
	if e.metrics != nil {
		duration := endTime.Sub(auction.startTime)
		e.metrics.RecordAuctionDuration(ctx, auction.callID, duration)
	}

	// Mark as closed and remove from active auctions
	auction.mu.Lock()
	auction.status = "closed"
	if auction.closeTimer != nil {
		auction.closeTimer.Stop()
	}
	auction.mu.Unlock()

	e.mu.Lock()
	delete(e.auctions, auction.callID)
	e.mu.Unlock()

	return nil
}

// HandleNewBid processes a new bid in the auction
func (e *auctionEngine) HandleNewBid(ctx context.Context, newBid *bid.Bid) error {
	e.mu.RLock()
	auction, exists := e.auctions[newBid.CallID]
	e.mu.RUnlock()

	if !exists {
		// Start new auction
		_, err := e.RunAuction(ctx, newBid.CallID)
		return err
	}

	// Reset close timer
	e.resetCloseTimer(auction)

	return nil
}
