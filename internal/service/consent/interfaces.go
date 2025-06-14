package consent

import (
	"context"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/consent"
	"github.com/google/uuid"
)

// Service defines the consent management service interface
type Service interface {
	// Consent Management
	GrantConsent(ctx context.Context, req GrantConsentRequest) (*ConsentResponse, error)
	RevokeConsent(ctx context.Context, consumerID uuid.UUID, consentType consent.Type) error
	UpdateConsent(ctx context.Context, req UpdateConsentRequest) (*ConsentResponse, error)
	
	// Consent Queries
	GetConsent(ctx context.Context, consumerID uuid.UUID, consentType consent.Type) (*ConsentResponse, error)
	GetActiveConsents(ctx context.Context, consumerID uuid.UUID) ([]*ConsentResponse, error)
	CheckConsent(ctx context.Context, phoneNumber string, consentType consent.Type) (*ConsentStatus, error)
	
	// Consumer Management
	CreateConsumer(ctx context.Context, req CreateConsumerRequest) (*ConsumerResponse, error)
	GetConsumerByPhone(ctx context.Context, phoneNumber string) (*ConsumerResponse, error)
	GetConsumerByEmail(ctx context.Context, email string) (*ConsumerResponse, error)
	
	// Bulk Operations
	ImportConsents(ctx context.Context, req ImportConsentsRequest) (*ImportResult, error)
	ExportConsents(ctx context.Context, req ExportConsentsRequest) (*ExportResult, error)
	
	// Analytics
	GetConsentMetrics(ctx context.Context, req MetricsRequest) (*ConsentMetrics, error)
}

// Request/Response DTOs

type GrantConsentRequest struct {
	ConsumerID   uuid.UUID             `json:"consumer_id,omitempty"`
	PhoneNumber  string                `json:"phone_number,omitempty"`
	Email        string                `json:"email,omitempty"`
	ConsentType  consent.Type          `json:"consent_type"`
	Channel      consent.Channel       `json:"channel"`
	IPAddress    string                `json:"ip_address,omitempty"`
	UserAgent    string                `json:"user_agent,omitempty"`
	Preferences  map[string]string     `json:"preferences,omitempty"`
	ExpiresAt    *time.Time            `json:"expires_at,omitempty"`
}

type UpdateConsentRequest struct {
	ConsumerID   uuid.UUID             `json:"consumer_id"`
	ConsentType  consent.Type          `json:"consent_type"`
	Preferences  map[string]string     `json:"preferences"`
	ExpiresAt    *time.Time            `json:"expires_at,omitempty"`
}

type CreateConsumerRequest struct {
	PhoneNumber string                 `json:"phone_number"`
	Email       string                 `json:"email,omitempty"`
	FirstName   string                 `json:"first_name,omitempty"`
	LastName    string                 `json:"last_name,omitempty"`
	Attributes  map[string]string      `json:"attributes,omitempty"`
}

type ImportConsentsRequest struct {
	Format      string                 `json:"format"` // csv, json
	Data        []byte                 `json:"data"`
	Source      string                 `json:"source"`
	ValidateOnly bool                  `json:"validate_only"`
}

type ExportConsentsRequest struct {
	Format      string                 `json:"format"` // csv, json
	Filters     ExportFilters          `json:"filters"`
}

type ExportFilters struct {
	ConsentTypes []consent.Type        `json:"consent_types,omitempty"`
	Status       []consent.ConsentStatus      `json:"status,omitempty"`
	StartDate    *time.Time            `json:"start_date,omitempty"`
	EndDate      *time.Time            `json:"end_date,omitempty"`
}

type MetricsRequest struct {
	StartDate    time.Time             `json:"start_date"`
	EndDate      time.Time             `json:"end_date"`
	GroupBy      string                `json:"group_by"` // day, week, month
	ConsentTypes []consent.Type        `json:"consent_types,omitempty"`
}

// Response DTOs

type ConsentResponse struct {
	ID           uuid.UUID             `json:"id"`
	ConsumerID   uuid.UUID             `json:"consumer_id"`
	Type         consent.Type          `json:"type"`
	Status       consent.ConsentStatus        `json:"status"`
	Channel      consent.Channel       `json:"channel"`
	Preferences  map[string]string     `json:"preferences"`
	Version      int                   `json:"version"`
	GrantedAt    time.Time             `json:"granted_at"`
	ExpiresAt    *time.Time            `json:"expires_at,omitempty"`
	RevokedAt    *time.Time            `json:"revoked_at,omitempty"`
	UpdatedAt    time.Time             `json:"updated_at"`
}

type ConsumerResponse struct {
	ID          uuid.UUID              `json:"id"`
	PhoneNumber string                 `json:"phone_number"`
	Email       string                 `json:"email,omitempty"`
	FirstName   string                 `json:"first_name,omitempty"`
	LastName    string                 `json:"last_name,omitempty"`
	Attributes  map[string]string      `json:"attributes"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

type ConsentStatus struct {
	HasConsent   bool                  `json:"has_consent"`
	ConsentID    *uuid.UUID            `json:"consent_id,omitempty"`
	Status       consent.ConsentStatus        `json:"status"`
	GrantedAt    *time.Time            `json:"granted_at,omitempty"`
	ExpiresAt    *time.Time            `json:"expires_at,omitempty"`
	Preferences  map[string]string     `json:"preferences,omitempty"`
}

type ImportResult struct {
	TotalRecords   int                  `json:"total_records"`
	SuccessCount   int                  `json:"success_count"`
	FailureCount   int                  `json:"failure_count"`
	Errors         []ImportError        `json:"errors,omitempty"`
	Duration       time.Duration        `json:"duration"`
}

type ImportError struct {
	Row     int                      `json:"row"`
	Field   string                   `json:"field"`
	Value   string                   `json:"value"`
	Message string                   `json:"message"`
}

type ExportResult struct {
	Format       string                `json:"format"`
	RecordCount  int                   `json:"record_count"`
	FileSize     int64                 `json:"file_size"`
	Data         []byte                `json:"data"`
	GeneratedAt  time.Time             `json:"generated_at"`
}

type ConsentMetrics struct {
	Period       string                           `json:"period"`
	TotalGrants  map[consent.Type]int             `json:"total_grants"`
	TotalRevokes map[consent.Type]int             `json:"total_revokes"`
	ActiveCount  map[consent.Type]int             `json:"active_count"`
	Trends       []ConsentTrend                   `json:"trends"`
}

type ConsentTrend struct {
	Date    time.Time                        `json:"date"`
	Grants  map[consent.Type]int             `json:"grants"`
	Revokes map[consent.Type]int             `json:"revokes"`
}

// Repository interfaces that the service depends on

type ConsentRepository interface {
	consent.Repository
}

type ConsumerRepository interface {
	consent.ConsumerRepository
}

type ConsentQueryRepository interface {
	consent.QueryRepository
}