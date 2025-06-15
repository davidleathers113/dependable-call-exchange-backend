package audit

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEvent(t *testing.T) {
	tests := []struct {
		name      string
		eventType EventType
		actorID   string
		targetID  string
		action    string
		wantErr   bool
		errCode   string
	}{
		{
			name:      "valid consent event",
			eventType: EventConsentGranted,
			actorID:   "user123",
			targetID:  "+15551234567",
			action:    "grant_consent",
			wantErr:   false,
		},
		{
			name:      "valid call event",
			eventType: EventCallInitiated,
			actorID:   "system",
			targetID:  "call-456",
			action:    "initiate_call",
			wantErr:   false,
		},
		{
			name:      "empty event type",
			eventType: "",
			actorID:   "user123",
			targetID:  "target123",
			action:    "test_action",
			wantErr:   true,
			errCode:   "INVALID_EVENT_TYPE",
		},
		{
			name:      "empty actor ID",
			eventType: EventConsentGranted,
			actorID:   "",
			targetID:  "target123",
			action:    "test_action",
			wantErr:   true,
			errCode:   "MISSING_ACTOR_ID",
		},
		{
			name:      "empty target ID",
			eventType: EventConsentGranted,
			actorID:   "user123",
			targetID:  "",
			action:    "test_action",
			wantErr:   true,
			errCode:   "MISSING_TARGET_ID",
		},
		{
			name:      "empty action",
			eventType: EventConsentGranted,
			actorID:   "user123",
			targetID:  "target123",
			action:    "",
			wantErr:   true,
			errCode:   "MISSING_ACTION",
		},
		{
			name:      "invalid event type",
			eventType: "invalid.type",
			actorID:   "user123",
			targetID:  "target123",
			action:    "test_action",
			wantErr:   true,
			errCode:   "INVALID_EVENT_TYPE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, err := NewEvent(tt.eventType, tt.actorID, tt.targetID, tt.action)

			if tt.wantErr {
				require.Error(t, err)
				var appErr *errors.AppError
				require.ErrorAs(t, err, &appErr)
				assert.Equal(t, tt.errCode, appErr.Code)
				assert.Nil(t, event)
			} else {
				require.NoError(t, err)
				require.NotNil(t, event)

				// Verify basic fields
				assert.NotEqual(t, uuid.Nil, event.ID)
				assert.Equal(t, tt.eventType, event.Type)
				assert.Equal(t, tt.actorID, event.ActorID)
				assert.Equal(t, tt.targetID, event.TargetID)
				assert.Equal(t, tt.action, event.Action)
				assert.Equal(t, "success", event.Result)
				assert.False(t, event.Timestamp.IsZero())
				assert.Greater(t, event.TimestampNano, int64(0))
				assert.Equal(t, 2555, event.RetentionDays) // Default 7 years
				assert.NotNil(t, event.Metadata)
				assert.NotNil(t, event.ComplianceFlags)
				assert.NotNil(t, event.Tags)
				assert.False(t, event.IsImmutable())
			}
		})
	}
}

func TestEvent_ComputeHash(t *testing.T) {
	t.Run("compute hash with empty previous hash", func(t *testing.T) {
		event, err := NewEvent(EventConsentGranted, "user123", "+15551234567", "grant_consent")
		require.NoError(t, err)

		hash, err := event.ComputeHash("")
		require.NoError(t, err)
		assert.NotEmpty(t, hash)
		assert.Equal(t, hash, event.EventHash)
		assert.Equal(t, "", event.PreviousHash)
		assert.True(t, event.IsImmutable())
	})

	t.Run("compute hash with previous hash", func(t *testing.T) {
		event, err := NewEvent(EventConsentGranted, "user123", "+15551234567", "grant_consent")
		require.NoError(t, err)

		previousHash := "abcd1234567890"
		hash, err := event.ComputeHash(previousHash)
		require.NoError(t, err)
		assert.NotEmpty(t, hash)
		assert.Equal(t, hash, event.EventHash)
		assert.Equal(t, previousHash, event.PreviousHash)
		assert.True(t, event.IsImmutable())
	})

	t.Run("cannot compute hash on immutable event", func(t *testing.T) {
		event, err := NewEvent(EventConsentGranted, "user123", "+15551234567", "grant_consent")
		require.NoError(t, err)

		// First computation
		_, err = event.ComputeHash("")
		require.NoError(t, err)

		// Second computation should fail
		_, err = event.ComputeHash("new_hash")
		require.Error(t, err)
		var appErr *errors.AppError
		require.ErrorAs(t, err, &appErr)
		assert.Equal(t, "EVENT_IMMUTABLE", appErr.Code)
	})

	t.Run("hash determinism", func(t *testing.T) {
		event1, err := NewEvent(EventConsentGranted, "user123", "+15551234567", "grant_consent")
		require.NoError(t, err)
		event1.ID = uuid.MustParse("12345678-1234-5678-9012-123456789012")
		event1.TimestampNano = 1640995200000000000 // Fixed timestamp

		event2, err := NewEvent(EventConsentGranted, "user123", "+15551234567", "grant_consent")
		require.NoError(t, err)
		event2.ID = uuid.MustParse("12345678-1234-5678-9012-123456789012")
		event2.TimestampNano = 1640995200000000000 // Same timestamp

		hash1, err := event1.ComputeHash("previous123")
		require.NoError(t, err)

		hash2, err := event2.ComputeHash("previous123")
		require.NoError(t, err)

		assert.Equal(t, hash1, hash2, "Same event data should produce same hash")
	})
}

func TestEvent_Validate(t *testing.T) {
	tests := []struct {
		name     string
		setupFn  func() *Event
		wantErr  bool
		errCode  string
	}{
		{
			name: "valid event",
			setupFn: func() *Event {
				event, _ := NewEvent(EventConsentGranted, "user123", "+15551234567", "grant_consent")
				return event
			},
			wantErr: false,
		},
		{
			name: "invalid event type",
			setupFn: func() *Event {
				event, _ := NewEvent(EventConsentGranted, "user123", "+15551234567", "grant_consent")
				event.Type = "invalid.type"
				return event
			},
			wantErr: true,
			errCode: "INVALID_EVENT_TYPE",
		},
		{
			name: "invalid severity",
			setupFn: func() *Event {
				event, _ := NewEvent(EventConsentGranted, "user123", "+15551234567", "grant_consent")
				event.Severity = "INVALID"
				return event
			},
			wantErr: true,
			errCode: "INVALID_SEVERITY",
		},
		{
			name: "missing actor ID",
			setupFn: func() *Event {
				event, _ := NewEvent(EventConsentGranted, "user123", "+15551234567", "grant_consent")
				event.ActorID = ""
				return event
			},
			wantErr: true,
			errCode: "MISSING_ACTOR_ID",
		},
		{
			name: "missing target ID",
			setupFn: func() *Event {
				event, _ := NewEvent(EventConsentGranted, "user123", "+15551234567", "grant_consent")
				event.TargetID = ""
				return event
			},
			wantErr: true,
			errCode: "MISSING_TARGET_ID",
		},
		{
			name: "missing action",
			setupFn: func() *Event {
				event, _ := NewEvent(EventConsentGranted, "user123", "+15551234567", "grant_consent")
				event.Action = ""
				return event
			},
			wantErr: true,
			errCode: "MISSING_ACTION",
		},
		{
			name: "invalid result",
			setupFn: func() *Event {
				event, _ := NewEvent(EventConsentGranted, "user123", "+15551234567", "grant_consent")
				event.Result = "invalid_result"
				return event
			},
			wantErr: true,
			errCode: "INVALID_RESULT",
		},
		{
			name: "invalid retention days",
			setupFn: func() *Event {
				event, _ := NewEvent(EventConsentGranted, "user123", "+15551234567", "grant_consent")
				event.RetentionDays = -1
				return event
			},
			wantErr: true,
			errCode: "INVALID_RETENTION",
		},
		{
			name: "missing hash on immutable event",
			setupFn: func() *Event {
				event, _ := NewEvent(EventConsentGranted, "user123", "+15551234567", "grant_consent")
				event.immutable = true
				event.EventHash = ""
				return event
			},
			wantErr: true,
			errCode: "MISSING_HASH",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := tt.setupFn()
			err := event.Validate()

			if tt.wantErr {
				require.Error(t, err)
				var appErr *errors.AppError
				require.ErrorAs(t, err, &appErr)
				assert.Equal(t, tt.errCode, appErr.Code)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestEvent_ComplianceHelpers(t *testing.T) {
	t.Run("HasComplianceFlag", func(t *testing.T) {
		event, err := NewEvent(EventConsentGranted, "user123", "+15551234567", "grant_consent")
		require.NoError(t, err)

		// Flag not set
		assert.False(t, event.HasComplianceFlag("tcpa_compliant"))

		// Set flag to true
		event.ComplianceFlags["tcpa_compliant"] = true
		assert.True(t, event.HasComplianceFlag("tcpa_compliant"))

		// Set flag to false
		event.ComplianceFlags["tcpa_compliant"] = false
		assert.False(t, event.HasComplianceFlag("tcpa_compliant"))
	})

	t.Run("IsGDPRRelevant", func(t *testing.T) {
		event, err := NewEvent(EventDataAccessed, "user123", "user456", "access_profile")
		require.NoError(t, err)

		// Not GDPR relevant initially
		assert.False(t, event.IsGDPRRelevant())

		// Set GDPR flag
		event.ComplianceFlags["gdpr_relevant"] = true
		assert.True(t, event.IsGDPRRelevant())

		// Clear flag, set PII flag
		delete(event.ComplianceFlags, "gdpr_relevant")
		event.ComplianceFlags["contains_pii"] = true
		assert.True(t, event.IsGDPRRelevant())

		// Clear PII flag, add GDPR data class
		delete(event.ComplianceFlags, "contains_pii")
		event.DataClasses = []string{"email", "phone_number"}
		assert.True(t, event.IsGDPRRelevant())
	})

	t.Run("IsTCPARelevant", func(t *testing.T) {
		// Consent event is always TCPA relevant
		consentEvent, err := NewEvent(EventConsentGranted, "user123", "+15551234567", "grant_consent")
		require.NoError(t, err)
		assert.True(t, consentEvent.IsTCPARelevant())

		// Call event is always TCPA relevant
		callEvent, err := NewEvent(EventCallInitiated, "user123", "call456", "initiate_call")
		require.NoError(t, err)
		assert.True(t, callEvent.IsTCPARelevant())

		// Other event with TCPA flag
		otherEvent, err := NewEvent(EventDataAccessed, "user123", "data456", "access_data")
		require.NoError(t, err)
		assert.False(t, otherEvent.IsTCPARelevant())

		otherEvent.ComplianceFlags["tcpa_relevant"] = true
		assert.True(t, otherEvent.IsTCPARelevant())
	})
}

func TestEvent_RetentionHelpers(t *testing.T) {
	t.Run("GetRetentionExpiryDate", func(t *testing.T) {
		event, err := NewEvent(EventConsentGranted, "user123", "+15551234567", "grant_consent")
		require.NoError(t, err)

		baseTime := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
		event.Timestamp = baseTime
		event.RetentionDays = 365

		expectedExpiry := baseTime.AddDate(0, 0, 365)
		assert.Equal(t, expectedExpiry, event.GetRetentionExpiryDate())
	})

	t.Run("IsRetentionExpired", func(t *testing.T) {
		event, err := NewEvent(EventConsentGranted, "user123", "+15551234567", "grant_consent")
		require.NoError(t, err)

		// Event from yesterday with 1 day retention
		event.Timestamp = time.Now().UTC().AddDate(0, 0, -2)
		event.RetentionDays = 1
		assert.True(t, event.IsRetentionExpired())

		// Event from today with 1 day retention
		event.Timestamp = time.Now().UTC()
		event.RetentionDays = 1
		assert.False(t, event.IsRetentionExpired())

		// Event from today with 10 year retention
		event.Timestamp = time.Now().UTC()
		event.RetentionDays = 3650
		assert.False(t, event.IsRetentionExpired())
	})
}

func TestEvent_Clone(t *testing.T) {
	original, err := NewEvent(EventConsentGranted, "user123", "+15551234567", "grant_consent")
	require.NoError(t, err)

	// Set up original with full data
	original.ActorType = "user"
	original.ActorIP = "192.168.1.1"
	original.ActorAgent = "Mozilla/5.0"
	original.TargetType = "phone_number"
	original.TargetOwner = "user123"
	original.ErrorCode = "TEST_ERROR"
	original.ErrorMessage = "Test error message"
	original.RequestID = "req123"
	original.SessionID = "sess456"
	original.CorrelationID = "corr789"
	original.ComplianceFlags = map[string]bool{"tcpa_compliant": true}
	original.DataClasses = []string{"phone_number", "consent"}
	original.LegalBasis = "consent"
	original.Metadata = map[string]interface{}{"test": "value"}
	original.Tags = []string{"tag1", "tag2"}
	original.Signature = "signature123"

	// Make original immutable
	_, err = original.ComputeHash("previous123")
	require.NoError(t, err)

	// Clone the event
	clone := original.Clone()

	// Verify clone is correct
	assert.Equal(t, original.ID, clone.ID)
	assert.Equal(t, original.Type, clone.Type)
	assert.Equal(t, original.ActorID, clone.ActorID)
	assert.Equal(t, original.TargetID, clone.TargetID)
	assert.Equal(t, original.Action, clone.Action)
	assert.Equal(t, original.ComplianceFlags["tcpa_compliant"], clone.ComplianceFlags["tcpa_compliant"])
	assert.Equal(t, original.DataClasses, clone.DataClasses)
	assert.Equal(t, original.Metadata["test"], clone.Metadata["test"])
	assert.Equal(t, original.Tags, clone.Tags)

	// Verify clone is mutable
	assert.False(t, clone.IsImmutable())

	// Verify deep copy (modifying clone doesn't affect original)
	clone.ComplianceFlags["new_flag"] = true
	clone.DataClasses = append(clone.DataClasses, "new_class")
	clone.Metadata["new_key"] = "new_value"
	clone.Tags = append(clone.Tags, "new_tag")

	assert.False(t, original.ComplianceFlags["new_flag"])
	assert.NotContains(t, original.DataClasses, "new_class")
	assert.Nil(t, original.Metadata["new_key"])
	assert.NotContains(t, original.Tags, "new_tag")
}

func TestEvent_HelperFunctions(t *testing.T) {
	t.Run("validateEventType", func(t *testing.T) {
		// Valid event types
		assert.NoError(t, validateEventType(EventConsentGranted))
		assert.NoError(t, validateEventType(EventCallInitiated))
		assert.NoError(t, validateEventType(EventDataAccessed))

		// Invalid event types
		assert.Error(t, validateEventType(""))
		assert.Error(t, validateEventType("invalid.type"))
		assert.Error(t, validateEventType("unknown.event"))
	})

	t.Run("validateSeverity", func(t *testing.T) {
		// Valid severities
		assert.NoError(t, validateSeverity(SeverityInfo))
		assert.NoError(t, validateSeverity(SeverityWarning))
		assert.NoError(t, validateSeverity(SeverityError))
		assert.NoError(t, validateSeverity(SeverityCritical))

		// Invalid severities
		assert.Error(t, validateSeverity("INVALID"))
		assert.Error(t, validateSeverity(""))
	})

	t.Run("isValidResult", func(t *testing.T) {
		// Valid results
		assert.True(t, isValidResult("success"))
		assert.True(t, isValidResult("failure"))
		assert.True(t, isValidResult("partial"))

		// Invalid results
		assert.False(t, isValidResult(""))
		assert.False(t, isValidResult("invalid"))
		assert.False(t, isValidResult("unknown"))
	})

	t.Run("containsGDPRDataClasses", func(t *testing.T) {
		// Contains GDPR data classes
		assert.True(t, containsGDPRDataClasses([]string{"email", "other"}))
		assert.True(t, containsGDPRDataClasses([]string{"phone_number"}))
		assert.True(t, containsGDPRDataClasses([]string{"personal_data", "financial_data"}))

		// Does not contain GDPR data classes
		assert.False(t, containsGDPRDataClasses([]string{"non_sensitive", "other"}))
		assert.False(t, containsGDPRDataClasses([]string{}))
		assert.False(t, containsGDPRDataClasses(nil))
	})

	t.Run("deriveCategory", func(t *testing.T) {
		assert.Equal(t, "consent", deriveCategory(EventConsentGranted))
		assert.Equal(t, "call", deriveCategory(EventCallInitiated))
		assert.Equal(t, "data_access", deriveCategory(EventDataAccessed))
		assert.Equal(t, "security", deriveCategory(EventAuthFailure))
		assert.Equal(t, "marketplace", deriveCategory(EventBidPlaced))
		assert.Equal(t, "financial", deriveCategory(EventPaymentProcessed))
		assert.Equal(t, "system", deriveCategory(EventAPICall))
		assert.Equal(t, "configuration", deriveCategory(EventConfigChanged))
		assert.Equal(t, "other", deriveCategory("unknown.type"))
	})
}

// Property-based tests - following DCE specification for 1000+ iterations

// TestPropertyEventCreationAlwaysSucceedsWithValidInputs tests that event creation with valid inputs always succeeds
func TestPropertyEventCreationAlwaysSucceedsWithValidInputs(t *testing.T) {
	validEventTypes := []EventType{
		EventCallInitiated, EventBidPlaced, EventConsentGranted,
		EventDataAccessed, EventAuthSuccess, EventPaymentProcessed,
		EventConsentRevoked, EventConsentUpdated, EventOptOutRequested,
		EventComplianceViolation, EventTCPAComplianceCheck, EventGDPRDataRequest,
		EventDataExported, EventDataDeleted, EventDataModified,
		EventCallRouted, EventCallCompleted, EventCallFailed,
		EventConfigChanged, EventRuleUpdated, EventPermissionChanged,
		EventAuthFailure, EventAccessDenied, EventAnomalyDetected,
		EventBidWon, EventBidLost, EventBidCancelled, EventAuctionCompleted,
		EventTransactionCompleted, EventChargebackInitiated, EventRefundProcessed,
		EventAPICall, EventDatabaseQuery, EventSystemStartup, EventSystemShutdown,
	}

	for i := 0; i < 1000; i++ {
		// Generate random valid inputs
		eventType := validEventTypes[rand.Intn(len(validEventTypes))]
		actorID := fmt.Sprintf("actor-%d", rand.Intn(10000))
		targetID := fmt.Sprintf("target-%d", rand.Intn(10000))
		action := fmt.Sprintf("action-%d", rand.Intn(10000))

		event, err := NewEvent(eventType, actorID, targetID, action)

		// Should always succeed with valid inputs
		require.NoError(t, err, "iteration %d failed with eventType=%s", i, eventType)
		require.NotNil(t, event, "iteration %d failed", i)

		// Verify invariants
		assert.Equal(t, eventType, event.Type, "iteration %d: event type mismatch", i)
		assert.Equal(t, actorID, event.ActorID, "iteration %d: actor ID mismatch", i)
		assert.Equal(t, targetID, event.TargetID, "iteration %d: target ID mismatch", i)
		assert.Equal(t, action, event.Action, "iteration %d: action mismatch", i)
		assert.False(t, event.IsImmutable(), "iteration %d: event should be mutable initially", i)
		assert.NotZero(t, event.Timestamp, "iteration %d: timestamp should be set", i)
		assert.Greater(t, event.RetentionDays, 0, "iteration %d: retention days must be positive", i)
		assert.NotNil(t, event.Metadata, "iteration %d: metadata should be initialized", i)
		assert.NotNil(t, event.ComplianceFlags, "iteration %d: compliance flags should be initialized", i)

		// Validation should pass
		err = event.Validate()
		assert.NoError(t, err, "validation failed for iteration %d", i)
	}
}

// TestPropertyHashComputationIsDeterministic tests that hash computation is deterministic
func TestPropertyHashComputationIsDeterministic(t *testing.T) {
	for i := 0; i < 1000; i++ {
		// Create event
		event, err := NewEvent(EventCallInitiated, "actor-123", "target-456", "test-action")
		require.NoError(t, err, "iteration %d: event creation failed", i)

		// Generate random previous hash
		prevHash := fmt.Sprintf("prev-hash-%d", rand.Intn(100000))

		// Compute hash
		hash1, err := event.ComputeHash(prevHash)
		require.NoError(t, err, "iteration %d: first hash computation failed", i)

		// Clone and compute hash with same data
		clone := event.Clone()
		clone.ID = event.ID
		clone.Timestamp = event.Timestamp
		clone.TimestampNano = event.TimestampNano
		clone.SequenceNum = event.SequenceNum

		hash2, err := clone.ComputeHash(prevHash)
		require.NoError(t, err, "iteration %d: second hash computation failed", i)

		// Hashes must be identical
		assert.Equal(t, hash1, hash2, "iteration %d: hashes differ", i)
	}
}

// TestPropertyEventImmutabilityInvariant tests that once immutable, events cannot be re-hashed
func TestPropertyEventImmutabilityInvariant(t *testing.T) {
	for i := 0; i < 1000; i++ {
		event, err := NewEvent(EventCallInitiated, "actor-123", "target-456", "test-action")
		require.NoError(t, err, "iteration %d: event creation failed", i)

		// Make immutable
		prevHash := fmt.Sprintf("prev-hash-%d", i)
		_, err = event.ComputeHash(prevHash)
		require.NoError(t, err, "iteration %d: hash computation failed", i)
		require.True(t, event.IsImmutable(), "iteration %d: event should be immutable", i)

		// Attempting to compute hash again should fail
		differentHash := fmt.Sprintf("different-hash-%d", i)
		_, err = event.ComputeHash(differentHash)
		require.Error(t, err, "iteration %d: re-hashing should fail", i)

		var appErr *errors.AppError
		require.ErrorAs(t, err, &appErr, "iteration %d: should be AppError", i)
		assert.Equal(t, "EVENT_IMMUTABLE", appErr.Code, "iteration %d: wrong error code", i)
	}
}

// TestPropertyGDPRComplianceDetection tests GDPR compliance flag detection across scenarios
func TestPropertyGDPRComplianceDetection(t *testing.T) {
	gdprDataClasses := []string{
		"personal_data", "sensitive_data", "biometric_data",
		"health_data", "email", "phone_number", "ip_address",
		"location_data", "financial_data",
	}

	for i := 0; i < 1000; i++ {
		event, err := NewEvent(EventDataAccessed, "actor-123", "target-456", "access-data")
		require.NoError(t, err, "iteration %d: event creation failed", i)

		// Test different GDPR detection scenarios
		scenario := rand.Intn(3)
		switch scenario {
		case 0:
			// Set flag directly
			event.ComplianceFlags["gdpr_relevant"] = true
		case 1:
			// Set contains_pii flag
			event.ComplianceFlags["contains_pii"] = true
		case 2:
			// Set GDPR data class
			dataClass := gdprDataClasses[rand.Intn(len(gdprDataClasses))]
			event.DataClasses = []string{dataClass}
		}

		// Should be detected as GDPR relevant
		assert.True(t, event.IsGDPRRelevant(), "iteration %d (scenario %d): event should be GDPR relevant", i, scenario)
	}
}

// TestPropertyTCPAComplianceDetection tests TCPA compliance detection
func TestPropertyTCPAComplianceDetection(t *testing.T) {
	tcpaEventTypes := []EventType{
		EventConsentGranted, EventConsentRevoked, EventCallInitiated,
	}

	for i := 0; i < 1000; i++ {
		eventType := tcpaEventTypes[rand.Intn(len(tcpaEventTypes))]
		event, err := NewEvent(eventType, "actor-123", "target-456", "test-action")
		require.NoError(t, err, "iteration %d: event creation failed", i)

		// TCPA events should be detected
		assert.True(t, event.IsTCPARelevant(), "iteration %d: event type %s should be TCPA relevant", i, eventType)
	}

	// Test non-TCPA events with flag
	for i := 0; i < 100; i++ {
		event, err := NewEvent(EventDataAccessed, "actor-123", "target-456", "test-action")
		require.NoError(t, err, "iteration %d: event creation failed", i)

		// Initially not TCPA relevant
		assert.False(t, event.IsTCPARelevant(), "iteration %d: should not be TCPA relevant initially", i)

		// Set flag
		event.ComplianceFlags["tcpa_relevant"] = true
		assert.True(t, event.IsTCPARelevant(), "iteration %d: should be TCPA relevant with flag", i)
	}
}

// TestPropertyRetentionPeriodAlwaysPositive tests that retention period is always positive
func TestPropertyRetentionPeriodAlwaysPositive(t *testing.T) {
	validEventTypes := []EventType{
		EventCallInitiated, EventBidPlaced, EventConsentGranted,
		EventDataAccessed, EventAuthSuccess, EventPaymentProcessed,
	}

	for i := 0; i < 1000; i++ {
		eventType := validEventTypes[rand.Intn(len(validEventTypes))]
		event, err := NewEvent(eventType, "actor-123", "target-456", "test-action")
		require.NoError(t, err, "iteration %d: event creation failed", i)

		// Retention days should always be positive
		assert.Greater(t, event.RetentionDays, 0, "iteration %d: retention days must be positive", i)

		// Expiry date should be in the future for new events
		expiryDate := event.GetRetentionExpiryDate()
		assert.True(t, expiryDate.After(event.Timestamp), "iteration %d: expiry should be after creation", i)

		// Test setting random valid retention periods
		randomDays := rand.Intn(7300) + 1 // 1 to 20 years
		event.RetentionDays = randomDays
		
		expectedExpiry := event.Timestamp.AddDate(0, 0, randomDays)
		actualExpiry := event.GetRetentionExpiryDate()
		assert.True(t, actualExpiry.Equal(expectedExpiry), "iteration %d: expiry calculation incorrect", i)
	}
}

// TestPropertyEventClonePreservesData tests that event cloning preserves all data correctly
func TestPropertyEventClonePreservesData(t *testing.T) {
	for i := 0; i < 1000; i++ {
		// Create original event with random data
		original, err := NewEvent(EventCallInitiated, fmt.Sprintf("actor-%d", i), fmt.Sprintf("target-%d", i), fmt.Sprintf("action-%d", i))
		require.NoError(t, err, "iteration %d: event creation failed", i)

		// Add random complex data
		original.ComplianceFlags[fmt.Sprintf("flag_%d", i)] = rand.Float32() < 0.5
		original.DataClasses = []string{fmt.Sprintf("class_%d", i), fmt.Sprintf("class_%d_extra", i)}
		original.Tags = []string{fmt.Sprintf("tag_%d", i)}
		original.Metadata[fmt.Sprintf("key_%d", i)] = fmt.Sprintf("value_%d", i)
		original.ActorType = fmt.Sprintf("type_%d", i)
		original.ErrorCode = fmt.Sprintf("error_%d", i)

		// Make it immutable randomly
		if rand.Float32() < 0.5 {
			_, err = original.ComputeHash(fmt.Sprintf("prev-hash-%d", i))
			require.NoError(t, err, "iteration %d: hash computation failed", i)
		}

		// Clone the event
		clone := original.Clone()

		// Verify core fields are preserved
		assert.Equal(t, original.ID, clone.ID, "iteration %d: ID not preserved", i)
		assert.Equal(t, original.Type, clone.Type, "iteration %d: Type not preserved", i)
		assert.Equal(t, original.ActorID, clone.ActorID, "iteration %d: ActorID not preserved", i)
		assert.Equal(t, original.TargetID, clone.TargetID, "iteration %d: TargetID not preserved", i)
		assert.Equal(t, original.Action, clone.Action, "iteration %d: Action not preserved", i)
		assert.Equal(t, original.EventHash, clone.EventHash, "iteration %d: EventHash not preserved", i)
		assert.Equal(t, original.ActorType, clone.ActorType, "iteration %d: ActorType not preserved", i)
		assert.Equal(t, original.ErrorCode, clone.ErrorCode, "iteration %d: ErrorCode not preserved", i)

		// Clone should always be mutable
		assert.False(t, clone.IsImmutable(), "iteration %d: clone should be mutable", i)

		// Verify deep copy - modifications to clone should not affect original
		clone.ComplianceFlags["new_flag"] = true
		clone.DataClasses = append(clone.DataClasses, "new_class")
		clone.Tags = append(clone.Tags, "new_tag")
		clone.Metadata["new_key"] = "new_value"

		assert.False(t, original.ComplianceFlags["new_flag"], "iteration %d: original compliance flags modified", i)
		assert.NotContains(t, original.DataClasses, "new_class", "iteration %d: original data classes modified", i)
		assert.NotContains(t, original.Tags, "new_tag", "iteration %d: original tags modified", i)
		assert.Nil(t, original.Metadata["new_key"], "iteration %d: original metadata modified", i)
	}
}

// TestPropertyEventValidationConsistency tests that validation is consistent
func TestPropertyEventValidationConsistency(t *testing.T) {
	for i := 0; i < 1000; i++ {
		event, err := NewEvent(EventCallInitiated, "actor-123", "target-456", "test-action")
		require.NoError(t, err, "iteration %d: event creation failed", i)

		// Valid event should always pass validation
		err = event.Validate()
		assert.NoError(t, err, "iteration %d: valid event should pass validation", i)

		// Test different invalid scenarios
		scenarios := []struct {
			name     string
			modifyFn func(*Event)
			errCode  string
		}{
			{
				name: "empty actor ID",
				modifyFn: func(e *Event) { e.ActorID = "" },
				errCode:  "MISSING_ACTOR_ID",
			},
			{
				name: "empty target ID",
				modifyFn: func(e *Event) { e.TargetID = "" },
				errCode:  "MISSING_TARGET_ID",
			},
			{
				name: "empty action",
				modifyFn: func(e *Event) { e.Action = "" },
				errCode:  "MISSING_ACTION",
			},
			{
				name: "invalid result",
				modifyFn: func(e *Event) { e.Result = "invalid" },
				errCode:  "INVALID_RESULT",
			},
			{
				name: "negative retention",
				modifyFn: func(e *Event) { e.RetentionDays = -1 },
				errCode:  "INVALID_RETENTION",
			},
		}

		scenarioIdx := rand.Intn(len(scenarios))
		scenario := scenarios[scenarioIdx]

		// Create a copy and make it invalid
		invalidEvent := event.Clone()
		scenario.modifyFn(invalidEvent)

		// Should fail validation
		err = invalidEvent.Validate()
		require.Error(t, err, "iteration %d (%s): invalid event should fail validation", i, scenario.name)

		var appErr *errors.AppError
		require.ErrorAs(t, err, &appErr, "iteration %d (%s): should be AppError", i, scenario.name)
		assert.Equal(t, scenario.errCode, appErr.Code, "iteration %d (%s): wrong error code", i, scenario.name)
	}
}

// TestPropertyJSONSerializationRoundTrip tests JSON serialization round-trip integrity
func TestPropertyJSONSerializationRoundTrip(t *testing.T) {
	for i := 0; i < 1000; i++ {
		// Create event with random data
		original, err := NewEvent(EventCallInitiated, fmt.Sprintf("actor-%d", i), fmt.Sprintf("target-%d", i), fmt.Sprintf("action-%d", i))
		require.NoError(t, err, "iteration %d: event creation failed", i)

		// Add complex data
		original.ComplianceFlags[fmt.Sprintf("flag_%d", i)] = rand.Float32() < 0.5
		original.DataClasses = []string{fmt.Sprintf("class_%d", i)}
		original.Tags = []string{fmt.Sprintf("tag_%d", i)}
		original.Metadata[fmt.Sprintf("key_%d", i)] = rand.Intn(1000)

		// Serialize to JSON
		jsonData, err := json.Marshal(original)
		require.NoError(t, err, "iteration %d: JSON marshaling failed", i)

		// Deserialize from JSON
		var deserialized Event
		err = json.Unmarshal(jsonData, &deserialized)
		require.NoError(t, err, "iteration %d: JSON unmarshaling failed", i)

		// Verify key fields are preserved
		assert.Equal(t, original.ID, deserialized.ID, "iteration %d: ID not preserved in JSON", i)
		assert.Equal(t, original.Type, deserialized.Type, "iteration %d: Type not preserved in JSON", i)
		assert.Equal(t, original.ActorID, deserialized.ActorID, "iteration %d: ActorID not preserved in JSON", i)
		assert.Equal(t, original.TargetID, deserialized.TargetID, "iteration %d: TargetID not preserved in JSON", i)
		assert.Equal(t, original.Action, deserialized.Action, "iteration %d: Action not preserved in JSON", i)
		assert.Equal(t, original.RetentionDays, deserialized.RetentionDays, "iteration %d: RetentionDays not preserved in JSON", i)
	}
}

// Benchmark tests

// BenchmarkEventCreation benchmarks event creation performance
func BenchmarkEventCreation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := NewEvent(EventCallInitiated, "actor-123", "target-456", "test-action")
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkEventHashComputation benchmarks hash computation performance
func BenchmarkEventHashComputation(b *testing.B) {
	events := make([]*Event, b.N)
	for i := 0; i < b.N; i++ {
		event, err := NewEvent(EventCallInitiated, "actor-123", "target-456", "test-action")
		if err != nil {
			b.Fatal(err)
		}
		events[i] = event
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := events[i].ComputeHash("prev-hash")
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkEventValidation benchmarks event validation performance
func BenchmarkEventValidation(b *testing.B) {
	event, err := NewEvent(EventCallInitiated, "actor-123", "target-456", "test-action")
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := event.Validate()
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkEventClone benchmarks event cloning performance
func BenchmarkEventClone(b *testing.B) {
	event, err := NewEvent(EventCallInitiated, "actor-123", "target-456", "test-action")
	if err != nil {
		b.Fatal(err)
	}
	
	// Add complex data
	event.ComplianceFlags["gdpr_relevant"] = true
	event.DataClasses = []string{"personal_data", "phone_number"}
	event.Tags = []string{"tag1", "tag2"}
	event.Metadata["key1"] = "value1"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = event.Clone()
	}
}

// BenchmarkEventJSONSerialization benchmarks JSON serialization performance
func BenchmarkEventJSONSerialization(b *testing.B) {
	event, err := NewEvent(EventCallInitiated, "actor-123", "target-456", "test-action")
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := json.Marshal(event)
		if err != nil {
			b.Fatal(err)
		}
	}
}