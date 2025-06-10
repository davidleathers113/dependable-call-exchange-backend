package buyer_routing

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/bid"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/call"
)

// BuyerRoutingService routes calls from sellers to buyers based on bids
type BuyerRoutingService interface {
	// RouteCallToBuyer finds the best buyer for a seller's call based on active bids
	RouteCallToBuyer(ctx context.Context, sellerCallID uuid.UUID) (*BuyerRoutingDecision, error)
	
	// GetActiveRoutesForSeller returns all active call routes for a specific seller
	GetActiveRoutesForSeller(ctx context.Context, sellerID uuid.UUID) ([]*ActiveBuyerRoute, error)
	
	// GetActiveRoutesForBuyer returns all calls currently routed to a buyer
	GetActiveRoutesForBuyer(ctx context.Context, buyerID uuid.UUID) ([]*ActiveBuyerRoute, error)
}

// BuyerRoutingDecision represents the decision to route a seller's call to a buyer
type BuyerRoutingDecision struct {
	CallID       uuid.UUID              `json:"call_id"`        // The seller's call being routed
	BidID        uuid.UUID              `json:"bid_id"`         // The winning bid
	BuyerID      uuid.UUID              `json:"buyer_id"`       // The buyer who won
	SellerID     uuid.UUID              `json:"seller_id"`      // The seller who owns the call
	Algorithm    string                 `json:"algorithm"`      // Algorithm used (e.g., "highest-bid", "quality-based")
	Score        float64                `json:"score"`          // Routing score
	Amount       float64                `json:"amount"`         // Winning bid amount
	Reason       string                 `json:"reason"`         // Human-readable reason
	Timestamp    time.Time              `json:"timestamp"`      
	Latency      time.Duration          `json:"latency"`        // Time taken to make decision
	Metadata     map[string]interface{} `json:"metadata"`       // Additional algorithm-specific data
}

// ActiveBuyerRoute represents an active connection between a seller's call and a buyer
type ActiveBuyerRoute struct {
	RouteID      uuid.UUID     `json:"route_id"`
	CallID       uuid.UUID     `json:"call_id"`
	SellerID     uuid.UUID     `json:"seller_id"`      // Seller who owns the call
	BuyerID      uuid.UUID     `json:"buyer_id"`       // Buyer who won the bid
	BidAmount    float64       `json:"bid_amount"`     // Amount buyer is paying
	Status       string        `json:"status"`         // Active, Completed, Failed
	ConnectedAt  time.Time     `json:"connected_at"`
	Duration     *int          `json:"duration,omitempty"`
	Cost         *float64      `json:"cost,omitempty"`
}

// BuyerRouter implements routing algorithms for selecting buyers
type BuyerRouter interface {
	// Route selects the best buyer for a seller's call based on bids
	Route(ctx context.Context, sellerCall *call.Call, buyerBids []*bid.Bid) (*BuyerRoutingDecision, error)
	
	// GetAlgorithm returns the name of the routing algorithm
	GetAlgorithm() string
}

// BuyerRoutingRules defines rules for routing calls to buyers
type BuyerRoutingRules struct {
	Algorithm        string  `json:"algorithm"`         // "highest-bid", "quality-based", "round-robin"
	MinBidAmount     float64 `json:"min_bid_amount"`    // Minimum acceptable bid
	QualityWeight    float64 `json:"quality_weight"`    // Weight for buyer quality (0-1)
	PriceWeight      float64 `json:"price_weight"`      // Weight for bid price (0-1)
	CapacityWeight   float64 `json:"capacity_weight"`   // Weight for buyer capacity (0-1)
	RequireConsent   bool    `json:"require_consent"`   // Require TCPA/GDPR consent
	GeographicMatch  bool    `json:"geographic_match"`  // Prefer geographic proximity
}

// BuyerMetrics tracks buyer performance metrics
type BuyerMetrics interface {
	// RecordBuyerRoutingDecision records a routing decision for analytics
	RecordBuyerRoutingDecision(ctx context.Context, decision *BuyerRoutingDecision) error
	
	// RecordBuyerPerformance tracks buyer performance on routed calls
	RecordBuyerPerformance(ctx context.Context, buyerID uuid.UUID, callID uuid.UUID, metrics map[string]interface{}) error
	
	// GetBuyerQualityScore calculates current quality score for a buyer
	GetBuyerQualityScore(ctx context.Context, buyerID uuid.UUID) (float64, error)
}