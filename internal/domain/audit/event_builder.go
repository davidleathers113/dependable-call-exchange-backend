package audit

import (
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
	"github.com/google/uuid"
)

// EventBuilder provides a fluent interface for creating audit events
// Following DCE patterns: builder pattern for complex object construction
type EventBuilder struct {
	event *Event
	err   error
}

// NewEventBuilder creates a new event builder
// This is the primary entry point for creating audit events
func NewEventBuilder(eventType EventType) *EventBuilder {
	now := time.Now().UTC()
	
	event := &Event{
		ID:            uuid.New(),
		Timestamp:     now,
		TimestampNano: now.UnixNano(),
		Type:          eventType,
		Severity:      eventType.GetDefaultSeverity(),
		Category:      deriveCategory(eventType),
		Result:        string(ResultSuccess), // Default to success
		Environment:   getEnvironment(),
		Service:       getServiceName(),
		Version:       getServiceVersion(),
		RetentionDays: eventType.GetRetentionDays(),
		Metadata:      make(map[string]interface{}),
		ComplianceFlags: make(map[string]bool),
		Tags:          make([]string, 0),
		immutable:     false,
	}

	return &EventBuilder{
		event: event,
		err:   nil,
	}
}

// WithActor sets the actor information
func (b *EventBuilder) WithActor(id, actorType string) *EventBuilder {
	if b.err != nil {
		return b
	}

	if id == "" {
		b.err = errors.NewValidationError("MISSING_ACTOR_ID", "actor ID cannot be empty")
		return b
	}

	b.event.ActorID = id
	b.event.ActorType = actorType
	return b
}

// WithActorDetails sets additional actor information
func (b *EventBuilder) WithActorDetails(ip, userAgent string) *EventBuilder {
	if b.err != nil {
		return b
	}

	b.event.ActorIP = ip
	b.event.ActorAgent = userAgent
	return b
}

// WithTarget sets the target information
func (b *EventBuilder) WithTarget(id, targetType string) *EventBuilder {
	if b.err != nil {
		return b
	}

	if id == "" {
		b.err = errors.NewValidationError("MISSING_TARGET_ID", "target ID cannot be empty")
		return b
	}

	b.event.TargetID = id
	b.event.TargetType = targetType
	return b
}

// WithTargetOwner sets the target owner
func (b *EventBuilder) WithTargetOwner(owner string) *EventBuilder {
	if b.err != nil {
		return b
	}

	b.event.TargetOwner = owner
	return b
}

// WithAction sets the action being performed
func (b *EventBuilder) WithAction(action string) *EventBuilder {
	if b.err != nil {
		return b
	}

	if action == "" {
		b.err = errors.NewValidationError("MISSING_ACTION", "action cannot be empty")
		return b
	}

	b.event.Action = action
	return b
}

// WithResult sets the action result
func (b *EventBuilder) WithResult(result Result) *EventBuilder {
	if b.err != nil {
		return b
	}

	b.event.Result = string(result)
	
	// Adjust severity based on result if not explicitly set
	if result.IsFailure() && b.event.Severity == SeverityInfo {
		b.event.Severity = SeverityError
	} else if result.IsPartial() && b.event.Severity == SeverityInfo {
		b.event.Severity = SeverityWarning
	}

	return b
}

// WithError sets error information and marks result as failure
func (b *EventBuilder) WithError(code, message string) *EventBuilder {
	if b.err != nil {
		return b
	}

	b.event.ErrorCode = code
	b.event.ErrorMessage = message
	b.event.Result = string(ResultFailure)
	
	// Escalate severity for errors
	if b.event.Severity == SeverityInfo {
		b.event.Severity = SeverityError
	}

	return b
}

// WithSeverity sets the event severity
func (b *EventBuilder) WithSeverity(severity Severity) *EventBuilder {
	if b.err != nil {
		return b
	}

	if err := validateSeverity(severity); err != nil {
		b.err = errors.NewValidationError("INVALID_SEVERITY", "invalid severity level").WithCause(err)
		return b
	}

	b.event.Severity = severity
	return b
}

// WithRequestContext sets request correlation information
func (b *EventBuilder) WithRequestContext(requestID, sessionID, correlationID string) *EventBuilder {
	if b.err != nil {
		return b
	}

	b.event.RequestID = requestID
	b.event.SessionID = sessionID
	b.event.CorrelationID = correlationID
	return b
}

// WithComplianceFlag sets a compliance flag
func (b *EventBuilder) WithComplianceFlag(flag string, value bool) *EventBuilder {
	if b.err != nil {
		return b
	}

	if b.event.ComplianceFlags == nil {
		b.event.ComplianceFlags = make(map[string]bool)
	}

	b.event.ComplianceFlags[flag] = value
	return b
}

// WithComplianceFlags sets multiple compliance flags
func (b *EventBuilder) WithComplianceFlags(flags map[string]bool) *EventBuilder {
	if b.err != nil {
		return b
	}

	if b.event.ComplianceFlags == nil {
		b.event.ComplianceFlags = make(map[string]bool)
	}

	for flag, value := range flags {
		b.event.ComplianceFlags[flag] = value
	}

	return b
}

// WithDataClasses sets the data classification tags
func (b *EventBuilder) WithDataClasses(dataClasses []string) *EventBuilder {
	if b.err != nil {
		return b
	}

	// Validate data classes
	for _, dataClass := range dataClasses {
		if dataClass == "" {
			b.err = errors.NewValidationError("INVALID_DATA_CLASS", "data class cannot be empty")
			return b
		}
	}

	b.event.DataClasses = make([]string, len(dataClasses))
	copy(b.event.DataClasses, dataClasses)
	return b
}

// WithLegalBasis sets the legal basis for processing (GDPR)
func (b *EventBuilder) WithLegalBasis(legalBasis string) *EventBuilder {
	if b.err != nil {
		return b
	}

	validLegalBases := []string{
		"consent", "contract", "legal_obligation", "vital_interests", 
		"public_task", "legitimate_interests",
	}

	if legalBasis != "" {
		isValid := false
		for _, valid := range validLegalBases {
			if legalBasis == valid {
				isValid = true
				break
			}
		}

		if !isValid {
			b.err = errors.NewValidationError("INVALID_LEGAL_BASIS", 
				"legal basis must be a valid GDPR value")
			return b
		}
	}

	b.event.LegalBasis = legalBasis
	return b
}

// WithRetentionDays sets the retention period for this event
func (b *EventBuilder) WithRetentionDays(days int) *EventBuilder {
	if b.err != nil {
		return b
	}

	if days <= 0 {
		b.err = errors.NewValidationError("INVALID_RETENTION", 
			"retention days must be positive")
		return b
	}

	b.event.RetentionDays = days
	return b
}

// WithMetadata adds metadata to the event
func (b *EventBuilder) WithMetadata(key string, value interface{}) *EventBuilder {
	if b.err != nil {
		return b
	}

	if key == "" {
		b.err = errors.NewValidationError("EMPTY_METADATA_KEY", "metadata key cannot be empty")
		return b
	}

	if b.event.Metadata == nil {
		b.event.Metadata = make(map[string]interface{})
	}

	b.event.Metadata[key] = value
	return b
}

// WithMetadataMap adds multiple metadata entries
func (b *EventBuilder) WithMetadataMap(metadata map[string]interface{}) *EventBuilder {
	if b.err != nil {
		return b
	}

	if b.event.Metadata == nil {
		b.event.Metadata = make(map[string]interface{})
	}

	for key, value := range metadata {
		if key == "" {
			b.err = errors.NewValidationError("EMPTY_METADATA_KEY", "metadata key cannot be empty")
			return b
		}
		b.event.Metadata[key] = value
	}

	return b
}

// WithTag adds a tag to the event
func (b *EventBuilder) WithTag(tag string) *EventBuilder {
	if b.err != nil {
		return b
	}

	if tag == "" {
		b.err = errors.NewValidationError("EMPTY_TAG", "tag cannot be empty")
		return b
	}

	// Check for duplicates
	for _, existingTag := range b.event.Tags {
		if existingTag == tag {
			return b // Already exists, no need to add
		}
	}

	b.event.Tags = append(b.event.Tags, tag)
	return b
}

// WithTags adds multiple tags to the event
func (b *EventBuilder) WithTags(tags []string) *EventBuilder {
	if b.err != nil {
		return b
	}

	for _, tag := range tags {
		b.WithTag(tag)
		if b.err != nil {
			return b
		}
	}

	return b
}

// WithSignature sets the cryptographic signature
func (b *EventBuilder) WithSignature(signature string) *EventBuilder {
	if b.err != nil {
		return b
	}

	b.event.Signature = signature
	return b
}

// WithSequenceNumber sets the sequence number (typically set by the logger)
func (b *EventBuilder) WithSequenceNumber(seqNum int64) *EventBuilder {
	if b.err != nil {
		return b
	}

	if seqNum <= 0 {
		b.err = errors.NewValidationError("INVALID_SEQUENCE", 
			"sequence number must be positive")
		return b
	}

	b.event.SequenceNum = seqNum
	return b
}

// WithTimestamp overrides the default timestamp (use with caution)
func (b *EventBuilder) WithTimestamp(timestamp time.Time) *EventBuilder {
	if b.err != nil {
		return b
	}

	if timestamp.IsZero() {
		b.err = errors.NewValidationError("INVALID_TIMESTAMP", 
			"timestamp cannot be zero")
		return b
	}

	b.event.Timestamp = timestamp.UTC()
	b.event.TimestampNano = timestamp.UnixNano()
	return b
}

// Build finalizes the event construction and returns the event
func (b *EventBuilder) Build() (*Event, error) {
	if b.err != nil {
		return nil, b.err
	}

	// Validate the constructed event
	if err := b.event.Validate(); err != nil {
		return nil, err
	}

	return b.event, nil
}

// MustBuild finalizes the event construction and panics on error
// Use only when you're certain the build will succeed
func (b *EventBuilder) MustBuild() *Event {
	event, err := b.Build()
	if err != nil {
		panic(err)
	}
	return event
}

// Convenience methods for common event patterns

// ForConsentGranted creates a builder for consent granted events
func ForConsentGranted(userID, phoneNumber string) *EventBuilder {
	return NewEventBuilder(EventConsentGranted).
		WithActor(userID, "user").
		WithTarget(phoneNumber, "phone_number").
		WithAction("grant_consent").
		WithComplianceFlag("tcpa_compliant", true).
		WithDataClasses([]string{"phone_number"}).
		WithLegalBasis("consent")
}

// ForConsentRevoked creates a builder for consent revoked events
func ForConsentRevoked(userID, phoneNumber string) *EventBuilder {
	return NewEventBuilder(EventConsentRevoked).
		WithActor(userID, "user").
		WithTarget(phoneNumber, "phone_number").
		WithAction("revoke_consent").
		WithComplianceFlag("tcpa_compliant", true).
		WithDataClasses([]string{"phone_number"}).
		WithLegalBasis("consent")
}

// ForDataAccess creates a builder for data access events
func ForDataAccess(actorID, targetID, dataType string) *EventBuilder {
	return NewEventBuilder(EventDataAccessed).
		WithActor(actorID, "user").
		WithTarget(targetID, dataType).
		WithAction("access_data").
		WithComplianceFlag("gdpr_relevant", true).
		WithLegalBasis("legitimate_interests")
}

// ForCallInitiated creates a builder for call initiation events
func ForCallInitiated(actorID, callID string) *EventBuilder {
	return NewEventBuilder(EventCallInitiated).
		WithActor(actorID, "user").
		WithTarget(callID, "call").
		WithAction("initiate_call").
		WithComplianceFlag("tcpa_relevant", true)
}

// ForBidPlaced creates a builder for bid placement events
func ForBidPlaced(buyerID, bidID string) *EventBuilder {
	return NewEventBuilder(EventBidPlaced).
		WithActor(buyerID, "buyer").
		WithTarget(bidID, "bid").
		WithAction("place_bid")
}

// ForAuthFailure creates a builder for authentication failure events
func ForAuthFailure(userID, reason string) *EventBuilder {
	return NewEventBuilder(EventAuthFailure).
		WithActor(userID, "user").
		WithTarget("authentication_system", "system").
		WithAction("authenticate").
		WithResult(ResultFailure).
		WithError("AUTH_FAILED", reason).
		WithSeverity(SeverityWarning)
}

// ForAPICall creates a builder for API call events
func ForAPICall(userID, endpoint, method string) *EventBuilder {
	return NewEventBuilder(EventAPICall).
		WithActor(userID, "user").
		WithTarget(endpoint, "api_endpoint").
		WithAction(method).
		WithMetadata("endpoint", endpoint).
		WithMetadata("method", method)
}