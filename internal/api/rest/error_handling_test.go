package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestErrorHandling_ServiceErrors tests how various service errors are handled
func TestErrorHandling_ServiceErrors(t *testing.T) {
	tests := []struct {
		name           string
		setupMocks     func(*SimpleMockServices)
		makeRequest    func(handler http.Handler) *httptest.ResponseRecorder
		expectedStatus int
		expectedError  string
		expectedCode   string
	}{
		{
			name: "call routing service timeout",
			setupMocks: func(m *SimpleMockServices) {
				m.CallRouting.On("RouteCall", mock.Anything, mock.Anything).
					Return(nil, context.DeadlineExceeded)
			},
			makeRequest: func(handler http.Handler) *httptest.ResponseRecorder {
				return makeRequest(handler, "POST", "/api/v1/calls/"+uuid.New().String()+"/route", nil)
			},
			expectedStatus: http.StatusInternalServerError,
			expectedError:  "ROUTING_FAILED",
			expectedCode:   "ROUTING_FAILED",
		},
		{
			name: "call routing service not found",
			setupMocks: func(m *SimpleMockServices) {
				m.CallRouting.On("RouteCall", mock.Anything, mock.Anything).
					Return(nil, errors.NewNotFoundError("call"))
			},
			makeRequest: func(handler http.Handler) *httptest.ResponseRecorder {
				return makeRequest(handler, "POST", "/api/v1/calls/"+uuid.New().String()+"/route", nil)
			},
			expectedStatus: http.StatusNotFound,
			expectedError:  "CALL_NOT_FOUND",
			expectedCode:   "CALL_NOT_FOUND",
		},
		{
			name: "call routing validation error", 
			setupMocks: func(m *SimpleMockServices) {
				m.CallRouting.On("RouteCall", mock.Anything, mock.Anything).
					Return(nil, errors.NewValidationError("INVALID_STATE", "call not in pending state"))
			},
			makeRequest: func(handler http.Handler) *httptest.ResponseRecorder {
				return makeRequest(handler, "POST", "/api/v1/calls/"+uuid.New().String()+"/route", nil)
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "INVALID_STATE",
			expectedCode:   "INVALID_STATE",
		},
		{
			name: "call creation with invalid phone number",
			makeRequest: func(handler http.Handler) *httptest.ResponseRecorder {
				request := CreateCallRequest{
					FromNumber: "invalid-phone",
					ToNumber:   "+18005551234",
				}
				return makeRequest(handler, "POST", "/api/v1/calls", request)
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid from number",
			expectedCode:   "INVALID_PHONE_NUMBER",
		},
		{
			name: "auction creation with negative reserve price",
			makeRequest: func(handler http.Handler) *httptest.ResponseRecorder {
				return makeRequest(handler, "POST", "/api/v1/auctions", CreateAuctionRequest{
					CallID:       uuid.New(),
					ReservePrice: -5.00,
					Duration:     30,
				})
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Reserve price cannot be negative",
			expectedCode:   "INVALID_RESERVE_PRICE",
		},
		{
			name: "bid creation with negative amount",
			makeRequest: func(handler http.Handler) *httptest.ResponseRecorder {
				return makeRequest(handler, "POST", "/api/v1/bids", CreateBidRequest{
					AuctionID: uuid.New(),
					Amount:    -10.00,
				})
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Bid amount must be positive",
			expectedCode:   "INVALID_AMOUNT",
		},
		{
			name: "compliance DNC invalid phone number",
			makeRequest: func(handler http.Handler) *httptest.ResponseRecorder {
				return makeRequest(handler, "POST", "/api/v1/compliance/dnc", AddDNCRequest{
					PhoneNumber: "invalid-phone",
					Reason:      "consumer request",
				})
			},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Invalid phone number",
			expectedCode:   "INVALID_PHONE_NUMBER",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler, mocks := setupHandler(t)
			
			if tt.setupMocks != nil {
				tt.setupMocks(mocks)
			}
			
			w := tt.makeRequest(handler)
			
			assert.Equal(t, tt.expectedStatus, w.Code)
			
			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			
			// Check error structure
			if w.Code >= 400 {
				assert.Contains(t, fmt.Sprintf("%v", response), tt.expectedError)
			}
		})
	}
}
// TestErrorHandling_Panics tests panic recovery
func TestErrorHandling_Panics(t *testing.T) {
	handler, mocks := setupHandler(t)
	
	// Simulate a panic in service
	mocks.CallRouting.On("RouteCall", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			panic("unexpected nil pointer")
		}).Once()
	
	w := makeRequest(handler, "POST", "/api/v1/calls/"+uuid.New().String()+"/route", nil)
	
	// Should recover and return 500
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	
	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	
	// Check for nested error structure
	errorObj, ok := response["error"].(map[string]interface{})
	require.True(t, ok, "Expected error object in response")
	assert.Equal(t, "INTERNAL_ERROR", errorObj["code"])
}

// TestErrorHandling_RateLimiting tests rate limit errors
func TestErrorHandling_RateLimiting(t *testing.T) {
	handler, _ := setupHandler(t)
	
	// Simulate rate limit by making many requests
	// Note: This assumes rate limiting middleware is configured
	numRequests := 100
	results := make([]int, numRequests)
	
	for i := 0; i < numRequests; i++ {
		w := makeRequest(handler, "GET", "/api/v1/calls", nil)
		results[i] = w.Code
	}
	
	// Check that some requests were rate limited
	rateLimited := 0
	for _, code := range results {
		if code == http.StatusTooManyRequests {
			rateLimited++
		}
	}
	
	// At least some requests should be rate limited
	// Exact number depends on rate limit configuration
	assert.Greater(t, rateLimited, 0, "Expected some requests to be rate limited")
}

// TestErrorHandling_AuthErrors tests authentication and authorization errors
func TestErrorHandling_AuthErrors(t *testing.T) {
	handler, _ := setupHandler(t)
	
	tests := []struct {
		name           string
		setupRequest   func() *http.Request
		expectedStatus int
		expectedError  string
	}{
		{
			name: "missing authorization header",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/api/v1/calls", nil)
				// No authorization header
				return req
			},
			expectedStatus: http.StatusUnauthorized,
			expectedError:  "Authorization required",
		},
		{
			name: "invalid token format",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/api/v1/calls", nil)
				req.Header.Set("Authorization", "InvalidToken")
				return req
			},
			expectedStatus: http.StatusUnauthorized,
			expectedError:  "Invalid authorization format",
		},
		{
			name: "expired token",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/api/v1/calls", nil)
				req.Header.Set("Authorization", "Bearer expired.token.here")
				return req
			},
			expectedStatus: http.StatusUnauthorized,
			expectedError:  "Token expired",
		},
		{
			name: "insufficient permissions",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/api/v1/admin/users", nil)
				req.Header.Set("Authorization", "Bearer valid.buyer.token")
				ctx := context.WithValue(req.Context(), "account_type", "buyer")
				return req.WithContext(ctx)
			},
			expectedStatus: http.StatusForbidden,
			expectedError:  "Insufficient permissions",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := tt.setupRequest()
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			
			assert.Equal(t, tt.expectedStatus, w.Code)
			
			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)
			
			assert.Contains(t, fmt.Sprintf("%v", response), tt.expectedError)
		})
	}
}

// Helper function for making requests with context
func makeRequestWithContext(handler http.Handler, req *http.Request, body interface{}) *httptest.ResponseRecorder {
	if body != nil {
		jsonBody, _ := json.Marshal(body)
		req.Body = io.NopCloser(bytes.NewReader(jsonBody))
		req.Header.Set("Content-Type", "application/json")
	}
	
	if !isPublicEndpoint(req.URL.Path) {
		req.Header.Set("Authorization", "Bearer test-token")
	}
	
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	return w
}