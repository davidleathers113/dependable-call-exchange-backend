package fixtures

import (
	"testing"
	"time"
	
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/call"
)

// CallBuilder builds test Call entities
type CallBuilder struct {
	t         *testing.T
	id        uuid.UUID
	from      string
	to        string
	status    call.Status
	direction call.Direction
	buyerID   uuid.UUID
	sellerID  *uuid.UUID
	callSID   string
	location  *call.Location
}

// NewCallBuilder creates a new CallBuilder with defaults
func NewCallBuilder(t *testing.T) *CallBuilder {
	t.Helper()
	id, err := uuid.NewRandom()
	require.NoError(t, err)
	buyerID, err := uuid.NewRandom()
	require.NoError(t, err)
	
	return &CallBuilder{
		t:         t,
		id:        id,
		from:      "+15551234567",
		to:        "+15559876543",
		status:    call.StatusPending,
		direction: call.DirectionInbound,
		buyerID:   buyerID,
		callSID:   "CALL" + uuid.New().String()[:8],
	}
}

// WithID sets the call ID
func (b *CallBuilder) WithID(id uuid.UUID) *CallBuilder {
	b.id = id
	return b
}

// WithPhoneNumbers sets from and to numbers
func (b *CallBuilder) WithPhoneNumbers(from, to string) *CallBuilder {
	b.from = from
	b.to = to
	return b
}

// WithStatus sets the call status
func (b *CallBuilder) WithStatus(status call.Status) *CallBuilder {
	b.status = status
	return b
}

// WithDirection sets the call direction
func (b *CallBuilder) WithDirection(direction call.Direction) *CallBuilder {
	b.direction = direction
	return b
}

// WithBuyerID sets the buyer ID
func (b *CallBuilder) WithBuyerID(buyerID uuid.UUID) *CallBuilder {
	b.buyerID = buyerID
	return b
}

// WithSellerID sets the seller ID
func (b *CallBuilder) WithSellerID(sellerID uuid.UUID) *CallBuilder {
	b.sellerID = &sellerID
	return b
}

// WithLocation sets the call location
func (b *CallBuilder) WithLocation(location *call.Location) *CallBuilder {
	b.location = location
	return b
}

// Build creates the Call entity
func (b *CallBuilder) Build() *call.Call {
	now := time.Now().UTC()
	c := &call.Call{
		ID:         b.id,
		FromNumber: b.from,
		ToNumber:   b.to,
		Status:     b.status,
		Direction:  b.direction,
		BuyerID:    b.buyerID,
		SellerID:   b.sellerID,
		CallSID:    b.callSID,
		Location:   b.location,
		StartTime:  now,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	
	// Set additional fields based on status
	switch b.status {
	case call.StatusInProgress:
		// Call is in progress, no end time yet
	case call.StatusCompleted:
		endTime := now.Add(5 * time.Minute)
		duration := int(endTime.Sub(now).Seconds())
		cost := float64(duration) * 0.01 // $0.01 per second
		
		c.EndTime = &endTime
		c.Duration = &duration
		c.Cost = &cost
	}
	
	return c
}

// CallScenarios provides common call test scenarios
type CallScenarios struct {
	t *testing.T
}

// NewCallScenarios creates a new CallScenarios helper
func NewCallScenarios(t *testing.T) *CallScenarios {
	t.Helper()
	return &CallScenarios{t: t}
}

// InboundCall creates a typical inbound call
func (cs *CallScenarios) InboundCall() *call.Call {
	return NewCallBuilder(cs.t).
		WithDirection(call.DirectionInbound).
		WithLocation(&call.Location{
			Country:  "US",
			State:    "CA",
			City:     "Los Angeles",
			Timezone: "America/Los_Angeles",
		}).
		Build()
}

// OutboundCall creates a typical outbound call
func (cs *CallScenarios) OutboundCall() *call.Call {
	sellerID := uuid.New()
	return NewCallBuilder(cs.t).
		WithDirection(call.DirectionOutbound).
		WithSellerID(sellerID).
		Build()
}

// ActiveCall creates an active call
func (cs *CallScenarios) ActiveCall() *call.Call {
	return NewCallBuilder(cs.t).
		WithStatus(call.StatusInProgress).
		Build()
}

// CompletedCall creates a completed call with duration and cost
func (cs *CallScenarios) CompletedCall() *call.Call {
	return NewCallBuilder(cs.t).
		WithStatus(call.StatusCompleted).
		Build()
}

// FailedCall creates a failed call
func (cs *CallScenarios) FailedCall() *call.Call {
	return NewCallBuilder(cs.t).
		WithStatus(call.StatusFailed).
		Build()
}

// CallSet creates a set of calls for testing
func (cs *CallScenarios) CallSet(count int) []*call.Call {
	calls := make([]*call.Call, count)
	for i := 0; i < count; i++ {
		calls[i] = NewCallBuilder(cs.t).
			WithPhoneNumbers(
				GeneratePhoneNumber(cs.t),
				GeneratePhoneNumber(cs.t),
			).
			Build()
	}
	return calls
}

// GeneratePhoneNumber generates a valid test phone number
func GeneratePhoneNumber(t *testing.T) string {
	t.Helper()
	// Use 555-01XX range which is reserved for fictional use
	return "+1555010" + generateDigits(t, 4)
}

func generateDigits(t *testing.T, n int) string {
	t.Helper()
	digits := "0123456789"
	result := make([]byte, n)
	for i := 0; i < n; i++ {
		result[i] = digits[time.Now().UnixNano()%int64(len(digits))]
	}
	return string(result)
}