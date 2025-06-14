package audit

import (
	"time"

	"github.com/google/uuid"
)

// Compliance Domain Events
// These events are published when compliance-related actions occur across all domains

// ConsentGrantedEvent is published when a user grants consent for data processing
type ConsentGrantedEvent struct {
	*BaseDomainEvent
	ConsentID        uuid.UUID `json:"consent_id"`
	DataSubjectID    uuid.UUID `json:"data_subject_id"`
	ConsentType      string    `json:"consent_type"`
	Purpose          string    `json:"purpose"`
	LegalBasisType   string    `json:"legal_basis_type"`
	ConsentMethod    string    `json:"consent_method"`
	ConsentText      string    `json:"consent_text,omitempty"`
	ConsentLanguage  string    `json:"consent_language"`
	ConsentVersion   string    `json:"consent_version"`
	ExpiresAt        *time.Time `json:"expires_at,omitempty"`
	IPAddress        string    `json:"ip_address,omitempty"`
	UserAgent        string    `json:"user_agent,omitempty"`
	GeolocationData  string    `json:"geolocation_data,omitempty"`
	ConsentScope     []string  `json:"consent_scope"`
	OptInMethod      string    `json:"opt_in_method"`
	WitnessID        *uuid.UUID `json:"witness_id,omitempty"`
}

// NewConsentGrantedEvent creates a new consent granted event
func NewConsentGrantedEvent(actorID string, consentID, dataSubjectID uuid.UUID, consentType, purpose string) *ConsentGrantedEvent {
	base := NewBaseDomainEvent(EventConsentGranted, actorID, consentID.String(), "consent_granted")
	base.TargetType = "consent"
	base.ActorType = "user"

	event := &ConsentGrantedEvent{
		BaseDomainEvent: base,
		ConsentID:       consentID,
		DataSubjectID:   dataSubjectID,
		ConsentType:     consentType,
		Purpose:         purpose,
		LegalBasisType:  "consent",
		ConsentLanguage: "en",
		ConsentVersion:  "1.0",
		ConsentScope:    make([]string, 0),
		OptInMethod:     "explicit",
	}

	// Mark as GDPR relevant and requiring signature
	event.MarkGDPRRelevant("consent")
	event.MarkRequiresSignature()
	event.MarkContainsPII()

	// Add relevant data classes
	event.AddDataClass("consent_data")
	event.AddDataClass("personal_data")
	event.AddDataClass("legal_basis")

	// Set metadata for consent granting
	event.SetMetadata("action_type", "consent_grant")
	event.SetMetadata("data_subject_id", dataSubjectID.String())
	event.SetMetadata("consent_type", consentType)
	event.SetMetadata("purpose", purpose)

	return event
}

// ConsentRevokedEvent is published when a user revokes previously granted consent
type ConsentRevokedEvent struct {
	*BaseDomainEvent
	ConsentID         uuid.UUID `json:"consent_id"`
	DataSubjectID     uuid.UUID `json:"data_subject_id"`
	ConsentType       string    `json:"consent_type"`
	OriginalPurpose   string    `json:"original_purpose"`
	RevocationReason  string    `json:"revocation_reason"`
	RevocationMethod  string    `json:"revocation_method"`
	OriginalGrantDate time.Time `json:"original_grant_date"`
	RevokedAt         time.Time `json:"revoked_at"`
	IPAddress         string    `json:"ip_address,omitempty"`
	UserAgent         string    `json:"user_agent,omitempty"`
	DataRetention     string    `json:"data_retention_action"`
	NotificationsSent []string  `json:"notifications_sent"`
}

// NewConsentRevokedEvent creates a new consent revoked event
func NewConsentRevokedEvent(actorID string, consentID, dataSubjectID uuid.UUID, consentType, reason string) *ConsentRevokedEvent {
	base := NewBaseDomainEvent(EventConsentRevoked, actorID, consentID.String(), "consent_revoked")
	base.TargetType = "consent"
	base.ActorType = "user"

	event := &ConsentRevokedEvent{
		BaseDomainEvent:   base,
		ConsentID:         consentID,
		DataSubjectID:     dataSubjectID,
		ConsentType:       consentType,
		RevocationReason:  reason,
		RevocationMethod:  "user_request",
		RevokedAt:         time.Now().UTC(),
		DataRetention:     "immediate_deletion",
		NotificationsSent: make([]string, 0),
	}

	// Mark as GDPR relevant and requiring signature
	event.MarkGDPRRelevant("consent")
	event.MarkRequiresSignature()
	event.MarkContainsPII()

	// Add relevant data classes
	event.AddDataClass("consent_data")
	event.AddDataClass("personal_data")
	event.AddDataClass("revocation_data")

	// Set metadata for consent revocation
	event.SetMetadata("action_type", "consent_revocation")
	event.SetMetadata("data_subject_id", dataSubjectID.String())
	event.SetMetadata("consent_type", consentType)
	event.SetMetadata("revocation_reason", reason)

	return event
}

// ConsentUpdatedEvent is published when consent details are modified
type ConsentUpdatedEvent struct {
	*BaseDomainEvent
	ConsentID        uuid.UUID              `json:"consent_id"`
	DataSubjectID    uuid.UUID              `json:"data_subject_id"`
	ConsentType      string                 `json:"consent_type"`
	UpdatedFields    []string               `json:"updated_fields"`
	PreviousValues   map[string]interface{} `json:"previous_values"`
	NewValues        map[string]interface{} `json:"new_values"`
	UpdateReason     string                 `json:"update_reason"`
	UpdateMethod     string                 `json:"update_method"`
	UpdatedAt        time.Time              `json:"updated_at"`
	NewVersion       string                 `json:"new_version"`
	PreviousVersion  string                 `json:"previous_version"`
	RequiresReopt    bool                   `json:"requires_reopt"`
}

// NewConsentUpdatedEvent creates a new consent updated event
func NewConsentUpdatedEvent(actorID string, consentID, dataSubjectID uuid.UUID, consentType string, updatedFields []string) *ConsentUpdatedEvent {
	base := NewBaseDomainEvent(EventConsentUpdated, actorID, consentID.String(), "consent_updated")
	base.TargetType = "consent"
	base.ActorType = "user"

	event := &ConsentUpdatedEvent{
		BaseDomainEvent: base,
		ConsentID:       consentID,
		DataSubjectID:   dataSubjectID,
		ConsentType:     consentType,
		UpdatedFields:   updatedFields,
		PreviousValues:  make(map[string]interface{}),
		NewValues:       make(map[string]interface{}),
		UpdateMethod:    "user_request",
		UpdatedAt:       time.Now().UTC(),
		NewVersion:      "1.1",
		PreviousVersion: "1.0",
	}

	// Mark as GDPR relevant and requiring signature
	event.MarkGDPRRelevant("consent")
	event.MarkRequiresSignature()
	event.MarkContainsPII()

	// Add relevant data classes
	event.AddDataClass("consent_data")
	event.AddDataClass("personal_data")
	event.AddDataClass("version_data")

	// Set metadata for consent update
	event.SetMetadata("action_type", "consent_update")
	event.SetMetadata("data_subject_id", dataSubjectID.String())
	event.SetMetadata("consent_type", consentType)
	event.SetMetadata("updated_fields", updatedFields)

	return event
}

// OptOutRequestedEvent is published when a user requests to opt out of communications
type OptOutRequestedEvent struct {
	*BaseDomainEvent
	OptOutID          uuid.UUID `json:"opt_out_id"`
	DataSubjectID     uuid.UUID `json:"data_subject_id"`
	OptOutType        string    `json:"opt_out_type"`
	OptOutScope       []string  `json:"opt_out_scope"`
	RequestMethod     string    `json:"request_method"`
	RequestedAt       time.Time `json:"requested_at"`
	EffectiveAt       time.Time `json:"effective_at"`
	PhoneNumber       string    `json:"phone_number,omitempty"`
	EmailAddress      string    `json:"email_address,omitempty"`
	IPAddress         string    `json:"ip_address,omitempty"`
	UserAgent         string    `json:"user_agent,omitempty"`
	Reason            string    `json:"reason,omitempty"`
	ProcessingStatus  string    `json:"processing_status"`
	ConfirmationSent  bool      `json:"confirmation_sent"`
}

// NewOptOutRequestedEvent creates a new opt-out requested event
func NewOptOutRequestedEvent(actorID string, optOutID, dataSubjectID uuid.UUID, optOutType string, scope []string) *OptOutRequestedEvent {
	base := NewBaseDomainEvent(EventOptOutRequested, actorID, optOutID.String(), "opt_out_requested")
	base.TargetType = "opt_out"
	base.ActorType = "user"

	event := &OptOutRequestedEvent{
		BaseDomainEvent:  base,
		OptOutID:         optOutID,
		DataSubjectID:    dataSubjectID,
		OptOutType:       optOutType,
		OptOutScope:      scope,
		RequestMethod:    "web_form",
		RequestedAt:      time.Now().UTC(),
		EffectiveAt:      time.Now().UTC(),
		ProcessingStatus: "pending",
		ConfirmationSent: false,
	}

	// Mark as GDPR and TCPA relevant
	event.MarkGDPRRelevant("legitimate_interest")
	event.MarkTCPARelevant()
	event.MarkRequiresSignature()
	event.MarkContainsPII()

	// Add relevant data classes
	event.AddDataClass("opt_out_data")
	event.AddDataClass("personal_data")
	event.AddDataClass("communication_preferences")

	// Set metadata for opt-out request
	event.SetMetadata("action_type", "opt_out_request")
	event.SetMetadata("data_subject_id", dataSubjectID.String())
	event.SetMetadata("opt_out_type", optOutType)
	event.SetMetadata("opt_out_scope", scope)

	return event
}

// ComplianceViolationEvent is published when a compliance violation is detected
type ComplianceViolationEvent struct {
	*BaseDomainEvent
	ViolationID       uuid.UUID `json:"violation_id"`
	ViolationType     string    `json:"violation_type"`
	Regulation        string    `json:"regulation"`
	RuleID            string    `json:"rule_id"`
	Severity          string    `json:"severity_level"`
	Description       string    `json:"description"`
	AffectedDataTypes []string  `json:"affected_data_types"`
	DataSubjectID     *uuid.UUID `json:"data_subject_id,omitempty"`
	DetectionMethod   string    `json:"detection_method"`
	DetectedAt        time.Time `json:"detected_at"`
	ReportedAt        time.Time `json:"reported_at"`
	Status            string    `json:"status"`
	AssignedTo        string    `json:"assigned_to,omitempty"`
	RemediationPlan   string    `json:"remediation_plan,omitempty"`
	NotificationsSent []string  `json:"notifications_sent"`
	EscalationLevel   int       `json:"escalation_level"`
}

// NewComplianceViolationEvent creates a new compliance violation event
func NewComplianceViolationEvent(actorID string, violationID uuid.UUID, violationType, regulation string) *ComplianceViolationEvent {
	base := NewBaseDomainEvent(EventAnomalyDetected, actorID, violationID.String(), "compliance_violation_detected")
	base.TargetType = "compliance_violation"
	base.ActorType = "system"

	event := &ComplianceViolationEvent{
		BaseDomainEvent:   base,
		ViolationID:       violationID,
		ViolationType:     violationType,
		Regulation:        regulation,
		Severity:          "high",
		DetectionMethod:   "automated",
		DetectedAt:        time.Now().UTC(),
		ReportedAt:        time.Now().UTC(),
		Status:            "open",
		NotificationsSent: make([]string, 0),
		EscalationLevel:   1,
	}

	// Mark as security sensitive and requiring signature
	event.MarkSecuritySensitive()
	event.MarkRequiresSignature()

	// Add relevant data classes
	event.AddDataClass("compliance_data")
	event.AddDataClass("violation_data")
	event.AddDataClass("regulatory_data")

	// Set metadata for violation
	event.SetMetadata("action_type", "compliance_violation")
	event.SetMetadata("violation_type", violationType)
	event.SetMetadata("regulation", regulation)
	event.SetMetadata("severity", "high")

	return event
}

// TCPAComplianceCheckEvent is published when TCPA compliance is verified for a call
type TCPAComplianceCheckEvent struct {
	*BaseDomainEvent
	CheckID           uuid.UUID `json:"check_id"`
	CallID            uuid.UUID `json:"call_id"`
	PhoneNumber       string    `json:"phone_number"`
	CallingTime       time.Time `json:"calling_time"`
	CallerTimeZone    string    `json:"caller_timezone"`
	CalleeTimeZone    string    `json:"callee_timezone"`
	IsWithinHours     bool      `json:"is_within_hours"`
	HasConsent        bool      `json:"has_consent"`
	ConsentType       string    `json:"consent_type,omitempty"`
	ConsentDate       *time.Time `json:"consent_date,omitempty"`
	IsOnDNC           bool      `json:"is_on_dnc"`
	DNCCheckDate      time.Time `json:"dnc_check_date"`
	ComplianceResult  string    `json:"compliance_result"`
	RulesChecked      []string  `json:"rules_checked"`
	RuleResults       map[string]bool `json:"rule_results"`
	RecommendedAction string    `json:"recommended_action"`
}

// NewTCPAComplianceCheckEvent creates a new TCPA compliance check event
func NewTCPAComplianceCheckEvent(actorID string, checkID, callID uuid.UUID, phoneNumber string) *TCPAComplianceCheckEvent {
	base := NewBaseDomainEvent(EventAuthSuccess, actorID, checkID.String(), "tcpa_compliance_checked")
	base.TargetType = "compliance_check"
	base.ActorType = "system"

	event := &TCPAComplianceCheckEvent{
		BaseDomainEvent:   base,
		CheckID:           checkID,
		CallID:            callID,
		PhoneNumber:       phoneNumber,
		CallingTime:       time.Now().UTC(),
		DNCCheckDate:      time.Now().UTC(),
		ComplianceResult:  "compliant",
		RulesChecked:      []string{"calling_hours", "consent", "dnc"},
		RuleResults:       make(map[string]bool),
		RecommendedAction: "proceed",
	}

	// Mark as TCPA relevant
	event.MarkTCPARelevant()

	// Add relevant data classes
	event.AddDataClass("compliance_data")
	event.AddDataClass("phone_number")
	event.AddDataClass("call_data")
	event.AddDataClass("tcpa_data")

	// Set metadata for TCPA check
	event.SetMetadata("action_type", "tcpa_compliance_check")
	event.SetMetadata("call_id", callID.String())
	event.SetMetadata("phone_number", phoneNumber)

	return event
}

// GDPRDataRequestEvent is published when a GDPR data subject request is received
type GDPRDataRequestEvent struct {
	*BaseDomainEvent
	RequestID         uuid.UUID `json:"request_id"`
	DataSubjectID     uuid.UUID `json:"data_subject_id"`
	RequestType       string    `json:"request_type"`
	RequestScope      []string  `json:"request_scope"`
	RequestMethod     string    `json:"request_method"`
	RequestDate       time.Time `json:"request_date"`
	DueDate           time.Time `json:"due_date"`
	VerificationMethod string   `json:"verification_method"`
	VerificationStatus string   `json:"verification_status"`
	ProcessingStatus  string    `json:"processing_status"`
	AssignedTo        string    `json:"assigned_to,omitempty"`
	CompletedAt       *time.Time `json:"completed_at,omitempty"`
	DeliveryMethod    string    `json:"delivery_method,omitempty"`
	NotificationsSent []string  `json:"notifications_sent"`
}

// NewGDPRDataRequestEvent creates a new GDPR data request event
func NewGDPRDataRequestEvent(actorID string, requestID, dataSubjectID uuid.UUID, requestType string) *GDPRDataRequestEvent {
	base := NewBaseDomainEvent(EventDataAccessed, actorID, requestID.String(), "gdpr_request_received")
	base.TargetType = "gdpr_request"
	base.ActorType = "user"

	dueDate := time.Now().UTC().AddDate(0, 0, 30) // 30 days to respond

	event := &GDPRDataRequestEvent{
		BaseDomainEvent:    base,
		RequestID:          requestID,
		DataSubjectID:      dataSubjectID,
		RequestType:        requestType,
		RequestScope:       make([]string, 0),
		RequestMethod:      "web_form",
		RequestDate:        time.Now().UTC(),
		DueDate:            dueDate,
		VerificationMethod: "email",
		VerificationStatus: "pending",
		ProcessingStatus:   "received",
		NotificationsSent:  make([]string, 0),
	}

	// Mark as GDPR relevant and requiring signature
	event.MarkGDPRRelevant("legal_obligation")
	event.MarkRequiresSignature()
	event.MarkContainsPII()

	// Add relevant data classes
	event.AddDataClass("gdpr_request")
	event.AddDataClass("personal_data")
	event.AddDataClass("request_data")

	// Set metadata for GDPR request
	event.SetMetadata("action_type", "gdpr_data_request")
	event.SetMetadata("data_subject_id", dataSubjectID.String())
	event.SetMetadata("request_type", requestType)
	event.SetMetadata("due_date", dueDate.Format(time.RFC3339))

	return event
}