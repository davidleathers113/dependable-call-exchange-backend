package bidding

import (
	"context"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/account"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/bid"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/call"
	"github.com/google/uuid"
)

// Service defines the bidding service interface
type Service interface {
	// PlaceBid creates a new bid for a call
	PlaceBid(ctx context.Context, req *PlaceBidRequest) (*bid.Bid, error)
	// UpdateBid modifies an existing bid
	UpdateBid(ctx context.Context, bidID uuid.UUID, updates *BidUpdate) (*bid.Bid, error)
	// CancelBid cancels an active bid
	CancelBid(ctx context.Context, bidID uuid.UUID) error
	// GetBid retrieves a specific bid
	GetBid(ctx context.Context, bidID uuid.UUID) (*bid.Bid, error)
	// GetBidsForCall returns all bids for a specific call
	GetBidsForCall(ctx context.Context, callID uuid.UUID) ([]*bid.Bid, error)
	// GetBidsForBuyer returns all bids for a specific buyer
	GetBidsForBuyer(ctx context.Context, buyerID uuid.UUID) ([]*bid.Bid, error)
	// ProcessExpiredBids handles bid expiration
	ProcessExpiredBids(ctx context.Context) error
}

// AuctionEngine defines the interface for auction processing
type AuctionEngine interface {
	// RunAuction executes the auction for a call
	RunAuction(ctx context.Context, callID uuid.UUID) (*AuctionResult, error)
	// GetAuctionStatus returns current auction state
	GetAuctionStatus(ctx context.Context, callID uuid.UUID) (*AuctionStatus, error)
	// CloseAuction finalizes the auction
	CloseAuction(ctx context.Context, callID uuid.UUID) error
}

// BidRepository defines the interface for bid storage
type BidRepository interface {
	// Create stores a new bid
	Create(ctx context.Context, bid *bid.Bid) error
	// GetByID retrieves a bid by ID
	GetByID(ctx context.Context, id uuid.UUID) (*bid.Bid, error)
	// Update modifies an existing bid
	Update(ctx context.Context, bid *bid.Bid) error
	// Delete removes a bid
	Delete(ctx context.Context, id uuid.UUID) error
	// GetActiveBidsForCall returns active bids for a call
	GetActiveBidsForCall(ctx context.Context, callID uuid.UUID) ([]*bid.Bid, error)
	// GetByBuyer returns bids by buyer
	GetByBuyer(ctx context.Context, buyerID uuid.UUID) ([]*bid.Bid, error)
	// GetExpiredBids returns bids past expiration
	GetExpiredBids(ctx context.Context, before time.Time) ([]*bid.Bid, error)
}

// CallRepository defines the interface for call storage
type CallRepository interface {
	// GetByID retrieves a call by ID
	GetByID(ctx context.Context, id uuid.UUID) (*call.Call, error)
	// Update modifies a call
	Update(ctx context.Context, call *call.Call) error
}

// AccountRepository defines the interface for account storage
type AccountRepository interface {
	// GetByID retrieves an account by ID
	GetByID(ctx context.Context, id uuid.UUID) (*account.Account, error)
	// UpdateBalance updates account balance
	UpdateBalance(ctx context.Context, id uuid.UUID, amount float64) error
	// GetBalance returns current balance
	GetBalance(ctx context.Context, id uuid.UUID) (float64, error)
}

// FraudChecker defines the interface for fraud detection
type FraudChecker interface {
	// CheckBid validates bid for fraud indicators
	CheckBid(ctx context.Context, bid *bid.Bid, buyer *account.Account) (*FraudCheckResult, error)
	// GetRiskScore returns risk score for buyer
	GetRiskScore(ctx context.Context, buyerID uuid.UUID) (float64, error)
}

// NotificationService defines the interface for notifications
type NotificationService interface {
	// NotifyBidPlaced sends bid placement notification
	NotifyBidPlaced(ctx context.Context, bid *bid.Bid) error
	// NotifyBidWon sends winning bid notification
	NotifyBidWon(ctx context.Context, bid *bid.Bid) error
	// NotifyBidLost sends losing bid notification
	NotifyBidLost(ctx context.Context, bid *bid.Bid) error
	// NotifyBidExpired sends expiration notification
	NotifyBidExpired(ctx context.Context, bid *bid.Bid) error
}

// MetricsCollector defines the interface for metrics
type MetricsCollector interface {
	// RecordBidPlaced records bid placement metrics
	RecordBidPlaced(ctx context.Context, bid *bid.Bid)
	// RecordAuctionDuration records auction timing
	RecordAuctionDuration(ctx context.Context, callID uuid.UUID, duration time.Duration)
	// RecordBidAmount records bid amount distribution
	RecordBidAmount(ctx context.Context, amount float64)
}

// PlaceBidRequest represents a bid placement request
type PlaceBidRequest struct {
	CallID       uuid.UUID
	BuyerID      uuid.UUID
	Amount       float64
	Criteria     map[string]interface{}
	Duration     time.Duration // How long bid is valid
	AutoRenew    bool         // Auto-renew on expiration
	MaxAmount    float64      // Maximum for auto-bidding
}

// BidUpdate represents bid modification request
type BidUpdate struct {
	Amount       *float64
	Criteria     map[string]interface{}
	ExtendBy     *time.Duration
	AutoRenew    *bool
	MaxAmount    *float64
}

// AuctionResult represents the outcome of an auction
type AuctionResult struct {
	CallID       uuid.UUID
	WinningBidID uuid.UUID
	WinnerID     uuid.UUID
	FinalAmount  float64
	RunnerUpBids []uuid.UUID
	StartTime    time.Time
	EndTime      time.Time
	Participants int
}

// AuctionStatus represents current auction state
type AuctionStatus struct {
	CallID       uuid.UUID
	Status       string // "open", "closing", "closed"
	BidCount     int
	TopBidAmount float64
	TimeLeft     time.Duration
	LastUpdate   time.Time
}

// FraudCheckResult represents fraud detection outcome
type FraudCheckResult struct {
	Approved    bool
	RiskScore   float64
	Reasons     []string
	Flags       []string
	RequiresMFA bool
}