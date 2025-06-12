package buyer_routing

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/call"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
	"github.com/google/uuid"
)

// service implements the BuyerRoutingService interface
// This service routes calls from sellers to buyers based on marketplace bids
type service struct {
	callRepo    CallRepository
	bidRepo     BidRepository
	accountRepo AccountRepository
	metrics     BuyerMetrics
	router      BuyerRouter
	rules       *BuyerRoutingRules
	mu          sync.RWMutex
}

// NewService creates a new buyer routing service for marketplace call routing
func NewService(
	callRepo CallRepository,
	bidRepo BidRepository,
	accountRepo AccountRepository,
	metrics BuyerMetrics,
	initialRules *BuyerRoutingRules,
) BuyerRoutingService {
	// Create router based on initial rules
	router := createRouter(initialRules)

	return &service{
		callRepo:    callRepo,
		bidRepo:     bidRepo,
		accountRepo: accountRepo,
		metrics:     metrics,
		router:      router,
		rules:       initialRules,
	}
}

// RouteCallToBuyer finds the best buyer for a seller's call based on active bids
func (s *service) RouteCallToBuyer(ctx context.Context, sellerCallID uuid.UUID) (*BuyerRoutingDecision, error) {
	start := time.Now()

	// Get the seller's call
	c, err := s.callRepo.GetByID(ctx, sellerCallID)
	if err != nil {
		return nil, errors.NewNotFoundError("seller call").
			WithDetails(map[string]interface{}{"call_id": sellerCallID}).
			WithCause(err)
	}

	// Validate call is in correct state
	if c.Status != call.StatusPending {
		return nil, errors.NewValidationError("INVALID_CALL_STATE",
			fmt.Sprintf("call is not in pending state: %s", c.Status)).
			WithDetails(map[string]interface{}{
				"call_id": sellerCallID,
				"status":  c.Status.String(),
			})
	}

	// Get active bids for the call
	bids, err := s.bidRepo.GetActiveBidsForCall(ctx, sellerCallID)
	if err != nil {
		return nil, errors.NewInternalError("failed to get bids").
			WithCause(err).
			WithDetails(map[string]interface{}{"call_id": sellerCallID})
	}

	if len(bids) == 0 {
		return nil, errors.NewBusinessError("NO_BIDS_AVAILABLE",
			"no bids available for call").
			WithDetails(map[string]interface{}{"call_id": sellerCallID})
	}

	// Route the call
	s.mu.RLock()
	router := s.router
	s.mu.RUnlock()

	decision, err := router.Route(ctx, c, bids)
	if err != nil {
		return nil, errors.NewInternalError("routing failed").
			WithCause(err).
			WithDetails(map[string]interface{}{
				"call_id":   sellerCallID,
				"algorithm": router.GetAlgorithm(),
			})
	}

	// Update timing information
	decision.Latency = time.Since(start)

	// Update call with routing information using status check for concurrency safety
	c.Status = call.StatusQueued
	c.RouteID = &decision.BidID
	c.UpdatedAt = time.Now()

	if err := s.callRepo.UpdateWithStatusCheck(ctx, c, call.StatusPending); err != nil {
		// Check if this is a concurrent update issue
		if err.Error() == fmt.Sprintf("call status has changed, expected %s", call.StatusPending) {
			return nil, errors.NewValidationError("CALL_ALREADY_ROUTED",
				"call has already been routed by another process").
				WithDetails(map[string]interface{}{
					"call_id":         sellerCallID,
					"expected_status": call.StatusPending.String(),
				})
		}
		return nil, errors.NewInternalError("failed to update call").
			WithCause(err).
			WithDetails(map[string]interface{}{"call_id": sellerCallID})
	}

	// Update winning bid status
	winningBid, err := s.bidRepo.GetBidByID(ctx, decision.BidID)
	if err != nil {
		return nil, errors.NewInternalError("failed to get winning bid").
			WithCause(err).
			WithDetails(map[string]interface{}{"bid_id": decision.BidID})
	}

	winningBid.Accept() // This sets status to "won"
	if err := s.bidRepo.Update(ctx, winningBid); err != nil {
		return nil, errors.NewInternalError("failed to update winning bid").
			WithCause(err).
			WithDetails(map[string]interface{}{"bid_id": decision.BidID})
	}

	// Record metrics
	if s.metrics != nil {
		s.metrics.RecordBuyerRoutingDecision(ctx, decision)
		s.metrics.RecordBuyerPerformance(ctx, decision.BuyerID, decision.CallID, map[string]interface{}{
			"algorithm": decision.Algorithm,
			"latency":   decision.Latency,
			"score":     decision.Score,
		})
	}

	return decision, nil
}

// GetActiveRoutesForSeller returns all active call routes for a specific seller
func (s *service) GetActiveRoutesForSeller(ctx context.Context, sellerID uuid.UUID) ([]*ActiveBuyerRoute, error) {
	// This would typically query a database or cache
	// For now, returning empty slice
	return []*ActiveBuyerRoute{}, nil
}

// GetActiveRoutesForBuyer returns all calls currently routed to a buyer
func (s *service) GetActiveRoutesForBuyer(ctx context.Context, buyerID uuid.UUID) ([]*ActiveBuyerRoute, error) {
	// This would typically query a database or cache
	// For now, returning empty slice
	return []*ActiveBuyerRoute{}, nil
}

// UpdateBuyerRoutingRules updates buyer routing configuration
func (s *service) UpdateBuyerRoutingRules(ctx context.Context, rules *BuyerRoutingRules) error {
	if rules == nil {
		return errors.NewValidationError("INVALID_RULES", "rules cannot be nil")
	}

	// Create new router based on new rules
	newRouter := createRouter(rules)

	// Update atomically
	s.mu.Lock()
	s.rules = rules
	s.router = newRouter
	s.mu.Unlock()

	return nil
}

// createRouter creates a router based on the routing rules
func createRouter(rules *BuyerRoutingRules) BuyerRouter {
	if rules == nil {
		return NewRoundRobinBuyerRouter()
	}

	switch rules.Algorithm {
	case "round-robin":
		return NewRoundRobinBuyerRouter()
	case "skill-based":
		// Extract skill weights from rules metadata if available
		skillWeights := make(map[string]float64)
		// In a real implementation, skill weights might come from configuration
		// For now, using default weights
		return NewSkillBasedBuyerRouter(skillWeights)
	case "cost-based":
		return NewCostBasedBuyerRouter(
			rules.QualityWeight,
			rules.PriceWeight,
			rules.CapacityWeight,
		)
	default:
		// Default to round-robin
		return NewRoundRobinBuyerRouter()
	}
}
