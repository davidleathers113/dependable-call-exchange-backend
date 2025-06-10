package instrumentation

import (
	"context"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/infrastructure/telemetry"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/metrics"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/service/callrouting"
	"github.com/google/uuid"
)

// CallRoutingTracedService wraps the call routing service with OpenTelemetry instrumentation
type CallRoutingTracedService struct {
	service callrouting.Service
	tracer  telemetry.TracerInterface
	metrics *metrics.Registry
}

// NewCallRoutingTracedService creates a new instrumented call routing service
func NewCallRoutingTracedService(service callrouting.Service, tracer telemetry.TracerInterface, metrics *metrics.Registry) *CallRoutingTracedService {
	return &CallRoutingTracedService{
		service: service,
		tracer:  tracer,
		metrics: metrics,
	}
}

// RouteCall instruments the call routing operation
func (s *CallRoutingTracedService) RouteCall(ctx context.Context, callID uuid.UUID) (*callrouting.RoutingDecision, error) {
	// Start span for call routing
	ctx, span := s.tracer.StartSpanWithAttributes(ctx, "callrouting.RouteCall", map[string]interface{}{
		"call.id":    callID.String(),
		"span.kind":  "internal",
		"component":  "callrouting",
	})
	defer span.End()

	// Record start time for latency measurement
	startTime := time.Now()

	// Execute the routing
	decision, err := s.service.RouteCall(ctx, callID)
	
	// Calculate latency in microseconds
	latencyUS := float64(time.Since(startTime).Microseconds())

	if err != nil {
		// Record error on span
		s.tracer.RecordError(span, err, "Call routing failed")
		s.tracer.AddEvent(span, "routing_failed", map[string]interface{}{
			"error.type": getErrorType(err),
			"call.id":    callID.String(),
		})
		
		// Record metrics
		s.metrics.RecordCallRouting(ctx, latencyUS, "unknown", false)
		return nil, err
	}

	// Record metrics with algorithm from decision
	s.metrics.RecordCallRouting(ctx, latencyUS, decision.Algorithm, true)

	// Add success attributes
	s.tracer.SetAttributes(span, map[string]interface{}{
		"buyer.id":           decision.BuyerID.String(),
		"bid.id":             decision.BidID.String(),
		"routing.algorithm":  decision.Algorithm,
		"routing.score":      decision.Score,
		"routing.latency_us": latencyUS,
		"routing.success":    true,
	})

	// Add routing event
	s.tracer.AddEvent(span, "call_routed", map[string]interface{}{
		"buyer.id":           decision.BuyerID.String(),
		"routing.latency_us": latencyUS,
		"routing.algorithm":  decision.Algorithm,
	})

	return decision, nil
}

// GetActiveRoutes instruments the active routes retrieval
func (s *CallRoutingTracedService) GetActiveRoutes(ctx context.Context) ([]*callrouting.ActiveRoute, error) {
	ctx, span := s.tracer.StartSpan(ctx, "callrouting.GetActiveRoutes")
	defer span.End()

	routes, err := s.service.GetActiveRoutes(ctx)
	if err != nil {
		s.tracer.RecordError(span, err, "Failed to get active routes")
		return nil, err
	}

	// Add routes summary to span
	if routes != nil {
		s.tracer.SetAttributes(span, map[string]interface{}{
			"routes.count": len(routes),
		})
	}

	return routes, nil
}

// UpdateRoutingRules instruments routing rules updates
func (s *CallRoutingTracedService) UpdateRoutingRules(ctx context.Context, rules *callrouting.RoutingRules) error {
	ctx, span := s.tracer.StartSpanWithAttributes(ctx, "callrouting.UpdateRoutingRules", map[string]interface{}{
		"rules.algorithm":         rules.Algorithm,
		"rules.priority_threshold": rules.PriorityThreshold,
		"span.kind":              "internal",
	})
	defer span.End()

	err := s.service.UpdateRoutingRules(ctx, rules)
	if err != nil {
		s.tracer.RecordError(span, err, "Failed to update routing rules")
		return err
	}

	// Add event for rules update
	s.tracer.AddEvent(span, "routing_rules_updated", map[string]interface{}{
		"algorithm": rules.Algorithm,
		"timestamp": time.Now().Format(time.RFC3339),
	})

	return nil
}

// Helper functions

// getErrorType categorizes errors for better observability
func getErrorType(err error) string {
	if err == nil {
		return ""
	}

	// Check for specific error types
	switch err.Error() {
	case "no available buyers":
		return "no_buyers_available"
	case "all buyers at capacity":
		return "buyers_at_capacity"
	case "no buyers match criteria":
		return "no_matching_buyers"
	case "routing timeout":
		return "timeout"
	default:
		return "unknown"
	}
}

