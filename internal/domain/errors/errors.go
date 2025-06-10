package errors

import (
	"errors"
	"fmt"
)

// Error types for different domains
type ErrorType string

const (
	ErrorTypeValidation   ErrorType = "validation"
	ErrorTypeBusiness     ErrorType = "business"
	ErrorTypeInternal     ErrorType = "internal"
	ErrorTypeExternal     ErrorType = "external"
	ErrorTypeNotFound     ErrorType = "not_found"
	ErrorTypeUnauthorized ErrorType = "unauthorized"
	ErrorTypeForbidden    ErrorType = "forbidden"
	ErrorTypeConflict     ErrorType = "conflict"
	ErrorTypeCompliance   ErrorType = "compliance"
	ErrorTypeFraud        ErrorType = "fraud"
)

// AppError represents a structured application error
type AppError struct {
	Type        ErrorType              `json:"type"`
	Code        string                 `json:"code"`
	Message     string                 `json:"message"`
	Details     map[string]interface{} `json:"details,omitempty"`
	Cause       error                  `json:"-"`
	Retryable   bool                   `json:"retryable"`
	StatusCode  int                    `json:"status_code"`
}

func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

func (e *AppError) Unwrap() error {
	return e.Cause
}

func (e *AppError) WithDetails(details map[string]interface{}) *AppError {
	e.Details = details
	return e
}

func (e *AppError) WithCause(cause error) *AppError {
	e.Cause = cause
	return e
}

// Error constructors
func NewValidationError(code, message string) *AppError {
	return &AppError{
		Type:       ErrorTypeValidation,
		Code:       code,
		Message:    message,
		Retryable:  false,
		StatusCode: 400,
	}
}

func NewBusinessError(code, message string) *AppError {
	return &AppError{
		Type:       ErrorTypeBusiness,
		Code:       code,
		Message:    message,
		Retryable:  false,
		StatusCode: 422,
	}
}

func NewNotFoundError(resource string) *AppError {
	return &AppError{
		Type:       ErrorTypeNotFound,
		Code:       "RESOURCE_NOT_FOUND",
		Message:    fmt.Sprintf("%s not found", resource),
		Retryable:  false,
		StatusCode: 404,
	}
}

func NewUnauthorizedError(message string) *AppError {
	return &AppError{
		Type:       ErrorTypeUnauthorized,
		Code:       "UNAUTHORIZED",
		Message:    message,
		Retryable:  false,
		StatusCode: 401,
	}
}

func NewForbiddenError(message string) *AppError {
	return &AppError{
		Type:       ErrorTypeForbidden,
		Code:       "FORBIDDEN",
		Message:    message,
		Retryable:  false,
		StatusCode: 403,
	}
}

func NewConflictError(message string) *AppError {
	return &AppError{
		Type:       ErrorTypeConflict,
		Code:       "CONFLICT",
		Message:    message,
		Retryable:  false,
		StatusCode: 409,
	}
}

func NewInternalError(message string) *AppError {
	return &AppError{
		Type:       ErrorTypeInternal,
		Code:       "INTERNAL_ERROR",
		Message:    message,
		Retryable:  true,
		StatusCode: 500,
	}
}

func NewExternalError(service, message string) *AppError {
	return &AppError{
		Type:       ErrorTypeExternal,
		Code:       "EXTERNAL_SERVICE_ERROR",
		Message:    fmt.Sprintf("%s service error: %s", service, message),
		Retryable:  true,
		StatusCode: 502,
		Details:    map[string]interface{}{"service": service},
	}
}

func NewComplianceError(violation, message string) *AppError {
	return &AppError{
		Type:       ErrorTypeCompliance,
		Code:       "COMPLIANCE_VIOLATION",
		Message:    message,
		Retryable:  false,
		StatusCode: 403,
		Details:    map[string]interface{}{"violation_type": violation},
	}
}

func NewRateLimitError(message string) *AppError {
	return &AppError{
		Type:       ErrorTypeForbidden,
		Code:       "RATE_LIMIT_EXCEEDED",
		Message:    message,
		Retryable:  true,
		StatusCode: 429,
	}
}

func NewFraudError(reason, message string) *AppError {
	return &AppError{
		Type:       ErrorTypeFraud,
		Code:       "FRAUD_DETECTED",
		Message:    message,
		Retryable:  false,
		StatusCode: 403,
		Details:    map[string]interface{}{"fraud_reason": reason},
	}
}

// Predefined common errors
var (
	ErrInvalidInput       = NewValidationError("INVALID_INPUT", "Invalid input provided")
	ErrInsufficientFunds  = NewBusinessError("INSUFFICIENT_FUNDS", "Insufficient funds")
	ErrAccountSuspended   = NewBusinessError("ACCOUNT_SUSPENDED", "Account is suspended")
	ErrCallNotFound       = NewNotFoundError("call")
	ErrBidNotFound        = NewNotFoundError("bid")
	ErrAccountNotFound    = NewNotFoundError("account")
	ErrAuctionExpired     = NewBusinessError("AUCTION_EXPIRED", "Auction has expired")
	ErrBidTooLow          = NewBusinessError("BID_TOO_LOW", "Bid amount is below minimum")
	ErrDuplicateBid       = NewConflictError("Duplicate bid detected")
)

// Wrap wraps an error with a message using fmt.Errorf with %w
func Wrap(err error, message string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", message, err)
}

// WrapWithCode wraps an error and returns an AppError
func WrapWithCode(err error, code, message string) *AppError {
	return NewInternalError(message).WithCause(err)
}

// IsType checks if an error is of a specific type
func IsType(err error, errorType ErrorType) bool {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Type == errorType
	}
	return false
}

// IsRetryable checks if an error is retryable
func IsRetryable(err error) bool {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Retryable
	}
	return false
}

// GetStatusCode extracts HTTP status code from error
func GetStatusCode(err error) int {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.StatusCode
	}
	return 500
}