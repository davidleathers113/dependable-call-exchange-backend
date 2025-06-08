package fixtures

import (
	"testing"
	"time"
	
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/bid"
)

// BidBuilder builds test Bid entities
type BidBuilder struct {
	t          *testing.T
	id         uuid.UUID
	callID     uuid.UUID
	buyerID    uuid.UUID
	sellerID   uuid.UUID
	amount     float64
	status     bid.Status
	criteria   bid.BidCriteria
	quality    bid.QualityMetrics
	auctionID  uuid.UUID
	rank       int
	placedAt   time.Time
	expiresAt  time.Time
}

// NewBidBuilder creates a new BidBuilder with defaults
func NewBidBuilder(t *testing.T) *BidBuilder {
	t.Helper()
	id, err := uuid.NewRandom()
	require.NoError(t, err)
	callID, err := uuid.NewRandom()
	require.NoError(t, err)
	buyerID, err := uuid.NewRandom()
	require.NoError(t, err)
	sellerID, err := uuid.NewRandom()
	require.NoError(t, err)
	auctionID, err := uuid.NewRandom()
	require.NoError(t, err)
	
	now := time.Now().UTC()
	return &BidBuilder{
		t:         t,
		id:        id,
		callID:    callID,
		buyerID:   buyerID,
		sellerID:  sellerID,
		auctionID: auctionID,
		amount:    5.00,
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
			MaxBudget: 100.00,
		},
		quality: bid.QualityMetrics{
			ConversionRate:   0.15,
			AverageCallTime:  180,
			FraudScore:       0.05,
			HistoricalRating: 4.5,
		},
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
	b.amount = amount
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
func (b *BidBuilder) WithQuality(quality bid.QualityMetrics) *BidBuilder {
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

// WithExpiration sets the expiration time
func (b *BidBuilder) WithExpiration(duration time.Duration) *BidBuilder {
	b.expiresAt = b.placedAt.Add(duration)
	return b
}

// Build creates the Bid entity
func (b *BidBuilder) Build() *bid.Bid {
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
	
	return bidEntity
}

// BidScenarios provides common bid test scenarios
type BidScenarios struct {
	t *testing.T
}

// NewBidScenarios creates a new BidScenarios helper
func NewBidScenarios(t *testing.T) *BidScenarios {
	t.Helper()
	return &BidScenarios{t: t}
}

// HighValueBid creates a high value bid
func (bs *BidScenarios) HighValueBid(callID uuid.UUID) *bid.Bid {
	return NewBidBuilder(bs.t).
		WithCallID(callID).
		WithAmount(25.00).
		WithCriteria(bid.BidCriteria{
			Geography: bid.GeoCriteria{
				States: []string{"CA", "NY"},
			},
			TimeWindow: bid.TimeWindow{
				StartHour: 8,
				EndHour:   20,
				Days:      []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat"},
				Timezone:  "America/New_York",
			},
			CallType:  []string{"inbound"},
			Keywords:  []string{"sales", "insurance", "premium"},
			MaxBudget: 500.00,
		}).
		WithQuality(bid.QualityMetrics{
			ConversionRate:   0.25,
			AverageCallTime:  240,
			FraudScore:       0.02,
			HistoricalRating: 4.8,
		}).
		Build()
}

// LowValueBid creates a low value bid
func (bs *BidScenarios) LowValueBid(callID uuid.UUID) *bid.Bid {
	return NewBidBuilder(bs.t).
		WithCallID(callID).
		WithAmount(2.50).
		WithCriteria(bid.BidCriteria{
			Geography: bid.GeoCriteria{
				Countries: []string{"US"}, // Any US state
			},
			TimeWindow: bid.TimeWindow{
				StartHour: 9,
				EndHour:   17,
				Days:      []string{"Mon", "Tue", "Wed", "Thu", "Fri"},
				Timezone:  "America/Chicago",
			},
			CallType:  []string{"inbound", "outbound"},
			MaxBudget: 50.00,
		}).
		WithQuality(bid.QualityMetrics{
			ConversionRate:   0.08,
			AverageCallTime:  90,
			FraudScore:       0.10,
			HistoricalRating: 3.5,
		}).
		Build()
}

// ExpiredBid creates an expired bid
func (bs *BidScenarios) ExpiredBid(callID uuid.UUID) *bid.Bid {
	return NewBidBuilder(bs.t).
		WithCallID(callID).
		WithStatus(bid.StatusExpired).
		WithExpiration(-1 * time.Minute). // Already expired
		Build()
}

// WinningBid creates a winning bid
func (bs *BidScenarios) WinningBid(callID uuid.UUID) *bid.Bid {
	return NewBidBuilder(bs.t).
		WithCallID(callID).
		WithStatus(bid.StatusWon).
		WithAmount(15.00).
		Build()
}

// CompetingBids creates multiple competing bids for the same call
func (bs *BidScenarios) CompetingBids(callID uuid.UUID, count int) []*bid.Bid {
	bids := make([]*bid.Bid, count)
	baseAmount := 5.00
	
	for i := 0; i < count; i++ {
		buyerID, err := uuid.NewRandom()
		require.NoError(bs.t, err)
		
		// Vary amounts to create competition
		amount := baseAmount + float64(i)*0.50
		
		bids[i] = NewBidBuilder(bs.t).
			WithCallID(callID).
			WithBuyerID(buyerID).
			WithAmount(amount).
			Build()
	}
	
	return bids
}

// CriteriaBuilder helps build complex bid criteria
type CriteriaBuilder struct {
	criteria bid.BidCriteria
}

// NewCriteriaBuilder creates a new CriteriaBuilder
func NewCriteriaBuilder() *CriteriaBuilder {
	return &CriteriaBuilder{
		criteria: bid.BidCriteria{
			Geography: bid.GeoCriteria{},
			TimeWindow: bid.TimeWindow{
				StartHour: 9,
				EndHour:   17,
				Timezone:  "America/New_York",
			},
			CallType: []string{},
		},
	}
}

// WithGeography sets geographic criteria
func (cb *CriteriaBuilder) WithGeography(countries, states, cities []string) *CriteriaBuilder {
	cb.criteria.Geography.Countries = countries
	cb.criteria.Geography.States = states
	cb.criteria.Geography.Cities = cities
	return cb
}

// WithTimeWindow sets time window criteria
func (cb *CriteriaBuilder) WithTimeWindow(startHour, endHour int, days []string, timezone string) *CriteriaBuilder {
	cb.criteria.TimeWindow = bid.TimeWindow{
		StartHour: startHour,
		EndHour:   endHour,
		Days:      days,
		Timezone:  timezone,
	}
	return cb
}

// WithCallTypes sets accepted call types
func (cb *CriteriaBuilder) WithCallTypes(types ...string) *CriteriaBuilder {
	cb.criteria.CallType = append(cb.criteria.CallType, types...)
	return cb
}

// WithKeywords adds keyword targeting
func (cb *CriteriaBuilder) WithKeywords(keywords ...string) *CriteriaBuilder {
	cb.criteria.Keywords = append(cb.criteria.Keywords, keywords...)
	return cb
}

// WithMaxBudget sets the maximum budget
func (cb *CriteriaBuilder) WithMaxBudget(budget float64) *CriteriaBuilder {
	cb.criteria.MaxBudget = budget
	return cb
}

// Build returns the constructed criteria
func (cb *CriteriaBuilder) Build() bid.BidCriteria {
	return cb.criteria
}