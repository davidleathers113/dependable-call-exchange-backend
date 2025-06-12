package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/call"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/compliance"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// ====================================
// Authentication & Account Tests
// ====================================

func TestHandler_AuthEndpoints(t *testing.T) {
	handler, _ := setupHandler(t)

	t.Run("Register", func(t *testing.T) {
		tests := []struct {
			name           string
			request        map[string]interface{}
			expectedStatus int
			expectedError  string
		}{
			{
				name: "successful buyer registration",
				request: map[string]interface{}{
					"email":        "buyer@example.com",
					"password":     "SecurePass123!",
					"company_name": "Test Buyer Inc",
					"type":         "buyer",
				},
				expectedStatus: http.StatusCreated,
			},
			{
				name: "successful seller registration",
				request: map[string]interface{}{
					"email":        "seller@example.com",
					"password":     "SecurePass123!",
					"company_name": "Test Seller Inc",
					"type":         "seller",
				},
				expectedStatus: http.StatusCreated,
			},
			{
				name: "invalid email format",
				request: map[string]interface{}{
					"email":        "invalid-email",
					"password":     "SecurePass123!",
					"company_name": "Test Inc",
					"type":         "buyer",
				},
				expectedStatus: http.StatusBadRequest,
				expectedError:  "Invalid email format",
			},
			{
				name: "weak password",
				request: map[string]interface{}{
					"email":        "test@example.com",
					"password":     "weak",
					"company_name": "Test Inc",
					"type":         "buyer",
				},
				expectedStatus: http.StatusBadRequest,
				expectedError:  "Password too weak",
			},
			{
				name: "missing required fields",
				request: map[string]interface{}{
					"email": "test@example.com",
				},
				expectedStatus: http.StatusBadRequest,
				expectedError:  "Missing required fields",
			},
			{
				name: "invalid account type",
				request: map[string]interface{}{
					"email":        "test@example.com",
					"password":     "SecurePass123!",
					"company_name": "Test Inc",
					"type":         "invalid",
				},
				expectedStatus: http.StatusBadRequest,
				expectedError:  "Invalid account type",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				w := makeRequest(handler, "POST", "/api/v1/auth/register", tt.request)
				
				assert.Equal(t, tt.expectedStatus, w.Code)
				
				if tt.expectedError != "" {
					var response map[string]interface{}
					err := json.Unmarshal(w.Body.Bytes(), &response)
					require.NoError(t, err)
					assert.Contains(t, fmt.Sprintf("%v", response), tt.expectedError)
				}
			})
		}
	})

	t.Run("Login", func(t *testing.T) {
		tests := []struct {
			name           string
			request        map[string]interface{}
			expectedStatus int
			expectedError  string
		}{
			{
				name: "successful login",
				request: map[string]interface{}{
					"email":    "buyer@example.com",
					"password": "SecurePass123!",
				},
				expectedStatus: http.StatusOK,
			},
			{
				name: "invalid credentials",
				request: map[string]interface{}{
					"email":    "buyer@example.com",
					"password": "WrongPassword",
				},
				expectedStatus: http.StatusUnauthorized,
				expectedError:  "Invalid credentials",
			},
			{
				name: "non-existent user",
				request: map[string]interface{}{
					"email":    "nonexistent@example.com",
					"password": "SecurePass123!",
				},
				expectedStatus: http.StatusUnauthorized,
				expectedError:  "Invalid credentials",
			},
			{
				name: "missing email",
				request: map[string]interface{}{
					"password": "SecurePass123!",
				},
				expectedStatus: http.StatusBadRequest,
				expectedError:  "Email is required",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				w := makeRequest(handler, "POST", "/api/v1/auth/login", tt.request)
				
				assert.Equal(t, tt.expectedStatus, w.Code)
				
				if tt.expectedError != "" {
					var response map[string]interface{}
					err := json.Unmarshal(w.Body.Bytes(), &response)
					require.NoError(t, err)
					assert.Contains(t, fmt.Sprintf("%v", response), tt.expectedError)
				}
				
				if tt.expectedStatus == http.StatusOK {
					var response map[string]interface{}
					err := json.Unmarshal(w.Body.Bytes(), &response)
					require.NoError(t, err)
					assert.NotEmpty(t, response["token"])
					assert.NotEmpty(t, response["refresh_token"])
				}
			})
		}
	})
}

// ====================================
// Call Lifecycle Tests
// ====================================

func TestHandler_CallLifecycle(t *testing.T) {
	handler, mocks := setupHandler(t)

	t.Run("UpdateCallStatus", func(t *testing.T) {
		callID := uuid.New()
		
		tests := []struct {
			name           string
			callID         string
			request        UpdateCallStatusRequest
			setupMocks     func()
			expectedStatus int
			expectedError  string
		}{
			{
				name:   "valid status transition: pending to ringing",
				callID: callID.String(),
				request: UpdateCallStatusRequest{
					Status: "ringing",
				},
				setupMocks: func() {
					// Mock get call
					existingCall := &call.Call{
						ID:     callID,
						Status: call.StatusPending,
					}
					mocks.Repositories.Call.On("GetByID", mock.Anything, callID).
						Return(existingCall, nil)
					
					// Mock update
					mocks.Repositories.Call.On("Update", mock.Anything, mock.AnythingOfType("*call.Call")).
						Return(nil)
				},
				expectedStatus: http.StatusOK,
			},
			{
				name:   "invalid status transition: completed to ringing",
				callID: callID.String(),
				request: UpdateCallStatusRequest{
					Status: "ringing",
				},
				setupMocks: func() {
					existingCall := &call.Call{
						ID:     callID,
						Status: call.StatusCompleted,
					}
					mocks.Repositories.Call.On("GetByID", mock.Anything, callID).
						Return(existingCall, nil)
				},
				expectedStatus: http.StatusBadRequest,
				expectedError:  "Invalid status transition",
			},
			{
				name:   "unknown status",
				callID: callID.String(),
				request: UpdateCallStatusRequest{
					Status: "unknown",
				},
				expectedStatus: http.StatusBadRequest,
				expectedError:  "Invalid status",
			},
			{
				name:   "call not found",
				callID: callID.String(),
				request: UpdateCallStatusRequest{
					Status: "ringing",
				},
				setupMocks: func() {
					mocks.Repositories.Call.On("GetByID", mock.Anything, callID).
						Return(nil, errors.NewNotFoundError("call"))
				},
				expectedStatus: http.StatusNotFound,
				expectedError:  "not found",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// Reset mocks
				mocks.Repositories.Call.ExpectedCalls = nil
				
				if tt.setupMocks != nil {
					tt.setupMocks()
				}
				
				w := makeRequest(handler, "PATCH", "/api/v1/calls/"+tt.callID+"/status", tt.request)
				
				assert.Equal(t, tt.expectedStatus, w.Code)
				
				if tt.expectedError != "" {
					var response map[string]interface{}
					err := json.Unmarshal(w.Body.Bytes(), &response)
					require.NoError(t, err)
					assert.Contains(t, fmt.Sprintf("%v", response), tt.expectedError)
				}
			})
		}
	})

	t.Run("CompleteCall", func(t *testing.T) {
		callID := uuid.New()
		
		tests := []struct {
			name           string
			callID         string
			request        CompleteCallRequest
			setupMocks     func()
			expectedStatus int
			expectedError  string
		}{
			{
				name:   "successful call completion",
				callID: callID.String(),
				request: CompleteCallRequest{
					Duration: 240, // 4 minutes
				},
				setupMocks: func() {
					existingCall := &call.Call{
						ID:       callID,
						Status:   call.StatusInProgress,
						BuyerID:  uuid.New(),
						SellerID: &uuid.UUID{},
					}
					mocks.Repositories.Call.On("GetByID", mock.Anything, callID).
						Return(existingCall, nil)
					
					// Mock update for status change
					mocks.Repositories.Call.On("Update", mock.Anything, mock.AnythingOfType("*call.Call")).
						Return(nil)
					
					// Mock financial transaction creation
					mocks.Repositories.Financial.On("CreateTransaction", mock.Anything, mock.AnythingOfType("*financial.Transaction")).
						Return(nil)
				},
				expectedStatus: http.StatusOK,
			},
			{
				name:   "call not in progress",
				callID: callID.String(),
				request: CompleteCallRequest{
					Duration: 240,
				},
				setupMocks: func() {
					existingCall := &call.Call{
						ID:     callID,
						Status: call.StatusPending,
					}
					mocks.Repositories.Call.On("GetByID", mock.Anything, callID).
						Return(existingCall, nil)
				},
				expectedStatus: http.StatusBadRequest,
				expectedError:  "Call must be in progress",
			},
			{
				name:   "negative duration",
				callID: callID.String(),
				request: CompleteCallRequest{
					Duration: -10,
				},
				expectedStatus: http.StatusBadRequest,
				expectedError:  "Duration must be positive",
			},
			{
				name:   "zero duration",
				callID: callID.String(),
				request: CompleteCallRequest{
					Duration: 0,
				},
				expectedStatus: http.StatusBadRequest,
				expectedError:  "Duration must be positive",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// Reset mocks
				mocks.Repositories.Call.ExpectedCalls = nil
				mocks.Repositories.Financial.ExpectedCalls = nil
				
				if tt.setupMocks != nil {
					tt.setupMocks()
				}
				
				w := makeRequest(handler, "POST", "/api/v1/calls/"+tt.callID+"/complete", tt.request)
				
				assert.Equal(t, tt.expectedStatus, w.Code)
				
				if tt.expectedError != "" {
					var response map[string]interface{}
					err := json.Unmarshal(w.Body.Bytes(), &response)
					require.NoError(t, err)
					assert.Contains(t, fmt.Sprintf("%v", response), tt.expectedError)
				}
			})
		}
	})
}

// ====================================
// Account Balance Tests
// ====================================

func TestHandler_AccountBalance(t *testing.T) {
	handler, mocks := setupHandler(t)

	t.Run("GetAccountBalance", func(t *testing.T) {
		userID := uuid.New()
		
		tests := []struct {
			name           string
			userID         string
			userType       string
			setupMocks     func()
			expectedStatus int
			expectedError  string
			validateBody   func(*testing.T, map[string]interface{})
		}{
			{
				name:     "successful balance retrieval",
				userID:   userID.String(),
				userType: "buyer",
				setupMocks: func() {
					mocks.Repositories.Account.On("GetBalance", mock.Anything, userID).
						Return(1234.56, nil)
				},
				expectedStatus: http.StatusOK,
				validateBody: func(t *testing.T, body map[string]interface{}) {
					assert.Equal(t, 1234.56, body["balance"])
					assert.Equal(t, "USD", body["currency"])
				},
			},
			{
				name:     "zero balance",
				userID:   userID.String(),
				userType: "seller",
				setupMocks: func() {
					mocks.Repositories.Account.On("GetBalance", mock.Anything, userID).
						Return(0.0, nil)
				},
				expectedStatus: http.StatusOK,
				validateBody: func(t *testing.T, body map[string]interface{}) {
					assert.Equal(t, 0.0, body["balance"])
				},
			},
			{
				name:     "account not found",
				userID:   userID.String(),
				userType: "buyer",
				setupMocks: func() {
					mocks.Repositories.Account.On("GetBalance", mock.Anything, userID).
						Return(0.0, errors.NewNotFoundError("account"))
				},
				expectedStatus: http.StatusNotFound,
				expectedError:  "Account not found",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// Reset mocks
				mocks.Repositories.Account.ExpectedCalls = nil
				
				if tt.setupMocks != nil {
					tt.setupMocks()
				}
				
				// Add user context
				req := httptest.NewRequest("GET", "/api/v1/account/balance", nil)
				ctx := context.WithValue(req.Context(), contextKeyUserID, tt.userID)
				ctx = context.WithValue(ctx, contextKeyAccountType, tt.userType)
				req = req.WithContext(ctx)
				req.Header.Set("Authorization", "Bearer test-token")
				
				w := httptest.NewRecorder()
				handler.ServeHTTP(w, req)
				
				assert.Equal(t, tt.expectedStatus, w.Code)
				
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				
				if tt.expectedError != "" {
					assert.Contains(t, fmt.Sprintf("%v", response), tt.expectedError)
				}
				
				if tt.validateBody != nil {
					tt.validateBody(t, response)
				}
			})
		}
	})
}

// ====================================
// Advanced Compliance Tests
// ====================================

func TestHandler_AdvancedCompliance(t *testing.T) {
	handler, mocks := setupHandler(t)

	t.Run("CheckDNC", func(t *testing.T) {
		tests := []struct {
			name           string
			phoneNumber    string
			setupMocks     func()
			expectedStatus int
			expectedError  string
			validateBody   func(*testing.T, map[string]interface{})
		}{
			{
				name:        "number on DNC list",
				phoneNumber: "+14155551234",
				setupMocks: func() {
					entry := &compliance.ConsentRecord{
						PhoneNumber: "+14155551234",
						Source:      "consumer request",
						Status:      compliance.ConsentStatus(0), // opted out
						CreatedAt:   time.Now().Add(-30 * 24 * time.Hour),
					}
					mocks.Repositories.Compliance.On("CheckDNC", mock.Anything, "+14155551234").
						Return(entry, nil)
				},
				expectedStatus: http.StatusOK,
				validateBody: func(t *testing.T, body map[string]interface{}) {
					assert.Equal(t, true, body["is_dnc"])
					assert.Equal(t, "federal", body["list_type"])
					assert.Equal(t, "consumer request", body["source"])
				},
			},
			{
				name:        "number not on DNC list",
				phoneNumber: "+18005551234",
				setupMocks: func() {
					mocks.Repositories.Compliance.On("CheckDNC", mock.Anything, "+18005551234").
						Return(nil, nil)
				},
				expectedStatus: http.StatusOK,
				validateBody: func(t *testing.T, body map[string]interface{}) {
					assert.Equal(t, false, body["is_dnc"])
				},
			},
			{
				name:           "invalid phone number format",
				phoneNumber:    "invalid",
				expectedStatus: http.StatusBadRequest,
				expectedError:  "Invalid phone number",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// Reset mocks
				mocks.Repositories.Compliance.ExpectedCalls = nil
				
				if tt.setupMocks != nil {
					tt.setupMocks()
				}
				
				w := makeRequest(handler, "GET", "/api/v1/compliance/dnc/"+tt.phoneNumber, nil)
				
				assert.Equal(t, tt.expectedStatus, w.Code)
				
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				
				if tt.expectedError != "" {
					assert.Contains(t, fmt.Sprintf("%v", response), tt.expectedError)
				}
				
				if tt.validateBody != nil {
					tt.validateBody(t, response)
				}
			})
		}
	})

	t.Run("GetTCPAHours", func(t *testing.T) {
		tests := []struct {
			name           string
			setupMocks     func()
			expectedStatus int
			validateBody   func(*testing.T, map[string]interface{})
		}{
			{
				name: "retrieve TCPA hours",
				setupMocks: func() {
					hours := &compliance.TCPARestrictions{
						TimeZone:  "America/New_York",
						StartTime: "09:00",
						EndTime:   "20:00",
					}
					mocks.Repositories.Compliance.On("GetTCPAHours", mock.Anything).
						Return(hours, nil)
				},
				expectedStatus: http.StatusOK,
				validateBody: func(t *testing.T, body map[string]interface{}) {
					assert.Equal(t, "09:00", body["start_time"])
					assert.Equal(t, "20:00", body["end_time"])
					assert.Equal(t, "America/New_York", body["timezone"])
					
					days := body["allowed_days"].([]interface{})
					assert.Len(t, days, 5)
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// Reset mocks
				mocks.Repositories.Compliance.ExpectedCalls = nil
				
				if tt.setupMocks != nil {
					tt.setupMocks()
				}
				
				w := makeRequest(handler, "GET", "/api/v1/compliance/tcpa/hours", nil)
				
				assert.Equal(t, tt.expectedStatus, w.Code)
				
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				
				if tt.validateBody != nil {
					tt.validateBody(t, response)
				}
			})
		}
	})
}

// ====================================
// Edge Cases and Error Scenarios
// ====================================

func TestHandler_EdgeCases(t *testing.T) {
	handler, mocks := setupHandler(t)

	t.Run("MaximumBidAmount", func(t *testing.T) {
		tests := []struct {
			name           string
			request        CreateBidRequest
			expectedStatus int
			expectedError  string
		}{
			{
				name: "extremely high bid amount",
				request: CreateBidRequest{
					AuctionID: uuid.New(),
					Amount:    1000000.00, // $1M
				},
				expectedStatus: http.StatusBadRequest,
				expectedError:  "Bid amount exceeds maximum",
			},
			{
				name: "very small bid amount",
				request: CreateBidRequest{
					AuctionID: uuid.New(),
					Amount:    0.001, // Less than 1 cent
				},
				expectedStatus: http.StatusBadRequest,
				expectedError:  "Bid amount too small",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				w := makeRequest(handler, "POST", "/api/v1/bids", tt.request)
				
				assert.Equal(t, tt.expectedStatus, w.Code)
				
				if tt.expectedError != "" {
					var response map[string]interface{}
					err := json.Unmarshal(w.Body.Bytes(), &response)
					require.NoError(t, err)
					assert.Contains(t, fmt.Sprintf("%v", response), tt.expectedError)
				}
			})
		}
	})

	t.Run("ConcurrentStateChanges", func(t *testing.T) {
		callID := uuid.New()
		
		// Simulate concurrent status updates
		mocks.Repositories.Call.On("GetByID", mock.Anything, callID).
			Return(&call.Call{
				ID:     callID,
				Status: call.StatusPending,
			}, nil).Once()
		
		mocks.Repositories.Call.On("Update", mock.Anything, mock.AnythingOfType("*call.Call")).
			Return(errors.NewConflictError("Call status already changed")).Once()
		
		request := UpdateCallStatusRequest{
			Status: "ringing",
		}
		
		w := makeRequest(handler, "PATCH", "/api/v1/calls/"+callID.String()+"/status", request)
		
		assert.Equal(t, http.StatusConflict, w.Code)
		
		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Contains(t, fmt.Sprintf("%v", response), "already changed")
	})

	t.Run("MalformedJSON", func(t *testing.T) {
		malformedJSON := `{"from_number": "+14155551234", "to_number": "+18005551234", invalid json}`
		
		req := httptest.NewRequest("POST", "/api/v1/calls", bytes.NewBufferString(malformedJSON))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer test-token")
		
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		
		assert.Equal(t, http.StatusBadRequest, w.Code)
		
		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Contains(t, fmt.Sprintf("%v", response), "Invalid JSON")
	})
}

// ====================================
// Rate Limiting Tests
// ====================================

func TestHandler_RateLimiting(t *testing.T) {
	handler, _ := setupHandler(t)

	t.Run("ExceedRateLimit", func(t *testing.T) {
		// Make many requests rapidly
		hitRateLimit := false
		
		for i := 0; i < 200; i++ {
			w := makeRequest(handler, "GET", "/api/v1/calls", nil)
			
			if w.Code == http.StatusTooManyRequests {
				hitRateLimit = true
				
				// Check rate limit headers
				assert.NotEmpty(t, w.Header().Get("X-RateLimit-Limit"))
				assert.NotEmpty(t, w.Header().Get("X-RateLimit-Remaining"))
				assert.NotEmpty(t, w.Header().Get("X-RateLimit-Reset"))
				break
			}
		}
		
		assert.True(t, hitRateLimit, "Should hit rate limit after many requests")
	})
}

// ====================================
// Helper Functions for Testing
// ====================================


