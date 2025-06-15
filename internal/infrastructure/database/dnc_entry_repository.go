package database

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/dnc"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/errors"
	"github.com/davidleathers/dependable-call-exchange-backend/internal/domain/values"
)

// DNCEntryRepository implements the dnc.DNCEntryRepository interface using PostgreSQL
// Performance targets:
// - Save: < 10ms for single entry, < 100ms for batch operations
// - FindByPhone: < 5ms with proper indexing
// - BulkInsert: > 10K entries/second throughput
type DNCEntryRepository struct {
	db *pgxpool.Pool
}

// NewDNCEntryRepository creates a new PostgreSQL DNC entry repository
func NewDNCEntryRepository(db *pgxpool.Pool) *DNCEntryRepository {
	return &DNCEntryRepository{db: db}
}

// Save creates or updates a DNC entry
func (r *DNCEntryRepository) Save(ctx context.Context, entry *dnc.DNCEntry) error {
	query := `
		INSERT INTO dnc_entries (
			id, phone_number, list_source, suppress_reason, added_at, expires_at,
			source_reference, notes, metadata, added_by, updated_at, updated_by
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
		)
		ON CONFLICT (phone_number, list_source) 
		DO UPDATE SET
			suppress_reason = EXCLUDED.suppress_reason,
			expires_at = EXCLUDED.expires_at,
			source_reference = EXCLUDED.source_reference,
			notes = EXCLUDED.notes,
			metadata = EXCLUDED.metadata,
			updated_at = EXCLUDED.updated_at,
			updated_by = EXCLUDED.updated_by
	`

	metadataJSON, err := json.Marshal(entry.Metadata)
	if err != nil {
		return errors.NewInternalError("failed to marshal metadata").WithCause(err)
	}

	_, err = r.db.Exec(ctx, query,
		entry.ID,
		entry.PhoneNumber.String(),
		string(entry.ListSource),
		string(entry.SuppressReason),
		entry.AddedAt,
		entry.ExpiresAt,
		entry.SourceReference,
		entry.Notes,
		metadataJSON,
		entry.AddedBy,
		entry.UpdatedAt,
		entry.UpdatedBy,
	)

	if err != nil {
		return errors.NewInternalError("failed to save DNC entry").WithCause(err)
	}

	return nil
}

// SaveWithTx saves a DNC entry within an existing transaction
func (r *DNCEntryRepository) SaveWithTx(ctx context.Context, tx dnc.Transaction, entry *dnc.DNCEntry) error {
	pgxTx := tx.(*PgxTransaction)
	
	query := `
		INSERT INTO dnc_entries (
			id, phone_number, list_source, suppress_reason, added_at, expires_at,
			source_reference, notes, metadata, added_by, updated_at, updated_by
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
		)
		ON CONFLICT (phone_number, list_source) 
		DO UPDATE SET
			suppress_reason = EXCLUDED.suppress_reason,
			expires_at = EXCLUDED.expires_at,
			source_reference = EXCLUDED.source_reference,
			notes = EXCLUDED.notes,
			metadata = EXCLUDED.metadata,
			updated_at = EXCLUDED.updated_at,
			updated_by = EXCLUDED.updated_by
	`

	metadataJSON, err := json.Marshal(entry.Metadata)
	if err != nil {
		return errors.NewInternalError("failed to marshal metadata").WithCause(err)
	}

	_, err = pgxTx.tx.Exec(ctx, query,
		entry.ID,
		entry.PhoneNumber.String(),
		string(entry.ListSource),
		string(entry.SuppressReason),
		entry.AddedAt,
		entry.ExpiresAt,
		entry.SourceReference,
		entry.Notes,
		metadataJSON,
		entry.AddedBy,
		entry.UpdatedAt,
		entry.UpdatedBy,
	)

	if err != nil {
		return errors.NewInternalError("failed to save DNC entry in transaction").WithCause(err)
	}

	return nil
}

// GetByID retrieves a DNC entry by its unique identifier
func (r *DNCEntryRepository) GetByID(ctx context.Context, id uuid.UUID) (*dnc.DNCEntry, error) {
	query := `
		SELECT id, phone_number, list_source, suppress_reason, added_at, expires_at,
			   source_reference, notes, metadata, added_by, updated_at, updated_by
		FROM dnc_entries
		WHERE id = $1 AND deleted_at IS NULL
	`

	row := r.db.QueryRow(ctx, query, id)
	return r.scanDNCEntry(row)
}

// Delete removes a DNC entry (soft delete for audit trail)
func (r *DNCEntryRepository) Delete(ctx context.Context, id uuid.UUID, deletedBy uuid.UUID) error {
	query := `
		UPDATE dnc_entries 
		SET deleted_at = NOW(), deleted_by = $2, updated_at = NOW(), updated_by = $2
		WHERE id = $1 AND deleted_at IS NULL
	`

	result, err := r.db.Exec(ctx, query, id, deletedBy)
	if err != nil {
		return errors.NewInternalError("failed to delete DNC entry").WithCause(err)
	}

	if result.RowsAffected() == 0 {
		return errors.NewNotFoundError("DNC entry not found")
	}

	return nil
}

// DeleteWithTx removes a DNC entry within an existing transaction
func (r *DNCEntryRepository) DeleteWithTx(ctx context.Context, tx dnc.Transaction, id uuid.UUID, deletedBy uuid.UUID) error {
	pgxTx := tx.(*PgxTransaction)
	
	query := `
		UPDATE dnc_entries 
		SET deleted_at = NOW(), deleted_by = $2, updated_at = NOW(), updated_by = $2
		WHERE id = $1 AND deleted_at IS NULL
	`

	result, err := pgxTx.tx.Exec(ctx, query, id, deletedBy)
	if err != nil {
		return errors.NewInternalError("failed to delete DNC entry in transaction").WithCause(err)
	}

	if result.RowsAffected() == 0 {
		return errors.NewNotFoundError("DNC entry not found")
	}

	return nil
}

// FindByPhone retrieves DNC entries for a specific phone number
// Performance target: < 5ms with B-tree indexing on phone_number
func (r *DNCEntryRepository) FindByPhone(ctx context.Context, phoneNumber values.PhoneNumber) ([]*dnc.DNCEntry, error) {
	query := `
		SELECT id, phone_number, list_source, suppress_reason, added_at, expires_at,
			   source_reference, notes, metadata, added_by, updated_at, updated_by
		FROM dnc_entries
		WHERE phone_number = $1 AND deleted_at IS NULL
		ORDER BY added_at DESC
	`

	rows, err := r.db.Query(ctx, query, phoneNumber.String())
	if err != nil {
		return nil, errors.NewInternalError("failed to query DNC entries by phone").WithCause(err)
	}
	defer rows.Close()

	return r.scanDNCEntries(rows)
}

// FindActiveByPhone retrieves only active (non-expired) DNC entries for a phone number
// This is the most frequently called method - optimized for sub-5ms performance
func (r *DNCEntryRepository) FindActiveByPhone(ctx context.Context, phoneNumber values.PhoneNumber) ([]*dnc.DNCEntry, error) {
	query := `
		SELECT id, phone_number, list_source, suppress_reason, added_at, expires_at,
			   source_reference, notes, metadata, added_by, updated_at, updated_by
		FROM dnc_entries
		WHERE phone_number = $1 
		  AND deleted_at IS NULL
		  AND (expires_at IS NULL OR expires_at > NOW())
		ORDER BY added_at DESC
	`

	rows, err := r.db.Query(ctx, query, phoneNumber.String())
	if err != nil {
		return nil, errors.NewInternalError("failed to query active DNC entries").WithCause(err)
	}
	defer rows.Close()

	return r.scanDNCEntries(rows)
}

// CheckPhoneExists performs a fast existence check without retrieving full records
// Used for bloom filter miss validation - must be sub-millisecond
func (r *DNCEntryRepository) CheckPhoneExists(ctx context.Context, phoneNumber values.PhoneNumber) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM dnc_entries 
			WHERE phone_number = $1 
			  AND deleted_at IS NULL
			  AND (expires_at IS NULL OR expires_at > NOW())
		)
	`

	var exists bool
	err := r.db.QueryRow(ctx, query, phoneNumber.String()).Scan(&exists)
	if err != nil {
		return false, errors.NewInternalError("failed to check phone existence").WithCause(err)
	}

	return exists, nil
}

// FindByProvider retrieves all DNC entries from a specific provider
func (r *DNCEntryRepository) FindByProvider(ctx context.Context, providerID uuid.UUID) ([]*dnc.DNCEntry, error) {
	query := `
		SELECT d.id, d.phone_number, d.list_source, d.suppress_reason, d.added_at, d.expires_at,
			   d.source_reference, d.notes, d.metadata, d.added_by, d.updated_at, d.updated_by
		FROM dnc_entries d
		JOIN dnc_providers p ON p.id = $1
		WHERE d.metadata->>'provider_id' = $1::text
		  AND d.deleted_at IS NULL
		ORDER BY d.added_at DESC
	`

	rows, err := r.db.Query(ctx, query, providerID)
	if err != nil {
		return nil, errors.NewInternalError("failed to query DNC entries by provider").WithCause(err)
	}
	defer rows.Close()

	return r.scanDNCEntries(rows)
}

// FindBySource retrieves DNC entries by list source type
func (r *DNCEntryRepository) FindBySource(ctx context.Context, source dnc.ListSource) ([]*dnc.DNCEntry, error) {
	query := `
		SELECT id, phone_number, list_source, suppress_reason, added_at, expires_at,
			   source_reference, notes, metadata, added_by, updated_at, updated_by
		FROM dnc_entries
		WHERE list_source = $1 AND deleted_at IS NULL
		ORDER BY added_at DESC
	`

	rows, err := r.db.Query(ctx, query, string(source))
	if err != nil {
		return nil, errors.NewInternalError("failed to query DNC entries by source").WithCause(err)
	}
	defer rows.Close()

	return r.scanDNCEntries(rows)
}

// FindBySourceAndProvider retrieves entries by both source and provider
func (r *DNCEntryRepository) FindBySourceAndProvider(ctx context.Context, source dnc.ListSource, providerID uuid.UUID) ([]*dnc.DNCEntry, error) {
	query := `
		SELECT id, phone_number, list_source, suppress_reason, added_at, expires_at,
			   source_reference, notes, metadata, added_by, updated_at, updated_by
		FROM dnc_entries
		WHERE list_source = $1 
		  AND metadata->>'provider_id' = $2::text
		  AND deleted_at IS NULL
		ORDER BY added_at DESC
	`

	rows, err := r.db.Query(ctx, query, string(source), providerID)
	if err != nil {
		return nil, errors.NewInternalError("failed to query DNC entries by source and provider").WithCause(err)
	}
	defer rows.Close()

	return r.scanDNCEntries(rows)
}

// BulkInsert inserts multiple DNC entries in a single transaction
// Expected throughput: > 10K entries/second
func (r *DNCEntryRepository) BulkInsert(ctx context.Context, entries []*dnc.DNCEntry) error {
	if len(entries) == 0 {
		return nil
	}

	tx, err := r.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	err = r.BulkInsertWithTx(ctx, tx, entries)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// BulkInsertWithTx performs bulk insert within an existing transaction
func (r *DNCEntryRepository) BulkInsertWithTx(ctx context.Context, tx dnc.Transaction, entries []*dnc.DNCEntry) error {
	if len(entries) == 0 {
		return nil
	}

	pgxTx := tx.(*PgxTransaction)

	// Use COPY for maximum performance
	copyQuery := `
		COPY dnc_entries (
			id, phone_number, list_source, suppress_reason, added_at, expires_at,
			source_reference, notes, metadata, added_by, updated_at, updated_by
		) FROM STDIN
	`

	_, err := pgxTx.tx.CopyFrom(
		ctx,
		pgx.Identifier{"dnc_entries"},
		[]string{
			"id", "phone_number", "list_source", "suppress_reason", "added_at", "expires_at",
			"source_reference", "notes", "metadata", "added_by", "updated_at", "updated_by",
		},
		pgx.CopyFromSlice(len(entries), func(i int) ([]interface{}, error) {
			entry := entries[i]
			metadataJSON, err := json.Marshal(entry.Metadata)
			if err != nil {
				return nil, err
			}

			return []interface{}{
				entry.ID,
				entry.PhoneNumber.String(),
				string(entry.ListSource),
				string(entry.SuppressReason),
				entry.AddedAt,
				entry.ExpiresAt,
				entry.SourceReference,
				entry.Notes,
				metadataJSON,
				entry.AddedBy,
				entry.UpdatedAt,
				entry.UpdatedBy,
			}, nil
		}),
	)

	if err != nil {
		return errors.NewInternalError("failed to bulk insert DNC entries").WithCause(err)
	}

	return nil
}

// BulkUpdate updates multiple DNC entries efficiently
func (r *DNCEntryRepository) BulkUpdate(ctx context.Context, entries []*dnc.DNCEntry) error {
	if len(entries) == 0 {
		return nil
	}

	tx, err := r.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	pgxTx := tx.(*PgxTransaction)

	// Build batch update query
	query := `
		UPDATE dnc_entries 
		SET suppress_reason = data.suppress_reason,
		    expires_at = data.expires_at,
		    source_reference = data.source_reference,
		    notes = data.notes,
		    metadata = data.metadata,
		    updated_at = data.updated_at,
		    updated_by = data.updated_by
		FROM (VALUES %s) AS data(
			id, suppress_reason, expires_at, source_reference, notes, metadata, updated_at, updated_by
		)
		WHERE dnc_entries.id = data.id::uuid 
		  AND dnc_entries.deleted_at IS NULL
	`

	valueStrings := make([]string, len(entries))
	args := make([]interface{}, 0, len(entries)*8)

	for i, entry := range entries {
		metadataJSON, err := json.Marshal(entry.Metadata)
		if err != nil {
			return errors.NewInternalError("failed to marshal metadata").WithCause(err)
		}

		valueStrings[i] = fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d)",
			i*8+1, i*8+2, i*8+3, i*8+4, i*8+5, i*8+6, i*8+7, i*8+8)

		args = append(args,
			entry.ID,
			string(entry.SuppressReason),
			entry.ExpiresAt,
			entry.SourceReference,
			entry.Notes,
			metadataJSON,
			entry.UpdatedAt,
			entry.UpdatedBy,
		)
	}

	finalQuery := fmt.Sprintf(query, strings.Join(valueStrings, ","))
	_, err = pgxTx.tx.Exec(ctx, finalQuery, args...)
	if err != nil {
		return errors.NewInternalError("failed to bulk update DNC entries").WithCause(err)
	}

	return tx.Commit()
}

// BulkDelete removes multiple entries (soft delete)
func (r *DNCEntryRepository) BulkDelete(ctx context.Context, entryIDs []uuid.UUID, deletedBy uuid.UUID) error {
	if len(entryIDs) == 0 {
		return nil
	}

	query := `
		UPDATE dnc_entries 
		SET deleted_at = NOW(), deleted_by = $1, updated_at = NOW(), updated_by = $1
		WHERE id = ANY($2) AND deleted_at IS NULL
	`

	_, err := r.db.Exec(ctx, query, deletedBy, entryIDs)
	if err != nil {
		return errors.NewInternalError("failed to bulk delete DNC entries").WithCause(err)
	}

	return nil
}

// Upsert creates or updates entries based on phone number uniqueness
func (r *DNCEntryRepository) Upsert(ctx context.Context, entries []*dnc.DNCEntry) error {
	if len(entries) == 0 {
		return nil
	}

	tx, err := r.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	pgxTx := tx.(*PgxTransaction)

	// Use ON CONFLICT for upsert behavior
	for _, entry := range entries {
		query := `
			INSERT INTO dnc_entries (
				id, phone_number, list_source, suppress_reason, added_at, expires_at,
				source_reference, notes, metadata, added_by, updated_at, updated_by
			) VALUES (
				$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
			)
			ON CONFLICT (phone_number, list_source) 
			DO UPDATE SET
				suppress_reason = EXCLUDED.suppress_reason,
				expires_at = EXCLUDED.expires_at,
				source_reference = EXCLUDED.source_reference,
				notes = EXCLUDED.notes,
				metadata = EXCLUDED.metadata,
				updated_at = EXCLUDED.updated_at,
				updated_by = EXCLUDED.updated_by
		`

		metadataJSON, err := json.Marshal(entry.Metadata)
		if err != nil {
			return errors.NewInternalError("failed to marshal metadata").WithCause(err)
		}

		_, err = pgxTx.tx.Exec(ctx, query,
			entry.ID,
			entry.PhoneNumber.String(),
			string(entry.ListSource),
			string(entry.SuppressReason),
			entry.AddedAt,
			entry.ExpiresAt,
			entry.SourceReference,
			entry.Notes,
			metadataJSON,
			entry.AddedBy,
			entry.UpdatedAt,
			entry.UpdatedBy,
		)

		if err != nil {
			return errors.NewInternalError("failed to upsert DNC entry").WithCause(err)
		}
	}

	return tx.Commit()
}

// Find searches for DNC entries based on filter criteria with pagination
func (r *DNCEntryRepository) Find(ctx context.Context, filter dnc.DNCEntryFilter) (*dnc.DNCEntryPage, error) {
	startTime := time.Now()
	
	whereClause, args, err := r.buildWhereClause(filter)
	if err != nil {
		return nil, err
	}

	// Count query
	countQuery := fmt.Sprintf(`
		SELECT COUNT(*) 
		FROM dnc_entries 
		WHERE deleted_at IS NULL %s
	`, whereClause)

	var totalCount int64
	err = r.db.QueryRow(ctx, countQuery, args...).Scan(&totalCount)
	if err != nil {
		return nil, errors.NewInternalError("failed to count DNC entries").WithCause(err)
	}

	// Data query
	orderBy := "added_at"
	if filter.OrderBy != "" {
		orderBy = filter.OrderBy
	}
	
	direction := "DESC"
	if !filter.OrderDesc {
		direction = "ASC"
	}

	limit := 50
	if filter.Limit > 0 {
		limit = filter.Limit
	}

	offset := 0
	if filter.Offset > 0 {
		offset = filter.Offset
	}

	selectFields := `id, phone_number, list_source, suppress_reason, added_at, expires_at,
					 source_reference, notes, metadata, added_by, updated_at, updated_by`
	if !filter.IncludeMetadata {
		selectFields = `id, phone_number, list_source, suppress_reason, added_at, expires_at,
						source_reference, notes, '{}' as metadata, added_by, updated_at, updated_by`
	}

	dataQuery := fmt.Sprintf(`
		SELECT %s
		FROM dnc_entries 
		WHERE deleted_at IS NULL %s
		ORDER BY %s %s
		LIMIT $%d OFFSET $%d
	`, selectFields, whereClause, orderBy, direction, len(args)+1, len(args)+2)

	args = append(args, limit, offset)

	rows, err := r.db.Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, errors.NewInternalError("failed to query DNC entries").WithCause(err)
	}
	defer rows.Close()

	entries, err := r.scanDNCEntries(rows)
	if err != nil {
		return nil, err
	}

	queryTime := time.Since(startTime)
	hasMore := totalCount > int64(offset+len(entries))

	return &dnc.DNCEntryPage{
		Entries:    entries,
		TotalCount: totalCount,
		HasMore:    hasMore,
		QueryTime:  queryTime,
	}, nil
}

// Count returns the total number of entries matching the filter
func (r *DNCEntryRepository) Count(ctx context.Context, filter dnc.DNCEntryFilter) (int64, error) {
	whereClause, args, err := r.buildWhereClause(filter)
	if err != nil {
		return 0, err
	}

	query := fmt.Sprintf(`
		SELECT COUNT(*) 
		FROM dnc_entries 
		WHERE deleted_at IS NULL %s
	`, whereClause)

	var count int64
	err = r.db.QueryRow(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, errors.NewInternalError("failed to count DNC entries").WithCause(err)
	}

	return count, nil
}

// FindExpired retrieves entries that have expired before the given time
func (r *DNCEntryRepository) FindExpired(ctx context.Context, before time.Time, limit int) ([]*dnc.DNCEntry, error) {
	query := `
		SELECT id, phone_number, list_source, suppress_reason, added_at, expires_at,
			   source_reference, notes, metadata, added_by, updated_at, updated_by
		FROM dnc_entries
		WHERE deleted_at IS NULL 
		  AND expires_at IS NOT NULL 
		  AND expires_at < $1
		ORDER BY expires_at ASC
		LIMIT $2
	`

	rows, err := r.db.Query(ctx, query, before, limit)
	if err != nil {
		return nil, errors.NewInternalError("failed to query expired DNC entries").WithCause(err)
	}
	defer rows.Close()

	return r.scanDNCEntries(rows)
}

// FindExpiring retrieves entries expiring within the specified duration
func (r *DNCEntryRepository) FindExpiring(ctx context.Context, within time.Duration, limit int) ([]*dnc.DNCEntry, error) {
	expiryThreshold := time.Now().Add(within)

	query := `
		SELECT id, phone_number, list_source, suppress_reason, added_at, expires_at,
			   source_reference, notes, metadata, added_by, updated_at, updated_by
		FROM dnc_entries
		WHERE deleted_at IS NULL 
		  AND expires_at IS NOT NULL 
		  AND expires_at BETWEEN NOW() AND $1
		ORDER BY expires_at ASC
		LIMIT $2
	`

	rows, err := r.db.Query(ctx, query, expiryThreshold, limit)
	if err != nil {
		return nil, errors.NewInternalError("failed to query expiring DNC entries").WithCause(err)
	}
	defer rows.Close()

	return r.scanDNCEntries(rows)
}

// FindModifiedSince retrieves entries modified since a specific time
func (r *DNCEntryRepository) FindModifiedSince(ctx context.Context, since time.Time) ([]*dnc.DNCEntry, error) {
	query := `
		SELECT id, phone_number, list_source, suppress_reason, added_at, expires_at,
			   source_reference, notes, metadata, added_by, updated_at, updated_by
		FROM dnc_entries
		WHERE deleted_at IS NULL 
		  AND updated_at > $1
		ORDER BY updated_at ASC
	`

	rows, err := r.db.Query(ctx, query, since)
	if err != nil {
		return nil, errors.NewInternalError("failed to query modified DNC entries").WithCause(err)
	}
	defer rows.Close()

	return r.scanDNCEntries(rows)
}

// CleanupExpired removes expired entries older than the retention period
func (r *DNCEntryRepository) CleanupExpired(ctx context.Context, retentionDays int) (int64, error) {
	retentionDate := time.Now().AddDate(0, 0, -retentionDays)

	query := `
		DELETE FROM dnc_entries
		WHERE expires_at IS NOT NULL 
		  AND expires_at < $1
		  AND deleted_at IS NULL
	`

	result, err := r.db.Exec(ctx, query, retentionDate)
	if err != nil {
		return 0, errors.NewInternalError("failed to cleanup expired DNC entries").WithCause(err)
	}

	return result.RowsAffected(), nil
}

// Vacuum performs database maintenance operations
func (r *DNCEntryRepository) Vacuum(ctx context.Context) error {
	// PostgreSQL VACUUM cannot be run in a transaction
	queries := []string{
		"VACUUM ANALYZE dnc_entries",
		"REINDEX INDEX idx_dnc_entries_phone_number",
		"REINDEX INDEX idx_dnc_entries_list_source",
	}

	for _, query := range queries {
		_, err := r.db.Exec(ctx, query)
		if err != nil {
			return errors.NewInternalError("failed to vacuum DNC entries").WithCause(err)
		}
	}

	return nil
}

// GetStats returns repository performance and usage statistics
func (r *DNCEntryRepository) GetStats(ctx context.Context) (*dnc.DNCEntryStats, error) {
	stats := &dnc.DNCEntryStats{
		CollectedAt: time.Now(),
	}

	// Total entries
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM dnc_entries WHERE deleted_at IS NULL
	`).Scan(&stats.TotalEntries)
	if err != nil {
		return nil, errors.NewInternalError("failed to get total entries count").WithCause(err)
	}

	// Active vs expired entries
	err = r.db.QueryRow(ctx, `
		SELECT 
			COUNT(CASE WHEN expires_at IS NULL OR expires_at > NOW() THEN 1 END) as active,
			COUNT(CASE WHEN expires_at IS NOT NULL AND expires_at <= NOW() THEN 1 END) as expired
		FROM dnc_entries WHERE deleted_at IS NULL
	`).Scan(&stats.ActiveEntries, &stats.ExpiredEntries)
	if err != nil {
		return nil, errors.NewInternalError("failed to get active/expired counts").WithCause(err)
	}

	// Entries by source
	stats.EntriesBySource = make(map[string]int64)
	rows, err := r.db.Query(ctx, `
		SELECT list_source, COUNT(*) 
		FROM dnc_entries 
		WHERE deleted_at IS NULL 
		GROUP BY list_source
	`)
	if err != nil {
		return nil, errors.NewInternalError("failed to get entries by source").WithCause(err)
	}
	defer rows.Close()

	for rows.Next() {
		var source string
		var count int64
		if err := rows.Scan(&source, &count); err != nil {
			return nil, errors.NewInternalError("failed to scan source stats").WithCause(err)
		}
		stats.EntriesBySource[source] = count
	}

	// Recent entries (time-based stats)
	timeStats := []struct {
		query  string
		target *int64
	}{
		{`SELECT COUNT(*) FROM dnc_entries WHERE deleted_at IS NULL AND added_at >= NOW() - INTERVAL '1 day'`, &stats.EntriesThisDay},
		{`SELECT COUNT(*) FROM dnc_entries WHERE deleted_at IS NULL AND added_at >= NOW() - INTERVAL '1 week'`, &stats.EntriesThisWeek},
		{`SELECT COUNT(*) FROM dnc_entries WHERE deleted_at IS NULL AND added_at >= NOW() - INTERVAL '1 month'`, &stats.EntriesThisMonth},
	}

	for _, stat := range timeStats {
		err = r.db.QueryRow(ctx, stat.query).Scan(stat.target)
		if err != nil {
			return nil, errors.NewInternalError("failed to get time-based stats").WithCause(err)
		}
	}

	return stats, nil
}

// GetSyncChecksum calculates a checksum for entries from a specific provider
func (r *DNCEntryRepository) GetSyncChecksum(ctx context.Context, providerID uuid.UUID) (string, error) {
	query := `
		SELECT STRING_AGG(
			CONCAT(phone_number, '|', list_source, '|', suppress_reason, '|', 
				   COALESCE(expires_at::text, ''), '|', updated_at::text),
			'' ORDER BY phone_number, list_source
		)
		FROM dnc_entries
		WHERE metadata->>'provider_id' = $1::text
		  AND deleted_at IS NULL
	`

	var concatenated sql.NullString
	err := r.db.QueryRow(ctx, query, providerID).Scan(&concatenated)
	if err != nil {
		return "", errors.NewInternalError("failed to calculate sync checksum").WithCause(err)
	}

	if !concatenated.Valid {
		return "", nil // No entries for this provider
	}

	// Calculate SHA256 hash
	hash := sha256.Sum256([]byte(concatenated.String))
	return hex.EncodeToString(hash[:]), nil
}

// GetLastSyncTime returns the timestamp of the most recent entry from a provider
func (r *DNCEntryRepository) GetLastSyncTime(ctx context.Context, providerID uuid.UUID) (*time.Time, error) {
	query := `
		SELECT MAX(updated_at)
		FROM dnc_entries
		WHERE metadata->>'provider_id' = $1::text
		  AND deleted_at IS NULL
	`

	var lastSyncTime sql.NullTime
	err := r.db.QueryRow(ctx, query, providerID).Scan(&lastSyncTime)
	if err != nil {
		return nil, errors.NewInternalError("failed to get last sync time").WithCause(err)
	}

	if !lastSyncTime.Valid {
		return nil, nil
	}

	return &lastSyncTime.Time, nil
}

// ValidateIntegrity performs integrity checks on the repository
func (r *DNCEntryRepository) ValidateIntegrity(ctx context.Context) (*dnc.DNCIntegrityReport, error) {
	report := &dnc.DNCIntegrityReport{
		GeneratedAt: time.Now(),
		IsHealthy:   true,
	}

	startTime := time.Now()

	// Check total entries
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM dnc_entries WHERE deleted_at IS NULL
	`).Scan(&report.TotalEntries)
	if err != nil {
		return nil, errors.NewInternalError("failed to count total entries").WithCause(err)
	}

	// Validate phone number formats
	err = r.db.QueryRow(ctx, `
		SELECT COUNT(*) 
		FROM dnc_entries 
		WHERE deleted_at IS NULL 
		  AND phone_number ~ '^\+[1-9]\d{1,14}$'
	`).Scan(&report.ValidEntries)
	if err != nil {
		return nil, errors.NewInternalError("failed to count valid entries").WithCause(err)
	}

	report.InvalidEntries = report.TotalEntries - report.ValidEntries

	// Check for duplicates
	err = r.db.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM (
			SELECT phone_number, list_source
			FROM dnc_entries
			WHERE deleted_at IS NULL
			GROUP BY phone_number, list_source
			HAVING COUNT(*) > 1
		) duplicates
	`).Scan(&report.DuplicateEntries)
	if err != nil {
		return nil, errors.NewInternalError("failed to count duplicates").WithCause(err)
	}

	// Check referential integrity
	var orphanedCount int64
	err = r.db.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM dnc_entries d
		LEFT JOIN dnc_providers p ON p.id::text = d.metadata->>'provider_id'
		WHERE d.deleted_at IS NULL
		  AND d.metadata->>'provider_id' IS NOT NULL
		  AND p.id IS NULL
	`).Scan(&orphanedCount)
	if err != nil {
		return nil, errors.NewInternalError("failed to count orphaned entries").WithCause(err)
	}
	report.OrphanedEntries = orphanedCount

	// Overall health assessment
	if report.InvalidEntries > 0 {
		report.IsHealthy = false
		report.CriticalIssues = append(report.CriticalIssues, 
			fmt.Sprintf("Found %d entries with invalid phone number format", report.InvalidEntries))
	}

	if report.DuplicateEntries > 0 {
		report.Warnings = append(report.Warnings, 
			fmt.Sprintf("Found %d duplicate entries", report.DuplicateEntries))
	}

	if report.OrphanedEntries > 0 {
		report.Warnings = append(report.Warnings, 
			fmt.Sprintf("Found %d orphaned entries with invalid provider references", report.OrphanedEntries))
	}

	// Set overall status
	if len(report.CriticalIssues) > 0 {
		report.OverallStatus = "CRITICAL"
		report.IsHealthy = false
	} else if len(report.Warnings) > 0 {
		report.OverallStatus = "DEGRADED"
	} else {
		report.OverallStatus = "HEALTHY"
	}

	report.VerificationTime = time.Since(startTime)

	return report, nil
}

// BeginTx starts a new database transaction
func (r *DNCEntryRepository) BeginTx(ctx context.Context) (dnc.Transaction, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, errors.NewInternalError("failed to begin transaction").WithCause(err)
	}

	return &PgxTransaction{tx: tx, ctx: ctx}, nil
}

// WithTx executes a function within a database transaction
func (r *DNCEntryRepository) WithTx(ctx context.Context, fn func(tx dnc.Transaction) error) error {
	tx, err := r.BeginTx(ctx)
	if err != nil {
		return err
	}

	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p)
		}
		if err != nil {
			tx.Rollback()
		}
	}()

	err = fn(tx)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// Helper methods

// scanDNCEntry scans a single DNC entry from a database row
func (r *DNCEntryRepository) scanDNCEntry(row pgx.Row) (*dnc.DNCEntry, error) {
	var entry dnc.DNCEntry
	var phoneNumber string
	var listSource string
	var suppressReason string
	var metadataJSON []byte

	err := row.Scan(
		&entry.ID,
		&phoneNumber,
		&listSource,
		&suppressReason,
		&entry.AddedAt,
		&entry.ExpiresAt,
		&entry.SourceReference,
		&entry.Notes,
		&metadataJSON,
		&entry.AddedBy,
		&entry.UpdatedAt,
		&entry.UpdatedBy,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errors.NewNotFoundError("DNC entry not found")
		}
		return nil, errors.NewInternalError("failed to scan DNC entry").WithCause(err)
	}

	// Convert string fields to value objects
	phone, err := values.NewPhoneNumber(phoneNumber)
	if err != nil {
		return nil, errors.NewValidationError("INVALID_PHONE_NUMBER", "invalid phone number in database").WithCause(err)
	}
	entry.PhoneNumber = phone

	source, err := values.NewListSource(listSource)
	if err != nil {
		return nil, errors.NewValidationError("INVALID_LIST_SOURCE", "invalid list source in database").WithCause(err)
	}
	entry.ListSource = source

	reason, err := values.NewSuppressReason(suppressReason)
	if err != nil {
		return nil, errors.NewValidationError("INVALID_SUPPRESS_REASON", "invalid suppress reason in database").WithCause(err)
	}
	entry.SuppressReason = reason

	// Unmarshal metadata
	if metadataJSON != nil {
		err = json.Unmarshal(metadataJSON, &entry.Metadata)
		if err != nil {
			return nil, errors.NewInternalError("failed to unmarshal metadata").WithCause(err)
		}
	}

	return &entry, nil
}

// scanDNCEntries scans multiple DNC entries from database rows
func (r *DNCEntryRepository) scanDNCEntries(rows pgx.Rows) ([]*dnc.DNCEntry, error) {
	var entries []*dnc.DNCEntry

	for rows.Next() {
		entry, err := r.scanDNCEntry(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}

	if err := rows.Err(); err != nil {
		return nil, errors.NewInternalError("error iterating DNC entries").WithCause(err)
	}

	return entries, nil
}

// buildWhereClause builds a WHERE clause and arguments from a filter
func (r *DNCEntryRepository) buildWhereClause(filter dnc.DNCEntryFilter) (string, []interface{}, error) {
	conditions := make([]string, 0)
	args := make([]interface{}, 0)
	argIndex := 1

	// Phone number filters
	if len(filter.PhoneNumbers) > 0 {
		phoneStrs := make([]string, len(filter.PhoneNumbers))
		for i, phone := range filter.PhoneNumbers {
			phoneStrs[i] = phone.String()
		}
		conditions = append(conditions, fmt.Sprintf("phone_number = ANY($%d)", argIndex))
		args = append(args, phoneStrs)
		argIndex++
	}

	if filter.PhonePattern != nil {
		conditions = append(conditions, fmt.Sprintf("phone_number LIKE $%d", argIndex))
		args = append(args, *filter.PhonePattern)
		argIndex++
	}

	// Source filters
	if len(filter.Sources) > 0 {
		sourceStrs := make([]string, len(filter.Sources))
		for i, source := range filter.Sources {
			sourceStrs[i] = string(source)
		}
		conditions = append(conditions, fmt.Sprintf("list_source = ANY($%d)", argIndex))
		args = append(args, sourceStrs)
		argIndex++
	}

	// Provider filters
	if len(filter.ProviderIDs) > 0 {
		providerStrs := make([]string, len(filter.ProviderIDs))
		for i, id := range filter.ProviderIDs {
			providerStrs[i] = id.String()
		}
		conditions = append(conditions, fmt.Sprintf("metadata->>'provider_id' = ANY($%d)", argIndex))
		args = append(args, providerStrs)
		argIndex++
	}

	// Reason filters
	if len(filter.SuppressReasons) > 0 {
		reasonStrs := make([]string, len(filter.SuppressReasons))
		for i, reason := range filter.SuppressReasons {
			reasonStrs[i] = string(reason)
		}
		conditions = append(conditions, fmt.Sprintf("suppress_reason = ANY($%d)", argIndex))
		args = append(args, reasonStrs)
		argIndex++
	}

	// Time range filters
	if filter.AddedAfter != nil {
		conditions = append(conditions, fmt.Sprintf("added_at > $%d", argIndex))
		args = append(args, *filter.AddedAfter)
		argIndex++
	}

	if filter.AddedBefore != nil {
		conditions = append(conditions, fmt.Sprintf("added_at < $%d", argIndex))
		args = append(args, *filter.AddedBefore)
		argIndex++
	}

	if filter.ExpiresAfter != nil {
		conditions = append(conditions, fmt.Sprintf("expires_at > $%d", argIndex))
		args = append(args, *filter.ExpiresAfter)
		argIndex++
	}

	if filter.ExpiresBefore != nil {
		conditions = append(conditions, fmt.Sprintf("expires_at < $%d", argIndex))
		args = append(args, *filter.ExpiresBefore)
		argIndex++
	}

	// Status filters
	if filter.OnlyActive != nil && *filter.OnlyActive {
		conditions = append(conditions, "(expires_at IS NULL OR expires_at > NOW())")
	}

	if filter.OnlyExpired != nil && *filter.OnlyExpired {
		conditions = append(conditions, "expires_at IS NOT NULL AND expires_at <= NOW()")
	}

	if filter.HasExpiry != nil {
		if *filter.HasExpiry {
			conditions = append(conditions, "expires_at IS NOT NULL")
		} else {
			conditions = append(conditions, "expires_at IS NULL")
		}
	}

	// User filters
	if len(filter.AddedBy) > 0 {
		conditions = append(conditions, fmt.Sprintf("added_by = ANY($%d)", argIndex))
		args = append(args, filter.AddedBy)
		argIndex++
	}

	if len(filter.UpdatedBy) > 0 {
		conditions = append(conditions, fmt.Sprintf("updated_by = ANY($%d)", argIndex))
		args = append(args, filter.UpdatedBy)
		argIndex++
	}

	// Search text
	if filter.SearchText != nil {
		conditions = append(conditions, fmt.Sprintf("(notes ILIKE $%d OR metadata::text ILIKE $%d)", argIndex, argIndex))
		searchPattern := "%" + *filter.SearchText + "%"
		args = append(args, searchPattern)
		argIndex++
	}

	// Metadata filters
	for key, value := range filter.MetadataValues {
		conditions = append(conditions, fmt.Sprintf("metadata->>'%s' = $%d", key, argIndex))
		args = append(args, value)
		argIndex++
	}

	// Build final WHERE clause
	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "AND " + strings.Join(conditions, " AND ")
	}

	return whereClause, args, nil
}

// PgxTransaction implements the dnc.Transaction interface using pgx
type PgxTransaction struct {
	tx  pgx.Tx
	ctx context.Context
}

// Commit commits the transaction
func (t *PgxTransaction) Commit() error {
	return t.tx.Commit(t.ctx)
}

// Rollback rolls back the transaction
func (t *PgxTransaction) Rollback() error {
	return t.tx.Rollback(t.ctx)
}

// Context returns the transaction context
func (t *PgxTransaction) Context() context.Context {
	return t.ctx
}