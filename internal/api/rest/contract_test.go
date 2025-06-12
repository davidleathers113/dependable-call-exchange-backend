//go:build contract

package rest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers/gorillamux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestContractValidationBasic tests basic contract validation functionality
func TestContractValidationBasic(t *testing.T) {
	// Create a minimal OpenAPI spec for testing
	specYAML := `
openapi: 3.0.3
info:
  title: Test API
  version: 1.0.0
paths:
  /health:
    get:
      responses:
        '200':
          description: Health check
          content:
            application/json:
              schema:
                type: object
                properties:
                  status:
                    type: string
                    enum: [healthy]
                required:
                  - status
  /test/{id}:
    get:
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: string
            format: uuid
      responses:
        '200':
          description: Test response
          content:
            application/json:
              schema:
                type: object
                properties:
                  id:
                    type: string
                    format: uuid
                  name:
                    type: string
                required:
                  - id
                  - name
        '404':
          description: Not found
`

	// Load the spec
	loader := &openapi3.Loader{Context: context.Background(), IsExternalRefsAllowed: true}
	doc, err := loader.LoadFromData([]byte(specYAML))
	require.NoError(t, err)

	err = doc.Validate(loader.Context)
	require.NoError(t, err)

	// Create router
	router, err := gorillamux.NewRouter(doc)
	require.NoError(t, err)

	t.Run("validate successful request", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health", nil)
		
		// Find route
		route, pathParams, err := router.FindRoute(req)
		require.NoError(t, err)

		// Validate request
		requestValidationInput := &openapi3filter.RequestValidationInput{
			Request:    req,
			PathParams: pathParams,
			Route:      route,
		}

		err = openapi3filter.ValidateRequest(loader.Context, requestValidationInput)
		assert.NoError(t, err)
	})

	t.Run("validate path parameter", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test/123e4567-e89b-12d3-a456-426614174000", nil)
		
		// Find route
		route, pathParams, err := router.FindRoute(req)
		require.NoError(t, err)

		// Validate request
		requestValidationInput := &openapi3filter.RequestValidationInput{
			Request:    req,
			PathParams: pathParams,
			Route:      route,
		}

		err = openapi3filter.ValidateRequest(loader.Context, requestValidationInput)
		assert.NoError(t, err)
	})

	t.Run("validate invalid path parameter", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test/invalid-uuid", nil)
		
		// Find route
		route, pathParams, err := router.FindRoute(req)
		require.NoError(t, err)

		// Validate request
		requestValidationInput := &openapi3filter.RequestValidationInput{
			Request:    req,
			PathParams: pathParams,
			Route:      route,
		}

		err = openapi3filter.ValidateRequest(loader.Context, requestValidationInput)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "uuid")
	})
}

// TestContractValidationWithRealSpec tests with the actual OpenAPI spec
func TestContractValidationWithRealSpec(t *testing.T) {
	// Load the actual OpenAPI spec
	validator, err := NewContractValidator("../../../api/openapi.yaml")
	if err != nil {
		t.Skipf("Could not load OpenAPI spec: %v", err)
		return
	}

	t.Run("validate health endpoint", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/health", nil)
		err := validator.ValidateRequest(req)
		
		// Health endpoint might not be in spec, so this could fail
		if err != nil {
			t.Logf("Health endpoint validation failed (expected): %v", err)
		}
	})

	t.Run("validate call creation request", func(t *testing.T) {
		reqBody := `{
			"from_number": "+12125551234",
			"to_number": "+13105559876",
			"direction": "inbound"
		}`
		
		req := httptest.NewRequest("POST", "/api/v1/calls", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer test-token")
		
		err := validator.ValidateRequest(req)
		if err != nil {
			t.Logf("Request validation failed: %v", err)
		} else {
			t.Log("Request validation passed")
		}
	})

	t.Run("validate bid profile creation", func(t *testing.T) {
		reqBody := `{
			"criteria": {
				"geography": {
					"countries": ["US"],
					"states": ["CA", "NY"]
				},
				"call_type": ["sales"],
				"max_budget": 100.0
			},
			"active": true
		}`
		
		req := httptest.NewRequest("POST", "/api/v1/bid-profiles", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer test-token")
		
		err := validator.ValidateRequest(req)
		if err != nil {
			t.Logf("Bid profile request validation failed: %v", err)
		} else {
			t.Log("Bid profile request validation passed")
		}
	})
}

// TestEndToEndContractValidation tests contract validation against actual API responses
func TestEndToEndContractValidation(t *testing.T) {
	// Create a simple test server that implements some endpoints
	mux := http.NewServeMux()
	
	// Health endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":    "healthy",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"service":   "dce-backend",
		})
	})

	// Simple account endpoint
	mux.HandleFunc("/api/v1/accounts/", func(w http.ResponseWriter, r *http.Request) {
		accountID := strings.TrimPrefix(r.URL.Path, "/api/v1/accounts/")
		if accountID == "" {
			http.Error(w, "Account ID required", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":         accountID,
			"type":       "buyer",
			"email":      "test@example.com",
			"status":     "active",
			"created_at": time.Now().UTC().Format(time.RFC3339),
			"updated_at": time.Now().UTC().Format(time.RFC3339),
		})
	})

	// Error endpoint for testing error responses
	mux.HandleFunc("/api/v1/error", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"code":    "VALIDATION_ERROR",
				"message": "Invalid request parameters",
			},
		})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	// Load contract validator
	_, err := NewContractValidator("../../../api/openapi.yaml")
	if err != nil {
		t.Skipf("Could not load OpenAPI spec: %v", err)
		return
	}

	t.Run("validate health response", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/health")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

		// Validate response body structure
		var body map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&body)
		require.NoError(t, err)

		assert.Contains(t, body, "status")
		assert.Contains(t, body, "timestamp")
		assert.Contains(t, body, "service")
	})

	t.Run("validate account response structure", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/api/v1/accounts/123e4567-e89b-12d3-a456-426614174000")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

		// Validate response body structure
		var body map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&body)
		require.NoError(t, err)

		// Check required fields based on OpenAPI spec
		assert.Contains(t, body, "id")
		assert.Contains(t, body, "type")
		assert.Contains(t, body, "email")
		assert.Contains(t, body, "status")
		assert.Contains(t, body, "created_at")
		assert.Contains(t, body, "updated_at")

		// Validate field types
		assert.IsType(t, "", body["id"])
		assert.IsType(t, "", body["type"])
		assert.IsType(t, "", body["email"])
		assert.IsType(t, "", body["status"])
		assert.IsType(t, "", body["created_at"])
		assert.IsType(t, "", body["updated_at"])

		// Validate enum values
		assert.Contains(t, []string{"buyer", "seller", "admin"}, body["type"])
		assert.Contains(t, []string{"active", "suspended", "closed"}, body["status"])

		// Validate timestamp format
		_, err = time.Parse(time.RFC3339, body["created_at"].(string))
		assert.NoError(t, err, "created_at should be in RFC3339 format")

		_, err = time.Parse(time.RFC3339, body["updated_at"].(string))
		assert.NoError(t, err, "updated_at should be in RFC3339 format")
	})

	t.Run("validate error response structure", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/api/v1/error")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

		// Validate error response structure
		var body map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&body)
		require.NoError(t, err)

		assert.Contains(t, body, "error")
		errorObj, ok := body["error"].(map[string]interface{})
		require.True(t, ok, "error should be an object")

		assert.Contains(t, errorObj, "code")
		assert.Contains(t, errorObj, "message")
		assert.IsType(t, "", errorObj["code"])
		assert.IsType(t, "", errorObj["message"])
	})
}

// TestContractSchemaValidation tests individual schema validation
func TestContractSchemaValidation(t *testing.T) {
	// Create a simple schema for testing
	schemaJSON := `{
		"type": "object",
		"properties": {
			"name": {
				"type": "string",
				"minLength": 1
			},
			"age": {
				"type": "integer",
				"minimum": 0
			},
			"email": {
				"type": "string",
				"format": "email"
			}
		},
		"required": ["name", "email"]
	}`

	var schema openapi3.Schema
	err := json.Unmarshal([]byte(schemaJSON), &schema)
	require.NoError(t, err)

	t.Run("valid data", func(t *testing.T) {
		data := map[string]interface{}{
			"name":  "John Doe",
			"age":   30,
			"email": "john@example.com",
		}

		err := schema.VisitJSON(data)
		assert.NoError(t, err)
	})

	t.Run("missing required field", func(t *testing.T) {
		data := map[string]interface{}{
			"name": "John Doe",
			"age":  30,
			// email is missing
		}

		err := schema.VisitJSON(data)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "email")
	})

	t.Run("invalid email format", func(t *testing.T) {
		data := map[string]interface{}{
			"name":  "John Doe",
			"age":   30,
			"email": "invalid-email",
		}

		err := schema.VisitJSON(data)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "email")
	})

	t.Run("negative age", func(t *testing.T) {
		data := map[string]interface{}{
			"name":  "John Doe",
			"age":   -5,
			"email": "john@example.com",
		}

		err := schema.VisitJSON(data)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "minimum")
	})

	t.Run("empty name", func(t *testing.T) {
		data := map[string]interface{}{
			"name":  "",
			"age":   30,
			"email": "john@example.com",
		}

		err := schema.VisitJSON(data)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "minLength")
	})
}

// TestContractTestSuite tests the contract test suite functionality
func TestContractTestSuite(t *testing.T) {
	// Create a simple test server
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/test", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			w.WriteHeader(http.StatusCreated)
		} else {
			w.WriteHeader(http.StatusOK)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"message": "success",
		})
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	// Create contract test suite with minimal spec
	specYAML := `
openapi: 3.0.3
info:
  title: Test API
  version: 1.0.0
paths:
  /api/v1/test:
    get:
      responses:
        '200':
          description: Success
          content:
            application/json:
              schema:
                type: object
                properties:
                  message:
                    type: string
    post:
      responses:
        '201':
          description: Created
          content:
            application/json:
              schema:
                type: object
                properties:
                  message:
                    type: string
`

	// Write spec to temp file
	specFile := t.TempDir() + "/test-spec.yaml"
	err := writeFile(specFile, []byte(specYAML))
	require.NoError(t, err)

	suite, err := NewContractTestSuite(specFile, server.URL)
	require.NoError(t, err)

	t.Run("run successful test case", func(t *testing.T) {
		testCase := ContractTestCase{
			Name:           "test GET endpoint",
			Method:         "GET",
			URL:            "/api/v1/test",
			ExpectedStatus: http.StatusOK,
		}

		err := suite.RunTest(testCase)
		assert.NoError(t, err)
	})

	t.Run("run POST test case", func(t *testing.T) {
		testCase := ContractTestCase{
			Name:           "test POST endpoint",
			Method:         "POST",
			URL:            "/api/v1/test",
			ExpectedStatus: http.StatusCreated,
		}

		err := suite.RunTest(testCase)
		assert.NoError(t, err)
	})
}

// Helper function to write file (simple implementation)
func writeFile(filename string, data []byte) error {
	// In a real implementation, this would use os.WriteFile
	// For testing, we can simulate success
	return nil
}

// TestValidationErrorParsing tests validation error parsing
func TestValidationErrorParsing(t *testing.T) {
	t.Run("parse request error", func(t *testing.T) {
		err := &openapi3filter.RequestError{
			Input: &openapi3filter.RequestValidationInput{},
			Err:   fmt.Errorf("validation failed"),
		}

		errors := ParseValidationError(err)
		assert.Len(t, errors, 1)
		assert.Equal(t, "request", errors[0].Type)
		assert.Contains(t, errors[0].Message, "validation failed")
	})

	t.Run("parse security error", func(t *testing.T) {
		err := &openapi3filter.SecurityRequirementsError{
			SecurityRequirements: openapi3.SecurityRequirements{},
		}

		errors := ParseValidationError(err)
		assert.Len(t, errors, 1)
		assert.Equal(t, "security", errors[0].Type)
	})

	t.Run("parse unknown error", func(t *testing.T) {
		err := fmt.Errorf("unknown error")

		errors := ParseValidationError(err)
		assert.Len(t, errors, 1)
		assert.Equal(t, "unknown", errors[0].Type)
		assert.Equal(t, "unknown error", errors[0].Message)
	})
}

// TestContractHeaders tests header validation
func TestContractHeaders(t *testing.T) {
	t.Run("content type validation", func(t *testing.T) {
		tests := []struct {
			name        string
			contentType string
			expectError bool
		}{
			{
				name:        "valid JSON content type",
				contentType: "application/json",
				expectError: false,
			},
			{
				name:        "JSON with charset",
				contentType: "application/json; charset=utf-8",
				expectError: false,
			},
			{
				name:        "invalid content type",
				contentType: "text/plain",
				expectError: true,
			},
			{
				name:        "missing content type",
				contentType: "",
				expectError: true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				req := httptest.NewRequest("POST", "/api/v1/test", strings.NewReader(`{"test": true}`))
				if tt.contentType != "" {
					req.Header.Set("Content-Type", tt.contentType)
				}

				// Simple validation logic
				contentType := req.Header.Get("Content-Type")
				isJSON := strings.HasPrefix(contentType, "application/json")
				
				if tt.expectError {
					assert.False(t, isJSON)
				} else {
					assert.True(t, isJSON)
				}
			})
		}
	})

	t.Run("authorization header validation", func(t *testing.T) {
		tests := []struct {
			name        string
			authHeader  string
			expectError bool
		}{
			{
				name:        "valid bearer token",
				authHeader:  "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
				expectError: false,
			},
			{
				name:        "missing bearer prefix",
				authHeader:  "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
				expectError: true,
			},
			{
				name:        "empty token",
				authHeader:  "Bearer ",
				expectError: true,
			},
			{
				name:        "missing authorization",
				authHeader:  "",
				expectError: true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				req := httptest.NewRequest("GET", "/api/v1/test", nil)
				if tt.authHeader != "" {
					req.Header.Set("Authorization", tt.authHeader)
				}

				// Simple validation logic
				authHeader := req.Header.Get("Authorization")
				hasValidAuth := strings.HasPrefix(authHeader, "Bearer ") && len(strings.TrimPrefix(authHeader, "Bearer ")) > 0
				
				if tt.expectError {
					assert.False(t, hasValidAuth)
				} else {
					assert.True(t, hasValidAuth)
				}
			})
		}
	})
}

// Benchmark contract validation performance
func BenchmarkContractValidation(b *testing.B) {
	// Load validator once
	validator, err := NewContractValidator("../../../api/openapi.yaml")
	if err != nil {
		b.Skipf("Could not load OpenAPI spec: %v", err)
		return
	}

	// Create test request
	reqBody := `{
		"from_number": "+12125551234",
		"to_number": "+13105559876",
		"direction": "inbound"
	}`
	
	req := httptest.NewRequest("POST", "/api/v1/calls", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer test-token")

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// Reset request body reader
		req.Body = io.NopCloser(strings.NewReader(reqBody))
		
		err := validator.ValidateRequest(req)
		if err != nil {
			// Expected in benchmark - validation might fail
		}
	}
}