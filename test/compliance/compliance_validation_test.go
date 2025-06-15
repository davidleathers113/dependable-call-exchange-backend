//go:build compliance

package compliance

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestComplianceFrameworkValidation validates the compliance testing framework
func TestComplianceFrameworkValidation(t *testing.T) {
	t.Run("ImmutableAuditTrailCreation", func(t *testing.T) {
		// Test that we can create an immutable audit trail
		auditID := uuid.New()
		timestamp := time.Now()
		
		trail := ImmutableAuditTrail{
			TrailID:   auditID,
			CreatedAt: timestamp,
			AuditChain: []AuditChainLink{
				{
					LinkID:      uuid.New(),
					EventID:     uuid.New(),
					EventType:   "compliance_check",
					Timestamp:   timestamp,
					EventData:   `{"action": "consent_verification", "result": "valid"}`,
					CurrentHash: "abc123",
					BlockNumber: 1,
					Signature:   "sig123",
				},
			},
			ChainIntegrity:      true,
			Immutable:          true,
			Encrypted:          true,
			ComplianceStandards: []string{"GDPR", "TCPA", "SOX", "CCPA"},
		}
		
		assert.Equal(t, auditID, trail.TrailID)
		assert.True(t, trail.ChainIntegrity)
		assert.True(t, trail.Immutable)
		assert.Len(t, trail.ComplianceStandards, 4)
		require.Len(t, trail.AuditChain, 1)
		assert.Equal(t, "compliance_check", trail.AuditChain[0].EventType)
	})
	
	t.Run("GDPRDataSubjectCreation", func(t *testing.T) {
		// Test GDPR test subject creation
		phoneNumber := "+33123456789"
		subject := GDPRTestSubject{
			SubjectID:   uuid.New(),
			PhoneNumber: phoneNumber,
			Nationality: "FR",
			Residence:   "FR",
			ConsentDate: time.Now().Add(-30 * 24 * time.Hour),
			ConsentType: "explicit",
			ConsentScope: []string{"marketing_calls", "analytics"},
			LegalBasis:  "gdpr_art_6_a",
			DataCategories: []string{
				"identifiers",
				"commercial_information",
			},
			ProcessingPurposes: []string{
				"marketing_calls",
				"service_delivery",
			},
		}
		
		assert.Equal(t, phoneNumber, subject.PhoneNumber)
		assert.Equal(t, "FR", subject.Nationality)
		assert.Equal(t, "explicit", subject.ConsentType)
		assert.Contains(t, subject.ConsentScope, "marketing_calls")
		assert.Contains(t, subject.DataCategories, "identifiers")
	})
	
	t.Run("TCPAComplianceScenario", func(t *testing.T) {
		// Test TCPA compliance scenario creation
		phoneNumber := "+14155551234"
		scenario := TCPATestScenario{
			PhoneNumber:   phoneNumber,
			CallType:     "marketing",
			ConsentMethod: "written_agreement",
			ConsentDate:  time.Now().Add(-7 * 24 * time.Hour),
			ConsentScope: "marketing_calls",
			CallTime:     time.Now(),
			CallVolume: TCPACallVolume{
				Daily:   3,
				Weekly:  15,
				Monthly: 45,
			},
			PriorBusinessRelationship: true,
		}
		
		assert.Equal(t, phoneNumber, scenario.PhoneNumber)
		assert.Equal(t, "marketing", scenario.CallType)
		assert.Equal(t, "written_agreement", scenario.ConsentMethod)
		assert.True(t, scenario.PriorBusinessRelationship)
		assert.Equal(t, 3, scenario.CallVolume.Daily)
	})
	
	t.Run("SOXTransactionValidation", func(t *testing.T) {
		// Test SOX transaction creation
		transactionID := uuid.New()
		transaction := SOXTestTransaction{
			TransactionID:   transactionID,
			Amount:         "125.50",
			Currency:       "USD",
			BuyerID:        uuid.New(),
			SellerID:       uuid.New(),
			CallID:         uuid.New(),
			TransactionType: "call_payment",
			Timestamp:      time.Now(),
			FiscalPeriod: FiscalPeriod{
				Year:    2024,
				Quarter: 1,
				Month:   1,
			},
		}
		
		assert.Equal(t, transactionID, transaction.TransactionID)
		assert.Equal(t, "125.50", transaction.Amount)
		assert.Equal(t, "USD", transaction.Currency)
		assert.Equal(t, "call_payment", transaction.TransactionType)
		assert.Equal(t, 2024, transaction.FiscalPeriod.Year)
	})
	
	t.Run("CCPAConsumerRights", func(t *testing.T) {
		// Test CCPA consumer creation
		consumerID := uuid.New()
		phoneNumber := "+14085551234"
		consumer := CCPATestConsumer{
			ConsumerID:         consumerID,
			PhoneNumber:        phoneNumber,
			CaliforniaResident: true,
			DataCategories: []CCPADataCategoryTest{
				{
					Category:  "identifiers",
					Examples:  []string{"phone_number", "account_id"},
					Collected: true,
					Sold:      false,
					Disclosed: true,
					Purpose:   "service_delivery",
					Retention: "2_years",
				},
			},
		}
		
		assert.Equal(t, consumerID, consumer.ConsumerID)
		assert.Equal(t, phoneNumber, consumer.PhoneNumber)
		assert.True(t, consumer.CaliforniaResident)
		require.Len(t, consumer.DataCategories, 1)
		assert.Equal(t, "identifiers", consumer.DataCategories[0].Category)
		assert.True(t, consumer.DataCategories[0].Collected)
	})
}

// TestComplianceHelperUtilities validates helper utility functions
func TestComplianceHelperUtilities(t *testing.T) {
	helper := NewComplianceAuditTestHelper(t)
	
	t.Run("TimezoneConversion", func(t *testing.T) {
		utcTime := time.Date(2024, 1, 15, 15, 30, 0, 0, time.UTC) // 3:30 PM UTC
		
		// Test California timezone conversion
		pstTime := helper.convertToTimezone(utcTime, "America/Los_Angeles")
		assert.Equal(t, 7, pstTime.Hour()) // 7:30 AM PST (UTC-8)
		
		// Test New York timezone conversion
		estTime := helper.convertToTimezone(utcTime, "America/New_York")
		assert.Equal(t, 10, estTime.Hour()) // 10:30 AM EST (UTC-5)
	})
	
	t.Run("TCPATimeCompliance", func(t *testing.T) {
		// Test time within TCPA hours (8 AM - 9 PM)
		morningTime := time.Date(2024, 1, 15, 16, 0, 0, 0, time.UTC) // 8 AM PST
		assert.True(t, helper.isWithinTCPAHours(morningTime, "America/Los_Angeles"))
		
		// Test time outside TCPA hours
		lateTime := time.Date(2024, 1, 15, 6, 0, 0, 0, time.UTC) // 10 PM PST (previous day)
		assert.False(t, helper.isWithinTCPAHours(lateTime, "America/Los_Angeles"))
	})
	
	t.Run("TransactionHashGeneration", func(t *testing.T) {
		transactionID := uuid.New()
		amount := "100.00"
		currency := "USD"
		timestamp := time.Now()
		
		hash1 := helper.calculateTransactionHash(transactionID, amount, currency, timestamp)
		hash2 := helper.calculateTransactionHash(transactionID, amount, currency, timestamp)
		
		// Same inputs should produce same hash
		assert.Equal(t, hash1, hash2)
		
		// Different amount should produce different hash
		hash3 := helper.calculateTransactionHash(transactionID, "200.00", currency, timestamp)
		assert.NotEqual(t, hash1, hash3)
	})
	
	t.Run("AuditChainIntegrityValidation", func(t *testing.T) {
		// Test valid audit chain
		validChain := []AuditChainLink{
			{
				LinkID:       uuid.New(),
				CurrentHash:  "hash1",
				PreviousHash: "",
				BlockNumber:  1,
			},
			{
				LinkID:       uuid.New(),
				CurrentHash:  "hash2",
				PreviousHash: "hash1",
				BlockNumber:  2,
			},
		}
		
		assert.True(t, helper.validateAuditChainIntegrity(validChain))
		
		// Test invalid audit chain (broken link)
		invalidChain := []AuditChainLink{
			{
				LinkID:       uuid.New(),
				CurrentHash:  "hash1",
				PreviousHash: "",
				BlockNumber:  1,
			},
			{
				LinkID:       uuid.New(),
				CurrentHash:  "hash2",
				PreviousHash: "wrong_hash",
				BlockNumber:  2,
			},
		}
		
		assert.False(t, helper.validateAuditChainIntegrity(invalidChain))
	})
}