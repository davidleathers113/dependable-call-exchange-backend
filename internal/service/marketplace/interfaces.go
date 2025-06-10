package marketplace

import (
	"context"
	"time"

	"github.com/google/uuid"
	callpkg "github.com/davidleathers/dependable-call-exchange-backend/internal/domain/call"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/bid"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/account"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/service/buyer_routing"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/service/seller_distribution"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/service/bidding"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/service/fraud"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/service/telephony"
)

// MarketplaceOrchestrator defines the interface for coordinating the complete marketplace flow
type MarketplaceOrchestrator interface {
	// ProcessIncomingCall handles a new call entering the marketplace system
	ProcessIncomingCall(ctx context.Context, request *IncomingCallRequest) (*CallProcessingResult, error)
	
	// ProcessSellerCall handles a call from a seller for distribution
	ProcessSellerCall(ctx context.Context, callID uuid.UUID) (*SellerCallResult, error)
	
	// ProcessBuyerBid handles a new bid from a buyer
	ProcessBuyerBid(ctx context.Context, request *BidRequest) (*BidProcessingResult, error)
	
	// ExecuteCallRouting performs the complete call routing workflow
	ExecuteCallRouting(ctx context.Context, callID uuid.UUID) (*RoutingResult, error)
	
	// HandleAuctionCompletion processes auction results and assigns calls
	HandleAuctionCompletion(ctx context.Context, auctionID uuid.UUID) (*AuctionResult, error)
	
	// GetMarketplaceStatus returns current marketplace status and metrics
	GetMarketplaceStatus(ctx context.Context) (*MarketplaceStatus, error)
}

// IncomingCallRequest represents a new call entering the system
type IncomingCallRequest struct {
	FromNumber   string                 `json:"from_number"`
	ToNumber     string                 `json:"to_number"`
	Direction    callpkg.Direction         `json:"direction"`
	SellerID     *uuid.UUID             `json:"seller_id,omitempty"`     // For seller-originated calls
	BuyerID      *uuid.UUID             `json:"buyer_id,omitempty"`      // For buyer-originated calls
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	Priority     CallPriority           `json:"priority"`
	RequiredSkills []string             `json:"required_skills,omitempty"`
}

// CallProcessingResult represents the outcome of processing an incoming call
type CallProcessingResult struct {
	CallID           uuid.UUID                                    `json:"call_id"`
	Status           CallProcessingStatus                         `json:"status"`
	ProcessingPath   ProcessingPath                               `json:"processing_path"`
	SellerDecision   *seller_distribution.SellerDistributionDecision `json:"seller_decision,omitempty"`
	BuyerDecision    *buyer_routing.BuyerRoutingDecision          `json:"buyer_decision,omitempty"`
	AuctionID        *uuid.UUID                                   `json:"auction_id,omitempty"`
	EstimatedDelay   time.Duration                                `json:"estimated_delay"`
	ProcessedAt      time.Time                                    `json:"processed_at"`
	Errors           []ProcessingError                            `json:"errors,omitempty"`
	Metadata         map[string]interface{}                       `json:"metadata,omitempty"`
}

// SellerCallResult represents the outcome of processing a seller call
type SellerCallResult struct {
	CallID          uuid.UUID                                    `json:"call_id"`
	DistributionResult *seller_distribution.SellerDistributionDecision `json:"distribution_result"`
	AuctionStarted  bool                                         `json:"auction_started"`
	AuctionID       *uuid.UUID                                   `json:"auction_id,omitempty"`
	NotifiedBuyers  []uuid.UUID                                  `json:"notified_buyers"`
	EstimatedDuration time.Duration                              `json:"estimated_duration"`
	Status          string                                       `json:"status"`
	ProcessedAt     time.Time                                    `json:"processed_at"`
}

// BidRequest represents a bid submission from a buyer
type BidRequest struct {
	CallID     uuid.UUID              `json:"call_id"`
	BuyerID    uuid.UUID              `json:"buyer_id"`
	Amount     float64                `json:"amount"`
	Currency   string                 `json:"currency"`
	Criteria   bid.BidCriteria        `json:"criteria"`
	Quality    values.QualityMetrics  `json:"quality"`
	ExpiresAt  time.Time              `json:"expires_at"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// BidProcessingResult represents the outcome of processing a bid
type BidProcessingResult struct {
	BidID          uuid.UUID                           `json:"bid_id"`
	Status         BidProcessingStatus                 `json:"status"`
	FraudCheck     *fraud.FraudCheckResult             `json:"fraud_check,omitempty"`
	RoutingDecision *buyer_routing.BuyerRoutingDecision `json:"routing_decision,omitempty"`
	Rank           int                                 `json:"rank"`
	IsWinning      bool                                `json:"is_winning"`
	ProcessedAt    time.Time                           `json:"processed_at"`
	Errors         []ProcessingError                   `json:"errors,omitempty"`
}

// RoutingResult represents the complete routing decision
type RoutingResult struct {
	CallID           uuid.UUID                           `json:"call_id"`
	SelectedBuyerID  *uuid.UUID                          `json:"selected_buyer_id,omitempty"`
	SelectedBid      *bid.Bid                            `json:"selected_bid,omitempty"`
	RoutingDecision  *buyer_routing.BuyerRoutingDecision `json:"routing_decision"`
	TelephonyResult  *telephony.CallResponse             `json:"telephony_result,omitempty"`
	FinalStatus      callpkg.Status                         `json:"final_status"`
	ProcessingTime   time.Duration                       `json:"processing_time"`
	CompletedAt      time.Time                           `json:"completed_at"`
	QualityMetrics   *RoutingQualityMetrics              `json:"quality_metrics,omitempty"`
}

// AuctionResult represents the outcome of an auction
type AuctionResult struct {
	AuctionID       uuid.UUID                 `json:"auction_id"`
	CallID          uuid.UUID                 `json:"call_id"`
	WinningBid      *bid.Bid                  `json:"winning_bid,omitempty"`
	WinningBuyerID  *uuid.UUID                `json:"winning_buyer_id,omitempty"`
	TotalBids       int                       `json:"total_bids"`
	AuctionDuration time.Duration             `json:"auction_duration"`
	FinalPrice      float64                   `json:"final_price"`
	Currency        string                    `json:"currency"`
	Status          AuctionStatus             `json:"status"`
	CompletedAt     time.Time                 `json:"completed_at"`
	Metrics         *AuctionMetrics           `json:"metrics,omitempty"`
}

// MarketplaceStatus represents current marketplace state
type MarketplaceStatus struct {
	ActiveCalls        int                    `json:"active_calls"`
	PendingAuctions    int                    `json:"pending_auctions"`
	ActiveBuyers       int                    `json:"active_buyers"`
	ActiveSellers      int                    `json:"active_sellers"`
	AverageProcessingTime time.Duration       `json:"average_processing_time"`
	SuccessRate        float64               `json:"success_rate"`
	LastUpdated        time.Time             `json:"last_updated"`
	SystemHealth       SystemHealthStatus    `json:"system_health"`
	Metrics            *MarketplaceMetrics   `json:"metrics,omitempty"`
}

// Supporting types and enums

type CallPriority int

const (
	PriorityLow CallPriority = iota
	PriorityNormal
	PriorityHigh
	PriorityCritical
)

func (p CallPriority) String() string {
	switch p {
	case PriorityLow:
		return "low"
	case PriorityNormal:
		return "normal"
	case PriorityHigh:
		return "high"
	case PriorityCritical:
		return "critical"
	default:
		return "normal"
	}
}

type CallProcessingStatus int

const (
	ProcessingStatusAccepted CallProcessingStatus = iota
	ProcessingStatusRouting
	ProcessingStatusAuction
	ProcessingStatusAssigned
	ProcessingStatusFailed
	ProcessingStatusRejected
)

func (s CallProcessingStatus) String() string {
	switch s {
	case ProcessingStatusAccepted:
		return "accepted"
	case ProcessingStatusRouting:
		return "routing"
	case ProcessingStatusAuction:
		return "auction"
	case ProcessingStatusAssigned:
		return "assigned"
	case ProcessingStatusFailed:
		return "failed"
	case ProcessingStatusRejected:
		return "rejected"
	default:
		return "unknown"
	}
}

type ProcessingPath int

const (
	PathDirectAssignment ProcessingPath = iota  // Call directly assigned to buyer
	PathSellerDistribution                      // Call distributed to sellers first
	PathAuction                                // Call goes through auction process
	PathFailover                               // Backup routing path
)

func (p ProcessingPath) String() string {
	switch p {
	case PathDirectAssignment:
		return "direct_assignment"
	case PathSellerDistribution:
		return "seller_distribution"
	case PathAuction:
		return "auction"
	case PathFailover:
		return "failover"
	default:
		return "unknown"
	}
}

type BidProcessingStatus int

const (
	BidStatusAccepted BidProcessingStatus = iota
	BidStatusRejected
	BidStatusWinning
	BidStatusLost
	BidStatusFraudulent
)

func (s BidProcessingStatus) String() string {
	switch s {
	case BidStatusAccepted:
		return "accepted"
	case BidStatusRejected:
		return "rejected"
	case BidStatusWinning:
		return "winning"
	case BidStatusLost:
		return "lost"
	case BidStatusFraudulent:
		return "fraudulent"
	default:
		return "unknown"
	}
}

type AuctionStatus int

const (
	AuctionStatusActive AuctionStatus = iota
	AuctionStatusCompleted
	AuctionStatusCanceled
	AuctionStatusExpired
)

func (s AuctionStatus) String() string {
	switch s {
	case AuctionStatusActive:
		return "active"
	case AuctionStatusCompleted:
		return "completed"
	case AuctionStatusCanceled:
		return "canceled"
	case AuctionStatusExpired:
		return "expired"
	default:
		return "unknown"
	}
}

type SystemHealthStatus int

const (
	HealthHealthy SystemHealthStatus = iota
	HealthDegraded
	HealthUnhealthy
)

func (s SystemHealthStatus) String() string {
	switch s {
	case HealthHealthy:
		return "healthy"
	case HealthDegraded:
		return "degraded"
	case HealthUnhealthy:
		return "unhealthy"
	default:
		return "unknown"
	}
}

// Supporting data structures

type ProcessingError struct {
	Code        string    `json:"code"`
	Message     string    `json:"message"`
	Service     string    `json:"service"`
	Recoverable bool      `json:"recoverable"`
	Timestamp   time.Time `json:"timestamp"`
}

type RoutingQualityMetrics struct {
	LatencyMs           int     `json:"latency_ms"`
	BuyerMatchAccuracy  float64 `json:"buyer_match_accuracy"`
	SellerUtilization   float64 `json:"seller_utilization"`
	PriceOptimization   float64 `json:"price_optimization"`
	SuccessRate         float64 `json:"success_rate"`
}

type AuctionMetrics struct {
	ParticipationRate   float64 `json:"participation_rate"`
	AverageBidAmount    float64 `json:"average_bid_amount"`
	PriceSpread         float64 `json:"price_spread"`
	CompetitionIndex    float64 `json:"competition_index"`
	BuyerSatisfaction   float64 `json:"buyer_satisfaction"`
}

type MarketplaceMetrics struct {
	CallsPerHour        int     `json:"calls_per_hour"`
	AverageAuctionTime  int     `json:"average_auction_time_ms"`
	BuyerUtilization    float64 `json:"buyer_utilization"`
	SellerUtilization   float64 `json:"seller_utilization"`
	RevenuePerHour      float64 `json:"revenue_per_hour"`
	FailureRate         float64 `json:"failure_rate"`
}

// Repository and external service interfaces needed by marketplace orchestrator

type CallRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*callpkg.Call, error)
	Create(ctx context.Context, call *callpkg.Call) error
	Update(ctx context.Context, call *callpkg.Call) error
	GetIncomingCalls(ctx context.Context, limit int) ([]*callpkg.Call, error)
	GetPendingSellerCalls(ctx context.Context, limit int) ([]*callpkg.Call, error)
}

type BidRepository interface {
	Create(ctx context.Context, bid *bid.Bid) error
	GetByID(ctx context.Context, id uuid.UUID) (*bid.Bid, error)
	GetActiveBidsForCall(ctx context.Context, callID uuid.UUID) ([]*bid.Bid, error)
	Update(ctx context.Context, bid *bid.Bid) error
}

type AccountRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*account.Account, error)
	GetActiveBuyers(ctx context.Context, limit int) ([]*account.Account, error)
	GetActiveSellers(ctx context.Context, limit int) ([]*account.Account, error)
}

// External service interfaces

type BuyerRoutingService interface {
	RouteCall(ctx context.Context, callID uuid.UUID) (*buyer_routing.BuyerRoutingDecision, error)
	GetActiveBuyers(ctx context.Context) ([]*account.Account, error)
}

type SellerDistributionService interface {
	DistributeCall(ctx context.Context, callID uuid.UUID) (*seller_distribution.SellerDistributionDecision, error)
	GetAvailableSellers(ctx context.Context) ([]*account.Account, error)
}

type BiddingService interface {
	StartAuction(ctx context.Context, callID uuid.UUID, duration time.Duration) (*AuctionInfo, error)
	PlaceBid(ctx context.Context, auctionID uuid.UUID, bid *bid.Bid) (*BidResult, error)
	CompleteAuction(ctx context.Context, auctionID uuid.UUID) (*bidding.AuctionResult, error)
}

type FraudService interface {
	CheckCall(ctx context.Context, call *callpkg.Call) (*fraud.FraudCheckResult, error)
	CheckBid(ctx context.Context, bid *bid.Bid, buyer *account.Account) (*fraud.FraudCheckResult, error)
	CheckAccount(ctx context.Context, account *account.Account) (*fraud.FraudCheckResult, error)
}

type TelephonyService interface {
	InitiateCall(ctx context.Context, call *callpkg.Call) (*telephony.CallResponse, error)
	TransferCall(ctx context.Context, callID uuid.UUID, targetNumber string) (*telephony.CallResponse, error)
	GetCallStatus(ctx context.Context, callID uuid.UUID) (*telephony.CallStatus, error)
}

// MarketplaceMetricsCollector defines the interface for marketplace metrics collection
type MarketplaceMetricsCollector interface {
	RecordCallProcessing(ctx context.Context, result *CallProcessingResult)
	RecordAuctionCompletion(ctx context.Context, result *AuctionResult)
	RecordRoutingDecision(ctx context.Context, result *RoutingResult)
	GetCurrentMetrics(ctx context.Context) (*MarketplaceMetrics, error)
}

// Additional type definitions needed for the service interfaces

// AuctionInfo represents auction information
type AuctionInfo struct {
	ID      uuid.UUID
	CallID  uuid.UUID
	Status  string
	StartedAt time.Time
}

// BidResult represents the result of placing a bid
type BidResult struct {
	BidID     uuid.UUID
	Accepted  bool
	Rank      int
	IsWinning bool
	Reason    string
}