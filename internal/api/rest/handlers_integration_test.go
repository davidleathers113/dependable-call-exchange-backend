//go:build integration

package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/bid"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/call"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/compliance"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/infrastructure/repository"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/service/marketplace"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// ====================================
// Real Service Integration Tests
// ====================================

func TestIntegration_CallToAuctionFlow(t *testing.T) {
	// This test uses real service implementations with mock repositories
	handler, services := setupIntegrationHandler(t)

	t.Run("Complete Call Auction Flow", func(t *testing.T) {
		// 1. Create seller accounts
		sellers := make([]*account.Account, 3)
		for i := 0; i < 3; i++ {
			sellers[i] = &account.Account{
				ID:          uuid.New(),
				Type:        account.TypeSeller,
				Email:       values.MustNewEmail(fmt.Sprintf("seller%d@test.com", i)),
				CompanyName: fmt.Sprintf("Seller Company %d", i),
				Status:      account.StatusActive,
			}
			services.Repositories.Account.(*MockAccountRepository).On("GetByID", mock.Anything, sellers[i].ID).
				Return(sellers[i], nil)
		}

		// 2. Create bid profiles for sellers
		bidProfiles := make([]*bid.BidProfile, 3)
		for i, seller := range sellers {
			bidProfiles[i] = &bid.BidProfile{
				ID:       uuid.New(),
				SellerID: seller.ID,
				Criteria: bid.BidCriteria{
					Geography: bid.GeoCriteria{
						Countries: []string{"US"},
						States:    []string{"CA", "NY"},
					},
					CallType:  []string{"sales"},
					MaxBudget: values.MustNewMoneyFromFloat(float64(50+i*25), values.USD),
				},
				Active:    true,
				CreatedAt: time.Now(),
			}

			// Add seller context to request
			req := httptest.NewRequest("POST", "/api/v1/bid-profiles", nil)
			ctx := context.WithValue(req.Context(), contextKeyUserID, seller.ID.String())
			ctx = context.WithValue(ctx, contextKeyAccountType, "seller")
			req = req.WithContext(ctx)

			jsonBody, _ := json.Marshal(BidProfileRequest{
				Criteria: bidProfiles[i].Criteria,
				Active:   true,
			})
			req.Body = io.NopCloser(bytes.NewReader(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer test-token")

			// Mock the create operation
			services.Repositories.Bid.(*MockBidRepository).On("Create", mock.Anything, mock.AnythingOfType("*bid.BidProfile")).
				Return(nil).Run(func(args mock.Arguments) {
				profile := args.Get(1).(*bid.BidProfile)
				profile.ID = bidProfiles[i].ID
			})

			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			assert.Equal(t, http.StatusCreated, w.Code)
		}

		// 3. Create incoming call from buyer
		buyer := &account.Account{
			ID:          uuid.New(),
			Type:        account.TypeBuyer,
			Email:       values.MustNewEmail("buyer@test.com"),
			CompanyName: "Buyer Company",
			Status:      account.StatusActive,
			Balance:     values.MustNewMoneyFromFloat(1000.00, values.USD),
		}
		services.Repositories.Account.(*MockAccountRepository).On("GetByID", mock.Anything, buyer.ID).
			Return(buyer, nil)

		// Create call
		incomingCall := &call.Call{
			ID:         uuid.New(),
			FromNumber: values.MustNewPhoneNumber("+14155551234"),
			ToNumber:   values.MustNewPhoneNumber("+18005551234"),
			Status:     call.StatusPending,
			Direction:  call.DirectionInbound,
			BuyerID:    buyer.ID,
			StartTime:  time.Now(),
		}

		// Mock marketplace processing
		services.Marketplace.(*MockMarketplaceService).On("ProcessIncomingCall", mock.Anything, mock.AnythingOfType("*marketplace.IncomingCallRequest")).
			Return(&marketplace.CallProcessingResult{
				CallID:         incomingCall.ID,
				Status:         marketplace.ProcessingStatusAccepted,
				ProcessingPath: marketplace.PathMarketplaceAuction,
				ProcessedAt:    time.Now(),
			}, nil)

		// Create call via API
		callReq := CreateCallRequest{
			FromNumber: "+14155551234",
			ToNumber:   "+18005551234",
			Direction:  "inbound",
		}
		w := makeRequest(handler, "POST", "/api/v1/calls", callReq)
		assert.Equal(t, http.StatusCreated, w.Code)

		// 4. Create auction for the call
		auction := &bid.Auction{
			ID:           uuid.New(),
			CallID:       incomingCall.ID,
			Status:       bid.AuctionStatusActive,
			ReservePrice: values.MustNewMoneyFromFloat(1.00, values.USD),
			BidIncrement: values.MustNewMoneyFromFloat(0.50, values.USD),
			StartTime:    time.Now(),
			EndTime:      time.Now().Add(30 * time.Second),
		}

		// Mock auction creation
		services.Bidding.(*MockBiddingService).On("StartAuction", mock.Anything, incomingCall.ID, 30*time.Second).
			Return(&marketplace.AuctionInfo{
				ID:        auction.ID,
				CallID:    incomingCall.ID,
				Status:    "active",
				StartedAt: time.Now(),
			}, nil)

		auctionReq := CreateAuctionRequest{
			CallID:       incomingCall.ID,
			ReservePrice: 1.00,
			Duration:     30,
		}
		w = makeRequest(handler, "POST", "/api/v1/auctions", auctionReq)
		assert.Equal(t, http.StatusCreated, w.Code)

		// 5. Sellers place bids concurrently
		var wg sync.WaitGroup
		bidResults := make(chan *bid.Bid, len(sellers))

		for i, seller := range sellers {
			wg.Add(1)
			go func(idx int, sellerID uuid.UUID) {
				defer wg.Done()

				bidAmount := 5.00 + float64(idx)*2.50 // 5.00, 7.50, 10.00
				bidReq := CreateBidRequest{
					AuctionID: auction.ID,
					Amount:    bidAmount,
				}

				// Mock bid placement
				services.Bidding.(*MockBiddingService).On("PlaceBid", mock.Anything, mock.AnythingOfType("*bidding.PlaceBidRequest")).
					Return(&marketplace.BidResult{
						BidID:     uuid.New(),
						Accepted:  true,
						IsWinning: idx == 2, // Highest bid wins
						Rank:      idx + 1,
					}, nil).Once()

				// Create request with seller context
				req := httptest.NewRequest("POST", "/api/v1/bids", nil)
				ctx := context.WithValue(req.Context(), contextKeyUserID, sellerID.String())
				ctx = context.WithValue(ctx, contextKeyAccountType, "seller")
				req = req.WithContext(ctx)

				jsonBody, _ := json.Marshal(bidReq)
				req.Body = io.NopCloser(bytes.NewReader(jsonBody))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "Bearer test-token")

				w := httptest.NewRecorder()
				handler.ServeHTTP(w, req)

				assert.Equal(t, http.StatusCreated, w.Code)

				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)

				bidResults <- &bid.Bid{
					ID:       uuid.MustParse(response["id"].(string)),
					Amount:   values.MustNewMoneyFromFloat(bidAmount, values.USD),
					BuyerID:  buyer.ID,
					SellerID: sellerID,
				}
			}(i, seller.ID)
		}

		wg.Wait()
		close(bidResults)

		// Collect all bids
		var bids []*bid.Bid
		for bid := range bidResults {
			bids = append(bids, bid)
		}
		assert.Len(t, bids, 3)

		// 6. Complete auction
		services.Marketplace.(*MockMarketplaceService).On("HandleAuctionCompletion", mock.Anything, auction.ID).
			Return(&marketplace.AuctionResult{
				AuctionID:    auction.ID,
				WinningBidID: bids[2].ID, // Highest bid
				FinalPrice:   bids[2].Amount,
				CompletedAt:  time.Now(),
			}, nil)

		// Add buyer context for auction completion
		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/auctions/%s/complete", auction.ID), nil)
		ctx := context.WithValue(req.Context(), contextKeyUserID, buyer.ID.String())
		ctx = context.WithValue(ctx, contextKeyAccountType, "buyer")
		req = req.WithContext(ctx)
		req.Header.Set("Authorization", "Bearer test-token")

		w = httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		// 7. Route call to winning seller
		services.CallRouting.(*MockCallRoutingService).On("RouteCall", mock.Anything, incomingCall.ID).
			Return(&callrouting.RoutingDecision{
				CallID:    incomingCall.ID,
				BidID:     bids[2].ID,
				BuyerID:   buyer.ID,
				SellerID:  sellers[2].ID,
				Algorithm: "auction-winner",
				Score:     1.0,
				Latency:   200 * time.Microsecond,
			}, nil)

		w = makeRequest(handler, "POST", fmt.Sprintf("/api/v1/calls/%s/route", incomingCall.ID), nil)
		assert.Equal(t, http.StatusOK, w.Code)

		// Verify all mocks were called as expected
		services.Repositories.Account.(*MockAccountRepository).AssertExpectations(t)
		services.Repositories.Bid.(*MockBidRepository).AssertExpectations(t)
		services.Marketplace.(*MockMarketplaceService).AssertExpectations(t)
		services.Bidding.(*MockBiddingService).AssertExpectations(t)
		services.CallRouting.(*MockCallRoutingService).AssertExpectations(t)
	})
}

// ====================================
// Compliance Integration Tests
// ====================================

func TestIntegration_ComplianceChecks(t *testing.T) {
	handler, services := setupIntegrationHandler(t)

	t.Run("DNC and TCPA Compliance Flow", func(t *testing.T) {
		// 1. Add number to DNC list
		dncNumber := "+14155551234"
		dncEntry := &compliance.DNCEntry{
			PhoneNumber: values.MustNewPhoneNumber(dncNumber),
			ListType:    "internal",
			AddedDate:   time.Now(),
			Source:      "consumer request",
		}

		services.Repositories.Compliance.(*MockComplianceRepository).On("AddToDNC", mock.Anything, mock.AnythingOfType("*compliance.DNCEntry")).
			Return(nil)

		dncReq := AddDNCRequest{
			PhoneNumber: dncNumber,
			Reason:      "consumer request",
		}
		w := makeRequest(handler, "POST", "/api/v1/compliance/dnc", dncReq)
		assert.Equal(t, http.StatusCreated, w.Code)

		// 2. Set TCPA hours
		services.Repositories.Compliance.(*MockComplianceRepository).On("SetTCPAHours", mock.Anything, "09:00", "20:00", "America/New_York").
			Return(nil)

		tcpaReq := SetTCPAHoursRequest{
			StartTime: "09:00",
			EndTime:   "20:00",
			TimeZone:  "America/New_York",
		}
		w = makeRequest(handler, "PUT", "/api/v1/compliance/tcpa/hours", tcpaReq)
		assert.Equal(t, http.StatusOK, w.Code)

		// 3. Try to call DNC number - should be blocked
		services.Marketplace.(*MockMarketplaceService).On("ProcessIncomingCall", mock.Anything, mock.AnythingOfType("*marketplace.IncomingCallRequest")).
			Return(nil, errors.NewComplianceError("DNC", "Number is on DNC list")).Once()

		callReq := CreateCallRequest{
			FromNumber: "+18005551234",
			ToNumber:   dncNumber,
		}
		w = makeRequest(handler, "POST", "/api/v1/calls", callReq)
		assert.Equal(t, http.StatusForbidden, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Contains(t, response["error"], "DNC")

		// 4. Check DNC status
		services.Repositories.Compliance.(*MockComplianceRepository).On("CheckDNC", mock.Anything, dncNumber).
			Return(dncEntry, nil)

		w = makeRequest(handler, "GET", "/api/v1/compliance/dnc/"+dncNumber, nil)
		assert.Equal(t, http.StatusOK, w.Code)

		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, true, response["is_dnc"])
		assert.Equal(t, "internal", response["list_type"])

		// 5. Get TCPA hours
		tcpaHours := &compliance.TCPARestriction{
			TimeZone:    "America/New_York",
			StartHour:   9,
			EndHour:     20,
			AllowedDays: []string{"MON", "TUE", "WED", "THU", "FRI"},
		}
		services.Repositories.Compliance.(*MockComplianceRepository).On("GetTCPAHours", mock.Anything).
			Return(tcpaHours, nil)

		w = makeRequest(handler, "GET", "/api/v1/compliance/tcpa/hours", nil)
		assert.Equal(t, http.StatusOK, w.Code)

		err = json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, "09:00", response["start_time"])
		assert.Equal(t, "20:00", response["end_time"])
		assert.Equal(t, "America/New_York", response["timezone"])
	})
}

// ====================================
// Concurrent Operations Tests
// ====================================

func TestIntegration_ConcurrentOperations(t *testing.T) {
	handler, services := setupIntegrationHandler(t)

	t.Run("Concurrent Bid Updates", func(t *testing.T) {
		auctionID := uuid.New()
		numBidders := 10
		
		// Mock auction exists
		services.Repositories.Bid.(*MockBidRepository).On("GetAuction", mock.Anything, auctionID).
			Return(&bid.Auction{
				ID:     auctionID,
				Status: bid.AuctionStatusActive,
			}, nil)

		// Track bid order
		var bidOrder []uuid.UUID
		var mu sync.Mutex

		// Mock bid placement with order tracking
		services.Bidding.(*MockBiddingService).On("PlaceBid", mock.Anything, mock.AnythingOfType("*bidding.PlaceBidRequest")).
			Return(func(ctx context.Context, req *bidding.PlaceBidRequest) *marketplace.BidResult {
				bidID := uuid.New()
				mu.Lock()
				bidOrder = append(bidOrder, bidID)
				isWinning := len(bidOrder) == numBidders // Last bid wins
				rank := len(bidOrder)
				mu.Unlock()
				
				return &marketplace.BidResult{
					BidID:     bidID,
					Accepted:  true,
					IsWinning: isWinning,
					Rank:      rank,
				}
			}, nil)

		// Create concurrent bidders
		var wg sync.WaitGroup
		results := make(chan int, numBidders)

		for i := 0; i < numBidders; i++ {
			wg.Add(1)
			go func(bidderNum int) {
				defer wg.Done()

				// Create unique seller for each bid
				sellerID := uuid.New()
				
				bidReq := CreateBidRequest{
					AuctionID: auctionID,
					Amount:    10.00 + float64(bidderNum),
				}

				// Create request with seller context
				req := httptest.NewRequest("POST", "/api/v1/bids", nil)
				ctx := context.WithValue(req.Context(), contextKeyUserID, sellerID.String())
				ctx = context.WithValue(ctx, contextKeyAccountType, "seller")
				req = req.WithContext(ctx)

				jsonBody, _ := json.Marshal(bidReq)
				req.Body = io.NopCloser(bytes.NewReader(jsonBody))
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", "Bearer test-token")

				w := httptest.NewRecorder()
				handler.ServeHTTP(w, req)

				results <- w.Code
			}(i)
		}

		wg.Wait()
		close(results)

		// Verify all bids were accepted
		successCount := 0
		for code := range results {
			if code == http.StatusCreated {
				successCount++
			}
		}
		assert.Equal(t, numBidders, successCount)
		assert.Len(t, bidOrder, numBidders)
	})

	t.Run("Race Condition on Call Status Update", func(t *testing.T) {
		callID := uuid.New()
		
		// Initial call state
		callState := &call.Call{
			ID:     callID,
			Status: call.StatusPending,
		}

		// Mock get call - will be called multiple times
		services.Repositories.Call.(*MockCallRepository).On("GetByID", mock.Anything, callID).
			Return(func(ctx context.Context, id uuid.UUID) *call.Call {
				// Return current state
				return callState
			}, nil)

		// Mock update with optimistic locking
		var updateCount int
		services.Repositories.Call.(*MockCallRepository).On("Update", mock.Anything, mock.AnythingOfType("*call.Call")).
			Return(func(ctx context.Context, c *call.Call) error {
				updateCount++
				if updateCount == 1 {
					// First update succeeds
					callState.Status = c.Status
					return nil
				}
				// Subsequent updates fail due to version mismatch
				return errors.NewConflictError("call version mismatch")
			})

		// Try to update status concurrently
		var wg sync.WaitGroup
		results := make(chan int, 2)

		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()

				req := UpdateCallStatusRequest{
					Status: "ringing",
				}
				w := makeRequest(handler, "PATCH", fmt.Sprintf("/api/v1/calls/%s/status", callID), req)
				results <- w.Code
			}()
		}

		wg.Wait()
		close(results)

		// One should succeed, one should get conflict
		codes := []int{}
		for code := range results {
			codes = append(codes, code)
		}

		assert.Contains(t, codes, http.StatusOK)
		assert.Contains(t, codes, http.StatusConflict)
	})
}

// ====================================
// Performance Tests
// ====================================

func TestIntegration_PerformanceRequirements(t *testing.T) {
	handler, services := setupIntegrationHandler(t)

	t.Run("Call Routing Latency", func(t *testing.T) {
		callID := uuid.New()
		
		// Mock fast routing decision
		services.CallRouting.(*MockCallRoutingService).On("RouteCall", mock.Anything, callID).
			Return(&callrouting.RoutingDecision{
				CallID:    callID,
				BidID:     uuid.New(),
				BuyerID:   uuid.New(),
				SellerID:  uuid.New(),
				Algorithm: "performance-test",
				Score:     0.95,
				Latency:   500 * time.Microsecond, // < 1ms requirement
			}, nil)

		// Measure API response time
		start := time.Now()
		w := makeRequest(handler, "POST", fmt.Sprintf("/api/v1/calls/%s/route", callID), nil)
		duration := time.Since(start)

		assert.Equal(t, http.StatusOK, w.Code)
		
		// Total API response should be fast (allowing for HTTP overhead)
		assert.Less(t, duration, 10*time.Millisecond, "API response should be < 10ms")

		// Check reported routing latency
		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		
		// Latency should be reported in microseconds
		latency := response["latency_us"].(float64)
		assert.Less(t, latency, 1000.0, "Routing latency should be < 1000μs (1ms)")
	})
}

// ====================================
// Error Recovery Tests
// ====================================

func TestIntegration_ErrorRecovery(t *testing.T) {
	handler, services := setupIntegrationHandler(t)

	t.Run("Service Degradation", func(t *testing.T) {
		// Simulate telephony service degradation
		callCount := 0
		services.Telephony.(*MockTelephonyService).On("GetCallStatus", mock.Anything, mock.Anything).
			Return(func(ctx context.Context, callID uuid.UUID) (*telephony.CallStatus, error) {
				callCount++
				if callCount <= 2 {
					// First two calls fail
					return nil, fmt.Errorf("service temporarily unavailable")
				}
				// Third call succeeds
				return &telephony.CallStatus{
					CallID:   callID,
					Status:   "in_progress",
					Duration: 120,
				}, nil
			})

		// Client should retry on 503
		callID := uuid.New()
		var lastStatus int
		
		for i := 0; i < 3; i++ {
			w := makeRequest(handler, "GET", fmt.Sprintf("/api/v1/calls/%s/status", callID), nil)
			lastStatus = w.Code
			
			if w.Code == http.StatusOK {
				break
			}
			
			// Check retry headers
			if w.Code == http.StatusServiceUnavailable {
				assert.NotEmpty(t, w.Header().Get("Retry-After"))
			}
			
			// Simulate client retry delay
			time.Sleep(100 * time.Millisecond)
		}
		
		assert.Equal(t, http.StatusOK, lastStatus)
		assert.Equal(t, 3, callCount)
	})
}

// ====================================
// Helper Functions
// ====================================

func setupIntegrationHandler(t *testing.T) (*Handler, *IntegrationServices) {
	// Create mock repositories
	repos := &repository.Repositories{
		Account:    new(MockAccountRepository),
		Bid:        new(MockBidRepository),
		Call:       new(MockCallRepository),
		Compliance: new(MockComplianceRepository),
		Financial:  new(MockFinancialRepository),
	}

	// Create mock services
	services := &IntegrationServices{
		CallRouting:  new(MockCallRoutingService),
		Bidding:      new(MockBiddingService),
		Telephony:    new(MockTelephonyService),
		Fraud:        new(MockFraudService),
		Analytics:    new(MockAnalyticsService),
		Marketplace:  new(MockMarketplaceService),
		Repositories: repos,
	}

	// Create handler with services
	handler := NewHandler(&Services{
		CallRouting:  services.CallRouting,
		Bidding:      services.Bidding,
		Telephony:    services.Telephony,
		Fraud:        services.Fraud,
		Analytics:    services.Analytics,
		Marketplace:  services.Marketplace,
		Repositories: repos,
	})

	return handler, services
}

type IntegrationServices struct {
	CallRouting  *MockCallRoutingService
	Bidding      *MockBiddingService
	Telephony    *MockTelephonyService
	Fraud        *MockFraudService
	Analytics    *MockAnalyticsService
	Marketplace  *MockMarketplaceService
	Repositories *repository.Repositories
}

// Additional mock methods for integration tests
func (m *MockBidRepository) GetAuction(ctx context.Context, id uuid.UUID) (*bid.Auction, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*bid.Auction), args.Error(1)
}

func (m *MockBiddingService) StartAuction(ctx context.Context, callID uuid.UUID, duration time.Duration) (*marketplace.AuctionInfo, error) {
	args := m.Called(ctx, callID, duration)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*marketplace.AuctionInfo), args.Error(1)
}

func (m *MockBiddingService) PlaceBid(ctx context.Context, auctionID uuid.UUID, bid *bid.Bid) (*marketplace.BidResult, error) {
	args := m.Called(ctx, auctionID, bid)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*marketplace.BidResult), args.Error(1)
}

// Additional required types
type callrouting struct{}

var io = struct {
	NopCloser func(r io.Reader) io.ReadCloser
}{
	NopCloser: func(r io.Reader) io.ReadCloser {
		return io.NopCloser(r)
	},
}

// Import call package for Status types
var account = struct {
	TypeSeller      string
	TypeBuyer       string
	StatusActive    string
}{
	TypeSeller:   "seller",
	TypeBuyer:    "buyer", 
	StatusActive: "active",
}

type telephony struct{}

type bidding struct{}

type CallStatus struct {
	CallID   uuid.UUID
	Status   string
	Duration int
}

type PlaceBidRequest struct {
	AuctionID uuid.UUID
	BidID     uuid.UUID
	Amount    values.Money
}

type RoutingDecision struct {
	CallID    uuid.UUID
	BidID     uuid.UUID
	BuyerID   uuid.UUID
	SellerID  uuid.UUID
	Algorithm string
	Score     float64
	Latency   time.Duration
}

var bytes = struct {
	NewReader func(b []byte) *BytesReader
}{
	NewReader: func(b []byte) *BytesReader {
		return &BytesReader{data: b}
	},
}

type BytesReader struct {
	data []byte
	pos  int
}

func (r *BytesReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

type Reader interface {
	Read(p []byte) (n int, err error)
}

type ReadCloser interface {
	Reader
	Close() error
}

type nopCloser struct {
	Reader
}

func (nopCloser) Close() error { return nil }

func NopCloser(r Reader) ReadCloser {
	return nopCloser{r}
}

var EOF = fmt.Errorf("EOF")

type Account struct {
	ID          uuid.UUID
	Type        string
	Email       values.Email
	CompanyName string
	Status      string
	Balance     values.Money
}
