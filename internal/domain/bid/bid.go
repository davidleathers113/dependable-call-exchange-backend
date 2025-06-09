package bid

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/validation"
)

type Bid struct {
	ID         uuid.UUID `json:"id"`
	CallID     uuid.UUID `json:"call_id"`
	BuyerID    uuid.UUID `json:"buyer_id"`
	SellerID   uuid.UUID `json:"seller_id"`
	Amount     float64   `json:"amount"`
	Status     Status    `json:"status"`
	
	// Auction details
	AuctionID  uuid.UUID `json:"auction_id"`
	Rank       int       `json:"rank"`
	
	// Targeting criteria
	Criteria   BidCriteria `json:"criteria"`
	
	// Quality metrics
	Quality    QualityMetrics `json:"quality"`
	
	// Timestamps
	PlacedAt   time.Time  `json:"placed_at"`
	ExpiresAt  time.Time  `json:"expires_at"`
	AcceptedAt *time.Time `json:"accepted_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

type Status int

const (
	StatusPending Status = iota
	StatusActive
	StatusWinning
	StatusWon
	StatusLost
	StatusExpired
	StatusCanceled
)

func (s Status) String() string {
	switch s {
	case StatusPending:
		return "pending"
	case StatusActive:
		return "active"
	case StatusWinning:
		return "winning"
	case StatusWon:
		return "won"
	case StatusLost:
		return "lost"
	case StatusExpired:
		return "expired"
	case StatusCanceled:
		return "canceled"
	default:
		return "unknown"
	}
}

type BidCriteria struct {
	Geography   GeoCriteria    `json:"geography"`
	TimeWindow  TimeWindow     `json:"time_window"`
	CallType    []string       `json:"call_type"`
	Keywords    []string       `json:"keywords"`
	ExcludeList []string       `json:"exclude_list"`
	MaxBudget   float64        `json:"max_budget"`
}

type GeoCriteria struct {
	Countries []string `json:"countries"`
	States    []string `json:"states"`
	Cities    []string `json:"cities"`
	ZipCodes  []string `json:"zip_codes"`
	Radius    *float64 `json:"radius,omitempty"`
}

type TimeWindow struct {
	StartHour int      `json:"start_hour"`
	EndHour   int      `json:"end_hour"`
	Days      []string `json:"days"`
	Timezone  string   `json:"timezone"`
}

type QualityMetrics struct {
	ConversionRate   float64 `json:"conversion_rate"`
	AverageCallTime  int     `json:"average_call_time"`
	FraudScore       float64 `json:"fraud_score"`
	HistoricalRating float64 `json:"historical_rating"`
}

type Auction struct {
	ID          uuid.UUID `json:"id"`
	CallID      uuid.UUID `json:"call_id"`
	Status      AuctionStatus `json:"status"`
	StartTime   time.Time `json:"start_time"`
	EndTime     time.Time `json:"end_time"`
	WinningBid  *uuid.UUID `json:"winning_bid,omitempty"`
	Bids        []Bid     `json:"bids"`
	
	// Auction parameters
	ReservePrice float64 `json:"reserve_price"`
	BidIncrement float64 `json:"bid_increment"`
	MaxDuration  int     `json:"max_duration"`
	
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type AuctionStatus int

const (
	AuctionStatusPending AuctionStatus = iota
	AuctionStatusActive
	AuctionStatusCompleted
	AuctionStatusCanceled
	AuctionStatusExpired
)

func (s AuctionStatus) String() string {
	switch s {
	case AuctionStatusPending:
		return "pending"
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

func NewBid(callID, buyerID, sellerID uuid.UUID, amount float64, criteria BidCriteria) (*Bid, error) {
	// Validate UUIDs
	if callID == uuid.Nil {
		return nil, fmt.Errorf("call ID cannot be nil")
	}
	if buyerID == uuid.Nil {
		return nil, fmt.Errorf("buyer ID cannot be nil")
	}
	if sellerID == uuid.Nil {
		return nil, fmt.Errorf("seller ID cannot be nil")
	}
	
	// Validate amount
	if err := validation.ValidateAmount(amount, "bid amount"); err != nil {
		return nil, err
	}
	
	// Minimum bid amount
	const minBidAmount = 0.01
	if amount < minBidAmount {
		return nil, fmt.Errorf("bid amount must be at least $%.2f", minBidAmount)
	}
	
	// Validate criteria
	if err := validateBidCriteria(criteria); err != nil {
		return nil, fmt.Errorf("invalid bid criteria: %w", err)
	}
	
	now := time.Now()
	return &Bid{
		ID:        uuid.New(),
		CallID:    callID,
		BuyerID:   buyerID,
		SellerID:  sellerID,
		Amount:    amount,
		Status:    StatusPending,
		Criteria:  criteria,
		PlacedAt:  now,
		ExpiresAt: now.Add(5 * time.Minute), // 5-minute expiry
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func validateBidCriteria(criteria BidCriteria) error {
	// Validate time window
	if criteria.TimeWindow.StartHour < 0 || criteria.TimeWindow.StartHour > 23 {
		return fmt.Errorf("invalid start hour: must be between 0 and 23")
	}
	if criteria.TimeWindow.EndHour < 0 || criteria.TimeWindow.EndHour > 23 {
		return fmt.Errorf("invalid end hour: must be between 0 and 23")
	}
	
	// Validate max budget if set
	if criteria.MaxBudget > 0 {
		if err := validation.ValidateAmount(criteria.MaxBudget, "max budget"); err != nil {
			return err
		}
	}
	
	// Validate geography radius if set
	if criteria.Geography.Radius != nil && *criteria.Geography.Radius < 0 {
		return fmt.Errorf("radius cannot be negative")
	}
	
	return nil
}

func (b *Bid) Accept() {
	now := time.Now()
	b.Status = StatusWon
	b.AcceptedAt = &now
	b.UpdatedAt = now
}

func (b *Bid) Reject() {
	b.Status = StatusLost
	b.UpdatedAt = time.Now()
}

func NewAuction(callID uuid.UUID, reservePrice float64) (*Auction, error) {
	// Validate call ID
	if callID == uuid.Nil {
		return nil, fmt.Errorf("call ID cannot be nil")
	}
	
	// Validate reserve price
	if err := validation.ValidateAmount(reservePrice, "reserve price"); err != nil {
		return nil, err
	}
	
	now := time.Now()
	return &Auction{
		ID:           uuid.New(),
		CallID:       callID,
		Status:       AuctionStatusPending,
		StartTime:    now,
		EndTime:      now.Add(30 * time.Second), // 30-second auction window
		ReservePrice: reservePrice,
		BidIncrement: 0.01,
		MaxDuration:  30,
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}