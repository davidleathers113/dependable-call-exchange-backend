package rest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/consent"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
	consentService "github.com/davidleathers/dependable-call-exchange-backend/internal/service/consent"
	"github.com/google/uuid"
	"github.com/go-playground/validator/v10"
)

// ConsentHandler handles consent management API endpoints
type ConsentHandler struct {
	*BaseHandler
	consentService consentService.Service
	logger         *slog.Logger
	validator      *validator.Validate
}

// NewConsentHandler creates a new consent handler
func NewConsentHandler(baseHandler *BaseHandler, service consentService.Service, logger *slog.Logger) *ConsentHandler {
	validator := validator.New()
	
	return &ConsentHandler{
		BaseHandler:    baseHandler,
		consentService: service,
		logger:         logger,
		validator:      validator,
	}
}

// RegisterConsentRoutes registers all consent-related routes
func (h *ConsentHandler) RegisterConsentRoutes(mux *http.ServeMux) {
	// Create middleware chain for protected endpoints
	authChain := NewMiddlewareChain(
		SecurityHeadersMiddleware(),
		RequestIDMiddleware(),
		RequestLoggingMiddleware(h.logger),
		MetricsMiddleware(),
		ContentNegotiationMiddleware(),
		// AuthMiddleware would be added here
	)

	// Rate limiter for consent operations
	consentRateLimiter := NewRateLimiter(RateLimitConfig{
		RequestsPerSecond: 50,
		Burst:             100,
		ByUser:            true,
	})

	// Consent management endpoints
	mux.Handle("POST /api/v1/consents", authChain.Then(
		consentRateLimiter.Middleware()(
			h.WrapHandler("POST", "/api/v1/consents", h.handleGrantConsent,
				WithMaxBodySize(10<<10), // 10KB
				WithTimeout(30*time.Second),
			),
		),
	))

	mux.Handle("GET /api/v1/consents/{id}", authChain.Then(
		consentRateLimiter.Middleware()(
			h.WrapHandler("GET", "/api/v1/consents/{id}", h.handleGetConsent,
				WithCache(5*time.Minute),
			),
		),
	))

	mux.Handle("PUT /api/v1/consents/{id}", authChain.Then(
		consentRateLimiter.Middleware()(
			h.WrapHandler("PUT", "/api/v1/consents/{id}", h.handleUpdateConsent,
				WithMaxBodySize(10<<10),
				WithTimeout(30*time.Second),
			),
		),
	))

	mux.Handle("DELETE /api/v1/consents/{id}", authChain.Then(
		consentRateLimiter.Middleware()(
			h.WrapHandler("DELETE", "/api/v1/consents/{id}", h.handleRevokeConsent,
				WithTimeout(30*time.Second),
			),
		),
	))

	// Consent verification endpoints
	mux.Handle("POST /api/v1/consent/verify", authChain.Then(
		consentRateLimiter.Middleware()(
			h.WrapHandler("POST", "/api/v1/consent/verify", h.handleVerifyConsent,
				WithMaxBodySize(1<<10), // 1KB
				WithTimeout(5*time.Second), // Fast verification
			),
		),
	))

	mux.Handle("POST /api/v1/consent/verify/batch", authChain.Then(
		consentRateLimiter.Middleware()(
			h.WrapHandler("POST", "/api/v1/consent/verify/batch", h.handleVerifyConsentBatch,
				WithMaxBodySize(100<<10), // 100KB for batch
				WithTimeout(60*time.Second),
			),
		),
	))

	// Consumer management endpoints
	mux.Handle("POST /api/v1/consumers", authChain.Then(
		consentRateLimiter.Middleware()(
			h.WrapHandler("POST", "/api/v1/consumers", h.handleCreateConsumer,
				WithMaxBodySize(5<<10), // 5KB
				WithTimeout(30*time.Second),
			),
		),
	))

	mux.Handle("GET /api/v1/consumers", authChain.Then(
		consentRateLimiter.Middleware()(
			h.WrapHandler("GET", "/api/v1/consumers", h.handleListConsumers,
				WithCache(2*time.Minute),
			),
		),
	))

	mux.Handle("GET /api/v1/consumers/{id}/consents", authChain.Then(
		consentRateLimiter.Middleware()(
			h.WrapHandler("GET", "/api/v1/consumers/{id}/consents", h.handleGetConsumerConsents,
				WithCache(1*time.Minute),
			),
		),
	))

	// Bulk operations with higher limits
	bulkRateLimiter := NewRateLimiter(RateLimitConfig{
		RequestsPerSecond: 5, // Stricter for bulk operations
		Burst:             10,
		ByUser:            true,
	})

	mux.Handle("POST /api/v1/consent/bulk/import", authChain.Then(
		bulkRateLimiter.Middleware()(
			h.WrapHandler("POST", "/api/v1/consent/bulk/import", h.handleImportConsents,
				WithMaxBodySize(50<<20), // 50MB for bulk import
				WithTimeout(10*time.Minute),
			),
		),
	))

	mux.Handle("POST /api/v1/consent/bulk/export", authChain.Then(
		bulkRateLimiter.Middleware()(
			h.WrapHandler("POST", "/api/v1/consent/bulk/export", h.handleExportConsents,
				WithTimeout(5*time.Minute),
			),
		),
	))

	// Analytics and metrics
	mux.Handle("GET /api/v1/consent/metrics", authChain.Then(
		consentRateLimiter.Middleware()(
			h.WrapHandler("GET", "/api/v1/consent/metrics", h.handleGetConsentMetrics,
				WithCache(10*time.Minute), // Cache metrics longer
			),
		),
	))
}

// Handler implementations

func (h *ConsentHandler) handleGrantConsent(ctx context.Context, r *http.Request) (interface{}, error) {
	var req consentService.GrantConsentRequest
	body, err := h.readBody(r)
	if err != nil {
		return nil, err
	}
	
	if err := h.parseAndValidate(body, &req); err != nil {
		return nil, errors.NewValidationError("INVALID_REQUEST", "request validation failed").WithCause(err)
	}

	// Add request metadata
	req.IPAddress = h.getClientIP(r)
	req.UserAgent = r.UserAgent()

	response, err := h.consentService.GrantConsent(ctx, req)
	if err != nil {
		return nil, h.handleServiceError(err)
	}

	return response, nil
}

func (h *ConsentHandler) handleGetConsent(ctx context.Context, r *http.Request) (interface{}, error) {
	consumerID, err := h.parseUUIDParam(r, "id")
	if err != nil {
		return nil, errors.NewValidationError("INVALID_CONSUMER_ID", "consumer ID must be a valid UUID")
	}

	consentType := r.URL.Query().Get("type")
	if consentType == "" {
		return nil, errors.NewValidationError("MISSING_CONSENT_TYPE", "consent type parameter is required")
	}

	parsedType, err := consent.ParseType(consentType)
	if err != nil {
		return nil, errors.NewValidationError("INVALID_CONSENT_TYPE", "invalid consent type").WithCause(err)
	}

	response, err := h.consentService.GetConsent(ctx, consumerID, parsedType)
	if err != nil {
		return nil, h.handleServiceError(err)
	}

	return response, nil
}

func (h *ConsentHandler) handleUpdateConsent(ctx context.Context, r *http.Request) (interface{}, error) {
	consumerID, err := h.parseUUIDParam(r, "id")
	if err != nil {
		return nil, errors.NewValidationError("INVALID_CONSUMER_ID", "consumer ID must be a valid UUID")
	}

	var req consentService.UpdateConsentRequest
	body, err := h.readBody(r)
	if err != nil {
		return nil, err
	}
	
	if err := h.parseAndValidate(body, &req); err != nil {
		return nil, errors.NewValidationError("INVALID_REQUEST", "request validation failed").WithCause(err)
	}

	req.ConsumerID = consumerID

	response, err := h.consentService.UpdateConsent(ctx, req)
	if err != nil {
		return nil, h.handleServiceError(err)
	}

	return response, nil
}

func (h *ConsentHandler) handleRevokeConsent(ctx context.Context, r *http.Request) (interface{}, error) {
	consumerID, err := h.parseUUIDParam(r, "id")
	if err != nil {
		return nil, errors.NewValidationError("INVALID_CONSUMER_ID", "consumer ID must be a valid UUID")
	}

	consentType := r.URL.Query().Get("type")
	if consentType == "" {
		return nil, errors.NewValidationError("MISSING_CONSENT_TYPE", "consent type parameter is required")
	}

	parsedType, err := consent.ParseType(consentType)
	if err != nil {
		return nil, errors.NewValidationError("INVALID_CONSENT_TYPE", "invalid consent type").WithCause(err)
	}

	err = h.consentService.RevokeConsent(ctx, consumerID, parsedType)
	if err != nil {
		return nil, h.handleServiceError(err)
	}

	return map[string]interface{}{
		"success": true,
		"message": "consent revoked successfully",
	}, nil
}

func (h *ConsentHandler) handleVerifyConsent(ctx context.Context, r *http.Request) (interface{}, error) {
	var req VerifyConsentRequest
	body, err := h.readBody(r)
	if err != nil {
		return nil, err
	}
	
	if err := h.parseAndValidate(body, &req); err != nil {
		return nil, errors.NewValidationError("INVALID_REQUEST", "request validation failed").WithCause(err)
	}

	status, err := h.consentService.CheckConsent(ctx, req.PhoneNumber, req.ConsentType)
	if err != nil {
		return nil, h.handleServiceError(err)
	}

	return &VerifyConsentResponse{
		PhoneNumber: req.PhoneNumber,
		ConsentType: req.ConsentType,
		HasConsent:  status.HasConsent,
		Status:      status.Status,
		GrantedAt:   status.GrantedAt,
		ExpiresAt:   status.ExpiresAt,
		VerifiedAt:  time.Now(),
	}, nil
}

func (h *ConsentHandler) handleVerifyConsentBatch(ctx context.Context, r *http.Request) (interface{}, error) {
	var req BatchVerifyConsentRequest
	body, err := h.readBody(r)
	if err != nil {
		return nil, err
	}
	
	if err := h.parseAndValidate(body, &req); err != nil {
		return nil, errors.NewValidationError("INVALID_REQUEST", "request validation failed").WithCause(err)
	}

	if len(req.Requests) > 1000 {
		return nil, errors.NewValidationError("TOO_MANY_REQUESTS", "batch size cannot exceed 1000 requests")
	}

	responses := make([]VerifyConsentResponse, len(req.Requests))
	verifiedAt := time.Now()

	for i, item := range req.Requests {
		status, err := h.consentService.CheckConsent(ctx, item.PhoneNumber, item.ConsentType)
		if err != nil {
			responses[i] = VerifyConsentResponse{
				PhoneNumber: item.PhoneNumber,
				ConsentType: item.ConsentType,
				HasConsent:  false,
				Error:       err.Error(),
				VerifiedAt:  verifiedAt,
			}
		} else {
			responses[i] = VerifyConsentResponse{
				PhoneNumber: item.PhoneNumber,
				ConsentType: item.ConsentType,
				HasConsent:  status.HasConsent,
				Status:      status.Status,
				GrantedAt:   status.GrantedAt,
				ExpiresAt:   status.ExpiresAt,
				VerifiedAt:  verifiedAt,
			}
		}
	}

	return &BatchVerifyConsentResponse{
		Results:      responses,
		TotalCount:   len(responses),
		SuccessCount: countSuccessful(responses),
		FailureCount: countFailed(responses),
	}, nil
}

func (h *ConsentHandler) handleCreateConsumer(ctx context.Context, r *http.Request) (interface{}, error) {
	var req consentService.CreateConsumerRequest
	body, err := h.readBody(r)
	if err != nil {
		return nil, err
	}
	
	if err := h.parseAndValidate(body, &req); err != nil {
		return nil, errors.NewValidationError("INVALID_REQUEST", "request validation failed").WithCause(err)
	}

	response, err := h.consentService.CreateConsumer(ctx, req)
	if err != nil {
		return nil, h.handleServiceError(err)
	}

	return response, nil
}

func (h *ConsentHandler) handleListConsumers(ctx context.Context, r *http.Request) (interface{}, error) {
	// Parse query parameters
	phone := r.URL.Query().Get("phone")
	email := r.URL.Query().Get("email")

	if phone != "" {
		response, err := h.consentService.GetConsumerByPhone(ctx, phone)
		if err != nil {
			return nil, h.handleServiceError(err)
		}
		return []*consentService.ConsumerResponse{response}, nil
	}

	if email != "" {
		response, err := h.consentService.GetConsumerByEmail(ctx, email)
		if err != nil {
			return nil, h.handleServiceError(err)
		}
		return []*consentService.ConsumerResponse{response}, nil
	}

	return nil, errors.NewValidationError("MISSING_SEARCH_CRITERIA", "either phone or email parameter is required")
}

func (h *ConsentHandler) handleGetConsumerConsents(ctx context.Context, r *http.Request) (interface{}, error) {
	consumerID, err := h.parseUUIDParam(r, "id")
	if err != nil {
		return nil, errors.NewValidationError("INVALID_CONSUMER_ID", "consumer ID must be a valid UUID")
	}

	response, err := h.consentService.GetActiveConsents(ctx, consumerID)
	if err != nil {
		return nil, h.handleServiceError(err)
	}

	return response, nil
}

func (h *ConsentHandler) handleImportConsents(ctx context.Context, r *http.Request) (interface{}, error) {
	var req consentService.ImportConsentsRequest
	body, err := h.readBody(r)
	if err != nil {
		return nil, err
	}
	
	if err := h.parseAndValidate(body, &req); err != nil {
		return nil, errors.NewValidationError("INVALID_REQUEST", "request validation failed").WithCause(err)
	}

	response, err := h.consentService.ImportConsents(ctx, req)
	if err != nil {
		return nil, h.handleServiceError(err)
	}

	return response, nil
}

func (h *ConsentHandler) handleExportConsents(ctx context.Context, r *http.Request) (interface{}, error) {
	var req consentService.ExportConsentsRequest
	body, err := h.readBody(r)
	if err != nil {
		return nil, err
	}
	
	if err := h.parseAndValidate(body, &req); err != nil {
		return nil, errors.NewValidationError("INVALID_REQUEST", "request validation failed").WithCause(err)
	}

	response, err := h.consentService.ExportConsents(ctx, req)
	if err != nil {
		return nil, h.handleServiceError(err)
	}

	return response, nil
}

func (h *ConsentHandler) handleGetConsentMetrics(ctx context.Context, r *http.Request) (interface{}, error) {
	var req consentService.MetricsRequest

	// Parse query parameters
	startDate := r.URL.Query().Get("start_date")
	endDate := r.URL.Query().Get("end_date")
	groupBy := r.URL.Query().Get("group_by")

	if startDate == "" || endDate == "" {
		return nil, errors.NewValidationError("MISSING_DATE_RANGE", "start_date and end_date parameters are required")
	}

	var err error
	req.StartDate, err = time.Parse(time.RFC3339, startDate)
	if err != nil {
		return nil, errors.NewValidationError("INVALID_START_DATE", "start_date must be in RFC3339 format")
	}

	req.EndDate, err = time.Parse(time.RFC3339, endDate)
	if err != nil {
		return nil, errors.NewValidationError("INVALID_END_DATE", "end_date must be in RFC3339 format")
	}

	if groupBy != "" {
		req.GroupBy = groupBy
	} else {
		req.GroupBy = "day"
	}

	response, err := h.consentService.GetConsentMetrics(ctx, req)
	if err != nil {
		return nil, h.handleServiceError(err)
	}

	return response, nil
}

// Helper methods

func (h *ConsentHandler) parseUUIDParam(r *http.Request, paramName string) (uuid.UUID, error) {
	value := r.PathValue(paramName)
	if value == "" {
		return uuid.Nil, fmt.Errorf("missing %s parameter", paramName)
	}

	parsed, err := uuid.Parse(value)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid %s format: %w", paramName, err)
	}

	return parsed, nil
}

func (h *ConsentHandler) getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first (for load balancers/proxies)
	if xForwardedFor := r.Header.Get("X-Forwarded-For"); xForwardedFor != "" {
		// Take the first IP in the chain
		if idx := len(xForwardedFor); idx > 0 {
			return xForwardedFor[:idx]
		}
	}

	// Check X-Real-IP header
	if xRealIP := r.Header.Get("X-Real-IP"); xRealIP != "" {
		return xRealIP
	}

	// Fall back to RemoteAddr
	return r.RemoteAddr
}

func (h *ConsentHandler) handleServiceError(err error) error {
	// Convert service errors to API errors
	switch e := err.(type) {
	case *errors.AppError:
		return e
	default:
		// Log internal errors but don't expose details
		h.logger.Error("internal service error", "error", err)
		return errors.NewInternalError("an internal error occurred")
	}
}

func countSuccessful(responses []VerifyConsentResponse) int {
	count := 0
	for _, r := range responses {
		if r.Error == "" {
			count++
		}
	}
	return count
}

func countFailed(responses []VerifyConsentResponse) int {
	count := 0
	for _, r := range responses {
		if r.Error != "" {
			count++
		}
	}
	return count
}

// Request/Response DTOs specific to API

type VerifyConsentRequest struct {
	PhoneNumber string       `json:"phone_number" validate:"required,e164"`
	ConsentType consent.Type `json:"consent_type" validate:"required"`
}

type VerifyConsentResponse struct {
	PhoneNumber string         `json:"phone_number"`
	ConsentType consent.Type   `json:"consent_type"`
	HasConsent  bool           `json:"has_consent"`
	Status      consent.ConsentStatus `json:"status,omitempty"`
	GrantedAt   *time.Time     `json:"granted_at,omitempty"`
	ExpiresAt   *time.Time     `json:"expires_at,omitempty"`
	VerifiedAt  time.Time      `json:"verified_at"`
	Error       string         `json:"error,omitempty"`
}

type BatchVerifyConsentRequest struct {
	Requests []VerifyConsentRequest `json:"requests" validate:"required,max=1000,dive"`
}

type BatchVerifyConsentResponse struct {
	Results      []VerifyConsentResponse `json:"results"`
	TotalCount   int                     `json:"total_count"`
	SuccessCount int                     `json:"success_count"`
	FailureCount int                     `json:"failure_count"`
}

// Helper methods for ConsentHandler

func (h *ConsentHandler) readBody(r *http.Request) ([]byte, error) {
	// Set a maximum body size (10MB for imports, 1MB for regular requests)
	maxBodySize := int64(1 << 20) // 1MB default
	if r.URL.Path == "/api/v1/consent/bulk/import" {
		maxBodySize = 50 << 20 // 50MB for bulk imports
	}
	
	r.Body = http.MaxBytesReader(nil, r.Body, maxBodySize)
	
	body, err := io.ReadAll(r.Body)
	if err != nil {
		if err.Error() == "http: request body too large" {
			return nil, errors.NewValidationError("BODY_TOO_LARGE", "request body exceeds maximum size")
		}
		return nil, errors.NewValidationError("INVALID_BODY", "failed to read request body")
	}
	
	return body, nil
}

func (h *ConsentHandler) parseAndValidate(body []byte, v interface{}) error {
	if len(body) == 0 {
		return errors.NewValidationError("EMPTY_BODY", "request body cannot be empty")
	}
	
	if err := json.Unmarshal(body, v); err != nil {
		return errors.NewValidationError("INVALID_JSON", "invalid JSON format").WithCause(err)
	}
	
	if err := h.validator.Struct(v); err != nil {
		return h.formatValidationError(err)
	}
	
	return nil
}

func (h *ConsentHandler) formatValidationError(err error) error {
	if validationErrors, ok := err.(validator.ValidationErrors); ok {
		details := make(map[string]interface{})
		
		for _, fe := range validationErrors {
			field := fe.Field()
			tag := fe.Tag()
			
			var msg string
			switch tag {
			case "required":
				msg = "This field is required"
			case "e164":
				msg = "Must be a valid E.164 phone number (e.g., +1234567890)"
			case "email":
				msg = "Must be a valid email address"
			case "uuid":
				msg = "Must be a valid UUID"
			case "max":
				msg = fmt.Sprintf("Maximum length is %s", fe.Param())
			case "min":
				msg = fmt.Sprintf("Minimum length is %s", fe.Param())
			default:
				msg = fmt.Sprintf("Invalid value for field %s", field)
			}
			
			details[field] = msg
		}
		
		return errors.NewValidationError("VALIDATION_FAILED", "request validation failed").WithDetails(details)
	}
	
	return errors.NewValidationError("VALIDATION_FAILED", "request validation failed").WithCause(err)
}