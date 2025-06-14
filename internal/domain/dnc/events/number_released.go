package events

import (
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/audit"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/google/uuid"
)

// ReleaseReason represents why a number was released from the DNC list
type ReleaseReason string

const (
	ReleaseReasonExpired         ReleaseReason = "expired"
	ReleaseReasonConsumerRequest ReleaseReason = "consumer_request"
	ReleaseReasonDataCorrection  ReleaseReason = "data_correction"
	ReleaseReasonAdminOverride   ReleaseReason = "admin_override"
	ReleaseReasonSystemCleanup   ReleaseReason = "system_cleanup"
	ReleaseReasonRegulatory      ReleaseReason = "regulatory_change"
	ReleaseReasonTesting         ReleaseReason = "testing"
)

// NumberReleasedEvent represents when a phone number is removed from the DNC list
type NumberReleasedEvent struct {
	// Base event information
	EventID       uuid.UUID                    `json:"event_id"`
	EventType     audit.EventType              `json:"event_type"`
	EventVersion  string                       `json:"event_version"`
	Timestamp     time.Time                    `json:"timestamp"`
	
	// DNC specific fields
	PhoneNumber   values.PhoneNumber           `json:"phone_number"`
	ReleasedAt    time.Time                    `json:"released_at"`
	ReleasedBy    uuid.UUID                    `json:"released_by"`
	Reason        ReleaseReason                `json:"reason"`
	
	// Previous suppression context
	PreviousDNCEntryID uuid.UUID               `json:"previous_dnc_entry_id"`
	PreviousReason     string                  `json:"previous_reason,omitempty"`
	SuppressedAt       *time.Time              `json:"suppressed_at,omitempty"`
	SuppressedBy       *uuid.UUID              `json:"suppressed_by,omitempty"`
	
	// Additional context
	Notes         *string                      `json:"notes,omitempty"`
	AdminNotes    *string                      `json:"admin_notes,omitempty"`
	
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
	
	// Verification metadata
	VerifiedBy     *uuid.UUID                  `json:"verified_by,omitempty"`
	VerifiedAt     *time.Time                  `json:"verified_at,omitempty"`
	ApprovalCode   *string                     `json:"approval_code,omitempty"`
}

// NewNumberReleasedEvent creates a new number released event
func NewNumberReleasedEvent(
	phoneNumber string,
	reason ReleaseReason,
	releasedBy uuid.UUID,
	previousDNCEntryID uuid.UUID,
) (*NumberReleasedEvent, error) {
	// Validate phone number
	phone, err := values.NewPhoneNumber(phoneNumber)
	if err != nil {
		return nil, errors.NewValidationError("INVALID_PHONE_NUMBER", 
			"phone number must be valid E.164 format").WithCause(err)
	}
	
	// Validate releasedBy user ID
	if releasedBy == uuid.Nil {
		return nil, errors.NewValidationError("INVALID_USER_ID", 
			"released by user ID cannot be empty")
	}
	
	// Validate previous DNC entry ID
	if previousDNCEntryID == uuid.Nil {
		return nil, errors.NewValidationError("INVALID_DNC_ENTRY_ID", 
			"previous DNC entry ID cannot be empty")
	}
	
	// Validate release reason
	if err := validateReleaseReason(reason); err != nil {
		return nil, err
	}
	
	now := time.Now().UTC()
	
	event := &NumberReleasedEvent{
		EventID:            uuid.New(),
		EventType:          audit.EventDNCNumberReleased,
		EventVersion:       "1.0",
		Timestamp:          now,
		PhoneNumber:        phone,
		ReleasedAt:         now,
		ReleasedBy:         releasedBy,
		Reason:             reason,
		PreviousDNCEntryID: previousDNCEntryID,
		ComplianceFlags:    make(map[string]bool),
		DataClasses:        []string{"phone_number", "dnc_status"},
		LegalBasis:         "legitimate_interest",
	}
	
	// Set compliance flags based on reason
	event.setComplianceFlags()
	
	return event, nil
}

// GetEventType returns the event type
func (e *NumberReleasedEvent) GetEventType() audit.EventType {
	return audit.EventDNCNumberReleased
}

// GetEventVersion returns the event version
func (e *NumberReleasedEvent) GetEventVersion() string {
	return e.EventVersion
}

// GetEventID returns the event ID
func (e *NumberReleasedEvent) GetEventID() uuid.UUID {
	return e.EventID
}

// GetTimestamp returns the event timestamp
func (e *NumberReleasedEvent) GetTimestamp() time.Time {
	return e.Timestamp
}

// GetAggregateID returns the phone number as the aggregate ID
func (e *NumberReleasedEvent) GetAggregateID() string {
	return e.PhoneNumber.String()
}

// GetAggregateType returns the aggregate type
func (e *NumberReleasedEvent) GetAggregateType() string {
	return "phone_number"
}

// SetPreviousSuppressionContext sets context about the previous suppression
func (e *NumberReleasedEvent) SetPreviousSuppressionContext(reason string, suppressedAt time.Time, suppressedBy uuid.UUID) {
	e.PreviousReason = reason
	e.SuppressedAt = &suppressedAt
	e.SuppressedBy = &suppressedBy
}

// SetNotes sets additional notes for the release
func (e *NumberReleasedEvent) SetNotes(notes string) {
	e.Notes = &notes
}

// SetAdminNotes sets administrative notes for the release
func (e *NumberReleasedEvent) SetAdminNotes(notes string) {
	e.AdminNotes = &notes
}

// SetRequestContext sets the request context information
func (e *NumberReleasedEvent) SetRequestContext(requestID, sessionID, userAgent, ipAddress string) {
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
func (e *NumberReleasedEvent) SetBatchID(batchID string) {
	e.BatchID = &batchID
}

// SetProcessingTime sets the processing time for performance tracking
func (e *NumberReleasedEvent) SetProcessingTime(duration time.Duration) {
	e.ProcessingTime = duration
}

// SetVerification sets verification details for the release
func (e *NumberReleasedEvent) SetVerification(verifiedBy uuid.UUID, approvalCode string) {
	now := time.Now().UTC()
	e.VerifiedBy = &verifiedBy
	e.VerifiedAt = &now
	if approvalCode != "" {
		e.ApprovalCode = &approvalCode
	}
}

// IsExpiredRelease returns true if the release was due to expiration
func (e *NumberReleasedEvent) IsExpiredRelease() bool {
	return e.Reason == ReleaseReasonExpired
}

// IsConsumerRequested returns true if the release was requested by the consumer
func (e *NumberReleasedEvent) IsConsumerRequested() bool {
	return e.Reason == ReleaseReasonConsumerRequest
}

// IsAdminOverride returns true if the release was an administrative override
func (e *NumberReleasedEvent) IsAdminOverride() bool {
	return e.Reason == ReleaseReasonAdminOverride
}

// IsDataCorrection returns true if the release was due to data correction
func (e *NumberReleasedEvent) IsDataCorrection() bool {
	return e.Reason == ReleaseReasonDataCorrection
}

// IsSystemCleanup returns true if the release was due to system cleanup
func (e *NumberReleasedEvent) IsSystemCleanup() bool {
	return e.Reason == ReleaseReasonSystemCleanup
}

// IsVerified returns true if the release has been verified by an admin
func (e *NumberReleasedEvent) IsVerified() bool {
	return e.VerifiedBy != nil && e.VerifiedAt != nil
}

// IsTCPARelevant returns true if this event is relevant for TCPA compliance
func (e *NumberReleasedEvent) IsTCPARelevant() bool {
	return e.ComplianceFlags["tcpa_relevant"]
}

// IsGDPRRelevant returns true if this event is relevant for GDPR compliance
func (e *NumberReleasedEvent) IsGDPRRelevant() bool {
	return e.ComplianceFlags["gdpr_relevant"]
}

// RequiresApproval returns true if this type of release requires approval
func (e *NumberReleasedEvent) RequiresApproval() bool {
	return e.IsAdminOverride() || e.IsDataCorrection()
}

// GetSuppressionDuration returns the duration the number was suppressed
func (e *NumberReleasedEvent) GetSuppressionDuration() *time.Duration {
	if e.SuppressedAt == nil {
		return nil
	}
	
	duration := e.ReleasedAt.Sub(*e.SuppressedAt)
	return &duration
}

// GetComplianceMetadata returns compliance-specific metadata
func (e *NumberReleasedEvent) GetComplianceMetadata() map[string]interface{} {
	metadata := map[string]interface{}{
		"phone_number":      e.PhoneNumber.String(),
		"release_reason":    string(e.Reason),
		"released_at":       e.ReleasedAt,
		"released_by":       e.ReleasedBy.String(),
		"is_expired":        e.IsExpiredRelease(),
		"is_consumer_req":   e.IsConsumerRequested(),
		"is_admin_override": e.IsAdminOverride(),
		"is_data_correction": e.IsDataCorrection(),
		"is_verified":       e.IsVerified(),
		"requires_approval": e.RequiresApproval(),
		"tcpa_relevant":     e.IsTCPARelevant(),
		"gdpr_relevant":     e.IsGDPRRelevant(),
		"legal_basis":       e.LegalBasis,
		"data_classes":      e.DataClasses,
	}
	
	if e.SuppressedAt != nil {
		metadata["suppressed_at"] = *e.SuppressedAt
		if duration := e.GetSuppressionDuration(); duration != nil {
			metadata["suppression_duration"] = duration.String()
		}
	}
	
	if e.SuppressedBy != nil {
		metadata["suppressed_by"] = e.SuppressedBy.String()
	}
	
	if e.PreviousReason != "" {
		metadata["previous_reason"] = e.PreviousReason
	}
	
	if e.Notes != nil {
		metadata["notes"] = *e.Notes
	}
	
	if e.AdminNotes != nil {
		metadata["admin_notes"] = *e.AdminNotes
	}
	
	if e.BatchID != nil {
		metadata["batch_id"] = *e.BatchID
	}
	
	if e.VerifiedBy != nil {
		metadata["verified_by"] = e.VerifiedBy.String()
	}
	
	if e.VerifiedAt != nil {
		metadata["verified_at"] = *e.VerifiedAt
	}
	
	if e.ApprovalCode != nil {
		metadata["approval_code"] = *e.ApprovalCode
	}
	
	return metadata
}

// ToAuditEvent converts the domain event to an audit event
func (e *NumberReleasedEvent) ToAuditEvent() (*audit.Event, error) {
	auditEvent, err := audit.NewEvent(
		e.EventType,
		e.ReleasedBy.String(),
		e.PhoneNumber.String(),
		"release_number",
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
		"previous_dnc_entry_id": e.PreviousDNCEntryID.String(),
		"release_reason":        string(e.Reason),
		"released_at":           e.ReleasedAt,
		"is_expired":            e.IsExpiredRelease(),
		"is_consumer_req":       e.IsConsumerRequested(),
		"is_verified":           e.IsVerified(),
		"processing_time":       e.ProcessingTime.String(),
	}
	
	if e.SuppressedAt != nil {
		auditEvent.Metadata["suppressed_at"] = *e.SuppressedAt
	}
	
	if e.SuppressedBy != nil {
		auditEvent.Metadata["suppressed_by"] = e.SuppressedBy.String()
	}
	
	if e.PreviousReason != "" {
		auditEvent.Metadata["previous_reason"] = e.PreviousReason
	}
	
	if e.BatchID != nil {
		auditEvent.Metadata["batch_id"] = *e.BatchID
	}
	
	if e.VerifiedBy != nil {
		auditEvent.Metadata["verified_by"] = e.VerifiedBy.String()
	}
	
	return auditEvent, nil
}

// setComplianceFlags sets compliance flags based on reason
func (e *NumberReleasedEvent) setComplianceFlags() {
	e.ComplianceFlags["tcpa_relevant"] = true
	e.ComplianceFlags["contains_pii"] = true
	e.ComplianceFlags["dnc_operation"] = true
	
	// Set GDPR relevance for consumer requests
	if e.IsConsumerRequested() {
		e.ComplianceFlags["gdpr_relevant"] = true
		e.ComplianceFlags["data_subject_request"] = true
	}
	
	// Set admin override flags
	if e.IsAdminOverride() {
		e.ComplianceFlags["admin_override"] = true
		e.ComplianceFlags["requires_approval"] = true
	}
	
	// Set data correction flags
	if e.IsDataCorrection() {
		e.ComplianceFlags["data_correction"] = true
		e.ComplianceFlags["requires_verification"] = true
	}
	
	// Set system flags
	if e.IsSystemCleanup() {
		e.ComplianceFlags["system_operation"] = true
		e.ComplianceFlags["automated"] = true
	}
}

// validateReleaseReason validates the release reason
func validateReleaseReason(reason ReleaseReason) error {
	switch reason {
	case ReleaseReasonExpired, ReleaseReasonConsumerRequest, ReleaseReasonDataCorrection,
		ReleaseReasonAdminOverride, ReleaseReasonSystemCleanup, ReleaseReasonRegulatory,
		ReleaseReasonTesting:
		return nil
	default:
		return errors.NewValidationError("INVALID_RELEASE_REASON", 
			"invalid release reason: "+string(reason))
	}
}

// Validate performs validation of the event
func (e *NumberReleasedEvent) Validate() error {
	if e.EventID == uuid.Nil {
		return errors.NewValidationError("MISSING_EVENT_ID", "event ID is required")
	}
	
	if e.EventType != audit.EventDNCNumberReleased {
		return errors.NewValidationError("INVALID_EVENT_TYPE", "invalid event type")
	}
	
	if e.EventVersion == "" {
		return errors.NewValidationError("MISSING_EVENT_VERSION", "event version is required")
	}
	
	if e.PhoneNumber.String() == "" {
		return errors.NewValidationError("MISSING_PHONE_NUMBER", "phone number is required")
	}
	
	if e.ReleasedBy == uuid.Nil {
		return errors.NewValidationError("MISSING_RELEASED_BY", "released by user ID is required")
	}
	
	if e.PreviousDNCEntryID == uuid.Nil {
		return errors.NewValidationError("MISSING_PREVIOUS_DNC_ENTRY_ID", "previous DNC entry ID is required")
	}
	
	if e.RequestID == "" {
		return errors.NewValidationError("MISSING_REQUEST_ID", "request ID is required")
	}
	
	// Validate release reason
	if err := validateReleaseReason(e.Reason); err != nil {
		return err
	}
	
	// Additional validation for admin overrides
	if e.RequiresApproval() && !e.IsVerified() {
		return errors.NewValidationError("MISSING_VERIFICATION", 
			"admin override and data correction releases require verification")
	}
	
	return nil
}