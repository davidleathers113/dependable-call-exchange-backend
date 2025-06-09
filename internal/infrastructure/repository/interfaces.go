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
	
	// Delete removes a call from the database
	Delete(ctx context.Context, id uuid.UUID) error
	
	// List returns a list of calls based on filter criteria
	List(ctx context.Context, filter CallFilter) ([]*call.Call, error)
	
	// CountByStatus returns the count of calls grouped by status
	CountByStatus(ctx context.Context) (map[call.Status]int, error)
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
