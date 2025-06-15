package events

import (
	"sync"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/audit"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// EventRouter efficiently routes events to matching subscriptions
type EventRouter struct {
	logger *zap.Logger
	
	// Indexes for fast lookup
	byEventType  map[audit.EventType][]string
	bySeverity   map[audit.Severity][]string
	byEntityType map[string][]string
	byEntityID   map[uuid.UUID][]string
	byUserID     map[uuid.UUID][]string
	
	// All subscriptions
	subscriptions map[string]*Subscription
	
	// Protect concurrent access
	mu sync.RWMutex
	
	// Cache for recent routing decisions
	routeCache *RouteCache
}

// RouteCache caches recent routing decisions for performance
type RouteCache struct {
	entries map[string]*RouteCacheEntry
	mu      sync.RWMutex
	maxSize int
	ttl     time.Duration
}

// RouteCacheEntry represents a cached routing decision
type RouteCacheEntry struct {
	SubscriptionIDs []string
	CreatedAt       time.Time
}

// NewEventRouter creates a new event router
func NewEventRouter(logger *zap.Logger) *EventRouter {
	return &EventRouter{
		logger:        logger,
		byEventType:   make(map[audit.EventType][]string),
		bySeverity:    make(map[audit.Severity][]string),
		byEntityType:  make(map[string][]string),
		byEntityID:    make(map[uuid.UUID][]string),
		byUserID:      make(map[uuid.UUID][]string),
		subscriptions: make(map[string]*Subscription),
		routeCache:    NewRouteCache(1000, 1*time.Minute),
	}
}

// AddSubscription adds a subscription to the router
func (r *EventRouter) AddSubscription(subscription *Subscription) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	id := subscription.ID
	r.subscriptions[id] = subscription
	
	// Index by event types
	for _, eventType := range subscription.Filters.EventTypes {
		r.byEventType[eventType] = append(r.byEventType[eventType], id)
	}
	
	// Index by severity
	for _, severity := range subscription.Filters.Severity {
		r.bySeverity[severity] = append(r.bySeverity[severity], id)
	}
	
	// Index by entity types
	for _, entityType := range subscription.Filters.EntityTypes {
		r.byEntityType[entityType] = append(r.byEntityType[entityType], id)
	}
	
	// Index by entity IDs
	for _, entityID := range subscription.Filters.EntityIDs {
		r.byEntityID[entityID] = append(r.byEntityID[entityID], id)
	}
	
	// Index by user IDs
	for _, userID := range subscription.Filters.UserIDs {
		r.byUserID[userID] = append(r.byUserID[userID], id)
	}
	
	// Clear route cache as routing may have changed
	r.routeCache.Clear()
}

// RemoveSubscription removes a subscription from the router
func (r *EventRouter) RemoveSubscription(subscriptionID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	subscription, exists := r.subscriptions[subscriptionID]
	if !exists {
		return
	}
	
	delete(r.subscriptions, subscriptionID)
	
	// Remove from all indexes
	r.removeFromIndex(&r.byEventType, subscription.Filters.EventTypes, subscriptionID)
	r.removeFromIndex(&r.bySeverity, subscription.Filters.Severity, subscriptionID)
	r.removeFromStringIndex(&r.byEntityType, subscription.Filters.EntityTypes, subscriptionID)
	r.removeFromUUIDIndex(&r.byEntityID, subscription.Filters.EntityIDs, subscriptionID)
	r.removeFromUUIDIndex(&r.byUserID, subscription.Filters.UserIDs, subscriptionID)
	
	// Clear route cache
	r.routeCache.Clear()
}

// Route finds all subscriptions that match the given event
func (r *EventRouter) Route(event *audit.Event) []*Subscription {
	// Check cache first
	cacheKey := r.getCacheKey(event)
	if cached := r.routeCache.Get(cacheKey); cached != nil {
		r.mu.RLock()
		subscriptions := make([]*Subscription, 0, len(cached))
		for _, id := range cached {
			if sub, exists := r.subscriptions[id]; exists {
				subscriptions = append(subscriptions, sub)
			}
		}
		r.mu.RUnlock()
		return subscriptions
	}
	
	// Find matching subscriptions
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	// Use a map to track unique subscription IDs
	matches := make(map[string]bool)
	
	// Find subscriptions with no filters (match all events)
	for id, sub := range r.subscriptions {
		if r.hasNoFilters(sub.Filters) {
			matches[id] = true
		}
	}
	
	// Find by event type
	if subs, ok := r.byEventType[event.Type]; ok {
		for _, id := range subs {
			matches[id] = true
		}
	}
	
	// Find by severity
	if subs, ok := r.bySeverity[event.Severity]; ok {
		for _, id := range subs {
			matches[id] = true
		}
	}
	
	// Find by entity type
	if event.EntityType != "" {
		if subs, ok := r.byEntityType[event.EntityType]; ok {
			for _, id := range subs {
				matches[id] = true
			}
		}
	}
	
	// Find by entity ID
	if event.EntityID != uuid.Nil {
		if subs, ok := r.byEntityID[event.EntityID]; ok {
			for _, id := range subs {
				matches[id] = true
			}
		}
	}
	
	// Find by user ID
	if event.UserID != uuid.Nil {
		if subs, ok := r.byUserID[event.UserID]; ok {
			for _, id := range subs {
				matches[id] = true
			}
		}
	}
	
	// Filter matches based on all criteria
	result := make([]*Subscription, 0, len(matches))
	resultIDs := make([]string, 0, len(matches))
	
	for id := range matches {
		sub := r.subscriptions[id]
		if r.matchesAllFilters(sub.Filters, event) {
			result = append(result, sub)
			resultIDs = append(resultIDs, id)
		}
	}
	
	// Cache the result
	r.routeCache.Set(cacheKey, resultIDs)
	
	return result
}

// Private helper methods

func (r *EventRouter) hasNoFilters(filters EventFilters) bool {
	return len(filters.EventTypes) == 0 &&
		len(filters.Severity) == 0 &&
		len(filters.EntityTypes) == 0 &&
		len(filters.EntityIDs) == 0 &&
		len(filters.UserIDs) == 0 &&
		filters.TimeRange == nil &&
		len(filters.CustomFilters) == 0
}

func (r *EventRouter) matchesAllFilters(filters EventFilters, event *audit.Event) bool {
	// Check event type filter
	if len(filters.EventTypes) > 0 {
		found := false
		for _, t := range filters.EventTypes {
			if t == event.Type {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	
	// Check severity filter
	if len(filters.Severity) > 0 {
		found := false
		for _, s := range filters.Severity {
			if s == event.Severity {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	
	// Check entity type filter
	if len(filters.EntityTypes) > 0 && event.EntityType != "" {
		found := false
		for _, t := range filters.EntityTypes {
			if t == event.EntityType {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	
	// Check entity ID filter
	if len(filters.EntityIDs) > 0 && event.EntityID != uuid.Nil {
		found := false
		for _, id := range filters.EntityIDs {
			if id == event.EntityID {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	
	// Check user ID filter
	if len(filters.UserIDs) > 0 && event.UserID != uuid.Nil {
		found := false
		for _, id := range filters.UserIDs {
			if id == event.UserID {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	
	// Check time range filter
	if filters.TimeRange != nil {
		if event.Timestamp.Before(filters.TimeRange.Start) ||
			event.Timestamp.After(filters.TimeRange.End) {
			return false
		}
	}
	
	// TODO: Implement custom filter matching
	// This would require a more sophisticated filtering engine
	
	return true
}

func (r *EventRouter) removeFromIndex[K comparable](
	index *map[K][]string,
	keys []K,
	subscriptionID string,
) {
	for _, key := range keys {
		if subs, ok := (*index)[key]; ok {
			(*index)[key] = removeString(subs, subscriptionID)
			if len((*index)[key]) == 0 {
				delete(*index, key)
			}
		}
	}
}

func (r *EventRouter) removeFromStringIndex(
	index *map[string][]string,
	keys []string,
	subscriptionID string,
) {
	for _, key := range keys {
		if subs, ok := (*index)[key]; ok {
			(*index)[key] = removeString(subs, subscriptionID)
			if len((*index)[key]) == 0 {
				delete(*index, key)
			}
		}
	}
}

func (r *EventRouter) removeFromUUIDIndex(
	index *map[uuid.UUID][]string,
	keys []uuid.UUID,
	subscriptionID string,
) {
	for _, key := range keys {
		if subs, ok := (*index)[key]; ok {
			(*index)[key] = removeString(subs, subscriptionID)
			if len((*index)[key]) == 0 {
				delete(*index, key)
			}
		}
	}
}

func removeString(slice []string, str string) []string {
	result := make([]string, 0, len(slice))
	for _, s := range slice {
		if s != str {
			result = append(result, s)
		}
	}
	return result
}

func (r *EventRouter) getCacheKey(event *audit.Event) string {
	// Create a unique cache key based on event properties
	// This is a simple implementation - could be optimized
	return event.Type.String() + ":" + 
		event.Severity.String() + ":" +
		event.EntityType + ":" +
		event.EntityID.String() + ":" +
		event.UserID.String()
}

// RouteCache implementation

func NewRouteCache(maxSize int, ttl time.Duration) *RouteCache {
	cache := &RouteCache{
		entries: make(map[string]*RouteCacheEntry),
		maxSize: maxSize,
		ttl:     ttl,
	}
	
	// Start cleanup goroutine
	go cache.cleanup()
	
	return cache
}

func (c *RouteCache) Get(key string) []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	entry, exists := c.entries[key]
	if !exists {
		return nil
	}
	
	// Check if entry is expired
	if time.Since(entry.CreatedAt) > c.ttl {
		return nil
	}
	
	return entry.SubscriptionIDs
}

func (c *RouteCache) Set(key string, subscriptionIDs []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Simple LRU: remove oldest entry if at capacity
	if len(c.entries) >= c.maxSize {
		var oldestKey string
		var oldestTime time.Time
		for k, v := range c.entries {
			if oldestTime.IsZero() || v.CreatedAt.Before(oldestTime) {
				oldestKey = k
				oldestTime = v.CreatedAt
			}
		}
		delete(c.entries, oldestKey)
	}
	
	c.entries[key] = &RouteCacheEntry{
		SubscriptionIDs: subscriptionIDs,
		CreatedAt:       time.Now(),
	}
}

func (c *RouteCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.entries = make(map[string]*RouteCacheEntry)
}

func (c *RouteCache) cleanup() {
	ticker := time.NewTicker(c.ttl / 2)
	defer ticker.Stop()
	
	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for key, entry := range c.entries {
			if now.Sub(entry.CreatedAt) > c.ttl {
				delete(c.entries, key)
			}
		}
		c.mu.Unlock()
	}
}