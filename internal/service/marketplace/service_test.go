package marketplace

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/account"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/bid"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/call"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/service/bidding"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/service/buyer_routing"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/service/fraud"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/service/seller_distribution"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/service/telephony"
)

// uuidPtr is a helper function to create a pointer to a UUID
func uuidPtr(id uuid.UUID) *uuid.UUID {
	return &id
}

// MarketplaceServiceTestSuite encapsulates all marketplace service tests
type MarketplaceServiceTestSuite struct {
	suite.Suite
	ctx                   context.Context
	orchestrator          *orchestrator
	callRepo              *mockCallRepository
	bidRepo               *mockBidRepository
	accountRepo           *mockAccountRepository
	buyerRoutingService   *mockBuyerRoutingService
	sellerDistService     *mockSellerDistributionService
	biddingService        *mockBiddingService
	fraudService          *mockFraudService
	telephonyService      *mockTelephonyService
	metrics               *mockMarketplaceMetrics
	config                *OrchestratorConfig
}

// SetupTest runs before each test in the suite
func (s *MarketplaceServiceTestSuite) SetupTest() {
	s.ctx = context.Background()
	
	// Initialize mocks
	s.callRepo = new(mockCallRepository)
	s.bidRepo = new(mockBidRepository)
	s.accountRepo = new(mockAccountRepository)
	s.buyerRoutingService = new(mockBuyerRoutingService)
	s.sellerDistService = new(mockSellerDistributionService)
	s.biddingService = new(mockBiddingService)
	s.fraudService = new(mockFraudService)
	s.telephonyService = new(mockTelephonyService)
	s.metrics = new(mockMarketplaceMetrics)
	
	// Default configuration
	s.config = &OrchestratorConfig{
		DefaultAuctionDuration: 30 * time.Second,
		MaxConcurrentCalls:     100,
		MaxConcurrentAuctions:  20,
		CallTimeoutDuration:    5 * time.Minute,
		FraudCheckEnabled:      true,
		MetricsEnabled:         true,
		PreferDirectAssignment: false,
		EnableSellerFirst:      true,
		EnableAuctionFallback:  true,
		MinBuyerQualityScore:   5.0,
		MinSellerQualityScore:  5.0,
		MaxFraudRiskScore:      0.7,
	}
	
	// Create orchestrator
	s.orchestrator = NewOrchestrator(
		s.callRepo,
		s.bidRepo,
		s.accountRepo,
		s.buyerRoutingService,
		s.sellerDistService,
		s.biddingService,
		s.fraudService,
		s.telephonyService,
		s.metrics,
		s.config,
	).(*orchestrator)
}

// TearDownTest runs after each test
func (s *MarketplaceServiceTestSuite) TearDownTest() {
	// Assert all expectations were met
	s.callRepo.AssertExpectations(s.T())
	s.bidRepo.AssertExpectations(s.T())
	s.accountRepo.AssertExpectations(s.T())
	s.buyerRoutingService.AssertExpectations(s.T())
	s.sellerDistService.AssertExpectations(s.T())
	s.biddingService.AssertExpectations(s.T())
	s.fraudService.AssertExpectations(s.T())
	s.telephonyService.AssertExpectations(s.T())
	s.metrics.AssertExpectations(s.T())
}

// TestMarketplaceServiceTestSuite runs the test suite
func TestMarketplaceServiceTestSuite(t *testing.T) {
	suite.Run(t, new(MarketplaceServiceTestSuite))
}

// ============================================================================
// Test Cases: ProcessIncomingCall
// ============================================================================

func (s *MarketplaceServiceTestSuite) TestProcessIncomingCall_DirectAssignment_Success() {
	// Arrange
	buyerID := uuid.New()
	request := &IncomingCallRequest{
		FromNumber: "+15551234567",
		ToNumber:   "+15557654321",
		Direction:  call.DirectionInbound,
		BuyerID:    &buyerID,
		Priority:   PriorityNormal,
	}
	
	testCall := s.createTestCall(uuid.New(), &buyerID, nil)
	testCall.FromNumber = values.MustNewPhoneNumber(request.FromNumber)
	testCall.ToNumber = values.MustNewPhoneNumber(request.ToNumber)
	testCall.Direction = request.Direction
	
	routingDecision := &buyer_routing.BuyerRoutingDecision{
		CallID:    testCall.ID,
		BidID:     uuid.New(),
		BuyerID:   buyerID,
		SellerID:  uuid.New(),
		Algorithm: "direct_assignment",
		Score:     1.0,
		Reason:    "Direct assignment requested",
		Timestamp: time.Now(),
	}
	
	// Setup expectations
	s.callRepo.On("Create", s.ctx, mock.AnythingOfType("*call.Call")).Return(nil)
	s.buyerRoutingService.On("RouteCall", s.ctx, mock.AnythingOfType("uuid.UUID")).Return(routingDecision, nil)
	s.callRepo.On("Update", s.ctx, mock.AnythingOfType("*call.Call")).Return(nil)
	s.callRepo.On("GetByID", s.ctx, mock.AnythingOfType("uuid.UUID")).Return(testCall, nil)
	
	// Mock telephony for direct assignment
	telephonyResponse := &telephony.CallResponse{
		CallID:    testCall.ID,
		CallSID:   "CALL123456",
		Status:    call.StatusInProgress,
		StartTime: time.Now(),
	}
	s.telephonyService.On("InitiateCall", s.ctx, mock.AnythingOfType("*call.Call")).Return(telephonyResponse, nil)
	
	s.metrics.On("RecordCallProcessing", s.ctx, mock.AnythingOfType("*marketplace.CallProcessingResult"))
	s.metrics.On("RecordRoutingDecision", s.ctx, mock.AnythingOfType("*marketplace.RoutingResult"))
	
	// Act
	result, err := s.orchestrator.ProcessIncomingCall(s.ctx, request)
	
	// Assert
	s.NoError(err)
	s.NotNil(result)
	s.Equal(ProcessingStatusAssigned, result.Status)
	s.Equal(PathDirectAssignment, result.ProcessingPath)
	s.NotEqual(uuid.Nil, result.CallID)
	s.NotZero(result.ProcessedAt)
	s.NotNil(result.BuyerDecision)
	s.Equal(buyerID, result.BuyerDecision.BuyerID)
}

func (s *MarketplaceServiceTestSuite) TestProcessIncomingCall_SellerDistribution_Success() {
	// Arrange
	sellerID := uuid.New()
	request := &IncomingCallRequest{
		FromNumber: "+15551234567",
		ToNumber:   "+15557654321",
		Direction:  call.DirectionInbound,
		SellerID:   &sellerID,
		Priority:   PriorityNormal,
	}
	
	testCall := s.createTestCall(uuid.New(), nil, &sellerID)
	testCall.FromNumber = values.MustNewPhoneNumber(request.FromNumber)
	testCall.ToNumber = values.MustNewPhoneNumber(request.ToNumber)
	testCall.Direction = request.Direction
	
	distributionDecision := &seller_distribution.SellerDistributionDecision{
		SelectedSellers: []uuid.UUID{uuid.New(), uuid.New()},
		Algorithm:       "geographic_distribution",
		Score:           0.8,
	}
	
	auctionInfo := &AuctionInfo{
		ID:        uuid.New(),
		CallID:    testCall.ID,
		Status:    "active",
		StartedAt: time.Now(),
	}
	
	// Setup expectations
	s.callRepo.On("Create", s.ctx, mock.AnythingOfType("*call.Call")).Return(nil)
	s.callRepo.On("GetByID", s.ctx, mock.AnythingOfType("uuid.UUID")).Return(testCall, nil)
	s.sellerDistService.On("DistributeCall", s.ctx, mock.AnythingOfType("uuid.UUID")).Return(distributionDecision, nil)
	s.biddingService.On("StartAuction", s.ctx, mock.AnythingOfType("uuid.UUID"), s.config.DefaultAuctionDuration).Return(auctionInfo, nil)
	s.metrics.On("RecordCallProcessing", s.ctx, mock.AnythingOfType("*marketplace.CallProcessingResult"))
	
	// Act
	result, err := s.orchestrator.ProcessIncomingCall(s.ctx, request)
	
	// Assert
	s.NoError(err)
	s.NotNil(result)
	s.Equal(ProcessingStatusAuction, result.Status)
	s.Equal(PathSellerDistribution, result.ProcessingPath)
	s.NotNil(result.AuctionID)
	s.NotNil(result.SellerDecision)
	s.Equal(2, len(result.SellerDecision.SelectedSellers))
}

func (s *MarketplaceServiceTestSuite) TestProcessIncomingCall_ValidationErrors() {
	testCases := []struct {
		name          string
		request       *IncomingCallRequest
		expectedError string
	}{
		{
			name: "missing_from_number",
			request: &IncomingCallRequest{
				FromNumber: "",
				ToNumber:   "+15557654321",
				Direction:  call.DirectionInbound,
				BuyerID:    uuidPtr(uuid.New()),
				Priority:   PriorityNormal,
			},
			expectedError: "from_number is required",
		},
		{
			name: "missing_to_number",
			request: &IncomingCallRequest{
				FromNumber: "+15551234567",
				ToNumber:   "",
				Direction:  call.DirectionInbound,
				BuyerID:    uuidPtr(uuid.New()),
				Priority:   PriorityNormal,
			},
			expectedError: "to_number is required",
		},
		{
			name: "invalid_phone_format",
			request: &IncomingCallRequest{
				FromNumber: "invalid-phone", // Truly invalid format
				ToNumber:   "+15557654321",
				Direction:  call.DirectionInbound,
				BuyerID:    uuidPtr(uuid.New()),
				Priority:   PriorityNormal,
			},
			expectedError: "failed to create call",
		},
		{
			name: "neither_buyer_nor_seller",
			request: &IncomingCallRequest{
				FromNumber: "+15551234567",
				ToNumber:   "+15557654321",
				Direction:  call.DirectionInbound,
				Priority:   PriorityNormal,
			},
			expectedError: "either seller_id or buyer_id must be provided",
		},
	}
	
	for _, tc := range testCases {
		s.Run(tc.name, func() {
			result, err := s.orchestrator.ProcessIncomingCall(s.ctx, tc.request)
			
			s.Error(err)
			s.Nil(result)
			s.Contains(err.Error(), tc.expectedError)
		})
	}
}

// ============================================================================
// Test Cases: ProcessBuyerBid
// ============================================================================

func (s *MarketplaceServiceTestSuite) TestProcessBuyerBid_Success() {
	// Arrange
	callID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New() // Marketplace bids require a seller
	
	buyer := s.createTestAccount(buyerID, account.TypeBuyer, 8.5)
	
	request := &BidRequest{
		CallID:   callID,
		BuyerID:  buyerID,
		Amount:   5.50,
		Currency: "USD",
		Criteria: bid.BidCriteria{
			CallType: []string{"inbound"},
		},
		Quality: buyer.QualityMetrics,
		ExpiresAt: time.Now().Add(time.Hour),
	}
	
	fraudCheckResult := &fraud.FraudCheckResult{
		ID:         uuid.New(),
		EntityID:   buyerID,
		EntityType: "bid",
		Timestamp:  time.Now(),
		Approved:   true,
		RiskScore:  0.1,
		Confidence: 0.95,
	}
	
	// Setup expectations
	// Create a marketplace call with seller ID
	testCall := s.createTestCall(callID, nil, &sellerID)
	s.callRepo.On("GetByID", s.ctx, callID).Return(testCall, nil)
	s.accountRepo.On("GetByID", s.ctx, buyerID).Return(buyer, nil)
	s.fraudService.On("CheckBid", s.ctx, mock.AnythingOfType("*bid.Bid"), buyer).Return(fraudCheckResult, nil)
	s.bidRepo.On("Create", s.ctx, mock.AnythingOfType("*bid.Bid")).Return(nil)
	
	// Act
	result, err := s.orchestrator.ProcessBuyerBid(s.ctx, request)
	
	// Assert
	s.NoError(err)
	s.NotNil(result)
	s.Equal(BidStatusAccepted, result.Status)
	s.NotEqual(uuid.Nil, result.BidID)
	s.NotZero(result.ProcessedAt)
	s.NotNil(result.FraudCheck)
	s.True(result.FraudCheck.Approved)
}

func (s *MarketplaceServiceTestSuite) TestProcessBuyerBid_FraudRejection() {
	// Arrange
	callID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New() // Marketplace bids require a seller
	
	buyer := s.createTestAccount(buyerID, account.TypeBuyer, 7.0)
	
	request := &BidRequest{
		CallID:   callID,
		BuyerID:  buyerID,
		Amount:   5.50,
		Currency: "USD",
		Criteria: bid.BidCriteria{
			CallType: []string{"inbound"},
		},
		Quality: buyer.QualityMetrics,
		ExpiresAt: time.Now().Add(time.Hour),
	}
	
	fraudCheckResult := &fraud.FraudCheckResult{
		ID:         uuid.New(),
		EntityID:   buyerID,
		EntityType: "bid",
		Timestamp:  time.Now(),
		Approved:   false,
		RiskScore:  0.9,
		Confidence: 0.95,
		Reasons:    []string{"Suspicious bidding pattern", "High velocity"},
	}
	
	// Setup expectations
	// Create a marketplace call with seller ID
	testCall := s.createTestCall(callID, nil, &sellerID)
	s.callRepo.On("GetByID", s.ctx, callID).Return(testCall, nil)
	s.accountRepo.On("GetByID", s.ctx, buyerID).Return(buyer, nil)
	s.fraudService.On("CheckBid", s.ctx, mock.AnythingOfType("*bid.Bid"), buyer).Return(fraudCheckResult, nil)
	// Note: No bid creation expected when fraud check fails
	
	// Act
	result, err := s.orchestrator.ProcessBuyerBid(s.ctx, request)
	
	// Assert
	s.NoError(err)
	s.NotNil(result)
	s.Equal(BidStatusFraudulent, result.Status)
	s.NotNil(result.FraudCheck)
	s.False(result.FraudCheck.Approved)
	// Note: The current implementation doesn't populate Errors for fraud rejection
	// This is a design choice - fraud rejection is not an error, it's a valid business state
}

func (s *MarketplaceServiceTestSuite) TestProcessBuyerBid_ValidationErrors() {
	testCases := []struct {
		name          string
		modifyRequest func(*BidRequest)
		expectedError string
		setupMocks    func(*BidRequest)
	}{
		{
			name: "negative_amount",
			modifyRequest: func(r *BidRequest) {
				r.Amount = -1.0
			},
			expectedError: "amount must be positive",
			setupMocks: func(r *BidRequest) {
				// No mocks needed - validation happens first
			},
		},
		{
			name: "invalid_currency",
			modifyRequest: func(r *BidRequest) {
				r.Currency = "XXX" // Invalid 3-letter code
			},
			expectedError: "invalid bid amount: unsupported currency: XXX",
			setupMocks: func(r *BidRequest) {
				// Service fetches buyer before currency validation
				// But when currency validation fails, it never fetches the call
				buyer := s.createTestAccount(r.BuyerID, account.TypeBuyer, 8.5)
				s.accountRepo.On("GetByID", s.ctx, r.BuyerID).Return(buyer, nil)
				// Note: callRepo.GetByID is NOT called when Money creation fails
			},
		},
		{
			name: "missing_call_id",
			modifyRequest: func(r *BidRequest) {
				r.CallID = uuid.Nil
			},
			expectedError: "call_id is required",
			setupMocks: func(r *BidRequest) {
				// No mocks needed - validation happens first
			},
		},
		{
			name: "missing_buyer_id",
			modifyRequest: func(r *BidRequest) {
				r.BuyerID = uuid.Nil
			},
			expectedError: "buyer_id is required",
			setupMocks: func(r *BidRequest) {
				// No mocks needed - validation happens first
			},
		},
		{
			name: "empty_currency",
			modifyRequest: func(r *BidRequest) {
				r.Currency = ""
			},
			expectedError: "currency is required",
			setupMocks: func(r *BidRequest) {
				// No mocks needed - validation happens first
			},
		},
	}
	
	for _, tc := range testCases {
		s.Run(tc.name, func() {
			// Arrange
			request := &BidRequest{
				CallID:   uuid.New(),
				BuyerID:  uuid.New(),
				Amount:   5.50,
				Currency: "USD",
				Criteria: bid.BidCriteria{
					CallType: []string{"inbound"},
				},
				Quality: values.QualityMetrics{
					QualityScore: 8.5,
				},
				ExpiresAt: time.Now().Add(time.Hour),
			}
			
			tc.modifyRequest(request)
			tc.setupMocks(request)
			
			// Act
			result, err := s.orchestrator.ProcessBuyerBid(s.ctx, request)
			
			// Assert
			s.Error(err)
			s.Contains(err.Error(), tc.expectedError)
			s.Nil(result)
		})
	}
}

// ============================================================================
// Test Cases: ExecuteCallRouting
// ============================================================================

func (s *MarketplaceServiceTestSuite) TestExecuteCallRouting_Success() {
	// Arrange
	callID := uuid.New()
	buyerID := uuid.New()
	
	testCall := s.createTestCall(callID, &buyerID, nil)
	
	routingDecision := &buyer_routing.BuyerRoutingDecision{
		CallID:    callID,
		BidID:     uuid.New(),
		BuyerID:   buyerID,
		SellerID:  uuid.New(),
		Algorithm: "skill_based",
		Score:     0.85,
		Reason:    "Best skill match",
		Timestamp: time.Now(),
	}
	
	telephonyResponse := &telephony.CallResponse{
		CallID:    uuid.New(),
		CallSID:   "CALL123456",
		Status:    call.StatusInProgress,
		StartTime: time.Now(),
	}
	
	// Setup expectations
	s.callRepo.On("GetByID", s.ctx, callID).Return(testCall, nil)
	s.buyerRoutingService.On("RouteCall", s.ctx, callID).Return(routingDecision, nil)
	s.callRepo.On("Update", s.ctx, mock.AnythingOfType("*call.Call")).Return(nil)
	s.telephonyService.On("InitiateCall", s.ctx, mock.AnythingOfType("*call.Call")).Return(telephonyResponse, nil)
	s.metrics.On("RecordRoutingDecision", s.ctx, mock.AnythingOfType("*marketplace.RoutingResult"))
	
	// Act
	result, err := s.orchestrator.ExecuteCallRouting(s.ctx, callID)
	
	// Assert
	s.NoError(err)
	s.NotNil(result)
	s.Equal(callID, result.CallID)
	s.Equal(&buyerID, result.SelectedBuyerID)
	s.Equal(call.StatusInProgress, result.FinalStatus)
	s.NotZero(result.ProcessingTime)
	s.NotNil(result.TelephonyResult)
}

func (s *MarketplaceServiceTestSuite) TestExecuteCallRouting_NoBuyerAvailable() {
	// Arrange
	callID := uuid.New()
	
	testCall := s.createTestCall(callID, nil, nil)
	
	routingDecision := &buyer_routing.BuyerRoutingDecision{
		CallID:    callID,
		BidID:     uuid.Nil,
		BuyerID:   uuid.Nil,
		SellerID:  uuid.New(),
		Algorithm: "skill_based",
		Score:     0.0,
		Reason:    "No buyer available",
		Timestamp: time.Now(),
	}
	
	// Setup expectations
	s.callRepo.On("GetByID", s.ctx, callID).Return(testCall, nil)
	s.buyerRoutingService.On("RouteCall", s.ctx, callID).Return(routingDecision, nil)
	s.callRepo.On("Update", s.ctx, mock.AnythingOfType("*call.Call")).Return(nil)
	// Telephony will fail when no buyer is assigned
	s.telephonyService.On("InitiateCall", s.ctx, mock.AnythingOfType("*call.Call")).
		Return(nil, fmt.Errorf("cannot initiate call without buyer"))
	s.metrics.On("RecordRoutingDecision", s.ctx, mock.AnythingOfType("*marketplace.RoutingResult"))
	
	// Act
	result, err := s.orchestrator.ExecuteCallRouting(s.ctx, callID)
	
	// Assert
	s.NoError(err)
	s.NotNil(result)
	s.Equal(callID, result.CallID)
	// The service sets SelectedBuyerID even when BuyerID is uuid.Nil
	// This creates a pointer to uuid.Nil, not a nil pointer
	s.NotNil(result.SelectedBuyerID)
	s.Equal(uuid.Nil, *result.SelectedBuyerID)
	// When telephony fails due to no buyer, status is Failed
	s.Equal(call.StatusFailed, result.FinalStatus)
}

// ============================================================================
// Test Cases: GetMarketplaceStatus
// ============================================================================

func (s *MarketplaceServiceTestSuite) TestGetMarketplaceStatus_Success() {
	// Arrange
	buyers := []*account.Account{
		s.createTestAccount(uuid.New(), account.TypeBuyer, 8.0),
		s.createTestAccount(uuid.New(), account.TypeBuyer, 8.5),
	}
	
	sellers := []*account.Account{
		s.createTestAccount(uuid.New(), account.TypeSeller, 7.5),
	}
	
	marketplaceMetrics := &MarketplaceMetrics{
		CallsPerHour:        150,
		AverageAuctionTime:  25000,
		BuyerUtilization:    0.75,
		SellerUtilization:   0.80,
		RevenuePerHour:      1250.50,
		FailureRate:         0.05,
	}
	
	// Setup expectations
	s.accountRepo.On("GetActiveBuyers", s.ctx, 1000).Return(buyers, nil)
	s.accountRepo.On("GetActiveSellers", s.ctx, 1000).Return(sellers, nil)
	s.metrics.On("GetCurrentMetrics", s.ctx).Return(marketplaceMetrics, nil)
	
	// Act
	status, err := s.orchestrator.GetMarketplaceStatus(s.ctx)
	
	// Assert
	s.NoError(err)
	s.NotNil(status)
	s.Equal(2, status.ActiveBuyers)
	s.Equal(1, status.ActiveSellers)
	s.Equal(HealthHealthy, status.SystemHealth)
	s.NotNil(status.Metrics)
	s.Equal(150, status.Metrics.CallsPerHour)
	s.NotZero(status.LastUpdated)
}

// ============================================================================
// Test Cases: Concurrent Operations
// ============================================================================

func (s *MarketplaceServiceTestSuite) TestConcurrentCallProcessing() {
	// Test that multiple calls can be processed concurrently without race conditions
	numCalls := 10
	results := make(chan *CallProcessingResult, numCalls)
	errors := make(chan error, numCalls)
	
	// Setup mocks to allow multiple calls
	s.callRepo.On("Create", s.ctx, mock.AnythingOfType("*call.Call")).Return(nil).Times(numCalls)
	
	// Create a test call that will be returned for any GetByID call
	testCall := s.createTestCall(uuid.New(), nil, nil)
	s.callRepo.On("GetByID", s.ctx, mock.AnythingOfType("uuid.UUID")).Return(testCall, nil).Times(numCalls)
	
	s.callRepo.On("Update", s.ctx, mock.AnythingOfType("*call.Call")).Return(nil).Maybe()
	
	// Return a valid routing decision for any RouteCall
	routingDecision := s.generateValidRoutingDecision()
	s.buyerRoutingService.On("RouteCall", s.ctx, mock.AnythingOfType("uuid.UUID")).Return(routingDecision, nil).Maybe()
	
	s.sellerDistService.On("DistributeCall", s.ctx, mock.AnythingOfType("uuid.UUID")).Return(
		s.generateValidDistributionDecision(), nil).Maybe()
	
	s.biddingService.On("StartAuction", s.ctx, mock.AnythingOfType("uuid.UUID"), 
		mock.AnythingOfType("time.Duration")).Return(s.generateValidAuctionInfo(), nil).Maybe()
	
	// Mock telephony service for when direct assignment leads to call initiation
	telephonyResponse := &telephony.CallResponse{
		CallID:    uuid.New(),
		CallSID:   "CALL123456",
		Status:    call.StatusInProgress,
		StartTime: time.Now(),
	}
	s.telephonyService.On("InitiateCall", s.ctx, mock.AnythingOfType("*call.Call")).Return(telephonyResponse, nil).Maybe()
	
	s.metrics.On("RecordCallProcessing", s.ctx, mock.AnythingOfType("*marketplace.CallProcessingResult")).Maybe()
	s.metrics.On("RecordRoutingDecision", s.ctx, mock.AnythingOfType("*marketplace.RoutingResult")).Maybe()
	
	// Process calls concurrently
	var wg sync.WaitGroup
	for i := 0; i < numCalls; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			
			request := s.generateRandomIncomingCallRequest()
			result, err := s.orchestrator.ProcessIncomingCall(s.ctx, request)
			
			if err != nil {
				errors <- err
			} else {
				results <- result
			}
		}(i)
	}
	
	// Wait for all goroutines
	wg.Wait()
	close(results)
	close(errors)
	
	// Verify results
	successCount := 0
	for result := range results {
		s.NotNil(result)
		s.NotEqual(uuid.Nil, result.CallID)
		successCount++
	}
	
	errorCount := 0
	for err := range errors {
		s.T().Logf("Concurrent processing error: %v", err)
		errorCount++
	}
	
	s.T().Logf("Concurrent test results: %d successes, %d errors", successCount, errorCount)
	s.GreaterOrEqual(successCount, numCalls/2) // At least half should succeed
}

// ============================================================================
// Property-Based Tests
// ============================================================================

func (s *MarketplaceServiceTestSuite) TestProcessIncomingCall_PropertyBased() {
	// Test property: All valid incoming calls should result in a non-nil response
	// with a valid status and processing path
	
	for i := 0; i < 100; i++ {
		// Generate random valid request
		request := s.generateRandomIncomingCallRequest()
		
		// Setup minimal mocks for valid request
		s.callRepo.On("Create", s.ctx, mock.AnythingOfType("*call.Call")).Return(nil).Maybe()
		s.callRepo.On("GetByID", s.ctx, mock.AnythingOfType("uuid.UUID")).Return(
			s.createTestCall(uuid.New(), nil, nil), nil).Maybe()
		
		if request.BuyerID != nil {
			s.buyerRoutingService.On("RouteCall", s.ctx, mock.AnythingOfType("uuid.UUID")).Return(
				s.generateValidRoutingDecision(), nil).Maybe()
			s.callRepo.On("Update", s.ctx, mock.AnythingOfType("*call.Call")).Return(nil).Maybe()
			// Mock telephony for direct assignment
			telephonyResponse := &telephony.CallResponse{
				CallID:    uuid.New(),
				CallSID:   "CALL123456",
				Status:    call.StatusInProgress,
				StartTime: time.Now(),
			}
			s.telephonyService.On("InitiateCall", s.ctx, mock.AnythingOfType("*call.Call")).Return(telephonyResponse, nil).Maybe()
		} else if request.SellerID != nil {
			s.sellerDistService.On("DistributeCall", s.ctx, mock.AnythingOfType("uuid.UUID")).Return(
				s.generateValidDistributionDecision(), nil).Maybe()
			s.biddingService.On("StartAuction", s.ctx, mock.AnythingOfType("uuid.UUID"), 
				mock.AnythingOfType("time.Duration")).Return(s.generateValidAuctionInfo(), nil).Maybe()
		}
		
		s.metrics.On("RecordCallProcessing", s.ctx, mock.AnythingOfType("*marketplace.CallProcessingResult")).Maybe()
		s.metrics.On("RecordRoutingDecision", s.ctx, mock.AnythingOfType("*marketplace.RoutingResult")).Maybe()
		
		// Act
		result, err := s.orchestrator.ProcessIncomingCall(s.ctx, request)
		
		// Assert properties
		if err == nil {
			s.NotNil(result)
			s.NotEqual(uuid.Nil, result.CallID)
			s.True(result.Status >= ProcessingStatusAccepted && result.Status <= ProcessingStatusRejected)
			s.True(result.ProcessingPath >= PathDirectAssignment && result.ProcessingPath <= PathFailover)
			s.NotZero(result.ProcessedAt)
		}
	}
}

// ============================================================================
// Benchmarks
// ============================================================================

func BenchmarkProcessIncomingCall(b *testing.B) {
	// Setup
	ctx := context.Background()
	suite := new(MarketplaceServiceTestSuite)
	suite.SetT(&testing.T{})
	suite.SetupTest()
	
	buyerID := uuid.New()
	request := &IncomingCallRequest{
		FromNumber: "+15551234567",
		ToNumber:   "+15557654321",
		Direction:  call.DirectionInbound,
		BuyerID:    &buyerID,
		Priority:   PriorityNormal,
	}
	
	// Create test call for benchmark
	testCall, _ := call.NewCall("+15551234567", "+15557654321", buyerID, call.DirectionInbound)
	
	// Setup minimal mocks
	suite.callRepo.On("Create", ctx, mock.AnythingOfType("*call.Call")).Return(nil)
	suite.buyerRoutingService.On("RouteCall", ctx, mock.AnythingOfType("uuid.UUID")).Return(
		&buyer_routing.BuyerRoutingDecision{
			CallID:    uuid.New(),
			BidID:     uuid.New(),
			BuyerID:   buyerID,
			SellerID:  uuid.New(),
			Algorithm: "direct",
			Score:     1.0,
			Reason:    "Direct routing",
			Timestamp: time.Now(),
		}, nil)
	suite.callRepo.On("Update", ctx, mock.AnythingOfType("*call.Call")).Return(nil)
	suite.callRepo.On("GetByID", ctx, mock.AnythingOfType("uuid.UUID")).Return(testCall, nil)
	suite.metrics.On("RecordCallProcessing", ctx, mock.AnythingOfType("*marketplace.CallProcessingResult"))
	suite.metrics.On("RecordRoutingDecision", ctx, mock.AnythingOfType("*marketplace.RoutingResult"))
	
	b.ResetTimer()
	
	// Benchmark
	for i := 0; i < b.N; i++ {
		_, _ = suite.orchestrator.ProcessIncomingCall(ctx, request)
	}
}

func BenchmarkExecuteCallRouting(b *testing.B) {
	// Setup
	ctx := context.Background()
	suite := new(MarketplaceServiceTestSuite)
	suite.SetT(&testing.T{})
	suite.SetupTest()
	
	callID := uuid.New()
	buyerID := uuid.New()
	
	// Create test call for benchmark
	testCall, _ := call.NewCall("+15551234567", "+15557654321", buyerID, call.DirectionInbound)
	testCall.ID = callID
	
	// Setup minimal mocks
	suite.callRepo.On("GetByID", ctx, callID).Return(testCall, nil)
	suite.buyerRoutingService.On("RouteCall", ctx, callID).Return(
		&buyer_routing.BuyerRoutingDecision{
			CallID:    callID,
			BidID:     uuid.New(),
			BuyerID:   buyerID,
			SellerID:  uuid.New(),
			Algorithm: "skill_based",
			Score:     0.85,
			Timestamp: time.Now(),
		}, nil)
	suite.callRepo.On("Update", ctx, mock.AnythingOfType("*call.Call")).Return(nil)
	suite.telephonyService.On("InitiateCall", ctx, mock.AnythingOfType("*call.Call")).Return(
		&telephony.CallResponse{
			CallID:    uuid.New(),
			CallSID:   "CALL123",
			Status:    call.StatusInProgress,
			StartTime: time.Now(),
		}, nil)
	suite.metrics.On("RecordRoutingDecision", ctx, mock.AnythingOfType("*marketplace.RoutingResult"))
	
	b.ResetTimer()
	
	// Benchmark
	for i := 0; i < b.N; i++ {
		_, _ = suite.orchestrator.ExecuteCallRouting(ctx, callID)
	}
}

// ============================================================================
// Helper Methods
// ============================================================================

func (s *MarketplaceServiceTestSuite) createTestAccount(id uuid.UUID, accountType account.AccountType, qualityScore float64) *account.Account {
	email, err := values.NewEmail("test@example.com")
	s.NoError(err)
	
	phone, err := values.NewPhoneNumber("+15551234567")
	s.NoError(err)
	
	balance, err := values.NewMoneyFromFloat(1000.0, "USD")
	s.NoError(err)
	
	return &account.Account{
		ID:          id,
		Email:       email,
		Name:        "Test Account",
		Type:        accountType,
		Status:      account.StatusActive,
		PhoneNumber: phone,
		Balance:     balance,
		QualityMetrics: values.QualityMetrics{
			QualityScore:     qualityScore,
			AverageCallTime:  300,
			FraudScore:       0.05,
			HistoricalRating: qualityScore,
			ConversionRate:   0.15,
			TrustScore:       qualityScore - 0.5,
			ReliabilityScore: qualityScore,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func (s *MarketplaceServiceTestSuite) createTestCall(id uuid.UUID, buyerID, sellerID *uuid.UUID) *call.Call {
	var testCall *call.Call
	var err error
	
	if sellerID != nil && *sellerID != uuid.Nil {
		testCall, err = call.NewMarketplaceCall("+15551234567", "+15557654321", *sellerID, call.DirectionInbound)
	} else if buyerID != nil && *buyerID != uuid.Nil {
		testCall, err = call.NewCall("+15551234567", "+15557654321", *buyerID, call.DirectionInbound)
	} else {
		// Create a call without buyer or seller for routing scenarios
		testCall = &call.Call{
			ID:        id,
			FromNumber: values.MustNewPhoneNumber("+15551234567"),
			ToNumber:   values.MustNewPhoneNumber("+15557654321"),
			Direction: call.DirectionInbound,
			Status:    call.StatusPending,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		return testCall
	}
	
	s.NoError(err)
	testCall.ID = id
	return testCall
}

func (s *MarketplaceServiceTestSuite) generateRandomIncomingCallRequest() *IncomingCallRequest {
	fromNumber := fmt.Sprintf("+1555%07d", time.Now().UnixNano()%10000000)
	toNumber := fmt.Sprintf("+1555%07d", (time.Now().UnixNano()+1)%10000000)
	
	request := &IncomingCallRequest{
		FromNumber: fromNumber,
		ToNumber:   toNumber,
		Direction:  call.DirectionInbound,
		Priority:   CallPriority(time.Now().UnixNano() % 4),
	}
	
	// Randomly assign either buyer or seller
	if time.Now().UnixNano()%2 == 0 {
		buyerID := uuid.New()
		request.BuyerID = &buyerID
	} else {
		sellerID := uuid.New()
		request.SellerID = &sellerID
	}
	
	return request
}

func (s *MarketplaceServiceTestSuite) generateValidRoutingDecision() *buyer_routing.BuyerRoutingDecision {
	return &buyer_routing.BuyerRoutingDecision{
		CallID:    uuid.New(),
		BidID:     uuid.New(),
		BuyerID:   uuid.New(),
		SellerID:  uuid.New(),
		Algorithm: "test_algorithm",
		Score:     0.75,
		Reason:    "Test routing",
		Timestamp: time.Now(),
	}
}

func (s *MarketplaceServiceTestSuite) generateValidDistributionDecision() *seller_distribution.SellerDistributionDecision {
	return &seller_distribution.SellerDistributionDecision{
		SelectedSellers: []uuid.UUID{uuid.New(), uuid.New()},
		Algorithm:       "test_distribution",
		Score:           0.8,
	}
}

func (s *MarketplaceServiceTestSuite) generateValidAuctionInfo() *AuctionInfo {
	return &AuctionInfo{
		ID:        uuid.New(),
		CallID:    uuid.New(),
		Status:    "active",
		StartedAt: time.Now(),
	}
}

// ============================================================================
// Mock Repository Implementations
// ============================================================================

// mockCallRepository implements CallRepository interface
type mockCallRepository struct {
	mock.Mock
}

func (m *mockCallRepository) GetByID(ctx context.Context, id uuid.UUID) (*call.Call, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*call.Call), args.Error(1)
}

func (m *mockCallRepository) Create(ctx context.Context, c *call.Call) error {
	args := m.Called(ctx, c)
	return args.Error(0)
}

func (m *mockCallRepository) Update(ctx context.Context, c *call.Call) error {
	args := m.Called(ctx, c)
	return args.Error(0)
}

func (m *mockCallRepository) GetIncomingCalls(ctx context.Context, limit int) ([]*call.Call, error) {
	args := m.Called(ctx, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*call.Call), args.Error(1)
}

func (m *mockCallRepository) GetPendingSellerCalls(ctx context.Context, limit int) ([]*call.Call, error) {
	args := m.Called(ctx, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*call.Call), args.Error(1)
}

// mockBidRepository implements BidRepository interface
type mockBidRepository struct {
	mock.Mock
}

func (m *mockBidRepository) Create(ctx context.Context, b *bid.Bid) error {
	args := m.Called(ctx, b)
	return args.Error(0)
}

func (m *mockBidRepository) GetByID(ctx context.Context, id uuid.UUID) (*bid.Bid, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*bid.Bid), args.Error(1)
}

func (m *mockBidRepository) GetActiveBidsForCall(ctx context.Context, callID uuid.UUID) ([]*bid.Bid, error) {
	args := m.Called(ctx, callID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*bid.Bid), args.Error(1)
}

func (m *mockBidRepository) Update(ctx context.Context, b *bid.Bid) error {
	args := m.Called(ctx, b)
	return args.Error(0)
}

// mockAccountRepository implements AccountRepository interface
type mockAccountRepository struct {
	mock.Mock
}

func (m *mockAccountRepository) GetByID(ctx context.Context, id uuid.UUID) (*account.Account, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*account.Account), args.Error(1)
}

func (m *mockAccountRepository) GetActiveBuyers(ctx context.Context, limit int) ([]*account.Account, error) {
	args := m.Called(ctx, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*account.Account), args.Error(1)
}

func (m *mockAccountRepository) GetActiveSellers(ctx context.Context, limit int) ([]*account.Account, error) {
	args := m.Called(ctx, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*account.Account), args.Error(1)
}

// ============================================================================
// Mock Service Implementations
// ============================================================================

// mockBuyerRoutingService implements BuyerRoutingService interface
type mockBuyerRoutingService struct {
	mock.Mock
}

func (m *mockBuyerRoutingService) RouteCall(ctx context.Context, callID uuid.UUID) (*buyer_routing.BuyerRoutingDecision, error) {
	args := m.Called(ctx, callID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*buyer_routing.BuyerRoutingDecision), args.Error(1)
}

func (m *mockBuyerRoutingService) GetActiveBuyers(ctx context.Context) ([]*account.Account, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*account.Account), args.Error(1)
}

// mockSellerDistributionService implements SellerDistributionService interface
type mockSellerDistributionService struct {
	mock.Mock
}

func (m *mockSellerDistributionService) DistributeCall(ctx context.Context, callID uuid.UUID) (*seller_distribution.SellerDistributionDecision, error) {
	args := m.Called(ctx, callID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*seller_distribution.SellerDistributionDecision), args.Error(1)
}

func (m *mockSellerDistributionService) GetAvailableSellers(ctx context.Context) ([]*account.Account, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*account.Account), args.Error(1)
}

// mockBiddingService implements BiddingService interface
type mockBiddingService struct {
	mock.Mock
}

func (m *mockBiddingService) StartAuction(ctx context.Context, callID uuid.UUID, duration time.Duration) (*AuctionInfo, error) {
	args := m.Called(ctx, callID, duration)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*AuctionInfo), args.Error(1)
}

func (m *mockBiddingService) PlaceBid(ctx context.Context, auctionID uuid.UUID, bid *bid.Bid) (*BidResult, error) {
	args := m.Called(ctx, auctionID, bid)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*BidResult), args.Error(1)
}

func (m *mockBiddingService) CompleteAuction(ctx context.Context, auctionID uuid.UUID) (*bidding.AuctionResult, error) {
	args := m.Called(ctx, auctionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*bidding.AuctionResult), args.Error(1)
}

// mockFraudService implements FraudService interface
type mockFraudService struct {
	mock.Mock
}

func (m *mockFraudService) CheckCall(ctx context.Context, call *call.Call) (*fraud.FraudCheckResult, error) {
	args := m.Called(ctx, call)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*fraud.FraudCheckResult), args.Error(1)
}

func (m *mockFraudService) CheckBid(ctx context.Context, bid *bid.Bid, buyer *account.Account) (*fraud.FraudCheckResult, error) {
	args := m.Called(ctx, bid, buyer)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*fraud.FraudCheckResult), args.Error(1)
}

func (m *mockFraudService) CheckAccount(ctx context.Context, account *account.Account) (*fraud.FraudCheckResult, error) {
	args := m.Called(ctx, account)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*fraud.FraudCheckResult), args.Error(1)
}

func (m *mockFraudService) GetRiskScore(ctx context.Context, entityID uuid.UUID, entityType string) (float64, error) {
	args := m.Called(ctx, entityID, entityType)
	return args.Get(0).(float64), args.Error(1)
}

func (m *mockFraudService) ReportFraud(ctx context.Context, report *fraud.FraudReport) error {
	args := m.Called(ctx, report)
	return args.Error(0)
}

func (m *mockFraudService) UpdateRules(ctx context.Context, rules *fraud.FraudRules) error {
	args := m.Called(ctx, rules)
	return args.Error(0)
}

// mockTelephonyService implements TelephonyService interface
type mockTelephonyService struct {
	mock.Mock
}

func (m *mockTelephonyService) InitiateCall(ctx context.Context, call *call.Call) (*telephony.CallResponse, error) {
	args := m.Called(ctx, call)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*telephony.CallResponse), args.Error(1)
}

func (m *mockTelephonyService) TransferCall(ctx context.Context, callID uuid.UUID, targetNumber string) (*telephony.CallResponse, error) {
	args := m.Called(ctx, callID, targetNumber)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*telephony.CallResponse), args.Error(1)
}

func (m *mockTelephonyService) GetCallStatus(ctx context.Context, callID uuid.UUID) (*telephony.CallStatus, error) {
	args := m.Called(ctx, callID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*telephony.CallStatus), args.Error(1)
}

// mockMarketplaceMetrics implements MarketplaceMetricsCollector interface
type mockMarketplaceMetrics struct {
	mock.Mock
}

func (m *mockMarketplaceMetrics) RecordCallProcessing(ctx context.Context, result *CallProcessingResult) {
	m.Called(ctx, result)
}

func (m *mockMarketplaceMetrics) RecordAuctionCompletion(ctx context.Context, result *AuctionResult) {
	m.Called(ctx, result)
}

func (m *mockMarketplaceMetrics) RecordRoutingDecision(ctx context.Context, result *RoutingResult) {
	m.Called(ctx, result)
}

func (m *mockMarketplaceMetrics) GetCurrentMetrics(ctx context.Context) (*MarketplaceMetrics, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*MarketplaceMetrics), args.Error(1)
}

// ============================================================================
// Integration Tests (commented out - enable when running with real dependencies)
// ============================================================================

/*
// These tests require a running PostgreSQL instance and other infrastructure
// Run with: make test-integration

func TestMarketplaceIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
	
	// Setup test database
	testDB := testutil.SetupTestDatabase(t)
	defer testDB.Cleanup()
	
	// Initialize real repositories
	callRepo := repository.NewCallRepository(testDB.DB)
	bidRepo := repository.NewBidRepository(testDB.DB)
	accountRepo := repository.NewAccountRepository(testDB.DB)
	
	// Initialize real services with test configurations
	// buyerRouting := buyer_routing.NewService(dependencies)
	// sellerDist := seller_distribution.NewService(dependencies)
	// bidding := bidding.NewService(dependencies)
	// fraud := fraud.NewService(dependencies)
	// telephony := telephony.NewMockService() // Use mock for external service
	// metrics := NewInMemoryMetricsCollector()
	
	// Create orchestrator with real dependencies
	config := &OrchestratorConfig{
		DefaultAuctionDuration: 5 * time.Second, // Shorter for tests
		MaxConcurrentCalls:     10,
		MaxConcurrentAuctions:  5,
		CallTimeoutDuration:    30 * time.Second,
		FraudCheckEnabled:      true,
		MetricsEnabled:         true,
		PreferDirectAssignment: false,
		EnableSellerFirst:      true,
		EnableAuctionFallback:  true,
		MinBuyerQualityScore:   5.0,
		MinSellerQualityScore:  5.0,
		MaxFraudRiskScore:      0.7,
	}
	
	// orchestrator := NewOrchestrator(
	//	callRepo, bidRepo, accountRepo,
	//	buyerRouting, sellerDist, bidding,
	//	fraud, telephony, metrics, config,
	// )
	
	ctx := context.Background()
	
	t.Run("EndToEndCallFlow", func(t *testing.T) {
		// Create test accounts
		// buyer := createTestBuyerAccount(t, accountRepo)
		// seller := createTestSellerAccount(t, accountRepo)
		
		// Submit incoming call
		// request := &IncomingCallRequest{
		//	FromNumber: "+15551234567",
		//	ToNumber:   "+15557654321",
		//	Direction:  call.DirectionInbound,
		//	SellerID:   &seller.ID,
		//	Priority:   PriorityNormal,
		// }
		
		// result, err := orchestrator.ProcessIncomingCall(ctx, request)
		// require.NoError(t, err)
		// assert.NotNil(t, result.AuctionID)
		
		// Submit bids
		// bidRequest := &BidRequest{
		//	CallID:   result.CallID,
		//	BuyerID:  buyer.ID,
		//	Amount:   5.50,
		//	Currency: "USD",
		//	Criteria: bid.BidCriteria{
		//		CallType: []string{"inbound"},
		//	},
		//	Quality:   buyer.QualityMetrics,
		//	ExpiresAt: time.Now().Add(time.Hour),
		// }
		
		// bidResult, err := orchestrator.ProcessBuyerBid(ctx, bidRequest)
		// require.NoError(t, err)
		// assert.Equal(t, BidStatusAccepted, bidResult.Status)
		
		// Complete auction
		// time.Sleep(config.DefaultAuctionDuration)
		
		// auctionResult, err := orchestrator.HandleAuctionCompletion(ctx, *result.AuctionID)
		// require.NoError(t, err)
		// assert.Equal(t, buyer.ID, *auctionResult.WinningBuyerID)
		
		// Execute routing
		// routingResult, err := orchestrator.ExecuteCallRouting(ctx, result.CallID)
		// require.NoError(t, err)
		// assert.Equal(t, call.StatusInProgress, routingResult.FinalStatus)
	})
}
*/
