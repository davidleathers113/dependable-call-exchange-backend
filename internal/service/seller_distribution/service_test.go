package seller_distribution

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/account"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/call"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
)

// Mock implementations for testing

type mockCallRepository struct {
	calls map[uuid.UUID]*call.Call
}

func newMockCallRepository() *mockCallRepository {
	return &mockCallRepository{
		calls: make(map[uuid.UUID]*call.Call),
	}
}

func (m *mockCallRepository) GetByID(ctx context.Context, id uuid.UUID) (*call.Call, error) {
	if c, exists := m.calls[id]; exists {
		return c, nil
	}
	return nil, assert.AnError
}

func (m *mockCallRepository) Update(ctx context.Context, c *call.Call) error {
	m.calls[c.ID] = c
	return nil
}

func (m *mockCallRepository) GetIncomingCalls(ctx context.Context, limit int) ([]*call.Call, error) {
	var calls []*call.Call
	count := 0
	for _, c := range m.calls {
		if c.Status == call.StatusPending && count < limit {
			calls = append(calls, c)
			count++
		}
	}
	return calls, nil
}

type mockAccountRepository struct {
	accounts   map[uuid.UUID]*account.Account
	capacities map[uuid.UUID]*SellerCapacity
}

func newMockAccountRepository() *mockAccountRepository {
	return &mockAccountRepository{
		accounts:   make(map[uuid.UUID]*account.Account),
		capacities: make(map[uuid.UUID]*SellerCapacity),
	}
}

func (m *mockAccountRepository) GetByID(ctx context.Context, id uuid.UUID) (*account.Account, error) {
	if acc, exists := m.accounts[id]; exists {
		return acc, nil
	}
	return nil, assert.AnError
}

func (m *mockAccountRepository) GetAvailableSellers(ctx context.Context, criteria *SellerCriteria) ([]*account.Account, error) {
	var sellers []*account.Account
	for _, acc := range m.accounts {
		if acc.Type == account.TypeSeller && acc.Status == account.StatusActive {
			// Apply quality filter
			if criteria.MinQuality > 0 && acc.QualityMetrics.OverallScore() < criteria.MinQuality {
				continue
			}
			sellers = append(sellers, acc)
		}
	}
	return sellers, nil
}

func (m *mockAccountRepository) GetSellerCapacity(ctx context.Context, sellerID uuid.UUID) (*SellerCapacity, error) {
	if capacity, exists := m.capacities[sellerID]; exists {
		return capacity, nil
	}

	// Return default capacity if not found
	return &SellerCapacity{
		SellerID:           sellerID,
		MaxConcurrentCalls: 5,
		CurrentCalls:       1,
		AvailableSlots:     4,
		LastUpdated:        time.Now(),
	}, nil
}

type mockNotificationService struct {
	notifications []notificationRecord
}

type notificationRecord struct {
	sellerID uuid.UUID
	callID   uuid.UUID
	notified bool
}

func newMockNotificationService() *mockNotificationService {
	return &mockNotificationService{
		notifications: make([]notificationRecord, 0),
	}
}

func (m *mockNotificationService) NotifyCallAvailable(ctx context.Context, sellerID uuid.UUID, callID uuid.UUID) error {
	m.notifications = append(m.notifications, notificationRecord{
		sellerID: sellerID,
		callID:   callID,
		notified: true,
	})
	return nil
}

func (m *mockNotificationService) NotifyAuctionStarted(ctx context.Context, sellerIDs []uuid.UUID, callID uuid.UUID, auctionDuration time.Duration) error {
	for _, sellerID := range sellerIDs {
		m.notifications = append(m.notifications, notificationRecord{
			sellerID: sellerID,
			callID:   callID,
			notified: true,
		})
	}
	return nil
}

// Helper function to create seller account with quality metrics
func createTestSeller(id uuid.UUID, qualityScore float64) *account.Account {
	qualityMetrics := values.MustNewQualityMetrics(
		qualityScore, // quality score
		0.0,          // fraud score
		qualityScore, // historical rating
		0.5,          // conversion rate
		300,          // average call time
		qualityScore, // trust score
		qualityScore, // reliability score
	)

	return &account.Account{
		ID:             id,
		Type:           account.TypeSeller,
		Status:         account.StatusActive,
		QualityMetrics: qualityMetrics,
	}
}

// Test functions

func TestService_DistributeCall_BroadcastAlgorithm(t *testing.T) {
	// Setup mocks
	callRepo := newMockCallRepository()
	accountRepo := newMockAccountRepository()
	notificationSvc := newMockNotificationService()
	metrics := NewNoopMetrics()

	// Create test data
	callID := uuid.New()
	testCall := &call.Call{
		ID:        callID,
		Status:    call.StatusPending,
		Direction: call.DirectionInbound,
	}
	callRepo.calls[callID] = testCall

	// Create test sellers with quality metrics
	seller1ID := uuid.New()
	seller1 := createTestSeller(seller1ID, 9.0)
	accountRepo.accounts[seller1ID] = seller1

	seller2ID := uuid.New()
	seller2 := createTestSeller(seller2ID, 8.0)
	accountRepo.accounts[seller2ID] = seller2

	// Setup rules
	rules := &SellerDistributionRules{
		Algorithm:       "broadcast",
		MaxSellers:      10,
		MinQualityScore: 7.0,
		AuctionDuration: 5 * time.Minute,
	}

	// Create service
	service := NewService(callRepo, accountRepo, notificationSvc, metrics, rules)

	// Test distribution
	ctx := context.Background()
	decision, err := service.DistributeCall(ctx, callID)

	// Assertions
	require.NoError(t, err)
	require.NotNil(t, decision)

	assert.Equal(t, callID, decision.CallID)
	assert.Equal(t, "broadcast", decision.Algorithm)
	assert.Len(t, decision.SelectedSellers, 2)
	assert.Equal(t, 2, decision.NotifiedCount)
	assert.Contains(t, decision.SelectedSellers, seller1ID)
	assert.Contains(t, decision.SelectedSellers, seller2ID)

	// Verify call status was updated
	updatedCall, err := callRepo.GetByID(ctx, callID)
	require.NoError(t, err)
	assert.Equal(t, call.StatusQueued, updatedCall.Status)

	// Verify notifications were sent (2 individual + 2 auction started)
	assert.Len(t, notificationSvc.notifications, 4)
}

func TestService_DistributeCall_NoSellersAvailable(t *testing.T) {
	// Setup mocks with no sellers
	callRepo := newMockCallRepository()
	accountRepo := newMockAccountRepository()
	notificationSvc := newMockNotificationService()
	metrics := NewNoopMetrics()

	// Create test call
	callID := uuid.New()
	testCall := &call.Call{
		ID:        callID,
		Status:    call.StatusPending,
		Direction: call.DirectionInbound,
	}
	callRepo.calls[callID] = testCall

	// Setup rules
	rules := &SellerDistributionRules{
		Algorithm:       "broadcast",
		MaxSellers:      10,
		MinQualityScore: 7.0,
		AuctionDuration: 5 * time.Minute,
	}

	// Create service
	service := NewService(callRepo, accountRepo, notificationSvc, metrics, rules)

	// Test distribution
	ctx := context.Background()
	decision, err := service.DistributeCall(ctx, callID)

	// Assertions
	require.Error(t, err)
	assert.Nil(t, decision)
	assert.Contains(t, err.Error(), "no sellers available")
}

func TestService_DistributeCall_InvalidCallStatus(t *testing.T) {
	// Setup mocks
	callRepo := newMockCallRepository()
	accountRepo := newMockAccountRepository()
	notificationSvc := newMockNotificationService()
	metrics := NewNoopMetrics()

	// Create test call with invalid status
	callID := uuid.New()
	testCall := &call.Call{
		ID:        callID,
		Status:    call.StatusCompleted, // Invalid for distribution
		Direction: call.DirectionInbound,
	}
	callRepo.calls[callID] = testCall

	// Setup rules
	rules := &SellerDistributionRules{
		Algorithm:       "broadcast",
		MaxSellers:      10,
		MinQualityScore: 7.0,
		AuctionDuration: 5 * time.Minute,
	}

	// Create service
	service := NewService(callRepo, accountRepo, notificationSvc, metrics, rules)

	// Test distribution
	ctx := context.Background()
	decision, err := service.DistributeCall(ctx, callID)

	// Assertions
	require.Error(t, err)
	assert.Nil(t, decision)
	assert.Contains(t, err.Error(), "must be in pending status")
}

func TestService_GetAvailableSellers(t *testing.T) {
	// Setup mocks
	callRepo := newMockCallRepository()
	accountRepo := newMockAccountRepository()
	notificationSvc := newMockNotificationService()
	metrics := NewNoopMetrics()

	// Create test sellers with different quality scores
	seller1ID := uuid.New()
	seller1 := createTestSeller(seller1ID, 9.0) // High quality
	accountRepo.accounts[seller1ID] = seller1

	seller2ID := uuid.New()
	seller2 := createTestSeller(seller2ID, 6.0) // Below min quality
	accountRepo.accounts[seller2ID] = seller2

	// Setup rules
	rules := &SellerDistributionRules{
		Algorithm:       "broadcast",
		MaxSellers:      10,
		MinQualityScore: 7.0,
		AuctionDuration: 5 * time.Minute,
	}

	// Create service
	service := NewService(callRepo, accountRepo, notificationSvc, metrics, rules)

	// Test getting available sellers
	ctx := context.Background()
	criteria := &SellerCriteria{
		MinQuality:   7.0,
		AvailableNow: true,
	}

	sellers, err := service.GetAvailableSellers(ctx, criteria)

	// Assertions
	require.NoError(t, err)
	assert.Len(t, sellers, 1) // Only seller1 meets quality requirement
	assert.Equal(t, seller1ID, sellers[0].ID)
	assert.True(t, sellers[0].QualityMetrics.OverallScore() >= 7.0)
}
