package consent

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewConsentAggregate(t *testing.T) {
	tests := []struct {
		name        string
		consumerID  uuid.UUID
		businessID  uuid.UUID
		consentType Type
		channels    []Channel
		purpose     Purpose
		source      ConsentSource
		wantErr     bool
		errCode     string
	}{
		{
			name:        "valid consent creation",
			consumerID:  uuid.New(),
			businessID:  uuid.New(),
			consentType: TypeTCPA,
			channels:    []Channel{ChannelVoice, ChannelSMS},
			purpose:     PurposeMarketing,
			source:      SourceWebForm,
			wantErr:     false,
		},
		{
			name:        "missing consumer ID",
			consumerID:  uuid.Nil,
			businessID:  uuid.New(),
			consentType: TypeTCPA,
			channels:    []Channel{ChannelVoice},
			purpose:     PurposeMarketing,
			source:      SourceWebForm,
			wantErr:     true,
			errCode:     "INVALID_CONSUMER",
		},
		{
			name:        "missing business ID",
			consumerID:  uuid.New(),
			businessID:  uuid.Nil,
			consentType: TypeTCPA,
			channels:    []Channel{ChannelVoice},
			purpose:     PurposeMarketing,
			source:      SourceWebForm,
			wantErr:     true,
			errCode:     "INVALID_BUSINESS",
		},
		{
			name:        "no channels specified",
			consumerID:  uuid.New(),
			businessID:  uuid.New(),
			consentType: TypeTCPA,
			channels:    []Channel{},
			purpose:     PurposeMarketing,
			source:      SourceWebForm,
			wantErr:     true,
			errCode:     "NO_CHANNELS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			consent, err := NewConsentAggregate(tt.consumerID, tt.businessID, tt.consentType, tt.channels, tt.purpose, tt.source)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errCode)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, consent)
				assert.NotEqual(t, uuid.Nil, consent.ID)
				assert.Equal(t, tt.consumerID, consent.ConsumerID)
				assert.Equal(t, tt.businessID, consent.BusinessID)
				assert.Equal(t, 1, consent.CurrentVersion)
				assert.Len(t, consent.Versions, 1)
				assert.Equal(t, StatusPending, consent.Versions[0].Status)
				assert.Equal(t, tt.channels, consent.Versions[0].Channels)
				assert.Equal(t, tt.purpose, consent.Versions[0].Purpose)
				assert.Equal(t, tt.source, consent.Versions[0].Source)
				assert.Len(t, consent.GetEvents(), 1)
			}
		})
	}
}

func TestConsentActivation(t *testing.T) {
	tests := []struct {
		name      string
		setup     func() *ConsentAggregate
		proofs    []ConsentProof
		expiresAt *time.Time
		wantErr   bool
		errCode   string
	}{
		{
			name: "successful activation with recording proof",
			setup: func() *ConsentAggregate {
				c, _ := NewConsentAggregate(
					uuid.New(), uuid.New(),
					TypeTCPA,
					[]Channel{ChannelVoice},
					PurposeMarketing,
					SourceVoiceRecording,
				)
				return c
			},
			proofs: []ConsentProof{
				{
					ID:              uuid.New(),
					Type:            ProofTypeRecording,
					StorageLocation: "s3://bucket/recording.mp3",
					Hash:            "sha256:abcd1234...",
					Metadata: ProofMetadata{
						TCPALanguage: "By pressing 1, you consent...",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "activation with expiration",
			setup: func() *ConsentAggregate {
				c, _ := NewConsentAggregate(
					uuid.New(), uuid.New(),
					TypeTCPA,
					[]Channel{ChannelVoice, ChannelSMS},
					PurposeMarketing,
					SourceWebForm,
				)
				return c
			},
			proofs: []ConsentProof{
				{
					ID:              uuid.New(),
					Type:            ProofTypeFormSubmission,
					StorageLocation: "s3://bucket/form.pdf",
					Hash:            "sha256:efgh5678...",
				},
			},
			expiresAt: func() *time.Time {
				t := time.Now().Add(365 * 24 * time.Hour)
				return &t
			}(),
			wantErr: false,
		},
		{
			name: "fails without proof",
			setup: func() *ConsentAggregate {
				c, _ := NewConsentAggregate(
					uuid.New(), uuid.New(),
					TypeTCPA,
					[]Channel{ChannelVoice},
					PurposeMarketing,
					SourceVoiceRecording,
				)
				return c
			},
			proofs:  []ConsentProof{},
			wantErr: true,
			errCode: "NO_PROOF",
		},
		{
			name: "fails when already active",
			setup: func() *ConsentAggregate {
				c, _ := NewConsentAggregate(
					uuid.New(), uuid.New(),
					TypeTCPA,
					[]Channel{ChannelVoice},
					PurposeMarketing,
					SourceVoiceRecording,
				)
				// Activate it first
				c.ActivateConsent([]ConsentProof{{
					ID:              uuid.New(),
					Type:            ProofTypeRecording,
					StorageLocation: "s3://bucket/recording.mp3",
					Hash:            "sha256:abcd1234...",
				}}, nil)
				return c
			},
			proofs: []ConsentProof{{
				ID:              uuid.New(),
				Type:            ProofTypeRecording,
				StorageLocation: "s3://bucket/recording2.mp3",
				Hash:            "sha256:xyz789...",
			}},
			wantErr: true,
			errCode: "INVALID_STATE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			consent := tt.setup()
			initialEventCount := len(consent.GetEvents())

			err := consent.ActivateConsent(tt.proofs, tt.expiresAt)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errCode)
			} else {
				require.NoError(t, err)
				
				// Check version is updated
				current := consent.getCurrentVersion()
				assert.Equal(t, StatusActive, current.Status)
				assert.NotNil(t, current.ConsentedAt)
				assert.Equal(t, tt.proofs, current.Proofs)
				
				if tt.expiresAt != nil {
					assert.Equal(t, *tt.expiresAt, *current.ExpiresAt)
				}

				// Check event was emitted
				assert.Len(t, consent.GetEvents(), initialEventCount+1)
			}
		})
	}
}

func TestConsentRevocation(t *testing.T) {
	tests := []struct {
		name      string
		setup     func() *ConsentAggregate
		reason    string
		revokedBy uuid.UUID
		wantErr   bool
		errCode   string
	}{
		{
			name: "successful revocation",
			setup: func() *ConsentAggregate {
				c, _ := NewConsentAggregate(
					uuid.New(), uuid.New(),
					TypeTCPA,
					[]Channel{ChannelVoice, ChannelSMS},
					PurposeMarketing,
					SourceWebForm,
				)
				c.ActivateConsent([]ConsentProof{{
					ID:              uuid.New(),
					Type:            ProofTypeFormSubmission,
					StorageLocation: "s3://bucket/form.pdf",
					Hash:            "sha256:abcd1234...",
				}}, nil)
				return c
			},
			reason:    "Consumer requested opt-out",
			revokedBy: uuid.New(),
			wantErr:   false,
		},
		{
			name: "fails when not active",
			setup: func() *ConsentAggregate {
				c, _ := NewConsentAggregate(
					uuid.New(), uuid.New(),
					TypeTCPA,
					[]Channel{ChannelVoice},
					PurposeMarketing,
					SourceWebForm,
				)
				// Leave in pending state
				return c
			},
			reason:    "Consumer requested opt-out",
			revokedBy: uuid.New(),
			wantErr:   true,
			errCode:   "NOT_ACTIVE",
		},
		{
			name: "fails when already revoked",
			setup: func() *ConsentAggregate {
				c, _ := NewConsentAggregate(
					uuid.New(), uuid.New(),
					TypeTCPA,
					[]Channel{ChannelVoice},
					PurposeMarketing,
					SourceWebForm,
				)
				c.ActivateConsent([]ConsentProof{{
					ID:              uuid.New(),
					Type:            ProofTypeFormSubmission,
					StorageLocation: "s3://bucket/form.pdf",
					Hash:            "sha256:abcd1234...",
				}}, nil)
				c.RevokeConsent("First revocation", uuid.New())
				return c
			},
			reason:    "Second revocation attempt",
			revokedBy: uuid.New(),
			wantErr:   true,
			errCode:   "ALREADY_REVOKED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			consent := tt.setup()
			initialVersion := consent.CurrentVersion
			initialEventCount := len(consent.GetEvents())

			err := consent.RevokeConsent(tt.reason, tt.revokedBy)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errCode)
			} else {
				require.NoError(t, err)
				
				// Check new version was created
				assert.Equal(t, initialVersion+1, consent.CurrentVersion)
				assert.Len(t, consent.Versions, initialVersion+1)
				
				// Check current version
				current := consent.getCurrentVersion()
				assert.Equal(t, StatusRevoked, current.Status)
				assert.NotNil(t, current.RevokedAt)
				assert.Equal(t, tt.reason, current.SourceDetails["revoke_reason"])
				assert.Equal(t, tt.revokedBy, current.CreatedBy)

				// Check event was emitted
				assert.Len(t, consent.GetEvents(), initialEventCount+1)
			}
		})
	}
}

func TestConsentChannelUpdate(t *testing.T) {
	tests := []struct {
		name        string
		setup       func() *ConsentAggregate
		newChannels []Channel
		updatedBy   uuid.UUID
		wantErr     bool
		errCode     string
	}{
		{
			name: "successful channel update",
			setup: func() *ConsentAggregate {
				c, _ := NewConsentAggregate(
					uuid.New(), uuid.New(),
					TypeTCPA,
					[]Channel{ChannelVoice},
					PurposeMarketing,
					SourceWebForm,
				)
				c.ActivateConsent([]ConsentProof{{
					ID:              uuid.New(),
					Type:            ProofTypeFormSubmission,
					StorageLocation: "s3://bucket/form.pdf",
					Hash:            "sha256:abcd1234...",
				}}, nil)
				return c
			},
			newChannels: []Channel{ChannelVoice, ChannelSMS, ChannelEmail},
			updatedBy:   uuid.New(),
			wantErr:     false,
		},
		{
			name: "remove channels",
			setup: func() *ConsentAggregate {
				c, _ := NewConsentAggregate(
					uuid.New(), uuid.New(),
					TypeTCPA,
					[]Channel{ChannelVoice, ChannelSMS, ChannelEmail},
					PurposeMarketing,
					SourceWebForm,
				)
				c.ActivateConsent([]ConsentProof{{
					ID:              uuid.New(),
					Type:            ProofTypeFormSubmission,
					StorageLocation: "s3://bucket/form.pdf",
					Hash:            "sha256:abcd1234...",
				}}, nil)
				return c
			},
			newChannels: []Channel{ChannelEmail}, // Remove voice and SMS
			updatedBy:   uuid.New(),
			wantErr:     false,
		},
		{
			name: "fails with empty channels",
			setup: func() *ConsentAggregate {
				c, _ := NewConsentAggregate(
					uuid.New(), uuid.New(),
					TypeTCPA,
					[]Channel{ChannelVoice},
					PurposeMarketing,
					SourceWebForm,
				)
				c.ActivateConsent([]ConsentProof{{
					ID:              uuid.New(),
					Type:            ProofTypeFormSubmission,
					StorageLocation: "s3://bucket/form.pdf",
					Hash:            "sha256:abcd1234...",
				}}, nil)
				return c
			},
			newChannels: []Channel{},
			updatedBy:   uuid.New(),
			wantErr:     true,
			errCode:     "NO_CHANNELS",
		},
		{
			name: "fails when not active",
			setup: func() *ConsentAggregate {
				c, _ := NewConsentAggregate(
					uuid.New(), uuid.New(),
					TypeTCPA,
					[]Channel{ChannelVoice},
					PurposeMarketing,
					SourceWebForm,
				)
				// Leave in pending state
				return c
			},
			newChannels: []Channel{ChannelVoice, ChannelSMS},
			updatedBy:   uuid.New(),
			wantErr:     true,
			errCode:     "NOT_ACTIVE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			consent := tt.setup()
			initialVersion := consent.CurrentVersion
			initialEventCount := len(consent.GetEvents())

			err := consent.UpdateChannels(tt.newChannels, tt.updatedBy)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errCode)
			} else {
				require.NoError(t, err)
				
				// Check new version was created
				assert.Equal(t, initialVersion+1, consent.CurrentVersion)
				assert.Len(t, consent.Versions, initialVersion+1)
				
				// Check current version
				current := consent.getCurrentVersion()
				assert.Equal(t, StatusActive, current.Status)
				assert.Equal(t, tt.newChannels, current.Channels)
				assert.Equal(t, tt.updatedBy, current.CreatedBy)

				// Check event was emitted
				assert.Len(t, consent.GetEvents(), initialEventCount+1)
			}
		})
	}
}

func TestConsentStatusChecks(t *testing.T) {
	t.Run("IsActive checks", func(t *testing.T) {
		// Create active consent
		consent, _ := NewConsentAggregate(
			uuid.New(), uuid.New(),
			TypeTCPA,
			[]Channel{ChannelVoice, ChannelSMS},
			PurposeMarketing,
			SourceWebForm,
		)

		// Initially pending
		assert.False(t, consent.IsActive())
		assert.Equal(t, StatusPending, consent.GetCurrentStatus())

		// Activate
		consent.ActivateConsent([]ConsentProof{{
			ID:              uuid.New(),
			Type:            ProofTypeFormSubmission,
			StorageLocation: "s3://bucket/form.pdf",
			Hash:            "sha256:abcd1234...",
		}}, nil)

		// Now active
		assert.True(t, consent.IsActive())
		assert.Equal(t, StatusActive, consent.GetCurrentStatus())

		// Revoke
		consent.RevokeConsent("Test revocation", uuid.New())

		// No longer active
		assert.False(t, consent.IsActive())
		assert.Equal(t, StatusRevoked, consent.GetCurrentStatus())
	})

	t.Run("Expired consent", func(t *testing.T) {
		consent, _ := NewConsentAggregate(
			uuid.New(), uuid.New(),
			TypeTCPA,
			[]Channel{ChannelVoice},
			PurposeMarketing,
			SourceWebForm,
		)

		// Activate with past expiration
		pastTime := time.Now().Add(-1 * time.Hour)
		consent.ActivateConsent([]ConsentProof{{
			ID:              uuid.New(),
			Type:            ProofTypeFormSubmission,
			StorageLocation: "s3://bucket/form.pdf",
			Hash:            "sha256:abcd1234...",
		}}, &pastTime)

		// Should not be active due to expiration
		assert.False(t, consent.IsActive())
		assert.Equal(t, StatusExpired, consent.GetCurrentStatus())
	})
}

func TestHasChannelConsent(t *testing.T) {
	consent, _ := NewConsentAggregate(
		uuid.New(), uuid.New(),
		TypeTCPA,
		[]Channel{ChannelVoice, ChannelSMS},
		PurposeMarketing,
		SourceWebForm,
	)

	// Activate consent
	consent.ActivateConsent([]ConsentProof{{
		ID:              uuid.New(),
		Type:            ProofTypeFormSubmission,
		StorageLocation: "s3://bucket/form.pdf",
		Hash:            "sha256:abcd1234...",
	}}, nil)

	// Check channels
	assert.True(t, consent.HasChannelConsent(ChannelVoice))
	assert.True(t, consent.HasChannelConsent(ChannelSMS))
	assert.False(t, consent.HasChannelConsent(ChannelEmail))
	assert.False(t, consent.HasChannelConsent(ChannelFax))

	// Check active channels
	activeChannels := consent.GetActiveChannels()
	assert.Len(t, activeChannels, 2)
	assert.Contains(t, activeChannels, ChannelVoice)
	assert.Contains(t, activeChannels, ChannelSMS)
}

func TestValidationHelpers(t *testing.T) {
	t.Run("ValidateChannel", func(t *testing.T) {
		assert.NoError(t, ValidateChannel(ChannelVoice))
		assert.NoError(t, ValidateChannel(ChannelSMS))
		assert.NoError(t, ValidateChannel(ChannelEmail))
		assert.NoError(t, ValidateChannel(ChannelFax))
		assert.Error(t, ValidateChannel("invalid"))
	})

	t.Run("ValidatePurpose", func(t *testing.T) {
		assert.NoError(t, ValidatePurpose(PurposeMarketing))
		assert.NoError(t, ValidatePurpose(PurposeServiceCalls))
		assert.NoError(t, ValidatePurpose(PurposeDebtCollection))
		assert.NoError(t, ValidatePurpose(PurposeEmergency))
		assert.Error(t, ValidatePurpose("invalid"))
	})

	t.Run("ValidateSource", func(t *testing.T) {
		assert.NoError(t, ValidateSource(SourceWebForm))
		assert.NoError(t, ValidateSource(SourceVoiceRecording))
		assert.NoError(t, ValidateSource(SourceSMS))
		assert.NoError(t, ValidateSource(SourceEmailReply))
		assert.NoError(t, ValidateSource(SourceAPI))
		assert.NoError(t, ValidateSource(SourceImport))
		assert.Error(t, ValidateSource("invalid"))
	})
}