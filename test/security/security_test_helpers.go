//go:build security

package security

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/test/e2e/infrastructure"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// AuthenticatedUser represents a test user with authentication tokens
type AuthenticatedUser struct {
	ID           uuid.UUID
	Email        string
	Role         string
	Token        string
	RefreshToken string
}

// AuthResponse represents the authentication response
type AuthResponse struct {
	Token        string    `json:"token"`
	RefreshToken string    `json:"refresh_token"`
	UserID       uuid.UUID `json:"user_id"`
	Email        string    `json:"email"`
	Role         string    `json:"role"`
}

// createTestUser creates and authenticates a test user
func createTestUser(t *testing.T, client *infrastructure.APIClient, email, role string) *AuthenticatedUser {
	// Register user
	registerReq := map[string]interface{}{
		"email":        email,
		"password":     "TestPass123!",
		"company_name": "Test Company",
		"type":         role,
	}

	resp := client.Post("/api/v1/auth/register", registerReq)
	require.Equal(t, 201, resp.StatusCode, "Failed to register user")

	var registerResp map[string]interface{}
	err := json.NewDecoder(resp.Body).Decode(&registerResp)
	require.NoError(t, err)
	resp.Body.Close()

	// Login to get tokens
	loginReq := map[string]interface{}{
		"email":    email,
		"password": "TestPass123!",
	}

	resp = client.Post("/api/v1/auth/login", loginReq)
	require.Equal(t, 200, resp.StatusCode, "Failed to login user")

	var authResp AuthResponse
	err = json.NewDecoder(resp.Body).Decode(&authResp)
	require.NoError(t, err)
	resp.Body.Close()

	return &AuthenticatedUser{
		ID:           authResp.UserID,
		Email:        email,
		Role:         role,
		Token:        authResp.Token,
		RefreshToken: authResp.RefreshToken,
	}
}

// authenticateAccount authenticates an existing account
func authenticateAccount(t *testing.T, client *infrastructure.APIClient, email string) *AuthenticatedUser {
	loginReq := map[string]interface{}{
		"email":    email,
		"password": "TestPass123!",
	}

	resp := client.Post("/api/v1/auth/login", loginReq)
	require.Equal(t, 200, resp.StatusCode, "Failed to login user")

	var authResp AuthResponse
	err := json.NewDecoder(resp.Body).Decode(&authResp)
	require.NoError(t, err)
	resp.Body.Close()

	return &AuthenticatedUser{
		ID:           authResp.UserID,
		Email:        email,
		Role:         authResp.Role,
		Token:        authResp.Token,
		RefreshToken: authResp.RefreshToken,
	}
}

// extractErrorMessage extracts error message from various response formats
func extractErrorMessage(errResp map[string]interface{}) string {
	// Try different common error message fields
	if msg, ok := errResp["error"].(string); ok {
		return msg
	}
	if msg, ok := errResp["message"].(string); ok {
		return msg
	}
	if err, ok := errResp["error"].(map[string]interface{}); ok {
		if msg, ok := err["message"].(string); ok {
			return msg
		}
	}
	return ""
}

// contains checks if a string slice contains a value
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// generateValidToken generates a valid JWT token for testing
func generateValidToken(t *testing.T, secret string, userID uuid.UUID, role string) string {
	claims := jwt.MapClaims{
		"sub":  userID.String(),
		"role": role,
		"exp":  time.Now().Add(time.Hour).Unix(),
		"iat":  time.Now().Unix(),
		"iss":  "dce-backend",
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(secret))
	require.NoError(t, err)

	return signedToken
}

// NewRequest creates a new HTTP request with common headers
func NewRequest(method, path string, body interface{}) *http.Request {
	var bodyReader *bytes.Reader
	if body != nil {
		bodyBytes, _ := json.Marshal(body)
		bodyReader = bytes.NewReader(bodyBytes)
	} else {
		bodyReader = bytes.NewReader([]byte{})
	}

	req, _ := http.NewRequest(method, path, bodyReader)
	req.Header.Set("Content-Type", "application/json")
	return req
}

// Helper function to create HTTP requests
func createHTTPRequest(method, path string, body interface{}) *http.Request {
	var bodyReader *bytes.Reader
	if body != nil {
		bodyBytes, _ := json.Marshal(body)
		bodyReader = bytes.NewReader(bodyBytes)
	} else {
		bodyReader = bytes.NewReader([]byte{})
	}

	req, _ := http.NewRequest(method, path, bodyReader)
	req.Header.Set("Content-Type", "application/json")
	return req
}
