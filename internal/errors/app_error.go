package errors

import "fmt"

// AppError represents a domain-specific error
type AppError struct {
    Code    string
    Message string
    Cause   error
}

func (e *AppError) Error() string {
    if e.Cause != nil {
        return fmt.Sprintf("%s: %s (caused by: %v)", e.Code, e.Message, e.Cause)
    }
    return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func NewValidationError(code, message string) *AppError {
    return &AppError{Code: code, Message: message}
}

func NewDomainError(code, message string) *AppError {
    return &AppError{Code: code, Message: message}
}

func (e *AppError) WithCause(cause error) *AppError {
    e.Cause = cause
    return e
}

