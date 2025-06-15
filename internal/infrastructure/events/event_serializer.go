package events

import (
	"encoding/json"
	"fmt"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/audit"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
)

// JSONEventSerializer implements EventSerializer using JSON
type JSONEventSerializer struct {
	versionRegistry *EventVersionRegistry
}

// NewJSONEventSerializer creates a new JSON event serializer
func NewJSONEventSerializer(versionRegistry *EventVersionRegistry) *JSONEventSerializer {
	return &JSONEventSerializer{
		versionRegistry: versionRegistry,
	}
}

// Serialize serializes a domain event to JSON bytes
func (s *JSONEventSerializer) Serialize(event DNCDomainEvent) ([]byte, error) {
	// Create envelope with metadata
	envelope := EventEnvelope{
		EventID:     event.GetEventID(),
		EventType:   event.GetEventType(),
		Version:     event.GetEventVersion(),
		Timestamp:   event.GetTimestamp(),
		AggregateID: event.GetAggregateID(),
		AggregateType: event.GetAggregateType(),
		Data:        event,
	}
	
	data, err := json.Marshal(envelope)
	if err != nil {
		return nil, errors.NewInternalError("failed to serialize event").WithCause(err)
	}
	
	return data, nil
}

// Deserialize deserializes JSON bytes to a domain event
func (s *JSONEventSerializer) Deserialize(data []byte, eventType audit.EventType, version string) (DNCDomainEvent, error) {
	// Get schema for event type and version
	schema, err := s.versionRegistry.GetSchema(eventType, version)
	if err != nil {
		return nil, errors.NewValidationError("UNSUPPORTED_EVENT_VERSION",
			fmt.Sprintf("unsupported event type %s version %s", eventType, version)).WithCause(err)
	}
	
	// First unmarshal the envelope to get the event data
	var envelope EventEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, errors.NewValidationError("INVALID_EVENT_ENVELOPE",
			"failed to unmarshal event envelope").WithCause(err)
	}
	
	// Re-marshal just the Data portion for schema-specific deserialization
	eventData, err := json.Marshal(envelope.Data)
	if err != nil {
		return nil, errors.NewInternalError("failed to marshal event data").WithCause(err)
	}
	
	// Use schema deserializer
	event, err := schema.Deserializer(eventData)
	if err != nil {
		return nil, errors.NewValidationError("DESERIALIZATION_FAILED",
			"failed to deserialize event data").WithCause(err)
	}
	
	return event, nil
}

// GetSupportedVersions returns supported versions for an event type
func (s *JSONEventSerializer) GetSupportedVersions(eventType audit.EventType) []string {
	return s.versionRegistry.GetSupportedVersions(eventType)
}

// EventEnvelope wraps domain events with metadata for serialization
type EventEnvelope struct {
	EventID       interface{}       `json:"event_id"`
	EventType     audit.EventType   `json:"event_type"`
	Version       string            `json:"version"`
	Timestamp     interface{}       `json:"timestamp"`
	AggregateID   string            `json:"aggregate_id"`
	AggregateType string            `json:"aggregate_type"`
	Data          interface{}       `json:"data"`
}