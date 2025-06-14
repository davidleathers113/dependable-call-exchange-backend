package consent

import (
	"context"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/consent"
)

// RoutingAdapter adapts the consent service for use by the call routing service
type RoutingAdapter struct {
	service Service
}

// NewRoutingAdapter creates a new routing adapter
func NewRoutingAdapter(service Service) *RoutingAdapter {
	return &RoutingAdapter{
		service: service,
	}
}

// CheckConsent implements the callrouting.ConsentService interface
func (a *RoutingAdapter) CheckConsent(ctx context.Context, phoneNumber string, consentType string) (bool, error) {
	// Map the consent type string to the domain type
	// For call routing, we use TCPA consent type
	var domainType consent.Type
	switch consentType {
	case "CALL", "VOICE":
		domainType = consent.TypeTCPA // TCPA covers telephone calls
	case "SMS":
		domainType = consent.TypeMarketing // Marketing consent for SMS
	case "EMAIL":
		domainType = consent.TypeMarketing // Marketing consent for email
	default:
		// Default to TCPA for call routing
		domainType = consent.TypeTCPA
	}

	// Check consent using the service
	status, err := a.service.CheckConsent(ctx, phoneNumber, domainType)
	if err != nil {
		return false, err
	}

	// Return true if consent exists
	return status.HasConsent, nil
}