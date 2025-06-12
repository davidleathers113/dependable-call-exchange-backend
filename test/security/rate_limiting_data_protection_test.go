//go:build security

package security

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/test/e2e/infrastructure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSecurity_RateLimiting(t *testing.T) {
	env := infrastructure.NewTestEnvironment(t)
	client := infrastructure.NewAPIClient(t, env.APIURL)
	
	t.Run("API Rate Limiting", func(t *testing.T) {
		user := createTestUser(t, client, "ratelimit@test.com", "buyer")
		client.SetToken(user.Token)
		
		// Make many requests rapidly
		hitRateLimit := false
		rateLimitStatus := 0
		
		for i := 0; i < 200; i++ {
			resp := client.Get("/api/v1/calls")
			if resp.StatusCode == 429 {
				hitRateLimit = true
				rateLimitStatus = resp.StatusCode
				
				// Check rate limit headers
				assert.NotEmpty(t, resp.Header.Get("X-RateLimit-Limit"))
				assert.NotEmpty(t, resp.Header.Get("X-RateLimit-Remaining"))
				assert.NotEmpty(t, resp.Header.Get("X-RateLimit-Reset"))
				
				// Check retry-after header
				retryAfter := resp.Header.Get("Retry-After")
				assert.NotEmpty(t, retryAfter, "Should include Retry-After header")
				
				break
			}
			resp.Body.Close()
		}
		
		assert.True(t, hitRateLimit, "Rate limiting should be enforced")
		assert.Equal(t, 429, rateLimitStatus, "Should return 429 Too Many Requests")
	})
	
	t.Run("Per-Endpoint Rate Limiting", func(t *testing.T) {
		user := createTestUser(t, client, "endpoint-limit@test.com", "buyer")
		client.SetToken(user.Token)
		
		// Different endpoints should have different rate limits
		endpoints := []struct {
			path          string
			method        string
			body          interface{}
			expectedLimit int // requests before rate limit
		}{
			{
				path:          "/api/v1/auth/login",
				method:        "POST",
				body:          map[string]interface{}{"email": "test@test.com", "password": "wrong"},
				expectedLimit: 5, // Strict limit for auth endpoints
			},
			{
				path:          "/api/v1/calls",
				method:        "GET",
				expectedLimit: 100, // Higher limit for read operations
			},
			{
				path:          "/api/v1/calls",
				method:        "POST",
				body:          map[string]interface{}{"from_number": "+14155551234", "to_number": "+18005551234"},
				expectedLimit: 50, // Moderate limit for write operations
			},
		}
		
		for _, endpoint := range endpoints {
			t.Run(endpoint.path, func(t *testing.T) {
				hitLimit := false
				requestCount := 0
				
				// Make requests until rate limited
				for i := 0; i < endpoint.expectedLimit*2; i++ {
					var resp *http.Response
					
					switch endpoint.method {
					case "GET":
						resp = client.Get(endpoint.path)
					case "POST":
						resp = client.Post(endpoint.path, endpoint.body)
					}
					
					requestCount++
					
					if resp.StatusCode == 429 {
						hitLimit = true
						resp.Body.Close()
						break
					}
					resp.Body.Close()
					
					// Small delay to avoid overwhelming the server
					time.Sleep(10 * time.Millisecond)
				}
				
				assert.True(t, hitLimit, "Should hit rate limit for %s", endpoint.path)
				t.Logf("Hit rate limit after %d requests for %s", requestCount, endpoint.path)
			})
		}
	})
	
	t.Run("Distributed Rate Limiting", func(t *testing.T) {
		// Test that rate limiting works across multiple client connections
		user := createTestUser(t, client, "distributed@test.com", "buyer")
		
		// Create multiple clients simulating different connections
		clients := make([]*infrastructure.APIClient, 10)
		for i := range clients {
			clients[i] = infrastructure.NewAPIClient(t, env.APIURL)
			clients[i].SetToken(user.Token)
		}
		
		// Make concurrent requests from all clients
		var totalRequests int32
		var rateLimitHits int32
		var wg sync.WaitGroup
		
		for _, c := range clients {
			wg.Add(1)
			go func(client *infrastructure.APIClient) {
				defer wg.Done()
				
				for i := 0; i < 50; i++ {
					atomic.AddInt32(&totalRequests, 1)
					resp := client.Get("/api/v1/calls")
					
					if resp.StatusCode == 429 {
						atomic.AddInt32(&rateLimitHits, 1)
					}
					resp.Body.Close()
					
					time.Sleep(5 * time.Millisecond)
				}
			}(c)
		}
		
		wg.Wait()
		
		// Should have hit rate limits across clients
		assert.Greater(t, rateLimitHits, int32(0), 
			"Distributed rate limiting should be enforced across multiple clients")
		t.Logf("Total requests: %d, Rate limit hits: %d", totalRequests, rateLimitHits)
	})
	
	t.Run("Rate Limit Recovery", func(t *testing.T) {
		user := createTestUser(t, client, "recovery@test.com", "buyer")
		client.SetToken(user.Token)
		
		// Hit rate limit
		for i := 0; i < 200; i++ {
			resp := client.Get("/api/v1/calls")
			if resp.StatusCode == 429 {
				// Get reset time
				resetHeader := resp.Header.Get("X-RateLimit-Reset")
				retryAfter := resp.Header.Get("Retry-After")
				resp.Body.Close()
				
				// Wait for the specified time
				waitTime, _ := time.ParseDuration(retryAfter + "s")
				if waitTime > 10*time.Second {
					waitTime = 10 * time.Second // Cap wait time for testing
				}
				
				t.Logf("Rate limited. Waiting %v before retry", waitTime)
				time.Sleep(waitTime)
				
				// Should be able to make requests again
				resp = client.Get("/api/v1/calls")
				assert.NotEqual(t, 429, resp.StatusCode, 
					"Should recover from rate limit after waiting")
				resp.Body.Close()
				
				t.Logf("Reset time was: %s", resetHeader)
				break
			}
			resp.Body.Close()
		}
	})
}

func TestSecurity_DataProtection(t *testing.T) {
	env := infrastructure.NewTestEnvironment(t)
	client := infrastructure.NewAPIClient(t, env.APIURL)
	
	t.Run("Sensitive Data Masking", func(t *testing.T) {
		admin := createTestUser(t, client, "admin-data@test.com", "admin")
		client.SetToken(admin.Token)
		
		// Create account with sensitive payment info
		resp := client.Post("/api/v1/accounts", map[string]interface{}{
			"email":        "sensitive@example.com",
			"company_name": "Test Corp",
			"type":         "buyer",
			"password":     "SecurePass123!",
			"payment_info": map[string]interface{}{
				"card_number": "4111111111111111",
				"cvv":         "123",
				"expiry":      "12/25",
			},
			"ssn":    "123-45-6789",
			"api_key": "sk_test_abcdef123456",
		})
		
		require.Equal(t, 201, resp.StatusCode)
		
		var createResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&createResp)
		
		// Check that sensitive data is masked in response
		respStr := fmt.Sprintf("%v", createResp)
		
		// Card number should be masked
		assert.NotContains(t, respStr, "4111111111111111", "Full card number should not be exposed")
		assert.Contains(t, respStr, "****1111", "Card number should be masked")
		
		// CVV should never be returned
		assert.NotContains(t, respStr, "123", "CVV should never be returned")
		
		// SSN should be masked
		assert.NotContains(t, respStr, "123-45-6789", "Full SSN should not be exposed")
		
		// API keys should be masked
		assert.NotContains(t, respStr, "sk_test_abcdef123456", "Full API key should not be exposed")
		
		// Password should never be returned
		assert.NotContains(t, respStr, "SecurePass123!", "Password should never be returned")
	})
	
	t.Run("PII Data Access Control", func(t *testing.T) {
		// Create accounts
		buyer := createTestUser(t, client, "buyer-pii@test.com", "buyer")
		seller := createTestUser(t, client, "seller-pii@test.com", "seller")
		admin := createTestUser(t, client, "admin-pii@test.com", "admin")
		
		// Admin creates a call with PII
		client.SetToken(admin.Token)
		resp := client.Post("/api/v1/calls", map[string]interface{}{
			"from_number": "+14155551234",
			"to_number":   "+18005551234",
			"caller_info": map[string]interface{}{
				"name":    "John Doe",
				"address": "123 Main St, Anytown, USA",
				"email":   "john.doe@example.com",
			},
		})
		require.Equal(t, 201, resp.StatusCode)
		
		var call map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&call)
		callID := call["id"].(string)
		
		// Test PII access for different roles
		tests := []struct {
			user     *AuthenticatedUser
			role     string
			canSeePII bool
		}{
			{buyer, "buyer", false},
			{seller, "seller", false},
			{admin, "admin", true},
		}
		
		for _, tt := range tests {
			t.Run(tt.role+"_access", func(t *testing.T) {
				client.SetToken(tt.user.Token)
				resp := client.Get("/api/v1/calls/" + callID)
				
				if resp.StatusCode == 200 {
					var callData map[string]interface{}
					json.NewDecoder(resp.Body).Decode(&callData)
					
					if tt.canSeePII {
						// Admin should see PII
						assert.NotNil(t, callData["caller_info"], 
							"%s should see caller info", tt.role)
					} else {
						// Non-admin should not see PII
						if callerInfo, exists := callData["caller_info"]; exists {
							// If included, should be redacted
							info := callerInfo.(map[string]interface{})
							assert.Equal(t, "[REDACTED]", info["name"])
							assert.Equal(t, "[REDACTED]", info["address"])
							assert.Equal(t, "[REDACTED]", info["email"])
						}
					}
				}
			})
		}
	})
}
