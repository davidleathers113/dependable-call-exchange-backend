package consent

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Repository defines the interface for consent persistence
type Repository interface {
	// Save creates or updates a consent aggregate
	Save(ctx context.Context, consent *ConsentAggregate) error

	// GetByID retrieves a consent by its ID
	GetByID(ctx context.Context, id uuid.UUID) (*ConsentAggregate, error)

	// GetByConsumerAndType retrieves consent for a consumer by type
	GetByConsumerAndType(ctx context.Context, consumerID uuid.UUID, consentType Type) (*ConsentAggregate, error)

	// GetByConsumerAndBusiness retrieves consents for a consumer-business pair
	GetByConsumerAndBusiness(ctx context.Context, consumerID, businessID uuid.UUID) ([]*ConsentAggregate, error)

	// FindActiveConsent finds active consent for a specific channel
	FindActiveConsent(ctx context.Context, consumerID, businessID uuid.UUID, channel Channel) (*ConsentAggregate, error)

	// FindByPhoneNumber finds consents by phone number
	FindByPhoneNumber(ctx context.Context, phoneNumber string, businessID uuid.UUID) ([]*ConsentAggregate, error)

	// ListExpired lists consents that have expired before a given time
	ListExpired(ctx context.Context, before time.Time) ([]*ConsentAggregate, error)

	// Delete removes a consent aggregate (for GDPR compliance)
	Delete(ctx context.Context, id uuid.UUID) error
}

// ConsentFilter defines filters for querying consents
type ConsentFilter struct {
	ConsumerID    *uuid.UUID
	BusinessID    *uuid.UUID
	PhoneNumber   *string
	Email         *string
	Status        *ConsentStatus
	Channels      []Channel
	Purpose       *Purpose
	CreatedAfter  *time.Time
	CreatedBefore *time.Time
	ExpiringBefore *time.Time
	Limit         int
	Offset        int
}

// QueryRepository defines additional query methods
type QueryRepository interface {
	// Find searches for consents based on filter criteria
	Find(ctx context.Context, filter ConsentFilter) ([]*ConsentAggregate, error)

	// FindActiveByConsumer finds all active consents for a consumer
	FindActiveByConsumer(ctx context.Context, consumerID uuid.UUID) ([]*ConsentAggregate, error)

	// FindByFilters searches consents with advanced filtering
	FindByFilters(ctx context.Context, filters QueryFilters) ([]*ConsentAggregate, error)

	// Count returns the number of consents matching the filter
	Count(ctx context.Context, filter ConsentFilter) (int64, error)

	// GetConsentHistory retrieves all versions of a consent
	GetConsentHistory(ctx context.Context, consentID uuid.UUID) ([]ConsentVersion, error)

	// GetProofs retrieves all proofs for a consent
	GetProofs(ctx context.Context, consentID uuid.UUID) ([]ConsentProof, error)

	// GetMetrics retrieves consent metrics for reporting
	GetMetrics(ctx context.Context, query MetricsQuery) (*ConsentMetrics, error)

	// FindExpiring finds consents expiring within specified days
	FindExpiring(ctx context.Context, days int) ([]*ConsentAggregate, error)
}

// QueryFilters defines advanced filtering options
type QueryFilters struct {
	ConsumerID   *uuid.UUID
	ConsentType  *Type
	Status       *ConsentStatus
	PhoneNumber  *string
	Email        *string
	Channel      *Channel
	Purpose      *Purpose
	CreatedAfter *time.Time
	ExpiringDays *int
	Limit        int
	Offset       int
}

// MetricsQuery defines parameters for consent metrics
type MetricsQuery struct {
	DateRange    DateRange
	ConsentTypes []Type
	Granularity  string // "day", "week", "month"
	GroupBy      []string
}

// DateRange represents a time range for queries
type DateRange struct {
	Start time.Time
	End   time.Time
}

// ConsentMetrics represents aggregated consent data
type ConsentMetrics struct {
	TotalGrants  int64
	TotalRevokes int64
	ActiveCount  int64
	Trends       []MetricTrend
}

// MetricTrend represents trend data over time
type MetricTrend struct {
	Date    time.Time
	Grants  map[Type]int
	Revokes map[Type]int
}

// EventStore defines the interface for storing domain events
type EventStore interface {
	// SaveEvents stores domain events
	SaveEvents(ctx context.Context, events []interface{}) error

	// GetEvents retrieves events for an aggregate
	GetEvents(ctx context.Context, aggregateID uuid.UUID) ([]interface{}, error)

	// GetEventsByType retrieves events of a specific type
	GetEventsByType(ctx context.Context, eventType string, limit int) ([]interface{}, error)
}