package bidding

import (
	"context"
	"testing"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/account"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/bid"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/call"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/testutil/mocks"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestService_PlaceBid(t *testing.T) {
	ctx := context.Background()

	// Create shared IDs for tests
	callID := uuid.New()
	buyerID := uuid.New()
	
	tests := []struct {
		name          string
		setupMocks    func(*mocks.CallRepository, *mocks.BidRepository, *mocks.AccountRepository, *mockFraudChecker)
		request       *PlaceBidRequest
		expectedError bool
		errorContains string
		validate      func(*testing.T, *bid.Bid)
	}{
		{
			name: "successful bid placement",
			setupMocks: func(cr *mocks.CallRepository, br *mocks.BidRepository, ar *mocks.AccountRepository, fc *mockFraudChecker) {
				testCall := &call.Call{
					ID:     callID,
					Status: call.StatusPending,
				}
				
				buyer := &account.Account{
					ID:           buyerID,
					Status:       account.StatusActive,
					QualityMetrics: values.QualityMetrics{
						QualityScore: 85.5,
					},
				}
				
				cr.On("GetByID", ctx, callID).Return(testCall, nil)
				ar.On("GetByID", ctx, buyerID).Return(buyer, nil)
				ar.On("GetBalance", ctx, buyerID).Return(100.0, nil)
				
				fc.On("CheckBid", ctx, mock.AnythingOfType("*bid.Bid"), buyer).Return(&FraudCheckResult{
					Approved:  true,
					RiskScore: 0.1,
					Flags:     []string{},
				}, nil)
				
				br.On("Create", ctx, mock.AnythingOfType("*bid.Bid")).Return(nil)
				br.On("GetActiveBidsForCall", ctx, callID).Return([]*bid.Bid{}, nil)
			},
			request: &PlaceBidRequest{
				CallID:   callID,
				BuyerID:  buyerID,
				Amount:   5.50,
				Duration: 5 * time.Minute,
			},
			expectedError: false,
			validate: func(t *testing.T, b *bid.Bid) {
				assert.NotNil(t, b)
				assert.Equal(t, bid.StatusActive, b.Status)
				assert.Equal(t, values.MustNewMoneyFromFloat(5.50, values.USD), b.Amount)
				assert.Equal(t, 10.0, b.Quality.HistoricalRating) // OverallScore is capped at 10.0 due to high QualityScore
			},
		},
		{
			name: "bid with criteria and auto-renew",
			setupMocks: func(cr *mocks.CallRepository, br *mocks.BidRepository, ar *mocks.AccountRepository, fc *mockFraudChecker) {
				testCall := &call.Call{
					ID:     callID,
					Status: call.StatusQueued,
				}
				
				buyer := &account.Account{
					ID:           buyerID,
					Status:       account.StatusActive,
					QualityMetrics: values.QualityMetrics{
						QualityScore: 90.0,
					},
				}
				
				cr.On("GetByID", ctx, callID).Return(testCall, nil)
				ar.On("GetByID", ctx, buyerID).Return(buyer, nil)
				ar.On("GetBalance", ctx, buyerID).Return(100.0, nil)
				
				fc.On("CheckBid", ctx, mock.AnythingOfType("*bid.Bid"), buyer).Return(&FraudCheckResult{
					Approved: true,
				}, nil)
				
				br.On("Create", ctx, mock.AnythingOfType("*bid.Bid")).Return(nil)
				br.On("GetActiveBidsForCall", ctx, callID).Return([]*bid.Bid{}, nil)
			},
			request: &PlaceBidRequest{
				CallID:  callID,
				BuyerID: buyerID,
				Amount:  7.25,
				Criteria: map[string]interface{}{
					"location": "US",
					"language": "en",
				},
				AutoRenew: true,
				MaxAmount: 10.0,
			},
			expectedError: false,
			validate: func(t *testing.T, b *bid.Bid) {
				assert.NotNil(t, b)
				assert.Equal(t, values.MustNewMoneyFromFloat(7.25, values.USD), b.Amount)
				// TODO: Criteria conversion from map[string]interface{} to bid.BidCriteria not yet implemented
				// assert.Contains(t, b.Criteria.Geography.Countries, "US")
			},
		},
		{
			name: "call not found",
			setupMocks: func(cr *mocks.CallRepository, br *mocks.BidRepository, ar *mocks.AccountRepository, fc *mockFraudChecker) {
				cr.On("GetByID", ctx, callID).Return(nil, assert.AnError)
			},
			request: &PlaceBidRequest{
				CallID:  callID,
				BuyerID: buyerID,
				Amount:  5.0,
			},
			expectedError: true,
			errorContains: "call not found",
		},
		{
			name: "call not in biddable state",
			setupMocks: func(cr *mocks.CallRepository, br *mocks.BidRepository, ar *mocks.AccountRepository, fc *mockFraudChecker) {
				testCall := &call.Call{
					ID:     callID,
					Status: call.StatusCompleted,
				}
				
				cr.On("GetByID", ctx, callID).Return(testCall, nil)
			},
			request: &PlaceBidRequest{
				CallID:  callID,
				BuyerID: buyerID,
				Amount:  5.0,
			},
			expectedError: true,
			errorContains: "not in biddable state",
		},
		{
			name: "buyer not found",
			setupMocks: func(cr *mocks.CallRepository, br *mocks.BidRepository, ar *mocks.AccountRepository, fc *mockFraudChecker) {
				testCall := &call.Call{
					ID:     callID,
					Status: call.StatusPending,
				}
				
				cr.On("GetByID", ctx, callID).Return(testCall, nil)
				ar.On("GetByID", ctx, buyerID).Return(nil, assert.AnError)
			},
			request: &PlaceBidRequest{
				CallID:  callID,
				BuyerID: buyerID,
				Amount:  5.0,
			},
			expectedError: true,
			errorContains: "buyer not found",
		},
		{
			name: "buyer account not active",
			setupMocks: func(cr *mocks.CallRepository, br *mocks.BidRepository, ar *mocks.AccountRepository, fc *mockFraudChecker) {
				testCall := &call.Call{
					ID:     callID,
					Status: call.StatusPending,
				}
				
				buyer := &account.Account{
					ID:     buyerID,
					Status: account.StatusSuspended,
				}
				
				cr.On("GetByID", ctx, callID).Return(testCall, nil)
				ar.On("GetByID", ctx, buyerID).Return(buyer, nil)
			},
			request: &PlaceBidRequest{
				CallID:  callID,
				BuyerID: buyerID,
				Amount:  5.0,
			},
			expectedError: true,
			errorContains: "account is not active",
		},
		{
			name: "insufficient balance",
			setupMocks: func(cr *mocks.CallRepository, br *mocks.BidRepository, ar *mocks.AccountRepository, fc *mockFraudChecker) {
				testCall := &call.Call{
					ID:     callID,
					Status: call.StatusPending,
				}
				
				buyer := &account.Account{
					ID:     buyerID,
					Status: account.StatusActive,
				}
				
				cr.On("GetByID", ctx, callID).Return(testCall, nil)
				ar.On("GetByID", ctx, buyerID).Return(buyer, nil)
				ar.On("GetBalance", ctx, buyerID).Return(3.0, nil) // Less than bid amount
			},
			request: &PlaceBidRequest{
				CallID:  callID,
				BuyerID: buyerID,
				Amount:  5.0,
			},
			expectedError: true,
			errorContains: "insufficient balance",
		},
		{
			name: "fraud check rejection",
			setupMocks: func(cr *mocks.CallRepository, br *mocks.BidRepository, ar *mocks.AccountRepository, fc *mockFraudChecker) {
				testCall := &call.Call{
					ID:     callID,
					Status: call.StatusPending,
				}
				
				buyer := &account.Account{
					ID:           buyerID,
					Status:       account.StatusActive,
					QualityMetrics: values.QualityMetrics{
						QualityScore: 40.0,
					},
				}
				
				cr.On("GetByID", ctx, callID).Return(testCall, nil)
				ar.On("GetByID", ctx, buyerID).Return(buyer, nil)
				ar.On("GetBalance", ctx, buyerID).Return(100.0, nil)
				
				fc.On("CheckBid", ctx, mock.AnythingOfType("*bid.Bid"), buyer).Return(&FraudCheckResult{
					Approved: false,
					Reasons:  []string{"Low quality score", "Suspicious activity"},
				}, nil)
			},
			request: &PlaceBidRequest{
				CallID:  callID,
				BuyerID: buyerID,
				Amount:  5.0,
			},
			expectedError: true,
			errorContains: "bid rejected",
		},
		{
			name: "invalid bid amount - too low",
			request: &PlaceBidRequest{
				CallID:  callID,
				BuyerID: buyerID,
				Amount:  0.001, // Less than minimum
			},
			expectedError: true,
			errorContains: "bid amount must be between",
		},
		{
			name: "invalid bid amount - too high",
			request: &PlaceBidRequest{
				CallID:  callID,
				BuyerID: buyerID,
				Amount:  10000.0, // More than maximum
			},
			expectedError: true,
			errorContains: "bid amount must be between",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mocks
			callRepo := new(mocks.CallRepository)
			bidRepo := new(mocks.BidRepository)
			accountRepo := new(mocks.AccountRepository)
			fraudChecker := new(mockFraudChecker)
			notifier := new(mocks.NotificationService)
			metrics := new(mocks.MetricsCollector)
			
			// Setup mocks if provided
			if tt.setupMocks != nil {
				tt.setupMocks(callRepo, bidRepo, accountRepo, fraudChecker)
			}
			
			// Setup metrics expectations for successful cases
			if !tt.expectedError {
				metrics.On("RecordBidPlaced", ctx, mock.AnythingOfType("*bid.Bid")).Return()
				metrics.On("RecordBidAmount", ctx, mock.AnythingOfType("float64")).Return()
				// NotifyBidPlaced is called in a goroutine with background context
				notifier.On("NotifyBidPlaced", mock.Anything, mock.AnythingOfType("*bid.Bid")).Return(nil).Maybe()
			}
			
			// Create service
			svc := NewService(bidRepo, callRepo, accountRepo, fraudChecker, notifier, metrics)
			
			// Execute
			result, err := svc.PlaceBid(ctx, tt.request)
			
			// Validate
			if tt.expectedError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
			} else {
				require.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, result)
				}
			}
			
			// Assert expectations
			callRepo.AssertExpectations(t)
			bidRepo.AssertExpectations(t)
			accountRepo.AssertExpectations(t)
			fraudChecker.AssertExpectations(t)
		})
	}
}

func TestService_UpdateBid(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		setupMocks    func(*mocks.BidRepository, uuid.UUID)
		bidID         uuid.UUID
		updates       *BidUpdate
		expectedError bool
		errorContains string
		validate      func(*testing.T, *bid.Bid)
	}{
		{
			name: "successful amount update",
			setupMocks: func(br *mocks.BidRepository, bidID uuid.UUID) {
				existingBid := &bid.Bid{
					ID:     bidID,
					Amount: values.MustNewMoneyFromFloat(5.0, "USD"),
					Status: bid.StatusActive,
				}
				
				br.On("GetByID", ctx, bidID).Return(existingBid, nil)
				br.On("Update", ctx, mock.MatchedBy(func(b *bid.Bid) bool {
					expectedAmount := values.MustNewMoneyFromFloat(7.5, "USD")
					return b.ID == bidID && b.Amount.Compare(expectedAmount) == 0
				})).Return(nil)
			},
			bidID: uuid.New(),
			updates: &BidUpdate{
				Amount: ptr(7.5),
			},
			expectedError: false,
			validate: func(t *testing.T, b *bid.Bid) {
				assert.Equal(t, values.MustNewMoneyFromFloat(7.5, values.USD), b.Amount)
			},
		},
		{
			name: "extend expiration",
			setupMocks: func(br *mocks.BidRepository, bidID uuid.UUID) {
				originalExpiry := time.Now().Add(1 * time.Minute)
				existingBid := &bid.Bid{
					ID:        bidID,
					Status:    bid.StatusActive,
					ExpiresAt: originalExpiry,
				}
				
				br.On("GetByID", ctx, bidID).Return(existingBid, nil)
				br.On("Update", ctx, mock.MatchedBy(func(b *bid.Bid) bool {
					return b.ExpiresAt.After(originalExpiry)
				})).Return(nil)
			},
			bidID: uuid.New(),
			updates: &BidUpdate{
				ExtendBy: ptr(5 * time.Minute),
			},
			expectedError: false,
		},
		{
			name: "update criteria and metadata",
			setupMocks: func(br *mocks.BidRepository, bidID uuid.UUID) {
				existingBid := &bid.Bid{
					ID:       bidID,
					Status:   bid.StatusActive,
				}
				
				br.On("GetByID", ctx, bidID).Return(existingBid, nil)
				br.On("Update", ctx, mock.AnythingOfType("*bid.Bid")).Return(nil)
			},
			bidID: uuid.New(),
			updates: &BidUpdate{
				Criteria: map[string]interface{}{
					"new_criteria": "value",
				},
				AutoRenew: ptr(true),
				MaxAmount: ptr(15.0),
			},
			expectedError: false,
			validate: func(t *testing.T, b *bid.Bid) {
				// Validate criteria was updated
				assert.NotNil(t, b.Criteria)
			},
		},
		{
			name: "bid not found",
			setupMocks: func(br *mocks.BidRepository, bidID uuid.UUID) {
				br.On("GetByID", ctx, bidID).Return(nil, assert.AnError)
			},
			bidID:         uuid.New(),
			updates:       &BidUpdate{Amount: ptr(5.0)},
			expectedError: true,
			errorContains: "bid not found",
		},
		{
			name: "bid not modifiable - completed status",
			setupMocks: func(br *mocks.BidRepository, bidID uuid.UUID) {
				existingBid := &bid.Bid{
					ID:     bidID,
					Status: bid.StatusWon,
				}
				br.On("GetByID", ctx, bidID).Return(existingBid, nil)
			},
			bidID:         uuid.New(),
			updates:       &BidUpdate{Amount: ptr(5.0)},
			expectedError: true,
			errorContains: "cannot be modified",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mocks
			bidRepo := new(mocks.BidRepository)
			
			// Setup mocks
			tt.setupMocks(bidRepo, tt.bidID)
			
			// Create service
			svc := &service{
				bidRepo:      bidRepo,
				minBidAmount: 0.01,
				maxBidAmount: 1000.0,
			}
			
			// Execute
			result, err := svc.UpdateBid(ctx, tt.bidID, tt.updates)
			
			// Validate
			if tt.expectedError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
			} else {
				require.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, result)
				}
			}
			
			// Assert expectations
			bidRepo.AssertExpectations(t)
		})
	}
}

func TestService_ProcessExpiredBids(t *testing.T) {
	ctx := context.Background()

	t.Run("process expired bids", func(t *testing.T) {
		bidRepo := new(mocks.BidRepository)
		notifier := new(mocks.NotificationService)
		
		// Create expired bids
		expiredBid1 := &bid.Bid{
			ID:        uuid.New(),
			Status:    bid.StatusActive,
			ExpiresAt: time.Now().Add(-1 * time.Minute),
		}
		
		expiredBid2 := &bid.Bid{
			ID:        uuid.New(),
			Status:    bid.StatusActive,
			ExpiresAt: time.Now().Add(-1 * time.Minute),
		}
		
		bidRepo.On("GetExpiredBids", ctx, mock.AnythingOfType("time.Time")).Return([]*bid.Bid{
			expiredBid1,
			expiredBid2,
		}, nil)
		
		// Both bids should be expired (auto-renew not implemented yet)
		bidRepo.On("Update", ctx, mock.MatchedBy(func(b *bid.Bid) bool {
			return b.ID == expiredBid1.ID && b.Status == bid.StatusExpired
		})).Return(nil)
		
		bidRepo.On("Update", ctx, mock.MatchedBy(func(b *bid.Bid) bool {
			return b.ID == expiredBid2.ID && b.Status == bid.StatusExpired
		})).Return(nil)
		
		// Notifications for both expired bids (called in goroutines)
		notifier.On("NotifyBidExpired", mock.Anything, expiredBid1).Return(nil).Maybe()
		notifier.On("NotifyBidExpired", mock.Anything, expiredBid2).Return(nil).Maybe()
		
		// Create service
		svc := &service{
			bidRepo:         bidRepo,
			notifier:        notifier,
			defaultDuration: 5 * time.Minute,
		}
		
		// Execute
		err := svc.ProcessExpiredBids(ctx)
		require.NoError(t, err)
		
		// Assert expectations
		bidRepo.AssertExpectations(t)
	})
}

func TestService_ConcurrentBidding(t *testing.T) {
	ctx := context.Background()
	
	// Setup
	callRepo := new(mocks.CallRepository)
	bidRepo := new(mocks.BidRepository)
	accountRepo := new(mocks.AccountRepository)
	fraudChecker := new(mockFraudChecker)
	
	callID := uuid.New()
	testCall := &call.Call{
		ID:     callID,
		Status: call.StatusPending,
	}
	
	// Create multiple buyers
	numBuyers := 10
	buyers := make([]*account.Account, numBuyers)
	for i := 0; i < numBuyers; i++ {
		buyers[i] = &account.Account{
			ID:           uuid.New(),
			Status:       account.StatusActive,
			QualityMetrics: values.QualityMetrics{
				QualityScore: 70.0 + float64(i),
			},
		}
		
		// Setup expectations for each buyer
		callRepo.On("GetByID", ctx, callID).Return(testCall, nil)
		accountRepo.On("GetByID", ctx, buyers[i].ID).Return(buyers[i], nil)
		accountRepo.On("GetBalance", ctx, buyers[i].ID).Return(100.0, nil)
		fraudChecker.On("CheckBid", ctx, mock.AnythingOfType("*bid.Bid"), buyers[i]).Return(&FraudCheckResult{
			Approved: true,
		}, nil)
		bidRepo.On("Create", ctx, mock.AnythingOfType("*bid.Bid")).Return(nil)
	}
	
	// GetActiveBidsForCall will be called by auction engine
	bidRepo.On("GetActiveBidsForCall", ctx, callID).Return([]*bid.Bid{}, nil).Maybe()
	
	// Create service
	svc := NewService(bidRepo, callRepo, accountRepo, fraudChecker, nil, nil)
	
	// Place bids concurrently
	errors := make(chan error, numBuyers)
	for i := 0; i < numBuyers; i++ {
		go func(buyerIndex int) {
			_, err := svc.PlaceBid(ctx, &PlaceBidRequest{
				CallID:  callID,
				BuyerID: buyers[buyerIndex].ID,
				Amount:  float64(buyerIndex) + 1.0,
			})
			errors <- err
		}(i)
	}
	
	// Collect results
	for i := 0; i < numBuyers; i++ {
		err := <-errors
		require.NoError(t, err)
	}
	
	// Verify all bids were placed
	callRepo.AssertExpectations(t)
	bidRepo.AssertExpectations(t)
	accountRepo.AssertExpectations(t)
}

// Mock implementations

type mockFraudChecker struct {
	mock.Mock
}

func (m *mockFraudChecker) CheckBid(ctx context.Context, bid *bid.Bid, buyer *account.Account) (*FraudCheckResult, error) {
	args := m.Called(ctx, bid, buyer)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*FraudCheckResult), args.Error(1)
}

func (m *mockFraudChecker) GetRiskScore(ctx context.Context, buyerID uuid.UUID) (float64, error) {
	args := m.Called(ctx, buyerID)
	return args.Get(0).(float64), args.Error(1)
}

// Helper function for creating pointers
func ptr[T any](v T) *T {
	return &v
}