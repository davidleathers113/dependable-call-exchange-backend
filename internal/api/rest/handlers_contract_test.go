//go:build contract

package rest

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/account"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/bid"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/call"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestContractResponseStructure verifies that all API responses match expected schema
func TestContractResponseStructure(t *testing.T) {
	h, mocks := setupHandler(t)

	tests := []struct {
		name           string
		setupMocks     func()
		makeRequest    func() *httptest.ResponseRecorder
		validateSchema func(t *testing.T, body []byte)
	}{
		{
			name: "GET /accounts/:id response schema",
			setupMocks: func() {
				mocks.accountSvc.GetByIDFunc = func(ctx context.Context, id uuid.UUID) (*account.Account, error) {
					money, _ := values.NewMoneyFromFloat(1000.00, "USD")
					email, _ := values.NewEmail("buyer@example.com")
					return &account.Account{
						ID:          id,
						Type:        account.TypeBuyer,
						Email:       email,
						CompanyName: "Test Company",
						Status:      account.StatusActive,
						Balance:     money,
						CreatedAt:   time.Now(),
						UpdatedAt:   time.Now(),
					}, nil
				}
			},
			makeRequest: func() *httptest.ResponseRecorder {
				req := httptest.NewRequest("GET", "/api/v1/accounts/"+testBuyerID.String(), nil)
				req = req.WithContext(setUserContext(req.Context(), testBuyerID, "buyer"))
				return makeAuthenticatedRequest(h, req)
			},
			validateSchema: func(t *testing.T, body []byte) {
				var resp map[string]interface{}
				require.NoError(t, json.Unmarshal(body, &resp))

				// Verify required fields exist
				assert.Contains(t, resp, "id")
				assert.Contains(t, resp, "type")
				assert.Contains(t, resp, "email")
				assert.Contains(t, resp, "company_name")
				assert.Contains(t, resp, "status")
				assert.Contains(t, resp, "balance")
				assert.Contains(t, resp, "created_at")
				assert.Contains(t, resp, "updated_at")

				// Verify field types
				assert.IsType(t, "", resp["id"])
				assert.IsType(t, "", resp["type"])
				assert.IsType(t, "", resp["email"])
				assert.IsType(t, "", resp["company_name"])
				assert.IsType(t, "", resp["status"])
				assert.IsType(t, float64(0), resp["balance"])
				assert.IsType(t, "", resp["created_at"])
				assert.IsType(t, "", resp["updated_at"])

				// Verify enum values
				assert.Contains(t, []string{"buyer", "seller"}, resp["type"])
				assert.Contains(t, []string{"active", "suspended", "closed"}, resp["status"])
			},
		},
		{
			name: "POST /calls response schema",
			setupMocks: func() {
				mocks.callSvc.CreateCallFunc = func(ctx context.Context, req CreateCallRequest) (*call.Call, error) {
					fromNum, _ := values.NewPhoneNumber("+12125551234")
					toNum, _ := values.NewPhoneNumber("+13105559876")
					cost, _ := values.NewMoneyFromFloat(0.05, "USD")
					
					return &call.Call{
						ID:         uuid.New(),
						FromNumber: fromNum,
						ToNumber:   toNum,
						Status:     call.StatusPending,
						Direction:  call.DirectionInbound,
						BuyerID:    &testBuyerID,
						StartTime:  time.Now(),
						Cost:       &cost,
						CallSID:    "CALL123456",
						CreatedAt:  time.Now(),
						UpdatedAt:  time.Now(),
					}, nil
				}
			},
			makeRequest: func() *httptest.ResponseRecorder {
				req := createCallRequest()
				body, _ := json.Marshal(req)
				httpReq := httptest.NewRequest("POST", "/api/v1/calls", bytes.NewReader(body))
				httpReq.Header.Set("Content-Type", "application/json")
				httpReq = httpReq.WithContext(setUserContext(httpReq.Context(), testBuyerID, "buyer"))
				return makeAuthenticatedRequest(h, httpReq)
			},
			validateSchema: func(t *testing.T, body []byte) {
				var resp map[string]interface{}
				require.NoError(t, json.Unmarshal(body, &resp))

				// Verify required fields
				assert.Contains(t, resp, "id")
				assert.Contains(t, resp, "from_number")
				assert.Contains(t, resp, "to_number")
				assert.Contains(t, resp, "status")
				assert.Contains(t, resp, "direction")
				assert.Contains(t, resp, "start_time")
				assert.Contains(t, resp, "created_at")
				assert.Contains(t, resp, "updated_at")

				// Verify optional fields structure when present
				if buyer, ok := resp["buyer_id"]; ok {
					assert.IsType(t, "", buyer)
				}
				if cost, ok := resp["cost"]; ok {
					assert.IsType(t, float64(0), cost)
				}
			},
		},
		{
			name: "GET /bids/:id response schema",
			setupMocks: func() {
				mocks.bidSvc.GetBidFunc = func(ctx context.Context, id uuid.UUID) (*bid.Bid, error) {
					amount, _ := values.NewMoneyFromFloat(0.10, "USD")
					criteria := bid.BidCriteria{
						Geography: bid.GeoCriteria{
							Countries: []string{"US"},
							States:    []string{"CA", "NY"},
						},
						CallType:  []string{"sales"},
						Keywords:  []string{"insurance"},
					}
					
					return &bid.Bid{
						ID:        id,
						CallID:    uuid.New(),
						BuyerID:   testBuyerID,
						SellerID:  uuid.New(),
						Amount:    amount,
						Status:    bid.StatusActive,
						Criteria:  criteria,
						PlacedAt:  time.Now(),
						ExpiresAt: time.Now().Add(5 * time.Minute),
					}, nil
				}
			},
			makeRequest: func() *httptest.ResponseRecorder {
				req := httptest.NewRequest("GET", "/api/v1/bids/"+uuid.New().String(), nil)
				req = req.WithContext(setUserContext(req.Context(), testBuyerID, "buyer"))
				return makeAuthenticatedRequest(h, req)
			},
			validateSchema: func(t *testing.T, body []byte) {
				var resp map[string]interface{}
				require.NoError(t, json.Unmarshal(body, &resp))

				// Verify bid structure
				assert.Contains(t, resp, "id")
				assert.Contains(t, resp, "call_id")
				assert.Contains(t, resp, "buyer_id")
				assert.Contains(t, resp, "seller_id")
				assert.Contains(t, resp, "amount")
				assert.Contains(t, resp, "status")
				assert.Contains(t, resp, "criteria")
				assert.Contains(t, resp, "placed_at")
				assert.Contains(t, resp, "expires_at")

				// Verify criteria structure
				criteria, ok := resp["criteria"].(map[string]interface{})
				require.True(t, ok, "criteria should be an object")
				assert.Contains(t, criteria, "geography")
				assert.Contains(t, criteria, "call_type")
				assert.Contains(t, criteria, "keywords")
			},
		},
		{
			name: "GET /compliance/tcpa/check response schema",
			setupMocks: func() {
				mocks.complianceSvc.CheckTCPAFunc = func(ctx context.Context, phoneNumber string, callTime time.Time) (bool, string, error) {
					return true, "Call allowed during business hours", nil
				}
			},
			makeRequest: func() *httptest.ResponseRecorder {
				req := httptest.NewRequest("GET", "/api/v1/compliance/tcpa/check?phone_number=+12125551234", nil)
				req = req.WithContext(setUserContext(req.Context(), testBuyerID, "buyer"))
				return makeAuthenticatedRequest(h, req)
			},
			validateSchema: func(t *testing.T, body []byte) {
				var resp map[string]interface{}
				require.NoError(t, json.Unmarshal(body, &resp))

				// Verify TCPA check response
				assert.Contains(t, resp, "allowed")
				assert.Contains(t, resp, "reason")
				assert.Contains(t, resp, "checked_at")
				
				assert.IsType(t, true, resp["allowed"])
				assert.IsType(t, "", resp["reason"])
				assert.IsType(t, "", resp["checked_at"])
			},
		},
		{
			name: "Error response schema",
			setupMocks: func() {
				mocks.accountSvc.GetByIDFunc = func(ctx context.Context, id uuid.UUID) (*account.Account, error) {
					return nil, account.ErrAccountNotFound
				}
			},
			makeRequest: func() *httptest.ResponseRecorder {
				req := httptest.NewRequest("GET", "/api/v1/accounts/"+uuid.New().String(), nil)
				req = req.WithContext(setUserContext(req.Context(), testBuyerID, "buyer"))
				return makeAuthenticatedRequest(h, req)
			},
			validateSchema: func(t *testing.T, body []byte) {
				var resp map[string]interface{}
				require.NoError(t, json.Unmarshal(body, &resp))

				// Verify error response structure
				assert.Contains(t, resp, "error")
				errorObj, ok := resp["error"].(map[string]interface{})
				require.True(t, ok, "error should be an object")
				
				assert.Contains(t, errorObj, "code")
				assert.Contains(t, errorObj, "message")
				assert.IsType(t, "", errorObj["code"])
				assert.IsType(t, "", errorObj["message"])
				
				// Optional fields
				if details, ok := errorObj["details"]; ok {
					assert.IsType(t, map[string]interface{}{}, details)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMocks()
			w := tt.makeRequest()
			tt.validateSchema(t, w.Body.Bytes())
		})
	}
}

// TestContractPaginationResponse verifies pagination response structure
func TestContractPaginationResponse(t *testing.T) {
	h, mocks := setupHandler(t)

	tests := []struct {
		name       string
		setupMocks func()
		endpoint   string
	}{
		{
			name: "GET /calls pagination",
			setupMocks: func() {
				mocks.callSvc.ListCallsFunc = func(ctx context.Context, filter CallFilter, pagination Pagination) ([]*call.Call, int, error) {
					calls := make([]*call.Call, 2)
					for i := range calls {
						fromNum, _ := values.NewPhoneNumber("+12125551234")
						toNum, _ := values.NewPhoneNumber("+13105559876")
						calls[i] = &call.Call{
							ID:         uuid.New(),
							FromNumber: fromNum,
							ToNumber:   toNum,
							Status:     call.StatusCompleted,
							Direction:  call.DirectionInbound,
							StartTime:  time.Now(),
							CreatedAt:  time.Now(),
							UpdatedAt:  time.Now(),
						}
					}
					return calls, 100, nil
				}
			},
			endpoint: "/api/v1/calls?page=1&limit=10",
		},
		{
			name: "GET /auctions pagination",
			setupMocks: func() {
				mocks.auctionSvc.ListAuctionsFunc = func(ctx context.Context, filter AuctionFilter, pagination Pagination) ([]*bid.Auction, int, error) {
					auctions := make([]*bid.Auction, 2)
					reservePrice, _ := values.NewMoneyFromFloat(0.05, "USD")
					bidIncrement, _ := values.NewMoneyFromFloat(0.01, "USD")
					
					for i := range auctions {
						auctions[i] = &bid.Auction{
							ID:           uuid.New(),
							CallID:       uuid.New(),
							Status:       bid.AuctionStatusOpen,
							StartTime:    time.Now(),
							EndTime:      time.Now().Add(5 * time.Minute),
							ReservePrice: reservePrice,
							BidIncrement: bidIncrement,
							MaxDuration:  300,
						}
					}
					return auctions, 50, nil
				}
			},
			endpoint: "/api/v1/auctions?page=2&limit=20",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMocks()
			
			req := httptest.NewRequest("GET", tt.endpoint, nil)
			req = req.WithContext(setUserContext(req.Context(), testBuyerID, "buyer"))
			w := makeAuthenticatedRequest(h, req)
			
			assert.Equal(t, http.StatusOK, w.Code)
			
			var resp map[string]interface{}
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
			
			// Verify pagination structure
			assert.Contains(t, resp, "data")
			assert.Contains(t, resp, "pagination")
			
			pagination, ok := resp["pagination"].(map[string]interface{})
			require.True(t, ok, "pagination should be an object")
			
			assert.Contains(t, pagination, "page")
			assert.Contains(t, pagination, "limit")
			assert.Contains(t, pagination, "total")
			assert.Contains(t, pagination, "total_pages")
			
			assert.IsType(t, float64(0), pagination["page"])
			assert.IsType(t, float64(0), pagination["limit"])
			assert.IsType(t, float64(0), pagination["total"])
			assert.IsType(t, float64(0), pagination["total_pages"])
		})
	}
}

// TestContractHeaderValidation verifies required headers in responses
func TestContractHeaderValidation(t *testing.T) {
	h, mocks := setupHandler(t)

	endpoints := []struct {
		name       string
		method     string
		path       string
		setupMocks func()
	}{
		{
			name:   "GET endpoint headers",
			method: "GET",
			path:   "/api/v1/accounts/" + testBuyerID.String(),
			setupMocks: func() {
				mocks.accountSvc.GetByIDFunc = func(ctx context.Context, id uuid.UUID) (*account.Account, error) {
					money, _ := values.NewMoneyFromFloat(1000.00, "USD")
					email, _ := values.NewEmail("buyer@example.com")
					return &account.Account{
						ID:          id,
						Type:        account.TypeBuyer,
						Email:       email,
						CompanyName: "Test Company",
						Status:      account.StatusActive,
						Balance:     money,
						CreatedAt:   time.Now(),
						UpdatedAt:   time.Now(),
					}, nil
				}
			},
		},
		{
			name:   "POST endpoint headers",
			method: "POST",
			path:   "/api/v1/calls",
			setupMocks: func() {
				mocks.callSvc.CreateCallFunc = func(ctx context.Context, req CreateCallRequest) (*call.Call, error) {
					fromNum, _ := values.NewPhoneNumber("+12125551234")
					toNum, _ := values.NewPhoneNumber("+13105559876")
					return &call.Call{
						ID:         uuid.New(),
						FromNumber: fromNum,
						ToNumber:   toNum,
						Status:     call.StatusPending,
						Direction:  call.DirectionInbound,
						StartTime:  time.Now(),
						CreatedAt:  time.Now(),
						UpdatedAt:  time.Now(),
					}, nil
				}
			},
		},
	}

	for _, tt := range endpoints {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMocks()
			
			var req *http.Request
			if tt.method == "POST" {
				reqBody := createCallRequest()
				body, _ := json.Marshal(reqBody)
				req = httptest.NewRequest(tt.method, tt.path, bytes.NewReader(body))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req = httptest.NewRequest(tt.method, tt.path, nil)
			}
			
			req = req.WithContext(setUserContext(req.Context(), testBuyerID, "buyer"))
			w := makeAuthenticatedRequest(h, req)
			
			// Verify standard headers
			assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
			assert.NotEmpty(t, w.Header().Get("X-Request-ID"))
			
			// Verify security headers
			assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
			assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))
			assert.Equal(t, "1; mode=block", w.Header().Get("X-XSS-Protection"))
			
			// Verify cache headers for GET requests
			if tt.method == "GET" {
				assert.NotEmpty(t, w.Header().Get("Cache-Control"))
			}
		})
	}
}

// TestContractVersioning verifies API versioning consistency
func TestContractVersioning(t *testing.T) {
	h, mocks := setupHandler(t)

	// Setup mock for version testing
	mocks.accountSvc.GetByIDFunc = func(ctx context.Context, id uuid.UUID) (*account.Account, error) {
		money, _ := values.NewMoneyFromFloat(1000.00, "USD")
		email, _ := values.NewEmail("buyer@example.com")
		return &account.Account{
			ID:          id,
			Type:        account.TypeBuyer,
			Email:       email,
			CompanyName: "Test Company",
			Status:      account.StatusActive,
			Balance:     money,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}, nil
	}

	tests := []struct {
		name            string
		path            string
		expectedVersion string
	}{
		{
			name:            "v1 endpoint",
			path:            "/api/v1/accounts/" + testBuyerID.String(),
			expectedVersion: "1.0",
		},
		// Add v2 endpoint tests when implemented
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			req = req.WithContext(setUserContext(req.Context(), testBuyerID, "buyer"))
			w := makeAuthenticatedRequest(h, req)
			
			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, tt.expectedVersion, w.Header().Get("API-Version"))
		})
	}
}

// TestContractContentNegotiation verifies content type handling
func TestContractContentNegotiation(t *testing.T) {
	h, mocks := setupHandler(t)

	mocks.accountSvc.GetByIDFunc = func(ctx context.Context, id uuid.UUID) (*account.Account, error) {
		money, _ := values.NewMoneyFromFloat(1000.00, "USD")
		email, _ := values.NewEmail("buyer@example.com")
		return &account.Account{
			ID:          id,
			Type:        account.TypeBuyer,
			Email:       email,
			CompanyName: "Test Company",
			Status:      account.StatusActive,
			Balance:     money,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}, nil
	}

	tests := []struct {
		name           string
		acceptHeader   string
		expectedStatus int
		expectedType   string
	}{
		{
			name:           "accept JSON",
			acceptHeader:   "application/json",
			expectedStatus: http.StatusOK,
			expectedType:   "application/json",
		},
		{
			name:           "accept any",
			acceptHeader:   "*/*",
			expectedStatus: http.StatusOK,
			expectedType:   "application/json",
		},
		{
			name:           "accept unsupported type",
			acceptHeader:   "application/xml",
			expectedStatus: http.StatusNotAcceptable,
			expectedType:   "application/json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/v1/accounts/"+testBuyerID.String(), nil)
			req.Header.Set("Accept", tt.acceptHeader)
			req = req.WithContext(setUserContext(req.Context(), testBuyerID, "buyer"))
			
			w := makeAuthenticatedRequest(h, req)
			
			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Equal(t, tt.expectedType, w.Header().Get("Content-Type"))
		})
	}
}

// TestContractFieldConsistency verifies field naming and type consistency
func TestContractFieldConsistency(t *testing.T) {
	h, mocks := setupHandler(t)

	// Test timestamp format consistency
	t.Run("timestamp format consistency", func(t *testing.T) {
		mocks.callSvc.CreateCallFunc = func(ctx context.Context, req CreateCallRequest) (*call.Call, error) {
			fromNum, _ := values.NewPhoneNumber("+12125551234")
			toNum, _ := values.NewPhoneNumber("+13105559876")
			now := time.Now()
			
			return &call.Call{
				ID:         uuid.New(),
				FromNumber: fromNum,
				ToNumber:   toNum,
				Status:     call.StatusPending,
				Direction:  call.DirectionInbound,
				StartTime:  now,
				CreatedAt:  now,
				UpdatedAt:  now,
			}, nil
		}

		req := createCallRequest()
		body, _ := json.Marshal(req)
		httpReq := httptest.NewRequest("POST", "/api/v1/calls", bytes.NewReader(body))
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq = httpReq.WithContext(setUserContext(httpReq.Context(), testBuyerID, "buyer"))
		
		w := makeAuthenticatedRequest(h, httpReq)
		require.Equal(t, http.StatusCreated, w.Code)

		var resp map[string]interface{}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

		// Verify all timestamps are in RFC3339 format
		timestamps := []string{"start_time", "created_at", "updated_at"}
		for _, field := range timestamps {
			if value, ok := resp[field].(string); ok {
				_, err := time.Parse(time.RFC3339, value)
				assert.NoError(t, err, "%s should be in RFC3339 format", field)
			}
		}
	})

	// Test money field consistency
	t.Run("money field consistency", func(t *testing.T) {
		mocks.accountSvc.GetBalanceFunc = func(ctx context.Context, accountID uuid.UUID) (*values.Money, error) {
			return values.NewMoneyFromFloat(1234.56, "USD")
		}

		req := httptest.NewRequest("GET", "/api/v1/accounts/"+testBuyerID.String()+"/balance", nil)
		req = req.WithContext(setUserContext(req.Context(), testBuyerID, "buyer"))
		
		w := makeAuthenticatedRequest(h, req)
		require.Equal(t, http.StatusOK, w.Code)

		var resp map[string]interface{}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

		// Verify money is represented as a number
		assert.IsType(t, float64(0), resp["balance"])
		assert.Equal(t, "USD", resp["currency"])
	})
}

// TestContractBackwardCompatibility verifies backward compatibility
func TestContractBackwardCompatibility(t *testing.T) {
	h, mocks := setupHandler(t)

	t.Run("deprecated fields still present", func(t *testing.T) {
		mocks.callSvc.GetCallFunc = func(ctx context.Context, id uuid.UUID) (*call.Call, error) {
			fromNum, _ := values.NewPhoneNumber("+12125551234")
			toNum, _ := values.NewPhoneNumber("+13105559876")
			cost, _ := values.NewMoneyFromFloat(0.05, "USD")
			
			return &call.Call{
				ID:         id,
				FromNumber: fromNum,
				ToNumber:   toNum,
				Status:     call.StatusCompleted,
				Direction:  call.DirectionInbound,
				BuyerID:    &testBuyerID,
				StartTime:  time.Now().Add(-5 * time.Minute),
				EndTime:    &[]time.Time{time.Now()}[0],
				Duration:   &[]int{300}[0],
				Cost:       &cost,
				CallSID:    "CALL123456",
				CreatedAt:  time.Now(),
				UpdatedAt:  time.Now(),
			}, nil
		}

		req := httptest.NewRequest("GET", "/api/v1/calls/"+uuid.New().String(), nil)
		req = req.WithContext(setUserContext(req.Context(), testBuyerID, "buyer"))
		
		w := makeAuthenticatedRequest(h, req)
		require.Equal(t, http.StatusOK, w.Code)

		var resp map[string]interface{}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

		// Verify both new and legacy fields are present
		assert.Contains(t, resp, "duration") // Current field
		// Add checks for any deprecated fields that need to remain for compatibility
	})
}

// Helper function for authenticated requests in contract tests
func makeAuthenticatedRequest(h *Handler, req *http.Request) *httptest.ResponseRecorder {
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

// Helper function to create a valid call request
func createCallRequest() CreateCallRequest {
	return CreateCallRequest{
		FromNumber: "+12125551234",
		ToNumber:   "+13105559876",
		Direction:  "inbound",
	}
}
