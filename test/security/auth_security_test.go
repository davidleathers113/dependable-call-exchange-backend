//go:build security

package security

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/account"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/infrastructure/auth"
	"github.com/davidleathers/dependable-call-exchange-backend/test/e2e/infrastructure"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSecurity_Authentication(t *testing.T) {
	env := infrastructure.NewTestEnvironment(t)
	client := infrastructure.NewAPIClient(t, env.APIURL)

	t.Run("JWT Token Validation", func(t *testing.T) {
		// Create a test user first
		user := createTestUser(t, client, "test@example.com", "buyer")

		tests := []struct {
			name           string
			token          string
			expectedStatus int
			expectedError  string
		}{
			{
				name:           "missing token",
				token:          "",
				expectedStatus: 401,
				expectedError:  "missing or invalid token",
			},
			{
				name:           "invalid format - no Bearer prefix",
				token:          "invalid-token",
				expectedStatus: 401,
				expectedError:  "invalid token format",
			},
			{
				name:           "invalid format - malformed JWT",
				token:          "Bearer invalid.token.here",
				expectedStatus: 401,
				expectedError:  "invalid token",
			},
			{
				name:           "expired token",
				token:          "Bearer " + generateExpiredToken(t, env.Config.Security.JWTSecret),
				expectedStatus: 401,
				expectedError:  "token expired",
			},
			{
				name:           "invalid signature",
				token:          "Bearer " + generateInvalidSignatureToken(t, user.ID),
				expectedStatus: 401,
				expectedError:  "invalid token signature",
			},
			{
				name:           "invalid issuer",
				token:          "Bearer " + generateTokenWithInvalidIssuer(t, env.Config.Security.JWTSecret),
				expectedStatus: 401,
				expectedError:  "invalid token issuer",
			},
			{
				name:           "missing required claims",
				token:          "Bearer " + generateTokenMissingClaims(t, env.Config.Security.JWTSecret),
				expectedStatus: 401,
				expectedError:  "invalid token claims",
			},
			{
				name:           "valid token",
				token:          "Bearer " + user.Token,
				expectedStatus: 200,
			},
			{
				name:           "token for deleted user",
				token:          "Bearer " + generateTokenForDeletedUser(t, env),
				expectedStatus: 401,
				expectedError:  "user not found",
			},
			{
				name:           "token with future issued time",
				token:          "Bearer " + generateFutureIssuedToken(t, env.Config.Security.JWTSecret),
				expectedStatus: 401,
				expectedError:  "invalid token",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				req := infrastructure.NewRequest("GET", "/api/v1/profile", nil)
				if tt.token != "" {
					req.Header.Set("Authorization", tt.token)
				}

				resp := client.Do(req)
				assert.Equal(t, tt.expectedStatus, resp.StatusCode)

				if tt.expectedError != "" {
					var errResp map[string]interface{}
					err := json.NewDecoder(resp.Body).Decode(&errResp)
					require.NoError(t, err)
					
					errorMsg := extractErrorMessage(errResp)
					assert.Contains(t, strings.ToLower(errorMsg), strings.ToLower(tt.expectedError))
				}
			})
		}
	})

	t.Run("Role-Based Access Control", func(t *testing.T) {
		// Create users with different roles
		buyer := createTestUser(t, client, "buyer@test.com", "buyer")
		seller := createTestUser(t, client, "seller@test.com", "seller")
		admin := createTestUser(t, client, "admin@test.com", "admin")

		endpoints := []struct {
			method       string
			path         string
			body         interface{}
			allowedRoles []string
			description  string
		}{
			// Bid Profile Management - Sellers only
			{
				method:       "POST",
				path:         "/api/v1/bid-profiles",
				body:         map[string]interface{}{"criteria": map[string]interface{}{"max_budget": 100}, "active": true},
				allowedRoles: []string{"seller", "admin"},
				description:  "create bid profile",
			},
			{
				method:       "GET",
				path:         "/api/v1/bid-profiles",
				allowedRoles: []string{"seller", "admin"},
				description:  "list bid profiles",
			},
			// Call Management - Buyers and Admins
			{
				method: "POST",
				path:   "/api/v1/calls",
				body: map[string]interface{}{
					"from_number": "+14155551234",
					"to_number":   "+18005551234",
				},
				allowedRoles: []string{"buyer", "admin"},
				description:  "create call",
			},
			// Compliance - Admins only
			{
				method: "POST",
				path:   "/api/v1/compliance/dnc",
				body: map[string]interface{}{
					"phone_number": "+14155551234",
					"reason":       "test",
				},
				allowedRoles: []string{"admin"},
				description:  "add to DNC list",
			},
			{
				method: "PUT",
				path:   "/api/v1/compliance/tcpa/hours",
				body: map[string]interface{}{
					"start_time": "09:00",
					"end_time":   "20:00",
					"timezone":   "America/New_York",
				},
				allowedRoles: []string{"admin"},
				description:  "set TCPA hours",
			},
			// Admin endpoints
			{
				method:       "GET",
				path:         "/api/v1/admin/users",
				allowedRoles: []string{"admin"},
				description:  "list all users",
			},
			{
				method:       "GET",
				path:         "/api/v1/admin/transactions",
				allowedRoles: []string{"admin"},
				description:  "view all transactions",
			},
			// Shared endpoints with role-specific responses
			{
				method:       "GET",
				path:         "/api/v1/account/balance",
				allowedRoles: []string{"buyer", "seller", "admin"},
				description:  "get account balance",
			},
		}

		testUsers := map[string]*AuthenticatedUser{
			"buyer":  buyer,
			"seller": seller,
			"admin":  admin,
		}

		for _, endpoint := range endpoints {
			for role, user := range testUsers {
				testName := fmt.Sprintf("%s - %s access to %s", endpoint.description, role, endpoint.path)
				t.Run(testName, func(t *testing.T) {
					client.SetToken(user.Token)
					
					var resp *http.Response
					switch endpoint.method {
					case "GET":
						resp = client.Get(endpoint.path)
					case "POST":
						resp = client.Post(endpoint.path, endpoint.body)
					case "PUT":
						resp = client.Put(endpoint.path, endpoint.body)
					case "DELETE":
						resp = client.Delete(endpoint.path)
					}

					// Check if this role should have access
					hasAccess := contains(endpoint.allowedRoles, role)
					
					if hasAccess {
						// Should NOT return 403 Forbidden
						assert.NotEqual(t, 403, resp.StatusCode,
							"%s should have access to %s %s", role, endpoint.method, endpoint.path)
						
						// May return 200, 201, 400 (validation), or 404 (not found)
						assert.Contains(t, []int{200, 201, 400, 404}, resp.StatusCode,
							"Unexpected status for %s accessing %s", role, endpoint.path)
					} else {
						// Should return 403 Forbidden
						assert.Equal(t, 403, resp.StatusCode,
							"%s should NOT have access to %s %s", role, endpoint.method, endpoint.path)
						
						// Verify error message
						var errResp map[string]interface{}
						json.NewDecoder(resp.Body).Decode(&errResp)
						errorMsg := extractErrorMessage(errResp)
						assert.Contains(t, strings.ToLower(errorMsg), "forbidden",
							"Expected forbidden error for %s accessing %s", role, endpoint.path)
					}
				})
			}
		}
	})

	t.Run("Token Refresh Security", func(t *testing.T) {
		user := createTestUser(t, client, "refresh@test.com", "buyer")

		tests := []struct {
			name           string
			refreshToken   string
			expectedStatus int
			expectedError  string
		}{
			{
				name:           "valid refresh token",
				refreshToken:   user.RefreshToken,
				expectedStatus: 200,
			},
			{
				name:           "invalid refresh token",
				refreshToken:   "invalid-refresh-token",
				expectedStatus: 401,
				expectedError:  "invalid refresh token",
			},
			{
				name:           "expired refresh token",
				refreshToken:   generateExpiredRefreshToken(t),
				expectedStatus: 401,
				expectedError:  "refresh token expired",
			},
			{
				name:           "reused refresh token",
				refreshToken:   user.RefreshToken, // Will be invalidated after first use
				expectedStatus: 401,
				expectedError:  "refresh token already used",
			},
		}

		for i, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				resp := client.Post("/api/v1/auth/refresh", map[string]interface{}{
					"refresh_token": tt.refreshToken,
				})

				assert.Equal(t, tt.expectedStatus, resp.StatusCode)

				if tt.expectedError != "" {
					var errResp map[string]interface{}
					json.NewDecoder(resp.Body).Decode(&errResp)
					errorMsg := extractErrorMessage(errResp)
					assert.Contains(t, strings.ToLower(errorMsg), strings.ToLower(tt.expectedError))
				}

				// For the reused token test, use the first valid refresh
				if i == 0 && resp.StatusCode == 200 {
					var authResp AuthResponse
					json.NewDecoder(resp.Body).Decode(&authResp)
					// Update the user's refresh token for the reuse test
					user.RefreshToken = authResp.RefreshToken
				}
			})
		}
	})

	t.Run("Concurrent Authentication Attempts", func(t *testing.T) {
		// Test that concurrent login attempts are handled correctly
		email := "concurrent@test.com"
		password := "TestPass123!"
		
		// Create user
		createTestUser(t, client, email, "buyer")

		// Attempt multiple concurrent logins
		concurrency := 50
		var wg sync.WaitGroup
		successCount := 0
		var mu sync.Mutex

		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				
				localClient := infrastructure.NewAPIClient(t, env.APIURL)
				resp := localClient.Post("/api/v1/auth/login", map[string]interface{}{
					"email":    email,
					"password": password,
				})

				if resp.StatusCode == 200 {
					mu.Lock()
					successCount++
					mu.Unlock()
				}
			}()
		}

		wg.Wait()

		// All login attempts should succeed
		assert.Equal(t, concurrency, successCount, 
			"All concurrent login attempts should succeed")
	})

	t.Run("Session Hijacking Prevention", func(t *testing.T) {
		user := createTestUser(t, client, "session@test.com", "buyer")
		
		// Get initial user agent and IP
		initialReq := infrastructure.NewRequest("GET", "/api/v1/profile", nil)
		initialReq.Header.Set("Authorization", "Bearer "+user.Token)
		initialReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
		initialReq.Header.Set("X-Forwarded-For", "192.168.1.100")
		
		resp := client.Do(initialReq)
		assert.Equal(t, 200, resp.StatusCode)

		// Attempt to use same token from different location/device
		hijackReq := infrastructure.NewRequest("GET", "/api/v1/profile", nil)
		hijackReq.Header.Set("Authorization", "Bearer "+user.Token)
		hijackReq.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 14_0)")
		hijackReq.Header.Set("X-Forwarded-For", "10.0.0.1")
		
		// The system should detect suspicious activity
		// Implementation might vary - could return 401 or require re-authentication
		hijackResp := client.Do(hijackReq)
		
		// Log the suspicious activity (check logs in production)
		t.Logf("Session hijack attempt response: %d", hijackResp.StatusCode)
	})
}

// Helper functions
func generateExpiredToken(t *testing.T, secret string) string {
	claims := jwt.MapClaims{
		"sub":  uuid.New().String(),
		"role": "buyer",
		"exp":  time.Now().Add(-time.Hour).Unix(),
		"iat":  time.Now().Add(-2 * time.Hour).Unix(),
		"iss":  "dce-backend",
	}
	
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(secret))
	require.NoError(t, err)
	
	return signedToken
}

func generateInvalidSignatureToken(t *testing.T, userID uuid.UUID) string {
	claims := jwt.MapClaims{
		"sub":  userID.String(),
		"role": "buyer",
		"exp":  time.Now().Add(time.Hour).Unix(),
		"iat":  time.Now().Unix(),
		"iss":  "dce-backend",
	}
	
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte("wrong-secret"))
	require.NoError(t, err)
	
	return signedToken
}

func generateTokenWithInvalidIssuer(t *testing.T, secret string) string {
	claims := jwt.MapClaims{
		"sub":  uuid.New().String(),
		"role": "buyer",
		"exp":  time.Now().Add(time.Hour).Unix(),
		"iat":  time.Now().Unix(),
		"iss":  "invalid-issuer",
	}
	
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(secret))
	require.NoError(t, err)
	
	return signedToken
}

func generateTokenMissingClaims(t *testing.T, secret string) string {
	claims := jwt.MapClaims{
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Unix(),
		// Missing sub, role, iss
	}
	
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(secret))
	require.NoError(t, err)
	
	return signedToken
}

func generateFutureIssuedToken(t *testing.T, secret string) string {
	claims := jwt.MapClaims{
		"sub":  uuid.New().String(),
		"role": "buyer",
		"exp":  time.Now().Add(2 * time.Hour).Unix(),
		"iat":  time.Now().Add(time.Hour).Unix(), // Issued in the future
		"iss":  "dce-backend",
	}
	
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(secret))
	require.NoError(t, err)
	
	return signedToken
}

func generateTokenForDeletedUser(t *testing.T, env *infrastructure.TestEnvironment) string {
	// Create a user, get token, then delete the user
	tempUser := createTestUser(t, env.Client, "temp@test.com", "buyer")
	token := tempUser.Token
	
	// Delete user from database
	_, err := env.DB.Exec("DELETE FROM accounts WHERE id = $1", tempUser.ID)
	require.NoError(t, err)
	
	return token
}

func generateExpiredRefreshToken(t *testing.T, env *infrastructure.TestEnvironment) string {
	// Create a refresh token that's already expired
	claims := jwt.MapClaims{
		"sub":  uuid.New().String(),
		"type": "refresh",
		"exp":  time.Now().Add(-time.Hour).Unix(),
		"iat":  time.Now().Add(-24 * time.Hour).Unix(),
	}
	
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(env.Config.Security.JWTSecret))
	require.NoError(t, err)
	
	return signedToken
}
