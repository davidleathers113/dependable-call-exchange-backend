// +build integration

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/compliance"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/consent"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/infrastructure/cache"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/infrastructure/database"
	consentservice "github.com/davidleathers/dependable-call-exchange-backend/internal/service/consent"
	"github.com/davidleathers/dependable-call-exchange-backend/test/testutil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/zap"
)

func TestConsentServiceIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	ctx := context.Background()

	// Start PostgreSQL container
	postgresContainer, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("postgres:16-alpine"),
		postgres.WithDatabase("consent_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	require.NoError(t, err)
	defer postgresContainer.Terminate(ctx)

	// Start Redis container
	redisContainer, err := redis.RunContainer(ctx,
		testcontainers.WithImage("redis:7-alpine"),
		redis.WithSnapshotting(10, 1),
		redis.WithLogLevel(redis.LogLevelVerbose),
	)
	require.NoError(t, err)
	defer redisContainer.Terminate(ctx)

	// Get connection strings
	postgresURL, err := postgresContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	redisURL, err := redisContainer.ConnectionString(ctx)
	require.NoError(t, err)

	// Initialize database
	db, err := testutil.InitTestDB(postgresURL)
	require.NoError(t, err)
	defer db.Close()

	// Run migrations
	err = testutil.RunMigrations(db, "../../migrations")
	require.NoError(t, err)

	// Initialize repositories
	consentRepo := database.NewConsentRepository(db)
	consumerRepo := database.NewConsumerRepository(db)
	queryRepo := database.NewConsentQueryRepository(db)

	// Initialize cache
	redisCache, err := cache.NewConsentCache(redisURL)
	require.NoError(t, err)

	// Initialize logger
	logger := zap.NewNop()

	// Initialize mocks
	mockComplianceChecker := &mockComplianceChecker{}
	mockEventPublisher := &mockEventPublisher{}

	// Initialize service
	service := consentservice.NewService(
		logger,
		consentRepo,
		consumerRepo,
		queryRepo,
		mockComplianceChecker,
		mockEventPublisher,
	)

	// Run test scenarios
	t.Run("BasicConsentWorkflow", func(t *testing.T) {
		testBasicConsentWorkflow(t, ctx, service)
	})

	t.Run("MultipleConsentTypes", func(t *testing.T) {
		testMultipleConsentTypes(t, ctx, service)
	})

	t.Run("ConsentUpdateFlow", func(t *testing.T) {
		testConsentUpdateFlow(t, ctx, service)
	})

	t.Run("ConsentSearch", func(t *testing.T) {
		testConsentSearch(t, ctx, service)
	})

	t.Run("ImportExport", func(t *testing.T) {
		testImportExport(t, ctx, service)
	})
}

func testBasicConsentWorkflow(t *testing.T, ctx context.Context, service consentservice.Service) {
	// Grant consent
	grantReq := consentservice.GrantConsentRequest{
		PhoneNumber: "+14155551234",
		Email:       "test@example.com",
		ConsentType: consent.TypeMarketing,
		Channel:     consent.ChannelSMS,
		IPAddress:   "192.168.1.1",
		UserAgent:   "Mozilla/5.0",
		Preferences: map[string]string{
			"frequency": "weekly",
		},
	}

	consentResp, err := service.GrantConsent(ctx, grantReq)
	require.NoError(t, err)
	assert.NotNil(t, consentResp)
	assert.Equal(t, consent.StatusActive, consentResp.Status)
	assert.Equal(t, consent.TypeMarketing, consentResp.Type)
	assert.Equal(t, consent.ChannelSMS, consentResp.Channel)

	// Check consent
	status, err := service.CheckConsent(ctx, "+14155551234", consent.TypeMarketing)
	require.NoError(t, err)
	assert.True(t, status.HasConsent)
	assert.Equal(t, consent.StatusActive, status.Status)
	assert.NotNil(t, status.ConsentID)

	// Get active consents
	activeConsents, err := service.GetActiveConsents(ctx, consentResp.ConsumerID)
	require.NoError(t, err)
	assert.Len(t, activeConsents, 1)
	assert.Equal(t, consentResp.ID, activeConsents[0].ID)

	// Revoke consent
	err = service.RevokeConsent(ctx, consentResp.ConsumerID, consent.TypeMarketing)
	require.NoError(t, err)

	// Verify consent is revoked
	status, err = service.CheckConsent(ctx, "+14155551234", consent.TypeMarketing)
	require.NoError(t, err)
	assert.False(t, status.HasConsent)
	assert.Equal(t, consent.StatusRevoked, status.Status)
}

func testMultipleConsentTypes(t *testing.T, ctx context.Context, service consentservice.Service) {
	// Create consumer
	createReq := consentservice.CreateConsumerRequest{
		PhoneNumber: "+14155552222",
		Email:       "multi@example.com",
		FirstName:   "Test",
		LastName:    "User",
	}

	consumer, err := service.CreateConsumer(ctx, createReq)
	require.NoError(t, err)

	// Grant multiple consent types
	consentTypes := []consent.Type{
		consent.TypeMarketing,
		consent.TypeTransactional,
		consent.TypeInformational,
	}

	for _, cType := range consentTypes {
		grantReq := consentservice.GrantConsentRequest{
			ConsumerID:  consumer.ID,
			ConsentType: cType,
			Channel:     consent.ChannelEmail,
		}
		_, err := service.GrantConsent(ctx, grantReq)
		require.NoError(t, err)
	}

	// Get all active consents
	activeConsents, err := service.GetActiveConsents(ctx, consumer.ID)
	require.NoError(t, err)
	assert.Len(t, activeConsents, 3)

	// Revoke one consent type
	err = service.RevokeConsent(ctx, consumer.ID, consent.TypeMarketing)
	require.NoError(t, err)

	// Verify only 2 active consents remain
	activeConsents, err = service.GetActiveConsents(ctx, consumer.ID)
	require.NoError(t, err)
	assert.Len(t, activeConsents, 2)
}

func testConsentUpdateFlow(t *testing.T, ctx context.Context, service consentservice.Service) {
	// Grant initial consent
	grantReq := consentservice.GrantConsentRequest{
		PhoneNumber: "+14155553333",
		Email:       "update@example.com",
		ConsentType: consent.TypeMarketing,
		Channel:     consent.ChannelSMS,
		Preferences: map[string]string{
			"frequency": "daily",
		},
	}

	consentResp, err := service.GrantConsent(ctx, grantReq)
	require.NoError(t, err)

	// Update consent preferences
	updateReq := consentservice.UpdateConsentRequest{
		ConsumerID:  consentResp.ConsumerID,
		ConsentType: consent.TypeMarketing,
		Preferences: map[string]string{
			"frequency": "weekly",
			"topics":    "promotions,news",
		},
	}

	updated, err := service.UpdateConsent(ctx, updateReq)
	require.NoError(t, err)
	assert.Equal(t, "weekly", updated.Preferences["frequency"])
	assert.Equal(t, "promotions,news", updated.Preferences["topics"])
	assert.Greater(t, updated.Version, consentResp.Version)
}

func testConsentSearch(t *testing.T, ctx context.Context, service consentservice.Service) {
	// Create test data
	for i := 0; i < 5; i++ {
		grantReq := consentservice.GrantConsentRequest{
			PhoneNumber: fmt.Sprintf("+1415555%04d", 4000+i),
			Email:       fmt.Sprintf("search%d@example.com", i),
			ConsentType: consent.TypeMarketing,
			Channel:     consent.ChannelSMS,
		}
		_, err := service.GrantConsent(ctx, grantReq)
		require.NoError(t, err)
	}

	// Get consumer by phone
	consumer, err := service.GetConsumerByPhone(ctx, "+14155554002")
	require.NoError(t, err)
	assert.Equal(t, "+14155554002", consumer.PhoneNumber)

	// Get consumer by email
	consumer, err = service.GetConsumerByEmail(ctx, "search3@example.com")
	require.NoError(t, err)
	assert.Equal(t, "search3@example.com", consumer.Email)
}

func testImportExport(t *testing.T, ctx context.Context, service consentservice.Service) {
	// Prepare CSV data
	csvData := `phone_number,email,consent_type,channel
+14155556001,import1@example.com,marketing,sms
+14155556002,import2@example.com,transactional,email
+14155556003,import3@example.com,marketing,voice
`

	// Import consents
	importReq := consentservice.ImportConsentsRequest{
		Format: "csv",
		Data:   []byte(csvData),
		Source: "test_import",
	}

	importResult, err := service.ImportConsents(ctx, importReq)
	require.NoError(t, err)
	assert.Equal(t, 3, importResult.TotalRecords)
	assert.Equal(t, 3, importResult.SuccessCount)
	assert.Equal(t, 0, importResult.FailureCount)

	// Export consents
	exportReq := consentservice.ExportConsentsRequest{
		Format: "json",
		Filters: consentservice.ExportFilters{
			ConsentTypes: []consent.Type{consent.TypeMarketing},
			Status:       []consent.ConsentStatus{consent.StatusActive},
		},
	}

	exportResult, err := service.ExportConsents(ctx, exportReq)
	require.NoError(t, err)
	assert.Equal(t, "json", exportResult.Format)
	assert.Greater(t, exportResult.RecordCount, 0)
	assert.Greater(t, len(exportResult.Data), 0)

	// Verify exported data
	var exportedConsents []map[string]interface{}
	err = json.Unmarshal(exportResult.Data, &exportedConsents)
	require.NoError(t, err)
	assert.Greater(t, len(exportedConsents), 0)
}

// Mock implementations
type mockComplianceChecker struct{}

func (m *mockComplianceChecker) CheckConsentRequirements(ctx context.Context, phoneNumber string, consentType consent.Type) (*compliance.ComplianceRule, error) {
	// Mock compliance check - allow all except specific numbers
	if phoneNumber == "+19999999999" {
		return nil, fmt.Errorf("number blocked by compliance")
	}
	return &compliance.ComplianceRule{
		ID:          uuid.New(),
		Name:        "default_rule",
		Description: "Default compliance rule",
		Type:        "consent",
		Enabled:     true,
	}, nil
}

func (m *mockComplianceChecker) ValidateConsentGrant(ctx context.Context, req consentservice.GrantConsentRequest) error {
	// Mock validation - block specific numbers
	if req.PhoneNumber == "+19999999999" {
		return fmt.Errorf("number failed compliance validation")
	}
	return nil
}

type mockEventPublisher struct{}

func (m *mockEventPublisher) PublishConsentGranted(ctx context.Context, event consent.ConsentCreatedEvent) error {
	return nil
}

func (m *mockEventPublisher) PublishConsentRevoked(ctx context.Context, event consent.ConsentRevokedEvent) error {
	return nil
}

func (m *mockEventPublisher) PublishConsentUpdated(ctx context.Context, event consent.ConsentUpdatedEvent) error {
	return nil
}