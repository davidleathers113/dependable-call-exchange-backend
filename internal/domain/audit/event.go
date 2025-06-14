package audit

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
	"github.com/google/uuid"
)

// Event represents an immutable audit log entry
// Following DCE patterns: immutable after creation, validation in constructor
type Event struct {
	// Immutable identifiers (set once, never modified)
	ID            uuid.UUID `json:"id"`
	SequenceNum   int64     `json:"sequence_num"`
	Timestamp     time.Time `json:"timestamp"`
	TimestampNano int64     `json:"timestamp_nano"`

	// Event classification
	Type     EventType `json:"type"`
	Severity Severity  `json:"severity"`
	Category string    `json:"category"`

	// Actor information (who performed the action)
	ActorID    string `json:"actor_id"`
	ActorType  string `json:"actor_type"` // user, system, api
	ActorIP    string `json:"actor_ip,omitempty"`
	ActorAgent string `json:"actor_agent,omitempty"`

	// Target information (what was acted upon)
	TargetID    string `json:"target_id"`
	TargetType  string `json:"target_type"`
	TargetOwner string `json:"target_owner,omitempty"`

	// Action details
	Action       string `json:"action"`
	Result       string `json:"result"` // success, failure, partial
	ErrorCode    string `json:"error_code,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`

	// Request correlation
	RequestID     string `json:"request_id"`
	SessionID     string `json:"session_id,omitempty"`
	CorrelationID string `json:"correlation_id,omitempty"`

	// Service metadata
	Environment string `json:"environment"`
	Service     string `json:"service"`
	Version     string `json:"version"`

	// Compliance metadata
	ComplianceFlags map[string]bool `json:"compliance_flags,omitempty"`
	DataClasses     []string        `json:"data_classes,omitempty"`
	LegalBasis      string          `json:"legal_basis,omitempty"`
	RetentionDays   int             `json:"retention_days"`

	// Additional context
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	Tags     []string               `json:"tags,omitempty"`

	// Cryptographic integrity
	PreviousHash string `json:"previous_hash"`
	EventHash    string `json:"event_hash"`
	Signature    string `json:"signature,omitempty"`

	// Immutability marker - set to true after hash calculation
	immutable bool `json:"-"`
}

// NewEvent creates a new audit event with validation
// Following DCE pattern: all validation in domain constructors
func NewEvent(eventType EventType, actorID, targetID, action string) (*Event, error) {
	// Validate required fields
	if err := validateEventType(eventType); err != nil {
		return nil, errors.NewValidationError("INVALID_EVENT_TYPE", 
			"event type must be valid").WithCause(err)
	}

	if actorID == "" {
		return nil, errors.NewValidationError("MISSING_ACTOR_ID", 
			"actor ID is required")
	}

	if targetID == "" {
		return nil, errors.NewValidationError("MISSING_TARGET_ID", 
			"target ID is required")
	}

	if action == "" {
		return nil, errors.NewValidationError("MISSING_ACTION", 
			"action is required")
	}

	now := time.Now().UTC()
	
	event := &Event{
		ID:            uuid.New(),
		Timestamp:     now,
		TimestampNano: now.UnixNano(),
		Type:          eventType,
		Severity:      SeverityInfo, // Default severity
		Category:      deriveCategory(eventType),
		ActorID:       actorID,
		TargetID:      targetID,
		Action:        action,
		Result:        "success", // Default result
		Environment:   getEnvironment(),
		Service:       getServiceName(),
		Version:       getServiceVersion(),
		RetentionDays: 2555, // 7 years default
		Metadata:      make(map[string]interface{}),
		ComplianceFlags: make(map[string]bool),
		Tags:          make([]string, 0),
		immutable:     false,
	}

	return event, nil
}

// ComputeHash calculates the SHA-256 hash chain for integrity
// This follows the specification's hash chaining approach
func (e *Event) ComputeHash(previousHash string) (string, error) {
	if e.immutable {
		return "", errors.NewBusinessError("EVENT_IMMUTABLE", 
			"cannot compute hash on immutable event")
	}

	e.PreviousHash = previousHash

	// Create deterministic JSON representation for hashing
	// Include only immutable fields that affect integrity
	hashData := map[string]interface{}{
		"id":             e.ID.String(),
		"sequence_num":   e.SequenceNum,
		"timestamp_nano": e.TimestampNano,
		"type":           string(e.Type),
		"actor_id":       e.ActorID,
		"target_id":      e.TargetID,
		"action":         e.Action,
		"result":         e.Result,
		"previous_hash":  e.PreviousHash,
	}

	jsonBytes, err := json.Marshal(hashData)
	if err != nil {
		return "", errors.NewInternalError("failed to marshal hash data").WithCause(err)
	}

	hash := sha256.Sum256(jsonBytes)
	e.EventHash = fmt.Sprintf("%x", hash)
	
	// Mark as immutable after hash calculation
	e.immutable = true

	return e.EventHash, nil
}

// IsImmutable returns whether the event has been made immutable
func (e *Event) IsImmutable() bool {
	return e.immutable
}

// Validate performs comprehensive validation of the event
func (e *Event) Validate() error {
	// Validate event type
	if err := validateEventType(e.Type); err != nil {
		return errors.NewValidationError("INVALID_EVENT_TYPE", 
			"event type validation failed").WithCause(err)
	}

	// Validate severity
	if err := validateSeverity(e.Severity); err != nil {
		return errors.NewValidationError("INVALID_SEVERITY", 
			"severity validation failed").WithCause(err)
	}

	// Validate required fields
	if e.ActorID == "" {
		return errors.NewValidationError("MISSING_ACTOR_ID", "actor ID is required")
	}

	if e.TargetID == "" {
		return errors.NewValidationError("MISSING_TARGET_ID", "target ID is required")
	}

	if e.Action == "" {
		return errors.NewValidationError("MISSING_ACTION", "action is required")
	}

	// Validate result
	if !isValidResult(e.Result) {
		return errors.NewValidationError("INVALID_RESULT", 
			"result must be 'success', 'failure', or 'partial'")
	}

	// Validate retention period
	if e.RetentionDays <= 0 {
		return errors.NewValidationError("INVALID_RETENTION", 
			"retention days must be positive")
	}

	// Validate hash chain if event is immutable
	if e.immutable && e.EventHash == "" {
		return errors.NewValidationError("MISSING_HASH", 
			"immutable event must have hash")
	}

	return nil
}

// HasComplianceFlag checks if a specific compliance flag is set and true
func (e *Event) HasComplianceFlag(flag string) bool {
	if e.ComplianceFlags == nil {
		return false
	}
	value, exists := e.ComplianceFlags[flag]
	return exists && value
}

// IsGDPRRelevant checks if the event contains GDPR-relevant data
func (e *Event) IsGDPRRelevant() bool {
	return e.HasComplianceFlag("gdpr_relevant") || 
		   e.HasComplianceFlag("contains_pii") ||
		   containsGDPRDataClasses(e.DataClasses)
}

// IsTCPARelevant checks if the event is relevant for TCPA compliance
func (e *Event) IsTCPARelevant() bool {
	return e.HasComplianceFlag("tcpa_relevant") ||
		   e.Type == EventConsentGranted ||
		   e.Type == EventConsentRevoked ||
		   e.Type == EventCallInitiated
}

// GetRetentionExpiryDate calculates when this event should be archived/deleted
func (e *Event) GetRetentionExpiryDate() time.Time {
	return e.Timestamp.AddDate(0, 0, e.RetentionDays)
}

// IsRetentionExpired checks if the event has exceeded its retention period
func (e *Event) IsRetentionExpired() bool {
	return time.Now().UTC().After(e.GetRetentionExpiryDate())
}

// Clone creates a deep copy of the event (used for testing)
// The clone will not be immutable and will need new hash computation
func (e *Event) Clone() *Event {
	clone := &Event{
		ID:            e.ID,
		SequenceNum:   e.SequenceNum,
		Timestamp:     e.Timestamp,
		TimestampNano: e.TimestampNano,
		Type:          e.Type,
		Severity:      e.Severity,
		Category:      e.Category,
		ActorID:       e.ActorID,
		ActorType:     e.ActorType,
		ActorIP:       e.ActorIP,
		ActorAgent:    e.ActorAgent,
		TargetID:      e.TargetID,
		TargetType:    e.TargetType,
		TargetOwner:   e.TargetOwner,
		Action:        e.Action,
		Result:        e.Result,
		ErrorCode:     e.ErrorCode,
		ErrorMessage:  e.ErrorMessage,
		RequestID:     e.RequestID,
		SessionID:     e.SessionID,
		CorrelationID: e.CorrelationID,
		Environment:   e.Environment,
		Service:       e.Service,
		Version:       e.Version,
		LegalBasis:    e.LegalBasis,
		RetentionDays: e.RetentionDays,
		PreviousHash:  e.PreviousHash,
		EventHash:     e.EventHash,
		Signature:     e.Signature,
		immutable:     false, // Clone is mutable until re-hashed
	}

	// Deep copy slices and maps
	if e.DataClasses != nil {
		clone.DataClasses = make([]string, len(e.DataClasses))
		copy(clone.DataClasses, e.DataClasses)
	}

	if e.Tags != nil {
		clone.Tags = make([]string, len(e.Tags))
		copy(clone.Tags, e.Tags)
	}

	if e.ComplianceFlags != nil {
		clone.ComplianceFlags = make(map[string]bool)
		for k, v := range e.ComplianceFlags {
			clone.ComplianceFlags[k] = v
		}
	}

	if e.Metadata != nil {
		clone.Metadata = make(map[string]interface{})
		for k, v := range e.Metadata {
			clone.Metadata[k] = v
		}
	}

	return clone
}

// Helper functions

// validateEventType validates that the event type is known and valid
func validateEventType(eventType EventType) error {
	if eventType == "" {
		return fmt.Errorf("event type cannot be empty")
	}

	// Check if it's a known event type
	validTypes := []EventType{
		// Consent/Compliance events
		EventConsentGranted, EventConsentRevoked, EventConsentUpdated, EventOptOutRequested,
		EventComplianceViolation, EventTCPAComplianceCheck, EventGDPRDataRequest, EventRecordingConsent,
		// Data access events
		EventDataAccessed, EventDataExported, EventDataDeleted, EventDataModified,
		// Call events
		EventCallInitiated, EventCallRouted, EventCallCompleted, EventCallFailed, EventRecordingStarted,
		// Configuration events
		EventConfigChanged, EventRuleUpdated, EventPermissionChanged,
		// Security events
		EventAuthSuccess, EventAuthFailure, EventAccessDenied, EventAnomalyDetected,
		EventSessionTerminated, EventDataExfiltrationAttempt,
		// Marketplace events
		EventBidPlaced, EventBidWon, EventBidLost, EventBidCancelled, EventBidModified,
		EventBidExpired, EventAuctionCompleted,
		// Financial events
		EventPaymentProcessed, EventTransactionCompleted, EventChargebackInitiated,
		EventRefundProcessed, EventPayoutInitiated, EventFinancialComplianceCheck,
		// System events
		EventAPICall, EventDatabaseQuery, EventSystemStartup, EventSystemShutdown,
	}

	for _, validType := range validTypes {
		if eventType == validType {
			return nil
		}
	}

	return fmt.Errorf("unknown event type: %s", eventType)
}

// validateSeverity validates the severity level
func validateSeverity(severity Severity) error {
	switch severity {
	case SeverityInfo, SeverityWarning, SeverityError, SeverityCritical:
		return nil
	default:
		return fmt.Errorf("invalid severity: %s", severity)
	}
}

// isValidResult checks if the result value is valid
func isValidResult(result string) bool {
	return result == "success" || result == "failure" || result == "partial"
}

// containsGDPRDataClasses checks if any data classes are GDPR-relevant
func containsGDPRDataClasses(dataClasses []string) bool {
	gdprDataClasses := []string{
		"personal_data", "sensitive_data", "biometric_data", 
		"health_data", "email", "phone_number", "ip_address",
		"location_data", "financial_data",
	}

	for _, dataClass := range dataClasses {
		for _, gdprClass := range gdprDataClasses {
			if dataClass == gdprClass {
				return true
			}
		}
	}
	return false
}

// deriveCategory derives the category from event type
func deriveCategory(eventType EventType) string {
	categoryMap := map[EventType]string{
		EventConsentGranted:     "consent",
		EventConsentRevoked:     "consent",
		EventConsentUpdated:     "consent",
		EventOptOutRequested:    "consent",
		EventDataAccessed:       "data_access",
		EventDataExported:       "data_access",
		EventDataDeleted:        "data_access",
		EventDataModified:       "data_access",
		EventCallInitiated:      "call",
		EventCallRouted:         "call",
		EventCallCompleted:      "call",
		EventCallFailed:         "call",
		EventRecordingStarted:   "call",
		EventConfigChanged:      "configuration",
		EventRuleUpdated:        "configuration",
		EventPermissionChanged:  "configuration",
		EventAuthSuccess:        "security",
		EventAuthFailure:        "security",
		EventAccessDenied:       "security",
		EventAnomalyDetected:    "security",
		EventBidPlaced:          "marketplace",
		EventBidWon:             "marketplace",
		EventBidLost:            "marketplace",
		EventPaymentProcessed:   "financial",
		EventAPICall:            "system",
		EventDatabaseQuery:      "system",
		EventSystemStartup:      "system",
		EventSystemShutdown:     "system",
	}

	if category, exists := categoryMap[eventType]; exists {
		return category
	}
	return "other"
}

// getEnvironment returns the current environment (configurable)
func getEnvironment() string {
	// This would typically come from configuration
	return "development" // Default value
}

// getServiceName returns the service name
func getServiceName() string {
	return "dce-backend"
}

// getServiceVersion returns the service version
func getServiceVersion() string {
	return "1.0.0" // This would come from build info
}