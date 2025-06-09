package cache

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Cache provides a generic caching interface with support for TTL and atomic operations
type Cache interface {
	// Get retrieves a value by key
	Get(ctx context.Context, key string) (string, error)
	
	// Set stores a value with optional TTL
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	
	// Delete removes a key
	Delete(ctx context.Context, key string) error
	
	// Exists checks if a key exists
	Exists(ctx context.Context, key string) (bool, error)
	
	// SetNX sets a value only if the key doesn't exist (atomic)
	SetNX(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error)
	
	// Increment atomically increments a numeric value
	Increment(ctx context.Context, key string) (int64, error)
	
	// Expire sets TTL on an existing key
	Expire(ctx context.Context, key string, ttl time.Duration) error
	
	// GetJSON retrieves and unmarshals JSON data
	GetJSON(ctx context.Context, key string, dest interface{}) error
	
	// SetJSON marshals and stores JSON data
	SetJSON(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	
	// Close closes the cache connection
	Close() error
}

// RateLimiter provides rate limiting functionality using various algorithms
type RateLimiter interface {
	// Allow checks if a request is allowed under the rate limit
	Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error)
	
	// Count returns the current count for a rate limit key
	Count(ctx context.Context, key string, window time.Duration) (int, error)
	
	// Reset clears the rate limit counter for a key
	Reset(ctx context.Context, key string) error
	
	// Remaining returns how many requests are remaining in the current window
	Remaining(ctx context.Context, key string, limit int, window time.Duration) (int, error)
}

// SessionStore provides session management with automatic cleanup
type SessionStore interface {
	// CreateSession creates a new session with the given data
	CreateSession(ctx context.Context, userID uuid.UUID, data map[string]interface{}) (string, error)
	
	// GetSession retrieves session data by session ID
	GetSession(ctx context.Context, sessionID string) (map[string]interface{}, error)
	
	// UpdateSession updates existing session data
	UpdateSession(ctx context.Context, sessionID string, data map[string]interface{}) error
	
	// DeleteSession removes a session
	DeleteSession(ctx context.Context, sessionID string) error
	
	// ExtendSession updates the session TTL
	ExtendSession(ctx context.Context, sessionID string, ttl time.Duration) error
	
	// ListSessions returns all active sessions for a user
	ListSessions(ctx context.Context, userID uuid.UUID) ([]string, error)
	
	// CleanupExpired removes expired sessions (called by background job)
	CleanupExpired(ctx context.Context) (int64, error)
}

// Key prefixes for consistent cache key naming
const (
	SessionPrefix    = "dce:session:"
	RiskScorePrefix  = "dce:risk:"
	RoutePrefix      = "dce:route:"
	RateLimitPrefix  = "dce:ratelimit:"
	UserPrefix       = "dce:user:"
	CallPrefix       = "dce:call:"
	BidPrefix        = "dce:bid:"
)

// Common TTL values
const (
	DefaultTTL     = 1 * time.Hour
	SessionTTL     = 24 * time.Hour
	RiskScoreTTL   = 5 * time.Minute
	RouteCacheTTL  = 10 * time.Minute
	RateLimitTTL   = 1 * time.Minute
	ShortCacheTTL  = 30 * time.Second
)

// ErrCacheKeyNotFound is returned when a cache key doesn't exist
type ErrCacheKeyNotFound struct {
	Key string
}

func (e ErrCacheKeyNotFound) Error() string {
	return "cache key not found: " + e.Key
}

// ErrRateLimitExceeded is returned when rate limit is exceeded
type ErrRateLimitExceeded struct {
	Key   string
	Limit int
}

func (e ErrRateLimitExceeded) Error() string {
	return "rate limit exceeded for key: " + e.Key
}

// ErrSessionExpired is returned when a session has expired
type ErrSessionExpired struct {
	SessionID string
}

func (e ErrSessionExpired) Error() string {
	return "session expired: " + e.SessionID
}