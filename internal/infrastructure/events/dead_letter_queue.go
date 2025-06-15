package events

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// MemoryDeadLetterQueue implements DeadLetterQueue using in-memory storage
type MemoryDeadLetterQueue struct {
	logger     *zap.Logger
	maxSize    int
	
	// Storage
	failedEvents map[uuid.UUID]*FailedEvent
	mu          sync.RWMutex
	
	// Metrics
	totalAdded   int64
	totalRetried int64
	totalRemoved int64
}

// NewMemoryDeadLetterQueue creates a new in-memory dead letter queue
func NewMemoryDeadLetterQueue(maxSize int, logger *zap.Logger) *MemoryDeadLetterQueue {
	return &MemoryDeadLetterQueue{
		logger:       logger,
		maxSize:      maxSize,
		failedEvents: make(map[uuid.UUID]*FailedEvent),
	}
}

// Add adds a failed event to the dead letter queue
func (q *MemoryDeadLetterQueue) Add(
	ctx context.Context, 
	event DNCDomainEvent, 
	reason string, 
	attempts int,
) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	
	// Check size limit
	if len(q.failedEvents) >= q.maxSize {
		// Remove oldest entry to make space
		q.removeOldest()
	}
	
	eventID := event.GetEventID()
	now := time.Now()
	
	// Check if event already exists
	if existing, exists := q.failedEvents[eventID]; exists {
		// Update existing entry
		existing.Attempts = attempts
		existing.Reason = reason
		existing.LastFail = now
		
		q.logger.Debug("Updated failed event in dead letter queue",
			zap.String("event_id", eventID.String()),
			zap.String("reason", reason),
			zap.Int("attempts", attempts),
		)
	} else {
		// Add new entry
		q.failedEvents[eventID] = &FailedEvent{
			Event:     event,
			Reason:    reason,
			Attempts:  attempts,
			FirstFail: now,
			LastFail:  now,
		}
		
		q.totalAdded++
		
		q.logger.Info("Added failed event to dead letter queue",
			zap.String("event_id", eventID.String()),
			zap.String("event_type", string(event.GetEventType())),
			zap.String("reason", reason),
			zap.Int("attempts", attempts),
		)
	}
	
	return nil
}

// GetFailed retrieves failed events from the queue
func (q *MemoryDeadLetterQueue) GetFailed(ctx context.Context, limit int) ([]FailedEvent, error) {
	q.mu.RLock()
	defer q.mu.RUnlock()
	
	// If limit is 0, return count only (for metrics)
	if limit == 0 {
		result := make([]FailedEvent, 0, len(q.failedEvents))
		for _, failedEvent := range q.failedEvents {
			result = append(result, *failedEvent)
		}
		return result, nil
	}
	
	// Collect all failed events
	allEvents := make([]*FailedEvent, 0, len(q.failedEvents))
	for _, failedEvent := range q.failedEvents {
		allEvents = append(allEvents, failedEvent)
	}
	
	// Sort by last failure time (oldest first)
	sort.Slice(allEvents, func(i, j int) bool {
		return allEvents[i].LastFail.Before(allEvents[j].LastFail)
	})
	
	// Apply limit
	if limit > 0 && limit < len(allEvents) {
		allEvents = allEvents[:limit]
	}
	
	// Convert to value slice
	result := make([]FailedEvent, len(allEvents))
	for i, failedEvent := range allEvents {
		result[i] = *failedEvent
	}
	
	return result, nil
}

// Retry marks an event for retry (removes it from the dead letter queue)
func (q *MemoryDeadLetterQueue) Retry(ctx context.Context, eventID uuid.UUID) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	
	failedEvent, exists := q.failedEvents[eventID]
	if !exists {
		return errors.NewNotFoundError("failed event not found in dead letter queue")
	}
	
	delete(q.failedEvents, eventID)
	q.totalRetried++
	
	q.logger.Info("Retrying failed event from dead letter queue",
		zap.String("event_id", eventID.String()),
		zap.String("event_type", string(failedEvent.Event.GetEventType())),
		zap.Int("attempts", failedEvent.Attempts),
	)
	
	return nil
}

// Remove permanently removes an event from the dead letter queue
func (q *MemoryDeadLetterQueue) Remove(ctx context.Context, eventID uuid.UUID) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	
	if _, exists := q.failedEvents[eventID]; !exists {
		return errors.NewNotFoundError("failed event not found in dead letter queue")
	}
	
	delete(q.failedEvents, eventID)
	q.totalRemoved++
	
	q.logger.Info("Removed failed event from dead letter queue",
		zap.String("event_id", eventID.String()),
	)
	
	return nil
}

// GetStats returns dead letter queue statistics
func (q *MemoryDeadLetterQueue) GetStats() map[string]interface{} {
	q.mu.RLock()
	defer q.mu.RUnlock()
	
	return map[string]interface{}{
		"current_size":  len(q.failedEvents),
		"max_size":      q.maxSize,
		"total_added":   q.totalAdded,
		"total_retried": q.totalRetried,
		"total_removed": q.totalRemoved,
	}
}

// Cleanup removes old entries based on age
func (q *MemoryDeadLetterQueue) Cleanup(maxAge time.Duration) int {
	q.mu.Lock()
	defer q.mu.Unlock()
	
	cutoff := time.Now().Add(-maxAge)
	removed := 0
	
	for eventID, failedEvent := range q.failedEvents {
		if failedEvent.FirstFail.Before(cutoff) {
			delete(q.failedEvents, eventID)
			removed++
		}
	}
	
	if removed > 0 {
		q.logger.Info("Cleaned up old dead letter queue entries",
			zap.Int("removed", removed),
			zap.Duration("max_age", maxAge),
		)
	}
	
	return removed
}

// Private methods

func (q *MemoryDeadLetterQueue) removeOldest() {
	if len(q.failedEvents) == 0 {
		return
	}
	
	var oldestID uuid.UUID
	var oldestTime time.Time
	
	for eventID, failedEvent := range q.failedEvents {
		if oldestTime.IsZero() || failedEvent.FirstFail.Before(oldestTime) {
			oldestID = eventID
			oldestTime = failedEvent.FirstFail
		}
	}
	
	delete(q.failedEvents, oldestID)
	
	q.logger.Debug("Removed oldest entry from dead letter queue to make space",
		zap.String("event_id", oldestID.String()),
	)
}

// PersistentDeadLetterQueue implements DeadLetterQueue using persistent storage
type PersistentDeadLetterQueue struct {
	logger *zap.Logger
	// TODO: Add database connection and implement persistent storage
}

// NewPersistentDeadLetterQueue creates a persistent dead letter queue
func NewPersistentDeadLetterQueue(logger *zap.Logger) *PersistentDeadLetterQueue {
	return &PersistentDeadLetterQueue{
		logger: logger,
	}
}

// Add adds a failed event to persistent storage
func (q *PersistentDeadLetterQueue) Add(
	ctx context.Context, 
	event DNCDomainEvent, 
	reason string, 
	attempts int,
) error {
	// TODO: Implement persistent storage
	// This would typically involve:
	// 1. Serialize the event
	// 2. Store in database table with metadata
	// 3. Handle concurrent access
	
	q.logger.Info("Would add to persistent dead letter queue",
		zap.String("event_id", event.GetEventID().String()),
		zap.String("reason", reason),
		zap.Int("attempts", attempts),
	)
	
	return nil
}

// GetFailed retrieves failed events from persistent storage
func (q *PersistentDeadLetterQueue) GetFailed(ctx context.Context, limit int) ([]FailedEvent, error) {
	// TODO: Implement persistent retrieval
	return []FailedEvent{}, nil
}

// Retry marks an event for retry in persistent storage
func (q *PersistentDeadLetterQueue) Retry(ctx context.Context, eventID uuid.UUID) error {
	// TODO: Implement persistent retry
	return nil
}

// Remove permanently removes an event from persistent storage
func (q *PersistentDeadLetterQueue) Remove(ctx context.Context, eventID uuid.UUID) error {
	// TODO: Implement persistent removal
	return nil
}