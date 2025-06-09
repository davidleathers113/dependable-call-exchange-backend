package callrouting

import (
	"context"
	"testing"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/bid"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/call"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/testutil/mocks"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestService_RouteCall(t *testing.T) {
	ctx := context.Background()

	// Generate test data
	testCallID1 := uuid.New()
	testCallID2 := uuid.New()
	testCallID3 := uuid.New()
	testCallID4 := uuid.New()
	testCallID5 := uuid.New()
	testCallID6 := uuid.New()
	testCallID7 := uuid.New()

	tests := []struct {
		name          string
		setupMocks    func(*mocks.CallRepository, *mocks.BidRepository, *mocks.AccountRepository, uuid.UUID)
		callID        uuid.UUID
		rules         *RoutingRules
		expectedError bool
		errorContains string
		validate      func(*testing.T, *RoutingDecision)
	}{
		{
			name: "successful round-robin routing",
			callID: testCallID1,
			setupMocks: func(cr *mocks.CallRepository, br *mocks.BidRepository, ar *mocks.AccountRepository, callID uuid.UUID) {
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
						Quality: bid.QualityMetrics{
							ConversionRate:   0.15,
							AverageCallTime:  180,
							FraudScore:       0.05,
							HistoricalRating: 4.5,
						},
					},
					{
						ID:           uuid.New(),
						CallID:       callID,
						BuyerID:      uuid.New(),
						Amount:       4.5,
						Status:       bid.StatusActive,
						Quality: bid.QualityMetrics{
							ConversionRate:   0.20,
							AverageCallTime:  160,
							FraudScore:       0.02,
							HistoricalRating: 4.8,
						},
					},
				}
				
				cr.On("GetByID", ctx, callID).Return(testCall, nil)
				br.On("GetActiveBidsForCall", ctx, callID).Return(bids, nil)
				cr.On("Update", ctx, testCall).Return(nil)
				
				// Mock GetBidByID for any bid ID (round-robin will select the first bid)
				br.On("GetBidByID", ctx, mock.Anything).Return(bids[0], nil)
				br.On("Update", ctx, mock.Anything).Return(nil)
			},
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
			callID: testCallID2,
			setupMocks: func(cr *mocks.CallRepository, br *mocks.BidRepository, ar *mocks.AccountRepository, callID uuid.UUID) {
				testCall := &call.Call{
					ID:        callID,
					Status:    call.StatusPending,
					Direction: call.DirectionInbound,
				}
				
				bids := []*bid.Bid{
					{
						ID:           uuid.New(),
						CallID:       callID,
						BuyerID:      uuid.New(),
						Amount:       5.0,
						Status:       bid.StatusActive,
						Criteria: bid.BidCriteria{
							CallType: []string{"outbound"}, // Won't match
						},
						Quality: bid.QualityMetrics{
							ConversionRate:   0.10,
							AverageCallTime:  200,
							FraudScore:       0.10,
							HistoricalRating: 3.5,
						},
					},
					{
						ID:           uuid.New(),
						CallID:       callID,
						BuyerID:      uuid.New(),
						Amount:       4.5,
						Status:       bid.StatusActive,
						Criteria: bid.BidCriteria{
							CallType: []string{"inbound"}, // Will match
						},
						Quality: bid.QualityMetrics{
							ConversionRate:   0.20,
							AverageCallTime:  180,
							FraudScore:       0.02,
							HistoricalRating: 4.8,
						},
					},
				}
				
				cr.On("GetByID", ctx, callID).Return(testCall, nil)
				br.On("GetActiveBidsForCall", ctx, callID).Return(bids, nil)
				cr.On("Update", ctx, testCall).Return(nil)
				
				// Mock GetBidByID for any bid ID (skill-based will select the second bid)
				br.On("GetBidByID", ctx, mock.Anything).Return(bids[1], nil)
				br.On("Update", ctx, mock.Anything).Return(nil)
			},
			rules: &RoutingRules{
				Algorithm: "skill-based",
			},
			expectedError: false,
			validate: func(t *testing.T, decision *RoutingDecision) {
				assert.NotNil(t, decision)
				assert.Equal(t, "skill-based", decision.Algorithm)
				assert.Greater(t, decision.Score, 0.0) // Should have a score based on quality
			},
		},
		{
			name: "successful cost-based routing",
			callID: testCallID3,
			setupMocks: func(cr *mocks.CallRepository, br *mocks.BidRepository, ar *mocks.AccountRepository, callID uuid.UUID) {
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
						Quality: bid.QualityMetrics{
							ConversionRate:   0.10,
							AverageCallTime:  200,
							FraudScore:       0.15,
							HistoricalRating: 3.0,
						},
					},
					{
						ID:           uuid.New(),
						CallID:       callID,
						BuyerID:      uuid.New(),
						Amount:       3.0,
						Status:       bid.StatusActive,
						Quality: bid.QualityMetrics{
							ConversionRate:   0.25,
							AverageCallTime:  180,
							FraudScore:       0.02,
							HistoricalRating: 4.8,
						},
					},
				}
				
				cr.On("GetByID", ctx, callID).Return(testCall, nil)
				br.On("GetActiveBidsForCall", ctx, callID).Return(bids, nil)
				cr.On("Update", ctx, testCall).Return(nil)
				
				// Mock GetBidByID for any bid ID
				br.On("GetBidByID", ctx, mock.Anything).Return(bids[0], nil)
				br.On("Update", ctx, mock.Anything).Return(nil)
			},
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
			callID: testCallID4,
			setupMocks: func(cr *mocks.CallRepository, br *mocks.BidRepository, ar *mocks.AccountRepository, callID uuid.UUID) {
				cr.On("GetByID", ctx, callID).Return(nil, assert.AnError)
			},
			rules:         &RoutingRules{Algorithm: "round-robin"},
			expectedError: true,
			errorContains: "not found",
		},
		{
			name: "call not in pending state",
			callID: testCallID5,
			setupMocks: func(cr *mocks.CallRepository, br *mocks.BidRepository, ar *mocks.AccountRepository, callID uuid.UUID) {
				testCall := &call.Call{
					ID:     callID,
					Status: call.StatusInProgress,
				}
				cr.On("GetByID", ctx, callID).Return(testCall, nil)
			},
			rules:         &RoutingRules{Algorithm: "round-robin"},
			expectedError: true,
			errorContains: "not in pending state",
		},
		{
			name: "no bids available",
			callID: testCallID6,
			setupMocks: func(cr *mocks.CallRepository, br *mocks.BidRepository, ar *mocks.AccountRepository, callID uuid.UUID) {
				testCall := &call.Call{
					ID:     callID,
					Status: call.StatusPending,
				}
				cr.On("GetByID", ctx, callID).Return(testCall, nil)
				br.On("GetActiveBidsForCall", ctx, callID).Return([]*bid.Bid{}, nil)
			},
			rules:         &RoutingRules{Algorithm: "round-robin"},
			expectedError: true,
			errorContains: "no bids available",
		},
		{
			name: "no active bids",
			callID: testCallID7,
			setupMocks: func(cr *mocks.CallRepository, br *mocks.BidRepository, ar *mocks.AccountRepository, callID uuid.UUID) {
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
			metrics := new(MockMetricsCollector)
			
			// Setup mocks
			tt.setupMocks(callRepo, bidRepo, accountRepo, tt.callID)
			
			// Setup metrics expectations if metrics is provided
			if !tt.expectedError {
				metrics.On("RecordRoutingDecision", ctx, mock.Anything).Return()
				metrics.On("RecordRoutingLatency", ctx, mock.Anything, mock.Anything).Return()
			}
			
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
	allBids := make(map[uuid.UUID][]*bid.Bid)
	
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
				Quality: bid.QualityMetrics{
					ConversionRate:   0.15,
					AverageCallTime:  180,
					FraudScore:       0.05,
					HistoricalRating: 4.5,
				},
			},
		}
		
		allBids[callID] = bids
		
		callRepo.On("GetByID", ctx, callID).Return(calls[i], nil)
		bidRepo.On("GetActiveBidsForCall", ctx, callID).Return(bids, nil)
		callRepo.On("Update", ctx, calls[i]).Return(nil)
	}
	
	// Set up generic mocks for GetBidByID and Update that work for any bid
	// We need to set up individual mocks for each bid since testify doesn't support dynamic returns
	for _, bids := range allBids {
		for _, b := range bids {
			bidCopy := b // Capture the bid in a closure
			bidRepo.On("GetBidByID", ctx, bidCopy.ID).Return(bidCopy, nil).Maybe()
		}
	}
	
	bidRepo.On("Update", ctx, mock.Anything).Return(nil)
	
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
			Quality: bid.QualityMetrics{
				ConversionRate:   float64(i%20) / 100.0,
				AverageCallTime:  150 + i%100,
				FraudScore:       float64(i%10) / 100.0,
				HistoricalRating: 3.0 + float64(i%20)/10.0,
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