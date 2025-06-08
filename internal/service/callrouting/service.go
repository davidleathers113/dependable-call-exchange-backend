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
	callRepo    CallRepository
	bidRepo     BidRepository
	accountRepo AccountRepository
	metrics     MetricsCollector
	router      Router
	rules       *RoutingRules
	mu          sync.RWMutex
}

// NewService creates a new call routing service
func NewService(
	callRepo CallRepository,
	bidRepo BidRepository,
	accountRepo AccountRepository,
	metrics MetricsCollector,
	initialRules *RoutingRules,
) Service {
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

// RouteCall processes a call and returns routing decision
func (s *service) RouteCall(ctx context.Context, callID uuid.UUID) (*RoutingDecision, error) {
	start := time.Now()
	
	// Get the call
	c, err := s.callRepo.GetByID(ctx, callID)
	if err != nil {
		return nil, errors.NewAppError(
			errors.ErrCodeNotFound,
			fmt.Sprintf("call not found: %v", err),
			err,
		)
	}

	// Validate call is in correct state
	if c.Status != call.StatusPending {
		return nil, errors.NewAppError(
			errors.ErrCodeValidation,
			fmt.Sprintf("call is not in pending state: %s", c.Status),
			nil,
		)
	}

	// Get active bids for the call
	bids, err := s.bidRepo.GetActiveBidsForCall(ctx, callID)
	if err != nil {
		return nil, errors.NewAppError(
			errors.ErrCodeInternal,
			fmt.Sprintf("failed to get bids: %v", err),
			err,
		)
	}

	if len(bids) == 0 {
		return nil, errors.NewAppError(
			errors.ErrCodeNotFound,
			"no bids available for call",
			nil,
		)
	}

	// Route the call
	s.mu.RLock()
	router := s.router
	s.mu.RUnlock()

	decision, err := router.Route(ctx, c, bids)
	if err != nil {
		return nil, errors.NewAppError(
			errors.ErrCodeInternal,
			fmt.Sprintf("routing failed: %v", err),
			err,
		)
	}

	// Update timing information
	decision.Latency = time.Since(start)

	// Update call with routing information
	c.Status = call.StatusRouted
	c.Metadata["routing_decision"] = map[string]interface{}{
		"bid_id":    decision.BidID.String(),
		"buyer_id":  decision.BuyerID.String(),
		"algorithm": decision.Algorithm,
		"score":     decision.Score,
		"timestamp": decision.Timestamp,
	}
	
	if err := s.callRepo.Update(ctx, c); err != nil {
		return nil, errors.NewAppError(
			errors.ErrCodeInternal,
			fmt.Sprintf("failed to update call: %v", err),
			err,
		)
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
		return errors.NewAppError(
			errors.ErrCodeValidation,
			"rules cannot be nil",
			nil,
		)
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
		// Extract skill weights from rules
		skillWeights := make(map[string]float64)
		if weights, ok := rules.SkillRequirements["weights"].(map[string]float64); ok {
			skillWeights = weights
		}
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