package events

import (
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/audit"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/dnc"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/google/uuid"
)

// NumberSuppressedEvent represents when a phone number is added to the DNC list
type NumberSuppressedEvent struct {
	// Base event information
	EventID       uuid.UUID                    `json:"event_id"`
	EventType     audit.EventType              `json:"event_type"`
	EventVersion  string                       `json:"event_version"`
	Timestamp     time.Time                    `json:"timestamp"`
	
	// DNC specific fields
	PhoneNumber   values.PhoneNumber           `json:"phone_number"`
	Reason        dnc.SuppressReason          `json:"reason"`
	Source        dnc.ListSource              `json:"source"`
	SuppressedAt  time.Time                    `json:"suppressed_at"`
	SuppressedBy  uuid.UUID                    `json:"suppressed_by"`
	
	// Additional context
	ExpiresAt     *time.Time                   `json:"expires_at,omitempty"`
	SourceRef     *string                      `json:"source_reference,omitempty"`
	Notes         *string                      `json:"notes,omitempty"`
	
	// Audit trail metadata
	RequestID     string                       `json:"request_id"`
	SessionID     string                       `json:"session_id,omitempty"`
	UserAgent     string                       `json:"user_agent,omitempty"`
	IPAddress     string                       `json:"ip_address,omitempty"`
	
	// Compliance metadata
	ComplianceFlags map[string]bool            `json:"compliance_flags,omitempty"`
	DataClasses     []string                   `json:"data_classes,omitempty"`
	LegalBasis      string                     `json:"legal_basis,omitempty"`
	
	// System metadata
	ProcessingTime  time.Duration              `json:"processing_time,omitempty"`
	BatchID        *string                     `json:"batch_id,omitempty"`
	
	// Associated DNC entry ID
	DNCEntryID     uuid.UUID                   `json:"dnc_entry_id"`
}

// NewNumberSuppressedEvent creates a new number suppressed event
func NewNumberSuppressedEvent(
	phoneNumber string,
	reason dnc.SuppressReason,
	source dnc.ListSource,
	suppressedBy uuid.UUID,
	dncEntryID uuid.UUID,
) (*NumberSuppressedEvent, error) {
	// Validate phone number
	phone, err := values.NewPhoneNumber(phoneNumber)
	if err != nil {
		return nil, errors.NewValidationError("INVALID_PHONE_NUMBER", 
			"phone number must be valid E.164 format").WithCause(err)
	}
	
	// Validate suppressedBy user ID
	if suppressedBy == uuid.Nil {
		return nil, errors.NewValidationError("INVALID_USER_ID", 
			"suppressed by user ID cannot be empty")
	}
	
	// Validate DNC entry ID
	if dncEntryID == uuid.Nil {
		return nil, errors.NewValidationError("INVALID_DNC_ENTRY_ID", 
			"DNC entry ID cannot be empty")
	}
	
	now := time.Now().UTC()
	
	event := &NumberSuppressedEvent{
		EventID:         uuid.New(),
		EventType:       audit.EventDNCNumberSuppressed,
		EventVersion:    "1.0",
		Timestamp:       now,
		PhoneNumber:     phone,
		Reason:          reason,
		Source:          source,
		SuppressedAt:    now,
		SuppressedBy:    suppressedBy,
		DNCEntryID:      dncEntryID,
		ComplianceFlags: make(map[string]bool),
		DataClasses:     []string{"phone_number", "dnc_status"},
		LegalBasis:      "legitimate_interest",
	}
	
	// Set compliance flags based on source and reason
	event.setComplianceFlags()
	
	return event, nil
}

// GetEventType returns the event type
func (e *NumberSuppressedEvent) GetEventType() audit.EventType {
	return audit.EventDNCNumberSuppressed
}

// GetEventVersion returns the event version
func (e *NumberSuppressedEvent) GetEventVersion() string {
	return e.EventVersion
}

// GetEventID returns the event ID
func (e *NumberSuppressedEvent) GetEventID() uuid.UUID {
	return e.EventID
}

// GetTimestamp returns the event timestamp
func (e *NumberSuppressedEvent) GetTimestamp() time.Time {
	return e.Timestamp
}

// GetAggregateID returns the phone number as the aggregate ID
func (e *NumberSuppressedEvent) GetAggregateID() string {
	return e.PhoneNumber.String()
}

// GetAggregateType returns the aggregate type
func (e *NumberSuppressedEvent) GetAggregateType() string {
	return "phone_number"
}

// SetExpiration sets the expiration time for the suppression
func (e *NumberSuppressedEvent) SetExpiration(expiresAt time.Time) error {
	if expiresAt.Before(e.SuppressedAt) {
		return errors.NewValidationError("INVALID_EXPIRATION", 
			"expiration cannot be before suppression time")
	}
	
	e.ExpiresAt = &expiresAt
	return nil
}

// SetSourceReference sets the external source reference
func (e *NumberSuppressedEvent) SetSourceReference(ref string) {
	e.SourceRef = &ref
}

// SetNotes sets additional notes for the suppression
func (e *NumberSuppressedEvent) SetNotes(notes string) {
	e.Notes = &notes
}

// SetRequestContext sets the request context information
func (e *NumberSuppressedEvent) SetRequestContext(requestID, sessionID, userAgent, ipAddress string) {
	e.RequestID = requestID
	if sessionID != "" {
		e.SessionID = sessionID
	}
	if userAgent != "" {
		e.UserAgent = userAgent
	}
	if ipAddress != "" {
		e.IPAddress = ipAddress
	}
}

// SetBatchID sets the batch ID for bulk operations
func (e *NumberSuppressedEvent) SetBatchID(batchID string) {
	e.BatchID = &batchID
}

// SetProcessingTime sets the processing time for performance tracking
func (e *NumberSuppressedEvent) SetProcessingTime(duration time.Duration) {
	e.ProcessingTime = duration
}

// IsTemporary returns true if the suppression has an expiration date
func (e *NumberSuppressedEvent) IsTemporary() bool {
	return e.ExpiresAt != nil
}

// IsPermanent returns true if the suppression has no expiration date
func (e *NumberSuppressedEvent) IsPermanent() bool {
	return e.ExpiresAt == nil
}

// IsConsumerRequested returns true if the suppression was requested by the consumer
func (e *NumberSuppressedEvent) IsConsumerRequested() bool {
	return e.Reason == dnc.SuppressReasonConsumerRequest
}

// IsRegulatory returns true if the suppression is due to regulatory requirements
func (e *NumberSuppressedEvent) IsRegulatory() bool {
	return e.Reason == dnc.SuppressReasonRegulatory
}

// IsTCPARelevant returns true if this event is relevant for TCPA compliance
func (e *NumberSuppressedEvent) IsTCPARelevant() bool {
	return e.ComplianceFlags["tcpa_relevant"]
}

// IsGDPRRelevant returns true if this event is relevant for GDPR compliance
func (e *NumberSuppressedEvent) IsGDPRRelevant() bool {
	return e.ComplianceFlags["gdpr_relevant"]
}

// GetComplianceMetadata returns compliance-specific metadata
func (e *NumberSuppressedEvent) GetComplianceMetadata() map[string]interface{} {
	metadata := map[string]interface{}{
		"phone_number":     e.PhoneNumber.String(),
		"suppress_reason":  string(e.Reason),
		"source":           string(e.Source),
		"suppressed_at":    e.SuppressedAt,
		"suppressed_by":    e.SuppressedBy.String(),
		"is_temporary":     e.IsTemporary(),
		"is_consumer_req":  e.IsConsumerRequested(),
		"is_regulatory":    e.IsRegulatory(),
		"tcpa_relevant":    e.IsTCPARelevant(),
		"gdpr_relevant":    e.IsGDPRRelevant(),
		"legal_basis":      e.LegalBasis,
		"data_classes":     e.DataClasses,
	}
	
	if e.ExpiresAt != nil {
		metadata["expires_at"] = *e.ExpiresAt
	}
	
	if e.SourceRef != nil {
		metadata["source_reference"] = *e.SourceRef
	}
	
	if e.Notes != nil {
		metadata["notes"] = *e.Notes
	}
	
	if e.BatchID != nil {
		metadata["batch_id"] = *e.BatchID
	}
	
	return metadata
}

// ToAuditEvent converts the domain event to an audit event
func (e *NumberSuppressedEvent) ToAuditEvent() (*audit.Event, error) {
	auditEvent, err := audit.NewEvent(
		e.EventType,
		e.SuppressedBy.String(),
		e.PhoneNumber.String(),
		"suppress_number",
	)
	if err != nil {
		return nil, err
	}
	
	// Set additional audit event fields
	auditEvent.TargetType = "phone_number"
	auditEvent.Result = "success"
	auditEvent.RequestID = e.RequestID
	auditEvent.SessionID = e.SessionID
	auditEvent.ActorAgent = e.UserAgent
	auditEvent.ActorIP = e.IPAddress
	auditEvent.LegalBasis = e.LegalBasis
	auditEvent.DataClasses = e.DataClasses
	auditEvent.ComplianceFlags = e.ComplianceFlags
	
	// Add DNC-specific metadata
	auditEvent.Metadata = map[string]interface{}{
		"dnc_entry_id":     e.DNCEntryID.String(),
		"suppress_reason":  string(e.Reason),
		"source":           string(e.Source),
		"suppressed_at":    e.SuppressedAt,
		"is_temporary":     e.IsTemporary(),
		"processing_time":  e.ProcessingTime.String(),
	}
	
	if e.ExpiresAt != nil {
		auditEvent.Metadata["expires_at"] = *e.ExpiresAt
	}
	
	if e.SourceRef != nil {
		auditEvent.Metadata["source_reference"] = *e.SourceRef
	}
	
	if e.BatchID != nil {
		auditEvent.Metadata["batch_id"] = *e.BatchID
	}
	
	return auditEvent, nil
}

// setComplianceFlags sets compliance flags based on source and reason
func (e *NumberSuppressedEvent) setComplianceFlags() {
	e.ComplianceFlags["tcpa_relevant"] = true
	e.ComplianceFlags["contains_pii"] = true
	e.ComplianceFlags["dnc_operation"] = true
	
	// Set GDPR relevance for consumer requests
	if e.IsConsumerRequested() {
		e.ComplianceFlags["gdpr_relevant"] = true
		e.ComplianceFlags["data_subject_request"] = true
	}
	
	// Set regulatory flags
	if e.IsRegulatory() {
		e.ComplianceFlags["regulatory_compliance"] = true
	}
	
	// Set source-specific flags
	switch e.Source {
	case dnc.ListSourceFederal:
		e.ComplianceFlags["federal_dnc"] = true
	case dnc.ListSourceState:
		e.ComplianceFlags["state_dnc"] = true
	case dnc.ListSourceInternal:
		e.ComplianceFlags["internal_policy"] = true
	}
}

// Validate performs validation of the event
func (e *NumberSuppressedEvent) Validate() error {
	if e.EventID == uuid.Nil {
		return errors.NewValidationError("MISSING_EVENT_ID", "event ID is required")
	}
	
	if e.EventType != audit.EventDNCNumberSuppressed {
		return errors.NewValidationError("INVALID_EVENT_TYPE", "invalid event type")
	}
	
	if e.EventVersion == "" {
		return errors.NewValidationError("MISSING_EVENT_VERSION", "event version is required")
	}
	
	if e.PhoneNumber.String() == "" {
		return errors.NewValidationError("MISSING_PHONE_NUMBER", "phone number is required")
	}
	
	if e.SuppressedBy == uuid.Nil {
		return errors.NewValidationError("MISSING_SUPPRESSED_BY", "suppressed by user ID is required")
	}
	
	if e.DNCEntryID == uuid.Nil {
		return errors.NewValidationError("MISSING_DNC_ENTRY_ID", "DNC entry ID is required")
	}
	
	if e.RequestID == "" {
		return errors.NewValidationError("MISSING_REQUEST_ID", "request ID is required")
	}
	
	// Validate expiration if set
	if e.ExpiresAt != nil && e.ExpiresAt.Before(e.SuppressedAt) {
		return errors.NewValidationError("INVALID_EXPIRATION", 
			"expiration cannot be before suppression time")
	}
	
	return nil
}