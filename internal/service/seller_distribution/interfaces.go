package seller_distribution

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/call"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/account"
)

// SellerDistributionService defines the interface for distributing calls to sellers
type SellerDistributionService interface {
	// DistributeCall distributes an incoming call to available sellers for bidding
	DistributeCall(ctx context.Context, callID uuid.UUID) (*SellerDistributionDecision, error)
	
	// GetAvailableSellers returns sellers who can accept calls based on criteria
	GetAvailableSellers(ctx context.Context, criteria *SellerCriteria) ([]*account.Account, error)
	
	// NotifySellers sends call availability notifications to eligible sellers
	NotifySellers(ctx context.Context, callID uuid.UUID, sellerIDs []uuid.UUID) error
}

// SellerDistributionDecision represents the outcome of call distribution to sellers
type SellerDistributionDecision struct {
	CallID           uuid.UUID            `json:"call_id"`
	Algorithm        string               `json:"algorithm"`
	SelectedSellers  []uuid.UUID          `json:"selected_sellers"`
	NotifiedCount    int                  `json:"notified_count"`
	Score            float64              `json:"score,omitempty"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
	ProcessedAt      time.Time            `json:"processed_at"`
	AuctionStartTime time.Time            `json:"auction_start_time"`
	AuctionDuration  time.Duration        `json:"auction_duration"`
}

// SellerCriteria defines the criteria for selecting sellers
type SellerCriteria struct {
	Geography    *GeoCriteria     `json:"geography,omitempty"`
	Skills       []string         `json:"skills,omitempty"`
	CallType     []string         `json:"call_type,omitempty"`
	MinQuality   float64          `json:"min_quality,omitempty"`
	MaxCapacity  int              `json:"max_capacity,omitempty"`
	AvailableNow bool             `json:"available_now"`
	Languages    []string         `json:"languages,omitempty"`
}

// GeoCriteria defines geographical selection criteria
type GeoCriteria struct {
	Countries []string `json:"countries,omitempty"`
	States    []string `json:"states,omitempty"`
	Cities    []string `json:"cities,omitempty"`
	Radius    float64  `json:"radius,omitempty"`    // km radius
	Latitude  float64  `json:"latitude,omitempty"`
	Longitude float64  `json:"longitude,omitempty"`
}

// SellerDistributionRules defines the rules for seller distribution
type SellerDistributionRules struct {
	Algorithm         string  `json:"algorithm"`          // "broadcast", "targeted", "capacity-based"
	MaxSellers        int     `json:"max_sellers"`        // Maximum number of sellers to notify
	MinQualityScore   float64 `json:"min_quality_score"`  // Minimum seller quality score
	QualityWeight     float64 `json:"quality_weight"`     // Weight for quality in selection
	CapacityWeight    float64 `json:"capacity_weight"`    // Weight for available capacity
	GeographyWeight   float64 `json:"geography_weight"`   // Weight for geographic proximity
	AuctionDuration   time.Duration `json:"auction_duration"` // How long to keep auction open
	RequireSkillMatch bool    `json:"require_skill_match"` // Must match call type skills
}

// SellerMetrics defines the interface for seller distribution metrics
type SellerMetrics interface {
	// RecordDistribution records a distribution decision
	RecordDistribution(ctx context.Context, decision *SellerDistributionDecision)
	
	// RecordSellerNotification records seller notification metrics
	RecordSellerNotification(ctx context.Context, sellerID uuid.UUID, callID uuid.UUID, notified bool)
	
	// RecordSellerResponse records seller response metrics
	RecordSellerResponse(ctx context.Context, sellerID uuid.UUID, callID uuid.UUID, responded bool)
}

// CallRepository defines the interface for call storage operations needed by seller distribution
type CallRepository interface {
	// GetByID retrieves a call by its ID
	GetByID(ctx context.Context, id uuid.UUID) (*call.Call, error)
	
	// Update updates an existing call
	Update(ctx context.Context, call *call.Call) error
	
	// GetIncomingCalls returns calls awaiting seller assignment
	GetIncomingCalls(ctx context.Context, limit int) ([]*call.Call, error)
}

// AccountRepository defines the interface for account operations needed by seller distribution
type AccountRepository interface {
	// GetByID retrieves an account by its ID
	GetByID(ctx context.Context, id uuid.UUID) (*account.Account, error)
	
	// GetAvailableSellers returns sellers available for call assignment
	GetAvailableSellers(ctx context.Context, criteria *SellerCriteria) ([]*account.Account, error)
	
	// GetSellerCapacity returns current capacity information for a seller
	GetSellerCapacity(ctx context.Context, sellerID uuid.UUID) (*SellerCapacity, error)
}

// NotificationService defines the interface for seller notifications
type NotificationService interface {
	// NotifyCallAvailable sends call availability notification to seller
	NotifyCallAvailable(ctx context.Context, sellerID uuid.UUID, callID uuid.UUID) error
	
	// NotifyAuctionStarted sends auction start notification to multiple sellers
	NotifyAuctionStarted(ctx context.Context, sellerIDs []uuid.UUID, callID uuid.UUID, auctionDuration time.Duration) error
}

// SellerCapacity represents a seller's current capacity information
type SellerCapacity struct {
	SellerID         uuid.UUID `json:"seller_id"`
	MaxConcurrentCalls int      `json:"max_concurrent_calls"`
	CurrentCalls     int       `json:"current_calls"`
	AvailableSlots   int       `json:"available_slots"`
	LastUpdated      time.Time `json:"last_updated"`
}