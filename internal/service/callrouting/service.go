package callrouting

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/call"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
	"github.com/google/uuid"
)

// service implements the Service interface
type service struct {
	callRepo       CallRepository
	bidRepo        BidRepository
	accountRepo    AccountRepository
	consentService ConsentService
	metrics        MetricsCollector
	router         Router
	rules          *RoutingRules
	mu             sync.RWMutex
}

// NewService creates a new call routing service
func NewService(
	callRepo CallRepository,
	bidRepo BidRepository,
	accountRepo AccountRepository,
	consentService ConsentService,
	metrics MetricsCollector,
	initialRules *RoutingRules,
) Service {
	// Create router based on initial rules
	router := createRouter(initialRules)

	return &service{
		callRepo:       callRepo,
		bidRepo:        bidRepo,
		accountRepo:    accountRepo,
		consentService: consentService,
		metrics:        metrics,
		router:         router,
		rules:          initialRules,
	}
}

// RouteCall processes a call and returns routing decision
func (s *service) RouteCall(ctx context.Context, callID uuid.UUID) (*RoutingDecision, error) {
	start := time.Now()

	// Get the call
	c, err := s.callRepo.GetByID(ctx, callID)
	if err != nil {
		return nil, errors.NewNotFoundError("call").
			WithDetails(map[string]interface{}{"call_id": callID}).
			WithCause(err)
	}

	// Validate call is in correct state
	if c.Status != call.StatusPending {
		return nil, errors.NewValidationError("INVALID_CALL_STATE",
			fmt.Sprintf("call is not in pending state: %s", c.Status)).
			WithDetails(map[string]interface{}{
				"call_id": callID,
				"status":  c.Status.String(),
			})
	}

	// Check consent for the call
	// For inbound calls, check if the caller has given consent
	// For outbound calls, check if the callee has given consent
	phoneNumberToCheck := ""
	if c.Direction == call.DirectionInbound {
		phoneNumberToCheck = c.FromNumber.String()
	} else {
		phoneNumberToCheck = c.ToNumber.String()
	}

	hasConsent, err := s.consentService.CheckConsent(ctx, phoneNumberToCheck, "CALL")
	if err != nil {
		return nil, errors.NewInternalError("failed to check consent").
			WithCause(err).
			WithDetails(map[string]interface{}{
				"call_id":      callID,
				"phone_number": phoneNumberToCheck,
			})
	}

	if !hasConsent {
		return nil, errors.NewComplianceError("NO_CONSENT",
			"no consent found for phone number").
			WithDetails(map[string]interface{}{
				"call_id":      callID,
				"phone_number": phoneNumberToCheck,
				"direction":    c.Direction.String(),
			})
	}

	// Get active bids for the call
	bids, err := s.bidRepo.GetActiveBidsForCall(ctx, callID)
	if err != nil {
		return nil, errors.NewInternalError("failed to get bids").
			WithCause(err).
			WithDetails(map[string]interface{}{"call_id": callID})
	}

	if len(bids) == 0 {
		return nil, errors.NewBusinessError("NO_BIDS_AVAILABLE",
			"no bids available for call").
			WithDetails(map[string]interface{}{"call_id": callID})
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
				"call_id":   callID,
				"algorithm": router.GetAlgorithm(),
			})
	}

	// Update timing information
	decision.Latency = time.Since(start)

	// Update call with routing information
	c.Status = call.StatusQueued
	c.RouteID = &decision.BidID
	c.UpdatedAt = time.Now()

	if err := s.callRepo.Update(ctx, c); err != nil {
		return nil, errors.NewInternalError("failed to update call").
			WithCause(err).
			WithDetails(map[string]interface{}{"call_id": callID})
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
		s.metrics.RecordRoutingDecision(ctx, decision)
		s.metrics.RecordRoutingLatency(ctx, decision.Algorithm, decision.Latency)
	}

	return decision, nil
}

// GetActiveRoutes returns currently active routes
func (s *service) GetActiveRoutes(ctx context.Context) ([]*ActiveRoute, error) {
	// This would typically query a database or cache
	// For now, returning empty slice
	return []*ActiveRoute{}, nil
}

// UpdateRoutingRules updates routing configuration
func (s *service) UpdateRoutingRules(ctx context.Context, rules *RoutingRules) error {
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
func createRouter(rules *RoutingRules) Router {
	if rules == nil {
		return NewRoundRobinRouter()
	}

	switch rules.Algorithm {
	case "round-robin":
		return NewRoundRobinRouter()
	case "skill-based":
		// Extract skill weights from rules metadata if available
		skillWeights := make(map[string]float64)
		// In a real implementation, skill weights might come from configuration
		// For now, using default weights
		return NewSkillBasedRouter(skillWeights)
	case "cost-based":
		return NewCostBasedRouter(
			rules.QualityWeight,
			rules.PriceWeight,
			rules.CapacityWeight,
		)
	default:
		// Default to round-robin
		return NewRoundRobinRouter()
	}
}
