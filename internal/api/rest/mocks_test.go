package rest

import (
	"context"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/account"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/bid"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/call"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/service/bidding"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/service/callrouting"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/service/fraud"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/service/marketplace"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/service/telephony"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

// SimpleMockServices holds all mock services with correct interfaces
type SimpleMockServices struct {
	CallRouting  *MockCallRoutingService
	Bidding      *MockBiddingService  
	Telephony    *MockTelephonyService
	Fraud        *MockFraudService
	Analytics    *MockAnalyticsService
	Marketplace  *MockMarketplaceService
	Auth         *MockAuthService
	Repositories *MockRepositories
}

// NewSimpleMockServices creates all mock services
func NewSimpleMockServices() *SimpleMockServices {
	return &SimpleMockServices{
		CallRouting:  new(MockCallRoutingService),
		Bidding:      new(MockBiddingService),
		Telephony:    new(MockTelephonyService),
		Fraud:        new(MockFraudService),
		Analytics:    new(MockAnalyticsService),
		Marketplace:  new(MockMarketplaceService),
		Auth:         new(MockAuthService),
		Repositories: NewMockRepositories(),
	}
}

// Mock implementations with correct signatures

// MockCallRoutingService implements callrouting.Service
type MockCallRoutingService struct {
	mock.Mock
}

func (m *MockCallRoutingService) RouteCall(ctx context.Context, callID uuid.UUID) (*callrouting.RoutingDecision, error) {
	args := m.Called(ctx, callID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*callrouting.RoutingDecision), args.Error(1)
}

func (m *MockCallRoutingService) GetActiveRoutes(ctx context.Context) ([]*callrouting.ActiveRoute, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*callrouting.ActiveRoute), args.Error(1)
}

func (m *MockCallRoutingService) UpdateRoutingRules(ctx context.Context, rules *callrouting.RoutingRules) error {
	args := m.Called(ctx, rules)
	return args.Error(0)
}

// MockBiddingService implements bidding.Service
type MockBiddingService struct {
	mock.Mock
}

func (m *MockBiddingService) PlaceBid(ctx context.Context, req *bidding.PlaceBidRequest) (*bid.Bid, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*bid.Bid), args.Error(1)
}

func (m *MockBiddingService) UpdateBid(ctx context.Context, bidID uuid.UUID, updates *bidding.BidUpdate) (*bid.Bid, error) {
	args := m.Called(ctx, bidID, updates)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*bid.Bid), args.Error(1)
}

func (m *MockBiddingService) CancelBid(ctx context.Context, bidID uuid.UUID) error {
	args := m.Called(ctx, bidID)
	return args.Error(0)
}

func (m *MockBiddingService) GetBid(ctx context.Context, bidID uuid.UUID) (*bid.Bid, error) {
	args := m.Called(ctx, bidID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*bid.Bid), args.Error(1)
}

func (m *MockBiddingService) GetBidsForCall(ctx context.Context, callID uuid.UUID) ([]*bid.Bid, error) {
	args := m.Called(ctx, callID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*bid.Bid), args.Error(1)
}

func (m *MockBiddingService) GetBidsForBuyer(ctx context.Context, buyerID uuid.UUID) ([]*bid.Bid, error) {
	args := m.Called(ctx, buyerID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*bid.Bid), args.Error(1)
}

func (m *MockBiddingService) ProcessExpiredBids(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockBiddingService) StartAuction(ctx context.Context, callID uuid.UUID, duration interface{}) (*marketplace.AuctionInfo, error) {
	args := m.Called(ctx, callID, duration)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*marketplace.AuctionInfo), args.Error(1)
}

// MockTelephonyService implements telephony.Service
type MockTelephonyService struct {
	mock.Mock
}

func (m *MockTelephonyService) InitiateCall(ctx context.Context, req *telephony.InitiateCallRequest) (*telephony.CallResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*telephony.CallResponse), args.Error(1)
}

func (m *MockTelephonyService) TerminateCall(ctx context.Context, callID uuid.UUID) error {
	args := m.Called(ctx, callID)
	return args.Error(0)
}

func (m *MockTelephonyService) TransferCall(ctx context.Context, callID uuid.UUID, to string) error {
	args := m.Called(ctx, callID, to)
	return args.Error(0)
}

func (m *MockTelephonyService) GetCallStatus(ctx context.Context, callID uuid.UUID) (*telephony.CallStatus, error) {
	args := m.Called(ctx, callID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*telephony.CallStatus), args.Error(1)
}

func (m *MockTelephonyService) RecordCall(ctx context.Context, callID uuid.UUID, record bool) error {
	args := m.Called(ctx, callID, record)
	return args.Error(0)
}

func (m *MockTelephonyService) SendDTMF(ctx context.Context, callID uuid.UUID, digits string) error {
	args := m.Called(ctx, callID, digits)
	return args.Error(0)
}

func (m *MockTelephonyService) BridgeCalls(ctx context.Context, callID1, callID2 uuid.UUID) error {
	args := m.Called(ctx, callID1, callID2)
	return args.Error(0)
}

func (m *MockTelephonyService) HandleWebhook(ctx context.Context, provider string, data interface{}) error {
	args := m.Called(ctx, provider, data)
	return args.Error(0)
}

// MockFraudService implements fraud.Service
type MockFraudService struct {
	mock.Mock
}

func (m *MockFraudService) CheckCall(ctx context.Context, call *call.Call) (*fraud.FraudCheckResult, error) {
	args := m.Called(ctx, call)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*fraud.FraudCheckResult), args.Error(1)
}

func (m *MockFraudService) CheckBid(ctx context.Context, bid *bid.Bid, buyer *account.Account) (*fraud.FraudCheckResult, error) {
	args := m.Called(ctx, bid, buyer)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*fraud.FraudCheckResult), args.Error(1)
}

func (m *MockFraudService) CheckAccount(ctx context.Context, account *account.Account) (*fraud.FraudCheckResult, error) {
	args := m.Called(ctx, account)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*fraud.FraudCheckResult), args.Error(1)
}

func (m *MockFraudService) GetRiskScore(ctx context.Context, entityID uuid.UUID, entityType string) (float64, error) {
	args := m.Called(ctx, entityID, entityType)
	return args.Get(0).(float64), args.Error(1)
}

func (m *MockFraudService) ReportFraud(ctx context.Context, report *fraud.FraudReport) error {
	args := m.Called(ctx, report)
	return args.Error(0)
}

func (m *MockFraudService) UpdateRules(ctx context.Context, rules *fraud.FraudRules) error {
	args := m.Called(ctx, rules)
	return args.Error(0)
}

// MockAnalyticsService provides basic analytics mock
type MockAnalyticsService struct {
	mock.Mock
}

// MockMarketplaceService implements marketplace.MarketplaceOrchestrator
type MockMarketplaceService struct {
	mock.Mock
}

func (m *MockMarketplaceService) ProcessIncomingCall(ctx context.Context, request *marketplace.IncomingCallRequest) (*marketplace.CallProcessingResult, error) {
	args := m.Called(ctx, request)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*marketplace.CallProcessingResult), args.Error(1)
}

func (m *MockMarketplaceService) ProcessSellerCall(ctx context.Context, callID uuid.UUID) (*marketplace.SellerCallResult, error) {
	args := m.Called(ctx, callID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*marketplace.SellerCallResult), args.Error(1)
}

func (m *MockMarketplaceService) ProcessBuyerBid(ctx context.Context, request *marketplace.BidRequest) (*marketplace.BidProcessingResult, error) {
	args := m.Called(ctx, request)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*marketplace.BidProcessingResult), args.Error(1)
}

func (m *MockMarketplaceService) ExecuteCallRouting(ctx context.Context, callID uuid.UUID) (*marketplace.RoutingResult, error) {
	args := m.Called(ctx, callID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*marketplace.RoutingResult), args.Error(1)
}

func (m *MockMarketplaceService) HandleAuctionCompletion(ctx context.Context, auctionID uuid.UUID) (*marketplace.AuctionResult, error) {
	args := m.Called(ctx, auctionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*marketplace.AuctionResult), args.Error(1)
}

func (m *MockMarketplaceService) GetMarketplaceStatus(ctx context.Context) (*marketplace.MarketplaceStatus, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*marketplace.MarketplaceStatus), args.Error(1)
}

// MockAuthService implements authentication service
type MockAuthService struct {
	mock.Mock
}

func (m *MockAuthService) Login(ctx context.Context, email, password string) (*LoginResponse, error) {
	args := m.Called(ctx, email, password)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*LoginResponse), args.Error(1)
}

func (m *MockAuthService) RefreshToken(ctx context.Context, refreshToken string) (*LoginResponse, error) {
	args := m.Called(ctx, refreshToken)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*LoginResponse), args.Error(1)
}

func (m *MockAuthService) GetProfile(ctx context.Context, userID uuid.UUID) (*UserProfile, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*UserProfile), args.Error(1)
}

// MockRepositories provides mock repositories for testing
// This matches the structure of repository.Repositories
type MockRepositories struct {
	Account    *MockAccountRepository
	Bid        *MockBidRepository
	Call       *MockCallRepository
	Compliance *MockComplianceRepository
	Financial  *MockFinancialRepository
}

func NewMockRepositories() *MockRepositories {
	return &MockRepositories{
		Account:    new(MockAccountRepository),
		Bid:        new(MockBidRepository),
		Call:       new(MockCallRepository),
		Compliance: new(MockComplianceRepository),
		Financial:  new(MockFinancialRepository),
	}
}

// MockAccountRepository provides mock for account repository
type MockAccountRepository struct {
	mock.Mock
}

func (m *MockAccountRepository) GetByID(ctx context.Context, id uuid.UUID) (*account.Account, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*account.Account), args.Error(1)
}

func (m *MockAccountRepository) UpdateBalance(ctx context.Context, id uuid.UUID, amount float64) error {
	args := m.Called(ctx, id, amount)
	return args.Error(0)
}

func (m *MockAccountRepository) GetBalance(ctx context.Context, id uuid.UUID) (*AccountBalance, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*AccountBalance), args.Error(1)
}

func (m *MockAccountRepository) Create(ctx context.Context, account *account.Account) error {
	args := m.Called(ctx, account)
	return args.Error(0)
}

// MockBidRepository provides mock for bid repository
type MockBidRepository struct {
	mock.Mock
}

func (m *MockBidRepository) Create(ctx context.Context, bidProfile *bid.Bid) error {
	args := m.Called(ctx, bidProfile)
	return args.Error(0)
}

func (m *MockBidRepository) GetByID(ctx context.Context, id uuid.UUID) (*bid.Bid, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*bid.Bid), args.Error(1)
}

func (m *MockBidRepository) List(ctx context.Context, options interface{}) ([]*bid.Bid, int, error) {
	args := m.Called(ctx, options)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*bid.Bid), args.Int(1), args.Error(2)
}

// MockCallRepository provides mock for call repository
type MockCallRepository struct {
	mock.Mock
}

func (m *MockCallRepository) List(ctx context.Context, options interface{}) ([]*call.Call, int, error) {
	args := m.Called(ctx, options)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*call.Call), args.Int(1), args.Error(2)
}

func (m *MockCallRepository) GetByID(ctx context.Context, id uuid.UUID) (*call.Call, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*call.Call), args.Error(1)
}

func (m *MockCallRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status call.Status) error {
	args := m.Called(ctx, id, status)
	return args.Error(0)
}

func (m *MockCallRepository) Complete(ctx context.Context, id uuid.UUID, duration int) (*call.Call, error) {
	args := m.Called(ctx, id, duration)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*call.Call), args.Error(1)
}

// MockComplianceRepository provides mock for compliance repository
type MockComplianceRepository struct {
	mock.Mock
}

func (m *MockComplianceRepository) AddToDNC(ctx context.Context, entry interface{}) error {
	args := m.Called(ctx, entry)
	return args.Error(0)
}

func (m *MockComplianceRepository) SetTCPAHours(ctx context.Context, startTime, endTime, timezone string) error {
	args := m.Called(ctx, startTime, endTime, timezone)
	return args.Error(0)
}

func (m *MockComplianceRepository) CheckDNC(ctx context.Context, phoneNumber string) (bool, error) {
	args := m.Called(ctx, phoneNumber)
	return args.Bool(0), args.Error(1)
}

// MockFinancialRepository provides mock for financial repository
type MockFinancialRepository struct {
	mock.Mock
}

// AssertExpectations asserts all mock expectations
func (m *SimpleMockServices) AssertExpectations(t mock.TestingT) {
	m.CallRouting.AssertExpectations(t)
	m.Bidding.AssertExpectations(t)
	m.Telephony.AssertExpectations(t)
	m.Fraud.AssertExpectations(t)
	m.Analytics.AssertExpectations(t)
	m.Marketplace.AssertExpectations(t)
}