package dnc

import (
	"fmt"
	"net/url"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/google/uuid"
)

// ProviderType represents the type of DNC provider
type ProviderType string

const (
	ProviderTypeFederal  ProviderType = "federal"
	ProviderTypeState    ProviderType = "state"
	ProviderTypeInternal ProviderType = "internal"
	ProviderTypeCustom   ProviderType = "custom"
)

// ProviderStatus represents the operational status of a provider
type ProviderStatus string

const (
	ProviderStatusActive   ProviderStatus = "active"
	ProviderStatusInactive ProviderStatus = "inactive"
	ProviderStatusError    ProviderStatus = "error"
	ProviderStatusSyncing  ProviderStatus = "syncing"
)

// AuthType represents the authentication method for the provider
type AuthType string

const (
	AuthTypeNone   AuthType = "none"
	AuthTypeAPIKey AuthType = "api_key"
	AuthTypeOAuth  AuthType = "oauth"
	AuthTypeBasic  AuthType = "basic"
)

// DNCProvider represents a source of DNC data
// This entity manages external providers of Do Not Call data
type DNCProvider struct {
	ID               uuid.UUID            `json:"id"`
	Name             string               `json:"name"`
	Type             ProviderType         `json:"type"`
	BaseURL          string               `json:"base_url"`
	AuthType         AuthType             `json:"auth_type"`
	APIKey           *string              `json:"-"` // Sensitive, not serialized
	UpdateFrequency  time.Duration        `json:"update_frequency"`
	LastSyncAt       *time.Time           `json:"last_sync_at,omitempty"`
	NextSyncAt       *time.Time           `json:"next_sync_at,omitempty"`
	Status           ProviderStatus       `json:"status"`
	
	// Configuration
	Enabled          bool                 `json:"enabled"`
	Priority         int                  `json:"priority"` // Lower number = higher priority
	RetryAttempts    int                  `json:"retry_attempts"`
	TimeoutSeconds   int                  `json:"timeout_seconds"`
	RateLimitPerMin  int                  `json:"rate_limit_per_min"`
	
	// Sync statistics
	LastSyncDuration *time.Duration       `json:"last_sync_duration,omitempty"`
	LastSyncRecords  *int                 `json:"last_sync_records,omitempty"`
	LastError        *string              `json:"last_error,omitempty"`
	ErrorCount       int                  `json:"error_count"`
	SuccessCount     int                  `json:"success_count"`
	
	// Metadata
	Config           map[string]string    `json:"config,omitempty"`
	
	// Audit fields
	CreatedAt        time.Time            `json:"created_at"`
	UpdatedAt        time.Time            `json:"updated_at"`
	CreatedBy        uuid.UUID            `json:"created_by"`
	UpdatedBy        *uuid.UUID           `json:"updated_by,omitempty"`
}

// NewDNCProvider creates a new DNC provider with validation
// All business rules and validation are enforced in the constructor
func NewDNCProvider(name string, providerType ProviderType, baseURL string, createdBy uuid.UUID) (*DNCProvider, error) {
	// Validate name
	if name == "" {
		return nil, errors.NewValidationError("INVALID_NAME", "provider name cannot be empty")
	}

	// Validate provider type
	if err := validateProviderType(providerType); err != nil {
		return nil, err
	}

	// Validate and normalize base URL
	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return nil, errors.NewValidationError("INVALID_URL", "invalid base URL format").WithCause(err)
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return nil, errors.NewValidationError("INVALID_URL", "base URL must use http or https scheme")
	}

	// Validate createdBy
	if createdBy == uuid.Nil {
		return nil, errors.NewValidationError("INVALID_USER", "created by user ID cannot be empty")
	}

	now := time.Now().UTC()
	return &DNCProvider{
		ID:              uuid.New(),
		Name:            name,
		Type:            providerType,
		BaseURL:         parsedURL.String(),
		AuthType:        AuthTypeNone,
		UpdateFrequency: 24 * time.Hour, // Default to daily
		Status:          ProviderStatusInactive,
		Enabled:         false,
		Priority:        100, // Default priority
		RetryAttempts:   3,
		TimeoutSeconds:  30,
		RateLimitPerMin: 60,
		ErrorCount:      0,
		SuccessCount:    0,
		Config:          make(map[string]string),
		CreatedAt:       now,
		UpdatedAt:       now,
		CreatedBy:       createdBy,
	}, nil
}

// SetAuthentication sets the authentication details for the provider
func (p *DNCProvider) SetAuthentication(authType AuthType, credentials string) error {
	if err := validateAuthType(authType); err != nil {
		return err
	}

	if authType != AuthTypeNone && credentials == "" {
		return errors.NewValidationError("INVALID_CREDENTIALS", "credentials cannot be empty for authenticated providers")
	}

	p.AuthType = authType
	if authType != AuthTypeNone {
		p.APIKey = &credentials
	} else {
		p.APIKey = nil
	}
	
	p.UpdatedAt = time.Now().UTC()
	return nil
}

// SetUpdateFrequency sets how often the provider should be synced
func (p *DNCProvider) SetUpdateFrequency(frequency time.Duration) error {
	if frequency < time.Minute {
		return errors.NewValidationError("INVALID_FREQUENCY", "update frequency must be at least 1 minute")
	}

	if frequency > 30*24*time.Hour {
		return errors.NewValidationError("INVALID_FREQUENCY", "update frequency cannot exceed 30 days")
	}

	p.UpdateFrequency = frequency
	p.UpdatedAt = time.Now().UTC()
	
	// Update next sync time if we have a last sync
	if p.LastSyncAt != nil {
		nextSync := p.LastSyncAt.Add(frequency)
		p.NextSyncAt = &nextSync
	}
	
	return nil
}

// Enable activates the provider
func (p *DNCProvider) Enable() error {
	if p.Status == ProviderStatusError {
		return errors.NewBusinessError("PROVIDER_IN_ERROR", "cannot enable provider in error state")
	}

	p.Enabled = true
	p.Status = ProviderStatusActive
	p.UpdatedAt = time.Now().UTC()
	
	// Set next sync time if not already set
	if p.NextSyncAt == nil {
		now := time.Now().UTC()
		p.NextSyncAt = &now
	}
	
	return nil
}

// Disable deactivates the provider
func (p *DNCProvider) Disable() {
	p.Enabled = false
	p.Status = ProviderStatusInactive
	p.UpdatedAt = time.Now().UTC()
}

// RecordSyncStart marks the beginning of a sync operation
func (p *DNCProvider) RecordSyncStart() error {
	if !p.Enabled {
		return errors.NewBusinessError("PROVIDER_DISABLED", "cannot sync disabled provider")
	}

	p.Status = ProviderStatusSyncing
	p.UpdatedAt = time.Now().UTC()
	return nil
}

// RecordSyncSuccess records a successful sync operation
func (p *DNCProvider) RecordSyncSuccess(duration time.Duration, recordCount int) {
	now := time.Now().UTC()
	p.LastSyncAt = &now
	p.LastSyncDuration = &duration
	p.LastSyncRecords = &recordCount
	p.Status = ProviderStatusActive
	p.SuccessCount++
	p.ErrorCount = 0 // Reset error count on success
	p.LastError = nil
	
	// Calculate next sync time
	nextSync := now.Add(p.UpdateFrequency)
	p.NextSyncAt = &nextSync
	
	p.UpdatedAt = now
}

// RecordSyncError records a failed sync operation
func (p *DNCProvider) RecordSyncError(err error) {
	p.Status = ProviderStatusError
	p.ErrorCount++
	errorMsg := err.Error()
	p.LastError = &errorMsg
	
	// Calculate exponential backoff for next sync (max 1 hour)
	backoffMinutes := p.ErrorCount * p.ErrorCount * 5
	if backoffMinutes > 60 {
		backoffMinutes = 60
	}
	
	nextSync := time.Now().UTC().Add(time.Duration(backoffMinutes) * time.Minute)
	p.NextSyncAt = &nextSync
	
	p.UpdatedAt = time.Now().UTC()
}

// NeedsSync checks if the provider needs to be synchronized
func (p *DNCProvider) NeedsSync() bool {
	if !p.Enabled {
		return false
	}

	if p.Status == ProviderStatusSyncing {
		return false // Already syncing
	}

	// Never synced before
	if p.LastSyncAt == nil {
		return true
	}

	// Check if next sync time has passed
	if p.NextSyncAt != nil && time.Now().After(*p.NextSyncAt) {
		return true
	}

	// Fallback: check if update frequency has elapsed
	return time.Since(*p.LastSyncAt) >= p.UpdateFrequency
}

// GetSyncInterval returns the effective sync interval considering errors
func (p *DNCProvider) GetSyncInterval() time.Duration {
	if p.ErrorCount > 0 {
		// Use exponential backoff for errors
		backoffMinutes := p.ErrorCount * p.ErrorCount * 5
		if backoffMinutes > 60 {
			backoffMinutes = 60
		}
		return time.Duration(backoffMinutes) * time.Minute
	}

	return p.UpdateFrequency
}

// GetHealthStatus returns the health status of the provider
func (p *DNCProvider) GetHealthStatus() map[string]interface{} {
	status := map[string]interface{}{
		"id":           p.ID,
		"name":         p.Name,
		"type":         string(p.Type),
		"status":       string(p.Status),
		"enabled":      p.Enabled,
		"error_count":  p.ErrorCount,
		"success_rate": p.GetSuccessRate(),
	}

	if p.LastSyncAt != nil {
		status["last_sync_at"] = *p.LastSyncAt
		status["time_since_last_sync"] = time.Since(*p.LastSyncAt).String()
	}

	if p.NextSyncAt != nil {
		status["next_sync_at"] = *p.NextSyncAt
		status["time_until_next_sync"] = time.Until(*p.NextSyncAt).String()
	}

	if p.LastError != nil {
		status["last_error"] = *p.LastError
	}

	return status
}

// GetSuccessRate returns the success rate as a percentage
func (p *DNCProvider) GetSuccessRate() float64 {
	total := p.SuccessCount + p.ErrorCount
	if total == 0 {
		return 0.0
	}
	return float64(p.SuccessCount) / float64(total) * 100
}

// SetConfig sets a configuration value
func (p *DNCProvider) SetConfig(key, value string) {
	if p.Config == nil {
		p.Config = make(map[string]string)
	}
	p.Config[key] = value
	p.UpdatedAt = time.Now().UTC()
}

// GetConfig retrieves a configuration value
func (p *DNCProvider) GetConfig(key string) (string, bool) {
	if p.Config == nil {
		return "", false
	}
	value, exists := p.Config[key]
	return value, exists
}

// validateProviderType validates the provider type
func validateProviderType(providerType ProviderType) error {
	switch providerType {
	case ProviderTypeFederal, ProviderTypeState, ProviderTypeInternal, ProviderTypeCustom:
		return nil
	default:
		return errors.NewValidationError("INVALID_PROVIDER_TYPE", 
			fmt.Sprintf("invalid provider type: %s", providerType))
	}
}

// validateAuthType validates the authentication type
func validateAuthType(authType AuthType) error {
	switch authType {
	case AuthTypeNone, AuthTypeAPIKey, AuthTypeOAuth, AuthTypeBasic:
		return nil
	default:
		return errors.NewValidationError("INVALID_AUTH_TYPE", 
			fmt.Sprintf("invalid authentication type: %s", authType))
	}
}

// String methods for string conversion
func (t ProviderType) String() string {
	return string(t)
}

func (s ProviderStatus) String() string {
	return string(s)
}

func (a AuthType) String() string {
	return string(a)
}

// IsRegulatory checks if this provider is a regulatory/government source
func (p *DNCProvider) IsRegulatory() bool {
	return p.Type == ProviderTypeFederal || p.Type == ProviderTypeState
}

// GetListSource returns the corresponding ListSource for this provider
func (p *DNCProvider) GetListSource() values.ListSource {
	switch p.Type {
	case ProviderTypeFederal:
		return values.FederalListSource()
	case ProviderTypeState:
		return values.StateListSource()
	case ProviderTypeInternal:
		return values.InternalListSource()
	default:
		return values.CustomListSource()
	}
}

// RequiresCompliance checks if this provider requires compliance tracking
func (p *DNCProvider) RequiresCompliance() bool {
	return p.IsRegulatory()
}

// GetDefaultSuppressReason returns the default suppress reason for this provider type
func (p *DNCProvider) GetDefaultSuppressReason() values.SuppressReason {
	switch p.Type {
	case ProviderTypeFederal:
		return values.FederalDNCSuppressReason()
	case ProviderTypeState:
		return values.StateDNCSuppressReason()
	default:
		return values.CompanyPolicySuppressReason()
	}
}

// IsHighPriority checks if this provider is considered high priority
func (p *DNCProvider) IsHighPriority() bool {
	return p.Priority <= 10 // Lower numbers = higher priority
}

// GetComplianceCode returns the compliance code for this provider
func (p *DNCProvider) GetComplianceCode() string {
	switch p.Type {
	case ProviderTypeFederal:
		return "FEDERAL_DNC"
	case ProviderTypeState:
		return "STATE_DNC"
	default:
		return ""
	}
}