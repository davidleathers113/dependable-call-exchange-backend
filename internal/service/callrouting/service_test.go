package callrouting

import (
	"context"
	"testing"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/account"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/bid"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/call"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/testutil/mocks"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestService_RouteCall(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		setupMocks    func(*mocks.CallRepository, *mocks.BidRepository, *mocks.AccountRepository)
		callID        uuid.UUID
		rules         *RoutingRules
		expectedError bool
		errorContains string
		validate      func(*testing.T, *RoutingDecision)
	}{
		{
			name: "successful round-robin routing",
			setupMocks: func(cr *mocks.CallRepository, br *mocks.BidRepository, ar *mocks.AccountRepository) {
				callID := uuid.New()
				testCall := &call.Call{
					ID:     callID,
					Status: call.StatusPending,
				}
				
				bids := []*bid.Bid{
					{
						ID:           uuid.New(),
						CallID:       callID,
						BuyerID:      uuid.New(),
						Amount:       5.0,
						Status:       bid.StatusActive,
						QualityScore: 85,
					},
					{
						ID:           uuid.New(),
						CallID:       callID,
						BuyerID:      uuid.New(),
						Amount:       4.5,
						Status:       bid.StatusActive,
						QualityScore: 90,
					},
				}
				
				cr.On("GetByID", ctx, callID).Return(testCall, nil)
				br.On("GetActiveBidsForCall", ctx, callID).Return(bids, nil)
				cr.On("Update", ctx, testCall).Return(nil)
			},
			callID: uuid.New(),
			rules: &RoutingRules{
				Algorithm: "round-robin",
			},
			expectedError: false,
			validate: func(t *testing.T, decision *RoutingDecision) {
				assert.NotNil(t, decision)
				assert.Equal(t, "round-robin", decision.Algorithm)
				assert.Equal(t, 1.0, decision.Score)
				assert.NotZero(t, decision.BidID)
				assert.NotZero(t, decision.BuyerID)
			},
		},
		{
			name: "successful skill-based routing",
			setupMocks: func(cr *mocks.CallRepository, br *mocks.BidRepository, ar *mocks.AccountRepository) {
				callID := uuid.New()
				testCall := &call.Call{
					ID:     callID,
					Status: call.StatusPending,
					Metadata: map[string]interface{}{
						"required_skills": []string{"sales", "support"},
					},
				}
				
				bids := []*bid.Bid{
					{
						ID:           uuid.New(),
						CallID:       callID,
						BuyerID:      uuid.New(),
						Amount:       5.0,
						Status:       bid.StatusActive,
						QualityScore: 80,
						Criteria: map[string]interface{}{
							"skills": []string{"sales"},
						},
					},
					{
						ID:           uuid.New(),
						CallID:       callID,
						BuyerID:      uuid.New(),
						Amount:       4.5,
						Status:       bid.StatusActive,
						QualityScore: 85,
						Criteria: map[string]interface{}{
							"skills": []string{"sales", "support"},
						},
					},
				}
				
				cr.On("GetByID", ctx, callID).Return(testCall, nil)
				br.On("GetActiveBidsForCall", ctx, callID).Return(bids, nil)
				cr.On("Update", ctx, testCall).Return(nil)
			},
			callID: uuid.New(),
			rules: &RoutingRules{
				Algorithm: "skill-based",
			},
			expectedError: false,
			validate: func(t *testing.T, decision *RoutingDecision) {
				assert.NotNil(t, decision)
				assert.Equal(t, "skill-based", decision.Algorithm)
				assert.Greater(t, decision.Score, 0.5) // Should have higher score for better match
			},
		},
		{
			name: "successful cost-based routing",
			setupMocks: func(cr *mocks.CallRepository, br *mocks.BidRepository, ar *mocks.AccountRepository) {
				callID := uuid.New()
				testCall := &call.Call{
					ID:     callID,
					Status: call.StatusPending,
				}
				
				bids := []*bid.Bid{
					{
						ID:           uuid.New(),
						CallID:       callID,
						BuyerID:      uuid.New(),
						Amount:       5.0,
						Status:       bid.StatusActive,
						QualityScore: 70,
						Criteria: map[string]interface{}{
							"available_capacity": 500.0,
						},
					},
					{
						ID:           uuid.New(),
						CallID:       callID,
						BuyerID:      uuid.New(),
						Amount:       3.0,
						Status:       bid.StatusActive,
						QualityScore: 95,
						Criteria: map[string]interface{}{
							"available_capacity": 800.0,
						},
					},
				}
				
				cr.On("GetByID", ctx, callID).Return(testCall, nil)
				br.On("GetActiveBidsForCall", ctx, callID).Return(bids, nil)
				cr.On("Update", ctx, testCall).Return(nil)
			},
			callID: uuid.New(),
			rules: &RoutingRules{
				Algorithm:      "cost-based",
				QualityWeight:  0.4,
				PriceWeight:    0.4,
				CapacityWeight: 0.2,
			},
			expectedError: false,
			validate: func(t *testing.T, decision *RoutingDecision) {
				assert.NotNil(t, decision)
				assert.Equal(t, "cost-based", decision.Algorithm)
				metadata := decision.Metadata
				assert.Contains(t, metadata, "quality_score")
				assert.Contains(t, metadata, "price_score")
				assert.Contains(t, metadata, "capacity_score")
			},
		},
		{
			name: "call not found",
			setupMocks: func(cr *mocks.CallRepository, br *mocks.BidRepository, ar *mocks.AccountRepository) {
				callID := uuid.New()
				cr.On("GetByID", ctx, callID).Return(nil, assert.AnError)
			},
			callID:        uuid.New(),
			rules:         &RoutingRules{Algorithm: "round-robin"},
			expectedError: true,
			errorContains: "call not found",
		},
		{
			name: "call not in pending state",
			setupMocks: func(cr *mocks.CallRepository, br *mocks.BidRepository, ar *mocks.AccountRepository) {
				callID := uuid.New()
				testCall := &call.Call{
					ID:     callID,
					Status: call.StatusActive,
				}
				cr.On("GetByID", ctx, callID).Return(testCall, nil)
			},
			callID:        uuid.New(),
			rules:         &RoutingRules{Algorithm: "round-robin"},
			expectedError: true,
			errorContains: "not in pending state",
		},
		{
			name: "no bids available",
			setupMocks: func(cr *mocks.CallRepository, br *mocks.BidRepository, ar *mocks.AccountRepository) {
				callID := uuid.New()
				testCall := &call.Call{
					ID:     callID,
					Status: call.StatusPending,
				}
				cr.On("GetByID", ctx, callID).Return(testCall, nil)
				br.On("GetActiveBidsForCall", ctx, callID).Return([]*bid.Bid{}, nil)
			},
			callID:        uuid.New(),
			rules:         &RoutingRules{Algorithm: "round-robin"},
			expectedError: true,
			errorContains: "no bids available",
		},
		{
			name: "no active bids",
			setupMocks: func(cr *mocks.CallRepository, br *mocks.BidRepository, ar *mocks.AccountRepository) {
				callID := uuid.New()
				testCall := &call.Call{
					ID:     callID,
					Status: call.StatusPending,
				}
				
				bids := []*bid.Bid{
					{
						ID:      uuid.New(),
						CallID:  callID,
						BuyerID: uuid.New(),
						Status:  bid.StatusExpired,
					},
				}
				
				cr.On("GetByID", ctx, callID).Return(testCall, nil)
				br.On("GetActiveBidsForCall", ctx, callID).Return(bids, nil)
			},
			callID:        uuid.New(),
			rules:         &RoutingRules{Algorithm: "round-robin"},
			expectedError: true,
			errorContains: "no active bids",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mocks
			callRepo := new(mocks.CallRepository)
			bidRepo := new(mocks.BidRepository)
			accountRepo := new(mocks.AccountRepository)
			metrics := new(mocks.MetricsCollector)
			
			// Setup mocks
			tt.setupMocks(callRepo, bidRepo, accountRepo)
			
			// Create service
			svc := NewService(callRepo, bidRepo, accountRepo, metrics, tt.rules)
			
			// Execute
			decision, err := svc.RouteCall(ctx, tt.callID)
			
			// Validate
			if tt.expectedError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
			} else {
				require.NoError(t, err)
				tt.validate(t, decision)
			}
			
			// Assert expectations
			callRepo.AssertExpectations(t)
			bidRepo.AssertExpectations(t)
			accountRepo.AssertExpectations(t)
		})
	}
}

func TestService_UpdateRoutingRules(t *testing.T) {
	// Create service with initial rules
	svc := NewService(
		new(mocks.CallRepository),
		new(mocks.BidRepository),
		new(mocks.AccountRepository),
		nil,
		&RoutingRules{Algorithm: "round-robin"},
	)

	// Test successful update
	newRules := &RoutingRules{
		Algorithm:      "cost-based",
		QualityWeight:  0.5,
		PriceWeight:    0.3,
		CapacityWeight: 0.2,
	}
	
	err := svc.UpdateRoutingRules(context.Background(), newRules)
	require.NoError(t, err)
	
	// Verify router was updated (would need to expose router type or test behavior)
	s := svc.(*service)
	assert.Equal(t, "cost-based", s.router.GetAlgorithm())
	
	// Test nil rules
	err = svc.UpdateRoutingRules(context.Background(), nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rules cannot be nil")
}

func TestService_ConcurrentRouting(t *testing.T) {
	ctx := context.Background()
	
	// Setup mocks
	callRepo := new(mocks.CallRepository)
	bidRepo := new(mocks.BidRepository)
	accountRepo := new(mocks.AccountRepository)
	
	// Create multiple calls and bids
	numCalls := 10
	calls := make([]*call.Call, numCalls)
	for i := 0; i < numCalls; i++ {
		callID := uuid.New()
		calls[i] = &call.Call{
			ID:     callID,
			Status: call.StatusPending,
		}
		
		bids := []*bid.Bid{
			{
				ID:           uuid.New(),
				CallID:       callID,
				BuyerID:      uuid.New(),
				Amount:       5.0,
				Status:       bid.StatusActive,
				QualityScore: 85,
			},
		}
		
		callRepo.On("GetByID", ctx, callID).Return(calls[i], nil)
		bidRepo.On("GetActiveBidsForCall", ctx, callID).Return(bids, nil)
		callRepo.On("Update", ctx, calls[i]).Return(nil)
	}
	
	// Create service
	svc := NewService(
		callRepo,
		bidRepo,
		accountRepo,
		nil,
		&RoutingRules{Algorithm: "round-robin"},
	)
	
	// Route calls concurrently
	errors := make(chan error, numCalls)
	for i := 0; i < numCalls; i++ {
		go func(callID uuid.UUID) {
			_, err := svc.RouteCall(ctx, callID)
			errors <- err
		}(calls[i].ID)
	}
	
	// Collect results
	for i := 0; i < numCalls; i++ {
		err := <-errors
		require.NoError(t, err)
	}
	
	// Verify all calls were processed
	callRepo.AssertExpectations(t)
	bidRepo.AssertExpectations(t)
}

func BenchmarkService_RouteCall(b *testing.B) {
	ctx := context.Background()
	
	// Setup
	callRepo := new(mocks.CallRepository)
	bidRepo := new(mocks.BidRepository)
	accountRepo := new(mocks.AccountRepository)
	
	callID := uuid.New()
	testCall := &call.Call{
		ID:     callID,
		Status: call.StatusPending,
	}
	
	bids := make([]*bid.Bid, 100)
	for i := 0; i < 100; i++ {
		bids[i] = &bid.Bid{
			ID:           uuid.New(),
			CallID:       callID,
			BuyerID:      uuid.New(),
			Amount:       float64(i) + 1.0,
			Status:       bid.StatusActive,
			QualityScore: float64(50 + i%50),
			Criteria: map[string]interface{}{
				"available_capacity": float64(100 + i*10),
			},
		}
	}
	
	callRepo.On("GetByID", ctx, callID).Return(testCall, nil)
	bidRepo.On("GetActiveBidsForCall", ctx, callID).Return(bids, nil)
	callRepo.On("Update", ctx, testCall).Return(nil)
	
	svc := NewService(
		callRepo,
		bidRepo,
		accountRepo,
		nil,
		&RoutingRules{
			Algorithm:      "cost-based",
			QualityWeight:  0.4,
			PriceWeight:    0.4,
			CapacityWeight: 0.2,
		},
	)
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		_, _ = svc.RouteCall(ctx, callID)
	}
}