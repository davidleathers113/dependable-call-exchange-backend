package database

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lib/pq"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/consent"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
)

// ConsentRepository implements the consent.Repository interface
type ConsentRepository struct {
	db *pgxpool.Pool
}

// NewConsentRepository creates a new PostgreSQL consent repository
func NewConsentRepository(db *pgxpool.Pool) *ConsentRepository {
	return &ConsentRepository{db: db}
}

// Save creates or updates a consent aggregate
func (r *ConsentRepository) Save(ctx context.Context, consentAgg *consent.ConsentAggregate) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return errors.NewInternalError("failed to begin transaction").WithCause(err)
	}
	defer tx.Rollback(ctx)

	// Check if aggregate exists
	var exists bool
	err = tx.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM consent_aggregates WHERE id = $1)
	`, consentAgg.ID).Scan(&exists)
	if err != nil {
		return errors.NewInternalError("failed to check aggregate existence").WithCause(err)
	}

	if exists {
		// Update existing aggregate
		_, err = tx.Exec(ctx, `
			UPDATE consent_aggregates 
			SET current_version = $2, updated_at = NOW()
			WHERE id = $1
		`, consentAgg.ID, consentAgg.CurrentVersion)
		if err != nil {
			return errors.NewInternalError("failed to update aggregate").WithCause(err)
		}
	} else {
		// Insert new aggregate
		_, err = tx.Exec(ctx, `
			INSERT INTO consent_aggregates (id, consumer_id, business_id, current_version, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, consentAgg.ID, consentAgg.ConsumerID, consentAgg.BusinessID, 
		   consentAgg.CurrentVersion, consentAgg.CreatedAt, consentAgg.UpdatedAt)
		if err != nil {
			return errors.NewInternalError("failed to insert aggregate").WithCause(err)
		}
	}

	// Save new versions
	for _, version := range consentAgg.Versions {
		// Check if version already exists
		var versionExists bool
		err = tx.QueryRow(ctx, `
			SELECT EXISTS(SELECT 1 FROM consent_versions WHERE consent_id = $1 AND version_number = $2)
		`, consentAgg.ID, version.Version).Scan(&versionExists)
		if err != nil {
			return errors.NewInternalError("failed to check version existence").WithCause(err)
		}

		if !versionExists {
			// Convert channels to string array
			channels := make([]string, len(version.Channels))
			for j, ch := range version.Channels {
				channels[j] = string(ch)
			}

			// Marshal source details
			sourceDetails, err := json.Marshal(version.SourceDetails)
			if err != nil {
				return errors.NewInternalError("failed to marshal source details").WithCause(err)
			}

			// Insert version
			var versionID uuid.UUID
			err = tx.QueryRow(ctx, `
				INSERT INTO consent_versions (
					id, consent_id, version_number, status, channels, purpose, 
					source, source_details, consented_at, expires_at, revoked_at,
					created_by, created_at
				) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
				RETURNING id
			`, uuid.New(), consentAgg.ID, version.Version, string(version.Status),
			   pq.Array(channels), string(version.Purpose), string(version.Source),
			   sourceDetails, version.ConsentedAt, version.ExpiresAt, version.RevokedAt,
			   version.CreatedBy, version.CreatedAt).Scan(&versionID)
			if err != nil {
				return errors.NewInternalError("failed to insert version").WithCause(err)
			}

			// Save proofs for this version
			for _, proof := range version.Proofs {
				metadata, err := json.Marshal(proof.Metadata)
				if err != nil {
					return errors.NewInternalError("failed to marshal proof metadata").WithCause(err)
				}

				_, err = tx.Exec(ctx, `
					INSERT INTO consent_proofs (
						id, consent_version_id, proof_type, storage_location, 
						hash, algorithm, metadata, created_at
					) VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
				`, proof.ID, versionID, string(proof.Type), proof.StorageLocation,
				   proof.Hash, "SHA256", metadata)
				if err != nil {
					return errors.NewInternalError("failed to insert proof").WithCause(err)
				}
			}
		}
	}

	// Save events
	events := consentAgg.GetEvents()
	for _, event := range events {
		eventData, err := json.Marshal(event)
		if err != nil {
			return errors.NewInternalError("failed to marshal event").WithCause(err)
		}

		eventType := fmt.Sprintf("%T", event)
		_, err = tx.Exec(ctx, `
			INSERT INTO consent_events (aggregate_id, event_type, event_data, version, occurred_at)
			VALUES ($1, $2, $3, $4, NOW())
		`, consentAgg.ID, eventType, eventData, consentAgg.CurrentVersion)
		if err != nil {
			return errors.NewInternalError("failed to insert event").WithCause(err)
		}
	}

	// Clear events after saving
	consentAgg.ClearEvents()

	return tx.Commit(ctx)
}

// GetByID retrieves a consent by its ID
func (r *ConsentRepository) GetByID(ctx context.Context, id uuid.UUID) (*consent.ConsentAggregate, error) {
	// Get aggregate
	var agg consent.ConsentAggregate
	err := r.db.QueryRow(ctx, `
		SELECT id, consumer_id, business_id, current_version, created_at, updated_at
		FROM consent_aggregates
		WHERE id = $1
	`, id).Scan(&agg.ID, &agg.ConsumerID, &agg.BusinessID, 
	            &agg.CurrentVersion, &agg.CreatedAt, &agg.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errors.NewNotFoundError("consent not found")
		}
		return nil, errors.NewInternalError("failed to get consent").WithCause(err)
	}

	// Get versions
	rows, err := r.db.Query(ctx, `
		SELECT cv.id, cv.version_number, cv.status, cv.channels, cv.purpose, 
		       cv.source, cv.source_details, cv.consented_at, cv.expires_at, 
		       cv.revoked_at, cv.created_by, cv.created_at
		FROM consent_versions cv
		WHERE cv.consent_id = $1
		ORDER BY cv.version_number ASC
	`, id)
	if err != nil {
		return nil, errors.NewInternalError("failed to get versions").WithCause(err)
	}
	defer rows.Close()

	versions := []consent.ConsentVersion{}
	for rows.Next() {
		var v consent.ConsentVersion
		var versionID uuid.UUID
		var channels pq.StringArray
		var sourceDetails json.RawMessage
		
		err := rows.Scan(&versionID, &v.Version, &v.Status, &channels,
		                &v.Purpose, &v.Source, &sourceDetails, &v.ConsentedAt,
		                &v.ExpiresAt, &v.RevokedAt, &v.CreatedBy, &v.CreatedAt)
		if err != nil {
			return nil, errors.NewInternalError("failed to scan version").WithCause(err)
		}

		// Convert channels
		v.Channels = make([]consent.Channel, len(channels))
		for i, ch := range channels {
			v.Channels[i] = consent.Channel(ch)
		}

		// Unmarshal source details
		if sourceDetails != nil {
			err = json.Unmarshal(sourceDetails, &v.SourceDetails)
			if err != nil {
				return nil, errors.NewInternalError("failed to unmarshal source details").WithCause(err)
			}
		}

		// Get proofs for this version
		proofs, err := r.getProofsForVersion(ctx, versionID)
		if err != nil {
			return nil, err
		}
		v.Proofs = proofs

		versions = append(versions, v)
	}

	if err = rows.Err(); err != nil {
		return nil, errors.NewInternalError("error iterating versions").WithCause(err)
	}

	agg.Versions = versions
	return &agg, nil
}

// GetByConsumerAndBusiness retrieves consents for a consumer-business pair
func (r *ConsentRepository) GetByConsumerAndBusiness(ctx context.Context, consumerID, businessID uuid.UUID) ([]*consent.ConsentAggregate, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id FROM consent_aggregates
		WHERE consumer_id = $1 AND business_id = $2
		ORDER BY created_at DESC
	`, consumerID, businessID)
	if err != nil {
		return nil, errors.NewInternalError("failed to query consents").WithCause(err)
	}
	defer rows.Close()

	var consents []*consent.ConsentAggregate
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, errors.NewInternalError("failed to scan id").WithCause(err)
		}

		consentAgg, err := r.GetByID(ctx, id)
		if err != nil {
			return nil, err
		}
		consents = append(consents, consentAgg)
	}

	return consents, nil
}

// FindActiveConsent finds active consent for a specific channel
func (r *ConsentRepository) FindActiveConsent(ctx context.Context, consumerID, businessID uuid.UUID, channel consent.Channel) (*consent.ConsentAggregate, error) {
	var id uuid.UUID
	err := r.db.QueryRow(ctx, `
		SELECT ca.id
		FROM consent_aggregates ca
		JOIN consent_versions cv ON cv.consent_id = ca.id AND cv.version_number = ca.current_version
		WHERE ca.consumer_id = $1 
		AND ca.business_id = $2
		AND cv.status = 'active'
		AND $3 = ANY(cv.channels)
		AND (cv.expires_at IS NULL OR cv.expires_at > NOW())
		LIMIT 1
	`, consumerID, businessID, string(channel)).Scan(&id)
	
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // No active consent found
		}
		return nil, errors.NewInternalError("failed to find active consent").WithCause(err)
	}

	return r.GetByID(ctx, id)
}

// FindByPhoneNumber finds consents by phone number
func (r *ConsentRepository) FindByPhoneNumber(ctx context.Context, phoneNumber string, businessID uuid.UUID) ([]*consent.ConsentAggregate, error) {
	rows, err := r.db.Query(ctx, `
		SELECT ca.id
		FROM consent_aggregates ca
		JOIN consent_consumers cc ON cc.id = ca.consumer_id
		WHERE cc.phone_number = $1 AND ca.business_id = $2
		ORDER BY ca.created_at DESC
	`, phoneNumber, businessID)
	if err != nil {
		return nil, errors.NewInternalError("failed to query by phone").WithCause(err)
	}
	defer rows.Close()

	var consents []*consent.ConsentAggregate
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, errors.NewInternalError("failed to scan id").WithCause(err)
		}

		consentAgg, err := r.GetByID(ctx, id)
		if err != nil {
			return nil, err
		}
		consents = append(consents, consentAgg)
	}

	return consents, nil
}

// ListExpired lists consents that have expired before a given time
func (r *ConsentRepository) ListExpired(ctx context.Context, before time.Time) ([]*consent.ConsentAggregate, error) {
	rows, err := r.db.Query(ctx, `
		SELECT DISTINCT ca.id
		FROM consent_aggregates ca
		JOIN consent_versions cv ON cv.consent_id = ca.id AND cv.version_number = ca.current_version
		WHERE cv.status = 'active' 
		AND cv.expires_at IS NOT NULL 
		AND cv.expires_at < $1
		ORDER BY ca.id
	`, before)
	if err != nil {
		return nil, errors.NewInternalError("failed to query expired consents").WithCause(err)
	}
	defer rows.Close()

	var consents []*consent.ConsentAggregate
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, errors.NewInternalError("failed to scan id").WithCause(err)
		}

		consentAgg, err := r.GetByID(ctx, id)
		if err != nil {
			return nil, err
		}
		consents = append(consents, consentAgg)
	}

	return consents, nil
}

// Delete removes a consent aggregate (for GDPR compliance)
func (r *ConsentRepository) Delete(ctx context.Context, id uuid.UUID) error {
	result, err := r.db.Exec(ctx, `
		DELETE FROM consent_aggregates WHERE id = $1
	`, id)
	if err != nil {
		return errors.NewInternalError("failed to delete consent").WithCause(err)
	}

	if result.RowsAffected() == 0 {
		return errors.NewNotFoundError("consent not found")
	}

	return nil
}

// getProofsForVersion retrieves proofs for a specific version
func (r *ConsentRepository) getProofsForVersion(ctx context.Context, versionID uuid.UUID) ([]consent.ConsentProof, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, proof_type, storage_location, hash, metadata
		FROM consent_proofs
		WHERE consent_version_id = $1
		ORDER BY created_at ASC
	`, versionID)
	if err != nil {
		return nil, errors.NewInternalError("failed to get proofs").WithCause(err)
	}
	defer rows.Close()

	var proofs []consent.ConsentProof
	for rows.Next() {
		var p consent.ConsentProof
		var metadata json.RawMessage
		
		err := rows.Scan(&p.ID, &p.Type, &p.StorageLocation, &p.Hash, &metadata)
		if err != nil {
			return nil, errors.NewInternalError("failed to scan proof").WithCause(err)
		}

		if metadata != nil {
			err = json.Unmarshal(metadata, &p.Metadata)
			if err != nil {
				return nil, errors.NewInternalError("failed to unmarshal metadata").WithCause(err)
			}
		}

		proofs = append(proofs, p)
	}

	return proofs, nil
}