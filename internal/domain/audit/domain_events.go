package audit

import (
	"time"

	"github.com/google/uuid"
)

// DomainEvent represents the base interface for all audit-triggering events
// These events are published by domain entities when audit-worthy actions occur
type DomainEvent interface {
	// Event metadata
	GetEventID() uuid.UUID
	GetEventType() EventType
	GetTimestamp() time.Time
	GetVersion() int

	// Actor information (who performed the action)
	GetActorID() string
	GetActorType() string

	// Target information (what was acted upon)
	GetTargetID() string
	GetTargetType() string

	// Action context
	GetAction() string
	GetResult() Result

	// Correlation for tracing
	GetRequestID() string
	GetSessionID() string
	GetCorrelationID() string

	// Compliance metadata
	GetComplianceFlags() map[string]bool
	GetDataClasses() []string
	GetLegalBasis() string

	// Additional context
	GetMetadata() map[string]interface{}

	// Convert to audit.Event for logging
	ToAuditEvent() (*Event, error)
}

// BaseDomainEvent provides common implementation for all domain events
// Following DCE patterns: composition over inheritance
type BaseDomainEvent struct {
	EventID       uuid.UUID              `json:"event_id"`
	EventType     EventType              `json:"event_type"`
	Timestamp     time.Time              `json:"timestamp"`
	Version       int                    `json:"version"`
	ActorID       string                 `json:"actor_id"`
	ActorType     string                 `json:"actor_type"`
	ActorIP       string                 `json:"actor_ip,omitempty"`
	ActorAgent    string                 `json:"actor_agent,omitempty"`
	TargetID      string                 `json:"target_id"`
	TargetType    string                 `json:"target_type"`
	TargetOwner   string                 `json:"target_owner,omitempty"`
	Action        string                 `json:"action"`
	Result        Result                 `json:"result"`
	ErrorCode     string                 `json:"error_code,omitempty"`
	ErrorMessage  string                 `json:"error_message,omitempty"`
	RequestID     string                 `json:"request_id"`
	SessionID     string                 `json:"session_id,omitempty"`
	CorrelationID string                 `json:"correlation_id,omitempty"`
	Environment   string                 `json:"environment"`
	Service       string                 `json:"service"`
	ComplianceFlags map[string]bool      `json:"compliance_flags,omitempty"`
	DataClasses   []string               `json:"data_classes,omitempty"`
	LegalBasis    string                 `json:"legal_basis,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// NewBaseDomainEvent creates a new base domain event with required fields
func NewBaseDomainEvent(eventType EventType, actorID, targetID, action string) *BaseDomainEvent {
	now := time.Now().UTC()
	
	return &BaseDomainEvent{
		EventID:         uuid.New(),
		EventType:       eventType,
		Timestamp:       now,
		Version:         1,
		ActorID:         actorID,
		TargetID:        targetID,
		Action:          action,
		Result:          ResultSuccess,
		RequestID:       getRequestIDFromContext(),
		SessionID:       getSessionIDFromContext(),
		CorrelationID:   getCorrelationIDFromContext(),
		Environment:     getEnvironment(),
		Service:         getServiceName(),
		ComplianceFlags: make(map[string]bool),
		DataClasses:     make([]string, 0),
		Metadata:        make(map[string]interface{}),
	}
}

// Implement DomainEvent interface
func (e *BaseDomainEvent) GetEventID() uuid.UUID       { return e.EventID }
func (e *BaseDomainEvent) GetEventType() EventType     { return e.EventType }
func (e *BaseDomainEvent) GetTimestamp() time.Time     { return e.Timestamp }
func (e *BaseDomainEvent) GetVersion() int             { return e.Version }
func (e *BaseDomainEvent) GetActorID() string          { return e.ActorID }
func (e *BaseDomainEvent) GetActorType() string        { return e.ActorType }
func (e *BaseDomainEvent) GetTargetID() string         { return e.TargetID }
func (e *BaseDomainEvent) GetTargetType() string       { return e.TargetType }
func (e *BaseDomainEvent) GetAction() string           { return e.Action }
func (e *BaseDomainEvent) GetResult() Result           { return e.Result }
func (e *BaseDomainEvent) GetRequestID() string        { return e.RequestID }
func (e *BaseDomainEvent) GetSessionID() string        { return e.SessionID }
func (e *BaseDomainEvent) GetCorrelationID() string    { return e.CorrelationID }
func (e *BaseDomainEvent) GetComplianceFlags() map[string]bool { return e.ComplianceFlags }
func (e *BaseDomainEvent) GetDataClasses() []string    { return e.DataClasses }
func (e *BaseDomainEvent) GetLegalBasis() string       { return e.LegalBasis }
func (e *BaseDomainEvent) GetMetadata() map[string]interface{} { return e.Metadata }

// SetFailure marks the event as failed with error details
func (e *BaseDomainEvent) SetFailure(errorCode, errorMessage string) {
	e.Result = ResultFailure
	e.ErrorCode = errorCode
	e.ErrorMessage = errorMessage
}

// SetPartial marks the event as partially successful
func (e *BaseDomainEvent) SetPartial(reason string) {
	e.Result = ResultPartial
	if e.Metadata == nil {
		e.Metadata = make(map[string]interface{})
	}
	e.Metadata["partial_reason"] = reason
}

// AddComplianceFlag sets a compliance flag
func (e *BaseDomainEvent) AddComplianceFlag(flag string, value bool) {
	if e.ComplianceFlags == nil {
		e.ComplianceFlags = make(map[string]bool)
	}
	e.ComplianceFlags[flag] = value
}

// AddDataClass adds a data classification
func (e *BaseDomainEvent) AddDataClass(dataClass string) {
	if e.DataClasses == nil {
		e.DataClasses = make([]string, 0)
	}
	
	// Avoid duplicates
	for _, existing := range e.DataClasses {
		if existing == dataClass {
			return
		}
	}
	
	e.DataClasses = append(e.DataClasses, dataClass)
}

// SetMetadata adds metadata key-value pairs
func (e *BaseDomainEvent) SetMetadata(key string, value interface{}) {
	if e.Metadata == nil {
		e.Metadata = make(map[string]interface{})
	}
	e.Metadata[key] = value
}

// ToAuditEvent converts the domain event to an audit.Event for logging
func (e *BaseDomainEvent) ToAuditEvent() (*Event, error) {
	auditEvent, err := NewEvent(e.EventType, e.ActorID, e.TargetID, e.Action)
	if err != nil {
		return nil, err
	}

	// Copy all fields from domain event to audit event
	auditEvent.ActorType = e.ActorType
	auditEvent.ActorIP = e.ActorIP
	auditEvent.ActorAgent = e.ActorAgent
	auditEvent.TargetType = e.TargetType
	auditEvent.TargetOwner = e.TargetOwner
	auditEvent.Result = string(e.Result)
	auditEvent.ErrorCode = e.ErrorCode
	auditEvent.ErrorMessage = e.ErrorMessage
	auditEvent.RequestID = e.RequestID
	auditEvent.SessionID = e.SessionID
	auditEvent.CorrelationID = e.CorrelationID
	auditEvent.Environment = e.Environment
	auditEvent.Service = e.Service
	auditEvent.LegalBasis = e.LegalBasis

	// Copy compliance flags
	if e.ComplianceFlags != nil {
		auditEvent.ComplianceFlags = make(map[string]bool)
		for k, v := range e.ComplianceFlags {
			auditEvent.ComplianceFlags[k] = v
		}
	}

	// Copy data classes
	if e.DataClasses != nil {
		auditEvent.DataClasses = make([]string, len(e.DataClasses))
		copy(auditEvent.DataClasses, e.DataClasses)
	}

	// Copy metadata
	if e.Metadata != nil {
		auditEvent.Metadata = make(map[string]interface{})
		for k, v := range e.Metadata {
			auditEvent.Metadata[k] = v
		}
	}

	// Set severity based on result
	switch e.Result {
	case ResultFailure:
		auditEvent.Severity = SeverityError
	case ResultPartial:
		auditEvent.Severity = SeverityWarning
	default:
		auditEvent.Severity = e.EventType.GetDefaultSeverity()
	}

	// Set retention based on event type and compliance requirements
	auditEvent.RetentionDays = e.EventType.GetRetentionDays()

	return auditEvent, nil
}

// Compliance helper methods for marking event compliance requirements

// MarkGDPRRelevant marks the event as containing GDPR-relevant data
func (e *BaseDomainEvent) MarkGDPRRelevant(legalBasis string) {
	e.AddComplianceFlag("gdpr_relevant", true)
	e.LegalBasis = legalBasis
	e.AddDataClass("personal_data")
}

// MarkTCPARelevant marks the event as TCPA-relevant for telephony compliance
func (e *BaseDomainEvent) MarkTCPARelevant() {
	e.AddComplianceFlag("tcpa_relevant", true)
	e.AddDataClass("phone_number")
}

// MarkContainsPII marks the event as containing personally identifiable information
func (e *BaseDomainEvent) MarkContainsPII() {
	e.AddComplianceFlag("contains_pii", true)
	e.AddDataClass("personal_data")
}

// MarkFinancialData marks the event as containing financial information
func (e *BaseDomainEvent) MarkFinancialData() {
	e.AddComplianceFlag("financial_data", true)
	e.AddDataClass("financial_data")
}

// MarkSecuritySensitive marks the event as security-sensitive
func (e *BaseDomainEvent) MarkSecuritySensitive() {
	e.AddComplianceFlag("security_sensitive", true)
}

// MarkRequiresSignature marks the event as requiring cryptographic signing
func (e *BaseDomainEvent) MarkRequiresSignature() {
	e.AddComplianceFlag("requires_signature", true)
}

// Context helper functions (would be implemented to extract from request context)

func getRequestIDFromContext() string {
	// TODO: Extract from context.Context
	return uuid.New().String()
}

func getSessionIDFromContext() string {
	// TODO: Extract from context.Context
	return ""
}

func getCorrelationIDFromContext() string {
	// TODO: Extract from context.Context
	return ""
}