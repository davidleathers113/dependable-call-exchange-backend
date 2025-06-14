package consent

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/consent"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Benchmark for GrantConsent operation
func BenchmarkService_GrantConsent(b *testing.B) {
	ctx := context.Background()

	// Initialize repositories
	consentRepo := &mockConsentRepository{
		consents: make(map[uuid.UUID]*consent.ConsentAggregate),
	}
	consumerRepo := &mockConsumerRepository{
		consumers: make(map[string]*consent.Consumer),
	}
	queryRepo := &mockQueryRepository{
		consents: make(map[uuid.UUID]*consent.ConsentAggregate),
	}
	
	// Initialize service
	service := NewService(
		zap.NewNop(),
		consentRepo,
		consumerRepo,
		queryRepo,
		&mockComplianceChecker{},
		&mockEventPublisher{},
	)

	// Pre-create consumers
	consumers := make([]*consent.Consumer, 100)
	for i := 0; i < 100; i++ {
		phoneNumber := fmt.Sprintf("+1415555%04d", i)
		email := fmt.Sprintf("bench%d@example.com", i)
		consumer, _ := consent.NewConsumer(phoneNumber, &email, "John", "Doe")
		consumer.ID = uuid.New()
		consumerRepo.Save(ctx, consumer)
		consumers[i] = consumer
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			req := GrantConsentRequest{
				ConsumerID:  consumers[i%len(consumers)].ID,
				ConsentType: consent.TypeMarketing,
				Channel:     consent.ChannelSMS,
				Preferences: map[string]string{
					"frequency": "daily",
				},
			}
			_, err := service.GrantConsent(ctx, req)
			if err != nil {
				b.Fatal(err)
			}
			i++
		}
	})
}

// Benchmark for CheckConsent operation (most frequent operation)
func BenchmarkService_CheckConsent(b *testing.B) {
	ctx := context.Background()

	// Initialize repositories
	consentRepo := &mockConsentRepository{
		consents: make(map[uuid.UUID]*consent.ConsentAggregate),
	}
	consumerRepo := &mockConsumerRepository{
		consumers: make(map[string]*consent.Consumer),
	}
	queryRepo := &mockQueryRepository{
		consents: make(map[uuid.UUID]*consent.ConsentAggregate),
	}
	
	// Initialize service with cache
	service := NewService(
		zap.NewNop(),
		consentRepo,
		consumerRepo,
		queryRepo,
		&mockComplianceChecker{},
		&mockEventPublisher{},
	)

	// Pre-populate data
	phoneNumbers := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		phoneNumber := fmt.Sprintf("+1415555%04d", i)
		phoneNumbers[i] = phoneNumber
		
		// Create consumer
		email := fmt.Sprintf("bench%d@example.com", i)
		consumer, _ := consent.NewConsumer(phoneNumber, &email, "Test", "User")
		consumer.ID = uuid.New()
		consumerRepo.Save(ctx, consumer)
		
		// Grant consent
		channels := []consent.Channel{consent.ChannelSMS}
		consentAggregate, _ := consent.NewConsentAggregate(
			consumer.ID,
			uuid.New(), // businessID
			consent.TypeMarketing,
			channels,
			consent.PurposeMarketing,
			consent.SourceAPI,
		)
		
		// Activate the consent
		proof := consent.ConsentProof{
			ID:              uuid.New(),
			Type:            consent.ProofTypeDigital,
			StorageLocation: "benchmark",
			Hash:            "hash",
			Metadata:        consent.ProofMetadata{},
			CreatedAt:       time.Now(),
		}
		expiresAt := time.Now().Add(365 * 24 * time.Hour)
		consentAggregate.Grant(proof, nil, &expiresAt)
		
		consentRepo.Save(ctx, consentAggregate)
		queryRepo.SaveConsent(ctx, consentAggregate)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			phoneNumber := phoneNumbers[i%len(phoneNumbers)]
			_, err := service.CheckConsent(ctx, phoneNumber, consent.TypeMarketing)
			if err != nil {
				b.Fatal(err)
			}
			i++
		}
	})
}

// Benchmark for GetActiveConsents operation
func BenchmarkService_GetActiveConsents(b *testing.B) {
	ctx := context.Background()

	// Initialize repositories
	consentRepo := &mockConsentRepository{
		consents: make(map[uuid.UUID]*consent.ConsentAggregate),
	}
	consumerRepo := &mockConsumerRepository{
		consumers: make(map[string]*consent.Consumer),
	}
	queryRepo := &mockQueryRepository{
		consents: make(map[uuid.UUID]*consent.ConsentAggregate),
	}
	
	// Initialize service
	service := NewService(
		zap.NewNop(),
		consentRepo,
		consumerRepo,
		queryRepo,
		&mockComplianceChecker{},
		&mockEventPublisher{},
	)

	// Pre-populate consumers with multiple consents
	consumerIDs := make([]uuid.UUID, 100)
	for i := 0; i < 100; i++ {
		phoneNumber := fmt.Sprintf("+1415555%04d", i)
		email := fmt.Sprintf("bench%d@example.com", i)
		consumer, _ := consent.NewConsumer(phoneNumber, &email, "Test", "User")
		consumer.ID = uuid.New()
		consumerRepo.Save(ctx, consumer)
		consumerIDs[i] = consumer.ID
		
		// Create multiple consents per consumer
		consentTypes := []consent.Type{
			consent.TypeMarketing,
			consent.TypeTCPA,
			consent.TypeGDPR,
		}
		
		for _, cType := range consentTypes {
			channels := []consent.Channel{consent.ChannelSMS}
			consentAggregate, _ := consent.NewConsentAggregate(
				consumer.ID,
				uuid.New(), // businessID
				cType,
				channels,
				consent.PurposeMarketing,
				consent.SourceAPI,
			)
			
			// Activate the consent
			proof := consent.ConsentProof{
				ID:              uuid.New(),
				Type:            consent.ProofTypeDigital,
				StorageLocation: "benchmark",
				Hash:            "hash",
				Metadata:        consent.ProofMetadata{},
				CreatedAt:       time.Now(),
			}
			expiresAt := time.Now().Add(365 * 24 * time.Hour)
			consentAggregate.Grant(proof, nil, &expiresAt)
			
			consentRepo.Save(ctx, consentAggregate)
			queryRepo.SaveConsent(ctx, consentAggregate)
		}
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			consumerID := consumerIDs[i%len(consumerIDs)]
			_, err := service.GetActiveConsents(ctx, consumerID)
			if err != nil {
				b.Fatal(err)
			}
			i++
		}
	})
}

// Benchmark for bulk import operation
func BenchmarkService_ImportConsents(b *testing.B) {
	ctx := context.Background()

	// Initialize repositories
	consentRepo := &mockConsentRepository{
		consents: make(map[uuid.UUID]*consent.ConsentAggregate),
	}
	consumerRepo := &mockConsumerRepository{
		consumers: make(map[string]*consent.Consumer),
	}
	queryRepo := &mockQueryRepository{
		consents: make(map[uuid.UUID]*consent.ConsentAggregate),
	}
	
	// Initialize service
	service := NewService(
		zap.NewNop(),
		consentRepo,
		consumerRepo,
		queryRepo,
		&mockComplianceChecker{},
		&mockEventPublisher{},
	)

	// Prepare CSV data for different sizes
	benchmarks := []struct {
		name    string
		records int
	}{
		{"10_records", 10},
		{"100_records", 100},
		{"1000_records", 1000},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			// Generate CSV data
			csvData := "phone_number,email,consent_type,channel\n"
			for i := 0; i < bm.records; i++ {
				csvData += fmt.Sprintf("+1415555%04d,bench%d@example.com,marketing,sms\n", i, i)
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				req := ImportConsentsRequest{
					Format: "csv",
					Data:   []byte(csvData),
					Source: "benchmark",
				}
				_, err := service.ImportConsents(ctx, req)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// Benchmark for concurrent consent updates
func BenchmarkService_ConcurrentUpdates(b *testing.B) {
	ctx := context.Background()

	// Initialize repositories
	consentRepo := &mockConsentRepository{
		consents: make(map[uuid.UUID]*consent.ConsentAggregate),
	}
	consumerRepo := &mockConsumerRepository{
		consumers: make(map[string]*consent.Consumer),
	}
	queryRepo := &mockQueryRepository{
		consents: make(map[uuid.UUID]*consent.ConsentAggregate),
	}
	
	// Initialize service
	service := NewService(
		zap.NewNop(),
		consentRepo,
		consumerRepo,
		queryRepo,
		&mockComplianceChecker{},
		&mockEventPublisher{},
	)

	// Create a consumer with consent
	email := "concurrent@example.com"
	consumer, _ := consent.NewConsumer("+14155551234", &email, "Test", "User")
	consumer.ID = uuid.New()
	consumerRepo.Save(ctx, consumer)

	// Grant initial consent
	grantReq := GrantConsentRequest{
		ConsumerID:  consumer.ID,
		ConsentType: consent.TypeMarketing,
		Channel:     consent.ChannelSMS,
	}
	_, err := service.GrantConsent(ctx, grantReq)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			updateReq := UpdateConsentRequest{
				ConsumerID:  consumer.ID,
				ConsentType: consent.TypeMarketing,
				Preferences: map[string]string{
					"update_index": fmt.Sprintf("%d", i),
					"timestamp":    fmt.Sprintf("%d", time.Now().Unix()),
				},
			}
			_, err := service.UpdateConsent(ctx, updateReq)
			if err != nil {
				b.Fatal(err)
			}
			i++
		}
	})
}

// Benchmark for GetConsentMetrics operation
func BenchmarkService_GetConsentMetrics(b *testing.B) {
	ctx := context.Background()

	// Initialize repositories
	consentRepo := &mockConsentRepository{
		consents: make(map[uuid.UUID]*consent.ConsentAggregate),
	}
	consumerRepo := &mockConsumerRepository{
		consumers: make(map[string]*consent.Consumer),
	}
	queryRepo := &mockQueryRepository{
		consents: make(map[uuid.UUID]*consent.ConsentAggregate),
	}
	
	// Initialize service
	service := NewService(
		zap.NewNop(),
		consentRepo,
		consumerRepo,
		queryRepo,
		&mockComplianceChecker{},
		&mockEventPublisher{},
	)

	// Pre-populate with consents over time
	startDate := time.Now().Add(-30 * 24 * time.Hour)
	for day := 0; day < 30; day++ {
		for i := 0; i < 100; i++ {
			phoneNumber := fmt.Sprintf("+1415555%04d", i)
			email := fmt.Sprintf("metrics%d@example.com", i)
			consumer, _ := consent.NewConsumer(phoneNumber, &email, "Test", "User")
			consumer.ID = uuid.New()
			consumer.CreatedAt = startDate.Add(time.Duration(day) * 24 * time.Hour)
			consumerRepo.Save(ctx, consumer)
			
			// Create consent
			channels := []consent.Channel{consent.ChannelSMS}
			consentAggregate, _ := consent.NewConsentAggregate(
				consumer.ID,
				uuid.New(), // businessID
				consent.TypeMarketing,
				channels,
				consent.PurposeMarketing,
				consent.SourceAPI,
			)
			
			// Activate the consent
			proof := consent.ConsentProof{
				ID:              uuid.New(),
				Type:            consent.ProofTypeDigital,
				StorageLocation: "benchmark",
				Hash:            "hash",
				Metadata:        consent.ProofMetadata{},
				CreatedAt:       consumer.CreatedAt,
			}
			expiresAt := time.Now().Add(365 * 24 * time.Hour)
			consentAggregate.Grant(proof, nil, &expiresAt)
			
			consentRepo.Save(ctx, consentAggregate)
			queryRepo.SaveConsent(ctx, consentAggregate)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := MetricsRequest{
			StartDate: startDate,
			EndDate:   time.Now(),
			GroupBy:   "day",
		}
		_, err := service.GetConsentMetrics(ctx, req)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Mock implementations for benchmarks
