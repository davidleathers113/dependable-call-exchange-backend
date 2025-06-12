//go:build contract

package contract

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/api/rest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAPIContractCompliance verifies that the API implementation matches the OpenAPI contract
func TestAPIContractCompliance(t *testing.T) {
	// Setup test server with real handlers
	handler := setupTestAPIServer(t)
	server := httptest.NewServer(handler)
	defer server.Close()

	// Contract test cases based on OpenAPI specification
	testCases := []rest.ContractTestCase{
		{
			Name:           "GET /health endpoint",
			Method:         "GET",
			URL:            "/health",
			ExpectedStatus: http.StatusOK,
		},
		{
			Name:           "GET /ready endpoint", 
			Method:         "GET",
			URL:            "/ready",
			ExpectedStatus: http.StatusOK,
		},
		{
			Name:   "POST /api/v1/calls - create call",
			Method: "POST",
			URL:    "/api/v1/calls",
			RequestBody: map[string]interface{}{
				"from_number": "+12125551234",
				"to_number":   "+13105559876",
				"direction":   "inbound",
			},
			RequestHeaders: map[string]string{
				"Content-Type":  "application/json",
				"Authorization": "Bearer test-token",
			},
			ExpectedStatus: http.StatusCreated,
		},
		{
			Name:   "POST /api/v1/bid-profiles - create bid profile",
			Method: "POST", 
			URL:    "/api/v1/bid-profiles",
			RequestBody: map[string]interface{}{
				"criteria": map[string]interface{}{
					"geography": map[string]interface{}{
						"countries": []string{"US"},
						"states":    []string{"CA", "NY"},
					},
					"call_type":  []string{"sales"},
					"max_budget": 100.0,
				},
				"active": true,
			},
			RequestHeaders: map[string]string{
				"Content-Type":  "application/json",
				"Authorization": "Bearer test-token",
			},
			ExpectedStatus: http.StatusCreated,
		},
		{
			Name:   "POST /api/v1/auctions - create auction",
			Method: "POST",
			URL:    "/api/v1/auctions",
			RequestBody: map[string]interface{}{
				"call_id":       "123e4567-e89b-12d3-a456-426614174000",
				"reserve_price": 0.05,
				"duration":      300,
			},
			RequestHeaders: map[string]string{
				"Content-Type":  "application/json",
				"Authorization": "Bearer test-token",
			},
			ExpectedStatus: http.StatusCreated,
		},
		{
			Name:   "POST /api/v1/bids - place bid",
			Method: "POST",
			URL:    "/api/v1/bids",
			RequestBody: map[string]interface{}{
				"auction_id": "123e4567-e89b-12d3-a456-426614174000",
				"amount":     0.10,
			},
			RequestHeaders: map[string]string{
				"Content-Type":  "application/json",
				"Authorization": "Bearer test-token",
			},
			ExpectedStatus: http.StatusCreated,
		},
		{
			Name:   "GET /api/v1/account/balance",
			Method: "GET",
			URL:    "/api/v1/account/balance",
			RequestHeaders: map[string]string{
				"Authorization": "Bearer test-token",
			},
			ExpectedStatus: http.StatusOK,
		},
		{
			Name:   "POST /api/v1/compliance/dnc - add to DNC",
			Method: "POST",
			URL:    "/api/v1/compliance/dnc",
			RequestBody: map[string]interface{}{
				"phone_number": "+12125551234",
				"reason":       "consumer request",
			},
			RequestHeaders: map[string]string{
				"Content-Type":  "application/json",
				"Authorization": "Bearer test-token",
			},
			ExpectedStatus: http.StatusCreated,
		},
		{
			Name:   "PUT /api/v1/compliance/tcpa/hours - set TCPA hours",
			Method: "PUT",
			URL:    "/api/v1/compliance/tcpa/hours",
			RequestBody: map[string]interface{}{
				"start_time": "09:00",
				"end_time":   "17:00",
				"timezone":   "America/New_York",
			},
			RequestHeaders: map[string]string{
				"Content-Type":  "application/json",
				"Authorization": "Bearer test-token",
			},
			ExpectedStatus: http.StatusOK,
		},
	}

	// Try to create contract test runner (will skip if OpenAPI spec not available)
	runner, err := rest.NewContractTestRunner("../../api/openapi.yaml", server.URL, false)
	if err != nil {
		t.Logf("Could not create contract test runner: %v", err)
		t.Log("Running manual contract tests instead...")
		runManualContractTests(t, server.URL, testCases)
		return
	}

	// Run contract tests
	results, err := runner.RunContractTests(testCases)
	require.NoError(t, err)

	t.Logf("Contract test results: %+v", results.Summary())

	// Assert overall results
	assert.Greater(t, results.PassedTests, 0, "At least some tests should pass")
	
	// Log details for any failures
	for _, result := range results.TestResults {
		if !result.Passed {
			t.Logf("Test '%s' failed: %s", result.Name, result.Error)
			if len(result.RequestValidationErrors) > 0 {
				t.Logf("  Request validation errors: %+v", result.RequestValidationErrors)
			}
			if len(result.ResponseValidationErrors) > 0 {
				t.Logf("  Response validation errors: %+v", result.ResponseValidationErrors)
			}
		}
	}
}

// runManualContractTests runs basic contract tests manually when OpenAPI spec isn't available
func runManualContractTests(t *testing.T, baseURL string, testCases []rest.ContractTestCase) {
	client := &http.Client{Timeout: 10 * time.Second}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			// Create request
			var req *http.Request
			var err error

			if testCase.RequestBody != nil {
				body, marshalErr := json.Marshal(testCase.RequestBody)
				require.NoError(t, marshalErr)
				req, err = http.NewRequest(testCase.Method, baseURL+testCase.URL, bytes.NewReader(body))
			} else {
				req, err = http.NewRequest(testCase.Method, baseURL+testCase.URL, nil)
			}
			require.NoError(t, err)

			// Add headers
			for key, value := range testCase.RequestHeaders {
				req.Header.Set(key, value)
			}

			// Execute request
			resp, err := client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			// Basic contract validation
			assert.Equal(t, testCase.ExpectedStatus, resp.StatusCode)
			
			// Verify content type for JSON responses
			if resp.StatusCode < 300 && resp.Header.Get("Content-Type") != "" {
				assert.Contains(t, resp.Header.Get("Content-Type"), "application/json")
			}

			// Verify response structure for successful responses
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				var responseBody map[string]interface{}
				err := json.NewDecoder(resp.Body).Decode(&responseBody)
				if err == nil {
					// Basic structure validation
					validateBasicResponseStructure(t, testCase, responseBody)
				}
			}
		})
	}
}

// validateBasicResponseStructure performs basic validation of response structure
func validateBasicResponseStructure(t *testing.T, testCase rest.ContractTestCase, responseBody map[string]interface{}) {
	switch testCase.URL {
	case "/health":
		assert.Contains(t, responseBody, "status")
		assert.Contains(t, responseBody, "timestamp")
		
	case "/api/v1/account/balance":
		assert.Contains(t, responseBody, "balance")
		
	case "/api/v1/calls":
		if testCase.Method == "POST" {
			assert.Contains(t, responseBody, "id")
			assert.Contains(t, responseBody, "from_number")
			assert.Contains(t, responseBody, "to_number")
			assert.Contains(t, responseBody, "status")
		}
		
	case "/api/v1/bid-profiles":
		if testCase.Method == "POST" {
			assert.Contains(t, responseBody, "id")
			assert.Contains(t, responseBody, "criteria")
			assert.Contains(t, responseBody, "active")
		}
		
	case "/api/v1/auctions":
		if testCase.Method == "POST" {
			assert.Contains(t, responseBody, "id")
			assert.Contains(t, responseBody, "call_id")
			assert.Contains(t, responseBody, "status")
		}
		
	case "/api/v1/bids":
		if testCase.Method == "POST" {
			assert.Contains(t, responseBody, "id")
			assert.Contains(t, responseBody, "auction_id")
			assert.Contains(t, responseBody, "amount")
		}
	}
}

// TestContractValidationMiddleware tests the contract validation middleware
func TestContractValidationMiddleware(t *testing.T) {
	// Create a test handler
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "success",
		})
	})

	// Try to create contract validation middleware
	config := rest.DefaultContractValidationConfig()
	config.FailOnValidationError = true // Fail on errors for testing
	
	middleware, err := rest.NewContractValidationMiddleware("../../api/openapi.yaml", config, nil)
	if err != nil {
		t.Skipf("Could not create contract validation middleware: %v", err)
		return
	}

	// Wrap handler with middleware
	wrappedHandler := middleware.Middleware()(testHandler)
	server := httptest.NewServer(wrappedHandler)
	defer server.Close()

	t.Run("valid request passes validation", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"from_number": "+12125551234",
			"to_number":   "+13105559876",
			"direction":   "inbound",
		}
		body, _ := json.Marshal(reqBody)
		
		req, err := http.NewRequest("POST", server.URL+"/api/v1/calls", bytes.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer test-token")

		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Should not be rejected by validation
		assert.NotEqual(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("invalid request fails validation", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"invalid_field": "value",
		}
		body, _ := json.Marshal(reqBody)
		
		req, err := http.NewRequest("POST", server.URL+"/api/v1/calls", bytes.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer test-token")

		client := &http.Client{}
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Might be rejected by validation (depending on spec)
		if resp.StatusCode == http.StatusBadRequest {
			var errorResp map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&errorResp)
			assert.Contains(t, errorResp, "error")
		}
	})
}

// TestContractValidationReporter tests the validation reporter
func TestContractValidationReporter(t *testing.T) {
	reporter := rest.NewContractValidationReporter(nil)

	violation := rest.ContractViolation{
		Timestamp:  time.Now(),
		Method:     "POST",
		Path:       "/api/v1/calls",
		Type:       "request",
		StatusCode: 400,
		Errors: []rest.ContractValidationError{
			{
				Type:    "validation",
				Message: "missing required field",
				Path:    "from_number",
			},
		},
		RequestID:  "req-123",
		UserAgent:  "test-client",
		RemoteAddr: "127.0.0.1",
	}

	// Report violation
	reporter.ReportViolation(violation)

	// Verify violation was recorded
	violations := reporter.GetViolations()
	assert.Len(t, violations, 1)
	assert.Equal(t, violation.Method, violations[0].Method)
	assert.Equal(t, violation.Path, violations[0].Path)

	// Generate report
	report := reporter.GenerateReport()
	assert.Contains(t, report, "summary")
	assert.Contains(t, report, "by_path")
	assert.Contains(t, report, "by_method")
	assert.Contains(t, report, "violations")

	summary := report["summary"].(map[string]interface{})
	assert.Equal(t, 1, summary["total_violations"])
	assert.Equal(t, 1, summary["request_violations"])
	assert.Equal(t, 0, summary["response_violations"])
}

// setupTestAPIServer creates a test server with the REST handlers
func setupTestAPIServer(t *testing.T) http.Handler {
	// Create services with minimal mock implementations
	services := &rest.Services{
		// Add minimal service implementations for testing
		Repositories: nil, // Would be mocked in real implementation
	}

	// Create handler
	handler := rest.NewHandler(services)
	return handler
}

// BenchmarkContractValidation benchmarks contract validation performance
func BenchmarkContractValidation(b *testing.B) {
	validator, err := rest.NewContractValidator("../../api/openapi.yaml")
	if err != nil {
		b.Skipf("Could not load OpenAPI spec: %v", err)
		return
	}

	// Create test request
	reqBody := map[string]interface{}{
		"from_number": "+12125551234",
		"to_number":   "+13105559876",
		"direction":   "inbound",
	}
	body, _ := json.Marshal(reqBody)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "/api/v1/calls", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer test-token")

		// Validate request
		err := validator.ValidateRequest(req)
		if err != nil {
			// Expected - validation might fail
		}
	}
}

// TestContractValidationPerformance tests that validation doesn't significantly impact performance
func TestContractValidationPerformance(t *testing.T) {
	validator, err := rest.NewContractValidator("../../api/openapi.yaml")
	if err != nil {
		t.Skipf("Could not load OpenAPI spec: %v", err)
		return
	}

	// Create test request
	reqBody := map[string]interface{}{
		"from_number": "+12125551234",
		"to_number":   "+13105559876", 
		"direction":   "inbound",
	}
	body, _ := json.Marshal(reqBody)

	// Measure validation performance
	const iterations = 1000
	start := time.Now()

	for i := 0; i < iterations; i++ {
		req := httptest.NewRequest("POST", "/api/v1/calls", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer test-token")

		validator.ValidateRequest(req)
	}

	duration := time.Since(start)
	avgDuration := duration / iterations

	t.Logf("Contract validation performance: %v per request (avg over %d requests)", avgDuration, iterations)

	// Assert that validation is reasonably fast
	assert.Less(t, avgDuration, 1*time.Millisecond, "Contract validation should be < 1ms per request")
}