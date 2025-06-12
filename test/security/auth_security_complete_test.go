//go:build security

package security

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

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
				token:          "Bearer " + generateExpiredToken(t, ""),
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
				token:          "Bearer " + generateTokenWithInvalidIssuer(t, ""),
				expectedStatus: 401,
				expectedError:  "invalid token issuer",
			},
			{
				name:           "missing required claims",
				token:          "Bearer " + generateTokenMissingClaims(t, ""),
				expectedStatus: 401,
				expectedError:  "invalid token claims",
			},
			{
				name:           "valid token",
				token:          "Bearer " + user.Token,
				expectedStatus: 200,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				req := NewRequest("GET", env.APIURL+"/api/v1/profile", nil)
				if tt.token != "" {
					req.Header.Set("Authorization", tt.token)
				}

				resp, err := http.DefaultClient.Do(req)
				require.NoError(t, err)
				defer resp.Body.Close()

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
				method: "POST",
				path:   "/api/v1/bid-profiles",
				body: map[string]interface{}{
					"criteria": map[string]interface{}{
						"geography": map[string]interface{}{
							"countries": []string{"US"},
						},
						"max_budget": 100.00,
					},
					"active": true,
				},
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
					"direction":   "inbound",
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
					} else {
						// Should return 403 Forbidden
						assert.Equal(t, 403, resp.StatusCode,
							"%s should NOT have access to %s %s", role, endpoint.method, endpoint.path)
					}
				})
			}
		}
	})
}
