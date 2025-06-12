package bidding

import (
	"context"
	"testing"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/bid"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock for testing
type mockNotificationService struct {
	mock.Mock
}

func (m *mockNotificationService) NotifyBidPlaced(ctx context.Context, bid *bid.Bid) error {
	args := m.Called(ctx, bid)
	return args.Error(0)
}

func (m *mockNotificationService) NotifyBidWon(ctx context.Context, bid *bid.Bid) error {
	args := m.Called(ctx, bid)
	return args.Error(0)
}

func (m *mockNotificationService) NotifyBidLost(ctx context.Context, bid *bid.Bid) error {
	args := m.Called(ctx, bid)
	return args.Error(0)
}

func (m *mockNotificationService) NotifyBidExpired(ctx context.Context, bid *bid.Bid) error {
	args := m.Called(ctx, bid)
	return args.Error(0)
}

func (m *mockNotificationService) NotifyAuctionStarted(ctx context.Context, callID uuid.UUID) error {
	args := m.Called(ctx, callID)
	return args.Error(0)
}

func (m *mockNotificationService) NotifyAuctionClosed(ctx context.Context, result any) error {
	args := m.Called(ctx, result)
	return args.Error(0)
}

type mockMetricsCollector struct {
	mock.Mock
}

func (m *mockMetricsCollector) RecordBidPlaced(ctx context.Context, bid *bid.Bid) {
	m.Called(ctx, bid)
}

func (m *mockMetricsCollector) RecordAuctionDuration(ctx context.Context, callID uuid.UUID, duration time.Duration) {
	m.Called(ctx, callID, duration)
}

func (m *mockMetricsCollector) RecordBidAmount(ctx context.Context, amount float64) {
	m.Called(ctx, amount)
}

func (m *mockMetricsCollector) RecordBidValidation(ctx context.Context, bidID uuid.UUID, valid bool, reason string) {
	m.Called(ctx, bidID, valid, reason)
}

func (m *mockMetricsCollector) RecordAuctionParticipants(ctx context.Context, callID uuid.UUID, count int) {
	m.Called(ctx, callID, count)
}

func TestInfrastructureServices_NotifyBidPlaced(t *testing.T) {
	// Test successful notification
	mockNotifier := new(mockNotificationService)
	mockMetrics := new(mockMetricsCollector)

	testBid := &bid.Bid{ID: uuid.New()}
	mockNotifier.On("NotifyBidPlaced", mock.Anything, testBid).Return(nil)

	facade := NewInfrastructureServices(mockNotifier, mockMetrics)
	err := facade.NotifyBidPlaced(context.Background(), testBid)

	assert.NoError(t, err)
	mockNotifier.AssertExpectations(t)
}

func TestInfrastructureServices_WithNilServices(t *testing.T) {
	// Test facade handles nil services gracefully
	facade := NewInfrastructureServices(nil, nil)

	err := facade.NotifyBidPlaced(context.Background(), &bid.Bid{})
	assert.NoError(t, err) // Should not panic
}

func TestInfrastructureServices_MetricsRecording(t *testing.T) {
	mockNotifier := new(mockNotificationService)
	mockMetrics := new(mockMetricsCollector)

	testBid := &bid.Bid{ID: uuid.New()}
	mockMetrics.On("RecordBidPlaced", mock.Anything, testBid).Return()

	facade := NewInfrastructureServices(mockNotifier, mockMetrics)
	facade.RecordBidPlaced(context.Background(), testBid)

	mockMetrics.AssertExpectations(t)
}

func TestInfrastructureServices_AuctionNotifications(t *testing.T) {
	mockNotifier := new(mockNotificationService)
	mockMetrics := new(mockMetricsCollector)

	callID := uuid.New()
	result := &AuctionResult{
		CallID:       callID,
		WinningBidID: uuid.New(),
		WinnerID:     uuid.New(),
		FinalAmount:  100.0,
	}

	mockNotifier.On("NotifyAuctionStarted", mock.Anything, callID).Return(nil)
	mockNotifier.On("NotifyAuctionClosed", mock.Anything, result).Return(nil)

	facade := NewInfrastructureServices(mockNotifier, mockMetrics)

	err := facade.NotifyAuctionStarted(context.Background(), callID)
	assert.NoError(t, err)

	err = facade.NotifyAuctionClosed(context.Background(), result)
	assert.NoError(t, err)

	mockNotifier.AssertExpectations(t)
}

func TestInfrastructureServices_ExtendedMetrics(t *testing.T) {
	mockNotifier := new(mockNotificationService)
	mockMetrics := new(mockMetricsCollector)

	bidID := uuid.New()
	callID := uuid.New()

	mockMetrics.On("RecordBidValidation", mock.Anything, bidID, true, "valid").Return()
	mockMetrics.On("RecordAuctionParticipants", mock.Anything, callID, 5).Return()

	facade := NewInfrastructureServices(mockNotifier, mockMetrics)

	facade.RecordBidValidation(context.Background(), bidID, true, "valid")
	facade.RecordAuctionParticipants(context.Background(), callID, 5)

	mockMetrics.AssertExpectations(t)
}
