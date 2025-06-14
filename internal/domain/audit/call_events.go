package audit

import (
	"time"

	"github.com/google/uuid"
)

// Call Domain Events
// These events are published by the call domain when audit-worthy actions occur

// CallInitiatedEvent is published when a new call is initiated
type CallInitiatedEvent struct {
	*BaseDomainEvent
	CallID       uuid.UUID `json:"call_id"`
	FromNumber   string    `json:"from_number"`
	ToNumber     string    `json:"to_number"`
	Direction    string    `json:"direction"`
	BuyerID      uuid.UUID `json:"buyer_id"`
	SellerID     *uuid.UUID `json:"seller_id,omitempty"`
	CallType     string    `json:"call_type"`
	UserAgent    string    `json:"user_agent,omitempty"`
	IPAddress    string    `json:"ip_address,omitempty"`
}

// NewCallInitiatedEvent creates a new call initiated event
func NewCallInitiatedEvent(actorID string, callID uuid.UUID, fromNumber, toNumber string) *CallInitiatedEvent {
	base := NewBaseDomainEvent(EventCallInitiated, actorID, callID.String(), "call_initiated")
	base.TargetType = "call"
	base.ActorType = "user"

	event := &CallInitiatedEvent{
		BaseDomainEvent: base,
		CallID:          callID,
		FromNumber:      fromNumber,
		ToNumber:        toNumber,
	}

	// Mark as TCPA relevant due to phone numbers
	event.MarkTCPARelevant()
	
	// Add phone number data classes
	event.AddDataClass("phone_number")
	event.AddDataClass("call_data")

	// Set metadata for call initiation
	event.SetMetadata("action_type", "call_initiation")
	event.SetMetadata("call_direction", "outbound")

	return event
}

// CallRoutedEvent is published when a call is successfully routed to a buyer
type CallRoutedEvent struct {
	*BaseDomainEvent
	CallID          uuid.UUID  `json:"call_id"`
	RouteID         uuid.UUID  `json:"route_id"`
	BuyerID         uuid.UUID  `json:"buyer_id"`
	SellerID        *uuid.UUID `json:"seller_id,omitempty"`
	WinningBidID    *uuid.UUID `json:"winning_bid_id,omitempty"`
	RoutingDuration int64      `json:"routing_duration_ms"`
	Algorithm       string     `json:"routing_algorithm"`
	Score           float64    `json:"routing_score"`
}

// NewCallRoutedEvent creates a new call routed event
func NewCallRoutedEvent(actorID string, callID, routeID, buyerID uuid.UUID) *CallRoutedEvent {
	base := NewBaseDomainEvent(EventCallRouted, actorID, callID.String(), "call_routed")
	base.TargetType = "call"
	base.ActorType = "system"

	event := &CallRoutedEvent{
		BaseDomainEvent: base,
		CallID:          callID,
		RouteID:         routeID,
		BuyerID:         buyerID,
	}

	// Mark as containing call routing data
	event.AddDataClass("call_data")
	event.AddDataClass("routing_data")

	// Set metadata for routing decision
	event.SetMetadata("action_type", "call_routing")
	event.SetMetadata("buyer_id", buyerID.String())
	event.SetMetadata("route_id", routeID.String())

	return event
}

// CallCompletedEvent is published when a call is completed successfully
type CallCompletedEvent struct {
	*BaseDomainEvent
	CallID       uuid.UUID  `json:"call_id"`
	BuyerID      uuid.UUID  `json:"buyer_id"`
	SellerID     *uuid.UUID `json:"seller_id,omitempty"`
	Duration     int        `json:"duration_seconds"`
	StartTime    time.Time  `json:"start_time"`
	EndTime      time.Time  `json:"end_time"`
	Cost         string     `json:"cost,omitempty"`
	Quality      float64    `json:"quality_score"`
	WasRecorded  bool       `json:"was_recorded"`
	DisconnectBy string     `json:"disconnect_by"`
}

// NewCallCompletedEvent creates a new call completed event
func NewCallCompletedEvent(actorID string, callID, buyerID uuid.UUID, duration int) *CallCompletedEvent {
	base := NewBaseDomainEvent(EventCallCompleted, actorID, callID.String(), "call_completed")
	base.TargetType = "call"
	base.ActorType = "system"

	event := &CallCompletedEvent{
		BaseDomainEvent: base,
		CallID:          callID,
		BuyerID:         buyerID,
		Duration:        duration,
		EndTime:         time.Now().UTC(),
	}

	// Mark as containing call data and potentially financial data
	event.AddDataClass("call_data")
	if event.Cost != "" {
		event.MarkFinancialData()
	}

	// Set metadata for call completion
	event.SetMetadata("action_type", "call_completion")
	event.SetMetadata("call_duration", duration)
	event.SetMetadata("buyer_id", buyerID.String())

	return event
}

// CallFailedEvent is published when a call fails to complete
type CallFailedEvent struct {
	*BaseDomainEvent
	CallID       uuid.UUID  `json:"call_id"`
	BuyerID      uuid.UUID  `json:"buyer_id"`
	SellerID     *uuid.UUID `json:"seller_id,omitempty"`
	FailureCode  string     `json:"failure_code"`
	FailureStage string     `json:"failure_stage"`
	AttemptCount int        `json:"attempt_count"`
	Duration     int        `json:"duration_seconds"`
}

// NewCallFailedEvent creates a new call failed event
func NewCallFailedEvent(actorID string, callID, buyerID uuid.UUID, failureCode, failureStage string) *CallFailedEvent {
	base := NewBaseDomainEvent(EventCallFailed, actorID, callID.String(), "call_failed")
	base.TargetType = "call"
	base.ActorType = "system"
	base.SetFailure(failureCode, "Call failed during "+failureStage)

	event := &CallFailedEvent{
		BaseDomainEvent: base,
		CallID:          callID,
		BuyerID:         buyerID,
		FailureCode:     failureCode,
		FailureStage:    failureStage,
	}

	// Mark as containing call data
	event.AddDataClass("call_data")

	// Set metadata for call failure
	event.SetMetadata("action_type", "call_failure")
	event.SetMetadata("failure_code", failureCode)
	event.SetMetadata("failure_stage", failureStage)
	event.SetMetadata("buyer_id", buyerID.String())

	return event
}

// RecordingConsentEvent is published when recording consent is granted or revoked
type RecordingConsentEvent struct {
	*BaseDomainEvent
	CallID        uuid.UUID `json:"call_id"`
	ParticipantID uuid.UUID `json:"participant_id"`
	ConsentGiven  bool      `json:"consent_given"`
	ConsentMethod string    `json:"consent_method"`
	Timestamp     time.Time `json:"consent_timestamp"`
	IPAddress     string    `json:"ip_address,omitempty"`
	UserAgent     string    `json:"user_agent,omitempty"`
}

// NewRecordingConsentEvent creates a new recording consent event
func NewRecordingConsentEvent(actorID string, callID, participantID uuid.UUID, consentGiven bool) *RecordingConsentEvent {
	action := "recording_consent_granted"
	eventType := EventConsentGranted
	if !consentGiven {
		action = "recording_consent_revoked"
		eventType = EventConsentRevoked
	}

	base := NewBaseDomainEvent(eventType, actorID, callID.String(), action)
	base.TargetType = "call"
	base.ActorType = "user"

	event := &RecordingConsentEvent{
		BaseDomainEvent: base,
		CallID:          callID,
		ParticipantID:   participantID,
		ConsentGiven:    consentGiven,
		Timestamp:       time.Now().UTC(),
	}

	// Mark as GDPR and TCPA relevant due to consent and recording
	event.MarkGDPRRelevant("consent")
	event.MarkTCPARelevant()
	event.MarkRequiresSignature()

	// Add relevant data classes
	event.AddDataClass("consent_data")
	event.AddDataClass("call_data")
	event.AddDataClass("recording_data")

	// Set metadata for consent
	event.SetMetadata("action_type", "recording_consent")
	event.SetMetadata("consent_given", consentGiven)
	event.SetMetadata("participant_id", participantID.String())

	return event
}

// CallTransferEvent is published when a call is transferred
type CallTransferEvent struct {
	*BaseDomainEvent
	CallID           uuid.UUID `json:"call_id"`
	FromParticipant  uuid.UUID `json:"from_participant"`
	ToParticipant    uuid.UUID `json:"to_participant"`
	TransferType     string    `json:"transfer_type"`
	TransferReason   string    `json:"transfer_reason,omitempty"`
	WasSuccessful    bool      `json:"was_successful"`
}

// NewCallTransferEvent creates a new call transfer event
func NewCallTransferEvent(actorID string, callID, fromParticipant, toParticipant uuid.UUID, transferType string) *CallTransferEvent {
	base := NewBaseDomainEvent(EventCallRouted, actorID, callID.String(), "call_transferred")
	base.TargetType = "call"
	base.ActorType = "user"

	event := &CallTransferEvent{
		BaseDomainEvent: base,
		CallID:          callID,
		FromParticipant: fromParticipant,
		ToParticipant:   toParticipant,
		TransferType:    transferType,
		WasSuccessful:   true,
	}

	// Mark as containing call data
	event.AddDataClass("call_data")

	// Set metadata for transfer
	event.SetMetadata("action_type", "call_transfer")
	event.SetMetadata("transfer_type", transferType)
	event.SetMetadata("from_participant", fromParticipant.String())
	event.SetMetadata("to_participant", toParticipant.String())

	return event
}

// CallQualityEvent is published when call quality metrics are recorded
type CallQualityEvent struct {
	*BaseDomainEvent
	CallID        uuid.UUID            `json:"call_id"`
	QualityScore  float64              `json:"quality_score"`
	AudioQuality  float64              `json:"audio_quality"`
	Latency       int                  `json:"latency_ms"`
	PacketLoss    float64              `json:"packet_loss_percent"`
	Jitter        int                  `json:"jitter_ms"`
	Metrics       map[string]interface{} `json:"quality_metrics"`
}

// NewCallQualityEvent creates a new call quality event
func NewCallQualityEvent(actorID string, callID uuid.UUID, qualityScore float64) *CallQualityEvent {
	base := NewBaseDomainEvent(EventCallCompleted, actorID, callID.String(), "call_quality_recorded")
	base.TargetType = "call"
	base.ActorType = "system"

	event := &CallQualityEvent{
		BaseDomainEvent: base,
		CallID:          callID,
		QualityScore:    qualityScore,
		Metrics:         make(map[string]interface{}),
	}

	// Mark as containing call data
	event.AddDataClass("call_data")
	event.AddDataClass("quality_metrics")

	// Set metadata for quality recording
	event.SetMetadata("action_type", "quality_recording")
	event.SetMetadata("quality_score", qualityScore)

	return event
}

// CallRecordingStartedEvent is published when call recording begins
type CallRecordingStartedEvent struct {
	*BaseDomainEvent
	CallID             uuid.UUID `json:"call_id"`
	RecordingID        uuid.UUID `json:"recording_id"`
	RecordingFormat    string    `json:"recording_format"`
	ConsentVerified    bool      `json:"consent_verified"`
	ParticipantsCount  int       `json:"participants_count"`
	StorageLocation    string    `json:"storage_location,omitempty"`
	EncryptionEnabled  bool      `json:"encryption_enabled"`
}

// NewCallRecordingStartedEvent creates a new recording started event
func NewCallRecordingStartedEvent(actorID string, callID, recordingID uuid.UUID) *CallRecordingStartedEvent {
	base := NewBaseDomainEvent(EventRecordingStarted, actorID, callID.String(), "recording_started")
	base.TargetType = "call"
	base.ActorType = "system"

	event := &CallRecordingStartedEvent{
		BaseDomainEvent:   base,
		CallID:            callID,
		RecordingID:       recordingID,
		EncryptionEnabled: true, // Default to encrypted
	}

	// Mark as GDPR and TCPA relevant due to recording
	event.MarkGDPRRelevant("legitimate_interest")
	event.MarkTCPARelevant()
	event.MarkRequiresSignature()

	// Add relevant data classes
	event.AddDataClass("recording_data")
	event.AddDataClass("call_data")
	event.AddDataClass("audio_data")

	// Set metadata for recording
	event.SetMetadata("action_type", "recording_start")
	event.SetMetadata("recording_id", recordingID.String())
	event.SetMetadata("encryption_enabled", true)

	return event
}