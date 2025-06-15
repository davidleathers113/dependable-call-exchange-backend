package audit

// EventType represents the category of audit event
// Following DCE patterns: typed constants for domain values
type EventType string

// Consent Management Events
const (
	EventConsentGranted       EventType = "consent.granted"
	EventConsentRevoked       EventType = "consent.revoked"
	EventConsentUpdated       EventType = "consent.updated"
	EventOptOutRequested      EventType = "consent.opt_out"
	EventComplianceViolation  EventType = "consent.compliance_violation"
	EventTCPAComplianceCheck  EventType = "consent.tcpa_check"
	EventGDPRDataRequest      EventType = "consent.gdpr_request"
	EventRecordingConsent     EventType = "consent.recording_consent"
)

// Data Access Events
const (
	EventDataAccessed EventType = "data.accessed"
	EventDataExported EventType = "data.exported"
	EventDataDeleted  EventType = "data.deleted"
	EventDataModified EventType = "data.modified"
)

// Call Events
const (
	EventCallInitiated    EventType = "call.initiated"
	EventCallRouted       EventType = "call.routed"
	EventCallCompleted    EventType = "call.completed"
	EventCallFailed       EventType = "call.failed"
	EventRecordingStarted EventType = "call.recording_started"
)

// Configuration Events
const (
	EventConfigChanged     EventType = "config.changed"
	EventRuleUpdated       EventType = "config.rule_updated"
	EventPermissionChanged EventType = "config.permission_changed"
)

// Security Events
const (
	EventAuthSuccess             EventType = "security.auth_success"
	EventAuthFailure             EventType = "security.auth_failure"
	EventAccessDenied            EventType = "security.access_denied"
	EventAnomalyDetected         EventType = "security.anomaly_detected"
	EventSessionTerminated       EventType = "security.session_terminated"
	EventDataExfiltrationAttempt EventType = "security.data_exfiltration_attempt"
)

// Marketplace Events
const (
	EventBidPlaced     EventType = "marketplace.bid_placed"
	EventBidWon        EventType = "marketplace.bid_won"
	EventBidLost       EventType = "marketplace.bid_lost"
	EventBidCancelled  EventType = "marketplace.bid_cancelled"
	EventBidModified   EventType = "marketplace.bid_modified"
	EventBidExpired    EventType = "marketplace.bid_expired"
	EventAuctionCompleted EventType = "marketplace.auction_completed"
)

// Financial Events
const (
	EventPaymentProcessed       EventType = "financial.payment_processed"
	EventTransactionCompleted   EventType = "financial.transaction_completed"
	EventChargebackInitiated    EventType = "financial.chargeback_initiated"
	EventRefundProcessed        EventType = "financial.refund_processed"
	EventPayoutInitiated        EventType = "financial.payout_initiated"
	EventFinancialComplianceCheck EventType = "financial.compliance_check"
)

// DNC Events
const (
	EventDNCNumberSuppressed   EventType = "dnc.number_suppressed"
	EventDNCNumberReleased     EventType = "dnc.number_released"
	EventDNCCheckPerformed     EventType = "dnc.check_performed"
	EventDNCListSynced         EventType = "dnc.list_synced"
)

// System Events
const (
	EventAPICall        EventType = "system.api_call"
	EventDatabaseQuery  EventType = "system.database_query"
	EventSystemStartup  EventType = "system.startup"
	EventSystemShutdown EventType = "system.shutdown"
)

// String returns the string representation of the event type
func (et EventType) String() string {
	return string(et)
}

// IsConsentEvent returns true if the event type is consent-related
func (et EventType) IsConsentEvent() bool {
	switch et {
	case EventConsentGranted, EventConsentRevoked, EventConsentUpdated, EventOptOutRequested:
		return true
	default:
		return false
	}
}

// IsDataEvent returns true if the event type is data access-related
func (et EventType) IsDataEvent() bool {
	switch et {
	case EventDataAccessed, EventDataExported, EventDataDeleted, EventDataModified:
		return true
	default:
		return false
	}
}

// IsCallEvent returns true if the event type is call-related
func (et EventType) IsCallEvent() bool {
	switch et {
	case EventCallInitiated, EventCallRouted, EventCallCompleted, EventCallFailed, EventRecordingStarted:
		return true
	default:
		return false
	}
}

// IsSecurityEvent returns true if the event type is security-related
func (et EventType) IsSecurityEvent() bool {
	switch et {
	case EventAuthSuccess, EventAuthFailure, EventAccessDenied, EventAnomalyDetected:
		return true
	default:
		return false
	}
}

// IsMarketplaceEvent returns true if the event type is marketplace-related
func (et EventType) IsMarketplaceEvent() bool {
	switch et {
	case EventBidPlaced, EventBidWon, EventBidLost:
		return true
	default:
		return false
	}
}

// GetDefaultSeverity returns the default severity level for this event type
func (et EventType) GetDefaultSeverity() Severity {
	switch et {
	case EventAuthFailure, EventAccessDenied, EventAnomalyDetected:
		return SeverityWarning
	case EventCallFailed, EventDataDeleted:
		return SeverityError
	case EventSystemShutdown:
		return SeverityCritical
	default:
		return SeverityInfo
	}
}

// RequiresSignature returns true if this event type requires cryptographic signing
func (et EventType) RequiresSignature() bool {
	switch et {
	case EventConsentGranted, EventConsentRevoked, EventDataDeleted, 
		 EventPaymentProcessed, EventPermissionChanged:
		return true
	default:
		return false
	}
}

// GetRetentionDays returns the default retention period for this event type
func (et EventType) GetRetentionDays() int {
	switch et {
	case EventConsentGranted, EventConsentRevoked, EventConsentUpdated:
		return 2555 // 7 years for consent records
	case EventPaymentProcessed:
		return 2920 // 8 years for financial records
	case EventDataDeleted:
		return 3650 // 10 years for deletion records
	case EventAuthFailure, EventAnomalyDetected:
		return 1095 // 3 years for security events
	default:
		return 2555 // 7 years default
	}
}

// Severity levels for audit events
type Severity string

const (
	SeverityInfo     Severity = "INFO"
	SeverityWarning  Severity = "WARNING"
	SeverityError    Severity = "ERROR"
	SeverityCritical Severity = "CRITICAL"
)

// String returns the string representation of the severity
func (s Severity) String() string {
	return string(s)
}

// Level returns a numeric level for the severity (higher = more severe)
func (s Severity) Level() int {
	switch s {
	case SeverityInfo:
		return 1
	case SeverityWarning:
		return 2
	case SeverityError:
		return 3
	case SeverityCritical:
		return 4
	default:
		return 0
	}
}

// IsAtLeast returns true if this severity is at least as severe as the other
func (s Severity) IsAtLeast(other Severity) bool {
	return s.Level() >= other.Level()
}

// GetColor returns a color code for the severity (useful for UI/logging)
func (s Severity) GetColor() string {
	switch s {
	case SeverityInfo:
		return "blue"
	case SeverityWarning:
		return "yellow"
	case SeverityError:
		return "red"
	case SeverityCritical:
		return "purple"
	default:
		return "gray"
	}
}

// Category represents the high-level category of an audit event
type Category string

const (
	CategoryConsent       Category = "consent"
	CategoryDataAccess    Category = "data_access"
	CategoryCall          Category = "call"
	CategoryConfiguration Category = "configuration"
	CategorySecurity      Category = "security"
	CategoryMarketplace   Category = "marketplace"
	CategoryFinancial     Category = "financial"
	CategorySystem        Category = "system"
	CategoryOther         Category = "other"
)

// String returns the string representation of the category
func (c Category) String() string {
	return string(c)
}

// GetIcon returns an icon representation for the category
func (c Category) GetIcon() string {
	switch c {
	case CategoryConsent:
		return "✓"
	case CategoryDataAccess:
		return "🔍"
	case CategoryCall:
		return "📞"
	case CategoryConfiguration:
		return "⚙️"
	case CategorySecurity:
		return "🔒"
	case CategoryMarketplace:
		return "💰"
	case CategoryFinancial:
		return "💳"
	case CategorySystem:
		return "🖥️"
	default:
		return "📋"
	}
}

// GetEventTypes returns all event types for this category
func (c Category) GetEventTypes() []EventType {
	switch c {
	case CategoryConsent:
		return []EventType{
			EventConsentGranted, EventConsentRevoked, 
			EventConsentUpdated, EventOptOutRequested,
		}
	case CategoryDataAccess:
		return []EventType{
			EventDataAccessed, EventDataExported, 
			EventDataDeleted, EventDataModified,
		}
	case CategoryCall:
		return []EventType{
			EventCallInitiated, EventCallRouted, 
			EventCallCompleted, EventCallFailed, EventRecordingStarted,
		}
	case CategoryConfiguration:
		return []EventType{
			EventConfigChanged, EventRuleUpdated, EventPermissionChanged,
		}
	case CategorySecurity:
		return []EventType{
			EventAuthSuccess, EventAuthFailure, 
			EventAccessDenied, EventAnomalyDetected,
		}
	case CategoryMarketplace:
		return []EventType{
			EventBidPlaced, EventBidWon, EventBidLost,
		}
	case CategoryFinancial:
		return []EventType{
			EventPaymentProcessed,
		}
	case CategorySystem:
		return []EventType{
			EventAPICall, EventDatabaseQuery, 
			EventSystemStartup, EventSystemShutdown,
		}
	default:
		return []EventType{}
	}
}

// Result represents the outcome of an audited action
type Result string

const (
	ResultSuccess Result = "success"
	ResultFailure Result = "failure"
	ResultPartial Result = "partial"
)

// String returns the string representation of the result
func (r Result) String() string {
	return string(r)
}

// IsSuccess returns true if the result indicates success
func (r Result) IsSuccess() bool {
	return r == ResultSuccess
}

// IsFailure returns true if the result indicates failure
func (r Result) IsFailure() bool {
	return r == ResultFailure
}

// IsPartial returns true if the result indicates partial success
func (r Result) IsPartial() bool {
	return r == ResultPartial
}

// GetIcon returns an icon representation for the result
func (r Result) GetIcon() string {
	switch r {
	case ResultSuccess:
		return "✅"
	case ResultFailure:
		return "❌"
	case ResultPartial:
		return "⚠️"
	default:
		return "❓"
	}
}

// GetDefaultSeverity returns the default severity for this result type
func (r Result) GetDefaultSeverity() Severity {
	switch r {
	case ResultSuccess:
		return SeverityInfo
	case ResultFailure:
		return SeverityError
	case ResultPartial:
		return SeverityWarning
	default:
		return SeverityInfo
	}
}