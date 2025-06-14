package audit

import (
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