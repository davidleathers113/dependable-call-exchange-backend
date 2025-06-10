package auth

import (
	"time"

	"github.com/google/uuid"
)

// TokenClaims represents JWT token claims
type TokenClaims struct {
	UserID   uuid.UUID
	Email    string
	UserType string
	Scopes   []string
	IssuedAt time.Time
	ExpireAt time.Time
}

// Service provides authentication and authorization
type Service interface {
	// GenerateToken creates a new JWT token
	GenerateToken(userID uuid.UUID, email, userType string, scopes []string) (string, error)
	
	// ValidateToken validates and parses a JWT token
	ValidateToken(token string) (*TokenClaims, error)
	
	// GenerateRefreshToken creates a new refresh token
	GenerateRefreshToken(userID uuid.UUID) (string, error)
	
	// ValidateRefreshToken validates a refresh token
	ValidateRefreshToken(token string) (uuid.UUID, error)
	
	// HashPassword hashes a password
	HashPassword(password string) (string, error)
	
	// ComparePassword compares a password with its hash
	ComparePassword(hash, password string) error
}

// AuthError represents an authentication error
type AuthError struct {
	Code    string
	Message string
}

func (e AuthError) Error() string {
	return e.Message
}
