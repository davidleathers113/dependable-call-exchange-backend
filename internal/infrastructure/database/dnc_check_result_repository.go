package database

import (
	"context"
	"database/sql"
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

// DNCCheckResultRepository implements the dnc.DNCCheckResultRepository interface using PostgreSQL
// Performance targets:
// - Save: < 5ms for single result (hot path for call routing)
// - FindByPhone: < 2ms with proper caching integration
// - FindRecent: < 10ms for compliance audit trails
// - Cleanup: > 10K results/second for data retention
type DNCCheckResultRepository struct {
	db *pgxpool.Pool
}

// NewDNCCheckResultRepository creates a new PostgreSQL DNC check result repository
func NewDNCCheckResultRepository(db *pgxpool.Pool) *DNCCheckResultRepository {
	return &DNCCheckResultRepository{db: db}
}

// Save persists a DNC check result for caching and audit purposes
// This is called on every DNC check - optimized for sub-5ms latency
func (r *DNCCheckResultRepository) Save(ctx context.Context, result *dnc.DNCCheckResult) error {
	query := `
		INSERT INTO dnc_check_results (
			id, phone_number, is_blocked, checked_at, ttl, check_duration,
			sources_count, compliance_level, risk_score, reasons, sources, metadata
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
		)
		ON CONFLICT (phone_number, checked_at) 
		DO UPDATE SET
			is_blocked = EXCLUDED.is_blocked,
			ttl = EXCLUDED.ttl,
			check_duration = EXCLUDED.check_duration,
			sources_count = EXCLUDED.sources_count,
			compliance_level = EXCLUDED.compliance_level,
			risk_score = EXCLUDED.risk_score,
			reasons = EXCLUDED.reasons,
			sources = EXCLUDED.sources,
			metadata = EXCLUDED.metadata
	`

	reasonsJSON, err := json.Marshal(result.Reasons)
	if err != nil {
		return errors.NewInternalError("failed to marshal reasons").WithCause(err)
	}

	sourcesJSON, err := json.Marshal(result.Sources)
	if err != nil {
		return errors.NewInternalError("failed to marshal sources").WithCause(err)
	}

	metadataJSON, err := json.Marshal(result.Metadata)
	if err != nil {
		return errors.NewInternalError("failed to marshal metadata").WithCause(err)
	}

	_, err = r.db.Exec(ctx, query,
		result.ID,
		result.PhoneNumber.String(),
		result.IsBlocked,
		result.CheckedAt,
		result.TTL,
		result.CheckDuration,
		result.SourcesCount,
		result.ComplianceLevel,
		result.RiskScore,
		reasonsJSON,
		sourcesJSON,
		metadataJSON,
	)

	if err != nil {
		return errors.NewInternalError("failed to save DNC check result").WithCause(err)
	}

	return nil
}

// SaveWithTx saves a DNC check result within an existing transaction
func (r *DNCCheckResultRepository) SaveWithTx(ctx context.Context, tx dnc.Transaction, result *dnc.DNCCheckResult) error {
	pgxTx := tx.(*PgxTransaction)
	
	query := `
		INSERT INTO dnc_check_results (
			id, phone_number, is_blocked, checked_at, ttl, check_duration,
			sources_count, compliance_level, risk_score, reasons, sources, metadata
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
		)
		ON CONFLICT (phone_number, checked_at) 
		DO UPDATE SET
			is_blocked = EXCLUDED.is_blocked,
			ttl = EXCLUDED.ttl,
			check_duration = EXCLUDED.check_duration,
			sources_count = EXCLUDED.sources_count,
			compliance_level = EXCLUDED.compliance_level,
			risk_score = EXCLUDED.risk_score,
			reasons = EXCLUDED.reasons,
			sources = EXCLUDED.sources,
			metadata = EXCLUDED.metadata
	`

	reasonsJSON, err := json.Marshal(result.Reasons)
	if err != nil {
		return errors.NewInternalError("failed to marshal reasons").WithCause(err)
	}

	sourcesJSON, err := json.Marshal(result.Sources)
	if err != nil {
		return errors.NewInternalError("failed to marshal sources").WithCause(err)
	}

	metadataJSON, err := json.Marshal(result.Metadata)
	if err != nil {
		return errors.NewInternalError("failed to marshal metadata").WithCause(err)
	}

	_, err = pgxTx.tx.Exec(ctx, query,
		result.ID,
		result.PhoneNumber.String(),
		result.IsBlocked,
		result.CheckedAt,
		result.TTL,
		result.CheckDuration,
		result.SourcesCount,
		result.ComplianceLevel,
		result.RiskScore,
		reasonsJSON,
		sourcesJSON,
		metadataJSON,
	)

	if err != nil {
		return errors.NewInternalError("failed to save DNC check result in transaction").WithCause(err)
	}

	return nil
}

// GetByID retrieves a check result by its unique identifier
func (r *DNCCheckResultRepository) GetByID(ctx context.Context, id uuid.UUID) (*dnc.DNCCheckResult, error) {
	query := `
		SELECT id, phone_number, is_blocked, checked_at, ttl, check_duration,
			   sources_count, compliance_level, risk_score, reasons, sources, metadata
		FROM dnc_check_results
		WHERE id = $1
	`

	row := r.db.QueryRow(ctx, query, id)
	return r.scanDNCCheckResult(row)
}

// FindByPhone retrieves recent check results for a specific phone number
// Used for compliance auditing and debugging - expected latency < 2ms
func (r *DNCCheckResultRepository) FindByPhone(ctx context.Context, phoneNumber values.PhoneNumber) ([]*dnc.DNCCheckResult, error) {
	query := `
		SELECT id, phone_number, is_blocked, checked_at, ttl, check_duration,
			   sources_count, compliance_level, risk_score, reasons, sources, metadata
		FROM dnc_check_results
		WHERE phone_number = $1
		ORDER BY checked_at DESC
		LIMIT 100
	`

	rows, err := r.db.Query(ctx, query, phoneNumber.String())
	if err != nil {
		return nil, errors.NewInternalError("failed to query DNC check results by phone").WithCause(err)
	}
	defer rows.Close()

	return r.scanDNCCheckResults(rows)
}

// FindLatestByPhone retrieves the most recent check result for a phone number
func (r *DNCCheckResultRepository) FindLatestByPhone(ctx context.Context, phoneNumber values.PhoneNumber) (*dnc.DNCCheckResult, error) {
	query := `
		SELECT id, phone_number, is_blocked, checked_at, ttl, check_duration,
			   sources_count, compliance_level, risk_score, reasons, sources, metadata
		FROM dnc_check_results
		WHERE phone_number = $1
		ORDER BY checked_at DESC
		LIMIT 1
	`

	row := r.db.QueryRow(ctx, query, phoneNumber.String())
	result, err := r.scanDNCCheckResult(row)
	if err != nil {
		if errors.IsNotFoundError(err) {
			return nil, nil // No results found
		}
		return nil, err
	}

	return result, nil
}

// FindValidCachedResult retrieves a cached result that is still within TTL
// This is the primary cache lookup method - must be sub-millisecond
func (r *DNCCheckResultRepository) FindValidCachedResult(ctx context.Context, phoneNumber values.PhoneNumber) (*dnc.DNCCheckResult, error) {
	query := `
		SELECT id, phone_number, is_blocked, checked_at, ttl, check_duration,
			   sources_count, compliance_level, risk_score, reasons, sources, metadata
		FROM dnc_check_results
		WHERE phone_number = $1
		  AND checked_at + ttl > NOW()
		ORDER BY checked_at DESC
		LIMIT 1
	`

	row := r.db.QueryRow(ctx, query, phoneNumber.String())
	result, err := r.scanDNCCheckResult(row)
	if err != nil {
		if errors.IsNotFoundError(err) {
			return nil, nil // No valid cached result
		}
		return nil, err
	}

	return result, nil
}

// FindRecent retrieves check results within a specified time range
func (r *DNCCheckResultRepository) FindRecent(ctx context.Context, since time.Time, limit int) ([]*dnc.DNCCheckResult, error) {
	query := `
		SELECT id, phone_number, is_blocked, checked_at, ttl, check_duration,
			   sources_count, compliance_level, risk_score, reasons, sources, metadata
		FROM dnc_check_results
		WHERE checked_at >= $1
		ORDER BY checked_at DESC
		LIMIT $2
	`

	rows, err := r.db.Query(ctx, query, since, limit)
	if err != nil {
		return nil, errors.NewInternalError("failed to query recent DNC check results").WithCause(err)
	}
	defer rows.Close()

	return r.scanDNCCheckResults(rows)
}

// FindByTimeRange retrieves check results within a specific time window
func (r *DNCCheckResultRepository) FindByTimeRange(ctx context.Context, start, end time.Time, filter dnc.DNCCheckFilter) ([]*dnc.DNCCheckResult, error) {
	whereClause, args, err := r.buildCheckResultWhereClause(filter)
	if err != nil {
		return nil, err
	}

	// Add time range to existing conditions
	timeCondition := fmt.Sprintf("checked_at BETWEEN $%d AND $%d", len(args)+1, len(args)+2)
	args = append(args, start, end)

	if whereClause != "" {
		whereClause = whereClause + " AND " + timeCondition
	} else {
		whereClause = "WHERE " + timeCondition
	}

	limit := 1000
	if filter.Limit > 0 {
		limit = filter.Limit
	}

	orderBy := "checked_at DESC"
	if filter.OrderBy != "" {
		direction := "DESC"
		if !filter.OrderDesc {
			direction = "ASC"
		}
		orderBy = fmt.Sprintf("%s %s", filter.OrderBy, direction)
	}

	query := fmt.Sprintf(`
		SELECT id, phone_number, is_blocked, checked_at, ttl, check_duration,
			   sources_count, compliance_level, risk_score, reasons, sources, metadata
		FROM dnc_check_results
		%s
		ORDER BY %s
		LIMIT %d
	`, whereClause, orderBy, limit)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, errors.NewInternalError("failed to query DNC check results by time range").WithCause(err)
	}
	defer rows.Close()

	return r.scanDNCCheckResults(rows)
}

// FindBlockedCalls retrieves check results that resulted in blocked calls
func (r *DNCCheckResultRepository) FindBlockedCalls(ctx context.Context, timeRange dnc.TimeRange) ([]*dnc.DNCCheckResult, error) {
	query := `
		SELECT id, phone_number, is_blocked, checked_at, ttl, check_duration,
			   sources_count, compliance_level, risk_score, reasons, sources, metadata
		FROM dnc_check_results
		WHERE is_blocked = true
		  AND checked_at BETWEEN $1 AND $2
		ORDER BY checked_at DESC
	`

	rows, err := r.db.Query(ctx, query, timeRange.Start, timeRange.End)
	if err != nil {
		return nil, errors.NewInternalError("failed to query blocked calls").WithCause(err)
	}
	defer rows.Close()

	return r.scanDNCCheckResults(rows)
}

// FindByCompliance retrieves results filtered by compliance criteria
func (r *DNCCheckResultRepository) FindByCompliance(ctx context.Context, filter dnc.ComplianceFilter) ([]*dnc.DNCCheckResult, error) {
	conditions := make([]string, 0)
	args := make([]interface{}, 0)
	argIndex := 1

	// Time range is required for compliance queries
	conditions = append(conditions, fmt.Sprintf("checked_at BETWEEN $%d AND $%d", argIndex, argIndex+1))
	args = append(args, filter.TimeRange.Start, filter.TimeRange.End)
	argIndex += 2

	// Compliance codes
	if len(filter.ComplianceCodes) > 0 {
		conditions = append(conditions, fmt.Sprintf("compliance_level = ANY($%d)", argIndex))
		args = append(args, filter.ComplianceCodes)
		argIndex++
	}

	// Risk levels (map to risk score ranges)
	if len(filter.RiskLevels) > 0 {
		riskConditions := make([]string, 0)
		for _, level := range filter.RiskLevels {
			switch level {
			case "low":
				riskConditions = append(riskConditions, fmt.Sprintf("risk_score < $%d", argIndex))
				args = append(args, 0.3)
				argIndex++
			case "medium":
				riskConditions = append(riskConditions, fmt.Sprintf("risk_score BETWEEN $%d AND $%d", argIndex, argIndex+1))
				args = append(args, 0.3, 0.7)
				argIndex += 2
			case "high":
				riskConditions = append(riskConditions, fmt.Sprintf("risk_score > $%d", argIndex))
				args = append(args, 0.7)
				argIndex++
			}
		}
		if len(riskConditions) > 0 {
			conditions = append(conditions, "("+strings.Join(riskConditions, " OR ")+")")
		}
	}

	// Documentation requirement
	if filter.RequiresDocumentation != nil && *filter.RequiresDocumentation {
		conditions = append(conditions, "is_blocked = true")
	}

	// Business hours filter
	if filter.BusinessHours != nil && *filter.BusinessHours {
		conditions = append(conditions, "EXTRACT(hour FROM checked_at) BETWEEN 8 AND 17")
		conditions = append(conditions, "EXTRACT(dow FROM checked_at) BETWEEN 1 AND 5")
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	query := fmt.Sprintf(`
		SELECT id, phone_number, is_blocked, checked_at, ttl, check_duration,
			   sources_count, compliance_level, risk_score, reasons, sources, metadata
		FROM dnc_check_results
		%s
		ORDER BY checked_at DESC
		LIMIT 10000
	`, whereClause)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, errors.NewInternalError("failed to query compliance filtered results").WithCause(err)
	}
	defer rows.Close()

	return r.scanDNCCheckResults(rows)
}

// GetCacheStats retrieves caching performance statistics
func (r *DNCCheckResultRepository) GetCacheStats(ctx context.Context) (*dnc.CacheStats, error) {
	stats := &dnc.CacheStats{
		CollectedAt: time.Now(),
	}

	// Total requests (approximated by total results)
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM dnc_check_results
	`).Scan(&stats.TotalRequests)
	if err != nil {
		return nil, errors.NewInternalError("failed to get total requests").WithCause(err)
	}

	// Valid vs expired entries
	err = r.db.QueryRow(ctx, `
		SELECT 
			COUNT(CASE WHEN checked_at + ttl > NOW() THEN 1 END) as valid,
			COUNT(CASE WHEN checked_at + ttl <= NOW() THEN 1 END) as expired
		FROM dnc_check_results
	`).Scan(&stats.ValidEntries, &stats.ExpiredEntries)
	if err != nil {
		return nil, errors.NewInternalError("failed to get cache validity stats").WithCause(err)
	}

	stats.TotalEntries = stats.ValidEntries + stats.ExpiredEntries

	// Calculate hit rate (approximation based on valid cache entries)
	if stats.TotalRequests > 0 {
		stats.HitRate = float64(stats.ValidEntries) / float64(stats.TotalRequests)
	}

	// Average performance metrics
	err = r.db.QueryRow(ctx, `
		SELECT 
			COALESCE(AVG(EXTRACT(EPOCH FROM check_duration)), 0) as avg_time_seconds,
			COALESCE(AVG(EXTRACT(EPOCH FROM ttl)), 0) as avg_age_seconds
		FROM dnc_check_results
		WHERE checked_at > NOW() - INTERVAL '24 hours'
	`).Scan(&stats.AvgHitTime, &stats.AverageEntryAge)
	if err != nil {
		return nil, errors.NewInternalError("failed to get performance metrics").WithCause(err)
	}

	// Convert seconds to time.Duration
	stats.AvgHitTime = time.Duration(stats.AvgHitTime.Seconds() * float64(time.Second))
	stats.AverageEntryAge = time.Duration(stats.AverageEntryAge.Seconds() * float64(time.Second))

	// Set miss time (typically higher than hit time due to additional processing)
	stats.AvgMissTime = stats.AvgHitTime * 3

	return stats, nil
}

// InvalidatePhoneCache invalidates all cached results for a phone number
func (r *DNCCheckResultRepository) InvalidatePhoneCache(ctx context.Context, phoneNumber values.PhoneNumber) error {
	query := `
		UPDATE dnc_check_results 
		SET ttl = INTERVAL '0'
		WHERE phone_number = $1
		  AND checked_at + ttl > NOW()
	`

	_, err := r.db.Exec(ctx, query, phoneNumber.String())
	if err != nil {
		return errors.NewInternalError("failed to invalidate phone cache").WithCause(err)
	}

	return nil
}

// InvalidateProviderCache invalidates cached results from a specific provider
func (r *DNCCheckResultRepository) InvalidateProviderCache(ctx context.Context, providerID uuid.UUID) error {
	query := `
		UPDATE dnc_check_results 
		SET ttl = INTERVAL '0'
		WHERE metadata->>'provider_id' = $1::text
		  AND checked_at + ttl > NOW()
	`

	_, err := r.db.Exec(ctx, query, providerID)
	if err != nil {
		return errors.NewInternalError("failed to invalidate provider cache").WithCause(err)
	}

	return nil
}

// RefreshCache rebuilds cache entries for performance optimization
func (r *DNCCheckResultRepository) RefreshCache(ctx context.Context, phoneNumbers []values.PhoneNumber) error {
	if len(phoneNumbers) == 0 {
		return nil
	}

	phoneStrs := make([]string, len(phoneNumbers))
	for i, phone := range phoneNumbers {
		phoneStrs[i] = phone.String()
	}

	// Extend TTL for frequently accessed numbers
	query := `
		UPDATE dnc_check_results 
		SET ttl = ttl + INTERVAL '1 hour'
		WHERE phone_number = ANY($1)
		  AND checked_at + ttl > NOW()
		  AND checked_at > NOW() - INTERVAL '24 hours'
	`

	_, err := r.db.Exec(ctx, query, phoneStrs)
	if err != nil {
		return errors.NewInternalError("failed to refresh cache").WithCause(err)
	}

	return nil
}

// Cleanup removes expired check results based on TTL and retention policies
func (r *DNCCheckResultRepository) Cleanup(ctx context.Context, retentionPolicy dnc.RetentionPolicy) (*dnc.CleanupResult, error) {
	startTime := time.Now()
	result := &dnc.CleanupResult{
		StartedAt: startTime,
	}

	// Count records to be examined
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM dnc_check_results
		WHERE checked_at < $1
	`, time.Now().Add(-retentionPolicy.MaxAge)).Scan(&result.RecordsExamined)
	if err != nil {
		return nil, errors.NewInternalError("failed to count records for cleanup").WithCause(err)
	}

	batchSize := retentionPolicy.BatchSize
	if batchSize == 0 {
		batchSize = 1000
	}

	var totalDeleted int64
	var totalRetained int64

	for {
		// Process in batches to avoid locking the table for too long
		query := `
			WITH records_to_delete AS (
				SELECT id
				FROM dnc_check_results
				WHERE checked_at + ttl < NOW()
				  AND checked_at < $1
				  AND (
				    ($2 = false OR is_blocked = false) AND
				    ($3 = false OR risk_score < 0.7) AND
				    ($4 = false OR compliance_level != 'strict')
				  )
				LIMIT $5
			)
			DELETE FROM dnc_check_results
			WHERE id IN (SELECT id FROM records_to_delete)
		`

		cutoffTime := time.Now().Add(-retentionPolicy.MaxAge)
		
		deleteResult, err := r.db.Exec(ctx, query,
			cutoffTime,
			retentionPolicy.RetainBlocked,
			retentionPolicy.RetainHighRisk,
			retentionPolicy.RetainCompliance,
			batchSize,
		)
		if err != nil {
			return nil, errors.NewInternalError("failed to cleanup expired results").WithCause(err)
		}

		deleted := deleteResult.RowsAffected()
		totalDeleted += deleted

		if deleted == 0 {
			break // No more records to delete
		}

		// Check execution time limit
		if retentionPolicy.MaxExecutionTime > 0 && time.Since(startTime) > retentionPolicy.MaxExecutionTime {
			break
		}
	}

	// Count retained records
	err = r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM dnc_check_results
		WHERE checked_at < $1
	`, time.Now().Add(-retentionPolicy.MaxAge)).Scan(&totalRetained)
	if err != nil {
		return nil, errors.NewInternalError("failed to count retained records").WithCause(err)
	}

	result.CompletedAt = time.Now()
	result.Duration = result.CompletedAt.Sub(result.StartedAt)
	result.RecordsDeleted = totalDeleted
	result.RecordsRetained = totalRetained

	if result.Duration.Seconds() > 0 {
		result.ThroughputPerSecond = float64(totalDeleted) / result.Duration.Seconds()
	}

	return result, nil
}

// CleanupExpired removes results that have exceeded their TTL
func (r *DNCCheckResultRepository) CleanupExpired(ctx context.Context, before time.Time) (int64, error) {
	query := `
		DELETE FROM dnc_check_results
		WHERE checked_at + ttl < $1
	`

	result, err := r.db.Exec(ctx, query, before)
	if err != nil {
		return 0, errors.NewInternalError("failed to cleanup expired results").WithCause(err)
	}

	return result.RowsAffected(), nil
}

// CleanupByAge removes results older than specified age
func (r *DNCCheckResultRepository) CleanupByAge(ctx context.Context, maxAge time.Duration) (int64, error) {
	cutoffTime := time.Now().Add(-maxAge)

	query := `
		DELETE FROM dnc_check_results
		WHERE checked_at < $1
	`

	result, err := r.db.Exec(ctx, query, cutoffTime)
	if err != nil {
		return 0, errors.NewInternalError("failed to cleanup old results").WithCause(err)
	}

	return result.RowsAffected(), nil
}

// ArchiveOldResults moves old results to archive storage
func (r *DNCCheckResultRepository) ArchiveOldResults(ctx context.Context, archivePolicy dnc.ArchivePolicy) (*dnc.ArchiveResult, error) {
	startTime := time.Now()
	result := &dnc.ArchiveResult{
		StartedAt:       startTime,
		ArchiveLocation: archivePolicy.ArchiveLocation,
	}

	cutoffTime := time.Now().Add(-archivePolicy.ArchiveAge)

	// Count records to archive
	var recordsToArchive int64
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM dnc_check_results WHERE checked_at < $1
	`, cutoffTime).Scan(&recordsToArchive)
	if err != nil {
		return nil, errors.NewInternalError("failed to count records to archive").WithCause(err)
	}

	if recordsToArchive == 0 {
		result.CompletedAt = time.Now()
		result.Duration = result.CompletedAt.Sub(result.StartedAt)
		return result, nil
	}

	// Create archive table if it doesn't exist
	createArchiveQuery := `
		CREATE TABLE IF NOT EXISTS dnc_check_results_archive (
			LIKE dnc_check_results INCLUDING ALL
		)
	`
	_, err = r.db.Exec(ctx, createArchiveQuery)
	if err != nil {
		return nil, errors.NewInternalError("failed to create archive table").WithCause(err)
	}

	// Move data to archive in batches
	batchSize := archivePolicy.BatchSize
	if batchSize == 0 {
		batchSize = 1000
	}

	var totalArchived int64

	for {
		tx, err := r.BeginTx(ctx)
		if err != nil {
			return nil, err
		}

		pgxTx := tx.(*PgxTransaction)

		// Insert into archive
		insertQuery := `
			INSERT INTO dnc_check_results_archive
			SELECT * FROM dnc_check_results
			WHERE checked_at < $1
			LIMIT $2
		`

		insertResult, err := pgxTx.tx.Exec(ctx, insertQuery, cutoffTime, batchSize)
		if err != nil {
			tx.Rollback()
			return nil, errors.NewInternalError("failed to insert into archive").WithCause(err)
		}

		archivedInBatch := insertResult.RowsAffected()
		if archivedInBatch == 0 {
			tx.Rollback()
			break
		}

		// Delete from main table
		deleteQuery := `
			DELETE FROM dnc_check_results
			WHERE id IN (
				SELECT id FROM dnc_check_results
				WHERE checked_at < $1
				LIMIT $2
			)
		`

		_, err = pgxTx.tx.Exec(ctx, deleteQuery, cutoffTime, batchSize)
		if err != nil {
			tx.Rollback()
			return nil, errors.NewInternalError("failed to delete archived records").WithCause(err)
		}

		err = tx.Commit()
		if err != nil {
			return nil, errors.NewInternalError("failed to commit archive transaction").WithCause(err)
		}

		totalArchived += archivedInBatch

		// Check execution time limit
		if archivePolicy.MaxExecutionTime > 0 && time.Since(startTime) > archivePolicy.MaxExecutionTime {
			break
		}
	}

	result.CompletedAt = time.Now()
	result.Duration = result.CompletedAt.Sub(result.StartedAt)
	result.RecordsArchived = totalArchived

	if result.Duration.Seconds() > 0 {
		result.ThroughputPerSecond = float64(totalArchived) / result.Duration.Seconds()
	}

	// Verify integrity if requested
	if archivePolicy.VerifyIntegrity {
		var mainCount, archiveCount int64
		
		err = r.db.QueryRow(ctx, `SELECT COUNT(*) FROM dnc_check_results WHERE checked_at < $1`, cutoffTime).Scan(&mainCount)
		if err == nil {
			err = r.db.QueryRow(ctx, `SELECT COUNT(*) FROM dnc_check_results_archive WHERE checked_at < $1`, cutoffTime).Scan(&archiveCount)
		}
		
		result.IntegrityVerified = (err == nil && mainCount == 0 && archiveCount == totalArchived)
	}

	return result, nil
}

// BulkSave saves multiple check results efficiently
func (r *DNCCheckResultRepository) BulkSave(ctx context.Context, results []*dnc.DNCCheckResult) error {
	if len(results) == 0 {
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

	// Use COPY for maximum performance
	_, err = pgxTx.tx.CopyFrom(
		ctx,
		pgx.Identifier{"dnc_check_results"},
		[]string{
			"id", "phone_number", "is_blocked", "checked_at", "ttl", "check_duration",
			"sources_count", "compliance_level", "risk_score", "reasons", "sources", "metadata",
		},
		pgx.CopyFromSlice(len(results), func(i int) ([]interface{}, error) {
			result := results[i]

			reasonsJSON, err := json.Marshal(result.Reasons)
			if err != nil {
				return nil, err
			}

			sourcesJSON, err := json.Marshal(result.Sources)
			if err != nil {
				return nil, err
			}

			metadataJSON, err := json.Marshal(result.Metadata)
			if err != nil {
				return nil, err
			}

			return []interface{}{
				result.ID,
				result.PhoneNumber.String(),
				result.IsBlocked,
				result.CheckedAt,
				result.TTL,
				result.CheckDuration,
				result.SourcesCount,
				result.ComplianceLevel,
				result.RiskScore,
				reasonsJSON,
				sourcesJSON,
				metadataJSON,
			}, nil
		}),
	)

	if err != nil {
		return errors.NewInternalError("failed to bulk save DNC check results").WithCause(err)
	}

	return tx.Commit()
}

// BulkDelete removes multiple check results
func (r *DNCCheckResultRepository) BulkDelete(ctx context.Context, resultIDs []uuid.UUID) error {
	if len(resultIDs) == 0 {
		return nil
	}

	query := `
		DELETE FROM dnc_check_results
		WHERE id = ANY($1)
	`

	_, err := r.db.Exec(ctx, query, resultIDs)
	if err != nil {
		return errors.NewInternalError("failed to bulk delete DNC check results").WithCause(err)
	}

	return nil
}

// BulkInvalidate invalidates multiple cache entries
func (r *DNCCheckResultRepository) BulkInvalidate(ctx context.Context, phoneNumbers []values.PhoneNumber) error {
	if len(phoneNumbers) == 0 {
		return nil
	}

	phoneStrs := make([]string, len(phoneNumbers))
	for i, phone := range phoneNumbers {
		phoneStrs[i] = phone.String()
	}

	query := `
		UPDATE dnc_check_results 
		SET ttl = INTERVAL '0'
		WHERE phone_number = ANY($1)
		  AND checked_at + ttl > NOW()
	`

	_, err := r.db.Exec(ctx, query, phoneStrs)
	if err != nil {
		return errors.NewInternalError("failed to bulk invalidate cache").WithCause(err)
	}

	return nil
}

// Find searches for check results based on filter criteria with pagination
func (r *DNCCheckResultRepository) Find(ctx context.Context, filter dnc.DNCCheckFilter) (*dnc.DNCCheckResultPage, error) {
	startTime := time.Now()
	
	whereClause, args, err := r.buildCheckResultWhereClause(filter)
	if err != nil {
		return nil, err
	}

	// Count query
	countQuery := fmt.Sprintf(`
		SELECT COUNT(*) 
		FROM dnc_check_results 
		%s
	`, whereClause)

	var totalCount int64
	err = r.db.QueryRow(ctx, countQuery, args...).Scan(&totalCount)
	if err != nil {
		return nil, errors.NewInternalError("failed to count DNC check results").WithCause(err)
	}

	// Data query
	orderBy := "checked_at"
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

	selectFields := `id, phone_number, is_blocked, checked_at, ttl, check_duration,
					 sources_count, compliance_level, risk_score, reasons, sources, metadata`
	if !filter.IncludeReasons {
		selectFields = `id, phone_number, is_blocked, checked_at, ttl, check_duration,
						sources_count, compliance_level, risk_score, '[]' as reasons, sources, metadata`
	}
	if !filter.IncludeMetadata {
		selectFields = `id, phone_number, is_blocked, checked_at, ttl, check_duration,
						sources_count, compliance_level, risk_score, reasons, sources, '{}' as metadata`
	}

	dataQuery := fmt.Sprintf(`
		SELECT %s
		FROM dnc_check_results 
		%s
		ORDER BY %s %s
		LIMIT $%d OFFSET $%d
	`, selectFields, whereClause, orderBy, direction, len(args)+1, len(args)+2)

	args = append(args, limit, offset)

	rows, err := r.db.Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, errors.NewInternalError("failed to query DNC check results").WithCause(err)
	}
	defer rows.Close()

	results, err := r.scanDNCCheckResults(rows)
	if err != nil {
		return nil, err
	}

	queryTime := time.Since(startTime)
	hasMore := totalCount > int64(offset+len(results))

	return &dnc.DNCCheckResultPage{
		Results:    results,
		TotalCount: totalCount,
		HasMore:    hasMore,
		QueryTime:  queryTime,
		// Cache stats would be populated by higher-level cache layer
	}, nil
}

// Count returns the total number of results matching the filter
func (r *DNCCheckResultRepository) Count(ctx context.Context, filter dnc.DNCCheckFilter) (int64, error) {
	whereClause, args, err := r.buildCheckResultWhereClause(filter)
	if err != nil {
		return 0, err
	}

	query := fmt.Sprintf(`
		SELECT COUNT(*) 
		FROM dnc_check_results 
		%s
	`, whereClause)

	var count int64
	err = r.db.QueryRow(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, errors.NewInternalError("failed to count DNC check results").WithCause(err)
	}

	return count, nil
}

// FindByRiskScore retrieves results by risk score range
func (r *DNCCheckResultRepository) FindByRiskScore(ctx context.Context, minScore, maxScore float64, limit int) ([]*dnc.DNCCheckResult, error) {
	query := `
		SELECT id, phone_number, is_blocked, checked_at, ttl, check_duration,
			   sources_count, compliance_level, risk_score, reasons, sources, metadata
		FROM dnc_check_results
		WHERE risk_score BETWEEN $1 AND $2
		ORDER BY risk_score DESC, checked_at DESC
		LIMIT $3
	`

	rows, err := r.db.Query(ctx, query, minScore, maxScore, limit)
	if err != nil {
		return nil, errors.NewInternalError("failed to query by risk score").WithCause(err)
	}
	defer rows.Close()

	return r.scanDNCCheckResults(rows)
}

// FindByDecision retrieves results by compliance decision
func (r *DNCCheckResultRepository) FindByDecision(ctx context.Context, decision string, timeRange dnc.TimeRange) ([]*dnc.DNCCheckResult, error) {
	query := `
		SELECT id, phone_number, is_blocked, checked_at, ttl, check_duration,
			   sources_count, compliance_level, risk_score, reasons, sources, metadata
		FROM dnc_check_results
		WHERE compliance_level = $1
		  AND checked_at BETWEEN $2 AND $3
		ORDER BY checked_at DESC
	`

	rows, err := r.db.Query(ctx, query, decision, timeRange.Start, timeRange.End)
	if err != nil {
		return nil, errors.NewInternalError("failed to query by decision").WithCause(err)
	}
	defer rows.Close()

	return r.scanDNCCheckResults(rows)
}

// Implement remaining methods (analytics, reporting, transaction support)...

// BeginTx starts a new database transaction
func (r *DNCCheckResultRepository) BeginTx(ctx context.Context) (dnc.Transaction, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, errors.NewInternalError("failed to begin transaction").WithCause(err)
	}

	return &PgxTransaction{tx: tx, ctx: ctx}, nil
}

// WithTx executes a function within a database transaction
func (r *DNCCheckResultRepository) WithTx(ctx context.Context, fn func(tx dnc.Transaction) error) error {
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

// scanDNCCheckResult scans a single DNC check result from a database row
func (r *DNCCheckResultRepository) scanDNCCheckResult(row pgx.Row) (*dnc.DNCCheckResult, error) {
	var result dnc.DNCCheckResult
	var phoneNumber string
	var reasonsJSON, sourcesJSON, metadataJSON []byte

	err := row.Scan(
		&result.ID,
		&phoneNumber,
		&result.IsBlocked,
		&result.CheckedAt,
		&result.TTL,
		&result.CheckDuration,
		&result.SourcesCount,
		&result.ComplianceLevel,
		&result.RiskScore,
		&reasonsJSON,
		&sourcesJSON,
		&metadataJSON,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errors.NewNotFoundError("DNC check result not found")
		}
		return nil, errors.NewInternalError("failed to scan DNC check result").WithCause(err)
	}

	// Convert phone number
	phone, err := values.NewPhoneNumber(phoneNumber)
	if err != nil {
		return nil, errors.NewValidationError("INVALID_PHONE_NUMBER", "invalid phone number in database").WithCause(err)
	}
	result.PhoneNumber = phone

	// Unmarshal JSON fields
	if reasonsJSON != nil {
		err = json.Unmarshal(reasonsJSON, &result.Reasons)
		if err != nil {
			return nil, errors.NewInternalError("failed to unmarshal reasons").WithCause(err)
		}
	}

	if sourcesJSON != nil {
		err = json.Unmarshal(sourcesJSON, &result.Sources)
		if err != nil {
			return nil, errors.NewInternalError("failed to unmarshal sources").WithCause(err)
		}
	}

	if metadataJSON != nil {
		err = json.Unmarshal(metadataJSON, &result.Metadata)
		if err != nil {
			return nil, errors.NewInternalError("failed to unmarshal metadata").WithCause(err)
		}
	}

	return &result, nil
}

// scanDNCCheckResults scans multiple DNC check results from database rows
func (r *DNCCheckResultRepository) scanDNCCheckResults(rows pgx.Rows) ([]*dnc.DNCCheckResult, error) {
	var results []*dnc.DNCCheckResult

	for rows.Next() {
		result, err := r.scanDNCCheckResult(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}

	if err := rows.Err(); err != nil {
		return nil, errors.NewInternalError("error iterating DNC check results").WithCause(err)
	}

	return results, nil
}

// buildCheckResultWhereClause builds a WHERE clause and arguments from a filter
func (r *DNCCheckResultRepository) buildCheckResultWhereClause(filter dnc.DNCCheckFilter) (string, []interface{}, error) {
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

	// Result filters
	if filter.IsBlocked != nil {
		conditions = append(conditions, fmt.Sprintf("is_blocked = $%d", argIndex))
		args = append(args, *filter.IsBlocked)
		argIndex++
	}

	if len(filter.Decisions) > 0 {
		conditions = append(conditions, fmt.Sprintf("compliance_level = ANY($%d)", argIndex))
		args = append(args, filter.Decisions)
		argIndex++
	}

	// Risk score filters
	if filter.RiskScoreMin != nil {
		conditions = append(conditions, fmt.Sprintf("risk_score >= $%d", argIndex))
		args = append(args, *filter.RiskScoreMin)
		argIndex++
	}

	if filter.RiskScoreMax != nil {
		conditions = append(conditions, fmt.Sprintf("risk_score <= $%d", argIndex))
		args = append(args, *filter.RiskScoreMax)
		argIndex++
	}

	// Time range filters
	if filter.CheckedAfter != nil {
		conditions = append(conditions, fmt.Sprintf("checked_at > $%d", argIndex))
		args = append(args, *filter.CheckedAfter)
		argIndex++
	}

	if filter.CheckedBefore != nil {
		conditions = append(conditions, fmt.Sprintf("checked_at < $%d", argIndex))
		args = append(args, *filter.CheckedBefore)
		argIndex++
	}

	// Performance filters
	if filter.DurationMin != nil {
		conditions = append(conditions, fmt.Sprintf("check_duration >= $%d", argIndex))
		args = append(args, *filter.DurationMin)
		argIndex++
	}

	if filter.DurationMax != nil {
		conditions = append(conditions, fmt.Sprintf("check_duration <= $%d", argIndex))
		args = append(args, *filter.DurationMax)
		argIndex++
	}

	// Cache filters
	if filter.OnlyExpired != nil && *filter.OnlyExpired {
		conditions = append(conditions, "checked_at + ttl <= NOW()")
	}

	if filter.OnlyValid != nil && *filter.OnlyValid {
		conditions = append(conditions, "checked_at + ttl > NOW()")
	}

	// Search text
	if filter.SearchText != nil {
		conditions = append(conditions, fmt.Sprintf("metadata::text ILIKE $%d", argIndex))
		searchPattern := "%" + *filter.SearchText + "%"
		args = append(args, searchPattern)
		argIndex++
	}

	// Build final WHERE clause
	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	return whereClause, args, nil
}

// Note: Additional methods for analytics and reporting would be implemented here
// GetCheckMetrics, GetComplianceReport, GetPerformanceMetrics, GetTrendAnalysis, etc.
// These follow similar patterns but with more complex aggregation queries