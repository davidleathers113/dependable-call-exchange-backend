package callrouting

import (
	"context"
	"testing"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/bid"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/call"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRoundRobinRouter(t *testing.T) {
	ctx := context.Background()
	router := NewRoundRobinRouter()

	testCall := &call.Call{
		ID:     uuid.New(),
		Status: call.StatusPending,
	}

	tests := []struct {
		name          string
		bids          []*bid.Bid
		expectedError bool
		expectedIndex int
	}{
		{
			name: "routes to first bid initially",
			bids: []*bid.Bid{
				{ID: uuid.New(), Status: bid.StatusActive, BuyerID: uuid.New()},
				{ID: uuid.New(), Status: bid.StatusActive, BuyerID: uuid.New()},
			},
			expectedError: false,
			expectedIndex: 0,
		},
		{
			name: "routes in round-robin fashion",
			bids: []*bid.Bid{
				{ID: uuid.New(), Status: bid.StatusActive, BuyerID: uuid.New()},
				{ID: uuid.New(), Status: bid.StatusActive, BuyerID: uuid.New()},
				{ID: uuid.New(), Status: bid.StatusActive, BuyerID: uuid.New()},
			},
			expectedError: false,
			expectedIndex: 1, // Second call should go to index 1
		},
		{
			name:          "errors on empty bids",
			bids:          []*bid.Bid{},
			expectedError: true,
		},
		{
			name: "errors when no active bids",
			bids: []*bid.Bid{
				{ID: uuid.New(), Status: bid.StatusExpired, BuyerID: uuid.New()},
				{ID: uuid.New(), Status: bid.StatusWon, BuyerID: uuid.New()},
			},
			expectedError: true,
		},
		{
			name: "skips inactive bids",
			bids: []*bid.Bid{
				{ID: uuid.New(), Status: bid.StatusExpired, BuyerID: uuid.New()},
				{ID: uuid.New(), Status: bid.StatusActive, BuyerID: uuid.New()},
				{ID: uuid.New(), Status: bid.StatusActive, BuyerID: uuid.New()},
			},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision, err := router.Route(ctx, testCall, tt.bids)
			
			if tt.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, decision)
				assert.Equal(t, "round-robin", decision.Algorithm)
				assert.Equal(t, 1.0, decision.Score)
				assert.NotZero(t, decision.BidID)
				assert.NotZero(t, decision.BuyerID)
				
				// Check metadata
				metadata := decision.Metadata
				assert.Contains(t, metadata, "index")
				assert.Contains(t, metadata, "total")
			}
		})
	}
}

func TestSkillBasedRouter(t *testing.T) {
	ctx := context.Background()
	
	skillWeights := map[string]float64{
		"sales":    2.0,
		"support":  1.5,
		"billing":  1.0,
	}
	
	router := NewSkillBasedRouter(skillWeights)

	tests := []struct {
		name            string
		call            *call.Call
		bids            []*bid.Bid
		expectedError   bool
		expectedWinner  int // Index of expected winning bid
		expectedMinScore float64
	}{
		{
			name: "selects bid with best skill match",
			call: &call.Call{
				ID:     uuid.New(),
				Status: call.StatusPending,
				Metadata: map[string]interface{}{
					"required_skills": []string{"sales", "support"},
				},
			},
			bids: []*bid.Bid{
				{
					ID: uuid.New(), Status: bid.StatusActive, BuyerID: uuid.New(),
					Criteria: map[string]interface{}{
						"skills": []string{"sales"},
					},
				},
				{
					ID: uuid.New(), Status: bid.StatusActive, BuyerID: uuid.New(),
					Criteria: map[string]interface{}{
						"skills": []string{"sales", "support"},
					},
				},
				{
					ID: uuid.New(), Status: bid.StatusActive, BuyerID: uuid.New(),
					Criteria: map[string]interface{}{
						"skills": []string{"billing"},
					},
				},
			},
			expectedError:   false,
			expectedWinner:  1, // Bid with both skills
			expectedMinScore: 1.0,
		},
		{
			name: "handles weighted skills correctly",
			call: &call.Call{
				ID:     uuid.New(),
				Status: call.StatusPending,
				Metadata: map[string]interface{}{
					"required_skills": []string{"sales", "billing"},
				},
			},
			bids: []*bid.Bid{
				{
					ID: uuid.New(), Status: bid.StatusActive, BuyerID: uuid.New(),
					Criteria: map[string]interface{}{
						"skills": []string{"billing"}, // weight 1.0
					},
				},
				{
					ID: uuid.New(), Status: bid.StatusActive, BuyerID: uuid.New(),
					Criteria: map[string]interface{}{
						"skills": []string{"sales"}, // weight 2.0
					},
				},
			},
			expectedError:   false,
			expectedWinner:  1, // Sales has higher weight
			expectedMinScore: 0.5,
		},
		{
			name: "handles call without required skills",
			call: &call.Call{
				ID:       uuid.New(),
				Status:   call.StatusPending,
				Metadata: map[string]interface{}{},
			},
			bids: []*bid.Bid{
				{ID: uuid.New(), Status: bid.StatusActive, BuyerID: uuid.New()},
			},
			expectedError: false,
			expectedMinScore: 1.0, // Default score
		},
		{
			name: "handles bid without skills",
			call: &call.Call{
				ID:     uuid.New(),
				Status: call.StatusPending,
				Metadata: map[string]interface{}{
					"required_skills": []string{"sales"},
				},
			},
			bids: []*bid.Bid{
				{
					ID: uuid.New(), Status: bid.StatusActive, BuyerID: uuid.New(),
					Criteria: map[string]interface{}{},
				},
			},
			expectedError: false,
			expectedMinScore: 0.5, // Partial score
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision, err := router.Route(ctx, tt.call, tt.bids)
			
			if tt.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, decision)
				assert.Equal(t, "skill-based", decision.Algorithm)
				assert.GreaterOrEqual(t, decision.Score, tt.expectedMinScore)
				
				if tt.expectedWinner >= 0 {
					assert.Equal(t, tt.bids[tt.expectedWinner].ID, decision.BidID)
				}
			}
		})
	}
}

func TestCostBasedRouter(t *testing.T) {
	ctx := context.Background()
	
	router := NewCostBasedRouter(0.4, 0.4, 0.2) // quality, price, capacity

	testCall := &call.Call{
		ID:     uuid.New(),
		Status: call.StatusPending,
	}

	tests := []struct {
		name            string
		bids            []*bid.Bid
		expectedError   bool
		expectedWinner  int
		expectedMinScore float64
	}{
		{
			name: "balances quality, price, and capacity",
			bids: []*bid.Bid{
				{
					ID: uuid.New(), Status: bid.StatusActive, BuyerID: uuid.New(),
					Amount: 10.0, QualityScore: 60,
					Criteria: map[string]interface{}{
						"available_capacity": 200.0,
					},
				},
				{
					ID: uuid.New(), Status: bid.StatusActive, BuyerID: uuid.New(),
					Amount: 5.0, QualityScore: 90,
					Criteria: map[string]interface{}{
						"available_capacity": 500.0,
					},
				},
				{
					ID: uuid.New(), Status: bid.StatusActive, BuyerID: uuid.New(),
					Amount: 7.0, QualityScore: 80,
					Criteria: map[string]interface{}{
						"available_capacity": 800.0,
					},
				},
			},
			expectedError:   false,
			expectedWinner:  1, // Best balance
			expectedMinScore: 0.5,
		},
		{
			name: "handles missing capacity data",
			bids: []*bid.Bid{
				{
					ID: uuid.New(), Status: bid.StatusActive, BuyerID: uuid.New(),
					Amount: 5.0, QualityScore: 80,
					Criteria: map[string]interface{}{},
				},
			},
			expectedError: false,
			expectedMinScore: 0.3, // Should still calculate with default capacity
		},
		{
			name: "normalizes weights correctly",
			bids: []*bid.Bid{
				{
					ID: uuid.New(), Status: bid.StatusActive, BuyerID: uuid.New(),
					Amount: 5.0, QualityScore: 100,
					Criteria: map[string]interface{}{
						"available_capacity": 1000.0,
					},
				},
			},
			expectedError: false,
			expectedMinScore: 0.8, // High score expected
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision, err := router.Route(ctx, testCall, tt.bids)
			
			if tt.expectedError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, decision)
				assert.Equal(t, "cost-based", decision.Algorithm)
				assert.GreaterOrEqual(t, decision.Score, tt.expectedMinScore)
				
				// Verify metadata
				metadata := decision.Metadata
				assert.Contains(t, metadata, "quality_score")
				assert.Contains(t, metadata, "price_score")
				assert.Contains(t, metadata, "capacity_score")
				assert.Contains(t, metadata, "weights")
				
				if tt.expectedWinner >= 0 {
					assert.Equal(t, tt.bids[tt.expectedWinner].ID, decision.BidID)
				}
			}
		})
	}
}

func TestCostBasedRouter_EdgeCases(t *testing.T) {
	ctx := context.Background()
	testCall := &call.Call{
		ID:     uuid.New(),
		Status: call.StatusPending,
	}

	t.Run("handles zero weights", func(t *testing.T) {
		router := NewCostBasedRouter(0, 0, 0)
		// Should normalize to equal weights
		assert.NotNil(t, router)
	})

	t.Run("handles same price for all bids", func(t *testing.T) {
		router := NewCostBasedRouter(0.4, 0.4, 0.2)
		
		bids := []*bid.Bid{
			{
				ID: uuid.New(), Status: bid.StatusActive, BuyerID: uuid.New(),
				Amount: 5.0, QualityScore: 70,
			},
			{
				ID: uuid.New(), Status: bid.StatusActive, BuyerID: uuid.New(),
				Amount: 5.0, QualityScore: 90,
			},
		}
		
		decision, err := router.Route(ctx, testCall, bids)
		require.NoError(t, err)
		
		// Should pick higher quality when price is same
		assert.Equal(t, bids[1].ID, decision.BidID)
	})
}

func BenchmarkRoundRobinRouter(b *testing.B) {
	ctx := context.Background()
	router := NewRoundRobinRouter()
	
	testCall := &call.Call{
		ID:     uuid.New(),
		Status: call.StatusPending,
	}
	
	// Create many bids
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
		_, _ = router.Route(ctx, testCall, bids)
	}
}

func BenchmarkSkillBasedRouter(b *testing.B) {
	ctx := context.Background()
	router := NewSkillBasedRouter(map[string]float64{
		"sales": 2.0,
		"support": 1.5,
	})
	
	testCall := &call.Call{
		ID:     uuid.New(),
		Status: call.StatusPending,
		Metadata: map[string]interface{}{
			"required_skills": []string{"sales", "support"},
		},
	}
	
	// Create many bids with varying skills
	bids := make([]*bid.Bid, 100)
	skills := [][]string{
		{"sales"},
		{"support"},
		{"sales", "support"},
		{"billing"},
		{"sales", "billing"},
	}
	
	for i := 0; i < 100; i++ {
		bids[i] = &bid.Bid{
			ID:      uuid.New(),
			Status:  bid.StatusActive,
			BuyerID: uuid.New(),
			Criteria: map[string]interface{}{
				"skills": skills[i%len(skills)],
			},
		}
	}
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		_, _ = router.Route(ctx, testCall, bids)
	}
}

func BenchmarkCostBasedRouter(b *testing.B) {
	ctx := context.Background()
	router := NewCostBasedRouter(0.4, 0.4, 0.2)
	
	testCall := &call.Call{
		ID:     uuid.New(),
		Status: call.StatusPending,
	}
	
	// Create many bids with varying attributes
	bids := make([]*bid.Bid, 100)
	for i := 0; i < 100; i++ {
		bids[i] = &bid.Bid{
			ID:           uuid.New(),
			Status:       bid.StatusActive,
			BuyerID:      uuid.New(),
			Amount:       float64(i%20) + 1.0,
			QualityScore: float64(50 + i%50),
			Criteria: map[string]interface{}{
				"available_capacity": float64(100 + i*10),
			},
		}
	}
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		_, _ = router.Route(ctx, testCall, bids)
	}
}