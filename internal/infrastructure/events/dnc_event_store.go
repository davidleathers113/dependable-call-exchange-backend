package events

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/audit"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// PostgresDNCEventStore implements DNCEventStore using PostgreSQL
type PostgresDNCEventStore struct {
	db         *sql.DB
	serializer EventSerializer
	logger     *zap.Logger
	
	// Configuration
	batchSize      int
	streamingDelay time.Duration
	
	// Active streams
	activeStreams map[string]chan DNCDomainEvent
	streamsMu     sync.RWMutex
}

// NewPostgresDNCEventStore creates a new PostgreSQL-based event store
func NewPostgresDNCEventStore(
	db *sql.DB,
	serializer EventSerializer,
	logger *zap.Logger,
) *PostgresDNCEventStore {
	store := &PostgresDNCEventStore{
		db:             db,
		serializer:     serializer,
		logger:         logger,
		batchSize:      100,
		streamingDelay: 100 * time.Millisecond,
		activeStreams:  make(map[string]chan DNCDomainEvent),
	}
	
	// Initialize database schema
	if err := store.initSchema(); err != nil {
		logger.Error("Failed to initialize event store schema", zap.Error(err))
	}
	
	return store
}

// Store stores a DNC domain event
func (s *PostgresDNCEventStore) Store(ctx context.Context, event DNCDomainEvent) error {
	data, err := s.serializer.Serialize(event)
	if err != nil {
		return errors.NewInternalError("failed to serialize event").WithCause(err)
	}
	
	query := `
		INSERT INTO dnc_events (
			event_id, 
			event_type, 
			event_version, 
			aggregate_id, 
			aggregate_type, 
			event_data, 
			created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	
	_, err = s.db.ExecContext(ctx, query,
		event.GetEventID(),
		string(event.GetEventType()),
		event.GetEventVersion(),
		event.GetAggregateID(),
		event.GetAggregateType(),
		data,
		event.GetTimestamp(),
	)
	
	if err != nil {
		return errors.NewInternalError("failed to store event").WithCause(err)
	}
	
	// Notify active streams
	s.notifyStreams(event)
	
	return nil
}

// Get retrieves a DNC domain event by ID
func (s *PostgresDNCEventStore) Get(ctx context.Context, eventID uuid.UUID) (DNCDomainEvent, error) {
	query := `
		SELECT event_type, event_version, event_data 
		FROM dnc_events 
		WHERE event_id = $1
	`
	
	var eventType string
	var eventVersion string
	var eventData []byte
	
	err := s.db.QueryRowContext(ctx, query, eventID).Scan(
		&eventType, &eventVersion, &eventData)
	
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.NewNotFoundError("event not found")
		}
		return nil, errors.NewInternalError("failed to query event").WithCause(err)
	}
	
	event, err := s.serializer.Deserialize(eventData, audit.EventType(eventType), eventVersion)
	if err != nil {
		return nil, errors.NewInternalError("failed to deserialize event").WithCause(err)
	}
	
	return event, nil
}

// GetByAggregateID retrieves events for a specific aggregate
func (s *PostgresDNCEventStore) GetByAggregateID(
	ctx context.Context, 
	aggregateID string, 
	fromVersion int,
) ([]DNCDomainEvent, error) {
	query := `
		SELECT event_type, event_version, event_data 
		FROM dnc_events 
		WHERE aggregate_id = $1 AND sequence_number >= $2
		ORDER BY sequence_number ASC
	`
	
	rows, err := s.db.QueryContext(ctx, query, aggregateID, fromVersion)
	if err != nil {
		return nil, errors.NewInternalError("failed to query events by aggregate").WithCause(err)
	}
	defer rows.Close()
	
	var events []DNCDomainEvent
	
	for rows.Next() {
		var eventType string
		var eventVersion string
		var eventData []byte
		
		if err := rows.Scan(&eventType, &eventVersion, &eventData); err != nil {
			return nil, errors.NewInternalError("failed to scan event row").WithCause(err)
		}
		
		event, err := s.serializer.Deserialize(eventData, audit.EventType(eventType), eventVersion)
		if err != nil {
			s.logger.Error("Failed to deserialize event, skipping",
				zap.Error(err),
				zap.String("event_type", eventType),
				zap.String("aggregate_id", aggregateID),
			)
			continue
		}
		
		events = append(events, event)
	}
	
	if err := rows.Err(); err != nil {
		return nil, errors.NewInternalError("row iteration error").WithCause(err)
	}
	
	return events, nil
}

// GetEventStream returns a channel of events from a specific timestamp
func (s *PostgresDNCEventStore) GetEventStream(
	ctx context.Context, 
	fromTimestamp time.Time,
) (<-chan DNCDomainEvent, error) {
	streamID := uuid.New().String()
	eventChan := make(chan DNCDomainEvent, 100)
	
	// Register stream
	s.streamsMu.Lock()
	s.activeStreams[streamID] = eventChan
	s.streamsMu.Unlock()
	
	// Start streaming goroutine
	go s.streamEvents(ctx, streamID, eventChan, fromTimestamp)
	
	return eventChan, nil
}

// Close closes the event store and cleans up resources
func (s *PostgresDNCEventStore) Close() error {
	// Close all active streams
	s.streamsMu.Lock()
	for streamID, eventChan := range s.activeStreams {
		close(eventChan)
		delete(s.activeStreams, streamID)
	}
	s.streamsMu.Unlock()
	
	// Database connection is managed externally
	return nil
}

// Private methods

func (s *PostgresDNCEventStore) initSchema() error {
	schema := `
		CREATE TABLE IF NOT EXISTS dnc_events (
			id BIGSERIAL PRIMARY KEY,
			event_id UUID UNIQUE NOT NULL,
			event_type VARCHAR(100) NOT NULL,
			event_version VARCHAR(10) NOT NULL,
			aggregate_id VARCHAR(255) NOT NULL,
			aggregate_type VARCHAR(100) NOT NULL,
			sequence_number BIGINT DEFAULT 0,
			event_data JSONB NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE NOT NULL,
			processed_at TIMESTAMP WITH TIME ZONE DEFAULT NULL
		);
		
		-- Indexes for performance
		CREATE INDEX IF NOT EXISTS idx_dnc_events_aggregate_id ON dnc_events(aggregate_id);
		CREATE INDEX IF NOT EXISTS idx_dnc_events_event_type ON dnc_events(event_type);
		CREATE INDEX IF NOT EXISTS idx_dnc_events_created_at ON dnc_events(created_at);
		CREATE INDEX IF NOT EXISTS idx_dnc_events_sequence ON dnc_events(aggregate_id, sequence_number);
		
		-- Trigger for auto-incrementing sequence numbers per aggregate
		CREATE OR REPLACE FUNCTION update_sequence_number()
		RETURNS TRIGGER AS $$
		BEGIN
			SELECT COALESCE(MAX(sequence_number), 0) + 1
			INTO NEW.sequence_number
			FROM dnc_events
			WHERE aggregate_id = NEW.aggregate_id;
			
			RETURN NEW;
		END;
		$$ LANGUAGE plpgsql;
		
		DROP TRIGGER IF EXISTS trg_update_sequence_number ON dnc_events;
		CREATE TRIGGER trg_update_sequence_number
			BEFORE INSERT ON dnc_events
			FOR EACH ROW
			EXECUTE FUNCTION update_sequence_number();
	`
	
	_, err := s.db.Exec(schema)
	if err != nil {
		return fmt.Errorf("failed to initialize schema: %w", err)
	}
	
	return nil
}

func (s *PostgresDNCEventStore) streamEvents(
	ctx context.Context,
	streamID string,
	eventChan chan DNCDomainEvent,
	fromTimestamp time.Time,
) {
	defer func() {
		// Cleanup on exit
		s.streamsMu.Lock()
		delete(s.activeStreams, streamID)
		s.streamsMu.Unlock()
		
		close(eventChan)
	}()
	
	ticker := time.NewTicker(s.streamingDelay)
	defer ticker.Stop()
	
	lastTimestamp := fromTimestamp
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Query for new events since last timestamp
			events, newLastTimestamp, err := s.getEventsSince(ctx, lastTimestamp)
			if err != nil {
				s.logger.Error("Failed to get events for stream",
					zap.Error(err),
					zap.String("stream_id", streamID),
				)
				continue
			}
			
			// Send events to channel
			for _, event := range events {
				select {
				case eventChan <- event:
				case <-ctx.Done():
					return
				default:
					// Channel full, log warning
					s.logger.Warn("Event stream channel full, dropping event",
						zap.String("stream_id", streamID),
						zap.String("event_id", event.GetEventID().String()),
					)
				}
			}
			
			if !newLastTimestamp.IsZero() {
				lastTimestamp = newLastTimestamp
			}
		}
	}
}

func (s *PostgresDNCEventStore) getEventsSince(
	ctx context.Context, 
	since time.Time,
) ([]DNCDomainEvent, time.Time, error) {
	query := `
		SELECT event_type, event_version, event_data, created_at
		FROM dnc_events 
		WHERE created_at > $1
		ORDER BY created_at ASC
		LIMIT $2
	`
	
	rows, err := s.db.QueryContext(ctx, query, since, s.batchSize)
	if err != nil {
		return nil, time.Time{}, err
	}
	defer rows.Close()
	
	var events []DNCDomainEvent
	var lastTimestamp time.Time
	
	for rows.Next() {
		var eventType string
		var eventVersion string
		var eventData []byte
		var createdAt time.Time
		
		if err := rows.Scan(&eventType, &eventVersion, &eventData, &createdAt); err != nil {
			return nil, time.Time{}, err
		}
		
		event, err := s.serializer.Deserialize(eventData, audit.EventType(eventType), eventVersion)
		if err != nil {
			s.logger.Error("Failed to deserialize event in stream, skipping",
				zap.Error(err),
				zap.String("event_type", eventType),
			)
			continue
		}
		
		events = append(events, event)
		lastTimestamp = createdAt
	}
	
	if err := rows.Err(); err != nil {
		return nil, time.Time{}, err
	}
	
	return events, lastTimestamp, nil
}

func (s *PostgresDNCEventStore) notifyStreams(event DNCDomainEvent) {
	s.streamsMu.RLock()
	defer s.streamsMu.RUnlock()
	
	for streamID, eventChan := range s.activeStreams {
		select {
		case eventChan <- event:
		default:
			// Channel full, log warning
			s.logger.Warn("Event stream channel full during notification",
				zap.String("stream_id", streamID),
				zap.String("event_id", event.GetEventID().String()),
			)
		}
	}
}

// MemoryDNCEventStore provides an in-memory implementation for testing
type MemoryDNCEventStore struct {
	events     map[uuid.UUID]DNCDomainEvent
	aggregates map[string][]DNCDomainEvent
	serializer EventSerializer
	mu         sync.RWMutex
	
	// Streaming
	activeStreams map[string]chan DNCDomainEvent
	streamsMu     sync.RWMutex
}

// NewMemoryDNCEventStore creates a new in-memory event store
func NewMemoryDNCEventStore(serializer EventSerializer) *MemoryDNCEventStore {
	return &MemoryDNCEventStore{
		events:        make(map[uuid.UUID]DNCDomainEvent),
		aggregates:    make(map[string][]DNCDomainEvent),
		serializer:    serializer,
		activeStreams: make(map[string]chan DNCDomainEvent),
	}
}

// Store stores a DNC domain event in memory
func (s *MemoryDNCEventStore) Store(ctx context.Context, event DNCDomainEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// Simulate serialization/deserialization for testing
	data, err := s.serializer.Serialize(event)
	if err != nil {
		return err
	}
	
	deserializedEvent, err := s.serializer.Deserialize(data, event.GetEventType(), event.GetEventVersion())
	if err != nil {
		return err
	}
	
	s.events[event.GetEventID()] = deserializedEvent
	
	aggregateID := event.GetAggregateID()
	s.aggregates[aggregateID] = append(s.aggregates[aggregateID], deserializedEvent)
	
	// Notify streams
	s.notifyStreams(deserializedEvent)
	
	return nil
}

// Get retrieves a DNC domain event by ID from memory
func (s *MemoryDNCEventStore) Get(ctx context.Context, eventID uuid.UUID) (DNCDomainEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	event, exists := s.events[eventID]
	if !exists {
		return nil, errors.NewNotFoundError("event not found")
	}
	
	return event, nil
}

// GetByAggregateID retrieves events for a specific aggregate from memory
func (s *MemoryDNCEventStore) GetByAggregateID(
	ctx context.Context, 
	aggregateID string, 
	fromVersion int,
) ([]DNCDomainEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	events, exists := s.aggregates[aggregateID]
	if !exists {
		return []DNCDomainEvent{}, nil
	}
	
	// Filter by version (simplified)
	if fromVersion > 0 && fromVersion < len(events) {
		return events[fromVersion:], nil
	}
	
	return events, nil
}

// GetEventStream returns a channel of events from memory
func (s *MemoryDNCEventStore) GetEventStream(
	ctx context.Context, 
	fromTimestamp time.Time,
) (<-chan DNCDomainEvent, error) {
	streamID := uuid.New().String()
	eventChan := make(chan DNCDomainEvent, 100)
	
	// Register stream
	s.streamsMu.Lock()
	s.activeStreams[streamID] = eventChan
	s.streamsMu.Unlock()
	
	// Send existing events matching criteria
	go func() {
		defer func() {
			s.streamsMu.Lock()
			delete(s.activeStreams, streamID)
			s.streamsMu.Unlock()
			close(eventChan)
		}()
		
		s.mu.RLock()
		for _, event := range s.events {
			if event.GetTimestamp().After(fromTimestamp) {
				select {
				case eventChan <- event:
				case <-ctx.Done():
					s.mu.RUnlock()
					return
				}
			}
		}
		s.mu.RUnlock()
		
		// Keep stream open for new events
		<-ctx.Done()
	}()
	
	return eventChan, nil
}

// Close closes the in-memory event store
func (s *MemoryDNCEventStore) Close() error {
	s.streamsMu.Lock()
	for streamID, eventChan := range s.activeStreams {
		close(eventChan)
		delete(s.activeStreams, streamID)
	}
	s.streamsMu.Unlock()
	
	return nil
}

func (s *MemoryDNCEventStore) notifyStreams(event DNCDomainEvent) {
	s.streamsMu.RLock()
	defer s.streamsMu.RUnlock()
	
	for _, eventChan := range s.activeStreams {
		select {
		case eventChan <- event:
		default:
			// Channel full, skip
		}
	}
}