package consent

import (
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/consent"
	"github.com/google/uuid"
)

// createTestConsent creates a basic consent aggregate for testing
func createTestConsent(consumerID uuid.UUID, consentType consent.Type) *consent.ConsentAggregate {
	businessID := uuid.New()
	channels := []consent.Channel{consent.ChannelVoice}
	purpose := consent.PurposeServiceCalls
	source := consent.SourceAPI
	
	aggregate, _ := consent.NewConsentAggregate(
		consumerID,
		businessID, 
		consentType,
		channels,
		purpose,
		source,
	)
	
	// Activate the consent to make it available for updates
	proof := consent.ConsentProof{
		ID:              uuid.New(),
		VersionID:       uuid.New(),
		Type:            consent.ProofTypeDigital,
		StorageLocation: "",
		Hash:            "",
		Metadata: consent.ProofMetadata{
			IPAddress: "127.0.0.1",
			UserAgent: "test",
			FormData:  map[string]string{},
		},
		CreatedAt: time.Now(),
	}
	
	_ = aggregate.ActivateConsent([]consent.ConsentProof{proof}, nil)
	
	return aggregate
}

// createExpiredConsent creates a consent that is expired
func createExpiredConsent(consumerID uuid.UUID, consentType consent.Type) *consent.ConsentAggregate {
	aggregate := createTestConsent(consumerID, consentType)
	
	// Set expiration in the past
	expiredTime := time.Now().Add(-24 * time.Hour)
	if len(aggregate.Versions) > 0 {
		aggregate.Versions[0].ExpiresAt = &expiredTime
	}
	
	return aggregate
}

// createRevokedConsent creates a consent that is revoked
func createRevokedConsent(consumerID uuid.UUID, consentType consent.Type) *consent.ConsentAggregate {
	aggregate := createTestConsent(consumerID, consentType)
	
	// Revoke the consent
	aggregate.RevokeConsent("test revocation", uuid.New())
	
	return aggregate
}