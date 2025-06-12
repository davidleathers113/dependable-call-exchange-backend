package bidding

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
	"github.com/google/uuid"
)

// rateLimitService provides generic rate limiting
type rateLimitService struct {
	mu      sync.RWMutex
	configs map[string]*rateLimitConfig
	limits  map[string]map[uuid.UUID]*rateLimitEntry
}

// rateLimitConfig holds configuration for a limit type
type rateLimitConfig struct {
	count  int
	window time.Duration
}

// rateLimitEntry tracks rate limit state for an entity
type rateLimitEntry struct {
	count       int
	windowStart time.Time
	mu          sync.Mutex
}

// NewRateLimitService creates a new rate limit service
func NewRateLimitService() RateLimitService {
	return &rateLimitService{
		configs: make(map[string]*rateLimitConfig),
		limits:  make(map[string]map[uuid.UUID]*rateLimitEntry),
	}
}

// Configure sets rate limit parameters
func (s *rateLimitService) Configure(limitType string, count int, window time.Duration) error {
	if limitType == "" {
		return errors.NewValidationError("INVALID_LIMIT_TYPE", "limit type cannot be empty")
	}

	if count <= 0 {
		return errors.NewValidationError("INVALID_COUNT", "count must be positive")
	}

	if window <= 0 {
		return errors.NewValidationError("INVALID_WINDOW", "window must be positive")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.configs[limitType] = &rateLimitConfig{
		count:  count,
		window: window,
	}

	// Initialize the limits map for this type if not exists
	if _, exists := s.limits[limitType]; !exists {
		s.limits[limitType] = make(map[uuid.UUID]*rateLimitEntry)
	}

	return nil
}

// CheckRateLimit checks if entity is within rate limit
func (s *rateLimitService) CheckRateLimit(ctx context.Context, entityID uuid.UUID, limitType string) error {
	s.mu.RLock()
	config, exists := s.configs[limitType]
	if !exists {
		s.mu.RUnlock()
		return errors.NewInternalError(fmt.Sprintf("rate limit type %s not configured", limitType))
	}

	limitsForType, exists := s.limits[limitType]
	if !exists {
		s.mu.RUnlock()
		return errors.NewInternalError(fmt.Sprintf("rate limit type %s not initialized", limitType))
	}
	s.mu.RUnlock()

	// Get or create entry for entity
	s.mu.Lock()
	entry, exists := limitsForType[entityID]
	if !exists {
		entry = &rateLimitEntry{
			count:       0,
			windowStart: time.Now(),
		}
		limitsForType[entityID] = entry
	}
	s.mu.Unlock()

	// Check rate limit
	entry.mu.Lock()
	defer entry.mu.Unlock()

	now := time.Now()

	// Reset window if expired
	if now.Sub(entry.windowStart) > config.window {
		entry.count = 0
		entry.windowStart = now
	}

	// Check if within limit
	if entry.count >= config.count {
		return errors.NewRateLimitError(
			fmt.Sprintf("rate limit exceeded: %d requests in %v", config.count, config.window))
	}

	return nil
}

// RecordAction records an action for rate limiting
func (s *rateLimitService) RecordAction(ctx context.Context, entityID uuid.UUID, limitType string) error {
	s.mu.RLock()
	config, exists := s.configs[limitType]
	if !exists {
		s.mu.RUnlock()
		return errors.NewInternalError(fmt.Sprintf("rate limit type %s not configured", limitType))
	}

	limitsForType, exists := s.limits[limitType]
	if !exists {
		s.mu.RUnlock()
		return errors.NewInternalError(fmt.Sprintf("rate limit type %s not initialized", limitType))
	}
	s.mu.RUnlock()

	// Get or create entry for entity
	s.mu.Lock()
	entry, exists := limitsForType[entityID]
	if !exists {
		entry = &rateLimitEntry{
			count:       0,
			windowStart: time.Now(),
		}
		limitsForType[entityID] = entry
	}
	s.mu.Unlock()

	// Record action
	entry.mu.Lock()
	defer entry.mu.Unlock()

	now := time.Now()

	// Reset window if expired
	if now.Sub(entry.windowStart) > config.window {
		entry.count = 1
		entry.windowStart = now
	} else {
		entry.count++
	}

	return nil
}

// GetCurrentCount returns current count for entity
func (s *rateLimitService) GetCurrentCount(ctx context.Context, entityID uuid.UUID, limitType string) (int, error) {
	s.mu.RLock()
	config, exists := s.configs[limitType]
	if !exists {
		s.mu.RUnlock()
		return 0, errors.NewInternalError(fmt.Sprintf("rate limit type %s not configured", limitType))
	}

	limitsForType, exists := s.limits[limitType]
	if !exists {
		s.mu.RUnlock()
		return 0, errors.NewInternalError(fmt.Sprintf("rate limit type %s not initialized", limitType))
	}

	entry, exists := limitsForType[entityID]
	s.mu.RUnlock()

	if !exists {
		return 0, nil
	}

	entry.mu.Lock()
	defer entry.mu.Unlock()

	now := time.Now()

	// Check if window expired
	if now.Sub(entry.windowStart) > config.window {
		return 0, nil
	}

	return entry.count, nil
}

// ResetLimit resets rate limit for entity
func (s *rateLimitService) ResetLimit(ctx context.Context, entityID uuid.UUID, limitType string) error {
	s.mu.RLock()
	limitsForType, exists := s.limits[limitType]
	if !exists {
		s.mu.RUnlock()
		return errors.NewInternalError(fmt.Sprintf("rate limit type %s not initialized", limitType))
	}
	s.mu.RUnlock()

	s.mu.Lock()
	delete(limitsForType, entityID)
	s.mu.Unlock()

	return nil
}

// CleanupExpired removes expired entries (should be called periodically)
func (s *rateLimitService) CleanupExpired() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()

	for limitType, config := range s.configs {
		limitsForType := s.limits[limitType]

		// Collect entities to remove
		var toRemove []uuid.UUID

		for entityID, entry := range limitsForType {
			entry.mu.Lock()
			if now.Sub(entry.windowStart) > config.window {
				toRemove = append(toRemove, entityID)
			}
			entry.mu.Unlock()
		}

		// Remove expired entries
		for _, entityID := range toRemove {
			delete(limitsForType, entityID)
		}
	}
}
