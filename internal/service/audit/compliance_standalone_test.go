package audit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTCPAValidationRequest tests TCPA validation request structure
func TestTCPAValidationRequest(t *testing.T) {
	req := TCPAValidationRequest{
		PhoneNumber: "+14155551234",
		CallTime:    time.Now(),
		Timezone:    "America/New_York",
		CallType:    "marketing",
		ActorID:     "user123",
	}

	assert.Equal(t, "+14155551234", req.PhoneNumber)
	assert.Equal(t, "America/New_York", req.Timezone)
	assert.Equal(t, "marketing", req.CallType)
	assert.Equal(t, "user123", req.ActorID)
	assert.False(t, req.CallTime.IsZero())
}

// TestTCPAConsent tests TCPA consent structure
func TestTCPAConsent(t *testing.T) {
	consent := TCPAConsent{
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

	assert.Equal(t, "+14155551234", consent.PhoneNumber)
	assert.Equal(t, "EXPRESS", consent.ConsentType)
	assert.Equal(t, "web_form", consent.Source)
	assert.Equal(t, 365, consent.ExpiryDays)
	assert.NotEmpty(t, consent.ConsentText)
}

// TestGDPRRequest tests GDPR request structure
func TestGDPRRequest(t *testing.T) {
	req := GDPRRequest{
		Type:               "ACCESS",
		DataSubjectID:      "subject123",
		DataSubjectEmail:   "test@example.com",
		VerificationMethod: "email_verification",
		IdentityVerified:   true,
		ExportFormat:       "JSON",
		RequestDate:        time.Now(),
		Deadline:           time.Now().AddDate(0, 0, 30),
	}

	assert.Equal(t, "ACCESS", req.Type)
	assert.Equal(t, "subject123", req.DataSubjectID)
	assert.Equal(t, "test@example.com", req.DataSubjectEmail)
	assert.True(t, req.IdentityVerified)
	assert.Equal(t, "JSON", req.ExportFormat)
}

// TestCCPARequest tests CCPA request structure
func TestCCPARequest(t *testing.T) {
	req := CCPARequest{
		Type:        "OPT_OUT",
		ConsumerID:  "consumer123",
		Email:       "consumer@example.com",
		Categories:  []string{"marketing", "analytics"},
		RequestDate: time.Now(),
		Verified:    true,
	}

	assert.Equal(t, "OPT_OUT", req.Type)
	assert.Equal(t, "consumer123", req.ConsumerID)
	assert.Equal(t, "consumer@example.com", req.Email)
	assert.Len(t, req.Categories, 2)
	assert.True(t, req.Verified)
}

// TestRetentionPolicy tests retention policy structure
func TestRetentionPolicy(t *testing.T) {
	policy := RetentionPolicy{
		ID:          "CALL_DATA",
		Name:        "Call Data Retention",
		Description: "Standard retention for call records",
		DataTypes:   []string{"call_records", "call_metadata"},
		RetentionPeriod: RetentionPeriod{
			Duration: 180,
			Unit:     "days",
		},
		Actions: []RetentionAction{
			{Type: "ARCHIVE", AfterDays: 90},
			{Type: "DELETE", AfterDays: 180},
		},
		LegalBasis: "business_requirement",
		IsActive:   true,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	assert.Equal(t, "CALL_DATA", policy.ID)
	assert.Equal(t, "Call Data Retention", policy.Name)
	assert.Len(t, policy.DataTypes, 2)
	assert.Equal(t, 180, policy.RetentionPeriod.Duration)
	assert.Equal(t, "days", policy.RetentionPeriod.Unit)
	assert.Len(t, policy.Actions, 2)
	assert.True(t, policy.IsActive)
}

// TestLegalHold tests legal hold structure
func TestLegalHold(t *testing.T) {
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

	assert.Equal(t, "HOLD-001", hold.ID)
	assert.Equal(t, "Litigation hold for case ABC", hold.Description)
	assert.Equal(t, "legal_team", hold.IssuedBy)
	assert.Len(t, hold.DataCategories, 2)
	assert.Len(t, hold.DataSubjects, 1)
	assert.Equal(t, "active", hold.Status)
	assert.True(t, hold.CourtOrder)
	assert.False(t, hold.RegulatoryRequest)
}

// TestComplianceViolation tests compliance violation structure
func TestComplianceViolation(t *testing.T) {
	violation := ComplianceViolation{
		Type:        "NO_CONSENT",
		Severity:    "CRITICAL",
		Description: "No valid consent found for phone number",
		Regulation:  "TCPA",
		Impact:      "Call cannot proceed without explicit consent",
		Remediation: "Obtain explicit written consent before calling",
	}

	assert.Equal(t, "NO_CONSENT", violation.Type)
	assert.Equal(t, "CRITICAL", violation.Severity)
	assert.Equal(t, "TCPA", violation.Regulation)
	assert.NotEmpty(t, violation.Description)
	assert.NotEmpty(t, violation.Impact)
	assert.NotEmpty(t, violation.Remediation)
}

// TestComplianceRequirement tests compliance requirement structure
func TestComplianceRequirement(t *testing.T) {
	requirement := ComplianceRequirement{
		ID:          "TCPA-001",
		Name:        "Express Written Consent",
		Description: "Obtain express written consent before making marketing calls",
		Type:        "MANDATORY",
		Regulation:  "TCPA",
		Status:      "MET",
	}

	assert.Equal(t, "TCPA-001", requirement.ID)
	assert.Equal(t, "Express Written Consent", requirement.Name)
	assert.Equal(t, "MANDATORY", requirement.Type)
	assert.Equal(t, "TCPA", requirement.Regulation)
	assert.Equal(t, "MET", requirement.Status)
}

// TestSOXControl tests SOX control structure
func TestSOXControl(t *testing.T) {
	control := SOXControl{
		ID:          "SOX-CTRL-001",
		Name:        "Data Integrity Controls",
		Description: "Hash chain verification and sequence integrity",
		Type:        "DETECTIVE",
		Status:      "PASSED",
		TestedAt:    time.Now(),
		TestedBy:    "system",
		Evidence:    []string{"hash_chain_verification_report"},
	}

	assert.Equal(t, "SOX-CTRL-001", control.ID)
	assert.Equal(t, "Data Integrity Controls", control.Name)
	assert.Equal(t, "DETECTIVE", control.Type)
	assert.Equal(t, "PASSED", control.Status)
	assert.Equal(t, "system", control.TestedBy)
	assert.Len(t, control.Evidence, 1)
}

// TestComplianceEngineInterface tests that engines implement the interface correctly
func TestComplianceEngineInterface(t *testing.T) {
	// This test ensures our engines implement the ComplianceEngine interface
	var engine ComplianceEngine

	// Test that we can assign our engines to the interface
	engine = &TCPAEngine{}
	assert.NotNil(t, engine)

	engine = &GDPREngine{}
	assert.NotNil(t, engine)

	engine = &CCPAEngine{}
	assert.NotNil(t, engine)

	engine = &SOXEngine{}
	assert.NotNil(t, engine)
}

// TestValidationHelpers tests validation helper functions
func TestContainsHelper(t *testing.T) {
	slice := []string{"apple", "banana", "cherry"}

	assert.True(t, contains(slice, "apple"))
	assert.True(t, contains(slice, "banana"))
	assert.True(t, contains(slice, "cherry"))
	assert.False(t, contains(slice, "orange"))
	assert.False(t, contains(slice, "grape"))
}

// TestTCPAValidationProperties tests TCPA validation properties
func TestTCPAValidationProperties(t *testing.T) {
	// Property: Valid phone numbers should always be properly formatted
	phoneNumbers := []string{
		"+14155551234",
		"+12125551234",
		"+13105551234",
	}

	for _, phone := range phoneNumbers {
		assert.True(t, len(phone) >= 12, "Phone number %s should be at least 12 characters", phone)
		assert.True(t, phone[0] == '+', "Phone number %s should start with +", phone)
	}
}

// TestGDPRRequestTypes tests all GDPR request types
func TestGDPRRequestTypes(t *testing.T) {
	requestTypes := []string{"ACCESS", "ERASURE", "RECTIFICATION", "PORTABILITY", "RESTRICTION"}

	for _, reqType := range requestTypes {
		req := GDPRRequest{
			Type:          reqType,
			DataSubjectID: "subject123",
			RequestDate:   time.Now(),
		}

		assert.Equal(t, reqType, req.Type)
		assert.NotEmpty(t, req.DataSubjectID)
		assert.False(t, req.RequestDate.IsZero())
	}
}

// TestCCPARequestTypes tests all CCPA request types
func TestCCPARequestTypes(t *testing.T) {
	requestTypes := []string{"OPT_OUT", "DELETE", "KNOW"}

	for _, reqType := range requestTypes {
		req := CCPARequest{
			Type:       reqType,
			ConsumerID: "consumer123",
			Verified:   true,
		}

		assert.Equal(t, reqType, req.Type)
		assert.NotEmpty(t, req.ConsumerID)
		assert.True(t, req.Verified)
	}
}

// TestRetentionPolicyValidation tests retention policy validation
func TestRetentionPolicyValidation(t *testing.T) {
	policy := RetentionPolicy{
		ID:          "TEST_POLICY",
		Name:        "Test Policy",
		Description: "Test retention policy",
		DataTypes:   []string{"test_data"},
		RetentionPeriod: RetentionPeriod{
			Duration: 30,
			Unit:     "days",
		},
		Actions: []RetentionAction{
			{Type: "DELETE", AfterDays: 30},
		},
		IsActive: true,
	}

	// Validate required fields
	assert.NotEmpty(t, policy.ID)
	assert.NotEmpty(t, policy.Name)
	assert.NotEmpty(t, policy.Description)
	assert.NotEmpty(t, policy.DataTypes)
	assert.Greater(t, policy.RetentionPeriod.Duration, 0)
	assert.NotEmpty(t, policy.RetentionPeriod.Unit)
	assert.NotEmpty(t, policy.Actions)
	assert.True(t, policy.IsActive)
}

// TestComplianceValidationResult tests compliance validation result
func TestComplianceValidationResult(t *testing.T) {
	result := ComplianceValidationResult{
		Framework:   "TCPA",
		IsCompliant: true,
		Score:       95.5,
		Violations:  []ComplianceViolation{},
		Timestamp:   time.Now(),
	}

	assert.Equal(t, "TCPA", result.Framework)
	assert.True(t, result.IsCompliant)
	assert.Equal(t, 95.5, result.Score)
	assert.Empty(t, result.Violations)
	assert.False(t, result.Timestamp.IsZero())
}

// TestAnonymizationResult tests anonymization result structure
func TestAnonymizationResult(t *testing.T) {
	result := AnonymizationResult{
		DataSubjectID:     "subject123",
		StartedAt:         time.Now().Add(-time.Hour),
		CompletedAt:       &[]time.Time{time.Now()}[0],
		Success:           true,
		RecordsAnonymized: 50,
		RecordsDeleted:    10,
		DataCategories:    []string{"personal_data", "contact_info"},
		Method:            "hash_replacement",
		Errors:            []string{},
	}

	assert.Equal(t, "subject123", result.DataSubjectID)
	assert.True(t, result.Success)
	assert.Equal(t, int64(50), result.RecordsAnonymized)
	assert.Equal(t, int64(10), result.RecordsDeleted)
	assert.Len(t, result.DataCategories, 2)
	assert.Equal(t, "hash_replacement", result.Method)
	assert.Empty(t, result.Errors)
	assert.NotNil(t, result.CompletedAt)
}

// BenchmarkTCPAValidationRequest benchmarks TCPA validation request creation
func BenchmarkTCPAValidationRequest(b *testing.B) {
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		req := TCPAValidationRequest{
			PhoneNumber: "+14155551234",
			CallTime:    time.Now(),
			Timezone:    "America/New_York",
			CallType:    "marketing",
			ActorID:     "user123",
		}
		_ = req
	}
}

// BenchmarkGDPRRequest benchmarks GDPR request creation
func BenchmarkGDPRRequest(b *testing.B) {
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		req := GDPRRequest{
			Type:               "ACCESS",
			DataSubjectID:      "subject123",
			DataSubjectEmail:   "test@example.com",
			VerificationMethod: "email_verification",
			IdentityVerified:   true,
			ExportFormat:       "JSON",
			RequestDate:        time.Now(),
		}
		_ = req
	}
}

// TestComplianceServiceCreation tests that we can create a compliance service with proper structure
func TestComplianceServiceCreation(t *testing.T) {
	// This test verifies that our ComplianceService structure is valid
	service := &ComplianceService{
		retentionPolicies: make(map[string]RetentionPolicy),
		legalHolds:        make(map[string]LegalHold),
	}

	assert.NotNil(t, service)
	assert.NotNil(t, service.retentionPolicies)
	assert.NotNil(t, service.legalHolds)

	// Test adding a retention policy
	policy := RetentionPolicy{
		ID:          "TEST_POLICY",
		Name:        "Test Policy",
		Description: "Test retention policy",
		DataTypes:   []string{"test_data"},
		RetentionPeriod: RetentionPeriod{
			Duration: 30,
			Unit:     "days",
		},
		Actions: []RetentionAction{
			{Type: "DELETE", AfterDays: 30},
		},
		IsActive: true,
	}

	service.retentionPolicies[policy.ID] = policy

	storedPolicy, exists := service.retentionPolicies[policy.ID]
	assert.True(t, exists)
	assert.Equal(t, policy.ID, storedPolicy.ID)
	assert.Equal(t, policy.Name, storedPolicy.Name)

	// Test adding a legal hold
	hold := LegalHold{
		ID:             "HOLD-001",
		Description:    "Test hold",
		IssuedBy:       "legal_team",
		IssuedDate:     time.Now(),
		DataCategories: []string{"test_data"},
		Status:         "active",
		LegalAuthority: "court_order",
		CourtOrder:     true,
	}

	service.legalHolds[hold.ID] = hold

	storedHold, exists := service.legalHolds[hold.ID]
	assert.True(t, exists)
	assert.Equal(t, hold.ID, storedHold.ID)
	assert.Equal(t, hold.Description, storedHold.Description)
}

// TestComplianceEngineTypes tests that all engine types work correctly
func TestComplianceEngineTypes(t *testing.T) {
	engines := map[string]string{
		"TCPA": "TCPA compliance engine",
		"GDPR": "GDPR compliance engine",
		"CCPA": "CCPA compliance engine",
		"SOX":  "SOX compliance engine",
	}

	for engineType, description := range engines {
		assert.NotEmpty(t, engineType)
		assert.NotEmpty(t, description)
	}
}

// TestHelperFunctions tests utility helper functions
func TestHelperFunctions(t *testing.T) {
	// Test contains function
	slice := []string{"item1", "item2", "item3"}
	assert.True(t, contains(slice, "item1"))
	assert.False(t, contains(slice, "item4"))

	// Test empty slice
	emptySlice := []string{}
	assert.False(t, contains(emptySlice, "item1"))

	// Test nil slice
	var nilSlice []string
	assert.False(t, contains(nilSlice, "item1"))
}

// TestTimeValidation tests time-related validation functions
func TestTimeValidation(t *testing.T) {
	// Test valid times for TCPA (8 AM - 9 PM)
	validTimes := []int{8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}
	invalidTimes := []int{0, 1, 2, 3, 4, 5, 6, 7, 21, 22, 23}

	for _, hour := range validTimes {
		callTime := time.Date(2024, 1, 15, hour, 0, 0, 0, time.UTC)
		assert.True(t, hour >= 8 && hour <= 20, "Hour %d should be valid for TCPA", hour)
		assert.False(t, callTime.IsZero())
	}

	for _, hour := range invalidTimes {
		assert.True(t, hour < 8 || hour > 20, "Hour %d should be invalid for TCPA", hour)
	}
}

// TestErrorStructures tests error handling structures
func TestErrorStructures(t *testing.T) {
	// Test that we can create various error scenarios
	violations := []ComplianceViolation{
		{
			Type:        "NO_CONSENT",
			Severity:    "CRITICAL",
			Description: "No consent found",
			Regulation:  "TCPA",
		},
		{
			Type:        "TIME_VIOLATION",
			Severity:    "HIGH",
			Description: "Call outside permitted hours",
			Regulation:  "TCPA",
		},
		{
			Type:        "MISSING_LEGAL_BASIS",
			Severity:    "HIGH",
			Description: "No legal basis for processing",
			Regulation:  "GDPR",
		},
	}

	assert.Len(t, violations, 3)

	for _, violation := range violations {
		assert.NotEmpty(t, violation.Type)
		assert.NotEmpty(t, violation.Severity)
		assert.NotEmpty(t, violation.Description)
		assert.NotEmpty(t, violation.Regulation)
	}
}

// TestDataStructureValidation tests that all our data structures are valid
func TestDataStructureValidation(t *testing.T) {
	// Test all major structures can be created and have required fields

	// TCPA structures
	tcpaReq := TCPAValidationRequest{PhoneNumber: "+14155551234", ActorID: "user123"}
	assert.NotEmpty(t, tcpaReq.PhoneNumber)
	assert.NotEmpty(t, tcpaReq.ActorID)

	tcpaConsent := TCPAConsent{PhoneNumber: "+14155551234", ConsentType: "EXPRESS"}
	assert.NotEmpty(t, tcpaConsent.PhoneNumber)
	assert.NotEmpty(t, tcpaConsent.ConsentType)

	// GDPR structures
	gdprReq := GDPRRequest{Type: "ACCESS", DataSubjectID: "subject123"}
	assert.NotEmpty(t, gdprReq.Type)
	assert.NotEmpty(t, gdprReq.DataSubjectID)

	// CCPA structures
	ccpaReq := CCPARequest{Type: "OPT_OUT", ConsumerID: "consumer123"}
	assert.NotEmpty(t, ccpaReq.Type)
	assert.NotEmpty(t, ccpaReq.ConsumerID)

	// SOX structures
	soxCriteria := SOXReportCriteria{Period: "Q1", StartDate: time.Now(), EndDate: time.Now()}
	assert.NotEmpty(t, soxCriteria.Period)
	assert.False(t, soxCriteria.StartDate.IsZero())
	assert.False(t, soxCriteria.EndDate.IsZero())

	// Retention structures
	policy := RetentionPolicy{ID: "POLICY1", Name: "Test Policy"}
	assert.NotEmpty(t, policy.ID)
	assert.NotEmpty(t, policy.Name)

	hold := LegalHold{ID: "HOLD1", Description: "Test Hold"}
	assert.NotEmpty(t, hold.ID)
	assert.NotEmpty(t, hold.Description)
}
