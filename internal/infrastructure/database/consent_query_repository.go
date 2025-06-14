package database

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lib/pq"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/consent"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
)

// ConsentQueryRepository implements the consent.QueryRepository interface
type ConsentQueryRepository struct {
	db *pgxpool.Pool
}

// NewConsentQueryRepository creates a new PostgreSQL consent query repository
func NewConsentQueryRepository(db *pgxpool.Pool) *ConsentQueryRepository {
	return &ConsentQueryRepository{db: db}
}

// Find searches for consents based on filter criteria
func (r *ConsentQueryRepository) Find(ctx context.Context, filter consent.ConsentFilter) ([]*consent.ConsentAggregate, error) {
	query, args := r.buildFilterQuery(filter)
	
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, errors.NewInternalError("failed to query consents").WithCause(err)
	}
	defer rows.Close()

	// Get the main repository to reuse GetByID
	mainRepo := NewConsentRepository(r.db)
	
	var consents []*consent.ConsentAggregate
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, errors.NewInternalError("failed to scan id").WithCause(err)
		}

		consentAgg, err := mainRepo.GetByID(ctx, id)
		if err != nil {
			return nil, err
		}
		consents = append(consents, consentAgg)
	}

	return consents, nil
}

// Count returns the number of consents matching the filter
func (r *ConsentQueryRepository) Count(ctx context.Context, filter consent.ConsentFilter) (int64, error) {
	// Build count query
	query, args := r.buildCountQuery(filter)
	
	var count int64
	err := r.db.QueryRow(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, errors.NewInternalError("failed to count consents").WithCause(err)
	}

	return count, nil
}

// GetConsentHistory retrieves all versions of a consent
func (r *ConsentQueryRepository) GetConsentHistory(ctx context.Context, consentID uuid.UUID) ([]consent.ConsentVersion, error) {
	rows, err := r.db.Query(ctx, `
		SELECT cv.id, cv.version_number, cv.status, cv.channels, cv.purpose, 
		       cv.source, cv.source_details, cv.consented_at, cv.expires_at, 
		       cv.revoked_at, cv.created_by, cv.created_at
		FROM consent_versions cv
		WHERE cv.consent_id = $1
		ORDER BY cv.version_number ASC
	`, consentID)
	if err != nil {
		return nil, errors.NewInternalError("failed to get consent history").WithCause(err)
	}
	defer rows.Close()

	var versions []consent.ConsentVersion
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

	return versions, nil
}

// GetProofs retrieves all proofs for a consent
func (r *ConsentQueryRepository) GetProofs(ctx context.Context, consentID uuid.UUID) ([]consent.ConsentProof, error) {
	rows, err := r.db.Query(ctx, `
		SELECT cp.id, cp.proof_type, cp.storage_location, cp.hash, cp.metadata
		FROM consent_proofs cp
		JOIN consent_versions cv ON cv.id = cp.consent_version_id
		WHERE cv.consent_id = $1
		ORDER BY cp.created_at ASC
	`, consentID)
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

// buildFilterQuery builds the SQL query for finding consents
func (r *ConsentQueryRepository) buildFilterQuery(filter consent.ConsentFilter) (string, []interface{}) {
	query := `
		SELECT DISTINCT ca.id
		FROM consent_aggregates ca
		JOIN consent_versions cv ON cv.consent_id = ca.id AND cv.version_number = ca.current_version
		JOIN consent_consumers cc ON cc.id = ca.consumer_id
		WHERE 1=1
	`
	
	var conditions []string
	var args []interface{}
	argCount := 1

	// Add filter conditions
	if filter.ConsumerID != nil {
		conditions = append(conditions, fmt.Sprintf("ca.consumer_id = $%d", argCount))
		args = append(args, *filter.ConsumerID)
		argCount++
	}

	if filter.BusinessID != nil {
		conditions = append(conditions, fmt.Sprintf("ca.business_id = $%d", argCount))
		args = append(args, *filter.BusinessID)
		argCount++
	}

	if filter.PhoneNumber != nil {
		conditions = append(conditions, fmt.Sprintf("cc.phone_number = $%d", argCount))
		args = append(args, *filter.PhoneNumber)
		argCount++
	}

	if filter.Email != nil {
		conditions = append(conditions, fmt.Sprintf("cc.email = $%d", argCount))
		args = append(args, *filter.Email)
		argCount++
	}

	if filter.Status != nil {
		conditions = append(conditions, fmt.Sprintf("cv.status = $%d", argCount))
		args = append(args, string(*filter.Status))
		argCount++
	}

	if len(filter.Channels) > 0 {
		channels := make([]string, len(filter.Channels))
		for i, ch := range filter.Channels {
			channels[i] = string(ch)
		}
		conditions = append(conditions, fmt.Sprintf("cv.channels && $%d", argCount))
		args = append(args, pq.Array(channels))
		argCount++
	}

	if filter.Purpose != nil {
		conditions = append(conditions, fmt.Sprintf("cv.purpose = $%d", argCount))
		args = append(args, string(*filter.Purpose))
		argCount++
	}

	if filter.CreatedAfter != nil {
		conditions = append(conditions, fmt.Sprintf("ca.created_at >= $%d", argCount))
		args = append(args, *filter.CreatedAfter)
		argCount++
	}

	if filter.CreatedBefore != nil {
		conditions = append(conditions, fmt.Sprintf("ca.created_at <= $%d", argCount))
		args = append(args, *filter.CreatedBefore)
		argCount++
	}

	if filter.ExpiringBefore != nil {
		conditions = append(conditions, fmt.Sprintf("cv.expires_at IS NOT NULL AND cv.expires_at <= $%d", argCount))
		args = append(args, *filter.ExpiringBefore)
		argCount++
	}

	// Add conditions to query
	if len(conditions) > 0 {
		query += " AND " + strings.Join(conditions, " AND ")
	}

	// Add ordering and pagination
	query += " ORDER BY ca.created_at DESC"
	
	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", filter.Limit)
	}
	
	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", filter.Offset)
	}

	return query, args
}

// buildCountQuery builds the SQL query for counting consents
func (r *ConsentQueryRepository) buildCountQuery(filter consent.ConsentFilter) (string, []interface{}) {
	query := `
		SELECT COUNT(DISTINCT ca.id)
		FROM consent_aggregates ca
		JOIN consent_versions cv ON cv.consent_id = ca.id AND cv.version_number = ca.current_version
		JOIN consent_consumers cc ON cc.id = ca.consumer_id
		WHERE 1=1
	`
	
	var conditions []string
	var args []interface{}
	argCount := 1

	// Add same filter conditions as buildFilterQuery (without limit/offset)
	if filter.ConsumerID != nil {
		conditions = append(conditions, fmt.Sprintf("ca.consumer_id = $%d", argCount))
		args = append(args, *filter.ConsumerID)
		argCount++
	}

	if filter.BusinessID != nil {
		conditions = append(conditions, fmt.Sprintf("ca.business_id = $%d", argCount))
		args = append(args, *filter.BusinessID)
		argCount++
	}

	if filter.PhoneNumber != nil {
		conditions = append(conditions, fmt.Sprintf("cc.phone_number = $%d", argCount))
		args = append(args, *filter.PhoneNumber)
		argCount++
	}

	if filter.Email != nil {
		conditions = append(conditions, fmt.Sprintf("cc.email = $%d", argCount))
		args = append(args, *filter.Email)
		argCount++
	}

	if filter.Status != nil {
		conditions = append(conditions, fmt.Sprintf("cv.status = $%d", argCount))
		args = append(args, string(*filter.Status))
		argCount++
	}

	if len(filter.Channels) > 0 {
		channels := make([]string, len(filter.Channels))
		for i, ch := range filter.Channels {
			channels[i] = string(ch)
		}
		conditions = append(conditions, fmt.Sprintf("cv.channels && $%d", argCount))
		args = append(args, pq.Array(channels))
		argCount++
	}

	if filter.Purpose != nil {
		conditions = append(conditions, fmt.Sprintf("cv.purpose = $%d", argCount))
		args = append(args, string(*filter.Purpose))
		argCount++
	}

	if filter.CreatedAfter != nil {
		conditions = append(conditions, fmt.Sprintf("ca.created_at >= $%d", argCount))
		args = append(args, *filter.CreatedAfter)
		argCount++
	}

	if filter.CreatedBefore != nil {
		conditions = append(conditions, fmt.Sprintf("ca.created_at <= $%d", argCount))
		args = append(args, *filter.CreatedBefore)
		argCount++
	}

	if filter.ExpiringBefore != nil {
		conditions = append(conditions, fmt.Sprintf("cv.expires_at IS NOT NULL AND cv.expires_at <= $%d", argCount))
		args = append(args, *filter.ExpiringBefore)
		argCount++
	}

	// Add conditions to query
	if len(conditions) > 0 {
		query += " AND " + strings.Join(conditions, " AND ")
	}

	return query, args
}

// getProofsForVersion is a helper method shared with main repository
func (r *ConsentQueryRepository) getProofsForVersion(ctx context.Context, versionID uuid.UUID) ([]consent.ConsentProof, error) {
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