package middleware

import (
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/audit"
)

// AuditMiddlewareInitializer helps initialize audit middleware with DCE-specific defaults
type AuditMiddlewareInitializer struct {
	logger *zap.Logger
}

// NewAuditMiddlewareInitializer creates a new initializer
func NewAuditMiddlewareInitializer(logger *zap.Logger) *AuditMiddlewareInitializer {
	return &AuditMiddlewareInitializer{
		logger: logger,
	}
}

// InitializeForDCEAPI creates audit middleware configured for DCE API endpoints
func (ami *AuditMiddlewareInitializer) InitializeForDCEAPI(
	auditLogger AuditLoggerInterface,
	environment string,
) (*AuditMiddleware, error) {

	config := ami.createDCEAPIConfig(auditLogger, environment)
	
	middleware, err := NewAuditMiddleware(config, ami.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create DCE API audit middleware: %w", err)
	}

	ami.logger.Info("DCE API audit middleware initialized",
		zap.String("environment", environment),
		zap.Bool("enabled", config.Enabled),
		zap.Int("rate_limit_endpoints", len(config.RateLimits)),
	)

	return middleware, nil
}

// InitializeForDCEAdmin creates audit middleware configured for DCE admin endpoints
func (ami *AuditMiddlewareInitializer) InitializeForDCEAdmin(
	auditLogger AuditLoggerInterface,
	environment string,
) (*AuditMiddleware, error) {

	config := ami.createDCEAdminConfig(auditLogger, environment)
	
	middleware, err := NewAuditMiddleware(config, ami.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create DCE admin audit middleware: %w", err)
	}

	ami.logger.Info("DCE admin audit middleware initialized",
		zap.String("environment", environment),
		zap.Bool("enabled", config.Enabled),
		zap.Int("rate_limit_endpoints", len(config.RateLimits)),
	)

	return middleware, nil
}

// InitializeForDCEWebhooks creates audit middleware configured for DCE webhook endpoints
func (ami *AuditMiddlewareInitializer) InitializeForDCEWebhooks(
	auditLogger AuditLoggerInterface,
	environment string,
) (*AuditMiddleware, error) {

	config := ami.createDCEWebhookConfig(auditLogger, environment)
	
	middleware, err := NewAuditMiddleware(config, ami.logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create DCE webhook audit middleware: %w", err)
	}

	ami.logger.Info("DCE webhook audit middleware initialized",
		zap.String("environment", environment),
		zap.Bool("enabled", config.Enabled),
		zap.Int("rate_limit_endpoints", len(config.RateLimits)),
	)

	return middleware, nil
}

// createDCEAPIConfig creates configuration optimized for DCE API endpoints
func (ami *AuditMiddlewareInitializer) createDCEAPIConfig(
	auditLogger AuditLoggerInterface,
	environment string,
) AuditMiddlewareConfig {

	// Adjust settings based on environment
	auditAll := environment != "production"
	rateLimitMultiplier := ami.getRateLimitMultiplier(environment)

	return AuditMiddlewareConfig{
		AuditLogger:    auditLogger,
		Enabled:        true,
		AuditRequests:  auditAll,
		AuditResponses: auditAll,
		AuditHeaders: []string{
			"Authorization",
			"X-API-Key",
			"X-Request-ID",
			"X-Session-ID",
			"User-Agent",
			"X-Forwarded-For",
			"Accept",
			"Content-Type",
		},
		SensitiveKeys: []string{
			// Authentication
			"password", "token", "secret", "key", "auth", "bearer",
			"api_key", "oauth", "credential", "authorization",
			
			// DCE-specific PII
			"phone_number", "caller_id", "called_number",
			"recording_url", "call_audio", "voice_data",
			
			// Financial
			"billing_info", "payment_method", "credit_card",
			"bank_account", "routing_number", "ssn", "tax_id",
			
			// Personal data
			"email", "address", "zipcode", "postal_code",
			"first_name", "last_name", "full_name",
		},
		SecurityChecks: SecurityChecks{
			ValidateContentType: true,
			AllowedContentTypes: []string{
				"application/json",
				"application/x-www-form-urlencoded",
				"multipart/form-data",
			},
			MaxRequestSize: 50 * 1024 * 1024, // 50MB for call recordings
			RequireAuth:    true,
			ValidateOrigin: environment == "production",
			AllowedOrigins: ami.getAllowedOrigins(environment),
		},
		PerformanceThresholds: PerformanceThresholds{
			SlowRequestThreshold: ami.getSlowRequestThreshold(environment),
			ErrorRateThreshold:   0.01, // 1% error rate
			AlertOnBreach:        environment == "production",
		},
		EventFilters: EventFilters{
			ExcludeEndpoints: []string{
				"/health",
				"/ready",
				"/metrics",
				"/debug/pprof",
				"/favicon.ico",
			},
			IncludeEndpoints: []string{
				"/api/v1",
				"/api/v2",
			},
			MinSeverity: audit.SeverityLow,
			EventTypes: []audit.EventType{
				audit.EventTypeAPIRequest,
				audit.EventTypeAPIResponse,
				audit.EventTypeDataAccess,
				audit.EventTypeComplianceViolation,
				audit.EventTypeSecurityIncident,
				audit.EventTypeRateLimitExceeded,
			},
		},
		ContinueOnError: true,
		RateLimits:      ami.createAPIRateLimits(rateLimitMultiplier),
	}
}

// createDCEAdminConfig creates configuration for DCE admin endpoints
func (ami *AuditMiddlewareInitializer) createDCEAdminConfig(
	auditLogger AuditLoggerInterface,
	environment string,
) AuditMiddlewareConfig {

	config := ami.createDCEAPIConfig(auditLogger, environment)
	
	// Admin-specific overrides
	config.AuditRequests = true  // Always audit admin requests
	config.AuditResponses = true // Always audit admin responses
	config.SecurityChecks.RequireAuth = true
	config.SecurityChecks.ValidateOrigin = true
	config.EventFilters.IncludeEndpoints = []string{"/admin"}
	config.EventFilters.MinSeverity = audit.SeverityMedium
	
	// More restrictive rate limits for admin
	rateLimitMultiplier := 0.1 // 10x more restrictive
	config.RateLimits = ami.createAdminRateLimits(rateLimitMultiplier)
	
	return config
}

// createDCEWebhookConfig creates configuration for DCE webhook endpoints
func (ami *AuditMiddlewareInitializer) createDCEWebhookConfig(
	auditLogger AuditLoggerInterface,
	environment string,
) AuditMiddlewareConfig {

	config := ami.createDCEAPIConfig(auditLogger, environment)
	
	// Webhook-specific overrides
	config.SecurityChecks.RequireAuth = false // Webhooks use different auth
	config.SecurityChecks.ValidateOrigin = false // External webhook sources
	config.EventFilters.IncludeEndpoints = []string{"/webhook"}
	config.RateLimits = ami.createWebhookRateLimits(1.0)
	
	return config
}

// createAPIRateLimits creates rate limits for API endpoints
func (ami *AuditMiddlewareInitializer) createAPIRateLimits(multiplier float64) map[string]EndpointRateLimit {
	multiply := func(base int) int {
		return int(float64(base) * multiplier)
	}

	return map[string]EndpointRateLimit{
		// High-volume call routing endpoints
		"GET:/api/v1/calls": {
			RequestsPerSecond: multiply(2000), // DCE targets high throughput
			Burst:             multiply(4000),
			Window:            time.Minute,
			ByUser:            true,
		},
		"POST:/api/v1/calls": {
			RequestsPerSecond: multiply(500),
			Burst:             multiply(1000),
			Window:            time.Minute,
			ByUser:            true,
			ByIP:              true,
		},
		"PUT:/api/v1/calls/{id}": {
			RequestsPerSecond: multiply(200),
			Burst:             multiply(400),
			Window:            time.Minute,
			ByUser:            true,
		},
		
		// Real-time bidding endpoints
		"GET:/api/v1/bids": {
			RequestsPerSecond: multiply(1000),
			Burst:             multiply(2000),
			Window:            time.Minute,
			ByUser:            true,
		},
		"POST:/api/v1/bids": {
			RequestsPerSecond: multiply(800),
			Burst:             multiply(1600),
			Window:            time.Minute,
			ByUser:            true,
			ByIP:              true,
		},
		"PUT:/api/v1/bids/{id}": {
			RequestsPerSecond: multiply(100),
			Burst:             multiply(200),
			Window:            time.Minute,
			ByUser:            true,
		},
		
		// Account management
		"GET:/api/v1/accounts": {
			RequestsPerSecond: multiply(100),
			Burst:             multiply(200),
			Window:            time.Minute,
			ByUser:            true,
		},
		"POST:/api/v1/accounts": {
			RequestsPerSecond: multiply(10),
			Burst:             multiply(20),
			Window:            time.Minute,
			ByIP:              true,
		},
		"PUT:/api/v1/accounts/{id}": {
			RequestsPerSecond: multiply(50),
			Burst:             multiply(100),
			Window:            time.Minute,
			ByUser:            true,
		},
		
		// Authentication endpoints
		"POST:/api/v1/auth/login": {
			RequestsPerSecond: multiply(20),
			Burst:             multiply(40),
			Window:            time.Minute,
			ByIP:              true,
		},
		"POST:/api/v1/auth/register": {
			RequestsPerSecond: multiply(5),
			Burst:             multiply(10),
			Window:            time.Minute,
			ByIP:              true,
		},
		"POST:/api/v1/auth/refresh": {
			RequestsPerSecond: multiply(100),
			Burst:             multiply(200),
			Window:            time.Minute,
			ByUser:            true,
		},
		
		// File upload endpoints
		"POST:/api/v1/upload": {
			RequestsPerSecond: multiply(10),
			Burst:             multiply(20),
			Window:            time.Minute,
			ByUser:            true,
			ByIP:              true,
		},
		
		// Reporting endpoints
		"GET:/api/v1/reports": {
			RequestsPerSecond: multiply(20),
			Burst:             multiply(40),
			Window:            time.Minute,
			ByUser:            true,
		},
		"POST:/api/v1/reports": {
			RequestsPerSecond: multiply(5),
			Burst:             multiply(10),
			Window:            time.Minute,
			ByUser:            true,
		},
	}
}

// createAdminRateLimits creates restrictive rate limits for admin endpoints
func (ami *AuditMiddlewareInitializer) createAdminRateLimits(multiplier float64) map[string]EndpointRateLimit {
	multiply := func(base int) int {
		result := int(float64(base) * multiplier)
		if result < 1 {
			return 1
		}
		return result
	}

	return map[string]EndpointRateLimit{
		// System administration
		"GET:/admin/system/status": {
			RequestsPerSecond: multiply(10),
			Burst:             multiply(20),
			Window:            time.Minute,
			ByUser:            true,
			ByIP:              true,
		},
		"POST:/admin/system/restart": {
			RequestsPerSecond: multiply(1),
			Burst:             multiply(2),
			Window:            time.Hour,
			ByUser:            true,
			ByIP:              true,
		},
		
		// User management
		"GET:/admin/users": {
			RequestsPerSecond: multiply(20),
			Burst:             multiply(40),
			Window:            time.Minute,
			ByUser:            true,
		},
		"POST:/admin/users/{id}/suspend": {
			RequestsPerSecond: multiply(5),
			Burst:             multiply(10),
			Window:            time.Minute,
			ByUser:            true,
			ByIP:              true,
		},
		
		// Audit management
		"GET:/admin/audit/events": {
			RequestsPerSecond: multiply(50),
			Burst:             multiply(100),
			Window:            time.Minute,
			ByUser:            true,
		},
		"GET:/admin/audit/reports": {
			RequestsPerSecond: multiply(10),
			Burst:             multiply(20),
			Window:            time.Minute,
			ByUser:            true,
		},
		"POST:/admin/audit/integrity/check": {
			RequestsPerSecond: multiply(2),
			Burst:             multiply(5),
			Window:            time.Minute,
			ByUser:            true,
			ByIP:              true,
		},
		"POST:/admin/audit/integrity/repair": {
			RequestsPerSecond: multiply(1),
			Burst:             multiply(2),
			Window:            time.Hour,
			ByUser:            true,
			ByIP:              true,
		},
	}
}

// createWebhookRateLimits creates rate limits for webhook endpoints
func (ami *AuditMiddlewareInitializer) createWebhookRateLimits(multiplier float64) map[string]EndpointRateLimit {
	multiply := func(base int) int {
		return int(float64(base) * multiplier)
	}

	return map[string]EndpointRateLimit{
		// External webhooks
		"POST:/webhook/twilio": {
			RequestsPerSecond: multiply(100),
			Burst:             multiply(200),
			Window:            time.Minute,
			ByIP:              true,
		},
		"POST:/webhook/stripe": {
			RequestsPerSecond: multiply(50),
			Burst:             multiply(100),
			Window:            time.Minute,
			ByIP:              true,
		},
		"POST:/webhook/compliance": {
			RequestsPerSecond: multiply(20),
			Burst:             multiply(40),
			Window:            time.Minute,
			ByIP:              true,
		},
		
		// Internal webhooks
		"POST:/webhook/internal/call-status": {
			RequestsPerSecond: multiply(500),
			Burst:             multiply(1000),
			Window:            time.Minute,
			ByIP:              false, // Internal, don't limit by IP
		},
	}
}

// getAllowedOrigins returns allowed origins based on environment
func (ami *AuditMiddlewareInitializer) getAllowedOrigins(environment string) []string {
	switch environment {
	case "production":
		return []string{
			"https://app.dependablecallexchange.com",
			"https://admin.dependablecallexchange.com",
			"https://portal.dependablecallexchange.com",
		}
	case "staging":
		return []string{
			"https://staging-app.dependablecallexchange.com",
			"https://staging-admin.dependablecallexchange.com",
			"http://localhost:3000",
			"http://localhost:3001",
		}
	case "development":
		return []string{
			"http://localhost:3000",
			"http://localhost:3001",
			"http://localhost:8080",
			"http://127.0.0.1:3000",
		}
	default:
		return []string{"*"} // Allow all for unknown environments
	}
}

// getSlowRequestThreshold returns slow request threshold based on environment
func (ami *AuditMiddlewareInitializer) getSlowRequestThreshold(environment string) time.Duration {
	switch environment {
	case "production":
		return 1 * time.Second  // DCE production targets < 1ms routing
	case "staging":
		return 2 * time.Second
	case "development":
		return 5 * time.Second
	default:
		return 10 * time.Second
	}
}

// getRateLimitMultiplier returns rate limit multiplier based on environment
func (ami *AuditMiddlewareInitializer) getRateLimitMultiplier(environment string) float64 {
	switch environment {
	case "production":
		return 1.0 // Full rate limits
	case "staging":
		return 0.5 // Half rate limits for testing
	case "development":
		return 0.1 // Very low limits for development
	default:
		return 0.2 // Conservative for unknown environments
	}
}

// CreateMiddlewareChain creates a complete middleware chain for DCE
func (ami *AuditMiddlewareInitializer) CreateMiddlewareChain(
	auditMiddleware *AuditMiddleware,
	additionalMiddlewares ...func(http.Handler) http.Handler,
) func(http.Handler) http.Handler {
	
	return func(next http.Handler) http.Handler {
		handler := next
		
		// Apply additional middlewares first (innermost)
		for i := len(additionalMiddlewares) - 1; i >= 0; i-- {
			handler = additionalMiddlewares[i](handler)
		}
		
		// Apply audit middleware (before auth to capture all attempts)
		handler = auditMiddleware.Middleware()(handler)
		
		return handler
	}
}