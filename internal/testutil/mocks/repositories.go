package mocks

import (
	"context"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/call"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/bid"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/account"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

// CallRepository mock
type CallRepository struct {
	mock.Mock
}

func (m *CallRepository) Create(ctx context.Context, c *call.Call) error {
	args := m.Called(ctx, c)
	return args.Error(0)
}

func (m *CallRepository) GetByID(ctx context.Context, id uuid.UUID) (*call.Call, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*call.Call), args.Error(1)
}

func (m *CallRepository) Update(ctx context.Context, c *call.Call) error {
	args := m.Called(ctx, c)
	return args.Error(0)
}

func (m *CallRepository) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *CallRepository) List(ctx context.Context, filters map[string]interface{}) ([]*call.Call, error) {
	args := m.Called(ctx, filters)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*call.Call), args.Error(1)
}

func (m *CallRepository) GetByStatus(ctx context.Context, status call.Status) ([]*call.Call, error) {
	args := m.Called(ctx, status)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*call.Call), args.Error(1)
}

// BidRepository mock
type BidRepository struct {
	mock.Mock
}

func (m *BidRepository) Create(ctx context.Context, b *bid.Bid) error {
	args := m.Called(ctx, b)
	return args.Error(0)
}

func (m *BidRepository) GetByID(ctx context.Context, id uuid.UUID) (*bid.Bid, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*bid.Bid), args.Error(1)
}

func (m *BidRepository) Update(ctx context.Context, b *bid.Bid) error {
	args := m.Called(ctx, b)
	return args.Error(0)
}

func (m *BidRepository) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *BidRepository) GetActiveBidsForCall(ctx context.Context, callID uuid.UUID) ([]*bid.Bid, error) {
	args := m.Called(ctx, callID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*bid.Bid), args.Error(1)
}

func (m *BidRepository) GetByBuyer(ctx context.Context, buyerID uuid.UUID) ([]*bid.Bid, error) {
	args := m.Called(ctx, buyerID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*bid.Bid), args.Error(1)
}

func (m *BidRepository) CleanupExpiredBids(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// AccountRepository mock
type AccountRepository struct {
	mock.Mock
}

func (m *AccountRepository) Create(ctx context.Context, a *account.Account) error {
	args := m.Called(ctx, a)
	return args.Error(0)
}

func (m *AccountRepository) GetByID(ctx context.Context, id uuid.UUID) (*account.Account, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*account.Account), args.Error(1)
}

func (m *AccountRepository) GetByEmail(ctx context.Context, email string) (*account.Account, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*account.Account), args.Error(1)
}

func (m *AccountRepository) Update(ctx context.Context, a *account.Account) error {
	args := m.Called(ctx, a)
	return args.Error(0)
}

func (m *AccountRepository) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *AccountRepository) List(ctx context.Context, filters map[string]interface{}) ([]*account.Account, error) {
	args := m.Called(ctx, filters)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*account.Account), args.Error(1)
}

// MetricsCollector mock
type MetricsCollector struct {
	mock.Mock
}

func (m *MetricsCollector) RecordCallRouted(ctx context.Context, algorithm string, duration time.Duration) {
	m.Called(ctx, algorithm, duration)
}

func (m *MetricsCollector) RecordCallCompleted(ctx context.Context, callID uuid.UUID, duration int, cost float64) {
	m.Called(ctx, callID, duration, cost)
}

func (m *MetricsCollector) RecordBidPlaced(ctx context.Context, bidID uuid.UUID, amount float64) {
	m.Called(ctx, bidID, amount)
}

func (m *MetricsCollector) IncrementCounter(name string, tags map[string]string) {
	m.Called(name, tags)
}

func (m *MetricsCollector) RecordHistogram(name string, value float64, tags map[string]string) {
	m.Called(name, value, tags)
}

func (m *MetricsCollector) RecordGauge(name string, value float64, tags map[string]string) {
	m.Called(name, value, tags)
}

// NotificationService mock
type NotificationService struct {
	mock.Mock
}

func (m *NotificationService) SendCallRouted(ctx context.Context, callID uuid.UUID, buyerID uuid.UUID) error {
	args := m.Called(ctx, callID, buyerID)
	return args.Error(0)
}

func (m *NotificationService) SendCallCompleted(ctx context.Context, callID uuid.UUID, duration int, cost float64) error {
	args := m.Called(ctx, callID, duration, cost)
	return args.Error(0)
}

func (m *NotificationService) SendBidWon(ctx context.Context, bidID uuid.UUID, callID uuid.UUID) error {
	args := m.Called(ctx, bidID, callID)
	return args.Error(0)
}

func (m *NotificationService) SendBidLost(ctx context.Context, bidID uuid.UUID, callID uuid.UUID, reason string) error {
	args := m.Called(ctx, bidID, callID, reason)
	return args.Error(0)
}

// PaymentService mock  
type PaymentService struct {
	mock.Mock
}

func (m *PaymentService) ProcessPayment(ctx context.Context, accountID uuid.UUID, amount float64, callID uuid.UUID) error {
	args := m.Called(ctx, accountID, amount, callID)
	return args.Error(0)
}

func (m *PaymentService) RefundPayment(ctx context.Context, accountID uuid.UUID, amount float64, callID uuid.UUID) error {
	args := m.Called(ctx, accountID, amount, callID)
	return args.Error(0)
}

func (m *PaymentService) GetBalance(ctx context.Context, accountID uuid.UUID) (float64, error) {
	args := m.Called(ctx, accountID)
	return args.Get(0).(float64), args.Error(1)
}

// TelephonyService mock
type TelephonyService struct {
	mock.Mock
}

func (m *TelephonyService) InitiateCall(ctx context.Context, from, to string, callID uuid.UUID) (string, error) {
	args := m.Called(ctx, from, to, callID)
	return args.String(0), args.Error(1)
}

func (m *TelephonyService) HangupCall(ctx context.Context, callSID string) error {
	args := m.Called(ctx, callSID)
	return args.Error(0)
}

func (m *TelephonyService) GetCallStatus(ctx context.Context, callSID string) (string, error) {
	args := m.Called(ctx, callSID)
	return args.String(0), args.Error(1)
}

func (m *TelephonyService) RecordCall(ctx context.Context, callSID string, recordingURL string) error {
	args := m.Called(ctx, callSID, recordingURL)
	return args.Error(0)
}

// ComplianceService mock
type ComplianceService struct {
	mock.Mock
}

func (m *ComplianceService) ValidateCall(ctx context.Context, fromNumber, toNumber string) error {
	args := m.Called(ctx, fromNumber, toNumber)
	return args.Error(0)
}

func (m *ComplianceService) CheckConsent(ctx context.Context, phoneNumber string) (bool, error) {
	args := m.Called(ctx, phoneNumber)
	return args.Bool(0), args.Error(1)
}

func (m *ComplianceService) RecordConsent(ctx context.Context, phoneNumber string, consentType string) error {
	args := m.Called(ctx, phoneNumber, consentType)
	return args.Error(0)
}

func (m *ComplianceService) RevokeConsent(ctx context.Context, phoneNumber string) error {
	args := m.Called(ctx, phoneNumber)
	return args.Error(0)
}

// Helper methods to setup common mock behaviors

// WithDelay adds a delay to mock method calls (useful for testing timeouts)
func (m *CallRepository) WithDelay(duration time.Duration) *mock.Call {
	return m.On("GetByID").WaitUntil(time.After(duration))
}

// ExpectCallLifecycle sets up expectations for a typical call lifecycle
func (m *CallRepository) ExpectCallLifecycle(ctx context.Context, callID uuid.UUID) {
	testCall := &call.Call{
		ID:     callID,
		Status: call.StatusPending,
	}
	
	m.On("GetByID", ctx, callID).Return(testCall, nil)
	m.On("Update", ctx, mock.MatchedBy(func(c *call.Call) bool {
		return c.ID == callID && c.Status == call.StatusQueued
	})).Return(nil)
}

// ExpectActiveBids sets up expectations for active bids
func (m *BidRepository) ExpectActiveBids(ctx context.Context, callID uuid.UUID, bids []*bid.Bid) {
	m.On("GetActiveBidsForCall", ctx, callID).Return(bids, nil)
}

// ExpectBidUpdate sets up expectations for bid status updates
func (m *BidRepository) ExpectBidUpdate(ctx context.Context, bidID uuid.UUID, newStatus bid.Status) {
	m.On("Update", ctx, mock.MatchedBy(func(b *bid.Bid) bool {
		return b.ID == bidID && b.Status == newStatus
	})).Return(nil)
}

// Mock builders for fluent test setup

// CallMockBuilder provides fluent interface for setting up call mocks
type CallMockBuilder struct {
	repo *CallRepository
	call *call.Call
	ctx  context.Context
}

func NewCallMockBuilder(repo *CallRepository, ctx context.Context) *CallMockBuilder {
	return &CallMockBuilder{
		repo: repo,
		ctx:  ctx,
		call: &call.Call{
			ID:     uuid.New(),
			Status: call.StatusPending,
		},
	}
}

func (b *CallMockBuilder) WithStatus(status call.Status) *CallMockBuilder {
	b.call.Status = status
	return b
}

func (b *CallMockBuilder) WithID(id uuid.UUID) *CallMockBuilder {
	b.call.ID = id
	return b
}

func (b *CallMockBuilder) ExpectGet() *CallMockBuilder {
	b.repo.On("GetByID", b.ctx, b.call.ID).Return(b.call, nil)
	return b
}

func (b *CallMockBuilder) ExpectUpdate() *CallMockBuilder {
	b.repo.On("Update", b.ctx, b.call).Return(nil)
	return b
}

func (b *CallMockBuilder) Build() *call.Call {
	return b.call
}