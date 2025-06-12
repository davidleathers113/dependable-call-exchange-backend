package repository

import (
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Common repository errors
var (
	ErrNotFound         = errors.New("entity not found")
	ErrDuplicateKey     = errors.New("duplicate key violation")
	ErrForeignKey       = errors.New("foreign key violation")
	ErrInvalidInput     = errors.New("invalid input")
	ErrOptimisticLock   = errors.New("optimistic lock failure")
	ErrConnectionClosed = errors.New("database connection closed")
)

// IsForeignKeyViolation checks if the error is a foreign key constraint violation
func IsForeignKeyViolation(err error) bool {
	if err == nil {
		return false
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		// PostgreSQL foreign key violation error code
		return pgErr.Code == "23503"
	}

	// Fallback to string matching for wrapped errors
	return strings.Contains(err.Error(), "foreign key") ||
		strings.Contains(err.Error(), "violates foreign key constraint")
}

// IsDuplicateKeyViolation checks if the error is a unique constraint violation
func IsDuplicateKeyViolation(err error) bool {
	if err == nil {
		return false
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		// PostgreSQL unique violation error code
		return pgErr.Code == "23505"
	}

	// Fallback to string matching for wrapped errors
	return strings.Contains(err.Error(), "duplicate key") ||
		strings.Contains(err.Error(), "violates unique constraint")
}

// IsNotFound checks if the error indicates a record was not found
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound) || errors.Is(err, pgx.ErrNoRows)
}

// IsConnectionError checks if the error is related to database connectivity
func IsConnectionError(err error) bool {
	if err == nil {
		return false
	}

	// Check for common connection errors
	return errors.Is(err, ErrConnectionClosed) ||
		strings.Contains(err.Error(), "connection refused") ||
		strings.Contains(err.Error(), "connection reset") ||
		strings.Contains(err.Error(), "no connection to the server")
}

// WrapRepositoryError wraps database errors into domain-appropriate errors
func WrapRepositoryError(err error, operation string) error {
	if err == nil {
		return nil
	}

	// Handle specific error cases
	if IsNotFound(err) {
		return ErrNotFound
	}

	if IsDuplicateKeyViolation(err) {
		return ErrDuplicateKey
	}

	if IsForeignKeyViolation(err) {
		return ErrForeignKey
	}

	// Return the original error with context
	return err
}
