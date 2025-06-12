package telephony

import (
	"context"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/call"
	"github.com/google/uuid"
)

// Service defines the telephony service interface
type Service interface {
	// InitiateCall starts a new outbound call
	InitiateCall(ctx context.Context, req *InitiateCallRequest) (*CallResponse, error)
	// TerminateCall ends an active call
	TerminateCall(ctx context.Context, callID uuid.UUID) error
	// GetCallStatus returns the current status of a call
	GetCallStatus(ctx context.Context, callID uuid.UUID) (*CallStatus, error)
	// TransferCall transfers a call to another number
	TransferCall(ctx context.Context, callID uuid.UUID, toNumber string) error
	// RecordCall starts or stops call recording
	RecordCall(ctx context.Context, callID uuid.UUID, record bool) error
	// SendDTMF sends DTMF tones on a call
	SendDTMF(ctx context.Context, callID uuid.UUID, digits string) error
	// BridgeCalls bridges two calls together
	BridgeCalls(ctx context.Context, callID1, callID2 uuid.UUID) error
	// HandleWebhook processes telephony provider webhooks
	HandleWebhook(ctx context.Context, provider string, data interface{}) error
}

// CallRepository defines the interface for call storage
type CallRepository interface {
	// GetByID retrieves a call by ID
	GetByID(ctx context.Context, callID uuid.UUID) (*call.Call, error)
	// GetByCallSID retrieves a call by provider SID
	GetByCallSID(ctx context.Context, callSID string) (*call.Call, error)
	// Update updates a call
	Update(ctx context.Context, call *call.Call) error
	// Create creates a new call
	Create(ctx context.Context, call *call.Call) error
}

// Provider defines the interface for telephony providers (Twilio, Vonage, etc.)
type Provider interface {
	// InitiateCall starts a new call
	InitiateCall(ctx context.Context, from, to string, callbackURL string) (string, error)
	// TerminateCall ends a call
	TerminateCall(ctx context.Context, callSID string) error
	// GetCallStatus gets call status
	GetCallStatus(ctx context.Context, callSID string) (*ProviderCallStatus, error)
	// TransferCall transfers a call
	TransferCall(ctx context.Context, callSID string, to string) error
	// SendDTMF sends DTMF tones
	SendDTMF(ctx context.Context, callSID string, digits string) error
	// BridgeCalls bridges two calls
	BridgeCalls(ctx context.Context, callSID1, callSID2 string) error
	// GetProviderName returns the provider name
	GetProviderName() string
}

// EventPublisher defines the interface for publishing telephony events
type EventPublisher interface {
	// PublishCallEvent publishes a call event
	PublishCallEvent(ctx context.Context, event *CallEvent) error
}

// MetricsCollector defines the interface for collecting telephony metrics
type MetricsCollector interface {
	// RecordCallInitiated records a call initiation
	RecordCallInitiated(ctx context.Context, provider string)
	// RecordCallCompleted records a call completion
	RecordCallCompleted(ctx context.Context, duration time.Duration, cost float64)
	// RecordCallFailed records a call failure
	RecordCallFailed(ctx context.Context, reason string)
	// RecordProviderLatency records provider API latency
	RecordProviderLatency(ctx context.Context, provider string, operation string, latency time.Duration)
}

// InitiateCallRequest represents a request to initiate a call
type InitiateCallRequest struct {
	FromNumber  string
	ToNumber    string
	BuyerID     uuid.UUID
	SellerID    *uuid.UUID
	CallbackURL string
	RecordCall  bool
	MaxDuration int // Maximum call duration in seconds
	Metadata    map[string]string
}

// CallResponse represents a response from call operations
type CallResponse struct {
	CallID    uuid.UUID
	CallSID   string
	Status    call.Status
	StartTime time.Time
	Provider  string
}

// CallStatus represents the current status of a call
type CallStatus struct {
	CallID      uuid.UUID
	Status      call.Status
	Duration    *int
	RecordingID *string
	StartTime   time.Time
	EndTime     *time.Time
	Cost        *float64
}

// ProviderCallStatus represents call status from a provider
type ProviderCallStatus struct {
	CallSID     string
	Status      string
	Duration    int
	Price       *float64
	Currency    string
	RecordingID *string
}

// CallEvent represents a telephony event
type CallEvent struct {
	EventID   uuid.UUID
	CallID    uuid.UUID
	EventType string
	Timestamp time.Time
	Data      map[string]interface{}
}

// WebhookData represents incoming webhook data from providers
type WebhookData struct {
	Provider  string
	EventType string
	CallSID   string
	Status    string
	Duration  *int
	Price     *float64
	Timestamp time.Time
	RawData   map[string]interface{}
}
