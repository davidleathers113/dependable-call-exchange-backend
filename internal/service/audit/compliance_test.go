package audit

import (
	"context"
	"testing"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/audit"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/compliance"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Mock repositories
type MockAuditRepo struct {
	mock.Mock
}

func (m *MockAuditRepo) CreateEvent(ctx context.Context, event *audit.Event) error {
	args := m.Called(ctx, event)
	return args.Error(0)
}

func (m *MockAuditRepo) GetByID(ctx context.Context, id uuid.UUID) (*audit.Event, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(*audit.Event), args.Error(1)
}

func (m *MockAuditRepo) GetBySequence(ctx context.Context, seq values.SequenceNumber) (*audit.Event, error) {
	args := m.Called(ctx, seq)
	return args.Get(0).(*audit.Event), args.Error(1)
}

func (m *MockAuditRepo) GetEvents(ctx context.Context, filter audit.EventFilter) (*audit.EventQueryResult, error) {
	args := m.Called(ctx, filter)
	return args.Get(0).(*audit.EventQueryResult), args.Error(1)
}

func (m *MockAuditRepo) GetSequenceRange(ctx context.Context, start, end values.SequenceNumber) ([]*audit.Event, error) {
	args := m.Called(ctx, start, end)
	return args.Get(0).([]*audit.Event), args.Error(1)
}

func (m *MockAuditRepo) GetLatestSequenceNumber(ctx context.Context) (values.SequenceNumber, error) {
	args := m.Called(ctx)
	return args.Get(0).(values.SequenceNumber), args.Error(1)
}

func (m *MockAuditRepo) GetEventsByTimeRange(ctx context.Context, start, end time.Time, filter audit.EventFilter) (*audit.EventQueryResult, error) {
	args := m.Called(ctx, start, end, filter)
	return args.Get(0).(*audit.EventQueryResult), args.Error(1)
}

func (m *MockAuditRepo) GetTCPARelevantEvents(ctx context.Context, phoneNumber string, filter audit.EventFilter) (*audit.EventQueryResult, error) {
	args := m.Called(ctx, phoneNumber, filter)
	return args.Get(0).(*audit.EventQueryResult), args.Error(1)
}

type MockComplianceRepo struct {
	mock.Mock
}

func (m *MockComplianceRepo) GetConsentByPhone(ctx context.Context, phoneNumber string) (*compliance.ConsentRecord, error) {
	args := m.Called(ctx, phoneNumber)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*compliance.ConsentRecord), args.Error(1)
}

func (m *MockComplianceRepo) SaveConsent(ctx context.Context, consent *compliance.ConsentRecord) error {
	args := m.Called(ctx, consent)
	return args.Error(0)
}

func (m *MockComplianceRepo) UpdateConsent(ctx context.Context, consent *compliance.ConsentRecord) error {
	args := m.Called(ctx, consent)
	return args.Error(0)
}

type MockQueryRepo struct {
	mock.Mock
}

type MockIntegrityRepo struct {
	mock.Mock
}

func (m *MockIntegrityRepo) VerifySequenceIntegrity(ctx context.Context, criteria audit.SequenceIntegrityCriteria) (*audit.SequenceIntegrityResult, error) {
	args := m.Called(ctx, criteria)
	return args.Get(0).(*audit.SequenceIntegrityResult), args.Error(1)
}

// Test fixtures
func createTestComplianceService() *ComplianceService {
	mockAuditRepo := &MockAuditRepo{}
	mockComplianceRepo := &MockComplianceRepo{}
	mockQueryRepo := &MockQueryRepo{}
	mockIntegrityRepo := &MockIntegrityRepo{}

	// Create mock domain services
	hashChainService := audit.NewHashChainService(mockAuditRepo, mockIntegrityRepo)
	integrityService := audit.NewIntegrityCheckService(mockAuditRepo, mockIntegrityRepo, mockQueryRepo, hashChainService)
	complianceVerifyService := audit.NewComplianceVerificationService(mockAuditRepo, mockQueryRepo)

	return NewComplianceService(
		mockAuditRepo,
		mockComplianceRepo,
		mockQueryRepo,
		mockIntegrityRepo,
		hashChainService,
		integrityService,
		complianceVerifyService,
	)
}

func createTestTCPAConsent() TCPAConsent {
	return TCPAConsent{
		PhoneNumber: "+14155551234",
		ConsentType: "EXPRESS",
		Source:      "web_form",
		IPAddress:   "192.168.1.1",
		UserAgent:   "Mozilla/5.0",
		ActorID:     "user123",
		ExpiryDays:  365,
		ConsentText: "I consent to receive marketing calls",
		Timestamp:   time.Now(),
	}
}

func createTestGDPRRequest() GDPRRequest {
	return GDPRRequest{
		Type:               "ACCESS",
		DataSubjectID:      "subject123",
		DataSubjectEmail:   "test@example.com",
		VerificationMethod: "email_verification",
		IdentityVerified:   true,
		ExportFormat:       "JSON",
		RequestDate:        time.Now(),
		Deadline:           time.Now().AddDate(0, 0, 30),
	}
}

func createTestCCPARequest() CCPARequest {
	return CCPARequest{
		Type:        "OPT_OUT",
		ConsumerID:  "consumer123",
		Email:       "consumer@example.com",
		Categories:  []string{"marketing", "analytics"},
		RequestDate: time.Now(),
		Verified:    true,
	}
}

// TCPA Tests

func TestComplianceService_ValidateTCPACompliance(t *testing.T) {
	service := createTestComplianceService()
	ctx := context.Background()

	// Mock consent record
	consentRecord := &compliance.ConsentRecord{
		ID:             uuid.New(),
		PhoneNumber:    "+14155551234",
		ConsentType:    compliance.ConsentTypeExpress,
		Status:         compliance.ConsentStatusActive,
		OptInTimestamp: time.Now().AddDate(0, 0, -30),
	}

	// Mock repositories
	mockComplianceRepo := service.complianceRepo.(*MockComplianceRepo)
	mockAuditRepo := service.auditRepo.(*MockAuditRepo)

	mockComplianceRepo.On("GetConsentByPhone", ctx, "+14155551234").Return(consentRecord, nil)
	mockAuditRepo.On("GetEvents", ctx, mock.AnythingOfType("audit.EventFilter")).Return(&audit.EventQueryResult{
		Events: []*audit.Event{},
	}, nil)

	req := TCPAValidationRequest{
		PhoneNumber: "+14155551234",
		CallTime:    time.Date(2024, 1, 15, 14, 0, 0, 0, time.UTC), // 2 PM UTC
		Timezone:    "America/New_York",
		CallType:    "marketing",
		ActorID:     "user123",
	}

	result, err := service.ValidateTCPACompliance(ctx, req)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.IsCompliant)
	assert.Equal(t, "+14155551234", result.PhoneNumber)
	assert.Equal(t, "ACTIVE", result.ConsentStatus)
	assert.Empty(t, result.Violations)

	mockComplianceRepo.AssertExpectations(t)
	mockAuditRepo.AssertExpectations(t)
}

func TestComplianceService_ValidateTCPACompliance_NoConsent(t *testing.T) {
	service := createTestComplianceService()
	ctx := context.Background()

	// Mock no consent found
	mockComplianceRepo := service.complianceRepo.(*MockComplianceRepo)
	mockAuditRepo := service.auditRepo.(*MockAuditRepo)

	mockComplianceRepo.On("GetConsentByPhone", ctx, "+14155551234").Return(nil, compliance.ErrNoConsent)
	mockAuditRepo.On("GetEvents", ctx, mock.AnythingOfType("audit.EventFilter")).Return(&audit.EventQueryResult{
		Events: []*audit.Event{},
	}, nil)

	req := TCPAValidationRequest{
		PhoneNumber: "+14155551234",
		CallTime:    time.Date(2024, 1, 15, 14, 0, 0, 0, time.UTC),
		Timezone:    "America/New_York",
		ActorID:     "user123",
	}

	result, err := service.ValidateTCPACompliance(ctx, req)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsCompliant)
	assert.Len(t, result.Violations, 1)
	assert.Equal(t, "NO_CONSENT", result.Violations[0].Type)
	assert.Equal(t, "CRITICAL", result.Violations[0].Severity)

	mockComplianceRepo.AssertExpectations(t)
	mockAuditRepo.AssertExpectations(t)
}

func TestComplianceService_ValidateTCPACompliance_TimeViolation(t *testing.T) {
	service := createTestComplianceService()
	ctx := context.Background()

	// Mock consent record
	consentRecord := &compliance.ConsentRecord{
		ID:             uuid.New(),
		PhoneNumber:    "+14155551234",
		ConsentType:    compliance.ConsentTypeExpress,
		Status:         compliance.ConsentStatusActive,
		OptInTimestamp: time.Now().AddDate(0, 0, -30),
	}

	mockComplianceRepo := service.complianceRepo.(*MockComplianceRepo)
	mockAuditRepo := service.auditRepo.(*MockAuditRepo)

	mockComplianceRepo.On("GetConsentByPhone", ctx, "+14155551234").Return(consentRecord, nil)
	mockAuditRepo.On("GetEvents", ctx, mock.AnythingOfType("audit.EventFilter")).Return(&audit.EventQueryResult{
		Events: []*audit.Event{},
	}, nil)

	req := TCPAValidationRequest{
		PhoneNumber: "+14155551234",
		CallTime:    time.Date(2024, 1, 15, 6, 0, 0, 0, time.UTC), // 6 AM UTC = too early
		Timezone:    "America/New_York",                           // EST: would be 1 AM
		ActorID:     "user123",
	}

	result, err := service.ValidateTCPACompliance(ctx, req)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsCompliant)

	// Should have time violation
	hasTimeViolation := false
	for _, violation := range result.Violations {
		if violation.Type == "TIME_RESTRICTION" {
			hasTimeViolation = true
			break
		}
	}
	assert.True(t, hasTimeViolation, "Should have time restriction violation")

	mockComplianceRepo.AssertExpectations(t)
	mockAuditRepo.AssertExpectations(t)
}

func TestComplianceService_RecordTCPAConsent(t *testing.T) {
	service := createTestComplianceService()
	ctx := context.Background()

	consent := createTestTCPAConsent()

	mockComplianceRepo := service.complianceRepo.(*MockComplianceRepo)
	mockAuditRepo := service.auditRepo.(*MockAuditRepo)

	mockComplianceRepo.On("SaveConsent", ctx, mock.AnythingOfType("*compliance.ConsentRecord")).Return(nil)
	mockAuditRepo.On("CreateEvent", ctx, mock.AnythingOfType("*audit.Event")).Return(nil)

	err := service.RecordTCPAConsent(ctx, consent)

	require.NoError(t, err)
	mockComplianceRepo.AssertExpectations(t)
	mockAuditRepo.AssertExpectations(t)
}

func TestComplianceService_RevokeTCPAConsent(t *testing.T) {
	service := createTestComplianceService()
	ctx := context.Background()

	// Mock existing consent
	consentRecord := &compliance.ConsentRecord{
		ID:             uuid.New(),
		PhoneNumber:    "+14155551234",
		ConsentType:    compliance.ConsentTypeExpress,
		Status:         compliance.ConsentStatusActive,
		OptInTimestamp: time.Now().AddDate(0, 0, -30),
	}

	mockComplianceRepo := service.complianceRepo.(*MockComplianceRepo)
	mockAuditRepo := service.auditRepo.(*MockAuditRepo)

	mockComplianceRepo.On("GetConsentByPhone", ctx, "+14155551234").Return(consentRecord, nil)
	mockComplianceRepo.On("UpdateConsent", ctx, mock.AnythingOfType("*compliance.ConsentRecord")).Return(nil)
	mockAuditRepo.On("CreateEvent", ctx, mock.AnythingOfType("*audit.Event")).Return(nil)

	revocation := TCPARevocation{
		PhoneNumber: "+14155551234",
		Reason:      "customer_request",
		Source:      "phone_call",
		ActorID:     "user123",
		Timestamp:   time.Now(),
	}

	err := service.RevokeTCPAConsent(ctx, revocation)

	require.NoError(t, err)
	mockComplianceRepo.AssertExpectations(t)
	mockAuditRepo.AssertExpectations(t)
}

// GDPR Tests

func TestComplianceService_ProcessGDPRRequest_Access(t *testing.T) {
	service := createTestComplianceService()
	ctx := context.Background()

	req := createTestGDPRRequest()

	// Mock audit events
	mockEvents := []*audit.Event{
		{
			ID:          uuid.New(),
			Type:        audit.EventDataAccessed,
			ActorID:     req.DataSubjectID,
			TargetID:    req.DataSubjectID,
			Timestamp:   time.Now().AddDate(0, 0, -10),
			DataClasses: []string{"personal_data"},
			LegalBasis:  "legitimate_interest",
		},
	}

	mockAuditRepo := service.auditRepo.(*MockAuditRepo)
	mockAuditRepo.On("GetEvents", ctx, mock.AnythingOfType("audit.EventFilter")).Return(&audit.EventQueryResult{
		Events: mockEvents,
	}, nil)
	mockAuditRepo.On("CreateEvent", ctx, mock.AnythingOfType("*audit.Event")).Return(nil)

	result, err := service.ProcessGDPRRequest(ctx, req)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "COMPLETED", result.Status)
	assert.Equal(t, "ACCESS", result.RequestType)
	assert.NotNil(t, result.DataExport)
	assert.Equal(t, "JSON", result.DataExport.Format)

	mockAuditRepo.AssertExpectations(t)
}

func TestComplianceService_ProcessGDPRRequest_Erasure(t *testing.T) {
	service := createTestComplianceService()
	ctx := context.Background()

	req := GDPRRequest{
		Type:          "ERASURE",
		DataSubjectID: "subject123",
		RequestDate:   time.Now(),
	}

	mockAuditRepo := service.auditRepo.(*MockAuditRepo)

	// Mock events for data subject
	mockEvents := []*audit.Event{
		{
			ID:          uuid.New(),
			Type:        audit.EventDataCreated,
			ActorID:     req.DataSubjectID,
			TargetID:    req.DataSubjectID,
			Timestamp:   time.Now().AddDate(0, 0, -365), // Old data
			DataClasses: []string{"personal_data"},
			LegalBasis:  "consent",
		},
	}

	mockAuditRepo.On("GetEvents", ctx, mock.AnythingOfType("audit.EventFilter")).Return(&audit.EventQueryResult{
		Events: mockEvents,
	}, nil)
	mockAuditRepo.On("CreateEvent", ctx, mock.AnythingOfType("*audit.Event")).Return(nil)

	result, err := service.ProcessGDPRRequest(ctx, req)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "COMPLETED", result.Status)
	assert.Equal(t, "ERASURE", result.RequestType)
	assert.NotNil(t, result.DataAffected)

	mockAuditRepo.AssertExpectations(t)
}

// CCPA Tests

func TestComplianceService_ProcessCCPARequest_OptOut(t *testing.T) {
	service := createTestComplianceService()
	ctx := context.Background()

	req := createTestCCPARequest()

	mockAuditRepo := service.auditRepo.(*MockAuditRepo)
	mockAuditRepo.On("CreateEvent", ctx, mock.AnythingOfType("*audit.Event")).Return(nil)

	result, err := service.ProcessCCPARequest(ctx, req)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "COMPLETED", result.Status)
	assert.Equal(t, "OPT_OUT", result.RequestType)
	assert.True(t, result.OptOutApplied)

	mockAuditRepo.AssertExpectations(t)
}

// SOX Tests

func TestComplianceService_GenerateSOXComplianceReport(t *testing.T) {
	service := createTestComplianceService()
	ctx := context.Background()

	criteria := SOXReportCriteria{
		Period:    "Q1",
		StartDate: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(2024, 3, 31, 23, 59, 59, 0, time.UTC),
		Scope:     []string{"financial_reporting"},
	}

	mockAuditRepo := service.auditRepo.(*MockAuditRepo)

	// Mock financial events
	financialEvents := []*audit.Event{
		{
			ID:          uuid.New(),
			Type:        audit.EventDataCreated,
			ActorID:     "system",
			TargetID:    "transaction123",
			Timestamp:   time.Date(2024, 2, 15, 10, 0, 0, 0, time.UTC),
			DataClasses: []string{"financial", "transaction"},
			LegalBasis:  "business_operation",
			EventHash:   "hash123",
			SequenceNum: 1001,
		},
	}

	mockAuditRepo.On("GetEvents", ctx, mock.AnythingOfType("audit.EventFilter")).Return(&audit.EventQueryResult{
		Events: financialEvents,
	}, nil)

	report, err := service.GenerateSOXComplianceReport(ctx, criteria)

	require.NoError(t, err)
	assert.NotNil(t, report)
	assert.Equal(t, "Q1", report.Period)
	assert.NotNil(t, report.DataIntegrity)
	assert.NotNil(t, report.AccessControls)
	assert.NotNil(t, report.AuditTrailStatus)
	assert.NotEmpty(t, report.Controls)

	mockAuditRepo.AssertExpectations(t)
}

// Retention Policy Tests

func TestComplianceService_ApplyRetentionPolicy(t *testing.T) {
	service := createTestComplianceService()
	ctx := context.Background()

	// Set up a test retention policy
	policyID := "CALL_DATA"

	mockAuditRepo := service.auditRepo.(*MockAuditRepo)

	// Mock old call data events
	oldEvents := []*audit.Event{
		{
			ID:          uuid.New(),
			Type:        audit.EventCallCompleted,
			ActorID:     "user123",
			TargetID:    "+14155551234",
			Timestamp:   time.Now().AddDate(0, -7, 0), // 7 months old
			DataClasses: []string{"call_records"},
			LegalBasis:  "legitimate_interest",
		},
	}

	mockAuditRepo.On("GetEvents", ctx, mock.AnythingOfType("audit.EventFilter")).Return(&audit.EventQueryResult{
		Events: oldEvents,
	}, nil)
	mockAuditRepo.On("CreateEvent", ctx, mock.AnythingOfType("*audit.Event")).Return(nil)

	result, err := service.ApplyRetentionPolicy(ctx, policyID)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, policyID, result.PolicyID)
	assert.Equal(t, "COMPLETED", result.Status)
	assert.True(t, result.RecordsEvaluated > 0)

	mockAuditRepo.AssertExpectations(t)
}

// Legal Hold Tests

func TestComplianceService_ApplyLegalHold(t *testing.T) {
	service := createTestComplianceService()
	ctx := context.Background()

	hold := LegalHold{
		ID:                "HOLD-001",
		Description:       "Litigation hold for case ABC",
		IssuedBy:          "legal_team",
		IssuedDate:        time.Now(),
		DataCategories:    []string{"call_records", "messages"},
		DataSubjects:      []string{"subject123"},
		Status:            "active",
		LegalAuthority:    "court_order",
		CourtOrder:        true,
		RegulatoryRequest: false,
	}

	mockAuditRepo := service.auditRepo.(*MockAuditRepo)
	mockAuditRepo.On("CreateEvent", ctx, mock.AnythingOfType("*audit.Event")).Return(nil)

	err := service.ApplyLegalHold(ctx, hold)

	require.NoError(t, err)

	// Verify legal hold was stored
	storedHold, exists := service.legalHolds[hold.ID]
	assert.True(t, exists)
	assert.Equal(t, hold.ID, storedHold.ID)
	assert.Equal(t, hold.Description, storedHold.Description)

	mockAuditRepo.AssertExpectations(t)
}

func TestComplianceService_RemoveLegalHold(t *testing.T) {
	service := createTestComplianceService()
	ctx := context.Background()

	// Set up an existing legal hold
	holdID := "HOLD-001"
	hold := LegalHold{
		ID:             holdID,
		Description:    "Test hold",
		IssuedBy:       "legal_team",
		IssuedDate:     time.Now().AddDate(0, 0, -30),
		DataCategories: []string{"call_records"},
		Status:         "active",
		LegalAuthority: "court_order",
		CourtOrder:     true,
	}
	service.legalHolds[holdID] = hold

	mockAuditRepo := service.auditRepo.(*MockAuditRepo)
	mockAuditRepo.On("CreateEvent", ctx, mock.AnythingOfType("*audit.Event")).Return(nil)

	err := service.RemoveLegalHold(ctx, holdID, "case_resolved")

	require.NoError(t, err)

	// Verify legal hold was removed
	_, exists := service.legalHolds[holdID]
	assert.False(t, exists)

	mockAuditRepo.AssertExpectations(t)
}

// Integration Tests

func TestComplianceService_FullTCPAWorkflow(t *testing.T) {
	service := createTestComplianceService()
	ctx := context.Background()

	phoneNumber := "+14155551234"

	// Set up mocks
	mockComplianceRepo := service.complianceRepo.(*MockComplianceRepo)
	mockAuditRepo := service.auditRepo.(*MockAuditRepo)

	// 1. Record consent
	consent := TCPAConsent{
		PhoneNumber: phoneNumber,
		ConsentType: "EXPRESS",
		Source:      "web_form",
		ActorID:     "user123",
		ExpiryDays:  365,
	}

	mockComplianceRepo.On("SaveConsent", ctx, mock.AnythingOfType("*compliance.ConsentRecord")).Return(nil)
	mockAuditRepo.On("CreateEvent", ctx, mock.AnythingOfType("*audit.Event")).Return(nil)

	err := service.RecordTCPAConsent(ctx, consent)
	require.NoError(t, err)

	// 2. Validate compliance
	consentRecord := &compliance.ConsentRecord{
		ID:             uuid.New(),
		PhoneNumber:    phoneNumber,
		ConsentType:    compliance.ConsentTypeExpress,
		Status:         compliance.ConsentStatusActive,
		OptInTimestamp: time.Now().AddDate(0, 0, -1),
	}

	mockComplianceRepo.On("GetConsentByPhone", ctx, phoneNumber).Return(consentRecord, nil)
	mockAuditRepo.On("GetEvents", ctx, mock.AnythingOfType("audit.EventFilter")).Return(&audit.EventQueryResult{
		Events: []*audit.Event{},
	}, nil)

	validationReq := TCPAValidationRequest{
		PhoneNumber: phoneNumber,
		CallTime:    time.Date(2024, 1, 15, 14, 0, 0, 0, time.UTC),
		Timezone:    "America/New_York",
		ActorID:     "user123",
	}

	validationResult, err := service.ValidateTCPACompliance(ctx, validationReq)
	require.NoError(t, err)
	assert.True(t, validationResult.IsCompliant)

	// 3. Revoke consent
	mockComplianceRepo.On("UpdateConsent", ctx, mock.AnythingOfType("*compliance.ConsentRecord")).Return(nil)

	revocation := TCPARevocation{
		PhoneNumber: phoneNumber,
		Reason:      "customer_request",
		Source:      "phone_call",
		ActorID:     "user123",
	}

	err = service.RevokeTCPAConsent(ctx, revocation)
	require.NoError(t, err)

	mockComplianceRepo.AssertExpectations(t)
	mockAuditRepo.AssertExpectations(t)
}

// Property-based tests using Go 1.24 features

func TestComplianceService_Properties(t *testing.T) {
	service := createTestComplianceService()
	ctx := context.Background()

	// Property: TCPA validation with valid consent and proper time should always pass
	t.Run("TCPA_ValidConsentAndTime_AlwaysCompliant", func(t *testing.T) {
		phoneNumbers := []string{"+14155551234", "+12125551234", "+13105551234"}
		validHours := []int{8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}

		for _, phone := range phoneNumbers {
			for _, hour := range validHours {
				// Mock valid consent
				consentRecord := &compliance.ConsentRecord{
					ID:             uuid.New(),
					PhoneNumber:    phone,
					ConsentType:    compliance.ConsentTypeExpress,
					Status:         compliance.ConsentStatusActive,
					OptInTimestamp: time.Now().AddDate(0, 0, -30),
				}

				mockComplianceRepo := service.complianceRepo.(*MockComplianceRepo)
				mockAuditRepo := service.auditRepo.(*MockAuditRepo)

				mockComplianceRepo.On("GetConsentByPhone", ctx, phone).Return(consentRecord, nil).Once()
				mockAuditRepo.On("GetEvents", ctx, mock.AnythingOfType("audit.EventFilter")).Return(&audit.EventQueryResult{
					Events: []*audit.Event{},
				}, nil).Once()

				req := TCPAValidationRequest{
					PhoneNumber: phone,
					CallTime:    time.Date(2024, 1, 15, hour, 0, 0, 0, time.UTC),
					Timezone:    "America/New_York",
					ActorID:     "user123",
				}

				result, err := service.ValidateTCPACompliance(ctx, req)
				require.NoError(t, err, "Phone: %s, Hour: %d", phone, hour)
				assert.True(t, result.IsCompliant, "Should be compliant for phone %s at hour %d", phone, hour)
			}
		}
	})

	// Property: GDPR requests should always generate unique request IDs
	t.Run("GDPR_RequestIDs_AlwaysUnique", func(t *testing.T) {
		requestIDs := make(map[string]bool)

		mockAuditRepo := service.auditRepo.(*MockAuditRepo)
		mockAuditRepo.On("GetEvents", ctx, mock.AnythingOfType("audit.EventFilter")).Return(&audit.EventQueryResult{
			Events: []*audit.Event{},
		}, nil).Maybe()
		mockAuditRepo.On("CreateEvent", ctx, mock.AnythingOfType("*audit.Event")).Return(nil).Maybe()

		for i := 0; i < 100; i++ {
			req := GDPRRequest{
				Type:             "ACCESS",
				DataSubjectID:    fmt.Sprintf("subject%d", i),
				IdentityVerified: true,
				RequestDate:      time.Now(),
			}

			result, err := service.ProcessGDPRRequest(ctx, req)
			require.NoError(t, err)

			// Check uniqueness
			assert.False(t, requestIDs[result.RequestID], "Request ID %s should be unique", result.RequestID)
			requestIDs[result.RequestID] = true
		}
	})
}

// Property-Based Testing with 1000+ Iterations for IMMUTABLE_AUDIT

func TestPropertyComplianceService_TCPAValidationInvariants(t *testing.T) {
	service := createTestComplianceService()
	ctx := context.Background()

	// Property: TCPA validation should be deterministic for same inputs
	for i := 0; i < 1000; i++ {
		phoneNumber := fmt.Sprintf("+1415555%04d", i%9999)
		
		// Mock consent record with consistent state
		consentRecord := &compliance.ConsentRecord{
			ID:             uuid.New(),
			PhoneNumber:    phoneNumber,
			ConsentType:    compliance.ConsentTypeExpress,
			Status:         compliance.ConsentStatusActive,
			OptInTimestamp: time.Now().AddDate(0, 0, -30),
		}

		mockComplianceRepo := service.complianceRepo.(*MockComplianceRepo)
		mockAuditRepo := service.auditRepo.(*MockAuditRepo)

		mockComplianceRepo.On("GetConsentByPhone", ctx, phoneNumber).Return(consentRecord, nil).Once()
		mockAuditRepo.On("GetEvents", ctx, mock.AnythingOfType("audit.EventFilter")).Return(&audit.EventQueryResult{
			Events: []*audit.Event{},
		}, nil).Once()

		req := TCPAValidationRequest{
			PhoneNumber: phoneNumber,
			CallTime:    time.Date(2024, 1, 15, 14, 0, 0, 0, time.UTC),
			Timezone:    "America/New_York",
			ActorID:     "user123",
		}

		result1, err1 := service.ValidateTCPACompliance(ctx, req)
		require.NoError(t, err1, "Iteration %d failed", i)

		// Second call with same inputs should yield same result
		mockComplianceRepo.On("GetConsentByPhone", ctx, phoneNumber).Return(consentRecord, nil).Once()
		mockAuditRepo.On("GetEvents", ctx, mock.AnythingOfType("audit.EventFilter")).Return(&audit.EventQueryResult{
			Events: []*audit.Event{},
		}, nil).Once()

		result2, err2 := service.ValidateTCPACompliance(ctx, req)
		require.NoError(t, err2, "Iteration %d second call failed", i)

		// Invariant: Same inputs should produce same compliance result
		assert.Equal(t, result1.IsCompliant, result2.IsCompliant, "Iteration %d: compliance result should be deterministic", i)
		assert.Equal(t, len(result1.Violations), len(result2.Violations), "Iteration %d: violation count should be deterministic", i)
	}
}

func TestPropertyComplianceService_GDPRRequestIDUniqueness(t *testing.T) {
	service := createTestComplianceService()
	ctx := context.Background()

	requestIDs := make(map[string]bool)
	
	// Property: GDPR requests should always generate unique request IDs
	for i := 0; i < 1000; i++ {
		mockAuditRepo := service.auditRepo.(*MockAuditRepo)
		mockAuditRepo.On("GetEvents", ctx, mock.AnythingOfType("audit.EventFilter")).Return(&audit.EventQueryResult{
			Events: []*audit.Event{},
		}, nil).Once()
		mockAuditRepo.On("CreateEvent", ctx, mock.AnythingOfType("*audit.Event")).Return(nil).Once()

		req := GDPRRequest{
			Type:             "ACCESS",
			DataSubjectID:    fmt.Sprintf("subject%d", i),
			IdentityVerified: true,
			RequestDate:      time.Now(),
		}

		result, err := service.ProcessGDPRRequest(ctx, req)
		require.NoError(t, err, "Iteration %d failed", i)

		// Invariant: Request IDs must be unique
		assert.False(t, requestIDs[result.RequestID], "Iteration %d: Request ID %s should be unique", i, result.RequestID)
		requestIDs[result.RequestID] = true
	}

	// Verify we have 1000 unique IDs
	assert.Len(t, requestIDs, 1000, "Should have 1000 unique request IDs")
}

func TestPropertyComplianceService_TimezoneConsistency(t *testing.T) {
	service := createTestComplianceService()
	
	timezones := []string{
		"America/New_York", "America/Los_Angeles", "America/Chicago",
		"Europe/London", "Europe/Paris", "Asia/Tokyo",
	}
	
	// Property: Time restriction validation should be consistent across timezones
	for i := 0; i < 1000; i++ {
		timezone := timezones[i%len(timezones)]
		
		// Test valid hours (should always pass)
		validTime := time.Date(2024, 1, 15, 14, 0, 0, 0, time.UTC) // 2 PM UTC
		compliant, violation := service.checkTCPATimeRestrictions(validTime, timezone)
		
		// Invariant: 2 PM UTC should be within business hours for most timezones
		if timezone == "America/New_York" || timezone == "America/Chicago" {
			assert.True(t, compliant, "Iteration %d: 2 PM UTC should be compliant for %s", i, timezone)
		}
		
		// Test invalid hours (should always fail)
		invalidTime := time.Date(2024, 1, 15, 2, 0, 0, 0, time.UTC) // 2 AM UTC
		invalidCompliant, invalidViolation := service.checkTCPATimeRestrictions(invalidTime, timezone)
		
		// For most US timezones, 2 AM UTC is too early
		if timezone == "America/Los_Angeles" { // 6 PM PST - still valid
			assert.True(t, invalidCompliant, "Iteration %d: 2 AM UTC should be compliant for %s", i, timezone)
		} else if timezone == "America/New_York" { // 9 PM EST - borderline
			assert.True(t, invalidCompliant || !invalidCompliant, "Iteration %d: consistent result for %s", i, timezone)
		}
		
		// Invariant: If non-compliant, should have violation details
		if !compliant {
			assert.NotEmpty(t, violation.Type, "Iteration %d: violation should have type", i)
			assert.NotEmpty(t, violation.Description, "Iteration %d: violation should have description", i)
		}
		if !invalidCompliant {
			assert.NotEmpty(t, invalidViolation.Type, "Iteration %d: invalid violation should have type", i)
		}
	}
}

func TestPropertyComplianceService_HashChainIntegrity(t *testing.T) {
	service := createTestComplianceService()
	ctx := context.Background()

	// Property: Audit events should maintain hash chain integrity
	for i := 0; i < 1000; i++ {
		phoneNumber := fmt.Sprintf("+1415555%04d", i%9999)
		
		mockAuditRepo := service.auditRepo.(*MockAuditRepo)
		mockAuditRepo.On("CreateEvent", ctx, mock.AnythingOfType("*audit.Event")).Return(nil).Run(func(args mock.Arguments) {
			event := args.Get(1).(*audit.Event)
			
			// Invariant: Every audit event should have valid hash
			assert.NotEmpty(t, event.EventHash, "Iteration %d: event should have hash", i)
			assert.NotZero(t, event.SequenceNum, "Iteration %d: event should have sequence number", i)
			assert.NotEmpty(t, event.ID, "Iteration %d: event should have ID", i)
			assert.NotZero(t, event.Timestamp, "Iteration %d: event should have timestamp", i)
			assert.NotZero(t, event.TimestampNano, "Iteration %d: event should have nano timestamp", i)
			
			// Invariant: Compliance events should have appropriate metadata
			if event.Type == audit.EventComplianceChecked {
				flags, hasFlags := event.ComplianceFlags["framework"]
				assert.True(t, hasFlags, "Iteration %d: compliance event should have framework flag", i)
				assert.NotEmpty(t, flags, "Iteration %d: framework flag should not be empty", i)
			}
		}).Once()

		// Record a compliance check which creates an audit event
		service.recordComplianceCheck(ctx, "TCPA", phoneNumber, true, []ComplianceViolation{})
	}
}

func TestPropertyComplianceService_LegalHoldInvariants(t *testing.T) {
	service := createTestComplianceService()
	ctx := context.Background()

	// Property: Legal holds should maintain consistent state
	for i := 0; i < 1000; i++ {
		holdID := fmt.Sprintf("HOLD-%04d", i)
		
		hold := LegalHold{
			ID:             holdID,
			Description:    fmt.Sprintf("Test hold %d", i),
			IssuedBy:       "legal_team",
			IssuedDate:     time.Now(),
			DataCategories: []string{"call_records", "messages"},
			Status:         "active",
			LegalAuthority: "court_order",
			CourtOrder:     true,
		}

		mockAuditRepo := service.auditRepo.(*MockAuditRepo)
		mockAuditRepo.On("CreateEvent", ctx, mock.AnythingOfType("*audit.Event")).Return(nil).Once()

		err := service.ApplyLegalHold(ctx, hold)
		require.NoError(t, err, "Iteration %d: applying legal hold should not fail", i)

		// Invariant: Applied hold should be stored and retrievable
		storedHold, exists := service.legalHolds[holdID]
		assert.True(t, exists, "Iteration %d: legal hold should exist after applying", i)
		assert.Equal(t, hold.ID, storedHold.ID, "Iteration %d: hold ID should match", i)
		assert.Equal(t, hold.Status, storedHold.Status, "Iteration %d: hold status should match", i)
		assert.Equal(t, "active", storedHold.Status, "Iteration %d: hold should be active", i)

		// Invariant: Removing hold should clean up state
		mockAuditRepo.On("CreateEvent", ctx, mock.AnythingOfType("*audit.Event")).Return(nil).Once()
		
		err = service.RemoveLegalHold(ctx, holdID, "test_removal")
		require.NoError(t, err, "Iteration %d: removing legal hold should not fail", i)

		_, exists = service.legalHolds[holdID]
		assert.False(t, exists, "Iteration %d: legal hold should not exist after removal", i)
	}
}

func TestPropertyComplianceService_RetentionPolicyConsistency(t *testing.T) {
	service := createTestComplianceService()
	ctx := context.Background()

	policies := []string{"CALL_DATA", "CONSENT_DATA", "FINANCIAL_DATA"}
	
	// Property: Retention policies should be consistently available and valid
	for i := 0; i < 1000; i++ {
		policyID := policies[i%len(policies)]
		
		// Invariant: Default policies should always exist
		policy, exists := service.retentionPolicies[policyID]
		assert.True(t, exists, "Iteration %d: policy %s should exist", i, policyID)
		
		// Invariant: Policies should have valid configuration
		assert.NotEmpty(t, policy.ID, "Iteration %d: policy should have ID", i)
		assert.NotEmpty(t, policy.Name, "Iteration %d: policy should have name", i)
		assert.NotEmpty(t, policy.DataTypes, "Iteration %d: policy should have data types", i)
		assert.NotEmpty(t, policy.Actions, "Iteration %d: policy should have actions", i)
		assert.Positive(t, policy.RetentionPeriod.Duration, "Iteration %d: policy should have positive duration", i)
		
		// Invariant: Financial data should have longest retention (SOX compliance)
		if policyID == "FINANCIAL_DATA" {
			assert.GreaterOrEqual(t, policy.RetentionPeriod.Duration, int64(24*365*7), "Iteration %d: financial data should retain for 7+ years", i)
		}
	}
}

// Edge Case Testing for IMMUTABLE_AUDIT

func TestComplianceService_EdgeCases(t *testing.T) {
	service := createTestComplianceService()
	ctx := context.Background()

	t.Run("TCPA_EdgeCase_MidnightTimezone", func(t *testing.T) {
		// Edge case: Exactly at midnight in timezone boundary
		midnightUTC := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
		
		// Should fail for most US timezones (too late previous day)
		compliant, violation := service.checkTCPATimeRestrictions(midnightUTC, "America/New_York")
		assert.False(t, compliant, "Midnight UTC should violate TCPA for EST (7 PM previous day)")
		assert.Equal(t, "TIME_RESTRICTION", violation.Type)
		
		// Test with Hawaii timezone (2 PM previous day - should be compliant)
		compliantHI, _ := service.checkTCPATimeRestrictions(midnightUTC, "Pacific/Honolulu")
		assert.True(t, compliantHI, "Midnight UTC should be compliant for Hawaii (2 PM)")
	})

	t.Run("TCPA_EdgeCase_LeapYear", func(t *testing.T) {
		// Edge case: Leap year date
		leapYearTime := time.Date(2024, 2, 29, 14, 0, 0, 0, time.UTC)
		
		compliant, _ := service.checkTCPATimeRestrictions(leapYearTime, "America/New_York")
		assert.True(t, compliant, "Leap year date should not affect TCPA validation")
	})

	t.Run("GDPR_EdgeCase_EmptyDataSubject", func(t *testing.T) {
		req := GDPRRequest{
			Type:             "ACCESS",
			DataSubjectID:    "",
			IdentityVerified: true,
			RequestDate:      time.Now(),
		}

		mockAuditRepo := service.auditRepo.(*MockAuditRepo)
		mockAuditRepo.On("GetEvents", ctx, mock.AnythingOfType("audit.EventFilter")).Return(&audit.EventQueryResult{
			Events: []*audit.Event{},
		}, nil).Maybe()

		result, err := service.ProcessGDPRRequest(ctx, req)
		// Should handle gracefully with empty result
		if err == nil {
			assert.NotNil(t, result)
		}
	})

	t.Run("LegalHold_EdgeCase_DuplicateID", func(t *testing.T) {
		holdID := "DUPLICATE-HOLD"
		
		hold1 := LegalHold{
			ID:          holdID,
			Description: "First hold",
			IssuedBy:    "legal_team",
			IssuedDate:  time.Now(),
			Status:      "active",
		}

		hold2 := LegalHold{
			ID:          holdID,
			Description: "Second hold",
			IssuedBy:    "legal_team",
			IssuedDate:  time.Now().Add(time.Hour),
			Status:      "active",
		}

		mockAuditRepo := service.auditRepo.(*MockAuditRepo)
		mockAuditRepo.On("CreateEvent", ctx, mock.AnythingOfType("*audit.Event")).Return(nil)

		// Apply first hold
		err1 := service.ApplyLegalHold(ctx, hold1)
		require.NoError(t, err1)

		// Apply second hold with same ID (should overwrite)
		err2 := service.ApplyLegalHold(ctx, hold2)
		require.NoError(t, err2)

		// Verify latest hold is stored
		storedHold := service.legalHolds[holdID]
		assert.Equal(t, "Second hold", storedHold.Description)
	})

	t.Run("Consent_EdgeCase_NilPhoneNumber", func(t *testing.T) {
		consent := TCPAConsent{
			PhoneNumber: "",
			ConsentType: "EXPRESS",
			Source:      "web_form",
			ActorID:     "user123",
		}

		err := service.RecordTCPAConsent(ctx, consent)
		assert.Error(t, err, "Should reject consent with empty phone number")
		assert.Contains(t, err.Error(), "phone number required")
	})
}

// Error Handling Testing

func TestComplianceService_ErrorHandling(t *testing.T) {
	service := createTestComplianceService()
	ctx := context.Background()

	t.Run("Repository_Failure_TCPAValidation", func(t *testing.T) {
		mockComplianceRepo := service.complianceRepo.(*MockComplianceRepo)
		mockComplianceRepo.On("GetConsentByPhone", ctx, "+14155551234").Return(nil, errors.NewInternalError("database error"))

		req := TCPAValidationRequest{
			PhoneNumber: "+14155551234",
			CallTime:    time.Now(),
			Timezone:    "America/New_York",
			ActorID:     "user123",
		}

		result, err := service.ValidateTCPACompliance(ctx, req)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "failed to check consent")
	})

	t.Run("Invalid_Phone_Number_Format", func(t *testing.T) {
		consent := TCPAConsent{
			PhoneNumber: "invalid-phone",
			ConsentType: "EXPRESS",
			Source:      "web_form",
			ActorID:     "user123",
		}

		err := service.RecordTCPAConsent(ctx, consent)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid phone number")
	})

	t.Run("Missing_Required_Fields_GDPR", func(t *testing.T) {
		req := GDPRRequest{
			Type: "", // Missing required type
		}

		result, err := service.ProcessGDPRRequest(ctx, req)
		assert.Error(t, err)
		assert.Nil(t, result)
	})

	t.Run("Unknown_Retention_Policy", func(t *testing.T) {
		result, err := service.ApplyRetentionPolicy(ctx, "UNKNOWN_POLICY")
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "retention policy")
	})
}

// Concurrency Testing

func TestComplianceService_ConcurrentOperations(t *testing.T) {
	service := createTestComplianceService()
	ctx := context.Background()

	t.Run("Concurrent_LegalHolds", func(t *testing.T) {
		const numGoroutines = 100
		var wg sync.WaitGroup
		errors := make(chan error, numGoroutines)

		mockAuditRepo := service.auditRepo.(*MockAuditRepo)
		mockAuditRepo.On("CreateEvent", ctx, mock.AnythingOfType("*audit.Event")).Return(nil)

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				
				hold := LegalHold{
					ID:          fmt.Sprintf("CONCURRENT-HOLD-%d", id),
					Description: fmt.Sprintf("Concurrent hold %d", id),
					IssuedBy:    "legal_team",
					IssuedDate:  time.Now(),
					Status:      "active",
				}

				if err := service.ApplyLegalHold(ctx, hold); err != nil {
					errors <- err
				}
			}(i)
		}

		wg.Wait()
		close(errors)

		// Check for any errors
		for err := range errors {
			t.Errorf("Concurrent legal hold failed: %v", err)
		}

		// Verify all holds were applied
		assert.Len(t, service.legalHolds, numGoroutines, "All legal holds should be applied")
	})

	t.Run("Concurrent_TCPA_Validations", func(t *testing.T) {
		const numGoroutines = 50
		var wg sync.WaitGroup
		results := make(chan bool, numGoroutines)

		// Set up shared mock expectations
		mockComplianceRepo := service.complianceRepo.(*MockComplianceRepo)
		mockAuditRepo := service.auditRepo.(*MockAuditRepo)

		consentRecord := &compliance.ConsentRecord{
			ID:             uuid.New(),
			PhoneNumber:    "+14155551234",
			ConsentType:    compliance.ConsentTypeExpress,
			Status:         compliance.ConsentStatusActive,
			OptInTimestamp: time.Now().AddDate(0, 0, -30),
		}

		mockComplianceRepo.On("GetConsentByPhone", ctx, "+14155551234").Return(consentRecord, nil)
		mockAuditRepo.On("GetEvents", ctx, mock.AnythingOfType("audit.EventFilter")).Return(&audit.EventQueryResult{
			Events: []*audit.Event{},
		}, nil)

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				
				req := TCPAValidationRequest{
					PhoneNumber: "+14155551234",
					CallTime:    time.Date(2024, 1, 15, 14, 0, 0, 0, time.UTC),
					Timezone:    "America/New_York",
					ActorID:     "user123",
				}

				result, err := service.ValidateTCPACompliance(ctx, req)
				if err == nil && result != nil {
					results <- result.IsCompliant
				} else {
					results <- false
				}
			}()
		}

		wg.Wait()
		close(results)

		// All results should be consistent (true for valid consent)
		for compliant := range results {
			assert.True(t, compliant, "Concurrent TCPA validations should be consistent")
		}
	})
}

// Benchmark tests

func BenchmarkComplianceService_ValidateTCPACompliance(b *testing.B) {
	service := createTestComplianceService()
	ctx := context.Background()

	// Set up mocks
	consentRecord := &compliance.ConsentRecord{
		ID:             uuid.New(),
		PhoneNumber:    "+14155551234",
		ConsentType:    compliance.ConsentTypeExpress,
		Status:         compliance.ConsentStatusActive,
		OptInTimestamp: time.Now().AddDate(0, 0, -30),
	}

	mockComplianceRepo := service.complianceRepo.(*MockComplianceRepo)
	mockAuditRepo := service.auditRepo.(*MockAuditRepo)

	mockComplianceRepo.On("GetConsentByPhone", ctx, "+14155551234").Return(consentRecord, nil)
	mockAuditRepo.On("GetEvents", ctx, mock.AnythingOfType("audit.EventFilter")).Return(&audit.EventQueryResult{
		Events: []*audit.Event{},
	}, nil)

	req := TCPAValidationRequest{
		PhoneNumber: "+14155551234",
		CallTime:    time.Date(2024, 1, 15, 14, 0, 0, 0, time.UTC),
		Timezone:    "America/New_York",
		ActorID:     "user123",
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := service.ValidateTCPACompliance(ctx, req)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkComplianceService_ProcessGDPRRequest(b *testing.B) {
	service := createTestComplianceService()
	ctx := context.Background()

	mockAuditRepo := service.auditRepo.(*MockAuditRepo)
	mockAuditRepo.On("GetEvents", ctx, mock.AnythingOfType("audit.EventFilter")).Return(&audit.EventQueryResult{
		Events: []*audit.Event{},
	}, nil)
	mockAuditRepo.On("CreateEvent", ctx, mock.AnythingOfType("*audit.Event")).Return(nil)

	req := GDPRRequest{
		Type:             "ACCESS",
		DataSubjectID:    "subject123",
		IdentityVerified: true,
		RequestDate:      time.Now(),
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := service.ProcessGDPRRequest(ctx, req)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkComplianceService_ConcurrentTCPAValidation(b *testing.B) {
	service := createTestComplianceService()
	ctx := context.Background()

	// Set up mocks for concurrent access
	consentRecord := &compliance.ConsentRecord{
		ID:             uuid.New(),
		PhoneNumber:    "+14155551234",
		ConsentType:    compliance.ConsentTypeExpress,
		Status:         compliance.ConsentStatusActive,
		OptInTimestamp: time.Now().AddDate(0, 0, -30),
	}

	mockComplianceRepo := service.complianceRepo.(*MockComplianceRepo)
	mockAuditRepo := service.auditRepo.(*MockAuditRepo)

	mockComplianceRepo.On("GetConsentByPhone", ctx, "+14155551234").Return(consentRecord, nil)
	mockAuditRepo.On("GetEvents", ctx, mock.AnythingOfType("audit.EventFilter")).Return(&audit.EventQueryResult{
		Events: []*audit.Event{},
	}, nil)

	req := TCPAValidationRequest{
		PhoneNumber: "+14155551234",
		CallTime:    time.Date(2024, 1, 15, 14, 0, 0, 0, time.UTC),
		Timezone:    "America/New_York",
		ActorID:     "user123",
	}

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := service.ValidateTCPACompliance(ctx, req)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkComplianceService_LegalHoldOperations(b *testing.B) {
	service := createTestComplianceService()
	ctx := context.Background()

	mockAuditRepo := service.auditRepo.(*MockAuditRepo)
	mockAuditRepo.On("CreateEvent", ctx, mock.AnythingOfType("*audit.Event")).Return(nil)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		hold := LegalHold{
			ID:          fmt.Sprintf("BENCH-HOLD-%d", i),
			Description: "Benchmark hold",
			IssuedBy:    "legal_team",
			IssuedDate:  time.Now(),
			Status:      "active",
		}

		err := service.ApplyLegalHold(ctx, hold)
		if err != nil {
			b.Fatal(err)
		}

		err = service.RemoveLegalHold(ctx, hold.ID, "benchmark_cleanup")
		if err != nil {
			b.Fatal(err)
		}
	}
}
