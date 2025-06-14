package consent

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
)

// BenchmarkConsentAggregate_Grant benchmarks consent granting
func BenchmarkConsentAggregate_Grant(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		consumerID := uuid.New()
		businessID := uuid.New()
		channels := []Channel{ChannelSMS}
		purpose := PurposeMarketing
		source := SourceWebForm
		
		aggregate, err := NewConsentAggregate(consumerID, businessID, TypeMarketing, channels, purpose, source)
		if err != nil {
			b.Fatal(err)
		}
		
		// Grant consent with proof
		proof := ConsentProof{
			ID:              uuid.New(),
			Type:            ProofTypeFormSubmission,
			StorageLocation: "s3://bucket/proof.pdf",
			Hash:            "sha256:abc123",
			Metadata: ProofMetadata{
				IPAddress: "192.168.1.1",
				UserAgent: "Mozilla/5.0",
			},
			CreatedAt: time.Now(),
		}
		
		expiresAt := time.Now().Add(365 * 24 * time.Hour)
		err = aggregate.Grant(proof, nil, &expiresAt)
		if err != nil {
			b.Fatal(err)
		}
		
		if aggregate.ConsumerID != consumerID {
			b.Fatal("ConsumerID not set correctly")
		}
	}
}

// BenchmarkConsentAggregate_Update benchmarks consent updates
func BenchmarkConsentAggregate_Update(b *testing.B) {
	consumerID := uuid.New()
	businessID := uuid.New()
	channels := []Channel{ChannelSMS}
	purpose := PurposeMarketing
	source := SourceWebForm
	
	aggregate, err := NewConsentAggregate(consumerID, businessID, TypeMarketing, channels, purpose, source)
	if err != nil {
		b.Fatal(err)
	}
	
	// Activate consent first
	proof := ConsentProof{
		ID:              uuid.New(),
		Type:            ProofTypeFormSubmission,
		StorageLocation: "s3://bucket/proof.pdf",
		Hash:            "sha256:abc123",
		Metadata:        ProofMetadata{},
		CreatedAt:       time.Now(),
	}
	expiresAt := time.Now().Add(365 * 24 * time.Hour)
	aggregate.ActivateConsent([]ConsentProof{proof}, &expiresAt)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		preferences := map[string]string{
			"frequency":    "daily",
			"update_count": fmt.Sprintf("%d", i),
		}
		err := aggregate.UpdatePreferences(preferences)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkConsentAggregate_IsValid benchmarks validity checks
func BenchmarkConsentAggregate_IsValid(b *testing.B) {
	consumerID := uuid.New()
	businessID := uuid.New()
	channels := []Channel{ChannelSMS}
	purpose := PurposeMarketing
	source := SourceWebForm
	
	aggregate, err := NewConsentAggregate(consumerID, businessID, TypeMarketing, channels, purpose, source)
	if err != nil {
		b.Fatal(err)
	}
	
	// Activate consent
	proof := ConsentProof{
		ID:              uuid.New(),
		Type:            ProofTypeFormSubmission,
		StorageLocation: "s3://bucket/proof.pdf",
		Hash:            "sha256:abc123",
		Metadata:        ProofMetadata{},
		CreatedAt:       time.Now(),
	}
	expiresAt := time.Now().Add(365 * 24 * time.Hour)
	aggregate.ActivateConsent([]ConsentProof{proof}, &expiresAt)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		valid := aggregate.IsActive()
		if !valid {
			b.Fatal("Consent should be valid")
		}
	}
}

// BenchmarkConsumer_Validate benchmarks consumer creation and validation
func BenchmarkConsumer_Create(b *testing.B) {
	phoneNumbers := []string{
		"+14155551234",
		"+442071234567",
		"+81312345678",
		"+61234567890",
		"+33123456789",
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		phoneNumber := phoneNumbers[i%len(phoneNumbers)]
		email := "bench@example.com"
		
		consumer, err := NewConsumer(phoneNumber, &email, "John", "Doe")
		if err != nil {
			b.Fatal(err)
		}
		
		// Verify consumer was created correctly
		if consumer.PhoneNumber == nil {
			b.Fatal("Phone number should not be nil")
		}
	}
}

// BenchmarkConsumer_UpdateMetadata benchmarks metadata management
func BenchmarkConsumer_UpdateMetadata(b *testing.B) {
	email := "bench@example.com"
	consumer, err := NewConsumer("+14155551234", &email, "John", "Doe")
	if err != nil {
		b.Fatal(err)
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("attr_%d", i%100)
		value := fmt.Sprintf("value_%d", i)
		consumer.Metadata[key] = value
	}
}

// BenchmarkStatus_String benchmarks status string conversion
func BenchmarkStatus_String(b *testing.B) {
	statuses := []ConsentStatus{
		StatusActive,
		StatusRevoked,
		StatusExpired,
		StatusPending,
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		status := statuses[i%len(statuses)]
		_ = status.String()
	}
}

// BenchmarkPurpose_IsValid benchmarks purpose validation
func BenchmarkPurpose_IsValid(b *testing.B) {
	purposes := []Purpose{
		PurposeMarketing,
		PurposeServiceCalls,
		PurposeDebtCollection,
		PurposeEmergency,
		Purpose("invalid"),
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		purpose := purposes[i%len(purposes)]
		err := ValidatePurpose(purpose)
		if err != nil && i%len(purposes) < 4 {
			b.Fatal("Valid purpose marked as invalid")
		}
	}
}

// BenchmarkChannel_IsValid benchmarks channel validation
func BenchmarkChannel_IsValid(b *testing.B) {
	channels := []Channel{
		ChannelSMS,
		ChannelVoice,
		ChannelEmail,
		ChannelFax,
		Channel("invalid"),
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		channel := channels[i%len(channels)]
		err := ValidateChannel(channel)
		if err != nil && i%len(channels) < 4 {
			b.Fatal("Valid channel marked as invalid")
		}
	}
}

// BenchmarkConsentAggregate_MultipleOperations benchmarks a realistic workflow
func BenchmarkConsentAggregate_MultipleOperations(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Create consent
		consumerID := uuid.New()
		businessID := uuid.New()
		channels := []Channel{ChannelSMS, ChannelEmail}
		purpose := PurposeMarketing
		source := SourceWebForm
		
		aggregate, err := NewConsentAggregate(consumerID, businessID, TypeMarketing, channels, purpose, source)
		if err != nil {
			b.Fatal(err)
		}
		
		// Activate
		proof := ConsentProof{
			ID:              uuid.New(),
			Type:            ProofTypeFormSubmission,
			StorageLocation: "s3://bucket/proof.pdf",
			Hash:            "sha256:abc123",
			Metadata:        ProofMetadata{},
			CreatedAt:       time.Now(),
		}
		expiresAt := time.Now().Add(365 * 24 * time.Hour)
		err = aggregate.ActivateConsent([]ConsentProof{proof}, &expiresAt)
		if err != nil {
			b.Fatal(err)
		}
		
		// Update preferences
		preferences := map[string]string{
			"frequency": "weekly",
			"topics":    "promotions,news",
		}
		err = aggregate.UpdatePreferences(preferences)
		if err != nil {
			b.Fatal(err)
		}
		
		// Check validity
		if !aggregate.IsActive() {
			b.Fatal("Consent should be active")
		}
		
		// Revoke
		err = aggregate.RevokeConsent("User request", uuid.New())
		if err != nil {
			b.Fatal(err)
		}
		
		// Verify revoked
		if aggregate.GetCurrentStatus() != StatusRevoked {
			b.Fatal("Consent should be revoked")
		}
	}
}

// BenchmarkConsumer_BulkOperations benchmarks bulk consumer operations
func BenchmarkConsumer_BulkOperations(b *testing.B) {
	consumers := make([]*Consumer, 1000)
	for i := 0; i < 1000; i++ {
		phoneNumber := fmt.Sprintf("+1415555%04d", i)
		email := fmt.Sprintf("bench%d@example.com", i)
		consumer, err := NewConsumer(phoneNumber, &email, "John", fmt.Sprintf("Doe%d", i))
		if err != nil {
			b.Fatal(err)
		}
		consumers[i] = consumer
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		consumer := consumers[i%len(consumers)]
		
		// Update metadata
		consumer.Metadata["source"] = "benchmark"
		consumer.Metadata["iteration"] = i
		
		// Update contact
		newPhone := fmt.Sprintf("+1650555%04d", i%10000)
		err := consumer.UpdateContact(newPhone, consumer.Email)
		if err != nil {
			b.Fatal(err)
		}
		
		// Get primary contact
		_ = consumer.GetPrimaryContact()
	}
}