package bidding

import (
	"context"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/account"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/bid"
	"github.com/google/uuid"
)

// BidManagementService handles CRUD operations for bids
type BidManagementService interface {
	// GetBid retrieves a specific bid
	GetBid(ctx context.Context, bidID uuid.UUID) (*bid.Bid, error)
	// GetBidsForCall returns all bids for a specific call
	GetBidsForCall(ctx context.Context, callID uuid.UUID) ([]*bid.Bid, error)
	// GetBidsForBuyer returns all bids for a specific buyer
	GetBidsForBuyer(ctx context.Context, buyerID uuid.UUID) ([]*bid.Bid, error)
	// ProcessExpiredBids handles bid expiration
	ProcessExpiredBids(ctx context.Context) error
	// CreateBid creates a new bid (internal use)
	CreateBid(ctx context.Context, bid *bid.Bid) error
	// UpdateBid updates an existing bid
	UpdateBid(ctx context.Context, bid *bid.Bid) error
	// CancelBid cancels a bid
	CancelBid(ctx context.Context, bidID uuid.UUID) error
}

// AuctionOrchestrationService manages auction lifecycle
type AuctionOrchestrationService interface {
	// RunAuction executes the auction for a call
	RunAuction(ctx context.Context, callID uuid.UUID) (*AuctionResult, error)
	// GetAuctionStatus returns current auction state
	GetAuctionStatus(ctx context.Context, callID uuid.UUID) (*AuctionStatus, error)
	// CloseAuction finalizes the auction
	CloseAuction(ctx context.Context, callID uuid.UUID) error
	// HandleNewBid processes a new bid in the auction
	HandleNewBid(ctx context.Context, bid *bid.Bid) error
	// GetWinningBid returns the current winning bid
	GetWinningBid(ctx context.Context, callID uuid.UUID) (*bid.Bid, error)
}

// BidValidationService handles business rule validation
type BidValidationService interface {
	// ValidateBidRequest validates a bid placement request
	ValidateBidRequest(ctx context.Context, req *PlaceBidRequest) error
	// ValidateBidAmount checks if bid amount is within allowed range
	ValidateBidAmount(amount float64) error
	// ValidateBuyerEligibility checks if buyer can place bids
	ValidateBuyerEligibility(ctx context.Context, buyer *account.Account) error
	// ValidateBidUpdate validates bid modification request
	ValidateBidUpdate(ctx context.Context, bid *bid.Bid, updates *BidUpdate) error
	// CheckFraud performs fraud detection on bid
	CheckFraud(ctx context.Context, bid *bid.Bid, buyer *account.Account) (*FraudCheckResult, error)
}

// RateLimitService provides generic rate limiting
type RateLimitService interface {
	// CheckRateLimit checks if entity is within rate limit
	CheckRateLimit(ctx context.Context, entityID uuid.UUID, limitType string) error
	// RecordAction records an action for rate limiting
	RecordAction(ctx context.Context, entityID uuid.UUID, limitType string) error
	// GetCurrentCount returns current count for entity
	GetCurrentCount(ctx context.Context, entityID uuid.UUID, limitType string) (int, error)
	// ResetLimit resets rate limit for entity
	ResetLimit(ctx context.Context, entityID uuid.UUID, limitType string) error
	// Configure sets rate limit parameters
	Configure(limitType string, count int, window time.Duration) error
}

// CoordinatorService orchestrates the split services (implements original Service interface)
type CoordinatorService interface {
	Service // Implements the original interface
}

// ServiceConfig holds configuration for the bidding services
type ServiceConfig struct {
	MinBidAmount    float64
	MaxBidAmount    float64
	DefaultDuration time.Duration
	RateLimitCount  int
	RateLimitWindow time.Duration
}