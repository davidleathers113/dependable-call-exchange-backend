//go:build e2e

package e2e

import (
	"testing"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/test/e2e/infrastructure"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuth_EndToEnd(t *testing.T) {
	// Setup test environment with Testcontainers
	env := infrastructure.NewTestEnvironment(t)
	client := infrastructure.NewAPIClient(t, env.APIURL)
	
	t.Run("User Registration and Login", func(t *testing.T) {
		// Reset database for test isolation
		env.ResetDatabase()
		
		// Register new user
		registerReq := map[string]interface{}{
			"email":    "test@example.com",
			"password": "SecurePass123!",
			"name":     "Test User",
			"type":     "buyer",
		}
		
		resp := client.Post("/api/v1/auth/register", registerReq)
		assert.Equal(t, 201, resp.StatusCode)
		
		var registerResp map[string]interface{}
		client.DecodeResponse(resp, &registerResp)
		assert.NotEmpty(t, registerResp["user_id"])
		assert.NotEmpty(t, registerResp["token"])
		
		// Login with credentials
		loginReq := map[string]interface{}{
			"email":    "test@example.com",
			"password": "SecurePass123!",
		}
		
		resp = client.Post("/api/v1/auth/login", loginReq)
		assert.Equal(t, 200, resp.StatusCode)
		
		var loginResp map[string]interface{}
		client.DecodeResponse(resp, &loginResp)
		assert.NotEmpty(t, loginResp["token"])
		assert.NotEmpty(t, loginResp["refresh_token"])
		
		// Verify token works
		client.SetToken(loginResp["token"].(string))
		resp = client.Get("/api/v1/profile")
		assert.Equal(t, 200, resp.StatusCode)
	})
	
	t.Run("Invalid Credentials", func(t *testing.T) {
		env.ResetDatabase()
		
		// Create a user first
		createTestUser(t, client, "test@example.com", "buyer")
		
		// Attempt login with wrong password
		loginReq := map[string]interface{}{
			"email":    "test@example.com",
			"password": "WrongPassword",
		}
		
		resp := client.Post("/api/v1/auth/login", loginReq)
		assert.Equal(t, 401, resp.StatusCode)
		
		var errResp map[string]string
		client.DecodeResponse(resp, &errResp)
		assert.Contains(t, errResp["error"], "invalid credentials")
	})
	
	t.Run("Token Refresh", func(t *testing.T) {
		env.ResetDatabase()
		
		// Create user and login
		user := createAuthenticatedUser(t, client, "refresh-test@example.com", "buyer")
		
		// Wait briefly to ensure different token timestamps
		time.Sleep(100 * time.Millisecond)
		
		// Use refresh token
		refreshReq := map[string]interface{}{
			"refresh_token": user.RefreshToken,
		}
		
		resp := client.Post("/api/v1/auth/refresh", refreshReq)
		assert.Equal(t, 200, resp.StatusCode)
		
		var refreshResp map[string]interface{}
		client.DecodeResponse(resp, &refreshResp)
		newToken := refreshResp["token"].(string)
		
		// Verify new token is different but works
		assert.NotEqual(t, user.Token, newToken)
		
		client.SetToken(newToken)
		resp = client.Get("/api/v1/profile")
		assert.Equal(t, 200, resp.StatusCode)
	})
}

func TestAuth_RoleBasedAccess(t *testing.T) {
	env := infrastructure.NewTestEnvironment(t)
	client := infrastructure.NewAPIClient(t, env.APIURL)
	
	// Create users with different roles
	env.ResetDatabase()
	buyer := createAuthenticatedUser(t, client, "buyer@example.com", "buyer")
	seller := createAuthenticatedUser(t, client, "seller@example.com", "seller")
	admin := createAuthenticatedUser(t, client, "admin@example.com", "admin")
	
	t.Run("Buyer Permissions", func(t *testing.T) {
		client.SetToken(buyer.Token)
		
		// Buyers can create calls
		callReq := map[string]interface{}{
			"from_number": "+14155551234",
			"to_number":   "+18005551234",
		}
		
		resp := client.Post("/api/v1/calls", callReq)
		assert.Equal(t, 201, resp.StatusCode)
		
		// Buyers cannot create bid profiles
		bidProfileReq := map[string]interface{}{
			"criteria": map[string]interface{}{
				"max_budget": 100.00,
			},
		}
		
		resp = client.Post("/api/v1/bid-profiles", bidProfileReq)
		assert.Equal(t, 403, resp.StatusCode)
	})
	
	t.Run("Seller Permissions", func(t *testing.T) {
		client.SetToken(seller.Token)
		
		// Sellers can create bid profiles
		bidProfileReq := map[string]interface{}{
			"criteria": map[string]interface{}{
				"max_budget": 100.00,
				"call_type":  []string{"sales"},
			},
		}
		
		resp := client.Post("/api/v1/bid-profiles", bidProfileReq)
		assert.Equal(t, 201, resp.StatusCode)
	})
	
	t.Run("Admin Permissions", func(t *testing.T) {
		client.SetToken(admin.Token)
		
		// Admins can access all resources
		resp := client.Get("/api/v1/admin/users")
		assert.Equal(t, 200, resp.StatusCode)
		
		// Admins can modify system settings
		settingsReq := map[string]interface{}{
			"max_bid_amount":   1000.00,
			"auction_duration": 60,
		}
		
		resp = client.Put("/api/v1/admin/settings", settingsReq)
		assert.Equal(t, 200, resp.StatusCode)
		
		// Regular users cannot access admin endpoints
		client.SetToken(buyer.Token)
		resp = client.Get("/api/v1/admin/users")
		assert.Equal(t, 403, resp.StatusCode)
	})
}

func TestAuth_APIKeys(t *testing.T) {
	env := infrastructure.NewTestEnvironment(t)
	client := infrastructure.NewAPIClient(t, env.APIURL)
	
	t.Run("API Key Generation", func(t *testing.T) {
		env.ResetDatabase()
		user := createAuthenticatedUser(t, client, "apikey-test@example.com", "buyer")
		client.SetToken(user.Token)
		
		// Generate API key
		keyReq := map[string]interface{}{
			"name":   "Test API Key",
			"scopes": []string{"calls:create", "calls:read"},
		}
		
		resp := client.Post("/api/v1/api-keys", keyReq)
		assert.Equal(t, 201, resp.StatusCode)
		
		var keyResp map[string]interface{}
		client.DecodeResponse(resp, &keyResp)
		apiKey := keyResp["key"].(string)
		assert.NotEmpty(t, apiKey)
		
		// Use API key
		client.SetToken("") // Clear JWT token
		req := client.Post("/api/v1/calls", map[string]interface{}{
			"from_number": "+14155551234",
			"to_number":   "+18005551234",
		})
		req.Header.Set("X-API-Key", apiKey)
		
		assert.Equal(t, 201, req.StatusCode)
	})
}

func TestAuth_RateLimiting(t *testing.T) {
	env := infrastructure.NewTestEnvironment(t)
	client := infrastructure.NewAPIClient(t, env.APIURL)
	
	t.Run("Request Rate Limiting", func(t *testing.T) {
		env.ResetDatabase()
		user := createAuthenticatedUser(t, client, "ratelimit@example.com", "buyer")
		client.SetToken(user.Token)
		
		// Make requests up to limit
		limit := 100 // Assuming 100 requests per minute
		
		for i := 0; i < limit+5; i++ {
			resp := client.Get("/api/v1/profile")
			
			if i < limit {
				assert.Equal(t, 200, resp.StatusCode)
			} else {
				// Should be rate limited
				assert.Equal(t, 429, resp.StatusCode)
				assert.NotEmpty(t, resp.Header.Get("X-RateLimit-Limit"))
				assert.Equal(t, "0", resp.Header.Get("X-RateLimit-Remaining"))
				break
			}
			resp.Body.Close()
		}
	})
}

// Helper functions
type AuthenticatedUser struct {
	ID           uuid.UUID
	Email        string
	Token        string
	RefreshToken string
	Type         string
}

func createTestUser(t *testing.T, client *infrastructure.APIClient, email, userType string) {
	registerReq := map[string]interface{}{
		"email":    email,
		"password": "TestPass123!",
		"name":     "Test User",
		"type":     userType,
	}
	
	resp := client.Post("/api/v1/auth/register", registerReq)
	require.Equal(t, 201, resp.StatusCode)
	resp.Body.Close()
}

func createAuthenticatedUser(t *testing.T, client *infrastructure.APIClient, email, userType string) *AuthenticatedUser {
	// Register user
	createTestUser(t, client, email, userType)
	
	// Login to get tokens
	loginReq := map[string]interface{}{
		"email":    email,
		"password": "TestPass123!",
	}
	
	resp := client.Post("/api/v1/auth/login", loginReq)
	require.Equal(t, 200, resp.StatusCode)
	
	var loginResp map[string]interface{}
	client.DecodeResponse(resp, &loginResp)
	
	return &AuthenticatedUser{
		Email:        email,
		Token:        loginResp["token"].(string),
		RefreshToken: loginResp["refresh_token"].(string),
		Type:         userType,
	}
}
