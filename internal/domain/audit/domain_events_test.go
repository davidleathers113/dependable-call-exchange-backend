package audit

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBaseDomainEvent_Creation(t *testing.T) {
	actorID := "user123"
	targetID := "target456"
	action := "test_action"
	eventType := EventCallInitiated

	event := NewBaseDomainEvent(eventType, actorID, targetID, action)

	assert.NotZero(t, event.EventID)
	assert.Equal(t, eventType, event.EventType)
	assert.Equal(t, actorID, event.ActorID)
	assert.Equal(t, targetID, event.TargetID)
	assert.Equal(t, action, event.Action)
	assert.Equal(t, ResultSuccess, event.Result)
	assert.Equal(t, 1, event.Version)
	assert.NotZero(t, event.Timestamp)
	assert.NotNil(t, event.ComplianceFlags)
	assert.NotNil(t, event.DataClasses)
	assert.NotNil(t, event.Metadata)
}

func TestBaseDomainEvent_ComplianceFlags(t *testing.T) {
	event := NewBaseDomainEvent(EventConsentGranted, "actor", "target", "action")

	// Test adding compliance flags
	event.AddComplianceFlag("gdpr_relevant", true)
	event.AddComplianceFlag("tcpa_relevant", false)

	assert.True(t, event.ComplianceFlags["gdpr_relevant"])
	assert.False(t, event.ComplianceFlags["tcpa_relevant"])

	// Test convenience methods
	event.MarkGDPRRelevant("consent")
	assert.True(t, event.ComplianceFlags["gdpr_relevant"])
	assert.Equal(t, "consent", event.LegalBasis)
	assert.Contains(t, event.DataClasses, "personal_data")

	event.MarkTCPARelevant()
	assert.True(t, event.ComplianceFlags["tcpa_relevant"])
	assert.Contains(t, event.DataClasses, "phone_number")

	event.MarkContainsPII()
	assert.True(t, event.ComplianceFlags["contains_pii"])

	event.MarkFinancialData()
	assert.True(t, event.ComplianceFlags["financial_data"])
	assert.Contains(t, event.DataClasses, "financial_data")

	event.MarkSecuritySensitive()
	assert.True(t, event.ComplianceFlags["security_sensitive"])

	event.MarkRequiresSignature()
	assert.True(t, event.ComplianceFlags["requires_signature"])
}

func TestBaseDomainEvent_DataClasses(t *testing.T) {
	event := NewBaseDomainEvent(EventCallInitiated, "actor", "target", "action")

	// Test adding data classes
	event.AddDataClass("personal_data")
	event.AddDataClass("phone_number")
	event.AddDataClass("personal_data") // Duplicate should be ignored

	assert.Len(t, event.DataClasses, 2)
	assert.Contains(t, event.DataClasses, "personal_data")
	assert.Contains(t, event.DataClasses, "phone_number")
}

func TestBaseDomainEvent_SetFailure(t *testing.T) {
	event := NewBaseDomainEvent(EventCallInitiated, "actor", "target", "action")

	event.SetFailure("ERROR_CODE", "Something went wrong")

	assert.Equal(t, ResultFailure, event.Result)
	assert.Equal(t, "ERROR_CODE", event.ErrorCode)
	assert.Equal(t, "Something went wrong", event.ErrorMessage)
}

func TestBaseDomainEvent_SetPartial(t *testing.T) {
	event := NewBaseDomainEvent(EventCallInitiated, "actor", "target", "action")

	event.SetPartial("Partially completed due to timeout")

	assert.Equal(t, ResultPartial, event.Result)
	assert.Equal(t, "Partially completed due to timeout", event.Metadata["partial_reason"])
}

func TestBaseDomainEvent_ToAuditEvent(t *testing.T) {
	event := NewBaseDomainEvent(EventCallInitiated, "actor", "target", "action")
	event.ActorType = "user"
	event.TargetType = "call"
	event.AddDataClass("call_data")
	event.MarkTCPARelevant()
	event.SetMetadata("test_key", "test_value")

	auditEvent, err := event.ToAuditEvent()
	require.NoError(t, err)
	require.NotNil(t, auditEvent)

	assert.Equal(t, event.EventType, auditEvent.Type)
	assert.Equal(t, event.ActorID, auditEvent.ActorID)
	assert.Equal(t, event.ActorType, auditEvent.ActorType)
	assert.Equal(t, event.TargetID, auditEvent.TargetID)
	assert.Equal(t, event.TargetType, auditEvent.TargetType)
	assert.Equal(t, event.Action, auditEvent.Action)
	assert.Equal(t, string(event.Result), auditEvent.Result)
	assert.Equal(t, event.RequestID, auditEvent.RequestID)
	assert.Equal(t, event.LegalBasis, auditEvent.LegalBasis)
	assert.Equal(t, event.ComplianceFlags["tcpa_relevant"], auditEvent.ComplianceFlags["tcpa_relevant"])
	assert.Contains(t, auditEvent.DataClasses, "call_data")
	assert.Equal(t, "test_value", auditEvent.Metadata["test_key"])
}

// Test Call Domain Events

func TestCallInitiatedEvent(t *testing.T) {
	actorID := "user123"
	callID := uuid.New()
	fromNumber := "+1234567890"
	toNumber := "+9876543210"

	event := NewCallInitiatedEvent(actorID, callID, fromNumber, toNumber)

	assert.Equal(t, EventCallInitiated, event.EventType)
	assert.Equal(t, actorID, event.ActorID)
	assert.Equal(t, callID.String(), event.TargetID)
	assert.Equal(t, "call_initiated", event.Action)
	assert.Equal(t, "call", event.TargetType)
	assert.Equal(t, "user", event.ActorType)
	assert.Equal(t, callID, event.CallID)
	assert.Equal(t, fromNumber, event.FromNumber)
	assert.Equal(t, toNumber, event.ToNumber)
	assert.True(t, event.ComplianceFlags["tcpa_relevant"])
	assert.Contains(t, event.DataClasses, "phone_number")
	assert.Contains(t, event.DataClasses, "call_data")
}

func TestCallRoutedEvent(t *testing.T) {
	actorID := "system"
	callID := uuid.New()
	routeID := uuid.New()
	buyerID := uuid.New()

	event := NewCallRoutedEvent(actorID, callID, routeID, buyerID)

	assert.Equal(t, EventCallRouted, event.EventType)
	assert.Equal(t, actorID, event.ActorID)
	assert.Equal(t, callID.String(), event.TargetID)
	assert.Equal(t, "call_routed", event.Action)
	assert.Equal(t, "call", event.TargetType)
	assert.Equal(t, "system", event.ActorType)
	assert.Equal(t, callID, event.CallID)
	assert.Equal(t, routeID, event.RouteID)
	assert.Equal(t, buyerID, event.BuyerID)
	assert.Contains(t, event.DataClasses, "call_data")
	assert.Contains(t, event.DataClasses, "routing_data")
}

func TestCallCompletedEvent(t *testing.T) {
	actorID := "system"
	callID := uuid.New()
	buyerID := uuid.New()
	duration := 300 // 5 minutes

	event := NewCallCompletedEvent(actorID, callID, buyerID, duration)

	assert.Equal(t, EventCallCompleted, event.EventType)
	assert.Equal(t, actorID, event.ActorID)
	assert.Equal(t, callID.String(), event.TargetID)
	assert.Equal(t, "call_completed", event.Action)
	assert.Equal(t, callID, event.CallID)
	assert.Equal(t, buyerID, event.BuyerID)
	assert.Equal(t, duration, event.Duration)
	assert.Contains(t, event.DataClasses, "call_data")
}

func TestCallFailedEvent(t *testing.T) {
	actorID := "system"
	callID := uuid.New()
	buyerID := uuid.New()
	failureCode := "NO_ANSWER"
	failureStage := "dialing"

	event := NewCallFailedEvent(actorID, callID, buyerID, failureCode, failureStage)

	assert.Equal(t, EventCallFailed, event.EventType)
	assert.Equal(t, actorID, event.ActorID)
	assert.Equal(t, callID.String(), event.TargetID)
	assert.Equal(t, "call_failed", event.Action)
	assert.Equal(t, ResultFailure, event.Result)
	assert.Equal(t, failureCode, event.ErrorCode)
	assert.Equal(t, callID, event.CallID)
	assert.Equal(t, buyerID, event.BuyerID)
	assert.Equal(t, failureCode, event.FailureCode)
	assert.Equal(t, failureStage, event.FailureStage)
}

func TestRecordingConsentEvent(t *testing.T) {
	actorID := "user123"
	callID := uuid.New()
	participantID := uuid.New()

	// Test consent granted
	grantedEvent := NewRecordingConsentEvent(actorID, callID, participantID, true)
	assert.Equal(t, EventConsentGranted, grantedEvent.EventType)
	assert.Equal(t, "recording_consent_granted", grantedEvent.Action)
	assert.True(t, grantedEvent.ConsentGiven)
	assert.True(t, grantedEvent.ComplianceFlags["gdpr_relevant"])
	assert.True(t, grantedEvent.ComplianceFlags["tcpa_relevant"])
	assert.True(t, grantedEvent.ComplianceFlags["requires_signature"])

	// Test consent revoked
	revokedEvent := NewRecordingConsentEvent(actorID, callID, participantID, false)
	assert.Equal(t, EventConsentRevoked, revokedEvent.EventType)
	assert.Equal(t, "recording_consent_revoked", revokedEvent.Action)
	assert.False(t, revokedEvent.ConsentGiven)
}

func TestCallRecordingStartedEvent(t *testing.T) {
	actorID := "system"
	callID := uuid.New()
	recordingID := uuid.New()

	event := NewCallRecordingStartedEvent(actorID, callID, recordingID)

	assert.Equal(t, EventRecordingStarted, event.EventType)
	assert.Equal(t, actorID, event.ActorID)
	assert.Equal(t, callID.String(), event.TargetID)
	assert.Equal(t, "recording_started", event.Action)
	assert.Equal(t, callID, event.CallID)
	assert.Equal(t, recordingID, event.RecordingID)
	assert.True(t, event.EncryptionEnabled)
	assert.True(t, event.ComplianceFlags["gdpr_relevant"])
	assert.True(t, event.ComplianceFlags["tcpa_relevant"])
	assert.Contains(t, event.DataClasses, "recording_data")
}

// Test Bid Domain Events

func TestBidPlacedEvent(t *testing.T) {
	actorID := "user123"
	bidID := uuid.New()
	callID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New()
	amount := "5.00"

	event := NewBidPlacedEvent(actorID, bidID, callID, buyerID, sellerID, amount)

	assert.Equal(t, EventBidPlaced, event.EventType)
	assert.Equal(t, actorID, event.ActorID)
	assert.Equal(t, bidID.String(), event.TargetID)
	assert.Equal(t, "bid_placed", event.Action)
	assert.Equal(t, "bid", event.TargetType)
	assert.Equal(t, "user", event.ActorType)
	assert.Equal(t, bidID, event.BidID)
	assert.Equal(t, callID, event.CallID)
	assert.Equal(t, buyerID, event.BuyerID)
	assert.Equal(t, sellerID, event.SellerID)
	assert.Equal(t, amount, event.Amount)
	assert.Equal(t, "USD", event.Currency)
	assert.True(t, event.ComplianceFlags["financial_data"])
	assert.Contains(t, event.DataClasses, "bid_data")
	assert.Contains(t, event.DataClasses, "financial_data")
}

func TestBidWonEvent(t *testing.T) {
	actorID := "system"
	bidID := uuid.New()
	callID := uuid.New()
	buyerID := uuid.New()
	sellerID := uuid.New()
	winningAmount := "7.50"

	event := NewBidWonEvent(actorID, bidID, callID, buyerID, sellerID, winningAmount)

	assert.Equal(t, EventBidWon, event.EventType)
	assert.Equal(t, actorID, event.ActorID)
	assert.Equal(t, bidID.String(), event.TargetID)
	assert.Equal(t, "bid_won", event.Action)
	assert.Equal(t, bidID, event.BidID)
	assert.Equal(t, winningAmount, event.WinningAmount)
	assert.Equal(t, 1, event.FinalRank)
	assert.True(t, event.ComplianceFlags["financial_data"])
	assert.True(t, event.ComplianceFlags["requires_signature"])
	assert.Contains(t, event.DataClasses, "auction_data")
}

func TestAuctionCompletedEvent(t *testing.T) {
	actorID := "system"
	auctionID := uuid.New()
	callID := uuid.New()
	sellerID := uuid.New()
	totalBids := 5

	event := NewAuctionCompletedEvent(actorID, auctionID, callID, sellerID, totalBids)

	assert.Equal(t, EventBidWon, event.EventType)
	assert.Equal(t, actorID, event.ActorID)
	assert.Equal(t, auctionID.String(), event.TargetID)
	assert.Equal(t, "auction_completed", event.Action)
	assert.Equal(t, auctionID, event.AuctionID)
	assert.Equal(t, callID, event.CallID)
	assert.Equal(t, sellerID, event.SellerID)
	assert.Equal(t, totalBids, event.TotalBids)
	assert.Equal(t, "realtime", event.AuctionType)
	assert.True(t, event.ComplianceFlags["financial_data"])
}

// Test Compliance Domain Events

func TestConsentGrantedEvent(t *testing.T) {
	actorID := "user123"
	consentID := uuid.New()
	dataSubjectID := uuid.New()
	consentType := "marketing"
	purpose := "email marketing"

	event := NewConsentGrantedEvent(actorID, consentID, dataSubjectID, consentType, purpose)

	assert.Equal(t, EventConsentGranted, event.EventType)
	assert.Equal(t, actorID, event.ActorID)
	assert.Equal(t, consentID.String(), event.TargetID)
	assert.Equal(t, "consent_granted", event.Action)
	assert.Equal(t, "consent", event.TargetType)
	assert.Equal(t, "user", event.ActorType)
	assert.Equal(t, consentID, event.ConsentID)
	assert.Equal(t, dataSubjectID, event.DataSubjectID)
	assert.Equal(t, consentType, event.ConsentType)
	assert.Equal(t, purpose, event.Purpose)
	assert.Equal(t, "consent", event.LegalBasisType)
	assert.Equal(t, "explicit", event.OptInMethod)
	assert.True(t, event.ComplianceFlags["gdpr_relevant"])
	assert.True(t, event.ComplianceFlags["requires_signature"])
	assert.Contains(t, event.DataClasses, "consent_data")
}

func TestConsentRevokedEvent(t *testing.T) {
	actorID := "user123"
	consentID := uuid.New()
	dataSubjectID := uuid.New()
	consentType := "marketing"
	reason := "user_request"

	event := NewConsentRevokedEvent(actorID, consentID, dataSubjectID, consentType, reason)

	assert.Equal(t, EventConsentRevoked, event.EventType)
	assert.Equal(t, actorID, event.ActorID)
	assert.Equal(t, consentID.String(), event.TargetID)
	assert.Equal(t, "consent_revoked", event.Action)
	assert.Equal(t, consentID, event.ConsentID)
	assert.Equal(t, dataSubjectID, event.DataSubjectID)
	assert.Equal(t, consentType, event.ConsentType)
	assert.Equal(t, reason, event.RevocationReason)
	assert.Equal(t, "user_request", event.RevocationMethod)
	assert.Equal(t, "immediate_deletion", event.DataRetention)
	assert.True(t, event.ComplianceFlags["gdpr_relevant"])
	assert.Contains(t, event.DataClasses, "revocation_data")
}

func TestOptOutRequestedEvent(t *testing.T) {
	actorID := "user123"
	optOutID := uuid.New()
	dataSubjectID := uuid.New()
	optOutType := "all_communications"
	scope := []string{"email", "sms", "calls"}

	event := NewOptOutRequestedEvent(actorID, optOutID, dataSubjectID, optOutType, scope)

	assert.Equal(t, EventOptOutRequested, event.EventType)
	assert.Equal(t, actorID, event.ActorID)
	assert.Equal(t, optOutID.String(), event.TargetID)
	assert.Equal(t, "opt_out_requested", event.Action)
	assert.Equal(t, optOutID, event.OptOutID)
	assert.Equal(t, dataSubjectID, event.DataSubjectID)
	assert.Equal(t, optOutType, event.OptOutType)
	assert.Equal(t, scope, event.OptOutScope)
	assert.Equal(t, "web_form", event.RequestMethod)
	assert.Equal(t, "pending", event.ProcessingStatus)
	assert.False(t, event.ConfirmationSent)
	assert.True(t, event.ComplianceFlags["gdpr_relevant"])
	assert.True(t, event.ComplianceFlags["tcpa_relevant"])
}

func TestComplianceViolationEvent(t *testing.T) {
	actorID := "system"
	violationID := uuid.New()
	violationType := "tcpa_violation"
	regulation := "TCPA"

	event := NewComplianceViolationEvent(actorID, violationID, violationType, regulation)

	assert.Equal(t, EventAnomalyDetected, event.EventType)
	assert.Equal(t, actorID, event.ActorID)
	assert.Equal(t, violationID.String(), event.TargetID)
	assert.Equal(t, "compliance_violation_detected", event.Action)
	assert.Equal(t, "compliance_violation", event.TargetType)
	assert.Equal(t, "system", event.ActorType)
	// Severity is set during ToAuditEvent conversion based on event type
	assert.Equal(t, violationID, event.ViolationID)
	assert.Equal(t, violationType, event.ViolationType)
	assert.Equal(t, regulation, event.Regulation)
	assert.Equal(t, "high", event.Severity)
	assert.Equal(t, "automated", event.DetectionMethod)
	assert.Equal(t, "open", event.Status)
	assert.Equal(t, 1, event.EscalationLevel)
	assert.True(t, event.ComplianceFlags["security_sensitive"])
	assert.Contains(t, event.DataClasses, "compliance_data")
}

// Test Financial Domain Events

func TestPaymentProcessedEvent(t *testing.T) {
	actorID := "system"
	paymentID := uuid.New()
	transactionID := uuid.New()
	payerID := uuid.New()
	payeeID := uuid.New()
	amount := "25.00"

	event := NewPaymentProcessedEvent(actorID, paymentID, transactionID, payerID, payeeID, amount)

	assert.Equal(t, EventPaymentProcessed, event.EventType)
	assert.Equal(t, actorID, event.ActorID)
	assert.Equal(t, paymentID.String(), event.TargetID)
	assert.Equal(t, "payment_processed", event.Action)
	assert.Equal(t, "payment", event.TargetType)
	assert.Equal(t, "system", event.ActorType)
	assert.Equal(t, paymentID, event.PaymentID)
	assert.Equal(t, transactionID, event.TransactionID)
	assert.Equal(t, payerID, event.PayerID)
	assert.Equal(t, payeeID, event.PayeeID)
	assert.Equal(t, amount, event.Amount)
	assert.Equal(t, "USD", event.Currency)
	assert.Equal(t, "credit_card", event.PaymentMethod)
	assert.Equal(t, "stripe", event.PaymentProvider)
	assert.Equal(t, "completed", event.PaymentStatus)
	assert.True(t, event.ComplianceFlags["financial_data"])
	assert.True(t, event.ComplianceFlags["requires_signature"])
	assert.Contains(t, event.DataClasses, "payment_data")
}

func TestChargebackInitiatedEvent(t *testing.T) {
	actorID := "system"
	chargebackID := uuid.New()
	paymentID := uuid.New()
	transactionID := uuid.New()
	payerID := uuid.New()
	payeeID := uuid.New()
	amount := "25.00"
	reasonCode := "4855"

	event := NewChargebackInitiatedEvent(actorID, chargebackID, paymentID, transactionID, payerID, payeeID, amount, reasonCode)

	assert.Equal(t, EventPaymentProcessed, event.EventType)
	assert.Equal(t, actorID, event.ActorID)
	assert.Equal(t, chargebackID.String(), event.TargetID)
	assert.Equal(t, "chargeback_initiated", event.Action)
	assert.Equal(t, "chargeback", event.TargetType)
	// Severity is set during ToAuditEvent conversion based on event type
	assert.Equal(t, chargebackID, event.ChargebackID)
	assert.Equal(t, paymentID, event.PaymentID)
	assert.Equal(t, amount, event.Amount)
	assert.Equal(t, reasonCode, event.ReasonCode)
	assert.Equal(t, "Goods or Services Not Provided", event.ReasonDescription)
	assert.Equal(t, "open", event.Status)
	assert.Contains(t, event.EvidenceRequired, "receipt")
	assert.True(t, event.ComplianceFlags["financial_data"])
	assert.True(t, event.ComplianceFlags["security_sensitive"])
}

// Test Security Domain Events

func TestAuthenticationSuccessEvent(t *testing.T) {
	actorID := "user123"
	userID := uuid.New()
	sessionID := uuid.New()
	authMethod := "password"
	ipAddress := "192.168.1.100"

	event := NewAuthenticationSuccessEvent(actorID, userID, sessionID, authMethod, ipAddress)

	assert.Equal(t, EventAuthSuccess, event.EventType)
	assert.Equal(t, actorID, event.ActorID)
	assert.Equal(t, userID.String(), event.TargetID)
	assert.Equal(t, "authentication_success", event.Action)
	assert.Equal(t, "user", event.TargetType)
	assert.Equal(t, "user", event.ActorType)
	assert.Equal(t, ipAddress, event.ActorIP)
	assert.Equal(t, userID, event.UserID)
	assert.Equal(t, sessionID, event.SessionID)
	assert.Equal(t, authMethod, event.AuthMethod)
	assert.Equal(t, ipAddress, event.IPAddress)
	assert.False(t, event.Is2FAUsed)
	assert.Equal(t, "bearer", event.TokenType)
	assert.Equal(t, int64(500), event.LoginDuration)
	assert.Equal(t, 0.1, event.RiskScore)
	assert.Equal(t, "medium", event.TrustLevel)
	assert.True(t, event.ComplianceFlags["security_sensitive"])
	assert.Contains(t, event.DataClasses, "authentication_data")
}

func TestAuthenticationFailureEvent(t *testing.T) {
	actorID := "unknown"
	authMethod := "password"
	failureReason := "invalid_password"
	ipAddress := "192.168.1.100"

	event := NewAuthenticationFailureEvent(actorID, authMethod, failureReason, ipAddress)

	assert.Equal(t, EventAuthFailure, event.EventType)
	assert.Equal(t, actorID, event.ActorID)
	assert.Equal(t, actorID, event.TargetID)
	assert.Equal(t, "authentication_failure", event.Action)
	// Severity is set during ToAuditEvent conversion based on event type
	assert.Equal(t, authMethod, event.AuthMethod)
	assert.Equal(t, failureReason, event.FailureReason)
	assert.Equal(t, "invalid_credentials", event.FailureCode)
	assert.Equal(t, ipAddress, event.IPAddress)
	assert.Equal(t, 1, event.ConsecutiveFailures)
	assert.False(t, event.IsAccountLocked)
	assert.Equal(t, 0.5, event.RiskScore)
	assert.False(t, event.SuspiciousActivity)
	assert.True(t, event.ComplianceFlags["security_sensitive"])
}

func TestAnomalyDetectedEvent(t *testing.T) {
	actorID := "system"
	anomalyID := uuid.New()
	anomalyType := "unusual_login_pattern"
	description := "Multiple failed login attempts from different IPs"

	event := NewAnomalyDetectedEvent(actorID, anomalyID, anomalyType, description)

	assert.Equal(t, EventAnomalyDetected, event.EventType)
	assert.Equal(t, actorID, event.ActorID)
	assert.Equal(t, anomalyID.String(), event.TargetID)
	assert.Equal(t, "anomaly_detected", event.Action)
	assert.Equal(t, "anomaly", event.TargetType)
	assert.Equal(t, "system", event.ActorType)
	// Severity is set during ToAuditEvent conversion based on event type
	assert.Equal(t, anomalyID, event.AnomalyID)
	assert.Equal(t, anomalyType, event.AnomalyType)
	assert.Equal(t, description, event.Description)
	assert.Equal(t, "ml_model", event.DetectionMethod)
	assert.Equal(t, "medium", event.Severity)
	assert.Equal(t, 0.85, event.ConfidenceScore)
	assert.Equal(t, 0.6, event.RiskScore)
	assert.Equal(t, "v2.1", event.MLModelVersion)
	assert.Equal(t, "monitor", event.Response)
	assert.False(t, event.IsBlocked)
	assert.Equal(t, 1, event.EscalationLevel)
	assert.True(t, event.ComplianceFlags["security_sensitive"])
	assert.Contains(t, event.DataClasses, "anomaly_data")
}

// Integration tests

func TestDomainEvent_ToAuditEvent_Integration(t *testing.T) {
	tests := []struct {
		name  string
		event DomainEvent
	}{
		{
			name:  "CallInitiatedEvent",
			event: NewCallInitiatedEvent("user123", uuid.New(), "+1234567890", "+9876543210"),
		},
		{
			name:  "BidPlacedEvent",
			event: NewBidPlacedEvent("user123", uuid.New(), uuid.New(), uuid.New(), uuid.New(), "5.00"),
		},
		{
			name:  "ConsentGrantedEvent",
			event: NewConsentGrantedEvent("user123", uuid.New(), uuid.New(), "marketing", "email"),
		},
		{
			name:  "PaymentProcessedEvent",
			event: NewPaymentProcessedEvent("system", uuid.New(), uuid.New(), uuid.New(), uuid.New(), "25.00"),
		},
		{
			name:  "AuthenticationSuccessEvent",
			event: NewAuthenticationSuccessEvent("user123", uuid.New(), uuid.New(), "password", "192.168.1.1"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auditEvent, err := tt.event.ToAuditEvent()
			require.NoError(t, err)
			require.NotNil(t, auditEvent)

			// Validate the audit event
			err = auditEvent.Validate()
			assert.NoError(t, err)

			// Check that all required fields are populated
			assert.NotZero(t, auditEvent.ID)
			assert.NotZero(t, auditEvent.Timestamp)
			assert.NotEmpty(t, auditEvent.ActorID)
			assert.NotEmpty(t, auditEvent.TargetID)
			assert.NotEmpty(t, auditEvent.Action)
			assert.NotEmpty(t, auditEvent.Result)
		})
	}
}

func TestDomainEvent_Serialization(t *testing.T) {
	// Test that events can be properly serialized for audit logging
	callEvent := NewCallInitiatedEvent("user123", uuid.New(), "+1234567890", "+9876543210")
	callEvent.MarkTCPARelevant()
	callEvent.SetMetadata("test_key", "test_value")

	auditEvent, err := callEvent.ToAuditEvent()
	require.NoError(t, err)

	// Test that compliance flags are preserved
	assert.True(t, auditEvent.HasComplianceFlag("tcpa_relevant"))
	assert.True(t, auditEvent.IsTCPARelevant())

	// Test that metadata is preserved
	assert.Equal(t, "test_value", auditEvent.Metadata["test_key"])

	// Test that data classes are preserved
	assert.Contains(t, auditEvent.DataClasses, "phone_number")
	assert.Contains(t, auditEvent.DataClasses, "call_data")
}