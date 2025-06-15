package events

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/audit"
	"go.uber.org/zap"
)

// RedisCacheInvalidator implements CacheInvalidator using Redis
type RedisCacheInvalidator struct {
	logger *zap.Logger
	rules  map[audit.EventType][]InvalidationRule
	mu     sync.RWMutex
	
	// Redis client would be injected here
	// redisClient redis.Client
}

// NewRedisCacheInvalidator creates a new Redis-based cache invalidator
func NewRedisCacheInvalidator(logger *zap.Logger) *RedisCacheInvalidator {
	invalidator := &RedisCacheInvalidator{
		logger: logger,
		rules:  make(map[audit.EventType][]InvalidationRule),
	}
	
	// Register default DNC invalidation rules
	invalidator.registerDefaultRules()
	
	return invalidator
}

// InvalidateOnEvent invalidates cache entries based on the event
func (ci *RedisCacheInvalidator) InvalidateOnEvent(ctx context.Context, event DNCDomainEvent) error {
	ci.mu.RLock()
	rules, exists := ci.rules[event.GetEventType()]
	ci.mu.RUnlock()
	
	if !exists {
		// No invalidation rules for this event type
		return nil
	}
	
	for _, rule := range rules {
		if err := ci.applyRule(ctx, event, rule); err != nil {
			ci.logger.Error("Failed to apply cache invalidation rule",
				zap.Error(err),
				zap.String("event_type", string(event.GetEventType())),
				zap.String("event_id", event.GetEventID().String()),
			)
			// Continue with other rules even if one fails
		}
	}
	
	return nil
}

// RegisterInvalidationRule registers a cache invalidation rule for an event type
func (ci *RedisCacheInvalidator) RegisterInvalidationRule(eventType audit.EventType, rule InvalidationRule) {
	ci.mu.Lock()
	defer ci.mu.Unlock()
	
	ci.rules[eventType] = append(ci.rules[eventType], rule)
	
	ci.logger.Info("Registered cache invalidation rule",
		zap.String("event_type", string(eventType)),
		zap.Strings("cache_keys", rule.CacheKeys),
		zap.String("cache_pattern", rule.CachePattern),
		zap.Bool("ttl_reset", rule.TTLReset),
	)
}

// Private methods

func (ci *RedisCacheInvalidator) registerDefaultRules() {
	// Number suppressed events
	ci.RegisterInvalidationRule(audit.EventDNCNumberSuppressed, InvalidationRule{
		CacheKeys:    []string{"dnc:list", "dnc:stats"},
		CachePattern: "dnc:check:*",
		TTLReset:     false,
	})
	
	// Number released events
	ci.RegisterInvalidationRule(audit.EventDNCNumberReleased, InvalidationRule{
		CacheKeys:    []string{"dnc:list", "dnc:stats"},
		CachePattern: "dnc:check:*",
		TTLReset:     false,
	})
	
	// DNC check performed events
	ci.RegisterInvalidationRule(audit.EventDNCCheckPerformed, InvalidationRule{
		CacheKeys:    []string{"dnc:stats"},
		CachePattern: "",
		TTLReset:     false,
	})
	
	// DNC list synced events
	ci.RegisterInvalidationRule(audit.EventDNCListSynced, InvalidationRule{
		CacheKeys:    []string{"dnc:list", "dnc:sync:status"},
		CachePattern: "dnc:*",
		TTLReset:     true,
	})
}

func (ci *RedisCacheInvalidator) applyRule(ctx context.Context, event DNCDomainEvent, rule InvalidationRule) error {
	// Invalidate specific cache keys
	for _, key := range rule.CacheKeys {
		// Expand key template with event data
		expandedKey := ci.expandKey(key, event)
		if err := ci.invalidateKey(ctx, expandedKey, rule.TTLReset); err != nil {
			return fmt.Errorf("failed to invalidate key %s: %w", expandedKey, err)
		}
	}
	
	// Invalidate by pattern
	if rule.CachePattern != "" {
		expandedPattern := ci.expandKey(rule.CachePattern, event)
		if err := ci.invalidateByPattern(ctx, expandedPattern, rule.TTLReset); err != nil {
			return fmt.Errorf("failed to invalidate pattern %s: %w", expandedPattern, err)
		}
	}
	
	return nil
}

func (ci *RedisCacheInvalidator) expandKey(keyTemplate string, event DNCDomainEvent) string {
	// Replace placeholders with actual event data
	expanded := keyTemplate
	expanded = strings.ReplaceAll(expanded, "{aggregate_id}", event.GetAggregateID())
	expanded = strings.ReplaceAll(expanded, "{aggregate_type}", event.GetAggregateType())
	expanded = strings.ReplaceAll(expanded, "{event_type}", string(event.GetEventType()))
	
	return expanded
}

func (ci *RedisCacheInvalidator) invalidateKey(ctx context.Context, key string, resetTTL bool) error {
	// In a real implementation, this would use Redis commands
	// For now, just log the operation
	ci.logger.Debug("Would invalidate cache key",
		zap.String("key", key),
		zap.Bool("reset_ttl", resetTTL),
	)
	
	// Example Redis implementation:
	// if resetTTL {
	//     return ci.redisClient.Del(ctx, key).Err()
	// } else {
	//     return ci.redisClient.Expire(ctx, key, 0).Err()
	// }
	
	return nil
}

func (ci *RedisCacheInvalidator) invalidateByPattern(ctx context.Context, pattern string, resetTTL bool) error {
	// In a real implementation, this would scan for keys matching pattern
	ci.logger.Debug("Would invalidate cache by pattern",
		zap.String("pattern", pattern),
		zap.Bool("reset_ttl", resetTTL),
	)
	
	// Example Redis implementation:
	// keys, err := ci.redisClient.Keys(ctx, pattern).Result()
	// if err != nil {
	//     return err
	// }
	// 
	// if len(keys) > 0 {
	//     if resetTTL {
	//         return ci.redisClient.Del(ctx, keys...).Err()
	//     } else {
	//         pipe := ci.redisClient.Pipeline()
	//         for _, key := range keys {
	//             pipe.Expire(ctx, key, 0)
	//         }
	//         _, err := pipe.Exec(ctx)
	//         return err
	//     }
	// }
	
	return nil
}

// MemoryCacheInvalidator provides an in-memory implementation for testing
type MemoryCacheInvalidator struct {
	logger         *zap.Logger
	rules          map[audit.EventType][]InvalidationRule
	invalidatedKeys map[string]bool
	mu             sync.RWMutex
}

// NewMemoryCacheInvalidator creates a new in-memory cache invalidator
func NewMemoryCacheInvalidator(logger *zap.Logger) *MemoryCacheInvalidator {
	return &MemoryCacheInvalidator{
		logger:          logger,
		rules:           make(map[audit.EventType][]InvalidationRule),
		invalidatedKeys: make(map[string]bool),
	}
}

// InvalidateOnEvent invalidates cache entries in memory
func (ci *MemoryCacheInvalidator) InvalidateOnEvent(ctx context.Context, event DNCDomainEvent) error {
	ci.mu.RLock()
	rules, exists := ci.rules[event.GetEventType()]
	ci.mu.RUnlock()
	
	if !exists {
		return nil
	}
	
	ci.mu.Lock()
	defer ci.mu.Unlock()
	
	for _, rule := range rules {
		// Invalidate specific keys
		for _, key := range rule.CacheKeys {
			expandedKey := ci.expandKey(key, event)
			ci.invalidatedKeys[expandedKey] = true
		}
		
		// Invalidate by pattern (simplified)
		if rule.CachePattern != "" {
			expandedPattern := ci.expandKey(rule.CachePattern, event)
			ci.invalidatedKeys[expandedPattern] = true
		}
	}
	
	ci.logger.Debug("Invalidated cache entries in memory",
		zap.String("event_type", string(event.GetEventType())),
		zap.Int("total_invalidated", len(ci.invalidatedKeys)),
	)
	
	return nil
}

// RegisterInvalidationRule registers a rule for in-memory cache
func (ci *MemoryCacheInvalidator) RegisterInvalidationRule(eventType audit.EventType, rule InvalidationRule) {
	ci.mu.Lock()
	defer ci.mu.Unlock()
	
	ci.rules[eventType] = append(ci.rules[eventType], rule)
}

// GetInvalidatedKeys returns the list of invalidated keys (for testing)
func (ci *MemoryCacheInvalidator) GetInvalidatedKeys() []string {
	ci.mu.RLock()
	defer ci.mu.RUnlock()
	
	keys := make([]string, 0, len(ci.invalidatedKeys))
	for key := range ci.invalidatedKeys {
		keys = append(keys, key)
	}
	
	return keys
}

// ClearInvalidatedKeys clears the invalidated keys list (for testing)
func (ci *MemoryCacheInvalidator) ClearInvalidatedKeys() {
	ci.mu.Lock()
	defer ci.mu.Unlock()
	
	ci.invalidatedKeys = make(map[string]bool)
}

func (ci *MemoryCacheInvalidator) expandKey(keyTemplate string, event DNCDomainEvent) string {
	expanded := keyTemplate
	expanded = strings.ReplaceAll(expanded, "{aggregate_id}", event.GetAggregateID())
	expanded = strings.ReplaceAll(expanded, "{aggregate_type}", event.GetAggregateType())
	expanded = strings.ReplaceAll(expanded, "{event_type}", string(event.GetEventType()))
	return expanded
}