package repository

import (
	"encoding/json"
	"fmt"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/call"
)

// CallMetadata represents the structured metadata for a call
type CallMetadata struct {
	CallSID   string        `json:"call_sid"`
	SessionID *string       `json:"session_id,omitempty"`
	UserAgent *string       `json:"user_agent,omitempty"`
	IPAddress *string       `json:"ip_address,omitempty"`
	Location  *CallLocation `json:"location,omitempty"`
}

// CallLocation represents location metadata
type CallLocation struct {
	Country   string  `json:"country"`
	State     string  `json:"state"`
	City      string  `json:"city"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Timezone  string  `json:"timezone"`
}

// ParseCallMetadata safely parses JSON metadata into structured data
func ParseCallMetadata(metadataJSON []byte) (*CallMetadata, error) {
	if len(metadataJSON) == 0 {
		return &CallMetadata{}, nil
	}

	var metadata CallMetadata
	if err := json.Unmarshal(metadataJSON, &metadata); err != nil {
		return nil, fmt.Errorf("failed to parse call metadata: %w", err)
	}

	return &metadata, nil
}

// ToCallLocation converts CallLocation to domain Location
func (cl *CallLocation) ToCallLocation() *call.Location {
	if cl == nil {
		return nil
	}

	return &call.Location{
		Country:   cl.Country,
		State:     cl.State,
		City:      cl.City,
		Latitude:  cl.Latitude,
		Longitude: cl.Longitude,
		Timezone:  cl.Timezone,
	}
}

// ApplyToCall applies parsed metadata to a call entity
func (cm *CallMetadata) ApplyToCall(c *call.Call) {
	if cm == nil {
		return
	}

	c.CallSID = cm.CallSID
	c.SessionID = cm.SessionID
	c.UserAgent = cm.UserAgent
	c.IPAddress = cm.IPAddress
	c.Location = cm.Location.ToCallLocation()
}

// SerializeCallMetadata converts call metadata to JSON
func SerializeCallMetadata(c *call.Call) ([]byte, error) {
	metadata := CallMetadata{
		CallSID:   c.CallSID,
		SessionID: c.SessionID,
		UserAgent: c.UserAgent,
		IPAddress: c.IPAddress,
	}

	if c.Location != nil {
		metadata.Location = &CallLocation{
			Country:   c.Location.Country,
			State:     c.Location.State,
			City:      c.Location.City,
			Latitude:  c.Location.Latitude,
			Longitude: c.Location.Longitude,
			Timezone:  c.Location.Timezone,
		}
	}

	return json.Marshal(metadata)
}
