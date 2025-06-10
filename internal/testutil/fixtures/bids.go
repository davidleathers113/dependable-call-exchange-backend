package fixtures

import (
	"context"
	"testing"
	"time"
	
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/bid"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/testutil"
)

// BidBuilder builds test Bid entities
type BidBuilder struct {
	t          *testing.T
	testDB     *testutil.TestDB
	id         uuid.UUID
	callID     uuid.UUID
	buyerID    uuid.UUID
	sellerID   uuid.UUID
	amount     values.Money
	status     bid.Status
	criteria   bid.BidCriteria
	quality    values.QualityMetrics
	auctionID  uuid.UUID
	rank       int
	placedAt   time.Time
	expiresAt  time.Time
}

// NewBidBuilder creates a new BidBuilder with defaults
func NewBidBuilder(testDB *testutil.TestDB) *BidBuilder {
	id := uuid.New()
	callID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New()
	auctionID := uuid.New()
	
	now := time.Now().UTC()
	return &BidBuilder{
		t:         nil, // Will be set when Build is called
		testDB:    testDB,
		id:        id,
		callID:    callID,
		buyerID:   buyerID,
		sellerID:  sellerID,
		auctionID: auctionID,
		amount:    values.MustNewMoneyFromFloat(5.00, values.USD),
		status:    bid.StatusActive,
		rank:      1,
		placedAt:  now,
		expiresAt: now.Add(5 * time.Minute),
		criteria: bid.BidCriteria{
			Geography: bid.GeoCriteria{
				States: []string{"CA", "TX", "NY"},
			},
			TimeWindow: bid.TimeWindow{
				StartHour: 9,
				EndHour:   17,
				Days:      []string{"Mon", "Tue", "Wed", "Thu", "Fri"},
				Timezone:  "America/New_York",
			},
			CallType:  []string{"inbound"},
			MaxBudget: values.MustNewMoneyFromFloat(100.00, values.USD),
		},
		quality: values.MustNewQualityMetrics(
			4.5,  // qualityScore
			0.05, // fraudScore
			4.5,  // historicalRating
			0.15, // conversionRate
			180,  // averageCallTime
			5.0,  // trustScore
			5.0,  // reliabilityScore
		),
	}
}

// WithID sets the bid ID
func (b *BidBuilder) WithID(id uuid.UUID) *BidBuilder {
	b.id = id
	return b
}

// WithCallID sets the call ID
func (b *BidBuilder) WithCallID(callID uuid.UUID) *BidBuilder {
	b.callID = callID
	return b
}

// WithBuyerID sets the buyer ID
func (b *BidBuilder) WithBuyerID(buyerID uuid.UUID) *BidBuilder {
	b.buyerID = buyerID
	return b
}

// WithAmount sets the bid amount
func (b *BidBuilder) WithAmount(amount float64) *BidBuilder {
	b.amount = values.MustNewMoneyFromFloat(amount, values.USD)
	return b
}

// WithStatus sets the bid status
func (b *BidBuilder) WithStatus(status bid.Status) *BidBuilder {
	b.status = status
	return b
}

// WithSellerID sets the seller ID
func (b *BidBuilder) WithSellerID(sellerID uuid.UUID) *BidBuilder {
	b.sellerID = sellerID
	return b
}

// WithCriteria sets the bid criteria
func (b *BidBuilder) WithCriteria(criteria bid.BidCriteria) *BidBuilder {
	b.criteria = criteria
	return b
}

// WithQuality sets the quality metrics
func (b *BidBuilder) WithQuality(quality values.QualityMetrics) *BidBuilder {
	b.quality = quality
	return b
}

// WithAuctionID sets the auction ID
func (b *BidBuilder) WithAuctionID(auctionID uuid.UUID) *BidBuilder {
	b.auctionID = auctionID
	return b
}

// WithRank sets the bid rank
func (b *BidBuilder) WithRank(rank int) *BidBuilder {
	b.rank = rank
	return b
}

// WithPlacedAt sets when the bid was placed
func (b *BidBuilder) WithPlacedAt(placedAt time.Time) *BidBuilder {
	b.placedAt = placedAt
	return b
}

// WithExpiration sets the expiration duration from placement time
func (b *BidBuilder) WithExpiration(duration time.Duration) *BidBuilder {
	b.expiresAt = b.placedAt.Add(duration)
	return b
}

// WithQualityMetrics sets the quality metrics using individual values
func (b *BidBuilder) WithQualityMetrics(conversionRate float64, avgCallTime int, fraudScore float64, rating float64) *BidBuilder {
	b.quality = values.MustNewQualityMetrics(
		rating,         // qualityScore (use historical rating as base)
		fraudScore,     // fraudScore
		rating,         // historicalRating
		conversionRate, // conversionRate
		avgCallTime,    // averageCallTime
		5.0,           // trustScore (default)
		5.0,           // reliabilityScore (default)
	)
	return b
}

// Build creates the Bid entity
func (b *BidBuilder) Build(t *testing.T) *bid.Bid {
	t.Helper()
	now := time.Now().UTC()
	bidEntity := &bid.Bid{
		ID:        b.id,
		CallID:    b.callID,
		BuyerID:   b.buyerID,
		SellerID:  b.sellerID,
		Amount:    b.amount,
		Status:    b.status,
		AuctionID: b.auctionID,
		Rank:      b.rank,
		Criteria:  b.criteria,
		Quality:   b.quality,
		PlacedAt:  b.placedAt,
		ExpiresAt: b.expiresAt,
		CreatedAt: now,
		UpdatedAt: now,
	}
	
	// Set accepted time if bid is won
	if b.status == bid.StatusWon {
		acceptedAt := now
		bidEntity.AcceptedAt = &acceptedAt
	}
	
	// Note: The BuildWithRepo method should be used for DB persistence
	
	return bidEntity
}

// BuildWithRepo creates the Bid entity and saves it using the provided repository
func (b *BidBuilder) BuildWithRepo(t *testing.T, repo interface{
	Create(ctx context.Context, bid *bid.Bid) error
}, ctx context.Context) *bid.Bid {
	t.Helper()
	bidEntity := b.Build(t)
	
	// Save using repository
	err := repo.Create(ctx, bidEntity)
	require.NoError(t, err, "failed to create bid via repository")
	
	return bidEntity
}

// BidScenarios provides common bid test scenarios
type BidScenarios struct {
	t      *testing.T
	testDB *testutil.TestDB
}

// NewBidScenarios creates a new BidScenarios helper
func NewBidScenarios(t *testing.T, testDB *testutil.TestDB) *BidScenarios {
	t.Helper()
	return &BidScenarios{t: t, testDB: testDB}
}

// HighValueBid creates a high-value bid scenario
func (bs *BidScenarios) HighValueBid(callID uuid.UUID) *bid.Bid {
	bs.t.Helper()
	return NewBidBuilder(bs.testDB).
		WithCallID(callID).
		WithAmount(25.00).
		WithCriteria(bid.BidCriteria{
			Geography: bid.GeoCriteria{
				Countries: []string{"US", "CA"},
			},
			TimeWindow: bid.TimeWindow{
				StartHour: 0,
				EndHour:   24,
				Days:      []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"},
			},
			CallType:   []string{"inbound", "outbound"},
			MaxBudget:  values.MustNewMoneyFromFloat(500.00, values.USD),
			Keywords:   []string{"premium", "high-value", "qualified"},
		}).
		WithQuality(values.MustNewQualityMetrics(
			4.8, // qualityScore (use historical rating)
			0.01, // fraudScore
			4.8, // historicalRating  
			0.35, // conversionRate
			420, // averageCallTime
			5.0, // trustScore (default)
			5.0, // reliabilityScore (default)
		)).
		Build(bs.t)
}

// LowValueBid creates a low-value bid scenario
func (bs *BidScenarios) LowValueBid(callID uuid.UUID) *bid.Bid {
	bs.t.Helper()
	return NewBidBuilder(bs.testDB).
		WithCallID(callID).
		WithAmount(2.50).
		WithCriteria(bid.BidCriteria{
			Geography: bid.GeoCriteria{
				States: []string{"CA"},
			},
			TimeWindow: bid.TimeWindow{
				StartHour: 9,
				EndHour:   17,
				Days:      []string{"Mon", "Tue", "Wed", "Thu", "Fri"},
				Timezone:  "America/Chicago",
			},
			CallType:  []string{"inbound"},
			MaxBudget: values.MustNewMoneyFromFloat(50.00, values.USD),
		}).
		WithQuality(values.MustNewQualityMetrics(
			3.2, // qualityScore (use historical rating)
			0.15, // fraudScore
			3.2, // historicalRating
			0.08, // conversionRate
			120, // averageCallTime
			4.0, // trustScore (lower for low value)
			4.0, // reliabilityScore (lower for low value)
		)).
		Build(bs.t)
}

// ExpiredBid creates an expired bid scenario
func (bs *BidScenarios) ExpiredBid(callID uuid.UUID) *bid.Bid {
	bs.t.Helper()
	return NewBidBuilder(bs.testDB).
		WithCallID(callID).
		WithStatus(bid.StatusExpired).
		WithExpiration(-1 * time.Minute).
		Build(bs.t)
}

// WinningBid creates a winning bid scenario
func (bs *BidScenarios) WinningBid(callID uuid.UUID) *bid.Bid {
	bs.t.Helper()
	return NewBidBuilder(bs.testDB).
		WithCallID(callID).
		WithStatus(bid.StatusWon).
		WithAmount(15.00).
		Build(bs.t)
}

// CompetingBids creates multiple competing bids for the same call
func (bs *BidScenarios) CompetingBids(callID uuid.UUID, count int) []*bid.Bid {
	bs.t.Helper()
	bids := make([]*bid.Bid, count)
	
	for i := 0; i < count; i++ {
		buyerID := uuid.New()
		amount := 5.00 + float64(i)*2.50
		
		bids[i] = NewBidBuilder(bs.testDB).
			WithCallID(callID).
			WithBuyerID(buyerID).
			WithAmount(amount).
			Build(bs.t)
	}
	
	return bids
}