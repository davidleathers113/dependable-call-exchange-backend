package middleware

import (
	"context"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/audit"
	auditService "github.com/davidleathers/dependable-call-exchange-backend/internal/service/audit"
)

// AuditIntegrationExample demonstrates how to integrate the audit middleware
// with the existing DCE REST API handlers
type AuditIntegrationExample struct {
	auditLogger   *auditService.Logger
	auditConfig   AuditMiddlewareConfig
	middleware    *AuditMiddleware
	logger        *zap.Logger
}

// NewAuditIntegrationExample creates a complete audit integration example
func NewAuditIntegrationExample(
	auditLogger *auditService.Logger,
	logger *zap.Logger,
) (*AuditIntegrationExample, error) {

	// Configure audit middleware for DCE API endpoints
	config := AuditMiddlewareConfig{
		AuditLogger:    auditLogger,
		Enabled:        true,
		AuditRequests:  true,
		AuditResponses: true,
		AuditHeaders: []string{
			"Authorization",
			"X-API-Key", 
			"X-Request-ID",
			"X-Session-ID",
			"User-Agent",
			"X-Forwarded-For",
		},
		SensitiveKeys: []string{
			"password",
			"token", 
			"secret",
			"key",
			"auth",
			"credential",
			"api_key",
			"bearer",
			"oauth",
		},
		SecurityChecks: SecurityChecks{
			ValidateContentType: true,
			AllowedContentTypes: []string{
				"application/json",
				"application/x-www-form-urlencoded",
				"multipart/form-data",
			},
			MaxRequestSize: 10 * 1024 * 1024, // 10MB for file uploads
			RequireAuth:    true,
			ValidateOrigin: true,
			AllowedOrigins: []string{
				"https://app.dependablecallexchange.com",
				"https://admin.dependablecallexchange.com",
				"http://localhost:3000", // Development
				"http://localhost:8080", // Development
			},
		},
		PerformanceThresholds: PerformanceThresholds{
			SlowRequestThreshold: 2 * time.Second,   // DCE targets < 1ms routing
			ErrorRateThreshold:   0.01,              // 1% error rate threshold  
			AlertOnBreach:        true,
		},
		EventFilters: EventFilters{
			// Exclude health and monitoring endpoints
			ExcludeEndpoints: []string{
				"/health",
				"/ready", 
				"/metrics",
				"/debug/pprof",
			},
			// Include all API endpoints
			IncludeEndpoints: []string{
				"/api/v1",
				"/api/v2",
				"/webhook",
				"/admin",
			},
			MinSeverity: audit.SeverityLow,
			EventTypes: []audit.EventType{
				audit.EventTypeAPIRequest,
				audit.EventTypeAPIResponse,
				audit.EventTypeDataAccess,
				audit.EventTypeUserLogin,
				audit.EventTypeUserLogout,
				audit.EventTypeComplianceViolation,
				audit.EventTypeSecurityIncident,
				audit.EventTypeRateLimitExceeded,
				audit.EventTypeSystemFailure,
			},
		},
		ContinueOnError: true, // Don't fail requests on audit errors
		
		// DCE-specific rate limits for audit API
		RateLimits: map[string]EndpointRateLimit{
			// High-volume endpoints
			"GET:/api/v1/calls": {
				RequestsPerSecond: 500,
				Burst:             1000,
				Window:            time.Minute,
				ByUser:            true,
				ByEndpoint:        true,
			},
			"POST:/api/v1/calls": {
				RequestsPerSecond: 100,
				Burst:             200,
				Window:            time.Minute,
				ByUser:            true,
				ByIP:              true,
			},
			"GET:/api/v1/bids": {
				RequestsPerSecond: 1000, // High volume bidding
				Burst:             2000,
				Window:            time.Minute,
				ByUser:            true,
			},
			"POST:/api/v1/bids": {
				RequestsPerSecond: 200,
				Burst:             500,
				Window:            time.Minute,
				ByUser:            true,
				ByIP:              true,
			},
			
			// Audit-specific endpoints - more restrictive
			"GET:/api/v1/audit/events": {
				RequestsPerSecond: 50,
				Burst:             100,
				Window:            time.Minute,
				ByUser:            true,
			},
			"POST:/api/v1/audit/events": {
				RequestsPerSecond: 100,
				Burst:             200,
				Window:            time.Minute,
				ByUser:            true,
				ByIP:              true,
			},
			"GET:/api/v1/audit/reports": {
				RequestsPerSecond: 10,
				Burst:             20,
				Window:            time.Minute,
				ByUser:            true,
			},
			
			// Admin endpoints - very restrictive
			"GET:/admin/audit/integrity": {
				RequestsPerSecond: 5,
				Burst:             10,
				Window:            time.Minute,
				ByUser:            true,
				ByIP:              true,
			},
			"POST:/admin/audit/repair": {
				RequestsPerSecond: 1,
				Burst:             2,
				Window:            time.Minute,
				ByUser:            true,
				ByIP:              true,
			},
			
			// File upload endpoints
			"POST:/api/v1/upload": {
				RequestsPerSecond: 5,
				Burst:             10,
				Window:            time.Minute,
				ByUser:            true,
				ByIP:              true,
			},
			
			// Authentication endpoints
			"POST:/api/v1/auth/login": {
				RequestsPerSecond: 10,
				Burst:             20,
				Window:            time.Minute,
				ByIP:              true,
			},
			"POST:/api/v1/auth/register": {
				RequestsPerSecond: 5,
				Burst:             10,
				Window:            time.Minute,
				ByIP:              true,
			},
		},
	}

	// Create audit middleware
	middleware, err := NewAuditMiddleware(config, logger)
	if err != nil {
		return nil, err
	}

	return &AuditIntegrationExample{
		auditLogger: auditLogger,
		auditConfig: config,
		middleware:  middleware,
		logger:      logger,
	}, nil
}

// GetMiddleware returns the configured audit middleware
func (aie *AuditIntegrationExample) GetMiddleware() func(http.Handler) http.Handler {
	return aie.middleware.Middleware()
}

// CreateMiddlewareChain creates a complete middleware chain for DCE API
func (aie *AuditIntegrationExample) CreateMiddlewareChain() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		// Build middleware chain in correct order:
		// 1. Recovery (outermost)
		// 2. Logging
		// 3. Security headers  
		// 4. CORS
		// 5. Request ID
		// 6. Metrics
		// 7. Tracing
		// 8. Audit (before auth to capture all attempts)
		// 9. Rate limiting (before auth)
		// 10. Authentication
		// 11. Authorization
		// 12. Timeout
		// 13. Handler (innermost)
		
		handler := next
		
		// Apply in reverse order (innermost first)
		handler = aie.timeoutMiddleware(30 * time.Second)(handler)
		// handler = authorizationMiddleware(handler) // Apply after auth
		// handler = authenticationMiddleware(handler) // Apply after audit
		handler = aie.middleware.Middleware()(handler) // Audit middleware
		// handler = rateLimitMiddleware(handler) // Apply after audit
		// handler = tracingMiddleware(handler)
		// handler = metricsMiddleware(handler)  
		// handler = requestIDMiddleware(handler)
		// handler = corsMiddleware(handler)
		// handler = securityHeadersMiddleware(handler)
		// handler = loggingMiddleware(handler)
		// handler = recoveryMiddleware(handler)
		
		return handler
	}
}

// timeoutMiddleware provides request timeout
func (aie *AuditIntegrationExample) timeoutMiddleware(timeout time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()
			
			r = r.WithContext(ctx)
			next.ServeHTTP(w, r)
		})
	}
}

// GetAuditConfiguration returns the audit configuration for reference
func (aie *AuditIntegrationExample) GetAuditConfiguration() AuditMiddlewareConfig {
	return aie.auditConfig
}

// UpdateRateLimit allows dynamic rate limit updates
func (aie *AuditIntegrationExample) UpdateRateLimit(endpoint string, limit EndpointRateLimit) {
	aie.middleware.config.RateLimits[endpoint] = limit
	aie.logger.Info("Updated rate limit",
		zap.String("endpoint", endpoint),
		zap.Int("requests_per_second", limit.RequestsPerSecond),
		zap.Int("burst", limit.Burst),
	)
}

// EnableAuditEndpoint enables auditing for a specific endpoint
func (aie *AuditIntegrationExample) EnableAuditEndpoint(endpoint string) {
	filters := &aie.middleware.config.EventFilters
	for i, excluded := range filters.ExcludeEndpoints {
		if excluded == endpoint {
			// Remove from exclusion list
			filters.ExcludeEndpoints = append(
				filters.ExcludeEndpoints[:i],
				filters.ExcludeEndpoints[i+1:]...,
			)
			aie.logger.Info("Enabled auditing for endpoint", zap.String("endpoint", endpoint))
			return
		}
	}
	
	// Add to inclusion list if not already there
	for _, included := range filters.IncludeEndpoints {
		if included == endpoint {
			return // Already included
		}
	}
	
	filters.IncludeEndpoints = append(filters.IncludeEndpoints, endpoint)
	aie.logger.Info("Added endpoint to audit inclusion list", zap.String("endpoint", endpoint))
}

// DisableAuditEndpoint disables auditing for a specific endpoint
func (aie *AuditIntegrationExample) DisableAuditEndpoint(endpoint string) {
	filters := &aie.middleware.config.EventFilters
	
	// Add to exclusion list
	for _, excluded := range filters.ExcludeEndpoints {
		if excluded == endpoint {
			return // Already excluded
		}
	}
	
	filters.ExcludeEndpoints = append(filters.ExcludeEndpoints, endpoint)
	aie.logger.Info("Disabled auditing for endpoint", zap.String("endpoint", endpoint))
}

// GetAuditStats returns current audit middleware statistics
func (aie *AuditIntegrationExample) GetAuditStats() map[string]interface{} {
	// This would be extended to return actual statistics
	// from the middleware components
	return map[string]interface{}{
		"enabled":                aie.middleware.config.Enabled,
		"audit_requests":         aie.middleware.config.AuditRequests,
		"audit_responses":        aie.middleware.config.AuditResponses,
		"rate_limit_endpoints":   len(aie.middleware.config.RateLimits),
		"excluded_endpoints":     len(aie.middleware.config.EventFilters.ExcludeEndpoints),
		"included_endpoints":     len(aie.middleware.config.EventFilters.IncludeEndpoints),
		"security_checks_enabled": aie.middleware.config.SecurityChecks.ValidateContentType,
		"continue_on_error":      aie.middleware.config.ContinueOnError,
	}
}

// Example usage in main API server setup
func ExampleUsageInAPIServer() {
	// This would be called in the main API server setup
	logger := zap.Must(zap.NewProduction())
	
	// Initialize audit logger (from DCE audit service)
	auditConfig := auditService.DefaultLoggerConfig()
	// auditLogger, _ := auditService.NewLogger(...)
	
	// Create audit integration
	// auditIntegration, err := NewAuditIntegrationExample(auditLogger, logger)
	// if err != nil {
	//     logger.Fatal("Failed to create audit integration", zap.Error(err))
	// }
	
	// Create API handlers
	// handlers := NewAPIHandlers(...)
	
	// Apply middleware chain
	// middlewareChain := auditIntegration.CreateMiddlewareChain()
	// http.Handle("/", middlewareChain(handlers))
	
	logger.Info("Example audit integration setup complete")
}

// DCE-specific audit event types for the middleware
const (
	// Call management events
	EventTypeCallCreated    audit.EventType = "call.created"
	EventTypeCallRouted     audit.EventType = "call.routed"
	EventTypeCallCompleted  audit.EventType = "call.completed"
	EventTypeCallFailed     audit.EventType = "call.failed"
	
	// Bid management events  
	EventTypeBidPlaced      audit.EventType = "bid.placed"
	EventTypeBidWon         audit.EventType = "bid.won"
	EventTypeBidLost        audit.EventType = "bid.lost"
	EventTypeBidCancelled   audit.EventType = "bid.cancelled"
	
	// Account management events
	EventTypeAccountCreated audit.EventType = "account.created"
	EventTypeAccountUpdated audit.EventType = "account.updated"
	EventTypeAccountSuspended audit.EventType = "account.suspended"
	
	// Financial events
	EventTypePaymentProcessed audit.EventType = "payment.processed"
	EventTypePaymentFailed    audit.EventType = "payment.failed"
	EventTypeBillingUpdated   audit.EventType = "billing.updated"
	
	// Compliance events
	EventTypeTCPAViolation    audit.EventType = "compliance.tcpa_violation"
	EventTypeDNCViolation     audit.EventType = "compliance.dnc_violation"
	EventTypeGDPRRequest      audit.EventType = "compliance.gdpr_request"
	
	// System events
	EventTypeSystemMaintenance audit.EventType = "system.maintenance"
	EventTypeConfigUpdated     audit.EventType = "system.config_updated"
	EventTypeIntegrityCheck    audit.EventType = "system.integrity_check"
)