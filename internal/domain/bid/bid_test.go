package bid_test

import (
	"testing"
	"time"
	
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/bid"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/testutil/fixtures"
)

func TestNewBid(t *testing.T) {
	tests := []struct {
		name      string
		callID    uuid.UUID
		buyerID   uuid.UUID
		sellerID  uuid.UUID
		amount    float64
		criteria  bid.BidCriteria
		validate  func(t *testing.T, b *bid.Bid)
	}{
		{
			name:     "creates bid with valid data",
			callID:   uuid.New(),
			buyerID:  uuid.New(),
			sellerID: uuid.New(),
			amount:   10.50,
			criteria: bid.BidCriteria{
				Geography: bid.GeoCriteria{
					States: []string{"CA", "TX"},
				},
				CallType:  []string{"inbound"},
				MaxBudget: 100.00,
			},
			validate: func(t *testing.T, b *bid.Bid) {
				assert.NotEqual(t, uuid.Nil, b.ID)
				assert.Equal(t, 10.50, b.Amount)
				assert.Equal(t, bid.StatusPending, b.Status)
				assert.NotZero(t, b.PlacedAt)
				assert.NotZero(t, b.ExpiresAt)
				assert.True(t, b.ExpiresAt.After(b.PlacedAt))
				assert.Nil(t, b.AcceptedAt)
				assert.Equal(t, 2, len(b.Criteria.Geography.States))
			},
		},
		{
			name:     "creates bid with complex criteria",
			callID:   uuid.New(),
			buyerID:  uuid.New(),
			sellerID: uuid.New(),
			amount:   25.00,
			criteria: fixtures.NewCriteriaBuilder().
				WithGeography([]string{"US"}, []string{"CA", "NY", "TX"}, []string{"Los Angeles", "New York"}).
				WithTimeWindow(8, 20, []string{"Mon", "Tue", "Wed", "Thu", "Fri"}, "America/New_York").
				WithCallTypes("inbound", "transfer").
				WithKeywords("insurance", "auto", "quote").
				WithMaxBudget(500.00).
				Build(),
			validate: func(t *testing.T, b *bid.Bid) {
				assert.Equal(t, 25.00, b.Amount)
				assert.Equal(t, 3, len(b.Criteria.Geography.States))
				assert.Equal(t, 2, len(b.Criteria.Geography.Cities))
				assert.Equal(t, 8, b.Criteria.TimeWindow.StartHour)
				assert.Equal(t, 20, b.Criteria.TimeWindow.EndHour)
				assert.Equal(t, 3, len(b.Criteria.Keywords))
			},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := bid.NewBid(tt.callID, tt.buyerID, tt.sellerID, tt.amount, tt.criteria)
			require.NoError(t, err)
			require.NotNil(t, b)
			tt.validate(t, b)
		})
	}
}

func TestBid_Accept(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() *bid.Bid
		validate func(t *testing.T, b *bid.Bid, oldUpdatedAt time.Time)
	}{
		{
			name: "accepts pending bid",
			setup: func() *bid.Bid {
				return fixtures.NewBidBuilder(t).
					WithStatus(bid.StatusPending).
					Build()
			},
			validate: func(t *testing.T, b *bid.Bid, oldUpdatedAt time.Time) {
				assert.Equal(t, bid.StatusWon, b.Status)
				assert.NotNil(t, b.AcceptedAt)
				assert.True(t, b.UpdatedAt.After(oldUpdatedAt))
				assert.True(t, b.AcceptedAt.After(b.PlacedAt))
			},
		},
		{
			name: "accepts active bid",
			setup: func() *bid.Bid {
				return fixtures.NewBidBuilder(t).
					WithStatus(bid.StatusActive).
					Build()
			},
			validate: func(t *testing.T, b *bid.Bid, oldUpdatedAt time.Time) {
				assert.Equal(t, bid.StatusWon, b.Status)
				assert.NotNil(t, b.AcceptedAt)
			},
		},
		{
			name: "accepts winning bid",
			setup: func() *bid.Bid {
				return fixtures.NewBidBuilder(t).
					WithStatus(bid.StatusWinning).
					WithRank(1).
					Build()
			},
			validate: func(t *testing.T, b *bid.Bid, oldUpdatedAt time.Time) {
				assert.Equal(t, bid.StatusWon, b.Status)
				assert.Equal(t, 1, b.Rank)
			},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := tt.setup()
			oldUpdatedAt := b.UpdatedAt
			
			time.Sleep(10 * time.Millisecond)
			b.Accept()
			
			tt.validate(t, b, oldUpdatedAt)
		})
	}
}

func TestBid_Reject(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() *bid.Bid
		validate func(t *testing.T, b *bid.Bid)
	}{
		{
			name: "rejects active bid",
			setup: func() *bid.Bid {
				return fixtures.NewBidBuilder(t).
					WithStatus(bid.StatusActive).
					Build()
			},
			validate: func(t *testing.T, b *bid.Bid) {
				assert.Equal(t, bid.StatusLost, b.Status)
				assert.Nil(t, b.AcceptedAt)
			},
		},
		{
			name: "rejects winning bid",
			setup: func() *bid.Bid {
				return fixtures.NewBidBuilder(t).
					WithStatus(bid.StatusWinning).
					Build()
			},
			validate: func(t *testing.T, b *bid.Bid) {
				assert.Equal(t, bid.StatusLost, b.Status)
			},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := tt.setup()
			oldUpdatedAt := b.UpdatedAt
			
			time.Sleep(10 * time.Millisecond)
			b.Reject()
			
			tt.validate(t, b)
			assert.True(t, b.UpdatedAt.After(oldUpdatedAt))
		})
	}
}

func TestStatus_String(t *testing.T) {
	tests := []struct {
		status   bid.Status
		expected string
	}{
		{bid.StatusPending, "pending"},
		{bid.StatusActive, "active"},
		{bid.StatusWinning, "winning"},
		{bid.StatusWon, "won"},
		{bid.StatusLost, "lost"},
		{bid.StatusExpired, "expired"},
		{bid.StatusCanceled, "canceled"},
		{bid.Status(999), "unknown"},
	}
	
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.status.String())
		})
	}
}

func TestBid_Expiration(t *testing.T) {
	t.Run("default expiration is 5 minutes", func(t *testing.T) {
		b, err := bid.NewBid(uuid.New(), uuid.New(), uuid.New(), 10.00, bid.BidCriteria{})
		require.NoError(t, err)
		
		expectedExpiry := b.PlacedAt.Add(5 * time.Minute)
		assert.WithinDuration(t, expectedExpiry, b.ExpiresAt, time.Second)
	})
	
	t.Run("custom expiration time", func(t *testing.T) {
		b := fixtures.NewBidBuilder(t).
			WithExpiration(10 * time.Minute).
			Build()
		
		expectedExpiry := b.PlacedAt.Add(10 * time.Minute)
		assert.Equal(t, expectedExpiry, b.ExpiresAt)
	})
	
	t.Run("expired bid detection", func(t *testing.T) {
		b := fixtures.NewBidBuilder(t).
			WithExpiration(-1 * time.Minute). // Already expired
			Build()
		
		assert.True(t, time.Now().After(b.ExpiresAt))
	})
}

func TestBid_QualityMetrics(t *testing.T) {
	t.Run("high quality bid", func(t *testing.T) {
		b := fixtures.NewBidBuilder(t).
			WithQuality(bid.QualityMetrics{
				ConversionRate:   0.30,
				AverageCallTime:  300,
				FraudScore:       0.01,
				HistoricalRating: 4.9,
			}).
			Build()
		
		assert.Equal(t, 0.30, b.Quality.ConversionRate)
		assert.Equal(t, 300, b.Quality.AverageCallTime)
		assert.Equal(t, 0.01, b.Quality.FraudScore)
		assert.Equal(t, 4.9, b.Quality.HistoricalRating)
	})
	
	t.Run("quality affects ranking", func(t *testing.T) {
		highQuality := fixtures.NewBidBuilder(t).
			WithAmount(10.00).
			WithQuality(bid.QualityMetrics{
				ConversionRate:   0.25,
				FraudScore:       0.02,
				HistoricalRating: 4.8,
			}).
			Build()
		
		lowQuality := fixtures.NewBidBuilder(t).
			WithAmount(12.00). // Higher bid amount
			WithQuality(bid.QualityMetrics{
				ConversionRate:   0.05,
				FraudScore:       0.20,
				HistoricalRating: 2.5,
			}).
			Build()
		
		// In a real auction, high quality might win despite lower amount
		assert.Greater(t, highQuality.Quality.ConversionRate, lowQuality.Quality.ConversionRate)
		assert.Less(t, highQuality.Quality.FraudScore, lowQuality.Quality.FraudScore)
	})
}

func TestBid_GeographicTargeting(t *testing.T) {
	t.Run("country level targeting", func(t *testing.T) {
		b := fixtures.NewBidBuilder(t).
			WithCriteria(bid.BidCriteria{
				Geography: bid.GeoCriteria{
					Countries: []string{"US", "CA", "MX"},
				},
			}).
			Build()
		
		assert.Equal(t, 3, len(b.Criteria.Geography.Countries))
		assert.Contains(t, b.Criteria.Geography.Countries, "US")
	})
	
	t.Run("state level targeting", func(t *testing.T) {
		b := fixtures.NewBidBuilder(t).
			WithCriteria(bid.BidCriteria{
				Geography: bid.GeoCriteria{
					States: []string{"CA", "TX", "FL", "NY"},
				},
			}).
			Build()
		
		assert.Equal(t, 4, len(b.Criteria.Geography.States))
	})
	
	t.Run("city level targeting with radius", func(t *testing.T) {
		radius := 50.0
		b := fixtures.NewBidBuilder(t).
			WithCriteria(bid.BidCriteria{
				Geography: bid.GeoCriteria{
					Cities: []string{"Los Angeles", "San Francisco"},
					Radius: &radius,
				},
			}).
			Build()
		
		assert.Equal(t, 2, len(b.Criteria.Geography.Cities))
		assert.NotNil(t, b.Criteria.Geography.Radius)
		assert.Equal(t, 50.0, *b.Criteria.Geography.Radius)
	})
}

func TestBid_TimeWindowTargeting(t *testing.T) {
	t.Run("business hours targeting", func(t *testing.T) {
		b := fixtures.NewBidBuilder(t).
			WithCriteria(bid.BidCriteria{
				TimeWindow: bid.TimeWindow{
					StartHour: 9,
					EndHour:   17,
					Days:      []string{"Mon", "Tue", "Wed", "Thu", "Fri"},
					Timezone:  "America/New_York",
				},
			}).
			Build()
		
		assert.Equal(t, 9, b.Criteria.TimeWindow.StartHour)
		assert.Equal(t, 17, b.Criteria.TimeWindow.EndHour)
		assert.Equal(t, 5, len(b.Criteria.TimeWindow.Days))
		assert.NotContains(t, b.Criteria.TimeWindow.Days, "Sat")
		assert.NotContains(t, b.Criteria.TimeWindow.Days, "Sun")
	})
	
	t.Run("24/7 targeting", func(t *testing.T) {
		b := fixtures.NewBidBuilder(t).
			WithCriteria(bid.BidCriteria{
				TimeWindow: bid.TimeWindow{
					StartHour: 0,
					EndHour:   23,
					Days:      []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"},
					Timezone:  "UTC",
				},
			}).
			Build()
		
		assert.Equal(t, 0, b.Criteria.TimeWindow.StartHour)
		assert.Equal(t, 23, b.Criteria.TimeWindow.EndHour)
		assert.Equal(t, 7, len(b.Criteria.TimeWindow.Days))
	})
}

func TestBid_Scenarios(t *testing.T) {
	scenarios := fixtures.NewBidScenarios(t)
	callID := uuid.New()
	
	t.Run("high value bid", func(t *testing.T) {
		b := scenarios.HighValueBid(callID)
		assert.Equal(t, 25.00, b.Amount)
		assert.Greater(t, b.Quality.ConversionRate, 0.20)
		assert.Contains(t, b.Criteria.Keywords, "premium")
	})
	
	t.Run("low value bid", func(t *testing.T) {
		b := scenarios.LowValueBid(callID)
		assert.Equal(t, 2.50, b.Amount)
		assert.Less(t, b.Quality.ConversionRate, 0.10)
		assert.Equal(t, "America/Chicago", b.Criteria.TimeWindow.Timezone)
	})
	
	t.Run("expired bid", func(t *testing.T) {
		b := scenarios.ExpiredBid(callID)
		assert.Equal(t, bid.StatusExpired, b.Status)
		assert.True(t, time.Now().After(b.ExpiresAt))
	})
	
	t.Run("winning bid", func(t *testing.T) {
		b := scenarios.WinningBid(callID)
		assert.Equal(t, bid.StatusWon, b.Status)
		assert.NotNil(t, b.AcceptedAt)
	})
	
	t.Run("competing bids", func(t *testing.T) {
		bids := scenarios.CompetingBids(callID, 5)
		assert.Len(t, bids, 5)
		
		// Verify amounts increase
		for i := 1; i < len(bids); i++ {
			assert.Greater(t, bids[i].Amount, bids[i-1].Amount)
		}
		
		// All bids should be for the same call
		for _, b := range bids {
			assert.Equal(t, callID, b.CallID)
		}
	})
}

func TestNewAuction(t *testing.T) {
	tests := []struct {
		name         string
		callID       uuid.UUID
		reservePrice float64
		validate     func(t *testing.T, a *bid.Auction)
	}{
		{
			name:         "creates auction with reserve price",
			callID:       uuid.New(),
			reservePrice: 5.00,
			validate: func(t *testing.T, a *bid.Auction) {
				assert.NotEqual(t, uuid.Nil, a.ID)
				assert.Equal(t, 5.00, a.ReservePrice)
				assert.Equal(t, bid.AuctionStatusPending, a.Status)
				assert.Equal(t, 0.01, a.BidIncrement)
				assert.Equal(t, 30, a.MaxDuration)
				assert.Nil(t, a.WinningBid)
				assert.Empty(t, a.Bids)
			},
		},
		{
			name:         "auction timing is correct",
			callID:       uuid.New(),
			reservePrice: 10.00,
			validate: func(t *testing.T, a *bid.Auction) {
				expectedEnd := a.StartTime.Add(30 * time.Second)
				assert.Equal(t, expectedEnd, a.EndTime)
				assert.True(t, a.EndTime.After(a.StartTime))
			},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, err := bid.NewAuction(tt.callID, tt.reservePrice)
			require.NoError(t, err)
			require.NotNil(t, a)
			tt.validate(t, a)
		})
	}
}

func TestAuctionStatus_String(t *testing.T) {
	tests := []struct {
		status   bid.AuctionStatus
		expected string
	}{
		{bid.AuctionStatusPending, "pending"},
		{bid.AuctionStatusActive, "active"},
		{bid.AuctionStatusCompleted, "completed"},
		{bid.AuctionStatusCanceled, "canceled"},
		{bid.AuctionStatusExpired, "expired"},
		{bid.AuctionStatus(999), "unknown"},
	}
	
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.status.String())
		})
	}
}

func TestBid_EdgeCases(t *testing.T) {
	t.Run("zero amount bid", func(t *testing.T) {
		b := fixtures.NewBidBuilder(t).
			WithAmount(0.00).
			Build()
		
		assert.Equal(t, 0.00, b.Amount)
	})
	
	t.Run("very high amount bid", func(t *testing.T) {
		b := fixtures.NewBidBuilder(t).
			WithAmount(999999.99).
			Build()
		
		assert.Equal(t, 999999.99, b.Amount)
	})
	
	t.Run("accepting already won bid", func(t *testing.T) {
		b := fixtures.NewBidBuilder(t).
			WithStatus(bid.StatusWon).
			Build()
		
		firstAcceptedAt := time.Now()
		b.AcceptedAt = &firstAcceptedAt
		
		time.Sleep(10 * time.Millisecond)
		b.Accept()
		
		// Should update the accepted time
		assert.NotEqual(t, firstAcceptedAt, *b.AcceptedAt)
		assert.True(t, b.AcceptedAt.After(firstAcceptedAt))
	})
	
	t.Run("rejecting already lost bid", func(t *testing.T) {
		b := fixtures.NewBidBuilder(t).
			WithStatus(bid.StatusLost).
			Build()
		
		oldUpdatedAt := b.UpdatedAt
		
		time.Sleep(10 * time.Millisecond)
		b.Reject()
		
		assert.Equal(t, bid.StatusLost, b.Status)
		assert.True(t, b.UpdatedAt.After(oldUpdatedAt))
	})
}

func TestBid_ConcurrentModifications(t *testing.T) {
	t.Run("concurrent accept and reject", func(t *testing.T) {
		b := fixtures.NewBidBuilder(t).
			WithStatus(bid.StatusActive).
			Build()
		
		done := make(chan bool, 2)
		
		go func() {
			b.Accept()
			done <- true
		}()
		
		go func() {
			b.Reject()
			done <- true
		}()
		
		<-done
		<-done
		
		// One status should have won
		assert.Contains(t, []bid.Status{bid.StatusWon, bid.StatusLost}, b.Status)
	})
}

func TestBid_Performance(t *testing.T) {
	t.Run("bid creation performance", func(t *testing.T) {
		criteria := fixtures.NewCriteriaBuilder().
			WithGeography([]string{"US"}, []string{"CA", "TX", "NY"}, nil).
			WithCallTypes("inbound").
			Build()
		
		start := time.Now()
		count := 10000
		
		for i := 0; i < count; i++ {
			_, _ = bid.NewBid(uuid.New(), uuid.New(), uuid.New(), 10.00, criteria)
		}
		
		elapsed := time.Since(start)
		perBid := elapsed / time.Duration(count)
		
		assert.Less(t, perBid, 20*time.Microsecond,
			"Bid creation took %v per bid, expected < 20µs", perBid)
	})
}