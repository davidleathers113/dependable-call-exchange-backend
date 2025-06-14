package consent

import (
	"context"
	"testing"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/consent"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestService_GrantConsent(t *testing.T) {
	ctx := context.Background()
	logger := zaptest.NewLogger(t)

	tests := []struct {
		name          string
		setupMocks    func(*mockConsentRepository, *mockConsumerRepository, *mockQueryRepository, *mockComplianceChecker, *mockEventPublisher)
		request       GrantConsentRequest
		expectedError bool
		errorContains string
		validate      func(*testing.T, *ConsentResponse)
	}{
		{
			name: "successful consent grant for new consumer",
			setupMocks: func(cr *mockConsentRepository, consR *mockConsumerRepository, qr *mockQueryRepository, cc *mockComplianceChecker, ep *mockEventPublisher) {
				// Compliance check passes
				cc.On("ValidateConsentGrant", ctx, mock.AnythingOfType("GrantConsentRequest")).Return(nil)

				// Consumer not found, will create new
				consR.On("GetByPhoneNumber", ctx, "+14155551234").Return(nil, errors.NewNotFoundError("not found"))

				// Save new consumer
				consR.On("Save", ctx, mock.AnythingOfType("*consent.Consumer")).Return(nil)

				// No existing consent
				cr.On("GetByConsumerAndType", ctx, mock.AnythingOfType("uuid.UUID"), consent.TypeTCPA).
					Return(nil, errors.NewNotFoundError("not found"))

				// Save consent
				cr.On("Save", ctx, mock.AnythingOfType("*consent.ConsentAggregate")).Return(nil)

				// Publish event
				ep.On("PublishConsentGranted", ctx, mock.AnythingOfType("consent.ConsentCreatedEvent")).Return(nil)
			},
			request: GrantConsentRequest{
				PhoneNumber: "+14155551234",
				ConsentType: consent.TypeTCPA,
				Channel:     consent.ChannelWeb,
				IPAddress:   "192.168.1.1",
				UserAgent:   "Mozilla/5.0",
				Preferences: map[string]string{
					"frequency": "daily",
				},
			},
			expectedError: false,
			validate: func(t *testing.T, resp *ConsentResponse) {
				assert.NotNil(t, resp)
				assert.Equal(t, consent.TypeTCPA, resp.Type)
				assert.Equal(t, consent.StatusActive, resp.Status)
				assert.Equal(t, consent.ChannelWeb, resp.Channel)
				assert.Equal(t, "daily", resp.Preferences["frequency"])
			},
		},
		{
			name: "successful consent grant for existing consumer",
			setupMocks: func(cr *mockConsentRepository, consR *mockConsumerRepository, qr *mockQueryRepository, cc *mockComplianceChecker, ep *mockEventPublisher) {
				// Use a fixed UUID so the test is deterministic
				consumerID := uuid.MustParse("12345678-1234-1234-1234-123456789012")
				existingConsumer := &consent.Consumer{
					ID:          consumerID,
					PhoneNumber: func() *values.PhoneNumber { p, _ := values.NewPhoneNumber("+14155551234"); return &p }(),
				}

				cc.On("ValidateConsentGrant", ctx, mock.AnythingOfType("GrantConsentRequest")).Return(nil)
				consR.On("GetByID", ctx, consumerID).Return(existingConsumer, nil)
				cr.On("GetByConsumerAndType", ctx, consumerID, consent.TypeMarketing).
					Return(nil, errors.NewNotFoundError("not found"))
				cr.On("Save", ctx, mock.AnythingOfType("*consent.ConsentAggregate")).Return(nil)
				ep.On("PublishConsentGranted", ctx, mock.AnythingOfType("consent.ConsentCreatedEvent")).Return(nil)
			},
			request: GrantConsentRequest{
				ConsumerID:  uuid.MustParse("12345678-1234-1234-1234-123456789012"),
				ConsentType: consent.TypeMarketing,
				Channel:     consent.ChannelEmail,
			},
			expectedError: false,
			validate: func(t *testing.T, resp *ConsentResponse) {
				assert.NotNil(t, resp)
				assert.Equal(t, consent.TypeMarketing, resp.Type)
				assert.Equal(t, consent.StatusActive, resp.Status)
			},
		},
		{
			name: "update existing consent",
			setupMocks: func(cr *mockConsentRepository, consR *mockConsumerRepository, qr *mockQueryRepository, cc *mockComplianceChecker, ep *mockEventPublisher) {
				consumerID := uuid.New()
				existingConsumer := &consent.Consumer{
					ID:          consumerID,
					PhoneNumber: func() *values.PhoneNumber { p, _ := values.NewPhoneNumber("+14155551234"); return &p }(),
				}

existingConsent := createTestConsent(consumerID, consent.TypeTCPA)
				cc.On("ValidateConsentGrant", ctx, mock.AnythingOfType("GrantConsentRequest")).Return(nil)
				consR.On("GetByPhoneNumber", ctx, "+14155551234").Return(existingConsumer, nil)
				cr.On("GetByConsumerAndType", ctx, consumerID, consent.TypeTCPA).Return(existingConsent, nil)
				cr.On("Save", ctx, mock.AnythingOfType("*consent.ConsentAggregate")).Return(nil)
				ep.On("PublishConsentGranted", ctx, mock.AnythingOfType("consent.ConsentCreatedEvent")).Return(nil)
			},
			request: GrantConsentRequest{
				PhoneNumber: "+14155551234",
				ConsentType: consent.TypeTCPA,
				Channel:     consent.ChannelSMS,
			},
			expectedError: false,
			validate: func(t *testing.T, resp *ConsentResponse) {
				assert.NotNil(t, resp)
				assert.Equal(t, consent.StatusActive, resp.Status)
				assert.Equal(t, 2, resp.Version) // Version incremented
			},
		},
		{
			name: "compliance validation fails",
			setupMocks: func(cr *mockConsentRepository, consR *mockConsumerRepository, qr *mockQueryRepository, cc *mockComplianceChecker, ep *mockEventPublisher) {
				cc.On("ValidateConsentGrant", ctx, mock.AnythingOfType("GrantConsentRequest")).
					Return(errors.NewValidationError("COMPLIANCE_FAILED", "compliance check failed"))
			},
			request: GrantConsentRequest{
				PhoneNumber: "+14155551234",
				ConsentType: consent.TypeTCPA,
				Channel:     consent.ChannelWeb,
			},
			expectedError: true,
			errorContains: "compliance validation",
		},
		{
			name: "invalid phone number",
			setupMocks: func(cr *mockConsentRepository, consR *mockConsumerRepository, qr *mockQueryRepository, cc *mockComplianceChecker, ep *mockEventPublisher) {
				cc.On("ValidateConsentGrant", ctx, mock.AnythingOfType("GrantConsentRequest")).Return(nil)
			},
			request: GrantConsentRequest{
				PhoneNumber: "invalid",
				ConsentType: consent.TypeTCPA,
				Channel:     consent.ChannelWeb,
			},
			expectedError: true,
			errorContains: "invalid phone number",
		},
		{
			name: "missing identifier",
			setupMocks: func(cr *mockConsentRepository, consR *mockConsumerRepository, qr *mockQueryRepository, cc *mockComplianceChecker, ep *mockEventPublisher) {
				cc.On("ValidateConsentGrant", ctx, mock.AnythingOfType("GrantConsentRequest")).Return(nil)
			},
			request: GrantConsentRequest{
				ConsentType: consent.TypeTCPA,
				Channel:     consent.ChannelWeb,
			},
			expectedError: true,
			errorContains: "consumer_id, phone_number, or email is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mocks
			consentRepo := new(mockConsentRepository)
			consumerRepo := new(mockConsumerRepository)
			queryRepo := new(mockQueryRepository)
			complianceChecker := new(mockComplianceChecker)
			eventPublisher := new(mockEventPublisher)

			// Setup mocks
			tt.setupMocks(consentRepo, consumerRepo, queryRepo, complianceChecker, eventPublisher)

			// Create service
			svc := NewService(logger, consentRepo, consumerRepo, queryRepo, complianceChecker, eventPublisher)

			// Execute
			resp, err := svc.GrantConsent(ctx, tt.request)

			// Validate
			if tt.expectedError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				require.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, resp)
				}
			}

			// Verify mocks
			consentRepo.AssertExpectations(t)
			consumerRepo.AssertExpectations(t)
			queryRepo.AssertExpectations(t)
			complianceChecker.AssertExpectations(t)
			eventPublisher.AssertExpectations(t)
		})
	}
}

func TestService_RevokeConsent(t *testing.T) {
	ctx := context.Background()
	logger := zaptest.NewLogger(t)

	tests := []struct {
		name          string
		consumerID    uuid.UUID
		consentType   consent.Type
		setupMocks    func(*mockConsentRepository, *mockEventPublisher)
		expectedError bool
		errorContains string
	}{
		{
			name:        "successful revocation",
			consumerID:  uuid.New(),
			consentType: consent.TypeTCPA,
			setupMocks: func(cr *mockConsentRepository, ep *mockEventPublisher) {
				consumerID := uuid.New()
				existingConsent := createTestConsent(consumerID, consent.TypeTCPA)
				cr.On("GetByConsumerAndType", ctx, mock.AnythingOfType("uuid.UUID"), consent.TypeTCPA).
					Return(existingConsent, nil)
				cr.On("Save", ctx, mock.AnythingOfType("*consent.ConsentAggregate")).Return(nil)
				ep.On("PublishConsentRevoked", ctx, mock.AnythingOfType("consent.ConsentRevokedEvent")).Return(nil)
			},
			expectedError: false,
		},
		{
			name:        "consent not found",
			consumerID:  uuid.New(),
			consentType: consent.TypeMarketing,
			setupMocks: func(cr *mockConsentRepository, ep *mockEventPublisher) {
				cr.On("GetByConsumerAndType", ctx, mock.AnythingOfType("uuid.UUID"), consent.TypeMarketing).
					Return(nil, errors.NewNotFoundError("not found"))
			},
			expectedError: true,
			errorContains: "not found",
		},
		{
			name:        "already revoked consent",
			consumerID:  uuid.New(),
			consentType: consent.TypeGDPR,
			setupMocks: func(cr *mockConsentRepository, ep *mockEventPublisher) {
				consumerID := uuid.New()
				revokedConsent := createRevokedConsent(consumerID, consent.TypeGDPR)
				cr.On("GetByConsumerAndType", ctx, mock.AnythingOfType("uuid.UUID"), consent.TypeGDPR).
					Return(revokedConsent, nil)
			},
			expectedError: true,
			errorContains: "already revoked",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mocks
			consentRepo := new(mockConsentRepository)
			consumerRepo := new(mockConsumerRepository)
			queryRepo := new(mockQueryRepository)
			complianceChecker := new(mockComplianceChecker)
			eventPublisher := new(mockEventPublisher)

			// Setup mocks
			tt.setupMocks(consentRepo, eventPublisher)

			// Create service
			svc := NewService(logger, consentRepo, consumerRepo, queryRepo, complianceChecker, eventPublisher)

			// Execute
			err := svc.RevokeConsent(ctx, tt.consumerID, tt.consentType)

			// Validate
			if tt.expectedError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				require.NoError(t, err)
			}

			// Verify mocks
			consentRepo.AssertExpectations(t)
			eventPublisher.AssertExpectations(t)
		})
	}
}

func TestService_CheckConsent(t *testing.T) {
	ctx := context.Background()
	logger := zaptest.NewLogger(t)

	tests := []struct {
		name          string
		phoneNumber   string
		consentType   consent.Type
		setupMocks    func(*mockConsentRepository, *mockConsumerRepository)
		expectedError bool
		errorContains string
		validate      func(*testing.T, *ConsentStatus)
	}{
		{
			name:        "active consent found",
			phoneNumber: "+14155551234",
			consentType: consent.TypeTCPA,
			setupMocks: func(cr *mockConsentRepository, consR *mockConsumerRepository) {
				consumerID := uuid.New()
				consumer := &consent.Consumer{
					ID:          consumerID,
					PhoneNumber: func() *values.PhoneNumber { p, _ := values.NewPhoneNumber("+14155551234"); return &p }(),
				}

activeConsent := createTestConsent(consumerID, consent.TypeTCPA)
				consR.On("GetByPhoneNumber", ctx, "+14155551234").Return(consumer, nil)
				cr.On("GetByConsumerAndType", ctx, consumerID, consent.TypeTCPA).Return(activeConsent, nil)
			},
			expectedError: false,
			validate: func(t *testing.T, status *ConsentStatus) {
				assert.True(t, status.HasConsent)
				assert.Equal(t, consent.StatusActive, status.Status)
				assert.NotNil(t, status.ConsentID)
			},
		},
		{
			name:        "expired consent",
			phoneNumber: "+14155551234",
			consentType: consent.TypeMarketing,
			setupMocks: func(cr *mockConsentRepository, consR *mockConsumerRepository) {
				consumerID := uuid.New()
				consumer := &consent.Consumer{
					ID:          consumerID,
					PhoneNumber: func() *values.PhoneNumber { p, _ := values.NewPhoneNumber("+14155551234"); return &p }(),
				}

				expiredConsent := createExpiredConsent(consumerID, consent.TypeMarketing)
				consR.On("GetByPhoneNumber", ctx, "+14155551234").Return(consumer, nil)
				cr.On("GetByConsumerAndType", ctx, consumerID, consent.TypeMarketing).Return(expiredConsent, nil)
			},
			expectedError: false,
			validate: func(t *testing.T, status *ConsentStatus) {
				assert.False(t, status.HasConsent)
				assert.Equal(t, consent.StatusExpired, status.Status) // Status becomes expired when past expiration time
			},
		},
		{
			name:        "consumer not found",
			phoneNumber: "+14155551234",
			consentType: consent.TypeGDPR,
			setupMocks: func(cr *mockConsentRepository, consR *mockConsumerRepository) {
				consR.On("GetByPhoneNumber", ctx, "+14155551234").Return(nil, errors.NewNotFoundError("not found"))
			},
			expectedError: false,
			validate: func(t *testing.T, status *ConsentStatus) {
				assert.False(t, status.HasConsent)
				assert.Equal(t, consent.StatusRevoked, status.Status)
			},
		},
		{
			name:        "invalid phone number",
			phoneNumber: "invalid",
			consentType: consent.TypeTCPA,
			setupMocks:  func(cr *mockConsentRepository, consR *mockConsumerRepository) {},
			expectedError: true,
			errorContains: "invalid phone number",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mocks
			consentRepo := new(mockConsentRepository)
			consumerRepo := new(mockConsumerRepository)
			queryRepo := new(mockQueryRepository)
			complianceChecker := new(mockComplianceChecker)
			eventPublisher := new(mockEventPublisher)

			// Setup mocks
			tt.setupMocks(consentRepo, consumerRepo)

			// Create service
			svc := NewService(logger, consentRepo, consumerRepo, queryRepo, complianceChecker, eventPublisher)

			// Execute
			status, err := svc.CheckConsent(ctx, tt.phoneNumber, tt.consentType)

			// Validate
			if tt.expectedError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				require.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, status)
				}
			}

			// Verify mocks
			consentRepo.AssertExpectations(t)
			consumerRepo.AssertExpectations(t)
		})
	}
}

func TestService_ImportConsents(t *testing.T) {
	ctx := context.Background()
	logger := zaptest.NewLogger(t)

	tests := []struct {
		name          string
		request       ImportConsentsRequest
		setupMocks    func(*mockConsentRepository, *mockConsumerRepository, *mockComplianceChecker, *mockEventPublisher)
		expectedError bool
		errorContains string
		validate      func(*testing.T, *ImportResult)
	}{
		{
			name: "successful CSV import",
			request: ImportConsentsRequest{
				Format: "csv",
				Data:   []byte("phone_number,consent_type,channel\n+14155551234,tcpa,web\n+14155551235,marketing,email"),
				Source: "bulk_import",
			},
			setupMocks: func(cr *mockConsentRepository, consR *mockConsumerRepository, cc *mockComplianceChecker, ep *mockEventPublisher) {
				// Set up expectations for two imports
				cc.On("ValidateConsentGrant", ctx, mock.AnythingOfType("GrantConsentRequest")).Return(nil).Times(2)
				consR.On("GetByPhoneNumber", ctx, mock.AnythingOfType("string")).Return(nil, errors.NewNotFoundError("not found")).Times(2)
				consR.On("Save", ctx, mock.AnythingOfType("*consent.Consumer")).Return(nil).Times(2)
				cr.On("GetByConsumerAndType", ctx, mock.AnythingOfType("uuid.UUID"), mock.AnythingOfType("consent.Type")).
					Return(nil, errors.NewNotFoundError("not found")).Times(2)
				cr.On("Save", ctx, mock.AnythingOfType("*consent.ConsentAggregate")).Return(nil).Times(2)
				ep.On("PublishConsentGranted", ctx, mock.AnythingOfType("consent.ConsentCreatedEvent")).Return(nil).Times(4)
			},
			expectedError: false,
			validate: func(t *testing.T, result *ImportResult) {
				assert.Equal(t, 2, result.TotalRecords)
				assert.Equal(t, 2, result.SuccessCount)
				assert.Equal(t, 0, result.FailureCount)
				assert.Empty(t, result.Errors)
			},
		},
		{
			name: "validation only mode",
			request: ImportConsentsRequest{
				Format:       "csv",
				Data:         []byte("phone_number,consent_type,channel\n+14155551234,tcpa,web\ninvalid,marketing,email"),
				Source:       "bulk_import",
				ValidateOnly: true,
			},
			setupMocks: func(cr *mockConsentRepository, consR *mockConsumerRepository, cc *mockComplianceChecker, ep *mockEventPublisher) {
				// No repository calls in validation mode
			},
			expectedError: false,
			validate: func(t *testing.T, result *ImportResult) {
				assert.Equal(t, 2, result.TotalRecords)
				assert.Equal(t, 1, result.SuccessCount)
				assert.Equal(t, 1, result.FailureCount)
				assert.Len(t, result.Errors, 1)
			},
		},
		{
			name: "invalid format",
			request: ImportConsentsRequest{
				Format: "xml",
				Data:   []byte(""),
			},
			setupMocks:    func(cr *mockConsentRepository, consR *mockConsumerRepository, cc *mockComplianceChecker, ep *mockEventPublisher) {},
			expectedError: true,
			errorContains: "unsupported import format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mocks
			consentRepo := new(mockConsentRepository)
			consumerRepo := new(mockConsumerRepository)
			queryRepo := new(mockQueryRepository)
			complianceChecker := new(mockComplianceChecker)
			eventPublisher := new(mockEventPublisher)

			// Setup mocks
			tt.setupMocks(consentRepo, consumerRepo, complianceChecker, eventPublisher)

			// Create service
			svc := NewService(logger, consentRepo, consumerRepo, queryRepo, complianceChecker, eventPublisher)

			// Execute
			result, err := svc.ImportConsents(ctx, tt.request)

			// Validate
			if tt.expectedError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				require.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, result)
				}
			}

			// Verify mocks
			consentRepo.AssertExpectations(t)
			consumerRepo.AssertExpectations(t)
			complianceChecker.AssertExpectations(t)
			eventPublisher.AssertExpectations(t)
		})
	}
}


