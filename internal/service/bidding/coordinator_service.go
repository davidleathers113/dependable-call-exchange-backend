package bidding

import (
	"context"
	"log/slog"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/account"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/bid"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/google/uuid"
)

// coordinatorService orchestrates the split bidding services and implements the original Service interface.
//
// As an orchestrator service, it coordinates multiple subsystems and is allowed up to 8 dependencies
// per ADR-001. Each dependency serves a specific orchestration purpose:
//
// - bidMgmt: Handles CRUD operations for bids
// - validation: Enforces business rules and fraud checks
// - auction: Manages auction lifecycle and winner selection
// - rateLimit: Prevents abuse through request throttling
// - accountRepo: Validates buyer eligibility and balances
// - infrastructure: Combined notification and metrics facade
// - config: Provides service configuration
//
// This reduces dependencies from 8 to 7 through the infrastructure facade pattern.
// The service acts as a facade to maintain backward compatibility while delegating
// to specialized services that each handle a specific concern.
type coordinatorService struct {
	bidMgmt        BidManagementService
	validation     BidValidationService
	auction        AuctionOrchestrationService
	rateLimit      RateLimitService
	accountRepo    AccountRepository
	infrastructure InfrastructureServices // Combines notifier + metrics
	config         *ServiceConfig
}

// NewCoordinatorService creates a new coordinator service that implements the original Service interface
func NewCoordinatorService(
	bidMgmt BidManagementService,
	validation BidValidationService,
	auction AuctionOrchestrationService,
	rateLimit RateLimitService,
	accountRepo AccountRepository,
	infrastructure InfrastructureServices, // Changed from notifier + metrics
	config *ServiceConfig,
) Service {
	// Configure rate limiting
	if rateLimit != nil && config != nil {
		rateLimit.Configure("bid_placement", config.RateLimitCount, config.RateLimitWindow)
	}

	return &coordinatorService{
		bidMgmt:        bidMgmt,
		validation:     validation,
		auction:        auction,
		rateLimit:      rateLimit,
		accountRepo:    accountRepo,
		infrastructure: infrastructure,
		config:         config,
	}
}

// PlaceBid creates a new bid for a call
func (s *coordinatorService) PlaceBid(ctx context.Context, req *PlaceBidRequest) (*bid.Bid, error) {
	// Validate request
	if err := s.validation.ValidateBidRequest(ctx, req); err != nil {
		return nil, err
	}

	// Check rate limit
	if err := s.checkRateLimit(ctx, req.BuyerID); err != nil {
		return nil, err
	}
	defer s.recordRateLimitAction(ctx, req.BuyerID)

	// Validate buyer and balance
	buyer, err := s.validateBuyerAndBalance(ctx, req.BuyerID, req.Amount)
	if err != nil {
		return nil, err
	}

	// Create and validate bid
	newBid, err := s.createBid(req, buyer)
	if err != nil {
		return nil, err
	}

	// Perform fraud check
	if err := s.performFraudCheck(ctx, newBid, buyer); err != nil {
		return nil, err
	}

	// Activate and persist bid
	newBid.Status = bid.StatusActive
	if err := s.bidMgmt.CreateBid(ctx, newBid); err != nil {
		return nil, err
	}

	// Handle post-creation tasks asynchronously
	s.handlePostBidCreation(ctx, newBid)

	return newBid, nil
}

// UpdateBid modifies an existing bid
func (s *coordinatorService) UpdateBid(ctx context.Context, bidID uuid.UUID, updates *BidUpdate) (*bid.Bid, error) {
	// Get existing bid
	existingBid, err := s.bidMgmt.GetBid(ctx, bidID)
	if err != nil {
		return nil, err
	}

	// Validate update
	if err := s.validation.ValidateBidUpdate(ctx, existingBid, updates); err != nil {
		return nil, err
	}

	// Apply updates
	if updates.Amount != nil {
		existingBid.Amount, _ = values.NewMoneyFromFloat(*updates.Amount, "USD")
	}

	if updates.Criteria != nil {
		newCriteria, err := bid.NewBidCriteriaFromMap(updates.Criteria)
		if err != nil {
			return nil, errors.NewValidationError("INVALID_CRITERIA", "failed to parse criteria: "+err.Error())
		}
		existingBid.Criteria = newCriteria
	}

	if updates.ExtendBy != nil {
		existingBid.ExpiresAt = existingBid.ExpiresAt.Add(*updates.ExtendBy)
	}

	// Update bid
	if err := s.bidMgmt.UpdateBid(ctx, existingBid); err != nil {
		return nil, err
	}

	return existingBid, nil
}

// CancelBid cancels an active bid
func (s *coordinatorService) CancelBid(ctx context.Context, bidID uuid.UUID) error {
	return s.bidMgmt.CancelBid(ctx, bidID)
}

// GetBid retrieves a specific bid
func (s *coordinatorService) GetBid(ctx context.Context, bidID uuid.UUID) (*bid.Bid, error) {
	return s.bidMgmt.GetBid(ctx, bidID)
}

// GetBidsForCall returns all bids for a specific call
func (s *coordinatorService) GetBidsForCall(ctx context.Context, callID uuid.UUID) ([]*bid.Bid, error) {
	return s.bidMgmt.GetBidsForCall(ctx, callID)
}

// GetBidsForBuyer returns all bids for a specific buyer
func (s *coordinatorService) GetBidsForBuyer(ctx context.Context, buyerID uuid.UUID) ([]*bid.Bid, error) {
	return s.bidMgmt.GetBidsForBuyer(ctx, buyerID)
}

// ProcessExpiredBids handles bid expiration
func (s *coordinatorService) ProcessExpiredBids(ctx context.Context) error {
	return s.bidMgmt.ProcessExpiredBids(ctx)
}

// Helper methods for PlaceBid

func (s *coordinatorService) checkRateLimit(ctx context.Context, buyerID uuid.UUID) error {
	if s.rateLimit == nil {
		return nil
	}
	return s.rateLimit.CheckRateLimit(ctx, buyerID, "bid_placement")
}

func (s *coordinatorService) recordRateLimitAction(ctx context.Context, buyerID uuid.UUID) {
	if s.rateLimit != nil {
		s.rateLimit.RecordAction(ctx, buyerID, "bid_placement")
	}
}

func (s *coordinatorService) validateBuyerAndBalance(ctx context.Context, buyerID uuid.UUID, amount float64) (*account.Account, error) {
	// Get buyer account
	buyer, err := s.accountRepo.GetByID(ctx, buyerID)
	if err != nil {
		return nil, errors.NewNotFoundError("buyer").WithCause(err)
	}

	// Validate eligibility
	if err := s.validation.ValidateBuyerEligibility(ctx, buyer); err != nil {
		return nil, err
	}

	// Check balance
	balance, err := s.accountRepo.GetBalance(ctx, buyerID)
	if err != nil {
		return nil, errors.NewInternalError("failed to get balance").WithCause(err)
	}

	if balance < amount {
		return nil, errors.NewValidationError("INSUFFICIENT_BALANCE", "insufficient balance")
	}

	return buyer, nil
}

func (s *coordinatorService) createBid(req *PlaceBidRequest, buyer *account.Account) (*bid.Bid, error) {
	// Determine duration
	duration := req.Duration
	if duration == 0 && s.config != nil {
		duration = s.config.DefaultDuration
	}

	now := time.Now()
	newBid := &bid.Bid{
		ID:      uuid.New(),
		CallID:  req.CallID,
		BuyerID: req.BuyerID,
		Amount:  values.MustNewMoneyFromFloat(req.Amount, "USD"),
		Status:  bid.StatusPending,
		Quality: values.QualityMetrics{
			HistoricalRating: buyer.QualityMetrics.OverallScore(),
			FraudScore:       buyer.QualityMetrics.FraudScore,
		},
		PlacedAt:  now,
		ExpiresAt: now.Add(duration),
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Convert criteria if provided
	if req.Criteria != nil {
		criteria, err := bid.NewBidCriteriaFromMap(req.Criteria)
		if err != nil {
			return nil, errors.NewValidationError("INVALID_CRITERIA", "failed to parse criteria: "+err.Error())
		}

		// Set max budget from request if not already set in criteria
		if criteria.MaxBudget.IsZero() && req.MaxAmount > 0 {
			criteria.MaxBudget = values.MustNewMoneyFromFloat(req.MaxAmount, "USD")
		}

		newBid.Criteria = criteria
	} else if req.MaxAmount > 0 {
		// Set basic criteria with max budget only
		newBid.Criteria = bid.BidCriteria{
			MaxBudget: values.MustNewMoneyFromFloat(req.MaxAmount, "USD"),
		}
	}

	return newBid, nil
}

func (s *coordinatorService) performFraudCheck(ctx context.Context, b *bid.Bid, buyer *account.Account) error {
	fraudResult, err := s.validation.CheckFraud(ctx, b, buyer)
	if err != nil {
		return err
	}

	if !fraudResult.Approved {
		return errors.NewForbiddenError("bid rejected by fraud check")
	}

	return nil
}

func (s *coordinatorService) handlePostBidCreation(ctx context.Context, b *bid.Bid) {
	// Update auction asynchronously
	if s.auction != nil {
		go func() {
			if err := s.auction.HandleNewBid(context.Background(), b); err != nil {
				slog.Error("failed to handle new bid in auction",
					"error", err,
					"bid_id", b.ID,
					"call_id", b.CallID,
					"buyer_id", b.BuyerID,
					"amount", b.Amount.ToFloat64())
			}
		}()
	}

	// Send notification asynchronously
	if s.infrastructure != nil {
		go s.infrastructure.NotifyBidPlaced(context.Background(), b)
	}

	// Record metrics synchronously (fast operation)
	if s.infrastructure != nil {
		s.infrastructure.RecordBidPlaced(ctx, b)
		s.infrastructure.RecordBidAmount(ctx, b.Amount.ToFloat64())
	}
}
