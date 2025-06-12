package callrouting

import (
	"context"
	"testing"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/bid"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/call"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRoundRobinRouter_Route(t *testing.T) {
	ctx := context.Background()
	router := NewRoundRobinRouter()

	// Create fixed UUIDs for deterministic testing
	bid1ID := uuid.MustParse("00000000-0000-0000-0000-000000000001")
	bid2ID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	bid3ID := uuid.MustParse("00000000-0000-0000-0000-000000000003")
	bid4ID := uuid.MustParse("00000000-0000-0000-0000-000000000004")

	tests := []struct {
		name           string
		call           *call.Call
		bids           []*bid.Bid
		expectedError  bool
		expectedBidIDs []uuid.UUID // Expected sequence of bid IDs
	}{
		{
			name: "cycles through bids in order",
			call: &call.Call{
				ID:     uuid.New(),
				Status: call.StatusPending,
			},
			bids: []*bid.Bid{
				{ID: bid1ID, Status: bid.StatusActive, BuyerID: uuid.New()},
				{ID: bid2ID, Status: bid.StatusActive, BuyerID: uuid.New()},
				{ID: bid3ID, Status: bid.StatusActive, BuyerID: uuid.New()},
			},
			expectedError:  false,
			expectedBidIDs: []uuid.UUID{bid1ID, bid2ID, bid3ID, bid1ID, bid2ID}, // Round-robin pattern
		},
		{
			name: "handles single bid",
			call: &call.Call{
				ID:     uuid.New(),
				Status: call.StatusPending,
			},
			bids: []*bid.Bid{
				{ID: bid1ID, Status: bid.StatusActive, BuyerID: uuid.New()},
			},
			expectedError:  false,
			expectedBidIDs: []uuid.UUID{bid1ID, bid1ID, bid1ID}, // Always selects the only bid
		},
		{
			name: "filters out inactive bids",
			call: &call.Call{
				ID:     uuid.New(),
				Status: call.StatusPending,
			},
			bids: []*bid.Bid{
				{ID: bid1ID, Status: bid.StatusExpired, BuyerID: uuid.New()},
				{ID: bid2ID, Status: bid.StatusActive, BuyerID: uuid.New()},
				{ID: bid3ID, Status: bid.StatusCanceled, BuyerID: uuid.New()},
				{ID: bid4ID, Status: bid.StatusActive, BuyerID: uuid.New()},
			},
			expectedError:  false,
			expectedBidIDs: []uuid.UUID{bid2ID, bid4ID, bid2ID}, // Only active bids (bid2 and bid4)
		},
		{
			name:          "errors with no bids",
			call:          &call.Call{ID: uuid.New(), Status: call.StatusPending},
			bids:          []*bid.Bid{},
			expectedError: true,
		},
		{
			name: "errors with no active bids",
			call: &call.Call{ID: uuid.New(), Status: call.StatusPending},
			bids: []*bid.Bid{
				{ID: uuid.New(), Status: bid.StatusExpired, BuyerID: uuid.New()},
				{ID: uuid.New(), Status: bid.StatusCanceled, BuyerID: uuid.New()},
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset router state for each test
			router = NewRoundRobinRouter()

			if tt.expectedError {
				_, err := router.Route(ctx, tt.call, tt.bids)
				require.Error(t, err)
			} else {
				// Test the sequence of selections
				for i, expectedBidID := range tt.expectedBidIDs {
					decision, err := router.Route(ctx, tt.call, tt.bids)
					require.NoError(t, err, "routing failed at iteration %d", i)
					assert.Equal(t, expectedBidID, decision.BidID, "wrong bid selected at iteration %d", i)
					assert.Equal(t, "round-robin", decision.Algorithm)
					assert.Equal(t, 1.0, decision.Score)
				}
			}
		})
	}
}

func TestSkillBasedRouter_Route(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name           string
		call           *call.Call
		bids           []*bid.Bid
		skillWeights   map[string]float64
		expectedError  bool
		expectedWinner int // Index of expected winning bid
	}{
		{
			name: "selects bid with matching call type",
			call: &call.Call{
				ID:        uuid.New(),
				Status:    call.StatusPending,
				Direction: call.DirectionInbound,
			},
			bids: []*bid.Bid{
				{
					ID: uuid.New(), Status: bid.StatusActive, BuyerID: uuid.New(),
					Criteria: bid.BidCriteria{
						CallType: []string{"outbound"},
					},
					Quality: values.QualityMetrics{
						ConversionRate:   0.20,
						FraudScore:       0.05,
						HistoricalRating: 4.5,
					},
				},
				{
					ID: uuid.New(), Status: bid.StatusActive, BuyerID: uuid.New(),
					Criteria: bid.BidCriteria{
						CallType: []string{"inbound"},
					},
					Quality: values.QualityMetrics{
						ConversionRate:   0.15,
						FraudScore:       0.08,
						HistoricalRating: 4.0,
					},
				},
			},
			expectedError:  false,
			expectedWinner: 1, // Bid that accepts inbound calls
		},
		{
			name: "prioritizes quality metrics",
			call: &call.Call{
				ID:        uuid.New(),
				Status:    call.StatusPending,
				Direction: call.DirectionInbound,
			},
			bids: []*bid.Bid{
				{
					ID: uuid.New(), Status: bid.StatusActive, BuyerID: uuid.New(),
					Criteria: bid.BidCriteria{
						CallType: []string{"inbound"},
					},
					Quality: values.QualityMetrics{
						ConversionRate:   0.10,
						FraudScore:       0.15,
						HistoricalRating: 3.0,
					},
				},
				{
					ID: uuid.New(), Status: bid.StatusActive, BuyerID: uuid.New(),
					Criteria: bid.BidCriteria{
						CallType: []string{"inbound"},
					},
					Quality: values.QualityMetrics{
						ConversionRate:   0.25,
						FraudScore:       0.02,
						HistoricalRating: 4.8,
					},
				},
			},
			expectedError:  false,
			expectedWinner: 1, // Bid with better quality metrics
		},
		{
			name: "considers geographic match",
			call: &call.Call{
				ID:        uuid.New(),
				Status:    call.StatusPending,
				Direction: call.DirectionInbound,
				Location: &call.Location{
					State: "CA",
				},
			},
			bids: []*bid.Bid{
				{
					ID: uuid.New(), Status: bid.StatusActive, BuyerID: uuid.New(),
					Criteria: bid.BidCriteria{
						CallType: []string{"inbound"},
						Geography: bid.GeoCriteria{
							States: []string{"NY", "TX"},
						},
					},
					Quality: values.QualityMetrics{
						ConversionRate:   0.20,
						FraudScore:       0.05,
						HistoricalRating: 4.5,
					},
				},
				{
					ID: uuid.New(), Status: bid.StatusActive, BuyerID: uuid.New(),
					Criteria: bid.BidCriteria{
						CallType: []string{"inbound"},
						Geography: bid.GeoCriteria{
							States: []string{"CA", "WA"},
						},
					},
					Quality: values.QualityMetrics{
						ConversionRate:   0.18,
						FraudScore:       0.06,
						HistoricalRating: 4.3,
					},
				},
			},
			expectedError:  false,
			expectedWinner: 1, // Bid with geographic match gets bonus
		},
		{
			name:          "errors with no bids",
			call:          &call.Call{ID: uuid.New(), Status: call.StatusPending},
			bids:          []*bid.Bid{},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := NewSkillBasedRouter(tt.skillWeights)

			decision, err := router.Route(ctx, tt.call, tt.bids)

			if tt.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.bids[tt.expectedWinner].ID, decision.BidID)
				assert.Equal(t, "skill-based", decision.Algorithm)
				assert.Greater(t, decision.Score, 0.0)
			}
		})
	}
}

func TestCostBasedRouter_Route(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name           string
		call           *call.Call
		bids           []*bid.Bid
		qualityWeight  float64
		priceWeight    float64
		capacityWeight float64
		expectedError  bool
		expectedWinner int
	}{
		{
			name: "balances quality and price",
			call: &call.Call{
				ID:     uuid.New(),
				Status: call.StatusPending,
			},
			bids: []*bid.Bid{
				{
					ID: uuid.New(), Status: bid.StatusActive, BuyerID: uuid.New(),
					Amount: values.MustNewMoneyFromFloat(10.0, "USD"), // High price
					Quality: values.QualityMetrics{
						ConversionRate:   0.10,
						AverageCallTime:  240,
						FraudScore:       0.15,
						HistoricalRating: 3.0,
					},
				},
				{
					ID: uuid.New(), Status: bid.StatusActive, BuyerID: uuid.New(),
					Amount: values.MustNewMoneyFromFloat(5.0, "USD"), // Lower price
					Quality: values.QualityMetrics{
						ConversionRate:   0.25,
						AverageCallTime:  180,
						FraudScore:       0.02,
						HistoricalRating: 4.8,
					},
				},
			},
			qualityWeight:  0.5,
			priceWeight:    0.3,
			capacityWeight: 0.2,
			expectedError:  false,
			expectedWinner: 1, // Better quality and lower price
		},
		{
			name: "handles equal bids",
			call: &call.Call{
				ID:     uuid.New(),
				Status: call.StatusPending,
			},
			bids: []*bid.Bid{
				{
					ID: uuid.New(), Status: bid.StatusActive, BuyerID: uuid.New(),
					Amount: values.MustNewMoneyFromFloat(5.0, "USD"),
					Quality: values.QualityMetrics{
						ConversionRate:   0.15,
						AverageCallTime:  180,
						FraudScore:       0.05,
						HistoricalRating: 4.0,
					},
				},
				{
					ID: uuid.New(), Status: bid.StatusActive, BuyerID: uuid.New(),
					Amount: values.MustNewMoneyFromFloat(5.0, "USD"),
					Quality: values.QualityMetrics{
						ConversionRate:   0.15,
						AverageCallTime:  180,
						FraudScore:       0.05,
						HistoricalRating: 4.0,
					},
				},
			},
			qualityWeight:  0.33,
			priceWeight:    0.33,
			capacityWeight: 0.34,
			expectedError:  false,
			expectedWinner: 0, // First bid when equal
		},
		{
			name:          "errors with no bids",
			call:          &call.Call{ID: uuid.New(), Status: call.StatusPending},
			bids:          []*bid.Bid{},
			qualityWeight: 0.5,
			priceWeight:   0.5,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := NewCostBasedRouter(tt.qualityWeight, tt.priceWeight, tt.capacityWeight)

			decision, err := router.Route(ctx, tt.call, tt.bids)

			if tt.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.bids[tt.expectedWinner].ID, decision.BidID)
				assert.Equal(t, "cost-based", decision.Algorithm)

				// Verify metadata contains scores
				metadata := decision.Metadata
				assert.Contains(t, metadata, "quality_score")
				assert.Contains(t, metadata, "price_score")
				assert.Contains(t, metadata, "capacity_score")
				assert.Contains(t, metadata, "weights")
			}
		})
	}
}

func TestCostBasedRouter_WeightNormalization(t *testing.T) {
	tests := []struct {
		name                   string
		qualityWeight          float64
		priceWeight            float64
		capacityWeight         float64
		expectedQualityWeight  float64
		expectedPriceWeight    float64
		expectedCapacityWeight float64
	}{
		{
			name:                   "normalizes non-unit weights",
			qualityWeight:          2.0,
			priceWeight:            3.0,
			capacityWeight:         5.0,
			expectedQualityWeight:  0.2,
			expectedPriceWeight:    0.3,
			expectedCapacityWeight: 0.5,
		},
		{
			name:                   "handles zero weights with defaults",
			qualityWeight:          0,
			priceWeight:            0,
			capacityWeight:         0,
			expectedQualityWeight:  0.33,
			expectedPriceWeight:    0.33,
			expectedCapacityWeight: 0.34,
		},
		{
			name:                   "preserves unit weights",
			qualityWeight:          0.4,
			priceWeight:            0.4,
			capacityWeight:         0.2,
			expectedQualityWeight:  0.4,
			expectedPriceWeight:    0.4,
			expectedCapacityWeight: 0.2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := NewCostBasedRouter(tt.qualityWeight, tt.priceWeight, tt.capacityWeight)

			assert.InDelta(t, tt.expectedQualityWeight, router.qualityWeight, 0.01)
			assert.InDelta(t, tt.expectedPriceWeight, router.priceWeight, 0.01)
			assert.InDelta(t, tt.expectedCapacityWeight, router.capacityWeight, 0.01)
		})
	}
}

func BenchmarkRoundRobinRouter_Route(b *testing.B) {
	ctx := context.Background()
	router := NewRoundRobinRouter()

	call := &call.Call{
		ID:     uuid.New(),
		Status: call.StatusPending,
	}

	bids := make([]*bid.Bid, 100)
	for i := 0; i < 100; i++ {
		bids[i] = &bid.Bid{
			ID:      uuid.New(),
			Status:  bid.StatusActive,
			BuyerID: uuid.New(),
		}
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = router.Route(ctx, call, bids)
	}
}

func BenchmarkCostBasedRouter_Route(b *testing.B) {
	ctx := context.Background()
	router := NewCostBasedRouter(0.4, 0.4, 0.2)

	call := &call.Call{
		ID:     uuid.New(),
		Status: call.StatusPending,
	}

	bids := make([]*bid.Bid, 100)
	for i := 0; i < 100; i++ {
		bids[i] = &bid.Bid{
			ID:      uuid.New(),
			Status:  bid.StatusActive,
			BuyerID: uuid.New(),
			Amount:  values.MustNewMoneyFromFloat(float64(i%50)+1.0, "USD"),
			Quality: values.QualityMetrics{
				ConversionRate:   float64(i%20) / 100.0,
				AverageCallTime:  150 + i%100,
				FraudScore:       float64(i%10) / 100.0,
				HistoricalRating: 3.0 + float64(i%20)/10.0,
			},
		}
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = router.Route(ctx, call, bids)
	}
}
