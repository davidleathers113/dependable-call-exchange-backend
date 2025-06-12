//go:build security

package security

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/davidleathers/dependable-call-exchange-backend/test/e2e/infrastructure"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// SecurityTestSuite runs all security tests
type SecurityTestSuite struct {
	suite.Suite
	env    *infrastructure.TestEnvironment
	client *infrastructure.APIClient
}

func (s *SecurityTestSuite) SetupSuite() {
	// Initialize test environment
	s.env = infrastructure.NewTestEnvironment(s.T())
	s.client = infrastructure.NewAPIClient(s.T(), s.env.APIURL)
}

func (s *SecurityTestSuite) TearDownSuite() {
	// Cleanup is handled by TestEnvironment
}

func (s *SecurityTestSuite) SetupTest() {
	// Reset database before each test
	s.env.ResetDatabase()
}

// TestAuthentication runs all authentication tests
func (s *SecurityTestSuite) TestAuthentication() {
	s.Run("JWT_Token_Validation", func() {
		TestSecurity_Authentication(s.T())
	})
}

// TestInputValidation runs all input validation tests
func (s *SecurityTestSuite) TestInputValidation() {
	s.Run("SQL_Injection_Prevention", func() {
		TestSecurity_InputValidation(s.T())
	})
}

// TestRateLimiting runs all rate limiting tests
func (s *SecurityTestSuite) TestRateLimiting() {
	s.Run("API_Rate_Limiting", func() {
		TestSecurity_RateLimiting(s.T())
	})
}

// TestDataProtection runs all data protection tests
func (s *SecurityTestSuite) TestDataProtection() {
	s.Run("Sensitive_Data_Masking", func() {
		TestSecurity_DataProtection(s.T())
	})
}

// TestSecurityHeaders verifies security headers
func (s *SecurityTestSuite) TestSecurityHeaders() {
	user := createTestUser(s.T(), s.client, "headers@test.com", "buyer")
	s.client.SetToken(user.Token)
	
	resp := s.client.Get("/api/v1/profile")
	defer resp.Body.Close()
	
	// Check security headers
	headers := map[string]string{
		"X-Content-Type-Options":            "nosniff",
		"X-Frame-Options":                   "DENY",
		"X-XSS-Protection":                  "1; mode=block",
		"Strict-Transport-Security":         "max-age=31536000; includeSubDomains",
		"Content-Security-Policy":           "default-src 'self'",
		"Referrer-Policy":                   "strict-origin-when-cross-origin",
		"Permissions-Policy":                "geolocation=(), microphone=(), camera=()",
	}
	
	for header, expectedValue := range headers {
		actual := resp.Header.Get(header)
		s.Assert().NotEmpty(actual, "Missing security header: %s", header)
		if expectedValue != "" {
			s.Assert().Contains(actual, expectedValue, 
				"Invalid value for header %s: got %s, expected %s", 
				header, actual, expectedValue)
		}
	}
}

// TestCORS verifies CORS configuration
func (s *SecurityTestSuite) TestCORS() {
	// Test preflight request
	req := NewRequest("OPTIONS", s.env.APIURL+"/api/v1/calls", nil)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "Authorization, Content-Type")
	
	resp, err := http.DefaultClient.Do(req)
	require.NoError(s.T(), err)
	defer resp.Body.Close()
	
	// Check CORS headers
	s.Assert().Equal(200, resp.StatusCode, "Preflight request should succeed")
	
	// Verify CORS headers based on configuration
	allowedOrigin := resp.Header.Get("Access-Control-Allow-Origin")
	if allowedOrigin == "*" {
		s.T().Log("Warning: CORS allows all origins. Consider restricting in production.")
	} else {
		s.Assert().NotEmpty(allowedOrigin, "Should have Access-Control-Allow-Origin header")
	}
	
	s.Assert().Contains(resp.Header.Get("Access-Control-Allow-Methods"), "POST")
	s.Assert().Contains(resp.Header.Get("Access-Control-Allow-Headers"), "Authorization")
}

// TestSecurityMisconfiguration checks for common misconfigurations
func (s *SecurityTestSuite) TestSecurityMisconfiguration() {
	s.Run("Debug_Mode_Disabled", func() {
		// Attempt to access debug endpoints
		debugEndpoints := []string{
			"/debug/pprof/",
			"/debug/vars",
			"/metrics",
			"/.env",
			"/config",
		}
		
		for _, endpoint := range debugEndpoints {
			resp := s.client.Get(endpoint)
			s.Assert().Contains([]int{401, 403, 404}, resp.StatusCode,
				"Debug endpoint %s should not be accessible", endpoint)
			resp.Body.Close()
		}
	})
	
	s.Run("Error_Messages_Sanitized", func() {
		// Trigger various errors and check messages don't expose internals
		resp := s.client.Post("/api/v1/calls", map[string]interface{}{
			"invalid_field": "test",
		})
		
		var errResp map[string]interface{}
		err := json.NewDecoder(resp.Body).Decode(&errResp)
		require.NoError(s.T(), err)
		
		errorMsg := extractErrorMessage(errResp)
		
		// Should not contain:
		s.Assert().NotContains(errorMsg, "database")
		s.Assert().NotContains(errorMsg, "SQL")
		s.Assert().NotContains(errorMsg, "postgres")
		s.Assert().NotContains(errorMsg, "redis")
		s.Assert().NotContains(errorMsg, "panic")
		s.Assert().NotContains(errorMsg, "stack trace")
	})
}

// Run the security test suite
func TestSecuritySuite(t *testing.T) {
	suite.Run(t, new(SecurityTestSuite))
}

// Standalone test runner for specific security tests
func TestSecurity_All(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping security tests in short mode")
	}
	
	t.Run("Authentication", func(t *testing.T) {
		TestSecurity_Authentication(t)
	})
	
	t.Run("InputValidation", func(t *testing.T) {
		TestSecurity_InputValidation(t)
	})
	
	t.Run("RateLimiting", func(t *testing.T) {
		TestSecurity_RateLimiting(t)
	})
	
	t.Run("DataProtection", func(t *testing.T) {
		TestSecurity_DataProtection(t)
	})
}
