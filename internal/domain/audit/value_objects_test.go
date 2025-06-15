package audit

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestActorType tests
func TestActorType(t *testing.T) {
	t.Run("NewActorType valid inputs", func(t *testing.T) {
		validActors := []struct {
			input    string
			expected ActorType
		}{
			{"user", ActorTypeUser},
			{"system", ActorTypeSystem},
			{"api", ActorTypeAPI},
			{"service", ActorTypeService},
			{"admin", ActorTypeAdmin},
			{"guest", ActorTypeGuest},
			{"bot", ActorTypeBot},
			{"scheduler", ActorTypeScheduler},
			{"USER", ActorTypeUser},   // Case insensitive
			{" user ", ActorTypeUser}, // Trimmed
		}

		for _, test := range validActors {
			t.Run(test.input, func(t *testing.T) {
				actorType, err := NewActorType(test.input)
				require.NoError(t, err)
				assert.Equal(t, test.expected, actorType)
				assert.True(t, actorType.IsValid())
			})
		}
	})

	t.Run("NewActorType invalid inputs", func(t *testing.T) {
		invalidInputs := []struct {
			input     string
			errorCode string
		}{
			{"", "EMPTY_ACTOR_TYPE"},
			{"invalid", "INVALID_ACTOR_TYPE"},
			{"unknown_type", "INVALID_ACTOR_TYPE"},
		}

		for _, test := range invalidInputs {
			t.Run(test.input, func(t *testing.T) {
				_, err := NewActorType(test.input)
				require.Error(t, err)
				
				var appErr *errors.AppError
				require.ErrorAs(t, err, &appErr)
				assert.Equal(t, test.errorCode, appErr.Code)
			})
		}
	})

	t.Run("ActorType methods", func(t *testing.T) {
		actor := ActorTypeUser
		
		assert.Equal(t, "user", actor.String())
		assert.True(t, actor.Equal(ActorTypeUser))
		assert.False(t, actor.Equal(ActorTypeSystem))
		assert.True(t, actor.IsHuman())
		assert.False(t, actor.IsAutomated())
		assert.Equal(t, "medium", actor.GetDefaultTrustLevel())
	})

	t.Run("ActorType IsHuman", func(t *testing.T) {
		humanActors := []ActorType{ActorTypeUser, ActorTypeAdmin, ActorTypeGuest}
		nonHumanActors := []ActorType{ActorTypeSystem, ActorTypeBot, ActorTypeScheduler, ActorTypeAPI, ActorTypeService}

		for _, actor := range humanActors {
			assert.True(t, actor.IsHuman(), "%s should be human", actor)
		}

		for _, actor := range nonHumanActors {
			assert.False(t, actor.IsHuman(), "%s should not be human", actor)
		}
	})

	t.Run("ActorType IsAutomated", func(t *testing.T) {
		automatedActors := []ActorType{ActorTypeSystem, ActorTypeBot, ActorTypeScheduler}
		nonAutomatedActors := []ActorType{ActorTypeUser, ActorTypeAdmin, ActorTypeGuest, ActorTypeAPI, ActorTypeService}

		for _, actor := range automatedActors {
			assert.True(t, actor.IsAutomated(), "%s should be automated", actor)
		}

		for _, actor := range nonAutomatedActors {
			assert.False(t, actor.IsAutomated(), "%s should not be automated", actor)
		}
	})

	t.Run("ActorType JSON marshaling", func(t *testing.T) {
		actor := ActorTypeUser
		
		data, err := json.Marshal(actor)
		require.NoError(t, err)
		assert.Equal(t, `"user"`, string(data))
		
		var unmarshaled ActorType
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)
		assert.Equal(t, actor, unmarshaled)
	})

	t.Run("ActorType database operations", func(t *testing.T) {
		actor := ActorTypeUser
		
		// Test Value()
		val, err := actor.Value()
		require.NoError(t, err)
		assert.Equal(t, "user", val)
		
		// Test Scan()
		var scanned ActorType
		err = scanned.Scan("user")
		require.NoError(t, err)
		assert.Equal(t, actor, scanned)
		
		// Test Scan() with []byte
		err = scanned.Scan([]byte("admin"))
		require.NoError(t, err)
		assert.Equal(t, ActorTypeAdmin, scanned)
		
		// Test Scan() with nil
		err = scanned.Scan(nil)
		require.NoError(t, err)
		assert.Equal(t, ActorType(""), scanned)
	})
}

// TestTargetType tests
func TestTargetType(t *testing.T) {
	t.Run("NewTargetType valid inputs", func(t *testing.T) {
		validTargets := []struct {
			input    string
			expected TargetType
		}{
			{"user", TargetTypeUser},
			{"call", TargetTypeCall},
			{"bid", TargetTypeBid},
			{"account", TargetTypeAccount},
			{"phone_number", TargetTypePhoneNumber},
			{"user_profile", TargetTypeUserProfile},
			{"configuration", TargetTypeConfiguration},
			{"rule", TargetTypeRule},
			{"permission", TargetTypePermission},
			{"session", TargetTypeSession},
			{"transaction", TargetTypeTransaction},
			{"payment", TargetTypePayment},
			{"audit_log", TargetTypeAuditLog},
			{"database", TargetTypeDatabase},
			{"file", TargetTypeFile},
			{"system", TargetTypeSystem},
		}

		for _, test := range validTargets {
			t.Run(test.input, func(t *testing.T) {
				targetType, err := NewTargetType(test.input)
				require.NoError(t, err)
				assert.Equal(t, test.expected, targetType)
				assert.True(t, targetType.IsValid())
			})
		}
	})

	t.Run("TargetType methods", func(t *testing.T) {
		target := TargetTypeUser
		
		assert.Equal(t, "user", target.String())
		assert.True(t, target.Equal(TargetTypeUser))
		assert.False(t, target.Equal(TargetTypeCall))
		assert.True(t, target.IsPII())
		assert.False(t, target.IsFinancial())
		
		dataClasses := target.GetDefaultDataClasses()
		assert.Contains(t, dataClasses, DataClassPersonalData)
	})

	t.Run("TargetType IsPII", func(t *testing.T) {
		piiTargets := []TargetType{TargetTypeUser, TargetTypePhoneNumber, TargetTypeUserProfile}
		nonPiiTargets := []TargetType{TargetTypeCall, TargetTypeBid, TargetTypeSystem}

		for _, target := range piiTargets {
			assert.True(t, target.IsPII(), "%s should contain PII", target)
		}

		for _, target := range nonPiiTargets {
			assert.False(t, target.IsPII(), "%s should not contain PII", target)
		}
	})

	t.Run("TargetType IsFinancial", func(t *testing.T) {
		financialTargets := []TargetType{TargetTypeTransaction, TargetTypePayment}
		nonFinancialTargets := []TargetType{TargetTypeUser, TargetTypeCall, TargetTypeSystem}

		for _, target := range financialTargets {
			assert.True(t, target.IsFinancial(), "%s should be financial", target)
		}

		for _, target := range nonFinancialTargets {
			assert.False(t, target.IsFinancial(), "%s should not be financial", target)
		}
	})
}

// TestEventCategory tests
func TestEventCategory(t *testing.T) {
	t.Run("NewEventCategory valid inputs", func(t *testing.T) {
		validCategories := []struct {
			input    string
			expected EventCategory
		}{
			{"consent", EventCategoryConsent},
			{"data_access", EventCategoryDataAccess},
			{"call", EventCategoryCall},
			{"configuration", EventCategoryConfiguration},
			{"security", EventCategorySecurity},
			{"marketplace", EventCategoryMarketplace},
			{"financial", EventCategoryFinancial},
			{"system", EventCategorySystem},
			{"compliance", EventCategoryCompliance},
			{"audit", EventCategoryAudit},
			{"other", EventCategoryOther},
		}

		for _, test := range validCategories {
			t.Run(test.input, func(t *testing.T) {
				category, err := NewEventCategory(test.input)
				require.NoError(t, err)
				assert.Equal(t, test.expected, category)
				assert.True(t, category.IsValid())
			})
		}
	})

	t.Run("EventCategory methods", func(t *testing.T) {
		category := EventCategoryConsent
		
		assert.Equal(t, "consent", category.String())
		assert.True(t, category.Equal(EventCategoryConsent))
		assert.False(t, category.Equal(EventCategoryCall))
		assert.True(t, category.IsComplianceRelevant())
		assert.Equal(t, 7, category.GetDefaultRetentionYears())
		assert.Equal(t, "✓", category.GetIcon())
	})

	t.Run("EventCategory IsComplianceRelevant", func(t *testing.T) {
		complianceCategories := []EventCategory{
			EventCategoryConsent, EventCategoryDataAccess,
			EventCategoryCompliance, EventCategoryAudit,
		}
		nonComplianceCategories := []EventCategory{
			EventCategoryCall, EventCategoryMarketplace,
			EventCategorySystem, EventCategoryOther,
		}

		for _, category := range complianceCategories {
			assert.True(t, category.IsComplianceRelevant(), "%s should be compliance relevant", category)
		}

		for _, category := range nonComplianceCategories {
			assert.False(t, category.IsComplianceRelevant(), "%s should not be compliance relevant", category)
		}
	})

	t.Run("EventCategory GetDefaultRetentionYears", func(t *testing.T) {
		testCases := []struct {
			category        EventCategory
			expectedYears   int
		}{
			{EventCategoryConsent, 7},
			{EventCategoryCompliance, 7},
			{EventCategoryAudit, 7},
			{EventCategoryFinancial, 10},
			{EventCategorySecurity, 3},
			{EventCategorySystem, 7}, // Default
		}

		for _, test := range testCases {
			assert.Equal(t, test.expectedYears, test.category.GetDefaultRetentionYears(),
				"category %s should have %d year retention", test.category, test.expectedYears)
		}
	})
}

// TestComplianceFlag tests
func TestComplianceFlag(t *testing.T) {
	t.Run("NewComplianceFlag valid inputs", func(t *testing.T) {
		validFlags := []struct {
			name     string
			value    bool
			expected string
		}{
			{"gdpr_compliant", true, "gdpr_compliant"},
			{"tcpa_compliant", false, "tcpa_compliant"},
			{"contains_pii", true, "contains_pii"},
			{"GDPR Relevant", true, "gdpr_relevant"}, // Normalized
			{" requires consent ", false, "requires_consent"}, // Trimmed and normalized
		}

		for _, test := range validFlags {
			t.Run(test.name, func(t *testing.T) {
				flag, err := NewComplianceFlag(test.name, test.value)
				require.NoError(t, err)
				assert.Equal(t, test.expected, flag.Name())
				assert.Equal(t, test.value, flag.Value())
			})
		}
	})

	t.Run("NewComplianceFlag invalid inputs", func(t *testing.T) {
		invalidInputs := []struct {
			name      string
			errorCode string
		}{
			{"", "EMPTY_COMPLIANCE_FLAG"},
			{"x", "INVALID_COMPLIANCE_FLAG"}, // Too short
		}

		for _, test := range invalidInputs {
			t.Run(test.name, func(t *testing.T) {
				_, err := NewComplianceFlag(test.name, true)
				require.Error(t, err)
				
				var appErr *errors.AppError
				require.ErrorAs(t, err, &appErr)
				assert.Equal(t, test.errorCode, appErr.Code)
			})
		}
	})

	t.Run("ComplianceFlag methods", func(t *testing.T) {
		flag, err := NewComplianceFlag("gdpr_compliant", true)
		require.NoError(t, err)
		
		assert.Equal(t, "gdpr_compliant", flag.Name())
		assert.True(t, flag.Value())
		assert.True(t, flag.IsTrue())
		assert.False(t, flag.IsFalse())
		assert.Equal(t, "gdpr_compliant:true", flag.String())
		
		otherFlag, err := NewComplianceFlag("gdpr_compliant", true)
		require.NoError(t, err)
		assert.True(t, flag.Equal(otherFlag))
		
		differentFlag, err := NewComplianceFlag("tcpa_compliant", true)
		require.NoError(t, err)
		assert.False(t, flag.Equal(differentFlag))
	})
}

// TestDataClass tests
func TestDataClass(t *testing.T) {
	t.Run("NewDataClass valid inputs", func(t *testing.T) {
		validClasses := []DataClass{
			DataClassPersonalData, DataClassSensitiveData, DataClassBiometricData,
			DataClassHealthData, DataClassFinancialData, DataClassLocationData,
			DataClassCommunicationData, DataClassBehavioralData, DataClassPhoneNumber,
			DataClassEmail, DataClassIPAddress, DataClassContactInfo,
			DataClassDemographicData, DataClassPreferences, DataClassUsageData,
			DataClassDeviceData, DataClassPublicData,
		}

		for _, dataClass := range validClasses {
			t.Run(string(dataClass), func(t *testing.T) {
				dc, err := NewDataClass(string(dataClass))
				require.NoError(t, err)
				assert.Equal(t, dataClass, dc)
				assert.True(t, dc.IsValid())
			})
		}
	})

	t.Run("DataClass methods", func(t *testing.T) {
		dataClass := DataClassPersonalData
		
		assert.Equal(t, "personal_data", dataClass.String())
		assert.True(t, dataClass.Equal(DataClassPersonalData))
		assert.False(t, dataClass.Equal(DataClassPublicData))
		assert.True(t, dataClass.IsGDPRRelevant())
		assert.True(t, dataClass.IsCCPARelevant())
		assert.False(t, dataClass.IsSensitive())
		assert.Equal(t, 7, dataClass.GetMinimumRetentionYears())
	})

	t.Run("DataClass IsGDPRRelevant", func(t *testing.T) {
		// All except public data should be GDPR relevant
		gdprRelevant := []DataClass{
			DataClassPersonalData, DataClassSensitiveData, DataClassFinancialData,
		}
		nonGdprRelevant := []DataClass{
			DataClassPublicData,
		}

		for _, dataClass := range gdprRelevant {
			assert.True(t, dataClass.IsGDPRRelevant(), "%s should be GDPR relevant", dataClass)
		}

		for _, dataClass := range nonGdprRelevant {
			assert.False(t, dataClass.IsGDPRRelevant(), "%s should not be GDPR relevant", dataClass)
		}
	})

	t.Run("DataClass IsCCPARelevant", func(t *testing.T) {
		ccpaRelevant := []DataClass{
			DataClassPersonalData, DataClassSensitiveData, DataClassBiometricData,
			DataClassLocationData, DataClassFinancialData,
		}
		nonCcpaRelevant := []DataClass{
			DataClassPublicData, DataClassUsageData, DataClassDeviceData,
		}

		for _, dataClass := range ccpaRelevant {
			assert.True(t, dataClass.IsCCPARelevant(), "%s should be CCPA relevant", dataClass)
		}

		for _, dataClass := range nonCcpaRelevant {
			assert.False(t, dataClass.IsCCPARelevant(), "%s should not be CCPA relevant", dataClass)
		}
	})

	t.Run("DataClass IsSensitive", func(t *testing.T) {
		sensitiveClasses := []DataClass{
			DataClassSensitiveData, DataClassBiometricData,
			DataClassHealthData, DataClassFinancialData,
		}
		nonSensitiveClasses := []DataClass{
			DataClassPersonalData, DataClassPublicData, DataClassUsageData,
		}

		for _, dataClass := range sensitiveClasses {
			assert.True(t, dataClass.IsSensitive(), "%s should be sensitive", dataClass)
		}

		for _, dataClass := range nonSensitiveClasses {
			assert.False(t, dataClass.IsSensitive(), "%s should not be sensitive", dataClass)
		}
	})

	t.Run("DataClass GetMinimumRetentionYears", func(t *testing.T) {
		testCases := []struct {
			dataClass     DataClass
			expectedYears int
		}{
			{DataClassFinancialData, 10},
			{DataClassHealthData, 7},
			{DataClassSensitiveData, 7},
			{DataClassBiometricData, 7},
			{DataClassPersonalData, 7}, // Default
		}

		for _, test := range testCases {
			assert.Equal(t, test.expectedYears, test.dataClass.GetMinimumRetentionYears(),
				"data class %s should have %d year retention", test.dataClass, test.expectedYears)
		}
	})
}

// TestLegalBasis tests
func TestLegalBasis(t *testing.T) {
	t.Run("NewLegalBasis valid inputs", func(t *testing.T) {
		validBases := []LegalBasis{
			LegalBasisConsent, LegalBasisContract, LegalBasisLegalObligation,
			LegalBasisVitalInterests, LegalBasisPublicTask, LegalBasisLegitimateInterest,
		}

		for _, legalBasis := range validBases {
			t.Run(string(legalBasis), func(t *testing.T) {
				lb, err := NewLegalBasis(string(legalBasis))
				require.NoError(t, err)
				assert.Equal(t, legalBasis, lb)
				assert.True(t, lb.IsValid())
			})
		}
	})

	t.Run("LegalBasis methods", func(t *testing.T) {
		basis := LegalBasisConsent
		
		assert.Equal(t, "consent", basis.String())
		assert.True(t, basis.Equal(LegalBasisConsent))
		assert.False(t, basis.Equal(LegalBasisContract))
		assert.True(t, basis.RequiresExplicitConsent())
		assert.True(t, basis.AllowsWithdrawal())
		assert.Contains(t, basis.GetDescription(), "consent")
	})

	t.Run("LegalBasis RequiresExplicitConsent", func(t *testing.T) {
		consentRequired := []LegalBasis{LegalBasisConsent}
		consentNotRequired := []LegalBasis{
			LegalBasisContract, LegalBasisLegalObligation,
			LegalBasisVitalInterests, LegalBasisPublicTask, LegalBasisLegitimateInterest,
		}

		for _, basis := range consentRequired {
			assert.True(t, basis.RequiresExplicitConsent(), "%s should require explicit consent", basis)
		}

		for _, basis := range consentNotRequired {
			assert.False(t, basis.RequiresExplicitConsent(), "%s should not require explicit consent", basis)
		}
	})

	t.Run("LegalBasis AllowsWithdrawal", func(t *testing.T) {
		withdrawalAllowed := []LegalBasis{LegalBasisConsent, LegalBasisLegitimateInterest}
		withdrawalNotAllowed := []LegalBasis{
			LegalBasisContract, LegalBasisLegalObligation,
			LegalBasisVitalInterests, LegalBasisPublicTask,
		}

		for _, basis := range withdrawalAllowed {
			assert.True(t, basis.AllowsWithdrawal(), "%s should allow withdrawal", basis)
		}

		for _, basis := range withdrawalNotAllowed {
			assert.False(t, basis.AllowsWithdrawal(), "%s should not allow withdrawal", basis)
		}
	})
}

// TestEventResult tests
func TestEventResult(t *testing.T) {
	t.Run("NewEventResult valid inputs", func(t *testing.T) {
		validResults := []EventResult{
			EventResultSuccess, EventResultFailure, EventResultPartial,
			EventResultPending, EventResultTimeout, EventResultCancelled,
		}

		for _, result := range validResults {
			t.Run(string(result), func(t *testing.T) {
				er, err := NewEventResult(string(result))
				require.NoError(t, err)
				assert.Equal(t, result, er)
				assert.True(t, er.IsValid())
			})
		}
	})

	t.Run("EventResult methods", func(t *testing.T) {
		result := EventResultSuccess
		
		assert.Equal(t, "success", result.String())
		assert.True(t, result.Equal(EventResultSuccess))
		assert.False(t, result.Equal(EventResultFailure))
		assert.True(t, result.IsSuccess())
		assert.False(t, result.IsFailure())
		assert.False(t, result.IsPartial())
		assert.True(t, result.IsCompleted())
		assert.False(t, result.IsPending())
		assert.Equal(t, "✅", result.GetIcon())
		assert.Equal(t, SeverityInfo, result.GetDefaultSeverity())
	})

	t.Run("EventResult status checks", func(t *testing.T) {
		testCases := []struct {
			result      EventResult
			isSuccess   bool
			isFailure   bool
			isPartial   bool
			isCompleted bool
			isPending   bool
			icon        string
			severity    Severity
		}{
			{EventResultSuccess, true, false, false, true, false, "✅", SeverityInfo},
			{EventResultFailure, false, true, false, true, false, "❌", SeverityError},
			{EventResultPartial, false, false, true, true, false, "⚠️", SeverityWarning},
			{EventResultPending, false, false, false, false, true, "⏳", SeverityInfo},
			{EventResultTimeout, false, false, false, false, false, "⏰", SeverityWarning},
			{EventResultCancelled, false, false, false, false, false, "🚫", SeverityInfo},
		}

		for _, test := range testCases {
			t.Run(string(test.result), func(t *testing.T) {
				assert.Equal(t, test.isSuccess, test.result.IsSuccess())
				assert.Equal(t, test.isFailure, test.result.IsFailure())
				assert.Equal(t, test.isPartial, test.result.IsPartial())
				assert.Equal(t, test.isCompleted, test.result.IsCompleted())
				assert.Equal(t, test.isPending, test.result.IsPending())
				assert.Equal(t, test.icon, test.result.GetIcon())
				assert.Equal(t, test.severity, test.result.GetDefaultSeverity())
			})
		}
	})
}

// TestHelperFunctions tests helper functions for value objects
func TestHelperFunctions(t *testing.T) {
	t.Run("NewRetentionPeriodFromDataClasses", func(t *testing.T) {
		// Empty data classes should return default
		duration, err := NewRetentionPeriodFromDataClasses([]DataClass{})
		require.NoError(t, err)
		assert.Equal(t, 7*365*24*time.Hour, duration)
		
		// Financial data should return 10 years
		duration, err = NewRetentionPeriodFromDataClasses([]DataClass{DataClassFinancialData})
		require.NoError(t, err)
		assert.Equal(t, 10*365*24*time.Hour, duration)
		
		// Mix should return maximum
		duration, err = NewRetentionPeriodFromDataClasses([]DataClass{
			DataClassPersonalData,    // 7 years
			DataClassFinancialData,   // 10 years
			DataClassHealthData,      // 7 years
		})
		require.NoError(t, err)
		assert.Equal(t, 10*365*24*time.Hour, duration)
	})

	t.Run("ValidateComplianceFlags", func(t *testing.T) {
		// Valid flags
		validFlags := map[string]bool{
			"gdpr_compliant": true,
			"tcpa_relevant":  false,
			"contains_pii":   true,
		}
		err := ValidateComplianceFlags(validFlags)
		assert.NoError(t, err)
		
		// Invalid flag
		invalidFlags := map[string]bool{
			"gdpr_compliant": true,
			"invalid_flag_x": false, // Too short
		}
		err = ValidateComplianceFlags(invalidFlags)
		assert.Error(t, err)
	})

	t.Run("ValidateDataClasses", func(t *testing.T) {
		// Valid data classes
		validClasses := []string{"personal_data", "financial_data", "health_data"}
		err := ValidateDataClasses(validClasses)
		assert.NoError(t, err)
		
		// Invalid data class
		invalidClasses := []string{"personal_data", "invalid_class"}
		err = ValidateDataClasses(invalidClasses)
		assert.Error(t, err)
	})

	t.Run("isValidComplianceFlagName", func(t *testing.T) {
		// Known valid flags
		knownFlags := []string{
			"gdpr_compliant", "tcpa_compliant", "contains_pii",
			"requires_consent", "explicit_consent", "encrypted",
		}
		
		for _, flag := range knownFlags {
			assert.True(t, isValidComplianceFlagName(flag), "flag %s should be valid", flag)
		}
		
		// Custom flags following convention
		customFlags := []string{
			"custom_compliance_flag", "internal_audit_required", "special_handling",
		}
		
		for _, flag := range customFlags {
			assert.True(t, isValidComplianceFlagName(flag), "custom flag %s should be valid", flag)
		}
		
		// Invalid flags
		invalidFlags := []string{
			"x", "ab", // Too short
			strings.Repeat("a", 51), // Too long
		}
		
		for _, flag := range invalidFlags {
			assert.False(t, isValidComplianceFlagName(flag), "flag %s should be invalid", flag)
		}
	})
}

// Property-based tests for value objects

// TestPropertyActorTypeValidationIsConsistent tests that actor type validation is consistent
func TestPropertyActorTypeValidationIsConsistent(t *testing.T) {
	validActorStrings := []string{"user", "system", "api", "service", "admin", "guest", "bot", "scheduler"}
	
	for i := 0; i < 1000; i++ {
		// Test valid actor types
		actorString := validActorStrings[rand.Intn(len(validActorStrings))]
		
		// Test with different casings and whitespace
		variations := []string{
			actorString,
			strings.ToUpper(actorString),
			strings.Title(actorString),
			" " + actorString + " ",
			"\t" + actorString + "\n",
		}
		
		expectedActor, err := NewActorType(actorString)
		require.NoError(t, err, "iteration %d: base actor creation failed", i)
		
		for j, variation := range variations {
			actualActor, err := NewActorType(variation)
			require.NoError(t, err, "iteration %d variation %d: actor creation failed", i, j)
			assert.Equal(t, expectedActor, actualActor, "iteration %d variation %d: actors should be equal", i, j)
		}
	}
}

// TestPropertyDataClassRetentionInvariants tests data class retention invariants
func TestPropertyDataClassRetentionInvariants(t *testing.T) {
	allDataClasses := []DataClass{
		DataClassPersonalData, DataClassSensitiveData, DataClassBiometricData,
		DataClassHealthData, DataClassFinancialData, DataClassLocationData,
		DataClassCommunicationData, DataClassBehavioralData, DataClassPhoneNumber,
		DataClassEmail, DataClassIPAddress, DataClassContactInfo,
		DataClassDemographicData, DataClassPreferences, DataClassUsageData,
		DataClassDeviceData, DataClassPublicData,
	}
	
	for i := 0; i < 1000; i++ {
		// Generate random subset of data classes
		numClasses := rand.Intn(len(allDataClasses)) + 1
		selectedClasses := make([]DataClass, 0, numClasses)
		
		// Randomly select classes
		indices := rand.Perm(len(allDataClasses))[:numClasses]
		for _, idx := range indices {
			selectedClasses = append(selectedClasses, allDataClasses[idx])
		}
		
		// Calculate retention period
		duration, err := NewRetentionPeriodFromDataClasses(selectedClasses)
		require.NoError(t, err, "iteration %d: retention calculation failed", i)
		
		// Test invariants
		assert.Greater(t, duration, time.Duration(0), "iteration %d: retention must be positive", i)
		
		// Retention should be at least as long as the maximum individual class requirement
		maxIndividualRetention := 0
		for _, dataClass := range selectedClasses {
			if years := dataClass.GetMinimumRetentionYears(); years > maxIndividualRetention {
				maxIndividualRetention = years
			}
		}
		
		expectedMinDuration := time.Duration(maxIndividualRetention) * 365 * 24 * time.Hour
		assert.GreaterOrEqual(t, duration, expectedMinDuration, "iteration %d: retention should meet minimum", i)
	}
}

// TestPropertyComplianceFlagNormalizationIsConsistent tests flag normalization consistency
func TestPropertyComplianceFlagNormalizationIsConsistent(t *testing.T) {
	testFlags := []string{
		"gdpr_compliant", "tcpa_relevant", "contains_pii", "requires_consent",
		"explicit_consent", "data_minimization", "pseudonymized", "encrypted",
	}
	
	for i := 0; i < 1000; i++ {
		baseFlag := testFlags[rand.Intn(len(testFlags))]
		
		// Create variations
		variations := []string{
			baseFlag,
			strings.ToUpper(baseFlag),
			strings.ReplaceAll(baseFlag, "_", " "),
			strings.Title(strings.ReplaceAll(baseFlag, "_", " ")),
			" " + baseFlag + " ",
		}
		
		expectedFlag, err := NewComplianceFlag(baseFlag, true)
		require.NoError(t, err, "iteration %d: base flag creation failed", i)
		
		for j, variation := range variations {
			actualFlag, err := NewComplianceFlag(variation, true)
			require.NoError(t, err, "iteration %d variation %d: flag creation failed", i, j)
			assert.Equal(t, expectedFlag.Name(), actualFlag.Name(), "iteration %d variation %d: flag names should be equal", i, j)
		}
	}
}

// TestPropertyEventResultClassificationIsComplete tests that all results are properly classified
func TestPropertyEventResultClassificationIsComplete(t *testing.T) {
	allResults := []EventResult{
		EventResultSuccess, EventResultFailure, EventResultPartial,
		EventResultPending, EventResultTimeout, EventResultCancelled,
	}
	
	for i := 0; i < 1000; i++ {
		result := allResults[rand.Intn(len(allResults))]
		
		// Every result should be classified into exactly one primary category
		classifications := []bool{
			result.IsSuccess(),
			result.IsFailure(),
			result.IsPartial(),
			result.IsPending(),
		}
		
		// Timeout and cancelled are not in primary categories but have their own logic
		if result == EventResultTimeout || result == EventResultCancelled {
			// These should not be in any primary category
			for j, classified := range classifications {
				assert.False(t, classified, "iteration %d: result %s should not be in classification %d", i, result, j)
			}
		} else {
			// Should be in exactly one primary category
			trueCount := 0
			for _, classified := range classifications {
				if classified {
					trueCount++
				}
			}
			assert.Equal(t, 1, trueCount, "iteration %d: result %s should be in exactly one primary category", i, result)
		}
		
		// All results should have valid icons and severities
		icon := result.GetIcon()
		assert.NotEmpty(t, icon, "iteration %d: result %s should have icon", i, result)
		
		severity := result.GetDefaultSeverity()
		assert.True(t, severity == SeverityInfo || severity == SeverityWarning || 
			severity == SeverityError || severity == SeverityCritical,
			"iteration %d: result %s should have valid severity", i, result)
	}
}

// TestPropertyLegalBasisClassificationIsComplete tests legal basis classification completeness
func TestPropertyLegalBasisClassificationIsComplete(t *testing.T) {
	allBases := []LegalBasis{
		LegalBasisConsent, LegalBasisContract, LegalBasisLegalObligation,
		LegalBasisVitalInterests, LegalBasisPublicTask, LegalBasisLegitimateInterest,
	}
	
	for i := 0; i < 1000; i++ {
		basis := allBases[rand.Intn(len(allBases))]
		
		// Test that all bases have proper classifications
		requiresConsent := basis.RequiresExplicitConsent()
		allowsWithdrawal := basis.AllowsWithdrawal()
		description := basis.GetDescription()
		
		// Validate description is not empty and contains meaningful content
		assert.NotEmpty(t, description, "iteration %d: basis %s should have description", i, basis)
		assert.NotEqual(t, "Unknown legal basis", description, "iteration %d: basis %s should have proper description", i, basis)
		
		// Test logical consistency
		if requiresConsent {
			// If explicit consent is required, withdrawal should be allowed
			assert.True(t, allowsWithdrawal, "iteration %d: basis %s requires consent so should allow withdrawal", i, basis)
		}
		
		// Consent basis should require explicit consent
		if basis == LegalBasisConsent {
			assert.True(t, requiresConsent, "iteration %d: consent basis should require explicit consent", i)
		}
	}
}

// Benchmark tests for value objects

// BenchmarkActorTypeCreation benchmarks actor type creation
func BenchmarkActorTypeCreation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := NewActorType("user")
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkDataClassValidation benchmarks data class validation
func BenchmarkDataClassValidation(b *testing.B) {
	dataClasses := []string{"personal_data", "financial_data", "health_data", "location_data"}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := ValidateDataClasses(dataClasses)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkComplianceFlagCreation benchmarks compliance flag creation
func BenchmarkComplianceFlagCreation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := NewComplianceFlag("gdpr_compliant", true)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkLegalBasisMethods benchmarks legal basis method calls
func BenchmarkLegalBasisMethods(b *testing.B) {
	basis := LegalBasisConsent
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = basis.RequiresExplicitConsent()
		_ = basis.AllowsWithdrawal()
		_ = basis.GetDescription()
	}
}

// BenchmarkEventResultClassification benchmarks event result classification
func BenchmarkEventResultClassification(b *testing.B) {
	result := EventResultSuccess
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = result.IsSuccess()
		_ = result.IsCompleted()
		_ = result.GetIcon()
		_ = result.GetDefaultSeverity()
	}
}