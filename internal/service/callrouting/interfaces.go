package callrouting

import (
	"context"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/account"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/bid"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/call"
	"github.com/google/uuid"
)

// Router defines the interface for call routing strategies
type Router interface {
	// Route determines the best buyer for a call based on the routing algorithm
	Route(ctx context.Context, call *call.Call, bids []*bid.Bid) (*RoutingDecision, error)
	// GetAlgorithm returns the name of the routing algorithm
	GetAlgorithm() string
}

// Service defines the call routing service interface
type Service interface {
	// RouteCall processes a call and returns routing decision
	RouteCall(ctx context.Context, callID uuid.UUID) (*RoutingDecision, error)
	// GetActiveRoutes returns currently active routes
	GetActiveRoutes(ctx context.Context) ([]*ActiveRoute, error)
	// UpdateRoutingRules updates routing configuration
	UpdateRoutingRules(ctx context.Context, rules *RoutingRules) error
}

// BidRepository defines the interface for bid storage
type BidRepository interface {
	// GetActiveBidsForCall returns all active bids for a specific call
	GetActiveBidsForCall(ctx context.Context, callID uuid.UUID) ([]*bid.Bid, error)
	// GetBidByID retrieves a specific bid
	GetBidByID(ctx context.Context, bidID uuid.UUID) (*bid.Bid, error)
	// Update modifies an existing bid
	Update(ctx context.Context, bid *bid.Bid) error
}

// CallRepository defines the interface for call storage
type CallRepository interface {
	// GetByID retrieves a call by ID
	GetByID(ctx context.Context, callID uuid.UUID) (*call.Call, error)
	// Update updates a call
	Update(ctx context.Context, call *call.Call) error
}

// AccountRepository defines the interface for account storage
type AccountRepository interface {
	// GetByID retrieves an account by ID
	GetByID(ctx context.Context, accountID uuid.UUID) (*account.Account, error)
	// UpdateQualityScore updates an account's quality score
	UpdateQualityScore(ctx context.Context, accountID uuid.UUID, score float64) error
}

// ConsentService defines the interface for consent checking
type ConsentService interface {
	// CheckConsent verifies if consent exists for a phone number
	CheckConsent(ctx context.Context, phoneNumber string, consentType string) (bool, error)
}

// MetricsCollector defines the interface for collecting routing metrics
type MetricsCollector interface {
	// RecordRoutingDecision records a routing decision
	RecordRoutingDecision(ctx context.Context, decision *RoutingDecision)
	// RecordRoutingLatency records routing decision latency
	RecordRoutingLatency(ctx context.Context, algorithm string, latency time.Duration)
}

// RoutingDecision represents the result of a routing decision
type RoutingDecision struct {
	CallID    uuid.UUID
	BidID     uuid.UUID
	BuyerID   uuid.UUID
	Algorithm string
	Score     float64
	Reason    string
	Timestamp time.Time
	Latency   time.Duration
	Metadata  map[string]interface{}
}

// ActiveRoute represents a currently active call route
type ActiveRoute struct {
	CallID    uuid.UUID
	BuyerID   uuid.UUID
	StartTime time.Time
	Duration  time.Duration
	Status    string
}

// RoutingRules defines routing configuration
type RoutingRules struct {
	Algorithm         string
	PriorityThreshold float64
	QualityWeight     float64
	PriceWeight       float64
	CapacityWeight    float64
	GeographicRules   map[string][]string
	TimeBasedRules    []TimeRule
	SkillRequirements map[string][]string
}

// TimeRule defines time-based routing rules
type TimeRule struct {
	StartTime  string
	EndTime    string
	DaysOfWeek []string
	Algorithm  string
	Modifiers  map[string]float64
}
