package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"golang.org/x/time/rate"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/audit"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/service/audit"
)

// AuditMiddlewareConfig configures the audit middleware
type AuditMiddlewareConfig struct {
	// Audit logging
	AuditLogger AuditLoggerInterface `json:"-"`
	Enabled     bool                 `json:"enabled"`

	// Rate limiting per endpoint
	RateLimits map[string]EndpointRateLimit `json:"rate_limits"`

	// Request/Response auditing
	AuditRequests  bool     `json:"audit_requests"`
	AuditResponses bool     `json:"audit_responses"`
	AuditHeaders   []string `json:"audit_headers"`
	SensitiveKeys  []string `json:"sensitive_keys"`

	// Security validation
	SecurityChecks SecurityChecks `json:"security"`

	// Performance monitoring
	PerformanceThresholds PerformanceThresholds `json:"performance"`

	// Event filtering
	EventFilters EventFilters `json:"filters"`

	// Error handling
	ContinueOnError bool `json:"continue_on_error"`
}

// EndpointRateLimit defines rate limits for specific endpoints
type EndpointRateLimit struct {
	RequestsPerSecond int           `json:"requests_per_second"`
	Burst             int           `json:"burst"`
	Window            time.Duration `json:"window"`
	ByIP              bool          `json:"by_ip"`
	ByUser            bool          `json:"by_user"`
	ByEndpoint        bool          `json:"by_endpoint"`
}

// SecurityChecks defines security validation settings
type SecurityChecks struct {
	ValidateContentType bool     `json:"validate_content_type"`
	AllowedContentTypes []string `json:"allowed_content_types"`
	MaxRequestSize      int64    `json:"max_request_size"`
	RequireAuth         bool     `json:"require_auth"`
	ValidateOrigin      bool     `json:"validate_origin"`
	AllowedOrigins      []string `json:"allowed_origins"`
}

// PerformanceThresholds defines performance monitoring thresholds
type PerformanceThresholds struct {
	SlowRequestThreshold time.Duration `json:"slow_request_threshold"`
	ErrorRateThreshold   float64       `json:"error_rate_threshold"`
	AlertOnBreach        bool          `json:"alert_on_breach"`
}

// EventFilters defines which events to audit
type EventFilters struct {
	IncludeEndpoints []string          `json:"include_endpoints"`
	ExcludeEndpoints []string          `json:"exclude_endpoints"`
	MinSeverity      audit.Severity    `json:"min_severity"`
	EventTypes       []audit.EventType `json:"event_types"`
}

// AuditLoggerInterface defines the contract for audit logging
type AuditLoggerInterface interface {
	LogEventWithRequest(ctx context.Context, request *http.Request,
		eventType audit.EventType, actorID, targetID, action, result string,
		metadata map[string]interface{}) error
}

// AuditMiddleware provides comprehensive audit functionality
type AuditMiddleware struct {
	config   AuditMiddlewareConfig
	logger   *zap.Logger
	tracer   trace.Tracer
	meter    metric.Meter
	
	// Rate limiting
	rateLimiters sync.Map
	
	// Metrics
	requestCounter    metric.Int64Counter
	requestDuration   metric.Float64Histogram
	errorCounter      metric.Int64Counter
	securityCounter   metric.Int64Counter
	rateLimitCounter  metric.Int64Counter
	
	// Performance monitoring
	performanceMonitor *PerformanceMonitor
}

// NewAuditMiddleware creates a new audit middleware instance
func NewAuditMiddleware(config AuditMiddlewareConfig, logger *zap.Logger) (*AuditMiddleware, error) {
	if config.AuditLogger == nil {
		return nil, errors.NewValidationError("MISSING_AUDIT_LOGGER", "audit logger is required")
	}

	tracer := otel.Tracer("audit.middleware")
	meter := otel.Meter("audit.middleware")

	// Initialize metrics
	requestCounter, err := meter.Int64Counter(
		"audit_middleware_requests_total",
		metric.WithDescription("Total number of requests processed by audit middleware"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create request counter: %w", err)
	}

	requestDuration, err := meter.Float64Histogram(
		"audit_middleware_request_duration_seconds",
		metric.WithDescription("Request duration in seconds"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create request duration histogram: %w", err)
	}

	errorCounter, err := meter.Int64Counter(
		"audit_middleware_errors_total",
		metric.WithDescription("Total number of errors in audit middleware"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create error counter: %w", err)
	}

	securityCounter, err := meter.Int64Counter(
		"audit_middleware_security_events_total",
		metric.WithDescription("Total number of security events"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create security counter: %w", err)
	}

	rateLimitCounter, err := meter.Int64Counter(
		"audit_middleware_rate_limit_total",
		metric.WithDescription("Total number of rate limit violations"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create rate limit counter: %w", err)
	}

	am := &AuditMiddleware{
		config:            config,
		logger:            logger,
		tracer:            tracer,
		meter:             meter,
		requestCounter:    requestCounter,
		requestDuration:   requestDuration,
		errorCounter:      errorCounter,
		securityCounter:   securityCounter,
		rateLimitCounter:  rateLimitCounter,
		performanceMonitor: NewPerformanceMonitor(config.PerformanceThresholds),
	}

	logger.Info("Audit middleware initialized",
		zap.Bool("enabled", config.Enabled),
		zap.Bool("audit_requests", config.AuditRequests),
		zap.Bool("audit_responses", config.AuditResponses),
		zap.Int("rate_limit_endpoints", len(config.RateLimits)),
	)

	return am, nil
}

// Middleware returns the HTTP middleware function
func (am *AuditMiddleware) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !am.config.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			am.processRequest(w, r, next)
		})
	}
}

// processRequest handles the complete audit middleware processing
func (am *AuditMiddleware) processRequest(w http.ResponseWriter, r *http.Request, next http.Handler) {
	ctx, span := am.tracer.Start(r.Context(), "audit.middleware",
		trace.WithAttributes(
			attribute.String("http.method", r.Method),
			attribute.String("http.path", r.URL.Path),
			attribute.String("http.remote_addr", r.RemoteAddr),
		),
	)
	defer span.End()

	start := time.Now()
	requestID := am.getOrCreateRequestID(r)

	// Enrich context with audit metadata
	ctx = am.enrichContext(ctx, r, requestID)
	r = r.WithContext(ctx)

	// Security validation
	if err := am.validateSecurity(ctx, r); err != nil {
		am.logSecurityEvent(ctx, r, "SECURITY_VALIDATION_FAILED", err.Error())
		am.writeSecurityError(w, err)
		return
	}

	// Rate limiting check
	if err := am.checkRateLimit(ctx, r); err != nil {
		am.logRateLimitEvent(ctx, r, err.Error())
		am.writeRateLimitError(w, err)
		return
	}

	// Audit request
	if am.config.AuditRequests {
		am.auditRequest(ctx, r)
	}

	// Create response wrapper for capturing response data
	wrapped := &auditResponseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
		body:          &bytes.Buffer{},
		headers:       make(http.Header),
	}

	// Process request
	next.ServeHTTP(wrapped, r)

	// Record metrics
	duration := time.Since(start)
	am.recordMetrics(ctx, r, wrapped.statusCode, duration)

	// Audit response
	if am.config.AuditResponses {
		am.auditResponse(ctx, r, wrapped, duration)
	}

	// Performance monitoring
	am.performanceMonitor.Record(r.URL.Path, duration, wrapped.statusCode >= 400)

	// Log completion
	span.SetAttributes(
		attribute.Int("http.status_code", wrapped.statusCode),
		attribute.Int64("http.response_size", int64(wrapped.body.Len())),
		attribute.Float64("duration_ms", float64(duration.Nanoseconds())/1e6),
	)
}

// validateSecurity performs security checks on the request
func (am *AuditMiddleware) validateSecurity(ctx context.Context, r *http.Request) error {
	checks := am.config.SecurityChecks

	// Content-Type validation
	if checks.ValidateContentType && len(checks.AllowedContentTypes) > 0 {
		contentType := r.Header.Get("Content-Type")
		if !am.isAllowedContentType(contentType, checks.AllowedContentTypes) {
			am.securityCounter.Add(ctx, 1, metric.WithAttributes(
				attribute.String("violation", "invalid_content_type"),
			))
			return errors.NewValidationError("INVALID_CONTENT_TYPE",
				fmt.Sprintf("content type %s not allowed", contentType))
		}
	}

	// Request size validation
	if checks.MaxRequestSize > 0 && r.ContentLength > checks.MaxRequestSize {
		am.securityCounter.Add(ctx, 1, metric.WithAttributes(
			attribute.String("violation", "request_too_large"),
		))
		return errors.NewValidationError("REQUEST_TOO_LARGE",
			fmt.Sprintf("request size %d exceeds limit %d", r.ContentLength, checks.MaxRequestSize))
	}

	// Origin validation
	if checks.ValidateOrigin && len(checks.AllowedOrigins) > 0 {
		origin := r.Header.Get("Origin")
		if origin != "" && !am.isAllowedOrigin(origin, checks.AllowedOrigins) {
			am.securityCounter.Add(ctx, 1, metric.WithAttributes(
				attribute.String("violation", "invalid_origin"),
			))
			return errors.NewValidationError("INVALID_ORIGIN",
				fmt.Sprintf("origin %s not allowed", origin))
		}
	}

	return nil
}

// checkRateLimit performs rate limiting checks
func (am *AuditMiddleware) checkRateLimit(ctx context.Context, r *http.Request) error {
	endpoint := am.getEndpointKey(r)
	rateLimitConfig, exists := am.config.RateLimits[endpoint]
	if !exists {
		// No rate limit configured for this endpoint
		return nil
	}

	key := am.buildRateLimitKey(r, rateLimitConfig)
	
	// Get or create rate limiter for this key
	limiterInterface, _ := am.rateLimiters.LoadOrStore(key, rate.NewLimiter(
		rate.Limit(rateLimitConfig.RequestsPerSecond),
		rateLimitConfig.Burst,
	))
	limiter := limiterInterface.(*rate.Limiter)

	if !limiter.Allow() {
		am.rateLimitCounter.Add(ctx, 1, metric.WithAttributes(
			attribute.String("endpoint", endpoint),
			attribute.String("key", key),
		))
		
		// Calculate retry after
		reservation := limiter.Reserve()
		retryAfter := reservation.Delay()
		reservation.Cancel()

		return &RateLimitError{
			Message:    "Rate limit exceeded",
			RetryAfter: retryAfter,
			Limit:      rateLimitConfig.RequestsPerSecond,
		}
	}

	return nil
}

// auditRequest logs request audit events
func (am *AuditMiddleware) auditRequest(ctx context.Context, r *http.Request) {
	if !am.shouldAuditEndpoint(r.URL.Path) {
		return
	}

	metadata := am.buildRequestMetadata(r)
	
	actorID := am.getActorID(ctx)
	targetID := am.getTargetID(r)
	action := fmt.Sprintf("%s %s", r.Method, r.URL.Path)

	err := am.config.AuditLogger.LogEventWithRequest(
		ctx,
		r,
		audit.EventTypeAPIRequest,
		actorID,
		targetID,
		action,
		"INITIATED",
		metadata,
	)

	if err != nil && !am.config.ContinueOnError {
		am.logger.Error("Failed to log audit event",
			zap.Error(err),
			zap.String("action", action),
		)
	}
}

// auditResponse logs response audit events
func (am *AuditMiddleware) auditResponse(ctx context.Context, r *http.Request, 
	wrapped *auditResponseWriter, duration time.Duration) {
	
	if !am.shouldAuditEndpoint(r.URL.Path) {
		return
	}

	metadata := am.buildResponseMetadata(r, wrapped, duration)
	
	actorID := am.getActorID(ctx)
	targetID := am.getTargetID(r)
	action := fmt.Sprintf("%s %s", r.Method, r.URL.Path)
	result := am.getResultFromStatus(wrapped.statusCode)

	err := am.config.AuditLogger.LogEventWithRequest(
		ctx,
		r,
		audit.EventTypeAPIResponse,
		actorID,
		targetID,
		action,
		result,
		metadata,
	)

	if err != nil && !am.config.ContinueOnError {
		am.logger.Error("Failed to log audit event",
			zap.Error(err),
			zap.String("action", action),
		)
	}
}

// enrichContext adds audit-specific metadata to the context
func (am *AuditMiddleware) enrichContext(ctx context.Context, r *http.Request, requestID string) context.Context {
	// Add request metadata
	ctx = context.WithValue(ctx, "audit.request_id", requestID)
	ctx = context.WithValue(ctx, "audit.client_ip", am.getClientIP(r))
	ctx = context.WithValue(ctx, "audit.user_agent", r.UserAgent())
	ctx = context.WithValue(ctx, "audit.start_time", time.Now())
	
	// Add session information if available
	if sessionID := am.getSessionID(r); sessionID != "" {
		ctx = context.WithValue(ctx, "audit.session_id", sessionID)
	}

	return ctx
}

// Helper methods

func (am *AuditMiddleware) getOrCreateRequestID(r *http.Request) string {
	requestID := r.Header.Get("X-Request-ID")
	if requestID == "" {
		requestID = uuid.New().String()
	}
	return requestID
}

func (am *AuditMiddleware) getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		return strings.TrimSpace(ips[0])
	}
	
	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	
	// Fall back to RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

func (am *AuditMiddleware) getSessionID(r *http.Request) string {
	// Try to get session ID from cookie or header
	if cookie, err := r.Cookie("session_id"); err == nil {
		return cookie.Value
	}
	return r.Header.Get("X-Session-ID")
}

func (am *AuditMiddleware) getActorID(ctx context.Context) string {
	// Try to get user ID from context (set by auth middleware)
	if userID := ctx.Value("user_id"); userID != nil {
		return fmt.Sprintf("%v", userID)
	}
	
	// Fall back to client IP
	if clientIP := ctx.Value("audit.client_ip"); clientIP != nil {
		return fmt.Sprintf("ip:%v", clientIP)
	}
	
	return "anonymous"
}

func (am *AuditMiddleware) getTargetID(r *http.Request) string {
	// Extract resource ID from path if available
	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	for i, part := range pathParts {
		if part == "calls" || part == "bids" || part == "accounts" {
			if i+1 < len(pathParts) {
				return pathParts[i+1]
			}
		}
	}
	
	return r.URL.Path
}

func (am *AuditMiddleware) getEndpointKey(r *http.Request) string {
	// Normalize endpoint path for rate limiting
	path := r.URL.Path
	method := r.Method
	
	// Replace resource IDs with placeholders
	normalizedPath := am.normalizeEndpointPath(path)
	
	return fmt.Sprintf("%s:%s", method, normalizedPath)
}

func (am *AuditMiddleware) normalizeEndpointPath(path string) string {
	// Replace UUIDs and numeric IDs with placeholders
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for i, part := range parts {
		if am.isUUID(part) || am.isNumeric(part) {
			parts[i] = "{id}"
		}
	}
	return "/" + strings.Join(parts, "/")
}

func (am *AuditMiddleware) isUUID(s string) bool {
	_, err := uuid.Parse(s)
	return err == nil
}

func (am *AuditMiddleware) isNumeric(s string) bool {
	_, err := strconv.ParseInt(s, 10, 64)
	return err == nil
}

func (am *AuditMiddleware) buildRateLimitKey(r *http.Request, config EndpointRateLimit) string {
	var parts []string
	
	if config.ByEndpoint {
		parts = append(parts, am.getEndpointKey(r))
	}
	
	if config.ByIP {
		parts = append(parts, am.getClientIP(r))
	}
	
	if config.ByUser {
		if userID := am.getActorID(r.Context()); userID != "anonymous" {
			parts = append(parts, userID)
		}
	}
	
	return strings.Join(parts, ":")
}

func (am *AuditMiddleware) shouldAuditEndpoint(path string) bool {
	filters := am.config.EventFilters
	
	// Check exclusions first
	for _, excluded := range filters.ExcludeEndpoints {
		if strings.HasPrefix(path, excluded) {
			return false
		}
	}
	
	// Check inclusions
	if len(filters.IncludeEndpoints) > 0 {
		for _, included := range filters.IncludeEndpoints {
			if strings.HasPrefix(path, included) {
				return true
			}
		}
		return false
	}
	
	return true
}

func (am *AuditMiddleware) buildRequestMetadata(r *http.Request) map[string]interface{} {
	metadata := map[string]interface{}{
		"method":       r.Method,
		"path":         r.URL.Path,
		"query":        r.URL.RawQuery,
		"content_type": r.Header.Get("Content-Type"),
		"user_agent":   r.UserAgent(),
		"referer":      r.Referer(),
		"content_length": r.ContentLength,
	}
	
	// Add specified headers
	if len(am.config.AuditHeaders) > 0 {
		headers := make(map[string]string)
		for _, headerName := range am.config.AuditHeaders {
			if value := r.Header.Get(headerName); value != "" {
				headers[headerName] = value
			}
		}
		if len(headers) > 0 {
			metadata["headers"] = headers
		}
	}
	
	// Add request body if it's JSON and not too large
	if am.shouldAuditRequestBody(r) {
		if body := am.captureRequestBody(r); body != nil {
			metadata["request_body"] = body
		}
	}
	
	return metadata
}

func (am *AuditMiddleware) buildResponseMetadata(r *http.Request, wrapped *auditResponseWriter, duration time.Duration) map[string]interface{} {
	metadata := map[string]interface{}{
		"status_code":     wrapped.statusCode,
		"response_size":   wrapped.body.Len(),
		"duration_ms":     duration.Milliseconds(),
		"content_type":    wrapped.Header().Get("Content-Type"),
	}
	
	// Add response body if it's JSON and not too large
	if am.shouldAuditResponseBody(wrapped) {
		if body := am.captureResponseBody(wrapped); body != nil {
			metadata["response_body"] = body
		}
	}
	
	return metadata
}

func (am *AuditMiddleware) shouldAuditRequestBody(r *http.Request) bool {
	contentType := r.Header.Get("Content-Type")
	return strings.Contains(contentType, "application/json") && 
		   r.ContentLength > 0 && 
		   r.ContentLength < 64*1024 // 64KB limit
}

func (am *AuditMiddleware) shouldAuditResponseBody(wrapped *auditResponseWriter) bool {
	contentType := wrapped.Header().Get("Content-Type")
	return strings.Contains(contentType, "application/json") && 
		   wrapped.body.Len() < 64*1024 // 64KB limit
}

func (am *AuditMiddleware) captureRequestBody(r *http.Request) interface{} {
	if r.Body == nil {
		return nil
	}
	
	// Read body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil
	}
	
	// Restore body for subsequent handlers
	r.Body = io.NopCloser(bytes.NewReader(body))
	
	// Parse JSON if possible
	var jsonBody interface{}
	if err := json.Unmarshal(body, &jsonBody); err == nil {
		return am.sanitizeData(jsonBody)
	}
	
	return string(body)
}

func (am *AuditMiddleware) captureResponseBody(wrapped *auditResponseWriter) interface{} {
	if wrapped.body.Len() == 0 {
		return nil
	}
	
	// Parse JSON if possible
	var jsonBody interface{}
	if err := json.Unmarshal(wrapped.body.Bytes(), &jsonBody); err == nil {
		return am.sanitizeData(jsonBody)
	}
	
	return wrapped.body.String()
}

func (am *AuditMiddleware) sanitizeData(data interface{}) interface{} {
	// Remove sensitive fields from audit data
	switch v := data.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{})
		for key, value := range v {
			if am.isSensitiveKey(key) {
				result[key] = "[REDACTED]"
			} else {
				result[key] = am.sanitizeData(value)
			}
		}
		return result
	case []interface{}:
		result := make([]interface{}, len(v))
		for i, item := range v {
			result[i] = am.sanitizeData(item)
		}
		return result
	default:
		return v
	}
}

func (am *AuditMiddleware) isSensitiveKey(key string) bool {
	lowerKey := strings.ToLower(key)
	for _, sensitiveKey := range am.config.SensitiveKeys {
		if strings.Contains(lowerKey, strings.ToLower(sensitiveKey)) {
			return true
		}
	}
	return false
}

func (am *AuditMiddleware) isAllowedContentType(contentType string, allowed []string) bool {
	for _, allowedType := range allowed {
		if strings.Contains(contentType, allowedType) {
			return true
		}
	}
	return false
}

func (am *AuditMiddleware) isAllowedOrigin(origin string, allowed []string) bool {
	for _, allowedOrigin := range allowed {
		if origin == allowedOrigin || allowedOrigin == "*" {
			return true
		}
	}
	return false
}

func (am *AuditMiddleware) getResultFromStatus(statusCode int) string {
	if statusCode >= 200 && statusCode < 300 {
		return "SUCCESS"
	} else if statusCode >= 400 && statusCode < 500 {
		return "CLIENT_ERROR"
	} else if statusCode >= 500 {
		return "SERVER_ERROR"
	}
	return "UNKNOWN"
}

func (am *AuditMiddleware) recordMetrics(ctx context.Context, r *http.Request, statusCode int, duration time.Duration) {
	labels := []attribute.KeyValue{
		attribute.String("method", r.Method),
		attribute.String("endpoint", am.normalizeEndpointPath(r.URL.Path)),
		attribute.String("status_class", fmt.Sprintf("%dxx", statusCode/100)),
	}
	
	am.requestCounter.Add(ctx, 1, metric.WithAttributes(labels...))
	am.requestDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(labels...))
	
	if statusCode >= 400 {
		am.errorCounter.Add(ctx, 1, metric.WithAttributes(labels...))
	}
}

func (am *AuditMiddleware) logSecurityEvent(ctx context.Context, r *http.Request, eventType, details string) {
	metadata := map[string]interface{}{
		"event_type": eventType,
		"details":    details,
		"method":     r.Method,
		"path":       r.URL.Path,
		"client_ip":  am.getClientIP(r),
		"user_agent": r.UserAgent(),
	}
	
	am.config.AuditLogger.LogEventWithRequest(
		ctx,
		r,
		audit.EventTypeSecurityIncident,
		am.getActorID(ctx),
		r.URL.Path,
		eventType,
		"BLOCKED",
		metadata,
	)
}

func (am *AuditMiddleware) logRateLimitEvent(ctx context.Context, r *http.Request, details string) {
	metadata := map[string]interface{}{
		"event_type": "RATE_LIMIT_EXCEEDED",
		"details":    details,
		"method":     r.Method,
		"path":       r.URL.Path,
		"client_ip":  am.getClientIP(r),
		"endpoint":   am.getEndpointKey(r),
	}
	
	am.config.AuditLogger.LogEventWithRequest(
		ctx,
		r,
		audit.EventTypeRateLimitExceeded,
		am.getActorID(ctx),
		r.URL.Path,
		"RATE_LIMIT_CHECK",
		"EXCEEDED",
		metadata,
	)
}

// Error response writers

func (am *AuditMiddleware) writeSecurityError(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	
	response := map[string]interface{}{
		"error": map[string]interface{}{
			"code":    "SECURITY_VIOLATION",
			"message": "Request blocked by security policy",
			"details": err.Error(),
		},
	}
	
	json.NewEncoder(w).Encode(response)
}

func (am *AuditMiddleware) writeRateLimitError(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")
	
	if rateLimitErr, ok := err.(*RateLimitError); ok {
		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(rateLimitErr.Limit))
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.Header().Set("Retry-After", fmt.Sprintf("%.0f", rateLimitErr.RetryAfter.Seconds()))
	}
	
	w.WriteHeader(http.StatusTooManyRequests)
	
	response := map[string]interface{}{
		"error": map[string]interface{}{
			"code":    "RATE_LIMIT_EXCEEDED",
			"message": "Too many requests",
			"details": err.Error(),
		},
	}
	
	json.NewEncoder(w).Encode(response)
}

// auditResponseWriter captures response data for auditing
type auditResponseWriter struct {
	http.ResponseWriter
	statusCode int
	body       *bytes.Buffer
	headers    http.Header
}

func (w *auditResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *auditResponseWriter) Write(data []byte) (int, error) {
	// Capture response body for auditing
	w.body.Write(data)
	return w.ResponseWriter.Write(data)
}

func (w *auditResponseWriter) Header() http.Header {
	return w.ResponseWriter.Header()
}

// RateLimitError represents a rate limiting error
type RateLimitError struct {
	Message    string
	RetryAfter time.Duration
	Limit      int
}

func (e *RateLimitError) Error() string {
	return e.Message
}

// PerformanceMonitor tracks performance metrics
type PerformanceMonitor struct {
	thresholds PerformanceThresholds
	mutex      sync.RWMutex
	stats      map[string]*EndpointStats
}

type EndpointStats struct {
	TotalRequests int64
	ErrorCount    int64
	TotalDuration time.Duration
	MaxDuration   time.Duration
}

func NewPerformanceMonitor(thresholds PerformanceThresholds) *PerformanceMonitor {
	return &PerformanceMonitor{
		thresholds: thresholds,
		stats:      make(map[string]*EndpointStats),
	}
}

func (pm *PerformanceMonitor) Record(endpoint string, duration time.Duration, isError bool) {
	pm.mutex.Lock()
	defer pm.mutex.Unlock()
	
	stats, exists := pm.stats[endpoint]
	if !exists {
		stats = &EndpointStats{}
		pm.stats[endpoint] = stats
	}
	
	stats.TotalRequests++
	stats.TotalDuration += duration
	
	if duration > stats.MaxDuration {
		stats.MaxDuration = duration
	}
	
	if isError {
		stats.ErrorCount++
	}
}

// DefaultAuditMiddlewareConfig returns a default configuration
func DefaultAuditMiddlewareConfig() AuditMiddlewareConfig {
	return AuditMiddlewareConfig{
		Enabled:        true,
		AuditRequests:  true,
		AuditResponses true,
		AuditHeaders:   []string{"Authorization", "X-API-Key", "X-Request-ID"},
		SensitiveKeys:  []string{"password", "token", "secret", "key", "auth", "credential"},
		SecurityChecks: SecurityChecks{
			ValidateContentType: true,
			AllowedContentTypes: []string{"application/json", "application/x-www-form-urlencoded"},
			MaxRequestSize:      10 * 1024 * 1024, // 10MB
			RequireAuth:         true,
			ValidateOrigin:      false,
		},
		PerformanceThresholds: PerformanceThresholds{
			SlowRequestThreshold: 5 * time.Second,
			ErrorRateThreshold:   0.05, // 5%
			AlertOnBreach:        true,
		},
		EventFilters: EventFilters{
			ExcludeEndpoints: []string{"/health", "/metrics", "/ready"},
			MinSeverity:      audit.SeverityLow,
		},
		ContinueOnError: true,
		RateLimits: map[string]EndpointRateLimit{
			"POST:/api/v1/audit/events": {
				RequestsPerSecond: 100,
				Burst:             200,
				Window:            time.Minute,
				ByIP:              true,
				ByUser:            true,
			},
			"GET:/api/v1/audit/events": {
				RequestsPerSecond: 50,
				Burst:             100,
				Window:            time.Minute,
				ByUser:            true,
			},
		},
	}
}