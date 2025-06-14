package database

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/consent"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
)

// ConsentEventStore implements the consent.EventStore interface
type ConsentEventStore struct {
	db *pgxpool.Pool
}

// NewConsentEventStore creates a new PostgreSQL event store
func NewConsentEventStore(db *pgxpool.Pool) *ConsentEventStore {
	return &ConsentEventStore{db: db}
}

// SaveEvents stores domain events
func (s *ConsentEventStore) SaveEvents(ctx context.Context, events []interface{}) error {
	if len(events) == 0 {
		return nil
	}

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return errors.NewInternalError("failed to begin transaction").WithCause(err)
	}
	defer tx.Rollback(ctx)

	for _, event := range events {
		// Determine aggregate ID and version based on event type
		aggregateID, version, err := s.extractEventMetadata(event)
		if err != nil {
			return err
		}

		eventData, err := json.Marshal(event)
		if err != nil {
			return errors.NewInternalError("failed to marshal event").WithCause(err)
		}

		eventType := fmt.Sprintf("%T", event)
		
		_, err = tx.Exec(ctx, `
			INSERT INTO consent_events (
				id, aggregate_id, event_type, event_data, version, occurred_at
			) VALUES (gen_random_uuid(), $1, $2, $3, $4, NOW())
		`, aggregateID, eventType, eventData, version)
		
		if err != nil {
			return errors.NewInternalError("failed to insert event").WithCause(err)
		}
	}

	return tx.Commit(ctx)
}

// GetEvents retrieves events for an aggregate
func (s *ConsentEventStore) GetEvents(ctx context.Context, aggregateID uuid.UUID) ([]interface{}, error) {
	rows, err := s.db.Query(ctx, `
		SELECT event_type, event_data, version, occurred_at
		FROM consent_events
		WHERE aggregate_id = $1
		ORDER BY version ASC, occurred_at ASC
	`, aggregateID)
	if err != nil {
		return nil, errors.NewInternalError("failed to query events").WithCause(err)
	}
	defer rows.Close()

	var events []interface{}
	for rows.Next() {
		var eventType string
		var eventData json.RawMessage
		var version int
		var occurredAt interface{} // timestamp, but we don't need it for reconstruction
		
		err := rows.Scan(&eventType, &eventData, &version, &occurredAt)
		if err != nil {
			return nil, errors.NewInternalError("failed to scan event").WithCause(err)
		}

		event, err := s.deserializeEvent(eventType, eventData)
		if err != nil {
			return nil, err
		}

		events = append(events, event)
	}

	return events, nil
}

// GetEventsByType retrieves events of a specific type
func (s *ConsentEventStore) GetEventsByType(ctx context.Context, eventType string, limit int) ([]interface{}, error) {
	query := `
		SELECT event_data, aggregate_id, version, occurred_at
		FROM consent_events
		WHERE event_type = $1
		ORDER BY occurred_at DESC
	`
	
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", limit)
	}

	rows, err := s.db.Query(ctx, query, eventType)
	if err != nil {
		return nil, errors.NewInternalError("failed to query events by type").WithCause(err)
	}
	defer rows.Close()

	var events []interface{}
	for rows.Next() {
		var eventData json.RawMessage
		var aggregateID uuid.UUID
		var version int
		var occurredAt interface{}
		
		err := rows.Scan(&eventData, &aggregateID, &version, &occurredAt)
		if err != nil {
			return nil, errors.NewInternalError("failed to scan event").WithCause(err)
		}

		event, err := s.deserializeEvent(eventType, eventData)
		if err != nil {
			return nil, err
		}

		events = append(events, event)
	}

	return events, nil
}

// extractEventMetadata extracts aggregate ID and version from event
func (s *ConsentEventStore) extractEventMetadata(event interface{}) (uuid.UUID, int, error) {
	switch e := event.(type) {
	case consent.ConsentCreatedEvent:
		return e.ConsentID, 1, nil
	case consent.ConsentActivatedEvent:
		return e.ConsentID, 1, nil // Events don't have version field, use aggregate version
	case consent.ConsentRevokedEvent:
		return e.ConsentID, 1, nil
	case consent.ConsentUpdatedEvent:
		return e.ConsentID, 1, nil
	default:
		return uuid.Nil, 0, errors.NewInternalError(fmt.Sprintf("unknown event type: %T", event))
	}
}

// deserializeEvent deserializes event data based on type
func (s *ConsentEventStore) deserializeEvent(eventType string, data json.RawMessage) (interface{}, error) {
	switch eventType {
	case "consent.ConsentCreatedEvent":
		var event consent.ConsentCreatedEvent
		if err := json.Unmarshal(data, &event); err != nil {
			return nil, errors.NewInternalError("failed to unmarshal ConsentCreatedEvent").WithCause(err)
		}
		return event, nil
		
	case "consent.ConsentActivatedEvent":
		var event consent.ConsentActivatedEvent
		if err := json.Unmarshal(data, &event); err != nil {
			return nil, errors.NewInternalError("failed to unmarshal ConsentActivatedEvent").WithCause(err)
		}
		return event, nil
		
	case "consent.ConsentRevokedEvent":
		var event consent.ConsentRevokedEvent
		if err := json.Unmarshal(data, &event); err != nil {
			return nil, errors.NewInternalError("failed to unmarshal ConsentRevokedEvent").WithCause(err)
		}
		return event, nil
		
	case "consent.ConsentUpdatedEvent":
		var event consent.ConsentUpdatedEvent
		if err := json.Unmarshal(data, &event); err != nil {
			return nil, errors.NewInternalError("failed to unmarshal ConsentUpdatedEvent").WithCause(err)
		}
		return event, nil
		
	default:
		return nil, errors.NewInternalError(fmt.Sprintf("unknown event type for deserialization: %s", eventType))
	}
}