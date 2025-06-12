package bidding

import (
	"context"
	"fmt"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/bid"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
	"github.com/google/uuid"
)

// bidManagementService handles CRUD operations for bids
type bidManagementService struct {
	bidRepo  BidRepository
	callRepo CallRepository
}

// NewBidManagementService creates a new bid management service
func NewBidManagementService(bidRepo BidRepository, callRepo CallRepository) BidManagementService {
	return &bidManagementService{
		bidRepo:  bidRepo,
		callRepo: callRepo,
	}
}

// GetBid retrieves a specific bid
func (s *bidManagementService) GetBid(ctx context.Context, bidID uuid.UUID) (*bid.Bid, error) {
	b, err := s.bidRepo.GetByID(ctx, bidID)
	if err != nil {
		return nil, errors.NewNotFoundError("bid").WithCause(err)
	}
	return b, nil
}

// GetBidsForCall returns all bids for a specific call
func (s *bidManagementService) GetBidsForCall(ctx context.Context, callID uuid.UUID) ([]*bid.Bid, error) {
	// Verify call exists
	if _, err := s.callRepo.GetByID(ctx, callID); err != nil {
		return nil, errors.NewNotFoundError("call").WithCause(err)
	}

	return s.bidRepo.GetActiveBidsForCall(ctx, callID)
}

// GetBidsForBuyer returns all bids for a specific buyer
func (s *bidManagementService) GetBidsForBuyer(ctx context.Context, buyerID uuid.UUID) ([]*bid.Bid, error) {
	return s.bidRepo.GetByBuyer(ctx, buyerID)
}

// ProcessExpiredBids handles bid expiration
func (s *bidManagementService) ProcessExpiredBids(ctx context.Context) error {
	// Get expired bids
	expiredBids, err := s.bidRepo.GetExpiredBids(ctx, time.Now())
	if err != nil {
		return errors.NewInternalError("failed to get expired bids").WithCause(err)
	}

	// Process each expired bid
	for _, b := range expiredBids {
		// Skip if already expired
		if b.Status == bid.StatusExpired {
			continue
		}

		// Update status to expired
		b.Status = bid.StatusExpired

		// Update bid
		if err := s.bidRepo.Update(ctx, b); err != nil {
			// Log error but continue processing other bids
			// In production, this would be logged to monitoring
			continue
		}
	}

	return nil
}

// CreateBid creates a new bid (internal use)
func (s *bidManagementService) CreateBid(ctx context.Context, b *bid.Bid) error {
	// Validate bid has required fields
	if b.CallID == uuid.Nil {
		return errors.NewValidationError("INVALID_CALL_ID", "call ID is required")
	}
	if b.BuyerID == uuid.Nil {
		return errors.NewValidationError("INVALID_BUYER_ID", "buyer ID is required")
	}

	// Verify call exists
	if _, err := s.callRepo.GetByID(ctx, b.CallID); err != nil {
		return errors.NewNotFoundError("call").WithCause(err)
	}

	// Create bid
	if err := s.bidRepo.Create(ctx, b); err != nil {
		return errors.NewInternalError("failed to create bid").WithCause(err)
	}

	return nil
}

// UpdateBid updates an existing bid
func (s *bidManagementService) UpdateBid(ctx context.Context, b *bid.Bid) error {
	// Verify bid exists
	existing, err := s.bidRepo.GetByID(ctx, b.ID)
	if err != nil {
		return errors.NewNotFoundError("bid").WithCause(err)
	}

	// Check if bid is modifiable
	if !s.isBidModifiable(existing) {
		return errors.NewValidationError("INVALID_BID_STATUS",
			fmt.Sprintf("bid cannot be modified in status: %s", existing.Status))
	}

	// Update bid
	if err := s.bidRepo.Update(ctx, b); err != nil {
		return errors.NewInternalError("failed to update bid").WithCause(err)
	}

	return nil
}

// CancelBid cancels a bid
func (s *bidManagementService) CancelBid(ctx context.Context, bidID uuid.UUID) error {
	// Get bid
	b, err := s.bidRepo.GetByID(ctx, bidID)
	if err != nil {
		return errors.NewNotFoundError("bid").WithCause(err)
	}

	// Check if bid can be cancelled
	if !s.isBidCancellable(b) {
		return errors.NewValidationError("INVALID_BID_STATUS",
			fmt.Sprintf("bid cannot be cancelled in status: %s", b.Status))
	}

	// Update status
	b.Status = bid.StatusCanceled

	// Save update
	if err := s.bidRepo.Update(ctx, b); err != nil {
		return errors.NewInternalError("failed to cancel bid").WithCause(err)
	}

	return nil
}

// isBidModifiable checks if a bid can be modified
func (s *bidManagementService) isBidModifiable(b *bid.Bid) bool {
	return b.Status == bid.StatusActive || b.Status == bid.StatusPending
}

// isBidCancellable checks if a bid can be cancelled
func (s *bidManagementService) isBidCancellable(b *bid.Bid) bool {
	return b.Status == bid.StatusActive || b.Status == bid.StatusPending
}
