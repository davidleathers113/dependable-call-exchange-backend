package mocks

import (
	"context"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/account"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/bid"
	"github.com/google/uuid"
)

// MockBiddingNotificationService implements notification interface without importing bidding package
type MockBiddingNotificationService struct{}

func NewMockBiddingNotificationService() *MockBiddingNotificationService {
	return &MockBiddingNotificationService{}
}

func (m *MockBiddingNotificationService) NotifyBidPlaced(ctx context.Context, bid *bid.Bid) error {
	return nil
}

func (m *MockBiddingNotificationService) NotifyBidWon(ctx context.Context, bid *bid.Bid) error {
	return nil
}

func (m *MockBiddingNotificationService) NotifyBidLost(ctx context.Context, bid *bid.Bid) error {
	return nil
}

func (m *MockBiddingNotificationService) NotifyBidExpired(ctx context.Context, bid *bid.Bid) error {
	return nil
}

func (m *MockBiddingNotificationService) NotifyAuctionStarted(ctx context.Context, callID uuid.UUID) error {
	return nil
}

func (m *MockBiddingNotificationService) NotifyAuctionClosed(ctx context.Context, result any) error {
	return nil
}

// MockBiddingMetricsCollector implements metrics interface without importing bidding package
type MockBiddingMetricsCollector struct{}

func NewMockBiddingMetricsCollector() *MockBiddingMetricsCollector {
	return &MockBiddingMetricsCollector{}
}

func (m *MockBiddingMetricsCollector) RecordBidPlaced(ctx context.Context, bid *bid.Bid) {}

func (m *MockBiddingMetricsCollector) RecordAuctionDuration(ctx context.Context, callID uuid.UUID, duration time.Duration) {
}

func (m *MockBiddingMetricsCollector) RecordBidAmount(ctx context.Context, amount float64) {}

func (m *MockBiddingMetricsCollector) RecordBidValidation(ctx context.Context, bidID uuid.UUID, valid bool, reason string) {
}

func (m *MockBiddingMetricsCollector) RecordAuctionParticipants(ctx context.Context, callID uuid.UUID, count int) {
}

// MockBiddingFraudCheckResult represents fraud check result without importing bidding package
type MockBiddingFraudCheckResult struct {
	Approved    bool
	RiskScore   float64
	Reasons     []string
	Flags       []string
	RequiresMFA bool
}

// MockBiddingFraudChecker implements fraud checker interface without importing bidding package
type MockBiddingFraudChecker struct{}

func NewMockBiddingFraudChecker() *MockBiddingFraudChecker {
	return &MockBiddingFraudChecker{}
}

func (m *MockBiddingFraudChecker) CheckBid(ctx context.Context, bid *bid.Bid, buyer *account.Account) (*MockBiddingFraudCheckResult, error) {
	return &MockBiddingFraudCheckResult{
		Approved:    true,
		RiskScore:   0.1,
		Reasons:     []string{},
		Flags:       []string{},
		RequiresMFA: false,
	}, nil
}

func (m *MockBiddingFraudChecker) GetRiskScore(ctx context.Context, buyerID uuid.UUID) (float64, error) {
	return 0.1, nil
}
