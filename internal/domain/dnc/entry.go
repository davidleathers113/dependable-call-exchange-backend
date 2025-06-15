package dnc

import (
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/google/uuid"
)

// DNCEntry represents a phone number on the Do Not Call list
// This is the core domain entity for managing Do Not Call registrations
type DNCEntry struct {
	ID             uuid.UUID                `json:"id"`
	PhoneNumber    values.PhoneNumber       `json:"phone_number"`
	ListSource     values.ListSource        `json:"list_source"`
	SuppressReason values.SuppressReason    `json:"suppress_reason"`
	AddedAt        time.Time                `json:"added_at"`
	ExpiresAt      *time.Time               `json:"expires_at,omitempty"`
	
	// Metadata
	SourceReference *string                  `json:"source_reference,omitempty"` // External ID from provider
	Notes           *string                  `json:"notes,omitempty"`
	Metadata        map[string]string        `json:"metadata,omitempty"`
	
	// Audit fields
	AddedBy   uuid.UUID  `json:"added_by"`
	UpdatedAt time.Time  `json:"updated_at"`
	UpdatedBy *uuid.UUID `json:"updated_by,omitempty"`
}

// NewDNCEntry creates a new DNC entry with validation
// All business rules and validation are enforced in the constructor
func NewDNCEntry(phoneNumber string, source string, reason string, addedBy uuid.UUID) (*DNCEntry, error) {
	// Validate phone number using value object
	phone, err := values.NewPhoneNumber(phoneNumber)
	if err != nil {
		return nil, errors.NewValidationError("INVALID_PHONE_NUMBER", "invalid phone number format").WithCause(err)
	}

	// Validate list source using value object
	listSource, err := values.NewListSource(source)
	if err != nil {
		return nil, errors.NewValidationError("INVALID_LIST_SOURCE", "invalid list source").WithCause(err)
	}

	// Validate suppress reason using value object
	suppressReason, err := values.NewSuppressReason(reason)
	if err != nil {
		return nil, errors.NewValidationError("INVALID_SUPPRESS_REASON", "invalid suppress reason").WithCause(err)
	}

	// Validate addedBy
	if addedBy == uuid.Nil {
		return nil, errors.NewValidationError("INVALID_USER", "added by user ID cannot be empty")
	}

	now := time.Now().UTC()
	return &DNCEntry{
		ID:             uuid.New(),
		PhoneNumber:    phone,
		ListSource:     listSource,
		SuppressReason: suppressReason,
		AddedAt:        now,
		AddedBy:        addedBy,
		UpdatedAt:      now,
		Metadata:       make(map[string]string),
	}, nil
}

// SetExpiration sets the expiration time for the DNC entry
func (e *DNCEntry) SetExpiration(expiresAt time.Time) error {
	if expiresAt.Before(e.AddedAt) {
		return errors.NewValidationError("INVALID_EXPIRATION", "expiration date cannot be before added date")
	}

	if expiresAt.Before(time.Now()) {
		return errors.NewValidationError("INVALID_EXPIRATION", "expiration date cannot be in the past")
	}

	e.ExpiresAt = &expiresAt
	e.UpdatedAt = time.Now().UTC()
	return nil
}

// SetSourceReference sets the external reference ID from the provider
func (e *DNCEntry) SetSourceReference(ref string) {
	e.SourceReference = &ref
	e.UpdatedAt = time.Now().UTC()
}

// AddNote adds a note to the DNC entry
func (e *DNCEntry) AddNote(note string, updatedBy uuid.UUID) error {
	if updatedBy == uuid.Nil {
		return errors.NewValidationError("INVALID_USER", "updated by user ID cannot be empty")
	}

	e.Notes = &note
	e.UpdatedBy = &updatedBy
	e.UpdatedAt = time.Now().UTC()
	return nil
}

// SetMetadata sets metadata key-value pairs
func (e *DNCEntry) SetMetadata(key, value string) {
	if e.Metadata == nil {
		e.Metadata = make(map[string]string)
	}
	e.Metadata[key] = value
	e.UpdatedAt = time.Now().UTC()
}

// IsExpired checks if the DNC entry has expired
func (e *DNCEntry) IsExpired() bool {
	if e.ExpiresAt == nil {
		return false // No expiration means permanent
	}
	return time.Now().After(*e.ExpiresAt)
}

// IsActive checks if the DNC entry is currently active
func (e *DNCEntry) IsActive() bool {
	return !e.IsExpired()
}

// CanCall determines if a call can be made to this number
func (e *DNCEntry) CanCall() bool {
	// Cannot call if entry is active (not expired)
	return !e.IsActive()
}

// GetComplianceInfo returns compliance-relevant information
func (e *DNCEntry) GetComplianceInfo() map[string]interface{} {
	info := map[string]interface{}{
		"phone_number":    e.PhoneNumber.String(),
		"list_source":     string(e.ListSource),
		"suppress_reason": string(e.SuppressReason),
		"added_at":        e.AddedAt,
		"is_active":       e.IsActive(),
		"can_call":        e.CanCall(),
	}

	if e.ExpiresAt != nil {
		info["expires_at"] = *e.ExpiresAt
	}

	if e.SourceReference != nil {
		info["source_reference"] = *e.SourceReference
	}

	return info
}

// TimeUntilExpiration returns the duration until the entry expires
func (e *DNCEntry) TimeUntilExpiration() *time.Duration {
	if e.ExpiresAt == nil {
		return nil // No expiration
	}

	duration := time.Until(*e.ExpiresAt)
	if duration < 0 {
		duration = 0 // Already expired
	}

	return &duration
}

// GetPriority returns the priority level of this DNC entry for conflict resolution
// Higher priority entries take precedence in business rules
func (e *DNCEntry) GetPriority() int {
	return e.ListSource.AuthorityLevel()
}

// GetComplianceCode returns the regulatory compliance code for this entry
func (e *DNCEntry) GetComplianceCode() string {
	return e.SuppressReason.GetComplianceCode()
}

// RequiresDocumentation checks if this entry requires compliance documentation
func (e *DNCEntry) RequiresDocumentation() bool {
	return e.SuppressReason.RequiresDocumentation()
}

// GetRetentionDays returns how long this entry must be retained for compliance
func (e *DNCEntry) GetRetentionDays() int {
	return e.SuppressReason.GetRetentionDays()
}

// IsTemporary checks if this DNC entry is temporary (has expiration)
func (e *DNCEntry) IsTemporary() bool {
	return e.ExpiresAt != nil
}

// IsPermanent checks if this DNC entry is permanent (no expiration)
func (e *DNCEntry) IsPermanent() bool {
	return e.ExpiresAt == nil
}