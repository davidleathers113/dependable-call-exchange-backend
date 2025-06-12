package rest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"runtime"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// RequestHandler processes HTTP requests with type safety
type RequestHandler interface {
	Handle(w http.ResponseWriter, r *http.Request)
}

// RequestMeta contains metadata about the current request
type RequestMeta struct {
	RequestID    string
	UserID       uuid.UUID
	AccountType  string
	TraceID      string
	SpanID       string
	APIVersion   string
	ClientIP     string
	UserAgent    string
	AcceptHeader string
}

// ResponseEnvelope wraps all API responses
type ResponseEnvelope struct {
	Success   bool               `json:"success"`
	Data      interface{}        `json:"data,omitempty"`
	Error     *ErrorResponse     `json:"error,omitempty"`
	Meta      ResponseMeta       `json:"meta"`
	Links     map[string]string  `json:"_links,omitempty"`
}

// ResponseMeta contains response metadata
type ResponseMeta struct {
	RequestID    string    `json:"request_id"`
	Timestamp    time.Time `json:"timestamp"`
	Version      string    `json:"version"`
	ResponseTime string    `json:"response_time,omitempty"`
}

// ErrorResponse provides detailed error information
type ErrorResponse struct {
	Code       string                 `json:"code"`
	Message    string                 `json:"message"`
	Details    string                 `json:"details,omitempty"`
	Fields     map[string][]string    `json:"fields,omitempty"`
	TraceID    string                 `json:"trace_id,omitempty"`
	HelpURL    string                 `json:"help_url,omitempty"`
	RetryAfter *time.Duration         `json:"retry_after,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// BaseHandler provides common functionality for all handlers
type BaseHandler struct {
	validator    *validator.Validate
	tracer       trace.Tracer
	errorHandler ErrorHandler
	apiVersion   string
	baseURL      string
}

// NewBaseHandler creates a new base handler with all the goodies
func NewBaseHandler(apiVersion, baseURL string) *BaseHandler {
	v := validator.New()
	
	// Register custom validators
	v.RegisterValidation("phone", validatePhoneNumber)
	v.RegisterValidation("money", validateMoney)
	v.RegisterValidation("uuid", validateUUID)
	v.RegisterValidation("e164", validateE164)
	v.RegisterValidation("timezone", validateTimezone)
	v.RegisterValidation("iso4217", validateISO4217)
	v.RegisterValidation("jwt", validateJWT)
	v.RegisterValidation("cron", validateCron)
	v.RegisterValidation("datetime", validateDateTime)
	
	return &BaseHandler{
		validator:    v,
		tracer:       otel.Tracer("api.rest"),
		errorHandler: NewErrorHandler(),
		apiVersion:   apiVersion,
		baseURL:      baseURL,
	}
}

// WrapHandler creates a type-safe handler wrapper
func (h *BaseHandler) WrapHandler(
	method, pattern string,
	handler func(context.Context, *http.Request) (interface{}, error),
	opts ...HandlerOption,
) http.HandlerFunc {
	// Apply options
	config := &handlerConfig{
		maxBodySize:      1 << 20, // 1MB default
		timeout:          30 * time.Second,
		requireAuth:      true,
		rateLimit:        100,
		rateLimitWindow:  time.Minute,
		cacheDuration:    0,
		validateRequest:  true,
		validateResponse: true,
	}
	
	for _, opt := range opts {
		opt(config)
	}
	
	// Get handler name for tracing
	handlerName := runtime.FuncForPC(reflect.ValueOf(handler).Pointer()).Name()
	
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		
		// Create span
		ctx, span := h.tracer.Start(r.Context(), fmt.Sprintf("%s %s", method, pattern),
			trace.WithAttributes(
				attribute.String("http.method", method),
				attribute.String("http.url", r.URL.String()),
				attribute.String("handler.name", handlerName),
			),
		)
		defer span.End()
		
		// Enhanced response writer
		rw := &responseWriter{
			ResponseWriter: w,
			statusCode:     200,
			written:        false,
			startTime:      start,
		}
		
		// Extract request metadata
		meta := h.extractRequestMeta(r)
		ctx = context.WithValue(ctx, contextKeyRequestMeta, meta)
		
		// Apply timeout
		if config.timeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, config.timeout)
			defer cancel()
		}
		
		// Check rate limit
		if config.requireAuth && config.rateLimit > 0 {
			if !h.checkRateLimit(ctx, meta.UserID, config.rateLimit, config.rateLimitWindow) {
				h.writeError(rw, http.StatusTooManyRequests, "RATE_LIMIT_EXCEEDED",
					"Too many requests", "", map[string]interface{}{
						"retry_after": config.rateLimitWindow.Seconds(),
					})
				return
			}
		}
		
		// Update request with new context
		r = r.WithContext(ctx)
		
		// Call the handler
		res, err := handler(ctx, r)
		if err != nil {
			h.handleError(rw, err)
			return
		}
		
		// Validate response if configured
		if config.validateResponse && res != nil {
			if err := h.validator.Struct(res); err != nil {
				span.RecordError(err)
				h.writeError(rw, http.StatusInternalServerError, "RESPONSE_VALIDATION_FAILED",
					"Internal validation error", "", nil)
				return
			}
		}
		
		// Write success response
		h.writeSuccess(rw, http.StatusOK, res, meta)
	}
}

// JSONHandler creates a handler that expects JSON input and returns JSON output
func (h *BaseHandler) JSONHandler(
	handler func(context.Context, json.RawMessage) (interface{}, error),
	opts ...HandlerOption,
) func(context.Context, *http.Request) (interface{}, error) {
	return func(ctx context.Context, r *http.Request) (interface{}, error) {
		// Apply body size limit
		config := &handlerConfig{maxBodySize: 1 << 20} // 1MB default
		for _, opt := range opts {
			opt(config)
		}
		
		r.Body = http.MaxBytesReader(nil, r.Body, config.maxBodySize)
		
		// Read body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			return nil, h.parseBodyError(err, config.maxBodySize)
		}
		
		// Validate content type
		contentType := r.Header.Get("Content-Type")
		if !strings.HasPrefix(contentType, "application/json") && len(body) > 0 {
			return nil, &ValidationError{Message: "Content-Type must be application/json"}
		}
		
		return handler(ctx, json.RawMessage(body))
	}
}

// ParseAndValidate parses JSON and validates the structure
func (h *BaseHandler) ParseAndValidate(data json.RawMessage, v interface{}) error {
	if err := json.Unmarshal(data, v); err != nil {
		return &ValidationError{
			Message: "Invalid JSON",
			Details: err.Error(),
		}
	}
	
	if err := h.validator.Struct(v); err != nil {
		return h.formatValidationError(err)
	}
	
	return nil
}

// parseBodyError converts body reading errors to validation errors
func (h *BaseHandler) parseBodyError(err error, maxSize int64) error {
	if errors.Is(err, http.ErrBodyReadAfterClose) {
		return &ValidationError{Message: "Request body already read"}
	}
	
	var maxBytesError *http.MaxBytesError
	if errors.As(err, &maxBytesError) {
		return &ValidationError{
			Message: fmt.Sprintf("Request body too large (max %d bytes)", maxSize),
		}
	}
	
	return &ValidationError{Message: "Failed to read request body"}
}

// formatValidationError converts validator errors to our format
func (h *BaseHandler) formatValidationError(err error) error {
	var validationErrors validator.ValidationErrors
	if errors.As(err, &validationErrors) {
		fields := make(map[string][]string)
		
		for _, fe := range validationErrors {
			field := fe.Field()
			tag := fe.Tag()
			param := fe.Param()
			
			// Create human-readable error message
			var msg string
			switch tag {
			case "required":
				msg = "This field is required"
			case "min":
				msg = fmt.Sprintf("Minimum value is %s", param)
			case "max":
				msg = fmt.Sprintf("Maximum value is %s", param)
			case "email":
				msg = "Must be a valid email address"
			case "phone":
				msg = "Must be a valid phone number"
			case "e164":
				msg = "Must be a valid E.164 phone number (e.g., +1234567890)"
			case "uuid":
				msg = "Must be a valid UUID"
			case "oneof":
				msg = fmt.Sprintf("Must be one of: %s", param)
			case "gtfield":
				msg = fmt.Sprintf("Must be greater than %s", param)
			case "ltfield":
				msg = fmt.Sprintf("Must be less than %s", param)
			case "eqfield":
				msg = fmt.Sprintf("Must equal %s", param)
			case "nefield":
				msg = fmt.Sprintf("Must not equal %s", param)
			case "timezone":
				msg = "Must be a valid timezone (e.g., America/New_York)"
			case "iso4217":
				msg = "Must be a valid ISO 4217 currency code"
			case "jwt":
				msg = "Must be a valid JWT token"
			case "cron":
				msg = "Must be a valid cron expression"
			case "datetime":
				msg = fmt.Sprintf("Must be a valid datetime in format %s", param)
			default:
				msg = fmt.Sprintf("Failed %s validation", tag)
			}
			
			fields[field] = append(fields[field], msg)
		}
		
		return &ValidationError{
			Message: "Validation failed",
			Fields:  fields,
		}
	}
	
	return &ValidationError{
		Message: "Validation error",
		Details: err.Error(),
	}
}

// writeSuccess writes a successful response
func (h *BaseHandler) writeSuccess(w http.ResponseWriter, status int, data interface{}, meta *RequestMeta) {
	elapsed := time.Since(w.(*responseWriter).startTime)
	
	response := ResponseEnvelope{
		Success: true,
		Data:    data,
		Meta: ResponseMeta{
			RequestID:    meta.RequestID,
			Timestamp:    time.Now().UTC(),
			Version:      h.apiVersion,
			ResponseTime: elapsed.String(),
		},
	}
	
	// Add HATEOAS links if applicable
	if linker, ok := data.(interface{ Links() map[string]string }); ok {
		response.Links = linker.Links()
	}
	
	h.writeJSON(w, status, response)
}

// writeError writes an error response
func (h *BaseHandler) writeError(w http.ResponseWriter, status int, code, message, details string, metadata map[string]interface{}) {
	rw, ok := w.(*responseWriter)
	if !ok {
		// Fallback if not our response writer
		rw = &responseWriter{
			ResponseWriter: w,
			startTime:      time.Now(),
		}
	}
	
	meta := &RequestMeta{RequestID: uuid.New().String()}
	if rw.request != nil {
		meta = h.getRequestMeta(rw.request.Context())
	}
	
	elapsed := time.Since(rw.startTime)
	
	errorResp := &ErrorResponse{
		Code:     code,
		Message:  message,
		Details:  details,
		TraceID:  meta.TraceID,
		HelpURL:  fmt.Sprintf("%s/docs/errors/%s", h.baseURL, strings.ToLower(code)),
		Metadata: metadata,
	}
	
	// Add retry-after for rate limits
	if status == http.StatusTooManyRequests {
		retryAfter := time.Minute
		errorResp.RetryAfter = &retryAfter
		w.Header().Set("Retry-After", "60")
	}
	
	response := ResponseEnvelope{
		Success: false,
		Error:   errorResp,
		Meta: ResponseMeta{
			RequestID:    meta.RequestID,
			Timestamp:    time.Now().UTC(),
			Version:      h.apiVersion,
			ResponseTime: elapsed.String(),
		},
	}
	
	h.writeJSON(w, status, response)
}

// writeJSON writes JSON response with proper headers
func (h *BaseHandler) writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(true)
	
	if err := encoder.Encode(v); err != nil {
		// Can't write error response, log it
		span := trace.SpanFromContext(context.Background())
		span.RecordError(err, trace.WithAttributes(
			attribute.String("error", err.Error()),
		))
	}
}

// handleError converts domain errors to HTTP responses
func (h *BaseHandler) handleError(w http.ResponseWriter, err error) {
	status, code, message, details := h.errorHandler.HandleError(err)
	h.writeError(w, status, code, message, details, nil)
}

// Helper methods

func (h *BaseHandler) extractRequestMeta(r *http.Request) *RequestMeta {
	meta := &RequestMeta{
		RequestID:    r.Header.Get("X-Request-ID"),
		APIVersion:   h.extractAPIVersion(r),
		ClientIP:     h.extractClientIP(r),
		UserAgent:    r.UserAgent(),
		AcceptHeader: r.Header.Get("Accept"),
	}
	
	if meta.RequestID == "" {
		meta.RequestID = uuid.New().String()
	}
	
	// Extract from context (set by auth middleware)
	if userID, ok := r.Context().Value(contextKeyUserID).(uuid.UUID); ok {
		meta.UserID = userID
	}
	if accountType, ok := r.Context().Value(contextKeyAccountType).(string); ok {
		meta.AccountType = accountType
	}
	
	// Extract trace info
	if span := trace.SpanFromContext(r.Context()); span.SpanContext().IsValid() {
		meta.TraceID = span.SpanContext().TraceID().String()
		meta.SpanID = span.SpanContext().SpanID().String()
	}
	
	return meta
}

func (h *BaseHandler) getRequestMeta(ctx context.Context) *RequestMeta {
	if meta, ok := ctx.Value(contextKeyRequestMeta).(*RequestMeta); ok {
		return meta
	}
	return &RequestMeta{RequestID: uuid.New().String()}
}

func (h *BaseHandler) extractAPIVersion(r *http.Request) string {
	// Check header first
	if v := r.Header.Get("API-Version"); v != "" {
		return v
	}
	
	// Extract from URL path
	parts := strings.Split(r.URL.Path, "/")
	for i, part := range parts {
		if part == "api" && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	
	return h.apiVersion
}

func (h *BaseHandler) extractClientIP(r *http.Request) string {
	// Check X-Forwarded-For first
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		return strings.TrimSpace(ips[0])
	}
	
	// Check X-Real-IP
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	
	// Fall back to RemoteAddr
	ip := r.RemoteAddr
	if colon := strings.LastIndex(ip, ":"); colon != -1 {
		ip = ip[:colon]
	}
	
	return ip
}

func (h *BaseHandler) checkRateLimit(ctx context.Context, userID uuid.UUID, limit int, window time.Duration) bool {
	// This would integrate with Redis or similar
	// For now, return true
	return true
}

// Custom validators

func validatePhoneNumber(fl validator.FieldLevel) bool {
	phone := fl.Field().String()
	// Basic phone validation
	return len(phone) >= 10 && len(phone) <= 15
}

func validateE164(fl validator.FieldLevel) bool {
	phone := fl.Field().String()
	// E.164 validation
	return strings.HasPrefix(phone, "+") && len(phone) >= 10 && len(phone) <= 15
}

func validateMoney(fl validator.FieldLevel) bool {
	// Validate money format
	return true
}

func validateUUID(fl validator.FieldLevel) bool {
	_, err := uuid.Parse(fl.Field().String())
	return err == nil
}

func validateTimezone(fl validator.FieldLevel) bool {
	tz := fl.Field().String()
	_, err := time.LoadLocation(tz)
	return err == nil
}

func validateISO4217(fl validator.FieldLevel) bool {
	// Simple validation for common currencies
	currency := fl.Field().String()
	validCurrencies := []string{"USD", "EUR", "GBP", "JPY", "CAD", "AUD", "CHF", "CNY"}
	for _, valid := range validCurrencies {
		if currency == valid {
			return true
		}
	}
	return false
}

func validateJWT(fl validator.FieldLevel) bool {
	token := fl.Field().String()
	// Basic JWT format validation
	parts := strings.Split(token, ".")
	return len(parts) == 3
}

func validateCron(fl validator.FieldLevel) bool {
	// Basic cron validation
	cron := fl.Field().String()
	parts := strings.Fields(cron)
	return len(parts) == 5 || len(parts) == 6
}

func validateDateTime(fl validator.FieldLevel) bool {
	format := fl.Param()
	value := fl.Field().String()
	_, err := time.Parse(format, value)
	return err == nil
}

// Context keys
type contextKey string

const (
	contextKeyRequestMeta contextKey = "request_meta"
	contextKeyUserID      contextKey = "user_id"
	contextKeyAccountType contextKey = "account_type"
)

// HandlerOption configures handler behavior
type HandlerOption func(*handlerConfig)

type handlerConfig struct {
	maxBodySize      int64
	timeout          time.Duration
	requireAuth      bool
	rateLimit        int
	rateLimitWindow  time.Duration
	cacheDuration    time.Duration
	validateRequest  bool
	validateResponse bool
}

// Handler options
func WithMaxBodySize(size int64) HandlerOption {
	return func(c *handlerConfig) { c.maxBodySize = size }
}

func WithTimeout(d time.Duration) HandlerOption {
	return func(c *handlerConfig) { c.timeout = d }
}

func WithoutAuth() HandlerOption {
	return func(c *handlerConfig) { c.requireAuth = false }
}

func WithRateLimit(limit int, window time.Duration) HandlerOption {
	return func(c *handlerConfig) {
		c.rateLimit = limit
		c.rateLimitWindow = window
	}
}

func WithCache(duration time.Duration) HandlerOption {
	return func(c *handlerConfig) { c.cacheDuration = duration }
}

func WithoutValidation() HandlerOption {
	return func(c *handlerConfig) {
		c.validateRequest = false
		c.validateResponse = false
	}
}

// ValidationError represents a validation error
type ValidationError struct {
	Message string
	Details string
	Fields  map[string][]string
}

func (e *ValidationError) Error() string {
	return e.Message
}

// Enhanced response writer
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
	startTime  time.Time
	request    *http.Request
}

func (rw *responseWriter) WriteHeader(code int) {
	if !rw.written {
		rw.statusCode = code
		rw.ResponseWriter.WriteHeader(code)
		rw.written = true
	}
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.written {
		rw.WriteHeader(http.StatusOK)
	}
	return rw.ResponseWriter.Write(b)
}