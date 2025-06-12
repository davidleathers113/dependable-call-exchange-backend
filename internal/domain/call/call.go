package call

import (
	"fmt"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/google/uuid"
)

type Call struct {
	ID         uuid.UUID          `json:"id"`
	FromNumber values.PhoneNumber `json:"from_number"`
	ToNumber   values.PhoneNumber `json:"to_number"`
	Status     Status             `json:"status"`
	Direction  Direction          `json:"direction"`
	StartTime  time.Time          `json:"start_time"`
	EndTime    *time.Time         `json:"end_time,omitempty"`
	Duration   *int               `json:"duration,omitempty"`
	Cost       *values.Money      `json:"cost,omitempty"`

	// Call routing and ownership
	// IMPORTANT: Understanding buyer/seller relationships:
	// - For marketplace calls (seller → buyer):
	//   - SellerID: The seller who owns/generated this call
	//   - BuyerID: Initially empty, set when a buyer wins the bid
	// - For direct calls:
	//   - BuyerID: The account making the call
	//   - SellerID: May be null for non-marketplace calls
	RouteID  *uuid.UUID `json:"route_id,omitempty"`  // ID of the routing decision
	BuyerID  uuid.UUID  `json:"buyer_id"`            // Buyer who won bid OR originating account
	SellerID *uuid.UUID `json:"seller_id,omitempty"` // Seller who owns this call (marketplace only)

	// Telephony details
	CallSID   string  `json:"call_sid"`
	SessionID *string `json:"session_id,omitempty"`

	// Metadata
	UserAgent *string   `json:"user_agent,omitempty"`
	IPAddress *string   `json:"ip_address,omitempty"`
	Location  *Location `json:"location,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
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

func NewCall(fromNumberStr, toNumberStr string, buyerID uuid.UUID, direction Direction) (*Call, error) {
	// Create phone number value objects
	fromNumber, err := values.NewPhoneNumber(fromNumberStr)
	if err != nil {
		return nil, fmt.Errorf("invalid from number: %w", err)
	}

	toNumber, err := values.NewPhoneNumber(toNumberStr)
	if err != nil {
		return nil, fmt.Errorf("invalid to number: %w", err)
	}

	// Note: buyerID can be nil for marketplace calls awaiting routing

	// Validate direction
	switch direction {
	case DirectionInbound, DirectionOutbound:
		// Valid directions
	default:
		return nil, fmt.Errorf("invalid call direction")
	}

	now := clock.Now()
	return &Call{
		ID:         uuid.New(),
		FromNumber: fromNumber,
		ToNumber:   toNumber,
		Status:     StatusPending,
		Direction:  direction,
		BuyerID:    buyerID,
		StartTime:  now,
		CreatedAt:  now,
		UpdatedAt:  now,
	}, nil
}

func (c *Call) UpdateStatus(status Status) {
	c.Status = status
	c.UpdatedAt = clock.Now()
}

func (c *Call) Complete(duration int, cost values.Money) error {
	// Validate duration (moved from validation package to domain)
	if err := validateDuration(duration); err != nil {
		return fmt.Errorf("invalid duration: %w", err)
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

// NewMarketplaceCall creates a new call from a seller for the marketplace
// These calls do not have a buyer initially - buyers will bid on them
func NewMarketplaceCall(fromNumber, toNumber string, sellerID uuid.UUID, direction Direction) (*Call, error) {
	// Create call without buyer
	c, err := NewCall(fromNumber, toNumber, uuid.Nil, direction)
	if err != nil {
		return nil, err
	}

	// Validate seller ID
	if sellerID == uuid.Nil {
		return nil, fmt.Errorf("seller ID cannot be nil for marketplace call")
	}

	// Set the seller who owns this call
	c.SellerID = &sellerID

	return c, nil
}

// AssignToBuyer assigns a marketplace call to a buyer after winning bid
func (c *Call) AssignToBuyer(buyerID uuid.UUID) error {
	if buyerID == uuid.Nil {
		return fmt.Errorf("buyer ID cannot be nil")
	}

	if c.BuyerID != uuid.Nil {
		return fmt.Errorf("call already assigned to buyer")
	}

	c.BuyerID = buyerID
	c.UpdatedAt = clock.Now()
	return nil
}

// validateDuration validates call duration within the call domain
func validateDuration(duration int) error {
	if duration < 0 {
		return fmt.Errorf("duration cannot be negative")
	}

	// Max call duration of 24 hours (86400 seconds)
	if duration > 86400 {
		return fmt.Errorf("duration too long (max 24 hours)")
	}

	return nil
}
