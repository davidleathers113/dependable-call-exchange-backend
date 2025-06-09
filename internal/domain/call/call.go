package call

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/validation"
)

type Call struct {
	ID          uuid.UUID `json:"id"`
	FromNumber  string    `json:"from_number"`
	ToNumber    string    `json:"to_number"`
	Status      Status    `json:"status"`
	Direction   Direction `json:"direction"`
	StartTime   time.Time `json:"start_time"`
	EndTime     *time.Time `json:"end_time,omitempty"`
	Duration    *int      `json:"duration,omitempty"`
	Cost        *float64  `json:"cost,omitempty"`
	
	// Call routing
	RouteID     *uuid.UUID `json:"route_id,omitempty"`
	BuyerID     uuid.UUID  `json:"buyer_id"`
	SellerID    *uuid.UUID `json:"seller_id,omitempty"`
	
	// Telephony details
	CallSID     string     `json:"call_sid"`
	SessionID   *string    `json:"session_id,omitempty"`
	
	// Metadata
	UserAgent  *string    `json:"user_agent,omitempty"`
	IPAddress  *string    `json:"ip_address,omitempty"`
	Location   *Location  `json:"location,omitempty"`
	
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type Status int

const (
	StatusPending Status = iota
	StatusQueued
	StatusRinging
	StatusInProgress
	StatusCompleted
	StatusFailed
	StatusCanceled
	StatusNoAnswer
	StatusBusy
)

func (s Status) String() string {
	switch s {
	case StatusPending:
		return "pending"
	case StatusQueued:
		return "queued"
	case StatusRinging:
		return "ringing"
	case StatusInProgress:
		return "in_progress"
	case StatusCompleted:
		return "completed"
	case StatusFailed:
		return "failed"
	case StatusCanceled:
		return "canceled"
	case StatusNoAnswer:
		return "no_answer"
	case StatusBusy:
		return "busy"
	default:
		return "unknown"
	}
}

type Direction int

const (
	DirectionInbound Direction = iota
	DirectionOutbound
)

func (d Direction) String() string {
	switch d {
	case DirectionInbound:
		return "inbound"
	case DirectionOutbound:
		return "outbound"
	default:
		return "unknown"
	}
}

type Location struct {
	Country   string  `json:"country"`
	State     string  `json:"state"`
	City      string  `json:"city"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Timezone  string  `json:"timezone"`
}

func NewCall(fromNumber, toNumber string, buyerID uuid.UUID, direction Direction) (*Call, error) {
	// Validate phone numbers
	if err := validation.ValidatePhoneNumber(fromNumber); err != nil {
		return nil, fmt.Errorf("invalid from number: %w", err)
	}
	
	if err := validation.ValidatePhoneNumber(toNumber); err != nil {
		return nil, fmt.Errorf("invalid to number: %w", err)
	}
	
	// Validate buyer ID
	if buyerID == uuid.Nil {
		return nil, fmt.Errorf("buyer ID cannot be nil")
	}
	
	// Validate direction
	switch direction {
	case DirectionInbound, DirectionOutbound:
		// Valid directions
	default:
		return nil, fmt.Errorf("invalid call direction")
	}
	
	now := clock.Now()
	return &Call{
		ID:          uuid.New(),
		FromNumber:  fromNumber,
		ToNumber:    toNumber,
		Status:      StatusPending,
		Direction:   direction,
		BuyerID:     buyerID,
		StartTime:   now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

func (c *Call) UpdateStatus(status Status) {
	c.Status = status
	c.UpdatedAt = clock.Now()
}

func (c *Call) Complete(duration int, cost float64) error {
	// Validate duration
	if err := validation.ValidateDuration(duration); err != nil {
		return fmt.Errorf("invalid duration: %w", err)
	}
	
	// Validate cost
	if err := validation.ValidateAmount(cost, "cost"); err != nil {
		return fmt.Errorf("invalid cost: %w", err)
	}
	
	now := clock.Now()
	c.Status = StatusCompleted
	c.EndTime = &now
	c.Duration = &duration
	c.Cost = &cost
	c.UpdatedAt = now
	return nil
}

func (c *Call) Fail() {
	c.Status = StatusFailed
	c.UpdatedAt = clock.Now()
}