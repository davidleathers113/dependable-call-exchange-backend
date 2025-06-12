package bidding

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/account"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/bid"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/call"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/google/uuid"
)

// service implements the Service interface
type service struct {
	bidRepo      BidRepository
	callRepo     CallRepository
	accountRepo  AccountRepository
	fraudChecker FraudChecker
	notifier     NotificationService
	metrics      MetricsCollector
	auction      AuctionEngine

	// Configuration
	minBidAmount    float64
	maxBidAmount    float64
	defaultDuration time.Duration

	// Rate limiting
	mu        sync.RWMutex
	bidLimits map[uuid.UUID]*rateLimiter
}

// rateLimiter tracks bid rate limits per buyer
type rateLimiter struct {
	count       int
	windowStart time.Time
	mu          sync.Mutex
}

// NewService creates a new bidding service
func NewService(
	bidRepo BidRepository,
	callRepo CallRepository,
	accountRepo AccountRepository,
	fraudChecker FraudChecker,
	notifier NotificationService,
	metrics MetricsCollector,
) Service {
	auction := NewAuctionEngine(bidRepo, callRepo, notifier, metrics)

	return &service{
		bidRepo:         bidRepo,
		callRepo:        callRepo,
		accountRepo:     accountRepo,
		fraudChecker:    fraudChecker,
		notifier:        notifier,
		metrics:         metrics,
		auction:         auction,
		minBidAmount:    0.01,
		maxBidAmount:    1000.0,
		defaultDuration: 5 * time.Minute,
		bidLimits:       make(map[uuid.UUID]*rateLimiter),
	}
}

// PlaceBid creates a new bid for a call
func (s *service) PlaceBid(ctx context.Context, req *PlaceBidRequest) (*bid.Bid, error) {
	// Validate request
	if err := s.validateBidRequest(req); err != nil {
		return nil, err
	}

	// Check rate limit
	if err := s.checkRateLimit(req.BuyerID); err != nil {
		return nil, err
	}

	// Get call
	c, err := s.callRepo.GetByID(ctx, req.CallID)
	if err != nil {
		return nil, errors.NewNotFoundError("call").WithCause(err)
	}

	// Validate call state
	if c.Status != call.StatusPending && c.Status != call.StatusQueued {
		return nil, errors.NewValidationError("INVALID_CALL_STATE", fmt.Sprintf("call not in biddable state: %s", c.Status))
	}

	// Get buyer account
	buyer, err := s.accountRepo.GetByID(ctx, req.BuyerID)
	if err != nil {
		return nil, errors.NewNotFoundError("buyer").WithCause(err)
	}

	// Check buyer status
	if buyer.Status != account.StatusActive {
		return nil, errors.NewForbiddenError("buyer account is not active")
	}

	// Check balance
	balance, err := s.accountRepo.GetBalance(ctx, req.BuyerID)
	if err != nil {
		return nil, errors.NewInternalError("failed to get balance").WithCause(err)
	}

	if balance < req.Amount {
		return nil, errors.NewValidationError("INSUFFICIENT_BALANCE", "insufficient balance")
	}

	// Create bid
	duration := req.Duration
	if duration == 0 {
		duration = s.defaultDuration
	}

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
		PlacedAt:  time.Now(),
		ExpiresAt: time.Now().Add(duration),
		// Note: Criteria conversion needed from map[string]any to bid.BidCriteria
		// Note: Metadata field doesn't exist in bid.Bid domain model
	}

	// Fraud check
	if s.fraudChecker != nil {
		fraudResult, err := s.fraudChecker.CheckBid(ctx, newBid, buyer)
		if err != nil {
			return nil, errors.NewInternalError("fraud check failed").WithCause(err)
		}

		if !fraudResult.Approved {
			return nil, errors.NewForbiddenError(fmt.Sprintf("bid rejected: %v", fraudResult.Reasons))
		}

		// Note: Fraud score and flags would need to be stored in a different way
		// since bid.Bid doesn't have a Metadata field
	}

	// Set bid status to active
	newBid.Status = bid.StatusActive

	// Save bid
	if err := s.bidRepo.Create(ctx, newBid); err != nil {
		return nil, errors.NewInternalError("failed to create bid").WithCause(err)
	}

	// Start or update auction
	if auctionEngine, ok := s.auction.(*auctionEngine); ok {
		if err := auctionEngine.HandleNewBid(ctx, newBid); err != nil {
			// Log error but don't fail bid placement
		}
	}

	// Send notification
	if s.notifier != nil {
		go s.notifier.NotifyBidPlaced(context.Background(), newBid)
	}

	// Record metrics
	if s.metrics != nil {
		s.metrics.RecordBidPlaced(ctx, newBid)
		s.metrics.RecordBidAmount(ctx, newBid.Amount.ToFloat64())
	}

	return newBid, nil
}

// UpdateBid modifies an existing bid
func (s *service) UpdateBid(ctx context.Context, bidID uuid.UUID, updates *BidUpdate) (*bid.Bid, error) {
	// Get existing bid
	existingBid, err := s.bidRepo.GetByID(ctx, bidID)
	if err != nil {
		return nil, errors.NewNotFoundError("bid").WithCause(err)
	}

	// Check if bid is modifiable
	if existingBid.Status != bid.StatusActive && existingBid.Status != bid.StatusPending {
		return nil, errors.NewValidationError("INVALID_BID_STATUS", fmt.Sprintf("bid cannot be modified in status: %s", existingBid.Status))
	}

	// Apply updates
	if updates.Amount != nil {
		if *updates.Amount < s.minBidAmount || *updates.Amount > s.maxBidAmount {
			return nil, errors.NewValidationError("INVALID_BID_AMOUNT", fmt.Sprintf("bid amount must be between %.2f and %.2f", s.minBidAmount, s.maxBidAmount))
		}
		existingBid.Amount, _ = values.NewMoneyFromFloat(*updates.Amount, "USD")
	}

	if updates.Criteria != nil {
		// Note: Type conversion needed from map[string]any to bid.BidCriteria
		// existingBid.Criteria = updates.Criteria
	}

	if updates.ExtendBy != nil {
		existingBid.ExpiresAt = existingBid.ExpiresAt.Add(*updates.ExtendBy)
	}

	if updates.AutoRenew != nil {
		// Note: Metadata field doesn't exist in bid.Bid
		// existingBid.Metadata["auto_renew"] = *updates.AutoRenew
	}

	if updates.MaxAmount != nil {
		// Note: Metadata field doesn't exist in bid.Bid
		// existingBid.Metadata["max_amount"] = *updates.MaxAmount
	}

	// Update bid
	if err := s.bidRepo.Update(ctx, existingBid); err != nil {
		return nil, errors.NewInternalError("failed to update bid").WithCause(err)
	}

	return existingBid, nil
}

// CancelBid cancels an active bid
func (s *service) CancelBid(ctx context.Context, bidID uuid.UUID) error {
	// Get bid
	b, err := s.bidRepo.GetByID(ctx, bidID)
	if err != nil {
		return errors.NewNotFoundError("bid").WithCause(err)
	}

	// Check if bid can be cancelled
	if b.Status != bid.StatusActive && b.Status != bid.StatusPending {
		return errors.NewValidationError("INVALID_BID_STATUS", fmt.Sprintf("bid cannot be cancelled in status: %s", b.Status))
	}

	// Cancel bid
	b.Status = bid.StatusCanceled

	// Update bid
	if err := s.bidRepo.Update(ctx, b); err != nil {
		return errors.NewInternalError("failed to update bid").WithCause(err)
	}

	return nil
}

// GetBid retrieves a specific bid
func (s *service) GetBid(ctx context.Context, bidID uuid.UUID) (*bid.Bid, error) {
	b, err := s.bidRepo.GetByID(ctx, bidID)
	if err != nil {
		return nil, errors.NewNotFoundError("bid").WithCause(err)
	}
	return b, nil
}

// GetBidsForCall returns all bids for a specific call
func (s *service) GetBidsForCall(ctx context.Context, callID uuid.UUID) ([]*bid.Bid, error) {
	bids, err := s.bidRepo.GetActiveBidsForCall(ctx, callID)
	if err != nil {
		return nil, errors.NewInternalError("failed to get bids").WithCause(err)
	}
	return bids, nil
}

// GetBidsForBuyer returns all bids for a specific buyer
func (s *service) GetBidsForBuyer(ctx context.Context, buyerID uuid.UUID) ([]*bid.Bid, error) {
	bids, err := s.bidRepo.GetByBuyer(ctx, buyerID)
	if err != nil {
		return nil, errors.NewInternalError("failed to get bids").WithCause(err)
	}
	return bids, nil
}

// ProcessExpiredBids handles bid expiration
func (s *service) ProcessExpiredBids(ctx context.Context) error {
	// Get expired bids
	expiredBids, err := s.bidRepo.GetExpiredBids(ctx, time.Now())
	if err != nil {
		return errors.NewInternalError("failed to get expired bids").WithCause(err)
	}

	// Process each expired bid
	for _, b := range expiredBids {
		// Check if auto-renew is enabled
		// Note: Metadata field doesn't exist in bid.Bid
		// Would need to implement auto-renew logic differently
		if false { // Placeholder for auto-renew logic
			// Extend bid
			b.ExpiresAt = time.Now().Add(s.defaultDuration)
			if err := s.bidRepo.Update(ctx, b); err != nil {
				// Log error but continue
				continue
			}
		} else {
			// Expire bid
			b.Status = bid.StatusExpired
			if err := s.bidRepo.Update(ctx, b); err != nil {
				// Log error but continue
				continue
			}

			// Send notification
			if s.notifier != nil {
				go s.notifier.NotifyBidExpired(context.Background(), b)
			}
		}
	}

	return nil
}

// validateBidRequest validates bid placement request
func (s *service) validateBidRequest(req *PlaceBidRequest) error {
	if req.CallID == uuid.Nil {
		return errors.NewValidationError("MISSING_CALL_ID", "call ID is required")
	}

	if req.BuyerID == uuid.Nil {
		return errors.NewValidationError("MISSING_BUYER_ID", "buyer ID is required")
	}

	if req.Amount < s.minBidAmount || req.Amount > s.maxBidAmount {
		return errors.NewValidationError("INVALID_BID_AMOUNT", fmt.Sprintf("bid amount must be between %.2f and %.2f", s.minBidAmount, s.maxBidAmount))
	}

	if req.MaxAmount > 0 && req.MaxAmount < req.Amount {
		return errors.NewValidationError("INVALID_MAX_AMOUNT", "max amount must be greater than or equal to bid amount")
	}

	return nil
}

// checkRateLimit checks if buyer has exceeded bid rate limit
func (s *service) checkRateLimit(buyerID uuid.UUID) error {
	s.mu.Lock()
	limiter, exists := s.bidLimits[buyerID]
	if !exists {
		limiter = &rateLimiter{
			windowStart: time.Now(),
		}
		s.bidLimits[buyerID] = limiter
	}
	s.mu.Unlock()

	limiter.mu.Lock()
	defer limiter.mu.Unlock()

	// Reset window if expired (5 minute window)
	if time.Since(limiter.windowStart) > 5*time.Minute {
		limiter.count = 0
		limiter.windowStart = time.Now()
	}

	// Check limit (100 bids per 5 minutes)
	if limiter.count >= 100 {
		return errors.NewBusinessError("RATE_LIMIT_EXCEEDED", "bid rate limit exceeded")
	}

	limiter.count++
	return nil
}
