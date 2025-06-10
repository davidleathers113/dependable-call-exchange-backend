package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/call"
)

// CallRepository defines the interface for call data persistence
type CallRepository interface {
	// Create inserts a new call into the database
	Create(ctx context.Context, call *call.Call) error
	
	// GetByID retrieves a call by its ID
	GetByID(ctx context.Context, id uuid.UUID) (*call.Call, error)
	
	// Update updates an existing call
	Update(ctx context.Context, call *call.Call) error
	
	// UpdateWithStatusCheck updates a call only if it has the expected status
	UpdateWithStatusCheck(ctx context.Context, c *call.Call, expectedStatus call.Status) error
	
	// Delete removes a call from the database
	Delete(ctx context.Context, id uuid.UUID) error
	
	// List returns a list of calls based on filter criteria
	List(ctx context.Context, filter CallFilter) ([]*call.Call, error)
	
	// CountByStatus returns the count of calls grouped by status
	CountByStatus(ctx context.Context) (map[call.Status]int, error)
	
	// GetActiveCallsForSeller returns active calls owned by a seller
	GetActiveCallsForSeller(ctx context.Context, sellerID uuid.UUID) ([]*call.Call, error)
	
	// GetActiveCallsForBuyer returns active calls assigned to a buyer
	GetActiveCallsForBuyer(ctx context.Context, buyerID uuid.UUID) ([]*call.Call, error)
	
	// GetPendingSellerCalls returns pending calls from sellers awaiting routing
	GetPendingSellerCalls(ctx context.Context, limit int) ([]*call.Call, error)
}

// CallFilter defines filtering options for listing calls
type CallFilter struct {
	// Status filters
	Status *call.Status
	
	// ID filters
	BuyerID  *uuid.UUID
	SellerID *uuid.UUID
	
	// Time range filters
	StartTimeFrom *time.Time
	StartTimeTo   *time.Time
	
	// Pagination
	Limit  int
	Offset int
	
	// Sorting
	OrderBy string // e.g., "created_at DESC"
}
