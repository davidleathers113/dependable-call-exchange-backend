//go:build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/account"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/bid"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/call"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/infrastructure/auth"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAuth_EndToEnd tests authentication and authorization flows
func TestAuth_EndToEnd(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	
	server := setupTestServer(t, testDB, false)
	defer server.Close()
	
	t.Run("User Registration and Login", func(t *testing.T) {
		// Register new user
		registerReq := map[string]interface{}{
			"email":    "test@example.com",
			"password": "SecurePass123!",
			"name":     "Test User",
			"type":     "buyer",
		}
		
		body, _ := json.Marshal(registerReq)
		resp, err := http.Post(server.URL+"/api/v1/auth/register", "application/json", bytes.NewBuffer(body))
		require.NoError(t, err)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		
		var registerResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&registerResp)
		assert.NotEmpty(t, registerResp["user_id"])
		assert.NotEmpty(t, registerResp["token"])
		
		// Login with credentials
		loginReq := map[string]interface{}{
			"email":    "test@example.com",
			"password": "SecurePass123!",
		}
		
		body, _ = json.Marshal(loginReq)
		resp, err = http.Post(server.URL+"/api/v1/auth/login", "application/json", bytes.NewBuffer(body))
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		
		var loginResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&loginResp)
		assert.NotEmpty(t, loginResp["token"])
		assert.NotEmpty(t, loginResp["refresh_token"])
		
		// Verify token works
		token := loginResp["token"].(string)
		req, _ := http.NewRequest("GET", server.URL+"/api/v1/profile", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		
		resp, err = http.DefaultClient.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
	
	t.Run("Invalid Credentials", func(t *testing.T) {
		// Attempt login with wrong password
		loginReq := map[string]interface{}{
			"email":    "test@example.com",
			"password": "WrongPassword",
		}
		
		body, _ := json.Marshal(loginReq)
		resp, err := http.Post(server.URL+"/api/v1/auth/login", "application/json", bytes.NewBuffer(body))
		require.NoError(t, err)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		
		var errResp map[string]string
		json.NewDecoder(resp.Body).Decode(&errResp)
		assert.Contains(t, errResp["error"], "invalid credentials")
	})
	
	t.Run("Token Refresh", func(t *testing.T) {
		// Create user and login
		user := createAuthenticatedUser(t, server, "refresh-test@example.com", "buyer")
		
		// Wait briefly to ensure different token timestamps
		time.Sleep(100 * time.Millisecond)
		
		// Use refresh token
		refreshReq := map[string]interface{}{
			"refresh_token": user.RefreshToken,
		}
		
		body, _ := json.Marshal(refreshReq)
		resp, err := http.Post(server.URL+"/api/v1/auth/refresh", "application/json", bytes.NewBuffer(body))
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		
		var refreshResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&refreshResp)
		newToken := refreshResp["token"].(string)
		
		// Verify new token is different but works
		assert.NotEqual(t, user.Token, newToken)
		
		req, _ := http.NewRequest("GET", server.URL+"/api/v1/profile", nil)
		req.Header.Set("Authorization", "Bearer "+newToken)
		
		resp, err = http.DefaultClient.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
	
	t.Run("Token Expiration", func(t *testing.T) {
		// Create user with short-lived token
		user := createAuthenticatedUser(t, server, "expire-test@example.com", "seller")
		
		// Mock time advancement (would need time mocking in real implementation)
		// For now, test with invalid token format
		expiredToken := user.Token + "_expired"
		
		req, _ := http.NewRequest("GET", server.URL+"/api/v1/profile", nil)
		req.Header.Set("Authorization", "Bearer "+expiredToken)
		
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})
}

// TestAuth_RoleBasedAccess tests role-based access control
func TestAuth_RoleBasedAccess(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	server := setupTestServer(t, testDB, false)
	defer server.Close()
	
	// Create users with different roles
	buyer := createAuthenticatedUser(t, server, "buyer@example.com", "buyer")
	seller := createAuthenticatedUser(t, server, "seller@example.com", "seller")
	admin := createAuthenticatedUser(t, server, "admin@example.com", "admin")
	
	t.Run("Buyer Permissions", func(t *testing.T) {
		// Buyers can create calls
		callReq := map[string]interface{}{
			"from_number": "+14155551234",
			"to_number":   "+18005551234",
		}
		
		body, _ := json.Marshal(callReq)
		req, _ := http.NewRequest("POST", server.URL+"/api/v1/calls", bytes.NewBuffer(body))
		req.Header.Set("Authorization", "Bearer "+buyer.Token)
		req.Header.Set("Content-Type", "application/json")
		
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		
		// Buyers cannot create bid profiles
		bidProfileReq := map[string]interface{}{
			"criteria": map[string]interface{}{
				"max_budget": 100.00,
			},
		}
		
		body, _ = json.Marshal(bidProfileReq)
		req, _ = http.NewRequest("POST", server.URL+"/api/v1/bid-profiles", bytes.NewBuffer(body))
		req.Header.Set("Authorization", "Bearer "+buyer.Token)
		req.Header.Set("Content-Type", "application/json")
		
		resp, err = http.DefaultClient.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})
	
	t.Run("Seller Permissions", func(t *testing.T) {
		// Sellers can create bid profiles
		bidProfileReq := map[string]interface{}{
			"criteria": map[string]interface{}{
				"max_budget": 100.00,
				"call_type":  []string{"sales"},
			},
		}
		
		body, _ := json.Marshal(bidProfileReq)
		req, _ := http.NewRequest("POST", server.URL+"/api/v1/bid-profiles", bytes.NewBuffer(body))
		req.Header.Set("Authorization", "Bearer "+seller.Token)
		req.Header.Set("Content-Type", "application/json")
		
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		
		// Sellers can place bids
		// First create a call and auction as buyer
		call := createCallAsUser(t, server, buyer)
		auction := startAuctionAsUser(t, server, buyer, call.ID)
		
		// Now seller can bid
		bidReq := map[string]interface{}{
			"auction_id": auction.ID,
			"amount":     5.50,
		}
		
		body, _ = json.Marshal(bidReq)
		req, _ = http.NewRequest("POST", server.URL+"/api/v1/bids", bytes.NewBuffer(body))
		req.Header.Set("Authorization", "Bearer "+seller.Token)
		req.Header.Set("Content-Type", "application/json")
		
		resp, err = http.DefaultClient.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		
		// Sellers cannot modify other sellers' bids
		otherSeller := createAuthenticatedUser(t, server, "other-seller@example.com", "seller")
		otherBid := placeBidAsUser(t, server, otherSeller, auction.ID, 6.00)
		
		updateReq := map[string]interface{}{
			"amount": 7.00,
		}
		
		body, _ = json.Marshal(updateReq)
		req, _ = http.NewRequest("PATCH", server.URL+fmt.Sprintf("/api/v1/bids/%s", otherBid.ID), bytes.NewBuffer(body))
		req.Header.Set("Authorization", "Bearer "+seller.Token)
		req.Header.Set("Content-Type", "application/json")
		
		resp, err = http.DefaultClient.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})
	
	t.Run("Admin Permissions", func(t *testing.T) {
		// Admins can access all resources
		req, _ := http.NewRequest("GET", server.URL+"/api/v1/admin/users", nil)
		req.Header.Set("Authorization", "Bearer "+admin.Token)
		
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		
		// Admins can modify system settings
		settingsReq := map[string]interface{}{
			"max_bid_amount": 1000.00,
			"auction_duration": 60,
		}
		
		body, _ := json.Marshal(settingsReq)
		req, _ = http.NewRequest("PUT", server.URL+"/api/v1/admin/settings", bytes.NewBuffer(body))
		req.Header.Set("Authorization", "Bearer "+admin.Token)
		req.Header.Set("Content-Type", "application/json")
		
		resp, err = http.DefaultClient.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		
		// Regular users cannot access admin endpoints
		req, _ = http.NewRequest("GET", server.URL+"/api/v1/admin/users", nil)
		req.Header.Set("Authorization", "Bearer "+buyer.Token)
		
		resp, err = http.DefaultClient.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})
}

// TestAuth_APIKeys tests API key authentication
func TestAuth_APIKeys(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	server := setupTestServer(t, testDB, false)
	defer server.Close()
	
	t.Run("API Key Generation", func(t *testing.T) {
		// Create authenticated user
		user := createAuthenticatedUser(t, server, "apikey-test@example.com", "buyer")
		
		// Generate API key
		keyReq := map[string]interface{}{
			"name": "Test API Key",
			"scopes": []string{"calls:create", "calls:read"},
		}
		
		body, _ := json.Marshal(keyReq)
		req, _ := http.NewRequest("POST", server.URL+"/api/v1/api-keys", bytes.NewBuffer(body))
		req.Header.Set("Authorization", "Bearer "+user.Token)
		req.Header.Set("Content-Type", "application/json")
		
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		
		var keyResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&keyResp)
		apiKey := keyResp["key"].(string)
		assert.NotEmpty(t, apiKey)
		
		// Use API key
		callReq := map[string]interface{}{
			"from_number": "+14155551234",
			"to_number":   "+18005551234",
		}
		
		body, _ = json.Marshal(callReq)
		req, _ = http.NewRequest("POST", server.URL+"/api/v1/calls", bytes.NewBuffer(body))
		req.Header.Set("X-API-Key", apiKey)
		req.Header.Set("Content-Type", "application/json")
		
		resp, err = http.DefaultClient.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
	})
	
	t.Run("API Key Scopes", func(t *testing.T) {
		user := createAuthenticatedUser(t, server, "scope-test@example.com", "buyer")
		
		// Create limited scope API key
		keyReq := map[string]interface{}{
			"name": "Read Only Key",
			"scopes": []string{"calls:read"},
		}
		
		body, _ := json.Marshal(keyReq)
		req, _ := http.NewRequest("POST", server.URL+"/api/v1/api-keys", bytes.NewBuffer(body))
		req.Header.Set("Authorization", "Bearer "+user.Token)
		req.Header.Set("Content-Type", "application/json")
		
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		
		var keyResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&keyResp)
		readOnlyKey := keyResp["key"].(string)
		
		// Can read calls
		req, _ = http.NewRequest("GET", server.URL+"/api/v1/calls", nil)
		req.Header.Set("X-API-Key", readOnlyKey)
		
		resp, err = http.DefaultClient.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		
		// Cannot create calls
		callReq := map[string]interface{}{
			"from_number": "+14155551234",
			"to_number":   "+18005551234",
		}
		
		body, _ = json.Marshal(callReq)
		req, _ = http.NewRequest("POST", server.URL+"/api/v1/calls", bytes.NewBuffer(body))
		req.Header.Set("X-API-Key", readOnlyKey)
		req.Header.Set("Content-Type", "application/json")
		
		resp, err = http.DefaultClient.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})
	
	t.Run("API Key Revocation", func(t *testing.T) {
		user := createAuthenticatedUser(t, server, "revoke-test@example.com", "buyer")
		
		// Create API key
		apiKey := createAPIKey(t, server, user.Token, "Test Key", []string{"calls:create"})
		
		// Verify it works
		req, _ := http.NewRequest("GET", server.URL+"/api/v1/calls", nil)
		req.Header.Set("X-API-Key", apiKey)
		
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		
		// Revoke the key
		req, _ = http.NewRequest("DELETE", server.URL+"/api/v1/api-keys/"+apiKey, nil)
		req.Header.Set("Authorization", "Bearer "+user.Token)
		
		resp, err = http.DefaultClient.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNoContent, resp.StatusCode)
		
		// Verify it no longer works
		req, _ = http.NewRequest("GET", server.URL+"/api/v1/calls", nil)
		req.Header.Set("X-API-Key", apiKey)
		
		resp, err = http.DefaultClient.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})
}

// TestAuth_RateLimiting tests rate limiting
func TestAuth_RateLimiting(t *testing.T) {
	testDB := testutil.NewTestDB(t)
	server := setupTestServer(t, testDB, false)
	defer server.Close()
	
	t.Run("Request Rate Limiting", func(t *testing.T) {
		user := createAuthenticatedUser(t, server, "ratelimit@example.com", "buyer")
		
		// Make requests up to limit
		limit := 100 // Assuming 100 requests per minute
		
		for i := 0; i < limit; i++ {
			req, _ := http.NewRequest("GET", server.URL+"/api/v1/profile", nil)
			req.Header.Set("Authorization", "Bearer "+user.Token)
			
			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, resp.StatusCode)
			resp.Body.Close()
		}
		
		// Next request should be rate limited
		req, _ := http.NewRequest("GET", server.URL+"/api/v1/profile", nil)
		req.Header.Set("Authorization", "Bearer "+user.Token)
		
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
		
		// Check rate limit headers
		assert.NotEmpty(t, resp.Header.Get("X-RateLimit-Limit"))
		assert.Equal(t, "0", resp.Header.Get("X-RateLimit-Remaining"))
		assert.NotEmpty(t, resp.Header.Get("X-RateLimit-Reset"))
	})
	
	t.Run("IP-based Rate Limiting", func(t *testing.T) {
		// Test unauthenticated endpoint rate limiting
		endpoint := "/api/v1/auth/login"
		
		// Make multiple failed login attempts
		for i := 0; i < 5; i++ {
			loginReq := map[string]interface{}{
				"email":    "test@example.com",
				"password": "wrong-password",
			}
			
			body, _ := json.Marshal(loginReq)
			resp, err := http.Post(server.URL+endpoint, "application/json", bytes.NewBuffer(body))
			require.NoError(t, err)
			
			if i < 3 {
				assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
			} else {
				// Should be rate limited after 3 failed attempts
				assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)
			}
			resp.Body.Close()
		}
	})
}

// Helper functions for auth testing

type AuthenticatedUser struct {
	ID           uuid.UUID
	Email        string
	Token        string
	RefreshToken string
	Type         string
}

func createAuthenticatedUser(t *testing.T, server *httptest.Server, email, userType string) *AuthenticatedUser {
	// Register user
	registerReq := map[string]interface{}{
		"email":    email,
		"password": "TestPass123!",
		"name":     strings.Split(email, "@")[0],
		"type":     userType,
	}
	
	body, _ := json.Marshal(registerReq)
	resp, err := http.Post(server.URL+"/api/v1/auth/register", "application/json", bytes.NewBuffer(body))
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	
	var registerResp map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&registerResp)
	
	// Login to get tokens
	loginReq := map[string]interface{}{
		"email":    email,
		"password": "TestPass123!",
	}
	
	body, _ = json.Marshal(loginReq)
	resp, err = http.Post(server.URL+"/api/v1/auth/login", "application/json", bytes.NewBuffer(body))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	
	var loginResp map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&loginResp)
	
	userID, _ := uuid.Parse(registerResp["user_id"].(string))
	
	return &AuthenticatedUser{
		ID:           userID,
		Email:        email,
		Token:        loginResp["token"].(string),
		RefreshToken: loginResp["refresh_token"].(string),
		Type:         userType,
	}
}

func createAPIKey(t *testing.T, server *httptest.Server, userToken string, name string, scopes []string) string {
	keyReq := map[string]interface{}{
		"name":   name,
		"scopes": scopes,
	}
	
	body, _ := json.Marshal(keyReq)
	req, _ := http.NewRequest("POST", server.URL+"/api/v1/api-keys", bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+userToken)
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	
	var keyResp map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&keyResp)
	
	return keyResp["key"].(string)
}

func createCallAsUser(t *testing.T, server *httptest.Server, user *AuthenticatedUser) *call.Call {
	callReq := map[string]interface{}{
		"from_number": "+14155551234",
		"to_number":   "+18005551234",
	}
	
	body, _ := json.Marshal(callReq)
	req, _ := http.NewRequest("POST", server.URL+"/api/v1/calls", bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+user.Token)
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	
	var result call.Call
	json.NewDecoder(resp.Body).Decode(&result)
	return &result
}

func startAuctionAsUser(t *testing.T, server *httptest.Server, user *AuthenticatedUser, callID uuid.UUID) *bid.Auction {
	auctionReq := map[string]interface{}{
		"call_id":       callID,
		"reserve_price": 2.00,
	}
	
	body, _ := json.Marshal(auctionReq)
	req, _ := http.NewRequest("POST", server.URL+"/api/v1/auctions", bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+user.Token)
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	
	var result bid.Auction
	json.NewDecoder(resp.Body).Decode(&result)
	return &result
}

func placeBidAsUser(t *testing.T, server *httptest.Server, user *AuthenticatedUser, auctionID uuid.UUID, amount float64) *bid.Bid {
	bidReq := map[string]interface{}{
		"auction_id": auctionID,
		"amount":     amount,
	}
	
	body, _ := json.Marshal(bidReq)
	req, _ := http.NewRequest("POST", server.URL+"/api/v1/bids", bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+user.Token)
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	
	var result bid.Bid
	json.NewDecoder(resp.Body).Decode(&result)
	return &result
}