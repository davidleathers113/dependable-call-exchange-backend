package audit

import (
	"time"

	"github.com/google/uuid"
)

// Security Domain Events
// These events are published when security-related actions occur

// AuthenticationSuccessEvent is published when a user successfully authenticates
type AuthenticationSuccessEvent struct {
	*BaseDomainEvent
	UserID            uuid.UUID `json:"user_id"`
	SessionID         uuid.UUID `json:"session_id"`
	AuthMethod        string    `json:"auth_method"`
	AuthProvider      string    `json:"auth_provider,omitempty"`
	IPAddress         string    `json:"ip_address"`
	UserAgent         string    `json:"user_agent"`
	GeolocationData   string    `json:"geolocation_data,omitempty"`
	DeviceFingerprint string    `json:"device_fingerprint,omitempty"`
	Is2FAUsed         bool      `json:"is_2fa_used"`
	TokenType         string    `json:"token_type"`
	TokenExpiresAt    time.Time `json:"token_expires_at"`
	LoginDuration     int64     `json:"login_duration_ms"`
	PreviousLoginAt   *time.Time `json:"previous_login_at,omitempty"`
	RiskScore         float64   `json:"risk_score"`
	TrustLevel        string    `json:"trust_level"`
}

// NewAuthenticationSuccessEvent creates a new authentication success event
func NewAuthenticationSuccessEvent(actorID string, userID, sessionID uuid.UUID, authMethod, ipAddress string) *AuthenticationSuccessEvent {
	base := NewBaseDomainEvent(EventAuthSuccess, actorID, userID.String(), "authentication_success")
	base.TargetType = "user"
	base.ActorType = "user"
	base.ActorIP = ipAddress

	event := &AuthenticationSuccessEvent{
		BaseDomainEvent:   base,
		UserID:            userID,
		SessionID:         sessionID,
		AuthMethod:        authMethod,
		IPAddress:         ipAddress,
		Is2FAUsed:         false,
		TokenType:         "bearer",
		TokenExpiresAt:    time.Now().UTC().Add(24 * time.Hour),
		LoginDuration:     500, // 500ms
		RiskScore:         0.1,
		TrustLevel:        "medium",
	}

	// Mark as security sensitive
	event.MarkSecuritySensitive()
	event.MarkContainsPII()

	// Add relevant data classes
	event.AddDataClass("authentication_data")
	event.AddDataClass("session_data")
	event.AddDataClass("personal_data")
	event.AddDataClass("ip_address")

	// Set metadata for authentication success
	event.SetMetadata("action_type", "authentication_success")
	event.SetMetadata("user_id", userID.String())
	event.SetMetadata("session_id", sessionID.String())
	event.SetMetadata("auth_method", authMethod)
	event.SetMetadata("risk_score", 0.1)

	return event
}

// AuthenticationFailureEvent is published when authentication fails
type AuthenticationFailureEvent struct {
	*BaseDomainEvent
	UserID              *uuid.UUID `json:"user_id,omitempty"`
	AttemptedUsername   string     `json:"attempted_username,omitempty"`
	AuthMethod          string     `json:"auth_method"`
	FailureReason       string     `json:"failure_reason"`
	FailureCode         string     `json:"failure_code"`
	IPAddress           string     `json:"ip_address"`
	UserAgent           string     `json:"user_agent"`
	GeolocationData     string     `json:"geolocation_data,omitempty"`
	DeviceFingerprint   string     `json:"device_fingerprint,omitempty"`
	ConsecutiveFailures int        `json:"consecutive_failures"`
	IsAccountLocked     bool       `json:"is_account_locked"`
	LockoutDuration     *int       `json:"lockout_duration_minutes,omitempty"`
	RiskScore           float64    `json:"risk_score"`
	SuspiciousActivity  bool       `json:"suspicious_activity"`
	BlockedAt           *time.Time `json:"blocked_at,omitempty"`
}

// NewAuthenticationFailureEvent creates a new authentication failure event
func NewAuthenticationFailureEvent(actorID string, authMethod, failureReason, ipAddress string) *AuthenticationFailureEvent {
	base := NewBaseDomainEvent(EventAuthFailure, actorID, actorID, "authentication_failure")
	base.TargetType = "user"
	base.ActorType = "user"
	base.ActorIP = ipAddress

	event := &AuthenticationFailureEvent{
		BaseDomainEvent:     base,
		AuthMethod:          authMethod,
		FailureReason:       failureReason,
		FailureCode:         "invalid_credentials",
		IPAddress:           ipAddress,
		ConsecutiveFailures: 1,
		IsAccountLocked:     false,
		RiskScore:           0.5,
		SuspiciousActivity:  false,
	}

	// Mark as security sensitive
	event.MarkSecuritySensitive()

	// Add relevant data classes
	event.AddDataClass("authentication_data")
	event.AddDataClass("security_event")
	event.AddDataClass("ip_address")

	// Set metadata for authentication failure
	event.SetMetadata("action_type", "authentication_failure")
	event.SetMetadata("auth_method", authMethod)
	event.SetMetadata("failure_reason", failureReason)
	event.SetMetadata("risk_score", 0.5)

	return event
}

// AccessDeniedEvent is published when access to a resource is denied
type AccessDeniedEvent struct {
	*BaseDomainEvent
	UserID            uuid.UUID `json:"user_id"`
	ResourceType      string    `json:"resource_type"`
	ResourceID        string    `json:"resource_id"`
	RequestedAction   string    `json:"requested_action"`
	DenialReason      string    `json:"denial_reason"`
	RequiredRole      string    `json:"required_role,omitempty"`
	UserRole          string    `json:"user_role,omitempty"`
	RequiredPermission string   `json:"required_permission,omitempty"`
	IPAddress         string    `json:"ip_address"`
	UserAgent         string    `json:"user_agent"`
	SessionID         uuid.UUID `json:"session_id"`
	Endpoint          string    `json:"endpoint,omitempty"`
	HTTPMethod        string    `json:"http_method,omitempty"`
	RiskScore         float64   `json:"risk_score"`
	IsEscalated       bool      `json:"is_escalated"`
}

// NewAccessDeniedEvent creates a new access denied event
func NewAccessDeniedEvent(actorID string, userID uuid.UUID, resourceType, resourceID, requestedAction, denialReason string) *AccessDeniedEvent {
	base := NewBaseDomainEvent(EventAccessDenied, actorID, resourceID, "access_denied")
	base.TargetType = resourceType
	base.ActorType = "user"

	event := &AccessDeniedEvent{
		BaseDomainEvent: base,
		UserID:          userID,
		ResourceType:    resourceType,
		ResourceID:      resourceID,
		RequestedAction: requestedAction,
		DenialReason:    denialReason,
		RiskScore:       0.3,
		IsEscalated:     false,
	}

	// Mark as security sensitive
	event.MarkSecuritySensitive()

	// Add relevant data classes
	event.AddDataClass("access_control_data")
	event.AddDataClass("security_event")
	event.AddDataClass("authorization_data")

	// Set metadata for access denial
	event.SetMetadata("action_type", "access_denied")
	event.SetMetadata("user_id", userID.String())
	event.SetMetadata("resource_type", resourceType)
	event.SetMetadata("resource_id", resourceID)
	event.SetMetadata("requested_action", requestedAction)
	event.SetMetadata("denial_reason", denialReason)

	return event
}

// AnomalyDetectedEvent is published when suspicious activity is detected
type AnomalyDetectedEvent struct {
	*BaseDomainEvent
	AnomalyID         uuid.UUID `json:"anomaly_id"`
	AnomalyType       string    `json:"anomaly_type"`
	DetectionMethod   string    `json:"detection_method"`
	Severity          string    `json:"severity_level"`
	Description       string    `json:"description"`
	AffectedUserID    *uuid.UUID `json:"affected_user_id,omitempty"`
	AffectedResource  string    `json:"affected_resource,omitempty"`
	IPAddress         string    `json:"ip_address,omitempty"`
	UserAgent         string    `json:"user_agent,omitempty"`
	DetectedAt        time.Time `json:"detected_at"`
	ConfidenceScore   float64   `json:"confidence_score"`
	RiskScore         float64   `json:"risk_score"`
	MLModelVersion    string    `json:"ml_model_version,omitempty"`
	Indicators        []string  `json:"indicators"`
	Response          string    `json:"response"`
	IsBlocked         bool      `json:"is_blocked"`
	EscalationLevel   int       `json:"escalation_level"`
	AssignedTo        string    `json:"assigned_to,omitempty"`
	Context           map[string]interface{} `json:"context,omitempty"`
}

// NewAnomalyDetectedEvent creates a new anomaly detected event
func NewAnomalyDetectedEvent(actorID string, anomalyID uuid.UUID, anomalyType, description string) *AnomalyDetectedEvent {
	base := NewBaseDomainEvent(EventAnomalyDetected, actorID, anomalyID.String(), "anomaly_detected")
	base.TargetType = "anomaly"
	base.ActorType = "system"

	event := &AnomalyDetectedEvent{
		BaseDomainEvent: base,
		AnomalyID:       anomalyID,
		AnomalyType:     anomalyType,
		DetectionMethod: "ml_model",
		Severity:        "medium",
		Description:     description,
		DetectedAt:      time.Now().UTC(),
		ConfidenceScore: 0.85,
		RiskScore:       0.6,
		MLModelVersion:  "v2.1",
		Indicators:      make([]string, 0),
		Response:        "monitor",
		IsBlocked:       false,
		EscalationLevel: 1,
		Context:         make(map[string]interface{}),
	}

	// Mark as security sensitive and requiring signature
	event.MarkSecuritySensitive()
	event.MarkRequiresSignature()

	// Add relevant data classes
	event.AddDataClass("anomaly_data")
	event.AddDataClass("security_event")
	event.AddDataClass("threat_intelligence")

	// Set metadata for anomaly detection
	event.SetMetadata("action_type", "anomaly_detection")
	event.SetMetadata("anomaly_type", anomalyType)
	event.SetMetadata("confidence_score", 0.85)
	event.SetMetadata("risk_score", 0.6)

	return event
}

// PermissionChangedEvent is published when user permissions are modified
type PermissionChangedEvent struct {
	*BaseDomainEvent
	TargetUserID      uuid.UUID              `json:"target_user_id"`
	ModifiedByUserID  uuid.UUID              `json:"modified_by_user_id"`
	ChangeType        string                 `json:"change_type"`
	ResourceType      string                 `json:"resource_type,omitempty"`
	ResourceID        string                 `json:"resource_id,omitempty"`
	PreviousRole      string                 `json:"previous_role,omitempty"`
	NewRole           string                 `json:"new_role,omitempty"`
	AddedPermissions  []string               `json:"added_permissions,omitempty"`
	RemovedPermissions []string              `json:"removed_permissions,omitempty"`
	ChangeReason      string                 `json:"change_reason"`
	ApprovalRequired  bool                   `json:"approval_required"`
	ApprovedBy        *uuid.UUID             `json:"approved_by,omitempty"`
	ApprovedAt        *time.Time             `json:"approved_at,omitempty"`
	EffectiveAt       time.Time              `json:"effective_at"`
	ExpiresAt         *time.Time             `json:"expires_at,omitempty"`
	IsTemporary       bool                   `json:"is_temporary"`
	Context           map[string]interface{} `json:"context,omitempty"`
}

// NewPermissionChangedEvent creates a new permission changed event
func NewPermissionChangedEvent(actorID string, targetUserID, modifiedByUserID uuid.UUID, changeType, changeReason string) *PermissionChangedEvent {
	base := NewBaseDomainEvent(EventPermissionChanged, actorID, targetUserID.String(), "permission_changed")
	base.TargetType = "user"
	base.ActorType = "admin"

	event := &PermissionChangedEvent{
		BaseDomainEvent:    base,
		TargetUserID:       targetUserID,
		ModifiedByUserID:   modifiedByUserID,
		ChangeType:         changeType,
		ChangeReason:       changeReason,
		ApprovalRequired:   false,
		EffectiveAt:        time.Now().UTC(),
		IsTemporary:        false,
		AddedPermissions:   make([]string, 0),
		RemovedPermissions: make([]string, 0),
		Context:            make(map[string]interface{}),
	}

	// Mark as security sensitive and requiring signature
	event.MarkSecuritySensitive()
	event.MarkRequiresSignature()

	// Add relevant data classes
	event.AddDataClass("permission_data")
	event.AddDataClass("access_control_data")
	event.AddDataClass("security_event")

	// Set metadata for permission change
	event.SetMetadata("action_type", "permission_change")
	event.SetMetadata("target_user_id", targetUserID.String())
	event.SetMetadata("modified_by_user_id", modifiedByUserID.String())
	event.SetMetadata("change_type", changeType)
	event.SetMetadata("change_reason", changeReason)

	return event
}

// SessionTerminatedEvent is published when a user session is terminated
type SessionTerminatedEvent struct {
	*BaseDomainEvent
	SessionID         uuid.UUID `json:"session_id"`
	UserID            uuid.UUID `json:"user_id"`
	TerminationReason string    `json:"termination_reason"`
	TerminationType   string    `json:"termination_type"`
	SessionDuration   int64     `json:"session_duration_ms"`
	StartedAt         time.Time `json:"started_at"`
	TerminatedAt      time.Time `json:"terminated_at"`
	IPAddress         string    `json:"ip_address"`
	UserAgent         string    `json:"user_agent"`
	LastActivity      time.Time `json:"last_activity"`
	IsForced          bool      `json:"is_forced"`
	TerminatedBy      *uuid.UUID `json:"terminated_by,omitempty"`
	SecurityIncident  bool      `json:"security_incident"`
}

// NewSessionTerminatedEvent creates a new session terminated event
func NewSessionTerminatedEvent(actorID string, sessionID, userID uuid.UUID, reason string) *SessionTerminatedEvent {
	base := NewBaseDomainEvent(EventAuthSuccess, actorID, sessionID.String(), "session_terminated")
	base.TargetType = "session"
	base.ActorType = "system"

	now := time.Now().UTC()
	startedAt := now.Add(-2 * time.Hour) // Example: 2 hours ago

	event := &SessionTerminatedEvent{
		BaseDomainEvent:   base,
		SessionID:         sessionID,
		UserID:            userID,
		TerminationReason: reason,
		TerminationType:   "timeout",
		SessionDuration:   7200000, // 2 hours in milliseconds
		StartedAt:         startedAt,
		TerminatedAt:      now,
		LastActivity:      now.Add(-30 * time.Minute),
		IsForced:          false,
		SecurityIncident:  false,
	}

	// Mark as security sensitive
	event.MarkSecuritySensitive()

	// Add relevant data classes
	event.AddDataClass("session_data")
	event.AddDataClass("authentication_data")
	event.AddDataClass("security_event")

	// Set metadata for session termination
	event.SetMetadata("action_type", "session_termination")
	event.SetMetadata("session_id", sessionID.String())
	event.SetMetadata("user_id", userID.String())
	event.SetMetadata("termination_reason", reason)
	event.SetMetadata("session_duration_hours", 2)

	return event
}

// DataExfiltrationAttemptEvent is published when data exfiltration is detected
type DataExfiltrationAttemptEvent struct {
	*BaseDomainEvent
	AttemptID         uuid.UUID `json:"attempt_id"`
	UserID            uuid.UUID `json:"user_id"`
	SessionID         uuid.UUID `json:"session_id"`
	DataType          string    `json:"data_type"`
	DataVolume        int64     `json:"data_volume_bytes"`
	ExtractionMethod  string    `json:"extraction_method"`
	DetectionMethod   string    `json:"detection_method"`
	IPAddress         string    `json:"ip_address"`
	UserAgent         string    `json:"user_agent"`
	AttemptedAt       time.Time `json:"attempted_at"`
	WasBlocked        bool      `json:"was_blocked"`
	BlockReason       string    `json:"block_reason,omitempty"`
	ConfidenceScore   float64   `json:"confidence_score"`
	RiskScore         float64   `json:"risk_score"`
	MLModelVersion    string    `json:"ml_model_version,omitempty"`
	Indicators        []string  `json:"indicators"`
	ResponseActions   []string  `json:"response_actions"`
	IsEscalated       bool      `json:"is_escalated"`
}

// NewDataExfiltrationAttemptEvent creates a new data exfiltration attempt event
func NewDataExfiltrationAttemptEvent(actorID string, attemptID, userID, sessionID uuid.UUID, dataType string, dataVolume int64) *DataExfiltrationAttemptEvent {
	base := NewBaseDomainEvent(EventAnomalyDetected, actorID, attemptID.String(), "data_exfiltration_attempt")
	base.TargetType = "data_exfiltration"
	base.ActorType = "system"

	event := &DataExfiltrationAttemptEvent{
		BaseDomainEvent:  base,
		AttemptID:        attemptID,
		UserID:           userID,
		SessionID:        sessionID,
		DataType:         dataType,
		DataVolume:       dataVolume,
		ExtractionMethod: "bulk_download",
		DetectionMethod:  "ml_anomaly_detection",
		AttemptedAt:      time.Now().UTC(),
		WasBlocked:       true,
		BlockReason:      "anomalous_volume",
		ConfidenceScore:  0.95,
		RiskScore:        0.9,
		MLModelVersion:   "v3.0",
		Indicators:       []string{"unusual_volume", "off_hours", "new_location"},
		ResponseActions:  []string{"block_user", "alert_admin", "log_incident"},
		IsEscalated:      true,
	}

	// Mark as security sensitive and requiring signature
	event.MarkSecuritySensitive()
	event.MarkRequiresSignature()

	// Add relevant data classes
	event.AddDataClass("security_incident")
	event.AddDataClass("data_breach_attempt")
	event.AddDataClass("threat_intelligence")

	// Set metadata for data exfiltration attempt
	event.SetMetadata("action_type", "data_exfiltration_attempt")
	event.SetMetadata("user_id", userID.String())
	event.SetMetadata("data_type", dataType)
	event.SetMetadata("data_volume_mb", dataVolume/1024/1024)
	event.SetMetadata("confidence_score", 0.95)
	event.SetMetadata("risk_score", 0.9)

	return event
}