# Comprehensive API Testing Implementation Guide
## Dependable Call Exchange Backend

## Table of Contents
1. [Overview](#overview)
2. [Testing Strategy](#testing-strategy)
3. [Unit Tests Implementation](#unit-tests-implementation)
4. [E2E Tests Enhancement](#e2e-tests-enhancement)
5. [API Contract Testing](#api-contract-testing)
6. [Security Testing](#security-testing)
7. [Performance Testing](#performance-testing)
8. [Test Data Management](#test-data-management)
9. [CI/CD Integration](#cicd-integration)
10. [Implementation Roadmap](#implementation-roadmap)

## Overview

This guide provides a comprehensive approach to implementing API tests for the Dependable Call Exchange Backend, covering unit tests, integration tests, E2E tests, and specialized testing scenarios.

### Current State
- ✅ Test infrastructure with Testcontainers
- ✅ E2E test framework with API client
- ⚠️ Handler unit tests (mostly NOT_IMPLEMENTED)
- ⚠️ Limited security and contract testing
- ❌ Missing tests for new endpoints

### Target State
- Complete unit test coverage for all handlers
- Enhanced E2E tests for all business flows
- API contract validation
- Security test suite
- Performance benchmarks
- Automated test execution in CI/CD

## Testing Strategy

### Testing Pyramid
```
         E2E Tests
        /         \
    Integration    \
      Tests         \
    /                \
   Unit Tests        Contract Tests
```

### Test Types and Purposes

| Test Type | Purpose | Execution Time | Dependencies |
|-----------|---------|----------------|--------------|
| Unit Tests | Test handlers in isolation | < 1s | Mocks |
| Integration Tests | Test service interactions | < 10s | Test DB |
| E2E Tests | Test complete workflows | < 30s | Full stack |
| Contract Tests | Validate API spec | < 5s | OpenAPI spec |
| Security Tests | Test auth/authz | < 10s | Test env |
| Performance Tests | Validate SLAs | < 5m | Load tools |

## Unit Tests Implementation

### 1. Handler Test Structure

Create comprehensive unit tests for each handler group:

#### `internal/api/rest/handlers_test.go` (Enhanced)

```go
package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/account"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/bid"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/call"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/compliance"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Test Helpers
func setupHandler(t *testing.T) (*Handler, *MockServices) {
	mocks := NewMockServices()
	handler := NewHandler(Services{
		CallRouting: mocks.CallRouting,
		Bidding:     mocks.Bidding,
		Telephony:   mocks.Telephony,
		Fraud:       mocks.Fraud,
		Account:     mocks.Account,
		Compliance:  mocks.Compliance,
		Auction:     mocks.Auction,
	})
	return handler, mocks
}

func makeRequest(handler http.Handler, method, path string, body interface{}) *httptest.ResponseRecorder {
	var req *http.Request
	if body != nil {
		jsonBody, _ := json.Marshal(body)
		req = httptest.NewRequest(method, path, bytes.NewReader(jsonBody))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	
	// Add auth token for protected endpoints
	if !isPublicEndpoint(path) {
		req.Header.Set("Authorization", "Bearer test-token")
	}
	
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	return w
}

// Call Management Tests
func TestHandler_CreateCall(t *testing.T) {
	tests := []struct {
		name           string
		request        CreateCallRequest
		setupMocks     func(*MockServices)
		expectedStatus int
		expectedError  string
		validateBody   func(*testing.T, map[string]interface{})
	}{
		{
			name: "successful call creation",
			request: CreateCallRequest{
				FromNumber: "+14155551234",
				ToNumber:   "+18005551234",
				Direction:  "inbound",
			},
			setupMocks: func(m *MockServices) {
				m.Compliance.On("CheckDNC", mock.Anything, "+18005551234").
					Return(false, nil)
			},
			expectedStatus: http.StatusCreated,
			validateBody: func(t *testing.T, body map[string]interface{}) {
				assert.NotEmpty(t, body["id"])
				assert.Equal(t, "pending", body["status"])
				assert.Equal(t, "+14155551234", body["from_number"])
			},
		},
		{
			name: "DNC blocked number",
			request: CreateCallRequest{
				FromNumber: "+14155551234",
				ToNumber:   "+18005551234",
			},
			setupMocks: func(m *MockServices) {
				m.Compliance.On("CheckDNC", mock.Anything, "+18005551234").
					Return(true, nil)
			},
			expectedStatus: http.StatusForbidden,
			expectedError:  "Number is on DNC list",
		},
		{
			name: "invalid phone number",
			request: CreateCallRequest{
				FromNumber: "invalid",
				ToNumber:   "+18005551234",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid from number",
		},
		{
			name: "missing required fields",
			request: CreateCallRequest{
				FromNumber: "",
				ToNumber:   "+18005551234",
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid from number",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, mocks := setupHandler(t)
			
			if tt.setupMocks != nil {
				tt.setupMocks(mocks)
			}
			
			w := makeRequest(handler, "POST", "/api/v1/calls", tt.request)
			
			assert.Equal(t, tt.expectedStatus, w.Code)
			
			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			
			if tt.expectedError != "" {
				assert.Contains(t, w.Body.String(), tt.expectedError)
			}
			
			if tt.validateBody != nil {
				tt.validateBody(t, response)
			}
			
			// Verify mock expectations
			mocks.AssertExpectations(t)
		})
	}
}

func TestHandler_RouteCall(t *testing.T) {
	callID := uuid.New()
	
	tests := []struct {
		name           string
		callID         string
		setupMocks     func(*MockServices)
		expectedStatus int
		expectedError  string
	}{
		{
			name:   "successful routing",
			callID: callID.String(),
			setupMocks: func(m *MockServices) {
				decision := &callrouting.RoutingDecision{
					CallID:    callID,
					BidID:     uuid.New(),
					BuyerID:   uuid.New(),
					SellerID:  uuid.New(),
					Algorithm: "round-robin",
					Score:     0.85,
					Latency:   500 * time.Microsecond,
				}
				m.CallRouting.On("RouteCall", mock.Anything, callID).
					Return(decision, nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:   "call not found",
			callID: callID.String(),
			setupMocks: func(m *MockServices) {
				m.CallRouting.On("RouteCall", mock.Anything, callID).
					Return(nil, errors.NewNotFoundError("call"))
			},
			expectedStatus: http.StatusNotFound,
			expectedError:  "CALL_NOT_FOUND",
		},
		{
			name:   "invalid call state",
			callID: callID.String(),
			setupMocks: func(m *MockServices) {
				m.CallRouting.On("RouteCall", mock.Anything, callID).
					Return(nil, errors.NewValidationError("INVALID_STATE", "call not in pending state"))
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "INVALID_STATE",
		},
		{
			name:           "invalid UUID",
			callID:         "invalid-uuid",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "INVALID_UUID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, mocks := setupHandler(t)
			
			if tt.setupMocks != nil {
				tt.setupMocks(mocks)
			}
			
			w := makeRequest(handler, "POST", "/api/v1/calls/"+tt.callID+"/route", nil)
			
			assert.Equal(t, tt.expectedStatus, w.Code)
			
			if tt.expectedError != "" {
				assert.Contains(t, w.Body.String(), tt.expectedError)
			}
			
			mocks.AssertExpectations(t)
		})
	}
}

// Bid Profile Tests
func TestHandler_CreateBidProfile(t *testing.T) {
	tests := []struct {
		name           string
		userType       string
		request        BidProfileRequest
		expectedStatus int
		validateBody   func(*testing.T, map[string]interface{})
	}{
		{
			name:     "seller creates bid profile",
			userType: "seller",
			request: BidProfileRequest{
				Criteria: bid.BidCriteria{
					Geography: bid.GeoCriteria{
						Countries: []string{"US"},
						States:    []string{"CA", "NY"},
					},
					MaxBudget: values.MustNewMoneyFromFloat(100.00, values.USD),
					CallType:  []string{"sales"},
				},
				Active: true,
			},
			expectedStatus: http.StatusCreated,
			validateBody: func(t *testing.T, body map[string]interface{}) {
				assert.NotEmpty(t, body["id"])
				assert.Equal(t, true, body["active"])
				criteria := body["criteria"].(map[string]interface{})
				assert.NotNil(t, criteria)
			},
		},
		{
			name:     "buyer cannot create bid profile",
			userType: "buyer",
			request: BidProfileRequest{
				Criteria: bid.BidCriteria{
					MaxBudget: values.MustNewMoneyFromFloat(100.00, values.USD),
				},
				Active: true,
			},
			expectedStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, _ := setupHandler(t)
			
			// Add user context
			req := httptest.NewRequest("POST", "/api/v1/bid-profiles", nil)
			ctx := context.WithValue(req.Context(), "account_type", tt.userType)
			req = req.WithContext(ctx)
			
			jsonBody, _ := json.Marshal(tt.request)
			req.Body = io.NopCloser(bytes.NewReader(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer test-token")
			
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			
			assert.Equal(t, tt.expectedStatus, w.Code)
			
			if tt.validateBody != nil {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				tt.validateBody(t, response)
			}
		})
	}
}

// Auction Tests
func TestHandler_CreateAuction(t *testing.T) {
	callID := uuid.New()
	
	tests := []struct {
		name           string
		request        CreateAuctionRequest
		setupMocks     func(*MockServices)
		expectedStatus int
		expectedError  string
	}{
		{
			name: "successful auction creation",
			request: CreateAuctionRequest{
				CallID:       callID,
				ReservePrice: 2.00,
				Duration:     30,
			},
			setupMocks: func(m *MockServices) {
				auction := &bid.Auction{
					ID:           uuid.New(),
					CallID:       callID,
					Status:       bid.AuctionStatusActive,
					ReservePrice: values.MustNewMoneyFromFloat(2.00, values.USD),
				}
				m.Auction.On("CreateAuction", mock.Anything, mock.AnythingOfType("*bid.Auction")).
					Return(nil).
					Run(func(args mock.Arguments) {
						// Populate the auction ID
						a := args.Get(1).(*bid.Auction)
						a.ID = auction.ID
					})
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name: "negative reserve price",
			request: CreateAuctionRequest{
				CallID:       callID,
				ReservePrice: -1.00,
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Reserve price cannot be negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, mocks := setupHandler(t)
			
			if tt.setupMocks != nil {
				tt.setupMocks(mocks)
			}
			
			w := makeRequest(handler, "POST", "/api/v1/auctions", tt.request)
			
			assert.Equal(t, tt.expectedStatus, w.Code)
			
			if tt.expectedError != "" {
				assert.Contains(t, w.Body.String(), tt.expectedError)
			}
			
			mocks.AssertExpectations(t)
		})
	}
}

// Compliance Tests
func TestHandler_ComplianceEndpoints(t *testing.T) {
	t.Run("add to DNC list", func(t *testing.T) {
		handler, mocks := setupHandler(t)
		
		request := AddDNCRequest{
			PhoneNumber: "+14155551234",
			Reason:      "consumer request",
		}
		
		mocks.Compliance.On("AddToDNC", mock.Anything, mock.AnythingOfType("*compliance.DNCEntry")).
			Return(nil)
		
		w := makeRequest(handler, "POST", "/api/v1/compliance/dnc", request)
		
		assert.Equal(t, http.StatusCreated, w.Code)
		
		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		
		assert.Equal(t, "+14155551234", response["phone_number"])
		assert.Equal(t, "internal", response["list_type"])
		
		mocks.AssertExpectations(t)
	})
	
	t.Run("set TCPA hours", func(t *testing.T) {
		handler, mocks := setupHandler(t)
		
		request := SetTCPAHoursRequest{
			StartTime: "09:00",
			EndTime:   "20:00",
			TimeZone:  "America/New_York",
		}
		
		mocks.Compliance.On("SetTCPAHours", mock.Anything, "09:00", "20:00", "America/New_York").
			Return(nil)
		
		w := makeRequest(handler, "PUT", "/api/v1/compliance/tcpa/hours", request)
		
		assert.Equal(t, http.StatusOK, w.Code)
		
		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		
		assert.Equal(t, "09:00", response["start_time"])
		assert.Equal(t, "20:00", response["end_time"])
		
		mocks.AssertExpectations(t)
	})
}
```

### 2. Mock Implementations

#### `internal/api/rest/mocks.go`

```go
package rest

import (
	"context"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/bid"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/compliance"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/service/bidding"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/service/callrouting"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/service/fraud"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/service/telephony"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

// MockServices holds all mock services
type MockServices struct {
	CallRouting *MockCallRoutingService
	Bidding     *MockBiddingService
	Telephony   *MockTelephonyService
	Fraud       *MockFraudService
	Account     *MockAccountService
	Compliance  *MockComplianceService
	Auction     *MockAuctionService
}

// NewMockServices creates all mock services
func NewMockServices() *MockServices {
	return &MockServices{
		CallRouting: new(MockCallRoutingService),
		Bidding:     new(MockBiddingService),
		Telephony:   new(MockTelephonyService),
		Fraud:       new(MockFraudService),
		Account:     new(MockAccountService),
		Compliance:  new(MockComplianceService),
		Auction:     new(MockAuctionService),
	}
}

// AssertExpectations asserts all mock expectations
func (m *MockServices) AssertExpectations(t mock.TestingT) {
	m.CallRouting.AssertExpectations(t)
	m.Bidding.AssertExpectations(t)
	m.Telephony.AssertExpectations(t)
	m.Fraud.AssertExpectations(t)
	m.Account.AssertExpectations(t)
	m.Compliance.AssertExpectations(t)
	m.Auction.AssertExpectations(t)
}

// MockCallRoutingService mocks the call routing service
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

// MockBiddingService mocks the bidding service
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

// MockAccountService mocks the account service
type MockAccountService struct {
	mock.Mock
}

func (m *MockAccountService) GetBalance(ctx context.Context, accountID uuid.UUID) (float64, error) {
	args := m.Called(ctx, accountID)
	return args.Get(0).(float64), args.Error(1)
}

func (m *MockAccountService) UpdateBalance(ctx context.Context, accountID uuid.UUID, amount float64) error {
	args := m.Called(ctx, accountID, amount)
	return args.Error(0)
}

// MockComplianceService mocks the compliance service
type MockComplianceService struct {
	mock.Mock
}

func (m *MockComplianceService) CheckDNC(ctx context.Context, phoneNumber string) (bool, error) {
	args := m.Called(ctx, phoneNumber)
	return args.Get(0).(bool), args.Error(1)
}

func (m *MockComplianceService) AddToDNC(ctx context.Context, entry *compliance.DNCEntry) error {
	args := m.Called(ctx, entry)
	return args.Error(0)
}

func (m *MockComplianceService) CheckTCPA(ctx context.Context, phoneNumber string, callTime time.Time) (bool, error) {
	args := m.Called(ctx, phoneNumber, callTime)
	return args.Get(0).(bool), args.Error(1)
}

func (m *MockComplianceService) SetTCPAHours(ctx context.Context, startTime, endTime string, timezone string) error {
	args := m.Called(ctx, startTime, endTime, timezone)
	return args.Error(0)
}

// MockAuctionService mocks the auction service
type MockAuctionService struct {
	mock.Mock
}

func (m *MockAuctionService) CreateAuction(ctx context.Context, auction *bid.Auction) error {
	args := m.Called(ctx, auction)
	return args.Error(0)
}

func (m *MockAuctionService) GetAuction(ctx context.Context, auctionID uuid.UUID) (*bid.Auction, error) {
	args := m.Called(ctx, auctionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*bid.Auction), args.Error(1)
}

func (m *MockAuctionService) CompleteAuction(ctx context.Context, auctionID uuid.UUID) (*bid.Auction, error) {
	args := m.Called(ctx, auctionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*bid.Auction), args.Error(1)
}

// MockTelephonyService and MockFraudService would follow the same pattern...
```

## E2E Tests Enhancement

### 1. Enhanced Call Flow Tests

#### `test/e2e/call_exchange_flow_enhanced_test.go`

```go
//go:build e2e

package e2e

import (
	"sync"
	"testing"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/bid"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/call"
	"github.com/davidleathers/dependable-call-exchange-backend/test/e2e/infrastructure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCallExchangeFlow_Enhanced(t *testing.T) {
	env := infrastructure.NewTestEnvironment(t)
	client := infrastructure.NewAPIClient(t, env.APIURL)
	
	t.Run("Complete Bid Profile Lifecycle", func(t *testing.T) {
		env.ResetDatabase()
		
		// Create seller account
		seller := createAuthenticatedUser(t, client, "seller@example.com", "seller")
		client.SetToken(seller.Token)
		
		// Create bid profile
		profileResp := client.Post("/api/v1/bid-profiles", map[string]interface{}{
			"criteria": map[string]interface{}{
				"geography": map[string]interface{}{
					"countries": []string{"US"},
					"states":    []string{"CA", "NY", "TX"},
				},
				"call_type":  []string{"sales", "support"},
				"max_budget": 150.00,
				"keywords":   []string{"insurance", "auto", "home"},
			},
			"active": true,
		})
		require.Equal(t, 201, profileResp.StatusCode)
		
		var profile bid.BidProfile
		client.DecodeResponse(profileResp, &profile)
		
		// Update bid profile
		updateResp := client.Put("/api/v1/bid-profiles/"+profile.ID.String(), map[string]interface{}{
			"criteria": map[string]interface{}{
				"max_budget": 200.00,
			},
		})
		assert.Equal(t, 200, updateResp.StatusCode)
		
		// List bid profiles
		listResp := client.Get("/api/v1/bid-profiles")
		assert.Equal(t, 200, listResp.StatusCode)
		
		var profiles []bid.BidProfile
		client.DecodeResponse(listResp, &profiles)
		assert.Len(t, profiles, 1)
		
		// Delete bid profile
		deleteResp := client.Delete("/api/v1/bid-profiles/" + profile.ID.String())
		assert.Equal(t, 204, deleteResp.StatusCode)
	})
	
	t.Run("Multi-Seller Auction Competition", func(t *testing.T) {
		env.ResetDatabase()
		
		// Create multiple sellers with different bid profiles
		sellers := []struct {
			email    string
			criteria bid.BidCriteria
		}{
			{
				email: "premium-seller@example.com",
				criteria: bid.BidCriteria{
					Geography: bid.GeoCriteria{Countries: []string{"US"}},
					CallType:  []string{"sales"},
					MaxBudget: values.MustNewMoneyFromFloat(50.00, values.USD),
				},
			},
			{
				email: "budget-seller@example.com",
				criteria: bid.BidCriteria{
					Geography: bid.GeoCriteria{Countries: []string{"US", "CA"}},
					CallType:  []string{"sales", "support"},
					MaxBudget: values.MustNewMoneyFromFloat(25.00, values.USD),
				},
			},
			{
				email: "specialized-seller@example.com",
				criteria: bid.BidCriteria{
					Geography: bid.GeoCriteria{
						Countries: []string{"US"},
						States:    []string{"CA", "NY"},
					},
					CallType:  []string{"sales"},
					Keywords:  []string{"insurance", "finance"},
					MaxBudget: values.MustNewMoneyFromFloat(75.00, values.USD),
				},
			},
		}
		
		// Create sellers and bid profiles
		for _, s := range sellers {
			auth := createAuthenticatedUser(t, client, s.email, "seller")
			client.SetToken(auth.Token)
			
			resp := client.Post("/api/v1/bid-profiles", map[string]interface{}{
				"criteria": s.criteria,
				"active":   true,
			})
			require.Equal(t, 201, resp.StatusCode)
		}
		
		// Create buyer and incoming call
		buyer := createAuthenticatedUser(t, client, "buyer@example.com", "buyer")
		client.SetToken(buyer.Token)
		
		callResp := client.Post("/api/v1/calls", map[string]interface{}{
			"from_number": "+14155551234",
			"to_number":   "+18005551234",
			"direction":   "inbound",
		})
		require.Equal(t, 201, callResp.StatusCode)
		
		var incomingCall call.Call
		client.DecodeResponse(callResp, &incomingCall)
		
		// Create auction
		auctionResp := client.Post("/api/v1/auctions", map[string]interface{}{
			"call_id":       incomingCall.ID,
			"reserve_price": 1.00,
			"duration":      10, // 10 second auction
		})
		require.Equal(t, 201, auctionResp.StatusCode)
		
		var auction bid.Auction
		client.DecodeResponse(auctionResp, &auction)
		
		// Sellers place bids concurrently
		var wg sync.WaitGroup
		bidAmounts := []float64{15.50, 8.25, 22.00}
		
		for i, s := range sellers {
			wg.Add(1)
			go func(idx int, sellerEmail string, amount float64) {
				defer wg.Done()
				
				// Create new client for concurrent requests
				sellerClient := infrastructure.NewAPIClient(t, env.APIURL)
				sellerAuth := authenticateAccount(t, sellerClient, sellerEmail)
				sellerClient.SetToken(sellerAuth.Token)
				
				bidResp := sellerClient.Post("/api/v1/bids", map[string]interface{}{
					"auction_id": auction.ID,
					"amount":     amount,
				})
				assert.Equal(t, 201, bidResp.StatusCode)
			}(i, s.email, bidAmounts[i])
		}
		
		wg.Wait()
		
		// Complete auction
		client.SetToken(buyer.Token)
		completeResp := client.Post("/api/v1/auctions/"+auction.ID.String()+"/complete", nil)
		require.Equal(t, 200, completeResp.StatusCode)
		
		var completedAuction bid.Auction
		client.DecodeResponse(completeResp, &completedAuction)
		
		// Verify highest bidder won
		assert.Equal(t, bid.AuctionStatusCompleted, completedAuction.Status)
		assert.NotNil(t, completedAuction.WinningBid)
		
		// Route call to winner
		routeResp := client.Post("/api/v1/calls/"+incomingCall.ID.String()+"/route", nil)
		assert.Equal(t, 200, routeResp.StatusCode)
		
		// Progress call through lifecycle
		statusUpdates := []string{"ringing", "in_progress"}
		for _, status := range statusUpdates {
			updateResp := client.Patch("/api/v1/calls/"+incomingCall.ID.String()+"/status", 
				map[string]interface{}{"status": status})
			assert.Equal(t, 200, updateResp.StatusCode)
			time.Sleep(100 * time.Millisecond) // Simulate real timing
		}
		
		// Complete call
		completeCallResp := client.Post("/api/v1/calls/"+incomingCall.ID.String()+"/complete", 
			map[string]interface{}{"duration": 240}) // 4 minutes
		assert.Equal(t, 200, completeCallResp.StatusCode)
		
		// Verify billing
		var completedCall call.Call
		client.DecodeResponse(completeCallResp, &completedCall)
		assert.Greater(t, completedCall.Cost.ToFloat64(), 0.0)
		
		// Check balances
		balanceResp := client.Get("/api/v1/account/balance")
		assert.Equal(t, 200, balanceResp.StatusCode)
		
		var balance map[string]float64
		client.DecodeResponse(balanceResp, &balance)
		assert.Less(t, balance["balance"], 1000.0) // Balance reduced
	})
}

func TestCallExchangeFlow_ErrorScenarios(t *testing.T) {
	env := infrastructure.NewTestEnvironment(t)
	client := infrastructure.NewAPIClient(t, env.APIURL)
	
	t.Run("Routing Without Bids", func(t *testing.T) {
		env.ResetDatabase()
		
		buyer := createAuthenticatedUser(t, client, "buyer@example.com", "buyer")
		client.SetToken(buyer.Token)
		
		// Create call
		callResp := client.Post("/api/v1/calls", map[string]interface{}{
			"from_number": "+14155551234",
			"to_number":   "+18005551234",
		})
		require.Equal(t, 201, callResp.StatusCode)
		
		var call call.Call
		client.DecodeResponse(callResp, &call)
		
		// Try to route without any bids
		routeResp := client.Post("/api/v1/calls/"+call.ID.String()+"/route", nil)
		assert.Equal(t, 400, routeResp.StatusCode)
		assert.Contains(t, routeResp.Body.String(), "NO_BIDS_AVAILABLE")
	})
	
	t.Run("Complete Call Without Routing", func(t *testing.T) {
		env.ResetDatabase()
		
		buyer := createAuthenticatedUser(t, client, "buyer@example.com", "buyer")
		client.SetToken(buyer.Token)
		
		// Create call
		callResp := client.Post("/api/v1/calls", map[string]interface{}{
			"from_number": "+14155551234",
			"to_number":   "+18005551234",
		})
		require.Equal(t, 201, callResp.StatusCode)
		
		var call call.Call
		client.DecodeResponse(callResp, &call)
		
		// Try to complete without routing
		completeResp := client.Post("/api/v1/calls/"+call.ID.String()+"/complete", 
			map[string]interface{}{"duration": 180})
		assert.Equal(t, 400, completeResp.StatusCode)
		assert.Contains(t, completeResp.Body.String(), "INVALID_STATE")
	})
}
```

### 2. Compliance Testing

#### `test/e2e/compliance_test.go`

```go
//go:build e2e

package e2e

import (
	"testing"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/test/e2e/infrastructure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompliance_DNCManagement(t *testing.T) {
	env := infrastructure.NewTestEnvironment(t)
	client := infrastructure.NewAPIClient(t, env.APIURL)
	
	t.Run("DNC List Operations", func(t *testing.T) {
		env.ResetDatabase()
		
		// Create admin user
		admin := createAuthenticatedUser(t, client, "admin@example.com", "admin")
		client.SetToken(admin.Token)
		
		// Add number to DNC list
		dncResp := client.Post("/api/v1/compliance/dnc", map[string]interface{}{
			"phone_number": "+14155551234",
			"reason":       "consumer request",
		})
		require.Equal(t, 201, dncResp.StatusCode)
		
		// Create buyer
		buyer := createAuthenticatedUser(t, client, "buyer@example.com", "buyer")
		client.SetToken(buyer.Token)
		
		// Try to call DNC number
		callResp := client.Post("/api/v1/calls", map[string]interface{}{
			"from_number": "+18005551234",
			"to_number":   "+14155551234",
		})
		assert.Equal(t, 403, callResp.StatusCode)
		assert.Contains(t, callResp.Body.String(), "DNC")
		
		// Check DNC status
		client.SetToken(admin.Token)
		checkResp := client.Get("/api/v1/compliance/dnc/+14155551234")
		assert.Equal(t, 200, checkResp.StatusCode)
		
		var dncStatus map[string]interface{}
		client.DecodeResponse(checkResp, &dncStatus)
		assert.Equal(t, true, dncStatus["is_dnc"])
		assert.Equal(t, "consumer request", dncStatus["reason"])
	})
	
	t.Run("TCPA Time Restrictions", func(t *testing.T) {
		env.ResetDatabase()
		
		admin := createAuthenticatedUser(t, client, "admin@example.com", "admin")
		client.SetToken(admin.Token)
		
		// Set TCPA hours
		tcpaResp := client.Put("/api/v1/compliance/tcpa/hours", map[string]interface{}{
			"start_time": "09:00",
			"end_time":   "20:00",
			"timezone":   "America/New_York",
		})
		require.Equal(t, 200, tcpaResp.StatusCode)
		
		// Get TCPA hours
		getResp := client.Get("/api/v1/compliance/tcpa/hours")
		assert.Equal(t, 200, getResp.StatusCode)
		
		var tcpaHours map[string]interface{}
		client.DecodeResponse(getResp, &tcpaHours)
		assert.Equal(t, "09:00", tcpaHours["start_time"])
		assert.Equal(t, "20:00", tcpaHours["end_time"])
		
		// Test time-based call blocking (would need time mocking)
		// This is a placeholder for more sophisticated time-based tests
		buyer := createAuthenticatedUser(t, client, "buyer@example.com", "buyer")
		client.SetToken(buyer.Token)
		
		// Verify TCPA info is available
		callResp := client.Post("/api/v1/calls", map[string]interface{}{
			"from_number": "+18005551234",
			"to_number":   "+14155551234",
		})
		// During allowed hours, should succeed
		// Outside hours, should return 403
		// Actual behavior depends on current time
		assert.Contains(t, []int{201, 403}, callResp.StatusCode)
	})
}

func TestCompliance_GeographicRestrictions(t *testing.T) {
	env := infrastructure.NewTestEnvironment(t)
	client := infrastructure.NewAPIClient(t, env.APIURL)
	
	t.Run("State-Level Restrictions", func(t *testing.T) {
		env.ResetDatabase()
		
		// Setup geographic restrictions
		admin := createAuthenticatedUser(t, client, "admin@example.com", "admin")
		client.SetToken(admin.Token)
		
		// Add state restriction
		restrictResp := client.Post("/api/v1/compliance/geographic/restrictions", map[string]interface{}{
			"state":      "CA",
			"restricted": true,
			"reason":     "state regulations",
		})
		require.Equal(t, 201, restrictResp.StatusCode)
		
		buyer := createAuthenticatedUser(t, client, "buyer@example.com", "buyer")
		client.SetToken(buyer.Token)
		
		// Try to call California number
		callResp := client.Post("/api/v1/calls", map[string]interface{}{
			"from_number": "+18005551234",
			"to_number":   "+14155551234", // 415 is San Francisco
		})
		assert.Equal(t, 403, callResp.StatusCode)
		assert.Contains(t, callResp.Body.String(), "geographic restriction")
	})
}
```

## API Contract Testing

### 1. OpenAPI Specification

#### `api/openapi.yaml`

```yaml
openapi: 3.0.3
info:
  title: Dependable Call Exchange API
  version: 1.0.0
  description: Pay-per-call marketplace platform API
servers:
  - url: http://localhost:8080/api/v1
paths:
  /calls:
    post:
      summary: Create a new call
      operationId: createCall
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/CreateCallRequest'
      responses:
        '201':
          description: Call created
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Call'
        '400':
          $ref: '#/components/responses/BadRequest'
        '403':
          $ref: '#/components/responses/Forbidden'
  
  /calls/{id}/route:
    post:
      summary: Route call to buyer
      operationId: routeCall
      parameters:
        - $ref: '#/components/parameters/CallId'
      responses:
        '200':
          description: Call routed
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/RoutedCall'
        '404':
          $ref: '#/components/responses/NotFound'

components:
  schemas:
    CreateCallRequest:
      type: object
      required:
        - from_number
        - to_number
      properties:
        from_number:
          type: string
          pattern: '^\+[1-9]\d{1,14}$'
          example: '+14155551234'
        to_number:
          type: string
          pattern: '^\+[1-9]\d{1,14}$'
          example: '+18005551234'
        direction:
          type: string
          enum: [inbound, outbound]
          default: outbound
    
    Call:
      type: object
      properties:
        id:
          type: string
          format: uuid
        from_number:
          type: string
        to_number:
          type: string
        status:
          type: string
          enum: [pending, queued, ringing, in_progress, completed, failed, canceled, no_answer, busy]
        direction:
          type: string
          enum: [inbound, outbound]
        buyer_id:
          type: string
          format: uuid
        seller_id:
          type: string
          format: uuid
        start_time:
          type: string
          format: date-time
        end_time:
          type: string
          format: date-time
        duration:
          type: integer
          description: Duration in seconds
        cost:
          $ref: '#/components/schemas/Money'
```

### 2. Contract Test Implementation

#### `test/contract/api_contract_test.go`

```go
//go:build contract

package contract

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/api/rest"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIContract(t *testing.T) {
	// Load OpenAPI spec
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromFile("../../api/openapi.yaml")
	require.NoError(t, err)
	
	// Validate spec
	err = doc.Validate(loader.Context)
	require.NoError(t, err)
	
	// Create router from spec
	router, err := gorillamux.NewRouter(doc)
	require.NoError(t, err)
	
	// Create handler with mocks
	handler, mocks := setupTestHandler()
	
	tests := []struct {
		name     string
		method   string
		path     string
		body     interface{}
		setup    func()
		validate func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:   "create call - valid request",
			method: "POST",
			path:   "/api/v1/calls",
			body: map[string]interface{}{
				"from_number": "+14155551234",
				"to_number":   "+18005551234",
				"direction":   "inbound",
			},
			setup: func() {
				mocks.Compliance.On("CheckDNC", mock.Anything, "+18005551234").
					Return(false, nil)
			},
			validate: func(t *testing.T, w *httptest.ResponseRecorder) {
				assert.Equal(t, 201, w.Code)
				validateResponse(t, router, "POST", "/api/v1/calls", w)
			},
		},
		{
			name:   "create call - invalid phone number",
			method: "POST",
			path:   "/api/v1/calls",
			body: map[string]interface{}{
				"from_number": "invalid",
				"to_number":   "+18005551234",
			},
			validate: func(t *testing.T, w *httptest.ResponseRecorder) {
				assert.Equal(t, 400, w.Code)
				// Should still match error response schema
				validateResponse(t, router, "POST", "/api/v1/calls", w)
			},
		},
		{
			name:   "route call - valid request",
			method: "POST",
			path:   "/api/v1/calls/550e8400-e29b-41d4-a716-446655440000/route",
			setup: func() {
				// Setup routing mock
			},
			validate: func(t *testing.T, w *httptest.ResponseRecorder) {
				validateResponse(t, router, "POST", "/api/v1/calls/{id}/route", w)
			},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup()
			}
			
			// Make request
			var body io.Reader
			if tt.body != nil {
				jsonBody, _ := json.Marshal(tt.body)
				body = bytes.NewReader(jsonBody)
			}
			
			req := httptest.NewRequest(tt.method, tt.path, body)
			if body != nil {
				req.Header.Set("Content-Type", "application/json")
			}
			req.Header.Set("Authorization", "Bearer test-token")
			
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			
			// Validate against contract
			tt.validate(t, w)
			
			mocks.AssertExpectations(t)
		})
	}
}

func validateResponse(t *testing.T, router routers.Router, method, path string, w *httptest.ResponseRecorder) {
	route, pathParams, err := router.FindRoute(method, path)
	require.NoError(t, err)
	
	// Validate response
	responseRef := route.Operation.Responses[strconv.Itoa(w.Code)]
	require.NotNil(t, responseRef, "Response %d not defined in spec", w.Code)
	
	// Create request/response for validation
	req := &http.Request{
		Method: method,
		URL:    &url.URL{Path: path},
	}
	
	// Validate response body against schema
	requestValidationInput := &openapi3filter.RequestValidationInput{
		Request:     req,
		PathParams:  pathParams,
		Route:       route,
	}
	
	responseValidationInput := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: requestValidationInput,
		Status:                 w.Code,
		Header:                 w.Header(),
		Body:                   io.NopCloser(bytes.NewReader(w.Body.Bytes())),
	}
	
	err = openapi3filter.ValidateResponse(context.Background(), responseValidationInput)
	assert.NoError(t, err, "Response does not match OpenAPI schema")
}
```

## Security Testing

### 1. Authentication & Authorization Tests

#### `test/security/auth_security_test.go`

```go
//go:build security

package security

import (
	"testing"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/test/e2e/infrastructure"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSecurity_Authentication(t *testing.T) {
	env := infrastructure.NewTestEnvironment(t)
	client := infrastructure.NewAPIClient(t, env.APIURL)
	
	t.Run("JWT Token Validation", func(t *testing.T) {
		tests := []struct {
			name           string
			token          string
			expectedStatus int
		}{
			{
				name:           "missing token",
				token:          "",
				expectedStatus: 401,
			},
			{
				name:           "invalid format",
				token:          "invalid-token",
				expectedStatus: 401,
			},
			{
				name:           "expired token",
				token:          generateExpiredToken(t),
				expectedStatus: 401,
			},
			{
				name:           "invalid signature",
				token:          generateInvalidSignatureToken(t),
				expectedStatus: 401,
			},
			{
				name:           "valid token",
				token:          generateValidToken(t),
				expectedStatus: 200,
			},
		}
		
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				if tt.token != "" {
					client.SetToken(tt.token)
				} else {
					client.SetToken("") // Clear token
				}
				
				resp := client.Get("/api/v1/profile")
				assert.Equal(t, tt.expectedStatus, resp.StatusCode)
			})
		}
	})
	
	t.Run("Role-Based Access Control", func(t *testing.T) {
		endpoints := []struct {
			path         string
			method       string
			allowedRoles []string
			body         interface{}
		}{
			{
				path:         "/api/v1/bid-profiles",
				method:       "POST",
				allowedRoles: []string{"seller", "admin"},
				body:         map[string]interface{}{"criteria": map[string]interface{}{"max_budget": 100}},
			},
			{
				path:         "/api/v1/calls",
				method:       "POST",
				allowedRoles: []string{"buyer", "admin"},
				body:         map[string]interface{}{"from_number": "+14155551234", "to_number": "+18005551234"},
			},
			{
				path:         "/api/v1/compliance/dnc",
				method:       "POST",
				allowedRoles: []string{"admin"},
				body:         map[string]interface{}{"phone_number": "+14155551234", "reason": "test"},
			},
			{
				path:         "/api/v1/admin/users",
				method:       "GET",
				allowedRoles: []string{"admin"},
			},
		}
		
		roles := []string{"buyer", "seller", "admin"}
		
		for _, endpoint := range endpoints {
			for _, role := range roles {
				t.Run(endpoint.path+"_"+role, func(t *testing.T) {
					// Create user with role
					user := createAuthenticatedUser(t, client, role+"@test.com", role)
					client.SetToken(user.Token)
					
					// Make request
					var resp *http.Response
					switch endpoint.method {
					case "GET":
						resp = client.Get(endpoint.path)
					case "POST":
						resp = client.Post(endpoint.path, endpoint.body)
					}
					
					// Check access
					if contains(endpoint.allowedRoles, role) {
						assert.NotEqual(t, 403, resp.StatusCode, 
							"%s should have access to %s", role, endpoint.path)
					} else {
						assert.Equal(t, 403, resp.StatusCode,
							"%s should NOT have access to %s", role, endpoint.path)
					}
				})
			}
		}
	})
}

func TestSecurity_InputValidation(t *testing.T) {
	env := infrastructure.NewTestEnvironment(t)
	client := infrastructure.NewAPIClient(t, env.APIURL)
	
	buyer := createAuthenticatedUser(t, client, "buyer@test.com", "buyer")
	client.SetToken(buyer.Token)
	
	t.Run("SQL Injection Prevention", func(t *testing.T) {
		sqlInjectionPayloads := []string{
			"'; DROP TABLE calls; --",
			"1' OR '1'='1",
			"admin'--",
			"1; UPDATE accounts SET balance = 999999",
		}
		
		for _, payload := range sqlInjectionPayloads {
			resp := client.Post("/api/v1/calls", map[string]interface{}{
				"from_number": payload,
				"to_number":   "+18005551234",
			})
			
			// Should fail validation, not execute SQL
			assert.Equal(t, 400, resp.StatusCode)
			assert.Contains(t, resp.Body.String(), "Invalid")
		}
	})
	
	t.Run("XSS Prevention", func(t *testing.T) {
		xssPayloads := []string{
			"<script>alert('XSS')</script>",
			"javascript:alert('XSS')",
			"<img src=x onerror=alert('XSS')>",
			"<iframe src='javascript:alert(\"XSS\")'></iframe>",
		}
		
		for _, payload := range xssPayloads {
			resp := client.Post("/api/v1/accounts", map[string]interface{}{
				"email":        "test@example.com",
				"company_name": payload,
				"type":         "buyer",
			})
			
			// Check response doesn't reflect unescaped payload
			assert.NotContains(t, resp.Body.String(), "<script>")
			assert.NotContains(t, resp.Body.String(), "javascript:")
		}
	})
	
	t.Run("Rate Limiting", func(t *testing.T) {
		// Make many requests rapidly
		hitRateLimit := false
		for i := 0; i < 200; i++ {
			resp := client.Get("/api/v1/calls")
			if resp.StatusCode == 429 {
				hitRateLimit = true
				break
			}
		}
		
		assert.True(t, hitRateLimit, "Rate limiting should be enforced")
	})
}

func TestSecurity_DataProtection(t *testing.T) {
	env := infrastructure.NewTestEnvironment(t)
	client := infrastructure.NewAPIClient(t, env.APIURL)
	
	t.Run("Sensitive Data Masking", func(t *testing.T) {
		// Create account
		resp := client.Post("/api/v1/accounts", map[string]interface{}{
			"email":         "sensitive@example.com",
			"company_name":  "Test Corp",
			"type":          "buyer",
			"payment_info": map[string]interface{}{
				"card_number": "4111111111111111",
				"cvv":         "123",
			},
		})
		
		// Response should not contain full card number
		assert.NotContains(t, resp.Body.String(), "4111111111111111")
		assert.Contains(t, resp.Body.String(), "****1111") // Masked
	})
	
	t.Run("HTTPS Enforcement", func(t *testing.T) {
		// In production, test that HTTP redirects to HTTPS
		// This is a placeholder for environment-specific tests
		t.Skip("HTTPS enforcement tested in production environment")
	})
}

// Helper functions
func generateValidToken(t *testing.T) string {
	claims := jwt.MapClaims{
		"sub":  "test-user-123",
		"role": "buyer",
		"exp":  time.Now().Add(time.Hour).Unix(),
		"iat":  time.Now().Unix(),
	}
	
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte("test-secret"))
	require.NoError(t, err)
	
	return signedToken
}

func generateExpiredToken(t *testing.T) string {
	claims := jwt.MapClaims{
		"sub":  "test-user-123",
		"role": "buyer",
		"exp":  time.Now().Add(-time.Hour).Unix(), // Expired
		"iat":  time.Now().Add(-2 * time.Hour).Unix(),
	}
	
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte("test-secret"))
	require.NoError(t, err)
	
	return signedToken
}

func generateInvalidSignatureToken(t *testing.T) string {
	claims := jwt.MapClaims{
		"sub":  "test-user-123",
		"role": "buyer",
		"exp":  time.Now().Add(time.Hour).Unix(),
		"iat":  time.Now().Unix(),
	}
	
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte("wrong-secret"))
	require.NoError(t, err)
	
	return signedToken
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
```

## Performance Testing

### 1. Load Testing

#### `test/performance/load_test.go`

```go
//go:build performance

package performance

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/test/e2e/infrastructure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPerformance_APILoad(t *testing.T) {
	env := infrastructure.NewTestEnvironment(t)
	
	// Create test users
	buyers := createTestUsers(t, env, "buyer", 10)
	sellers := createTestUsers(t, env, "seller", 5)
	
	t.Run("Call Creation Load", func(t *testing.T) {
		targetRPS := 1000
		duration := 30 * time.Second
		
		results := runLoadTest(t, env, LoadTestConfig{
			Name:          "Call Creation",
			TargetRPS:     targetRPS,
			Duration:      duration,
			Concurrency:   50,
			TestFunc: func(client *infrastructure.APIClient) error {
				buyer := buyers[rand.Intn(len(buyers))]
				client.SetToken(buyer.Token)
				
				resp := client.Post("/api/v1/calls", map[string]interface{}{
					"from_number": generatePhoneNumber(),
					"to_number":   generatePhoneNumber(),
				})
				
				if resp.StatusCode != 201 {
					return fmt.Errorf("unexpected status: %d", resp.StatusCode)
				}
				
				return nil
			},
		})
		
		// Validate performance metrics
		assert.Greater(t, results.SuccessRate, 0.95, "Success rate should be > 95%")
		assert.Less(t, results.P99Latency, 50*time.Millisecond, "P99 latency should be < 50ms")
		assert.Greater(t, results.ActualRPS, float64(targetRPS)*0.9, "Should achieve 90% of target RPS")
	})
	
	t.Run("Concurrent Bidding Load", func(t *testing.T) {
		// Create auctions
		auctions := createTestAuctions(t, env, buyers[0], 100)
		
		results := runLoadTest(t, env, LoadTestConfig{
			Name:          "Concurrent Bidding",
			TargetRPS:     5000,
			Duration:      30 * time.Second,
			Concurrency:   100,
			TestFunc: func(client *infrastructure.APIClient) error {
				seller := sellers[rand.Intn(len(sellers))]
				client.SetToken(seller.Token)
				
				auction := auctions[rand.Intn(len(auctions))]
				amount := 1.0 + rand.Float64()*50.0
				
				resp := client.Post("/api/v1/bids", map[string]interface{}{
					"auction_id": auction.ID,
					"amount":     amount,
				})
				
				if resp.StatusCode != 201 && resp.StatusCode != 409 {
					return fmt.Errorf("unexpected status: %d", resp.StatusCode)
				}
				
				return nil
			},
		})
		
		// Bidding has stricter latency requirements
		assert.Less(t, results.P99Latency, 10*time.Millisecond, "P99 latency should be < 10ms")
		assert.Greater(t, results.ThroughputPerSecond, 4500, "Should process > 4500 bids/second")
	})
	
	t.Run("Call Routing Performance", func(t *testing.T) {
		// Create calls with bids
		callsWithBids := createCallsWithBids(t, env, buyers, sellers, 50)
		
		results := runLoadTest(t, env, LoadTestConfig{
			Name:        "Call Routing",
			TargetRPS:   500,
			Duration:    30 * time.Second,
			Concurrency: 20,
			TestFunc: func(client *infrastructure.APIClient) error {
				call := callsWithBids[rand.Intn(len(callsWithBids))]
				
				buyer := buyers[0] // Use first buyer's token
				client.SetToken(buyer.Token)
				
				start := time.Now()
				resp := client.Post(fmt.Sprintf("/api/v1/calls/%s/route", call.ID), nil)
				routingTime := time.Since(start)
				
				if resp.StatusCode != 200 && resp.StatusCode != 400 {
					return fmt.Errorf("unexpected status: %d", resp.StatusCode)
				}
				
				// Verify < 1ms routing decision
				if routingTime > 1*time.Millisecond {
					return fmt.Errorf("routing took %v, exceeds 1ms SLA", routingTime)
				}
				
				return nil
			},
		})
		
		// Routing must meet < 1ms SLA
		assert.Less(t, results.P99Latency, 1*time.Millisecond, "P99 routing latency must be < 1ms")
		assert.Equal(t, float64(0), results.SLAViolations, "No SLA violations allowed")
	})
}

func TestPerformance_WebSocketLoad(t *testing.T) {
	env := infrastructure.NewTestEnvironment(t)
	
	t.Run("Concurrent WebSocket Connections", func(t *testing.T) {
		numConnections := 1000
		messagesPerConnection := 100
		
		var wg sync.WaitGroup
		var successCount int64
		var errorCount int64
		
		start := time.Now()
		
		for i := 0; i < numConnections; i++ {
			wg.Add(1)
			go func(connID int) {
				defer wg.Done()
				
				wsClient := infrastructure.NewWebSocketClient(t, env.WSURL+"/ws/events")
				err := wsClient.Connect(fmt.Sprintf("client-%d", connID))
				if err != nil {
					atomic.AddInt64(&errorCount, 1)
					return
				}
				defer wsClient.Close()
				
				// Send and receive messages
				for j := 0; j < messagesPerConnection; j++ {
					// Send message
					err := wsClient.Send(map[string]interface{}{
						"action": "ping",
						"id":     j,
					})
					if err != nil {
						atomic.AddInt64(&errorCount, 1)
						continue
					}
					
					// Receive response
					var response map[string]interface{}
					err = wsClient.ReceiveWithTimeout(&response, 5*time.Second)
					if err != nil {
						atomic.AddInt64(&errorCount, 1)
						continue
					}
					
					atomic.AddInt64(&successCount, 1)
				}
			}(i)
		}
		
		wg.Wait()
		duration := time.Since(start)
		
		totalMessages := int64(numConnections * messagesPerConnection)
		successRate := float64(successCount) / float64(totalMessages)
		
		t.Logf("WebSocket Load Test Results:")
		t.Logf("- Connections: %d", numConnections)
		t.Logf("- Messages per connection: %d", messagesPerConnection)
		t.Logf("- Total messages: %d", totalMessages)
		t.Logf("- Successful: %d", successCount)
		t.Logf("- Errors: %d", errorCount)
		t.Logf("- Success rate: %.2f%%", successRate*100)
		t.Logf("- Duration: %v", duration)
		t.Logf("- Messages/second: %.2f", float64(successCount)/duration.Seconds())
		
		assert.Greater(t, successRate, 0.95, "WebSocket success rate should be > 95%")
	})
}

// Load Test Helpers

type LoadTestConfig struct {
	Name        string
	TargetRPS   int
	Duration    time.Duration
	Concurrency int
	TestFunc    func(*infrastructure.APIClient) error
}

type LoadTestResults struct {
	TotalRequests       int64
	SuccessfulRequests  int64
	FailedRequests      int64
	SuccessRate         float64
	ActualRPS           float64
	ThroughputPerSecond int
	P50Latency          time.Duration
	P95Latency          time.Duration
	P99Latency          time.Duration
	MaxLatency          time.Duration
	SLAViolations       float64
}

func runLoadTest(t *testing.T, env *infrastructure.TestEnvironment, config LoadTestConfig) LoadTestResults {
	var (
		totalRequests      int64
		successfulRequests int64
		failedRequests     int64
		latencies          []time.Duration
		mu                 sync.Mutex
	)
	
	// Rate limiter
	ticker := time.NewTicker(time.Second / time.Duration(config.TargetRPS))
	defer ticker.Stop()
	
	// Worker pool
	ctx, cancel := context.WithTimeout(context.Background(), config.Duration)
	defer cancel()
	
	var wg sync.WaitGroup
	for i := 0; i < config.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			
			client := infrastructure.NewAPIClient(t, env.APIURL)
			
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					start := time.Now()
					err := config.TestFunc(client)
					duration := time.Since(start)
					
					mu.Lock()
					latencies = append(latencies, duration)
					atomic.AddInt64(&totalRequests, 1)
					if err != nil {
						atomic.AddInt64(&failedRequests, 1)
					} else {
						atomic.AddInt64(&successfulRequests, 1)
					}
					mu.Unlock()
				}
			}
		}()
	}
	
	wg.Wait()
	
	// Calculate results
	return calculateResults(totalRequests, successfulRequests, failedRequests, latencies, config.Duration)
}

func calculateResults(total, successful, failed int64, latencies []time.Duration, duration time.Duration) LoadTestResults {
	if len(latencies) == 0 {
		return LoadTestResults{}
	}
	
	// Sort latencies for percentile calculation
	sort.Slice(latencies, func(i, j int) bool {
		return latencies[i] < latencies[j]
	})
	
	return LoadTestResults{
		TotalRequests:       total,
		SuccessfulRequests:  successful,
		FailedRequests:      failed,
		SuccessRate:         float64(successful) / float64(total),
		ActualRPS:           float64(total) / duration.Seconds(),
		ThroughputPerSecond: int(float64(successful) / duration.Seconds()),
		P50Latency:          latencies[len(latencies)*50/100],
		P95Latency:          latencies[len(latencies)*95/100],
		P99Latency:          latencies[len(latencies)*99/100],
		MaxLatency:          latencies[len(latencies)-1],
	}
}
```

## Test Data Management

### 1. Test Data Builders

#### `test/testdata/builders.go`

```go
package testdata

import (
	"fmt"
	"testing"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/account"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/bid"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/call"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// CallBuilder builds test calls
type CallBuilder struct {
	call *call.Call
}

func NewCallBuilder() *CallBuilder {
	return &CallBuilder{
		call: &call.Call{
			ID:         uuid.New(),
			FromNumber: values.MustNewPhoneNumber("+14155551234"),
			ToNumber:   values.MustNewPhoneNumber("+18005551234"),
			Status:     call.StatusPending,
			Direction:  call.DirectionOutbound,
			BuyerID:    uuid.New(),
			StartTime:  time.Now(),
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		},
	}
}

func (b *CallBuilder) WithID(id uuid.UUID) *CallBuilder {
	b.call.ID = id
	return b
}

func (b *CallBuilder) WithFromNumber(number string) *CallBuilder {
	b.call.FromNumber = values.MustNewPhoneNumber(number)
	return b
}

func (b *CallBuilder) WithToNumber(number string) *CallBuilder {
	b.call.ToNumber = values.MustNewPhoneNumber(number)
	return b
}

func (b *CallBuilder) WithStatus(status call.Status) *CallBuilder {
	b.call.Status = status
	return b
}

func (b *CallBuilder) WithDirection(direction call.Direction) *CallBuilder {
	b.call.Direction = direction
	return b
}

func (b *CallBuilder) WithBuyerID(id uuid.UUID) *CallBuilder {
	b.call.BuyerID = id
	return b
}

func (b *CallBuilder) WithSellerID(id uuid.UUID) *CallBuilder {
	b.call.SellerID = &id
	return b
}

func (b *CallBuilder) Build() *call.Call {
	return b.call
}

// BidBuilder builds test bids
type BidBuilder struct {
	bid *bid.Bid
}

func NewBidBuilder() *BidBuilder {
	return &BidBuilder{
		bid: &bid.Bid{
			ID:       uuid.New(),
			CallID:   uuid.New(),
			BuyerID:  uuid.New(),
			SellerID: uuid.New(),
			Amount:   values.MustNewMoneyFromFloat(5.00, values.USD),
			Status:   bid.StatusActive,
			Quality: values.QualityMetrics{
				HistoricalRating: 0.85,
				CallVolume:       100,
				FraudScore:       0.01,
				CustomerRating:   8.5,
			},
			PlacedAt:  time.Now(),
			ExpiresAt: time.Now().Add(5 * time.Minute),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}
}

func (b *BidBuilder) WithCallID(id uuid.UUID) *BidBuilder {
	b.bid.CallID = id
	return b
}

func (b *BidBuilder) WithBuyerID(id uuid.UUID) *BidBuilder {
	b.bid.BuyerID = id
	return b
}

func (b *BidBuilder) WithSellerID(id uuid.UUID) *BidBuilder {
	b.bid.SellerID = id
	return b
}

func (b *BidBuilder) WithAmount(amount float64) *BidBuilder {
	b.bid.Amount = values.MustNewMoneyFromFloat(amount, values.USD)
	return b
}

func (b *BidBuilder) WithCriteria(criteria bid.BidCriteria) *BidBuilder {
	b.bid.Criteria = criteria
	return b
}

func (b *BidBuilder) Build() *bid.Bid {
	return b.bid
}

// AccountBuilder builds test accounts
type AccountBuilder struct {
	account *account.Account
}

func NewAccountBuilder() *AccountBuilder {
	return &AccountBuilder{
		account: &account.Account{
			ID:          uuid.New(),
			Type:        account.TypeBuyer,
			Email:       values.MustNewEmail("test@example.com"),
			CompanyName: "Test Company",
			Status:      account.StatusActive,
			Balance:     values.MustNewMoneyFromFloat(1000.00, values.USD),
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}
}

func (b *AccountBuilder) WithType(accountType account.AccountType) *AccountBuilder {
	b.account.Type = accountType
	return b
}

func (b *AccountBuilder) WithEmail(email string) *AccountBuilder {
	b.account.Email = values.MustNewEmail(email)
	return b
}

func (b *AccountBuilder) WithCompanyName(name string) *AccountBuilder {
	b.account.CompanyName = name
	return b
}

func (b *AccountBuilder) WithBalance(amount float64) *AccountBuilder {
	b.account.Balance = values.MustNewMoneyFromFloat(amount, values.USD)
	return b
}

func (b *AccountBuilder) WithStatus(status account.Status) *AccountBuilder {
	b.account.Status = status
	return b
}

func (b *AccountBuilder) Build() *account.Account {
	return b.account
}

// Test Data Scenarios

func CreateTestScenario(t *testing.T) *TestScenario {
	scenario := &TestScenario{
		Buyers:  make([]*account.Account, 0),
		Sellers: make([]*account.Account, 0),
		Calls:   make([]*call.Call, 0),
		Bids:    make([]*bid.Bid, 0),
	}
	
	// Create buyers
	for i := 0; i < 3; i++ {
		buyer := NewAccountBuilder().
			WithType(account.TypeBuyer).
			WithEmail(fmt.Sprintf("buyer%d@test.com", i)).
			WithCompanyName(fmt.Sprintf("Buyer Company %d", i)).
			WithBalance(1000.00 + float64(i*500)).
			Build()
		scenario.Buyers = append(scenario.Buyers, buyer)
	}
	
	// Create sellers
	for i := 0; i < 5; i++ {
		seller := NewAccountBuilder().
			WithType(account.TypeSeller).
			WithEmail(fmt.Sprintf("seller%d@test.com", i)).
			WithCompanyName(fmt.Sprintf("Seller Company %d", i)).
			WithBalance(0).
			Build()
		scenario.Sellers = append(scenario.Sellers, seller)
	}
	
	// Create calls
	for i := 0; i < 10; i++ {
		call := NewCallBuilder().
			WithFromNumber(fmt.Sprintf("+1415555%04d", i)).
			WithToNumber("+18005551234").
			WithDirection(call.DirectionInbound).
			WithBuyerID(scenario.Buyers[i%len(scenario.Buyers)].ID).
			Build()
		scenario.Calls = append(scenario.Calls, call)
	}
	
	// Create bids
	for i, call := range scenario.Calls {
		for j := 0; j < 3; j++ {
			bid := NewBidBuilder().
				WithCallID(call.ID).
				WithBuyerID(scenario.Buyers[j%len(scenario.Buyers)].ID).
				WithSellerID(scenario.Sellers[j%len(scenario.Sellers)].ID).
				WithAmount(float64(j+1) * 2.50).
				WithCriteria(bid.BidCriteria{
					CallType: []string{"inbound"},
					Geography: bid.GeoCriteria{
						Countries: []string{"US"},
					},
					MaxBudget: values.MustNewMoneyFromFloat(100.00, values.USD),
				}).
				Build()
			scenario.Bids = append(scenario.Bids, bid)
		}
	}
	
	return scenario
}

type TestScenario struct {
	Buyers  []*account.Account
	Sellers []*account.Account
	Calls   []*call.Call
	Bids    []*bid.Bid
}
```

## CI/CD Integration

### 1. GitHub Actions Workflow

#### `.github/workflows/api-tests.yml`

```yaml
name: API Tests

on:
  push:
    branches: [ main, develop ]
  pull_request:
    branches: [ main ]
  schedule:
    - cron: '0 */4 * * *'  # Every 4 hours

env:
  GO_VERSION: '1.24'
  DOCKER_BUILDKIT: 1

jobs:
  unit-tests:
    name: Unit Tests
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: ${{ env.GO_VERSION }}
          cache: true
      
      - name: Run unit tests
        run: |
          make test-unit
          make coverage
      
      - name: Upload coverage
        uses: codecov/codecov-action@v4
        with:
          file: ./coverage.out
          flags: unit
      
  integration-tests:
    name: Integration Tests
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:15
        env:
          POSTGRES_PASSWORD: test
          POSTGRES_DB: dce_test
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
        ports:
          - 5432:5432
      
    steps:
      - uses: actions/checkout@v4
      
      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: ${{ env.GO_VERSION }}
          cache: true
      
      - name: Run integration tests
        env:
          DCE_DATABASE_URL: postgres://postgres:test@localhost:5432/dce_test?sslmode=disable
        run: |
          make migrate-up
          make test-integration
  
  e2e-tests:
    name: E2E Tests
    runs-on: ubuntu-latest
    strategy:
      matrix:
        suite: [auth, flow, financial, realtime]
    
    steps:
      - uses: actions/checkout@v4
      
      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: ${{ env.GO_VERSION }}
          cache: true
      
      - name: Start Docker
        run: |
          docker info
          docker-compose version
      
      - name: Run E2E test suite - ${{ matrix.suite }}
        run: |
          make test-e2e-${{ matrix.suite }}
      
      - name: Upload test results
        if: always()
        uses: actions/upload-artifact@v4
        with:
          name: e2e-results-${{ matrix.suite }}
          path: test-results/
  
  contract-tests:
    name: API Contract Tests
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: ${{ env.GO_VERSION }}
          cache: true
      
      - name: Validate OpenAPI spec
        run: |
          npm install -g @apidevtools/swagger-cli
          swagger-cli validate api/openapi.yaml
      
      - name: Run contract tests
        run: make test-contract
  
  security-tests:
    name: Security Tests
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: ${{ env.GO_VERSION }}
          cache: true
      
      - name: Run security tests
        run: make test-security
      
      - name: Run vulnerability scan
        run: |
          make vulncheck
          make gosec
  
  performance-tests:
    name: Performance Tests
    runs-on: ubuntu-latest
    if: github.event_name == 'schedule' || contains(github.event.head_commit.message, '[perf]')
    steps:
      - uses: actions/checkout@v4
      
      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: ${{ env.GO_VERSION }}
          cache: true
      
      - name: Run performance tests
        run: make test-e2e-performance
      
      - name: Upload performance results
        uses: actions/upload-artifact@v4
        with:
          name: performance-results
          path: performance-results/
```

### 2. Makefile Additions

Add these targets to your Makefile:

```makefile
# API Testing targets
test-unit: ## Run unit tests only
	$(GOTEST) -v -short ./internal/...

test-contract: ## Run API contract tests
	$(GOTEST) -tags=contract -v ./test/contract/...

test-security: ## Run security tests
	$(GOTEST) -tags=security -v ./test/security/...

test-api: test-unit test-contract ## Run all API tests

test-all: test-unit test-integration test-e2e test-contract test-security ## Run all tests

# Coverage targets
coverage-unit: ## Unit test coverage
	$(GOTEST) -coverprofile=coverage-unit.out -covermode=atomic ./internal/...
	$(GOTOOL) cover -html=coverage-unit.out -o coverage-unit.html

coverage-integration: ## Integration test coverage
	$(GOTEST) -tags=integration -coverprofile=coverage-integration.out -covermode=atomic ./test/integration/...
	$(GOTOOL) cover -html=coverage-integration.out -o coverage-integration.html

coverage-all: ## Combined coverage report
	$(GOTEST) -coverpkg=./... -coverprofile=coverage-all.out -covermode=atomic ./...
	$(GOTOOL) cover -html=coverage-all.out -o coverage-all.html

# Benchmark targets
bench-api: ## Run API benchmarks
	$(GOTEST) -bench=. -benchmem -run=^$ ./internal/api/...

bench-routing: ## Run routing benchmarks
	$(GOTEST) -bench=BenchmarkRouting -benchmem -run=^$ ./internal/service/callrouting/...

# Mock generation
mocks: ## Generate mocks
	mockery --all --output=./internal/api/rest/mocks --outpkg=mocks
```

## Implementation Roadmap

### Phase 1: Foundation (Week 1)
1. ✅ Implement missing API endpoints
2. ✅ Create mock services for testing
3. ⬜ Update handler unit tests
4. ⬜ Create test data builders

### Phase 2: Unit & Integration Tests (Week 2)
1. ⬜ Complete unit tests for all handlers
2. ⬜ Add validation tests
3. ⬜ Add error handling tests
4. ⬜ Enhance integration tests

### Phase 3: E2E Tests (Week 3)
1. ⬜ Enhance call flow E2E tests
2. ⬜ Add compliance E2E tests
3. ⬜ Add financial E2E tests
4. ⬜ Add real-time event tests

### Phase 4: Specialized Testing (Week 4)
1. ⬜ Implement contract tests
2. ⬜ Implement security tests
3. ⬜ Implement performance tests
4. ⬜ Set up CI/CD integration

### Phase 5: Documentation & Optimization (Week 5)
1. ⬜ Document test patterns
2. ⬜ Optimize test execution
3. ⬜ Create test reports
4. ⬜ Train team on testing

## Best Practices

### 1. Test Organization
- Group tests by feature/domain
- Use consistent naming: `Test{Feature}_{Scenario}`
- Keep tests independent and idempotent
- Use table-driven tests for multiple scenarios

### 2. Test Data
- Use builders for complex objects
- Create reusable test scenarios
- Clean up test data after each test
- Use deterministic data for reproducibility

### 3. Assertions
- Use specific assertions (Equal vs Contains)
- Include helpful error messages
- Test both positive and negative cases
- Verify side effects

### 4. Performance
- Run tests in parallel when possible
- Use test containers efficiently
- Cache test dependencies
- Profile slow tests

### 5. Maintenance
- Keep tests close to implementation
- Update tests with code changes
- Remove flaky tests
- Regular test review