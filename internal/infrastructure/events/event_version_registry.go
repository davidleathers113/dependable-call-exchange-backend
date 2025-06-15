package events

import (
	"fmt"
	"sync"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/audit"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
)

// NewEventVersionRegistry creates a new event version registry
func NewEventVersionRegistry() *EventVersionRegistry {
	return &EventVersionRegistry{
		versions: make(map[audit.EventType]map[string]EventSchema),
	}
}

// Register registers an event schema for a specific type and version
func (r *EventVersionRegistry) Register(eventType audit.EventType, version string, schema EventSchema) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if r.versions[eventType] == nil {
		r.versions[eventType] = make(map[string]EventSchema)
	}
	
	r.versions[eventType][version] = schema
}

// GetSchema returns the schema for a specific event type and version
func (r *EventVersionRegistry) GetSchema(eventType audit.EventType, version string) (EventSchema, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	versions, exists := r.versions[eventType]
	if !exists {
		return EventSchema{}, errors.NewNotFoundError(
			fmt.Sprintf("no schemas registered for event type %s", eventType))
	}
	
	schema, exists := versions[version]
	if !exists {
		return EventSchema{}, errors.NewNotFoundError(
			fmt.Sprintf("schema version %s not found for event type %s", version, eventType))
	}
	
	return schema, nil
}

// GetLatestVersion returns the latest version for an event type
func (r *EventVersionRegistry) GetLatestVersion(eventType audit.EventType) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	versions, exists := r.versions[eventType]
	if !exists {
		return "", errors.NewNotFoundError(
			fmt.Sprintf("no schemas registered for event type %s", eventType))
	}
	
	if len(versions) == 0 {
		return "", errors.NewNotFoundError(
			fmt.Sprintf("no versions found for event type %s", eventType))
	}
	
	// For simplicity, return the "1.0" version
	// In a real implementation, you'd want proper version comparison
	if _, exists := versions["1.0"]; exists {
		return "1.0", nil
	}
	
	// Return any available version as fallback
	for version := range versions {
		return version, nil
	}
	
	return "", errors.NewNotFoundError("no versions available")
}

// GetSupportedVersions returns all supported versions for an event type
func (r *EventVersionRegistry) GetSupportedVersions(eventType audit.EventType) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	versions, exists := r.versions[eventType]
	if !exists {
		return []string{}
	}
	
	result := make([]string, 0, len(versions))
	for version := range versions {
		result = append(result, version)
	}
	
	return result
}

// IsVersionSupported checks if a version is supported for an event type
func (r *EventVersionRegistry) IsVersionSupported(eventType audit.EventType, version string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	versions, exists := r.versions[eventType]
	if !exists {
		return false
	}
	
	_, exists = versions[version]
	return exists
}

// GetAllEventTypes returns all registered event types
func (r *EventVersionRegistry) GetAllEventTypes() []audit.EventType {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	result := make([]audit.EventType, 0, len(r.versions))
	for eventType := range r.versions {
		result = append(result, eventType)
	}
	
	return result
}