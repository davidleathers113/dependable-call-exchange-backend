package rest

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"runtime/debug"
	"strings"

	domainErrors "github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// ErrorHandler provides sophisticated error handling with proper status codes and messages
type ErrorHandler interface {
	HandleError(err error) (status int, code, message, details string)
	HandlePanic(recovered interface{}) (status int, code, message, details string)
	IsRetryable(err error) bool
	SuggestRetryAfter(err error) *int
}

// DefaultErrorHandler implements ErrorHandler with comprehensive error mapping
type DefaultErrorHandler struct {
	debugMode bool
	tracer    trace.Tracer
}

// NewErrorHandler creates a new error handler
func NewErrorHandler() ErrorHandler {
	return &DefaultErrorHandler{
		debugMode: false, // Set from config in production
		tracer:    trace.SpanFromContext(context.Background()).TracerProvider().Tracer("api.rest.errors"),
	}
}

// HandleError converts various error types to HTTP responses
func (h *DefaultErrorHandler) HandleError(err error) (status int, code, message, details string) {
	if err == nil {
		return http.StatusOK, "", "", ""
	}

	// Log error with trace
	span := trace.SpanFromContext(context.Background())
	span.RecordError(err, trace.WithAttributes(
		attribute.String("error.type", fmt.Sprintf("%T", err)),
		attribute.String("error.message", err.Error()),
	))

	// Handle wrapped errors
	err = h.unwrapError(err)

	// Domain errors
	var domainErr *domainErrors.AppError
	if errors.As(err, &domainErr) {
		return h.handleDomainError(domainErr)
	}

	// Validation errors (check for our custom ValidationError)
	var validationErr *ValidationError
	if errors.As(err, &validationErr) {
		return h.handleValidationError(validationErr)
	}

	// Database errors
	if errors.Is(err, sql.ErrNoRows) {
		return http.StatusNotFound, "NOT_FOUND", "Resource not found", ""
	}

	// Context errors
	if errors.Is(err, context.Canceled) {
		return http.StatusRequestTimeout, "REQUEST_CANCELED", "Request was canceled", ""
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return http.StatusRequestTimeout, "REQUEST_TIMEOUT", "Request timed out", ""
	}

	// JSON errors
	var jsonErr *json.SyntaxError
	if errors.As(err, &jsonErr) {
		return http.StatusBadRequest, "INVALID_JSON", "Invalid JSON syntax", 
			fmt.Sprintf("Error at position %d", jsonErr.Offset)
	}

	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) {
		return http.StatusBadRequest, "TYPE_MISMATCH", 
			fmt.Sprintf("Invalid type for field '%s'", typeErr.Field),
			fmt.Sprintf("Expected %s but got %s", typeErr.Type, typeErr.Value)
	}

	// Network errors
	if h.isNetworkError(err) {
		return http.StatusBadGateway, "UPSTREAM_ERROR", "Upstream service unavailable", ""
	}

	// Default to internal server error
	details = ""
	if h.debugMode {
		details = err.Error()
	}
	
	return http.StatusInternalServerError, "INTERNAL_ERROR", "An internal error occurred", details
}

// HandlePanic converts panic recovery to error response
func (h *DefaultErrorHandler) HandlePanic(recovered interface{}) (status int, code, message, details string) {
	// Log panic with stack trace
	span := trace.SpanFromContext(context.Background())
	span.RecordError(fmt.Errorf("panic: %v", recovered), trace.WithAttributes(
		attribute.String("panic.type", fmt.Sprintf("%T", recovered)),
		attribute.String("panic.stack", string(debug.Stack())),
	))

	message = "An unexpected error occurred"
	code = "PANIC"
	status = http.StatusInternalServerError
	
	if h.debugMode {
		details = fmt.Sprintf("Panic: %v\n\nStack trace:\n%s", recovered, debug.Stack())
	}
	
	return
}

// IsRetryable determines if an error is retryable
func (h *DefaultErrorHandler) IsRetryable(err error) bool {
	// Unwrap to get root cause
	err = h.unwrapError(err)

	// Check for explicit retryable errors
	var domainErr *domainErrors.AppError
	if errors.As(err, &domainErr) && domainErr.Retryable {
		return true
	}

	// Context deadline exceeded is retryable
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	// Network errors are often retryable
	if h.isNetworkError(err) {
		return true
	}

	// Database deadlocks are retryable
	if h.isDatabaseDeadlock(err) {
		return true
	}

	return false
}

// SuggestRetryAfter suggests retry delay in seconds
func (h *DefaultErrorHandler) SuggestRetryAfter(err error) *int {
	// Rate limit errors
	var domainErr *domainErrors.AppError
	if errors.As(err, &domainErr) {
		if domainErr.Code == "RATE_LIMIT_EXCEEDED" {
			delay := 60 // Default 60 seconds
			return &delay
		}
	}

	// For retryable errors, use exponential backoff hint
	if h.IsRetryable(err) {
		delay := 5 // Start with 5 seconds
		return &delay
	}

	return nil
}

// Private helper methods

func (h *DefaultErrorHandler) handleDomainError(err *domainErrors.AppError) (int, string, string, string) {
	status := err.StatusCode
	code := err.Code
	
	details := ""
	if h.debugMode && err.Details != nil {
		detailBytes, _ := json.Marshal(err.Details)
		details = string(detailBytes)
	}
	
	return status, code, err.Message, details
}

func (h *DefaultErrorHandler) handleValidationError(err *ValidationError) (int, string, string, string) {
	details := ""
	if len(err.Fields) > 0 {
		var fieldErrors []string
		for field, messages := range err.Fields {
			fieldErrors = append(fieldErrors, fmt.Sprintf("%s: %s", field, strings.Join(messages, "; ")))
		}
		details = strings.Join(fieldErrors, ", ")
	}
	
	return http.StatusBadRequest, "VALIDATION_ERROR", err.Message, details
}

// Removed errorTypeToStatus as we now use StatusCode from AppError directly

func (h *DefaultErrorHandler) unwrapError(err error) error {
	for {
		unwrapped := errors.Unwrap(err)
		if unwrapped == nil {
			return err
		}
		err = unwrapped
	}
}

func (h *DefaultErrorHandler) isNetworkError(err error) bool {
	errStr := err.Error()
	networkErrors := []string{
		"connection refused",
		"no such host",
		"network is unreachable",
		"connection reset by peer",
		"broken pipe",
		"timeout",
	}
	
	for _, netErr := range networkErrors {
		if strings.Contains(strings.ToLower(errStr), netErr) {
			return true
		}
	}
	
	return false
}

func (h *DefaultErrorHandler) isDatabaseDeadlock(err error) bool {
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "deadlock") || 
		strings.Contains(errStr, "lock timeout") ||
		strings.Contains(errStr, "serialization failure")
}

// ErrorInterceptor provides a middleware for error interception and transformation
type ErrorInterceptor struct {
	handler     ErrorHandler
	transformers []ErrorTransformer
}

// ErrorTransformer allows custom error transformation
type ErrorTransformer func(err error) error

// NewErrorInterceptor creates a new error interceptor
func NewErrorInterceptor(handler ErrorHandler) *ErrorInterceptor {
	return &ErrorInterceptor{
		handler:      handler,
		transformers: make([]ErrorTransformer, 0),
	}
}

// AddTransformer adds an error transformer
func (i *ErrorInterceptor) AddTransformer(transformer ErrorTransformer) {
	i.transformers = append(i.transformers, transformer)
}

// InterceptError applies transformations and handles the error
func (i *ErrorInterceptor) InterceptError(err error) (status int, code, message, details string) {
	// Apply transformations
	for _, transformer := range i.transformers {
		if transformed := transformer(err); transformed != nil {
			err = transformed
		}
	}
	
	return i.handler.HandleError(err)
}

// Common error transformers

// SanitizeErrorTransformer removes sensitive information from errors
func SanitizeErrorTransformer(err error) error {
	errStr := err.Error()
	
	// Remove potential sensitive patterns
	patterns := []string{
		`password[\s]*=[\s]*['"][^'"]+['"]`,
		`token[\s]*=[\s]*['"][^'"]+['"]`,
		`key[\s]*=[\s]*['"][^'"]+['"]`,
		`secret[\s]*=[\s]*['"][^'"]+['"]`,
	}
	
	for _, pattern := range patterns {
		errStr = strings.ReplaceAll(errStr, pattern, "[REDACTED]")
	}
	
	if errStr != err.Error() {
		return errors.New(errStr)
	}
	
	return err
}

// EnrichErrorTransformer adds context to errors
func EnrichErrorTransformer(ctx context.Context) ErrorTransformer {
	return func(err error) error {
		// Add request ID to error if available
		if meta, ok := ctx.Value(contextKeyRequestMeta).(*RequestMeta); ok {
			return fmt.Errorf("[RequestID: %s] %w", meta.RequestID, err)
		}
		return err
	}
}