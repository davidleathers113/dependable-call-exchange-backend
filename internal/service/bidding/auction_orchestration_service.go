package bidding

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/bid"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/call"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
	"github.com/google/uuid"
)

// auctionOrchestrationService manages the complete auction lifecycle for calls.
//
// As an orchestrator service, it coordinates bid collection, winner selection,
// and notification dispatch. It maintains 5 dependencies (allowed up to 8 per ADR-001):
//
// - bidRepo: Access to bid data
// - callRepo: Access to call data
// - infrastructure: Combined notifier + metrics facade
// - mu & auctions: Internal state management
type auctionOrchestrationService struct {
	bidRepo        BidRepository
	callRepo       CallRepository
	infrastructure InfrastructureServices

	// Active auctions tracking
	mu       sync.RWMutex
	auctions map[uuid.UUID]*orchestratedAuction
}

// orchestratedAuction represents an ongoing auction with full state
type orchestratedAuction struct {
	callID       uuid.UUID
	status       string
	startTime    time.Time
	endTime      time.Time
	bids         []*bid.Bid
	winningBidID uuid.UUID
	mu           sync.RWMutex
}

// NewAuctionOrchestrationService creates a new auction orchestration service
func NewAuctionOrchestrationService(
	bidRepo BidRepository,
	callRepo CallRepository,
	infrastructure InfrastructureServices,
) AuctionOrchestrationService {
	return &auctionOrchestrationService{
		bidRepo:        bidRepo,
		callRepo:       callRepo,
		infrastructure: infrastructure,
		auctions:       make(map[uuid.UUID]*orchestratedAuction),
	}
}

// RunAuction executes the auction for a call
func (s *auctionOrchestrationService) RunAuction(ctx context.Context, callID uuid.UUID) (*AuctionResult, error) {
	// Verify call exists and is eligible
	c, err := s.callRepo.GetByID(ctx, callID)
	if err != nil {
		return nil, errors.NewNotFoundError("call").WithCause(err)
	}

	if c.Status != call.StatusPending && c.Status != call.StatusQueued {
		return nil, errors.NewValidationError("INVALID_CALL_STATUS",
			fmt.Sprintf("call must be in pending or queued status, got %s", c.Status))
	}

	// Get or create auction
	s.mu.Lock()
	auction, exists := s.auctions[callID]
	if !exists {
		auction = &orchestratedAuction{
			callID:    callID,
			status:    "open",
			startTime: time.Now(),
			endTime:   time.Now().Add(30 * time.Second), // 30 second auction
			bids:      make([]*bid.Bid, 0),
		}
		s.auctions[callID] = auction
	}
	s.mu.Unlock()

	// Get active bids for the call
	bids, err := s.bidRepo.GetActiveBidsForCall(ctx, callID)
	if err != nil {
		return nil, errors.NewInternalError("failed to get bids").WithCause(err)
	}

	if len(bids) == 0 {
		return nil, errors.NewBusinessError("NO_BIDS", "no active bids for call")
	}

	// Update auction with current bids
	auction.mu.Lock()
	auction.bids = bids
	auction.mu.Unlock()

	// Determine winner based on bid amount and quality
	winner := s.selectWinner(bids)
	if winner == nil {
		return nil, errors.NewBusinessError("NO_WINNER", "could not determine auction winner")
	}

	// Mark winning bid
	winner.Status = bid.StatusWon
	if err := s.bidRepo.Update(ctx, winner); err != nil {
		return nil, errors.NewInternalError("failed to update winning bid").WithCause(err)
	}

	// Mark losing bids
	for _, b := range bids {
		if b.ID != winner.ID {
			b.Status = bid.StatusLost
			if err := s.bidRepo.Update(ctx, b); err != nil {
				// Log error but continue
			}
		}
	}

	// Create result
	result := &AuctionResult{
		CallID:       callID,
		WinningBidID: winner.ID,
		WinnerID:     winner.BuyerID,
		FinalAmount:  winner.Amount.ToFloat64(),
		StartTime:    auction.startTime,
		EndTime:      time.Now(),
		Participants: len(bids),
	}

	// Collect runner-up IDs
	for _, b := range bids {
		if b.ID != winner.ID {
			result.RunnerUpBids = append(result.RunnerUpBids, b.ID)
		}
	}

	// Close auction
	auction.mu.Lock()
	auction.status = "closed"
	auction.winningBidID = winner.ID
	auction.mu.Unlock()

	// Send notifications
	if s.infrastructure != nil {
		go s.infrastructure.NotifyBidWon(context.Background(), winner)
		for _, b := range bids {
			if b.ID != winner.ID {
				go s.infrastructure.NotifyBidLost(context.Background(), b)
			}
		}
	}

	// Record metrics
	if s.infrastructure != nil {
		duration := result.EndTime.Sub(result.StartTime)
		s.infrastructure.RecordAuctionDuration(ctx, callID, duration)
		s.infrastructure.RecordBidAmount(ctx, winner.Amount.ToFloat64())
	}

	// Clean up auction after delay
	go func() {
		time.Sleep(5 * time.Minute)
		s.mu.Lock()
		delete(s.auctions, callID)
		s.mu.Unlock()
	}()

	return result, nil
}

// GetAuctionStatus returns current auction state
func (s *auctionOrchestrationService) GetAuctionStatus(ctx context.Context, callID uuid.UUID) (*AuctionStatus, error) {
	s.mu.RLock()
	auction, exists := s.auctions[callID]
	s.mu.RUnlock()

	if !exists {
		return nil, errors.NewNotFoundError("auction not found")
	}

	auction.mu.RLock()
	defer auction.mu.RUnlock()

	// Calculate top bid
	topBidAmount := 0.0
	if len(auction.bids) > 0 {
		// Sort by amount descending
		sorted := make([]*bid.Bid, len(auction.bids))
		copy(sorted, auction.bids)
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].Amount.Compare(sorted[j].Amount) > 0
		})
		topBidAmount = sorted[0].Amount.ToFloat64()
	}

	// Calculate time left
	timeLeft := auction.endTime.Sub(time.Now())
	if timeLeft < 0 {
		timeLeft = 0
	}

	return &AuctionStatus{
		CallID:       callID,
		Status:       auction.status,
		BidCount:     len(auction.bids),
		TopBidAmount: topBidAmount,
		TimeLeft:     timeLeft,
		LastUpdate:   time.Now(),
	}, nil
}

// CloseAuction finalizes the auction
func (s *auctionOrchestrationService) CloseAuction(ctx context.Context, callID uuid.UUID) error {
	s.mu.RLock()
	auction, exists := s.auctions[callID]
	s.mu.RUnlock()

	if !exists {
		return errors.NewNotFoundError("auction not found")
	}

	auction.mu.Lock()
	defer auction.mu.Unlock()

	if auction.status == "closed" {
		return errors.NewValidationError("ALREADY_CLOSED", "auction is already closed")
	}

	auction.status = "closed"
	auction.endTime = time.Now()

	return nil
}

// HandleNewBid processes a new bid in the auction
func (s *auctionOrchestrationService) HandleNewBid(ctx context.Context, b *bid.Bid) error {
	// Get or create auction for the call
	s.mu.Lock()
	auction, exists := s.auctions[b.CallID]
	if !exists {
		// Create new auction if not exists
		auction = &orchestratedAuction{
			callID:    b.CallID,
			status:    "open",
			startTime: time.Now(),
			endTime:   time.Now().Add(30 * time.Second),
			bids:      make([]*bid.Bid, 0),
		}
		s.auctions[b.CallID] = auction
	}
	s.mu.Unlock()

	// Add bid to auction
	auction.mu.Lock()
	defer auction.mu.Unlock()

	if auction.status == "closed" {
		return errors.NewValidationError("AUCTION_CLOSED", "auction is closed")
	}

	// Check if bid already exists
	for _, existingBid := range auction.bids {
		if existingBid.ID == b.ID {
			return nil // Already added
		}
	}

	auction.bids = append(auction.bids, b)

	// Extend auction if near end
	timeLeft := auction.endTime.Sub(time.Now())
	if timeLeft < 10*time.Second {
		auction.endTime = time.Now().Add(15 * time.Second)
	}

	return nil
}

// GetWinningBid returns the current winning bid
func (s *auctionOrchestrationService) GetWinningBid(ctx context.Context, callID uuid.UUID) (*bid.Bid, error) {
	s.mu.RLock()
	auction, exists := s.auctions[callID]
	s.mu.RUnlock()

	if !exists {
		// Check if there's a won bid in the database
		bids, err := s.bidRepo.GetActiveBidsForCall(ctx, callID)
		if err != nil {
			return nil, errors.NewInternalError("failed to get bids").WithCause(err)
		}

		for _, b := range bids {
			if b.Status == bid.StatusWon {
				return b, nil
			}
		}

		return nil, errors.NewNotFoundError("no winning bid found")
	}

	auction.mu.RLock()
	defer auction.mu.RUnlock()

	if auction.winningBidID == uuid.Nil {
		return nil, errors.NewNotFoundError("no winning bid determined yet")
	}

	// Find winning bid
	for _, b := range auction.bids {
		if b.ID == auction.winningBidID {
			return b, nil
		}
	}

	return nil, errors.NewNotFoundError("winning bid not found in auction")
}

// selectWinner determines the winning bid based on amount and quality
func (s *auctionOrchestrationService) selectWinner(bids []*bid.Bid) *bid.Bid {
	if len(bids) == 0 {
		return nil
	}

	// Sort bids by score (combination of amount and quality)
	scored := make([]struct {
		bid   *bid.Bid
		score float64
	}, len(bids))

	for i, b := range bids {
		// Calculate score: 70% amount, 30% quality
		amountScore := b.Amount.ToFloat64()
		qualityScore := b.Quality.HistoricalRating * 10 // Convert 0-1 to 0-10
		score := (amountScore * 0.7) + (qualityScore * 0.3)

		// Apply fraud penalty
		if b.Quality.FraudScore > 0.5 {
			score *= (1 - b.Quality.FraudScore)
		}

		scored[i] = struct {
			bid   *bid.Bid
			score float64
		}{bid: b, score: score}
	}

	// Sort by score descending
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	return scored[0].bid
}

// cleanupExpiredAuctions removes old auctions from memory
func (s *auctionOrchestrationService) cleanupExpiredAuctions() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for callID, auction := range s.auctions {
		auction.mu.RLock()
		expired := auction.status == "closed" && now.Sub(auction.endTime) > 5*time.Minute
		auction.mu.RUnlock()

		if expired {
			delete(s.auctions, callID)
		}
	}
}
