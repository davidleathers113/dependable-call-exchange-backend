package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/audit"
)

// MockAuditLogger for testing
type MockAuditLogger struct {
	mock.Mock
}

func (m *MockAuditLogger) LogEventWithRequest(ctx context.Context, request *http.Request,
	eventType audit.EventType, actorID, targetID, action, result string,
	metadata map[string]interface{}) error {
	
	args := m.Called(ctx, request, eventType, actorID, targetID, action, result, metadata)
	return args.Error(0)
}

func TestNewAuditMiddleware(t *testing.T) {
	logger := zap.NewNop()

	t.Run("valid_config", func(t *testing.T) {
		mockLogger := &MockAuditLogger{}
		config := DefaultAuditMiddlewareConfig()
		config.AuditLogger = mockLogger

		middleware, err := NewAuditMiddleware(config, logger)
		
		assert.NoError(t, err)
		assert.NotNil(t, middleware)
		assert.Equal(t, config.Enabled, middleware.config.Enabled)
	})

	t.Run("missing_audit_logger", func(t *testing.T) {
		config := DefaultAuditMiddlewareConfig()
		config.AuditLogger = nil

		middleware, err := NewAuditMiddleware(config, logger)
		
		assert.Error(t, err)
		assert.Nil(t, middleware)
		assert.Contains(t, err.Error(), "audit logger is required")
	})
}

func TestAuditMiddleware_Middleware(t *testing.T) {
	mockLogger := &MockAuditLogger{}
	config := DefaultAuditMiddlewareConfig()
	config.AuditLogger = mockLogger
	config.Enabled = true

	middleware, err := NewAuditMiddleware(config, zap.NewNop())
	require.NoError(t, err)

	t.Run("middleware_disabled", func(t *testing.T) {
		config.Enabled = false
		disabledMiddleware, err := NewAuditMiddleware(config, zap.NewNop())
		require.NoError(t, err)

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		})

		wrappedHandler := disabledMiddleware.Middleware()(handler)
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		wrappedHandler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		mockLogger.AssertNotCalled(t, "LogEventWithRequest")
	})

	t.Run("successful_request", func(t *testing.T) {
		mockLogger.ExpectedCalls = nil // Reset mock
		mockLogger.On("LogEventWithRequest", 
			mock.Anything, mock.Anything, audit.EventTypeAPIRequest,
			mock.AnythingOfType("string"), mock.AnythingOfType("string"),
			mock.AnythingOfType("string"), "INITIATED", mock.Anything).Return(nil)
		mockLogger.On("LogEventWithRequest", 
			mock.Anything, mock.Anything, audit.EventTypeAPIResponse,
			mock.AnythingOfType("string"), mock.AnythingOfType("string"),
			mock.AnythingOfType("string"), "SUCCESS", mock.Anything).Return(nil)

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"message": "success"}`))
		})

		wrappedHandler := middleware.Middleware()(handler)
		req := httptest.NewRequest("GET", "/api/v1/test", nil)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Request-ID", "test-123")
		w := httptest.NewRecorder()

		wrappedHandler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		mockLogger.AssertExpectations(t)
	})

	t.Run("request_with_body", func(t *testing.T) {
		mockLogger.ExpectedCalls = nil // Reset mock
		mockLogger.On("LogEventWithRequest", 
			mock.Anything, mock.Anything, audit.EventTypeAPIRequest,
			mock.AnythingOfType("string"), mock.AnythingOfType("string"),
			mock.AnythingOfType("string"), "INITIATED", 
			mock.MatchedBy(func(metadata map[string]interface{}) bool {
				// Check that request body is captured
				_, hasBody := metadata["request_body"]
				return hasBody
			})).Return(nil)
		mockLogger.On("LogEventWithRequest", 
			mock.Anything, mock.Anything, audit.EventTypeAPIResponse,
			mock.AnythingOfType("string"), mock.AnythingOfType("string"),
			mock.AnythingOfType("string"), "SUCCESS", mock.Anything).Return(nil)

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"id": "123", "status": "created"}`))
		})

		requestBody := `{"name": "test", "password": "secret123"}`
		wrappedHandler := middleware.Middleware()(handler)
		req := httptest.NewRequest("POST", "/api/v1/calls", strings.NewReader(requestBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		wrappedHandler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)
		mockLogger.AssertExpectations(t)
	})
}

func TestAuditMiddleware_SecurityValidation(t *testing.T) {
	mockLogger := &MockAuditLogger{}
	config := DefaultAuditMiddlewareConfig()
	config.AuditLogger = mockLogger
	config.SecurityChecks = SecurityChecks{
		ValidateContentType: true,
		AllowedContentTypes: []string{"application/json"},
		MaxRequestSize:      1024,
		ValidateOrigin:      true,
		AllowedOrigins:      []string{"https://example.com"},
	}

	middleware, err := NewAuditMiddleware(config, zap.NewNop())
	require.NoError(t, err)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	t.Run("invalid_content_type", func(t *testing.T) {
		mockLogger.ExpectedCalls = nil
		mockLogger.On("LogEventWithRequest", 
			mock.Anything, mock.Anything, audit.EventTypeSecurityIncident,
			mock.AnythingOfType("string"), mock.AnythingOfType("string"),
			"SECURITY_VALIDATION_FAILED", "BLOCKED", mock.Anything).Return(nil)

		wrappedHandler := middleware.Middleware()(handler)
		req := httptest.NewRequest("POST", "/api/v1/test", nil)
		req.Header.Set("Content-Type", "text/plain")
		w := httptest.NewRecorder()

		wrappedHandler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
		
		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, "SECURITY_VIOLATION", response["error"].(map[string]interface{})["code"])
	})

	t.Run("request_too_large", func(t *testing.T) {
		mockLogger.ExpectedCalls = nil
		mockLogger.On("LogEventWithRequest", 
			mock.Anything, mock.Anything, audit.EventTypeSecurityIncident,
			mock.AnythingOfType("string"), mock.AnythingOfType("string"),
			"SECURITY_VALIDATION_FAILED", "BLOCKED", mock.Anything).Return(nil)

		largeBody := strings.Repeat("a", 2048) // Larger than 1024 limit
		wrappedHandler := middleware.Middleware()(handler)
		req := httptest.NewRequest("POST", "/api/v1/test", strings.NewReader(largeBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		wrappedHandler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("invalid_origin", func(t *testing.T) {
		mockLogger.ExpectedCalls = nil
		mockLogger.On("LogEventWithRequest", 
			mock.Anything, mock.Anything, audit.EventTypeSecurityIncident,
			mock.AnythingOfType("string"), mock.AnythingOfType("string"),
			"SECURITY_VALIDATION_FAILED", "BLOCKED", mock.Anything).Return(nil)

		wrappedHandler := middleware.Middleware()(handler)
		req := httptest.NewRequest("POST", "/api/v1/test", nil)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", "https://malicious.com")
		w := httptest.NewRecorder()

		wrappedHandler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("valid_origin", func(t *testing.T) {
		mockLogger.ExpectedCalls = nil
		mockLogger.On("LogEventWithRequest", 
			mock.Anything, mock.Anything, audit.EventTypeAPIRequest,
			mock.AnythingOfType("string"), mock.AnythingOfType("string"),
			mock.AnythingOfType("string"), "INITIATED", mock.Anything).Return(nil)
		mockLogger.On("LogEventWithRequest", 
			mock.Anything, mock.Anything, audit.EventTypeAPIResponse,
			mock.AnythingOfType("string"), mock.AnythingOfType("string"),
			mock.AnythingOfType("string"), "SUCCESS", mock.Anything).Return(nil)

		wrappedHandler := middleware.Middleware()(handler)
		req := httptest.NewRequest("POST", "/api/v1/test", nil)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", "https://example.com")
		w := httptest.NewRecorder()

		wrappedHandler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		mockLogger.AssertExpectations(t)
	})
}

func TestAuditMiddleware_RateLimiting(t *testing.T) {
	mockLogger := &MockAuditLogger{}
	config := DefaultAuditMiddlewareConfig()
	config.AuditLogger = mockLogger
	config.RateLimits = map[string]EndpointRateLimit{
		"GET:/api/v1/test": {
			RequestsPerSecond: 2,
			Burst:             2,
			ByIP:              true,
		},
	}

	middleware, err := NewAuditMiddleware(config, zap.NewNop())
	require.NoError(t, err)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrappedHandler := middleware.Middleware()(handler)

	t.Run("within_rate_limit", func(t *testing.T) {
		mockLogger.ExpectedCalls = nil
		mockLogger.On("LogEventWithRequest", mock.Anything, mock.Anything, 
			mock.Anything, mock.Anything, mock.Anything, mock.Anything, 
			mock.Anything, mock.Anything).Return(nil)

		req := httptest.NewRequest("GET", "/api/v1/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		w := httptest.NewRecorder()

		wrappedHandler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("rate_limit_exceeded", func(t *testing.T) {
		mockLogger.ExpectedCalls = nil
		mockLogger.On("LogEventWithRequest", mock.Anything, mock.Anything, 
			audit.EventTypeRateLimitExceeded, mock.Anything, mock.Anything, 
			"RATE_LIMIT_CHECK", "EXCEEDED", mock.Anything).Return(nil)

		// Make requests to exceed rate limit
		for i := 0; i < 5; i++ {
			req := httptest.NewRequest("GET", "/api/v1/test", nil)
			req.RemoteAddr = "192.168.1.2:12345"
			w := httptest.NewRecorder()

			wrappedHandler.ServeHTTP(w, req)

			if i < 2 {
				assert.Equal(t, http.StatusOK, w.Code)
			} else {
				assert.Equal(t, http.StatusTooManyRequests, w.Code)
				
				// Check rate limit headers
				assert.NotEmpty(t, w.Header().Get("Retry-After"))
				
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Equal(t, "RATE_LIMIT_EXCEEDED", response["error"].(map[string]interface{})["code"])
			}
		}
	})
}

func TestAuditMiddleware_ContextEnrichment(t *testing.T) {
	mockLogger := &MockAuditLogger{}
	config := DefaultAuditMiddlewareConfig()
	config.AuditLogger = mockLogger

	middleware, err := NewAuditMiddleware(config, zap.NewNop())
	require.NoError(t, err)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify context enrichment
		assert.NotNil(t, r.Context().Value("audit.request_id"))
		assert.NotNil(t, r.Context().Value("audit.client_ip"))
		assert.NotNil(t, r.Context().Value("audit.user_agent"))
		assert.NotNil(t, r.Context().Value("audit.start_time"))
		
		w.WriteHeader(http.StatusOK)
	})

	wrappedHandler := middleware.Middleware()(handler)

	t.Run("context_enrichment", func(t *testing.T) {
		mockLogger.ExpectedCalls = nil
		mockLogger.On("LogEventWithRequest", mock.Anything, mock.Anything, 
			mock.Anything, mock.Anything, mock.Anything, mock.Anything, 
			mock.Anything, mock.Anything).Return(nil)

		req := httptest.NewRequest("GET", "/api/v1/test", nil)
		req.Header.Set("X-Request-ID", "test-request-123")
		req.Header.Set("User-Agent", "TestAgent/1.0")
		req.Header.Set("X-Forwarded-For", "203.0.113.1, 198.51.100.1")
		w := httptest.NewRecorder()

		wrappedHandler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestAuditMiddleware_DataSanitization(t *testing.T) {
	mockLogger := &MockAuditLogger{}
	config := DefaultAuditMiddlewareConfig()
	config.AuditLogger = mockLogger
	config.SensitiveKeys = []string{"password", "secret", "token"}

	middleware, err := NewAuditMiddleware(config, zap.NewNop())
	require.NoError(t, err)

	t.Run("sanitize_sensitive_data", func(t *testing.T) {
		data := map[string]interface{}{
			"username": "testuser",
			"password": "secret123",
			"api_token": "abc123",
			"user_info": map[string]interface{}{
				"email": "test@example.com",
				"secret_key": "hidden",
			},
		}

		sanitized := middleware.sanitizeData(data)
		sanitizedMap := sanitized.(map[string]interface{})

		assert.Equal(t, "testuser", sanitizedMap["username"])
		assert.Equal(t, "[REDACTED]", sanitizedMap["password"])
		assert.Equal(t, "[REDACTED]", sanitizedMap["api_token"])
		
		userInfo := sanitizedMap["user_info"].(map[string]interface{})
		assert.Equal(t, "test@example.com", userInfo["email"])
		assert.Equal(t, "[REDACTED]", userInfo["secret_key"])
	})

	t.Run("sanitize_array_data", func(t *testing.T) {
		data := []interface{}{
			map[string]interface{}{
				"name": "user1",
				"password": "secret1",
			},
			map[string]interface{}{
				"name": "user2",
				"password": "secret2",
			},
		}

		sanitized := middleware.sanitizeData(data)
		sanitizedArray := sanitized.([]interface{})

		assert.Len(t, sanitizedArray, 2)
		
		user1 := sanitizedArray[0].(map[string]interface{})
		assert.Equal(t, "user1", user1["name"])
		assert.Equal(t, "[REDACTED]", user1["password"])
	})
}

func TestAuditMiddleware_HelperMethods(t *testing.T) {
	config := DefaultAuditMiddlewareConfig()
	middleware := &AuditMiddleware{config: config}

	t.Run("getClientIP", func(t *testing.T) {
		// Test X-Forwarded-For
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Forwarded-For", "203.0.113.1, 198.51.100.1")
		assert.Equal(t, "203.0.113.1", middleware.getClientIP(req))

		// Test X-Real-IP
		req = httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Real-IP", "198.51.100.1")
		assert.Equal(t, "198.51.100.1", middleware.getClientIP(req))

		// Test RemoteAddr
		req = httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		assert.Equal(t, "192.168.1.1", middleware.getClientIP(req))
	})

	t.Run("normalizeEndpointPath", func(t *testing.T) {
		tests := []struct {
			input    string
			expected string
		}{
			{"/api/v1/calls/123", "/api/v1/calls/{id}"},
			{"/api/v1/calls/550e8400-e29b-41d4-a716-446655440000", "/api/v1/calls/{id}"},
			{"/api/v1/accounts/abc123/settings", "/api/v1/accounts/{id}/settings"},
			{"/api/v1/status", "/api/v1/status"},
		}

		for _, test := range tests {
			result := middleware.normalizeEndpointPath(test.input)
			assert.Equal(t, test.expected, result, "Input: %s", test.input)
		}
	})

	t.Run("shouldAuditEndpoint", func(t *testing.T) {
		// Test exclusions
		config.EventFilters.ExcludeEndpoints = []string{"/health", "/metrics"}
		assert.False(t, middleware.shouldAuditEndpoint("/health"))
		assert.False(t, middleware.shouldAuditEndpoint("/metrics"))
		assert.True(t, middleware.shouldAuditEndpoint("/api/v1/calls"))

		// Test inclusions
		config.EventFilters.ExcludeEndpoints = []string{}
		config.EventFilters.IncludeEndpoints = []string{"/api/v1"}
		assert.True(t, middleware.shouldAuditEndpoint("/api/v1/calls"))
		assert.False(t, middleware.shouldAuditEndpoint("/health"))
	})

	t.Run("isUUID", func(t *testing.T) {
		assert.True(t, middleware.isUUID("550e8400-e29b-41d4-a716-446655440000"))
		assert.True(t, middleware.isUUID(uuid.New().String()))
		assert.False(t, middleware.isUUID("not-a-uuid"))
		assert.False(t, middleware.isUUID("123"))
	})

	t.Run("isNumeric", func(t *testing.T) {
		assert.True(t, middleware.isNumeric("123"))
		assert.True(t, middleware.isNumeric("0"))
		assert.True(t, middleware.isNumeric("-456"))
		assert.False(t, middleware.isNumeric("abc"))
		assert.False(t, middleware.isNumeric("12.34"))
	})

	t.Run("isSensitiveKey", func(t *testing.T) {
		assert.True(t, middleware.isSensitiveKey("password"))
		assert.True(t, middleware.isSensitiveKey("api_token"))
		assert.True(t, middleware.isSensitiveKey("secret_key"))
		assert.True(t, middleware.isSensitiveKey("Password")) // Case insensitive
		assert.False(t, middleware.isSensitiveKey("username"))
		assert.False(t, middleware.isSensitiveKey("email"))
	})
}

func TestAuditMiddleware_PerformanceMonitoring(t *testing.T) {
	thresholds := PerformanceThresholds{
		SlowRequestThreshold: 100 * time.Millisecond,
		ErrorRateThreshold:   0.1,
		AlertOnBreach:        true,
	}

	monitor := NewPerformanceMonitor(thresholds)

	t.Run("record_performance", func(t *testing.T) {
		endpoint := "/api/v1/test"
		
		// Record some requests
		monitor.Record(endpoint, 50*time.Millisecond, false)
		monitor.Record(endpoint, 150*time.Millisecond, false)
		monitor.Record(endpoint, 75*time.Millisecond, true)

		stats := monitor.stats[endpoint]
		assert.Equal(t, int64(3), stats.TotalRequests)
		assert.Equal(t, int64(1), stats.ErrorCount)
		assert.Equal(t, 150*time.Millisecond, stats.MaxDuration)
	})
}

func TestAuditResponseWriter(t *testing.T) {
	w := httptest.NewRecorder()
	auditWriter := &auditResponseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
		body:          &bytes.Buffer{},
		headers:       make(http.Header),
	}

	t.Run("capture_response", func(t *testing.T) {
		auditWriter.WriteHeader(http.StatusCreated)
		auditWriter.Write([]byte("test response"))

		assert.Equal(t, http.StatusCreated, auditWriter.statusCode)
		assert.Equal(t, "test response", auditWriter.body.String())
	})
}

func BenchmarkAuditMiddleware(b *testing.B) {
	mockLogger := &MockAuditLogger{}
	mockLogger.On("LogEventWithRequest", mock.Anything, mock.Anything, 
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, 
		mock.Anything, mock.Anything).Return(nil)

	config := DefaultAuditMiddlewareConfig()
	config.AuditLogger = mockLogger

	middleware, err := NewAuditMiddleware(config, zap.NewNop())
	require.NoError(b, err)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message": "ok"}`))
	})

	wrappedHandler := middleware.Middleware()(handler)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest("GET", "/api/v1/test", nil)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			wrappedHandler.ServeHTTP(w, req)
		}
	})
}

// Example usage test
func ExampleAuditMiddleware() {
	// Create mock audit logger
	mockLogger := &MockAuditLogger{}
	mockLogger.On("LogEventWithRequest", mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything).Return(nil)

	// Configure audit middleware
	config := DefaultAuditMiddlewareConfig()
	config.AuditLogger = mockLogger
	config.RateLimits = map[string]EndpointRateLimit{
		"POST:/api/v1/calls": {
			RequestsPerSecond: 100,
			Burst:             200,
			ByIP:              true,
			ByUser:            true,
		},
	}

	// Create middleware
	auditMiddleware, err := NewAuditMiddleware(config, zap.NewNop())
	if err != nil {
		panic(err)
	}

	// Create handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message": "success"}`))
	})

	// Apply middleware
	wrappedHandler := auditMiddleware.Middleware()(handler)

	// Use in HTTP server
	server := &http.Server{
		Addr:    ":8080",
		Handler: wrappedHandler,
	}

	fmt.Printf("Server configured with audit middleware")
	_ = server
	// Output: Server configured with audit middleware
}