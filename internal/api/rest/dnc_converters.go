package rest

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/dnc"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	dncService "github.com/davidleathers/dependable-call-exchange-backend/internal/service/dnc"
	"github.com/google/uuid"
)

// DNC DTO Converters - Domain to DTO and DTO to Domain conversions
// Includes privacy protection and validation for sensitive data

// ToEntity methods for requests (DTO -> Domain)

// ToEntity converts CreateDNCEntryRequest to domain entity
func (r CreateDNCEntryRequest) ToEntity(addedBy uuid.UUID) (*dnc.DNCEntry, error) {
	entry, err := dnc.NewDNCEntry(r.PhoneNumber, r.ListSource, r.SuppressReason, addedBy)
	if err != nil {
		return nil, errors.NewValidationError("INVALID_ENTRY", "failed to create DNC entry").WithCause(err)
	}

	// Set optional fields
	if r.ExpiresAt != nil {
		if err := entry.SetExpiration(*r.ExpiresAt); err != nil {
			return nil, err
		}
	}

	if r.SourceReference != nil {
		entry.SetSourceReference(*r.SourceReference)
	}

	if r.Notes != nil {
		if err := entry.AddNote(*r.Notes, addedBy); err != nil {
			return nil, err
		}
	}

	// Set metadata
	for key, value := range r.Metadata {
		entry.SetMetadata(key, value)
	}

	return entry, nil
}

// ToEntity converts CreateDNCProviderRequest to domain entity
func (r CreateDNCProviderRequest) ToEntity(createdBy uuid.UUID) (*dnc.DNCProvider, error) {
	provider, err := dnc.NewDNCProvider(r.Name, dnc.ProviderType(r.Type), r.BaseURL, createdBy)
	if err != nil {
		return nil, errors.NewValidationError("INVALID_PROVIDER", "failed to create DNC provider").WithCause(err)
	}

	// Set authentication
	authType := dnc.AuthType(r.AuthType)
	credentials := ""
	if r.APIKey != nil {
		credentials = *r.APIKey
	}
	if err := provider.SetAuthentication(authType, credentials); err != nil {
		return nil, err
	}

	// Set optional fields with defaults
	if r.UpdateFrequency != nil {
		frequency := time.Duration(*r.UpdateFrequency) * time.Hour
		if err := provider.SetUpdateFrequency(frequency); err != nil {
			return nil, err
		}
	}

	if r.Priority != nil {
		provider.Priority = *r.Priority
	}

	if r.RetryAttempts != nil {
		provider.RetryAttempts = *r.RetryAttempts
	}

	if r.TimeoutSeconds != nil {
		provider.TimeoutSeconds = *r.TimeoutSeconds
	}

	if r.RateLimitPerMin != nil {
		provider.RateLimitPerMin = *r.RateLimitPerMin
	}

	// Set configuration
	for key, value := range r.Config {
		provider.SetConfig(key, value)
	}

	// Enable if requested
	if r.Enabled {
		if err := provider.Enable(); err != nil {
			return nil, err
		}
	}

	return provider, nil
}

// UpdateEntity applies UpdateDNCEntryRequest to existing domain entity
func (r UpdateDNCEntryRequest) UpdateEntity(entry *dnc.DNCEntry, updatedBy uuid.UUID) error {
	if r.ExpiresAt != nil {
		if err := entry.SetExpiration(*r.ExpiresAt); err != nil {
			return err
		}
	}

	if r.Notes != nil {
		if err := entry.AddNote(*r.Notes, updatedBy); err != nil {
			return err
		}
	}

	if r.SourceReference != nil {
		entry.SetSourceReference(*r.SourceReference)
	}

	// Update metadata
	for key, value := range r.Metadata {
		entry.SetMetadata(key, value)
	}

	return nil
}

// UpdateEntity applies UpdateDNCProviderRequest to existing domain entity
func (r UpdateDNCProviderRequest) UpdateEntity(provider *dnc.DNCProvider, updatedBy uuid.UUID) error {
	if r.Name != nil {
		provider.Name = *r.Name
	}

	if r.BaseURL != nil {
		provider.BaseURL = *r.BaseURL
	}

	if r.AuthType != nil && r.APIKey != nil {
		authType := dnc.AuthType(*r.AuthType)
		if err := provider.SetAuthentication(authType, *r.APIKey); err != nil {
			return err
		}
	}

	if r.UpdateFrequency != nil {
		frequency := time.Duration(*r.UpdateFrequency) * time.Hour
		if err := provider.SetUpdateFrequency(frequency); err != nil {
			return err
		}
	}

	if r.Priority != nil {
		provider.Priority = *r.Priority
	}

	if r.RetryAttempts != nil {
		provider.RetryAttempts = *r.RetryAttempts
	}

	if r.TimeoutSeconds != nil {
		provider.TimeoutSeconds = *r.TimeoutSeconds
	}

	if r.RateLimitPerMin != nil {
		provider.RateLimitPerMin = *r.RateLimitPerMin
	}

	// Update configuration
	for key, value := range r.Config {
		provider.SetConfig(key, value)
	}

	if r.Enabled != nil {
		if *r.Enabled {
			if err := provider.Enable(); err != nil {
				return err
			}
		} else {
			provider.Disable()
		}
	}

	provider.UpdatedBy = &updatedBy
	provider.UpdatedAt = time.Now().UTC()

	return nil
}

// FromEntity methods for responses (Domain -> DTO)

// FromEntity converts DNC entry domain entity to response DTO with privacy protection
func (r *DNCEntryResponse) FromEntity(entry *dnc.DNCEntry, maskPhoneNumber bool) {
	r.ID = entry.ID
	r.ListSource = entry.ListSource.String()
	r.SuppressReason = entry.SuppressReason.String()
	r.AddedAt = entry.AddedAt
	r.ExpiresAt = entry.ExpiresAt
	r.IsActive = entry.IsActive()
	r.IsExpired = entry.IsExpired()
	r.IsTemporary = entry.IsTemporary()
	r.IsPermanent = entry.IsPermanent()
	r.CanCall = entry.CanCall()
	r.Priority = entry.GetPriority()
	r.ComplianceCode = entry.GetComplianceCode()
	r.RequiresDocs = entry.RequiresDocumentation()
	r.RetentionDays = entry.GetRetentionDays()
	r.SourceReference = entry.SourceReference
	r.Notes = entry.Notes
	r.UpdatedAt = entry.UpdatedAt

	// Privacy protection for phone number
	phoneStr := entry.PhoneNumber.String()
	if maskPhoneNumber {
		r.PhoneNumber = maskPhoneForDisplay(phoneStr)
	} else {
		r.PhoneNumber = phoneStr
	}

	// Always provide hash for identification
	r.PhoneHash = hashPhoneNumber(phoneStr)

	// Time until expiry (human readable)
	if duration := entry.TimeUntilExpiration(); duration != nil {
		durationStr := formatDuration(*duration)
		r.TimeUntilExpiry = &durationStr
	}

	// Convert metadata to interface{} map for JSON compatibility
	r.Metadata = make(map[string]interface{})
	for k, v := range entry.Metadata {
		// Filter out sensitive metadata keys
		if !isSensitiveMetadataKey(k) {
			r.Metadata[k] = v
		}
	}
}

// FromEntity converts DNC provider domain entity to response DTO
func (r *ProviderStatusResponse) FromEntity(provider *dnc.DNCProvider) {
	r.ID = provider.ID
	r.Name = provider.Name
	r.Type = string(provider.Type)
	r.Status = string(provider.Status)
	r.Enabled = provider.Enabled
	r.IsRegulatory = provider.IsRegulatory()
	r.BaseURL = provider.BaseURL
	r.AuthType = string(provider.AuthType)
	r.Priority = provider.Priority
	r.UpdateFrequency = provider.UpdateFrequency.String()
	r.LastSyncAt = provider.LastSyncAt
	r.NextSyncAt = provider.NextSyncAt
	r.SuccessRate = provider.GetSuccessRate()
	r.ErrorCount = provider.ErrorCount
	r.SuccessCount = provider.SuccessCount
	r.LastError = provider.LastError
	r.NeedsSync = provider.NeedsSync()
	r.ComplianceCode = provider.GetComplianceCode()
	r.CreatedAt = provider.CreatedAt
	r.UpdatedAt = provider.UpdatedAt

	// Format duration if available
	if provider.LastSyncDuration != nil {
		duration := provider.LastSyncDuration.String()
		r.LastSyncDuration = &duration
	}

	r.LastSyncRecords = provider.LastSyncRecords

	// Determine health status
	r.HealthStatus = determineHealthStatus(provider)

	// Convert config to interface{} map (filtered for sensitive data)
	r.Metadata = make(map[string]interface{})
	for k, v := range provider.Config {
		if !isSensitiveConfigKey(k) {
			r.Metadata[k] = v
		}
	}
}

// FromEntity converts DNC check result domain entity to response DTO
func (r *DNCCheckResponse) FromEntity(result *dnc.DNCCheckResult) {
	r.PhoneNumber = result.PhoneNumber.String()
	r.IsBlocked = result.IsBlocked
	r.CanCall = result.CanCall()
	r.CheckedAt = result.CheckedAt
	r.ComplianceLevel = result.ComplianceLevel
	r.RiskScore = result.RiskScore
	r.TTLSeconds = int(result.TTL.Seconds())
	r.CheckDuration = result.CheckDuration.String()
	r.Recommendation = result.GetComplianceRecommendation()
	r.ComplianceCodes = result.GetComplianceCodes()
	r.HighestSeverity = result.GetHighestSeverity()

	// Convert sources to strings
	r.SourcesChecked = make([]string, len(result.Sources))
	for i, source := range result.Sources {
		r.SourcesChecked[i] = source.String()
	}
	r.SourcesCount = result.SourcesCount

	// Convert blocking reasons
	r.BlockingReasons = make([]DNCBlockReasonResponse, len(result.Reasons))
	for i, reason := range result.Reasons {
		r.BlockingReasons[i] = DNCBlockReasonResponse{
			Source:         reason.Source.String(),
			Reason:         reason.Reason.String(),
			Description:    reason.Description,
			Provider:       reason.ProviderName,
			ProviderID:     reason.ProviderID,
			Severity:       reason.Severity,
			ComplianceCode: reason.ComplianceCode,
			ExpiresAt:      reason.ExpiresAt,
			IsPermanent:    reason.ExpiresAt == nil,
			IsRegulatory:   reason.Reason.IsRegulatory(),
		}
	}

	// Convert metadata
	r.Metadata = make(map[string]interface{})
	for k, v := range result.Metadata {
		r.Metadata[k] = v
	}
}

// Validation helpers

// ValidateCreateDNCEntry performs additional validation beyond struct tags
func ValidateCreateDNCEntry(req CreateDNCEntryRequest) error {
	// Validate list source
	if err := values.ValidateListSource(req.ListSource); err != nil {
		return err
	}

	// Validate that expiration is reasonable (not too far in future)
	if req.ExpiresAt != nil {
		maxExpiry := time.Now().AddDate(10, 0, 0) // 10 years max
		if req.ExpiresAt.After(maxExpiry) {
			return errors.NewValidationError("INVALID_EXPIRATION", 
				"expiration date cannot be more than 10 years in the future")
		}
	}

	return nil
}

// ValidateCreateDNCProvider performs additional validation beyond struct tags
func ValidateCreateDNCProvider(req CreateDNCProviderRequest) error {
	// Validate provider type specific requirements
	switch req.Type {
	case "federal", "state":
		if req.AuthType == "none" {
			return errors.NewValidationError("INVALID_CONFIG", 
				"regulatory providers must have authentication configured")
		}
	case "internal", "custom":
		// Allow any configuration for internal/custom providers
	default:
		return errors.NewValidationError("INVALID_PROVIDER_TYPE", 
			fmt.Sprintf("unsupported provider type: %s", req.Type))
	}

	return nil
}

// Privacy and security helpers

// maskPhoneForDisplay masks phone number for display while preserving format
func maskPhoneForDisplay(phoneNumber string) string {
	if len(phoneNumber) < 4 {
		return "****"
	}
	
	// Show last 4 digits: +1234567890 -> +123****7890
	if strings.HasPrefix(phoneNumber, "+") && len(phoneNumber) > 8 {
		prefix := phoneNumber[:4]
		suffix := phoneNumber[len(phoneNumber)-4:]
		stars := strings.Repeat("*", len(phoneNumber)-8)
		return prefix + stars + suffix
	}
	
	// Fallback masking
	return phoneNumber[:2] + strings.Repeat("*", len(phoneNumber)-4) + phoneNumber[len(phoneNumber)-2:]
}

// hashPhoneNumber creates a consistent hash for phone number identification
func hashPhoneNumber(phoneNumber string) string {
	hash := sha256.Sum256([]byte(phoneNumber))
	return fmt.Sprintf("%x", hash)[:16] // Return first 16 chars of hex
}

// isSensitiveMetadataKey checks if a metadata key contains sensitive information
func isSensitiveMetadataKey(key string) bool {
	sensitiveKeys := []string{
		"password", "secret", "key", "token", "credential",
		"ssn", "social", "dob", "birth", "address", "email",
		"internal_id", "customer_id", "account_number",
	}
	
	keyLower := strings.ToLower(key)
	for _, sensitive := range sensitiveKeys {
		if strings.Contains(keyLower, sensitive) {
			return true
		}
	}
	return false
}

// isSensitiveConfigKey checks if a config key contains sensitive information
func isSensitiveConfigKey(key string) bool {
	sensitiveKeys := []string{
		"password", "secret", "key", "token", "credential",
		"api_key", "auth", "cert", "private",
	}
	
	keyLower := strings.ToLower(key)
	for _, sensitive := range sensitiveKeys {
		if strings.Contains(keyLower, sensitive) {
			return true
		}
	}
	return false
}

// formatDuration formats a duration in a human-readable way
func formatDuration(d time.Duration) string {
	if d < 0 {
		return "expired"
	}
	
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	
	if days > 0 {
		if days == 1 {
			return "1 day"
		}
		return fmt.Sprintf("%d days", days)
	}
	
	if hours > 0 {
		if hours == 1 {
			return "1 hour"
		}
		return fmt.Sprintf("%d hours", hours)
	}
	
	if minutes > 0 {
		if minutes == 1 {
			return "1 minute"
		}
		return fmt.Sprintf("%d minutes", minutes)
	}
	
	return "less than a minute"
}

// determineHealthStatus determines the health status of a provider
func determineHealthStatus(provider *dnc.DNCProvider) string {
	if !provider.Enabled {
		return "disabled"
	}
	
	switch provider.Status {
	case dnc.ProviderStatusActive:
		if provider.GetSuccessRate() >= 95.0 {
			return "healthy"
		} else if provider.GetSuccessRate() >= 80.0 {
			return "degraded"
		}
		return "unhealthy"
	case dnc.ProviderStatusSyncing:
		return "syncing"
	case dnc.ProviderStatusError:
		return "error"
	case dnc.ProviderStatusInactive:
		return "inactive"
	default:
		return "unknown"
	}
}

// Error handling helpers

// ConvertDomainError converts domain errors to appropriate HTTP responses
func ConvertDomainError(err error) error {
	if err == nil {
		return nil
	}
	
	// If it's already an API error, return as-is
	if _, ok := err.(*errors.AppError); ok {
		return err
	}
	
	// Convert validation errors
	if strings.Contains(err.Error(), "validation") {
		return errors.NewValidationError("VALIDATION_FAILED", err.Error()).WithCause(err)
	}
	
	// Convert business rule violations
	if strings.Contains(err.Error(), "business") {
		return errors.NewBusinessError("BUSINESS_RULE_VIOLATION", err.Error()).WithCause(err)
	}
	
	// Default to internal error
	return errors.NewInternalError("Internal error occurred").WithCause(err)
}

// Service Request Converters - Convert DTOs to service layer requests

// ToAddSuppressionRequest converts CreateDNCEntryRequest to service request
func (r CreateDNCEntryRequest) ToAddSuppressionRequest(addedBy uuid.UUID) (*dncService.AddSuppressionRequest, error) {
	phoneNumber, err := values.NewPhoneNumber(r.PhoneNumber)
	if err != nil {
		return nil, errors.NewValidationError("INVALID_PHONE_NUMBER", "phone number must be in E.164 format").WithCause(err)
	}

	req := &dncService.AddSuppressionRequest{
		PhoneNumber:     phoneNumber,
		ListSource:      r.ListSource,
		SuppressReason:  r.SuppressReason,
		AddedBy:         addedBy,
		SourceReference: r.SourceReference,
		Notes:           r.Notes,
		Metadata:        r.Metadata,
	}

	if r.ExpiresAt != nil {
		req.ExpiresAt = r.ExpiresAt
	}

	return req, nil
}

// ToUpdateSuppressionRequest converts UpdateDNCEntryRequest to service request
func (r UpdateDNCEntryRequest) ToUpdateSuppressionRequest(entryID uuid.UUID, updatedBy uuid.UUID) (*dncService.UpdateSuppressionRequest, error) {
	req := &dncService.UpdateSuppressionRequest{
		EntryID:         entryID,
		UpdatedBy:       updatedBy,
		ExpiresAt:       r.ExpiresAt,
		Notes:           r.Notes,
		SourceReference: r.SourceReference,
		Metadata:        r.Metadata,
	}

	return req, nil
}

// ToSearchCriteria converts ListDNCParams to search criteria
func (p ListDNCParams) ToSearchCriteria() (*dncService.SearchCriteria, error) {
	criteria := &dncService.SearchCriteria{
		Page:      p.Page,
		Limit:     p.Limit,
		SortBy:    p.SortBy,
		SortOrder: p.SortOrder,
	}

	if p.Source != nil {
		criteria.ListSource = p.Source
	}

	if p.Phone != nil {
		criteria.PhonePattern = p.Phone
	}

	if p.Status != nil {
		criteria.Status = p.Status
	}

	return criteria, nil
}

// ToReportCriteria converts ComplianceReportParams to report criteria
func (p ComplianceReportParams) ToReportCriteria() (*dncService.ReportCriteria, error) {
	criteria := &dncService.ReportCriteria{
		Format:            p.Format,
		IncludeViolations: p.IncludeViolations,
		IncludeStats:      p.IncludeStats,
	}

	if p.StartDate != nil {
		criteria.StartDate = p.StartDate
	}

	if p.EndDate != nil {
		criteria.EndDate = p.EndDate
	}

	return criteria, nil
}