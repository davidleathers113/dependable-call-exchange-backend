package consent

import (
	"fmt"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/consent"
	"github.com/google/uuid"
)

// Mapper functions for converting between domain objects and DTOs

// MapConsentToResponse converts a domain consent to a response DTO
func MapConsentToResponse(c *consent.ConsentAggregate) *ConsentResponse {
	if c == nil {
		return nil
	}

	// Get current version - use the current version from the aggregate
	if len(c.Versions) == 0 || c.CurrentVersion == 0 {
		return nil
	}
	current := &c.Versions[c.CurrentVersion-1]

	var grantedAt time.Time
	if current.ConsentedAt != nil {
		grantedAt = *current.ConsentedAt
	}

	return &ConsentResponse{
		ID:          c.ID,
		ConsumerID:  c.ConsumerID,
		Type:        c.Type,
		Status:      current.Status,
		Channel:     current.Channels[0], // Use first channel, should be improved
		Preferences: current.SourceDetails, // Using SourceDetails as preferences
		Version:     current.Version,
		GrantedAt:   grantedAt,
		ExpiresAt:   current.ExpiresAt,
		RevokedAt:   current.RevokedAt,
		UpdatedAt:   c.UpdatedAt,
	}
}

// MapConsentsToResponses converts multiple domain consents to response DTOs
func MapConsentsToResponses(consents []*consent.ConsentAggregate) []*ConsentResponse {
	responses := make([]*ConsentResponse, len(consents))
	for i, c := range consents {
		responses[i] = MapConsentToResponse(c)
	}
	return responses
}

// MapConsumerToResponse converts a domain consumer to a response DTO
func MapConsumerToResponse(c *consent.Consumer) *ConsumerResponse {
	if c == nil {
		return nil
	}

	var phoneNumber string
	if c.PhoneNumber != nil {
		phoneNumber = c.PhoneNumber.String()
	}

	var email string
	if c.Email != nil {
		email = *c.Email
	}

	return &ConsumerResponse{
		ID:          c.ID,
		PhoneNumber: phoneNumber,
		Email:       email,
		FirstName:   c.FirstName,
		LastName:    c.LastName,
		Attributes:  copyMetadataToAttributes(c.Metadata),
		CreatedAt:   c.CreatedAt,
		UpdatedAt:   c.UpdatedAt,
	}
}

// MapConsentStatusFromDomain converts domain consent to status DTO
func MapConsentStatusFromDomain(c *consent.ConsentAggregate) *ConsentStatus {
	if c == nil {
		return &ConsentStatus{
			HasConsent: false,
			Status:     consent.StatusRevoked,
		}
	}

	isActive := c.IsActive()
	status := c.GetCurrentStatus()
	
	var grantedAt *time.Time
	var expiresAt *time.Time
	
	if len(c.Versions) > 0 && c.CurrentVersion > 0 {
		current := &c.Versions[c.CurrentVersion-1]
		grantedAt = current.ConsentedAt
		expiresAt = current.ExpiresAt
	}

	return &ConsentStatus{
		HasConsent:  isActive,
		ConsentID:   &c.ID,
		Status:      status,
		GrantedAt:   grantedAt,
		ExpiresAt:   expiresAt,
	}
}

// MapGrantRequestToProof converts a grant request to a consent proof
func MapGrantRequestToProof(req GrantConsentRequest) consent.ConsentProof {
	return consent.ConsentProof{
		ID:              uuid.New(),
		VersionID:       uuid.New(), // Will be set properly when version is created
		Type:            consent.ProofTypeDigital,
		StorageLocation: "",
		Hash:            "",
		Metadata: consent.ProofMetadata{
			IPAddress: req.IPAddress,
			UserAgent: req.UserAgent,
			FormData:  make(map[string]string),
		},
		CreatedAt: time.Now(),
	}
}

// MapMetricsToResponse converts domain metrics to response DTO
func MapMetricsToResponse(metrics *consent.ConsentMetrics, period string) *ConsentMetrics {
	if metrics == nil {
		return nil
	}

	return &ConsentMetrics{
		Period:       period,
		TotalGrants:  make(map[consent.Type]int),  // TODO: Convert from int64 metrics
		TotalRevokes: make(map[consent.Type]int),  // TODO: Convert from int64 metrics
		ActiveCount:  make(map[consent.Type]int),  // TODO: Convert from int64 metrics
		Trends:       MapMetricTrends(metrics.Trends),
	}
}

// MapMetricTrends converts domain metric trends to response DTOs
func MapMetricTrends(trends []consent.MetricTrend) []ConsentTrend {
	result := make([]ConsentTrend, len(trends))
	for i, trend := range trends {
		result[i] = ConsentTrend{
			Date:    trend.Date,
			Grants:  copyTypeIntMap(trend.Grants),
			Revokes: copyTypeIntMap(trend.Revokes),
		}
	}
	return result
}

// MapImportDataToConsents parses import data into consent grant requests
func MapImportDataToConsents(format string, data []byte, source string) ([]GrantConsentRequest, error) {
	// Implementation depends on format (CSV, JSON, etc.)
	// This would be implemented based on specific import requirements
	return nil, nil
}

// MapConsentsToExportData converts consents to export format
func MapConsentsToExportData(consents []*consent.ConsentAggregate, format string) ([]byte, error) {
	// Implementation depends on format (CSV, JSON, etc.)
	// This would be implemented based on specific export requirements
	return nil, nil
}

// Helper functions

func copyPreferences(prefs map[string]string) map[string]string {
	if prefs == nil {
		return nil
	}
	
	result := make(map[string]string, len(prefs))
	for k, v := range prefs {
		result[k] = v
	}
	return result
}

func copyAttributes(attrs map[string]string) map[string]string {
	if attrs == nil {
		return nil
	}
	
	result := make(map[string]string, len(attrs))
	for k, v := range attrs {
		result[k] = v
	}
	return result
}

func copyMetadataToAttributes(metadata map[string]interface{}) map[string]string {
	if metadata == nil {
		return nil
	}
	
	result := make(map[string]string, len(metadata))
	for k, v := range metadata {
		if str, ok := v.(string); ok {
			result[k] = str
		} else {
			result[k] = fmt.Sprintf("%v", v)
		}
	}
	return result
}

func copyTypeIntMap(m map[consent.Type]int) map[consent.Type]int {
	if m == nil {
		return nil
	}
	
	result := make(map[consent.Type]int, len(m))
	for k, v := range m {
		result[k] = v
	}
	return result
}

// Validation helpers

// ValidateGrantRequest validates a grant consent request
func ValidateGrantRequest(req GrantConsentRequest) error {
	// Validation is done in the service layer and domain constructors
	// This is just for additional business rule validation if needed
	return nil
}

// ValidateUpdateRequest validates an update consent request  
func ValidateUpdateRequest(req UpdateConsentRequest) error {
	// Validation is done in the service layer and domain constructors
	// This is just for additional business rule validation if needed
	return nil
}

// ValidateImportRequest validates an import request
func ValidateImportRequest(req ImportConsentsRequest) error {
	// Validation is done in the service layer
	// This is just for additional business rule validation if needed
	return nil
}

// ValidateExportRequest validates an export request
func ValidateExportRequest(req ExportConsentsRequest) error {
	// Validation is done in the service layer
	// This is just for additional business rule validation if needed
	return nil
}