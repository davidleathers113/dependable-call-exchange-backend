package database

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/consent"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/testutil"
)

func TestConsentRepository_Save(t *testing.T) {
	ctx := context.Background()
	db := testutil.NewTestDB(t)
	repo := NewConsentRepository(db.PgxPool())

	t.Run("save new consent aggregate", func(t *testing.T) {
		// Create a new consent
		consentAgg, err := consent.NewConsentAggregate(
			uuid.New(), uuid.New(),
			consent.TypeTCPA,
			[]consent.Channel{consent.ChannelVoice, consent.ChannelSMS},
			consent.PurposeMarketing,
			consent.SourceWebForm,
		)
		require.NoError(t, err)

		// Save it
		err = repo.Save(ctx, consentAgg)
		require.NoError(t, err)

		// Retrieve and verify
		retrieved, err := repo.GetByID(ctx, consentAgg.ID)
		require.NoError(t, err)
		assert.Equal(t, consentAgg.ID, retrieved.ID)
		assert.Equal(t, consentAgg.ConsumerID, retrieved.ConsumerID)
		assert.Equal(t, consentAgg.BusinessID, retrieved.BusinessID)
		assert.Equal(t, consentAgg.CurrentVersion, retrieved.CurrentVersion)
		assert.Len(t, retrieved.Versions, 1)
	})

	t.Run("update existing consent aggregate", func(t *testing.T) {
		// Create and save initial consent
		consentAgg, err := consent.NewConsentAggregate(
			uuid.New(), uuid.New(),
			consent.TypeTCPA,
			[]consent.Channel{consent.ChannelVoice},
			consent.PurposeMarketing,
			consent.SourceWebForm,
		)
		require.NoError(t, err)
		err = repo.Save(ctx, consentAgg)
		require.NoError(t, err)

		// Activate consent
		err = consentAgg.ActivateConsent([]consent.ConsentProof{{
			ID:              uuid.New(),
			Type:            consent.ProofTypeFormSubmission,
			StorageLocation: "s3://bucket/form.pdf",
			Hash:            "sha256:abcd1234",
		}}, nil)
		require.NoError(t, err)

		// Save updated aggregate
		err = repo.Save(ctx, consentAgg)
		require.NoError(t, err)

		// Retrieve and verify
		retrieved, err := repo.GetByID(ctx, consentAgg.ID)
		require.NoError(t, err)
		assert.Equal(t, 2, retrieved.CurrentVersion)
		assert.Len(t, retrieved.Versions, 2)
		assert.Equal(t, consent.StatusActive, retrieved.Versions[1].Status)
	})

	t.Run("save with proofs", func(t *testing.T) {
		consentAgg, err := consent.NewConsentAggregate(
			uuid.New(), uuid.New(),
			consent.TypeTCPA,
			[]consent.Channel{consent.ChannelVoice},
			consent.PurposeMarketing,
			consent.SourceVoiceRecording,
		)
		require.NoError(t, err)

		// Activate with proof
		proof := consent.ConsentProof{
			ID:              uuid.New(),
			Type:            consent.ProofTypeRecording,
			StorageLocation: "s3://bucket/recording.mp3",
			Hash:            "sha256:xyz789",
			Metadata: consent.ProofMetadata{
				Duration:     &[]time.Duration{45 * time.Second}[0],
				TCPALanguage: "By pressing 1, you consent...",
			},
		}
		err = consentAgg.ActivateConsent([]consent.ConsentProof{proof}, nil)
		require.NoError(t, err)

		// Save
		err = repo.Save(ctx, consentAgg)
		require.NoError(t, err)

		// Retrieve and verify proofs
		retrieved, err := repo.GetByID(ctx, consentAgg.ID)
		require.NoError(t, err)
		assert.Len(t, retrieved.Versions[1].Proofs, 1)
		assert.Equal(t, proof.Type, retrieved.Versions[1].Proofs[0].Type)
		assert.Equal(t, proof.StorageLocation, retrieved.Versions[1].Proofs[0].StorageLocation)
		assert.Equal(t, proof.Metadata.TCPALanguage, retrieved.Versions[1].Proofs[0].Metadata.TCPALanguage)
	})
}

func TestConsentRepository_GetByID(t *testing.T) {
	ctx := context.Background()
	db := testutil.NewTestDB(t)
	repo := NewConsentRepository(db.PgxPool())

	t.Run("get existing consent", func(t *testing.T) {
		// Create and save consent
		consentAgg, _ := consent.NewConsentAggregate(
			uuid.New(), uuid.New(),
			consent.TypeTCPA,
			[]consent.Channel{consent.ChannelVoice, consent.ChannelSMS},
			consent.PurposeMarketing,
			consent.SourceWebForm,
		)
		repo.Save(ctx, consentAgg)

		// Retrieve
		retrieved, err := repo.GetByID(ctx, consentAgg.ID)
		require.NoError(t, err)
		assert.NotNil(t, retrieved)
		assert.Equal(t, consentAgg.ID, retrieved.ID)
	})

	t.Run("get non-existent consent", func(t *testing.T) {
		retrieved, err := repo.GetByID(ctx, uuid.New())
		assert.Error(t, err)
		assert.Nil(t, retrieved)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestConsentRepository_GetByConsumerAndBusiness(t *testing.T) {
	ctx := context.Background()
	db := testutil.NewTestDB(t)
	repo := NewConsentRepository(db.PgxPool())

	consumerID := uuid.New()
	businessID := uuid.New()

	// Create multiple consents for same consumer-business pair
	for i := 0; i < 3; i++ {
		consentAgg, _ := consent.NewConsentAggregate(
			consumerID, businessID,
			consent.TypeTCPA,
			[]consent.Channel{consent.ChannelVoice},
			consent.PurposeMarketing,
			consent.SourceWebForm,
		)
		repo.Save(ctx, consentAgg)
		time.Sleep(10 * time.Millisecond) // Ensure different timestamps
	}

	// Retrieve
	consents, err := repo.GetByConsumerAndBusiness(ctx, consumerID, businessID)
	require.NoError(t, err)
	assert.Len(t, consents, 3)

	// Should be ordered by created_at DESC
	for i := 1; i < len(consents); i++ {
		assert.True(t, consents[i-1].CreatedAt.After(consents[i].CreatedAt))
	}
}

func TestConsentRepository_FindActiveConsent(t *testing.T) {
	ctx := context.Background()
	db := testutil.NewTestDB(t)
	repo := NewConsentRepository(db.PgxPool())

	consumerID := uuid.New()
	businessID := uuid.New()

	t.Run("find active consent for channel", func(t *testing.T) {
		// Create and activate consent
		consentAgg, _ := consent.NewConsentAggregate(
			consumerID, businessID,
			consent.TypeTCPA,
			[]consent.Channel{consent.ChannelVoice, consent.ChannelSMS},
			consent.PurposeMarketing,
			consent.SourceWebForm,
		)
		consentAgg.ActivateConsent([]consent.ConsentProof{{
			ID:              uuid.New(),
			Type:            consent.ProofTypeFormSubmission,
			StorageLocation: "s3://bucket/form.pdf",
			Hash:            "sha256:abcd1234",
		}}, nil)
		repo.Save(ctx, consentAgg)

		// Find active consent for voice channel
		found, err := repo.FindActiveConsent(ctx, consumerID, businessID, consent.ChannelVoice)
		require.NoError(t, err)
		assert.NotNil(t, found)
		assert.Equal(t, consentAgg.ID, found.ID)

		// Find active consent for SMS channel
		found, err = repo.FindActiveConsent(ctx, consumerID, businessID, consent.ChannelSMS)
		require.NoError(t, err)
		assert.NotNil(t, found)

		// Should not find for email channel
		found, err = repo.FindActiveConsent(ctx, consumerID, businessID, consent.ChannelEmail)
		require.NoError(t, err)
		assert.Nil(t, found)
	})

	t.Run("expired consent not returned", func(t *testing.T) {
		// Create consent that expires in the past
		consentAgg, _ := consent.NewConsentAggregate(
			uuid.New(), uuid.New(),
			consent.TypeTCPA,
			[]consent.Channel{consent.ChannelVoice},
			consent.PurposeMarketing,
			consent.SourceWebForm,
		)
		pastTime := time.Now().Add(-1 * time.Hour)
		consentAgg.ActivateConsent([]consent.ConsentProof{{
			ID:              uuid.New(),
			Type:            consent.ProofTypeFormSubmission,
			StorageLocation: "s3://bucket/form.pdf",
			Hash:            "sha256:abcd1234",
		}}, &pastTime)
		repo.Save(ctx, consentAgg)

		// Should not find expired consent
		found, err := repo.FindActiveConsent(ctx, consentAgg.ConsumerID, consentAgg.BusinessID, consent.ChannelVoice)
		require.NoError(t, err)
		assert.Nil(t, found)
	})

	t.Run("revoked consent not returned", func(t *testing.T) {
		// Create, activate, then revoke consent
		consentAgg, _ := consent.NewConsentAggregate(
			uuid.New(), uuid.New(),
			consent.TypeTCPA,
			[]consent.Channel{consent.ChannelVoice},
			consent.PurposeMarketing,
			consent.SourceWebForm,
		)
		consentAgg.ActivateConsent([]consent.ConsentProof{{
			ID:              uuid.New(),
			Type:            consent.ProofTypeFormSubmission,
			StorageLocation: "s3://bucket/form.pdf",
			Hash:            "sha256:abcd1234",
		}}, nil)
		consentAgg.RevokeConsent("Consumer request", uuid.New())
		repo.Save(ctx, consentAgg)

		// Should not find revoked consent
		found, err := repo.FindActiveConsent(ctx, consentAgg.ConsumerID, consentAgg.BusinessID, consent.ChannelVoice)
		require.NoError(t, err)
		assert.Nil(t, found)
	})
}

func TestConsentRepository_Delete(t *testing.T) {
	ctx := context.Background()
	db := testutil.NewTestDB(t)
	repo := NewConsentRepository(db.PgxPool())

	// Create and save consent
	consentAgg, _ := consent.NewConsentAggregate(
		uuid.New(), uuid.New(),
		consent.TypeTCPA,
		[]consent.Channel{consent.ChannelVoice},
		consent.PurposeMarketing,
		consent.SourceWebForm,
	)
	repo.Save(ctx, consentAgg)

	// Delete it
	err := repo.Delete(ctx, consentAgg.ID)
	require.NoError(t, err)

	// Should not be retrievable
	_, err = repo.GetByID(ctx, consentAgg.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	// Delete non-existent should error
	err = repo.Delete(ctx, uuid.New())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}